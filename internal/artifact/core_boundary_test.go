package artifact

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCorePackagesDoNotDependOnDesktop(t *testing.T) {
	root := projectRootFromTestFile(t)
	patterns := []string{
		"./internal/artifact/...",
		"./internal/graph/...",
		"./internal/fpf/...",
		"./internal/reff/...",
		"./internal/codebase/...",
	}
	args := append([]string{"list", "-deps"}, patterns...)
	cmd := exec.Command("go", args...)
	cmd.Dir = root

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps failed: %v\n%s", err, output)
	}

	offenders := collectDesktopDependencies(string(output))
	if len(offenders) == 0 {
		return
	}

	t.Fatalf("core packages depend on desktop packages:\n%s", strings.Join(offenders, "\n"))
}

func projectRootFromTestFile(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func collectDesktopDependencies(output string) []string {
	offenders := make([]string, 0)
	seen := make(map[string]struct{})

	for _, line := range strings.Split(output, "\n") {
		importPath := strings.TrimSpace(line)
		if importPath == "" || strings.HasPrefix(importPath, "go:") {
			continue
		}
		if !isDesktopImportPath(importPath) {
			continue
		}
		if _, exists := seen[importPath]; exists {
			continue
		}

		seen[importPath] = struct{}{}
		offenders = append(offenders, importPath)
	}

	return offenders
}

func isDesktopImportPath(importPath string) bool {
	return importPath == "github.com/m0n0x41d/haft/desktop" ||
		strings.HasPrefix(importPath, "github.com/m0n0x41d/haft/desktop/")
}
