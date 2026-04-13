// L1: Pure ANSI SGR parser.
// Parses ANSI-escaped strings (from cli-highlight) into typed spans.
// No React, no side effects — pure data transformation.

export type SpanStyle = {
  readonly bold?: true
  readonly dim?: true
  readonly italic?: true
  readonly underline?: true
  readonly strikethrough?: true
  readonly inverse?: true
  readonly fg?: string   // Ink color: "red", "ansi256(123)", "rgb(r,g,b)"
  readonly bg?: string   // Ink backgroundColor
}

export type Span = {
  readonly text: string
  readonly style: SpanStyle
}

// --- SGR color tables ---

const FG_BASIC: readonly string[] = [
  "black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
]

const FG_BRIGHT: readonly string[] = [
  "blackBright", "redBright", "greenBright", "yellowBright",
  "blueBright", "magentaBright", "cyanBright", "whiteBright",
]

const BG_BASIC: readonly string[] = [
  "black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
]

const BG_BRIGHT: readonly string[] = [
  "blackBright", "redBright", "greenBright", "yellowBright",
  "blueBright", "magentaBright", "cyanBright", "whiteBright",
]

// Mutable accumulator used only inside parseAnsi — never escapes.
type StyleAcc = {
  bold?: true
  dim?: true
  italic?: true
  underline?: true
  strikethrough?: true
  inverse?: true
  fg?: string
  bg?: string
}

function emptyStyle(): StyleAcc {
  return {}
}

function freezeStyle(acc: StyleAcc): SpanStyle {
  const s: SpanStyle = {}
  if (acc.bold) (s as any).bold = true
  if (acc.dim) (s as any).dim = true
  if (acc.italic) (s as any).italic = true
  if (acc.underline) (s as any).underline = true
  if (acc.strikethrough) (s as any).strikethrough = true
  if (acc.inverse) (s as any).inverse = true
  if (acc.fg) (s as any).fg = acc.fg
  if (acc.bg) (s as any).bg = acc.bg
  return s
}

function stylesEqual(a: SpanStyle, b: SpanStyle): boolean {
  return a.bold === b.bold
    && a.dim === b.dim
    && a.italic === b.italic
    && a.underline === b.underline
    && a.strikethrough === b.strikethrough
    && a.inverse === b.inverse
    && a.fg === b.fg
    && a.bg === b.bg
}

// Parse 256-color or RGB sequences from params starting at `offset`.
// Returns [color-string, new-offset].
function parseExtendedColor(params: readonly number[], offset: number): [string | undefined, number] {
  const mode = params[offset]
  if (mode === 5 && offset + 1 < params.length) {
    return [`ansi256(${params[offset + 1]})`, offset + 2]
  }
  if (mode === 2 && offset + 3 < params.length) {
    return [`rgb(${params[offset + 1]},${params[offset + 2]},${params[offset + 3]})`, offset + 4]
  }
  return [undefined, offset + 1]
}

function applySgr(params: readonly number[], style: StyleAcc): void {
  let i = 0
  while (i < params.length) {
    const p = params[i]!
    switch (true) {
      case p === 0:
        Object.keys(style).forEach((k) => delete (style as any)[k])
        break
      case p === 1:  style.bold = true; break
      case p === 2:  style.dim = true; break
      case p === 3:  style.italic = true; break
      case p === 4:  style.underline = true; break
      case p === 7:  style.inverse = true; break
      case p === 9:  style.strikethrough = true; break
      case p === 22: delete style.bold; delete style.dim; break
      case p === 23: delete style.italic; break
      case p === 24: delete style.underline; break
      case p === 27: delete style.inverse; break
      case p === 29: delete style.strikethrough; break
      case p >= 30 && p <= 37:  style.fg = FG_BASIC[p - 30]; break
      case p === 38: {
        const [color, next] = parseExtendedColor(params, i + 1)
        if (color) style.fg = color
        i = next
        continue
      }
      case p === 39: delete style.fg; break
      case p >= 40 && p <= 47:  style.bg = BG_BASIC[p - 40]; break
      case p === 48: {
        const [color, next] = parseExtendedColor(params, i + 1)
        if (color) style.bg = color
        i = next
        continue
      }
      case p === 49: delete style.bg; break
      case p >= 90 && p <= 97:  style.fg = FG_BRIGHT[p - 90]; break
      case p >= 100 && p <= 107: style.bg = BG_BRIGHT[p - 100]; break
    }
    i++
  }
}

// ESC[ ... m  pattern — captures the parameter string between [ and m.
const SGR_RE = /\x1b\[([0-9;]*)m/

/**
 * Parse an ANSI-escaped string into an array of styled spans.
 * Adjacent spans with identical styles are merged.
 */
export function parseAnsi(input: string): readonly Span[] {
  if (!input) return []

  const spans: Span[] = []
  const style = emptyStyle()
  let rest = input

  while (rest.length > 0) {
    const match = SGR_RE.exec(rest)

    if (!match) {
      pushSpan(spans, rest, freezeStyle(style))
      break
    }

    if (match.index > 0) {
      pushSpan(spans, rest.slice(0, match.index), freezeStyle(style))
    }

    const paramStr = match[1]!
    const params = paramStr === ""
      ? [0]
      : paramStr.split(";").map(Number)
    applySgr(params, style)

    rest = rest.slice(match.index + match[0].length)
  }

  return spans
}

function pushSpan(spans: Span[], text: string, style: SpanStyle): void {
  if (!text) return
  const last = spans[spans.length - 1]
  if (last && stylesEqual(last.style, style)) {
    ;(spans[spans.length - 1] as any) = { text: last.text + text, style }
  } else {
    spans.push({ text, style })
  }
}
