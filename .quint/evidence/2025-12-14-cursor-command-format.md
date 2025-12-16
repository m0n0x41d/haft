---
id: cursor-command-format
type: external-research
source: docs
created: 2025-12-14T12:35:00Z
hypothesis: .quint/knowledge/L1/h1-adapter-layer-hypothesis.md
assumption_tested: "Cursor command format is identical to Claude Code (no transformation needed)"
valid_until: 2025-06-14
decay_action: refresh
congruence:
  level: high
  penalty: 0.00
  source_context: "Cursor IDE custom commands documentation"
  our_context: "Crucible Code FPF commands for Claude Code"
  justification: "Same use case (AI coding assistant custom commands), same format (Markdown), same invocation pattern (slash commands)"
sources:
  - url: https://cursor.com/docs/agent/chat/commands
    title: Cursor Commands Documentation
    type: official-docs
    accessed: 2025-12-14
    credibility: high
  - url: https://github.com/hamzafer/cursor-commands
    title: Cursor Commands Community Examples
    type: community-repo
    accessed: 2025-12-14
    credibility: medium
  - url: https://ezablocki.com/posts/cursor-slash-commands/
    title: Cursor Slash Commands Blog Post
    type: tech-blog
    accessed: 2025-12-14
    credibility: medium
scope:
  applies_to: "Cursor IDE with Agent/Chat features"
  not_valid_for: "Older Cursor versions without command support"
---

# Research: Cursor Command Format

## Purpose
Verify whether Cursor's custom command format is compatible with Claude Code's format, determining if transformation is needed.

## Hypothesis Reference
- **File:** `.quint/knowledge/L1/h1-adapter-layer-hypothesis.md`
- **Assumption tested:** Cursor command format is identical to Claude Code (no transformation needed)

## Congruence Assessment

**Source context:** Cursor IDE, AI coding assistant, custom slash commands
**Our context:** Claude Code CLI, AI coding assistant, custom slash commands (FPF methodology)

| Dimension | Match | Notes |
|-----------|-------|-------|
| Technology | ✓ | Both use Markdown files for commands |
| Scale | ✓ | Same scale (individual developer tool) |
| Use Case | ✓ | Identical (custom AI prompts as slash commands) |
| Environment | ✓ | Both are coding assistants with file-based commands |

**Congruence Level:** High
**Penalty:** 0.00
**R_eff:** 1.0

## Findings

### Source 1: Cursor Official Documentation

**URL:** https://cursor.com/docs/agent/chat/commands
**Type:** official-docs
**Credibility:** High
**Accessed:** 2025-12-14

**Key points:**
- Commands stored as **plain Markdown files** (`.md`)
- **Two locations:** `.cursor/commands/` (project) or `~/.cursor/commands/` (global)
- **Invocation:** `/command-name` in chat
- **Arguments:** Text after command name included in prompt
- **No frontmatter required** — pure Markdown content

**Format comparison with Claude Code:**

| Aspect | Claude Code | Cursor | Compatible? |
|--------|-------------|--------|-------------|
| File format | `.md` | `.md` | ✓ Yes |
| Location | `.claude/commands/` | `.cursor/commands/` | ✓ Path change only |
| Arguments | `$ARGUMENTS`, `$1-$9` | Text appended to prompt | ⚠ Different |
| Frontmatter | None | None | ✓ Yes |
| Invocation | `/command-name` | `/command-name` | ✓ Yes |

### Source 2: Community Examples

**URL:** https://github.com/hamzafer/cursor-commands
**Type:** community-repo
**Credibility:** Medium

**Key points:**
- Confirms directory structure: `.cursor/commands/*.md`
- Commands work as "AI-driven shortcuts"
- Version controlled with git
- Example commands: `review-code.md`, `create-pr.md`, `security-audit.md`

## Synthesis

**Cursor format is nearly identical to Claude Code format.** The only differences are:

1. **Directory path:** `.cursor/commands/` vs `.claude/commands/`
2. **Argument handling:** Cursor appends text after command name; Claude Code uses explicit `$ARGUMENTS` placeholder

**Critical finding:** If FPF commands use `$ARGUMENTS` or positional placeholders, they may need adjustment for Cursor OR Cursor may simply ignore them and append arguments anyway. Testing required.

## Verdict

- [x] Assumption **SUPPORTED** by external evidence (with congruence: high)
- [ ] Assumption **CONTRADICTED** by external evidence
- [ ] **MIXED** evidence — need internal testing to resolve
- [ ] **INSUFFICIENT** evidence — need more research
- [ ] **LOW CONGRUENCE** — supports hypothesis but verify internally

**Confidence:** High for format compatibility. Medium for argument placeholder behavior — needs empirical test.

## Gaps

- Cursor documentation doesn't explicitly document argument placeholder syntax (`$ARGUMENTS`)
- Unknown if Cursor ignores or errors on Claude-style placeholders

## Recommendations

1. **Simple path adaptation** — Copy files, change directory name
2. **Test argument handling** — Try FPF command with `$ARGUMENTS` in Cursor
3. **If placeholders don't work** — Document that arguments are appended, not injected
