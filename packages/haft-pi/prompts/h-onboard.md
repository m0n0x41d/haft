Bootstrap Haft through the native Pi `haft_onboard` tool only when project
setup or its canonical basis is incomplete.

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
- `ready` — continue with the operator's current question.

`profile_prepare` may use repository detection. When the result needs an
explicit scope, pass only the readable scope fields exposed by the tool:
`scope_id`, `label`, `realization_kind`, and supporting `evidence_paths`, plus
the top-level readable `basis`. Preparation may materialize or reuse only a
non-binding review carrier. It does not apply canonical profile state.

Automatic `h-onboard` may inspect and prepare, but it must not apply. Only a
current explicit operator invocation of `h-onboard`, after the readable review
and engineering assessment, authorizes `haft onboard profile apply`. Do not ask
for a second confirmation after that valid explicit invocation.

`memory_prepare` may likewise materialize or reuse only a non-binding review
carrier. Present both real choices. Explicitly selected enablement is a
separate binding effect routed through manual `h-decide` and
`haft onboard memory enable`; it creates no substitute DecisionRecord. Only
after the operator actually selects **Not now**, run
`haft onboard memory defer`. A successful `memory_deferred` result is
non-binding, grants no authority, creates no DecisionRecord, and does not
pretend structured memory is enabled.

To reconsider `memory_deferred`, call `haft_onboard` with
`{ "action": "memory_prepare" }`; this safely reopens the review and does not
enable memory. If enablement returns `restart_required`, reconnect and repeat
`haft_onboard({ "action": "status" })`. Treat structured memory as ready only
when that fresh call returns `ready`; deferral remains readable as
`memory_deferred`.

Do not expose or ask the operator to choose internal memory schemas, revision
heads, staging records, or implementation letters. Missing setup or known
absence does not authorize EntityOfConcern creation and does not block
unrelated already-authorized Work.

After canonical applicability exists, inspect the spec lifecycle and draft
only applicable carriers. A non-software or unresolved scope must not receive
false SoftwareSystemSpec pressure. Keep every spec lifecycle gate human.

Do not create a ProblemCard merely because onboarding is happening and do not
infer a project sequence. Continuing lifecycle work belongs to `h-spec`.
