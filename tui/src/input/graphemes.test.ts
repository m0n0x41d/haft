import { strict as assert } from "node:assert"
import { test } from "node:test"
import {
  normalizeGraphemeBoundaryLeft,
  segmentGraphemes,
} from "./graphemes.js"

test("segments graphemes with stable offsets", () => {
  const graphemes = segmentGraphemes("A👍🏻B")

  assert.deepEqual(
    graphemes.map((grapheme) => ({
      text: grapheme.text,
      start: grapheme.start,
      end: grapheme.end,
    })),
    [
      { text: "A", start: 0, end: 1 },
      { text: "👍🏻", start: 1, end: 5 },
      { text: "B", start: 5, end: 6 },
    ],
  )
})

test("snaps cursor offsets to the left grapheme boundary", () => {
  assert.equal(normalizeGraphemeBoundaryLeft("👍🏻", 1), 0)
  assert.equal(normalizeGraphemeBoundaryLeft("👍🏻", 4), 4)
})
