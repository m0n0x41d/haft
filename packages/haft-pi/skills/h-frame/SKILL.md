---
name: h-frame
description: Shape an under-articulated engineering problem without assuming a solution or forcing a project phase. Default to a conversational frame; create a ProblemCard only on explicit save intent or when a named receiving use needs a durable accepted problem statement.
---

# h-frame — Shape the current problem

Retrieve current FPF source first. Use
`haft_query(action="fpf", mode="inspect", identifier="...")` for a known
SourceID or UnitID; otherwise use `mode="concern"` with the operator's query,
then inspect the direct pattern body. Query retrieves candidates only; it does
not choose the governing pattern or justify persistence.

Separate the object from its description and carrier. State the observed
signal, unresolved relation, scope, constraints, acceptance basis, uncertainty,
blast radius, and reversibility. Unpack umbrella words and remove preferred
solutions from the frame.

Call `haft_problem(action="frame")` only for explicit persistence or a named
receiving use. An inline `ProblemCard@Context`-shaped result is not a persisted
artifact. When exact current identity is known, include
`entity_ref={ref_kind_id:"U.EntityRef",reference_id:"..."}` and
`bounded_context_ref`; never invent them from prose. A committed typed
projection returns an exact `record_reference`. Without that basis, preserve
the card and report its projection as underdetermined. `spec_fit_probe` is
advisory and only relevant when a named spec relation is current. Stop with the
frame; do not prescribe exploration next.
