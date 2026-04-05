import React, { useState, useImperativeHandle, forwardRef, useRef, useEffect } from "react"
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
  moveUp,
  moveDown,
  moveWordLeft,
  moveWordRight,
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
import { segmentGraphemes } from "../input/graphemes.js"
import {
  buildInputDisplayLayout,
  type InputDisplayRow,
  type InputVisualRow,
} from "./inputLayout.js"
import {
  hasSubmittableText,
  type PromptSubmission,
} from "./promptSubmission.js"

interface Props {
  phase: "input" | "streaming" | "permission" | "question" | "picker"
  onSubmit: (text: string) => void
  onAtMention?: () => void
  onSlashCommand?: () => void
  onPopQueue?: () => PromptSubmission | null
  onEnterAttachmentSelection?: () => void
  onPasteImage?: (path: string) => void
  onTerminalScroll?: (direction: "up" | "down", amount?: number) => void
  hasAttachments?: boolean
  width: number
  hasQueuedMessages?: boolean
  onRowsChange?: (rows: number) => void
  onTextChange?: (text: string) => void
}

export interface InputAreaHandle {
  insert: (text: string) => void
  getValue: () => string
}

export const InputArea = React.memo(forwardRef<InputAreaHandle, Props>(function InputArea(
  { phase, onSubmit, onAtMention, onSlashCommand, onPopQueue, onEnterAttachmentSelection, onPasteImage, onTerminalScroll, hasAttachments, width, hasQueuedMessages, onRowsChange, onTextChange },
  ref,
) {
  const [edit, setEdit] = useState<EditState>(empty)
  const historyRef = useRef<History>(emptyHistory)
  const isVisible = phase === "input" || phase === "streaming"
  const layout = isVisible
    ? buildInputDisplayLayout({
        text: edit.text,
        cursor: edit.cursor,
        width,
        hasQueuedMessages: hasQueuedMessages ?? false,
      })
    : null
  const rows = layout?.rows ?? []
  const visualRows = rows.length

  useImperativeHandle(ref, () => ({
    insert(text: string) { setEdit((s) => insertAt(s, text)) },
    getValue() { return edit.text },
  }), [edit.text])

  useEffect(() => {
    onRowsChange?.(visualRows)
  }, [visualRows, onRowsChange])

  useEffect(() => {
    onTextChange?.(edit.text)
  }, [edit.text, onTextChange])

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
      if (hasSubmittableText(edit.text)) {
        historyRef.current = push(historyRef.current, edit.text)
        onSubmit(edit.text)
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
      const nextEdit = moveUp(edit)

      if (nextEdit.cursor !== edit.cursor) {
        setEdit(moveUp)
        return
      }

      // Empty input: attachments > queue > history
      if (!edit.text) {
        if (hasAttachments && onEnterAttachmentSelection) {
          onEnterAttachmentSelection()
          return
        }
        if (hasQueuedMessages && onPopQueue) {
          const queued = onPopQueue()
          if (queued) {
            setEdit(fromText(queued.text))
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
      const nextEdit = moveDown(edit)

      if (nextEdit.cursor !== edit.cursor) {
        setEdit(moveDown)
        return
      }

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
  }, { isActive: isVisible })

  if (!isVisible) return null

  return (
    <Box flexDirection="column" paddingX={1} width={width}>
      {rows.map((row, i) => (
        <Box key={i} width={width}>
          {renderInputRow(row)}
        </Box>
      ))}
    </Box>
  )
}))

function renderInputRow(row: InputDisplayRow): React.ReactNode {
  if (row.kind === "editor") {
    return renderEditorRow(row.row)
  }

  if (row.kind === "placeholder") {
    return (
      <>
        <Text>{row.prefix}</Text>
        <Text inverse> </Text>
        {row.hint.length > 0 && <Text dimColor>{row.hint}</Text>}
      </>
    )
  }

  return (
    <>
      <Text>{row.prefix}</Text>
      <Text dimColor>{row.text}</Text>
    </>
  )
}

function renderEditorRow(row: InputVisualRow): React.ReactNode {
  const cursorOffset = row.cursorOffset

  if (cursorOffset === null) {
    return (
      <>
        <Text>{row.prefix}</Text>
        <Text>{row.text}</Text>
      </>
    )
  }

  const beforeCursor = row.text.slice(0, cursorOffset)
  const afterCursorBoundary = row.text.slice(cursorOffset)
  const cursorChar = segmentGraphemes(afterCursorBoundary)[0]?.text
    ?? " "
  const afterCursor = afterCursorBoundary.length > 0
    ? afterCursorBoundary.slice(cursorChar.length)
    : ""

  return (
    <>
      <Text>{row.prefix}</Text>
      {beforeCursor.length > 0 && <Text>{beforeCursor}</Text>}
      <Text inverse>{cursorChar}</Text>
      {afterCursor.length > 0 && <Text>{afterCursor}</Text>}
    </>
  )
}

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
