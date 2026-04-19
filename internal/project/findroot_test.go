package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRootWalksUpward(t *testing.T) {
	tmp := t.TempDir()
	nested := filepath.Join(tmp, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(tmp, HaftDirName), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", HaftDirName, err)
	}

	got, ok := FindRoot(nested)
	if !ok {
		t.Fatalf("FindRoot = (_, false), want project found")
	}
	// On macOS /tmp is a symlink to /private/tmp — normalize both sides.
	wantResolved, _ := filepath.EvalSymlinks(tmp)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != wantResolved {
		t.Fatalf("FindRoot = %q, want %q", gotResolved, wantResolved)
	}
}

func TestFindRootNoMatchReturnsFalse(t *testing.T) {
	tmp := t.TempDir()
	if _, ok := FindRoot(tmp); ok {
		t.Fatalf("expected not found in bare tmp dir")
	}
}

func TestFindRootIgnoresHaftFileNotDirectory(t *testing.T) {
	tmp := t.TempDir()
	// A regular file named ".haft" must not satisfy the marker check.
	if err := os.WriteFile(filepath.Join(tmp, HaftDirName), []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, ok := FindRoot(tmp); ok {
		t.Fatalf("FindRoot matched a regular file named .haft")
	}
}

func TestFindRootFromCwd(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, HaftDirName), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", HaftDirName, err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	got, ok := FindRootFromCwd()
	if !ok {
		t.Fatalf("FindRootFromCwd = (_, false)")
	}
	wantResolved, _ := filepath.EvalSymlinks(tmp)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != wantResolved {
		t.Fatalf("FindRootFromCwd = %q, want %q", gotResolved, wantResolved)
	}
}
