import {
  normalizeGraphemeBoundaryLeft,
  segmentGraphemes,
} from "../input/graphemes.js"

const INPUT_PROMPT_PREFIX = "\u276F "
const INPUT_CONTINUATION_PREFIX = "  "
const INPUT_HORIZONTAL_PADDING = 2
const INPUT_PREFIX_COLUMNS = INPUT_PROMPT_PREFIX.length
const EMPTY_INPUT_CURSOR = " "
const QUEUED_MESSAGES_HINT = "  Press up to edit queued messages"

type CursorLocation = {
  line: number
  offset: number
}

interface WrappedLineSegment {
  text: string
  start: number
  end: number
  width: number
}

export interface InputVisualRow {
  prefix: string
  text: string
  cursorOffset: number | null
}

export interface InputLayout {
  contentWidth: number
  rows: InputVisualRow[]
}

export type InputDisplayRow =
  | { kind: "editor"; row: InputVisualRow }
  | { kind: "placeholder"; prefix: string; hint: string }
  | { kind: "hint"; prefix: string; text: string }

export interface InputDisplayLayout {
  contentWidth: number
  rows: InputDisplayRow[]
}

interface BuildInputDisplayLayoutOptions {
  text: string
  cursor: number
  width: number
  hasQueuedMessages: boolean
}

export function buildInputLayout(
  text: string,
  cursor: number,
  width: number,
): InputLayout {
  const contentWidth = getInputContentWidth(width)
  const clampedCursor = clampCursor(text, cursor)
  const cursorLocation = getCursorLocation(text, clampedCursor)
  const logicalLines = text.length === 0
    ? [""]
    : text.split("\n")
  const rows = logicalLines.flatMap((line, lineIndex) =>
    buildLogicalLineRows(
      line,
      lineIndex,
      cursorLocation,
      contentWidth,
    ),
  )

  return {
    contentWidth,
    rows,
  }
}

export function measureInputRows(
  text: string,
  cursor: number,
  width: number,
): number {
  return buildInputLayout(text, cursor, width).rows.length
}

export function buildInputDisplayLayout(
  options: BuildInputDisplayLayoutOptions,
): InputDisplayLayout {
  const { text, cursor, width, hasQueuedMessages } = options

  if (text.length === 0) {
    return buildEmptyInputDisplayLayout(width, hasQueuedMessages)
  }

  const layout = buildInputLayout(text, cursor, width)
  const rows = layout.rows
    .map((row) => ({ kind: "editor", row } satisfies InputDisplayRow))

  return {
    contentWidth: layout.contentWidth,
    rows,
  }
}

export function measureInputDisplayRows(
  options: BuildInputDisplayLayoutOptions,
): number {
  return buildInputDisplayLayout(options).rows.length
}

function buildLogicalLineRows(
  line: string,
  lineIndex: number,
  cursorLocation: CursorLocation,
  contentWidth: number,
): InputVisualRow[] {
  const segments = wrapLogicalLine(line, contentWidth)
  const editorRows = segments.map((segment, segmentIndex) => ({
    prefix: getRowPrefix(lineIndex, segmentIndex),
    text: segment.text,
    cursorOffset: getCursorOffset(
      lineIndex,
      segmentIndex,
      segments,
      segment,
      cursorLocation,
    ),
  }))
  const trailingCursorRows = needsTrailingCursorRow(
    lineIndex,
    line.length,
    segments,
    contentWidth,
    cursorLocation,
  )
    ? [{
        prefix: INPUT_CONTINUATION_PREFIX,
        text: "",
        cursorOffset: 0,
      }]
    : []

  return [...editorRows, ...trailingCursorRows]
}

function buildEmptyInputDisplayLayout(
  width: number,
  hasQueuedMessages: boolean,
): InputDisplayLayout {
  const contentWidth = getInputContentWidth(width)
  const hint = hasQueuedMessages
    ? QUEUED_MESSAGES_HINT
    : ""
  const firstRowHintWidth = Math.max(0, contentWidth - EMPTY_INPUT_CURSOR.length)
  const firstRowHint = firstRowHintWidth > 0
    ? (wrapLogicalLine(hint, firstRowHintWidth)[0]?.text ?? "")
    : ""
  const remainingHint = hint.slice(firstRowHint.length)
  const continuationRows = wrapLogicalLine(remainingHint, contentWidth)
    .map((segment) => segment.text)
    .filter((text) => text.length > 0)
    .map((text) => ({
      kind: "hint",
      prefix: INPUT_CONTINUATION_PREFIX,
      text,
    } satisfies InputDisplayRow))
  const rows: InputDisplayRow[] = [
    {
      kind: "placeholder",
      prefix: INPUT_PROMPT_PREFIX,
      hint: firstRowHint,
    },
    ...continuationRows,
  ]

  return {
    contentWidth,
    rows,
  }
}

function wrapLogicalLine(
  line: string,
  contentWidth: number,
): WrappedLineSegment[] {
  if (line.length === 0) {
    return [{
      text: "",
      start: 0,
      end: 0,
      width: 0,
    }]
  }

  const graphemes = segmentGraphemes(line)
  const segments: WrappedLineSegment[] = []
  let segmentStart = 0
  let segmentEnd = 0
  let segmentWidth = 0
  let segmentText = ""

  for (const grapheme of graphemes) {
    const nextWidth = segmentWidth + grapheme.width
    const shouldWrap = segmentText.length > 0 && nextWidth > contentWidth

    if (shouldWrap) {
      segments.push({
        text: segmentText,
        start: segmentStart,
        end: segmentEnd,
        width: segmentWidth,
      })
      segmentStart = grapheme.start
      segmentEnd = grapheme.end
      segmentWidth = grapheme.width
      segmentText = grapheme.text
      continue
    }

    if (segmentText.length === 0) {
      segmentStart = grapheme.start
    }

    segmentEnd = grapheme.end
    segmentWidth = nextWidth
    segmentText += grapheme.text
  }

  segments.push({
    text: segmentText,
    start: segmentStart,
    end: segmentEnd,
    width: segmentWidth,
  })

  return segments
}

function getCursorOffset(
  lineIndex: number,
  segmentIndex: number,
  segments: readonly WrappedLineSegment[],
  segment: WrappedLineSegment,
  cursorLocation: CursorLocation,
): number | null {
  if (lineIndex !== cursorLocation.line) {
    return null
  }

  const isLastSegment = segmentIndex === segments.length - 1
  const lineOffset = cursorLocation.offset

  if (lineOffset < segment.start) {
    return null
  }

  if (lineOffset > segment.end) {
    return null
  }

  if (lineOffset === segment.end && !isLastSegment) {
    return null
  }

  const rowOffset = lineOffset - segment.start

  return normalizeGraphemeBoundaryLeft(segment.text, rowOffset)
}

function needsTrailingCursorRow(
  lineIndex: number,
  lineLength: number,
  segments: readonly WrappedLineSegment[],
  contentWidth: number,
  cursorLocation: CursorLocation,
): boolean {
  if (lineIndex !== cursorLocation.line) {
    return false
  }

  if (lineLength === 0) {
    return false
  }

  if (cursorLocation.offset !== lineLength) {
    return false
  }

  const lastSegment = segments[segments.length - 1]

  return (lastSegment?.width ?? 0) >= contentWidth
}

function getRowPrefix(
  lineIndex: number,
  segmentIndex: number,
): string {
  if (lineIndex === 0 && segmentIndex === 0) {
    return INPUT_PROMPT_PREFIX
  }

  return INPUT_CONTINUATION_PREFIX
}

function getCursorLocation(
  text: string,
  cursor: number,
): CursorLocation {
  const beforeCursor = text.slice(0, cursor)
  const logicalLines = beforeCursor.split("\n")
  const currentLine = logicalLines[logicalLines.length - 1] ?? ""

  return {
    line: logicalLines.length - 1,
    offset: currentLine.length,
  }
}

function clampCursor(
  text: string,
  cursor: number,
): number {
  return Math.max(0, Math.min(cursor, text.length))
}

function getInputContentWidth(width: number): number {
  const reservedColumns = INPUT_HORIZONTAL_PADDING + INPUT_PREFIX_COLUMNS

  return Math.max(1, width - reservedColumns)
}
