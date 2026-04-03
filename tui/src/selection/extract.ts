// L2: Text Extraction — pure.
// Maps terminal row selections to transcript entry text.

import type { TranscriptEntry } from "../state/transcript.js"
import type { VisibleWindow } from "../scroll/measure.js"

export interface ViewportLayout {
  chatHeight: number
  atBottom: boolean
  visibleWindow: VisibleWindow
  entryHeights: readonly number[]
  transcript: readonly TranscriptEntry[]
}

// Map a terminal row (1-based SGR) to a visible entry index.
// Returns null if the row falls outside content (padding or below chat area).
export function termRowToEntryIndex(
  termRow: number,
  layout: ViewportLayout,
): number | null {
  const { visibleWindow: vw, entryHeights, atBottom, chatHeight } = layout
  const chatRow = termRow - 1 // 0-based within chat area

  if (chatRow < 0 || chatRow >= chatHeight) return null

  let totalVisible = 0
  for (let i = vw.start; i < vw.end; i++) totalVisible += entryHeights[i]

  // Account for cropTop: the first entry is shifted up by cropTop lines
  const effectiveVisible = totalVisible - vw.cropTop
  const padding = atBottom ? Math.max(0, chatHeight - effectiveVisible) : 0
  // Map terminal row to absolute line within the visible entries
  // cropTop offsets into the first entry (its first cropTop lines are above the viewport)
  const contentRow = chatRow - padding + vw.cropTop

  if (contentRow < 0) return null

  let cumHeight = 0
  for (let i = vw.start; i < vw.end; i++) {
    if (contentRow < cumHeight + entryHeights[i]) return i
    cumHeight += entryHeights[i]
  }

  return null
}

// Extract readable text from a transcript entry.
function entryText(entry: TranscriptEntry): string {
  switch (entry.type) {
    case "userPrompt": return entry.text
    case "assistantText": return entry.text
    case "thinking": return entry.lines.join("\n")
    case "toolCall": {
      const parts = [entry.tool.name]
      if (entry.tool.output) parts.push(entry.tool.output)
      return parts.join("\n")
    }
    case "indicator": return ""
    case "error": return entry.message
  }
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
