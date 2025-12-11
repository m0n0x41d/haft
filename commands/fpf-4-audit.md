---
description: "Critical review of assumptions and biases (FPF Bias-Audit phase)"
arguments:
  - name: hypothesis
    description: "Specific hypothesis ID to audit (optional, audits all L1+ if omitted)"
    required: false
---

# FPF Phase 4: Bias-Audit (Critical Review)

## Your Role

You are the **Scrutineer**. Challenge assumptions, find blind spots, stress-test thinking.

This phase answers: "What are we missing? What could go wrong that we haven't considered?"

## When to Use

- **Required:** Before any decision with significant/irreversible impact
- **Optional but valuable:** After Induction, before Decide
- **Can skip:** For low-stakes, easily reversible decisions

## Input

- Audit hypotheses in `.fpf/knowledge/L1/` and `.fpf/knowledge/L2/`
- Review evidence in `.fpf/evidence/`
- Consider the full decision context from `.fpf/session.md`

## Process

### 1. Load All Context

```bash
# All verified and candidate hypotheses
cat .fpf/knowledge/L1/*.md
cat .fpf/knowledge/L2/*.md

# All evidence
cat .fpf/evidence/*.md

# Decision context
cat .fpf/session.md
```

### 2. Assumption Audit

List ALL assumptions — explicit and implicit:

```markdown
## Assumption Inventory

### Explicit Assumptions (from hypothesis files)
| # | Assumption | Tested? | Confidence |
|---|------------|---------|------------|
| 1 | [from file] | Yes/No | High/Med/Low |

### Implicit Assumptions (unstated)
| # | Hidden Assumption | Risk if Wrong |
|---|-------------------|---------------|
| 1 | [e.g., "Users have modern browsers"] | [impact] |
| 2 | [e.g., "Traffic won't 10x suddenly"] | [impact] |
| 3 | [e.g., "Team can maintain this"] | [impact] |

### Environmental Assumptions
- [ ] Infrastructure stays same
- [ ] Dependencies remain stable  
- [ ] Team composition unchanged
- [ ] Requirements won't shift significantly
```

### 3. Bias Check

Actively look for cognitive biases:

```markdown
## Bias Analysis

### Confirmation Bias
Did we design tests that could only confirm, not refute?
- [Example of potentially biased test]
- [What counter-evidence would look like]

### Sunk Cost
Are we favoring an option because we've invested time in it?
- Time spent: [X]
- Would we choose same if starting fresh? 

### Availability Bias
Are we overweighting recent experiences or familiar patterns?
- [Pattern we're applying]
- [Why it might not fit here]

### Anchoring
Did early information overly constrain our thinking?
- First hypothesis was: [X]
- Did others get fair consideration?

### Survivorship Bias
Are we only looking at successful examples?
- [What failures might teach us]
```

### 4. Adversarial Analysis

Think like an attacker / skeptic:

```markdown
## Adversarial Review

### "What's the worst that happens if we're wrong?"
- Technical worst case: [scenario]
- Business worst case: [scenario]
- Recovery cost: [estimate]

### "Who would disagree with this decision?"
- [Stakeholder 1] might argue: [their view]
- [Stakeholder 2] might argue: [their view]
- Are their concerns addressed?

### "What would make us revisit this in 3 months?"
- [Trigger 1]
- [Trigger 2]

### "What's the cheapest way this fails?"
- [Failure mode requiring least effort to trigger]
```

### 5. Evidence Quality Review

```markdown
## Evidence Audit

### WLNK Check (Weakest Link Analysis)
**CRITICAL: Assurance = min(evidence assurances), NEVER average**

| Evidence | Source | Level | Congruence |
|----------|--------|-------|------------|
| [evidence1] | internal | L2 | — |
| [evidence2] | external | L1 | high |
| [evidence3] | external | L2 | low |

**Weakest Link:** [evidence3] — external with low congruence
**Effective Assurance:** L1 (capped by weakest)

⚠️ WARNING if:
- Any evidence has `congruence: low` → flag for review
- Mix of internal/external without congruence assessment
- Single source of truth for critical claim

### Congruence Warnings
For external evidence, check context match:
| Evidence | Source Context | Our Context | Congruence | Penalty |
|----------|---------------|-------------|------------|---------|
| [ext1] | "Company X, 100k users" | "Our app, 1k users" | low | ⚠️ Scale mismatch |
| [ext2] | "Redis official docs" | "Our Redis setup" | high | ✓ Direct match |

### Coverage
| Key Claim | Evidence Exists? | Quality |
|-----------|------------------|---------|
| [claim 1] | Yes/No | Strong/Weak |
| [claim 2] | Yes/No | Strong/Weak |

### Evidence Gaps
- [ ] [What we should have tested but didn't]
- [ ] [What would increase confidence]

### Evidence Freshness (Validity Windows)
| Evidence | Valid Until | Status | Action Needed |
|----------|-------------|--------|---------------|
| [file1] | 2025-06-01 | ✓ Valid | — |
| [file2] | 2024-12-01 | ⚠️ Expired | Refresh/Deprecate/Waive |
| [file3] | (none) | ⚠️ No window | Add validity |

Run `/fpf:decay` to see all evidence needing attention.
```

### 6. Final Scrutiny Verdict

```markdown
## Audit Verdict

### Blocker Issues
[Issues that MUST be resolved before deciding]
- [ ] [Issue 1]
- [ ] [Issue 2]

### Accepted Risks
[Risks we acknowledge and accept]
- [Risk 1]: Accepted because [reason]
- [Risk 2]: Mitigated by [plan]

### Recommendations
- [ ] Proceed to decision
- [ ] Need more evidence on: [X]
- [ ] Revisit hypothesis: [Y]
- [ ] Add validity conditions: [Z]

### Dissenting View
[If you were arguing AGAINST the leading hypothesis, what would you say?]
```

### 7. Update Session

```markdown
## Status
Phase: AUDIT_COMPLETE

## Audit Summary
- Blocker issues: [count]
- Accepted risks: [count]
- Evidence gaps: [count]

## Next Step
- If blockers: resolve before `/fpf:5-decide`
- If clear: `/fpf:5-decide` to finalize
```

## Output Format

```markdown
## Audit Complete

### Summary
- Hypotheses audited: [N]
- Blocker issues found: [N]
- Accepted risks documented: [N]

### Critical Findings

**Blockers (must resolve):**
1. [Blocker 1 — what and why]
2. [Blocker 2 — what and why]

**Risks (acknowledged):**
1. [Risk 1] — Accepted because [reason]
2. [Risk 2] — Mitigated by [plan]

**Blind Spots Identified:**
- [Previously unconsidered factor]

### Recommendation
[PROCEED / PAUSE / REVISIT]

**If PROCEED:** `/fpf:5-decide`
**If PAUSE:** Address [specific blockers]
**If REVISIT:** `/fpf:1-hypothesize` with new constraints
```

## Audit Smells (Red Flags)

| Smell | What It Means |
|-------|---------------|
| "No risks identified" | You're not looking hard enough |
| All assumptions "High confidence" | Overconfidence bias |
| No dissenting view possible | Groupthink or weak analysis |
| Evidence all from same source | Single point of failure |
| No validity conditions | Will be stale without knowing |
