---
description: "Define comparison dimensions for a framed problem"
---

# Characterize

Define the characteristic space — what dimensions matter and how they're measured. Without this, comparisons are arbitrary.

Current runtime does not persist characterization as a standalone artifact. Use this step to prepare comparison dimensions, then carry them into `/h-explore` and `/h-compare`.

Recommended fields to define in your reasoning:
- `name`: dimension name (e.g., "throughput", "ops complexity")
- `scale_type`: ordinal, ratio, nominal
- `unit`: measurement unit
- `polarity`: higher_better or lower_better
- `how_to_measure`: measurement procedure
- `parity_rules`: what must be equal across variants for fair comparison

$ARGUMENTS
