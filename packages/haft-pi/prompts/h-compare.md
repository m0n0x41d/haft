Compare portfolio variants fairly and return a Pareto front, not a scalar
winner.

Before substantive comparison, call `haft_query`:

```json
{
  "action": "pattern_use",
  "mode": "compact",
  "query": "<operator concern>"
}
```

Skip only mechanical/status/exact-lookup requests where no FPF pattern choice is
material. If `should_use_pattern=true` and comparison needs output-shape
detail, ask for `mode="full"` before applying the returned pattern. PatternUse
is advisory/read-only: not approval, not evidence, not a DecisionRecord, not a
WorkCommission, not MethodPack, and not a gate. Do not inline the FPF catalog
or route list in this prompt.

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
