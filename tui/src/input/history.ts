// Pure input history. Oldest-first stack with draft preservation.
// All functions: History -> History (or null when no-op).

const MAX_ENTRIES = 100

export type HistoryEntry = {
  readonly id: number
  readonly text: string
}

export type History = {
  readonly entries: readonly HistoryEntry[]
  readonly position: number // entries.length = at draft (not browsing)
  readonly draft: string
  readonly nextId: number
}

export type PushHistoryResult = {
  readonly history: History
  readonly stored: HistoryEntry | null
  readonly evicted: HistoryEntry | null
}

export const emptyHistory: History = {
  entries: [],
  position: 0,
  draft: "",
  nextId: 1,
}

export const push = (h: History, entry: string): PushHistoryResult => {
  if (!entry.trim()) {
    return {
      history: { ...h, position: h.entries.length, draft: "" },
      stored: null,
      evicted: null,
    }
  }

  const last = h.entries[h.entries.length - 1]

  if (entry === last?.text) {
    return {
      history: { ...h, position: h.entries.length, draft: "" },
      stored: last,
      evicted: null,
    }
  }

  const stored = {
    id: h.nextId,
    text: entry,
  }
  const hasOverflow = h.entries.length >= MAX_ENTRIES
  const evicted = hasOverflow
    ? h.entries[0] ?? null
    : null
  const retained = hasOverflow
    ? h.entries.slice(1)
    : h.entries
  const entries = [...retained, stored]

  return {
    history: {
      entries,
      position: entries.length,
      draft: "",
      nextId: h.nextId + 1,
    },
    stored,
    evicted,
  }
}

export const navigateUp = (
  h: History,
  currentText: string,
): History | null => {
  if (h.entries.length === 0) return null
  const atDraft = h.position >= h.entries.length
  const draft = atDraft ? currentText : h.draft
  const newPos = atDraft ? h.entries.length - 1 : h.position - 1
  if (newPos < 0) return null
  return { ...h, position: newPos, draft }
}

export const navigateDown = (h: History): History | null => {
  if (h.position >= h.entries.length) return null
  return { ...h, position: h.position + 1 }
}

export const currentText = (h: History): string =>
  currentEntry(h)?.text ?? h.draft

export const currentEntry = (h: History): HistoryEntry | null =>
  h.position >= h.entries.length
    ? null
    : (h.entries[h.position] ?? null)

export const isNavigating = (h: History): boolean =>
  h.position < h.entries.length
