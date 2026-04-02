// L2 React hook: wires L1 input events → L2 scroll reducer.
// Also exposes keyboard scroll commands for useInput integration.

import { useReducer, useEffect, useCallback } from "react"
import type { EventEmitter } from "node:events"
import type { InputEvent } from "../terminal/input.js"
import { initialScroll, reduceScroll, visibleRange, isAtBottom, type ScrollState, type ScrollCommand } from "./state.js"

export interface UseScrollResult {
  state: ScrollState
  scroll: (cmd: ScrollCommand) => void
  visibleRange: { start: number; end: number }
  isAtBottom: boolean
}

export function useScroll(
  inputEvents: EventEmitter | null,
  contentLength: number,
  viewportSize: number,
): UseScrollResult {
  const [state, dispatch] = useReducer(
    (s: ScrollState, cmd: ScrollCommand) => reduceScroll(s, cmd),
    initialScroll(),
  )

  // Sync content length changes
  useEffect(() => {
    dispatch({ type: "contentGrew", newLength: contentLength })
  }, [contentLength])

  // Sync viewport size changes
  useEffect(() => {
    dispatch({ type: "resize", viewportSize })
  }, [viewportSize])

  // Route mouse events from L1
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

  return {
    state,
    scroll,
    visibleRange: visibleRange(state),
    isAtBottom: isAtBottom(state),
  }
}
