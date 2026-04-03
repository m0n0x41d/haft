// L2 React hook: wires L1 input events → L2 scroll reducer.
// Accepts per-entry heights and computes a measured virtual window.

import { useReducer, useEffect, useCallback, useMemo } from "react"
import type { EventEmitter } from "node:events"
import type { InputEvent } from "../terminal/input.js"
import { initialScroll, reduceScroll, isAtBottom, type ScrollState, type ScrollCommand } from "./state.js"
import { computeOffsets, computeVisibleWindow, type VisibleWindow } from "./measure.js"

export interface UseScrollResult {
  state: ScrollState
  scroll: (cmd: ScrollCommand) => void
  entryOffsets: readonly number[]
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

  const entryOffsets = useMemo(
    () => computeOffsets(entryHeights),
    [entryHeights],
  )
  const totalLines = entryOffsets[entryOffsets.length - 1] ?? 0

  useEffect(() => {
    dispatch({ type: "contentChanged", newTotalLines: totalLines })
  }, [totalLines])

  useEffect(() => {
    dispatch({ type: "resize", viewportSize })
  }, [viewportSize])

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

  const visibleWindow = useMemo(
    () => computeVisibleWindow(entryOffsets, state.offset, state.viewportSize),
    [entryOffsets, state.offset, state.viewportSize],
  )

  return {
    state,
    scroll,
    entryOffsets,
    visibleWindow,
    isAtBottom: isAtBottom(state),
  }
}
