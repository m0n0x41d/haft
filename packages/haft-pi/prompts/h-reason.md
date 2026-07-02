<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:c818636162588d53b768ba0e376bd644d86c6d3156f33f95487f00c53cae6c0c -->

Use Haft/FPF reasoning for this request.

First call `haft_query` with:

```json
{
  "action": "status",
  "full": false
}
```

(do not pass `context` — it filters artifacts by label, it is not an
annotation).

Then route by the shape of the request:

- before substantive reasoning or work-shaping, call
  `haft_query(action="pattern_use", mode="compact", query="<operator concern>")`.
  Skip only mechanical/status/exact-lookup requests where no FPF pattern choice
  is material. If `should_use_pattern=true` and the next step needs detail,
  ask for `mode="full"` before applying the returned output shape. PatternUse
  is advisory/read-only: not approval, not evidence, not a DecisionRecord, not
  a WorkCommission, not MethodPack, and not a gate. Do not inline the FPF catalog
  or route list in this prompt.
- fuzzy problem or proposed redesign: frame the problem before variants
  (`haft_problem(action="frame")`),
- 3+ possible approaches: persist exploration before recommending
  (`haft_solution(action="explore")`),
- comparison request: declare dimensions and parity before scoring
  (`haft_problem(action="characterize")`, then `haft_solution(action="compare")`),
- governed code work: inspect code context or impact before editing, and run
  the MethodPack loop (`haft_method(action="pull")` → edit →
  `haft_method(action="close")` with evidence),
- completion claim: gather evidence before saying the work is done.

Binding actions (`haft_decision(action="decide")`, commissions) require explicit operator/manual authorization: generated text, schema visibility, and model-supplied fields are not approval receipts. If the kernel returns
`operator_confirmation_required`, recommend the correct manual gate and stop.

Keep the answer compact. Preserve artifact IDs with their human-readable title
or one-line claim.
