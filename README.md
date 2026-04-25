<img src="assets/banner.svg" alt="Haft" width="600">

*formerly [quint-code](https://github.com/m0n0x41d/quint-code)*

**True harness engineering for AI-assisted software delivery.**

Your agents write code fast. Most repositories are not ready for serious
harness engineering: the target system is underspecified, the enabling system
is implicit, term maps are missing, and runtime evidence is detached from the
spec. Haft makes the project harnessable before it scales execution.

---

## What is Haft?

Haft is the engineering governor that sits between your intentions and your agents' execution. It enforces the discipline that separates "we shipped fast" from "we shipped right": frame the problem before solving it, compare options under parity, record decisions as falsifiable contracts, and know the moment assumptions go stale.

**Specify → Think → Run → Govern.**

Not a coding agent. Not a generic documentation tool. The handle between the
tool and the hand — the part that turns raw capability into formal
specification, governed decisions, bounded commissions, and evidence-backed
engineering work.

### Three surfaces, one core

- **Desktop Cockpit** — human surface for onboarding, specs, governance, runtime visibility, and evidence
- **MCP plugin** — embedded agent surface for Claude Code and Codex to reason, draft, query, and create commissions
- **CLI Harness** — operator/runtime surface for prepare, run, status, result, apply, requeue, and cancel

All three surfaces compile into the same Haft Core artifact graph. Desktop does
not own semantic truth, MCP does not own long-running runtime lifecycle, and
CLI does not become a second source of meaning.

> **Note:** The TUI (`haft agent`) and Desktop app are in **pre-alpha** and under active development. They are not recommended for production use. The MCP plugin mode (`haft serve`) is the stable, proven interface.

---

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/m0n0x41d/haft/main/install.sh | bash
```

The install URL still points at the historical `quint-code` repository path. The installed binary is `haft`.

Then in your project, run init **with your supported host-agent flag**:

```bash
# Claude Code (default if no flag)
haft init

# Claude Code with repo-local commands
haft init --local

# Codex CLI / Codex App
haft init --codex

# Claude Code + Codex
haft init --all
```

v7 supported embedded host-agent surfaces:

```text
Claude Code
Codex CLI / Codex App
```

Cursor, Gemini CLI, JetBrains Air, and generic MCP clients remain
experimental/legacy integration targets. Their flags may exist while the
runtime and docs converge, but v7 product support is intentionally narrowed to
Claude Code and Codex.

```bash
# Experimental Cursor config
haft init --cursor

# Experimental Gemini CLI config
haft init --gemini
```

### What init does per tool

The binary is the same — only the MCP config and command/prompt installation
locations differ. Supported v7 hosts:

| Tool | MCP Config | Commands / Prompts | Skill |
|------|-----------|--------------------|-------|
| Claude Code | `.mcp.json` (project root) | `~/.claude/commands/` or `.claude/commands/` with `--local` | `~/.claude/skills/h-reason/` or local install with `--local` |
| Codex CLI / Codex App | `.codex/config.toml` | `~/.codex/prompts/` or `.codex/prompts/` with `--local` | `~/.agents/skills/h-reason/` |

Experimental/legacy hosts:

| Tool | MCP Config | Commands / Prompts | Skill |
|------|-----------|--------------------|-------|
| Cursor | `.cursor/mcp.json` | `~/.cursor/commands/` or `.cursor/commands/` with `--local` | `~/.cursor/skills/h-reason/` or local install with `--local` |
| Gemini CLI | `~/.gemini/settings.json` | `~/.gemini/commands/` or local install with `--local` | — |
| Air | `.codex/config.toml` | project `skills/` | project `skills/h-reason/` |

**Important for Cursor:** After init, open Cursor Settings → MCP → find `haft` → enable the toggle. Cursor adds MCP servers as disabled by default.

Project-scoped MCP configs are safe to commit for shared repositories: Claude
Code `.mcp.json` and Codex `.codex/config.toml` use portable project-root
values instead of your machine's absolute checkout path.

Existing project? Run `/h-onboard` after init. The target direction is deeper
than codebase summarization: onboarding should build a parseable target-system
spec, enabling-system spec, term map, and spec coverage graph before broad
harness execution.

Check the spec carriers locally with:

```bash
haft spec check
haft spec check --json
```

`haft spec check` is intentionally deterministic L0/L1/L1.5 only: it parses
fenced `yaml spec-section` blocks, checks required structural fields, validates
known carrier shapes, and verifies that the term-map carrier contains parseable
term entries. It does not make L2 semantic judgments, perform LLM review, prove
product correctness, or make L3 runtime/evidence claims.

---

## How It Works

### Seven MCP tools

| Tool | What it does |
|------|-------------|
| `haft_note` | Micro-decisions with validation + auto-expiry |
| `haft_problem` | Frame problems, define comparison dimensions with roles |
| `haft_solution` | Explore variants with diversity check, compare with parity |
| `haft_decision` | Decision contract with invariants, claims, evidence, baseline lifecycle |
| `haft_commission` | WorkCommission create/list/claim lifecycle for execution harnesses |
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

### WorkCommission harness

Open-Sleigh uses `WorkCommission` as the bounded execution authority between a
DecisionRecord and a runtime run. In plugin mode, use `/h-commission` to create
the missing WorkCommissions without starting execution. In Codex this installs
as an explicit-only `$h-commission` skill.

The packaged CLI path is:

```bash
haft harness run --prepare-only  # create/reuse commissions, do not start runtime
haft harness run                 # create/reuse commissions and start Open-Sleigh
haft harness status              # inspect active/recent runs
haft harness result wc-...       # inspect one completed run and workspace diff
haft harness apply wc-...        # apply a completed workspace patch to this checkout
```

Broad harness execution is blocked for `needs_onboard` projects by default. If
you intentionally need tactical out-of-spec work, pass
`--tactical-override-reason "..."`; Haft records that reason on the selected
WorkCommissions.

Starting Open-Sleigh is an operator/runtime action. Use the CLI Harness or
Desktop Cockpit for that boundary rather than a plugin slash command. MCP may
create or inspect WorkCommissions, but it must not own the long-running runtime
lifecycle.

Commissions carry a `delivery_policy`. The default is
`workspace_patch_manual`: Open-Sleigh changes stay in the isolated workspace
until an operator runs `haft harness apply <commission-id>`. Use
`--delivery-policy workspace_patch_manual` explicitly when preparing or running
plans that must not mutate the main checkout automatically.

Release installs include the Open-Sleigh runtime under:

```text
~/.haft/runtimes/open-sleigh/current
```

`haft harness run` uses a repo-local `open-sleigh/` checkout when present and
falls back to that installed runtime. Release archives bundle the BEAM runtime,
so users do not need to install Elixir/Mix for normal harness use. Source
fallback installs may install Elixir when they need to build the runtime
locally.

The lower-level MCP tool is `haft_commission`; the local CLI helper is:

```bash
haft commission create-from-decision dec-...
haft commission create-batch dec-a dec-b
haft commission create-from-plan .haft/plans/implementation.yaml
haft commission create --json commission.json
haft commission list --selector stale
haft commission show wc-...
haft commission requeue wc-... --reason stale_operator_recovery
haft commission cancel wc-... --reason no_longer_relevant
haft commission list-runnable
haft commission claim wc-...
```

For the repeatable local E2E smoke:

```bash
task open-sleigh:smoke-real-haft
```

The same loop powers Desktop Cockpit workflow buttons. A button must compile to
a typed artifact transition, not a free prompt:

```text
SpecSection(s) -> DecisionRecord -> WorkCommission -> RuntimeRun -> Evidence -> SpecCoverage
```

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

Built with Tauri v2 (Rust shell + React frontend). Launch with:

```bash
haft desktop        # finds Haft.app or falls back to dev build
```

Build from source (requires Rust toolchain + bun/npm for the frontend):

```bash
./scripts/build.sh --install   # builds Go binary + TUI bundle, installs locally
cd desktop-tauri && cargo tauri build   # builds the desktop app bundle
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

### v6.2 — Dashboard + Execution + Design System (shipped 2026-04-20)

The desktop became a real operator surface, the reasoning vocabulary grew semiotic teeth, and the two transport layers stopped drifting from each other:

- **Unified Dashboard** — decisions, governance findings, recent activity in one view
- **Implement** — click a decision, agent spawns in worktree with full reasoning context, baseline taken on success, PR body generated from decision rationale
- **Adopt** — governance finding → agent thread for interactive resolution; agent never auto-resolves
- **`haft run`** — same Implement pipeline from CLI, with planning + per-task verification + final invariant review
- **Tauri v2 desktop migration** (from Wails v2)
- **Haft Design System** — typed React primitives (Eyebrow, Button, Badge, Card, Input, StatCard, MonoId, Pill) + ComparisonTable with border-first Pareto grid + DecayWindow progress bar on decision detail
- **Seven new FPF semiotic patterns** (FRAME-08 / FRAME-09 / CHR-10 / CHR-11 / CHR-12 / X-STATEMENT-TYPE / X-FANOUT-AUDIT) sourced from Levenchuk's seminar, auto-injected into reasoning tool responses
- **`governance_mode` on DecisionRecord** — file-level vs module-level governance, opt-in, honors FPF X-SCOPE
- **Random-hex artifact IDs** (`dec-20260420-a3f7c1d2`) with optional DecisionRecord task slugs (`dec-20260420-task-4-a3f7c1d2`) to prevent merge conflicts while keeping filenames navigable ([#63](https://github.com/m0n0x41d/haft/issues/63), [#66](https://github.com/m0n0x41d/haft/issues/66))
- **MCP `parity_plan` exposure** for deep-mode comparison ([#62](https://github.com/m0n0x41d/haft/issues/62))
- **Transport-parity drift detection** + layered architecture boundary tests
- **`internal/embedding` extraction**; `internal/fpf` is now pure Core
- **`Valid-until` self-application** on FPF pattern files with a failing test when content ages past six months

### v7 — Project Harnessability MVP

One proved cycle:

**Add project → Init → Target spec → Enabling spec → Spec check → Decision → WorkCommission → RuntimeRun → Evidence → SpecCoverage.**

The goal is not a better task runner. The goal is to turn an arbitrary repo
into a harnessable engineering system.

### v8 — Governor Signals

Background detection loops (stale, drift, dependencies) with dashboard alerts. Autonomous actuation after trust is earned.

---

## Requirements

- Go 1.25+ (for building from source)
- Claude Code or Codex for supported v7 plugin mode
- Rust toolchain + Tauri v2 (only when building the desktop app from source)

## License

MIT
