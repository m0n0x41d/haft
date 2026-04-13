#!/usr/bin/env bun
// Build script: bundles the TUI into a single .mjs file for go:embed distribution.
// Usage: bun run build   (or: bun tui/build.ts)

import { build } from "esbuild"
import { resolve, dirname } from "path"

const __dirname = dirname(new URL(import.meta.url).pathname)

await build({
  entryPoints: [resolve(__dirname, "src/index.tsx")],
  bundle: true,
  platform: "node",
  format: "esm",
  outfile: resolve(__dirname, "dist/tui.mjs"),
  external: [
    // Ink's native deps — resolved at runtime from node_modules
    "yoga-wasm-web",
    "@aspect-build/rules_js",
    "react-devtools-core",
  ],
  target: "node20",
  minify: false, // keep readable for debugging
  sourcemap: false,
  define: {
    "process.env.NODE_ENV": '"production"',
  },
})

console.log("Built tui/dist/tui.mjs")
