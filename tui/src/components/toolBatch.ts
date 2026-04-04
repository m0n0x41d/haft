import type { SubagentRun, ToolCall } from "../protocol/types.js"

export interface RegularToolDisplay {
  kind: "regular"
  tool: ToolCall
}

export interface SpawnedAgentToolDisplay {
  kind: "spawnedAgent"
  tool: ToolCall
  subagentLabel?: string
  children: ToolBatchItemDisplay[]
  collapsedChildren?: CollapsedToolGroupDisplay
}

export interface CollapsedToolGroupDisplay {
  kind: "collapsedHistory"
  id: string
  tools: readonly ToolCall[]
  summary: string
  hint?: string
  running: boolean
  isError: boolean
}

export type ToolBatchItemDisplay =
  | RegularToolDisplay
  | SpawnedAgentToolDisplay
  | CollapsedToolGroupDisplay

export interface ToolBatchDisplayOptions {
  expanded?: boolean
}

// Display order is a stable partition:
// render regular sibling tools first, then spawned-agent parents.
export function buildToolBatchDisplay(
  tools: readonly ToolCall[],
  options: ToolBatchDisplayOptions = {},
): ToolBatchItemDisplay[] {
  const expanded = options.expanded ?? false
  const regularTools = tools.filter((tool) => !isSpawnAgentTool(tool))
  const spawnedAgentTools = tools.filter(isSpawnAgentTool)
  const regularItems = buildRegularItems(regularTools, expanded)
  const spawnedAgentItems = spawnedAgentTools.map((tool) => buildSpawnedAgentDisplay(tool, expanded))

  return [...regularItems, ...spawnedAgentItems]
}

export function estimateToolBatchDisplayHeight(
  tools: readonly ToolCall[],
  width: number,
  options: ToolBatchDisplayOptions = {},
): number {
  const items = buildToolBatchDisplay(tools, options)

  return items.reduce(
    (height, item) => height + estimateToolBatchItemHeight(item, width),
    0,
  )
}

export function isSpawnAgentTool(tool: Pick<ToolCall, "name">): boolean {
  return tool.name === "spawn_agent"
}

export function formatSubagentLabel(subagent: SubagentRun): string {
  const rawParts = [subagent.name.trim(), subagent.task.trim()]
  const filledParts = rawParts.filter((part) => part.length > 0)
  const uniqueParts = filledParts.filter((part, index) => {
    return filledParts.indexOf(part) === index
  })

  return uniqueParts.join(" - ") || "agent"
}

export function getToolBatchItemKey(item: ToolBatchItemDisplay): string {
  if (item.kind === "collapsedHistory") {
    return item.id
  }

  return item.tool.callId
}

export function extractToolParam(name: string, args: string): string | null {
  const KEY_MAP: Record<string, string> = {
    bash: "command",
    read: "path",
    write: "path",
    edit: "path",
    multiedit: "path",
    glob: "pattern",
    grep: "pattern",
    spawn_agent: "task",
    fetch: "url",
    web_search: "query",
    lsp_references: "symbol",
    lsp_diagnostics: "path",
  }
  const key = KEY_MAP[name]

  if (!key) {
    return null
  }

  try {
    return JSON.parse(args)[key] ?? null
  } catch {
    return null
  }
}

function buildRegularItems(
  tools: readonly ToolCall[],
  expanded: boolean,
): ToolBatchItemDisplay[] {
  const items: ToolBatchItemDisplay[] = []
  let index = 0

  while (index < tools.length) {
    const currentTool = tools[index]!

    if (expanded || !isCollapsibleHistoryTool(currentTool)) {
      items.push(buildRegularToolDisplay(currentTool))
      index += 1
      continue
    }

    let nextIndex = index + 1

    while (nextIndex < tools.length && isCollapsibleHistoryTool(tools[nextIndex]!)) {
      nextIndex += 1
    }

    const toolRun = tools.slice(index, nextIndex)

    if (!shouldCollapseToolRun(toolRun)) {
      items.push(...toolRun.map(buildRegularToolDisplay))
      index = nextIndex
      continue
    }

    items.push(buildCollapsedHistoryDisplay(toolRun))
    index = nextIndex
  }

  return items
}

function buildRegularToolDisplay(tool: ToolCall): RegularToolDisplay {
  return {
    kind: "regular",
    tool,
  }
}

function estimateToolBatchItemHeight(
  item: ToolBatchItemDisplay,
  width: number,
): number {
  if (item.kind === "collapsedHistory") {
    return estimateCollapsedHistoryHeight(item, width)
  }

  if (item.kind === "spawnedAgent") {
    return estimateSpawnedAgentHeight(item, width)
  }

  return estimateRegularToolHeight(item.tool)
}

function buildSpawnedAgentDisplay(
  tool: ToolCall,
  expanded: boolean,
): SpawnedAgentToolDisplay {
  const subagent = tool.subagent
  const subagentTools = subagent?.tools ?? []
  const collapseSubagentChildren = shouldCollapseSubagentHistory(subagent, expanded)
  const children = collapseSubagentChildren
    ? []
    : buildToolBatchDisplay(subagentTools, { expanded })

  return {
    kind: "spawnedAgent",
    tool,
    subagentLabel: subagent ? formatSubagentLabel(subagent) : undefined,
    children,
    collapsedChildren: collapseSubagentChildren
      ? buildCollapsedHistoryDisplay(subagentTools)
      : undefined,
  }
}

function estimateSpawnedAgentHeight(
  item: SpawnedAgentToolDisplay,
  width: number,
): number {
  const subagent = item.tool.subagent
  const nestedWidth = Math.max(24, width - 2)

  if (!subagent) {
    return estimateRegularToolHeight(item.tool)
  }

  const showWaitingState =
    subagent.running &&
    item.children.length === 0 &&
    !item.collapsedChildren &&
    !subagent.summary

  const collapsedHeight = item.collapsedChildren
    ? estimateCollapsedHistoryHeight(item.collapsedChildren, nestedWidth)
    : 0
  const childHeight = item.children.reduce(
    (height, child) => height + estimateToolBatchItemHeight(child, nestedWidth),
    0,
  )
  const summaryHeight = subagent.summary && !subagent.running
    ? 1
    : 0

  return 3 + collapsedHeight + childHeight + summaryHeight + (showWaitingState ? 1 : 0)
}

function shouldCollapseSubagentHistory(
  subagent: SubagentRun | undefined,
  expanded: boolean,
): boolean {
  if (!subagent || expanded) {
    return false
  }

  return shouldCollapseToolRun(subagent.tools)
}

function shouldCollapseToolRun(tools: readonly ToolCall[]): boolean {
  if (tools.length >= 4) {
    return true
  }

  const toolNames = tools.map((tool) => tool.name)
  const uniqueToolNames = new Set(toolNames)

  return uniqueToolNames.size < toolNames.length
}

function estimateRegularToolHeight(tool: ToolCall): number {
  const summary = tool.subagent?.summary ?? tool.output
  const streamingHeight = tool.output && tool.running
    ? Math.min(3, tool.output.split("\n").length)
    : 0
  const summaryHeight = summary && !tool.running
    ? 1
    : 0

  return 2 + summaryHeight + streamingHeight
}

function buildCollapsedHistoryDisplay(
  tools: readonly ToolCall[],
): CollapsedToolGroupDisplay {
  const summary = summarizeToolRun(tools)
  const hint = extractLatestHint(tools)
  const running = tools.some((tool) => tool.running)
  const isError = tools.some((tool) => tool.isError)
  const firstCallId = tools[0]?.callId ?? "tool-history"
  const lastCallId = tools[tools.length - 1]?.callId ?? "tool-history"

  return {
    kind: "collapsedHistory",
    id: `collapsed-${firstCallId}-${lastCallId}`,
    tools,
    summary,
    hint,
    running,
    isError,
  }
}

function estimateCollapsedHistoryHeight(
  item: CollapsedToolGroupDisplay,
  width: number,
): number {
  if (!item.hint) {
    return 2
  }

  const hintWidth = Math.max(1, width - 8)
  const hintHeight = item.hint
    .split("\n")
    .reduce((height, line) => height + countWrappedLines(line, hintWidth), 0)

  return 2 + hintHeight
}

function summarizeToolRun(tools: readonly ToolCall[]): string {
  const counts = {
    search: 0,
    read: 0,
    bash: 0,
    fetch: 0,
    other: 0,
  }

  for (const tool of tools) {
    const category = categorizeTool(tool)
    counts[category] += 1
  }

  const parts = [
    counts.search > 0 ? `searched ${counts.search} ${pluralize(counts.search, "pattern")}` : null,
    counts.read > 0 ? `read ${counts.read} ${pluralize(counts.read, "file")}` : null,
    counts.bash > 0 ? `ran ${counts.bash} bash ${pluralize(counts.bash, "command")}` : null,
    counts.fetch > 0 ? `fetched ${counts.fetch} ${pluralize(counts.fetch, "resource")}` : null,
    counts.other > 0 ? `ran ${counts.other} ${pluralize(counts.other, "tool call")}` : null,
  ].filter((part): part is string => Boolean(part))

  if (parts.length === 0) {
    return `Ran ${tools.length} ${pluralize(tools.length, "tool call")}`
  }

  return capitalize(parts.join(", "))
}

function extractLatestHint(tools: readonly ToolCall[]): string | undefined {
  const hint = [...tools]
    .reverse()
    .map(extractToolHint)
    .find((candidate) => candidate !== undefined)

  return hint
}

function extractToolHint(tool: ToolCall): string | undefined {
  const param = extractToolParam(tool.name, tool.args)

  if (!param) {
    return undefined
  }

  if (tool.name === "bash") {
    return formatCommandHint(param)
  }

  if (tool.name === "grep" || tool.name === "glob" || tool.name === "web_search") {
    return `"${param}"`
  }

  return param
}

function isCollapsibleHistoryTool(tool: ToolCall): boolean {
  const COLLAPSIBLE_TOOL_NAMES = new Set([
    "bash",
    "read",
    "grep",
    "glob",
    "fetch",
    "web_search",
    "tool_search",
    "lsp_references",
    "lsp_diagnostics",
  ])

  return COLLAPSIBLE_TOOL_NAMES.has(tool.name)
}

function categorizeTool(tool: ToolCall): "search" | "read" | "bash" | "fetch" | "other" {
  if (tool.name === "read") {
    return "read"
  }

  if (tool.name === "grep" || tool.name === "glob" || tool.name === "tool_search") {
    return "search"
  }

  if (tool.name === "bash") {
    return "bash"
  }

  if (tool.name === "fetch" || tool.name === "web_search") {
    return "fetch"
  }

  return "other"
}

function formatCommandHint(command: string): string {
  const normalizedLines = command
    .split("\n")
    .map((line) => line.replace(/\s+/g, " ").trim())
    .filter((line) => line.length > 0)

  const compactCommand = normalizedLines.join(" && ")
  const prefix = "$ "

  return `${prefix}${compactCommand}`
}

function pluralize(count: number, noun: string): string {
  if (count === 1) {
    return noun
  }

  return `${noun}s`
}

function capitalize(text: string): string {
  const firstChar = text[0]

  if (!firstChar) {
    return text
  }

  return `${firstChar.toUpperCase()}${text.slice(1)}`
}

function countWrappedLines(text: string, width: number): number {
  if (!text) {
    return 1
  }

  return text
    .split("\n")
    .reduce((height, line) => height + Math.max(1, Math.ceil(line.length / width)), 0)
}
