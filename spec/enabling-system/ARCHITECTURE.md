# Functional Architecture

> Layered enabling-system architecture for Haft as a project harnessability
> system. Each layer depends only on the layer directly below it.

## Module Hierarchy

```
┌─────────────────────────────────────────────┐
│               SURFACES                       │
│ Desktop Cockpit │ MCP Plugin │ CLI │ Runtime │
└────────────────────┬────────────────────────┘
                     ▼
┌─────────────────────────────────────────────┐
│              GOVERNOR                        │
│ spec freshness, drift, stale refresh,        │
│ invariant verification, problem factory      │
└────────────────────┬────────────────────────┘
                     ▼
┌─────────────────────────────────────────────┐
│                FLOW                          │
│ onboarding, spec planning, commissioning,    │
│ worktree lifecycle, agent spawning, verify   │
└────────────────────┬────────────────────────┘
                     ▼
┌─────────────────────────────────────────────┐
│             REASONING CORE                   │
│ ProblemCard │ SolutionPortfolio │            │
│ DecisionRecord │ EvidencePack │ Note         │
│ R_eff │ Pareto │ Refresh │ FPF index          │
└────────────────────┬────────────────────────┘
                     ▼
┌─────────────────────────────────────────────┐
│          SPECIFICATION CORE                  │
│ TargetSystemSpec │ EnablingSystemSpec        │
│ TermMap │ SpecSection │ SpecCoverage          │
│ SemanticArchitecture │ SpecCheck              │
└────────────────────┬────────────────────────┘
                     ▼
┌─────────────────────────────────────────────┐
│            CODEBASE CORE                     │
│ module detection │ imports │ symbols          │
│ file/module/function refs │ test refs          │
└────────────────────┬────────────────────────┘
                     ▼
┌─────────────────────────────────────────────┐
│             PERSISTENCE                      │
│ SQLite (per project) │ .haft/ markdown        │
│ ~/.haft/ global │ fpf.db embedded             │
└─────────────────────────────────────────────┘
```

## Layer Rules

1. **Codebase Core depends on Persistence only.** It normalizes files/modules/tests/symbols into references.
2. **Specification Core depends on Codebase Core.** It parses specs, validates terms, and computes spec coverage edges.
3. **Reasoning Core depends on Specification Core.** Decisions may reference spec sections; evidence may satisfy spec requirements.
4. **Flow depends on Reasoning Core.** It runs onboarding, spec planning, commissioning, runtime control, and verification.
5. **Governor depends on Flow.** It scans specs, artifacts, code, evidence, and runtime state for drift/staleness.
6. **Surfaces depend on Governor/Flow only.** Desktop/MCP/CLI do not query SQLite or raw files directly.
7. **Side effects only at Flow and above.** Core layers expose pure transformations plus explicit store interfaces.

## Data Flow: Specify → Think → Run → Govern

```
SPECIFY (human + onboarding agent)
  │
  ├─ Initialize .haft and host-agent MCP config
  ├─ Draft TargetSystemSpec
  ├─ Draft EnablingSystemSpec
  ├─ Validate TermMap and SpecSections
  └─ Compute SpecCoverage
       │
       ▼
THINK (human via desktop, or agent via MCP)
  │
  ├─ Frame problem from spec gap, drift, or human signal
  ├─ Explore variants (3+ genuinely distinct)
  ├─ [Probe-or-commit gate]
  ├─ Compare under parity (Pareto front computed)
  └─ Decide (spec refs, invariants, claims, rollback, valid_until)
       │
       ▼
RUN (agent via Flow layer)
  │
  ├─ Create WorkCommission from DecisionRecord
  ├─ Preflight spec/decision/scope freshness
  ├─ Create isolated workspace
  ├─ Inject invariants from knowledge graph
  ├─ Spawn agent (Claude Code / Codex / custom)
  ├─ Agent executes with full reasoning context
  ├─ Post-execution: verify invariants
  └─ Attach evidence and update SpecCoverage
       │
       ▼
GOVERN (background via Governor layer)
  │
  ├─ Detect spec drift and file drift
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
internal/spec/                  CORE: spec parser/checker, term map, spec coverage (planned)
internal/fpf/                   CORE: FPF spec index and search
internal/reff/                  CORE: R_eff computation, evidence scoring
internal/cli/serve.go           MCP: tool dispatch, schema, cross-project recall
internal/cli/spec.go            CLI: spec check commands
internal/cli/agent.go           FLOW: standalone agent launcher
internal/cli/sync.go            FLOW: team sync (.haft/*.md → SQLite)
internal/agentloop/             FLOW: ReAct coordinator (standalone mode)
internal/tools/                 FLOW: tool implementations
internal/mcp/                   MCP: protocol handler
desktop-tauri/src/              SURFACE/FLOW: Tauri command shell, task runner, project readiness
desktop/frontend/src/           SURFACE: React pages, typed workflow UI, and view models
db/                             PERSISTENCE: SQLite schema, migrations
```
