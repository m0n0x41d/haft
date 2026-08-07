---
name: h-explore
description: Generate 3-5 genuinely distinct candidate approaches for a current question, with the weakest link of each visible. May work from an inline question or a durable ProblemCard; persistence is conditional.
---

# h-explore — Keep alternatives live

Retrieve current FPF source with `mode="inspect"` for a known SourceID/UnitID,
or `mode="concern"` for the exploration question. Inspect the direct pattern
body; retrieval does not choose or rank a candidate.

Treat a concern result's `candidate_set` as incomplete navigation. Before
relying on one candidate, inspect its exact identifier and direct pattern
body. Keep several candidates live or abstain when the basis is insufficient.
Never query after exploration merely to manufacture support for candidates
already chosen.

Generate candidates that differ in kind, not only degree. For each state its
novelty marker, strengths, risks, weakest link, and any defensible stepping-
stone role. Keep unresolved branches and return conditions visible.

Return candidates conversationally by default. Call
`haft_solution(action="explore")` only on explicit save intent or when current
Work supplies a concrete operator-named or agent-inferred receiving use. A
typed durable portfolio needs one independently addressable
ProjectRecord per option: persist candidate-description Notes under the same
exact concern, keep each returned `Haft.ProjectRecordRef`, and pass it as the
variant's `project_record_ref`. Do not derive or invent record IDs. Without
exact option refs, preserve the legacy portfolio but report typed projection
as underdetermined. Do not invent a ProblemCard to satisfy a sequence and do
not prescribe comparison next.
