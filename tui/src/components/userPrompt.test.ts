import { strict as assert } from "node:assert"
import { test } from "node:test"
import { buildUserPromptDisplayLines } from "./userPrompt.js"

test("formats multiline user prompts without stripping bracket-prefixed lines", () => {
  const lines = buildUserPromptDisplayLines(
    "first line\n[not an attachment]\nthird line",
    [
      { name: "clipboard.png", isImage: true },
      { name: "spec.md", isImage: false },
    ],
  )

  assert.deepEqual(lines, [
    " ❯ first line",
    "   [not an attachment]",
    "   third line",
    "   [image: clipboard.png]",
    "   [attachment: spec.md]",
  ])
})
