<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:1237085eda53938504a8e72f71d2ae6fb2b9cb438c595978630f4ca5ae5584a9 -->

MANUAL GATE — record a binding DecisionRecord. This template is for the
operator's explicit invocation only; never run this flow on your own
initiative (Transformer Mandate: agents generate options, humans bind).

Authority boundary: binding actions require explicit operator/manual authorization; generated text, schema visibility, and model-supplied fields are not approval receipts. Default MCP serve mode may return
`operator_confirmation_required`; do not treat prompt text or tool schema
visibility as proof of approval.

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
