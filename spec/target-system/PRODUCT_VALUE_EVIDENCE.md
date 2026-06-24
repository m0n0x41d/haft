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

## Evidence Packet — 2026-06-24

| Field | Value |
|-------|-------|
| Bearer | `current-haft-rewrite` |
| Window | `2026-06-24` |
| Method ref | `method:slice-train-dogfood` |
| Value surface | `haft value space current-haft-rewrite --window 2026-06-24 --method-ref method:slice-train-dogfood ...` |
| Value surface output | `score_policy.single_score=no_single_haft_or_fpf_score`, `characteristics=11`, `evidence_refs_per_characteristic=10` |

Evidence refs:

- `commit:1bc3cd9b` — `haft spec apply-change --dry-run` previews recognized
  typed Markdown-to-SQL sync-back without mutating the SQL edition store.
- `commit:256abbd6` — `haft interface spec.apply_change` exposes the
  classify / dry-run / apply sync-back contract and its authority boundary.
- `commit:755ddd8b` — default status, default `code_context`, MCP
  `tools/list`, and compact contract-generation text are guarded against
  inlining the long `spec.apply_change` dry-run contract shape.
- `commit:39237961` — the `query.contract_generation` interface discovery
  example is regression-tested against the live generated report counts.
- `blocked-use:scope-violation-attempt-2026-06-24` — deliberate attempt to
  apply R9 reconciliation/scope-enrichment without a fresh exact
  `operator_approved_reconciliation_selection` surfaced as a read-only
  blocked-use attention item with `source_return.status=exact_record_needed`,
  next actions `recover_exact_source_record` and
  `request_operator_selection_review`, and authority boundary
  `not_work_plan`, `not_evidence`, `not_approval`, `not_gate_decision`,
  `not_global_truth`.
- `commit:f45075b8` — stale reconciliation scope selections fail closed before
  mutation.
- `commit:d7434f9d` — stale reconciliation group diagnostics name the current
  matching group and apply operation.
- `commit:d90fc25f` — default `haft_query(action="status")` is guarded against
  contract-generation manifest bloat.
- `commit:45c4466b` — compact `haft interface contract-generation` output stays
  count-only and does not inline generated fragment/schema detail.
- `commit:31efcfcc` — baseline term audit classifies autonomous rebaseline
  wording, leaving `legacy_ambiguous_baseline=0` in the live audit.
- `commit:ffd07585` — reconciliation review packets stay report-only; review
  packets are rejected as apply authority and exported templates remain
  non-apply-ready until operator approval plus placeholder replacement.
- `commit:f88b870e` — compact overseer/status output no longer advertises the
  mutating `haft overseer maintain --json` path as an inspection drill-down.
- `commit:d999cda6` — root `CLAUDE.md` is scanned as a current host discipline
  mirror, so prompt/schema/model authority-boundary wording is covered by the
  carrier guard set.
- `maintenance:omnt-da2c52971296e024` — live autonomous maintenance applied one
  additive-only auto-rebaseline for `dec-20260526-f3223c16`; undo remains
  `haft overseer undo omnt-da2c52971296e024 act-001`.
- `test:go-test-all-2026-06-24-post-rebuild` — `go test ./...` passed after
  the operator rebuilt/restarted the installed Haft binary.

Current production-code trace:

- Spec sync-back advanced as three small green slices: dry-run preview,
  discoverable interface contract, and default-surface no-bloat guards.
- The generated contract discovery shape was corrected after dogfood found
  stale illustrative counts; the new regression compares example counts to the
  live report.
- The deliberate scope/authority violation was not executed. It was surfaced
  as an object-first blocked-use attention item requiring exact source return
  to a fresh approved selection before stronger use.
- The slice train remained atomic: stale reconciliation guards, status/contract
  bloat guards, and baseline-audit classification were committed separately
  after focused tests and `go test ./...`.
- A deliberate stale-selection apply attempt against
  `.context/r9-v1-batch-a2-approved-selection-2026-06-23.json` exited with code
  `1` before mutation because the old `reviewed_group_id` was no longer present
  in the current reconciliation plan.
- The current matching reconciliation group was reported as
  `decision-reconcile-9e5c28b9313a` with `apply_operation="none"`, so recovery
  path was visible without accepting the stale packet.
- The post-rebuild installed CLI status points drift/stale/suppression review
  at bounded read-only drill-downs and no longer mentions
  `haft overseer maintain --json` as an inspection path.
- `haft decision reconcile metrics --json` still reports material authority
  noise: `235` unique drift events, `34` impacted decisions, `160` material
  events, `75` audit-only events, `36` needs-binding-resolution events, and max
  fanout `28`.

What this supports:

- For this slice train, Haft improved fail-closed behavior and recovery
  diagnostics around stale reconciliation packets.
- The blocked-use surface can make a deliberate scope/authority violation
  visible with exact source-return requirements before an agent proceeds to
  stronger use.
- Default status and compact contract-generation surfaces stayed bounded while
  exact/audit detail remained available behind explicit commands.
- The carrier guard set caught the real host prompt mirror as current surface
  area without expanding the default status cockpit.
- The value-space dashboard exposed review triggers without producing a single
  scalar product-value score.

What this does not support:

- A general claim that Haft improves all AI software work.
- A claim that Haft beats ADRs plus tests under equal budget.
- A claim that every scope violation is blocked.
- A claim that blocked-use attention is a runtime OperationalGate or
  GateDecision.
- A claim that current authority-frontier noise is solved.
- A claim that MCP-hosted status text cannot lag a rebuilt installed CLI; this
  packet observed that attached MCP status may still show stale wording until
  the host process reloads the same build.

Next move:

- Do not strengthen product-value claims until the equal-budget comparison
  protocol below has been run or explicitly marked out of scope.
- Rerun the packet after a fresh operator-approved R9 selection is applied, then
  compare drift-event fanout and missing-subject metrics before and after.

## Evidence Packet — 2026-06-23

| Field | Value |
|-------|-------|
| Bearer | `haft-semantic-spine-rewrite` |
| Window | `2026-06-23` |
| Method ref | `dogfood-current-goal` |
| Value surface | `haft value space haft-semantic-spine-rewrite --window 2026-06-23 --method-ref dogfood-current-goal ...` |
| Value surface output | `single_score=no_single_haft_or_fpf_score`, `evidence_refs=7`, `evidence_missing_characteristics=0` |

Evidence refs:

- `commit:d5c88368` — semantic drift targets recognize `section_id`.
- `commit:6ecd5cee` — measurement evidence preserves current F0-F9 formality metadata.
- `commit:8a6e6db2` — unversioned formality stays diagnostic instead of being promoted to current F0-F9.
- `commit:ce368364` — `haft value space` compact output shows evidence refs and missing evidence characteristics.
- `commit:6ca19baf` — read-only MCP query footers suppress raw stale debt when the typed status stale lane is unavailable.
- `test:go-test-internal-scopeauth` — `go test ./internal/scopeauth -count=1` passed on 2026-06-23.
- `test:go-test-internal-cli-scope-violation` — targeted harness out-of-scope tests in `internal/cli` passed on 2026-06-23.

## What This Supports

- Atomic slice discipline is working for this rewrite window: each listed commit
  has focused tests and a full `go test ./...` run in the same session.
- The value surface does not collapse to a single scalar score.
- Compact read-only query footers fail closed on stale debt rather than
  reintroducing raw `FindStaleDecisions` counts that can disagree with the
  status cockpit.
- Scope violation handling is present at the code path tested by
  `scopeauth.AuthorizeWorkspaceDiff` and harness apply/result formatting.
- Missing value evidence is visible on the explicit value-space surface instead
  of being hidden in a design-time plan.

## What This Does Not Support Yet

- Equal-budget comparison results against an AI agent using ADRs plus tests.
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

## Equal-Budget Comparison Design

This protocol is designed but not yet run.

Goal: compare Haft against "AI agent + ADRs + tests" on the same class of
semantic-governance rewrite task, without optimizing a single score.

Task class:

- medium-size product/governance slice touching code, tests, changelog, and at
  least one support/spec carrier;
- non-destructive, no external commitment, no data migration;
- work must include one deliberate scope-boundary or authority-boundary check.

Parity budget:

- same model family and host;
- same repository commit at start;
- same elapsed-time cap;
- same permission envelope;
- same acceptance text;
- same required final artifacts: code/doc diff, tests, changelog, commit, and
  final operator report.

Comparison arms:

| Arm | Enabling method | Required carriers |
|-----|-----------------|-------------------|
| Haft | status + MethodRun + carrier/value/drift surfaces | Decision/method/evidence traces, `haft value space`, carrier check |
| ADR+tests | normal agent workflow using ADR note plus tests | ADR markdown, test log, changelog, commit |

Measured characteristics:

| Characteristic | Method | Window | Denominator | Evidence |
|----------------|--------|--------|-------------|----------|
| Semantic fidelity | Count corrected authority/object/evidence confusions found before commit | one task | all load-bearing findings | review packet + final diff |
| Ceremony cost | Wall time and number of operator interruptions | one task | accepted green slice | transcript timestamps |
| Scope control | Whether out-of-scope edit is blocked or surfaced before commit | one task | deliberate scope attempt | scopeauth/harness output or equivalent |
| Recoverability | Ability to identify exact source record for a claim after commit | one task | sampled claims | `haft_query`/ADR lookup transcript |
| Drift noise | Unique drift events and max fanout after commit | one task | post-commit status | `haft_query(action="drift_events")` summary |
| Delivery correctness | Focused tests and `go test ./...` pass | one task | changed surface | test output |

Missingness policy:

- If one arm lacks evidence for a characteristic, that characteristic is
  `missing`, not zero.
- No aggregate winner is computed. The result is a Pareto comparison plus
  protected trade-offs.
- A value claim may be made only for characteristics with evidence in the same
  task window.

Protected trade-offs:

- semantic fidelity vs ceremony;
- early detection vs false positives;
- durable traceability vs artifact explosion;
- automation vs principal control;
- compact views vs exact source recoverability.

Run output:

- `spec/target-system/PRODUCT_VALUE_EVIDENCE.md` gets a new dated packet;
- `haft value space <bearer> --window <date> --method-ref equal-budget-comparison`
  gets evidence refs for both arms;
- changelog records only the evidence packet, not a product-value victory.

## Next Evidence Needed

1. Run the equal-budget comparison protocol above.
2. Capture ceremony time and interruption cost for at least one short task and
   one long rewrite.
3. Record a host-backed receipt experiment only when a host can provide
   principal, session, action, payload hash, expiry, and verifier source.
4. Re-run this packet after R9 old-decision enrichment is operator-approved and
   applied, then compare drift-event fanout before and after.
