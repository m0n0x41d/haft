Inspect the live Haft governance state for this project.

Contract truth: typed neighborhood and recall are **V9 CONTRACT**
capabilities. Source, schema, skill, or local-test presence is not
installed-runtime proof and does not establish Pi host parity. A readiness
claim requires current **EXACT-CANDIDATE EVIDENCE** from P14 tied to one exact
candidate; RC or release status additionally requires release authority. Pi
support is experimental, and contract inclusion or evidence alone does not
establish **CURRENT PRODUCT** status.

If the operator's current question is not about project status, resumption,
coverage, or reliance on recorded project state, return without calling this
prompt. Do not run status after completing unrelated work merely to backfill a
governance-looking report.

Call `haft_query` with:

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

This is the compact operator cockpit, not the full project audit. If a detail
is omitted, treat it as "not shown here", not as absent. Use explicit follow-up
calls when needed:

- `haft_query` with `{ "action": "status", "full": true }` for detailed status,
- `haft_query` with `{ "action": "coverage" }` for full module coverage plus
  bounded exact `affected_files` link gaps when the code index is current,
- `haft_query` with `{ "action": "drift_events", "full": true }` for the
  complete currently recorded drift fanout,
- `haft_query` with `{ "action": "decision_reconcile", "full": true }` for a
  read-only reconciliation audit.

Coverage reads stored module data and the current derived code index. If the
query reports that the code index is uninitialized, stale, or degraded,
surface that as unavailable rather than an empty-clean result; do not refresh
it from this read-only prompt.

For an exact current EntityOfConcern and bounded context, use
`haft_query(action="memory",
memory_request={"mode":"neighborhood",...})`. Put the exact
`contract_version`, project basis, EntityRef, bounded context,
`agent_orientation.v2` view, explicit facets, and dimensioned budget inside
the closed `memory_request` branch as advertised by the tool schema. Read the
snapshot/projection basis, interpretation contract, facet coverage, item
postures, and applied budget. Only complete coverage supports known emptiness
at that exact basis. Read affordances only retrieve more basis. Resolve an
alias before neighborhood; use recall only inside the resolved scope. Preserve
retry/abstention limitations.

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

Do not pass `context` unless you intend to FILTER artifacts by a context
label — it is not a free-text annotation.

Report:

- overseer or drift signals that affect the current request,
- active or recently relevant problem/decision/method artifacts,
- relevant module coverage cues and bounded files in decision-bearing modules
  that lack an exact active `affected_files` link; never relabel those link
  gaps as proof that the files are undocumented, unconstrained, or incorrect,
- which drill-down call is needed before relying on omitted detail,
- possible current concerns without turning one into a universal next step.

Do not create or mutate Haft artifacts from this prompt alone.
