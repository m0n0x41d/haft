import React from "react"
import { Box, Text } from "ink"
import type { ToolCall } from "../protocol/types.js"

const BLACK_CIRCLE = process.platform === "darwin" ? "\u23FA" : "\u25CF"

// Chaotic animation constants removed — only ThinkingIndicator uses them.
// Tool calls use simple colored dot.

interface Props {
  tool: ToolCall
  width: number
}

// ⎿ prefix (2 chars) + ToolDot + bold name + (params)
// All aligned at paddingX={1} from parent
export function ToolCallView({ tool, width }: Props) {
  const displayName = TOOL_NAMES[tool.name] ?? tool.name
  const param = extractParam(tool.name, tool.args)
  const summary = tool.subagent?.summary ?? tool.output

  return (
    <Box flexDirection="column" paddingX={1} marginTop={1}>
      {/* Header: dot Name (param) */}
      <Box>
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
      {tool.output && tool.running && (
        <Box marginLeft={2} flexDirection="column">
          {tool.output.split("\n").slice(-3).map((line, i) => (
            <Text key={i} dimColor>{truncate(line, width - 6)}</Text>
          ))}
        </Box>
      )}

      {/* Subagent children */}
      {tool.subagent?.tools.length && (
        <SubagentChildren children={tool.subagent.tools} />
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
    <Box marginLeft={2}>
      <Text dimColor>{"\u21B3 "}</Text>
      <Text dimColor>{truncate(firstLine.trim(), width - 6)}</Text>
    </Box>
  )
}

function SubagentChildren({ children }: { children: ToolCall[] }) {
  const visible = children.slice(-5)
  const hiddenCount = children.length - visible.length

  return (
    <Box flexDirection="column" marginLeft={2}>
      {hiddenCount > 0 && <Text dimColor>  +{hiddenCount} more</Text>}
      {visible.map((child, i) => {
        const isLast = i === visible.length - 1 && !child.running
        const childName = TOOL_NAMES[child.name] ?? child.name
        return (
          <Box key={child.callId}>
            <Text dimColor>{isLast ? "\u2514\u2500 " : "\u251C\u2500 "}</Text>
            <ToolDot tool={child} />
            <Text bold>{childName}</Text>
          </Box>
        )
      })}
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

function extractParam(name: string, args: string): string | null {
  const KEY_MAP: Record<string, string> = {
    bash: "command", read: "path", write: "path", edit: "path", multiedit: "path",
    glob: "pattern", grep: "pattern", spawn_agent: "task",
    haft_problem: "action", haft_solution: "action", haft_decision: "action",
    haft_query: "action", haft_note: "title", web_search: "query",
  }
  const key = KEY_MAP[name]
  if (!key) return null
  try { return JSON.parse(args)[key] ?? null } catch { return null }
}

function truncate(s: string, max: number): string {
  if (max < 4) return ""
  if (s.length <= max) return s
  return s.slice(0, max - 3) + "\u2026"
}
