---
id: h2-symlink-same-format
type: hypothesis
created: 2025-12-14T12:15:00Z
problem: multi-platform-repo-structure
status: L0
deduction_result: conditional
conditions_needed: |
  - Clarify Gemini priority: Is it "nice to have" (H2 passes) or "required first-class" (H2 fails)?
  - If Gemini is truly needed for "complex reasonings with full FPF in context," second-class treatment is a strategic mismatch.
formality: 3
novelty: Minimal
complexity: Low
author: Claude (generated), Human (to review)
scope:
  applies_to: "Platforms with identical or near-identical command formats"
  not_valid_for: "Platforms requiring format transformation (Gemini TOML)"
  scale: "Works for Claude + Cursor + Codex; Gemini excluded or handled separately"
---

# Hypothesis: Symlinks for Compatible Platforms, Separate Gemini

## 1. The Method (Design-Time)

### Proposed Approach
Keep current `commands/` structure unchanged. For platforms with identical formats (Claude, Cursor, likely Codex), use symlinks or direct copies. Handle Gemini as a special case with a separate `gemini-commands/` directory maintained manually or with a minimal converter.

### Rationale
The simplest solution that could work. If Cursor and Codex accept the same Markdown format as Claude Code, no transformation is needed — just different install paths. Only Gemini requires real work. Don't build infrastructure for a problem that mostly doesn't exist.

### Implementation Steps
1. Test: Copy `commands/*.md` to `.cursor/commands/` — verify they work
2. Test: Copy `commands/*.md` to `.codex/prompts/` with minimal frontmatter — verify
3. If both work: Update `install.sh` with `--platform {claude|cursor|codex}` flag
4. For Gemini: Create `gemini-commands/` with TOML versions, maintain manually or with simple script
5. Document which platforms are "first-class" (auto) vs "manual" (Gemini)

### Expected Capability
- Zero build step for 3/4 platforms
- Gemini supported but with explicit maintenance cost
- Minimal repo changes

## 2. The Validation (Run-Time)

### Plausibility Assessment

| Filter | Score | Justification |
|--------|-------|---------------|
| **Simplicity** | High | No build step, no adapters for most platforms |
| **Explanatory Power** | Medium | Doesn't fully solve Gemini; treats it as exception |
| **Consistency** | High | Aligns with "don't become bloated" |
| **Falsifiability** | High | Trivial to test: do files work in Cursor/Codex? |

**Plausibility Verdict:** PLAUSIBLE

### Assumptions to Verify
- [ ] Cursor accepts `.md` files in `.cursor/commands/` with same syntax as Claude
- [ ] Codex accepts `.md` files with only additive YAML frontmatter
- [ ] Gemini TOML maintenance is acceptable (11 files, infrequent changes)

### Required Evidence
- [ ] **Internal Test:** Copy fpf-status.md to .cursor/commands/, invoke in Cursor
  - **Performer:** Developer
- [ ] **Internal Test:** Copy fpf-status.md to .codex/prompts/ with frontmatter, invoke
  - **Performer:** Developer
- [ ] **Research:** Check if Cursor/Codex have documented format differences
  - **Performer:** AI Agent

## Falsification Criteria
- If Cursor requires different argument syntax (`@` vs `$ARGUMENTS`), this fails
- If Codex frontmatter changes command semantics, this fails
- If Gemini is high-priority and manual maintenance is unacceptable, this is insufficient

## Estimated Effort
Low — hours to test and update install.sh; Gemini effort is deferred/manual

## Weakest Link
Gemini support is second-class. If the 1M context use case is important, this approach deprioritizes it.
