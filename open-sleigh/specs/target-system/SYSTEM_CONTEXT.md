---
title: "1. System Context"
description: Subsystem-level system context for Haft's commissioned execution runtime — why it exists, its role, who it affects
reading_order: 1
---

# Open-Sleigh: System Context

> **Subsystem note.** Product target system = **Haft**. This document scopes
> the commissioned execution subsystem currently implemented by
> **Open-Sleigh** and exposed operator-side as `haft harness`. Local
> references to "target system" below mean this subsystem, not a peer product.

## 1. What needs to change in the environment of the target system

### The physical world today (without Haft's harness runtime)

A small engineering organisation (single principal or tiny team) runs
software development through Haft reasoning artifacts and optional external
coordination carriers (Linear, GitHub Issues, Jira). The human does
**framing** (problem definition, decomposition, scope), **choice** (which
DecisionRecord is authoritative), and **commissioning** (which work may run
now or later). AI coding agents (Codex CLI, Claude Code) author code after the
work is commissioned. Framing is expensive cognitive work; execution
supervision is repetitive toil.

**What goes wrong physically:**

- **Agent work is un-gated.** The agent runs, produces a PR, and
  whatever happens happens. There's no structural check that the agent
  understood the ticket, distinguished design from runtime, produced
  evidence external to its own claims, or declared when its own work is
  stale. The human has to catch these by reading.
- **Framing and decisions evaporate before execution.** The human does careful
  problem framing and chooses a DecisionRecord, but actual implementation may
  start days later. If no WorkCommission and preflight boundary exists, stale
  decisions can be executed as if nothing changed.
- **No provenance on agent output.** Artifacts produced by the agent
  (PRs, commit messages, test output) are not linked back to the
  specific prompt version, tool allowlist, or adapter version that
  produced them. When an artifact is later audited, it's "the agent did
  it" — no deeper.
- **Human oversight is binary.** Either the human approves every PR
  (bottleneck) or approves none (risk). There's no structural surface
  for "human approves this narrow class of transitions, agent handles
  the rest."
- **Agent failures are invisible.** If the agent is silently gaming a
  proxy metric (writing tests that pass but don't cover the bug,
  refactoring without fixing), the failure looks like a success until
  the next reopen.
- **The meta-level eats the work.** When the human adds "process" to
  fix the above, the process itself grows unbounded (see
  `~/Repos/hustles/thai/` as a case study: 105KB CLAUDE.md, 70K lines
  of tooling, 99% of commits from the agent writing governance rather
  than product code).

### The physical world after (with the harness runtime functioning in its role)

- Agent work passes through a **phase-gated lifecycle**: Preflight → Frame →
  Execute → Measure. Each phase has a scoped prompt, a scoped toolset, typed
  gates (structural / semantic / human), and a typed artifact handed
  forward to Haft as evidence.
- Framing and choice produced upstream by the human (via Haft + `/h-reason`)
  enter Open-Sleigh as a linked `WorkCommission`. Open-Sleigh **verifies** the
  commission, its ProblemCard, and its DecisionRecord are present, fresh, and
  not self-authored by Open-Sleigh; it does not re-derive framing or decide
  that stale work is still admissible. A commission without a valid upstream
  frame/decision fail-fasts at Preflight/Frame and never enters Execute.
- Every artifact carries `config_hash` + `valid_until` +
  `authoring_role`. Provenance is structural, not a discipline.
- Human oversight is **structurally targeted**: HumanGate fires only on
  transitions that cross a reversibility / blast-radius threshold or leave the
  approved AutonomyEnvelope. Inside a phase, the agent has full runway.
- Failure-mode signals (gate_bypass_rate, reopen_after_measure_rate,
  judge false-neg rate) are observation indicators (§6d of SPEC) —
  watched, never optimised. The agent cannot game the gates because
  the observation surface watches the gaming.
- **The meta-level is bounded by construction.** `sleigh.md` has a
  300-line size budget, enforced by the L6 compiler. `ObservationsBus`
  has no compile-time path to the Haft artifact graph. The engine
  cannot document itself into oblivion because the module graph refuses
  it.

### The measurable change

| Metric | Before | After |
|---|---|---|
| Provenance on agent output | Agent signature in commit, nothing else | `config_hash` + `valid_until` + `authoring_role` on every artifact |
| Human intervention points | Every PR (bottleneck) or none (risk) | Only on transitions crossing a declared blast-radius threshold |
| Stale artifact detection | Manual | `haft_refresh` scans `valid_until` every poll tick |
| Agent gaming surface | Opaque | `gate_bypass_rate`, `judge_false_neg_rate`, `reopen_after_measure_rate` tracked as observations |
| Framing-to-execution link | Implicit, often broken | Explicit WorkCommission links to ProblemCard + DecisionRecord; engine fails fast if stale/missing |
| Governance documentation growth | Unbounded (see thai case) | Compile-time size budget on `sleigh.md`; zero harness-about-harness artifacts permitted |

---

## 2. By what method does the target system change its environment

The method: **OTP-supervised orchestration of AI coding agents through an
FPF-compliant governance lifecycle.**

Specific sub-methods:

### Lifecycle sub-methods

1. **Phase gating.** A `WorkCommission × Phase` session cannot exit a phase
   without its structural, semantic, and (where applicable) human
   gates returning pass verdicts. Gates are typed; their kinds cannot
   be confused.
2. **Scoped capability.** Each phase has a compile-time-fixed toolset.
   An agent in Frame phase cannot call `write` or `bash`; an agent in
   Execute cannot call `haft_decision`. Capability leak is a
   constructor-level or runtime-guard enforcement (see
   `ILLEGAL_STATES.md §Four-label enforcement taxonomy`), not a
   discipline. Elixir's type system is not Haskell-grade; the
   four-label taxonomy is how we stay honest about *which kind* of wall
   stops each illegal state.
3. **Provenance pinning.** Every artifact carries `config_hash`
   (sha256 of the effective `sleigh.md` slice), `valid_until`
   (refresh-by date), and `authoring_role`. Constructors refuse
   artifacts without these.
4. **Evidence externalisation.** Measure phase emits typed evidence
   whose `ref` cannot equal the authoring artifact's `self_id`. No
   self-evidence at the type level.
5. **Human targeting.** HumanGate fires on a declared set of transitions (PR
   → `main`, external publication, out-of-envelope change); elsewhere the
   agent runs unsupervised inside the approved commission/envelope.

### Supervision sub-methods

1. **Single-writer orchestration.** `Orchestrator` is the only process
   holding session state. No concurrent writes.
2. **WAL on Haft unavailability.** Phase outcomes that can't reach
   Haft are written to a per-commission local WAL; replayed in
   append-order on reconnect.
3. **Haft-wins reconciliation.** On a race between external projection state
   and Haft WorkCommission state, Haft wins. External tracker changes are
   recorded as projection drift/override. Worker cancellation is driven by
   Haft commission state, not by tracker state alone.
4. **Canary gate.** Every harness change must run 24h green on a
   throwaway canary repo with seeded gate-activation tickets before
   touching real work.

### Anti-pathology sub-methods (Thai-disaster guardrails)

1. **Harness never operates on itself.** Agent workspace path is the
   downstream repo; `open_sleigh/` and `~/.open-sleigh/` are
   structurally unwritable from any adapter.
2. **Bounded prose.** No unbounded narrative fields in any artifact.
   `rationale` is ≤1000 chars by type.
3. **Bounded operator DSL.** `sleigh.md` compilation fails if the file
   exceeds 300 lines or any prompt exceeds 150 lines.
4. **Isolated observations.** `ObservationsBus` has zero compile-time
   path to `Haft.Client`. Harness telemetry cannot become a Haft
   artifact.

---

## 3. What is the role of the target system

> **Commissioned execution subsystem for Haft, enforcing an FPF-compliant
> governance lifecycle around every WorkCommission.**

The system is NOT a coding agent. Codex / Claude / whatever is the
coding agent. The system is NOT a peer product beside Haft. The system is NOT
a reasoning surface for the operator — that lives upstream in Haft /
`/h-reason`. The system is NOT a CI system — the downstream project's CI runs
the tests. The system is the **harness runtime**: the gated conveyor that
delivers each commissioned unit of work to the agent, supervises its
execution, records provenanced artifacts, and escalates to the human at
declared decision points.

### Sub-roles

| Sub-role | What it does | Analogy |
|---|---|---|
| **Phase conductor** | Routes commissions through Preflight → Frame → Execute → Measure; full lemniscate later | A governed pipeline with interlocked gates |
| **Capability warden** | Enforces scoped toolsets per phase and WorkCommission Scope for mutations | A range officer: right tool, right lane, or cease fire |
| **Provenance ledger** | Pins `config_hash` + `valid_until` + `authoring_role` on every artifact | A court reporter: every statement is tied to when, who, under which rules |
| **Human-gate dispatcher** | Fires human approval on declared transitions only | An escalation bell, not a mandatory review loop |
| **Observation watcher** | Emits anti-Goodhart signals (gate_bypass_rate etc.) as telemetry | A smoke detector: never a target, always a watch |

### The "narrow autonomy" principle

Inside a phase, the agent has autonomy within its scoped toolset and
WorkCommission Scope.
The walls are at phase boundaries, not inside phases.

| Agent autonomous (system does not supervise) | Structural enforcement (system refuses silently-illegal states) | Human-escalated (HumanGate fires) |
|---|---|---|
| Editing source files inside WorkCommission Scope | Calling a tool not in the phase's scoped list | PR targeting `main` |
| Running tests, reading logs | Writing outside workspace or outside WorkCommission Scope | External publication or terminal projection update |
| Iterating on its own output within `max_turns` | Emitting a `PhaseOutcome` without provenance fields | Creating/approving a WorkCommission or AutonomyEnvelope |
| Deciding how to decompose a refactor | Skipping a phase in the graph | (MVP-2) Applying a Pareto selection |

---

## 4. The target system in its supersystem

```
SUPERSYSTEM: "AI-assisted software development for a small engineering org"
│
├── Human principal (the operator — Ivan)
│     Role: Frames problems, chooses decisions, commissions work,
│            approves HumanGates/envelopes, reads observations, edits sleigh.md
│     Relationship: Open-Sleigh receives authorized WorkCommissions FROM,
│                   escalates gates/reviews TO
│
├── Haft (parent product, artifact graph, operator surfaces)
│     Role: Authoritative object store for ProblemCards, decisions,
│           WorkCommissions, RuntimeRuns, evidence, and external projection intent
│     Relationship: Open-Sleigh is a commissioned-execution subsystem. In the
│                   current implementation seam it is also an MCP/JSON-RPC client
│                   of `haft serve`
│
├── External tracker (Linear/Jira/GitHub Issues, optional)
│     Role: Coordination carrier for managers/analysts/leads
│     Relationship: Haft publishes ExternalProjections; Open-Sleigh does not
│                   use tracker state as work authority
│
├── AI coding agent (Codex CLI day 1; Claude Code MVP-1.5)
│     Role: Authors the actual code change within a phase
│     Relationship: Open-Sleigh spawns via AdapterSession; enforces scoped tools
│
├── Downstream repo (the project the agent modifies)
│     Role: Where the work happens; has its own CI, its own history
│     Relationship: Open-Sleigh scopes the agent's workspace to this repo ONLY
│
├── CI system (downstream project's own — GitHub Actions, etc.)
│     Role: Runs tests, reports pass/fail
│     Relationship: Open-Sleigh reads CI verdict refs as evidence carriers; does not run CI itself
│
├── LLM-judge (for semantic gates — typically the same adapter with a judge prompt)
│     Role: Evaluates object-of-talk specificity, LADE splits, evidence semantics
│     Relationship: Open-Sleigh invokes via JudgeClient; calibrated via golden sets
│
└── >>> Open-Sleigh (Haft Harness runtime) <<<
      Role: Harness runtime — phase-gated agent orchestration
      Boundary INSIDE:  phase machine, gate algebra, session state,
                        adapter invocation, WAL, observations,
                        sleigh.md compilation
      Boundary OUTSIDE: problem framing (human + Haft does this),
                        code authoring (agent does this),
                        CI execution (downstream does this),
                        evidence persistence (Haft does this),
                        external projection publication (Haft does this)
```

### What is INSIDE the boundary

- `WorkCommission × Phase` session lifecycle management
- Phase graph + transition decisions (`PhaseMachine`)
- Gate evaluation (structural / semantic / human)
- Adapter invocation with scoped tool enforcement
- Artifact construction with provenance pinning
- WAL for Haft unavailability
- Observation bus for anti-Goodhart signals
- `sleigh.md` parsing, validation, hash-pinning
- Human-gate dispatch and approval collection

### What is OUTSIDE the boundary

- Problem framing (upstream, human + Haft)
- Code authoring (inside Execute phase — agent does it)
- Test execution (downstream CI)
- Evidence persistence (Haft SQLite — we write artifacts, we don't store them)
- WorkCommission lifecycle authorship (Haft owns it; Open-Sleigh leases/runs)
- External projection authorship (Haft projection engine owns it; Open-Sleigh reports facts)
- Dashboard UI (MVP-1 is terminal-only; LiveView is MVP-2)
- SSH worker distribution (MVP-2)
- Any form of self-reflection or governance about the harness itself

---

## 5. Project roles

### External project roles (exploit / affected by the system)

| Project Role | Agent examples | Interest | Method (how they interact) |
|---|---|---|---|
| **Human principal** | Ivan, any solo engineer / tiny team lead using AI agents | Work gets done on downstream repos with structural quality floors; not drown in process | Frames problems, chooses decisions, commissions work, approves HumanGates, approves AutonomyEnvelopes |
| **Downstream project owner** | Same person or teammate | Their repo gets correct work; no rogue edits; clear trail | Reviews PRs the engine produced; sees Haft/local/external projection state change predictably |
| **AI agent (Codex / Claude)** | LLM behind an adapter | Clear inputs (scoped prompt + scoped tools + declared acceptance) | Receives per-phase prompts; calls tools via dispatcher; emits structured PhaseOutcome |
| **Attacker** | Supply-chain or prompt-injection adversary | Escalate beyond scoped tools, write outside workspace, exfiltrate tokens (INVERSE — designed against) | Prompt injection in ticket body, crafted adapter replies — we scope aggressively |

### Internal project roles (create / operate the system)

| Project Role | Agent examples | Interest | Method |
|---|---|---|---|
| **Engine author** | Ivan, any future contributor | Correct, small, maintainable Elixir; no meta-creep | Adds L1-L6 code; extends adapters / gates only via declared behaviours |
| **Operator** | Same person at runtime | Configure engine per downstream project | Writes `sleigh.md`; runs `mix open_sleigh.start`; monitors observations |
| **Reviewer (5.4 Pro or human)** | Extended-thinking review pass | Find spec-vs-code drift, illegal-state gaps, architectural violations | Reads `specs/`, runs `AGENTS.md` review prompt over repomix |

---

## 6. Product integrity principle

> "Open-Sleigh is Haft's harness runtime. It is not a coding agent, not a
> peer product, not a reasoner, not a ticket tracker, not a CI system, not a
> knowledge base. Every feature that doesn't serve phase-gated AI-agent
> orchestration is either owned by Haft or it doesn't exist."

The system's role is its immune system. Concretely:

- If a feature proposes that Open-Sleigh **write about itself** — it doesn't belong. That's the thai-disaster attractor.
- If a feature proposes that Open-Sleigh **author a problem frame** — it doesn't belong. Framing is upstream in Haft.
- If a feature proposes that Open-Sleigh **author work permission** — it doesn't belong. WorkCommission authority is upstream in Haft.
- If a feature proposes that Open-Sleigh **run tests directly** — it doesn't belong. Tests run in the downstream project's CI.
- If a feature proposes that Open-Sleigh **reason about solutions** — it doesn't belong. That's what the agent inside Execute does.
- If a feature proposes that Open-Sleigh **grow `sleigh.md`** beyond its size budget — it doesn't belong. Compile fails.

---

## Reading order for the full spec set

1. **This document** (SYSTEM_CONTEXT)
2. [TERM_MAP](TERM_MAP.md) — canonical vocabulary
3. [SCOPE_FREEZE](SCOPE_FREEZE.md) — MVP tiers
4. [PHASE_ONTOLOGY](PHASE_ONTOLOGY.md) — phase graph + gate kinds + verdicts
5. [TARGET_SYSTEM_MODEL](TARGET_SYSTEM_MODEL.md) — engine entities
6. [ILLEGAL_STATES](ILLEGAL_STATES.md) — structural impossibilities
7. [OPEN_QUESTIONS](OPEN_QUESTIONS.md) — unresolved
8. [../enabling-system/FUNCTIONAL_ARCHITECTURE](../enabling-system/FUNCTIONAL_ARCHITECTURE.md) — 6-layer hierarchy
9. [../enabling-system/STACK_DECISION](../enabling-system/STACK_DECISION.md) — Elixir/OTP rationale
