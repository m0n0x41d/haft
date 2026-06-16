Inspect the live Haft governance state for this project.

Call `haft_query` with:

```json
{
  "action": "status",
  "full": false
}
```

This is the compact operator cockpit, not the full project audit. If a detail
is omitted, treat it as "not shown here", not as absent. Use explicit follow-up
calls when needed:

- `haft_query` with `{ "action": "status", "full": true }` for detailed status,
- `haft_query` with `{ "action": "coverage" }` for full module coverage,
- `haft_refresh` with `{ "action": "scan", "verbose": true }` for drift/stale detail,
- `haft_refresh` with `{ "action": "plan" }` for the maintenance work order.

Do not pass `context` unless you intend to FILTER artifacts by a context
label — it is not a free-text annotation.

Report:

- overseer or drift signals that affect the current request,
- active or recently relevant problem/decision/method artifacts,
- whether the project is framed enough for the requested work,
- which drill-down call is needed before relying on omitted detail,
- the next reversible action.

Do not create or mutate Haft artifacts from this prompt alone.
