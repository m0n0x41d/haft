# Mode Ontology

> Reading order: 4 of N. Read after SCOPE_FREEZE.

## The 5 Engineering Modes

Modes describe **what kind of engineering work** is happening. They are sequential in the common case, with legitimate reroutes upstream.

```
Understand → Explore → Choose → Execute → Verify
    ↑            ↑        ↑        ↑        │
    └────────────┴────────┴────────┴────────┘
                    reroutes
```

### Understand

**Question:** "What's actually going on?"

| Attribute | Value |
|-----------|-------|
| Entry | `/h-frame` or `/h-reason` |
| Output | ProblemCard |
| Tool | `haft_problem(action="frame")` |
| Optional | Characterization via `haft_problem(action="characterize")` |

**Framing protocol (questions before recording):**
1. What observation doesn't fit? (signal, not assumed cause)
2. What have you already tried?
3. What would solved look like? (measurable acceptance)
4. What constraints are non-negotiable?
5. How reversible is this? What's the blast radius?
6. What should we watch but NOT optimize? (observation indicators)

**Problem typing:** Classify before exploring:
- Optimization — working system, want it better on a known dimension
- Diagnosis — something's broken, don't know why
- Search — need something that doesn't exist yet
- Synthesis — combine existing elements into something new

**Language precision triggers:** If signal uses ambiguous terms (service, process, quality, component) → unpack to precise meaning before recording.

### Explore

**Question:** "What are the real options?"

| Attribute | Value |
|-----------|-------|
| Entry | `/h-explore` (or `/h-char` first for characterization) |
| Output | SolutionPortfolio with 2+ variants |
| Tool | `haft_solution(action="explore")` |
| Prerequisite | Active ProblemCard |

**Variant requirements:**
- 2+ variants minimum (3+ preferred)
- Must differ in **kind**, not degree
- Each gets: description, strengths, weakest link (WLNK), risks
- Stepping stone flag for options that open future paths
- Diversity check: >50% word overlap = warning

**Characterization (optional but recommended for standard+ depth):**
- Define comparison dimensions BEFORE seeing options
- Tag each dimension with indicator role: constraint / target / observation
- State selection policy BEFORE results

### Choose

**Question:** "Which is better — and should I even decide now?"

| Attribute | Value |
|-----------|-------|
| Entry | `/h-compare` |
| Output | Comparison results on SolutionPortfolio |
| Tool | `haft_solution(action="compare")` |
| Gate | Probe-or-commit assessment |

**Probe-or-commit gate (before comparison):**

| Outcome | Meaning | Next action |
|---------|---------|-------------|
| **commit** | All dimensions covered, variants diverse, no investigation would change ranking | Proceed to compare |
| **probe** | Specific investigation could change ranking | Name the investigation, its cost, and what comparison defect it repairs |
| **widen** | All variants are from same family | Go back to Explore with diversity requirement |
| **reroute** | The burden shifted — this isn't a choice problem | Go back to Understand with new framing |

**Comparison mechanics:**
- Constraint elimination first (hard limits remove variants)
- Pareto front computation (non-dominated set)
- Parity enforcement (same conditions for all variants)
- Trade-off table, not recommendation essay

**Transformer Mandate:** Agent compares and may persist an **advisory recommendation** (`selected_ref` + `recommendation_rationale`). The human confirms or overrides before `/h-decide` records the actual decision. `selected_ref` is advisory — it never crosses the human choice boundary by itself.

Exception: autonomous execution mode (explicitly enabled per-session). Even in autonomous mode, one-way-door decisions require confirmation.

### Execute

**Question:** "Let's do it — carefully."

| Attribute | Value |
|-----------|-------|
| Entry | `/h-decide` |
| Output | DecisionRecord + baseline |
| Tool | `haft_decision(action="decide")` |
| Post-action | Auto-baseline when affected_files present |

**DecisionRecord contents:**
- Selected variant + why
- Invariants (what MUST hold)
- Pre-conditions, post-conditions, admissibility
- Claims with `observable`, `threshold`, optional `verify_after`
- Rollback plan + triggers
- Affected files
- Valid-until date
- Weakest link

**Adversarial verification gate (before recording):**
- Tactical: one-line counter-argument
- Standard/deep: 5 probes (deductive consequences, strongest counter-argument, self-evidence check, tail failure, WLNK challenge)

### Verify

**Question:** "Did it work? Is it still valid?"

| Attribute | Value |
|-----------|-------|
| Entry | `/h-verify` |
| Tool | `haft_refresh(action="scan")` + `haft_decision(action="measure")` |
| Covers | 4 distinct functions via one surface |

**Four internal state machines:**

| Function | What it does |
|----------|-------------|
| Empirical verification | Run measurement, check claim against threshold |
| Drift detection | Compare current file hashes against baseline |
| Staleness scanning | Find expired valid_until, compute R_eff degradation |
| Pending scheduling | Surface claims with past verify_after dates |

**Lifecycle actions:** waive, reopen, supersede, deprecate, reconcile (note-decision overlap check)

### Note (Fast Path)

**Question:** "Quick, capture this before it's forgotten."

| Attribute | Value |
|-----------|-------|
| Entry | `/h-note` or auto-capture by agent |
| Output | Note artifact |
| Tool | `haft_note` |

Validation: rationale required, conflict check against active decisions, overlap check (>70% = rejected), scope check (many files → suggest /h-frame). Auto-expires 90 days.

## Legitimate Reroutes

| From | To | Trigger |
|------|----|---------|
| Choose → Understand | Comparison reveals ambiguous framing or category error |
| Choose → Explore | All variants from same family, need genuine diversity |
| Explore → Understand | Exploration reveals wrong problem type |
| Execute → Choose | Implementation reveals chosen option doesn't work |
| Execute → Understand | Implementation reveals different problem than framed |
| Verify → Understand | Verification shows problem was misframed |
| Verify → Explore | Chosen option failed, need new variants |
| Verify → Choose | Drift invalidated comparison basis |

Reroutes are not failures. They prevent compounding errors downstream.

## Depth Calibration

| Depth | When | Ceremony |
|-------|------|----------|
| **note** | Micro-decisions during coding | `/h-note` — done |
| **tactical** | Reversible, <2 weeks blast radius | Compact Understand + Explore + Execute. Skip characterization, skip formal comparison. |
| **standard** | Most architectural decisions | Full 5-mode cycle with characterization, 3+ variants, Pareto comparison |
| **deep** | Irreversible, security, cross-team | All standard + rich parity rules, rollback runbook, refresh triggers, evidence requirements |

Default is tactical. Escalate when: hard to reverse, multiple teams affected, problem framing unclear.

## Interaction Modes (orthogonal to engineering modes)

| Interaction mode | Who drives | Agent stops at |
|-----------------|-----------|----------------|
| **Direct response** | Human asks, agent responds directly | No MCP calls unless persistence requested |
| **Research / prepare-and-wait** | Agent gathers context, then stops | After presenting findings |
| **Delegated reasoning** | Agent drives frame→explore→compare | After compare (Pareto shown, human chooses) |
| **Autonomous execution** | Agent drives full cycle | Only when autonomous mode is ON for the session |
