// L0: Terminal Adapter — effect boundary.
// The ONLY module that touches raw TTY streams.
// Everything above works with typed values.

import * as fs from "node:fs"
import * as tty from "node:tty"

export interface TerminalStreams {
  input: tty.ReadStream    // raw keyboard + mouse bytes
  output: NodeJS.WriteStream  // rendering target (stderr)
}

// Open /dev/tty for input, use stderr for output
export function openTerminal(): TerminalStreams {
  const fd = fs.openSync("/dev/tty", "r")
  return {
    input: new tty.ReadStream(fd),
    output: process.stderr,
  }
}

// --- Escape sequence writers (fire-and-forget effects) ---

const ESC = "\x1b["

// Mouse tracking: SGR extended mode (1006) + button events (1000) + any-event (1003)
export function enableMouseTracking(out: NodeJS.WriteStream): void {
  if (!out.isTTY) return
  out.write(`${ESC}?1000h${ESC}?1003h${ESC}?1006h`)
}

export function disableMouseTracking(out: NodeJS.WriteStream): void {
  if (!out.isTTY) return
  out.write(`${ESC}?1006l${ESC}?1003l${ESC}?1000l`)
}

// Bracketed paste mode — paste content wrapped in ESC[200~ ... ESC[201~
// Without this, pasted text arrives as individual keystrokes (breaks multi-char input).
export function enableBracketedPaste(out: NodeJS.WriteStream): void {
  if (!out.isTTY) return
  out.write(`${ESC}?2004h`)
}

export function disableBracketedPaste(out: NodeJS.WriteStream): void {
  if (!out.isTTY) return
  out.write(`${ESC}?2004l`)
}

// Focus events — terminal reports focus in/out via ESC[I / ESC[O
export function enableFocusEvents(out: NodeJS.WriteStream): void {
  if (!out.isTTY) return
  out.write(`${ESC}?1004h`)
}

export function disableFocusEvents(out: NodeJS.WriteStream): void {
  if (!out.isTTY) return
  out.write(`${ESC}?1004l`)
}

export function showCursor(out: NodeJS.WriteStream): void {
  if (!out.isTTY) return
  out.write(`${ESC}?25h`)
}

export function hideCursor(out: NodeJS.WriteStream): void {
  if (!out.isTTY) return
  out.write(`${ESC}?25l`)
}

// Re-assert all terminal modes — call after reconnect / tmux reattach / wake
export function reassertTerminalModes(out: NodeJS.WriteStream): void {
  enableMouseTracking(out)
  enableBracketedPaste(out)
  enableFocusEvents(out)
}

// Signal-safe cleanup — call on exit, SIGINT, SIGTERM, crash
export function cleanupTerminal(out: NodeJS.WriteStream): void {
  disableBracketedPaste(out)
  disableFocusEvents(out)
  disableMouseTracking(out)
  showCursor(out)
}
