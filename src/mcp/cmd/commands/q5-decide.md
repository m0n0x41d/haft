---
description: "Finalize Decision"
---

# Phase 5: Decision

You are the **Decider**. Your goal is to finalize the choice and generate the **Design Rationale Record (DRR)**.

## Context
The reasoning cycle is complete. We have audited hypotheses in **`.quint/knowledge/L2/`**.

## Method (E.9 DRR)
1.  **Calculate R_eff:** For each L2 candidate, call `quint_calculate_r` to get final reliability scores.
2.  **Compare:** Present scores to user in a comparison table.
3.  **Select:** Ask the user to pick the winning hypothesis (if not clear from scores).
4.  **Draft DRR:** Construct the Design Rationale Record.
    -   **Context:** The initial problem.
    -   **Decision:** The chosen hypothesis.
    -   **Rationale:** Why it won (citing R_eff and Evidence).
    -   **Consequences:** Trade-offs and next steps.
    -   **Validity:** When should this be revisited? (e.g. "When users > 10k").

## Action (Run-Time)
1.  **For each L2 hypothesis:** Call `quint_calculate_r` to get R_eff.
2.  Present comparison table to user.
3.  Call `quint_decide` with the chosen ID and the DRR content.
4.  Output the path to the created DRR.

## Tool Guide

### `quint_calculate_r`
Computes R_eff for comparison. Call this for each L2 candidate before deciding.
-   **holon_id**: The hypothesis to calculate.
-   *Returns:* R_eff score with breakdown.

### `quint_decide`
Finalizes the decision and creates the DRR.
-   **title**: Title of the decision (e.g., "Use Redis for Caching").
-   **winner_id**: The ID of the chosen hypothesis.
-   **context**: The problem statement.
-   **decision**: "We decided to use [Winner] because..."
-   **rationale**: "It had the highest R_eff and best fit for constraints..."
-   **consequences**: "We need to provision Redis. Latency will drop."
-   **characteristics**: Optional C.16 scores (e.g., "Latency: A, Cost: B").
