#!/usr/bin/env node
// L0 + L5 entry point: terminal setup → render App.
//
// stdin/stdout = JSON-RPC protocol (Go backend)
// stderr = terminal rendering (Ink)
// /dev/tty = keyboard + mouse input

import React from "react"
import { render } from "ink"
import { App } from "./components/App.js"
import { JsonRpcClient } from "./protocol/client.js"
import {
  openTerminal,
  enableMouseTracking,
  reassertTerminalModes,
  cleanupTerminal,
} from "./terminal/adapter.js"
import { createInputRouter } from "./terminal/inputStream.js"

// L0: open terminal
const terminal = openTerminal()

// L1: create input router (parses mouse, strips SGR before Ink sees it)
// Pass onStdinResume callback to re-assert terminal modes after reconnect/wake
const router = createInputRouter(terminal.input, () => {
  reassertTerminalModes(terminal.output)
})

// L0: enable terminal modes
// Bracketed paste intentionally OFF — tmux doesn't reliably forward
// ESC[201~ (paste end), trapping all subsequent input in paste buffer.
// Text paste works without it (terminal sends raw chars).
// Image paste via Ctrl+V keybinding instead.
enableMouseTracking(terminal.output)

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

// L5: create protocol client + render
const client = new JsonRpcClient()

render(<App client={client} inputEvents={router.events} />, {
  stdin: router.stream,
  stdout: terminal.output,
  stderr: terminal.output,
})
