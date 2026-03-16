# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased] — v5.0.0

### Breaking Changes

This is a complete product redesign. v5 is not backward-compatible with v4.

- All v4 MCP tools removed (internalize, propose, verify, test, audit, decide, resolve, implement, link, context, reset, compact)
- All v4 slash commands removed (q1-hypothesize through q5-decide)
- Hypothesis/phase-based model replaced with problem/solution/decision model
- Phase FSM, role system, L0/L1/L2 user-facing layers removed
- .quint/ directory structure changed

### Added — New Product Model

- **6 MCP tools** (hard cap per ADR-2):
  - `quint_note` — micro-decisions with rationale validation (conflict detection, scope check)
  - `quint_problem` — frame problems (signal, constraints, targets, acceptance), characterize comparison dimensions, list active problems
  - `quint_solution` — explore variants with WLNK per option, compare on Pareto front with non-dominated set
  - `quint_decision` — create DecisionRecords with invariants, pre/post-conditions, admissibility, rollback plan, refresh triggers; generate implementation briefs
  - `quint_refresh` — scan stale decisions, waive/reopen/supersede/deprecate with full audit trail
  - `quint_query` — FTS5 search, status dashboard, file-to-decision lookup

- **11 slash commands**: `/q-note`, `/q-frame`, `/q-char`, `/q-problems`, `/q-explore`, `/q-compare`, `/q-decide`, `/q-apply`, `/q-refresh`, `/q-search`, `/q-status`

- **`/q-reason` skill** — teaches agent FPF-based structured reasoning with Quint tool integration

- **Artifact system** — 6 artifact types (Note, ProblemCard, SolutionPortfolio, DecisionRecord, EvidencePack, RefreshReport) with markdown+YAML frontmatter files and SQLite projection

- **Dual-write storage** — DB primary for engine queries, markdown files also written for git tracking and human readability

- **Navigation strip** — every tool response includes computed state (context, mode, derived status, next action) for context window survival

- **Note validation** — quint_note checks for missing rationale, conflicts with active decisions, and architectural scope before recording

- **Decision modes** — note, tactical, standard, deep with different artifact requirements per mode

### Added — Infrastructure (from earlier in dev cycle)

- **FPF spec search** — `quint-code fpf search/section/info` CLI with embedded FTS5 index (4243 sections from FPF-Spec.md)
- **FPF upstream submodule** — `data/FPF/` tracking github.com/ailev/FPF
- **goreleaser** — cross-platform release builds
- **CI workflows** — weekly FPF upstream check, release on tag push

### Removed

- All v4 internal code: hypothesis engine, phase FSM, role system, preconditions, projections, git integration, context maturity heuristic
- Old slash commands: q-internalize, q-implement, q-query, q-reset, q-resolve, q1-q5
- /fpf skill (replaced by /q-reason)

### Database

- Migration 13: v5 artifact model (artifacts, artifact_links, evidence_items, affected_files, artifacts_fts tables)
