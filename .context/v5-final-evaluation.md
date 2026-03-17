# Quint Code v5 — Final Evaluation

Date: 2026-03-17
Method: FPF Deep — against v5.1 plan, FPF-Spec.md, slideument
Stage: Evidence

---

## 1. Coverage of the Three Factories

### Factory 1: Problem Factory (left cycle of lemniscate)

**Slideument requires**: characterization → indicatorization → measurement → problem portfolio → Goldilocks selection → comparison & acceptance spec.

| Step | Implemented? | How |
|------|-------------|-----|
| Signal/anomaly capture | Yes | ProblemCard `signal` field |
| Characterization (dimensions, scales, polarity) | Yes | `quint_problem(action="characterize")` with versioned history |
| Indicatorization (constraints vs targets vs observation) | Yes | ProblemCard fields: `constraints`, `optimization_targets`, `observation_indicators` |
| Problem portfolio | Partial | `quint_problem(action="select")` lists problems with Goldilocks signals, but doesn't actually HELP select — no scoring, no "too easy/too hard" computation |
| Goldilocks selection | Partial | Shows blast radius and reversibility from body text. No computation. The agent must do the reasoning. |
| Comparison & acceptance spec as handoff | No | Buried inside ProblemCard body. Not a locked/versioned subsection referenced by explore/compare |
| Stale problem detection | Yes | `FindStaleArtifacts` checks valid_until on ProblemCards |
| Diagnostic framing | Yes (skill-level) | SKILL.md 7-question protocol teaches agent to drive conversation |

**Verdict: 60% implemented.** The recording side works. The intelligence side (Goldilocks scoring, handoff locking) doesn't. The SKILL.md partially compensates by teaching the agent to do the reasoning work.

### Factory 2: Solution Factory (right cycle)

**Slideument requires**: adopt spec → variant generation → architecture generation → parity run → selection policy → record decision → reversible change → commission → impact measurement → evidence pack.

| Step | Implemented? | How |
|------|-------------|-----|
| Variant generation | Yes | `quint_solution(action="explore")` with ≥2 variants, WLNK required |
| Parity comparison | Partial | `quint_solution(action="compare")` stores scores + non-dominated set, but parity_rules are text, not verified |
| Selection policy | Yes (textual) | Stored in comparison results |
| Record decision | Yes | Full DRR with invariants, pre/post-conditions, admissibility, rollback, refresh triggers, WLNK |
| Implementation brief | Yes | `quint_decision(action="apply")` |
| Impact measurement | Yes | `quint_decision(action="measure")` records findings against criteria |
| Evidence pack | Yes | `quint_decision(action="evidence")` attaches items with type/verdict/CL/F |
| WLNK summary | Yes (tracked) | `ComputeWLNKSummary` shows evidence chain: count, verdicts, min CL, freshness |
| Refresh / stale detection | Yes | `quint_refresh` with scan/waive/reopen/supersede/deprecate |
| Reopen with lineage | Yes | Prior characterization, failure reason, evidence refs carry forward |

**Verdict: 80% implemented.** The full cycle works end-to-end. Weak spots: parity is textual, no NQD computation. But the structure is correct and the artifacts capture the right information.

### Factory 3: Factory Factory (governance / org-level)

**Slideument**: org-level process, autonomy budgets, cadence, policy packs.

**Not implemented. By design (ADR, v5.1 plan).** Correct decision for this cycle.

---

## 2. What's Strong

1. **Action-based surface.** 6 tools, 11 commands. No phase numbers, no ceremony. A real engineer can use this without learning FPF theory.

2. **DRR quality.** The DecisionRecord with invariants, pre/post-conditions, admissibility, rollback, refresh triggers — this is genuinely useful operational documentation, not a ceremony artifact.

3. **Note validation.** The rationale/conflict/scope checks are real logic, not just fields. This is the product's most "alive" feature.

4. **Refresh lifecycle.** scan → waive/reopen/supersede/deprecate with lineage carry-forward. This is unique. No other tool does this.

5. **Dual-write clarity.** DB is primary, files are projections. Documented honestly. No confusion.

6. **Maturity honesty.** Every FPF concept tagged textual/tracked/computed/enforced. The skill doesn't oversell.

7. **FPF spec search.** 4243 sections embedded in binary. The agent can look up any FPF concept on demand.

---

## 3. What's Critically Missing

### M1. No brownfield onboarding story

**`quint-code init` creates empty `.quint/` and leaves.** On a brownfield project with existing ADRs, architecture docs, README, test plans — zero is discovered. The user must manually `/q-note` everything.

**What should happen on brownfield**:
```
$ quint-code init
  ✓ Created .quint/ directory structure
  ✓ Initialized database
  ✓ Configured MCP for Claude Code
  ✓ Installed 11 slash commands
  ✓ Installed /q-reason skill

  Detected existing artifacts:
    3 ADRs in docs/adr/
    1 architecture.md
    2 test plans

  Use /q-status to review. Consider importing with quint_query.
```

This doesn't exist. A brownfield user gets the same empty state as greenfield.

### M2. No `ingest` capability

The refactor spec (Section 15) required: "Quint should pick up existing ADRs, architecture docs, TODO docs, OpenSpec files, design docs, benchmark notes." None of this was built. The `quint_query(action="related")` finds artifacts by affected file, but there's no way to import external docs as evidence or candidate artifacts.

### M3. No `doctor` command

The refactor spec (Section 11.3) required: check `.quint/` integrity, schema version, broken refs, stale evidence summary. Not built. If the DB gets corrupted or a file is deleted, there's no diagnostic.

### M4. Comparison & Acceptance Spec is not a formal handoff

The slideument (slide 14, line 274) says this is the formal exit of Problem Factory and formal entry of Solution Factory. In our implementation, it's just fields inside ProblemCard. The solution tools don't verify that a proper acceptance spec exists before accepting variants.

---

## 4. User Stories: Greenfield vs Brownfield

### Greenfield: "New Go microservice, choosing event infrastructure"

```
1. quint-code init                      → .quint/ created, MCP configured
2. /q-frame "Event infrastructure for domain events — DB polling hitting 70% CPU"
   Agent asks the 7 diagnostic questions:
   - What observation doesn't fit? "DB CPU at 70%, 3-month runway"
   - Constraints? "At-least-once delivery, <500ms p99"
   - Acceptance? "Sustained 100k events/sec with p99 < 50ms"
   → ProblemCard created in .quint/problems/

3. /q-char — agent adds comparison dimensions:
   throughput, ops complexity, cost, migration effort
   → Characterization v1 appended to ProblemCard

4. /q-explore — agent generates 3 variants:
   Kafka (WLNK: ops), NATS (WLNK: ecosystem maturity), Redis Streams (WLNK: durability)
   → SolutionPortfolio created in .quint/solutions/

5. /q-compare — agent fills comparison table, identifies Pareto front:
   Non-dominated: {Kafka, NATS}
   → Comparison added to portfolio

6. /q-decide — user picks NATS:
   Full DRR with invariants, pre/post-conditions, rollback plan, refresh triggers
   → DecisionRecord created in .quint/decisions/

7. /q-apply — generates implementation brief
   → User/agent implements

8. quint_decision(action="measure") — records: "11/12 producers migrated, 120k/s achieved"
   → Impact section added to DRR, evidence item created

9. 3 months later: /q-status shows "dec-001 valid_until expired"
   /q-refresh → reopen with lineage
   → New ProblemCard inherits prior characterization
```

**This flow works today.** All tools are implemented. The agent reads the SKILL.md and follows the protocol. The artifacts are created in `.quint/` and searchable.

### Brownfield: "Join existing project, need to understand past decisions"

```
1. quint-code init                      → .quint/ created
   BUT: no existing knowledge discovered

2. Developer asks: "why are we using NATS?"
   Agent calls: quint_query(action="search", query="NATS")
   → Nothing found (empty DB)

   Developer has to explain: "the ADR is in docs/adr/003-nats.md"
   Agent reads the file but CAN'T import it into Quint

3. To make it searchable, developer must manually:
   /q-note "NATS JetStream for events — chosen for ops simplicity, see docs/adr/003-nats.md"

4. Over time, notes accumulate. But there's no way to bulk-import existing docs.
```

**This flow is painful.** The brownfield story is the weakest part of the product. A real engineer joining an existing project gets zero value from Quint until they manually re-enter past decisions.

### What's needed for brownfield:
1. `quint-code init` should scan for common knowledge artifacts (ADRs, architecture docs, README)
2. `quint_query(action="ingest")` or a CLI `quint-code ingest` should import external files as Notes or evidence
3. At minimum: the agent should be taught (via SKILL.md) to proactively read existing docs and suggest creating Notes from them

---

## 5. How Notes Work as Structural Memory

Notes are the **long tail** of engineering decisions. Most decisions during coding are too small for a ProblemCard but too important to lose.

**What works:**
- Rationale validation prevents garbage
- Conflict detection catches contradictions with active decisions
- Scope check escalates architectural decisions to `/q-frame`
- FTS5 makes notes searchable: `quint_query(action="search", query="mutex")` finds past notes
- `quint_query(action="related", file="cache.go")` finds notes linked to specific files

**What doesn't work:**
- Notes have no explicit link to each other. If you made 3 related notes about caching, they don't form a cluster.
- Notes can't be "promoted" to a ProblemCard. If a pattern of notes reveals a bigger problem, you start from scratch.
- No note aggregation: `/q-status` shows "5 recent notes" but no summary of what they say.

**Structural memory verdict:** Notes work as individual records. They don't work as a knowledge graph. For a solo developer this is fine. For a team inheriting the project, it's a flat list.

---

## 6. What to "Sell" — the MOTO

The product's message should be built on what ACTUALLY WORKS, not aspirations.

**What actually works and is unique:**

1. **"Why did we decide X?"** — searchable decision records that survive chat session endings. No other tool does this.

2. **"Is this decision still valid?"** — automatic staleness detection with `valid_until`. Show me what's expired. Let me waive, reopen, or supersede.

3. **"What are the real options?"** — structured comparison with variants, weakest links, and non-dominated set. Not "pick one," but "here are the trade-offs."

4. **"Don't just do X — tell me why."** — Note validation forces rationale. Catches conflicts with existing decisions. Escalates big decisions.

5. **"What happens when I touch this file?"** — file-to-decision lookup. Which decisions affect this code?

**Proposed MOTO:**

> **Quint Code: engineering decisions that outlive the chat session.**
> Frame problems. Compare options. Record rationale. Know when to revisit.

**What NOT to sell yet:**
- "FPF-native" — too academic for most engineers
- "Pareto front" — jargon
- "Lemniscate cycle" — nobody knows what this means
- "WLNK computation" — it's tracked, not computed
- "Fair comparison" — parity is textual, not enforced

Sell the OUTCOMES: decisions survive, reasoning is searchable, staleness is visible, rationale is required. The FPF methodology is the engine under the hood, not the brand.

---

## 7. Quality Verification Plan

How to prove this works:

### Smoke test (10 minutes)
```
1. quint-code init in a new dir
2. /q-note "test note" — should be rejected (no rationale)
3. /q-note "using X because Y" — should be accepted
4. /q-frame "test problem" — signal, constraints, targets
5. /q-explore — 2 variants with WLNK
6. /q-decide — select one
7. /q-status — shows 1 decision, 1 problem, 1 note
8. /q-search "test" — finds all three
```

### Integration test (1 hour)
Full tactical and standard flows on a real brownfield project. Create 3 decisions, let one expire, refresh it. Verify files in `.quint/` are readable. Verify search finds everything.

### Dogfooding (ongoing)
Use Quint on Quint development itself. Frame the next problem with `/q-frame`. Record architecture decisions with `/q-decide`. See what breaks in actual use.

---

## 8. Gap Summary with Priorities

| # | Gap | Impact | Effort | Priority |
|---|-----|--------|--------|----------|
| 1 | **No brownfield onboarding** — init doesn't discover existing knowledge | Critical for adoption | Medium (scan for ADR/docs patterns in init, teach SKILL.md to suggest import) | **P0** |
| 2 | **No ingest** — can't import external docs as evidence/notes | High for brownfield | Medium (CLI command or query action) | **P1** |
| 3 | **Comparison & acceptance spec not a formal handoff** — just fields in ProblemCard | Medium for FPF fidelity | Low (add locked section, check in explore) | **P2** |
| 4 | **No doctor** — can't check .quint/ integrity | Medium for reliability | Low | **P2** |
| 5 | **Notes don't cluster** — related notes have no group structure | Low for solo dev, medium for teams | Medium | **P3** |
| 6 | **Parity not verified** — stored as text | Low for v5.0 | Medium | **P3** |
| 7 | **NQD not implemented** — novelty/diversity not tracked | Low (academic concept) | High | **P4** |

---

## 9. Honest Assessment

**Как далеко от плана?** 85% реализовано. 6 тулов, все артефакты, refresh lifecycle, impact measurement, evidence, WLNK tracked, lineage. Главное упущение — brownfield onboarding.

**Реализуем ли три фабрики?** Фабрика решений — 80%. Фабрика проблем — 60%. Фабрика фабрик — 0% (по плану). Общий счёт: мы ближе к "solution-centric with problem support" чем к "problem-first product". Но для v5.0 это honest scope.

**Понятен ли продукт пользователю?** README и SKILL.md чётко объясняют что делать. Maturity table не врёт. Brownfield user gets nothing from init though — и это главная UX-проблема.

**MOTO:** "Engineering decisions that outlive the chat session." Продавать: searchable decisions, staleness detection, rationale enforcement, file-to-decision lookup. НЕ продавать: FPF jargon, computed WLNK, Pareto front.

**WLNK текущей реализации:** brownfield onboarding. Это то, что bounds product quality для реальных пользователей. Пока init создаёт пустую директорию на проекте с 50 ADR — продукт не для brownfield.
