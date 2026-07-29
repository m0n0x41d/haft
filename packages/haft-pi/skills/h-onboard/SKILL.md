---
name: h-onboard
description: Bootstrap Haft through the readable task-level onboarding surface, prepare non-binding project-profile or structured-memory reviews, and orient only applicable typed spec carriers. Apply and lifecycle gates remain human.
---

# h-onboard — Bootstrap through one readable setup contract

Use the native Pi `haft_onboard` tool rather than exposing low-level profile or
memory machinery.

Contract truth: project-profile onboarding and structured-memory setup are
**V9 CONTRACT** capabilities. Source, schema, skill, or local-test presence is
not installed-runtime proof and does not establish Pi host parity. A readiness
claim requires current **EXACT-CANDIDATE EVIDENCE** from P14 tied to one exact
candidate; RC or release status additionally requires release authority. Pi
support is experimental, and contract inclusion or evidence alone does not
establish **CURRENT PRODUCT** status.

Start with:

```json
{
  "action": "status"
}
```

Follow only its closed result kind:

- `needs_init` — run the explicitly requested `haft init`, reconnect when
  instructed, and repeat `status`;
- `needs_profile` — call `haft_onboard` with
  `{ "action": "profile_prepare" }`;
- `profile_review_ready` — present the readable review and its exact next act;
- `needs_memory` — call `haft_onboard` with
  `{ "action": "memory_prepare" }`;
- `memory_review_ready` — present the enable/defer choice and route a selected
  enablement through manual `h-decide`;
- `memory_deferred` — setup is intentionally usable without structured memory;
- `ready` — continue with the current project question.

If repository detection cannot establish the profile basis, call
`profile_prepare` again with the top-level readable `basis` and explicit
`scopes`. Each scope uses only `scope_id`, `label`, `realization_kind`, and
supporting `evidence_paths`. A prepared or reused review is non-binding and
does not change canonical profile state.

Detector proposals are read-only. Profile and spec gates remain human. A
readable scope_id is not a ScopeID or proof of canonical applicability.

Automatic `h-onboard` may inspect and prepare but must not apply. Only a current
explicit operator invocation of `h-onboard`, after the readable review and
engineering assessment, authorizes `haft onboard profile apply`. Do not ask for
a second confirmation after that valid explicit invocation.

`memory_prepare` may likewise create or reuse only a non-binding review.
Present both real choices. Explicitly selected enablement is a separate
binding effect routed through manual `h-decide` and
`haft onboard memory enable`; it creates no substitute DecisionRecord. Only
after the operator actually selects **Not now**, run
`haft onboard memory defer`. A successful `memory_deferred` result is
non-binding, grants no authority, creates no DecisionRecord, and does not
pretend structured memory is enabled.

To reconsider `memory_deferred`, call `haft_onboard` with
`{ "action": "memory_prepare" }`; this safely reopens the review and does not
enable memory. On enablement `restart_required`, reconnect and retry
`haft_onboard({ "action": "status" })`. Rely on structured-memory readiness
only when that fresh status returns `ready`; deferral remains readable as
`memory_deferred`.

Never expose or ask the operator to choose internal memory schemas, revision
heads, staging records, or implementation letters. Missing setup or known
absence does not authorize EntityOfConcern creation and does not block
unrelated already-authorized Work.

After exact project applicability is readable, inspect the spec lifecycle and
draft only applicable carriers. A non-software or unresolved scope must not
receive false SoftwareSystemSpec pressure. Keep every spec lifecycle gate
human.

Do not create a ProblemCard merely because onboarding is occurring and do not
infer a project sequence. Continuing lifecycle work belongs to `h-spec`.
