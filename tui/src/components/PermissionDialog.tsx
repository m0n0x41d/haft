import React from "react"
import { Box, Text } from "ink"
import { useInput } from "../hooks/useInput.js"
import { DiffView } from "./DiffView.js"

interface Props {
  request: {
    toolName: string
    args: string
    description: string
    diff?: string
    adds?: number
    dels?: number
  }
  onRespond: (action: "allow" | "allow_session" | "deny") => void
  width: number
}

// Round border, yellow, top-aligned
export const PermissionDialog = React.memo(function PermissionDialog({ request, onRespond, width }: Props) {
  useInput((input, key) => {
    switch (input) {
      case "y": case "1": onRespond("allow"); break
      case "a": case "2": onRespond("allow_session"); break
      case "n": case "3": onRespond("deny"); break
    }
    if (key.escape) onRespond("deny")
  })

  const boxWidth = Math.min(width - 4, 70)
  const param = extractMainParam(request.args, boxWidth - 4)

  return (
    <Box
      flexDirection="column"
      borderStyle="round"
      borderColor="yellow"
      paddingX={2}
      paddingY={1}
      width={boxWidth}
    >
      <Text color="yellow" bold>{"Allow "}{request.toolName}{"?"}</Text>

      {request.description && <Text dimColor>{request.description}</Text>}
      {param && <Text>{param}</Text>}

      {request.diff && (
        <Box flexDirection="column" marginTop={1}>
          {request.adds !== undefined && request.dels !== undefined && (
            <Text>
              <Text color="green">{"+"}{request.adds}</Text>
              <Text> </Text>
              <Text color="red">{"-"}{request.dels}</Text>
            </Text>
          )}
          <DiffView diff={request.diff.slice(0, 800)} width={boxWidth - 4} />
        </Box>
      )}

      <Box marginTop={1} gap={2}>
        <Text backgroundColor="green" color="black" bold> y allow </Text>
        <Text backgroundColor="blue" color="white" bold> a all </Text>
        <Text backgroundColor="red" color="white" bold> n deny </Text>
      </Box>
    </Box>
  )
})

function extractMainParam(args: string, max: number): string | null {
  try {
    const parsed = JSON.parse(args)
    const key = parsed.command ?? parsed.path ?? parsed.pattern ?? parsed.task ?? ""
    if (!key) return null
    return key.length > max ? key.slice(0, max - 3) + "\u2026" : key
  } catch {
    return args.length > max ? args.slice(0, max - 3) + "\u2026" : args
  }
}
