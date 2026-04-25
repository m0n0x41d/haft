/// <reference types="node" />

import assert from "node:assert/strict";
import test from "node:test";

import {
  taskAcceptsInput,
  taskCanSubmitFollowUp,
  taskFollowUpAction,
  taskInputCapability,
  taskRunState,
} from "./taskInput.ts";

test("running and idle tasks accept follow-up input", () => {
  const statuses = ["running", "idle"];
  const accepted = statuses.map(taskAcceptsInput);

  assert.deepEqual(accepted, [true, true]);
});

test("terminal tasks do not write to live PTY input", () => {
  const statuses = [
    "completed",
    "failed",
    "cancelled",
    "checkpointed",
    "blocked",
    "Ready for PR",
    "",
  ];
  const accepted = statuses.map(taskAcceptsInput);

  assert.deepEqual(accepted, [false, false, false, false, false, false, false]);
});

test("terminal and checkpointed tasks submit follow-up as continuation", () => {
  const statuses = ["completed", "checkpointed", "failed", "blocked"];
  const capabilities = statuses.map(taskInputCapability);
  const actions = statuses.map(taskFollowUpAction);

  assert.deepEqual(capabilities, statuses.map(() => ({
    kind: "continuation",
    placeholder: "Continue this conversation...",
  })));
  assert.deepEqual(actions, statuses.map(() => ({ kind: "continue_task" })));
  assert.equal(taskCanSubmitFollowUp("completed"), true);
  assert.equal(taskCanSubmitFollowUp("checkpointed"), true);
});

test("empty task status cannot submit follow-up", () => {
  const capability = taskInputCapability("");

  assert.deepEqual(capability, {
    kind: "unavailable",
    placeholder: "Select a task",
  });
  assert.deepEqual(taskFollowUpAction(""), { kind: "none" });
  assert.equal(taskCanSubmitFollowUp(""), false);
});

test("task run state vocabulary normalizes runtime statuses", () => {
  assert.deepEqual(taskRunState("running"), { kind: "running" });
  assert.deepEqual(taskRunState("idle"), { kind: "running" });
  assert.deepEqual(taskRunState("checkpointed"), { kind: "checkpointed" });
  assert.deepEqual(taskRunState("completed"), { kind: "completed" });
  assert.deepEqual(taskRunState("failed"), { kind: "blocked" });
});
