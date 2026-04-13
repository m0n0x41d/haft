import { strict as assert } from "node:assert"
import { test } from "node:test"
import type { MsgUpdateParams } from "../protocol/types.js"
import { createStreamUpdateBuffer } from "./streamUpdateBuffer.js"

function buildUpdate(
  text: string,
  streaming: boolean = true,
): MsgUpdateParams {
  return {
    id: "assistant-1",
    streaming,
    text,
  }
}

function createManualScheduler() {
  let callback: (() => void) | null = null

  return {
    schedule(next: () => void) {
      callback = next
      return 1
    },
    cancel() {
      callback = null
    },
    run() {
      const current = callback

      callback = null
      current?.()
    },
  }
}

test("coalesces bursty streaming updates into one scheduled flush", () => {
  const scheduler = createManualScheduler()
  const flushed: Array<{ update: MsgUpdateParams; reason: string }> = []
  const buffer = createStreamUpdateBuffer({
    onFlush(update, reason) {
      flushed.push({ update, reason })
    },
    schedule: scheduler.schedule,
    cancel: scheduler.cancel,
  })

  buffer.push(buildUpdate("first"))
  buffer.push(buildUpdate("second"))
  buffer.push(buildUpdate("third"))
  scheduler.run()

  assert.equal(flushed.length, 1)
  assert.equal(flushed[0]?.reason, "scheduled_flush")
  assert.equal(flushed[0]?.update.text, "third")
  assert.equal(buffer.hasPending(), false)
})

test("flushes the latest pending update with explicit final overrides", () => {
  const scheduler = createManualScheduler()
  const flushed: Array<{ update: MsgUpdateParams; reason: string }> = []
  const buffer = createStreamUpdateBuffer({
    onFlush(update, reason) {
      flushed.push({ update, reason })
    },
    schedule: scheduler.schedule,
    cancel: scheduler.cancel,
  })

  buffer.push(buildUpdate("partial"))
  const didFlush = buffer.flush("coord_done", { streaming: false })

  assert.equal(didFlush, true)
  assert.equal(flushed.length, 1)
  assert.equal(flushed[0]?.reason, "coord_done")
  assert.equal(flushed[0]?.update.streaming, false)
  assert.equal(flushed[0]?.update.text, "partial")
})

test("replaces a pending streaming update with an immediate final update", () => {
  const scheduler = createManualScheduler()
  const flushed: Array<{ update: MsgUpdateParams; reason: string }> = []
  const buffer = createStreamUpdateBuffer({
    onFlush(update, reason) {
      flushed.push({ update, reason })
    },
    schedule: scheduler.schedule,
    cancel: scheduler.cancel,
  })

  buffer.push(buildUpdate("partial"))
  buffer.replace(buildUpdate("final", false), "final_msg_update")
  scheduler.run()

  assert.equal(flushed.length, 1)
  assert.equal(flushed[0]?.reason, "final_msg_update")
  assert.equal(flushed[0]?.update.text, "final")
  assert.equal(flushed[0]?.update.streaming, false)
})
