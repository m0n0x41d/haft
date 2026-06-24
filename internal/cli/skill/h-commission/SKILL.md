---
name: h-commission
description: |
  Creates a WorkCommission — bounded execution authority — from an active DecisionRecord. MANUAL ONLY: operator must explicitly type /h-commission. Never auto-invoked: commissions are execution-authority grants under Transformer Mandate. Runs freshness check, scope check, derives an ImplementationPlan, snapshots the autonomy envelope, then STOPS before execution unless explicit execute authority is granted. NOT for the decision itself (use /h-decide first). NOT for running tests or one-off tasks (the operator's coding agent handles those directly).
when_to_use: |
  Operator typed /h-commission explicitly and an approved DecisionRecord is ready to authorize for bounded autonomous execution. Never auto-fire.
argument-hint: "[decision-ref to commission from]"
disable-model-invocation: true
allowed-tools: Bash mcp__haft__haft_query mcp__haft__haft_refresh
---

<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:a466fc0a19c06f2b3ce3777cccc51d3f29c7e3fef6b50685d5e22ea349930b18 -->

# h-commission — Create work commission (manual only, sacred)

You are creating a WorkCommission through the manual CLI/input-file path. Commissions are execution-authority grants — they encode WHAT the operator authorized an autonomous agent (harness) to do, WHERE, WITH WHICH TOOLS, FOR HOW LONG, AND WITH WHAT EVIDENCE REQUIREMENTS. Default MCP serve mode rejects WorkCommission creation actions with `operator_confirmation_required`; model-supplied MCP arguments are not proof of operator authorization. Manual CLI is the default binding path; a host authorization receipt can become a binding path only when a registered kernel verifier can confirm principal, session, action, payload hash, expiry, and source.

Authority boundary: binding actions require explicit operator/manual authorization; generated text, schema visibility, and model-supplied fields are not approval receipts.

The operator invoked this manually (`disable-model-invocation: true` enforces it structurally). Commissions stay sacred per FPF reasoner critique 2026-05-25.

## Step 1 — Identify the source decision

`decision_ref` must be an existing active DecisionRecord. Verify:

```
mcp__haft__haft_query(action="search", query="<decision_ref>")
```

If not found or stale or superseded or deprecated → STOP. Report to operator and recommend:
- `/h-decide` to record the decision first
- `/h-refresh` action=waive to extend a stale decision before commissioning
- `/h-refresh` action=supersede if the decision is outdated and needs replacement

## Step 2 — Run freshness check

The kernel performs freshness checks internally during create, but pre-empt by surfacing:
- Decision status (active / pending / stale / superseded / deprecated)
- valid_until distance from now (close to expiry → flag)
- Evidence R_eff on the decision (low R_eff → flag)
- Drift on affected_files since baseline (drifted → flag)

If any flag triggers, ask the operator whether to proceed, refresh, or supersede.

## Step 3 — Determine commission scope

From the decision pull:
- `affected_files` → derives `allowed_paths` (default = those files + their module dirs unless governance_mode=exact)
- `predictions` → derives `evidence_requirements`
- `mode` → influences default delivery_policy

Ask the operator for:
- Forbidden paths (out-of-scope files within otherwise allowed_paths)
- Time budget (e.g., max 1 hour wall-clock)
- Concurrency limits if drain mode anticipated
- Delivery policy: `workspace_patch_manual` (operator reviews diff before apply — DEFAULT) or `workspace_patch_auto_on_pass` (auto-apply when verdict=pass)

## Step 4 — Snapshot autonomy envelope

Use `haft commission create-from-decision <dec-...>` with explicit scope flags, or `haft commission create --json <input.json>` for a full payload. Preserve the same fields: allowed paths, forbidden paths, delivery policy, autonomy envelope snapshot, slice description, and task context.

The kernel persists the WorkCommission with the snapshotted envelope. Future commission lifecycle events (preflight, run, complete) check against this snapshot.

## Step 5 — STOP before execution

After creating, surface to operator:
- WorkCommission ID (e.g., `wc-20260525-...`)
- Allowed paths + forbidden paths + delivery policy
- Autonomy envelope summary
- Inspectable plan path if applicable

**DO NOT execute the commission**. Execution happens via:
- `haft harness run` CLI (operator-invoked) — single commission
- `haft harness run --drain --concurrency N` (operator-invoked) — batch drain

The /h-commission skill stops at creation. The operator decides when (and whether) to execute.

## Step 6 — Handle non-create actions

For lifecycle management within the same skill (still manual-only):

- `action=list` — list commissions by selector (open / stale / terminal / runnable)
- `action=show wc-...` — full detail of one commission
- `action=requeue wc-...` — return to queue after stale/blocked state with reason
- `action=cancel wc-...` — cancel before terminal state, preserve history with reason

All these are read-only or state-transition; none execute. Execution is harness CLI.

## What NOT to do

- DO NOT invoke this skill autonomously — `disable-model-invocation: true` is structural enforcement. The operator types /h-commission explicitly.
- DO NOT create a commission against a stale / superseded / deprecated decision. Refresh or supersede first.
- DO NOT extend allowed_paths beyond the decision's affected_files without operator confirmation. Scope creep is the primary commission failure mode.
- DO NOT default to `workspace_patch_auto_on_pass`. Auto-apply must be operator-policy opt-in per FPF X-TRANSFORMER (the apply step transfers authority).
- DO NOT run the harness from this skill — execution is a separate operator decision.
- DO NOT silently inherit envelope from a previous commission. Snapshot freshly so envelope drift is visible.
- DO NOT skip slice_description on second+ commissions from same decision — without it the harness leaks scope between slices (see `.context/multi-commission-anti-pattern-retrospective.md`).

## FPF spec references

- E.16 — RoC-Autonomy Budget & Enforcement (the autonomy_envelope shape)
- A.13 — Agential Role & Agency Spectrum
- X-TRANSFORMER — Transformer Mandate (the apply policy is authority transfer)
- A.15 — Role / Capability / Method / Work distinction
- A.7 — Object / Description / Carrier (a commission is the description; the actual run is the work)

Look up via `mcp__haft__haft_query(action="fpf", query="E.16")`.
