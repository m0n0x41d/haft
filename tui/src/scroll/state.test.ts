import { strict as assert } from "node:assert"
import { test } from "node:test"
import { initialScroll, reduceScroll, unreadLinesBelow } from "./state.js"

test("starts in sticky mode at the live bottom", () => {
  const state = initialScroll()

  assert.equal(state.mode, "sticky")
  assert.equal(state.offset, 0)
})

test("moves into reading mode when the user scrolls up", () => {
  const initial = {
    ...initialScroll(),
    totalLines: 80,
    viewportSize: 20,
  }
  const state = reduceScroll(initial, { type: "wheelUp", amount: 4 })

  assert.equal(state.mode, "reading")
  assert.equal(state.readingStartTotalLines, 80)
  assert.equal(state.offset, 4)
  assert.equal(unreadLinesBelow(state), 0)
})

test("returns to sticky mode only on an explicit jump to end", () => {
  const reading = {
    ...initialScroll(),
    mode: "reading" as const,
    offset: 12,
    totalLines: 80,
    viewportSize: 20,
  }
  const state = reduceScroll(reading, { type: "end" })

  assert.equal(state.mode, "sticky")
  assert.equal(state.readingStartTotalLines, null)
  assert.equal(state.offset, 0)
  assert.equal(unreadLinesBelow(state), 0)
})

test("preserves reader position when content grows in reading mode", () => {
  const reading = {
    ...initialScroll(),
    mode: "reading" as const,
    readingStartTotalLines: 30,
    offset: 5,
    totalLines: 30,
    viewportSize: 10,
  }
  const state = reduceScroll(reading, { type: "contentChanged", newTotalLines: 34 })

  assert.equal(state.mode, "reading")
  assert.equal(state.offset, 9)
  assert.equal(state.totalLines, 34)
  assert.equal(state.readingStartTotalLines, 30)
  assert.equal(unreadLinesBelow(state), 4)
})

test("keeps the live tail pinned while sticky content grows", () => {
  const sticky = {
    ...initialScroll(),
    totalLines: 30,
    viewportSize: 10,
  }
  const state = reduceScroll(sticky, { type: "contentChanged", newTotalLines: 34 })

  assert.equal(state.mode, "sticky")
  assert.equal(state.offset, 0)
  assert.equal(state.totalLines, 34)
  assert.equal(unreadLinesBelow(state), 0)
})

test("stays in reading mode even after scrolling back down to the old bottom", () => {
  const reading = {
    ...initialScroll(),
    mode: "reading" as const,
    readingStartTotalLines: 50,
    offset: 3,
    totalLines: 50,
    viewportSize: 12,
  }
  const state = reduceScroll(reading, { type: "wheelDown", amount: 3 })

  assert.equal(state.mode, "reading")
  assert.equal(state.offset, 0)
  assert.equal(unreadLinesBelow(state), 0)
})

test("does not mark unread content below just because the user scrolled up", () => {
  const sticky = {
    ...initialScroll(),
    totalLines: 60,
    viewportSize: 20,
  }
  const reading = reduceScroll(sticky, { type: "pageUp" })

  assert.equal(reading.mode, "reading")
  assert.equal(reading.readingStartTotalLines, 60)
  assert.equal(unreadLinesBelow(reading), 0)
})
