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

export function stripControlPromptSections(text: string): string {
  const withoutPrefixedSections = stripPrefixedControlPromptSections(text);

  return stripOrphanedControlPromptTail(withoutPrefixedSections);
}

function stripPrefixedControlPromptSections(text: string): string {
  if (!text.includes(CONTINUATION_PREFIX)) {
    return text;
  }

  const sections = text.split(CONTINUATION_PREFIX);
  const [head = "", ...tails] = sections;
  const visibleTails = tails.map(stripControlPromptTail);

  return [head, ...visibleTails].join("");
}

function stripControlPromptTail(section: string): string {
  const suffixIndex = section.indexOf(CONTINUATION_SUFFIX);

  if (suffixIndex === -1) {
    return "";
  }

  return section.slice(suffixIndex + CONTINUATION_SUFFIX.length);
}

function stripOrphanedControlPromptTail(text: string): string {
  if (!text.includes(CONTINUATION_FOLLOW_UP) || !text.includes(CONTINUATION_SUFFIX)) {
    return text;
  }

  let output = "";
  let remaining = text;

  while (true) {
    const start = remaining.indexOf(CONTINUATION_FOLLOW_UP);

    if (start === -1) {
      return `${output}${remaining}`;
    }

    const tail = remaining.slice(start);
    const endOffset = tail.indexOf(CONTINUATION_SUFFIX);

    if (endOffset === -1) {
      return `${output}${remaining}`;
    }

    const end = start + endOffset + CONTINUATION_SUFFIX.length;

    output = `${output}${remaining.slice(0, start)}`;
    remaining = remaining.slice(end);
  }
}
