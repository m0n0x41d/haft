type GraphemeSegment = {
  text: string
  start: number
  end: number
  width: 1 | 2
}

const graphemeSegmenter = new Intl.Segmenter(undefined, {
  granularity: "grapheme",
})

const bunStringWidth = (
  globalThis as {
    Bun?: { stringWidth?: (value: string) => number }
  }
).Bun?.stringWidth

export function segmentGraphemes(text: string): GraphemeSegment[] {
  return [...graphemeSegmenter.segment(text)].map(({ segment, index }) => ({
    text: segment,
    start: index,
    end: index + segment.length,
    width: measureGraphemeWidth(segment),
  }))
}

export function normalizeGraphemeBoundaryLeft(
  text: string,
  offset: number,
): number {
  const clampedOffset = clampOffset(text, offset)

  for (const grapheme of segmentGraphemes(text)) {
    if (clampedOffset === grapheme.end) {
      return grapheme.end
    }

    if (clampedOffset < grapheme.end) {
      return grapheme.start
    }
  }

  return clampedOffset
}

export function findPreviousGraphemeBoundary(
  text: string,
  offset: number,
): number {
  const clampedOffset = clampOffset(text, offset)
  let previousBoundary = 0

  for (const grapheme of segmentGraphemes(text)) {
    if (clampedOffset <= grapheme.start) {
      return previousBoundary
    }

    if (clampedOffset <= grapheme.end) {
      return grapheme.start
    }

    previousBoundary = grapheme.end
  }

  return previousBoundary
}

export function findNextGraphemeBoundary(
  text: string,
  offset: number,
): number {
  const clampedOffset = clampOffset(text, offset)

  for (const grapheme of segmentGraphemes(text)) {
    if (clampedOffset < grapheme.end) {
      return grapheme.end
    }
  }

  return clampedOffset
}

function clampOffset(text: string, offset: number): number {
  return Math.max(0, Math.min(offset, text.length))
}

function measureGraphemeWidth(grapheme: string): 1 | 2 {
  const width = bunStringWidth?.(grapheme)

  if (typeof width === "number") {
    return width > 1 ? 2 : 1
  }

  const codePoint = grapheme.codePointAt(0) ?? 0

  return isWideCodePoint(codePoint) ? 2 : 1
}

function isWideCodePoint(codePoint: number): boolean {
  return (
    (codePoint >= 0x1100 && codePoint <= 0x115F)
    || (codePoint >= 0x2329 && codePoint <= 0x232A)
    || (codePoint >= 0x2E80 && codePoint <= 0xA4CF)
    || (codePoint >= 0xAC00 && codePoint <= 0xD7A3)
    || (codePoint >= 0xF900 && codePoint <= 0xFAFF)
    || (codePoint >= 0xFE10 && codePoint <= 0xFE19)
    || (codePoint >= 0xFE30 && codePoint <= 0xFE6F)
    || (codePoint >= 0xFF00 && codePoint <= 0xFF60)
    || (codePoint >= 0xFFE0 && codePoint <= 0xFFE6)
    || (codePoint >= 0x1F300 && codePoint <= 0x1FAFF)
  )
}
