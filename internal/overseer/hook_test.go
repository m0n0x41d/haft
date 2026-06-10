package overseer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallPostCommitHookCreatesSoftIdempotentBlock(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0o755); err != nil {
		t.Fatalf("create hooks dir: %v", err)
	}

	first, err := InstallPostCommitHook(root, "haft")
	if err != nil {
		t.Fatalf("InstallPostCommitHook returned error: %v", err)
	}
	if !first.Installed || first.Skipped {
		t.Fatalf("first install result = %+v, want installed", first)
	}

	hookPath := filepath.Join(root, ".git", "hooks", "post-commit")
	firstContent := readText(t, hookPath)
	if !strings.Contains(firstContent, hookStartMarker) {
		t.Fatalf("hook missing start marker:\n%s", firstContent)
	}
	if !strings.Contains(firstContent, "haft overseer hook --commit HEAD --async || true") {
		t.Fatalf("hook is not soft async e2e:\n%s", firstContent)
	}

	second, err := InstallPostCommitHook(root, "haft")
	if err != nil {
		t.Fatalf("second InstallPostCommitHook returned error: %v", err)
	}
	secondContent := readText(t, hookPath)
	if firstContent != secondContent {
		t.Fatalf("hook install is not idempotent:\nfirst:\n%s\nsecond:\n%s", firstContent, secondContent)
	}
	if !second.Installed || second.Updated {
		t.Fatalf("second install result = %+v, want installed without update", second)
	}
}

func TestInstallPostCommitHookSkipsNonGitProject(t *testing.T) {
	result, err := InstallPostCommitHook(t.TempDir(), "haft")
	if err != nil {
		t.Fatalf("InstallPostCommitHook returned error: %v", err)
	}
	if !result.Skipped {
		t.Fatalf("result = %+v, want skipped", result)
	}
}

func readText(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
