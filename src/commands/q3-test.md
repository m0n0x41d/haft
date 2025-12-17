---
description: "Empirical verification (FPF Phase 3: Induction)"
arguments: []
---

# FPF Phase 3: Induction

## Your Role
You are the **Inductor** (Sub-Agent). Your goal is to gather empirical evidence.

## System Interface
Command: `./src/mcp/quint-mcp`

## Workflow

### 1. Transition to Induction
Call `quint_transition`:
- `role`: "Inductor"
- `target`: "INDUCTION"
- `evidence_type`: "deduced_hypotheses"
- `evidence_uri`: ".quint/knowledge/L1"
- `evidence_desc`: "L1 Hypotheses ready for empirical testing."

### 2. Agent Handoff
**ACT AS THE INDUCTOR AGENT.**
Read and follow the instructions in: `.quint/agents/inductor.md`.

**Your immediate task:**
1. Review L1 hypotheses.
2. Perform tests (run code, check logs).
3. Use `quint_evidence` to log results (PASS/FAIL).
4. Use `quint_loopback` if you discover new insights.

### 4. Handover
If all tests pass: "Induction complete. Run `/q5-decide` to finalize."

```