import { strict as assert } from "node:assert"
import { test } from "node:test"
import {
  createPromptSubmission,
  drainPromptSubmissions,
  hasSubmittableText,
  leadingSlashCommand,
  queuedPromptReplayDisposition,
  restoreQueuedSubmission,
  shouldResumeQueuedReplayAfterCommandResolution,
  shouldResumeQueuedReplayAfterPickerCancel,
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

test("classifies queued slash commands by replay behavior", () => {
  assert.equal(leadingSlashCommand("/help later"), "/help")
  assert.equal(leadingSlashCommand("real prompt"), null)
  assert.equal(queuedPromptReplayDisposition("/help"), "pause")
  assert.equal(queuedPromptReplayDisposition("/model"), "pause")
  assert.equal(queuedPromptReplayDisposition("/resume"), "pause")
  assert.equal(queuedPromptReplayDisposition("/compact"), "pause")
  assert.equal(queuedPromptReplayDisposition("/search"), "submit")
  assert.equal(queuedPromptReplayDisposition("real prompt"), "submit")
})

test("marks queued interactive pickers for replay-on-cancel continuation", () => {
  assert.equal(shouldResumeQueuedReplayAfterPickerCancel("/help"), true)
  assert.equal(shouldResumeQueuedReplayAfterPickerCancel("/model"), true)
  assert.equal(shouldResumeQueuedReplayAfterPickerCancel("/resume"), true)
  assert.equal(shouldResumeQueuedReplayAfterPickerCancel("/compact"), false)
  assert.equal(shouldResumeQueuedReplayAfterPickerCancel("real prompt"), false)
})

test("marks queued compact for replay-on-resolution continuation", () => {
  assert.equal(shouldResumeQueuedReplayAfterCommandResolution("/compact"), true)
  assert.equal(shouldResumeQueuedReplayAfterCommandResolution("/help"), false)
  assert.equal(shouldResumeQueuedReplayAfterCommandResolution("/model"), false)
  assert.equal(shouldResumeQueuedReplayAfterCommandResolution("/resume"), false)
  assert.equal(shouldResumeQueuedReplayAfterCommandResolution("real prompt"), false)
})

test("replays queued help alone and leaves later prompts pending", () => {
  const queued = [
    createPromptSubmission("/help", []),
    createPromptSubmission("real prompt", [
      { id: 1, name: "image.png", path: "/tmp/image.png", isImage: true },
    ]),
    createPromptSubmission("later prompt", []),
  ]
  const drained = drainPromptSubmissions(queued)

  assert.deepEqual(
    submissionTexts(drained.replay),
    ["/help"],
  )
  assert.deepEqual(
    submissionTexts(drained.remaining),
    ["real prompt", "later prompt"],
  )
  assert.equal(queued[1]?.attachments[0]?.name, "image.png")
})

test("stops queued replay before a model picker command", () => {
  const queued = [
    createPromptSubmission("/model", []),
    createPromptSubmission("real prompt", []),
  ]
  const drained = drainPromptSubmissions(queued)

  assert.deepEqual(
    submissionTexts(drained.replay),
    ["/model"],
  )
  assert.deepEqual(
    submissionTexts(drained.remaining),
    ["real prompt"],
  )
})

test("stops queued replay before a resume picker command", () => {
  const queued = [
    createPromptSubmission("/resume", []),
    createPromptSubmission("real prompt", []),
  ]
  const drained = drainPromptSubmissions(queued)

  assert.deepEqual(
    submissionTexts(drained.replay),
    ["/resume"],
  )
  assert.deepEqual(
    submissionTexts(drained.remaining),
    ["real prompt"],
  )
})

test("stops queued replay before compaction completes", () => {
  const queued = [
    createPromptSubmission("/compact", []),
    createPromptSubmission("real prompt", []),
  ]
  const drained = drainPromptSubmissions(queued)

  assert.deepEqual(
    submissionTexts(drained.replay),
    ["/compact"],
  )
  assert.deepEqual(
    submissionTexts(drained.remaining),
    ["real prompt"],
  )
})
