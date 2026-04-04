// L2: Protocol wire types + normalized TUI state types.
// Wire shapes mirror internal/protocol/*.go. State shapes normalize TUI-only data.

// --- Wire format ---

export interface JsonRpcMessage {
  jsonrpc: "2.0"
  method?: string
  params?: unknown
  result?: unknown
  error?: { code: number; message: string }
  id?: number
}

// --- Backend → TUI notifications ---

export interface InitParams {
  session: SessionInfo
  projectRoot: string
  width: number
  height: number
  messages?: WireMsgInfo[]
}

export interface SessionInfo {
  id: string
  title: string
  model: string
  interaction?: "symbiotic" | "autonomous"
  yolo?: boolean
}

export interface WireMsgInfo {
  id: string
  role: "user" | "assistant"
  text: string
  attachments?: MessageAttachment[]
  thinking?: string
  tools?: WireToolCall[]
}

export interface MsgUpdateParams {
  id: string
  text: string
  attachments?: MessageAttachment[]
  thinking?: string
  tools?: WireToolCall[]
  streaming: boolean
}

export interface WireToolCall {
  callId: string
  name: string
  args: string
  output?: string
  isError?: boolean
  running: boolean
  subagentId?: string
  children?: WireToolCall[]
}

export interface ChatMessage {
  id: string
  role: "user" | "assistant"
  text: string
  attachments?: MessageAttachment[]
  thinking?: string
  tools?: ToolCall[]
}

export interface MessageAttachment {
  name: string
  isImage: boolean
}

export interface SubagentRun {
  id: string
  name: string
  task: string
  running: boolean
  isError?: boolean
  summary?: string
  tools: ToolCall[]
}

export interface ToolCall {
  callId: string
  name: string
  args: string
  output?: string
  isError?: boolean
  running: boolean
  subagent?: SubagentRun
}

export interface ToolStartParams {
  callId: string
  name: string
  args: string
  subagentId?: string
}

export interface ToolProgressParams {
  callId: string
  text: string
}

export interface ToolDoneParams {
  callId: string
  name: string
  output: string
  isError: boolean
  subagentId?: string
}

export interface TokenUpdateParams {
  used: number
  limit: number
}

export interface SessionTitleParams {
  title: string
}

export interface CycleUpdateParams {
  cycleId: string
  problemRef: string
  problemTitle: string
  portfolioRef?: string
  decisionRef?: string
  phase: "frame" | "explore" | "compare" | "decide" | "implement" | "measure"
  status: "active" | "complete" | "abandoned"
  rEff: number
}

export interface SubagentStartParams {
  subagentId: string
  parentCallId: string
  name: string
  task: string
}

export interface SubagentDoneParams {
  subagentId: string
  summary: string
  isError: boolean
}

export interface OverseerDriftItemParams {
  path: string
  status: string
  linesChanged?: string
  invariants?: string[]
}

export interface OverseerDebtBreakdownParams {
  decisionId: string
  decisionTitle: string
  totalED: number
  expiredEvidence: number
  mostOverdueDays: number
}

export interface OverseerFindingParams {
  type: string
  category?: string
  artifactId?: string
  title?: string
  kind?: string
  summary: string
  reason?: string
  daysStale?: number
  rEff?: number
  totalED?: number
  budget?: number
  excess?: number
  driftItems?: OverseerDriftItemParams[]
  debtBreakdown?: OverseerDebtBreakdownParams[]
}

export interface OverseerAlertParams {
  alerts: string[]
  findings?: OverseerFindingParams[]
}

export interface DriftUpdateParams {
  drifted: number
  stale: number
  coverage: number
}

export interface LspUpdateParams {
  servers: Record<string, string>
  errors: number
  warnings: number
}

export interface ErrorParams {
  message: string
}

// --- Backend → TUI requests ---

export interface PermissionAskParams {
  toolName: string
  args: string
  description: string
  filePath?: string
  diff?: string
  adds?: number
  dels?: number
}

export interface PermissionReply {
  action: "allow" | "allow_session" | "deny"
  yolo?: boolean
}

export interface QuestionAskParams {
  question: string
  options?: string[]
}

export interface QuestionReply {
  answer: string
}

// --- TUI → Backend notifications ---

export interface SubmitParams {
  text: string
  displayText?: string
  attachments?: Attachment[]
}

export interface Attachment {
  name: string
  path: string
  mimeType?: string
  isImage: boolean
  content?: string
  data?: string
}

export interface ResizeParams {
  width: number
  height: number
}

// --- TUI → Backend requests ---

export interface SessionListResponse {
  sessions: SessionInfo[]
}

export interface ModelInfo {
  id: string
  name: string
  provider: string
  contextWindow: number
  canReason: boolean
}

export interface ModelListResponse {
  models: ModelInfo[]
}

export interface FileInfo {
  path: string
  size: number
}

export interface FileListResponse {
  files: FileInfo[]
}
