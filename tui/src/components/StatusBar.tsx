import React from "react"
import { Box, Text } from "ink"
import type { CycleUpdateParams, DriftUpdateParams } from "../protocol/types.js"

interface Props {
  model: string
  tokensUsed: number
  tokensLimit: number
  mode: "symbiotic" | "autonomous"
  yolo: boolean
  streaming: boolean
  subagents: number
  cycle: CycleUpdateParams | null
  drift: DriftUpdateParams | null
  notification: string | null
  width: number
}

// Single line, paddingX=1, gap=2, dimColor for most text
export function StatusBar(props: Props) {
  const { model, tokensUsed, tokensLimit, mode, yolo, streaming, subagents, cycle, drift, notification, width } = props

  const parts: React.ReactNode[] = [<Text key="model" dimColor>{model}</Text>]

  if (tokensLimit > 0) {
    parts.push(<Text key="tokens" dimColor>{formatTokens(tokensUsed)}/{formatTokens(tokensLimit)}</Text>)
  }

  if (mode === "autonomous") {
    parts.push(<Text key="auto" color="cyanBright" bold>auto</Text>)
  }

  if (yolo) {
    parts.push(<Text key="yolo" color="yellowBright" bold>yolo</Text>)
  }

  if (streaming) {
    parts.push(<Text key="streaming" color="greenBright">stream</Text>)
  }

  if (subagents > 0) {
    parts.push(<Text key="subagents" dimColor>{subagents} agent{subagents > 1 ? "s" : ""}</Text>)
  }

  if (cycle?.problemTitle) {
    parts.push(<Text key="cycle" dimColor>{cycle.phase}: {cycle.problemTitle}</Text>)
  }

  if (drift && drift.drifted > 0) {
    parts.push(<Text key="drift" color="redBright">▲{drift.drifted} drift</Text>)
  }

  return (
    <Box paddingX={1} gap={1} width={width}>
      <Box flexGrow={1} flexShrink={1}>
        {parts.flatMap((part, index) => index === 0 ? [part] : [<Text key={`sep-${index}`} dimColor> ∙ </Text>, part])}
      </Box>
      {notification && <Text color="cyanBright">{notification}</Text>}
    </Box>
  )
}

function formatTokens(n: number): string {
  if (n < 1000) return String(n)
  if (n < 1_000_000) return (n / 1000).toFixed(1) + "k"
  return (n / 1_000_000).toFixed(1) + "M"
}
