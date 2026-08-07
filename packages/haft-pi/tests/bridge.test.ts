import assert from "node:assert/strict";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, test } from "node:test";

import { HaftBridge } from "../extensions/haft/bridge.ts";

const fakeHaft = join(dirname(fileURLToPath(import.meta.url)), "fake-haft.mjs");
const savedEnv = { HAFT_BIN: process.env.HAFT_BIN, FAKE_TOOLS: process.env.FAKE_TOOLS };
const bridges: HaftBridge[] = [];

function makeBridge(tools?: string): HaftBridge {
  process.env.HAFT_BIN = fakeHaft;
  if (tools !== undefined) {
    process.env.FAKE_TOOLS = tools;
  }

  const bridge = new HaftBridge(mkdtempSync(join(tmpdir(), "haft-pi-test-")));
  bridges.push(bridge);
  return bridge;
}

afterEach(() => {
  bridges.splice(0).forEach((bridge) => bridge.stop());
  process.env.HAFT_BIN = savedEnv.HAFT_BIN;
  process.env.FAKE_TOOLS = savedEnv.FAKE_TOOLS;
  if (savedEnv.HAFT_BIN === undefined) {
    delete process.env.HAFT_BIN;
  }
  if (savedEnv.FAKE_TOOLS === undefined) {
    delete process.env.FAKE_TOOLS;
  }
});

test("callTool returns text content from tools/call", async () => {
  const bridge = makeBridge("haft_query,haft_method");
  await bridge.start(undefined);

  const text = await bridge.callTool("haft_query", { action: "status" }, undefined);
  assert.equal(text, 'ok:haft_query:{"action":"status"}');
});

test("tool missing from tools/list gets an upgrade-pointing error", async () => {
  const bridge = makeBridge("haft_query");
  await bridge.start(undefined);

  await assert.rejects(
    bridge.callTool("haft_method", { action: "status" }, undefined),
    /not advertised by this haft binary.*Upgrade haft/s
  );
});

test("isError tool result surfaces as a thrown error with kernel text", async () => {
  const bridge = makeBridge("haft_query");
  await bridge.start(undefined);

  await assert.rejects(
    bridge.callTool("haft_query", { boom: true }, undefined),
    /kernel says no/
  );
});

test("concurrent starts share one initialize handshake", async () => {
  const bridge = makeBridge("haft_query");
  await Promise.all([bridge.start(undefined), bridge.start(undefined), bridge.start(undefined)]);

  // fake-haft exits with code 7 on a duplicate initialize, which would make
  // this call fail with a bridge-exited error.
  const text = await bridge.callTool("haft_query", { action: "status" }, undefined);
  assert.match(text, /^ok:haft_query:/);
});

test("missing haft binary rejects start and allows a retry", async () => {
  process.env.HAFT_BIN = "/nonexistent/haft-binary-zzz";
  const bridge = new HaftBridge(mkdtempSync(join(tmpdir(), "haft-pi-test-")));
  bridges.push(bridge);

  await assert.rejects(bridge.start(undefined), /ENOENT|spawn/);
  // A dead child must not be left registered: the retry spawns fresh and
  // fails the same honest way instead of "bridge is not running".
  await assert.rejects(bridge.start(undefined), /ENOENT|spawn/);
});
