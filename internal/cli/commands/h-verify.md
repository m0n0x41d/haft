---
description: "Verify mode — check what's stale, drifted, or ready for measurement, then act"
---

# Verify

Single entry point for everything in Verify mode. Finds problems, then helps resolve them.

## Step 1: Discovery (always run first)

Call `haft_refresh(action="scan")` to find ALL issues at once:
- expired decisions and artifacts (valid_until passed)
- evidence-degraded decisions (R_eff < 0.5 or AT RISK < 0.3)
- claims with verify_after dates that passed but remain unverified
- file drift on baselined decisions (use `haft_refresh(action="drift")` if project root available)

## Step 2: Triage (present to user)

For each stale item, explain in human terms:
- lead with the artifact **title**, not just the ID
- say what the decision/problem/note was about in one short phrase
- explain the concrete issue: weak evidence, no baseline, expired, drift, or pending verification
- for pending verify_after claims: show the observable and threshold so the user knows what to check

## Step 3: Act (for each item, suggest an action)

| Situation | Action | Tool call |
|-----------|--------|-----------|
| Claim ready to verify | Run the measurement | `haft_decision(action="measure", ...)` |
| Decision still valid but expired | Extend with justification | `haft_refresh(action="waive", artifact_ref=..., reason=..., new_valid_until=...)` |
| Decision outdated, needs rethink | Reopen as new problem | `haft_refresh(action="reopen", artifact_ref=..., reason=...)` |
| Artifact replaced by newer one | Supersede | `haft_refresh(action="supersede", artifact_ref=..., new_artifact_ref=..., reason=...)` |
| Problem/note no longer relevant | Archive | `haft_refresh(action="deprecate", artifact_ref=..., reason=...)` |
| Code drifted materially | Re-verify or reopen | Check if drift breaks invariants, then measure or reopen |

Present the triage to the user. Let them decide what to do with each item. Execute the chosen actions.

## When to use

- `/h-verify` — "what needs attention? what's stale? what drifted?"
- Session start — agent should proactively run verify discovery
- After implementation — check if claims are ready for measurement
- Periodic hygiene — keep the decision base healthy

$ARGUMENTS
