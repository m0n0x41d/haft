# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- **DB-backed context management in quint_internalize**: New remember/forget/overwrite parameters for structured context management. Context stored in SQLite `context_facts` table, with context.md auto-regenerated as projection.
  - `remember={category, content}` - Append content to a category
  - `forget="category"` - Remove a category
  - `overwrite={category, content}` - Replace category content entirely
- **L/A/D/E Boundary Norm Square for Contract structure**: Contract now uses FPF A.6.B quadrant structure (Laws/Admissibility/Deontics/Evidence) with teaching prompts in `quint_implement` output
- **Teaching prompts in implementation directives**: `quint_implement` now explains each L/A/D/E quadrant's purpose to guide agents toward comprehensive contract specification
- **Evidence quadrant (E) in contracts**: New field for specifying test strategies, observables, and verification methods
- **approach_type parameter for quint_propose**: New optional parameter to classify hypothesis approaches (e.g., "conservative", "novel", "incremental", "radical", "hybrid") for NQD-CAL diversity tracking per FPF spec C.18
- **Approach diversity warnings**: When all hypotheses in a decision context share the same approach_type, a warning is displayed during internalize to encourage exploring alternative approaches
- **formality_level parameter for evidence**: New parameter to track evidence formality (F1-F5) per FPF spec, affecting F_eff calculation in reliability scores

### Changed

- **Contract struct now uses L/A/D/E fields**: `laws`, `admissibility`, `deontics`, `evidence` replace old names while maintaining backward compatibility via getter methods
- **DRR markdown files now use L/A/D/E section headers**: Contract sections in decision files now labeled with quadrant names and descriptions
- **quint_implement output restructured**: Implementation directive now organized by L/A/D/E quadrants with teaching prompts
- `quint_propose` now accepts optional `approach_type` parameter
- `quint_verify` and `quint_test` now accept optional `formality_level` parameter
- Decision context summaries now include diversity warnings when applicable

### Backward Compatibility

- Old contract field names (`invariants`, `anti_patterns`, `acceptance_criteria`) continue to work and map to L/A/D/E via getter methods
- Existing DRRs with old contract format are fully supported

### Database

- Migration 12: Added `context_facts` table for DB-backed context management
- Migration 11: Added `approach_type` column to holons table with index
- Migration 10: Added `formality_level` column to evidence table
