---
description: "Check evidence validity and manage decay (FPF Evidence Maintenance)"
arguments:
  - name: action
    description: "Action: check (default), refresh, deprecate, waive"
    required: false
  - name: evidence
    description: "Specific evidence file to act on"
    required: false
---

# FPF Evidence Decay Management

## Purpose

Evidence has a shelf life. This command helps manage evidence freshness and validity.

## Process

### 1. Scan Evidence Files

```bash
# Find all evidence files
find .fpf/evidence -name "*.md" -type f

# Extract validity info from frontmatter
for file in .fpf/evidence/*.md; do
    grep -A1 "valid_until:" "$file"
done
```

### 2. Check Validity Status

For each evidence file, determine status:

```markdown
## Evidence Validity Report

### ✓ Valid Evidence
| Evidence | Valid Until | Days Left | Type |
|----------|-------------|-----------|------|
| [file1] | 2025-06-01 | 45 | internal |
| [file2] | 2025-08-15 | 120 | external |

### ⚠️ Expiring Soon (< 30 days)
| Evidence | Valid Until | Days Left | Action Needed |
|----------|-------------|-----------|---------------|
| [file3] | 2025-02-01 | 21 | Plan refresh |

### ❌ Expired
| Evidence | Expired On | Days Overdue | Decay Action |
|----------|------------|--------------|--------------|
| [file4] | 2024-12-01 | 40 | refresh |
| [file5] | 2024-11-15 | 56 | deprecate |

### ❓ No Validity Window
| Evidence | Created | Type | Risk |
|----------|---------|------|------|
| [file6] | 2024-10-01 | external | ⚠️ High — add validity |
| [file7] | 2024-09-15 | internal | Medium — consider adding |
```

### 3. Actions

**check** (default): Report validity status of all evidence

**refresh**: Update evidence with new data
```bash
/fpf:decay refresh --evidence [file]
```
- Re-run the test or research
- Update the evidence file
- Set new `valid_until` date

**deprecate**: Mark evidence as no longer valid
```bash
/fpf:decay deprecate --evidence [file]
```
- Add `deprecated: true` to frontmatter
- Add `deprecated_reason` and `deprecated_date`
- Evidence remains for historical reference
- Claims depending on it drop to L0/L1

**waive**: Explicitly accept stale evidence (temporary)
```bash
/fpf:decay waive --evidence [file]
```
- Add `waived_until: [date]` (max 90 days)
- Add `waived_reason: "[justification]"`
- Must be reviewed at waive expiry

### 4. Impact on Knowledge

When evidence expires or is deprecated:

```markdown
## Affected Knowledge

### Claims at Risk
| Knowledge | Level | Depends On | New Status |
|-----------|-------|------------|------------|
| [L2/claim1] | L2 | [expired evidence] | → L1 (demote) |
| [L1/claim2] | L1 | [deprecated evidence] | → L0 (demote) |

### Recommended Actions
1. [claim1]: Run `/fpf:3-test` to refresh evidence
2. [claim2]: Run `/fpf:3-research` for current info
```

## Validity Guidelines

| Evidence Type | Recommended Validity | Rationale |
|---------------|---------------------|-----------|
| Benchmark (internal) | 3-6 months | Environment changes |
| API behavior test | Until next version | Behavior may change |
| External docs | 6-12 months | Docs update |
| Blog posts | 1-2 years | May become outdated |
| Academic papers | 2-5 years | Slower to invalidate |
| Official specs | Until superseded | Stable reference |

## Output Format

```markdown
## Evidence Decay Report

**Scanned:** [N] evidence files
**Healthy:** [N] (valid, >30 days remaining)
**Warning:** [N] (expiring within 30 days)
**Expired:** [N] (past valid_until)
**No window:** [N] (validity not set)

### Action Required

**Immediate (expired):**
- [ ] `.fpf/evidence/[file1].md` — refresh or deprecate
- [ ] `.fpf/evidence/[file2].md` — refresh or deprecate

**Soon (expiring):**
- [ ] `.fpf/evidence/[file3].md` — plan refresh by [date]

**Housekeeping (no window):**
- [ ] `.fpf/evidence/[file4].md` — add valid_until

### Knowledge Impact
[N] L2 claims may need review if evidence not refreshed.
```

## Integration with Audit

`/fpf:4-audit` automatically runs decay check and includes results.

Evidence issues found in audit:
- Expired evidence → Blocker (must resolve)
- Expiring soon → Warning
- No validity window → Note (fix for hygiene)
