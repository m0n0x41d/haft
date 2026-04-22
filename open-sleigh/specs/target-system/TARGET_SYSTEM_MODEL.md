---
title: "5. Target System Model"
description: Engine entities — WorkCommission, Session, PhaseOutcome, Workflow, Evidence, HumanGateApproval. The L1 structures that carry the domain.
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

### WorkCommission

**Definition:** A Haft-authored, human-authorized unit of work that
Open-Sleigh may execute only if Preflight admits it. Immutable in Open-Sleigh;
Haft owns mutable commission state and lifecycle.

**Attributes:**

- `id :: String.t()` (required) — Haft commission identifier
- `decision_ref :: DecisionRecordRef.t()` (required)
- `decision_revision_hash :: String.t()` (required) — pinned at queue time
- `problem_card_ref :: ProblemCardRef.t()` (required)
- `implementation_plan_ref :: String.t() | nil` — parent plan when scheduled
  as part of a batch/YOLO graph
- `implementation_plan_revision :: String.t() | nil` — parent plan revision
  pinned in the commission snapshot
- `scope :: Scope.t()` (required) — repo, target branch, base sha, allowed
  paths/actions, affected files/modules
- `scope_hash :: String.t()` (required) — canonical hash of `scope`
- `base_sha :: String.t()` (required) — repository commit admitted at queue time
- `lockset :: [String.t()]` (required) — paths/modules/resources this
  commission may mutate or conflicts with
- `evidence_requirements :: [EvidenceRequirement.t()]` (required)
- `projection_policy :: :local_only | :external_optional | :external_required`
  (required)
- `autonomy_envelope_ref :: String.t() | nil` — required for automatic
  continuation beyond a single human-started commission
- `autonomy_envelope_revision :: String.t() | nil` — approved envelope
  revision pinned in the commission snapshot
- `state :: :draft | :queued | :ready | :preflighting | :running |
  :blocked_stale | :blocked_policy | :blocked_conflict |
  :needs_human_review | :completed | :completed_with_projection_debt |
  :failed | :cancelled | :expired`
  (required; snapshot returned by Haft)
- `valid_until :: DateTime.t()` (required)
- `fetched_at :: DateTime.t()` (required)

**Relationships:**

- Belongs to one DecisionRecord.
- Points to one ProblemCard.
- May belong to one ImplementationPlan.
- Has zero or more RuntimeRuns.
- Has zero or more ExternalProjections owned by Haft.

**Key rules:**

- Open-Sleigh never creates or approves WorkCommissions. It may ask Haft for
  runnable commissions and lease one for Preflight.
- A WorkCommission whose linked DecisionRecord is stale, superseded,
  deprecated, hash-mismatched, or expired cannot enter Execute. It becomes
  `:blocked_stale` or `:needs_human_review`.
- A WorkCommission may be `:queued` for an arbitrary time. Queueing is not
  execution permission; Preflight re-checks immediately before Execute.
- `projection_policy == :local_only` is valid. Linear/Jira/GitHub credentials
  are not required for correctness.
- `scope_hash`, `decision_revision_hash`, `base_sha`,
  `implementation_plan_revision`, and `autonomy_envelope_revision` form part
  of the deterministic snapshot equality set. Mismatch blocks Execute.
- Scope is not prompt context. Open-Sleigh must pass it into Session and
  AdapterSession, enforce it before every mutating adapter call, and validate
  the terminal diff against it.

---

### Scope

**Definition:** Closed authorization object owned by Haft. It says what the
runner may touch for one WorkCommission. It is immutable in Open-Sleigh and
hashed into the commission snapshot.

**Attributes:**

- `repo_ref :: String.t()` (required) — repository identity
- `base_sha :: String.t()` (required) — pinned commit for deterministic
  comparison
- `target_branch :: String.t()` (required) — branch or policy admitted for
  this commission
- `allowed_paths :: [String.t()]` (required) — path globs the runner may
  mutate
- `forbidden_paths :: [String.t()]` (required) — path globs the runner must
  not mutate even if a broad allowed path matches
- `allowed_actions :: MapSet.t(atom())` (required) — e.g. `:edit_files`,
  `:run_tests`, `:commit`
- `affected_files :: [String.t()]` (required) — expected evidence/mutation
  surface
- `allowed_modules :: [String.t()]` — optional module-level authorization
- `lockset :: [String.t()]` (required) — concurrency-control projection
- `hash :: String.t()` (required) — canonical serialized hash

**Key rules:**

- Scope is stronger than workspace safety. A path can be inside
  `workspace_path` and still be illegal for this commission.
- `PathGuard` prevents filesystem escapes; Scope prevents authority escapes.
  Both checks are required for any mutating tool.
- End-of-run diff validation must fail terminally if any mutation is outside
  Scope, even when all phase tools and workspace checks passed.

---

### Ticket / ExternalWorkItemSnapshot

**Definition:** Legacy/current-implementation snapshot of a unit of work in an
external tracker. In the commission-first target model it becomes
`ExternalWorkItemSnapshot`: projection evidence and approval/comment carrier,
not source of work authority.

**Attributes:**

- `id :: String.t()` (required) — tracker-native identifier
- `source :: {:linear, workspace :: String.t()} | {:github, repo :: String.t()} | ...` (required) — tracker + scope
- `title :: String.t()` (required)
- `body :: String.t()` (required; may be empty)
- `state :: atom()` (required) — current tracker state (snapshot)
- `problem_card_ref :: ProblemCardRef.t() | nil` — legacy tracker-first
  bridge. Commission-first mode reads this from WorkCommission, not tracker
  text.
- `target_branch :: String.t() | nil` — hint for HumanGate `external_publication` matching
- `metadata :: map()` — tracker-specific fields (labels, assignees, etc.)
- `fetched_at :: DateTime.t()` (required)

**Relationships:**

- May link to one ExternalProjection.
- Does not own Session or PhaseOutcome in commission-first mode.

**Key rules:**

- A legacy Ticket without `problem_card_ref` is not accepted by the
  tracker-first bridge. Commission-first mode should not require parsing this
  field from tracker text.
- A tracker state snapshot may go stale. External changes are observed as
  projection drift/override and never complete a WorkCommission by themselves.

---

### Session

**Definition:** One `(WorkCommission × Phase × ConfigHash × AdapterSession)` unit
of work owned by one `AgentWorker`. Identified by opaque `SessionId`.

**Attributes:**

- `id :: SessionId.t()` (required; opaque binary)
- `commission :: WorkCommission.t()` (required; snapshot at preflight claim time)
- `scope :: Scope.t()` (required; copied from `commission.scope` for direct
  adapter access)
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

- Belongs to one `WorkCommission`.
- Owned by exactly one `AgentWorker` (L5 Task).
- Produces zero or more `PhaseOutcome`s during its lifetime.

**Key rules:**

- `workspace_path` MUST be outside `open_sleigh/` and `~/.open-sleigh/` — validated at session construction. This is the Thai-disaster architectural guardrail expressed at L1.
- `scope` MUST be present and hash-matched against `commission.scope_hash`.
  Session construction fails on mismatch.
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

### PreflightReport

**Definition:** Structured result of the first Open-Sleigh phase. It combines
deterministic Haft checks with agent-assisted context inspection, then returns
to Haft for validation. It is a report, not authority.

**Attributes:**

- `commission_id :: String.t()` (required)
- `decision_ref :: DecisionRecordRef.t()` (required)
- `decision_revision_seen :: String.t()` (required)
- `problem_revision_seen :: String.t()` (required)
- `scope_hash_seen :: String.t()` (required)
- `base_sha_seen :: String.t()` (required)
- `implementation_plan_revision_seen :: String.t() | nil`
- `autonomy_envelope_revision_seen :: String.t() | nil`
- `snapshot_checks :: %{atom() => :match | :mismatch | :not_applicable}`
  (required)
- `verdict :: :pass | :block_stale | :block_policy | :block_conflict |
  :needs_human_review` (required)
- `checked_artifacts :: [String.t()]` (required)
- `checked_files :: [String.t()]` (required)
- `context_changes :: [map()]` (required)
- `blocking_reasons :: [String.t()]` (required when verdict != `:pass`)
- `non_blocking_observations :: [String.t()]`
- `recommended_next_action :: String.t() | nil`

**Key rules:**

- PreflightReport cannot start Execute. Haft validates it and changes
  WorkCommission state.
- `decision_revision_seen` must match Haft's current DecisionRecord revision
  for `:pass`.
- `scope_hash_seen`, `base_sha_seen`, plan revision, and envelope revision
  must match Haft's deterministic snapshot equality set for `:pass`.
- Agent uncertainty maps to `:needs_human_review`, not optimistic pass.

---

### Workflow

**Definition:** Immutable graph data describing legal phase transitions.

**Attributes:**

- `id :: :mvp1 | :mvp2 | atom()` (required)
- `phases :: [Phase.t()]` (required; declared alphabet)
- `transitions :: %{Phase.t() => [Phase.t()]}` (required; from-phase → legal to-phases)
- `terminal :: [Phase.t()]` (required; absorbing states)
- `entry_phase :: Phase.t()` (required; `:preflight` in commission-first mode)

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
- `commission_snapshot_hash :: String.t()` (required) — exact commission
  snapshot the human approved
- `reason :: String.t() | nil` — ≤ 500 chars
- `signal_source :: :desktop_ack | :cli_ack | :tracker_comment | :github_review`
  (required)
- `signal_ref :: String.t()` (required) — the external reference to the approval event

**Key rules:**

- Constructed only by `HumanGateListener` (L5) upon receiving a valid signal.
- `approver` must be in the `approvers` set from the active `SleighConfig`; otherwise constructor fails with `{:error, :approver_not_authorised}`.
- `HumanGateApproval`, when required, flows into `PhaseOutcome.gate_results`
  as a `{:human, HumanGateApproval.t()}` tuple. Per Q-OS-3 resolution (v0.5),
  there is **no `PhaseOutcome.new_external/3`** — a single
  `PhaseOutcome.new/2` validates **gate-config consistency**: if the active
  `PhaseConfig.gates.human` list declares `publish_approved`,
  `one_way_door_approved`, or any other human gate, the constructor requires a
  matching approved `{:human, HumanGateApproval.t()}` entry in `gate_results`,
  else it fails with `:human_gate_required_by_phase_config_but_missing`.
  The invariant lives at the single constructor; no special-case path exists
  that bypasses it.
- Approval is scoped to `commission_snapshot_hash`. Snapshot drift invalidates
  reuse and requires re-approval or re-preflight.

---

### AdapterSession

**Definition:** L4 effect context passed to every adapter call. Carries
the session-level metadata that every I/O call needs.

**Attributes:**

- `session_id :: SessionId.t()`
- `config_hash :: ConfigHash.t()`
- `scoped_tools :: MapSet.t(atom())`
- `workspace_path :: Path.t()`
- `scope :: Scope.t()`
- `adapter_kind :: :codex | :claude | atom()`
- `adapter_version :: String.t()`
- `max_turns :: pos_integer()`
- `max_tokens_per_turn :: pos_integer()`
- `wall_clock_timeout_s :: pos_integer()`

**Key rules:**

- `AdapterSession` is constructed by L5 when spawning an `AgentWorker`, then passed verbatim to every adapter call. No L4 function takes session-level metadata separately.
- `scoped_tools` is the phase's tool set intersected with the adapter's capabilities; compilation of `sleigh.md` at L6 rejects tools the adapter can't bind.
- `scope` is checked before every mutating adapter call. A tool may be known
  to the adapter and allowed in the phase, but still rejected as
  `:mutation_outside_commission_scope`.

---

### SleighConfig

**Definition:** L6 compiled, immutable configuration struct produced by
`Sleigh.Compiler.compile/1`. Contains all phase/gate/adapter bindings
resolved from `sleigh.md`.

**Attributes:**

- `engine :: EngineConfig.t()` — poll interval, concurrency
- `commission_source :: CommissionSourceConfig.t()` — Haft command/version,
  poll interval, lease timeout, plan/queue selectors
- `projection :: ProjectionConfig.t()` — optional external targets and writer
  profile; may be disabled
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
WorkCommission ──1──> Session (owned by AgentWorker)
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
DecisionRecordRef ── points to Haft artifact (upstream)
ExternalWorkItemSnapshot ── optional projection carrier
```

---

## Storage model — what lives where

| Entity | MVP-1 location | MVP-2 location |
|---|---|---|
| `WorkCommission` (snapshot) | Orchestrator ETS | Same |
| `ExternalWorkItemSnapshot` | Optional projection cache | Same |
| `Session` | Orchestrator GenServer state | Same (plus SQLite optional for crash recovery) |
| `PhaseOutcome` | WAL until written to Haft; then Haft SQLite via MCP | Same |
| `Evidence` | Attached to PhaseOutcome, flows with it | Same |
| `HumanGateApproval` | Inside PhaseOutcome.gate_results | Same |
| `SleighConfig` | WorkflowStore GenServer state | Same |
| `Workflow` | Module attribute (compile-time) | Same |
| Observations (`gate_bypass_rate` etc.) | `ObservationsBus` ETS | Same + export adapter (opt-in) |

Canonical rules:

- **Haft SQLite is the authoritative object store for WorkCommissions,
  decisions, runs, evidence, and projection intent.**
- **External trackers are optional carriers for coordination.**
- **Orchestrator is the sole writer of in-RAM session state.**
- **ObservationsBus never reaches Haft.** Compile-time enforced.
