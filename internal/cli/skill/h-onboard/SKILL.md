---
name: h-onboard
description: |
  Bootstrap Haft through one readable onboarding surface, prepare a project-profile review when needed, and orient applicable specification carriers. Use for first-time setup or when haft_onboard reports that setup is incomplete. Project memory is ready immediately after haft init. Status and repository detection are read-only; haft init may admit only an obvious supported singleton as detector_default, while reviewed or ambiguous profile application and specification lifecycle gates remain explicit human acts.
when_to_use: |
  The repository has no Haft state or its initial spec lifecycle is not ready.
argument-hint: "[optional project description]"
allowed-tools: Bash Read Grep Glob Write Edit mcp__haft__haft_onboard mcp__haft__haft_query mcp__haft__haft_spec_section
---

# h-onboard — Bootstrap Haft through one readable setup contract

Contract truth: project-profile onboarding and automatic project-memory setup are
**V9 CONTRACT** capabilities. Source, schema, skill, and local-test presence is
not installed-runtime proof. A current readiness claim requires
**EXACT-CANDIDATE EVIDENCE** from P14 tied to one exact candidate; RC or release
status additionally requires release authority. Do not infer
**CURRENT PRODUCT** status from contract inclusion or evidence alone.

## 1. Read one onboarding status

If `.haft/` is absent, run the explicitly requested `haft init`; do not
hand-roll its state directories. Then call:

```text
mcp__haft__haft_onboard(action="status")
```

Interpret only its closed result kind:

- `needs_init` — initialize, reconnect when instructed, and repeat `status`;
- `needs_profile` — follow `next_action`; an eligible supported singleton routes
  through `haft init --core-only`, otherwise prepare a profile review;
- `profile_review_ready` — present the readable review and its exact next act;
- `ready` — continue with the current project question.

`haft init` installs default project memory as part of initialization. Never
ask the operator to enable, defer, select, or understand a memory schema. If a
legacy or partial installation reports `needs_init`, rerun `haft init`; do not
route initial memory setup through `h-decide`.

Do not expose or ask the operator to choose internal schema composites,
revision heads, staging records, or implementation letters. `status` is a
readable setup projection, not authority and not performed setup Work.

## 2. Prepare a project-profile review

`haft init` may admit `origin=detector_default` without a review only when the
detector observation is complete and non-truncated, confidence is supported,
exactly one scope is suggested, no canonical profile exists, and any existing
review is an unchanged Haft-generated carrier. It never changes an existing
profile. A human-authored, semantically enriched, or foreign review blocks this
automatic path and remains operator-mediated profile-review work.

When status reports `origin=detector_default` and
`profile_override_eligible=true`, `profile_prepare` may prepare a reviewed
replacement and a direct, unambiguous operator request may apply it. Successful
application appends a `host_routed_operator_request` admission. Profiles already
marked `host_routed_operator_request`, or carrying legacy `explicit_operator`
or `legacy_unknown` provenance, require the separate profile-change contract
and are not replaceable through this initial onboarding path.

When status returns `needs_profile`, call:

```text
mcp__haft__haft_onboard(action="profile_prepare")
```

Omitting `scopes` uses repository detection. If the basis is insufficient, the
tool returns `needs_scope_review` without writing canonical profile state. When
the operator supplies a scope, use only the readable shape:

```json
{
  "action": "profile_prepare",
  "basis": "<readable reason for these explicit scopes>",
  "scopes": [
    {
      "scope_id": "<stable readable id>",
      "label": "<what this repository scope is>",
      "realization_kind": "software",
      "evidence_paths": ["<path supporting the classification>"]
    }
  ]
}
```

`basis` is top-level on the `profile_prepare` request, alongside `scopes`; it is
required when repository detection cannot establish the scope. Evidence paths
may be empty for an empty repository when that readable basis is explicit.
`realization_kind` is `software` or `non_software`. A
`profile_review_prepared` or `profile_review_reused` result writes only the
non-binding review carrier; `canonical_profile_changed` remains false.

Automatic `h-onboard` may inspect and prepare, but it must not apply. After the
readable review and engineering assessment are current, route only a direct,
unambiguous operator choice of that exact profile and scope to:

```bash
haft onboard profile apply
```

Do not require a skill name or ask for a second confirmation after that valid
request. A bare `да` is usable only as the answer to one current unambiguous
profile brief.
Report the readable scope and applicability result, not internal profile
machinery.

Missing setup, known absence, or explicit abstention does not block unrelated
already-authorized Work. Never establish an EntityOfConcern or persist typed
memory merely because a read could not resolve it.

## 3. Continue to applicable specifications

Only after exact project applicability is readable, call
`mcp__haft__haft_spec_section(action="lifecycle")` for carriers applicable to
the selected concern. Do not draft a `SoftwareSystemSpec` for a non-software
scope or an unresolved profile. For draft or clarify, follow
`workflow_intent`, ground edits in repository evidence, run `haft spec check`,
and inspect lifecycle again. Approve, rebaseline, and reopen remain explicit
human lifecycle gates.

Read README, build/test configuration, source entry points, existing specs,
and relevant decisions before drafting. Ask only for facts that cannot be
recovered without invention.

Do not create an onboarding ProblemCard by default. The sequence above is the
local onboarding method, not the project's general reasoning order.

`TargetSystemSpec` and `SoftwareSystemSpec` are Haft local-practice carriers,
not FPF Core kinds by label. A project-profile declaration alone does not
establish the separate target-system relation required by TargetSystemSpec.
Preserve object/description/carrier, suggestion/declaration, and plan/Work
boundaries; use `h-spec` for detailed lifecycle rules.
