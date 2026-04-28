---
description: "Verify mode — check what's stale, drifted, or ready for measurement, then act"
---

# Verify

Single entry point for everything in Verify mode. Finds problems, then helps resolve them.

## Step 1: Discovery (always run first)

Call `haft_query(action="check")` — the unified, CI-actionable enforcement
report covering every kind of debt in one structured response:
- **Stale** decisions/artifacts (valid_until passed; evidence-degraded R_eff)
- **Drift** on baselined decisions (file hashes diverged from baseline)
- **Unassessed** decisions (active but no evidence/measurement yet)
- **Coverage gaps** on decisions
- **Spec health** — drift on approved SpecSections (`spec_section_drifted`),
  missing baselines (`spec_section_needs_baseline`), time-based staleness
  (`spec_section_stale` — active section whose `valid_until` is past today),
  and L0/L1/L1.5 structural carrier findings.

`haft_query(action="check")` is the plugin-mode parity for the CLI `haft check`
command and returns the same JSON shape — fixture parity is enforced by tests.
Use `status` for an at-a-glance overview; use `check` when the operator or CI
must act on debt. If you need finer-grained legacy data,
`haft_refresh(action="scan")` and `haft_refresh(action="drift")` still work
for decision/evidence-only scans.

Then call `haft_commission(action="list", selector="stale")` to find execution
authority objects that still need operator attention:
- queued/ready commissions open longer than the default threshold
- preflighting/running commissions with stale leases
- blocked, failed, or human-review commissions
- expired commissions that never reached a terminal state

## Step 2: Triage (present to user)

For each stale item, explain in human terms:
- lead with the artifact **title**, not just the ID
- say what the decision/problem/note was about in one short phrase
- explain the concrete issue: weak evidence, no baseline, expired, drift, or pending verification
- for pending verify_after claims: show the observable and threshold so the user knows what to check
- for stale WorkCommissions: show the commission id, linked decision, state,
  reason, and suggested actions

## Step 3: Act (for each item, suggest an action)

| Situation | Action | Tool call |
|-----------|--------|-----------|
| Claim ready to verify | Run the measurement | `haft_decision(action="measure", ...)` |
| Decision still valid but expired | Extend with justification | `haft_refresh(action="waive", artifact_ref=..., reason=..., new_valid_until=...)` |
| Decision outdated, needs rethink | Reopen as new problem | `haft_refresh(action="reopen", artifact_ref=..., reason=...)` |
| Artifact replaced by newer one | Supersede | `haft_refresh(action="supersede", artifact_ref=..., new_artifact_ref=..., reason=...)` |
| Problem/note no longer relevant | Archive | `haft_refresh(action="deprecate", artifact_ref=..., reason=...)` |
| Code drifted materially | Re-verify or reopen | Check if drift breaks invariants, then measure or reopen |
| WorkCommission still valid but stuck | Requeue | `haft_commission(action="requeue", commission_id=..., reason=...)` |
| WorkCommission obsolete or duplicate | Cancel | `haft_commission(action="cancel", commission_id=..., reason=...)` |
| Spec section drifted (intentional evolution) | Rebaseline with reason | `haft_spec_section(action="rebaseline", section_id=..., reason=...)` |
| Spec section drifted (needs review) | Reopen the section | `haft_spec_section(action="reopen", section_id=..., reason=...)` |
| Spec section stale (`valid_until` past) | Extend valid_until in carrier, then rebaseline; or reopen | `haft_spec_section(action="rebaseline" | "reopen", ...)` |
| Spec section missing baseline | Approve | `haft_spec_section(action="approve", section_id=...)` |

Present the triage to the user. Let them decide what to do with each item. Execute the chosen actions.

## When to use

- `/h-verify` — "what needs attention? what's stale? what drifted?"
- Session start — agent should proactively run verify discovery
- After implementation — check if claims are ready for measurement
- Periodic hygiene — keep the decision base healthy

$ARGUMENTS
