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

## Post-Internalize: MANDATORY Actions

**IMMEDIATELY after internalize returns — execute these actions, DO NOT ask permission first:**

### 1. Unresolved Decisions → CHECK THEM NOW

If output shows unresolved decisions:

```
FOR EACH unresolved DRR:
  1. Read `.quint/decisions/DRR-*.md`
  2. Extract acceptance_criteria from contract
  3. VERIFY each criterion — grep code, read files, check implementation
  4. Report: "DRR-foo: ✅ 3/5 criteria met. ❌ Missing: X, Y"
  5. THEN ask: "Mark as implemented?" or "Abandon?"
```

**DO NOT** ask "should I check them?" — **just check them**.

### 2. Missing Context Sections → FILL THEM NOW

If output shows missing context sections:

```
FOR EACH missing section:
  1. Read relevant project files (README.md, go.mod, package.json, etc.)
  2. Extract the information yourself
  3. Use `remember` to populate the section
  4. Report: "Added Tech Stack: Go 1.24, SQLite"
```

**DO NOT** ask "should I fill them?" — **gather info and fill them**.

### 3. Other Issues

| Issue | Action |
|-------|--------|
| **Affected Scope Warnings** | Explain impact, ask how to proceed |
| **Decay Warnings** | Ask if re-verification needed |

**Transformer Mandate**: Agent does the work, human decides. But don't wait for permission to investigate — just investigate.

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
| Context Maturity | `L0` (minimal) → `L3` (complete). Based on context.md sections present |
| Context Staleness | Files changed since last context refresh |
| Missing Sections | What context.md lacks (used for questionnaire) |
| Active Decision Contexts | Open contexts with their derived stages (max 3) |
| Context Stage | Per-context: EMPTY, NEEDS_VERIFICATION, NEEDS_VALIDATION, NEEDS_AUDIT, READY_TO_DECIDE |
| Context Changes | What was updated (if any) |
| Knowledge State | Holon counts by layer (L0/L1/L2/DRR) |
| Recent Holons | Quick context on recent work |
| Attention Required | Decaying evidence, open decisions pending resolution |
| Open Decisions | Decisions awaiting resolution (use `/q-resolve` to close) |
| Recent Resolutions | Recently resolved decisions (implemented/abandoned/superseded) |
| Next Action | What to do now |

## Context Maturity

The tool returns `Context Maturity` (L0-L3) based on how much project context is available.

| Level | Meaning |
|-------|---------|
| **L0** | Minimal — no resolved DRRs yet |
| **L1** | Basic — some DRR invariants aggregated |
| **L2** | Good — multiple DRRs with invariants and anti-patterns |
| **L3** | Complete — rich knowledge base with custom notes |

**context.md is auto-generated.** Agent reads it but does NOT edit it directly.

- **Tech Stack**: Auto-detected from project files
- **Invariants/Anti-Patterns**: Aggregated from resolved DRRs
- **Custom Notes**: Use remember/overwrite to add

## Context Write Operations

Use these parameters to manage project context:

| Operation | Parameter | Purpose |
|-----------|-----------|---------|
| **Remember** | `remember={category:"...", content:"..."}` | Append fact to category |
| **Forget** | `forget="category"` | Remove category entirely |
| **Overwrite** | `overwrite={category:"...", content:"..."}` | Replace category content |

**Example usage:**
```
quint_internalize(remember={category:"Tech Stack", content:"Go 1.24, SQLite"})
quint_internalize(remember={category:"Notes", content:"Auth uses JWT"})
quint_internalize(overwrite={category:"Notes", content:"Auth uses OAuth2"})
quint_internalize(forget="Notes")
```

**Important:**
- State stored in DB (`context_facts` table)
- context.md regenerated after every write
- Read context via `Read(.quint/context.md)` — no recall parameter needed

If context seems incomplete, agent can:
1. Read README.md, CLAUDE.md directly
2. Ask user clarifying questions in chat
3. Use `remember` to persist learned facts
4. Build knowledge through FPF cycle (hypotheses → decisions → DRRs)

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

## Context Stage ↔ Reasoning Lifecycle

Understanding WHERE you are in the reasoning cycle, not just WHAT to do next.

### The Four Phases

| Phase | Goal | Input | Output |
|-------|------|-------|--------|
| **Explore (Abduction)** | Generate possibilities | Problem/anomaly | Multiple L0 hypotheses |
| **Shape (Deduction)** | Derive testable predictions | L0 hypotheses | L1 hypotheses with predictions |
| **Evidence (Induction)** | Test against reality | L1 + predictions | L2 corroborated claims |
| **Operate** | Monitor deployed decisions | Implemented DRR | Drift alerts, refresh triggers |

### Stage-to-Phase Mapping

| Context Stage | Phase | What's Happening | Next Command |
|---------------|-------|------------------|--------------|
| **EMPTY** | pre-Explore | No hypotheses exist | `/q1-hypothesize` |
| **NEEDS_VERIFICATION** | Explore→Shape | Have L0s, need logical check | `/q2-verify` |
| **NEEDS_VALIDATION** | Shape→Evidence | Have L1s with predictions, need tests | `/q3-validate` |
| **NEEDS_AUDIT** | Evidence | Have L2s, need trust calculation | `/q4-audit` |
| **READY_TO_DECIDE** | pre-Operate | Audited alternatives, ready to choose | `/q5-decide` |
| **(after DRR)** | Operate | Implemented, monitoring for drift | `/q-resolve` |

### Why You Can't Skip Phases

Each phase produces artifacts the next phase needs:

```
Phase 1 (Abduction) → L0 hypotheses with falsifiability criteria
                            ↓
Phase 2 (Deduction) → L1 hypotheses with testable predictions
                            ↓
Phase 3 (Induction) → L2 hypotheses with test results linked to predictions
                            ↓
Phase 4 (Audit)     → R_eff scores from WLNK analysis
                            ↓
Phase 5 (Decision)  → DRR with rationale and contract
```

**If you skip Phase 2:** No predictions exist → Phase 3 tests are random → confirmation bias.
**If you skip Phase 3:** No empirical evidence → Phase 4 has nothing to audit → R_eff=0.
**If you skip Phase 4:** No trust calculation → Phase 5 decision is ungrounded.

### Quick Reference: What Each Phase Produces

| Phase | Tool | Creates | Used By |
|-------|------|---------|---------|
| 1 | `quint_propose` | L0 with `falsifiability` in rationale | Phase 2 |
| 2 | `quint_verify` | L1 with `predictions` array | Phase 3 |
| 3 | `quint_test` | L2 with `tests_prediction` links | Phase 4 |
| 4 | `quint_audit` | R_eff scores, risk analysis | Phase 5 |
| 5 | `quint_decide` | DRR with contract | Implementation |

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
