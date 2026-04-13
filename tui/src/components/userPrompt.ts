import type { MessageAttachment } from "../protocol/types.js"

export const USER_PROMPT_PREFIX = " ❯ "
export const USER_PROMPT_CONTINUATION = "   "

export function buildUserPromptDisplayLines(
  text: string,
  attachments: readonly MessageAttachment[] = [],
): string[] {
  const sourceLines = text.split("\n")
  const [firstLine = "", ...continuationLines] = sourceLines
  const firstDisplayLine = `${USER_PROMPT_PREFIX}${firstLine}`
  const restDisplayLines = continuationLines.map((line) => `${USER_PROMPT_CONTINUATION}${line}`)
  const attachmentDisplayLines = attachments.map((attachment) => (
    `${USER_PROMPT_CONTINUATION}${formatUserPromptAttachmentLabel(attachment)}`
  ))

  return [firstDisplayLine, ...restDisplayLines, ...attachmentDisplayLines]
}

export function serializeUserPrompt(
  text: string,
  attachments: readonly MessageAttachment[] = [],
): string {
  const attachmentLines = attachments.map(formatUserPromptAttachmentLabel)
  const parts = [text, ...attachmentLines].filter((part) => part.length > 0)

  return parts.join("\n")
}

export function countWrappedUserPromptRows(
  text: string,
  width: number,
  attachments: readonly MessageAttachment[] = [],
): number {
  const safeWidth = Math.max(1, width)

  return buildUserPromptDisplayLines(text, attachments)
    .map((line) => countWrappedLineRows(line, safeWidth))
    .reduce((sum, rows) => sum + rows, 0)
}

function formatUserPromptAttachmentLabel(attachment: MessageAttachment): string {
  const kind = attachment.isImage ? "image" : "attachment"

  return `[${kind}: ${attachment.name}]`
}

function countWrappedLineRows(line: string, width: number): number {
  if (line.length === 0) {
    return 1
  }

  return Math.ceil(line.length / width)
}
