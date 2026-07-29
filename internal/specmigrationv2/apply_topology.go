package specmigrationv2

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func verifyEffectPathTopology(plan migrationEffectPlan) error {
	root := plan.request.projectRoot.String()
	paths := []string{
		plan.paths.lock,
		plan.paths.journal,
		plan.paths.journal + ".tmp",
		plan.paths.targetStage,
		plan.paths.lineage,
		plan.paths.lineage + ".tmp",
		plan.paths.receipt,
		plan.paths.receipt + ".tmp",
		plan.paths.source,
		plan.paths.target,
		plan.paths.archive,
	}
	if err := verifyCanonicalRealRoot(root); err != nil {
		return err
	}
	for _, path := range paths {
		if err := verifyConfinedPathComponents(root, path); err != nil {
			return err
		}
	}
	targetParent := filepath.Dir(plan.paths.target)
	if err := requireRealDirectory(targetParent); err != nil {
		return fmt.Errorf("migration target parent is not ready: %w", err)
	}
	archiveParent := filepath.Dir(plan.paths.archive)
	archiveAncestor, err := nearestExistingDirectory(root, archiveParent)
	if err != nil {
		return err
	}
	return sameFilesystem([]string{
		plan.paths.lock,
		filepath.Dir(plan.paths.source),
		targetParent,
		archiveAncestor,
	})
}

func verifyCanonicalRealRoot(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("inspect migration project root: %w", err)
	}
	mode := info.Mode()
	symlink := mode&os.ModeSymlink != 0
	if symlink || !info.IsDir() {
		return fmt.Errorf("migration project root must be a real directory, not a symlink")
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve migration project root: %w", err)
	}
	if resolved != root {
		return fmt.Errorf("migration project root must be its canonical physical path")
	}
	return nil
}

func verifyConfinedPathComponents(root string, path string) error {
	relative, err := confinedRelativePath(root, path)
	if err != nil {
		return err
	}
	if relative == "." {
		return nil
	}
	slashed := filepath.ToSlash(relative)
	parts := strings.Split(slashed, "/")
	current := root
	for index, part := range parts {
		component := filepath.FromSlash(part)
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
		if statErr != nil {
			return fmt.Errorf("inspect migration path component %s: %w", current, statErr)
		}
		mode := info.Mode()
		if mode&os.ModeSymlink != 0 {
			return fmt.Errorf("migration path component %s is a symlink", current)
		}
		isLeaf := index == len(parts)-1
		if !isLeaf && !info.IsDir() {
			return fmt.Errorf("migration path component %s is not a directory", current)
		}
	}
	return nil
}

func confinedRelativePath(root string, path string) (string, error) {
	absolute := filepath.IsAbs(path)
	cleaned := filepath.Clean(path)
	if !absolute || cleaned != path {
		return "", fmt.Errorf("migration effect path is not canonical and absolute")
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("relativize migration effect path: %w", err)
	}
	separator := string(filepath.Separator)
	parentPrefix := ".." + separator
	escape := relative == ".." || strings.HasPrefix(relative, parentPrefix)
	if escape || filepath.IsAbs(relative) {
		return "", fmt.Errorf("migration effect path escapes the project root")
	}
	return relative, nil
}

func requireRealDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	mode := info.Mode()
	if mode&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s is not a real directory", path)
	}
	return nil
}

func nearestExistingDirectory(root string, path string) (string, error) {
	current := path
	for {
		info, err := os.Lstat(current)
		realDirectory := false
		if err == nil {
			mode := info.Mode()
			realDirectory = mode&os.ModeSymlink == 0 && info.IsDir()
		}
		if realDirectory {
			return current, nil
		}
		if err == nil {
			return "", fmt.Errorf("migration archive ancestor %s is not a real directory", current)
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if current == root {
			return "", fmt.Errorf("migration archive path has no project-root ancestor")
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("migration archive path escaped its project root")
		}
		current = parent
	}
}

func readRegularFileNoFollow(path string) ([]byte, error) {
	file, err := openReadOnlyNoFollow(path)
	if err != nil {
		return nil, err
	}
	content, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return content, nil
}

func safePathExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("migration path %s is a symlink", path)
	}
	return true, nil
}

func archiveDirectoryChain(root string, archiveParent string) ([]string, error) {
	relative, err := confinedRelativePath(root, archiveParent)
	if err != nil {
		return nil, err
	}
	if relative == "." {
		return []string{}, nil
	}
	slashed := filepath.ToSlash(relative)
	parts := strings.Split(slashed, "/")
	chain := make([]string, 0, len(parts))
	current := root
	for _, part := range parts {
		component := filepath.FromSlash(part)
		current = filepath.Join(current, component)
		chain = append(chain, current)
	}
	return chain, nil
}
