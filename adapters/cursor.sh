#!/bin/bash
# Adapter: Cursor
# Format: Markdown (same as Claude Code)
# Output: dist/cursor/

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
SRC_DIR="$ROOT_DIR/src/commands"
OUT_DIR="$ROOT_DIR/dist/cursor"

mkdir -p "$OUT_DIR"

for file in "$SRC_DIR"/*.md; do
    filename=$(basename "$file")
    cp "$file" "$OUT_DIR/$filename"
done

echo "Cursor: $(ls "$OUT_DIR" | wc -l | tr -d ' ') commands"
