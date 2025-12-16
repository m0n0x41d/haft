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

**Key Principle:** Stale evidence undermines the entire assurance case. An L2 hypothesis supported by expired evidence may no longer be trustworthy.

## Process

### 1. Scan Evidence Files

```bash
# Find all evidence files
find .quint/evidence -name "*.md" -type f

# Extract validity info from frontmatter
for file in .quint/evidence/*.md; do
    grep -A1 "valid_until:" "$file"
done
```

### 2. Categorize by Status

For each evidence file, determine status based on `valid_until` field:

| Status | Condition | Risk Level |
|--------|-----------|------------|
| ✓ Valid | >30 days remaining | Low |
| ⚠ Expiring | ≤30 days remaining | Medium |
| ✗ Expired | Past valid_until | High |
| ? Unknown | No valid_until set | Medium |

### 3. Calculate Epistemic Debt

**Epistemic Debt** = accumulated days of expired evidence

```
For each expired evidence:
  debt += (today - valid_until).days
  
Total Epistemic Debt = sum of all debt
```

**Debt Thresholds:**
| Debt (days) | Severity | Action |
|-------------|----------|--------|
| 0 | ✓ Healthy | Maintain |
| 1-30 | ⚠ Warning | Plan refresh |
| 31-90 | ⚠ High | Prioritize refresh |
| >90 | ✗ Critical | Immediate action |

## Output Format

```markdown
## Evidence Decay Report

**Scan Date:** [timestamp]
**Total Evidence Files:** [N]

### Summary

| Status | Count | Percentage |
|--------|-------|------------|
| ✓ Valid (>30 days) | [N] | [%] |
| ⚠ Expiring (≤30 days) | [N] | [%] |
| ✗ Expired | [N] | [%] |
| ? No validity set | [N] | [%] |

**Epistemic Debt:** [N] days
**Debt Level:** [Healthy / Warning / High / Critical]

---

### ✓ Valid Evidence

| Evidence | Type | Valid Until | Days Left |
|----------|------|-------------|-----------|
| [file1] | internal | 2025-06-01 | 45 |
| [file2] | external | 2025-08-15 | 120 |

---

### ⚠ Expiring Soon (≤30 days)

**Action Required: Plan refresh**

| Evidence | Valid Until | Days Left | Depends On |
|----------|-------------|-----------|------------|
| [file3] | 2025-02-01 | 21 | H1, H2 |
| [file4] | 2025-02-10 | 30 | H1 |

**Impact if not refreshed:**
- [H1] would lose evidence support
- [H2] would drop from L2 to L1

---

### ✗ Expired

**Action Required: Refresh, deprecate, or waive**

| Evidence | Expired On | Days Overdue | Depends On | Suggested Action |
|----------|------------|--------------|------------|------------------|
| [file5] | 2024-12-01 | 40 | H3 | refresh |
| [file6] | 2024-11-15 | 56 | — | deprecate |

**Impact:**
- [H3] evidence is stale — reliability questionable
- Claims depending on expired evidence should be re-evaluated

---

### ? No Validity Window

**Action Required: Add valid_until dates**

| Evidence | Created | Type | Risk |
|----------|---------|------|------|
| [file7] | 2024-10-01 | external | ⚠ High — external evidence ages faster |
| [file8] | 2024-09-15 | internal | Medium |

**Recommendation:** Add validity windows based on evidence type:
- Internal benchmarks: 3-6 months
- External docs: 6-12 months
- Blog posts: 1-2 years

---

### Affected Knowledge

**L2 Claims at Risk:**

| Knowledge | Current Level | Depends On | Risk |
|-----------|---------------|------------|------|
| [L2/claim1] | L2 | [expired evidence] | ⚠ May need demotion to L1 |
| [L2/claim2] | L2 | [expiring evidence] | Plan refresh |

**Recommended Actions:**
1. `[claim1]`: Run `/q3-test` to refresh evidence
2. `[claim2]`: Run `/q3-research` for current info

---

### Action Items

**Immediate (expired):**
- [ ] `.quint/evidence/[file5].md` — refresh or deprecate
- [ ] `.quint/evidence/[file6].md` — deprecate (no longer relevant)

**Soon (expiring within 30 days):**
- [ ] `.quint/evidence/[file3].md` — plan refresh by [date]
- [ ] `.quint/evidence/[file4].md` — plan refresh by [date]

**Housekeeping (no validity window):**
- [ ] `.quint/evidence/[file7].md` — add valid_until
- [ ] `.quint/evidence/[file8].md` — add valid_until
```

## Actions

### check (default)

Report validity status of all evidence.

```bash
/q-decay
/q-decay check
```

### refresh

Update evidence with new data.

```bash
/q-decay refresh --evidence [file]
```

Process:
1. Re-run the test or research
2. Update the evidence file with new results
3. Set new `valid_until` date
4. Update any changed findings

### deprecate

Mark evidence as no longer valid.

```bash
/q-decay deprecate --evidence [file]
```

Process:
1. Add to evidence frontmatter:
   ```yaml
   deprecated: true
   deprecated_date: [timestamp]
   deprecated_reason: "[why no longer valid]"
   ```
2. Evidence remains for historical reference
3. Claims depending on it may need demotion (L2→L1, L1→L0)

### waive

Explicitly accept stale evidence (temporary).

```bash
/q-decay waive --evidence [file]
```

Process:
1. Add to evidence frontmatter:
   ```yaml
   waived_until: [date]  # Max 90 days from now
   waived_reason: "[justification for accepting stale evidence]"
   waived_by: "[who approved]"
   ```
2. Must be reviewed at waive expiry
3. Use sparingly — only when refresh isn't practical

## Validity Guidelines

| Evidence Type | Recommended Validity | Rationale |
|---------------|---------------------|-----------|
| Internal benchmark | 3-6 months | Environment/code changes |
| Internal test | 3-6 months | Code changes may invalidate |
| API behavior test | Until next version | Behavior may change |
| External official docs | 6-12 months | Docs update periodically |
| External blog posts | 1-2 years | May become outdated |
| Academic papers | 2-5 years | Slower to invalidate |
| Official specs | Until superseded | Stable reference |

## Integration with Audit

`/q4-audit` automatically includes decay check.

Evidence issues found in audit:
- **Expired evidence** → Blocker (must resolve before deciding)
- **Expiring soon** → Warning (plan refresh)
- **No validity window** → Note (fix for hygiene)

## Example Workflow

```markdown
## Quarterly Evidence Review

1. Run decay check:
   ```
   /q-decay
   ```

2. For each expired evidence, decide:
   - Still relevant? → refresh
   - No longer needed? → deprecate
   - Can't refresh now? → waive (with justification)

3. For evidence without validity:
   - Add appropriate valid_until based on type

4. Update affected L2 claims if evidence deprecated

5. Document review in session notes
```
