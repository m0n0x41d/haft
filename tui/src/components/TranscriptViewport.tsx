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
  return (
    <Box flexDirection="column" marginTop={-viewport.cropTop} flexShrink={0} width={width}>
      <ChatView
        entries={entries}
        width={width}
        toolHistoryExpanded={toolHistoryExpanded}
        measureRef={measureRef}
      />
    </Box>
  )
}
