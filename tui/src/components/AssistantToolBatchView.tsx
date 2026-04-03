import React from "react"
import { Box } from "ink"
import type { ToolCall } from "../protocol/types.js"
import { buildToolBatchDisplay } from "./toolBatch.js"
import { ToolCallView } from "./ToolCallView.js"

interface Props {
  tools: ToolCall[]
  width: number
}

export function AssistantToolBatchView({ tools, width }: Props) {
  const display = buildToolBatchDisplay(tools)

  return (
    <Box flexDirection="column">
      {display.map((item) => (
        <ToolCallView key={item.tool.callId} display={item} width={width} />
      ))}
    </Box>
  )
}
