---
title: "Open-Sleigh — Specifications Index"
version: v0.3 (post-SPEC.md thinning; spec-set v0.7)
date: 2026-04-22
status: SPEC.md is umbrella index; specs/ is canonical for all normative content
valid_until: 2026-05-20
---

# Open-Sleigh — Specifications

> **FPF note.** `../SPEC.md` is the **operator-facing umbrella index** —
> a short, narrative one-pager that points into this tree. These
> documents are the **canonical normative content**: engine behaviour,
> protocol contracts, config schema, type-level invariants, illegal
> states, and architectural layering all live here. No section is
> maintained in two places.
>
> Everything here is a **Description** of the target system (running
> engine) and its enabling system (code). Do not confuse: a type
> definition is a description, a passing test is evidence, a production
> run is reality.

## Reading order

### Target system — what Open-Sleigh is

| # | Document | What it covers |
|---|----------|---------------|
| 1 | [target-system/SYSTEM_CONTEXT.md](target-system/SYSTEM_CONTEXT.md) | Why Open-Sleigh exists, what changes in the world, role, supersystem, project roles |
| 2 | [target-system/TERM_MAP.md](target-system/TERM_MAP.md) | Canonical vocabulary — one meaning per term |
| 3 | [target-system/SCOPE_FREEZE.md](target-system/SCOPE_FREEZE.md) | MVP-1 / MVP-1.5 / MVP-2 / later / cut |
| 4 | [target-system/PHASE_ONTOLOGY.md](target-system/PHASE_ONTOLOGY.md) | 5-axis phase ontology: operational phase, gate kind, verdict, authoring role, run-attempt sub-state (with retry policy) |
| 5 | [target-system/TARGET_SYSTEM_MODEL.md](target-system/TARGET_SYSTEM_MODEL.md) | Engine entities: Session, Ticket, PhaseOutcome, Workflow, Evidence, HumanGateApproval, AdapterSession, SleighConfig, PhaseConfig |
| 6 | [target-system/ILLEGAL_STATES.md](target-system/ILLEGAL_STATES.md) | States that must be impossible. Four-label enforcement taxonomy (type-level / constructor-level / runtime-guard / CI-or-module-graph-check) |
| 7 | [target-system/OPEN_QUESTIONS.md](target-system/OPEN_QUESTIONS.md) | Live open questions (+ v0.5 resolutions kept for auditability) |
| 8 | [target-system/AGENT_PROTOCOL.md](target-system/AGENT_PROTOCOL.md) | Normative JSON-RPC contract for `Agent.Adapter`: handshake, continuation turns, events, error taxonomy. Modeled on Symphony §10 with FPF divergences |
| 9 | [target-system/GATES.md](target-system/GATES.md) | Gate catalogue: structural + semantic + human; LLM-judge calibration (CHR-04 tuple, golden sets, versioning); observation indicators (anti-Goodhart) |
| 10 | [target-system/HAFT_CONTRACT.md](target-system/HAFT_CONTRACT.md) | MCP contract: transport, tool surface, SPOF failure mode + WAL replay, token-accounting isolation, L4/L5 ownership seam, cancellation protocol |
| 11 | [target-system/WORKSPACE.md](target-system/WORKSPACE.md) | Per-ticket workspace lifecycle, hook kinds, trust posture, bootstrap-only guidance, startup terminal cleanup |
| 12 | [target-system/ADAPTER_PARITY.md](target-system/ADAPTER_PARITY.md) | Agent Adapter Parity Plan (CHR-09) — declared BEFORE a second adapter ships. MVP-1.5 time-box with revert criteria |
| 13 | [target-system/SLEIGH_CONFIG.md](target-system/SLEIGH_CONFIG.md) | `sleigh.md` schema with example, config-hash formula + freeze semantics, size budget, immutability rules |
| 14 | [target-system/RISKS.md](target-system/RISKS.md) | Accepted risks + mitigations: bootstrap-risk, in-RAM state loss, tracker-vs-engine race, probabilistic LLM-judge |
| 15 | [target-system/HTTP_API.md](target-system/HTTP_API.md) | Optional read-only HTTP observability API (Symphony-inherited). Never exposes Haft artifacts |

### Enabling system — how we build it

| # | Document | What it covers |
|---|----------|---------------|
| 16 | [enabling-system/FUNCTIONAL_ARCHITECTURE.md](enabling-system/FUNCTIONAL_ARCHITECTURE.md) | 6-layer hierarchy (L1–L6), dependency rules, inexpressibility per layer, compilation chain |
| 17 | [enabling-system/STACK_DECISION.md](enabling-system/STACK_DECISION.md) | Elixir/OTP choice, Haft MCP, storage model, rationale |
| 18 | [enabling-system/IMPLEMENTATION_PLAN.md](enabling-system/IMPLEMENTATION_PLAN.md) | Layered build order for MVP-1 with test gates per layer; canary sequencing; revert checkpoints |
| 19 | [enabling-system/REFERENCE_ALGORITHMS.md](enabling-system/REFERENCE_ALGORITHMS.md) | Non-normative pseudocode skeletons for service startup, poll tick, dispatch, worker loop, orchestrator message handling |
| 20 | [enabling-system/PRODUCT_TODO.md](enabling-system/PRODUCT_TODO.md) | Living product-readiness backlog: done work, P0 canary path, reliability, packaging, provider expansion |

### Agents

| # | Document | What it covers |
|---|----------|---------------|
| 21 | [agents/prompts/5.4-pro-review.md](agents/prompts/5.4-pro-review.md) | **Full** pre-code review prompt for GPT-5.4 Pro. Use for major revisions |
| 22 | [agents/prompts/5.4-pro-confirmation.md](agents/prompts/5.4-pro-confirmation.md) | Short confirmation-pass prompt: verify v0.4 CRITICALs closed + regression check from v0.5/v0.6 revisions |

## Relationship to `../SPEC.md`

`../SPEC.md` is the **operator-facing umbrella index** — short,
narrative, one screen per section, with 1–2 paragraph abstracts that
point into this tree for normative content. It retains project-level
content that has no natural home in the decomposition:

- project identity and MVP narrative (§1)
- influences and non-inheritance (§2)
- a one-screen core-abstractions pointer table (§3)
- the lemniscate overview diagram (§4)
- the resolved-questions audit trail for pre-v0.5 versions (§11)
- the full revision log

**No section is maintained in two places.** When SPEC.md and a
`specs/` document describe the same concept, SPEC.md carries a short
abstract and a pointer; the `specs/` document carries the full
content. This replaces the earlier v0.2 split that separated "engine
behaviour" (SPEC.md) from "type-level invariants / architectural
layering" (specs/) — both are `Description` of the same `Object` and
the split created drift.

If you find content in SPEC.md that reads as authoritative normative
prose rather than a pointer abstract, that is a review finding —
migrate to the appropriate `specs/` document and leave the abstract
behind.
