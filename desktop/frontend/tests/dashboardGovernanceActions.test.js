import assert from "node:assert/strict";
import test from "node:test";

import { getGovernanceFindingActionState } from "../src/pages/dashboardGovernanceActions.ts";

test("decision stale findings expose actions and map Adopt to the matching candidate", () => {
  const finding = {
    id: "finding-1",
    artifact_ref: "dec-1",
    title: "Decision drifted",
    kind: "DecisionRecord",
    category: "decision_stale",
    reason: "Three files drifted from baseline.",
    valid_until: "",
    days_stale: 4,
    r_eff: 0,
    drift_count: 3,
  };
  const candidates = [
    {
      id: "cand-keep",
      status: "active",
      title: "Investigate drift",
      signal: "Three files drifted from baseline.",
      acceptance: "Drift is resolved.",
      context: "desktop-governance",
      category: "decision_stale",
      source_artifact_ref: "dec-1",
      source_title: "Decision drifted",
      problem_ref: "",
    },
  ];

  const actionState = getGovernanceFindingActionState(finding, candidates);

  assert.equal(actionState.showActions, true);
  assert.equal(actionState.adoptDisabled, false);
  assert.equal(actionState.adoptCandidateID, "cand-keep");
});

test("non-actionable governance findings stay read-only", () => {
  const finding = {
    id: "finding-2",
    artifact_ref: "dec-2",
    title: "Pending verification",
    kind: "DecisionRecord",
    category: "pending_verification",
    reason: "A claim is due for measurement.",
    valid_until: "",
    days_stale: 0,
    r_eff: 0,
    drift_count: 0,
  };

  const actionState = getGovernanceFindingActionState(finding, []);

  assert.equal(actionState.showActions, false);
  assert.equal(actionState.adoptDisabled, true);
  assert.equal(actionState.adoptCandidateID, "");
});

test("actionable findings disable Adopt when no active candidate matches", () => {
  const finding = {
    id: "finding-3",
    artifact_ref: "dec-3",
    title: "Decision stale",
    kind: "DecisionRecord",
    category: "evidence_expired",
    reason: "The decision validity expired.",
    valid_until: "2026-04-01T00:00:00Z",
    days_stale: 13,
    r_eff: 0,
    drift_count: 0,
  };
  const candidates = [
    {
      id: "cand-dismissed",
      status: "dismissed",
      title: "Refresh stale artifact",
      signal: "The decision validity expired.",
      acceptance: "Decision is fresh again.",
      context: "desktop-governance",
      category: "evidence_expired",
      source_artifact_ref: "dec-3",
      source_title: "Decision stale",
      problem_ref: "",
    },
  ];

  const actionState = getGovernanceFindingActionState(finding, candidates);

  assert.equal(actionState.showActions, true);
  assert.equal(actionState.adoptDisabled, true);
  assert.equal(actionState.adoptCandidateID, "");
  assert.match(actionState.adoptReason, /no active follow-up candidate/i);
});
