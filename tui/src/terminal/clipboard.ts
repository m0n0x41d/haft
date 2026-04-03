// Async clipboard image extraction — macOS, Linux.
// All operations are async (child_process.spawn), never block event loop.

import { spawn } from "node:child_process"
import { readFile } from "node:fs/promises"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { trace } from "../debug.js"

export interface ClipboardImage {
  base64: string
  mediaType: string
  tempPath: string
}

// Run a command async, capture stdout, kill after timeout.
// Drains both stdout and stderr to prevent pipe deadlock.
function run(
  cmd: string,
  args: string[],
  opts: { timeoutMs?: number; captureStdout?: boolean } = {},
): Promise<{ code: number | null; stdout: string }> {
  const { timeoutMs = 3000, captureStdout = true } = opts
  return new Promise((resolve) => {
    const proc = spawn(cmd, args, { stdio: ["ignore", "pipe", "pipe"] })
    const chunks: Buffer[] = []
    if (captureStdout) {
      proc.stdout!.on("data", (d: Buffer) => chunks.push(d))
    } else {
      proc.stdout!.resume() // drain without storing
    }
    proc.stderr!.resume() // always drain stderr to prevent deadlock

    let settled = false
    const finish = (code: number | null) => {
      if (settled) return
      settled = true
      resolve({ code, stdout: captureStdout ? Buffer.concat(chunks).toString("utf-8") : "" })
    }

    proc.on("close", (code) => finish(code))
    proc.on("error", () => finish(null))
    const timer = setTimeout(() => { proc.kill("SIGKILL"); finish(null) }, timeoutMs)
    proc.on("close", () => clearTimeout(timer))
  })
}

// Try to extract an image from the system clipboard.
// Returns null if no image found.
export async function getImageFromClipboard(): Promise<ClipboardImage | null> {
  if (process.platform === "darwin") return getImageDarwin()
  if (process.platform === "linux") return getImageLinux()
  return null
}

async function getImageDarwin(): Promise<ClipboardImage | null> {
  trace("clipboard: step1 check types")
  // Step 1: Check clipboard types (no data materialization — just type names)
  const check = await run("osascript", [
    "-e", 'try',
    "-e", '  set theInfo to (clipboard info)',
    "-e", '  repeat with t in theInfo',
    "-e", '    if (item 1 of t) is «class PNGf» then return "png"',
    "-e", '    if (item 1 of t) is «class JPEG» then return "jpeg"',
    "-e", '  end repeat',
    "-e", '  return "none"',
    "-e", 'on error',
    "-e", '  return "none"',
    "-e", 'end try',
  ], { timeoutMs: 3000 })
  const imgType = (check.stdout ?? "").trim()
  trace(`clipboard: step1 done code=${check.code} type=${imgType}`)
  if (check.code !== 0 || imgType === "none" || imgType === "") return null

  trace("clipboard: step2 save to file")
  // Step 2: Save to temp file (osascript writes binary data directly)
  const imgPath = join(tmpdir(), `haft-paste-${Date.now()}.png`)
  const typeClass = imgType === "jpeg" ? "«class JPEG»" : "«class PNGf»"
  const save = await run("osascript", [
    "-e", `set img_data to (the clipboard as ${typeClass})`,
    "-e", `set fp to open for access POSIX file "${imgPath}" with write permission`,
    "-e", "write img_data to fp",
    "-e", "close access fp",
  ], { timeoutMs: 5000, captureStdout: false })

  trace(`clipboard: step2 done code=${save.code}`)
  if (save.code !== 0) return null

  trace("clipboard: step3 read file")
  // Step 3: Read and encode (async — does not block event loop)
  try {
    const buffer = await readFile(imgPath)
    trace(`clipboard: step3 done size=${buffer.length}`)
    if (buffer.length === 0) return null
    return {
      base64: buffer.toString("base64"),
      mediaType: detectMediaType(buffer),
      tempPath: imgPath,
    }
  } catch (e) {
    trace(`clipboard: step3 error ${e}`)
    return null
  }
}

async function getImageLinux(): Promise<ClipboardImage | null> {
  // Check xclip or wl-paste for image types
  const check = await run("sh", ["-c",
    'xclip -selection clipboard -t TARGETS -o 2>/dev/null | grep -qE "image/(png|jpeg)" || wl-paste -l 2>/dev/null | grep -qE "image/(png|jpeg)"'
  ], { timeoutMs: 2000 })
  if (check.code !== 0) return null

  const imgPath = join(tmpdir(), `haft-paste-${Date.now()}.png`)
  const save = await run("sh", ["-c",
    `xclip -selection clipboard -t image/png -o > "${imgPath}" 2>/dev/null || wl-paste --type image/png > "${imgPath}" 2>/dev/null`
  ], { timeoutMs: 3000, captureStdout: false })
  if (save.code !== 0) return null

  try {
    const buffer = await readFile(imgPath)
    if (buffer.length === 0) return null
    return {
      base64: buffer.toString("base64"),
      mediaType: detectMediaType(buffer),
      tempPath: imgPath,
    }
  } catch {
    return null
  }
}

function detectMediaType(buf: Buffer): string {
  if (buf[0] === 0x89 && buf[1] === 0x50) return "image/png"
  if (buf[0] === 0xFF && buf[1] === 0xD8) return "image/jpeg"
  if (buf[0] === 0x47 && buf[1] === 0x49) return "image/gif"
  if (buf[0] === 0x52 && buf[1] === 0x49) return "image/webp"
  return "image/png"
}
