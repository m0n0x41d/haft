import React, { useMemo, useState } from "react"
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
  const options: Array<{ key: string; label: string; action: "allow" | "allow_session" | "deny"; fg: string; bg?: string }> = useMemo(() => ([
    { key: "y", label: "allow once", action: "allow", fg: "black", bg: "greenBright" },
    { key: "a", label: "allow all (yolo)", action: "allow_session", fg: "black", bg: "yellowBright" },
    { key: "n", label: "deny", action: "deny", fg: "white", bg: "redBright" },
  ]), [])
  const [selected, setSelected] = useState(0)

  useInput((input, key) => {
    if (key.upArrow || key.leftArrow) {
      setSelected((current) => (current + options.length - 1) % options.length)
      return
    }
    if (key.downArrow || key.rightArrow || key.tab) {
      setSelected((current) => (current + 1) % options.length)
      return
    }
    switch (input) {
      case "y": case "1": onRespond("allow"); return
      case "a": case "2": onRespond("allow_session"); return
      case "n": case "3": onRespond("deny"); return
    }
    if (key.return || input === " ") {
      onRespond(options[selected]!.action)
      return
    }
    if (key.escape) onRespond("deny")
  })

  const boxWidth = Math.min(width - 4, 76)
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

      <Box marginTop={1} flexDirection="column" gap={1}>
        {options.map((option, index) => {
          const focused = index === selected
          return (
            <Box key={option.key}>
              <Text color={focused ? "black" : "gray"} backgroundColor={focused ? "white" : undefined} bold>{focused ? "▶" : " "}</Text>
              <Text> </Text>
              <Text backgroundColor={option.bg} color={option.fg} bold> {option.key} </Text>
              <Text> </Text>
              <Text color={focused ? option.fg : option.bg} backgroundColor={focused ? option.bg : undefined} bold={focused}>{option.label}</Text>
            </Box>
          )
        })}
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
