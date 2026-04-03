import assert from "node:assert/strict"
import test from "node:test"
import type { TranscriptRow } from "../state/transcript.js"
import { buildTranscriptLayout, computeRenderWindow, computeVisibleWindow } from "./measure.js"
import { getOffsetFromBottom, initialScroll, reduceScroll, resolveViewportTop, type ScrollCommand } from "./state.js"

test("preserves the anchored viewport when new tail rows stream in", () => {
  const initialRows = makeTranscriptRows(8)
  const initialLayout = buildTranscriptLayout(initialRows, 24)
  let viewport = reduceScroll(initialScroll(6), { type: "contentChanged" }, initialLayout)

  viewport = reduceScroll(viewport, { type: "pageUp" }, initialLayout)

  const anchorRowId = viewport.anchorRowId
  const viewportTop = resolveViewportTop(viewport, initialLayout)
  const initialOffsetFromBottom = getOffsetFromBottom(viewport, initialLayout)
  const streamedRows = initialRows.concat(makeTranscriptRows(3, initialRows.length))
  const streamedLayout = buildTranscriptLayout(streamedRows, 24, initialLayout.heightCache)
  const nextViewport = reduceScroll(viewport, { type: "contentChanged" }, streamedLayout)
  const nextViewportTop = resolveViewportTop(nextViewport, streamedLayout)
  const nextOffsetFromBottom = getOffsetFromBottom(nextViewport, streamedLayout)
  const renderWindow = computeRenderWindow(
    streamedLayout,
    nextViewportTop,
    nextViewport.viewportSize,
    nextViewport.pinnedToBottom,
  )

  assert.equal(nextViewport.anchorRowId, anchorRowId)
  assert.equal(nextViewportTop, viewportTop)
  assert.equal(nextViewport.pinnedToBottom, false)
  assert.equal(nextOffsetFromBottom, initialOffsetFromBottom + (streamedLayout.totalHeight - initialLayout.totalHeight))
  assert.ok(renderWindow.end > renderWindow.start)
})

test("visible window excludes overscan rows and paints only the active viewport", () => {
  const rows = makeUniformTranscriptRows(8)
  const layout = buildTranscriptLayout(rows, 24)
  const renderWindow = computeRenderWindow(layout, 4, 4, false)
  const visibleWindow = computeVisibleWindow(layout, renderWindow, 4)

  assert.equal(renderWindow.start, 0)
  assert.equal(renderWindow.end, 7)
  assert.equal(visibleWindow.start, 2)
  assert.equal(visibleWindow.end, 4)
  assert.equal(visibleWindow.cropTop, 0)
  assert.equal(visibleWindow.contentHeight, 4)
  assert.equal(visibleWindow.paddingBottom, 0)
})

test("keeps the painted viewport rows stable while new tail rows stream in", () => {
  const initialRows = makeTranscriptRows(10)
  const initialLayout = buildTranscriptLayout(initialRows, 24)
  let viewport = reduceScroll(initialScroll(6), { type: "contentChanged" }, initialLayout)

  viewport = reduceScroll(viewport, { type: "pageUp" }, initialLayout)

  const initialVisibleWindow = computeVisibleWindow(
    initialLayout,
    computeRenderWindow(initialLayout, resolveViewportTop(viewport, initialLayout), viewport.viewportSize, viewport.pinnedToBottom),
    viewport.viewportSize,
  )
  const initialRowIds = initialRows
    .slice(initialVisibleWindow.start, initialVisibleWindow.end)
    .map((row) => row.id)
  const streamedRows = initialRows.concat(makeTranscriptRows(3, initialRows.length))
  const streamedLayout = buildTranscriptLayout(streamedRows, 24, initialLayout.heightCache)
  const nextViewport = reduceScroll(viewport, { type: "contentChanged" }, streamedLayout)
  const nextVisibleWindow = computeVisibleWindow(
    streamedLayout,
    computeRenderWindow(streamedLayout, resolveViewportTop(nextViewport, streamedLayout), nextViewport.viewportSize, nextViewport.pinnedToBottom),
    nextViewport.viewportSize,
  )
  const nextRowIds = streamedRows
    .slice(nextVisibleWindow.start, nextVisibleWindow.end)
    .map((row) => row.id)

  assert.deepEqual(nextRowIds, initialRowIds)
})

test("invalidates cached row heights when the width changes", () => {
  const rows = [assistantTextRow("assistant-width", "x".repeat(48))]
  const wideLayout = buildTranscriptLayout(rows, 40)
  const narrowLayout = buildTranscriptLayout(rows, 12, wideLayout.heightCache)
  const narrowLayoutAgain = buildTranscriptLayout(rows, 12, narrowLayout.heightCache)

  assert.ok(narrowLayout.heights[0] > wideLayout.heights[0])
  assert.equal(narrowLayoutAgain.heights[0], narrowLayout.heights[0])
  assert.equal(narrowLayout.heightCache.width, 12)
})

test("line and page scrolling keep the same absolute viewport position without drift", () => {
  const rows = makeTranscriptRows(20)
  const layout = buildTranscriptLayout(rows, 24)
  const commands: ScrollCommand[] = [
    { type: "wheelUp", amount: 2 },
    { type: "wheelUp", amount: 5 },
    { type: "pageUp" },
    { type: "wheelDown", amount: 3 },
    { type: "pageDown" },
    { type: "home" },
    { type: "wheelDown", amount: 4 },
    { type: "pageDown" },
    { type: "end" },
  ]
  let viewport = reduceScroll(initialScroll(7), { type: "contentChanged" }, layout)
  let expectedTop = Math.max(0, layout.totalHeight - 7)

  for (const command of commands) {
    viewport = reduceScroll(viewport, command, layout)
    expectedTop = applyExpectedViewportTop(expectedTop, command, layout.totalHeight, viewport.viewportSize)
    assert.equal(resolveViewportTop(viewport, layout), expectedTop)
  }
})

function makeTranscriptRows(count: number, startIndex: number = 0): TranscriptRow[] {
  return Array.from({ length: count }, (_, offset) => {
    const index = startIndex + offset
    const lineLength = 12 + (index % 4) * 10

    return assistantTextRow(`assistant-${index}`, "x".repeat(lineLength))
  })
}

function makeUniformTranscriptRows(count: number): TranscriptRow[] {
  return Array.from({ length: count }, (_, index) => {
    return assistantTextRow(`uniform-${index}`, "x")
  })
}

function assistantTextRow(id: string, text: string): TranscriptRow {
  const measureToken = text
    .split("\n")
    .map((line) => line.length)
    .join(",")

  return {
    type: "assistantText",
    id,
    text,
    streaming: false,
    measureKey: ["assistantText", "static", measureToken].join(":"),
  }
}

function applyExpectedViewportTop(
  currentTop: number,
  command: ScrollCommand,
  totalHeight: number,
  viewportSize: number,
): number {
  const maxTop = Math.max(0, totalHeight - viewportSize)

  switch (command.type) {
    case "wheelUp":
      return clampLine(currentTop - (command.amount ?? 3), 0, maxTop)

    case "wheelDown":
      return clampLine(currentTop + (command.amount ?? 3), 0, maxTop)

    case "pageUp":
      return clampLine(currentTop - viewportSize, 0, maxTop)

    case "pageDown":
      return clampLine(currentTop + viewportSize, 0, maxTop)

    case "home":
      return 0

    case "end":
      return maxTop

    case "contentChanged":
      return clampLine(currentTop, 0, maxTop)

    case "resize":
      return clampLine(currentTop, 0, Math.max(0, totalHeight - command.viewportSize))
  }
}

function clampLine(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(value, max))
}
