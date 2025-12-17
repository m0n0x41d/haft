# Changelog

All notable changes to Quint Code will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [4.1.0] - 2025-12-17

### Added: Architectural Dept Repayment

#### Typed Reasoning (C.3 Kind-CAL)
- **`quint_propose` Upgrade:** Now requires a `kind` argument (`system` vs `episteme`) to distinguish between architectural changes and knowledge/documentation changes.
- **Database Schema:** Added `kind` column to `holons` table.
- **Agent Instruction:** Abductor now explicitly classifies hypotheses.

#### Structured Decision Making (E.9 & C.16)
- **`quint_decide` Upgrade:** Deprecated the free-form `content` blob in favor of strict E.9 DRR fields:
  - `context`: Problem frame.
  - `decision`: The choice made.
  - `rationale`: Why it was chosen.
  - `consequences`: Risks and benefits.
  - `characteristics`: C.16 Characteristic Space metrics (e.g., "Cost: Low").
- **Agent Instruction:** Decider now enforced to provide structured rationale.

---

## [4.0.0] - 2025-12-17

### The Agentic Kernel Update

This release fundamentally restructures Quint Code to strictly adhere to the First Principles Framework (FPF) by establishing the MCP server as the authoritative kernel and transforming CLI commands into lightweight entry points for specialized Agents.

### Breaking Changes
- **Project Directory:** Renamed from `.fpf` to `.quint` to align with the project name.
- **Database Schema:** `quint.db` schema updated to enforce FPF invariants.
  - Added `scope` to `holons` table (Pattern A.2.6).
  - Added `assurance_level` and `carrier_ref` to `evidence` table (Patterns B.3, A.10).
- **Command Architecture:** Commands (`/q1`, `/q2`, etc.) no longer contain agent prompts. They now strictly perform a `quint_transition` and instruct the LLM to adopt a persona defined in `.quint/agents/`.

### Added
- **Formal Agent Definitions:**
  - Dedicated agent profiles in `src/agents/`: `abductor.md`, `deductor.md`, `inductor.md`, `decider.md`, `auditor.md`.
  - Agents now have explicit instructions on how to use MCP tools to satisfy FPF obligations.
- **MCP Tooling Enhancements:**
  - **`quint_transition`:** Replaces ad-hoc phase changes. Requires `role`, `target` phase, and `evidence_stub` to validate the transition.
  - **`quint_propose`:** Now requires `scope` (Claim Scope/G) to prevent scope drift.
  - **`quint_evidence`:** Now requires `assurance_level` (L0/L1/L2) and `carrier_ref` (Symbol Carrier) to prevent trust inflation and hearsay.
  - **`quint_loopback`:** Explicit tool for the Induction -> Deduction refinement cycle.
- **Migration Utility:**
  - **`/q-actualize`:** New command to migrate legacy `.fpf` projects to `.quint` structure and schema.

### Changed
- **Evidence Graph Referring (A.10):** Evidence must now be anchored to a physical file or log (`carrier_ref`). The system no longer accepts "I checked it" as valid evidence; it demands "I checked file X".
- **Trust & Assurance (B.3):** Verdicts are no longer binary (PASS/FAIL). They are now graded by Assurance Level (L0=Unsubstantiated, L1=Substantiated, L2=Axiomatic/Validated).
- **Unified Scope Mechanism (A.2.6):** Hypotheses cannot be proposed without an explicit Context Slice (Scope).
- **Installer:** `install.sh` now sources agents from `src/agents`.

### Fixed
- **Role Spoofing:** `quint_transition` enforces that the `role` matches the valid actors for the current phase.
- **Logic Gaps:** The **Deductor** agent is now explicitly responsible for deriving "Necessary Consequences" before testing begins.
- **Hypothesis Zombie State:** The **Auditor** agent includes checks for orphaned L0 hypotheses.

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

1.  **Avoid Collision:** "Crucible" is an existing code review tool by Atlassian. We want a distinct identity.
2.  **Not Just Code:** This tool melts *ideas* and *reasoning*, not just source code.
3.  **The "Quintessence":** Anatoly Levenchuk described this project as a "distillate of FPF" (~5% of the full framework). It is the *quintessence*—the concentrated essence of the methodology.
4.  **The Invariant Quintet:** FPF is built on five invariants (IDEM, COMM, LOC, WLNK, MONO). Quint Code enforces a rigid 5-step sequence (`q1`–`q5`) to preserve these invariants in your reasoning.

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

#### Repository Context

- **`.quint/context.md`**: Created by `/fpf-0-init` to define the "Base Slice" (Tech Stack, Scale, Constraints).
- **Context Awareness**: All commands (`hypothesize`, `research`, `test`) now read this file to ground decisions.
- **CLAUDE.md Update**: Instructions for Claude to check `.quint/context.md` first.

#### Enhanced Hypothesis Structure

- **Formality (F-Score)**: Added `formality: [0-9]` to hypothesis frontmatter.
- **NQD Tags**: Added `novelty` and `complexity` to hypothesis frontmatter for diversity tracking.
- **Strict Method/Work Split**: Hypothesis body restructured into "1. The Method (Design-Time)" and "2. The Validation (Run-Time)" to enforce A.15.

#### Documentation

- **F-Score Definitions**: Added explanation of F0-F9 ranges to README.
- **TODOs**: Added roadmap items for deeper NQD and Method/Work integration.

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
