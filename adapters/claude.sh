#!/bin/bash
# Adapter: Claude Code
# Format: Markdown (native - no transformation needed)
# Output: dist/claude/

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
SRC_DIR="$ROOT_DIR/src/commands"
OUT_DIR="$ROOT_DIR/dist/claude"

mkdir -p "$OUT_DIR"

for file in "$SRC_DIR"/*.md; do
    filename=$(basename "$file")
    cp "$file" "$OUT_DIR/$filename"
done

echo "Claude Code: $(ls "$OUT_DIR" | wc -l | tr -d ' ') commands"
