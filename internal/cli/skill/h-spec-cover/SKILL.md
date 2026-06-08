---
name: h-spec-cover
description: |
  Surfaces uncovered files in modules that already have recorded decisions — highlights drift before it accumulates and suggests where new decisions are needed. Make sure to use this skill whenever the user asks "is X documented", "what's covered", "spec coverage", "drift detection", "what decisions apply here", "are we tracking this module", "what's undecided in X" — or whenever they are about to modify code in a module with existing DecisionRecords and should be reminded which decisions apply. Also use when /h-status flags large counts of undecided files in a module. NOT for spec lifecycle readiness, onboarding progress, or updating `.haft/specs/*` carriers (use h-spec). NOT for looking up decisions affecting one specific file (use mcp__haft__haft_query action=related). NOT for verifying one decision's predictions (use h-verify).
when_to_use: |
  Working in a module with existing decisions, OR explicit code/decision coverage question. For spec lifecycle/update questions use h-spec. For single-file lookup use haft_query action=related.
argument-hint: "[optional: module path]"
allowed-tools: Bash Read Grep Glob mcp__haft__haft_query mcp__haft__haft_problem mcp__haft__haft_spec_section
---

# h-spec-cover — Spec coverage analysis

You are running coverage analysis via `mcp__haft__haft_query(action="coverage")`. Identifies modules with no decisions (blind) and modules with stale decisions (degraded). Output drives the operator's triage — frame a problem for blind modules, refresh for stale ones.

TRIGGER on: "spec coverage", "what's spec coverage on internal/billing", "decision coverage", "covered by decisions", "what decisions apply", "documentation coverage gaps", "show documentation coverage gaps", "coverage on internal", "undecided files", "blind module".

This is **code/decision coverage**, not SpecSection lifecycle. The carrier
question "which file/module lacks decision rationale?" is different from the
lifecycle question "which SpecSection needs draft/approve/rebaseline?"

## Step 0 — Route lifecycle questions away

If the operator asks about any of these, use h-spec instead:

- onboarding progress / readiness
- `haft spec status` or `haft spec next`
- stale/drifted SpecSections
- updating `.haft/specs/*`
- "запиши это в спеки"
- approve / rebaseline / reopen for a spec section

For a quick read-only lifecycle check, call:

```
mcp__haft__haft_spec_section(action="lifecycle")
```

Then recommend `/h-spec` for edits or human gates.

## Step 1 — Call coverage analysis

```
mcp__haft__haft_query(
  action="coverage",
  context="<optional context filter>"
)
```

Returns per-module status:
- **governed** — module has at least one active decision
- **stale** — module has decisions but they're past valid_until / R_eff degraded
- **blind** — module has no decisions

Within each tier modules are **sorted by impact-score descending** (per dec-20260527-e4b86938 V1): impact = number of governed modules that depend on this one. A blind module with high impact is high-priority (governed code at risk if it changes); a blind module with zero impact is an isolated utility, low priority. The `(impact: N)` tag in the output line surfaces the count when > 0; absence of tag = zero impact.

## Step 2 — Identify the immediate gap

If the operator is currently editing a specific file, prioritize that file's module. Run:

```
mcp__haft__haft_query(
  action="related",
  file="<path operator is editing>"
)
```

Returns decisions whose `affected_files` include that path. If empty → the file is in a blind module (or governed only at parent module level — check governance_mode).

## Step 3 — Triage and recommend

For each significant gap, recommend the right next action:

- **Blind module + active operator work**: Recommend `/h-frame` to record a ProblemCard for the work, then `/h-decide` after exploration. Without this, the work commission later will see "no decisions" and block.
- **Stale module + recent operator work**: Recommend `/h-verify` to baseline+measure the existing decisions before further edits accumulate undeclared drift.
- **Blind module + no active work**: Just surface the gap; don't pressure the operator to frame for hypothetical work.

## Step 4 — Optional: framing assistant for a blind module

If the operator wants to address a blind module gap right now:

```
mcp__haft__haft_problem(
  action="frame",
  problem_type="<diagnose|optimization|synthesis depending on context>",
  title="Spec coverage gap: <module name>",
  signal="Module <X> is governed=blind — work happening here lacks decision-level rationale",
  acceptance="<what would constitute coverage: at least one active decision referencing the module's load-bearing files>",
  mode="tactical"
)
```

Note: this only frames the problem. The decision itself is manual-only via `/h-decide`.

## Step 5 — Present to operator

Surface:
- Coverage breakdown (governed / stale / blind counts)
- Module-level table sorted by relevance (operator's current file/branch first)
- For the operator's active module: any related decisions found
- Recommended next actions per gap

## What NOT to do

- DO NOT auto-frame ProblemCards for every blind module — that produces noise. Frame only when operator confirms active work in that area.
- DO NOT auto-baseline drifted modules — baseline is a binding act tied to a decision; let /h-verify do it explicitly.
- DO NOT recommend `/h-commission` from this skill — coverage gap is upstream of commission; the operator first frames + decides + then commissions if appropriate.
- DO NOT use coverage output as proof that spec lifecycle is ready. Use h-spec / `haft_spec_section(action="lifecycle")`.
- DO NOT silently filter blind modules to look better — the operator needs the raw signal.
- DO NOT modify any code or specs from this skill; it's analysis + recommendation only.

## FPF spec references

- A.10 — Evidence Graph Referring (decision-to-file mapping is the evidence graph's spatial dimension)
- A.7 — Object/Description/Carrier (file = carrier; decision = description; module behavior = object)
- F.17 — Unified Term Sheet (cross-module term reconciliation, related but distinct from file coverage)
- VER-02 — Evidence Decay (drives stale classification)

Look up via `mcp__haft__haft_query(action="fpf", query="A.10")`.
