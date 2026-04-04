import React from "react"
import { Box, type DOMElement } from "ink"
import type { VisibleWindow } from "../scroll/measure.js"
import type { TranscriptEntry } from "../state/transcript.js"
import { ChatView } from "./ChatView.js"

interface TranscriptViewportProps {
  entries: readonly TranscriptEntry[]
  measureRef: (entryId: string) => (node: DOMElement | null) => void
  viewport: VisibleWindow
  viewportHeight: number
  width: number
}

export function TranscriptViewport({
  entries,
  measureRef,
  viewport,
  width,
}: TranscriptViewportProps) {
  const topOffset = viewport.viewTop === 0
    ? 0
    : -viewport.viewTop

  return (
    <Box flexDirection="column" marginTop={topOffset}>
      <TranscriptSpacer height={viewport.topSpacer} />
      <ChatView entries={entries} width={width} measureRef={measureRef} />
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
