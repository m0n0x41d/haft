---
id: h4-monorepo-per-platform
type: hypothesis
created: 2025-12-14T12:15:00Z
problem: multi-platform-repo-structure
status: invalid
invalidated: 2025-12-14T12:45:00Z
invalidation_reason: |
  User requirement: Keep crucible-code as a single unified repo.
  Per-platform directories with duplicated content violates DRY principle.
  No evidence that platforms need different content (only different formats).
  Learning: Duplication should be a last resort when transformation is impossible.
  H1 (adapter layer) achieves multi-platform without duplication.
formality: 3
novelty: Conservative
complexity: Medium
author: Claude (generated), Human (to review)
scope:
  applies_to: "Projects where platforms may diverge or have platform-specific features"
  not_valid_for: "Projects committed to identical behavior across platforms"
  scale: "4 directories, potential for drift"
---

# Hypothesis: Monorepo with Per-Platform Directories

## 1. The Method (Design-Time)

### Proposed Approach
Structure the repo with explicit directories per platform, each containing the full command set in native format. Share common documentation and `.quint/` structure, but commands are duplicated per platform. Use a sync script to propagate changes.

### Rationale
Sometimes platforms genuinely need different behavior. Gemini's 1M context might enable richer prompts. Cursor might have unique integrations. Per-platform directories allow divergence when needed while keeping everything in one repo.

### Implementation Steps
1. Restructure repo:
   ```
   crucible-code/
   ├── platforms/
   │   ├── claude/commands/*.md
   │   ├── cursor/commands/*.md
   │   ├── codex/prompts/*.md
   │   └── gemini/commands/*.toml
   ├── shared/
   │   └── fpf-structure/  (the .quint/ template)
   ├── docs/
   └── install.sh
   ```
2. Create `sync.sh` that diffs platforms and warns of divergence
3. Update `install.sh` to copy from `platforms/{platform}/`
4. Document which differences are intentional vs drift

### Expected Capability
- Explicit per-platform customization possible
- Easy to see what each platform gets
- No build step — just copy
- Divergence is visible and manageable

## 2. The Validation (Run-Time)

### Plausibility Assessment

| Filter | Score | Justification |
|--------|-------|---------------|
| **Simplicity** | Medium | More directories, but no build step |
| **Explanatory Power** | High | Handles both identical and divergent platforms |
| **Consistency** | Medium | Duplication conflicts with DRY; sync script mitigates |
| **Falsifiability** | High | Can test: does sync script catch drift? |

**Plausibility Verdict:** PLAUSIBLE

### Assumptions to Verify
- [ ] Platform-specific customization is actually needed (or likely)
- [ ] Sync script can reliably detect meaningful divergence
- [ ] Maintainers will actually run sync checks before releases
- [ ] Duplication overhead is acceptable

### Required Evidence
- [ ] **Internal Test:** Create minimal platform directories, run install for each
  - **Performer:** Developer
- [ ] **Research:** Do other multi-platform CLI tools use this pattern?
  - **Performer:** AI Agent

## Falsification Criteria
- If platforms never diverge, duplication is pure overhead
- If sync script becomes complex (semantic diff vs textual), maintenance burden grows
- If contributors forget to sync, platforms drift silently

## Estimated Effort
Medium — half a day to restructure, ongoing sync discipline required

## Weakest Link
The sync discipline. If the team doesn't consistently propagate changes, platforms will silently diverge. The "feature" of allowing divergence becomes a bug.
