import assert from "node:assert/strict";
import test from "node:test";

import { buildRecentActivity } from "../src/pages/dashboardActivity.ts";

test("buildRecentActivity merges problems and decisions by recency", () => {
  const items = buildRecentActivity(
    [
      {
        id: "prob-1",
        title: "Investigate stale governance scan",
        status: "active",
        mode: "standard",
        signal: "The dashboard has not refreshed governance data in three days.",
        reversibility: "high",
        constraints: [],
        created_at: "2026-04-10T09:00:00Z",
      },
    ],
    [
      {
        id: "dec-1",
        title: "Keep dashboard as landing page",
        status: "active",
        mode: "standard",
        selected_title: "Use a single operator dashboard",
        weakest_link: "Recent activity hides decisions if ordering is wrong",
        valid_until: "2026-05-01",
        created_at: "2026-04-12T11:00:00Z",
      },
      {
        id: "dec-2",
        title: "Archive legacy verify page",
        status: "active",
        mode: "tactical",
        selected_title: "Fold verify work into dashboard",
        weakest_link: "",
        valid_until: "",
        created_at: "2026-04-11T11:00:00Z",
      },
    ],
  );

  assert.deepEqual(
    items.map((item) => item.id),
    ["dec-1", "dec-2", "prob-1"],
  );
  assert.deepEqual(
    items.map((item) => item.page),
    ["decisions", "decisions", "problems"],
  );
  assert.equal(items[0]?.summary, "WLNK: Recent activity hides decisions if ordering is wrong");
  assert.equal(items[1]?.summary, "active");
});

test("buildRecentActivity caps the feed and sends invalid timestamps to the end", () => {
  const problems = Array.from({ length: 5 }, (_, index) => ({
    id: `prob-${index}`,
    title: `Problem ${index}`,
    status: "active",
    mode: "standard",
    signal: `Signal ${index}`,
    reversibility: "high",
    constraints: [],
    created_at: `2026-04-0${index + 1}T09:00:00Z`,
  }));
  const decisions = [
    {
      id: "dec-fresh",
      title: "Fresh decision",
      status: "active",
      mode: "standard",
      selected_title: "Fresh decision",
      weakest_link: "Needs follow-up",
      valid_until: "",
      created_at: "2026-04-12T11:00:00Z",
    },
    {
      id: "dec-invalid",
      title: "Invalid timestamp decision",
      status: "active",
      mode: "standard",
      selected_title: "Invalid timestamp decision",
      weakest_link: "Needs sort fallback",
      valid_until: "",
      created_at: "not-a-date",
    },
    {
      id: "dec-also-fresh",
      title: "Another recent decision",
      status: "active",
      mode: "standard",
      selected_title: "Another recent decision",
      weakest_link: "Needs recent feed coverage",
      valid_until: "",
      created_at: "2026-04-11T11:00:00Z",
    },
  ];

  const items = buildRecentActivity(problems, decisions);

  assert.equal(items.length, 8);
  assert.equal(items[0]?.id, "dec-fresh");
  assert.equal(items.at(-1)?.id, "dec-invalid");
});
