---
description: "Logical verification of hypotheses (FPF Deduction phase)"
arguments:
  - name: hypothesis
    description: "Specific hypothesis ID to check (optional, checks all L0 if omitted)"
    required: false
---

# FPF Phase 2: Deduction (Logical Verification)

## Phase Gate (MANDATORY)

**STOP. Verify phase before proceeding:**

1. Read `.fpf/session.md`, extract `Phase:` value
2. Check validity:

| Current Phase | Can Run? | Action |
|---------------|----------|--------|
| ABDUCTION_COMPLETE | ✅ YES | Proceed |
| Any other | ❌ NO | See below |

**If INITIALIZED:**
```
⛔ BLOCKED: No hypotheses to check.
Run /fpf-1-hypothesize <problem> first.
```

**If DEDUCTION_COMPLETE or later:**
```
⛔ BLOCKED: Deduction already complete.
Current phase: [PHASE]
Proceed with /fpf-3-test, /fpf-3-research, or continue current phase.
```

---

## Your Role

You are the **Deductor**. Verify logical consistency without running code or experiments.

This phase answers: **"Does this hypothesis make sense? Are there logical contradictions?"**

## What Deduction IS

- Checking logical consistency
- Identifying contradictions with known facts (L2 knowledge)
- Tracing implications ("if X, then Y must follow")
- Reviewing assumptions for internal consistency
- Code review / design review (reading, not running)
- Thought experiments and edge case analysis

## What Deduction IS NOT

- Running tests (that's Induction)
- Benchmarking (that's Induction)
- User feedback (that's Induction)
- Anything requiring execution or external data gathering

## Input

- If `$ARGUMENTS.hypothesis` provided: check that specific hypothesis
- Otherwise: check all hypotheses in `.fpf/knowledge/L0/`

## Process

### 1. Load Context

```bash
# Read hypothesis file(s) to check
cat .fpf/knowledge/L0/*.md

# Read verified knowledge that might conflict
cat .fpf/knowledge/L2/*.md

# Read invalid hypotheses to avoid repeating mistakes
cat .fpf/knowledge/invalid/*.md

# Read session state
cat .fpf/session.md
```

### 2. Logical Consistency Check

For each hypothesis, perform systematic verification:

```markdown
## Deduction: [Hypothesis Name]

### 1. Consistency with Verified Knowledge (L2)

| L2 Fact | Compatible? | Analysis |
|---------|-------------|----------|
| [fact1] | ✓/✗ | [reasoning] |
| [fact2] | ✓/✗ | [reasoning] |

**Verdict:** [No contradictions / Contradicts X]

### 2. Internal Consistency

Check that the hypothesis doesn't contradict itself:

- [ ] Assumptions don't contradict each other
- [ ] Approach logically follows from assumptions
- [ ] No circular reasoning (A requires B, B requires A)
- [ ] Scope claims are consistent

**Verdict:** [Internally consistent / Has contradiction: X]

### 3. Implication Analysis

If this hypothesis is true, what MUST follow?

| Implication | Acceptable? | Notes |
|-------------|-------------|-------|
| [Impl 1] | ✓/✗/? | [analysis] |
| [Impl 2] | ✓/✗/? | [analysis] |
| [Impl 3] | ✓/✗/? | [analysis] |

**Verdict:** [All implications acceptable / Problematic implication: X]

### 4. Assumption Audit

| Assumption | Testable? | Reasonable? | Risk if Wrong |
|------------|-----------|-------------|---------------|
| [A1] | Yes/No | High/Med/Low | [impact] |
| [A2] | Yes/No | High/Med/Low | [impact] |
| [A3] | Yes/No | High/Med/Low | [impact] |

**Verdict:** [Assumptions reasonable / Questionable assumption: X]

### 5. Edge Cases (Logical)

| Edge Case | How Hypothesis Handles It | Gap? |
|-----------|---------------------------|------|
| [case 1] | [handling] | ✓/⚠ |
| [case 2] | [handling] | ✓/⚠ |
| [case 3] | [handling] | ✓/⚠ |

**Verdict:** [Edge cases covered / Gap in: X]

### 6. Weakest Link Reassessment

- **Original (from hypothesis):** [what was stated]
- **After deduction:** [updated assessment]
- **Change:** [Same / Revised because: X]
```

### 3. Verdict per Hypothesis

```markdown
### Verdict: [PASS / FAIL / CONDITIONAL]

**PASS** → Promote to L1
- Logically consistent
- No contradictions with L2 facts
- Assumptions are internally coherent
- Implications are acceptable

**CONDITIONAL** → Stays L0 with notes
- Consistent IF [condition]
- Needs clarification on [X]
- Minor concerns: [list]

**FAIL** → Move to invalid/
- Contradicts [specific L2 fact]
- Internal contradiction: [details]
- Logical flaw: [details]
- Unacceptable implication: [details]
```

### 4. Update Files

**If PASS:** Move hypothesis to L1

```bash
mv .fpf/knowledge/L0/[hyp].md .fpf/knowledge/L1/[hyp].md
```

Update the file's frontmatter:

```yaml
---
status: L1
deduction_passed: [timestamp]
deduction_notes: |
  Passed logical consistency check.
  Key finding: [brief summary]
  Ready for empirical verification.
---
```

**If FAIL:** Move to invalid

```bash
mv .fpf/knowledge/L0/[hyp].md .fpf/knowledge/invalid/[hyp].md
```

Add invalidation reason:

```yaml
---
status: invalid
invalidated: [timestamp]
invalidation_reason: |
  Failed deduction: [specific reason]
  Contradicts: [what]
  Learning: [what we learned from this failure]
---
```

**If CONDITIONAL:** Keep in L0, update notes

```yaml
---
status: L0
deduction_result: conditional
conditions_needed: |
  - [Condition 1 to clarify]
  - [Condition 2 to clarify]
---
```

### 5. Update Session

```markdown
## Status
Phase: DEDUCTION_COMPLETE

## Active Hypotheses
| ID | Hypothesis | Status | Deduction Result |
|----|------------|--------|------------------|
| h1 | [name] | L1 | ✓ PASS |
| h2 | [name] | L0 | ⚠ CONDITIONAL: needs X |
| h3 | [name] | invalid | ✗ FAIL: contradicts Y |

## Phase Transitions Log
| Timestamp | From | To | Trigger |
|-----------|------|-----|---------|
| [...] | ... | ... | ... |
| [now] | ABDUCTION_COMPLETE | DEDUCTION_COMPLETE | /fpf-2-check |

## Next Step
- `/fpf-3-test` to run internal empirical tests
- `/fpf-3-research` to gather external evidence
- (Both can be run; order doesn't matter)
```

## Output Format

```markdown
## Deduction Complete

### Results Summary

| Hypothesis | Result | Reason | New Status |
|------------|--------|--------|------------|
| H1: [name] | ✓ PASS | Consistent, no contradictions | L1 |
| H2: [name] | ⚠ CONDITIONAL | Needs clarification on [X] | L0 |
| H3: [name] | ✗ FAIL | Contradicts [L2 fact] | invalid |

### Key Findings

**H1 passed because:**
- [Key reason 1]
- [Key reason 2]

**H3 failed because:**
- [Specific contradiction or flaw]
- **Learning:** [What this teaches us]

### Files Updated
- `.fpf/knowledge/L1/[h1].md` (promoted)
- `.fpf/knowledge/invalid/[h3].md` (invalidated)

### Remaining Concerns
- [Any issues to watch during empirical testing]
- [Assumptions that need verification]

---

**Next Step:**
`/fpf-3-test` — Run internal empirical tests (code, benchmarks)
`/fpf-3-research` — Gather external evidence (docs, papers, web)

Both can be done; they complement each other.
```

## Common Deduction Failures

| Pattern | What It Means | Example |
|---------|---------------|---------|
| **Contradicts L2** | Hypothesis ignores verified knowledge | "Use X" when L2 says "X doesn't work for our scale" |
| **Circular reasoning** | A assumes B, B assumes A | "Fast because cached" + "Cache works because fast" |
| **Hidden assumption** | "This works if X" but X never stated | Assuming network is reliable without stating it |
| **Scale blindness** | Works for N=10, breaks at N=10000 | Algorithm is O(n²) but hypothesis claims "fast" |
| **Scope creep** | Claims broader applicability than justified | "Works for all cases" when only tested one |

## Common Mistakes to Avoid

| Mistake | Why It's Wrong | Do This Instead |
|---------|----------------|-----------------|
| Running code to "verify" | That's induction, not deduction | Analyze logic only in this phase |
| Passing everything | Defeats the purpose of filtering | Be rigorous; failing hypotheses is valuable |
| Skipping L2 check | May contradict verified knowledge | Always check against existing L2 facts |
| Vague verdicts | "Probably okay" isn't actionable | Be specific: PASS/FAIL/CONDITIONAL with reasons |
