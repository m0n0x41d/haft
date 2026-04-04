<img src="assets/banner.svg" alt="Haft" width="600">

**Engineering decisions that know when they're stale.**

Frame problems. Compare options fairly. Record decisions as contracts. Know when to revisit.

Supports: Claude Code, Cursor, Gemini CLI, Codex CLI, Codex App, Air

---

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/m0n0x41d/quint-code/main/install.sh | bash
```

The install URL still points at the historical `quint-code` repository path. The installed binary is `haft`.

Then in your project, run init **with your tool's flag**:

```bash
# Claude Code (default if no flag)
haft init

# Claude Code with repo-local commands
haft init --local

# Cursor
haft init --cursor

# Gemini CLI
haft init --gemini

# Codex CLI / Codex App
haft init --codex

# JetBrains Air
haft init --air

# All tools at once
haft init --all
```

### What init does per tool

The binary is the same — only the MCP config and command/prompt installation locations differ:

| Tool | MCP Config | Commands / Prompts | Skill |
|------|-----------|--------------------|-------|
| Claude Code | `.mcp.json` (project root) | `~/.claude/commands/` or `.claude/commands/` with `--local` | `~/.claude/skills/h-reason/` or local install with `--local` |
| Cursor | `.cursor/mcp.json` | `~/.cursor/commands/` or `.cursor/commands/` with `--local` | `~/.cursor/skills/h-reason/` or local install with `--local` |
| Gemini CLI | `~/.gemini/settings.json` | `~/.gemini/commands/` or local install with `--local` | — |
| Codex CLI / Codex App | `.codex/config.toml` | `~/.codex/prompts/` or `.codex/prompts/` with `--local` | `~/.agents/skills/h-reason/` |
| Air | `.codex/config.toml` | project `skills/` | project `skills/h-reason/` |

**Important for Cursor:** After init, open Cursor Settings → MCP → find `haft` → enable the toggle. Cursor adds MCP servers as disabled by default.

**Note:** Cursor also picks up Claude Code commands from `~/.claude/commands/`, so slash commands can work even without `--cursor`. MCP config (`.cursor/mcp.json`) is still required for tool calls.

Existing project? Run `/h-onboard` after init — the agent scans your codebase for existing decisions worth capturing.

**First time?** Ask the agent to explain how it works:

```
/h-reason explain how to work with Haft effectively — what commands exist, when to use each one, and what's the recommended workflow
```

The agent has full knowledge of the project tools and will walk you through them in the context of your repo.

---

## Two supported interaction modes

### 1. MCP plugin / tool mode

Haft exposes six MCP tools:

| Tool | What it does |
|------|-------------|
| `haft_note` | Micro-decisions with validation + auto-expiry |
| `haft_problem` | Frame problems, define comparison dimensions with roles |
| `haft_solution` | Explore variants with diversity check, compare with parity |
| `haft_decision` | FPF E.9 decision contract, impact measurement, evidence/baseline lifecycle |
| `haft_refresh` | Lifecycle management for all artifacts |
| `haft_query` | Search, status dashboard, file-to-decision lookup, FPF spec search |

Use this mode when your client can call MCP tools directly.

### 2. Command-driven mode

Haft also installs slash commands / prompts such as `/h-reason`, `/h-frame`, `/h-explore`, `/h-decide`, `/h-status`, and `/h-refresh`.

Use this mode when the agent should be steered by explicit commands in chat. This remains supported alongside MCP tool mode; the two are complementary, not mutually exclusive.

---

## How It Works

### One command: `/h-reason`

Describe your problem. The agent frames it, generates alternatives, compares them fairly, and records the decision — all in one command. It auto-selects the right depth.

### Or drive each step manually

```
/h-frame  → /h-char  → /h-explore → /h-compare → /h-decide
  what's      what       genuinely     fair         engineering
  broken?     matters?   different     comparison   contract
                         options
```

### Micro-decisions on the fly

The agent captures decisions automatically when it notices them in conversation. No rationale — no record. Conflicts with active decisions are flagged. Auto-expires in 90 days.

### When decisions go stale

`/h-status` shows what's expired and what needs attention. `/h-refresh` manages the lifecycle of all artifact types — waive, reopen, supersede, or deprecate.

---

## What Makes It Different

- **Decisions are live** — they have computed trust scores (R_eff) that degrade as evidence ages. An expired benchmark drops the whole score.
- **Comparison is honest** — parity enforced, dimensions cross-checked, asymmetric scoring warned. Anti-Goodhart: tag dimensions as "observation" to prevent optimizing the wrong metric.
- **Memory across sessions** — when you frame a problem, the tool surfaces related past decisions. When you explore, it checks for similar variants.
- **The loop closes** — failed measurements suggest reopening. Evidence decay triggers review. Periodic refresh prompts ensure nothing goes stale silently.
- **Decisions are contracts** — FPF E.9 format: Problem Frame, Decision (invariants + DO/DON'T), Rationale, Consequences. A new engineer reads it 6 months later and gets everything.

---

## Built on First Principles Framework

[FPF](https://github.com/ailev/FPF) by [Anatoly Levenchuk](https://www.linkedin.com/in/ailev/) — a rigorous, transdisciplinary architecture for thinking.

`/h-reason` gives your AI agent an FPF-native operating system for engineering decisions: problem framing before solutions, characterization before comparison, parity enforcement, evidence with congruence penalties, weakest-link assurance, and the lemniscate cycle that closes itself when evidence ages or measurements fail.

`haft fpf search` gives you access to the indexed FPF specification. The retrieval path is local and tiered: exact pattern id lookup first, then route-aware concept matches, then bounded related-section expansion, then keyword fallback. In MCP-capable clients, the same retrieval core is available through `haft_query(action="fpf", query="...")`.

### Refresh the FPF index

```bash
task fpf-index
```

This rebuilds `internal/cli/fpf.db` from `data/FPF/FPF-Spec.md` and the route artifacts used during indexing. That SQLite database is a build artifact embedded into the `haft` binary, so after regenerating it, run `task build`, `task install`, or `task dev` before expecting a rebuilt binary to serve the new index. Use `haft fpf info` to inspect the embedded index provenance when debugging stale results.

### Query the indexed spec

Use exact pattern ids when you know the section, and route-style natural-language queries when you know the concept:

```bash
haft fpf search "A.6"
haft fpf search "a6:"
haft fpf search "boundary routing" --tier route --explain
haft fpf search "decision record" --full
haft fpf section "A.6"
haft fpf section "A.6 - Signature Stack & Boundary Discipline"
haft fpf info
```

Pattern ids are normalized, so common forms such as `A.6`, `a.6`, `A6`, and `A.6:` resolve to the same canonical section. `haft fpf section` supports exact lookup by heading or pattern id, while `haft fpf search` is the better entry point for route-aware discovery and explainable tiered search.

---

## Learn More

See the [documentation](https://quint.codes/learn) for detailed guides on decision modes, the DRR format, computed features, and lifecycle management.

## Requirements

- Go 1.25+ (for building from source)
- Any MCP-capable AI tool for direct tool mode
- Or a supported client that can use installed commands / prompts

## License

MIT
