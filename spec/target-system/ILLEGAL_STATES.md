# Illegal States

> Reading order: 7 of N. Read after EVIDENCE_ONTOLOGY.
>
> States that must be unrepresentable in the system. Each entry: what's illegal, why, how enforced.

## Artifact Graph

| # | Illegal state | Why | Enforcement |
|---|--------------|-----|-------------|
| 1 | DecisionRecord without problem_ref or portfolio_ref | Decisions must trace to a framed problem. Orphan decisions have no framing context. | `Decide()` auto-resolves active problem/portfolio. Tactical mode allows implicit link. |
| 2 | SolutionPortfolio with <2 variants | Comparison requires alternatives. One variant = no comparison. | `Explore()` validates variant count. |
| 3 | SolutionPortfolio with variants that have >50% word overlap | Disguised copies are not genuine alternatives. | Diversity check warns at >50%. Not hard-blocked (user may override). |
| 4 | Comparison without parity declaration | Comparison without same-conditions statement is invalid. | Skill instructions require parity. L2 enforcement planned. |
| 5 | DecisionRecord with status `active` and no `selected_title` | A decision must have chosen something. | `Decide()` requires selected_title. |
| 6 | EvidencePack with verdict `supports` and CL0 | Evidence from opposed context cannot support — inadmissible, not merely weak. | **Enforced:** reject at ingest or downgrade verdict to `weakens` before storage. CL0+supports must not enter R_eff computation. |
| 7 | Two active DecisionRecords for the same ProblemCard | One problem → one active decision. Previous must be superseded. | **Enforced:** `Decide()` rejects a new live DecisionRecord when another live decision already governs the same problem. |
| 8 | Note with >70% title word overlap with active DecisionRecord | Note duplicates an existing decision. | `haft_note` rejects at >70% overlap. Warns at 50-70%. |
| 9 | Artifact with status `addressed` that is not a ProblemCard | Only problems can be "addressed." Other artifacts use superseded/deprecated. | `close` action only on ProblemCard kind. |

## Evidence & Trust

| # | Illegal state | Why | Enforcement |
|---|--------------|-----|-------------|
| 10 | Measurement recorded without running actual verification | Calling `measure` from memory is fabricated evidence. | Skill instructions prohibit. No runtime enforcement (LLM discipline). |
| 11 | Measurement without baseline claiming CL3 | Without baseline, there's no reference state. Self-evidence at best. | `Measure()` degrades to CL1 when no baseline exists. |
| 12 | R_eff computed as average of evidence scores | WLNK principle: system reliability = min, not average. | `ComputeWLNKSummary()` uses min. |
| 13 | Superseded evidence counted in R_eff | Old measurements would drag R_eff down permanently. | `ComputeWLNKSummary()` excludes superseded items. |
| 14 | Evidence with expired valid_until scored at full strength | Decayed evidence is weak, not fresh. | Expired evidence scores 0.1. |

## Lifecycle

| # | Illegal state | Why | Enforcement |
|---|--------------|-----|-------------|
| 15 | Deprecated/superseded artifact appearing in active queries | Dead artifacts pollute the working set. | `ListActiveByKind()` filters by status=active. `/h-status` excludes deprecated/superseded notes. |
| 16 | Reopened decision without lineage to original | Reopen must trace back so context isn't lost. | `Reopen()` creates new ProblemCard with link to original. |
| 17 | Waive without justification | Extending validity without reason is rubber-stamping. | `Waive()` requires reason string. |

## Modes & Workflow

| # | Illegal state | Why | Enforcement |
|---|--------------|-----|-------------|
| 18 | Agent auto-executing Execute mode without human confirmation | Transformer Mandate: human decides at Choose→Execute boundary. | Skill instructions enforce pause. Exception: autonomous mode explicitly enabled. |
| 19 | Comparison with subjective dimensions not operationalized | "Maintainable" means nothing until decomposed into measurables. | Language precision triggers in skill (L1). L2 enforcement planned. |
| 20 | Constraint dimension scored instead of eliminating | Constraints are hard limits. Violating variants must be removed, not penalized. | `computeParetoFront()` eliminates constraint violations before dominance. |

## Persistence

| # | Illegal state | Why | Enforcement |
|---|--------------|-----|-------------|
| 21 | .haft/*.md projection out of sync with database | Projections are derived. Stale projections mislead team members. | `WriteFile()` called on every artifact create/update. `haft sync` reconciles. |
| 22 | Binary database file (.db) in git | Binary files can't merge. Defeats team collaboration. | `.gitignore` excludes .db files. Database lives in `~/.haft/`. |
| 23 | Artifact insert without transaction (partial state) | Link failure after insert → orphaned artifact. | `Create()` wraps insert + links in single transaction. |

## Authority & Lifecycle

| # | Illegal state | Why | Enforcement |
|---|--------------|-----|-------------|
| 24 | SQLite and `.haft/*.md` projection disagree on artifact content | Dual-truth corrupts team workflow. SQLite is runtime authority; projections are exchange format. | `WriteFile()` regenerates projection on every create/update. `haft sync` is explicit reconcile, fails closed on schema mismatch. |
| 25 | Derived phase (Pending/Shipped/Stale) stored in database | Phases are computed from status + evidence state. Storing them creates stale-view bugs. | Phases computed at query time only. Never written to artifacts table. |
| 26 | Advisory recommendation (`selected_ref`) treated as human choice in delegated reasoning | Violates Transformer Mandate. Agent recommends; human confirms before `/h-decide`. | Skill instructions enforce pause at Choose→Execute boundary. NavStrip shows "Available: /h-decide" not "Executing: /h-decide". |

## Work Execution & External Projection (vNext)

| # | Illegal state | Why | Enforcement |
|---|--------------|-----|-------------|
| 27 | RuntimeRun without a WorkCommission | Execution must be authorized separately from the decision. | Runner API accepts `commission_id`, not free-form decision/task text. |
| 28 | WorkCommission running while linked DecisionRecord is stale, superseded, deprecated, or hash-mismatched | A commission cannot extend the life of the decision that authorized it. | Mandatory preflight freshness gate before `running`. |
| 29 | WorkCommission start without an exclusive lease | Two runners can duplicate work or race on the same scope. | Atomic `claim_for_preflight` / `start_after_preflight` operation. |
| 30 | Two running WorkCommissions with overlapping locksets under one ImplementationPlan | Parallel agents will create avoidable merge conflicts and invalid evidence. | Scheduler lockset conflict check before lease grant. |
| 31 | YOLO/AutonomyEnvelope skipping freshness, evidence, lease, lockset, or one-way-door gates | YOLO is continuation policy, not authority expansion. | Envelope only controls auto-advance; gates remain unconditional. |
| 32 | Agent expands an AutonomyEnvelope beyond its approved repos/paths/actions/risk ceiling | The runner would self-author authority. | Any out-of-envelope need moves commission to `needs_human_review`. |
| 33 | ExternalProjection treated as WorkCommission/DecisionRecord authority | Linear/Jira/GitHub are carriers; their status changes are not evidence. | External observed state records drift/conflict only. Haft status changes require Haft evidence/actions. |
| 34 | ProjectionWriterAgent deciding status, severity, scope, owner, deadline, or completion | LLM writer is prose transformation only. Truth is deterministic Haft state. | ProjectionIntent carries facts; ProjectionValidation rejects invented/missing claims. |
| 35 | External projection required for local execution correctness | Haft must remain local-first and usable without tracker credentials. | `projection_policy=local_only` is a first-class mode; connector failure cannot invalidate RuntimeRun evidence. |
| 36 | RuntimeRun mutates outside WorkCommission Scope but inside the allowed workspace | Workspace safety is not commission authority. An agent can stay in the repo while editing the wrong slice. | Scope carried in Session/AdapterSession; every mutating adapter call checks Scope; terminal diff validation hard-fails `mutation_outside_commission_scope`. |
| 37 | Human approval reused after CommissionSnapshot drift | Approval applies to the exact decision/scope/base/envelope/plan state the human saw. Reusing it after drift silently expands authority. | HumanGateApproval references the snapshot hash; any snapshot mismatch invalidates approval and requires re-preflight/re-approval. |
| 38 | ImplementationPlan revision changes after WorkCommission lease without revalidation | Batch/YOLO dependencies, locksets, and ordering assumptions may be stale. | `start_after_preflight` compares plan revision; mismatch releases or blocks the lease before Execute. |
| 39 | `external_required` WorkCommission reaches terminal external-closed state with failed/missing publication and no ProjectionDebt | Execution evidence and external carrier closure are different facts. The system needs an explicit debt state. | Successful local evidence may produce `completed_with_projection_debt`; external closed requires debt resolution. |
| 40 | Base SHA or admitted repo context changes after queueing without deterministic re-preflight | The selected DecisionRecord may still be active while the code context changed underneath it. | CommissionSnapshot includes base SHA; preflight compares current repo context and blocks/re-preflights on mismatch. |
| 41 | Tracker terminal state surfaced as completion without adjacent Haft evidence state | External carrier state is a dangerous proxy for work truth. | Dashboards/projections render external state as carrier state and include Haft evidence/completion state next to it. |

## Known Gaps (not yet enforced)

| # | Gap | Impact | Priority |
|---|-----|--------|----------|
| G2 | #4: Parity declaration enforcement | Unfair comparisons pass without warning | **High** — planned for v6.x |
| G3 | #10: Measurement fabrication detection | Trust inflation | Low — fundamentally LLM discipline issue |
| G4 | #19: Subjective dimension enforcement | Entire Choose mode can look rigorous while being semantically hollow. Core value proposition corrupted. | **High** — L2 enforcement needed before v7 |
