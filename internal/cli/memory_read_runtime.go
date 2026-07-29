package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectmemory/memoryresolve"
	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/projectmemory/scopedrecall"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func (runtime projectMemoryReadRuntime) ResolveRead(
	ctx context.Context,
	wire typedmemorywire.ResolveReadRequest,
) (memoryresolve.EntityResolutionResult, error) {
	if !typedmemorywire.IsDecodedResolveReadRequest(wire) {
		return nil, fmt.Errorf(
			"decoded EntityOfConcern resolution request is required",
		)
	}
	snapshot, err := runtime.readSnapshot(ctx, wire.Basis())
	if err != nil {
		return nil, err
	}
	query, err := memoryresolve.NewResolutionQuery(wire.Query())
	if err != nil {
		return nil, fmt.Errorf(
			"construct EntityOfConcern resolution query: %w",
			err,
		)
	}
	queryContext, err := resolutionQueryContext(wire)
	if err != nil {
		return nil, err
	}
	request, err := memoryresolve.NewResolutionRequest(
		query,
		queryContext,
		snapshot,
		wire.MaxCandidates(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"construct EntityOfConcern resolution request: %w",
			err,
		)
	}
	return runtime.read.Resolve(ctx, request)
}

type projectMemoryReadRecovery struct {
	ContractVersion string                        `json:"contract_version"`
	Action          string                        `json:"action"`
	Mode            string                        `json:"mode,omitempty"`
	ResultKind      string                        `json:"result_kind"`
	Performed       bool                          `json:"performed"`
	Detail          string                        `json:"detail"`
	RecoveryCall    projectMemoryReadRecoveryCall `json:"recovery_call"`
}

type projectMemoryReadRecoveryCall struct {
	Tool      string                               `json:"tool"`
	Arguments projectMemoryReadRecoveryCallRequest `json:"arguments"`
}

type projectMemoryReadRecoveryCallRequest struct {
	Action string `json:"action"`
}

func projectMemoryUnavailableReadResponse(
	mode string,
) ([]byte, error) {
	return projectMemoryRecoveryResponse(
		typedmemorywire.ContractVersion,
		typedmemorywire.QueryActionMemory,
		mode,
		"project_basis_unavailable",
		"Structured project memory is not enabled for this project; "+
			"no memory read or write was performed.",
	)
}

func projectMemoryRestartRequiredResponse(
	contractVersion string,
	action string,
	mode string,
) ([]byte, error) {
	return projectMemoryRecoveryResponse(
		contractVersion,
		action,
		mode,
		"restart_required",
		"Structured project memory became enabled after this MCP process "+
			"started. This stale process performed no memory operation; "+
			"reconnect the host and retry the unchanged request.",
	)
}

func projectMemoryRecoveryResponse(
	contractVersion string,
	action string,
	mode string,
	resultKind string,
	detail string,
) ([]byte, error) {
	response := projectMemoryReadRecovery{
		ContractVersion: contractVersion,
		Action:          action,
		Mode:            mode,
		ResultKind:      resultKind,
		Performed:       false,
		Detail:          detail,
		RecoveryCall: projectMemoryReadRecoveryCall{
			Tool: "haft_onboard",
			Arguments: projectMemoryReadRecoveryCallRequest{
				Action: "status",
			},
		},
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf(
			"encode project-memory recovery response: %w",
			err,
		)
	}
	return append(encoded, '\n'), nil
}

func (runtime projectMemoryReadRuntime) NeighborhoodRead(
	ctx context.Context,
	wire typedmemorywire.NeighborhoodReadRequest,
) (neighborhood.NeighborhoodResult, error) {
	if !typedmemorywire.IsDecodedNeighborhoodReadRequest(wire) {
		return nil, fmt.Errorf(
			"decoded exact memory-neighborhood request is required",
		)
	}
	snapshot, err := runtime.readSnapshot(ctx, wire.Basis())
	if err != nil {
		return nil, err
	}
	request, err := neighborhoodRequestFromWire(
		wire.Entity(),
		wire.Context(),
		wire.View(),
		wire.ReadBudget(),
		snapshot,
	)
	if err != nil {
		return nil, err
	}
	return runtime.read.Neighborhood(ctx, request)
}

func (runtime projectMemoryReadRuntime) RecallRead(
	ctx context.Context,
	wire typedmemorywire.RecallReadRequest,
) (scopedrecall.ScopedRecallResult, error) {
	if !typedmemorywire.IsDecodedRecallReadRequest(wire) {
		return nil, fmt.Errorf(
			"decoded scoped memory-recall request is required",
		)
	}
	snapshot, err := runtime.readSnapshot(ctx, wire.Basis())
	if err != nil {
		return nil, err
	}
	request, err := neighborhoodRequestFromWire(
		wire.Entity(),
		wire.Context(),
		wire.View(),
		wire.ReadBudget(),
		snapshot,
	)
	if err != nil {
		return nil, err
	}
	query, err := scopedrecall.NewRecallQuery(wire.Query())
	if err != nil {
		return nil, fmt.Errorf(
			"construct scoped memory-recall query: %w",
			err,
		)
	}
	candidateBudget, err := scopedrecall.NewCandidateBudget(
		wire.CandidateBudget(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"construct scoped memory-recall budget: %w",
			err,
		)
	}
	return runtime.read.Recall(
		ctx,
		request,
		query,
		candidateBudget,
	)
}

func (runtime projectMemoryReadRuntime) readSnapshot(
	ctx context.Context,
	basis typedmemorywire.BasisSelector,
) (neighborhood.SnapshotBasis, error) {
	switch exact := basis.(type) {
	case typedmemorywire.ProjectCurrentSelector:
		return runtime.read.CurrentSnapshotBasis(ctx)
	case typedmemorywire.ExactProjectSelector:
		typeEnv, err := typedmemory.NewTypeEnvRef(
			exact.RequestedTypeEnvDigest(),
		)
		if err != nil {
			return neighborhood.SnapshotBasis{}, fmt.Errorf(
				"construct exact requested TypeEnv: %w",
				err,
			)
		}
		snapshot, err := neighborhood.NewSnapshotBasis(
			exact.RequestedGraphRevision(),
			typeEnv,
			exact.RequestedTypeEnvDigest(),
		)
		if err != nil {
			return neighborhood.SnapshotBasis{}, fmt.Errorf(
				"construct exact requested memory snapshot: %w",
				err,
			)
		}
		return snapshot, nil
	default:
		return neighborhood.SnapshotBasis{}, fmt.Errorf(
			"memory read requires project_current or exact_project basis",
		)
	}
}

func resolutionQueryContext(
	wire typedmemorywire.ResolveReadRequest,
) (memoryresolve.QueryContext, error) {
	context, exact := wire.Context()
	if !exact {
		return memoryresolve.AnyContext{}, nil
	}
	queryContext, err := memoryresolve.NewExactContext(context)
	if err != nil {
		return nil, fmt.Errorf(
			"construct exact resolution context: %w",
			err,
		)
	}
	return queryContext, nil
}

func neighborhoodRequestFromWire(
	entityWire typedmemorywire.ReadEntityReference,
	context typedmemory.BoundedContextRef,
	viewWire typedmemorywire.ReadViewSpec,
	budgetWire typedmemorywire.DimensionedReadBudget,
	snapshot neighborhood.SnapshotBasis,
) (neighborhood.NeighborhoodRequest, error) {
	entity, err := memoryReadEntityReference(entityWire, snapshot.TypeEnv())
	if err != nil {
		return neighborhood.NeighborhoodRequest{}, err
	}
	view, err := memoryReadView(viewWire)
	if err != nil {
		return neighborhood.NeighborhoodRequest{}, err
	}
	budget, err := memoryReadBudget(budgetWire)
	if err != nil {
		return neighborhood.NeighborhoodRequest{}, err
	}
	request, err := neighborhood.NewNeighborhoodRequestBuilder().
		SetEntity(entity).
		SetContext(context).
		SetTypeEnv(snapshot.TypeEnv()).
		SetGraphRevision(snapshot.GraphRevision()).
		SetView(view).
		SetBudget(budget).
		Build()
	if err != nil {
		return neighborhood.NeighborhoodRequest{}, fmt.Errorf(
			"construct exact memory-neighborhood request: %w",
			err,
		)
	}
	return request, nil
}

func memoryReadEntityReference(
	wire typedmemorywire.ReadEntityReference,
	typeEnv typedmemory.TypeEnvRef,
) (typedmemory.PersistedRef, error) {
	refKind, err := typedmemory.NewRefKindRef(
		typeEnv,
		wire.RefKindID(),
	)
	if err != nil {
		return typedmemory.PersistedRef{}, fmt.Errorf(
			"construct memory-read RefKind: %w",
			err,
		)
	}
	reference, err := typedmemory.NewPersistedRef(
		refKind,
		wire.ReferenceID(),
	)
	if err != nil {
		return typedmemory.PersistedRef{}, fmt.Errorf(
			"construct exact memory entity reference: %w",
			err,
		)
	}
	return reference, nil
}

func memoryReadView(
	wire typedmemorywire.ReadViewSpec,
) (neighborhood.NeighborhoodViewSpec, error) {
	profile, err := neighborhood.ParseProjectionProfileRef(
		wire.ProjectionProfileRef(),
	)
	if err != nil {
		return neighborhood.NeighborhoodViewSpec{}, fmt.Errorf(
			"parse memory projection profile: %w",
			err,
		)
	}
	rawFacets := wire.RequestedFacets()
	facets := make([]neighborhood.FacetKind, 0, len(rawFacets))
	for _, raw := range rawFacets {
		facet := neighborhood.FacetKind(raw)
		if !facet.Valid() {
			return neighborhood.NeighborhoodViewSpec{}, fmt.Errorf(
				"memory projection facet %q is unknown",
				raw,
			)
		}
		facets = append(facets, facet)
	}
	detail := neighborhood.DetailLevel(wire.Detail())
	if !detail.Valid() {
		return neighborhood.NeighborhoodViewSpec{}, fmt.Errorf(
			"memory projection detail %q is unknown",
			wire.Detail(),
		)
	}
	view, err := neighborhood.NewNeighborhoodViewSpec(
		profile,
		facets,
		detail,
		wire.IncludeHistory(),
	)
	if err != nil {
		return neighborhood.NeighborhoodViewSpec{}, fmt.Errorf(
			"construct memory projection view: %w",
			err,
		)
	}
	return view, nil
}

func memoryReadBudget(
	wire typedmemorywire.DimensionedReadBudget,
) (neighborhood.DimensionedReadBudget, error) {
	budget, err := neighborhood.NewReadBudgetBuilder().
		SetMaxFacets(wire.MaxFacets()).
		SetMaxItemsPerFacet(wire.MaxItemsPerFacet()).
		SetMaxRelationPathsPerItem(wire.MaxRelationPathsPerItem()).
		SetMaxCarrierExcerptCharacters(
			wire.MaxCarrierExcerptCharacters(),
		).
		SetMaxProvenanceDepth(wire.MaxProvenanceDepth()).
		Build()
	if err != nil {
		return neighborhood.DimensionedReadBudget{}, fmt.Errorf(
			"construct dimensioned memory-read budget: %w",
			err,
		)
	}
	return budget, nil
}
