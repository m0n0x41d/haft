import { strict as assert } from "node:assert"
import { test } from "node:test"
import {
  computeBottomRows,
  computeChatHeight,
  estimateAttachmentRows,
  estimateInputRows,
  estimateQueuedMessageRows,
} from "./appLayout.js"

test("keeps the legacy four-row footprint for an empty prompt", () => {
  const bottomRows = computeBottomRows({
    width: 80,
    queuedMessages: [],
    attachments: [],
    inputRows: estimateInputRows(""),
    showInput: true,
  })

  assert.equal(bottomRows, 4)
})

test("adds queued rows, one attachment strip row, and multiline input rows", () => {
  const bottomRows = computeBottomRows({
    width: 80,
    queuedMessages: ["queued one", "queued two"],
    attachments: [
      { id: 1, name: "Image #1", path: "/tmp/one.png", isImage: true },
      { id: 2, name: "notes.md", path: "/tmp/notes.md", isImage: false },
    ],
    inputRows: estimateInputRows("alpha\nbeta\ngamma"),
    showInput: true,
  })

  assert.equal(bottomRows, 9)
})

test("wraps queued message rows to the available terminal width", () => {
  const queuedRows = estimateQueuedMessageRows(["123456789012"], 8)

  assert.equal(queuedRows, 3)
})

test("drops input rows when the prompt is hidden", () => {
  const bottomRows = computeBottomRows({
    width: 80,
    queuedMessages: [],
    attachments: [],
    inputRows: estimateInputRows("alpha\nbeta"),
    showInput: false,
  })

  assert.equal(bottomRows, 3)
})

test("collapses the transcript before keeping a stale minimum height", () => {
  assert.equal(computeChatHeight(6, 8), 0)
})

test("reports a single attachment strip row when attachments are present", () => {
  const rows = estimateAttachmentRows([
    { id: 1, name: "clip.png", path: "/tmp/clip.png", isImage: true },
  ])

  assert.equal(rows, 1)
})
