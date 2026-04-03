// Temporary debug tracer — writes to /tmp/haft-debug.log
// Remove after debugging the freeze issue.

import { appendFileSync } from "node:fs"

const LOG_PATH = "/tmp/haft-debug.log"

export function trace(label: string): void {
  try {
    appendFileSync(LOG_PATH, `[${new Date().toISOString()}] ${label}\n`)
  } catch {
    // silently ignore — debug logging must never crash the app
  }
}

// Heartbeat: writes every 2 seconds so we can detect event loop freezes
let heartbeatCount = 0
setInterval(() => {
  trace(`heartbeat #${++heartbeatCount}`)
}, 2000).unref()
