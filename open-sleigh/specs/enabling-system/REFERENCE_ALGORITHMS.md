---
title: "Open-Sleigh Reference Algorithms (non-normative)"
version: v0.1 (extracted from SPEC.md v0.6.1 in spec-set v0.7)
date: 2026-04-22
status: implementer orientation; normative contract is the surrounding spec set
valid_until: 2026-05-20
---

# Open-Sleigh — Reference Algorithm Skeletons

> **FPF note — non-normative.** These are skeletons for implementers — the
> normative contract is the rest of the spec set. `Plan ≠ Reality`:
> pseudocode is a `Description` of intended flow, not the engine itself.
> Treat these as orientation, not specification.
>
> Adapted from Symphony §16 to our phase-gated model. Compare with
> Symphony's Elixir impl at `.context/symphony/elixir/lib/symphony_elixir/`
> for a reference realisation of equivalent loops.

---

## 1. Service startup

```text
function start_service():
  configure_logging()
  start_observations_bus()        # L5, ETS init
  compile_sleigh_md()              # L6, validates budget + gate/tool/phase registries
  state = orchestrator_initial_state()
  validate_dispatch_config()       # fail startup if invalid
  startup_terminal_workspace_cleanup()   # WORKSPACE.md §4
  schedule_tick(delay_ms=0)
  event_loop(state)
```

## 2. Poll tick

```text
on_tick(state):
  state = reconcile_running_sessions(state)   # RISKS.md §3 tracker-wins + AGENT_PROTOCOL.md §8 stall
  if not validate_dispatch_config(): return schedule_tick
  candidates = tracker.fetch_candidate_tickets()
  for ticket in sort_by_priority(candidates):
    if no_slots(state): break
    if ticket.problem_card_ref is nil:        # ILLEGAL_STATES.md UP1 hard gate
      post_tracker_comment(ticket, "upstream framing required")
      continue
    if should_dispatch(ticket, state):
      state = dispatch_session(ticket, state, phase=:frame)
  emit_observations(state)
  schedule_tick(state.poll_interval_ms)
  return state
```

## 3. Dispatch one session

```text
function dispatch_session(ticket, state, phase):
  session = Session.new(ticket, phase, compiled_config, path_guard_ok())
  state.running[session.id] = session
  state.claimed.add(ticket.id)
  Task.Supervisor.start_child(fn ->
    AgentWorker.run(session)    # AGENT_PROTOCOL.md §3 continuation-turn loop inside
  end)
  return state
```

## 4. Worker loop (per phase session)

```text
function AgentWorker.run(session):
  workspace = WorkspaceManager.ensure_for(session.ticket, session.workspace_path)
  run_hook(session, :before_run)
  adapter_session = AdapterSession.new(session)
  case Agent.Adapter.start_session(adapter_session):
    {:ok, thread} ->
      turn_count = 0
      loop:
        prompt = PromptBuilder.render(session, turn_count + 1, max_turns)
        case Agent.Adapter.run_turn(thread, prompt):
          {:ok, turn_result} ->
            turn_count += 1
            update_token_observations(turn_result.usage)
            if session.phase in [:frame, :measure]:
              break   # single-turn phases
            if turn_count >= max_turns: break
            if not Tracker.Adapter.still_active?(session.ticket.id): break
            # else continuation — loop with guidance prompt
          {:error, reason} ->
            emit_turn_error(reason)
            break
      # Canonical single-constructor shape (v0.6.1 — gate_results is
      # required PhaseOutcome provenance input per Q-OS-3, so it is
      # evaluated BEFORE construction, not applied afterward).
      self_id = SessionScopedArtifactId.next()
      gate_results = GateChain.evaluate_pre_construction(
                       session.phase_config.gates,
                       turn_result,
                       evidence_from_session,
                       self_id)
      outcome = PhaseOutcome.new(
                  payload_of(turn_result),
                  %{config_hash: session.config_hash,
                    valid_until: PhaseConfig.default_valid_until(session.phase_config),
                    authoring_role: session.phase_config.agent_role,
                    self_id: self_id,
                    gate_results: gate_results,
                    evidence: evidence_from_session,
                    phase_config: session.phase_config})
      send_to_orchestrator(session.id, outcome)
    {:error, reason} -> send_to_orchestrator(session.id, {:error, reason})
  run_hook(session, :after_run)
  Agent.Adapter.close_session(thread)
```

## 5. Orchestrator — on worker message

```text
on_message(state, {:outcome, session_id, phase_outcome}):
  Haft.Client.write_artifact_with_wal(phase_outcome)
  next_decision = PhaseMachine.next(state.workflow_state[session_id], phase_outcome)
  case next_decision:
    {:advance, next_phase} -> dispatch_session(ticket, state, next_phase)
    {:block, reasons}      -> schedule_retry(state, ticket, next_attempt)
    {:await_human, hg}     -> post_tracker_approval_request(hg); park session
    {:terminal, verdict}   -> emit_final_observation(verdict); release claim
```

---

## Normative counterparts

Each skeleton above has normative anchors in the spec set. When implementing,
cross-check:

| Skeleton | Normative anchor(s) |
|---|---|
| §1 Service startup | `WORKSPACE.md §4` (terminal cleanup), `SLEIGH_CONFIG.md §3` (budget), `FUNCTIONAL_ARCHITECTURE.md §Compilation chain` |
| §2 Poll tick | `RISKS.md §3` (tracker-wins), `ILLEGAL_STATES.md` UP1 (upstream framing gate), `AGENT_PROTOCOL.md §8` (stall) |
| §3 Dispatch | `TARGET_SYSTEM_MODEL.md §Session`, `PHASE_ONTOLOGY.md axis 5`, `AGENT_PROTOCOL.md §3` |
| §4 Worker loop | `AGENT_PROTOCOL.md §3/§5`, `WORKSPACE.md §2`, `TARGET_SYSTEM_MODEL.md §PhaseOutcome`, `ILLEGAL_STATES.md` PR1–PR10 |
| §5 Orchestrator | `HAFT_CONTRACT.md §3` (WAL), `PHASE_ONTOLOGY.md §Axes`, `GATES.md §4` (HumanGate), `RISKS.md §3` |

---

## See also

- [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md) — layered build order that these skeletons land into (L5 worker + orchestrator)
- [FUNCTIONAL_ARCHITECTURE.md](FUNCTIONAL_ARCHITECTURE.md) — §Compilation chain end-to-end trace that these skeletons realise
- [../target-system/AGENT_PROTOCOL.md](../target-system/AGENT_PROTOCOL.md) — normative continuation-turn model §3
- [../target-system/HAFT_CONTRACT.md](../target-system/HAFT_CONTRACT.md) — normative WAL replay + cancellation protocol
