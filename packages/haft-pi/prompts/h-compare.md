Compare two or more existing candidates fairly.

Retrieve current FPF source first. Use
`haft_query(action="fpf", mode="inspect", identifier="...")` for a known
SourceID or UnitID; otherwise call `haft_query(action="fpf",
mode="concern", query="<comparison question>")`, then inspect the direct
pattern body. Query does not select a governing pattern, rank the options, or
provide comparison evidence.

Treat a concern result's `candidate_set` as incomplete navigation. Before
relying on one candidate, inspect its exact identifier and direct pattern
body. Keep several candidates live or abstain when the basis is insufficient.
Never query after comparison merely to manufacture support for a verdict
already reached.

1. Declare the characteristic space, indicator roles, parity basis,
   missing-data policy, and selection policy before scoring.
2. Treat constraints as gates, targets as the few optimized dimensions, and
   observations as watched but not optimized.
3. Apply one scale and evidence window to every candidate per dimension.
4. Show dominated candidates, the non-dominated set, trade-offs, uncertainty,
   weakest links, and what new evidence could change the result.

This n-candidate comparison is Haft local practice, not automatically an
`A.19.CPM` actual application or an `A.19.SelectorMechanism` result. Claim exact
FPF conformance only when the current use also preserves each required binary
CPM application, pair coverage, token-to-producer trace, claim scope and
selected context slices, predicate basis, reference plane, evaluation window,
and separate eligibility/output bindings recovered from the direct source.

Do not collapse the comparison into one magic score. Work from inline
candidates or a portfolio; `h-frame` and `h-explore` are not prerequisites.
Persist characterization/comparison only on explicit save intent or when a
concrete operator-named or agent-inferred receiving use supplied by current
Work needs replay. For typed persistence, use the exact
`U.EntityRef`, bounded context, projected portfolio, and returned
`Haft.ProjectRecordRef` values for every compared option. Never derive record
references from artifact IDs. Preserve the comparison `record_reference`
returned by the kernel.

Do not send the legacy `selected_ref` for an ordinary typed comparison. The
typed `Haft.PortfolioComparison` excludes it: a non-dominated set is not a
winner, recommendation, ChoiceResult, or binding choice. Only explicit manual
`h-decide` may record a binding choice.
