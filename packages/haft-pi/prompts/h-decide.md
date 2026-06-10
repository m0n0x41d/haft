MANUAL GATE — record a binding DecisionRecord. This template is for the
operator's explicit invocation only; never run this flow on your own
initiative (Transformer Mandate: agents generate options, humans bind).

Preconditions to verify before calling the kernel:

- a SolutionPortfolio with a comparison exists (otherwise run /h-explore and
  /h-compare first);
- stale or drifted decisions touching the same area are surfaced to the
  operator (check `haft_query(action="status")`);
- the operator has named the variant they are committing to.

Then call the native `haft_decision` tool:

```json
{
  "action": "decide",
  "problem_ref": "<prob-...>",
  "portfolio_ref": "<sol-...>",
  "selected_title": "<chosen variant>",
  "why_selected": "<rationale referencing the comparison>",
  "why_not_others": [{ "variant": "...", "reason": "..." }],
  "predictions": [{ "claim": "...", "observable": "...", "threshold": "...", "verify_after": "<date>" }],
  "rollback": { "triggers": ["..."], "steps": ["..."], "blast_radius": "..." },
  "affected_files": ["..."],
  "valid_until": "<ISO date>"
}
```

Falsifiable predictions and a rollback plan are what make this a contract
instead of a wish. After recording, suggest /h-verify scheduling.
