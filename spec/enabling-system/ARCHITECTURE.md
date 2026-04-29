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
│ worktree lifecycle, drain/apply, verify      │
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
  ├─ Create WorkCommission from DecisionRecord and AutonomyEnvelope
  ├─ Claim runnable commission or enter bounded drain loop
  ├─ Preflight spec/decision/scope freshness and AutonomyEnvelope
  ├─ Create isolated workspace
  ├─ Inject invariants from knowledge graph
  ├─ Re-check AutonomyEnvelope before execution
  ├─ Spawn agent (Claude Code / Codex / custom)
  ├─ Agent executes with full reasoning context
  ├─ Post-execution: verify invariants and judge evidence
  ├─ Attach evidence and terminalize commission with verdict
  ├─ Apply only policy-allowed passing work through AutonomyEnvelope
  └─ Update SpecCoverage
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

## Flow: WorkCommission Draining

Batch execution is a Flow-layer behavior over WorkCommissions. It does not
change the Reasoning Core contract, the commission schema, or the outer
operator authority boundary.

The drainer owns this linear pipeline:

```
source commissions
  .filter(valid_until_not_expired)
  .filter(stale_lease_within_age_cap)
  .claim_without_lockset_conflict
  .preflight_decision_and_scope
  .preflight_autonomy_envelope
  .execute_autonomy_envelope
  .execute_in_isolated_workspace
  .verify_required_evidence
  .record_terminal_verdict
  .apply_when_policy_and_envelope_allow
```

`haft harness run` without drain remains a single-claim execution surface. Drain
mode is opt-in and keeps the runtime alive only while runnable commissions
remain. Concurrency is bounded by the operator-provided concurrency limit, and
lockset overlap is still rejected at claim time. Drain mode never relaxes
lockset, scope, freshness, or AutonomyEnvelope checks.

AutonomyEnvelope has three distinct checkpoints in this flow. Commission
creation decides whether the work may be handed to the harness at all.
Preflight/execute decides whether an already-created commission may start or
continue agent work. Apply decides whether a passing terminal commission may
land in the operator checkout without a manual apply command. Any
`DecisionBlocked` or checkpoint-required result stops automatic progress at
that boundary.

### Drain Exit Conditions

Drain mode exits when the runnable queue is empty. A commission is runnable only
when it is not terminal, has not expired, does not have a stale lease beyond the
configured age cap, is not blocked by lockset overlap, and can pass preflight.
Operator signal is a first-class shutdown path: the drainer stops claiming new
work, asks active agents to terminate, releases or preserves leases according to
their current state transition, and exits without orphaning subprocesses.

### Stale Lease Policy

A claimed commission whose lease age exceeds the configured cap is not resumed
silently. The default cap is 24 hours. Intake reports the typed reason
`lease_too_old`, and harness status surfaces that state so an operator can
choose an explicit intervention: requeue, cancel, or inspect manually.

This is an enabling-system guardrail, not a target-system judgement. A stale
lease says that the harness no longer trusts the execution carrier enough to
resume automatically; it does not say the underlying product change is invalid.

### Auto-Apply Policy

Per-commission apply is a separate revertable git operation. The drainer may
apply a terminal commission if and only if all of these facts are true:

1. The commission verdict is `pass`.
2. The commission delivery policy is `workspace_patch_auto_on_pass`.
3. The AutonomyEnvelope decision at the apply checkpoint is `allowed`.

Any missing fact, failing verdict, manual delivery policy, checkpoint-required
envelope result, blocked envelope result, or apply failure leaves the commission
available for explicit operator action through the manual harness apply path.
The default delivery policy remains `workspace_patch_manual`.

The auto-apply path performs no remote operation. It does not push, open pull
requests, comment externally, batch-squash commissions, or merge to a protected
branch.

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
