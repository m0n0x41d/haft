package artifact

import "testing"

func TestWarnSharedFiles_ManifestFiles(t *testing.T) {
	paths := []string{"src/auth.go", "composer.json", "src/handler.go"}
	warnings := WarnSharedFiles(paths)

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if warnings[0] != "composer.json: shared file — changes on any dependency update, causing false drift. Link implementation files instead." {
		t.Errorf("unexpected warning: %s", warnings[0])
	}
}

func TestWarnSharedFiles_LockFiles(t *testing.T) {
	paths := []string{"package-lock.json", "go.sum", "yarn.lock"}
	warnings := WarnSharedFiles(paths)

	if len(warnings) != 3 {
		t.Fatalf("expected 3 warnings, got %d", len(warnings))
	}
}

func TestWarnSharedFiles_CleanFiles(t *testing.T) {
	paths := []string{"src/main.go", "internal/auth/handler.go", "README.md"}
	warnings := WarnSharedFiles(paths)

	if len(warnings) != 0 {
		t.Fatalf("expected 0 warnings, got %d: %v", len(warnings), warnings)
	}
}

func TestWarnSharedFiles_NestedPaths(t *testing.T) {
	paths := []string{"tools/snippets-mcp/go.mod", "services/api/package.json"}
	warnings := WarnSharedFiles(paths)

	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings for nested manifest paths, got %d", len(warnings))
	}
}

func TestWarnSharedFiles_EmptyInput(t *testing.T) {
	warnings := WarnSharedFiles(nil)
	if len(warnings) != 0 {
		t.Fatalf("expected 0 warnings for nil input, got %d", len(warnings))
	}

	warnings = WarnSharedFiles([]string{})
	if len(warnings) != 0 {
		t.Fatalf("expected 0 warnings for empty input, got %d", len(warnings))
	}
}

func TestWarnSharedFiles_CaseInsensitive(t *testing.T) {
	paths := []string{"Cargo.lock", "PACKAGE.JSON"}
	warnings := WarnSharedFiles(paths)

	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings (case insensitive), got %d", len(warnings))
	}
}
