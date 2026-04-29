defmodule OpenSleigh.OrchestratorDispatchFailureTest do
  use ExUnit.Case, async: false

  alias OpenSleigh.Agent.Mock, as: AgentMock
  alias OpenSleigh.Haft.Mock, as: HaftMock
  alias OpenSleigh.Tracker.Mock, as: TrackerMock

  alias OpenSleigh.{
    JudgeClient,
    ObservationsBus,
    Orchestrator,
    PhaseConfig,
    RuntimeStatusWriter,
    Workflow,
    WorkflowStore
  }

  setup do
    ObservationsBus.reset()

    workspace_root =
      System.tmp_dir!()
      |> Path.join("orchestrator_dispatch_failure_#{unique_suffix()}")

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

  test "missing upstream ProblemCard posts one actionable tracker comment", ctx do
    :ok = seed_ticket(ctx.tracker, "OCT-NO-FRAME", "haft-missing-frame")

    orchestrator =
      start_orchestrator!(
        ctx,
        HaftMock.invoke_fun(ctx.haft)
      )

    status_path = Path.join(ctx.workspace_root, "status.json")

    {:ok, status_writer} =
      RuntimeStatusWriter.start_link(
        orchestrator: orchestrator,
        path: status_path,
        metadata: %{config_path: "test/sleigh.md"},
        interval_ms: 0
      )

    :ok = submit_and_sync(ctx.tracker, orchestrator)
    :ok = submit_and_sync(ctx.tracker, orchestrator)
    :ok = RuntimeStatusWriter.write(status_writer)

    assert [comment] = TrackerMock.comments(ctx.tracker, "OCT-NO-FRAME")
    assert String.contains?(comment, "Open-Sleigh could not dispatch this ticket.")
    assert String.contains?(comment, "Reason: `no_upstream_frame`")
    assert String.contains?(comment, "ProblemCard ref: `haft-missing-frame`")
    assert String.contains?(comment, "Create or link an upstream Haft ProblemCard")
    assert String.contains?(comment, "open-sleigh:dispatch-failed:no_upstream_frame")

    snapshot = Orchestrator.status(orchestrator)
    assert snapshot.claimed == []
    assert snapshot.running == []

    assert {:ok, status} =
             status_path
             |> File.read!()
             |> Jason.decode()

    assert [
             %{
               "metric" => "dispatch_failed",
               "reason" => "no_upstream_frame",
               "ticket" => "OCT-NO-FRAME"
             }
           ] = status["failures"]
  end

  test "self-authored upstream ProblemCard is rejected with replacement guidance", ctx do
    :ok = seed_ticket(ctx.tracker, "OCT-SELF-FRAME", "haft-self-frame")

    orchestrator =
      start_orchestrator!(
        ctx,
        problem_card_invoker("haft-self-frame", "open_sleigh_self")
      )

    :ok = submit_and_sync(ctx.tracker, orchestrator)

    assert [comment] = TrackerMock.comments(ctx.tracker, "OCT-SELF-FRAME")
    assert String.contains?(comment, "Reason: `upstream_self_authored`")
    assert String.contains?(comment, "Replace the ProblemCard")
    assert String.contains?(comment, "open-sleigh:dispatch-failed:upstream_self_authored")

    snapshot = Orchestrator.status(orchestrator)
    assert snapshot.claimed == []
    assert snapshot.running == []
  end

  defp start_orchestrator!(ctx, haft_invoker) do
    ws_name = String.to_atom("ws_dispatch_failure_#{unique_suffix()}")
    orch_name = String.to_atom("orch_dispatch_failure_#{unique_suffix()}")

    {:ok, _store} =
      WorkflowStore.start_link(
        phase_configs: %{frame: frame_phase_config()},
        prompts: %{frame: "Verify {{problem_card.description}}"},
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
        haft_invoker: haft_invoker,
        workspace_root: ctx.workspace_root,
        guard_config: %{forbidden_paths: [], forbidden_remote_substrings: ["open-sleigh"]},
        task_supervisor: OpenSleigh.AgentSupervisor,
        workflow_store: ws_name,
        name: orch_name
      )

    orch_name
  end

  defp frame_phase_config do
    PhaseConfig.new(%{
      phase: :frame,
      agent_role: :frame_verifier,
      tools: [:haft_query, :read],
      gates: %{structural: [:problem_card_ref_present], semantic: [], human: []},
      prompt_template_key: :frame,
      max_turns: 1,
      default_valid_until_days: 7
    })
    |> unwrap!()
  end

  defp seed_ticket(tracker, ticket_id, problem_card_ref) do
    TrackerMock.seed(tracker, [
      %{
        id: ticket_id,
        source: {:linear, "oct"},
        title: "Dispatch failure",
        body: "Needs upstream frame",
        state: :in_progress,
        problem_card_ref: problem_card_ref,
        fetched_at: ~U[2026-04-22 10:00:00Z],
        target_branch: "feature/dispatch-failure"
      }
    ])
  end

  defp submit_and_sync(tracker, orchestrator) do
    {:ok, tickets} = TrackerMock.list_active(tracker)
    Orchestrator.submit_candidates(orchestrator, tickets)
    _snapshot = Orchestrator.status(orchestrator)
    :ok
  end

  defp problem_card_invoker(problem_card_ref, authoring_source) do
    fn request_line ->
      {:ok, %{"id" => id}} = Jason.decode(request_line)

      response =
        %{
          "jsonrpc" => "2.0",
          "id" => id,
          "result" => %{
            "problem_card" => %{
              "id" => problem_card_ref,
              "describedEntity" => "lib/open_sleigh/orchestrator.ex",
              "groundingHolon" => "OpenSleigh.Orchestrator",
              "description" => "Self-authored frame should be rejected",
              "authoring_source" => authoring_source
            }
          }
        }
        |> Jason.encode!()
        |> Kernel.<>("\n")

      {:ok, response}
    end
  end

  defp unique_suffix do
    :erlang.unique_integer([:positive, :monotonic])
  end

  defp unwrap!({:ok, value}), do: value
end
