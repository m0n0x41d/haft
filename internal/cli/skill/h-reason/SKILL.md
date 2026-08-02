---
name: h-reason
description: |
  Source-first umbrella for FPF-aware reasoning in a Haft project. Use when the operator asks to think through an ambiguous engineering, management, architecture, specification, or project question without naming a narrower Haft capability. Recover the current object and question, query the bundled FPF source, inspect the governing pattern body, and choose only the capability that is current. Ordinary reasoning stays conversational; proactively persist the minimum needed memory when current Work supplies a concrete durable receiving use. h-decide may route a direct operator request; h-commission remains manual-only.
when_to_use: |
  The operator asks to reason with FPF or Haft, says "let's think", "помоги разобраться", or presents a concern that could belong to several narrower skills. Prefer a narrower skill when its condition is already clear.
argument-hint: "[current project question]"
allowed-tools: Bash Read Grep Glob Agent Write Edit mcp__haft__haft_problem mcp__haft__haft_solution mcp__haft__haft_decision mcp__haft__haft_query mcp__haft__haft_entity mcp__haft__haft_onboard mcp__haft__haft_note mcp__haft__haft_refresh mcp__haft__haft_spec_section
---

<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:748e5c014551af025c2b340b6d172f66229a257e1b366c647b1d6a0781258b5c -->

# h-reason — Source-first FPF entrypoint

Use FPF as a reference model and pattern language, not as a project workflow.
FPF is relation-first at the framework-navigation level: a text sequence, graph
path, skill list, or walkthrough does not create causal, temporal, method, or
performed-work order. Explicit causal claims, ordered methods, WorkPlans, and
performed Work remain valid when their direct governing patterns are current.

Contract truth: source-native FPF Query and typed-memory resolve, neighborhood,
and recall are **V9 CONTRACT** capabilities. Their presence in source, schemas,
skills, or local tests is not installed-runtime proof. Any
installed-runtime readiness claim requires current
**EXACT-CANDIDATE EVIDENCE** from P14 tied to one exact candidate; RC or release
status additionally requires release authority.
Do not infer **CURRENT PRODUCT** status from contract inclusion or evidence
alone.

## Critical distinctions

Keep these distinctions live before choosing a pattern:

- object != description != representation != carrier;
- method != MethodDescription != WorkPlan != performed Work;
- plan != reality; promise != delivery; claim != evidence;
- a practical-use card, mantra, or demonstrative traversal != project order;
- retrieval rank != applicability, recommendation, authorization, or work
  precedence;
- the product/system being changed and the engineering arrangement changing it
  are separate project concerns. `TargetSystemSpec` and related names are Haft
  local-practice carriers, not FPF Core kinds by label alone.

## Exact identifier namespaces

Classify an exact identifier before choosing a query field:

- FPF `PatternID`, `SourceID`, or `UnitID` ->
  `mcp__haft__haft_query(action="fpf", mode="lookup|inspect", identifier="<id>")`;
- canonical Haft artifact ID ->
  `mcp__haft__haft_query(action="related", artifact_ref="<id>")`;
- code symbol or `SymbolAnchor` ->
  `mcp__haft__haft_query(action="node", symbol="<name>")` or `anchor_id="<anchor>"`;
- typed-memory `EntityID` or `EntityAlias` ->
  `mcp__haft__haft_query(action="memory", memory_request={"mode":"resolve","query":"<id-or-alias>",...})`;
  keep it out of `identifier`, `artifact_ref`, and `symbol`.

After exact resolution, use the closed `memory_request` branch whose nested
`mode` is `neighborhood` to hydrate the EntityOfConcern graph, or `recall` for
bounded lexical recall inside that exact scope. Mode-specific required fields
come from the tool schema. When the project is not ready for these reads, use
`mcp__haft__haft_onboard(action="status")` and follow its readable next action.
Resolution, projection inclusion, and recall rank are not truth, applicability,
authority, or Work order.

If a read-only call returns `wrong_identifier_namespace` with
`same_call_retryable=false`, do not retry the same action and do not ask the
operator merely to acknowledge the error. Execute its exact `recovery_call`
when that call names an available read-only surface; otherwise report the
missing surface and preserve the identifier unchanged.

## Conditional code-graph orientation

Use `haft_query(action="explore", query="<current code concern>")` when the
current question is how an area or flow works and no exact symbol is known.
The default working view is bounded and returns advisory candidates without
selecting identity. Use `view="trace"` only when replay basis is a named
receiving use, and `view="diagnostic"` only when retrieval or traversal itself
is under diagnosis.

Before a non-mechanical edit where recorded governance may be material, use
`code_context` or `impact` on the actual target. Returned reasoning context is
a relevance surface: candidate rank, file/module proximity, and a displayed
invariant do not by themselves prove exact active authority. Inspect scope,
status, coverage, and limiting reasons, and use the exact `governing_set` or
artifact route before relying on a governing claim. Do not infer safety from an
empty caller list or incomplete traversal. A purely mechanical edit whose
meaning and blast radius cannot change may explicitly abstain and record
`not_applicable`. Typed-memory and code-graph orientation remain separate.

## Procedure

### 1. Recover the current question

State, in ordinary project language:

- the EntityOfConcern or object at stake;
- the current question about it;
- known values, constraints, and unresolved relations;
- the smallest useful result that would answer the question now.

Do not ask for a universal first step. Do not manufacture a ProblemCard merely
to make the work look started.

When an exact current EntityOfConcern and bounded context already exist,
hydrate the smallest relevant typed neighborhood before rediscovering project
history:

```text
mcp__haft__haft_query(
  action="memory",
  memory_request={
    "mode":"neighborhood",
    "contract_version":"haft.memory.v1",
    "basis":{"kind":"project_current"},
    "entity_ref":{
      "ref_kind_id":"U.EntityRef",
      "reference_id":"<exact EntityOfConcern>"
    },
    "bounded_context_ref":"<exact bounded context>",
    "view":{
      "projection_profile_ref":"agent_orientation.v2",
      "requested_facets":[
        "epistemes",
        "problems",
        "alternatives",
        "decisions",
        "specifications",
        "evidence",
        "work",
        "implementation",
        "unresolved"
      ],
      "detail":"standard",
      "include_history":false
    },
    "read_budget":{
      "max_facets":9,
      "max_items_per_facet":8,
      "max_relation_paths_per_item":4,
      "max_carrier_excerpt_characters":1200,
      "max_provenance_depth":3
    }
  }
)
```

The budget is task-local and may be narrowed or expanded explicitly. Inspect
`result_kind` before content:

- `exact_neighborhood`: pin `snapshot_basis` and `projection_basis`, then read
  `interpretation_contract`, each facet's `coverage`, every item's semantic,
  lifecycle, evidence-currentness, and projection-freshness postures, and the
  `applied_budget`;
- `retry_required`: do not combine the stale read with current graph facts;
  follow the returned snapshot-bound retry operation;
- `abstained`: preserve the exact missing basis and make no graph-absence
  claim.

`complete` coverage supports a known-empty claim only for that exact facet,
profile, snapshot, and bounded context. `partial`, `unavailable`, and `stale`
do not. If `hydrate_before_reliance=true`, hydrate the named carrier or facet
before relying on its description. A `read_affordance` is a read-only way to
obtain more basis; it never chooses a skill, pattern, plan, Work item, or next
project action.

Use `memory.resolve` only when exact identity is missing. Inspect its result
before continuing:

- an exact result supplies the canonical `entity_ref`;
- several candidates require selection by the current use, never by rank;
- `known_absent` says only that the identity was not found. It does not
  authorize persistence;
- unavailable setup routes through
  `mcp__haft__haft_onboard(action="status")` and does not block unrelated Work.

`known_absent` alone authorizes nothing. Separately decide whether current Work
has a concrete durability-requiring receiving use. That use may be
operator-named or agent-inferred. The agent must infer it when current Work
already makes cross-session continuation, handoff, audit, automation, delayed
or expensive feedback, or costly reversal dependent on stable identity. The
operator does not need to pre-name the use or grant separate permission.

When such a use exists and the stable identity, bounded context, and aliases
are recoverable, establish the minimum EntityOfConcern without asking for separate permission.
Record the concrete use in request provenance. Do not infer a receiving use
merely from `known_absent`, generic future usefulness, or a desire to populate
an empty graph. An unresolved identity or alias conflict blocks only that
establishment; it does not turn descriptive persistence into a human authority
gate.

Use the task-level surface:

```text
mcp__haft__haft_entity(
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

Use `persistence_reason="explicit_operator_request"` instead only when an
explicit save request, rather than the inferred receiving use, is the real
basis.
The task-level tool owns identity and alias conflict checks, validation, exact
project-basis selection, admission, and post-commit resolution; the agent must
not construct those internals.

Follow its closed result:

- `established` returns the canonical `entity_ref`,
  `bounded_context_ref`, and an exact `next_read` for
  `haft_query(action="memory", memory_request={"mode":"neighborhood",...})`;
  use its tool name and arguments unchanged when hydration is current.
  `delivery_kind` distinguishes fresh commit, replay, and already-exact
  identity;
- `identity_conflict`, `alias_conflict`, or `idempotency_conflict` blocks only
  this establishment; preserve both identities and ask only when the current
  use cannot disambiguate them;
- `onboarding_required` routes through `h-onboard`; a partial or legacy memory
  installation is repaired by `haft init`, never by exposing a schema choice;
- `restart_required` means reconnect and retry the unchanged request with the
  same idempotency key;
- `rejected` and `commit_outcome_unknown` remain explicit; never invent a
  successful entity.

If persistence is not authorized, continue ordinary reasoning without creating
an entity. Use scoped `memory.recall` only after exact entity/context
resolution and only when the current question needs discovery among records
inside that scope. Recall score is not truth, applicability, recommendation,
freshness, authority, or work priority.

### 2. Query FPF, then inspect the source

For purely mechanical, status-only, or exact project-lookup work where no FPF
pattern choice is material, caller abstention is the correct result: skip FPF
Query and do the bounded work directly. This is not a fabricated
`QueryResult(kind="abstained")`; no query ran. If pattern applicability is
material or uncertain, a neighborhood exposes an unfamiliar kind or missing
method basis, or the direct governing pattern is unclear, continue with source
retrieval.

If the operator names an exact SourceID or UnitID, use non-broadening
`mode="inspect", identifier="<exact id>"`. Otherwise query with the object,
current question, and the important domain words:

```text
mcp__haft__haft_query(
  action="fpf",
  mode="concern",
  query="<current object + question + terms>"
)
```

For a non-English concern, preserve the operator's original `query`. Add
`entity_of_concern`, `known_context`, and `intended_use` with precise
source-language or FPF terms when they are already known. Do not translate the
question into a hidden Haft route or invent a bilingual catalog. An unsupported
raw-language query may honestly abstain.

The result is source material, not applicability, selection, recommendation,
evidence, precedence, or authority. Use the source-owned navigation:

1. compare relevant README practical-use cards by situation, first-result
   difference, direct pattern, and stop/return boundary;
2. use the Table of Contents for PatternID, title, keywords, queries, and
   dependencies;
3. recover an exact source unit with `mode="lookup"` and its `identifier`, or
   use non-broadening `mode="inspect"` when the identifier must match exactly;
4. inspect the selected pattern's full Problem frame, Problem, Forces,
   Solution, ordinary boundary, worked slices, and checklist.

README practical-use lists are ordinary walkthroughs, not literal mantra
objects or `DemonstrativeUnfoldingSlice` instances unless the source identifies
them that way. FPF Query returns source candidates; `h-reason` selects by the
current condition and applies the selected direct Solution to produce the
ordinary result. The full pattern body governs. README, ToC, authored phrases,
headings/keywords, and role-local FTS
are indexes or coarsenings; none is a second specification. Dense retrieval is
**DEFERRED RESEARCH**, not part of the v9 FPF Query contract. If several
patterns remain plausible, keep them live and explain what fact would
discriminate them. Abstain when the source basis is insufficient.

`E.11.PUA Pattern Use in a Working Situation` and `E.11.PUR` are authoritative
FPF patterns. Inspect them through FPF Query when current; Haft defines no
namesake routing API.

### 3. Select the governing pattern

Select by the current condition and exact first-result kind, not by score,
familiarity, identifier order, or the skill the agent happens to know. State:

- selected direct pattern by `PatternID`, title, and stable source reference;
- source span, provenance, hashes, or repository-local paths only when the
  current use explicitly requires trace or audit;
- why its condition fits;
- exact first useful result;
- what the result permits now;
- stop, return, wrong-turn, and stronger-neighbor boundaries.

A recommendation is advisory. It is not evidence, a gate, a DecisionRecord, a
WorkCommission, or authorization.

### 4. Choose one current capability

Capabilities are independent entries, not phases:

- `h-frame` — problem shaping is current;
- `h-diagnose` — a concrete failure has rival causes;
- `h-explore` — distinct alternatives are needed;
- `h-compare` — existing alternatives need parity-aware comparison;
- `h-decide` — route the operator's direct, unambiguous request to bind a choice;
- manual `h-commission` — the operator explicitly grants execution authority;
- `h-verify` — a recorded claim or decision needs evidence against reality;
- `h-status` — live graph, drift, coverage, or spec readiness is current;
- `h-spec` — a specification carrier or lifecycle question is current;
- `h-onboard` — Haft/spec bootstrap is current;
- `h-note` — the operator explicitly wants a non-binding fact saved.

There is no public `h-plan` phase. When composing a plan is the current
question, inspect the exact WorkPlan source (`A.15.2`) and return an ordinary
`U.WorkPlan`-shaped result conversationally. Keep WorkPlan, performed Work, and
WorkCommission distinct. Route to manual `h-commission` only when bounded
execution authority is separately current; typed WorkPlan persistence remains
outside this skill until a receiving lifecycle exists.

Invoke a capability only while its condition is current. Completion of one
does not imply that another must follow. A concrete method or WorkPlan may
state order locally; that order belongs to that method or plan, not to this
catalog.

### 5. Interrupt only when a human choice is current

Before asking the operator, use the A.6 boundary discipline to separate
description and evidence from admissibility, binding choice, and authority.
Cockpit drift, refresh debt, missing bindings, stale prose, or reconciliation
cues are attention signals; they are not project-wide stop conditions.

Continue without another approval when the remaining action only:

- gathers or attaches evidence;
- treats obsolete implementation prose as historical without rewriting the
  binding choice;
- records a current implementation fact;
- performs reversible Work already inside the accepted task or
  WorkCommission; or
- leaves an unrelated unresolved artifact untouched and does not rely on it.

Ask only when the current action itself would bind, change, or supersede a
choice; create or broaden execution authority; cross a human SpecSection
lifecycle gate; make another material human-reserved choice; or rely on an
unresolved contradiction in binding content. Stop only the affected operation,
name the exact choice, and explain why that operation cannot continue without
it. Never ask for bare `OK`, `yes`, or `да` to acknowledge evidence,
historicity, technical cleanup, or already-authorized continuation.

Before making that request, publish a self-contained **Human Gate Brief**. The
operator cannot be expected to infer hidden project state. In ordinary language
state the gate kind, readable subject, affected operation and blocker; every
real option currently available; and for each option what changes, what stays
unchanged, the immediate consequence or return condition, and its weakest
link. Include defer or reject only when real; do not manufacture variants.

If a comparison exists, summarize its characteristic/parity basis, selection
policy, and non-dominated or Pareto set. If none exists or Pareto reasoning is
not applicable to the binary lifecycle act, say so explicitly. Mark the
recommendation as advisory, state evidence freshness or expiry, and ask for the
human engineer's assessment of the options, trade-offs, and recommendation in
natural language. Pair IDs and hashes with readable meaning.

Accept ordinary language as the substantive answer to the engineering
consultation. When exactly one current Human Gate Brief makes the effect,
subject, option, and scope unambiguous, that natural answer is also the direct
operator request the host may route; a bare `yes` or `да` is usable only in that
single-brief case. A command or skill invocation never adds authority and must
not substitute for the consultation. `h-commission` remains a separately
manual execution-authority grant. Never end a blocking message with “reply
exactly…” or an equivalent command-only instruction. The brief itself is not
authorization. `h-decide needed`, `approval required`, or `spec gate open`
without this brief is an invalid operator request.

### 6. Decide whether to persist

Default to `ordinaryBounded`: reason in the conversation and produce the
smallest useful result without creating Haft artifacts.

Persist only when either condition holds:

- the operator explicitly asks to save, record, remember, bind, commission,
  approve, rebaseline, or otherwise mutate project memory; or
- a concrete receiving use, operator-named or agent-inferred from current Work,
  depends on addressable replay, transfer, audit, delayed feedback, automation,
  expensive feedback, or costly reversal.

The second condition is proactive: when it is satisfied, do not ask the
operator whether memory should be used. Establish the minimum stable
EntityOfConcern and materialize only the records that the concrete use needs.

When persistence is justified, materialize only the records that receiving use
needs. Do not automatically create ProblemCard, SolutionPortfolio,
characterization, recommendation, DecisionRecord, or WorkPlan as a bundle.
Decision binding still requires a direct operator request, and `h-commission`
remains manual even when reliance is high.

### 7. Present one honest traversal

Explain the selected use in a readable order while labelling it as an
explanation. Keep alternative continuations and return conditions visible. Do
not imply that the explanation order is the order of project work.

## Internal routines

Use these inside the public skills; do not expose them as separate skills:

- abductive rival generation and falsification inside `h-diagnose`;
- L/A/D/E boundary-statement unpacking inside `h-reason` or `h-spec` when a
  sentence mixes definition, admissibility, commitment, and evidence;
- semantic fanout review inside `h-spec` when a rename or claim change crosses
  several carriers.

## Stop conditions

Stop when the current question has its smallest honest result and boundaries.
Return to source selection when the object, question, condition, expected
result, evidence basis, or receiving use changes.

Never claim that FPF is an acausal ontology. The precise claim is narrower:
FPF does not infer causal or work order from representation order; explicit
causal and work-order claims keep their own governing patterns.
