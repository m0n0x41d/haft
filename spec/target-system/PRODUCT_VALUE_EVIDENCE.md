# Product Value Evidence

> Reading order: after EVIDENCE_ONTOLOGY and EXECUTION_CONTRACT.

## Purpose

This file records bounded product-value evidence for Haft itself. It is not a
scorecard, approval, gate decision, release claim, or market proof.

The current claim is narrow:

Haft can help a coding agent keep a long semantic-governance rewrite moving as
small, reviewable, green slices while preserving authority boundaries and making
missing value evidence visible.

This claim is supported only for the bearer/window below. It does not prove that
Haft is better than ADRs plus tests, that the overhead is acceptable in all
teams, or that host-backed authorization receipts are complete.

## Current Evidence Packet

| Field | Value |
|-------|-------|
| Bearer | `haft-semantic-spine-rewrite` |
| Window | `2026-06-23` |
| Method ref | `dogfood-current-goal` |
| Value surface | `haft value space haft-semantic-spine-rewrite --window 2026-06-23 --method-ref dogfood-current-goal ...` |
| Value surface output | `single_score=no_single_haft_or_fpf_score`, `evidence_refs=6`, `evidence_missing_characteristics=0` |

Evidence refs:

- `commit:d5c88368` — semantic drift targets recognize `section_id`.
- `commit:6ecd5cee` — measurement evidence preserves current F0-F9 formality metadata.
- `commit:8a6e6db2` — unversioned formality stays diagnostic instead of being promoted to current F0-F9.
- `commit:ce368364` — `haft value space` compact output shows evidence refs and missing evidence characteristics.
- `test:go-test-internal-scopeauth` — `go test ./internal/scopeauth -count=1` passed on 2026-06-23.
- `test:go-test-internal-cli-scope-violation` — targeted harness out-of-scope tests in `internal/cli` passed on 2026-06-23.

## What This Supports

- Atomic slice discipline is working for this rewrite window: each listed commit
  has focused tests and a full `go test ./...` run in the same session.
- The value surface does not collapse to a single scalar score.
- Scope violation handling is present at the code path tested by
  `scopeauth.AuthorizeWorkspaceDiff` and harness apply/result formatting.
- Missing value evidence is visible on the explicit value-space surface instead
  of being hidden in a design-time plan.

## What This Does Not Support Yet

- Equal-budget comparison against an AI agent using ADRs plus tests.
- Quantified ceremony cost for a new team or a short-lived task.
- Production-user outcome claims.
- Real host-backed authorization receipts.
- Claim that every drift event in this repository is already resolved.

Until those are measured, external-facing docs must describe these as
hypotheses or unmeasured gaps, not as proved product value.

## Simplify / Kill Criteria

The current `haft value space` output names simplify/kill criteria as
read-only review triggers. The most important current triggers are:

- scope violation is not blocked or surfaced;
- ceremony increases without movement in any declared value characteristic;
- false blocking exceeds operator tolerance;
- equal-budget comparison is missing while a value claim is made;
- evidence refs are missing for the declared window;
- one proxy metric is presented as the value truth.

These criteria do not auto-retire a feature. They force review before stronger
use.

## Next Evidence Needed

1. Run an equal-budget comparison against ADRs plus tests for the same task
   class and window.
2. Capture ceremony time and interruption cost for at least one short task and
   one long rewrite.
3. Record a host-backed receipt experiment only when a host can provide
   principal, session, action, payload hash, expiry, and verifier source.
4. Re-run this packet after R9 old-decision enrichment is operator-approved and
   applied, then compare drift-event fanout before and after.
