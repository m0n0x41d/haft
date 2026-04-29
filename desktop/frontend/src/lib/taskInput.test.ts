/// <reference types="node" />

import assert from "node:assert/strict";
import test from "node:test";

import {
  taskAcceptsInput,
  taskCanArchive,
  taskCanCancel,
  taskCanSubmitFollowUp,
  taskFollowUpAction,
  taskFollowUpSubmission,
  taskInputCapability,
  taskRunState,
} from "./taskInput.ts";

test("running and idle tasks accept follow-up input", () => {
  const statuses = ["running", "idle"];
  const accepted = statuses
    .map(taskRunState)
    .map(taskAcceptsInput);

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
  const accepted = statuses
    .map(taskRunState)
    .map(taskAcceptsInput);

  assert.deepEqual(accepted, [false, false, false, false, false, false, false]);
});

test("dead PTY follow-ups compile to continuations, never live input", () => {
  const statuses = [
    "completed",
    "failed",
    "cancelled",
    "interrupted",
    "Ready for PR",
    "Needs attention",
    "checkpointed",
    "blocked",
    "provider_paused",
  ];
  const submissions = statuses
    .map(taskRunState)
    .map((state) => taskFollowUpSubmission("task-dead-pty", state, "continue"));

  assert.deepEqual(submissions.map((submission) => submission.kind), [
    "continue_task",
    "continue_task",
    "continue_task",
    "continue_task",
    "continue_task",
    "continue_task",
    "continue_task",
    "continue_task",
    "continue_task",
  ]);
});

test("terminal and checkpointed tasks submit follow-up as continuation", () => {
  const statuses = ["completed", "checkpointed", "failed", "blocked"];
  const states = statuses.map(taskRunState);
  const capabilities = states.map(taskInputCapability);
  const actions = states.map(taskFollowUpAction);

  assert.deepEqual(capabilities, statuses.map(() => ({
    kind: "continuation",
    placeholder: "Continue this conversation...",
  })));
  assert.deepEqual(actions, statuses.map(() => ({ kind: "continue_task" })));
  assert.equal(taskCanSubmitFollowUp(taskRunState("completed")), true);
  assert.equal(taskCanSubmitFollowUp(taskRunState("checkpointed")), true);
});

test("third and fourth follow-ups after non-running outcomes remain continuations", () => {
  const outcomes = ["completed", "checkpointed", "failed", "blocked"];
  const followUpTurns = outcomes
    .map(taskRunState)
    .map((state, index) => ({
      turn: index + 2,
      acceptsLiveInput: taskAcceptsInput(state),
      canSubmit: taskCanSubmitFollowUp(state),
      action: taskFollowUpAction(state),
    }));

  assert.deepEqual(followUpTurns, [
    {
      turn: 2,
      acceptsLiveInput: false,
      canSubmit: true,
      action: { kind: "continue_task" },
    },
    {
      turn: 3,
      acceptsLiveInput: false,
      canSubmit: true,
      action: { kind: "continue_task" },
    },
    {
      turn: 4,
      acceptsLiveInput: false,
      canSubmit: true,
      action: { kind: "continue_task" },
    },
    {
      turn: 5,
      acceptsLiveInput: false,
      canSubmit: true,
      action: { kind: "continue_task" },
    },
  ]);
});

test("empty task status cannot submit follow-up", () => {
  const state = taskRunState("");
  const capability = taskInputCapability(state);

  assert.deepEqual(capability, {
    kind: "unavailable",
    placeholder: "Select a task",
  });
  assert.deepEqual(taskFollowUpAction(state), { kind: "none" });
  assert.equal(taskCanSubmitFollowUp(state), false);
});

test("task run state vocabulary normalizes runtime statuses", () => {
  assert.deepEqual(taskRunState("running"), {
    kind: "live",
    rawStatus: "running",
    normalizedStatus: "running",
    mode: "running",
  });
  assert.deepEqual(taskRunState("idle"), {
    kind: "live",
    rawStatus: "idle",
    normalizedStatus: "idle",
    mode: "idle",
  });
  assert.deepEqual(taskRunState("checkpointed"), {
    kind: "needs_operator",
    rawStatus: "checkpointed",
    normalizedStatus: "checkpointed",
    reason: "checkpointed",
  });
  assert.deepEqual(taskRunState("completed"), {
    kind: "terminal",
    rawStatus: "completed",
    normalizedStatus: "completed",
    outcome: "completed",
  });
  assert.deepEqual(taskRunState("failed"), {
    kind: "terminal",
    rawStatus: "failed",
    normalizedStatus: "failed",
    outcome: "failed",
  });
});

test("normalized live states are not archiveable even when carrier formatting drifts", () => {
  const state = taskRunState(" RUNNING ");

  assert.equal(taskCanCancel(state), true);
  assert.equal(taskCanArchive(state), false);
  assert.deepEqual(taskFollowUpAction(state), { kind: "write_live_input" });
});

test("unknown task status is a continuation state, not live input", () => {
  const state = taskRunState("provider_paused");

  assert.deepEqual(state, {
    kind: "terminal",
    rawStatus: "provider_paused",
    normalizedStatus: "provider paused",
    outcome: "unknown",
  });
  assert.equal(taskAcceptsInput(state), false);
  assert.deepEqual(taskFollowUpAction(state), { kind: "continue_task" });
});

test("follow-up submission compiles live tasks to PTY input", () => {
  const submission = taskFollowUpSubmission(
    "task-live",
    taskRunState("running"),
    "  continue with tests  ",
  );

  assert.deepEqual(submission, {
    kind: "write_live_input",
    id: "task-live",
    data: "continue with tests",
  });
});

test("follow-up submission compiles terminal tasks to continuation", () => {
  const submission = taskFollowUpSubmission(
    "task-done",
    taskRunState("completed"),
    "  next turn  ",
  );

  assert.deepEqual(submission, {
    kind: "continue_task",
    id: "task-done",
    message: "next turn",
  });
});

test("follow-up submission keeps empty and unavailable input effect-free", () => {
  const empty = taskFollowUpSubmission(
    "task-live",
    taskRunState("running"),
    "   ",
  );
  const unavailable = taskFollowUpSubmission(
    "task-none",
    taskRunState(""),
    "hello",
  );

  assert.deepEqual(empty, {
    kind: "none",
    reason: "empty_input",
  });
  assert.deepEqual(unavailable, {
    kind: "none",
    reason: "unavailable",
  });
});
