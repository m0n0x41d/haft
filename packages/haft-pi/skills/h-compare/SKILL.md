---
name: h-compare
description: Compare two or more existing candidates under an explicit characteristic space, parity basis, and predeclared selection policy. Return trade-offs and a non-dominated set; persistence is conditional and a binding choice requires a direct unambiguous operator request.
---

# h-compare — Compare under parity

Retrieve current FPF source first. Inspect a known SourceID or UnitID with
`haft_query(action="fpf", mode="inspect", identifier="...")`; otherwise use
`mode="concern"` with the comparison question and inspect the direct pattern
body. Query returns source candidates, not a selected governing pattern or
verdict.

Treat a concern result's `candidate_set` as incomplete navigation. Before
relying on one candidate, inspect its exact identifier and direct pattern
body. Keep several candidates live or abstain when the basis is insufficient.
Never query after comparison merely to manufacture support for a verdict
already reached.

Declare dimensions, indicator roles, parity, missing-data policy, and selection
policy before scoring. Apply the same evidence window and scale to all
candidates per dimension. Show constraints, dominated candidates, the Pareto
front, uncertainty, weakest links, and evidence that could change the result.

This n-candidate comparison is Haft local practice, not automatically an
`A.19.CPM` actual application or an `A.19.SelectorMechanism` result. Claim exact
FPF conformance only when the current use also preserves each required binary
CPM application, pair coverage, token-to-producer trace, claim scope and
selected context slices, predicate basis, reference plane, evaluation window,
and separate eligibility/output bindings recovered from the direct source.

Inline candidates are sufficient; frame and explore are not prerequisites.
Persist only for explicit save intent or a concrete operator-named or
agent-inferred receiving use supplied by current Work. A durable
typed comparison requires an exact `U.EntityRef`, bounded context, an already
projected portfolio, and independently addressable option
`Haft.ProjectRecordRef` values. Preserve returned `record_reference` values;
never derive them from artifact IDs. Missing references make typed projection
underdetermined while the legacy carrier remains durable.

Do not use the legacy `selected_ref` for an ordinary typed comparison. It is
excluded from `Haft.PortfolioComparison`: a Pareto front is not a winner,
ChoiceResult, recommendation, or binding decision.
