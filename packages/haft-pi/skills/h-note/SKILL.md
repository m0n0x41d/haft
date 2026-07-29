---
name: h-note
description: Persist an explicitly requested non-binding fact, observation, caveat, or small rationale in Haft project memory. Do not auto-persist ordinary reasoning.
---

# h-note — Save a lightweight fact

Use `haft_note` only when the operator asks to remember/save something or a
named receiving use needs an addressable fact. Keep observations atomic, state
why they matter, and anchor them to relevant artifacts and stable implementation
files. A note is not a choice, ProblemCard, evidence verdict, approval, or
WorkPlan.

When the exact current concern is known, include `entity_ref` with
`ref_kind_id="U.EntityRef"` plus `bounded_context_ref`. Preserve the returned
`Haft.ProjectRecordRef` exactly for later typed relations; never derive a
record ID from the note artifact ID. Without exact concern basis, the note may
persist while typed projection remains underdetermined.
