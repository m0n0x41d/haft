<img src="assets/banner.svg" alt="Haft" width="600">

*formerly [quint-code](https://github.com/m0n0x41d/quint-code)*

**Engineering decisions that know when they're stale.**

Frame problems. Compare options fairly. Record decisions as contracts. Know when to revisit.

---

## What is Haft?

Haft is a local-first engineering governor for software projects. It helps engineers frame problems before solving them, compare options honestly, record decisions as contracts with invariants, track evidence with decay, and know when to revisit.

**Think → Run → Govern.**

### Two primary surfaces

- **Desktop app** — visual cockpit for reasoning state, agent orchestration, and governance dashboard
- **MCP plugin** — reasoning tools for AI coding agents (Claude Code, Cursor, Gemini CLI, Codex, Air)

Both share the same kernel. Desktop is where humans think. MCP is where agents think.

> **Note:** The TUI (`haft agent`) and Desktop app are in **pre-alpha** and under active development. They are not recommended for production use. The MCP plugin mode (`haft serve`) is the stable, proven interface.

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

Existing project? Run `/h-onboard` after init — the agent scans your codebase for existing decisions worth capturing.

---

## How It Works

### Six MCP tools

| Tool | What it does |
|------|-------------|
| `haft_note` | Micro-decisions with validation + auto-expiry |
| `haft_problem` | Frame problems, define comparison dimensions with roles |
| `haft_solution` | Explore variants with diversity check, compare with parity |
| `haft_decision` | Decision contract with invariants, claims, evidence, baseline lifecycle |
| `haft_refresh` | Lifecycle management for all artifacts |
| `haft_query` | Search, status dashboard, file-to-decision lookup, FPF spec search |

### One command: `/h-reason`

Describe your problem. The agent frames it, generates alternatives, compares them fairly, and records the decision — all in one command. It auto-selects the right depth.

### Or drive each step manually

```
/h-frame  → /h-char  → /h-explore → /h-compare → /h-decide
  what's      what       genuinely     fair         engineering
  broken?     matters?   different     comparison   contract
                         options
```

### Evidence workflow

Attach evidence to decisions with `haft_decision(action="evidence", ...)`. Evidence has formality levels (F0-F3), congruence levels (CL0-CL3), and expiry dates. Trust scores (R_eff) degrade as evidence ages. Stale evidence triggers refresh.

Use `haft_decision(action="measure", ...)` for post-implementation verification. Pair with `haft_decision(action="baseline", ...)` to snapshot affected files before measuring.

---

## What Makes It Different

- **Decisions are live** — computed trust scores (R_eff) degrade as evidence ages
- **Comparison is honest** — parity enforced, constraint-aware Pareto elimination, anti-Goodhart observation indicators
- **Invariants linked to code** — knowledge graph maps decisions to modules via dependency graph
- **Memory across sessions** — related past decisions surface during framing, similar variants during exploration
- **The loop closes** — failed measurements reopen decisions, evidence decay triggers review, drift detection flags violations
- **Decisions are contracts** — invariants, claims with thresholds, rollback plan, valid-until date

---

## Desktop App (pre-alpha)

> **Warning:** The desktop app is in pre-alpha. Use at your own risk.

Built with Wails v2 (Go + React). Run with:

```bash
task desktop        # dev mode with hot reload
task desktop:build  # production .app bundle
task desktop:open   # build and open
```

Features: dashboard with governance findings, problem board, decision detail with evidence decomposition, portfolio comparison with Pareto front, task spawning, agent chat view, terminal panel, multi-project management, search (Cmd+K).

---

## Built on First Principles Framework

[FPF](https://github.com/ailev/FPF) by [Anatoly Levenchuk](https://www.linkedin.com/in/ailev/) — a rigorous, transdisciplinary architecture for thinking.

`/h-reason` gives your AI agent an FPF-native operating system for engineering decisions: problem framing before solutions, characterization before comparison, parity enforcement, evidence with congruence penalties, weakest-link assurance, and the lemniscate cycle that closes itself when evidence ages or measurements fail.

`haft fpf search` gives access to the indexed FPF specification with tiered retrieval: exact pattern id → route-aware concept matching → keyword fallback.

---

## Requirements

- Go 1.25+ (for building from source)
- Any MCP-capable AI tool for plugin mode
- Wails v2 for desktop app (optional)

## License

MIT
