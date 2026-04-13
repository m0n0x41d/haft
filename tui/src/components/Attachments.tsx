import React, { useEffect, useState } from "react"
import { Box, Text } from "ink"
import { useInput } from "../hooks/useInput.js"
import {
  buildAttachmentRows,
  clampAttachmentCursor,
  moveAttachmentCursor,
  type AttachmentItem,
} from "./attachmentLayout.js"

interface Props {
  items: AttachmentItem[]
  onRemove: (id: number) => void
  selectionMode: boolean
  onExitSelection: () => void
  width: number
}

// Renders attachments above the input prompt
// Selection mode: arrow keys to navigate, Delete to remove, Esc to cancel
export function Attachments({ items, onRemove, selectionMode, onExitSelection, width }: Props) {
  const [cursor, setCursor] = useState(0)
  const rows = buildAttachmentRows({
    items,
    selectionMode,
    selectedIndex: cursor,
    width,
  })

  useEffect(() => {
    setCursor((value) => clampAttachmentCursor(value, items.length))
  }, [items.length])

  useInput((_input, key) => {
    if (key.rightArrow) {
      setCursor((value) => moveAttachmentCursor(value, "right", items.length))
      return
    }
    if (key.leftArrow) {
      setCursor((value) => moveAttachmentCursor(value, "left", items.length))
      return
    }
    if (key.delete || key.backspace) {
      if (items.length > 0) {
        onRemove(items[cursor].id)
        setCursor((value) => clampAttachmentCursor(value, items.length - 1))
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
    <Box flexDirection="column" width={width}>
      {rows.map((row, rowIndex) => (
        <Box key={`${row.type}-${rowIndex}`} paddingX={1} width={width}>
          {row.type === "items" && row.items.map((item, itemIndex) => (
            <React.Fragment key={item.id}>
              {itemIndex > 0 && <Text> </Text>}
              {item.selected ? (
                <Text inverse bold>{item.label}</Text>
              ) : (
                <Text dimColor>{item.label}</Text>
              )}
            </React.Fragment>
          ))}
          {row.type === "hint" && <Text dimColor>{row.text}</Text>}
        </Box>
      ))}
    </Box>
  )
}
