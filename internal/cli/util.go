package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// findProjectRoot walks up from cwd until it finds a .haft/ directory.
func findProjectRoot() (string, error) {
	dir, err := projectRootSearchStart()
	if err != nil {
		return "", err
	}

	return findProjectRootFrom(dir)
}

func projectRootSearchStart() (string, error) {
	envRoot := strings.TrimSpace(os.Getenv("HAFT_PROJECT_ROOT"))
	if envRoot != "" {
		return filepath.Abs(envRoot)
	}

	return os.Getwd()
}

func findProjectRootFrom(dir string) (string, error) {
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
