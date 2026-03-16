---
description: "Generate implementation brief from a decision"
---

# Apply

Generate an Implementation Brief from an existing DecisionRecord. Extracts invariants, pre/post-conditions, admissibility, evidence requirements, and rollback plan into an actionable brief.

Use `quint_decision` tool with `action="apply"` and:
- `decision_ref`: DecisionRecord ID (auto-detected if only one active)

$ARGUMENTS
