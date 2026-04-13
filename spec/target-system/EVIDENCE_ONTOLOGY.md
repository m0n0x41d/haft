# Evidence Ontology

> Reading order: 6 of N. Read after ARTIFACT_ONTOLOGY.

## Purpose

Evidence is the mechanism that prevents decisions from being narratives. A decision without evidence is a story. A decision with evidence has a computed trust score that degrades as assumptions expire.

## R_eff (Effective Reliability)

The trust score of a DecisionRecord, computed per-claim then aggregated.

### Two-level computation

```
R_eff(claim) = min(effective_score for each evidence item referencing that claim)
R_eff(decision) = min(R_eff(claim_i) for each claim on the decision)
```

Where each evidence item's effective score is:

```
effective_score = base_verdict_score - CL_penalty - decay_penalty
```

If a decision has no claims, R_eff is computed over all evidence items directly (backward-compatible with pre-claim decisions).

### Verdict → Base Score

| Verdict | Base score |
|---------|-----------|
| `supports` | 1.0 |
| `weakens` | 0.5 |
| `refutes` | 0.0 |
| `superseded` | excluded from R_eff (kept for audit) |

### CL → Penalty

| Congruence Level | Penalty | When |
|-----------------|---------|------|
| CL3 (same context) | 0.0 | Internal test on this project |
| CL2 (similar context) | 0.1 | Related project, same language |
| CL1 (different context) | 0.4 | External docs, different language |
| CL0 (opposed context) | **inadmissible** | `CL0 + supports` is invalid. Evidence from opposed context must be re-evaluated or rejected before entering computation. |

### Decay → Penalty

| State | Penalty |
|-------|---------|
| Fresh (within valid_until) | 0.0 |
| Expired (past valid_until) | score drops to 0.1 (weak, not absent) |

### R_eff Thresholds

| R_eff | Status | Action |
|-------|--------|--------|
| ≥ 0.5 | Healthy | No action needed |
| < 0.5 | Degraded | Surfaces in `/h-verify scan` |
| < 0.3 | AT RISK | Flagged with high severity |
| No evidence | **Unassessed** | Not healthy, not broken — shown separately from healthy decisions. UI surfaces coverage gaps (claims without evidence). |

**"No evidence ≠ healthy."** Missing evidence means the decision has not been verified. Unassessed decisions are visible so engineers can prioritize verification.

### Critical Rule: Weakest Link

R_eff uses **min**, not average. One weak evidence item on one claim bounds the whole decision's trust. This is the WLNK principle applied to evidence chains.

## F_eff and G_eff (Formality and Groundedness)

R_eff is the aggregate trust score. The desktop UI decomposes it into three dimensions:

| Dimension | What it measures | Computed from |
|-----------|-----------------|---------------|
| **F_eff (Formality)** | How structured is the evidence? | Evidence type: F0 (anecdote) → F1 (observation) → F2 (repeatable test) → F3 (formal proof) |
| **G_eff (Groundedness)** | How close to the thing verified? | CL: CL3 = direct, CL2 = similar, CL1 = indirect, CL0 = inadmissible |
| **R_eff (Reliability)** | Does evidence support the claim? | Verdict + CL penalty + decay (as above) |

F_eff and G_eff are **view concerns** — they decompose R_eff's inputs for diagnosis. They are not separate trust scores.

## Claims and Predictions

DecisionRecords contain **claims** — falsifiable statements about what the decision should achieve.

| Field | Purpose | Example |
|-------|---------|---------|
| `claim` | What we believe will happen | "Response latency drops below 200ms" |
| `observable` | What to measure | "p99 latency on /api/auth endpoint" |
| `threshold` | Success condition | "< 200ms sustained over 24h" |
| `verify_after` | When to check (optional) | "2026-05-01" |

**Pending verification:** When `verify_after` passes and the claim is still unverified, `/h-verify scan` surfaces it with the observable and threshold.

## Evidence Types

| Type | What it is | Typical CL |
|------|-----------|-----------|
| `test_result` | Automated test pass/fail | CL3 |
| `benchmark` | Performance measurement | CL3 |
| `code_review` | Human review verdict | CL3 |
| `user_feedback` | Production user observation | CL2-CL3 |
| `documentation` | External docs/specs | CL1-CL2 |
| `expert_opinion` | Human expert judgment | CL2 |
| `cross_project` | Evidence from another project | CL1-CL2 |

## Evidence Lifecycle

```
                  ┌─────────┐
                  │  Fresh  │ (within valid_until)
                  └────┬────┘
                       │ valid_until passes
                       ▼
                  ┌─────────┐
                  │ Expired │ (score = 0.1, flagged in scan)
                  └────┬────┘
                       │
            ┌──────────┼──────────┐
            ▼          ▼          ▼
       ┌────────┐ ┌────────┐ ┌──────────┐
       │ Waived │ │Reopened│ │Superseded│
       │(extend)│ │ (new   │ │(new meas │
       │        │ │problem)│ │replaces) │
       └────────┘ └────────┘ └──────────┘
```

**Evidence supersession (claim-scoped):** When `haft_decision(action="measure")` records a new measurement for a specific claim/observable, previous measurements **for the same claim** are marked `verdict='superseded'` and excluded from R_eff. Measurements for other claims on the same decision are unaffected. Old measurements stay for audit trail.

Supersession key: `(artifact_id, claim_ref, observable)`. A new latency measurement does not retire a still-valid correctness measurement.

## Measurement Protocol

1. **Baseline first** — `haft_decision(action="baseline")` snapshots file hashes (auto-runs after decide when affected_files present)
2. **Verify inductively** — run the actual test/benchmark/observation
3. **Attach evidence** — `haft_decision(action="evidence")` with type/verdict/CL/valid_until
4. **Record measurement** — `haft_decision(action="measure")` with findings and final verdict

**Violations:**
- Calling measure from memory without running verification → invalid
- Calling measure without baseline → evidence degrades to CL1 (self-evidence)
- Evidence without measure → doesn't close the loop

## Drift Detection

After baseline, the system detects three drift states:

| State | Meaning |
|-------|---------|
| **No drift** | File hashes match baseline |
| **MODIFIED** | File content changed since baseline |
| **FILE MISSING** | Baselined file no longer exists |

Drift is a **signal**, not automatic invalidation. Modified doesn't mean broken — it means someone should check if the decision still holds.

**Impact propagation:** When module A drifts, the system flags:
- All decisions governing module A
- All decisions governing modules that depend on A (transitive)
