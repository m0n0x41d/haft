---
description: "Empirical verification through internal tests (FPF Induction phase)"
arguments:
  - name: hypothesis
    description: "Specific hypothesis ID to test (optional)"
    required: false
  - name: focus
    description: "Specific assumption or aspect to test"
    required: false
---

# FPF Phase 3: Induction — Internal Testing

## Phase Gate (MANDATORY)

**STOP. Verify phase before proceeding:**

1. Read `.fpf/session.md`, extract `Phase:` value
2. Check validity:

| Current Phase | Can Run? | Action |
|---------------|----------|--------|
| DEDUCTION_COMPLETE | ✅ YES | Proceed |
| INDUCTION_COMPLETE | ✅ YES | Add more evidence |
| Any other | ❌ NO | See below |

**If INITIALIZED or ABDUCTION_COMPLETE:**
```
⛔ BLOCKED: Hypotheses not yet verified for logical consistency.
Run /fpf-2-check first.
```

**If AUDIT_COMPLETE or DECIDED:**
```
⛔ BLOCKED: Cycle already past induction phase.
Current phase: [PHASE]
Start new cycle with /fpf-1-hypothesize if needed.
```

---

## Your Role

You are the **Inductor**. Design and execute tests that produce **internal evidence**.

This phase answers: **"Does this actually work in practice, in OUR context?"**

## Difference from /fpf-3-research

| This Command (/fpf-3-test) | /fpf-3-research |
|----------------------------|-----------------|
| Run code, benchmarks, experiments | Search web, read docs |
| **Internal** evidence | **External** evidence |
| "Does it work HERE?" | "Does it work ELSEWHERE?" |
| Direct applicability (no congruence penalty) | Requires congruence assessment |

**Both are Induction** — gathering real-world evidence. Different sources.

## What Induction IS

- Running actual code/tests
- Benchmarking performance
- Prototyping and spiking
- Gathering metrics and data
- Integration testing
- Load/stress testing

## What Induction IS NOT

- Thinking about whether it would work (that's Deduction)
- Theorizing about performance (that's Deduction)
- Assuming it works because logic says so

## Input

- If `$ARGUMENTS.hypothesis` provided: test that specific hypothesis
- Otherwise: test all hypotheses in `.fpf/knowledge/L1/`
- If `$ARGUMENTS.focus` provided: prioritize testing that aspect

## Process

### 1. Load Context

```bash
# Read L1 hypotheses to test
cat .fpf/knowledge/L1/*.md

# Read project context
cat .fpf/context.md

# Check what assumptions need empirical verification
grep -A10 "Assumptions" .fpf/knowledge/L1/*.md

# Read existing evidence to avoid duplication
ls .fpf/evidence/
```

### 2. Design Test Plan

For each hypothesis, create explicit tests:

```markdown
## Test Plan: [Hypothesis Name]

### Assumptions to Verify Empirically

| Assumption | Test Method | Success Criteria | Priority |
|------------|-------------|------------------|----------|
| [A1] | [how to test] | [what proves it] | High/Med/Low |
| [A2] | [how to test] | [what proves it] | High/Med/Low |

### Falsification Tests

| Falsification Criterion | Test | Expected Result if FALSE |
|-------------------------|------|--------------------------|
| [from hypothesis] | [test] | [what we'd observe] |

### Metrics to Collect

| Metric | Target | Acceptable Range | Method |
|--------|--------|------------------|--------|
| [Metric 1] | [X] | [Y-Z] | [how measured] |
| [Metric 2] | [X] | [Y-Z] | [how measured] |

### Prototype/Spike Scope (if needed)

Minimal implementation to test core assumptions:
- [ ] Step 1: [specific action]
- [ ] Step 2: [specific action]
- [ ] Step 3: [specific action]

**Time budget:** [X hours/days]
```

### 3. Execute Tests

**Actually run the tests.** Create evidence artifacts for each significant test.

### 4. Create Evidence Artifact

For each test, create `.fpf/evidence/[YYYY-MM-DD]-[test-name].md`:

```markdown
---
id: [slug]
type: internal-test
source: internal
created: [timestamp]
hypothesis: [path to hypothesis file]
assumption_tested: "[which assumption this tests]"
valid_until: [date — typically 3-6 months for benchmarks]
decay_action: refresh
scope:
  applies_to: "[conditions where this evidence is valid]"
  not_valid_for: "[conditions where this doesn't apply]"
  environment: "[test environment details]"
---

# Test: [Descriptive Name]

## Purpose
[What this test verifies]

## Hypothesis Reference
- **File:** `.fpf/knowledge/L1/[hypothesis].md`
- **Assumption tested:** [specific assumption]
- **Falsification criterion:** [what would disprove]

## Test Environment
- **Date:** [timestamp]
- **System:** [hardware/software details]
- **Configuration:** [relevant config]
- **Data:** [test data description]

## Method
[Exact steps taken — reproducible]

```bash
# Commands run (if applicable)
[actual commands]
```

## Raw Results

```
[actual output, logs, metrics]
```

## Interpretation

### What the results show:
[Analysis of raw results]

### Regarding the assumption:
[How this relates to the hypothesis assumption]

### Confidence level:
[High/Medium/Low] because [reasoning]

## Scope of Validity

**This evidence applies when:**
- [Condition 1]
- [Condition 2]

**This evidence does NOT apply when:**
- [Condition 1 — e.g., different scale]
- [Condition 2 — e.g., different data patterns]

**Re-test triggers:**
- [What changes would invalidate this evidence]
- [When to re-run this test]

## Verdict

- [x] Assumption **CONFIRMED** — evidence supports hypothesis
- [ ] Assumption **REFUTED** — evidence contradicts hypothesis
- [ ] **INCONCLUSIVE** — need more data because [reason]

## Validity Window

**Valid until:** [date]
**Recommended refresh:** [timeframe]
**Decay action:** refresh / deprecate / waive
```

### 5. Aggregate Results

After all tests for a hypothesis:

```markdown
## Induction Results: [Hypothesis Name]

### Assumption Verification Summary

| Assumption | Test | Result | Evidence File |
|------------|------|--------|---------------|
| [A1] | [test name] | ✓ Confirmed | evidence/[file1].md |
| [A2] | [test name] | ✗ Refuted | evidence/[file2].md |
| [A3] | [test name] | ~ Partial | evidence/[file3].md |

### Falsification Status
- [ ] No falsification criteria triggered → Hypothesis remains viable
- [ ] Falsification triggered → Hypothesis invalid

### Performance Data (if applicable)

| Metric | Target | Actual | Status |
|--------|--------|--------|--------|
| [M1] | [X] | [Y] | ✓/✗ |
| [M2] | [X] | [Y] | ✓/✗ |

### Overall Verdict

**VERIFIED** / **PARTIALLY VERIFIED** / **REFUTED**

Justification: [brief explanation]
```

### 6. Update Hypothesis Files

**If VERIFIED:** Promote to L2

```bash
mv .fpf/knowledge/L1/[hyp].md .fpf/knowledge/L2/[hyp].md
```

Update frontmatter:

```yaml
---
status: L2
induction_passed: [timestamp]
evidence:
  - ../evidence/[file1].md
  - ../evidence/[file2].md
evidence_summary: |
  Verified through internal testing.
  Key finding: [summary]
validity_conditions:
  - [when this remains true]
  - [re-verify if X changes]
---
```

**If REFUTED:** Move to invalid

```bash
mv .fpf/knowledge/L1/[hyp].md .fpf/knowledge/invalid/[hyp].md
```

```yaml
---
status: invalid
invalidated: [timestamp]
invalidation_reason: |
  Failed empirical testing.
  Evidence: [reference]
  Learning: [what we learned]
---
```

**If PARTIAL:** Keep in L1, update notes

```yaml
---
status: L1
induction_result: partial
partial_notes: |
  Some assumptions verified, others need more testing.
  Confirmed: [list]
  Still uncertain: [list]
---
```

### 7. Update Session

```markdown
## Status
Phase: INDUCTION_COMPLETE

## Active Hypotheses
| ID | Hypothesis | Status | Evidence Files |
|----|------------|--------|----------------|
| h1 | [name] | L2 | 3 tests passed |
| h2 | [name] | invalid | Failed perf test |

## Phase Transitions Log
| Timestamp | From | To | Trigger |
|-----------|------|-----|---------|
| [...] | ... | ... | ... |
| [now] | DEDUCTION_COMPLETE | INDUCTION_COMPLETE | /fpf-3-test |

## Next Step
- `/fpf-3-research` for external evidence (optional, complements internal)
- `/fpf-4-audit` for critical review before deciding (recommended)
- `/fpf-5-decide` to finalize (if confident, audit recommended first)
```

## Output Format

```markdown
## Internal Testing Complete

### Results Summary

| Hypothesis | Tests Run | Passed | Failed | New Status |
|------------|-----------|--------|--------|------------|
| H1: [name] | 4 | 4 | 0 | ✓ L2 |
| H2: [name] | 3 | 1 | 2 | ✗ Invalid |

### Evidence Created
- `.fpf/evidence/[date]-[test1].md`
- `.fpf/evidence/[date]-[test2].md`
- `.fpf/evidence/[date]-[test3].md`

### Key Findings

**H1 verified:**
- [Key finding 1]
- [Key finding 2]

**H2 refuted:**
- [What failed]
- **Learning:** [Valuable insight from failure]

### Validity Conditions

H1 remains valid WHILE:
- [condition 1]
- [condition 2]

Re-verify IF:
- [trigger condition]

---

**Next Step:**
- `/fpf-3-research` — Gather external evidence (complements internal tests)
- `/fpf-4-audit` — Critical review of all evidence (recommended)
- `/fpf-5-decide` — Finalize decision (audit recommended first)
```

## Test Quality Checklist

| Quality | Check | ✓/✗ |
|---------|-------|-----|
| **Reproducible** | Can someone else run this test? | |
| **Falsifiable** | Could this test possibly fail? | |
| **Relevant** | Does it test the actual assumption? | |
| **Isolated** | Testing one thing at a time? | |
| **Documented** | Evidence captured for future reference? | |
| **Scoped** | Validity conditions clear? | |

## Common Mistakes to Avoid

| Mistake | Why It's Wrong | Do This Instead |
|---------|----------------|-----------------|
| Testing in wrong environment | Results won't match production | Match production as closely as possible |
| Confirmation bias in test design | Only testing happy path | Design tests that could fail |
| Missing validity window | Evidence will become stale silently | Always set valid_until date |
| Vague scope | Can't know when evidence applies | Be specific about conditions |
| Not documenting failures | Lose learning opportunity | Failed tests are valuable — document them |
