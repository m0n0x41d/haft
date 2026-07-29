---
name: h-onboard
description: |
  Bootstrap Haft through one readable onboarding surface, prepare project-profile or structured-memory reviews, and orient applicable specification carriers. Use for first-time setup or when haft_onboard reports that setup is incomplete. Status and repository detection are read-only; preparation may materialize or reuse only a non-binding review carrier. Profile application, structured-memory enablement, and specification lifecycle gates remain explicit human acts.
when_to_use: |
  The repository has no Haft state or its initial spec lifecycle is not ready.
argument-hint: "[optional project description]"
allowed-tools: Bash Read Grep Glob Write Edit mcp__haft__haft_onboard mcp__haft__haft_query mcp__haft__haft_spec_section
---

# h-onboard — Bootstrap Haft through one readable setup contract

Contract truth: project-profile onboarding and structured-memory setup are
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
- `needs_profile` — prepare a profile review;
- `profile_review_ready` — present the readable review and its exact next act;
- `needs_memory` — prepare the structured-memory review;
- `memory_review_ready` — present the enable/defer choice and route a selected
  enablement through manual `h-decide`;
- `memory_deferred` — setup is intentionally usable without structured memory;
- `ready` — continue with the current project question.

Do not expose or ask the operator to choose internal schema composites,
revision heads, staging records, or implementation letters. `status` is a
readable setup projection, not authority and not performed setup Work.

## 2. Prepare a project-profile review

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
review carrier; `canonical_profile_changed` remains false.

Automatic `h-onboard` may inspect and prepare, but it must not apply. Only an
explicit operator invocation of `h-onboard`, after the readable review and
engineering assessment are current, authorizes:

```bash
haft onboard profile apply
```

Do not ask for a second confirmation after that valid explicit invocation.
Report the readable scope and applicability result, not internal profile
machinery.

## 3. Prepare the structured-memory choice

When status returns `needs_memory`, call:

```text
mcp__haft__haft_onboard(action="memory_prepare")
```

This may materialize or reuse only a non-binding review carrier. It returns
`memory_review_prepared`, `memory_review_reused`, or `blocked`, plus a readable
brief and opaque `review_ref`. Present both real choices:

- **Enable structured memory** is a separate binding effect. Route an
  explicitly selected enablement through manual `h-decide`, which runs
  `haft onboard memory enable`. Do not substitute a DecisionRecord for this
  effect.
- **Not now** is a non-binding disposition. Only after the operator actually
  selects it, run `haft onboard memory defer`. A successful result is
  `memory_deferred`; it grants no authority, creates no DecisionRecord, and
  does not pretend structured memory is enabled.

To reconsider a current `memory_deferred` disposition, call
`mcp__haft__haft_onboard(action="memory_prepare")`. This safely reopens the
same review route; it does not enable memory by itself.

If enablement returns `restart_required`, reconnect the MCP client and repeat
`haft_onboard(action="status")`. Structured-memory readiness is established
only when that fresh status returns `ready`. Deferral instead remains readable
as `memory_deferred`.

Missing setup, known absence, or explicit abstention does not block unrelated
already-authorized Work. Never establish an EntityOfConcern or persist typed
memory merely because a read could not resolve it.

## 4. Continue to applicable specifications

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
