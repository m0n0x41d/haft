---
id: codex-cli-command-format
type: external-research
source: docs
created: 2025-12-14T12:35:00Z
hypothesis: .quint/knowledge/L1/h1-adapter-layer-hypothesis.md
assumption_tested: "Codex YAML frontmatter is additive (doesn't break existing content)"
valid_until: 2025-06-14
decay_action: refresh
congruence:
  level: high
  penalty: 0.00
  source_context: "OpenAI Codex CLI official documentation"
  our_context: "Crucible Code FPF commands for Claude Code"
  justification: "Same use case (AI coding assistant custom commands), official OpenAI documentation, nearly identical format to Claude Code"
sources:
  - url: https://github.com/openai/codex/blob/main/docs/prompts.md
    title: Codex CLI Prompts Documentation
    type: official-docs
    accessed: 2025-12-14
    credibility: high
  - url: https://developers.openai.com/codex/guides/slash-commands/
    title: OpenAI Developer Docs - Slash Commands
    type: official-docs
    accessed: 2025-12-14
    credibility: high
scope:
  applies_to: "OpenAI Codex CLI"
  not_valid_for: "Codex API directly (different interface)"
---

# Research: OpenAI Codex CLI Command Format

## Purpose
Verify Codex CLI prompt format compatibility with Claude Code, focusing on YAML frontmatter behavior.

## Hypothesis Reference
- **File:** `.quint/knowledge/L1/h1-adapter-layer-hypothesis.md`
- **Assumption tested:** Codex YAML frontmatter is additive (doesn't break existing content)

## Congruence Assessment

**Source context:** OpenAI Codex CLI, custom prompts
**Our context:** Claude Code CLI, custom commands (FPF methodology)

| Dimension | Match | Notes |
|-----------|-------|-------|
| Technology | ✓ | Same format (Markdown with optional YAML frontmatter) |
| Scale | ✓ | Same scale (individual developer tool) |
| Use Case | ✓ | Identical (custom AI prompts as slash commands) |
| Environment | ✓ | Both are CLI coding assistants |

**Congruence Level:** High
**Penalty:** 0.00
**R_eff:** 1.0

## Findings

### Source 1: Codex CLI Official Documentation

**URL:** https://github.com/openai/codex/blob/main/docs/prompts.md
**Type:** official-docs
**Credibility:** High
**Accessed:** 2025-12-14

**Format Structure:**

```markdown
---
description: Short description shown in slash popup
argument-hint: FILE=<path> [FOCUS=<section>]
---

Your prompt content here.

Use $ARGUMENTS for all args.
Use $1, $2, etc. for positional args.
Use $KEY for named args (KEY=value).
Use $$ for literal dollar sign.
```

**Key Features:**

| Feature | Syntax | Notes |
|---------|--------|-------|
| Location | `~/.codex/prompts/` | Global only (no project-local) |
| Format | Markdown (`.md`) | Same as Claude Code |
| Invocation | `/prompts:<name>` | Different prefix than Claude |
| All arguments | `$ARGUMENTS` | **Identical to Claude Code** |
| Positional | `$1` - `$9` | **Identical to Claude Code** |
| Named args | `$KEY` with `KEY=value` | Claude Code doesn't have this |
| Frontmatter | Optional YAML | Claude Code doesn't use this |

**Format comparison with Claude Code:**

| Aspect | Claude Code | Codex CLI | Transformation |
|--------|-------------|-----------|----------------|
| File format | Markdown | Markdown | None |
| Location | `.claude/commands/` | `~/.codex/prompts/` | Path change |
| Invocation | `/command-name` | `/prompts:name` | Different prefix |
| Arguments | `$ARGUMENTS` | `$ARGUMENTS` | **None** |
| Positional | `$1-$9` | `$1-$9` | **None** |
| Frontmatter | None | Optional | Additive only |

### Critical Finding: Near-Perfect Compatibility

**Codex CLI uses almost identical syntax to Claude Code:**
- Same `$ARGUMENTS` placeholder
- Same `$1-$9` positional arguments
- Same Markdown format

**Only differences:**
1. **Directory:** `~/.codex/prompts/` (global only, no project-local)
2. **Invocation prefix:** `/prompts:name` vs `/name`
3. **Optional frontmatter:** Can add `description` and `argument-hint`

### Frontmatter Behavior

**Frontmatter is truly additive:**
- If present, provides metadata for slash popup
- If absent, command still works
- Parser is "robust against malformed YAML"

**Example with frontmatter:**
```markdown
---
description: Initialize FPF for structured reasoning
argument-hint: (none required)
---

# FPF Initialization

[rest of prompt content unchanged]
```

## Synthesis

**Codex CLI is the MOST compatible platform** with Claude Code:

1. **Same argument syntax** — No transformation needed for `$ARGUMENTS` or `$1-$9`
2. **Same file format** — Pure Markdown
3. **Additive frontmatter** — Adding YAML metadata doesn't break anything

**Transformation is trivial:**
1. Copy `.md` files to `~/.codex/prompts/`
2. Optionally add YAML frontmatter for better UX in slash popup

## Verdict

- [x] Assumption **SUPPORTED** by external evidence (with congruence: high)
- [ ] Assumption **CONTRADICTED** by external evidence
- [ ] **MIXED** evidence — need internal testing to resolve
- [ ] **INSUFFICIENT** evidence — need more research
- [ ] **LOW CONGRUENCE** — supports hypothesis but verify internally

**Confidence:** Very High. Codex is essentially a Claude Code clone in terms of command format.

## Gaps

- Global-only location (`~/.codex/prompts/`) — no project-local commands
- Invocation prefix difference (`/prompts:name`) may confuse users

## Recommendations

1. **Minimal adapter** — Just copy files and optionally add frontmatter
2. **Consider frontmatter** — Adding `description` improves Codex UX
3. **Document invocation difference** — Users need to know `/prompts:fpf-status` not `/fpf-status`
