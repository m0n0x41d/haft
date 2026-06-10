---
name: h-method
description: Run the Haft MethodPack loop in Pi — pull task-local SWE method cards before non-trivial code edits, close the same run with evidence or explicit waivers before claiming completion.
---

# h-method for Pi

The MethodPack loop is the default habit for code work in a Haft project, not
optional advice. Use the native Pi tool `haft_method`.

## When to pull

Before non-trivial code action: feature, bugfix/debug, refactor, external
integration, governed files, cross-module edits, behavior changes, failing
tests.

```json
{
  "action": "pull",
  "task": "<operator task in one or two sentences>",
  "declared_task_kind": "<feature|bugfix|refactor|integration|debug>",
  "change_intent": "<what will change and why>",
  "intended_files": ["<paths you expect to touch>"],
  "risk_signals": [{"id": "<signal>", "source": "agent", "evidence": "<why>"}]
}
```

Keep the returned `pull_id`. If context compacts before close, recover with
`haft_method(action="status")` or `haft_method(action="show", pull_id=...)`.

## When NOT to manufacture ceremony

Mechanical edits (formatting, rename-only, comment-only, generated metadata)
should request `"ceremony_request": "low"` or `"none"` and avoid architecture
gates. Ordinary local verification is enough for them.

## Before claiming completion

Close the same run — this is what makes "done" a claim with evidence instead
of a description:

```json
{
  "action": "close",
  "pull_id": "<the pull_id>",
  "changed_files": ["<actually changed paths>"],
  "gate_results": [{"gate": "<id>", "status": "pass", "evidence": "<test/build/diff ref>"}],
  "verification": {"summary": "<what was run and what it showed>"}
}
```

Hard gates require evidence or an explicit waiver with a reason. Soft gates
are guidance and need no waiver. Never report completion to the operator while
the MethodRun is still open.
