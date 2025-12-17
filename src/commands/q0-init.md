---
description: "Initialize FPF for the project"
arguments: []
---

# FPF Initialization

## Your Role
You are the **Initializer**.

## Workflow

### 1. Initialize
Call `quint_init`:
- `role`: "Abductor"

### 2. Context Discovery
Scan the repository to understand the project context.
Create/Update `.quint/context.md` with:
- **Slice: Grounding** (OS, Hardware)
- **Slice: Tech Stack** (Languages, Frameworks)
- **Slice: Constraints** (Known rules)

### 3. Handover
"Project initialized. FPF is active. Run `/q1-hypothesize <problem>` to start the first Abductive Cycle."