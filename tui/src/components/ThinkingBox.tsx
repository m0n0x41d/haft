import React from "react"
import { Box, Text } from "ink"

interface Props {
  thinking: string
  expanded: boolean
  width: number
}

// Dim text, ┃ gutter, collapsible
export function ThinkingBox({ thinking, expanded }: Props) {
  const lines = thinking.split("\n")
  const maxLines = 5
  const display = expanded ? lines : lines.slice(-maxLines)
  const hidden = expanded ? 0 : Math.max(0, lines.length - maxLines)

  return (
    <Box flexDirection="column" marginLeft={5}>
      {hidden > 0 && (
        <Text dimColor>{"... ("}{hidden}{" lines hidden, press t to expand)"}</Text>
      )}
      {display.map((line, i) => (
        <Text key={i} dimColor><Text color="gray">{"\u2503"}</Text> {line}</Text>
      ))}
      {expanded && lines.length > maxLines && (
        <Text dimColor>{"(press t to collapse)"}</Text>
      )}
    </Box>
  )
}
