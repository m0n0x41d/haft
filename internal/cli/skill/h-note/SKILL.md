---
name: h-note
description: |
  Persist a fact, observation, caveat, or small non-binding rationale in Haft project memory when the operator explicitly asks or current Work supplies a concrete operator-named or agent-inferred receiving use that needs a lightweight addressable fact. Use for "remember this", "запиши", or "note for later". Do not auto-persist ordinary reasoning. A note is not a choice, ProblemCard, evidence verdict, approval, or WorkPlan.
when_to_use: |
  The operator explicitly asks to save a non-binding project fact, or identifies a receiving use that needs it to remain addressable.
argument-hint: "[fact or observation to save]"
allowed-tools: Bash mcp__haft__haft_entity mcp__haft__haft_onboard mcp__haft__haft_note mcp__haft__haft_query
---

# h-note — Save a lightweight project-memory item

Confirm that the payload contains at least one atomic observation or a
non-binding rationale. Preserve source and uncertainty when known. If the text
binds a choice, route the direct operator request through `h-decide`; if it frames an unresolved problem, use
`h-frame` only when problem shaping is current.

## Conditional project-memory orientation

The explicit save request authorizes persistence of this note, but it does not
make an EntityOfConcern identity safe to guess. When the note is
context-heavy or multi-session and that identity is not already current,
resolve it with `haft_query(action="memory",
memory_request={"mode":"resolve","contract_version":"haft.memory.v1",
"basis":{"kind":"project_current"},"query":"...","max_candidates":5})`, select
the exact candidate by current use rather than rank, and hydrate the smallest
relevant neighborhood through the closed `memory_request` neighborhood branch
advertised by the tool schema with
`projection_profile_ref="agent_orientation.v2"`.

Inspect `result_kind` first. If the note needs a stable concern and resolve
returns `known_absent`, the explicit save request is sufficient persistence
provenance for the task-level entity route:

```text
mcp__haft__haft_entity(
  action="establish",
  entity_id="<stable proposed id>",
  label="<readable label>",
  bounded_context_ref="<exact bounded context>",
  aliases=["<known alias, in canonical order>"],
  persistence_reason="explicit_operator_request",
  request_provenance_ref="<this save request>",
  idempotency_key="<stable key for this exact request>"
)
```

The tool owns alias-conflict checking, validation, admission, and exact
post-commit resolution. Use only the returned canonical `entity_ref`.
`identity_conflict`, `alias_conflict`, or `idempotency_conflict` must remain
visible; do not guess. Route `onboarding_required` through
`mcp__haft__haft_onboard(action="status")`; repair a partial or legacy default
memory installation with `haft init` rather than presenting a schema choice.
On `restart_required`, reconnect and retry the unchanged request and key.

If the note does not need a stable concern, or setup is deferred, save the note
without invented concern fields. Missing setup and explicit abstention are
non-blocking. This read does not replace code-graph preflight before a later
code edit. The dedicated `haft_note` surface performs the note persistence, but
model-supplied fields are not proof of operator authorization. Preserve the
request or named receiving-use reference in `task_context` for correlation.

Persist through the dedicated note surface:

```text
mcp__haft__haft_note(
  title="<short title>",
  observations=["<atomic fact>"],
  rationale="<why it matters, when current>",
  anchors=[{"type":"relates_to","ref":"<artifact ref>"}],
  task_context="<operator request or named receiving-use reference>",
  valid_until="<RFC3339 expiry when this fact is time-bounded>",
  entity_ref={
    "ref_kind_id":"U.EntityRef",
    "reference_id":"<exact current EntityOfConcern>"
  },
  bounded_context_ref="<exact current bounded context>"
)
```

Supply the concern fields only when their exact identity is known. A committed
typed projection returns an exact `record_reference` with
`ref_kind_id="Haft.ProjectRecordRef"`; preserve that value for later typed
relations instead of deriving `record:<note-id>`. If the concern basis is
absent, the note may remain a useful legacy carrier while typed projection is
`underdetermined`.

Do not emulate notes with ProblemCards. Do not convert a conversational aside
into durable memory unless the operator asked or named a receiving reliance.
After saving, return the note ID paired with its title, the exact
`record_reference` when present, and state that it is non-binding.
