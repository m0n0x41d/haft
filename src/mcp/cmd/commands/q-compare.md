---
description: "Compare solution variants fairly on the Pareto front"
---

# Compare Solutions

Run a parity comparison across variants using defined dimensions. Identifies the Pareto front (non-dominated set).

Use `quint_solution` tool with `action="compare"` and:
- `portfolio_ref`: SolutionPortfolio ID (auto-detected if only one active)
- `dimensions`: comparison dimension names
- `scores`: scores per variant — `{"V1": {"throughput": "100k/s", "cost": "$200"}}`
- `non_dominated_set`: variant IDs on the Pareto front (REQUIRED)
- `policy_applied`: selection policy that was used
- `selected_ref`: recommended variant ID

State the selection policy BEFORE seeing results. Ensure parity — same inputs, same scope across all options.

## After the tool call — what to show the user

The user needs to understand the reasoning, not just see a grid of scores. Do NOT compress into a table alone.

Present:
1. **Score table** — all dimensions x variants, for quick scanning
2. **Per-dimension justification** — for each dimension, briefly explain WHY each variant scored the way it did. "Medium-High" means nothing without "because every whitespace change triggers a flag"
3. **Elimination reasoning** — for each dominated variant, state which variant dominates it and on which dimensions. The user should understand WHY a variant was eliminated, not just that it was
4. **Pareto front analysis** — for non-dominated variants, explain the trade-off: what does each sacrifice for what it gains?
5. **Recommendation rationale** — which variant you recommend, the decisive dimension(s), and what risk the user accepts by choosing it

If there are comparison warnings (missing dimensions, expired measurements, parity violations), surface them — these are decision-quality signals.

$ARGUMENTS
