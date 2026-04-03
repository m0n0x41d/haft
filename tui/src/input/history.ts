// Pure input history. Oldest-first stack with draft preservation.
// All functions: History -> History (or null when no-op).

const MAX_ENTRIES = 100

export type History = {
  readonly entries: readonly string[]
  readonly position: number // entries.length = at draft (not browsing)
  readonly draft: string
}

export const emptyHistory: History = { entries: [], position: 0, draft: "" }

export const push = (h: History, entry: string): History => {
  if (!entry.trim()) return { ...h, position: h.entries.length, draft: "" }
  const last = h.entries[h.entries.length - 1]
  const entries =
    entry === last
      ? h.entries
      : h.entries.length >= MAX_ENTRIES
        ? [...h.entries.slice(1), entry]
        : [...h.entries, entry]
  return { entries, position: entries.length, draft: "" }
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
  h.position >= h.entries.length
    ? h.draft
    : (h.entries[h.position] ?? h.draft)

export const isNavigating = (h: History): boolean =>
  h.position < h.entries.length
