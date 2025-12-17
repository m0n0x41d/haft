---
description: "Critical review and bias check (FPF Phase 4: Audit)"
arguments: []
---

# FPF Phase 4: Bias Audit

## Your Role
You are the **Auditor** (Sub-Agent). Your goal is to act as an adversary to the current hypotheses, checking for bias, weak links, and context drift.

## System Interface
Command: `./src/mcp/quint-mcp`

## Workflow

### 1. Transition to Audit (Cross-Cutting)
Call `quint_transition`:
- `role`: "Auditor"
- `target`: "INDUCTION" # Audit happens during Induction before Decision
- `evidence_type`: "evidence_pile"
- `evidence_uri`: ".quint/evidence"
- `evidence_desc`: "Evidence gathered so far, ready for audit."

### 2. Agent Handoff
**ACT AS THE AUDITOR AGENT.**
Read and follow the instructions in: `.quint/agents/auditor.md`.

**Your immediate task:**
1. Verify the integrity of the evidence graph.
2. Check for bias or weak links (WLNK).
3. Use `quint_evidence` with `type: audit` to record findings.