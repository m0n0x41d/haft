# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- **FTS5 search keyword enrichment** — `search_keywords` column on artifacts, indexed by FTS5. Agent generates synonyms and related terms at write time (e.g., "Redis for session store" gets keywords "cache, caching, in-memory, key-value, nosql"). Accepted on `quint_note` and `quint_decision`. Migration 15 rebuilds FTS5 index.
- **C/C++ header-only module detection** — `-I` include directories from `compile_commands.json` are registered as modules (FileCount=0), so dependency edges to `include/` directories are no longer dropped by `ScanDependencies`.

### Fixed

- **`/q-refresh scan` now rescans modules** — module structure updates alongside drift and stale checks, keeping dependency graph fresh without requiring a separate `coverage` action.
- **C/C++ symlink-safe include resolution** — `resolveInclude` canonicalizes both `projectRoot` and `-I` paths with `EvalSymlinks` before computing relative paths. Fixes silent edge loss on macOS symlinked checkouts.

## [5.2.0] — 2026-03-23

### Added

- **C/C++ module detection** — `compile_commands.json` as primary source (searches project root, `build/`, `cmake-build-*/`). Falls back to directory-based heuristic with `Makefile`/`CMakeLists.txt`/`meson.build` markers. Graceful fallback if `compile_commands.json` paths don't resolve.
- **C/C++ import parsing** — extracts `#include "..."` edges (skips `<...>` system includes). Resolves include paths using `-I` flags from `compile_commands.json`. Falls back to relative and project-root resolution.
- **C/C++ extensions** — `.c`, `.h`, `.cpp`, `.cc`, `.cxx`, `.hpp`, `.hxx` registered in language registry.

### Fixed

- **`quint_solution(action="explore")` rejected valid variants** — MCP clients that serialize the `variants` array as a JSON string (instead of a parsed array) caused silent parsing failure, resulting in a misleading "genuinely distinct options" error with 0 variants. Same fix applied to all array fields across note/problem/decision handlers. ([#33](https://github.com/m0n0x41d/quint-code/issues/33))
- **Status always rescans modules** — `quint_query(action="status")` now runs a fresh module scan instead of showing stale cached data. Previously required `action="coverage"` to trigger rescan.
- **Symlink-safe path resolution** — C/C++ module detection uses `filepath.EvalSymlinks` on project root and source paths for reliable matching on macOS and symlinked project directories.

## [5.1.0] — 2026-03-20

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

### Added — Codebase Awareness

- **File drift detection (Level A)** — `baseline` action snapshots SHA-256 hashes of affected files. `/q-refresh` detects drift (modified, file missing, no baseline). Self-correcting: unbaselined decisions surface in `/q-status`.
- **Module detection (Level B)** — detects modules/packages across Go (`go/parser`), JS/TS (`package.json` + `index.ts` barrel files), Python (`__init__.py`), Rust (`Cargo.toml` + `mod.rs`). Interface-based architecture — one implementation per language behind `ModuleDetector`/`ImportParser` interfaces.
- **Dependency graph (Level C)** — parses imports to build module dependency graph. Go uses `go/parser` stdlib (100% accuracy), JS/TS/Python/Rust use regex. Impact propagation: when module A drifts, drift report shows dependent modules and their decisions.
- **Decision coverage report** — `quint_query(action="coverage")` shows governed/partial/blind modules. R_eff-aware: only `DecisionRecord` artifacts count as governance, `partial` status for modules where all decisions have degraded evidence (R_eff < 0.5).
- **`.quintignore` support** — project-specific exclusions for module scanning. Also respects `.gitignore` (local + global) via `go-gitignore` library.
- **Module coverage in `/q-status`** — coverage section appended to status dashboard when modules are scanned.
- **Module-aware onboarding** — `/q-onboard` now includes module coverage analysis step, prioritizes blind modules.

### Added — Unified Storage & Cross-Project Recall

- **Unified storage** — database moved from `.quint/quint.db` (in-repo) to `~/.quint-code/projects/{id}/quint.db` (home dir). Markdown projections remain in `.quint/` for code review. No binary files in git.
- **Project identity** — `.quint/project.yaml` with immutable generated ID (`qnt_` + 8 hex). Created on `init`, committed to git, same for all developers.
- **Cross-project decision recall** — `~/.quint-code/index.db` stores decision summaries across all projects. When framing a new problem (`/q-frame`), related decisions from other projects surface with CL2/CL1 tags.
- **CL matching** — same primary language = CL2 (similar context), different language = CL1 (different context). Auto-detected from codebase modules.
- **Serve guard** — if old `.quint/quint.db` exists but project not migrated, MCP blocks all tool calls with migration instructions.
- **`QUINT_SERVER_ORIGIN`** — new env var in MCP config. `local` (default) for solo dev. URL value accepted for future remote server mode (not implemented yet).
- **Context facts** — `context_facts` table auto-populated on startup with project fingerprint (languages, module count, decision count, domains).

### Added — Decision Quality & Integrity

- **Adversarial verification gate** — `/q-decide` runs a verification check before recording. Tactical: one-line counter-argument. Standard/deep: 5 probes (deductive consequences, strongest counter-argument, self-evidence check, tail failure scenarios, WLNK challenge). Grounded in FPF A.12 + Verbalized Sampling research.
- **Evidence supersession** — when `Measure()` records a new measurement, previous measurements on the same artifact are marked `verdict='superseded'` and excluded from R_eff. Prevents old partial measurements from dragging R_eff down permanently.
- **Inductive measurement gate** — `Measure()` warns if no baseline exists for the decision's affected files. Measurements without baseline record at CL1 (self-evidence), not CL3. R_eff honestly reflects verification quality.
- **R_eff shared package** — `internal/reff/` extracts `ScoreEvidence`, `VerdictToScore`, `CLPenalty` as shared pure functions. Single source of truth for both `artifact` and `codebase` packages.
- **Note-decision dedup** — containment-based overlap check at write time. If >70% of a note's title words appear in an active decision title, the note is rejected. 50-70% = warning. Also checks note-vs-note duplicates.
- **Reconcile action** — `quint_refresh(action="reconcile")` batch-scans all active notes against all active decisions for overlaps. One Go-side pass, no per-note agent calls.
- **Shipped vs Pending** — `/q-status` splits decisions into "Shipped" (has measurement) and "Pending Implementation" (no measurement).
- **Post-implementation ritual** — SKILL.md teaches agent to baseline + verify + measure after implementing a decision.

### Added — Developer Experience

- **Structured logging** — middleware in `serve.go` auto-logs every MCP tool call entry/exit with tool name, action, duration_ms, status. Domain logging for artifact create/baseline/drift and codebase scan operations.
- **Codex skill support** — `quint-code init --codex` installs `/q-reason` skill to `~/.agents/skills/q-reason/SKILL.md`.
- **Pre-commit hook** — `.githooks/pre-commit` mirrors CI pipeline exactly: `go mod tidy`, `golangci-lint`, `go test -race`, `go build`.

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
- **Codex/Air project config** — `init --codex` / `init --air` now write MCP settings to project-local `.codex/config.toml` instead of shared `~/.codex/config.toml`.
- **writeFileQuiet** — uses logger.Warn instead of fmt.Fprintf(stderr).
- **MCP JSON string arrays** — `parseStringArray` now handles arrays sent as JSON strings by MCP clients (e.g., `"[\"a\"]"` instead of `["a"]`).
- **Coverage governance honesty** — only `DecisionRecord` artifacts count as governance. Notes no longer inflate coverage percentage.
- **Root module coverage** — root modules (Path: "") now match all files in the project, not just root-level files. Fixes JS/TS and Rust single-package coverage.
- **Measurement CL scoring** — measurements without baseline record at CL1 (0.4 penalty), not CL3. Prevents unverified measurements from inflating R_eff.
- **Coverage R_eff consistency** — unknown verdict in coverage computation now scores 0.5 (weakening), matching artifact package. Was incorrectly 0.0.
- **Status notes filter** — `/q-status` recent notes section filters out deprecated/superseded notes.
- **Evidence supersession in R_eff** — `ComputeWLNKSummary` excludes superseded evidence items from R_eff calculation.
- **FTS5 sanitization** — cross-project recall query sanitizer now strips periods, commas, semicolons, dashes, and other punctuation that caused FTS5 syntax errors.

### Changed

- **Apply deprecated** — decide response includes full DRR body. Apply action returns body directly (backward compat). `/q-apply` slash command removed.
- **Refresh UX** — tool description, schema, and slash command updated: "manage artifact lifecycle" not "detect stale decisions". `artifact_ref` parameter added (alongside `decision_ref` for compat).
- **Nav strip** — shows tactical decide option after frame. No apply prescription after decide.
- **Storage location** — database moved from `.quint/quint.db` to `~/.quint-code/projects/{id}/quint.db`. Requires re-running `quint-code init` to migrate.
- **Coverage always rescans** — `quint_query(action="coverage")` always runs fresh module scan instead of caching for 7 days.

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
