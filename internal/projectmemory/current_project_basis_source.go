package projectmemory

import (
	"context"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemoryvalidation"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

var ErrCurrentProjectSnapshotLoaderMissing = errors.New(
	"project-memory current snapshot loader is missing",
)

// CurrentProjectBasisSource joins one immutable storage snapshot with its
// exact TypeEnv. It is deliberately read-only and cannot admit or activate
// state. The supplied loader may own SQLite; this source owns no database.
type CurrentProjectBasisSource struct {
	loader typedmemorystore.CurrentProjectSnapshotLoader
}

var _ ProjectBasisSource = (*CurrentProjectBasisSource)(nil)

func NewCurrentProjectBasisSource(
	loader typedmemorystore.CurrentProjectSnapshotLoader,
) (*CurrentProjectBasisSource, error) {
	if !interfaceValuePresent(loader) {
		return nil, ErrCurrentProjectSnapshotLoaderMissing
	}
	return &CurrentProjectBasisSource{loader: loader}, nil
}

func (source *CurrentProjectBasisSource) ResolveProjectBasis(
	ctx context.Context,
	projectID projectledger.ProjectID,
	selector typedmemorywire.BasisSelector,
) (typedmemoryvalidation.BasisResolution, error) {
	if source == nil || !interfaceValuePresent(source.loader) {
		return nil, ErrCurrentProjectSnapshotLoaderMissing
	}
	if projectID.String() == "" {
		return nil, ErrProjectIdentityMissing
	}
	if err := validationContextError(ctx); err != nil {
		return nil, err
	}
	if err := requireProjectSelector(selector); err != nil {
		return nil, err
	}

	current, err := source.loader.LoadCurrentProjectSnapshot(ctx, projectID)
	if contextErr := validationContextError(ctx); contextErr != nil {
		return nil, contextErr
	}
	if errors.Is(err, typedmemorystore.ErrProjectNotInitialized) {
		return typedmemoryvalidation.NewProjectBasisUnavailable(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("load current project-memory snapshot: %w", err)
	}
	if current.ProjectID() != projectID {
		return nil, fmt.Errorf(
			"%w: loaded snapshot project %q differs from requested project %q",
			ErrProjectBasisUncorrelated,
			current.ProjectID().String(),
			projectID.String(),
		)
	}

	return resolveLoadedProjectBasis(current, selector)
}

func resolveLoadedProjectBasis(
	current typedmemorystore.CurrentProjectSnapshot,
	selector typedmemorywire.BasisSelector,
) (typedmemoryvalidation.BasisResolution, error) {
	environment := current.Environment()
	codecs := current.Codecs()
	snapshot := current.Snapshot()

	switch requested := selector.(type) {
	case typedmemorywire.ProjectCurrentSelector:
		return typedmemoryvalidation.NewResolvedProjectBasis(
			environment,
			codecs,
			snapshot,
		)
	case typedmemorywire.ExactProjectSelector:
		return resolveExactLoadedProjectBasis(
			environment,
			codecs,
			snapshot,
			requested,
		)
	default:
		return nil, ErrProjectBasisUnsupported
	}
}

func resolveExactLoadedProjectBasis(
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	snapshot typedmemory.MemorySnapshot,
	requested typedmemorywire.ExactProjectSelector,
) (typedmemoryvalidation.BasisResolution, error) {
	observedTypeEnv := environment.Ref()
	observedRevision := snapshotGraphRevision(snapshot)
	requestedDigest := requested.RequestedTypeEnvDigest()
	requestedRevision := requested.RequestedGraphRevision()

	digestMatches := observedTypeEnv.Digest() == requestedDigest
	revisionMatches := observedRevision == requestedRevision
	if digestMatches && revisionMatches {
		return typedmemoryvalidation.NewResolvedProjectBasis(
			environment,
			codecs,
			snapshot,
		)
	}

	return typedmemoryvalidation.NewExactProjectBasisMismatch(
		observedTypeEnv,
		observedRevision,
	)
}

func snapshotGraphRevision(
	snapshot typedmemory.MemorySnapshot,
) typedmemory.GraphRevision {
	if !interfaceValuePresent(snapshot) {
		return typedmemory.GraphRevision{}
	}
	return snapshot.GraphRevision()
}
