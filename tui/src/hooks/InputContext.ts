// React context for keyboard input events.
// Replaces Ink's internal_eventEmitter — our input pipeline owns the
// entire path from /dev/tty to component handlers. Ink never touches stdin.

import { createContext, useContext } from "react"
import type { EventEmitter } from "node:events"

// The EventEmitter emits:
//   "keyboard" (raw: string) — keyboard data for useInput parsing
//   "input"    (InputEvent)  — mouse/wheel/selection events
//   "paste"    (text: string) — bracketed paste content
//   "force-exit" ()          — double ctrl-c

const InputContext = createContext<EventEmitter | null>(null)

export const InputProvider = InputContext.Provider

export function useInputEmitter(): EventEmitter {
  const emitter = useContext(InputContext)
  if (!emitter) throw new Error("useInputEmitter must be used within InputProvider")
  return emitter
}
