package neighborhood

import (
	"fmt"
	"math"
	"slices"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// NeighborhoodViewSpec names one immutable profile and a non-empty facet
// subset. Requested facets are canonicalized into profile presentation order;
// that order does not become semantic or Work order.
type NeighborhoodViewSpec struct {
	profile        ProjectionProfileRef
	profileDigest  typedmemory.SHA256Digest
	requested      []FacetKind
	detail         DetailLevel
	includeHistory bool
}

func NewNeighborhoodViewSpec(
	profileRef ProjectionProfileRef,
	requestedFacets []FacetKind,
	detail DetailLevel,
	includeHistory bool,
) (NeighborhoodViewSpec, error) {
	profile, found := LookupProjectionProfile(profileRef)
	if !found {
		return NeighborhoodViewSpec{}, fmt.Errorf(
			"projection profile %q is unknown",
			profileRef.String(),
		)
	}
	if !profile.AllowsDetail(detail) {
		return NeighborhoodViewSpec{}, fmt.Errorf(
			"detail level %q is not admitted by projection profile %q",
			detail,
			profileRef.String(),
		)
	}
	requested, err := canonicalRequestedFacets(profile, requestedFacets)
	if err != nil {
		return NeighborhoodViewSpec{}, err
	}
	return NeighborhoodViewSpec{
		profile:        profile.Ref(),
		profileDigest:  profile.Digest(),
		requested:      requested,
		detail:         detail,
		includeHistory: includeHistory,
	}, nil
}

func (spec NeighborhoodViewSpec) ProfileRef() ProjectionProfileRef {
	return spec.profile
}

func (spec NeighborhoodViewSpec) ProfileDigest() typedmemory.SHA256Digest {
	return spec.profileDigest
}

func (spec NeighborhoodViewSpec) RequestedFacets() []FacetKind {
	if !spec.Valid() {
		return nil
	}
	return append([]FacetKind{}, spec.requested...)
}

func (spec NeighborhoodViewSpec) Detail() DetailLevel {
	return spec.detail
}

func (spec NeighborhoodViewSpec) IncludeHistory() bool {
	return spec.includeHistory
}

func (spec NeighborhoodViewSpec) Valid() bool {
	profile, found := LookupProjectionProfile(spec.profile)
	if !found || profile.Digest() != spec.profileDigest {
		return false
	}
	if !profile.AllowsDetail(spec.detail) {
		return false
	}
	expected, err := canonicalRequestedFacets(profile, spec.requested)
	if err != nil {
		return false
	}
	return slices.Equal(expected, spec.requested)
}

func canonicalRequestedFacets(
	profile ProjectionProfileDefinition,
	requested []FacetKind,
) ([]FacetKind, error) {
	if len(requested) == 0 {
		return nil, fmt.Errorf("neighborhood view requires at least one facet")
	}
	requestedSet := make(map[FacetKind]struct{}, len(requested))
	for _, facet := range requested {
		if !profile.AllowsFacet(facet) {
			return nil, fmt.Errorf(
				"facet %q is not admitted by projection profile %q",
				facet,
				profile.Ref().String(),
			)
		}
		if _, found := requestedSet[facet]; found {
			return nil, fmt.Errorf(
				"neighborhood view repeats facet %q",
				facet,
			)
		}
		requestedSet[facet] = struct{}{}
	}
	result := make([]FacetKind, 0, len(requested))
	for _, facet := range profile.Facets() {
		if _, found := requestedSet[facet]; !found {
			continue
		}
		result = append(result, facet)
	}
	return result, nil
}

// DimensionedReadBudget keeps independent transport/read bounds. Every field
// is required in v1; zero is never an implicit default.
type DimensionedReadBudget struct {
	maxFacets              uint32
	maxItemsPerFacet       uint32
	maxRelationPaths       uint32
	maxCarrierExcerptChars uint32
	maxProvenanceDepth     uint32
}

func (budget DimensionedReadBudget) MaxFacets() uint32 {
	return budget.maxFacets
}

func (budget DimensionedReadBudget) MaxItemsPerFacet() uint32 {
	return budget.maxItemsPerFacet
}

func (budget DimensionedReadBudget) MaxRelationPathsPerItem() uint32 {
	return budget.maxRelationPaths
}

func (budget DimensionedReadBudget) MaxCarrierExcerptCharacters() uint32 {
	return budget.maxCarrierExcerptChars
}

func (budget DimensionedReadBudget) MaxProvenanceDepth() uint32 {
	return budget.maxProvenanceDepth
}

func (budget DimensionedReadBudget) Valid() bool {
	return budget.maxFacets > 0 &&
		budget.maxItemsPerFacet > 0 &&
		budget.maxRelationPaths > 0 &&
		budget.maxCarrierExcerptChars > 0 &&
		budget.maxProvenanceDepth > 0
}

type ReadBudgetBuilder struct {
	budget DimensionedReadBudget
}

func NewReadBudgetBuilder() *ReadBudgetBuilder {
	return &ReadBudgetBuilder{}
}

func (builder *ReadBudgetBuilder) SetMaxFacets(
	value uint32,
) *ReadBudgetBuilder {
	builder.budget.maxFacets = value
	return builder
}

func (builder *ReadBudgetBuilder) SetMaxItemsPerFacet(
	value uint32,
) *ReadBudgetBuilder {
	builder.budget.maxItemsPerFacet = value
	return builder
}

func (builder *ReadBudgetBuilder) SetMaxRelationPathsPerItem(
	value uint32,
) *ReadBudgetBuilder {
	builder.budget.maxRelationPaths = value
	return builder
}

func (builder *ReadBudgetBuilder) SetMaxCarrierExcerptCharacters(
	value uint32,
) *ReadBudgetBuilder {
	builder.budget.maxCarrierExcerptChars = value
	return builder
}

func (builder *ReadBudgetBuilder) SetMaxProvenanceDepth(
	value uint32,
) *ReadBudgetBuilder {
	builder.budget.maxProvenanceDepth = value
	return builder
}

func (builder *ReadBudgetBuilder) Build() (
	DimensionedReadBudget,
	error,
) {
	if builder == nil || !builder.budget.Valid() {
		return DimensionedReadBudget{}, fmt.Errorf(
			"all dimensioned read-budget limits must be positive",
		)
	}
	return builder.budget, nil
}

// NeighborhoodRequest is exact and already resolved. Unknown-entity discovery
// belongs to memory.resolve, not this request.
type NeighborhoodRequest struct {
	entity        typedmemory.PersistedRef
	context       typedmemory.BoundedContextRef
	typeEnv       typedmemory.TypeEnvRef
	graphRevision typedmemory.GraphRevision
	view          NeighborhoodViewSpec
	budget        DimensionedReadBudget
}

func (request NeighborhoodRequest) Entity() typedmemory.PersistedRef {
	return request.entity
}

func (request NeighborhoodRequest) Context() typedmemory.BoundedContextRef {
	return request.context
}

func (request NeighborhoodRequest) TypeEnv() typedmemory.TypeEnvRef {
	return request.typeEnv
}

func (request NeighborhoodRequest) GraphRevision() typedmemory.GraphRevision {
	return request.graphRevision
}

func (request NeighborhoodRequest) View() NeighborhoodViewSpec {
	return request.view
}

func (request NeighborhoodRequest) Budget() DimensionedReadBudget {
	return request.budget
}

func (request NeighborhoodRequest) Valid() bool {
	entityTypeEnv := request.entity.RefKind().TypeEnv()
	return request.entity.ReferenceID().String() != "" &&
		request.entity.RefKind().String() != "" &&
		request.typeEnv.String() != "" &&
		entityTypeEnv == request.typeEnv &&
		request.context.String() != "" &&
		request.graphRevision.Value() > 0 &&
		request.view.Valid() &&
		request.budget.Valid() &&
		requestedFacetsFitBudget(request.view, request.budget)
}

func requestedFacetsFitBudget(
	view NeighborhoodViewSpec,
	budget DimensionedReadBudget,
) bool {
	count := len(view.RequestedFacets())
	if count > math.MaxUint32 {
		return false
	}
	value := uint32(count) // #nosec G115 -- count is bounded by math.MaxUint32 above.
	return value <= budget.MaxFacets()
}

type NeighborhoodRequestBuilder struct {
	request NeighborhoodRequest
}

func NewNeighborhoodRequestBuilder() *NeighborhoodRequestBuilder {
	return &NeighborhoodRequestBuilder{}
}

func (builder *NeighborhoodRequestBuilder) SetEntity(
	value typedmemory.PersistedRef,
) *NeighborhoodRequestBuilder {
	builder.request.entity = value
	return builder
}

func (builder *NeighborhoodRequestBuilder) SetContext(
	value typedmemory.BoundedContextRef,
) *NeighborhoodRequestBuilder {
	builder.request.context = value
	return builder
}

func (builder *NeighborhoodRequestBuilder) SetTypeEnv(
	value typedmemory.TypeEnvRef,
) *NeighborhoodRequestBuilder {
	builder.request.typeEnv = value
	return builder
}

func (builder *NeighborhoodRequestBuilder) SetGraphRevision(
	value typedmemory.GraphRevision,
) *NeighborhoodRequestBuilder {
	builder.request.graphRevision = value
	return builder
}

func (builder *NeighborhoodRequestBuilder) SetView(
	value NeighborhoodViewSpec,
) *NeighborhoodRequestBuilder {
	builder.request.view = value
	return builder
}

func (builder *NeighborhoodRequestBuilder) SetBudget(
	value DimensionedReadBudget,
) *NeighborhoodRequestBuilder {
	builder.request.budget = value
	return builder
}

func (builder *NeighborhoodRequestBuilder) Build() (
	NeighborhoodRequest,
	error,
) {
	if builder == nil || !builder.request.Valid() {
		return NeighborhoodRequest{}, fmt.Errorf(
			"exact neighborhood request is incomplete or inconsistent",
		)
	}
	return builder.request, nil
}
