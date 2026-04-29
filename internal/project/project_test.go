package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateRepairsProjectNameForExistingConfig(t *testing.T) {
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatalf("mkdir .haft: %v", err)
	}

	cfgPath := filepath.Join(haftDir, "project.yaml")
	if err := os.WriteFile(cfgPath, []byte("id: qnt_existing\nname: old-name\n"), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	cfg, err := Create(haftDir, root)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if cfg.ID != "qnt_existing" {
		t.Fatalf("ID = %q, want immutable existing ID", cfg.ID)
	}
	if cfg.Name != filepath.Base(root) {
		t.Fatalf("Name = %q, want %q", cfg.Name, filepath.Base(root))
	}

	reloaded, err := Load(haftDir)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if reloaded.Name != filepath.Base(root) {
		t.Fatalf("persisted Name = %q, want %q", reloaded.Name, filepath.Base(root))
	}
}
