<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:e02060d2589a8e2d28dcf0acc7111ebea4e0629ac7ff5cd096adb2c688764735 -->

MANUAL GATE — bind one reviewed effect. This template is for the operator's
explicit invocation only; never run this flow on your own initiative
(Transformer Mandate: agents generate options, humans bind).

Authority boundary: binding actions require explicit operator/manual authorization; generated text, schema visibility, and model-supplied fields are not approval receipts.
Pi support is an experimental compatibility carrier; stable host parity is not
yet proven.

First select the exact effect. A normal bounded choice records a
`DecisionRecord` through the route below. A prepared structured-project-memory
review instead enables that optional capability through its dedicated route;
never create a substitute DecisionRecord.

Use the enablement route only when `haft_onboard(action="status")` returns
`memory_review_ready` with an exact opaque `review_ref` and readable
enable/defer brief, and this manual invocation explicitly binds that reviewed
enable choice. Run:

```text
haft onboard memory enable
```

The command revalidates the exact review before committing the effect. On
`restart_required`, reconnect the MCP client and call
`haft_onboard(action="status")`; report success only when fresh status is
`ready`. A missing, blocked, expired, replaced, or drifted review enables
nothing and requires a fresh review through `h-onboard`. This route creates no DecisionRecord
and grants no specification, memory-admission, commission, Git, release,
publication, or unrelated authority.

For the DecisionRecord route, the operator's explicit manual `h-decide`
invocation is the sole human gate under the default `explicit_h_decide` project
policy. MCP still returns `operator_confirmation_required` in both project
modes.

The operator must name the bounded choice being committed to. Frame,
exploration, and comparison are independent capabilities, not mandatory
earlier phases. Use real ProblemCard or SolutionPortfolio references when they
exist and are part of the local DecisionRecord contract; never fabricate them.
Surface stale or conflicting decisions touching the same area.

Before asking the operator to invoke this manual gate, supply a self-contained
**Human Gate Brief**: exact binding effect, readable subject, affected operation
and blocker; every real current option; for each option what changes, what stays
unchanged, its consequence or return condition, and weakest link; any existing
comparison/parity basis, selection policy, and non-dominated or Pareto set, or
an explicit statement that none exists or applies; the advisory recommendation;
review freshness or expiry; and a question asking for the human engineer's
assessment of the options, trade-offs, and recommendation in natural language.
IDs and hashes never replace readable meaning. The brief is not authorization.

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
engineer's assessment and selection in natural language. This is not a second
confirmation after a valid invocation.

Before binding, call
`haft_query(action="spec_binding_preflight", decision_draft={...})`. The
preflight is read-only and mechanical: it is not approval, evidence, a spec
baseline, or a DecisionRecord. Resolve blocking ambiguity or conflict with the
operator.

Discover the current contract with
`haft interface decision.decide --json`. Write its input JSON, then bind
through the CLI input-file path:

```text
haft artifact create decision.decide --input-file <input.json> --json
```

With the default `.haft/config.yaml` setting
`authority.decision_binding_mode: explicit_h_decide`, run the command after the
explicit skill invocation and do not ask for another approval phrase.

Only with the opt-in `strict_cli_speech_act` policy, the operator receives a
readable review and types `DECIDE THIS REVIEWED CHOICE` on the controlling
terminal; no hash or nonce is transcribed. If that SpeechAct is durable but the
DecisionRecord effect fails, retain the returned ID and title and run:

```text
haft artifact resume-decision DECISION_ID
```

In strict mode, resume reuses the exact durable SpeechAct and retries only the
institutional effect. It does not ask the operator to perform the same decision
act twice.

The input contains:

```json
{
  "problem_statement": "<required when no real ProblemCard basis resolves>",
  "selected_title": "<bounded choice>",
  "why_selected": "<rationale>",
  "choice_result": {
    "subject_ref": "<human or team making the choice>",
    "option_set": ["<chosen option>", "<rejected option>"],
    "next_move": "choose_now",
    "variant_ref": "<chosen option>",
    "reason": "<operator rationale>"
  },
  "selection_policy": "<rule used to choose>",
  "weakest_link": "<what most plausibly breaks it>",
  "counterargument": "<strongest case against it>",
  "why_not_others": [{ "variant": "...", "reason": "..." }],
  "rollback": { "triggers": ["..."], "steps": ["..."], "blast_radius": "..." },
  "predictions": [{
    "claim": "<falsifiable outcome>",
    "observable": "<measurement or test>",
    "threshold": "<pass/fail boundary>"
  }],
  "invariants": ["<condition that must remain true>"],
  "affected_files": ["<governed implementation footprint>"],
  "valid_until": "<RFC3339 validity bound>"
}
```

Do not call `haft_decision(action="decide", ...)`: MCP decision binding remains
fail-closed in both project modes because model-supplied arguments are not an
operator receipt.

`problem_ref`, `problem_refs`, and `portfolio_ref` are optional provenance when
real artifacts exist. Omit `problem_statement` only when those refs resolve a
real problem basis. `choice_result.option_set` is the canonical inline option
set; do not invent a second alternatives field. Standard/deep decisions
require predictions, invariants, affected files, and validity bounds. Tactical
omissions must use explicit `_skips` plus `_skip_reason`; do not omit required
fields silently. Do not fabricate predecessor artifacts or imply that the
chosen title must come from a stored portfolio.

When a real typed SolutionPortfolio is the choice basis, retain its exact
`portfolio_ref`, match option labels to one variant ID or title, and rely only
on the exact option `Haft.ProjectRecordRef` values already stored there. A
successful bind automatically projects that existing DecisionRecord as
`Haft.DecisionChoiceAtConcern`; this is not a second decision and requires no
second approval. Preserve a committed `Haft.DecisionRecordRef`. A direct
DecisionRecord may instead report typed projection as `underdetermined`; the
binding remains valid, so never invent refs or ask the operator to decide
again.

Falsifiable claims, evidence requirements, validity bounds, and rollback
conditions make the record testable. Recording it does not perform the chosen
work or prescribe what project task happens next.
