---
description: "Show FPF status, current phase, and available actions"
---

# FPF Status

## Purpose

Display current state of FPF reasoning cycle and guide next steps.

## Process

### 1. Check Initialization

```bash
if [ ! -d ".fpf" ]; then
    echo "FPF not initialized. Run /fpf:0-init"
    exit
fi
```

### 2. Gather Statistics

```bash
# Count epistemes at each level
L0_COUNT=$(ls .fpf/knowledge/L0/*.md 2>/dev/null | wc -l)
L1_COUNT=$(ls .fpf/knowledge/L1/*.md 2>/dev/null | wc -l)
L2_COUNT=$(ls .fpf/knowledge/L2/*.md 2>/dev/null | wc -l)
INVALID_COUNT=$(ls .fpf/knowledge/invalid/*.md 2>/dev/null | wc -l)

# Count evidence, decisions, and archived sessions
EVIDENCE_COUNT=$(ls .fpf/evidence/*.md 2>/dev/null | wc -l)
DRR_COUNT=$(ls .fpf/decisions/DRR-*.md 2>/dev/null | wc -l)
SESSIONS_COUNT=$(ls .fpf/sessions/*.md 2>/dev/null | wc -l)
```

### 3. Read Session State

```bash
cat .fpf/session.md
```

### 4. Determine Phase & Next Actions

Based on session.md `Phase:` field and file counts.

## Output Format

```markdown
## FPF Status

### Current Session
**Phase:** [INITIALIZED / ABDUCTION_COMPLETE / DEDUCTION_COMPLETE / INDUCTION_COMPLETE / AUDIT_COMPLETE / DECIDED]
**Problem:** [from session.md or "none"]
**Started:** [timestamp]

### Knowledge Base
| Level | Count | Description |
|-------|-------|-------------|
| L0 | [N] | Observations, hypotheses |
| L1 | [N] | Logically verified |
| L2 | [N] | Empirically tested |
| Invalid | [N] | Disproved |

### Artifacts
- Evidence files: [N]
- Decisions (DRRs): [N]
- Archived sessions: [N]

### Active Hypotheses
[If in active cycle, list hypotheses with status]

| ID | Name | Level | Next Action |
|----|------|-------|-------------|
| [id] | [name] | L0 | needs /fpf:2-check |
| [id] | [name] | L1 | needs /fpf:3-test |

### Suggested Next Step

[Based on phase:]

**If INITIALIZED:**
→ `/fpf:1-hypothesize <problem>` — Start reasoning cycle

**If ABDUCTION_COMPLETE:**
→ `/fpf:2-check` — Verify logical consistency of hypotheses

**If DEDUCTION_COMPLETE:**
→ `/fpf:3-test` — Internal empirical tests (code, benchmarks)
→ `/fpf:3-research` — External evidence (web, docs)
→ (can do both, order doesn't matter)

**If INDUCTION_COMPLETE:**
→ `/fpf:4-audit` — Critical review before deciding
→ `/fpf:5-decide` — If confident, finalize decision

**If AUDIT_COMPLETE:**
→ `/fpf:5-decide` — Finalize decision (if no blockers)
→ Address blockers first (if any)

**If DECIDED:**
→ `/fpf:1-hypothesize <new problem>` — Start new cycle
→ `/fpf:query <topic>` — Search knowledge base

### Available Commands
| Command | Description |
|---------|-------------|
| `/fpf:0-init` | Initialize FPF (if not done) |
| `/fpf:1-hypothesize` | Generate hypotheses |
| `/fpf:2-check` | Logical verification |
| `/fpf:3-test` | Internal tests, benchmarks |
| `/fpf:3-research` | External evidence (web, docs) |
| `/fpf:4-audit` | WLNK + critical review |
| `/fpf:5-decide` | Finalize decision |
| `/fpf:status` | This status view |
| `/fpf:query` | Search knowledge |
| `/fpf:decay` | Check evidence freshness |
```

## Quick Status (One-liner)

If user just needs quick check:

```
FPF: [Phase] | L0:[N] L1:[N] L2:[N] | Next: [command]
```

Example:
```
FPF: DEDUCTION_COMPLETE | L0:1 L1:2 L2:0 | Next: /fpf:3-test
```

## Warnings to Surface

Always check and report:
- **Expired evidence**: Run `/fpf:decay` output if any expired
- **Low congruence**: Flag external evidence with `congruence: low`
- **No validity window**: Evidence without `valid_until` dates
