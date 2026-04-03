// L2: Selection State Machine — pure.
// Tracks mouse-based text selection in terminal coordinates (1-based, SGR format).

export type Point = { col: number; row: number }

export interface SelectionState {
  anchor: Point | null   // mouse-down position
  focus: Point | null    // current drag position
  isDragging: boolean
}

export type SelectionAction =
  | { type: "mouseDown"; col: number; row: number }
  | { type: "mouseDrag"; col: number; row: number }
  | { type: "mouseUp" }
  | { type: "clear" }

export const INITIAL_SELECTION: SelectionState = {
  anchor: null,
  focus: null,
  isDragging: false,
}

export function reduceSelection(state: SelectionState, action: SelectionAction): SelectionState {
  switch (action.type) {
    case "mouseDown":
      return {
        anchor: { col: action.col, row: action.row },
        focus: null,
        isDragging: true,
      }

    case "mouseDrag": {
      if (!state.isDragging || !state.anchor) return state
      // Anti-tremor: require at least 1 cell of movement before starting selection
      if (!state.focus) {
        if (action.col === state.anchor.col && action.row === state.anchor.row) return state
      }
      return { ...state, focus: { col: action.col, row: action.row } }
    }

    case "mouseUp":
      return { ...state, isDragging: false }

    case "clear":
      return INITIAL_SELECTION
  }
}

export function hasSelection(state: SelectionState): boolean {
  return state.anchor !== null && state.focus !== null
}

// Normalize selection to reading order (top-left to bottom-right).
export function normalizedRange(state: SelectionState): { start: Point; end: Point } | null {
  if (!state.anchor || !state.focus) return null
  const a = state.anchor
  const f = state.focus
  const aFirst = a.row < f.row || (a.row === f.row && a.col <= f.col)
  return aFirst ? { start: a, end: f } : { start: f, end: a }
}
