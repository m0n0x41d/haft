/// <reference types="node" />

import assert from "node:assert/strict";
import test from "node:test";

import { isContinuationPrompt, visibleInitialPrompt } from "./taskPrompt.ts";

test("continuation envelope is not visible as a user prompt", () => {
  const prompt = [
    "Continue the existing desktop task.",
    "",
    "Task title:",
    "Hello",
    "",
    "Operator follow-up:",
    "what next?",
  ].join("\n");

  assert.equal(isContinuationPrompt(prompt), true);
  assert.equal(visibleInitialPrompt(prompt, []), "");
});

test("ordinary prompt remains visible until transcript contains it", () => {
  const prompt = "Build the feature";

  assert.equal(visibleInitialPrompt(prompt, []), prompt);
  assert.equal(
    visibleInitialPrompt(prompt, [
      {
        id: "user-1",
        type: "text",
        role: "user",
        text: prompt,
      },
    ]),
    "",
  );
});
