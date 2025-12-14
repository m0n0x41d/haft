---
description: "Critical review of assumptions and biases (FPF Bias-Audit phase)"
arguments:
  - name: hypothesis
    description: "Specific hypothesis ID to audit (optional, audits all L1+ if omitted)"
    required: false
---

# FPF Phase 4: Bias-Audit (Critical Review)

## Phase Gate (MANDATORY)

**STOP. Verify phase before proceeding:**

1. Read `.fpf/session.md`, extract `Phase:` value
2. Check validity:

| Current Phase | Can Run? | Action |
|---------------|----------|--------|
| INDUCTION_COMPLETE | ✅ YES | Proceed |
| AUDIT_COMPLETE | ✅ YES | Re-audit if needed |
| Any other | ❌ NO | See below |

**If earlier phase:**
```
⛔ BLOCKED: No evidence to audit yet.
Current phase: [PHASE]
Complete /q3-test or /q3-research first.
```

---

## Your Role

You are the **Scrutineer**. Challenge assumptions, find blind spots, stress-test thinking.

This phase answers: **"What are we missing? What could go wrong that we haven't considered?"**

## When to Use

- **Required:** Before any decision with significant/irreversible impact
- **Optional but valuable:** After Induction, before Decide
- **Can skip:** For low-stakes, easily reversible decisions (with warning in /q5-decide)

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

### 2. WLNK Analysis (Weakest Link)

**CRITICAL: Assurance = min(evidence assurances), NEVER average**

For each hypothesis, calculate effective reliability:

```markdown
## WLNK Analysis: [Hypothesis Name]

### Evidence Chain

| Evidence | Type | Base R | Congruence | Φ(CL) | R_eff |
|----------|------|--------|------------|-------|-------|
| [ev1] | internal | 1.0 | — | 0.00 | 1.00 |
| [ev2] | external | 1.0 | high | 0.00 | 1.00 |
| [ev3] | external | 1.0 | medium | 0.15 | 0.85 |
| [ev4] | external | 0.8 | low | 0.35 | 0.45 |

### Congruence Penalty Reference

| Congruence Level | Φ(CL) Penalty | Meaning |
|------------------|---------------|---------|
| High | 0.00 | Direct context match |
| Medium | 0.15 | Partial context match |
| Low | 0.35 | Weak context match |

### WLNK Calculation

```
R_eff(ev1) = 1.0 - 0.00 = 1.00
R_eff(ev2) = 1.0 - 0.00 = 1.00
R_eff(ev3) = 1.0 - 0.15 = 0.85
R_eff(ev4) = 0.8 - 0.35 = 0.45

Hypothesis R_eff = min(1.00, 1.00, 0.85, 0.45) = 0.45
                   ↑
                   WEAKEST LINK (ev4: blog post, low congruence)
```

### Verdict

**Weakest Link:** [ev4] — external evidence with low congruence
**Effective Assurance:** Capped at R_eff = 0.45

⚠️ **This hypothesis reliability is limited by low-congruence external evidence.**

**Options to improve:**
1. Find higher-congruence external evidence
2. Run internal test to replace external evidence
3. Accept the risk with documented justification
```

### 3. Assumption Inventory

List ALL assumptions — explicit and implicit:

```markdown
## Assumption Inventory

### Explicit Assumptions (from hypothesis files)

| # | Assumption | Source | Tested? | Confidence |
|---|------------|--------|---------|------------|
| 1 | [from file] | [hypothesis] | Yes/No | High/Med/Low |
| 2 | [from file] | [hypothesis] | Yes/No | High/Med/Low |

### Implicit Assumptions (unstated but required)

| # | Hidden Assumption | Why It Matters | Risk if Wrong |
|---|-------------------|----------------|---------------|
| 1 | [e.g., "Users have modern browsers"] | [affects X] | [impact] |
| 2 | [e.g., "Traffic won't 10x suddenly"] | [affects Y] | [impact] |
| 3 | [e.g., "Team can maintain this"] | [affects Z] | [impact] |

### Environmental Assumptions

- [ ] Infrastructure stays same
- [ ] Dependencies remain stable  
- [ ] Team composition unchanged
- [ ] Requirements won't shift significantly
- [ ] Budget constraints hold
```

### 4. Bias Check

Actively look for cognitive biases:

```markdown
## Bias Analysis

### Confirmation Bias
Did we design tests that could only confirm, not refute?

- **Risk indicators:** [list any]
- **Counter-evidence sought?** Yes/No
- **Mitigation:** [action if needed]

### Sunk Cost Fallacy
Are we favoring an option because we've invested time in it?

- **Time invested per hypothesis:** [compare]
- **Would we choose same if starting fresh?** Yes/No/Uncertain
- **Mitigation:** [action if needed]

### Availability Bias
Are we overweighting recent experiences or familiar patterns?

- **Pattern we're applying:** [what]
- **Why it might not fit:** [reasons]
- **Mitigation:** [action if needed]

### Anchoring
Did early information overly constrain our thinking?

- **First hypothesis was:** [X]
- **Did others get fair consideration?** Yes/No
- **Mitigation:** [action if needed]

### Survivorship Bias
Are we only looking at successful examples?

- **Failures considered?** Yes/No
- **What failures might teach us:** [insights]
- **Mitigation:** [action if needed]
```

### 5. Adversarial Analysis

Think like an attacker / skeptic:

```markdown
## Adversarial Review

### "What's the worst that happens if we're wrong?"

| Scenario | Impact | Recovery Cost | Likelihood |
|----------|--------|---------------|------------|
| Technical failure | [impact] | [cost] | H/M/L |
| Business impact | [impact] | [cost] | H/M/L |
| Security issue | [impact] | [cost] | H/M/L |

### "Who would disagree with this decision?"

| Stakeholder | Their Likely Objection | Addressed? |
|-------------|------------------------|------------|
| [Person/Role 1] | [their view] | Yes/No/Partially |
| [Person/Role 2] | [their view] | Yes/No/Partially |

### "What would make us revisit this in 3 months?"

- [Trigger 1]
- [Trigger 2]
- [Trigger 3]

### "What's the cheapest way this fails?"

[Failure mode requiring least effort to trigger]
```

### 6. Evidence Quality Review

```markdown
## Evidence Quality Audit

### Coverage Check

| Key Claim | Evidence Exists? | Quality | Gap? |
|-----------|------------------|---------|------|
| [claim 1] | Yes/No | Strong/Weak | ✓/⚠ |
| [claim 2] | Yes/No | Strong/Weak | ✓/⚠ |

### Evidence Freshness (Validity Windows)

| Evidence | Valid Until | Status | Action Needed |
|----------|-------------|--------|---------------|
| [file1] | 2025-06-01 | ✓ Valid | — |
| [file2] | 2024-12-01 | ⚠ Expired | Refresh/Deprecate |
| [file3] | (none) | ⚠ No window | Add validity |

**Run `/q-decay` for detailed decay analysis.**

### Congruence Warnings

| Evidence | Source Context | Our Context | CL | Issue |
|----------|---------------|-------------|-----|-------|
| [ext1] | "Company X, 100k users" | "Our app, 1k users" | Low | ⚠ Scale mismatch |
| [ext2] | "Redis 7.0" | "Redis 6.2" | Med | Version difference |

### Single Points of Failure

| Critical Claim | # of Supporting Sources | Risk |
|----------------|------------------------|------|
| [claim 1] | 1 | ⚠ Single source |
| [claim 2] | 3 | ✓ Multiple sources |
```

### 7. Final Scrutiny Verdict

```markdown
## Audit Verdict

### Blocker Issues (MUST resolve before deciding)

| # | Issue | Severity | Resolution Required |
|---|-------|----------|---------------------|
| 1 | [issue] | Blocker | [action needed] |
| 2 | [issue] | Blocker | [action needed] |

### Warnings (Should address, can proceed with risk acceptance)

| # | Warning | Risk Level | Mitigation |
|---|---------|------------|------------|
| 1 | [warning] | High/Med | [plan] |
| 2 | [warning] | High/Med | [plan] |

### Accepted Risks

| # | Risk | Accepted Because | Owner |
|---|------|------------------|-------|
| 1 | [risk] | [justification] | [who monitors] |
| 2 | [risk] | [justification] | [who monitors] |

### Recommendations

- [ ] **PROCEED** — Evidence sufficient, risks acceptable
- [ ] **PROCEED WITH CAUTION** — Address warnings first
- [ ] **PAUSE** — Resolve blockers before deciding
- [ ] **REVISIT** — Need more evidence or new hypotheses

### Dissenting View

**If arguing AGAINST the leading hypothesis, what would you say?**

[Present the strongest counter-argument]
```

### 8. Update Session

```markdown
## Status
Phase: AUDIT_COMPLETE

## Audit Summary
- **Blockers found:** [count]
- **Warnings:** [count]
- **Accepted risks:** [count]
- **WLNK R_eff:** [lowest value]
- **Recommendation:** [PROCEED/PAUSE/REVISIT]

## Phase Transitions Log
| Timestamp | From | To | Trigger |
|-----------|------|-----|---------|
| [...] | ... | ... | ... |
| [now] | INDUCTION_COMPLETE | AUDIT_COMPLETE | /q4-audit |

## Next Step
- If PROCEED: `/q5-decide` to finalize decision
- If blockers: Resolve issues first, then `/q5-decide`
- If REVISIT: `/q1-hypothesize` with new constraints
```

## Output Format

```markdown
## Audit Complete

### Summary

| Metric | Value |
|--------|-------|
| Hypotheses audited | [N] |
| Blocker issues | [N] |
| Warnings | [N] |
| Accepted risks | [N] |
| Lowest WLNK R_eff | [value] |

### WLNK Results

| Hypothesis | Evidence Count | Weakest Link | R_eff |
|------------|----------------|--------------|-------|
| H1: [name] | [N] | [evidence] | [value] |
| H2: [name] | [N] | [evidence] | [value] |

### Critical Findings

**Blockers (must resolve):**
1. [Blocker 1 — what and why]

**Warnings (should address):**
1. [Warning 1] — Mitigation: [plan]

**Blind Spots Identified:**
- [Previously unconsidered factor]

### Recommendation

**[PROCEED / PROCEED WITH CAUTION / PAUSE / REVISIT]**

**Reasoning:** [brief justification]

---

**If PROCEED:** `/q5-decide`
**If PAUSE:** Address [specific blockers] first
**If REVISIT:** `/q1-hypothesize` with [new constraints]
```

## Audit Smells (Red Flags)

| Smell | What It Means | Action |
|-------|---------------|--------|
| "No risks identified" | Not looking hard enough | Dig deeper |
| All assumptions "High confidence" | Overconfidence bias | Challenge each one |
| No dissenting view possible | Groupthink or weak analysis | Seek devil's advocate |
| Evidence all from same source | Single point of failure | Find diverse sources |
| No validity windows set | Will be stale without knowing | Add validity dates |
| All external evidence low congruence | May not apply to our context | Get internal evidence |
| WLNK R_eff < 0.5 | Very weak evidence chain | Strengthen before deciding |

## Common Mistakes to Avoid

| Mistake | Why It's Wrong | Do This Instead |
|---------|----------------|-----------------|
| Rubber-stamping | Defeats purpose of audit | Challenge everything |
| Ignoring WLNK | Overestimates confidence | Always calculate R_eff |
| Skipping bias check | Blindspots remain | Systematically check each bias |
| No dissenting view | Echo chamber | Force steel-man counter-argument |
| Proceeding with blockers | High-risk decision | Resolve blockers first |
