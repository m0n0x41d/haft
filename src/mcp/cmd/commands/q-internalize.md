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
3. **Current phase** - where you are in the ADI cycle

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
| Status | `INITIALIZED` (new), `UPDATED` (refreshed), `CURRENT` (no changes) |
| Phase | Current FSM phase: IDLE, ABDUCTION, DEDUCTION, INDUCTION, AUDIT, DECISION |
| Role | Expected role: Observer, Abductor, Deductor, Inductor, Auditor, Decider |
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
    |   +-> Proceed to /q1-hypothesize (IDLE -> ABDUCTION)
    |
    +-> Status: UPDATED
    |   +-> Review changes, then continue current phase
    |
    +-> Status: CURRENT
        +-> Phase: IDLE -> /q1-hypothesize
        +-> Phase: ABDUCTION -> /q1-hypothesize or /q2-verify
        +-> Phase: DEDUCTION -> /q2-verify or /q3-validate
        +-> Phase: INDUCTION -> /q3-validate or /q4-audit
        +-> Phase: AUDIT -> /q4-audit or /q5-decide
        +-> Phase: DECISION -> /q5-decide
```

## Examples

### Fresh Project

```
> quint_internalize

=== QUINT INTERNALIZE ===

Status: INITIALIZED
Phase: ABDUCTION
Role: Abductor
Context: default

Context Changes:
  - Created .quint/ structure
  - Auto-generated context from project analysis

Knowledge State:
  L0 (Conjecture): 0
  L1 (Substantiated): 0
  L2 (Corroborated): 0

Next Action: -> /q1-hypothesize to generate hypotheses
```

### Continuing Session

```
> quint_internalize

=== QUINT INTERNALIZE ===

Status: CURRENT
Phase: DEDUCTION
Role: Deductor
Context: default

Knowledge State:
  L0 (Conjecture): 3
  L1 (Substantiated): 2
  L2 (Corroborated): 0
  DRRs: 2

Recent Holons:
  - jwt-auth [L1] R=0.45 - 2h ago
  - session-cookies [L0] R=0.00 - 3h ago
  - oauth2-flow [L1] R=0.60 - 1h ago

Open Decisions (awaiting resolution):
  - DRR-cache-redis: Redis Caching (1w ago)

Recent Resolutions:
  - DRR-auth-jwt: JWT Authentication [implemented] 2d ago

Next Action: -> 2 L1 ready for /q3-validate
```

### Stale Context Detected

```
> quint_internalize

=== QUINT INTERNALIZE ===

Status: UPDATED
Phase: ABDUCTION
Role: Abductor
Context: default

Context Changes:
  - go.mod modified since last context update

Knowledge State:
  L0 (Conjecture): 2
  L1 (Substantiated): 1
  L2 (Corroborated): 0

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
