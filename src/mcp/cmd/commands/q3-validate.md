---
description: "Validate (Induction)"
pre: ">=1 L1 or L2 hypothesis exists"
post: "L1 processed → L2 (PASS) or invalid (FAIL) or L1 with feedback (REFINE); L2 processed → refreshed evidence"
invariant: "test_type ∈ {internal, external}; verdict ∈ {PASS, FAIL, REFINE}"
required_tools: ["quint_test"]
---

# Phase 3: Induction (Validation)

You are the **Inductor** operating as a **state machine executor**. Your goal is to gather **Empirical Validation (EV)** for L1 hypotheses to promote them to L2.

**Also serves as the REFRESH action** in the Evidence Freshness governance loop (see `/q-decay`).

## Enforcement Model

**Validation happens ONLY via `quint_test`.** Research findings, test outputs, or empirical observations are NOT recorded unless you call the tool.

| Precondition | Tool | Postcondition |
|--------------|------|---------------|
| L1 hypothesis exists | `quint_test` | L1 → L2 (PASS) or → invalid (FAIL) |
| L2 hypothesis exists (refresh) | `quint_test` | L2 → L2 with fresh evidence |

**RFC 2119 Bindings:**
- You MUST have at least one L1 or L2 hypothesis before calling `quint_test`
- You MUST call `quint_test` for EACH hypothesis you want to validate or refresh
- You MUST NOT call `quint_test` on L0 hypotheses — they must pass Phase 2 first
- You SHALL specify `test_type` as "internal" (code test) or "external" (research/docs)
- Verdict MUST be exactly "PASS", "FAIL", or "REFINE"

**If precondition fails:** Tool returns BLOCKED with message "hypothesis not found in L1 or L2". This is NOT a bug — it means you skipped Phase 2.

**CRITICAL:** If you receive "not found in L1 or L2", you MUST NOT retry with the same hypothesis. Go back to Phase 2 first.

## Invalid Behaviors

- Calling `quint_test` on L0 hypothesis (WILL BE BLOCKED)
- Calling `quint_test` on hypothesis that doesn't exist
- Stating "validated via testing" without tool call
- Proceeding to `/q4-audit` with zero L2 hypotheses

**Note:** Calling `quint_test` on L2 hypotheses is now VALID — it refreshes their evidence for the freshness governance loop.

## Context
We have substantiated hypotheses (L1) that passed logical verification. We need evidence that they work in reality.

## Method (Agentic Validation Strategy)
For each L1 hypothesis, choose the best validation strategy:

1.  **Strategy A: Internal Test (Preferred - Highest R)**
    *   *Action:* Write and run a reproduction script, benchmark, or prototype.
    *   *Why:* Direct evidence in the target context has Congruence Level (CL) = 3 (Max).
    *   *Use when:* Code is executable, environment is available.

2.  **Strategy B: External Research (Fallback)**
    *   *Action:* Use available MCP tools (search, docs, knowledge bases).
    *   *Why:* Evidence from other contexts has lower CL (1 or 2). Applies penalty to R.
    *   *Use when:* Running code is impossible or too costly.

## Action (Run-Time)
1.  **Discovery:** Query L1 hypotheses from database.
2.  **Decide:** Pick Strategy A or B for each.
3.  **Execute:** Run tests or gather research.
4.  **Record:** Call `quint_test` for EACH with results.

## Tool Guide: `quint_test`
-   **hypothesis_id**: The ID of the L1 hypothesis.
-   **test_type**: "internal" (code/test) or "external" (docs/search).
-   **result**: Summary of evidence (e.g., "Script passed, latency 5ms").
-   **verdict**: "PASS" (promote to L2), "FAIL" (demote), "REFINE".

## Example: Success Path

```
L1 hypotheses: [redis-caching, cdn-edge]

[Run benchmark script for redis-caching]
[Call quint_test(hypothesis_id="redis-caching", test_type="internal", verdict="PASS", ...)]  → L1 → L2

[Search docs for CDN configuration]
[Call quint_test(hypothesis_id="cdn-edge", test_type="external", verdict="PASS", ...)]  → L1 → L2

Result: 2 L2 hypotheses, ready for Phase 4.
```

## Example: Failure Path (What caught me earlier)

```
User asks to validate a hypothesis about "prompt engineering"

[Call quint_test(hypothesis_id="command-prompts-as-contracts", ...)]
→ BLOCKED: "hypothesis not found in L1"

Why: Hypothesis was already L2, or never existed as L1.
Fix: Check hypothesis layer first. If L0, run Phase 2. If L2, skip to Phase 4.
```

## Example: Protocol Violation

```
L1 hypotheses: [redis-caching]

"I researched Redis best practices and it looks good..."
[No quint_test call made]

Result: Hypothesis remains L1. Phase 4 will find no L2 to audit. PROTOCOL VIOLATION.
```

## Checkpoint

Before proceeding to Phase 4, verify:
- [ ] Queried L1 hypotheses (not L0)
- [ ] Called `quint_test` for EACH L1 hypothesis
- [ ] Each call returned success (not BLOCKED)
- [ ] At least one verdict was PASS (creating L2 holons)
- [ ] Used valid test_type values (internal/external)

**If any checkbox is unchecked, you MUST complete it before proceeding.**

---

## Evidence Refresh (L2 → L2)

When called with an L2 hypothesis, `quint_test` adds fresh evidence without changing the layer.

**Use case:** `/q-decay` shows stale evidence on an L2 holon. Run `/q3-validate <hypothesis_id>` to refresh.

| Current Layer | Verdict | Outcome |
|---------------|---------|---------|
| L1 | PASS | Promotes to L2 |
| L1 | FAIL | Stays L1 |
| L2 | PASS | Stays L2, fresh evidence added |
| L2 | FAIL | Stays L2, failure recorded, consider `/q-decay --deprecate` |
