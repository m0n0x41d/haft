import assert from "node:assert/strict";
import test from "node:test";

import { getTaskExecutionLadder } from "../src/pages/taskExecutionLadder.ts";

function implementationTask(overrides = {}) {
  return {
    id: "task-implement-1",
    title: "Implement: Status ladder",
    agent: "codex",
    project: "haft",
    project_path: "/tmp/haft",
    status: "running",
    prompt: [
      "## Implement Decision: Status ladder",
      "",
      "Decision ID: dec-201",
      "",
      "## Invariants (must hold)",
      "- Keep the task view derived from backend task state.",
      "",
      "## Affected Files",
      "- desktop/frontend/src/pages/Tasks.tsx",
    ].join("\n"),
    branch: "feat/status-ladder",
    worktree: true,
    worktree_path: "/tmp/haft/.haft/worktrees/feat/status-ladder",
    reused_worktree: false,
    started_at: "2026-04-14T10:00:00Z",
    completed_at: "",
    error_message: "",
    output: "",
    auto_run: false,
    ...overrides,
  };
}

function stepStateByLabel(ladder) {
  return Object.fromEntries(ladder.steps.map((step) => [step.label, step.state]));
}

test("running implement task highlights the running step", () => {
  const ladder = getTaskExecutionLadder(implementationTask());

  assert.ok(ladder);
  assert.equal(ladder.currentLabel, "Running");
  assert.match(ladder.summary, /updating live/i);
  assert.deepEqual(stepStateByLabel(ladder), {
    Planned: "complete",
    Running: "current",
    Verifying: "upcoming",
    "Ready for PR": "upcoming",
    "Needs attention": "upcoming",
  });
});

test("completed implement task with verification requirements advances to verifying", () => {
  const ladder = getTaskExecutionLadder(
    implementationTask({
      status: "completed",
      completed_at: "2026-04-14T10:30:00Z",
    }),
  );

  assert.ok(ladder);
  assert.equal(ladder.currentLabel, "Verifying");
  assert.match(ladder.summary, /post-execution verification/i);
  assert.equal(stepStateByLabel(ladder).Verifying, "current");
});

test("ready-for-pr task completes verification and exposes the success outcome", () => {
  const ladder = getTaskExecutionLadder(
    implementationTask({
      status: "Ready for PR",
      completed_at: "2026-04-14T10:40:00Z",
    }),
  );

  assert.ok(ladder);
  assert.equal(ladder.currentLabel, "Ready for PR");
  assert.equal(stepStateByLabel(ladder)["Ready for PR"], "current");
  assert.equal(stepStateByLabel(ladder)["Needs attention"], "upcoming");
});

test("verification failures keep the ladder on the attention outcome", () => {
  const ladder = getTaskExecutionLadder(
    implementationTask({
      status: "Needs attention",
      completed_at: "2026-04-14T10:40:00Z",
    }),
  );

  assert.ok(ladder);
  assert.equal(ladder.currentLabel, "Needs attention");
  assert.match(ladder.summary, /verification found an issue/i);
  assert.equal(stepStateByLabel(ladder).Verifying, "complete");
  assert.equal(stepStateByLabel(ladder)["Needs attention"], "current");
});

test("non-implementation tasks do not render the execution ladder", () => {
  const ladder = getTaskExecutionLadder({
    ...implementationTask(),
    title: "Verify stale decisions",
    prompt: "Review evidence coverage gaps.",
    status: "completed",
  });

  assert.equal(ladder, null);
});
