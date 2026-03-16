---
description: "Frame an engineering problem before solving it"
---

# Frame Problem

Frame the actual problem before jumping to solutions. The bottleneck is problem quality, not solution speed.

Use `quint_problem` tool with `action="frame"` and:
- `title`: problem title
- `signal`: what's anomalous, broken, or needs changing (required)
- `constraints`: hard limits that MUST hold
- `optimization_targets`: what to improve (1-3 max)
- `observation_indicators`: what to monitor but NOT optimize
- `acceptance`: how we'll know it's solved
- `blast_radius`: what systems/teams are affected
- `reversibility`: how easy to undo (low/medium/high)
- `mode`: tactical, standard (default), deep
- `context`: grouping tag

$ARGUMENTS
