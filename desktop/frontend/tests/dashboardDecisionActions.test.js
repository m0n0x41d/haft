import assert from "node:assert/strict";
import test from "node:test";

import { getDecisionImplementActionState } from "../src/pages/dashboardDecisionActions.ts";

test("active DecisionRecords keep Implement enabled", () => {
  const action = getDecisionImplementActionState("active");

  assert.equal(action.disabled, false);
  assert.equal(action.reason, "");
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
