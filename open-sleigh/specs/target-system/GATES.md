---
title: "9. Gates and Observations"
description: Structural gates, semantic gates (LLM-judge + calibration), HumanGate protocol, observation indicators (anti-Goodhart). Canonical gate catalogue.
reading_order: 9
---

# Open-Sleigh: Gates and Observations

> **Why a dedicated document.** The gate algebra appears in four places:
> the L2 contract (`../enabling-system/FUNCTIONAL_ARCHITECTURE.md` Layer 2),
> the phase binding (`PHASE_ONTOLOGY.md` "Gates per phase"), the illegal-
> states catalogue (`ILLEGAL_STATES.md` GK category), and the phase-exit
> rules. This document is the **canonical gate catalogue** — every gate's
> name, kind, firing site, check semantics, implementation, and
> calibration discipline lives here. The four references above now point
> in; this is the load-bearing source of truth.
>
> **FPF note.** Gates are **design-time claims** (closed atom set, kind-
> aware combine, constructor consistency). Passing tests against the
> golden set is **run-time evidence** that a semantic gate's implementation
> matches its design intent. A semantic gate without a calibrated golden
> set is indistinguishable from a non-deterministic filter.

---

## 1. Structural gates (pure, fast, deterministic)

These are `f(artifact) → :ok | {:error, reason}` where `artifact` is the
Haft JSON blob returned from the relevant `haft_*` tool call. They catch
"agent forgot a field" errors, not "agent wrote 'the system'" errors.

| Gate | Fires at | Checks |
|---|---|---|
| `commission_runnable` | Preflight entry | WorkCommission exists, is in `ready`/`queued` state, has an exclusive lease, and has not expired. |
| `decision_fresh` | Preflight entry | Linked DecisionRecord exists, is active, hash/revision matches the commission snapshot, and is not refresh_due/stale/superseded/deprecated. |
| `lockset_available` | Preflight entry | No other leased/running WorkCommission in the same plan has an overlapping lockset. |
| `autonomy_envelope_allows` | Preflight entry | If auto-started/batch/YOLO, the approved AutonomyEnvelope allows this repo/path/action/risk class and has remaining budget. |
| `problem_card_ref_present` | Frame **entry** | `WorkCommission.problem_card_ref` is non-nil, resolves to a live Haft artifact, and its `authoring_source ≠ :open_sleigh_self`. Hard-fails Frame with `:no_upstream_frame`; the commission goes back to Haft as blocked/review-needed. |
| `described_entity_field_present` | Frame exit | **Upstream** ProblemCard (the one referenced by `WorkCommission.problem_card_ref`) has non-empty `describedEntity` and `groundingHolon`. Checks upstream human-authored content; Open-Sleigh never creates these fields. |
| `valid_until_field_present` | every phase exit | Artifact has `valid_until` in the future |
| `evidence_ref_not_self` | Measure exit | Evidence carrier `ref`/`hash` field is non-empty AND `ref ≠ artifact.self_id` at `PhaseOutcome` construction (the check lives where `self_id` is known, not on `Evidence.new` in isolation — see `ILLEGAL_STATES.md` PR5). Whether the carrier is actually *external to the authoring role* is a semantic check, moved to `no_self_evidence_semantic` (§2). |
| `design_runtime_split_ok` | Execute entry | No `MethodDescription` nodes embedded in `Work` traces (structural check on Haft graph shape) |
| `language_state_complete` | Commission exit (MVP-2) | Artifact status field == `Complete`, not `PreArticulation` / `InProgress` |

---

## 2. Semantic gates (LLM-judge or Human)

These need **judgement**, not field presence. They run via either (a) a
dedicated "reviewer" agent call with a scoped prompt and a structured
verdict output, or (b) a `HumanGate`. MVP-1 runs the cheap ones as
LLM-judge; the expensive ones as HumanGate.

| Gate | Fires at | Check | Implementation in MVP-1 |
|---|---|---|---|
| `object_of_talk_is_specific` | Frame exit | Is `describedEntity` specific (file path, module, subsystem) or vacuous ("the system", "the code")? | LLM-judge |
| `context_material_change_review` | Preflight exit | Did repo/context changes since commission queue materially affect the selected DecisionRecord or commission scope? Agent reports; Haft validates hard facts. Uncertainty maps to human review. | LLM-judge + deterministic validation |
| `lade_quadrants_split_ok` | any obligation-language artifact | Sentences containing MUST / guarantees / accepted / evidence are decomposed into **Law** (definition) / **Admissibility** (gate) / **Deontics** (duty) / **Work-effect ∣ Evidence** (carrier). Per `../../.context/semiotics_slideument.md` §A.6.B Slide 33 literal. Renamed from `lade_split_ok` in v0.2 — the quad is **not** {Law, Admissibility, Deontics, Evidence}; the 4th axis is explicitly "Work-effect / Evidence", and conflating those two is itself a reportable error. | LLM-judge |
| `no_self_evidence_semantic` | Measure exit | **Two separate checks:** (a) the cited evidence is produced by a role **external to the authoring role** (FPF-Spec A.10 CC-A10.6: no self-evidence); (b) the **evidence carrier** (PR sha, CI run id, test log) is distinguished from the **work-effect** (what the merged code actually does in runtime). A carrier is not a work-effect; conflating them is the trap this gate exists to catch. | LLM-judge |
| `contract_unpacked_ok` | any artifact containing promise-language ("I implemented X", "delivered Y") | Promise content / speech act / commitment / work-effect-evidence decomposed per `../../.context/FPF-Spec.md` A.6.C. Deferred to MVP-2; flagged now so agents publishing "done" claims don't conflate promise content with delivery evidence. | **not in MVP-1** |
| `cg_frame_wellformed` | Parity-run exit (MVP-2) | Characteristics have scales/procedures; budgets equal; selection rule pre-declared; `valid_until` set; seeds recorded | LLM-judge |
| `commission_approved` | Commission entry (MVP-2) and Execute→Measure when PR→main (MVP-1) | Human principal confirmed the transition | **HumanGate** |

**Gate purity caveat.** Gates are pure as `f(inputs)` where `inputs` include
not just the artifact but the relevant Haft graph slice (e.g.
`no_self_evidence_semantic` reads the `relations` table via
`haft_query(related)`). They are pure in the functional sense (same inputs
→ same outputs, no observable side effects) but are **not** pure in the
loose "needs nothing but the artifact" sense. This matters when writing
tests: gate tests need Haft fixtures, not just JSON fixtures.

---

## 3. LLM-judge calibration (CHR-04 assurance tuple)

Semantic gates implemented via LLM-judge are probabilistic instruments, not
oracles. Per CHR-04, every such instrument carries an assurance tuple
(F/G/R/CL). For Open-Sleigh:

- **Golden set per gate.** Each LLM-judge gate has a reviewed corpus of
  **≥20 hand-labelled artifacts** (positive + negative cases, balanced).
  Lives in `golden/<gate_name>/`. Labels include ground-truth verdict +
  rationale.
- **Formality (F) and Reliability (R) reported per gate.** On every
  golden-set run, we compute and publish:
  - `false_positive_rate` — judge rejected, ground truth says pass
  - `false_negative_rate` — judge accepted, ground truth says fail
  - `congruence_level` (CL) — how close judge-model version/prompt matches
    the version used in production runs
- **Observation surface (§5).** `judge_false_pos_rate_<gate>` and
  `judge_false_neg_rate_<gate>` are emitted as observation indicators —
  NOT gates. A rising false-neg rate on `lade_quadrants_split_ok` tells us
  the judge is drifting before `gate_bypass_rate` does.
- **Re-run trigger.** Golden set is re-executed on any of: judge-model
  change, prompt template change, rubric change, quarterly cadence.
  A golden-set regression blocks the canary.
- **Labelling burden.** Initial 20-item sets per gate are part of the MVP-1
  deliverable, not deferred. A semantic gate without a golden set is
  indistinguishable from a non-deterministic filter — we do not ship those.

Without this calibration, `gate_bypass_rate` in §5 cannot distinguish
"agent gamed the gate" from "judge drifted" — the two failure modes
require different responses. Calibration separates them.

**Statistical caveat — rates below n=50 are trend indicators, not
thresholds.** At n=20 with observed FPR=15%, the Wilson 95% CI is
approximately [5%, 36%]. You cannot distinguish 15% from 30% at that
sample size. For MVP-1 bootstrap (n=20 per gate), act on **direction of
change across repeated runs**, not on absolute value crossing a line.
Move to absolute-threshold alerting once n≥50 per gate, which for a
production rate of ~10-15% gives CI half-width ≈ ±10 pp — still loose but
no longer dominated by sampling noise.

**Golden-set versioning rule — when to re-label.**

- **Rubric change** (the definition of what constitutes pass/fail on a
  gate changes) → golden set MUST be re-labelled by a human. Old labels
  are not portable across rubric revisions. Label versions are frozen
  with a rubric-version tag.
- **Prompt-only change** (the LLM-judge prompt is tweaked, but the rubric
  the human labeller applies is unchanged) → golden set labels stay
  valid. Re-run the judge against the existing labels; the delta in
  false-pos/false-neg is exactly the signal you want from the prompt
  change.
- **Judge-model change** (different model or version) → treat as
  prompt-only for label validity; re-run against existing labels and
  compare rates. Model change is CL-moving, not ground-truth-moving.

Without this rule, the first prompt tweak produces silent label drift —
labels that were true under one rubric are quietly reinterpreted under
another.

---

## 4. HumanGate behaviour

`HumanGate` is triggered, not computed. It:
1. Blocks the phase transition.
2. Posts a structured approval request to the configured local/external
   surface (Desktop/CLI, Linear/Jira/GitHub comment, or PR review request).
3. Listens for a confirming signal from an authorized approver.
4. On approval: releases the transition and records
   `HumanGateApproval { approver, at, reason?, config_hash }` as evidence
   on the Haft artifact.
5. On explicit `/reject`: returns the ticket to the previous phase.
6. On timeout (24h default): escalates with a second comment; after 72h,
   cancels the worker and surfaces the ticket on a "stalled-gate" dashboard.

**Triggers in MVP-1:**
- Execute → Measure, when PR target branch matches `external_publication`
  regex in `sleigh.md` (default: `^(main|master|release/.*)$`).
- Any external projection transition to a terminal external state (Done, Won't
  Do, Closed) when projection policy requires confirmation.
- Any proposed action outside the approved AutonomyEnvelope.

This is the concrete enforcement of the Transformer Mandate in MVP-1: the
agent can **verify framing**, can implement, can request approval — but it
cannot author framing, cannot unilaterally publish to `main`, and cannot
close a ticket. Authoring framing is the human's role upstream in Haft
+ `/h-reason`; Open-Sleigh verifies that work, it does not redo it.

---

## 5. Observation indicators (anti-Goodhart)

Per FPF CHR-01 + `../../.context/development_for_the_developed.md` Slide 16:
characteristics that are **monitored for risk, not optimized**. These are
explicitly NOT gates — they never block a transition. They are surfaced
on the dashboard (MVP-2) and emitted as metrics from day 1.

| Indicator | What it measures | Denominator | Alert threshold (indicative) |
|---|---|---|---|
| `gate_bypass_rate` | Fraction of gate-firings waived / overridden by a human | **Transitions where a gate fired** (not total transitions). A phase exit where no gate applied is excluded from both numerator and denominator. | > 15% over 7 days |
| `agent_retry_count_per_ticket` | Turn-retries per ticket before phase exit | per ticket-phase pair | > 5 on any single ticket |
| `human_override_count` | `/approve` events on HumanGates where structural gates had failed | count (absolute) | > 3 per week |
| `reopen_after_measure_rate` | Tickets returning to pre-Measure phase after reaching Measure | tickets that reached Measure in the window | > 10% over 14 days |
| `phase_dwell_time_p90` | 90th percentile time in each phase | per phase | Frame > 2h, Execute > 1d, Measure > 4h |
| `judge_false_pos_rate_<gate>` | LLM-judge golden-set false-positive rate | golden set size (≥20, ≥50 for thresholding) | trend-only until n=50; > 15% absolute once n≥50 |
| `judge_false_neg_rate_<gate>` | LLM-judge golden-set false-negative rate | golden set size (≥20, ≥50 for thresholding) | trend-only until n=50; > 10% absolute once n≥50 (asymmetric — false-neg is worse than false-pos) |
| `labeller_agreement_kappa` | Inter-rater Cohen's κ on golden-set overlap sample (pool phase only, OPEN_QUESTIONS Q-OS-5) | 10% overlap sample of each gate's golden set | κ < 0.7 → rubric insufficient, revert to solo labelling |
| `codex_total_tokens_per_ticket` | Aggregated adapter token cost per ticket | per ticket | informative only |
| `codex_total_tokens_per_phase` | Aggregated adapter token cost per phase class | per phase class | informative only |
| `codex_rate_limit_triggered_count` | How often provider rate-limits are hit | count per window | informative only |

**Why these and not others:** the gates will become the KPI for "ticket
progress" if left unobserved. These indicators watch for the characteristic
failure modes — gaming the gates, agent thrashing, rubber-stamp approvals,
rework from premature closure, stalls — without becoming a target the agent
can optimize. Anti-Goodhart discipline: observation, not reward.

Token-cost indicators are telemetry, not evidence. They are structurally
isolated from the Haft artifact graph per `ILLEGAL_STATES.md` TA1–TA3
(see `HAFT_CONTRACT.md §4` for the routing rules).

---

## 6. Phase → gate binding (MVP-1 canonical)

| Phase | Structural gates | Semantic gates | Human gate (trigger) |
|---|---|---|---|
| `:preflight` | `commission_runnable`, `decision_fresh`, `lockset_available`, `autonomy_envelope_allows` | `context_material_change_review` | — |
| `:frame` | `problem_card_ref_present` (entry), `described_entity_field_present`, `valid_until_field_present` | `object_of_talk_is_specific` | — |
| `:execute` | `design_runtime_split_ok` | `lade_quadrants_split_ok` | `commission_approved` (if `external_publication` matches) |
| `:measure` | `evidence_ref_not_self`, `valid_until_field_present` | `no_self_evidence_semantic` | — |

## 7. Phase → gate binding (MVP-2 additions)

| Phase | Added structural | Added semantic |
|---|---|---|
| `:problematize` | — | (ProblemCard completeness — TBD) |
| `:parity_run` | — | `cg_frame_wellformed` |
| `:commission` | `language_state_complete` | — |
| (any) obligation-language artifact | — | `contract_unpacked_ok` (A.6.C, deferred to MVP-2.1) |

---

## See also

- [PHASE_ONTOLOGY.md](PHASE_ONTOLOGY.md) — gate-kind sum type, verdict, gate-result combining rules
- [ILLEGAL_STATES.md](ILLEGAL_STATES.md) — GK category (gate kinds), PR5 (evidence self-reference), CF4 (unknown gate names)
- [../enabling-system/FUNCTIONAL_ARCHITECTURE.md](../enabling-system/FUNCTIONAL_ARCHITECTURE.md) — L2 gate algebra contracts
- [HAFT_CONTRACT.md](HAFT_CONTRACT.md) — where semantic gates read the Haft graph slice for inputs
- [SLEIGH_CONFIG.md](SLEIGH_CONFIG.md) — how gates are bound to phases in `sleigh.md`
