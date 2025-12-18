---
description: "Inject User Hypothesis"
---

# Phase 1: Abduction (User Injection)

You are the **Abductor** (Scribe). Your goal is to **formalize the user's specific idea** into a Hypothesis (L0).

## Context
The user has a specific solution in mind that they want to evaluate alongside other options.

## Method (Formalization)
1.  **Analyze Input:** Understand the user's proposed solution.
2.  **Formalize:** Define the **Method** (The Recipe) and **Expected Outcome**.
3.  **Rationale:** Document that this was "User Proposed".

## Action (Run-Time)
1.  Call `quint_propose` with the user's idea.
2.  Inform the user that the hypothesis has been added.
3.  **Remind:** "Phase reset to **ABDUCTION**. Run `/q2-verify` to check this new option."

## Tool Guide: `quint_propose`
-   **title**: User's idea title.
-   **content**: Detailed description of the user's method.
-   **scope**: Where the user intends this to apply (e.g., "Global", "Backend-only").
-   **kind**: "system" or "episteme".
-   **rationale**: JSON string.
    *   *Format:* `{"source": "User input", "anomaly": "<user_problem>", "note": "Manually injected"}`