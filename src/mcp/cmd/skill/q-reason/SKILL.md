---
name: q-reason
description: "Structured reasoning for engineering decisions — frame problems, compare options, decide with rationale, detect when decisions go stale."
argument-hint: "[problem, decision, architecture question, or 'what's stale?']"
---

# Quint Reasoning — Think Before You Build

This skill activates structured engineering reasoning powered by FPF (First Principles Framework) and Quint Code.

**When to use**: any decision that's not trivially reversible. Architecture choices, library selection, API design, data model changes, infrastructure decisions, process changes.

**When NOT to use**: obvious bug fixes, formatting, tiny refactors with clear acceptance.

---

## What you have

### Quint tools (MCP) — persist reasoning as artifacts

| Tool | What it does | Slash command |
|------|-------------|---------------|
| `quint_note` | Record micro-decisions with rationale | `/q-note` |
| `quint_problem` | Frame problems, define comparison space | `/q-frame`, `/q-char` |
| `quint_solution` | Explore variants, compare fairly | `/q-explore`, `/q-compare` |
| `quint_decision` | Decide with full rationale, generate implementation brief | `/q-decide`, `/q-apply` |
| `quint_refresh` | Detect stale decisions, manage validity | `/q-refresh` |
| `quint_query` | Search past decisions, check status | `/q-search`, `/q-status` |

### FPF spec search — deep methodology reference

```bash
quint-code fpf search "<query>"        # keyword search
quint-code fpf search "<query>" --full # full section content
quint-code fpf section "<heading>"     # exact section
```

Use this when you need formal FPF definitions, templates, aggregation rules, or conformance checklists.

---

## How to reason

### 1. Frame the problem BEFORE solving it

The bottleneck is **problem quality**, not solution speed.

- **State what's anomalous** — what doesn't fit the current model?
- **Identify trade-off axes** — what dimensions are in tension?
- **Define acceptance** — how will we know it's solved?
  - **Optimization targets** (1-3 max)
  - **Hard constraints** (binary pass/fail)
  - **Observation indicators** (monitor but don't optimize — Anti-Goodhart)

**Persist with**: `quint_problem(action="frame", ...)`

> RAG: `quint-code fpf search "problem card PROB"`

### 2. Characterize before comparing

Define the **characteristic space** before evaluating options. Without explicit dimensions, comparisons are arbitrary.

- State the **selection policy BEFORE seeing results**
- Ensure **parity** — same inputs, same scope, same budget across all options
- Keep it **multi-dimensional** — never collapse to a single score unless the fold is explicit

**Persist with**: `quint_problem(action="characterize", ...)`

> RAG: `quint-code fpf search "characterization CHR"`

### 3. Generate genuinely distinct variants

- **≥3 variants** that differ in **kind**, not degree
- Each variant gets a **weakest link** (WLNK) — the component that bounds overall quality
- Preserve **stepping stones** — options that open future possibilities
- **MONO**: if complexity goes up, the added parts must justify the new weak links

**Persist with**: `quint_solution(action="explore", ...)`

> RAG: `quint-code fpf search "NQD variant quality"`

### 4. Compare on the Pareto front

- Hold the **non-dominated set** — don't discard options that aren't strictly worse on ALL dimensions
- Apply the pre-declared selection policy
- Record what was compared, what won, and why

**Persist with**: `quint_solution(action="compare", ...)`

> RAG: `quint-code fpf search "selection policy SEL Pareto"`

### 5. Decide with full rationale

The decision record must contain:
- **Invariants** — what MUST hold at all times
- **Pre/post-conditions** — checklists for implementation
- **Admissibility** — what is NOT acceptable
- **Rollback plan** — when and how to reverse
- **Refresh triggers** — when to re-evaluate
- **Weakest link** — what bounds reliability

**Persist with**: `quint_decision(action="decide", ...)`

### 6. Detect staleness and refresh

Decisions expire. Evidence goes stale. Reality changes.

- Check `valid_until` and refresh triggers
- Surface what's affected
- Waive, reopen, or supersede as appropriate

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

## Reasoning cycle (ADI)

All reasoning follows: **Abduction → Deduction → Induction**

| Phase | What happens | Quint action |
|-------|-------------|--------------|
| **Abduction** | Frame problems, generate hypotheses | `/q-frame`, `/q-explore` |
| **Deduction** | Define what MUST follow, acceptance criteria | `/q-char`, `/q-compare` |
| **Induction** | Test against evidence, update confidence | `/q-decide` (evidence), `/q-refresh` |

**Anti-patterns**: solving before framing (skipping abduction), testing without predictions (skipping deduction), claiming "verified" without evidence (skipping induction).

---

## Lifecycle stages

| Stage | Activity | Typical commands |
|-------|----------|-----------------|
| **Explore** | Generate possibilities, brainstorm | `/q-frame`, `/q-explore` |
| **Shape** | Select direction, ensure consistency | `/q-char`, `/q-compare`, `/q-decide` |
| **Evidence** | Validate claims, measure performance | `/q-decide` (evidence), `/q-apply` |
| **Operate** | Deploy, monitor, refresh when stale | `/q-apply`, `/q-refresh` |

---

## Key distinctions (always maintain)

- **Object ≠ Description ≠ Carrier** — the system, its spec, and its code are three things
- **Plan ≠ Reality** — a model is not the thing it models
- **Target system ≠ creator system** — what must work vs who builds it
- **WLNK** — System quality = min(component qualities). Always identify the weakest link.
- **Commensurability (CL 0-3)** — before comparing, assess how comparable the options are

---

## Proactive agent behavior

When you have Quint tools available, use them proactively:

- **At session start**: call `quint_query(action="status")` to surface stale decisions
- **When dev works on files**: call `quint_query(action="related", file="path")` to find linked decisions
- **When dev makes inline decisions**: suggest `quint_note` to capture rationale
- **When dev says "let's just do X"**: ask "why X?" — rationale is always required
- **When note conflicts with active decision**: warn before recording

---

## When to search FPF spec

Use `quint-code fpf search` when you need:
- Formal definitions of FPF concepts
- Templates for problem cards, evidence records, decision records
- Aggregation rules (Gamma, fold, Quintet)
- Conformance checklists
- Patterns referenced as A.*/B.*/C.*/F.*

---

## Concept index (search terms)

**Problem design:** problem card, PROB, anomaly, characterization, CHR, problem portfolio, goldilocks, acceptance spec

**Solution design:** SoTA survey, strategy card, method family, solution portfolio, NQD, stepping stones

**Selection:** Pareto front, selection policy, SEL, parity plan, PAR, fair comparison

**Evidence:** evidence record, EVID, F-G-R, assurance level, corroboration, refutation

**Decisions:** decision record, DRR, rollback plan, rationale, constraints

**Invariants:** WLNK, MONO, IDEM, COMM, LOC, weakest link, cutset

**Reasoning:** ADI cycle, abduction, deduction, induction, lifecycle stages
