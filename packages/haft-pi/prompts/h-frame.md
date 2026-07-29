Shape the current problem without turning it into a universal project phase.

Retrieve current FPF source first. For a known SourceID or UnitID, call
`haft_query(action="fpf", mode="inspect", identifier="...")`; otherwise call
`haft_query(action="fpf", mode="concern", query="<operator problem
signal>")`, then inspect the direct pattern body. Query only retrieves source
candidates; it does not select a governing pattern or authorize persistence.

- Separate the affected object from its description and carrier.
- State the observed signal, unresolved relation, scope, constraints,
  acceptance basis, uncertainty, blast radius, and reversibility.
- Unpack umbrella words such as `quality`, `done`, or `scalable` into
  observable claims.
- Do not smuggle a preferred solution into the frame.

Return a conversational frame by default. It may have the source
`ProblemCard@Context` shape without being a persisted Haft artifact. Call
`haft_problem(action="frame")`
only when the operator asks to save it or a named receiving use needs a stable
ProblemCard. When exact current identity is known, supply
`entity_ref={ref_kind_id:"U.EntityRef",reference_id:"..."}` and
`bounded_context_ref`; do not infer either from the title. Preserve a returned
`record_reference`, or report the exact missing basis when typed projection is
underdetermined. Use `spec_fit_probe` only when a named spec relation is
current; it is advisory, not approval, evidence, baseline, or a universal
prerequisite.

Stop with the frame. Do not prescribe `h-explore` as the next project step.
