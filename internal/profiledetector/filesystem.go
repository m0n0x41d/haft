package profiledetector

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

const scanFileLimit = 100000

var ignoredDirectories = []string{
	".git",
	".haft",
	".venv",
	"build",
	"dist",
	"node_modules",
	"target",
	"vendor",
}

// Inspect walks only regular, non-symlink files under the canonical root.
// It does not read file contents and never writes project state.
func Inspect(projectRoot string) (Suggestion, error) {
	root, err := canonicalPhysicalRoot(projectRoot)
	if err != nil {
		return Suggestion{}, err
	}
	files, scanned, truncated, err := inspectRelativeFiles(root)
	if err != nil {
		return Suggestion{}, err
	}
	snapshot, err := NewSnapshot(root, files, scanned, truncated)
	if err != nil {
		return Suggestion{}, err
	}
	return Detect(snapshot), nil
}

func canonicalPhysicalRoot(raw string) (string, error) {
	absolute, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("resolve detector project root: %w", err)
	}
	physical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve physical detector project root: %w", err)
	}
	physical = filepath.Clean(physical)
	info, err := os.Lstat(physical)
	if err != nil {
		return "", fmt.Errorf("inspect detector project root: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("detector project root must be a real directory")
	}
	return physical, nil
}

func inspectRelativeFiles(root string) ([]string, int, bool, error) {
	files := []string{}
	scanned := 0
	truncated := false
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if entry.IsDir() && slices.Contains(ignoredDirectories, entry.Name()) {
			return filepath.SkipDir
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		scanned++
		if scanned > scanFileLimit {
			truncated = true
			return fs.SkipAll
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, 0, false, fmt.Errorf("inspect project profile signals: %w", err)
	}
	return files, scanned, truncated, nil
}
