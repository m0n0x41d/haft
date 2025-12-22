<img src="assets/banner.svg" alt="Quint Code" width="600">

**Structured reasoning for AI coding tools** — make better decisions, remember why you made them.

**Supports:** Claude Code, Cursor, Gemini CLI, Codex CLI

> **Works exceptionally well with Claude Code!**

## What Quint Does

### 1. Makes You and AI Think Structurally
Instead of jumping to conclusions and solutions, your AI generates competing hypotheses, checks them logically, tests against evidence, then you decide. Everything is in your plain sight. 

### 2. Preserves Every Decision And Related Evidence
No more archaeology in chat history. Decisions live in `.quint/` —
queryable, auditable, yours.

## Quick Start

### Step 1: Install the Binary

```bash
curl -fsSL https://raw.githubusercontent.com/m0n0x41d/quint-code/main/install.sh | bash
```

Or build from source:

```bash
git clone https://github.com/m0n0x41d/quint-code.git
cd quint-code/src/mcp
go build -o quint-code .
sudo mv quint-code /usr/local/bin/
```

### Step 2: Initialize a Project

```bash
cd /path/to/your/project
quint-code init
```

This creates:

- `.quint/` — knowledge base, evidence, decisions
- `.mcp.json` — MCP server configuration
- `~/.claude/commands/` — slash commands (global by default)

**Flags:**

| Flag | MCP Config | Commands |
|------|-----------|----------|
| `--claude` (default) | `.mcp.json` | `~/.claude/commands/*.md` |
| `--cursor` | `.cursor/mcp.json` | `~/.cursor/commands/*.md` |
| `--gemini` | `~/.gemini/settings.json` | `~/.gemini/commands/*.toml` |
| `--codex` | `~/.codex/config.toml`* | `~/.codex/prompts/*.md` |
| `--all` | All of the above | All of the above |
| `--local` | — | Commands in project dir instead of global |

> **\* Codex CLI limitation:** Codex [doesn't support per-project MCP configuration](https://github.com/openai/codex/issues/2628). Run `quint-code init --codex` in **each project before starting work to switch the active project in global codex mcp config**.

### Step 3: Start Reasoning

```bash
/q0-init                           # Initialize knowledge base
/q1-hypothesize "Your problem..."  # Generate hypotheses
```

Here is a library of some [workflow examples](docs/workflow_example/) that might help you kick off with probing.

But really, it would be better to hack into it straight away and feel the flow. Shash commands have a numeric prefix for your convenience.

### Recommended: Add FPF Context to Your Agent Rules

For best results, we highly recommend using the [`CLAUDE.md`](CLAUDE.md) from this repository as a reference for your own project's agent instructions. It's optimized for software engineering work with FPF.

At minimum, copy the **FPF Glossary** section to your:
- `CLAUDE.md` (Claude Code)
- `.cursorrules` or `AGENTS.md` (Cursor)
- Agent system prompts (other tools)

This helps the AI understand FPF concepts like L0/L1/L2 layers, WLNK, R_eff, and the Transformer Mandate without re-explanation each session.

## How Quint Code Works

Quint Code implements the **[First Principles Framework (FPF)](https://github.com/ailev/FPF)** by Anatoly Levenchuk — a methodology for rigorous, auditable reasoning. The killer feature is turning the black box of AI reasoning into a transparent, evidence-backed audit trail.

The core cycle follows three modes of inference:

1. **Abduction** — Generate competing hypotheses (don't anchor on the first idea).
2. **Deduction** — Verify logic and constraints (does the idea make sense?).
3. **Induction** — Gather evidence through tests or research (does the idea work in reality?).

Then, audit for bias, decide, and document the rationale in a durable record.

See [docs/fpf-engine.md](docs/fpf-engine.md) for the full breakdown.

## Commands

| Command | What It Does |
|---------|--------------|
| `/q0-init` | Initialize `.quint/` and record project context. |
| `/q1-hypothesize` | Generate competing ideas for a problem. |
| `/q1-add` | Manually add your own hypothesis. |
| `/q2-verify` | Check logic and constraints — does it make sense? |
| `/q3-validate` | Test against evidence — does it actually work? |
| `/q4-audit` | Check for bias and calculate confidence scores. |
| `/q5-decide` | Pick the winner, record the rationale. |
| `/q-status` | Show the current state of the reasoning cycle. |
| `/q-query` | Search the project's knowledge base. |
| `/q-decay` | Find evidence that's gone stale and needs refresh. |
| `/q-actualize` | Reconcile the knowledge base with recent code changes. |
| `/q-reset` | Discard the current reasoning cycle. |

## Documentation

- [Workflow Examples](docs/workflow_example/) — Step-by-step walkthroughs
- [Quick Reference](docs/fpf-engine.md) — Commands and workflow
- [Advanced: FPF Deep Dive](docs/advanced.md) — Theory, glossary, tuning
- [Architecture](docs/architecture.md) — How it works under the hood

## License

MIT License. FPF methodology by Anatoly Levenchuk.
