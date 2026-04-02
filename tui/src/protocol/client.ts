// L5: JSON-RPC client over stdin/stdout.
// Reads from process.stdin, writes to process.stdout.
// Thread-safe: multiple pending requests tracked by ID.

import * as readline from "node:readline"
import type { JsonRpcMessage, PermissionReply, QuestionReply } from "./types.js"

export type RequestHandler = (method: string, params: unknown, respond: (result: unknown) => void) => void
export type NotificationHandler = (method: string, params: unknown) => void

export class JsonRpcClient {
  private nextId = 1
  private pending = new Map<number, { resolve: (v: unknown) => void; reject: (e: Error) => void }>()
  private onRequest: RequestHandler | null = null
  private onNotification: NotificationHandler | null = null
  private rl: readline.Interface

  constructor() {
    this.rl = readline.createInterface({ input: process.stdin })
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
    this.writeLine(msg)
  }

  // Send a request and wait for response
  async request<T>(method: string, params: unknown): Promise<T> {
    const id = this.nextId++
    const msg: JsonRpcMessage = { jsonrpc: "2.0", method, params, id }
    return new Promise<T>((resolve, reject) => {
      this.pending.set(id, { resolve: resolve as (v: unknown) => void, reject })
      this.writeLine(msg)
    })
  }

  // Respond to a backend request
  respond(id: number, result: unknown): void {
    const msg: JsonRpcMessage = { jsonrpc: "2.0", result, id }
    this.writeLine(msg)
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

  private writeLine(msg: JsonRpcMessage) {
    process.stdout.write(JSON.stringify(msg) + "\n")
  }
}
