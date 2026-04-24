---
description: "Dashboard of active decisions, stale items, and recent notes"
---

# Status

Show what's active, what's stale, and what's recent.

Use `haft_query` tool with `action="status"`.
Optionally filter by `context`.

If status shows WorkCommissions needing attention, treat them as operator work:

- inspect with `haft_commission(action="show", commission_id="wc-...")`
- requeue only when the decision/scope is still current
- cancel when the work is obsolete, duplicated, or no longer authorized
- never infer completion from a WorkCommission merely being absent from
  runnable work

$ARGUMENTS
