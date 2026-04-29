/// <reference types="node" />

import assert from "node:assert/strict";
import test from "node:test";

import { buildChatEntries, taskTranscriptText, type ChatBlock } from "./api.ts";

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

test("raw provider envelopes are not rendered as chat entries", () => {
  const blocks: ChatBlock[] = [
    {
      id: "claude-result",
      type: "text",
      role: "assistant",
      text: '{"type":"result","usage":{"input_tokens":6},"result":{"duration_ms":10}}',
    },
    {
      id: "codex-turn",
      type: "text",
      role: "assistant",
      text: '{"type":"turn.completed","usage":{"input_tokens":9}}',
    },
    {
      id: "provider-batch",
      type: "text",
      role: "assistant",
      text: [
        '{"type":"system","subtype":"init"}',
        '{"type":"rate_limit_event","message":"ok"}',
      ].join("\n"),
    },
    {
      id: "assistant",
      type: "text",
      role: "assistant",
      text: "Visible response.",
    },
  ];

  const entries = buildChatEntries(blocks);

  assert.equal(entries.length, 1);
  assert.equal(entries[0].block.id, "assistant");
  assert.equal(entries[0].block.text, "Visible response.");
});

test("provider envelope lines inside visible chat blocks stay audit-only", () => {
  const blocks: ChatBlock[] = [
    {
      id: "assistant",
      type: "text",
      role: "assistant",
      text: [
        "Visible before.",
        '{"type":"turn.completed","usage":{"input_tokens":9}}',
        "Visible after.",
      ].join("\n"),
    },
  ];

  const entries = buildChatEntries(blocks);

  assert.equal(entries.length, 1);
  assert.equal(entries[0].block.text, "Visible before.\nVisible after.");
});

test("third and fourth continuation turns render only durable conversation messages", () => {
  const blocks: ChatBlock[] = [
    {
      id: "initial",
      type: "text",
      role: "user",
      text: "Original request",
    },
    {
      id: "assistant-1",
      type: "text",
      role: "assistant",
      text: "Completed first pass.",
    },
    {
      id: "control-2",
      type: "text",
      role: "user",
      text: "Continue the existing desktop task.\n\nOperator follow-up:\nSecond follow-up",
    },
    {
      id: "follow-up-2",
      type: "text",
      role: "user",
      text: "Second follow-up",
    },
    {
      id: "assistant-2",
      type: "text",
      role: "assistant",
      text: "Checkpoint saved.",
    },
    {
      id: "provider-result",
      type: "text",
      role: "assistant",
      text: '{"type":"result","usage":{"input_tokens":6}}',
    },
    {
      id: "control-3",
      type: "text",
      role: "user",
      text: [
        "Operator follow-up:",
        "Third follow-up",
        "",
        "Continue from the prior context. Do not repeat completed setup unless it is necessary.",
      ].join("\n"),
    },
    {
      id: "follow-up-3",
      type: "text",
      role: "user",
      text: "Third follow-up",
    },
    {
      id: "assistant-3",
      type: "text",
      role: "assistant",
      text: "Failed while verifying.",
    },
    {
      id: "provider-batch",
      type: "text",
      role: "assistant",
      text: [
        '{"type":"system","subtype":"init"}',
        '{"type":"turn.completed","usage":{"input_tokens":9}}',
      ].join("\n"),
    },
    {
      id: "control-4",
      type: "text",
      role: "user",
      text: [
        "Continue the existing desktop task.",
        "",
        "Operator follow-up:",
        "Fourth follow-up",
        "",
        "Continue from the prior context. Do not repeat completed setup unless it is necessary.",
      ].join("\n"),
    },
    {
      id: "follow-up-4",
      type: "text",
      role: "user",
      text: "Fourth follow-up",
    },
    {
      id: "assistant-4",
      type: "text",
      role: "assistant",
      text: "Blocked on operator approval.",
    },
  ];

  const entries = buildChatEntries(blocks);
  const visibleTurns = entries.map((entry) => [
    entry.block.role,
    entry.block.text,
  ]);
  const renderedText = entries
    .map((entry) => entry.block.text ?? "")
    .join("\n");

  assert.deepEqual(visibleTurns, [
    ["user", "Original request"],
    ["assistant", "Completed first pass."],
    ["user", "Second follow-up"],
    ["assistant", "Checkpoint saved."],
    ["user", "Third follow-up"],
    ["assistant", "Failed while verifying."],
    ["user", "Fourth follow-up"],
    ["assistant", "Blocked on operator approval."],
  ]);
  assert.equal(renderedText.includes('"type":"result"'), false);
  assert.equal(renderedText.includes("Operator follow-up:"), false);
  assert.equal(renderedText.includes("Continue the existing desktop task."), false);
});

test("raw fallback transcript hides provider envelopes and continuation control prompts", () => {
  const transcript = [
    "Visible before",
    '{"type":"result","usage":{"input_tokens":6}}',
    "Continue the existing desktop task.",
    "",
    "Task title:",
    "Original task",
    "",
    "Operator follow-up:",
    "Continue please",
    "",
    "Continue from the prior context. Do not repeat completed setup unless it is necessary.",
    "Visible after",
  ].join("\n");
  const visibleTranscript = taskTranscriptText({
    raw_output: transcript,
    output: "",
  });

  assert.equal(visibleTranscript.includes("Visible before"), true);
  assert.equal(visibleTranscript.includes("Visible after"), true);
  assert.equal(visibleTranscript.includes('"type":"result"'), false);
  assert.equal(visibleTranscript.includes("Operator follow-up:"), false);
  assert.equal(visibleTranscript.includes("Continue the existing desktop task."), false);
});
