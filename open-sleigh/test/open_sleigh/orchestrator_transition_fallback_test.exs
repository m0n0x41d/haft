defmodule OpenSleigh.OrchestratorTransitionFallbackTest do
  use ExUnit.Case, async: false

  alias OpenSleigh.Agent.Mock, as: AgentMock
  alias OpenSleigh.Haft.Mock, as: HaftMock
  alias OpenSleigh.Tracker.Mock, as: TrackerMock

  alias OpenSleigh.{
    JudgeClient,
    ObservationsBus,
    Orchestrator,
    PhaseConfig,
    Workflow,
    WorkflowStore
  }

  defmodule TransitionFailureTracker do
    @behaviour OpenSleigh.Tracker.Adapter

    @impl true
    def adapter_kind, do: :transition_failure_test
    @impl true
    def list_active(handle), do: TrackerMock.list_active(handle)
    @impl true
    def get(handle, ticket_id), do: TrackerMock.get(handle, ticket_id)
    @impl true
    def transition(_handle, _ticket_id, _state), do: {:error, :tracker_request_failed}
    @impl true
    def post_comment(handle, ticket_id, body),
      do: TrackerMock.post_comment(handle, ticket_id, body)

    @impl true
    def list_comments(handle, ticket_id), do: TrackerMock.list_comments(handle, ticket_id)
  end

  setup do
    ObservationsBus.reset()

    workspace_root =
      System.tmp_dir!()
      |> Path.join("orchestrator_transition_fallback_#{System.unique_integer([:positive])}")

    File.mkdir_p!(workspace_root)
    on_exit(fn -> File.rm_rf!(workspace_root) end)

    {:ok, haft} = HaftMock.start()
    {:ok, tracker} = TrackerMock.start()

    :ok =
      TrackerMock.seed(tracker, [
        %{
          id: "OCT-TRANSITION",
          source: {:linear, "oct"},
          title: "Transition fallback",
          body: "Exercise transition fallback",
          state: :in_progress,
          problem_card_ref: "haft-pc-transition",
          fetched_at: ~U[2026-04-22 10:00:00Z],
          target_branch: "feature/transition-fallback"
        }
      ])

    {:ok, orchestrator} = start_orchestrator(tracker, haft, workspace_root)

    %{
      orchestrator: orchestrator,
      tracker: tracker
    }
  end

  test "terminal tracker transition failure posts operator-visible fallback", ctx do
    {:ok, tickets} = TrackerMock.list_active(ctx.tracker)
    Orchestrator.submit_candidates(ctx.orchestrator, tickets)

    assert :ok = wait_for_terminal(ctx.orchestrator, 2_000)

    {:ok, ticket} = TrackerMock.get(ctx.tracker, "OCT-TRANSITION")
    assert ticket.state == :in_progress

    comments = TrackerMock.comments(ctx.tracker, "OCT-TRANSITION")
    assert Enum.any?(comments, &String.contains?(&1, "could not transition this ticket"))
    assert Enum.any?(comments, &String.contains?(&1, "open-sleigh:transition-failed:Done"))

    assert Enum.any?(
             ObservationsBus.snapshot(),
             &(&1.metric == :tracker_transition_failed and &1.tags.ticket == "OCT-TRANSITION")
           )
  end

  defp start_orchestrator(tracker, haft, workspace_root) do
    suffix = System.unique_integer([:positive])
    store = String.to_atom("ws_transition_fallback_#{suffix}")
    orchestrator = String.to_atom("orch_transition_fallback_#{suffix}")

    {:ok, _store} =
      WorkflowStore.start_link(
        phase_configs: %{
          frame: phase_config(:frame, :frame_verifier, 1),
          execute: phase_config(:execute, :executor, 20),
          measure: phase_config(:measure, :measurer, 1)
        },
        prompts: %{frame: "Frame", execute: "Execute", measure: "Measure"},
        name: store
      )

    judge_fun = JudgeClient.judge_fun(fn _prompt -> {:ok, %{}} end, %{})

    Orchestrator.start_link(
      workflow: Workflow.mvp1(),
      tracker_handle: tracker,
      tracker_adapter: TransitionFailureTracker,
      agent_adapter: AgentMock,
      external_publication: %{tracker_transition_to: ["Done"]},
      judge_fun: judge_fun,
      haft_invoker: HaftMock.invoke_fun(haft),
      workspace_root: workspace_root,
      guard_config: %{forbidden_paths: [], forbidden_remote_substrings: ["open-sleigh"]},
      task_supervisor: OpenSleigh.AgentSupervisor,
      workflow_store: store,
      name: orchestrator
    )
  end

  defp phase_config(phase, role, max_turns) do
    PhaseConfig.new(%{
      phase: phase,
      agent_role: role,
      tools: [],
      gates: %{structural: [], semantic: [], human: []},
      prompt_template_key: phase,
      max_turns: max_turns,
      default_valid_until_days: 30
    })
    |> unwrap!()
  end

  defp wait_for_terminal(orchestrator, timeout_ms) do
    deadline = System.monotonic_time(:millisecond) + timeout_ms
    do_wait_for_terminal(orchestrator, deadline)
  end

  defp do_wait_for_terminal(orchestrator, deadline) do
    status = Orchestrator.status(orchestrator)

    cond do
      status.claimed == [] and status.running == [] ->
        :ok

      System.monotonic_time(:millisecond) > deadline ->
        {:error, :timeout}

      true ->
        Process.sleep(10)
        do_wait_for_terminal(orchestrator, deadline)
    end
  end

  defp unwrap!({:ok, value}), do: value
end
