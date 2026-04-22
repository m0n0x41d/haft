defmodule OpenSleigh.Haft.ClientTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.{Fixtures, Haft.Client, Haft.Mock}

  setup do
    {:ok, haft} = Mock.start()
    invoke_fun = Mock.invoke_fun(haft)
    session = Fixtures.adapter_session()

    %{haft: haft, invoke_fun: invoke_fun, session: session}
  end

  test "call_tool/5 round-trips a success response", ctx do
    assert {:ok, encoded_result} =
             Client.call_tool(ctx.session, :haft_query, :status, %{}, ctx.invoke_fun)

    assert {:ok, %{"artifact_id" => "mock-haft-" <> _}} = Jason.decode(encoded_result)
  end

  test "call_tool/5 stores the request in the mock's artifacts", ctx do
    {:ok, _} =
      Client.call_tool(ctx.session, :haft_note, :append, %{"body" => "test"}, ctx.invoke_fun)

    [request_params] = Mock.artifacts(ctx.haft)
    assert request_params["name"] == "haft_note"
    assert request_params["arguments"]["body"] == "test"
  end

  test "write_artifact/3 serialises a PhaseOutcome and dispatches to the proper tool", ctx do
    outcome = Fixtures.phase_outcome()

    assert {:ok, _encoded_result} =
             Client.write_artifact(ctx.session, outcome, ctx.invoke_fun)

    [request_params] = Mock.artifacts(ctx.haft)
    # Execute phase routes through haft_note per Client.tool_for_phase/1.
    assert request_params["name"] == "haft_note"
    assert request_params["arguments"]["config_hash"] == ctx.session.config_hash
  end

  test "call_tool/5 propagates invoker errors", ctx do
    bad_invoker = fn _line -> {:error, :haft_unavailable} end

    assert {:error, :haft_unavailable} =
             Client.call_tool(ctx.session, :haft_query, :status, %{}, bad_invoker)
  end

  test "fetch_problem_card/3 decodes MCP content text", ctx do
    invoker = fn request_line ->
      {:ok, %{"id" => id}} = Jason.decode(request_line)

      card =
        Jason.encode!(%{
          "problem_card" => %{
            "id" => "haft-pc-1",
            "describedEntity" => "lib/open_sleigh/orchestrator.ex",
            "groundingHolon" => "OpenSleigh.Orchestrator",
            "authoring_source" => "human"
          }
        })

      response =
        %{
          "jsonrpc" => "2.0",
          "id" => id,
          "result" => %{"content" => [%{"type" => "text", "text" => card}]}
        }
        |> Jason.encode!()
        |> Kernel.<>("\n")

      {:ok, response}
    end

    assert {:ok, card} = Client.fetch_problem_card(ctx.session, "haft-pc-1", invoker)
    assert card["describedEntity"] == "lib/open_sleigh/orchestrator.ex"
    assert card["ref"] == "haft-pc-1"
  end

  test "fetch_problem_card/3 accepts canonical contract fixture shapes", ctx do
    fixtures = [
      %{"problem_card" => problem_card_fixture("haft-pc-shape")},
      %{"problemCard" => problem_card_fixture("haft-pc-shape")},
      %{"artifact" => problem_card_fixture("haft-pc-shape")},
      %{"artifacts" => [problem_card_fixture("other"), problem_card_fixture("haft-pc-shape")]},
      %{"related" => [problem_card_fixture("haft-pc-shape")]}
    ]

    Enum.each(fixtures, fn result ->
      assert {:ok, card} =
               Client.fetch_problem_card(
                 ctx.session,
                 "haft-pc-shape",
                 problem_card_invoker(result)
               )

      assert card["ref"] == "haft-pc-shape"
      assert card["describedEntity"] == "lib/open_sleigh/orchestrator.ex"
      assert card["valid_until"] == "2026-05-01T00:00:00Z"
    end)
  end

  test "fetch_problem_card/3 rejects missing and malformed contract fixtures", ctx do
    malformed_results = [
      %{},
      %{"problem_card" => "not a map"},
      %{"artifacts" => [problem_card_fixture("other")]},
      %{"content" => [%{"type" => "text", "text" => "not json"}]}
    ]

    Enum.each(malformed_results, fn result ->
      assert {:error, :tracker_response_malformed} =
               Client.fetch_problem_card(
                 ctx.session,
                 "haft-pc-missing",
                 problem_card_invoker(result)
               )
    end)
  end

  test "fetch_problem_card/3 normalizes self-authored and stale fixtures", ctx do
    result =
      %{
        "problem_card" =>
          "haft-pc-self"
          |> problem_card_fixture()
          |> Map.put("authoring_source", "open_sleigh_self")
          |> Map.put("valid_until", "2000-01-01T00:00:00Z")
      }

    assert {:ok, card} =
             Client.fetch_problem_card(ctx.session, "haft-pc-self", problem_card_invoker(result))

    assert card[:authoring_source] == :open_sleigh_self
    assert card["valid_until"] == "2000-01-01T00:00:00Z"
  end

  test "call_tool/5 rejects unknown tool atom at encode time", ctx do
    assert {:error, :tool_unknown_to_adapter} =
             Client.call_tool(ctx.session, :haft_nonexistent, :x, %{}, ctx.invoke_fun)
  end

  defp problem_card_invoker(result) do
    fn request_line ->
      {:ok, %{"id" => id}} = Jason.decode(request_line)

      response =
        %{
          "jsonrpc" => "2.0",
          "id" => id,
          "result" => result
        }
        |> Jason.encode!()
        |> Kernel.<>("\n")

      {:ok, response}
    end
  end

  defp problem_card_fixture(ref) do
    %{
      "id" => ref,
      "describedEntity" => "lib/open_sleigh/orchestrator.ex",
      "groundingHolon" => "OpenSleigh.Orchestrator",
      "description" => "Contract fixture ProblemCard",
      "authoring_source" => "human",
      "valid_until" => "2026-05-01T00:00:00Z"
    }
  end
end
