---
description: "Finalize decision and create Design Rationale Record (DRR)"
arguments:
  - name: hypothesis
    description: "Winning hypothesis ID (if not obvious from context)"
    required: false
---

# FPF Phase 5: Decide (Finalize & Document)

## Your Role

Synthesize the ADI cycle into a **Design Rationale Record (DRR)** — a permanent, auditable decision document.

## Prerequisites

Before running this command, ensure:
- [ ] At least one hypothesis at L1+ status
- [ ] `/fpf:4-audit` completed (or explicitly skipped for low-stakes decisions)
- [ ] No unresolved blocker issues

## Process

### 1. Identify Winner

If multiple L2 hypotheses exist, present comparison to human:

```markdown
## Decision Point

### Candidates
| Hypothesis | Level | Strengths | Weaknesses |
|------------|-------|-----------|------------|
| H1: [name] | L2 | [+] | [-] |
| H2: [name] | L1 | [+] | [-] |

### Recommendation
Based on evidence, [H1] is recommended because:
- [Reason 1]
- [Reason 2]

**Awaiting your decision.** Which hypothesis to proceed with?
```

**Wait for human input** unless only one viable candidate.

### 2. Create DRR

Create `.fpf/decisions/DRR-[NNN]-[slug].md`:

```markdown
---
id: DRR-[NNN]
title: [Decision Title]
status: ACCEPTED
date: [timestamp]
decision_makers:
  - [Human — name/role if known]
  - [Claude — as advisor/analyst]
supersedes: [previous DRR if any]
---

# DRR-[NNN]: [Decision Title]

## Context

### Problem Statement
[Original problem from session.md]

### Trigger
[Why this decision was needed now]

### Constraints
- [Constraint 1]
- [Constraint 2]

## Decision

**We will:** [Clear statement of chosen approach]

**Based on:** [Winning hypothesis reference]

## Alternatives Considered

### [Alternative 1 — from hypotheses]
- **Status:** [L0/L1/L2/Invalid]
- **Summary:** [What it proposed]
- **Why rejected:** [Specific reason with evidence reference]

### [Alternative 2]
- **Status:** [L0/L1/L2/Invalid]
- **Summary:** [What it proposed]
- **Why rejected:** [Specific reason with evidence reference]

## Evidence Summary

| Claim | Evidence | Confidence |
|-------|----------|------------|
| [Key claim 1] | [evidence/file.md] | High/Med |
| [Key claim 2] | [evidence/file.md] | High/Med |

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| [Risk 1] | [H/M/L] | [H/M/L] | [Plan] |
| [Risk 2] | [H/M/L] | [H/M/L] | [Plan] |

## Validity Conditions

This decision remains valid WHILE:
- [Condition 1]
- [Condition 2]

**Re-evaluate IF:**
- [Trigger 1]
- [Trigger 2]

## Implementation Notes

[Any specific guidance for implementing this decision]

## Consequences

### Positive
- [Expected benefit 1]
- [Expected benefit 2]

### Negative
- [Accepted tradeoff 1]
- [Accepted tradeoff 2]

## References

- Hypotheses: `.fpf/knowledge/L*/[files]`
- Evidence: `.fpf/evidence/[files]`
- Related DRRs: [if any]
```

### 3. Promote Knowledge

Winning hypothesis becomes permanent knowledge:

```markdown
## Knowledge Promotion

The winning hypothesis assertions become project knowledge:

1. Key claims → remain in `.fpf/knowledge/L2/`
2. Update frontmatter to reference DRR:

```yaml
---
status: L2
decided_in: DRR-[NNN]
---
```
```

### 4. Archive Session

Move or update session:

```markdown
# FPF Session

## Status
Phase: DECIDED
Closed: [timestamp]

## Outcome
Decision: DRR-[NNN]
Hypothesis selected: [name]

## Cycle Statistics
- Duration: [X days/hours]
- Hypotheses generated: [N]
- Hypotheses invalidated: [N]
- Evidence artifacts: [N]
- Audit issues resolved: [N]

---

To start new cycle: `/fpf:1-hypothesize <new problem>`
```

### 5. Archive and Reset Session

Archive completed session for future reference:

```bash
# Create sessions directory if needed
mkdir -p .fpf/sessions

# Archive with date and problem slug
mv .fpf/session.md ".fpf/sessions/$(date +%Y-%m-%d)-[problem-slug].md"
```

Create fresh session.md:

```markdown
# FPF Session

## Status
Phase: INITIALIZED
Started: (none)
Problem: (none)

## Active Hypotheses
(none)

## Next Step
Run `/fpf:1-hypothesize <problem>` to begin new reasoning cycle.

## Previous Cycle
Completed: [date]
Decision: DRR-[NNN]
Archive: sessions/[filename].md
```

## Output Format

```markdown
## Decision Recorded

### DRR Created
`.fpf/decisions/DRR-[NNN]-[slug].md`

### Summary
**Decision:** [One sentence]
**Based on:** [Winning hypothesis] at L[X]
**Alternatives rejected:** [Count]

### Key Evidence
- [Most important evidence point]
- [Second most important]

### Validity
Re-evaluate if: [Primary trigger]

### Implementation
Ready to implement. Key considerations:
- [Note 1]
- [Note 2]

---

**FPF cycle complete.** 

Knowledge updated:
- `.fpf/knowledge/L2/[promoted].md`

To start new cycle: `/fpf:1-hypothesize <problem>`
To query knowledge: `/fpf:query <topic>`
```

## DRR Quality Checklist

| Quality | Check |
|---------|-------|
| Traceable | Can follow from decision → evidence → tests → hypotheses? |
| Complete | All considered alternatives documented? |
| Actionable | Clear what to implement? |
| Bounded | Validity conditions specified? |
| Reversible | Know when/how to revisit? |

## Anti-Patterns

| Anti-Pattern | Why It's Wrong |
|--------------|----------------|
| "Decided by Claude" | Violates Transformer Mandate |
| No rejected alternatives | Looks like rubber-stamping |
| Missing validity conditions | Decision becomes stale silently |
| No evidence references | Untraceable, unauditable |
