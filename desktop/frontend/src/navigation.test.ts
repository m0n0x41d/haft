/// <reference types="node" />

import assert from "node:assert/strict";
import test from "node:test";

import { getPageTitle, normalizePage, resolveNavigation } from "./navigation.ts";

test("legacy reasoning pages redirect to dashboard", () => {
  const legacyPages = ["problems", "decisions"] as const;
  const redirectedPages = legacyPages.map(normalizePage);

  assert.deepEqual(redirectedPages, ["dashboard", "dashboard"]);
});

test("redirected pages drop stale selected ids", () => {
  const target = resolveNavigation("decisions", "dec-123");

  assert.deepEqual(target, {
    page: "dashboard",
    selectedId: null,
  });
});

test("non-legacy pages keep their selection and title", () => {
  const target = resolveNavigation("tasks", "task-123");
  const title = getPageTitle("dashboard");

  assert.deepEqual(target, {
    page: "tasks",
    selectedId: "task-123",
  });
  assert.equal(title, "Dashboard");
});
