import type { ChatBlock } from "./api";
import { isControlPromptText } from "./controlPrompt.ts";

export function isContinuationPrompt(prompt: string): boolean {
  return isControlPromptText(prompt);
}

export function visibleInitialPrompt(prompt: string, chatBlocks: ChatBlock[]): string {
  const trimmedPrompt = prompt.trim();

  if (trimmedPrompt === "" || isContinuationPrompt(trimmedPrompt)) {
    return "";
  }

  const transcriptHasPrompt = chatBlocks.some((block) =>
    block.role === "user" && (block.text ?? "").trim() === trimmedPrompt,
  );

  if (transcriptHasPrompt) {
    return "";
  }

  return trimmedPrompt;
}
