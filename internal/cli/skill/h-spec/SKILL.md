---
name: h-spec
description: |
  Manage Haft's typed specification lifecycle and source-currentness repair: inspect current SpecSections, draft or clarify carriers, classify FPF semantic fanout, record operator-requested spec changes, and cross explicit approve/rebaseline/reopen gates only with human authorization. Use for "spec status", "update specs", "запиши в спеки", stale spec sections, newer FPF source revisions, or semantic changes that must be repaired across several spec carriers. Treat markdown as a carrier and the kernel projection as the lifecycle contract. Use h-status for read-only module/file coverage and h-onboard for first bootstrap.
when_to_use: |
  A specification description, carrier, lifecycle state, or cross-carrier semantic repair is current.
argument-hint: "[spec question, section id, or clarification]"
allowed-tools: Bash Read Grep Glob Write Edit mcp__haft__haft_query mcp__haft__haft_spec_section
---

# h-spec — Typed spec lifecycle and semantic repair

Keep object, description, carrier, and authority separate:

- object: the system, behavior, role, boundary, or term being described;
- description: the SpecSection claim;
- carrier: `.haft/specs/*.md` and its fenced block;
- lifecycle authority: the kernel projection plus explicit human gates.

`TargetSystemSpec`, `SoftwareSystemSpec`, and related carrier names are Haft
local-practice concepts. Do not attribute a `target system vs enabling system`
kind distinction to FPF A.1. When FPF precision is needed, recover the actual
holon/system, context, transformation, role, method, WorkPlan, and Work
relations from the source.

## Conditional project-memory orientation

When this specification work is context-heavy, multi-session, or
reliance-bearing and the exact EntityOfConcern is not already current, resolve
its identity with `haft_query(action="memory",
memory_request={"mode":"resolve","contract_version":"haft.memory.v1",
"basis":{"kind":"project_current"},"query":"...","max_candidates":5})`. Select
the exact candidate by the current use rather than rank, then use the closed
`memory_request` neighborhood branch advertised by the tool schema with
`projection_profile_ref="agent_orientation.v2"`.

Inspect `result_kind` before relying on content. `project_basis_unavailable`,
known absence, or explicit abstention is visible but non-blocking: continue the
spec lifecycle work without inventing a profile, entity, artifact, or human
gate. This read does not replace code-graph preflight before editing code or a
generated carrier. Never persist typed memory merely because a read failed;
persistence requires an explicit operator save request or a named receiving
use with request provenance.

## 1. Read lifecycle first

```text
mcp__haft__haft_spec_section(action="lifecycle")
```

`lifecycle` and `next_step` are project/scope-level
`ProjectSpecificationSet` workflow projections. Never pass `section_id` to
them and never read their `ready` or terminal result as the lifecycle state of
one named section. For an exact section, use
`haft_query(action="spec_trace", section_id="<id>")` to inspect its current
edition, status, and baseline, then
`haft_query(action="spec_use", section_id="<id>",
use_context="<named receiving use>")` when stronger-use admission is current.
The kernel rejects an action-inapplicable `section_id` and returns these
recovery routes rather than silently ignoring it.

If the MCP action is unavailable, use `haft spec next --json` or
`haft spec status --json` as the read-only projection of the same lifecycle
contract and report that fallback. Do not infer lifecycle state from carrier
grep or Markdown status fields.

Use `state`, current `action`, `object`, `carrier`, section identity,
`workflow_intent`, and `human_gate` as returned. A lifecycle action belongs to
this spec state machine; it is not a universal project phase.

## 2. Draft or clarify

For `draft` or `clarify`:

1. Read `workflow_intent.context_for_agent`, expected fields, checks, and
   carrier.
2. Ground the draft in repository evidence: existing specs, source entry
   points, build/test configuration, decisions, and relevant docs.
3. Ask at most 1-3 questions only for values the repository cannot establish.
4. Edit the fenced `yaml spec-section` block and keep uncertainty explicit.
5. Run `haft spec check`, then call lifecycle again.

Do not maintain a second schema template in this skill.

### SoftwareSystemSpec scope

When `workflow_intent.document_kind=software-system`, describe the idealized
software that realizes the active `TargetSystemSpec` in Haft's local carrier
model. `TargetSystemSpec` is not asserted here as an FPF Core kind:

- assigned role and responsibility allocation;
- functional and procedural behavior;
- externally meaningful interfaces;
- software constraints and selected structure.

Do not put the team, coding agents, delivery workflow, release policy,
MethodPack rules, or evidence-production policy into SoftwareSystemSpec. Those
belong to the engineering/enabling system and its own carriers or
configuration. Also do not use SoftwareSystemSpec as an implementation plan,
source-tree tour, or runtime evidence report. It states the current software
contract; performed work and evidence remain separate artifacts.

## 3. Record an operator-requested change

Phrases such as `запиши в спеки`, `update the spec`, or `put this into the
software spec` authorize the relevant carrier edit. They do not authorize
approve, rebaseline, reopen, or a binding DecisionRecord.

If the carrier is ambiguous, use lifecycle and ask one narrow question. Do not
guess among target, software, and term-map concerns. These are Haft routing
labels, not a claim that FPF prescribes three corresponding system kinds.

## 4. Internal semantic fanout review

When a term, kind assignment, claim strength, or boundary changes across
carriers, run this routine inside `h-spec`:

1. Find every authored occurrence and generated mirror.
2. Classify each as definition, use, alias, historical citation, generated
   projection, or unrelated homonym.
3. Preserve the governed object and claim strength; do not perform blind text
   replacement.
4. Update only affected carriers and list deliberate non-changes.
5. Run spec checks and report remaining fanout.

When one boundary sentence mixes definition, admissibility, commitment, and
evidence, unpack those L/A/D/E claims internally before editing. These are
subroutines, not public skills.

### Source-currentness repair

When a SpecSection claim cites or relies on FPF meaning:

1. Recover the exact current direct pattern body and source identity through
   FPF Query before editing the claim.
2. Compare the claim with that source. A green structural check or semantic
   review of the existing claim register does not establish compatibility with
   a newer FPF source revision.
3. Classify each affected occurrence as current source meaning, Haft
   local-practice/API vocabulary, sealed legacy compatibility spelling,
   historical citation, or unrelated homonym.
4. Prepare an explicit before/after semantic change for every affected active
   section and name the implementation or wire surface that must change with
   it. Never repair source drift by blind token replacement.
5. Keep source compatibility, implementation evidence, and SpecSection
   baseline currentness as separate results.

In particular, an implementation symbol such as `MemberOf` or `EntitySet` may
remain a sealed legacy compatibility name while the current specification uses
the source-native classification objects. Never present a compatibility
spelling as current FPF meaning merely because old code or records still expose
it. Recover the current C.3 pattern body instead of maintaining a shadow C.3
inside this skill.

## 5. Human gates

- `approve`: show the section and obtain explicit operator approval before
  changing draft status or calling approve.
- `triage`: show findings and the admissible rebaseline, reopen, rollback,
  deprecate, or supersede choices. Call a mutation only after the operator
  selects one and supplies or accepts its reason.

Before requesting either gate, give a self-contained **Human Gate Brief**. Name
the lifecycle act and every affected section by readable title and ID, the
exact semantic fields or relations that would change, what remains unchanged,
and why only the affected operation is blocked. List every real option now and,
for each, the immediate consequence or return condition and weakest link.
Summarize an existing comparison/parity basis and non-dominated or Pareto set
when one exists; for a fixed apply/defer/reject lifecycle choice, explicitly say
that no Pareto front exists or applies. State the advisory recommendation,
freshness or expiry of the review/dry-run, and ask for the human engineer's
assessment of the options, trade-offs, and recommendation in natural language.
IDs, hashes, `human_gate`, and `requires_operator_act` are audit data, not
substitutes for this explanation. The brief itself is not approval.

Accept ordinary language as the substantive answer to the engineering
consultation, never as a lifecycle receipt. Never ask the engineer for a
command, skill invocation, exact reply phrase, or resumption token as a
substitute for explaining and choosing the lifecycle outcome. Only after the
engineer's position is explicit may a separately required lifecycle act be
explained, together with what it will and will not authorize.

Do not silently reclassify enabling-system policy as software behavior during
migration. Keep unresolved policy outside the SoftwareSystemSpec carrier and
surface the required human classification.
Never bind a DecisionRecord from this skill.

## 6. Relate the current edition to project memory

When an exact current SpecSection edition and exact EntityOfConcern are both
needed by a named receiving use, project their non-binding relation:

```text
mcp__haft__haft_spec_section(
  action="project",
  section_id="<exact section id>",
  entity_ref={
    "ref_kind_id":"U.EntityRef",
    "reference_id":"<exact current EntityOfConcern>"
  },
  bounded_context_ref="<exact current bounded context>"
)
```

This reloads the exact current SQL edition, seals its semantic hash, and may
admit `Haft.SpecSectionAtConcern`. It returns a
`Haft.SpecSectionRecordRef` when committed. The action does not edit the
carrier and cannot approve, rebaseline, reopen, or otherwise cross a lifecycle
gate. Never infer a concern identity from a section title.

## Result

Report lifecycle state, current local action, carrier and human-readable
section identity, edits, checks, any exact projected `record_reference`, and
any open human gate. Use `h-status` for read-only decision coverage of modules
or files.
