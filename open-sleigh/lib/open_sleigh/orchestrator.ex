defmodule OpenSleigh.Orchestrator do
  @moduledoc """
  The single-writer for session state (SE1).

  Per `specs/enabling-system/FUNCTIONAL_ARCHITECTURE.md` L5 +
  `REFERENCE_ALGORITHMS.md §2 & §5`:

  * Receives candidate tickets via `{:candidates, tickets}` casts
    from `TrackerPoller`.
  * Exclusive ownership of `claimed` (set of ticket ids) and
    `running` (map of ticket_id → session bundle). No module
    outside this GenServer mutates them (SE1).
  * Spawns `AgentWorker` Tasks under a `Task.Supervisor` for
    claimed tickets.
  * Receives `{:outcome, session_id, phase_outcome}` casts from
    workers; dispatches via `PhaseMachine.next/2`; spawns the next
    phase's worker or releases the claim on terminal.

  **MVP-1 simplifications explicitly accepted here** (v0.7 skeleton —
  all lands before canary):

  * No retry queue — failures release the claim; TrackerPoller
    picks the ticket up next tick. Full exponential-backoff retry
    lands with first canary run.
  * No tracker-wins reconciliation mid-session — first tick picks
    up state changes. Full mid-run cancellation per SPEC §10.3
    lands before octacore_nova tickets.
  * No stall-timer — the Mock adapter never stalls. Real timer
    wires when Codex adapter lands.
  """

  use GenServer

  alias OpenSleigh.{
    AdapterSession,
    AgentWorker,
    ObservationsBus,
    Haft.Client,
    HumanGateApproval,
    PhaseMachine,
    Session,
    SessionId,
    Ticket,
    Workflow,
    WorkflowState,
    WorkflowStore
  }

  alias OpenSleigh.Gates.Human.CommissionApproved

  require Logger

  @default_base_retry_backoff_ms 10_000
  @default_max_retry_backoff_ms 300_000
  @default_normal_exit_retry_ms 1_000

  # ——— public API ———

  @typedoc "Startup options."
  @type opts :: [
          workflow: Workflow.t(),
          tracker_handle: term(),
          tracker_adapter: module(),
          agent_adapter: module(),
          judge_fun: OpenSleigh.GateChain.judge_fun(),
          haft_invoker: (binary() -> {:ok, binary()} | {:error, atom()}),
          workspace_root: Path.t(),
          guard_config: map(),
          task_supervisor: pid() | atom(),
          workflow_store: GenServer.server(),
          external_publication: map(),
          hooks: %{optional(atom()) => String.t()},
          hook_failure_policy: %{optional(atom()) => atom()},
          hook_timeout_ms: pos_integer(),
          active_states: [atom()],
          base_retry_backoff_ms: pos_integer(),
          max_retry_backoff_ms: pos_integer(),
          normal_exit_retry_ms: pos_integer(),
          name: atom(),
          now_fun: (-> DateTime.t()),
          now_ms_fun: (-> integer())
        ]

  @spec start_link(opts()) :: GenServer.on_start()
  def start_link(opts) do
    GenServer.start_link(__MODULE__, opts, name: Keyword.get(opts, :name, __MODULE__))
  end

  @doc "Submit a batch of tracker-fetched candidate tickets for dispatch."
  @spec submit_candidates(GenServer.server(), [Ticket.t()]) :: :ok
  def submit_candidates(server \\ __MODULE__, tickets) when is_list(tickets) do
    GenServer.cast(server, {:candidates, tickets})
  end

  @doc "Introspect current orchestrator state (test / HTTP API)."
  @spec status(GenServer.server()) :: map()
  def status(server \\ __MODULE__) do
    GenServer.call(server, :status)
  end

  @doc "Return pending HumanGate entries for a listener."
  @spec pending_human_gates(GenServer.server()) :: [map()]
  def pending_human_gates(server) do
    GenServer.call(server, :pending_human_gates)
  end

  # ——— GenServer ———

  @impl true
  def init(opts) do
    state = %{
      workflow: Keyword.fetch!(opts, :workflow),
      tracker_handle: Keyword.fetch!(opts, :tracker_handle),
      tracker_adapter: Keyword.fetch!(opts, :tracker_adapter),
      agent_adapter: Keyword.fetch!(opts, :agent_adapter),
      judge_fun: Keyword.fetch!(opts, :judge_fun),
      haft_invoker: Keyword.fetch!(opts, :haft_invoker),
      workspace_root: Keyword.fetch!(opts, :workspace_root),
      guard_config: Keyword.fetch!(opts, :guard_config),
      task_supervisor: Keyword.fetch!(opts, :task_supervisor),
      workflow_store: Keyword.get(opts, :workflow_store, WorkflowStore),
      external_publication: Keyword.get(opts, :external_publication, %{}),
      hooks: Keyword.get(opts, :hooks, %{}),
      hook_failure_policy: Keyword.get(opts, :hook_failure_policy, %{}),
      hook_timeout_ms: Keyword.get(opts, :hook_timeout_ms, 60_000),
      active_states: Keyword.get(opts, :active_states, [:todo, :in_progress]),
      now_fun: Keyword.get(opts, :now_fun, &DateTime.utc_now/0),
      now_ms_fun: Keyword.get(opts, :now_ms_fun, fn -> System.monotonic_time(:millisecond) end),
      base_retry_backoff_ms:
        Keyword.get(opts, :base_retry_backoff_ms, @default_base_retry_backoff_ms),
      max_retry_backoff_ms:
        Keyword.get(opts, :max_retry_backoff_ms, @default_max_retry_backoff_ms),
      normal_exit_retry_ms:
        Keyword.get(opts, :normal_exit_retry_ms, @default_normal_exit_retry_ms),
      claimed: MapSet.new(),
      running: %{},
      pending_human: %{},
      retries: %{},
      retry_attempts: %{}
    }

    {:ok, state}
  end

  @impl true
  def handle_call(:status, _from, state) do
    snapshot = %{
      claimed: MapSet.to_list(state.claimed),
      running: Map.keys(state.running),
      pending_human: Map.keys(state.pending_human),
      retries: status_retries(state.retries),
      retry_attempts: state.retry_attempts
    }

    {:reply, snapshot, state}
  end

  def handle_call(:pending_human_gates, _from, state) do
    pending =
      state.pending_human
      |> Map.values()
      |> Enum.map(&pending_listener_view/1)

    {:reply, pending, state}
  end

  @impl true
  def handle_cast({:candidates, tickets}, state) do
    new_state = Enum.reduce(tickets, state, &maybe_dispatch/2)
    {:noreply, new_state}
  end

  def handle_cast({:outcome, session_id, phase_outcome}, state) do
    new_state = handle_outcome(state, session_id, phase_outcome)
    {:noreply, new_state}
  end

  def handle_cast({:error, session_id, reason}, state) do
    :ok =
      ObservationsBus.emit(
        :session_errored,
        reason_text(reason),
        session_error_tags(state, session_id)
      )

    new_state = schedule_retry(state, session_id, normalize_worker_error(reason))
    {:noreply, new_state}
  end

  def handle_cast({:await_human, session_id, outcome_attrs}, state) do
    new_state = park_human_gate(state, session_id, outcome_attrs)
    {:noreply, new_state}
  end

  def handle_cast({:human_approval, ticket_id, %HumanGateApproval{} = approval}, state) do
    new_state = resume_human_gate(state, ticket_id, approval)
    {:noreply, new_state}
  end

  def handle_cast({:human_rejection, ticket_id, reason}, state) do
    new_state = reject_human_gate(state, ticket_id, reason)
    {:noreply, new_state}
  end

  def handle_cast({:human_timeout, ticket_id}, state) do
    new_state = timeout_human_gate(state, ticket_id)
    {:noreply, new_state}
  end

  @impl true
  def handle_info({:DOWN, ref, :process, _pid, reason}, state) do
    new_state =
      state
      |> find_running_by_monitor(ref)
      |> handle_worker_down(state, reason)

    {:noreply, new_state}
  end

  def handle_info({:retry_timer_fired, ticket_id, token}, state) do
    new_state = handle_retry_timer(state, ticket_id, token)
    {:noreply, new_state}
  end

  def handle_info({:normal_worker_down, session_id}, state) do
    new_state = schedule_retry(state, session_id, :worker_down_normal, 0)
    {:noreply, new_state}
  end

  # ——— claim / dispatch ———

  @spec maybe_dispatch(Ticket.t(), map()) :: map()
  defp maybe_dispatch(%Ticket{} = ticket, state) do
    if MapSet.member?(state.claimed, ticket.id) do
      state
    else
      dispatch(ticket, state, state.workflow.entry_phase)
    end
  end

  @spec dispatch(Ticket.t(), map(), atom()) :: map()
  defp dispatch(%Ticket{} = ticket, state, phase) do
    case build_session(ticket, state, phase) do
      {:ok, session, phase_config, prompt, upstream_problem_card} ->
        start_worker(state, session, phase_config, prompt, upstream_problem_card, ticket)

      {:error, reason} ->
        :ok = ObservationsBus.emit(:dispatch_failed, Atom.to_string(reason), %{ticket: ticket.id})
        :ok = post_dispatch_failure_comment(state, ticket, reason)
        state
    end
  end

  @spec post_dispatch_failure_comment(map(), Ticket.t(), atom()) :: :ok
  defp post_dispatch_failure_comment(state, %Ticket{} = ticket, reason) when is_atom(reason) do
    if dispatch_failure_comment_posted?(state, ticket.id, reason) do
      :ok
    else
      body = dispatch_failure_comment_body(ticket, reason)
      _ = state.tracker_adapter.post_comment(state.tracker_handle, ticket.id, body)
      :ok
    end
  end

  @spec dispatch_failure_comment_posted?(map(), String.t(), atom()) :: boolean()
  defp dispatch_failure_comment_posted?(state, ticket_id, reason) do
    marker = dispatch_failure_marker(reason)

    tracker_comment_marker_posted?(state, ticket_id, marker)
  end

  @spec tracker_comment_marker_posted?(map(), String.t(), String.t()) :: boolean()
  defp tracker_comment_marker_posted?(state, ticket_id, marker) do
    case state.tracker_adapter.list_comments(state.tracker_handle, ticket_id) do
      {:ok, comments} ->
        comments
        |> Enum.map(&Map.get(&1, :body, Map.get(&1, "body", "")))
        |> Enum.any?(&String.contains?(&1, marker))

      {:error, _reason} ->
        false
    end
  end

  @spec dispatch_failure_comment_body(Ticket.t(), atom()) :: String.t()
  defp dispatch_failure_comment_body(%Ticket{} = ticket, reason) do
    [
      "Open-Sleigh could not dispatch this ticket.",
      "",
      "Reason: `#{reason}`",
      "ProblemCard ref: `#{ticket.problem_card_ref}`",
      "Action: #{dispatch_failure_action(reason)}",
      "Retry: Open-Sleigh will retry on the next poll after the ticket or runtime config is fixed.",
      "Marker: `#{dispatch_failure_marker(reason)}`"
    ]
    |> Enum.join("\n")
  end

  @spec dispatch_failure_action(atom()) :: String.t()
  defp dispatch_failure_action(:no_upstream_frame) do
    "Create or link an upstream Haft ProblemCard, then update this ticket's ProblemCard reference."
  end

  defp dispatch_failure_action(:upstream_self_authored) do
    "Replace the ProblemCard with one authored outside Open-Sleigh. Open-Sleigh will not use self-authored framing artifacts."
  end

  defp dispatch_failure_action(:unknown_phase) do
    "Fix the workflow configuration so the requested phase exists."
  end

  defp dispatch_failure_action(:unknown_prompt) do
    "Fix the workflow configuration so the requested phase has a prompt template."
  end

  defp dispatch_failure_action(_reason) do
    "Fix the reported runtime or configuration issue, then let the next poll retry this ticket."
  end

  @spec dispatch_failure_marker(atom()) :: String.t()
  defp dispatch_failure_marker(reason) do
    "open-sleigh:dispatch-failed:#{reason}"
  end

  @spec session_error_tags(map(), String.t()) :: map()
  defp session_error_tags(state, session_id) do
    case Map.fetch(state.running, session_id) do
      {:ok, entry} ->
        %{
          session_id: session_id,
          ticket: entry.ticket.id,
          phase: entry.session.phase
        }

      :error ->
        %{session_id: session_id}
    end
  end

  @spec build_session(Ticket.t(), map(), atom()) ::
          {:ok, Session.t(), OpenSleigh.PhaseConfig.t(), String.t(), map() | nil}
          | {:error, atom()}
  defp build_session(ticket, state, phase) do
    with {:ok, phase_config} <- WorkflowStore.phase_config(state.workflow_store, phase),
         phase_config <- effective_phase_config(ticket, state, phase, phase_config),
         {:ok, prompt_template} <- WorkflowStore.prompt_for(state.workflow_store, phase),
         session_id <- SessionId.generate(),
         config_hash <- config_hash_for_phase(state, phase),
         workspace_path <- default_workspace_path(state, ticket),
         {:ok, adapter_session} <-
           build_adapter_session(session_id, state, phase_config, workspace_path, config_hash),
         upstream_problem_card <-
           upstream_problem_card(
             ticket,
             adapter_session,
             state,
             phase,
             phase_config,
             prompt_template
           ),
         :ok <- validate_upstream_problem_card(phase, phase_config, upstream_problem_card),
         prompt <- render_prompt(prompt_template, ticket, upstream_problem_card),
         {:ok, session} <-
           Session.new(%{
             id: session_id,
             ticket: ticket,
             phase: phase,
             config_hash: config_hash,
             scoped_tools: adapter_session.scoped_tools,
             workspace_path: workspace_path,
             claimed_at: state.now_fun.(),
             adapter_session: adapter_session
           }) do
      {:ok, session, phase_config, prompt, upstream_problem_card}
    end
  end

  @spec default_workspace_path(map(), Ticket.t()) :: Path.t()
  defp default_workspace_path(state, ticket) do
    Path.join(state.workspace_root, ticket.id)
  end

  @spec build_adapter_session(
          SessionId.t(),
          map(),
          OpenSleigh.PhaseConfig.t(),
          Path.t(),
          OpenSleigh.ConfigHash.t()
        ) ::
          {:ok, AdapterSession.t()} | {:error, atom()}
  defp build_adapter_session(session_id, state, phase_config, workspace_path, config_hash) do
    AdapterSession.new(%{
      session_id: session_id,
      config_hash: config_hash,
      scoped_tools: MapSet.new(phase_config.tools),
      workspace_path: workspace_path,
      adapter_kind: state.agent_adapter.adapter_kind(),
      adapter_version: "mvp1",
      max_turns: phase_config.max_turns,
      max_tokens_per_turn: 80_000,
      wall_clock_timeout_s: 600
    })
  end

  @spec effective_phase_config(
          Ticket.t(),
          map(),
          atom(),
          OpenSleigh.PhaseConfig.t()
        ) :: OpenSleigh.PhaseConfig.t()
  defp effective_phase_config(%Ticket{} = ticket, state, phase, phase_config) do
    if CommissionApproved.fires?(phase, ticket, state.external_publication) do
      add_human_gate(phase_config, :commission_approved)
    else
      phase_config
    end
  end

  @spec add_human_gate(OpenSleigh.PhaseConfig.t(), atom()) :: OpenSleigh.PhaseConfig.t()
  defp add_human_gate(phase_config, gate) do
    if gate in phase_config.gates.human do
      phase_config
    else
      gates = Map.update!(phase_config.gates, :human, &(&1 ++ [gate]))
      %{phase_config | gates: gates}
    end
  end

  @spec upstream_problem_card(
          Ticket.t(),
          AdapterSession.t(),
          map(),
          atom(),
          OpenSleigh.PhaseConfig.t(),
          String.t()
        ) :: map() | nil
  defp upstream_problem_card(
         %Ticket{} = ticket,
         %AdapterSession{} = adapter_session,
         state,
         phase,
         phase_config,
         prompt_template
       ) do
    if requires_upstream_problem_card?(phase, phase_config, prompt_template) do
      fetch_upstream_problem_card(ticket, adapter_session, state)
    else
      nil
    end
  end

  @spec fetch_upstream_problem_card(Ticket.t(), AdapterSession.t(), map()) :: map() | nil
  defp fetch_upstream_problem_card(%Ticket{} = ticket, %AdapterSession{} = adapter_session, state) do
    case Client.fetch_problem_card(adapter_session, ticket.problem_card_ref, state.haft_invoker) do
      {:ok, card} -> card
      {:error, _reason} -> nil
    end
  end

  @spec requires_upstream_problem_card?(atom(), OpenSleigh.PhaseConfig.t(), String.t()) ::
          boolean()
  defp requires_upstream_problem_card?(:frame, phase_config, prompt_template) do
    Enum.any?([
      :problem_card_ref_present in phase_config.gates.structural,
      :described_entity_field_present in phase_config.gates.structural,
      String.contains?(prompt_template, "problem_card.")
    ])
  end

  defp requires_upstream_problem_card?(_phase, _phase_config, prompt_template) do
    String.contains?(prompt_template, "problem_card.")
  end

  @spec validate_upstream_problem_card(atom(), OpenSleigh.PhaseConfig.t(), map() | nil) ::
          :ok | {:error, atom()}
  defp validate_upstream_problem_card(:frame, phase_config, nil) do
    if :problem_card_ref_present in phase_config.gates.structural do
      {:error, :no_upstream_frame}
    else
      :ok
    end
  end

  defp validate_upstream_problem_card(:frame, _phase_config, %{
         authoring_source: :open_sleigh_self
       }) do
    {:error, :upstream_self_authored}
  end

  defp validate_upstream_problem_card(_phase, _phase_config, _upstream_problem_card), do: :ok

  @spec render_prompt(String.t(), Ticket.t(), map() | nil) :: String.t()
  defp render_prompt(template, %Ticket{} = ticket, upstream_problem_card) do
    replacements = prompt_replacements(ticket, upstream_problem_card)

    Regex.replace(~r/{{\s*([^}]+?)\s*}}/, template, fn _match, key ->
      Map.get(replacements, String.trim(key), "")
    end)
  end

  @spec prompt_replacements(Ticket.t(), map() | nil) :: %{String.t() => String.t()}
  defp prompt_replacements(%Ticket{} = ticket, upstream_problem_card) do
    %{}
    |> Map.merge(ticket_prompt_replacements(ticket))
    |> Map.merge(problem_card_prompt_replacements(upstream_problem_card))
    |> Map.merge(runtime_prompt_replacements(ticket))
  end

  @spec ticket_prompt_replacements(Ticket.t()) :: %{String.t() => String.t()}
  defp ticket_prompt_replacements(%Ticket{} = ticket) do
    %{
      "ticket.id" => ticket.id,
      "ticket.title" => ticket.title,
      "ticket.body" => ticket.body,
      "ticket.problem_card_ref" => ticket.problem_card_ref,
      "ticket.target_branch" => ticket.target_branch || ""
    }
  end

  @spec problem_card_prompt_replacements(map() | nil) :: %{String.t() => String.t()}
  defp problem_card_prompt_replacements(nil) do
    %{
      "problem_card.id" => "",
      "problem_card.ref" => "",
      "problem_card.title" => "",
      "problem_card.body" => "",
      "problem_card.description" => ""
    }
  end

  defp problem_card_prompt_replacements(%{} = problem_card) do
    %{
      "problem_card.id" => problem_card_value(problem_card, "id"),
      "problem_card.ref" => problem_card_value(problem_card, "ref"),
      "problem_card.title" => problem_card_value(problem_card, "title"),
      "problem_card.body" => problem_card_value(problem_card, "body"),
      "problem_card.description" => problem_card_value(problem_card, "description")
    }
  end

  @spec runtime_prompt_replacements(Ticket.t()) :: %{String.t() => String.t()}
  defp runtime_prompt_replacements(%Ticket{} = ticket) do
    metadata = ticket.metadata

    %{
      "pr.url" => metadata_value(metadata, :pr_url),
      "pr.sha" => metadata_value(metadata, :pr_sha),
      "ci.run_id" => metadata_value(metadata, :ci_run_id),
      "ci.status" => metadata_value(metadata, :ci_status)
    }
  end

  @spec problem_card_value(map(), String.t()) :: String.t()
  defp problem_card_value(problem_card, key) do
    atom_key = String.to_existing_atom(key)

    problem_card
    |> Map.get(key, Map.get(problem_card, atom_key, ""))
    |> string_value()
  rescue
    ArgumentError ->
      problem_card
      |> Map.get(key, "")
      |> string_value()
  end

  @spec metadata_value(map(), atom()) :: String.t()
  defp metadata_value(metadata, key) do
    string_key = Atom.to_string(key)

    metadata
    |> Map.get(key, Map.get(metadata, string_key, ""))
    |> string_value()
  end

  @spec string_value(term()) :: String.t()
  defp string_value(nil), do: ""
  defp string_value(value) when is_binary(value), do: value
  defp string_value(value), do: to_string(value)

  @spec config_hash_for_phase(map(), atom()) :: OpenSleigh.ConfigHash.t()
  defp config_hash_for_phase(state, phase) do
    case WorkflowStore.config_hash_for(state.workflow_store, phase) do
      {:ok, hash} ->
        hash

      {:error, :unknown_phase} ->
        state.guard_config |> elem_config_hash() |> fallback_config_hash()
    end
  end

  @spec elem_config_hash(map()) :: OpenSleigh.ConfigHash.t() | nil
  defp elem_config_hash(%{config_hash: hash}), do: hash
  defp elem_config_hash(_), do: nil

  @spec fallback_config_hash(OpenSleigh.ConfigHash.t() | nil) :: OpenSleigh.ConfigHash.t()
  defp fallback_config_hash(nil), do: OpenSleigh.ConfigHash.from_iodata("mvp1-bootstrap")
  defp fallback_config_hash(hash), do: hash

  @spec start_worker(
          map(),
          Session.t(),
          OpenSleigh.PhaseConfig.t(),
          String.t(),
          map() | nil,
          Ticket.t()
        ) :: map()
  defp start_worker(state, session, phase_config, prompt, upstream_problem_card, ticket) do
    ctx = %{
      session: session,
      phase_config: phase_config,
      prompt: prompt,
      upstream_problem_card: upstream_problem_card,
      agent_adapter: state.agent_adapter,
      judge_fun: state.judge_fun,
      haft_invoker: state.haft_invoker,
      orchestrator: self(),
      workspace_root: state.workspace_root,
      guard_config: state.guard_config,
      hooks: state.hooks,
      hook_failure_policy: state.hook_failure_policy,
      hook_timeout_ms: state.hook_timeout_ms,
      now_fun: state.now_fun,
      tracker_handle: state.tracker_handle,
      tracker_adapter: state.tracker_adapter,
      active_states: state.active_states
    }

    {:ok, pid} =
      Task.Supervisor.start_child(state.task_supervisor, fn -> AgentWorker.run(ctx) end)

    monitor_ref = Process.monitor(pid)
    workflow_state = WorkflowState.start(state.workflow)

    %{
      state
      | claimed: MapSet.put(state.claimed, ticket.id),
        running:
          Map.put(state.running, session.id, %{
            ticket: ticket,
            session: session,
            workflow_state: workflow_state,
            task_pid: pid,
            monitor_ref: monitor_ref
          })
    }
  end

  # ——— outcome handling ———

  @spec handle_outcome(map(), String.t(), OpenSleigh.PhaseOutcome.t()) :: map()
  defp handle_outcome(state, session_id, outcome) do
    case Map.fetch(state.running, session_id) do
      {:ok, entry} -> decide_next(state, session_id, entry, outcome)
      :error -> state
    end
  end

  @spec park_human_gate(map(), String.t(), map()) :: map()
  defp park_human_gate(state, session_id, outcome_attrs) do
    case Map.fetch(state.running, session_id) do
      {:ok, entry} -> do_park_human_gate(state, session_id, entry, outcome_attrs)
      :error -> state
    end
  end

  @spec do_park_human_gate(map(), String.t(), map(), map()) :: map()
  defp do_park_human_gate(state, session_id, entry, outcome_attrs) do
    gate_name =
      outcome_attrs
      |> Map.fetch!(:phase_config)
      |> human_gate_name()

    pending = %{
      ticket_id: entry.ticket.id,
      session_id: session_id,
      gate_name: gate_name,
      config_hash: entry.session.config_hash,
      requested_at: state.now_fun.(),
      outcome_attrs: outcome_attrs
    }

    :ok = post_human_gate_request(state, entry)

    :ok =
      ObservationsBus.emit(:human_gate_pending, Atom.to_string(gate_name), %{
        ticket: entry.ticket.id
      })

    %{state | pending_human: Map.put(state.pending_human, entry.ticket.id, pending)}
  end

  @spec post_human_gate_request(map(), map()) :: :ok
  defp post_human_gate_request(state, entry) do
    body = CommissionApproved.render_request(entry.session.phase, entry.ticket)

    _ = state.tracker_adapter.post_comment(state.tracker_handle, entry.ticket.id, body)
    :ok
  end

  @spec human_gate_name(OpenSleigh.PhaseConfig.t()) :: atom()
  defp human_gate_name(%OpenSleigh.PhaseConfig{gates: %{human: [gate_name | _]}}), do: gate_name
  defp human_gate_name(_phase_config), do: :commission_approved

  @spec pending_listener_view(map()) :: map()
  defp pending_listener_view(pending) do
    Map.take(pending, [:ticket_id, :session_id, :gate_name, :config_hash, :requested_at])
  end

  @spec resume_human_gate(map(), String.t(), HumanGateApproval.t()) :: map()
  defp resume_human_gate(state, ticket_id, approval) do
    case Map.pop(state.pending_human, ticket_id) do
      {nil, _pending} ->
        state

      {pending, pending_human} ->
        state
        |> Map.put(:pending_human, pending_human)
        |> build_approved_outcome(pending, approval)
    end
  end

  @spec build_approved_outcome(map(), map(), HumanGateApproval.t()) :: map()
  defp build_approved_outcome(state, pending, approval) do
    attrs =
      pending.outcome_attrs
      |> Map.update!(:gate_results, &(&1 ++ [{:human, approval}]))

    case OpenSleigh.PhaseOutcome.new(attrs) do
      {:ok, outcome} -> write_human_outcome(state, pending.session_id, outcome)
      {:error, reason} -> handle_human_resume_error(state, pending.session_id, reason)
    end
  end

  @spec write_human_outcome(map(), String.t(), OpenSleigh.PhaseOutcome.t()) :: map()
  defp write_human_outcome(state, session_id, outcome) do
    with {:ok, entry} <- Map.fetch(state.running, session_id),
         {:ok, _artifact_id} <-
           Client.write_ticket_artifact(
             entry.session.adapter_session,
             outcome,
             entry.ticket.id,
             state.haft_invoker
           ) do
      handle_outcome(state, session_id, outcome)
    else
      :error -> state
      {:error, reason} -> handle_human_resume_error(state, session_id, reason)
    end
  end

  @spec handle_human_resume_error(map(), String.t(), term()) :: map()
  defp handle_human_resume_error(state, session_id, reason) do
    :ok =
      ObservationsBus.emit(:human_gate_resume_failed, inspect(reason), %{session_id: session_id})

    release_claim(state, session_id)
  end

  @spec reject_human_gate(map(), String.t(), String.t()) :: map()
  defp reject_human_gate(state, ticket_id, reason) do
    case Map.pop(state.pending_human, ticket_id) do
      {nil, _pending} ->
        state

      {pending, pending_human} ->
        :ok = ObservationsBus.emit(:human_gate_rejected, reason, %{ticket: ticket_id})

        state
        |> Map.put(:pending_human, pending_human)
        |> release_claim(pending.session_id)
        |> dispatch(pending_ticket(state, pending), pending.outcome_attrs.phase)
    end
  end

  @spec timeout_human_gate(map(), String.t()) :: map()
  defp timeout_human_gate(state, ticket_id) do
    case Map.pop(state.pending_human, ticket_id) do
      {nil, _pending} ->
        state

      {pending, pending_human} ->
        :ok = ObservationsBus.emit(:human_gate_timeout, ticket_id, %{ticket: ticket_id})

        state
        |> Map.put(:pending_human, pending_human)
        |> release_claim(pending.session_id)
    end
  end

  @spec pending_ticket(map(), map()) :: Ticket.t()
  defp pending_ticket(state, pending) do
    state.running
    |> Map.fetch!(pending.session_id)
    |> Map.fetch!(:ticket)
  end

  @spec decide_next(map(), String.t(), map(), OpenSleigh.PhaseOutcome.t()) :: map()
  defp decide_next(state, session_id, entry, outcome) do
    case PhaseMachine.next(entry.workflow_state, outcome) do
      {:advance, next_phase} ->
        advance_to_next_phase(state, session_id, entry, outcome, next_phase)

      {:block, reasons} ->
        :ok =
          ObservationsBus.emit(:phase_blocked, length(reasons), %{
            ticket: entry.ticket.id,
            phase: entry.session.phase
          })

        release_claim(state, session_id, entry.ticket.id)

      {:terminal, verdict} ->
        finalize_terminal(state, session_id, entry, verdict)
    end
  end

  @spec finalize_terminal(map(), String.t(), map(), atom()) :: map()
  defp finalize_terminal(state, session_id, entry, verdict) do
    :ok =
      ObservationsBus.emit(:session_terminal, Atom.to_string(verdict), %{
        ticket: entry.ticket.id
      })

    :ok = maybe_publish_terminal(state, entry, verdict)

    release_claim(state, session_id, entry.ticket.id)
  end

  @spec maybe_publish_terminal(map(), map(), atom()) :: :ok
  defp maybe_publish_terminal(state, entry, :pass) do
    state.external_publication
    |> Map.get(:tracker_transition_to, [])
    |> first_transition_target()
    |> publish_transition(state, entry)
  end

  defp maybe_publish_terminal(_state, _entry, _verdict), do: :ok

  @spec first_transition_target([String.t()] | term()) :: String.t() | nil
  defp first_transition_target([target | _rest]) when is_binary(target), do: target
  defp first_transition_target(_targets), do: nil

  @spec publish_transition(String.t() | nil, map(), map()) :: :ok
  defp publish_transition(nil, _state, _entry), do: :ok

  defp publish_transition(target, state, entry) do
    target_atom = tracker_state_atom(target)

    case state.tracker_adapter.transition(state.tracker_handle, entry.ticket.id, target_atom) do
      :ok ->
        :ok = post_terminal_comment(state, entry, target)

        ObservationsBus.emit(:tracker_transitioned, target, %{
          ticket: entry.ticket.id
        })

      {:error, reason} ->
        :ok = post_transition_failure_comment(state, entry, target, reason)

        ObservationsBus.emit(:tracker_transition_failed, Atom.to_string(reason), %{
          ticket: entry.ticket.id,
          target: target
        })
    end
  end

  @spec post_terminal_comment(map(), map(), String.t()) :: :ok
  defp post_terminal_comment(state, entry, target) do
    body = "Open-Sleigh completed workflow and transitioned this ticket to `#{target}`."

    _ = state.tracker_adapter.post_comment(state.tracker_handle, entry.ticket.id, body)
    :ok
  end

  @spec post_transition_failure_comment(map(), map(), String.t(), atom()) :: :ok
  defp post_transition_failure_comment(state, entry, target, reason) do
    body =
      [
        "Open-Sleigh completed workflow but could not transition this ticket.",
        "",
        "Target state: `#{target}`",
        "Reason: `#{reason}`",
        "Action: transition this ticket manually or fix tracker permissions/config, then verify status.",
        "Marker: `open-sleigh:transition-failed:#{target}`"
      ]
      |> Enum.join("\n")

    _ = state.tracker_adapter.post_comment(state.tracker_handle, entry.ticket.id, body)
    :ok
  end

  @spec tracker_state_atom(String.t()) :: atom()
  defp tracker_state_atom(target) do
    target
    |> String.downcase()
    |> String.replace(~r/[^a-z0-9]+/, "_")
    |> String.trim("_")
    |> String.to_atom()
  end

  @spec advance_to_next_phase(
          map(),
          String.t(),
          map(),
          OpenSleigh.PhaseOutcome.t(),
          atom()
        ) :: map()
  defp advance_to_next_phase(state, session_id, entry, outcome, next_phase) do
    {:ok, new_workflow_state} = WorkflowState.apply_outcome(entry.workflow_state, outcome)

    state = release_claim(state, session_id)

    # Re-dispatch same ticket to next phase.
    case dispatch(entry.ticket, state, next_phase) do
      %{} = new_state ->
        update_workflow_state_for_next(new_state, entry.ticket.id, new_workflow_state)
    end
  end

  @spec update_workflow_state_for_next(map(), String.t(), WorkflowState.t()) :: map()
  defp update_workflow_state_for_next(state, ticket_id, new_workflow_state) do
    # Find the just-spawned session for this ticket and update its workflow_state.
    case find_running_by_ticket(state, ticket_id) do
      {sid, entry} ->
        entry = Map.put(entry, :workflow_state, new_workflow_state)
        %{state | running: Map.put(state.running, sid, entry)}

      nil ->
        state
    end
  end

  @spec find_running_by_ticket(map(), String.t()) :: {String.t(), map()} | nil
  defp find_running_by_ticket(state, ticket_id) do
    Enum.find_value(state.running, fn {sid, entry} ->
      if entry.ticket.id == ticket_id, do: {sid, entry}, else: nil
    end)
  end

  @spec find_running_by_monitor(map(), reference()) :: {String.t(), map()} | nil
  defp find_running_by_monitor(state, monitor_ref) do
    Enum.find_value(state.running, fn {sid, entry} ->
      if entry.monitor_ref == monitor_ref, do: {sid, entry}, else: nil
    end)
  end

  @spec handle_worker_down({String.t(), map()} | nil, map(), term()) :: map()
  defp handle_worker_down(nil, state, _reason), do: state

  defp handle_worker_down({session_id, _entry}, state, :normal) do
    Process.send_after(self(), {:normal_worker_down, session_id}, state.normal_exit_retry_ms)
    state
  end

  defp handle_worker_down({session_id, _entry}, state, reason) do
    schedule_retry(state, session_id, {:worker_down, reason})
  end

  @spec schedule_retry(map(), String.t(), term()) :: map()
  defp schedule_retry(state, session_id, reason) do
    schedule_retry(state, session_id, reason, :computed)
  end

  @spec schedule_retry(map(), String.t(), term(), :computed | non_neg_integer()) :: map()
  defp schedule_retry(state, session_id, reason, delay_override) do
    case Map.pop(state.running, session_id) do
      {nil, _running} ->
        state

      {entry, running} ->
        retry = build_retry(state, entry, reason, delay_override)
        :ok = maybe_emit_session_retry_failure(entry, reason)
        :ok = post_session_failure_comment(state, entry, reason, retry)
        demonitor(entry)

        %{
          state
          | running: running,
            claimed: MapSet.put(state.claimed, entry.ticket.id),
            retries: Map.put(state.retries, entry.ticket.id, retry),
            retry_attempts: Map.put(state.retry_attempts, entry.ticket.id, retry.attempt)
        }
    end
  end

  @spec build_retry(map(), map(), term(), :computed | non_neg_integer()) :: map()
  defp build_retry(state, entry, reason, delay_override) do
    ticket_id = entry.ticket.id
    attempt = retry_attempt(state.retry_attempts, ticket_id)
    delay_ms = retry_delay_ms(state, attempt, reason, delay_override)
    token = make_ref()

    %{
      attempt: attempt,
      due_at_ms: state.now_ms_fun.() + delay_ms,
      timer_ref: Process.send_after(self(), {:retry_timer_fired, ticket_id, token}, delay_ms),
      token: token,
      error: reason,
      delay_ms: delay_ms,
      phase: entry.session.phase
    }
  end

  @spec maybe_emit_session_retry_failure(map(), term()) :: :ok
  defp maybe_emit_session_retry_failure(_entry, :worker_down_normal), do: :ok

  defp maybe_emit_session_retry_failure(entry, reason) do
    ObservationsBus.emit(:session_errored, reason_text(reason), %{
      session_id: entry.session.id,
      ticket: entry.ticket.id,
      phase: entry.session.phase
    })
  end

  @spec post_session_failure_comment(map(), map(), term(), map()) :: :ok
  defp post_session_failure_comment(_state, _entry, :worker_down_normal, _retry), do: :ok

  defp post_session_failure_comment(state, entry, reason, retry) do
    marker = session_failure_marker(entry, reason)

    if tracker_comment_marker_posted?(state, entry.ticket.id, marker) do
      :ok
    else
      body = session_failure_comment_body(entry, reason, retry, marker)
      _ = state.tracker_adapter.post_comment(state.tracker_handle, entry.ticket.id, body)
      :ok
    end
  end

  @spec session_failure_comment_body(map(), term(), map(), String.t()) :: String.t()
  defp session_failure_comment_body(entry, reason, retry, marker) do
    [
      "Open-Sleigh session failed for this ticket.",
      "",
      "Session: `#{entry.session.id}`",
      "Phase: `#{entry.session.phase}`",
      "Reason: `#{reason_text(reason)}`",
      "Retry: attempt #{retry.attempt} scheduled in #{retry.delay_ms}ms.",
      "Action: #{session_failure_action(reason)}",
      "Marker: `#{marker}`"
    ]
    |> Enum.join("\n")
  end

  @spec session_failure_action(term()) :: String.t()
  defp session_failure_action(:agent_command_not_found) do
    "Install the configured agent command or fix PATH, then let the next retry run."
  end

  defp session_failure_action(:agent_launch_failed) do
    "Check the agent runtime can launch from this shell and that required credentials are present."
  end

  defp session_failure_action(:thread_start_failed) do
    "Check the agent app-server handshake/thread startup path, then let the next retry run."
  end

  defp session_failure_action(:initialize_failed) do
    "Check the agent app-server initialize response and local agent login state."
  end

  defp session_failure_action(:handshake_timeout) do
    "Check that the agent app-server starts promptly and is not waiting for interactive input."
  end

  defp session_failure_action(:stalled) do
    "Inspect the agent runtime; Open-Sleigh will retry with backoff unless the ticket is moved inactive."
  end

  defp session_failure_action(:timed_out) do
    "Inspect the agent turn timeout and workspace state before the next retry."
  end

  defp session_failure_action(:haft_unavailable) do
    "Start or fix Haft, then let the next retry persist the phase artifact."
  end

  defp session_failure_action(reason)
       when reason in [
              :path_outside_workspace,
              :path_symlink_escape,
              :path_hardlink_escape,
              :path_symlink_loop,
              :workspace_is_self,
              :path_forbidden
            ] do
    "Fix the workspace or path guard configuration before retrying this ticket."
  end

  defp session_failure_action(_reason) do
    "Fix the runtime issue shown above; Open-Sleigh will retry while the ticket remains active."
  end

  @spec session_failure_marker(map(), term()) :: String.t()
  defp session_failure_marker(entry, reason) do
    "open-sleigh:session-failed:#{entry.session.phase}:#{reason_id(reason)}"
  end

  @spec retry_attempt(map(), String.t()) :: pos_integer()
  defp retry_attempt(attempts, ticket_id) do
    attempts
    |> Map.get(ticket_id, 0)
    |> Kernel.+(1)
  end

  @spec retry_delay_ms(map(), pos_integer(), term(), :computed | non_neg_integer()) ::
          non_neg_integer()
  defp retry_delay_ms(_state, _attempt, _reason, delay_ms) when is_integer(delay_ms), do: delay_ms

  defp retry_delay_ms(state, _attempt, :worker_down_normal, :computed),
    do: state.normal_exit_retry_ms

  defp retry_delay_ms(state, attempt, _reason, :computed) do
    backoff = trunc(state.base_retry_backoff_ms * :math.pow(2, attempt - 1))
    min(backoff, state.max_retry_backoff_ms)
  end

  @spec handle_retry_timer(map(), String.t(), reference()) :: map()
  defp handle_retry_timer(state, ticket_id, token) do
    state.retries
    |> Map.get(ticket_id)
    |> retry_timer_action(state, ticket_id, token)
  end

  @spec retry_timer_action(map() | nil, map(), String.t(), reference()) :: map()
  defp retry_timer_action(%{token: token, phase: phase}, state, ticket_id, token) do
    case state.tracker_adapter.get(state.tracker_handle, ticket_id) do
      {:ok, %Ticket{} = ticket} -> retry_active_ticket(state, ticket, phase)
      {:error, _reason} -> clear_retry_claim(state, ticket_id)
    end
  end

  defp retry_timer_action(_retry, state, _ticket_id, _token), do: state

  @spec retry_active_ticket(map(), Ticket.t(), atom()) :: map()
  defp retry_active_ticket(state, %Ticket{} = ticket, phase) do
    if ticket.state in state.active_states do
      state = clear_retry(state, ticket.id)
      dispatch(ticket, state, phase)
    else
      clear_retry_claim(state, ticket.id)
    end
  end

  @spec clear_retry(map(), String.t()) :: map()
  defp clear_retry(state, ticket_id) do
    %{
      state
      | retries: Map.delete(state.retries, ticket_id),
        claimed: MapSet.delete(state.claimed, ticket_id)
    }
  end

  @spec clear_retry_claim(map(), String.t()) :: map()
  defp clear_retry_claim(state, ticket_id) do
    %{
      state
      | retries: Map.delete(state.retries, ticket_id),
        retry_attempts: Map.delete(state.retry_attempts, ticket_id),
        claimed: MapSet.delete(state.claimed, ticket_id)
    }
  end

  @spec release_claim(map(), String.t()) :: map()
  defp release_claim(state, session_id) do
    case Map.pop(state.running, session_id) do
      {nil, _} ->
        state

      {entry, new_running} ->
        demonitor(entry)

        %{
          state
          | running: new_running,
            claimed: MapSet.delete(state.claimed, entry.ticket.id),
            retry_attempts: Map.delete(state.retry_attempts, entry.ticket.id)
        }
    end
  end

  @spec release_claim(map(), String.t(), String.t()) :: map()
  defp release_claim(state, session_id, _ticket_id) do
    release_claim(state, session_id)
  end

  @spec demonitor(map()) :: :ok
  defp demonitor(%{monitor_ref: monitor_ref}) when is_reference(monitor_ref) do
    Process.demonitor(monitor_ref, [:flush])
    :ok
  end

  defp demonitor(_entry), do: :ok

  @spec normalize_worker_error(term()) :: term()
  defp normalize_worker_error(:stall_timeout), do: :stalled
  defp normalize_worker_error(:turn_timeout), do: :timed_out
  defp normalize_worker_error(reason), do: reason

  @spec reason_text(term()) :: String.t()
  defp reason_text(reason) when is_atom(reason), do: Atom.to_string(reason)
  defp reason_text(reason) when is_binary(reason), do: reason
  defp reason_text(reason), do: inspect(reason)

  @spec reason_id(term()) :: String.t()
  defp reason_id(reason) do
    reason
    |> reason_text()
    |> String.downcase()
    |> String.replace(~r/[^a-z0-9_]+/, "_")
    |> String.trim("_")
    |> fallback_reason_id()
  end

  @spec fallback_reason_id(String.t()) :: String.t()
  defp fallback_reason_id(""), do: "unknown"
  defp fallback_reason_id(reason_id), do: reason_id

  @spec status_retries(map()) :: map()
  defp status_retries(retries) do
    Map.new(retries, fn {ticket_id, retry} ->
      {ticket_id, Map.take(retry, [:attempt, :delay_ms, :due_at_ms, :error])}
    end)
  end
end
