import { strict as assert } from "node:assert"
import { test } from "node:test"
import { createTraceLogger, resolveTraceConfig } from "./debug.js"

test("keeps tracing disabled by default and assigns a fallback file", () => {
  const config = resolveTraceConfig({}, 1234)

  assert.equal(config.enabled, false)
  assert.match(config.filePath, /haft-tui-1234\.log$/)
})

test("uses the explicit trace file when provided", () => {
  const config = resolveTraceConfig({
    HAFT_TUI_TRACE: "1",
    HAFT_TUI_TRACE_FILE: "./tmp/tui-trace.log",
  }, 4321)

  assert.equal(config.enabled, true)
  assert.match(config.filePath, /tmp\/tui-trace\.log$/)
})

test("writes structured trace lines only when enabled", () => {
  const writes: string[] = []
  const logger = createTraceLogger({
    env: {
      HAFT_TUI_TRACE: "1",
      HAFT_TUI_TRACE_FILE: "/tmp/custom.log",
    },
    now: () => 250,
    pid: 999,
    writer: {
      write(line: string) {
        writes.push(line)
      },
    },
  })

  logger.trace("stream_update_received", { textLength: 42, streaming: true })

  assert.equal(writes.length, 1)

  const record = JSON.parse(writes[0]!)
  assert.equal(record.event, "stream_update_received")
  assert.equal(record.file, "/tmp/custom.log")
  assert.equal(record.ms, 0)
  assert.equal(record.textLength, 42)
  assert.equal(record.streaming, true)
})

test("does not write trace lines when tracing is disabled", () => {
  const writes: string[] = []
  const logger = createTraceLogger({
    env: {},
    writer: {
      write(line: string) {
        writes.push(line)
      },
    },
  })

  logger.trace("content_changed", { totalLines: 12 })

  assert.deepEqual(writes, [])
})
