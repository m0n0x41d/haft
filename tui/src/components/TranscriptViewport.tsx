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
    <Box flexDirection="column" flexShrink={0}>
      <TranscriptSpacer height={viewport.topSpacer} />
      <ChatView
        entries={entries}
        width={width}
        toolHistoryExpanded={toolHistoryExpanded}
        measureRef={measureRef}
      />
      <TranscriptSpacer height={viewport.bottomSpacer} />
    </Box>
  )
}

function TranscriptSpacer({ height }: { height: number }) {
  if (height <= 0) {
    return null
  }

  return <Box height={height} flexShrink={0} />
}
