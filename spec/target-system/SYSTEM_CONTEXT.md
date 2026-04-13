# System Context

> Reading order: 1 of N. Start here. 10 minutes.

## What needs to change in the environment

Engineers use AI coding agents (Claude Code, Codex, Cursor, Gemini CLI) daily. These agents generate solutions fast. The bottleneck shifted from "write the code" to "know what to build and why." Four things are broken:

1. **Decisions evaporate.** Agent recommends X in a chat session. Two weeks later nobody can answer "what did we decide about auth and why?" The rationale is buried in a conversation that no one will search.

2. **Comparison doesn't happen.** Agent jumps from "here are 3 options" to "I recommend X" without evaluating whether further exploration would change the outcome. The three options are usually variations on one idea.

3. **Evidence rots silently.** A decision made when traffic was 100 RPS is still governing the system at 10K RPS. Nobody tracks when assumptions expire.

4. **Past experience doesn't compound.** Every project starts from zero, even for the same engineer. Decisions from one project never inform another.

## Method — how we change the environment

A **reasoning runtime** that any AI agent can plug into (MCP plugin) or that operates as a standalone orchestrator (desktop app). The runtime adds engineering discipline:

- Structured problem framing before jumping to solutions
- Genuinely different alternatives, not 3 variations on the same idea
- Honest comparison with explicit dimensions and Pareto trade-offs
- "Probe or commit" gate before decision — should we keep looking?
- Persistent decisions with verifiable claims, evidence, and expiry dates
- Drift detection and staleness scanning after code ships
- Cross-project recall with context-transfer penalties

The discipline comes from FPF (First Principles Framework). Users never see FPF. They see 5 engineering modes: **Understand, Explore, Choose, Execute, Verify** — plus **Note** for quick captures.

## Role of the target system

**Haft = engineering reasoning runtime for AI-assisted software delivery.**

One-liner: the system that makes engineering decisions explicit, comparable, and verifiable.
Tagline: keeps the coder honest.

What it IS:
- Reasoning persistence layer (decisions survive sessions)
- Comparison discipline enforcer (Pareto, not recommendation essays)
- Evidence lifecycle manager (freshness, decay, drift)
- Governance governor (invariant verification, staleness alerts)

What it is NOT:
- Not a coding agent (doesn't compete with Claude Code on editing files)
- Not a pattern browser (doesn't expose FPF as a catalog)
- Not a documentation generator (persists reasoning artifacts, not specs)
- Not a project management tool (no sprints, no tickets, no Gantt charts)
- Not a general autonomous agent (no personal assistant, no omnichannel)

## Three delivery surfaces

### Surface A — Desktop App (primary: human)

The visual cockpit where the engineer lives during reasoning work.

- See: problem board, decision health, evidence quality, coverage, drift
- Think: frame problems, explore variants, compare on Pareto front, decide
- Act: spawn execution agents, verify claims, create PRs from decisions
- Govern: dashboard with findings, stale alerts, invariant violations

Technology: Wails (Go + native WebView). Single binary. Local-first.

### Surface B — MCP Plugin (primary: agent)

How AI agents access the reasoning kernel during their coding work.

- 6 reasoning tools: problem, solution, decision, query, refresh, note
- Stable API contract: tool names, required params, return shapes don't break
- Any MCP-compatible host: Claude Code, Codex, Cursor, Gemini CLI, Air

### Surface C — CLI (utility)

Quick access for scripting, CI, and terminal workflows.

- `haft init`, `haft serve`, `haft sync`, `haft board`, `haft search`
- `haft fpf search` (FPF spec lookup)
- `haft agent` (standalone agent mode — secondary to desktop)

**A and B are primary.** C is supporting utility.
Desktop is where humans think. MCP is where agents think. Same kernel underneath.

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
│                   │  Think → Run →     │         │
│                   │         Govern     │         │
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
| **Primary user** | Engineer using AI agent daily | Decisions that survive, honest comparisons, "what did we decide and why?" |
| **Host agent** | Claude Code, Codex, any MCP client | Clean tool interface, fast responses, no interference with coding workflow |
| **Solo engineer** | Working alone across multiple projects | Cross-project recall, accumulated judgment, local-first |
| **Tech lead** | Responsible for architectural consistency | Decision audit trail, staleness alerts, drift detection, coverage |
| **CI/CD pipeline** | Automated checks | `haft check` — verify decisions are fresh and evidence is current |
| **PR reviewer** | Reading diffs | `.haft/decisions/*.md` in the diff — rationale visible alongside code |

## Non-stakeholders (explicitly)

| Not for | Why |
|---------|-----|
| FPF researchers | Haft is a product, not an FPF reference implementation |
| Non-technical managers | No management dashboards — engineer-first |
| Compliance auditors (primary) | Audit views exist as secondary projections, not primary UX |
| Consumers / end users | No personal assistant surface |

## Key constraints

1. **Local-first.** Works without any server or cloud service.
2. **Solo-first.** Valuable for one engineer before needing teams.
3. **Desktop-first.** Desktop app is the primary human surface (not CLI, not web).
4. **Plugin-compatible.** MCP plugin is the highest-reach integration channel.
5. **FPF inside.** Users never need to learn FPF terminology. 5 words + Note.
6. **Single binary.** One `haft` binary serves desktop, MCP server, CLI, and agent mode.

## Enabling system (what builds Haft)

The enabling system is NOT the runtime. It's the "third factory":

- SoTA harvesting (Symphony, Zenflow, Hermes, Air — what patterns to adopt)
- Parity benchmarks (seeded corpus, catch rate, false positive rate)
- Workflow R&D (how to improve Think → Run → Govern cycle)
- FPF formalization (which of 214 patterns need L2/L3 enforcement)
- Semiotics review (term drift, authority confusion, gate/evidence mixing)

Creator: Ivan Zakutnii + AI coding agents. Solo developer with AI leverage.
