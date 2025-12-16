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

### 2. State Verification
Run:
```bash
.quint/bin/quint-core -action check -role Abductor
```
If this fails, STOP. Report the error.

### 2. Context Loading
Read `.quint/context.md` and `.quint/knowledge/L2` to ground your abduction.

### 3. Hypothesis Generation (Mental Sandbox)
Think about the problem: "$ARGUMENTS.problem"
Generate 3-5 hypotheses covering:
- **Conservative** (Low risk, proven)
- **Innovative** (High reward, novel)
- **Minimal** (Fastest path)

### 4. Persistence (Tool Use)
For EACH valid hypothesis, execute:

```bash
./src/mcp/quint-mcp -action propose \
  -role Abductor \
  -title "H1: [Title]" \
  -content "..."
```

**Content Format (Markdown body for the flag):**
```markdown
# [Title]
**Type:** [Conservative/Innovative]
**Rationale:** [Why this works]
**Weakest Link:** [What breaks first]
```

### 5. Handover
After proposing hypotheses, instruct the user:
"Abduction complete. Hypotheses registered. Run `/q2-check` to enter Deduction phase."

```