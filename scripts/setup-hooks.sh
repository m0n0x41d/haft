#!/bin/bash
# Setup git hooks for local development
#
# Usage:
#   ./scripts/setup-hooks.sh          # Use simple .githooks
#   ./scripts/setup-hooks.sh --precommit  # Use pre-commit tool (requires pip install pre-commit)

set -e

REPO_ROOT=$(git rev-parse --show-toplevel)

if [ "$1" = "--precommit" ]; then
    if ! command -v pre-commit &> /dev/null; then
        echo "pre-commit not found. Install with: pip install pre-commit"
        exit 1
    fi
    echo "Installing pre-commit hooks..."
    pre-commit install
    echo "Done! Hooks installed via pre-commit."
else
    echo "Configuring git to use .githooks directory..."
    git config core.hooksPath .githooks
    echo "Done! Git hooks installed from .githooks/"
    echo ""
    echo "Hooks enabled:"
    echo "  - pre-commit: format, build, test, lint checks"
    echo ""
    echo "To use pre-commit tool instead: ./scripts/setup-hooks.sh --precommit"
fi
