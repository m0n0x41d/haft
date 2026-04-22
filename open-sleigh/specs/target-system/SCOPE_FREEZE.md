---
title: "3. Scope Freeze"
description: What's in MVP-1 / MVP-1.5 / MVP-2 / later / explicitly cut
reading_order: 3
---

# Open-Sleigh: Scope Freeze

> **Why this document exists.** Scope creep is the #1 way the thai
> disaster replicates. Everything has a tier; everything has an explicit
> cut list. If it's not in a tier, it doesn't exist.

## Tiers

### MVP-1 — single-variant governed pipeline (current implementation)

**Goal:** One legacy tracker-first canary goes end-to-end through Frame →
Execute → Measure, producing a merged PR and a Haft evidence pack. This is the
bootstrap implementation already present in `open-sleigh/`.

**In scope:**

| Area | Item | Spec ref |
|---|---|---|
| Phases | Frame, Execute, Measure (linear) | `PHASE_ONTOLOGY.md §MVP-1 phase graph` |
| Structural gates | `described_entity_field_present`, `valid_until_field_present`, `evidence_ref_not_self`, `design_runtime_split_ok` | `GATES.md §1` |
| Semantic gates | `object_of_talk_is_specific`, `lade_quadrants_split_ok`, `no_self_evidence_semantic` (LLM-judge) | `GATES.md §2` |
| Human gate | Legacy runtime name: `commission_approved`. Commission-first target split: `one_way_door_approved` on PR → `main`; `publish_approved` on tracker → terminal | `GATES.md §4` |
| Observations | `gate_bypass_rate`, `agent_retry_count_per_ticket`, `human_override_count`, `reopen_after_measure_rate`, `phase_dwell_time_p90`, `judge_false_pos_rate`, `judge_false_neg_rate`, `labeller_agreement_kappa` | `GATES.md §5` |
| Adapters | Legacy `Tracker.Adapter` impl: Linear. `Agent.Adapter` impl: Codex CLI. `Haft.Client`. `JudgeClient` (uses Codex adapter with judge prompt). | `AGENT_PROTOCOL.md`, `HAFT_CONTRACT.md` |
| Storage | In-RAM orchestrator state + per-ticket WAL at `~/.open-sleigh/wal/` (commission-first uses per-commission WAL) | `HAFT_CONTRACT.md §3`, `RISKS.md §2` |
| Config | `sleigh.md` with YAML + Markdown. L6 compiler with size budget. Hash-pinning per session. | `SLEIGH_CONFIG.md` |
| Canary | Seeded gate-activation tickets T1/T1'/T2/T3; 24h green enforced by Taskfile | `RISKS.md §1`, `../../SPEC.md §11` |
| Judge calibration | Golden sets ≥20/gate; CHR-04 F/G/R/CL reported; labelling rubric as first-class artifact | `GATES.md §3` |
| **Continuation-turn model** (*Symphony-inherited, v0.6*) | Execute session runs N turns on the same thread until gates pass / ticket leaves active / `max_turns`. Frame and Measure are single-turn. | `AGENT_PROTOCOL.md §3` |
| **Run-attempt sub-states** (*v0.6*) | 11 sub-states (PreparingWorkspace → Succeeded/Failed/TimedOut/Stalled/CanceledByReconciliation) with distinct retry policies | `PHASE_ONTOLOGY.md §Axis 5` |
| **Workspace hooks** (*Symphony-inherited, v0.6*) | `after_create`, `before_run`, `after_run`, `before_remove` with `hooks.timeout_ms` | `WORKSPACE.md §2` |
| **Stall detection** (*Symphony-inherited, v0.6*) | `codex.stall_timeout_ms` — kill agent subprocess and retry on inactivity | `AGENT_PROTOCOL.md §8` |
| **Token accounting (observations-only)** (*v0.6*) | `codex_input/output/total_tokens` per session, aggregated on ObservationsBus, structurally isolated from Haft | `HAFT_CONTRACT.md §4`, `AGENT_PROTOCOL.md §4` |
| **Startup terminal workspace cleanup** (*v0.6*) | Sweep workspaces for terminal tickets on startup | `WORKSPACE.md §4` |
| **Per-state concurrency limits** (*v0.6*) | `agent.max_concurrent_agents_by_state` | `SLEIGH_CONFIG.md §1` |
| **Agent protocol contract** (*v0.6*) | Normative JSON-RPC shape for Agent.Adapter (Codex MVP-1, Claude MVP-1.5) | `AGENT_PROTOCOL.md` |
| **Optional HTTP observability API** (*v0.6, optional in MVP-1*) | `/api/v1/state`, `/api/v1/<ticket>`, `/api/v1/refresh` — observability-only, never exposes Haft artifacts | `HTTP_API.md` |

### MVP-1R — commission-first refactor (Haft monorepo integration)

**Goal:** Replace tracker-first intake with Haft WorkCommission intake while
preserving the OTP harness, phase gates, WAL, and status surface.

**In scope:**

| Area | Item | Spec ref |
|---|---|---|
| Intake | `CommissionSource.Adapter` talks to Haft for runnable WorkCommissions and leases | `HAFT_CONTRACT.md`, `TARGET_SYSTEM_MODEL.md` |
| Phase graph | Add `:preflight` before `:frame` | `PHASE_ONTOLOGY.md`, `GATES.md` |
| Freshness | Block stale/hash-mismatched/superseded decisions before Execute | `GATES.md`, Haft `EXECUTION_CONTRACT.md` |
| Scope enforcement | Carry Haft Scope into Session/AdapterSession; enforce mutating calls and terminal diff | `TARGET_SYSTEM_MODEL.md`, `GATES.md` |
| Runtime | `Session` owns WorkCommission, not tracker Ticket | `TARGET_SYSTEM_MODEL.md` |
| Local-only | Run with no Linear/Jira/GitHub credentials | `SYSTEM_CONTEXT.md`, `TERM_MAP.md` |
| Optional projection | ExternalProjection is published by Haft, not Open-Sleigh | Haft specs + `HAFT_CONTRACT.md` |
| Batch/YOLO | ImplementationPlan scheduling with dependencies, locksets, leases, AutonomyEnvelope | Haft `EXECUTION_CONTRACT.md` |

**Not in MVP-1R:**

- Rewriting the OTP runtime in Go/Rust/Tauri.
- Making Linear/Jira mandatory infrastructure.
- Letting Open-Sleigh create/approve WorkCommissions.
- Letting ProjectionWriterAgent decide status or completion.

### MVP-1.5 — Claude adapter + parity evidence

**Goal:** Prove the `Agent.Adapter` abstraction works with two LLM
families. Produce parity evidence before MVP-2's Solution Factory
depends on variant diversity.

**Time-box:** 14 calendar days from MVP-1 canary-green. Revert criteria:
Claude's `gate_bypass_rate` and `reopen_after_measure_rate` must be
within 20% relative of Codex's on canary T1/T2/T3. If not, document
the delta as an MVP-2.1 prior and revert to phases-first. See SPEC §11
Q3.

**In scope:**

| Area | Item |
|---|---|
| Adapter | `Agent.Adapter` second impl: Claude Code |
| Parity | `ADAPTER_PARITY.md` stood up: equal turn budgets, equal tool surface, equal prompt surface, equal canary bar |
| Golden-set labelling handoff | `LABELLING_RUBRIC.md` produced during MVP-1; cold-reviewer agreement ≥80% gates the handoff to a 2-person pool |

### MVP-2 — full two-lobe lemniscate

**Goal:** Problem Factory (Characterize situation → Measure situation
→ Problematize → Select+Spec) and Solution Factory (Accept-spec →
Generate → Parity-run → Select → Commission → Measure impact).

**In scope:**

| Area | Item | Spec ref |
|---|---|---|
| Phases | Full MVP-2 graph | `PHASE_ONTOLOGY.md §MVP-2 phase graph` |
| Structural gates (added) | `language_state_complete` | `GATES.md §1` |
| Semantic gates (added) | `cg_frame_wellformed` | `GATES.md §2` |
| Future gate (scoped in, deferred to MVP-2.1) | `contract_unpacked_ok` per A.6.C | `GATES.md §2` |
| Work-products (added) | Parity Report as first-class Haft artifact; ADR; Runbook+rollback | `PHASE_ONTOLOGY.md §MVP-2 phase graph` |
| Dashboard | LiveView-based observation dashboard | `HTTP_API.md` (MVP-1 precursor) |
| Claude adapter in Pareto | First MVP-2 Solution-Factory run uses ≥2 adapter families for variant epistemic plurality | `ADAPTER_PARITY.md §4`, `OPEN_QUESTIONS.md Q-OS-1` |

### MVP-2.1 and later (queued, not yet scheduled)

| Item | Why deferred |
|---|---|
| SSH worker distribution | Not needed until concurrency > 4 active commissions; single-host OTP is sufficient |
| GitHub/Jira Projection.Adapter | External projection after Linear semantics settle; not work intake |
| Dynamic tools beyond Haft MCP | Symphony-style dynamic tool dispatch; not load-bearing for MVP-1/2 |
| Problem portfolio management | One ticket = one problem today; portfolio-level view is Haft's concern, not ours |
| `contract_unpacked_ok` gate | A.6.C Contract Unpacking is valuable but not in the critical path; enable when MVP-2 stabilises |

---

## Explicitly cut (do not build)

These are things a reasonable reader might expect to exist. They
don't, and they won't, because they violate product integrity per
`SYSTEM_CONTEXT.md §6`.

| Cut item | Why cut |
|---|---|
| **Any reasoning surface in Open-Sleigh** (anything resembling `/h-reason` or a problem-framing UI) | Upstream in Haft. Open-Sleigh does not frame problems. |
| **ProblemCard authoring by any Open-Sleigh phase**, including Frame, Characterize situation, and Problematize (MVP-2) | The Frame phase **verifies** upstream human-authored ProblemCards; it does not write to `haft_problem(frame)`. MVP-2's Problem Factory phases operate on (enrich, characterize, refine) the upstream card — never create one ex nihilo. `haft_problem` is **not** in Frame's scoped toolset. Enforced by ILLEGAL_STATES UP1 (hard) + UP2 + UP3. |
| **Open-Sleigh's own code-as-agent-target** (using Open-Sleigh to improve Open-Sleigh) | L4 workspace-path allowlist structurally refuses it. Thai-disaster attractor. |
| **Harness-about-harness artifacts** (Haft notes / evidence describing Open-Sleigh's own operation) | `ObservationsBus` has zero compile-time path to `Haft.Client`. Enforced by module graph. |
| **CI integration** (running tests ourselves) | Downstream repos have their own CI. We read verdicts via tracker hooks; we don't execute tests. |
| **Code review** (Open-Sleigh suggesting changes to PRs) | Codex / Claude do the work inside Execute. Review is human or out-of-scope. |
| **Documentation generation** (Open-Sleigh producing docs about downstream projects) | Out of role. Agents can produce docs inside Execute if the ticket asks; Open-Sleigh itself does not. |
| **FOER / longform / retrospective prose at phase boundaries** | `rationale :: String.t() | nil` capped at 1000 chars by type. |
| **Telegram / Slack notifications** | Tracker comments are the notification surface for MVP-1/2. |
| **Web UI for HumanGate** | MVP-1: tracker comment `/approve`. MVP-2: LiveView observation dashboard, but HumanGate approval still flows through the tracker for single-channel evidence. |
| **Multi-tenant operation** | One principal, one engine instance. Multi-tenant is a v3 question. |
| **On-call rotation / PagerDuty** | Single-operator system; no on-call surface. |
| **Metrics to external services** (Datadog, Honeycomb, Prometheus push) | `ObservationsBus` is local ETS. Export is a later concern. |
| **LLM-based work intake or status authority at engine boundary** | Haft owns WorkCommissions and lifecycle state. LLM may write projection prose only after deterministic intent. |
| **Variant generation inside MVP-1** | MVP-1 is single-variant by construction. Pareto lives in MVP-2. |
| **Storage of raw agent transcripts** | Haft gets the PhaseOutcome + evidence refs. Raw transcripts are the adapter's temporary buffer; not persisted beyond Haft's scope. |

---

## How to request additions

If you want something that isn't in a tier or is in the cut list:

1. Read `SYSTEM_CONTEXT.md §6` (product integrity principle).
2. If the proposed feature serves phase-gated AI-agent orchestration and isn't upstream / downstream / already-someone-else's, argue for its tier.
3. If it's in the cut list, argue specifically against the cut rationale.
4. Open-questions go in `OPEN_QUESTIONS.md`, not here.

No un-tiered work. No "just add this quick." The scope is frozen until a revision explicitly moves an item between tiers.
