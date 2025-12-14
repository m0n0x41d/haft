#!/bin/bash
# Adapter: Gemini CLI
# Format: TOML
# Output: dist/gemini/
# Transforms: $ARGUMENTS -> {{args}}, Markdown -> TOML wrapper

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
SRC_DIR="$ROOT_DIR/src/commands"
OUT_DIR="$ROOT_DIR/dist/gemini"

mkdir -p "$OUT_DIR"

escape_toml() {
    # Escape backslashes and quotes for TOML multi-line string
    sed 's/\\/\\\\/g' | sed 's/"""/\\"""/g'
}

for file in "$SRC_DIR"/*.md; do
    filename=$(basename "$file")
    name="${filename%.md}"
    toml_file="$OUT_DIR/${name}.toml"

    # Extract first heading as description
    description=$(head -20 "$file" | grep -m1 "^#" | sed 's/^#* *//' || echo "FPF command: $name")
    # Escape quotes in description
    description=$(echo "$description" | sed 's/"/\\"/g')

    # Read content, transform arguments, escape for TOML
    content=$(cat "$file" | sed 's/\$ARGUMENTS/{{args}}/g' | sed 's/\$\([1-9]\)/{{\1}}/g' | escape_toml)

    # Write TOML file
    {
        echo "description = \"$description\""
        echo ""
        echo "prompt = \"\"\""
        echo "$content"
        echo "\"\"\""
    } > "$toml_file"
done

echo "Gemini CLI: $(ls "$OUT_DIR" | wc -l | tr -d ' ') commands"
