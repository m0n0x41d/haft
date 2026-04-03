import type { MsgInfo, ToolCall } from "../protocol/types.js"

export type ToolCallKind = "normal" | "agent"

export interface ToolSlot {
  kind: ToolCallKind
  ordinal: number
  tool: ToolCall
}

export interface UserTurnProjection {
  kind: "user"
  message: MsgInfo
  text: string
  attachments: string[]
}

export interface AssistantTurnProjection {
  kind: "assistant"
  message: MsgInfo
  thinking: string | null
  text: string
  normalTools: ToolSlot[]
  agentTools: ToolSlot[]
}

export type TurnProjection = UserTurnProjection | AssistantTurnProjection

export function projectTurns(messages: MsgInfo[]): TurnProjection[] {
  return messages.map(projectTurn)
}

function projectTurn(message: MsgInfo): TurnProjection {
  if (message.role === "user") {
    return projectUserTurn(message)
  }

  return projectAssistantTurn(message)
}

function projectUserTurn(message: MsgInfo): UserTurnProjection {
  const lines = message.text.split("\n")
  const attachments = lines
    .slice(1)
    .filter((line) => line.startsWith("["))

  return {
    kind: "user",
    message,
    text: lines[0] ?? "",
    attachments,
  }
}

function projectAssistantTurn(message: MsgInfo): AssistantTurnProjection {
  const toolSlots = buildToolSlots(message.tools ?? [])
  const normalTools = toolSlots.filter((slot) => slot.kind === "normal")
  const agentTools = toolSlots.filter((slot) => slot.kind === "agent")

  return {
    kind: "assistant",
    message,
    thinking: message.thinking ?? null,
    text: message.text,
    normalTools,
    agentTools,
  }
}

function buildToolSlots(tools: ToolCall[]): ToolSlot[] {
  const slots = tools.map((tool, ordinal) => ({
    kind: getToolCallKind(tool),
    ordinal,
    tool,
  }))
  const orderedSlots = [...slots].sort(compareToolSlots)

  return orderedSlots
}

function compareToolSlots(left: ToolSlot, right: ToolSlot): number {
  if (left.ordinal !== right.ordinal) {
    return left.ordinal - right.ordinal
  }

  return left.tool.callId.localeCompare(right.tool.callId)
}

function getToolCallKind(tool: ToolCall): ToolCallKind {
  const hasSubagent = Boolean(tool.subagentId)
  const hasChildren = (tool.children?.length ?? 0) > 0

  if (tool.name === "spawn_agent" || hasSubagent || hasChildren) {
    return "agent"
  }

  return "normal"
}
