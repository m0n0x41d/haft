---
description: "Create WorkCommissions from active DecisionRecords without starting the harness"
---

# Commission

Create or reuse WorkCommissions from Haft DecisionRecords. This is the
authorization step only: it must not start Open-Sleigh or any long-running
runtime. Plugin mode does not own runtime lifecycle; CLI or Desktop operates
the harness runtime after the operator chooses to run it.

Operator recovery path:

```bash
haft commission list --selector stale
haft commission show wc-...
haft commission requeue wc-... --reason stale_operator_recovery
haft commission cancel wc-... --reason no_longer_relevant
```

Use cancellation instead of deleting a WorkCommission. A WorkCommission is an
audit/authority record; "remove it from work" means move it to `cancelled` with
a reason.

Default path:

```bash
haft harness run --prepare-only
```

This selects active DecisionRecords that do not already have WorkCommissions,
generates an inspectable `.haft/plans/*.yaml`, creates bounded
WorkCommissions, and stops before execution.

Use explicit selectors only when the user asks for a narrower set:

```bash
haft harness run dec-a dec-b --prepare-only
haft harness run --problem prob-... --prepare-only
haft harness run --context harness-mvp --prepare-only
haft harness run --all-active-decisions --prepare-only
haft harness run --plan .haft/plans/implementation.yaml --prepare-only
```

If the user supplies exact DecisionRecord ids and asks for the low-level MCP
tool instead of the packaged operator path, use `haft_commission`:

- `action="create_from_decision"` for one decision
- `action="create_batch_from_decisions"` for several decisions
- `action="create_from_plan"` for a prepared ImplementationPlan
- `action="list", selector="stale"` to find old or blocked commissions
- `action="show"` with `commission_id` to inspect one commission and operator hints
- `action="requeue"` with `commission_id` and `reason` to return a recoverable
  commission to queued
- `action="cancel"` with `commission_id` and `reason` to close unfinished work
  without deleting the record

Lifecycle contract:

- `list` and `show` are read-only; they must not change commission state.
- `requeue` is for stale-but-still-current or blocked/failed work after the
  cause is fixed. It is allowed only from `queued`, `ready`, `preflighting`,
  `running`, `blocked_stale`, `blocked_policy`, `blocked_conflict`,
  `needs_human_review`, or `failed`; it clears the lease, refreshes
  `fetched_at`, records the reason, and moves the commission to `queued`.
- Do not requeue a commission whose `valid_until` has expired. Cancel it and
  create a fresh commission from the current DecisionRecord/scope if work is
  still needed.
- `cancel` is for obsolete, duplicate, unauthorized, or expired unfinished work.
  It records the reason, clears the lease, and moves the commission to
  `cancelled`.
- `completed`, `completed_with_projection_debt`, `cancelled`, and `expired`
  commissions are audit records; do not requeue or cancel them again.

Required discipline:

- Do not treat a DecisionRecord as scheduled work until a WorkCommission exists.
- Do not create a WorkCommission for a stale, superseded, deprecated, or
  ambiguous DecisionRecord.
- Preserve the boundary: DecisionRecord = chosen direction;
  WorkCommission = bounded permission to execute; RuntimeRun = actual attempt.
- Do not physically delete WorkCommissions during normal operation; cancel or
  requeue them so status/verify can explain what happened.
- Prefer default scope derived from `affected_files`; add explicit
  `--allowed-path`, `--lock`, or `--evidence` only when the user gives them or
  the DecisionRecord is too broad to run safely.
- Report the generated plan path and whether commissions were created or reused.

$ARGUMENTS
