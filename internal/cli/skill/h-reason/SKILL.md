---
name: h-reason
description: |
  Umbrella for FPF-style structured reasoning in a haft project. Carries the full reasoning palette in one place: framing, exploration, comparison, verification, notes, spec lifecycle routing, plus slideument patterns (NQD, Goldilocks, BLP, Scaling-Law Lens). Make sure to use this skill whenever the operator wants structured thinking but the workflow isn't pre-named — phrases like "давай подумаем", "помоги разобраться", "let's think this through", "structured approach", "apply FPF here", "FPF reasoning", "haft this" — or whenever a request is ambiguous between framing, exploration, comparison, and spec lifecycle. Also the manual entry point: /h-reason or /h-fpf alias. For sharp signals dedicated skills still fire (h-frame, h-diagnose, h-explore, h-compare, h-verify, h-status, h-note, h-spec, h-onboard, h-spec-cover); binding choices use manual /h-decide; commissioning uses manual /h-commission.
when_to_use: |
  Operator wants haft-discipline thinking but doesn't pre-select a specific workflow, OR explicitly types /h-reason. Specialized skills auto-fire on sharper signals; this umbrella catches the rest.
argument-hint: "[reasoning topic — what to think about / what to figure out]"
allowed-tools: Bash Read Grep Glob Agent Write Edit mcp__haft__haft_problem mcp__haft__haft_solution mcp__haft__haft_decision mcp__haft__haft_query mcp__haft__haft_note mcp__haft__haft_refresh mcp__haft__haft_spec_section
---

<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:dbf2635a1471bd02cd107527daf87cc663ef99442cd1467079537a5601f0d760 -->

# h-reason — FPF reasoning umbrella

You are running the **haft umbrella reasoning workflow**. This skill replaces the old narrow `/h-fpf` (still aliased — same skill) with a complete reasoning palette: framing, exploration, comparison, verification, notes, plus key patterns from the slideument that don't have dedicated skills (Goldilocks problem selection, NQD discipline, BLP, Scaling-Law Lens).

**This is the manual entry point for FPF-style work** when the operator doesn't pre-pick a specific skill. Specialized skills (h-frame, h-diagnose, h-explore, h-compare, h-verify) still auto-fire on sharp signals and you SHOULD prefer them when the signal is clear — they carry deeper procedures. Use this umbrella when:

- The operator's signal is ambiguous between framing / exploration / comparison
- The operator explicitly types `/h-reason` or `/h-fpf`
- You want the whole palette of FPF wisdom (slideument patterns, FPF glossary) accessible in one place
- The work spans multiple workflow steps (frame → explore → compare in one session)

---

## Compact interface discovery

When you need exact fields for an artifact operation, run the compact CLI
contract instead of reading or pasting long MCP schemas:

```bash
haft interface <capability> --json
```

Useful capabilities: `problem.frame`, `solution.explore`,
`solution.compare`, `decision.decide`, `note.record`, `query.status`,
`query.code_context`, `refresh.scan`.

Use the command as discovery. When the contract exposes a shipped
`haft artifact create <capability> --input-file ... --json` path, prefer that
for large write payloads; keep the matching `mcp__haft__*` tool as the
compatible fallback.

---

## The single most important rule: Description ≠ Work

This is the failure mode this umbrella is designed to NOT fall into.

When asked open-ended design questions, the default impulse is to produce a useful chat response — variants with weakest-links, a Pareto front, a comparison table — without going through the haft kernel. **Stop.** The visual shape of FPF output is not a substitute for invoking the kernel.

If you deliver an analysis without calling `mcp__haft__*` tools, the result is **ephemeral**: gone by tomorrow, no ProblemCard, no SolutionPortfolio, nothing to `/h-verify` in two weeks. The chat answer is wishlist, not work.

**Concrete failure patterns to catch in yourself:**

| About to do this in chat... | Stop and do this instead |
|---|---|
| Present 3+ alternative approaches | Call `haft_problem(action="frame")` then `haft_solution(action="explore")` |
| Compare two approaches with trade-offs | Frame first if needed, then `haft_solution(action="compare")` |
| State what the "real problem" is | Call `haft_problem(action="frame")` and persist it |
| Suggest "remember this" inline | Call `haft_note` |
| Verify a past decision claim | Call `haft_decision(action="evidence")` and `haft_refresh` |

**Friction tradeoff (honest).** Yes, calling kernel tools costs more in-the-moment than answering directly. The friction is the price for **durability and measurability**. Your job is not "best chat answer right now" — it is "leave the project with future-verifiable memory."

---

## Self-check before long responses

Before sending a long response, ask:

1. Is this response presenting **3+ alternatives**, a **comparison**, an **analysis with recommendation**, or **framing what the problem is**?
2. Did I call **any `mcp__haft__*` tool** in this turn?

If (1) = yes and (2) = no — STOP. Pick the right kernel action below and call it before responding.

---

## Maintenance check (FPF B.3.4) — before reasoning

When entering this umbrella, look at the most recent kernel response
for `Refresh reminder: N days since last stale scan`. If N > 30 —
call `mcp__haft__haft_refresh(action="scan")` BEFORE doing reasoning
work.

Reasoning on a stale graph is the same anti-pattern as reasoning on
stale code: variants you generate may rediscover what was already
decided, or contradict an already-decayed claim. Burn the
1-second scan first; reason against fresh state. If the scan finds
nothing new — mention briefly, proceed.

Same discipline for drift detected on files touched in this session:
re-baseline via `haft_decision(action="baseline", ...)` or surface
the drift explicitly. Do not silently proceed past a drift warning.

Surfacing the reminder is the kernel's job; acting on it is the
agent's job. See CLAUDE.md Critical Reminders.

---

## Code work handoff — MethodPack

If reasoning turns into non-trivial code edits, switch from advice to an
operational method run before editing:

- Call `haft_method(action="pull", ...)` with task kind, change intent,
  intended files, and known risk signals.
- Keep the returned `pull_id`; after context compaction recover it through
  `haft_method(action="status")` or `haft_method(action="show", pull_id=...)`.
- Before claiming completion, call `haft_method(action="close", pull_id=...)`
  with changed files, gate results, verification evidence, and explicit
  waivers for any hard gate not evidenced.

Mechanical edits should request low or no ceremony and avoid architecture
gates.

---

## Quick triage — what is the operator actually asking?

Before doing anything, read the operator's signal carefully and classify:

| Signal | Workflow | Section below |
|---|---|---|
| Proposing a refactor/rewrite/redesign/migration without naming the problem | Framing | [Framing](#framing) |
| Concrete failure with unclear root cause (tests fail, X doesn't work) | Diagnosis | [Diagnosis](#diagnosis-failure-investigation) |
| Asking for options / variants / alternatives / "how could we" | Exploration | [Exploration](#exploration-nqd-variants) |
| Comparing 2+ approaches ("A or B", "which is better", "X vs Y") | Comparison | [Comparison](#comparison-parity-pareto) |
| Wants to commit to a specific variant (a binding choice) | Decision (manual) | [Decision is manual](#decision-is-manual) |
| Wants to check if a past decision still holds | Verification | [Verification](#verification) |
| Wants to record a micro-decision with rationale | Note | [Note](#note-micro-decision) |
| Wants situational awareness ("where are we", "what's stale") | Status | Call `haft_query(action="status")` directly |
| Wants spec lifecycle/update ("spec status", "запиши в спеки", approve/rebaseline/reopen spec) | Spec lifecycle | [Spec lifecycle](#spec-lifecycle) |
| Repository has no `.haft/` yet | Onboarding | Delegate to `/h-onboard` |
| General FPF question ("what is FPF", "look up pattern E.9") | FPF spec lookup | Call `haft_query(action="fpf", query=...)` |

If the signal is **ambiguous** (spans multiple workflows) — start with framing. Without a framed problem, exploration / comparison float.

---

## Spec lifecycle

Use when: the operator asks to inspect, discuss, update, approve, rebaseline, or
reopen project specs.

Spec lifecycle is not a normal "write docs" task. Keep the strict distinction:
SpecSection = description, `.haft/specs/*.md` = carrier, project behavior =
object, kernel lifecycle projection = method authority.

Procedure:

1. Call `mcp__haft__haft_spec_section(action="lifecycle")`.
2. If the action is `draft` or `clarify`, read `workflow_intent` and follow
   `/h-spec`: draft or fix the carrier, run `haft spec check`, then call
   lifecycle again.
3. If the operator says "запиши" / "write this into specs", treat that as
   permission to edit the carrier only. It is not approval or rebaseline.
4. If the action is `approve` or `triage`, surface the human gate. In triage,
   `rebaseline` and `reopen` are explicit mutation choices from
   `allowed_commands`, not autonomous lifecycle states. Only call a mutation
   after explicit operator instruction with a reason where required.
5. Do not create feature-local spec markdown as an authority. Decisions,
   WorkCommissions, RuntimeRuns, code refs, and SpecSections link through the
   graph.

For a full procedure, delegate to `/h-spec`.

---

## Framing

Use when: a solution is proposed before the problem is stated, or scope/acceptance is unclear.

**Procedure (compressed; full version in /h-frame):**

1. **Stabilize the signal.** What's actually broken or wanted? In one sentence, what condition would make the operator say "solved"?
2. **Type the problem** — pick one:
   - `diagnosis` — something broke, root cause unknown
   - `optimization` — known target, want better numbers
   - `search` — looking for an unknown thing (e.g., a library, a pattern)
   - `synthesis` — building something new
3. **Umbrella-word repair** — if the signal uses overloaded terms ("service", "quality", "scalable", "готово"), unpack to specifics. Refuse to record a ProblemCard built on umbrella words.
4. **Acceptance criterion** — one observable condition that signals "minimum viable success". Without this, the problem can't be verified later.
5. **Record:**
   ```
   mcp__haft__haft_problem(
     action="frame",
     problem_type="<diagnosis|optimization|search|synthesis>",
     title="<short title>",
     signal="<what's happening / what's needed>",
     acceptance="<observable condition>",
     constraints=["<hard limit>"],
     blast_radius="<what gets affected>",
     reversibility="<low|medium|high>",
     mode="<tactical|standard|deep>"
   )
   ```
6. Surface the ProblemCard ID to the operator. They can edit, supersede, or move on.

**Common mistakes:**
- Framing during code-grinding sessions (h-reason isn't for every task — code work doesn't need a ProblemCard)
- Letting acceptance stay vague ("works better") instead of observable
- Skipping problem-type → comparisons later anchor wrong

For deeper framing (B.4.1 + B.5.2 with parallel-rival generation on diagnosis problems), use `/h-frame` or `/h-diagnose` directly.

---

## Diagnosis (failure investigation)

Use when: concrete failure, unclear cause. Stronger than framing — runs parallel rival-hypothesis testing.

**Compressed procedure (full version in /h-diagnose):**

1. **Stabilize** — what's the symptom in one sentence? When did it start? Reproducible?
2. **Frame** as `problem_type="diagnosis"` via `haft_problem(action="frame")`.
3. **Generate ≥3 rival hypotheses** (FPF B.5.2 four-step abductive). Optionally spawn parallel Agent subagents — each takes one hypothesis and tests it independently against the codebase. Sequential is fine for lightweight investigations.
4. **Filter for falsifiability** — drop hypotheses with no testable prediction.
5. **Rank by evidence weight.** Keep losing rivals visible (CC-B.5.2-2) — do not collapse to a single answer prematurely.
6. **Record findings** as `haft_solution(action="explore", variants=[...])` where each variant = one hypothesis + its evidence + its weakest_link.
7. **Recommend next action.** Usually: test the leading hypothesis, then `/h-decide` if a fix path is clear.

For full parallel-subagent dance use `/h-diagnose` directly.

---

## Exploration (NQD variants)

Use when: problem is framed, operator wants 3-5 distinct candidate solutions.

**Compressed procedure (full version in /h-explore):**

1. **Confirm there is a framed problem.** If not, frame first (above). Without a frame, variants float.
2. **Generate 3-5 distinct variants — each must differ in KIND, not just degree** (FPF EXP-08). Parallel directions to consider:
   - Data-flow restructure (avoid the operation)
   - Algorithmic alternative (same op, different algo)
   - Infrastructure swap (different runtime / service)
   - Caching / batching / queuing (smooth load)
   - Architectural extraction (move to different layer)
   - Workflow restructure (change when/how triggered)
   - Stepping-stone (suboptimal now, opens future search space)
3. **Each variant carries:**
   - `title`
   - `description` (2-3 sentences)
   - `novelty_marker` — what makes this distinct from typical AI suggestions
   - `weakest_link` — what bounds quality if pursued (NOT title repeated)
   - `stepping_stone: true|false` (+ basis if true)
   - `risks` + `strengths`
4. **Record:**
   ```
   mcp__haft__haft_solution(
     action="explore",
     problem_ref="<prob-...>",
     variants=[...],
     no_stepping_stone_rationale="<if all stepping_stone=false>"
   )
   ```
5. Kernel returns SolutionPortfolio ID + may emit soft warnings about disguised duplicates, missing parity_rules, or weakest_links that just repeat titles. Read and self-correct.
6. Recommend next step (usually `/h-compare` if 3+ variants).

For full parallel-subagent generation use `/h-explore` directly — it spawns one Agent per direction in parallel.

---

## Comparison (parity + Pareto)

Use when: SolutionPortfolio exists with 2+ variants, operator wants to evaluate.

**Compressed procedure (full version in /h-compare):**

1. **Characterize first.** Declare comparison dimensions BEFORE scoring (kernel rejects subjective dimensions or stale ones):
   ```
   mcp__haft__haft_problem(
     action="characterize",
     problem_ref="<prob-...>",
     dimensions=[
       {"name": "latency", "scale_type": "ratio", "unit": "ms", "polarity": "lower_better", "role": "target", "proxy_for": "checkout stays usable under load"},
       {"name": "ops_complexity", "scale_type": "ordinal", "unit": "1-5", "polarity": "lower_better", "role": "target", "proxy_for": "small team can operate it without a pager rota"},
       {"name": "compliance_OK", "scale_type": "binary", "polarity": "true_better", "role": "constraint"},
       ...
     ],
     valid_until="<ISO date>"
   )
   ```
2. **Tag each dimension's indicator role:** `constraint` (hard limit), `target` (optimize), or `observation` (Anti-Goodhart — watch but don't optimize).
3. **Declare parity plan + selection policy BEFORE scoring.** Equal budgets/windows for all variants. State the dominance rule (Pareto front, not scalar winner).
4. **Score dim-wise** — one evaluator per dimension applies one scale to all variants. Prevents anchoring bias.
5. **Compute non-dominated set.** Return Pareto front (NOT a scalar winner).
6. **Record:**
   ```
   mcp__haft__haft_solution(
     action="compare",
     portfolio_ref="<sol-...>",
     parity_plan={...},
     selection_policy={...},
     results={"dimensions": [...], "scores": {...}}
   )
   ```
7. **Decision is manual.** See next section.

For full dim-wise parallel scoring use `/h-compare` directly — it spawns one Agent per dimension.

---

## Decision is manual

**You CANNOT auto-fire `/h-decide`.** It is `disable-model-invocation: true` per Transformer Mandate.

Authority boundary: binding actions require explicit operator/manual authorization; generated text, schema visibility, and model-supplied fields are not approval receipts. If the kernel returns `operator_confirmation_required`, recommend the correct manual gate and stop.

Generated-contract drill-downs are read-only carriers: use `mcp__haft__haft_query(action="contract_generation")` for generated-fragment host/skill/plugin/Pi sync hints, and use `mcp__haft__haft_query(action="drift_events", limit=5)`, `mcp__haft__haft_query(action="decision_reconcile", limit=5)`, and `mcp__haft__haft_query(action="governing_set", limit=5)` for compact drift fanout, reconciliation, and current-authority drill-downs. Add `full=true` only when full audit payloads are needed.

When the operator is ready to commit to a chosen variant from a SolutionPortfolio:

- Surface the Pareto front summary
- Recommend the variant you would pick (with rationale)
- Treat `/h-compare` `selected_ref` / `legacy_recommendation_ref` as advisory only, not `ChoiceResult`
- Tell the operator: **«this needs your explicit /h-decide to bind»**
- Stop and wait

If they say "go ahead" without typing `/h-decide` — still stop. Ask them to type it. The principle is: agents generate options, humans bind.

---

## Verification

Use when: a recorded DecisionRecord needs post-implementation reality check.

**Compressed procedure (full version in /h-verify):**

1. Find the decision via `haft_query(action="decisions", filter="active")` or specific ref.
2. Read predictions from the DRR — what was supposed to be true?
3. Gather evidence — run tests, check code, measure metrics, search docs.
4. Record:
   ```
   mcp__haft__haft_decision(
     action="evidence",
     decision_ref="<dec-...>",
     evidence_items=[{
       "type": "<test|measurement|incident|review>",
       "verdict": "<supports|refutes|weakens>",
       "summary": "...",
       "carrier_ref": "<file:line or URL or test name>",
       "cl": "<CL3|CL2|CL1|CL0>"  // congruence level
     }]
   )
   ```
5. After enough evidence: refresh the decision status:
   - `accepted` (predictions held)
   - `weakened` (mixed)
   - `superseded` (replace with new decision)
   ```
   mcp__haft__haft_refresh(action="<waive|supersede|deprecate>", artifact_ref="<dec-...>")
   ```
6. Close with a loop verdict (FPF E.23): `stop` (further movement dominated —
   "good enough" is a legitimate terminal state), `continue`, `switch-method`
   (supersede), `open-new-frame` (reopen), or `hold` (waive with revisit date).
   Narrowing — shrinking scope or suppressing signals to look better — is NOT
   a verdict; scope reduction requires an explicit dominance justification.

For full verify loop (drift detection across code/affected_files, FSRS-style intervals) use `/h-verify` directly.

---

## Note (micro-decision)

Use when: small choice with rationale that doesn't justify full DRR ceremony.

```
mcp__haft__haft_note(
  text="<one or two sentences: what + why>",
  rationale="<the why is required — kernel rejects content-free notes>",
  links={
    "based_on": ["<prob-... or dec-...>"],
    "tags": [...]
  }
)
```

Common moments to note:
- After resolving a non-trivial bug — capture cause + fix lesson
- After ruling out a variant during exploration — capture why
- After a user correction — capture the right approach
- A surprising discovery in the codebase
- A constraint discovered mid-work

---

## Slideument patterns (Levenchuk's 2026 seminar wisdom)

These don't have dedicated skills but you should apply them when relevant:

**Three factories.** Engineering has three production lines:
1. **Problem factory** — generates new framed problems (problematization)
2. **Solution factory** — generates solutions for framed problems
3. **Factory of factories** — improves both production lines

In haft: ProblemCard archive = problem factory output; SolutionPortfolio = solution factory output. Bring this lens when the operator confuses "what problem to solve" with "how to solve it" — they're separate factories.

**Goldilocks problem selection.** Pick problems just above current capability (LLM heuristic: solvable in 10-20% of runs, not less, not more). Pre-pick checks:
- Measurable acceptance (not vague)
- Reversibility (can we undo?)
- Stepping-stone potential (does solving open new search space?)
- Multi-axis trade-off (otherwise not really a Pareto problem)
- Valid_until on framing (postings rot)

Apply: when triaging which problem to work on next, surface these criteria explicitly.

**NQD discipline (Novelty / Quality / Diversity).**
- **Novelty** — how different from known solutions
- **Quality** — measurable use-value in acceptance spec
- **Diversity** — coverage of independent niches by the portfolio

Never collapse NQD into a scalar. Hold a Pareto front. Keep 1-2 stepping-stones (high N or D, lower Q now) for future search-space expansion. Apply: in exploration, score each variant on NQD axes, not just utility.

**BLP — Bitter Lesson Preference.** When choosing between hand-tuned specialist and scalable universal agent, prefer the latter as tie-breaker. Apply: in comparisons that have tied Q-scores, BLP can break ties toward general-purpose / learnable approaches.

**Scaling-Law Lens (SLL).** When claiming "this approach scales", declare:
- Scale variables (compute / data / capacity / iteration budget)
- ScaleWindow (range over which claim holds)
- ElasticityClass (rising / knee / flat / declining)

Don't say "X scales" without naming what S is, what window, what slope. Apply: in AI/agent-related comparisons especially.

**Stepping stones** (Lehman & Stanley 2015). Sub-optimal-by-Q solutions that open new search areas. Keep 1-2 in portfolio explicitly. Apply: in exploration, mark them; in comparison, don't drop them based on current Q alone.

**Anti-Goodhart via Indicator Roles.** Tag each dimension:
- `constraint` — hard limit, must satisfy
- `target` — what you optimize
- `observation` — watch but DON'T optimize

If you optimize an `observation`, you're Goodharting. Apply: at characterization time, before any scoring.

---

## FPF glossary (key concepts)

**R_eff (Effective Reliability):** computed trust score in [0,1]. `R_eff = min(evidence_scores)` with CL penalties. Never average — weakest-link principle.

**CL (Congruence Level):** how well evidence transfers across contexts.
- CL3: same context (internal test) — no penalty
- CL2: similar context (related project) — 0.1 penalty
- CL1: different context (external docs) — 0.4 penalty
- CL0: opposed context — 0.9 penalty

**Evidence Decay:** evidence has `valid_until`. Expired evidence scores 0.1 (weak, not absent). Graduated epistemic debt sorted by severity.

**DRR (Decision Record):** FPF E.9 four-component structure: Problem Frame, Decision/Contract, Rationale, Consequences. Created ONLY via manual `/h-decide`.

**Indicator Roles:** see Anti-Goodhart above.

**Transformer Mandate:** systems cannot transform themselves. Humans bind; agents document. Autonomous binding = protocol violation.

**Strict Distinction (A.7):** Object ≠ Description ≠ Carrier. Plan ≠ Reality. Design-time ≠ Run-time. Promise/commitment ≠ actual delivery.

**Three systems always present:**
1. **Target system** — what delivers value to users
2. **Enabling system** — what creates / governs the target (team + tooling + process)
3. **Method** — the way of creating it

Confusing these is the most common FPF anti-pattern.

---

## When to delegate to a specialized skill

This umbrella covers compressed versions of the workflows. For deeper procedures (parallel subagent dancing, full parity discipline, complete drift detection), recommend the specialized skill:

| Heavy version | When to delegate |
|---|---|
| `/h-frame` | Standard/deep mode with full B.4.1 stabilize-route + problem-typing |
| `/h-diagnose` | Need parallel rival hypothesis testing across the codebase |
| `/h-explore` | Need parallel Agent-per-direction NQD variant generation |
| `/h-compare` | Need parallel dim-wise scoring (one evaluator per dimension) |
| `/h-verify` | Full drift detection + Evidence Decay scan + FSRS-style scheduling |
| `/h-decide` (manual) | Always — binding action |
| `/h-commission` (manual) | Always — execution authority grant |
| `/h-spec` | Spec lifecycle, spec carrier updates, approve/rebaseline/reopen gates |
| `/h-onboard` | No `.haft/` directory yet, or legacy bootstrap wording |
| `/h-status` | Cheap dashboard, can call directly |
| `/h-spec-cover` | Module-level coverage analysis |

For routing on natural language signal, the specialized skill descriptions ALSO auto-trigger — so if the signal is sharp, you may never enter this umbrella. That's fine: both routes converge on the same kernel.

---

## What NOT to do in this umbrella

- DO NOT auto-fire `/h-decide` or `/h-commission`. Transformer Mandate. Even if the user says "decide for me" — stop and ask for explicit slash command.
- DO NOT skip `haft_problem(action="frame")` before exploration. Without framing, variants are wishlist.
- DO NOT collapse NQD into a scalar. Always present Pareto front.
- DO NOT use FPF jargon in your responses to the operator unless they used it first. Translate `R_eff` to "trust score", `CL2` to "different project context", `Evidence Decay` to "stale evidence".
- DO NOT replicate full procedures from specialized skills here. This umbrella's job is compressed coverage. For thoroughness — delegate.
- DO NOT skip the kernel — even compressed procedures must persist via `mcp__haft__*` calls.

---

## FPF spec lookup

For deep references on any concept:

```
mcp__haft__haft_query(action="fpf", query="<topic or pattern-id>")
```

Add `full=true` for complete pattern text, `explain=true` for guided explanation.

Common pattern IDs:
- `A.1` — Holonic Foundation (target vs enabling system)
- `A.7` — Strict Distinction (Clarity Lattice)
- `A.17` / `A.18` — Characteristic Norm (dimension declaration)
- `B.3` — Trust & Assurance (R_eff, CL)
- `B.3.4` — Evidence Decay
- `B.4.1` — Observe → Notice → Stabilize → Route
- `B.5.2` — Abductive Loop (four-step rival generation)
- `B.5.2.1` — NQD-style Creative Abduction
- `C.18` — NQD-CAL (Open-Ended Search Calculus)
- `C.18.1` — Scaling-Law Lens
- `C.19` — Explore/Exploit Governor + BLP
- `E.9` — Design-Rationale Record method (DRR)
- `E.16` — Autonomy Budget
- `F.18` — Local-First Unification Naming
