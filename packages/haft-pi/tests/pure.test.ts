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

function toolSpec(name: string) {
  const found = HAFT_TOOLS.find((tool) => tool.name === name);
  assert.ok(found, `missing tool spec ${name}`);
  return found;
}
