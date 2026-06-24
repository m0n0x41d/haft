<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:2446c125bbb60d02b80687334530028d79027a9ce26678d41753fd8e600ab508 -->

MANUAL GATE — create a WorkCommission (bounded execution authority). For
the operator's explicit invocation only; never run this flow on your own
initiative (Transformer Mandate).

Authority boundary: binding actions require explicit operator/manual authorization; generated text, schema visibility, and model-supplied fields are not approval receipts. Default MCP serve mode may return
`operator_confirmation_required`; do not treat prompt text or tool schema
visibility as proof of approval.

A WorkCommission turns a DecisionRecord into a bounded execution grant:
allowed paths, forbidden paths, allowed actions, autonomy envelope, delivery
policy. It is the contract between planning and execution.

Preconditions:

- an active DecisionRecord to commission from (`haft_query(action="status")`);
- the operator has stated the slice to execute and its bounds.

Then call the native `haft_commission` tool:

```json
{
  "action": "create_from_decision",
  "decision_ref": "<dec-...>",
  "slice_description": "<what exactly this commission authorizes>",
  "allowed_paths": ["<paths the executor may touch>"],
  "forbidden_paths": ["<paths off-limits>"],
  "delivery_policy": "workspace_patch_manual"
}
```

Surface the created commission ID and its bounds back to the operator.
Execution state changes (claim, preflight, complete_or_block) follow the
commission lifecycle — each is its own explicit step, not an implied
consequence of creation.
