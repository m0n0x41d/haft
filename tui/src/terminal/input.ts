// L1: Input Parser — pure.
// Raw bytes → typed InputEvent[].
// No side effects, no Ink dependency.

export type InputEvent =
  | { type: "key"; raw: string }
  | { type: "wheelUp" }
  | { type: "wheelDown" }
  | { type: "mouseClick"; col: number; row: number; button: number }
  | { type: "mouseRelease"; col: number; row: number }
  | { type: "paste"; text: string }

// SGR mouse sequence: \x1b[<button;col;rowM (press) or \x1b[<button;col;rowm (release)
const SGR_MOUSE_RE = /\x1b\[<(\d+);(\d+);(\d+)([Mm])/g

// Parse a raw input chunk into typed events.
// Mouse sequences are extracted; remaining bytes become key events.
export function parseInput(raw: string): InputEvent[] {
  const events: InputEvent[] = []
  let cleaned = raw

  // Extract all SGR mouse sequences
  SGR_MOUSE_RE.lastIndex = 0
  let match: RegExpExecArray | null
  while ((match = SGR_MOUSE_RE.exec(raw)) !== null) {
    const button = parseInt(match[1], 10)
    const col = parseInt(match[2], 10)
    const row = parseInt(match[3], 10)
    const isPress = match[4] === "M"

    if (!isPress) {
      events.push({ type: "mouseRelease", col, row })
    } else if ((button & 0x43) === 0x40) {
      events.push({ type: "wheelUp" })
    } else if ((button & 0x43) === 0x41) {
      events.push({ type: "wheelDown" })
    } else {
      events.push({ type: "mouseClick", col, row, button: button & 0x03 })
    }
  }

  // Strip mouse sequences from the raw input
  cleaned = raw.replace(SGR_MOUSE_RE, "")

  // Also catch orphaned SGR tails (when ESC was consumed by readline)
  // These look like: [<64;30;15M or <64;30;15M without the ESC prefix
  const ORPHAN_RE = /\[?<\d+;\d+;\d+[Mm]/g
  cleaned = cleaned.replace(ORPHAN_RE, "")

  // Remaining non-empty content is keyboard input
  if (cleaned.length > 0) {
    events.push({ type: "key", raw: cleaned })
  }

  return events
}

// Check if a raw string contains any SGR mouse data
export function containsMouse(raw: string): boolean {
  return raw.includes("\x1b[<") || /\[?<\d+;\d+;\d+[Mm]/.test(raw)
}
