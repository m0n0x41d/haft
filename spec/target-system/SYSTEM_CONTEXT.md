# System Context

> Reading order: 1 of N. Start here. 10 minutes.

## What needs to change in the environment

Engineers use AI coding agents daily. Haft v9 treats the host agent as the
primary reasoning surface rather than shipping another coding agent. These
agents generate solutions fast. The bottleneck shifted from "write the code"
to "know what to build and why." Five things are broken:

1. **Decisions evaporate.** Agent recommends X in a chat session. Two weeks later nobody can answer "what did we decide about auth and why?" The rationale is buried in a conversation that no one will search.

2. **Comparison doesn't happen.** Agent jumps from "here are 3 options" to "I recommend X" without evaluating whether further exploration would change the outcome. The three options are usually variations on one idea.

3. **Evidence rots silently.** A decision made when traffic was 100 RPS is still governing the system at 10K RPS. Nobody tracks when assumptions expire.

4. **Projects are not harness-ready.** Runner-style systems assume the repository already has clear specs, term maps, boundaries, test contracts, and execution policy. Most real repositories do not. Agents can run, but the project is not yet admissible for harness engineering.

5. **Past experience doesn't compound.** Every project starts from zero, even for the same engineer. Decisions from one project never inform another.

## Method — how we change the environment

Haft v9 combines two primary capabilities:

1. **source-native FPF delivery** — versioned upstream FPF source, provenance,
   and project-local application surfaces in the host agent;
2. **reliance-bearing project memory** — typed local records when handoff,
   replay, authority, automation, evidence, or later verification depends on
   the result.

Specification onboarding, drift checks, evidence lifecycle, commissioning, and
runtime execution remain project-governance capabilities around that core. They
do not turn every FPF use into an artifact. Ordinary bounded and reversible
reasoning may remain conversational; Haft materializes a trace when a named
receiving use will rely on it.

FPF is relation-first. Text, graph, card, and skill order do not by themselves
prescribe causal, temporal, method, planning, or performed-work order. Such
order must be carried by an explicit causal claim, `U.MethodDescription`,
ImplementationPlan, WorkCommission, or work relation. This is not a claim that
FPF is acausal; it is a prohibition on inferring causality from layout.

## Role of the target system

**Haft = source-native FPF delivery and reliance-bearing project memory for
AI-assisted software engineering.**

One-liner: the system that brings FPF source into the agent's work and keeps
the project records that later engineering action must be able to trust.
Tagline: keeps the coder honest.

What it IS:
- Versioned FPF source-delivery surface with provenance
- Reliance-bearing project memory (decisions and evidence survive sessions when
  another use depends on them)
- Project onboarding system (turns existing or greenfield repos into Haft projects)
- Specification harness (TargetSystemSpec, SoftwareSystemSpec, TermMap, with linked SpecCoverage)
- Project-local application surfaces for FPF methods, including framing,
  exploration, comparison, and verification where applicable
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

## Reliance-bearing project layer

Haft's primary product promise is:

```text
Deliver FPF source where the agent can use it, and preserve only the project
memory that future engineering work must rely on.
```

Governed or commissioned work may require a **ProjectSpecificationSet**:

```text
ProjectSpecificationSet
  = TargetSystemSpec
  + SoftwareSystemSpec
  + TermMap

ProjectSpecificationSet
  -> SpecCoverage
  -> DecisionRecords
  -> WorkCommissions
  -> RuntimeRuns
  -> Evidence
```

The arrows above name admissible relations, not a temporal workflow or an
execution plan. Planning stays in an explicit ImplementationPlan or
WorkCommission.

The TargetSystemSpec answers what must change in the target system's
environment, by what method, and what role the target system plays. The
SoftwareSystemSpec answers what idealized software realizes that role: its
responsibilities, behavior, interfaces, constraints, and selected structure.
Repository workflows, tests, agents, CI, release policy, and harness runtime
belong to the enabling system, not the SoftwareSystemSpec.

`TargetSystemSpec`, `SoftwareSystemSpec`, and the target/enabling distinction
are Haft local practice for Agentic SWE, not normative kinds defined by FPF
A.1. Their subjects may still be typed and related with FPF primitives.

Formal specs are justified where delegated work relies on stable boundaries
and acceptance claims. Ordinary local, reversible FPF use does not require a
ProjectSpecificationSet or a durable trace.

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

## Three delivery surfaces over one semantic core

All delivery surfaces share the same Haft Core artifact graph:

```text
ProjectSpecificationSet
  = TargetSystemSpec
  + SoftwareSystemSpec
  + TermMap

ProjectSpecificationSet
  -> SpecCoverage
  -> ProblemCards
  -> DecisionRecords
  -> WorkCommissions
  -> RuntimeRuns
  -> Evidence
```

Again, these arrows are typed relations. They do not infer work order.

No surface owns truth. The operator cockpit is the human-facing view. MCP is the
embedded agent-facing authoring surface. CLI is the runtime/operator surface.
Haft Core owns semantic authority; Open-Sleigh owns execution mechanics.

### Surface A — Historical Desktop App (archived, not current)

The visual cockpit where the engineer lives during reasoning work.

- See: problem board, decision health, evidence quality, coverage, drift
- Specify: build target/software specs, term maps, and spec coverage
- Think: frame problems, explore variants, compare on Pareto front, decide
- Act: create commissions, start/stop harness runs, verify claims, create PRs from decisions
- Govern: dashboard with findings, stale alerts, invariant violations

Technology: Tauri v2 (Rust shell + native WebView + React frontend).
Local-first. The archived desktop surface remained over Haft Core and CLI/RPC contracts.

### Surface B — MCP Plugin (primary: embedded agent)

How AI agents access the reasoning kernel during their coding work.

- Supported v7 hosts: Claude Code and Codex
- 7 reasoning tools: problem, solution, decision, commission, query, refresh, note
- Commissioning tools for bounded execution work
- Spec/onboarding tools for target specs, software specs, term map, status, and refresh
- Stable API contract: tool names, required params, return shapes don't break
- May create or inspect WorkCommissions, but must not own long-running runtime lifecycle

Deferred or experimental hosts: Cursor, Gemini CLI, JetBrains Air, and generic
MCP clients. They may remain installable while v7 narrows support, but product
support and acceptance tests target Claude Code and Codex.

### Surface C — CLI Harness (primary: runtime/operator)

Operator access for scripting, CI, terminal workflows, and the harness runtime
boundary.

- `haft init`, `haft serve`, `haft sync`, `haft board`, `haft search`
- `haft commission ...`
- `haft harness prepare/run/status/watch/tail/result/apply/requeue/cancel`
- `haft fpf query`, `haft fpf lookup`, and `haft fpf inspect` (source-native FPF retrieval)
- `haft agent` (removed standalone agent mode; archived)

The CLI is not a second semantic system. It exposes operator commands for the
harness runtime and local automation. Runtime preflight must still check
WorkCommission, linked DecisionRecord, Scope, freshness, lockset, and autonomy
envelope through Haft Core.

## Surface transition rule

Archived desktop workflow buttons, MCP slash/tool calls, and CLI commands must compile
to typed artifact transitions, not free prompts:

```text
Button or command
  -> typed workflow
  -> explicit artifact mutation/proposal
  -> deterministic check
  -> derived status
```

Examples:

- `Draft Target Spec` -> OnboardingAgent draft -> `SpecSection` carriers -> spec check -> human approval.
- `Create WorkCommission` -> `DecisionRecord` + scope -> `WorkCommission` snapshot -> runnable queue.
- `Delegate to Harness` -> runnable `WorkCommission` -> preflight -> `RuntimeRun`.
- `Review Evidence` -> evidence carrier -> claim/spec coverage derivation.

### Optional external projections

Haft must work with no Linear, Jira, GitHub Issues, or cloud tracker
configured. Local state, CLI status, and `.haft/` artifact
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
│                   │  FPF source +      │         │
│                   │  project memory    │         │
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
| **Host agent** | Claude Code, Codex | Clean tool interface, fast responses, no interference with coding workflow |
| **Solo engineer** | Working alone across multiple projects | Cross-project recall, accumulated judgment, local-first |
| **Tech lead** | Responsible for architectural consistency | Target/software spec coverage, decision audit trail, staleness alerts, drift detection |
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
3. **Reliance-first.** Persist only what a named receiving use must trust;
   governed or commissioned work may require formal target/software specs.
4. **Host-agent-first.** Skills/prompts plus MCP are the primary embedded agent surface; CLI remains the operator/runtime surface.
5. **Plugin-compatible.** MCP plugin is the highest-reach integration channel, with Claude Code and Codex as supported hosts.
6. **FPF source-native.** Haft delivers versioned upstream source and
   project-local application records; it does not define a substitute pattern
   ontology.
7. **Single binary.** One `haft` binary serves the MCP server and CLI, and
   installs or operates the harness runtime.

## Enabling system (what builds Haft)

The enabling system is NOT the runtime. It's the "third factory":

- SoTA harvesting (Symphony, Zenflow, Hermes, Air — what patterns to adopt)
- Parity benchmarks (seeded corpus, catch rate, false positive rate)
- Workflow R&D (how to improve independent reasoning, planning, execution, and
  verification surfaces without inventing a universal order)
- FPF source integration (versioning, provenance, retrieval, and bounded
  project-local application)
- Semiotics review (term drift, authority confusion, gate/evidence mixing)

Creator: Ivan Zakutnii + AI coding agents. Solo developer with AI leverage.
