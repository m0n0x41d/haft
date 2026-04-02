// L1 bridge: strips SGR mouse sequences from tty input before Ink sees them.
// Mouse/scroll events emitted on side channel.

import type * as tty from "node:tty"
import { EventEmitter } from "node:events"
import { parseInput, type InputEvent } from "./input.js"

export interface InputRouter {
  stream: tty.ReadStream
  events: EventEmitter
  destroy: () => void
}

export function createInputRouter(
  ttyInput: tty.ReadStream,
  _onStdinResume?: () => void,
): InputRouter {
  const events = new EventEmitter()
  const origPush = ttyInput.push.bind(ttyInput)

  ;(ttyInput as any).push = function(chunk: Buffer | string | null): boolean {
    if (chunk === null) return origPush(null)

    const str = typeof chunk === "string" ? chunk : chunk.toString("utf-8")
    const parsed = parseInput(str)

    for (const ev of parsed) {
      if (ev.type === "wheelUp" || ev.type === "wheelDown" || ev.type === "mouseClick" || ev.type === "mouseRelease") {
        events.emit("input", ev)
      }
    }

    const keyEvents = parsed.filter((e): e is Extract<InputEvent, { type: "key" }> => e.type === "key")
    const cleanStr = keyEvents.map((e) => e.raw).join("")

    if (cleanStr.length > 0) {
      return origPush(Buffer.from(cleanStr, "utf-8"))
    }

    return true
  }

  return {
    stream: ttyInput,
    events,
    destroy: () => {
      ;(ttyInput as any).push = origPush
    },
  }
}
