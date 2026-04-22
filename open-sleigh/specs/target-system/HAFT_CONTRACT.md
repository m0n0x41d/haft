---
title: "10. Haft Contract"
description: The MCP contract between Open-Sleigh and `haft serve` — transport, tool surface, SPOF failure mode, WAL replay, token-accounting isolation.
reading_order: 10
---

# Open-Sleigh: Haft Contract

> **FPF note.** Haft is an **external MCP server**, not a library. Open-
> Sleigh is a client. The contract is the MCP wire format and the
> enumerated tool set below — nothing else. This document is the
> `Description` of that contract. The `Carrier` is `OpenSleigh.Haft.Protocol`
> + `OpenSleigh.Haft.Client` (L4 stateless) plus `OpenSleigh.HaftSupervisor`
> + `OpenSleigh.HaftServer` (L5 process-owning). The `Object` is the
> running `haft serve` subprocess and the Haft SQLite it owns.

---

## 1. Transport

- **Protocol:** MCP JSON-RPC 2.0 over stdio.
- **Process model:** **one `haft serve` child process per engine
  instance** (per-engine, not per-ticket — resolved from v0.1 §11.2),
  managed by `OpenSleigh.Haft.Supervisor`.
- **Launch:** `bash -lc <haft.command>` per `sleigh.md`. Version pinned
  in `mix.exs` + `sleigh.md` and requires explicit upgrade.
- **No direct SQLite access ever.** Haft owns its SQLite; the MCP tool
  surface is the only read/write path.

## 2. Current tool surface

| Tool | Used by phase | Purpose |
|---|---|---|
| `haft_problem(frame/characterize)` | Frame (verifier-read only), Characterize (MVP-2) | Create ProblemCard, add characteristics. **Frame in MVP-1 does NOT call the `frame` verb** — framing is upstream-human-only per ILLEGAL_STATES UP1. Frame phase toolset excludes `haft_problem` entirely (CL3). |
| `haft_solution(explore/compare)` | Generate, Parity-run (MVP-2) | Register variants, Pareto compare |
| `haft_decision(decide/apply/measure/evidence)` | Select, Commission, Measure | Record decision contract, attach evidence |
| `haft_refresh(scan/drift)` | periodic, poll tick | Staleness check on active commission-linked artifacts |
| `haft_note` | any phase | Micro-decision capture; also used for compensating-note protocol (§7) |
| `haft_query(status/search/related/fpf)` | any phase | Dashboard, cross-project recall, FPF-spec retrieval |

**`haft check --json`** runs as an external CLI at Execute-gate — it's the
CI-style governance gate, not an MCP tool.

**Contract stability.** The Haft MCP tool surface is declared stable in
`quint-code/spec/integration/MCP_PROTOCOL.md`.

## 2a. Required commission-first surface

The current tool set is enough for tracker-first MVP-1, but not for the
Haft-first integration. Commission-first Open-Sleigh requires new Haft-owned
operations. Names are provisional until the Haft API is implemented; the
semantics are not provisional.

| Operation | Used by | Purpose |
|---|---|---|
| `haft_commission(list_runnable)` | CommissionPoller | Return WorkCommissions eligible for preflight under the selected plan/queue. |
| `haft_commission(claim_for_preflight)` | Orchestrator | Atomically lease one WorkCommission for this runner and return the signed commission snapshot, including Scope. Does not grant Execute. |
| `haft_commission(record_preflight)` | Preflight phase | Attach PreflightReport and deterministic check results. |
| `haft_commission(start_after_preflight)` | Orchestrator | Move WorkCommission to running only if Haft validates preflight, CommissionRevisionSnapshot equality, scope hash, base SHA, plan/envelope revisions, lease state, and policy gates. |
| `haft_commission(record_run_event)` | any phase | Append RuntimeRun/phase status without mutating DecisionRecord truth. |
| `haft_commission(complete_or_block)` | Measure/terminal | Mark commission completed/failed/blocked with evidence refs, or completed_with_projection_debt when execution evidence passes but required external publish did not sync. |
| `haft_projection(intent/draft/publish/observe)` | Projection engine | Optional ExternalProjection lifecycle; may call a ProjectionWriterAgent for wording but Haft owns facts. |

Hard rules:

- Open-Sleigh cannot create, approve, or refresh a WorkCommission on its own.
- `claim_for_preflight` grants only Preflight. Execute requires
  `start_after_preflight`.
- Scope is an authority boundary. Open-Sleigh must enforce the returned Scope
  before mutating files and must report terminal diff validation.
- External tracker publish failure does not make a RuntimeRun invalid. For
  `external_required`, Haft records ProjectionDebt and withholds external
  closed status until the debt resolves.
- Manual external tracker changes are returned to Haft as observed drift,
  never as direct WorkCommission state changes.

## 3. Haft SPOF failure mode

`haft serve` is a single external process. Open-Sleigh must degrade
predictably when it's unreachable.

- **Detection:** `Haft.Client` health-ping every 10s; 3 consecutive misses
  → `:haft_unavailable` state.
- **In-flight behaviour:** running `AgentWorker`s are allowed to finish
  their current turn, but any `haft_*` tool call from the agent returns a
  `{:error, :haft_unavailable, retry_after}` frame. The agent is instructed
  (via prompt contract) to stop and wait.
- **New-entry behaviour:** the Orchestrator refuses to dispatch new phases
  — WorkCommissions accumulate in the `pending` queue.
- **Local WAL.** Phase outcomes that could not be written to Haft are
  appended to `~/.open-sleigh/wal/` as JSON-L. In commission-first mode, WAL is
  per-commission: `wal/<commission_id>.jsonl`, append-only. Legacy
  tracker-first mode used `wal/<ticket_id>.jsonl`.
- **Replay ordering.** On reconnect, `Haft.Supervisor` replays
  **per-commission append order, commission-by-commission in arrival order**
  (FIFO by first-entry timestamp of each commission file). Within a
  commission, order is preserved strictly; across commissions, parallelism is
  allowed once each commission's first entry has replayed in arrival order.
  Replay completes before new dispatches are accepted.
- **Operator surface:** the status dashboard (MVP-1 terminal, MVP-2 web)
  shows `:haft_unavailable` state with retry count and last error.

## 4. Token-accounting isolation (hard boundary)

Cost visibility is load-bearing for any LLM-agent system; Symphony's
aggregation rules are adopted for the data shape. Open-Sleigh routes
token counts through `ObservationsBus` (L5), **NEVER** `Haft.Client`.
This is a hard OB guardrail (`ILLEGAL_STATES.md` OB1–OB5, TA1–TA3):
token counts are telemetry, not FPF evidence. Treating them as evidence
is the Goodhart trap — the engine would optimise for "tokens used"
instead of "ticket delivered."

Aggregation rules (adopted from Symphony §13.5, canonical in
`AGENT_PROTOCOL.md §4`):

- Prefer absolute thread totals (`thread/tokenUsage/updated` event).
- Track deltas against `last_reported_*` to avoid double-counting.
- Ignore delta-style `last_token_usage` for dashboard totals.
- Accumulate `codex_input_tokens`, `codex_output_tokens`,
  `codex_total_tokens` per session; aggregate across sessions on the
  `ObservationsBus`.
- Rate-limit snapshots (`rate_limit` event) are tracked as
  observations — last-seen only, not historical.

Observation indicators derived from token counts live in `GATES.md §5`
(`codex_total_tokens_per_ticket`, `codex_total_tokens_per_phase`,
`codex_rate_limit_triggered_count`). Thresholds are informative, not
gating. Anti-Goodhart discipline.

## 5. L4 / L5 ownership seam

Clarified in v0.6.1. `Haft.Protocol` and `Haft.Client` are **L4 stateless
modules** — a JSON-RPC codec and a typed functional API. Every function
takes a session handle (`AdapterSession.t()`) and dispatches through the
L5 owner. The GenServer / Port that owns the `haft serve` subprocess
state lives at L5 — typically `HaftSupervisor` + `HaftServer`. L4
functions take a handle to the L5 owner and dispatch messages. See
`../enabling-system/FUNCTIONAL_ARCHITECTURE.md §LAYER 4` for the
canonical layering rule.

## 6. Config-hash provenance on writes

Every write tool (`haft_problem`, `haft_solution`, `haft_decision`,
`haft_note`) includes the session's frozen `config_hash` as artifact
metadata (via a dedicated metadata field when the tool accepts it, or
as an attached `haft_note` otherwise). See `SLEIGH_CONFIG.md §2` for
the hash formula and freeze semantics.

## 7. Cancellation protocol (partial-state discipline)

A `haft_*` call in flight when a ticket is reconciled out of Open-
Sleigh's claim (see `RISKS.md §3`) MUST either complete within the 30s
grace window or time out at `min(remaining_grace, 10s)`. It is never
silently abandoned. On cancel the `AgentWorker` writes
`haft_note(kind="cancelled", cause=<reason>, partial_refs=[...])`
before exiting, where `partial_refs` lists any `haft_*` artifacts that
were created but not finalized. After 30s, if the worker hasn't exited
cleanly, the `Orchestrator` writes the compensating note on the
worker's behalf using the last-known state snapshot. This turns silent
discard into auditable partial-state.

---

## See also

- [AGENT_PROTOCOL.md](AGENT_PROTOCOL.md) — §4 token accounting rules; §7 tool dispatch routing for `haft_*` tools
- [ILLEGAL_STATES.md](ILLEGAL_STATES.md) — OB1–OB5 (observation isolation), TA1–TA3 (token accounting), AD3 (no silent drop on unavailable)
- [TARGET_SYSTEM_MODEL.md](TARGET_SYSTEM_MODEL.md) — `Evidence`, `PhaseOutcome` (what gets written to Haft), `AdapterSession` (what the session handle carries)
- [../enabling-system/FUNCTIONAL_ARCHITECTURE.md](../enabling-system/FUNCTIONAL_ARCHITECTURE.md) — L4 `Haft.Client` stateless API + L5 `HaftSupervisor` / `HaftServer` process ownership
- [RISKS.md](RISKS.md) — Haft-wins reconciliation that invokes the cancellation protocol
