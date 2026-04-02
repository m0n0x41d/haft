import React from "react"
import { Box, Text } from "ink"

interface Props {
  diff: string
  width: number
}

export function DiffView({ diff, width }: Props) {
  const lines = diff.split("\n")

  return (
    <Box flexDirection="column">
      {lines.map((line, i) => (
        <DiffLine key={i} line={line} width={width} />
      ))}
    </Box>
  )
}

function DiffLine({ line, width }: { line: string; width: number }) {
  const truncated = line.length > width ? line.slice(0, width - 1) + "…" : line

  if (line.startsWith("@@")) {
    return <Text color="cyan">{truncated}</Text>
  }
  if (line.startsWith("+++") || line.startsWith("---")) {
    return <Text color="cyan" dimColor>{truncated}</Text>
  }
  if (line.startsWith("+")) {
    return (
      <Text>
        <Text color="green" bold>+</Text>
        <Text color="green" backgroundColor="blackBright">{truncated.slice(1)}</Text>
      </Text>
    )
  }
  if (line.startsWith("-")) {
    return (
      <Text>
        <Text color="red" bold>-</Text>
        <Text color="red" backgroundColor="blackBright">{truncated.slice(1)}</Text>
      </Text>
    )
  }
  return <Text dimColor>{truncated}</Text>
}

// Inline diff rendering for tool output — returns colored text
export function renderInlineDiff(output: string, maxLines: number): string {
  const lines = output.split("\n")
  const visible = lines.slice(0, maxLines)
  const hidden = lines.length - maxLines

  let result = visible.join("\n")
  if (hidden > 0) {
    result += `\n... (${hidden} more lines — press e to expand)`
  }
  return result
}
