import { strict as assert } from "node:assert"
import { test } from "node:test"
import {
  currentEntry,
  currentText,
  emptyHistory,
  navigateDown,
  navigateUp,
  push,
} from "./history.js"

test("restores multiline prompts from history without flattening lines", () => {
  const pushed = push(emptyHistory, "first line\nsecond line")
  const history = pushed.history
  const browsed = navigateUp(history, "")

  assert.ok(browsed)
  assert.equal(currentText(browsed), "first line\nsecond line")
  assert.equal(currentEntry(browsed)?.text, "first line\nsecond line")
})

test("restores a multiline draft after browsing older prompts", () => {
  const saved = push(emptyHistory, "saved prompt")
  const older = push(
    saved.history,
    "older line one\nolder line two",
  )
  const history = older.history
  const draft = "draft line one\ndraft line two"
  const browsed = navigateUp(history, draft)

  assert.ok(browsed)
  assert.equal(currentText(browsed), "older line one\nolder line two")

  const returned = navigateDown(browsed)

  assert.ok(returned)
  assert.equal(currentText(returned), draft)
})

test("reports evicted entries when history exceeds the cap", () => {
  let history = emptyHistory
  let firstStoredId: number | null = null
  let lastResult = push(history, "seed")

  history = lastResult.history
  firstStoredId = lastResult.stored?.id ?? null

  for (let index = 0; index < 100; index++) {
    lastResult = push(history, `prompt ${index}`)
    history = lastResult.history
  }

  assert.equal(lastResult.evicted?.id, firstStoredId)
  assert.equal(history.entries.length, 100)
  assert.equal(history.entries[0]?.text, "prompt 0")
})
