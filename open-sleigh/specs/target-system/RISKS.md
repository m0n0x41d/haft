---
title: "14. Acknowledged Risks and Mitigations"
description: Known risks accepted for MVP-1 (bootstrap-risk, in-RAM state loss, projection-vs-engine drift, probabilistic semantic gates) and their mitigations.
reading_order: 14
---

# Open-Sleigh: Acknowledged Risks and Mitigations

> **FPF note.** These are risks the design has chosen to **accept**, not
> risks the design has missed. Each row states the failure mode, the
> mitigation that makes the residual risk tolerable, and the evidence
> trigger that would reopen the acceptance decision.

---

## 1. Bootstrap-risk (canary rule)

Open-Sleigh runs against two repos in MVP-1:

- `canary/` — trivial Elixir project with 3 seeded fake tickets;
  Open-Sleigh must be green for 24h here before `octacore_nova` sees
  any change.
- `octacore_nova` — real work.

**No Open-Sleigh change touches `octacore_nova` until canary has been
green 24h.** Enforced by Taskfile, not policy.

See `SCOPE_FREEZE.md §MVP-1`, `OPEN_QUESTIONS.md §11.2 (resolved v0.3)`
and the canary suite T1/T1'/T2/T3 for the gate-activation regression
tests.

## 2. In-RAM state loss on crash (MVP-1 accepted)

All Orchestrator state is in-process Erlang maps. On crash / restart:

- **Lost:** in-flight `AgentWorker` state, unwritten Haft artifacts
  (mitigated by WAL, see `HAFT_CONTRACT.md §3`).
- **Recovered:** Haft is the authoritative object store for WorkCommissions
  and persistent evidence; workers respawn on the next poll tick if the
  commission remains runnable. Legacy tracker-first mode uses tracker state
  as a bootstrap intake source only.
- **Accepted for MVP-1.** The cost of adding SQLite persistence for
  engine state (estimated 3-4 days) is not justified until MVP-2
  concurrency. See `../enabling-system/STACK_DECISION.md §Storage
  (MVP-2+)` for the trigger.

## 3. External projection vs engine concurrency (race on manual transitions)

A human moving a Linear/Jira/GitHub issue while `AgentWorker` owns the
WorkCommission creates drift. **Resolution: Haft wins for work semantics;
external tracker state becomes observed projection drift.**

- Every poll tick, `Orchestrator` re-checks Haft WorkCommission state and
  external projection drift (if configured).
- If Haft state changed out from under an owned commission, the `AgentWorker`
  receives `:cancel_grace` with a **30s soft-stop window**.
- If only external tracker state changed, Haft records drift/conflict and may
  instruct cancellation; the tracker state alone does not cancel or complete
  the work.
- **In-flight `haft_*` call handling.** A `haft_*` call currently in
  flight when `:cancel_grace` fires MUST either (a) complete within the
  30s window, or (b) time out at `min(remaining_grace, 10s)`. It is
  NEVER silently abandoned.
- **Compensating note (always written).** On cancel, the `AgentWorker`
  writes `haft_note(kind="cancelled", cause=<reason>, partial_refs=[...])`
  before exiting. `partial_refs` lists any `haft_*` artifacts that
  were created but not finalized in the cancelled phase. This replaces
  silent discard with auditable partial-state. See `HAFT_CONTRACT.md §7`.
- After 30s, if the worker hasn't exited cleanly, it is killed; the
  compensating note is written by the `Orchestrator` on the worker's
  behalf using the last-known state snapshot.
- Commission is then released. Next poll tick may re-dispatch if Haft still
  reports it runnable.

## 4. LLM-judge gates are probabilistic

Semantic gates via LLM-judge have false positives and false negatives.
This is explicitly tracked in the `gate_bypass_rate` observation (see
`GATES.md §5`) plus the per-gate `judge_false_pos_rate` and
`judge_false_neg_rate` indicators. On sustained high rates, the
response is to:

1. Tighten prompts,
2. Add examples,
3. Promote the gate to HumanGate,

**NOT** to remove it. Removing a gate because its judge is drifting
hides the failure mode that calibration was built to surface.

See `GATES.md §3` for the calibration discipline (CHR-04 assurance
tuple, golden-set versioning, statistical-caveat rule for n < 50).

---

## See also

- [HAFT_CONTRACT.md](HAFT_CONTRACT.md) — SPOF failure mode (§3), cancellation protocol (§7)
- [GATES.md](GATES.md) — observation indicators, LLM-judge calibration
- [SCOPE_FREEZE.md](SCOPE_FREEZE.md) — canary discipline and T1/T1'/T2/T3 gate-regression suite
- [ILLEGAL_STATES.md](ILLEGAL_STATES.md) — AD3 (no silent Haft drop), AD4 (cancellation partial-state discipline)
- [../enabling-system/STACK_DECISION.md](../enabling-system/STACK_DECISION.md) — revisit triggers for the storage acceptance
