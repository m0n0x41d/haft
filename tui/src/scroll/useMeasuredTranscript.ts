import { measureElement, type DOMElement } from "ink"
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react"
import type { TranscriptEntry } from "../state/transcript.js"
import { resolveEntryHeights, scaleMeasuredHeights } from "./measure.js"

export interface UseMeasuredTranscriptResult {
  entryHeights: readonly number[]
  measureRef: (entryId: string) => (node: DOMElement | null) => void
}

export function useMeasuredTranscript(
  entries: readonly TranscriptEntry[],
  width: number,
  toolHistoryExpanded: boolean,
): UseMeasuredTranscriptResult {
  const measuredHeightsRef = useRef(new Map<string, number>())
  const entryNodesRef = useRef(new Map<string, DOMElement>())
  const refCacheRef = useRef(new Map<string, (node: DOMElement | null) => void>())
  const previousWidthRef = useRef(width)
  const previousToolHistoryExpandedRef = useRef(toolHistoryExpanded)
  const [version, setVersion] = useState(0)

  if (previousWidthRef.current !== width) {
    scaleMeasuredHeights(
      measuredHeightsRef.current,
      previousWidthRef.current,
      width,
    )
    previousWidthRef.current = width
  }

  useEffect(() => {
    const liveEntryIds = new Set(entries.map((entry) => entry.id))
    let cacheChanged = false

    for (const entryId of measuredHeightsRef.current.keys()) {
      if (liveEntryIds.has(entryId)) {
        continue
      }

      measuredHeightsRef.current.delete(entryId)
      cacheChanged = true
    }

    for (const entryId of entryNodesRef.current.keys()) {
      if (liveEntryIds.has(entryId)) {
        continue
      }

      entryNodesRef.current.delete(entryId)
    }

    for (const entryId of refCacheRef.current.keys()) {
      if (liveEntryIds.has(entryId)) {
        continue
      }

      refCacheRef.current.delete(entryId)
    }

    if (cacheChanged) {
      setVersion((currentVersion) => currentVersion + 1)
    }
  }, [entries])

  useEffect(() => {
    if (previousToolHistoryExpandedRef.current === toolHistoryExpanded) {
      return
    }

    previousToolHistoryExpandedRef.current = toolHistoryExpanded
    let cacheChanged = false

    for (const entry of entries) {
      if (entry.type !== "assistantToolBatch") {
        continue
      }

      if (!measuredHeightsRef.current.delete(entry.id)) {
        continue
      }

      cacheChanged = true
    }

    if (cacheChanged) {
      setVersion((currentVersion) => currentVersion + 1)
    }
  }, [entries, toolHistoryExpanded])

  useLayoutEffect(() => {
    let cacheChanged = false

    for (const [entryId, entryNode] of entryNodesRef.current) {
      const nextMeasurement = measureElement(entryNode)

      if (nextMeasurement.width <= 0) {
        continue
      }

      const nextHeight = Math.max(0, Math.ceil(nextMeasurement.height))
      const previousHeight = measuredHeightsRef.current.get(entryId)

      if (previousHeight === nextHeight) {
        continue
      }

      measuredHeightsRef.current.set(entryId, nextHeight)
      cacheChanged = true
    }

    if (cacheChanged) {
      setVersion((currentVersion) => currentVersion + 1)
    }
  })

  const entryHeights = useMemo(
    () => resolveEntryHeights(
      entries,
      width,
      measuredHeightsRef.current,
      { toolHistoryExpanded },
    ),
    [entries, width, version, toolHistoryExpanded],
  )

  const measureRef = useCallback((entryId: string) => {
    let entryRef = refCacheRef.current.get(entryId)

    if (entryRef) {
      return entryRef
    }

    entryRef = (entryNode: DOMElement | null) => {
      if (entryNode) {
        entryNodesRef.current.set(entryId, entryNode)
        return
      }

      const previousNode = entryNodesRef.current.get(entryId)

      if (previousNode) {
        const nextMeasurement = measureElement(previousNode)

        if (nextMeasurement.width > 0) {
          measuredHeightsRef.current.set(entryId, Math.max(0, Math.ceil(nextMeasurement.height)))
        }
      }

      entryNodesRef.current.delete(entryId)
    }

    refCacheRef.current.set(entryId, entryRef)

    return entryRef
  }, [])

  return {
    entryHeights,
    measureRef,
  }
}
