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

$ARGUMENTS
