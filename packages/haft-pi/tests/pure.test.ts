import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";

import { parseResponse, toolNames, toolText } from "../extensions/haft/bridge.ts";
import { clipText, withGovernorHeader } from "../extensions/haft/governor.ts";
import { findHaftProjectRoot } from "../extensions/haft/project.ts";
import { HAFT_TOOLS } from "../extensions/haft/tools.ts";

test("clipText passes short text through and clips long text with a marker", () => {
  assert.equal(clipText("short", 100), "short");

  const clipped = clipText("x".repeat(300), 100);
  assert.ok(clipped.length < 300);
  assert.match(clipped, /\[truncated by @haft\/pi prompt governor\]$/);
});

test("withGovernorHeader keeps kernel headers and wraps raw text", () => {
  const governor = "## Haft Project State (governor)\nbody";
  assert.equal(withGovernorHeader(governor), governor);

  assert.equal(withGovernorHeader("raw status"), "## Haft Project State\nraw status");
});

test("parseResponse tolerates non-JSON lines", () => {
  assert.equal(parseResponse("not json"), undefined);
  assert.deepEqual(parseResponse('{"jsonrpc":"2.0","id":1,"result":{}}'), {
    jsonrpc: "2.0",
    id: 1,
    result: {}
  });
});

test("toolNames extracts names defensively", () => {
  assert.deepEqual(toolNames({ tools: [{ name: "a" }, {}, { name: "b" }] }), ["a", "b"]);
  assert.deepEqual(toolNames({}), []);
});

test("toolText joins text content and throws on isError", () => {
  const ok = { content: [{ type: "text", text: "one" }, { type: "image" }, { type: "text", text: "two" }] };
  assert.equal(toolText(ok), "one\ntwo");

  assert.throws(() => toolText({ content: [{ type: "text", text: "bad" }], isError: true }), /bad/);
  assert.throws(() => toolText({ isError: true }), /returned an error/);
});

test("findHaftProjectRoot walks up to the nearest .haft", () => {
  const root = mkdtempSync(join(tmpdir(), "haft-pi-root-"));
  const project = join(root, "repo");
  const nested = join(project, "a", "b");
  mkdirSync(join(project, ".haft"), { recursive: true });
  mkdirSync(nested, { recursive: true });

  assert.equal(findHaftProjectRoot(nested), project);
  assert.equal(findHaftProjectRoot(root), undefined);
});

test("Pi tool schemas mirror maintenance, prediction, and problem dimension fields", () => {
  const refresh = toolSpec("haft_refresh");
  const decision = toolSpec("haft_decision");
  const problem = toolSpec("haft_problem");

  assert.match(JSON.stringify(refresh.parameters), /"plan"/);
  assert.match(JSON.stringify(decision.parameters), /"command"/);
  assert.match(JSON.stringify(problem.parameters), /"proxy_for"/);
});

test("Pi haft_query schema mirrors current read-only drill-down actions", () => {
  const query = toolSpec("haft_query");
  const schema = JSON.stringify(query.parameters);

  [
    "carrier_manifest",
    "carrier_check",
    "contract_audit",
    "contract_generation",
    "spec_review",
    "spec_use",
    "change_case",
    "correspondence_graph",
    "drift_route",
    "drift_events",
    "decision_reconcile",
    "governing_set",
    "blocked_use",
    "value_space",
    "evidence_path"
  ].forEach((action) => assert.match(schema, new RegExp(`"${action}"`)));

  [
    "section_id",
    "operational_gate",
    "source_refs",
    "requires_current_formality",
    "bearer_ref",
    "method_ref",
    "lane"
  ].forEach((field) => assert.match(schema, new RegExp(`"${field}"`)));
});

test("Pi haft_refresh schema mirrors review and drain actions", () => {
  const refresh = toolSpec("haft_refresh");
  const schema = JSON.stringify(refresh.parameters);

  ["review", "drain", "dry_run"].forEach((fragment) => {
    assert.match(schema, new RegExp(`"${fragment}"`));
  });
});

function toolSpec(name: string) {
  const found = HAFT_TOOLS.find((tool) => tool.name === name);
  assert.ok(found, `missing tool spec ${name}`);
  return found;
}
