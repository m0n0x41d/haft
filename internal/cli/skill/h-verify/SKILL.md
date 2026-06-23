---
name: h-verify
description: |
  Verifies that a recorded DecisionRecord still holds — baseline-vs-measure evidence loop with drift detection per FPF Evidence Decay. Make sure to use this skill whenever the user asks "did dec-X work", "is decision Y still valid", "did the prediction come true", "check if the migration held", "is X stale", "measure that decision against reality", "did we actually fix Z", "is our caching decision still right" — or whenever a shipped decision needs a post-implementation reality-check before further work relies on it. Also use when /h-status surfaces a refresh-due decision. NOT for ad-hoc sanity checks (just run the tests directly). NOT for re-framing the underlying problem (use h-frame).
when_to_use: |
  A shipped DecisionRecord needs reality check, OR refresh-due artifact surfaced. Skip for one-off sanity checks where you can just run the test.
argument-hint: "[decision-ref or 'what's stale' for full project verification]"
allowed-tools: Bash Read Grep Glob mcp__haft__haft_decision mcp__haft__haft_query mcp__haft__haft_refresh
---

<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:5e895ffff0de58df3654eb2ee2ce47be8dfad7524b0e1102ef8950689bbdf44c -->

# h-verify — Verify a decision still holds

You are running the FPF verification loop: baseline → measure → evidence → record. Drift detection compares current state against baselined affected_files; evidence decay reports surface when valid_until passes; measure verdict is recorded for the predictions the decide step declared.

## Step 1 — Identify the decision

If `decision_ref` is given, use it. Otherwise:
- `mcp__haft__haft_query(action="status")` — surfaces stale/refresh-due decisions
- `mcp__haft__haft_refresh(action="drain", dry_run=true)` — preview machine-safe maintenance closures and needs_operator groups
- `mcp__haft__haft_query(action="contract_generation")` — read-only generated-fragment carrier hints for host/skill/plugin/Pi sync
- `mcp__haft__haft_query(action="drift_events", limit=5)`, `mcp__haft__haft_query(action="decision_reconcile", limit=5)`, and `mcp__haft__haft_query(action="governing_set", limit=5)` — compact drift fanout, reconciliation, and current-authority drill-downs
- add `full=true` to those drill-down calls only when you need the full audit payload
- `mcp__haft__haft_query(action="list", kind="DecisionRecord")` — full list
- Ask the operator which decision to verify

read-only/generated text is discovery only; it is not evidence truth, gate passage, global approval, or operator authorization.

When the operator asks to verify/drain the project maintenance backlog, do not
stop after status. Use the explicit drain loop:

1. `mcp__haft__haft_refresh(action="drain", dry_run=true)` to preview closed /
   failed / needs_operator classes.
2. If the operator's request already authorizes safe closure (for example:
   "h-verify and close what you can"), call
   `mcp__haft__haft_refresh(action="drain", dry_run=false)`.
3. Report every automatic closure and every needs_operator item. The drain may
   close only rung-1 deterministic drift and rung-2 allowlisted machine
   evidence/revalidation. Material drift, semantic uncertainty, reopen/supersede
   choices, and weak waivers stay operator-facing.

## Step 2 — Read the decision's predictions

`mcp__haft__haft_query(action="search", query="<decision_ref>")` returns the DecisionRecord including its `predictions` field. Each prediction has:
- `claim` — the falsifiable statement
- `observable` — what to measure
- `threshold` — pass/fail boundary
- `verify_after` — when async evidence should be available (if any)

If predictions are empty (the decision was recorded tactical with `_skips: ["predictions"]`), there's nothing to measure — report that to operator and recommend either:
- `/h-refresh` action=reopen to add predictions and re-decide properly
- Just attach evidence directly via `haft_decision(action="evidence", ...)`

## Step 3 — Baseline (if drift detection wanted)

If the decision has `affected_files` and you want drift comparison:

```
mcp__haft__haft_decision(
  action="baseline",
  decision_ref="<dec-...>"
  // affected_files optional — kernel uses the decision's list
)
```

The kernel snapshots file content hashes. Subsequent comparisons detect drift. Call once after each commit cycle if you want continuous drift signal.

## Step 4 — Gather evidence per prediction

For each prediction:
- Run the observable (test, metric query, log scan, code grep)
- Compare to threshold
- Capture the actual measurement value

Tools available depending on the observable:
- `Bash` for test runners, metric queries, log scans
- `Read` / `Grep` / `Glob` for code-level invariant checks
- For external metrics: kernel has no special integration; agent describes the source

## Step 5 — Attach evidence to the artifact

For each material evidence item:

```
mcp__haft__haft_decision(
  action="evidence",
  artifact_ref="<dec-...>",
  evidence_type="measurement | test | research | benchmark | audit",
  evidence_content="<what you observed, with concrete numbers>",
  evidence_verdict="supports | weakens | refutes",
  carrier_ref="<file path or URL where the evidence lives>",
  claim_refs=["<prediction id or scope label>"],
  congruence_level=3,  // 3=same context, 2=similar, 1=different, 0=opposed
  valid_until="<RFC3339 or YYYY-MM-DD — when this evidence expires>",
  causal_support_basis="observational | interventional | realized_counterfactual | identified_estimate | simulation_only"
)
```

**congruence_level** (CL) defaults per FPF B.3.5:
- 3: same-context evidence (own production system, own tests)
- 2: similar-context (related project, similar load)
- 1: different-context (external docs, vendor benchmarks)
- 0: opposed-context (rare; conflicting framework)

CL impacts R_eff per FPF B.3:3 — never average across CL.

## Step 6 — Record the measurement verdict

**What-got-worse check (FPF E.13).** Before recording an `accepted` or
`partial` verdict, ask: the measured proxy improved — what got worse? Check
the protected qualities the decision did NOT optimize (usability, footprint,
maintainability, signal honesty, adjacent modules). Put the answer in
`findings`: either the concrete regression you found, or a reasoned
"none worsened: checked X, Y, Z" — a reflexive "nothing worsened" without
named loci is the Goodhart failure mode this check exists to catch.

After all evidence is attached, record the overall verdict:

```
mcp__haft__haft_decision(
  action="measure",
  decision_ref="<dec-...>",
  verdict="accepted | partial | failed",
  findings="<what actually happened compared to predictions>",
  measurements=["p99 latency: 42ms (predicted <50ms — accepted)", "..."],
  criteria_met=["<criterion that was met>"],
  criteria_not_met=["<criterion that was NOT met>"]
)
```

Kernel ties the verdict back to the predictions and surfaces:
- Accepted → decision health remains good
- Partial → some predictions held, some didn't → consider reopen or supersede
- Failed → decision invalidated → consider supersede or rollback per the decision's rollback spec

## Step 6.5 — Loop verdict (FPF E.23)

After the measure verdict, classify what the improvement loop does next. The
admissible verdict set is exactly:

- **stop** — further movement is dominated, unavailable, or out of scope.
  "Good enough" is a legitimate terminal state: a decision measured adequate
  does NOT owe an endless refresh cycle. Name WHY movement is dominated.
- **continue** — evidence gathering or improvement continues on the same frame.
- **switch-method** — same problem, different solution method → supersede.
- **open-new-frame** — the problem itself shifted → reopen with a new ProblemCard.
- **hold** — defer with an explicit revisit condition → waive with new_valid_until.

**Narrowing is NOT a verdict.** Shrinking scan scope, suppressing signals, or
restricting the evaluation window to make the picture look better is false
improvement — any scope reduction must carry an explicit dominance
justification ("movement on X is dominated because Y"), or it doesn't happen.

If any verified prediction carried a `probability` forecast (set at `/h-decide`),
the measure response also appends a **Calibration** read: the decomposed-Brier
profile (Brier = reliability − resolution + uncertainty) over all verified
forecasts, plus a directional over/under-confidence bias. Below ~15 accumulated
forecasts it reports cold-start and is not yet actionable — surface it to the
operator but do not over-read a sparse profile.

## Step 7 — Handle stale or drifted decisions

If verification reveals:
- **Evidence decayed** (valid_until passed): `mcp__haft__haft_refresh(action="waive", artifact_ref=..., evidence="<new evidence>", new_valid_until="...")` to extend validity, OR `action=reopen` to start a new problem cycle
- **Drift detected** (affected_files changed since baseline): classify drift as cosmetic / incidental / material via `haft_query(action="status")` and decide whether to re-baseline or reopen
- **Verdict failed**: `mcp__haft__haft_refresh(action="supersede", artifact_ref=<old>, new_artifact_ref=<replacement>)` after recording the replacement decision via `/h-decide`

## Step 8 — Present to operator

Surface:
- Predictions vs actual measurements
- Verdict (accepted / partial / failed)
- Evidence attached with CL
- Drift status if baseline existed
- Loop verdict (stop / continue / switch-method / open-new-frame / hold) with
  its kernel mapping: stop → leave active with rationale recorded; continue →
  schedule next verify; switch-method → supersede; open-new-frame → reopen;
  hold → waive with new_valid_until

**Re-grounding discipline (FPF A.7).** When you reference decision IDs
(`dec-20260525-...`), prediction labels, or evidence refs in the verdict
summary and recommendation paragraphs, pair each with its
human-readable title or claim — `dec-20260525-abc (NATS over Kafka for
ops simplicity) — verdict accepted` not bare `dec-20260525-abc verdict
accepted`. Bare IDs accumulate cognitive debt across long sessions. Keep
IDs for traceability but never let them stand alone in summaries. See
CLAUDE.md Critical Reminders for the project-wide rule.

## What NOT to do

- Do NOT call `action="measure"` without first gathering evidence — kernel rejects measure-from-memory (the protocol requires evidence before verdict).
- Do NOT use congruence_level=3 for evidence that came from a different context (vendor benchmark, external doc). Misclassifying CL inflates R_eff and corrupts trust signal.
- Do NOT skip baseline if the decision has affected_files and drift matters — without baseline, drift is invisible.
- Do NOT silently change a decision's verify_after to "kick the can". Use `action="waive"` with `evidence` parameter so the extension is auditable.
- Do NOT mark verdict=accepted when predictions were skipped — verify what was declared, not what was hoped.

## FPF spec references

- B.3 — Trust & Assurance Calculus (F-G-R + CL)
- B.3.3 — Assurance Subtypes & Levels (L0/L1/L2 + TA/VA/LA)
- B.3.4 — Evidence Decay & Epistemic Debt
- A.10 — Evidence Graph Referring
- VER-01 (evidence graph), VER-02 (decay), VER-03 (R_eff), VER-07 (refresh triggers)
- C.27 — Temporal Claim Adequacy (state reading vs trend vs intervention claim)

Look up via `mcp__haft__haft_query(action="fpf", query="B.3")`.
