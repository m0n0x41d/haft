---
name: h-reason
description: "Think before building. Use when the user asks to reason about, analyze, evaluate, compare options, make an architecture decision, choose between approaches, think through a problem, or assess trade-offs. Also use when the user asks 'why did we...', 'should we...', 'what are our options', 'is this the right approach', or wants to frame/reframe a problem."
argument-hint: "[problem, decision, architecture question, trade-off, or 'what's stale?']"
---

# Haft Reasoning — Think Before You Build

This skill activates structured engineering reasoning powered by FPF (First Principles Framework) and Haft.

**When to use**: any non-trivial engineering question. Architecture choices, library selection, API design, data model changes, infrastructure decisions, process changes. Also: when the user asks to "think about", "reason about", "evaluate", or "compare" anything significant.

**When NOT to use**: obvious bug fixes, formatting, tiny refactors with clear acceptance.

---

## Context-aware entry — read what the user actually wants

**Before doing anything, assess the user's intent from context and arguments.** Do NOT always fall through into the full FPF cycle. FPF reasoning and FPF artifact persistence are different things. Use this canonical interaction matrix:

### Direct response / direct action
**Trigger:** "think about X", "what do you think about X", "analyze X", "is this the right approach?", "what are our options?", "save this as md", "make this a ranked list", "turn this into a checklist", "move this to .context", "summarize what you found"

The user wants structured thinking, packaging, persistence, formatting, or summarization. Reason through the problem or do the direct artifact work with normal tools. **Do not call Haft MCP tools** unless the user explicitly asks to persist something.

This also applies when the user says things like "use FPF in your thinking". That means use FPF as a reasoning discipline, not automatically as an artifact workflow.

### Research / prepare-and-wait
**Trigger:** "/h-reason [topic], prepare for framing", "let's think about X before deciding", "I want to reason through X"

The user wants to drive the cycle themselves. Gather context (read relevant code, search existing decisions, research). Present findings. **Stop and wait** for the user to decide the next step — they will call `/h-frame`, `/h-char`, etc. when ready.

### Delegated reasoning
**Trigger:** "/h-reason [topic], go ahead", "work through the options and bring me a recommendation", "frame it and compare approaches", natural-language delegation like "давай" / "do it" / "go ahead" when autonomous mode is NOT enabled

The user wants the agent to drive the reasoning work, but not to make the final choice. Run frame → characterize if needed → explore → compare in one pass. **Do not stop after frame. Do not require a manual `/h-explore` or `/h-compare` step after frame.** Stop after compare, show the Pareto front, and ask the human to choose. The Transformer Mandate applies at the compare → decide boundary: the agent may frame/explore/compare when delegated, but the human still chooses the winning variant before `/h-decide`.

### Autonomous execution
**Trigger:** "/h-reason [topic] and implement" or equivalent implementation delegation ONLY when autonomous mode is already enabled for the session (Ctrl+Q / interaction=autonomous).

The user wants the agent to run the full cycle: frame → explore → compare → decide → implement. Only in this mode does the agent drive without pausing.

If autonomous mode is OFF, phrases like "figure out the best approach and do it" or "fix everything" are NOT enough to skip the compare → decide pause by themselves. Treat them as delegated reasoning or ask whether the user wants to enable autonomous execution.

**If unclear which mode:** default to research / prepare-and-wait. Never default to autonomous execution. Ask: "Want me to answer directly, prepare and wait, drive the reasoning through compare and stop for your choice, or drive the full cycle and implement?"

---

## What you have

### Haft tools (MCP) — persist reasoning as artifacts

| Tool | What it does | Slash command |
|------|-------------|---------------|
| `haft_note` | Record micro-decisions with rationale validation | `/h-note` |
| `haft_problem` | Frame problems and persist characterization dimensions on the ProblemCard | `/h-frame`, `/h-char` |
| `haft_solution` | Explore variants, compare and identify Pareto front | `/h-explore`, `/h-compare` |
| `haft_decision` | Decide with formal rationale; record measurement results | `/h-decide` |
| `haft_refresh` | Detect stale decisions, manage lifecycle | `/h-refresh` |
| `haft_query` | Search, status dashboard, file-to-decision lookup, FPF spec lookup | `/h-search`, `/h-status` |

### FPF spec lookup — prefer MCP when available

In MCP-native clients, use `haft_query(action="fpf", query="...")` first. `query` is required. `limit`, `full`, and `explain` are optional.

```text
haft_query(action="fpf", query="A.6")
haft_query(action="fpf", query="How do I route boundary statements?", limit=3, explain=true)
haft_query(action="fpf", query="Boundary Norm Square", full=true)
```

In shell-only environments, use the CLI:

```bash
haft fpf search "<query>"          # tiered retrieval: exact pattern id -> routes -> related -> keyword fallback
haft fpf search "A.6"              # exact pattern lookup by pattern id
haft fpf search "boundary routing" # route-aware natural-language lookup
haft fpf search "<query>" --full   # full section content
haft fpf section "<heading-or-id>" # exact heading or pattern id
```

Use exact pattern ids directly when you know them. Use route-style natural-language queries for concept lookup through the indexed spec. Use `--full` or `full=true` when you need the full section body instead of snippets.

---

## Feature maturity

Not all FPF concepts are at the same implementation depth. This matters — don't present tracked annotations as computed evaluations.

| Concept | Status | What Haft does |
|---------|--------|-----------------|
| Problem framing | **tracked** | Stores signal, constraints, targets, acceptance. You do the framing, Haft persists it. |
| Characterization | **tracked** | Stores comparison dimensions with scale/unit/polarity/role on the ProblemCard. Persisted via `haft_problem(action="characterize")`. |
| WLNK | **tracked** | Required label on variants. Haft stores the stated weakest link on explored variants and decisions. |
| Parity | **textual** | Stored as rules text. Not enforced or verified. You ensure parity yourself. |
| Pareto front | **tracked** | You identify the non-dominated set, Haft stores and displays it. Not computed automatically. |
| Stepping stones | **tracked** | Boolean flag on variants, shown in summary table. |
| MCP tool mode | **supported** | Agents can call `haft_*` tools directly when the client exposes MCP tools. |
| Command-driven mode | **supported** | Agents can also be steered with installed `/h-*` commands or prompts. |
| Refresh (valid_until) | **enforced** | All artifacts (decisions, problems, portfolios) with expired valid_until detected by scan. |
| Refresh triggers | **textual** | Stored in decision body. Only valid_until date is actually scanned. Text triggers are reminders, not automated checks. |
| CL (congruence) | **artifact-level** | Reliability/evidence calculations exist in artifact/runtime code. `haft_decision(action="evidence")` stores explicit evidence with CL values, but the tool does not auto-infer CL from arbitrary sources. |
| F-G-R | **textual** | Formality labels may exist in artifact data, but are not a first-class haft decision-tool workflow step. |
| NQD | **absent** | Multi-dimensional quality vectors are not implemented. Use comparison dimensions in exploration/compare instead. |
| Impact measurement | **tracked** | `haft_decision(action="measure")` records post-implementation findings against acceptance criteria. |
| Evidence attachment | **supported** | Use `haft_decision(action="evidence")` to attach explicit benchmark/test/research/audit evidence to an artifact. This complements `baseline` + `measure`; it does not replace post-implementation measurement. |

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

**Persist with**: `haft_problem(action="characterize", problem_ref="...", dimensions=[...], parity_rules="...")`

Define dimensions before `/h-explore` or `/h-compare`, then reuse the same characterized space during comparison instead of inventing dimensions mid-stream.

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
| **tactical** | Reversible, <2 weeks blast radius | compact frame → compact explore → compact compare → `/h-decide` |
| **standard** | Most architectural decisions | full frame → define dimensions → `/h-explore` → `/h-compare` → `/h-decide` |
| **deep** | Irreversible, security, cross-team | all standard steps + rich parity, runbook, refresh triggers |

**Default is tactical depth.** Escalate when: hard to reverse, multiple teams affected, or problem framing is unclear.
Depth changes how much evidence and structure you show, not whether explore/compare exist.
If a flow reaches `/h-decide`, it must first pass through `/h-explore` and `/h-compare` for the active portfolio.

---

## Key distinctions (always maintain)

- **Object ≠ Description ≠ Carrier** — the system, its spec, and its code are three things
- **Plan ≠ Reality** — a model is not the thing it models
- **Target system ≠ enabling system** — what must work vs who builds/maintains it
- **Design-time ≠ run-time** — Quint stores your reasoning artifacts (design-time). It does not measure, test, or verify your system (run-time). Don't confuse stored claims with validated evidence.

---

## Proactive agent behavior

When you have Quint tools available, use them automatically only when the task actually calls for persisted reasoning artifacts. Availability of tools is not itself a reason to create artifacts.

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
3. If you have explicit supporting or contradictory artifacts, attach them with `haft_decision(action="evidence", artifact_ref="<id>", evidence_content="...", evidence_type="benchmark|test|research|audit", evidence_verdict="supports|weakens|refutes")`
4. Call `haft_decision(action="measure", decision_ref="<id>", findings="...", verdict="accepted")` — record verified results

**Calling measure from memory without verification is a FPF B.5:4.3 violation.**
**Calling measure without a baseline (when `affected_files` exist) degrades evidence to CL1 self-evidence.**
**Calling evidence without measure is not enough to close the implementation loop.**

This closes the lemniscate. Without measure, the decision stays open.

### Proactive checks (at key moments)

- **At session start**: call `haft_query(action="status")` to surface stale decisions and active problems
- **When /h-status shows pending decisions**: check if they were actually implemented and measured.
- **When code changed after a decision**: do NOT summarize drift without reading diffs. For each modified file: (1) read `git diff`, (2) classify as cosmetic / incidental / material to the decision's invariants, (3) present classification to user.
- **If status returns zero artifacts on a project with code**: suggest `/h-onboard`
- **When dev works on files**: call `haft_query(action="related", file="path")` to find linked decisions — mention them if relevant
- **When dev says "let's just do X" without rationale**: ask "why X?" before recording
- **When auto-captured note conflicts with an active decision**: surface the conflict clearly

### User steering

Slash commands are steering handles, not always mandatory triggers:
- In **research / prepare-and-wait** mode, the user explicitly triggers `/h-frame`, `/h-char`, `/h-explore`, or `/h-compare` when ready.
- In **delegated reasoning** mode, natural-language delegation like "давай", "do it", or "go ahead" is enough to continue through frame → explore → compare without waiting for a manual `/h-explore`.
- In symbiotic reasoning, stop at comparison and wait for the human's post-compare choice before `/h-decide`.
- `/h-frame` — full problem framing (diagnostic conversation)
- `/h-explore` — variant generation
- `/h-compare` — parity comparison
- `/h-decide` — formal decision record

Direct operational requests should stay direct. Do not escalate them into `/h-frame` just because they touch engineering content.

### NavStrip interpretation

The `── Quint ──` strip appended to tool responses shows current state and available actions. Key rules:

- **"Available:" = menu for the user, not instructions for the agent.** Do not auto-execute these actions.
- **Mode determines ceremony depth, not whether decide may bypass exploration/comparison.** The Available line reflects the current state, but any path that reaches `/h-decide` must already have an explored and compared active portfolio.
- **Fewer words ≠ a weaker decision boundary.** Tactical depth keeps frame/explore/compare compact; it does not license jumping straight from frame to decide.
- **Research / prepare-and-wait vs delegated reasoning:** In research + wait mode, do not auto-advance from Available actions. In delegated reasoning mode, you may advance through frame/explore/compare without extra slash commands.
- **Autonomous execution override:** Only when the user has explicitly delegated full autonomy ("and implement", "fix everything") and autonomy is enabled may the agent proceed through decide/implement/measure without pausing.

---

## RAG search reference

Use MCP-native FPF lookup when tools are available. Fall back to `haft fpf` in shell-first environments. Reach for this when you need formal definitions, templates, aggregation rules, conformance checklists, or exact pattern-id lookup.

```text
haft_query(action="fpf", query="problem card PROB")
haft_query(action="fpf", query="A.6", full=true)
haft_query(action="fpf", query="How do I route boundary statements?", limit=3, explain=true)
```

```bash
haft fpf search "problem card PROB"
haft fpf search "A.6" --full
haft fpf section "A.6"
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
