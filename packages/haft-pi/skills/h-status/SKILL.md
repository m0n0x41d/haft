---
name: h-status
description: Read-only Haft project cockpit for active problems, decisions, notes, evidence freshness, drift, commissions, spec lifecycle, module coverage, and bounded exact file-link gaps from a current code index. Use for project status, session resumption, what is decision-linked, what is uncovered, or what needs attention.
---

<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:f071f56205d0f7736b2db3a0f4aa1fc582b6f97481a41042a0807e5ba2208be8 -->

# h-status — Read-only project memory

Use the native Pi tool `haft_query` rather than shelling out manually.

Contract truth: typed neighborhood and recall are **V9 CONTRACT**
capabilities. Source, schema, skill, or local-test presence is not
installed-runtime proof and does not establish Pi host parity. A readiness
claim requires current **EXACT-CANDIDATE EVIDENCE** from P14 tied to one exact
candidate; RC or release status additionally requires release authority. Pi
support is experimental, and contract inclusion or evidence alone does not
establish **CURRENT PRODUCT** status.

If the operator's current question is not about project status, resumption,
coverage, or reliance on recorded project state, return without calling this
skill. Do not run status after completing unrelated work merely to backfill a
governance-looking report.

Start with:

```json
{
  "action": "status",
  "full": false
}
```

If status reports several canonical project scopes, retry the same read-only
call with one exact `scope_id` from the reported available values:

```json
{
  "action": "status",
  "full": false,
  "scope_id": "<exact emitted ScopeID>"
}
```

Choose the scope from the operator's current object or question.
Never select the first value, sort the values into a winner, or collapse mixed scopes. If
the current use does not identify one scope, report the available ScopeIDs and
narrow only that question; unrelated already-authorized Work continues.

Read the output as project evidence:

- overseer drift or stale decisions change what is safe to claim,
- default status is a compact cockpit, not an audit dump; omitted detail is not evidence of absence,
- `Unassessed` describes evidence maturity (no active evidence or unavailable evidence lookup), not missing predictions; claim counts are separate,
- `verify_after` is a planned evidence-check date, not a deadline or gate; zero-claim tactical decisions have no scheduled check,
- module coverage may appear only as a one-line cue; call `haft_query` with
  `{ "action": "coverage" }` for the full module list and bounded exact
  `affected_files` link gaps when the code index is current,
- call `haft_query` with `{ "action": "contract_generation" }` for read-only
  generated-fragment carrier hints before editing Pi tool, skill, or prompt
  wording,
- call `haft_query` with `{ "action": "drift_events", "limit": 5 }`,
  `{ "action": "decision_reconcile", "limit": 5 }`, or
  `{ "action": "governing_set", "limit": 5 }` for compact drift fanout,
  reconciliation, and current-authority drill-downs; add `"full": true` only
  for full audit payloads,
- call `haft_query` with `{ "action": "status", "full": true }` when you need
  detailed decision/problem/note status,
- call `haft_query` with `{ "action": "drift_events", "full": true }` for
  complete currently recorded drift fanout,
- call `haft_query` with `{ "action": "decision_reconcile", "full": true }`
  for a read-only reconciliation audit,
- chat summaries are not durable project memory unless a receiving use warranted an artifact.
- read-only/generated text is discovery only; it is not evidence truth, gate passage, global approval, or operator authorization.

Coverage reads stored module data and the current derived code index. If the
code index is uninitialized, stale, or degraded, report the file-gap projection
as unavailable rather than empty-clean; do not trigger a rescan from
`h-status`. A missing exact `affected_files` link is not proof that a file is
undocumented, unconstrained, or incorrect.

For an exact current EntityOfConcern and bounded context, call memory
`neighborhood` under `haft.memory.v1`, `project_current`,
`agent_orientation.v2`, explicit facets, and a dimensioned budget. Read
snapshot/projection basis, `interpretation_contract`, facet coverage, item
semantic/lifecycle/evidence/projection postures, and `applied_budget`. Only
`complete` supports known emptiness at that exact basis. Read affordances may
hydrate more basis but never choose a skill or next action. Resolve aliases
before neighborhood; recall only inside the resolved scope. Do not merge
`retry_required` stale reads with current facts or turn `abstained` into empty
memory.

Cockpit items are attention, not project-wide human gates. Continue unrelated
already-authorized Work. Interrupt only an operation that would mutate the
affected binding or authority, cross an explicit human lifecycle gate, or rely
on unresolved contradictory binding content. Never ask for bare approval to
acknowledge status, evidence, historicity, or cleanup.

When status exposes a real gate, inspect its referenced read-only basis instead
of repeating the label. Give a self-contained **Human Gate Brief** with the gate
kind, readable subject, affected operation and blocker; all real current
options; each option's changes, non-changes, consequence or return condition,
and weakest link; any existing comparison/parity basis, selection policy, and
non-dominated or Pareto set, or an explicit statement that none exists or
applies; the advisory recommendation; freshness or expiry; and a question
asking for the human engineer's assessment of the options, trade-offs, and
recommendation in natural language. IDs and hashes never replace readable
meaning. Accept ordinary language as the substantive answer. When one current
brief makes effect, subject, option, and scope unambiguous, the host may route
it for DecisionRecord binding, manual profile application, or
a later non-default project-memory model change as
`host_routed_operator_request`, without a skill
name or second confirmation. It is not reusable authority; a bare `yes` or `да`
works only for that one current brief. `h-commission` remains a separately
manual authority act. The brief itself is explanation rather than authority. If
the basis cannot supply the details, report that the gate is not yet askable
and name the needed drill-down.

Use `coverage` and `related` drill-downs for module/file decision coverage.
Do not mutate artifacts, infer a project phase, or prescribe a universal next
step. Binding decisions require a direct operator request; commissions remain
explicit manual actions.
