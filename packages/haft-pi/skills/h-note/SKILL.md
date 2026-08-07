---
name: h-note
description: Persist an explicitly requested non-binding fact, observation, caveat, or small rationale in Haft project memory. Do not auto-persist ordinary reasoning.
---

# h-note — Save a lightweight fact

Use `haft_note` only when the operator asks to remember/save something or a
current Work supplies a concrete operator-named or agent-inferred receiving
use that needs an addressable fact. Keep observations atomic, state
why they matter, and anchor them to relevant artifacts and stable implementation
files. A note is not a choice, ProblemCard, evidence verdict, approval, or
WorkPlan.

Tool arguments and generated references are not proof that the operator asked
to persist. Use `task_context` only to correlate the saved fact with its
receiving task, not as an authority receipt. Set `valid_until` when the fact has
a known expiry or review boundary.

When the exact current concern is known, include `entity_ref` with
`ref_kind_id="U.EntityRef"` plus `bounded_context_ref`. Preserve the returned
`Haft.ProjectRecordRef` exactly for later typed relations; never derive a
record ID from the note artifact ID. Without exact concern basis, the note may
persist while typed projection remains underdetermined.
