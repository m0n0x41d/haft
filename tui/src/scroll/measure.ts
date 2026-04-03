// L2: Scroll Measurement — pure.
// Estimates terminal-row height for transcript entries.
// Bridges data model (TranscriptEntry) and viewport (terminal rows).

import type { TranscriptEntry } from "../state/transcript.js"

// Approximate terminal-row height of a single transcript entry.
// Matches the rendering logic in ChatView / ToolCallView / etc.
// Deliberately conservative — a few lines off is fine, overflowY="hidden" clips the rest.
export function measureEntry(entry: TranscriptEntry, width: number): number {
  switch (entry.type) {
    case "userPrompt":
      // marginTop(1) + prompt line(1) + attachment lines
      return 1 + 1 + entry.attachments.length

    case "assistantText": {
      const contentWidth = Math.max(1, Math.min(width - 4, 120))
      // marginTop(1) + wrapped text lines
      return 1 + countWrappedLines(entry.text, contentWidth)
    }

    case "thinking":
      // optional "... (N hidden)" line + visible lines
      return (entry.hiddenCount > 0 ? 1 : 0) + Math.max(1, entry.lines.length)

    case "toolCall": {
      const t = entry.tool
      let h = 2 // marginTop(1) + header(1)
      if (t.output && !t.running) h += 1
      if (t.output && t.running) h += Math.min(3, t.output.split("\n").length)
      if (t.children && t.children.length > 0) {
        if (t.children.length > 5) h += 1
        h += Math.min(5, t.children.length)
      }
      return h
    }

    case "indicator":
      return 2 // marginTop(1) + animation(1)

    case "error":
      return 6 // marginTop(1) + border-top(1) + "Error"(1) + message(1) + hint(1) + border-bottom(1)
  }
}

// Count terminal rows for text, accounting for line wrapping at width boundary.
function countWrappedLines(text: string, width: number): number {
  if (!text) return 1
  return text
    .split("\n")
    .reduce((sum, line) => sum + (line.length === 0 ? 1 : Math.ceil(line.length / width)), 0)
}

// Measure all entries in a transcript.
export function measureTranscript(entries: readonly TranscriptEntry[], width: number): number[] {
  return entries.map((e) => measureEntry(e, width))
}

// Visible window: which entries overlap the viewport given a line-based scroll offset.
export interface VisibleWindow {
  start: number   // first visible entry index (inclusive)
  end: number     // past-end entry index (exclusive)
  cropTop: number // lines to skip from the top of entry[start] (intra-entry offset)
}

// Compute the entry range visible in the viewport.
// offset = lines scrolled back from bottom (0 = at bottom).
// Returns entries that overlap the viewport, plus cropTop: the number of lines
// from the first visible entry that fall above the viewport top edge.
export function computeVisibleWindow(
  heights: readonly number[],
  offset: number,
  viewportSize: number,
): VisibleWindow {
  if (heights.length === 0) return { start: 0, end: 0, cropTop: 0 }

  const totalLines = heights.reduce((a, b) => a + b, 0)
  const safeOffset = Math.max(0, Math.min(offset, Math.max(0, totalLines - viewportSize)))

  // Viewport spans [viewTop, viewBottom) in absolute line coordinates
  const viewBottom = totalLines - safeOffset
  const viewTop = Math.max(0, viewBottom - viewportSize)

  let linePos = 0
  let start = -1
  let end = 0
  let cropTop = 0

  for (let i = 0; i < heights.length; i++) {
    const entryTop = linePos
    const entryBottom = linePos + heights[i]

    // Entry overlaps viewport when entryBottom > viewTop AND entryTop < viewBottom
    if (entryBottom > viewTop && entryTop < viewBottom) {
      if (start === -1) {
        start = i
        // How many lines of this first entry are above the viewport top?
        cropTop = Math.max(0, viewTop - entryTop)
      }
      end = i + 1
    }

    linePos = entryBottom
  }

  return start === -1 ? { start: 0, end: 0, cropTop: 0 } : { start, end, cropTop }
}
