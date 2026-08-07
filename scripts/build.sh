#!/usr/bin/env bash
# Build haft CLI binary.
# Usage: ./scripts/build.sh [--install]
#
# Output:
#   bin/haft — Go binary

set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"

echo "=== Building haft CLI ==="

echo "Building Go binary..."
mkdir -p bin
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo none)"
if ! git diff --quiet --ignore-submodules -- 2>/dev/null || ! git diff --cached --quiet --ignore-submodules -- 2>/dev/null; then
  COMMIT="${COMMIT}-dirty"
fi
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-X github.com/m0n0x41d/haft/internal/cli.Commit=${COMMIT} -X github.com/m0n0x41d/haft/internal/cli.BuildDate=${BUILD_DATE}"
go build -ldflags "$LDFLAGS" -o bin/haft ./cmd/haft
echo "  bin/haft"

if [[ "${1:-}" == "--install" ]]; then
  echo "Installing CLI..."
  mkdir -p "$HOME/.local/bin"
  INSTALL_TARGET="$HOME/.local/bin/haft"
  INSTALL_TMP="${INSTALL_TARGET}.tmp.$$"
  cp bin/haft "$INSTALL_TMP"
  chmod +x "$INSTALL_TMP"
  mv "$INSTALL_TMP" "$INSTALL_TARGET"
  echo "  ~/.local/bin/haft"
fi

echo ""
echo "Done. Run: ./bin/haft"
