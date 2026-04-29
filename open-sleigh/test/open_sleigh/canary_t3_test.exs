defmodule OpenSleigh.CanaryT3Test do
  @moduledoc """
  End-to-end T3 canary with HumanGate firing on Execute -> Measure.
  """

  use ExUnit.Case, async: false

  alias OpenSleigh.Agent.Mock, as: AgentMock
  alias OpenSleigh.Haft.Mock, as: HaftMock
  alias OpenSleigh.Tracker.Mock, as: TrackerMock

  alias OpenSleigh.{
    HumanGateListener,
    JudgeClient,
    ObservationsBus,
    Orchestrator,
    PhaseConfig,
    Workflow,
    WorkflowStore
  }

  setup do
    ObservationsBus.reset()

    workspace_root =
      System.tmp_dir!()
      |> Path.join("canary_t3_#{:erlang.unique_integer([:positive, :monotonic])}")

    File.mkdir_p!(workspace_root)
    on_exit(fn -> File.rm_rf!(workspace_root) end)

    {:ok, haft} = HaftMock.start()
    {:ok, tracker} = TrackerMock.start()

    :ok =
      TrackerMock.seed(tracker, [
        %{
          id: "OCT-T3-HG",
          source: {:linear, "oct"},
          title: "Publish rate limiter",
          body: "Add rate limiter to MyApp.Api.Auth per RFC-XYZ",
          state: :in_progress,
          problem_card_ref: "haft-pc-t3",
          fetched_at: ~U[2026-04-22 10:00:00Z],
          target_branch: "main"
        }
      ])

    frame_cfg = phase_config(:frame, :frame_verifier, [:haft_query, :read, :grep], [])

    execute_cfg = phase_config(:execute, :executor, [:read, :write, :bash], [])

    measure_cfg = phase_config(:measure, :measurer, [:haft_decision, :haft_refresh], [])

    suffix = :erlang.unique_integer([:positive])
    ws_name = String.to_atom("ws_t3_#{suffix}")
    orch_name = String.to_atom("orch_t3_#{suffix}")

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

    judge_fun = JudgeClient.judge_fun(fn _p -> {:ok, %{}} end, %{})

    {:ok, _} =
      Orchestrator.start_link(
        workflow: Workflow.mvp1(),
        tracker_handle: tracker,
        tracker_adapter: TrackerMock,
        agent_adapter: AgentMock,
        external_publication: %{branch_regex: "^(main|master)$"},
        judge_fun: judge_fun,
        haft_invoker: HaftMock.invoke_fun(haft),
        workspace_root: workspace_root,
        guard_config: %{forbidden_paths: [], forbidden_remote_substrings: ["open-sleigh"]},
        task_supervisor: OpenSleigh.AgentSupervisor,
        workflow_store: ws_name,
        name: orch_name
      )

    {:ok, listener} =
      HumanGateListener.start_link(
        tracker_handle: tracker,
        tracker_adapter: TrackerMock,
        orchestrator: orch_name,
        approvers: ["ivan@example.com"],
        poll_interval_ms: 0
      )

    %{
      orchestrator: orch_name,
      listener: listener,
      tracker: tracker,
      haft: haft
    }
  end

  test "T3 execute waits for /approve before advancing to measure", ctx do
    {:ok, tickets} = TrackerMock.list_active(ctx.tracker)
    Orchestrator.submit_candidates(ctx.orchestrator, tickets)

    assert :ok =
             wait_until(
               fn -> Orchestrator.status(ctx.orchestrator).pending_human != [] end,
               2_000
             )

    comments = TrackerMock.comments(ctx.tracker, "OCT-T3-HG")
    assert Enum.any?(comments, &String.contains?(&1, "Open-Sleigh HumanGate"))

    :ok = TrackerMock.add_comment(ctx.tracker, "OCT-T3-HG", "ivan@example.com", "/approve LGTM")
    HumanGateListener.poke(ctx.listener)

    assert :ok = wait_for_terminal(ctx.orchestrator, 2_000)

    snapshot = Orchestrator.status(ctx.orchestrator)
    assert snapshot.claimed == []
    assert snapshot.running == []
    assert snapshot.pending_human == []

    assert length(HaftMock.artifacts(ctx.haft)) == 3
  end

  defp phase_config(phase, role, tools, human_gates) do
    max_turns = if phase == :execute, do: 20, else: 1

    PhaseConfig.new(%{
      phase: phase,
      agent_role: role,
      tools: tools,
      gates: %{structural: [], semantic: [], human: human_gates},
      prompt_template_key: phase,
      max_turns: max_turns,
      default_valid_until_days: 30
    })
    |> unwrap!()
  end

  defp wait_for_terminal(orchestrator, timeout_ms) do
    wait_until(
      fn ->
        status = Orchestrator.status(orchestrator)
        status.claimed == [] and status.running == [] and status.pending_human == []
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

  defp unwrap!({:ok, value}), do: value
end
