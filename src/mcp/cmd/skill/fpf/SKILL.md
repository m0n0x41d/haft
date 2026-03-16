---
name: fpf
description: "Apply First Principles Framework (FPF) reasoning — principled problem design, variant generation, and evidence-based decisions."
argument-hint: "[task, decision, ambiguity, architecture question, or management problem]"
---

# FPF — First Principles Framework

FPF is a systems thinking methodology by Anatoly Levenchuk. This skill gives you its operational core — apply it to reason about problems, solutions, and decisions.

---

## When to invoke

Use this skill when at least one is true:
- the task is ambiguous or badly framed;
- the choice is architectural, organizational, strategic, or hard to reverse;
- multiple stakeholders or viewpoints matter;
- you need to compare serious alternatives;
- acceptance is unclear and must be designed;
- you must separate target system from creator system;
- overloaded words (`process`, `service`, `function`, `quality`, `done`, `validated`) are causing confusion;
- you need an ADR/DRR-like rationale, parity plan, evidence pack, or explicit selection policy.

Do **not** invoke for tiny local edits with explicit acceptance and low blast radius.

---

## Depth calibration

Before starting, assess the scale of the request:

| Mode | When | What to do |
|------|------|------------|
| **Quick** | Tactical choices, clear trade-offs, ≤2 real options | Frame → Variants (table) → Recommendation. 1-2 paragraphs. |
| **Deep** | Architectural decisions, ambiguous problems, irreversible choices, user explicitly asks for depth | Full ADI cycle with evidence records, Pareto analysis, lifecycle stage. |

**Default is Quick.** Escalate to Deep when: the decision is hard to reverse, multiple stakeholders are affected, or the problem framing itself is unclear.

---

## What to do first

Before proposing solutions, state:
1. **Lifecycle stage** — `Explore | Shape | Evidence | Operate`
2. **Target system** — what must work in operation
3. **Creator system** — who builds/changes/operates it
4. **Context** — which bounded context defines the meaning of terms and rules
5. **Problem owner** — whose problem this is

If these are fuzzy, the task is still under-framed.

---

## Core thinking algorithm

### 1. Frame the problem BEFORE solving it

The bottleneck is **problem quality**, not solution speed. Before generating any solution:

- **State what's anomalous** — what observation doesn't fit the current model?
- **Generate ≥3 hypotheses** — genuinely distinct explanations, not variations of one
- **Identify trade-off axes** — what dimensions are in tension? (speed vs safety, generality vs performance, etc.)
- **Define acceptance criteria** — how will you know the problem is solved? Separate:
  - **Optimization targets** (1-3 max) — what you're actively improving
  - **Hard constraints** — must hold, binary pass/fail
  - **Observation indicators** — things you monitor but don't optimize (Anti-Goodhart)

> RAG trigger: for formal problem card format → `quint-code fpf search "problem card PROB"`

### 2. Characterize before comparing

Before evaluating anything, define the **characteristic space** — what dimensions matter and how they're measured. Without this, comparisons are arbitrary.

- State the **selection policy BEFORE seeing results**
- Ensure **parity** — fair comparison requires same inputs, same scope, same budget, same constraints
- Keep comparison **multi-dimensional** — never collapse into one score unless the fold is explicit and justified

> RAG trigger: for characterization templates → `quint-code fpf search "characterization CHR"`

### 3. Generate genuinely distinct variants

- Produce **≥3 variants** that differ in **kind**, not degree
- For each variant, assess quality as **multi-dimensional** (NQD) — never collapse to a single score
- Identify each variant's **weakest link** (WLNK) — the component that bounds overall quality
- Preserve **1-2 stepping stones** — variants that open future possibilities even if not optimal now
- If a more complex option adds moving parts, justify the added weak links (**MONO**)
- At comparable budgets, prefer methods with **better scaling slopes** over hand-tuned solutions (**BLP**)

> RAG trigger: for NQD assessment rules → `quint-code fpf search "NQD variant quality"`

### 4. Select from the Pareto front

- Hold the **Pareto front** — don't discard non-dominated options prematurely
- Select using the pre-declared policy
- Record which variants were non-dominated and why the chosen one wins

> RAG trigger: for formal selection procedure → `quint-code fpf search "selection policy SEL Pareto"`

### 5. Test against reality

- **Predict before testing** — state what you expect to observe if the hypothesis is correct AND if it's wrong
- **Record evidence** — commands run, outputs observed, interpretation
- **Assess confidence** — using F-G-R:
  - **F** (Formality): how rigorous is the method? (ordinal, min across chain)
  - **G** (ClaimScope): what exactly does the claim cover? (set-valued, NOT ordinal)
  - **R** (Reliability): how likely is the claim true? ([0,1], min across chain)
- **Close the loop** — evidence either corroborates or refutes. If refuted, update the problem framing and iterate.
- Record: what was predicted, checked, observed, what changed in confidence, what remains unknown, when evidence goes stale.

No induction without prior deduction. No deduction without prior abduction.

> RAG trigger: for evidence record format → `quint-code fpf search "evidence record EVID F-G-R"`

---

## Reasoning cycle (ADI)

All thinking follows: **Abduction → Deduction → Induction**

| Phase | What happens | Output |
|-------|-------------|--------|
| **Abduction** | Generate hypotheses, frame problems, propose explanations | Problem cards, anomaly records, candidate hypotheses |
| **Deduction** | Derive predictions, define what MUST follow if hypothesis is true | Falsifiable predictions, acceptance specs, logical consequences |
| **Induction** | Test predictions against evidence, update confidence | Evidence records, corroboration/refutation, confidence update |

---

## Lifecycle stages

Every artifact progresses: **Explore → Shape → Evidence → Operate**

| Stage | Activity | ADI phase |
|-------|----------|-----------|
| **Explore** | Generate possibilities, brainstorm, question assumptions | Abduction |
| **Shape** | Select direction, define architecture, ensure internal consistency | Deduction |
| **Evidence** | Test against reality, validate claims, measure performance | Induction |
| **Operate** | Deploy, monitor, maintain | Continuous induction |

Always state which stage you're in. Don't skip stages.

---

## Key distinctions (always maintain)

- **Object ≠ Description ≠ Carrier** — the system, its spec, and its implementation are three different things
- **Plan ≠ Reality** — a model is not the thing it models
- **Role ≠ Capability ≠ Method ≠ WorkPlan ≠ Work**
- **Design-time ≠ Run-time** — planning and modeling vs acting and observing
- **Promise/commitment ≠ delivery/work**
- **Target system ≠ creator system**

**Resolve overloaded words:**
- `process` → resolve into: Role | Capability | Method | WorkPlan | Work
- `service` → resolve whether it means: promise clause, provider, access point, method, or work

**Commensurability (CL 0-3)** — before comparing two things, assess how comparable they are:
- 0 = Opposed (contradictory frames)
- 1 = Comparable (same domain, different frameworks)
- 2 = Translatable (systematic mapping exists)
- 3 = Near-identity (same framework, minor differences)

---

## Core invariants

- **WLNK** — System quality = min(component qualities). The weakest link bounds the whole. Always identify it.
- **MONO** — Improving a part cannot worsen the whole. Adding a part adds a new potential weak link — justify the cost.
- **IDEM** — Evaluating a single element in isolation must return that element unchanged (no accidental upgrade/downgrade).
- **COMM/LOC** — For independent components, evaluation order and location don't matter. When dependencies exist, order matters and must be controlled.

---

## Coding-agent interpretation

### Small code task
If the task is local and clear:
- identify the changed object;
- make the smallest reversible diff;
- run the cheapest relevant checks first;
- state what changed in reality, not just in text.

### Architecture / design task
If the task affects interfaces, data, deployment, observability, reliability, security, cost, or team topology:
1. define the **concept of use** first (actors, scenarios, boundary, value);
2. then define the **system concept** (major parts and why);
3. then capture **architecture decisions** with rationale and trade-offs;
4. only then move to implementation details.

### Engineering-management task
If the task is about planning, delegation, backlog, governance, roadmap, release policy, staffing, or process change:
- distinguish **problematization** from **strategizing**;
- define the problem as a **checkable acceptance spec**;
- manage a **portfolio of problems**, not a random queue;
- make explicit who decides, who executes, and what evidence will count.

---

## Human in the loop

Escalate to the human before acting when the task:
- changes product scope, market commitment, pricing, or external promises;
- is hard to reverse;
- affects security, legal, privacy, compliance, or finance materially;
- changes public interfaces or deletes data;
- changes autonomy budgets, authority, or team responsibilities;
- requires choosing between competing values, not just technical trade-offs.

When escalating, present: the problem, viable variants, selection policy, current recommendation, and main uncertainty.

---

## Anti-patterns to stop immediately

- Solving before framing
- Comparing before characterizing
- Using one KPI as the truth
- Hiding value choices inside technical language
- Treating a spec as proof of execution
- Treating a document or API as the acting system
- Testing without predictions (data dredging)
- Claiming "verified" without recorded evidence
- Relying on context-less labels
- Using FPF as ceremony instead of decision support

---

## Output template

Structure your response using this template. In Quick mode, compress to the essential sections.

```
## Stage
<Lifecycle stage: Explore | Shape | Evidence | Operate>

## Target system / creator system / context

## Problem
<What's anomalous or unclear. One paragraph.>

## Acceptance / constraints / indicators

## Hypotheses
1. <Hypothesis A> — <why it could be true>
2. <Hypothesis B> — <why it could be true>
3. <Hypothesis C> — <why it could be true>

## Variants

| Variant | <Axis 1> | <Axis 2> | <Axis 3> | Weak link |
|---------|----------|----------|----------|-----------|
| A. ...  | ...      | ...      | ...      | ...       |
| B. ...  | ...      | ...      | ...      | ...       |
| C. ...  | ...      | ...      | ...      | ...       |

## Selection
**Policy:** <what matters most and why>
**Recommendation:** <which variant and why>
**Stepping stone:** <what option to preserve for the future, if any>
**Revisit when:** <trigger condition to reconsider this decision>

## Evidence / uncertainty
<What was checked, what remains unknown, when evidence goes stale>

## Next action
<Concrete step to take now>
```

In **Quick mode**, the minimum viable output is: Problem (1-2 sentences) → Variants (table) → Recommendation + Next action.

In **Deep mode**, include all sections plus: ADI phase labels, F-G-R confidence assessments, evidence records.

---

## RAG search reference

The above is enough for applying FPF reasoning. Search `quint-code fpf` when you need formal templates, deep definitions, conformance checklists, aggregation rules, or specific patterns (A.*/B.*).

```bash
# Quick search
quint-code fpf search "<query>"

# Full section content
quint-code fpf search "<query>" --full

# Specific section by heading
quint-code fpf section "<heading>"
```

---

## Concept index (search terms)

**Problem design:** problem card, PROB, anomaly, ANOM, characterization, CHR, problem portfolio, PPORT, goldilocks, trade-off axes, acceptance spec

**Solution design:** SoTA survey, SOTA, strategy card, STRAT, method family, invalidation conditions, solution portfolio, SPORT, variant generation, NQD, stepping stones

**Selection:** Pareto front, selection policy, SEL, parity plan, PAR, fair comparison, Pareto analysis

**Evidence:** evidence record, EVID, predictions, corroboration, refutation, F-G-R, assurance level, L0, L1, L2

**Decisions:** decision record, DRR, irreversible, rollback plan, options, rationale

**Aggregation:** Gamma, fold, Quintet, IDEM, COMM, LOC, WLNK, MONO, weakest link, cutset

**Reasoning:** ADI cycle, abduction, deduction, induction, explore, shape, evidence, operate, lifecycle

**Comparison:** commensurability, CL 0-3, bridge matrix, translation, near-identity, opposed

**Systems:** target system, creator system, concept of use, system concept, architecture decisions, service polysemy, PromiseContent
