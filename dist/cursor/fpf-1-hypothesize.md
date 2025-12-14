---
description: "Generate hypotheses for a problem (FPF Abduction phase)"
arguments:
  - name: problem
    description: "Problem statement or question to investigate"
    required: true
---

# FPF Phase 1: Abduction (Hypothesis Generation)

## Phase Gate (MANDATORY)

**STOP. Verify phase before proceeding:**

1. Read `.fpf/session.md`, extract `Phase:` value
2. Check validity:

| Current Phase | Can Run? | Action |
|---------------|----------|--------|
| INITIALIZED | ✅ YES | Proceed |
| DECIDED | ✅ YES | Start new cycle |
| Any other | ❌ NO | Complete current cycle first |

**If blocked:** 
```
⛔ BLOCKED: Cannot start new hypothesis cycle.
Current phase: [PHASE] 
Complete current cycle with /fpf-5-decide or /fpf-discard first.
```

---

## Your Role

You are the **Abductor**. Generate multiple competing hypotheses, not one "best" solution.

**Critical:** You generate options. Human decides which to pursue. This is the Transformer Mandate.

## Input

Problem: `$ARGUMENTS.problem`

## Process

### 1. Load Context

- Read `.fpf/session.md` for any active context
- Read `.fpf/context.md` for project context (Tech Stack, Scale, Constraints)
- Read relevant project files to understand constraints
- Check `.fpf/knowledge/L2/` for verified facts that constrain solution space
- Check `.fpf/knowledge/invalid/` for approaches already disproven

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

Create **3-5 diverse hypotheses**. 

**MANDATORY DIVERSITY CHECK:**
- [ ] At least one **conservative/safe** approach (proven patterns, lower risk)
- [ ] At least one **innovative/novel** approach (newer techniques, higher potential)
- [ ] At least one **minimal/simple** approach (least complexity, fastest)

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
formality: [0-9] # F-Score (0=Sketch, 9=Proof)
novelty: [Conservative|Novel|Radical]
complexity: [Low|Medium|High]
author: Claude (generated), Human (to review)
scope:
  applies_to: "[conditions where this solution applies]"
  not_valid_for: "[conditions where this won't work]"
  scale: "[expected scale/size constraints]"
---

# Hypothesis: [Clear one-line statement]

## 1. The Method (Design-Time)
*This is the plan/recipe. (A.15 MethodDescription)*

### Proposed Approach
[2-3 sentences: what this solution proposes]

### Rationale
[Why this might work — the abductive reasoning]

### Implementation Steps
1. [Step 1]
2. [Step 2]
3. [Step 3]

### Expected Capability
- [Capability Claim 1]
- [Capability Claim 2]

## 2. The Validation (Run-Time)
*This section tracks the Work performed to verify this.*

### Plausibility Assessment

| Filter | Score | Justification |
|--------|-------|---------------|
| **Simplicity** | High/Med/Low | [Occam's razor — is this the simplest solution?] |
| **Explanatory Power** | High/Med/Low | [Does it resolve the core problem fully?] |
| **Consistency** | High/Med/Low | [Compatible with known facts in L2?] |
| **Falsifiability** | High/Med/Low | [Can we clearly disprove this?] |

**Plausibility Verdict:** [PLAUSIBLE / MARGINAL / IMPLAUSIBLE]

### Assumptions to Verify
- [ ] [Assumption 1 — must be true for this to work]
- [ ] [Assumption 2]
- [ ] [Assumption 3]

### Required Evidence
- [ ] **Internal Test:** [Verification test]
  - **Performer:** [Developer | AI Agent | CI Pipeline]
- [ ] **Research:** [External validation]
  - **Performer:** [Developer | AI Agent]

## Falsification Criteria
[What evidence would DISPROVE this hypothesis?]
- If [X happens], this approach fails
- If [Y is true], this won't work

## Estimated Effort
[Rough: hours/days/weeks]

## Weakest Link
[What's the riskiest assumption or component of this approach?]
```

### 4. Apply Plausibility Filters

After generating all hypotheses, rank them:

```markdown
## Plausibility Ranking

| Hypothesis | Simplicity | Explanatory | Consistency | Falsifiable | Overall |
|------------|------------|-------------|-------------|-------------|---------|
| H1: [name] | High | High | High | High | ⭐ Strong |
| H2: [name] | Med | High | Med | High | Good |
| H3: [name] | Low | Med | High | Med | Marginal |

**Recommendation for human review:**
- H1 appears strongest on plausibility filters
- H2 offers [specific advantage]
- H3 is worth keeping because [reason] despite lower scores
```

### 5. AWAIT HUMAN INPUT

**STOP HERE. Present hypotheses to human for review.**

```markdown
## Hypotheses Generated — Awaiting Your Review

I've generated [N] hypotheses for: "[problem]"

**Quick Summary:**
| ID | Hypothesis | Type | Plausibility | Weakest Link |
|----|------------|------|--------------|--------------|
| H1 | [name] | Conservative | Strong | [risk] |
| H2 | [name] | Innovative | Good | [risk] |
| H3 | [name] | Minimal | Marginal | [risk] |

**Your options:**
1. **Proceed** → Run `/fpf-2-check` to verify logical consistency
2. **Refine** → Ask me to adjust/add/remove hypotheses
3. **Provide context** → Share additional constraints or information

Which hypotheses should we carry forward to deduction phase?
```

**Wait for human response before updating session.**

### 6. Update Session (After Human Confirms)

Update `.fpf/session.md`:

```markdown
# FPF Session

## Status
Phase: ABDUCTION_COMPLETE
Started: [timestamp]
Problem: [problem statement]

## Active Hypotheses
| ID | Hypothesis | Status | Weakest Link | Human Approved |
|----|------------|--------|--------------|----------------|
| h1 | [name] | L0 | [risk] | ✓ |
| h2 | [name] | L0 | [risk] | ✓ |
| h3 | [name] | L0 | [risk] | ✓ |

## Phase Transitions Log
| Timestamp | From | To | Trigger |
|-----------|------|-----|---------|
| [prev] | — | INITIALIZED | /fpf-0-init |
| [now] | INITIALIZED | ABDUCTION_COMPLETE | /fpf-1-hypothesize |

## Next Step
Run `/fpf-2-check` to verify logical consistency.
Or `/fpf-2-check --hypothesis [id]` for specific hypothesis.
```

## Output Format

```markdown
## Hypotheses Generated

**Problem:** [restated problem]

### H1: [Name] (Conservative)
[One paragraph summary]
- **Plausibility:** Strong
- **Weakest link:** [X]
- **Falsifiable by:** [Y]

### H2: [Name] (Innovative)
[One paragraph summary]
- **Plausibility:** Good
- **Weakest link:** [X]
- **Falsifiable by:** [Y]

### H3: [Name] (Minimal)
[One paragraph summary]
- **Plausibility:** [score]
- **Weakest link:** [X]
- **Falsifiable by:** [Y]

---

**Files created:**
- `.fpf/knowledge/L0/[h1-slug].md`
- `.fpf/knowledge/L0/[h2-slug].md`
- `.fpf/knowledge/L0/[h3-slug].md`

---

**⏸ AWAITING YOUR INPUT**

Review the hypotheses above. You can:
- Approve all → proceed to `/fpf-2-check`
- Request modifications → tell me what to change
- Add constraints → share more context

What would you like to do?
```

## Common Mistakes to Avoid

| Mistake | Why It's Wrong | Do This Instead |
|---------|----------------|-----------------|
| Single "best" solution | Premature optimization without evidence | Generate 3-5 diverse options |
| All similar approaches | Limits learning, no real comparison | Force diversity: conservative + innovative + minimal |
| No falsification criteria | Unfalsifiable = untestable = useless | Every hypothesis must have clear disproval conditions |
| Vague assumptions | Can't verify what isn't concrete | Make assumptions explicit and testable |
| Proceeding without human review | Violates Transformer Mandate | Always pause for human confirmation |
| Ignoring L2 knowledge | May contradict verified facts | Check existing knowledge before generating |
