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
  const ttyInput = new EventEmitter() as FakeTTYInput

  ttyInput.isRaw = false
  ttyInput.setRawMode = (mode: boolean) => {
    ttyInput.isRaw = mode
    return ttyInput
  }
  ttyInput.resume = () => ttyInput
  ttyInput.pause = () => ttyInput

  return ttyInput
}

test("emits bracketed multiline paste as one paste event and resumes typing after it", () => {
  const ttyInput = createFakeTTYInput()
  const router = createInputRouter(ttyInput as any)
  const pastes: string[] = []
  const keyboard: string[] = []

  router.events.on("paste", (text: string) => pastes.push(text))
  router.events.on("keyboard", (text: string) => keyboard.push(text))

  ttyInput.emit("data", "\x1b[200~first line\nsecond line\x1b[201~z")

  assert.deepEqual(pastes, ["first line\nsecond line"])
  assert.deepEqual(keyboard, ["z"])

  router.destroy()
})

test("keeps mouse scroll events out of pasted text", () => {
  const ttyInput = createFakeTTYInput()
  const router = createInputRouter(ttyInput as any)
  const pastes: string[] = []
  const inputs: unknown[] = []

  router.events.on("paste", (text: string) => pastes.push(text))
  router.events.on("input", (event) => inputs.push(event))

  ttyInput.emit("data", "\x1b[200~before\x1b[<64;30;15Mafter\x1b[201~")

  assert.deepEqual(pastes, ["beforeafter"])
  assert.deepEqual(inputs, [{ type: "wheelUp" }])

  router.destroy()
})
