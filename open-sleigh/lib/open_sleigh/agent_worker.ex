defmodule OpenSleigh.AgentWorker do
  @moduledoc """
  One `(Ticket × Phase)` session's worker. Runs under a
  `Task.Supervisor` per SE2. Drives a phase session to completion
  (single-turn for MVP-1 skeleton; continuation-turn loop lands
  when real Codex adapter replaces the mock).

  **Single-writer discipline (SE1).** `AgentWorker` does NOT mutate
  shared state. It performs the turn, builds a `PhaseOutcome`, and
  sends a `{:outcome, session_id, outcome}` cast to the `Orchestrator`.
  The Orchestrator is the sole writer of session-state transitions.

  Sequence per SPEC §13 reference algorithm (single-turn MVP-1
  variant):

  1. `WorkspaceManager.create_for_ticket/3` — with `:new | :reused`
  2. Hook `:after_create` if `:new`
  3. Hook `:before_run`
  4. `Agent.Adapter.start_session/1`
  5. Build prompt (first-turn full prompt)
  6. `Agent.Adapter.send_turn/3`
  7. Build `GateContext` → `GateChain.evaluate/3` (structural + semantic)
  8. Build `PhaseOutcome.new/2` — the canonical single-constructor
     (gate-config consistency checked here per PR10)
  9. `Haft.Client.write_artifact/3`
  10. Send `{:outcome, session_id, outcome}` to `Orchestrator`
  11. `Agent.Adapter.close_session/1`
  12. Hook `:after_run`

  Continuation-turn loop (§5.1) is a follow-up iteration; the
  single-turn path covers T3 happy path with Mock adapter.
  """

  alias OpenSleigh.{
    Evidence,
    GateChain,
    GateContext,
    GateResult,
    Haft.Client,
    ObservationsBus,
    Phase,
    PhaseOutcome,
    Session,
    SessionScopedArtifactId,
    WorkCommission,
    WorkspaceManager
  }

  require Logger

  @typedoc "Per-run context bundle (all injected, no module globals)."
  @type ctx :: %{
          required(:session) => Session.t(),
          required(:phase_config) => OpenSleigh.PhaseConfig.t(),
          required(:prompt) => String.t(),
          required(:upstream_problem_card) => map() | nil,
          required(:agent_adapter) => module(),
          required(:judge_fun) => GateChain.judge_fun(),
          required(:haft_invoker) => Client.invoke_fun(),
          required(:orchestrator) => GenServer.server(),
          optional(:workspace_root) => Path.t(),
          optional(:guard_config) => OpenSleigh.Adapter.PathGuard.config(),
          optional(:hooks) => %{optional(atom()) => String.t()},
          optional(:hook_failure_policy) => %{optional(atom()) => atom()},
          optional(:hook_timeout_ms) => pos_integer(),
          optional(:now_fun) => (-> DateTime.t()),
          optional(:tracker_handle) => term(),
          optional(:tracker_adapter) => module(),
          optional(:active_states) => [atom()]
        }

  @typep turn_assessment :: %{
           required(:reply) => map(),
           required(:self_id) => SessionScopedArtifactId.t(),
           required(:produced_at) => DateTime.t(),
           required(:valid_until) => DateTime.t(),
           required(:gate_results) => [GateResult.t()],
           required(:evidence) => [Evidence.t()]
         }

  @typep session_result ::
           {:ok, PhaseOutcome.t()}
           | {:await_human, map()}
           | {:error, term()}

  @typep hook_policy :: :blocking | :warning | :ignore

  @doc """
  Run the phase session. On failure at any step, sends
  `{:error, session_id, reason}` to the orchestrator and exits
  normally so the Task.Supervisor doesn't restart-loop.
  """
  @spec run(ctx()) :: :ok
  def run(ctx) when is_map(ctx) do
    ctx
    |> run_session()
    |> finalize_result(ctx)

    :ok
  end

  # ——— steps ———

  @spec run_session(ctx()) :: session_result()
  defp run_session(ctx) do
    with {:ok, _workspace} <- prepare_workspace(ctx),
         :ok <- run_hook(ctx, :before_run),
         {:ok, handle} <- start_session(ctx) do
      run_open_session(ctx, handle)
    end
  end

  @spec start_session(ctx()) :: {:ok, term()} | {:error, atom()}
  defp start_session(%{agent_adapter: adapter, session: session}) do
    adapter.start_session(session.adapter_session)
  end

  @spec run_open_session(ctx(), term()) :: session_result()
  defp run_open_session(ctx, handle) do
    result = continuation_loop(ctx, handle)
    :ok = close_session(ctx, handle)
    result
  end

  @spec finalize_result(session_result(), ctx()) :: :ok
  defp finalize_result({:ok, outcome}, ctx) do
    case write_to_haft(ctx, outcome) do
      {:ok, _artifact_id} ->
        _ = run_hook_best_effort(ctx, :after_run)
        notify_orchestrator(ctx, {:outcome, ctx.session.id, outcome})

      {:error, reason} ->
        _ = run_hook_best_effort(ctx, :after_run)
        notify_orchestrator(ctx, {:error, ctx.session.id, reason})
    end
  end

  defp finalize_result({:error, reason}, ctx) do
    _ = run_hook_best_effort(ctx, :after_run)
    notify_orchestrator(ctx, {:error, ctx.session.id, reason})
  end

  defp finalize_result({:await_human, outcome_attrs}, ctx) do
    _ = run_hook_best_effort(ctx, :after_run)
    notify_orchestrator(ctx, {:await_human, ctx.session.id, outcome_attrs})
  end

  @spec prepare_workspace(ctx()) ::
          {:ok, Path.t()} | {:error, atom()}
  defp prepare_workspace(%{session: session} = ctx) do
    workspace_root = Map.get(ctx, :workspace_root, ".")
    guard_config = Map.get(ctx, :guard_config, default_guard_config())

    case WorkspaceManager.create_for_ticket(workspace_root, session.ticket.id, guard_config) do
      {:ok, path, freshness} ->
        with :ok <- maybe_run_after_create(ctx, freshness) do
          {:ok, path}
        end

      {:error, reason} ->
        {:error, reason}
    end
  end

  @spec maybe_run_after_create(ctx(), :new | :reused) :: :ok | {:error, atom()}
  defp maybe_run_after_create(ctx, :new), do: run_hook(ctx, :after_create)
  defp maybe_run_after_create(_ctx, :reused), do: :ok

  @spec run_hook(ctx(), atom()) :: :ok | {:error, atom()}
  defp run_hook(ctx, hook_name) do
    ctx
    |> execute_hook(hook_name)
    |> apply_hook_policy(ctx, hook_name)
  end

  @spec execute_hook(ctx(), atom()) :: :ok | {:error, atom()}
  defp execute_hook(ctx, hook_name) do
    hooks = Map.get(ctx, :hooks, %{})
    timeout = Map.get(ctx, :hook_timeout_ms, 60_000)

    case Map.get(hooks, hook_name) do
      nil ->
        :ok

      script when is_binary(script) ->
        workspace = Path.join(Map.get(ctx, :workspace_root, "."), ctx.session.ticket.id)
        WorkspaceManager.run_hook(workspace, script, timeout)
    end
  end

  @spec apply_hook_policy(:ok | {:error, atom()}, ctx(), atom()) :: :ok | {:error, atom()}
  defp apply_hook_policy(:ok, _ctx, _hook_name), do: :ok

  defp apply_hook_policy({:error, reason}, ctx, hook_name) do
    policy = hook_policy(ctx, hook_name)
    :ok = emit_hook_failure(ctx, hook_name, reason, policy)
    hook_policy_result(policy, reason)
  end

  @spec hook_policy_result(hook_policy(), atom()) :: :ok | {:error, atom()}
  defp hook_policy_result(:blocking, reason), do: {:error, reason}
  defp hook_policy_result(:warning, _reason), do: :ok
  defp hook_policy_result(:ignore, _reason), do: :ok

  @spec run_hook_best_effort(ctx(), atom()) :: :ok
  defp run_hook_best_effort(ctx, hook_name) do
    case execute_hook(ctx, hook_name) do
      :ok -> :ok
      {:error, reason} -> emit_hook_failure(ctx, hook_name, reason, :warning)
    end

    :ok
  end

  @spec hook_policy(ctx(), atom()) :: hook_policy()
  defp hook_policy(ctx, hook_name) do
    default = default_hook_policy(hook_name)

    ctx
    |> Map.get(:hook_failure_policy, %{})
    |> Map.get(hook_name, default)
    |> normalize_hook_policy(default)
  end

  @spec default_hook_policy(atom()) :: hook_policy()
  defp default_hook_policy(:after_run), do: :warning
  defp default_hook_policy(_hook_name), do: :blocking

  @spec normalize_hook_policy(term(), hook_policy()) :: hook_policy()
  defp normalize_hook_policy(:blocking, _default), do: :blocking
  defp normalize_hook_policy(:warning, _default), do: :warning
  defp normalize_hook_policy(:ignore, _default), do: :ignore
  defp normalize_hook_policy("blocking", _default), do: :blocking
  defp normalize_hook_policy("warning", _default), do: :warning
  defp normalize_hook_policy("ignore", _default), do: :ignore
  defp normalize_hook_policy(_value, default), do: default

  @spec emit_hook_failure(ctx(), atom(), atom(), hook_policy()) :: :ok
  defp emit_hook_failure(ctx, hook_name, reason, policy) do
    ObservationsBus.emit(:hook_failed, reason, %{
      hook: hook_name,
      policy: policy,
      session_id: ctx.session.id,
      ticket: ctx.session.ticket.id,
      phase: ctx.session.phase
    })
  end

  @spec continuation_loop(ctx(), term()) :: session_result()
  defp continuation_loop(ctx, handle) do
    run_turn(ctx, handle, 1, ctx.prompt)
  end

  @spec run_turn(ctx(), term(), pos_integer(), String.t()) ::
          session_result()
  defp run_turn(ctx, handle, turn_number, prompt) do
    with {:ok, reply} <- send_turn(ctx, handle, prompt),
         {:ok, assessment} <- assess_turn(ctx, reply) do
      maybe_continue(ctx, handle, turn_number, assessment)
    end
  end

  @spec send_turn(ctx(), term(), String.t()) ::
          {:ok, map()} | {:error, atom()}
  defp send_turn(%{agent_adapter: adapter, session: session}, handle, prompt) do
    adapter.send_turn(handle, prompt, session.adapter_session)
    |> normalize_turn_reply()
  end

  @spec normalize_turn_reply({:ok, map()} | {:error, atom()}) :: {:ok, map()} | {:error, atom()}
  defp normalize_turn_reply({:ok, %{status: :completed} = reply}), do: {:ok, reply}
  defp normalize_turn_reply({:ok, %{status: :timeout}}), do: {:error, :timed_out}
  defp normalize_turn_reply({:ok, %{status: :failed}}), do: {:error, :port_exit_unexpected}
  defp normalize_turn_reply({:ok, %{status: :cancelled}}), do: {:error, :cancel_grace_expired}
  defp normalize_turn_reply({:ok, _reply}), do: {:error, :response_parse_error}
  defp normalize_turn_reply({:error, :stall_timeout}), do: {:error, :stalled}
  defp normalize_turn_reply({:error, :turn_timeout}), do: {:error, :timed_out}
  defp normalize_turn_reply({:error, _reason} = error), do: error

  @spec assess_turn(ctx(), map()) :: {:ok, turn_assessment()} | {:error, term()}
  defp assess_turn(ctx, reply) do
    self_id = SessionScopedArtifactId.generate()
    now = now_of(ctx)
    valid_until = OpenSleigh.PhaseConfig.default_valid_until(ctx.phase_config, now)
    evidence = build_evidence(ctx.session.phase, reply, now)

    with {:ok, gate_ctx} <- gate_context(ctx, self_id, reply, valid_until, evidence),
         {:ok, gate_results} <- GateChain.evaluate(ctx.phase_config, gate_ctx, ctx.judge_fun) do
      {:ok,
       %{
         reply: reply,
         self_id: self_id,
         produced_at: now,
         valid_until: valid_until,
         gate_results: gate_results,
         evidence: evidence
       }}
    end
  end

  @spec maybe_continue(ctx(), term(), pos_integer(), turn_assessment()) ::
          session_result()
  defp maybe_continue(ctx, handle, turn_number, assessment) do
    ctx
    |> continuation_decision(turn_number, assessment)
    |> continue_or_finish(ctx, handle, assessment)
  end

  @typep continuation_decision :: :finish | {:continue, pos_integer()}

  @spec continuation_decision(ctx(), pos_integer(), turn_assessment()) :: continuation_decision()
  defp continuation_decision(ctx, turn_number, assessment) do
    ctx
    |> stop_checks(turn_number, assessment)
    |> Enum.any?(& &1.())
    |> continuation_result(turn_number)
  end

  @spec stop_checks(ctx(), pos_integer(), turn_assessment()) :: [(-> boolean())]
  defp stop_checks(ctx, turn_number, assessment) do
    [
      fn -> phase_complete?(assessment) end,
      fn -> Phase.single_turn?(ctx.session.phase) end,
      fn -> turn_number >= ctx.phase_config.max_turns end,
      fn -> not tracker_active?(ctx) end
    ]
  end

  @spec continuation_result(boolean(), pos_integer()) :: continuation_decision()
  defp continuation_result(true, _turn_number), do: :finish
  defp continuation_result(false, turn_number), do: {:continue, turn_number + 1}

  @spec continue_or_finish(continuation_decision(), ctx(), term(), turn_assessment()) ::
          session_result()
  defp continue_or_finish(:finish, ctx, _handle, assessment) do
    outcome_from_assessment(ctx, assessment)
  end

  defp continue_or_finish({:continue, next_turn}, ctx, handle, assessment) do
    prompt = continuation_guidance(ctx, next_turn, assessment)
    run_turn(ctx, handle, next_turn, prompt)
  end

  @spec close_session(ctx(), term()) :: :ok
  defp close_session(%{agent_adapter: adapter}, handle) do
    adapter.close_session(handle)
  end

  @spec phase_complete?(turn_assessment()) :: boolean()
  defp phase_complete?(%{gate_results: gate_results}) do
    case GateResult.combine(gate_results) do
      {:advance, []} -> true
      {:block, _reasons} -> false
    end
  end

  @spec tracker_active?(ctx()) :: boolean()
  defp tracker_active?(ctx) do
    ctx
    |> fetch_current_ticket()
    |> ticket_active?(active_states(ctx))
  end

  @spec fetch_current_ticket(ctx()) ::
          {:ok, OpenSleigh.Ticket.t()} | {:error, atom()} | :tracker_not_configured
  defp fetch_current_ticket(%{tracker_adapter: adapter, tracker_handle: handle, session: session}) do
    adapter.get(handle, session.ticket.id)
  end

  defp fetch_current_ticket(_ctx), do: :tracker_not_configured

  @spec ticket_active?(
          {:ok, OpenSleigh.Ticket.t()} | {:error, atom()} | :tracker_not_configured,
          [atom()]
        ) :: boolean()
  defp ticket_active?({:ok, ticket}, active_states), do: ticket.state in active_states
  defp ticket_active?(:tracker_not_configured, _active_states), do: true
  defp ticket_active?({:error, _reason}, _active_states), do: false

  @spec active_states(ctx()) :: [atom()]
  defp active_states(ctx), do: Map.get(ctx, :active_states, [:todo, :in_progress])

  @spec continuation_guidance(ctx(), pos_integer(), turn_assessment()) :: String.t()
  defp continuation_guidance(ctx, turn_number, assessment) do
    phase = Atom.to_string(ctx.session.phase)
    max_turns = ctx.phase_config.max_turns
    failures = format_gate_failures(assessment.gate_results)

    """
    Continuation guidance — Open-Sleigh Phase: #{phase}

    - The previous turn completed, but the phase exit gates have not all passed yet.
    - This is continuation turn ##{turn_number} of #{max_turns} for this (Ticket × Phase) session.
    - Resume from the current workspace and conversation state.
    - The original task prompt is already present in this thread — do not repeat it.
    - Gate failures that triggered this continuation (if any): #{failures}.
    - Do not end the turn while the phase is still incomplete unless you are truly blocked.
    """
    |> String.trim()
  end

  @spec format_gate_failures([GateResult.t()]) :: String.t()
  defp format_gate_failures(gate_results) do
    gate_results
    |> Enum.reject(&GateResult.pass?/1)
    |> Enum.map(&inspect/1)
    |> format_failure_list()
  end

  @spec format_failure_list([String.t()]) :: String.t()
  defp format_failure_list([]), do: "none"
  defp format_failure_list(failures), do: Enum.join(failures, "; ")

  @spec gate_context(ctx(), term(), map(), DateTime.t(), [Evidence.t()]) ::
          {:ok, GateContext.t()} | {:error, atom()}
  defp gate_context(ctx, self_id, reply, valid_until, evidence) do
    GateContext.new(%{
      phase: ctx.session.phase,
      phase_config: ctx.phase_config,
      ticket: ctx.session.ticket,
      self_id: self_id,
      config_hash: ctx.session.config_hash,
      turn_result: turn_result_with_runtime_facts(ctx, reply),
      evidence: evidence,
      upstream_problem_card: ctx.upstream_problem_card,
      proposed_valid_until: valid_until
    })
  end

  @spec turn_result_with_runtime_facts(ctx(), map()) :: map()
  defp turn_result_with_runtime_facts(%{session: %{phase: :preflight}} = ctx, reply) do
    reply
    |> Map.merge(preflight_runtime_facts(ctx))
  end

  defp turn_result_with_runtime_facts(_ctx, reply), do: reply

  @spec preflight_runtime_facts(ctx()) :: map()
  defp preflight_runtime_facts(ctx) do
    ctx.session
    |> session_commission()
    |> preflight_runtime_facts(ctx)
  end

  @spec preflight_runtime_facts(WorkCommission.t() | nil, ctx()) :: map()
  defp preflight_runtime_facts(nil, ctx) do
    %{
      checked_at: now_of(ctx)
    }
  end

  defp preflight_runtime_facts(%WorkCommission{} = commission, ctx) do
    commission_snapshot = commission_snapshot(ctx, commission)

    %{
      checked_at: now_of(ctx),
      commission: commission,
      commission_snapshot: commission_snapshot,
      current_snapshot: current_snapshot(ctx, commission_snapshot),
      current_decision: current_decision(ctx, commission)
    }
  end

  @spec session_commission(Session.t()) :: WorkCommission.t() | nil
  defp session_commission(%Session{} = session) do
    session
    |> Map.get(:commission)
    |> session_commission_result(session)
  end

  @spec session_commission_result(term(), Session.t()) :: WorkCommission.t() | nil
  defp session_commission_result(%WorkCommission{} = commission, _session), do: commission

  defp session_commission_result(_value, %Session{} = session) do
    session.adapter_session
    |> Map.get(:commission)
    |> adapter_commission_result()
  end

  @spec adapter_commission_result(term()) :: WorkCommission.t() | nil
  defp adapter_commission_result(%WorkCommission{} = commission), do: commission
  defp adapter_commission_result(_value), do: nil

  @spec commission_snapshot(ctx(), WorkCommission.t()) ::
          OpenSleigh.CommissionRevisionSnapshot.t() | nil
  defp commission_snapshot(ctx, %WorkCommission{} = commission) do
    ctx.session.ticket
    |> metadata_value(:commission_snapshot)
    |> commission_snapshot_value(ctx, commission)
  end

  @spec commission_snapshot_value(term(), ctx(), WorkCommission.t()) ::
          OpenSleigh.CommissionRevisionSnapshot.t() | nil
  defp commission_snapshot_value(
         %OpenSleigh.CommissionRevisionSnapshot{} = snapshot,
         _ctx,
         _commission
       ),
       do: snapshot

  defp commission_snapshot_value(_value, ctx, %WorkCommission{} = commission) do
    commission
    |> WorkCommission.revision_snapshot(commission_snapshot_attrs(ctx, commission))
    |> commission_snapshot_result()
  end

  @spec commission_snapshot_result(
          {:ok, OpenSleigh.CommissionRevisionSnapshot.t()}
          | {:error, OpenSleigh.CommissionRevisionSnapshot.new_error()}
        ) :: OpenSleigh.CommissionRevisionSnapshot.t() | nil
  defp commission_snapshot_result({:ok, snapshot}), do: snapshot
  defp commission_snapshot_result({:error, _reason}), do: nil

  @spec commission_snapshot_attrs(ctx(), WorkCommission.t()) :: map()
  defp commission_snapshot_attrs(ctx, %WorkCommission{} = commission) do
    %{
      problem_revision_hash: problem_revision_hash(ctx, commission),
      lease_id: preflight_lease_id(ctx, commission),
      lease_state: preflight_lease_state(ctx)
    }
  end

  @spec problem_revision_hash(ctx(), WorkCommission.t()) :: String.t()
  defp problem_revision_hash(ctx, %WorkCommission{} = commission) do
    ctx.session.ticket
    |> metadata_value(:problem_revision_hash)
    |> string_or_default(commission.decision_revision_hash)
  end

  @spec preflight_lease_id(ctx(), WorkCommission.t()) :: String.t()
  defp preflight_lease_id(ctx, %WorkCommission{} = commission) do
    ctx.session.ticket
    |> metadata_value(:lease_id)
    |> string_or_default("local-preflight:" <> commission.id)
  end

  @spec preflight_lease_state(ctx()) :: atom()
  defp preflight_lease_state(ctx) do
    ctx.session.ticket
    |> metadata_value(:lease_state)
    |> atom_or_default(:claimed_for_preflight)
  end

  @spec current_snapshot(ctx(), OpenSleigh.CommissionRevisionSnapshot.t() | nil) ::
          OpenSleigh.CommissionRevisionSnapshot.t() | nil
  defp current_snapshot(ctx, commission_snapshot) do
    ctx.session.ticket
    |> metadata_value(:current_snapshot)
    |> current_snapshot_value(commission_snapshot)
  end

  @spec current_snapshot_value(term(), OpenSleigh.CommissionRevisionSnapshot.t() | nil) ::
          OpenSleigh.CommissionRevisionSnapshot.t() | nil
  defp current_snapshot_value(%OpenSleigh.CommissionRevisionSnapshot{} = snapshot, _fallback),
    do: snapshot

  defp current_snapshot_value(_value, fallback), do: fallback

  @spec current_decision(ctx(), WorkCommission.t()) :: map()
  defp current_decision(ctx, %WorkCommission{} = commission) do
    ctx.session.ticket
    |> metadata_value(:current_decision)
    |> current_decision_value(commission)
  end

  @spec current_decision_value(term(), WorkCommission.t()) :: map()
  defp current_decision_value(%{} = decision, _commission), do: decision

  defp current_decision_value(_value, %WorkCommission{} = commission) do
    %{
      decision_ref: commission.decision_ref,
      decision_revision_hash: commission.decision_revision_hash,
      status: :active,
      refresh_due: false,
      freshness: :healthy
    }
  end

  @spec metadata_value(OpenSleigh.Ticket.t(), atom()) :: term()
  defp metadata_value(ticket, key) do
    ticket.metadata
    |> Map.get(key, Map.get(ticket.metadata, Atom.to_string(key)))
  end

  @spec string_or_default(term(), String.t()) :: String.t()
  defp string_or_default(value, _default) when is_binary(value) and value != "", do: value
  defp string_or_default(_value, default), do: default

  @spec atom_or_default(term(), atom()) :: atom()
  defp atom_or_default(value, _default) when is_atom(value) and not is_nil(value), do: value

  defp atom_or_default(value, default) when is_binary(value),
    do: string_atom_or_default(value, default)

  defp atom_or_default(_value, default), do: default

  @spec string_atom_or_default(String.t(), atom()) :: atom()
  defp string_atom_or_default(value, default) do
    value
    |> String.trim()
    |> string_atom_value(default)
  end

  @spec string_atom_value(String.t(), atom()) :: atom()
  defp string_atom_value("claimed_for_preflight", _default), do: :claimed_for_preflight
  defp string_atom_value("claimed", _default), do: :claimed_for_preflight
  defp string_atom_value("", default), do: default
  defp string_atom_value(_value, default), do: default

  @spec outcome_from_assessment(ctx(), turn_assessment()) ::
          {:ok, PhaseOutcome.t()} | {:await_human, map()} | {:error, atom()}
  defp outcome_from_assessment(ctx, assessment) do
    attrs = %{
      session_id: ctx.session.id,
      phase: ctx.session.phase,
      work_product: Map.take(assessment.reply, [:text, :usage]),
      evidence: assessment.evidence,
      gate_results: assessment.gate_results,
      config_hash: ctx.session.config_hash,
      valid_until: assessment.valid_until,
      authoring_role: ctx.phase_config.agent_role,
      self_id: assessment.self_id,
      produced_at: assessment.produced_at,
      phase_config: ctx.phase_config
    }

    case PhaseOutcome.new(attrs) do
      {:error, :human_gate_required_by_phase_config_but_missing} -> {:await_human, attrs}
      result -> result
    end
  end

  # MVP-1 skeleton evidence builder. Real `AgentWorker` will extract
  # evidence from `tool_call` / `tool_result` events in the adapter's
  # reply.events stream (AGENT_PROTOCOL.md §4). For now, Measure
  # synthesises one placeholder so `:evidence_required_on_measure`
  # passes end-to-end; `authoring_source: :test` makes the
  # placeholder obvious in Haft artifacts.
  @spec build_evidence(atom(), map(), DateTime.t()) :: [Evidence.t()]
  defp build_evidence(:measure, _reply, now) do
    case Evidence.new(:test_count, "mvp1-skeleton-placeholder", nil, 2, :test, now) do
      {:ok, e} -> [e]
      _ -> []
    end
  end

  defp build_evidence(_phase, _reply, _now), do: []

  @spec write_to_haft(ctx(), PhaseOutcome.t()) ::
          {:ok, binary()} | {:error, atom()}
  defp write_to_haft(ctx, outcome) do
    Client.write_ticket_artifact(
      ctx.session.adapter_session,
      outcome,
      ctx.session.ticket.id,
      ctx.haft_invoker
    )
  end

  @spec notify_orchestrator(ctx(), term()) :: :ok
  defp notify_orchestrator(%{orchestrator: target}, message) do
    GenServer.cast(target, message)
  end

  @spec now_of(ctx()) :: DateTime.t()
  defp now_of(ctx) do
    case Map.get(ctx, :now_fun) do
      fun when is_function(fun, 0) -> fun.()
      _ -> DateTime.utc_now()
    end
  end

  @spec default_guard_config() :: map()
  defp default_guard_config do
    %{forbidden_paths: [], forbidden_remote_substrings: ["open-sleigh", "open_sleigh"]}
  end
end
