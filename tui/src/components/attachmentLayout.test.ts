import { strict as assert } from "node:assert"
import { test } from "node:test"
import {
  buildAttachmentRows,
  clampAttachmentCursor,
  moveAttachmentCursor,
  estimateAttachmentRows,
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

test("counts wrapped rows for a single long attachment label", () => {
  const rows = estimateAttachmentRows({
    items: [
      {
        id: 1,
        name: "averyverylongattachmentname.txt",
        path: "/tmp/averyverylongattachmentname.txt",
        isImage: false,
      },
    ],
    selectionMode: false,
    width: 14,
  })

  assert.equal(rows, 3)
})

test("truncates an image chip before packing on a narrow terminal", () => {
  const rows = buildAttachmentRows({
    items: [
      { id: 1, name: "clip.png", path: "/tmp/clip.png", isImage: true },
    ],
    selectionMode: false,
    selectedIndex: 0,
    width: 8,
  })

  assert.deepEqual(rows, [
    {
      type: "items",
      items: [
        { id: 1, label: "[Ima…]", selected: false },
      ],
    },
    { type: "hint", text: "(↑ to " },
    { type: "hint", text: "select" },
    { type: "hint", text: ")" },
  ])
})

test("truncates long filenames before packing attachment rows", () => {
  const rows = buildAttachmentRows({
    items: [
      {
        id: 1,
        name: "averyverylongattachmentname.txt",
        path: "/tmp/averyverylongattachmentname.txt",
        isImage: false,
      },
    ],
    selectionMode: false,
    selectedIndex: 0,
    width: 12,
  })

  assert.deepEqual(rows, [
    {
      type: "items",
      items: [
        { id: 1, label: "[averyve…]", selected: false },
      ],
    },
    { type: "hint", text: "(↑ to sele" },
    { type: "hint", text: "ct)" },
  ])
})
