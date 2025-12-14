#!/bin/bash
# Adapter: OpenAI Codex CLI
# Format: Markdown with optional YAML frontmatter
# Output: dist/codex/
# Note: Codex uses same $ARGUMENTS syntax as Claude Code

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
SRC_DIR="$ROOT_DIR/src/commands"
OUT_DIR="$ROOT_DIR/dist/codex"

mkdir -p "$OUT_DIR"

for file in "$SRC_DIR"/*.md; do
    filename=$(basename "$file")
    name="${filename%.md}"

    # Extract first heading as description (remove # prefix)
    description=$(head -20 "$file" | grep -m1 "^#" | sed 's/^#* *//' || echo "FPF command: $name")

    # Create file with YAML frontmatter
    {
        echo "---"
        echo "description: $description"
        echo "argument-hint: (optional)"
        echo "---"
        echo ""
        cat "$file"
    } > "$OUT_DIR/$filename"
done

echo "Codex CLI: $(ls "$OUT_DIR" | wc -l | tr -d ' ') commands"
