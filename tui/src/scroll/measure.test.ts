import { strict as assert } from "node:assert"
import { test } from "node:test"
import type { TranscriptEntry } from "../state/transcript.js"
import {
  computeOffsets,
  computeVisibleWindow,
  estimateEntryHeight,
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
    cropTop: 6,
    topSpacer: 4,
    bottomSpacer: 0,
    totalLines: 18,
  })
})

test("tracks a crop offset for non-zero scroll windows", () => {
  const heights = [4, 6, 5, 3]
  const offsets = computeOffsets(heights)
  const window = computeVisibleWindow(offsets, 3, 8, 2)

  assert.deepEqual(window, {
    start: 1,
    end: 4,
    viewTop: 7,
    viewBottom: 15,
    cropTop: 3,
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

test("estimates collapsed tool batches from their displayed rows", () => {
  const entry: TranscriptEntry = {
    type: "assistantToolBatch",
    id: "tools-1",
    tools: [
      {
        callId: "bash-1",
        name: "bash",
        args: "{\"command\":\"git status\"}",
        output: "ok",
        running: false,
      },
      {
        callId: "bash-2",
        name: "bash",
        args: "{\"command\":\"git diff --stat\"}",
        output: "ok",
        running: false,
      },
      {
        callId: "bash-3",
        name: "bash",
        args: "{\"command\":\"git log --oneline -3\"}",
        output: "ok",
        running: false,
      },
    ],
  }

  assert.equal(estimateEntryHeight(entry, 80), 3)
  assert.ok(estimateEntryHeight(entry, 80, { toolHistoryExpanded: true }) > 3)
})

test("estimates multiline user prompts from their full preserved text", () => {
  const entry: TranscriptEntry = {
    type: "userPrompt",
    id: "user-1",
    text: "first line\n[not an attachment]\nthird line",
    attachments: [],
  }

  assert.equal(estimateEntryHeight(entry, 80), 4)
})

test("counts structured attachments separately from the user prompt text", () => {
  const entry: TranscriptEntry = {
    type: "userPrompt",
    id: "user-1",
    text: "[not an attachment]",
    attachments: [
      { name: "clipboard.png", isImage: true },
      { name: "spec.md", isImage: false },
    ],
  }

  assert.equal(estimateEntryHeight(entry, 80), 4)
})

test("prefers fresh estimates while active transcript entries are still growing", () => {
  const entries: TranscriptEntry[] = [
    {
      type: "assistantText",
      id: "assistant-1",
      text: "alpha\nbeta\ngamma\ndelta",
      streaming: true,
    },
    {
      type: "assistantToolBatch",
      id: "tools-1",
      tools: [
        {
          callId: "bash-1",
          name: "bash",
          args: "{\"command\":\"git status\"}",
          output: "line 1\nline 2\nline 3",
          running: true,
        },
      ],
    },
  ]
  const measuredHeights = new Map<string, number>([
    ["assistant-1", 2],
    ["tools-1", 2],
  ])
  const heights = resolveEntryHeights(entries, 80, measuredHeights)

  assert.deepEqual(heights, [5, 5])
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
