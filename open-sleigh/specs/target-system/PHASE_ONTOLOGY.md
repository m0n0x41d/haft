---
title: "4. Phase Ontology"
description: Phase graph, gate kinds (3), verdicts, transition rules. The multi-axis model of what Open-Sleigh is governing.
reading_order: 4
---

# Open-Sleigh: Phase Ontology

> **Why multi-axis.** A single "ticket status" enum collapses several
> orthogonal concerns into one string and guarantees drift. Open-Sleigh
> factors the governance surface into **4 independent axes** that
> combine to describe any session's state. Each axis has its own
> constructor discipline, its own illegal states, and its own
> transition rules.

## The 5 axes (v0.6 — added sub-state axis)

| Axis | Type | Who owns it | Example values |
|---|---|---|---|
| 1. Operational phase | `Phase.t()` closed sum (all MVP-1 + MVP-2 atoms pre-declared from day 1 per Q-OS-2 v0.5 resolution) | `PhaseMachine` (L3) | Commission-first MVP: `:preflight, :frame, :execute, :measure, :terminal`. MVP-2 added: `:characterize_situation, :measure_situation, :problematize, :select_spec, :accept_spec, :generate, :parity_run, :select, :commission, :measure_impact`. |
| 2. Gate-result kind | `GateKind.t()` sum | `Gate` modules (L2) | `:structural`, `:semantic`, `:human` |
| 3. Verdict | `Verdict.t()` sum | Semantic gate or Measure outcome | `:pass`, `:fail`, `:partial` |
| 4. Authoring role | `AuthoringRole.t()` sum | Adapter / human | `:frame_verifier`, `:executor`, `:measurer`, `:judge`, `:human` |
| 5. **Run-attempt sub-state** (v0.6, Symphony-inherited) | `RunAttemptSubState.t()` closed sum | `AgentWorker` (L5) | `:preparing_workspace, :building_prompt, :launching_agent_process, :initializing_session, :streaming_turn, :finishing, :succeeded, :failed, :timed_out, :stalled, :canceled_by_reconciliation` |

Why 5 axes. Axes 1–4 describe the artifact's *position in the
governance lifecycle* (what phase, what gate kind, what verdict, who
authored). Axis 5 describes *the runtime state of the worker that's
producing the artifact*. Retry policy, observability, and cancellation
semantics depend on axis 5 (e.g., a `:stalled` failure retries with
exponential backoff; a `:canceled_by_reconciliation` releases claim
without retry). Axis 5 is runtime-only and does NOT appear in
persisted `PhaseOutcome` — it lives on `Session.t()` and dies with
the session.

### Axis 5 — per-sub-state retry policy

Each sub-state has a distinct failure-retry class so logs, metrics,
and reopen decisions can differentiate them. Retry classes map 1:1 to
`EffectError.t()` variants (see `AGENT_PROTOCOL.md §6`).

| Sub-state | What's happening | On failure |
|---|---|---|
| `:preparing_workspace` | Workspace creation, `after_create` hook, clone/bootstrap | Retry with workspace-error class |
| `:building_prompt` | Template render for this turn | Abort (prompt error ≠ transient) |
| `:launching_agent_process` | `Port.open` + subprocess start | Retry if transient; classify `:agent_launch_failed` |
| `:initializing_session` | JSON-RPC handshake (initialize → initialized → thread/start) | Classify `:handshake_timeout` / `:initialize_failed` |
| `:streaming_turn` | Active turn; events flowing | Stall-timeout or turn-timeout class |
| `:finishing` | `turn/completed` received; gate evaluation running | Gate-block class (agent retry vs human escalation) |
| `:succeeded` | Terminal; all gates green for this phase | — |
| `:failed` | Terminal; non-recoverable class | Schedule exp-backoff retry |
| `:timed_out` | Turn exceeded `turn_timeout_ms` | Retry |
| `:stalled` | No events for `stall_timeout_ms` | Retry |
| `:canceled_by_reconciliation` | Tracker moved ticket out under us | Release claim; do not retry |

Runtime behaviour of these sub-states (code-level) is implemented by
`OpenSleigh.RunAttemptSubState` — see `lib/open_sleigh/run_attempt_sub_state.ex`
for the closed-sum type, `terminal?/1`, and `retryable?/1`.

A `PhaseOutcome` records values on all 4 axes. Losing any axis means
losing the ability to audit the artifact — so all 4 are constructor-
required at L1.

---

## Axis 1 — Operational phase

### MVP-1 phase graph (single-variant governed pipeline)

```
              ┌─────────────┐
              │ :preflight  │
              └──────┬──────┘
                     │ gates: commission_runnable,
                     │        decision_fresh,
                     │        lockset_available,
                     │        autonomy_envelope_allows
                     ▼
              ┌─────────────┐
              │   :frame    │
              └──────┬──────┘
                     │ gates: described_entity_field_present,
                     │        valid_until_field_present,
                     │        object_of_talk_is_specific (semantic)
                     ▼
              ┌─────────────┐
              │  :execute   │
              └──────┬──────┘
                     │ gates: design_runtime_split_ok,
                     │        lade_quadrants_split_ok (semantic)
                     │ HumanGate: commission_approved
                     │            (if PR targets main / release)
                     ▼
              ┌─────────────┐
              │  :measure   │
              └──────┬──────┘
                     │ gates: evidence_ref_not_self,
                     │        valid_until_field_present,
                     │        no_self_evidence_semantic
                     ▼
              ┌─────────────┐
              │  :terminal  │  (Verdict + ticket transition request)
              └─────────────┘
```

**Legal transitions (MVP-1):**

| From | To | Condition |
|---|---|---|
| (new session) | `:preflight` | Orchestrator leases a Haft WorkCommission |
| `:preflight` | `:frame` | Haft validates PreflightReport as pass |
| `:preflight` | `:terminal` | Commission stale/policy/conflict block or human review required |
| `:frame` | `:execute` | All Frame gates pass |
| `:execute` | `:measure` | All Execute gates pass + HumanGate approved (if triggered) |
| `:measure` | `:terminal` | All Measure gates pass |
| any | `:terminal` | Irrecoverable gate failure + exhausted retries → `Verdict.fail` |

**Illegal transitions (MVP-1):**

- `:preflight` → `:execute` (must verify frame before implementation)
- `:frame` → `:measure` (no clause)
- `:measure` → `:execute` (no loop-back in MVP-1; reopen creates a new session)
- `:terminal` → anything (terminal is absorbing)

### MVP-2 phase graph (full lemniscate)

Canonical names from `development_for_the_developed.md` Slide 12:

```
Problem Factory:
  Characterize situation → Measure situation → Problematize → Select+Spec

Solution Factory:
  Accept-spec → Generate → Parity-run → Select → Commission → Measure impact

Loop: Measure impact → (may reopen as new Characterize situation cycle)
```

Full MVP-2 phase machine defined in SPEC §4. MVP-2 compresses some
canonical steps per SPEC §4 notes (e.g., `:generate` combines variant
and architecture generation).

---

## Axis 2 — Gate-result kind

The three kinds are **compile-time distinct sum variants**; never
confused, never merged.

### Structural

- Pure L2 function `StructuralGate.apply(name, artifact)`.
- Deterministic; same input → same output.
- Fast; milliseconds.
- Failure mode: field missing, type wrong, graph shape illegal.
- Result type: `{:ok, :pass} | {:error, reason_atom}`.

### Semantic

- Effectful L2 contract; L4 invokes via `JudgeClient`.
- Non-deterministic; LLM judge may drift.
- Slow; seconds.
- Failure mode: meaning wrong (vacuous object of talk, mixed quadrants, evidence semantically self-referential).
- Result type: `{:ok, %{verdict: Verdict.t(), cl: 0..3, rationale: String.t()}} | {:error, reason}`.
- Every SemanticGate MUST have a GoldenSet (≥20 hand-labelled items); failure without a GoldenSet is `{:error, :uncalibrated}`.

### Human

- Triggered, not computed.
- Blocks transition pending external `/approve` signal.
- Latency: human-scale (minutes to hours; default 24h timeout).
- Result: `{:approved, HumanGateApproval.t()} | :rejected | :timeout`.

### Combining gate results

Structural failures are **blocking without retry** — they indicate a
bug in prompt or adapter.

Semantic failures are **blocking with retry** — agent is asked to
revise. Repeated failures feed `judge_false_pos_rate` observation.

Human rejections are **retrying at previous phase** — the session
regresses one phase with the human's reason attached.

Mixing kinds in aggregation is a type error: `GateResult.combine/1`
pattern-matches on kind and refuses untyped merges.

---

## Axis 3 — Verdict

| Verdict | Meaning | When emitted |
|---|---|---|
| `:pass` | Outcome satisfies the phase's acceptance | All gates returned pass; agent produced the declared work-product |
| `:fail` | Outcome does not satisfy; retry exhausted | Gate failed with no recoverable path; or max_turns exceeded; or adapter unrecoverable error |
| `:partial` | Partial success; some of the work-product shipped | Reserved for MVP-2 Solution Factory where one variant lands but the Pareto selection wasn't clean |

`Verdict` is a sum type. A free-string verdict is a type error.

### Verdict on Measure

Measure's Verdict is the **ticket-level** verdict. It's the value that
becomes the basis for:

- WorkCommission transition (`:pass` → request completion/evidence attach in Haft,
  subject to HumanGate where configured; `:fail` → write failure evidence and
  leave commission unresolved/failed).
- ExternalProjection update when configured (`:pass` may map to Ready for
  Review/Done, but tracker state never completes Haft work by itself).
- `reopen_after_measure_rate` observation denominator.
- Downstream consumers (Haft artifact graph, dashboards).

---

## Axis 4 — Authoring role

Records which role *produced* a given artifact or evidence item. Load-
bearing for `no_self_evidence_semantic`: the evidence carrier must be
authored by a role **external** to the authoring role of the artifact
it validates.

| Role | Produces | Example |
|---|---|---|
| `:preflight_checker` | PreflightReport over WorkCommission, DecisionRecord, ProblemCard, repo context, and envelope. **Never** authorizes execution by itself. | Agent in Preflight phase, reporting context to Haft |
| `:frame_verifier` | Frame PhaseOutcome with verification result on an **upstream** ProblemCardRef carried by WorkCommission. **Never** authors a ProblemCard — that is the upstream human's role in Haft + `/h-reason`. | Agent in Frame phase, verifying upstream framing |
| `:executor` | Code changes, PR, Execute PhaseOutcome | Agent in Execute phase |
| `:measurer` | Measure PhaseOutcome + evidence assembly | Agent in Measure phase |
| `:judge` | SemanticGateResult with rationale | JudgeClient (LLM-judge) |
| `:human` | HumanGateApproval | Human principal via `/approve` |

**Rule.** Evidence attached to a Measure outcome must NOT be authored by
`:measurer` alone. CI test results are authored by the CI system
(external; `authoring_role: :external, source: :ci`). A PR's merge sha
is authored by the tracker / git host. The `:measurer` role assembles
and references external evidence; it does not manufacture it.

---

## Gates per phase — MVP-1 canonical binding

This table is the ground truth; `sleigh.md.example` mirrors it. Drift is
a review finding.

| Phase | Structural gates | Semantic gates | Human gate (trigger) |
|---|---|---|---|
| `:preflight` | `commission_runnable`, `decision_fresh`, `lockset_available`, `autonomy_envelope_allows` | `context_material_change_review` | — |
| `:frame` | `problem_card_ref_present`, `described_entity_field_present`, `valid_until_field_present` | `object_of_talk_is_specific` | — |
| `:execute` | `design_runtime_split_ok` | `lade_quadrants_split_ok` | `commission_approved` (if `external_publication` matches) |
| `:measure` | `evidence_ref_not_self`, `valid_until_field_present` | `no_self_evidence_semantic` | — |

## Gates per phase — MVP-2 additions

| Phase | Added structural | Added semantic |
|---|---|---|
| `:problematize` | — | (ProblemCard completeness — TBD) |
| `:parity_run` | — | `cg_frame_wellformed` |
| `:commission` | `language_state_complete` | — |
| (any) obligation-language artifact | — | `contract_unpacked_ok` (A.6.C, deferred to MVP-2.1) |

---

## Refresh (`valid_until`) policy per artifact kind

| Artifact kind | Default `valid_until` | Refresh trigger |
|---|---|---|
| Frame PhaseOutcome | 7 days | Tracker state change, explicit `/refresh` comment |
| Execute PhaseOutcome | until PR merged or 30 days | PR close, CI run invalidation |
| Measure PhaseOutcome (evidence pack) | 30 days (hard) / 90 days with waiver | Reopen, new competing evidence |
| Parity Report (MVP-2) | 30 days | New adapter version, prompt change, judge drift |
| ADR (MVP-2) | 180 days | Refresh triggers declared in decision record |
| HumanGateApproval | session-lifetime only (no refresh) | (n/a — approval is a point-in-time event) |
| golden-set label | rubric-version-scoped | Rubric change → re-label; prompt/model-only change → reuse |

---

## The "4-axis product" view

Any session's state is the tuple `(Phase, GateKind history, Verdict,
AuthoringRole history)`. A well-formed session has:

- Exactly one current `Phase`.
- A chain of `GateResult` values per phase exit; all three kinds may appear in any phase.
- A `Verdict` only on `:terminal`.
- An `AuthoringRole` per artifact produced in the session, never a free string.

Losing any axis is losing a dimension of auditability. Constructors
for `PhaseOutcome`, `GateResult`, and `HumanGateApproval` refuse
incomplete tuples.

---

## See also

- [ILLEGAL_STATES.md](ILLEGAL_STATES.md) — concrete states the axis combinations must refuse
- [TARGET_SYSTEM_MODEL.md](TARGET_SYSTEM_MODEL.md) — the L1 structs that carry these axes
- [../enabling-system/FUNCTIONAL_ARCHITECTURE.md](../enabling-system/FUNCTIONAL_ARCHITECTURE.md) — L2 / L3 type mechanics
