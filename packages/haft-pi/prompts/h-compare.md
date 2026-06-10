Compare portfolio variants fairly and return a Pareto front, not a scalar
winner.

1. Characterize FIRST — declare dimensions before any scoring, via the
   native `haft_problem` tool:

```json
{
  "action": "characterize",
  "problem_ref": "<prob-...>",
  "dimensions": [{ "name": "...", "scale_type": "ordinal", "unit": "1-5", "polarity": "higher_better", "role": "target", "how_to_measure": "...", "valid_until": "<ISO date>" }]
}
```

   Tag each dimension's role: constraint (hard limit) | target (optimize,
   1-3 max) | observation (watch, never optimize — anti-Goodhart).
2. Declare the parity plan and selection policy BEFORE scoring: same
   evidence set, same budgets/windows for every variant, explicit
   missing-data policy.
3. Score dimension-wise: one scale applied across ALL variants per
   dimension (prevents anchoring on a favorite variant).
4. Persist via `haft_solution(action="compare", portfolio_ref=..., parity_plan=..., scores=...)`.
5. Present the Pareto front with trade-offs. Never collapse to one number.
6. Binding choice is manual: recommend a variant with rationale, then tell
   the operator it needs their explicit /h-decide. Stop there.
