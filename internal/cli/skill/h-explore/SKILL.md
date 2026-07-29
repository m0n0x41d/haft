---
name: h-explore
description: |
  Generate 3-5 genuinely distinct candidate approaches for a current question, with the weakest link of each kept visible. Use when alternatives are needed or one favored approach is prematurely closing the search. This skill is independent: it may work from an inline question, cue, accepted problem, or other current basis. Default to conversational candidates; persist a SolutionPortfolio only on explicit save intent or when a named receiving use needs replay.
when_to_use: |
  The current need is option generation, not diagnosis of a failure or comparison of already available options.
argument-hint: "[current question, cue, or problem reference]"
allowed-tools: Bash Read Grep Glob Agent mcp__haft__haft_problem mcp__haft__haft_solution mcp__haft__haft_note mcp__haft__haft_query
---

# h-explore — Keep distinct possibilities live

Inspect an exact FPF SourceID or UnitID with `haft_query(action="fpf",
mode="inspect", identifier="...")`; otherwise use `mode="concern"` with the
current exploration question. Retrieval is recall; the full pattern body
supplies the conditions and result semantics.

## Conditional project-memory orientation

When this exploration is context-heavy, multi-session, or reliance-bearing and
the exact EntityOfConcern is not already current, resolve its identity with
`haft_query(action="memory", memory_request={"mode":"resolve",
"contract_version":"haft.memory.v1","basis":{"kind":"project_current"},
"query":"...","max_candidates":5})`. Select the exact candidate by the current
use rather than rank, then use the closed `memory_request` neighborhood branch
advertised by the tool schema with
`projection_profile_ref="agent_orientation.v2"`.

Inspect `result_kind` before relying on content. `project_basis_unavailable`,
known absence, or explicit abstention is visible but non-blocking: continue the
exploration without inventing a profile, entity, artifact, or human gate. This
read does not replace code-graph preflight before a later code edit. Never
persist typed memory merely because a read failed; persistence requires an
explicit operator save request or a named receiving use with provenance.

## Procedure

1. Recover the current object, question, constraints, and useful candidate
   kind. Use an existing ProblemCard when it is relevant, but do not create one
   merely to satisfy this skill.
2. Generate 3-5 variants that differ in kind, not degree. Use parallel agents
   when independent directions materially improve diversity.
3. For each variant state: title, mechanism or structural move, expected
   benefit, weakest link, evidence gap, and whether it opens useful future
   search space.
4. Keep unattractive but informative stepping stones when they reveal a new
   action, interface, data source, or method family. Do not keep duplicates for
   cosmetic diversity.
5. Preserve uncertainty. Candidate generation is not recommendation, choice,
   authorization, WorkPlan, or performed Work.

## Persistence boundary

In ordinary use, return candidates in conversation and stop. Persist only on
explicit save intent or a named reliance-bearing receiving use. A durable
typed portfolio requires each candidate to be an independently addressable
ProjectRecord. When that receiving use is current, persist one non-binding
candidate-description Note per variant through `haft_note`, using the same
exact `entity_ref` and `bounded_context_ref`, and retain each returned
`record_reference`. This is reliance-gated materialization, not an automatic
bundle and not a claim that a Note is an FPF solution kind.

If the kernel requires a ProblemCard for the legacy portfolio carrier, use an
existing card or ask whether the operator wants the current frame recorded;
never invent one silently.

When persistence is justified:

```text
mcp__haft__haft_solution(
  action="explore",
  problem_ref="<existing durable problem ref>",
  entity_ref={
    "ref_kind_id":"U.EntityRef",
    "reference_id":"<exact current EntityOfConcern>"
  },
  bounded_context_ref="<exact current bounded context>",
  variants=[{
    "title":"<name>",
    "description":"<distinct move>",
    "novelty_marker":"<difference in kind>",
    "weakest_link":"<failure boundary>",
    "stepping_stone":false,
    "risks":["<risk>"],
    "strengths":["<strength>"],
    "project_record_ref":{
      "ref_kind_id":"Haft.ProjectRecordRef",
      "reference_id":"<exact record_reference from the candidate Note>"
    }
  }]
)
```

Do not construct or guess a record ID from an artifact ID. If an exact option
record is unavailable, the legacy SolutionPortfolio may still be retained but
its typed projection must remain `underdetermined`; never mint a substitute
entity or hide the missing basis.

## Result

Present each option in plain language with its main catch. If persisted, pair
IDs with titles and report the portfolio's exact `record_reference` when
committed. Stop when the option field is adequate for the current use.
Comparison, decision, planning, or implementation occurs only if one of those
questions later becomes current; none is an automatic next step.
