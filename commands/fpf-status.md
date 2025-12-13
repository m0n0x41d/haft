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
    echo "FPF not initialized."
    echo "Run /fpf-0-init to set up FPF structure."
    exit
fi
```

### 2. Gather Statistics

```bash
# Count knowledge at each level
L0_COUNT=$(find .fpf/knowledge/L0 -name "*.md" 2>/dev/null | wc -l)
L1_COUNT=$(find .fpf/knowledge/L1 -name "*.md" 2>/dev/null | wc -l)
L2_COUNT=$(find .fpf/knowledge/L2 -name "*.md" 2>/dev/null | wc -l)
INVALID_COUNT=$(find .fpf/knowledge/invalid -name "*.md" 2>/dev/null | wc -l)

# Count artifacts
EVIDENCE_COUNT=$(find .fpf/evidence -name "*.md" 2>/dev/null | wc -l)
DRR_COUNT=$(find .fpf/decisions -name "DRR-*.md" 2>/dev/null | wc -l)
SESSIONS_COUNT=$(find .fpf/sessions -name "*.md" 2>/dev/null | wc -l)
```

### 3. Read Session State

```bash
cat .fpf/session.md
```

Extract: `Phase:`, `Problem:`, Active hypotheses, Last transition

### 4. Check for Issues

Scan for:
- Expired evidence (check `valid_until` dates)
- Low-congruence external evidence
- Missing validity windows

## Output Format

```markdown
## FPF Status

### Current Session

| Field | Value |
|-------|-------|
| **Phase** | [INITIALIZED / ABDUCTION_COMPLETE / DEDUCTION_COMPLETE / INDUCTION_COMPLETE / AUDIT_COMPLETE / DECIDED] |
| **Problem** | [problem statement or "none"] |
| **Started** | [timestamp] |
| **Last Activity** | [timestamp] |

### Knowledge Base

| Level | Count | Description |
|-------|-------|-------------|
| **L0** | [N] | Observations, unverified hypotheses |
| **L1** | [N] | Logically verified (passed deduction) |
| **L2** | [N] | Empirically tested (passed induction) |
| **Invalid** | [N] | Disproved (kept for learning) |

### Artifacts

| Type | Count |
|------|-------|
| Evidence files | [N] |
| Decisions (DRRs) | [N] |
| Archived sessions | [N] |

### Active Hypotheses

| ID | Name | Level | Next Action |
|----|------|-------|-------------|
| h1 | [name] | L0 | needs /fpf-2-check |
| h2 | [name] | L1 | needs /fpf-3-test |
| h3 | [name] | L2 | ready for decision |

### Issues & Warnings

**Evidence Health:**
- ✓ Healthy: [N] files
- ⚠ Expiring soon: [N] files
- ✗ Expired: [N] files
- ? No validity: [N] files

Run `/fpf-decay` for detailed report.

### Phase State Machine

```
Current: ──► [PHASE]

INITIALIZED ──► ABDUCTION_COMPLETE ──► DEDUCTION_COMPLETE
                                              │
                    ┌─────────────────────────┤
                    ▼                         ▼
            (fpf-3-test)              (fpf-3-research)
                    │                         │
                    └──────────┬──────────────┘
                               ▼
                    INDUCTION_COMPLETE
                               │
              ┌────────────────┼────────────────┐
              ▼                                 ▼
      (fpf-4-audit)                    (fpf-5-decide)
              │                          ⚠ warning
              ▼                                 │
      AUDIT_COMPLETE ───────────────────────────┤
                                                ▼
                                            DECIDED
```

### Suggested Next Step

**If INITIALIZED:**
→ `/fpf-1-hypothesize <problem>` — Start reasoning cycle

**If ABDUCTION_COMPLETE:**
→ `/fpf-2-check` — Verify logical consistency

**If DEDUCTION_COMPLETE:**
→ `/fpf-3-test` — Internal tests, benchmarks
→ `/fpf-3-research` — External evidence (can do both)

**If INDUCTION_COMPLETE:**
→ `/fpf-4-audit` — Critical review (recommended)
→ `/fpf-5-decide` — Finalize (with warning if no audit)

**If AUDIT_COMPLETE:**
→ `/fpf-5-decide` — Finalize decision

**If DECIDED:**
→ `/fpf-1-hypothesize <new problem>` — Start new cycle
→ `/fpf-query <topic>` — Search knowledge

### Command Reference

| Command | Description | Valid From |
|---------|-------------|------------|
| `/fpf-0-init` | Initialize FPF | (any) |
| `/fpf-1-hypothesize` | Generate hypotheses | INITIALIZED, DECIDED |
| `/fpf-2-check` | Logical verification | ABDUCTION_COMPLETE |
| `/fpf-3-test` | Internal tests | DEDUCTION_COMPLETE+ |
| `/fpf-3-research` | External evidence | DEDUCTION_COMPLETE+ |
| `/fpf-4-audit` | WLNK + bias review | INDUCTION_COMPLETE |
| `/fpf-5-decide` | Finalize decision | INDUCTION_COMPLETE*, AUDIT_COMPLETE |
| `/fpf-status` | This view | (any) |
| `/fpf-query` | Search knowledge | (any) |
| `/fpf-decay` | Evidence freshness | (any) |
| `/fpf-discard` | Abandon cycle | (active cycle) |

*With warning if audit skipped
```

## Quick Status

Single-line format:

```
FPF: [Phase] | L0:[N] L1:[N] L2:[N] | Evidence:[N] | Next: [command]
```

## Not Initialized

```markdown
## FPF Status

**Not initialized.**

Run `/fpf-0-init` to set up FPF structure.
```
