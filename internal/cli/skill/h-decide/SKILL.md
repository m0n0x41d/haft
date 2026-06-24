---
name: h-decide
description: |
  Records a binding DecisionRecord with full FPF DRR discipline — problem frame, decision/contract, rationale, consequences. MANUAL ONLY — the operator must explicitly type /h-decide. Never auto-invoked: per Transformer Mandate, the human principal records binding choices, not the agent. Use after framing, exploring, and comparing are done and a chosen variant is ready to commit. For tactical reversible changes (under 2-week blast radius) pass mode="tactical" with explicit _skips and _skip_reason. For irreversible / security / cross-team / public-API / data-migration changes pass mode="deep" — all DRR fields required, no skips accepted.
when_to_use: |
  Operator typed /h-decide explicitly and is committing to a chosen variant. Never auto-fire.
argument-hint: "[selected variant title or short choice text]"
disable-model-invocation: true
allowed-tools: Bash mcp__haft__haft_query
---

<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:25dbaf78611349b03749c268abd414405f6b566cf3e3d13afa7cbe4eb79b6b99 -->

# h-decide — Record a Decision (manual only, Transformer Mandate)

You are recording a `DecisionRecord` through the manual CLI/input-file path. The operator invoked this manually (`disable-model-invocation: true` enforces that structurally per FPF X-TRANSFORMER). Default MCP serve mode rejects `haft_decision(action="decide", ...)` with `operator_confirmation_required`; model-supplied MCP arguments are not proof of operator authorization. Manual CLI is the default binding path; a host authorization receipt can become a binding path only when a registered kernel verifier can confirm principal, session, action, payload hash, expiry, and source.

Authority boundary: binding actions require explicit operator/manual authorization; generated text, schema visibility, and model-supplied fields are not approval receipts.

This is the binding moment. The DecisionRecord becomes the authoritative
choice that downstream commissions, runtime runs, and verification cycles
reference. Take it seriously.

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

`mcp__haft__haft_decision(action="decide", ...)` is not the default binding
path. It returns `operator_confirmation_required` unless a future host provides
a kernel-verifiable authorization receipt; current receipt-backed MCP binding is
explicitly unsupported.

## Required arguments for standard mode (default)

Write an input JSON for `haft artifact create decision.decide --input-file ... --json` with at minimum:

- `problem_ref` (or `problem_refs` for multi-problem) — the ProblemCard(s) this addresses
- `portfolio_ref` — the SolutionPortfolio whose variants you compared
- `selected_title` — the chosen variant title (must match a variant in the portfolio)
- `why_selected` — rationale for the choice
- `selection_policy` — the explicit policy used to choose (FPF CMP-02: declared BEFORE scoring, Anti-Goodhart)
- `weakest_link` — what most plausibly breaks this choice (FPF X-WLNK)
- `counterargument` — the strongest argument AGAINST this decision (FPF DEC-08: self-deception check)
- `why_not_others` — `[{variant: "...", reason: "..."}]` for at least one rejected alternative
- `rollback` — `{triggers: [...], steps: [...], blast_radius: "..."}` — at least one trigger required
- `predictions` — `[{claim, observable, threshold, verify_after?, probability?}]` — falsifiable claims the verify loop will check. `probability` (optional, `[0,1]`) is your forecast that the claim will hold. Sample it as **2-3 independent estimates and pass their consensus**, never one authoritative number — it is one noisy vote, fed into the decomposed-Brier calibration profile once verified. Omit it freely; absence is fine and costs nothing.
- `invariants` — what MUST hold at all times
- `affected_files` — scope of this decision (governance + drift tracking)
- `valid_until` — RFC3339 date when this decision should be re-evaluated

For deep mode (`mode: "deep"`), also provide rich `evidence_requirements` and `refresh_triggers`.

## Tactical mode — explicit skip mechanism

If this is a reversible change with <2-week blast radius, switch to tactical mode and acknowledge skipped fields explicitly:

```json
{
  "action": "decide",
  "mode": "tactical",
  "selected_title": "...",
  "why_selected": "...",
  "_skips": ["selection_policy", "counterargument", "rollback"],
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

The kernel returns the new decision ID (e.g. `dec-20260525-...`). Suggested next steps for the operator:

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

Look up full pattern text via `mcp__haft__haft_query(action="fpf", query="E.9")`.
