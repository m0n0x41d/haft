defmodule OpenSleigh.Canary do
  @moduledoc """
  Local executable canary for the MVP-1 runtime pipeline.

  This module runs the currently executable mock-backed T3 path:
  Frame -> Execute -> HumanGate approval -> Measure -> terminal. The
  full T1/T1'/T2 gate-regression suite still needs real upstream Haft
  fixtures and calibrated semantic judge responses.
  """

  alias OpenSleigh.Agent.Mock, as: AgentMock
  alias OpenSleigh.Haft.Mock, as: HaftMock
  alias OpenSleigh.Tracker.Mock, as: TrackerMock

  alias OpenSleigh.{
    HumanGateListener,
    JudgeClient,
    ObservationsBus,
    Orchestrator,
    Workflow,
    WorkflowStore
  }

  alias OpenSleigh.Sleigh.Compiler

  @ticket_id "CANARY-T3"
  @approver "ivan@example.com"
  @timeout_ms 2_000

  @type summary :: %{
          required(:ticket_id) => String.t(),
          required(:haft_artifacts) => non_neg_integer(),
          required(:coverage) => [atom()]
        }

  @doc "Run the mock-backed T3 canary once."
  @spec run(keyword()) :: {:ok, summary()} | {:error, atom() | [term()]}
  def run(_opts) do
    :ok = ensure_application_started()
    :ok = ObservationsBus.reset()

    with {:ok, bundle} <- Compiler.compile(canary_source()),
         {:ok, ctx} <- start_runtime(bundle),
         {:ok, summary} <- run_ticket(ctx) do
      cleanup(ctx)
      {:ok, summary}
    end
  end

  @spec ensure_application_started() :: :ok
  defp ensure_application_started do
    case Application.ensure_all_started(:open_sleigh) do
      {:ok, _apps} -> :ok
      {:error, _reason} -> :ok
    end
  end

  @spec start_runtime(WorkflowStore.bundle()) :: {:ok, map()} | {:error, atom()}
  defp start_runtime(bundle) do
    workspace_root = tmp_workspace()

    with :ok <- File.mkdir_p(workspace_root),
         {:ok, haft} <- HaftMock.start(),
         {:ok, tracker} <- TrackerMock.start(),
         :ok <- seed_tracker(tracker),
         {:ok, store} <- start_workflow_store(bundle),
         {:ok, orchestrator} <- start_orchestrator(store, tracker, haft, workspace_root),
         {:ok, listener} <- start_human_gate_listener(tracker, orchestrator) do
      {:ok,
       %{
         workspace_root: workspace_root,
         haft: haft,
         tracker: tracker,
         store: store,
         orchestrator: orchestrator,
         listener: listener
       }}
    end
  end

  @spec seed_tracker(pid()) :: :ok | {:error, atom()}
  defp seed_tracker(tracker) do
    TrackerMock.seed(tracker, [
      %{
        id: @ticket_id,
        source: {:linear, "canary"},
        title: "Publish canary rate limiter",
        body: "Add rate limiter to MyApp.Api.Auth per RFC-XYZ",
        state: :in_progress,
        problem_card_ref: "haft-pc-canary-t3",
        fetched_at: DateTime.utc_now(),
        target_branch: "main"
      }
    ])
  end

  @spec start_workflow_store(WorkflowStore.bundle()) :: GenServer.on_start()
  defp start_workflow_store(bundle) do
    WorkflowStore.start_link(
      phase_configs: bundle.phase_configs,
      prompts: bundle.prompts,
      config_hashes: bundle.config_hashes,
      external_publication: bundle.external_publication,
      name: server_name("canary_ws")
    )
  end

  @spec start_orchestrator(pid(), pid(), pid(), Path.t()) :: GenServer.on_start()
  defp start_orchestrator(store, tracker, haft, workspace_root) do
    Orchestrator.start_link(
      workflow: Workflow.mvp1(),
      tracker_handle: tracker,
      tracker_adapter: TrackerMock,
      agent_adapter: AgentMock,
      judge_fun: JudgeClient.judge_fun(fn _prompt -> {:ok, %{}} end, %{}),
      haft_invoker: HaftMock.invoke_fun(haft),
      workspace_root: workspace_root,
      guard_config: %{forbidden_paths: [], forbidden_remote_substrings: ["open-sleigh"]},
      task_supervisor: OpenSleigh.AgentSupervisor,
      workflow_store: store,
      name: server_name("canary_orch")
    )
  end

  @spec start_human_gate_listener(pid(), GenServer.server()) :: GenServer.on_start()
  defp start_human_gate_listener(tracker, orchestrator) do
    HumanGateListener.start_link(
      tracker_handle: tracker,
      tracker_adapter: TrackerMock,
      orchestrator: orchestrator,
      approvers: [@approver],
      poll_interval_ms: 0,
      name: server_name("canary_human_gate")
    )
  end

  @spec run_ticket(map()) :: {:ok, summary()} | {:error, atom()}
  defp run_ticket(ctx) do
    with {:ok, tickets} <- TrackerMock.list_active(ctx.tracker),
         :ok <- submit_tickets(ctx.orchestrator, tickets),
         :ok <- wait_for_human_gate(ctx.orchestrator),
         :ok <- approve_ticket(ctx),
         :ok <- wait_for_terminal(ctx.orchestrator),
         {:ok, artifacts} <- canary_artifacts(ctx.haft) do
      {:ok,
       %{
         ticket_id: @ticket_id,
         haft_artifacts: length(artifacts),
         coverage: [:t3_human_gate]
       }}
    end
  end

  @spec submit_tickets(GenServer.server(), [OpenSleigh.Ticket.t()]) :: :ok
  defp submit_tickets(orchestrator, tickets) do
    Orchestrator.submit_candidates(orchestrator, tickets)
  end

  @spec wait_for_human_gate(GenServer.server()) :: :ok | {:error, :timeout}
  defp wait_for_human_gate(orchestrator) do
    wait_until(
      fn -> Orchestrator.status(orchestrator).pending_human != [] end,
      @timeout_ms
    )
  end

  @spec approve_ticket(map()) :: :ok | {:error, atom()}
  defp approve_ticket(ctx) do
    with :ok <- TrackerMock.add_comment(ctx.tracker, @ticket_id, @approver, "/approve canary"),
         :ok <- HumanGateListener.poke(ctx.listener) do
      :ok
    end
  end

  @spec wait_for_terminal(GenServer.server()) :: :ok | {:error, :timeout}
  defp wait_for_terminal(orchestrator) do
    wait_until(
      fn ->
        status = Orchestrator.status(orchestrator)
        status.claimed == [] and status.running == [] and status.pending_human == []
      end,
      @timeout_ms
    )
  end

  @spec canary_artifacts(pid()) :: {:ok, [map()]} | {:error, atom()}
  defp canary_artifacts(haft) do
    artifacts = HaftMock.artifacts(haft)

    if length(artifacts) == 3 do
      {:ok, artifacts}
    else
      {:error, :unexpected_artifact_count}
    end
  end

  @spec wait_until((-> boolean()), non_neg_integer()) :: :ok | {:error, :timeout}
  defp wait_until(check_fun, timeout_ms) do
    deadline = System.monotonic_time(:millisecond) + timeout_ms
    do_wait_until(check_fun, deadline)
  end

  @spec do_wait_until((-> boolean()), integer()) :: :ok | {:error, :timeout}
  defp do_wait_until(check_fun, deadline) do
    cond do
      check_fun.() ->
        :ok

      System.monotonic_time(:millisecond) > deadline ->
        {:error, :timeout}

      true ->
        Process.sleep(10)
        do_wait_until(check_fun, deadline)
    end
  end

  @spec cleanup(map()) :: :ok
  defp cleanup(ctx) do
    [
      ctx.listener,
      ctx.orchestrator,
      ctx.store,
      ctx.tracker,
      ctx.haft
    ]
    |> Enum.each(&stop_process/1)

    File.rm_rf!(ctx.workspace_root)
    :ok
  end

  @spec stop_process(pid()) :: :ok
  defp stop_process(pid) when is_pid(pid) do
    if Process.alive?(pid) do
      GenServer.stop(pid)
    else
      :ok
    end
  end

  @spec tmp_workspace() :: Path.t()
  defp tmp_workspace do
    System.tmp_dir!()
    |> Path.join("open_sleigh_canary_#{System.unique_integer([:positive, :monotonic])}")
  end

  @spec server_name(String.t()) :: atom()
  defp server_name(prefix) do
    String.to_atom("#{prefix}_#{System.unique_integer([:positive, :monotonic])}")
  end

  @spec canary_source() :: String.t()
  defp canary_source do
    """
    ---
    engine:
      poll_interval_ms: 30000
      concurrency: 1

    tracker:
      kind: mock
      team: CANARY
      active_states: [In Progress]
      terminal_states: [Done]

    agent:
      kind: mock
      version_pin: test
      command: mock
      max_turns: 20
      max_tokens_per_turn: 80000
      wall_clock_timeout_s: 600

    haft:
      command: mock
      version: test

    external_publication:
      branch_regex: "^(main|master)$"
      tracker_transition_to: ["Done"]
      approvers: ["#{@approver}"]
      timeout_h: 24

    phases:
      frame:
        agent_role: frame_verifier
        tools: [haft_query, read, grep]
        gates:
          structural: []
          semantic: []
      execute:
        agent_role: executor
        tools: [read, write, bash]
        gates:
          structural: []
          semantic: []
          human: [commission_approved]
      measure:
        agent_role: measurer
        tools: [haft_decision, haft_refresh]
        gates:
          structural: []
          semantic: []
    ---

    # Prompt templates

    ## Frame
    Verify upstream framing for {{ticket.title}}.

    ## Execute
    Implement the bounded canary change for {{ticket.title}} and keep evidence external.

    ## Measure
    Measure the canary result for {{ticket.title}}.
    """
  end
end
