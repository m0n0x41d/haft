---
description: "Empirical verification of hypotheses (FPF Induction phase)"
arguments:
  - name: hypothesis
    description: "Specific hypothesis ID to test (optional)"
    required: false
  - name: focus
    description: "Specific assumption or aspect to test"
    required: false
---

# FPF Phase 3: Induction (Empirical Verification)

## Your Role

You are the **Inductor**. Design and execute tests that produce evidence.

This phase answers: "Does this actually work in practice?"

## Input

- If `$ARGUMENTS.hypothesis` provided: test that specific hypothesis
- Otherwise: test all hypotheses in `.fpf/knowledge/L1/`
- If `$ARGUMENTS.focus` provided: prioritize testing that aspect

## What Induction IS

- Running actual code/tests
- Benchmarking performance
- Prototyping and spiking
- Gathering metrics and data
- User testing / feedback
- Integration testing

## What Induction IS NOT

- Thinking about whether it would work (that's Deduction)
- Theorizing about performance (that's Deduction)
- Assuming it works because logic says so

## Process

### 1. Load Context

```bash
# Read L1 hypothesis
cat .fpf/knowledge/L1/[hypothesis].md

# Check what assumptions need empirical verification
grep -A5 "Assumptions" .fpf/knowledge/L1/[hypothesis].md
```

### 2. Design Test Plan

For each hypothesis, create explicit tests:

```markdown
## Test Plan: [Hypothesis Name]

### Assumptions to Verify
| Assumption | Test Method | Success Criteria |
|------------|-------------|------------------|
| [A1] | [how to test] | [what proves it] |
| [A2] | [how to test] | [what proves it] |

### Falsification Tests
| Falsification Criterion | Test | Expected if FALSE |
|-------------------------|------|-------------------|
| [from hypothesis] | [test] | [what we'd see] |

### Prototype/Spike Scope
[Minimal implementation to test core assumptions]
- [ ] Step 1
- [ ] Step 2
- [ ] Step 3

### Metrics to Collect
- [Metric 1]: target [X], acceptable range [Y-Z]
- [Metric 2]: target [X], acceptable range [Y-Z]
```

### 3. Execute Tests

**Actually run the tests.** Create evidence artifacts:

```markdown
## Evidence: [test-name]

**File:** `.fpf/evidence/[YYYY-MM-DD]-[test-name].md`

---
id: [slug]
type: empirical-test
source: internal
created: [timestamp]
hypothesis: [reference]
assumption_tested: [which one]
valid_until: [date — typically 3-6 months for benchmarks]
decay_action: refresh | deprecate | waive
scope:
  applies_to: "[conditions where this evidence is valid]"
  not_valid_for: "[conditions where this doesn't apply]"
---

### Test Executed
- Date: [timestamp]
- Environment: [test environment details]
- Hypothesis: [reference]
- Assumption tested: [which one]

### Method
[What exactly was done]

### Raw Results
```
[actual output, logs, metrics]
```

### Interpretation
[What this means for the hypothesis]

### Scope of Validity
- Applies when: [conditions]
- Does NOT apply when: [conditions]
- Re-test triggers: [what would invalidate this]

### Verdict
- [ ] Assumption CONFIRMED
- [ ] Assumption REFUTED
- [ ] Inconclusive, need more data
```

### 4. Aggregate Results

After all tests for a hypothesis:

```markdown
## Induction Results: [Hypothesis Name]

### Assumption Verification
| Assumption | Result | Evidence |
|------------|--------|----------|
| [A1] | ✓ Confirmed | evidence/[file1].md |
| [A2] | ✗ Refuted | evidence/[file2].md |
| [A3] | ~ Partial | evidence/[file3].md |

### Falsification Status
- [ ] No falsification criteria triggered → Still viable
- [ ] Falsification triggered → Hypothesis invalid

### Performance Data (if applicable)
| Metric | Target | Actual | Status |
|--------|--------|--------|--------|
| [M1] | [X] | [Y] | ✓/✗ |

### Overall Verdict
**VERIFIED** / **PARTIALLY VERIFIED** / **REFUTED**
```

### 5. Update Files

**If VERIFIED:** Promote to L2

```bash
git mv .fpf/knowledge/L1/[hyp].md .fpf/knowledge/L2/[hyp].md
```

Update frontmatter:

```yaml
---
status: L2
induction_passed: [timestamp]
evidence:
  - ../evidence/[file1].md
  - ../evidence/[file2].md
validity_conditions:
  - [when this remains true]
  - [re-verify if X changes]
---
```

**If REFUTED:** Move to invalid

```bash
git mv .fpf/knowledge/L1/[hyp].md .fpf/knowledge/invalid/[hyp].md
```

**If PARTIAL:** Keep in L1, update notes

### 6. Update Session

```markdown
## Status
Phase: INDUCTION_COMPLETE

## Hypotheses
| ID | Hypothesis | Status | Evidence |
|----|------------|--------|----------|
| h1 | [name] | L2 | 3 tests passed |
| h2 | [name] | invalid | Failed perf test |

## Next Step
- `/fpf:4-audit` for critical review before deciding
- `/fpf:5-decide` if ready to finalize
```

## Output Format

```markdown
## Induction Complete

### Results

| Hypothesis | Tests | Passed | Failed | Status |
|------------|-------|--------|--------|--------|
| H1: [name] | 4 | 4 | 0 | ✓ L2 |
| H2: [name] | 3 | 1 | 2 | ✗ Invalid |

### Evidence Created
- `.fpf/evidence/[date]-[test1].md`
- `.fpf/evidence/[date]-[test2].md`
- `.fpf/evidence/[date]-[test3].md`

### Key Findings
- [Surprising result or important learning]
- [Assumption that held/failed unexpectedly]

### Validity Conditions
H1 remains valid WHILE:
- [condition 1]
- [condition 2]

Re-verify IF:
- [trigger condition]

### Next Step
`/fpf:4-audit` — Critical review of remaining assumptions
or
`/fpf:5-decide` — Finalize decision if confident
```

## Test Quality Checklist

| Quality | Check |
|---------|-------|
| Reproducible | Can someone else run this test? |
| Falsifiable | Could this test possibly fail? |
| Relevant | Does it test the actual assumption? |
| Isolated | Testing one thing at a time? |
| Documented | Evidence captured for future reference? |
