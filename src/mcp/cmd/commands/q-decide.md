---
description: "Finalize a decision with full rationale"
---

# Decide

Create a DecisionRecord — the crown jewel artifact. Must include what was chosen, why, and what to watch for.

Use `quint_decision` tool with `action="decide"` and:
- `selected_title`: name of chosen variant (required)
- `why_selected`: rationale (required)
- `why_not_others`: [{variant, reason}] for each rejected option
- `invariants`: what MUST hold at all times
- `pre_conditions`: checklist before implementation
- `post_conditions`: checklist after (definition of done)
- `admissibility`: what is NOT acceptable
- `evidence_requirements`: what to measure/prove
- `rollback`: {triggers, steps, blast_radius}
- `refresh_triggers`: when to re-evaluate
- `weakest_link`: what bounds reliability
- `valid_until`: expiry date (RFC3339)
- `affected_files`: files affected

$ARGUMENTS
