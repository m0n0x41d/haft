// L4: Chat viewport — renders transcript entries from L3.
// Scroll clipping done by entry slicing from L2.
// Does NOT guess heights. Ink handles layout.

import React from "react"
import { Box, Text, type DOMElement } from "ink"
import type { TranscriptEntry } from "../state/transcript.js"
import { MarkdownView } from "./MarkdownView.js"
import { AssistantToolBatchView } from "./AssistantToolBatchView.js"
import { ThinkingIndicator } from "./ThinkingIndicator.js"
import { buildUserPromptDisplayLines } from "./userPrompt.js"

const BLACK_CIRCLE = process.platform === "darwin" ? "\u23FA" : "\u25CF"

interface Props {
  entries: readonly TranscriptEntry[]
  width: number
  toolHistoryExpanded: boolean
  measureRef?: (entryId: string) => (node: DOMElement | null) => void
}

export function ChatView({ entries, width, toolHistoryExpanded, measureRef }: Props) {
  return (
    <Box flexDirection="column" flexShrink={0} width={width}>
      {entries.map((entry) => (
        <Box key={entry.id} flexDirection="column" flexShrink={0} width={width} ref={measureRef?.(entry.id)}>
          <EntryBlock entry={entry} width={width} toolHistoryExpanded={toolHistoryExpanded} />
        </Box>
      ))}
    </Box>
  )
}

const EntryBlock = React.memo(function EntryBlock({
  entry,
  width,
  toolHistoryExpanded,
}: {
  entry: TranscriptEntry
  width: number
  toolHistoryExpanded: boolean
}) {
  switch (entry.type) {
    case "userPrompt":
      return <UserPromptBlock text={entry.text} width={width} />
    case "assistantText":
      return <AssistantTextBlock text={entry.text} streaming={entry.streaming} width={width} />
    case "thinking":
      return <ThinkingBlock lines={entry.lines} hiddenCount={entry.hiddenCount} width={width} />
    case "assistantToolBatch":
      return <AssistantToolBatchView tools={entry.tools} width={width} expanded={toolHistoryExpanded} />
    case "indicator":
      return <ThinkingIndicator model={entry.model} />
    case "error":
      return <ErrorBlock message={entry.message} width={width} />
  }
})

// --- Entry renderers ---

function UserPromptBlock({ text, width }: { text: string; width: number }) {
  const lines = buildUserPromptDisplayLines(text)

  return (
    <Box flexDirection="column" marginTop={1} flexShrink={0} width={width}>
      {lines.map((line, index) => (
        <Box key={index} width={width}>
          <Text backgroundColor="blackBright" bold wrap="wrap">
            {line}
          </Text>
        </Box>
      ))}
    </Box>
  )
}

function AssistantTextBlock({ text, streaming, width }: { text: string; streaming: boolean; width: number }) {
  const contentWidth = Math.min(width - 4, 120)

  return (
    <Box flexDirection="row" marginTop={1} paddingX={1} flexShrink={0} width={width}>
      <Box flexShrink={0} minWidth={2}>
        <Text>{BLACK_CIRCLE}</Text>
      </Box>
      <Box flexDirection="column" flexShrink={1} flexGrow={1}>
        <MarkdownView text={text} width={contentWidth} />
        {streaming && <Text dimColor>{"\u2588"}</Text>}
      </Box>
    </Box>
  )
}

function ThinkingBlock({ lines, hiddenCount, width }: { lines: string[]; hiddenCount: number; width: number }) {
  return (
    <Box flexDirection="column" marginLeft={3} flexShrink={0} width={Math.max(0, width - 3)}>
      {hiddenCount > 0 && (
        <Text dimColor>{"... ("}{hiddenCount}{" lines hidden, press t to expand)"}</Text>
      )}
      {lines.map((line, i) => (
        <Text key={i} dimColor><Text color="gray">{"\u2503"}</Text> {line}</Text>
      ))}
    </Box>
  )
}

function ErrorBlock({ message, width }: { message: string; width: number }) {
  return (
    <Box flexDirection="column" borderStyle="round" borderColor="red" paddingX={1} marginTop={1} flexShrink={0} width={width}>
      <Text color="red" bold>Error</Text>
      <Text color="red">{message}</Text>
      <Text dimColor>press esc to dismiss</Text>
    </Box>
  )
}
