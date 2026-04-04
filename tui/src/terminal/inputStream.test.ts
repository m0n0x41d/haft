import { strict as assert } from "node:assert"
import { EventEmitter } from "node:events"
import { test } from "node:test"
import { createInputRouter } from "./inputStream.js"

type FakeTTYInput = EventEmitter & {
  isRaw: boolean
  setRawMode: (mode: boolean) => FakeTTYInput
  resume: () => FakeTTYInput
  pause: () => FakeTTYInput
}

function createFakeTTYInput() {
  const calls: string[] = []
  const ttyInput = new EventEmitter() as FakeTTYInput

  ttyInput.isRaw = false
  ttyInput.setRawMode = (mode: boolean) => {
    ttyInput.isRaw = mode
    calls.push(`raw:${String(mode)}`)
    return ttyInput
  }
  ttyInput.resume = () => {
    calls.push("resume")
    return ttyInput
  }
  ttyInput.pause = () => {
    calls.push("pause")
    return ttyInput
  }

  return { ttyInput, calls }
}

test("destroy restores raw mode and pauses tty input", () => {
  const { ttyInput, calls } = createFakeTTYInput()
  const router = createInputRouter(ttyInput as any)

  assert.equal(ttyInput.isRaw, true)
  assert.deepEqual(calls, ["raw:true", "resume"])

  router.destroy()

  assert.equal(ttyInput.isRaw, false)
  assert.deepEqual(calls, ["raw:true", "resume", "raw:false", "pause"])
})
