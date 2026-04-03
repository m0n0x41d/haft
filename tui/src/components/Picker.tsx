import React, { useState, useMemo } from "react"
import { Box, Text } from "ink"
import { useInput } from "../hooks/useInput.js"

export interface PickerItem {
  id: string
  label: string
  desc?: string
}

interface Props {
  title: string
  items: PickerItem[]
  onSelect: (item: PickerItem) => void
  onCancel: () => void
  width: number
}

export function Picker({ title, items, onSelect, onCancel, width }: Props) {
  const [filter, setFilter] = useState("")
  const [cursor, setCursor] = useState(0)

  const filtered = useMemo(() => {
    if (!filter) return items
    const lower = filter.toLowerCase()
    return items.filter(
      (item) =>
        item.label.toLowerCase().includes(lower) ||
        item.id.toLowerCase().includes(lower) ||
        (item.desc?.toLowerCase().includes(lower) ?? false)
    )
  }, [items, filter])

  const maxVisible = 15

  useInput((input, key) => {
    if (key.escape) {
      onCancel()
      return
    }
    if (key.return) {
      if (filtered.length > 0 && cursor < filtered.length) {
        onSelect(filtered[cursor])
      }
      return
    }
    if (key.upArrow) {
      setCursor((c) => Math.max(0, c - 1))
      return
    }
    if (key.downArrow) {
      setCursor((c) => Math.min(filtered.length - 1, c + 1))
      return
    }
    if (key.backspace || key.delete) {
      setFilter((f) => f.slice(0, -1))
      setCursor(0)
      return
    }
    if (input && !key.ctrl && !key.meta) {
      setFilter((f) => f + input)
      setCursor(0)
    }
  })

  const boxWidth = Math.min(width - 4, 60)

  return (
    <Box
      flexDirection="column"
      borderStyle="round"
      borderColor="blue"
      paddingX={2}
      paddingY={1}
      width={boxWidth}
    >
      <Text color="blue" bold>{title}</Text>

      {/* Filter */}
      <Box marginTop={1}>
        {filter ? (
          <Text><Text color="blue">/ {filter}</Text><Text>_</Text></Text>
        ) : (
          <Text dimColor>Type to filter</Text>
        )}
      </Box>

      {/* Items */}
      <Box flexDirection="column" marginTop={1}>
        {filtered.length === 0 ? (
          <Text dimColor>No matches</Text>
        ) : (
          filtered.slice(0, maxVisible).map((item, i) => (
            <Box key={item.id}>
              <Text color={i === cursor ? "blue" : undefined}>
                {i === cursor ? "> " : "  "}
              </Text>
              <Text bold={i === cursor} color={i === cursor ? "white" : undefined}>
                {item.label}
              </Text>
              {item.desc && (
                <Text dimColor> {truncate(item.desc, boxWidth - item.label.length - 6)}</Text>
              )}
            </Box>
          ))
        )}
        {filtered.length > maxVisible && (
          <Text dimColor>  ... and {filtered.length - maxVisible} more</Text>
        )}
      </Box>

      <Box marginTop={1}>
        <Text dimColor>Esc cancel · ↑↓ navigate · Enter select</Text>
      </Box>
    </Box>
  )
}

function truncate(s: string, max: number): string {
  if (max < 4) return ""
  if (s.length <= max) return s
  return s.slice(0, max - 3) + "..."
}
