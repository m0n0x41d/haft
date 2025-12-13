# Changelog

All notable changes to Crucible Code will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.1.0] - 2025-12-13

### Added

#### Repository Context

- **`.fpf/context.md`**: Created by `/fpf-0-init` to define the "Base Slice" (Tech Stack, Scale, Constraints).
- **Context Awareness**: All commands (`hypothesize`, `research`, `test`) now read this file to ground decisions.
- **CLAUDE.md Update**: Instructions for Claude to check `.fpf/context.md` first.

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
- **`.fpf/context.md`**: New foundational file that grounds all reasoning in the project's specific reality.

#### Repository Context Integration

- **Context Awareness**: All commands (`hypothesize`, `research`, `test`) now read `.fpf/context.md` to make decisions relevant.
- **CLAUDE.md Update**: Instructions for Claude to check `.fpf/context.md` first.

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

- **Optional `.fpf/config.yaml`** for project-level settings
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

Initial release of Crucible Code.

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

**Session file format changed.** Existing `.fpf/session.md` files should be updated to include:

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
