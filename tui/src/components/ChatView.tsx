// L4: Chat viewport — renders transcript entries from L3.
// Scroll clipping done by entry slicing from L2.
// Does NOT guess heights. Ink handles layout.

import React from "react"
import { Box, Text } from "ink"
import type { TranscriptEntry } from "../state/transcript.js"
import { MarkdownView } from "./MarkdownView.js"
import { AssistantToolBatchView } from "./AssistantToolBatchView.js"
import { ThinkingIndicator } from "./ThinkingIndicator.js"

const BLACK_CIRCLE = process.platform === "darwin" ? "\u23FA" : "\u25CF"

interface Props {
  entries: TranscriptEntry[]
  width: number
}

export function ChatView({ entries, width }: Props) {
  return (
    <Box flexDirection="column">
      {entries.map((entry) => (
        <EntryBlock key={entry.id} entry={entry} width={width} />
      ))}
    </Box>
  )
}

const EntryBlock = React.memo(function EntryBlock({ entry, width }: { entry: TranscriptEntry; width: number }) {
  switch (entry.type) {
    case "userPrompt":
      return <UserPromptBlock text={entry.text} attachments={entry.attachments} width={width} />
    case "assistantText":
      return <AssistantTextBlock text={entry.text} streaming={entry.streaming} width={width} />
    case "thinking":
      return <ThinkingBlock lines={entry.lines} hiddenCount={entry.hiddenCount} />
    case "assistantToolBatch":
      return <AssistantToolBatchView tools={entry.tools} width={width} />
    case "indicator":
      return <ThinkingIndicator model={entry.model} />
    case "error":
      return <ErrorBlock message={entry.message} />
  }
})

// --- Entry renderers ---

function UserPromptBlock({ text, attachments, width }: { text: string; attachments: string[]; width: number }) {
  const content = ` \u276F ${text}`
  const pad = Math.max(0, width - content.length)

  return (
    <Box flexDirection="column" marginTop={1}>
      <Box>
        <Text backgroundColor="blackBright">
          <Text dimColor>{" \u276F"}</Text> <Text bold>{text}</Text>{" ".repeat(pad)}
        </Text>
      </Box>
      {attachments.map((line, i) => (
        <Box key={i} paddingX={1}>
          <Text dimColor>{"\u21B3  "}{line}</Text>
        </Box>
      ))}
    </Box>
  )
}

function AssistantTextBlock({ text, streaming, width }: { text: string; streaming: boolean; width: number }) {
  const contentWidth = Math.min(width - 4, 120)

  return (
    <Box flexDirection="row" marginTop={1} paddingX={1}>
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

function ThinkingBlock({ lines, hiddenCount }: { lines: string[]; hiddenCount: number }) {
  return (
    <Box flexDirection="column" marginLeft={3}>
      {hiddenCount > 0 && (
        <Text dimColor>{"... ("}{hiddenCount}{" lines hidden, press t to expand)"}</Text>
      )}
      {lines.map((line, i) => (
        <Text key={i} dimColor><Text color="gray">{"\u2503"}</Text> {line}</Text>
      ))}
    </Box>
  )
}

function ErrorBlock({ message }: { message: string }) {
  return (
    <Box flexDirection="column" borderStyle="round" borderColor="red" paddingX={1} marginTop={1}>
      <Text color="red" bold>Error</Text>
      <Text color="red">{message}</Text>
      <Text dimColor>press esc to dismiss</Text>
    </Box>
  )
}
