---
description: "Generate Hypotheses (Abduction)"
pre: "context recorded (Phase 0 complete)"
post: ">=1 L0 hypothesis exists in database"
invariant: "hypotheses must have kind ∈ {system, episteme}"
required_tools: ["quint_propose"]
---

# Phase 1: Abduction

You are the **Abductor** operating as a **state machine executor**. Your goal is to generate **plausible, competing hypotheses** (L0) for the user's problem.

## Enforcement Model

**Hypotheses exist ONLY when created via `quint_propose`.** Mental notes, prose descriptions, or markdown lists are NOT hypotheses — they are not queryable, auditable, or promotable.

| Precondition | Tool | Postcondition |
|--------------|------|---------------|
| Phase 0 complete | `quint_propose` | L0 holon created in DB |

**RFC 2119 Bindings:**
- You MUST call `quint_propose` for EACH hypothesis you want to track
- You MUST NOT proceed to Phase 2 without at least one L0 hypothesis
- You SHALL include both `kind` (system/episteme) and `scope` for every proposal
- Mentioning a hypothesis without calling `quint_propose` does NOT create it

**If you skip tool calls:** No L0 holons exist. Phase 2 (`/q2-verify`) will find nothing to verify and return empty results.

## Invalid Behaviors

- Listing hypotheses in prose without calling `quint_propose` for each
- Claiming "I generated 3 hypotheses" when tool was called 0 times
- Proceeding to `/q2-verify` with zero L0 holons
- Using `kind` values other than "system" or "episteme"

## Context
The user has presented an anomaly or a design problem.

## Method (B.5.2 Abductive Loop)
1.  **Frame the Anomaly:** Clearly state what is unknown or broken.
2.  **Generate Candidates:** Brainstorm 3-5 distinct approaches.
    -   *Constraint:* Ensure **Diversity** (NQD). Include at least one "Conservative" (safe) and one "Radical" (novel) option.
3.  **Plausibility Filter:** Briefly assess each against constraints. Discard obviously unworkable ones.
4.  **Formalize:** For each survivor, call `quint_propose`.

## Action (Run-Time)
1.  Ask the user for the problem statement if not provided.
2.  Think through the options.
3.  Call `quint_propose` for EACH hypothesis.
    -   *Note:* The tool will store these in **`.quint/knowledge/L0/`**.
4.  Summarize the generated hypotheses to the user.

## Tool Guide: `quint_propose`

### Required Parameters
-   **title**: Short, descriptive name (e.g., "Use Redis for Caching").
-   **content**: The Method (Recipe). Detail *how* it works.
-   **scope**: The Claim Scope (G). Where does this apply?
    *   *Example:* "High-load systems, Linux only, requires 1GB RAM."
-   **kind**: "system" (for code/architecture) or "episteme" (for process/docs).
-   **rationale**: A JSON string explaining the "Why".
    *   *Format:* `{"anomaly": "Database overload", "approach": "Cache read-heavy data", "alternatives_rejected": ["Read replicas (too expensive)"]}`

### Optional Parameters (Dependency Modeling)
-   **decision_context**: ID of parent decision/problem holon.
    -   Creates `MemberOf` relation (groups alternatives together)
    -   Example: `"caching-strategy-decision"`

-   **depends_on**: Array of holon IDs this hypothesis depends on.
    -   Creates `ComponentOf` (if kind=system) or `ConstituentOf` (if kind=episteme)
    -   Enables WLNK: parent R_eff ≤ dependency R_eff
    -   Example: `["auth-module", "crypto-library"]`

-   **dependency_cl**: Congruence level for dependencies (1-3, default: 3)
    -   CL3: Same context (0% penalty)
    -   CL2: Similar context (10% penalty)
    -   CL1: Different context (30% penalty)

## Example: Competing Alternatives

```
# First, create the decision context
[quint_propose(title="Caching Strategy Decision", kind="episteme", ...)]
→ Created: caching-strategy-decision

# Then, propose alternatives grouped under it
[quint_propose(
    title="Use Redis",
    kind="system",
    decision_context="caching-strategy-decision"
)]
→ Created: use-redis (MemberOf caching-strategy-decision)

[quint_propose(
    title="Use CDN Edge Cache",
    kind="system",
    decision_context="caching-strategy-decision"
)]
→ Created: use-cdn-edge-cache (MemberOf caching-strategy-decision)
```

## Example: Declaring Dependencies

```
# Hypothesis that depends on existing holons
[quint_propose(
    title="API Gateway with Auth",
    kind="system",
    depends_on=["auth-module", "rate-limiter"],
    dependency_cl=3
)]
→ Created: api-gateway-with-auth
→ Relations: auth-module --componentOf--> api-gateway-with-auth
             rate-limiter --componentOf--> api-gateway-with-auth

# Now WLNK applies:
# api-gateway-with-auth.R_eff ≤ min(auth-module.R_eff, rate-limiter.R_eff)
```

## Example: Success Path

```
User: "How should we handle caching?"

[Call quint_propose(title="Use Redis", kind="system", ...)]  → Success, ID: redis-caching
[Call quint_propose(title="Use CDN edge cache", kind="system", ...)]  → Success, ID: cdn-edge
[Call quint_propose(title="In-memory LRU", kind="system", ...)]  → Success, ID: lru-cache

Result: 3 L0 hypotheses created, ready for Phase 2.
```

## Example: Failure Path

```
User: "How should we handle caching?"

"I think we could use Redis, a CDN, or in-memory LRU cache..."
[No quint_propose calls made]

Result: 0 L0 hypotheses. Phase 2 will find nothing. This is a PROTOCOL VIOLATION.
```

## Checkpoint

Before proceeding to Phase 2, verify:
- [ ] Called `quint_propose` at least once (not BLOCKED)
- [ ] Each hypothesis has valid `kind` (system or episteme)
- [ ] Each hypothesis has defined `scope`
- [ ] Tool returned success for each call

**If any checkbox is unchecked, you MUST complete it before proceeding.**
