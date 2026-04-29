# Dogfood Spec Readiness State

This note records the current Haft repository dogfood state. It is not a
`yaml spec-section` carrier and does not create active target-system or
enabling-system authority.

As of 2026-04-26, this repository has local `.haft/specs/*` carriers, but the
root `.gitignore` ignores `.haft/`. Edits to those local carriers are therefore
not captured in a normal repository patch. The local carriers are generated
draft placeholders from `internal/project/spec_carriers.go`:

- `.haft/specs/target-system.md` has one draft target placeholder.
- `.haft/specs/enabling-system.md` has one draft enabling placeholder.
- `.haft/specs/term-map.md` has an empty draft `entries: []` term map.

The honest readiness state is `needs_onboard`, not ready. Operators should keep
the placeholders draft, run `haft spec check --json`, and either unignore the
reviewable `.haft/specs` carriers or continue recording dogfood state in
tracked specs/tests until the project-local carrier policy is reconciled.

## Batch Drainer Readiness Target

As of 2026-04-29, decision
`dec-20260428-harness-drain-v3-16bf21f3` defines the next dogfood readiness
target for batch WorkCommission execution. This note records the readiness
target only; it is not runtime evidence that the harness or Open-Sleigh
implementation already satisfies the target.

The readiness state remains `needs_onboard` until the Measure phase observes all
of the following on the target system:

- `haft harness run --drain --concurrency N` is opt-in and does not change the
  existing single-commission `haft harness run` behavior when `--drain` is
  absent.
- Drain mode keeps the runtime alive while runnable WorkCommissions remain,
  runs up to the requested concurrency, and exits cleanly when the runnable
  queue is empty.
- Lockset-overlapping commissions remain blocked at claim time in drain mode.
- AutonomyEnvelope is evaluated at commission creation and re-evaluated at
  preflight and execute before any auto-apply path can act.
- A terminal commission is auto-applied only when verdict is `pass`,
  `delivery_policy` is `workspace_patch_auto_on_pass`, and the envelope decision
  is `allowed`.
- Commissions that fail any auto-apply precondition remain available for
  operator-controlled `haft harness apply`.
- Stale leases older than the configured age cap, with 24 hours as the default,
  are skipped at intake with a typed `lease_too_old` reason and are surfaced by
  `haft harness status` for operator action.
- Per-commission apply remains a discrete, revertable local git operation. Drain
  mode performs no remote operations.
- Operator intervention through `harness apply`, `harness requeue`, or
  `harness cancel` remains available without SIGKILL or SQLite surgery.

The evidence gate for moving this repository beyond `needs_onboard` is the
bounded command set carried by the active WorkCommission, including the mixed
delivery-policy end-to-end batch dogfood run. Until that evidence exists, the
batch-drainer behavior is a readiness target, not a verified project fact.
