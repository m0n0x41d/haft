---
name: h-diagnose
description: |
  Diagnose a concrete failure with parallel rival-hypothesis testing. Use when symptoms are observable but cause is unclear. Stabilize the signal, generate distinct explanations, test them read-only in parallel, and rank by evidence while keeping losing rivals visible. This skill is independent and ordinarily non-persistent; save a diagnosis frame or hypothesis portfolio only on explicit request or when a named receiving use needs replay.
when_to_use: |
  A test, runtime, data, integration, or operational failure has more than one plausible cause. For a feature or redesign question use h-frame.
argument-hint: "[observed failure]"
allowed-tools: Bash Read Grep Glob Agent mcp__haft__haft_problem mcp__haft__haft_solution mcp__haft__haft_query
---

# h-diagnose — Test rival explanations

Abduction is an internal routine of this skill, not a separate public skill.
Retrieve the current FPF source before relying on its diagnostic semantics:
inspect a known SourceID or UnitID with
`mcp__haft__haft_query(action="fpf", mode="inspect", identifier="...")`, or
use `mode="concern"` with the concrete failure question and inspect the direct
pattern body. Query returns source candidates, not a selected cause or verdict.

## Conditional project-memory orientation

When this diagnosis is context-heavy, multi-session, or reliance-bearing and
the exact EntityOfConcern is not already current, resolve its identity with
`haft_query(action="memory", memory_request={"mode":"resolve",
"contract_version":"haft.memory.v1","basis":{"kind":"project_current"},
"query":"...","max_candidates":5})`. Select the exact candidate by the current
use rather than rank, then use the closed `memory_request` neighborhood branch
advertised by the tool schema with
`projection_profile_ref="agent_orientation.v2"`.

Inspect `result_kind` before relying on content. `project_basis_unavailable`,
known absence, or explicit abstention is visible but non-blocking: continue the
diagnosis without inventing a profile, entity, artifact, or human gate. This
read does not replace code-graph preflight before a later code edit. Never
persist typed memory merely because a read failed; persistence requires an
explicit operator save request or a named receiving use with provenance.

## Procedure

1. Stabilize the observed signal without naming a cause. State time window,
   scope, reproducibility, and what remains unaffected.
2. Inspect the failing code, logs, tests, and related Haft decisions when they
   are current. Treat stale governing assumptions as one possible hypothesis,
   not as proof.
3. Generate at least three rival hypotheses that differ in kind. For each,
   state a discriminating probe that could refute it.
4. Run read-only probes in parallel where possible. Give each investigator one
   hypothesis and the same signal; prohibit edits.
5. Rank by evidence for, evidence against, and remaining uncertainty. Keep
   refuted and inconclusive rivals visible when they matter to fallback or
   replay.
6. State the smallest supported conclusion. `Inconclusive` is valid; a
   plausible story is not a root-cause finding.

## Persistence boundary

Default to a conversational diagnosis. Do not automatically create a
ProblemCard or SolutionPortfolio. Persist only on explicit save intent or when
a named receiving use needs the diagnosis to remain addressable. In that case,
record the stable diagnosis problem and the rival hypotheses with their
evidence limits; do not present hypothesis ranking as a binding decision.

## Result

Return the leading explanation, concrete evidence, live rivals, and the next
discriminating probe if uncertainty remains. Diagnosis does not authorize a
fix. Implement only when the operator separately asks for the change.
