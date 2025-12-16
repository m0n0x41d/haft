---
description: "Synchronize project state, migrate legacy structures, and reconcile evidence (FPF Maintenance)"
arguments: []
---

# FPF Actualizer

## Purpose

The **Actualizer** agent ensures that the FPF project state matches reality. It handles migrations (e.g., `.fpf` -> `.quint`), reconciles evidence with codebase changes, and discovers new context.

## Workflow

### 1. Run Actualize
Run:
```bash
./src/mcp/quint-mcp -action actualize
```

### 2. Capabilities

#### A. Migration
- Detects legacy `.fpf` directory.
- Renames `.fpf` to `.quint`.
- Migrates `fpf.db` to `quint.db`.
- Updates configuration paths.

#### B. Reconciliation (Drift Detection)
- **Git Tracking:** Compares current git commit with the last FPF cycle's commit.
- **File Drift:** Lists files changed since the last cycle.
- Scans `context.md` slices against the current codebase.
- Checks if file paths referenced in **Evidence** still exist.
- Flags "Zombie Evidence" (evidence for files that were deleted).

#### C. Discovery
- Identifies new potential **Context Slices** (e.g., new `Dockerfile` or `go.mod` found).
- Suggests updates to `.quint/context.md`.

## Output

The command will output a report of actions taken and suggestions for the user:

```markdown
## Actualization Report

### Migration
- [x] Renamed .fpf -> .quint
- [x] Migrated database

### Reconciliation
- **Context Drift:** Changes since `a1b2c3d`
  - M src/main.go
  - A src/new_module.go
- ⚠ Evidence `evidence/2024-01-01-test.md` references missing file `src/old_module.go`.
  -> Suggested: Deprecate evidence.

### Discovery
- ℹ Found new `rust-project.toml`. Add Rust to Tech Stack slice?
```
