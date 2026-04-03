// Pure cursor-aware text editing. All functions: EditState -> EditState.
// Cursor is a 0-based index — it sits BEFORE the character at that index.

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
  s.cursor === 0
    ? s
    : {
        text: s.text.slice(0, s.cursor - 1) + s.text.slice(s.cursor),
        cursor: s.cursor - 1,
      }

export const deleteForward = (s: EditState): EditState =>
  s.cursor >= s.text.length
    ? s
    : {
        text: s.text.slice(0, s.cursor) + s.text.slice(s.cursor + 1),
        cursor: s.cursor,
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
  s.cursor === 0 ? s : { ...s, cursor: s.cursor - 1 }

export const moveRight = (s: EditState): EditState =>
  s.cursor >= s.text.length ? s : { ...s, cursor: s.cursor + 1 }

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
  const before = s.text.slice(0, s.cursor)
  const lines = before.split("\n")
  return { line: lines.length - 1, col: lines[lines.length - 1]!.length }
}
