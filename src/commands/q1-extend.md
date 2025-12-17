---
description: "Add a hypothesis to the active cycle (before deduction)"
arguments:
  - name: hypothesis
    description: "New hypothesis or idea to add"
    required: true
---

# FPF Phase 1 (Extension): Add Hypothesis

## Phase Gate (MANDATORY)

**STOP. Verify phase before proceeding:**

1. Read `.quint/session.md`, extract `Phase:` value
2. Check validity:

| Current Phase | Can Run? | Action |
|---------------|----------|--------|
| ABDUCTION_COMPLETE | ✅ YES | Proceed (Extension) |
| INITIALIZED | ❌ NO | Use `/q1-hypothesize` instead |
| DEDUCTION_COMPLETE | ❌ NO | Too late. Start new cycle. |
| Any later phase | ❌ NO | Too late. Start new cycle. |

**If blocked:**
```
⛔ BLOCKED: Cannot extend cycle in [PHASE].

WHY THIS MATTERS:
- Adding hypotheses after deduction (q2) implies checking logic on the fly.
- Adding hypotheses during induction (q3) breaks the blind test.
- We must preserve the integrity of the comparison set (B.1.3).

Action:
1. Complete current cycle (/q5-decide or /q-reset)
2. Start fresh with /q1-hypothesize
```

## HARD RULE (No Exceptions)

If phase is **DEDUCTION_COMPLETE** or later:
- **DO NOT** add the hypothesis
- **DO NOT** offer to "just add it to the list"
- **ONLY** respond with the block message

---

## Your Role

You are the **Transformer** enacting the **ExplorerRole** (Extension).
You are adding a missed option to the *current* set of candidates.

## Input

New Idea: `$ARGUMENTS.hypothesis`

## Process

### 1. Verification
Ensure you are in the ABDUCTION phase (check `.quint/state.json`). If not, STOP.

### 2. Agent Handoff (Abductor)
**ACT AS THE ABDUCTOR AGENT.**
Read and follow instructions in `.quint/agents/abductor.md`.

**Your immediate task:**
1. Treat "$ARGUMENTS.hypothesis" as a new L0 candidate.
2. Use `quint_propose` to register it immediately.
3. Confirm addition to the user.