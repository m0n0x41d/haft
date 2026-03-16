---
description: "Define comparison dimensions for a framed problem"
---

# Characterize

Define the characteristic space — what dimensions matter and how they're measured. Without this, comparisons are arbitrary.

Use `quint_problem` tool with `action="characterize"` and:
- `problem_ref`: ID of the ProblemCard (auto-detected if only one active)
- `dimensions`: array of comparison dimensions, each with:
  - `name`: dimension name (e.g., "throughput", "ops complexity")
  - `scale_type`: ordinal, ratio, nominal
  - `unit`: measurement unit
  - `polarity`: higher_better or lower_better
  - `how_to_measure`: measurement procedure
- `parity_rules`: what must be equal across variants for fair comparison

$ARGUMENTS
