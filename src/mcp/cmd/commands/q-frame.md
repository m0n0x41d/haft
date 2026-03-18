---
description: "Frame an engineering problem before solving it"
---

# Frame Problem

Frame the actual problem before jumping to solutions. The bottleneck is problem quality, not solution speed.

Use `quint_problem` tool with `action="frame"` and:
- `title`: problem title
- `signal`: what's anomalous, broken, or needs changing (required)
- `constraints`: hard limits that MUST hold
- `optimization_targets`: what to improve (1-3 max)
- `observation_indicators`: what to monitor but NOT optimize
- `acceptance`: how we'll know it's solved
- `blast_radius`: what systems/teams are affected
- `reversibility`: how easy to undo (low/medium/high)
- `mode`: tactical, standard (default), deep
- `context`: grouping tag

## After framing — what comes next

Every problem needs a decision record before implementation. No exceptions.

**Tactical mode** (fix is obvious, low blast radius, easily reversible):
1. Frame the problem
2. Create a decision record immediately (`/q-decide`) — even for trivial fixes
3. Implement
4. The decision closes the problem automatically

**Standard/deep mode** (multiple approaches, architectural impact, needs comparison):
1. Frame → `/q-char` → `/q-explore` → `/q-compare` → `/q-decide`
2. Then implement

**How to choose mode:**
- If you already know the fix and it touches ≤3 files → tactical
- If there are 2+ genuinely different approaches, or the blast radius is unclear → standard
- If unsure → ask the user: "This looks tactical — should I decide and implement directly, or do you want to explore variants?"

**The rule:** framing without a decision is an open wound. If you implement, you MUST have decided first.

$ARGUMENTS
