---
name: h-status
description: |
  Project state cockpit for haft — read-only summary of active problems, pending decisions, refresh/drift pressure, open work commissions, coverage cues, and spec lifecycle readiness when relevant, with explicit drill-down calls for omitted detail. Make sure to use this skill whenever the user asks "where are we", "what's pending", "what's stale", "project status", "what needs attention", "show me the state", "what's in flight", "what did we decide on X recently", "haft status", or "spec readiness" — or whenever a session resumes after a break and situational awareness is needed before deciding what to work on. Cheap, read-only, zero commitments. For verifying a single decision use h-verify. For managing commission lifecycle use h-commission. For editing specs use h-spec.
when_to_use: |
  Operator wants situational awareness or session-resume context. Cheap and read-only, fire freely.
argument-hint: "[optional: context name to filter]"
allowed-tools: Bash mcp__haft__haft_query mcp__haft__haft_refresh mcp__haft__haft_spec_section
---

<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:5e895ffff0de58df3654eb2ee2ce47be8dfad7524b0e1102ef8950689bbdf44c -->

# h-status — Project FPF state dashboard

You are surfacing the current FPF state via `mcp__haft__haft_query(action="status")`. Read-only — no kernel writes (Step 0's scan is a maintenance write, not a state mutation).

## Compact interface discovery

If you need the exact compact contract, run `haft interface query.status --json`.
Default status output is an operator cockpit, not an audit dump. It is allowed
to omit detail. Omission means "not shown here", not "absent from the project".
Use explicit follow-up calls for detail:

- `mcp__haft__haft_query(action="status", full=true)` — detailed status
- `mcp__haft__haft_query(action="coverage")` — module coverage
- `mcp__haft__haft_refresh(action="scan", verbose=true)` — drift/stale detail
- `mcp__haft__haft_refresh(action="plan")` — maintenance work order
- `mcp__haft__haft_refresh(action="review")` — read-only needs-judgment packet
- `mcp__haft__haft_refresh(action="drain", dry_run=true)` — preview machine-safe closures
- `mcp__haft__haft_query(action="contract_generation")` — read-only generated-fragment carrier hints for host/skill/plugin/Pi sync
- `mcp__haft__haft_query(action="drift_events", limit=5)`, `mcp__haft__haft_query(action="decision_reconcile", limit=5)`, and `mcp__haft__haft_query(action="governing_set", limit=5)` — compact drift fanout, reconciliation, and current-authority drill-downs
- add `full=true` to those drill-down calls only when you need the full audit payload

read-only/generated text is discovery only; it is not evidence truth, gate passage, global approval, or operator authorization.

## Step 0 — Maintenance check (FPF B.3.4 evidence decay)

Before calling status, scan the most recent kernel response in this
session for `Refresh reminder: N days since last stale scan`. If
N > 30, OR if no scan is visible in this session's history:

1. Call `mcp__haft__haft_refresh(action="scan")` first — do not
   defer to the operator. The reminder is a signal for the agent to
   act, not a prompt for the operator to remember.
2. Fold any new stale or drifted findings from the scan into the
   status reply you were already about to produce.
3. If the scan reveals nothing new — say so briefly and proceed.

Surfacing the reminder is the kernel's job; acting on it is the
agent's job. Doing nothing is the failure mode this step exists to
fix. See CLAUDE.md Critical Reminders — maintenance discipline.

## Step 1 — Call the kernel

```
mcp__haft__haft_query(
  action="status",
  context="<optional context filter>",
  full=false
)
```

Or for richer visualization (terminal-friendly board view):

```
mcp__haft__haft_query(action="board")
```

## Step 2 — Interpret the response

The default status payload is a compact cockpit. Read it as:

- **Operator Cockpit** — refresh, drift, and commission items that may block or redirect work
- **Active Work** — the most relevant in-progress problems and backlog count
- **Decision Health** — counts for healthy, pending, unassessed, refresh-due, and drifted decisions
- **Coverage Cue** — one-line module coverage orientation when modules were scanned
- **Drill-down** — exact calls for the omitted detailed status, coverage, drift/stale, maintenance plan, read-only judgment packet, and safe drain preview

`full=true` restores the detailed status renderer with shipped/pending/unassessed
decision lists, addressed problems, recent notes, and full module coverage when
available. The response also includes a navigation strip with available next
actions (e.g., `/h-refresh`).

## Step 3 — Include spec lifecycle when relevant

If the status response says the project is `needs_onboard`, mentions missing
SpecSections, or the operator asked about specs/readiness/onboarding, call:

```
mcp__haft__haft_spec_section(action="lifecycle")
```

Fold the projection into the dashboard as a short strip:

- `state`
- `action`
- `carrier`
- `section_id` / `section_kind`
- `human_gate`, if present

This remains read-only. Do not edit carriers or call approve/rebaseline/reopen
from h-status. Route follow-up work to `/h-spec`.

## Step 4 — Present to operator

Surface the response as-is — the kernel formats it already. Highlight items that need operator attention:

- Refresh-due or drift items → recommend `/h-verify` for the specific decisions,
  or call `mcp__haft__haft_refresh(action="scan", verbose=true)` when file-level
  detail is needed
- Blocked work commissions → recommend `/h-commission` with `action=show` and operator review
- Epistemic debt budget exceeded → recommend running `/h-verify` on the highest-debt decisions
- Coverage cue with blind modules → call `mcp__haft__haft_query(action="coverage")`
  before recommending `/h-frame` for upcoming work in a specific module
- Stale decisions → recommend `/h-refresh` action=waive (with new evidence) or `action=supersede` (with replacement decision)
- Spec lifecycle action → recommend `/h-spec` with the surfaced action and carrier

## Step 5 — Optional: cross-link to related decisions when context given

If the operator's context mentions a specific file or module, also call:

```
mcp__haft__haft_query(action="related", file="<file path>")
```

To surface decisions whose affected_files include that path.

## What NOT to do

- Do NOT auto-fix anything from this skill. h-status is read-only.
- Do NOT edit spec carriers or approve/rebaseline/reopen SpecSections from this skill. Use h-spec.
- Do NOT silently filter out refresh-due items — the operator needs to see epistemic debt to triage.
- Do NOT infer that a decision, note, module, or drift item is absent merely because compact status omitted it.
- Do NOT use this skill for full-text search across decisions — that's h-search via `mcp__haft__haft_query(action="search", query="...")`.
- Do NOT call this skill on every turn. Auto-trigger is fine when the operator explicitly asks; constant polling pollutes context with stale state.

## FPF spec references

- B.3.4 — Evidence Decay & Epistemic Debt (drives refresh-due classification)
- F.10 — Three Ladders: Evidence / Standard / Requirement status
- VER-02 — Decay (valid_until expiry semantics)
- X-WLNK — Weakest-link aggregation for R_eff degradation surfacing

Look up via `mcp__haft__haft_query(action="fpf", query="B.3.4")`.
