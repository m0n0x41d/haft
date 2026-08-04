---
name: h-status
description: |
  Read-only Haft project cockpit for active problems, decisions, notes, evidence freshness, drift, commissions, spec lifecycle, module decision coverage, and bounded exact file-link gaps from a current code index. Use for project status, session resumption, "what is decision-linked", "what is uncovered", or "what needs attention". This skill reports current graph state and drill-downs; it does not mutate artifacts, infer a project phase, or prescribe a universal next step.
when_to_use: |
  The operator needs situational awareness, coverage inspection, or a read-only spec-readiness view. Use h-verify to gather new evidence and h-spec to edit specs.
argument-hint: "[optional file, module, context, or artifact reference]"
allowed-tools: Bash Read Grep Glob mcp__haft__haft_query mcp__haft__haft_spec_section
---

<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:f071f56205d0f7736b2db3a0f4aa1fc582b6f97481a41042a0807e5ba2208be8 -->

# h-status — Read-only project memory and coverage

If there is no current status, resumption, coverage, or recorded-state
question, return without calling status. Never call `h-status` after completed
reasoning or edits only to backfill process compliance.

Start with the smallest read-only surface that answers the question:

```text
mcp__haft__haft_query(action="status", full=false)
```

If the status response says the canonical project profile has several scopes,
treat that as a read-only retry requirement. Use one exact `scope_id` from the
reported available values and retry the same status call:

```text
mcp__haft__haft_query(
  action="status",
  full=false,
  scope_id="<exact emitted ScopeID>"
)
```

Choose the scope from the operator's current object or question.
Never select the first value, sort the values into a winner, or collapse mixed scopes. If
the current use does not identify one scope, report the available ScopeIDs and
narrow only that question; unrelated already-authorized Work continues.

Compact output is a cockpit, not proof of absence. Drill down explicitly:

- `status, full=true` for detailed artifact state;
- `coverage` for module decision coverage plus a bounded exact `affected_files`
  link-gap projection when the derived code index is current;
- `related, file="<path>"` for decisions and notes touching a file;
- `related, artifact_ref="<ref>"` for the exact artifact neighborhood;
- `drift_events`, `decision_reconcile`, or `governing_set` for the named
  governance question;
- `contract_generation` for generated carrier-sync hints.

Generated text and read-only projections are discovery surfaces. They are not
evidence truth, gate passage, approval, authority, or work.
Coverage is a read-only projection over stored module data and the current
derived code index. If the code index is uninitialized, stale, or degraded,
report the limitation as unavailable rather than an empty-clean result; do not
rescan it from `h-status`.

## EntityOfConcern memory

Contract truth: typed neighborhood and recall are **V9 CONTRACT**
capabilities. Source, schema, skill, and local-test presence is not
installed-runtime proof. A current readiness claim requires
**EXACT-CANDIDATE EVIDENCE** from P14 tied to one exact candidate; RC or release
status additionally requires release authority. Do not infer
**CURRENT PRODUCT** status from contract inclusion or evidence alone.

When the operator names an exact current EntityOfConcern and bounded context,
read its typed neighborhood under `agent_orientation.v2` rather than searching
the whole artifact corpus:

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
      "detail":"overview",
      "include_history":false
    },
    "read_budget":{
      "max_facets":9,
      "max_items_per_facet":6,
      "max_relation_paths_per_item":3,
      "max_carrier_excerpt_characters":800,
      "max_provenance_depth":2
    }
  }
)
```

Report the exact snapshot/profile basis and distinguish facet coverage:
`complete`, `partial`, `not_applicable`, `unavailable`, and `stale`. Known
empty is valid only for `complete` at that exact basis. Keep item semantic and
lifecycle posture separate from `evidence_currentness` and
`projection_freshness`. Honor `hydrate_before_reliance`; a read affordance may
recover more basis but never becomes a recommendation, skill choice,
NextAction, or authority.

If only a name or alias is known, use `memory.resolve` first. Use
`memory.recall` only after exact entity/context resolution and describe its
ranked result as discovery candidates inside that scope. On `retry_required`,
do not merge stale and current snapshots. On `abstained`, report missing basis
rather than an empty project memory.

## Attention is not interruption

Status reports where judgment may eventually be needed; it does not suspend
the project. Drift, refresh debt, missing bindings, stale descriptions, and
reconciliation cues do not block unrelated already-authorized Work.

Before suggesting a human interruption, inspect the exact affected artifact
and current use. Interrupt only an operation that would mutate the affected
binding or authority, cross an explicit human lifecycle gate, or rely on an
unresolved contradiction in binding content. Otherwise report the cue and
continue. Never request bare approval merely to acknowledge status, evidence,
historicity, or technical cleanup.

When status exposes a real gate, do not merely repeat its label. First inspect
the referenced review, section, artifact, or other read-only basis. Then give a
self-contained **Human Gate Brief**: gate kind and readable subject; the exact
affected operation and why only it is blocked; every real option available now;
for each option what changes, what stays unchanged, the immediate consequence
or return condition, and the weakest link; any existing comparison/parity basis,
selection policy, and non-dominated or Pareto set, or an explicit statement
that none exists or applies; the advisory recommendation; evidence freshness or
expiry; and a question asking for the human engineer's assessment of the
options, trade-offs, and recommendation in natural language. Pair every ID or
hash with readable meaning. Accept ordinary language as the substantive answer.
When one current brief makes effect, subject, option, and scope unambiguous,
the host may route it for DecisionRecord binding, manual profile application,
or a later non-default project-memory model change as
`host_routed_operator_request`, without a
skill name or second confirmation. It is not reusable authority; a bare `yes`
or `да` works only for that one current brief. `h-commission` remains a
separately manual authority act. The brief itself is explanation rather than
authority. If the read-only basis cannot supply those details,
report that the gate is not yet askable and name the drill-down needed to
recover them.

## Coverage inspection

1. Call `mcp__haft__haft_query(action="coverage")`.
2. Read file gaps only as files in a decision-bearing module without an exact
   active `affected_files` link. This does not prove that a file is
   undocumented, unconstrained, or incorrect.
3. If the operator names a file or module, call `related` for that exact path.
4. If coverage reports an absent or stale derived index, limit only the current
   coverage claim and surface that a separately authorized refresh is required
   before relying on it. Continue unrelated Work.
5. Distinguish:
   - no relevant decision found;
   - a decision exists but the file is outside its recorded footprint;
   - the file is covered but the governing decision is stale or drifted;
   - compact output omitted detail and a full query is still needed.
6. Report blind spots as orientation cues. Coverage is not a quality score and
   does not prove conformance, implementation correctness, or spec completeness.

## Spec lifecycle strip

When the operator asks about spec readiness or status mentions missing
SpecSections, call:

```text
mcp__haft__haft_spec_section(action="lifecycle")
```

Report state, current action, carrier, section identity, and any human gate.
The lifecycle action is local to the spec state machine; it is not the next
step of the whole project. Do not edit, approve, rebaseline, or reopen from
this skill.

## Presentation

Surface:

- current items relevant to the operator's question;
- stale, drifted, uncovered, or unassessed state with its exact meaning;
- whether a cue affects the current operation or is only background attention;
- which details were not shown and the read-only call that can recover them;
- available capabilities, only when their condition is current.

Do not call refresh mutations, close problems, attach evidence, edit specs, or
create artifacts. Pair every artifact ID with its title or one-line claim.
