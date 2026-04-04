const INPUT_PROMPT_PREFIX = "\u276F "
const INPUT_CONTINUATION_PREFIX = "  "
const INPUT_HORIZONTAL_PADDING = 2
const INPUT_PREFIX_COLUMNS = INPUT_PROMPT_PREFIX.length

type CursorLocation = {
  line: number
  column: number
}

export interface InputVisualRow {
  prefix: string
  text: string
  cursorColumn: number | null
}

export interface InputLayout {
  contentWidth: number
  rows: InputVisualRow[]
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
  const layout = buildInputLayout(text, cursor, width)

  return layout.rows.length
}

function buildLogicalLineRows(
  line: string,
  lineIndex: number,
  cursorLocation: CursorLocation,
  contentWidth: number,
): InputVisualRow[] {
  const segments = wrapLogicalLine(line, contentWidth)
  const rows = segments.map((segment, segmentIndex) => ({
    prefix: getRowPrefix(lineIndex, segmentIndex),
    text: segment,
    cursorColumn: getCursorColumn(
      line,
      lineIndex,
      segmentIndex,
      segments.length,
      cursorLocation,
      contentWidth,
    ),
  }))
  const trailingCursorRows = needsTrailingCursorRow(
    line,
    lineIndex,
    cursorLocation,
    contentWidth,
  )
    ? [{
        prefix: INPUT_CONTINUATION_PREFIX,
        text: "",
        cursorColumn: 0,
      }]
    : []

  return [...rows, ...trailingCursorRows]
}

function wrapLogicalLine(
  line: string,
  contentWidth: number,
): string[] {
  if (line.length === 0) {
    return [""]
  }

  const segments: string[] = []
  let start = 0

  while (start < line.length) {
    const end = start + contentWidth

    segments.push(line.slice(start, end))
    start = end
  }

  return segments
}

function getCursorColumn(
  line: string,
  lineIndex: number,
  segmentIndex: number,
  segmentCount: number,
  cursorLocation: CursorLocation,
  contentWidth: number,
): number | null {
  if (lineIndex !== cursorLocation.line) {
    return null
  }

  const segmentStart = segmentIndex * contentWidth
  const isLastSegment = segmentIndex === segmentCount - 1
  const segmentLimit = isLastSegment
    ? line.length
    : segmentStart + contentWidth

  if (cursorLocation.column < segmentStart) {
    return null
  }

  if (cursorLocation.column < segmentLimit) {
    return cursorLocation.column - segmentStart
  }

  if (!isLastSegment) {
    return null
  }

  if (line.length < segmentStart + contentWidth) {
    return segmentLimit - segmentStart
  }

  return null
}

function needsTrailingCursorRow(
  line: string,
  lineIndex: number,
  cursorLocation: CursorLocation,
  contentWidth: number,
): boolean {
  if (lineIndex !== cursorLocation.line) {
    return false
  }

  if (line.length === 0) {
    return false
  }

  return cursorLocation.column === line.length
    && line.length % contentWidth === 0
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
    column: currentLine.length,
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
