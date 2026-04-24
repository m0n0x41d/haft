/// <reference types="node" />

import assert from "node:assert/strict";
import test from "node:test";

import { buildChatEntries, type ChatBlock } from "./api.ts";

test("control continuation prompt blocks are not rendered as chat entries", () => {
  const blocks: ChatBlock[] = [
    {
      id: "control",
      type: "text",
      role: "user",
      text: "Continue the existing desktop task.\n\nOperator follow-up:\nhello",
    },
    {
      id: "user",
      type: "text",
      role: "user",
      text: "hello",
    },
  ];

  const entries = buildChatEntries(blocks);

  assert.equal(entries.length, 1);
  assert.equal(entries[0].block.id, "user");
});

test("partially parsed control continuation prompt blocks are not rendered", () => {
  const blocks: ChatBlock[] = [
    {
      id: "control",
      type: "text",
      role: "user",
      text: [
        '{"type":"result","usage":{"input_tokens":6}}',
        "",
        "Operator follow-up:",
        "how are you?",
        "",
        "Continue from the prior context. Do not repeat completed setup unless it is necessary.",
      ].join("\n"),
    },
    {
      id: "assistant",
      type: "text",
      role: "assistant",
      text: "Running fine.",
    },
  ];

  const entries = buildChatEntries(blocks);

  assert.equal(entries.length, 1);
  assert.equal(entries[0].block.id, "assistant");
});
