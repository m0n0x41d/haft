import React, { useState } from "react"
import { Box, Text } from "ink"
import { useInput } from "../hooks/useInput.js"

export interface AttachmentItem {
  id: number
  name: string
  path: string
  isImage: boolean
  size?: number
}

interface Props {
  items: AttachmentItem[]
  onRemove: (id: number) => void
  selectionMode: boolean
  onExitSelection: () => void
}

// Renders attachments above the input prompt
// Selection mode: arrow keys to navigate, Delete to remove, Esc to cancel
export function Attachments({ items, onRemove, selectionMode, onExitSelection }: Props) {
  const [cursor, setCursor] = useState(0)

  useInput((_input, key) => {
    if (key.rightArrow) {
      setCursor((c) => Math.min(c + 1, items.length - 1))
      return
    }
    if (key.leftArrow) {
      setCursor((c) => Math.max(0, c - 1))
      return
    }
    if (key.delete || key.backspace) {
      if (items.length > 0) {
        onRemove(items[cursor].id)
        setCursor((c) => Math.max(0, c - 1))
      }
      return
    }
    if (key.escape) {
      onExitSelection()
      return
    }
  }, { isActive: selectionMode })

  if (items.length === 0) return null

  return (
    <Box paddingX={1}>
      {items.map((item, i) => {
        const isSelected = selectionMode && i === cursor
        const label = item.isImage
          ? `[Image #${item.id}]`
          : `[${item.name}]`

        return (
          <Box key={item.id} marginRight={1}>
            {isSelected ? (
              <Text inverse bold>{label}</Text>
            ) : (
              <Text dimColor>{label}</Text>
            )}
          </Box>
        )
      })}
      {selectionMode && (
        <Text dimColor>{"\u2192"} to next{" \u00B7 "}Delete to remove{" \u00B7 "}Esc to cancel</Text>
      )}
      {!selectionMode && (
        <Text dimColor>({"\u2191"} to select)</Text>
      )}
    </Box>
  )
}
