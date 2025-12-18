<img src="assets/banner.svg" alt="Quint Code" width="600">

**Structured reasoning for AI coding tools** — make better decisions, remember why you made them.

**Supports:** Claude Code, Cursor, Gemini CLI

> **Works exceptionally well with Claude Code!**

## The Problem

Your AI coding assistant gives you an answer. It looks right. But in three months, will you remember *why* it was the right choice? What alternatives were considered? What evidence was there? Was the decision based on a solid foundation or a statistical fluke?

Unauditable AI suggestions create a debt of hidden risk.

Quint Code provides the structure to turn AI-assisted development into a rigorous, auditable reasoning process. It's decision hygiene for the age of AI.

## Quick Start

### Install

The installation is **per-project by design**. FPF reasoning is always grounded in a specific **Bounded Context** (Pattern A.1.1)—in this case, your project directory. Quint Code operates within this context to ensure all decisions and evidence are relevant to the work at hand.

```bash
cd /path/to/your/project
curl -fsSL https://raw.githubusercontent.com/m0n0x41d/quint-code/main/install.sh | bash
```

The installer will create a `.quint/` directory, install the Quint MCP server, and add slash commands to your AI tool's config directories (e.g., `.claude/`, `.gemini/`).

### Initialize

```bash
/q0-init   # Scans context and initializes the knowledge base
/q1-hypothesize "How should we handle state sync across browser tabs?"
```

## How It Works

Quint Code implements the **[First Principles Framework (FPF)](https://ailev.livejournal.com/)** by Anatoly Levenchuk — a methodology for rigorous, auditable reasoning. The killer feature is turning the black box of AI reasoning into a transparent, evidence-backed audit trail.

The core cycle follows three modes of inference:

1.  **Abduction** — Generate competing hypotheses (don't anchor on the first idea).
2.  **Deduction** — Verify logic and constraints (does the idea make sense?).
3.  **Induction** — Gather evidence through tests or research (does the idea work in reality?).

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

MIT License. FPF methodology by [Anatoly Levenchuk](https://ailev.livejournal.com/).
