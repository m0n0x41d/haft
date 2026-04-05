import { strict as assert } from "node:assert"
import { EventEmitter } from "node:events"
import { test } from "node:test"
import { buildTranscript } from "../state/transcript.js"
import { buildInputDisplayLayout, measureInputDisplayRows } from "./inputLayout.js"
import { computeBottomRows, computeChatHeight } from "./appLayout.js"
import { buildUserPromptDisplayLines } from "./userPrompt.js"
import { collapsePastedText } from "./pastedText.js"
import { createInputRouter } from "../terminal/inputStream.js"
import { empty, insertAt, moveDown, moveEnd, moveHome, moveUp, cursorPosition } from "../input/editBuffer.js"
import { emptyHistory, currentText, navigateUp, push } from "../input/history.js"

// Contract tests for tui/MULTILINE_UX_CHECKLIST.md.

type FakeTTYInput = EventEmitter & {
  isRaw: boolean
  setRawMode: (mode: boolean) => FakeTTYInput
  resume: () => FakeTTYInput
  pause: () => FakeTTYInput
}

function createFakeTTYInput(): FakeTTYInput {
  const ttyInput = new EventEmitter() as FakeTTYInput

  ttyInput.isRaw = false
  ttyInput.setRawMode = (mode: boolean) => {
    ttyInput.isRaw = mode
    return ttyInput
  }
  ttyInput.resume = () => ttyInput
  ttyInput.pause = () => ttyInput

  return ttyInput
}

test("collapses a large bracketed paste into a rows-inserted placeholder", () => {
  const ttyInput = createFakeTTYInput()
  const router = createInputRouter(ttyInput as never)
  const pastedChunks: string[] = []
  const pastedText = [
    ...Array.from({ length: 28 }, (_, index) => `line ${index + 1}`),
  ].join("\n")

  router.events.on("paste", (text: string) => pastedChunks.push(text))

  ttyInput.emit("data", `\x1b[200~${pastedText}\x1b[201~`)

  assert.deepEqual(pastedChunks, [pastedText])
  assert.equal(collapsePastedText(pastedText, 1).displayText, "[28 rows inserted #1]")

  router.destroy()
})

test("keeps typed multiline input and attachment rows in the visible bottom layout", () => {
  const multilinePrompt = insertAt(
    insertAt(
      insertAt(empty, "first line"),
      "\n",
    ),
    "second line",
  )
  const home = moveHome(multilinePrompt)
  const end = moveEnd(home)
  const inputRows = measureInputDisplayRows({
    text: multilinePrompt.text,
    cursor: multilinePrompt.cursor,
    width: 18,
    hasQueuedMessages: false,
  })
  const withoutAttachments = computeBottomRows({
    width: 18,
    queuedMessages: [],
    attachments: [],
    attachmentSelection: false,
    inputRows,
    showInput: true,
  })
  const withImageAttachment = computeBottomRows({
    width: 18,
    queuedMessages: [],
    attachments: [{
      id: 1,
      name: "clipboard.png",
      path: "/tmp/clipboard.png",
      isImage: true,
    }],
    attachmentSelection: false,
    inputRows,
    showInput: true,
  })

  assert.deepEqual(cursorPosition(multilinePrompt), { line: 1, col: 11 })
  assert.equal(home.cursor, "first line\n".length)
  assert.equal(end.cursor, multilinePrompt.cursor)
  assert.ok(inputRows >= 2)
  assert.ok(withImageAttachment > withoutAttachments)
  assert.ok(computeChatHeight(18, withImageAttachment) > 0)
})

test("round-trips multiline history into transcript rendering without attachment guesses", () => {
  const prompt = "first line\n[not an attachment]\nthird line"
  const history = push(emptyHistory, prompt)
  const recalled = navigateUp(history, "")

  assert.ok(recalled)
  assert.equal(currentText(recalled), prompt)

  const entries = buildTranscript({
    messages: [{
      id: "user-1",
      role: "user",
      text: currentText(recalled),
      attachments: [{ name: "clipboard.png", isImage: true }],
    }],
    streaming: false,
    streamingMsgId: null,
    thinkExpanded: false,
    error: null,
    model: "model",
  })
  const firstEntry = entries[0]

  assert.equal(firstEntry?.type, "userPrompt")

  if (firstEntry?.type !== "userPrompt") {
    throw new Error("expected user prompt transcript entry")
  }

  assert.deepEqual(
    buildUserPromptDisplayLines(firstEntry.text, firstEntry.attachments),
    [
      " ❯ first line",
      "   [not an attachment]",
      "   third line",
      "   [image: clipboard.png]",
    ],
  )
})

test("collapses large raw transcript prompts into a compact display line", () => {
  const prompt = Array.from({ length: 32 }, (_, index) => `article line ${index + 1}`).join("\n")

  assert.deepEqual(
    buildUserPromptDisplayLines(prompt),
    [" ❯ [32 rows inserted]"],
  )
})

test("keeps up and down arrows inside a multiline prompt until the boundary", () => {
  const prompt = {
    text: "first line\nsecond line\nthird line",
    cursor: 17,
  }
  const movedUp = moveUp(prompt)
  const movedDown = moveDown(prompt)
  const firstLine = { ...prompt, cursor: 6 }
  const lastLine = { ...prompt, cursor: 29 }

  assert.equal(movedUp.cursor, 6)
  assert.equal(movedDown.cursor, 29)
  assert.equal(moveUp(firstLine).cursor, firstLine.cursor)
  assert.equal(moveDown(lastLine).cursor, lastLine.cursor)
})
