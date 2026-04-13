import { strict as assert } from "node:assert"
import { test } from "node:test"
import { shouldFinalizeStreaming } from "./streamLifecycle.js"

test("treats permission and question overlays as stream-owned phases", () => {
  assert.equal(shouldFinalizeStreaming("streaming", false), true)
  assert.equal(shouldFinalizeStreaming("permission", false), true)
  assert.equal(shouldFinalizeStreaming("question", false), true)
})

test("still finalizes when a buffered update exists outside a stream-owned phase", () => {
  assert.equal(shouldFinalizeStreaming("input", true), true)
  assert.equal(shouldFinalizeStreaming("input", false), false)
})
