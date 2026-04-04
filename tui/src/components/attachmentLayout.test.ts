import { strict as assert } from "node:assert"
import { test } from "node:test"
import {
  buildAttachmentRows,
  clampAttachmentCursor,
  moveAttachmentCursor,
} from "./attachmentLayout.js"

test("wraps multiple attachment chips into visible rows", () => {
  const rows = buildAttachmentRows({
    items: [
      { id: 1, name: "clip-1.png", path: "/tmp/clip-1.png", isImage: true },
      { id: 2, name: "clip-2.png", path: "/tmp/clip-2.png", isImage: true },
      { id: 3, name: "clip-3.png", path: "/tmp/clip-3.png", isImage: true },
    ],
    selectionMode: false,
    selectedIndex: 0,
    width: 24,
  })

  assert.deepEqual(rows, [
    {
      type: "items",
      items: [
        { id: 1, label: "[Image #1]", selected: false },
        { id: 2, label: "[Image #2]", selected: false },
      ],
    },
    {
      type: "items",
      items: [
        { id: 3, label: "[Image #3]", selected: false },
      ],
    },
    {
      type: "hint",
      text: "(↑ to select)",
    },
  ])
})

test("keeps attachment selection cursor in range after navigation and removal", () => {
  assert.equal(moveAttachmentCursor(0, "right", 3), 1)
  assert.equal(moveAttachmentCursor(2, "right", 3), 2)
  assert.equal(moveAttachmentCursor(1, "left", 3), 0)
  assert.equal(clampAttachmentCursor(2, 2), 1)
  assert.equal(clampAttachmentCursor(0, 0), 0)
})
