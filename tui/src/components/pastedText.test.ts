import { strict as assert } from "node:assert"
import { test } from "node:test"
import {
  collapsePastedText,
  collapsedPromptDisplayText,
  expandCollapsedPastes,
  filterReferencedCollapsedPastes,
  summarizeQueuedPromptText,
} from "./pastedText.js"

test("collapses large pasted text into a rows-inserted placeholder", () => {
  const pastedText = Array.from({ length: 30 }, (_, index) => `line ${index + 1}`).join("\n")
  const collapsed = collapsePastedText(pastedText, 3)

  assert.equal(collapsed.displayText, "[30 rows inserted #3]")
  assert.deepEqual(collapsed.pastes, [{
    id: 3,
    text: pastedText,
    rowCount: 30,
  }])
})

test("expands only referenced collapsed paste placeholders on submit", () => {
  const expanded = expandCollapsedPastes(
    "prefix [30 rows inserted #3] suffix",
    [{
      id: 3,
      text: "expanded body",
      rowCount: 30,
    }],
  )

  assert.equal(expanded, "prefix expanded body suffix")
})

test("drops hidden pasted bodies that are no longer referenced", () => {
  const filtered = filterReferencedCollapsedPastes(
    "keep [18 rows inserted #2]",
    [
      { id: 1, text: "drop me", rowCount: 12 },
      { id: 2, text: "keep me", rowCount: 18 },
    ],
  )

  assert.deepEqual(filtered, [{
    id: 2,
    text: "keep me",
    rowCount: 18,
  }])
})

test("collapses large raw user prompts on display and queue previews", () => {
  const prompt = Array.from({ length: 28 }, (_, index) => `paragraph ${index + 1}`).join("\n")

  assert.equal(collapsedPromptDisplayText(prompt), "[28 rows inserted]")
  assert.equal(summarizeQueuedPromptText(prompt), "[28 rows inserted]")
})
