// L2: Scroll State Machine — pure.
// No React, no Ink, no side effects.
// One canonical place for all scroll logic.

export interface ScrollState {
  // Lines scrolled back from bottom. 0 = at bottom (sticky).
  offset: number
  // Total number of scrollable items (messages, not rows)
  contentLength: number
  // How many items fit in the viewport
  viewportSize: number
}

export type ScrollCommand =
  | { type: "wheelUp"; amount?: number }
  | { type: "wheelDown"; amount?: number }
  | { type: "pageUp" }
  | { type: "pageDown" }
  | { type: "home" }
  | { type: "end" }
  | { type: "contentGrew"; newLength: number }
  | { type: "resize"; viewportSize: number }

export function initialScroll(): ScrollState {
  return { offset: 0, contentLength: 0, viewportSize: 20 }
}

export function reduceScroll(state: ScrollState, cmd: ScrollCommand): ScrollState {
  const maxOffset = Math.max(0, state.contentLength - 1)

  switch (cmd.type) {
    case "wheelUp":
      return { ...state, offset: Math.min(state.offset + (cmd.amount ?? 3), maxOffset) }

    case "wheelDown":
      return { ...state, offset: Math.max(0, state.offset - (cmd.amount ?? 3)) }

    case "pageUp":
      return { ...state, offset: Math.min(state.offset + state.viewportSize, maxOffset) }

    case "pageDown":
      return { ...state, offset: Math.max(0, state.offset - state.viewportSize) }

    case "home":
      return { ...state, offset: maxOffset }

    case "end":
      return { ...state, offset: 0 }

    case "contentGrew": {
      const wasAtBottom = state.offset === 0
      if (wasAtBottom) {
        // Stay at bottom — offset stays 0
        return { ...state, contentLength: cmd.newLength }
      }
      // Preserve distance from bottom: shift offset by the delta
      const delta = cmd.newLength - state.contentLength
      return {
        ...state,
        contentLength: cmd.newLength,
        offset: Math.min(state.offset + delta, Math.max(0, cmd.newLength - 1)),
      }
    }

    case "resize":
      return { ...state, viewportSize: cmd.viewportSize }
  }
}

export function isAtBottom(state: ScrollState): boolean {
  return state.offset === 0
}

// Compute visible range: [start, end) indices into content array
export function visibleRange(state: ScrollState): { start: number; end: number } {
  const end = state.contentLength - state.offset
  const start = Math.max(0, end - state.viewportSize)
  return { start, end: Math.min(end, state.contentLength) }
}
