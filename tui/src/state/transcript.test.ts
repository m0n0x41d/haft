import assert from "node:assert/strict"
import test from "node:test"
import type { ToolCall } from "../protocol/types.js"
import { initialState, reducer, type Action } from "./store.js"
import { buildTranscript } from "./transcript.js"

test("defers agent tool rows until after sibling non-agent tools", () => {
  const state = reduceActions([
    assistantMessage("assistant-1", [
      toolCall("read-1", "read", { path: "src/auth.ts" }),
      toolCall("agent-1", "spawn_agent", { agent_type: "explore", task: "Investigate auth flow" }),
      toolCall("bash-1", "bash", { command: "npm test" }),
    ]),
    { type: "tool.start", params: { callId: "read-1", name: "read", args: json({ path: "src/auth.ts" }) } },
    { type: "tool.start", params: { callId: "bash-1", name: "bash", args: json({ command: "npm test" }) } },
    { type: "tool.start", params: { callId: "agent-1", name: "spawn_agent", args: json({ agent_type: "explore", task: "Investigate auth flow" }) } },
    { type: "subagent.start", params: { subagentId: "sub-1", name: "explore", task: "Investigate auth flow" } },
    { type: "tool.start", params: { callId: "child-read-1", name: "read", args: json({ path: "src/routes.ts" }), subagentId: "sub-1" } },
  ])

  assert.deepEqual(transcriptToolCallIds(state), ["read-1", "bash-1", "agent-1"])
  assert.deepEqual(findAssistantTool(state, "agent-1")?.children?.map((tool) => tool.callId), ["child-read-1"])
})

test("matches concurrent subagents to their original spawn tool slot", () => {
  const state = reduceActions([
    assistantMessage("assistant-2", [
      toolCall("agent-a", "spawn_agent", { agent_type: "explore", task: "Audit auth" }),
      toolCall("read-2", "read", { path: "src/index.ts" }),
      toolCall("agent-b", "spawn_agent", { agent_type: "plan", task: "Draft rollout" }),
    ]),
    { type: "subagent.start", params: { subagentId: "sub-b", name: "plan", task: "Draft rollout" } },
    { type: "subagent.start", params: { subagentId: "sub-a", name: "explore", task: "Audit auth" } },
    { type: "tool.start", params: { callId: "child-b", name: "grep", args: json({ pattern: "TODO" }), subagentId: "sub-b" } },
    { type: "tool.start", params: { callId: "child-a", name: "read", args: json({ path: "src/auth.ts" }), subagentId: "sub-a" } },
  ])

  assert.equal(findAssistantTool(state, "agent-a")?.subagentId, "sub-a")
  assert.equal(findAssistantTool(state, "agent-b")?.subagentId, "sub-b")
  assert.deepEqual(findAssistantTool(state, "agent-a")?.children?.map((tool) => tool.callId), ["child-a"])
  assert.deepEqual(findAssistantTool(state, "agent-b")?.children?.map((tool) => tool.callId), ["child-b"])
  assert.deepEqual(transcriptToolCallIds(state), ["read-2", "agent-a", "agent-b"])
})

test("retains streamed tool state when msg.update only registers a partial tool list", () => {
  const state = reduceActions([
    { type: "msg.update", params: { id: "assistant-3", text: "Working", streaming: false } },
    { type: "tool.start", params: { callId: "read-9", name: "read", args: json({ path: "src/config.ts" }) } },
    { type: "tool.progress", params: { callId: "read-9", text: "line one\n" } },
    { type: "tool.start", params: { callId: "agent-9", name: "spawn_agent", args: json({ agent_type: "explore", task: "Trace settings" }) } },
    assistantMessage("assistant-3", [
      toolCall("agent-9", "spawn_agent", { agent_type: "explore", task: "Trace settings" }),
    ]),
  ])

  assert.equal(findAssistantTool(state, "read-9")?.output, "line one\n")
  assert.deepEqual(transcriptToolCallIds(state), ["read-9", "agent-9"])
})

function reduceActions(actions: Action[]): ReturnType<typeof initialState> {
  return actions.reduce(reducer, initialState())
}

function assistantMessage(id: string, tools: ToolCall[]): Action {
  return {
    type: "msg.update",
    params: {
      id,
      text: "Assistant update",
      tools,
      streaming: false,
    },
  }
}

function toolCall(callId: string, name: string, args: Record<string, unknown>): ToolCall {
  return {
    callId,
    name,
    args: json(args),
    running: false,
  }
}

function transcriptToolCallIds(state: ReturnType<typeof initialState>): string[] {
  const transcript = buildTranscript({
    messages: state.messages,
    streaming: false,
    streamingMsgId: null,
    thinkExpanded: false,
    error: null,
    model: "test-model",
  })

  return transcript
    .filter((entry) => entry.type === "toolCall")
    .map((entry) => entry.tool.callId)
}

function findAssistantTool(state: ReturnType<typeof initialState>, callId: string): ToolCall | undefined {
  const assistant = state.messages.find((message) => message.role === "assistant")

  return assistant?.tools?.find((tool) => tool.callId === callId)
}

function json(value: Record<string, unknown>): string {
  return JSON.stringify(value)
}
