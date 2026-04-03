import React, { useMemo } from "react"
import { Box, Transform, type DOMElement } from "ink"
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
  viewportHeight,
  width,
}: TranscriptViewportProps) {
  const transform = useMemo(
    () => createViewportTransform(viewport.viewTop, viewportHeight),
    [viewport.viewTop, viewportHeight],
  )

  return (
    <Transform transform={transform}>
      <Box flexDirection="column">
        <TranscriptSpacer height={viewport.topSpacer} />
        <ChatView entries={entries} width={width} measureRef={measureRef} />
        <TranscriptSpacer height={viewport.bottomSpacer} />
      </Box>
    </Transform>
  )
}

function TranscriptSpacer({ height }: { height: number }) {
  if (height <= 0) {
    return null
  }

  return <Box height={height} flexShrink={0} />
}

function createViewportTransform(startLine: number, viewportHeight: number) {
  return (output: string) => sliceRenderedLines(output, startLine, viewportHeight)
}

function sliceRenderedLines(output: string, startLine: number, viewportHeight: number): string {
  if (output.length === 0 || viewportHeight <= 0) {
    return ""
  }

  const lines = output.split("\n")
  const endLine = startLine + viewportHeight

  return lines.slice(startLine, endLine).join("\n")
}
