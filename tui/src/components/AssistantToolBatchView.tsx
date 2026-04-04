import React from "react"
import { Box } from "ink"
import type { ToolCall } from "../protocol/types.js"
import { buildToolBatchDisplay, getToolBatchItemKey } from "./toolBatch.js"
import { ToolCallView } from "./ToolCallView.js"

interface Props {
  tools: ToolCall[]
  width: number
  expanded: boolean
}

export function AssistantToolBatchView({ tools, width, expanded }: Props) {
  const display = buildToolBatchDisplay(tools, { expanded })

  return (
    <Box flexDirection="column" flexShrink={0}>
      {display.map((item) => (
        <ToolCallView key={getToolBatchItemKey(item)} item={item} width={width} />
      ))}
    </Box>
  )
}
