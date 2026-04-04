import React from "react"
import { Box, Text } from "ink"
import type { ToolCall } from "../protocol/types.js"
import type {
  CollapsedToolGroupDisplay,
  RegularToolDisplay,
  SpawnedAgentToolDisplay,
  ToolBatchItemDisplay,
} from "./toolBatch.js"
import { extractToolParam } from "./toolBatch.js"

const BLACK_CIRCLE = process.platform === "darwin" ? "\u23FA" : "\u25CF"

// Chaotic animation constants removed — only ThinkingIndicator uses them.
// Tool calls use simple colored dot.

interface Props {
  item: ToolBatchItemDisplay
  width: number
  depth?: number
}

// ⎿ prefix (2 chars) + ToolDot + bold name + (params)
// All aligned at paddingX={1} from parent
export function ToolCallView({ item, width, depth = 0 }: Props) {
  if (item.kind === "collapsedHistory") {
    return <CollapsedToolHistoryView display={item} width={width} depth={depth} />
  }

  if (item.kind === "spawnedAgent") {
    return <SpawnedAgentToolCallView display={item} width={width} depth={depth} />
  }

  return <RegularToolCallView display={item} width={width} depth={depth} />
}

function RegularToolCallView({
  display,
  width,
  depth,
}: {
  display: RegularToolDisplay
  width: number
  depth: number
}) {
  const { tool } = display
  const displayName = TOOL_NAMES[tool.name] ?? tool.name
  const param = extractToolParam(tool.name, tool.args)
  const summary = getToolSummary(tool)

  return (
    <Box flexDirection="column" paddingX={1} marginTop={1} marginLeft={depth > 0 ? 2 : 0} flexShrink={0} width={width}>
      {/* Header: dot Name (param) */}
      <Box width={width}>
        <ToolDot tool={tool} />
        <Text bold>{displayName}</Text>
        {param && (
          <Text dimColor> ({truncate(param, width - displayName.length - 8)})</Text>
        )}
      </Box>

      {/* Completed output — single summary line, indented under name */}
      {summary && !tool.running && (
        <ToolResultSummary output={summary} toolName={tool.name} width={width} />
      )}

      {/* Streaming output — last 3 lines */}
      {tool.output && tool.running && !tool.subagent && (
        <StreamingToolOutput output={tool.output} width={width} />
      )}
    </Box>
  )
}

function getToolSummary(tool: ToolCall): string | undefined {
  if (tool.subagent) {
    return undefined
  }

  return tool.output
}

function StreamingToolOutput({ output, width }: { output: string; width: number }) {
  return (
    <Box marginLeft={2} flexDirection="column" flexShrink={0} width={Math.max(0, width - 2)}>
      {output.split("\n").slice(-3).map((line, i) => (
        <Text key={i} dimColor>{truncate(line, width - 6)}</Text>
      ))}
    </Box>
  )
}

function SpawnedAgentToolCallView({
  display,
  width,
  depth,
}: {
  display: SpawnedAgentToolDisplay
  width: number
  depth: number
}) {
  const tool = display.tool
  const displayName = TOOL_NAMES[tool.name] ?? tool.name
  const param = extractToolParam(tool.name, tool.args)
  const nestedWidth = Math.max(24, width - 2)
  const subagent = display.tool.subagent

  if (!subagent) {
    return null
  }

  const showWaitingState =
    subagent.running &&
    display.children.length === 0 &&
    !display.collapsedChildren &&
    !subagent.summary

  return (
    <Box flexDirection="column" paddingX={1} marginTop={1} marginLeft={depth > 0 ? 2 : 0} flexShrink={0} width={width}>
      <Box width={width}>
        <ToolDot tool={tool} />
        <Text bold>{displayName}</Text>
        {param && (
          <Text dimColor> ({truncate(param, width - displayName.length - 8)})</Text>
        )}
      </Box>

      <Box flexDirection="column" marginLeft={2} flexShrink={0} width={nestedWidth}>
        <Box>
          <Text dimColor>{"\u21B3 "}</Text>
          <Text color="cyan" bold>{display.subagentLabel ?? "agent"}</Text>
          {subagent.running && <Text dimColor>{" (running)"}</Text>}
          {subagent.isError && !subagent.running && <Text color="red">{" (failed)"}</Text>}
        </Box>

        {showWaitingState && (
          <Box marginLeft={2}>
            <Text dimColor>waiting for tool activity</Text>
          </Box>
        )}

        {display.collapsedChildren && (
          <ToolCallView
            item={display.collapsedChildren}
            width={nestedWidth}
            depth={depth + 1}
          />
        )}

        {display.children.map((child) => (
          <ToolCallView
            key={child.kind === "collapsedHistory" ? child.id : child.tool.callId}
            item={child}
            width={nestedWidth}
            depth={depth + 1}
          />
        ))}

        {subagent.summary && !subagent.running && (
          <ToolResultSummary output={subagent.summary} toolName={display.tool.name} width={nestedWidth} />
        )}
      </Box>
    </Box>
  )
}

function CollapsedToolHistoryView({
  display,
  width,
  depth,
}: {
  display: CollapsedToolGroupDisplay
  width: number
  depth: number
}) {
  const dotState = {
    running: display.running,
    isError: display.isError,
  }

  return (
    <Box flexDirection="column" paddingX={1} marginTop={1} marginLeft={depth > 0 ? 2 : 0} flexShrink={0} width={width}>
      <Box width={width}>
        <ToolStateDot running={dotState.running} isError={dotState.isError} />
        <Text dimColor>{display.summary}</Text>
        <Text dimColor>{" (ctrl+o to expand)"}</Text>
      </Box>

      {display.hint && (
        <Box marginLeft={2} width={Math.max(0, width - 2)}>
          <Text dimColor>{"\u21B3 "}{truncate(display.hint, width - 8)}</Text>
        </Box>
      )}
    </Box>
  )
}

// Simple colored dot for tool calls. Chaotic animation is ONLY in ThinkingIndicator.
function ToolDot({ tool }: { tool: ToolCall }) {
  const [blink, setBlink] = React.useState(true)
  React.useEffect(() => {
    if (!tool.running) return
    const timer = setInterval(() => setBlink((b) => !b), 500)
    return () => clearInterval(timer)
  }, [tool.running])

  if (tool.running) {
    return <Box minWidth={2}><Text color="yellow">{blink ? BLACK_CIRCLE : " "}</Text></Box>
  }
  if (tool.isError) {
    return <Box minWidth={2}><Text color="red">{BLACK_CIRCLE}</Text></Box>
  }
  return <Box minWidth={2}><Text color="green">{BLACK_CIRCLE}</Text></Box>
}

function ToolStateDot({ running, isError }: { running: boolean; isError: boolean }) {
  const [blink, setBlink] = React.useState(true)

  React.useEffect(() => {
    if (!running) {
      return
    }

    const timer = setInterval(() => setBlink((currentBlink) => !currentBlink), 500)
    return () => clearInterval(timer)
  }, [running])

  if (running) {
    return <Box minWidth={2}><Text color="yellow">{blink ? BLACK_CIRCLE : " "}</Text></Box>
  }

  if (isError) {
    return <Box minWidth={2}><Text color="red">{BLACK_CIRCLE}</Text></Box>
  }

  return <Box minWidth={2}><Text color="green">{BLACK_CIRCLE}</Text></Box>
}

// Tool result — ⎿ + one-line summary
function ToolResultSummary({
  output,
  toolName,
  width,
}: {
  output: string
  toolName: string
  width: number
}) {
  if (!output) return null

  const isEditTool = toolName === "edit" || toolName === "multiedit"
  const hasDiff = isEditTool && (output.includes("--- old") || output.includes("@@"))

  if (hasDiff) {
    const adds = (output.match(/^\+[^+]/gm) || []).length
    const dels = (output.match(/^-[^-]/gm) || []).length
    return (
      <Box>
        <Text dimColor>{"\u21B3"} </Text>
        <Text color="green">+{adds}</Text>
        <Text> </Text>
        <Text color="red">-{dels}</Text>
      </Box>
    )
  }

  const firstLine = output.split("\n").find((l) => l.trim().length > 0)
  if (!firstLine) return null

  return (
    <Box marginLeft={2} flexShrink={0} width={Math.max(0, width - 2)}>
      <Text dimColor>{"\u21B3 "}</Text>
      <Text dimColor>{truncate(firstLine.trim(), width - 6)}</Text>
    </Box>
  )
}

const TOOL_NAMES: Record<string, string> = {
  bash: "Bash", read: "Read", write: "Write", edit: "Edit", multiedit: "MultiEdit",
  glob: "Glob", grep: "Grep", spawn_agent: "Agent", fetch: "Fetch",
  haft_problem: "Frame", haft_solution: "Explore", haft_decision: "Decide",
  haft_query: "Query", haft_refresh: "Refresh", haft_note: "Note",
  web_search: "WebSearch", ask_user_question: "AskUser", tool_search: "ToolSearch",
  enter_plan_mode: "EnterPlanMode", exit_plan_mode: "ExitPlanMode",
  enter_worktree: "Worktree", exit_worktree: "ExitWorktree",
  lsp_diagnostics: "LSP", lsp_references: "LSP", lsp_restart: "LSP",
  task_create: "TaskCreate", task_get: "TaskGet", task_list: "TaskList",
  task_stop: "TaskStop", task_update: "TaskUpdate", task_output: "TaskOutput",
}

function truncate(s: string, max: number): string {
  if (max < 4) return ""
  if (s.length <= max) return s
  return s.slice(0, max - 3) + "\u2026"
}
