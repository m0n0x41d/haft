---
description: "Logical verification of hypotheses (FPF Deduction phase)"
arguments:
  - name: hypothesis
    description: "Specific hypothesis ID to check (optional, checks all L0 if omitted)"
    required: false
---

# FPF Phase 2: Deduction (Logical Verification)

## Your Role

You are the **Deductor**. Verify logical consistency without running code or experiments.

This phase answers: "Does this hypothesis make sense? Are there logical contradictions?"

## Input

- If `$ARGUMENTS.hypothesis` provided: check that specific hypothesis
- Otherwise: check all hypotheses in `.fpf/knowledge/L0/`

## What Deduction IS

- Checking logical consistency
- Identifying contradictions with known facts (L2 knowledge)
- Tracing implications ("if X, then Y must follow")
- Reviewing assumptions for internal consistency
- Code review / design review (reading, not running)

## What Deduction IS NOT

- Running tests (that's Induction)
- Benchmarking (that's Induction)
- User feedback (that's Induction)
- Anything requiring execution

## Process

### 1. Load Context

```bash
# Read hypothesis file(s)
cat .fpf/knowledge/L0/[hypothesis].md

# Read verified knowledge that might conflict
ls .fpf/knowledge/L2/

# Read session state
cat .fpf/session.md
```

### 2. Logical Consistency Check

For each hypothesis, verify:

```markdown
## Deduction: [Hypothesis Name]

### Consistency with Known Facts
| L2 Fact | Compatible? | Notes |
|---------|-------------|-------|
| [fact1] | ✓/✗ | [why] |
| [fact2] | ✓/✗ | [why] |

### Internal Consistency
- [ ] Assumptions don't contradict each other
- [ ] Approach logically follows from assumptions
- [ ] No circular reasoning

### Implication Analysis
If this hypothesis is true, then:
1. [Implication 1] — acceptable? 
2. [Implication 2] — acceptable?
3. [Implication 3] — acceptable?

### Edge Cases (Logical)
| Edge Case | How hypothesis handles it |
|-----------|---------------------------|
| [case 1] | [handling / gap] |
| [case 2] | [handling / gap] |

### Weakest Link Reassessment
Original: [from hypothesis]
After deduction: [updated assessment]
```

### 3. Verdict per Hypothesis

```markdown
### Verdict: [PASS / FAIL / CONDITIONAL]

**PASS** → Promote to L1
- Logically consistent
- No contradictions with L2 facts
- Assumptions are internally coherent

**CONDITIONAL** → Stays L0 with notes
- Consistent IF [condition]
- Needs clarification on [X]

**FAIL** → Move to invalid/
- Contradicts [specific L2 fact]
- Internal contradiction: [details]
- Logical flaw: [details]
```

### 4. Update Files

**If PASS:** Move hypothesis to L1

```bash
git mv .fpf/knowledge/L0/[hyp].md .fpf/knowledge/L1/[hyp].md
```

Update the file's frontmatter:

```yaml
---
status: L1
deduction_passed: [timestamp]
deduction_notes: [brief summary]
---
```

**If FAIL:** Move to invalid

```bash
git mv .fpf/knowledge/L0/[hyp].md .fpf/knowledge/invalid/[hyp].md
```

Add invalidation reason to file.

### 5. Update Session

```markdown
## Status
Phase: DEDUCTION_COMPLETE

## Hypotheses
| ID | Hypothesis | Status | Deduction Result |
|----|------------|--------|------------------|
| h1 | [name] | L1 | PASS |
| h2 | [name] | L0 | CONDITIONAL: needs X |
| h3 | [name] | invalid | FAIL: contradicts Y |

## Next Step
- `/fpf:3-test` to empirically verify L1 hypotheses
- `/fpf:1-hypothesize` if all failed, need new approaches
```

## Output Format

```markdown
## Deduction Complete

### Results

| Hypothesis | Result | Reason |
|------------|--------|--------|
| H1: [name] | ✓ PASS → L1 | Consistent, no contradictions |
| H2: [name] | ⚠ CONDITIONAL | Needs clarification on [X] |
| H3: [name] | ✗ FAIL | Contradicts [L2 fact] |

### Key Findings
- [Important logical insight discovered]
- [Assumption that needs empirical verification]

### Files Updated
- `.fpf/knowledge/L1/[h1].md` (promoted)
- `.fpf/knowledge/invalid/[h3].md` (invalidated)

### Next Step
`/fpf:3-test` to empirically verify L1 hypotheses

Or if specific concerns:
`/fpf:3-test --hypothesis [id] --focus [specific assumption]`
```

## Common Deduction Failures

| Pattern | What It Means |
|---------|---------------|
| Contradicts L2 fact | Hypothesis ignores verified knowledge |
| Circular reasoning | A assumes B, B assumes A |
| Hidden assumption | "This works if X" but X never stated |
| Scale blindness | Works for N=10, breaks at N=10000 |
