package project

import (
	"os"
	"path/filepath"
	"strings"
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

func TestProposedProjectConfigIsPureAndPersistsOnlyExactIdentity(
	t *testing.T,
) {
	projectRoot := filepath.Join(t.TempDir(), "new-project")
	haftDir := filepath.Join(projectRoot, ".haft")
	proposal, err := ProposeConfig(projectRoot)
	if err != nil {
		t.Fatalf("ProposeConfig: %v", err)
	}
	if proposal.Name != "new-project" ||
		!strings.HasPrefix(proposal.ID, "qnt_") ||
		len(proposal.ID) != len("qnt_")+8 {
		t.Fatalf("proposal = %#v", proposal)
	}
	if _, err := os.Stat(haftDir); !os.IsNotExist(err) {
		t.Fatalf("proposal mutated project root: %v", err)
	}

	persisted, err := PersistExactConfig(
		haftDir,
		projectRoot,
		proposal,
	)
	if err != nil {
		t.Fatalf("PersistExactConfig: %v", err)
	}
	if *persisted != proposal {
		t.Fatalf("persisted = %#v, want %#v", persisted, proposal)
	}
	again, err := PersistExactConfig(
		haftDir,
		projectRoot,
		proposal,
	)
	if err != nil {
		t.Fatalf("PersistExactConfig repeat: %v", err)
	}
	if *again != proposal {
		t.Fatalf("repeat = %#v, want %#v", again, proposal)
	}

	other := proposal
	other.ID = "qnt_aaaaaaaa"
	if other.ID == proposal.ID {
		other.ID = "qnt_bbbbbbbb"
	}
	_, err = PersistExactConfig(haftDir, projectRoot, other)
	if err == nil || !strings.Contains(
		err.Error(),
		"already carries project identity",
	) {
		t.Fatalf("identity replacement was not rejected: %v", err)
	}
}

func TestCanonicalDBPathDoesNotCreateDirectories(t *testing.T) {
	homeRoot := t.TempDir()
	projectID := "qnt_e3149c17"
	path, err := CanonicalDBPath(homeRoot, projectID)
	if err != nil {
		t.Fatalf("CanonicalDBPath: %v", err)
	}
	want := filepath.Join(
		homeRoot,
		".haft",
		"projects",
		projectID,
		"haft.db",
	)
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("CanonicalDBPath created storage: %v", err)
	}
}
