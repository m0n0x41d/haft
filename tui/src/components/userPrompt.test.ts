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

test("shows submitted large prompts in full instead of collapsing them", () => {
  const prompt = Array.from({ length: 30 }, (_, index) => `article line ${index + 1}`).join("\n")
  const lines = buildUserPromptDisplayLines(prompt)

  assert.equal(lines.length, 30)
  assert.equal(lines[0], " ❯ article line 1")
  assert.equal(lines[29], "   article line 30")
})
