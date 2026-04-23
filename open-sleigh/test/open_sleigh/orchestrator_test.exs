defmodule OpenSleigh.OrchestratorTest do
  use ExUnit.Case, async: false

  alias OpenSleigh.Agent.Mock, as: AgentMock
  alias OpenSleigh.Haft.Mock, as: HaftMock
  alias OpenSleigh.Tracker.Mock, as: TrackerMock

  alias OpenSleigh.{
    JudgeClient,
    ObservationsBus,
    Orchestrator,
    PhaseConfig,
    Ticket,
    Workflow,
    WorkflowStore
  }

  setup do
    ObservationsBus.reset()

    workspace_root =
      System.tmp_dir!()
      |> Path.join("orchestrator_commission_#{unique_suffix()}")

    File.mkdir_p!(workspace_root)
    on_exit(fn -> File.rm_rf!(workspace_root) end)

    {:ok, haft} = HaftMock.start()
    {:ok, tracker} = TrackerMock.start()

    %{
      haft: haft,
      tracker: tracker,
      workspace_root: workspace_root
    }
  end

  test "rejects session construction when commission scope hash mismatches scope snapshot", ctx do
    ticket =
      ticket!(
        id: "OCT-SCOPE-DRIFT",
        metadata: %{
          allowed_paths: ["open-sleigh/lib/open_sleigh/orchestrator.ex"],
          affected_files: ["open-sleigh/lib/open_sleigh/orchestrator.ex"],
          lockset: ["open-sleigh/lib/open_sleigh/orchestrator.ex"],
          scope_hash: String.duplicate("0", 64)
        }
      )

    orchestrator = start_orchestrator!(ctx)

    Orchestrator.submit_candidates(orchestrator, [ticket])
    snapshot = Orchestrator.status(orchestrator)

    assert snapshot.claimed == []
    assert snapshot.running == []

    assert [
             %{
               metric: :dispatch_failed,
               tags: %{ticket: "OCT-SCOPE-DRIFT"},
               value: "scope_hash_mismatch"
             }
           ] = ObservationsBus.snapshot()
  end

  defp start_orchestrator!(ctx) do
    ws_name = String.to_atom("ws_orchestrator_commission_#{unique_suffix()}")
    orch_name = String.to_atom("orch_commission_#{unique_suffix()}")

    {:ok, _store} =
      WorkflowStore.start_link(
        phase_configs: %{frame: frame_phase_config()},
        prompts: %{frame: "Frame {{ticket.id}}"},
        name: ws_name
      )

    judge_fun = JudgeClient.judge_fun(fn _payload -> {:ok, %{}} end, %{})

    {:ok, _orchestrator} =
      Orchestrator.start_link(
        workflow: Workflow.mvp1(),
        tracker_handle: ctx.tracker,
        tracker_adapter: TrackerMock,
        agent_adapter: AgentMock,
        judge_fun: judge_fun,
        haft_invoker: HaftMock.invoke_fun(ctx.haft),
        workspace_root: ctx.workspace_root,
        guard_config: %{forbidden_paths: [], forbidden_remote_substrings: ["open-sleigh"]},
        task_supervisor: OpenSleigh.AgentSupervisor,
        workflow_store: ws_name,
        name: orch_name
      )

    orch_name
  end

  defp frame_phase_config do
    %{
      phase: :frame,
      agent_role: :frame_verifier,
      tools: [:read],
      gates: %{structural: [], semantic: [], human: []},
      prompt_template_key: :frame,
      max_turns: 1,
      default_valid_until_days: 7
    }
    |> PhaseConfig.new()
    |> unwrap!()
  end

  defp ticket!(overrides) do
    %{
      id: "OCT-1",
      source: {:linear, "oct"},
      title: "Commission scope drift",
      body: "",
      state: :in_progress,
      problem_card_ref: "haft-pc-abc",
      target_branch: "feature/commission",
      fetched_at: ~U[2026-04-22 10:00:00Z],
      metadata: %{}
    }
    |> Map.merge(Map.new(overrides))
    |> Ticket.new()
    |> unwrap!()
  end

  defp unique_suffix do
    :erlang.unique_integer([:positive, :monotonic])
  end

  defp unwrap!({:ok, value}), do: value
end
