import React from "react"
import { Box, Text } from "ink"
import type { CycleUpdateParams, DriftUpdateParams } from "../protocol/types.js"

interface Props {
  model: string
  tokensUsed: number
  tokensLimit: number
  mode: "symbiotic" | "autonomous"
  streaming: boolean
  subagents: number
  cycle: CycleUpdateParams | null
  drift: DriftUpdateParams | null
  notification: string | null
  width: number
}

// Single line, paddingX=1, gap=2, dimColor for most text
export function StatusBar(props: Props) {
  const { model, tokensUsed, tokensLimit, mode, streaming, subagents, cycle, drift, notification, width } = props

  // Build status text parts — CC renders this as a single dimColor Text
  const parts: string[] = []

  // Model
  parts.push(model)

  // Tokens
  if (tokensLimit > 0) {
    parts.push(`${formatTokens(tokensUsed)}/${formatTokens(tokensLimit)}`)
  }

  // Mode (only show if not default)
  if (mode === "autonomous") {
    parts.push("auto")
  }

  // Subagents
  if (subagents > 0) {
    parts.push(`${subagents} agent${subagents > 1 ? "s" : ""}`)
  }

  // Cycle
  if (cycle?.problemTitle) {
    parts.push(`${cycle.phase}: ${cycle.problemTitle}`)
  }

  // Drift
  if (drift && drift.drifted > 0) {
    parts.push(`\u25B2${drift.drifted} drift`)
  }

  const statusText = parts.join(" \u2219 ")

  return (
    <Box paddingX={1} gap={2} width={width}>
      <Box flexGrow={1} flexShrink={1}>
        <Text dimColor wrap="truncate-end">{statusText}</Text>
      </Box>
      {notification && <Text dimColor>{notification}</Text>}
    </Box>
  )
}

function formatTokens(n: number): string {
  if (n < 1000) return String(n)
  if (n < 1_000_000) return (n / 1000).toFixed(1) + "k"
  return (n / 1_000_000).toFixed(1) + "M"
}
