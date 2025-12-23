---
description: "Finalize Decision"
pre: ">=1 L2 hypothesis exists with audit results"
post: "DRR created and persisted"
invariant: "human selects winner; agent documents rationale"
required_tools: ["quint_calculate_r", "quint_decide"]
---

# Phase 5: Decision

You are the **Decider** operating as a **state machine executor**. Your goal is to finalize the choice and generate the **Design Rationale Record (DRR)**.

## Enforcement Model

**Decisions are recorded ONLY via `quint_decide`.** Stating "we decided to use X" without a tool call does NOT create a DRR — the decision is not documented, not auditable, not queryable.

| Precondition | Tool | Postcondition |
|--------------|------|---------------|
| L2 hypothesis exists | `quint_calculate_r` | Final R_eff for comparison |
| Winner selected by human | `quint_decide` | DRR created in `.quint/decisions/` |

**RFC 2119 Bindings:**
- You MUST have at least one audited L2 hypothesis before deciding
- You MUST call `quint_calculate_r` for each candidate to present comparison
- You MUST present comparison to user and GET USER APPROVAL before finalizing
- You MUST call `quint_decide` to create the DRR
- You SHALL NOT select the winner autonomously — this is the **Transformer Mandate**
- The human decides; you document

**If precondition fails:** `quint_decide` will be BLOCKED if no L2 hypotheses exist.

**CRITICAL: Transformer Mandate**
A system cannot transform itself. You (Claude) generate options with evidence. The human decides. Making architectural choices autonomously is a PROTOCOL VIOLATION.

## Invalid Behaviors

- Selecting winner without user approval
- Calling `quint_decide` without presenting comparison first
- Stating "we decided X" without tool call
- Making the decision for the user ("I recommend X, so I'll proceed with X")
- Proceeding with implementation before DRR is created

## Context
The reasoning cycle is complete. We have audited hypotheses in L2.

## Method (E.9 DRR)
1.  **Calculate R_eff:** For each L2 candidate, call `quint_calculate_r`.
2.  **Compare:** Present scores to user in comparison table.
3.  **Select:** ASK user to pick the winning hypothesis.
4.  **Draft DRR:** After user confirms, construct the Design Rationale Record:
    -   **Context:** The initial problem.
    -   **Decision:** The chosen hypothesis.
    -   **Rationale:** Why it won (citing R_eff and evidence).
    -   **Consequences:** Trade-offs and next steps.
    -   **Validity:** When should this be revisited?

## Action (Run-Time)
1.  **For each L2 hypothesis:** Call `quint_calculate_r` to get R_eff.
2.  Present comparison table to user.
3.  **WAIT for user to select winner.**
4.  Call `quint_decide` with the chosen ID and DRR content.
5.  Output the path to the created DRR.

## Tool Guide

### `quint_calculate_r`
Computes R_eff for comparison.
-   **holon_id**: The hypothesis to calculate.
-   *Returns:* R_eff score with breakdown.

### `quint_decide`
Finalizes the decision and creates the DRR.
-   **title**: Title of the decision (e.g., "Use Redis for Caching").
-   **winner_id**: The ID of the chosen hypothesis.
-   **rejected_ids**: Array of IDs of rejected L2 alternatives (creates `rejects` relations).
-   **context**: The problem statement.
-   **decision**: "We decided to use [Winner] because..."
-   **rationale**: "It had the highest R_eff and best fit for constraints..."
-   **consequences**: "We need to provision Redis. Latency will drop."
-   **characteristics**: Optional C.16 scores.

## Example: Success Path

```
L2 hypotheses: [redis-caching, cdn-edge]

[Call quint_calculate_r for each]

Presenting comparison:
| Hypothesis | R_eff | Weakest Link |
|------------|-------|--------------|
| redis-caching | 0.85 | internal test |
| cdn-edge | 0.72 | external docs |

"Which hypothesis should we proceed with?"

[User responds: "redis-caching"]

[Call quint_decide(
    title="Use Redis for Caching",
    winner_id="redis-caching",
    rejected_ids=["cdn-edge"],
    context="...",
    decision="...",
    rationale="...",
    consequences="..."
)]
→ DRR created at .quint/decisions/DRR-XXXX-use-redis-for-caching.md
→ Relations created:
  - DRR --selects--> redis-caching
  - DRR --rejects--> cdn-edge

Result: Decision recorded with full audit trail. Ready for implementation.
```

## Example: Failure Path (Transformer Mandate Violation)

```
L2 hypotheses: [redis-caching, cdn-edge]

"Redis has higher R_eff, so I'll go ahead and implement that..."
[No quint_decide call, no user confirmation]

Result: PROTOCOL VIOLATION. Agent made autonomous architectural decision.
The human must select. You document.
```

## Checkpoint

Before proceeding to implementation, verify:
- [ ] Called `quint_calculate_r` for each L2 hypothesis
- [ ] Presented comparison table to user
- [ ] User explicitly selected the winner
- [ ] Called `quint_decide` with user's choice
- [ ] DRR file created successfully

**If any checkbox is unchecked, you MUST complete it before proceeding.**

## After Implementation

Once the decision is implemented (or abandoned/superseded), close the lifecycle with `/q-resolve`:

```
quint_resolve(
    decision_id="DRR-auth-jwt",
    resolution="implemented",
    reference="pr:42"
)
```

This creates evidence linking the plan (DRR) to reality (implementation). Use `quint_internalize` to see open decisions awaiting resolution.
