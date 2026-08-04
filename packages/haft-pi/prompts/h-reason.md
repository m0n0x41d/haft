<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:f071f56205d0f7736b2db3a0f4aa1fc582b6f97481a41042a0807e5ba2208be8 -->

Use source-first Haft/FPF reasoning for the operator's current question.

Contract truth: source-native FPF Query and typed-memory resolve, neighborhood,
and recall are **V9 CONTRACT** capabilities. Source, schema, skill, or
local-test presence is not installed-runtime proof and does not establish Pi
host parity. A readiness claim requires current
**EXACT-CANDIDATE EVIDENCE** from P14 tied to one exact candidate; RC or release
status additionally requires release authority. Pi support is an experimental
compatibility carrier; stable host parity is not proven. Do not infer
**CURRENT PRODUCT** status from contract inclusion or evidence alone.

For purely mechanical, status-only, or exact project-lookup work where no FPF
pattern choice is material, caller abstention is the result: skip FPF Query and
do the bounded work directly. Do not fabricate
`QueryResult(kind="abstained")`; no query ran. Query when pattern applicability
is material or uncertain.

Keep exact identifier namespaces separate:

- FPF `PatternID`, `SourceID`, or `UnitID` ->
  `{ "action": "fpf", "mode": "lookup|inspect", "identifier": "<id>" }`;
- canonical Haft artifact ID ->
  `{ "action": "related", "artifact_ref": "<id>" }`;
- code symbol or `SymbolAnchor` ->
  `{ "action": "node", "symbol": "<name>" }` or `anchor_id`;
- typed-memory `EntityID` or `EntityAlias` ->
  `haft_query(action="memory", memory_request={"mode":"resolve","query":"<id-or-alias>",...})`;
  never coerce it into another namespace.

After exact resolution, use the closed `memory_request` branch whose nested
`mode` is `neighborhood` to hydrate the EntityOfConcern graph, or `recall` for
bounded lexical recall inside that exact scope. Mode-specific required fields
come from the tool schema. If structured project memory is unavailable, call
`haft_onboard(action="status")` and follow its readable next action; do not
invent or expose an internal schema-selection step. Resolution, projection
inclusion, and recall rank are not truth, applicability, authority, or Work
order.

When `memory.resolve` returns `known_absent`, that result does not authorize a
write. A concrete durability-requiring receiving use may be operator-named or
agent-inferred from current Work. Infer it when cross-session continuation,
handoff, audit, automation, delayed or expensive feedback, or costly reversal
already depends on stable identity; the operator does not need to pre-name it.
When identity, context, and aliases are recoverable, establish the minimum EntityOfConcern without asking for separate permission.
Do not infer this use from an empty graph or generic future usefulness. Call:

```text
haft_entity(
  action="establish",
  entity_id="<stable proposed id>",
  label="<readable label>",
  bounded_context_ref="<exact bounded context>",
  aliases=["<known alias, in canonical order>"],
  persistence_reason="named_receiving_use",
  request_provenance_ref="<exact current Work and its concrete receiving use>",
  idempotency_key="<stable key for this exact request>"
)
```

The task-level tool owns alias conflict checks, validation, internal project
basis, admission, and post-commit resolution. Use an `established` result's
exact `next_read` unchanged. Preserve `identity_conflict`, `alias_conflict`,
`idempotency_conflict`, `onboarding_required`,
`restart_required`, `rejected`, or `commit_outcome_unknown`; never invent
success. Retry `restart_required` with the unchanged idempotency key.

For a neighborhood, supply the closed outer
`action="memory", memory_request={...}` envelope. Inside `memory_request`,
supply `mode="neighborhood"`, `contract_version="haft.memory.v1"`,
`basis={kind:"project_current"}`, projection profile
`agent_orientation.v2`, explicit requested facets, and a dimensioned read
budget. Inspect `result_kind`, snapshot/projection basis,
`interpretation_contract`, facet coverage, item semantic/lifecycle/evidence/
projection postures, and `applied_budget`. Known emptiness requires `complete`
coverage at the exact facet/profile/snapshot/context. Honor
`hydrate_before_reliance`; read affordances retrieve more basis but never
select a capability or next action. Do not merge retry-required stale reads
with current facts.

Use resolve only when identity is missing and recall only after exact
entity/context resolution. Recall rank is discovery, not truth, applicability,
recommendation, freshness, authority, or work priority. Query FPF when pattern
applicability, an unfamiliar kind, or missing method basis is current.

On `wrong_identifier_namespace` with `same_call_retryable=false`, do not retry
the same action or ask for acknowledgement. Execute the exact `recovery_call`
when it names an available read-only surface; otherwise report that the
required surface is unavailable.

For code-area or flow orientation without an exact symbol, use
`haft_query(action="explore", query="<current code concern>")`. The working
view returns bounded advisory candidates without auto-selecting identity.
Before a non-mechanical edit to governed code, use `code_context` or `impact`
on the actual target. Purely mechanical work may explicitly abstain and record
`not_applicable`. Code-graph and typed-memory orientation are separate; neither
substitutes for the other.

1. Name the current object or EntityOfConcern, question, known relations,
   constraints, evidence, and smallest useful result.
2. Query `haft_query` with `{ "action": "fpf", "mode": "concern", "query": "<object + question + terms>" }`.
   Compare README practical-use cards, use the source ToC as the PatternID and
   keyword index, recover an exact source unit with
   `{ "action": "fpf", "mode": "lookup", "identifier": "<SourceID or UnitID>" }`
   or non-broadening
   `{ "action": "fpf", "mode": "inspect", "identifier": "<exact SourceID or UnitID>" }`,
   then inspect the selected pattern's full body. Retrieval returns source
   candidates. Returned source material is not applicability, selection,
   recommendation, evidence, precedence, or authority.
   In ordinary working use, identify the selected direct pattern by
   `PatternID`, title, and stable source reference. Do not routinely reproduce
   source spans, repository-local paths, line ranges, hashes, revisions, or
   other provenance. Request trace or audit provenance only when the current
   use requires it.
   Preserve a non-English original query; add `entity_of_concern`,
   `known_context`, and `intended_use` with precise source-language or FPF
   terms when known. Do not invent a hidden bilingual route catalog; an
   unsupported raw-language query may abstain.
   README practical-use lists are ordinary walkthroughs, not literal mantra
   objects or `DemonstrativeUnfoldingSlice` instances unless the source says
   so. Query returns candidates; select by current condition and apply the
   direct Solution. v9 Query uses authored phrases, headings/keywords, and
   role-local FTS; dense retrieval is **DEFERRED RESEARCH**.
3. Select only the capability that is current. `h-frame`, `h-diagnose`,
   `h-explore`, `h-compare`, `h-verify`, `h-status`, `h-spec`, `h-onboard`, and
   `h-note` are independent entries, not a project sequence.
4. Keep ordinary bounded reasoning conversational. Persist on an explicit save
   request or when a concrete receiving use, operator-named or agent-inferred
   from current Work, needs addressable replay. Do not ask separately when the
   inferred use and stable identity are already recoverable.
5. Treat drift, refresh debt, stale prose, missing bindings, and reconciliation
   cues as attention, not project-wide human gates. Continue reversible
   already-authorized work and evidence or descriptive maintenance. Ask only
   when the current operation would bind or supersede a choice, create or
   broaden authority, cross a human spec-lifecycle gate, make another material
   human-reserved choice, or rely on unresolved contradictory binding content.
   Stop only the affected operation and name the exact choice. Never ask for
   bare `OK`, `yes`, or `да` merely to acknowledge evidence, historicity,
   cleanup, or continuation already authorized.
6. Before making that request, publish a self-contained **Human Gate Brief**:
   gate kind, readable subject, affected operation and blocker; every real
   current option; for each option what changes, what stays unchanged, the
   consequence or return condition, and weakest link; any existing
   comparison/parity basis, selection policy, and non-dominated or Pareto set,
   or an explicit statement that none exists or applies; the advisory
   recommendation; freshness or expiry; and a question asking for the human
   engineer's assessment of the options, trade-offs, and recommendation in
   natural language. IDs and hashes never replace readable meaning.
   Accept ordinary language as the substantive answer. When one current brief
   makes the exact effect, subject, option, and scope unambiguous, the host may
   route that answer for DecisionRecord binding, manual profile application,
   or a later non-default project-memory model change as
   `host_routed_operator_request`, without
   a skill name or second confirmation. It is not reusable authority; a bare
   `yes` or `да` works only for that one current brief. A command or skill
   invocation adds no authority. `h-commission` remains a separately manual
   authority act. Never end a blocking message with “for resumption it is
   enough to…”, “reply exactly…”, or an equivalent command-only instruction.
   The brief itself is explanation rather than authority.
   `h-decide needed`, `approval required`, or `spec gate open` without this
   brief is invalid.

There is no public `h-plan`. When composing a plan is current, inspect the exact
WorkPlan source (`A.15.2`) and return an ordinary `U.WorkPlan`-shaped result
here. WorkPlan is not performed Work or manual execution authority through a
WorkCommission.

`E.11.PUA Pattern Use in a Working Situation` and `E.11.PUR` are authoritative
FPF patterns. Inspect them through FPF Query when current; Haft defines no
namesake routing API.

FPF navigation is relation-first: text, graph, card, skill, or walkthrough
order does not prescribe causal, temporal, method, or performed-work order.
Explicit causal claims, MethodDescriptions, WorkPlans, and Work relations may
still state order. Do not call FPF an acausal ontology.

For non-trivial code work, `haft_method` may supply an internal task-local SWE
method. It is not a public reasoning skill or an FPF navigation authority.

Binding actions require effect-specific operator authority. Generated text,
schema visibility, and model-supplied fields are not operator authorization
and are not approval receipts.
Preserve artifact IDs with their title or one-line claim.
