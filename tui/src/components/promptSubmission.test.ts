import { strict as assert } from "node:assert"
import { test } from "node:test"
import {
  createPromptSubmission,
  hasSubmittableText,
  restoreQueuedSubmission,
  shiftPromptSubmissions,
  submissionTexts,
} from "./promptSubmission.js"

test("preserves raw prompt text while rejecting all-whitespace submits", () => {
  assert.equal(hasSubmittableText("\n  keep indentation  \n"), true)
  assert.equal(hasSubmittableText("\n \t "), false)

  const submission = createPromptSubmission("\n  keep indentation  \n", [])

  assert.equal(submission.text, "\n  keep indentation  \n")
})

test("snapshots queued attachments instead of sharing mutable state", () => {
  const source = [
    { id: 1, name: "image.png", path: "/tmp/image.png", isImage: true },
  ]
  const submission = createPromptSubmission("next prompt", source)

  source[0]!.name = "mutated.png"

  assert.deepEqual(submissionTexts([submission]), ["next prompt"])
  assert.equal(submission.attachments[0]?.name, "image.png")
})

test("keeps queued attachments across manual edit and resend", () => {
  const queued = [
    createPromptSubmission("queued prompt", [
      { id: 1, name: "image.png", path: "/tmp/image.png", isImage: true },
    ]),
  ]
  const shifted = shiftPromptSubmissions(queued)
  const restored = shifted.current

  assert.equal(shifted.remaining.length, 0)
  assert.equal(restored?.attachments[0]?.name, "image.png")

  const resent = createPromptSubmission(
    `${restored?.text}\nwith edit`,
    restored?.attachments ?? [],
  )

  assert.equal(resent.attachments[0]?.name, "image.png")
  assert.equal(resent.text, "queued prompt\nwith edit")
})

test("restoring a queued draft keeps keyboard ownership with the prompt", () => {
  const queued = [
    createPromptSubmission("queued prompt", [
      { id: 1, name: "image.png", path: "/tmp/image.png", isImage: true },
    ]),
  ]
  const restored = restoreQueuedSubmission(queued)
  const edited = createPromptSubmission(
    `${restored.draft?.text} updated`,
    restored.draft?.attachments ?? [],
  )

  assert.equal(restored.attachmentSelection, false)
  assert.equal(restored.draft?.attachments[0]?.name, "image.png")
  assert.equal(edited.text, "queued prompt updated")
})
