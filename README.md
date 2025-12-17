<img src="assets/banner.svg" alt="Quint Code" width="600">

**Structured reasoning for AI coding tools** — make better decisions, remember why you made them.

**Supports:** Claude Code, Cursor, Gemini CLI, Codex CLI

> **Works exceptionally well with Claude Code!**

## The Problem This Solves

You're deep in a codebase. You need to handle cross-service transactions in a system that's grown beyond the point where distributed transactions are viable. Event choreography? Saga pattern? Outbox with CDC? Each has non-obvious failure modes.

Or: you're adding real-time features to an existing app. WebSockets vs SSE vs polling? The "obvious" answer depends on your infrastructure, scale trajectory, and failure tolerance — none of which fit neatly into a Stack Overflow answer.

Your AI tool gives you *an* answer. It's coherent. But:

- **Why** this approach? (You won't remember the reasoning in 3 months)
- **What alternatives** were considered? (Were there alternatives, or did you anchor on the first idea?)
- **What evidence** supported it? (Documentation? Benchmarks? Or just pattern-matching from training data?)
- **When does this decision expire?** (The tradeoffs shift as the system evolves)

FPF gives you a structured way to think through these decisions with your AI tool as a reasoning partner — not a black-box oracle. You generate hypotheses, verify them, test them, and document *why* you chose what you chose.

## Quick Start

### One-liner Install

##### Global install (recommended for personal use)
```bash
curl -fsSL https://raw.githubusercontent.com/m0n0x41d/quint-code/main/install.sh | bash -s -- -g
```

##### Per-project install (run from project root)
```bash
curl -fsSL https://raw.githubusercontent.com/m0n0x41d/quint-code/main/install.sh | bash
```

This launches an interactive installer where you select which AI coding tools to install FPF commands for (Claude Code, Cursor, Gemini CLI, Codex CLI).

> **Tip:** Cursor can import `.claude/commands/` directly — install for Claude Code, use in both! In Cursor: Settings → Rules and Commands. The toggle will be right there!

**Global** installs to `~/.claude/commands/` etc. — available in all projects.
**Per-project** installs to `./.claude/commands/` — commit to repo for team use.

### Manual Install

```bash
git clone https://github.com/m0n0x41d/quint-code.git
cd quint-code
./install.sh                    # Interactive TUI
./install.sh --claude -g        # Install Claude Code globally
./install.sh --all              # Install all platforms
```

### Initialize in Your Project

```bash
cd /path/to/your/project
# Then in your AI coding tool:
/q0-init  # Creates .quint/ structure and scans your repo

# Start reasoning
/q1-hypothesize "How should we handle state synchronization across browser tabs?"
```

Follow the prompts. The AI guides you through the cycle. You make the calls.

## What Is This, Really?

This project offers a **set of slash commands** for AI coding tools that **leverage the `quint-mcp` (Model Context Protocol) Go module** as its core logic for **hypothesis-driven reasoning** following the **First Principles Framework (FPF)** workflow. This powerful combination enables AI coding tools to:

-   Generate competing ideas
-   Verify them logically
-   Test them empirically
-   Manage their lifecycle (L0, L1, L2, Invalid)
-   Document the rationale behind decisions

The various platform-specific installations (Claude Code, Cursor, Gemini CLI, Codex CLI) integrate `quint-mcp`'s functionality through these slash commands, acting as a user-friendly interface to this powerful FPF engine.

Based on the [First Principles Framework (FPF)](https://github.com/ailev/FPF) by Anatoly Levenchuk.

**The FPF Workflow Phases and Core Loop (managed by `quint-mcp`):**

The `quint-mcp` (Model Context Protocol) project manages the FPF workflow, ensuring a structured approach to problem-solving. Hypotheses move through distinct knowledge levels (L0, L1, L2, Invalid) based on their verification and validation status.

1.  **IDLE**: The initial state, or after a decision is finalized.
2.  **ABDUCTION (Hypothesize)**: Generate multiple hypotheses (L0). Outputs unverified ideas.
3.  **DEDUCTION (Check)**: Verify logical consistency. Moves L0 hypotheses to L1/ (Substantiated) or Invalid/.
4.  **INDUCTION (Test/Research)**: Test hypotheses empirically. Moves L1 hypotheses to L2/ (Validated/Axiomatic) or Invalid/. Loopback to ABDUCTION for refinement is supported.
5.  **DECISION**: Finalize the decision, archive the session. Moves L2 hypotheses to the final decision and related files.

AI generates. You decide.

### Before/After

| Without FPF | With FPF |
|-------------|----------|
| "Let's use the Saga pattern" (first idea that came up) | 3 approaches analyzed with evidence, tradeoffs documented |
| "Why did we build it this way?" (3 months later) | Check `DRR-001.md` — full rationale preserved |
| "Is this assumption still valid?" | Run `/q-decay` — evidence freshness tracked |
| Same architectural debates, different meetings | Query knowledge base — past decisions searchable |

## Why "Quint"?

The name references the **Invariant Quintet** — five properties that FPF requires any valid aggregation or composition to preserve. They're not arbitrary rules; they're formalizations of common-sense engineering realities that hold across domains.

### The Five Invariants

| Invariant | Plain English | Example |
|-----------|---------------|---------|
| **IDEM** (Idempotence) | One part alone stays itself | A single hypothesis doesn't magically gain confidence by being the only option |
| **COMM** (Commutativity) | Order of independent parts doesn't matter | Gathering evidence A then B = gathering B then A (same conclusion) |
| **LOC** (Locality) | Which worker/machine runs the analysis is irrelevant | Your reasoning should be reproducible regardless of who re-runs it |
| **WLNK** (Weakest Link) | The whole is only as strong as its weakest part | Your decision confidence = min(evidence confidences), never average |
| **MONO** (Monotonicity) | Improving a part cannot worsen the whole | Adding better evidence can only help; it can't make your case weaker |

**Mnemonic:** S-O-L-I-D → Same · Order-free · Location-free · Inferior-cap · Don't-regress

### How Quint Code Supports Them

| Invariant | Support Level | Implementation |
|-----------|---------------|----------------|
| **WLNK** | ✅ Explicit | R_eff calculations, Φ(CL) penalties, "min(evidence) never average" enforced in `/q4-audit` |
| **IDEM** | ⚠️ By design | Single items pass through unchanged, but no computational verification |
| **COMM** | ⚠️ By design | `/q3-test` and `/q3-research` run in any order; min() is commutative |
| **LOC** | ⚠️ By design | Git-trackable artifacts, reproducible methods documented |
| MONO | ✅ Enforced | Hypotheses progress L0→L1→L2. Invalidated hypotheses move to `invalid/` (never deleted) for future learning and historical audit. |

**Honest assessment:** WLNK is the only invariant with explicit enforcement. The others are *preserved by the design* of the workflow rather than *verified computationally*. For a CLI tool that assists reasoning, this is the right trade-off. Full formal verification would require something closer to a theorem prover.

The 5-step sequence (`q1`–`q5`) maps to the **ADI cycle** (Abduction → Deduction → Induction + Audit → Decision), not directly to the five invariants. But the invariants inform *why* the cycle works: you can't skip steps (MONO), evidence order doesn't matter (COMM), and your conclusion is capped by your weakest evidence (WLNK).

### Why This Matters

Most reasoning failures in software architecture come from violating WLNK: averaging away weak evidence, hiding uncertainty behind confident prose, or treating "mostly sure" as "sure." Quint Code makes WLNK violations visible. The other invariants ensure that the process of getting there is sound.

If you want the full theory: [First Principles Framework specification](https://github.com/ailev/FPF).

So after all, **quint** is a great name for this tool — it is ~5% distilled *quintessence* of FPF, and it tries to follow all *five* invariants 😊.

## The ADI Cycle

The ADI cycle is **strictly sequential** — each phase builds on the output of the previous one. You cannot skip ahead or reorder phases 1-3.

```
Problem Statement
       │
       ▼
┌──────────────────────────────────────────────┐
│  1. ABDUCTION (Hypothesize)                  │
│  Generate 3-5 competing hypotheses           │
│  Output: L0/ (new hypotheses)                │
└──────────────────────┬───────────────────────┘
                       ▼ (REQUIRED)
┌──────────────────────────────────────────────┐
│  2. DEDUCTION (Check)                        │
│  Verify logical consistency                  │
│  Output: L0/ -> L1/ (logically sound) or invalid/ (on FAIL/REFINE) │
└──────────────────────┬───────────────────────┘
                       │ (REQUIRED)
          ┌────────────┴────────────┐
          ▼                         ▼
   ┌─────────────┐          ┌─────────────┐
   │  3a. TEST   │          │ 3b. RESEARCH│
   │  Internal   │          │  evidence   │
   └──────┬──────┘          └──────┬──────┘
          └────────────┬───────────┘
                       │
                       ▼ (OPTIONAL - but recommended)
┌──────────────────────────────────────────────┐
│  4. AUDIT                                    │
│  WLNK analysis, bias check, adversarial view │
└──────────────────────┬───────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────┐
│  5. DECIDE                                   │
│  Create DRR, archive session                 │
│  Output: L2/ (final decision), decisions/DRR, session archived │
└──────────────────────────────────────────────┘
```

### Phase Strictness

**Phases 1 → 2 → 3: Strictly sequential.** You cannot run `/q2-check` without hypotheses from phase 1. You cannot gather evidence (phase 3) without logically verified hypotheses from phase 2. The commands enforce this — they will error if prerequisites aren't met.

**Phase 4 (Audit): Optional, but recommended.** You can skip directly from phase 3 to `/q5-decide`. This produces a valid Design Rationale Record (DRR). However, skipping audit is generally not recommended — the WLNK analysis and bias check typically take only a few minutes and often catch blind spots that would otherwise ship with your decision.

**When skipping audit is acceptable:**

- Time-critical decisions where you've already done thorough evidence gathering
- Simple decisions with clear evidence and no competing hypotheses remaining
- You've committed to the ADI cycle and want to complete it — better to finish with a DRR than abandon mid-cycle

**When you should NOT skip audit:**

- Multiple hypotheses survived to phase 3 with similar evidence strength
- Evidence came primarily from external sources (congruence assessment needed)
- The decision affects multiple teams or has long-term architectural impact

## Testing and Code Quality

The `quint-mcp` project prioritizes code quality and reliability.

- **Comprehensive Testing:** The project includes a suite of unit tests for core FSM logic (`fsm_test.go`) and file system operations (`tools_test.go`). A robust integration test (`integration_test.go`) simulates the entire FPF workflow, ensuring seamless operation across phases and file lifecycle management. All tests are executed using Go's native testing framework.
- **Static Analysis:** Code quality is continuously maintained and enforced using `golangci-lint`, a popular Go linter aggregator. This ensures adherence to best practices, identifies potential bugs, and improves code readability and maintainability.

## Commands Reference

### Core Cycle

| Command | Role | Phase | What It Does | Required |
|---------|------|-------|--------------|----------|
| `/q0-init` | Any | Setup | Create `.quint/` structure | Yes |
| `/q1-hypothesize` | Abductor | Abduction | Generate competing hypotheses → `L0/` | Yes |
| `/q2-check` | Deductor | Deduction | Verify logical consistency → `L1/` (on PASS) or `invalid/` (on FAIL/REFINE) | Yes |
| `/q3-test` | Inductor | Induction | Run internal tests, benchmarks → `L2/` (on PASS) or `invalid/` (on FAIL/REFINE/LOOPBACK) | Yes* |
| `/q3-research` | Inductor | Induction | Gather external evidence (web, docs) → `L2/` (on PASS) or `invalid/` (on FAIL/REFINE/LOOPBACK) | Yes* |
| `/q4-audit` | Auditor | Audit | WLNK analysis, bias check | No** |
| `/q5-decide` | Decider | Decision | Create DRR, finalize, archive session | Yes |

*At least one of `/q3-test` or `/q3-research` required before proceeding.
**Optional but recommended — see [Phase Strictness](#phase-strictness) above.

### Utilities

| Command | Purpose |
|---------|---------|
| `/q-status` | Show current phase and next steps |
| `/q-query <topic>` | Search knowledge base |
| `/q-decay` | Check evidence freshness |
| `/q-reset` | Abandon cycle, preserve learnings |
| `/q-actualize` | Migrate legacy `.fpf` structure to `.quint` and reconcile state |

## Key Concepts

### Deep Reasoning (v3.1+)

Quint Code now enforces **Context Slicing**, **Explicit Roles**, and **Typed Reasoning** to prevent "works on my machine" architecture and AI drift.

**1. Context Slicing (A.2.6)**
Truth depends on where you stand. `q0-init` now scans your repo to generate structured slices.

**2. Explicit Role Injection**
The AI enacts specific FPF roles: Explorer, Logician, Auditor.

**3. Typed Hypotheses (C.3 Kind-CAL)**
Hypotheses are typed as **System** or **Episteme**.
- **Deductor** applies different logic: Structural/Component checks for Systems, Consistency/Clarity checks for Epistemes.

**4. Structured Decisions (E.9 & C.16)**
Decisions are recorded as formal **DRRs** backed by a **Characteristic Space** (C.16).
- **Abductor** defines Success Metrics (e.g., "Latency < 50ms").
- **Inductor** measures and records actuals against them.

### Assurance Levels

| Level | Name | Meaning | How to Reach |
|-------|------|---------|--------------|
| **L0** | Observation | Unverified hypothesis | Created by /q1-hypothesize in L0/ |
| **L1** | Substantiated | Passed logical check | Moved from L0/ to L1/ by /q2-check (on PASS) |
| **L2** | Validated | Empirically tested | Moved from L1/ to L2/ by /q3-test or /q3-research (on PASS) |
| **Invalid** | Disproved | Was wrong — kept for learning | Moved from L0/, L1/, or L2/ to invalid/ (on FAIL/REFINE/LOOPBACK) |

### Formality (F-Score)

Rigor of expression (not truth, but precision).

- **F0-F2 (Sketch):** Rough ideas, whiteboard notes, vague constraints.
- **F3-F5 (Structured):** Steps, clear constraints, executable code/tests.
- **F6-F9 (Rigorous):** Formal proofs, math models, machine-checked invariants.

Most engineering work targets **F3-F5**.

### WLNK (Weakest Link)

**Critical rule:** Assurance = min(evidence), NEVER average.

```
Evidence A: L2 (internal benchmark)     → R_eff = 1.0
Evidence B: L2 (official docs)          → R_eff = 1.0
Evidence C: L1 (blog, low congruence)   → R_eff = 0.65

Combined Assurance: R_eff = 0.65 (capped by weakest)
```

One weak piece of evidence caps your entire argument. If you want solid decisions, you need to strengthen — or acknowledge — your weakest link.

### NQD (Novelty, Quality, Diversity)

We track three metrics to ensure we aren't just guessing:

- **Novelty:** How new is this idea? (Conservative vs. Radical). We want a mix.
- **Quality:** How likely is it to work? (High complexity = Higher risk).
- **Diversity:** Are we exploring different *types* of solutions? (e.g. Architectural vs. Operational).

### Method vs. Work

Quint Code strictly separates the **Plan** from the **Result**.

- **Method (Design-Time):** The recipe. The code you plan to write. The "How-To".
- **Work (Run-Time):** The cooking. The test results. The logs. The "What Happened".

Hypotheses now define the **Method** (Plan) first, then outline the **Validation** (Work) needed to prove it.

### Congruence (for External Evidence)

External evidence may not apply to your context. Assess congruence:

| Level | Φ Penalty | Example |
|-------|-----------|---------|
| **High** | 0.00 | Same tech, similar scale, similar use case |
| **Medium** | 0.15 | Same tech, different scale |
| **Low** | 0.35 | Related tech, very different context |

Formula: `R_eff = R_base - Φ(CL)`

### Transformer Mandate

**AI generates options. You decide.**

This is non-negotiable. At every decision point, the AI presents analysis and waits for your input.
A system cannot transform itself — external decision-maker always required.

## When to Use FPF

### Use FPF When

- **Architectural decisions** — Long-term consequences need documented rationale
- **Multiple viable approaches** — Systematic comparison prevents anchoring bias
- **Team decisions** — Auditable trail for async review and future onboarding
- **Unfamiliar territory** — Structured research reduces "confident but wrong" outcomes
- **Decisions you'll need to defend** — Evidence is stored; re-evaluation becomes trivial

### Don't Use FPF When

- **Quick fixes** — Just fix it
- **Genuinely obvious solutions** — Implement directly. But be honest: is it *actually* obvious, or just familiar? Obvious to whom?
- **Easily reversible choices** — Try it, iterate, learn
- **Time-critical situations** — Use built-in plan mode instead
- **Well-understood patterns** — Apply known patterns. Same caveat as "obvious" applies.

### Decision Heuristic

```
Is this decision:
  □ Hard to reverse?
  □ Affecting more than a few days of work?
  □ Involving multiple unknowns?
  □ Likely to be questioned later?

If any checked → Consider FPF
If none → Skip FPF, use built-in plan mode or just decide
```

## Installation

See [Quick Start](#quick-start) for one-liner install.

```bash
# Clone and install
git clone https://github.com/m0n0x41d/quint-code.git
cd quint-code
./install.sh                    # Interactive TUI
./install.sh --claude -g        # Claude Code globally
./install.sh --cursor           # Cursor locally
./install.sh --all -g           # All platforms globally
```

## Directory Structure

After initialization:

```
your-project/
└── .quint/
    ├── knowledge/
    │   ├── L0/           # Hypotheses (Unverified ideas)
    │   ├── L1/           # Hypotheses (Logically sound)
    │   ├── L2/           # Hypotheses (Empirically tested, Verified)
    │   └── invalid/      # Hypotheses (Disproved, Refined, or Failed - kept for learning)
    ├── evidence/         # Test results, research findings
    ├── decisions/        # DRRs (Design Rationale Records)
    ├── sessions/         # Archived reasoning cycles
    ├── session.md        # Current cycle state
    ├── context.md        # Project context, constraints, and tech stack (new in v2.1.0)
    └── config.yaml       # Project settings (optional)
```

## Example Walkthrough

### Problem: Handling Eventual Consistency

Your order service needs to update inventory, notify the warehouse, and send a confirmation email — all reliably. Distributed transactions aren't viable at your scale.

```bash
/q1-hypothesize "How should we handle cross-service coordination for order fulfillment?"
```

**AI generates hypotheses:**

- H1: Saga pattern with orchestrator — Central coordinator manages the workflow
- H2: Event choreography — Services react to domain events, no coordinator
- H3: Outbox pattern with CDC — Transactional outbox, Change Data Capture for publishing

**You review and approve.** Then:

```bash
/q2-check
```

The AI checks logical consistency. H1 and H3 pass. H2 has issues: circular dependencies emerge when you trace the event flow. H2 stays L0 with noted concerns.

```bash
/q3-research  # Check external evidence
/q3-test      # Run failure scenario simulations
```

Evidence gathered. H3 reaches L2 (Postgres + Debezium is well-documented, tested with simulated network partitions). H1 reaches L1 (good in theory, but your team lacks orchestrator experience).

```bash
/q4-audit
```

WLNK analysis: H3's weakest evidence is the CDC setup complexity, but it's a known quantity. H1's weakest link is team experience — harder to derisk.

```bash
/q5-decide
```

The AI presents the analysis. You choose H3. DRR created:

`decisions/DRR-001-order-fulfillment-coordination.md`

The DRR includes: what you chose, why, what alternatives existed, evidence that supported it, and conditions that would trigger re-evaluation (e.g., "if order volume exceeds 10k/day").

### Six Months Later

```bash
/q-query "order fulfillment"
```

New team member asks why you use the outbox pattern. You point them to the DRR. Full context preserved — not just *what*, but *why* and *when to reconsider*.

## Core Principles

These aren't arbitrary rules — they're what makes the framework actually work:

1. **Transformer Mandate** — AI generates options; you decide. A system cannot objectively evaluate its own outputs.
2. **Evidence Anchoring** — Every decision traces back to specific evidence. No "I just think this is better."
3. **Falsifiability** — Each hypothesis specifies what would disprove it. If nothing could prove it wrong, it's not useful.
4. **WLNK (Weakest Link)** — Your argument is only as strong as its weakest evidence. No averaging away uncertainty.
5. **Bounded Validity** — Knowledge has scope (applies under specific conditions) and expiry (becomes stale). Both are explicit.
6. **Explicit Over Hidden** — Assumptions get documented, not buried in code comments or forgotten.

## Troubleshooting

### "This is slowing me down"

Then don't use it. FPF is for decisions that *warrant* the overhead. For quick decisions, just use built-in plan mode or decide directly.

### "All the hypotheses look the same"

Force diversity. The commands expect at least one conservative approach, one innovative approach, and one minimal approach. If you're getting samey options, your problem statement might be too narrow — add more context about constraints, tradeoffs, or what you're optimizing for. Between commands, you can chat normally to explore the space.

### "The evidence is from a different context"

Assess congruence. Evidence from a different scale, tech stack, or domain gets penalized. Low congruence = high penalty to effective assurance. Consider running internal tests to verify external claims in your actual context.

### "This evidence is old — is it still valid?"

Run `/q-decay`. You can refresh it, deprecate it, or waive the freshness requirement with justification.

### "Audit found blockers"

Resolve them before `/q5-decide`. That's the point of auditing.

### "Need to revisit an old decision"

Run `/q-query <topic>`. Check the DRR's validity conditions. If they've been triggered, start a new cycle.

## Learning FPF

The best way to learn FPF is to use it on itself:

1. Clone the original [FPF spec repo](https://github.com/ailev/FPF)
2. Install quint-code in it
3. Run `/q1-hypothesize "What is the core insight of FPF and how should I apply it?"`

The framework teaches itself through use. A few cycles and it clicks.

## License

This software is available under the [MIT License](LICENSE).

**Note on Intellectual Property:**
The **First Principles Framework (FPF)** methodology, concepts, and ontology belong to **Anatoly Levenchuk**. This project is an independent implementation of those concepts for AI coding tools. We claim no ownership over the FPF methodology itself.

## Contributing

Issues are welcome.