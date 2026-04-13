---
description: "Define comparison dimensions for a framed problem"
---

# Characterize

Define the characteristic space — what dimensions matter and how they're measured. Without this, comparisons are arbitrary.

Current runtime persists characterization on the ProblemCard via `haft_problem(action="characterize")`. Define dimensions once, keep parity rules explicit, then carry that same characterized space into `/h-explore` and `/h-compare`.

Recommended fields to define in your reasoning:
- `name`: dimension name (e.g., "throughput", "ops complexity")
- `scale_type`: ordinal, ratio, nominal
- `unit`: measurement unit
- `polarity`: higher_better or lower_better
- `how_to_measure`: measurement procedure
- `parity_rules`: what must be equal across variants for fair comparison

$ARGUMENTS
