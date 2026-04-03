// Stable useInput — reads from our own EventEmitter, NOT Ink's stdin.
//
// Ink is rendering-only. All input flows:
//   /dev/tty → data event → inputStream router → "keyboard" event → this hook
//
// This eliminates the class of bugs where Ink's internal stdin management
// (pause/resume/buffer) interferes with input delivery.

import { useEffect } from "react"
import { useInputEmitter } from "./InputContext.js"
import { useEventCallback } from "./useEventCallback.js"
import parseKeypress, { nonAlphanumericKeys } from "./parseKeypress.js"

export type Key = {
  upArrow: boolean
  downArrow: boolean
  leftArrow: boolean
  rightArrow: boolean
  pageDown: boolean
  pageUp: boolean
  home: boolean
  end: boolean
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
  const emitter = useInputEmitter()

  // Stable handler via ref — NEVER causes listener re-registration.
  const stableHandler = useEventCallback((data: string) => {
    if (options.isActive === false) return

    const keypress = parseKeypress(data)
    const key: Key = {
      upArrow: keypress.name === "up",
      downArrow: keypress.name === "down",
      leftArrow: keypress.name === "left",
      rightArrow: keypress.name === "right",
      pageDown: keypress.name === "pagedown",
      pageUp: keypress.name === "pageup",
      home: keypress.name === "home",
      end: keypress.name === "end",
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

    inputHandler(input, key)
  })

  // Register listener ONCE on mount. stableHandler identity is fixed.
  useEffect(() => {
    emitter.on("keyboard", stableHandler)
    return () => { emitter.removeListener("keyboard", stableHandler) }
  }, [emitter, stableHandler])
}
