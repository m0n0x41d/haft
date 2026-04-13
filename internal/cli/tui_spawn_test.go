package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTUIProcessEnvForcesSafeRuntimeDefaults(t *testing.T) {
	base := []string{
		"DEV=true",
		"FORCE_COLOR=0",
		"PATH=/usr/bin",
		"TERM=screen",
	}

	env := tuiProcessEnv(base, "screen-256color")

	if hasEnvEntry(env, "DEV=true") {
		t.Fatalf("DEV=true should be removed from TUI environment")
	}
	if !hasEnvEntry(env, "DEV=false") {
		t.Fatalf("DEV=false missing from TUI environment")
	}
	if !hasEnvEntry(env, "FORCE_COLOR=1") {
		t.Fatalf("FORCE_COLOR=1 missing from TUI environment")
	}
	if !hasEnvEntry(env, "TERM=screen-256color") {
		t.Fatalf("TERM override missing from TUI environment")
	}
}

func hasEnvEntry(env []string, want string) bool {
	for _, entry := range env {
		if entry == want {
			return true
		}
	}

	return false
}

func TestFindTUIEntry_PrefersInstalledBundleOutsideHaftRepo(t *testing.T) {
	homeDir := t.TempDir()
	projectRoot := t.TempDir()

	t.Setenv("HOME", homeDir)

	installedBundle := filepath.Join(homeDir, ".haft", "tui", "bundle.mjs")
	projectSource := filepath.Join(projectRoot, "tui", "src", "index.tsx")

	if err := os.MkdirAll(filepath.Dir(installedBundle), 0o755); err != nil {
		t.Fatalf("mkdir installed bundle dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(projectSource), 0o755); err != nil {
		t.Fatalf("mkdir project source dir: %v", err)
	}
	if err := os.WriteFile(installedBundle, []byte("bundle"), 0o644); err != nil {
		t.Fatalf("write installed bundle: %v", err)
	}
	if err := os.WriteFile(projectSource, []byte("source"), 0o644); err != nil {
		t.Fatalf("write project source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "go.mod"), []byte("module github.com/example/app\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	got, err := findTUIEntry(projectRoot)
	if err != nil {
		t.Fatalf("findTUIEntry returned error: %v", err)
	}
	if got != installedBundle {
		t.Fatalf("findTUIEntry = %q, want %q", got, installedBundle)
	}
}

func TestFindTUIEntry_UsesRepoSourceForHaftCheckout(t *testing.T) {
	homeDir := t.TempDir()
	projectRoot := t.TempDir()

	t.Setenv("HOME", homeDir)

	projectSource := filepath.Join(projectRoot, "tui", "src", "index.tsx")

	if err := os.MkdirAll(filepath.Dir(projectSource), 0o755); err != nil {
		t.Fatalf("mkdir project source dir: %v", err)
	}
	if err := os.WriteFile(projectSource, []byte("source"), 0o644); err != nil {
		t.Fatalf("write project source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "go.mod"), []byte("module github.com/m0n0x41d/haft\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	got, err := findTUIEntry(projectRoot)
	if err != nil {
		t.Fatalf("findTUIEntry returned error: %v", err)
	}
	if got != projectSource {
		t.Fatalf("findTUIEntry = %q, want %q", got, projectSource)
	}
}
