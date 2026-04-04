import { strict as assert } from "node:assert"
import { test } from "node:test"
import {
  currentText,
  emptyHistory,
  navigateDown,
  navigateUp,
  push,
} from "./history.js"

test("restores multiline prompts from history without flattening lines", () => {
  const history = push(emptyHistory, "first line\nsecond line")
  const browsed = navigateUp(history, "")

  assert.ok(browsed)
  assert.equal(currentText(browsed), "first line\nsecond line")
})

test("restores a multiline draft after browsing older prompts", () => {
  const history = push(
    push(emptyHistory, "saved prompt"),
    "older line one\nolder line two",
  )
  const draft = "draft line one\ndraft line two"
  const browsed = navigateUp(history, draft)

  assert.ok(browsed)
  assert.equal(currentText(browsed), "older line one\nolder line two")

  const returned = navigateDown(browsed)

  assert.ok(returned)
  assert.equal(currentText(returned), draft)
})
