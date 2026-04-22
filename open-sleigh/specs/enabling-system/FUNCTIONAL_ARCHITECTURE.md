---
title: "Open-Sleigh Functional Architecture"
version: v0.2 (post-5.4-Pro first review; pending confirmation pass)
date: 2026-04-22
status: revised per 5.4 Pro CRITICAL-2/3/4 + HIGH-1/4/5/6
valid_until: 2026-05-20
---

# Open-Sleigh — Functional Architecture

> **FPF note.** This document is a `Description` of the enabling system that
> produces the Open-Sleigh target system. It describes the layered code
> structure, not the engine at runtime. Architecture compliance is a
> design-time claim; passing layer-boundary tests is run-time evidence.
> Don't confuse them.

## Reading order

1. [SYSTEM_CONTEXT.md](../target-system/SYSTEM_CONTEXT.md) — why Open-Sleigh exists (target system)
2. [TERM_MAP.md](../target-system/TERM_MAP.md) — canonical vocabulary (one meaning per term)
3. [TARGET_SYSTEM_MODEL.md](../target-system/TARGET_SYSTEM_MODEL.md) — domain entities (what this architecture enables)
4. [ILLEGAL_STATES.md](../target-system/ILLEGAL_STATES.md) — the states the type system forbids
5. **This document** — 6 layers, dependency rules, inexpressibility per layer
6. [STACK_DECISION.md](STACK_DECISION.md) — Elixir/OTP rationale

---

## Thai-disaster architectural guardrails

These are cross-cutting rules that predate and override any layer-specific
design. They are a direct response to the pathology catalogued in
`~/Repos/hustles/thai/.context/audit-general-analysis.md` (70K lines of
tooling + 230K lines of governance docs against ~1500 lines of actual AI
product logic; 99% AI-agent commits in self-service governance loops; 105KB
CLAUDE.md as recursive ceremony generator; three serial app rewrites with
zero production deployments).

| Guardrail | Mechanism (v0.5 hardened) | Rating | Anti-pattern prevented |
|---|---|---|---|
| Open-Sleigh never operates on its own source | L4 `PathGuard.canonical/1` algorithm: `Path.expand` → recursive `File.read_link` (max depth 8) → inode compare against `open_sleigh/` and `~/.open-sleigh/` trees → `.git/config remote.origin.url` check (rejects `git clone` of Open-Sleigh into workspace). Five enumerated rejection reasons (`:path_outside_workspace` / `:path_symlink_escape` / `:path_hardlink_escape` / `:path_symlink_loop` / `:workspace_is_self`). See `ILLEGAL_STATES.md` CL5–CL12. | **Strong** | Recursive self-improvement spiral |
| `PhaseOutcome` prose fields length-bounded | L1 constructor validates `rationale ≤ 1000 chars`; all other narrative fields bounded (HumanGateApproval.reason ≤ 500; Evidence.kind is an atom, not a string). See PR4. | **Moderate** (sharding across multiple artifacts remains possible; observable via `judge_false_pos_rate` on `lade_quadrants_split_ok`) | FOER / longform ceremony generation |
| `sleigh.md` has a declared size budget | L6 `Sleigh.SizeBudget.check/1`: rejects > 300 lines (CF1) AND > 50 KB bytes (CF10, v0.5 hardening). Transitive include-directive following in CF9. | **Strong** | CLAUDE.md-pathology (105KB governance sprawl) |
| No meta-telemetry enters the Haft artifact graph | L5 `ObservationsBus` has zero transitive compile-time path to `Haft.Client` (OB1, `mix xref`) AND no module may simultaneously import both via `import`/`alias`/`use` (OB5, v0.5 P5 proxy-module rule). Plus type-narrowing on `emit/3` signature (OB3). | **Strong** | Factory-of-factories: harness documenting itself in its own evidence plane |
| No `v2/` or `legacy/` directories | Repo layout policy enforced by `mix compile.check` Taskfile step + Credo rule `OpenSleigh.Credo.NoLegacyTree` | **Strong** | Serial-rewrite graveyard |
| Bounded enabling-system surface | Only `lib/`, `test/`, `config/`, `mix.exs`, `sleigh.md.example`, `ADAPTER_PARITY.md`, `specs/`, `golden/` shipped; anything else requires an explicit ADR | **Moderate** (policy, not mechanism; ADR compliance relies on PR review) | 28% of codebase being tooling |
| Upstream framing lock (v0.5) | Frame phase is **verifier-only**; `problem_card_ref_present` at Frame entry + `haft_problem` excluded from Frame tools + `AuthoringRole` sum separates `:frame_verifier` from `:human`. See UP1–UP3. | **Strong** | Agent drifting into self-framing; recursive problem-authoring loops |

**Rating method.** "Strong" = primary and secondary bypass classes closed by
the mechanism; residual bypass requires threat-model expansion out of scope.
"Moderate" = primary closed, secondary open but observable.
See `ILLEGAL_STATES.md §Guardrail strength ratings` for the complete honest
assessment including acknowledged residual bypass paths.

---

## Layer hierarchy at a glance

```
L6  Operator DSL          — sleigh.md → compiled SleighConfig.t()
L5  OTP orchestration     — GenServers, Tasks, Supervisors (single-writer)
L4  Typed adapter boundary — the only effectful layer
L3  Phase machine          — pure graph semantics
L2  Gate algebra           — pure gate evaluation over L1
L1  Core types             — algebraic data, immutable, pure
```

**Dependency rule:** each layer depends only on the layer(s) directly
below it. L1 depends on nothing. L6 can traverse down to L1 through
intermediaries but never short-circuits (no L6 → L2 skip).

---

## LAYER 1 — Core types

**Purpose.** Pure algebraic data and domain predicates. Immutable, composable,
testable without a runtime. This layer is the ontology.

### Concepts

- `SessionId` (opaque)
- `Ticket` (struct: id, source, problem_card_ref, metadata)
- `Phase` (closed sum with **all MVP-1 and MVP-2 atoms pre-declared from day 1**):
  ```elixir
  @type t ::
          # MVP-1 alphabet
          :frame | :execute | :measure | :terminal
          # MVP-2 additions (pre-declared per Q-OS-2 resolution, v0.5)
          | :characterize_situation | :measure_situation | :problematize
          | :select_spec | :accept_spec | :generate | :parity_run
          | :select | :commission | :measure_impact
  ```
  `Workflow.mvp1/0` routes only through the MVP-1 atoms; `Workflow.mvp2/0`
  routes through the full alphabet. No `{:m2, atom()}` open-tagged variant.
  `PhaseMachine.next/2` exhaustiveness is total over this closed alphabet.
- `PhaseOutcome` (struct with required provenance fields)
- `Verdict` (sum: `:pass | :fail | :partial`)
- `Evidence` (struct: `kind`, `ref`, `hash`, `cl`)
- `GateResult` (sum: structural / semantic / human)
- `GateKind` (sum: `:structural | :semantic | :human`)
- `HumanGateApproval` (struct with approver, at, reason?, config_hash)
- `ConfigHash` (opaque binary)
- `ProblemCardRef` (opaque pointer to a Haft artifact)
- `Workflow` (immutable graph data)
- `AuthoringRole` (sum: `:frame_verifier | :executor | :measurer | :judge | :human`). Renamed from `:framer` in v0.5: the Frame-phase role is verification of upstream human-authored framing, not authorship.

### Functions (examples, all pure)

```elixir
# Single constructor per Q-OS-3 resolution (v0.5). No new_external/3.
# Gate-config consistency: if phase_config declares `commission_approved`
# in its human gates, the passed gate_results MUST contain a matching
# {:human, HumanGateApproval.t()} element, else construction fails.
@spec PhaseOutcome.new(payload :: map(), provenance :: %{
        required(:config_hash)     => ConfigHash.t(),
        required(:valid_until)     => DateTime.t(),
        required(:authoring_role)  => AuthoringRole.t(),
        required(:self_id)         => SessionScopedArtifactId.t(),
        required(:gate_results)    => [GateResult.t()],
        required(:evidence)        => [Evidence.t()],
        required(:phase_config)    => PhaseConfig.t()
      }) :: {:ok, PhaseOutcome.t()} | {:error, reason :: atom()}
# Reasons the constructor can return:
#   :missing_config_hash | :missing_valid_until | :missing_authoring_role
#   | :rationale_too_long
#   | :evidence_self_reference          — any Evidence.ref == self_id
#   | :evidence_required_on_measure     — phase == :measure AND evidence == []
#   | :human_gate_required_by_phase_config_but_missing
#   | :approver_not_authorised_for_config_hash

@spec Evidence.new(kind :: atom(), ref :: String.t(),
                   hash :: String.t(), cl :: 0..3) ::
        {:ok, Evidence.t()} | {:error, :invalid_cl | :empty_ref}
# Note: self-reference check (PR5) lives in PhaseOutcome.new/2 where
# self_id is known, NOT here. Evidence in isolation cannot know what it
# is evidence *for*. This is the v0.5 fix to the CRITICAL-2 finding.

@spec Workflow.mvp1() :: Workflow.t()
@spec Workflow.mvp2() :: Workflow.t()
```

### Inexpressible in L1 (with precise enforcement labels per v0.5)

Labels (see `specs/target-system/ILLEGAL_STATES.md` for the full taxonomy):
**type-level** (pattern-match exhaustive, function-clause enforced, sum-type
closed) / **constructor-level** (`@enforce_keys` + constructor-only module
API; bypass via direct struct literal `%Foo{...}` is a Credo-rule violation,
not a compile error) / **runtime-guard** (checked in the function body,
returns `{:error, _}`) / **CI-or-module-graph-check** (enforced by
`mix xref`, Credo, or CI rule, not by the language itself).

- `Phase` not in the declared workflow's alphabet — **type-level**; closed sum pins the alphabet per Q-OS-2 resolution.
- `Verdict` as a free string — **type-level**; only `:pass | :fail | :partial` admitted.
- `PhaseOutcome` without `config_hash` / `valid_until` / `authoring_role` — **constructor-level**; `PhaseOutcome.new/2` requires them in the provenance map. Direct struct literal `%PhaseOutcome{...}` bypasses this but is forbidden by Credo `OpenSleigh.Credo.NoDirectStructLiteral` and caught at CI.
- `Evidence` with `ref == self_id` of the authoring PhaseOutcome — **constructor-level at `PhaseOutcome.new/2`** (not at `Evidence.new/5`; the evidence in isolation doesn't know its authoring artifact's id). This is the v0.5 fix to 5.4 Pro's CRITICAL-2.
- `PhaseOutcome` with `phase == :measure` and empty `evidence` — **constructor-level**.
- `PhaseOutcome` whose gate_results don't match the `phase_config`'s declared gates (e.g. `commission_approved` declared in phase_config but missing from gate_results) — **constructor-level**; this is the gate-config consistency rule that replaces v0.4's proposed `new_external/3` (Q-OS-3 resolution).
- Unbounded prose in `PhaseOutcome` — **constructor-level**; `rationale :: String.t() | nil` validated ≤ 1000 chars.

### Canonical normal form

One struct per concept. One constructor per struct. Pattern-match on the
struct, never on free-form maps.

### Depends on

Nothing.

---

## LAYER 2 — Gate algebra

**Purpose.** Pure evaluation of gate conditions over L1 artifacts. Gate
registration is compile-time; invocation is pure modulo L4 judge effects
(L2 defines the contract; L4 invokes the judge).

### Concepts

- `StructuralGate` (behaviour + module per gate name)
- `SemanticGate` (behaviour + judge-backed impls)
- `GateChain` (ordered list with kind-aware combine)
- `JudgeCalibration` (golden-set CL-per-gate, compile-time data)

### Functions

```elixir
@spec StructuralGate.apply(gate_name :: atom(), artifact :: PhaseOutcome.t()) ::
        {:ok, :pass} | {:error, reason :: atom()}

@spec SemanticGate.apply(gate_name :: atom(), artifact :: PhaseOutcome.t(),
                         judge_ctx :: JudgeContext.t()) ::
        {:ok, %{verdict: Verdict.t(), cl: 0..3, rationale: String.t()}}
        | {:error, reason :: atom()}

@spec GateChain.evaluate(chain :: [GateBinding.t()], artifact :: PhaseOutcome.t()) ::
        {:ok, [GateResult.t()]} | {:error, first_failure :: GateResult.t()}

@spec GateResult.combine([GateResult.t()]) ::
        {:advance | :block | :await_human, reasons :: [term()]}
```

### Inexpressible in L2

- Merging structural and semantic results as if the same kind — `combine` pattern-matches on `GateKind`
- `SemanticGate` result without a `cl` — struct-enforced
- Invoking a gate on anything that isn't a `PhaseOutcome` — guard-enforced
- Gate name that doesn't resolve to a registered module — `{:error, :unknown_gate}` at L2; L6 catches this at compile time

### Canonical normal form

One module per gate (e.g. `OpenSleigh.Gates.Structural.DescribedEntityFieldPresent`).
One `apply/1` per module. Registration via compile-time registry. No inline
anonymous gate closures.

### Depends on

L1.

---

## LAYER 3 — Phase machine

**Purpose.** Pure graph semantics over phases. Given a workflow state and a
phase outcome, compute the next decision. Total function over legal
inputs; illegal inputs return typed errors.

### Concepts

- `PhaseGraph` (nodes + legal transitions, immutable data)
- `PhaseMachine` (pure transition function)
- `WorkflowState` (current phase + accumulated outcomes + pending human gates)
- `NextDecision` (sum: `{:advance, next_phase} | {:block, [GateResult.t()]} | {:await_human, HumanGate.t()} | {:terminal, Verdict.t()}`)

### Functions

```elixir
@spec PhaseMachine.next(state :: WorkflowState.t(), outcome :: PhaseOutcome.t()) ::
        NextDecision.t()

@spec PhaseGraph.legal_transitions(graph :: PhaseGraph.t(), from :: Phase.t()) ::
        [Phase.t()]

@spec WorkflowState.apply_outcome(state :: WorkflowState.t(),
                                  outcome :: PhaseOutcome.t()) ::
        WorkflowState.t()
```

### Inexpressible in L3

- Advancing from Frame to Measure without Execute — no clause in `next/2` produces that transition
- `WorkflowState` with two simultaneously active phases — `current :: Phase.t()` is a singleton
- Advancing while a `HumanGate` is pending — `:await_human` is sticky until resolved
- Re-entering a terminal verdict — terminal is absorbing in the graph

### Canonical normal form

One `PhaseGraph.t()` per workflow (MVP-1, MVP-2), defined as a compile-time
module attribute. No dynamic graph mutation at runtime.

### Depends on

L1, L2.

---

## LAYER 4 — Typed adapter boundary

**Purpose.** The only effectful layer. All I/O flows through typed adapter
boundaries. Every call returns `{:ok, _} | {:error, EffectError.t()}`; no
throws for expected failure modes.

**L4/L5 ownership seam (v0.6.1 clarification).** L4 modules expose
**stateless typed APIs** (behaviour contracts, codecs, pure
request/response shapes). Any OTP process, `Port`, or long-lived
resource that an adapter's effects require (the `haft serve`
subprocess, the Codex `app-server` `Port`, an HTTP connection pool)
is **owned at L5** — typically by a supervisor like `HaftSupervisor`
or by `AgentWorker` itself. L4 functions take a handle to the L5
owner as argument and dispatch through messages. Concretely:
`Haft.Client.write_artifact(session, outcome)` at L4 is a function
that resolves `session`'s L5 `HaftServer` GenServer pid and calls
`GenServer.call(pid, {:write, outcome})`; the GenServer that owns
the subprocess Port lives at L5. This keeps L4 pure-as-function while
allowing effectful behaviour.

### Concepts

- `Agent.Adapter` behaviour (Codex, Claude impls)
- `Tracker.Adapter` behaviour (Linear first, GitHub/Jira later)
- `Haft.Client` (MCP JSON-RPC over stdio)
- `JudgeClient` (for semantic gates)
- `AdapterSession` (per-session effect context: `session_id`, `config_hash`, `scoped_tools`, `workspace_path`)
- `EffectError` (enumerated sum type over every failure mode)

### Functions

```elixir
@spec Agent.Adapter.send_turn(session :: AdapterSession.t(), prompt :: String.t()) ::
        {:ok, AgentReply.t()} | {:error, EffectError.t()}

@spec Agent.Adapter.dispatch_tool(session :: AdapterSession.t(),
                                  tool :: adapter_tool :: atom(),
                                  args :: map()) ::
        {:ok, ToolResult.t()}
        | {:error, :tool_forbidden_by_phase_scope
                 | :tool_unknown_to_adapter
                 | :arg_invalid
                 | EffectError.t()}
# Hybrid scoping per Q-OS-4 resolution (v0.5):
#   Compile-time (type-level): `tool` must be one of the adapter's
#     declared tool atoms. Each adapter module exposes
#     `@tool_registry [:read, :write, :bash, :haft_query, ...]` and
#     dispatch_tool has function clauses only for those atoms. A tool
#     unknown to the adapter fails function-clause match.
#   Runtime (guard): per-phase scoping is a MapSet.member? check on
#     AdapterSession.scoped_tools. Violations return
#     :tool_forbidden_by_phase_scope. This lives at runtime because
#     sleigh.md hot-reload can change phase scopes without recompile.

@spec Haft.Client.write_artifact(session :: AdapterSession.t(),
                                 outcome :: PhaseOutcome.t()) ::
        {:ok, HaftArtifactId.t()}
        | {:error, :unavailable | :wal_append_failed}

@spec Tracker.Adapter.transition(session :: AdapterSession.t(),
                                 ticket_id :: String.t(),
                                 state :: atom()) ::
        {:ok, Transition.t()} | {:error, EffectError.t()}

@spec JudgeClient.evaluate(gate_name :: atom(), artifact :: PhaseOutcome.t()) ::
        {:ok, SemanticGateResult.t()}
        | {:error, :judge_unavailable | :golden_set_regression}
```

### Inexpressible in L4 (hybrid scoping + canonical path resolution)

- **Capability leak — unknown adapter tool:** `dispatch_tool/3` has function clauses only for atoms in the adapter's `@tool_registry`. Unknown atom → function-clause mismatch. **type-level** (closed dispatch set per adapter).
- **Capability leak — tool out of phase scope:** `AdapterSession.scoped_tools :: MapSet.t(atom())` is checked in the function body; out-of-scope returns `:tool_forbidden_by_phase_scope`. **runtime-guard** (live configurable via `sleigh.md` hot-reload per Q-OS-4 resolution).
- **Haft write without attached `config_hash`:** signature requires `session :: AdapterSession.t()`, which carries the hash; no arity-1 variant exists. **type-level** (signature).
- **Adapter writing outside its `workspace_path` — canonical-path resolution** (P5 hardening, v0.5). The allowlist check is NOT a prefix compare on the raw string. It is the following algorithm:
  1. `canonical = Path.expand(path)` — resolve `..` and `~`.
  2. Recursively dereference symlinks: while `{:ok, target} = File.read_link(canonical)`, `canonical = Path.expand(target, Path.dirname(canonical))`. Max depth 8; exceed → `:path_symlink_loop`.
  3. Compare the **realpath** (stat-resolved) against the forbidden prefixes: `open_sleigh/` (the harness source tree), `~/.open-sleigh/` (WAL + config dir), and any path that inodes-equal a file already in those trees (defence against hardlinks).
  4. Additionally forbid **clone-into-workspace** bypass: if `workspace_path` contains a `.git/` directory whose `remote.origin.url` matches the Open-Sleigh repository, session construction fails with `:workspace_is_self`. This prevents the trivial bypass of `git clone open_sleigh /tmp/work && cd /tmp/work && $EDITOR`.
  5. Violations are enumerated: `:path_outside_workspace` | `:path_symlink_escape` | `:path_hardlink_escape` | `:path_symlink_loop` | `:workspace_is_self`. **runtime-guard + CI-check**: the algorithm is runtime; CI asserts no module calls filesystem APIs outside `OpenSleigh.Adapter.PathGuard`.
- **`EffectError` as a free string:** sum type enumeration; adding a new error requires extending the sum explicitly. **type-level** (closed sum).

### Canonical normal form

One module per adapter impl; all effectful calls return
`{:ok, _} | {:error, EffectError.t()}`; no `raise` for expected failures
(raises only for programmer error). Every adapter implements its behaviour
exactly; no ad-hoc extension points.

### Depends on

L1, L2, L3.

---

## LAYER 5 — OTP orchestration

**Purpose.** Supervision, single-writer state discipline, WAL, observation
bus. This layer owns concurrency and crash recovery.

### Concepts

- `Orchestrator` (GenServer — single writer for session state)
- `AgentWorker` (`Task` under `Task.Supervisor`, owns one session)
- `WorkflowStore` (GenServer — hot-reloads compiled `SleighConfig`)
- `HaftSupervisor` (owns `haft serve` + WAL)
- `TrackerPoller` (periodic tracker sync)
- `HumanGateListener` (listens for `/approve` signals)
- `ObservationsBus` (ETS-backed metrics sink — **never** a Haft artifact)

### Functions (message handlers expressed as state transformations)

```elixir
@spec Orchestrator.handle_call({:claim, Ticket.t()}, from :: term(),
                               state :: Orchestrator.State.t()) ::
        {:reply, {:ok, Session.t()} | :already_owned, Orchestrator.State.t()}

@spec AgentWorker.run(session :: Session.t()) :: PhaseOutcome.t()

@spec HaftSupervisor.with_wal(fun :: (-> any())) ::
        {:ok, any()} | {:error, term()}

@spec ObservationsBus.emit(metric :: atom(), value :: number(), tags :: map()) ::
        :ok
```

### Inexpressible in L5

- Two writers mutating the same session's state — `Orchestrator` is the only process holding session state; `AgentWorker` reports via message passing
- `AgentWorker` without a claimed session — `Task.start_link` signature requires `Session.t()`
- Observation data entering the Haft artifact graph — `ObservationsBus` has no `Haft.Client` dependency; the module graph prevents it at compile time (the Thai-disaster guardrail, mechanized)
- `AgentWorker` writing into `open_sleigh/` or `~/.open-sleigh/` — workspace path is session-scoped to the tracker's repo-ref

### Canonical normal form

One GenServer per kind of singleton state. One Task per (Ticket × Phase)
session. One Supervisor per failure domain. No "manager" god-processes.

### Depends on

L1, L2, L3, L4.

---

## LAYER 6 — Operator DSL

**Purpose.** `sleigh.md` is the top-level code. Operator edits it; compiler
validates and produces an immutable `SleighConfig.t()`. This is the
ONLY audience-facing L_top for daily operation. Elixir-level extension
(new gate, new adapter) is an L5 engineering-discipline surface, not L6.

### Concepts

- `SleighConfig` (compiled, immutable)
- `PhaseConfig` (gate chain + tool list + prompt template ref per phase)
- `GateBinding` (gate name resolved to L2 module ref)
- `AdapterSpec` (adapter kind, version pin, tool registry)
- `PromptTemplate` (validated against `PhaseInput.t()` schema)
- `Sleigh.Compiler` (pure transformer)

### Functions

```elixir
@spec Sleigh.Compiler.compile(source_md :: String.t()) ::
        {:ok, SleighConfig.t()} | {:error, [CompilerError.t()]}

@spec SleighConfig.hash_for_phase(config :: SleighConfig.t(),
                                  phase :: Phase.t()) :: ConfigHash.t()

@spec Sleigh.SizeBudget.check(source_md :: String.t()) ::
        :ok | {:error, :over_budget}
```

### Inexpressible in L6

- Gate name in `sleigh.md` that doesn't resolve to an L2 module — compiler fails
- Tool name not in the adapter's tool registry — compiler fails
- Phase name not in the phase graph — compiler fails
- Prompt template referencing an undefined variable — compiler fails
- `sleigh.md` exceeding its size budget — `Sleigh.SizeBudget.check` fails before compilation; the CLAUDE.md-pathology prevention mechanised

### Canonical normal form

One `sleigh.md` per engine instance. One `SleighConfig.t()` per hot-reload
generation. Config is immutable post-compile. New generation → new struct
→ next session reads new struct; in-flight sessions keep their frozen
`config_hash` per `../target-system/SLEIGH_CONFIG.md §2`.

### Depends on

L1, L2, L3, L4, L5.

---

## Compilation chain — end-to-end

Operator edits `sleigh.md`, saves. Follow the data:

```
sleigh.md (L6 source text)
  │
  ▼  WorkflowStore (L5) detects change, calls Sleigh.Compiler.compile/1
  │
  │  Sleigh.Compiler (L6 pure) — parses YAML + Markdown, validates
  │  against L2 gate registry, L4 adapter tool registry, L3 phase graph
  │
  ▼
SleighConfig.t()  — immutable, validated, with per-phase ConfigHash baked in
  │
  ▼  Orchestrator (L5) polls tracker, sees new Ticket T
  │
  │  Orchestrator.handle_call({:claim, T}, ...) — pure state transition,
  │  returns Session.t()
  │
  ▼
Session.t() :: %{session_id, ticket_id, phase, config_hash,
                 scoped_tools, workspace_path}   (L1 value)
  │
  ▼  Task.Supervisor starts AgentWorker with Session.t()
  │
  │  AgentWorker.run(session) — loop:
  │    Agent.Adapter.send_turn(session, prompt)         (L4 effect, typed errors)
  │    → AgentReply.t() with tool_calls                  (L1)
  │    Agent.Adapter.dispatch_tool(session, tool, args)  (L4, phase-scoped)
  │    → ToolResult.t()                                  (L1)
  │    … agent iterates …
  │    → returns PhaseOutcome.t() via PhaseOutcome.new/3 (L1 constructor
  │      validates provenance; cannot construct a no-provenance outcome)
  │
  ▼
PhaseOutcome.t()
  │
  ▼  AgentWorker hands outcome to gate chain
  │
  │  GateChain.evaluate(phase_config.gates, outcome)    (L2 pure + L4
  │                                                      judge effect for
  │                                                      semantic gates)
  │  → [GateResult.t()]                                  (L1)
  │  GateResult.combine([...])                           (L2 pure)
  │  → :advance | :block | :await_human
  │
  ▼
PhaseMachine.next(workflow_state, outcome)              (L3 pure)
  → NextDecision.t()
  │
  ▼  AgentWorker messages Orchestrator with NextDecision
  │
  │  Orchestrator updates session state (single writer) (L5)
  │  → writes PhaseOutcome via Haft.Client.write_artifact(session, outcome)
  │    (L4; session carries config_hash; HaftSupervisor handles WAL fallback
  │    on :haft_unavailable)
  │
  ▼
Haft artifact persisted; next AgentWorker spawned for next phase,
or ticket terminates.
```

Every arrow is a pure `State → State` transformation except the marked L4
effect boundaries. Errors at every step are typed `EffectError.t()` sum
values; nothing throws for an expected failure mode.

---

## Layer-boundary enforcement — design-time AND run-time

Per FPF: a type-level invariant is a design-time claim. Run-time tests
provide evidence the claim actually holds. Both are required.

| Layer boundary | Design-time mechanism | Run-time evidence |
|---|---|---|
| L1 construction | Opaque structs via `@enforce_keys` + constructor-only module API | Property tests: no `PhaseOutcome` without provenance fields across 1000 fuzzed inputs |
| L2 gate registry | Compile-time registry built from module attributes | Test that every gate named in `sleigh.md.example` resolves at L6 compile time |
| L3 phase graph | Pure data, module attribute | Property test: `PhaseMachine.next/2` is total over `{WorkflowState, PhaseOutcome}` legal inputs |
| L4 adapter tool registry (compile-time) | Function-clause dispatch on closed `@tool_registry` atom set | Test: unknown atom to `dispatch_tool/3` raises `FunctionClauseError` |
| L4 phase-scoped tool filter (runtime) | `MapSet.member?(session.scoped_tools, tool)` before dispatch | Integration test: in-adapter-but-out-of-phase-scope tool returns `:tool_forbidden_by_phase_scope` |
| L4 adapter path safety — canonical path resolution | `PathGuard` module: `Path.expand` + recursive `File.read_link` + inode compare + `.git` remote-URL check | Tests (P5 bypass matrix): absolute-path write, symlink-to-forbidden, hardlink-to-forbidden, symlink-loop, `git clone` of Open-Sleigh into workspace |
| L5 single-writer discipline | Only `Orchestrator` holds session state | Stress test: N concurrent `claim/1` calls yield exactly 1 owner + N-1 `:already_owned` |
| L5 observations isolation | Module dependency graph (`mix xref`) | CI check: `ObservationsBus` has zero path to `Haft.Client` in the call graph |
| L6 size budget | `Sleigh.SizeBudget.check/1` fails compilation | Test: a 301-line `sleigh.md` fails with `:over_budget` |

---

## What this architecture explicitly refuses

- **No ORM at L1.** The core does not know Ecto exists. Ecto lives at L4 (if at all for MVP-1; MVP-1 is in-RAM + WAL per §10.2).
- **No GenServer in L1 / L2 / L3.** OTP shapes are L5 only.
- **No protocols for L1 core types.** Protocols invite ad-hoc extension that bypasses the type system. Core types are closed sum types with pattern-matching dispatch.
- **No default parameters.** Overloads, Builder, or explicit config structs only. Mutable defaults are bugs; `nil` defaults are noise.
- **No inheritance of any kind** (not even via `defdelegate` chaining). Composition via function pipes.
- **No global mutable state.** ETS for `ObservationsBus` and adapter registries only — both write-scoped to their owning process, read-open.

---

## Open architectural questions (none — all resolved in v0.5)

All three questions previously listed here were closed in v0.5 after the
5.4 Pro extended-thinking review. See `specs/target-system/OPEN_QUESTIONS.md`
for the full rationale:

- **Q-OS-2 resolved:** `Phase.t()` is a closed sum with all MVP-1 and MVP-2 atoms pre-declared from day 1. See L1 Concepts section.
- **Q-OS-3 resolved:** single `PhaseOutcome.new/2` constructor with gate-config consistency check. No `new_external/3`. See L1 Functions section.
- **Q-OS-4 resolved:** hybrid `AllowedTool` — compile-time adapter tool registry + runtime per-phase `MapSet` scope. See L4 Inexpressible section.

Open questions that remain (tracked in `OPEN_QUESTIONS.md`, non-blocking for
`mix new`) are operational or process concerns — adapter-priority
sign-off (Q-OS-1), golden-set labelling handoff (Q-OS-5), OSS timing
(Q-OS-6), downstream-repo CI coupling (Q-OS-7).
