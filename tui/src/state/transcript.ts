// L3: Transcript Model — pure.
// Messages[] → TranscriptEntry[].
// Does NOT guess rendered heights. Does NOT wrap text.
// Just transforms the data shape for the component layer.

import type { ChatMessage, ToolCall } from "../protocol/types.js"

export type TranscriptEntry =
  | { type: "userPrompt"; id: string; text: string }
  | { type: "assistantText"; id: string; text: string; streaming: boolean }
  | { type: "thinking"; id: string; lines: string[]; hiddenCount: number }
  | { type: "assistantToolBatch"; id: string; tools: ToolCall[] }
  | { type: "indicator"; id: string; model: string }
  | { type: "error"; id: string; message: string }

export interface TranscriptOptions {
  messages: ChatMessage[]
  streaming: boolean
  streamingMsgId: string | null
  thinkExpanded: boolean
  error: string | null
  model: string
}

// Build transcript from messages. Each entry is one renderable block.
// The component layer (L4) decides how to render and how tall each block is.
export function buildTranscript(opts: TranscriptOptions): TranscriptEntry[] {
  const entries: TranscriptEntry[] = []

  for (const msg of opts.messages) {
    if (msg.role === "user") {
      entries.push({ type: "userPrompt", id: `${msg.id}-user`, text: msg.text })
      continue
    }

    // Assistant message
    if (msg.thinking) {
      const allLines = msg.thinking.split("\n")
      const maxVisible = 5
      const visible = opts.thinkExpanded ? allLines : allLines.slice(-maxVisible)
      const hidden = opts.thinkExpanded ? 0 : Math.max(0, allLines.length - maxVisible)
      entries.push({ type: "thinking", id: `${msg.id}-thinking`, lines: visible, hiddenCount: hidden })
    }

    if (msg.text) {
      entries.push({
        type: "assistantText",
        id: `${msg.id}-text`,
        text: msg.text,
        streaming: msg.id === opts.streamingMsgId,
      })
    }

    if (msg.tools?.length) {
      entries.push({
        type: "assistantToolBatch",
        id: `${msg.id}-tools`,
        tools: msg.tools,
      })
    }
  }

  // Thinking indicator when streaming with no content yet
  const streamingMsg = opts.streamingMsgId
    ? opts.messages.find((m) => m.id === opts.streamingMsgId)
    : null
  const hasContent = streamingMsg && (streamingMsg.text || (streamingMsg.tools && streamingMsg.tools.length > 0))
  if (opts.streaming && !hasContent) {
    entries.push({ type: "indicator", id: "thinking-indicator", model: opts.model })
  }

  if (opts.error) {
    entries.push({ type: "error", id: "error", message: opts.error })
  }

  return entries
}
