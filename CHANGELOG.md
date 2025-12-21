# Changelog

All notable changes to Quint Code will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [4.1.0] - Unreleased

### Added

- **sqlc Integration**: Type-safe database queries generated from SQL.
  - All database operations now use sqlc-generated code with proper type safety.
  - New `db/store.go` wrapper provides clean API while preserving schema bootstrap.
  - Added comprehensive test suite for database operations (`db/store_test.go`).

- **GetHolon Query**: Added query to fetch hypothesis metadata by ID (foundation for future Kind-CAL work).

- **New MCP Tools for Trust Calculus (B.3)**:
  - `quint_audit_tree`: Visualize assurance tree with R scores, dependencies, and CL penalties.
  - `quint_calculate_r`: Compute R_eff with detailed breakdown (self score, weakest link, decay penalties).
  - `quint_check_decay`: Identify holons with expired evidence (epistemic debt detection).

- **Parent ID Chain (FPF Enforcement)**:
  - Added `parent_id` foreign key to holons table for L0→L1→L2 progression tracking.
  - New queries: `GetHolonsByParent`, `GetHolonLineage` for traversing hypothesis chains.
  - `CreateHolon` now accepts parent_id parameter for linking hypothesis progression.
  - Enables auditable chain from L2 decision back to original L0 hypothesis.

- **Derived Phase (FPF Enforcement)**:
  - Phase is now computed from holons.layer data instead of stored in state.json.
  - New `DerivePhase()` method computes phase from database state.
  - New `GetPhase()` returns derived phase when DB available, falls back to State.Phase.
  - Prevents AI bypass of FPF phase controls via direct file manipulation.

- **Audit Logging (FPF Enforcement)**:
  - New `audit_log` table tracks all MCP tool invocations.
  - Captures: tool name, operation, actor, target ID, input hash, result, and details.
  - Instrumented tools: `quint_propose`, `quint_verify`, `quint_decide`, `quint_move`.
  - Enables detection of FPF bypasses through audit trail analysis.
  - Context-aware logging supports multi-session isolation.

- **Self-Healing Signed Projections (FPF Enforcement)**:
  - All hypothesis/evidence/DRR files now include `content_hash` in YAML frontmatter.
  - New `WriteWithHash()` function adds cryptographic hash (SHA-256 truncated) on write.
  - New `ValidateFile()` detects tampering by comparing stored vs computed hash.
  - New `ReadWithValidation()` on Tools automatically detects and logs tampering.
  - When tampering detected: regenerates file from DB (DB is source of truth).
  - Tampering events logged to audit_log for violation tracking.

- **DRR Holon Tracking**:
  - `FinalizeDecision` now creates DRR holon in database (enables derived phase detection).
  - DRR holons linked to winner hypothesis via parent_id.

- **Tool Preconditions (FPF Enforcement)**:
  - All MCP tools now validate preconditions before execution.
  - `quint_propose`: Validates title, content, and kind fields.
  - `quint_verify`: Confirms hypothesis exists in L0, validates verdict.
  - `quint_test`: Ensures hypothesis is in L1 (not L0), validates verdict.
  - `quint_audit`: Confirms hypothesis is in L2 before audit.
  - `quint_decide`: Requires L2 hypotheses exist, validates winner_id and title.
  - `quint_calculate_r` / `quint_audit_tree`: Validates holon existence in DB.
  - Precondition failures logged to audit_log with BLOCKED status.
  - Each error includes actionable suggestion for the user.

- **Command Contracts (FPF Enforcement)**:
  - All FPF command prompts (q0-q5) now include formal enforcement contracts.
  - YAML frontmatter with `pre`, `post`, `invariant`, and `required_tools` fields.
  - RFC 2119 bindings (MUST, MUST NOT, SHALL) for mandatory behaviors.
  - "Invalid Behaviors" section explicitly lists protocol violations.
  - "Checkpoint" section with verification checklist before phase transition.
  - Success/failure path examples with few-shot learning.
  - "State machine executor" mechanical persona to reduce AI improvisation.
  - Defense in depth: soft gate (prompts) + hard gate (preconditions).

- **Inline Schema Migrations**:
  - Existing databases automatically upgraded on startup.
  - Adds `parent_id` and `cached_r_score` columns to existing `holons` table.
  - Safe to run multiple times (idempotent).

- **Holon Linking in `quint_propose`**:
  - New `depends_on` parameter to declare dependencies on other holons.
  - New `decision_context` parameter to group alternatives under a decision.
  - New `dependency_cl` parameter (1-3) for congruence level of dependencies.
  - Creates `ComponentOf` relations for system holons, `ConstituentOf` for episteme.
  - Creates `MemberOf` relations for decision grouping (no R propagation).
  - Added SQL indexes for efficient WLNK traversal.
  - Documented structural relations (B.1.1) in CLAUDE.md.

- **Evidence Freshness Management (FPF B.3.4)**:
  - New `waivers` table for tracking temporary risk acceptance with full audit trail.
  - `quint_check_decay` now supports three modes:
    - **Report mode** (default): Shows freshness report with STALE/FRESH/WAIVED holons.
    - **Deprecate mode**: Downgrades hypothesis (L2→L1 or L1→L0) when evidence is terminally stale.
    - **Waive mode**: Records explicit risk acceptance with rationale and expiration date.
  - `quint_test` now accepts L2 hypotheses for evidence refresh (L2 + PASS stays L2 with fresh evidence).
  - Freshness report now shows individual evidence IDs (not just counts) for actionable output.
  - Implements WLNK principle: one expired evidence item = entire holon is STALE.
  - Natural language support: users can say "waive the benchmark until February" and the agent handles ID resolution.
  - New documentation: `docs/evidence-freshness.md` — practical guide to managing stale evidence.
  - Updated command documentation: `q-decay.md` and `q3-validate.md`.

- **CI/CD Pipeline**:
  - New GitHub Actions workflow (`.github/workflows/ci.yml`) for pull requests.
  - Triggers on PRs and pushes to `main` and `dev` branches.
  - Runs tests with race detector and coverage reporting.
  - Runs `golangci-lint` for code quality (errcheck, govet, staticcheck, unused, misspell).
  - Uses `golangci-lint-action@v7` with golangci-lint v2 config schema.
  - Added `.golangci.yml` configuration for consistent linting.

- **Pre-commit Hooks**:
  - Added `.pre-commit-config.yaml` for use with pre-commit tool.
  - Added `.githooks/pre-commit` for simple git-native hooks (no dependencies).
  - Hooks include: gofmt, goimports, go build, go test, golangci-lint.
  - Setup via `./scripts/setup-hooks.sh` or `./scripts/setup-hooks.sh --precommit`.
  - Also checks: trailing whitespace, end-of-file, yaml syntax, large files, merge conflicts, private keys.

### Changed

- **Updated FPF Commands**: Commands now leverage new MCP tools for computed data:
  - `/q4-audit`: Now calls `quint_calculate_r` and `quint_audit_tree` before recording findings.
  - `/q5-decide`: Now uses `quint_calculate_r` for final R_eff comparison before decision.
  - `/q-audit`: Updated to use visualization tools.
  - `/q-decay`: Updated to use `quint_check_decay` for proactive decay detection.
  - `/q-status`: Now proactively checks for expired evidence.

- **SQLite Driver Migration**: Replaced CGO-based `mattn/go-sqlite3` with pure Go `modernc.org/sqlite`.
  - Enables `CGO_ENABLED=0` builds for simplified cross-compilation.
  - Cross-compilation now works for linux/amd64, linux/arm64, darwin/*, windows/amd64.
  - Unblocks single-runner GoReleaser builds.
  - No functional changes to database behavior.

- **FSM Phase Derivation**: `CanTransition()` now uses `GetPhase()` (derived from DB) instead of `State.Phase`.
  - Phase transitions are validated against actual database state.
  - Hardens FPF enforcement against state.json manipulation.

### Fixed

- **Evidence Decay Bug**: Evidence was stored with `NULL` `valid_until`, making `/q-decay` always report "no expired evidence."
  - `ManageEvidence` now sets a default 90-day validity period when `validUntil` is empty.
  - Affects all evidence added via `quint_verify`, `quint_test`, and `quint_audit`.

- **Go Module Import Paths**: Standardized import paths to use the correct module name across all packages. (PR #16, @blib)

---

## [4.0.0] - 2025-12-18

### Added

- **MCP Server Architecture**:
  - This release introduces the MCP (Model Context Protocol) server as the core of Quint Code, replacing the previous prompt-only approach. The server provides structured tools for AI assistants to interact with the FPF knowledge base.
  - The MCP server has been restructured into `cmd` and `internal` packages for better organization and maintainability.
- **New Commands**:
  - `/q-audit`: Visualize assurance tree with R-scores.
  - `/q-decay`: Calculate epistemic debt from expired evidence.
- **Command Updates**:
  - `/q-actualize`: Reconcile the knowledge base with recent code changes. This command has been updated for better performance and accuracy.
- **Renamed Commands**:
  - `/q2-check` is now `/q2-verify`.
  - `/q3-research` and `/q3-test` are consolidated into `/q3-validate`.
  - `/q1-add` has been added for manually adding hypotheses.
- **SQLite Database (`quint.db`)**:
  - The project now uses SQLite for deterministic FPF, ensuring consistency and reproducibility.
  - `holons` table now includes `scope`, `kind`, and `cached_r_score` columns to support advanced FPF features.
  - `evidence` table now includes a `valid_until` column for evidence decay tracking.
  - `relations` table now includes a `congruence_level` column for WLNK calculation.
- **Trust & Assurance Calculator (B.3)**:
  - Implemented the FPF B.3 standard for calculating trust and assurance.
  - **Weakest Link (WLNK)**: R-score is now capped by the lowest-scoring dependency in the evidence chain.
  - **Congruence Penalty**: Low-congruence relations between artifacts now reduce the overall reliability score.
  - **Evidence Decay**: Expired evidence, as determined by the `valid_until` field, now penalizes the R-score, introducing the concept of "epistemic debt."
  - **Cycle Detection**: The calculator now detects and flags circular dependencies in the evidence graph.
- **Typed Reasoning (Kind-CAL)**:
  - Hypotheses are now classified by `kind` as either `system` (for code and architecture) or `episteme` (for knowledge and theory).
  - Validation logic now branches based on the hypothesis kind, allowing for more targeted and relevant analysis.
- **Characteristic Space (C.16)**:
  - Success metrics are now defined upfront, before testing, as part of the `Characteristic Space`.
  - These metrics are measured during the induction phase (`/q3-validate`).
  - The results are recorded in the Design Rationale Record (DRR) for full traceability.
- **CI/CD**:
  - A new GitHub Actions workflow has been added to automate the build and release process.
- **Testing**:
  - Added integration tests for the assurance calculator to ensure correctness and stability.
- **Error Handling**:
  - Improved error handling to surface previously-silent errors with warnings.

### Changed

- **Project Directory**: Renamed from `.fpf` to `.quint`. Migration: run `/q-actualize` to migrate, then delete `.fpf`.
- **Multi-platform Support**:
  - Installer now supports Claude Code, Cursor, Gemini CLI, and Codex CLI.
  - Commands are now sourced from `src/commands/` and built to platform-specific formats in `dist/`.

### Removed

- **Redundant Agents**: Removed legacy standalone agent files for a cleaner codebase.

---

## [3.4.0] - 2025-12-15

### Security: Executable Phase Gating

#### Physics-First Enforcement (`/q1-hypothesize`)

- **Vulnerability Closed:** Previous prompts used "soft" text instructions to prevent adding hypotheses mid-cycle, which "helpful" AI models would bypass.
- **Executable Gate:** Now injects a bash script that checks `.quint/session.md`. If the phase is locked (Deduction/Induction complete), the script exits with `1`.
- **Hard Stop:** The prompt explicitly instructs the AI to treat a script failure as a hard stop ("Physics says no"), preventing "helpfulness bias" overrides.

## [3.3.0] - 2025-12-15

### Added: Legacy Project Repair

#### Smart Initialization (`/q0-init`)

- **Self-Healing Capability:** The init command now detects incomplete FPF setups (e.g., legacy projects missing `context.md` from v2.x).
- **Deterministic Diagnostic:** Injects a bash script to verify file existence before deciding actions, preventing AI "hallucinated" skips.
- **Repair Mode:** If `.quint/` exists but is incomplete, it triggers a surgical repair (generating only missing files) while preserving existing session data.

## [3.2.0] - 2025-12-15

### Added: Process Hardening & Flexibility

#### Strict Phase Gating (FPF Integrity)

- **Hard Block in `/q1-hypothesize`:** Explicitly forbids generating new hypotheses if the cycle has passed Deduction. This prevents the "Helpfulness Bias" vulnerability where AI assistants might break process integrity to be "nice".
- **Conditional Logic in `/q2-check`:** The cycle phase now only advances to `DEDUCTION_COMPLETE` when *all* active L0 hypotheses are resolved. If any remain unchecked, the door stays open for extensions.

#### New Command: `/q1-extend`

- **Legitimate Extension Path:** A dedicated command to add a missed hypothesis during the `ABDUCTION_COMPLETE` phase.
- **Safety Rails:** Strictly blocked once `DEDUCTION_COMPLETE` is reached, ensuring evidence integrity (WLNK validity) during testing.

### Changed

- **Updated `/q-status`:** State machine visualization now includes the `(q1-extend)` loop.
- **Refined `/q3-test` & `/q3-research`:** Reinforced checks to ensure testing only happens after deduction is fully complete.

## [3.1.0] - 2025-12-14

### Added: Deep Reasoning Capabilities

#### Context Slicing (A.2.6)

- **Structured Context:** `.quint/context.md` is now structured into explicit slices:
  - **Slice: Grounding** (Infrastructure, Region)
  - **Slice: Tech Stack** (Language, Frameworks)
  - **Slice: Constraints** (Compliance, Budget, Team)
- **Context-Aware Init:** `/q0-init` now scans `package.json`, `Dockerfile`, etc., to auto-populate slices.

#### Explicit Role Injection (A.2)

- **Role-Swapping Prompts:** Commands now enforce specific FPF roles to prevent "AI drift":
  - `/q1-hypothesize`: **ExplorerRole** (Creative, Abductive)
  - `/q2-check`: **LogicianRole** (Strict, Deductive)
  - `/q4-audit`: **AuditorRole** (Adversarial, Normative)

#### Context Drift Analysis

- **New Audit Step:** `/q4-audit` now includes a mandatory **Context Drift Check**.
- **Validation:** Verifies that hypotheses generated in step 1 still match the constraints in step 4 (preventing "works on my machine" architecture).

### Changed

- **Command Prompts:** Updated `q0`, `q1`, `q2`, `q4` to enforce the new reasoning standards.

---

## [3.0.0] - 2025-12-14

### Major Breaking Change: Rebrand to Quint Code

**Crucible Code is now Quint Code.**

### Why the Name Change?

1. **Avoid Collision:** "Crucible" is an existing code review tool by Atlassian. We want a distinct identity.
2. **Not Just Code:** This tool melts *ideas* and *reasoning*, not just source code.
3. **The "Quintessence":** Anatoly Levenchuk described this project as a "distillate of FPF" (~5% of the full framework). It is the *quintessence*—the concentrated essence of the methodology.
4. **The Invariant Quintet:** FPF is built on five invariants (IDEM, COMM, LOC, WLNK, MONO). Quint Code enforces a rigid 5-step sequence (`q1`–`q5`) to preserve these invariants in your reasoning.

### Changed

- **Project Name**: `crucible-code` → `quint-code`
- **Commands Prefix**: `/fpf-*` → `/q*`
  - `/q0-init`
  - `/q1-hypothesize`
  - `/q2-check`
  - `/q3-test`
  - `/q3-research`
  - `/q4-audit`
  - `/q5-decide`
- **Utility Commands**:
  - `/fpf-status` → `/q-status`
  - `/fpf-query` → `/q-query`
  - `/fpf-decay` → `/q-decay`
  - `/fpf-discard` → `/q-reset` (Renamed to avoid tab-completion clash with decay)

### Migration Guide

1. **Delete old commands**: Run the uninstall script or manually delete `~/.claude/commands/fpf-*`.
2. **Install new commands**: Run `./install.sh`.
3. **Update mental model**: Think "Quintet" (5 invariants, 5 steps).

---

## [2.2.0] - 2025-12-14

### Added

#### Multi-Platform Support

- **Four AI coding tools supported**: Claude Code, Cursor, Gemini CLI, Codex CLI
- **Adapter-based build system**: Source commands in `src/commands/`, platform-specific outputs in `dist/`
- **Platform adapters**: Transform markdown to platform-specific formats (TOML for Gemini, etc.)

#### Interactive TUI Installer

- **`curl | bash` one-liner install**: `curl -fsSL https://...install.sh | bash -s -- -g`
- **Interactive platform selection**: Choose which AI tools to install commands for
- **Global and per-project modes**: `-g` flag for global install, default for project-local
- **Vim-style navigation**: Arrow keys and j/k for selection
- **Bash 3.x compatibility**: Works on macOS default shell (no associative arrays)

#### Uninstall Functionality

- **`--uninstall` flag**: Remove installed FPF commands
- **Auto-detection**: Finds commands in both global and local locations
- **Platform-specific cleanup**: Only removes selected platforms

#### CI/CD

- **GitHub Actions workflow**: Verifies `dist/` stays in sync with `src/commands/`
- **Build check on PR/push**: Fails if `./build.sh` produces uncommitted changes

#### Visual Improvements

- **Melted steel gradient**: Red → orange → yellow → white color scheme for ASCII banner
- **SVG banner for GitHub**: `assets/banner.svg` with same gradient colors
- **Cleaner TUI**: Simplified instructions, highlighted keys

### Changed

- **Directory structure**: Commands moved from `commands/` to `src/commands/` (source of truth)
- **Installation targets**: Installer copies from `dist/{platform}/` not source
- **README**: Updated with new install instructions and SVG banner

---

## [2.1.0] - 2025-12-13

### Added

#### Agentic Initialization

- **Smart `/fpf-0-init`**: Now scans the repository (package.json, Dockerfile, etc.) to infer tech stack.
- **Interactive Interview**: Asks the user clarifying questions about Scale, Budget, and Constraints to build a robust Context.
- **`.quint/context.md`**: New foundational file that grounds all reasoning in the project's specific reality.

#### Repository Context Integration

- **Context Awareness**: All commands (`hypothesize`, `research`, `test`) now read `.quint/context.md` to make decisions relevant.
- **CLAUDE.md Update**: Instructions for Claude to check `.quint/context.md` first.

#### Enhanced Hypothesis Structure

- **Formality (F-Score)**: Added `formality: [0-9]` to hypothesis frontmatter.
- **NQD Tags**: Added `novelty` and `complexity` to hypothesis frontmatter for diversity tracking.
- **Strict Method/Work Split**: Hypothesis body restructured into "1. The Method (Design-Time)" and "2. The Validation (Run-Time)" to enforce A.15.

#### Documentation

- **F-Score Definitions**: Added explanation of F0-F9 ranges to README.
- **Concepts**: Added simple explanations for NQD and Method vs. Work.

## [2.0.0] - 2025-12-13

### Added

#### ADI Cycle Strictness Documentation

- **Phase strictness clearly documented in README** with visual annotations in the cycle diagram
- Phases 1→2→3 marked as `(REQUIRED)` in diagram
- Phase 4 (Audit) marked as `(OPTIONAL - but recommended)`
- New "Phase Strictness" section explaining:
  - Sequential enforcement for phases 1-3
  - When skipping audit is acceptable vs. not recommended
  - Commands enforce prerequisites and error on invalid transitions
- Commands Reference table updated with "Required" column and footnotes

#### Phase Gate Enforcement

- **All commands now verify phase prerequisites** before executing
- Invalid phase transitions are blocked with clear error messages
- Phase state machine documented in `/fpf-0-init` and `/fpf-status`
- Transitions logged in session file for audit trail

#### Transformer Mandate Enforcement

- **Explicit "AWAIT HUMAN INPUT" sections** at all decision points
- `/fpf-1-hypothesize` now pauses for human approval of generated hypotheses
- `/fpf-5-decide` requires explicit human selection of winning hypothesis
- Clear separation: Claude generates options, human decides

#### WLNK Calculation in Audit

- **Quantitative R_eff calculation** with formula: `R_eff = R_base - Φ(CL)`
- Evidence chain table showing base R, congruence, penalty, and effective R
- Weakest link identification with specific evidence file reference
- Impact analysis on hypothesis reliability

#### Congruence Penalty System

- **Formal Φ(CL) penalty values**: High=0.00, Medium=0.15, Low=0.35
- Congruence assessment required for all external evidence
- Penalty table in `/fpf-3-research` and `/fpf-4-audit`
- Low-congruence evidence flagged as WLNK risk

#### Plausibility Filters

- **Four-filter assessment** in `/fpf-1-hypothesize`:
  - Simplicity (Occam's razor)
  - Explanatory Power (does it resolve the core problem?)
  - Consistency (compatible with L2 knowledge?)
  - Falsifiability (can we disprove it?)
- Plausibility verdict: PLAUSIBLE / MARGINAL / IMPLAUSIBLE
- Ranking table for hypothesis comparison

#### Enhanced Evidence Templates

- **Mandatory fields**: `valid_until`, `scope`, `congruence` (for external)
- Structured verdict section with checkboxes
- Re-test triggers documentation
- Environment and method reproducibility sections

#### Project Configuration

- **Optional `.quint/config.yaml`** for project-level settings
- Configurable validity defaults by evidence type
- Congruence penalty values customizable
- Epistemic debt thresholds

#### Improved Session Tracking

- **Phase transitions log** in session.md
- Valid phase transition diagram
- Previous cycle reference after completion
- State machine visualization in `/fpf-status`

#### Better Learning Preservation

- `/fpf-discard` now captures key insights before cleanup
- Optional learning note creation for significant findings
- Preservation options: L2-only (default), L1+, all, none
- "Don't repeat" section for mistakes to avoid

#### Documentation Improvements

- **"Common Mistakes to Avoid"** section in each command
- Anti-pattern tables with explanations
- Quality checklists for evidence and DRRs
- Quick start guide in README

### Changed

#### Command Structure

- All commands now start with Phase Gate section
- Consistent output format across commands
- Clearer section headers and structure
- More actionable next steps guidance

#### Hypothesis Template

- Added plausibility assessment table
- Scope section now has explicit applies_to / not_valid_for
- Weakest link analysis required
- Author attribution (Claude generated, Human reviewed)

#### Evidence Template

- Congruence assessment now mandatory for external evidence
- Validity window required with decay action
- Scope conditions more detailed
- Structured verdict with confidence level

#### DRR Template

- WLNK R_eff included in evidence summary
- Trade-off analysis table for alternatives
- Validity conditions with re-evaluation triggers
- Audit trail section with cycle statistics

#### Audit Command

- WLNK calculation now quantitative, not just qualitative
- Bias check more systematic with specific bias types
- Adversarial analysis section expanded
- Evidence quality audit with freshness check

#### Status Command

- Shows phase state machine diagram
- Evidence health summary
- Congruence warnings for low-CL evidence
- Quick status one-liner format

#### Query Command

- Confidence assessment for search results
- Validity status shown for each result
- Related decisions linked
- Pre-investigation check workflow

#### Decay Command

- Epistemic debt calculation
- Debt severity thresholds
- Impact on L2 claims shown
- Action items prioritized

### Removed

- Advisory-only checklists (replaced with mandatory gates)
- Vague "ensure" language (replaced with specific checks)

### Fixed

- Phase skipping now actually blocked, not just warned
- Human decision points clearly marked
- Evidence without validity no longer silently ages
- Congruence impact now quantified

---

## [1.0.0] - 2025-12-11

### Added

Initial release of Quint Code.

#### Core Commands

- `/fpf-0-init` — Initialize FPF structure
- `/fpf-1-hypothesize` — Generate hypotheses (Abduction phase)
- `/fpf-2-check` — Verify logical consistency (Deduction phase)
- `/fpf-3-test` — Internal empirical testing (Induction phase)
- `/fpf-3-research` — External evidence gathering (Induction phase)
- `/fpf-4-audit` — Critical review and WLNK analysis
- `/fpf-5-decide` — Finalize decision and create DRR

#### Utility Commands

- `/fpf-status` — Show current state
- `/fpf-query` — Search knowledge base
- `/fpf-decay` — Check evidence freshness
- `/fpf-discard` — Abandon cycle

#### Knowledge Structure

- L0/L1/L2/invalid assurance levels
- Evidence directory for test results and research
- Decisions directory for DRRs
- Sessions directory for archived cycles

#### Core Concepts

- ADI (Abduction-Deduction-Induction) cycle
- WLNK (Weakest Link) principle
- Congruence levels for external evidence
- Evidence validity windows
- Transformer Mandate (human decides)

#### Documentation

- README with usage guide
- CLAUDE.md template for project integration
- Installation script
- Examples for common scenarios

### Notes

This was the initial implementation based on the First Principles Framework (FPF) specification. The focus was on establishing the core workflow and making FPF accessible to developers through Claude Code commands.

Key design decisions:

- Commands over subagents (human must be in the loop)
- File-based persistence (git-trackable)
- Minimal tooling (no external dependencies)
- Advisory guidance (not enforced gates)

---

## Upgrade Notes

### 1.0.0 → 2.0.0

**Session file format changed.** Existing `.quint/session.md` files should be updated to include:

- Phase Transitions Log table
- Valid Phase Transitions diagram reference

**Evidence files should add:**

- `congruence:` block for external evidence
- `valid_until:` if not already present

**No breaking changes to:**

- Knowledge directory structure
- DRR format (only additions)
- Command names and basic arguments

**Recommended migration:**

1. Run `/fpf-decay` to identify evidence needing validity dates
2. Add congruence assessment to existing external evidence
3. No need to re-run completed cycles
