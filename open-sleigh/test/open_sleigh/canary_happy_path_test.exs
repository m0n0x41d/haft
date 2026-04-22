defmodule OpenSleigh.CanaryHappyPathTest do
  @moduledoc """
  End-to-end skeleton — the first failing/passing integration test
  per `IMPLEMENTATION_PLAN.md §Canary suite`.

  Wires the full L5 pipeline with L4 mocks:

      TrackerPoller(Mock tracker) →
        Orchestrator(single-writer) →
          AgentWorker(Mock agent) → GateChain → PhaseOutcome →
            Haft.Client(Mock) → Orchestrator(next phase) →
              ... Frame → Execute → Measure → terminal

  The Mock agent always returns `:completed`, so every phase passes
  single-turn. This validates the full dispatch/outcome/next-phase
  loop without requiring real Codex / Linear / haft serve.

  Canary T1 / T1' / T2 (gate-activation negative tests) land when
  adapter-side prompts are wired; this module is the skeleton green
  path.
  """

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

  setup do
    ObservationsBus.reset()

    # Workspace dir for agent to (not really) write to.
    workspace_root =
      Path.join(
        System.tmp_dir!(),
        "canary_happy_#{:erlang.unique_integer([:positive, :monotonic])}"
      )

    File.mkdir_p!(workspace_root)
    on_exit(fn -> File.rm_rf!(workspace_root) end)

    # ——— L4 wiring via mocks ———
    {:ok, haft} = HaftMock.start()
    {:ok, tracker} = TrackerMock.start()

    :ok =
      TrackerMock.seed(tracker, [
        %{
          id: "OCT-T3",
          source: {:linear, "oct"},
          title: "Add rate limiter",
          body: "Add rate limiter to MyApp.Api.Auth per RFC-XYZ",
          state: :in_progress,
          problem_card_ref: "haft-pc-t3",
          fetched_at: ~U[2026-04-22 10:00:00Z],
          target_branch: "feature/rate-limiter"
        }
      ])

    # ——— Phase configs (no human gate on execute for skeleton test;
    # canary T3 with HumanGate lands when HumanGateListener wires) ———
    frame_cfg =
      PhaseConfig.new(%{
        phase: :frame,
        agent_role: :frame_verifier,
        tools: [:haft_query, :read, :grep],
        gates: %{structural: [], semantic: [], human: []},
        prompt_template_key: :frame,
        max_turns: 1,
        default_valid_until_days: 7
      })
      |> unwrap!()

    execute_cfg =
      PhaseConfig.new(%{
        phase: :execute,
        agent_role: :executor,
        tools: [:read, :write, :bash, :haft_note],
        gates: %{structural: [], semantic: [], human: []},
        prompt_template_key: :execute,
        max_turns: 20,
        default_valid_until_days: 30
      })
      |> unwrap!()

    measure_cfg =
      PhaseConfig.new(%{
        phase: :measure,
        agent_role: :measurer,
        tools: [:haft_decision, :haft_refresh],
        gates: %{structural: [], semantic: [], human: []},
        prompt_template_key: :measure,
        max_turns: 1,
        default_valid_until_days: 30
      })
      |> unwrap!()

    # ——— L5 process tree (each under a unique name so async/ordered
    # test runs don't collide) ———
    suffix = :erlang.unique_integer([:positive])
    ws_name = String.to_atom("ws_#{suffix}")
    orch_name = String.to_atom("orch_#{suffix}")

    {:ok, _} =
      WorkflowStore.start_link(
        phase_configs: %{frame: frame_cfg, execute: execute_cfg, measure: measure_cfg},
        prompts: %{
          frame: "Verify upstream framing",
          execute: "Implement the change",
          measure: "Produce evidence"
        },
        external_publication: %{branch_regex: "^(main|master)$"},
        name: ws_name
      )

    haft_invoker = HaftMock.invoke_fun(haft)

    # Judge calibration — all MVP-1 semantic gates calibrated.
    judge_fun = JudgeClient.judge_fun(fn _p -> {:ok, %{}} end, %{})

    {:ok, _} =
      Orchestrator.start_link(
        workflow: Workflow.mvp1(),
        tracker_handle: tracker,
        tracker_adapter: TrackerMock,
        agent_adapter: AgentMock,
        external_publication: %{tracker_transition_to: ["Done"]},
        judge_fun: judge_fun,
        haft_invoker: haft_invoker,
        workspace_root: workspace_root,
        guard_config: %{forbidden_paths: [], forbidden_remote_substrings: ["open-sleigh"]},
        task_supervisor: OpenSleigh.AgentSupervisor,
        workflow_store: ws_name,
        name: orch_name
      )

    %{
      orchestrator: orch_name,
      tracker: tracker,
      haft: haft,
      workspace_root: workspace_root
    }
  end

  test "T3 skeleton — tracker ticket advances frame → execute → measure → terminal", ctx do
    # Submit the tracker's ticket to the Orchestrator.
    {:ok, tickets} = TrackerMock.list_active(ctx.tracker)
    Orchestrator.submit_candidates(ctx.orchestrator, tickets)

    # Wait for the three phases + terminal to complete. The mock
    # agent + all-empty gate lists makes the pipeline fast.
    :ok = wait_for_terminal(ctx.orchestrator, 2_000)

    # After terminal, nothing should still be claimed.
    snapshot = Orchestrator.status(ctx.orchestrator)
    assert snapshot.claimed == []
    assert snapshot.running == []

    # Haft received three write_artifact calls (frame + execute + measure).
    artifacts = HaftMock.artifacts(ctx.haft)
    assert length(artifacts) == 3

    # Observations include a :session_terminal entry.
    terminal_obs = Enum.filter(ObservationsBus.snapshot(), &(&1.metric == :session_terminal))
    assert terminal_obs != []

    {:ok, ticket} = TrackerMock.get(ctx.tracker, "OCT-T3")
    assert ticket.state == :done

    comments = TrackerMock.comments(ctx.tracker, "OCT-T3")
    assert Enum.any?(comments, &String.contains?(&1, "transitioned this ticket to `Done`"))
  end

  # ——— helpers ———

  defp wait_for_terminal(orchestrator, timeout_ms) do
    wait_until(
      fn ->
        status = Orchestrator.status(orchestrator)
        status.claimed == [] and status.running == []
      end,
      timeout_ms
    )
  end

  defp wait_until(check_fun, timeout_ms) do
    deadline = System.monotonic_time(:millisecond) + timeout_ms
    do_wait(check_fun, deadline)
  end

  defp do_wait(check_fun, deadline) do
    cond do
      check_fun.() ->
        :ok

      System.monotonic_time(:millisecond) > deadline ->
        {:error, :timeout}

      true ->
        Process.sleep(10)
        do_wait(check_fun, deadline)
    end
  end

  defp unwrap!({:ok, v}), do: v
end
