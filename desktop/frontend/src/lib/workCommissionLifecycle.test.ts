/// <reference types="node" />

import assert from "node:assert/strict";
import test from "node:test";

import {
  isExecutingWorkCommissionState,
  isRecoverableWorkCommissionState,
  isRunnableWorkCommissionState,
  isTerminalWorkCommissionState,
  satisfiesDependencyWorkCommissionState,
} from "./workCommissionLifecycle.ts";

test("work commission lifecycle semantics stay centralized", () => {
  const cases = [
    {
      state: "failed",
      terminal: false,
      recoverable: true,
      runnable: false,
      executing: false,
      satisfiesDependency: false,
    },
    {
      state: "cancelled",
      terminal: true,
      recoverable: false,
      runnable: false,
      executing: false,
      satisfiesDependency: false,
    },
    {
      state: "expired",
      terminal: true,
      recoverable: false,
      runnable: false,
      executing: false,
      satisfiesDependency: false,
    },
    {
      state: "completed",
      terminal: true,
      recoverable: false,
      runnable: false,
      executing: false,
      satisfiesDependency: true,
    },
    {
      state: "completed_with_projection_debt",
      terminal: true,
      recoverable: false,
      runnable: false,
      executing: false,
      satisfiesDependency: true,
    },
  ];

  const observed = cases.map((item) => ({
    state: item.state,
    terminal: isTerminalWorkCommissionState(item.state),
    recoverable: isRecoverableWorkCommissionState(item.state),
    runnable: isRunnableWorkCommissionState(item.state),
    executing: isExecutingWorkCommissionState(item.state),
    satisfiesDependency: satisfiesDependencyWorkCommissionState(item.state),
  }));

  assert.deepEqual(observed, cases);
});
