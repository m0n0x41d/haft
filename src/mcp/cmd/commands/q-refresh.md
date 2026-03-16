---
description: "Detect stale decisions and manage their lifecycle"
---

# Refresh

Find expired decisions and take action: waive, reopen, supersede, or deprecate.

Use `quint_refresh` tool with:
- `action="scan"` — find all stale/expired decisions
- `action="waive"` — extend validity (decision_ref, reason, evidence required)
- `action="reopen"` — start new problem cycle linked to old decision
- `action="supersede"` — replace with a different decision
- `action="deprecate"` — mark as no longer relevant

$ARGUMENTS
