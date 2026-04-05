// L2: Scroll State Machine — pure, line-based.
// Offset is in rendered lines, not entry indices.
// No React, no Ink, no side effects.

export interface ScrollState {
  // Live-tail following vs explicit reading mode.
  mode: "sticky" | "reading"
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
  return { mode: "sticky", offset: 0, totalLines: 0, viewportSize: 20 }
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
      return { ...state, mode: "sticky", offset: 0 }

    case "contentChanged": {
      if (state.mode === "sticky") {
        return { ...state, totalLines: cmd.newTotalLines, offset: 0 }
      }

      // Preserve viewing position: shift offset by the delta in total lines
      const delta = cmd.newTotalLines - state.totalLines
      return {
        ...state,
        totalLines: cmd.newTotalLines,
        offset: clampOffset(state.offset + delta, cmd.newTotalLines, state.viewportSize),
      }
    }

    case "resize":
      return {
        ...state,
        viewportSize: cmd.viewportSize,
        offset: clampOffset(state.offset, state.totalLines, cmd.viewportSize),
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

  return {
    ...state,
    mode,
    offset,
  }
}

export function isAtBottom(state: ScrollState): boolean {
  return state.offset === 0
}
