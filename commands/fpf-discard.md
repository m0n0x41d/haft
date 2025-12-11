---
description: "Discard current hypothesis cycle when empirical results invalidate the premise"
arguments:
  - name: reason
    description: "Brief reason for discarding (e.g., 'empirical test showed no benefit')"
    required: false
---

# FPF Discard: Abandon Current Cycle

## Purpose

Cleanly abandon a hypothesis cycle when:
- Empirical testing invalidates the core premise
- The problem turns out to be a non-problem
- A simpler solution emerges that bypasses the hypotheses entirely
- Circumstances changed, making the investigation irrelevant

**This is NOT failure** — discovering that a problem doesn't need solving is valuable knowledge.

## When to Use

| Scenario | Use Discard? | Alternative |
|----------|--------------|-------------|
| All hypotheses failed testing | ✓ Yes | — |
| Found simpler solution outside hypotheses | ✓ Yes | — |
| Problem was misdiagnosed | ✓ Yes | — |
| One hypothesis won clearly | ✗ No | Use `/fpf-5-decide` |
| Need to pause, continue later | ✗ No | Just stop, session persists |
| Want to start fresh on same problem | ✗ No | Use `/fpf-5-decide` with "no action" |

## Process

### 1. Confirm Discard

Present summary to user:

```markdown
## Discard Confirmation

**Current Session:**
- Problem: [from session.md]
- Phase: [current phase]
- Hypotheses: [count at each level]
- Evidence created: [count]

**Reason for discard:** $ARGUMENTS.reason

**What will happen:**
- Session archived to `.fpf/sessions/[date]-DISCARDED-[slug].md`
- L0 hypotheses: DELETED (unverified speculation)
- L1 hypotheses: DELETED (logically valid but empirically untested/failed)
- L2 hypotheses: KEPT (empirically verified — valuable knowledge)
- Invalid hypotheses: KEPT (valuable negative knowledge)
- Evidence files: KEPT (may be useful for future cycles)
- DRRs: Unchanged

**Proceed with discard?**
```

Wait for user confirmation unless `--force` or clear context.

### 2. Archive Session

```bash
# Create archive with DISCARDED marker
SLUG=$(echo "[problem]" | tr ' ' '-' | tr '[:upper:]' '[:lower:]' | cut -c1-30)
ARCHIVE_PATH=".fpf/sessions/$(date +%Y-%m-%d)-DISCARDED-${SLUG}.md"
```

Update session.md before archiving:

```markdown
# FPF Session (DISCARDED)

## Status
Phase: DISCARDED
Started: [original timestamp]
Discarded: [current timestamp]
Problem: [original problem]

## Discard Reason
[User-provided reason or inferred from context]

## What We Learned
[Extract key insights even from failed cycle:]
- [Insight 1 — e.g., "Gemini Vision direct is sufficient, no preprocessing needed"]
- [Insight 2 — e.g., "Problem was premature optimization"]

## Hypotheses at Discard

| ID | Level | Fate | Note |
|----|-------|------|------|
| [id] | L0 | Deleted | Never tested |
| [id] | L1 | Deleted | Logic valid, empirically moot |
| [id] | L2 | Kept | Verified knowledge |
| [id] | Invalid | Kept | Disproved |

## Evidence Created
[List evidence files — these persist]

## Statistics
- Duration: [X hours/days]
- Hypotheses generated: [N]
- Hypotheses tested: [N]
- Evidence artifacts: [N]
```

### 3. Clean Up Hypotheses

**Delete L0 and L1 from CURRENT CYCLE only:**

```bash
# Read session to get list of current cycle hypotheses
# Only delete those listed in session's "Active Hypotheses"

# Pattern: check hypothesis frontmatter for session_id or created date
# matching current session

# Safe deletion — only files from this cycle
rm .fpf/knowledge/L0/[current-cycle-files].md
rm .fpf/knowledge/L1/[current-cycle-files].md

# KEEP:
# - .fpf/knowledge/L2/* (empirically proven)
# - .fpf/knowledge/invalid/* (valuable negative knowledge)
# - .fpf/evidence/* (may inform future work)
```

### 4. Reset Session

Create fresh session.md:

```markdown
# FPF Session

## Status
Phase: INITIALIZED
Started: [new timestamp]
Problem: none

## Previous Cycle
Discarded: [date]
Problem: [what was discarded]
Reason: [brief reason]
Archive: sessions/[filename].md

## Notes
Ready for new hypothesis cycle.
```

### 5. Optional: Create Learning Note

If significant insights emerged, suggest creating evidence:

```markdown
## Suggested Learning Capture

The discarded cycle produced insights worth preserving:

**Insight:** [e.g., "Gemini Vision direct outperforms Paddle+Gemini"]

Want to create an evidence file to capture this? This prevents re-investigating the same dead end.

Suggested file: `.fpf/evidence/[topic]-empirical-finding.md`
```

## Output Format

```markdown
## Cycle Discarded

**Problem:** [original problem]
**Reason:** [discard reason]
**Duration:** [X hours/days]

### Cleanup Summary
| Category | Count | Action |
|----------|-------|--------|
| L0 hypotheses | [N] | Deleted |
| L1 hypotheses | [N] | Deleted |
| L2 hypotheses | [N] | Kept |
| Invalid | [N] | Kept |
| Evidence | [N] | Kept |

### Archived To
`.fpf/sessions/[filename].md`

### Key Learnings
- [Insight 1]
- [Insight 2]

---

**Session reset.** Ready for new cycle.

Next: `/fpf-1-hypothesize <new problem>`
```

## Examples

### Example 1: Empirical Test Invalidated Premise

```
User: /fpf-discard empirical test showed Gemini Vision direct is 3.5x faster with same accuracy

Output:
## Cycle Discarded

**Problem:** Should we add image preprocessing before PaddleOCR?
**Reason:** Empirical test showed Gemini Vision direct is 3.5x faster with same accuracy
**Duration:** 4 hours

### Key Learnings
- Gemini Vision handles low-quality scanned documents well without preprocessing
- Paddle adds latency (49s) and memory pressure without accuracy benefit
- Higher confidence scores don't guarantee better extraction

### Archived To
`.fpf/sessions/2025-12-12-DISCARDED-image-preprocessing.md`
```

### Example 2: Problem Turned Out to Be Non-Problem

```
User: /fpf-discard the performance issue was a logging misconfiguration, not algorithmic

Output:
## Cycle Discarded

**Problem:** API response time exceeds 500ms SLA
**Reason:** The performance issue was a logging misconfiguration, not algorithmic
**Duration:** 2 hours

### Key Learnings
- Always check observability overhead before algorithmic investigation
- Debug logging was synchronous and writing 10MB/request

### Archived To
`.fpf/sessions/2025-12-12-DISCARDED-api-performance.md`
```

## Anti-Patterns

| Anti-Pattern | Why It's Wrong | Do This Instead |
|--------------|----------------|-----------------|
| Discard without reason | Loses learning opportunity | Always capture why |
| Delete L2 knowledge | Destroys proven facts | L2 always survives |
| Delete evidence | May need for future cycles | Evidence always survives |
| Discard to avoid decision | Procrastination | Use `/fpf-5-decide` |
| Repeated discards on same problem | Indicates unclear problem definition | Step back, reframe problem |
