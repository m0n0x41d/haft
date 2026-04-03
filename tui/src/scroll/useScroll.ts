// L2 React hook: wires L1 input events → L2 scroll reducer.
// Accepts per-entry heights (line counts) and computes the visible entry window.

import { useReducer, useEffect, useCallback, useMemo } from "react"
import type { EventEmitter } from "node:events"
import type { InputEvent } from "../terminal/input.js"
import { initialScroll, reduceScroll, isAtBottom, type ScrollState, type ScrollCommand } from "./state.js"
import { computeVisibleWindow, type VisibleWindow } from "./measure.js"

export interface UseScrollResult {
  state: ScrollState
  scroll: (cmd: ScrollCommand) => void
  visibleWindow: VisibleWindow
  isAtBottom: boolean
}

export function useScroll(
  inputEvents: EventEmitter | null,
  entryHeights: readonly number[],
  viewportSize: number,
): UseScrollResult {
  const [state, dispatch] = useReducer(
    (s: ScrollState, cmd: ScrollCommand) => reduceScroll(s, cmd),
    initialScroll(),
  )

  // Derive total lines from entry heights (cheap O(n), n = transcript length)
  const totalLines = entryHeights.reduce((a, b) => a + b, 0)

  // Sync total line count into scroll state
  useEffect(() => {
    dispatch({ type: "contentChanged", newTotalLines: totalLines })
  }, [totalLines])

  // Sync viewport size changes
  useEffect(() => {
    dispatch({ type: "resize", viewportSize })
  }, [viewportSize])

  // Route mouse wheel events from L1
  useEffect(() => {
    if (!inputEvents) return
    const handler = (ev: InputEvent) => {
      if (ev.type === "wheelUp") dispatch({ type: "wheelUp" })
      else if (ev.type === "wheelDown") dispatch({ type: "wheelDown" })
    }
    inputEvents.on("input", handler)
    return () => { inputEvents.off("input", handler) }
  }, [inputEvents])

  const scroll = useCallback((cmd: ScrollCommand) => dispatch(cmd), [])

  // Compute visible entry window from current heights and scroll offset
  const visibleWindow = useMemo(
    () => computeVisibleWindow(entryHeights, state.offset, state.viewportSize),
    [entryHeights, state.offset, state.viewportSize],
  )

  return {
    state,
    scroll,
    visibleWindow,
    isAtBottom: isAtBottom(state),
  }
}
