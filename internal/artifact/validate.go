package artifact

import (
	"path/filepath"
	"slices"
	"strings"
)

// sharedFilePatterns lists file names and extensions that are shared mutable state
// across an entire project. Linking these to a single decision guarantees false
// drift alerts whenever any unrelated dependency changes.
var sharedFilePatterns = []string{
	// Lock files (auto-generated, change on any dependency update)
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"composer.lock",
	"cargo.lock",
	"gemfile.lock",
	"poetry.lock",
	"go.sum",
	"flake.lock",
	"bun.lockb",
	"uv.lock",

	// Manifest files (shared across all project dependencies)
	"package.json",
	"composer.json",
	"cargo.toml",
	"go.mod",
	"gemfile",
	"pyproject.toml",
	"requirements.txt",
	"pom.xml",
	"build.gradle",
	"build.gradle.kts",
}

// WarnSharedFiles checks a list of file paths for known shared/generated files.
// Returns warning strings for each problematic file. Never rejects — warn only.
func WarnSharedFiles(paths []string) []string {
	var warnings []string
	for _, p := range paths {
		base := strings.ToLower(filepath.Base(p))
		if slices.Contains(sharedFilePatterns, base) {
			warnings = append(warnings,
				p+": shared file — changes on any dependency update, causing false drift. Link implementation files instead.")
		}
	}
	return warnings
}
