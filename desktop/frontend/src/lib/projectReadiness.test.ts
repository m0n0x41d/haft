/// <reference types="node" />

import assert from "node:assert/strict";
import test from "node:test";

import { projectIsRunnable, projectReadiness } from "./projectReadiness.ts";

test("missing path dominates project readiness", () => {
  const readiness = projectReadiness({
    exists: false,
    has_haft: true,
    status: "ready",
  });

  assert.equal(readiness, "missing");
});

test("project without Haft config needs initialization", () => {
  const readiness = projectReadiness({
    exists: true,
    has_haft: false,
  });

  assert.equal(readiness, "needs_init");
});

test("only ready projects are runnable", () => {
  const states = [
    { exists: true, has_haft: true, status: "ready" as const },
    { exists: true, has_haft: false },
    { exists: false, has_haft: false },
  ];
  const runnable = states.map(projectIsRunnable);

  assert.deepEqual(runnable, [true, false, false]);
});
