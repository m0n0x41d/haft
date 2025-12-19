<img src="assets/banner.svg" alt="Quint Code" width="600">

**Structured reasoning for AI coding tools** — make better decisions, remember why you made them.

**Supports:** Claude Code, Cursor, Gemini CLI, Codex CLI

> **Works exceptionally well with Claude Code!**

## The Problem

Your AI coding assistant gives you an answer. It looks right. But in three months, will you remember *why* it was the right choice? What alternatives were considered? What evidence was there? Was the decision based on a solid foundation or a statistical fluke?

Unauditable AI suggestions create a debt of hidden risk.

Quint Code provides the structure to turn AI-assisted development into a rigorous, auditable reasoning process. It's decision hygiene for the age of AI.

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

## How It Works

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
| `/q0-init` | Initialize `.quint/` and record the Bounded Context. |
| `/q1-hypothesize` | Generate L0 hypotheses for a problem. |
| `/q1-add` | Manually add your own L0 hypothesis. |
| `/q2-verify` | Verify logic and constraints, promoting claims from L0 to L1. |
| `/q3-validate` | Gather empirical evidence, promoting claims from L1 to L2. |
| `/q4-audit` | Run an assurance audit and calculate trust scores. |
| `/q5-decide` | Select the winning hypothesis and create a Design Rationale Record. |
| `/q-status` | Show the current state of the reasoning cycle. |
| `/q-query` | Search the project's knowledge base. |
| `/q-decay` | Check for and report expired evidence (Epistemic Debt). |
| `/q-actualize` | Reconcile the knowledge base with recent code changes. |
| `/q-reset` | Discard the current reasoning cycle. |

## Documentation

- [FPF Engine Details](docs/fpf-engine.md) — ADI cycle, commands, when to use
- [Architecture](docs/architecture.md) — Internals, knowledge levels, Transformer Mandate

## License

MIT License. FPF methodology by [Anatoly Levenchuk](https://github.com/ailev/FPF).
