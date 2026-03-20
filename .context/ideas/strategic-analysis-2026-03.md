# Strategic Analysis: Quint Code Next Moves (March 2026)

## Context

Initial analysis March 18, 2026. **Updated March 19** after shipping cross-project recall, unified storage, adversarial verification, evidence integrity, and note dedup. Re-analyzed with deep FPF research (E.16 Autonomy Budget, A.12 Transformer Mandate, A.21 Gate Decisions, A.2.8 Commitment Governance) and market research (Straion, Beads/Dolt, MCP multi-agent patterns, CRDT collaboration).

Inputs:
- FPF slideument (74 slides, Levenchuk Feb 2026)
- FPF spec patterns: E.16, A.12, A.13, A.21, A.2.8, B.2.3, F.9
- 18 idea files in `.context/ideas/`
- 8 reference repos in `.context/repos/`
- Quint Code 5.1.0-dev (Go MCP server, 6 tools, 11 commands, unified storage, cross-project recall, 4-language codebase awareness)
- Market: Straion (team ADR SaaS), Beads (Dolt-based agent coordination), ADR.github.io ecosystem

## Product Stage

**Solo developer features: COMPLETE.** The individual developer experience is fully built:
- Full FPF reasoning cycle (frame → char → explore → compare → decide → measure → refresh)
- Codebase awareness (modules, dependencies, coverage, drift detection)
- Cross-project decision recall with CL matching
- Adversarial verification, inductive measurement gates
- Evidence integrity (supersession, R_eff, CL-based scoring)
- Note dedup, Shipped/Pending split, structured logging

**The product boundary has shifted.** Further solo-dev features (CLI dashboard, cognitive loops) are polish. The next category-defining move is **team collaboration** — where no tool exists.

## Market Gap Analysis

| Tier | Tools | What they do | What's missing |
|------|-------|-------------|----------------|
| **Tier 1** (90% market) | Plain ADRs in git, adr-tools | Markdown files in `docs/adr/` | Everything — no review, no staleness, no evidence, no search |
| **Tier 2** | Straion, Confluence, Notion | Process management (DACI roles, due dates, Slack integration) | Computed trust (R_eff), evidence decay, cross-project recall, codebase connection |
| **Tier 3** | **Nobody** | Structured decisions with computed trust, evidence chains, and codebase awareness | This is where Quint operates. No competition. |

**Straion** is the closest competitor: team ADR platform with DACI roles, templates, Git/Slack integration. But it treats decisions as static documents, not computable artifacts. No R_eff, no evidence decay, no drift detection.

**Beads** is the closest technical peer: Dolt-based agent coordination with claim mechanism, file reservation, federation. But no reasoning framework — it coordinates work, not knowledge.

## MOTO Calculation Table (Updated)

MOTO = Revolutionary Impact × Feasibility. Scale: 1-10 each.

### Shipped (v5.1.0)

| Idea | Impact | Feasibility | MOTO | Status |
|------|--------|-------------|------|--------|
| Codebase Awareness | 9 | 7 | **63** | ✅ SHIPPED — drift detection, module map (4 langs), dependency graph, R_eff coverage |
| Reality-Aware Decisions | 8 | 7 | **56** | ✅ SHIPPED — baseline snapshots, drift detection, impact propagation |
| Engineering Judgment Accumulator | 9 | 8 | **72** | ✅ SHIPPED — unified storage, index.db, cross-project recall, CL matching |
| Adversarial Verification | 8 | 7 | **56** | ✅ SHIPPED — 5-probe verification gate, inductive measurement gate, CL-based scoring |

### Next Moves (ranked by updated MOTO)

| Rank | Idea | Impact | Feasibility | MOTO | Why this ranking |
|------|------|--------|-------------|------|-----------------|
| **1** | **Team Decision Collaboration** | 10 | 6 | **60** | No competitor. Architecture prepared (unified storage, project ID, QUINT_SERVER_ORIGIN). FPF E.16/A.21 provide theoretical framework. Market gap is complete — Straion does process, nobody does computed trust for teams. |
| **2** | **Decision-as-Infrastructure** | 9 | 5 | **45** | `quint-code check` in CI. R_eff as merge gate. PR comments with decision health. Transforms Quint from "developer tool" to "team infrastructure." Compounds with team collab. |
| **3** | **CLI Dashboard** | 6 | 9 | **54** | Highest feasibility (2-3 days, all data exists). The "screenshot for the blog post." But it's presentation, not category expansion. |
| **4** | **Cognitive Review Loops** | 7 | 6 | **42** | FSRS-based review scheduling. Background contemplation. Requires long-running process (server mode). Phase 2+ feature. |
| **5** | **Standalone Agent** | 10 | 4 | **40** | Highest ceiling but market is crowded with coding agents. Differentiation unclear until team features prove the value proposition. |
| **6** | **Autonomy Budget System** | 8 | 5 | **40** | FPF E.16. Typed delegation with guards, SoD, override protocol. Critical for team mode but needs server infrastructure first. |
| **7** | **Disagreement Protocol** | 7 | 7 | **49** | Track human vs agent disagreements. Valuable for team insights. Low effort but needs team context to be meaningful. |

## Deep Analysis: Team Decision Collaboration

### Why This Is #1

**Market:** Nobody does this. Straion manages decision process (who approves). Quint would manage decision quality (is the evidence still valid, did reality diverge, which decisions affect which modules).

**Architecture:** Already prepared:
- Unified storage with project ID → multi-project server ready
- `QUINT_SERVER_ORIGIN` env var → remote server ready
- Cross-project recall via index.db → team knowledge sharing ready
- Markdown projections in git → async collaboration ready

**FPF theory (researched):**

| FPF Pattern | What it says about team collaboration |
|-------------|--------------------------------------|
| **E.16 (Autonomy Budget)** | Delegation is a typed budget with guards, ledger, override SpeechActs. SoD structurally enforced. "Autonomy-by-label" without a budget declaration is an anti-pattern. |
| **A.12 (Transformer Mandate)** | No self-review. Teams reviewing their own work must model review as a distinct role (Reflexive Split). |
| **A.21 (Gate Decisions)** | Worst verdict wins (join-semilattice). No voting, no averaging. `abstain ≤ pass ≤ degrade ≤ block`. |
| **A.2.8 (Commitment)** | Lifecycle changes are explicit. When updated/superseded/revoked, new object + SpeechAct. No silent mutation. |
| **F.9 (Bridges)** | Cross-team knowledge must cite a Bridge with CL, loss notes. "Coordinate-by-name without a Bridge fails." |
| **A.13 (Collective Agency)** | Team is not "set of people" — it's a U.System with boundary, coordination Method, and measurable Agency-CHR. |

### Technical Approach (from research)

**CRDT insight:** Most decision artifacts are append-only (problems, variants, evidence, notes). These are natural CRDTs — merge automatically. Only `/q-decide` (selecting variant) and `/q-refresh` (lifecycle change) can conflict. For those two operations: human arbitration.

| Conflict type | Resolution |
|---------------|-----------|
| Two people frame same problem | Additive — both frames captured |
| Two people explore variants | Additive — all variants visible |
| Two people decide differently | Cannot auto-merge. Surface as competing decisions. Human resolves. |
| Evidence added by different people | Additive — no conflict |
| R_eff drops below threshold | Notification, not auto-action |
| Decision refreshed by different people | Last-writer-wins with audit trail |

**Implementation: Hybrid (server for real-time, git for async)**

```
Real-time: Agent → MCP → quint-code serve → quint-code server → PostgreSQL/SQLite
Async:     quint-code serve → writes .quint/*.md → git push/pull → merge
```

This matches how teams actually work — some in the same office (real-time), some remote (async via git).

### Phase 2 Scope (Team Collaboration)

| Feature | Effort | Compounds with |
|---------|--------|---------------|
| `quint-code server` command (thin HTTP server over Store interface) | 1-2 weeks | Everything below |
| RemoteStore (HTTP client in `serve`) | 1 week | Server mode |
| Version compatibility check (client ↔ server) | 1 day | Server mode |
| Team-visible `/q-status` (who decided what, when) | 2-3 days | Server mode |
| Notification on decision staleness (webhook/Slack) | 2-3 days | Server mode |
| `quint-code check` CLI for CI integration | 1 week | Decision-as-Infrastructure |
| Autonomy Budget declarations (FPF E.16) | 2 weeks | Team governance |

### What Beads Teaches Us

| Pattern | Beads approach | Quint adaptation |
|---------|---------------|-----------------|
| Multi-writer coordination | Dolt cell-level merge | PostgreSQL/SQLite with event-based sync |
| Work claiming | `bd update --claim` | Decision ownership via `affected_files` + `project_id` |
| Conflict prevention | File reservation | Decision-level locks (optional, for `/q-decide`) |
| Federation | Dolt remotes (DoltHub, S3) | Git for markdown projections, server for real-time |
| Stealth mode | No git integration option | Already have — Quint works without git |

## Recommended Sequence (Updated)

### v5.1.0 (current release scope — NEARLY COMPLETE)
Everything shipped + changelog update + release.

### v5.2.0 — Team Foundation
1. **CLI Dashboard** (`quint-code status`) — quick win, demo-able, 2-3 days
2. **`quint-code server`** — thin HTTP layer over Store, SQLite backend
3. **RemoteStore** in `serve` — connects to server via QUINT_SERVER_ORIGIN
4. **Version check** — client reports version, server warns on mismatch
5. **Team-visible dashboard** — who decided what, R_eff across all team decisions

### v5.3.0 — Team Intelligence
6. **Decision-as-Infrastructure** — `quint-code check` in CI, PR comments
7. **Notifications** — Slack/webhook on staleness, drift, R_eff degradation
8. **Autonomy Budget** — FPF E.16 declarations, SoD enforcement
9. **Cognitive Review Loops** — FSRS scheduling, background contemplation (requires server)

### Strategic Bet (parallel track)
10. **Standalone Agent** — own runtime with lemniscate loop. Only pursue when team features prove the value proposition.

## Key Risks

1. **Server complexity** — building and maintaining a server is a different game from an MCP plugin. Ops burden, auth, deployment, monitoring.
2. **Market timing** — team tools need adoption. Solo dev is "install and go." Team is "convince your lead, then your team, then your org."
3. **Beads competition** — if Beads adds reasoning/evidence framework, it has a head start on multi-agent coordination.
4. **Straion competition** — if Straion adds computed trust (R_eff-like), it has existing team user base.

## Differentiators That No Competitor Can Easily Replicate

1. **R_eff with evidence decay** — computed trust scores that degrade over time. Requires the full evidence model (CL, supersession, inductive gates).
2. **Codebase awareness connected to decisions** — module coverage, dependency impact propagation. Nobody else connects code structure to decisions.
3. **Cross-project recall with CL penalties** — FPF-native knowledge transfer across project boundaries.
4. **Adversarial verification gate** — decisions challenged before recording. Grounded in FPF A.12 + Verbalized Sampling research.
5. **The lemniscate** — the full cycle from problem → decision → evidence → reframe. No other tool implements this as a first-class concept.

## Success Metric

After v5.2.0, Quint should be able to answer — for a team of 3-5 engineers:

1. "Which of our team's decisions are stale?" → Team-wide R_eff dashboard
2. "What did we decide about auth, and is it still valid?" → Cross-project recall + drift status
3. "Should this PR be blocked?" → `quint-code check` with R_eff gate
4. "Who owns decisions about this module?" → Module coverage × team ownership
5. "Where do our agents disagree?" → Disagreement protocol (future)

If it can answer all five, the narrative writes itself: **"The first tool that makes engineering judgment a team asset, not a tribal artifact."**
