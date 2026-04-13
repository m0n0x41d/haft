export interface AttachmentItem {
  id: number
  name: string
  path: string
  isImage: boolean
  size?: number
}

export interface AttachmentDisplayItem {
  id: number
  label: string
  selected: boolean
}

export type AttachmentRow =
  | { type: "items"; items: AttachmentDisplayItem[] }
  | { type: "hint"; text: string }

interface BuildAttachmentRowsOptions {
  items: readonly AttachmentItem[]
  selectionMode: boolean
  selectedIndex?: number
  width: number
}

const HORIZONTAL_PADDING = 2
const DEFAULT_HINT = "(↑ to select)"
const SELECTION_HINT = "→ to next · Delete to remove · Esc to cancel"
const ELLIPSIS = "…"

export function buildAttachmentRows(options: BuildAttachmentRowsOptions): AttachmentRow[] {
  const { items, selectionMode, selectedIndex = 0, width } = options

  if (items.length === 0) {
    return []
  }

  const contentWidth = Math.max(1, width - HORIZONTAL_PADDING)
  const itemRows = buildItemRows(items, selectionMode, selectedIndex, contentWidth)
  const hintRows = wrapText(getHintText(selectionMode), contentWidth)
    .map((text) => ({ type: "hint", text } satisfies AttachmentRow))

  return [...itemRows, ...hintRows]
}

export function estimateAttachmentRows(options: Omit<BuildAttachmentRowsOptions, "selectedIndex">): number {
  const rows = buildAttachmentRows(options)

  return rows.length
}

export function moveAttachmentCursor(
  cursor: number,
  direction: "left" | "right",
  itemCount: number,
): number {
  if (itemCount <= 0) {
    return 0
  }

  if (direction === "left") {
    return Math.max(0, cursor - 1)
  }

  return Math.min(itemCount - 1, cursor + 1)
}

export function clampAttachmentCursor(cursor: number, itemCount: number): number {
  if (itemCount <= 0) {
    return 0
  }

  return Math.min(Math.max(0, cursor), itemCount - 1)
}

function buildItemRows(
  items: readonly AttachmentItem[],
  selectionMode: boolean,
  selectedIndex: number,
  contentWidth: number,
): AttachmentRow[] {
  const rows: AttachmentRow[] = []
  let currentItems: AttachmentDisplayItem[] = []
  let currentWidth = 0

  for (let index = 0; index < items.length; index += 1) {
    const item = items[index]
    const rawLabel = formatAttachmentLabel(item)
    const boundedLabel = truncateAttachmentLabel(rawLabel, contentWidth)
    const displayItem: AttachmentDisplayItem = {
      id: item.id,
      label: boundedLabel,
      selected: selectionMode && index === selectedIndex,
    }
    const itemWidth = displayItem.label.length
    const gapWidth = currentItems.length === 0 ? 0 : 1

    if (currentItems.length > 0 && currentWidth + gapWidth + itemWidth > contentWidth) {
      rows.push({ type: "items", items: currentItems })
      currentItems = []
      currentWidth = 0
    }

    currentItems = [...currentItems, displayItem]
    currentWidth += (currentItems.length === 1 ? 0 : 1) + itemWidth
  }

  if (currentItems.length > 0) {
    rows.push({ type: "items", items: currentItems })
  }

  return rows
}

function formatAttachmentLabel(item: AttachmentItem): string {
  if (item.isImage) {
    return `[Image #${item.id}]`
  }

  return `[${item.name}]`
}

function getHintText(selectionMode: boolean): string {
  return selectionMode ? SELECTION_HINT : DEFAULT_HINT
}

function truncateAttachmentLabel(label: string, width: number): string {
  if (label.length <= width) {
    return label
  }

  if (width <= 1) {
    return ELLIPSIS
  }

  if (label.startsWith("[") && label.endsWith("]")) {
    return truncateBracketedLabel(label, width)
  }

  return truncatePlainLabel(label, width)
}

function truncateBracketedLabel(label: string, width: number): string {
  if (width <= 2) {
    return ELLIPSIS
  }

  const inner = label.slice(1, -1)
  const visibleInnerWidth = Math.max(0, width - 3)
  const visibleInner = inner.slice(0, visibleInnerWidth)

  return `[${visibleInner}${ELLIPSIS}]`
}

function truncatePlainLabel(label: string, width: number): string {
  const visibleWidth = Math.max(0, width - 1)
  const visibleText = label.slice(0, visibleWidth)

  return `${visibleText}${ELLIPSIS}`
}

function wrapText(text: string, width: number): string[] {
  if (!text) {
    return [""]
  }

  return text
    .split("\n")
    .flatMap((line) => wrapLine(line, width))
}

function wrapLine(line: string, width: number): string[] {
  if (!line) {
    return [""]
  }

  const chunks: string[] = []

  for (let start = 0; start < line.length; start += width) {
    chunks.push(line.slice(start, start + width))
  }

  return chunks
}
