#!/bin/bash
# Build script: Runs all adapters to generate platform-specific commands
# Output: dist/{platform}/

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ADAPTERS_DIR="$SCRIPT_DIR/adapters"

echo "Building Quint Code for all platforms..."
echo ""

# Clean dist
rm -rf "$SCRIPT_DIR/dist"
mkdir -p "$SCRIPT_DIR/dist"

# Run each adapter
for adapter in "$ADAPTERS_DIR"/*.sh; do
    bash "$adapter"
done

echo ""
echo "Build complete. Output in dist/"
