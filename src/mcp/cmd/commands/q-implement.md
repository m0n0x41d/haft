---
description: "Transform DRR into implementation directive"
pre: "DRR exists with implementation contract"
post: "Agent has structured implementation task with constraints"
invariant: "Contract becomes executable specification via agent programming"
required_tools: ["quint_implement"]
---

# Implement Decision

You are the **Implementer**. Transform finalized decisions into code.

## Purpose

`quint_implement` doesn't create a plan — it **programs your internal planning capabilities** with the decision's contract as specification.

The tool returns an **implementation directive** containing:
1. Invariants that MUST be true in your implementation
2. Anti-patterns to verify against in final verification
3. Acceptance criteria to check before resolving
4. Inherited constraints from dependency chain (WLNK for constraints)

## When to Use

| Situation | Action |
|-----------|--------|
| DRR finalized, ready to implement | Call `quint_implement` |
| Resuming interrupted implementation | Re-call to restore context |
| Want to see what constraints apply | Call to preview contract |

## Action

```
quint_implement(decision_id="<drr-id>")
```

The tool returns an implementation directive — follow it using your native TODO/planning features.

## Flow

```
quint_implement(decision_id)
    ↓
[Implementation Directive with invariants + constraints]
    ↓
Use YOUR planning capabilities (TodoWrite, etc.) to execute
    ↓
Final verification: check all constraints in your todo list
    ↓
quint_resolve(decision_id, "implemented", criteria_verified=true)
```

## Example Output

```markdown
# IMPLEMENTATION DIRECTIVE

## Task
Implement: **Redis Caching Layer**
Decision: drr-cache-redis
Scope: internal/cache/*.go, internal/repository/*.go

## Instructions

Using your internal TODO/planning capabilities, implement this task.

If project context is insufficient, conduct preliminary investigation first.

## Invariants to Implement

These MUST be true in your implementation:

### This decision:
1. Cache misses must fall through to DB transparently
2. TTL must be configurable per entity type
3. Cache invalidation on write operations

### Inherited from drr-jwt-auth:
- Tokens must be stateless (no server-side session storage)
- All token operations must be logged to audit trail

## Final Verification

Your LAST todo items must verify these constraints were NOT violated:

### This decision:
- [ ] No cache-aside pattern in business logic
- [ ] No hardcoded TTL values
- [ ] No silent failures

### Inherited from drr-jwt-auth:
- [ ] No server-side token storage introduced
- [ ] Token operations still logged after cache layer added

## Acceptance Criteria

Before calling quint_resolve, verify:

- [ ] Cache hit returns data without DB call
- [ ] Cache miss fetches from DB and populates cache
- [ ] Write operations invalidate relevant cache keys

---
When complete: `quint_resolve drr-cache-redis implemented criteria_verified=true`
```

## Constraint Inheritance (WLNK)

When a DRR depends on other decisions, their constraints are inherited:

```
DRR-jwt-auth
  invariants: ["tokens stateless", "audit all operations"]
      ↓ depends_on
DRR-cache-redis
  invariants: ["configurable TTL", "transparent fallthrough"]
```

This ensures WLNK applies not just to reliability (R_eff) but also to constraints — violating an upstream invariant breaks the foundation your decision builds on.

## Why This Works

You have internal planning capabilities. This tool doesn't replace them — it **programs them** with the right constraints from the decision record.

```
Traditional: Tool generates plan → Agent executes blindly
quint_implement: Tool injects constraints → Agent uses ITS OWN planner
```

The agent remains autonomous while being constrained by the decision contract.

## Checkpoint

Before implementing:
- [ ] DRR exists and has a contract
- [ ] Reviewed the invariants and anti-patterns
- [ ] Understand the acceptance criteria
- [ ] Ready to use your TODO/planning capabilities

After implementing:
- [ ] All invariants are satisfied
- [ ] No anti-patterns were introduced
- [ ] All acceptance criteria pass
- [ ] Ready to call `quint_resolve ... implemented criteria_verified=true`
