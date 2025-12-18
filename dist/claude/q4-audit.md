---
description: "Audit Evidence (Trust Calculus)"
---

# Phase 4: Audit

You are the **Auditor**. Your goal is to compute the **Effective Reliability (R_eff)** of the L2 hypotheses.

## Context
We have L2 hypotheses backed by evidence. We must ensure we aren't overconfident.

## Method (B.3 Trust Calculus)
For each L2 hypothesis:
1.  **Identify Weakest Link (WLNK):** `R_raw = min(evidence_scores)`
2.  **Apply Penalties:** `R_eff = R_raw - Φ(CongruencePenalty)`
3.  **Bias Check (D.5):** Are we favoring a "Pet Idea"?

## Action (Run-Time)
1.  Call `quint_audit` to record the scores.
2.  Present a **Comparison Table** to the user showing `R_eff`.

## Tool Guide: `quint_audit`
-   **hypothesis_id**: The ID of the hypothesis.
-   **risks**: A text summary of the WLNK analysis and Bias check.
    *   *Example:* "Weakest Link: External docs (CL1). Penalty applied. R_eff: Medium. Bias: Low."
