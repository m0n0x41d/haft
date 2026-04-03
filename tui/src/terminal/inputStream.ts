// L1 bridge: routes TTY input to keyboard (React) and side channel (mouse, paste).
//
// Architecture (following Claude Code's pattern):
//   - Reads from ttyInput via standard 'data' events (NO monkey-patching of push())
//   - Keyboard data → emitted as "keyboard" for useInput hooks
//   - Mouse/wheel → emitted as "input" for scroll/selection
//   - Paste content → emitted as "paste" for InputArea
//   - Ink gets a dummy stdin that never emits — Ink is rendering-only
//
// This eliminates the class of bugs where Ink's internal stdin management
// (pause/resume/raw mode) interferes with input delivery.

import type * as tty from "node:tty"
import { PassThrough } from "node:stream"
import { EventEmitter } from "node:events"
import { parseInput, type InputEvent } from "./input.js"
import { trace } from "../debug.js"

const PASTE_START = "\x1b[200~"
const PASTE_END = "\x1b[201~"
const CTRL_C = "\x03"
const PASTE_TIMEOUT_MS = 1000
const PASTE_HARD_DEADLINE_MS = 3000
const STDIN_RESUME_GAP_MS = 5000
const DOUBLE_CTRLC_MS = 1000
const MAX_PASTE_BYTES = 512 * 1024

// SGR mouse sequences: must not be accumulated during paste mode
const SGR_MOUSE_RE = /\x1b\[<\d+;\d+;\d+[Mm]/g

export interface InputRouter {
  /** EventEmitter for all input events: "keyboard", "input", "paste", "force-exit" */
  events: EventEmitter
  /** Dummy TTY stdin for Ink — never emits data, supports setRawMode() */
  inkStdin: NodeJS.ReadableStream
  /** Cleanup function */
  destroy: () => void
}

/**
 * Create a dummy TTY stdin that satisfies Ink's requirements
 * (isTTY, setRawMode, ref/unref) without touching any real terminal stream.
 */
function createDummyTTYStdin(): NodeJS.ReadableStream {
  const stream = new PassThrough() as any
  stream.isTTY = true
  stream.isRaw = false
  stream.setRawMode = function(mode: boolean) { stream.isRaw = mode; return stream }
  stream.ref = function() { return stream }
  stream.unref = function() { return stream }
  return stream
}

/**
 * Create an input router that reads from the real TTY and dispatches events.
 *
 * Events emitted:
 *   "keyboard"   (raw: string)  — keyboard data for useInput hooks
 *   "input"      (InputEvent)   — mouse/wheel events for scroll/selection
 *   "paste"      (text: string) — accumulated paste text
 *   "force-exit" ()             — double ctrl-c
 */
export function createInputRouter(
  ttyInput: tty.ReadStream,
  onStdinResume?: () => void,
): InputRouter {
  const events = new EventEmitter()
  const inkStdin = createDummyTTYStdin()

  // --- Paste accumulator ---
  let inPaste = false
  let pasteBuffer = ""
  let pasteTimer: ReturnType<typeof setTimeout> | null = null
  let pasteDeadlineTimer: ReturnType<typeof setTimeout> | null = null

  function flushPaste() {
    inPaste = false
    if (pasteTimer) { clearTimeout(pasteTimer); pasteTimer = null }
    if (pasteDeadlineTimer) { clearTimeout(pasteDeadlineTimer); pasteDeadlineTimer = null }
    const text = pasteBuffer
    pasteBuffer = ""
    if (text) {
      trace(`paste: flushed ${text.length} chars`)
      events.emit("paste", text)
    }
  }

  function abortPaste() {
    trace("paste: aborted (timeout/overflow)")
    inPaste = false
    if (pasteTimer) { clearTimeout(pasteTimer); pasteTimer = null }
    if (pasteDeadlineTimer) { clearTimeout(pasteDeadlineTimer); pasteDeadlineTimer = null }
    pasteBuffer = ""
  }

  function enterPasteMode() {
    inPaste = true
    pasteBuffer = ""
    trace("paste: enter")
    pasteTimer = setTimeout(flushPaste, PASTE_TIMEOUT_MS)
    pasteDeadlineTimer = setTimeout(abortPaste, PASTE_HARD_DEADLINE_MS)
  }

  function resetPasteTimer() {
    if (pasteTimer) clearTimeout(pasteTimer)
    pasteTimer = setTimeout(flushPaste, PASTE_TIMEOUT_MS)
  }

  // --- Double ctrl-c detector ---
  let lastCtrlCTime = 0

  // --- Stdin resume detection ---
  let lastStdinTime = Date.now()

  // --- Forward mouse events from a string (used during paste to strip mouse) ---
  function emitMouseEvents(str: string) {
    const parsed = parseInput(str)
    for (const ev of parsed) {
      if (ev.type !== "key") events.emit("input", ev)
    }
  }

  // --- Strip mouse sequences, return non-mouse remainder ---
  function stripMouse(str: string): string {
    SGR_MOUSE_RE.lastIndex = 0
    const stripped = str.replace(SGR_MOUSE_RE, "")
    if (stripped.length < str.length) emitMouseEvents(str)
    return stripped
  }

  // --- Route a chunk of non-paste input ---
  function routeNormal(str: string) {
    const parsed = parseInput(str)

    // Emit mouse/wheel events on side channel
    for (const ev of parsed) {
      if (ev.type !== "key") events.emit("input", ev)
    }

    // Emit keyboard data for useInput hooks
    const keys = parsed
      .filter((e): e is Extract<InputEvent, { type: "key" }> => e.type === "key")
      .map((e) => e.raw)
      .join("")

    if (keys.length > 0) events.emit("keyboard", keys)
  }

  // --- Process a chunk (may contain paste start) ---
  function processChunk(str: string): void {
    const startIdx = str.indexOf(PASTE_START)
    if (startIdx >= 0) {
      if (startIdx > 0) routeNormal(str.slice(0, startIdx))
      enterPasteMode()
      const afterStart = str.slice(startIdx + PASTE_START.length)
      const endIdx = afterStart.indexOf(PASTE_END)
      if (endIdx >= 0) {
        pasteBuffer = afterStart.slice(0, endIdx)
        flushPaste()
        const afterEnd = afterStart.slice(endIdx + PASTE_END.length)
        if (afterEnd) processChunk(afterEnd)
      } else {
        pasteBuffer = afterStart
      }
      return
    }
    routeNormal(str)
  }

  // --- Main data handler on the real TTY stream ---
  function onData(chunk: Buffer | string) {
    const str = typeof chunk === "string" ? chunk : chunk.toString("utf-8")

    // Stdin resume detection (tmux reattach, laptop wake)
    const now = Date.now()
    if (now - lastStdinTime > STDIN_RESUME_GAP_MS) onStdinResume?.()
    lastStdinTime = now

    // Ctrl-C: always handled, even during paste
    if (str.includes(CTRL_C)) {
      const ctrlCNow = Date.now()
      if (ctrlCNow - lastCtrlCTime < DOUBLE_CTRLC_MS) {
        events.emit("force-exit")
        return
      }
      lastCtrlCTime = ctrlCNow
      if (inPaste) abortPaste()
      // Emit ctrl-c as keyboard event for useInput
      events.emit("keyboard", CTRL_C)
      return
    }

    // Paste mode: accumulate
    if (inPaste) {
      const endIdx = str.indexOf(PASTE_END)
      if (endIdx >= 0) {
        const beforeEnd = stripMouse(str.slice(0, endIdx))
        pasteBuffer += beforeEnd
        if (pasteBuffer.length > MAX_PASTE_BYTES) pasteBuffer = ""
        flushPaste()
        const after = str.slice(endIdx + PASTE_END.length)
        if (after) processChunk(after)
      } else {
        const cleaned = stripMouse(str)
        if (cleaned.length > 0) {
          pasteBuffer += cleaned
          resetPasteTimer()
        }
        if (pasteBuffer.length > MAX_PASTE_BYTES) abortPaste()
      }
      return
    }

    processChunk(str)
  }

  // Enable raw mode on the real TTY
  ttyInput.setRawMode(true)
  ttyInput.resume()
  ttyInput.on("data", onData)

  return {
    events,
    inkStdin,
    destroy: () => {
      ttyInput.removeListener("data", onData)
      if (pasteTimer) clearTimeout(pasteTimer)
      if (pasteDeadlineTimer) clearTimeout(pasteDeadlineTimer)
    },
  }
}
