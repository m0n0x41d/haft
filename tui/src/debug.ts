import { appendFileSync, mkdirSync } from "node:fs"
import { tmpdir } from "node:os"
import { dirname, resolve } from "node:path"

export type TraceFields = Record<string, boolean | number | string | null | undefined>

export interface TraceEnv {
  HAFT_TUI_TRACE?: string
  HAFT_TUI_TRACE_FILE?: string
}

export interface TraceConfig {
  enabled: boolean
  filePath: string
}

interface TraceWriter {
  write: (line: string) => void
}

interface TraceLoggerOptions {
  env?: TraceEnv
  now?: () => number
  pid?: number
  writer?: TraceWriter
}

export function resolveTraceConfig(
  env: TraceEnv = process.env,
  pid: number = process.pid,
): TraceConfig {
  const enabled = env.HAFT_TUI_TRACE === "1"
  const requestedPath = env.HAFT_TUI_TRACE_FILE?.trim()
  const fallbackPath = resolve(tmpdir(), `haft-tui-${pid}.log`)
  const filePath = requestedPath && requestedPath.length > 0
    ? resolve(requestedPath)
    : fallbackPath

  return { enabled, filePath }
}

function createFileWriter(filePath: string): TraceWriter {
  mkdirSync(dirname(filePath), { recursive: true })

  return {
    write(line: string) {
      try {
        appendFileSync(filePath, `${line}\n`, "utf8")
      } catch {
        // Tracing must never crash the TUI.
      }
    },
  }
}

export function createTraceLogger(options: TraceLoggerOptions = {}) {
  const now = options.now ?? Date.now
  const config = resolveTraceConfig(options.env, options.pid)
  const writer = options.writer ?? (config.enabled ? createFileWriter(config.filePath) : null)
  const startMs = now()

  return {
    config,
    trace(event: string, fields: TraceFields = {}) {
      if (!config.enabled) {
        return
      }

      const record = {
        event,
        file: config.filePath,
        ms: now() - startMs,
        ts: new Date().toISOString(),
        ...fields,
      }

      writer?.write(JSON.stringify(record))
    },
  }
}

const traceLogger = createTraceLogger()

export function trace(
  event: string,
  fields?: TraceFields,
): void {
  traceLogger.trace(event, fields)
}
