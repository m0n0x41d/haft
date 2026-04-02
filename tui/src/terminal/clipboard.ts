// Async clipboard image extraction — macOS, Linux, Windows.
// Platform-specific clipboard image extraction.
// All operations are async (child_process.spawn), never block event loop.

import { spawn } from "node:child_process"
import { readFileSync, unlinkSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"

export interface ClipboardImage {
  base64: string
  mediaType: string
  tempPath: string
}

// Run a command async, capture stdout, kill after timeout.
function run(cmd: string, args: string[], timeoutMs = 3000): Promise<{ code: number | null; stdout: string }> {
  return new Promise((resolve) => {
    const proc = spawn(cmd, args, { stdio: ["ignore", "pipe", "pipe"] })
    const chunks: Buffer[] = []
    proc.stdout!.on("data", (d: Buffer) => chunks.push(d))

    let settled = false
    const finish = (code: number | null) => {
      if (settled) return
      settled = true
      resolve({ code, stdout: Buffer.concat(chunks).toString("utf-8") })
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
  // Step 1: Check if clipboard has image data
  const check = await run("osascript", ["-e", "the clipboard as «class PNGf»"], 3000)
  if (check.code !== 0) return null

  // Step 2: Save to temp file
  const imgPath = join(tmpdir(), `haft-paste-${Date.now()}.png`)
  const save = await run("osascript", [
    "-e", "set png_data to (the clipboard as «class PNGf»)",
    "-e", `set fp to open for access POSIX file "${imgPath}" with write permission`,
    "-e", "write png_data to fp",
    "-e", "close access fp",
  ], 5000)

  if (save.code !== 0) return null

  // Step 3: Read and encode
  try {
    const buffer = readFileSync(imgPath)
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

async function getImageLinux(): Promise<ClipboardImage | null> {
  // Check xclip or wl-paste
  const check = await run("sh", ["-c",
    'xclip -selection clipboard -t TARGETS -o 2>/dev/null | grep -qE "image/(png|jpeg)" || wl-paste -l 2>/dev/null | grep -qE "image/(png|jpeg)"'
  ], 2000)
  if (check.code !== 0) return null

  const imgPath = join(tmpdir(), `haft-paste-${Date.now()}.png`)
  const save = await run("sh", ["-c",
    `xclip -selection clipboard -t image/png -o > "${imgPath}" 2>/dev/null || wl-paste --type image/png > "${imgPath}" 2>/dev/null`
  ], 3000)
  if (save.code !== 0) return null

  try {
    const buffer = readFileSync(imgPath)
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
