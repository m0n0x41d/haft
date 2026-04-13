// L5: JSON-RPC client over stdin/stdout.
// Reads from process.stdin, writes to process.stdout.
// Thread-safe: multiple pending requests tracked by ID.
//
// IMPORTANT: Writes are serialized through an async queue using fs.write(fd)
// instead of process.stdout.write(). This prevents Bun from blocking the
// event loop on pipe writes when the Go backend is temporarily not reading.

import * as readline from "node:readline"
import { write as fsWrite } from "node:fs"
import { trace } from "../debug.js"
import type { JsonRpcMessage, PermissionReply, QuestionReply } from "./types.js"

export type RequestHandler = (method: string, params: unknown, respond: (result: unknown) => void) => void
export type NotificationHandler = (method: string, params: unknown) => void

interface ReadlineLike {
  on(event: "line", handler: (line: string) => void): ReadlineLike
  on(event: "close", handler: () => void): ReadlineLike
}

type WriteCallback = (err: NodeJS.ErrnoException | null) => void
type WriteFn = (fd: number, buffer: Buffer, offset: number, length: number, position: number | null, callback: WriteCallback) => unknown

interface JsonRpcClientDeps {
  createInterface?: () => ReadlineLike
  write?: WriteFn
}

export class JsonRpcClient {
  private nextId = 1
  private pending = new Map<number, { resolve: (v: unknown) => void; reject: (e: Error) => void }>()
  private onRequest: RequestHandler | null = null
  private onNotification: NotificationHandler | null = null
  private rl: ReadlineLike
  private readonly write: WriteFn

  // Async write queue: serializes writes so they never interleave,
  // and uses fs.write(fd) which goes through libuv's threadpool (non-blocking).
  // A failed write must not poison the queue forever.
  private writeChain: Promise<void> = Promise.resolve()

  constructor(deps: JsonRpcClientDeps = {}) {
    this.rl = deps.createInterface?.() ?? readline.createInterface({ input: process.stdin })
    this.write = deps.write ?? fsWrite
    this.rl.on("line", (line) => this.handleLine(line))
    this.rl.on("close", () => process.exit(0))
  }

  setRequestHandler(handler: RequestHandler) {
    this.onRequest = handler
  }

  setNotificationHandler(handler: NotificationHandler) {
    this.onNotification = handler
  }

  // Send a notification (no response expected)
  send(method: string, params: unknown): void {
    const msg: JsonRpcMessage = { jsonrpc: "2.0", method, params }
    this.enqueueWrite(msg)
  }

  // Send a request and wait for response
  async request<T>(method: string, params: unknown): Promise<T> {
    const id = this.nextId++
    const msg: JsonRpcMessage = { jsonrpc: "2.0", method, params, id }
    return new Promise<T>((resolve, reject) => {
      this.pending.set(id, { resolve: resolve as (v: unknown) => void, reject })
      this.enqueueWrite(msg)
    })
  }

  // Respond to a backend request
  respond(id: number, result: unknown): void {
    const msg: JsonRpcMessage = { jsonrpc: "2.0", result, id }
    this.enqueueWrite(msg)
  }

  private handleLine(line: string) {
    if (!line.trim()) return
    let msg: JsonRpcMessage
    try {
      msg = JSON.parse(line)
    } catch {
      return // skip malformed
    }

    // Response to our pending request
    if (msg.id !== undefined && !msg.method) {
      const pending = this.pending.get(msg.id)
      if (pending) {
        this.pending.delete(msg.id)
        if (msg.error) {
          pending.reject(new Error(`RPC error ${msg.error.code}: ${msg.error.message}`))
        } else {
          pending.resolve(msg.result)
        }
      }
      return
    }

    // Request from backend (needs response)
    if (msg.id !== undefined && msg.method) {
      if (this.onRequest) {
        this.onRequest(msg.method, msg.params, (result) => {
          this.respond(msg.id!, result)
        })
      }
      return
    }

    // Notification from backend
    if (msg.method && msg.id === undefined) {
      if (this.onNotification) {
        this.onNotification(msg.method, msg.params)
      }
    }
  }

  // Enqueue a write. Writes are serialized: each starts only after the
  // previous one completes. Uses fs.write(fd=1) which is async (libuv
  // threadpool) rather than process.stdout.write() which can block in Bun.
  private enqueueWrite(msg: JsonRpcMessage): void {
    const line = JSON.stringify(msg) + "\n"
    this.writeChain = this.writeChain
      .catch(() => {})
      .then(() => this.writeAsync(line))
      .catch((err) => {
        trace(`rpc.write.queue.err ${err instanceof Error ? err.message : String(err)}`)
      })
  }

  private writeAsync(line: string): Promise<void> {
    const buf = Buffer.from(line, "utf-8")
    trace(`rpc.write len=${buf.length}`)

    // Use process.stdout.write() with drain handling for large messages.
    // Raw fs.write() can block the event loop when the pipe buffer is full,
    // causing a deadlock (TUI can't read Go's responses while write blocks).
    return new Promise<void>((resolve, reject) => {
      const ok = process.stdout.write(buf, (err) => {
        if (err) {
          trace(`rpc.write.err ${err.message}`)
          reject(err)
        } else {
          resolve()
        }
      })

      if (!ok) {
        // Pipe buffer full — wait for drain before resolving.
        // This yields the event loop so incoming messages can still be processed.
        trace(`rpc.write.backpressure len=${buf.length}`)
      }
    })
  }
}
