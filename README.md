<img src="assets/banner.svg" alt="Quint Code" width="600">

**Frame problems. Compare options fairly. Know when your decisions need revisiting.**

Supports: Claude Code, Cursor, Gemini CLI, Codex CLI

---

## What Quint Code Does

Quint Code is an FPF-native reasoning layer for engineering decisions. It helps you:

1. **Frame the actual problem** before jumping to solutions
2. **Structure the comparison space** — define dimensions, record variants with weakest links
3. **Record decisions as engineering contracts** — invariants, DO/DON'T, rollback plans, refresh triggers
4. **Detect when decisions go stale** — expired validity and degraded evidence surfaced automatically
5. **Manage artifact lifecycle** — notes, problems, decisions all have refresh, deprecation, and supersession

**Storage model:** SQLite is the source of truth for the engine (search, stale detection, WLNK computation, link traversal). Markdown files in `.quint/` are written as git-friendly projections — readable, diffable, portable, but not what the engine reads. See [why DB is primary](#why-db-is-primary).

---

## Quick Start

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/m0n0x41d/quint-code/main/install.sh | bash

# Initialize in your project
cd your-project
quint-code init

# Start using
# /q-note   — capture a quick decision
# /q-frame  — frame a problem properly
# /q-reason — structured FPF reasoning
```

---

## How It Works

### Quick decisions: `/q-note`

```
Dev: "using RWMutex for session cache — contention <0.1% per load test"

Quint validates:
  - Rationale provided? Yes
  - Conflicts with active decisions? No
  - Scope too large for a note? No
  -> Recorded. Searchable. Auto-expires in 90 days.
```

### Tactical choices: `/q-frame` -> `/q-explore` -> `/q-decide`

```
/q-frame "Rate limiting on public API — scraper traffic causing degraded response times"
/q-explore — generates 3 variants with weakest link per option
/q-decide — full DRR with invariants, pre/post-conditions, rollback
```

### Architectural decisions: full flow

```
/q-frame    — define the problem, constraints, acceptance criteria
/q-char     — define comparison dimensions (throughput, ops complexity, cost)
/q-explore  — generate genuinely distinct variants, label weakest link per option
/q-compare  — record comparison results, identify non-dominated set (Pareto front)
/q-decide   — full DecisionRecord as FPF E.9 engineering contract
```

### When decisions go stale: `/q-refresh`

```
/q-status   — shows what's expired, what needs attention, open vs addressed problems
/q-refresh  — waive (extend), reopen (new problem cycle), supersede, or deprecate
             works on ALL artifact types: notes, problems, decisions, portfolios
```

---

## 6 Tools, 11 Commands

| Tool | What it does | Commands |
|------|-------------|----------|
| `quint_note` | Micro-decisions with validation + auto-expiry | `/q-note` |
| `quint_problem` | Frame problems, define comparison space | `/q-frame` `/q-char` `/q-problems` |
| `quint_solution` | Explore variants with WLNK, diversity check, compare with parity | `/q-explore` `/q-compare` |
| `quint_decision` | Decide with full FPF E.9 rationale, measure impact, attach evidence | `/q-decide` |
| `quint_refresh` | Manage lifecycle for ALL artifacts — detect stale, waive, deprecate | `/q-refresh` |
| `quint_query` | Search, status dashboard, file lookups, full artifact listing | `/q-search` `/q-status` |

Plus: `quint-code fpf search` for deep FPF methodology lookups.

---

## Decision Modes

| Mode | When | What you get |
|------|------|-------------|
| **note** | Micro-decisions during coding | Note with rationale, auto-expires 90 days |
| **tactical** | Reversible, < 2 weeks impact | Problem + Decision (light) |
| **standard** | Most architectural decisions | Problem + Portfolio + Decision |
| **deep** | Irreversible, cross-team, security | All standard + parity, runbook, refresh |

---

## What Gets Recorded

Artifacts are markdown files with YAML frontmatter in `.quint/`:

```
.quint/
  notes/        — quick decisions (auto-expire at 90 days)
  problems/     — framed problems with characterization
  solutions/    — variant portfolios with comparison + diversity check
  decisions/    — FPF E.9 decision records (the crown jewel)
  evidence/     — evidence packs
  refresh/      — refresh reports
  quint.db      — SQLite engine (search, stale detection, WLNK, links)
```

Every file is git-tracked, human-readable, and searchable via `quint_query`.

---

## DecisionRecord — FPF E.9 Engineering Contract

A full DRR follows the FPF E.9 four-component structure:

### 1. Problem Frame
- **Signal** — what's anomalous (pulled from linked ProblemCard)
- **Constraints** — hard limits
- **Acceptance** — how we know it's solved

### 2. Decision (the contract)
- **Selected variant** — what was chosen and why
- **Invariants** — what MUST hold at all times
- **Pre-conditions** — checklist before implementation
- **Post-conditions** — definition of done
- **Admissibility** — what is NOT acceptable

### 3. Rationale
- **Why this, not others** — comparison table
- **Weakest link** — what bounds reliability
- **Evidence requirements** — what to measure

### 4. Consequences
- **Rollback plan** — triggers, steps, blast radius
- **Refresh triggers** — when to re-evaluate
- **Affected files** — what code is touched

A new engineer reads this 6 months later and understands everything — context, contract, reasoning, and risk.

---

## Computed Features

Quint doesn't just store — it computes:

| Feature | What it does |
|---------|-------------|
| **R_eff** | Effective reliability = min(evidence scores) with CL penalties. WLNK principle: never average. |
| **Evidence decay** | Expired evidence scores 0.1 (weak, not absent). R_eff < 0.5 triggers stale detection. |
| **Epistemic debt** | Graduated staleness severity — most overdue items shown first, debt magnitude displayed. |
| **Note validation** | Rationale check + conflict detection against active decisions + scope gate. |
| **Diversity check** | Jaccard similarity on variants — warns when explore submissions look too similar. |
| **Archive recall** | FTS5 search on title at frame/explore time — surfaces related past decisions and notes. |
| **Characterization cross-check** | Compare warns when dimensions don't match characterization, scores are asymmetric, or parity rules exist. |
| **Parity checklist** | Auto-generated per-dimension parity questions from characterization. |
| **Problem lifecycle** | Status dashboard splits open vs addressed problems via backlink analysis. |

---

## Why DB Is Primary

Quint Code uses SQLite as the engine's source of truth, not the markdown files. This isn't an accident — it follows from what the product does.

**The lemniscate cycle requires computation, not file reading.** Stale detection scans `valid_until` across all artifacts. R_eff aggregates evidence items with CL penalties and freshness decay. Search uses FTS5 indexes. Status computes derived state from artifact completeness. Link traversal finds what problem a decision is based on. All of these are SQL operations — indexed, transactional, fast.

If files were primary, every tool call would parse N markdown files, build an in-memory graph, and hope nothing is malformed. That's slow, fragile, and non-atomic.

**FPF distinction: Object ≠ Carrier.** The artifact model (the object — its state, links, evidence) lives in the DB. The markdown file (the carrier — human-readable, git-tracked) is a projection. Confusing the carrier with the object is the exact anti-pattern FPF warns about: treating the description as the thing it describes.

**What the files are for:**
- Git history — who changed what decision, when
- Human review — read a DRR without running Quint
- Code review — PRs include decision artifacts as readable diffs
- Portability — copy `.quint/` to another machine

**What the files are NOT for:**
- Engine queries (use `quint_query`)
- Stale detection (use `quint_refresh`)
- Link traversal (use `quint_query(action="related")`)

If you manually edit a `.quint/*.md` file, the DB won't know. The engine will continue using the DB version. A future `sync` command may reimport edited files.

---

## Requirements

- Go 1.24+ (for building from source)
- Any MCP-capable AI tool (Claude Code, Cursor, Gemini CLI, Codex CLI)

## License

MIT
