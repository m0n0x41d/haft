import { strict as assert } from "node:assert"
import { test } from "node:test"

import { JsonRpcClient } from "./client.js"

class FakeReadline {
  on(_event: "line" | "close", _handler: ((line: string) => void) | (() => void)) {
    return this
  }
}

test("write queue recovers after a failed write", async () => {
  const writes: string[] = []
  let callCount = 0

  const client = new JsonRpcClient({
    createInterface: () => new FakeReadline(),
    write: (_fd, buf, _offset, _length, _position, cb) => {
      callCount += 1
      writes.push(buf.toString("utf-8"))
      if (callCount === 1) {
        cb({ name: "Error", message: "boom" } as NodeJS.ErrnoException)
        return undefined
      }
      cb(null)
      return undefined
    },
  })

  client.send("first", { ok: false })
  client.send("second", { ok: true })

  await new Promise((resolve) => setTimeout(resolve, 20))

  assert.equal(callCount, 2)
  assert.match(writes[0]!, /"method":"first"/)
  assert.match(writes[1]!, /"method":"second"/)
})
