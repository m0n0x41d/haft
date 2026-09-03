---
name: h-reason
description: Source-first umbrella for FPF-aware reasoning in a Haft project. Use for ambiguous engineering, management, architecture, specification, or project questions when no narrower Haft capability is already current. Ordinary reasoning stays conversational; persistence is conditional, h-decide may route a direct operator request, and h-commission remains manual-only.
---

<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:26e174fdd87993d53721c925be9727239d77e8a425b7c52d28fd9f833b6d1153 -->

# h-reason — Source-first FPF entry

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

Name the current object, question, relations, constraints, evidence, and
smallest useful result. Query `haft_query(action="fpf", mode="concern",
query="<object + question + terms>")`; compare README practical-use cards, use
the source ToC as the identifier/keyword index, recover exact source units with
`mode="lookup"` or non-broadening `mode="inspect"`, and inspect the selected
pattern's Problem frame, Problem, Forces, Solution, Consequences, ordinary
boundary, nearest stronger neighbor, worked slices, and checklist. Retrieval
returns source candidates. Returned source
material is not applicability, selection, recommendation, evidence,
precedence, or authority.

For a non-English concern, keep the operator's wording in `query` and add
precise English or FPF terms already known in `entity_of_concern`,
`known_context`, and `intended_use`. The bundled source is English, so raw
non-English input may return insufficient basis; keep that diagnostic visible.
Do not translate `query`, invent a bilingual route, or turn an unversioned
benchmark observation into a permanent routing rule.

In ordinary working use, identify the selected direct pattern by `PatternID`,
title, and stable source reference. Do not routinely reproduce source spans,
repository-local paths, line ranges, hashes, revisions, or other provenance.
Request trace or audit provenance only when the current use requires it.

Keep exact identifier namespaces separate:

- FPF `PatternID`, `SourceID`, or `UnitID` ->
  `haft_query(action="fpf", mode="lookup|inspect", identifier="<id>")`;
- canonical Haft artifact ID ->
  `haft_query(action="related", artifact_ref="<id>")`;
- code symbol or `SymbolAnchor` ->
  `haft_query(action="node", symbol="<name>")` or `anchor_id="<anchor>"`;
- typed-memory `EntityID` or `EntityAlias` ->
  `haft_query(action="memory", memory_request={"mode":"resolve","query":"<id-or-alias>",...})`;
  never coerce it into another namespace.

After exact resolution, use the closed `memory_request` branch whose nested
`mode` is `neighborhood` to hydrate the EntityOfConcern graph, or `recall` for
bounded lexical recall inside that exact scope. Mode-specific required fields
come from the tool schema. When the project is not ready for these reads, use
`haft_onboard(action="status")` and follow its readable next action; do not
invent or expose an internal schema-selection step. Resolution, projection
inclusion, and recall rank are not truth, applicability, authority, or Work
order.

`known_absent` alone authorizes nothing. A concrete durability-requiring
receiving use may be operator-named or agent-inferred from current Work. Infer
it when cross-session continuation, handoff, audit, automation, delayed or
expensive feedback, or costly reversal already depends on stable identity; the
operator does not need to pre-name it. When identity, context, and aliases are
recoverable, establish the minimum EntityOfConcern without asking for separate permission.
Do not infer this use from an empty graph or generic future usefulness. Call
`haft_entity(action="establish", entity_id=..., label=...,
bounded_context_ref=..., aliases=[...],
persistence_reason="named_receiving_use",
request_provenance_ref=..., idempotency_key=...)`.

The task-level tool owns alias conflict checks, validation, internal project
basis, admission, and post-commit resolution. Use an `established` result's
exact `next_read` unchanged. Preserve `identity_conflict`, `alias_conflict`,
`idempotency_conflict`, `onboarding_required`,
`restart_required`, `rejected`, or `commit_outcome_unknown`; never invent
success. Retry `restart_required` with the unchanged idempotency key.

For an exact current concern, use the closed outer
`action="memory", memory_request={...}` envelope. Put the exact nested mode,
`contract_version="haft.memory.v1"`, `basis={kind:"project_current"}`,
projection profile
`agent_orientation.v2`, explicit requested facets, and a dimensioned read
budget. Inspect `result_kind`, `snapshot_basis`, `projection_basis`,
`interpretation_contract`, facet coverage, item semantic/lifecycle/evidence/
projection postures, and `applied_budget`. Only `complete` supports known
emptiness for that exact facet/profile/snapshot/context. Partial, unavailable,
stale, abstained, or retry-required results must keep their limitation. Honor
`hydrate_before_reliance`. Read affordances obtain more basis; they never
select a capability or next action.

Use `memory.resolve` only when identity is missing, and scoped `memory.recall`
only after exact entity/context resolution. Recall candidates and scores are
discovery, not truth, applicability, recommendation, freshness, authority, or
work priority. Enter FPF Query when pattern applicability, an unfamiliar kind,
or missing method basis is current.

On `wrong_identifier_namespace` with `same_call_retryable=false`, do not retry
the same action or ask for acknowledgement. Execute the exact `recovery_call`
when it names an available read-only surface; otherwise report that the
required surface is unavailable.

For code-area or flow orientation without an exact symbol, use
`haft_query(action="explore", query="<current code concern>")`. Its default
working view returns bounded advisory candidates and never auto-selects
identity. Use trace only for a named replay use and diagnostic only when
retrieval/traversal is itself under diagnosis. Before a non-mechanical edit to
governed code, use `code_context` or `impact` on the actual target. Purely
mechanical work may explicitly abstain and record `not_applicable`. Code-graph
and typed-memory orientation are separate; neither substitutes for the other.

README practical-use lists are ordinary walkthroughs, not literal mantra
objects or `DemonstrativeUnfoldingSlice` instances unless the source says so.
Query returns candidates; select by current condition and apply the direct
Solution. v9 Query uses authored phrases, headings/keywords, and role-local FTS;
dense retrieval is **DEFERRED RESEARCH**.

When applicability, recommendation, or coordination among candidates is
current, inspect current `E.11.PUR` and every evaluated candidate's full
`Solution`. Give a compact rationale for `problemFrame`, `forces`,
`solutionConditions`, `ordinaryBoundary`, and `resultAndReceivingUse`, then
aggregate the candidate as `applicable`, `inapplicable`, or
`insufficientBasis`. Recommend only an applicable candidate after comparing its
expected first result with live alternatives. Keep complementary uses
`unordered` unless a declared basis supports `partialOrder` or `totalOrder`;
`prerequisiteResult` also needs an exact PUA expectation and current closure
with its category-correct direct basis.
Every coordinated member answers the same bounded coordination question while
remaining a distinct candidate use. Never infer a winner or order from score,
display, or catalog position. Problem-frame fit or a ProblemCard is not an
actual Problem; a relied-on actual Problem resolves to one exact `C.22.PFR`
occurrence. Plain *next move* wording is only advice or a conditional
continuation: it creates no Move identity and performs no Work or
Transformation.

For one selected pattern, use current `E.11.PUA`. Name the subject, question,
expected first result, inspected Solution and Consequences, ordinary boundary,
stronger neighbor, and stop/return. Close as exactly one of
`newlyCurrentSubjectResult`, `preExistingWithGrounding`, or
`expectedSubjectResultAbsent`; in the last case name the interim result or
blocker and return condition. An expectation, recommendation, planning draft,
grounding, or support record is not an achieved result or performed Work.
Ordinary selected use remains conversational; materialize a support record only
when it names the exact later reliance that consumes it.
Recommendation and coordination create no evidence, gate, DecisionRecord,
WorkCommission, authorization, WorkPlan, Work, actual Problem,
Transformation, or downstream result. A `U.Work` claim needs one exact
A.15.1-grounded occurrence and is never inferred from planning, setup,
authorization, or another Work. Entity inception by Work also needs current
A.15.PROD and the exact work-to-change basis. Intended later use asserts no
obtaining relation; realized use names the exact later object and basis; a
genuine stop has no receiver. A changed concern, basis, result, pattern, or use
opens a named return.

For reliance-bearing replay, pin exact EntityOfConcern/reference scheme,
question, source edition/revision or digest, selected Solution locator,
expected result kind and locator, grounded actual-result assertion or honest
interim entity, relative object when relevant, direct basis, closure
disposition, receiving use, and stop/return. Materialize separate fit and
applicability findings only for that named receiving use. This is Haft-local
persistence policy, not FPF permission; receiving use never weakens
effect-specific authority.

FPF navigation is relation-first. Text, graph, card, skill, or walkthrough
order does not prescribe causal, temporal, method, or performed-work order.
Explicit causal claims, MethodDescriptions, WorkPlans, and Work relations may
still state order. A skill or pattern carrier does not itself prove
`U.MethodDescription` membership under A.3.2. Do not call FPF an acausal
ontology.

Choose the minimal current capability set. One capability is normally enough;
complementary uses may remain `unordered` unless an explicit pairwise basis
supports `partialOrder` or `totalOrder`. Public skills are independent, not
phases or a project sequence. Keep ordinary reasoning conversational; persist
on explicit save intent or for a concrete receiving use, operator-named or
agent-inferred from current Work. `haft_method` remains internal task-local
code guidance, not a public skill. A direct, unambiguous operator request may
route through h-decide; h-commission remains manual-only.

Cockpit drift, refresh debt, stale prose, missing bindings, and reconciliation
cues are attention, not project-wide human gates. Continue reversible
already-authorized work and evidence or descriptive maintenance without asking
again. Ask only when the current operation would bind or supersede a choice,
create or broaden authority, cross a human spec-lifecycle gate, make another
material human-reserved choice, or rely on unresolved contradictory binding
content. Stop only the affected operation and name the exact choice. Never ask
for bare `OK`, `yes`, or `да` merely to acknowledge evidence, historicity,
cleanup, or already-authorized continuation.

The Human Gate Brief is Haft-local governance UX, not authority. Use A.6/A.6.B
L/A/D/E unpacking only for an actually mixed-normativity boundary package;
permission, SpeechAct, readiness, gate, and spec lifecycle stay with their
direct governing contracts.

Before making a human-gate request, publish a self-contained
**Human Gate Brief**. State the gate kind, readable subject, affected operation and blocker;
every real current option; and for each option what changes, what stays
unchanged, its consequence or return condition, and weakest link. Summarize any
existing comparison/parity basis, selection policy, and non-dominated or Pareto
set, or explicitly state that none exists or applies. Mark the recommendation
as advisory, state freshness or expiry, and ask for the human engineer's
assessment of the options, trade-offs, and recommendation in natural language.
Accept ordinary language as the substantive answer. When one current brief
makes the exact effect, subject, option, and scope unambiguous, the host may
route that answer for DecisionRecord binding, manual profile application, or
a later non-default project-memory model change as
`host_routed_operator_request`, without a skill
name or second confirmation. It is not reusable authority; a bare `yes` or `да`
works only for that one current brief. A command or skill invocation adds no
authority. `h-commission` remains a separately manual authority act. Never end
a blocking message with “for resumption it is enough to…”, “reply exactly…”,
or an equivalent command-only instruction. IDs and hashes never replace
readable meaning, and the brief itself is explanation rather than authority.
A bare `h-decide needed`, `approval required`, or `spec gate open` request is
invalid.

There is no public `h-plan`. When composing intended-work content is current,
inspect the exact WorkPlan source and return a `planning draft` or `planning
cue`. Call it a `U.WorkPlan` only when its C.2.1/A.15.2 identity and membership
basis is established; a merely similar shape is not membership. Planning
content, WorkPlan, performed Work, and manual execution authority stay distinct.

`E.11.PUA` and `E.11.PUR` are authoritative FPF patterns. Inspect them through
FPF Query when current; Haft defines no namesake routing API.
