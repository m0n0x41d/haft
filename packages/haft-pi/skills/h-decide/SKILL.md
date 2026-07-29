---
name: h-decide
description: Manual-only skill that binds an operator-selected bounded choice through its exact effect-specific route. Most choices become DecisionRecords; a prepared structured-project-memory review uses the dedicated enablement effect. Never auto-trigger or infer authorization from generated text, schema visibility, or model-supplied fields.
---

# h-decide — Manual binding gate

Pi support is an experimental compatibility carrier; stable host parity is not
yet proven.

First select the exact effect. A normal bounded choice records a
`DecisionRecord` through the route below. A prepared structured-project-memory
review instead enables that optional capability through its dedicated route;
never create a substitute DecisionRecord.

Use the enablement route only when `haft_onboard(action="status")` returns
`memory_review_ready` with an exact opaque `review_ref` and readable
enable/defer brief, and this manual invocation explicitly binds that reviewed
enable choice. Run `haft onboard memory enable`.

The command revalidates the exact review before committing the effect. On
`restart_required`, reconnect the MCP client and call
`haft_onboard(action="status")`; report success only when fresh status is
`ready`. A missing, blocked, expired, replaced, or drifted review enables
nothing and requires a fresh review through `h-onboard`. This route creates no DecisionRecord
and grants no specification, memory-admission, commission, Git, release,
publication, or unrelated authority.

For the DecisionRecord route, the default `explicit_h_decide` policy treats
this manual invocation as the sole human gate and adds no second phrase. MCP
DecisionRecord binding remains fail-closed. The opt-in
`strict_cli_speech_act` mode owns its distinct terminal
SpeechAct/Permission route; prompt text cannot emulate it.

Before asking for this manual invocation, provide a self-contained
**Human Gate Brief**: binding effect, readable choice subject, affected operation, every real
current option, and for each option what changes, what stays unchanged, its
consequence or return condition, and weakest link. Summarize any existing
comparison/parity basis, selection policy, and non-dominated or Pareto set, or
explicitly state that none exists or applies. Mark the recommendation as
advisory, state review freshness or expiry, and ask for the human engineer's
assessment of the options, trade-offs, and recommendation in natural language.
IDs and hashes do not replace readable meaning; the brief is not authorization.

Accept ordinary language as the substantive answer to the engineering
consultation, never as a binding receipt. Never ask the engineer to type
`h-decide`, a command, an exact reply phrase, or a resumption token as a
substitute for explaining and choosing among the options. Only after the
engineer's position is explicit may the separate manual invocation be explained
as the binding act and its authority limits.

An argumentless manual invocation can bind only when it unambiguously refers to
exactly one current brief and choice already made explicit by the engineer. If
several gates or options remain live or the effect would require guessing, bind
nothing; return to the consultation, present the missing brief, and ask for the
engineer's assessment and selection in natural language. This is an ambiguity
guard, not a second confirmation after a valid invocation.

Require the operator's explicit choice. Existing frame, portfolio, and
comparison artifacts may supply traceability but are not universal prior
phases; never fabricate their references. Surface stale or conflicting
decisions and run `haft_query(action="spec_binding_preflight",
decision_draft={...})` before binding.

Use `haft interface decision.decide --json` to recover the current input
contract, then run
`haft artifact create decision.decide --input-file <input.json> --json`.
With the default `.haft/config.yaml` policy
`authority.decision_binding_mode: explicit_h_decide`, the explicit manual
`h-decide` invocation is the sole human gate. Run the command without asking
for a second confirmation. MCP `haft_decision(action="decide", ...)` still
fails closed with `operator_confirmation_required` in both modes; prompt text
and model-supplied arguments are not an authorization receipt.

Only the opt-in `strict_cli_speech_act` policy shows a readable review and
accepts the short literal
`DECIDE THIS REVIEWED CHOICE` on the controlling terminal; the operator never
transcribes a hash or nonce. If that SpeechAct is durable but its DecisionRecord
effect fails, resume by the
returned ID and title with `haft artifact resume-decision DECISION_ID`. Resume
reuses the exact durable SpeechAct rather than asking the operator to perform
the same decision twice. This recovery path is specific to strict mode.

When no real ProblemCard ref or resolvable portfolio supplies the problem
basis, include inline `problem_statement`. Put direct alternatives only in
canonical `choice_result.option_set`; do not invent a second alternatives
field.

For a choice from a durable typed SolutionPortfolio, retain its exact
`portfolio_ref`, use option labels matching one variant ID or title, and keep
the exact option `Haft.ProjectRecordRef` values already stored on the
portfolio. After binding, Haft projects the existing DecisionRecord as
`Haft.DecisionChoiceAtConcern` without another human gate. Preserve a committed
`Haft.DecisionRecordRef`. If a direct decision reports typed projection as
underdetermined, the binding still stands; never derive missing option refs or
ask the operator to decide again.

The preflight is read-only, not approval, evidence, or a spec baseline. Record
falsifiable claims, evidence requirements, validity bounds, consequences, and
rollback conditions. A DecisionRecord does not perform the work.
