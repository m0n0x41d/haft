// L2: Scroll Measurement — pure.
// Estimates terminal-row height for transcript rows.
// Bridges data model (TranscriptRow) and viewport (terminal rows).

import type { TranscriptRow } from "../state/transcript.js"

const MIN_ROW_HEIGHT = 1
const MIN_OVERSCAN_LINES = 6

export interface TranscriptHeightCache {
  width: number
  heightsByRowId: ReadonlyMap<string, number>
  measureKeysByRowId: ReadonlyMap<string, string>
}

export interface TranscriptLayout {
  rows: readonly TranscriptRow[]
  heights: readonly number[]
  offsets: readonly number[]
  totalHeight: number
  width: number
  heightCache: TranscriptHeightCache
  rowIndexById: ReadonlyMap<string, number>
}

export interface RenderWindow {
  start: number
  end: number
  cropTop: number
  paddingTop: number
  viewportTop: number
  viewportBottom: number
  totalHeight: number
}

export interface VisibleWindow {
  start: number
  end: number
  cropTop: number
  paddingTop: number
  paddingBottom: number
  contentHeight: number
  viewportTop: number
  viewportBottom: number
  totalHeight: number
}

export function createTranscriptHeightCache(): TranscriptHeightCache {
  return {
    width: 0,
    heightsByRowId: new Map(),
    measureKeysByRowId: new Map(),
  }
}

// Approximate terminal-row height of a single transcript row.
// Matches the rendering logic in ChatView / ToolCallView / etc.
// Deliberately conservative — a few lines off is fine, overflowY="hidden" clips the rest.
export function measureRow(row: TranscriptRow, width: number): number {
  switch (row.type) {
    case "userPrompt":
      return 1 + 1 + row.attachments.length

    case "assistantText": {
      const contentWidth = Math.max(1, Math.min(width - 4, 120))
      const wrappedLines = countWrappedLines(row.text, contentWidth)
      const streamingCursor = row.streaming ? 1 : 0

      return 1 + wrappedLines + streamingCursor
    }

    case "thinking":
      return (row.hiddenCount > 0 ? 1 : 0) + Math.max(1, row.lines.length)

    case "toolCall": {
      const childCount = row.tool.children?.length ?? 0
      const visibleChildren = Math.min(5, childCount)
      const hiddenChildren = childCount > 5 ? 1 : 0
      const outputLines = row.tool.output?.split("\n").length ?? 0
      let height = 2

      if (row.tool.output && !row.tool.running) {
        height += 1
      }

      if (row.tool.output && row.tool.running) {
        height += Math.min(3, outputLines)
      }

      return height + visibleChildren + hiddenChildren
    }

    case "indicator":
      return 2

    case "error":
      return 6
  }
}

export const measureEntry = measureRow

export function measureTranscript(rows: readonly TranscriptRow[], width: number): number[] {
  return rows.map((row) => measureRow(row, width))
}

export function buildTranscriptLayout(
  rows: readonly TranscriptRow[],
  width: number,
  previousCache: TranscriptHeightCache = createTranscriptHeightCache(),
): TranscriptLayout {
  const canReuseHeights = previousCache.width === width
  const heightsByRowId = new Map<string, number>()
  const measureKeysByRowId = new Map<string, string>()
  const heights = rows.map((row) => {
    const cachedMeasureKey = canReuseHeights ? previousCache.measureKeysByRowId.get(row.id) : undefined
    const cachedHeight = canReuseHeights ? previousCache.heightsByRowId.get(row.id) : undefined
    const height = cachedMeasureKey === row.measureKey && cachedHeight !== undefined
      ? cachedHeight
      : measureRow(row, width)

    heightsByRowId.set(row.id, height)
    measureKeysByRowId.set(row.id, row.measureKey)

    return height
  })
  const offsets = buildOffsets(heights)
  const rowIndexById = new Map(rows.map((row, index) => [row.id, index]))
  const totalHeight = offsets[offsets.length - 1] ?? 0
  const heightCache = {
    width,
    heightsByRowId,
    measureKeysByRowId,
  }

  return {
    rows,
    heights,
    offsets,
    totalHeight,
    width,
    heightCache,
    rowIndexById,
  }
}

export function computeRenderWindow(
  layout: TranscriptLayout,
  viewportTop: number,
  viewportSize: number,
  pinnedToBottom: boolean,
  overscanLines: number = getOverscanLines(viewportSize),
): RenderWindow {
  if (layout.rows.length === 0) {
    return {
      start: 0,
      end: 0,
      cropTop: 0,
      paddingTop: 0,
      viewportTop: 0,
      viewportBottom: 0,
      totalHeight: 0,
    }
  }

  const clampedViewportTop = clampLine(viewportTop, 0, Math.max(0, layout.totalHeight - viewportSize))
  const viewportBottom = Math.min(layout.totalHeight, clampedViewportTop + viewportSize)
  const windowTop = Math.max(0, clampedViewportTop - overscanLines)
  const windowBottom = Math.min(layout.totalHeight, viewportBottom + overscanLines)
  const start = findRowIndexForLine(layout.offsets, windowTop)
  const end = findWindowEnd(layout.offsets, windowBottom)
  const cropTop = Math.max(0, clampedViewportTop - layout.offsets[start])
  const visibleHeight = Math.max(0, viewportBottom - clampedViewportTop)
  const paddingTop = pinnedToBottom ? Math.max(0, viewportSize - visibleHeight) : 0

  return {
    start,
    end,
    cropTop,
    paddingTop,
    viewportTop: clampedViewportTop,
    viewportBottom,
    totalHeight: layout.totalHeight,
  }
}

export function computeVisibleWindow(
  layout: TranscriptLayout,
  renderWindow: RenderWindow,
  viewportSize: number,
): VisibleWindow {
  if (layout.rows.length === 0) {
    return {
      start: 0,
      end: 0,
      cropTop: 0,
      paddingTop: renderWindow.paddingTop,
      paddingBottom: Math.max(0, viewportSize - renderWindow.paddingTop),
      contentHeight: 0,
      viewportTop: renderWindow.viewportTop,
      viewportBottom: renderWindow.viewportBottom,
      totalHeight: renderWindow.totalHeight,
    }
  }

  const start = findRowIndexForLine(layout.offsets, renderWindow.viewportTop)
  const end = findWindowEnd(layout.offsets, renderWindow.viewportBottom)
  const cropTop = Math.max(0, renderWindow.viewportTop - layout.offsets[start])
  const contentHeight = Math.max(0, renderWindow.viewportBottom - renderWindow.viewportTop)
  const paddingBottom = Math.max(0, viewportSize - renderWindow.paddingTop - contentHeight)

  return {
    start,
    end,
    cropTop,
    paddingTop: renderWindow.paddingTop,
    paddingBottom,
    contentHeight,
    viewportTop: renderWindow.viewportTop,
    viewportBottom: renderWindow.viewportBottom,
    totalHeight: renderWindow.totalHeight,
  }
}

function countWrappedLines(text: string, width: number): number {
  if (!text) {
    return 1
  }

  return text
    .split("\n")
    .reduce((sum, line) => sum + (line.length === 0 ? 1 : Math.ceil(line.length / width)), 0)
}

function buildOffsets(heights: readonly number[]): number[] {
  const offsets = new Array(heights.length + 1)

  offsets[0] = 0

  for (let index = 0; index < heights.length; index++) {
    offsets[index + 1] = offsets[index] + Math.max(MIN_ROW_HEIGHT, heights[index] ?? MIN_ROW_HEIGHT)
  }

  return offsets
}

function findWindowEnd(offsets: readonly number[], windowBottom: number): number {
  const rowCount = Math.max(0, offsets.length - 1)

  for (let index = 0; index < rowCount; index++) {
    if (offsets[index] >= windowBottom) {
      return index
    }
  }

  return rowCount
}

function findRowIndexForLine(offsets: readonly number[], line: number): number {
  const rowCount = Math.max(0, offsets.length - 1)

  if (rowCount === 0) {
    return 0
  }

  let low = 0
  let high = rowCount - 1

  while (low <= high) {
    const mid = Math.floor((low + high) / 2)
    const rowTop = offsets[mid]
    const rowBottom = offsets[mid + 1]

    if (line < rowTop) {
      high = mid - 1
      continue
    }

    if (line >= rowBottom) {
      low = mid + 1
      continue
    }

    return mid
  }

  return Math.max(0, Math.min(low, rowCount - 1))
}

function getOverscanLines(viewportSize: number): number {
  return Math.max(MIN_OVERSCAN_LINES, Math.floor(viewportSize / 2))
}

function clampLine(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(value, max))
}
