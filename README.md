<img src="assets/banner.svg" alt="Quint Code" width="600">

**Engineering decisions that know when they're stale.**

Frame problems. Compare options fairly. Record decisions as contracts. Know when to revisit.

Supports: Claude Code, Cursor, Gemini CLI, Codex CLI, Codex App, Air

---

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/m0n0x41d/quint-code/main/install.sh | bash
```

Then in your project:

```bash
quint-code init
```

Existing project? Run `/q-onboard` after init — the agent scans your codebase for existing decisions worth capturing.

---

## How It Works

### One command: `/q-reason`

Describe your problem. The agent frames it, generates alternatives, compares them fairly, and records the decision — all in one command. It auto-selects the right depth.

### Or drive each step manually

```
/q-frame  → /q-char  → /q-explore → /q-compare → /q-decide
  what's      what       genuinely     fair         engineering
  broken?     matters?   different     comparison   contract
                         options
```

### Micro-decisions on the fly

The agent captures decisions automatically when it notices them in conversation. No rationale — no record. Conflicts with active decisions are flagged. Auto-expires in 90 days.

### When decisions go stale

`/q-status` shows what's expired and what needs attention. `/q-refresh` manages the lifecycle of ALL artifact types — waive, reopen, supersede, or deprecate.

---

## What Makes It Different

- **Decisions are live** — they have computed trust scores (R_eff) that degrade as evidence ages. An expired benchmark drops the whole score.
- **Comparison is honest** — parity enforced, dimensions cross-checked, asymmetric scoring warned. Anti-Goodhart: tag dimensions as "observation" to prevent optimizing the wrong metric.
- **Memory across sessions** — when you frame a problem, the tool surfaces related past decisions. When you explore, it checks for similar variants.
- **The loop closes** — failed measurements suggest reopening. Evidence decay triggers review. Periodic refresh prompts ensure nothing goes stale silently.
- **Decisions are contracts** — FPF E.9 format: Problem Frame, Decision (invariants + DO/DON'T), Rationale, Consequences. A new engineer reads it 6 months later and gets everything.

---

## 6 Tools

| Tool | What it does |
|------|-------------|
| `quint_note` | Micro-decisions with validation + auto-expiry |
| `quint_problem` | Frame problems, define comparison dimensions with roles |
| `quint_solution` | Explore variants with diversity check, compare with parity |
| `quint_decision` | FPF E.9 decision contract, impact measurement, evidence |
| `quint_refresh` | Lifecycle management for all artifacts |
| `quint_query` | Search, status dashboard, file-to-decision lookup |

---

## Built on First Principles Framework

[FPF](https://github.com/ailev/FPF) by [Anatoly Levenchuk](https://www.linkedin.com/in/ailev/) — a rigorous, transdisciplinary architecture for thinking.

`/q-reason` gives your AI agent an FPF-native operating system for engineering decisions: problem framing before solutions, characterization before comparison, parity enforcement, evidence with congruence penalties, weakest-link assurance, and the lemniscate cycle that closes itself when evidence ages or measurements fail.

`quint-code fpf search` gives you access to 4243 indexed sections from the FPF specification — the agent can look up any concept on demand.

---

## Learn More

See the [documentation](https://quint.codes/learn) for detailed guides on decision modes, the DRR format, computed features, and lifecycle management.

## Requirements

- Go 1.24+ (for building from source)
- Any MCP-capable AI tool

## License

MIT
