package typedmemorystore

import (
	"context"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectledger"
)

var (
	ErrKindClassificationSourceOverlayRequired = errors.New(
		"typed-memory snapshot kind-classification source overlay is required",
	)
	ErrKindClassificationSourceOverlayUnsupported = errors.New(
		"typed-memory kind-classification source overlay requires a store-owned current snapshot",
	)
)

// NewCurrentProjectSnapshotLoaderWithKindClassificationSourceOverlay adds
// request-scoped current delivery sources to an otherwise immutable read. The
// base graph observation, selected TypeEnv, and evaluator registry are not
// replaced.
func NewCurrentProjectSnapshotLoaderWithKindClassificationSourceOverlay(
	base CurrentProjectSnapshotLoader,
	overlay SnapshotKindClassificationSourceOverlay,
) (CurrentProjectSnapshotLoader, error) {
	if !currentProjectSnapshotLoaderIsPresent(base) {
		return nil, ErrCurrentProjectSnapshotLoaderRequired
	}
	if !snapshotKindClassificationSourceOverlayIsPresent(overlay) {
		return nil, ErrKindClassificationSourceOverlayRequired
	}
	return &kindClassificationSourceOverlaySnapshotLoader{
		base:    base,
		overlay: overlay,
	}, nil
}

type kindClassificationSourceOverlaySnapshotLoader struct {
	base    CurrentProjectSnapshotLoader
	overlay SnapshotKindClassificationSourceOverlay
}

func (loader *kindClassificationSourceOverlaySnapshotLoader) LoadCurrentProjectSnapshot(
	ctx context.Context,
	project projectledger.ProjectID,
) (CurrentProjectSnapshot, error) {
	if loader == nil ||
		!currentProjectSnapshotLoaderIsPresent(loader.base) ||
		!snapshotKindClassificationSourceOverlayIsPresent(loader.overlay) {
		return CurrentProjectSnapshot{}, ErrKindClassificationSourceOverlayRequired
	}
	if ctx == nil {
		return CurrentProjectSnapshot{}, fmt.Errorf(
			"load current project snapshot with kind-classification source overlay: context is required",
		)
	}
	if err := ctx.Err(); err != nil {
		return CurrentProjectSnapshot{}, err
	}
	current, err := loader.base.LoadCurrentProjectSnapshot(ctx, project)
	if err != nil {
		return CurrentProjectSnapshot{}, err
	}
	if current.ProjectID() != project {
		return CurrentProjectSnapshot{}, fmt.Errorf(
			"%w: base snapshot project %q differs from requested project %q",
			ErrKindClassificationSourceOverlayUnsupported,
			current.ProjectID().String(),
			project.String(),
		)
	}
	sources, err := loader.overlay.LoadSnapshotKindClassificationSources(
		ctx,
		project,
	)
	if err != nil {
		return CurrentProjectSnapshot{}, fmt.Errorf(
			"load typed-memory snapshot kind-classification source overlay: %w",
			err,
		)
	}
	if err := ctx.Err(); err != nil {
		return CurrentProjectSnapshot{}, err
	}
	return current.withKindClassificationSourceOverlay(sources)
}

func (snapshot CurrentProjectSnapshot) withKindClassificationSourceOverlay(
	sources []KindClassificationSourceBlob,
) (CurrentProjectSnapshot, error) {
	base, ok := snapshot.snapshot.(*currentMemorySnapshot)
	if !ok || base == nil {
		return CurrentProjectSnapshot{}, ErrKindClassificationSourceOverlayUnsupported
	}
	if len(sources) == 0 {
		return CurrentProjectSnapshot{}, ErrKindClassificationSourceOverlayRequired
	}
	merged := base.classificationSources.Blobs()
	merged = append(merged, sources...)
	catalog, err := newImmutableKindClassificationSourceCatalog(merged)
	if err != nil {
		return CurrentProjectSnapshot{}, fmt.Errorf(
			"construct typed-memory snapshot kind-classification source overlay: %w",
			err,
		)
	}
	copy := *base
	copy.classificationSources = catalog
	result := snapshot
	result.snapshot = &copy
	return result, nil
}
