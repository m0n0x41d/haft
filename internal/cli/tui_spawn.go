package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/m0n0x41d/haft/internal/envutil"
)

// spawnTUI finds the TUI entry point and launches it with bun or node.
// Returns the process, its stdin (we write to it), and stdout (we read from it).
// The TUI process inherits the terminal (stderr + tty) for rendering.
func spawnTUI(projectRoot string) (*exec.Cmd, io.WriteCloser, io.ReadCloser, error) {
	tuiEntry, err := findTUIEntry(projectRoot)
	if err != nil {
		return nil, nil, nil, err
	}

	runtime_, args, err := findJSRuntime(tuiEntry)
	if err != nil {
		return nil, nil, nil, err
	}

	cmd := exec.Command(runtime_, append(args, tuiEntry)...)
	cmd.Stderr = os.Stderr // TUI debug output goes to stderr
	cmd.Dir = projectRoot

	// TUI opens /dev/tty directly for terminal rendering.
	// stdin/stdout pipes are used exclusively for JSON-RPC.
	cmd.Env = tuiProcessEnv(os.Environ(), os.Getenv("TERM"))

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, nil, fmt.Errorf("start TUI: %w", err)
	}

	return cmd, stdin, stdout, nil
}

// findTUIEntry locates the TUI entry point.
// Search order:
// 1. ~/.haft/tui/bundle.mjs (installed bundle)
// 2. tui/src/index.tsx in the Haft repo (dev mode)
// 3. tui/dist/tui.mjs in the Haft repo (built)
func findTUIEntry(projectRoot string) (string, error) {
	expectedPaths := []string{
		filepath.Join(homeDir(), ".haft", "tui", "bundle.mjs"),
		filepath.Join(projectRoot, "tui", "src", "index.tsx"),
		filepath.Join(projectRoot, "tui", "dist", "tui.mjs"),
	}
	candidates := []string{
		expectedPaths[0],
	}

	if isHaftRepo(projectRoot) {
		candidates = expectedPaths
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("TUI not found. Expected at:\n  %s\n  %s\n  %s",
		expectedPaths[0], expectedPaths[1], expectedPaths[2])
}

func isHaftRepo(projectRoot string) bool {
	goModPath := filepath.Join(projectRoot, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return false
	}

	return strings.Contains(string(content), "module github.com/m0n0x41d/haft")
}

// findJSRuntime finds bun or node in PATH.
// Returns (binary, extra args before entry file).
// For .tsx files: bun runs them directly; node needs tsx loader.
func findJSRuntime(entryFile string) (string, []string, error) {
	isTSX := filepath.Ext(entryFile) == ".tsx" || filepath.Ext(entryFile) == ".ts"

	// Prefer bun — handles .tsx natively
	if path, err := exec.LookPath("bun"); err == nil {
		return path, nil, nil
	}
	// tsx (node + TypeScript loader)
	if isTSX {
		if path, err := exec.LookPath("tsx"); err == nil {
			return path, nil, nil
		}
	}
	// node (only for .mjs/.js bundles)
	if !isTSX {
		if path, err := exec.LookPath("node"); err == nil {
			return path, nil, nil
		}
	}
	return "", nil, fmt.Errorf("haft requires bun or node 20+ for the terminal UI.\n  Install: https://bun.sh or https://nodejs.org")
}

func homeDir() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("USERPROFILE")
	}
	return os.Getenv("HOME")
}

func tuiProcessEnv(base []string, term string) []string {
	return append(envutil.Strip(base, "DEV", "FORCE_COLOR", "TERM"),
		"DEV=false",
		"FORCE_COLOR=1",
		"TERM="+term,
	)
}
