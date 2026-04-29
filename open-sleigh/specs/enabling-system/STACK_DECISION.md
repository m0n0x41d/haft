---
title: "Open-Sleigh: Stack Decision"
version: v0.2 (post-5.4-Pro first review)
date: 2026-04-22
status: revised with P5 guardrail hardening — proxy-module rule, filesystem I/O discipline
valid_until: 2026-05-20
---

# Open-Sleigh: Stack Decision

## Chosen Stack

| Layer | Choice | Rationale |
|---|---|---|
| **Language** | Elixir (OTP 26+) | BEAM supervision trees match the single-writer / message-passing discipline the architecture requires. Pattern matching + sum types via tagged tuples express the L1 algebraic domain cleanly. Hot reload matches the `sleigh.md` hot-reload model (`../target-system/SLEIGH_CONFIG.md §2`). |
| **Concurrency / Supervision** | Plain OTP (`GenServer`, `Task.Supervisor`, `Supervisor`) | No additional library. OTP is the canonical tool for exactly this shape (one-sole-writer + many-workers + failure isolation). |
| **Core data types** | Elixir structs + `@enforce_keys` + constructor-only module API | Structs are the closest Elixir has to nominal types. Constructor-only API gives us the "no default constructor" discipline from FUNCTIONAL_ARCHITECTURE L1. |
| **Type checking** | Dialyzer with `@spec` annotations everywhere at layer boundaries | Elixir's gradual typing. Dialyzer catches mismatches at L2/L3/L4 boundaries. Not Haskell-strong but enough for the layer-boundary invariants this architecture cares about. |
| **MCP client** | Hand-rolled JSON-RPC 2.0 over stdio (no library) | `haft serve` is the only MCP server we speak to; the protocol surface is tiny (6 tools); a 200-line module is more maintainable than depending on a generic MCP library. Pin `haft` version in `mix.exs`. |
| **Tracker adapter** | Thin HTTP client per adapter impl (`Linear` first) | Use `Finch` for HTTP (lightweight, pooled, no implicit behavior). Avoid SDK wrappers — they add coupling to library release cadence. |
| **Agent adapter** | JSON-RPC over stdio (Codex CLI, Claude Code) via `Port` | BEAM `Port` handles the subprocess stdio natively. One `Port`-wrapping process per adapter session. No extra library. |
| **Storage (MVP-1)** | In-RAM orchestrator state + per-ticket WAL as JSON-Lines files | Per `../target-system/RISKS.md §2` accepted risk. Haft SQLite is the persistent source of truth for evidence; Orchestrator state rebuilds from tracker + WAL on restart. |
| **Storage (MVP-2+)** | Optional SQLite via `Exqlite` for Orchestrator crash recovery | Only if measured evidence shows tracker-rebuild latency is the bottleneck. Default stays in-RAM. |
| **Configuration parser** | `YamlElixir` (YAML front matter) + `EarmarkParser` (Markdown body) | Both are maintained, zero-coupling libraries. L6 compiler owns validation; parsers only produce a canonical AST. |
| **CLI** | `Mix` task (`mix open_sleigh.start`, `mix open_sleigh.canary`) | No separate escript for MVP-1. Operator's working directory is always a Mix project. |
| **Telemetry / Observations** | `:telemetry` library + ETS-backed observation bus (`ObservationsBus`) | `:telemetry` is the canonical BEAM event-dispatch library. `ObservationsBus` wraps it with ETS aggregates. No Datadog / Prometheus push in MVP-1. |
| **Tests** | `ExUnit` + `StreamData` (property-based for L1–L3) | Layer-boundary tests are the primary test mode. Property tests for total functions. Integration tests for L4 adapters against mocked and real services. |
| **Deployment (MVP-1)** | `mix release` + systemd on the operator's machine | No containers. Single-principal single-operator system. |
| **Deployment (MVP-2+)** | Optional container for team-shared engine | Only if a team ever uses it. Single-operator today. |

---

## Day-1 Infrastructure

```
Operator laptop / server:
  systemd --user:
    open_sleigh.service     (one BEAM node running the engine)

  ~/.open-sleigh/:
    wal/                    (per-ticket JSON-Lines)
    sleigh.md               (the operator DSL)
    haft.sqlite             (owned by `haft serve` child process)
```

One BEAM node. No Postgres. No Redis. No message broker.

Add Postgres only if MVP-2 introduces team-shared state (it won't for a
while). Add nothing else "pre-emptively."

---

## Why Elixir / OTP

Elixir provides everything the architecture demands — most of it in
the language itself, rest in OTP's standard library:

| Engineering Principle (from CLAUDE.md) | Elixir Implementation |
|---|---|
| Functional core / imperative shell | L1–L3 are pure Elixir modules (no GenServer). L4 is the only effectful layer. L5 is OTP processes — the shell. |
| Make illegal states unrepresentable | Structs + `@enforce_keys` + constructor-only module API + sum types via tagged tuples + pattern match exhaustiveness |
| Error handling via Result/Either | Conventional `{:ok, value} \| {:error, reason}` across every L4 call. No exceptions for expected failures. |
| Side effects only at outermost shell | L4 adapters are the only modules that do I/O. L1–L3 are pure. L5 orchestrates but doesn't itself do I/O (it delegates to L4). |
| Composition via pipe/compose | `\|>` is the native Elixir idiom. Pipelines are one step per line, chained by `\|>`. |
| Modules as abstraction barriers | `defmodule` with `@moduledoc` + explicit `@spec` on public functions + opaque types (`@opaque t :: ...`) hide internals. |
| No default parameters | Elixir style: pattern-matching function heads or explicit constructor functions. |
| Immutability | Elixir is immutable by default. |
| Minimize cyclomatic complexity | Multi-clause function heads + pattern matching replace `if`/`case` chains. |

### What Elixir gives us that a simpler stack wouldn't

- **Supervision trees** expressed as code, not config. `Supervisor.start_link/2` with child specs is the clearest articulation of "if X dies, restart Y under policy Z." No Docker healthcheck duct-tape needed.
- **Hot reload** matches `sleigh.md` hot-reload semantics naturally. `Code.compile_file/1` applied to new compiled config doesn't restart OTP processes; they see the new config on next `WorkflowStore.get/0`.
- **Ports** for subprocess stdio — first-class BEAM primitive. Codex CLI / Claude Code / haft serve all ride the same mechanism.
- **Processes as types-of-failure-domain** — one `AgentWorker` per session dies without taking the Orchestrator down. Canonical OTP.

---

## Why Not Node.js / TypeScript

Considered because octacore_nova uses TypeScript + Effect-TS. Rejected
for Open-Sleigh because:

- **Octacore's workload is CRUD + financial arithmetic + adapters to
  accounting APIs.** That's well-served by Effect-TS + Drizzle.
- **Open-Sleigh's workload is supervision + typed state machine +
  subprocess orchestration.** That's better served by OTP.
- Effect-TS's equivalent of OTP supervision would require substantial
  custom infrastructure (process lifecycles, subprocess stdio, hot
  reload). OTP gives it for free.
- TypeScript's ad-hoc sum types via discriminated unions are fine, but
  pattern-match exhaustiveness is less ergonomic than Elixir's.

Compounding reason: a pure-product team may benefit from one stack
everywhere. Open-Sleigh is enabling-system code for Ivan's
engineering workflow; it doesn't ship to octacore customers; using
a different stack is a free choice.

---

## Why Not Rust / Go

- **Rust:** Capabilities match (strong types, sum types via enums). Cost
  is higher: async runtime choice, supervision has to be hand-rolled,
  hot reload isn't a thing, subprocess stdio is painful. For a system
  whose defining feature is "supervise external processes and their
  typed failure modes," Rust is more punishment than payoff.
- **Go:** Expressiveness ceiling is low. No sum types. Error handling
  is verbose. Goroutines + channels solve a different problem than
  OTP supervisors do.

---

## Why Not Python

Dynamic typing is a risk for a type-driven architecture where the
illegal-states catalogue requires 58 invariants to be type-enforced.
Python would push every one of them to runtime checks, which is
exactly the discipline-vs-type-system trade the architecture refuses.

Also: the harness orchestrates agents written in other languages
(Codex CLI is Rust binary; Claude Code is Node CLI). The harness
doesn't need Python-native libraries. BEAM + Ports is strictly better
for this shape.

---

## Dependency policy

Explicit dependencies for MVP-1 (via `mix.exs`):

| Dep | Purpose | Why this one |
|---|---|---|
| `finch` | HTTP client (Tracker adapters) | Pooled, well-maintained, no surprise behavior |
| `jason` | JSON encode/decode | Canonical fast JSON lib for Elixir |
| `yaml_elixir` | YAML parsing (front matter of `sleigh.md`) | Mature, pure-Elixir |
| `earmark_parser` | Markdown parsing (body of `sleigh.md`) | AST-only; we do our own rendering |
| `telemetry` | Metrics/events | Canonical BEAM telemetry library |
| `stream_data` | Property testing | Part of ExUnit in practice |

**No other runtime dependencies for MVP-1.** Any addition requires an
ADR justifying why a 200-line implementation won't do. The
Thai-disaster lesson: dep count is a meta-complexity leading indicator.

Explicitly NOT depending on:

- **Ecto** — no persistent relational state in MVP-1. If MVP-2.1 adds
  SQLite, `Exqlite` direct, not Ecto.
- **Phoenix** — no web surface in MVP-1. LiveView dashboard in MVP-2
  is a separate app, not a Phoenix-first architecture.
- **Broadway / Oban** — no job-queue semantics. Orchestrator claims,
  Worker runs, outcome reported via message. One-shot work per
  AgentWorker; no queue.
- **Horde / libcluster** — single-node in MVP-1. Distribution is
  MVP-2.1+.

---

## Boundary enforcement at stack level

- **Module dependency checks:** `mix xref graph --fail-above 0` in CI
  asserts zero path from `OpenSleigh.ObservationsBus` to
  `OpenSleigh.Haft.Client` (`ILLEGAL_STATES.md` OB1).
- **Proxy-module rule (v0.5 P5 hardening).** Custom Credo rule
  `OpenSleigh.Credo.NoObservationToHaftProxy` walks each module's
  `import` / `alias` / `use` headers; any module that references BOTH
  `OpenSleigh.ObservationsBus` AND `OpenSleigh.Haft.Client` fails CI.
  This is stricter than `mix xref` (which only catches transitive call
  paths) because it forbids even holding the capability to bridge them
  in the same module. See `ILLEGAL_STATES.md` OB5.
- **Filesystem I/O discipline.** Credo rule
  `OpenSleigh.Credo.NoDirectFilesystemIO` requires all L4 adapter
  filesystem operations to go through `OpenSleigh.Adapter.PathGuard`.
  Direct `File.write!/1` in adapter code fails CI (`ILLEGAL_STATES.md`
  CL11).
- **Immutable compiled config.** Credo rule
  `OpenSleigh.Credo.ImmutableCompiledConfig` forbids struct-update syntax
  on `SleighConfig`; only `Sleigh.Compiler.compile/1` produces instances
  (CF8).
- **No direct struct literals for L1 types.** Credo rule
  `OpenSleigh.Credo.NoDirectStructLiteral` forbids `%PhaseOutcome{...}`,
  `%Evidence{...}`, `%HumanGateApproval{...}` literals outside their
  owning module. Construction must go through `new/_`. This backstops
  constructor-level invariants (PR1–PR10) against the direct-literal
  bypass.
- **Human-role ownership.** Credo rule
  `OpenSleigh.Credo.HumanRoleOwnership` ensures `HumanGateApproval`
  construction only happens inside `OpenSleigh.HumanGateListener`; no
  agent-adapter code path can produce `:human`-authored artifacts
  (UP2).
- **Compiler warnings as errors:** `ElixirLS` / `mix compile
  --warnings-as-errors` in CI; Dialyzer in CI.
- **Credo strict-mode** bans: default function parameters where
  avoidable, nested case statements, functions with CC > 5, and the
  custom rules above.

---

## Build / CI

- `mix format` + `mix credo --strict` + `mix dialyzer` + `mix test` +
  `mix xref graph --fail-above 0` all gate PRs.
- `mix open_sleigh.canary` runs the full canary suite. Must be green
  24h before any change reaches `octacore_nova`.
- GitHub Actions workflow; no self-hosted runners in MVP-1.

---

## What this stack explicitly refuses

- No ORM. Data types are defined in L1, not in schema files.
- **No web framework. No load-bearing HTTP surface.** Orchestrator
  correctness is fully independent of any HTTP server. The optional
  minimal HTTP observability endpoint defined in
  `../target-system/HTTP_API.md` is admitted at L6 — it is
  **read-only**, observability-only, never exposes Haft artifacts, and
  the engine runs correctly whether it is enabled or not. (Refined in
  v0.6.1 to eliminate the earlier "no HTTP server in the engine"
  overclaim that conflicted with the HTTP_API spec.) Tracker adapters
  speak HTTP outbound only — that is client behaviour, not a server
  surface.
- No dependency injection library. OTP supervision trees + module-function references are the DI mechanism.
- No configuration library (like `Vapor` / `Skogsrå`). `sleigh.md` is the config; `Sleigh.Compiler` is the parser.
- No feature-flag system. Feature flags are a refactoring anti-pattern for a small codebase.
- No telemetry-export middleware for MVP-1. `ObservationsBus` is local ETS; export is a separate adapter if ever needed.

---

## Revisit triggers

Trigger a STACK_DECISION refresh if:

- MVP-1 doesn't reach canary-green in 3 weeks — may indicate stack friction; reconsider specific choices (likely `mix xref` or Dialyzer tooling).
- MVP-2 introduces a web UI — Phoenix LiveView revisit required.
- A second principal joins — team-shared state may require Postgres + Ecto; revisit persistence choice.
- Hot-reload proves unreliable in production — WAL replay model may need hardening; no stack change, but operational practices change.

This document's `valid_until` is 2026-05-20 by default; earlier refresh if any trigger fires.
