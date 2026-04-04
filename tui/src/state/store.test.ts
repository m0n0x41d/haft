import { strict as assert } from "node:assert"
import { test } from "node:test"
import { initialState, reducer, type Action } from "./store.js"

function reduceActions(actions: Action[]) {
  return actions.reduce(reducer, initialState())
}

test("keeps one canonical spawned-agent structure across reducer updates", () => {
  const state = reduceActions([
    {
      type: "msg.update",
      params: {
        id: "msg-1",
        text: "",
        streaming: true,
        tools: [
          {
            callId: "grep-1",
            name: "grep",
            args: "{\"pattern\":\"todo\"}",
            running: true,
          },
          {
            callId: "spawn-1",
            name: "spawn_agent",
            args: "{\"task\":\"Investigate scroll issue\"}",
            running: true,
          },
        ],
      },
    },
    {
      type: "tool.start",
      params: {
        callId: "grep-1",
        name: "grep",
        args: "{\"pattern\":\"todo\"}",
      },
    },
    {
      type: "tool.done",
      params: {
        callId: "grep-1",
        name: "grep",
        output: "matched line",
        isError: false,
      },
    },
    {
      type: "subagent.start",
      params: {
        subagentId: "sub-1",
        parentCallId: "spawn-1",
        name: "research",
        task: "Investigate scroll issue",
      },
    },
    {
      type: "tool.start",
      params: {
        callId: "bash-1",
        name: "bash",
        args: "{\"command\":\"git status\"}",
        subagentId: "sub-1",
      },
    },
    {
      type: "tool.progress",
      params: {
        callId: "bash-1",
        text: "streaming line\n",
      },
    },
    {
      type: "tool.done",
      params: {
        callId: "bash-1",
        name: "bash",
        output: "child output",
        isError: false,
        subagentId: "sub-1",
      },
    },
    {
      type: "subagent.done",
      params: {
        subagentId: "sub-1",
        summary: "final summary",
        isError: false,
      },
    },
    {
      type: "tool.done",
      params: {
        callId: "spawn-1",
        name: "spawn_agent",
        output: "final summary",
        isError: false,
      },
    },
    {
      type: "msg.update",
      params: {
        id: "msg-1",
        text: "",
        streaming: true,
        tools: [
          {
            callId: "grep-1",
            name: "grep",
            args: "{\"pattern\":\"todo\"}",
            running: true,
          },
          {
            callId: "spawn-1",
            name: "spawn_agent",
            args: "{\"task\":\"Investigate scroll issue\"}",
            running: true,
          },
        ],
      },
    },
  ])

  const message = state.messages[0]
  assert.ok(message)
  assert.equal(message.tools?.length, 2)

  const grepTool = message.tools?.[0]
  const spawnTool = message.tools?.[1]

  assert.ok(grepTool)
  assert.ok(spawnTool)
  assert.equal(grepTool.output, "matched line")
  assert.equal(grepTool.running, false)

  assert.equal(spawnTool.output, undefined)
  assert.equal(spawnTool.running, false)
  assert.equal(spawnTool.subagent?.id, "sub-1")
  assert.equal(spawnTool.subagent?.name, "research")
  assert.equal(spawnTool.subagent?.task, "Investigate scroll issue")
  assert.equal(spawnTool.subagent?.summary, "final summary")
  assert.equal(spawnTool.subagent?.running, false)
  assert.equal(spawnTool.subagent?.tools.length, 1)
  assert.equal(spawnTool.subagent?.tools[0]?.callId, "bash-1")
  assert.equal(spawnTool.subagent?.tools[0]?.output, "child output")
  assert.equal(state.activeSubagents, 0)
})

test("normalizes legacy wire children into explicit subagent state", () => {
  const state = reducer(initialState(), {
    type: "init",
    session: { id: "session-1", title: "Title", model: "model" },
    projectRoot: "/repo",
    messages: [
      {
        id: "msg-1",
        role: "assistant",
        text: "",
        tools: [
          {
            callId: "spawn-1",
            name: "spawn_agent",
            args: "{\"task\":\"Legacy task\"}",
            output: "legacy summary",
            isError: false,
            running: false,
            subagentId: "legacy-subagent",
            children: [
              {
                callId: "bash-1",
                name: "bash",
                args: "{\"command\":\"pwd\"}",
                output: "/repo",
                isError: false,
                running: false,
              },
            ],
          },
        ],
      },
    ],
  })

  const spawnTool = state.messages[0]?.tools?.[0]

  assert.ok(spawnTool)
  assert.equal(spawnTool.output, undefined)
  assert.equal(spawnTool.subagent?.id, "legacy-subagent")
  assert.equal(spawnTool.subagent?.task, "Legacy task")
  assert.equal(spawnTool.subagent?.summary, "legacy summary")
  assert.equal(spawnTool.subagent?.tools.length, 1)
  assert.equal(spawnTool.subagent?.tools[0]?.callId, "bash-1")
})

test("ignores unmatched subagent lifecycle events", () => {
  const state = reduceActions([
    {
      type: "msg.update",
      params: {
        id: "msg-1",
        text: "",
        streaming: true,
        tools: [
          {
            callId: "spawn-1",
            name: "spawn_agent",
            args: "{\"task\":\"Investigate scroll issue\"}",
            running: true,
          },
        ],
      },
    },
    {
      type: "subagent.start",
      params: {
        subagentId: "sub-missing",
        parentCallId: "spawn-missing",
        name: "research",
        task: "Should not attach",
      },
    },
    {
      type: "tool.start",
      params: {
        callId: "bash-missing",
        name: "bash",
        args: "{\"command\":\"pwd\"}",
        subagentId: "sub-missing",
      },
    },
    {
      type: "subagent.done",
      params: {
        subagentId: "sub-missing",
        summary: "Should be ignored",
        isError: false,
      },
    },
  ])

  const spawnTool = state.messages[0]?.tools?.[0]

  assert.ok(spawnTool)
  assert.equal(spawnTool.subagent, undefined)
  assert.equal(state.activeSubagents, 0)
})

test("matches interleaved subagent events to exact parent tool call ids", () => {
  const state = reduceActions([
    {
      type: "msg.update",
      params: {
        id: "msg-1",
        text: "",
        streaming: true,
        tools: [
          {
            callId: "spawn-1",
            name: "spawn_agent",
            args: "{\"task\":\"Inspect viewport\"}",
            running: true,
          },
          {
            callId: "spawn-2",
            name: "spawn_agent",
            args: "{\"task\":\"Inspect subagent ordering\"}",
            running: true,
          },
        ],
      },
    },
    {
      type: "subagent.start",
      params: {
        subagentId: "sub-2",
        parentCallId: "spawn-2",
        name: "ordering",
        task: "Inspect subagent ordering",
      },
    },
    {
      type: "subagent.start",
      params: {
        subagentId: "sub-1",
        parentCallId: "spawn-1",
        name: "viewport",
        task: "Inspect viewport",
      },
    },
    {
      type: "tool.start",
      params: {
        callId: "bash-2",
        name: "bash",
        args: "{\"command\":\"git status\"}",
        subagentId: "sub-2",
      },
    },
    {
      type: "tool.start",
      params: {
        callId: "grep-1",
        name: "grep",
        args: "{\"pattern\":\"viewport\"}",
        subagentId: "sub-1",
      },
    },
    {
      type: "tool.done",
      params: {
        callId: "grep-1",
        name: "grep",
        output: "viewport.tsx:42",
        isError: false,
        subagentId: "sub-1",
      },
    },
    {
      type: "tool.done",
      params: {
        callId: "bash-2",
        name: "bash",
        output: "working tree clean",
        isError: false,
        subagentId: "sub-2",
      },
    },
    {
      type: "subagent.done",
      params: {
        subagentId: "sub-1",
        summary: "viewport summary",
        isError: false,
      },
    },
    {
      type: "subagent.done",
      params: {
        subagentId: "sub-2",
        summary: "ordering summary",
        isError: false,
      },
    },
  ])

  const firstSpawn = state.messages[0]?.tools?.[0]
  const secondSpawn = state.messages[0]?.tools?.[1]

  assert.ok(firstSpawn)
  assert.ok(secondSpawn)
  assert.equal(firstSpawn.subagent?.id, "sub-1")
  assert.equal(firstSpawn.subagent?.name, "viewport")
  assert.equal(firstSpawn.subagent?.summary, "viewport summary")
  assert.equal(firstSpawn.subagent?.tools[0]?.callId, "grep-1")
  assert.equal(firstSpawn.subagent?.tools[0]?.output, "viewport.tsx:42")

  assert.equal(secondSpawn.subagent?.id, "sub-2")
  assert.equal(secondSpawn.subagent?.name, "ordering")
  assert.equal(secondSpawn.subagent?.summary, "ordering summary")
  assert.equal(secondSpawn.subagent?.tools[0]?.callId, "bash-2")
  assert.equal(secondSpawn.subagent?.tools[0]?.output, "working tree clean")

  assert.equal(state.activeSubagents, 0)
})

test("keeps structured attachments separate from the user prompt text", () => {
  const state = reduceActions([
    {
      type: "msg.update",
      params: {
        id: "user-1",
        text: "[not an attachment]",
        attachments: [
          {
            name: "clipboard.png",
            isImage: true,
          },
        ],
        streaming: false,
      },
    },
  ])

  assert.deepEqual(state.messages, [
    {
      id: "user-1",
      role: "user",
      text: "[not an attachment]",
      attachments: [
        {
          name: "clipboard.png",
          isImage: true,
        },
      ],
      thinking: undefined,
      tools: undefined,
    },
  ])
})
