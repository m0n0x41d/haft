package projectmemory

import (
	"context"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectmemory/identityreconciliation"
	"github.com/m0n0x41d/haft/internal/projectmemory/memoryresolve"
	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhoodcache"
	"github.com/m0n0x41d/haft/internal/projectmemory/scopedrecall"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

var ErrCurrentProjectReadFrameLoaderMissing = errors.New(
	"project-memory current read-frame loader is missing",
)

var ErrCommittedIdentityResolutionSourceMissing = errors.New(
	"project-memory committed identity-resolution source is missing",
)

// CurrentMemoryReadRuntime is the read-only selected-project shell for exact
// EntityOfConcern resolution and neighborhood projection. It owns no database,
// cache, mutation, Stage, TypeEnv-head selection, or authority capability.
type CurrentMemoryReadRuntime struct {
	project           projectledger.ProjectID
	loader            typedmemorystore.CurrentProjectReadFrameLoader
	neighborhoodCache neighborhoodcache.Shell
	identitySource    identityreconciliation.CommittedResolutionStateSource
}

// NewCurrentMemoryReadRuntime is the history-free compatibility constructor.
// Resolve uses only the immutable entity directory and makes no claim that a
// committed identity-reconciliation ledger was inspected. Public composition
// that can observe reviewed merge/split events must use
// NewCurrentMemoryReadRuntimeWithIdentityReconciliation.
func NewCurrentMemoryReadRuntime(
	project projectledger.ProjectID,
	loader typedmemorystore.CurrentProjectReadFrameLoader,
) (CurrentMemoryReadRuntime, error) {
	return NewCurrentMemoryReadRuntimeWithNeighborhoodCache(
		project,
		loader,
		neighborhoodcache.NewUnavailableShell(),
	)
}

// NewCurrentMemoryReadRuntimeWithIdentityReconciliation constructs the strict
// public read path. Absence of a source is an error and can never be
// interpreted as an empty reconciliation ledger.
func NewCurrentMemoryReadRuntimeWithIdentityReconciliation(
	project projectledger.ProjectID,
	loader typedmemorystore.CurrentProjectReadFrameLoader,
	source identityreconciliation.CommittedResolutionStateSource,
) (CurrentMemoryReadRuntime, error) {
	return NewCurrentMemoryReadRuntimeWithNeighborhoodCacheAndIdentityReconciliation(
		project,
		loader,
		neighborhoodcache.NewUnavailableShell(),
		source,
	)
}

func NewCurrentMemoryReadRuntimeWithNeighborhoodCache(
	project projectledger.ProjectID,
	loader typedmemorystore.CurrentProjectReadFrameLoader,
	cache neighborhoodcache.Shell,
) (CurrentMemoryReadRuntime, error) {
	canonical, err := projectledger.ParseProjectID(project.String())
	if err != nil || canonical != project {
		return CurrentMemoryReadRuntime{}, ErrProjectIdentityMissing
	}
	if !interfaceValuePresent(loader) {
		return CurrentMemoryReadRuntime{},
			ErrCurrentProjectReadFrameLoaderMissing
	}
	return CurrentMemoryReadRuntime{
		project:           project,
		loader:            loader,
		neighborhoodCache: cache,
	}, nil
}

func NewCurrentMemoryReadRuntimeWithNeighborhoodCacheAndIdentityReconciliation(
	project projectledger.ProjectID,
	loader typedmemorystore.CurrentProjectReadFrameLoader,
	cache neighborhoodcache.Shell,
	source identityreconciliation.CommittedResolutionStateSource,
) (CurrentMemoryReadRuntime, error) {
	if !interfaceValuePresent(source) {
		return CurrentMemoryReadRuntime{},
			ErrCommittedIdentityResolutionSourceMissing
	}
	runtime, err := NewCurrentMemoryReadRuntimeWithNeighborhoodCache(
		project,
		loader,
		cache,
	)
	if err != nil {
		return CurrentMemoryReadRuntime{}, err
	}
	runtime.identitySource = source
	return runtime, nil
}

// CurrentSnapshotBasis returns only the exact read coordinate observed by one
// current-project frame load. It does not retain the frame, open a write path,
// or make a caller-supplied basis current.
func (runtime CurrentMemoryReadRuntime) CurrentSnapshotBasis(
	ctx context.Context,
) (neighborhood.SnapshotBasis, error) {
	frame, err := runtime.loadFrame(ctx)
	if err != nil {
		return neighborhood.SnapshotBasis{}, err
	}
	basis, err := currentResolutionSnapshotBasis(frame.EntityDirectory())
	if err != nil {
		return neighborhood.SnapshotBasis{}, err
	}
	return basis, nil
}

func (runtime CurrentMemoryReadRuntime) Resolve(
	ctx context.Context,
	request memoryresolve.ResolutionRequest,
) (memoryresolve.EntityResolutionResult, error) {
	frame, err := runtime.loadFrame(ctx)
	if err != nil {
		return nil, err
	}
	index, err := BuildCurrentResolutionIndex(frame)
	if err != nil {
		return nil, err
	}
	result, err := runtime.resolveFrame(ctx, request, frame, index)
	if err != nil {
		return nil, fmt.Errorf(
			"resolve current project EntityOfConcern: %w",
			err,
		)
	}
	return result, nil
}

func (runtime CurrentMemoryReadRuntime) resolveFrame(
	ctx context.Context,
	request memoryresolve.ResolutionRequest,
	frame typedmemorystore.CurrentProjectReadFrame,
	index memoryresolve.ResolutionIndex,
) (memoryresolve.EntityResolutionResult, error) {
	if !interfaceValuePresent(runtime.identitySource) {
		return memoryresolve.Resolve(request, index)
	}
	directory := frame.EntityDirectory()
	basis, err := identityreconciliation.NewCommittedResolutionBasis(
		directory.ProjectID(),
		directory.GraphSnapshotBasis().GraphRevision(),
		directory.ActiveTypeEnv(),
	)
	if err != nil {
		return nil, err
	}
	state, err := runtime.identitySource.LoadCommittedResolutionState(
		ctx,
		basis,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"load current reviewed identity resolution: %w",
			err,
		)
	}
	reviewed, err := BuildCurrentReviewedIdentityIndex(frame, state)
	if err != nil {
		return nil, err
	}
	return memoryresolve.ResolveWithReviewedIdentity(
		request,
		index,
		reviewed,
	)
}

func (runtime CurrentMemoryReadRuntime) Neighborhood(
	ctx context.Context,
	request neighborhood.NeighborhoodRequest,
) (neighborhood.NeighborhoodResult, error) {
	if !request.Valid() {
		return nil, fmt.Errorf(
			"current project neighborhood request is invalid",
		)
	}
	frame, err := runtime.loadFrame(ctx)
	if err != nil {
		return nil, err
	}
	required, err := currentResolutionSnapshotBasis(
		frame.EntityDirectory(),
	)
	if err != nil {
		return nil, err
	}
	observed, err := neighborhood.NewSnapshotBasis(
		request.GraphRevision(),
		request.TypeEnv(),
		request.TypeEnv().Digest(),
	)
	if err != nil {
		return nil, err
	}
	if observed != required {
		cause, causeErr := neighborhood.NewStaleSnapshotCause(
			observed,
			required,
		)
		if causeErr != nil {
			return nil, causeErr
		}
		retry, retryErr := neighborhood.NewRetryRequiredResult(
			cause,
			required,
		)
		if retryErr != nil {
			return nil, retryErr
		}
		return retry, nil
	}
	_, found := currentDirectoryEntry(
		frame.EntityDirectory(),
		request.Entity(),
		request.Context(),
	)
	if !found {
		return currentNeighborhoodAbstained(request, required, frame)
	}
	cacheKey, err := currentNeighborhoodCacheKey(
		runtime.project,
		frame,
		request,
	)
	if err != nil {
		return nil, err
	}
	read, err := runtime.neighborhoodCache.ReadThrough(
		ctx,
		cacheKey,
		func(
			context.Context,
		) (neighborhood.ExactNeighborhood, error) {
			input, sourceFound, buildErr :=
				BuildCurrentNeighborhoodInput(frame, request)
			if buildErr != nil {
				return neighborhood.ExactNeighborhood{}, buildErr
			}
			if !sourceFound {
				return neighborhood.ExactNeighborhood{},
					fmt.Errorf(
						"current neighborhood source disappeared inside one read frame",
					)
			}
			result, assembleErr := neighborhood.Assemble(input)
			if assembleErr != nil {
				return neighborhood.ExactNeighborhood{},
					fmt.Errorf(
						"assemble current project neighborhood: %w",
						assembleErr,
					)
			}
			return result, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return read.Result(), nil
}

func currentNeighborhoodCacheKey(
	project projectledger.ProjectID,
	frame typedmemorystore.CurrentProjectReadFrame,
	request neighborhood.NeighborhoodRequest,
) (neighborhoodcache.Key, error) {
	correlated, err := typedmemorystore.NewCurrentProjectReadFrame(
		frame.Snapshot(),
		frame.EntityDirectory(),
		frame.GraphObservation(),
	)
	if err != nil {
		return neighborhoodcache.Key{}, fmt.Errorf(
			"neighborhood cache key requires correlated read frame: %w",
			err,
		)
	}
	graphBasis := correlated.GraphObservation().GraphSnapshotBasis()
	requestMatchesFrame :=
		correlated.Snapshot().ProjectID() == project &&
			correlated.EntityDirectory().ProjectID() == project &&
			graphBasis.Project().String() == project.String() &&
			correlated.Snapshot().Environment().Ref() == request.TypeEnv() &&
			correlated.EntityDirectory().ActiveTypeEnv() == request.TypeEnv() &&
			correlated.GraphObservation().ActiveTypeEnv() == request.TypeEnv() &&
			graphBasis.GraphRevision() == request.GraphRevision() &&
			correlated.EntityDirectory().GraphSnapshotBasis().Ref() ==
				graphBasis.Ref()
	if !request.Valid() || !requestMatchesFrame {
		return neighborhoodcache.Key{}, fmt.Errorf(
			"neighborhood cache key request differs from current read frame",
		)
	}
	profile, found := neighborhood.LookupProjectionProfile(
		request.View().ProfileRef(),
	)
	if !found {
		return neighborhoodcache.Key{},
			fmt.Errorf(
				"current neighborhood projection profile is unavailable",
			)
	}
	projectionSchema, err :=
		neighborhoodcache.NewProjectionSchemaVersion(
			profile.SchemaVersion(),
		)
	if err != nil {
		return neighborhoodcache.Key{}, err
	}
	return neighborhoodcache.NewKey(
		project,
		request,
		correlated.EntityDirectory().Digest(),
		graphBasis,
		projectionSchema,
	)
}

func (runtime CurrentMemoryReadRuntime) Recall(
	ctx context.Context,
	neighborhoodRequest neighborhood.NeighborhoodRequest,
	query scopedrecall.RecallQuery,
	budget scopedrecall.CandidateBudget,
) (scopedrecall.ScopedRecallResult, error) {
	if !neighborhoodRequest.Valid() {
		return nil, fmt.Errorf(
			"current project scoped-recall neighborhood request is invalid",
		)
	}
	scope, err := scopedrecall.NewExactRecallScope(
		neighborhoodRequest.Entity(),
		neighborhoodRequest.Context(),
		neighborhoodRequest.View().ProfileRef(),
	)
	if err != nil {
		return nil, err
	}
	observed, err := neighborhood.NewSnapshotBasis(
		neighborhoodRequest.GraphRevision(),
		neighborhoodRequest.TypeEnv(),
		neighborhoodRequest.TypeEnv().Digest(),
	)
	if err != nil {
		return nil, err
	}
	request, err := scopedrecall.NewScopedRecallRequest(
		scope,
		observed,
		query,
		budget,
	)
	if err != nil {
		return nil, err
	}
	projected, err := runtime.Neighborhood(ctx, neighborhoodRequest)
	if err != nil {
		return nil, err
	}
	switch result := projected.(type) {
	case neighborhood.ExactNeighborhood:
		return recallExactCurrentNeighborhood(request, result)
	case neighborhood.RetryRequiredResult:
		return scopedrecall.NewScopedRetryRequired(request, result)
	case neighborhood.AbstainedResult:
		return recallAbstainedCurrentNeighborhood(request, result)
	default:
		return nil, fmt.Errorf(
			"current project neighborhood returned unsupported result %T",
			projected,
		)
	}
}

func recallExactCurrentNeighborhood(
	request scopedrecall.ScopedRecallRequest,
	exact neighborhood.ExactNeighborhood,
) (scopedrecall.ScopedRecallResult, error) {
	units, err := scopedrecall.BuildRecallUnits(exact)
	if err != nil {
		return nil, err
	}
	corpus, err := scopedrecall.NewScopedCorpus(
		request.Scope(),
		request.SnapshotBasis(),
		units,
	)
	if err != nil {
		return nil, err
	}
	producer := scopedrecall.NewLexicalProducer()
	result, err := producer.Search(request, corpus)
	if err != nil {
		return nil, fmt.Errorf(
			"search current project scoped recall: %w",
			err,
		)
	}
	return result, nil
}

func recallAbstainedCurrentNeighborhood(
	request scopedrecall.ScopedRecallRequest,
	abstained neighborhood.AbstainedResult,
) (scopedrecall.ScopedRecallResult, error) {
	producer := scopedrecall.NewLexicalProducer()
	missing, err := neighborhood.NewMissingBasisRef(
		"current-neighborhood:" + string(abstained.Basis().Kind()),
	)
	if err != nil {
		return nil, err
	}
	basis, err := scopedrecall.NewNoUsableProducerBasis(
		[]scopedrecall.ProducerRef{producer.Ref()},
		missing,
	)
	if err != nil {
		return nil, err
	}
	return scopedrecall.NewScopedRecallAbstained(
		request,
		[]scopedrecall.ProducerRef{producer.Ref()},
		basis,
	)
}

func (runtime CurrentMemoryReadRuntime) loadFrame(
	ctx context.Context,
) (typedmemorystore.CurrentProjectReadFrame, error) {
	if !interfaceValuePresent(runtime.loader) {
		return typedmemorystore.CurrentProjectReadFrame{},
			ErrCurrentProjectReadFrameLoaderMissing
	}
	canonical, err := projectledger.ParseProjectID(runtime.project.String())
	if err != nil || canonical != runtime.project {
		return typedmemorystore.CurrentProjectReadFrame{},
			ErrProjectIdentityMissing
	}
	if err := validationContextError(ctx); err != nil {
		return typedmemorystore.CurrentProjectReadFrame{}, err
	}
	frame, err := runtime.loader.LoadCurrentProjectReadFrame(
		ctx,
		runtime.project,
	)
	if err != nil {
		return typedmemorystore.CurrentProjectReadFrame{}, fmt.Errorf(
			"load current project memory read frame: %w",
			err,
		)
	}
	if err := validationContextError(ctx); err != nil {
		return typedmemorystore.CurrentProjectReadFrame{}, err
	}
	if frame.Snapshot().ProjectID() != runtime.project {
		return typedmemorystore.CurrentProjectReadFrame{},
			ErrProjectBasisUncorrelated
	}
	return frame, nil
}

func currentNeighborhoodAbstained(
	request neighborhood.NeighborhoodRequest,
	snapshot neighborhood.SnapshotBasis,
	frame typedmemorystore.CurrentProjectReadFrame,
) (neighborhood.NeighborhoodResult, error) {
	basis, err := neighborhood.NewEntityOrContextNotFoundBasis(
		request.Entity(),
		request.Context(),
		snapshot,
	)
	if err != nil {
		return nil, err
	}
	directoryRef, err := neighborhood.NewInspectedSourceRef(
		"current-entity-directory:" +
			frame.EntityDirectory().Digest().String(),
	)
	if err != nil {
		return nil, err
	}
	graphRef, err := neighborhood.NewInspectedSourceRef(
		frame.GraphObservation().GraphSnapshotBasis().Ref().String(),
	)
	if err != nil {
		return nil, err
	}
	inspected := []neighborhood.InspectedSourceRef{
		directoryRef,
		graphRef,
	}
	if inspected[0].String() > inspected[1].String() {
		inspected[0], inspected[1] = inspected[1], inspected[0]
	}
	result, err := neighborhood.NewAbstainedResult(
		basis,
		inspected,
	)
	if err != nil {
		return nil, err
	}
	return result, nil
}
