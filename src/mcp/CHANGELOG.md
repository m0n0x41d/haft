# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- **approach_type parameter for quint_propose**: New optional parameter to classify hypothesis approaches (e.g., "conservative", "novel", "incremental", "radical", "hybrid") for NQD-CAL diversity tracking per FPF spec C.18
- **Approach diversity warnings**: When all hypotheses in a decision context share the same approach_type, a warning is displayed during internalize to encourage exploring alternative approaches
- **formality_level parameter for evidence**: New parameter to track evidence formality (F1-F5) per FPF spec, affecting F_eff calculation in reliability scores

### Changed

- `quint_propose` now accepts optional `approach_type` parameter
- `quint_verify` and `quint_test` now accept optional `formality_level` parameter
- Decision context summaries now include diversity warnings when applicable

### Database

- Migration 11: Added `approach_type` column to holons table with index
- Migration 10: Added `formality_level` column to evidence table
