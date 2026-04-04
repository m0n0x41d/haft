import { strict as assert } from "node:assert"
import { test } from "node:test"
import {
  measureGraphemeWidth,
  normalizeGraphemeBoundaryLeft,
  segmentGraphemes,
} from "./graphemes.js"

test("segments graphemes with stable offsets", () => {
  const graphemes = segmentGraphemes("A👍🏻B")

  assert.deepEqual(
    graphemes.map((grapheme) => ({
      text: grapheme.text,
      start: grapheme.start,
      end: grapheme.end,
    })),
    [
      { text: "A", start: 0, end: 1 },
      { text: "👍🏻", start: 1, end: 5 },
      { text: "B", start: 5, end: 6 },
    ],
  )
})

test("snaps cursor offsets to the left grapheme boundary", () => {
  assert.equal(normalizeGraphemeBoundaryLeft("👍🏻", 1), 0)
  assert.equal(normalizeGraphemeBoundaryLeft("👍🏻", 4), 4)
})

test("fallback width handles emoji presentation clusters without Bun stringWidth", () => {
  const widths = withRuntimeStringWidth(undefined, () => ({
    heart: measureGraphemeWidth("❤️"),
    star: measureGraphemeWidth("⭐️"),
    flag: measureGraphemeWidth("🇺🇸"),
    keycap: measureGraphemeWidth("1️⃣"),
    wide: measureGraphemeWidth("你"),
    ascii: measureGraphemeWidth("a"),
  }))

  assert.deepEqual(widths, {
    heart: 2,
    star: 2,
    flag: 2,
    keycap: 2,
    wide: 2,
    ascii: 1,
  })
})

function withRuntimeStringWidth<T>(
  runtimeStringWidth: ((value: string) => number) | undefined,
  run: () => T,
): T {
  const runtime = globalThis as {
    Bun?: { stringWidth?: (value: string) => number }
  }

  if (!runtime.Bun) {
    return run()
  }

  const original = runtime.Bun.stringWidth

  runtime.Bun.stringWidth = runtimeStringWidth

  try {
    return run()
  } finally {
    runtime.Bun.stringWidth = original
  }
}
