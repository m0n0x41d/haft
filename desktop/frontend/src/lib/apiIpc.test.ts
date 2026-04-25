/// <reference types="node" />

import assert from "node:assert/strict";
import test from "node:test";

import { projectRootIpcArgs } from "./api.ts";

test("spec check uses Tauri camelCase IPC argument shape", () => {
  const args = projectRootIpcArgs("/tmp/haft-product");

  assert.deepEqual(args, {
    projectRoot: "/tmp/haft-product",
  });
  assert.equal("project_root" in args, false);
});
