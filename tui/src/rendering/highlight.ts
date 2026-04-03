// L2: Syntax highlighting — effect boundary.
// Lazy-loads cli-highlight on first use.
// Single shared promise so the ~50MB highlight.js bundle loads once.
//
// IMPORTANT: highlight.js has pathological regex backtracking on certain inputs.
// We guard each call with a per-string length limit. If a code block is too
// large, we return it unhighlighted rather than risk freezing the event loop.

import { trace as _trace } from "../debug.js"

type CliHighlight = {
  highlight: typeof import("cli-highlight").highlight
  supportsLanguage: typeof import("cli-highlight").supportsLanguage
}

// Max code length we'll attempt to highlight. highlight.js regex engine
// can catastrophically backtrack on large inputs.
const MAX_HIGHLIGHT_LEN = 8192

let loadPromise: Promise<CliHighlight | null> | undefined
let hl: CliHighlight | null = null
let loadStarted = false

function load(): Promise<CliHighlight | null> {
  loadPromise ??= import("cli-highlight")
    .then((mod) => {
      _trace("highlight: module loaded")
      const loaded = {
        highlight: mod.highlight,
        supportsLanguage: mod.supportsLanguage,
      }
      hl = loaded
      return loaded
    })
    .catch((e) => {
      _trace(`highlight: load failed ${e}`)
      return null
    })
  return loadPromise
}

// LRU cache keyed by `lang\0code`.
const CACHE_MAX = 256
const cache = new Map<string, string>()

function cached(key: string): string | undefined {
  const hit = cache.get(key)
  if (hit !== undefined) {
    cache.delete(key)
    cache.set(key, hit)
  }
  return hit
}

function store(key: string, value: string): void {
  if (cache.size >= CACHE_MAX) {
    const first = cache.keys().next().value
    if (first !== undefined) cache.delete(first)
  }
  cache.set(key, value)
}

/**
 * Kick off the highlight.js load in the background.
 * Does NOT block, does NOT trigger re-renders.
 * Call once at app startup so the bundle is ready when first code block arrives.
 */
export function preloadHighlighter(): void {
  if (loadStarted) return
  loadStarted = true
  const t0 = Date.now()
  load().then((loaded) => {
    const elapsed = Date.now() - t0
    _trace(`preloadHighlighter: loaded in ${elapsed}ms, module=${loaded ? "ok" : "null"}`)
  })
}

/**
 * Highlight `code` with the given language.
 * Returns ANSI-escaped string, or plain `code` if highlighting unavailable.
 *
 * Pure synchronous call — returns immediately if the library hasn't loaded yet.
 * No re-render trigger: components will pick up highlighting on their next
 * natural render cycle (scroll, new message, etc.).
 */
export function highlightCode(code: string, language: string): string {
  if (!hl) {
    // Kick off lazy load if not started
    if (!loadStarted) preloadHighlighter()
    return code
  }

  // Guard against pathological inputs
  if (code.length > MAX_HIGHLIGHT_LEN) return code

  const key = `${language}\0${code}`
  const hit = cached(key)
  if (hit !== undefined) return hit

  const lang = language && hl.supportsLanguage(language) ? language : "plaintext"
  try {
    const result = hl.highlight(code, { language: lang })
    store(key, result)
    return result
  } catch {
    return code
  }
}

/**
 * Highlight with async guarantee — waits for load if needed.
 */
export async function highlightCodeAsync(code: string, language: string): Promise<string> {
  if (code.length > MAX_HIGHLIGHT_LEN) return code
  const loaded = await load()
  if (!loaded) return code
  return highlightCode(code, language)
}
