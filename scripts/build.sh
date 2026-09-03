#!/usr/bin/env bash
# Build haft CLI binary.
# Usage: HAFT_BUILD_VERSION=9.2.0 ./scripts/build.sh [--install]
#
# Output:
#   bin/haft — Go binary

set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"

echo "=== Building haft CLI ==="

echo "Building Go binary..."
mkdir -p bin
BUILD_VERSION="${HAFT_BUILD_VERSION:-dev}"
case "$BUILD_VERSION" in
  *[!0-9A-Za-z.+-]*)
    echo "HAFT_BUILD_VERSION must contain only version-safe characters" >&2
    exit 1
    ;;
esac
COMMIT="$(git rev-parse HEAD)"
if ! GIT_STATUS="$(git status --porcelain=v1 --untracked-files=all)"; then
  echo "Unable to read Git status for build identity" >&2
  exit 1
fi
if [ -n "$GIT_STATUS" ]; then
  COMMIT="${COMMIT}-dirty"
fi
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-X github.com/m0n0x41d/haft/internal/cli.Version=${BUILD_VERSION} -X github.com/m0n0x41d/haft/internal/cli.Commit=${COMMIT} -X github.com/m0n0x41d/haft/internal/cli.BuildDate=${BUILD_DATE}"
go build -buildvcs=true -ldflags "$LDFLAGS" -o bin/haft ./cmd/haft
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
