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
  let renderedFindings = 0

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
    renderedFindings++

    for (const driftItem of finding.driftItems ?? []) {
      if (lines.length >= MAX_OVERSEER_LINES) {
        break
      }

      lines.push({
        text: formatDriftLine(driftItem),
        tone: "detail",
      })
    }
  }

  const hiddenFindings = findings.length - renderedFindings
  if (hiddenFindings > 0 && lines.length >= MAX_OVERSEER_LINES) {
    lines[MAX_OVERSEER_LINES - 1] = {
      text: `… +${hiddenFindings} more overseer finding${hiddenFindings === 1 ? "" : "s"}`,
      tone: "detail",
    }
  }

  return lines
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
