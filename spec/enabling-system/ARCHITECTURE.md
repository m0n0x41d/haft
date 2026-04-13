# Functional Architecture

> Four modules, layered. Each depends only on the layer below.

## Module Hierarchy

```
┌─────────────────────────────────────────────┐
│               SURFACES                       │
│  Desktop (Wails)  │  MCP Plugin  │  CLI     │
└────────┬──────────┴──────┬───────┴────┬─────┘
         │                 │            │
         └────────┬────────┴────────┬───┘
                  ▼                 ▼
┌─────────────────────────────────────────────┐
│              GOVERNOR                        │
│  Background scanner, drift, stale refresh,  │
│  invariant verification, problem factory     │
└────────────────────┬────────────────────────┘
                     ▼
┌─────────────────────────────────────────────┐
│                FLOW                          │
│  Task runner, worktree lifecycle, agent      │
│  spawning, invariant injection, verification │
└────────────────────┬────────────────────────┘
                     ▼
┌─────────────────────────────────────────────┐
│                CORE                          │
│                                             │
│  Artifact Graph  │  Knowledge  │  Codebase  │
│  (problems,      │  Graph      │  Analysis  │
│   portfolios,    │  (decision→ │  (modules, │
│   decisions,     │   code via  │   imports,  │
│   evidence,      │   deps)     │   symbols)  │
│   notes)         │             │             │
│                  │             │             │
│  FPF Spec Index  │  Evidence Engine          │
│  (~800 sections, │  (R_eff, WLNK, CL,       │
│   route-aware)   │   decay, valid_until)     │
└─────────────────────────────────────────────┘
                     ▼
┌─────────────────────────────────────────────┐
│             PERSISTENCE                      │
│  SQLite (per project) │  .haft/ (markdown)  │
│  ~/.haft/ (global)    │  fpf.db (embedded)  │
└─────────────────────────────────────────────┘
```

## Layer Rules

1. **Core depends on nothing above.** Pure domain logic + store interface.
2. **Flow depends on Core.** Uses artifact store, knowledge graph, codebase analysis.
3. **Governor depends on Core + Flow.** Scans artifacts, checks invariants, spawns verification tasks.
4. **Surfaces depend on everything below.** Desktop/MCP/CLI call through Go bindings.
5. **No skip-level access.** Desktop does NOT query SQLite directly — goes through Core.
6. **Side effects only at Flow and above.** Core is pure queries + mutations through Store interface.

## Data Flow: Think → Run → Govern

```
THINK (human via desktop, or agent via MCP)
  │
  ├─ Frame problem (signal, constraints, acceptance)
  ├─ Explore variants (3+ genuinely distinct)
  ├─ [Probe-or-commit gate]
  ├─ Compare under parity (Pareto front computed)
  └─ Decide (invariants, claims, rollback, valid_until)
       │
       ▼
RUN (agent via Flow layer)
  │
  ├─ Create worktree
  ├─ Inject invariants from knowledge graph
  ├─ Spawn agent (Claude Code / Codex / custom)
  ├─ Agent executes with full reasoning context
  ├─ Post-execution: verify invariants
  └─ Baseline affected files
       │
       ▼
GOVERN (background via Governor layer)
  │
  ├─ Detect file drift (hash mismatch vs baseline)
  ├─ Verify invariants against dependency graph
  ├─ Check evidence freshness (valid_until countdown)
  ├─ Compute impact propagation (transitive dependencies)
  ├─ Generate problem candidates for violations
  └─ Alert via desktop dashboard
       │
       ▼
  (cycle back to THINK if problems found)
```

## File Map

```
cmd/haft/main.go               CLI entry point
internal/artifact/              CORE: artifact store, types, refresh, drift
internal/graph/                 CORE: knowledge graph queries, impact, verify
internal/codebase/              CORE: module detection, imports, symbols, coverage
internal/fpf/                   CORE: FPF spec index and search
internal/reff/                  CORE: R_eff computation, evidence scoring
internal/cli/serve.go           MCP: tool dispatch, schema, cross-project recall
internal/cli/agent.go           FLOW: standalone agent launcher
internal/cli/sync.go            FLOW: team sync (.haft/*.md → SQLite)
internal/agentloop/             FLOW: ReAct coordinator (standalone mode)
internal/tools/                 FLOW: tool implementations
internal/mcp/                   MCP: protocol handler
desktop/app.go                  SURFACE: Wails bindings (52 methods)
desktop/agents.go               FLOW: task runner, worktree, MCP auto-wire
desktop/governance.go           GOVERNOR: background scanner, findings
desktop/decision_flow.go        FLOW: implementation/verification prompts (pure)
desktop/views.go                SURFACE: domain → view model projections
desktop/frontend/src/           SURFACE: React pages and components
db/                             PERSISTENCE: SQLite schema, migrations
```
