---
description: "Finalize decision and create Design Rationale Record (DRR)"
arguments:
  - name: hypothesis
    description: "Winning hypothesis ID (if not obvious from context)"
    required: false
---

# FPF Phase 5: Decide (Finalize & Document)

## Phase Gate (MANDATORY)

**STOP. Verify phase before proceeding:**

1. Read `.fpf/session.md`, extract `Phase:` value
2. Check validity:

| Current Phase | Can Run? | Action |
|---------------|----------|--------|
| AUDIT_COMPLETE | ✅ YES | Proceed |
| INDUCTION_COMPLETE | ⚠️ YES with WARNING | See below |
| Any other | ❌ NO | See below |

**If AUDIT_COMPLETE:** Proceed normally.

**If INDUCTION_COMPLETE (audit skipped):**
```
⚠️ WARNING: Proceeding without bias audit.

You are skipping /q4-audit. This means:
- No systematic WLNK calculation performed
- No bias check conducted  
- No adversarial analysis done
- Evidence quality not reviewed

This is ACCEPTABLE for:
- Low-stakes, easily reversible decisions
- Time-critical situations where audit overhead isn't justified

This is RISKY for:
- Architectural decisions with long-term consequences
- Decisions affecting multiple teams or systems
- Anything that's hard to reverse

Do you want to:
1. Proceed anyway → Continue with /q5-decide
2. Run audit first → /q4-audit

Awaiting your confirmation to proceed without audit.
```

**Wait for explicit human confirmation before proceeding.**

**If earlier phase:**
```
⛔ BLOCKED: Cannot decide without evidence.
Current phase: [PHASE]

Required steps:
- INITIALIZED → /q1-hypothesize
- ABDUCTION_COMPLETE → /q2-check  
- DEDUCTION_COMPLETE → /q3-test or /q3-research
- INDUCTION_COMPLETE → /q4-audit (recommended) or proceed with warning

Complete required phases first.
```

---

## Your Role

Synthesize the ADI cycle into a **Design Rationale Record (DRR)** — a permanent, auditable decision document.

**Critical Transformer Mandate:** You present options and evidence. **Human decides.** You do not make the final choice autonomously.

## Prerequisites Checklist

Before proceeding, verify:
- [ ] At least one hypothesis at L1+ status
- [ ] Evidence exists for key claims
- [ ] No unresolved blocker issues (if audit was run)
- [ ] Human is ready to make decision

## Process

### 1. Present Decision Point to Human

If multiple hypotheses remain viable, present comparison:

```markdown
## Decision Point — AWAITING YOUR INPUT

### Viable Candidates

| Hypothesis | Status | R_eff (WLNK) | Strengths | Weaknesses |
|------------|--------|--------------|-----------|------------|
| H1: [name] | L2 | 0.85 | [+] | [-] |
| H2: [name] | L1 | 0.70 | [+] | [-] |
| H3: [name] | L1 | 0.60 | [+] | [-] |

### Evidence Summary

**H1 evidence:**
- [evidence 1]: [finding]
- [evidence 2]: [finding]

**H2 evidence:**
- [evidence 1]: [finding]

### Trade-off Analysis

| Dimension | H1 | H2 | H3 |
|-----------|----|----|-----|
| Effort | [estimate] | [estimate] | [estimate] |
| Risk | [level] | [level] | [level] |
| Reversibility | [easy/hard] | [easy/hard] | [easy/hard] |
| WLNK R_eff | [value] | [value] | [value] |

### My Assessment

Based on evidence, **H1** appears strongest because:
- [Reason 1]
- [Reason 2]

However, **H2** might be preferred if:
- [Condition where H2 wins]

### Your Decision Required

**Which hypothesis should we proceed with?**

Options:
1. **H1: [name]** — [one-line summary]
2. **H2: [name]** — [one-line summary]  
3. **H3: [name]** — [one-line summary]
4. **None** — Need more information / Start new cycle

Please indicate your choice.
```

**STOP HERE. Wait for human response.**

### 2. Create DRR (After Human Decides)

Once human confirms choice, create `.fpf/decisions/DRR-[NNN]-[slug].md`:

```markdown
---
id: DRR-[NNN]
title: "[Decision Title]"
status: ACCEPTED
date: [timestamp]
decision_makers:
  - "[Human — name/role if known]"
  - "Claude — as analyst/advisor"
supersedes: [previous DRR if any, or "none"]
hypothesis_selected: "[hypothesis id]"
alternatives_rejected: ["[id1]", "[id2]"]
---

# DRR-[NNN]: [Decision Title]

## Executive Summary

**Decision:** [One sentence — what we decided]

**Based on:** [Winning hypothesis] with R_eff = [value]

**Key evidence:** [Most important supporting evidence]

## Context

### Problem Statement
[Original problem from session.md]

### Trigger
[Why this decision was needed now]

### Constraints
- [Constraint 1]
- [Constraint 2]

### Success Criteria
[How we'll know this decision was right]

## Decision

**We will:** [Clear statement of chosen approach]

**We will NOT:** [What we're explicitly not doing]

**Based on hypothesis:** `.fpf/knowledge/L2/[file].md` (or L1 if not fully verified)

## Alternatives Considered

### [Alternative 1 — from hypotheses]

- **Status:** [L0/L1/L2/Invalid]
- **Summary:** [What it proposed]
- **WLNK R_eff:** [value]
- **Why rejected:** [Specific reason with evidence reference]

### [Alternative 2]

- **Status:** [L0/L1/L2/Invalid]
- **Summary:** [What it proposed]
- **WLNK R_eff:** [value]
- **Why rejected:** [Specific reason with evidence reference]

## Evidence Summary

### Supporting Evidence

| Claim | Evidence | Type | Congruence | R_eff |
|-------|----------|------|------------|-------|
| [claim 1] | [evidence/file.md] | internal | — | 1.0 |
| [claim 2] | [evidence/file.md] | external | high | 1.0 |
| [claim 3] | [evidence/file.md] | external | medium | 0.85 |

### WLNK Calculation

```
Overall R_eff = min(all evidence R_eff) = [value]
Weakest link: [evidence file] because [reason]
```

### Evidence Gaps

[Any areas where evidence is weak or missing]

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation | Owner |
|------|------------|--------|------------|-------|
| [Risk 1] | H/M/L | H/M/L | [Plan] | [Who] |
| [Risk 2] | H/M/L | H/M/L | [Plan] | [Who] |

## Validity Conditions

This decision remains valid **WHILE:**
- [Condition 1]
- [Condition 2]
- Evidence remains fresh (check validity dates)

**Re-evaluate IF:**
- [Trigger 1 — e.g., "Scale exceeds 10k RPS"]
- [Trigger 2 — e.g., "New team members unfamiliar with X"]
- [Trigger 3 — e.g., "Dependency Y releases breaking change"]

## Implementation Notes

[Any specific guidance for implementing this decision]

### Immediate Actions
1. [Action 1]
2. [Action 2]

### Follow-up Items
- [ ] [Item 1]
- [ ] [Item 2]

## Consequences

### Expected Positive Outcomes
- [Benefit 1]
- [Benefit 2]

### Accepted Trade-offs
- [Trade-off 1]
- [Trade-off 2]

### Potential Negative Outcomes (Accepted Risks)
- [Risk 1] — Accepted because [reason]
- [Risk 2] — Mitigated by [plan]

## Audit Trail

### Reasoning Cycle
- **Problem defined:** [date]
- **Hypotheses generated:** [count] on [date]
- **Deduction completed:** [date] — [N] passed, [N] failed
- **Induction completed:** [date] — [N] evidence files created
- **Audit completed:** [date or "skipped"]
- **Decision finalized:** [date]

### Key Decisions During Cycle
- [Decision point 1]: [what was decided]
- [Decision point 2]: [what was decided]

## References

- **Session archive:** `.fpf/sessions/[date]-[slug].md`
- **Winning hypothesis:** `.fpf/knowledge/L2/[file].md`
- **Evidence files:** `.fpf/evidence/[files]`
- **Related DRRs:** [if any]
```

### 3. Promote Knowledge

The winning hypothesis becomes permanent knowledge:

```bash
# If not already in L2, move there
mv .fpf/knowledge/L1/[hyp].md .fpf/knowledge/L2/[hyp].md
```

Update frontmatter:

```yaml
---
status: L2
decided_in: DRR-[NNN]
decision_date: [timestamp]
---
```

### 4. Archive Session

Create archive of completed session:

```bash
mv .fpf/session.md ".fpf/sessions/$(date +%Y-%m-%d)-[problem-slug].md"
```

Update the archived session:

```markdown
# FPF Session (COMPLETED)

## Status
Phase: DECIDED
Started: [original timestamp]
Completed: [current timestamp]

## Outcome
- **Decision:** DRR-[NNN]
- **Hypothesis selected:** [name]
- **Alternatives rejected:** [count]

## Cycle Statistics
- Duration: [X days/hours]
- Hypotheses generated: [N]
- Hypotheses passed deduction: [N]
- Hypotheses invalidated: [N]
- Evidence artifacts: [N]
- Audit issues resolved: [N]

## Phase Transitions Log
[full log from session]
```

### 5. Create Fresh Session

Create new `.fpf/session.md`:

```markdown
# FPF Session

## Status
Phase: INITIALIZED
Started: [timestamp]
Problem: (none yet)

## Active Hypotheses
(none)

## Phase Transitions Log
| Timestamp | From | To | Trigger |
|-----------|------|-----|---------|
| [now] | — | INITIALIZED | (auto after DRR-[NNN]) |

## Previous Cycle
- **Completed:** [date]
- **Decision:** DRR-[NNN]
- **Archive:** `.fpf/sessions/[filename].md`

## Next Step
Run `/q1-hypothesize <problem>` to begin new reasoning cycle.
```

## Output Format

```markdown
## Decision Recorded

### DRR Created
`.fpf/decisions/DRR-[NNN]-[slug].md`

### Summary
- **Decision:** [One sentence]
- **Based on:** [Hypothesis] at L[X] with R_eff = [value]
- **Alternatives rejected:** [Count]

### Key Evidence
1. [Most important evidence point]
2. [Second most important]

### Validity
**Re-evaluate if:** [Primary trigger]

### Implementation
Ready to implement. Key actions:
1. [Action 1]
2. [Action 2]

---

**FPF cycle complete.**

**Files created/updated:**
- `.fpf/decisions/DRR-[NNN]-[slug].md` (new)
- `.fpf/knowledge/L2/[hypothesis].md` (promoted)
- `.fpf/sessions/[date]-[slug].md` (archived)
- `.fpf/session.md` (reset for new cycle)

**Next steps:**
- Implement the decision
- Start new cycle: `/q1-hypothesize <problem>`
- Query knowledge: `/q-query <topic>`
```

## DRR Quality Checklist

| Quality | Check | ✓/✗ |
|---------|-------|-----|
| **Traceable** | Can follow from decision → evidence → tests → hypotheses? | |
| **Complete** | All considered alternatives documented? | |
| **Actionable** | Clear what to implement? | |
| **Bounded** | Validity conditions specified? | |
| **Reversible** | Know when/how to revisit? | |
| **Attributed** | Human decision-maker identified? | |

## Common Mistakes to Avoid

| Mistake | Why It's Wrong | Do This Instead |
|---------|----------------|-----------------|
| "Decided by Claude" | Violates Transformer Mandate | Always attribute to human |
| No rejected alternatives | Looks like rubber-stamping | Document all considered options |
| Missing validity conditions | Decision becomes stale silently | Specify re-evaluation triggers |
| No evidence references | Untraceable, unauditable | Link to specific evidence files |
| Proceeding without human choice | Autonomous decision-making | Always wait for explicit human selection |
| Skipping audit without acknowledgment | Hidden risk | Explicitly note if audit was skipped |
