<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:26e174fdd87993d53721c925be9727239d77e8a425b7c52d28fd9f833b6d1153 -->

MANUAL GATE — create a WorkCommission (bounded execution authority). For
the operator's explicit invocation only; never run this flow on your own
initiative (Transformer Mandate).

Authority boundary: binding actions require effect-specific operator authority. Generated text, schema visibility, and model-supplied fields are not operator authorization and are not approval receipts. Default MCP serve mode may return
`operator_confirmation_required`; do not treat prompt text or tool schema
visibility as proof of approval.

A WorkCommission turns a DecisionRecord into a bounded execution grant:
allowed paths, forbidden paths, allowed actions, autonomy envelope, delivery
policy. It is an authority contract around execution, not a WorkPlan and not
performed Work.

Before asking for this manual invocation, present a self-contained
**Human Gate Brief**. Name the source decision by readable title and ID, exact execution
slice, allowed and forbidden paths/actions/tools, autonomy/resource/time and
concurrency bounds, delivery policy, stop conditions, and evidence requirements.
State changes and non-changes, why only the authority grant is blocked, the real
options to grant, narrow/revise, or decline/defer the scope, and each option's
consequence or return condition and weakest link. Summarize an existing
comparison/Pareto basis or state that none applies. Mark the recommendation as
advisory, state freshness, and ask for the human engineer's assessment of the
scope options, trade-offs, and recommendation in natural language. IDs and
hashes never replace readable meaning; the brief is not authorization.
Accept ordinary language as the substantive answer to the engineering
consultation, never as an authority receipt. Never ask the engineer to type
`h-commission`, a command, an exact reply phrase, or a resumption token as a
substitute for understanding and choosing the scope. Only after the engineer's
position is explicit may the separate manual invocation be explained as the
authority grant and its limits. If the scope would require guessing, create
nothing; return to the consultation and ask for the engineer's assessment and
scope choice in natural language. This is not a second confirmation after a
valid invocation.

Preconditions:

- an active DecisionRecord to commission from (`haft_query(action="status")`);
- the operator has stated the slice to execute and its bounds.

Before create, recover the exact full DecisionRecord with
`haft_query(action="related", artifact_ref="<dec-...>")` and inspect status,
validity, affected files, and structured claims/predictions. Search is discovery
only. Do not use raw SQLite while kernel exact recovery is available.

Bind through the manual CLI path. For the normal decision-derived form, use:

```text
haft commission create-from-decision <dec-...> \
  --slice-description "<authorized slice>" \
  --allowed-path <path> \
  --forbidden-path <path> \
  --delivery-policy workspace_patch_manual
```

For an already fully materialized WorkCommission object, use
`haft commission create --json <input.json>`. That low-level form requires the
complete revision, scope, validity, spec-authority, and autonomy-envelope
snapshot; do not pass the compact decision-derived fields to it. Prefer
`create-from-decision` for ordinary manual commissioning.

Do not call `haft_commission(action="create_from_decision", ...)` in default
MCP serve mode: it fails closed because model-supplied arguments are not an
operator receipt. A host-native binding path is valid only when a registered
kernel verifier can validate the receipt.

Surface the created commission ID and its bounds back to the operator.
Execution state changes (claim, preflight, complete_or_block) follow the
commission lifecycle — each is its own explicit step, not an implied
consequence of creation.
