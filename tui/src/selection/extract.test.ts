import { strict as assert } from "node:assert"
import { test } from "node:test"
import type { TranscriptEntry } from "../state/transcript.js"
import { computeOffsets, computeVisibleWindow } from "../scroll/measure.js"
import { extractSelection, termRowToEntryIndex, type ViewportLayout } from "./extract.js"

function createLayout(
  transcript: readonly TranscriptEntry[],
  entryHeights: readonly number[],
  chatHeight: number,
  offset: number,
  atBottom: boolean,
): ViewportLayout {
  const entryOffsets = computeOffsets(entryHeights)

  return {
    chatHeight,
    atBottom,
    visibleWindow: computeVisibleWindow(entryOffsets, offset, chatHeight, 2),
    entryHeights,
    entryOffsets,
    transcript,
  }
}

test("maps terminal rows through the measured viewport without cropTop", () => {
  const transcript: TranscriptEntry[] = [
    { type: "assistantText", id: "entry-1", text: "entry one", streaming: false },
    { type: "assistantText", id: "entry-2", text: "entry two", streaming: false },
    { type: "assistantText", id: "entry-3", text: "entry three", streaming: false },
  ]
  const layout = createLayout(transcript, [3, 4, 2], 4, 1, false)

  assert.equal(termRowToEntryIndex(1, layout), 1)
  assert.equal(termRowToEntryIndex(4, layout), 2)
})

test("ignores top padding rows when the transcript is shorter than the viewport", () => {
  const transcript: TranscriptEntry[] = [
    { type: "assistantText", id: "entry-1", text: "entry one", streaming: false },
  ]
  const layout = createLayout(transcript, [2], 5, 0, true)

  assert.equal(termRowToEntryIndex(1, layout), null)
  assert.equal(termRowToEntryIndex(4, layout), 0)
  assert.equal(termRowToEntryIndex(5, layout), 0)
})

test("extracts entry text from the measured viewport range", () => {
  const transcript: TranscriptEntry[] = [
    { type: "assistantText", id: "entry-1", text: "entry one", streaming: false },
    { type: "assistantText", id: "entry-2", text: "entry two", streaming: false },
    { type: "assistantText", id: "entry-3", text: "entry three", streaming: false },
  ]
  const layout = createLayout(transcript, [3, 4, 2], 4, 1, false)
  const selectedText = extractSelection(1, 4, layout)

  assert.equal(selectedText, "entry two\n\nentry three")
})
