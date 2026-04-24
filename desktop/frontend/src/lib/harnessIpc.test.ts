/// <reference types="node" />

import assert from "node:assert/strict";
import test from "node:test";

import { commissionIpcArgs, listCommissionsIpcArgs } from "./harnessIpc.ts";

test("list commissions uses Tauri camelCase IPC arguments", () => {
  const args = listCommissionsIpcArgs("open");

  assert.deepEqual(args, {
    selector: "open",
    state: "",
    olderThan: "",
  });
  assert.equal("older_than" in args, false);
});

test("commission id uses Tauri camelCase IPC argument", () => {
  const args = commissionIpcArgs("wc-1");

  assert.deepEqual(args, {
    commissionId: "wc-1",
  });
  assert.equal("commission_id" in args, false);
});
