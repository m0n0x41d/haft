---
description: "Synchronize agent understanding with project and Quint state"
pre: "none"
post: "agent has current context, state initialized if needed"
invariant: "allowed in any phase, safe to call anytime"
required_tools: ["quint_internalize"]
---

# Internalize

You are the **Observer**. This is your entry point for every session.

## Purpose

Bring your mental model in sync with:
1. **Project structure** - code, dependencies, architecture
2. **Knowledge base** - hypotheses, evidence, decisions
3. **Current stage** - where each decision context is in the ADI cycle

## When to Use

| Situation | Action |
|-----------|--------|
| Session start | **Always** call first |
| After interruption | Re-sync after context loss |
| After project changes | When dependencies or structure changed |
| Before major decisions | Verify current understanding |

## Action

Call `quint_internalize`. The tool handles everything:

1. **Initialize** (if needed): Creates `.quint/` structure, analyzes project, generates context
2. **Update** (if stale): Detects project changes, refreshes context
3. **Load**: Returns current knowledge state and phase
4. **Surface issues**: Decaying evidence, failing tests
5. **Guide**: Phase-appropriate next action

## Output Fields

| Field | Meaning |
|-------|---------|
| Status | `INITIALIZED` (new), `UPDATED` (refreshed), `READY` (no changes) |
| Active Decision Contexts | Open contexts with their derived stages (max 3) |
| Context Stage | Per-context: EMPTY, NEEDS_VERIFICATION, NEEDS_VALIDATION, NEEDS_AUDIT, READY_TO_DECIDE |
| Context Changes | What was updated (if any) |
| Knowledge State | Holon counts by layer (L0/L1/L2/DRR) |
| Recent Holons | Quick context on recent work |
| Attention Required | Decaying evidence, open decisions pending resolution |
| Open Decisions | Decisions awaiting resolution (use `/q-resolve` to close) |
| Recent Resolutions | Recently resolved decisions (implemented/abandoned/superseded) |
| Next Action | What to do now |

## Flow After Internalize

```
quint_internalize
    |
    +-> Status: INITIALIZED
    |   +-> No contexts exist -> /q1-hypothesize to start
    |
    +-> Status: UPDATED
    |   +-> Review changes, continue with active contexts
    |
    +-> Status: READY
        +-> For each active context, stage determines next action:
            +-> EMPTY -> /q1-hypothesize (add hypotheses)
            +-> NEEDS_VERIFICATION -> /q2-verify (verify L0)
            +-> NEEDS_VALIDATION -> /q3-validate (test L1)
            +-> NEEDS_AUDIT -> /q4-audit (audit L2)
            +-> READY_TO_DECIDE -> /q5-decide (finalize)
```

## Examples

### Fresh Project

```
> quint_internalize

=== QUINT INTERNALIZE ===

Status: INITIALIZED
Context: default

Active Decision Contexts (0/3):
  No active contexts.

Context Changes:
  - Created .quint/ structure
  - Auto-generated context from project analysis

Next Action: -> /q1-hypothesize to create first decision context
```

### Continuing Session

```
> quint_internalize

=== QUINT INTERNALIZE ===

Status: READY
Context: default

Active Decision Contexts (2/3):
  [dc-auth-strategy] "Auth Strategy"
    Stage: NEEDS_VERIFICATION
    Hypotheses: 3 (L0: 2, L1: 1)
    Next: /q2-verify

  [dc-caching] "Caching Strategy"
    Stage: READY_TO_DECIDE
    Hypotheses: 2 (L2: 2, audited)
    Next: /q5-decide

Recent Holons:
  - jwt-auth [L1] R=0.45 - 2h ago
  - session-cookies [L0] R=0.00 - 3h ago
  - oauth2-flow [L1] R=0.60 - 1h ago

Open Decisions (awaiting resolution):
  - DRR-cache-redis: Redis Caching (1w ago)

Recent Resolutions:
  - DRR-auth-jwt: JWT Authentication [implemented] 2d ago

Next Action: -> dc-caching ready for /q5-decide
```

### Stale Context Detected

```
> quint_internalize

=== QUINT INTERNALIZE ===

Status: UPDATED
Context: default

Active Decision Contexts (1/3):
  [dc-api-design] "API Design"
    Stage: NEEDS_VERIFICATION
    Hypotheses: 2 (L0: 2)
    Next: /q2-verify

Context Changes:
  - go.mod modified since last context update

Next Action: -> 2 L0 ready for /q2-verify
```

## Search Prior Work

After internalizing, use `quint_search` to find relevant prior work:

```
quint_search(query="authentication")
quint_search(query="caching", layer_filter="L2")
quint_search(query="database", scope="holons")
quint_search(query="*", status_filter="open")       # Find unresolved decisions
quint_search(query="auth", status_filter="implemented")  # Find implemented decisions
```

## Why No Automatic Context Injection?

Unlike some memory systems that automatically inject context at session start, Quint requires explicit `quint_internalize` calls. This is intentional:

1. **Agent as Transformer**: FPF requires identifiable actors for all state changes. "The system" automatically injecting context violates this principle.

2. **Agent Controls Context**: Agent decides when to load context and can see exactly what it receives.

3. **No Hidden Processing**: No background workers compressing or transforming data without agent awareness.

4. **Audit Trail**: Every context load is an explicit tool call, visible in logs.

The agent is always in control. Call `quint_internalize` to orient yourself.
