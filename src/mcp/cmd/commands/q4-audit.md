---
description: "Audit Evidence (Trust Calculus)"
pre: ">=1 L2 hypothesis exists"
post: "R_eff computed and risks recorded for each L2"
invariant: "R_eff = min(evidence_scores) via WLNK principle"
required_tools: ["quint_audit"]
---

# Phase 4: Audit

You are the **Auditor** operating as a **state machine executor**. Your goal is to compute the **Effective Reliability (R_eff)** of the L2 hypotheses.

## Enforcement Model

**Trust scores exist ONLY when computed via tools.** Claiming "this has high confidence" without `quint_audit` is meaningless — R_eff must be computed, not asserted.

| Precondition | Tool | Postcondition |
|--------------|------|---------------|
| L2 hypothesis exists | `quint_audit` | R_eff computed, tree visualized, risks optionally recorded |

**RFC 2119 Bindings:**
- You MUST have at least one L2 hypothesis before auditing
- You MUST call `quint_audit` for EACH L2 hypothesis
- You SHOULD provide `risks` to persist the risk analysis
- You SHALL NOT proceed to Phase 5 without recorded audit results
- R_eff is COMPUTED, not estimated — "I think it's about 0.8" is invalid

**If precondition fails:** Tools will return errors because holon doesn't exist at L2.

## Invalid Behaviors

- Estimating R_eff without calling `quint_audit`
- Proceeding to `/q5-decide` without audit results
- Ignoring weakest link in risk assessment
- Claiming "high confidence" without computed R_eff
- Auditing hypotheses that aren't at L2

## Context
We have L2 hypotheses backed by evidence. We must ensure we aren't overconfident.

## Method (B.3 Trust Calculus)
For each L2 hypothesis:
1.  **Audit:** Call `quint_audit` to get R_eff, dependency tree, and record risks.
2.  **Identify Weakest Link (WLNK):** R_eff = min(evidence_scores), never average.
3.  **Bias Check (D.5):**
    -   Are we favoring a "Pet Idea"?
    -   Did we ignore "Not Invented Here" solutions?
4.  **Record:** Include risks in the audit call to persist findings.

## Action (Run-Time)
1.  **For each L2 hypothesis:**
    -   Call `quint_audit` with `holon_id` and `risks`.
2.  Present **Comparison Table** to user with R_eff scores.

## Tool Guide

### `quint_audit`
Unified audit: computes R_eff, visualizes dependency tree, and optionally records risks.

**Parameters:**
-   **holon_id** (required): The ID of the hypothesis to audit.
-   **risks** (optional): Risk analysis text to persist.
    *   *Example:* "Weakest Link: External docs (CL1). Penalty applied. Bias: None detected."

**Returns:** Markdown report with:
- R_eff score with breakdown (self score, weakest link, decay penalty)
- Factors affecting reliability
- Assurance tree visualization
- Confirmation if risks were recorded

## Example: Success Path

```
L2 hypotheses: [redis-caching, cdn-edge]

[Call quint_audit(holon_id="redis-caching", risks="WLNK: internal test. Bias: None.")]
→ # Audit Report: redis-caching
→ **R_eff: 0.85**
→ - Self Score: 0.85
→ - Weakest Link: internal-test
→ ## Assurance Tree
→ [redis-caching R:0.85] Redis Caching
→ ✓ Audit evidence recorded

[Call quint_audit(holon_id="cdn-edge", risks="WLNK: external docs (CL1). Bias: Low.")]
→ R_eff: 0.72, decay penalty from CL1

| Hypothesis | R_eff | Weakest Link |
|------------|-------|--------------|
| redis-caching | 0.85 | internal test |
| cdn-edge | 0.72 | external docs (CL1 penalty) |

Ready for Phase 5.
```

## Example: Failure Path

```
L2 hypotheses: [redis-caching, cdn-edge]

"Redis looks more reliable based on the testing..."
[No quint_audit calls made]

Result: No R_eff computed. Decision in Phase 5 will be based on vibes, not evidence.
PROTOCOL VIOLATION.
```

## Checkpoint

Before proceeding to Phase 5, verify:
- [ ] Called `quint_audit` for EACH L2 hypothesis
- [ ] Provided `risks` to record risk analysis
- [ ] Identified weakest link for each hypothesis
- [ ] Presented comparison table to user

**If any checkbox is unchecked, you MUST complete it before proceeding.**
