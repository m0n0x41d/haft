---
name: h-onboard
description: Bootstrap Haft through the readable task-level onboarding surface, prepare a non-binding project-profile review when needed, and orient only applicable typed spec carriers. Project memory is ready immediately after haft init; profile apply and lifecycle gates remain human.
---

# h-onboard — Bootstrap through one readable setup contract

Use the native Pi `haft_onboard` tool rather than exposing low-level profile or
memory machinery.

Contract truth: project-profile onboarding and automatic project-memory setup are
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
- `ready` — continue with the current project question.

`haft init` installs default project memory as part of initialization. Never
ask the operator to enable, defer, select, or understand a memory schema. A
legacy or partial installation reporting `needs_init` is repaired by rerunning
`haft init`, reconnecting, and repeating `status`.

If repository detection cannot establish the profile basis, call
`profile_prepare` again with the top-level readable `basis` and explicit
`scopes`. Each scope uses only `scope_id`, `label`, `realization_kind`, and
supporting `evidence_paths`. A prepared or reused review is non-binding and
does not change canonical profile state.

Detector proposals are read-only. Profile and spec gates remain human. A
readable scope_id is not a ScopeID or proof of canonical applicability.

Automatic `h-onboard` may inspect and prepare but must not apply. After the
readable review and engineering assessment, route only a direct, unambiguous
operator selection of that exact profile and scope to `haft onboard profile
apply`. Do not require a skill name or ask for a second confirmation. A bare
`yes` or `да` works only for one current unambiguous profile brief.
Successful application records `host_routed_operator_request` provenance;
automatic singleton bootstrap remains the separate `detector_default` path.

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
