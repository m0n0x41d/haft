---
name: h-status
description: Inspect the live Haft governance state in Pi before governed work, open-ended FPF reasoning, or completion claims.
---

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
- module coverage tells you whether code context or impact should be queried before edits,
- chat summaries are not durable work unless the relevant Haft artifact exists.

Do not create binding decisions or commissions unless the operator explicitly asks for that manual action.
