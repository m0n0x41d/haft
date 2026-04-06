// L2 React hook: wires L1 input events → L2 scroll reducer.
// Accepts per-entry heights and computes a measured virtual window.

import { useReducer, useEffect, useCallback, useMemo, useRef } from "react"
import type { EventEmitter } from "node:events"
import type { InputEvent } from "../terminal/input.js"
import { trace } from "../debug.js"
import {
  initialScroll,
  reduceScroll,
  isAtBottom,
  unreadLinesBelow,
  type ScrollState,
  type ScrollCommand,
} from "./state.js"
import { computeOffsets, computeVisibleWindow, type VisibleWindow } from "./measure.js"

export interface UseScrollResult {
  state: ScrollState
  scroll: (cmd: ScrollCommand) => void
  scrollMode: ScrollState["mode"]
  unreadBelow: number
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
  const previousTotalLinesRef = useRef(totalLines)
  const previousWindowRef = useRef<VisibleWindow | null>(null)
  const previousModeRef = useRef(state.mode)

  const visibleWindow = useMemo(
    () => computeVisibleWindow(entryOffsets, state.offset, state.viewportSize),
    [entryOffsets, state.offset, state.viewportSize],
  )
  const unreadBelow = unreadLinesBelow(state)

  const emitScroll = useCallback((cmd: ScrollCommand) => {
    switch (cmd.type) {
      case "wheelUp":
      case "pageUp":
      case "home":
        trace("user_scrolled_up", {
          cmd: cmd.type,
          mode: state.mode,
          offset: state.offset,
        })
        break

      case "end":
        trace("jump_to_bottom", {
          mode: state.mode,
          offset: state.offset,
        })
        break
    }

    dispatch(cmd)
  }, [state.mode, state.offset])

  useEffect(() => {
    dispatch({ type: "contentChanged", newTotalLines: totalLines })
  }, [totalLines])

  useEffect(() => {
    dispatch({ type: "resize", viewportSize })
  }, [viewportSize])

  useEffect(() => {
    if (!inputEvents) return
    const handler = (ev: InputEvent) => {
      if (ev.type === "wheelUp") emitScroll({ type: "wheelUp" })
      else if (ev.type === "wheelDown") emitScroll({ type: "wheelDown" })
    }
    inputEvents.on("input", handler)
    return () => { inputEvents.off("input", handler) }
  }, [emitScroll, inputEvents])

  const scroll = useCallback((cmd: ScrollCommand) => emitScroll(cmd), [emitScroll])

  useEffect(() => {
    const previousTotalLines = previousTotalLinesRef.current

    if (previousTotalLines !== totalLines) {
      trace("content_changed", {
        mode: state.mode,
        offset: state.offset,
        totalLines,
        delta: totalLines - previousTotalLines,
        viewportSize: state.viewportSize,
      })
      previousTotalLinesRef.current = totalLines
    }
  }, [state.mode, state.offset, state.viewportSize, totalLines])

  useEffect(() => {
    const previousWindow = previousWindowRef.current

    if (
      !previousWindow
      || previousWindow.start !== visibleWindow.start
      || previousWindow.end !== visibleWindow.end
      || previousWindow.viewTop !== visibleWindow.viewTop
      || previousWindow.viewBottom !== visibleWindow.viewBottom
    ) {
      trace("render_range_changed", {
        mode: state.mode,
        start: visibleWindow.start,
        end: visibleWindow.end,
        viewTop: visibleWindow.viewTop,
        viewBottom: visibleWindow.viewBottom,
        totalLines: visibleWindow.totalLines,
        unreadBelow,
      })
      previousWindowRef.current = visibleWindow
    }
  }, [state.mode, unreadBelow, visibleWindow])

  useEffect(() => {
    const previousMode = previousModeRef.current

    if (previousMode !== state.mode) {
      trace("scroll_mode_changed", {
        from: previousMode,
        to: state.mode,
        offset: state.offset,
        unreadBelow,
      })
      previousModeRef.current = state.mode
    }
  }, [state.mode, state.offset, unreadBelow])

  return {
    state,
    scroll,
    scrollMode: state.mode,
    unreadBelow,
    entryOffsets,
    visibleWindow,
    isAtBottom: isAtBottom(state),
  }
}
