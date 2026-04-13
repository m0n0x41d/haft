---
name: h-reason
description: "Think before building. Use when the user asks to reason about, analyze, evaluate, compare options, make an architecture decision, choose between approaches, think through a problem, or assess trade-offs. Also use when the user asks 'why did we...', 'should we...', 'what are our options', 'is this the right approach', or wants to frame/reframe a problem."
argument-hint: "[problem, decision, architecture question, trade-off, or 'what's stale?']"
---

# Haft Reasoning — Think Before You Build

Haft helps with 5 engineering jobs: **Understand, Explore, Choose, Execute, Verify.** Plus **Note** for quick micro-decisions. This skill activates structured engineering reasoning powered by these modes.

**When to use**: any non-trivial engineering question. Architecture choices, library selection, API design, data model changes, infrastructure decisions, process changes. Also: "think about", "reason about", "evaluate", or "compare" anything significant.

**When NOT to use**: obvious bug fixes, formatting, tiny refactors with clear acceptance.

---

## Context-aware entry — how much the agent drives

**Before doing anything, assess the user's intent.** The 5 engineering modes (Understand/Explore/Choose/Execute/Verify) describe WHAT you're doing. The 4 interaction modes below describe HOW MUCH the agent drives. They're orthogonal.

### Direct response / direct action
**Trigger:** "think about X", "what do you think about X", "analyze X", "is this the right approach?", "what are our options?", "save this as md", "make this a ranked list", "turn this into a checklist", "move this to .context", "summarize what you found"

Reason through the problem or do the direct artifact work with normal tools. **Do not call Haft MCP tools** unless the user explicitly asks to persist something. "Use FPF in your thinking" means reasoning discipline, not artifact workflow.

### Research / prepare-and-wait
**Trigger:** "/h-reason [topic], prepare for framing", "let's think about X before deciding", "I want to reason through X"

Gather context (read code, search existing decisions, research). Present findings. **Stop and wait** for the user to decide the next step.

### Delegated reasoning
**Trigger:** "/h-reason [topic], go ahead", "work through the options and bring me a recommendation", natural-language delegation like "do it" / "go ahead"

Drive frame → explore → compare in one pass. **Do not stop after frame. Do not require a manual `/h-explore` or `/h-compare` step.** Stop after compare, show the Pareto front, and ask the human to choose. The Transformer Mandate applies at the Choose → Execute boundary: the agent may frame/explore/compare when delegated, but the human still chooses before `/h-decide`.

### Autonomous execution
**Trigger:** "/h-reason [topic] and implement" ONLY when autonomous mode is already enabled for the session (Ctrl+Q / interaction=autonomous).

Full cycle including decide + implement without pauses. If autonomous mode is OFF, phrases like "figure out the best approach and do it" or "fix everything" are NOT enough to skip the compare → decide pause. Treat them as delegated reasoning.

**If unclear:** default to research / prepare-and-wait. Never default to autonomous execution.

---

## What you have

### Haft tools (MCP) — persist reasoning as artifacts

| Tool | What it does | Slash command |
|------|-------------|---------------|
| `haft_note` | Record micro-decisions with rationale validation | `/h-note` |
| `haft_problem` | Frame problems and persist characterization dimensions on the ProblemCard | `/h-frame`, `/h-char` |
| `haft_solution` | Explore variants, compare and identify Pareto front | `/h-explore`, `/h-compare` |
| `haft_decision` | Decide with formal rationale; record measurement results | `/h-decide` |
| `haft_refresh` | Detect stale decisions, manage lifecycle | `/h-verify` |
| `haft_query` | Search, status dashboard, file-to-decision lookup, FPF spec lookup, deterministic audience projections | `/h-search`, `/h-status`, `/h-view` |

### FPF spec lookup — prefer MCP when available

```text
haft_query(action="fpf", query="A.6")
haft_query(action="fpf", query="How do I route boundary statements?", limit=3, explain=true)
haft_query(action="fpf", query="Boundary Norm Square", full=true)
```

In shell-only environments: `haft fpf search "<query>"` with optional `--full`.

---

## Feature maturity

| Concept | Status | What Haft does |
|---------|--------|-----------------|
| Problem framing | **tracked** | Stores signal, constraints, targets, acceptance. You do the framing, Haft persists it. |
| Characterization | **tracked** | Stores comparison dimensions with scale/unit/polarity/role on the ProblemCard. |
| WLNK | **tracked** | Required label on variants. Haft stores the stated weakest link. |
| Parity | **textual** | Stored as rules text. Not enforced or verified. You ensure parity yourself. |
| Pareto front | **computed + tracked** | Haft derives the non-dominated set from submitted comparison data. |
| Stepping stones | **tracked** | Boolean flag on variants, shown in summary table. |
| Refresh (valid_until) | **enforced** | All artifacts with expired valid_until detected by scan. |
| CL (congruence) | **artifact-level** | Evidence calculations exist. Tool stores explicit evidence with CL values. |
| Impact measurement | **tracked** | `haft_decision(action="measure")` records post-implementation findings. |
| Evidence attachment | **supported** | Attach evidence with type/verdict/CL. Set `valid_until` for decay. |

**Key rule: don't describe textual features as if they compute something.** "WLNK bounds quality" means the user identified what bounds quality, not that the system calculated it.

These skill instructions are **L1 — detection and questions.** The tools (L2) persist and enforce. Don't treat a prompt-based check as verified evidence.

---

## The 5 engineering modes

### Understand — "What's actually going on?"

The bottleneck is **problem quality**, not solution speed. Frame before solving.

**Framing protocol — ask these questions before recording:**

1. **"What observation doesn't fit?"** — the signal, not the assumed cause. "Webhook retries hit 15%" not "we need a new queue."
2. **"What have you already tried?"** — avoids re-treading dead ends.
3. **"Who owns this problem?"** — a specific person with authority, not "the team."
4. **"What would solved look like?"** — measurable acceptance, not "it should be better."
5. **"What constraints are non-negotiable?"** — hard limits no variant can violate.
6. **"How reversible is this? What's the blast radius?"** — determines depth (tactical vs standard vs deep).
7. **"What should we watch but NOT optimize?"** — Anti-Goodhart indicators.

**Problem typing:** Before exploring, classify:
- **Optimization** — working system, want it better on a known dimension
- **Diagnosis** — something's broken, don't know why
- **Search** — need to find something that doesn't exist yet
- **Synthesis** — need to combine existing elements into something new

Each type suggests different exploration strategies and ceremony levels.

**Language precision triggers:**
- If the signal uses ambiguous terms (service, process, function, quality, component), **unpack to precise meaning before recording.** "Which service — the OAuth provider, the token endpoint, or the session store?"
- If the user says "it should do X," clarify: **hard constraint, preference, or observation?**
- If two stakeholders use the same word differently, resolve before framing.

**Characterization:** Define the **characteristic space** before evaluating options.
- State the **selection policy BEFORE seeing results**
- Ensure **parity** — same inputs, same scope, same budget across all options
- Keep it **multi-dimensional** — never collapse to a single score unless the fold is explicit

**Persist with:** `haft_problem(action="frame")`, `haft_problem(action="characterize")`
**Commands:** `/h-frame`, `/h-char`
**Goldilocks check:** When multiple problems are active, use `haft_problem(action="select")` to pick the one in the growth zone.

### Explore — "What are the real options?"

- **>=2 variants** that differ in **kind**, not degree (3+ preferred)
- Each variant gets a **weakest link** label — what bounds its quality
- Mark **stepping stones** — options that open future possibilities even if not optimal now

**Diversity self-check:** Before submitting variants, verify they aren't disguised copies of the same approach. If all variants are cache-based, at least one should explore a genuinely different direction (e.g., restructure data flow to eliminate caching need).

**Hypothesis discipline:** When investigating during exploration, separate what you observe from what you hypothesize. State hypotheses explicitly. "I observe high latency on the /auth endpoint. Hypothesis: the bottleneck is the token validation call, not the database query."

**Past solutions:** Use `haft_solution(action="similar")` to search for relevant precedents from past or cross-project work.

**Persist with:** `haft_solution(action="explore")`
**Command:** `/h-explore`

### Choose — "Which is better — and should I even decide now?"

**Probe-or-commit gate:** Before jumping to comparison, assess readiness:

1. Are all key dimensions covered with data for all variants?
2. Do the variants span genuinely different approaches?
3. Is there a specific investigation that could change the ranking?

If 1+2 yes and 3 no → **commit** to comparison.
If 3 yes → **probe** — name the specific next investigation, what comparison defect it repairs, and estimated cost.
If 2 no → **widen** — explore more variants first.
If the burden shifted (this isn't really a choice problem) → **reroute** to Understand.

**Fair comparison:**
- Identify the **non-dominated set** (Pareto front) — variants not strictly worse on all dimensions
- Apply the pre-declared selection policy
- **Constraint elimination:** Constraints are hard limits. A variant that violates a constraint dimension is eliminated before Pareto computation, not merely scored lower.
- Record what was compared, what won, and why

**Language precision in comparison:** If comparison dimensions use subjective terms (maintainable, simple, scalable), ask for measurable specifics before scoring. "Maintainability" could mean: fewer dependencies? Lower cyclomatic complexity? Team already knows the stack? These point to different winners.

**Reroute upstream:** If comparison reveals that the framing was ambiguous, that the problem type was wrong, or that variants address different problems → reroute to Understand. Don't force a choice on a broken comparison basis.

**Transformer Mandate:** The human chooses at the Choose → Execute boundary. Agent may frame/explore/compare when delegated, but the human confirms the selection before recording a decision. Exception: autonomous mode.

**Persist with:** `haft_solution(action="compare")`
**Command:** `/h-compare`

### Execute — "Let's do it — carefully."

The decision record should contain:
- **Invariants** — what MUST hold at all times
- **Pre-conditions** — what must be true before implementation begins
- **Post-conditions** — checklist for implementation completion
- **Admissibility** — what is NOT acceptable
- **Evidence requirements** — what proof the verification loop must gather
- **Predictions** — falsifiable claims with `claim`, `observable`, and `threshold`
- **Refresh triggers** — concrete signals that should reopen the decision
- **Valid-until date** — when to re-evaluate automatically
- **Weakest link** — what most plausibly breaks this choice

**Async evidence:** When a claim can't be verified immediately (e.g., "error rate drops 30% after 1 week of production"), add a `verify_after` date to the prediction. This surfaces automatically in Verify mode when the date passes.

**Baseline:** After implementation, call `haft_decision(action="baseline")` to snapshot affected file hashes for drift detection.

**Projections for handoff:** Use projections when reasoning already exists and you need a boundary-crossing view:
- `/h-view brief` for delegated-agent implementation handoff
- `/h-view rationale` for PR/change rationale
- `/h-view audit` for evidence/assurance review
- `/h-view compare` for the current Pareto/trade-off surface

Projections render the same artifact graph for a different audience — no new semantics.

**Persist with:** `haft_decision(action="decide")`, `haft_decision(action="baseline")`
**Commands:** `/h-decide`, `/h-view`

### Verify — "Did it work? Is it still valid?"

**Post-implementation verification:**
1. **Baseline first** — if the decision has `affected_files`, call `haft_decision(action="baseline")`
2. **Verify inductively** — run tests, read affected files, or ask the user to confirm
3. **Attach evidence** — `haft_decision(action="evidence")` with type/verdict/CL
4. **Record measurement** — `haft_decision(action="measure")` with findings and verdict

**Calling measure from memory without verification is a violation.**
**Calling measure without baseline degrades evidence to CL1 self-evidence.**
**Evidence without measure doesn't close the loop.**

**Ongoing verification:**
- **Claim status:** Which predictions were verified, which aren't?
- **Drift detection:** Files changed since baseline → classify as cosmetic / incidental / material
- **Staleness scan:** Expired valid_until, decayed evidence
- **Pending verifications:** Claims with verify_after dates that have passed — surface proactively

**Entity preservation:** When summarizing verification results, preserve entity count and identity. Don't merge "5 claims" into "several verified items."

**Lifecycle management:**
- `haft_refresh(action="waive")` — extend validity with evidence
- `haft_refresh(action="reopen")` — start new problem cycle from a decision
- `haft_refresh(action="supersede")` — replace one artifact with another
- `haft_refresh(action="deprecate")` — archive as no longer relevant

When reopening a stale decision, the new ProblemCard inherits lineage.

**Persist with:** `haft_decision(action="measure/evidence")`, `haft_refresh(...)`
**Commands:** `/h-verify`, `/h-status`

### The loop and legitimate reroutes

```
Understand → Explore → Choose → Execute → Verify
    ↑            ↑        ↑        ↑        │
    └────────────┴────────┴────────┴────────┘
```

- **Choose → Understand** — comparison reveals bad framing or ambiguity
- **Explore → Understand** — exploration reveals wrong problem type
- **Execute → Choose** — implementation reveals chosen option doesn't work
- **Verify → any** — verification shows problem was misframed, option failed, or comparison basis invalidated

Reroutes are not failures. They prevent compounding errors downstream. The simple forward arrow is the common case; reroutes are the important case.

### Fast path: Note

Not everything needs the 5-mode ceremony. Quick micro-decisions go to `haft_note`: captures what was decided and why, stays in the artifact graph for future recall and conflict detection.

---

## Depth calibration

| Depth | When | What changes |
|-------|------|-------------|
| **note** | Micro-decisions during coding | `/h-note` — done |
| **tactical** | Reversible, <2 weeks blast radius | Compact Understand + Explore + Choose, standard Execute |
| **standard** | Most architectural decisions | Full Understand with characterization, 3+ variants in Explore, full Choose with Pareto |
| **deep** | Irreversible, security, cross-team | All standard + rich parity rules, runbook, refresh triggers, evidence requirements |

**Default is tactical.** Escalate when: hard to reverse, multiple teams affected, or problem framing is unclear. Depth changes how much evidence and structure you show, not whether the modes exist.

---

## Key distinctions (always maintain)

- **Object ≠ Description ≠ Carrier** — the system, its spec, and its code are three things
- **Plan ≠ Reality** — a model is not the thing it models
- **Target system ≠ Enabling system** — what must work vs who builds/maintains it
- **Design-time ≠ Run-time** — stored reasoning artifacts ≠ verified system behavior

---

## Proactive agent behavior

### Auto-capture mode (always on)

Record notes automatically when you observe decisions in conversation. Don't ask — just call `haft_note`. Validation rejects bad notes.

Triggers: "I'm going with X", "let's use Y instead of Z", config choice, library pick, approach selection.
**Do NOT auto-capture:** formatting choices, import ordering, variable naming.

### Proactive checks

- **At session start**: call `haft_query(action="status")` to surface stale decisions and active problems
- **When code changed after a decision**: read `git diff`, classify as cosmetic / incidental / material
- **If status returns zero artifacts on a project with code**: suggest `/h-onboard`
- **When dev works on files**: call `haft_query(action="related", file="path")` to find linked decisions
- **When dev says "let's just do X" without rationale**: ask "why X?" before recording
- **When auto-captured note conflicts with active decision**: surface the conflict

### User steering

Slash commands are course corrections, not mandatory triggers:
- In **research / prepare-and-wait** mode, the user triggers `/h-frame`, `/h-explore`, etc. when ready.
- In **delegated reasoning** mode, natural-language delegation continues through modes without extra commands.
- Direct operational requests stay direct. Don't escalate them into `/h-frame` just because they touch engineering content.

### NavStrip interpretation

The `── Haft ──` strip appended to tool responses shows current state and available actions.
- **"Available:" = menu for the user**, not instructions for the agent.
- **Mode determines ceremony depth**, not whether Choose may bypass Explore.
- In research + wait mode, do not auto-advance. In delegated reasoning, you may advance through modes without extra commands.

---

## RAG search reference

Use MCP-native FPF lookup when tools are available:

```text
haft_query(action="fpf", query="problem card PROB")
haft_query(action="fpf", query="A.6", full=true)
haft_query(action="fpf", query="How do I route boundary statements?", limit=3, explain=true)
```

In shell-only environments: `haft fpf search "<query>"` or `haft fpf section "<heading-or-id>"`.

---

## Concept index (search terms)

**Understand:** problem card, PROB, anomaly, characterization, CHR, problem portfolio, goldilocks, acceptance spec, problem typing

**Explore:** SoTA survey, strategy card, method family, solution portfolio, NQD, stepping stones, diversity

**Choose:** Pareto front, selection policy, SEL, parity plan, PAR, fair comparison, probe-or-commit, probe-worthiness

**Execute:** decision record, DRR, rollback plan, rationale, constraints, verify_after, baseline

**Verify:** evidence record, EVID, F-G-R, assurance level, corroboration, refutation, drift, staleness, refresh

**Cross-cutting:** WLNK, MONO, IDEM, COMM, LOC, weakest link, cutset, ADI cycle, abduction, deduction, induction
