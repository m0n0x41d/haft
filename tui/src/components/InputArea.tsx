import React, { useState, useImperativeHandle, forwardRef } from "react"
import { Box, Text, useInput } from "ink"

interface Props {
  phase: "input" | "streaming" | "permission" | "question" | "picker"
  onSubmit: (text: string) => void
  onAtMention?: () => void
  onSlashCommand?: () => void
  onPopQueue?: () => string[] | null
  onEnterAttachmentSelection?: () => void
  onPasteImage?: (path: string) => void
  onTerminalScroll?: (direction: "up" | "down", amount?: number) => void
  hasAttachments?: boolean
  width: number
  hasQueuedMessages?: boolean
}

export interface InputAreaHandle {
  insert: (text: string) => void
  getValue: () => string
}

// Prompt input: ❯ prefix, inline text
// Paste: handled by terminal (Cmd+V sends text through stdin).
// Image paste: Ctrl+V → async clipboard check.
// Text paste: Cmd+V (terminal sends raw chars, no special handling needed).
// Image paste: Ctrl+V keybinding → async clipboard check.
export const InputArea = React.memo(forwardRef<InputAreaHandle, Props>(function InputArea(
  { phase, onSubmit, onAtMention, onSlashCommand, onPopQueue, onEnterAttachmentSelection, onPasteImage, onTerminalScroll, hasAttachments, width, hasQueuedMessages },
  ref,
) {
  const [value, setValue] = useState("")

  useImperativeHandle(ref, () => ({
    insert(text: string) { setValue((v) => v + text) },
    getValue() { return value },
  }), [value])

  useInput((input, key) => {
    // Allow typing during streaming (for message queuing)
    if (phase !== "input" && phase !== "streaming") return

    if (key.return) {
      if (value.endsWith("\\")) {
        setValue((v) => v.slice(0, -1) + "\n")
        return
      }
      if (key.meta || key.shift) {
        setValue((v) => v + "\n")
        return
      }
      if (value.trim()) {
        onSubmit(value.trim())
        setValue("")
      }
      return
    }

    if (key.ctrl && input === "j") {
      setValue((v) => v + "\n")
      return
    }

    if (key.backspace || key.delete) {
      setValue((v) => v.slice(0, -1))
      return
    }

    if (key.ctrl && input === "u") {
      if (value) {
        setValue("")
        return
      }
    }

    // Ctrl+V: image paste from clipboard (async, never blocks)
    // Text paste goes through terminal directly (Cmd+V sends raw chars).
    if (key.ctrl && input === "v") {
      if (onPasteImage) void checkClipboardImage(onPasteImage)
      return
    }

    if (key.ctrl && input === "w") {
      setValue((v) => {
        const trimmed = v.trimEnd()
        const lastSpace = trimmed.lastIndexOf(" ")
        return lastSpace >= 0 ? v.slice(0, lastSpace + 1) : ""
      })
      return
    }

    // Up arrow (only when input is empty): attachment selection → queue pop → history
    if (key.upArrow && !value) {
      if (hasAttachments && onEnterAttachmentSelection) {
        onEnterAttachmentSelection()
        return
      }
      if (hasQueuedMessages && onPopQueue) {
        const queued = onPopQueue()
        if (queued && queued.length > 0) {
          const merged = [...queued, value].filter(Boolean).join("\n")
          setValue(merged)
        }
        return
      }
      return
    }

    // Regular input (printable characters)
    if (input && !key.ctrl && !key.meta) {
      // Mouse sequences that leaked through — route to scroll
      const mouseMatch = /^(?:\x1b)?\[<(\d+);(\d+);(\d+)([Mm])$/.exec(input)
      if (mouseMatch) {
        const button = Number.parseInt(mouseMatch[1] ?? "", 10)
        if (button === 64 || button === 65) {
          onTerminalScroll?.(button === 64 ? "up" : "down", 3)
          return
        }
      }
      // @ triggers file picker
      if (input === "@" && onAtMention) {
        onAtMention()
        return
      }
      // / at start triggers command picker
      if (input === "/" && value === "" && onSlashCommand) {
        onSlashCommand()
        return
      }
      setValue((v) => v + input)
    }
  }, { isActive: phase === "input" || phase === "streaming" })

  if (phase !== "input" && phase !== "streaming") return null

  // Empty: ❯ + cursor block
  if (!value) {
    return (
      <Box paddingX={1}>
        <Text>{"\u276F"} </Text>
        <Text inverse> </Text>
        {hasQueuedMessages && <Text dimColor>  Press up to edit queued messages</Text>}
      </Box>
    )
  }

  // Multiline input
  const lines = value.split("\n")
  return (
    <Box flexDirection="column" paddingX={1}>
      {lines.map((line, i) => (
        <Box key={i}>
          {i === 0 ? (
            <Text>{"\u276F"} </Text>
          ) : (
            <Text>{"  "}</Text>
          )}
          <Text>{line}</Text>
          {i === lines.length - 1 && <Text inverse> </Text>}
        </Box>
      ))}
    </Box>
  )
}))

// Async clipboard image check — called on Ctrl+V.
// Uses getImageFromClipboard (spawns osascript in child process).
import { getImageFromClipboard } from "../terminal/clipboard.js"

async function checkClipboardImage(onPasteImage: (path: string) => void) {
  try {
    const img = await getImageFromClipboard()
    if (img) onPasteImage(img.tempPath)
  } catch {}
}
