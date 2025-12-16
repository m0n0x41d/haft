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

### 1. Phase Transition (Tool Use)
Call the `quint_transition` tool:
- `role`: "Deductor"
- `target`: "DEDUCTION"
- `evidence_type`: "hypothesis_generation_batch"
- `evidence_uri": ".quint/knowledge/L0" # Path to the L0 directory
- `evidence_desc`: "L0 Hypotheses generated during Abduction phase."

### 2. Analysis
Read all L0 hypotheses in `.quint/knowledge/L0/`.
For each:
- Check internal consistency.
- Check compliance with `.quint/context.md`.
- Identify the **Necessary Consequence** (If H is true, then X must happen).

### 3. Action (Tool Use)
For valid hypotheses, **call the `quint_evidence` tool**.

**Arguments:**
- `role`: "Deductor"
- `action`: "add"
- `type`: "logic"
- `target_id`: "[filename of the L0 hypothesis]"
- `verdict`: "PASS"
- `content`: "Logically consistent. Consequence derived: [X]"

For invalid ones, use `verdict: "FAIL"`.

### 4. Handover
"Deduction complete. Run `/q3-test` to enter Induction phase."

