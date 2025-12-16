---
id: h1-adapter-layer
type: hypothesis
created: 2025-12-14T12:15:00Z
problem: multi-platform-repo-structure
status: L1
deduction_passed: 2025-12-14T12:25:00Z
deduction_notes: |
  Passed logical consistency check.
  No contradictions with project constraints.
  Key risk: Gemini TOML transformation complexity — needs empirical verification.
  Implications acceptable with minor caveats (build friction, onboarding).
formality: 4
novelty: Conservative
complexity: Medium
author: Claude (generated), Human (to review)
scope:
  applies_to: "Projects needing to support 3+ agentic coding tools with different command formats"
  not_valid_for: "Single-platform projects or platforms with identical formats"
  scale: "11 command files × 3-4 platforms = ~40 generated files"
---

# Hypothesis: Adapter Layer with Build Step

## 1. The Method (Design-Time)

### Proposed Approach
Maintain canonical command files in a source format (Markdown), then use a build/transform step to generate platform-specific variants. Each platform gets its own adapter that transforms the canonical format into the target format (TOML for Gemini, YAML frontmatter for Codex, etc.).

### Rationale
This is the classic "single source of truth" pattern used in internationalization, documentation generation, and multi-platform SDKs. Changes to FPF methodology happen in one place; adapters handle the mechanical translation.

### Implementation Steps
1. Create `src/commands/` with canonical Markdown files (current format)
2. Create `adapters/` with transformation scripts per platform:
   - `adapters/claude.sh` — copy as-is to `.claude/commands/`
   - `adapters/cursor.sh` — copy as-is to `.cursor/commands/` (if compatible)
   - `adapters/gemini.sh` — transform Markdown → TOML
   - `adapters/codex.sh` — add YAML frontmatter
3. Create `build.sh` that runs all adapters, outputs to `dist/{platform}/`
4. Update `install.sh` to accept `--platform` flag, copy from `dist/`

### Expected Capability
- Single source of truth for FPF commands
- Platform-specific outputs generated automatically
- Easy to add new platforms (write one adapter)

## 2. The Validation (Run-Time)

### Plausibility Assessment

| Filter | Score | Justification |
|--------|-------|---------------|
| **Simplicity** | Medium | Adds build step, but centralizes changes |
| **Explanatory Power** | High | Solves the core problem of format divergence |
| **Consistency** | High | Compatible with "don't become bloated" if adapters are small |
| **Falsifiability** | High | Can test: do generated files work in each platform? |

**Plausibility Verdict:** PLAUSIBLE

### Assumptions to Verify
- [x] Cursor command format is identical to Claude Code (no transformation needed) — **SUPPORTED** (same Markdown, path change only)
- [x] Gemini TOML can express everything Markdown commands need — **SUPPORTED** (uses `{{args}}` instead of `$ARGUMENTS`)
- [x] Codex YAML frontmatter is additive (doesn't break existing content) — **SUPPORTED** (same syntax, optional frontmatter)
- [ ] Build step doesn't create maintenance burden — needs internal testing

### Required Evidence
- [ ] **Internal Test:** Create sample Gemini TOML from one FPF command, test in Gemini CLI
  - **Performer:** Developer
- [x] **Research:** Verify Cursor command format documentation
  - **Performer:** AI Agent
  - **Result:** Cursor uses same Markdown format, `.cursor/commands/` path

### External Evidence (from /fpf-3-research)

| Platform | Format | Compatibility | Evidence File |
|----------|--------|---------------|---------------|
| Cursor | Markdown | High — path change only | `evidence/2025-12-14-cursor-command-format.md` |
| Gemini CLI | TOML | Medium — requires transformer | `evidence/2025-12-14-gemini-cli-command-format.md` |
| Codex CLI | Markdown | Very High — same `$ARGUMENTS` syntax | `evidence/2025-12-14-codex-cli-command-format.md` |

**Key finding:** Adapters are simpler than anticipated. Gemini transformation is regex-based (no AST needed).

**Synthesis:** See `evidence/2025-12-14-platform-format-synthesis.md`

## Falsification Criteria
- If Gemini TOML cannot express multi-step prompts with argument injection, this fails — **NOT FALSIFIED** (uses `{{args}}`)
- If build step requires complex parsing (AST, not regex), complexity explodes — **NOT FALSIFIED** (regex sufficient)
- If platforms change formats frequently, adapter maintenance becomes unsustainable — **Not yet testable**

## Estimated Effort
**Revised:** Low-Medium — Simpler than expected
- Cursor: ~1 hour (path change in install script)
- Codex: ~2 hours (path change + optional frontmatter generator)
- Gemini: ~4 hours (Markdown→TOML transformer)

## Weakest Link
**Revised:** No longer Gemini TOML complexity.

New weakest link: **Empirical validation** — we have documentation evidence but haven't tested actual FPF commands in these platforms yet. Edge cases (10KB+ files, special characters) untested.
