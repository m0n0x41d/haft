---
name: h-diagnose
description: |
  Diagnoses a failure with parallel rival-hypothesis testing — multiple read-only subagents test distinct explanations in parallel, then rank by evidence weight while keeping losing rivals visible so the root cause is found honestly, not just plausibly. Make sure to use this skill whenever the user reports something broken with an unclear cause — "tests fail", "test is failing", "X doesn't work", "Y crashes", "why is Z happening", "investigate this bug", "what's causing this", "the bug is unclear", "something's wrong with X", "X used to work and now doesn't", "this is flaky" — or any failure report where the next diagnostic step isn't already obvious to the user. NOT for feature requests (use h-frame). NOT for performance work with a known bottleneck (use h-frame). NOT for verifying a hypothesis already recorded in a DecisionRecord (use h-verify).
when_to_use: |
  Concrete failure with unclear root cause. Skip if next diagnostic step is obvious and the user just wants you to run it. For framing a new design problem use h-frame.
argument-hint: "[symptom or failure description]"
allowed-tools: Bash Read Grep Glob Agent mcp__haft__haft_problem mcp__haft__haft_solution mcp__haft__haft_query
---

# h-diagnose — Diagnosis with parallel hypothesis testing

You are running the FPF diagnosis workflow: B.4.1 stabilize → B.5.2 abductive four-step → ranked verdict with rivals visible. Parallel subagents test distinct hypotheses in isolated contexts to prevent anchoring bias.

## PatternUse Gateway

Before substantive reasoning or work-shaping, call
`mcp__haft__haft_query(action="pattern_use", mode="compact", query="<operator concern>")`.
Skip only for mechanical/status/exact-lookup work where no FPF pattern choice is
material. If `should_use_pattern=true` and this skill needs detail, request
`mode="full"` and carry the returned output shape, boundary, evidence need, and
blocked stronger use. Do not inline the FPF catalog in this skill. PatternUse
is not approval, not evidence, not a DecisionRecord, not MethodPack, and not a
gate.

## Step 1 — Stabilize the signal (B.4.1)

Read the symptom carefully. Do not jump to causes. Compress to one sentence:
- What was observed (not assumed)
- What pattern the failures share
- What stays out of scope for now

Bad: "the cache is broken"
Good: "cache invalidation tests in schema-migration suite fail intermittently since 2026-05-22"

If the signal contains umbrella terms ("slow", "broken", "weird") run inline precision restoration per FPF A.6.P/Q/A before proceeding.

## Step 2 — Frame as diagnosis problem

Call:
```
mcp__haft__haft_problem(
  action="frame",
  problem_type="diagnosis",
  title="<short diagnosis title>",
  signal="<stabilized observation>",
  acceptance="<what would constitute root-cause identified + verified>",
  constraints=["<read-only investigation only — no fixes>"]
)
```

Capture the returned `prob-...` ID for downstream subagent calls.

## Step 2.5 — Ground the failing symbol in the graph (when the signal names code)

If the signal names a concrete failing symbol or file, run this ONCE in THIS
orchestrator. The Step 4 hypothesis subagents are read-only investigators and
**cannot call `haft_query`** — only you can pull the graph and pass it down, so do
it here:

```
mcp__haft__haft_query(action="code_context", file="<file>", symbol="<failing symbol>")
```

Extract a compact **graph-context block** from the result:
- the **invariants that must hold here** (each `_(from dec-…)_`) — a *violated*
  invariant is a concrete, named root-cause hypothesis, not a guess;
- the **trust-decay tags** on the governing decisions (`[refresh-due]`,
  `· N/M predictions unverified`). A STALE governing decision is itself a prime
  suspect: if the decision governing this code was never verified, its assumptions
  may no longer hold. Agents systematically MISS this hypothesis class because they
  reason about the code, not about the decay of the reasoning ABOUT the code.

If the signal does NOT name code (environmental, data, flaky infra), SKIP this step
— the graph adds nothing there; do not force it. Keep the block; you inject it into
every Step 4 subagent.

## Step 3 — Generate ≥3 hypotheses (B.5.2 abductive)

Produce 3-5 candidate root causes. They MUST differ in kind, not degree (FPF EXP-08). If you have three cache hypotheses, force at least one that ISN'T cache-related (data flow, race condition, environmental, etc.).

When Step 2.5 surfaced governed code, make at least one hypothesis structural: a
**violated invariant** or a **stale governing decision** whose assumptions may no
longer hold. This is the hypothesis class abduction-on-code-alone misses.

For each hypothesis state:
- One sentence claim
- Why plausible (one sentence)
- A discriminating probe — what evidence would falsify or confirm

## Step 4 — Test in PARALLEL (one Agent call per hypothesis)

For N hypotheses spawn N Agent subagents IN THE SAME MESSAGE (this is the structural parallelism that prevents anchoring). Each subagent runs in isolated context.

For each hypothesis, spawn:
```
Agent(
  description="Test diagnosis hypothesis #<n>",
  prompt="
    You are testing ONE hypothesis for the diagnosis of:
    <stabilized signal>

    Your hypothesis: <hypothesis claim>
    Why-plausible: <one-line>
    Discriminating probe: <one-line probe>

    Graph context (the orchestrator pulled this from haft's fused graph — you
    CANNOT query it yourself, so use what is given):
    <the Step 2.5 graph-context block: invariants that must hold here + governing
    decisions with their trust-decay; or "none — this code is ungoverned">

    Allowed tools: Read, Grep, Glob, Bash (test runners only — no edits).
    NOT allowed: Write, Edit, NotebookEdit. You are READ-ONLY.

    Return EXACTLY this structure:
    evidence_for:
    - <observation supporting hypothesis>
    evidence_against:
    - <observation contradicting hypothesis>
    inconclusive:
    - <observation that neither supports nor refutes>
    verdict: supported | refuted | inconclusive
    confidence: low | medium | high
    notes: <one-paragraph summary>
  "
)
```

All N Agent calls go in the SAME assistant message — Claude Code executes them in parallel. Wait for all results before Step 5.

## Step 5 — Rank by evidence weight (keep rivals visible per CC-B.5.2-2)

After collecting subagent results, rank hypotheses by:
1. Verdict (supported > inconclusive > refuted)
2. Confidence (high > medium > low)
3. Evidence_for count minus evidence_against count

Identify the **prime hypothesis** (top-ranked) but DO NOT discard the others. Per FPF CC-B.5.2-2 rivals stay visible in the artifact graph for replay and fallback.

## Step 6 — Record as SolutionPortfolio variants

Call:
```
mcp__haft__haft_solution(
  action="explore",
  problem_ref="<prob-... from Step 2>",
  variants=[
    {
      "title": "<hypothesis 1 short name>",
      "description": "<claim + evidence summary>",
      "weakest_link": "<what would invalidate this even though current evidence supports>",
      "novelty_marker": "<what makes this hypothesis distinct from the others>",
      "stepping_stone": false
    },
    // ... one entry per hypothesis, with the prime one marked as such in description
  ]
)
```

The kernel returns a SolutionPortfolio ID. Each hypothesis becomes a variant with its evidence trail attached. The operator can verify, supersede, or commission action on any of them.

## Step 7 — Present to operator

Surface to operator:
- Prime hypothesis with evidence summary
- Rivals visible (per CC-B.5.2-2) with their evidence
- Specific next-action recommendation:
  - If prime is `supported high-confidence` → recommend `/h-decide` to commit to the fix path
  - If prime is `supported medium` → recommend more targeted probes before deciding
  - If all hypotheses `inconclusive` → recommend widening exploration (different hypothesis space) via `/h-explore`
  - If all `refuted` → recommend re-framing the problem via `/h-frame` (signal may be misobserved)

Operator decides next step. Do NOT auto-fix; diagnosis stops at root-cause identification.

**Re-grounding discipline (FPF A.7).** Every hypothesis label (`H1`,
`H2`, …) and artifact ID (`prob-...`, `sol-...`) in your prime / rivals
/ recommendation summary MUST be paired with its one-line claim. Bare
`H3 dominates H1 by evidence weight` is opaque once context fades; `H3
(race condition in retry loop) dominates H1 (cache stale) by evidence
weight` restores what is actually being compared. Keep IDs for kernel
follow-up but never let them stand alone. See CLAUDE.md Critical
Reminders for the project-wide rule.

## What NOT to do

- Do not test hypotheses sequentially in one context — anchoring bias degrades evidence quality.
- Do not collapse to one hypothesis prematurely; keep rivals visible per CC-B.5.2-2.
- Do not let subagents WRITE or EDIT — they are read-only investigators.
- Do not skip the framing step — without a problem record the diagnosis floats.
- Do not run the fix yourself; this skill stops at evidence + recommendation.
- Do not generate hypotheses that differ only in degree (all cache-related with slight variations).
- Do not spawn fewer than 3 hypothesis subagents unless the search space is genuinely binary.

## FPF spec references

- B.4.1 — Observe → Notice → Stabilize → Route (pre-articulation)
- B.5.2 — Abductive four-step micro-cycle
- B.5.2.1 — Creative Abduction with NQD (forced diversity)
- CC-B.5.2-1, CC-B.5.2-2 — Conformance: rival candidates visible
- EXP-01 (abductive loop), EXP-04 (WLNK per variant), EXP-08 (NQD novelty marker)

Look up via `mcp__haft__haft_query(action="fpf", query="B.5.2")`.
