---
title: "12. Agent Adapter Parity Plan (CHR-09)"
description: The parity frame for adding a second Agent.Adapter (Claude after Codex). Declared BEFORE shipping the second adapter, not after.
reading_order: 12
---

# Open-Sleigh: Agent Adapter Parity Plan

> **FPF note.** Adding a second `Agent.Adapter` changes the **epistemic
> properties** of every artifact Open-Sleigh produces. The Pareto winner
> over "best-Codex-approach + best-Codex-approach' + best-Codex-approach''"
> is not "best approach"; it's "best Codex-flavored approach."
> Declaring a parity frame before shipping is the FPF CHR-09 discipline:
> comparability governance requires equal inputs, equal budgets, equal
> evaluation protocol.
>
> **Carrier scope.** This file IS the `ADAPTER_PARITY.md` referenced
> throughout the spec set. (Previously referred to as a future-plan
> stub in SPEC.md §7a and SCOPE_FREEZE.md — promoted to a first-class
> spec in v0.7.)

---

## 1. Parity requirements

These must hold before a second adapter ships on anything other than
canary.

- **Equal turn budgets.** Both adapters receive the same `max_turns`,
  `max_tokens_per_turn`, and `wall_clock_timeout` from `sleigh.md`.
- **Equal tool surface.** The phase-scoped tool list is identical across
  adapters; if Claude can't bind a tool that Codex can (or vice versa),
  that tool is disabled globally until parity is restored.
- **Equal prompt surface.** The phase prompt templates are adapter-
  agnostic; adapter-specific escape hatches are forbidden.
- **Equal canary bar.** A new adapter must pass the canary suite 24h
  green AND post comparable `gate_bypass_rate` /
  `reopen_after_measure_rate` (within 20% of incumbent) before it may
  be used on a non-canary repo.
- **Versioning.** `sleigh.md` records `agent.version_pin` per adapter;
  mixing adapter versions across a Pareto comparison (MVP-2) requires a
  `waiver` with rationale.

## 2. Time-box and revert (MVP-1.5)

Per `OPEN_QUESTIONS.md` Q-OS-1, the Claude adapter ships in MVP-1.5
under a **14-calendar-day time-box** from MVP-1 canary-green. Revert
criteria, measured on canary T1/T1'/T2/T3:

- Claude `gate_bypass_rate` must be within **20% relative** of Codex on
  the same canary suite.
- Claude `reopen_after_measure_rate` must be within **20% relative**.

If not met at day 14, **revert to phases-first** and re-document the
adapter gap as MVP-2.1 with the observed parity delta as prior.

## 3. Parity evidence location

- The parity **contract** (this document) stays in the open-sleigh
  repo for the life of the project. Lifecycle is coupled to adapter
  code and the rest of the spec set — same repo is correct.
- `fixtures/` directory (parity evidence artifacts) is governed by a
  **privacy rule**, not a size rule. Anything sourced from non-canary
  repos is `.gitignore`-excluded from day 1.
- **At OSS flip,** audit `fixtures/`. If it contains only canary
  traces, ship public. If it contains octacore_nova traces, split
  fixtures to a private sibling repo (e.g. `open-sleigh-fixtures`) and
  leave the contract public. Split is driven by OSS-flip audit, not by
  MB — size correlates with privacy but isn't the primitive.

## 4. MVP-1 Codex baseline

Codex is the incumbent MVP-1 adapter. These entries are the baseline
Claude must match in MVP-1.5.

- **Transport:** `codex app-server` subprocess, launched by a BEAM
  `Port` under `OpenSleigh.Agent.Codex.Server`.
- **Framing:** line-delimited JSON-RPC over stdio. Startup is
  `initialize → initialized → thread/start`; work turns are
  `turn/start` plus app-server notifications until `turn/completed`.
- **Turn budgets:** no adapter-local override. `max_turns`,
  `max_tokens_per_turn`, and `wall_clock_timeout_s` come from the
  frozen `AdapterSession` created for the phase.
- **Tool surface:** the adapter registry is the MVP-1 union:
  `read, write, edit, bash, grep, haft_query, haft_note, haft_problem,
  haft_decision, haft_refresh, haft_solution`. Phase access remains
  narrowed by `AdapterSession.scoped_tools`.
- **Prompt surface:** app-server receives the phase prompt or
  continuation guidance plus the required `config_hash` trailer. No
  Codex-specific prompt fork is admitted.
- **Runtime policy knobs:** `approval_policy`, `thread_sandbox`,
  `turn_sandbox_policy`, `read_timeout_ms`, `turn_timeout_ms`, and
  `stall_timeout_ms` are adapter configuration, not comparison
  dimensions. A parity run must pin them before comparing adapters.

## 5. Claude Code decision record

**Decision date:** 2026-04-22.

**Decision:** accept Claude Code support as an MVP-1.5 candidate, but
do not enable it for real project work until the Codex path has a
recorded live canary run.

**Why:** a second provider is useful for MVP-2 epistemic plurality, but
adding it before the incumbent path has live evidence would mix two
unknowns: Open-Sleigh runtime readiness and provider parity. The first
shipping step is therefore a skeleton adapter that implements the same
`Agent.Adapter` boundary and compile-time config path while returning an
explicit startup failure for live sessions.

**Parity criteria before live enablement:**

- same tool registry as the incumbent adapter;
- same phase-scoped tool rejection behavior;
- same turn-budget carrier (`AdapterSession`);
- same prompt templates and config-hash trailer;
- canary suite green for 24 hours after the Codex canary is green;
- `gate_bypass_rate` and `reopen_after_measure_rate` within the 20%
  relative bound defined above.

**Revert rule:** if parity is not demonstrated inside the 14-day MVP-1.5
time-box, keep the skeleton disabled and move provider expansion to
MVP-2.1 with the measured parity gap as input evidence.

## 6. Provider parity test matrix

The local parity matrix intentionally covers provider-independent
contract behavior, not live provider quality.

| Behavior | Codex | Claude skeleton | Evidence |
|---|---:|---:|---|
| Adapter kind is closed atom | yes | yes | `test/open_sleigh/agent/adapter_parity_test.exs` |
| Tool registry matches incumbent | yes | yes | parity test |
| Unknown tool rejected as `:tool_unknown_to_adapter` | yes | yes | parity test |
| Out-of-scope tool rejected before execution | yes | yes | parity test |
| Out-of-commission-scope mutation rejected before execution | yes | yes | parity test |
| Live session startup | enabled | explicit unsupported error | parity test + first real-run TODO |

## 7. MVP-2 implication

The first MVP-2 Solution-Factory run uses **≥2 adapter families** for
variant epistemic plurality. A Pareto front built over a single
adapter family is lexical substitution of "variant diversity" for
"agent-epistemic plurality" — a CL2 → CL1 assurance downgrade we would
not catch until MVP-2 evidence landed. See `SCOPE_FREEZE.md §MVP-2`
and `OPEN_QUESTIONS.md` Q-OS-1 rationale for the full argument.

---

## See also

- [AGENT_PROTOCOL.md](AGENT_PROTOCOL.md) — the normative JSON-RPC contract every adapter implements (§10 conformance requirements)
- [OPEN_QUESTIONS.md](OPEN_QUESTIONS.md) — Q-OS-1 adapter priority with fair counter and mitigation
- [SCOPE_FREEZE.md](SCOPE_FREEZE.md) — MVP-1.5 / MVP-2 tiering that binds adapter rollout to evidence
- [GATES.md](GATES.md) — `gate_bypass_rate` and `reopen_after_measure_rate` definitions used for the 20% parity check
