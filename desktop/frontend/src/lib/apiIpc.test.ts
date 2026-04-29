/// <reference types="node" />

import assert from "node:assert/strict";
import test from "node:test";

import {
  continueTaskIpcArgs,
  projectRootIpcArgs,
  writeTaskInputIpcArgs,
} from "./api.ts";
import {
  taskFollowUpSubmission,
  taskRunState,
} from "./taskInput.ts";

test("spec check uses Tauri camelCase IPC argument shape", () => {
  const args = projectRootIpcArgs("/tmp/haft-product");

  assert.deepEqual(args, {
    projectRoot: "/tmp/haft-product",
  });
  assert.equal("project_root" in args, false);
});

test("live task follow-up uses write task input IPC shape", () => {
  const submission = taskFollowUpSubmission(
    "task-live",
    taskRunState("running"),
    "send this",
  );

  if (submission.kind !== "write_live_input") {
    assert.fail(`expected live input submission, got ${submission.kind}`);
  }

  assert.deepEqual(writeTaskInputIpcArgs(submission), {
    id: "task-live",
    data: "send this",
  });
  assert.equal("message" in writeTaskInputIpcArgs(submission), false);
});

test("completed task follow-up uses continuation IPC shape", () => {
  const submission = taskFollowUpSubmission(
    "task-completed",
    taskRunState("completed"),
    "continue from here",
  );

  if (submission.kind !== "continue_task") {
    assert.fail(`expected continuation submission, got ${submission.kind}`);
  }

  assert.deepEqual(continueTaskIpcArgs(submission), {
    id: "task-completed",
    message: "continue from here",
  });
  assert.equal("data" in continueTaskIpcArgs(submission), false);
});
