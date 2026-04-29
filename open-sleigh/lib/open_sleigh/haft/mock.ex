defmodule OpenSleigh.Haft.Mock do
  @moduledoc """
  In-memory Haft for tests. Implements the `invoke_fun` signature
  used by `OpenSleigh.Haft.Client`, echoing well-formed responses
  without spawning `haft serve`.

  Uses an Agent for artifact storage so tests can inspect what was
  "written" to Haft after a call.
  """

  alias OpenSleigh.EffectError

  @type problem_card_mode :: :none | :generated
  @type commission_mode :: :none | :generated
  @type mock_modes :: %{
          problem_cards: problem_card_mode(),
          commissions: commission_mode()
        }

  @doc "Start a fresh mock Haft."
  @spec start(keyword()) :: {:ok, pid()}
  def start(opts \\ []) do
    Agent.start_link(fn ->
      %{
        artifacts: [],
        next_id: 1,
        problem_cards: Keyword.get(opts, :problem_cards, :none),
        commissions: Keyword.get(opts, :commissions, :none)
      }
    end)
  end

  @doc """
  Build an `invoke_fun` compatible with `Haft.Client.call_tool/5`
  and `write_artifact/3` — echoes back a success response with the
  same id.
  """
  @spec invoke_fun(pid()) ::
          (binary() -> {:ok, binary()} | {:error, EffectError.t()})
  def invoke_fun(handle) do
    fn request_line -> handle_request(handle, request_line) end
  end

  @spec handle_request(pid(), binary()) ::
          {:ok, binary()} | {:error, EffectError.t()}
  defp handle_request(handle, request_line) do
    with {:ok, %{"id" => id, "params" => params}} <- Jason.decode(request_line) do
      :ok = record_artifact(handle, params)

      modes =
        handle
        |> mock_modes()

      params
      |> response_for_params(id, modes)
      |> then(&{:ok, &1})
    else
      _ -> {:error, :response_parse_error}
    end
  end

  @spec record_artifact(pid(), map()) :: :ok
  defp record_artifact(handle, params) do
    Agent.update(handle, fn state ->
      %{state | artifacts: [params | state.artifacts]}
    end)
  end

  @spec mock_modes(pid()) :: mock_modes()
  defp mock_modes(handle) do
    handle
    |> Agent.get(&mock_modes_from_state/1)
  end

  @spec mock_modes_from_state(map()) :: mock_modes()
  defp mock_modes_from_state(state) do
    %{
      problem_cards:
        state
        |> Map.get(:problem_cards, :none)
        |> problem_card_mode_value(),
      commissions:
        state
        |> Map.get(:commissions, :none)
        |> commission_mode_value()
    }
  end

  @spec problem_card_mode_value(term()) :: problem_card_mode()
  defp problem_card_mode_value(:generated), do: :generated
  defp problem_card_mode_value(_value), do: :none

  @spec commission_mode_value(term()) :: commission_mode()
  defp commission_mode_value(:generated), do: :generated
  defp commission_mode_value(_value), do: :none

  @spec response_for_params(map(), integer(), mock_modes()) :: binary()
  defp response_for_params(
         %{"name" => "haft_query", "arguments" => %{"action" => "related"} = arguments},
         id,
         %{problem_cards: :generated}
       ) do
    id
    |> problem_card_response(problem_card_ref(arguments))
  end

  defp response_for_params(
         %{
           "name" => "haft_commission",
           "arguments" => %{"action" => "list_runnable"}
         },
         id,
         %{commissions: :generated}
       ) do
    id
    |> commission_list_response()
  end

  defp response_for_params(
         %{
           "name" => "haft_commission",
           "arguments" => %{"action" => "claim_for_preflight"} = arguments
         },
         id,
         %{commissions: :generated}
       ) do
    arguments
    |> claimed_commission_ref()
    |> commission_claim_response(id)
  end

  defp response_for_params(_params, id, _modes), do: ok_response(id)

  @spec problem_card_ref(map()) :: String.t()
  defp problem_card_ref(arguments) do
    arguments
    |> Map.get("artifact_id", Map.get(arguments, "ref", "mock-problem-card"))
    |> problem_card_ref_value()
  end

  @spec problem_card_ref_value(term()) :: String.t()
  defp problem_card_ref_value(value) when is_binary(value) and value != "", do: value
  defp problem_card_ref_value(_value), do: "mock-problem-card"

  @spec ok_response(integer()) :: binary()
  defp ok_response(id) do
    body =
      Jason.encode!(%{
        "jsonrpc" => "2.0",
        "id" => id,
        "result" => %{"artifact_id" => "mock-haft-" <> Integer.to_string(id)}
      })

    body <> "\n"
  end

  @spec problem_card_response(integer(), String.t()) :: binary()
  defp problem_card_response(id, ref) do
    body =
      %{
        "jsonrpc" => "2.0",
        "id" => id,
        "result" => %{"problem_card" => problem_card_fixture(ref)}
      }
      |> Jason.encode!()

    body <> "\n"
  end

  @spec commission_list_response(integer()) :: binary()
  defp commission_list_response(id) do
    body =
      %{
        "jsonrpc" => "2.0",
        "id" => id,
        "result" => %{
          "commissions" => [
            commission_fixture("wc-haft-bootstrap-001", "queued")
          ]
        }
      }
      |> Jason.encode!()

    body <> "\n"
  end

  @spec commission_claim_response(String.t(), integer()) :: binary()
  defp commission_claim_response(commission_id, id) do
    body =
      %{
        "jsonrpc" => "2.0",
        "id" => id,
        "result" => %{
          "commission" => commission_fixture(commission_id, "preflighting")
        }
      }
      |> Jason.encode!()

    body <> "\n"
  end

  @spec claimed_commission_ref(map()) :: String.t()
  defp claimed_commission_ref(arguments) do
    arguments
    |> Map.get("commission_id", "wc-haft-bootstrap-001")
    |> claimed_commission_ref_value()
  end

  @spec claimed_commission_ref_value(term()) :: String.t()
  defp claimed_commission_ref_value(value) when is_binary(value) and value != "", do: value
  defp claimed_commission_ref_value(_value), do: "wc-haft-bootstrap-001"

  @spec problem_card_fixture(String.t()) :: map()
  defp problem_card_fixture(ref) do
    %{
      "id" => ref,
      "ref" => ref,
      "artifact_id" => ref,
      "title" => "Mock ProblemCard " <> ref,
      "body" => "Mock Haft ProblemCard for local Open-Sleigh harness runs.",
      "description" => "Mock Haft ProblemCard for local Open-Sleigh harness runs.",
      "describedEntity" => "open-sleigh/lib/open_sleigh/agent_worker.ex",
      "groundingHolon" => "OpenSleigh.AgentWorker",
      "authoring_source" => "human",
      "valid_until" => "2099-01-01T00:00:00Z"
    }
  end

  @spec commission_fixture(String.t(), String.t()) :: map()
  defp commission_fixture(id, state) do
    %{
      "id" => id,
      "decision_ref" => "dec-haft-bootstrap-001",
      "decision_revision_hash" => "decision-r1",
      "problem_card_ref" => "pc-haft-bootstrap-001",
      "implementation_plan_ref" => "plan-haft-bootstrap-001",
      "implementation_plan_revision" => "plan-r1",
      "evidence_requirements" => [
        %{
          "kind" => "mix_test",
          "command" => "mix test"
        }
      ],
      "projection_policy" => "local_only",
      "state" => state,
      "valid_until" => "2099-01-01T00:00:00Z",
      "fetched_at" => "2026-04-22T10:00:00Z",
      "scope" => %{
        "repo_ref" => "local:open-sleigh-bootstrap",
        "base_sha" => "base-r1",
        "target_branch" => "feature/open-sleigh-haft-bootstrap",
        "allowed_paths" => [
          "open-sleigh/lib/open_sleigh/**",
          "open-sleigh/test/open_sleigh/**",
          "open-sleigh/fixtures/commissions/**"
        ],
        "forbidden_paths" => [
          ".env",
          ".env.local"
        ],
        "allowed_actions" => [
          "edit_files",
          "run_tests"
        ],
        "affected_files" => [
          "open-sleigh/lib/open_sleigh/commission_source/haft.ex",
          "open-sleigh/lib/mix/tasks/open_sleigh.start.ex"
        ],
        "allowed_modules" => [
          "OpenSleigh.CommissionSource.Haft",
          "Mix.Tasks.OpenSleigh.Start"
        ],
        "lockset" => [
          "open-sleigh/lib/open_sleigh/commission_source/haft.ex",
          "open-sleigh/lib/mix/tasks/open_sleigh.start.ex"
        ]
      }
    }
  end

  @doc "Read back all artifacts the mock received, oldest-first."
  @spec artifacts(pid()) :: [map()]
  def artifacts(handle), do: handle |> Agent.get(& &1.artifacts) |> Enum.reverse()
end
