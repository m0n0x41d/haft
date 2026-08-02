Generate 3-5 genuinely distinct candidates for the current question.

Retrieve current FPF source first. Inspect a known SourceID or UnitID with
`haft_query(action="fpf", mode="inspect", identifier="...")`; otherwise use
`mode="concern"` with the exploration question, then inspect the direct pattern
body. Retrieval does not choose or rank a candidate.

Treat a concern result's `candidate_set` as incomplete navigation. Before
relying on one candidate, inspect its exact identifier and direct pattern
body. Keep several candidates live or abstain when the basis is insufficient.
Never query after exploration merely to manufacture support for candidates
already chosen.

- Work from an inline question, cue, accepted problem, or ProblemCard; do not
  invent a ProblemCard merely to satisfy a sequence.
- Make variants differ in kind, not only degree.
- Give every variant a novelty marker, strengths, risks, and an explicit
  weakest link. Keep a stepping stone only when it opens later search space.
- Keep unresolved alternatives and return conditions visible.

Return candidates conversationally by default. Persist with
`haft_solution(action="explore")` only on explicit save intent or when current
Work supplies a concrete operator-named or agent-inferred receiving use that
needs a durable portfolio. If the kernel needs a ProblemCard for
that durable call and none exists, ask whether to materialize one; do not
fabricate it.

For a typed portfolio, first save one non-binding candidate-description Note
per option under the same exact `entity_ref` and `bounded_context_ref`. Pass
each returned exact `Haft.ProjectRecordRef` as that variant's
`project_record_ref`. Never derive a record ID from an artifact ID. If exact
option records are unavailable, retain the legacy portfolio but report typed
projection as underdetermined. Do not prescribe comparison as the next project
phase.
