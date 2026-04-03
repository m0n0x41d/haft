// L2: Scroll State Machine — pure, line-based.
// State is anchored to a stable transcript row instead of a raw bottom offset.
// No React, no Ink, no side effects.

import type { TranscriptLayout } from "./measure.js"

export interface ScrollState {
  anchorRowId: string | null
  anchorOffset: number
  pinnedToBottom: boolean
  viewportSize: number
}

export type ScrollCommand =
  | { type: "wheelUp"; amount?: number }
  | { type: "wheelDown"; amount?: number }
  | { type: "pageUp" }
  | { type: "pageDown" }
  | { type: "home" }
  | { type: "end" }
  | { type: "contentChanged" }
  | { type: "resize"; viewportSize: number }

export function initialScroll(viewportSize: number = 20): ScrollState {
  return {
    anchorRowId: null,
    anchorOffset: 0,
    pinnedToBottom: true,
    viewportSize,
  }
}

export function reduceScroll(state: ScrollState, cmd: ScrollCommand, layout: TranscriptLayout): ScrollState {
  switch (cmd.type) {
    case "wheelUp":
      return shiftViewport(state, layout, -(cmd.amount ?? 3))

    case "wheelDown":
      return shiftViewport(state, layout, cmd.amount ?? 3)

    case "pageUp":
      return shiftViewport(state, layout, -state.viewportSize)

    case "pageDown":
      return shiftViewport(state, layout, state.viewportSize)

    case "home":
      return createViewportState(0, state.viewportSize, layout)

    case "end":
      return createViewportState(getMaxViewportTop(layout.totalHeight, state.viewportSize), state.viewportSize, layout, true)

    case "contentChanged":
      return normalizeViewport(state, layout)

    case "resize":
      return normalizeViewport({ ...state, viewportSize: cmd.viewportSize }, layout)
  }
}

export function isAtBottom(state: ScrollState): boolean {
  return state.pinnedToBottom
}

export function resolveViewportTop(state: ScrollState, layout: TranscriptLayout): number {
  if (layout.rows.length === 0) {
    return 0
  }

  if (state.pinnedToBottom) {
    return getMaxViewportTop(layout.totalHeight, state.viewportSize)
  }

  if (!state.anchorRowId) {
    return getMaxViewportTop(layout.totalHeight, state.viewportSize)
  }

  const anchorIndex = layout.rowIndexById.get(state.anchorRowId)
  if (anchorIndex === undefined) {
    return getMaxViewportTop(layout.totalHeight, state.viewportSize)
  }

  const rowTop = layout.offsets[anchorIndex] ?? 0
  const rowHeight = layout.heights[anchorIndex] ?? 0
  const maxAnchorOffset = Math.max(0, rowHeight - 1)
  const clampedAnchorOffset = clampLine(state.anchorOffset, 0, maxAnchorOffset)
  const viewportTop = rowTop + clampedAnchorOffset
  const maxViewportTop = getMaxViewportTop(layout.totalHeight, state.viewportSize)

  return clampLine(viewportTop, 0, maxViewportTop)
}

export function getOffsetFromBottom(state: ScrollState, layout: TranscriptLayout): number {
  const maxViewportTop = getMaxViewportTop(layout.totalHeight, state.viewportSize)
  const viewportTop = resolveViewportTop(state, layout)

  return maxViewportTop - viewportTop
}

function shiftViewport(state: ScrollState, layout: TranscriptLayout, delta: number): ScrollState {
  const currentTop = resolveViewportTop(state, layout)
  const nextTop = currentTop + delta

  return createViewportState(nextTop, state.viewportSize, layout)
}

function normalizeViewport(state: ScrollState, layout: TranscriptLayout): ScrollState {
  if (state.pinnedToBottom) {
    return createViewportState(getMaxViewportTop(layout.totalHeight, state.viewportSize), state.viewportSize, layout, true)
  }

  const viewportTop = resolveViewportTop(state, layout)

  return createViewportState(viewportTop, state.viewportSize, layout)
}

function createViewportState(
  viewportTop: number,
  viewportSize: number,
  layout: TranscriptLayout,
  pinnedToBottom: boolean = false,
): ScrollState {
  const clampedViewportTop = clampLine(viewportTop, 0, getMaxViewportTop(layout.totalHeight, viewportSize))
  const anchor = findAnchor(layout, clampedViewportTop)

  return {
    anchorRowId: anchor.rowId,
    anchorOffset: anchor.offset,
    pinnedToBottom: pinnedToBottom || clampedViewportTop === getMaxViewportTop(layout.totalHeight, viewportSize),
    viewportSize,
  }
}

function findAnchor(layout: TranscriptLayout, viewportTop: number): { rowId: string | null; offset: number } {
  if (layout.rows.length === 0) {
    return { rowId: null, offset: 0 }
  }

  const anchorIndex = findRowIndex(layout.offsets, viewportTop)
  const anchorTop = layout.offsets[anchorIndex] ?? 0
  const anchorRow = layout.rows[anchorIndex]

  return {
    rowId: anchorRow?.id ?? null,
    offset: viewportTop - anchorTop,
  }
}

function findRowIndex(offsets: readonly number[], line: number): number {
  const rowCount = Math.max(0, offsets.length - 1)

  if (rowCount === 0) {
    return 0
  }

  let low = 0
  let high = rowCount - 1

  while (low <= high) {
    const mid = Math.floor((low + high) / 2)
    const rowTop = offsets[mid]
    const rowBottom = offsets[mid + 1]

    if (line < rowTop) {
      high = mid - 1
      continue
    }

    if (line >= rowBottom) {
      low = mid + 1
      continue
    }

    return mid
  }

  return Math.max(0, Math.min(low, rowCount - 1))
}

function getMaxViewportTop(totalHeight: number, viewportSize: number): number {
  return Math.max(0, totalHeight - viewportSize)
}

function clampLine(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(value, max))
}
