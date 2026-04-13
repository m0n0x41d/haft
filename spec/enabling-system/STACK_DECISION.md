# Stack Decision

> Technology choices and rationale.

## Core Runtime

| Layer | Technology | Why | Alternatives considered |
|-------|-----------|-----|----------------------|
| **Language (backend)** | Go 1.25 | Fast compilation, single binary, excellent concurrency, stdlib quality (go/parser for module detection). AI agents generate good Go. | Rust (too slow to iterate solo), TypeScript (can't do single binary easily) |
| **Language (desktop frontend)** | TypeScript + React | Proven UI framework, AI agents generate excellent React. shadcn/ui for components. | Svelte (smaller ecosystem), vanilla (too slow to build) |
| **Language (TUI)** | TypeScript + Ink | React-like model for terminal UI. Consistent mental model with desktop frontend. | Bubbletea/Go (was v5.3 choice, deprecated — Go TUI libs are less productive than Ink) |
| **Desktop framework** | Wails v2 | Go backend + native WebView. Single binary. No Electron bloat. Same Go code serves MCP, CLI, and desktop. | Tauri (Rust, doesn't match our Go backend), Electron (too heavy) |
| **Database** | SQLite (modernc.org/sqlite, pure Go) | Zero dependencies, local-first, per-project isolation. WAL mode + busy_timeout for concurrency. | PostgreSQL (overkill for local-first, server mode deferred) |
| **FPF search** | FTS5 (SQLite built-in) | Full-text search with ranking. Embedded, no external service. Route-aware tiered retrieval. | Tantivy (Rust, FFI overhead), Bleve (Go, but heavier than FTS5) |
| **MCP protocol** | JSON-RPC over stdin/stdout | MCP standard. All host agents (Claude Code, Codex, Cursor) speak it. | HTTP (more overhead, less standard for MCP) |

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
| **Desktop build** | Wails CLI (`wails build`) | Integrated with Go build, produces native binary |

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
