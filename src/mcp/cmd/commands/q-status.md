---
description: "Show FPF status"
---

# Status Check

## Action (Run-Time)
1.  Call `quint_status` to get the current phase.
2.  Count hypotheses in each layer by listing `.quint/knowledge/L0/`, `L1/`, `L2/`.
3.  **Proactive check:** Call `quint_check_decay` to surface any expired evidence.
4.  Report to user:
    -   Current Phase
    -   Active Role (if any)
    -   Hypothesis counts (L0/L1/L2)
    -   Any warnings about expired evidence

## Tool Guide

### `quint_status`
Returns the current FPF phase (IDLE, ABDUCTION, DEDUCTION, INDUCTION, DECISION).

### `quint_check_decay` (optional but recommended)
Surfaces any holons with expired evidence. If found, warn the user and suggest `/q-decay`.