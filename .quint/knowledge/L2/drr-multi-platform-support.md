---
id: drr-multi-platform-support
type: decision-rationale-record
created: 2025-12-14T13:00:00Z
problem: multi-platform-repo-structure
status: L2
winning_hypothesis: h1-adapter-layer
decision_maker: Human
formality: 4
novelty: Conservative
complexity: Medium
scope:
  applies_to: "Crucible Code repository structure for multi-platform support"
  not_valid_for: "Other FPF implementations or unrelated projects"
  scale: "4 platforms (Claude Code, Cursor, Gemini CLI, Codex CLI)"
---

# Decision Rationale Record: Multi-Platform Repository Structure

## Decision Summary

**Problem:** How should crucible-code's repository structure support multiple agentic coding tools (Cursor, Gemini CLI, Codex CLI) beyond Claude Code?

**Decision:** Implement Adapter Layer with Build Step (H1)

**Decision Maker:** Human (per Transformer Mandate)

**Date:** 2025-12-14

## Winning Hypothesis

### H1: Adapter Layer with Build Step

Maintain canonical command files in a source format (Markdown), then use a build/transform step to generate platform-specific variants.

**Structure:**
```
crucible-code/
├── src/commands/           # Canonical Markdown (Claude Code format)
├── adapters/
│   ├── claude.sh          # Copy as-is
│   ├── cursor.sh          # Copy as-is (path change)
│   ├── gemini.sh          # Markdown → TOML
│   └── codex.sh           # Copy + optional frontmatter
├── build.sh               # Runs all adapters
├── dist/{platform}/       # Generated outputs
└── install.sh             # curl | bash entry point with TUI
```

## Why This Hypothesis

### Evidence Summary

| Platform | Format | Transformation | Effort |
|----------|--------|----------------|--------|
| Claude Code | Markdown | None (source) | — |
| Cursor | Markdown | Path change only | Trivial |
| Codex CLI | Markdown | Path + optional frontmatter | Trivial |
| Gemini CLI | TOML | Regex-based converter | Medium |

**WLNK R_eff:** 1.0 (all high-congruence official documentation)

### Audit Results

- **Blockers:** 0
- **Warnings:** 3 (acceptable)
  1. No internal test yet (Gemini TOML conversion)
  2. TOML escaping edge cases
  3. Windows shell compatibility
- **Recommendation:** PROCEED

## Rejected Alternatives

### H2: Symlinks for Compatible, Separate Gemini
- **Status:** Superseded by H1
- **Reason:** H1 handles all platforms uniformly; symlinks create OS-specific issues

### H3: Universal Command Specification Format
- **Status:** Invalid
- **Reason:** YAGNI + meta-meta abstraction (creating a spec for specs)

### H4: Monorepo with Per-Platform Directories
- **Status:** Invalid
- **Reason:** User requirement for unified repo; violates DRY principle

## Implementation Plan

1. **Phase 1: Structure**
   - Create `src/commands/` directory
   - Move existing `.claude/commands/` content to `src/commands/`
   - Create `adapters/` directory with placeholder scripts

2. **Phase 2: Adapters**
   - `adapters/claude.sh` — Copy to `.claude/commands/`
   - `adapters/cursor.sh` — Copy to `.cursor/commands/`
   - `adapters/codex.sh` — Copy to `~/.codex/prompts/` + optional frontmatter
   - `adapters/gemini.sh` — Convert Markdown → TOML

3. **Phase 3: Build System**
   - Create `build.sh` that runs all adapters
   - Output to `dist/{platform}/`

4. **Phase 4: TUI Installer**
   - Create `install.sh` with interactive TUI
   - Support `curl | bash` installation
   - Let users select which platforms to install

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Gemini TOML edge cases | Test with complex FPF commands |
| Platform format changes | Adapters are isolated, easy to update |
| Windows compatibility | Document as Linux/macOS first |

## Success Criteria

1. All FPF commands work in all 4 platforms
2. Single source of truth maintained
3. `curl | bash` installer works smoothly
4. Adding new platform requires only new adapter

## Evidence Files

- `evidence/2025-12-14-cursor-command-format.md`
- `evidence/2025-12-14-gemini-cli-command-format.md`
- `evidence/2025-12-14-codex-cli-command-format.md`
- `evidence/2025-12-14-platform-format-synthesis.md`

## Post-Decision Notes

User added requirement during decision: TUI installer with "dynamic and astonishing" interface for platform selection.
