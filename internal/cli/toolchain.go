package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// toolchain describes how to build, test, and verify a project.
// Detected from filesystem markers at the project root.
type toolchain struct {
	lang       string // go, ts, js, rust, python, c_cpp, make, unknown
	buildCmd   string // shell command to verify compilation (empty = skip)
	testCmd    string // shell command to run tests (empty = skip)
	verifyHint string // human-readable instruction for agent prompts
}

// acceptLine returns a crisp pass/fail statement for plan acceptance criteria.
func (tc toolchain) acceptLine() string {
	var checks []string
	if tc.buildCmd != "" {
		checks = append(checks, "`"+tc.buildCmd+"` succeeds")
	}
	if tc.testCmd != "" {
		checks = append(checks, "`"+tc.testCmd+"` passes")
	}
	if len(checks) == 0 {
		return "All tests pass, build succeeds"
	}
	return strings.Join(checks, ", ")
}

// detectToolchain probes the project root for language markers.
// Checks in priority order: Go, Rust, JS/TS, Python, C/C++, Makefile.
func detectToolchain(projectRoot string) toolchain {
	if fileExists(projectRoot, "go.mod") {
		return goToolchain()
	}
	if fileExists(projectRoot, "Cargo.toml") {
		return rustToolchain()
	}
	if fileExists(projectRoot, "package.json") {
		return jstsToolchain(projectRoot)
	}
	for _, marker := range []string{"pyproject.toml", "setup.py", "setup.cfg"} {
		if fileExists(projectRoot, marker) {
			return pythonToolchain(projectRoot)
		}
	}
	if fileExists(projectRoot, "CMakeLists.txt") {
		return toolchain{
			lang:       "c/c++",
			buildCmd:   "cmake --build build",
			testCmd:    "ctest --test-dir build",
			verifyHint: "Run `cmake --build build` and `ctest --test-dir build` to verify.",
		}
	}
	if fileExists(projectRoot, "Makefile") || fileExists(projectRoot, "makefile") {
		return toolchain{
			lang:       "make",
			buildCmd:   "make",
			verifyHint: "Run `make` to verify the build.",
		}
	}
	return toolchain{
		lang:       "unknown",
		verifyHint: "Verify the changes compile and pass tests using the project's build system.",
	}
}

func goToolchain() toolchain {
	return toolchain{
		lang:       "go",
		buildCmd:   "go build ./...",
		testCmd:    "go test ./...",
		verifyHint: "Run `go build ./...` to verify compilation.",
	}
}

func rustToolchain() toolchain {
	return toolchain{
		lang:       "rust",
		buildCmd:   "cargo check",
		testCmd:    "cargo test",
		verifyHint: "Run `cargo check` to verify compilation.",
	}
}

func jstsToolchain(projectRoot string) toolchain {
	pm := detectPackageManager(projectRoot)
	hasTS := fileExists(projectRoot, "tsconfig.json")
	scripts := readPkgScripts(projectRoot)

	lang := "js"
	if hasTS {
		lang = "ts"
	}

	// Build: prefer explicit script, fallback to tsc for TypeScript
	var buildCmd string
	switch {
	case scripts["build"] != "":
		buildCmd = pm + " run build"
	case scripts["typecheck"] != "":
		buildCmd = pm + " run typecheck"
	case hasTS:
		buildCmd = "npx tsc --noEmit"
	}

	// Tests
	var testCmd string
	if scripts["test"] != "" {
		testCmd = pm + " test"
	}

	return toolchain{
		lang:       lang,
		buildCmd:   buildCmd,
		testCmd:    testCmd,
		verifyHint: buildVerifyHint(buildCmd, testCmd),
	}
}

func pythonToolchain(projectRoot string) toolchain {
	// Type checking: only if explicitly configured
	var buildCmd string
	switch {
	case fileExists(projectRoot, "mypy.ini") || fileExists(projectRoot, ".mypy.ini"):
		buildCmd = "mypy ."
	case fileExists(projectRoot, "pyrightconfig.json"):
		buildCmd = "pyright"
	}

	// pytest is the de facto standard
	testCmd := "python -m pytest"

	return toolchain{
		lang:       "python",
		buildCmd:   buildCmd,
		testCmd:    testCmd,
		verifyHint: buildVerifyHint(buildCmd, testCmd),
	}
}

func buildVerifyHint(buildCmd, testCmd string) string {
	var parts []string
	if buildCmd != "" {
		parts = append(parts, "`"+buildCmd+"`")
	}
	if testCmd != "" {
		parts = append(parts, "`"+testCmd+"`")
	}
	if len(parts) == 0 {
		return "Verify the changes compile and pass tests using the project's build system."
	}
	return "Run " + strings.Join(parts, " and ") + " to verify."
}

// detectPackageManager checks lockfiles to determine the JS/TS package manager.
func detectPackageManager(projectRoot string) string {
	switch {
	case fileExists(projectRoot, "pnpm-lock.yaml"):
		return "pnpm"
	case fileExists(projectRoot, "yarn.lock"):
		return "yarn"
	case fileExists(projectRoot, "bun.lockb") || fileExists(projectRoot, "bun.lock"):
		return "bun"
	default:
		return "npm"
	}
}

// readPkgScripts reads the "scripts" field from package.json.
func readPkgScripts(projectRoot string) map[string]string {
	data, err := os.ReadFile(filepath.Join(projectRoot, "package.json"))
	if err != nil {
		return nil
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	return pkg.Scripts
}

func fileExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}
