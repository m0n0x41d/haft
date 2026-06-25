<img src="assets/banner.svg" alt="Haft" width="600">

*formerly [quint-code](https://github.com/m0n0x41d/quint-code)*

**FPF governance substrate for AI-assisted software delivery.**

Haft is a local governance layer for AI coding agents. It gives your agent a
durable project memory: what problem is being solved, which options were
compared, which decision the human approved, what evidence supports it, and
what has gone stale. And ***more!***

Under the hood Haft is built on [FPF](https://github.com/ailev/FPF) by
[Anatoly Levenchuk](https://www.linkedin.com/in/ailev/). FPF is a rigorous
architecture for thinking about systems, and it is not small. Haft is the
practical handle: install it, let your agent use the skills and MCP gates, and
start getting the benefits before you have mastered the whole framework.

---

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/m0n0x41d/haft/main/install.sh | bash
```

Then initialize Haft in your project with the host you use:

```bash
haft init            # Claude Code (default)
haft init --local    # Claude Code, repo-local commands
haft init --codex    # Codex CLI / Codex App
haft init --hermes   # Hermes MCP + external skills directory
haft init --all      # Claude Code + Codex
```

Claude Code and Codex are the most used supported hosts. Pi, Hermes, Cursor,
Gemini CLI, and OpenCode have additional config flags (`--pi`, `--hermes`,
`--cursor`, `--gemini`, `--opencode`) while their runtime and docs converge.
Please report any host-specific issues with a PR or issue.

**Cursor:** after init, open Settings -> MCP -> find `haft` -> enable the
toggle. Cursor adds MCP servers disabled by default.

### What init does per tool

The binary is the same; only the MCP config and command/skill install locations
differ.

| Tool | MCP config | Commands / prompts | Skills |
|------|-----------|--------------------|--------|
| Claude Code | `.mcp.json` (project root) | `~/.claude/commands/` (or `.claude/commands/` with `--local`) | `~/.claude/skills/` (16 skills) |
| Codex CLI / App | `.codex/config.toml` | `~/.codex/prompts/` (or `.codex/prompts/` with `--local`) | `~/.agents/skills/` (16 skills) |
| Hermes | `~/.hermes/config.yaml` (or `$HERMES_HOME/config.yaml`; profile via `--profile`) | n/a | `skills.external_dirs` pointing at generated Hermes-adapted haft skills |

Project-scoped configs (`.mcp.json`, `.codex/config.toml`) use portable
project-root paths, so they are safe to commit for shared repositories.

---

## How to Start

FPF is <ins>**powerful**</ins> and *genuinely complex*.
You do not need to learn it all before Haft becomes useful.

Install Haft, run `haft init` for your agent, and keep working normally. When
the agent sees broad reasoning work, unclear architecture, risky changes, or a
need to compare options, it should route into `h-reason` automatically. You can
also call `/h-reason` by hand when you want to force the full reasoning loop.

Over time, learn the smaller commands:

- `/h-frame` — clarify the real problem and acceptance criteria
- `/h-explore` — generate genuinely different solution variants
- `/h-compare` — compare options under explicit parity rules
- `/h-decide` — record the human-approved binding decision
- `/h-verify` — check whether a past decision still holds
- `/h-status` — read the compact project cockpit

Haft will try to make the next required action explicit: when a human decision
is needed, when evidence has gone stale, when spec-drift should be reviewed, and
which command or drill-down can move the work forward.

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

## What is Haft?

Haft is a **governance substrate** that makes a repository harnessable for
principal-led engineering work. It turns problem frames, comparisons,
decisions, notes, commissions, specs, methods, and evidence into auditable
artifacts, with enforcement at the haft kernel boundary.

**Specify → Think → Run → Govern.**

Haft is not a coding agent, and it is not an AI documentation generator. It is
the handle between the human principal, the agent, the repository, and the
evolution lifecycle of your project.

## What Makes It Different

- **Durable reasoning, not chat-only memory** — problems, options, decisions,
  notes, specs, methods, and evidence live in one project graph.
- **Kernel gates, not prompt-only discipline** — skills carry the procedure; the MCP
  kernel validates required fields, parity gaps, missing evidence, and authority
  boundaries server-side.
- **Reasoning fused with code** — `haft_query` can show the decisions and
  invariants governing a file or symbol while the agent reads or changes it.
- **Evidence decays** — old proof is not treated as forever current; Haft
  surfaces stale claims and drift for review.
- **Human authority stays explicit** — agents can frame, compare, verify, and
  prepare records, but binding decisions and commissions require the human
  principal.
- **Agent guidance is built in** — status, method cards, spec checks, and
  structured errors tell the agent what is missing instead of leaving it to
  guess. People call it "harness" nowadays.

### Three surfaces, one artifact graph

Haft is consumed through three surfaces over one `.haft/` artifact graph:

- **Skills + slash commands** in your agent — workflow skills auto-trigger; `/h-frame /h-decide /h-verify ...` run manually
- **CLI** (`haft problem`, `haft solution`, `haft decision`, ...) — manual access. Those might be used manually, but usually it is just another surface for your agent to use.
- **MCP server** (`haft serve`) — programmatic access for any LLM agent over the Model Context Protocol

The kernel MCP server is the cross-host enforcement surface: it validates
arguments server-side and returns structured errors for FPF violations (missing
required fields, parity gaps, weakest-link omissions, predictions without
`verify_after`, and so on).

Skills carry the procedure; the kernel carries the gates. The same graph also
gives agents retrieval over project-local reasoning: notes for small facts,
decision records for authority, problem cards for open work, spec sections for
target/enabling-system shape, MethodRuns for execution discipline, and evidence
for verification.

### FPF under the hood

The skill set (`h-frame`, `h-explore`, `h-compare`, `h-decide`, `h-verify`, and
the full catalog below) gives your agent a project-local FPF governance
substrate for engineering decisions: framing before solutions,
characterization before comparison, parity enforcement, evidence with
congruence penalties, weakest-link assurance, and a cycle that reopens itself
when evidence ages or a measurement fails. It is a bounded kernel and workflow
surface for human-authorized AI engineering work, not a claim that Haft is a
complete FPF operating system.

The framing and comparison skills auto-trigger on operator context. The binding
step (`h-decide`, `h-commission`) is manual-only per the Transformer Mandate:
agents frame and compare; the human principal records the binding choice.

`haft fpf search` (and `haft_query(action="fpf")` from MCP) searches the indexed
FPF specification. Retrieval is hybrid: exact pattern id first, then keyword
(FTS5) fused with semantic recall over baked section vectors, so a reworded "how
do I think about X" finds the pattern that answers it. The vectors ship inside
the binary; semantic recall degrades to keyword when the embedding sidecar is
absent.

### What changed in v8

v8 dropped the standalone interactive agent (`haft agent`), the TUI, and the
desktop wrappers. Haft no longer competes with general coding agents on the
runtime surface — it adds governance discipline on top of whichever agent you
already use.

Upgrading from v7? See [MIGRATION-v8.md](MIGRATION-v8.md) — the upgrade checklist
plus what was dropped (`haft agent`, TUI, desktop, v7 helper commands).

## How It Works

### Nine MCP tools

| Tool | What it does |
|------|-------------|
| `haft_note` | Micro-decisions — atomic facts with typed anchors, validation, auto-expiry |
| `haft_problem` | Frame problems, declare comparison dimensions with indicator roles |
| `haft_solution` | Explore variants with diversity check, compare under parity |
| `haft_decision` | Decision contracts: invariants, claims, evidence, baseline lifecycle |
| `haft_refresh` | Lifecycle management for every artifact kind |
| `haft_query` | Search, status dashboard, code graph (callers/callees/impact/explore — each reached symbol fused with the decisions governing it), FPF spec search |
| `haft_method` | Task-local SWE MethodRun cards: pull gates before non-trivial work, close with evidence or waivers |
| `haft_commission` | WorkCommission lifecycle for execution harnesses |
| `haft_spec_section` | Typed SpecSection lifecycle projection; manual CLI gates approve, rebaseline, or reopen baselines |

### Sixteen skills installed by `haft init`

| Skill | Mode | What it does |
|---|---|---|
| **h-reason** | auto (umbrella) | Full FPF reasoning palette in one entry — framing, exploration, comparison, verification, notes, plus slideument patterns (Goldilocks, NQD, BLP, Scaling-Law Lens). Manual `/h-reason` always works; auto-fires on broad "let's think this through" signals where no specialized skill matches sharply. |
| **h-frame** | auto | Frame a problem with B.4.1 stabilize + problem typing + umbrella-word repair |
| **h-diagnose** | auto | Diagnose a failure with parallel hypothesis testing (one Agent subagent per hypothesis to prevent anchoring) |
| **h-explore** | auto | Generate distinct candidate variants with NQD diversity discipline (parallel direction-assigned agents) |
| **h-compare** | auto | Fair comparison with dim-wise parallel scoring + Pareto front (not a scalar winner) |
| **h-decide** | **manual** | Record a binding DecisionRecord with full DRR — Transformer Mandate (`disable-model-invocation`) |
| **h-verify** | auto | Baseline → measure → evidence loop with drift detection |
| **h-status** | auto | Read-only project FPF cockpit with explicit drill-down calls |
| **h-onboard** | auto | First-frame ceremony for projects new to haft |
| **h-spec** | auto | Spec lifecycle surface: inspect readiness, draft/clarify carriers, and route approve/rebaseline/reopen gates |
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

Routing reliability is testable: `haft check routing` runs 44 golden prompts
(current pass rate 77.3%).

### Evidence workflow

Attach evidence with `haft_decision(action="evidence", ...)`. Evidence carries
formality levels (F0–F9), congruence levels (CL0–CL3), and expiry dates. Trust
scores (R_eff) degrade as evidence ages; stale evidence triggers refresh. Use
`haft_decision(action="measure", ...)` for post-implementation verification.

### Harness execution engine (beta, Codex only)

> 🚨 It is likely to be suspended in future versions because the work commission is highly commoditized already by subagents of most agents. Those agents usually have access to the same skills and MCP, meaning... Subagents work quite well on half-created problems and decisions, so I find myself using Claude code and Codex subagents quite well without the need for any external harness circuit.

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

The lower-level surface is the `haft commission` CLI for binding creation
(`create-from-decision`, `create-batch`, `create-from-plan`, ...) plus the
`haft_commission` MCP tool for non-creation lifecycle/read actions. MCP creation
actions fail closed by default with `operator_confirmation_required`; every
authorized commission action must come through a kernel-accepted `manual_cli`
authorization receipt and becomes a typed artifact transition. Model-supplied
MCP arguments or prompt text are not proof of operator authorization:

```text
SpecSection(s) → DecisionRecord → WorkCommission → RuntimeRun → Evidence → SpecCoverage
```

---

## Cookbook — common workflows

### Record an architectural choice

```text
operator (to agent): "we need to pick a queue for the new ingestion path"
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

From the host agent: `/h-status` for the compact cockpit; use explicit
drill-down calls for full status, coverage, drift/stale detail, and maintenance
plans.

Shipped history lives in [CHANGELOG.md](CHANGELOG.md).

---

## License

MIT
