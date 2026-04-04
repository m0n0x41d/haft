import { strict as assert } from "node:assert"
import { test } from "node:test"
import {
  deleteBack,
  deleteForward,
  cursorPosition,
  empty,
  insertAt,
  fromText,
  moveEnd,
  moveHome,
  moveLeft,
  moveRight,
} from "./editBuffer.js"

test("keeps Ctrl+J-style newline insertion as multiline text", () => {
  const firstLine = insertAt(empty, "first line")
  const secondLineStart = insertAt(firstLine, "\n")
  const fullPrompt = insertAt(secondLineStart, "second line")

  assert.equal(fullPrompt.text, "first line\nsecond line")
  assert.deepEqual(cursorPosition(fullPrompt), { line: 1, col: 11 })
})

test("moves home and end within the current logical line", () => {
  const prompt = {
    text: "alpha\nbeta\ngamma",
    cursor: 8,
  }

  assert.equal(moveHome(prompt).cursor, 6)
  assert.equal(moveEnd(prompt).cursor, 10)
})

test("moveLeft and deleteBack treat emoji clusters as one grapheme", () => {
  const family = "👨‍👩‍👧‍👦"
  const state = fromText(family)
  const moved = moveLeft(state)
  const deleted = deleteBack(state)

  assert.equal(moved.cursor, 0)
  assert.equal(deleted.text, "")
  assert.equal(deleted.cursor, 0)
})

test("moveRight and deleteForward treat emoji clusters as one grapheme", () => {
  const family = "👨‍👩‍👧‍👦"
  const state = { text: family, cursor: 0 }
  const moved = moveRight(state)
  const deleted = deleteForward(state)

  assert.equal(moved.cursor, family.length)
  assert.equal(deleted.text, "")
  assert.equal(deleted.cursor, 0)
})

test("cursorPosition reports terminal column width for wide glyphs", () => {
  const state = fromText("你你")

  assert.deepEqual(cursorPosition(state), { line: 0, col: 4 })
})
