---
name: fpf-development
description: FPF development-for-the-developed discipline for engineering work — problem factory vs solution factory, Goldilocks problem selection, characterization and honest comparison, NQD portfolios with stepping stones, evidence decay, autonomy budgets. Use when framing problems, generating or comparing solution variants, planning improvements, or judging whether work is actually done.
---

# Development discipline (Levenchuk 2026 seminar distillate)

Cheap solution generation made problem framing the scarce skill. This skill
carries the operating principles; persistence always goes through the native
`haft_*` tools — a chat answer is wishlist, not work.

## Three factories

1. **Problem factory** — produces framed problems: signal → characterization
   → problem portfolio → comparison & acceptance spec. Output is a testable
   acceptance spec, not a wish. In haft: `haft_problem(action="frame")`.
2. **Solution factory** — produces solution portfolios for a framed problem:
   variants → parity comparison → selection by declared rule → reversible
   change → impact measurement → evidence pack. In haft: `haft_solution`,
   then manual `/h-decide`.
3. **Factory of factories** — improves both production lines (org/tooling).

Never confuse "which problem to solve" (factory 1) with "how to solve it"
(factory 2). Pre-project R&D is also problem-hypothesis production.

## A problem is an acceptance spec

A problem exists when no known method reliably passes acceptance within
constraints. Minimum problem card: symptom + data source, how improvement is
verified, what counts as accepted, hard constraints, valid-until. "I want X"
without "how will we know it's solved" is not yet a problem.

## Goldilocks selection

Pick problems just above current capability (for LLM runs: solvable in
~10-20% of attempts, so the found solution lifts it to 80-100%). Pre-checks:
measurable acceptance, reversibility + blast-radius control, stepping-stone
potential, real multi-axis trade-off, valid-until on the framing.

## Characterize before comparing

Pipeline: normalize → indicatorize → score → compare → select.

- A **characteristic** is anything measurable; an **indicator** is a
  characteristic explicitly admitted into this cycle's comparison by rule.
  Measurable ≠ indicator.
- Tag every indicator: **constraint** (hard limit) / **target** (optimize,
  1-3 per cycle) / **observation** (watch, never optimize — anti-Goodhart).
- Honest parity: same windows, same budgets, declared normalization,
  explicit "no data" policy, minimum repeats. Selection policy declared
  BEFORE scoring.
- Never fold scales into one magic number. "Better" = Pareto dominance.

In haft: `haft_problem(action="characterize")`, then
`haft_solution(action="compare")` with parity_plan.

## NQD portfolios and stepping stones

Score portfolios on Novelty / Quality / Diversity — never collapsed into a
scalar. Keep 1-2 **stepping stones**: variants weak on Q now that unlock new
search space (new actions, data, tools, interfaces). Remove dominated
variants to the archive; archive is memory, portfolio is this cycle's bets.

Explore↔exploit is governed: declared emitter policy, selection lens,
tie-breakers (novelty/diversity break Q-ties). **Bitter Lesson Preference**:
on tied quality, prefer the scalable/general/learnable approach — and any
"X scales" claim must name the scale variable, window, and slope.

## Evidence decay and trust debt

Every test run, eval, and framing has valid-until. After expiry, trust debt
accrues; over budget you must choose: refresh (re-verify), deprecate, or
waive (explicit, signed, time-boxed). In haft: `haft_refresh(action="scan")`
and the decision evidence loop.

## Minimal improvement protocol (small reversible steps)

1. State the problem so it is reproducible and measurable.
2. Declare acceptance criteria.
3. Generate 3-10 variants (each different in KIND).
4. Filter with cheap tests/evals.
5. Small diff, reversible rollout (canary, rollback plan).
6. Measure effect; update the record.
7. If it recurs — encode into a standard/guide.

## Principal-agent hygiene

The agent (you) operates inside an autonomy envelope: declared rights,
resource caps, logged actions, human confirmation for critical operations.
Contracts are artifacts — acceptance criteria, test packs, checklists — not
vibes. Goodhart/reward-hacking is the standing agent risk; explicit
multi-criteria indicators are the fix. Binding choices (decisions,
commissions) belong to the human principal: recommend, then stop.
