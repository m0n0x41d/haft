---
description: "Manage artifact lifecycle — detect stale, extend, archive, or replace"
---

# Refresh

Manage the lifecycle of ALL artifacts: decisions, problems, notes, portfolios. Find what's stale, extend what's still valid, archive what's done, replace what's outdated.

Use `haft_refresh` tool with:
- `action="scan"` — find all stale/expired artifacts + evidence-degraded decisions (R_eff < 0.5)
- `action="waive"` — extend validity with justification (artifact_ref, reason required)
- `action="reopen"` — start new problem cycle linked to old decision (decisions only)
- `action="supersede"` — replace with a different artifact (artifact_ref, new_artifact_ref, reason)
- `action="deprecate"` — archive as no longer relevant (artifact_ref, reason required)

Common use cases:
- Problem no longer relevant? → `deprecate`
- Note superseded by a full decision? → `supersede` with the decision ref
- Decision still valid but expired? → `waive` with evidence
- Problem needs re-examination? → `reopen` (creates new ProblemCard with lineage)

$ARGUMENTS
