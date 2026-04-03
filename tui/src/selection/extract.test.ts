import assert from "node:assert/strict"
import test from "node:test"
import type { TranscriptRow } from "../state/transcript.js"
import { buildTranscriptLayout, computeRenderWindow, computeVisibleWindow } from "../scroll/measure.js"
import { termRowToEntryIndex, type ViewportLayout } from "./extract.js"

test("does not map blank rows below the painted viewport to hidden transcript entries", () => {
  const transcript = [assistantTextRow("assistant-1", "hello world")]
  const layout = buildTranscriptLayout(transcript, 24)
  const visibleWindow = computeVisibleWindow(
    layout,
    computeRenderWindow(layout, 0, 5, false),
    5,
  )
  const viewportLayout: ViewportLayout = {
    chatHeight: 5,
    atBottom: false,
    visibleWindow,
    entryHeights: layout.heights,
    transcript,
  }

  assert.equal(termRowToEntryIndex(1, viewportLayout), 0)
  assert.equal(termRowToEntryIndex(2, viewportLayout), 0)
  assert.equal(termRowToEntryIndex(3, viewportLayout), null)
  assert.equal(termRowToEntryIndex(5, viewportLayout), null)
})

function assistantTextRow(id: string, text: string): TranscriptRow {
  const measureToken = text
    .split("\n")
    .map((line) => line.length)
    .join(",")

  return {
    type: "assistantText",
    id,
    text,
    streaming: false,
    measureKey: ["assistantText", "static", measureToken].join(":"),
  }
}
