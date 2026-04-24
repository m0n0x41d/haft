# Open-Sleigh — Haft Harness Runtime SPEC

> **FPF note.** This document is a `Description` (design-time episteme) of the
> commissioned execution subsystem inside the **Haft** product. The product
> target system is **Haft**. The object of talk here is the running harness
> runtime currently codenamed **Open-Sleigh** and exposed to operators as
> `haft harness`. The repository code is the `Carrier`. The running runtime is
> the `Object`. Do not confuse them.
>
> **Role of this file.** SPEC.md is the **operator-facing umbrella index** —
> short, narrative, one-screen-per-section. The normative content
> (type-level invariants, phase-gate catalogues, protocol contracts,
> config schema, illegal states, architectural layering) lives in
> `specs/`. Each section below is a 1–2 paragraph abstract + pointers.
>
> **valid_until:** 2026-05-20 (4 weeks). Refresh after MVP-1 runs on a canary
> repo for 24h and on octacore_nova for one real ticket.

---

## Revision log

- **v0.7.1 (2026-04-22) — commission-first review corrections.**
  Clarified that the checked-in runtime is still legacy tracker-first until
  MVP-1R lands, made WorkCommission Scope a hard authority boundary in the
  target docs, renamed overloaded `commission_approved` HumanGate language,
  and added the stale/scope canary expectations from the external review.
- **v0.7 (2026-04-22) — SPEC.md thinning.** SPEC.md refactored from a
  ~1200-line parallel authoring site to a ~300-line umbrella index,
  per the audit:
  > Currently SPEC.md is not a projection of `specs/` — it is a parallel
  > authoring site. That is where the FPF-level Description ≠ Description
  > issue lives.

  Canonical content moved from SPEC.md into `specs/`:
  - §6 gates (all subsections) + observations → **specs/target-system/GATES.md**
  - §7 Haft contract, SPOF, token accounting routing, cancellation → **specs/target-system/HAFT_CONTRACT.md**
  - §7.2 workspace hooks, §7.5 startup cleanup → **specs/target-system/WORKSPACE.md**
  - §7a Agent Adapter Parity Plan → **specs/target-system/ADAPTER_PARITY.md** (promoted from future-plan stub)
  - §8 sleigh.md config + §8.1 config hash → **specs/target-system/SLEIGH_CONFIG.md**
  - §10 acknowledged risks → **specs/target-system/RISKS.md**
  - §12 HTTP API → **specs/target-system/HTTP_API.md**
  - §13 reference algorithms → **specs/enabling-system/REFERENCE_ALGORITHMS.md**
  - §5.2 run-attempt sub-state retry-policy table → absorbed into **specs/target-system/PHASE_ONTOLOGY.md axis 5**

  SPEC.md retains: project identity (§1), influences / non-inheritance (§2),
  one-screen core-abstractions pointer table (§3), lemniscate overview
  diagram (§4), MVP-1 narrative + pointers (§5, §5.1, §5.2), non-goals
  pointer (§9), and the resolved-questions audit trail (§11). Section
  numbering preserved so inbound references from `lib/*` and
  `specs/agents/prompts/*` still anchor.
  `specs/README.md` updated to retire the two-sources-of-truth framing:
  SPEC.md is the umbrella; `specs/` is canonical for everything
  normative.
- v0.6.1 (2026-04-22): applied 5.4 Pro confirmation-pass review.
  All v0.4 CRITICALs confirmed closed; fixed 4 regressions v0.6
  introduced: (1) clarified L4/L5 ownership seam —
  `Haft.Client` / `Agent.Adapter` protocol + codec live at L4 as
  stateless modules; the GenServer/Port that owns subprocess state
  lives at L5 (`HaftSupervisor`, `AgentWorker`). (2) reworded
  `STACK_DECISION.md` HTTP refusal. (3) replaced
  `haft_problem(frame)` smoke-test example in `IMPLEMENTATION_PLAN.md`
  L4 green gate with `haft_query(status)` round-trip. (4) fixed
  reference-algorithm pseudocode to the canonical single-constructor
  shape. Plus: added bootstrap-only guidance for workspace hooks.
- v0.6 (2026-04-22): cross-referenced Symphony (`.context/symphony/SPEC.md`
  + its Elixir implementation) to close operational gaps in our MVP-1.
  Added continuation-turn model within Execute, run-attempt
  sub-states, workspace hooks, stall detection, token accounting,
  startup terminal workspace cleanup, optional minimal HTTP API,
  reference algorithm skeletons. New detail spec:
  `specs/target-system/AGENT_PROTOCOL.md`.
- v0.5 (2026-04-22): applied 5.4 Pro extended-thinking review.
  Committed to verifier-only Frame, closed Q-OS-2/Q-OS-3/Q-OS-4,
  swept stale gate names, hardened Thai-disaster guardrails with
  canonical-path resolution spec, adopted four-label enforcement
  taxonomy.
- v0.4 (2026-04-22): applied third-round review — statistical caveat
  for golden-set rates, default `valid_until` cadence per-phase,
  golden-set versioning rule, rubric-first labelling, adapter-first
  14-day time-box. **Role discipline:** removed "veto" framing per
  X-TRANSFORMER.
- v0.3 (2026-04-22): applied second-round review (source-verified
  against `semiotics_slideument.md`, `development_for_the_developed.md`,
  `FPF-Spec.md`). Corrected LADE fourth quadrant, aligned left-lobe
  first phase, declared "Generate" as compression, elevated Parity
  Report to first-class Haft artifact, added LLM-judge calibration,
  narrowed config hash scope, specified WAL ordering and cancel-mid-
  call compensation, re-defined canary tickets as gate-activation
  regression tests, bound OSS flip to evidence.
- v0.2 (2026-04-22): split gates into structural/semantic, relabeled
  MVP-1 as single-variant governed pipeline, added HumanGate,
  Observation indicators, Agent Adapter Parity Plan, Haft-SPOF
  failure mode, tracker reconciliation, hash-pinned prompt
  provenance.

---

## 1. What Open-Sleigh is inside Haft

Open-Sleigh is the long-running, OTP-supervised **commissioned execution
runtime** inside Haft. It is the current implementation of the user-facing
`haft harness` subsystem, not a peer product and not a second source of truth.

**Current reality:** the checked-in runtime is still a legacy tracker-first
bootstrap canary. It uses tracker tickets as intake while preserving the
phase/gate/adapter harness.

**Target reality (MVP-1R):** it claims Haft WorkCommissions, preflights the
linked DecisionRecord and Scope, spawns agents per commission, routes them
through phase-gated roles, records each phase as an evidenced work-product via
[Haft](https://github.com/m0n0x41d/quint-code), and only advances across
one-way doors when gates are green and a human principal or approved autonomy
envelope allows it.

**MVP-1** implements a **single-variant governed pipeline**
(`Frame → Execute → Measure`). It is a bootstrap, not yet a lemniscate: it
has no Generate/Parity-run/Select, therefore no Pareto discipline. Calling
MVP-1 a "lemniscate" would be a Description ≠ Reality drift.

**MVP-2** implements the full **two-lobe lemniscate** (Problem Factory +
Solution Factory) with variant generation, parity-run, and Pareto selection.
Only from MVP-2 onward is "FPF-compliant lemniscate lifecycle" an honest
label.

**In one sentence:** Haft Harness runtime = Symphony-inspired orchestration
under Haft-owned authority, scope, and evidence, built on OTP.

## 2. Influences and non-inheritance

- **OpenAI Symphony** (its `SPEC.md`) — design reference for Orchestrator,
  WorkflowStore, continuation turns, dynamic tools, SSH worker topology. We
  reimplement; we do not fork.
- **Haft** — the parent product and semantic authority. Haft owns
  problem/solution/decision/evidence/work-commission semantics and the
  operator-facing surfaces. Open-Sleigh owns runtime
  orchestration/lifecycle/supervision. In the current implementation seam, the
  runtime talks to the rest of Haft through `haft serve` over MCP JSON-RPC.
  That is a subsystem protocol boundary, not a peer-product boundary. See
  `specs/target-system/HAFT_CONTRACT.md`.
- **FPF** (Levenchuk's First Principles Framework) — methodological basis.
  The invariant distinctions, the lemniscate, the Comparability Governance
  frame, the valid_until discipline all come from here. See `.context/FPF-Spec.md`,
  `.context/slideument.md`, and `.context/semiotics_slideument.md`.

**Product / subsystem / enabling split** (FPF §Target ≠ Enabling):

- **Product target system** = Haft.
- **Subsystem in scope here** = Open-Sleigh, Haft's commissioned execution
  runtime. Canonical description: `specs/target-system/`.
- **Enabling systems for this subsystem** = (a) the engineering team that
  authors the SPEC and writes the code (`specs/enabling-system/`), (b)
  packaging/release/install machinery that ships the runtime under `haft`, and
  (c) agent-provider CLIs / app-servers that Open-Sleigh adapts to.
- **Optional external trackers** are part of the product supersystem as
  observer-facing carriers. They are not semantic authority and not the
  subsystem's target system.

**We are NOT:**
- a peer product next to Haft,
- a fork of Symphony,
- a reimplementation of Haft in Elixir,
- a coding agent (we orchestrate agents; the agent is Codex / Claude / etc.),
- an accounting or ticket system (external trackers are projection carriers),
- a CI system (we invoke the project's CI, we don't run tests ourselves).

See `specs/target-system/SCOPE_FREEZE.md §Explicitly cut` for the full
non-goals inventory.

## 3. Core abstractions (pointer table)

| Abstraction | OTP shape | Canonical spec |
|---|---|---|
| `Orchestrator` | `GenServer` (one per engine instance) | `specs/enabling-system/FUNCTIONAL_ARCHITECTURE.md` L5 |
| `WorkflowStore` | `GenServer` | `specs/enabling-system/FUNCTIONAL_ARCHITECTURE.md` L5 + L6 |
| `AgentWorker` | `Task` under `Task.Supervisor` | `specs/enabling-system/FUNCTIONAL_ARCHITECTURE.md` L5 |
| `PhaseMachine` | pure module | `specs/enabling-system/FUNCTIONAL_ARCHITECTURE.md` L3 + `specs/target-system/PHASE_ONTOLOGY.md` |
| `StructuralGate` / `SemanticGate` / `HumanGate` | pure / effectful / triggered | `specs/target-system/GATES.md` |
| `CommissionSource.Adapter` / `Projection.Adapter` / `Agent.Adapter` | L4 behaviours | `specs/target-system/AGENT_PROTOCOL.md` + `specs/enabling-system/FUNCTIONAL_ARCHITECTURE.md` L4 |
| `Haft.Protocol` + `Haft.Client` | L4 stateless codec + typed API | `specs/target-system/HAFT_CONTRACT.md` |
| `HaftSupervisor` + `HaftServer` | L5 process owner for `haft serve` | `specs/target-system/HAFT_CONTRACT.md §5` + `FUNCTIONAL_ARCHITECTURE.md` L5 |
| `Observations` | `GenServer` + ETS | `specs/target-system/GATES.md §5` |

The entity definitions (structs, constructors, relationships) are in
`specs/target-system/TARGET_SYSTEM_MODEL.md`.

## 4. Lemniscate phase machine — full vision (MVP-2 target)

Canonical FPF phase names, aligned with
`.context/slideument.md` and
`.context/FPF-Spec.md` B.4 Canonical Evolution Loop:

```
┌─────────────────────── Problem Factory ────────────────────────┐
│                                                                 │
│   (evidence pack + drift + new constraints enter here)          │
│                                                                 │
│   Characterize situation ──► Measure situation ──► Problematize │
│                                                        │        │
│                                                        ▼        │
│                                                  Select + Spec  │
│                                         (comparison & acceptance │
│                                          spec adopted)          │
└─────────────────────────────┬───────────────────────────────────┘
                              ▼
┌─────────────────────── Solution Factory ───────────────────────┐
│                                                                 │
│   Accept-spec ──► Generate ──► Parity-run ──► Select            │
│                      *                │                         │
│                                       ▼                         │
│                                   Commission                    │
│                                       │                         │
│                                       ▼                         │
│                              Measure impact ───────────┐        │
└────────────────────────────────────────────────────────┘        │
                                                                  │
                              (evidence pack exits to next loop)──┘
```

The full phase graph (MVP-1 + MVP-2 alphabets, legal transitions, axis
model, per-phase work-products, gate-per-phase binding) is canonical in
`specs/target-system/PHASE_ONTOLOGY.md`. Compression notes for phase
names (e.g. `Generate` collapsing Slide 12 items 4+5) are also
documented there.

## 5. MVP-1 scope cut — single-variant governed pipeline

**Four phases, linear, single-variant.** Not a lemniscate: no Generate /
Parity-run / Select, so no Pareto discipline. This is a bootstrap that
shortcuts straight to implementation with FPF-shaped governance around it.
The current implementation reaches this through legacy tracker-first intake;
MVP-1R reaches it through Haft WorkCommission intake.

```
Preflight (verify WorkCommission + DecisionRecord freshness)
  ──► Frame (verify upstream ProblemCardRef)
  ──► Execute ──► [HumanGate, if PR targets main / out-of-envelope]
  ──► Measure ──► [pass | block | reopen]
```

- **Preflight** — Open-Sleigh leases a Haft WorkCommission, checks that its
  linked DecisionRecord/ProblemCard/scope/envelope are still admissible, and
  sends a PreflightReport back to Haft. Execute starts only after Haft admits
  the commission.
- **Frame** — agent **verifies** the commission carries a valid upstream
  `ProblemCardRef` (authored by the human via Haft + `/h-reason`). The
  agent does NOT author a ProblemCard — that is explicitly outside the
  harness boundary. In commission-first mode the ref comes from the
  WorkCommission, not tracker text. If it is missing, resolves to a
  self-authored artifact, or is stale, Frame exits `Verdict.fail`.
- **Execute** — agent implements inside WorkCommission Scope, runs tests,
  pushes a PR. If the PR targets an `external_publication` branch,
  `one_way_door_approved` blocks Measure until `/approve`.
- **Measure** — agent calls `haft_decision(measure)`, attaches evidence
  (PR merge sha, CI run id, test counts).

The MVP-1 tier (what's in / deferred / cut) is canonical in
`specs/target-system/SCOPE_FREEZE.md`. Gates per phase in
`specs/target-system/GATES.md §6`. Phase-graph edges + legal transitions
in `specs/target-system/PHASE_ONTOLOGY.md`.

### 5.1 Continuation-turn model within Execute

A single Execute-phase session may run **multiple turns** on the same
live agent thread before exiting. Frame and Measure are single-turn
(`max_turns = 1`). The full turn-loop pattern, continuation-guidance
template, session-identity rules, and illegal-state invariants
(CT1–CT6) live in `specs/target-system/AGENT_PROTOCOL.md §3` +
`specs/target-system/ILLEGAL_STATES.md §Continuation-turn invariants`.

### 5.2 Run-attempt sub-states (per-session lifecycle)

Inside a single phase session, the `AgentWorker` passes through 11
sub-states (`:preparing_workspace` → `:succeeded | :failed | :timed_out
| :stalled | :canceled_by_reconciliation`). Each has a distinct
failure-retry policy. The full sub-state table with retry policies is
canonical in `specs/target-system/PHASE_ONTOLOGY.md §Axis 5 — per-sub-
state retry policy`. The `RunAttemptSubState` type is implemented at
`lib/open_sleigh/run_attempt_sub_state.ex`.

## 6. Gates

All gate content — structural gates, semantic gates (LLM-judge),
`HumanGate` protocol, calibration discipline (CHR-04 assurance tuple,
golden-set versioning, statistical caveat for n<50), and observation
indicators (anti-Goodhart) — is canonical in
`specs/target-system/GATES.md`.

## 7. Haft contract

The MCP contract, tool surface, SPOF failure mode, WAL replay semantics,
L4/L5 ownership seam, token-accounting isolation, and cancellation
protocol are canonical in `specs/target-system/HAFT_CONTRACT.md`.
Workspace management and hooks: `specs/target-system/WORKSPACE.md`.
Agent Adapter Parity Plan: `specs/target-system/ADAPTER_PARITY.md`.

## 8. Configuration — `sleigh.md`

The YAML schema, prompt-template structure, config-hash formula, freeze
semantics, size budget, and immutability rules are canonical in
`specs/target-system/SLEIGH_CONFIG.md`.

## 9. Non-goals

The complete inventory is in `specs/target-system/SCOPE_FREEZE.md
§Explicitly cut`. Load-bearing ones:

- **Not a CI system.** We invoke the project's existing CI; we don't run
  tests ourselves.
- **Not a code reviewer.** Codex/Claude do the work. We gate, we don't
  review.
- **Not a replacement for Haft.** Haft is the FPF authority/object store.
- **Not Linear-specific.** External projection targets are optional carriers.
- **Not Codex-specific long-term.** `Agent.Adapter` behaviour from day 1;
  Claude designed-in, shipped under Parity Plan.
- **Not a reasoning engine.** We orchestrate reasoning agents; we don't
  reason ourselves beyond structural/pattern checks.

## 10. Acknowledged risks and mitigations

Canonical in `specs/target-system/RISKS.md`. Covers bootstrap-risk
(canary rule), in-RAM state loss on crash, projection-vs-engine drift
(Haft-wins reconciliation, 30s soft-stop, compensating notes), and
the probabilistic nature of LLM-judge semantic gates.

## 11. Open questions and resolutions (pre-v0.5 audit trail)

Live open questions and resolutions from v0.5 onward live in
`specs/target-system/OPEN_QUESTIONS.md`. This section preserves the
pre-v0.5 audit trail so later readers can reconstruct how earlier
versions of the spec moved.

### Resolved in v0.2 (kept for auditability)

- ~~hot-reload granularity~~ → hash-pin per session
  (`specs/target-system/SLEIGH_CONFIG.md §2`).
- ~~haft serve per engine vs per ticket~~ → per-engine
  (`specs/target-system/HAFT_CONTRACT.md §1`).
- ~~state storage~~ → in-RAM + WAL (`specs/target-system/RISKS.md §2`
  + `specs/target-system/HAFT_CONTRACT.md §3`).

### Resolved in v0.3

**1. Canary repo** → create `m0n0x41d/sleigh-canary` as a new public
Elixir toy. The 3 seeded tickets are **gate-activation regression
tests**, not random work. Each ticket MUST exercise a specific gate
path:

| Canary work item | Content | Expected gate behaviour |
|---|---|---|
| T1 | WorkCommission **without** a valid `problem_card_ref` | Frame entry: `problem_card_ref_present` MUST hard-fail with `:no_upstream_frame`. Commission never enters Execute. |
| T1' | WorkCommission with a `problem_card_ref` whose upstream ProblemCard has a vacuous `describedEntity` | Frame exit: `object_of_talk_is_specific` MUST trip. Commission goes back to human; Open-Sleigh does NOT attempt to refine. |
| T2 | WorkCommission with valid upstream ProblemCard but obligation-heavy body | Execute/Measure exit: `lade_quadrants_split_ok` MUST trip. Agent must decompose before publishing. |
| T3 | WorkCommission with a valid upstream ProblemCard and clean specific body; PR targets `main` | All gates pass. `one_way_door_approved` HumanGate fires — operator `/approve`s to continue. |
| T4 | WorkCommission created from DecisionRecord revision R1, then decision superseded to R2 before start | Preflight MUST block as stale; Execute never starts. |
| T5 | WorkCommission Scope allows `lib/a.ex`; runner mutates `lib/b.ex` inside the same repo | `mutation_within_commission_scope` MUST hard-fail terminally; workspace safety is not enough. |

The canary ticket suite is the **regression set** for every gate,
prompt, or adapter change. See `specs/target-system/SCOPE_FREEZE.md
§MVP-1 canary` for the tier binding.

**2. OSS timing** → private until:
1. canary green for 24h continuous AND
2. ≥1 real octacore_nova ticket shipped with a full Haft evidence pack AND
3. `MEASUREMENT.md` summary of what actually ran written.

Flip is **bound to evidence, not calendar time**. Going public before
runtime evidence ships the Description as the Object — exactly the FPF
anti-pattern the whole project exists to prevent.

**3. Claude adapter priority** → **recommendation: MVP-1.5
(adapter-first) with a 14-day time-boxed revert.** Principal decides.
SPEC records recommendation + fair counter.

- **For adapter-first (CHR-11 + X-STATEMENT-TYPE):** Pareto over
  variants from one agent family is lexical substitution; parity is
  cheaper to establish at MVP-1 complexity; parity data calibrates
  MVP-2's Pareto prior.
- **Counter:** timeline cost is real; "if Claude fails parity that's
  evidence" is true only if the parity work lands.
- **Mitigation:** 14-day time-box from MVP-1 canary-green; revert
  criteria measured on canary T1/T2/T3 (`gate_bypass_rate` and
  `reopen_after_measure_rate` within 20% relative of Codex).

Live decision status is in `specs/target-system/OPEN_QUESTIONS.md
Q-OS-1`. Parity contract in `specs/target-system/ADAPTER_PARITY.md`.

### Resolved in v0.4

**4. Golden-set labelling owner** → **rubric-first, not labeller-
first.** The load-bearing artifact is `LABELLING_RUBRIC.md`, not the
headcount of labellers. MVP-1: Ivan solo-labels ≥20 items per gate
AND produces `LABELLING_RUBRIC.md` *during* labelling. Handoff gate
to pool: cold reviewer must label held-out sample with ≥80% agreement
against Ivan's labels. Pool phase uses 2 labellers per item on a 10%
overlap sample; `labeller_agreement_kappa` indicator (see
`specs/target-system/GATES.md §5`); κ < 0.7 reverts to solo. Live
status: `specs/target-system/OPEN_QUESTIONS.md Q-OS-5`.

**5. ADAPTER_PARITY.md location** → **contract in-repo permanently;
fixtures may split on privacy, not size.** See
`specs/target-system/ADAPTER_PARITY.md §3`.

### 12 / 13 — migrated

- Optional HTTP observability endpoint → `specs/target-system/HTTP_API.md`.
- Reference algorithm skeletons → `specs/enabling-system/REFERENCE_ALGORITHMS.md`.

---

## Reading order

For a first-time reader:

1. This file (SPEC.md) — orient + project identity.
2. `specs/README.md` — index into the analytical decomposition.
3. `specs/target-system/SYSTEM_CONTEXT.md` — why, role, supersystem.
4. `specs/target-system/TERM_MAP.md` — canonical vocabulary.
5. `specs/target-system/SCOPE_FREEZE.md` — MVP tiers.
6. `specs/target-system/PHASE_ONTOLOGY.md` — phase graph + axes.
7. `specs/target-system/TARGET_SYSTEM_MODEL.md` — L1 entities.
8. `specs/target-system/ILLEGAL_STATES.md` — states made impossible.
9. `specs/target-system/GATES.md` — gates + calibration + observations.
10. `specs/target-system/HAFT_CONTRACT.md` — MCP contract.
11. `specs/target-system/AGENT_PROTOCOL.md` — adapter protocol.
12. `specs/target-system/WORKSPACE.md` — workspace + hooks.
13. `specs/target-system/SLEIGH_CONFIG.md` — operator DSL.
14. `specs/target-system/RISKS.md` — accepted risks.
15. `specs/target-system/HTTP_API.md` — optional observability.
16. `specs/target-system/ADAPTER_PARITY.md` — Parity Plan.
17. `specs/target-system/OPEN_QUESTIONS.md` — live opens.
18. `specs/enabling-system/FUNCTIONAL_ARCHITECTURE.md` — 6-layer hierarchy.
19. `specs/enabling-system/STACK_DECISION.md` — Elixir/OTP rationale.
20. `specs/enabling-system/IMPLEMENTATION_PLAN.md` — L1 → L6 build order.
21. `specs/enabling-system/REFERENCE_ALGORITHMS.md` — non-normative skeletons.
