---
description: "Record a micro-decision with rationale"
---

# Quick Note

Record an engineering decision made during coding. Quint validates before recording:
- Rationale is required (why this choice?)
- Checks for conflicts with active decisions
- Suggests /h-frame if scope is too large for a note

Use `haft_note` tool with:
- `title`: what was decided
- `rationale`: why this choice, what alternatives existed
- `affected_files`: file paths affected (optional)
- `evidence`: supporting evidence (optional)
- `context`: grouping tag (optional)

$ARGUMENTS
