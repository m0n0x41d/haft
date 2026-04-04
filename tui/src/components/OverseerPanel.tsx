import React from "react"
import { Box, Text } from "ink"
import type { OverseerFindingParams, OverseerDriftItemParams } from "../protocol/types.js"

const MAX_OVERSEER_LINES = 4

export interface OverseerLine {
  text: string
  tone: "alert" | "detail"
}

interface Props {
  lines: OverseerLine[]
}

export function OverseerPanel(props: Props) {
  if (props.lines.length === 0) {
    return null
  }

  return (
    <Box flexDirection="column" paddingX={1}>
      {props.lines.map((line, index) => (
        <Text key={index} color={line.tone === "alert" ? "yellow" : undefined} dimColor={line.tone === "detail"} wrap="truncate-end">
          {line.text}
        </Text>
      ))}
    </Box>
  )
}

export function buildOverseerLines(alerts: string[], findings: OverseerFindingParams[]): OverseerLine[] {
  const lines: OverseerLine[] = []

  if (alerts.length > 0) {
    lines.push({
      text: formatAlertLine(alerts),
      tone: "alert",
    })
  }

  for (const finding of findings) {
    if (lines.length >= MAX_OVERSEER_LINES) {
      break
    }

    lines.push({
      text: formatFindingLine(finding),
      tone: "detail",
    })

    for (const driftItem of finding.driftItems ?? []) {
      lines.push({
        text: formatDriftLine(driftItem),
        tone: "detail",
      })
    }
  }

  if (lines.length <= MAX_OVERSEER_LINES) {
    return lines
  }

  const visibleLines = lines.slice(0, MAX_OVERSEER_LINES - 1)
  const hiddenLines = lines.length - visibleLines.length

  visibleLines.push({
    text: `… +${hiddenLines} more overseer detail${hiddenLines === 1 ? "" : "s"}`,
    tone: "detail",
  })

  return visibleLines
}

function formatAlertLine(alerts: string[]): string {
  const visibleAlerts = alerts.slice(0, 2)
  const hiddenAlerts = alerts.length - visibleAlerts.length
  const suffix = hiddenAlerts > 0 ? ` · +${hiddenAlerts} more` : ""

  return `Overseer: ${visibleAlerts.join(" · ")}${suffix}`
}

function formatFindingLine(finding: OverseerFindingParams): string {
  const target = finding.title ?? finding.artifactId ?? finding.type

  return `${target}: ${finding.summary}`
}

function formatDriftLine(driftItem: OverseerDriftItemParams): string {
  const status = driftItem.status.toUpperCase()
  const linesChanged = driftItem.linesChanged ? ` ${driftItem.linesChanged}` : ""
  const invariants = formatInvariantSummary(driftItem.invariants ?? [])

  return `${status} ${driftItem.path}${linesChanged}${invariants}`
}

function formatInvariantSummary(invariants: string[]): string {
  if (invariants.length === 0) {
    return ""
  }

  const visibleInvariants = invariants.slice(0, 2)
  const hiddenInvariants = invariants.length - visibleInvariants.length
  const suffix = hiddenInvariants > 0 ? `; +${hiddenInvariants} more` : ""

  return ` | inv: ${visibleInvariants.join("; ")}${suffix}`
}
