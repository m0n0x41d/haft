#!/bin/bash

# FPF-Claude Installation Script

set -e

# Parse flags
GLOBAL=false
TARGET_DIR=""

while [[ $# -gt 0 ]]; do
  case $1 in
  -g | --global)
    GLOBAL=true
    shift
    ;;
  *)
    TARGET_DIR="$1"
    shift
    ;;
  esac
done

if [ "$GLOBAL" = true ]; then
  TARGET_DIR="$HOME"
  echo "Installing FPF-Claude globally to: ~/.claude/commands/"
else
  TARGET_DIR="${TARGET_DIR:-.}"
  echo "Installing FPF-Claude to: $TARGET_DIR/.claude/commands/"
fi

# Create .claude/commands if not exists
mkdir -p "$TARGET_DIR/.claude/commands"

# Copy commands
echo "Copying commands..."
cp -r commands/fpf-*.md "$TARGET_DIR/.claude/commands/"

# Create .fpf structure (only for local installs)
if [ "$GLOBAL" = false ]; then
  if [ ! -d "$TARGET_DIR/.fpf" ]; then
    echo "Creating .fpf/ structure..."
    mkdir -p "$TARGET_DIR/.fpf/evidence"
    mkdir -p "$TARGET_DIR/.fpf/decisions"
    mkdir -p "$TARGET_DIR/.fpf/sessions"
    mkdir -p "$TARGET_DIR/.fpf/knowledge/L0"
    mkdir -p "$TARGET_DIR/.fpf/knowledge/L1"
    mkdir -p "$TARGET_DIR/.fpf/knowledge/L2"
    mkdir -p "$TARGET_DIR/.fpf/knowledge/invalid"
    touch "$TARGET_DIR/.fpf/evidence/.gitkeep"
    touch "$TARGET_DIR/.fpf/decisions/.gitkeep"
    touch "$TARGET_DIR/.fpf/sessions/.gitkeep"
    touch "$TARGET_DIR/.fpf/knowledge/L0/.gitkeep"
    touch "$TARGET_DIR/.fpf/knowledge/L1/.gitkeep"
    touch "$TARGET_DIR/.fpf/knowledge/L2/.gitkeep"
    touch "$TARGET_DIR/.fpf/knowledge/invalid/.gitkeep"
  else
    echo ".fpf/ already exists, skipping structure creation"
  fi

  # Check CLAUDE.md
  if [ -f "$TARGET_DIR/CLAUDE.md" ]; then
    if grep -q "FPF Mode" "$TARGET_DIR/CLAUDE.md"; then
      echo "CLAUDE.md already has FPF section"
    else
      echo ""
      echo "⚠️  Add FPF section to your CLAUDE.md manually."
      echo "   See CLAUDE.md in this repo for template."
    fi
  else
    echo ""
    echo "⚠️  No CLAUDE.md found. Consider creating one."
    echo "   See CLAUDE.md in this repo for template."
  fi
fi

echo ""
echo "✓ Installation complete!"
echo ""
echo "Commands available:"
echo "  /fpf-0-init        - Initialize FPF"
echo "  /fpf-1-hypothesize - Generate hypotheses"
echo "  /fpf-2-check       - Logical verification"
echo "  /fpf-3-test        - Internal tests, benchmarks"
echo "  /fpf-3-research    - External evidence (web, docs)"
echo "  /fpf-4-audit       - WLNK + critical review"
echo "  /fpf-5-decide      - Finalize decision"
echo "  /fpf-status        - Show status"
echo "  /fpf-query         - Search knowledge"
echo "  /fpf-decay         - Check evidence freshness"
echo "  /fpf-discard       - Stop session, preserve learnings"
echo ""
echo "Start with: /fpf-0-init"
echo ""
