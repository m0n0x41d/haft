import assert from "node:assert/strict";
import test from "node:test";

import {
  buildChatEntries,
  hasStructuredChatBlocks,
  taskTranscriptText,
} from "../src/lib/api.ts";

test("legacy tasks fall back to persisted raw transcript", () => {
  const task = {
    chat_blocks: [],
    raw_output: "legacy raw transcript",
    output: "structured display tail",
  };

  assert.equal(hasStructuredChatBlocks(task), false);
  assert.equal(taskTranscriptText(task), "legacy raw transcript");
});

test("user-only narrative blocks do not force structured transcript mode", () => {
  const task = {
    chat_blocks: [
      {
        id: "block-user-1",
        type: "text",
        role: "user",
        text: "Inspect the runtime state",
      },
    ],
  };

  assert.equal(hasStructuredChatBlocks(task), false);
});

test("tool results stay nested under their matching tool call", () => {
  const entries = buildChatEntries([
    {
      id: "block-user-1",
      type: "text",
      role: "user",
      text: "Inspect the runtime state",
    },
    {
      id: "block-tool-1",
      type: "tool_use",
      role: "assistant",
      name: "exec_command",
      call_id: "call-1",
      input: "go build ./cmd/haft/",
    },
    {
      id: "block-tool-result-1",
      type: "tool_result",
      role: "assistant",
      call_id: "call-1",
      output: "ok",
    },
    {
      id: "block-assistant-1",
      type: "text",
      role: "assistant",
      text: "Build passed.",
    },
  ]);

  assert.equal(entries.length, 3);
  assert.equal(entries[1].block.type, "tool_use");
  assert.equal(entries[1].toolResults.length, 1);
  assert.equal(entries[1].toolResults[0].type, "tool_result");
  assert.equal(entries[1].toolResults[0].parent_id, "block-tool-1");
  assert.equal(entries[2].block.text, "Build passed.");
});
