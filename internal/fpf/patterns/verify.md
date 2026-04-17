# FPF Patterns: Verify Phase

## VER-01: Evidence Graph
**Trigger:** Gathering results; tracking what supports what claim
**Spec:** A.10, B.3.5, B.1.1

Every evidence artifact is a node in a graph. Edges: supports, refutes, partially addresses. Each link typed (measurement, test, research, audit) with Congruence Level (CL). Graph is traceable: reviewers follow any claim back to evidence. No floating claims without evidence anchors.

## VER-02: Evidence Decay and Staleness
**Trigger:** Circumstances change; old evidence loses credibility
**Spec:** B.3.4, B.3.4.5, G.11

Every evidence artifact has valid_until date. Expired evidence scores 0.1 (weak, not absent). Track epistemic debt: as evidence ages, debt builds. Stale evidence at threshold triggers re-validation or decision reopening.

## VER-03: R_eff Computation
**Trigger:** Multiple evidence pieces; computing overall confidence
**Spec:** B.3.4.4

R_eff = max(0, R_raw - phi(CL_min)). R_raw = min(all component Rs). phi(CL_min) = penalty from lowest congruence level. Even strong individual pieces (high R) with poor integration (low CL) reduce R_eff. Conservative: never average Rs or CL; take minimum.

## VER-04: Assurance Subtypes (TA/VA/LA)
**Trigger:** Accumulating different evidence types
**Spec:** B.3.3, B.3.5.1

TA (Typological Assurance): concept map correct? Types aligned? VA (Verification Assurance): internal logic sound? Specs imply consequences? LA (Validation Assurance): works in real world? Track which subtype is weakest. Overinvestment in LA when VA is broken is waste.

## VER-05: Measurement Integrity
**Trigger:** Gathering evidence for a decision already made; bias danger
**Spec:** C.16, B.3

Metrics declared before testing. Collection method blind. Error bounds realistic. Null result acceptable. Avoid p-hacking, moving goalposts, hiding unfavorable data.

## VER-06: Assurance Level Progression (L0-L3)
**Trigger:** Tracking progress toward operational confidence
**Spec:** B.3.3, B.5.1, B.5

L0 (Unsubstantiated): hypothesis, no evidence. L1 (Shaped): deduced consequences, design-time verification. L2 (Tested): empirical evidence, field-tested. L3 (Operational): sustained in-service evidence, drift monitoring. Move up only by accumulating evidence in right subtypes.

## VER-07: Refresh Triggers
**Trigger:** Decision implemented; need to know when to re-evaluate
**Spec:** G.11, B.4

Triggers: evidence expiry (valid_until passed), material context change (assumptions K no longer hold), weakest link failure, competing alternative emerges. State explicitly in decision record. Automated monitoring can flag; decision owner decides whether to reopen.

## VER-08: Valid-Until as Lifecycle Trigger
**Trigger:** Artifact confidence expires; mandatory governance
**Source:** Haft operational pattern

Every evidence, problem, comparison result has valid_until. On expiry: Refresh (re-check), Deprecate (mark obsolete), or Waive (accept stale with explicit sign-off + new deadline). Track epistemic debt = gap between artifact date and now.

## VER-09: Cross-Session Term Persistence
**Trigger:** Multi-session projects; term drift risk
**Source:** Haft operational pattern (derived from semiotics practice)

Reuse disambiguation record from session 1 in session 2 IF: same project, same bounded context, same term family, no superseding record, within validity window. Reopen if: context shifts, term in new artifact class, ontology changes, record expired.
