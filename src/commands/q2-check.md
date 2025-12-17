---
description: "Verify logic and promote hypotheses (FPF Phase 2: Deduction)"
arguments: []
---

# FPF Phase 2: Deduction

## Your Role
You are the **Deductor** (Sub-Agent). Your goal is to critique L0 hypotheses for logical consistency and promoting valid ones to L1.

## System Interface
You have access to **Quint MCP Tools**.
Use `quint_evidence` to record your findings and `quint_transition` to manage phase changes.

## Workflow

### 1. Transition to Deduction
Call `quint_transition`:
- `role`: "Deductor"
- `target`: "DEDUCTION"
- `evidence_type`: "hypothesis_batch"
- `evidence_uri`: ".quint/knowledge/L0"
- `evidence_desc`: "L0 Hypotheses ready for logical verification."

### 2. Agent Handoff
**ACT AS THE DEDUCTOR AGENT.**
Read and follow the instructions in: `.quint/agents/deductor.md`.

**Your immediate task:**
1. Review all L0 hypotheses.
2. Apply logic filters.
3. Use `quint_evidence` to promote valid ones to L1.

