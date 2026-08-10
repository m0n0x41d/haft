//go:build darwin || linux

package projectledgermigration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPublishServeMigrationSnapshotNeverReplacesExistingBackup(
	t *testing.T,
) {
	directory := t.TempDir()
	partialPath := filepath.Join(directory, "snapshot.partial")
	finalPath := filepath.Join(directory, "snapshot.bak")
	if err := os.WriteFile(partialPath, []byte("new snapshot"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(finalPath, []byte("existing backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := publishServeMigrationSnapshot(partialPath, finalPath); err == nil {
		t.Fatal("snapshot publication replaced an existing backup")
	}
	assertSnapshotPublicationContent(t, partialPath, "new snapshot")
	assertSnapshotPublicationContent(t, finalPath, "existing backup")
}

func assertSnapshotPublicationContent(
	t *testing.T,
	path string,
	want string,
) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != want {
		t.Fatalf("snapshot publication content = %q, want %q", content, want)
	}
}
