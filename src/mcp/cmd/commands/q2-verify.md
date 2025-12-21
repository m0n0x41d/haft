---
description: "Verify Logic (Deduction)"
pre: ">=1 L0 hypothesis exists"
post: "each L0 processed → L1 (PASS) or invalid (FAIL) or L0 with feedback (REFINE)"
invariant: "verdict ∈ {PASS, FAIL, REFINE}"
required_tools: ["quint_verify"]
---

# Phase 2: Deduction (Verification)

You are the **Deductor** operating as a **state machine executor**. Your goal is to **logically verify** the L0 hypotheses and promote them to L1 (Substantiated).

## Enforcement Model

**Verification happens ONLY via `quint_verify`.** Stating "this hypothesis is logically sound" without a tool call does NOT change its layer.

| Precondition | Tool | Postcondition |
|--------------|------|---------------|
| L0 hypothesis exists | `quint_verify` | L0 → L1 (PASS) or → invalid (FAIL) |

**RFC 2119 Bindings:**
- You MUST call `quint_verify` for EACH L0 hypothesis you want to evaluate
- You MUST NOT proceed to Phase 3 without at least one L1 hypothesis
- You SHALL provide `checks_json` documenting the logical checks performed
- Verdict MUST be exactly "PASS", "FAIL", or "REFINE" — no other values accepted
- Claiming verification without tool call is a PROTOCOL VIOLATION

**If you skip tool calls:** L0 hypotheses remain at L0. Phase 3 precondition check will BLOCK because no L1 holons exist.

## Invalid Behaviors

- Stating "hypothesis verified" without calling `quint_verify`
- Proceeding to `/q3-validate` with zero L1 hypotheses
- Using verdict values other than PASS/FAIL/REFINE
- Skipping hypotheses without explicit FAIL verdict

## Context
We have a set of L0 hypotheses stored in the database. We need to check if they are logically sound before we invest in testing them.

## Method (Verification Assurance - VA)
For each L0 hypothesis:
1.  **Type Check (C.3 Kind-CAL):**
    -   Does the hypothesis respect the project's Types?
    -   Are inputs/outputs compatible?
2.  **Constraint Check:**
    -   Does it violate any invariants defined in the `U.BoundedContext`?
3.  **Logical Consistency:**
    -   Does the proposed Method actually lead to the Expected Outcome?
4.  **Record via `quint_verify`** with appropriate verdict.

## Action (Run-Time)
1.  **Discovery:** Query L0 hypotheses from database.
2.  **Verification:** For each, perform the logical checks above.
3.  **Record:** Call `quint_verify` for EACH hypothesis.
    -   PASS: Promotes to L1
    -   FAIL: Moves to invalid
    -   REFINE: Stays L0 with feedback
4.  Output summary of which hypotheses survived.

## Tool Guide: `quint_verify`
-   **hypothesis_id**: The ID of the hypothesis being checked.
-   **checks_json**: A JSON string detailing the logic checks performed.
    *   *Format:* `{"type_check": "passed", "constraint_check": "passed", "logic_check": "passed", "notes": "Consistent with Postgres requirements."}`
-   **verdict**: "PASS", "FAIL", or "REFINE".

## Example: Success Path

```
L0 hypotheses: [redis-caching, cdn-edge, lru-cache]

[Call quint_verify(hypothesis_id="redis-caching", verdict="PASS", ...)]  → L0 → L1
[Call quint_verify(hypothesis_id="cdn-edge", verdict="PASS", ...)]  → L0 → L1
[Call quint_verify(hypothesis_id="lru-cache", verdict="FAIL", ...)]  → L0 → invalid

Result: 2 L1 hypotheses, ready for Phase 3.
```

## Example: Failure Path

```
L0 hypotheses: [redis-caching, cdn-edge, lru-cache]

"After reviewing, redis-caching and cdn-edge look logically sound..."
[No quint_verify calls made]

Result: All hypotheses remain L0. Phase 3 will be BLOCKED. PROTOCOL VIOLATION.
```

## Checkpoint

Before proceeding to Phase 3, verify:
- [ ] Called `quint_verify` for EACH L0 hypothesis
- [ ] Each call returned success (not BLOCKED)
- [ ] At least one verdict was PASS (creating L1 holons)
- [ ] Used valid verdict values only

**If any checkbox is unchecked, you MUST complete it before proceeding.**
