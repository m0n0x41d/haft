---
name: h-reason
description: "Think before building. Use when the user asks to reason about, analyze, evaluate, compare options, make an architecture decision, choose between approaches, think through a problem, or assess trade-offs. Also use when the user asks 'why did we...', 'should we...', 'what are our options', 'is this the right approach', or wants to frame/reframe a problem."
argument-hint: "[problem, decision, architecture question, trade-off, or 'what's stale?']"
---

# Quint Reasoning — Think Before You Build

This skill activates structured engineering reasoning powered by FPF (First Principles Framework) and Quint Code.

**When to use**: any non-trivial engineering question. Architecture choices, library selection, API design, data model changes, infrastructure decisions, process changes. Also: when the user asks to "think about", "reason about", "evaluate", or "compare" anything significant.

**When NOT to use**: obvious bug fixes, formatting, tiny refactors with clear acceptance.

---

## Context-aware entry — read what the user actually wants

**Before doing anything, assess the user's intent from context and arguments.** Do NOT always fall through into the full FPF cycle. There are three distinct paths:

### Path 1: Think and respond (no artifacts)
**Trigger:** "think about X", "what do you think about X", "analyze X", "is this the right approach?", "what are our options?"

The user wants structured thinking, not tool calls. Reason through the problem using FPF principles (weakest link, parity, distinguish object/description/carrier, etc.). Give a well-structured answer. **Do not call quint tools** unless the user explicitly asks to persist something.

### Path 2: Prepare for human-driven cycle (research + wait)
**Trigger:** "/h-reason [topic], prepare for framing", "let's think about X before deciding", "I want to reason through X"

The user wants to drive the cycle themselves. Gather context (read relevant code, search existing decisions, research). Present findings. **Stop and wait** for the user to decide the next step — they will call `/h-frame`, `/h-char`, etc. when ready.

### Path 3: Full autonomous cycle (agent drives)
**Trigger:** "/h-reason [topic] and implement", "figure out the best approach and do it", "fix everything", explicit delegation to agent

The user wants the agent to run the full cycle: frame → explore → decide → implement. Only in this mode does the agent drive without pausing.

**If unclear which path:** default to Path 2 (prepare and wait). Never default to Path 3. Ask: "Want me to think this through and present options, or drive the full cycle and implement?"

---

## What you have

### Quint tools (MCP) — persist reasoning as artifacts

| Tool | What it does | Slash command |
|------|-------------|---------------|
| `haft_note` | Record micro-decisions with rationale validation | `/h-note` |
| `haft_problem` | Frame problems; characterization is currently done inline in reasoning | `/h-frame`, `/h-char` |
| `haft_solution` | Explore variants, compare and identify Pareto front | `/h-explore`, `/h-compare` |
| `haft_decision` | Decide with formal rationale; record measurement results | `/h-decide` |
| `haft_refresh` | Detect stale decisions, manage lifecycle | `/h-refresh` |
| `haft_query` | Search, status dashboard, file-to-decision lookup | `/h-search`, `/h-status` |

### FPF spec search — deep methodology reference

```bash
haft fpf search "<query>"        # keyword search
haft fpf search "<query>" --full # full section content
haft fpf section "<heading>"     # exact section
```

Use for formal FPF definitions, templates, aggregation rules, or conformance checklists.

---

## Feature maturity

Not all FPF concepts are at the same implementation depth. This matters — don't present tracked annotations as computed evaluations.

| Concept | Status | What Quint does |
|---------|--------|-----------------|
| Problem framing | **tracked** | Stores signal, constraints, targets, acceptance. You do the framing, Quint persists it. |
| Characterization | **tracked** | Stores comparison dimensions with scale/unit/polarity/role on the ProblemCard. Persisted via `haft_problem(action="characterize")`. |
| WLNK | **tracked** | Required label on variants. Quint stores the stated weakest link on explored variants and decisions. |
| Parity | **textual** | Stored as rules text. Not enforced or verified. You ensure parity yourself. |
| Pareto front | **tracked** | You identify the non-dominated set, Quint stores and displays it. Not computed automatically. |
| Stepping stones | **tracked** | Boolean flag on variants, shown in summary table. |
| Refresh (valid_until) | **enforced** | All artifacts (decisions, problems, portfolios) with expired valid_until detected by scan. |
| Refresh triggers | **textual** | Stored in decision body. Only valid_until date is actually scanned. Text triggers are reminders, not automated checks. |
| CL (congruence) | **artifact-level** | Reliability/evidence calculations exist in artifact/runtime code, but not as a `haft_decision` action exposed in the current haft tool schema. |
| F-G-R | **textual** | Formality labels may exist in artifact data, but are not a first-class haft decision-tool workflow step. |
| NQD | **absent** | Multi-dimensional quality vectors are not implemented. Use comparison dimensions in exploration/compare instead. |
| Impact measurement | **tracked** | `haft_decision(action="measure")` records post-implementation findings against acceptance criteria. |
| Evidence attachment | **not exposed here** | Do not instruct agents to call `haft_decision(action="evidence")`; that action is not available in the current haft tool schema. |

**Key rule: don't describe textual features as if they compute something.** When you say "WLNK bounds quality," you mean "the user identified what bounds quality" — not that the system calculated it.

---

## How to reason

### 1. Frame the problem BEFORE solving it

The bottleneck is **problem quality**, not solution speed. Don't just fill in fields — drive a diagnostic conversation.

**Framing protocol — ask these questions before recording:**

1. **"What observation doesn't fit?"** — the signal, not the assumed cause. "Webhook retries hit 15%" not "we need a new queue."
2. **"What have you already tried?"** — avoids re-treading known dead ends.
3. **"Who owns this problem?"** — establishes the principal. Not "the team" — a specific person with authority.
4. **"What would solved look like?"** — forces measurable acceptance, not vague "it should be better."
5. **"What constraints are non-negotiable?"** — hard limits that no variant can violate.
6. **"How reversible is this? What's the blast radius?"** — determines mode (tactical vs standard vs deep).
7. **"What should we watch but NOT optimize?"** — Anti-Goodhart indicators that prevent reward hacking.

Only after these are answered, persist the ProblemCard:

- **Signal**: the anomalous observation (from question 1)
- **Constraints**: hard limits (from question 5)
- **Optimization targets**: 1-3 max (from question 4)
- **Observation indicators**: monitor-only metrics (from question 7)
- **Blast radius / reversibility**: from question 6
- **Acceptance**: measurable "done" criteria (from question 4)

**Persist with**: `haft_problem(action="frame", ...)`

**Goldilocks check**: When multiple problems are active, use `haft_problem(action="select")` to see them with blast radius and reversibility signals. Pick the problem in the growth zone — not too trivial (low impact), not too impossible (exceeds current capacity).

> RAG: `haft fpf search "problem card PROB"`

### 2. Characterize before comparing

Define the **characteristic space** before evaluating options. Without explicit dimensions, comparisons are arbitrary.

- State the **selection policy BEFORE seeing results**
- Ensure **parity** — same inputs, same scope, same budget across all options (you enforce this, Quint stores your parity rules)
- Keep it **multi-dimensional** — never collapse to a single score unless the fold is explicit

**Persisting is not available as a separate step in the current runtime.** Define dimensions in reasoning, then include them in `haft_solution(action="compare", ...)`.

> RAG: `haft fpf search "characterization CHR"`

### 3. Generate genuinely distinct variants

- **≥2 variants** that differ in **kind**, not degree (3+ preferred)
- Each variant gets a **weakest link** label — what you judge bounds its quality
- Mark **stepping stones** — options that open future possibilities even if not optimal now

**Persist with**: `haft_solution(action="explore", ...)`

> RAG: `haft fpf search "NQD variant quality"`

### 4. Compare and select

- Identify the **non-dominated set** — variants not strictly worse on all dimensions
- Apply the pre-declared selection policy
- Record what was compared, what won, and why

Note: Quint stores your comparison results and Pareto front. It does not compute them for you — you (or the agent doing analysis) determine which variants are non-dominated.

**Persist with**: `haft_solution(action="compare", ...)`

> RAG: `haft fpf search "selection policy SEL Pareto"`

### 5. Decide with full rationale

The decision record should contain:
- **Invariants** — what MUST hold at all times
- **Post-conditions** — checklist for implementation completion
- **Admissibility** — what is NOT acceptable
- **Valid-until date** — when to re-evaluate automatically
- **Weakest link** — your assessment of what bounds reliability

**Persist with**: `haft_decision(action="decide", ...)`

### 6. Detect staleness and refresh

All artifacts (decisions, problems, portfolios) with expired `valid_until` dates are automatically detected by `/h-status` and `/h-refresh`. This includes stale ProblemCards — problem framings can go stale too when context changes.

Text-based refresh triggers (e.g., "re-evaluate if throughput >80k/s") are stored as reminders but not automatically checked — you or the agent must notice when conditions change.

When reopening a stale decision, the new ProblemCard inherits lineage: prior characterization, failure reason, and evidence references from the previous cycle.

**Persist with**: `haft_refresh(...)`

---

## Depth calibration

| Mode | When | Flow |
|------|------|------|
| **note** | Micro-decisions during coding | `/h-note` — done |
| **tactical** | Reversible, <2 weeks blast radius | `/h-frame` → `/h-decide` |
| **standard** | Most architectural decisions | define dimensions → `/h-explore` → `/h-compare` → `/h-decide` |
| **deep** | Irreversible, security, cross-team | All standard + rich parity, runbook, refresh triggers |

**Default is tactical.** Escalate when: hard to reverse, multiple teams affected, or problem framing is unclear.

---

## Key distinctions (always maintain)

- **Object ≠ Description ≠ Carrier** — the system, its spec, and its code are three things
- **Plan ≠ Reality** — a model is not the thing it models
- **Target system ≠ enabling system** — what must work vs who builds/maintains it
- **Design-time ≠ run-time** — Quint stores your reasoning artifacts (design-time). It does not measure, test, or verify your system (run-time). Don't confuse stored claims with validated evidence.

---

## Proactive agent behavior

When you have Quint tools available, use them **automatically** — don't wait for the user to ask.

### Auto-capture mode (always on)

**Record notes automatically when you observe decisions in conversation.** Don't ask "should I record this?" — just call `haft_note`. The validation will reject bad notes (no rationale) and warn on conflicts. Safe to call freely.

Examples of auto-capture triggers:
- Dev says "I'm going with Redis for this" → `haft_note(title="Redis for session cache", rationale="<extract from conversation>", affected_files=[...])`
- Dev says "let's use gRPC instead of REST" → `haft_note(title="gRPC over REST for payments API", rationale="<from conversation>")`
- Dev makes a config choice, picks a library, chooses an approach → capture it

**Do NOT auto-capture**: formatting choices, import ordering, variable naming (too trivial).

### Post-implementation ritual (after /h-decide → implement)

After implementing a decision:
1. **Baseline first** — if the decision has `affected_files`, call `haft_decision(action="baseline", decision_ref="<id>")`
2. **Verify inductively** — at least one of:
   - Run tests (`go test`, `npm test`, `cargo test`) and confirm they pass
   - Read affected files and verify the implementation matches the decision
   - Ask the user to confirm implementation
3. Call `haft_decision(action="measure", decision_ref="<id>", findings="...", verdict="accepted")` — record verified results

**Calling measure from memory without verification is a FPF B.5:4.3 violation.**
**Calling measure without a baseline (when `affected_files` exist) degrades evidence to CL1 self-evidence.**

This closes the lemniscate. Without measure, the decision stays open.

### Proactive checks (at key moments)

- **At session start**: call `haft_query(action="status")` to surface stale decisions and active problems
- **When /h-status shows pending decisions**: check if they were actually implemented and measured.
- **When code changed after a decision**: do NOT summarize drift without reading diffs. For each modified file: (1) read `git diff`, (2) classify as cosmetic / incidental / material to the decision's invariants, (3) present classification to user.
- **If status returns zero artifacts on a project with code**: suggest `/h-onboard`
- **When dev works on files**: call `haft_query(action="related", file="path")` to find linked decisions — mention them if relevant
- **When dev says "let's just do X" without rationale**: ask "why X?" before recording
- **When auto-captured note conflicts with an active decision**: surface the conflict clearly

### Escalation (user-triggered)

These require explicit user action — don't auto-trigger:
- `/h-frame` — full problem framing (diagnostic conversation)
- `/h-explore` — variant generation
- `/h-compare` — parity comparison
- `/h-decide` — formal decision record

### NavStrip interpretation

The `── Quint ──` strip appended to tool responses shows current state and available actions. Key rules:

- **"Available:" = menu for the user, not instructions for the agent.** Do not auto-execute these actions.
- **Mode determines flow shape** — tactical skips exploration; standard includes explore/compare. The Available line reflects the current mode.
- **Fewer steps ≠ fewer checkpoints.** Tactical mode has fewer FPF steps but the same human consent requirement at each transition.
- **Path 3 override:** Only when the user has explicitly delegated full autonomy ("and implement", "fix everything") may the agent proceed through Available actions without pausing.

---

## RAG search reference

Use `haft fpf search` when you need formal FPF definitions, templates, aggregation rules, conformance checklists, or patterns (A.*/B.*/C.*/F.*).

```bash
haft fpf search "<query>"
haft fpf search "<query>" --full
haft fpf section "<heading>"
```

---

## Concept index (search terms)

**Problem design:** problem card, PROB, anomaly, characterization, CHR, problem portfolio, goldilocks, acceptance spec

**Solution design:** SoTA survey, strategy card, method family, solution portfolio, NQD, stepping stones

**Selection:** Pareto front, selection policy, SEL, parity plan, PAR, fair comparison

**Evidence:** evidence record, EVID, F-G-R, assurance level, corroboration, refutation

**Decisions:** decision record, DRR, rollback plan, rationale, constraints

**Invariants:** WLNK, MONO, IDEM, COMM, LOC, weakest link, cutset

**Reasoning:** ADI cycle, abduction, deduction, induction, lifecycle stages
