// Pure cursor-aware text editing. All functions: EditState -> EditState.
// Cursor is a 0-based index — it sits BEFORE the character at that index.

import {
  findNextGraphemeBoundary,
  findPreviousGraphemeBoundary,
  normalizeGraphemeBoundaryLeft,
  segmentGraphemes,
} from "./graphemes.js"

export type EditState = {
  readonly text: string
  readonly cursor: number
}

// --- Construction ---

export const empty: EditState = { text: "", cursor: 0 }

export const fromText = (text: string): EditState => ({
  text,
  cursor: text.length,
})

// --- Insertion ---

export const insertAt = (s: EditState, str: string): EditState => ({
  text: s.text.slice(0, s.cursor) + str + s.text.slice(s.cursor),
  cursor: s.cursor + str.length,
})

// --- Deletion ---

export const deleteBack = (s: EditState): EditState =>
  currentBoundary(s.text, s.cursor) === 0
    ? s
    : {
        text: s.text.slice(0, previousBoundary(s.text, s.cursor))
          + s.text.slice(currentBoundary(s.text, s.cursor)),
        cursor: previousBoundary(s.text, s.cursor),
      }

export const deleteForward = (s: EditState): EditState =>
  currentBoundary(s.text, s.cursor) >= s.text.length
    ? s
    : {
        text: s.text.slice(0, currentBoundary(s.text, s.cursor))
          + s.text.slice(nextBoundary(s.text, s.cursor)),
        cursor: currentBoundary(s.text, s.cursor),
      }

const isWordChar = (ch: string): boolean =>
  ch !== " " && ch !== "\n" && ch !== "\t"

export const deleteWordBack = (s: EditState): EditState => {
  if (s.cursor === 0) return s
  let i = s.cursor
  while (i > 0 && !isWordChar(s.text[i - 1]!)) i--
  while (i > 0 && isWordChar(s.text[i - 1]!)) i--
  return { text: s.text.slice(0, i) + s.text.slice(s.cursor), cursor: i }
}

// --- Movement ---

export const moveLeft = (s: EditState): EditState =>
  currentBoundary(s.text, s.cursor) === 0
    ? s
    : { ...s, cursor: previousBoundary(s.text, s.cursor) }

export const moveRight = (s: EditState): EditState =>
  currentBoundary(s.text, s.cursor) >= s.text.length
    ? s
    : { ...s, cursor: nextBoundary(s.text, s.cursor) }

export const moveHome = (s: EditState): EditState => {
  const lineStart = s.text.lastIndexOf("\n", s.cursor - 1) + 1
  return { ...s, cursor: lineStart }
}

export const moveEnd = (s: EditState): EditState => {
  const lineEnd = s.text.indexOf("\n", s.cursor)
  return { ...s, cursor: lineEnd === -1 ? s.text.length : lineEnd }
}

export const moveWordLeft = (s: EditState): EditState => {
  if (s.cursor === 0) return s
  let i = s.cursor
  while (i > 0 && !isWordChar(s.text[i - 1]!)) i--
  while (i > 0 && isWordChar(s.text[i - 1]!)) i--
  return { ...s, cursor: i }
}

export const moveWordRight = (s: EditState): EditState => {
  if (s.cursor >= s.text.length) return s
  let i = s.cursor
  while (i < s.text.length && isWordChar(s.text[i]!)) i++
  while (i < s.text.length && !isWordChar(s.text[i]!)) i++
  return { ...s, cursor: i }
}

// --- Query ---

export const cursorPosition = (
  s: EditState,
): { line: number; col: number } => {
  const before = s.text.slice(0, currentBoundary(s.text, s.cursor))
  const lines = before.split("\n")
  const currentLine = lines[lines.length - 1] ?? ""
  const col = segmentGraphemes(currentLine)
    .reduce((width, grapheme) => width + grapheme.width, 0)

  return { line: lines.length - 1, col }
}

function currentBoundary(
  text: string,
  cursor: number,
): number {
  return normalizeGraphemeBoundaryLeft(text, cursor)
}

function previousBoundary(
  text: string,
  cursor: number,
): number {
  return findPreviousGraphemeBoundary(text, currentBoundary(text, cursor))
}

function nextBoundary(
  text: string,
  cursor: number,
): number {
  return findNextGraphemeBoundary(text, currentBoundary(text, cursor))
}
