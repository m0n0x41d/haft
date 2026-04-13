---
description: "Compare solution variants fairly on the Pareto front"
---

# Compare Solutions

Run a parity comparison across variants using defined dimensions. Identifies the Pareto front (non-dominated set).

## Fit in the canonical interaction matrix

- **Direct response / direct action**: do not use `/h-compare`; answer directly or do the packaging task.
- **Research / prepare-and-wait**: gather context and stop; the human may invoke `/h-compare` later when ready.
- **Delegated reasoning**: use `/h-compare` after frame/explore, present the Pareto front and recommendation, then stop for the human's choice.
- **Autonomous execution**: use the same comparison standard, but continue into `/h-decide` and implementation only when full autonomy is explicitly enabled.

Use `haft_solution` tool with `action="compare"` and:
- `portfolio_ref`: SolutionPortfolio ID (auto-detected if only one active)
- `dimensions`: comparison dimension names
- `scores`: scores per variant — `{"V1": {"throughput": "100k/s", "cost": "$200"}}`
- `non_dominated_set`: variant IDs on the Pareto front (REQUIRED)
- `dominated_variants`: persisted elimination notes — `[{"variant":"V3","dominated_by":["V1"],"summary":"Lower throughput with no compensating cost or simplicity win."}]`
- `pareto_tradeoffs`: persisted frontier trade-off notes — `[{"variant":"V1","summary":"Best throughput, but highest ops cost."}]`
- `policy_applied`: selection policy that was used
- `selected_ref`: advisory recommendation variant ID. In delegated reasoning this is not the human's selection; it is the candidate you recommend the human consider.
- `recommendation_rationale`: why that advisory recommendation wins under the declared policy

State the selection policy BEFORE seeing results. Ensure parity — same inputs, same scope across all options.

## After the tool call — what to show the user

The user needs to understand the reasoning, not just see a grid of scores. Do NOT compress into a table alone.
Do NOT jump from the score grid straight to "pick X".

Present:
1. **Score table** — all dimensions x variants, for quick scanning
2. **Per-dimension justification** — for each dimension, briefly explain WHY each variant scored the way it did. "Medium-High" means nothing without "because every whitespace change triggers a flag"
3. **Elimination reasoning** — for each dominated variant, state which variant dominates it and on which dimensions. The user should understand WHY a variant was eliminated, not just that it was
4. **Pareto front analysis** — for non-dominated variants, explain the trade-off: what does each sacrifice for what it gains?
5. **Recommendation rationale** — which variant you recommend, the decisive dimension(s), and what risk the user accepts by choosing it. Make clear this recommendation is advisory, not the user's choice
6. **Choice prompt** — only after the Pareto-front explanation and advisory recommendation, ask the human which non-dominated variant to take forward

If there are comparison warnings (missing dimensions, expired measurements, parity violations), surface them — these are decision-quality signals.

$ARGUMENTS
