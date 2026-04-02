#!/usr/bin/env bash
# Build haft: TUI bundle + Go binary.
# Usage: ./scripts/build.sh [--install]
#
# Output:
#   tui/dist/tui.mjs  — bundled TUI
#   bin/haft           — Go binary
#   ~/.haft/tui/bundle.mjs — installed TUI (with --install)

set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"

echo "=== Building haft ==="

# 1. Build TUI bundle
echo "Building TUI..."
cd tui
if command -v bun &>/dev/null; then
  bun run build
elif command -v npx &>/dev/null; then
  npx esbuild src/index.tsx --bundle --platform=node --format=esm \
    --outfile=dist/tui.mjs --external:yoga-wasm-web --external:@aspect-build/rules_js
else
  echo "Error: bun or npx required to build TUI" >&2
  exit 1
fi
cd "$PROJECT_DIR"
echo "  tui/dist/tui.mjs"

# 2. Build Go binary
echo "Building Go binary..."
mkdir -p bin
go build -o bin/haft ./cmd/haft
echo "  bin/haft"

# 3. Install TUI bundle (optional)
if [[ "${1:-}" == "--install" ]]; then
  echo "Installing TUI bundle..."
  mkdir -p "$HOME/.haft/tui"
  cp tui/dist/tui.mjs "$HOME/.haft/tui/bundle.mjs"
  echo "  ~/.haft/tui/bundle.mjs"
fi

echo ""
echo "Done. Run: ./bin/haft agent"
