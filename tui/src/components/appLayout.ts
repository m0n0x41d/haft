import {
  estimateAttachmentRows,
  type AttachmentItem,
} from "./attachmentLayout.js"

const TOP_SEPARATOR_ROWS = 1
const BOTTOM_SEPARATOR_ROWS = 1
const STATUS_BAR_ROWS = 1

interface BottomLayoutOptions {
  width: number
  queuedMessages: readonly string[]
  attachments: readonly AttachmentItem[]
  attachmentSelection: boolean
  inputRows: number
  showInput: boolean
}

export function computeBottomRows(options: BottomLayoutOptions): number {
  const {
    width,
    queuedMessages,
    attachments,
    attachmentSelection,
    inputRows,
    showInput,
  } = options

  const queuedRows = estimateQueuedMessageRows(queuedMessages, width)
  const attachmentRows = estimateAttachmentRows({
    items: attachments,
    selectionMode: attachmentSelection,
    width,
  })
  const visibleInputRows = showInput
    ? Math.max(1, inputRows)
    : 0

  return TOP_SEPARATOR_ROWS
    + queuedRows
    + attachmentRows
    + visibleInputRows
    + BOTTOM_SEPARATOR_ROWS
    + STATUS_BAR_ROWS
}

export function computeChatHeight(totalRows: number, bottomRows: number): number {
  return Math.max(0, totalRows - bottomRows)
}

export function estimateInputRows(text: string): number {
  if (!text) {
    return 1
  }

  return Math.max(1, text.split("\n").length)
}

export function estimateQueuedMessageRows(
  queuedMessages: readonly string[],
  width: number,
): number {
  const contentWidth = Math.max(1, width - 2)

  return queuedMessages.reduce((sum, message) => {
    const decoratedMessage = ` ❯ ${message} `

    return sum + countWrappedLines(decoratedMessage, contentWidth)
  }, 0)
}

function countWrappedLines(text: string, width: number): number {
  if (!text) {
    return 1
  }

  return text
    .split("\n")
    .reduce((sum, line) => sum + (line.length === 0 ? 1 : Math.ceil(line.length / width)), 0)
}
