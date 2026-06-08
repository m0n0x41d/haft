---
name: h-compare
description: |
  Compares 2+ candidate variants under parity discipline and returns a Pareto front (not a scalar winner) — declares the selection policy and parity plan BEFORE scoring, then scores each dimension across all variants in parallel to prevent anchoring bias. Make sure to use this skill whenever the user asks "A or B", "which is better", "compare X and Y", "trade-off between X and Y", "should we pick X or Y", "Pareto for these options", "PostgreSQL vs MySQL", "NATS vs Kafka", "library A vs library B" — anywhere two or more viable approaches are on the table and a fair, recorded comparison is wanted before committing. Also use when /h-explore has just produced a SolutionPortfolio. NOT for generating new variants (use h-explore first). NOT for committing to the winner — that requires manual /h-decide per Transformer Mandate.
when_to_use: |
  ≥2 viable variants on the table and the operator wants a fair comparison. For new variants use h-explore. For binding the winner use /h-decide (manual-only).
argument-hint: "[portfolio-ref or comparison topic]"
allowed-tools: Bash Agent mcp__haft__haft_problem mcp__haft__haft_solution mcp__haft__haft_query
---

# h-compare — Fair comparison with Pareto front

You are running the FPF compare workflow: characterize dimensions → declare parity plan → declare selection policy BEFORE scoring → dim-wise parallel scoring → compute non-dominated set → present Pareto front (NOT a scalar winner).

## Compact interface discovery

If you need the exact compact contract, run `haft interface solution.compare --json`.
Use it as discovery; do not paste long MCP schemas or CLI help into the
session. For large payloads prefer:

```bash
haft artifact create solution.compare --input-file <input.json> --json
```

`mcp__haft__haft_solution(action="compare", ...)` remains the compatible fallback.

## Step 1 — Ensure portfolio exists

If no `portfolio_ref` is given:
- Look up via `mcp__haft__haft_query(action="status")` for active portfolios
- If only one active portfolio matches the problem, ask the operator to confirm

The kernel auto-detects when only one active portfolio exists, but explicit reference is safer.

## Step 2 — Characterize dimensions (agent drafts, operator reviews)

If the portfolio's problem has no dimensions, **the agent drafts them** based on the variants and the problem signal. Do not delegate dimension authorship back to the operator — that defeats the value of the agent. Read the variants, identify the axes on which they actually differ, draft 2-5 dimensions inline, then call the kernel. Surface the drafted dimensions to the operator with "Drafted these dimensions — edit if any are wrong before scoring." Operator review is the gate, not operator authorship.

```
mcp__haft__haft_problem(
  action="characterize",
  problem_ref="<prob-...>",
  dimensions=[
    {
      "name": "latency_p95",
      "role": "target",         // constraint | target | observation
      "polarity": "lower_better",
      "scale_type": "ratio",
      "unit": "ms",
      "how_to_measure": "<single sentence>"
    },
    {
      "name": "memory_usage",
      "role": "constraint",     // hard limit — eliminates variant before scoring
      "polarity": "lower_better",
      "scale_type": "ratio",
      "unit": "MB",
      "how_to_measure": "<...>"
    },
    {
      "name": "ops_complexity",
      "role": "observation",    // Anti-Goodhart: watch but don't optimize
      "polarity": "lower_better",
      "scale_type": "ordinal",
      "how_to_measure": "<...>"
    }
  ]
)
```

Per FPF CHR-01: 1-3 targets max, plus constraints (hard limits) and observations (watch but do not optimize — Anti-Goodhart).

## Step 3 — Declare parity_plan (BEFORE scoring per FPF CMP-01)

```
parity_plan = {
  "baseline_set": ["<variant_id_1>", "<variant_id_2>", "<variant_id_3>"],
  "window": "<time/observation window scores are comparable in>",
  "budget": "<resource budget held equal across variants>",
  "missing_data_policy": "explicit_abstain | zero | exclude",
  "pinned_conditions": ["<must-hold condition>", ...]
}
```

For DEEP mode the kernel REQUIRES baseline_set, window, budget, missing_data_policy to be present. Standard mode accepts gaps with warnings.

## Step 4 — Declare selection_policy (BEFORE scoring per FPF CMP-02)

State the rule used to pick from the Pareto front BEFORE you see any scores. This is the Anti-Goodhart enforcement boundary.

Bad (post-hoc): "We picked X because it scored best on the dimensions we cared about."
Good (pre-declared): "Maximize latency_p95 subject to memory_usage < 200MB constraint; tie-break by ops_complexity."

Store the policy string for the kernel call.

## Step 5 — Score variants DIM-WISE in parallel (one Agent per dimension)

For M dimensions and N variants, spawn M Agent subagents IN THE SAME MESSAGE. Each subagent scores ALL variants on ONE dimension. This way the same evaluator applies the same scale, preventing the comparability problem you get if you instead spawned per-variant agents.

```
Agent(
  description="Score all variants on latency_p95",
  prompt="
    You are scoring dimension: latency_p95
    Unit: ms
    Polarity: lower_better
    How to measure: <from characterize step>

    Variants to score:
    1. <variant_id_1>: <description>
    2. <variant_id_2>: <description>
    3. <variant_id_3>: <description>

    Apply the SAME scoring approach to ALL variants. Use parity_plan:
    <parity_plan>

    Return EXACTLY:
    scores:
      <variant_id_1>: <numeric or ordinal value with unit>
      <variant_id_2>: <...>
      <variant_id_3>: <...>
    methodology: <one paragraph: how you measured, what you assumed,
                  any missing data treated per parity_plan policy>
    confidence: low | medium | high
  "
)
```

Spawn M of these in one message. After all return, assemble scores per variant.

## Step 6 — Call kernel with scores + Pareto computation

```
mcp__haft__haft_solution(
  action="compare",
  portfolio_ref="<sol-...>",
  dimensions=["latency_p95", "memory_usage", "ops_complexity"],
  scores={
    "<variant_id_1>": {"latency_p95": "...", "memory_usage": "...", ...},
    "<variant_id_2>": {...},
    "<variant_id_3>": {...}
  },
  parity_plan=<from Step 3>,
  policy_applied="<selection policy declared in Step 4 BEFORE scoring>",
  mode="<inherit from problem>"
)
```

The kernel computes the non-dominated set (Pareto front) from scores. Constraints eliminate variants that violate hard limits BEFORE Pareto computation.

## Step 7 — Present the Pareto front to operator

Surface:
- Non-dominated set (Pareto front) with their score profiles
- Dominated variants with explicit dominance explanation (which variants dominate them, on which dimensions)
- Pareto trade-offs: for non-dominated variants, what they each give up
- Recommendation (advisory only — the operator decides via /h-decide)
- Soft warnings from the kernel (read them — they may flag rigged comparison: missing parity, single-dimension, selected-not-in-non-dominated, etc.)

**Re-grounding discipline (FPF A.7).** Every variant label (`V1`, `V2`,
…) and artifact ID (`sol-...`, `prob-...`) in your Pareto-front summary,
dominance explanation, and recommendation paragraphs MUST be paired with
its human-readable title or one-line claim on first mention. Bare `V3
dominates V1 on latency_p95` is opaque when operator returns 30 minutes
later; `V3 (in-memory cache) dominates V1 (per-request DB read) on
latency_p95` restores the object behind the carrier. Apply consistently
to dimension labels too where they are abstract codes. See CLAUDE.md
Critical Reminders for the project-wide rule.

## Step 8 — Hand off to operator for decision

This skill STOPS at presentation. The binding choice is /h-decide (manual-only per Transformer Mandate). Recommend it as next step.

## Step 9 — Say it in plain words (MANDATORY, goes LAST)

Everything above — the Pareto front, dominated variants, scores, artifact IDs — is for traceability. It is NOT how you tell the operator what happened. ALWAYS end your reply with a short plain-language section the operator can read on its own and act on. The most common failure of this skill is dumping the ID-and-jargon structure and stopping — the operator then cannot tell what was decided.

End with an `## In plain words` header, then:
- **What we compared** — one sentence.
- **What's left worth doing** — the surviving options in plain words: what each gives you and what it costs, one line each.
- **What you'd pick and the honest reason.**
- **The real trade-off** — the single thing the choice actually turns on.
- **The one question** you need answered to move.

Hard rules for this section:
- ZERO artifact IDs (`sol-…`, `prob-…`, `dec-…`, `V1`/`V2`). Name each option by what it DOES, not its label.
- ZERO undefined FPF jargon. Do not write "Pareto front", "dominated", "constraint-eliminated", "Anti-Goodhart", "weakest-link", or "parity" here — say it plainly ("the options not worth it because…", "the thing we watch but don't chase").
- Short. If the operator cannot understand and decide from THIS section alone, it failed — a comparison the operator can't read is wishlist, not work.

## Ceremony

This workflow inherits its mode from the framed problem. If comparing without a
frame and a build will follow, right-size the effort first:
`mcp__haft__haft_query(action="ceremony", files=[...])` — the floor recommends a
mode and never lets a high-risk change run tactical.

## What NOT to do

- Do not pre-collapse to a scalar winner. The Pareto front IS the result. The decide step picks from it.
- Do not score per-variant (one agent scores all dimensions of one variant) — different scorers + different scales = uncomparable scores. SCORE DIM-WISE.
- Do not declare selection policy AFTER seeing scores. That's post-hoc rationalization (FPF CMP-02 violation).
- Do not silently substitute dimensions the operator hasn't reviewed. DRAFT them inline (Step 2) and surface for review — that's the correct pattern. Asking "what dimensions matter?" instead of drafting is delegation back, which defeats the agent's value.
- Do not skip parity_plan in deep mode — kernel rejects.
- Do not let a variant that violates a constraint dimension survive into the Pareto computation. Constraints eliminate first.
- Do not silently pick a dominated variant as "selected" — the operator must explicitly override with rationale if so.
- Do not commit the decision; /h-decide is the binding step and is manual-only.

## FPF spec references

- B.5.2 — Abductive loop (parent procedure)
- C.18 — NQD-CAL (open-ended search)
- C.18.1 — Scaling Law Lens
- A.17/A.18/A.19 — Characteristic + CSLC + CHR pipeline
- A.19.CN — Comparability/Normalization
- A.19.CPM — Comparison Mechanism (Pareto)
- G.0 — Frame Standard for selection
- G.9 — Parity / Benchmark Harness
- CMP-01 (parity), CMP-02 (selection policy up front), CMP-03 (Pareto front), CMP-06 (CL across options)
- CHR-01 (indicator role taxonomy), CHR-09 (parity plan)

Look up via `mcp__haft__haft_query(action="fpf", query="A.19.CPM")`.
