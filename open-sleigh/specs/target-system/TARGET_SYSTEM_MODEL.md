---
title: "5. Target System Model"
description: Engine entities — Session, Ticket, PhaseOutcome, Workflow, Evidence, HumanGateApproval. The L1 structures that carry the domain.
reading_order: 5
---

# Open-Sleigh: Target System Model

> **Reading this document.** Each entity section lists: Definition,
> Attributes (required + optional), Relationships, Key rules. All
> entities are L1 pure data; construction is via module constructor
> that validates; no default values for required fields.

---

## Reading order context

Read [SYSTEM_CONTEXT](SYSTEM_CONTEXT.md) and [TERM_MAP](TERM_MAP.md)
first. The entities here presume those vocabularies.

---

## Core entities

### Ticket

**Definition:** A unit of work claimed from the tracker. Immutable in
Open-Sleigh (tracker owns mutable ticket state); Open-Sleigh holds a
snapshot + reference.

**Attributes:**

- `id :: String.t()` (required) — tracker-native identifier
- `source :: {:linear, workspace :: String.t()} | {:github, repo :: String.t()} | ...` (required) — tracker + scope
- `title :: String.t()` (required)
- `body :: String.t()` (required; may be empty)
- `state :: atom()` (required) — current tracker state (snapshot)
- `problem_card_ref :: ProblemCardRef.t()` (required in MVP-1 per v0.5 Q-OS-1 framing resolution) — link to upstream Haft ProblemCard authored by the human via Haft + `/h-reason`. Not optional: a Ticket without `problem_card_ref` fail-fasts at Frame entry via `problem_card_ref_present` gate. `Orchestrator` rejects a tracker-fetched Ticket missing this field by posting a structured comment and NOT claiming the ticket.
- `target_branch :: String.t() | nil` — hint for HumanGate `external_publication` matching
- `metadata :: map()` — tracker-specific fields (labels, assignees, etc.)
- `fetched_at :: DateTime.t()` (required)

**Relationships:**

- Has zero or one active `Session` (Orchestrator enforces single-owner via the claim protocol)
- Has zero or more `PhaseOutcome`s (via persisted Haft artifacts)

**Key rules:**

- A Ticket without `problem_card_ref` (MVP-1 AND MVP-2, v0.5 hardening) is **not accepted** by the Frame phase — Orchestrator returns `{:error, :no_upstream_frame}`, posts a structured tracker comment asking the human to frame it via Haft + `/h-reason`, and does NOT attempt to author a ProblemCard itself. Open-Sleigh never frames; framing is the upstream human's role.
- A Ticket whose `problem_card_ref` resolves to a Haft artifact with `authoring_source == :open_sleigh_self` is rejected identically — prevents the self-authoring bypass (ILLEGAL_STATES UP3).
- Ticket state snapshot may go stale. `TrackerPoller` refreshes on each poll tick. If tracker state changed during an active Session, see `tracker-wins` reconciliation (`RISKS.md §3`).

---

### Session

**Definition:** One `(Ticket × Phase × ConfigHash × AdapterSession)` unit
of work owned by one `AgentWorker`. Identified by opaque `SessionId`.

**Attributes:**

- `id :: SessionId.t()` (required; opaque binary)
- `ticket :: Ticket.t()` (required; snapshot at claim time)
- `phase :: Phase.t()` (required; mutates via message to Orchestrator, never in-place)
- `config_hash :: ConfigHash.t()` (required; pinned at session start per `SLEIGH_CONFIG.md §2`)
- `scoped_tools :: MapSet.t(atom())` (required; derived from phase_config)
- `workspace_path :: Path.t()` (required; the downstream repo path, NOT the Open-Sleigh repo)
- `claimed_at :: DateTime.t()` (required)
- `adapter_session :: AdapterSession.t()` (required)
- `sub_state :: RunAttemptSubState.t()` (required; per SPEC §5.2 — `:preparing_workspace | :building_prompt | :launching_agent_process | :initializing_session | :streaming_turn | :finishing | :succeeded | :failed | :timed_out | :stalled | :canceled_by_reconciliation`)
- `thread_id :: String.t() | nil` (required for Execute; nil before handshake completes; set once on `thread/start` response)
- `turn_count :: non_neg_integer()` (required; starts at 0; incremented on each `turn_completed`; bounded by `PhaseConfig.max_turns` — see CT3/CT4)
- `last_event_at :: DateTime.t() | nil` (for stall detection — `AGENT_PROTOCOL.md §8`)
- `codex_input_tokens :: non_neg_integer()` (required; accumulated per session; feeds `ObservationsBus`, not Haft — TA1)
- `codex_output_tokens :: non_neg_integer()` (required; same discipline)
- `codex_total_tokens :: non_neg_integer()` (required; same discipline)
- `last_reported_total_tokens :: non_neg_integer()` (required; delta-tracking per `HAFT_CONTRACT.md §4` + `AGENT_PROTOCOL.md §4`)

**Relationships:**

- Belongs to one `Ticket`.
- Owned by exactly one `AgentWorker` (L5 Task).
- Produces zero or more `PhaseOutcome`s during its lifetime.

**Key rules:**

- `workspace_path` MUST be outside `open_sleigh/` and `~/.open-sleigh/` — validated at session construction. This is the Thai-disaster architectural guardrail expressed at L1.
- `scoped_tools` is a `MapSet.t(atom())` not a `list` — set semantics prevent accidental duplicates.
- `config_hash` is pinned at session construction; hot-reloads of `sleigh.md` don't retroactively change an active session's hash.
- Session has no mutable "status" field; all state transitions flow through `Orchestrator.handle_cast/2` → message to `AgentWorker` → new Session value.

---

### PhaseOutcome

**Definition:** Immutable artifact produced when a phase completes.
The **primary data type flowing through the system.** Every field is
type-validated at construction; invalid combinations are unrepresentable.

**Attributes:**

- `session_id :: SessionId.t()` (required)
- `phase :: Phase.t()` (required)
- `verdict :: Verdict.t() | nil` — non-nil only on `:terminal`; nil on intermediate phases
- `work_product :: WorkProduct.t()` (required; phase-specific payload)
- `evidence :: [Evidence.t()]` — may be empty on intermediate phases; Measure requires ≥1
- `gate_results :: [GateResult.t()]` (required; all gates this phase ran)
- `config_hash :: ConfigHash.t()` (required) — pinned from Session
- `valid_until :: DateTime.t()` (required)
- `authoring_role :: AuthoringRole.t()` (required)
- `rationale :: String.t() | nil` — bounded: ≤ 1000 chars
- `produced_at :: DateTime.t()` (required)

**Relationships:**

- Belongs to one `Session`.
- Has many `Evidence` items.
- Has many `GateResult`s.

**Key rules (all enforced by `PhaseOutcome.new/2` constructor):**

- `config_hash`, `valid_until`, `authoring_role` are ALL required. There is no default. This is the provenance-pinning invariant (`SLEIGH_CONFIG.md §2`).
- `rationale` cannot exceed 1000 characters. The narrative-bloat prevention.
- `verdict` may be non-nil only if `phase == :terminal` (MVP-1) or if the phase is explicitly declared as terminal in MVP-2 graph.
- Evidence list on a `:measure` phase must be non-empty and every item must satisfy `ref ≠ self_id`.
- Once constructed, `PhaseOutcome` is immutable. "Update" means producing a new outcome, never mutating.

---

### Evidence

**Definition:** A typed reference to a piece of external proof. Never
contains the proof payload itself; always a reference with provenance.

**Attributes:**

- `kind :: :pr_merge_sha | :ci_run_id | :test_count | :human_comment | :external_measurement | atom()` (required)
- `ref :: String.t()` (required) — the identifier in the external system
- `hash :: String.t() | nil` — cryptographic hash of the referenced artifact if applicable
- `cl :: 0..3` (required) — Congruence Level per FPF
- `authoring_source :: atom()` (required) — e.g. `:ci`, `:git_host`, `:tracker`, `:human`
- `captured_at :: DateTime.t()` (required)

**Key rules:**

- `Evidence.new/5` validates `cl ∈ 0..3` and non-empty `ref`. It does NOT check `ref == self_id` — that check requires knowledge of the authoring artifact and lives at `PhaseOutcome.new/2` (v0.5 fix to 5.4 Pro CRITICAL-2). Evidence in isolation doesn't know what it is evidence *for*.
- `cl` outside `0..3` is a type error.
- `kind` is a sum variant from a declared enumeration; unknown kinds require explicit extension.

---

### Workflow

**Definition:** Immutable graph data describing legal phase transitions.

**Attributes:**

- `id :: :mvp1 | :mvp2 | atom()` (required)
- `phases :: [Phase.t()]` (required; declared alphabet)
- `transitions :: %{Phase.t() => [Phase.t()]}` (required; from-phase → legal to-phases)
- `terminal :: [Phase.t()]` (required; absorbing states)
- `entry_phase :: Phase.t()` (required)

**Key rules:**

- Workflows are constructed at compile time (module attribute). No runtime graph mutation.
- `Workflow.mvp1/0` returns the MVP-1 graph; `Workflow.mvp2/0` returns the MVP-2 graph (separate for clarity).
- Invariant: every phase in `phases` either has an entry in `transitions` or is in `terminal`. A phase with no outgoing edges that isn't terminal is a construction-time error.

---

### GateResult

**Definition:** Sum type over all three gate kinds.

**Variants:**

```elixir
@type t ::
        {:structural, :ok}
      | {:structural, {:error, reason :: atom()}}
      | {:semantic, %{verdict: Verdict.t(), cl: 0..3, rationale: String.t()}}
      | {:semantic, {:error, reason :: atom()}}
      | {:human, HumanGateApproval.t()}
      | {:human, :rejected}
      | {:human, :timeout}
```

**Key rules:**

- Pattern-matching MUST match the kind tag first. Functions that accept a `GateResult` and don't match the kind are type-errors (caught by Dialyzer / pattern warnings).
- No `GateResult` variant collapses two kinds. Never `{:structural_or_semantic, ...}`.

---

### HumanGateApproval

**Definition:** Evidence that a human approved a specific transition.

**Attributes:**

- `approver :: String.t()` (required) — matched against `sleigh.md.approvers` list
- `at :: DateTime.t()` (required)
- `config_hash :: ConfigHash.t()` (required) — the hash active at approval time
- `reason :: String.t() | nil` — ≤ 500 chars
- `signal_source :: :tracker_comment | :github_review | :cli_ack` (required)
- `signal_ref :: String.t()` (required) — the external reference to the approval event

**Key rules:**

- Constructed only by `HumanGateListener` (L5) upon receiving a valid signal.
- `approver` must be in the `approvers` set from the active `SleighConfig`; otherwise constructor fails with `{:error, :approver_not_authorised}`.
- `HumanGateApproval`, when required, flows into `PhaseOutcome.gate_results` as a `{:human, HumanGateApproval.t()}` tuple. Per Q-OS-3 resolution (v0.5), there is **no `PhaseOutcome.new_external/3`** — a single `PhaseOutcome.new/2` validates **gate-config consistency**: if the active `PhaseConfig.gates.human` list declares `commission_approved` (or any other human gate), the constructor requires a matching approved `{:human, HumanGateApproval.t()}` entry in `gate_results`, else it fails with `:human_gate_required_by_phase_config_but_missing`. The invariant lives at the single constructor; no special-case path exists that bypasses it.

---

### AdapterSession

**Definition:** L4 effect context passed to every adapter call. Carries
the session-level metadata that every I/O call needs.

**Attributes:**

- `session_id :: SessionId.t()`
- `config_hash :: ConfigHash.t()`
- `scoped_tools :: MapSet.t(atom())`
- `workspace_path :: Path.t()`
- `adapter_kind :: :codex | :claude | atom()`
- `adapter_version :: String.t()`
- `max_turns :: pos_integer()`
- `max_tokens_per_turn :: pos_integer()`
- `wall_clock_timeout_s :: pos_integer()`

**Key rules:**

- `AdapterSession` is constructed by L5 when spawning an `AgentWorker`, then passed verbatim to every adapter call. No L4 function takes session-level metadata separately.
- `scoped_tools` is the phase's tool set intersected with the adapter's capabilities; compilation of `sleigh.md` at L6 rejects tools the adapter can't bind.

---

### SleighConfig

**Definition:** L6 compiled, immutable configuration struct produced by
`Sleigh.Compiler.compile/1`. Contains all phase/gate/adapter bindings
resolved from `sleigh.md`.

**Attributes:**

- `engine :: EngineConfig.t()` — poll interval, concurrency
- `tracker :: TrackerConfig.t()` — kind, workspace/repo, active_states, terminal_states
- `agent :: AgentConfig.t()` — kind, version_pin, command, limits
- `haft :: HaftConfig.t()` — command, version pin
- `external_publication :: ExternalPublicationConfig.t()` — branch regex, terminal transitions, approvers, timeout
- `phases :: %{Phase.t() => PhaseConfig.t()}`
- `prompts :: %{Phase.t() => String.t()}` — validated templates
- `hashes :: %{Phase.t() => ConfigHash.t()}` — precomputed per-phase hashes
- `source_hash :: binary()` — sha256 of source `sleigh.md` for auditing
- `compiled_at :: DateTime.t()`

**Key rules:**

- Only `Sleigh.Compiler.compile/1` produces a `SleighConfig`. No manual construction.
- Immutable; hot-reload replaces the struct atomically in `WorkflowStore`.
- Size budget check (`Sleigh.SizeBudget.check/1`) runs before compile; failure is `{:error, :over_budget}`.

---

### PhaseConfig

**Definition:** Per-phase slice of `SleighConfig`.

**Attributes:**

- `agent_role :: AuthoringRole.t()` (required)
- `tools :: [atom()]` (required; closed set)
- `gates :: %{structural: [atom()], semantic: [atom()], human: [atom()]}` (required)
- `prompt_template_key :: atom()` (required; reference into `SleighConfig.prompts`)

**Key rules:**

- Every gate name resolves to an L2 module at compile time. Unknown names fail L6 compilation.
- Every tool name is in the bound adapter's registry. Unknown tools fail L6 compilation.
- `agent_role` is a sum variant; free strings fail.

---

## Relationship overview

```
Ticket ──1──> Session (owned by AgentWorker)
                │
                ├──*──> PhaseOutcome
                │          │
                │          ├──*──> Evidence
                │          └──*──> GateResult
                │                      │
                │                      └── HumanGateApproval (when :human)
                │
                └── AdapterSession (L4 effect ctx)

SleighConfig ──1──> PhaseConfig per Phase
                │
                └── prompts, hashes, external_publication

Workflow (immutable graph) ── referenced by PhaseMachine
ProblemCardRef ── points to Haft artifact (upstream)
```

---

## Storage model — what lives where

| Entity | MVP-1 location | MVP-2 location |
|---|---|---|
| `Ticket` (snapshot) | Orchestrator ETS | Same |
| `Session` | Orchestrator GenServer state | Same (plus SQLite optional for crash recovery) |
| `PhaseOutcome` | WAL until written to Haft; then Haft SQLite via MCP | Same |
| `Evidence` | Attached to PhaseOutcome, flows with it | Same |
| `HumanGateApproval` | Inside PhaseOutcome.gate_results | Same |
| `SleighConfig` | WorkflowStore GenServer state | Same |
| `Workflow` | Module attribute (compile-time) | Same |
| Observations (`gate_bypass_rate` etc.) | `ObservationsBus` ETS | Same + export adapter (opt-in) |

Canonical rules:

- **Haft SQLite is the source of truth for persistent evidence.**
- **Tracker is the source of truth for active tickets and their states.**
- **Orchestrator is the sole writer of in-RAM session state.**
- **ObservationsBus never reaches Haft.** Compile-time enforced.
