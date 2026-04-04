import { strict as assert } from "node:assert"
import { test } from "node:test"
import { buildUserPromptDisplayLines } from "./userPrompt.js"

test("formats multiline user prompts without stripping bracket-prefixed lines", () => {
  const lines = buildUserPromptDisplayLines("first line\n[not an attachment]\nthird line")

  assert.deepEqual(lines, [
    " ❯ first line",
    "   [not an attachment]",
    "   third line",
  ])
})
