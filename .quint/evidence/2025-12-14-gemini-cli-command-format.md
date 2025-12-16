---
id: gemini-cli-command-format
type: external-research
source: docs
created: 2025-12-14T12:35:00Z
hypothesis: .quint/knowledge/L1/h1-adapter-layer-hypothesis.md
assumption_tested: "Gemini TOML can express everything Markdown commands need"
valid_until: 2025-06-14
decay_action: refresh
congruence:
  level: high
  penalty: 0.00
  source_context: "Google Gemini CLI official documentation"
  our_context: "Crucible Code FPF commands for Claude Code"
  justification: "Same use case (AI coding assistant custom commands), official Google documentation, direct applicability"
sources:
  - url: https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/custom-commands.md
    title: Gemini CLI Custom Commands Documentation
    type: official-docs
    accessed: 2025-12-14
    credibility: high
scope:
  applies_to: "Gemini CLI with custom commands feature"
  not_valid_for: "Gemini API directly (different interface)"
---

# Research: Gemini CLI Command Format (TOML)

## Purpose
Verify whether Gemini CLI's TOML format can express all FPF command semantics, and understand the transformation required.

## Hypothesis Reference
- **File:** `.quint/knowledge/L1/h1-adapter-layer-hypothesis.md`
- **Assumption tested:** Gemini TOML can express everything Markdown commands need

## Congruence Assessment

**Source context:** Google Gemini CLI, custom commands
**Our context:** Claude Code CLI, custom commands (FPF methodology)

| Dimension | Match | Notes |
|-----------|-------|-------|
| Technology | ⚠ | Different format (TOML vs Markdown) |
| Scale | ✓ | Same scale (individual developer tool) |
| Use Case | ✓ | Identical (custom AI prompts as slash commands) |
| Environment | ✓ | Both are CLI coding assistants |

**Congruence Level:** High
**Penalty:** 0.00
**R_eff:** 1.0

## Findings

### Source 1: Gemini CLI Official Documentation

**URL:** https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/custom-commands.md
**Type:** official-docs
**Credibility:** High
**Accessed:** 2025-12-14

**TOML Format Structure:**

```toml
# Location: ~/.gemini/commands/[name].toml or .gemini/commands/[name].toml
# Namespaced: ~/.gemini/commands/[namespace]/[name].toml -> /namespace:name

description = "Short description shown in command list"

prompt = """
Your multi-line prompt content here.

You can use:
- {{args}} for all arguments
- @{path/to/file.md} for file injection
- !{shell command} for shell command output injection
"""
```

**Key Features:**

| Feature | Syntax | Example |
|---------|--------|---------|
| Arguments | `{{args}}` | `Review: {{args}}` |
| File injection | `@{path}` | `@{docs/guide.md}` |
| Shell injection | `!{cmd}` | `!{git diff --staged}` |
| Namespacing | `dir/name.toml` | `/git:commit` |

**Format comparison with Claude Code:**

| Aspect | Claude Code | Gemini CLI | Transformation |
|--------|-------------|------------|----------------|
| File format | Markdown | TOML | **Required** |
| Location | `.claude/commands/` | `.gemini/commands/` | Path change |
| Arguments | `$ARGUMENTS` | `{{args}}` | Syntax replace |
| Positional | `$1`, `$2`, etc. | Not documented | May not support |
| File inject | Not native | `@{path}` | N/A (Gemini extra) |
| Shell inject | Not native | `!{cmd}` | N/A (Gemini extra) |

### Transformation Algorithm

**Markdown → TOML conversion:**

1. Extract filename as command name
2. First paragraph or first line → `description`
3. Full content → `prompt = """..."""`
4. Replace `$ARGUMENTS` → `{{args}}`
5. Replace `$1`, `$2`, etc. → Need to test if supported

**Example transformation:**

**Input (Claude Code):**
```markdown
# Review Code

Review the following code for issues:

$ARGUMENTS

Focus on security and performance.
```

**Output (Gemini CLI):**
```toml
description = "Review Code"

prompt = """
Review the following code for issues:

{{args}}

Focus on security and performance.
"""
```

## Synthesis

**Gemini TOML CAN express FPF command semantics**, but requires transformation:

1. **Structure change:** Markdown → TOML wrapper
2. **Argument syntax:** `$ARGUMENTS` → `{{args}}`
3. **Positional args:** `$1-$9` may not have direct equivalent (needs testing)

**Bonus:** Gemini offers features Claude Code doesn't have:
- File injection (`@{path}`)
- Shell command injection (`!{cmd}`)

These could enable richer FPF prompts on Gemini (potential platform divergence case).

## Verdict

- [x] Assumption **SUPPORTED** by external evidence (with congruence: high)
- [ ] Assumption **CONTRADICTED** by external evidence
- [ ] **MIXED** evidence — need internal testing to resolve
- [ ] **INSUFFICIENT** evidence — need more research
- [ ] **LOW CONGRUENCE** — supports hypothesis but verify internally

**Confidence:** High for basic transformation. Medium for positional argument support.

## Gaps

- Positional arguments (`$1`, `$2`) support in Gemini unclear
- Multi-line prompt escaping edge cases
- Command argument passing semantics (how does `{{args}}` split?)

## Recommendations

1. **Build simple transformer** — Regex-based Markdown→TOML should work
2. **Test complex commands** — Try FPF's longest commands (fpf-1-hypothesize)
3. **Document Gemini extras** — File/shell injection could enhance FPF on Gemini
