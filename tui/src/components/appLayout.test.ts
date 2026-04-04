import { strict as assert } from "node:assert"
import { test } from "node:test"
import {
  computeBottomRows,
  computeChatHeight,
  estimateInputRows,
  estimateQueuedMessageRows,
} from "./appLayout.js"
import { estimateAttachmentRows } from "./attachmentLayout.js"
import { measureInputDisplayRows } from "./inputLayout.js"

test("keeps the legacy four-row footprint for an empty prompt", () => {
  const bottomRows = computeBottomRows({
    width: 80,
    queuedMessages: [],
    attachments: [],
    attachmentSelection: false,
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
    attachmentSelection: false,
    inputRows: estimateInputRows("alpha\nbeta\ngamma"),
    showInput: true,
  })

  assert.equal(bottomRows, 10)
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
    attachmentSelection: false,
    inputRows: estimateInputRows("alpha\nbeta"),
    showInput: false,
  })

  assert.equal(bottomRows, 3)
})

test("collapses the transcript before keeping a stale minimum height", () => {
  assert.equal(computeChatHeight(6, 8), 0)
})

test("reserves rows for a pasted image plus multiline prompt text", () => {
  const bottomRows = computeBottomRows({
    width: 32,
    queuedMessages: [],
    attachments: [
      { id: 1, name: "clip.png", path: "/tmp/clip.png", isImage: true },
    ],
    attachmentSelection: false,
    inputRows: estimateInputRows("first line\nsecond line\nthird line"),
    showInput: true,
  })

  assert.equal(bottomRows, 8)
})

test("reserves wrapped attachment rows before the prompt for multiple images", () => {
  const rows = estimateAttachmentRows({
    items: [
      { id: 1, name: "clip-1.png", path: "/tmp/clip-1.png", isImage: true },
      { id: 2, name: "clip-2.png", path: "/tmp/clip-2.png", isImage: true },
      { id: 3, name: "clip-3.png", path: "/tmp/clip-3.png", isImage: true },
    ],
    selectionMode: true,
    width: 18,
  })

  assert.equal(rows, 6)
})

test("includes wrapped empty-input queue hints in the bottom budget", () => {
  const inputRows = measureInputDisplayRows({
    text: "",
    cursor: 0,
    width: 12,
    hasQueuedMessages: true,
  })
  const bottomRows = computeBottomRows({
    width: 12,
    queuedMessages: ["queued"],
    attachments: [],
    attachmentSelection: false,
    inputRows,
    showInput: true,
  })

  assert.equal(inputRows, 5)
  assert.equal(bottomRows, 9)
})
