---
id: dec-20260809-resolve-hg-a-in-context-v9-0-2-fix-plan-md-so-th-f317ed49
kind: DecisionRecord
version: 1
status: active
title: Keep the simplified public product story
mode: tactical
valid_until: 2026-08-23
created_at: 2026-08-09T14:57:07Z
updated_at: 2026-08-09T14:57:07Z
---

# Keep the simplified public product story

## 1. Problem Frame

**Problem statement:** Haft's public README and documentation must choose whether to expose four internal release-truth labels or present a simpler product story while preserving the same anti-overclaim boundaries in authoritative internal governance carriers.

## 2. Decision

**Selected:** Keep the simplified public product story

**Selection policy:** Prefer understandable user-facing product copy when strict truth and authority boundaries remain explicit in authoritative internal carriers and tests, and the public copy makes no broader readiness or release claim.

**Why selected:** The operator selected option 1 after reviewing the consequences: keep public copy concise and keep the strict contract, evidence, installed-runtime, and release distinctions in internal governance carriers and regression tests.


**Invariants:**
- Simplified public copy must not promote source, schema, skill, contract, or local-test presence to installed-runtime, RC, or release proof.
- P13, installed P14, host continuity, manual E2E, publication, and release authority remain separate gates.
- This product-copy choice grants no release, publication, P14, or deployment authority.
- The strict truth-boundary vocabulary remains available in authoritative internal governance carriers even when omitted from public copy.

## 3. Rationale

**Counterargument:** Publishing the four labels would make anti-overclaim boundaries directly visible to every reader and reduce dependence on internal governance material.

**Selected variant weakest link:** Public readers may lose the visible vocabulary that distinguishes contract inclusion, exact-candidate evidence, current product status, deferred research, and release authority.

**Rejected alternatives:**
| Variant | Verdict | Reason |
|---------|---------|--------|
| Keep the simplified public product story | **Selected** | The operator selected option 1 after reviewing the conseq... |
| Restore the four explicit release-truth labels publicly | Rejected | It conflicts with the operator-selected simplification and reverses the current README and documentation direction across a larger public-copy surface. |
| Defer the public-copy choice | Rejected | The operator resolved the gate by selecting option 1, so deferral would unnecessarily keep the exact-candidate work blocked. |

**Predictions:**
| Claim | Observable | Threshold |
|-------|------------|-----------|
| The public README and current documentation omit the four internal release-truth labels while retaining a simple Haft and FPF product explanation. | README and docs content plus internal/streamtruth tests | No occurrence of CURRENT PRODUCT, V9 CONTRACT, EXACT-CANDIDATE EVIDENCE, or DEFERRED RESEARCH in public copy, and stream-truth tests pass |
| Internal governance continues to distinguish contract inclusion, exact-candidate evidence, installed runtime, P13, P14, and release authority. | AGENTS.md, embedded agent template, active specs, and stream-truth regression tests | Required internal boundary assertions remain present and the relevant regression suite passes |

## 4. Consequences

**Rollback plan:**
Triggers:
- Public copy causes repeated confusion between contract inclusion, source evidence, installed-runtime readiness, and release authority
- A public compatibility or release claim can no longer be kept accurate without the explicit label vocabulary
Steps:
1. Prepare a separately reviewed patch restoring explicit public truth labels
2. Update the README and documentation together
3. Update stream-truth and P13 manifest anchors to enforce the restored public contract
Blast radius: README, public documentation submodule, and their stream-truth/P13 regression anchors; no runtime or data migration

**Affected files:** README.md, docs, internal/p13acceptance/manifest.json, internal/p13acceptance/manifest_test.go, internal/streamtruth/truth_test.go

<!-- haft:structured_data
{
  "authority_provenance": "host_routed_operator_request",
  "choice_result": {
    "choice_rule": "Prefer understandable user-facing product copy when strict truth and authority boundaries remain explicit in authoritative internal carriers and tests, and the public copy makes no broader readiness or release claim.",
    "comparison_basis": [
      "Selected: simpler public explanation with strict boundaries retained internally",
      "Rejected: maximum public anti-overclaim visibility with a larger reversal of current copy",
      "Rejected: preserves all current bytes but leaves exact-candidate work blocked"
    ],
    "next_move": "choose_now",
    "option_set": [
      "Keep the simplified public product story",
      "Restore the four explicit release-truth labels publicly",
      "Defer the public-copy choice"
    ],
    "reason": "The operator selected option 1 after reviewing the consequences: keep public copy concise and keep the strict contract, evidence, installed-runtime, and release distinctions in internal governance carriers and regression tests.",
    "reopen_condition": "Reopen if the simplified copy causes material truth-boundary ambiguity or requires unsupported public claims.",
    "reversibility": "Reversible through a separately reviewed README/docs/test patch; no runtime or data migration.",
    "subject_ref": "operator",
    "variant_ref": "Keep the simplified public product story"
  },
  "claims": [
    {
      "claim": "The public README and current documentation omit the four internal release-truth labels while retaining a simple Haft and FPF product explanation.",
      "id": "claim-001",
      "observable": "README and docs content plus internal/streamtruth tests",
      "probability": 0.9,
      "status": "unverified",
      "threshold": "No occurrence of CURRENT PRODUCT, V9 CONTRACT, EXACT-CANDIDATE EVIDENCE, or DEFERRED RESEARCH in public copy, and stream-truth tests pass",
      "verify_after": "2026-08-09"
    },
    {
      "claim": "Internal governance continues to distinguish contract inclusion, exact-candidate evidence, installed runtime, P13, P14, and release authority.",
      "id": "claim-002",
      "observable": "AGENTS.md, embedded agent template, active specs, and stream-truth regression tests",
      "probability": 0.9,
      "status": "unverified",
      "threshold": "Required internal boundary assertions remain present and the relevant regression suite passes",
      "verify_after": "2026-08-09"
    }
  ],
  "counterargument": "Publishing the four labels would make anti-overclaim boundaries directly visible to every reader and reduce dependence on internal governance material.",
  "implementation_footprint": {},
  "invariants": [
    "Simplified public copy must not promote source, schema, skill, contract, or local-test presence to installed-runtime, RC, or release proof.",
    "P13, installed P14, host continuity, manual E2E, publication, and release authority remain separate gates.",
    "This product-copy choice grants no release, publication, P14, or deployment authority.",
    "The strict truth-boundary vocabulary remains available in authoritative internal governance carriers even when omitted from public copy."
  ],
  "problem_statement": "Haft's public README and documentation must choose whether to expose four internal release-truth labels or present a simpler product story while preserving the same anti-overclaim boundaries in authoritative internal governance carriers.",
  "rollback_blast_radius": "README, public documentation submodule, and their stream-truth/P13 regression anchors; no runtime or data migration",
  "rollback_steps": [
    "Prepare a separately reviewed patch restoring explicit public truth labels",
    "Update the README and documentation together",
    "Update stream-truth and P13 manifest anchors to enforce the restored public contract"
  ],
  "rollback_triggers": [
    "Public copy causes repeated confusion between contract inclusion, source evidence, installed-runtime readiness, and release authority",
    "A public compatibility or release claim can no longer be kept accurate without the explicit label vocabulary"
  ],
  "selected_title": "Keep the simplified public product story",
  "selection_policy": "Prefer understandable user-facing product copy when strict truth and authority boundaries remain explicit in authoritative internal carriers and tests, and the public copy makes no broader readiness or release claim.",
  "spec_binding_preflight": {
    "authority_boundary": "preflight_is_advisory_validation_not_approval_baseline_evidence_gate_decision_claim_truth_global_truth_or_publication",
    "decision_draft_digest": "sha256:da8d18caf9b813bc6a9b6169e20f5b8220eb9085abff07781775616b6d606e8d",
    "decision_mode": "tactical",
    "load_bearing_level": "low",
    "operator_action_required": "record_rationale",
    "project_spec_state": "ready",
    "record_kind": "spec_binding_preflight",
    "schema_version": 1,
    "state": "out_of_spec",
    "status_debt": {
      "message": "decision is explicitly out-of-spec; status/overseer should retain debt until resolved",
      "severity": "high"
    }
  },
  "spec_binding_preflight_required": true,
  "task_context": "resolve-hg-a-in-context-v9-0-2-fix-plan-md-so-th",
  "weakest_link": "Public readers may lose the visible vocabulary that distinguishes contract inclusion, exact-candidate evidence, current product status, deferred research, and release authority.",
  "why_not_others": [
    {
      "reason": "It conflicts with the operator-selected simplification and reverses the current README and documentation direction across a larger public-copy surface.",
      "variant": "Restore the four explicit release-truth labels publicly"
    },
    {
      "reason": "The operator resolved the gate by selecting option 1, so deferral would unnecessarily keep the exact-candidate work blocked.",
      "variant": "Defer the public-copy choice"
    }
  ],
  "why_selected": "The operator selected option 1 after reviewing the consequences: keep public copy concise and keep the strict contract, evidence, installed-runtime, and release distinctions in internal governance carriers and regression tests."
}
haft:end -->
