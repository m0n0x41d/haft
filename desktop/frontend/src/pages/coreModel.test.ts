/// <reference types="node" />

import assert from "node:assert/strict";
import test from "node:test";

import {
  buildCoreAttention,
  buildCoreRuntimeItems,
  commissionPhase,
  type CoreAttentionInput,
} from "./coreModel.ts";

const EMPTY_INPUT: CoreAttentionInput = {
  overview: {
    last_scan_at: "",
    coverage: {
      total_modules: 0,
      covered_count: 0,
      partial_count: 0,
      blind_count: 0,
      governed_percent: 0,
      last_scanned: "",
      modules: [],
    },
    findings: [],
    problem_candidates: [],
  },
  tasks: [],
  commissions: [],
};

test("core attention treats blocked commissions as operator work", () => {
  const items = buildCoreAttention({
    ...EMPTY_INPUT,
    commissions: [
      {
        id: "wc-1",
        state: "blocked_policy",
        decision_ref: "dec-1",
        problem_card_ref: "prob-1",
      },
    ],
  });

  assert.equal(items.length, 1);
  assert.equal(items[0].kind, "runtime");
  assert.equal(items[0].tone, "danger");
  assert.equal(items[0].action, "open_runtime");
});

test("core attention does not surface completed commissions", () => {
  const items = buildCoreRuntimeItems([
    {
      id: "wc-1",
      state: "completed",
      decision_ref: "dec-1",
      problem_card_ref: "prob-1",
      operator: { terminal: true },
    },
  ]);

  assert.equal(items.length, 0);
});

test("core attention normalizes task status before choosing tone", () => {
  const items = buildCoreAttention({
    ...EMPTY_INPUT,
    tasks: [
      {
        id: "task-1",
        title: "Investigate failed run",
        agent: "codex",
        project: "haft",
        project_path: "/repo/haft",
        status: " FAILED ",
        prompt: "Fix the failing run",
        branch: "",
        worktree: false,
        worktree_path: "",
        reused_worktree: false,
        started_at: "2026-04-24T00:00:00Z",
        completed_at: "",
        error_message: "agent exited with code 1",
        output: "",
        chat_blocks: [],
        raw_output: "",
        auto_run: false,
      },
    ],
  });

  assert.equal(items.length, 1);
  assert.equal(items[0].kind, "conversation");
  assert.equal(items[0].tone, "danger");
  assert.equal(items[0].title, "Conversation failed");
});

test("commission phase is normalized from runtime state names", () => {
  assert.equal(
    commissionPhase({
      id: "wc-1",
      state: "preflighting",
      decision_ref: "dec-1",
      problem_card_ref: "prob-1",
    }),
    "preflight",
  );
  assert.equal(
    commissionPhase({
      id: "wc-2",
      state: "blocked_policy",
      decision_ref: "dec-1",
      problem_card_ref: "prob-1",
    }),
    "blocked",
  );
});
