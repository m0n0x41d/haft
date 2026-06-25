<img src="assets/banner.svg" alt="Haft" width="600">

*formerly [quint-code](https://github.com/m0n0x41d/quint-code)*

**FPF governance substrate for AI-assisted software delivery.**

Your agents (Claude Code, Codex) write code fast. Most repositories are not ready
for serious harness engineering: the target system is underspecified, the
enabling system is implicit, term maps are missing, and runtime evidence is
detached from the spec. Haft makes the project harnessable before it scales
execution.
testcodex
---

## What is Haft?

Haft is a **governance substrate** that makes a repository harnessable for
principal-led FPF engineering work. It turns problem frames, comparisons,
decisions, commissions, and evidence into auditable artifacts, with enforcement
at the kernel boundary.

**Specify → Think → Run → Govern.**

Not a coding agent. Not a documentation generator. The handle between the tool
and the hand: the part that turns raw model capability into formal
specification, governed decisions, bounded commissions, and evidence-backed
engineering work.

### Three surfaces, one artifact graph

Haft is consumed through three surfaces over one `.haft/` artifact graph:

- **Skills + slash commands** in your coding agent (Claude Code, Codex, OpenCode, Cursor) — workflow skills auto-trigger; `/h-frame /h-decide /h-verify ...` run manually
- **CLI** (`haft problem`, `haft solution`, `haft decision`, ...) — manual access, no LLM in the loop
- **MCP server** (`haft serve`) — programmatic access for any LLM agent over the Model Context Protocol

The kernel MCP server is the cross-host enforcement surface: it validates
arguments server-side and returns structured errors for FPF violations (missing
required fields, parity gaps, weakest-link omissions, predictions without
verify_after). Skills carry the procedure; the kernel carries the gates.

### What changed in v8

v8 dropped the standalone interactive agent (`haft agent`), the TUI, and the
desktop wrappers. Haft no longer competes with general coding agents on the
runtime surface — it adds governance discipline on top of whichever agent you
already use. The pivot, with parity-compared variants, rollback plan, and
falsifiable predictions, is recorded in
`.haft/decisions/dec-20260525-v8-architecture-pivot-from-standalone-agent-to-g-bbe45cb7.md`.

Upgrading from v7? See [MIGRATION-v8.md](MIGRATION-v8.md) — the upgrade checklist
plus what was dropped (`haft agent`, TUI, desktop, v7 helper commands).

---

## Built on First Principles Framework

[FPF](https://github.com/ailev/FPF) by
[Anatoly Levenchuk](https://www.linkedin.com/in/ailev/) — a rigorous,
transdisciplinary architecture for thinking.

The skill set (`h-frame`, `h-explore`, `h-compare`, `h-decide`, `h-verify`, and
the full catalog below) gives your agent an FPF-native operating system for
engineering decisions: framing before solutions, characterization before
comparison, parity enforcement, evidence with congruence penalties,
weakest-link assurance, and a cycle that reopens itself when evidence ages or a
measurement fails.

The framing and comparison skills auto-trigger on operator context. The binding
step (`h-decide`, `h-commission`) is manual-only per the Transformer Mandate:
agents frame and compare; the human principal records the binding choice.

`haft fpf search` (and `haft_query(action="fpf")` from MCP) searches the indexed
FPF specification. Retrieval is hybrid: exact pattern id first, then keyword
(FTS5) fused with semantic recall over baked section vectors, so a reworded "how
do I think about X" finds the pattern that answers it. The vectors ship inside
the binary; semantic recall degrades to keyword when the embedding sidecar is
absent.

---

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/m0n0x41d/haft/main/install.sh | bash
```

The install URL still points at the historical `quint-code` path. The installed
binary is `haft`.

Then in your project, init with your host-agent flag:

```bash
haft init            # Claude Code (default)
haft init --local    # Claude Code, repo-local commands
haft init --codex    # Codex CLI / Codex App
haft init --all      # Claude Code + Codex
```

Claude Code and Codex are the supported hosts. Cursor, Gemini CLI, and OpenCode
have experimental config flags (`--cursor`, `--gemini`, `--opencode`) while
their runtime and docs converge.

**Cursor:** after init, open Settings → MCP → find `haft` → enable the toggle.
Cursor adds MCP servers disabled by default.

### What init does per tool

The binary is the same; only the MCP config and command/skill install locations
differ.

| Tool | MCP config | Commands / prompts | Skills |
|------|-----------|--------------------|--------|
| Claude Code | `.mcp.json` (project root) | `~/.claude/commands/` (or `.claude/commands/` with `--local`) | `~/.claude/skills/` (15 skills) |
| Codex CLI / App | `.codex/config.toml` | `~/.codex/prompts/` (or `.codex/prompts/` with `--local`) | `~/.agents/skills/` (15 skills) |

Project-scoped configs (`.mcp.json`, `.codex/config.toml`) use portable
project-root paths, so they are safe to commit for shared repositories.

Existing project? Run `/h-onboard` after init. It builds a parseable
target-system spec, enabling-system spec, term map, and spec-coverage graph —
not just a codebase summary.

Check spec carriers locally:

```bash
haft spec check
haft spec check --json
```

`haft spec check` is deterministic L0/L1/L1.5 only: it parses fenced
`yaml spec-section` blocks, checks required structural fields, validates known
carrier shapes, and confirms the term-map carrier parses. It makes no L2
semantic judgment, no LLM review, and no L3 runtime claim.

---

## How It Works

### Seven MCP tools

| Tool | What it does |
|------|-------------|
| `haft_note` | Micro-decisions — atomic facts with typed anchors, validation, auto-expiry |
| `haft_problem` | Frame problems, declare comparison dimensions with indicator roles |
| `haft_solution` | Explore variants with diversity check, compare under parity |
| `haft_decision` | Decision contracts: invariants, claims, evidence, baseline lifecycle |
| `haft_commission` | WorkCommission lifecycle for execution harnesses |
| `haft_refresh` | Lifecycle management for every artifact kind |
| `haft_query` | Search, status dashboard, code graph (callers/callees/impact/explore — each reached symbol fused with the decisions governing it), FPF spec search |

### Fifteen skills installed by `haft init`

| Skill | Mode | What it does |
|---|---|---|
| **h-reason** | auto (umbrella) | Full FPF reasoning palette in one entry — framing, exploration, comparison, verification, notes, plus slideument patterns (Goldilocks, NQD, BLP, Scaling-Law Lens). Manual `/h-reason` always works; auto-fires on broad "let's think this through" signals where no specialized skill matches sharply. |
| **h-frame** | auto | Frame a problem with B.4.1 stabilize + problem typing + umbrella-word repair |
| **h-diagnose** | auto | Diagnose a failure with parallel hypothesis testing (one Agent subagent per hypothesis to prevent anchoring) |
| **h-explore** | auto | Generate distinct candidate variants with NQD diversity discipline (parallel direction-assigned agents) |
| **h-compare** | auto | Fair comparison with dim-wise parallel scoring + Pareto front (not a scalar winner) |
| **h-decide** | **manual** | Record a binding DecisionRecord with full DRR — Transformer Mandate (`disable-model-invocation`) |
| **h-verify** | auto | Baseline → measure → evidence loop with drift detection |
| **h-status** | auto | Read-only project FPF state dashboard |
| **h-onboard** | auto | First-frame ceremony for projects new to haft |
| **h-spec-cover** | auto | Spec-coverage check with blind/stale module triage |
| **h-note** | auto | Lightweight micro-decision recording |
| **h-commission** | **manual** | WorkCommission lifecycle — manual per Transformer Mandate (`disable-model-invocation`) |
| **h-abduct** | subroutine | Pure B.5.2 abductive four-step (frame prompt → ≥3 rivals → filters → prime) |
| **h-boundary-unpack** | subroutine | A.6.B L/A/D/E decomposition of boundary statements |
| **h-semio-review** | subroutine | X-FANOUT-AUDIT — concept-rename / spec-consistency audit |

Auto-triggering skills fire when their description matches operator context.
Manual-only skills (`h-decide`, `h-commission`) require explicit invocation per
the Transformer Mandate — binding artifacts come from the human principal, not
the agent. Subroutines (`h-abduct`, `h-boundary-unpack`, `h-semio-review`) are
called from other skills or invoked explicitly when working a specific FPF
sub-discipline.

Routing reliability is testable: `haft check routing` runs 40 golden prompts
(current pass rate 82.5%).

### Evidence workflow

Attach evidence with `haft_decision(action="evidence", ...)`. Evidence carries
formality levels (F0–F3), congruence levels (CL0–CL3), and expiry dates. Trust
scores (R_eff) degrade as evidence ages; stale evidence triggers refresh. Use
`haft_decision(action="measure", ...)` for post-implementation verification.

### Harness — execution engine (beta, Codex only)

The harness implements code from `DecisionRecord` artifacts under a real Codex
agent in an isolated workspace. It is **beta**, and the execution agent is
**Codex only** — there is no Claude execution path. Single-commission
`haft harness run` is the trustworthy operator path; drain mode and auto-apply
are validated on docs-class commissions, so treat them as beta on
production-code commissions.

Two entry points spawn the engine. `haft run` implements one decision directly:

```bash
haft run dec-20260414-001
```

It reads the decision's invariants, claims, and affected files from the graph,
builds a prompt with full reasoning context, spawns a Codex agent with the
invariants as guardrails, and takes a baseline snapshot on completion.

`haft harness` runs commissioned work through Open-Sleigh, with scope guards
(`allowed_paths` / `forbidden_paths`), per-commission locks, and discrete
revertable apply commits:

```bash
haft harness run --prepare-only      # create/reuse commissions, do not start runtime
haft harness run                     # create/reuse commissions and start Open-Sleigh
haft harness run --drain --concurrency 4   # drain the queue (apply still manual by default)
haft harness status                  # inspect active/recent runs
haft harness result wc-...           # inspect one completed run and its workspace diff
haft harness apply wc-...            # apply a completed workspace patch to this checkout
```

Commissions carry a `delivery_policy`. The default `workspace_patch_manual`
keeps changes in the isolated workspace until you run `haft harness apply`.
`workspace_patch_auto_on_pass` applies a passing run as a discrete commit;
`blocked_policy` / failed runs wait for an operator decision.

Broad harness execution is blocked for `needs_onboard` projects by default. For
intentional tactical out-of-spec work, pass `--force-skip-specs "<reason>"`;
haft records the reason on the selected commissions.

Release archives bundle the Open-Sleigh BEAM runtime, so normal harness use
needs no Elixir/Mix install:

```text
~/.haft/runtimes/open-sleigh/current
```

The lower-level surface is the `haft_commission` MCP tool and the `haft commission`
CLI (`create-from-decision`, `create-batch`, `create-from-plan`, `list`, `show`,
`requeue`, `cancel`, `claim`, ...). Every commission action becomes a typed
artifact transition, never a free prompt:

```text
SpecSection(s) → DecisionRecord → WorkCommission → RuntimeRun → Evidence → SpecCoverage
```

---

## Cookbook — common workflows

### Record an architectural choice

```text
operator (to Claude Code): "we need to pick a queue for the new ingestion path"
↓ h-explore auto-triggers, generates 3+ distinct variants with NQD diversity
↓ h-compare auto-triggers, scores dim-wise in parallel, surfaces the Pareto front
↓ operator picks a variant, then explicitly types:
/h-decide
↓ kernel validates required DRR fields; missing fields → structured error
↓ on pass: DRR written to .haft/decisions/, ready for `haft run`
```

### Diagnose a failure with rival hypotheses

```text
operator: "tests are failing on the schema migration after the deploy"
↓ h-diagnose auto-triggers, spawns 3+ parallel Agent subagents, one per hypothesis
↓ each subagent reads only what its hypothesis needs (no anchoring)
↓ results merged, ranked by the FPF B.5.2 filter chain
↓ if confirmed: /h-note records the diagnosis; if architectural: /h-frame
```

### Verify a decision still holds

```text
operator: "did dec-20260420-cache-redesign actually work"
↓ h-verify auto-triggers
↓ reads decision predictions + valid_until + baseline file hashes
↓ measures observable claims (test output, metric query, ...)
↓ writes evidence with CL/freshness; updates R_eff
↓ if R_eff < 0.5 → marks stale; if predictions failed → reopens the problem
```

### Quick operator status

```bash
haft check          # CI-friendly governance verification (exit 0 clean / 1 findings)
haft check routing  # sanity-check skill routing reliability
```

From the host agent: `/h-status` for the full dashboard.

---

## What Makes It Different

- **Decisions are live** — computed trust scores (R_eff) degrade as evidence ages
- **Comparison is honest** — parity enforced, constraint-aware Pareto elimination, anti-Goodhart observation indicators
- **Reasoning fused with code** — `haft_query` surfaces the decisions governing a symbol while you read or traverse it, so a governed node never reads as safe-to-change
- **Memory across sessions** — related past decisions surface during framing, similar variants during exploration
- **The loop closes** — failed measurements reopen decisions, evidence decay triggers review, drift detection flags violations
- **Decisions are contracts** — invariants, claims with thresholds, rollback plan, valid-until date

---

## Roadmap

### v8 — Governance Substrate Pivot (current)

Standalone interactive agent, TUI, and desktop wrappers dropped. Haft is a
kernel + CLI + MCP server + 15 skills, shared across Claude Code, Codex,
OpenCode, and Cursor over one `.haft/` artifact graph. The kernel MCP returns
structured errors as hard enforcement gates; binding artifacts stay manual-only
per the Transformer Mandate. Rationale:
`dec-20260525-v8-architecture-pivot-...`.

Shipped history lives in [CHANGELOG.md](CHANGELOG.md).

### Next

The defensible edge is the fusion of the code graph with the reasoning graph,
and the liveness of that reasoning graph — not raw code-graph coverage. Active
directions (idea-stage, tracked as live problems under `.haft/`): edit-time
invariant guardrails on governed symbols, trust state surfaced at the symbol,
coherence checks over the governance graph, and a run-time harness that measures
whether the fused graph actually reduces reads and prevents broken decisions.
None are committed to a release version.

---

## Requirements

- **Go 1.25+** — building from source
- **Claude Code or Codex** — plugin mode
- **Rust toolchain** — only to build the embedding sidecar (`haft-embed`) from source; without it, FPF semantic search degrades to keyword

## License

MIT
