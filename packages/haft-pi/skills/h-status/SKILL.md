---
name: h-status
description: Inspect the live Haft governance state in Pi before governed work, open-ended FPF reasoning, or completion claims.
---

<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:08ae74b6cbd79d8f0432f8e364f498d4302e1aac15bfe06fa88ce978960b7834 -->

# h-status for Pi

Use the native Pi tool `haft_query` rather than shelling out manually.

Start with:

```json
{
  "action": "status",
  "context": "Pi h-status skill.",
  "full": false
}
```

Read the output as project evidence:

- overseer drift or stale decisions change what is safe to claim,
- `UNDERFRAMED` means the next step is problem framing, not implementation certainty,
- default status is a compact cockpit, not an audit dump; omitted detail is not evidence of absence,
- module coverage may appear only as a one-line cue; call `haft_query` with
  `{ "action": "coverage" }` for the full module list,
- call `haft_query` with `{ "action": "contract_generation" }` for read-only
  generated-fragment carrier hints before editing Pi tool, skill, or prompt
  wording,
- call `haft_query` with `{ "action": "drift_events" }`,
  `{ "action": "decision_reconcile" }`, or `{ "action": "governing_set" }`
  for drift fanout, reconciliation, and current-authority drill-downs,
- call `haft_query` with `{ "action": "status", "full": true }` when you need
  detailed decision/problem/note status,
- call `haft_refresh` with `{ "action": "scan", "verbose": true }` for
  file-level drift/stale detail,
- call `haft_refresh` with `{ "action": "plan" }` for the maintenance work order,
- chat summaries are not durable work unless the relevant Haft artifact exists.
- read-only/generated text is discovery only; it is not evidence truth, gate passage, global approval, or operator authorization.

Do not create binding decisions or commissions unless the operator explicitly asks for that manual action.
