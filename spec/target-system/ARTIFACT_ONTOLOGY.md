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

### Derived phase (computed at query time, never stored)

DecisionRecords have a **derived phase** computed from stored status + evidence state:

| Phase | Derivation rule |
|-------|----------------|
| **Pending** | status=active AND no measurement exists |
| **Shipped** | status=active AND at least one measurement with verdict=accepted |
| **Stale** | status=active AND R_eff < 0.5 (evidence degraded) |
| **AT RISK** | status=active AND R_eff < 0.3 |

Phase is a **view concern** — shown in `/h-status`, desktop dashboard, and projections. It is never written to the database. Stored status and derived phase are independent axes that must not be conflated.

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
