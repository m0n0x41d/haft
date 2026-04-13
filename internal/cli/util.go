package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// findProjectRoot walks up from cwd until it finds a .haft/ directory.
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, ".haft")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no .haft/ found")
		}
		dir = parent
	}
}
