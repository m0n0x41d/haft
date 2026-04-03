import { strict as assert } from "node:assert"
import { test } from "node:test"
import type { ToolCall } from "../protocol/types.js"
import { buildToolBatchDisplay, formatSubagentLabel } from "./toolBatch.js"

test("renders regular tools before spawned-agent parents while preserving order", () => {
  const tools: ToolCall[] = [
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
      running: true,
      subagent: {
        id: "sub-1",
        name: "research",
        task: "trace transcript state",
        running: true,
        tools: [
          {
            callId: "bash-child-1",
            name: "bash",
            args: "{\"command\":\"git status\"}",
            output: "On branch dev",
            running: false,
          },
        ],
      },
    },
    {
      callId: "bash-1",
      name: "bash",
      args: "{\"command\":\"pwd\"}",
      output: "/repo",
      running: false,
    },
    {
      callId: "read-1",
      name: "read",
      args: "{\"path\":\"tui/src/components/ToolCallView.tsx\"}",
      output: "file contents",
      running: false,
    },
  ]

  const display = buildToolBatchDisplay(tools)

  assert.deepEqual(display.map((item) => item.tool.callId), [
    "grep-1",
    "bash-1",
    "read-1",
    "spawn-1",
  ])
})

test("keeps live subagent activity attached to the spawned-agent parent row", () => {
  const tools: ToolCall[] = [
    {
      callId: "grep-1",
      name: "grep",
      args: "{\"pattern\":\"spawn_agent\"}",
      output: "match",
      running: false,
    },
    {
      callId: "spawn-1",
      name: "spawn_agent",
      args: "{\"task\":\"trace transcript state\"}",
      running: true,
      subagent: {
        id: "sub-1",
        name: "research",
        task: "trace transcript state",
        running: true,
        tools: [
          {
            callId: "grep-child-1",
            name: "grep",
            args: "{\"pattern\":\"ChatView\"}",
            output: "child match",
            running: false,
          },
          {
            callId: "bash-child-1",
            name: "bash",
            args: "{\"command\":\"git diff\"}",
            running: true,
            output: "streaming line\n",
          },
        ],
      },
    },
  ]

  const display = buildToolBatchDisplay(tools)
  const spawnedAgent = display[1]

  assert.ok(spawnedAgent)
  assert.equal(spawnedAgent.kind, "spawnedAgent")
  assert.equal(spawnedAgent.subagentLabel, "research - trace transcript state")
  assert.deepEqual(
    spawnedAgent.children.map((child) => child.tool.callId),
    ["grep-child-1", "bash-child-1"],
  )
})

test("deduplicates repeated subagent labels", () => {
  const label = formatSubagentLabel({
    id: "sub-1",
    name: "research",
    task: "research",
    running: false,
    tools: [],
  })

  assert.equal(label, "research")
})
