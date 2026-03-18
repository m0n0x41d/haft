# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased] — v5.1.0

### Added — Computed Features

- **R_eff computation** — effective reliability = min(evidence_scores) with CL penalties (CL3=0, CL2=0.1, CL1=0.4, CL0=0.9). Expired evidence scores 0.1. Computed on every access.
- **Evidence decay → stale detection** — decisions with R_eff < 0.5 auto-surface in stale scan. R_eff < 0.3 = "AT RISK" label.
- **Graduated epistemic debt** — stale items sorted by severity (days overdue), debt magnitude displayed.
- **Diversity check** — Jaccard similarity on variant titles+descriptions. Warns at >50% word overlap.
- **Archive recall** — FTS5 search at frame/explore time surfaces related active artifacts as "Related History".
- **Characterization cross-check** — compare warns on dimension mismatch, asymmetric scoring, parity rules.
- **Parity checklist** — auto-generated per-dimension parity questions from characterization.
- **Goldilocks signals** — readiness score (section completeness) + complexity counts (constraints, targets, dimensions) in problem selection.
- **Problem lifecycle** — three-way split: Backlog (no work) → In Progress (has portfolio) → Addressed (has decision).
- **Proactive evidence prompts** — after frame/explore in standard+ mode, tool prompts agent to collect and attach evidence.
- **Periodic refresh prompt** — if >5 days since last scan, any tool response reminds agent to run refresh.
- **Lemniscate feedback** — failed/partial measurement suggests reopen with concrete command.

### Added — Product Features

- **FPF E.9 Decision Records** — four-component structure: Problem Frame, Decision/Contract, Rationale, Consequences. Decide response shows full DRR inline.
- **Indicator roles** — characterization dimensions tagged as constraint (hard limit), target (optimize), or observation (Anti-Goodhart).
- **Per-dimension measurement freshness** — valid_until on individual comparison dimensions. Compare warns on expired measurements.
- **Note auto-lifecycle** — notes auto-expire at 90 days. Detectable by scan. Waive/deprecate/supersede supported.
- **Generalized refresh** — waive/supersede/deprecate work on ALL artifact types (notes, problems, decisions, portfolios), not just decisions.
- **Multi-problem decisions** — `problem_refs` array parameter: one decision can address multiple problems.
- **Audit trail** — every tool call logged to audit_log table (fire-and-forget).
- **SoTA survey prompt** — explore in standard/deep mode reminds to check existing solutions before deciding.
- **Status caps** — dashboard sections capped (decisions=5, stale=5, problems=5, addressed=3) with overflow indicator.
- **List action** — `quint_query(action="list", kind="DecisionRecord")` for full artifact listing without caps.
- **Evidence display in problems** — /q-problems shows evidence count and verdict summary per problem.

### Fixed

- **CL=0 silent upgrade** — CL=0 (opposed context) no longer defaulted to CL=3. Uses -1 sentinel for "not provided".
- **NextSequence race condition** — uses MAX(id) instead of COUNT to avoid TOCTOU duplicate IDs.
- **Swallowed errors** — store.Update and store.AddLink errors in refresh operations now logged via logger.Warn.
- **FTS5 special characters** — comprehensive stripping of +, -, :, ~, single quote alongside existing chars.
- **MCP server stability** — panic recovery in request handler, 1MB stdin buffer (was 64KB), lifecycle logging (start/stop/EOF), stdout write error handling.
- **MCP init config** — uses QUINT_PROJECT_ROOT env instead of cwd. Removed stale nested .mcp.json.
- **writeFileQuiet** — uses logger.Warn instead of fmt.Fprintf(stderr).

### Changed

- **Apply deprecated** — decide response includes full DRR body. Apply action returns body directly (backward compat). `/q-apply` slash command removed.
- **Refresh UX** — tool description, schema, and slash command updated: "manage artifact lifecycle" not "detect stale decisions". `artifact_ref` parameter added (alongside `decision_ref` for compat).
- **Nav strip** — shows tactical decide option after frame. No apply prescription after decide.

### Removed

- `/q-apply` slash command
- Apply prescription from nav strip and decide response

## [5.0.0] — 2026-03-16

### Breaking Changes

Complete product redesign. v5 is not backward-compatible with v4.

- All v4 MCP tools removed
- All v4 slash commands removed
- Hypothesis/phase-based model replaced with problem/solution/decision model
- Phase FSM, role system, L0/L1/L2 user-facing layers removed
- .quint/ directory structure changed

### Added

- 6 MCP tools: quint_note, quint_problem, quint_solution, quint_decision, quint_refresh, quint_query
- 11 slash commands
- /q-reason skill with diagnostic framing protocol
- Artifact system with dual-write storage (DB primary, files secondary)
- Navigation strip in every tool response
- Note validation (rationale, conflicts, scope)
- Decision modes (note, tactical, standard, deep)
- Impact measurement and evidence attachment
- Versioned characterization
- All-artifact stale scan with lineage on reopen
- FPF spec search (4243 sections embedded)
- goreleaser for cross-platform builds

### Architecture Decisions

ADR-1 through ADR-19 documented in `.context/v5-architecture-decisions.md`
