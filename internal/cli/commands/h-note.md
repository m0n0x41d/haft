---
description: "Record a micro-decision with rationale"
---

# Quick Note

Record an engineering decision made during coding. Haft validates before recording:
- Rationale is required (why this choice?)
- Checks for conflicts with active decisions
- Suggests /h-frame if scope is too large for a note

## Investigation-first discipline

Before recording, if the title or rationale uses an umbrella term
("service", "process", "ready", "queue", "stable"), call
`haft_query(action="resolve_term", term="<umbrella>")` to ground it in
the project's bounded context. The note then references the resolved
canonical name, not the vague placeholder. Don't bounce back to the
operator with "what do you mean?" — the project corpus usually answers
it. Ask only the one specific question if the resolver returns
`ambiguous` with multiple real candidates.

Use `haft_note` tool with:
- `title`: what was decided
- `rationale`: why this choice, what alternatives existed
- `affected_files`: file paths affected (optional)
- `evidence`: supporting evidence (optional)
- `context`: grouping tag (optional)

$ARGUMENTS
