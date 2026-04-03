// L3: Transcript Model — pure.
// Messages[] → TranscriptEntry[].
// Does NOT guess rendered heights. Does NOT wrap text.
// Just transforms the data shape for the component layer.

import type { MsgInfo, ToolCall } from "../protocol/types.js"
import type { AssistantTurnProjection, ToolSlot, TurnProjection } from "./turnProjection.js"
import { projectTurns } from "./turnProjection.js"

interface TranscriptRowBase {
  id: string
  measureKey: string
}

export type TranscriptRow =
  | (TranscriptRowBase & { type: "userPrompt"; text: string; attachments: string[] })
  | (TranscriptRowBase & { type: "assistantText"; text: string; streaming: boolean })
  | (TranscriptRowBase & { type: "thinking"; lines: string[]; hiddenCount: number })
  | (TranscriptRowBase & { type: "toolCall"; tool: ToolCall })
  | (TranscriptRowBase & { type: "indicator"; model: string })
  | (TranscriptRowBase & { type: "error"; message: string })

export type TranscriptEntry = TranscriptRow

export interface TranscriptOptions {
  messages: MsgInfo[]
  streaming: boolean
  streamingMsgId: string | null
  thinkExpanded: boolean
  error: string | null
  model: string
}

// Build transcript from messages. Each entry is one renderable block.
// The component layer (L4) decides how to render and how tall each block is.
export function buildTranscriptRows(opts: TranscriptOptions): TranscriptRow[] {
  const turns = projectTurns(opts.messages)
  const rows = turns.flatMap((turn) => buildTurnRows(turn, opts))
  const indicatorRow = buildIndicatorRow(opts)
  const errorRow = buildErrorRow(opts)

  return rows
    .concat(indicatorRow ? [indicatorRow] : [])
    .concat(errorRow ? [errorRow] : [])
}

export const buildTranscript = buildTranscriptRows

function buildTurnRows(turn: TurnProjection, opts: TranscriptOptions): TranscriptRow[] {
  if (turn.kind === "user") {
    return [buildUserPromptRow(turn.message.id, turn.text, turn.attachments)]
  }

  return buildAssistantRows(turn, opts)
}

function buildUserPromptRow(messageId: string, text: string, attachments: string[]): TranscriptRow {
  return {
    type: "userPrompt",
    id: `${messageId}-user`,
    text,
    attachments,
    measureKey: ["userPrompt", attachments.length].join(":"),
  }
}

function buildAssistantRows(turn: AssistantTurnProjection, opts: TranscriptOptions): TranscriptRow[] {
  const rows: TranscriptRow[] = []
  const thinkingRow = buildThinkingRow(turn, opts)
  const assistantTextRow = buildAssistantTextRow(turn, opts)
  const orderedTools = [...turn.normalTools, ...turn.agentTools]
  const toolRows = orderedTools.map((slot) => buildToolCallRow(turn.message.id, slot))

  if (thinkingRow) {
    rows.push(thinkingRow)
  }

  if (assistantTextRow) {
    rows.push(assistantTextRow)
  }

  return rows.concat(toolRows)
}

function buildThinkingRow(turn: AssistantTurnProjection, opts: TranscriptOptions): TranscriptRow | null {
  if (!turn.thinking) {
    return null
  }

  const allLines = turn.thinking.split("\n")
  const maxVisible = 5
  const visible = opts.thinkExpanded ? allLines : allLines.slice(-maxVisible)
  const hidden = opts.thinkExpanded ? 0 : Math.max(0, allLines.length - maxVisible)

  return {
    type: "thinking",
    id: `${turn.message.id}-thinking`,
    lines: visible,
    hiddenCount: hidden,
    measureKey: ["thinking", visible.length, hidden].join(":"),
  }
}

function buildAssistantTextRow(turn: AssistantTurnProjection, opts: TranscriptOptions): TranscriptRow | null {
  if (!turn.text) {
    return null
  }

  const streaming = turn.message.id === opts.streamingMsgId

  return {
    type: "assistantText",
    id: `${turn.message.id}-text`,
    text: turn.text,
    streaming,
    measureKey: ["assistantText", streaming ? "streaming" : "static", getTextMeasureToken(turn.text)].join(":"),
  }
}

function buildToolCallRow(messageId: string, slot: ToolSlot): TranscriptRow {
  return {
    type: "toolCall",
    id: `${messageId}-tool-${slot.tool.callId}`,
    tool: slot.tool,
    measureKey: buildToolMeasureKey(slot.tool),
  }
}

function buildIndicatorRow(opts: TranscriptOptions): TranscriptRow | null {
  const streamingMsg = opts.streamingMsgId
    ? opts.messages.find((m) => m.id === opts.streamingMsgId)
    : null
  const hasContent = streamingMsg && (streamingMsg.text || (streamingMsg.tools && streamingMsg.tools.length > 0))

  if (opts.streaming && !hasContent) {
    return {
      type: "indicator",
      id: "thinking-indicator",
      model: opts.model,
      measureKey: "indicator",
    }
  }

  return null
}

function buildErrorRow(opts: TranscriptOptions): TranscriptRow | null {
  if (!opts.error) {
    return null
  }

  return {
    type: "error",
    id: "error",
    message: opts.error,
    measureKey: "error",
  }
}

function getTextMeasureToken(text: string): string {
  const lineLengths = text
    .split("\n")
    .map((line) => line.length)
    .join(",")

  return lineLengths
}

function buildToolMeasureKey(tool: ToolCall): string {
  const outputLineCount = tool.output?.split("\n").length ?? 0
  const childCount = tool.children?.length ?? 0

  return [
    "toolCall",
    tool.callId,
    tool.running ? "running" : "done",
    tool.output ? "output" : "empty",
    outputLineCount,
    childCount,
  ].join(":")
}
