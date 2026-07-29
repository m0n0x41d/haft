Diagnose the current concrete failure by testing rival explanations.

Retrieve current FPF source first. Inspect a known SourceID or UnitID with
`haft_query(action="fpf", mode="inspect", identifier="...")`; otherwise use
`mode="concern"` with the concrete failure question, then inspect the direct
pattern body. Query does not select a cause or provide runtime evidence.

1. Stabilize the exact symptom and reproduction boundary.
2. Generate distinct causal hypotheses, including one that challenges the
   initial framing.
3. Declare a discriminating observation for each hypothesis.
4. Test read-only probes in parallel where safe.
5. Rank hypotheses by evidence and keep losing rivals visible with their
   return conditions.

Do not confuse a plausible story with runtime evidence. Keep the diagnosis
conversational unless the operator asks to save it or a named receiving use
needs replay; then use the smallest needed ProblemCard or SolutionPortfolio.
