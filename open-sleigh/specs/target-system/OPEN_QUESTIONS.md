---
title: "7. Open Questions"
description: Questions that remain open (non-blocking for `mix new` unless marked BLOCKING)
reading_order: 7
---

# Open-Sleigh: Open Questions

> Separation of concerns. This document holds live open questions and
> the decision rationale for recently-resolved ones. Historical resolutions
> (pre-v0.5) live in `../SPEC.md §11 (resolved)` for audit trail.
>
> **Resolved in v0.5:** Q-OS-2, Q-OS-3, Q-OS-4 (architectural closures
> forced by the 5.4 Pro extended-thinking review).

---

## Resolved in v0.5 (kept for auditability)

### Q-OS-2 — Phase sum-type shape → **RESOLVED: pre-declare full alphabet**

**Resolution.** `Phase.t()` is a closed sum with all MVP-1 and MVP-2 atoms
pre-declared from day 1:

```elixir
@type Phase.t ::
        :frame | :execute | :measure | :terminal                 # MVP-1
        | :characterize_situation | :measure_situation           # MVP-2
        | :problematize | :select_spec | :accept_spec
        | :generate | :parity_run | :select
        | :commission | :measure_impact
```

`Workflow.mvp1/0` routes only through the MVP-1 atoms; `Workflow.mvp2/0`
routes through the full alphabet. No `{:m2, atom()}` open-tagged variant.

**Rationale.** Partiality of `PhaseMachine.next/2` over an unknown atom
space is a worse failure mode than committing to names ahead of evidence.
Canonical names come from `development_for_the_developed.md` Slide 12 — a
published source, not speculation. `ILLEGAL_STATES.md` TR5 ("dynamic phase
outside workflow alphabet") now maps to **type-level** enforcement via sum
closure + pattern-match exhaustiveness, not constructor-discipline.

**Why this was open in v0.4.** The earlier trade-off worried about MVP-2
naming commitment. The 5.4 Pro review (CRITICAL-3) flagged the contradiction:
ILLEGAL_STATES claimed closure while the type remained open. Close the type.

**Decision owner:** engine author. **Resolved:** 2026-04-22 (v0.5).

---

### Q-OS-3 — `HumanGateApproval` construction model → **RESOLVED: single constructor with gate-config consistency**

**Resolution.** `PhaseOutcome.new/2` is the **only** constructor. No
`new_external/3` special path. The constructor validates **gate-config
consistency**:

- If the active `PhaseConfig.gates.human` list declares any human gate
  (e.g. `commission_approved`), the passed `gate_results` list MUST
  contain a matching `{:human, HumanGateApproval.t()}` element with
  `HumanGateApproval.at > session.claimed_at` and `config_hash` matching
  the session.
- Else construction fails with
  `:human_gate_required_by_phase_config_but_missing`.
- `HumanGateApproval` itself has no `external_publication` field.
  `PhaseOutcome` has no `external_publication` field either — the concept
  is expressed as "this phase's `PhaseConfig` declares a human gate" not
  as a boolean flag.

**Rationale.** The 5.4 Pro review (CRITICAL-4) correctly noted that the
v0.4 design split the approval story across two artifacts (separate
constructor AND an `external_publication` field). A single constructor
reading the `PhaseConfig` it was given is simpler and closes the gap.
Gate-config consistency is the load-bearing invariant; everything else
was scaffolding for it.

**Trade-off accepted.** Constructor-level enforcement (with Credo rule
forbidding direct struct literals). This is NOT type-level-impossible in
the Haskell sense — a determined caller could bypass via `%PhaseOutcome{...}`
literal, but that violates `OpenSleigh.Credo.NoDirectStructLiteral` and is
caught at CI. See P3 four-label taxonomy in `ILLEGAL_STATES.md`.

**Decision owner:** engine author. **Resolved:** 2026-04-22 (v0.5).

---

### Q-OS-4 — Per-phase `AllowedTool.t()` → **RESOLVED: hybrid (compile-time adapter registry + runtime per-phase MapSet)**

**Resolution.** Capability-leak prevention is split across two mechanisms,
each chosen for its failure mode:

1. **Compile-time adapter tool registry (type-level).** Each
   `Agent.Adapter` impl module has `@tool_registry [:read, :write, :bash, ...]`
   — a closed atom set of tools the adapter actually supports.
   `dispatch_tool/3` has function clauses only for those atoms. Passing a
   tool not in the adapter's registry fails at function-clause match.
   This is **type-level** in the pattern-match-exhaustive sense.
2. **Runtime per-phase `MapSet` scope (runtime-guard).**
   `AdapterSession.scoped_tools :: MapSet.t(atom())` is set from the
   active `PhaseConfig` at session construction. Before dispatch,
   `MapSet.member?(session.scoped_tools, tool)` is checked; violations
   return `{:error, :tool_forbidden_by_phase_scope}`. This MUST be
   runtime because `sleigh.md` hot-reload can change phase scopes without
   a recompile, and the whole point of hot-reload is to avoid recompiles.

**Rationale.** Generating a per-phase `AllowedTool.t()` type via macros
would be purely type-level but fights hot-reload. Pure runtime check loses
the cheap compile-time catch for "tool doesn't exist in this adapter."
Hybrid gives both: unknown-to-adapter fails at compile, out-of-phase-scope
fails at runtime with a specific error.

**Trade-off accepted.** ILLEGAL_STATES CL1–CL3 (phase-scope violations)
are **runtime-guard**, not **type-level**. Documented honestly in the
four-label taxonomy (P3) rather than overclaimed.

**Decision owner:** engine author. **Resolved:** 2026-04-22 (v0.5).

---

## Live open questions

### Q-OS-1 — Claude adapter priority: MVP-1.5 vs MVP-2.1

**Status:** Recommended as MVP-1.5 (adapter-first) per `../../SPEC.md §11`.
14-day time-box with measurable revert criteria (Claude `gate_bypass_rate`
and `reopen_after_measure_rate` within 20% relative of Codex on canary
T1'/T2/T3). Non-blocking for `mix new` because scaffolding is identical
in both orderings.

**Decision owner:** Ivan (principal). **Decision window:** open through
MVP-1 canary-green (MVP-1.5 starts immediately after).

---

### Q-OS-5 — Golden-set labelling owner handoff criteria

**Status:** Recommended per `../../SPEC.md §11 v0.4`: Ivan solo for MVP-1
+ produce `LABELLING_RUBRIC.md`, handoff to pool gated by ≥80%
cold-reviewer agreement. Non-blocking for `mix new`.

**Specifics pending.** Cold-reviewer identity (second FPF-literate person
vs time-delayed self-re-labelling); labelling-rubric format (per-gate vs
unified); CI automation of the ≥80% gate.

**Recommendation.** Defer until there are ≥20 items in any single gate's
golden set. For cold-reviewer, time-delayed self-re-labelling is the
cheapest initial option.

**Decision owner:** Ivan. **Resolve during MVP-1 labelling.**

---

### Q-OS-6 — OSS timing: `MEASUREMENT.md` content + release gates

**Status:** Bound to evidence per `../../SPEC.md §11 v0.4` (canary 24h
green + ≥1 real octacore_nova ticket shipped + `MEASUREMENT.md` written).
Non-blocking for `mix new`.

**Specifics pending.** `MEASUREMENT.md` template content; observation
thresholds that gate OSS release.

**Recommendation.** Write `MEASUREMENT.md` template before MVP-1 canary
starts. Release gate: `human_override_count == 0`,
`reopen_after_measure_rate == 0` on the single real ticket; canary 24h
green with 0 threshold crossings.

**Decision owner:** Ivan. **Resolve during MVP-1.**

---

### Q-OS-7 — Downstream-repo coupling: CI evidence source

**Status:** Open, likely low-stakes.

**The question.** Where does Open-Sleigh get `ci_run_id` evidence in
Measure? Options: (1) tracker (Linear CI integration), (2) direct git-host
API, (3) agent reports from inside Execute phase.

**Recommendation.** (3) for MVP-1 with `cl: 2` (agent-reported evidence
from similar context). Migrate to (1) when Linear's CI-link is populated.
(2) only if (1) proves unreliable.

**Decision owner:** engine author. **Resolve during MVP-1 Measure gate
implementation.**

---

### Q-OS-8 — Linear carrier for `problem_card_ref`

**Status:** Blocking the production Linear rollout, non-blocking for
mock/canary adapter tests.

**The question.** Which Linear issue carrier holds the required upstream
`problem_card_ref`?

**Current implementation note (2026-04-22).** The public Linear GraphQL
`Issue` schema was checked via introspection and does not expose an
`Issue.customFields` field. `OpenSleigh.Tracker.Linear` therefore supports
two configured extraction paths without declaring either as the production
contract: direct payload field projection (`problem_card_ref_field`) for a
future/alternate Linear projection, and a Markdown marker in `description`
(`problem_card_ref_marker`, default `problem_card_ref`) for canary use.

**Decision owner:** Ivan. **Resolve before running Open-Sleigh on a real
`octacore_nova` ticket.**

---

## Parked (not formally open; do not block `mix new`)

- Multi-tenant operation — solo-principal system today.
- Dashboard (LiveView) content scope — MVP-2 concern.
- SSH worker topology — MVP-2.1+ concern.
- Secret management for adapter API keys — env var for MVP-1; Vault/1Password later.
- Versioning / migration of `sleigh.md` as it evolves — handle when concrete breakage appears.

---

## Process for closing a question

1. Write the decision rationale inline in this document's "Resolved" section.
2. Mirror the decision in `../../SPEC.md` revision log for the version that
   closed it.
3. If the decision produces new illegal states or changes existing ones,
   update `ILLEGAL_STATES.md` in the same change with the correct
   four-label taxonomy annotation.
4. If the decision changes L1–L6 shapes, update
   `../enabling-system/FUNCTIONAL_ARCHITECTURE.md` in the same change.
