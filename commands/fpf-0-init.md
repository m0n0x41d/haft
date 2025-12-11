---
description: "Initialize FPF (First Principles Framework) for structured reasoning"
---

# FPF Initialization

## What This Does

Creates the `.fpf/` directory structure for systematic hypothesis-driven reasoning.

## Process

### 1. Create Directory Structure

```bash
mkdir -p .fpf/knowledge/L0
mkdir -p .fpf/knowledge/L1
mkdir -p .fpf/knowledge/L2
mkdir -p .fpf/knowledge/invalid
mkdir -p .fpf/evidence
mkdir -p .fpf/decisions
mkdir -p .fpf/sessions
```

### 2. Create Session File

Create `.fpf/session.md`:

```markdown
# FPF Session

## Status
Phase: INITIALIZED
Started: [timestamp]
Problem: (none yet)

## Active Hypotheses
(none)

## Next Step
Run `/fpf:1-hypothesize <problem>` to begin reasoning cycle.

## Command Reference
| # | Command | Phase | Result |
|---|---------|-------|--------|
| 1 | `/fpf:1-hypothesize` | Abduction | Generate hypotheses → L0/ |
| 2 | `/fpf:2-check` | Deduction | Logical verification → L1/ |
| 3 | `/fpf:3-test` | Induction | Empirical verification → L2/ |
| 4 | `/fpf:4-audit` | Bias-Audit | Critical assumption review |
| 5 | `/fpf:5-decide` | Decision | Create DRR, close cycle |
| S | `/fpf:status` | — | Show current state |
| Q | `/fpf:query` | — | Search knowledge base |
```

### 3. Create .gitignore Entry (if needed)

Check if `.fpf/` should be tracked. Typically YES — this is valuable project knowledge.

### 4. Check for Existing Knowledge

If `.fpf/knowledge/` already has files, summarize:
- Count of L0/L1/L2 epistemes
- Any active session in progress

## Output

Confirm initialization:

```
FPF initialized.

Structure created:
  .fpf/
  ├── knowledge/
  │   ├── L0/     (observations, hypotheses)
  │   ├── L1/     (logically verified)
  │   ├── L2/     (empirically tested)
  │   └── invalid/
  ├── evidence/
  ├── decisions/
  ├── sessions/   (archived cycles)
  └── session.md

Next: /fpf:1-hypothesize <problem statement>
```

If already initialized, show current state instead (run `/fpf:status` logic).
