# Quint Code v5 — Architecture Decisions

Date: 2026-03-16
Status: binding for v5 implementation
Inputs: refactor-spec, spec-review-findings, 5.4 Pro analysis

---

## ADR-1: Breaking changes are expected

V5 is a new product, not an evolution of v4. Breaking changes are not just permitted — they're the point.

No backward compatibility obligation for:
- MCP tool names or schemas
- Slash command names
- .quint/ directory structure
- SQLite schema
- Artifact format
- Internal Go types

Old q1-q5 commands MAY be provided as thin aliases if trivial. If the alias requires a translation layer thicker than the reuse value, drop it.

---

## ADR-2: MCP surface capped at 6 tools

Hard cap. Six canonical MCP tools. Not 13, not 11, not 8. Six.

```
quint_note       — micro decisions, quick captures
quint_problem    — frame, characterize, select problems
quint_solution   — explore variants, compare, build portfolio
quint_decision   — decide, apply, record rationale
quint_refresh    — detect stale, reopen, waive, update validity
quint_query      — search, status, history, diagnostics
```

Each tool uses an `action` parameter to select sub-operations:

```
quint_problem(action="frame", ...)
quint_problem(action="characterize", ...)
quint_problem(action="select", ...)
```

Slash commands are richer UX wrappers over these 6 tools:

| Slash command | MCP tool | action |
|---------------|----------|--------|
| `/q-note` | `quint_note` | — |
| `/q-frame` | `quint_problem` | `frame` |
| `/q-char` | `quint_problem` | `characterize` |
| `/q-problems` | `quint_problem` | `select` |
| `/q-explore` | `quint_solution` | `explore` |
| `/q-compare` | `quint_solution` | `compare` |
| `/q-decide` | `quint_decision` | `decide` |
| `/q-apply` | `quint_decision` | `apply` |
| `/q-refresh` | `quint_refresh` | — |
| `/q-search` | `quint_query` | `search` |
| `/q-status` | `quint_query` | `status` |

---

## ADR-3: Core artifact types — 6, not 13

The refactor spec lists 13 artifact types. That's type inflation. Six are genuinely distinct objects:

| Artifact | What it is | Mode required |
|----------|-----------|---------------|
| **Note** | Micro-decision with rationale | `note` |
| **ProblemCard** | Framed problem with constraints, targets, acceptance | `tactical+` |
| **SolutionPortfolio** | Variants with NQD, WLNK, comparison results | `standard+` |
| **DecisionRecord** | Selected variant, rationale, contracts, rollback | `tactical+` |
| **EvidencePack** | Evidence items supporting any artifact | all modes |
| **RefreshReport** | What's stale, why, what to do about it | on demand |

### Merged into other artifacts (not standalone):

| Refactor spec artifact | Where it lives in v5 |
|----------------------|---------------------|
| CharacterizationPassport | Section of ProblemCard or context doc |
| ProblematizationPassport | Section of context doc (if needed at all) |
| ComparisonAcceptanceSpec | Section of ProblemCard (`acceptance` + `comparison_rules` fields) |
| CharacteristicCard | Entries within ProblemCard's `optimization_targets` / `observation_indicators` |
| ParityPlan | Section of SolutionPortfolio (`parity` field) |
| ParityReport | Section of SolutionPortfolio (`comparison_results` field) |
| Runbook / RollbackPlan | Section of DecisionRecord (`rollback` field) |
| RefreshPlan | Section of DecisionRecord (`refresh_triggers` field) |

Rationale: a solo developer deciding between Redis and PostgreSQL for caching does not need 11 separate files. They need one problem description, one comparison, one decision. The `deep` mode can expand sections into richer content, but it's still one file per concept, not one file per FPF theoretical entity.

---

## ADR-4: Dual-write — DB primary, files also written

Source of truth: **SQLite remains the primary engine store.** Files are also written for human readability and git tracking.

Write path:
1. DB insert/update (primary — this is what the engine queries)
2. File write to `.quint/` (secondary — human-readable projection, git-tracked)

Read path:
- Engine reads from DB (fast, indexed, joins)
- Humans/git read files (readable, diffable)

Why not files-first: the engine needs joins, FTS5 search, aggregate queries, and transactional consistency. Parsing markdown+YAML on every tool call is slower and error-prone. The DB handles this natively.

Why also write files: git history, code review, human readability, portability.

Conflict resolution: if files are manually edited, `quint_query(action="sync")` reimports them into DB. But DB is authoritative for the engine.

---

## ADR-5: No separate event log — audit_log is enough

The current `audit_log` table already captures: timestamp, tool_name, operation, actor, target_id, result, details, context_id.

This IS an event log. Adding a second one (ndjson file) for "future cloud sync" is premature. When cloud sync is built, the audit_log table can be exported to any format.

No ndjson event file in v5.

---

## ADR-6: Legacy reuse — internal kernels only

### Preserve as invisible internal subsystems

| Module | What it computes | Where used in v5 |
|--------|-----------------|------------------|
| WLNK calculator | `R_eff = min(evidence_scores)` | SolutionPortfolio variant scoring, DecisionRecord assurance |
| CL penalties | Congruence-based score reduction | Evidence quality assessment |
| Evidence decay | Freshness tracking, valid_until | RefreshReport stale detection |
| FTS5 search | Full-text search over artifacts | `quint_query(action="search")` |
| Audit log | Tool call history | All tools (append on every call) |

These are computation primitives. They don't dictate product ontology.

### Drop — do not carry into v5

| Module | Why dropped |
|--------|-----------|
| Phase FSM (StageNeedsVerify etc.) | Replaced by derived status from artifact completeness |
| Role system (Abductor/Deductor etc.) | Never surfaced to users, adds no value |
| Hypothesis as user concept | Replaced by ProblemCard + SolutionPortfolio variants |
| L0/L1/L2 as user-facing layers | Becomes internal `assurance_level` field on evidence |
| Old MCP tool contracts (propose/verify/test/audit) | Replaced by 6 new tools |
| Context maturity heuristic | Brittle, removed |
| Characteristics table | Absorbed into artifact fields |
| Predictions table | Absorbed into evidence |
| fpf_state table | Replaced by derived status |

### The test: if reuse requires a translation layer thicker than the reuse value, drop it.

---

## ADR-7: Decision modes

Four modes, same as refactor spec, but with reduced artifact requirements:

| Mode | Artifacts produced | When |
|------|-------------------|------|
| `note` | Note only | Micro-decisions, "I chose X because Y" |
| `tactical` | ProblemCard (light) + DecisionRecord (light) | Reversible, <2 weeks blast radius |
| `standard` | ProblemCard + SolutionPortfolio + DecisionRecord | Most architectural decisions |
| `deep` | All standard + rich characterization, parity, runbook, refresh triggers | Irreversible, security/legal, cross-team |

Mode is a parameter, not a workflow gate. The agent suggests mode based on problem characteristics. User overrides.

---

## ADR-8: Artifact format — markdown with YAML frontmatter

All artifacts are markdown files with YAML frontmatter:

```yaml
---
id: prob-20260316-001
kind: ProblemCard
version: 1
status: selected
created_at: 2026-03-16T12:00:00Z
updated_at: 2026-03-16T12:40:00Z
context: payments-reliability
valid_until: 2026-04-01T00:00:00Z
mode: standard
links:
  - ref: sol-20260316-001
    type: informs
---

# Webhook Delivery Ambiguity

## Signal
Payment webhook retries hitting 15% failure rate...

## Constraints
- Must maintain <500ms p99 latency
- Cannot break existing merchant integrations

## Optimization Targets
- Reduce webhook failure rate to <1%

## Observation Indicators
- CPU utilization on webhook workers
- Queue depth
```

ID format: `{kind_prefix}-{YYYYMMDD}-{sequence}`

---

## ADR-9: .quint/ directory structure

Minimal. Created on demand, not all at once.

```
.quint/
  quint.db              # SQLite — engine primary store
  notes/                # Note artifacts
  problems/             # ProblemCard artifacts
  solutions/            # SolutionPortfolio artifacts
  decisions/            # DecisionRecord artifacts
  evidence/             # EvidencePack artifacts
  refresh/              # RefreshReport artifacts
```

No: events/, imports/, projections/, legacy/, contexts/, characterization/, operations/, passports/, archives/, portfolios/.

Directories created when first artifact of that kind is written. Not pre-created.

---

## ADR-10: Nav strip in every tool response

Every MCP tool response appends:

```
── Quint ──────────────────────────
Context: payments-reliability
Mode: standard
Problem: webhook delivery ambiguity [FRAMED]
Portfolio: 3 variants, 2 non-dominated
Decision: pending
Stale: 1 benchmark expired
Next: quint_solution(action="compare", ...)
───────────────────────────────────
```

Derived status from artifact completeness:
- `UNDERFRAMED` — no ProblemCard
- `FRAMED` — ProblemCard exists
- `EXPLORING` — SolutionPortfolio has variants
- `COMPARED` — comparison results exist
- `DECIDED` — DecisionRecord exists
- `APPLIED` — implementation evidence recorded
- `REFRESH_DUE` — stale evidence detected

These are computed, not stored. No FSM.

---

## ADR-11: Implementation order

```
1. New artifact types + file writer + DB schema     ← foundation
2. quint_note                                        ← prove the pattern works
3. quint_problem                                     ← core differentiator
4. quint_solution                                    ← portfolio + comparison
5. quint_decision                                    ← close the loop
6. quint_refresh                                     ← the moat
7. quint_query                                       ← search + status + nav
8. Slash commands + SKILL.md update                  ← UX layer
9. Docs + README + install polish                    ← distribution
```

Do not start with docs or commands. Start with artifact types and the first tool.

---

## Summary

| Decision | Choice |
|----------|--------|
| Breaking changes | Yes, expected |
| MCP tools | 6 (hard cap) |
| Artifact types | 6 core (not 13) |
| Source of truth | DB primary, files also written |
| Event log | No separate file, audit_log table suffices |
| Legacy reuse | Internal kernels only (WLNK, CL, decay, FTS5) |
| Old tool contracts | Dropped |
| Old phase model | Dropped |
| Modes | note / tactical / standard / deep |
| Format | Markdown + YAML frontmatter |

---

## ADR-12: quint_note validates before recording

quint_note is not an append log. It's a minimal FPF checkpoint.

Three checks before any note is recorded:

1. **Rationale check**: No rationale or <10 words with affected_files → ask "why this choice?"
2. **Conflict check**: FTS5 search title + affected_files against active DecisionRecords. If invariant violated → warn and show the conflicting decision. Do not silently record.
3. **Scope check**: If affected_files > 3 or keywords like "migrate/replace/rewrite/architecture" detected → suggest `/q-frame` instead.

All three are soft blocks — Quint doesn't refuse, it asks questions. If user insists with rationale provided, record it.

---

## ADR-13: Tools are prescriptive, never blocking

Every tool knows what artifacts exist. If steps are skipped:

- `/q-explore` without ProblemCard → "No problem framed. What are we solving?" + option to create inline
- `/q-compare` without characterization → "No dimensions defined. Derive from variants?"
- `/q-decide` without comparison → "3 variants, no comparison. Compare first, or decide in tactical mode?"

Format: what's missing → what exists → what to call next. Never just "cannot do X."

---

## ADR-14: Context is optional metadata, ProblemCard IS the frame

No mandatory `quint_context()` call before anything works.

ProblemCard is the frame. When you `/q-frame`, you create a ProblemCard. That's the anchor.

Context is a tag in frontmatter (`context: payments-reliability`). Multiple ProblemCards can share it. It's for grouping and search. Not a workflow gate.

---

## ADR-15: DecisionRecord quality standard

The DRR is the crown jewel artifact. Required sections by mode:

| Section | tactical | standard | deep |
|---------|----------|----------|------|
| Selected Variant | ✅ | ✅ | ✅ |
| Why Not Others | light | full table | full table |
| Invariants | ✅ | ✅ | ✅ |
| Pre-conditions | — | ✅ checklist | ✅ checklist |
| Post-conditions | — | ✅ checklist | ✅ checklist |
| Admissibility | — | ✅ | ✅ |
| Evidence Requirements | — | ✅ | ✅ |
| Rollback Plan | light | full | full |
| Refresh Triggers | ✅ | ✅ | ✅ |
| Weakest Link | ✅ | ✅ | ✅ |
| Assurance | — | ✅ | ✅ |
| Parity Report | — | — | ✅ |
| Runbook | — | — | ✅ |

Quality bar: new engineer reads DRR 6 months later, understands what/why/boundaries/watch-for/when-to-revisit.

---

## ADR-16: Legacy reuse criteria (from 5.4 Pro analysis)

A v4 module stays in v5 ONLY if ALL conditions hold:

1. It provides real moat (not just "too much work to rewrite")
2. It lives behind new API without leaking old concepts
3. It doesn't require a new MCP tool for its own sake
4. It doesn't impose hypothesis/phases/L0-L2 as external mental model
5. The wrapper around it is thinner than the reuse value

If two or more fail → drop the module. No sentimental preservation.

**Keep**: WLNK calculator, CL penalties, evidence decay, FTS5, audit log
**Drop**: phase FSM, roles, hypothesis types, context maturity, predictions table, fpf_state

---

## ADR-18: Source of truth

**SQLite is the primary store for the engine.** All queries, status computation, search, and staleness detection read from DB.

**Markdown files are also written** on every create/update for git tracking, human readability, and portability.

If files and DB diverge, DB is authoritative for the engine. A future `quint_query(action="sync")` may reimport manually-edited files.

This is not "files-first" (refactor spec's proposal) and not "DB-only" (v4's model). It's dual-write with DB as engine authority.

---

## ADR-19: Feature maturity levels

Every FPF concept in user-facing surfaces must be tagged with its actual maturity:

| Level | Meaning | Example |
|-------|---------|---------|
| **textual** | Stored as text label, no logic behind it | WLNK on variant, CL on evidence |
| **tracked** | Structurally stored, displayed, queryable | Comparison dimensions, Pareto front members |
| **computed** | Calculated from data by the engine | (none yet in v5) |
| **enforced** | Participates in gates, blocks, or automated detection | valid_until staleness scan |

**Rule**: No surface (README, SKILL.md, slash command descriptions) may describe a concept at a higher maturity than its actual implementation. Textual features must not be presented as computed.

Current maturity map:

| Concept | Maturity | Notes |
|---------|----------|-------|
| Problem framing | tracked | ProblemCard stores fields |
| Characterization | tracked | Dimension table stored |
| WLNK | textual | Label on variant/decision |
| Parity | textual | Rules stored as text |
| Pareto front | tracked | User identifies, Quint stores |
| Stepping stones | tracked | Boolean flag, shown in summary |
| valid_until staleness | enforced | Scanned by refresh, surfaced in status |
| Refresh triggers (text) | textual | Stored, not scanned |
| CL (congruence) | textual | Field exists, unused |
| F (formality) | textual | Field exists, unused |
| NQD | absent | Not implemented |
| Note conflict check | enforced | FTS5 search + affected files |
| Note rationale check | enforced | Validates before recording |

---

## ADR-17: Agent proactive use

The SKILL.md teaches the agent to use Quint proactively:

- Call `quint_query(action="related", file="...")` when dev works on files linked to decisions
- Suggest `quint_note` when dev makes inline decisions in conversation
- Surface stale decisions at session start via `quint_query(action="status")`
- Capture decision rationale from conversation even without explicit `/q-note`

Quint is infrastructure the agent uses. The user works through the agent, not through raw MCP calls.
