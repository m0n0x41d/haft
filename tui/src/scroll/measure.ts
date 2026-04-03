// L2: Scroll Measurement — pure.
// Bridges transcript entries, measured row heights, and the virtualized viewport.

import type { ToolCall } from "../protocol/types.js"
import type { TranscriptEntry } from "../state/transcript.js"

export const DEFAULT_OVERSCAN_ROWS = 8

// Approximate terminal-row height of a single transcript entry.
// Used until the mounted entry has a measured Yoga height.
export function estimateEntryHeight(entry: TranscriptEntry, width: number): number {
  switch (entry.type) {
    case "userPrompt":
      return 1 + 1 + entry.attachments.length

    case "assistantText": {
      const contentWidth = Math.max(1, Math.min(width - 4, 120))
      return 1 + countWrappedLines(entry.text, contentWidth)
    }

    case "thinking":
      return (entry.hiddenCount > 0 ? 1 : 0) + Math.max(1, entry.lines.length)

    case "assistantToolBatch":
      return entry.tools.reduce((sum, tool) => sum + measureToolCall(tool), 0)

    case "indicator":
      return 2

    case "error":
      return 6
  }
}

function measureToolCall(tool: ToolCall): number {
  let height = 2 // marginTop(1) + header(1)
  const summary = tool.subagent?.summary ?? tool.output

  if (summary && !tool.running) {
    height += 1
  }
  if (tool.output && tool.running) {
    height += Math.min(3, tool.output.split("\n").length)
  }
  if (tool.subagent?.tools.length) {
    if (tool.subagent.tools.length > 5) {
      height += 1
    }
    height += Math.min(5, tool.subagent.tools.length)
  }

  return height
}

// Count terminal rows for text, accounting for line wrapping at width boundary.
function countWrappedLines(text: string, width: number): number {
  if (!text) return 1
  return text
    .split("\n")
    .reduce((sum, line) => sum + (line.length === 0 ? 1 : Math.ceil(line.length / width)), 0)
}

// Resolve per-entry heights using measured values when available and width-based
// estimates as the fallback before the first layout pass completes.
export function resolveEntryHeights(
  entries: readonly TranscriptEntry[],
  width: number,
  measuredHeights: ReadonlyMap<string, number>,
): number[] {
  return entries.map((entry) => measuredHeights.get(entry.id) ?? estimateEntryHeight(entry, width))
}

export function scaleMeasuredHeights(
  measuredHeights: Map<string, number>,
  prevWidth: number,
  nextWidth: number,
): boolean {
  if (prevWidth <= 0 || nextWidth <= 0 || prevWidth === nextWidth) {
    return false
  }

  const ratio = prevWidth / nextWidth
  let changed = false

  for (const [entryId, height] of measuredHeights) {
    const scaledHeight = height === 0
      ? 0
      : Math.max(1, Math.round(height * ratio))

    if (scaledHeight === height) {
      continue
    }

    measuredHeights.set(entryId, scaledHeight)
    changed = true
  }

  return changed
}

export function computeOffsets(heights: readonly number[]): number[] {
  const offsets = new Array<number>(heights.length + 1)
  let linePos = 0

  offsets[0] = 0

  for (let index = 0; index < heights.length; index++) {
    linePos += heights[index] ?? 0
    offsets[index + 1] = linePos
  }

  return offsets
}

export interface VisibleWindow {
  start: number
  end: number
  viewTop: number
  viewBottom: number
  topSpacer: number
  bottomSpacer: number
  totalLines: number
}

export function computeVisibleWindow(
  offsets: readonly number[],
  offset: number,
  viewportSize: number,
  overscanRows: number = DEFAULT_OVERSCAN_ROWS,
): VisibleWindow {
  const entryCount = Math.max(0, offsets.length - 1)

  if (entryCount === 0) {
    return {
      start: 0,
      end: 0,
      viewTop: 0,
      viewBottom: 0,
      topSpacer: 0,
      bottomSpacer: 0,
      totalLines: 0,
    }
  }

  const totalLines = offsets[entryCount] ?? 0
  const safeOffset = Math.max(0, Math.min(offset, Math.max(0, totalLines - viewportSize)))
  const viewBottom = totalLines - safeOffset
  const viewTop = Math.max(0, viewBottom - viewportSize)
  const mountedTop = Math.max(0, viewTop - overscanRows)
  const mountedBottom = Math.min(totalLines, viewBottom + overscanRows)
  const start = findEntryIndexByBottom(offsets, mountedTop)
  const end = Math.max(start + 1, findEntryIndexByTop(offsets, mountedBottom))

  return {
    start,
    end,
    viewTop,
    viewBottom,
    topSpacer: offsets[start] ?? 0,
    bottomSpacer: totalLines - (offsets[end] ?? totalLines),
    totalLines,
  }
}

export function findEntryIndexForLine(
  offsets: readonly number[],
  line: number,
): number | null {
  const entryCount = Math.max(0, offsets.length - 1)
  const totalLines = offsets[entryCount] ?? 0

  if (entryCount === 0 || line < 0 || line >= totalLines) {
    return null
  }

  let lo = 0
  let hi = entryCount

  while (lo < hi) {
    const mid = (lo + hi) >> 1
    const nextTop = offsets[mid + 1] ?? totalLines

    if (nextTop <= line) {
      lo = mid + 1
      continue
    }

    hi = mid
  }

  return lo
}

function findEntryIndexByBottom(offsets: readonly number[], line: number): number {
  const entryCount = Math.max(0, offsets.length - 1)
  let lo = 1
  let hi = entryCount

  while (lo < hi) {
    const mid = (lo + hi) >> 1

    if ((offsets[mid] ?? 0) <= line) {
      lo = mid + 1
      continue
    }

    hi = mid
  }

  return Math.max(0, lo - 1)
}

function findEntryIndexByTop(offsets: readonly number[], line: number): number {
  const entryCount = Math.max(0, offsets.length - 1)
  let lo = 0
  let hi = entryCount

  while (lo < hi) {
    const mid = (lo + hi) >> 1

    if ((offsets[mid] ?? 0) < line) {
      lo = mid + 1
      continue
    }

    hi = mid
  }

  return lo
}
