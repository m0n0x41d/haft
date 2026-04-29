/// <reference types="node" />

import assert from "node:assert/strict";
import test from "node:test";

import { mergeTaskStatusEvent } from "./taskState.ts";
import type { TaskState } from "./api.ts";

function taskState(overrides: Partial<TaskState> = {}): TaskState {
  return {
    id: "task-1",
    title: "Existing task",
    agent: "codex",
    project: "haft",
    project_path: "/repo/haft",
    status: "running",
    prompt: "Do work",
    branch: "",
    worktree: false,
    worktree_path: "",
    reused_worktree: false,
    started_at: "2026-04-24T00:00:00Z",
    completed_at: "",
    error_message: "",
    output: "transcript",
    chat_blocks: [],
    raw_output: "raw transcript",
    auto_run: false,
    ...overrides,
  };
}

test("status event updates only status fields on an existing task", () => {
  const current = [taskState()];
  const next = mergeTaskStatusEvent(current, {
    id: "task-1",
    status: "completed",
    error_message: "",
  });

  assert.equal(next[0].status, "completed");
  assert.equal(next[0].title, "Existing task");
  assert.equal(next[0].prompt, "Do work");
  assert.equal(next[0].raw_output, "raw transcript");
});

test("status event for unknown task does not construct a partial task", () => {
  const current = [taskState()];
  const next = mergeTaskStatusEvent(current, {
    id: "task-unknown",
    status: "completed",
    error_message: "",
  });

  assert.deepEqual(next, current);
});
