package typedmemorystore

import (
	"context"
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type overlaySnapshotLoader struct {
	snapshot CurrentProjectSnapshot
	err      error
}

func (loader *overlaySnapshotLoader) LoadCurrentProjectSnapshot(
	_ context.Context,
	_ projectledger.ProjectID,
) (CurrentProjectSnapshot, error) {
	return loader.snapshot, loader.err
}

type overlayInputProvider struct {
	blobs []ObservableInputBlob
	err   error
}

func (provider *overlayInputProvider) LoadSnapshotObservableInputs(
	_ context.Context,
	_ projectledger.ProjectID,
) ([]ObservableInputBlob, error) {
	return cloneObservableInputBlobs(provider.blobs), provider.err
}

func TestObservableInputOverlayLoaderAugmentsCopiedEvaluatorCatalogOnly(
	t *testing.T,
) {
	project := overlayProject(t, "qnt_a7f3b2c1")
	durable := overlayBlob(t, "observable:durable", []byte("durable"))
	staged := overlayBlob(t, "observable:staged", []byte("staged"))
	baseCatalog, err := newImmutableObservableInputCatalog(
		[]ObservableInputBlob{durable},
	)
	if err != nil {
		t.Fatalf("newImmutableObservableInputCatalog() error = %v", err)
	}
	baseMemory := &currentMemorySnapshot{
		project:         project,
		memberOfSources: baseCatalog,
	}
	base := CurrentProjectSnapshot{
		project:  project,
		snapshot: baseMemory,
	}
	provider := &overlayInputProvider{blobs: []ObservableInputBlob{staged}}
	loader, err := NewCurrentProjectSnapshotLoaderWithObservableInputOverlay(
		&overlaySnapshotLoader{snapshot: base},
		provider,
	)
	if err != nil {
		t.Fatalf("NewCurrentProjectSnapshotLoaderWithObservableInputOverlay() error = %v", err)
	}

	loaded, err := loader.LoadCurrentProjectSnapshot(context.Background(), project)
	if err != nil {
		t.Fatalf("LoadCurrentProjectSnapshot() error = %v", err)
	}
	loadedMemory, ok := loaded.Snapshot().(*currentMemorySnapshot)
	if !ok {
		t.Fatalf("loaded snapshot = %T, want store-owned snapshot", loaded.Snapshot())
	}
	if loadedMemory == baseMemory {
		t.Fatal("observable overlay mutated the base snapshot in place")
	}
	if baseMemory.memberOfSources.Len() != 1 {
		t.Fatalf("base catalog size = %d, want 1", baseMemory.memberOfSources.Len())
	}
	if loadedMemory.memberOfSources.Len() != 2 {
		t.Fatalf("overlaid catalog size = %d, want 2", loadedMemory.memberOfSources.Len())
	}
	if !loadedMemory.memberOfSources.ContainsAll(
		[]ObservableInputBlob{durable, staged},
	) {
		t.Fatal("overlaid catalog lost durable or staged observable bytes")
	}

	provider.blobs[0] = overlayBlob(
		t,
		"observable:replacement",
		[]byte("replacement"),
	)
	if !loadedMemory.memberOfSources.ContainsAll([]ObservableInputBlob{staged}) {
		t.Fatal("provider mutation changed the loaded immutable overlay")
	}
}

func TestObservableInputOverlayLoaderRejectsEmptyConflictingAndForeignBasis(
	t *testing.T,
) {
	project := overlayProject(t, "qnt_a7f3b2c1")
	foreign := overlayProject(t, "qnt_1234abcd")
	baseBlob := overlayBlob(t, "observable:shared", []byte("base"))
	baseCatalog, err := newImmutableObservableInputCatalog(
		[]ObservableInputBlob{baseBlob},
	)
	if err != nil {
		t.Fatalf("newImmutableObservableInputCatalog() error = %v", err)
	}
	base := CurrentProjectSnapshot{
		project: project,
		snapshot: &currentMemorySnapshot{
			project:         project,
			memberOfSources: baseCatalog,
		},
	}

	tests := []struct {
		name     string
		base     CurrentProjectSnapshot
		provider *overlayInputProvider
		target   error
	}{
		{
			name:     "empty overlay",
			base:     base,
			provider: &overlayInputProvider{},
			target:   ErrObservableInputOverlayRequired,
		},
		{
			name: "conflicting digest for reference",
			base: base,
			provider: &overlayInputProvider{blobs: []ObservableInputBlob{
				overlayBlob(t, "observable:shared", []byte("substitute")),
			}},
		},
		{
			name: "foreign base snapshot",
			base: CurrentProjectSnapshot{
				project:  foreign,
				snapshot: base.snapshot,
			},
			provider: &overlayInputProvider{blobs: []ObservableInputBlob{
				overlayBlob(t, "observable:staged", []byte("staged")),
			}},
			target: ErrObservableInputOverlayUnsupported,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			loader, constructErr := NewCurrentProjectSnapshotLoaderWithObservableInputOverlay(
				&overlaySnapshotLoader{snapshot: test.base},
				test.provider,
			)
			if constructErr != nil {
				t.Fatalf("construct overlay loader: %v", constructErr)
			}
			_, loadErr := loader.LoadCurrentProjectSnapshot(
				context.Background(),
				project,
			)
			if loadErr == nil {
				t.Fatal("overlay load succeeded, want fail-closed rejection")
			}
			if test.target != nil && !errors.Is(loadErr, test.target) {
				t.Fatalf("overlay load error = %v, want %v", loadErr, test.target)
			}
		})
	}
}

func TestObservableInputOverlayLoaderRejectsMissingDependenciesAndCancellation(
	t *testing.T,
) {
	provider := &overlayInputProvider{}
	if _, err := NewCurrentProjectSnapshotLoaderWithObservableInputOverlay(
		nil,
		provider,
	); err == nil {
		t.Fatal("nil base loader was accepted")
	}
	base := &overlaySnapshotLoader{}
	if _, err := NewCurrentProjectSnapshotLoaderWithObservableInputOverlay(
		base,
		nil,
	); !errors.Is(err, ErrObservableInputOverlayRequired) {
		t.Fatalf("nil overlay error = %v", err)
	}

	project := overlayProject(t, "qnt_a7f3b2c1")
	loader, err := NewCurrentProjectSnapshotLoaderWithObservableInputOverlay(
		base,
		provider,
	)
	if err != nil {
		t.Fatalf("construct cancellation fixture: %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = loader.LoadCurrentProjectSnapshot(canceled, project)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled overlay load error = %v", err)
	}
}

func overlayProject(t *testing.T, raw string) projectledger.ProjectID {
	t.Helper()
	project, err := projectledger.ParseProjectID(raw)
	if err != nil {
		t.Fatalf("ParseProjectID(%q): %v", raw, err)
	}
	return project
}

func overlayBlob(
	t *testing.T,
	referenceText string,
	content []byte,
) ObservableInputBlob {
	t.Helper()
	reference, err := typedmemory.NewObservableInputRef(referenceText)
	if err != nil {
		t.Fatalf("NewObservableInputRef(%q): %v", referenceText, err)
	}
	digest, err := digestBytes(content)
	if err != nil {
		t.Fatalf("digest observable bytes: %v", err)
	}
	blob, err := NewObservableInputBlob(reference, digest, content)
	if err != nil {
		t.Fatalf("NewObservableInputBlob(%q): %v", referenceText, err)
	}
	return blob
}
