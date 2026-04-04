export const USER_PROMPT_PREFIX = " ❯ "
export const USER_PROMPT_CONTINUATION = "   "

export function buildUserPromptDisplayLines(text: string): string[] {
  const sourceLines = text.split("\n")
  const [firstLine = "", ...continuationLines] = sourceLines
  const firstDisplayLine = `${USER_PROMPT_PREFIX}${firstLine}`
  const restDisplayLines = continuationLines.map((line) => `${USER_PROMPT_CONTINUATION}${line}`)

  return [firstDisplayLine, ...restDisplayLines]
}

export function countWrappedUserPromptRows(text: string, width: number): number {
  const safeWidth = Math.max(1, width)

  return buildUserPromptDisplayLines(text)
    .map((line) => countWrappedLineRows(line, safeWidth))
    .reduce((sum, rows) => sum + rows, 0)
}

function countWrappedLineRows(line: string, width: number): number {
  if (line.length === 0) {
    return 1
  }

  return Math.ceil(line.length / width)
}
