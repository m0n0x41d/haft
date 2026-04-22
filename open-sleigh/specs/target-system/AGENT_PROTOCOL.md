---
title: "8. Agent Protocol Contract"
description: Normative protocol contract between Agent.Adapter implementations and the engine. JSON-RPC over stdio; session/thread/turn lifecycle; event taxonomy; error categories; continuation-turn semantics; stall detection; token accounting.
reading_order: 8
---

# Open-Sleigh: Agent Protocol Contract

> **FPF note.** This document is a `Description` of the protocol between
> Open-Sleigh (the Object) and an AI coding agent subprocess (external
> system). A conforming `Agent.Adapter` module is the `Carrier` that
> implements this description. `Codex` (MVP-1) and `Claude` (MVP-1.5)
> are both Carriers of the same Description.
>
> **Influence, not inheritance.** This contract is adapted from
> OpenAI Symphony `SPEC.md` §10 (see `../../.context/symphony/SPEC.md`).
> We reuse Symphony's JSON-RPC app-server shape because it is a sound
> pattern for subprocess-based agent orchestration. We diverge on three
> points:
>
> 1. **Phase-scoped tool dispatch.** Symphony exposes a tool set per
>    session; we scope tools per **phase** within a session (Frame has
>    different scope than Execute; Q-OS-4 hybrid mechanism).
> 2. **No agent-owned tracker mutation.** Symphony lets the agent call
>    Linear directly via `linear_graphql` tool. Open-Sleigh routes all
>    tracker writes through `Tracker.Adapter` at L4 — the agent
>    proposes transitions as part of `PhaseOutcome`, the Orchestrator
>    effects them (with HumanGate where required).
> 3. **Continuation turns respect phase boundaries.** Symphony runs N
>    turns per issue in one session. We run N turns per `(Ticket ×
>    Phase)` session, and a phase exit closes the thread. A new phase
>    starts a new thread.

## 1. Transport

- **Protocol:** line-delimited JSON-RPC 2.0 over `stdio`.
- **Launch:** Elixir `Port` with `spawn: "bash -lc <agent.command>"`.
  `cwd` is the `AdapterSession.workspace_path` (the downstream repo),
  NEVER the Open-Sleigh tree. Enforced by `PathGuard` (L4).
- **Framing:** one JSON value per line on `stdout`. Max line size: 10 MB
  (safety buffer per Symphony §10.1).
- **`stderr`:** diagnostic logs only. Never parsed as protocol JSON.
- **Timeouts:** three distinct timeouts per session —
  - `read_timeout_ms` — request/response round-trip, default 5000 ms.
  - `turn_timeout_ms` — total time from `turn/start` to terminal event, default 3_600_000 ms (1 h).
  - `stall_timeout_ms` — inactivity between any two events, default 300_000 ms (5 min); `<= 0` disables.

## 2. Handshake (session startup)

On `AgentWorker` spawn, the adapter sends these messages in order and
awaits responses. Failure at any step produces
`{:error, EffectError.t()}` and the session is aborted.

```
# 1. initialize
{"jsonrpc": "2.0", "id": 1, "method": "initialize",
 "params": {"clientInfo": {"name": "open-sleigh", "version": "0.1"},
            "capabilities": {}}}

# 2. initialized (notification — no response)
{"jsonrpc": "2.0", "method": "initialized", "params": {}}

# 3. thread/start — opens a thread for this (Ticket × Phase) session
{"jsonrpc": "2.0", "id": 2, "method": "thread/start",
 "params": {"approvalPolicy": "<per sleigh.md codex.approval_policy>",
            "sandbox": "<per sleigh.md codex.thread_sandbox>",
            "cwd": "<absolute workspace_path>",
            "tools": ["<scoped tool names for this phase>"]}}
# → response: {"result": {"thread": {"id": "<thread_id>"}}}

# 4. turn/start — first turn with the full rendered prompt
{"jsonrpc": "2.0", "id": 3, "method": "turn/start",
 "params": {"threadId": "<thread_id>",
            "input": [{"type": "text", "text": "<rendered prompt>"}],
            "cwd": "<absolute workspace_path>",
            "title": "<Ticket.identifier>: <Ticket.title>",
            "approvalPolicy": "<codex.approval_policy>",
            "sandboxPolicy": {"type": "<codex.turn_sandbox_policy>"}}}
# → response: {"result": {"turn": {"id": "<turn_id>"}}}
```

**Session identity.**
- `session_id = "<thread_id>-<turn_id>"` (format per Symphony §10.2).
- Same `thread_id` across all turns of one `(Ticket × Phase)` session.
- New `turn_id` per turn.
- The `config_hash` from `AdapterSession` is included as a trailer line
  in the rendered prompt: `<!-- config_hash: <hex> -->` (§8.1 of SPEC).

## 3. Continuation turns within a phase session

A single `(Ticket × Phase)` session may run **multiple turns** on the
same live thread. The pattern:

1. **Turn 1** sends the full rendered prompt for the phase.
2. After each `turn/completed` event, the `AgentWorker`:
   a. Re-fetches the tracker state (via `Tracker.Adapter`).
   b. Asks: "is the ticket still in an active state AND is the phase
      still incomplete (gates not yet all green)?"
   c. If both yes AND `turn_count < agent.max_turns`: fire another
      `turn/start` on the same thread with **continuation guidance**
      (not the original prompt).
   d. If either no: close the session normally (Section 5).
3. **Continuation guidance** is a short, structured text that does not
   repeat the full task prompt (since the prompt is already in thread
   history):

   ```
   Continuation guidance — Open-Sleigh Phase: <phase>

   - The previous turn completed, but the phase exit gates have not all passed yet.
   - This is continuation turn #<N> of <max_turns> for this (Ticket × Phase) session.
   - Resume from the current workspace and conversation state.
   - The original task prompt is already present in this thread — do not repeat it.
   - Gate failures that triggered this continuation (if any): <structured list>.
   - Do not end the turn while the phase is still incomplete unless you are truly blocked.
   ```

4. **Scoped toolset stays constant within a session.** Phase changes
   require a new session (new thread); scoped tools do not mutate mid-
   thread.

**MVP-1 applicability.**

- **Frame phase:** single turn only. `max_turns = 1`. Frame is a
  verifier; there is nothing to "iterate on." If verification fails,
  the phase exits `Verdict.fail` and the ticket goes back to the
  human.
- **Execute phase:** multi-turn, bounded by `agent.max_turns` (default
  20). Continuation fires when tracker is still active and gates not
  yet green.
- **Measure phase:** single turn. Evidence assembly is deterministic;
  if the measure agent can't assemble external evidence on the first
  turn, that's a failure, not an iteration opportunity.

## 4. Streaming events (agent → orchestrator)

Every event arrives as a JSON-RPC notification on `stdout`. The adapter
translates them to typed `AgentEvent.t()` values and forwards to
`AgentWorker` via message. Required event categories:

| Event | When fires | Required fields |
|---|---|---|
| `session_started` | After `thread/start` response | `thread_id`, `codex_app_server_pid`, `timestamp` |
| `turn_started` | After `turn/start` response | `turn_id`, `timestamp` |
| `turn_completed` | Turn finished normally | `turn_id`, `timestamp`, `usage` |
| `turn_failed` | Turn terminated with error | `turn_id`, `reason`, `timestamp` |
| `turn_cancelled` | Turn cancelled externally | `turn_id`, `reason`, `timestamp` |
| `turn_input_required` | Agent asked for user input | `turn_id`, `prompt` — **hard failure in MVP-1** |
| `notification` | Info message (progress, status) | `message`, `timestamp` |
| `tool_call` | Agent invoked a tool | `tool`, `args`, `call_id`, `timestamp` |
| `tool_result` | Adapter returned tool result | `call_id`, `result` or `error`, `timestamp` |
| `usage` | Token-count update | `input_tokens`, `output_tokens`, `total_tokens`, `timestamp` |
| `rate_limit` | Rate-limit snapshot from provider | `provider_payload`, `timestamp` |
| `malformed` | Un-parseable line on stdout | `raw`, `timestamp` |

**Token accounting rules (from Symphony §13.5, adopted):**

- Prefer absolute thread totals when the agent emits them
  (`thread/tokenUsage/updated` or equivalent).
- Track deltas relative to `last_reported_*` fields to avoid
  double-counting when absolute totals are emitted repeatedly.
- Ignore delta-style `last_token_usage` for dashboard aggregates.
- Token counts feed `ObservationsBus` (L5); they are NOT written to
  Haft (OB1–OB5 isolation).

## 5. Session close

The adapter closes a session by terminating the `Port` after either:

- `turn_completed` + no continuation fires (phase gates all green, or
  tracker state no longer active, or `max_turns` reached);
- `turn_failed` / `turn_cancelled` → worker reports failure to
  `Orchestrator`, which schedules retry per Section 6;
- `turn_input_required` → immediate hard failure (MVP-1; never wait
  for human input inside a turn — that's what HumanGate is for at
  phase boundaries);
- timeout (turn / stall) → kill `Port`, report timeout.

Closing a session ends its thread. Re-claiming the same ticket for the
next phase opens a new thread in a new session.

## 6. Error taxonomy

All adapter errors map to typed `EffectError.t()` variants. The
closed sum:

```elixir
@type EffectError.t() ::
        :agent_command_not_found
      | :agent_launch_failed
      | :invalid_workspace_cwd
      | :handshake_timeout
      | :initialize_failed
      | :thread_start_failed
      | :turn_start_failed
      | :turn_timeout
      | :stall_timeout
      | :turn_input_required   # hard failure in MVP-1
      | :port_exit_unexpected
      | :response_parse_error
      | :tool_forbidden_by_phase_scope
      | :tool_unknown_to_adapter
      | :tool_arg_invalid
      | :tool_execution_failed
      | :haft_unavailable       # at a tool call bound to haft_*
      | :rate_limit_exceeded
      | :unsupported_event_category
```

Unknown error shapes are rejected at source. Adding a new error
variant requires extending the sum at the adapter module level (CI
enforced).

## 7. Tool dispatch within a turn

Agents request tool calls inside a turn via a protocol message (shape
varies per adapter; Symphony `item/tool/call` is the Codex shape). The
adapter:

1. Looks up `tool_name` in its `@tool_registry` module attribute.
   Unknown → respond with `{success: false, error: "tool_unknown_to_adapter"}`.
2. Checks `tool_name` ∈ `AdapterSession.scoped_tools` (MapSet). Not in
   scope → respond with `{success: false, error: "tool_forbidden_by_phase_scope"}`.
3. For filesystem-touching tools (`:read`, `:write`, `:edit`, `:bash`),
   routes through `PathGuard.canonical/1` (L4). Path violation →
   respond with `{success: false, error: "path_outside_workspace"}` (or
   other PathGuard reason).
4. For Haft tools (`:haft_problem`, `:haft_solution`, `:haft_decision`,
   `:haft_note`, `:haft_refresh`, `:haft_query`), routes through
   `Haft.Client` with the session's `config_hash` attached. Unavailable
   → respond with `{success: false, error: "haft_unavailable", retry_after: <ms>}`.
5. For tracker tools (if any are ever exposed — MVP-1 exposes none):
   NOT available to agents. Tracker mutation is orchestrator-owned per
   SPEC §9 "Not a coding agent of tracker mutation."
6. Returns the result inline; the turn continues.

**Tool approval is implementation-defined per `codex.approval_policy`**,
but for MVP-1 the recommended policy is `auto_approve_in_session`
(trusted environment; the walls are at the phase boundaries, not
inside a phase).

## 8. Stall detection

Every event resets `last_event_at`. `AgentWorker` monitors
`elapsed_ms_since_last_event`:

- If `elapsed_ms > stall_timeout_ms`, kill the `Port`, report
  `:stall_timeout` to `Orchestrator`, which:
  1. Writes compensating `haft_note(cancelled, :stall, partial_refs)`
     per `RISKS.md §3` (tracker-wins reconciliation + cancellation
     protocol) and `HAFT_CONTRACT.md §7`.
  2. Schedules exponential-backoff retry (Section 6 of this doc).
- If `stall_timeout_ms ≤ 0`, stall detection is disabled (used in
  tests where an agent may legitimately idle).
- Stall detection is orthogonal to turn timeout: the turn timeout
  fires even if events are arriving (total-time bound); stall fires
  when events stop (inactivity bound).

## 9. Retry / backoff

Retry policy per `RISKS.md §3` (tracker-wins reconciliation) + Symphony
§8.4 (exponential backoff), FPF-disciplined:

- **Continuation retry (after normal worker exit with gates passing):**
  1000 ms fixed delay. Re-checks whether the ticket wants the next
  phase or terminates.
- **Failure retry:** `delay = min(10_000 × 2^(attempt - 1), agent.max_retry_backoff_ms)` with
  `max_retry_backoff_ms` default 300_000 ms (5 min).
- **Stall retry:** same as failure retry.
- **Rate-limit retry:** use `retry_after` from `rate_limit` event when
  provided; else fallback to failure-retry formula.
- Retry attempt number is passed to the next turn's prompt template as
  the `attempt` variable so the prompt can include retry-specific
  guidance.

## 10. Conformance requirements

Every `Agent.Adapter` impl must:

1. Implement the JSON-RPC handshake (Section 2) in order.
2. Support continuation turns on the same thread (Section 3).
3. Emit all required event categories (Section 4) or explicit
   not-applicable (e.g., an adapter with no tool dispatch still emits
   `session_started`, `turn_started`, `turn_completed`, `usage`).
4. Map every failure to a typed `EffectError.t()` (Section 6).
5. Enforce phase-scoped tool dispatch (Section 7) — compile-time for
   unknown-to-adapter, runtime for out-of-phase-scope.
6. Enforce stall detection (Section 8).
7. Respect PathGuard for any filesystem-touching tool (Section 7.3).
8. NOT provide direct tracker-mutation tools (our boundary; see
   Section 7.5).

The **Agent Adapter Parity Plan** (`ADAPTER_PARITY.md`) ensures these
conformance requirements hold across Codex and Claude adapters before
MVP-1.5 ships.

## 11. Open question: dynamic tools via MCP in-adapter

Symphony supports dynamic tools via Codex's capability negotiation
(`item/tool/call` for arbitrary advertised tools). We have deferred
this to MVP-2 because MVP-1's scoped toolset is declarative in
`sleigh.md` and enumerated in each adapter's `@tool_registry`. A
future Q-OS-8 may revisit: "should `sleigh.md` be able to declare
dynamic MCP tools and have the adapter advertise them through
Codex/Claude capability negotiation?" Non-blocking for MVP-1.

---

## Cross-references

- `../../SPEC.md` §5 (MVP-1 phase narrative + pointers)
- `SCOPE_FREEZE.md` (MVP-1 / MVP-1.5 / MVP-2 scope tiers)
- `PHASE_ONTOLOGY.md` (5-axis phase ontology incl. run-attempt sub-states)
- `SLEIGH_CONFIG.md` (sleigh.md schema incl. `agent.*`, `codex.*`, `hooks` sections)
- `HAFT_CONTRACT.md` (routing of `haft_*` tool calls from §7 of this doc)
- `RISKS.md` (tracker-wins reconciliation + cancellation protocol)
- `../enabling-system/FUNCTIONAL_ARCHITECTURE.md` L4 (adapter
  boundary), L5 (Orchestrator)
- `../../.context/symphony/SPEC.md` §10 (source of the JSON-RPC shape)
- `ILLEGAL_STATES.md` CL category (tool-scope invariants), AD category
  (adapter / effect invariants)
- `TARGET_SYSTEM_MODEL.md` (Session / AdapterSession / AgentTurn)
