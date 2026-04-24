/// <reference types="node" />

import assert from "node:assert/strict";
import test from "node:test";

import { taskAcceptsInput, taskCanSubmitFollowUp, taskInputCapability } from "./taskInput.ts";

test("running and idle tasks accept follow-up input", () => {
  const statuses = ["running", "idle"];
  const accepted = statuses.map(taskAcceptsInput);

  assert.deepEqual(accepted, [true, true]);
});

test("terminal tasks do not write to live PTY input", () => {
  const statuses = ["completed", "failed", "cancelled", "Ready for PR", ""];
  const accepted = statuses.map(taskAcceptsInput);

  assert.deepEqual(accepted, [false, false, false, false, false]);
});

test("terminal tasks submit follow-up as continuation", () => {
  const capability = taskInputCapability("completed");

  assert.deepEqual(capability, {
    kind: "continuation",
    placeholder: "Continue this conversation...",
  });
  assert.equal(taskCanSubmitFollowUp("completed"), true);
});

test("empty task status cannot submit follow-up", () => {
  const capability = taskInputCapability("");

  assert.deepEqual(capability, {
    kind: "unavailable",
    placeholder: "Select a task",
  });
  assert.equal(taskCanSubmitFollowUp(""), false);
});
