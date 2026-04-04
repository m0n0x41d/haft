import { strict as assert } from "node:assert"
import { test } from "node:test"
import type { ChatMessage } from "../protocol/types.js"
import { buildTranscript } from "./transcript.js"

test("groups assistant tools into one batch entry", () => {
  const messages: ChatMessage[] = [
    {
      id: "user-1",
      role: "user",
      text: "Fix the TUI\n[file.txt]",
    },
    {
      id: "assistant-1",
      role: "assistant",
      text: "Working on it",
      thinking: "step 1\nstep 2",
      tools: [
        {
          callId: "grep-1",
          name: "grep",
          args: "{\"pattern\":\"ToolCallView\"}",
          output: "match",
          running: false,
        },
        {
          callId: "spawn-1",
          name: "spawn_agent",
          args: "{\"task\":\"trace transcript state\"}",
          running: false,
          subagent: {
            id: "sub-1",
            name: "research",
            task: "trace transcript state",
            running: false,
            summary: "done",
            tools: [],
          },
        },
      ],
    },
  ]

  const entries = buildTranscript({
    messages,
    streaming: false,
    streamingMsgId: null,
    thinkExpanded: true,
    error: null,
    model: "model",
  })

  assert.deepEqual(entries.map((entry) => entry.type), [
    "userPrompt",
    "thinking",
    "assistantText",
    "assistantToolBatch",
  ])

  const batch = entries[3]
  assert.equal(batch?.type, "assistantToolBatch")

  if (batch?.type !== "assistantToolBatch") {
    throw new Error("expected assistant tool batch")
  }

  assert.equal(batch.tools.length, 2)
  assert.equal(batch.tools[0]?.callId, "grep-1")
  assert.equal(batch.tools[1]?.subagent?.id, "sub-1")
})

test("keeps the thinking indicator when streaming has no text or tools yet", () => {
  const entries = buildTranscript({
    messages: [
      {
        id: "assistant-1",
        role: "assistant",
        text: "",
      },
    ],
    streaming: true,
    streamingMsgId: "assistant-1",
    thinkExpanded: false,
    error: null,
    model: "gpt-test",
  })

  assert.deepEqual(entries, [
    {
      type: "indicator",
      id: "thinking-indicator",
      model: "gpt-test",
    },
  ])
})

test("preserves the full multiline user prompt text in transcript entries", () => {
  const entries = buildTranscript({
    messages: [
      {
        id: "user-1",
        role: "user",
        text: "Fix the transcript\n[not an attachment]\nKeep every line",
      },
    ],
    streaming: false,
    streamingMsgId: null,
    thinkExpanded: false,
    error: null,
    model: "model",
  })

  assert.deepEqual(entries, [
    {
      type: "userPrompt",
      id: "user-1-user",
      text: "Fix the transcript\n[not an attachment]\nKeep every line",
    },
  ])
})
