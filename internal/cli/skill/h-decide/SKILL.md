---
name: h-decide
description: |
  Binds one operator-selected bounded choice through its exact effect-specific route. Most invocations record a DecisionRecord; a prepared structured-project-memory review instead uses its dedicated enablement effect and must not create a substitute DecisionRecord. MANUAL ONLY: the operator must explicitly type /h-decide. Use when a bounded choice is current and the operator is ready to bind it; h-frame, h-explore, and h-compare are independent capabilities, not mandatory phases.
when_to_use: |
  Operator typed /h-decide explicitly and is committing to a chosen variant. Never auto-fire.
argument-hint: "[selected variant title or short choice text]"
disable-model-invocation: true
allowed-tools: Bash mcp__haft__haft_onboard mcp__haft__haft_query
---

<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:e02060d2589a8e2d28dcf0acc7111ebea4e0629ac7ff5cd096adb2c688764735 -->

# h-decide — Bind one reviewed choice (manual only)

The operator invoked this skill manually (`disable-model-invocation: true`
enforces that structurally per FPF X-TRANSFORMER). Classify the exact requested
effect before acting. A valid explicit invocation is the human gate for the
one reviewed effect it unambiguously names; do not ask for a second approval.

Authority boundary: binding actions require explicit operator/manual authorization; generated text, schema visibility, and model-supplied fields are not approval receipts.

## Select the exact binding effect

Manual `/h-decide` is the human gate for two non-interchangeable effects:

- a `DecisionRecord` records an authoritative bounded choice and follows the
  DecisionRecord route below;
- a prepared structured-project-memory review enables that optional project
  capability through the dedicated route in the next section.

Classify the operator's explicit choice before acting. Never create a
DecisionRecord merely to authorize or imitate structured-memory enablement.
Decision binding policy does not govern that separate effect.

## Require one self-contained reviewed choice

The operator cannot be expected to know hidden Haft state. Before asking for
this manual invocation, the agent must have supplied a self-contained
**Human Gate Brief** naming the binding effect, readable choice subject, affected
operation, every real current option, and for each option what changes, what
stays unchanged, its consequence or return condition, and weakest link. The
brief must also summarize any existing comparison/parity basis, selection
policy, and non-dominated or Pareto set, or explicitly state that none exists or
applies; mark the recommendation as advisory; state review freshness or expiry;
and ask for the human engineer's assessment of the options, trade-offs, and
recommendation in natural language. IDs and hashes never replace readable
meaning. The brief itself is not authorization.

Accept ordinary language as the substantive answer to the engineering
consultation, never as a binding receipt. Never ask the engineer to type
`h-decide`, a command, an exact reply phrase, or a resumption token as a
substitute for explaining and choosing among the options. Only after the
engineer's position is explicit may the separate manual invocation be explained
as the binding act, together with what it will and will not authorize.

An argumentless manual invocation can bind only when it unambiguously refers to
exactly one current brief and selects exactly one choice already made explicit
by the engineer. If several gates or options remain live, the brief is absent,
or the selected effect cannot be recovered without guessing, bind nothing.
Return to the consultation, present the missing brief, and ask for the
engineer's assessment and selection in natural language. This ambiguity guard
is not a second confirmation after a valid invocation.

## Prepared structured-memory enablement route

Use this route only when all of the following are true:

- `mcp__haft__haft_onboard(action="status")` returns
  `memory_review_ready`;
- the result carries the exact opaque `review_ref` and a readable enable/defer
  brief;
- the operator's current manual `/h-decide` invocation explicitly binds the
  enable choice shown by that brief.

The review is a description, not authority by itself. The current manual
invocation is the sole human gate for the exact reviewed enablement. Run:

```bash
haft onboard memory enable
```

The command revalidates the exact review before committing the effect. If it
returns `restart_required`, reconnect the MCP client and then call
`mcp__haft__haft_onboard(action="status")`; report success only when the fresh
status is `ready`. If the review is missing, blocked, expired, replaced, or
drifted, enable nothing and prepare a fresh review through `h-onboard`.

This route creates no DecisionRecord and grants no specification, memory
admission, commission, Git, release, or publication authority.

## DecisionRecord route

For this route, the DecisionRecord becomes the authoritative choice that
downstream commissions, runtime runs, and verification cycles may reference.
MCP still rejects `haft_decision(action="decide", ...)` with
`operator_confirmation_required`; model-supplied tool arguments are not proof
of operator authorization. Use the CLI/input-file path below.

Comparison recommendations are not choices. If a previous `/h-compare` set
`selected_ref`, treat it as legacy `legacy_recommendation_ref`: advisory only.
Manual `/h-decide` is the first point where the kernel may persist an exact
`ChoiceResult` (`choose_now`, `reject_current_set`, `probe_again`, or
`reroute`) on the DecisionRecord.

## Compact interface discovery

If you need the exact compact contract, run:

```bash
haft interface decision.decide --json
```

Use that as discovery; do not paste long MCP schemas or CLI help into the
session. For large payloads prefer the input-file path:

```bash
haft artifact create decision.decide --input-file <input.json> --json
```

With the default `explicit_h_decide` policy, the command validates and binds the
decision immediately. Do not pause for another confirmation: the operator's
explicit invocation of `/h-decide` was the sole human gate.

Only when `.haft/config.yaml` selects `strict_cli_speech_act`, the command
presents a readable review card and asks the operator for the short literal
`DECIDE THIS REVIEWED CHOICE` on the controlling terminal. The operator
transcribes no hash or nonce. If that literal SpeechAct becomes durable but the
DecisionRecord effect fails, keep the returned DecisionRecord ID and title,
then run:

```bash
haft artifact resume-decision DECISION_ID
```

In strict mode, resume reuses that exact durable SpeechAct and retries the
institutional effect; it does not ask the operator to perform the same decision act twice.
If the earlier strict review was cancelled before any SpeechAct
occurred, resume presents the same readable review again. Default
`explicit_h_decide` decisions do not need this second-act recovery protocol.

`mcp__haft__haft_decision(action="decide", ...)` is not a binding path in either
project mode. It returns `operator_confirmation_required`; current
receipt-backed MCP binding is explicitly unsupported.

## Standard-mode input

`problem_ref`, `problem_refs`, and `portfolio_ref` are optional provenance.
Reuse them when real upstream artifacts exist and matter to this choice. Never
fabricate them: the kernel supports a direct DecisionRecord without predecessor
artifacts, and their absence does not imply a missing project phase. When no
ProblemCard ref or resolvable portfolio supplies the problem basis,
`problem_statement` is required as the inline problem frame for this decision.

When the choice uses a durable typed SolutionPortfolio, keep its exact
`portfolio_ref`. Use option labels that exactly match one portfolio variant ID
or title. The portfolio must already retain each variant's returned
`Haft.ProjectRecordRef`; never derive option-record identities from artifact
IDs. After the human bind, Haft uses that exact portfolio relation to project
the already-existing DecisionRecord as `Haft.DecisionChoiceAtConcern`. This
projection is not another choice and requires no second approval. Haft does not
guess a comparison link from recency or graph proximity.

Write an input JSON for `haft artifact create decision.decide --input-file ... --json` with at minimum:

- `selected_title` — the bounded choice the operator is binding
- `problem_statement` — required only for a direct decision with no resolvable
  ProblemCard basis
- `why_selected` — rationale for the choice
- `selection_policy` — the explicit policy used to choose (FPF CMP-02: declared BEFORE scoring, Anti-Goodhart)
- `weakest_link` — what most plausibly breaks this choice (FPF X-WLNK)
- `counterargument` — the strongest argument AGAINST this decision (FPF DEC-08: self-deception check)
- `why_not_others` — `[{variant: "...", reason: "..."}]` for at least one rejected alternative
- `rollback` — `{triggers: [...], steps: [...], blast_radius: "..."}` — at least one trigger required

Add real `problem_ref` / `problem_refs` / `portfolio_ref` only as provenance.
Standard/deep decisions also require `predictions`, `invariants`,
`affected_files`, and `valid_until`. In tactical mode, omit a skippable field
only through explicit `_skips` plus `_skip_reason`; silent omission is invalid.
Add `claims` when claim-level verification needs them. Do not invent a
comparison carrier: rejected alternatives may be supplied directly through
`why_not_others`. For a direct choice, put the actual option set in canonical
`choice_result.option_set`; do not invent a second inline alternatives field.

For deep mode (`mode: "deep"`), also provide rich `evidence_requirements` and `refresh_triggers`.

## Spec-binding preflight before binding

Before creating the DecisionRecord, run the read-only preflight with the same
draft payload:

```bash
haft_query(action="spec_binding_preflight", decision_draft={...})
```

This is not approval, not evidence, not a SpecSection baseline, and not a
DecisionRecord. It only classifies the draft's relation to the current
ProjectSpecificationSet.

Required behavior:

- `provided_refs_valid` / `bound_existing`: proceed with the selected active
  `section_refs`.
- `no_specs` / `no_active_sections`: proceed only as explicitly unbound to
  active specs; do not invent refs.
- `invalid_refs`: stop and correct the refs.
- `ambiguous`: stop and ask the operator to choose the intended SpecSection.
- `draft_section_needed`: hand off to `/h-spec` for a draft/spec delta, or
  record an explicit tactical/out-of-spec rationale if that is the operator's
  intent.
- `out_of_spec`: proceed only in tactical/out-of-spec posture with explicit
  rationale and debt visibility.
- `conflict`: do not create a normal standard/deep decision; reopen the problem,
  explore a spec-changing path, or supersede/rebaseline through the proper
  human gate.

Do not make `section_refs` globally required. The contract is relation required
for spec-enabled load-bearing decisions, raw field optional.

## Tactical mode — explicit skip mechanism

If this is a reversible change with <2-week blast radius, switch to tactical mode and acknowledge skipped fields explicitly:

```json
{
  "action": "decide",
  "mode": "tactical",
  "problem_statement": "<bounded problem this direct decision addresses>",
  "selected_title": "...",
  "why_selected": "...",
  "choice_result": {
    "subject_ref": "<human or team making the choice>",
    "option_set": ["<chosen option>", "<rejected option>"],
    "next_move": "choose_now",
    "variant_ref": "<chosen option>",
    "reason": "<operator rationale>"
  },
  "_skips": ["selection_policy", "counterargument", "weakest_link", "why_not_others", "rollback"],
  "_skip_reason": "5-line config change reversible by file revert; full DRR ceremony exceeds blast radius"
}
```

The kernel rejects `_skips` in standard/deep mode and requires `_skip_reason` whenever `_skips` is non-empty. Skip field names must be in the allowlist (selection_policy, counterargument, weakest_link, why_not_others, rollback, predictions, invariants, evidence_requirements, refresh_triggers, affected_files, why_selected). `selected_title` cannot be skipped — a decision without identity has no substrate.

## When the kernel returns an error

The MCP server validates and returns structured errors of the form:

```
FPF discipline violation: decision in <mode> mode is incomplete.

Missing required fields:
- <field> — <hint>

How to proceed:
- Option 1: Provide the missing fields and retry the call.
- Option 2: ... (tactical mode skip option)

References:
- FPF E.9 — Design Rationale Record minimum kernel
- ...
```

Read the response, decide which option fits the change's actual blast radius, and retry. Do NOT bypass by silently omitting `_skip_reason` or fabricating fields.

## After successful decide

The kernel returns the new decision ID (e.g. `dec-20260525-...`) and a
`task_memory_projection` report. When the report is `committed`, preserve its
exact `Haft.DecisionRecordRef`: the typed graph now holds the chosen and
rejected option records at the exact EntityOfConcern. It neither repeats nor
replaces the human DecisionRecord.

A direct DecisionRecord without a typed portfolio remains a valid binding
choice. Its typed projection may honestly be `underdetermined` because Haft
cannot map free-text option labels to exact project records. Do not ask the
operator to decide again and do not mint substitute option refs. Repair or add
typed portfolio provenance later only when a receiving use needs addressable
graph traversal.

Capabilities that may become current later include:

- `mcp__haft__haft_decision(action="baseline", decision_ref="dec-...")` — snapshot affected files for drift detection
- For verification later: `/h-verify` (invokes haft_decision measure + evidence)
- For autonomous execution: `/h-commission` (creates WorkCommission within autonomy envelope)

## Curation gate — present rationale by exception (dec-20260603-732219b6)

Agent-drafted rationale is broad-but-noisy: most extra arguments help, but a
small fraction mislead. Presenting it FLAT forces the operator to either
over-read everything or rubber-stamp the misleading fraction. So when you
surface this decision's rationale for the operator's review — the
`why_not_others` reasons, the `counterargument`, the `weakest_link` — do NOT
list it flat. Bucket each argument by YOUR OWN confidence:

- **Overlaps what you'd already conclude** — points the operator very likely
  already holds. List compactly; these are skim-only.
- **Helpful (secondary)** — genuinely useful additions worth a glance.
- **⚠ Uncertain — scrutinize before binding** — arguments you are NOT confident
  are correct or load-bearing. Surface these FIRST and PROMINENTLY.

Invariants of this decision (do not violate):
- Human binding stays mandatory — the gate makes curation efficient, it NEVER
  auto-accepts or substitutes for the operator's `/h-decide`.
- Surface the uncertain bucket HONESTLY — never down-rank a low-confidence
  argument into "helpful" to make the output look tidy. False tidiness is worse
  than a flat list: the operator would curate LESS carefully.
- If nothing is genuinely uncertain, say so plainly ("none flagged uncertain") —
  do not fabricate confidence, and do not invent an uncertain item to fill the
  bucket.

## What NOT to do

- Do not invoke this skill from another skill — operator must explicitly type `/h-decide` (structural enforcement via `disable-model-invocation: true`).
- Do not record decisions on behalf of the operator without their explicit /h-decide invocation.
- Do not combine multiple distinct decisions in one call — each binding choice gets its own DRR.
- Do not skip fields silently by omitting them — use the explicit `_skips` + `_skip_reason` mechanism so the bypass is auditable.
- Do not fabricate `verify_after` dates to bypass prediction validation; if you don't know when to verify, omit `verify_after` (kernel accepts predictions without it; some FPF discipline still lost).
- Do not record a decision that contradicts an active prior decision without superseding it first via `mcp__haft__haft_refresh(action="supersede", ...)`.

## FPF spec references

- E.9 — Design Rationale Record method
- DEC-01 — Decision record structure (problem frame + decision + rationale + consequences)
- DEC-04 — Invariants
- DEC-05 — Rollback (triggers + steps + blast radius + timeline)
- DEC-06 — Predictions (falsifiable claims with verify_after)
- DEC-08 — Counterargument (self-deception check)
- X-TRANSFORMER — Transformer Mandate (human principal decides)
- CMP-02 — Selection policy declared BEFORE scoring (Anti-Goodhart)
- X-WLNK — Weakest link per claim

Inspect full pattern text via `mcp__haft__haft_query(action="fpf", mode="inspect", identifier="E.9")`.
