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
  assert.equal(layout.rows[1]?.cursorOffset, 4)
})

test("wraps multiline pasted text across logical and visual rows", () => {
  const layout = buildInputLayout("abcdefghi\nxyz", 13, 8)

  assert.deepEqual(
    layout.rows.map((row) => row.text),
    ["abcd", "efgh", "i", "xyz"],
  )
  assert.equal(layout.rows[3]?.cursorOffset, 3)
})

test("moves the cursor to the next wrapped row at a soft-wrap boundary", () => {
  const layout = buildInputLayout("abcdefghij", 4, 8)

  assert.equal(layout.rows[0]?.cursorOffset, null)
  assert.equal(layout.rows[1]?.cursorOffset, 0)
  assert.equal(layout.rows[2]?.cursorOffset, null)
})

test("keeps the cursor visible after an exact-width wrapped line", () => {
  const rows = measureInputRows("abcd", 4, 8)
  const layout = buildInputLayout("abcd", 4, 8)

  assert.equal(rows, 2)
  assert.equal(layout.rows[1]?.text, "")
  assert.equal(layout.rows[1]?.cursorOffset, 0)
})

test("wraps wide CJK glyphs by terminal display width", () => {
  const layout = buildInputLayout("你你你你", 4, 8)

  assert.deepEqual(
    layout.rows.map((row) => row.text),
    ["你你", "你你", ""],
  )
  assert.equal(layout.rows[2]?.cursorOffset, 0)
})

test("keeps emoji rows on grapheme boundaries", () => {
  const family = "👨‍👩‍👧‍👦"
  const layout = buildInputLayout(`${family}${family}`, 1, 6)

  assert.deepEqual(
    layout.rows.map((row) => row.text),
    [family, family],
  )
  assert.equal(layout.rows[0]?.cursorOffset, 0)
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
