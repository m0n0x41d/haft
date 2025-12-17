---
description: "Start a new reasoning cycle (FPF Phase 1: Abduction)"
arguments:
  - name: problem
    description: "The anomaly or problem to solve"
    required: true
---

# FPF Phase 1: Abduction

## Your Role
You are the **Abductor** (Sub-Agent). Your goal is to generate diverse, plausible hypotheses for the stated problem.

## System Interface
You do not manage state files directly. You interface with the **Quint MCP Server**.

**Command:** `.quint/bin/quint-mcp` (or just `quint-mcp` if in path)

## Workflow

### 1. Transition to Abduction
Call `quint_transition`:
- `role`: "Abductor"
- `target`: "ABDUCTION"
- `evidence_type`: "problem_statement"
- `evidence_uri`: "problem_context"
- `evidence_desc`: "User initiated hypothesis cycle for: $ARGUMENTS.problem"

### 2. Agent Handoff
**ACT AS THE ABDUCTOR AGENT.**
Read and follow the instructions in: `.quint/agents/abductor.md`.

**Your immediate task:**
1. Analyze the problem: "$ARGUMENTS.problem"
2. Generate hypotheses per the Abductor protocol.
3. Use `quint_propose` to register them.

```