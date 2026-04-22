# Artifact Ontology

> Reading order: 5 of N. Read after MODE_ONTOLOGY.

## Artifact Kinds

| Kind | Created by | Purpose | Lifecycle |
|------|-----------|---------|-----------|
| **ProblemCard** | Understand mode | Frames what's broken: signal, constraints, acceptance | Backlog → In Progress → Addressed |
| **SolutionPortfolio** | Explore mode | Contains 2+ variants + optional characterization + comparison | Active → Superseded/Deprecated |
| **DecisionRecord** | Execute mode | Records what was chosen: rationale, invariants, claims, rollback | Pending → Shipped → Active → Stale → Superseded/Deprecated |
| **EvidencePack** | Verify mode | Measurement data with verdict, CL, valid_until | Active → Superseded (when new measurement replaces) |
| **Note** | Note fast path | Micro-decision with rationale | Active → (auto-expires 90 days) → Deprecated |
| **RefreshReport** | Verify mode | Documents lifecycle action (waive, reopen, etc.) | Active (immutable log) |

## Execution Records (vNext Model)

These records are part of the target model for the Haft/Open-Sleigh
integration. They are listed separately because the current artifact store does
not yet implement these kinds.

| Record | Created by | Purpose | Lifecycle |
|--------|-----------|---------|-----------|
| **ImplementationPlan** | Human-assisted planning from active DecisionRecord(s) | DAG of WorkCommissions with dependencies, locksets, evidence requirements, and scheduler policy | Draft → Approved → Running → Partially Blocked → Completed/Cancelled |
| **WorkCommission** | Human/User via Haft UI/CLI/agent draft | Bounded authorization to execute a selected DecisionRecord in a declared scope | Draft → Queued → Ready → Preflighting → Running → Completed/Failed/Blocked/Cancelled/Expired |
| **RuntimeRun** | Runner such as Open-Sleigh | One execution attempt against a WorkCommission, including phase outcomes and evidence refs | Claimed → Running → Passed/Failed/Cancelled/Stalled |
| **ExternalProjection** | Haft projection engine | Idempotent external tracker binding for observers | Desired → Drafted → Published → Synced/Drifted/Blocked |
| **AutonomyEnvelope** | Human principal | Batch/YOLO permission bounds for an ImplementationPlan | Draft → Approved → Active → Exhausted/Revoked/Expired |

## Artifact Status (Stored vs Derived)

### Stored status (persisted in database)

Each artifact has exactly one stored status:

| Status | Meaning | Applies to |
|--------|---------|-----------|
| `active` | Live, current, counts for governance | All kinds |
| `addressed` | Problem solved by a linked decision | ProblemCard only |
| `superseded` | Replaced by another artifact (link preserved) | All kinds |
| `deprecated` | Archived as no longer relevant | All kinds |
| `refresh_due` | Flagged by scan, needs attention | All kinds |

### Derived health (computed at query time, never stored)

DecisionRecords have **two independent derived dimensions** computed from stored status + evidence state:

**Maturity** (exclusive — exactly one):

| Maturity | Derivation rule |
|----------|----------------|
| **Unassessed** | status=active AND no evidence exists on any claim |
| **Pending** | status=active AND evidence exists but no measurement with verdict=accepted |
| **Shipped** | status=active AND at least one measurement with verdict=accepted |

**Freshness** (exclusive — exactly one, evaluated only when maturity is Shipped):

| Freshness | Derivation rule |
|-----------|----------------|
| **Healthy** | R_eff >= 0.5 |
| **Stale** | R_eff < 0.5 AND R_eff >= 0.3 |
| **AT RISK** | R_eff < 0.3 |

Precedence: maturity is evaluated first, freshness only applies to Shipped decisions. A Pending decision has no freshness rating (it hasn't been verified yet).

Display string: `Shipped / Healthy`, `Shipped / Stale`, `Pending`, `Unassessed`.

Both dimensions are **view concerns** — shown in `/h-status`, desktop dashboard, and projections. They are never written to the database. Stored status and derived health are independent axes that must not be conflated.

## Artifact Relationships (DAG)

```
ProblemCard
    │
    ├── characterization (dimensions on the ProblemCard itself)
    │
    └──→ SolutionPortfolio (linked via problem_ref)
              │
              ├── variants (embedded in portfolio body)
              ├── comparison results (embedded)
              │
              └──→ DecisionRecord (linked via portfolio_ref + problem_ref)
                        │
                        ├── claims (structured, persisted)
                        ├── predictions (observable + threshold + verify_after)
                        ├── affected_files (with baseline hashes)
                        │
                        └──→ EvidencePack (linked via artifact_ref)
                                  │
                                  ├── verdict: supports / weakens / refutes
                                  ├── congruence_level: CL3/CL2/CL1/CL0
                                  └── valid_until: expiry date
```

Notes are standalone — linked by semantic overlap detection, not explicit refs.
RefreshReports reference the artifact they acted on.

## Artifact → Code Mapping (Knowledge Graph)

```
DecisionRecord
    │
    ├── affected_files: [path1, path2, ...]
    │       │
    │       └──→ Module (detected by codebase scanner)
    │               │
    │               └──→ Dependencies (import graph)
    │                       │
    │                       └──→ Other Modules (transitive)
    │                               │
    │                               └──→ Their DecisionRecords
    │
    └── invariants: ["no dep from X to Y", "no circular deps", ...]
            │
            └──→ Verified against live dependency graph
```

## Decision → Work Mapping

```
ProblemCard
    └──→ SolutionPortfolio
              └──→ DecisionRecord
                        │
                        ├──→ ImplementationPlan (optional)
                        │         │
                        │         └──→ WorkCommission*
                        │                    │
                        │                    ├──→ RuntimeRun*
                        │                    │         └──→ PhaseOutcome / EvidencePack
                        │                    │
                        │                    └──→ ExternalProjection* (optional)
                        │
                        └──→ EvidencePack (decision evidence, independent of execution)
```

Rules:

- A DecisionRecord may have zero WorkCommissions. A decision can wait.
- A WorkCommission must reference an active DecisionRecord revision/hash and
  must not keep that decision alive if it later becomes stale, superseded, or
  deprecated.
- A RuntimeRun must reference one WorkCommission and may start only after
  preflight passes and a runner lease is acquired.
- ExternalProjection is optional per workspace/commission. It is a derived
  carrier for coordination, never semantic authority.
- ImplementationPlan is a graph, not a list. Dependencies and locksets govern
  batch/YOLO scheduling.

**Queries available:**
- `FindDecisionsForFile(path)` — which decisions govern this file?
- `FindInvariantsForFile(path)` — what invariants must hold here?
- `FindModuleForFile(path)` — which module owns this file?
- `TransitiveDependents(module)` — what depends on this module?
- `ComputeImpactSet(decision)` — what files/modules affected if this decision is revisited?

## Persistence Model

| Store | What | Where | Who sees it |
|-------|------|-------|-------------|
| **SQLite** | Artifacts, links, affected files, evidence, audit log | `~/.haft/projects/{id}/haft.db` | Haft runtime only |
| **Projections** | Markdown rendering of each artifact | `.haft/{kind}/{id}.md` | Git, PRs, code review, team |
| **Cross-project index** | Decision summaries from all projects | `~/.haft/index.db` | Cross-project recall |
| **FPF spec index** | ~800 FPF sections with route-aware retrieval | `internal/cli/fpf.db` (embedded) | FPF search tool |

## Authority Model (Source of Truth)

**Precedence rule:**

| Context | Source of truth | Rationale |
|---------|----------------|-----------|
| **Runtime (single engineer)** | Local SQLite database | Engine operates on structured data, not markdown |
| **Team exchange** | `.haft/*.md` projections in git | Human-readable, reviewable in PRs, mergeable |
| **External coordination** | ExternalProjection to Linear/Jira/GitHub Issues | Manager/analyst/lead status visibility |
| **Conflict** | SQLite wins locally; `haft sync` is explicit reconcile step | No implicit overwrite. Sync fails closed on schema mismatch. |

**Projection invariant:** `.haft/*.md` files are **derived outputs** of the database. They are generated on every artifact create/update. They are NOT the source of truth for the local engineer — the database is.

**Team workflow invariant:** `.haft/*.md` in git is the **exchange format**. When another engineer runs `haft sync`, projections are parsed back into their local database. This is an explicit reconcile step, not a background sync.

```
Engineer A (local):
  create decision → SQLite updated → .haft/decisions/dec-001.md generated
  git commit + push .haft/

Engineer B (local):
  git pull → .haft/decisions/dec-001.md appears
  haft sync → parse projections → insert into local SQLite
  Both engineers now see the same decisions in /h-status
```

**What is NOT the source of truth:**
- `.haft/*.md` is not authoritative for the local engineer (SQLite is)
- SQLite is not shared between engineers (each has their own)
- Neither is authoritative for the other engineer until `haft sync` runs
- Linear/Jira/GitHub Issues are not authoritative for Haft semantics. They are
  optional carriers. Manual external status changes are drift/override inputs,
  not proof that a WorkCommission completed.
