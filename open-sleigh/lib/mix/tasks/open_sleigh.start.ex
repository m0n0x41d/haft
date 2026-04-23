defmodule Mix.Tasks.OpenSleigh.Start do
  @shortdoc "Start the Open-Sleigh engine"

  @moduledoc """
  Start an Open-Sleigh engine from `sleigh.md`.

      mix open_sleigh.start
      mix open_sleigh.start --path=sleigh.md --mock
      mix open_sleigh.start --path=sleigh.md --mock --once
      mix open_sleigh.start --path=sleigh.md --mock-haft --mock-judge

  Options:

    * `--path` - config file path. Defaults to `sleigh.md`.
    * `--mock` - use in-memory tracker/agent/Haft adapters.
    * `--mock-agent` - use the in-memory agent adapter only.
    * `--mock-haft` - use in-memory Haft while keeping the configured tracker/agent.
    * `--mock-judge` - use the deterministic rule-based judge.
    * `--once` - boot, run one tracker poll, print status, and stop.
      Without this flag the task runs until interrupted.
    * `--once-timeout-ms` - when `--once` is set, wait up to this
      many milliseconds for the orchestrator to become idle before
      printing status. Defaults to 50.
    * `--help` - print this help.
  """

  use Mix.Task

  alias OpenSleigh.Agent
  alias OpenSleigh.CommissionSource.Intake, as: CommissionIntake
  alias OpenSleigh.CommissionSource.Haft, as: HaftCommissionSource
  alias OpenSleigh.CommissionSource.Local, as: LocalCommissionSource
  alias OpenSleigh.Haft.Mock
  alias OpenSleigh.Judge.{AgentInvoker, GoldenSets, RuleBased}
  alias OpenSleigh.Sleigh.{Compiler, Watcher}
  alias OpenSleigh.Tracker

  @default_commission_max_claims 50

  alias OpenSleigh.{
    HaftServer,
    HaftSupervisor,
    HumanGateListener,
    JudgeClient,
    Orchestrator,
    RuntimeLogWriter,
    RuntimeStatusWriter,
    StatusHTTPServer,
    TrackerPoller,
    Workflow,
    WorkflowStore
  }

  @impl true
  def run(args) do
    args
    |> parse_args()
    |> run_parsed()
  end

  @spec parse_args([String.t()]) :: {:help | :run, keyword()}
  defp parse_args(args) do
    {opts, _argv, invalid} =
      OptionParser.parse(
        args,
        switches: [
          path: :string,
          mock: :boolean,
          mock_agent: :boolean,
          mock_haft: :boolean,
          mock_judge: :boolean,
          once: :boolean,
          once_timeout_ms: :integer,
          help: :boolean
        ],
        aliases: [h: :help]
      )

    if invalid == [] do
      parsed_mode(opts)
    else
      Mix.raise("Invalid options: #{inspect(invalid)}")
    end
  end

  @spec parsed_mode(keyword()) :: {:help | :run, keyword()}
  defp parsed_mode(opts) do
    if Keyword.get(opts, :help, false) do
      {:help, opts}
    else
      {:run, opts}
    end
  end

  @spec run_parsed({:help | :run, keyword()}) :: :ok
  defp run_parsed({:help, _opts}) do
    Mix.shell().info(@moduledoc)
  end

  defp run_parsed({:run, opts}) do
    :ok = ensure_application_started()

    opts
    |> boot_runtime()
    |> handle_boot(opts)
  end

  @spec ensure_application_started() :: :ok
  defp ensure_application_started do
    case Application.ensure_all_started(:open_sleigh) do
      {:ok, _apps} -> :ok
      {:error, _reason} -> :ok
    end
  end

  @spec boot_runtime(keyword()) :: {:ok, map()} | {:error, term()}
  defp boot_runtime(opts) do
    with {:ok, bundle} <- compile_config(opts),
         :ok <- configure_agent(bundle, opts),
         {:ok, prestarted_haft} <- maybe_start_pretracker_haft(bundle, opts),
         {:ok, tracker} <- start_tracker(bundle, opts, prestarted_haft),
         {:ok, haft} <- ensure_started_haft(bundle, opts, prestarted_haft),
         {:ok, store} <- start_workflow_store(bundle),
         {:ok, watcher} <- start_watcher(config_path(opts), store),
         {:ok, orchestrator} <- start_orchestrator(bundle, tracker, haft, store, opts),
         {:ok, listener} <- start_human_gate_listener(bundle, tracker, orchestrator),
         {:ok, poller} <- start_tracker_poller(bundle, tracker, orchestrator),
         {:ok, status_writer} <- start_status_writer(bundle, orchestrator, opts),
         {:ok, status_http} <- start_status_http_server(bundle),
         {:ok, log_writer} <- start_log_writer(bundle, opts) do
      {:ok,
       %{
         bundle: bundle,
         tracker: tracker,
         haft: haft,
         store: store,
         watcher: watcher,
         orchestrator: orchestrator,
         listener: listener,
         poller: poller,
         status_writer: status_writer,
         status_http: status_http,
         log_writer: log_writer
       }}
    end
  end

  @spec compile_config(keyword()) :: {:ok, WorkflowStore.bundle()} | {:error, term()}
  defp compile_config(opts) do
    opts
    |> config_path()
    |> File.read()
    |> compile_source()
  end

  @spec compile_source({:ok, binary()} | {:error, term()}) ::
          {:ok, WorkflowStore.bundle()} | {:error, term()}
  defp compile_source({:ok, source}), do: Compiler.compile(source)
  defp compile_source({:error, reason}), do: {:error, {:config_read_failed, reason}}

  @spec maybe_start_pretracker_haft(WorkflowStore.bundle(), keyword()) ::
          {:ok, map() | nil} | {:error, term()}
  defp maybe_start_pretracker_haft(bundle, opts) do
    if commission_source_kind(bundle) == "haft" do
      start_haft(bundle, opts)
    else
      {:ok, nil}
    end
  end

  @spec ensure_started_haft(WorkflowStore.bundle(), keyword(), map() | nil) ::
          {:ok, map()} | {:error, term()}
  defp ensure_started_haft(_bundle, _opts, %{invoke_fun: invoke_fun} = haft)
       when is_function(invoke_fun, 1) do
    {:ok, haft}
  end

  defp ensure_started_haft(bundle, opts, nil), do: start_haft(bundle, opts)

  @spec start_tracker(WorkflowStore.bundle(), keyword(), map() | nil) ::
          {:ok, map()} | {:error, term()}
  defp start_tracker(bundle, opts, haft) do
    opts
    |> Keyword.get(:mock, false)
    |> start_tracker_for_mode(bundle, haft)
  end

  @spec start_tracker_for_mode(boolean(), WorkflowStore.bundle(), map() | nil) ::
          {:ok, map()} | {:error, term()}
  defp start_tracker_for_mode(true, bundle, haft) do
    bundle
    |> commission_source_kind()
    |> start_mock_tracker_for_source(bundle, haft)
  end

  defp start_tracker_for_mode(false, bundle, haft) do
    bundle
    |> commission_source_kind()
    |> start_tracker_for_source(bundle, haft)
  end

  @spec start_mock_tracker_for_source(String.t() | nil, WorkflowStore.bundle(), map() | nil) ::
          {:ok, map()} | {:error, term()}
  defp start_mock_tracker_for_source("local", bundle, _haft),
    do: start_local_commission_tracker(bundle)

  defp start_mock_tracker_for_source("haft", bundle, haft),
    do: start_haft_commission_tracker(bundle, haft)

  defp start_mock_tracker_for_source(_kind, _bundle, _haft), do: start_mock_tracker()

  @spec start_tracker_for_source(String.t() | nil, WorkflowStore.bundle(), map() | nil) ::
          {:ok, map()} | {:error, term()}
  defp start_tracker_for_source("local", bundle, _haft),
    do: start_local_commission_tracker(bundle)

  defp start_tracker_for_source("haft", bundle, haft),
    do: start_haft_commission_tracker(bundle, haft)

  defp start_tracker_for_source(_kind, bundle, _haft), do: start_linear_tracker(bundle)

  @spec start_mock_tracker() :: {:ok, map()} | {:error, term()}
  defp start_mock_tracker do
    case Tracker.Mock.start() do
      {:ok, handle} -> {:ok, %{adapter: Tracker.Mock, handle: handle, pids: [handle]}}
      {:error, reason} -> {:error, reason}
    end
  end

  @spec start_local_commission_tracker(WorkflowStore.bundle()) :: {:ok, map()} | {:error, term()}
  defp start_local_commission_tracker(bundle) do
    with {:ok, source} <- LocalCommissionSource.new(bundle),
         {:ok, handle} <- Tracker.Mock.start(),
         source_ref <-
           commission_source_ref(
             LocalCommissionSource,
             source,
             commission_source_max_claims(bundle),
             false
           ),
         {:ok, _claimed_count} <- CommissionIntake.replenish(source_ref, Tracker.Mock, handle) do
      {:ok,
       %{
         adapter: Tracker.Mock,
         handle: handle,
         pids: [handle],
         commission_source: source_ref
       }}
    end
  end

  @spec start_haft_commission_tracker(WorkflowStore.bundle(), map() | nil) ::
          {:ok, map()} | {:error, term()}
  defp start_haft_commission_tracker(_bundle, nil), do: {:error, :haft_unavailable}

  defp start_haft_commission_tracker(bundle, %{invoke_fun: invoke_fun})
       when is_function(invoke_fun, 1) do
    with {:ok, source} <- HaftCommissionSource.new(bundle, invoke_fun),
         {:ok, handle} <- Tracker.Mock.start(),
         source_ref <-
           commission_source_ref(
             HaftCommissionSource,
             source,
             commission_source_max_claims(bundle),
             true
           ),
         {:ok, _claimed_count} <- CommissionIntake.replenish(source_ref, Tracker.Mock, handle) do
      {:ok,
       %{
         adapter: Tracker.Mock,
         handle: handle,
         pids: [handle],
         commission_source: source_ref
       }}
    end
  end

  @spec commission_source_ref(module(), term(), pos_integer(), boolean()) ::
          CommissionIntake.source_ref()
  defp commission_source_ref(adapter, source, max_claims, dynamic?) do
    CommissionIntake.source_ref(adapter, source, max_claims, dynamic?)
  end

  @spec start_linear_tracker(WorkflowStore.bundle()) :: {:ok, map()} | {:error, term()}
  defp start_linear_tracker(bundle) do
    case System.get_env("LINEAR_API_KEY") do
      nil ->
        {:error, :missing_linear_api_key}

      api_key when api_key != "" ->
        with {:ok, handle} <- linear_handle(api_key, bundle),
             {:ok, finch} <- Finch.start_link(name: handle.finch_name) do
          {:ok, %{adapter: Tracker.Linear, handle: handle, pids: [finch]}}
        end

      _api_key ->
        {:error, :missing_linear_api_key}
    end
  end

  @spec linear_handle(String.t(), WorkflowStore.bundle()) :: {:ok, map()} | {:error, term()}
  defp linear_handle(api_key, bundle) do
    tracker = bundle.tracker
    active_states = value_at(tracker, :active_states, [])
    state_names = configured_state_names(bundle)

    with {:ok, project_slug} <- linear_project_slug(tracker),
         {:ok, active_state_names} <- linear_active_state_names(active_states) do
      {:ok,
       %{
         api_key: api_key,
         project_slug: project_slug,
         active_states: active_state_names,
         endpoint: value_at(tracker, :endpoint, "https://api.linear.app/graphql"),
         finch_name: server_name("open_sleigh_linear_finch"),
         problem_card_ref_field: value_at(tracker, :problem_card_ref_field),
         problem_card_ref_marker: value_at(tracker, :problem_card_ref_marker, "problem_card_ref"),
         state_name_map: state_name_map(state_names),
         state_atom_map: state_atom_map(state_names)
       }}
    end
  end

  @spec configured_state_names(WorkflowStore.bundle()) :: [String.t()]
  defp configured_state_names(bundle) do
    [
      value_at(bundle.tracker, :active_states, []),
      value_at(bundle.tracker, :terminal_states, []),
      Map.get(bundle.external_publication, :tracker_transition_to, [])
    ]
    |> List.flatten()
    |> Enum.map(&config_string/1)
    |> Enum.map(&String.trim/1)
    |> Enum.reject(&(&1 == ""))
    |> Enum.uniq()
  end

  @spec linear_project_slug(map()) :: {:ok, String.t()} | {:error, :missing_linear_project_slug}
  defp linear_project_slug(tracker) do
    tracker
    |> value_at(:project_slug, value_at(tracker, :team, System.get_env("LINEAR_PROJECT_SLUG")))
    |> present_string_result(:missing_linear_project_slug)
  end

  @spec linear_active_state_names(term()) :: {:ok, [String.t()]} | {:error, term()}
  defp linear_active_state_names(states) when is_list(states) do
    states
    |> Enum.map(&config_string/1)
    |> Enum.map(&String.trim/1)
    |> Enum.reject(&(&1 == ""))
    |> active_state_names_result()
  end

  defp linear_active_state_names(_states), do: {:error, :missing_linear_active_states}

  @spec active_state_names_result([String.t()]) :: {:ok, [String.t()]} | {:error, term()}
  defp active_state_names_result([]), do: {:error, :missing_linear_active_states}
  defp active_state_names_result(names), do: {:ok, names}

  @spec state_name_map([String.t()]) :: %{atom() => String.t()}
  defp state_name_map(state_names) do
    state_names
    |> Enum.map(&{state_atom(&1), &1})
    |> Map.new()
  end

  @spec state_atom_map([String.t()]) :: %{String.t() => atom()}
  defp state_atom_map(state_names) do
    state_names
    |> Enum.map(&{&1, state_atom(&1)})
    |> Map.new()
  end

  @spec state_atom(String.t()) :: atom()
  defp state_atom(name) do
    name
    |> String.downcase()
    |> String.replace(~r/[^a-z0-9]+/, "_")
    |> String.trim("_")
    |> String.to_atom()
  end

  @spec start_haft(WorkflowStore.bundle(), keyword()) :: {:ok, map()} | {:error, term()}
  defp start_haft(bundle, opts) do
    if mock_haft?(opts) do
      start_mock_haft()
    else
      start_real_haft(bundle)
    end
  end

  @spec start_mock_haft() :: {:ok, map()} | {:error, term()}
  defp start_mock_haft do
    case Mock.start(problem_cards: :generated, commissions: :generated) do
      {:ok, handle} -> {:ok, %{invoke_fun: Mock.invoke_fun(handle), pids: [handle]}}
      {:error, reason} -> {:error, reason}
    end
  end

  @spec start_real_haft(WorkflowStore.bundle()) :: {:ok, map()} | {:error, term()}
  defp start_real_haft(bundle) do
    command = config_string(value_at(bundle.haft, :command, "haft serve"))
    wal_dir = value_at(bundle.haft, :wal_dir, System.get_env("OPEN_SLEIGH_WAL_DIR"))
    server = server_name("open_sleigh_haft_server")

    case HaftSupervisor.start_link(
           command: command,
           project_root: File.cwd!(),
           wal_dir: wal_dir,
           server_name: server
         ) do
      {:ok, supervisor} -> started_haft_result(supervisor, server)
      {:error, reason} -> {:error, reason}
    end
  end

  @spec started_haft_result(pid(), atom()) :: {:ok, map()} | {:error, term()}
  defp started_haft_result(supervisor, server) do
    case HaftServer.status(server) do
      %{available: true} ->
        {:ok, %{invoke_fun: HaftServer.invoke_fun(server), pids: [supervisor]}}

      _status ->
        GenServer.stop(supervisor)
        {:error, :haft_unavailable}
    end
  end

  @spec start_workflow_store(WorkflowStore.bundle()) :: GenServer.on_start()
  defp start_workflow_store(bundle) do
    WorkflowStore.start_link(
      phase_configs: bundle.phase_configs,
      prompts: bundle.prompts,
      config_hashes: bundle.config_hashes,
      external_publication: bundle.external_publication,
      name: server_name("open_sleigh_workflow_store")
    )
  end

  @spec start_watcher(Path.t(), GenServer.server()) :: GenServer.on_start()
  defp start_watcher(path, store) do
    Watcher.start_link(
      path: path,
      workflow_store: store,
      name: server_name("open_sleigh_sleigh_watcher")
    )
  end

  @spec start_orchestrator(WorkflowStore.bundle(), map(), map(), GenServer.server(), keyword()) ::
          GenServer.on_start()
  defp start_orchestrator(bundle, tracker, haft, store, opts) do
    Orchestrator.start_link(
      workflow: runtime_workflow(bundle),
      tracker_handle: tracker.handle,
      tracker_adapter: tracker.adapter,
      agent_adapter: agent_adapter(bundle, opts),
      external_publication: bundle.external_publication,
      judge_fun: judge_fun(bundle, opts),
      haft_invoker: haft.invoke_fun,
      workspace_root: workspace_root(bundle),
      guard_config: %{forbidden_paths: [], forbidden_remote_substrings: ["open-sleigh"]},
      task_supervisor: OpenSleigh.AgentSupervisor,
      workflow_store: store,
      hooks: hooks(bundle),
      hook_failure_policy: hook_failure_policy(bundle),
      hook_timeout_ms: hook_timeout_ms(bundle),
      active_states: orchestrator_active_states(bundle),
      max_concurrency: runtime_max_concurrency(bundle),
      name: server_name("open_sleigh_orchestrator")
    )
  end

  @spec runtime_workflow(WorkflowStore.bundle()) :: Workflow.t()
  defp runtime_workflow(%{workflow: :mvp1r}), do: Workflow.mvp1r()
  defp runtime_workflow(_bundle), do: Workflow.mvp1()

  @spec judge_fun(WorkflowStore.bundle(), keyword()) :: OpenSleigh.GateChain.judge_fun()
  defp judge_fun(bundle, opts) do
    bundle
    |> judge_invoker(opts)
    |> JudgeClient.judge_fun(GoldenSets.calibration())
  end

  @spec judge_invoker(WorkflowStore.bundle(), keyword()) :: JudgeClient.invoke_fun()
  defp judge_invoker(bundle, opts) do
    opts
    |> mock_judge?()
    |> judge_invoker_for_mode(bundle)
  end

  @spec judge_invoker_for_mode(boolean(), WorkflowStore.bundle()) :: JudgeClient.invoke_fun()
  defp judge_invoker_for_mode(true, _bundle), do: &RuleBased.invoke/1

  defp judge_invoker_for_mode(false, bundle), do: live_judge_invoker(bundle)

  @spec live_judge_invoker(WorkflowStore.bundle()) :: JudgeClient.invoke_fun()
  defp live_judge_invoker(bundle) do
    bundle
    |> live_judge_config()
    |> AgentInvoker.invoke_fun()
  end

  @spec live_judge_config(WorkflowStore.bundle()) :: AgentInvoker.config()
  defp live_judge_config(bundle) do
    AgentInvoker.config(
      judge_adapter(bundle),
      judge_workspace(bundle),
      judge_config_hash(bundle),
      judge_attrs(bundle)
    )
  end

  @spec judge_adapter(WorkflowStore.bundle()) :: module()
  defp judge_adapter(bundle) do
    bundle
    |> judge_attrs()
    |> value_at(:kind, value_at(bundle.agent, :kind, "codex"))
    |> config_string()
    |> adapter_module()
  end

  @spec judge_workspace(WorkflowStore.bundle()) :: Path.t()
  defp judge_workspace(bundle) do
    bundle
    |> workspace_root()
    |> Path.join("_judge")
  end

  @spec judge_config_hash(WorkflowStore.bundle()) :: OpenSleigh.ConfigHash.t()
  defp judge_config_hash(bundle) do
    bundle.config_hashes
    |> Map.values()
    |> Enum.join(":")
    |> OpenSleigh.ConfigHash.from_iodata()
  end

  @spec judge_attrs(WorkflowStore.bundle()) :: map()
  defp judge_attrs(bundle), do: Map.get(bundle, :judge, %{})

  @spec configure_agent(WorkflowStore.bundle(), keyword()) :: :ok
  defp configure_agent(bundle, opts) do
    if mock_agent?(opts) do
      :ok
    else
      configure_real_agent(bundle)
    end
  end

  @spec configure_real_agent(WorkflowStore.bundle()) :: :ok
  defp configure_real_agent(bundle) do
    case agent_adapter(bundle, []) do
      Agent.Codex ->
        Application.put_env(:open_sleigh, Agent.Codex, codex_opts(bundle))
        :ok

      _adapter ->
        :ok
    end
  end

  @spec codex_opts(WorkflowStore.bundle()) :: keyword()
  defp codex_opts(bundle) do
    agent = bundle.agent
    codex = bundle.codex

    [
      command:
        config_string(value_at(codex, :command, value_at(agent, :command, "codex app-server"))),
      read_timeout_ms: positive_integer(value_at(codex, :read_timeout_ms), 5_000),
      turn_timeout_ms: positive_integer(value_at(codex, :turn_timeout_ms), 3_600_000),
      stall_timeout_ms: non_negative_integer(value_at(codex, :stall_timeout_ms), 300_000),
      approval_policy: config_string(value_at(codex, :approval_policy, "never")),
      thread_sandbox: config_string(value_at(codex, :thread_sandbox, "workspace-write")),
      turn_sandbox_policy: value_at(codex, :turn_sandbox_policy, %{"type" => "workspaceWrite"})
    ]
  end

  @spec agent_adapter(WorkflowStore.bundle(), keyword()) :: module()
  defp agent_adapter(bundle, opts) do
    if mock_agent?(opts) do
      Agent.Mock
    else
      real_agent_adapter(bundle)
    end
  end

  @spec real_agent_adapter(WorkflowStore.bundle()) :: module()
  defp real_agent_adapter(bundle) do
    bundle.agent
    |> value_at(:kind, "codex")
    |> config_string()
    |> adapter_module()
  end

  @spec adapter_module(String.t()) :: module()
  defp adapter_module("claude"), do: Agent.Claude
  defp adapter_module("mock"), do: Agent.Mock
  defp adapter_module(_kind), do: Agent.Codex

  @spec start_human_gate_listener(WorkflowStore.bundle(), map(), GenServer.server()) ::
          GenServer.on_start()
  defp start_human_gate_listener(bundle, tracker, orchestrator) do
    HumanGateListener.start_link(
      tracker_handle: tracker.handle,
      tracker_adapter: tracker.adapter,
      orchestrator: orchestrator,
      approvers: Map.get(bundle.external_publication, :approvers, []),
      name: server_name("open_sleigh_human_gate_listener")
    )
  end

  @spec start_tracker_poller(WorkflowStore.bundle(), map(), GenServer.server()) ::
          GenServer.on_start()
  defp start_tracker_poller(bundle, tracker, orchestrator) do
    TrackerPoller.start_link(
      tracker_handle: tracker.handle,
      tracker_adapter: tracker.adapter,
      commission_source: Map.get(tracker, :commission_source),
      orchestrator: orchestrator,
      interval_ms: get_in(bundle.engine, ["poll_interval_ms"]) || 30_000,
      name: server_name("open_sleigh_tracker_poller")
    )
  end

  @spec start_status_writer(WorkflowStore.bundle(), GenServer.server(), keyword()) ::
          GenServer.on_start()
  defp start_status_writer(bundle, orchestrator, opts) do
    RuntimeStatusWriter.start_link(
      orchestrator: orchestrator,
      path: status_path(bundle),
      metadata: runtime_metadata(bundle, opts),
      interval_ms: status_interval_ms(bundle),
      name: server_name("open_sleigh_status_writer")
    )
  end

  @spec start_log_writer(WorkflowStore.bundle(), keyword()) :: GenServer.on_start()
  defp start_log_writer(bundle, opts) do
    RuntimeLogWriter.start_link(
      path: log_path(bundle),
      metadata: runtime_metadata(bundle, opts),
      name: server_name("open_sleigh_log_writer")
    )
  end

  @spec start_status_http_server(WorkflowStore.bundle()) :: GenServer.on_start() | {:ok, nil}
  defp start_status_http_server(bundle) do
    if status_http_enabled?(bundle) do
      StatusHTTPServer.start_link(
        status_path: status_path(bundle),
        host: status_http_host(bundle),
        port: status_http_port(bundle),
        name: server_name("open_sleigh_status_http")
      )
    else
      {:ok, nil}
    end
  end

  @spec handle_boot({:ok, map()} | {:error, term()}, keyword()) :: :ok
  defp handle_boot({:ok, runtime}, opts) do
    Mix.shell().info("Open-Sleigh engine started")
    :ok = emit_status_http_url(runtime.status_http)
    :ok = RuntimeLogWriter.event(runtime.log_writer, :tracker_poll_requested, %{})
    :ok = TrackerPoller.poke(runtime.poller)

    if Keyword.get(opts, :once, false) do
      :ok = wait_once_idle(runtime.orchestrator, once_timeout_ms(opts))
      :ok = RuntimeStatusWriter.write(runtime.status_writer)
      :ok = RuntimeLogWriter.event(runtime.log_writer, :once_poll_completed, %{})

      Mix.shell().info(
        "Open-Sleigh status: #{Jason.encode!(Orchestrator.status(runtime.orchestrator))}"
      )

      cleanup(runtime)
    else
      Process.sleep(:infinity)
    end
  end

  defp handle_boot({:error, reason}, _opts) do
    Mix.raise("Open-Sleigh start failed: #{inspect(reason)}")
  end

  @spec wait_once_idle(GenServer.server(), non_neg_integer()) :: :ok
  defp wait_once_idle(orchestrator, timeout_ms) do
    Process.sleep(50)

    orchestrator
    |> wait_until_idle(System.monotonic_time(:millisecond) + timeout_ms)
  end

  @spec wait_until_idle(GenServer.server(), integer()) :: :ok
  defp wait_until_idle(orchestrator, deadline_ms) do
    if once_idle?(orchestrator) or System.monotonic_time(:millisecond) >= deadline_ms do
      :ok
    else
      Process.sleep(25)
      wait_until_idle(orchestrator, deadline_ms)
    end
  end

  @spec once_idle?(GenServer.server()) :: boolean()
  defp once_idle?(orchestrator) do
    status = Orchestrator.status(orchestrator)

    status.claimed == [] and status.running == [] and status.pending_human == [] and
      status.retries == %{}
  end

  @spec once_timeout_ms(keyword()) :: non_neg_integer()
  defp once_timeout_ms(opts) do
    opts
    |> Keyword.get(:once_timeout_ms, 50)
    |> non_negative_integer(50)
  end

  @spec cleanup(map()) :: :ok
  defp cleanup(runtime) do
    :ok = stop_log_writer(runtime.log_writer)

    [
      runtime.status_writer,
      runtime.status_http,
      runtime.poller,
      runtime.listener,
      runtime.orchestrator,
      runtime.watcher,
      runtime.store
    ]
    |> Enum.filter(&is_pid/1)
    |> Enum.concat(runtime.tracker.pids)
    |> Enum.concat(runtime.haft.pids)
    |> Enum.each(&stop_process/1)

    :ok
  end

  @spec emit_status_http_url(pid() | nil) :: :ok
  defp emit_status_http_url(nil), do: :ok

  defp emit_status_http_url(pid) when is_pid(pid) do
    Mix.shell().info(
      "Open-Sleigh dashboard: http://127.0.0.1:#{StatusHTTPServer.port(pid)}/dashboard"
    )
  end

  @spec stop_log_writer(pid()) :: :ok
  defp stop_log_writer(pid) when is_pid(pid) do
    if Process.alive?(pid) do
      RuntimeLogWriter.stop(pid)
    else
      :ok
    end
  end

  @spec stop_process(pid()) :: :ok
  defp stop_process(pid) when is_pid(pid) do
    if Process.alive?(pid) do
      GenServer.stop(pid)
    else
      :ok
    end
  end

  @spec config_path(keyword()) :: Path.t()
  defp config_path(opts), do: Keyword.get(opts, :path, "sleigh.md")

  @spec workspace_root(WorkflowStore.bundle()) :: Path.t()
  defp workspace_root(bundle) do
    bundle.workspace
    |> value_at(:root, System.get_env("OPEN_SLEIGH_WORKSPACE_ROOT"))
    |> present_string_or("~/.open-sleigh/workspaces")
    |> expand_path()
  end

  @spec status_path(WorkflowStore.bundle()) :: Path.t()
  defp status_path(bundle) do
    bundle.engine
    |> value_at(:status_path, System.get_env("OPEN_SLEIGH_STATUS_PATH"))
    |> present_string_or("~/.open-sleigh/status.json")
    |> expand_path()
  end

  @spec log_path(WorkflowStore.bundle()) :: Path.t()
  defp log_path(bundle) do
    bundle.engine
    |> value_at(:log_path, System.get_env("OPEN_SLEIGH_LOG_PATH"))
    |> present_string_or("~/.open-sleigh/runtime.jsonl")
    |> expand_path()
  end

  @spec status_http_config(WorkflowStore.bundle()) :: map()
  defp status_http_config(bundle) do
    bundle.engine
    |> value_at(:status_http, %{})
  end

  @spec status_http_enabled?(WorkflowStore.bundle()) :: boolean()
  defp status_http_enabled?(bundle) do
    bundle
    |> status_http_config()
    |> value_at(:enabled, false)
    |> truthy?()
  end

  @spec status_http_host(WorkflowStore.bundle()) :: :inet.ip_address()
  defp status_http_host(bundle) do
    bundle
    |> status_http_config()
    |> value_at(:host, "127.0.0.1")
    |> config_string()
    |> to_charlist()
    |> :inet.parse_address()
    |> status_http_host_result()
  end

  @spec status_http_host_result({:ok, :inet.ip_address()} | {:error, term()}) ::
          :inet.ip_address()
  defp status_http_host_result({:ok, host}), do: host
  defp status_http_host_result({:error, _reason}), do: {127, 0, 0, 1}

  @spec status_http_port(WorkflowStore.bundle()) :: :inet.port_number()
  defp status_http_port(bundle) do
    bundle
    |> status_http_config()
    |> value_at(:port, 4767)
    |> non_negative_integer(4767)
  end

  @spec status_interval_ms(WorkflowStore.bundle()) :: non_neg_integer()
  defp status_interval_ms(bundle) do
    bundle.engine
    |> value_at(:status_interval_ms)
    |> non_negative_integer(5_000)
  end

  @spec runtime_metadata(WorkflowStore.bundle(), keyword()) :: map()
  defp runtime_metadata(bundle, opts) do
    %{
      config_path: config_path(opts),
      workspace_root: workspace_root(bundle),
      workspace_cleanup_policy: workspace_cleanup_policy(bundle),
      tracker_kind: status_tracker_kind(bundle, opts),
      agent_kind: status_agent_kind(bundle, opts)
    }
  end

  @spec workspace_cleanup_policy(WorkflowStore.bundle()) :: String.t()
  defp workspace_cleanup_policy(bundle) do
    bundle.workspace
    |> value_at(:cleanup_policy, "keep")
    |> config_string()
  end

  @spec status_tracker_kind(WorkflowStore.bundle(), keyword()) :: String.t()
  defp status_tracker_kind(bundle, opts) do
    cond do
      commission_source_kind(bundle) == "local" ->
        "commission_source:local"

      commission_source_kind(bundle) == "haft" ->
        "commission_source:haft"

      Keyword.get(opts, :mock, false) ->
        "mock"

      true ->
        "linear"
    end
  end

  @spec commission_source_kind(WorkflowStore.bundle()) :: String.t() | nil
  defp commission_source_kind(bundle) do
    bundle
    |> Map.get(:commission_source, %{})
    |> value_at(:kind)
    |> config_string_or_nil()
  end

  @spec status_agent_kind(WorkflowStore.bundle(), keyword()) :: String.t()
  defp status_agent_kind(bundle, opts) do
    if mock_agent?(opts) do
      "mock"
    else
      bundle.agent
      |> value_at(:kind, "codex")
      |> config_string()
    end
  end

  @spec mock_agent?(keyword()) :: boolean()
  defp mock_agent?(opts) do
    Keyword.get(opts, :mock, false) or Keyword.get(opts, :mock_agent, false)
  end

  @spec mock_haft?(keyword()) :: boolean()
  defp mock_haft?(opts) do
    Keyword.get(opts, :mock, false) or Keyword.get(opts, :mock_haft, false)
  end

  @spec mock_judge?(keyword()) :: boolean()
  defp mock_judge?(opts) do
    Keyword.get(opts, :mock, false) or Keyword.get(opts, :mock_judge, false)
  end

  @spec hooks(WorkflowStore.bundle()) :: %{optional(atom()) => String.t()}
  defp hooks(bundle) do
    bundle.hooks
    |> Map.take(["after_create", "before_run", "after_run"])
    |> Enum.map(&hook_pair/1)
    |> Enum.reject(&is_nil/1)
    |> Map.new()
  end

  @spec hook_pair({String.t(), term()}) :: {atom(), String.t()} | nil
  defp hook_pair({"after_create", script}) when is_binary(script), do: {:after_create, script}
  defp hook_pair({"before_run", script}) when is_binary(script), do: {:before_run, script}
  defp hook_pair({"after_run", script}) when is_binary(script), do: {:after_run, script}
  defp hook_pair(_pair), do: nil

  @spec hook_failure_policy(WorkflowStore.bundle()) :: %{optional(atom()) => atom()}
  defp hook_failure_policy(bundle) do
    bundle.hooks
    |> value_at(:failure_policy, %{})
    |> hook_failure_policy_map()
  end

  @spec hook_failure_policy_map(term()) :: %{optional(atom()) => atom()}
  defp hook_failure_policy_map(%{} = policy) do
    [:after_create, :before_run, :after_run]
    |> Enum.map(&{&1, hook_failure_policy_value(policy, &1)})
    |> Map.new()
  end

  defp hook_failure_policy_map(_policy), do: hook_failure_policy_map(%{})

  @spec hook_failure_policy_value(map(), atom()) :: atom()
  defp hook_failure_policy_value(policy, hook_name) do
    default = default_hook_failure_policy(hook_name)

    policy
    |> value_at(hook_name, default)
    |> hook_failure_policy_atom(default)
  end

  @spec default_hook_failure_policy(atom()) :: atom()
  defp default_hook_failure_policy(:after_run), do: :warning
  defp default_hook_failure_policy(_hook_name), do: :blocking

  @spec hook_failure_policy_atom(term(), atom()) :: atom()
  defp hook_failure_policy_atom(value, default) do
    value
    |> config_string()
    |> hook_failure_policy_atom_from_string(default)
  end

  @spec hook_failure_policy_atom_from_string(String.t(), atom()) :: atom()
  defp hook_failure_policy_atom_from_string("blocking", _default), do: :blocking
  defp hook_failure_policy_atom_from_string("warning", _default), do: :warning
  defp hook_failure_policy_atom_from_string("ignore", _default), do: :ignore
  defp hook_failure_policy_atom_from_string(_value, default), do: default

  @spec hook_timeout_ms(WorkflowStore.bundle()) :: pos_integer()
  defp hook_timeout_ms(bundle) do
    bundle.hooks
    |> value_at(:timeout_ms)
    |> positive_integer(60_000)
  end

  @spec runtime_max_concurrency(WorkflowStore.bundle()) :: pos_integer()
  defp runtime_max_concurrency(bundle) do
    fallback =
      bundle.agent
      |> value_at(:max_concurrent_agents)
      |> positive_integer(1)

    bundle.engine
    |> value_at(:concurrency)
    |> positive_integer(fallback)
  end

  @spec commission_source_max_claims(WorkflowStore.bundle()) :: pos_integer()
  defp commission_source_max_claims(bundle) do
    fallback = Kernel.max(runtime_max_concurrency(bundle), @default_commission_max_claims)

    bundle.commission_source
    |> value_at(:max_claims)
    |> positive_integer(fallback)
  end

  @spec orchestrator_active_states(WorkflowStore.bundle()) :: [atom()]
  defp orchestrator_active_states(bundle) do
    bundle.tracker
    |> value_at(:active_states, [])
    |> active_state_atoms()
  end

  @spec active_state_atoms(term()) :: [atom()]
  defp active_state_atoms(states) when is_list(states) do
    states
    |> Enum.map(&config_string/1)
    |> Enum.map(&String.trim/1)
    |> Enum.reject(&(&1 == ""))
    |> Enum.map(&state_atom/1)
  end

  defp active_state_atoms(_states), do: [:todo, :in_progress]

  @spec present_string_result(term(), term()) :: {:ok, String.t()} | {:error, term()}
  defp present_string_result(value, error) do
    value
    |> present_string()
    |> present_string_result_value(error)
  end

  @spec present_string_result_value(String.t() | nil, term()) ::
          {:ok, String.t()} | {:error, term()}
  defp present_string_result_value(nil, error), do: {:error, error}
  defp present_string_result_value(value, _error), do: {:ok, value}

  @spec present_string_or(term(), String.t()) :: String.t()
  defp present_string_or(value, fallback) do
    value
    |> present_string()
    |> present_string_or_value(fallback)
  end

  @spec present_string_or_value(String.t() | nil, String.t()) :: String.t()
  defp present_string_or_value(nil, fallback), do: fallback
  defp present_string_or_value(value, _fallback), do: value

  @spec present_string(term()) :: String.t() | nil
  defp present_string(value) when is_binary(value) do
    value
    |> String.trim()
    |> blank_to_nil()
  end

  defp present_string(_value), do: nil

  @spec blank_to_nil(String.t()) :: String.t() | nil
  defp blank_to_nil(""), do: nil
  defp blank_to_nil(value), do: value

  @spec positive_integer(term(), pos_integer()) :: pos_integer()
  defp positive_integer(value, _fallback) when is_integer(value) and value > 0, do: value
  defp positive_integer(_value, fallback), do: fallback

  @spec non_negative_integer(term(), non_neg_integer()) :: non_neg_integer()
  defp non_negative_integer(value, _fallback) when is_integer(value) and value >= 0, do: value
  defp non_negative_integer(_value, fallback), do: fallback

  @spec truthy?(term()) :: boolean()
  defp truthy?(true), do: true
  defp truthy?("true"), do: true
  defp truthy?("yes"), do: true
  defp truthy?("1"), do: true
  defp truthy?(_value), do: false

  @spec value_at(term(), atom(), term()) :: term()
  defp value_at(%{} = map, key, fallback) do
    Map.get(map, Atom.to_string(key), Map.get(map, key, fallback))
  end

  defp value_at(_value, _key, fallback), do: fallback

  @spec value_at(term(), atom()) :: term()
  defp value_at(value, key), do: value_at(value, key, nil)

  @spec config_string(term()) :: String.t()
  defp config_string(value) when is_binary(value), do: value
  defp config_string(value) when is_atom(value), do: Atom.to_string(value)
  defp config_string(value), do: to_string(value)

  @spec config_string_or_nil(term()) :: String.t() | nil
  defp config_string_or_nil(nil), do: nil
  defp config_string_or_nil(value), do: config_string(value)

  @spec expand_path(String.t()) :: Path.t()
  defp expand_path("~/" <> rest) do
    System.user_home!()
    |> Path.join(rest)
    |> Path.expand()
  end

  defp expand_path(path), do: Path.expand(path)

  @spec server_name(String.t()) :: atom()
  defp server_name(prefix) do
    String.to_atom("#{prefix}_#{System.unique_integer([:positive, :monotonic])}")
  end
end
