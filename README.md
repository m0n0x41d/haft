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

---

### Example: Handling Payment Confirmations

Your checkout works. Stripe charges the card.
But three weeks later, finance finds $12,000 in "ghost payments" —
customers charged but never got access.

The webhook endpoint returned 200. Logs look clean.
What went wrong?

#### Without Quint

Your AI suggests: *"Just add a webhook endpoint that activates the subscription"*

You ship it. It works in testing. Production looks fine.

Until it doesn't. Webhooks fail silently. Your endpoint timed out during a DB hiccup. Stripe retried, you processed it twice. A network blip ate three webhooks completely.

Now you're debugging production with no record of why you built it this way.

#### With Quint

```bash
$ q1 hypothesize "handle stripe payment confirmation"
```

AI generates competing approaches:

| # | Approach | Risk | Recovery |
|---|----------|------|----------|
| H1 | Webhook-only | Silent failures, no detection | None without manual audit |
| H2 | Webhook + sync processing | Timeout = lost event, retries = duplicates | Stripe retry (3 days) |
| H3 | Webhook → Queue + Polling backup | Complex, two code paths | Self-healing |

```bash
$ q2 verify
```

AI checks each hypothesis:
- **H1 fails:** "No mechanism detects missed webhooks"
- **H2 partial:** "Idempotency key needed, still misses network failures"
- **H3 passes:** "Polling catches what webhooks miss, queue handles spikes"

```bash
$ q5 decide
```

```
Decision: H3 — Async queue + 15-min polling reconciliation

Rationale:
- Webhook acknowledges immediately (200 in <100ms)
- Background job processes with idempotency check
- Polling job catches silent failures
- Accepted tradeoff: 15-min max delay for edge cases

Evidence: Stripe docs recommend polling backup.
Review trigger: If webhook success rate drops below 99%
```

#### 3 weeks later

Finance asks: *"Why do we poll every 15 minutes? Can we remove it?"*

```bash
$ q query "payment confirmation architecture"
```

```
Decision: 2024-01-15 — H3 selected over webhook-only

Key evidence:
- Stripe admits webhook delivery "not guaranteed"
- Polling catches ~0.3% of transactions (measured)
- Removing polling = ~$400/month in silent failures

Recommendation: Keep polling. Document in runbook.
```

**The decision context survives. No archaeology needed.**

---

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
/q-internalize                     # Orient yourself (init if needed)
/q1-hypothesize "Your problem..."  # Generate hypotheses
```

> **Pro tip:** For best results, see [Advanced Setup](docs/advanced.md#agent-configuration) to optimize your AI's understanding of the reasoning process.

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
| `/q-internalize` | **Start here.** Initialize, update context, show state. |
| `/q1-hypothesize` | Generate competing ideas for a problem. |
| `/q1-add` | Manually add your own hypothesis. |
| `/q2-verify` | Check logic and constraints — does it make sense? |
| `/q3-validate` | Test against evidence — does it actually work? |
| `/q4-audit` | Check for bias and calculate confidence scores. |
| `/q5-decide` | Pick the winner, record the rationale. |
| `/q-resolve` | Record decision outcome (implemented/abandoned/superseded). |
| `/q-query` | Search the project's knowledge base. |
| `/q-reset` | Discard the current reasoning cycle. |

**Note:** `/q-internalize` replaces the old `/q0-init`, `/q-status`, `/q-actualize`, and `/q-decay` commands. It's your single entry point for every session.

## Documentation

- [Quick Reference](docs/fpf-engine.md) — Commands and workflow
- [Advanced: FPF Deep Dive](docs/advanced.md) — Theory, glossary, tuning
- [Architecture](docs/architecture.md) — How it works under the hood

## License

MIT License. FPF methodology by Anatoly Levenchuk.
