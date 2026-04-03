import React, { useState, useImperativeHandle, forwardRef, useRef } from "react"
import { Box, Text } from "ink"
import { trace } from "../debug.js"
import { useInput } from "../hooks/useInput.js"
import {
  type EditState,
  empty,
  fromText,
  insertAt,
  deleteBack,
  deleteForward,
  deleteWordBack,
  moveLeft,
  moveRight,
  moveHome,
  moveEnd,
  moveWordLeft,
  moveWordRight,
  cursorPosition,
} from "../input/editBuffer.js"
import {
  type History,
  emptyHistory,
  push,
  navigateUp,
  navigateDown,
  currentText,
  isNavigating,
} from "../input/history.js"

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

export const InputArea = React.memo(forwardRef<InputAreaHandle, Props>(function InputArea(
  { phase, onSubmit, onAtMention, onSlashCommand, onPopQueue, onEnterAttachmentSelection, onPasteImage, onTerminalScroll, hasAttachments, width, hasQueuedMessages },
  ref,
) {
  const [edit, setEdit] = useState<EditState>(empty)
  const historyRef = useRef<History>(emptyHistory)

  useImperativeHandle(ref, () => ({
    insert(text: string) { setEdit((s) => insertAt(s, text)) },
    getValue() { return edit.text },
  }), [edit.text])

  useInput((input, key) => {
    if (phase !== "input" && phase !== "streaming") return

    // --- Submit ---
    if (key.return) {
      if (edit.text.endsWith("\\")) {
        setEdit((s) => {
          const newText = s.text.slice(0, -1) + "\n"
          return { text: newText, cursor: Math.min(s.cursor, newText.length) }
        })
        return
      }
      if (key.meta || key.shift) {
        setEdit((s) => insertAt(s, "\n"))
        return
      }
      if (edit.text.trim()) {
        historyRef.current = push(historyRef.current, edit.text.trim())
        onSubmit(edit.text.trim())
        setEdit(empty)
      }
      return
    }

    // --- Newline ---
    if (key.ctrl && input === "j") {
      setEdit((s) => insertAt(s, "\n"))
      return
    }

    // --- Deletion ---
    // Alt+Backspace / Alt+Delete: delete word backward
    if ((key.backspace || key.delete) && key.meta) {
      setEdit(deleteWordBack)
      return
    }
    // Ctrl+W: delete word backward
    if (key.ctrl && input === "w") {
      setEdit(deleteWordBack)
      return
    }
    // Backspace (0x7f from terminal, or 0x08 from some terminals)
    if (key.backspace) {
      setEdit(deleteBack)
      return
    }
    // Forward Delete key (\x1b[3~ escape sequence)
    if (key.delete) {
      setEdit(deleteForward)
      return
    }

    // --- Clear ---
    if (key.ctrl && input === "u") {
      if (edit.text) { setEdit(empty); return }
    }

    // --- Clipboard ---
    if (key.ctrl && input === "v") {
      trace("ctrl-v: starting clipboard check")
      if (onPasteImage) void checkClipboardImage(onPasteImage)
      return
    }

    // --- Movement ---
    // Home / Ctrl+A: start of line
    if (key.home || (key.ctrl && input === "a")) {
      setEdit(moveHome)
      return
    }
    // End / Ctrl+E: end of line
    if (key.end || (key.ctrl && input === "e")) {
      setEdit(moveEnd)
      return
    }
    // Alt+Left / Ctrl+Left: word left
    if (key.leftArrow && (key.meta || key.ctrl)) {
      setEdit(moveWordLeft)
      return
    }
    // Alt+Right / Ctrl+Right: word right
    if (key.rightArrow && (key.meta || key.ctrl)) {
      setEdit(moveWordRight)
      return
    }
    // Left arrow
    if (key.leftArrow) {
      setEdit(moveLeft)
      return
    }
    // Right arrow
    if (key.rightArrow) {
      setEdit(moveRight)
      return
    }

    // --- History / Up arrow ---
    if (key.upArrow) {
      // Empty input: attachments > queue > history
      if (!edit.text) {
        if (hasAttachments && onEnterAttachmentSelection) {
          onEnterAttachmentSelection()
          return
        }
        if (hasQueuedMessages && onPopQueue) {
          const queued = onPopQueue()
          if (queued && queued.length > 0) {
            setEdit(fromText(queued.join("\n")))
          }
          return
        }
      }
      const result = navigateUp(historyRef.current, edit.text)
      if (result) {
        historyRef.current = result
        setEdit(fromText(currentText(result)))
      }
      return
    }

    // --- History / Down arrow ---
    if (key.downArrow) {
      if (isNavigating(historyRef.current)) {
        const result = navigateDown(historyRef.current)
        if (result) {
          historyRef.current = result
          setEdit(fromText(currentText(result)))
        }
      }
      return
    }

    // --- Regular input ---
    if (input && !key.ctrl && !key.meta) {
      // Mouse sequences that leaked through
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
      if (input === "/" && edit.text === "" && onSlashCommand) {
        onSlashCommand()
        return
      }
      setEdit((s) => insertAt(s, input))
    }
  }, { isActive: phase === "input" || phase === "streaming" })

  if (phase !== "input" && phase !== "streaming") return null

  // --- Render ---
  if (!edit.text) {
    return (
      <Box paddingX={1}>
        <Text>{"\u276F"} </Text>
        <Text inverse> </Text>
        {hasQueuedMessages && <Text dimColor>  Press up to edit queued messages</Text>}
      </Box>
    )
  }

  const { line: cursorLine, col: cursorCol } = cursorPosition(edit)
  const lines = edit.text.split("\n")

  return (
    <Box flexDirection="column" paddingX={1}>
      {lines.map((line, i) => (
        <Box key={i}>
          {i === 0 ? <Text>{"\u276F"} </Text> : <Text>{"  "}</Text>}
          {i === cursorLine ? (
            <>
              <Text>{line.slice(0, cursorCol)}</Text>
              <Text inverse>{cursorCol < line.length ? line[cursorCol] : " "}</Text>
              {cursorCol < line.length && <Text>{line.slice(cursorCol + 1)}</Text>}
            </>
          ) : (
            <Text>{line}</Text>
          )}
        </Box>
      ))}
    </Box>
  )
}))

// Async clipboard image check
import { getImageFromClipboard } from "../terminal/clipboard.js"

async function checkClipboardImage(onPasteImage: (path: string) => void) {
  trace("checkClipboardImage: start")
  try {
    const img = await getImageFromClipboard()
    trace(`checkClipboardImage: result=${img ? "found" : "null"}`)
    if (img) onPasteImage(img.tempPath)
  } catch (e) {
    trace(`checkClipboardImage: error=${e}`)
  }
}
