import assert from "node:assert/strict";
import { test } from "node:test";

import { governorWidgetLines, reportToolStatus, updateCockpitWidget } from "../extensions/haft/cockpit.ts";
import type { PiUI } from "../extensions/haft/pi-api.ts";

const GOVERNOR_SAMPLE = [
  "## Haft Project State (governor)",
  "",
  "Overseer: 41 signal(s), 18 high, 35 suppressed",
  "Decisions: 4 pending, 3 unassessed; 6 refresh-due, 28 drifted",
  "",
  "Open method runs:",
  `- \`mpull-x\` — ${"very long task description ".repeat(8)}`,
  "- `mpull-y` — second run",
  "",
  "Stale or drifted decisions above are evidence debt."
].join("\n");

test("governorWidgetLines picks counts and first open method run, clipped", () => {
  const lines = governorWidgetLines(GOVERNOR_SAMPLE);

  assert.equal(lines.length, 3);
  assert.equal(lines[0], "Overseer: 41 signal(s), 18 high, 35 suppressed");
  assert.equal(lines[1], "Decisions: 4 pending, 3 unassessed; 6 refresh-due, 28 drifted");
  assert.match(lines[2] ?? "", /^Open method run: `mpull-x`/);
  assert.ok((lines[2] ?? "").length <= 100);
  assert.match(lines[2] ?? "", /…$/);
});

test("governorWidgetLines returns nothing for non-governor text", () => {
  assert.deepEqual(governorWidgetLines("Haft status could not be loaded."), []);
});

test("updateCockpitWidget and reportToolStatus tolerate absent ui surfaces", () => {
  updateCockpitWidget(undefined, GOVERNOR_SAMPLE);
  updateCockpitWidget({}, GOVERNOR_SAMPLE);
  reportToolStatus(undefined, "haft_query", "status", true);
  reportToolStatus({}, "haft_query", undefined, false);

  const calls: Array<[string, string | string[]]> = [];
  const ui: PiUI = {
    setStatus: (key, text) => calls.push([key, text]),
    setWidget: (key, lines) => calls.push([key, lines])
  };

  updateCockpitWidget(ui, GOVERNOR_SAMPLE);
  reportToolStatus(ui, "haft_method", "pull", true);

  assert.equal(calls.length, 2);
  assert.deepEqual(calls[1], ["haft", "haft haft_method(pull) ✓"]);
});
