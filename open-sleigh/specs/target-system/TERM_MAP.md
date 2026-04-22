---
title: "2. Term Map"
description: Canonical domain vocabulary. One meaning per term. No drift.
reading_order: 2
---

# Open-Sleigh: Term Map

> **Rule.** If a term is used in code, specs, or prompts, it must appear
> here. If the same word means two things, it must be qualified (e.g.
> "phase" could be `Phase` the sum type OR a generic English word; when
> we mean the sum type, we write `Phase.t()` or "phase (the L1 type)").

## Core lifecycle vocabulary

| Term | Canonical meaning | NOT to be confused with |
|---|---|---|
| **Ticket** | A unit of work in the tracker (Linear issue). Immutable identifier; mutable state owned by tracker. | Task, issue-as-local-db-row, problem (which is upstream in Haft) |
| **Session** | One `(Ticket × Phase × ConfigHash × AdapterSession)` unit of work owned by one `AgentWorker`. Has a `SessionId`. | AdapterSession (which is the L4 effect context inside a Session) |
| **Phase** | One of: `:frame | :execute | :measure` (MVP-1) or the MVP-2 extensions. A value of `Phase.t()` sum type. | A vague "stage of work"; unit test "phase"; git rebase "phase" |
| **Workflow** | Pure-data graph of legal phase transitions. `Workflow.mvp1()` returns the MVP-1 graph. | Tracker's "workflow" (that's a `WorkflowState` there, not ours); reminder workflow (that's octacore's) |
| **WorkflowState** | Per-ticket runtime state: current phase + accumulated outcomes + pending human gates. | Workflow (the graph) |
| **PhaseOutcome** | Immutable artifact produced when a phase exits. Has provenance: `config_hash`, `valid_until`, `authoring_role`. The primary data type flowing through the system. | Phase (the discriminator), Haft artifact (which is the persisted form) |
| **Verdict** | Sum type: `:pass | :fail | :partial`. Outcome-level judgement. | Gate result (which is richer, with reasons), human decision |
| **Gate** | A pure or effectful function `PhaseOutcome.t() → GateResult.t()`. Three kinds by construction. | Tracker gate, CI gate — both of those are external signals, not our L2 gates |

## Gate vocabulary

| Term | Canonical meaning | NOT to be confused with |
|---|---|---|
| **GateKind** | Sum type: `:structural | :semantic | :human`. Compile-time distinct. | Gate severity, gate priority |
| **StructuralGate** | Pure L2 function checking field presence, type shape, or graph-level invariant. Never calls LLM or human. | Syntactic check, lint rule |
| **SemanticGate** | Effectful L2 contract (L4 invokes via `JudgeClient`). Returns `{verdict, cl, rationale}`. | Structural gate with a judge wrapper |
| **HumanGate** | Triggered, not computed. Blocks a transition pending external `/approve`. Fires on declared transitions only. | Code review, PR approval (which are evidence inputs, not gates themselves) |
| **GateResult** | Sum type covering all three kinds' results. Pattern-match on kind before combining. | Verdict (which is coarser) |
| **JudgeCalibration** | Golden-set evidence for a SemanticGate — FP rate, FN rate, CL, corpus size. | Gate threshold |
| **GoldenSet** | Hand-labelled corpus of ≥20 artifacts per SemanticGate, with ground-truth verdict + rationale. | Test fixture (which is single-purpose), training set (we don't train) |

## Provenance vocabulary

| Term | Canonical meaning | NOT to be confused with |
|---|---|---|
| **ConfigHash** | `sha256(engine_section + tracker_section + adapter_section + haft_section + phases[this_phase] + prompts[this_phase])`. Per-phase scope. Pinned per session. | Git commit hash, Haft artifact hash |
| **AuthoringRole** | Sum: `:frame_verifier | :executor | :measurer | :judge | :human`. Who produced this artifact. Renamed from `:framer` in v0.5 because the Frame-phase role is verification of upstream framing, not authorship of it. | Agent identity (Codex vs Claude — that's adapter identity, not role) |
| **ProblemCardRef** | Opaque pointer to a Haft ProblemCard produced upstream by the human. | ProblemCard (the Haft artifact); Ticket body (which is tracker-side) |
| **valid_until** | ISO-8601 date: when this artifact should be re-evaluated. Required on every PhaseOutcome. | Expiration (which implies deletion); deadline (which implies failure) |
| **Evidence** | Struct: `kind`, `ref`, `hash`, `cl`. `ref ≠ authoring artifact's self_id`. | Proof, test output (those are raw materials; Evidence wraps them with metadata) |
| **cl (Congruence Level)** | 0..3 integer from FPF (CL0=opposed, CL1=related, CL2=similar, CL3=exact). On every Evidence. | Confidence level (LLM-style float); certainty |

## Adapter / effect vocabulary

| Term | Canonical meaning | NOT to be confused with |
|---|---|---|
| **Agent.Adapter** | Elixir behaviour (L4). First impl: Codex. MVP-1.5: Claude. Satisfies Parity Plan. | The LLM itself; the CLI process |
| **Tracker.Adapter** | Elixir behaviour (L4). First impl: Linear. | Ticket tracker service; Linear's API client library |
| **Haft.Client** | L4 MCP JSON-RPC client to `haft serve`. Pooled via `Haft.Supervisor`. | Haft (which is the external MCP server); haft_* tools (which are its endpoints) |
| **JudgeClient** | L4 client for SemanticGate evaluation. Typically uses an agent adapter with a judge prompt. | Agent.Adapter (which is for phase execution) |
| **AdapterSession** | L4 effect context passed to every adapter call: `session_id`, `config_hash`, `scoped_tools`, `workspace_path`. | Session (which is L1 and richer); adapter process PID (which is L5 plumbing) |
| **AllowedTool** | Hybrid scoping (Q-OS-4 v0.5 resolution): (a) compile-time `@tool_registry` atom set per adapter — unknown-to-adapter tool fails at function-clause match; (b) runtime `AdapterSession.scoped_tools :: MapSet.t(atom())` per-phase filter — in-adapter-but-out-of-scope returns `:tool_forbidden_by_phase_scope`. NOT a per-phase generated type. | Macro-generated per-phase type (rejected as Q-OS-4 option — fights `sleigh.md` hot-reload) |
| **EffectError** | Sum type enumerating every expected failure mode across all L4 adapters. Extending requires a source change. | Exception; `Error` class |
| **WAL** | Write-Ahead Log. Per-ticket append-only JSON-L at `~/.open-sleigh/wal/<ticket_id>.jsonl`. | Journal, audit log (those are derived views) |

## Orchestration / OTP vocabulary

| Term | Canonical meaning | NOT to be confused with |
|---|---|---|
| **Orchestrator** | Singleton L5 GenServer. Sole writer of session state. | Supervisor (which is an OTP shape, not the unit of orchestration) |
| **AgentWorker** | L5 Task under `Task.Supervisor`. Owns one Session. | Worker in the generic distributed-systems sense |
| **WorkflowStore** | L5 GenServer that holds the compiled `SleighConfig`. Hot-reloads. | Workflow (the L1 data); tracker's workflow config |
| **HaftSupervisor** | L5 supervisor owning the `haft serve` process and its WAL. | Haft.Client (which is inside it) |
| **TrackerPoller** | L5 periodic task that fetches active tickets from the tracker. | Tracker.Adapter (which is the behaviour it invokes) |
| **HumanGateListener** | L5 process awaiting `/approve` signals from tracker comments or PR reviews. | HumanGate (the L1 value); approver (the human) |
| **ObservationsBus** | L5 ETS-backed metrics sink. **Zero compile-time path to `Haft.Client`.** | Haft artifact graph; telemetry library (Telemetry is a transport, not this bus) |

## Configuration vocabulary

| Term | Canonical meaning | NOT to be confused with |
|---|---|---|
| **sleigh.md** | The single operator-facing configuration file. YAML front matter + Markdown prompt templates. Size-budgeted (≤300 lines total; ≤150 per prompt). | CLAUDE.md (which is a different project's disease); SPEC.md |
| **SleighConfig** | L6 compiled, immutable struct produced by `Sleigh.Compiler.compile/1`. | sleigh.md (which is the source); phase_config (which is a field on it) |
| **PhaseConfig** | Per-phase slice of `SleighConfig`: agent_role, scoped tools, gate chain, prompt template. | Phase (the L1 value) |
| **Sleigh.Compiler** | L6 pure transformer. Validates gate names, tool names, phase names, prompt variables. | Elixir's compiler (`:elixir_compiler`) |
| **external_publication** | Declared `sleigh.md` section: branches + tracker states that require HumanGate. | Public OSS release; any other "external" |

## FPF vocabulary (re-declared here because it appears in gates / prompts)

| Term | Canonical meaning | Source |
|---|---|---|
| **LADE** | Law / Admissibility / Deontics / Work-effect-Evidence — the four quadrants of the Boundary Norm Square. | `semiotics_slideument.md` A.6.B |
| **Contract Unpacking (A.6.C)** | Decomposition of promise-language into: promise content / speech act / commitment / work-effect-evidence. | `FPF-Spec.md` A.6.C |
| **Object of talk** | The specific entity a statement is about. For Open-Sleigh: a file path, module, subsystem — never "the system" or "the code". | FPF A.7 Strict Distinction |
| **Lemniscate** | The ∞-shaped loop: Problem Factory ↔ Solution Factory. Full form is MVP-2. MVP-1 is a single-variant pipeline, NOT a lemniscate. | `development_for_the_developed.md` Slide 12 |
| **Parity Plan** | One-page declaration that comparison is fair: same budget, same scope, same eval protocol across variants. | Slide 37 |
| **Self-evidence** | Artifact whose supporting evidence is the artifact itself. Forbidden. | FPF-Spec A.10 CC-A10.6 |
| **Transformer Mandate** | "External agent decides; system doesn't self-improve; human is the principal." | h-reason skill X-TRANSFORMER |

## Reserved words (NOT to use)

| Banned term | Use instead | Why |
|---|---|---|
| "process" (ambiguous) | `Phase`, `Session`, `Pipeline`, or specific sub-method | Umbrella word per FPF; collapses multiple concepts |
| "the system" (in gates / prompts) | specific module / subsystem / file path | The very pattern `object_of_talk_is_specific` catches |
| "quality" | named indicator (`gate_bypass_rate`, `false_neg_rate`) | Umbrella word; forces anti-Goodhart discipline |
| "validated" | "structurally checked" OR "semantically judged" OR "human-approved" | The three GateKinds are never interchangeable |
| "review" (in harness vocabulary) | "HumanGate" OR "SemanticGate via JudgeClient" | Human review = HumanGate; LLM review = SemanticGate |
| "reasoning" (in harness vocabulary) | agent's internal activity (not a harness concept) | Upstream reasoning is human + Haft; we orchestrate, we don't reason |
| "done" (without qualification) | `Verdict.pass` OR `OperationalPhase = terminal` OR tracker-specific state | Overloaded across tracker state, phase state, verdict |

## Tracker-side terms (consumed, not owned)

| Term | Canonical meaning (in Open-Sleigh context) | Where owned |
|---|---|---|
| **Active state** | Tracker state in the `tracker.active_states` list from `sleigh.md`. Polled and claimed by Orchestrator. | Tracker (Linear) |
| **Terminal state** | Tracker state in `tracker.terminal_states`. Requires HumanGate for transition. | Tracker (Linear) |
| **Approver** | Tracker user whose `/approve` comment releases a HumanGate. Matched by the `approvers` list in `sleigh.md`. | Tracker + Open-Sleigh config |

---

## How to use this map

- If you're writing code: use these exact spellings for types, functions, and variables. Drift becomes bugs.
- If you're writing prompts: use these terms when addressing the agent; use plain English when addressing the human reader of a comment.
- If you're writing a new spec: check every load-bearing noun against this map. If it's not here, add it or use an existing one. Don't invent.
- If you're reviewing: any term used without a canonical binding here is a semiotic drift finding.
