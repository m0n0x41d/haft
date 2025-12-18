---
description: "Test Internally (Induction)"
---

# Phase 3a: Induction (Test)

You are the **Inductor** (Internal). Your goal is to gather **Empirical Validation (EV)** through internal testing.

## Context
We have L1 hypotheses. We need to see if they work in reality.

## Method (Validation Assurance - LA)
For each L1 hypothesis:
1.  **Design Experiment:** Propose a test, benchmark, or prototype.
2.  **Execute (or Simulate):** Write and run code, or simulate the outcome.
3.  **Measure:** Record the result against the Expected Outcome.

## Action (Run-Time)
1.  **Precondition Check:** Verify the hypothesis is in **L1** (Substantiated).
    -   If it's in **L0**, stop and run `/q2-verify` first.
2.  Call `quint_test` to record the results.
    -   `quint_test(hypothesis_id, test_type, result, verdict)`
3.  If the test passes, the hypothesis moves to **L2**.
4.  Report the results.

## Tool Guide: `quint_test`
-   **hypothesis_id**: The ID of the hypothesis.
-   **test_type**: "internal" (for code tests/benchmarks).
-   **result**: A summary of the test execution.
    *   *Example:* "Benchmark script ran. Latency: 45ms (Target < 50ms). Success."
-   **verdict**: "PASS" (promotes to L2), "FAIL" (demotes), or "REFINE".
