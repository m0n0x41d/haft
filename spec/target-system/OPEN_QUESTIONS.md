# Open Questions

> Non-blocking for v6 release. Classified by v7 impact after GPT-5.4 spec review.

## Decided (formerly blocking v7)

| # | Question | Decision | Rationale |
|---|----------|----------|-----------|
| Q1 | Workflow DSL shape | **`.haft/workflow.md` as hybrid markdown + structured YAML block.** Human-readable prose + parseable defaults + path policies. Not a workflow engine. | Pure markdown = too hard to validate. Pure YAML = config fatigue. FPF flow card = too heavy for solo dev. Hybrid gives reviewable prose + parseable structure + low implementation burden. Ships in v6.1. |
| Q2 | Fate of `haft agent` | **Transitional. Retained for CI/headless/power-user. Not co-equal strategic surface.** Desktop = primary interactive. MCP = primary embedded. Agent = secondary headless utility. No feature-parity promise with desktop. | Desktop subsumes interactive use. But CI stakeholder needs headless mode. SSH/remote use cases real. Removing entirely loses too much utility. |
| Q10 | L2 enforcement scope | **Both JSON Schema + Go validators, but narrow: parity minimums + subjective dimension operationalization only.** No broad A.6 universalization before v7. | Schema alone can't catch semantic hollowness. Go validators without schema = brittle text heuristics. Narrow scope = feasible for solo dev. Ships in v6.1. |

## Resolved (No Longer Open)

| # | Question | Resolution |
|---|----------|-----------|
| Q7 | Should the GitHub repo rename from quint-code to haft? | Rename completed in v6.0.0. GitHub redirects old URLs. Migration task, not design question. |
| Q8 | How to handle install.sh URL after rename? | install.sh updated to install `haft` binary. `quint.codes` domain stays. Operational task. |

## Deferred (Not Blocking v7)

| # | Question | Context | When relevant |
|---|----------|---------|--------------|
| Q3 | Multi-project governance views in desktop? | Per-project tabs exist. Cross-project portfolio view doesn't. | When multi-project users request it |
| Q4 | When to include research-before-code lane? | Some tasks need external knowledge. | Only if deep mode or compliance tasks enter v7 default execution loop |
| Q5 | Autonomy budget granularity? | Currently binary (on/off). FPF E.16 describes typed budgets. | When host agent permissions prove insufficient |
| Q6 | How to measure semio-quality? | No benchmark for fanout, authority confusion, gate/evidence mixing. | When L2 language precision ships |
| Q9 | Is "keeps the coder honest" the right tagline? | Validated in research. Slightly adversarial. Doesn't reflect verification/memory. | Next marketing pass |
| Q11 | Role/context-scoped skills vs global skill library? | FPF suggests role+context binding. Hermes uses global. | When demand signal exists |

## Project Harnessability Questions

These are now product-shaping questions for the spec-first harness direction.

| # | Question | Current stance | Review need |
|---|----------|----------------|-------------|
| Q17 | Are large formal target/enabling specs acceptable as product ceremony? | Yes. They are the price of making arbitrary projects harnessable. UX should make depth navigable, not hide it. | Validate whether this is a coherent market/category choice and whether the acceptance burden is framed honestly. |
| Q18 | What is the minimum strict markdown schema for SpecSections? | YAML block under stable markdown heading: id, kind, statement_type, owner, status, valid_until, terms, evidence_required. | Check if this is enough for parse/check/coverage without over-designing a DSL. |
| Q19 | Should TargetSystemSpec readiness gate EnablingSystemSpec readiness? | Yes. Enabling mechanics must not define target purpose. | Challenge edge cases: brownfield projects with strong repo architecture but weak product framing. |
| Q20 | Does SpecCoverage belong in the same graph as DecisionCoverage? | Yes as a higher-order derived graph: spec -> decision -> commission -> run -> evidence -> code/test. | Validate persistence frontier: edges vs derived views vs markdown carriers. |
| Q21 | Should broad YOLO/harness execution require spec readiness? | Yes by default. Tactical explicit override may exist, but must record an out-of-spec commission reason. | Review whether this balances product rigor and early adoption. |

## Open-Sleigh Integration Review Questions

## Review Evidence Received — 2026-04-22

The external FPF-aligned review supports the direction but identifies scope
enforcement and projection validation as the weakest links.

| Question | Review disposition | Follow-up |
|----------|--------------------|-----------|
| Q12 WorkCommission boundary | Correct boundary between DecisionRecord and RuntimeRun | Implement Scope as a hard authorization object, not prose metadata. |
| Q13 ImplementationPlan persistence | Hybrid recommended | Human principal should confirm artifact-vs-internal frontier before implementation. |
| Q14 ExternalProjection persistence | Hybrid recommended | Persist intent/drift/debt/outcome in Haft; keep connector retries/cursors internal. |
| Q15 ProjectionWriterAgent validation | Must be deterministic and closed before live publish | Start with deterministic templates; keep LLM writer disabled in first canary. |
| Q16 Minimum canary | Two canaries, not one | Green local-only path + stale/snapshot-block path before external projection. |

| # | Question | Context | Review need |
|---|----------|---------|-------------|
| Q12 | Is `WorkCommission` the right boundary between DecisionRecord and RuntimeRun? | We need decisions to wait safely before execution, and block if stale before work starts. | Review says yes; implementation now depends on hard Scope and snapshot enforcement. |
| Q13 | Should `ImplementationPlan` be a first-class artifact? | YOLO/batch mode needs DAG, dependencies, locksets, and envelope. | Decide whether plan is governance artifact, scheduler record, or both. |
| Q14 | Should ExternalProjection be persisted as an artifact or internal sync record? | Linear/Jira/GitHub projections are optional carriers for external observers. | Balance auditability against artifact graph noise. |
| Q15 | How strong should ProjectionWriterAgent validation be for manager-language text? | LLM writes low-formalism text, but cannot invent facts or status. | Identify minimum deterministic validator before first real tracker publish. |
| Q16 | What is the minimum live canary for commission-first Open-Sleigh? | Current Open-Sleigh is tracker-first. | Define the smallest E2E that proves Haft-first work intake, preflight, evidence, and optional projection. |
