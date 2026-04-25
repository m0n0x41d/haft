/// <reference types="node" />

import assert from "node:assert/strict";
import test from "node:test";

import {
  projectActivationLabel,
  projectIsMissing,
  projectIsRunnable,
  projectNeedsInitialization,
  projectNeedsOnboarding,
  projectReadiness,
  projectReadinessBadgeLabel,
  projectTaskBlockedTitle,
} from "./projectReadiness.ts";

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

test("initialized project without spec set needs onboarding", () => {
  const readiness = projectReadiness({
    exists: true,
    has_haft: true,
    has_specs: false,
  });

  assert.equal(readiness, "needs_onboard");
});

test("only ready projects are runnable", () => {
  const states = [
    { exists: true, has_haft: true, has_specs: true, status: "ready" as const },
    { exists: true, has_haft: true, has_specs: false },
    { exists: true, has_haft: false },
    { exists: false, has_haft: false },
  ];
  const runnable = states.map(projectIsRunnable);

  assert.deepEqual(runnable, [true, false, false, false]);
});

test("readiness facts prevent fake ready projects", () => {
  const states = [
    {
      exists: true,
      has_haft: false,
      has_specs: true,
      status: "ready" as const,
    },
    {
      exists: true,
      has_haft: true,
      has_specs: false,
      status: "ready" as const,
    },
    {
      exists: false,
      has_haft: true,
      has_specs: true,
      status: "ready" as const,
    },
  ];
  const readiness = states.map(projectReadiness);
  const runnable = states.map(projectIsRunnable);

  assert.deepEqual(readiness, ["needs_init", "needs_onboard", "missing"]);
  assert.deepEqual(runnable, [false, false, false]);
});

test("readiness helpers expose semantic UI states", () => {
  const missing = { exists: false, has_haft: false };
  const needsInit = { exists: true, has_haft: false };
  const needsOnboard = { exists: true, has_haft: true, has_specs: false };

  assert.equal(projectIsMissing(missing), true);
  assert.equal(projectNeedsInitialization(needsInit), true);
  assert.equal(projectNeedsOnboarding(needsOnboard), true);
  assert.equal(projectReadinessBadgeLabel(needsOnboard), "onboard");
  assert.equal(projectActivationLabel(needsOnboard), "Onboard");
  assert.equal(projectTaskBlockedTitle(needsOnboard), "Project needs onboarding before generic tasks");
});
