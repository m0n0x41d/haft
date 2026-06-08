---
name: h-status
description: |
  Project state dashboard for haft — read-only consolidation of active problems, pending decisions, refresh-due artifacts, open work commissions, recent notes, module coverage, and spec lifecycle readiness when relevant. Make sure to use this skill whenever the user asks "where are we", "what's pending", "what's stale", "project status", "what needs attention", "show me the state", "what's in flight", "what did we decide on X recently", "haft status", or "spec readiness" — or whenever a session resumes after a break and situational awareness is needed before deciding what to work on. Cheap, read-only, zero commitments. For verifying a single decision use h-verify. For managing commission lifecycle use h-commission. For editing specs use h-spec.
when_to_use: |
  Operator wants situational awareness or session-resume context. Cheap and read-only, fire freely.
argument-hint: "[optional: context name to filter]"
allowed-tools: Bash mcp__haft__haft_query mcp__haft__haft_refresh mcp__haft__haft_spec_section
---

# h-status — Project FPF state dashboard

You are surfacing the current FPF state via `mcp__haft__haft_query(action="status")`. Read-only — no kernel writes (Step 0's scan is a maintenance write, not a state mutation).

## Compact interface discovery

If you need the exact compact contract, run `haft interface query.status --json`.
Default status output is compact; pass `full=true` only when the operator needs
the complete module coverage list.

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

The status payload contains:

- **Shipped / Healthy** — recent decisions that are still active and within valid_until window
- **Pending** — decisions awaiting implementation, baseline, or measure
- **Unassessed** — decisions without recent measurement
- **Refresh Due** — decisions/evidence past valid_until, drift detected, or R_eff degraded; also epistemic debt budget if exceeded
- **WorkCommissions Need Attention** — blocked, stale, or expired commissions
- **Backlog** — active problems without committed decisions
- **Addressed** — recently closed problems with their decisions
- **Recent Notes** — last 5 micro-decisions
- **Module Coverage Summary** — compact per-tier coverage; use
  `mcp__haft__haft_query(action="status", full=true)` or
  `mcp__haft__haft_query(action="coverage")` for the full module list

The response also includes a navigation strip with available next actions (e.g., `/h-refresh`).

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

- Refresh-due items → recommend `/h-verify` for the specific decisions
- Blocked work commissions → recommend `/h-commission` with `action=show` and operator review
- Epistemic debt budget exceeded → recommend running `/h-verify` on the highest-debt decisions
- Blind modules (no decisions) → recommend `/h-frame` if upcoming work targets that module
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
- Do NOT use this skill for full-text search across decisions — that's h-search via `mcp__haft__haft_query(action="search", query="...")`.
- Do NOT call this skill on every turn. Auto-trigger is fine when the operator explicitly asks; constant polling pollutes context with stale state.

## FPF spec references

- B.3.4 — Evidence Decay & Epistemic Debt (drives refresh-due classification)
- F.10 — Three Ladders: Evidence / Standard / Requirement status
- VER-02 — Decay (valid_until expiry semantics)
- X-WLNK — Weakest-link aggregation for R_eff degradation surfacing

Look up via `mcp__haft__haft_query(action="fpf", query="B.3.4")`.
