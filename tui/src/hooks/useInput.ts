// Stable useInput — replaces Ink's stock useInput.
//
// Differences from stock Ink v5 useInput:
// 1. useLayoutEffect for raw mode (fires synchronously during commit, no echo gap)
// 2. useEventCallback for stable handler (listener registered ONCE, never re-appended)
// 3. isActive checked INSIDE handler, not in effect deps (preserves listener slot ordering)
//
// This eliminates the root cause of all input bugs: Ink's useEffect + inline handler
// causes listener re-registration on every render, with a gap where keypresses are lost.

import { useEffect, useLayoutEffect } from "react"
import { useStdin } from "ink"
import { useEventCallback } from "./useEventCallback.js"

import parseKeypress, { nonAlphanumericKeys } from "./parseKeypress.js"

export type Key = {
  upArrow: boolean
  downArrow: boolean
  leftArrow: boolean
  rightArrow: boolean
  pageDown: boolean
  pageUp: boolean
  return: boolean
  escape: boolean
  ctrl: boolean
  shift: boolean
  tab: boolean
  backspace: boolean
  delete: boolean
  meta: boolean
}

type InputHandler = (input: string, key: Key) => void

interface UseInputOptions {
  isActive?: boolean
}

export function useInput(inputHandler: InputHandler, options: UseInputOptions = {}) {
  // Access Ink's internal stdin context
  const { setRawMode, internal_exitOnCtrlC, internal_eventEmitter } = useStdin() as {
    setRawMode: (mode: boolean) => void
    internal_exitOnCtrlC: boolean
    internal_eventEmitter: import("node:events").EventEmitter | undefined
  }

  // Raw mode via useLayoutEffect — fires synchronously during React commit phase.
  // Stock Ink uses useEffect which defers to the next event loop tick,
  // leaving raw mode disabled during paint → keystrokes echo and cursor visible.
  useLayoutEffect(() => {
    if (options.isActive === false) return
    setRawMode(true)
    return () => { setRawMode(false) }
  }, [options.isActive, setRawMode])

  // Stable handler via ref — NEVER causes listener re-registration.
  // Always reads the latest isActive/inputHandler from the closure.
  const stableHandler = useEventCallback((data: string) => {
    // Guard inside handler, not in effect deps — preserves listener slot
    // in EventEmitter's array (critical for stopImmediatePropagation ordering)
    if (options.isActive === false) return

    const keypress = parseKeypress(data)
    const key: Key = {
      upArrow: keypress.name === "up",
      downArrow: keypress.name === "down",
      leftArrow: keypress.name === "left",
      rightArrow: keypress.name === "right",
      pageDown: keypress.name === "pagedown",
      pageUp: keypress.name === "pageup",
      return: keypress.name === "return",
      escape: keypress.name === "escape",
      ctrl: keypress.ctrl,
      shift: keypress.shift,
      tab: keypress.name === "tab",
      backspace: keypress.name === "backspace",
      delete: keypress.name === "delete",
      meta: keypress.meta || keypress.name === "escape" || keypress.option,
    }

    let input: string = keypress.ctrl ? keypress.name : keypress.sequence
    if (nonAlphanumericKeys.includes(keypress.name)) input = ""
    if (input.startsWith("\u001B")) input = input.slice(1)
    if (input.length === 1 && typeof input[0] === "string" && /[A-Z]/.test(input[0])) {
      key.shift = true
    }

    // Suppress Ctrl+C only if app wants to exit on it (Ink default behavior)
    if (!(input === "c" && key.ctrl) || !internal_exitOnCtrlC) {
      inputHandler(input, key)
    }
  })

  // Register listener ONCE on mount. stableHandler identity is fixed
  // (useEventCallback returns a stable reference), so this effect never re-runs.
  // Listener slot in EventEmitter's array is permanent — no re-ordering.
  useEffect(() => {
    internal_eventEmitter?.on("input", stableHandler)
    return () => { internal_eventEmitter?.removeListener("input", stableHandler) }
  }, [internal_eventEmitter, stableHandler])
}
