# Stack Decision

> Technology choices and rationale.

## Core Runtime

| Layer | Technology | Why | Alternatives considered |
|-------|-----------|-----|----------------------|
| **Language (backend)** | Go 1.25 | Fast compilation, single binary, excellent concurrency, stdlib quality (go/parser for module detection). AI agents generate good Go. | Rust (too slow to iterate solo), TypeScript (can't do single binary easily) |
| **Language (desktop frontend)** | TypeScript + React | Proven UI framework, AI agents generate excellent React. shadcn/ui for components. | Svelte (smaller ecosystem), vanilla (too slow to build) |
| **Language (TUI)** | TypeScript + Ink | React-like model for terminal UI. Consistent mental model with desktop frontend. | Bubbletea/Go (was v5.3 choice, deprecated — Go TUI libs are less productive than Ink) |
| **Desktop framework** | Tauri v2 | Native WebView shell with a Rust command boundary and React frontend. Keeps Desktop as a surface over the Haft CLI/Core instead of a second semantic kernel. | Wails v2 (retired after IPC/runtime mismatch), Electron (too heavy) |
| **Database** | SQLite (modernc.org/sqlite, pure Go) | Zero dependencies, local-first, per-project isolation. WAL mode + busy_timeout for concurrency. | PostgreSQL (overkill for local-first, server mode deferred) |
| **FPF search** | FTS5 (SQLite built-in) | Full-text search with ranking. Embedded, no external service. Route-aware tiered retrieval. | Tantivy (Rust, FFI overhead), Bleve (Go, but heavier than FTS5) |
| **MCP protocol** | JSON-RPC over stdin/stdout | MCP standard. v7 product support targets Claude Code and Codex; Cursor/Gemini/Air remain experimental or legacy carriers. | HTTP (more overhead, less standard for MCP) |

## Code Analysis

| Component | Technology | Why |
|-----------|-----------|-----|
| **Go module detection** | `go/parser` (stdlib) | 100% accuracy, no regex, no external dependency |
| **JS/TS/Python/Rust module detection** | Regex + filesystem heuristics | Good enough for dependency graph. Stdlib sufficient. |
| **C/C++ module detection** | `compile_commands.json` + fallback heuristics | Industry standard for C/C++ build info |
| **Symbol hashing** | tree-sitter (go-tree-sitter) | AST-level symbol extraction for semantic drift detection beyond file hashes |
| **Import parsing** | Per-language: stdlib (Go), regex (others) | Go stdlib is perfect; regex is pragmatic for others |

## Build & Distribution

| Component | Technology | Why |
|-----------|-----------|-----|
| **Release** | goreleaser v2 | Cross-platform binary builds, GitHub releases, checksums |
| **CI** | GitHub Actions | Standard, free for open source |
| **Lint** | golangci-lint v2 | Comprehensive Go linting |
| **Desktop build** | Tauri CLI (`cargo tauri build`) | Native app packaging for the Rust shell plus React frontend |

## Harness Batch Execution

Decision record: `dec-20260428-harness-drain-v3-16bf21f3`.

| Component | Technology / policy | Why | Alternatives considered |
|-----------|---------------------|-----|-------------------------|
| **Batch drainer owner** | Open-Sleigh durable intake loop, reached through `haft harness run --drain --concurrency N` | Keeps batch execution in the commission harness that already owns queue intake, lease observation, and agent concurrency. Drain mode is opt-in, so existing single-commission `haft harness run` behavior remains unchanged. | One-shot claimed-batch execution (cannot run unattended until the runnable queue is empty); ad hoc shell loop around `haft harness run` (cannot reliably surface typed skip reasons or clean shutdown semantics). |
| **Parallel claim safety** | Existing lockset-conflict rejection at claim time | Preserves the current invariant that overlapping WorkCommissions do not execute concurrently against the same files. Drain mode increases throughput only across non-overlapping commissions. | Relax lockset checks under drain mode (rejected: raises silent corruption risk); serialize all commissions (safe but defeats the batch throughput target). |
| **Auto-apply policy** | `workspace_patch_auto_on_pass` applies only when verdict is `pass` and AutonomyEnvelope re-evaluation returns `allowed` | Makes the green path low-touch while keeping the envelope as the authoritative autonomy boundary. Each apply remains a discrete, revertable local git operation. | Always manual apply (too much operator burden for overnight batches); batch squash apply (rejected: weak reversibility); remote push/PR automation (out of scope for local-only drainer). |
| **Manual fallback policy** | `workspace_patch_manual` remains the default delivery policy | Keeps autonomy explicit per decision. Commissions that fail, require a checkpoint, or use the manual policy wait for operator `haft harness apply`, `haft harness requeue`, or `haft harness cancel`. | Make auto-apply the default (rejected: changes the autonomy budget silently). |
| **Stale lease policy** | Intake skips leases claimed longer than 24h by default with typed reason `lease_too_old` | Prevents silent revival of old claimed work while leaving an operator-visible recovery path. The age cap is an intake policy, not proof that the commission is invalid. | Resume any unexpired lease (audit gap); require manual cleanup for every claimed lease (too noisy for unattended batches). |
| **External effects** | Local filesystem and local git only | Matches the commission projection policy and keeps the drainer out of remote authority: no push, no PR creation, no comments, no webhooks. | Merge agent or remote delivery automation (deferred until a later explicit autonomy decision). |

## What We Don't Use (and why)

| Technology | Why not |
|-----------|---------|
| **Docker** | Local-first product. Single binary is simpler than container. |
| **PostgreSQL** | Deferred to server mode. SQLite sufficient for solo/team-via-git. |
| **Redis** | No caching layer needed. SQLite is fast enough. |
| **gRPC** | MCP uses JSON-RPC. No need for two protocols. |
| **LangChain/LangGraph** | Too heavy for our needs. Direct LLM API calls via provider abstraction. |
| **Vector database** | FTS5 + route-aware retrieval sufficient for FPF search. Semantic search experimental only. |
| **Kubernetes** | Not a server product. Single binary on developer machine. |
