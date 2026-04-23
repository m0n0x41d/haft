defmodule OpenSleigh.CommissionSource.HaftTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.CommissionSource.Haft
  alias OpenSleigh.Haft.Mock
  alias OpenSleigh.WorkCommission

  setup do
    {:ok, haft} = Mock.start(commissions: :generated)

    %{
      haft: haft,
      invoke_fun: Mock.invoke_fun(haft)
    }
  end

  test "lists runnable commissions through haft_commission", ctx do
    assert {:ok, handle} =
             Haft.new(
               %{
                 "commission_source" => %{
                   "kind" => "haft",
                   "selector" => "runnable"
                 }
               },
               ctx.invoke_fun
             )

    assert Haft.adapter_kind() == :haft
    assert {:ok, [%WorkCommission{} = commission]} = Haft.list_runnable(handle)
    assert commission.id == "wc-haft-bootstrap-001"
    assert commission.state == :queued
    assert commission.scope_hash == commission.scope.hash
  end

  test "claims a named commission for preflight through haft_commission", ctx do
    assert {:ok, handle} = Haft.new(%{commission_source: %{kind: "haft"}}, ctx.invoke_fun)

    assert {:ok, %WorkCommission{} = claimed} =
             Haft.claim_for_preflight(handle, "wc-haft-bootstrap-001")

    assert claimed.id == "wc-haft-bootstrap-001"
    assert claimed.state == :preflighting
  end

  test "accepts MCP content text payloads" do
    payload =
      %{
        "commissions" => [
          commission_payload("wc-content-001", "ready", "2099-01-01T00:00:00Z")
        ]
      }
      |> Jason.encode!()

    invoke_fun = content_invoker(payload)

    assert {:ok, handle} = Haft.new(%{commission_source: %{kind: "haft"}}, invoke_fun)
    assert {:ok, [commission]} = Haft.list_runnable(handle)
    assert commission.id == "wc-content-001"
  end

  test "rejects malformed commission responses" do
    invoke_fun = result_invoker(%{"not_commissions" => []})

    assert {:ok, handle} = Haft.new(%{commission_source: %{kind: "haft"}}, invoke_fun)
    assert {:error, :commission_response_malformed} = Haft.list_runnable(handle)
  end

  test "propagates known claim conflicts from Haft" do
    invoke_fun = tool_error_invoker("commission_lock_conflict")

    assert {:ok, handle} = Haft.new(%{commission_source: %{kind: "haft"}}, invoke_fun)
    assert {:error, :commission_lock_conflict} = Haft.claim_for_preflight(handle, "wc-locked")
  end

  defp content_invoker(text) do
    fn request_line ->
      {:ok, %{"id" => id}} = Jason.decode(request_line)

      response =
        %{
          "jsonrpc" => "2.0",
          "id" => id,
          "result" => %{
            "content" => [
              %{
                "type" => "text",
                "text" => text
              }
            ]
          }
        }
        |> Jason.encode!()
        |> Kernel.<>("\n")

      {:ok, response}
    end
  end

  defp result_invoker(result) do
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

  defp tool_error_invoker(reason) do
    fn request_line ->
      {:ok, %{"id" => id}} = Jason.decode(request_line)

      response =
        %{
          "jsonrpc" => "2.0",
          "id" => id,
          "result" => %{
            "content" => [%{"type" => "text", "text" => reason}],
            "isError" => true
          }
        }
        |> Jason.encode!()
        |> Kernel.<>("\n")

      {:ok, response}
    end
  end

  defp commission_payload(id, state, valid_until) do
    %{
      "id" => id,
      "decision_ref" => "dec-20260422-001",
      "decision_revision_hash" => "decision-r1",
      "problem_card_ref" => "pc-20260422-001",
      "evidence_requirements" => [
        %{
          "kind" => "mix_test",
          "command" => "mix test"
        }
      ],
      "projection_policy" => "local_only",
      "state" => state,
      "valid_until" => valid_until,
      "fetched_at" => "2026-04-22T10:00:00Z",
      "scope" => %{
        "repo_ref" => "github:m0n0x41d/haft",
        "base_sha" => "abc123",
        "target_branch" => "feature/commission-source",
        "allowed_paths" => [
          "open-sleigh/lib/open_sleigh/commission_source/haft.ex"
        ],
        "forbidden_paths" => [],
        "allowed_actions" => [
          "edit_files",
          "run_tests"
        ],
        "affected_files" => [
          "open-sleigh/lib/open_sleigh/commission_source/haft.ex"
        ],
        "lockset" => [
          "open-sleigh/lib/open_sleigh/commission_source/haft.ex"
        ]
      }
    }
  end
end
