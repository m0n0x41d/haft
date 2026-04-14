<img src="assets/banner.svg" alt="Haft" width="600">

*formerly [quint-code](https://github.com/m0n0x41d/quint-code)*

**True harness engineering for AI-assisted software delivery.**

Your agents write code fast. Nobody checks if the decisions behind that code are any good — or still valid a month later. Haft does.

---

## What is Haft?

Haft is the engineering governor that sits between your intentions and your agents' execution. It enforces the discipline that separates "we shipped fast" from "we shipped right": frame the problem before solving it, compare options under parity, record decisions as falsifiable contracts, and know the moment assumptions go stale.

**Think → Run → Govern.**

Not a coding agent. Not a documentation tool. The handle between the tool and the hand — the part that turns raw capability into directed engineering work.

### Two primary surfaces

- **Desktop app** — visual cockpit for reasoning state, agent orchestration, and governance dashboard
- **MCP plugin** — reasoning tools for AI coding agents (Claude Code, Cursor, Gemini CLI, Codex, Air)

Both share the same kernel. Desktop is where humans think. MCP is where agents think.

> **Note:** The TUI (`haft agent`) and Desktop app are in **pre-alpha** and under active development. They are not recommended for production use. The MCP plugin mode (`haft serve`) is the stable, proven interface.

---

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/m0n0x41d/haft/main/install.sh | bash
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

### From decision to code: `haft run`

Once you have a decision, implement it:

```bash
haft run dec-20260414-001
```

Haft reads the decision's invariants, claims, affected files, and governing invariants from the knowledge graph — then spawns an agent (Codex or Claude) with full reasoning context. After execution, takes a baseline snapshot automatically.

```
/h-reason "redesign the caching layer"
  ↓ frame → explore → compare → decide
  ↓
haft run dec-20260414-001 --agent codex
  ↓ reads decision → builds prompt → spawns agent
  ↓ agent implements with invariants as guardrails
  ↓ baseline snapshot on completion
  ↓
haft check
  ↓ verify governance health
```

The same loop powers the desktop "Implement" button. CLI and desktop are two surfaces over one kernel.

### Evidence workflow

Attach evidence to decisions with `haft_decision(action="evidence", ...)`. Evidence has formality levels (F0-F3), congruence levels (CL0-CL3), and expiry dates. Trust scores (R_eff) degrade as evidence ages. Stale evidence triggers refresh.

Use `haft_decision(action="measure", ...)` for post-implementation verification.

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

## Roadmap

### v6.1 — Harden the Contract (shipped)

Decision quality enforcement before automating execution:
- `haft check` for local governance verification (exit 0 = clean, exit 1 = findings)
- `/h-verify` surfaces full governance state (problems, invariants, drift)
- `.haft/workflow.md` — repo-level agent policy, injected into every prompt
- Problem typing (optimization / diagnosis / search / synthesis)
- G1 enforced (one decision per problem), G2/G4 warnings (parity plan, subjective dimensions)
- Claim-scoped R_eff, evidence supersession, CL0 rejection
- Deep `/h-onboard` with module-by-module analysis for legacy projects

### v6.2 — Dashboard + Execution Primitives (next)

The desktop becomes an operator surface:
- **Unified Dashboard** — decisions, governance findings, automations in one view
- **Implement** — click a decision, agent spawns in worktree with full reasoning context
- **Adopt** — governance finding → agent thread for interactive resolution
- **Automation triggers** — CI fail, dependency update, scheduled → auto-create ProblemCards
- **DecisionRecord→Task Pipeline** — Implement generates subtasks from decision, auto-advance mode

### v7 — Desktop Loop MVP

One proved cycle: **Decision → Implement → Verify → Baseline → PR draft**. Verification failure → reopen as ProblemCard.

### v8 — Governor Signals

Background detection loops (stale, drift, dependencies) with dashboard alerts. Autonomous actuation after trust is earned.

---

## Requirements

- Go 1.25+ (for building from source)
- Any MCP-capable AI tool for plugin mode
- Wails v2 for desktop app (optional)

## License

MIT
