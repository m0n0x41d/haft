import { strict as assert } from "node:assert"
import { test } from "node:test"
import {
  buildInputDisplayLayout,
  buildInputLayout,
  measureInputDisplayRows,
  measureInputRows,
} from "./inputLayout.js"

test("wraps a long single-line prompt by available content width", () => {
  const layout = buildInputLayout("abcdefghij", 10, 10)

  assert.equal(layout.contentWidth, 6)
  assert.deepEqual(
    layout.rows.map((row) => row.text),
    ["abcdef", "ghij"],
  )
  assert.equal(layout.rows[0]?.prefix, "\u276F ")
  assert.equal(layout.rows[1]?.prefix, "  ")
  assert.equal(layout.rows[1]?.cursorColumn, 4)
})

test("wraps multiline pasted text across logical and visual rows", () => {
  const layout = buildInputLayout("abcdefghi\nxyz", 13, 8)

  assert.deepEqual(
    layout.rows.map((row) => row.text),
    ["abcd", "efgh", "i", "xyz"],
  )
  assert.equal(layout.rows[3]?.cursorColumn, 3)
})

test("moves the cursor to the next wrapped row at a soft-wrap boundary", () => {
  const layout = buildInputLayout("abcdefghij", 4, 8)

  assert.equal(layout.rows[0]?.cursorColumn, null)
  assert.equal(layout.rows[1]?.cursorColumn, 0)
  assert.equal(layout.rows[2]?.cursorColumn, null)
})

test("keeps the cursor visible after an exact-width wrapped line", () => {
  const rows = measureInputRows("abcd", 4, 8)
  const layout = buildInputLayout("abcd", 4, 8)

  assert.equal(rows, 2)
  assert.equal(layout.rows[1]?.text, "")
  assert.equal(layout.rows[1]?.cursorColumn, 0)
})

test("wraps the empty-input queued hint into measured prompt rows", () => {
  const rows = measureInputDisplayRows({
    text: "",
    cursor: 0,
    width: 12,
    hasQueuedMessages: true,
  })
  const layout = buildInputDisplayLayout({
    text: "",
    cursor: 0,
    width: 12,
    hasQueuedMessages: true,
  })

  assert.equal(rows, 5)
  assert.deepEqual(
    layout.rows.map((row) => row.kind),
    ["placeholder", "hint", "hint", "hint", "hint"],
  )
})
