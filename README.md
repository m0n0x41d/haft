<img src="assets/banner.svg" alt="Quint Code" width="600">

**Frame problems. Compare options fairly. Know when your decisions need revisiting.**

Supports: Claude Code, Cursor, Gemini CLI, Codex CLI

---

## What Quint Code Does

Quint Code is an FPF-native reasoning layer for engineering decisions. It helps you:

1. **Frame the actual problem** before jumping to solutions
2. **Compare options in a fair characteristic space** instead of arguing on vibes
3. **Record decisions with invariants, rollback plans, and refresh triggers**
4. **Detect when reasoning goes stale** and needs revisiting

Everything lives in `.quint/` as git-tracked markdown files. Searchable. Auditable. Yours.

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
  -> Recorded. Searchable. Linked to affected files.
```

### Tactical choices: `/q-frame` -> `/q-explore` -> `/q-decide`

```
/q-frame "Rate limiting on public API — scraper traffic causing degraded response times"
/q-explore — generates 3 variants with weakest link per option
/q-decide — records which variant, why, what to watch for, when to revisit
```

### Architectural decisions: full flow

```
/q-frame    — define the problem, constraints, acceptance criteria
/q-char     — define comparison dimensions (throughput, ops complexity, cost)
/q-explore  — generate genuinely distinct variants with WLNK per option
/q-compare  — fair parity comparison, Pareto front, non-dominated set
/q-decide   — full DecisionRecord with invariants, pre/post-conditions, rollback
/q-apply    — generate implementation brief from the decision
```

### When decisions go stale: `/q-refresh`

```
/q-status   — shows what's expired, what needs attention
/q-refresh  — waive (extend), reopen (new problem cycle), supersede, or deprecate
```

---

## 6 Tools, 11 Commands

| Tool | What it does | Commands |
|------|-------------|----------|
| `quint_note` | Micro-decisions with validation | `/q-note` |
| `quint_problem` | Frame problems, define comparison space | `/q-frame` `/q-char` `/q-problems` |
| `quint_solution` | Explore variants, compare on Pareto front | `/q-explore` `/q-compare` |
| `quint_decision` | Decide with full rationale, generate impl brief | `/q-decide` `/q-apply` |
| `quint_refresh` | Detect stale decisions, manage lifecycle | `/q-refresh` |
| `quint_query` | Search, status dashboard, file lookups | `/q-search` `/q-status` |

Plus: `quint-code fpf search` for deep FPF methodology lookups.

---

## Decision Modes

| Mode | When | What you get |
|------|------|-------------|
| **note** | Micro-decisions during coding | Note with rationale |
| **tactical** | Reversible, < 2 weeks impact | Problem + Decision (light) |
| **standard** | Most architectural decisions | Problem + Portfolio + Decision |
| **deep** | Irreversible, cross-team, security | All standard + parity, runbook, refresh |

---

## What Gets Recorded

Decisions are markdown files with YAML frontmatter in `.quint/`:

```
.quint/
  notes/        — quick decisions
  problems/     — framed problems
  solutions/    — variant portfolios with comparison
  decisions/    — full decision records
  evidence/     — evidence packs
  refresh/      — refresh reports
  quint.db      — SQLite index for search and status
```

Every file is git-tracked, human-readable, and searchable via `quint_query`.

---

## DecisionRecord — The Crown Jewel

A full DRR contains:

- **Selected Variant** — what was chosen
- **Why This, Not Others** — comparison table
- **Invariants** — what MUST hold at all times
- **Pre-conditions** — checklist before implementation
- **Post-conditions** — definition of done
- **Admissibility** — what is NOT acceptable
- **Evidence Requirements** — what to measure
- **Rollback Plan** — triggers, steps, blast radius
- **Refresh Triggers** — when to re-evaluate
- **Weakest Link** — what bounds reliability

A new engineer reads this 6 months later and understands everything.

---

## How It Differs

| | OpenSpec | BMAD | Quint Code |
|---|---------|------|------------|
| **Focus** | Spec before code | Agile workflow | Problem before spec |
| **Upstream reasoning** | None | Weak (PRD) | Frame → characterize → compare → decide |
| **Staleness tracking** | None | None | Refresh triggers, validity expiry |
| **Artifact count** | 3 | 10+ | 6 |
| **MCP tools** | N/A | N/A | 6 (hard cap) |
| **Ceremony** | Low | High | Scales with decision importance |

---

## Requirements

- Go 1.24+ (for building from source)
- Any MCP-capable AI tool (Claude Code, Cursor, Gemini CLI, Codex CLI)

## License

MIT
