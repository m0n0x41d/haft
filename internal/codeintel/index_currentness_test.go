package codeintel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	haftdb "github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestNodeRetriesWhenPublishedBasisChangesDuringQuery(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeCurrentnessSource(
		t,
		root,
		"stable.go",
		"package sample\nfunc Stable() {}\n",
	)
	store, closeStore := newCurrentnessArtifactStore(t, root)
	defer closeStore()
	service := NewService(store)
	first, err := service.Node(ctx, root, "Missing", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if first.Index.Epoch != 1 {
		t.Fatalf("initial node epoch = %d, want 1", first.Index.Epoch)
	}

	hookCalls := 0
	service.beforeBasisConfirm = func(ctx context.Context) error {
		hookCalls++
		if hookCalls > 1 {
			return nil
		}
		writeCurrentnessSource(
			t,
			root,
			"added.go",
			"package sample\nfunc Added() {}\n",
		)
		_, err := service.scanner.RefreshIncremental(ctx, root)
		return err
	}
	view, err := service.Node(ctx, root, "Missing", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if hookCalls != 2 {
		t.Fatalf("basis confirmation calls = %d, want retry then success", hookCalls)
	}
	if view.Index.Epoch != 2 ||
		view.NameResolution.Kind().String() != "seed_not_found" {
		t.Fatalf("retried node view = %+v", view)
	}
}

func TestNodeDoesNotClaimAbsenceUnderBoundedCoverage(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeCurrentnessSource(
		t,
		root,
		"stable.go",
		"package sample\nfunc Stable() {}\n",
	)
	writeCurrentnessSource(
		t,
		root,
		"oversized.go",
		"package sample\n"+strings.Repeat(" ", 500_001),
	)
	store, closeStore := newCurrentnessArtifactStore(t, root)
	defer closeStore()

	view, err := NewService(store).Node(ctx, root, "Missing", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if view.Index.Basis.Coverage.Posture !=
		"bounded_with_exclusions" {
		t.Fatalf("bounded index = %+v", view.Index)
	}
	if view.NameResolution.Kind().String() != "seed_unavailable" ||
		view.NameResolution.DetailCode() != "index_incomplete" {
		t.Fatalf("bounded missing seed = %T/%+v", view.NameResolution, view)
	}
}

func TestNodeReportsUnavailableWhenNoEpochCanBePublished(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, closeStore := newCurrentnessArtifactStore(t, root)
	defer closeStore()

	view, err := NewService(store).Node(
		ctx,
		filepath.Join(root, "missing-project-root"),
		"Missing",
		"",
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	if view.Index.Epoch != 0 ||
		view.NameResolution.Kind().String() != "seed_unavailable" ||
		view.NameResolution.DetailCode() != "index_unavailable" {
		t.Fatalf("unavailable initial index = %+v", view)
	}
}

func newCurrentnessArtifactStore(
	t *testing.T,
	root string,
) (*artifact.Store, func()) {
	t.Helper()
	legacy, err := haftdb.NewStore(filepath.Join(root, "haft.db"))
	if err != nil {
		t.Fatal(err)
	}
	return artifact.NewStore(legacy.GetRawDB()), func() {
		_ = legacy.Close()
	}
}

func writeCurrentnessSource(
	t *testing.T,
	root string,
	rel string,
	content string,
) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
