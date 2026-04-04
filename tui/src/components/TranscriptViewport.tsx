import React from "react"
import { Box, type DOMElement } from "ink"
import type { VisibleWindow } from "../scroll/measure.js"
import type { TranscriptEntry } from "../state/transcript.js"
import { ChatView } from "./ChatView.js"

interface TranscriptViewportProps {
  entries: readonly TranscriptEntry[]
  measureRef: (entryId: string) => (node: DOMElement | null) => void
  viewport: VisibleWindow
  toolHistoryExpanded: boolean
  width: number
}

export function TranscriptViewport({
  entries,
  measureRef,
  viewport,
  toolHistoryExpanded,
  width,
}: TranscriptViewportProps) {
  // Only crop the overscanned rows above the mounted slice. Shifting by the
  // full absolute transcript offset creates giant offscreen margins that Ink
  // can compress oddly while live tool batches keep growing.
  const cropTop = Math.max(0, viewport.viewTop - viewport.topSpacer)

  return (
    <Box flexDirection="column" marginTop={-cropTop} flexShrink={0}>
      <ChatView
        entries={entries}
        width={width}
        toolHistoryExpanded={toolHistoryExpanded}
        measureRef={measureRef}
      />
    </Box>
  )
}
