<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:332a5ef46498346e5fb591f3f92cf95dc7661bd588048e834468a2786bf81b5f -->

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
