// L2 React hook: wires L1 input events → L2 scroll reducer.
// Accepts transcript rows and computes an anchor-based render window.

import { useState, useEffect, useCallback, useMemo, useRef } from "react"
import type { EventEmitter } from "node:events"
import type { InputEvent } from "../terminal/input.js"
import type { TranscriptRow } from "../state/transcript.js"
import {
  buildTranscriptLayout,
  computeRenderWindow,
  computeVisibleWindow,
  createTranscriptHeightCache,
  type TranscriptLayout,
  type VisibleWindow,
} from "./measure.js"
import {
  getOffsetFromBottom,
  initialScroll,
  isAtBottom,
  reduceScroll,
  resolveViewportTop,
  type ScrollState,
  type ScrollCommand,
} from "./state.js"

export interface UseScrollResult {
  state: ScrollState
  scroll: (cmd: ScrollCommand) => void
  visibleWindow: VisibleWindow
  layout: TranscriptLayout
  linesAboveBottom: number
  isAtBottom: boolean
}

export function useScroll(
  inputEvents: EventEmitter | null,
  rows: readonly TranscriptRow[],
  width: number,
  viewportSize: number,
): UseScrollResult {
  const cacheRef = useRef(createTranscriptHeightCache())
  const layout = useMemo(
    () => buildTranscriptLayout(rows, width, cacheRef.current),
    [rows, width],
  )
  const layoutRef = useRef(layout)

  layoutRef.current = layout

  const [state, setState] = useState<ScrollState>(() => initialScroll(viewportSize))

  useEffect(() => {
    cacheRef.current = layout.heightCache
  }, [layout.heightCache])

  useEffect(() => {
    setState((current) => reduceScroll(current, { type: "contentChanged" }, layout))
  }, [layout])

  useEffect(() => {
    setState((current) => reduceScroll(current, { type: "resize", viewportSize }, layoutRef.current))
  }, [viewportSize])

  useEffect(() => {
    if (!inputEvents) return

    const handler = (ev: InputEvent) => {
      if (ev.type === "wheelUp") {
        setState((current) => reduceScroll(current, { type: "wheelUp" }, layoutRef.current))
      } else if (ev.type === "wheelDown") {
        setState((current) => reduceScroll(current, { type: "wheelDown" }, layoutRef.current))
      }
    }

    inputEvents.on("input", handler)

    return () => { inputEvents.off("input", handler) }
  }, [inputEvents])

  const scroll = useCallback((cmd: ScrollCommand) => {
    setState((current) => reduceScroll(current, cmd, layoutRef.current))
  }, [])

  const viewportTop = useMemo(
    () => resolveViewportTop(state, layout),
    [state, layout],
  )
  const renderWindow = useMemo(
    () => computeRenderWindow(layout, viewportTop, state.viewportSize, state.pinnedToBottom),
    [layout, viewportTop, state.viewportSize, state.pinnedToBottom],
  )
  const visibleWindow = useMemo(
    () => computeVisibleWindow(layout, renderWindow, state.viewportSize),
    [layout, renderWindow, state.viewportSize],
  )
  const linesAboveBottom = useMemo(
    () => getOffsetFromBottom(state, layout),
    [state, layout],
  )

  return {
    state,
    scroll,
    visibleWindow,
    layout,
    linesAboveBottom,
    isAtBottom: isAtBottom(state),
  }
}
