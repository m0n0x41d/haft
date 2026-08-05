# Software Architecture Reference

> Layered software architecture for Haft as a project harnessability system.
> This engineering carrier remains outside ProjectSpecificationSet; the
> project-local software contract lives in `.haft/specs/software-system.md`.
> Each layer depends only on the layer directly below it.

## Module Hierarchy

```
┌─────────────────────────────────────────────┐
│               SURFACES                       │
│ Host Skills │ MCP Server │ Governance CLI │
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
│ runner-neutral lifecycle records, verify     │
└────────────────────┬────────────────────────┘
                     ▼
┌─────────────────────────────────────────────┐
│             REASONING CORE                   │
│ ProblemCard │ SolutionPortfolio │            │
│ DecisionRecord │ EvidencePack │ Note         │
│ R_eff │ Pareto │ Refresh │ FPF source/index   │
└────────────────────┬────────────────────────┘
                     ▼
┌─────────────────────────────────────────────┐
│          SPECIFICATION CORE                  │
│ TargetSystemSpec │ SoftwareSystemSpec        │
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
4. **Flow depends on Reasoning Core.** It runs onboarding, spec planning,
   commissioning, runner-neutral lifecycle recording, and verification.
5. **Governor depends on Flow.** It scans specs, artifacts, code, evidence, and runtime state for drift/staleness.
6. **Surfaces depend on Governor/Flow only.** Host skills, MCP, and CLI do not query SQLite or raw files directly.
7. **Side effects only at Flow and above.** Core layers expose pure transformations plus explicit store interfaces.

## Relation Model and Explicit Work Flows

Haft Core stores typed relations. It does not infer a canonical reasoning or
delivery sequence from source order, card order, graph insertion order, or
skill order.

```text
versioned FPF source --retrieved-into--> agent context
project concern --may-use--> U.MethodDescription
named reliance --materializes--> project artifact
SpecSection --governs--> DecisionRecord / code / test
DecisionRecord --may-authorize--> WorkCommission
WorkCommission --may-start--> RuntimeRun
EvidencePack --supports-or-weakens--> claim / SpecSection
current relations --derive--> status / SpecCoverage / drift findings
```

Relation-first does not mean acausal. A causal claim may establish causal
order, and an explicit `U.MethodDescription` may establish method order. Haft
does not infer either from adjacency or presentation.

Planning remains a separate description. An `ImplementationPlan` states work
dependencies and scheduling; a `WorkCommission` states bounded authority for a
specific execution slice. Neither an FPF source lookup nor an application
record is a plan, and no plan is performed work.

Once a WorkCommission exists, Haft exposes runner-neutral lifecycle operations:
runnable selection, claim, preflight result, start, event recording, recovery,
and terminal completion. Governor scans current specs, artifacts, code,
evidence, and runtime state independently of any execution path.

## External-runner boundary

Haft v9 ships no drainer, agent adapter, workspace manager, or patch applier.
Those are responsibilities of a separately operated external runner. The
domain port remains the WorkCommission lifecycle:

```
external runner
  -> list_runnable
  -> claim_for_preflight
  -> record_preflight
  -> start_after_preflight
  -> record_run_event
  -> complete_or_block
```

Every transition is validated against one commission's state, freshness,
scope, lease, lockset, and recorded authority. Scheduling and concurrency are
adapter policy outside Haft Core; processing several commissions never weakens
the per-commission contract.

`delivery_policy` is retained as a runner-facing instruction. Haft stores and
projects it but does not apply a patch. A runner that implements local apply
must obtain and enforce effect-specific authority, verify scope against the
current checkout, keep the effect reversible, and treat remote push, PR,
merge, tag, and publication as separate authority boundaries.

## File Map

```
cmd/haft/main.go               CLI entry point
internal/artifact/              CORE: artifact store, types, refresh, drift
internal/graph/                 CORE: knowledge graph queries, impact, verify
internal/codebase/              CORE: module detection, imports, symbols, coverage
internal/fpf/                   CORE: FPF spec index and search
internal/reff/                  CORE: R_eff computation, evidence scoring
internal/cli/serve.go           MCP: tool dispatch, schema, cross-project recall
internal/cli/spec.go            CLI: spec check commands
internal/cli/sync.go            FLOW: team sync (.haft/*.md → SQLite)
internal/embedding/             FLOW: embedding adapters, direct-key compatibility, and retained local sidecar
desktop-tauri/src/              ARCHIVE/SURFACE: historical Tauri command shell and task runner
desktop/frontend/src/           ARCHIVE/SURFACE: historical React pages and view models
db/                             PERSISTENCE: SQLite schema, migrations
```
