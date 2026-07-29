Reality-check a recorded decision against current code and runtime. When its
evidence or validity semantics are current, inspect a known FPF SourceID or
UnitID with `haft_query(action="fpf", mode="inspect", identifier="...")`, or
use `mode="concern"` with the exact evidence question and inspect the direct
pattern body. Query is retrieval, not evidence or a verdict; do not route
through a shadow pattern catalog.

1. Locate the DecisionRecord with `haft_query(action="status")` or compact
   `haft_query(action="search", query=...)` discovery.
2. Recover the selected full record with
   `haft_query(action="related", artifact_ref="<dec-...>")`. Read its
   `structured_data` claims/predictions: what was supposed to be true by now,
   with which observable, threshold, and planned `verify_after` evidence check?
   Search hit/miss is not evidence that predictions exist or are absent. Do not
   use raw SQLite while kernel exact recovery is available.
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
