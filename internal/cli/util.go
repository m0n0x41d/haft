package cli

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/project"
)

// findProjectRoot walks up from cwd until it finds a .haft/ directory.
// Thin wrapper around project.FindRootFromCwd that preserves the error-
// returning shape the existing cli callers expect.
func findProjectRoot() (string, error) {
	root, ok := project.FindRootFromCwd()
	if !ok {
		return "", fmt.Errorf("no .haft/ found")
	}
	return root, nil
}
