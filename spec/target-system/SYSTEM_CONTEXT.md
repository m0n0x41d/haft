# System Context

> Reading order: 1 of N. Start here. 10 minutes.

## What needs to change in the environment

Engineers use AI coding agents (Claude Code, Codex, Cursor, Gemini CLI) daily. These agents generate solutions fast. The bottleneck shifted from "write the code" to "know what to build and why." Five things are broken:

1. **Decisions evaporate.** Agent recommends X in a chat session. Two weeks later nobody can answer "what did we decide about auth and why?" The rationale is buried in a conversation that no one will search.

2. **Comparison doesn't happen.** Agent jumps from "here are 3 options" to "I recommend X" without evaluating whether further exploration would change the outcome. The three options are usually variations on one idea.

3. **Evidence rots silently.** A decision made when traffic was 100 RPS is still governing the system at 10K RPS. Nobody tracks when assumptions expire.

4. **Projects are not harness-ready.** Runner-style systems assume the repository already has clear specs, term maps, boundaries, test contracts, and execution policy. Most real repositories do not. Agents can run, but the project is not yet admissible for harness engineering.

5. **Past experience doesn't compound.** Every project starts from zero, even for the same engineer. Decisions from one project never inform another.

## Method — how we change the environment

A **local-first project harnessability system** with three coupled methods:

1. a specification/onboarding runtime that turns a repository into a harnessable project,
2. a reasoning runtime any AI agent can plug into (`haft serve` / MCP),
3. a commissioned execution runtime (`haft harness`, currently implemented by
   the Open-Sleigh subsystem).

Together they add engineering discipline:

- Structured problem framing before jumping to solutions
- Genuinely different alternatives, not 3 variations on the same idea
- Honest comparison with explicit dimensions and Pareto trade-offs
- "Probe or commit" gate before decision — should we keep looking?
- Persistent decisions with verifiable claims, evidence, and expiry dates
- Explicit compilation from accepted decision to bounded WorkCommission
- Long-running, scope-bounded execution with preflight, gates, and evidence
- Drift detection and staleness scanning after code ships
- Cross-project recall with context-transfer penalties
- Parseable target-system and enabling-system specifications
- Spec coverage from specification sections to decisions, code, tests, commissions, and evidence
- Term-map and semantic-architecture validation before execution scaling

The discipline comes from FPF (First Principles Framework). Users do not need
to study FPF to operate Haft, but serious users may see the value explained in
product language: formal specs, term maps, target/enabling split, evidence,
and freshness. The everyday mode names remain **Understand, Explore, Choose,
Execute, Verify** — plus **Note** for quick captures.

## Role of the target system

**Haft = project harnessability cockpit and commissioned execution system for AI-assisted software delivery.**

One-liner: the system that makes a repository harnessable by building formal
project specifications, compiling them into decisions and commissions, and
closing the loop with runtime evidence.
Tagline: keeps the coder honest.

What it IS:
- Reasoning persistence layer (decisions survive sessions)
- Project onboarding system (turns existing or greenfield repos into Haft projects)
- Specification harness (TargetSystemSpec, EnablingSystemSpec, TermMap, SpecCoverage)
- Comparison discipline enforcer (Pareto, not recommendation essays)
- Evidence lifecycle manager (freshness, decay, drift)
- Governance governor (invariant verification, staleness alerts)
- Work authorization surface (turns an accepted decision into bounded,
  auditable execution work when the human chooses to commission it)
- Commission compiler (`DecisionRecord -> WorkCommission -> RuntimeRun`)
- Execution-runtime host (`haft harness`, with Open-Sleigh as the current
  runtime implementation)
- Optional external projection engine (Linear/Jira/GitHub issue text is a
  carrier for observers, not Haft's semantic authority)

What it is NOT:
- Not a coding agent (doesn't compete with Claude Code on editing files)
- Not a pattern browser (doesn't expose FPF as a catalog)
- Not a generic documentation generator (specs are parseable authority carriers
  that feed decisions, commissions, and evidence)
- Not a project management tool (no sprints, no Gantt charts; tracker
  projections are derived coordination surfaces)
- Not a general autonomous agent (no personal assistant, no omnichannel)

## Project harnessability layer

Haft's primary product promise is not "run agents on tickets". Its primary
promise is:

```text
Make this project ready for rigorous AI-assisted engineering.
```

That readiness requires a **ProjectSpecificationSet**:

```text
TargetSystemSpec
  -> EnablingSystemSpec
  -> TermMap
  -> SpecCoverage
  -> DecisionRecords
  -> WorkCommissions
  -> RuntimeRuns
  -> Evidence
```

The TargetSystemSpec answers what must change in the target system's
environment, by what method, and what role the target system plays. The
EnablingSystemSpec answers how the repository, tests, agents, CI, hooks,
runtime, and review process produce and maintain that target system.

Large formal specs are intentional. They are the price of admissible harness
engineering. The UX must make that depth navigable and valuable; it must not
pretend the depth is unnecessary.

## Execution subsystem

`Haft Harness` is the commissioned execution subsystem of Haft. Today its
runtime implementation is `Open-Sleigh`.

This distinction is load-bearing:

- **Haft owns semantic authority:** ProblemCards, DecisionRecords,
  WorkCommissions, Evidence, stale/refresh logic, and external projections.
- **Open-Sleigh owns runtime execution mechanics:** long-running orchestration,
  sessions, workspaces, retries, phase machine, leases, and agent adapters.

That means Open-Sleigh is **not** a peer product and **not** a second source
of truth. It is a subsystem/runtime of Haft, even if the implementation keeps
its own process boundary.

## Three delivery surfaces

### Surface A — Desktop App (primary: human)

The visual cockpit where the engineer lives during reasoning work.

- See: problem board, decision health, evidence quality, coverage, drift
- Specify: build target/enabling specs, term maps, and spec coverage
- Think: frame problems, explore variants, compare on Pareto front, decide
- Act: create commissions, start/stop harness runs, verify claims, create PRs from decisions
- Govern: dashboard with findings, stale alerts, invariant violations

Technology: Wails (Go + native WebView). Single binary. Local-first.

### Surface B — MCP Plugin (primary: agent)

How AI agents access the reasoning kernel during their coding work.

- 7 reasoning tools: problem, solution, decision, commission, query, refresh, note
- Commissioning tools for bounded execution work
- Stable API contract: tool names, required params, return shapes don't break
- Any MCP-compatible host: Claude Code, Codex, Cursor, Gemini CLI, Air

### Surface C — CLI (utility)

Quick access for scripting, CI, and terminal workflows.

- `haft init`, `haft serve`, `haft sync`, `haft board`, `haft search`
- `haft commission ...`, `haft harness run/status/result`
- `haft fpf search` (FPF spec lookup)
- `haft agent` (standalone agent mode — secondary to desktop)

**A and B are primary.** C is supporting utility.
Desktop is where humans think. MCP is where agents think. CLI is the operator
surface for automation and harness runtime control. Same kernel and artifact
graph underneath.

### Optional external projections

Haft must work with no Linear, Jira, GitHub Issues, or cloud tracker
configured. Local state, Desktop status, CLI status, and `.haft/` artifact
projections are sufficient for a solo/local workflow.

When an external tracker is configured, Haft publishes **ExternalProjections**
for human coordination. ExternalProjections may create/update Linear/Jira
issues, comments, labels, and statuses, but they do not author the semantic
state of work. Haft computes what is true; a bounded projection writer may
translate that truth into plain manager-facing language.

## Supersystem

Haft lives inside the software engineering delivery system:

```
┌─────────────────────────────────────────────────┐
│              Software Delivery Supersystem        │
│                                                   │
│  Issue Tracker ──→ Engineer ──→ AI Agent ──→ PR  │
│       │              │ ▲           │ ▲       │   │
│       │              │ │           │ │       │   │
│       │              ▼ │           ▼ │       │   │
│       │           ┌────────────────────┐     │   │
│       │           │       HAFT         │     │   │
│       └──────────→│                    │←────┘   │
│                   │  Specify → Think → │         │
│                   │  Run → Govern      │         │
│                   └────────────────────┘         │
│                        │                         │
│                   ┌────┴────┐                    │
│                   │  .haft/ │ (git-tracked)      │
│                   │  SQLite │ (local)            │
│                   └─────────┘                    │
│                                                   │
│  CI/CD ─── Tests ─── Docs ─── Code Review        │
└─────────────────────────────────────────────────┘
```

## Stakeholders

| Role | Who | What they need from Haft |
|------|-----|-------------------------|
| **Primary user** | Engineer using AI agent daily | A repo made harnessable: formal specs, decisions that survive, honest comparisons, evidence-backed execution |
| **Host agent** | Claude Code, Codex, any MCP client | Clean tool interface, fast responses, no interference with coding workflow |
| **Solo engineer** | Working alone across multiple projects | Cross-project recall, accumulated judgment, local-first |
| **Tech lead** | Responsible for architectural consistency | Target/enabling spec coverage, decision audit trail, staleness alerts, drift detection |
| **External observer** | Manager, analyst, lead, or teammate outside Haft | Plain-language status in Linear/Jira/GitHub, with links back to Haft artifacts |
| **CI/CD pipeline** | Automated checks | `haft check` — verify decisions are fresh and evidence is current |
| **PR reviewer** | Reading diffs | `.haft/decisions/*.md` in the diff — rationale visible alongside code |

## Non-stakeholders (explicitly)

| Not for | Why |
|---------|-----|
| FPF researchers | Haft is a product, not an FPF reference implementation |
| Non-technical managers (primary) | Haft is engineer-first. They may consume optional tracker projections, but they do not drive the reasoning model. |
| Compliance auditors (primary) | Audit views exist as secondary projections, not primary UX |
| Consumers / end users | No personal assistant surface |

## Key constraints

1. **Local-first.** Works without any server or cloud service.
2. **Solo-first.** Valuable for one engineer before needing teams.
3. **Spec-first.** Formal target/enabling specs are the entry point for serious harness work.
4. **Desktop-first.** Desktop app is the primary human surface (not CLI, not web).
5. **Plugin-compatible.** MCP plugin is the highest-reach integration channel.
6. **FPF inside.** Users should not need to study FPF terminology, but the product may explain why formal specs, term maps, and target/enabling split matter.
7. **Single binary.** One `haft` binary serves desktop, MCP server, CLI, and
   installs or operates the harness runtime.

## Enabling system (what builds Haft)

The enabling system is NOT the runtime. It's the "third factory":

- SoTA harvesting (Symphony, Zenflow, Hermes, Air — what patterns to adopt)
- Parity benchmarks (seeded corpus, catch rate, false positive rate)
- Workflow R&D (how to improve Specify → Think → Run → Govern cycle)
- FPF formalization (which of 214 patterns need L2/L3 enforcement)
- Semiotics review (term drift, authority confusion, gate/evidence mixing)

Creator: Ivan Zakutnii + AI coding agents. Solo developer with AI leverage.
