//go:build darwin || linux

package profileprojection

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicProjectionWriteCreatesReplacesAndRereadsExactBytes(t *testing.T) {
	root := t.TempDir()
	directory, err := openProjectionDirectory(root)
	if err != nil {
		t.Fatalf("open projection directory: %v", err)
	}
	t.Cleanup(func() { _ = directory.Close() })
	first := []byte("schema: first\n")
	if err := directory.writeAtomic(first, "stage-first"); err != nil {
		t.Fatalf("write first projection: %v", err)
	}
	observed, err := directory.readTarget()
	if err != nil {
		t.Fatalf("read first projection: %v", err)
	}
	if !bytes.Equal(observed, first) {
		t.Fatalf("first bytes = %q", observed)
	}
	second := []byte("schema: second\n")
	if err := directory.writeAtomic(second, "stage-second"); err != nil {
		t.Fatalf("replace projection: %v", err)
	}
	observed, err = directory.readTarget()
	if err != nil {
		t.Fatalf("read second projection: %v", err)
	}
	if !bytes.Equal(observed, second) {
		t.Fatalf("second bytes = %q", observed)
	}
	matches, err := filepath.Glob(filepath.Join(root, ".haft", ".project-profile.yaml.*.tmp"))
	if err != nil {
		t.Fatalf("glob stages: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary projection stages remain: %v", matches)
	}
}

func TestAtomicProjectionWriteRejectsSymlinkedHaftDirectory(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".haft")); err != nil {
		t.Fatalf("create .haft symlink: %v", err)
	}
	_, err := openProjectionDirectory(root)
	if err == nil {
		t.Fatal("projection boundary followed a symlinked .haft directory")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "project-profile.yaml")); !os.IsNotExist(statErr) {
		t.Fatalf("writer escaped through symlink: %v", statErr)
	}
}

func TestHeldDirectoryIdentityPreventsParentSwapWriteEscape(t *testing.T) {
	root := t.TempDir()
	directory, err := openProjectionDirectory(root)
	if err != nil {
		t.Fatalf("open projection directory: %v", err)
	}
	t.Cleanup(func() { _ = directory.Close() })
	heldPath := filepath.Join(root, ".haft-held")
	if err := os.Rename(filepath.Join(root, ".haft"), heldPath); err != nil {
		t.Fatalf("move held .haft directory: %v", err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".haft")); err != nil {
		t.Fatalf("replace .haft with symlink: %v", err)
	}
	err = directory.writeAtomic([]byte("must-not-escape\n"), "stage-parent-swap")
	if err == nil {
		t.Fatal("writer accepted a changed .haft parent identity")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "project-profile.yaml")); !os.IsNotExist(statErr) {
		t.Fatalf("writer escaped through swapped parent: %v", statErr)
	}
}

func TestStageReconciliationRemovesOnlyHeldRegularStages(t *testing.T) {
	root := t.TempDir()
	directory, err := openProjectionDirectory(root)
	if err != nil {
		t.Fatalf("open projection directory: %v", err)
	}
	t.Cleanup(func() { _ = directory.Close() })
	stageName := projectionStagePrefix + "crashed" + projectionStageSuffix
	stagePath := filepath.Join(root, ".haft", stageName)
	if err := os.WriteFile(stagePath, []byte("stale projection data\n"), 0o644); err != nil {
		t.Fatalf("write stale stage: %v", err)
	}
	if err := directory.reconcileStages(); err != nil {
		t.Fatalf("reconcile stale stages: %v", err)
	}
	if _, err := os.Stat(stagePath); !os.IsNotExist(err) {
		t.Fatalf("stale stage remains: %v", err)
	}
}
