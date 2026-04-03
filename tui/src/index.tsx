#!/usr/bin/env node
// L0 + L5 entry point: terminal setup -> render App.
//
// Architecture:
//   stdin/stdout = JSON-RPC protocol (Go backend)
//   stderr = terminal rendering (Ink)
//   /dev/tty = keyboard + mouse input (read by our inputStream, NOT by Ink)
//
// Ink receives a dummy stdin and is rendering-only.
// All input flows: /dev/tty → inputStream → EventEmitter → useInput hooks.

import React from "react"
import { render } from "ink"
import { trace } from "./debug.js"
import { InputProvider } from "./hooks/InputContext.js"
import { App } from "./components/App.js"
import { JsonRpcClient } from "./protocol/client.js"
import {
  openTerminal,
  enableMouseTracking,
  enableBracketedPaste,
  reassertTerminalModes,
  cleanupTerminal,
} from "./terminal/adapter.js"
import { createInputRouter } from "./terminal/inputStream.js"
import { preloadHighlighter } from "./rendering/highlight.js"

trace("tui.start")

// L0: open terminal
const terminal = openTerminal()
trace("tui.terminal_opened")

// L1: create input router
// - Reads from /dev/tty via standard 'data' events (no monkey-patching)
// - Sets raw mode on the real TTY stream
// - Creates a dummy stdin for Ink (Ink is rendering-only)
// - Emits "keyboard", "input", "paste", "force-exit" on the EventEmitter
const router = createInputRouter(terminal.input, () => {
  reassertTerminalModes(terminal.output)
})

// Eagerly load highlight.js bundle so code blocks render with colors
preloadHighlighter()

// L0: enable terminal modes
enableMouseTracking(terminal.output)
enableBracketedPaste(terminal.output)

// Signal-safe cleanup
function cleanup() {
  cleanupTerminal(terminal.output)
  router.destroy()
}

process.on("exit", cleanup)
process.on("SIGINT", () => { cleanup(); process.exit(130) })
process.on("SIGTERM", () => { cleanup(); process.exit(143) })
process.on("SIGTSTP", () => { cleanup(); process.kill(process.pid, "SIGTSTP") })
process.on("SIGCONT", () => { reassertTerminalModes(terminal.output) })
process.on("uncaughtException", (err) => { cleanup(); console.error("haft TUI crashed:", err); process.exit(1) })
process.on("unhandledRejection", (reason) => { cleanup(); console.error("haft TUI unhandled rejection:", reason); process.exit(1) })

// Double ctrl-c force exit (works even when React event loop is saturated)
router.events.on("force-exit", () => {
  cleanup()
  process.exit(130)
})

trace("tui.pre_render")

// L5: create protocol client + render
const client = new JsonRpcClient()

// Ink gets the dummy stdin — it never receives input data.
// All input goes through our EventEmitter → useInput hooks.
render(
  <InputProvider value={router.events}>
    <App client={client} inputEvents={router.events} />
  </InputProvider>,
  {
    stdin: router.inkStdin as any,
    stdout: terminal.output,
    stderr: terminal.output,
    exitOnCtrlC: false,
  },
)
