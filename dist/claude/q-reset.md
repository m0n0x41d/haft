---
description: "Hard reset of the current FPF cycle"
arguments: []
---

# FPF Reset

## Warning
This will clear the current session state.

## Workflow
```bash
./src/mcp/quint-mcp -action init
```
(Re-initializing resets the FSM to ABDUCTION or IDLE depending on implementation logic, effectively clearing the current active loop).