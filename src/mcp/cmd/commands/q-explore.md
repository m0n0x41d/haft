---
description: "Explore solution variants for a framed problem"
---

# Explore Solutions

Generate genuinely distinct solution variants. Each must have a weakest link identified.

Use `quint_solution` tool with `action="explore"` and:
- `problem_ref`: ProblemCard ID (auto-detected if only one active)
- `variants`: array of options, each with:
  - `title`: variant name
  - `description`: what this option does
  - `strengths`: array of advantages
  - `weakest_link`: what bounds this option's quality (REQUIRED)
  - `risks`: array of risk notes
  - `stepping_stone`: true if opens future possibilities
  - `rollback_notes`: how to reverse

At least 2 variants required. Prefer 3+ that differ in kind, not degree.

$ARGUMENTS
