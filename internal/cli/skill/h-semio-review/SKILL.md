---
name: h-semio-review
description: |
  INTERNAL SUBROUTINE — semiotic / fanout audit on a concept rename, deprecation sweep, or doc-vs-code consistency check. Walks all carriers (filenames, manifests, stale refs, review surfaces, dashboards, prompts) until fixed-point clean, because one-shot text replacement creates rework when carriers diverge. Manual invocation only. Do not auto-select for general work — for code review use Claude Code review; for FPF reasoning about decisions use h-frame / h-decide.
when_to_use: |
  Internal subroutine only. Manual invocation on a specific concept rename, deprecation cleanup, or spec consistency audit where carriers may have drifted.
argument-hint: "[concept being renamed or audit target]"
disable-model-invocation: true
allowed-tools: Bash Read Grep Glob mcp__haft__haft_query
---

# h-semio-review — Fanout audit (subroutine)

You are running the semiotic fanout audit per FPF X-FANOUT-AUDIT and the semiotics_slideument discipline. The premise: on concept rename / deprecation / spec consistency check, the change must sweep ALL carriers — prose, filenames, manifests, review bundles, provenance, tests, schemas — until fixed-point clean. Skipping carriers leaves zombie references that mislead future operators and agents.

TRIGGER on explicit/manual subroutine requests: "rename", "across the codebase", "fanout audit", "deprecation sweep", "spec consistency after the rename", "audit our spec for consistency after the rename".

Explicit-only — `disable-model-invocation: true`.

## Step 1 — Identify the object of talk

What CONCEPT or TERM is being renamed / deprecated / audited? Get it from the operator explicitly. Bad: "audit the docs"; good: "rename `Service` to `EnablingSystem` across docs and code".

If renaming, capture both old and new names. If deprecating, capture the old name and the replacement (or explicit "no replacement").

## Step 2 — Map carriers exhaustively

Run discovery across all carrier surfaces:

```bash
# Prose carriers
grep -rl "<term>" --include="*.md" --include="*.txt" --include="*.rst"

# Code carriers
grep -rl "<term>" --include="*.go" --include="*.py" --include="*.ts" --include="*.tsx" --include="*.rs" \
                  --include="*.java" --include="*.js" --include="*.jsx"

# Manifest carriers
grep -rl "<term>" --include="package.json" --include="Cargo.toml" --include="pyproject.toml" \
                  --include="go.mod" --include="*.yaml" --include="*.yml" --include="*.toml"

# Filenames
find . -name "*<term>*" -not -path "./node_modules/*" -not -path "./.git/*"

# Test fixtures
grep -rl "<term>" --include="*_test.*" --include="*.test.*" --include="testdata/*"

# Review bundles & specs
grep -rl "<term>" .haft/ docs/ .context/ 2>/dev/null
```

Build a carrier-by-carrier inventory. Every hit is a potential semio failure if the rename / deprecation doesn't sweep it.

## Step 3 — Classify each hit by statement type

For every hit, classify the statement type (per FPF X-STATEMENT-TYPE):
- Rule (definition / law)
- Promise (deontic commitment)
- Plan (intention)
- Explanation (descriptive)
- Decision (binding choice)
- Gate (admissibility condition)
- Evidence (observation)

This matters because rename behavior differs per type:
- Rule rename = update the definition itself
- Promise rename = potentially break compatibility — flag for operator
- Decision rename = check if referenced in active DRRs (`mcp__haft__haft_query(action="search", query="<term>")`)
- Evidence rename = update label only; observation content is immutable

## Step 4 — Same-name / different-thing check

Look for cases where the term being renamed already has a DIFFERENT meaning elsewhere in the codebase. Example: renaming `Service` to `EnablingSystem` — but `Service` already means something else in `auth/` (HTTP service vs FPF enabling system).

For each ambiguous hit, ask the operator whether this hit is in-scope for the rename.

## Step 5 — Different-name / same-thing check

The inverse: look for cases where the OLD concept appears under different aliases. Example: renaming `Service` to `EnablingSystem` — but the same concept might also be called `Creator`, `Builder`, `Producer` in different carriers. Surface aliases for operator decision.

## Step 6 — Produce fanout repair plan

Output as ordered repair list:

```
Fanout repair plan for rename "<old>" → "<new>":

Phase A — Definitional carriers (update first):
- docs/glossary.md (Rule)
- .haft/specs/term-map.md (Rule)
- internal/types/service.go:42 (Rule — Go type declaration)

Phase B — Reference carriers (update after definitions stable):
- internal/auth/handler.go:80 (uses term in implementation)
- internal/auth/handler_test.go:120 (test fixture)
- README.md:55 (prose explanation)

Phase C — Manifest carriers:
- internal/auth/_meta.yaml:3
- pyproject.toml:14

Out of scope (different concept, same name):
- internal/network/service.go (HTTP service abstraction, not FPF EnablingSystem)

Aliases to disambiguate (operator decision):
- "Creator" appears in 4 files — operator confirm: is this the same concept?
```

## Step 7 — Fixed-point check

After the operator (or downstream agent) applies the repair plan, re-run discovery. Repeat until grep returns zero unexpected hits. Per X-FANOUT-AUDIT: not iterating to fixed-point is the failure mode.

## Step 8 — Surface to operator

Present:
- Carrier inventory (with counts per carrier type)
- Statement-type classification
- Same-name / different-thing ambiguities
- Different-name / same-thing aliases
- Ordered fanout repair plan (Phase A definitional, then references, then manifests)
- Out-of-scope hits explicitly listed

The operator decides what to act on. This skill stops at audit + recommendation — it does NOT execute the renames.

## What NOT to do

- DO NOT bulk-rename without operator approval per ambiguous hit. Same-name / different-thing is the catastrophic semio failure.
- DO NOT skip the fixed-point re-check. Stopping after one pass leaves zombies.
- DO NOT auto-update spec carriers — those are operator-governed edits per h-spec discipline.
- DO NOT use this skill for general code search — it's specifically for semio / rename / fanout audit.
- DO NOT classify every hit identically — statement-type matters because rename semantics differ per type.

## FPF spec references

- X-FANOUT-AUDIT — on concept rename, sweep all carriers until fixed-point
- X-STATEMENT-TYPE — every load-bearing sentence = rule / promise / explanation / gate / evidence
- F.17 — Unified Term Sheet (UTS) — the canonical term-mapping carrier
- A.7 — Object / Description / Carrier — what changes vs what holds it
- A.6.9 — Cross-Context Sameness Disambiguation
- E.10 — Lexical discipline

Look up via `mcp__haft__haft_query(action="fpf", query="X-FANOUT-AUDIT")`. The semiotics_slideument in `.context/semiotics_slideument.md` carries the human-facing rationale.
