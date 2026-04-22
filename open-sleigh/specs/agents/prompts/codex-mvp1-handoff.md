# Open-Sleigh — MVP-1 Handoff for a Coding Agent (Codex / Claude Code)

> **Role for the incoming agent.** You are taking over implementation
> of Open-Sleigh MVP-1 at the "skeleton green" checkpoint. Your job is
> to replace in-memory mocks with real subprocess / HTTP integrations
> and add the remaining runtime features so the canary suite can run
> for 24h green on the `sleigh-canary` repo, and then on one real
> `octacore_nova` ticket.
>
> **You are not the system's principal. You do not frame problems.
> You implement bounded, specified tasks.** If a task requires a
> decision that isn't already in the specs, stop and ask the human —
> don't invent.

---

## 0. Read order (before you touch anything)

Read in this order. Do NOT skip. Each layer below builds on the one above.

1. **`SPEC.md`** — operator-facing umbrella (~300 lines). Pointer index only.
2. **`specs/README.md`** — canonical reading order for the spec tree.
3. **`specs/target-system/SYSTEM_CONTEXT.md`** — what Open-Sleigh is, why, boundary.
4. **`specs/target-system/TERM_MAP.md`** — canonical vocabulary. If you use a word not here, it's a review finding.
5. **`specs/target-system/SCOPE_FREEZE.md`** — what's in MVP-1 / what's explicitly cut. Do not build cut items.
6. **`specs/target-system/PHASE_ONTOLOGY.md`** — 5-axis phase model.
7. **`specs/target-system/ILLEGAL_STATES.md`** — 80+ illegal states with four-label enforcement taxonomy. **Read fully.** Every line of code you write must preserve these.
8. **`specs/target-system/AGENT_PROTOCOL.md`** — JSON-RPC contract for `Agent.Adapter`. Normative.
9. **`specs/target-system/HAFT_CONTRACT.md`** — MCP contract for `haft serve`.
10. **`specs/enabling-system/FUNCTIONAL_ARCHITECTURE.md`** — 6-layer hierarchy + L4/L5 ownership seam. Read the **Thai-disaster architectural guardrails** table at the top.
11. **`specs/enabling-system/IMPLEMENTATION_PLAN.md`** — layered build order; maps 1:1 to the current code state.
12. **`specs/enabling-system/STACK_DECISION.md`** — no extra deps without an ADR.

Then skim `.context/symphony/SPEC.md` + `.context/symphony/elixir/lib/symphony_elixir/*.ex` for reference of what Symphony does in equivalent places (especially `path_safety.ex`, `orchestrator.ex`, `codex/app_server.ex`). Reuse patterns where they match our spec; do **not** import Symphony's WORKFLOW.md prompt shape (that's the ceremony-generator anti-pattern).

Also skim `.context/hustles/thai/.context/audit-general-analysis.md` — the negative exemplar. Our Thai-disaster guardrails exist because of this. If a change of yours would re-enable any of the patterns there, the change is wrong regardless of how much cleaner the code looks.

---

## 1. Checkpoint state (2026-04-22)

**57 `.ex` files in `lib/`, 41 `_test.exs` files in `test/`, 290 tests passing.**

L1 (core types) → L5 (OTP orchestration) skeleton is **green**:

```
mix compile --warnings-as-errors    # clean
mix test                             # 2 properties, 290 tests, 0 failures
mix credo --strict                   # 616 mods/funs, 0 issues
mix format --check-formatted         # clean
```

**Canary integration test green:**
`test/open_sleigh/canary_happy_path_test.exs` wires the full pipeline
(TrackerMock → Orchestrator → AgentWorker → GateChain → PhaseOutcome →
HaftMock → NextDecision → …) and advances an MVP-1 T3-like ticket through
`:frame → :execute → :measure → {:terminal, :pass}`. Three Haft writes, one
`:session_terminal` observation. This is the baseline. If your changes
break this test without replacement, your changes are wrong.

**What's in the repo right now:**

| Layer | Modules | Purpose | Status |
|-------|---------|---------|--------|
| L1 core types | 19 | `Phase, Verdict, GateKind, AuthoringRole, RunAttemptSubState, ConfigHash, SessionId, ProblemCardRef, SessionScopedArtifactId, Evidence, HumanGateApproval, GateResult, Workflow, Ticket, AdapterSession, Session, PhaseConfig, PhaseOutcome` | **Complete.** PhaseOutcome.new/2 enforces PR5 (self-ref) + PR10 (gate-config consistency). |
| L2 gate algebra | 15 | `Gates.Registry, Gates.Structural/Semantic/Human behaviours, 5 structural impls, 3 semantic impls (contract-only), 1 human impl, GateChain, GateContext` | **Complete.** Semantic gates are contracts; real judge call is L4 `JudgeClient` with injected invoker. |
| L3 phase machine | 3 | `PhaseMachine.next/2, WorkflowState, NextDecision` | **Complete.** Total function over closed `Phase.t()` alphabet. |
| L4 adapter boundary | 12 | `EffectError, Adapter.PathGuard, Agent.{Adapter, Protocol, Mock}, Tracker.{Adapter, Mock}, Haft.{Protocol, Client, Mock}, JudgeClient` | **Skeleton.** Behaviours + pure codecs + mocks. Real Codex/Linear/haft-serve implementations are the first tasks below. |
| L5 OTP | 9 | `ObservationsBus, WorkspaceManager, HaftServer, HaftSupervisor, WorkflowStore, AgentWorker (single-turn), Orchestrator (single-writer), TrackerPoller, Application` | **Skeleton.** Continuation loop, retry/backoff, stall detection, WAL all named as TODO in code. |
| L6 DSL compiler | 0 | `Sleigh.Compiler` for `sleigh.md` YAML + Markdown | **Not started.** Task 8 below. |

**Dependencies (from `mix.exs`, all pinned):**
Runtime: `jason`. Dev/test: `stream_data, excoveralls, credo, dialyxir`.
Do not add new runtime deps without an ADR.

---

## 2. Hard rules — these are non-negotiable

### 2.1 Green-gate discipline

Before you commit any change:

```
mix format
mix compile --warnings-as-errors
mix test
mix credo --strict
```

All four MUST exit clean. A commit that breaks any of them is broken.

### 2.2 Spec-first, code-second

If a task description in this doc conflicts with the specs, the **specs win**. SPEC.md is an umbrella index; normative content lives under `specs/`. When they conflict, the `specs/` document is authoritative.

If you find a genuine ambiguity in the specs, STOP and post a question. Do not invent. Ambiguity left in is a review finding; ambiguity invented-through is a review failure.

### 2.3 Thai-disaster guardrails

These are rooted in architectural mechanism, not discipline. You MUST NOT break them:

| Guardrail | Mechanism | Where it's tested |
|-----------|-----------|-------------------|
| Workspace-path allowlist (CL5–CL11) | `OpenSleigh.Adapter.PathGuard` — `Path.expand` + symlink dereference + `.git` remote check | `test/open_sleigh/adapter/path_guard_test.exs` |
| Observation-to-Haft isolation (OB1/OB5) | `ObservationsBus` has zero `.beam` imports from any `OpenSleigh.Haft.*` module; no `alias/import/use` in source | `test/open_sleigh/observations_bus_test.exs` |
| Bounded prose (PR4) | `PhaseOutcome.rationale` ≤ 1000 chars; `HumanGateApproval.reason` ≤ 500; no unbounded narrative fields | `test/open_sleigh/phase_outcome_test.exs` |
| Upstream framing lock (UP1–UP3) | `Ticket.new/1` requires `problem_card_ref`; Frame has no `haft_problem` in its tools; `AuthoringRole` excludes `:human` from agent paths | `test/open_sleigh/ticket_test.exs` + framing gates |

If your code makes any of these tests need to be relaxed, **the problem is in your code**, not the test.

### 2.4 FPF distinctions

- **Object ≠ Description ≠ Carrier.** The running engine ≠ `specs/*.md` ≠ `lib/**/*.ex`. A passing test is evidence, not the thing.
- **Transformer Mandate (X-TRANSFORMER).** External agent decides; system doesn't self-improve; human is the principal. You are the coding agent; the human principal (Ivan) makes value / scope / one-way-door decisions.
- **Design-time ≠ Run-time.** A constructor check is design-time; a production run with measurement is run-time. Don't promise runtime behavior you can't verify.

### 2.5 Layer discipline (v0.6.1 L4/L5 ownership seam)

- **L1–L3 are pure.** No `GenServer`, no `Task`, no `Port`, no `File.*` writes.
- **L4 is effectful but stateless.** Behaviours + pure codecs + functions taking a handle/invoker as a parameter.
- **L5 owns processes.** Any `Port`, `GenServer`, `Task`, or long-lived resource lives here.
- If you need to put a subprocess or long-running connection somewhere, it goes at L5. The L4 stateless API calls into that L5 process.

### 2.6 Coding conventions

- Pipeline style: `data |> step1() |> step2()`, one step per line.
- Multi-clause function heads over `if/case` chains.
- `@spec` on every public function (Credo enforces).
- `@moduledoc` on every module.
- **No default function parameters.** Use pattern-matching heads or explicit overload.
- No `throw`/`raise` for control flow — return `{:ok, _} | {:error, atom()}` typed errors.
- Typed errors are closed sums in `OpenSleigh.EffectError` — extending requires source change.
- Immutability: every function returns a new value; never mutate in place.
- Max CC = 5 (Credo enforced).
- Max nesting depth = 2 (Credo enforced).
- No direct struct literals for L1 types outside their defining module (`%PhaseOutcome{...}`, `%Evidence{...}`, `%HumanGateApproval{...}` — always go through `.new/_`). Credo rule `OpenSleigh.Credo.NoDirectStructLiteral` is scheduled; until it lands, honour the convention manually.

### 2.7 Commit discipline

- One task = one commit (or a small series with obvious separation).
- Commit message: `<area>: <what changed>` one-liner + optional body. No "fixed things", no "wip", no emojis.
- Never `--amend` merged commits. Never `git push --force` to `main`.
- Do not `git add .` — list the files you meant to add.
- Before pushing, run all four green-gate commands again. If red, fix before push.

---

## 3. Tasks queue (execute in order; each is a PR-sized commit)

> **Task format.** Each task has: **What**, **Why**, **Files**, **Acceptance**, **Specs**. Stay inside the named scope — no "while I'm here" refactors.

### Task 1 — Continuation-turn loop in `AgentWorker`

**What.** Replace the single-turn implementation in `lib/open_sleigh/agent_worker.ex` with a continuation-turn loop that runs N turns on the same thread until gates pass OR ticket leaves active state OR `turn_count == max_turns`.

**Why.** Per `AGENT_PROTOCOL.md §3` + SPEC §5.1, Execute phase is multi-turn. Frame and Measure stay single-turn (`Phase.single_turn?/1` already returns true for them). Current single-turn AgentWorker closes the session after one turn regardless.

**Files.**

- Modify: `lib/open_sleigh/agent_worker.ex` — add `continuation_loop/N` helper.
- Modify: `lib/open_sleigh/agent/mock.ex` — let `send_turn/3` optionally return a non-completed status on turn 1 so continuation tests can exercise the loop (configurable via `Process` dict or an optional adapter-state).
- Add: `test/open_sleigh/agent_worker_continuation_test.exs` — new test file covering:
  - Frame / Measure sessions always run exactly 1 turn.
  - Execute sessions run up to `max_turns` if gates don't pass yet.
  - Execute sessions stop early when the tracker state changes to non-active (use `Tracker.Mock` + mid-test `transition/3`).
  - Continuation turn prompt is the `Continuation guidance` block (AGENT_PROTOCOL.md §3), not the first-turn prompt (CT1).

**Acceptance.**

- All existing 290 tests still pass.
- New tests in `agent_worker_continuation_test.exs` cover the four scenarios above.
- `mix credo --strict` 0 issues.
- **Key invariants preserved:** CT1 (continuation uses guidance text, not the full prompt); CT2 (same thread across turns — no new `start_session` call); CT5 (no turn after PhaseOutcome emitted); CT6 (scoped tools don't mutate mid-thread).

**Specs.** `AGENT_PROTOCOL.md §3`, `PHASE_ONTOLOGY.md §Axis 5`, `ILLEGAL_STATES.md` CT1/CT2/CT4/CT5/CT6.

---

### Task 2 — Real `OpenSleigh.Agent.Codex` adapter

**What.** Implement the `Agent.Adapter` behaviour backed by `codex app-server` subprocess over a BEAM `Port`. The L5 process that owns the Port is new: `OpenSleigh.Agent.Codex.Server` (GenServer). The L4 module `OpenSleigh.Agent.Codex` is stateless and dispatches through the L5 server.

**Why.** The Mock adapter is fine for skeleton tests, but canary needs to actually run Codex. Per v0.6.1 L4/L5 seam, L4 module `OpenSleigh.Agent.Codex` is stateless typed API; L5 `OpenSleigh.Agent.Codex.Server` owns the Port.

**Files.**

- Add: `lib/open_sleigh/agent/codex.ex` — L4 behaviour impl; each callback delegates via `GenServer.call(server_pid, ...)`.
- Add: `lib/open_sleigh/agent/codex/server.ex` — L5 GenServer owning `Port`; uses `Agent.Protocol` codec to encode/decode. Handles stdout line-buffering (max 10 MB per SPEC). Handles stderr as diagnostic only. Handles subprocess exit.
- Add: `lib/open_sleigh/agent/codex/supervisor.ex` — per-session Codex supervisor.
- Extend: `mix.exs` — no new deps needed (Port is stdlib).
- Add: `test/open_sleigh/agent/codex_test.exs` — uses a mock executable that speaks the protocol (a tiny Elixir script written to a tmp file + executed via `bash`). Or use `@tag :integration` + require real `codex app-server` binary.
- Add: `@tag :integration` setup in `test/test_helper.exs` — integration tests skip unless `CODEX_CMD` env is set.

**Acceptance.**

- Unit tests for codec round-tripping (protocol encode/decode) — already covered by `test/open_sleigh/agent/protocol_test.exs`; no duplication.
- Integration test (skip-by-default) that starts a real `codex app-server`, completes handshake, runs one turn, shuts down.
- Stall detection (`stall_timeout_ms`) fires if no event in that window — test with a mock binary that pauses.
- `ADAPTER_PARITY.md` updated with the Codex-specific parity entries (turn budgets, tool surface).
- All existing tests still pass.

**Specs.** `AGENT_PROTOCOL.md §1, §2, §4, §6, §8`, `FUNCTIONAL_ARCHITECTURE.md LAYER 4` (ownership seam), `ILLEGAL_STATES.md` CL1, AD1–AD6.

---

### Task 3 — Real `OpenSleigh.Tracker.Linear` adapter

**What.** Implement the `Tracker.Adapter` behaviour against the Linear GraphQL API via `Finch`.

**Why.** `Tracker.Mock` is fine for tests; canary needs to read actual Linear tickets.

**Files.**

- Add: `lib/open_sleigh/tracker/linear.ex` — L4 behaviour impl; uses a handle = `%{endpoint, api_key, project_slug}`.
- Add: `lib/open_sleigh/tracker/linear/queries.ex` — GraphQL queries for `list_active`, `get`, `transition`, `post_comment` (copy shapes from `.context/symphony/elixir/lib/symphony_elixir/linear/` for reference).
- Extend: `mix.exs` — add `{:finch, "~> 0.18"}`.
- Add: `test/open_sleigh/tracker/linear_test.exs` — unit tests for query builders (pure); `@tag :integration` tests against a real Linear sandbox (skip unless `LINEAR_API_KEY` set).

**Acceptance.**

- `list_active/1` returns `Ticket.t()` with `problem_card_ref` extracted from a ticket custom field (decide with the principal which custom field name to use — add to `OPEN_QUESTIONS.md` if undecided).
- `transition/3` requests a Linear state transition; `post_comment/3` posts a comment.
- All error classes from `EffectError` that apply map correctly (`:tracker_request_failed`, `:tracker_status_non_200`, `:tracker_response_malformed`).
- Integration test against a sandbox Linear project (skip-by-default).
- All existing tests still pass.

**Specs.** `HAFT_CONTRACT.md` + Linear GraphQL docs. `SYSTEM_CONTEXT.md §4` for the tracker integration boundary.

**Open question to resolve before implementing.** Which custom field on Linear issues holds `problem_card_ref`? Post as a question before coding.

---

### Task 4 — Real `haft serve` Port wrapper in `HaftServer`

**What.** Replace the injected `invoke_fun` in `HaftServer.init/1` with a Port-owning implementation that spawns `haft serve` as a subprocess and dispatches requests via stdin / stdout.

**Why.** `HaftMock` is a good test tool; production needs the real MCP subprocess.

**Files.**

- Modify: `lib/open_sleigh/haft_server.ex` — keep the `invoke_fun` injection for tests BUT add an alternative `start_link/1` mode: `[command: "haft serve", ...]` that spawns a Port. If `invoke_fun` is supplied, use it (test mode); if `command` is supplied, spawn the Port.
- Implement: line-buffered stdout reader that matches response ids to pending calls.
- Implement: health-ping every 10s; 3 consecutive misses → `:haft_unavailable` state (SPEC §7.1).
- Add: `test/open_sleigh/haft_server_integration_test.exs` with `@tag :integration`, requires `haft serve` binary on PATH.

**Acceptance.**

- Mock-based unit tests still pass.
- Integration test against real `haft serve` passes a round-trip of `haft_query(status)`.
- Health-ping + unavailable-state handling covered by a unit test that starts the server with a mock invoker that mimics unavailability.
- All existing tests still pass.

**Specs.** `HAFT_CONTRACT.md §2–§3`, `ILLEGAL_STATES.md` AD3 + SE7.

---

### Task 5 — `HumanGateListener`

**What.** New L5 module that listens for `/approve` / `/reject` signals from the tracker (polls comments on tickets with pending human gates) and delivers `HumanGateApproval` structs to the Orchestrator.

**Why.** Without this, T3 canary (PR → main) can't complete; the commission_approved gate is required by PhaseOutcome's gate-config consistency for the Execute phase when `external_publication` matches.

**Files.**

- Add: `lib/open_sleigh/human_gate_listener.ex` — GenServer. State: `%{pending: %{ticket_id => %{gate_name, posted_at, ...}}}`. Polls tracker comments via `Tracker.Adapter.get/2` + comment-fetch extension. Emits `{:human_approval, ticket_id, HumanGateApproval.t()}` cast to Orchestrator on `/approve`.
- Extend: `lib/open_sleigh/tracker/adapter.ex` + `lib/open_sleigh/tracker/mock.ex` — add `list_comments/2` callback.
- Extend: `lib/open_sleigh/tracker/linear.ex` — implement `list_comments/2`.
- Modify: `lib/open_sleigh/orchestrator.ex` — handle `{:human_approval, ...}` cast; when AgentWorker's PhaseOutcome construction is blocked waiting for approval (gate-config consistency), park the outcome and resume on approval arrival.
- Add: `test/open_sleigh/human_gate_listener_test.exs` — uses `Tracker.Mock` with seeded comments; verifies listener picks up `/approve`, builds `HumanGateApproval`, sends to orchestrator.
- Extend canary integration test: add `test/open_sleigh/canary_t3_test.exs` — T3 ticket with `target_branch: "main"`, HumanGate fires, operator posts `/approve` via `Tracker.Mock.post_comment`, session advances to Measure.

**Acceptance.**

- `HumanGateApproval` is built correctly with the active session's `config_hash` (PR9) and an approver from the configured list (PR8).
- `/approve` from an unauthorised approver is rejected with a tracker comment explaining why.
- `/reject <reason>` regresses the ticket to the previous phase per SPEC §6c.
- 24h timeout escalates; 72h cancels the worker — test with a shortened timeout (pass as option to HumanGateListener for tests).
- Canary T3 integration test passes end-to-end with HumanGate firing.
- All existing tests still pass.

**Specs.** `SPEC §6c`, `GATES.md §4`, `ILLEGAL_STATES.md` PR8, PR9, PR10.

---

### Task 6 — Retry queue + stall detection in `Orchestrator`

**What.** Add retry logic to Orchestrator. On `{:error, session_id, reason}` or on a `:stalled` / `:timed_out` outcome, schedule a retry with exponential backoff instead of just releasing the claim.

**Why.** Current Orchestrator releases claim on failure (deliberate MVP-1 simplification). Before canary, we need exponential backoff retries so transient failures don't just lose the work.

**Files.**

- Modify: `lib/open_sleigh/orchestrator.ex` — add `state.retries :: %{ticket_id => %{attempt, due_at_ms, timer_ref, error}}`. Add `handle_info(:retry_timer_fired, state)`. Backoff formula: `delay = min(10_000 * 2^(attempt - 1), max_retry_backoff_ms)` per SPEC §10 v0.3 wording (see HAFT_CONTRACT.md §3).
- Modify: `lib/open_sleigh/agent_worker.ex` — propagate `:stalled` / `:timed_out` as distinct `{:error, sid, :stalled | :timed_out}` casts.
- Add: stall-detection timer in `AgentWorker` — if no event on the Port for `codex.stall_timeout_ms`, kill the Port and exit with `:stalled`.
- Add: `test/open_sleigh/orchestrator_retry_test.exs` — stress tests:
  - Abnormal worker exit schedules retry with correct backoff.
  - Normal worker exit schedules 1000ms continuation-retry (Symphony pattern).
  - Retry timer fired with active ticket → re-dispatches.
  - Retry timer fired with non-active ticket → releases claim.

**Acceptance.**

- Exp backoff matches Symphony's `min(10_000 * 2^(attempt-1), max_retry_backoff_ms)`.
- Stall-timeout fires and retries.
- Retry queue persists across multiple failures for the same ticket (attempt counter).
- Worker-down monitoring — if the AgentWorker Task dies unexpectedly, orchestrator detects via Process.monitor and schedules retry.
- All existing tests still pass.

**Specs.** Symphony `SPEC.md §8.4, §14` (adapt, don't copy), our `AGENT_PROTOCOL.md §8 & §9`.

---

### Task 7 — WAL replay in `HaftSupervisor`

**What.** When Haft is unavailable, `HaftServer.call/2` currently just returns `{:error, :haft_unavailable}`. Per HAFT_CONTRACT.md §3, the PhaseOutcome write should instead be appended to `~/.open-sleigh/wal/<ticket_id>.jsonl` and replayed when Haft reconnects.

**Why.** Without WAL, a Haft blip loses provenance. Non-optional for canary.

**Files.**

- Modify: `lib/open_sleigh/haft_server.ex` — on `:haft_unavailable`, append to WAL file; on reconnect (state transitions to `available: true`), replay the WAL before accepting new calls.
- Add: `lib/open_sleigh/haft/wal.ex` — pure module, functions: `append/3`, `replay/2`. Append is per-ticket (parse `ticket_id` from the request params). Replay walks `~/.open-sleigh/wal/` directory, reads each `<ticket_id>.jsonl` in file-mtime order, replays line-by-line via the invoker, deletes the file on full success.
- Add: `test/open_sleigh/haft/wal_test.exs` — unit tests.
- Add: `test/open_sleigh/haft_server_wal_test.exs` — integration: start HaftServer with an invoker that fails for N calls then recovers; verify WAL files populated, then replayed, then deleted.

**Acceptance.**

- Per-ticket append order preserved (SPEC §7.1).
- Tickets-in-arrival order across per-ticket files on replay.
- Replay completes before new dispatches accepted (SE4).
- Failed writes during replay leave the WAL intact for next reconnect.
- `wal_dir` is configurable via `HaftServer.start_link/1` opts; default `~/.open-sleigh/wal/`.
- All existing tests still pass.

**Specs.** `HAFT_CONTRACT.md §3`, SPEC `§7.1` WAL ordering.

---

### Task 8 — L6 `Sleigh.Compiler`

**What.** Parse `sleigh.md` (YAML front matter + Markdown body) into a compiled bundle that `WorkflowStore.put_compiled/1` accepts. Validate every gate / tool / phase / prompt-variable reference against the L2/L4/L1 registries; reject over-budget files.

**Why.** Without L6, operators configure Open-Sleigh by calling `WorkflowStore.start_link/1` with Elixir maps — not sustainable. `sleigh.md` is the designed L_top surface.

**Files.**

- Add: `lib/open_sleigh/sleigh/compiler.ex` — `compile/1 :: (String.t() -> {:ok, bundle()} | {:error, [CompilerError.t()]})`.
- Add: `lib/open_sleigh/sleigh/size_budget.ex` — `check/1` per CF1/CF2/CF10 (300-line file, 150-line prompt, 50 KB byte cap).
- Add: `lib/open_sleigh/sleigh/compiler_error.ex` — typed error sum.
- Extend: `mix.exs` — add `{:yaml_elixir, "~> 2.11"}` and `{:earmark_parser, "~> 1.4"}`.
- Add: `lib/open_sleigh/sleigh/watcher.ex` — GenServer that polls `sleigh.md` (file stat mtime), re-compiles on change, hot-loads into `WorkflowStore` (for new sessions only per SE6 — in-flight sessions keep their pinned hash).
- Add: `test/open_sleigh/sleigh/compiler_test.exs` — cover CF1–CF10 illegal states; every CF row is a failing test.
- Add: `sleigh.md.example` at repo root — the canonical example from SPEC §8.

**Acceptance.**

- Every CF1–CF10 illegal state has a failing-case test that asserts the specific `CompilerError` atom.
- Valid `sleigh.md.example` compiles to a bundle that Orchestrator can use (covered via an integration test: start `Sleigh.Watcher`, touch `sleigh.md`, verify `WorkflowStore` updated).
- Hot-reload does not affect in-flight sessions' `config_hash` (SE6).
- Adversarial tests — include-directive bloat (CF9), byte-cap overflow (CF10) — fail compilation.
- All existing tests still pass.

**Specs.** `SLEIGH_CONFIG.md`, `ILLEGAL_STATES.md` CF1–CF10 + SE6.

---

### Task 9 — `mix open_sleigh.start` and `mix open_sleigh.canary` tasks

**What.** Add Mix tasks that start the engine per `SLEIGH_CONFIG.md` operator workflow.

**Why.** Operators need a one-command start; canary suite needs its own automation entry.

**Files.**

- Add: `lib/mix/tasks/open_sleigh.start.ex` — reads `sleigh.md`, starts the supervision tree (Orchestrator + TrackerPoller + HaftSupervisor + WorkflowStore + HumanGateListener + Sleigh.Watcher), runs until ctrl+c.
- Add: `lib/mix/tasks/open_sleigh.canary.ex` — starts engine against a seeded in-memory tracker with canary tickets T1/T1'/T2/T3 (fixtures in `test/support/canary_tickets.exs` or similar), runs for a configurable duration, asserts expected outcomes per `SCOPE_FREEZE.md §Canary`.
- Add: `test/open_sleigh/mix_tasks_test.exs` — tests for both tasks (uses `Mix.Task.run/2`).

**Acceptance.**

- `mix open_sleigh.start` boots with a minimal `sleigh.md`.
- `mix open_sleigh.canary` runs T1/T1'/T2/T3 and exits green if all assertions pass.
- Each task has a `--help` output documenting flags.
- All existing tests still pass.

**Specs.** `IMPLEMENTATION_PLAN.md §Canary suite`, SPEC `§10.1`.

---

### Task 10 — Optional HTTP observability API (MVP-1 optional)

**What.** If `server.port` is set in `sleigh.md`, start a minimal HTTP server exposing `GET /api/v1/state`, `GET /api/v1/<ticket>`, `POST /api/v1/refresh`.

**Why.** Operators want to see what the engine is doing without a terminal.

**Files.**

- Add: `lib/open_sleigh/http/server.ex` — `Bandit` or `Plug.Cowboy` HTTP server (prefer `Bandit` for zero-ceremony, but check current best-practice).
- Add: `lib/open_sleigh/http/router.ex` — `Plug.Router` with three endpoints per `HTTP_API.md`.
- Extend: `mix.exs` — add `{:plug, "~> 1.16"}` and `{:bandit, "~> 1.5"}` (get explicit sign-off before adding — these are runtime deps).
- Add: `test/open_sleigh/http/router_test.exs` — use `Plug.Test` to exercise each endpoint.

**Acceptance.**

- Each endpoint returns the documented JSON shape from `HTTP_API.md`.
- API is read-only except `POST /api/v1/refresh` (cast to Orchestrator).
- Orchestrator correctness does not depend on the HTTP server running (verify by killing the HTTP supervisor in a test and ensuring the engine still processes tickets).
- HTTP response never references any Haft artifact (verify via test that reads the endpoints and asserts no `haft_` field in responses).
- All existing tests still pass.

**Specs.** `HTTP_API.md`.

---

## 4. What is NOT your job

- **Do not write new spec documents.** The `specs/` tree is frozen for MVP-1 behaviour. Clarifying an existing spec is OK if a task forces it; opening a new spec without explicit sign-off is not.
- **Do not add UI / LiveView / frontend.** MVP-2 concern.
- **Do not build multi-tenant features.** Solo-principal system.
- **Do not introduce new runtime deps beyond those listed in Tasks 3, 8, 10.** If you think you need one, post an ADR draft first and wait for sign-off.
- **Do not restructure L1–L5 layering.** Ownership seam is settled (v0.6.1).
- **Do not modify `test/open_sleigh/canary_happy_path_test.exs`** to accommodate your changes. That test is the baseline. If your change breaks it, the change is wrong OR you need to ADD new tests and keep the old one passing.
- **Do not add runtime governance artifacts about Open-Sleigh's own operation.** `ObservationsBus` is the only legal telemetry sink, and it has zero compile-time path to `Haft.Client` (OB1/OB5). Don't write code that would let the agent produce Haft artifacts describing Open-Sleigh itself. This is the Thai-disaster core prevention.
- **Do not commit `.env*` files** (gitignore enforces; but don't stage them even once).
- **Do not operate on Open-Sleigh's own source** as an agent workspace target. `PathGuard` enforces, but don't construct tests or mocks that work around it.

---

## 5. Workflow per task

For each task:

1. **Read the referenced specs.** Every spec section named in the Specs line of the task. Do not skip.
2. **Read the existing code** for the files you'll touch. Understand what's there before you change it.
3. **Sketch the approach** in 3-5 sentences in your scratch (not committed). Identify the test that will prove the change works. If you can't name the test, you don't understand the task yet.
4. **Write the failing test first** (TDD-ish). It should fail in a way that names the thing you're about to implement.
5. **Implement the smallest change** that makes the test pass without breaking the 290 existing tests.
6. **Run all four green-gate commands.** Fix any failure.
7. **Run the green-gate commands AGAIN** — some failures only surface after fixes to other issues.
8. **Commit** with a tight message. One task = one commit (or a tight series).

If a task's acceptance criteria reveal a spec ambiguity:

1. STOP coding.
2. Post a concrete question: "Spec X says A; task says B; how should C be resolved?"
3. Wait for the principal's answer.
4. Record the answer inline in `OPEN_QUESTIONS.md` with a resolution date.
5. Resume.

**Do not silently resolve ambiguity.** That's how Description ≠ Reality drift starts.

---

## 6. Verification

### 6.1 Per-change (local)

```
mix format --check-formatted
mix compile --warnings-as-errors
mix test
mix credo --strict
```

All four MUST be clean. `mix test --cover` also runs; aim for > 70% coverage on new code.

### 6.2 Integration tests (ad-hoc)

Most new features land with integration tests tagged `:integration` that skip by default:

```
mix test --include integration
```

Set credentials via env:

```
CODEX_CMD=/path/to/codex LINEAR_API_KEY=lin_api_xxx mix test --include integration
```

### 6.3 Canary dry-run

After each task that touches the runtime pipeline, run:

```
mix open_sleigh.canary --duration=2m
```

This exercises the T1/T1'/T2/T3 canary suite with mocks. Must exit 0.

### 6.4 Before a PR

Full gate:

```
mix format --check-formatted \
  && mix compile --warnings-as-errors \
  && mix test --include integration \
  && mix credo --strict \
  && mix dialyzer
```

(Dialyzer PLT builds slow the first time; rebuild with `mix dialyzer.plt`.)

---

## 7. Module map — quick reference

| Module | Layer | Purpose |
|--------|-------|---------|
| `OpenSleigh.{Phase, Verdict, GateKind, AuthoringRole, RunAttemptSubState}` | L1 | Closed sum types |
| `OpenSleigh.{ConfigHash, SessionId, ProblemCardRef, SessionScopedArtifactId}` | L1 | Opaque ids |
| `OpenSleigh.{Evidence, HumanGateApproval, GateResult, Workflow, Ticket, AdapterSession, PhaseConfig, Session, PhaseOutcome}` | L1 | Structs with typed constructors |
| `OpenSleigh.Gates.Registry` | L2 | Atom → module table for gate names |
| `OpenSleigh.Gates.{Structural, Semantic, Human}` | L2 | Behaviours |
| `OpenSleigh.Gates.Structural.*` | L2 | 5 structural impls |
| `OpenSleigh.Gates.Semantic.*` | L2 | 3 semantic impls (contract-only) |
| `OpenSleigh.Gates.Human.CommissionApproved` | L2 | The MVP-1 human gate |
| `OpenSleigh.{GateChain, GateContext}` | L2 | Chain dispatcher + input bundle |
| `OpenSleigh.PhaseMachine` | L3 | `next/2` pure transition |
| `OpenSleigh.WorkflowState, NextDecision` | L3 | Runtime state + decision sum |
| `OpenSleigh.EffectError` | L4 | Closed sum of all expected adapter errors |
| `OpenSleigh.Adapter.PathGuard` | L4 | Canonical path resolution |
| `OpenSleigh.Agent.{Adapter, Protocol}` | L4 | Behaviour + JSON-RPC codec |
| `OpenSleigh.Agent.Mock` | L4 | In-memory adapter for tests |
| `OpenSleigh.Tracker.{Adapter, Mock}` | L4 | Behaviour + in-memory impl |
| `OpenSleigh.Haft.{Protocol, Client, Mock}` | L4 | MCP codec + stateless typed API + mock |
| `OpenSleigh.JudgeClient` | L4 | Wraps Agent.Adapter for semantic gates |
| `OpenSleigh.ObservationsBus` | L5 | ETS telemetry sink (OB1/OB5) |
| `OpenSleigh.WorkspaceManager` | L5 | Workspace creation + hook execution |
| `OpenSleigh.{HaftServer, HaftSupervisor}` | L5 | GenServer wrapping invoke_fun |
| `OpenSleigh.WorkflowStore` | L5 | Holds compiled SleighConfig |
| `OpenSleigh.AgentWorker` | L5 | Per-session Task (single-turn → Task 1 adds continuation) |
| `OpenSleigh.Orchestrator` | L5 | Single-writer GenServer |
| `OpenSleigh.TrackerPoller` | L5 | Periodic list_active |
| `OpenSleigh.Application` | L5 | Supervision tree root |

Tests live in `test/open_sleigh/` mirroring the `lib/` structure; `test/support/fixtures.ex` provides `OpenSleigh.Fixtures` for test-data construction.

---

## 8. If you get stuck

1. Re-read the failing test's error message. If it says `{:error, :atom}` — that atom is in the corresponding `ILLEGAL_STATES.md` row; the row's Enforcement column tells you where the check lives.
2. Re-read the spec document named in the task.
3. Check the `test/support/fixtures.ex` helpers — you're probably building struct literals manually where a helper exists.
4. Check `canary_happy_path_test.exs` — it's the live reference for how L5 pieces wire up.
5. If you're trying to add a new dep and can't justify it in one sentence, you don't need it.
6. If the task fights the layer discipline, the design you're choosing is wrong. Go back and rethink.
7. If after all this you're still stuck — STOP and ask. A 200-word question from you is cheaper than a 2000-line PR that needs to be reverted.

---

## 9. Done criteria for MVP-1 canary green

All ten tasks above complete. Plus:

1. `mix open_sleigh.canary` green for 24h continuous on `sleigh-canary` repo.
2. One real ticket on `octacore_nova` completed end-to-end: Frame → Execute → HumanGate approved → Measure → tracker → Done.
3. Evidence pack (ProblemCard ref + ADR + Measure evidence with external carriers) written to Haft.
4. `MEASUREMENT.md` summarising what actually ran.
5. `human_override_count == 0` and `reopen_after_measure_rate == 0` on the single real ticket (trivially so for n=1).

That's the definition of "MVP-1 complete." Do not ship public OSS before these five items.

---

**Version.** This handoff document, v0.1 (2026-04-22), corresponds to
repo state at the "L5 skeleton green" checkpoint. If major
revisions happen before you finish, ask for a refresh.
