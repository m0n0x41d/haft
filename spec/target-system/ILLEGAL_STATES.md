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
| 7 | Two active DecisionRecords for the same ProblemCard | One problem → one active decision. Previous must be superseded. | Not currently enforced. **Gap: should be.** |
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

## Known Gaps (not yet enforced)

| # | Gap | Impact | Priority |
|---|-----|--------|----------|
| G1 | #7: Multiple active decisions per problem | Poisons invariant injection, file-to-decision lookup, governance scans, and "what governs this file?" | **High** — must enforce before v7 execution loop |
| G2 | #4: Parity declaration enforcement | Unfair comparisons pass without warning | **High** — planned for v6.x |
| G3 | #10: Measurement fabrication detection | Trust inflation | Low — fundamentally LLM discipline issue |
| G4 | #19: Subjective dimension enforcement | Entire Choose mode can look rigorous while being semantically hollow. Core value proposition corrupted. | **High** — L2 enforcement needed before v7 |
