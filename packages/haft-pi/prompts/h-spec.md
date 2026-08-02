Manage the typed Haft specification lifecycle.

Call `haft_spec_section(action="lifecycle")` first. Treat `.haft/specs/*.md`
as carriers; the kernel projection and explicit human gates govern lifecycle
state. Ground drafts in repository evidence, ask only for irreducible missing
values, run `haft spec check`, and inspect lifecycle again.

`lifecycle` and `next_step` are project/scope-level
`ProjectSpecificationSet` workflow projections. Never pass `section_id` to
them or treat project workflow readiness as one named section's lifecycle.
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

`TargetSystemSpec` and `SoftwareSystemSpec` are Haft local-practice carrier
labels, not FPF Core kinds by label alone. A SoftwareSystemSpec describes the
idealized software contract, not the team, coding agents, delivery workflow,
release policy, MethodPack, evidence-production policy, implementation plan,
source-tree tour, or runtime evidence report. Do not silently reclassify such
enabling-system policy during migration; surface the human classification.

Use internal semantic fanout and L/A/D/E boundary repair when meanings cross
carriers. Never approve, rebaseline, reopen, or bind a decision without the
required explicit operator action.

When a claim relies on FPF meaning, recover the exact current pattern body and
source identity. A green structural or semantic carrier review does not prove
compatibility with a newer FPF revision. Separate current source meaning, Haft
API vocabulary, sealed legacy spellings, historical citations, and homonyms.
Do not present legacy `MemberOf` or `EntitySet` compatibility names as current
FPF C.3 semantics. Prepare explicit before/after changes and keep source
compatibility, implementation evidence, and baseline currentness separate.

Before requesting a spec lifecycle act, give a self-contained **Human Gate Brief**.
Name every affected section by readable title and ID, the exact fields
or relations that would change, what remains unchanged, and why only that
operation is blocked. List every real option and each option's consequence,
return condition, and weakest link. Summarize any existing comparison/parity
basis and non-dominated or Pareto set; for a fixed apply/defer/reject choice,
explicitly say that no Pareto front exists or applies. State the advisory
recommendation and review freshness or expiry, then ask for the human engineer's
assessment of the options, trade-offs, and recommendation in natural language.
Accept ordinary language as the substantive answer to the engineering
consultation, never as a lifecycle receipt. Never ask the engineer for a
command, skill invocation, exact reply phrase, or resumption token as a
substitute for explaining and choosing the lifecycle outcome. Only after the
engineer's position is explicit may a separately required lifecycle act be
explained with its authority limits. The brief is not approval.

When a concrete operator-named or agent-inferred receiving use needs the exact
current edition related to an exact
concern, call `haft_spec_section(action="project", section_id="...",
entity_ref=..., bounded_context_ref="...")`. Preserve a returned
`Haft.SpecSectionRecordRef`. This non-binding projection cannot edit the
carrier or cross any spec lifecycle gate.
