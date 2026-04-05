// L3 equivalent: pure state + transitions.
// All state changes go through the reducer. No side effects.

import type {
  ChatMessage,
  MessageAttachment,
  ToolCall,
  WireMsgInfo,
  WireToolCall,
  SessionInfo,
  CycleUpdateParams,
  MsgUpdateParams,
  ToolStartParams,
  ToolProgressParams,
  ToolDoneParams,
  TokenUpdateParams,
  SubagentRun,
  SubagentStartParams,
  SubagentDoneParams,
  OverseerAlertParams, OverseerFindingParams, DriftUpdateParams, LspUpdateParams, SessionModeWire,
} from "../protocol/types.js"

type SessionMode = "checkpointed" | "autonomous"

export interface AppState {
  // Session
  session: SessionInfo
  projectRoot: string

  // Chat
  messages: ChatMessage[]
  streamingMsgId: string | null

  // Status
  tokensUsed: number
  tokensLimit: number
  mode: SessionMode
  yolo: boolean

  // Cycle (FPF)
  cycle: CycleUpdateParams | null

  // Subagents
  activeSubagents: number

  // Drift / Health
  drift: DriftUpdateParams | null
  overseerAlerts: string[]
  overseerFindings: OverseerFindingParams[]
  lsp: LspUpdateParams | null

  // UI
  phase: "input" | "streaming" | "permission" | "question"
  error: string | null
  notification: string | null
  thinkExpanded: boolean

  // Permission (pending request)
  permissionRequest: { id: number; toolName: string; args: string; description: string; diff?: string; adds?: number; dels?: number } | null

  // Question (pending request)
  questionRequest: { id: number; question: string; options?: string[] } | null
}

export function initialState(): AppState {
  return {
    session: { id: "", title: "", model: "" },
    projectRoot: "",
    messages: [],
    streamingMsgId: null,
    tokensUsed: 0,
    tokensLimit: 0,
    mode: "checkpointed",
    yolo: false,
    cycle: null,
    activeSubagents: 0,
    drift: null,
    overseerAlerts: [],
    overseerFindings: [],
    lsp: null,
    phase: "input",
    error: null,
    notification: null,
    thinkExpanded: false,
    permissionRequest: null,
    questionRequest: null,
  }
}

// --- Pure reducer ---

export type Action =
  | { type: "init"; session: SessionInfo; projectRoot: string; messages?: WireMsgInfo[] }
  | { type: "msg.update"; params: MsgUpdateParams }
  | { type: "tool.start"; params: ToolStartParams }
  | { type: "tool.progress"; params: ToolProgressParams }
  | { type: "tool.done"; params: ToolDoneParams }
  | { type: "token.update"; params: TokenUpdateParams }
  | { type: "session.title"; title: string }
  | { type: "cycle.update"; params: CycleUpdateParams }
  | { type: "subagent.start"; params: SubagentStartParams }
  | { type: "subagent.done"; params: SubagentDoneParams }
  | { type: "overseer.alert"; params: OverseerAlertParams }
  | { type: "drift.update"; params: DriftUpdateParams }
  | { type: "lsp.update"; params: LspUpdateParams }
  | { type: "error"; message: string }
  | { type: "coord.done" }
  | { type: "permission.ask"; id: number; toolName: string; args: string; description: string; diff?: string; adds?: number; dels?: number }
  | { type: "permission.replied" }
  | { type: "question.ask"; id: number; question: string; options?: string[] }
  | { type: "question.replied" }
  | { type: "clear.error" }
  | { type: "clear.notification" }
  | { type: "toggle.autonomy" }
  | { type: "toggle.yolo" }
  | { type: "toggle.think" }
  | { type: "set.notification"; text: string }
  | { type: "submitted" }

export function reducer(state: AppState, action: Action): AppState {
  switch (action.type) {
    case "init": {
      const sessionMode = normalizeSessionMode(action.session.mode ?? action.session.interaction)
      return {
        ...state,
        session: action.session,
        projectRoot: action.projectRoot,
        messages: normalizeMessages(action.messages),
        mode: sessionMode,
        yolo: action.session.yolo ?? false,
        phase: "input",
      }
    }

    case "msg.update": {
      const { params } = action
      const idx = state.messages.findIndex((m) => m.id === params.id)
      const existing = idx >= 0 ? state.messages[idx] : null
      const msg: ChatMessage = {
        id: params.id,
        role: existing?.role ?? (params.id.startsWith("user-") ? "user" : "assistant"),
        text: params.text,
        attachments: params.attachments ?? existing?.attachments,
        thinking: params.thinking,
        tools: mergeToolCollections(existing?.tools, normalizeToolCalls(params.tools)),
      }
      const messages = [...state.messages]
      if (idx >= 0) {
        messages[idx] = msg
      } else {
        messages.push(msg)
      }
      return {
        ...state,
        messages,
        streamingMsgId: params.streaming ? params.id : null,
        phase: params.streaming ? "streaming" : state.phase,
        error: null,
      }
    }

    case "tool.start": {
      const { params } = action
      if (params.subagentId) {
        const subagentId = params.subagentId
        return updateAssistantToolBySubagentId(state, subagentId, (parent) => {
          const subagent = ensureSubagent(parent, subagentId)
          const tools = upsertToolCollection(
            subagent.tools,
            createToolCall(params.callId, params.name, params.args),
            (tool) => ({
              ...tool,
              name: params.name,
              args: params.args,
              running: true,
            }),
          )
          return {
            ...parent,
            subagent: {
              ...subagent,
              tools,
            },
          }
        })
      }
      return updateLastAssistant(state, (msg) => ({
        ...msg,
        tools: upsertToolCollection(
          msg.tools,
          createToolCall(params.callId, params.name, params.args),
          (tool) => ({
            ...tool,
            name: params.name,
            args: params.args,
            running: true,
          }),
        ),
      }))
    }

    case "tool.progress": {
      const { params } = action
      return updateToolInMessages(state, params.callId, (tool) => ({
        ...tool,
        output: (tool.output ?? "") + params.text,
      }))
    }

    case "tool.done": {
      const { params } = action
      if (params.subagentId) {
        return updateSubagentTool(state, params.subagentId, params.callId, (tool) => ({
          ...tool,
          output: params.output,
          isError: params.isError,
          running: false,
        }))
      }
      return updateToolInMessages(state, params.callId, (tool) => completeToolCall(tool, params.output, params.isError))
    }

    case "token.update":
      return { ...state, tokensUsed: action.params.used, tokensLimit: action.params.limit }

    case "session.title":
      return { ...state, session: { ...state.session, title: action.title } }

    case "cycle.update":
      return { ...state, cycle: action.params }

    case "subagent.start": {
      if (!action.params.parentCallId) {
        return state
      }
      if (!hasAssistantToolByCallId(state, action.params.parentCallId)) {
        return state
      }

      const newState = updateAssistantToolByCallId(state, action.params.parentCallId, (tool) => ({
        ...tool,
        subagent: {
          ...ensureSubagent(tool, action.params.subagentId),
          id: action.params.subagentId,
          name: action.params.name,
          task: action.params.task,
          running: true,
        },
      }))

      return {
        ...newState,
        activeSubagents: state.activeSubagents + 1,
      }
    }

    case "subagent.done": {
      if (!hasAssistantToolBySubagentId(state, action.params.subagentId)) {
        return state
      }

      const newState = updateAssistantToolBySubagentId(state, action.params.subagentId, (tool) => {
        const subagent = ensureSubagent(tool, action.params.subagentId)
        return {
          ...tool,
          isError: action.params.isError || tool.isError,
          subagent: {
            ...subagent,
            running: false,
            isError: action.params.isError || subagent.isError,
            summary: action.params.summary || subagent.summary,
          },
        }
      })

      return {
        ...newState,
        activeSubagents: Math.max(0, state.activeSubagents - 1),
      }
    }

    case "overseer.alert":
      return {
        ...state,
        overseerAlerts: action.params.alerts,
        overseerFindings: action.params.findings ?? [],
      }

    case "drift.update":
      return { ...state, drift: action.params }

    case "lsp.update":
      return { ...state, lsp: action.params }

    case "error":
      return { ...state, error: action.message }

    case "coord.done": {
      const messages = state.messages.map((msg) => ({
        ...msg,
        tools: msg.tools?.map(finishToolTree),
      }))
      return {
        ...state,
        messages,
        phase: "input",
        streamingMsgId: null,
        activeSubagents: 0,
      }
    }

    case "permission.ask":
      return {
        ...state,
        phase: "permission",
        permissionRequest: {
          id: action.id,
          toolName: action.toolName,
          args: action.args,
          description: action.description,
          diff: action.diff,
          adds: action.adds,
          dels: action.dels,
        },
      }

    case "permission.replied":
      return { ...state, phase: "streaming", permissionRequest: null }

    case "question.ask":
      return {
        ...state,
        phase: "question",
        questionRequest: { id: action.id, question: action.question, options: action.options },
      }

    case "question.replied":
      return { ...state, phase: "streaming", questionRequest: null }

    case "clear.error":
      return { ...state, error: null }

    case "clear.notification":
      return { ...state, notification: null }

    case "toggle.autonomy":
      return { ...state, mode: state.mode === "checkpointed" ? "autonomous" : "checkpointed" }

    case "toggle.yolo":
      return { ...state, yolo: !state.yolo }

    case "toggle.think":
      return { ...state, thinkExpanded: !state.thinkExpanded }

    case "set.notification":
      return { ...state, notification: action.text }

    case "submitted":
      return { ...state, phase: "streaming" }

    default:
      return state
  }
}

// --- Immutable state update helpers ---

function normalizeSessionMode(mode?: SessionModeWire): SessionMode {
  return mode === "autonomous" ? "autonomous" : "checkpointed"
}

function normalizeMessages(messages?: WireMsgInfo[]): ChatMessage[] {
  return messages?.map(normalizeMessage) ?? []
}

function normalizeMessage(message: WireMsgInfo): ChatMessage {
  return {
    id: message.id,
    role: message.role,
    text: message.text,
    attachments: normalizeMessageAttachments(message.attachments),
    thinking: message.thinking,
    tools: normalizeToolCalls(message.tools),
  }
}

function normalizeMessageAttachments(
  attachments?: MessageAttachment[],
): MessageAttachment[] | undefined {
  if (!attachments?.length) {
    return undefined
  }

  return attachments.map((attachment) => ({
    name: attachment.name,
    isImage: attachment.isImage,
  }))
}

function normalizeToolCalls(tools?: WireToolCall[]): ToolCall[] | undefined {
  if (!tools?.length) {
    return undefined
  }

  return tools.map(normalizeToolCall)
}

function normalizeToolCall(tool: WireToolCall): ToolCall {
  const subagent = normalizeSubagent(tool)

  return {
    callId: tool.callId,
    name: tool.name,
    args: tool.args,
    output: subagent ? undefined : tool.output,
    isError: subagent ? undefined : tool.isError,
    running: tool.running,
    subagent,
  }
}

function normalizeSubagent(tool: WireToolCall): SubagentRun | undefined {
  const hasSubagent = Boolean(tool.subagentId) || Boolean(tool.children?.length)
  if (!hasSubagent) {
    return undefined
  }

  return {
    id: tool.subagentId ?? `legacy-${tool.callId}`,
    name: "agent",
    task: extractSpawnTask(tool.args),
    running: tool.running,
    isError: tool.isError,
    summary: tool.output || undefined,
    tools: normalizeToolCalls(tool.children) ?? [],
  }
}

function mergeToolCollections(
  existing: ToolCall[] | undefined,
  incoming: ToolCall[] | undefined,
): ToolCall[] | undefined {
  if (!incoming?.length) {
    return existing
  }
  if (!existing?.length) {
    return incoming
  }

  const existingById = new Map(existing.map((tool) => [tool.callId, tool]))
  const merged = incoming.map((incomingTool) => {
    const current = existingById.get(incomingTool.callId)
    existingById.delete(incomingTool.callId)
    return current ? mergeToolCall(current, incomingTool) : incomingTool
  })
  const remaining = existing.filter((tool) => existingById.has(tool.callId))

  return [...merged, ...remaining]
}

function mergeToolCall(existing: ToolCall, incoming: ToolCall): ToolCall {
  const subagent = mergeSubagent(existing.subagent, incoming.subagent)

  return {
    ...incoming,
    output: subagent ? undefined : (existing.output ?? incoming.output),
    isError: existing.isError ?? incoming.isError ?? subagent?.isError,
    running: existing.running,
    subagent,
  }
}

function mergeSubagent(
  existing: SubagentRun | undefined,
  incoming: SubagentRun | undefined,
): SubagentRun | undefined {
  if (!existing) {
    return incoming
  }
  if (!incoming) {
    return existing
  }

  return {
    id: existing.id || incoming.id,
    name: existing.name || incoming.name,
    task: existing.task || incoming.task,
    running: existing.running,
    isError: existing.isError ?? incoming.isError,
    summary: existing.summary ?? incoming.summary,
    tools: mergeToolCollections(existing.tools, incoming.tools) ?? [],
  }
}

function createToolCall(callId: string, name: string, args: string): ToolCall {
  return {
    callId,
    name,
    args,
    running: true,
  }
}

function completeToolCall(tool: ToolCall, output: string, isError: boolean): ToolCall {
  if (tool.name !== "spawn_agent" || !tool.subagent) {
    return {
      ...tool,
      output,
      isError,
      running: false,
    }
  }

  return {
    ...tool,
    isError: isError || tool.subagent.isError || tool.isError,
    running: false,
    subagent: {
      ...tool.subagent,
      running: false,
      isError: isError || tool.subagent.isError,
      summary: output || tool.subagent.summary,
    },
  }
}

function finishToolTree(tool: ToolCall): ToolCall {
  return {
    ...tool,
    running: false,
    subagent: tool.subagent
      ? {
          ...tool.subagent,
          running: false,
          tools: tool.subagent.tools.map(finishToolTree),
        }
      : undefined,
  }
}

function ensureSubagent(parent: ToolCall, subagentId: string): SubagentRun {
  if (parent.subagent && parent.subagent.id === subagentId) {
    return parent.subagent
  }

  return {
    id: subagentId,
    name: parent.subagent?.name ?? "agent",
    task: parent.subagent?.task ?? extractSpawnTask(parent.args),
    running: parent.subagent?.running ?? parent.running,
    isError: parent.subagent?.isError,
    summary: parent.subagent?.summary,
    tools: parent.subagent?.tools ?? [],
  }
}

function extractSpawnTask(args: string): string {
  try {
    const parsed = JSON.parse(args) as { task?: string }
    return parsed.task ?? ""
  } catch {
    return ""
  }
}

function updateLastAssistant(state: AppState, fn: (msg: ChatMessage) => ChatMessage): AppState {
  const messages = [...state.messages]
  for (let i = messages.length - 1; i >= 0; i--) {
    if (messages[i].role === "assistant") {
      messages[i] = fn(messages[i])
      return { ...state, messages }
    }
  }
  return state
}

function updateAssistantTool(
  state: AppState,
  match: (tool: ToolCall) => boolean,
  fn: (tool: ToolCall) => ToolCall,
): AppState {
  const messages = [...state.messages]
  for (let msgIndex = messages.length - 1; msgIndex >= 0; msgIndex--) {
    const msg = messages[msgIndex]
    if (msg.role !== "assistant" || !msg.tools?.length) {
      continue
    }

    const tools = [...msg.tools]
    for (let toolIndex = tools.length - 1; toolIndex >= 0; toolIndex--) {
      if (!match(tools[toolIndex])) {
        continue
      }

      tools[toolIndex] = fn(tools[toolIndex])
      messages[msgIndex] = { ...msg, tools }
      return { ...state, messages }
    }
  }

  return state
}

function updateAssistantToolByCallId(
  state: AppState,
  callId: string,
  fn: (tool: ToolCall) => ToolCall,
): AppState {
  return updateAssistantTool(state, (tool) => tool.callId === callId, fn)
}

function updateAssistantToolBySubagentId(
  state: AppState,
  subagentId: string,
  fn: (tool: ToolCall) => ToolCall,
): AppState {
  return updateAssistantTool(state, (tool) => tool.subagent?.id === subagentId, fn)
}

function hasAssistantToolByCallId(state: AppState, callId: string): boolean {
  return hasAssistantTool(state, (tool) => tool.callId === callId)
}

function hasAssistantToolBySubagentId(state: AppState, subagentId: string): boolean {
  return hasAssistantTool(state, (tool) => tool.subagent?.id === subagentId)
}

function hasAssistantTool(state: AppState, match: (tool: ToolCall) => boolean): boolean {
  for (let msgIndex = state.messages.length - 1; msgIndex >= 0; msgIndex--) {
    const msg = state.messages[msgIndex]
    if (msg.role !== "assistant" || !msg.tools?.length) {
      continue
    }

    for (let toolIndex = msg.tools.length - 1; toolIndex >= 0; toolIndex--) {
      if (match(msg.tools[toolIndex])) {
        return true
      }
    }
  }

  return false
}

function updateToolInMessages(
  state: AppState,
  callId: string,
  fn: (tool: ToolCall) => ToolCall,
): AppState {
  const messages = state.messages.map((msg) => {
    if (msg.role !== "assistant" || !msg.tools?.length) {
      return msg
    }

    const tools = updateToolCollectionByCallId(msg.tools, callId, fn)
    if (tools === msg.tools) {
      return msg
    }

    return {
      ...msg,
      tools,
    }
  })

  const changed = messages.some((msg, index) => msg !== state.messages[index])
  if (!changed) {
    return state
  }

  return {
    ...state,
    messages,
  }
}

function updateToolCollectionByCallId(
  tools: ToolCall[],
  callId: string,
  fn: (tool: ToolCall) => ToolCall,
): ToolCall[] {
  let changed = false

  const next = tools.map((tool) => {
    if (tool.callId === callId) {
      changed = true
      return fn(tool)
    }

    if (!tool.subagent?.tools.length) {
      return tool
    }

    const childTools = updateToolCollectionByCallId(tool.subagent.tools, callId, fn)
    if (childTools === tool.subagent.tools) {
      return tool
    }

    changed = true
    return {
      ...tool,
      subagent: {
        ...tool.subagent,
        tools: childTools,
      },
    }
  })

  return changed ? next : tools
}

function updateSubagentTool(
  state: AppState,
  subagentId: string,
  callId: string,
  fn: (tool: ToolCall) => ToolCall,
): AppState {
  return updateAssistantToolBySubagentId(state, subagentId, (parent) => {
    const subagent = ensureSubagent(parent, subagentId)
    const tools = updateToolCollectionByCallId(subagent.tools, callId, fn)

    return {
      ...parent,
      subagent: {
        ...subagent,
        tools,
      },
    }
  })
}

function upsertToolCollection(
  tools: ToolCall[] | undefined,
  incoming: ToolCall,
  fn: (tool: ToolCall) => ToolCall,
): ToolCall[] {
  const list = tools ?? []
  const index = list.findIndex((tool) => tool.callId === incoming.callId)

  if (index < 0) {
    return [...list, incoming]
  }

  const next = [...list]
  next[index] = fn(next[index])
  return next
}
