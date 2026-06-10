Inspect the live Haft governance state for this project.

Call `haft_query` with:

```json
{
  "action": "status",
  "full": false
}
```

Do not pass `context` unless you intend to FILTER artifacts by a context
label — it is not a free-text annotation.

Report:

- overseer or drift signals that affect the current request,
- active or recently relevant problem/decision/method artifacts,
- whether the project is framed enough for the requested work,
- the next reversible action.

Do not create or mutate Haft artifacts from this prompt alone.
