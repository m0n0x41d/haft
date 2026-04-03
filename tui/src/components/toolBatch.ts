import type { SubagentRun, ToolCall } from "../protocol/types.js"

export interface ToolDisplay {
  tool: ToolCall
  kind: "regular" | "spawnedAgent"
  subagentLabel?: string
  children: ToolDisplay[]
}

// Display order is a stable partition:
// render regular sibling tools first, then spawned-agent parents.
export function buildToolBatchDisplay(tools: readonly ToolCall[]): ToolDisplay[] {
  const regularTools = tools.filter((tool) => !isSpawnAgentTool(tool))
  const spawnedAgentTools = tools.filter(isSpawnAgentTool)
  const orderedTools = [...regularTools, ...spawnedAgentTools]

  return orderedTools.map(buildToolDisplay)
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

function buildToolDisplay(tool: ToolCall): ToolDisplay {
  return {
    tool,
    kind: isSpawnAgentTool(tool) ? "spawnedAgent" : "regular",
    subagentLabel: tool.subagent ? formatSubagentLabel(tool.subagent) : undefined,
    children: tool.subagent?.tools.map(buildToolDisplay) ?? [],
  }
}
