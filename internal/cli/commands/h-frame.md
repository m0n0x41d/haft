---
description: "Frame an engineering problem before solving it"
---

# Frame Problem

Frame the actual problem before jumping to solutions. The bottleneck is problem quality, not solution speed.

## Investigation-first discipline (BEFORE asking the user)

Haft's bounded context is ONE repository. If the operator's signal
contains umbrella words ("service", "process", "ready", "stable",
"auth flow", "queue", etc.), DO NOT bounce back with "what do you
mean?" — the answer almost certainly already exists in the project.
Sweep the bounded context first:

1. Call `haft_query(action="resolve_term", term="<umbrella word>")`
   in ONE round-trip. It returns term-map entries + spec sections that
   reference the term + past artifact mentions (decisions/notes/
   problems). Read the `resolution` field:
   - `resolved` → use the canonical entry directly, do not ask.
   - `absent` → the term is not in the project's vocabulary; skip to
     step 4.
   - `ambiguous` → multiple candidates; jump to step 3.
2. Cross-check with `Glob` / `Grep` / `Read` if you need to ground
   the term in actual source code (e.g. "the auth service" — find the
   directory or package name, look at the structure).
3. If after investigation there is GENUINE ambiguity (multiple real
   referents, conflicting spec sections), ask the operator ONE
   concrete question that names the candidates you found. Bad: "what
   do you mean by 'service'?" Good: "I found `internal/auth/oauth/`
   and `internal/auth/sessions/` — which one is slow?"
4. If the term is load-bearing and absent, propose adding it to
   `.haft/specs/term-map.md` as a side-task, then frame with the
   working definition.

If `haft_problem(frame)` returns a `Project readiness` warning that the
project is `needs_onboard`, prefer running `/h-onboard` first so the
ProblemCard and any downstream decision can link to spec section refs.
Tactical exception: if the problem is urgent or exploratory, proceed and
mark the work as tactical so `haft spec coverage` will not later confuse
it with spec-driven work.

Use `haft_problem` tool with `action="frame"` and:
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
2. Create a decision record immediately (`/h-decide`) — even for trivial fixes
3. Implement
4. The decision closes the problem automatically

**Standard/deep mode** (multiple approaches, architectural impact, needs comparison):
1. Frame → `/h-char` → `/h-explore` → `/h-compare` → `/h-decide`
2. Then implement

**How to choose mode:**
- If you already know the fix and it touches ≤3 files → tactical
- If there are 2+ genuinely different approaches, or the blast radius is unclear → standard
- If unsure → ask the user: "This looks tactical — should I decide and implement directly, or do you want to explore variants?"

**The rule:** framing without a decision is an open wound. If you implement, you MUST have decided first.

$ARGUMENTS
