---
name: h-compare
description: Compare two or more existing candidates under an explicit characteristic space, parity basis, and predeclared selection policy. Return trade-offs and a non-dominated set; persistence is conditional and binding choice remains manual.
---

# h-compare — Compare under parity

Retrieve current FPF source first. Inspect a known SourceID or UnitID with
`haft_query(action="fpf", mode="inspect", identifier="...")`; otherwise use
`mode="concern"` with the comparison question and inspect the direct pattern
body. Query returns source candidates, not a selected governing pattern or
verdict.

Declare dimensions, indicator roles, parity, missing-data policy, and selection
policy before scoring. Apply the same evidence window and scale to all
candidates per dimension. Show constraints, dominated candidates, the Pareto
front, uncertainty, weakest links, and evidence that could change the result.

Inline candidates are sufficient; frame and explore are not prerequisites.
Persist only for explicit save intent or a named receiving use. A durable
typed comparison requires an exact `U.EntityRef`, bounded context, an already
projected portfolio, and independently addressable option
`Haft.ProjectRecordRef` values. Preserve returned `record_reference` values;
never derive them from artifact IDs. Missing references make typed projection
underdetermined while the legacy carrier remains durable.

Do not use the legacy `selected_ref` for an ordinary typed comparison. It is
excluded from `Haft.PortfolioComparison`: a Pareto front is not a winner,
ChoiceResult, recommendation, or binding decision.
