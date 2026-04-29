defmodule OpenSleigh.OrchestratorRetryTest do
  use ExUnit.Case, async: false

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

  defmodule StallAgent do
    @behaviour OpenSleigh.Agent.Adapter

    @impl true
    def adapter_kind, do: :stall_test
    @impl true
    def tool_registry, do: []
    @impl true
    def start_session(_session), do: {:ok, %{}}
    @impl true
    def send_turn(_handle, _prompt, _session), do: {:error, :stall_timeout}
    @impl true
    def dispatch_tool(_handle, _tool, _args, _session), do: {:error, :tool_unknown_to_adapter}
    @impl true
    def close_session(_handle), do: :ok
  end

  defmodule StartFailureAgent do
    @behaviour OpenSleigh.Agent.Adapter

    @impl true
    def adapter_kind, do: :start_failure_test
    @impl true
    def tool_registry, do: []
    @impl true
    def start_session(_session), do: {:error, :thread_start_failed}
    @impl true
    def send_turn(_handle, _prompt, _session), do: {:error, :turn_start_failed}
    @impl true
    def dispatch_tool(_handle, _tool, _args, _session), do: {:error, :tool_unknown_to_adapter}
    @impl true
    def close_session(_handle), do: :ok
  end

  defmodule NormalExitAgent do
    @behaviour OpenSleigh.Agent.Adapter

    @impl true
    def adapter_kind, do: :normal_exit_test
    @impl true
    def tool_registry, do: []
    @impl true
    def start_session(_session), do: {:ok, %{}}
    @impl true
    def send_turn(_handle, _prompt, _session), do: exit(:normal)
    @impl true
    def dispatch_tool(_handle, _tool, _args, _session), do: {:error, :tool_unknown_to_adapter}
    @impl true
    def close_session(_handle), do: :ok
  end

  defmodule CrashAgent do
    @behaviour OpenSleigh.Agent.Adapter

    @impl true
    def adapter_kind, do: :crash_test
    @impl true
    def tool_registry, do: []
    @impl true
    def start_session(_session), do: {:ok, %{}}
    @impl true
    def send_turn(_handle, _prompt, _session), do: exit(:boom)
    @impl true
    def dispatch_tool(_handle, _tool, _args, _session), do: {:error, :tool_unknown_to_adapter}
    @impl true
    def close_session(_handle), do: :ok
  end

  setup do
    ObservationsBus.reset()

    workspace_root =
      System.tmp_dir!()
      |> Path.join("orchestrator_retry_#{:erlang.unique_integer([:positive, :monotonic])}")

    File.mkdir_p!(workspace_root)
    on_exit(fn -> File.rm_rf!(workspace_root) end)

    {:ok, haft} = HaftMock.start()
    {:ok, tracker} = TrackerMock.start()

    :ok =
      TrackerMock.seed(tracker, [
        %{
          id: "OCT-RETRY",
          source: {:linear, "oct"},
          title: "Retry me",
          body: "",
          state: :in_progress,
          problem_card_ref: "haft-pc-retry",
          fetched_at: ~U[2026-04-22 10:00:00Z],
          target_branch: "feature/retry"
        }
      ])

    %{tracker: tracker, haft: haft, workspace_root: workspace_root}
  end

  test "worker error schedules retry with exponential backoff", ctx do
    orchestrator = start_orchestrator!(ctx, StallAgent, base_retry_backoff_ms: 200)

    submit_retry_ticket(ctx.tracker, orchestrator)

    assert :ok = wait_until(fn -> retry_attempt(orchestrator, "OCT-RETRY") == 1 end, 500)
    retry = Orchestrator.status(orchestrator).retries["OCT-RETRY"]
    assert retry.delay_ms == 200
    assert retry.error == :stalled
  end

  test "adapter start failure posts tracker comment and retry-visible status", ctx do
    orchestrator =
      start_orchestrator!(
        ctx,
        StartFailureAgent,
        base_retry_backoff_ms: 1_000,
        max_retry_backoff_ms: 1_000
      )

    submit_retry_ticket(ctx.tracker, orchestrator)

    assert :ok = wait_until(fn -> retry_attempt(orchestrator, "OCT-RETRY") == 1 end, 500)

    assert [comment] = TrackerMock.comments(ctx.tracker, "OCT-RETRY")
    assert String.contains?(comment, "Open-Sleigh session failed for this ticket.")
    assert String.contains?(comment, "Phase: `frame`")
    assert String.contains?(comment, "Reason: `thread_start_failed`")
    assert String.contains?(comment, "Retry: attempt 1 scheduled in 1000ms.")
    assert String.contains?(comment, "open-sleigh:session-failed:frame:thread_start_failed")

    status = Orchestrator.status(orchestrator)
    assert status.retries["OCT-RETRY"].error == :thread_start_failed
  end

  test "retry timer re-dispatches active tickets and increments attempt", ctx do
    orchestrator = start_orchestrator!(ctx, StallAgent, base_retry_backoff_ms: 50)

    submit_retry_ticket(ctx.tracker, orchestrator)

    assert :ok = wait_until(fn -> retry_attempt(orchestrator, "OCT-RETRY") >= 2 end, 1_000)
  end

  test "retry timer releases a non-active ticket", ctx do
    orchestrator = start_orchestrator!(ctx, StallAgent, base_retry_backoff_ms: 50)

    submit_retry_ticket(ctx.tracker, orchestrator)
    assert :ok = wait_until(fn -> retry_attempt(orchestrator, "OCT-RETRY") == 1 end, 500)
    :ok = TrackerMock.transition(ctx.tracker, "OCT-RETRY", :done)

    assert :ok =
             wait_until(
               fn ->
                 status = Orchestrator.status(orchestrator)
                 status.retries == %{} and status.claimed == []
               end,
               1_000
             )
  end

  test "normal worker exit schedules short continuation retry", ctx do
    orchestrator = start_orchestrator!(ctx, NormalExitAgent, normal_exit_retry_ms: 200)

    submit_retry_ticket(ctx.tracker, orchestrator)

    Process.sleep(50)
    assert retry_attempt(orchestrator, "OCT-RETRY") == 0
    assert :ok = wait_until(fn -> retry_attempt(orchestrator, "OCT-RETRY") >= 1 end, 500)
  end

  test "abnormal worker exit schedules retry", ctx do
    orchestrator = start_orchestrator!(ctx, CrashAgent)

    submit_retry_ticket(ctx.tracker, orchestrator)

    assert :ok = wait_until(fn -> retry_attempt(orchestrator, "OCT-RETRY") == 1 end, 500)
    retry = Orchestrator.status(orchestrator).retries["OCT-RETRY"]
    assert match?({:worker_down, _reason}, retry.error)
  end

  defp start_orchestrator!(ctx, agent_adapter) do
    start_orchestrator!(ctx, agent_adapter, [])
  end

  defp start_orchestrator!(ctx, agent_adapter, retry_opts) do
    suffix = :erlang.unique_integer([:positive])
    ws_name = String.to_atom("ws_retry_#{suffix}")
    orch_name = String.to_atom("orch_retry_#{suffix}")

    {:ok, _store} =
      WorkflowStore.start_link(
        phase_configs: %{
          frame: phase_config(:frame, :frame_verifier),
          execute: phase_config(:execute, :executor),
          measure: phase_config(:measure, :measurer)
        },
        prompts: %{frame: "Frame", execute: "Execute", measure: "Measure"},
        name: ws_name
      )

    judge_fun = JudgeClient.judge_fun(fn _p -> {:ok, %{}} end, %{})

    {:ok, _orchestrator} =
      Orchestrator.start_link(
        workflow: Workflow.mvp1(),
        tracker_handle: ctx.tracker,
        tracker_adapter: TrackerMock,
        agent_adapter: agent_adapter,
        judge_fun: judge_fun,
        haft_invoker: HaftMock.invoke_fun(ctx.haft),
        workspace_root: ctx.workspace_root,
        guard_config: %{forbidden_paths: [], forbidden_remote_substrings: ["open-sleigh"]},
        task_supervisor: OpenSleigh.AgentSupervisor,
        workflow_store: ws_name,
        base_retry_backoff_ms: Keyword.get(retry_opts, :base_retry_backoff_ms, 10),
        max_retry_backoff_ms: Keyword.get(retry_opts, :max_retry_backoff_ms, 400),
        normal_exit_retry_ms: Keyword.get(retry_opts, :normal_exit_retry_ms, 5),
        name: orch_name
      )

    orch_name
  end

  defp phase_config(phase, role) do
    max_turns = if phase == :execute, do: 20, else: 1

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

  defp submit_retry_ticket(tracker, orchestrator) do
    {:ok, tickets} = TrackerMock.list_active(tracker)
    Orchestrator.submit_candidates(orchestrator, tickets)
  end

  defp retry_attempt(orchestrator, ticket_id) do
    status = Orchestrator.status(orchestrator)

    status.retry_attempts
    |> Map.get(ticket_id)
    |> fallback_retry_attempt(status.retries, ticket_id)
  end

  defp fallback_retry_attempt(nil, retries, ticket_id) do
    retries
    |> Map.get(ticket_id, %{attempt: 0})
    |> Map.fetch!(:attempt)
  end

  defp fallback_retry_attempt(attempt, _retries, _ticket_id), do: attempt

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
