import assert from "node:assert/strict";
import test from "node:test";

import { getDecisionImplementActionState } from "../src/pages/dashboardDecisionActions.ts";

test("active DecisionRecords keep Implement enabled", () => {
  const action = getDecisionImplementActionState("active");

  assert.equal(action.disabled, false);
  assert.equal(action.reason, "");
  assert.deepEqual(action.confirmationMessages, []);
  assert.deepEqual(action.warningMessages, []);
});

test("superseded and deprecated DecisionRecords disable Implement", () => {
  const superseded = getDecisionImplementActionState("superseded");
  const deprecated = getDecisionImplementActionState("deprecated");

  assert.equal(superseded.disabled, true);
  assert.match(superseded.reason, /superseded/i);

  assert.equal(deprecated.disabled, true);
  assert.match(deprecated.reason, /deprecated/i);
});

test("non-active statuses stay blocked until they become active", () => {
  const refreshDue = getDecisionImplementActionState("refresh_due");

  assert.equal(refreshDue.disabled, true);
  assert.equal(
    refreshDue.reason,
    "Implement is available only for active DecisionRecords.",
  );
});

test("G1 blocks Implement even for active decisions", () => {
  const action = getDecisionImplementActionState("active", {
    blocked_reason: "Multiple active decisions for this problem — supersede one first",
    confirmation_messages: [],
    warning_messages: [],
  });

  assert.equal(action.disabled, true);
  assert.match(action.reason, /multiple active decisions/i);
});

test("warning and confirmation messages stay available for the click flow", () => {
  const action = getDecisionImplementActionState("active", {
    blocked_reason: "",
    confirmation_messages: [
      "No parity plan recorded — comparison may not be fair — proceed?",
      "Comparison basis includes unresolved subjective dimensions — proceed?",
    ],
    warning_messages: [
      "No invariants defined — post-execution verification will be skipped",
    ],
  });

  assert.equal(action.disabled, false);
  assert.equal(action.reason, "");
  assert.equal(action.confirmationMessages.length, 2);
  assert.equal(action.warningMessages.length, 1);
});
