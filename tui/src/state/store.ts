// L3 equivalent: pure state + transitions.
// All state changes go through the reducer. No side effects.

import type {
  MsgInfo, ToolCall, SessionInfo, CycleUpdateParams,
  MsgUpdateParams, ToolStartParams, ToolProgressParams, ToolDoneParams,
  TokenUpdateParams, SubagentStartParams, SubagentDoneParams,
  OverseerAlertParams, OverseerFindingParams, DriftUpdateParams, LspUpdateParams,
} from "../protocol/types.js"

export interface AppState {
  // Session
  session: SessionInfo
  projectRoot: string

  // Chat
  messages: MsgInfo[]
  streamingMsgId: string | null

  // Status
  tokensUsed: number
  tokensLimit: number
  mode: "symbiotic" | "autonomous"

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
  autoApprove: boolean
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
    mode: "symbiotic",
    cycle: null,
    activeSubagents: 0,
    drift: null,
    overseerAlerts: [],
    overseerFindings: [],
    lsp: null,
    phase: "input",
    error: null,
    notification: null,
    autoApprove: false,
    thinkExpanded: false,
    permissionRequest: null,
    questionRequest: null,
  }
}

// --- Pure reducer ---

export type Action =
  | { type: "init"; session: SessionInfo; projectRoot: string; messages?: MsgInfo[] }
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
  | { type: "toggle.think" }
  | { type: "set.notification"; text: string }
  | { type: "submitted" }

export function reducer(state: AppState, action: Action): AppState {
  switch (action.type) {
    case "init":
      return {
        ...state,
        session: action.session,
        projectRoot: action.projectRoot,
        messages: action.messages ?? [],
        phase: "input",
      }

    case "msg.update": {
      const { params } = action
      const idx = state.messages.findIndex((m) => m.id === params.id)
      const existing = idx >= 0 ? state.messages[idx] : null
      const msg: MsgInfo = {
        id: params.id,
        role: existing?.role ?? (params.id.startsWith("user-") ? "user" : "assistant"),
        text: params.text,
        thinking: params.thinking,
        tools: params.tools,
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
        return updateAssistantToolBySubagentId(state, params.subagentId, (parent) => ({
          ...parent,
          children: [...(parent.children ?? []), {
            callId: params.callId,
            name: params.name,
            args: params.args,
            running: true,
            subagentId: params.subagentId,
          }],
        }))
      }
      return updateLastAssistant(state, (msg) => {
        // Don't add if already present (msg.update may have included it)
        if (msg.tools?.some((t) => t.callId === params.callId)) {
          return {
            ...msg,
            tools: msg.tools.map((t) =>
              t.callId === params.callId ? { ...t, running: true } : t
            ),
          }
        }
        return {
          ...msg,
          tools: [...(msg.tools ?? []), {
            callId: params.callId,
            name: params.name,
            args: params.args,
            running: true,
          }],
        }
      })
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
        return updateChildTool(state, params.subagentId, params.callId, (tool) => ({
          ...tool,
          output: params.output,
          isError: params.isError,
          running: false,
        }))
      }
      return updateToolInMessages(state, params.callId, (tool) => ({
        ...tool,
        output: params.output,
        isError: params.isError,
        running: false,
      }))
    }

    case "token.update":
      return { ...state, tokensUsed: action.params.used, tokensLimit: action.params.limit }

    case "session.title":
      return { ...state, session: { ...state.session, title: action.title } }

    case "cycle.update":
      return { ...state, cycle: action.params }

    case "subagent.start": {
      const newState = { ...state, activeSubagents: state.activeSubagents + 1 }
      return updateAssistantToolByCallId(newState, action.params.parentCallId, (tool) => (
        tool.name !== "spawn_agent"
          ? tool
          : { ...tool, subagentId: action.params.subagentId }
      ))
    }

    case "subagent.done": {
      const newState = { ...state, activeSubagents: Math.max(0, state.activeSubagents - 1) }
      return updateAssistantToolBySubagentId(newState, action.params.subagentId, (tool) => ({
        ...tool,
        running: false,
        output: action.params.summary,
        isError: action.params.isError,
      }))
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
      // Mark all running tools as done
      const messages = state.messages.map((msg) => ({
        ...msg,
        tools: msg.tools?.map((t) => ({
          ...t,
          running: false,
          children: t.children?.map((c) => ({ ...c, running: false })),
        })),
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
      return { ...state, mode: state.mode === "symbiotic" ? "autonomous" : "symbiotic" }

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

function updateLastAssistant(state: AppState, fn: (msg: MsgInfo) => MsgInfo): AppState {
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

function updateAssistantToolByCallId(state: AppState, callId: string, fn: (tool: ToolCall) => ToolCall): AppState {
  return updateAssistantTool(state, (tool) => tool.callId === callId, fn)
}

function updateAssistantToolBySubagentId(state: AppState, subagentId: string, fn: (tool: ToolCall) => ToolCall): AppState {
  return updateAssistantTool(state, (tool) => tool.subagentId === subagentId, fn)
}

function updateToolInMessages(state: AppState, callId: string, fn: (tool: ToolCall) => ToolCall): AppState {
  return updateAssistantToolByCallId(state, callId, fn)
}

function updateChildTool(state: AppState, subagentId: string, callId: string, fn: (tool: ToolCall) => ToolCall): AppState {
  return updateAssistantToolBySubagentId(state, subagentId, (parent) => ({
    ...parent,
    children: parent.children?.map((c) => c.callId === callId ? fn(c) : c),
  }))
}
