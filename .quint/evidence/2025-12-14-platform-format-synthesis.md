---
id: platform-format-synthesis
type: external-research
source: synthesis
created: 2025-12-14T12:40:00Z
hypothesis: .quint/knowledge/L1/h1-adapter-layer-hypothesis.md
assumption_tested: "Multi-platform support is feasible with reasonable effort"
valid_until: 2025-06-14
decay_action: refresh
congruence:
  level: high
  penalty: 0.00
  source_context: "Combined analysis of Cursor, Gemini CLI, and Codex CLI"
  our_context: "Crucible Code FPF commands for Claude Code"
  justification: "Direct comparison of official documentation for all target platforms"
sources:
  - url: internal
    title: Synthesis of individual platform research
    type: synthesis
    accessed: 2025-12-14
    credibility: high
scope:
  applies_to: "Multi-platform FPF command support"
  not_valid_for: "Platforms not researched"
---

# Research Synthesis: Platform Command Format Comparison

## Purpose
Synthesize findings from individual platform research to assess overall multi-platform feasibility.

## Platform Comparison Matrix

| Feature | Claude Code | Cursor | Gemini CLI | Codex CLI |
|---------|-------------|--------|------------|-----------|
| **File Format** | Markdown | Markdown | TOML | Markdown |
| **Location (Project)** | `.claude/commands/` | `.cursor/commands/` | `.gemini/commands/` | N/A (global only) |
| **Location (Global)** | `~/.claude/commands/` | `~/.cursor/commands/` | `~/.gemini/commands/` | `~/.codex/prompts/` |
| **Invocation** | `/name` | `/name` | `/name` or `/ns:name` | `/prompts:name` |
| **All Args** | `$ARGUMENTS` | (appended) | `{{args}}` | `$ARGUMENTS` |
| **Positional** | `$1-$9` | (not documented) | (not documented) | `$1-$9` |
| **Frontmatter** | None | None | N/A (TOML native) | Optional YAML |
| **Extras** | — | — | `@{file}`, `!{shell}` | Named args `$KEY` |

## Compatibility Tiers

### Tier 1: Copy-Paste Compatible
**Codex CLI** — Nearly identical format
- Same `$ARGUMENTS` and `$1-$9` syntax
- Only change: directory path
- Optional: Add YAML frontmatter for better UX
- **Effort: Trivial**

### Tier 2: Path-Only Change
**Cursor** — Same format, different path
- Same Markdown format
- Unknown: Does Cursor respect `$ARGUMENTS` placeholder or just append?
- **Effort: Trivial (needs testing)**

### Tier 3: Format Transformation Required
**Gemini CLI** — TOML format
- Requires Markdown → TOML conversion
- Argument syntax change: `$ARGUMENTS` → `{{args}}`
- **Effort: Medium (build transformer)**

## Transformation Requirements

### Claude Code → Cursor
```bash
# Essentially just copy
cp -r .claude/commands/*.md .cursor/commands/
```

### Claude Code → Codex CLI
```bash
# Copy + optional frontmatter
# Could add description automatically
cp -r .claude/commands/*.md ~/.codex/prompts/
```

### Claude Code → Gemini CLI
```bash
# Requires transformation script
# Example pseudocode:
for file in .claude/commands/*.md; do
  name=$(basename "$file" .md)
  description=$(head -1 "$file" | sed 's/^# //')
  content=$(cat "$file")
  content=${content//\$ARGUMENTS/\{\{args\}\}}

  cat > ".gemini/commands/${name}.toml" << EOF
description = "$description"
prompt = """
$content
"""
EOF
done
```

## Key Findings

### 1. Format Convergence
Three of four platforms use Markdown. The industry is converging on Markdown as the standard for AI command prompts.

### 2. Argument Syntax Divergence
This is the main incompatibility:
- Claude Code & Codex: `$ARGUMENTS`, `$1-$9`
- Gemini: `{{args}}`
- Cursor: Unknown/appended

### 3. Gemini's Extra Features
Gemini CLI has unique capabilities that could enhance FPF:
- `@{path}` — Inject file content
- `!{cmd}` — Inject shell command output

This could enable richer hypothesis documentation or evidence injection.

### 4. Codex Global-Only Limitation
Codex CLI only supports global prompts (`~/.codex/prompts/`), not project-local. This means FPF commands would be user-wide, not project-specific.

## Implications for H1 (Adapter Layer)

The research **strongly supports H1**:

1. **Adapters can be simple:**
   - Cursor: Path change only (potentially zero transformation)
   - Codex: Path change + optional frontmatter
   - Gemini: Regex-based Markdown→TOML conversion

2. **Build step is lightweight:**
   - No AST parsing needed
   - Simple string replacements
   - Shell scripts sufficient (no npm/Node required)

3. **Single source of truth works:**
   - Claude Code Markdown can be canonical
   - All platforms derivable from it

## Implications for H2 (Symlinks)

Research **partially supports H2** but with caveats:
- Cursor may work with zero changes (needs testing)
- Codex requires path change at minimum
- Gemini cannot work with symlinks (different format)

**H2 is viable for Cursor/Codex but not for Gemini.**

## Verdict

| Hypothesis | Research Support | Confidence |
|------------|------------------|------------|
| H1 (Adapter Layer) | **Strong** | High |
| H2 (Symlinks, Gemini separate) | Partial | Medium |
| H4 (Per-platform dirs) | Unnecessary | N/A |

**Recommendation:** H1 is the best approach. Adapters are simpler than anticipated.

## Remaining Uncertainties

1. **Cursor argument handling** — Does `$ARGUMENTS` work or get ignored?
2. **Gemini positional args** — Can we do `$1`, `$2` equivalent?
3. **Long prompt handling** — Do 10KB+ FPF commands work in all platforms?

**Next Step:** Internal testing (`/fpf-3-test`) to validate these assumptions empirically.
