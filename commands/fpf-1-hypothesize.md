---
description: "Generate hypotheses for a problem (FPF Abduction phase)"
arguments:
  - name: problem
    description: "Problem statement or question to investigate"
    required: true
---

# FPF Phase 1: Abduction (Hypothesis Generation)

## Your Role

You are the **Abductor**. Generate multiple competing hypotheses, not one "best" solution.

## Input

Problem: $ARGUMENTS.problem

## Process

### 1. Load Context

- Read `.fpf/session.md` for any active context
- Read relevant project files to understand constraints
- Check `.fpf/knowledge/L2/` for verified facts that constrain solution space

### 2. Decompose the Problem

Before generating hypotheses, clarify:

```markdown
## Problem Decomposition

### Core Question
[What exactly needs to be decided/solved?]

### Constraints
- [Technical constraints from codebase]
- [Business/time constraints if known]
- [Dependencies on other systems]

### Success Criteria
- [How will we know a solution works?]
- [What must be true for success?]

### Out of Scope
- [What are we NOT solving here?]
```

### 3. Generate Hypotheses

Create **3-5 diverse hypotheses**. Ensure diversity:

- [ ] At least one conservative/safe approach
- [ ] At least one innovative/novel approach  
- [ ] At least one minimal/simple approach

For each hypothesis, create a file in `.fpf/knowledge/L0/`:

**Filename:** `[slug]-hypothesis.md`

**Content:**

```markdown
---
id: [slug]
type: hypothesis
created: [timestamp]
problem: [reference to problem]
status: L0
scope:
  applies_to: "[conditions where this solution applies]"
  not_valid_for: "[conditions where this won't work]"
  scale: "[expected scale/size constraints]"
---

# Hypothesis: [Clear one-line statement]

## Approach
[2-3 sentences: what this solution proposes]

## Rationale
[Why this might work — the abductive reasoning]

## Scope of Applicability
**This hypothesis applies when:**
- [Condition 1]
- [Condition 2]

**This hypothesis does NOT apply when:**
- [Condition 1]
- [Condition 2]

## Assumptions
- [ ] [Assumption 1 — must be true for this to work]
- [ ] [Assumption 2]
- [ ] [Assumption 3]

## Falsification Criteria
[What evidence would DISPROVE this hypothesis?]
- If [X happens], this approach fails
- If [Y is true], this won't work

## Estimated Effort
[Rough: hours/days/weeks]

## Weakest Link
[What's the riskiest part of this approach?]
```

### 4. Update Session

Update `.fpf/session.md`:

```markdown
# FPF Session

## Status
Phase: ABDUCTION_COMPLETE
Started: [timestamp]
Problem: [problem statement]

## Active Hypotheses
| ID | Hypothesis | Status | Weakest Link |
|----|------------|--------|--------------|
| h1 | [name] | L0 | [risk] |
| h2 | [name] | L0 | [risk] |
| h3 | [name] | L0 | [risk] |

## Next Step
Run `/fpf:2-check` to verify logical consistency.
Or `/fpf:2-check --hypothesis [id]` for specific hypothesis.
```

## Output Format

```markdown
## Hypotheses Generated

**Problem:** [restated problem]

### H1: [Name]
[One paragraph summary]
- Weakest link: [X]
- Falsifiable by: [Y]

### H2: [Name]
[One paragraph summary]
- Weakest link: [X]
- Falsifiable by: [Y]

### H3: [Name]
[One paragraph summary]
- Weakest link: [X]
- Falsifiable by: [Y]

---

**Files created:**
- `.fpf/knowledge/L0/[h1-slug].md`
- `.fpf/knowledge/L0/[h2-slug].md`
- `.fpf/knowledge/L0/[h3-slug].md`

**Next:** `/fpf:2-check` to verify logical consistency
```

## Anti-Patterns to Avoid

| Anti-Pattern | Why It's Wrong |
|--------------|----------------|
| Single "best" solution | Premature optimization; you don't have evidence yet |
| All similar approaches | Lack of diversity limits learning |
| No falsification criteria | Unfalsifiable hypotheses are useless |
| Vague assumptions | Can't verify what isn't concrete |
