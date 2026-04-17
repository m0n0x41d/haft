package artifact

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Layer groups define the directional architecture for the haft module:
//
//	Core      — pure data, persistence, computation. No side effects beyond DB.
//	Flow      — orchestration of Core artifacts. Allowed to import Core.
//	Surface   — CLI/UI/presentation. Allowed to import Core, Flow, Governor.
//	External  — Tauri desktop shell, BubbleTea TUI binaries. Off-limits to Core.
//
// The matrix asserts what each group MUST NOT import. Drift in either direction
// breaks the architecture even when the build still passes.
var (
	// pureCorePackages must be free of any flow/surface imports. internal/fpf
	// is excluded — it has a known controlled exception in semantic_embedder.go
	// (imports internal/provider for OpenAI embeddings), tracked as P2
	// architectural debt for 6.3.
	pureCorePackages = []string{
		"github.com/m0n0x41d/haft/internal/artifact",
		"github.com/m0n0x41d/haft/internal/graph",
		"github.com/m0n0x41d/haft/internal/reff",
		"github.com/m0n0x41d/haft/internal/codebase",
		"github.com/m0n0x41d/haft/assurance",
	}

	// Surface and external packages — Core MUST NOT import any of these.
	forbiddenForCorePrefixes = []string{
		"github.com/m0n0x41d/haft/internal/cli",
		"github.com/m0n0x41d/haft/internal/present",
		"github.com/m0n0x41d/haft/internal/ui",
		"github.com/m0n0x41d/haft/internal/setup",
		"github.com/m0n0x41d/haft/internal/skills",
		"github.com/m0n0x41d/haft/internal/provider",
		"github.com/m0n0x41d/haft/internal/agent",
		"github.com/m0n0x41d/haft/internal/agentloop",
		"github.com/m0n0x41d/haft/internal/tasks",
		"github.com/m0n0x41d/haft/internal/session",
		"github.com/m0n0x41d/haft/internal/tools",
		"github.com/m0n0x41d/haft/desktop",
		"github.com/m0n0x41d/haft/desktop-tauri",
		"github.com/m0n0x41d/haft/cmd/haft",
	}
)

// TestPureCoreDoesNotDependOnSurfaceOrFlow asserts the layered architecture:
// Pure Core packages depend only on stdlib, db, logger, config-like primitives
// — never on flow orchestration, surfaces, providers, or external shells.
// Catches silent skip-level dependencies the build would happily compile.
//
// Excluded: internal/fpf (known controlled exception, see pureCorePackages).
func TestPureCoreDoesNotDependOnSurfaceOrFlow(t *testing.T) {
	root := projectRootFromTestFile(t)

	args := append([]string{"list", "-deps"}, pureCorePackages...)
	cmd := exec.Command("go", args...)
	cmd.Dir = root

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps failed: %v\n%s", err, output)
	}

	offenders := collectForbiddenDependencies(string(output), forbiddenForCorePrefixes)
	if len(offenders) == 0 {
		return
	}

	t.Fatalf("pure-core packages depend on disallowed packages:\n%s", strings.Join(offenders, "\n"))
}

// TestFPFDependencyExceptions documents that internal/fpf has a known set of
// controlled flow imports (provider for OpenAI embeddings, agent transitively).
// Fails if NEW flow imports appear beyond this allowed set — catches drift
// while the architectural debt is still tracked as a follow-up.
func TestFPFDependencyExceptions(t *testing.T) {
	root := projectRootFromTestFile(t)
	cmd := exec.Command("go", "list", "-deps", "./internal/fpf/...")
	cmd.Dir = root

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps failed: %v\n%s", err, output)
	}

	// Allowed flow imports for fpf as of 2026-04-18 — semantic search prototype.
	allowedFlowImports := map[string]bool{
		"github.com/m0n0x41d/haft/internal/provider": true,
		"github.com/m0n0x41d/haft/internal/agent":    true,
	}

	offenders := make([]string, 0)
	seen := make(map[string]struct{})
	for _, line := range strings.Split(string(output), "\n") {
		importPath := strings.TrimSpace(line)
		if importPath == "" || strings.HasPrefix(importPath, "go:") {
			continue
		}
		if !matchesAnyPrefix(importPath, forbiddenForCorePrefixes) {
			continue
		}
		if allowedFlowImports[importPath] {
			continue
		}
		if _, exists := seen[importPath]; exists {
			continue
		}
		seen[importPath] = struct{}{}
		offenders = append(offenders, importPath)
	}

	if len(offenders) > 0 {
		t.Fatalf("fpf gained new flow imports beyond the documented exception list:\n%s\nIf intentional, update allowedFlowImports in this test.", strings.Join(offenders, "\n"))
	}
}

// TestCorePackagesDoNotDependOnDesktop is the original narrow assertion kept
// as a focused regression test. The broader matrix above subsumes it but this
// version produces a more targeted error message when desktop drift is the
// specific problem.
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

// collectForbiddenDependencies returns deduplicated import paths from the
// `go list -deps` output that match any of the forbidden prefixes.
func collectForbiddenDependencies(output string, forbiddenPrefixes []string) []string {
	offenders := make([]string, 0)
	seen := make(map[string]struct{})

	for _, line := range strings.Split(output, "\n") {
		importPath := strings.TrimSpace(line)
		if importPath == "" || strings.HasPrefix(importPath, "go:") {
			continue
		}
		if !matchesAnyPrefix(importPath, forbiddenPrefixes) {
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

func matchesAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if s == p || strings.HasPrefix(s, p+"/") {
			return true
		}
	}
	return false
}
