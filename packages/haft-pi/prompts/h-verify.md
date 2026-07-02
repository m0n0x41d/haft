Reality-check a recorded decision against the current code and runtime.

Before substantive verification reasoning, call `haft_query`:

```json
{
  "action": "pattern_use",
  "mode": "compact",
  "query": "<operator concern>"
}
```

Skip only mechanical/status/exact-lookup requests where no FPF pattern choice is
material. If `should_use_pattern=true` and verification needs output-shape
detail, ask for `mode="full"` before applying the returned pattern. PatternUse
is advisory/read-only: not approval, not evidence, not a DecisionRecord, not a
WorkCommission, not MethodPack, and not a gate. Do not inline the FPF catalog
or route list in this prompt.

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
