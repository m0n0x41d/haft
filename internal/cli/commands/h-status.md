---
description: "Dashboard of active decisions, stale items, and recent notes"
---

# Status

Show what's active, what's stale, and what's recent.

Use `haft_query` tool with `action="status"`.
Optionally filter by `context`.

`status` is the at-a-glance overview. When the operator or CI needs the
**actionable enforcement** picture (decision drift + evidence decay + spec
drift + spec stale + spec structural in one structured response), call
`haft_query(action="check")` instead — that is the plugin-mode parity for
the CLI `haft check` command and the right entry point for `/h-verify`.

First interpret project readiness if it is present:

- `missing`: the path is not a usable project.
- `needs_init`: initialize Haft before creating decisions or commissions.
- `needs_onboard`: run onboarding/spec work before broad harness execution.
- `ready`: spec-backed work can move through decisions and commissions.

If status shows `needs_onboard`, tell the user the next action is onboarding:
inspect or create TargetSystemSpec, EnablingSystemSpec, and TermMap, then run
`haft spec check`. Do not treat `.haft/project.yaml` alone as execution
readiness.

If status shows stale, blocked, or running-too-long WorkCommissions needing
attention, treat them as operator work:

- inspect with `haft_commission(action="show", commission_id="wc-...")`
- requeue with a reason only when the decision/scope is still current and the
  stale or blocked cause has been fixed
- cancel when the work is obsolete, duplicated, or no longer authorized
- cancel expired unfinished commissions instead of requeueing them; create a
  fresh commission only if the current DecisionRecord still authorizes the work
- never infer completion from a WorkCommission merely being absent from
  runnable work
- never ask an operator to physically delete a WorkCommission as normal
  lifecycle management
- these are lifecycle management actions only; do not start Open-Sleigh or a
  long-running runtime from `/h-status`

$ARGUMENTS
