type GraphemeSegment = {
  text: string
  start: number
  end: number
  width: 1 | 2
}

type StringWidthFn = (value: string) => number

const graphemeSegmenter = new Intl.Segmenter(undefined, {
  granularity: "grapheme",
})

const emojiPresentationRegex = /\p{Emoji_Presentation}/u
const emojiModifierRegex = /\p{Emoji_Modifier}/u
const emojiRegex = /\p{Emoji}/u
const extendedPictographicRegex = /\p{Extended_Pictographic}/u
const markRegex = /\p{Mark}/u
const regionalIndicatorFlagRegex = /^(?:\p{Regional_Indicator}){2}$/u
const keycapSequenceRegex = /^[#*0-9]\uFE0F?\u20E3$/u

const emojiVariationSelector = "\uFE0F"
const zeroWidthJoiner = "\u200D"

export function segmentGraphemes(text: string): GraphemeSegment[] {
  return [...graphemeSegmenter.segment(text)].map(({ segment, index }) => ({
    text: segment,
    start: index,
    end: index + segment.length,
    width: measureGraphemeWidth(segment),
  }))
}

export function measureGraphemeWidth(
  grapheme: string,
  runtimeStringWidth: StringWidthFn | null = getRuntimeStringWidth(),
): 1 | 2 {
  const fallbackWidth = measureFallbackGraphemeWidth(grapheme)
  const runtimeWidth = measureRuntimeGraphemeWidth(grapheme, runtimeStringWidth)
  const maxWidth = Math.max(fallbackWidth, runtimeWidth)

  return maxWidth > 1 ? 2 : 1
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

function getRuntimeStringWidth(): StringWidthFn | null {
  const runtime = globalThis as {
    Bun?: { stringWidth?: StringWidthFn }
  }

  return runtime.Bun?.stringWidth ?? null
}

function measureRuntimeGraphemeWidth(
  grapheme: string,
  runtimeStringWidth: StringWidthFn | null,
): number {
  const width = runtimeStringWidth?.(grapheme)

  if (typeof width !== "number" || !Number.isFinite(width)) {
    return 0
  }

  return Math.max(0, Math.min(2, Math.ceil(width)))
}

function measureFallbackGraphemeWidth(grapheme: string): 1 | 2 {
  const isEmojiCluster = regionalIndicatorFlagRegex.test(grapheme)
    || keycapSequenceRegex.test(grapheme)
    || emojiPresentationRegex.test(grapheme)
    || (grapheme.includes(emojiVariationSelector) && emojiRegex.test(grapheme))
    || (grapheme.includes(zeroWidthJoiner) && extendedPictographicRegex.test(grapheme))
    || (emojiModifierRegex.test(grapheme) && extendedPictographicRegex.test(grapheme))

  if (isEmojiCluster) {
    return 2
  }

  const codePoints = [...grapheme]
    .map((char) => char.codePointAt(0) ?? 0)
    .filter((codePoint) => isVisibleWidthCodePoint(codePoint))
  const hasWideCodePoint = codePoints.some((codePoint) => isWideCodePoint(codePoint))

  return hasWideCodePoint ? 2 : 1
}

function isVisibleWidthCodePoint(codePoint: number): boolean {
  if (codePoint === 0x200D) {
    return false
  }

  if (codePoint >= 0xFE00 && codePoint <= 0xFE0F) {
    return false
  }

  if (codePoint >= 0xE0100 && codePoint <= 0xE01EF) {
    return false
  }

  if (codePoint >= 0xE0020 && codePoint <= 0xE007F) {
    return false
  }

  const char = String.fromCodePoint(codePoint)

  return !markRegex.test(char)
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
