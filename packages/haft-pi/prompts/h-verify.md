Reality-check a recorded decision against the current code and runtime.

1. Locate the DecisionRecord: `haft_query(action="status")` or
   `haft_query(action="search", query=...)`.
2. Read its predictions: what was supposed to be true by now, with which
   observable and threshold?
3. Gather evidence — run tests, measure, inspect code. Design-time claims
   are not run-time evidence; label which is which.
4. Attach evidence via the native `haft_decision` tool:

```json
{
  "action": "evidence",
  "artifact_ref": "<dec-...>",
  "evidence_content": "<what was checked and what it showed>",
  "evidence_type": "<test|measurement|incident|review>",
  "evidence_verdict": "<supports|refutes|weakens>",
  "carrier_ref": "<file:line, test name, or URL>"
}
```

5. If the decision weakened or reality moved: surface it and suggest
   `haft_refresh` (waive / supersede / deprecate / reopen) — mutation only
   on explicit operator instruction.
6. Drift on touched files: re-baseline via
   `haft_decision(action="baseline")` or surface the drift inline. Never
   silently proceed past it.
