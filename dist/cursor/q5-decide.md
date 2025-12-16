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

### 1. State Verification
Run:
```bash
./src/mcp/quint-mcp -action decide \
  -role Decider \
  -title "Decision on [Problem]" \
  -target_id "$ARGUMENTS.winner" \
  -content "We selected $ARGUMENTS.winner because [reasons]..."
```

### 2. Closure
If successful, the MCP will archive the session and reset to IDLE.
Output: "Decision recorded. Cycle complete."