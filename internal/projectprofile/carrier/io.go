package carrier

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

// RelativePath is the location of the legacy, read-only compatibility carrier.
const RelativePath = ".haft/project-profile.yaml"

// Load reads a legacy project-profile carrier without modifying it. A missing
// carrier retains the historical Auto fallback, but does not establish a
// canonical profile admission.
func Load(projectRoot string) (projectprofile.ConfiguredProjectProfile, error) {
	path, err := carrierPath(projectRoot)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return projectprofile.Auto{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect project profile carrier: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("project profile carrier must be a regular non-symlink file")
	}
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read project profile carrier: %w", err)
	}
	profile, err := Decode(source)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	return profile, nil
}

func carrierPath(projectRoot string) (string, error) {
	root, err := canonicalProjectRoot(projectRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, filepath.FromSlash(RelativePath)), nil
}

func canonicalProjectRoot(projectRoot string) (string, error) {
	if projectRoot == "" || projectRoot != strings.TrimSpace(projectRoot) {
		return "", fmt.Errorf("project root must be a canonical absolute path without surrounding whitespace")
	}
	cleaned := filepath.Clean(projectRoot)
	if !filepath.IsAbs(cleaned) || cleaned != projectRoot {
		return "", fmt.Errorf("project root %q must be a canonical absolute path", projectRoot)
	}
	return cleaned, nil
}
