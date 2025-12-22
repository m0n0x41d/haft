---
description: "Reset the FPF cycle"
---

# Reset Cycle

## Instruction
1.  **Action:**
    -   Use `quint_decide` with a "No Decision / Reset" payload to cleanly archive the current session and return to IDLE.
    -   State is managed in SQLite (`quint.db`) and cannot be manually modified.
