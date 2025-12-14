---
description: FPF Discard: Abandon Current Cycle
argument-hint: (optional)
---

---
description: "Discard current hypothesis cycle when empirical results invalidate the premise"
arguments:
  - name: reason
    description: "Brief reason for discarding (e.g., 'empirical test showed no benefit')"
    required: false
  - name: preserve
    description: "Preserve level: all, L1+, L2-only, none (default: L2-only)"
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
| Circumstances changed, investigation irrelevant | ✓ Yes | — |
| One hypothesis won clearly | ✗ No | Use `/q5-decide` |
| Need to pause, continue later | ✗ No | Just stop, session persists |
| Want to start fresh on same problem | ✗ No | Use `/q5-decide` with learnings |

## Process

### 1. Confirm Discard

Present summary to user for confirmation:

```markdown
## Discard Confirmation — AWAITING YOUR INPUT

**Current Session:**
- **Problem:** [from session.md]
- **Phase:** [current phase]
- **Duration:** [time since started]

**Hypotheses Status:**
| Level | Count | Examples |
|-------|-------|----------|
| L0 (unverified) | [N] | [names] |
| L1 (logic-verified) | [N] | [names] |
| L2 (empirically verified) | [N] | [names] |
| Invalid | [N] | [names] |

**Evidence Created:** [N] files

**Reason for discard:** [user-provided or inferred]

---

### What Will Happen

| Category | Default Action | Rationale |
|----------|---------------|-----------|
| L0 hypotheses | **DELETED** | Unverified speculation |
| L1 hypotheses | **DELETED** | Logically valid but untested |
| L2 hypotheses | **KEPT** | Empirically verified — valuable |
| Invalid hypotheses | **KEPT** | Valuable negative knowledge |
| Evidence files | **KEPT** | May inform future cycles |
| Session | **ARCHIVED** | Historical record |

### Preservation Options

You can change what gets preserved:

| Option | Keeps | Deletes |
|--------|-------|---------|
| `--preserve L2-only` (default) | L2, Invalid, Evidence | L0, L1 |
| `--preserve L1+` | L1, L2, Invalid, Evidence | L0 |
| `--preserve all` | Everything | Nothing |
| `--preserve none` | Invalid, Evidence | L0, L1, L2 |

---

**Proceed with discard?**

Options:
1. **Yes, discard** — Archive session, clean up as specified
2. **Change preservation** — Tell me what to keep
3. **Cancel** — Return to current session

Awaiting your confirmation.
```

**Wait for explicit human confirmation.**

### 2. Capture Learnings

Before cleanup, extract key insights:

```markdown
## Learnings from Discarded Cycle

**Problem attempted:** [original problem]

**Why discarded:** [reason]

**Key insights gained:**
1. [Insight 1 — e.g., "Gemini Vision direct is sufficient, no preprocessing needed"]
2. [Insight 2 — e.g., "Problem was premature optimization"]
3. [Insight 3 — e.g., "Constraint X was incorrectly assumed"]

**Hypotheses that taught us something:**
| Hypothesis | What We Learned |
|------------|-----------------|
| [H1] | [learning] |
| [H2] | [learning] |

**Evidence worth preserving:**
- [evidence file] — [why valuable for future]

**Don't repeat:**
- [Mistake 1 — what not to do again]
- [Mistake 2]
```

### 3. Archive Session

Create archive with DISCARDED marker:

```bash
SLUG=$(echo "[problem]" | tr ' ' '-' | tr '[:upper:]' '[:lower:]' | cut -c1-30)
ARCHIVE_PATH=".fpf/sessions/$(date +%Y-%m-%d)-DISCARDED-${SLUG}.md"
```

Archive content:

```markdown
# FPF Session (DISCARDED)

## Status
Phase: DISCARDED
Started: [original timestamp]
Discarded: [current timestamp]
Problem: [original problem]

## Discard Reason
[User-provided reason]

## What We Learned

### Key Insights
1. [Insight 1]
2. [Insight 2]
3. [Insight 3]

### Don't Repeat
- [Mistake to avoid]

## Hypotheses at Discard

| ID | Name | Level | Fate | Notes |
|----|------|-------|------|-------|
| h1 | [name] | L0 | Deleted | Never tested |
| h2 | [name] | L1 | Deleted | Logic valid, empirically moot |
| h3 | [name] | L2 | Kept | Verified knowledge |
| h4 | [name] | Invalid | Kept | Disproved — valuable |

## Evidence Created
[List evidence files that were kept]

## Statistics
- Duration: [X hours/days]
- Hypotheses generated: [N]
- Hypotheses reached L1: [N]
- Hypotheses reached L2: [N]
- Hypotheses invalidated: [N]
- Evidence artifacts: [N]

## Phase Transitions Log
[Full log from session]
```

### 4. Clean Up (Based on Preservation Setting)

**Default (L2-only):**
```bash
# Delete L0 and L1 from THIS CYCLE only
rm .fpf/knowledge/L0/[current-cycle-files].md
rm .fpf/knowledge/L1/[current-cycle-files].md

# Keep:
# - .fpf/knowledge/L2/* (empirically proven)
# - .fpf/knowledge/invalid/* (valuable negative knowledge)
# - .fpf/evidence/* (may inform future work)
```

**Important:** Only delete files from the CURRENT cycle (check `created` date or session reference in frontmatter).

### 5. Create Learning Note (Optional)

If significant insights emerged, offer to create permanent learning:

```markdown
## Suggested Learning Capture

The discarded cycle produced insights worth preserving permanently.

**Create evidence file?**

Proposed: `.fpf/evidence/[date]-[topic]-learning.md`

```yaml
---
id: [topic]-learning
type: meta-learning
source: discarded-cycle
created: [timestamp]
from_session: [archived session path]
---

# Learning: [Topic]

## Context
[What we were investigating and why we stopped]

## Key Finding
[The main insight]

## Implications
[How this affects future decisions]

## Don't Repeat
[What to avoid]
```

Create this learning note? (y/n)
```

### 6. Reset Session

Create fresh session:

```markdown
# FPF Session

## Status
Phase: INITIALIZED
Started: [timestamp]
Problem: (none)

## Previous Cycle
- **Discarded:** [date]
- **Problem:** [what was discarded]
- **Reason:** [brief reason]
- **Archive:** `.fpf/sessions/[filename].md`
- **Learnings:** [count] insights captured

## Next Step
Run `/q1-hypothesize <problem>` to begin new reasoning cycle.
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

### Key Learnings Captured
1. [Insight 1]
2. [Insight 2]

### Learning Note Created
`.fpf/evidence/[date]-[topic]-learning.md`

---

**Session reset.** Ready for new cycle.

Next: `/q1-hypothesize <new problem>`
```

## Examples

### Example 1: Empirical Test Invalidated Premise

```
User: /q-reset "empirical test showed Gemini Vision direct is 3.5x faster with same accuracy"

Output:
## Cycle Discarded

**Problem:** Should we add image preprocessing before PaddleOCR?
**Reason:** Empirical test showed Gemini Vision direct is 3.5x faster with same accuracy
**Duration:** 4 hours

### Key Learnings Captured
1. Gemini Vision handles low-quality scanned documents well without preprocessing
2. Paddle adds latency (49s) and memory pressure without accuracy benefit
3. Higher confidence scores don't guarantee better extraction

### Archived To
`.fpf/sessions/2025-01-15-DISCARDED-image-preprocessing.md`
```

### Example 2: Problem Turned Out to Be Non-Problem

```
User: /q-reset "the performance issue was a logging misconfiguration, not algorithmic"

Output:
## Cycle Discarded

**Problem:** API response time exceeds 500ms SLA
**Reason:** The performance issue was a logging misconfiguration, not algorithmic
**Duration:** 2 hours

### Key Learnings Captured
1. Always check observability overhead before algorithmic investigation
2. Debug logging was synchronous and writing 10MB/request

### Don't Repeat
- Jumping to algorithmic solutions before checking infrastructure
```

## Common Mistakes to Avoid

| Mistake | Why It's Wrong | Do This Instead |
|---------|----------------|-----------------|
| Discard without reason | Loses learning opportunity | Always capture why |
| Delete L2 knowledge | Destroys proven facts | L2 always survives |
| Delete evidence | May need for future cycles | Evidence always survives |
| Discard to avoid decision | Procrastination | Use `/q5-decide` |
| Repeated discards on same problem | Indicates unclear problem | Step back, reframe problem |
| No learning capture | Waste of effort | Extract at least one insight |
