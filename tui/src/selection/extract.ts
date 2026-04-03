// L2: Text Extraction — pure.
// Maps terminal row selections to transcript entry text.

import type { ToolCall } from "../protocol/types.js"
import type { TranscriptEntry } from "../state/transcript.js"
import { findEntryIndexForLine, type VisibleWindow } from "../scroll/measure.js"

export interface ViewportLayout {
  chatHeight: number
  atBottom: boolean
  visibleWindow: VisibleWindow
  entryHeights: readonly number[]
  entryOffsets: readonly number[]
  transcript: readonly TranscriptEntry[]
}

export function termRowToEntryIndex(
  termRow: number,
  layout: ViewportLayout,
): number | null {
  const { visibleWindow: vw, entryOffsets, atBottom, chatHeight } = layout
  const chatRow = termRow - 1 // 0-based within chat area

  if (chatRow < 0 || chatRow >= chatHeight) return null

  const visibleLineCount = Math.max(0, vw.viewBottom - vw.viewTop)
  const topPadding = atBottom ? Math.max(0, chatHeight - visibleLineCount) : 0
  const contentRow = chatRow - topPadding

  if (contentRow < 0 || contentRow >= visibleLineCount) {
    return null
  }

  return findEntryIndexForLine(entryOffsets, vw.viewTop + contentRow)
}

// Extract readable text from a transcript entry.
function entryText(entry: TranscriptEntry): string {
  switch (entry.type) {
    case "userPrompt": return entry.text
    case "assistantText": return entry.text
    case "thinking": return entry.lines.join("\n")
    case "assistantToolBatch":
      return entry.tools
        .map((tool) => toolText(tool))
        .filter((text) => text.length > 0)
        .join("\n\n")
    case "indicator": return ""
    case "error": return entry.message
  }
}

function toolText(tool: ToolCall): string {
  const parts = [tool.name]
  const summary = tool.subagent?.summary ?? tool.output

  if (summary) {
    parts.push(summary)
  }

  const childText = tool.subagent?.tools
    .map((child) => toolText(child))
    .filter((text) => text.length > 0)
    .join("\n")

  if (childText) {
    parts.push(childText)
  }

  return parts.join("\n")
}

// Extract text from entries overlapping the selection row range.
// startRow/endRow: normalized (start <= end), 1-based SGR coordinates.
export function extractSelection(
  startRow: number,
  endRow: number,
  layout: ViewportLayout,
): string {
  const startIdx = termRowToEntryIndex(startRow, layout)
  const endIdx = termRowToEntryIndex(endRow, layout)

  if (startIdx === null && endIdx === null) return ""

  const lo = Math.min(startIdx ?? endIdx!, endIdx ?? startIdx!)
  const hi = Math.max(startIdx ?? endIdx!, endIdx ?? startIdx!)

  const parts: string[] = []
  for (let i = lo; i <= hi; i++) {
    if (i >= 0 && i < layout.transcript.length) {
      const text = entryText(layout.transcript[i])
      if (text) parts.push(text)
    }
  }

  return parts.join("\n\n")
}
