---
name: h-frame
description: |
  Shape an engineering problem without assuming a solution or forcing a project phase. Use when a redesign, refactor, or proposal arrives before the affected object, observed signal, constraints, unresolved relation, and acceptance basis are clear. Default to a conversational frame; record a ProblemCard only on explicit save intent or when a named receiving use needs a durable accepted problem statement. Prefer h-diagnose for a concrete failure with unclear cause.
when_to_use: |
  The problem itself is current and under-articulated. This skill may be used independently; it does not require or imply later exploration, comparison, or decision.
argument-hint: "[problem signal or proposed change]"
allowed-tools: Bash Read Grep Glob mcp__haft__haft_problem mcp__haft__haft_query
---

# h-frame — Shape the current problem

Use the FPF source for problem shaping rather than a hard-coded route. For an
exact SourceID or UnitID, call `mcp__haft__haft_query(action="fpf",
mode="inspect", identifier="...")`. Otherwise use `mode="concern"` with the
operator's query, then inspect the direct pattern body before relying on it.

## Conditional project-memory orientation

When this framing is context-heavy, multi-session, or reliance-bearing and the
exact EntityOfConcern is not already current, resolve its identity with
`haft_query(action="memory", memory_request={"mode":"resolve",
"contract_version":"haft.memory.v1","basis":{"kind":"project_current"},
"query":"...","max_candidates":5})`. Select the exact candidate by the current
use rather than rank, then use the closed `memory_request` neighborhood branch
advertised by the tool schema with
`projection_profile_ref="agent_orientation.v2"`.

Inspect `result_kind` before relying on content. `project_basis_unavailable`,
known absence, or explicit abstention is visible but non-blocking: continue the
frame without inventing a profile, entity, artifact, or human gate. This read
does not replace code-graph preflight before a later code edit. Never persist
typed memory merely because a read failed; persistence requires an explicit
operator save request or a named receiving use with request provenance.

## Procedure

1. **Stabilize the signal.** Separate what was observed from assumed cause and
   proposed solution. Name the affected object and bounded context.
2. **Restore overloaded words.** Replace vague terms such as `quality`,
   `scalable`, `ready`, or `process` with the exact characteristic, relation,
   or behavior that matters.
3. **Classify only if useful.** Diagnosis, optimization, search, or synthesis
   is a local aid, not a project phase. Hand a concrete unclear failure to
   `h-diagnose` when rival causes are the live question.
4. **State scope and constraints.** Say what is in and out. Separate hard
   constraints, optimization targets, and observations that must not become
   targets.
5. **Draft acceptance.** Propose an observable solved condition and label it as
   the agent's draft for operator correction. Do not delegate a blank field
   back to the operator when repository evidence supports a draft.
6. **Keep solution content out.** If a proposed method exposed the problem,
   retain it as context but do not make it part of the problem claim.

## When a ProblemCard is warranted

A `ProblemCard@Context` is appropriate only when the affected
EntityOfConcern, current constraints, unresolved relations, distinctions to
preserve, and acceptance basis are stable enough to survive reuse.

Default to an inline conversational frame. It may use the source-owned
`ProblemCard@Context` shape from `C.22.2`; that reasoning result is not yet a
materialized Haft artifact. Persist through
`mcp__haft__haft_problem(action="frame", ...)` only when:

- the operator explicitly asks to record or save the frame; or
- a named receiving use needs durable transfer, replay, audit, automation,
  delayed feedback, expensive feedback, or costly reversal.

Do not force an early cue, hypothesis, or ordinary local edit into a
ProblemCard. Do not auto-create a card merely because another Haft skill was
invoked.

## Persistent form

When persistence is justified, discover the compact contract with
`haft interface problem.frame --json`, then record the smallest honest payload:

```text
mcp__haft__haft_problem(
  action="frame",
  title="<short title>",
  signal="<observation, not assumed cause>",
  problem_type="diagnosis|optimization|search|synthesis",
  acceptance="<observable condition>",
  constraints=["<hard limit>"],
  optimization_targets=["<target>"],
  observation_indicators=["<watch, do not optimize>"],
  blast_radius="<affected scope>",
  reversibility="low|medium|high",
  seed_file="<if current>",
  mode="tactical|standard|deep",
  entity_ref={
    "ref_kind_id":"U.EntityRef",
    "reference_id":"<exact current EntityOfConcern>"
  },
  bounded_context_ref="<exact current bounded context>"
)
```

Supply `entity_ref` and `bounded_context_ref` only from an exact current
identity; never invent them from the title. With both present, the kernel may
admit a non-binding `Haft.ProblemCardAtConcern` relation and returns its exact
`record_reference`. Without that basis, the ProblemCard carrier may still be
saved while the typed projection honestly remains `underdetermined`; this is
not a failed card and must not be hidden.

Use `spec_fit_probe` only when a named spec relation is current. It is advisory,
not approval, evidence, baseline, or a prerequisite for every frame.

## Result

Return the frame and its uncertainties. If persisted, pair the ProblemCard ID
with its title or one-line signal. Report the returned `record_reference` when
committed, or the exact missing basis when the projection is underdetermined.
Stop there unless a different capability is now current; never prescribe
`h-explore` as the automatic next stage.
