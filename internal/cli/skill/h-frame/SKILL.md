---
name: h-frame
description: |
  Frames an engineering problem before any solution is explored — stabilizes the signal, names what is actually broken, declares acceptance criteria, and records a ProblemCard. Make sure to use this skill whenever the user proposes a refactor, rewrite, redesign, restructure, or rebuild without first naming the underlying problem or what acceptance looks like; whenever they say "let's rebuild X", "switch from Y to Z", "we should restructure", "I want to change A to B", "refactor this", "let's redo this" without stating success criteria; whenever a proposed solution arrives before the problem is defined; whenever scope is fuzzy and acceptance is unstated. Also catches explicit framing intent — "let me think about X", "before we solve this", "what's actually going on", "I want to understand X first". For broken tests or failing code with unclear cause prefer h-diagnose. For micro-decisions with rationale use h-note.
when_to_use: |
  Operator is about to commit to a direction before the problem is honestly named, OR is explicitly framing. If the signal is a concrete failure with unclear cause, prefer h-diagnose. For one-line micro-decisions use h-note.
argument-hint: "[problem signal text — what's anomalous, broken, or needs changing]"
allowed-tools: Bash mcp__haft__haft_problem mcp__haft__haft_query mcp__haft__haft_refresh
---

# h-frame — Frame the problem before solving

You are framing a problem via `mcp__haft__haft_problem(action="frame", ...)`. Problem quality dominates solution quality. Get the frame right; the rest follows.

## Compact interface discovery

If you need the exact compact contract, run `haft interface problem.frame --json`.
Use it as discovery; do not paste long MCP schemas or CLI help into the
session. For large payloads prefer:

```bash
haft artifact create problem.frame --input-file <input.json> --json
```

`mcp__haft__haft_problem(action="frame", ...)` remains the compatible fallback.

## Procedure

### Step 0 — Maintenance check (FPF B.3.4 evidence decay)

Before framing a NEW problem, check the most recent kernel response
for `Refresh reminder: N days since last stale scan`. If N > 30,
call `mcp__haft__haft_refresh(action="scan")` first. Stale or
drifted artifacts may already touch the area this new problem
operates in — discovering them after framing wastes the reasoning,
or worse, lets you frame a problem that's already been decided and
just decayed silently.

If the scan surfaces a conflict (stale decision in same module,
unresolved drift on overlapping files) — surface it inline and ask
the operator whether to verify the existing artifact first or
proceed with the new frame anyway. Do not silently re-frame around
obsolete artifacts.

See CLAUDE.md Critical Reminders — maintenance discipline.

### Step 1 — Stabilize the signal (FPF B.4.1)

Before writing anything, separate what the operator OBSERVED from what they ASSUMED. The signal is the anomaly/opportunity/probe; not the diagnosis.

Bad signal: "we need a new queue"
Good signal: "webhook retries hit 15% over baseline 2% since 2026-05-20"

### Step 2 — Detect umbrella words (FPF A.6.P/Q/A)

Scan the signal for umbrella terms: "quality", "service", "scalable", "maintainable", "simple", "stable", "ready", "process", "function", "component". Each needs precision restoration:
- "service is slow" → which service: the OAuth provider, the token endpoint, the session store?
- "quality is bad" → quality of what: maintainability (deps?), performance (p99?), readability (cyclomatic?)?
- "scalable" → scalable in what dimension: throughput? concurrent users? data size?

Ask one clarifying question if needed, or unpack inline with explicit re-statement.

### Step 3 — Classify the problem type (FPF FRAME-05)

One of:
- **optimization** — known-working system, want it better on a known dimension
- **diagnosis** — something's broken, root cause unclear → consider rerouting to h-diagnose
- **search** — need to find something that doesn't exist yet
- **synthesis** — combine existing elements into something new

Pass to kernel as `problem_type` field.

### Step 4 — Declare scope explicitly (FPF FRAME-02)

What's in-scope AND what's out-of-scope. Prevents silent scope inflation downstream.

### Step 5 — Declare acceptance criteria (FPF FRAME-03)

What observable condition signals "solved"? "Better" is not acceptance; "p95 webhook retry rate < 3% measured over 7-day window" is.

**Draft inline — do not delegate authorship back to the operator.** Read the signal carefully, propose a concrete observable acceptance condition based on the metric/behavior most directly tied to the signal. Then surface the draft to the operator with "I'm using this acceptance — edit if it's wrong". Operator review is the gate; operator authorship is not. Asking "what's your acceptance?" instead of drafting one defeats the value of the agent.

### Step 6 — Declare constraints (hard limits)

What no candidate variant may violate. Constraint role per FPF CHR-01.

### Step 7 — Optionally declare optimization targets + observation indicators

- `optimization_targets` (1-3 max) — what to optimize
- `observation_indicators` — watch but do NOT optimize (Anti-Goodhart per FPF CHR-01)

### Step 8 — Set mode based on blast radius (let the floor recommend it)

Don't default to `standard` by reflex. When you know the files the change will
touch, get a risk-proportioned recommendation first:

```
mcp__haft__haft_query(action="ceremony", files=["path/a.go", "db/0007.sql", ...])
```

It returns a recommended mode from path/content + governance signals, with a
deterministic safety floor: a HIGH-risk change (migration/sql, auth/secrets,
public-API, infra, destructive content, low-reversibility governance) is NEVER
recommended tactical, and when it can't tell it asks ONE question instead of
defaulting low. Use the recommendation — you can override it in one step. The
modes:

- `tactical` — reversible <2-week blast radius; minimum ceremony
- `standard` (default) — most architectural decisions
- `deep` — irreversible / security / cross-team / cross-repo; parity_plan required for downstream compare

### Step 9 — Call the kernel

```
mcp__haft__haft_problem(
  action="frame",
  title="<short problem title>",
  signal="<stabilized observation, not assumed cause>",
  problem_type="optimization|diagnosis|search|synthesis",
  acceptance="<observable solved-condition>",
  constraints=["<hard limit 1>", "<hard limit 2>"],
  optimization_targets=["<target 1>"],
  observation_indicators=["<Anti-Goodhart watch 1>"],
  blast_radius="<what systems/teams affected>",
  reversibility="low|medium|high",
  seed_file="<file the framing is about, if any>",
  mode="tactical|standard|deep"
)
```

When the problem is about a concrete file, pass `seed_file`: the response then
appends the artifacts the fused code+reasoning graph ranks nearest it — surfacing
a decision that already governs that code but which keyword recall would miss
(phrased differently). Check it before re-deciding what may already be decided.

### Step 10 — On success

Kernel returns ProblemCard ID (e.g. `prob-20260525-...`). Present to operator with suggested next steps:
- `/h-explore` (or h-diagnose if diagnosis-typed) to generate candidate solutions
- `/h-status` to see this problem in project FPF state
- For richer dimensions later: characterize via `mcp__haft__haft_problem(action="characterize", problem_ref=..., dimensions=[...])`

**Re-grounding discipline (FPF A.7).** When you reference the new
ProblemCard ID in subsequent operator-facing text, pair it with the
problem title or one-line signal — `prob-20260525-abc (cache-invalidation
flakes in schema-migration suite)` not bare `prob-20260525-abc`. Bare IDs
accumulate cognitive debt across long sessions. Keep the ID (needed for
follow-up kernel calls and traceability) but never let it stand alone.
See CLAUDE.md Critical Reminders for the project-wide rule.

## What NOT to do

- Do not jump from signal to solution; frame first.
- Do not record an assumed cause as the signal ("we need X" is the proposed solution, not the problem).
- Do not skip problem typing — it determines which exploration method applies.
- Do not silently substitute an acceptance you invented as if the operator chose it — DRAFT it explicitly and let them review. Drafting and labeling as DRAFT is the correct move; asking "what acceptance?" instead of drafting is delegation back, which defeats the agent's value.
- Do not record the problem if the operator's intent looks like a tactical edit — recommend `/h-note` or direct action instead.
- Do not auto-trigger when the operator is clearly already framing in chat without asking to record — wait for explicit intent to persist.

## Routing back upstream

If during framing you discover this is actually:
- A diagnosis problem with parallel hypothesis space → recommend `/h-diagnose` instead
- A choice between already-named alternatives → recommend `/h-compare` instead
- A bounded autonomous task → recommend `/h-commission` (after the frame is recorded)

## FPF spec references

- B.4.1 — Observe → Notice → Stabilize → Route (pre-articulation discipline)
- B.5.2 — Abductive loop (the upstream of decide)
- FRAME-01 through FRAME-09 — framing micro-patterns
- CHR-01 — Indicator role taxonomy (constraint / target / observation)
- A.6.P/Q/A — precision restoration for umbrella terms

Look up via `mcp__haft__haft_query(action="fpf", query="B.4.1")`.
