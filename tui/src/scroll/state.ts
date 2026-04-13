// L2: Scroll State Machine — pure, line-based.
// Offset is in rendered lines, not entry indices.
// No React, no Ink, no side effects.

export interface ScrollState {
  // Live-tail following vs explicit reading mode.
  mode: "sticky" | "reading"
  // Total rendered lines when the user left sticky mode.
  readingStartTotalLines: number | null
  // Lines scrolled back from bottom. 0 = at bottom (sticky).
  offset: number
  // Total rendered lines across all transcript entries
  totalLines: number
  // Terminal rows available for the chat viewport
  viewportSize: number
}

export type ScrollCommand =
  | { type: "wheelUp"; amount?: number }
  | { type: "wheelDown"; amount?: number }
  | { type: "pageUp" }
  | { type: "pageDown" }
  | { type: "home" }
  | { type: "end" }
  | { type: "contentChanged"; newTotalLines: number }
  | { type: "resize"; viewportSize: number }

export function initialScroll(): ScrollState {
  return {
    mode: "sticky",
    readingStartTotalLines: null,
    offset: 0,
    totalLines: 0,
    viewportSize: 20,
  }
}

function clampOffset(offset: number, totalLines: number, viewportSize: number): number {
  return Math.max(0, Math.min(offset, Math.max(0, totalLines - viewportSize)))
}

export function reduceScroll(state: ScrollState, cmd: ScrollCommand): ScrollState {
  const max = Math.max(0, state.totalLines - state.viewportSize)

  switch (cmd.type) {
    case "wheelUp":
      return moveIntoReadingMode(state, state.offset + (cmd.amount ?? 3))

    case "wheelDown":
      return moveIntoReadingMode(state, state.offset - (cmd.amount ?? 3))

    case "pageUp":
      return moveIntoReadingMode(state, state.offset + state.viewportSize)

    case "pageDown":
      return moveIntoReadingMode(state, state.offset - state.viewportSize)

    case "home":
      return moveIntoReadingMode(state, max)

    case "end":
      return {
        ...state,
        mode: "sticky",
        readingStartTotalLines: null,
        offset: 0,
      }

    case "contentChanged": {
      if (state.mode === "sticky") {
        return {
          ...state,
          readingStartTotalLines: null,
          totalLines: cmd.newTotalLines,
          offset: 0,
        }
      }

      // Preserve viewing position: shift offset by the delta in total lines
      const delta = cmd.newTotalLines - state.totalLines
      const offset = clampOffset(
        state.offset + delta,
        cmd.newTotalLines,
        state.viewportSize,
      )

      return {
        ...state,
        readingStartTotalLines: nextReadingStartTotalLines({
          nextMode: state.mode,
          nextOffset: offset,
          nextTotalLines: cmd.newTotalLines,
          previousMode: state.mode,
          previousTotalLines: state.totalLines,
          previousReadingStartTotalLines: state.readingStartTotalLines,
        }),
        totalLines: cmd.newTotalLines,
        offset,
      }
    }

    case "resize": {
      const offset = clampOffset(
        state.offset,
        state.totalLines,
        cmd.viewportSize,
      )

      return {
        ...state,
        readingStartTotalLines: nextReadingStartTotalLines({
          nextMode: state.mode,
          nextOffset: offset,
          nextTotalLines: state.totalLines,
          previousMode: state.mode,
          previousTotalLines: state.totalLines,
          previousReadingStartTotalLines: state.readingStartTotalLines,
        }),
        viewportSize: cmd.viewportSize,
        offset,
      }
    }
  }
}

function moveIntoReadingMode(
  state: ScrollState,
  nextOffset: number,
): ScrollState {
  const offset = clampOffset(
    nextOffset,
    state.totalLines,
    state.viewportSize,
  )
  const mode = offset > 0 || state.mode === "reading"
    ? "reading"
    : "sticky"
  const readingStartTotalLines = nextReadingStartTotalLines({
    nextMode: mode,
    nextOffset: offset,
    nextTotalLines: state.totalLines,
    previousMode: state.mode,
    previousTotalLines: state.totalLines,
    previousReadingStartTotalLines: state.readingStartTotalLines,
  })

  return {
    ...state,
    mode,
    readingStartTotalLines,
    offset,
  }
}

export function isAtBottom(state: ScrollState): boolean {
  return state.offset === 0
}

export function unreadLinesBelow(state: ScrollState): number {
  if (state.mode !== "reading") {
    return 0
  }

  const baseline = state.readingStartTotalLines ?? state.totalLines

  return Math.max(0, state.totalLines - baseline)
}

interface ReadingStartTotalLinesParams {
  nextMode: ScrollState["mode"]
  nextOffset: number
  nextTotalLines: number
  previousMode: ScrollState["mode"]
  previousTotalLines: number
  previousReadingStartTotalLines: number | null
}

function nextReadingStartTotalLines(
  params: ReadingStartTotalLinesParams,
): number | null {
  if (params.nextMode === "sticky") {
    return null
  }

  if (params.nextOffset === 0) {
    return params.nextTotalLines
  }

  if (params.previousMode === "sticky") {
    return params.previousTotalLines
  }

  return params.previousReadingStartTotalLines
}
