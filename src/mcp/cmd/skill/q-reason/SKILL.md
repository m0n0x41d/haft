---
name: q-reason
description: "Think before building. Use when the user asks to reason about, analyze, evaluate, compare options, make an architecture decision, choose between approaches, think through a problem, or assess trade-offs. Also use when the user asks 'why did we...', 'should we...', 'what are our options', 'is this the right approach', or wants to frame/reframe a problem."
argument-hint: "[problem, decision, architecture question, trade-off, or 'what's stale?']"
---

# Quint Reasoning — Think Before You Build

This skill activates structured engineering reasoning powered by FPF (First Principles Framework) and Quint Code.

**When to use**: any non-trivial engineering question. Architecture choices, library selection, API design, data model changes, infrastructure decisions, process changes. Also: when the user asks to "think about", "reason about", "evaluate", or "compare" anything significant.

**When NOT to use**: obvious bug fixes, formatting, tiny refactors with clear acceptance.

---

## What you have

### Quint tools (MCP) — persist reasoning as artifacts

| Tool | What it does | Slash command |
|------|-------------|---------------|
| `quint_note` | Record micro-decisions with rationale validation | `/q-note` |
| `quint_problem` | Frame problems, define comparison dimensions | `/q-frame`, `/q-char` |
| `quint_solution` | Explore variants, compare and identify Pareto front | `/q-explore`, `/q-compare` |
| `quint_decision` | Decide with full rationale, generate implementation brief | `/q-decide`, `/q-apply` |
| `quint_refresh` | Detect stale decisions, manage lifecycle | `/q-refresh` |
| `quint_query` | Search, status dashboard, file-to-decision lookup | `/q-search`, `/q-status` |

### FPF spec search — deep methodology reference

```bash
quint-code fpf search "<query>"        # keyword search
quint-code fpf search "<query>" --full # full section content
quint-code fpf section "<heading>"     # exact section
```

Use for formal FPF definitions, templates, aggregation rules, or conformance checklists.

---

## Feature maturity

Not all FPF concepts are at the same implementation depth. This matters — don't present tracked annotations as computed evaluations.

| Concept | Status | What Quint does |
|---------|--------|-----------------|
| Problem framing | **tracked** | Stores signal, constraints, targets, acceptance. You do the framing, Quint persists it. |
| Characterization | **tracked** | Stores comparison dimensions with scale/unit/polarity. You define them, Quint persists them. |
| WLNK | **tracked** | Required label on variants. After evidence is attached, WLNK summary shows min CL, freshness, supporting/weakening/refuting counts. |
| Parity | **textual** | Stored as rules text. Not enforced or verified. You ensure parity yourself. |
| Pareto front | **tracked** | You identify the non-dominated set, Quint stores and displays it. Not computed automatically. |
| Stepping stones | **tracked** | Boolean flag on variants, shown in summary table. |
| Refresh (valid_until) | **enforced** | All artifacts (decisions, problems, portfolios) with expired valid_until detected by scan. |
| Refresh triggers | **textual** | Stored in decision body. Only valid_until date is actually scanned. Text triggers are reminders, not automated checks. |
| CL (congruence) | **tracked** | Field on evidence items. Shown in WLNK summary (min CL across evidence chain). |
| F-G-R | **textual** | Formality field on evidence items. Stored, shown in WLNK summary when present. |
| NQD | **absent** | Multi-dimensional quality vectors are not implemented. Use comparison dimensions on ProblemCard instead. |
| Impact measurement | **tracked** | `quint_decision(action="measure")` records post-implementation findings against acceptance criteria. |
| Evidence attachment | **tracked** | `quint_decision(action="evidence")` attaches evidence items to any artifact with type/verdict/CL/carrier. |

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

**Persist with**: `quint_problem(action="frame", ...)`

**Goldilocks check**: When multiple problems are active, use `quint_problem(action="select")` to see them with blast radius and reversibility signals. Pick the problem in the growth zone — not too trivial (low impact), not too impossible (exceeds current capacity).

> RAG: `quint-code fpf search "problem card PROB"`

### 2. Characterize before comparing

Define the **characteristic space** before evaluating options. Without explicit dimensions, comparisons are arbitrary.

- State the **selection policy BEFORE seeing results**
- Ensure **parity** — same inputs, same scope, same budget across all options (you enforce this, Quint stores your parity rules)
- Keep it **multi-dimensional** — never collapse to a single score unless the fold is explicit

**Persist with**: `quint_problem(action="characterize", ...)`

> RAG: `quint-code fpf search "characterization CHR"`

### 3. Generate genuinely distinct variants

- **≥2 variants** that differ in **kind**, not degree (3+ preferred)
- Each variant gets a **weakest link** label — what you judge bounds its quality
- Mark **stepping stones** — options that open future possibilities even if not optimal now

**Persist with**: `quint_solution(action="explore", ...)`

> RAG: `quint-code fpf search "NQD variant quality"`

### 4. Compare and select

- Identify the **non-dominated set** — variants not strictly worse on all dimensions
- Apply the pre-declared selection policy
- Record what was compared, what won, and why

Note: Quint stores your comparison results and Pareto front. It does not compute them for you — you (or the agent doing analysis) determine which variants are non-dominated.

**Persist with**: `quint_solution(action="compare", ...)`

> RAG: `quint-code fpf search "selection policy SEL Pareto"`

### 5. Decide with full rationale

The decision record should contain:
- **Invariants** — what MUST hold at all times
- **Pre/post-conditions** — checklists for implementation
- **Admissibility** — what is NOT acceptable
- **Rollback plan** — when and how to reverse
- **Refresh triggers** — when to re-evaluate (set valid_until date for automatic detection)
- **Weakest link** — your assessment of what bounds reliability

**Persist with**: `quint_decision(action="decide", ...)`

### 6. Detect staleness and refresh

All artifacts (decisions, problems, portfolios) with expired `valid_until` dates are automatically detected by `/q-status` and `/q-refresh`. This includes stale ProblemCards — problem framings can go stale too when context changes.

Text-based refresh triggers (e.g., "re-evaluate if throughput >80k/s") are stored as reminders but not automatically checked — you or the agent must notice when conditions change.

When reopening a stale decision, the new ProblemCard inherits lineage: prior characterization, failure reason, and evidence references from the previous cycle.

**Persist with**: `quint_refresh(...)`

---

## Depth calibration

| Mode | When | Flow |
|------|------|------|
| **note** | Micro-decisions during coding | `/q-note` — done |
| **tactical** | Reversible, <2 weeks blast radius | `/q-frame` → `/q-explore` → `/q-decide` |
| **standard** | Most architectural decisions | `/q-frame` → `/q-char` → `/q-explore` → `/q-compare` → `/q-decide` |
| **deep** | Irreversible, security, cross-team | All standard + rich parity, runbook, refresh triggers |

**Default is tactical.** Escalate when: hard to reverse, multiple teams affected, or problem framing is unclear.

---

## Key distinctions (always maintain)

- **Object ≠ Description ≠ Carrier** — the system, its spec, and its code are three things
- **Plan ≠ Reality** — a model is not the thing it models
- **Target system ≠ creator system** — what must work vs who builds it
- **Design-time ≠ run-time** — Quint stores your reasoning artifacts (design-time). It does not measure, test, or verify your system (run-time). Don't confuse stored claims with validated evidence.

---

## Proactive agent behavior

When you have Quint tools available, use them **automatically** — don't wait for the user to ask.

### Auto-capture mode (always on)

**Record notes automatically when you observe decisions in conversation.** Don't ask "should I record this?" — just call `quint_note`. The validation will reject bad notes (no rationale) and warn on conflicts. Safe to call freely.

Examples of auto-capture triggers:
- Dev says "I'm going with Redis for this" → `quint_note(title="Redis for session cache", rationale="<extract from conversation>", affected_files=[...])`
- Dev says "let's use gRPC instead of REST" → `quint_note(title="gRPC over REST for payments API", rationale="<from conversation>")`
- Dev makes a config choice, picks a library, chooses an approach → capture it

**Do NOT auto-capture**: formatting choices, import ordering, variable naming (too trivial).

### Proactive checks (at key moments)

- **At session start**: call `quint_query(action="status")` to surface stale decisions and active problems
- **If status returns zero artifacts on a project with code**: suggest `/q-onboard`
- **When dev works on files**: call `quint_query(action="related", file="path")` to find linked decisions — mention them if relevant
- **When dev says "let's just do X" without rationale**: ask "why X?" before recording
- **When auto-captured note conflicts with an active decision**: surface the conflict clearly

### Escalation (user-triggered)

These require explicit user action — don't auto-trigger:
- `/q-frame` — full problem framing (diagnostic conversation)
- `/q-explore` — variant generation
- `/q-compare` — parity comparison
- `/q-decide` — formal decision record

---

## RAG search reference

Use `quint-code fpf search` when you need formal FPF definitions, templates, aggregation rules, conformance checklists, or patterns (A.*/B.*/C.*/F.*).

```bash
quint-code fpf search "<query>"
quint-code fpf search "<query>" --full
quint-code fpf section "<heading>"
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
