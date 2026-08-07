---
name: h-verify
description: Verify that a recorded DecisionRecord or claim still holds by comparing its baseline and predictions with current code, tests, measurements, or incidents. Keep design-time claims distinct from runtime evidence.
---

# h-verify — Evidence against reality

When an FPF evidence or validity distinction is material, inspect a known
SourceID/UnitID with `haft_query(action="fpf", mode="inspect",
identifier="...")`, or query the exact concern and inspect the direct pattern
body. Retrieval is not evidence or a verdict.

Recover the exact record and its claims, thresholds, validity window, and
planned evidence. Gather current evidence, state context transfer and expiry,
and attach it with `haft_decision(action="evidence", ...)`. Surface weakened,
refuted, stale, or drifted claims honestly. Any rebaseline, supersede,
deprecate, or reopen mutation requires explicit operator action.
