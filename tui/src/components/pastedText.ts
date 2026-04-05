export interface CollapsedPaste {
  id: number
  text: string
  rowCount: number
}

export interface CollapsedPasteResult {
  displayText: string
  pastes: CollapsedPaste[]
}

const COLLAPSE_ROW_THRESHOLD = 24
const COLLAPSE_CHAR_THRESHOLD = 4000
const QUEUE_PREVIEW_CHAR_LIMIT = 120
const COLLAPSED_PASTE_PATTERN = /\[(\d+) rows inserted #(\d+)\]/g

export function countPromptRows(text: string): number {
  if (text.length === 0) {
    return 1
  }

  return text.split("\n").length
}

export function shouldCollapsePromptText(text: string): boolean {
  return text.length >= COLLAPSE_CHAR_THRESHOLD
    || countPromptRows(text) >= COLLAPSE_ROW_THRESHOLD
}

export function formatCollapsedRowsInserted(
  rowCount: number,
  id?: number,
): string {
  if (id === undefined) {
    return `[${rowCount} rows inserted]`
  }

  return `[${rowCount} rows inserted #${id}]`
}

export function collapsePastedText(
  text: string,
  nextId: number,
): CollapsedPasteResult {
  if (!shouldCollapsePromptText(text)) {
    return {
      displayText: text,
      pastes: [],
    }
  }

  const rowCount = countPromptRows(text)
  const paste = {
    id: nextId,
    text,
    rowCount,
  }

  return {
    displayText: formatCollapsedRowsInserted(rowCount, nextId),
    pastes: [paste],
  }
}

export function expandCollapsedPastes(
  text: string,
  pastes: readonly CollapsedPaste[],
): string {
  const pastesById = new Map(
    pastes.map((paste) => [paste.id, paste.text] as const),
  )

  return text.replace(COLLAPSED_PASTE_PATTERN, (match, _rows, idText) => {
    const id = Number.parseInt(idText, 10)
    const pastedText = pastesById.get(id)

    return pastedText ?? match
  })
}

export function filterReferencedCollapsedPastes(
  text: string,
  pastes: readonly CollapsedPaste[],
): CollapsedPaste[] {
  const referencedIds = collectCollapsedPasteIds(text)

  return pastes.filter((paste) => referencedIds.has(paste.id))
}

export function summarizeQueuedPromptText(text: string): string {
  if (shouldCollapsePromptText(text)) {
    return formatCollapsedRowsInserted(countPromptRows(text))
  }

  const singleLine = text.replace(/\s+/g, " ").trim()

  if (singleLine.length <= QUEUE_PREVIEW_CHAR_LIMIT) {
    return singleLine
  }

  return `${singleLine.slice(0, QUEUE_PREVIEW_CHAR_LIMIT - 3)}...`
}

export function collapsedPromptDisplayText(text: string): string | null {
  if (!shouldCollapsePromptText(text)) {
    return null
  }

  return formatCollapsedRowsInserted(countPromptRows(text))
}

function collectCollapsedPasteIds(text: string): Set<number> {
  const ids = new Set<number>()

  for (const match of text.matchAll(COLLAPSED_PASTE_PATTERN)) {
    const id = Number.parseInt(match[2] ?? "", 10)

    if (Number.isNaN(id)) {
      continue
    }

    ids.add(id)
  }

  return ids
}
