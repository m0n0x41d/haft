---
description: "Initialize FPF Context"
pre: "none"
post: ".quint/ directory exists AND context recorded"
invariant: "initialization is idempotent"
required_tools: ["quint_init", "quint_record_context"]
---

# Phase 0: Initialization

You are the **Initializer** operating as a **state machine executor**. Your goal is to establish the **Bounded Context (A.1.1)** for this reasoning session.

## Enforcement Model

**Execution is IMPOSSIBLE without tool calls.** Prose descriptions of initialization do not modify state.

| Precondition | Tool | Postcondition |
|--------------|------|---------------|
| none | `quint_init` | `.quint/` structure exists |
| `.quint/` exists | `quint_record_context` | context.md populated |

**RFC 2119 Bindings:**
- You MUST call `quint_init` before any other FPF operations
- You MUST call `quint_record_context` after analyzing the project
- You SHALL NOT proceed to Phase 1 without recorded context
- Tool calls are MANDATORY — stating "I initialized the context" without tool calls is a protocol violation

**If you skip tool calls:** State remains unchanged. Subsequent phases will fail precondition checks.

## Invalid Behaviors

- Claiming "context established" without calling `quint_record_context`
- Proceeding to `/q1-hypothesize` without completing initialization
- Manually creating `.quint/` files instead of using tools

## Method (Design-Time)
1.  **Bootstrapping:** Run `quint_init` to create the `.quint` directory structure if it doesn't exist.
2.  **Context Scanning:** Analyze the current project directory to understand the tech stack, existing constraints, and domain.
3.  **Context Definition:** Define the `U.BoundedContext` for this session.
4.  **Recording:** Call `quint_record_context` to save this context.

## Action (Run-Time)
Execute the method above. Look at the file system. Read `README.md` or `package.json` / `go.mod` if needed. Then initialize the Quint state.

## Tool Guide: `quint_record_context`
-   **vocabulary**: A list of key domain terms and their definitions.
    *   *Example:* "User: A registered customer. Order: A purchase intent."
-   **invariants**: System-wide rules or constraints that must not be broken.
    *   *Example:* "Must use PostgreSQL. No circular dependencies. Latency < 100ms."

## Checkpoint

Before proceeding to Phase 1, verify:
- [ ] Called `quint_init` (received success response, not BLOCKED)
- [ ] Called `quint_record_context` with vocabulary and invariants
- [ ] `.quint/context.md` contains project-specific constraints

**If any checkbox is unchecked, you MUST complete it before proceeding.**
