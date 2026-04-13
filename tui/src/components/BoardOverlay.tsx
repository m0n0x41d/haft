import React, { useState, useEffect, useCallback } from "react"
import { Box, Text } from "ink"
import { useInput } from "../hooks/useInput.js"
import type { JsonRpcClient } from "../protocol/client.js"

const VIEW_NAMES = ["Overview", "Decisions", "Problems", "Coverage", "Evidence"] as const
const VIEW_KEYS = ["overview", "decisions", "problems", "coverage", "evidence"] as const

interface Props {
  client: JsonRpcClient
  onClose: () => void
  width: number
  height: number
}

export function BoardOverlay({ client, onClose, width, height }: Props) {
  const [currentView, setCurrentView] = useState(0)
  const [boardText, setBoardText] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const loadView = useCallback((viewIndex: number) => {
    setLoading(true)
    setError(null)
    client.request<{ text: string }>("board", { view: VIEW_KEYS[viewIndex] })
      .then((r) => {
        setBoardText(r.text || "No data.")
        setLoading(false)
      })
      .catch((e: Error) => {
        setError(e.message)
        setLoading(false)
      })
  }, [client])

  useEffect(() => {
    loadView(currentView)
  }, [currentView, loadView])

  useInput((input, key) => {
    if (key.escape || input === "q" || input === "Q") {
      onClose()
      return
    }
    if (input === "r" || input === "R") {
      loadView(currentView)
      return
    }
    if (input >= "1" && input <= "5") {
      setCurrentView(Number(input) - 1)
      return
    }
    if (key.leftArrow) {
      setCurrentView((v) => Math.max(0, v - 1))
      return
    }
    if (key.rightArrow) {
      setCurrentView((v) => Math.min(4, v + 1))
      return
    }
  })

  // Split board text into lines for display
  const contentLines = (boardText || "").split("\n")
  const headerRows = 3 // tab bar + separator + footer
  const maxContentRows = height - headerRows - 2 // padding

  return (
    <Box
      flexDirection="column"
      width={width}
      height={height}
      borderStyle="round"
      borderColor="cyan"
    >
      {/* Tab bar */}
      <Box flexShrink={0}>
        {VIEW_NAMES.map((name, i) => (
          <Text key={name}>
            {" "}
            {i === currentView
              ? <Text bold color="cyan">[{i + 1} {name}]</Text>
              : <Text dimColor> {i + 1} {name} </Text>
            }
          </Text>
        ))}
      </Box>

      {/* Separator */}
      <Text dimColor>{"\u2500".repeat(Math.max(0, width - 2))}</Text>

      {/* Content */}
      <Box flexDirection="column" flexGrow={1} paddingX={1} overflowY="hidden">
        {loading && <Text color="yellow">Loading...</Text>}
        {error && <Text color="red">Error: {error}</Text>}
        {!loading && !error && contentLines.slice(0, maxContentRows).map((line, i) => (
          <Text key={i}>{line}</Text>
        ))}
      </Box>

      {/* Footer */}
      <Text dimColor> 1-5 switch · ←→ navigate · r refresh · q/Esc close</Text>
    </Box>
  )
}
