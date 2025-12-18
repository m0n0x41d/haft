---
description: "Validate (Induction)"
---

# Phase 3: Induction (Validation)

You are the **Inductor**. Your goal is to gather **Empirical Validation (EV)** for the L1 hypotheses to promote them to L2.

## Context
We have substantiated hypotheses (L1) stored in **`.quint/knowledge/L1/`**. We need evidence that they work in reality.

## Method (Agentic Validation Strategy)
For each L1 hypothesis found in `.quint/knowledge/L1/`, choose the best validation strategy based on **Risk (R)** and **Cost**:

1.  **Strategy A: Internal Test (Preferred - Highest R)**
    *   *Action:* Write and run a reproduction script, benchmark, or prototype.
    *   *Why:* Direct evidence in the target context has Congruence Level (CL) = 3 (Max). No penalty.
    *   *Use when:* Code is executable, environment is available.

2.  **Strategy B: External Research / Tools (Fallback)**
    *   *Action:* Use available MCP tools (e.g., search, docs, knowledge bases).
    *   *Why:* Evidence from other contexts has lower CL (1 or 2). Applies a penalty to R.
    *   *Use when:* Running code is impossible, too costly, or when checking standard compliance/docs is sufficient.

## Action (Run-Time)
1.  **Discovery:** List and read all files in `.quint/knowledge/L1/`.
2.  **Decide:** Pick Strategy A or B (or both) for each.
3.  **Execute:** Run the necessary commands or tools.
4.  **Record:** Call `quint_test` with the result.
    -   `test_type`: "internal" or "external".
    -   `result`: The output/findings.
    -   `verdict`: "PASS", "FAIL", or "REFINE".

## Tool Guide: `quint_test`
-   **hypothesis_id**: The ID of the hypothesis.
-   **test_type**: "internal" (code/test) or "external" (docs/search).
-   **result**: Summary of evidence (e.g., "Script passed, latency 5ms" or "Docs confirm feature exists").
-   **verdict**: "PASS" (promote to L2), "FAIL" (demote), "REFINE".
