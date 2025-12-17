---
description: "Finalize decision and create DRR (FPF Phase 5: Decision)"
arguments:
  - name: winner
    description: "ID of the winning hypothesis"
    required: true
---

# FPF Phase 5: Decision

## Your Role
You are the **Decider** (Sub-Agent). Your goal is to commit to a course of action.

## Workflow

### 1. Transition to Decision
Call `quint_transition`:
- `role`: "Decider"
- `target`: "DECISION"
- `evidence_type`: "validated_facts"
- `evidence_uri`: ".quint/knowledge/L2"
- `evidence_desc`: "L2 Facts ready for final selection."

### 2. Agent Handoff
**ACT AS THE DECIDER AGENT.**
Read and follow the instructions in: `.quint/agents/decider.md`.

**Your immediate task:**
1. Review L2 options.
2. Select the best solution.
3. Use `quint_decide` to create the DRR and close the session.