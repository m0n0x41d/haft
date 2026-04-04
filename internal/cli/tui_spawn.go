package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
// 1. tui/src/index.tsx in project root (dev mode)
// 2. ~/.haft/tui/bundle.mjs (embedded extraction)
// 3. tui/dist/tui.mjs in project root (built)
func findTUIEntry(projectRoot string) (string, error) {
	candidates := []string{
		filepath.Join(projectRoot, "tui", "src", "index.tsx"),
		filepath.Join(homeDir(), ".haft", "tui", "bundle.mjs"),
		filepath.Join(projectRoot, "tui", "dist", "tui.mjs"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("TUI not found. Expected at:\n  %s\n  %s\n  %s",
		candidates[0], candidates[1], candidates[2])
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
	filtered := make([]string, 0, len(base)+3)

	for _, entry := range base {
		if strings.HasPrefix(entry, "DEV=") {
			continue
		}
		if strings.HasPrefix(entry, "FORCE_COLOR=") {
			continue
		}
		if strings.HasPrefix(entry, "TERM=") {
			continue
		}

		filtered = append(filtered, entry)
	}

	filtered = append(filtered, "DEV=false")
	filtered = append(filtered, "FORCE_COLOR=1")
	filtered = append(filtered, "TERM="+term)

	return filtered
}
