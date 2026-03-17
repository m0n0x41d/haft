# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased] — v5.0.0

### Breaking Changes

Complete product redesign. v5 is not backward-compatible with v4.

- All v4 MCP tools removed (internalize, propose, verify, test, audit, decide, resolve, implement, link, context, reset, compact)
- All v4 slash commands removed (q1-hypothesize through q5-decide, q-internalize, q-implement, q-query, q-reset, q-resolve)
- Hypothesis/phase-based model replaced with problem/solution/decision model
- Phase FSM, role system, L0/L1/L2 user-facing layers removed
- All v4 internal code removed (10,461 lines): hypothesis engine, preconditions, projections, git integration, context maturity, session management
- All v4 docs removed (architecture.md, advanced.md, fpf-engine.md, evidence-freshness.md, examples/)
- .quint/ directory structure changed to v5 model (notes/, problems/, solutions/, decisions/, evidence/, refresh/)
- /fpf skill removed, replaced by /q-reason
- MCP server version bumped to 5.0.0

### Added — Product Model

- **6 MCP tools** (hard cap, ADR-2):
  - `quint_note` — micro-decisions with rationale validation (conflict detection, scope check, architectural keyword detection)
  - `quint_problem` — frame problems (signal, constraints, targets, acceptance), characterize comparison dimensions (versioned), list active problems with Goldilocks signals
  - `quint_solution` — explore variants with WLNK per option, compare and identify Pareto front (non-dominated set)
  - `quint_decision` — create DecisionRecords with invariants, pre/post-conditions, admissibility, rollback plan, refresh triggers; generate implementation briefs; record impact measurements; attach evidence items
  - `quint_refresh` — scan stale artifacts (decisions AND problems), waive/reopen/supersede/deprecate with audit trail, reopen carries forward lineage (prior characterization, failure reason, evidence refs)
  - `quint_query` — FTS5 search, status dashboard with context filter, file-to-decision lookup

- **11 slash commands**: `/q-note`, `/q-frame`, `/q-char`, `/q-problems`, `/q-explore`, `/q-compare`, `/q-decide`, `/q-apply`, `/q-refresh`, `/q-search`, `/q-status`

- **`/q-reason` skill** — teaches agent FPF-based structured reasoning with diagnostic framing protocol (7-question conversation), Quint tool integration, feature maturity table, depth calibration, proactive agent behavior

- **Artifact system** — 6 types (Note, ProblemCard, SolutionPortfolio, DecisionRecord, EvidencePack, RefreshReport) with markdown+YAML frontmatter files and SQLite projection

- **Dual-write storage** — DB primary for engine queries, markdown files also written for git tracking (ADR-18)

- **Navigation strip** — every tool response includes computed state (context, mode, derived status, stale count, next action) with multiple-active-item awareness

- **Note validation** — checks rationale, conflicts with active decisions, scope escalation before recording

- **Decision modes** — note, tactical, standard, deep with different artifact requirements per mode

- **Impact measurement** — `quint_decision(action="measure")` records post-implementation findings against DRR acceptance criteria with verdict (accepted/partial/failed)

- **Evidence attachment** — `quint_decision(action="evidence")` attaches evidence items to any artifact with type, verdict, CL, formality, carrier ref, validity

- **WLNK summary** — tracked-maturity WLNK: shows evidence count, supporting/weakening/refuting, min CL, freshness, stale detection across evidence chain

- **Versioned characterization** — `quint_problem(action="characterize")` appends versions (v1, v2...) instead of overwriting, preserving history

- **Goldilocks assist** — `quint_problem(action="select")` shows blast radius, reversibility, characterization count, links, validity for problem prioritization

- **All-artifact stale scan** — refresh detects expired ProblemCards and other artifacts, not just decisions

- **Reopen with lineage** — reopening a stale decision carries forward prior characterization, failure reason, and evidence references to new ProblemCard

- **Diagnostic framing protocol** — SKILL.md teaches agent 7-question diagnostic conversation for problem discovery

- **Feature maturity levels** — every FPF concept tagged textual/tracked/computed/enforced in SKILL.md and ADR-19

### Added — Infrastructure

- **FPF spec search** — `quint-code fpf search/section/info` CLI with embedded FTS5 index (4243 sections from FPF-Spec.md via go:embed)
- **FPF upstream submodule** — `data/FPF/` tracking github.com/ailev/FPF
- **FPF spec indexer** — `cmd/indexer` builds FTS5 database from upstream FPF-Spec.md
- **goreleaser** — cross-platform release builds (darwin/linux × amd64/arm64)
- **CI: weekly FPF update** — checks upstream submodule, rebuilds index, creates PR if changed
- **CI: release on tag** — goreleaser via GitHub Actions
- **CI: updated** — Go 1.24, submodule checkout, lint

### Fixed

- Frontmatter parser no longer breaks on markdown horizontal rules (`---` in body)
- File write and affected files errors are surfaced as warnings instead of silently swallowed
- QueryStatus respects contextFilter parameter
- Nav strip shows count when multiple problems/portfolios/decisions are active
- FTS5 search escapes special characters (`"*(){}^`)
- ID collision returns clean "artifact already exists" error
- Measure generates unique evidence IDs (UnixNano-based)
- Measure checks and returns AddEvidenceItem errors
- ValidUntil display guards against strings shorter than 10 chars
- Title field written to YAML frontmatter (was lost on roundtrip)

### Removed

- All v4 internal code (hypothesis engine, phase FSM, role system, preconditions, projections, git integration, context maturity heuristic, evidence management, decision finalization, audit, lifecycle, search, relations, implementation)
- All v4 slash commands and /fpf skill
- Legacy docs (architecture.md, advanced.md, fpf-engine.md, evidence-freshness.md, examples/)

### Architecture Decisions (ADRs)

- ADR-1: Breaking changes expected
- ADR-2: MCP surface capped at 6 tools
- ADR-3: Core artifact types — 6
- ADR-4: Dual-write (DB primary, files also written)
- ADR-5: No separate event log (audit_log table suffices)
- ADR-6: Legacy reuse — internal kernels only (WLNK, CL, decay, FTS5)
- ADR-7: Decision modes (note/tactical/standard/deep)
- ADR-8: Artifact format (markdown + YAML frontmatter)
- ADR-9: .quint/ directory structure
- ADR-10: Nav strip in every tool response
- ADR-11: Implementation order
- ADR-12: quint_note validates before recording
- ADR-13: Tools are prescriptive, never blocking
- ADR-14: Context is optional metadata
- ADR-15: DecisionRecord quality standard
- ADR-16: Legacy reuse criteria
- ADR-17: Agent proactive use
- ADR-18: Source of truth (DB primary, files secondary)
- ADR-19: Feature maturity levels

### Database

- Migration 13: v5 artifact model (artifacts, artifact_links, evidence_items, affected_files, artifacts_fts with FTS5 triggers)
- Legacy v4 tables retained for migration compatibility (holons, evidence, relations, etc.)
- db/store.go marked as legacy with documentation header
