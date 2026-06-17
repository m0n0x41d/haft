package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/project"
)

func TestResolveProjectBindingFromInput_WalksUpAndDerivesDBPath(t *testing.T) {
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	if err := createDirectoryStructure(haftDir); err != nil {
		t.Fatalf("createDirectoryStructure: %v", err)
	}
	cfg, err := project.Create(haftDir, root)
	if err != nil {
		t.Fatalf("project.Create: %v", err)
	}
	dbPath, err := cfg.DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}
	if err := initializeDatabase(dbPath); err != nil {
		t.Fatalf("initializeDatabase: %v", err)
	}

	subdir := filepath.Join(root, "cmd", "tool")
	if err := mkdirAllForBindingTest(subdir); err != nil {
		t.Fatal(err)
	}

	input := projectRootInput{
		Path:   subdir,
		Source: projectRootSourceCWD,
	}
	binding, err := resolveProjectBindingFromInput(input, cfg.ID)
	if err != nil {
		t.Fatalf("resolveProjectBindingFromInput: %v", err)
	}

	if binding.ProjectRoot != root {
		t.Fatalf("ProjectRoot = %q, want %q", binding.ProjectRoot, root)
	}
	if binding.ProjectID != cfg.ID {
		t.Fatalf("ProjectID = %q, want %q", binding.ProjectID, cfg.ID)
	}
	if binding.DBPath != dbPath {
		t.Fatalf("DBPath = %q, want %q", binding.DBPath, dbPath)
	}
	if binding.DBState != "empty_ok_new_project" {
		t.Fatalf("DBState = %q, want empty_ok_new_project", binding.DBState)
	}
	if binding.ArtifactCount != 0 {
		t.Fatalf("ArtifactCount = %d, want 0", binding.ArtifactCount)
	}
}

func TestResolveProjectBindingFromInput_RejectsExpectedProjectIDMismatch(t *testing.T) {
	intendedRoot := t.TempDir()
	intendedHaftDir := filepath.Join(intendedRoot, ".haft")
	if err := createDirectoryStructure(intendedHaftDir); err != nil {
		t.Fatalf("create intended .haft: %v", err)
	}
	intendedCfg, err := project.Create(intendedHaftDir, intendedRoot)
	if err != nil {
		t.Fatalf("project.Create intended: %v", err)
	}

	actualRoot := t.TempDir()
	actualHaftDir := filepath.Join(actualRoot, ".haft")
	if err := createDirectoryStructure(actualHaftDir); err != nil {
		t.Fatalf("create actual .haft: %v", err)
	}
	actualCfg, err := project.Create(actualHaftDir, actualRoot)
	if err != nil {
		t.Fatalf("project.Create actual: %v", err)
	}

	input := projectRootInput{
		Path:   actualRoot,
		Source: projectRootSourceCWD,
	}
	binding, err := resolveProjectBindingFromInput(input, intendedCfg.ID)
	if !errors.Is(err, errExpectedProjectIDMiss) {
		t.Fatalf("err = %v, want errExpectedProjectIDMiss", err)
	}
	if binding.ProjectID != actualCfg.ID {
		t.Fatalf("binding.ProjectID = %q, want actual %q", binding.ProjectID, actualCfg.ID)
	}

	diagnostic := formatProjectBindingDiagnostic(binding)
	for _, want := range []string{intendedCfg.ID, actualCfg.ID, actualRoot} {
		if !strings.Contains(diagnostic, want) {
			t.Fatalf("diagnostic missing %q:\n%s", want, diagnostic)
		}
	}
}

func TestProjectEnvForRoot_AddsExpectedProjectIDWhenAvailable(t *testing.T) {
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	if err := createDirectoryStructure(haftDir); err != nil {
		t.Fatalf("createDirectoryStructure: %v", err)
	}
	cfg, err := project.Create(haftDir, root)
	if err != nil {
		t.Fatalf("project.Create: %v", err)
	}

	env := projectEnvForRoot(root, ".")

	if env[envProjectRoot] != "." {
		t.Fatalf("HAFT_PROJECT_ROOT = %q, want .", env[envProjectRoot])
	}
	if env[envExpectedProjectID] != cfg.ID {
		t.Fatalf("HAFT_EXPECTED_PROJECT_ID = %q, want %q", env[envExpectedProjectID], cfg.ID)
	}
}

func mkdirAllForBindingTest(path string) error {
	return os.MkdirAll(path, 0o755)
}
