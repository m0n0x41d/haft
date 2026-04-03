import { strict as assert } from "node:assert"
import { test } from "node:test"
import type { TranscriptEntry } from "../state/transcript.js"
import {
  computeOffsets,
  computeVisibleWindow,
  findEntryIndexForLine,
  resolveEntryHeights,
  scaleMeasuredHeights,
} from "./measure.js"

test("computes a measured virtual window with overscan spacers", () => {
  const heights = [4, 6, 5, 3]
  const offsets = computeOffsets(heights)
  const window = computeVisibleWindow(offsets, 0, 8, 2)

  assert.deepEqual(window, {
    start: 1,
    end: 4,
    viewTop: 10,
    viewBottom: 18,
    topSpacer: 4,
    bottomSpacer: 0,
    totalLines: 18,
  })
})

test("falls back to estimated heights until an entry has been measured", () => {
  const entries: TranscriptEntry[] = [
    {
      type: "assistantText",
      id: "assistant-1",
      text: "alpha\nbeta",
      streaming: false,
    },
    {
      type: "indicator",
      id: "indicator-1",
      model: "gpt-test",
    },
  ]
  const measuredHeights = new Map<string, number>([["assistant-1", 9]])
  const heights = resolveEntryHeights(entries, 80, measuredHeights)

  assert.deepEqual(heights, [9, 2])
})

test("rescales cached measurements for terminal width changes", () => {
  const measuredHeights = new Map<string, number>([
    ["assistant-1", 8],
    ["indicator-1", 0],
  ])
  const changed = scaleMeasuredHeights(measuredHeights, 80, 40)

  assert.equal(changed, true)
  assert.deepEqual(Array.from(measuredHeights.entries()), [
    ["assistant-1", 16],
    ["indicator-1", 0],
  ])
})

test("maps absolute transcript lines back to entry indexes", () => {
  const offsets = computeOffsets([3, 4, 2])

  assert.equal(findEntryIndexForLine(offsets, 0), 0)
  assert.equal(findEntryIndexForLine(offsets, 2), 0)
  assert.equal(findEntryIndexForLine(offsets, 3), 1)
  assert.equal(findEntryIndexForLine(offsets, 6), 1)
  assert.equal(findEntryIndexForLine(offsets, 7), 2)
  assert.equal(findEntryIndexForLine(offsets, 9), null)
})
