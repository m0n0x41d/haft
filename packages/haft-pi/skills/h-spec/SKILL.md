---
name: h-spec
description: Manage Haft's typed spec lifecycle, source-currentness, carrier edits, and semantic fanout repair. Treat markdown as a carrier and kernel lifecycle plus explicit human gates as authority.
---

# h-spec — Typed spec lifecycle

Call `haft_spec_section(action="lifecycle")` first. Ground drafts in repository
evidence, edit only the current carrier, run spec checks, and inspect lifecycle
again. `TargetSystemSpec` and `SoftwareSystemSpec` are Haft local-practice
carrier labels, not FPF Core kinds by label alone.

`lifecycle` and `next_step` are project/scope-level
`ProjectSpecificationSet` workflow projections. Never pass `section_id` to
them or treat project workflow readiness as the state of one named section.
Use `haft_query(action="spec_trace", section_id="<id>")` for the exact edition,
status, and baseline, and `haft_query(action="spec_use", section_id="<id>",
use_context="<concrete receiving use>")` for stronger-use admission. The kernel
rejects an action-inapplicable `section_id` with these recovery routes.

If the MCP action is unavailable, use `haft spec next --json` or
`haft spec status --json`; never infer lifecycle from Markdown status fields.

When a spec request returns `profile_underdetermined`, preserve the exact
request and any supplied ScopeID. Treat `recovery_surface=haft_onboard` and
`next_action` as navigation only. Read `haft_onboard(action="status")`;
`needs_profile` permits at most a non-binding review, while
`profile_review_ready` permits showing that review. Apply only after the
operator directly and unambiguously selects the exact reviewed profile and
scope; route that request through `h-onboard` without requiring a skill name.
After apply and any required restart retry the same specification request.
Unrelated draft or clarification
work may continue only when it does not rely on profile applicability. Never
infer or auto-admit a profile, select `software` for convenience, or invent
lifecycle state.

`TargetSystemSpec` is `Required` for every declared realization scope even
when its profile has `entity_reference: none`. The optional entity relation is
for exact EntityOfConcern memory, traceability, and stronger identity-bearing
use; never prepare a profile change merely to continue TargetSystemSpec
lifecycle. A response that still reports
`missing_basis=admitted_target_system_relation` comes from a stale runtime or
skill projection: rebuild or reconnect the exact candidate and retry the same
read-only request without asking the operator for a relation choice. Changing
an existing relation remains a separate profile effect when that relation is
itself current.

Retrieve the profile-independent draft grammar with
`haft_spec_section(action="draft_contract")`, then validate every authored
draft and active carrier with `haft_query(action="spec_validate")`. If MCP is
unavailable, use `haft spec draft-contract --json` and
`haft spec validate --json`. This validation does not determine applicability,
activate or approve sections, create evidence, admit stronger use, mutate a
carrier, or establish FPF source currentness. Keep `haft spec check` for the
separate profile-applicable health question.

Keep team, agent, delivery, release, MethodPack, and evidence-production policy
outside SoftwareSystemSpec. Do not silently reclassify enabling-system policy
during migration. Run semantic fanout and L/A/D/E repair internally. Approve,
rebaseline, reopen, and binding decisions require explicit operator action.

For claims that rely on FPF meaning, recover the exact current pattern body and
source identity before editing. A green carrier check or semantic review of the
existing claim register is not proof of compatibility with a newer FPF source.
Classify every occurrence as current source meaning, Haft API vocabulary,
sealed legacy compatibility spelling, historical citation, or unrelated
homonym. `MemberOf` or `EntitySet` may remain compatibility symbols, but must
not be presented as current FPF meaning when the current C.3 source says
otherwise. Prepare explicit before/after changes; never use blind replacement.
Keep source compatibility, implementation evidence, and baseline currentness
separate.

Before requesting a spec lifecycle act, give a self-contained
**Human Gate Brief**. Name every affected section by readable title and ID, the exact fields
or relations that would change, what remains unchanged, and why only that
operation is blocked. List every real option and, for each, its consequence or
return condition and weakest link. Summarize an existing comparison/parity basis
and non-dominated or Pareto set when one exists; for a fixed
apply/defer/reject choice, explicitly say no Pareto front exists or applies.
State the advisory recommendation, review freshness or expiry, and ask for the
human engineer's assessment of the options, trade-offs, and recommendation in
natural language. IDs, hashes, `human_gate`, and `requires_operator_act` never
replace the explanation; the brief is not approval.

Accept ordinary language as the substantive answer to the engineering
consultation, never as a lifecycle receipt. Never ask the engineer for a
command, skill invocation, exact reply phrase, or resumption token as a
substitute for explaining and choosing the lifecycle outcome. Only after the
engineer's position is explicit may a separately required lifecycle act be
explained with its authority limits.

For a concrete operator-named or agent-inferred receiving use with an exact
current edition and concern, call
`haft_spec_section(action="project", section_id="...", entity_ref=...,
bounded_context_ref="...")`. A committed result returns an exact
`Haft.SpecSectionRecordRef`. This action only relates the immutable edition to
the concern; it cannot edit, approve, rebaseline, or reopen the section.
