package typedmemorystore

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/m0n0x41d/haft/internal/projectledger"
)

var (
	ErrCurrentProjectSnapshotLoaderRequired = errors.New(
		"typed-memory current-project snapshot loader is required",
	)
	ErrObservableInputOverlayRequired = errors.New(
		"typed-memory snapshot observable-input overlay is required",
	)
	ErrObservableInputOverlayUnsupported = errors.New(
		"typed-memory observable-input overlay requires a store-owned current snapshot",
	)
)

// SnapshotObservableInputOverlay supplies immutable, pre-event observable
// bytes that may participate in read-only validation. It does not assert a
// membership result and does not make the bytes durable. The selected
// evaluator still verifies source format, project/entity/context correlation,
// and registration policy before producing a judgement.
type SnapshotObservableInputOverlay interface {
	LoadSnapshotObservableInputs(
		context.Context,
		projectledger.ProjectID,
	) ([]ObservableInputBlob, error)
}

// NewCurrentProjectSnapshotLoaderWithObservableInputOverlay composes a sealed
// current-project snapshot loader with request-scoped observable bytes. The
// base snapshot remains the sole graph/state observation; only its immutable
// evaluator catalog is augmented for this read. The returned loader has no
// write capability.
func NewCurrentProjectSnapshotLoaderWithObservableInputOverlay(
	base CurrentProjectSnapshotLoader,
	overlay SnapshotObservableInputOverlay,
) (CurrentProjectSnapshotLoader, error) {
	if !currentProjectSnapshotLoaderIsPresent(base) {
		return nil, ErrCurrentProjectSnapshotLoaderRequired
	}
	if !snapshotObservableInputOverlayIsPresent(overlay) {
		return nil, ErrObservableInputOverlayRequired
	}
	return &observableInputOverlaySnapshotLoader{
		base:    base,
		overlay: overlay,
	}, nil
}

type observableInputOverlaySnapshotLoader struct {
	base    CurrentProjectSnapshotLoader
	overlay SnapshotObservableInputOverlay
}

func (loader *observableInputOverlaySnapshotLoader) LoadCurrentProjectSnapshot(
	ctx context.Context,
	project projectledger.ProjectID,
) (CurrentProjectSnapshot, error) {
	if loader == nil ||
		!currentProjectSnapshotLoaderIsPresent(loader.base) ||
		!snapshotObservableInputOverlayIsPresent(loader.overlay) {
		return CurrentProjectSnapshot{}, ErrObservableInputOverlayRequired
	}
	if ctx == nil {
		return CurrentProjectSnapshot{}, fmt.Errorf(
			"load current project snapshot with observable-input overlay: context is required",
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
			ErrObservableInputOverlayUnsupported,
			current.ProjectID().String(),
			project.String(),
		)
	}
	blobs, err := loader.overlay.LoadSnapshotObservableInputs(ctx, project)
	if err != nil {
		return CurrentProjectSnapshot{}, fmt.Errorf(
			"load typed-memory snapshot observable-input overlay: %w",
			err,
		)
	}
	if err := ctx.Err(); err != nil {
		return CurrentProjectSnapshot{}, err
	}
	return current.withObservableInputOverlay(blobs)
}

func (snapshot CurrentProjectSnapshot) withObservableInputOverlay(
	blobs []ObservableInputBlob,
) (CurrentProjectSnapshot, error) {
	base, ok := snapshot.snapshot.(*currentMemorySnapshot)
	if !ok || base == nil {
		return CurrentProjectSnapshot{}, ErrObservableInputOverlayUnsupported
	}
	if len(blobs) == 0 {
		return CurrentProjectSnapshot{}, ErrObservableInputOverlayRequired
	}
	merged := base.memberOfSources.Blobs()
	merged = append(merged, blobs...)
	if err := requireOneObservableDigestPerReference(merged); err != nil {
		return CurrentProjectSnapshot{}, err
	}
	catalog, err := newImmutableObservableInputCatalog(merged)
	if err != nil {
		return CurrentProjectSnapshot{}, fmt.Errorf(
			"construct typed-memory snapshot observable-input overlay: %w",
			err,
		)
	}
	copy := *base
	copy.memberOfSources = catalog
	result := snapshot
	result.snapshot = &copy
	return result, nil
}

func requireOneObservableDigestPerReference(
	blobs []ObservableInputBlob,
) error {
	digests := make(map[string]string, len(blobs))
	for _, blob := range blobs {
		reference := blob.Reference().String()
		digest := blob.Digest().String()
		observed, found := digests[reference]
		if found && observed != digest {
			return fmt.Errorf(
				"typed-memory observable-input reference %q has conflicting exact digests",
				reference,
			)
		}
		digests[reference] = digest
	}
	return nil
}

func currentProjectSnapshotLoaderIsPresent(
	loader CurrentProjectSnapshotLoader,
) bool {
	return interfaceValueIsPresent(loader)
}

func snapshotObservableInputOverlayIsPresent(
	overlay SnapshotObservableInputOverlay,
) bool {
	return interfaceValueIsPresent(overlay)
}

func interfaceValueIsPresent(value any) bool {
	if value == nil {
		return false
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !reflected.IsNil()
	default:
		return true
	}
}
