const CONTINUATION_PREFIX = "Continue the existing desktop task.";
const CONTINUATION_FOLLOW_UP = "Operator follow-up:";
const CONTINUATION_SUFFIX =
  "Continue from the prior context. Do not repeat completed setup unless it is necessary.";

export function isControlPromptText(text: string): boolean {
  const normalized = text.trimStart();

  if (normalized.startsWith(CONTINUATION_PREFIX)) {
    return true;
  }

  if (normalized.includes(CONTINUATION_PREFIX)) {
    return true;
  }

  return normalized.includes(CONTINUATION_FOLLOW_UP)
    && normalized.includes(CONTINUATION_SUFFIX);
}
