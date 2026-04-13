// L0: Clipboard — effect boundary.
// Copies text to system clipboard via native tools and/or OSC 52.

import { spawn } from "node:child_process"

const ESC = "\x1b"
const BEL = "\x07"
const ST = ESC + "\\"

// Fire-and-forget: pipe text to a child process stdin.
function pipeToProcess(cmd: string, args: string[], text: string): void {
  try {
    const proc = spawn(cmd, args, { stdio: ["pipe", "ignore", "ignore"] })
    proc.stdin.write(text)
    proc.stdin.end()
    proc.on("error", () => {})
  } catch {
    // Command not found or spawn failed — silently ignore.
  }
}

// OSC 52 clipboard-set sequence.
function osc52Sequence(text: string): string {
  const b64 = Buffer.from(text, "utf-8").toString("base64")
  const terminator = process.env["TERM_PROGRAM"] === "kitty" ? ST : BEL
  return `${ESC}]52;c;${b64}${terminator}`
}

// Wrap escape sequence for tmux DCS passthrough.
function tmuxPassthrough(sequence: string): string {
  return `${ESC}Ptmux;${sequence.replaceAll(ESC, ESC + ESC)}${ST}`
}

// Copy text to clipboard.
// Three-path strategy (matches Claude Code):
//   1. Native tool (pbcopy/xclip/wl-copy) — fire first, wins race with focus-switch
//   2. tmux buffer + DCS-wrapped OSC 52
//   3. Raw OSC 52 fallback
export function copyToClipboard(text: string, output: NodeJS.WriteStream): void {
  if (!text) return

  // 1. Native clipboard (skip over SSH — would write to remote machine)
  if (!process.env["SSH_CONNECTION"]) {
    switch (process.platform) {
      case "darwin":
        pipeToProcess("pbcopy", [], text)
        break
      case "linux":
        if (process.env["WAYLAND_DISPLAY"]) {
          pipeToProcess("wl-copy", [], text)
        } else {
          pipeToProcess("xclip", ["-selection", "clipboard"], text)
        }
        break
      case "win32":
        pipeToProcess("clip", [], text)
        break
    }
  }

  // 2. tmux: load buffer + DCS-wrapped OSC 52 for outer terminal
  if (process.env["TMUX"]) {
    const isITerm = process.env["LC_TERMINAL"] === "iTerm2"
    pipeToProcess("tmux", isITerm ? ["load-buffer", "-"] : ["load-buffer", "-w", "-"], text)
    const b64 = Buffer.from(text, "utf-8").toString("base64")
    output.write(tmuxPassthrough(`${ESC}]52;c;${b64}${BEL}`))
    return
  }

  // 3. Raw OSC 52
  output.write(osc52Sequence(text))
}
