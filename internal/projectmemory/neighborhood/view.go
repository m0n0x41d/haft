package neighborhood

import (
	"fmt"
	"slices"
	"sort"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type ReadableItemText struct{ value string }

func NewReadableItemText(raw string) (ReadableItemText, error) {
	value, err := exactReference("readable item text", raw)
	if err != nil {
		return ReadableItemText{}, err
	}
	return ReadableItemText{value: value}, nil
}

func (text ReadableItemText) String() string { return text.value }

// ProjectedRoot is the preserved EntityOfConcern. Its direct/correspondence
// basis remains in ProjectionBasis; the root does not need an inclusion path
// to itself.
type ProjectedRoot struct {
	coordinate OutputItemCoordinate
	text       ReadableItemText
	postures   ItemPostures
	provenance typedmemory.ProvenanceRef
}

func NewProjectedRoot(
	coordinate OutputItemCoordinate,
	text ReadableItemText,
	postures ItemPostures,
	provenance typedmemory.ProvenanceRef,
) (ProjectedRoot, error) {
	root := ProjectedRoot{
		coordinate: coordinate,
		text:       text,
		postures:   postures,
		provenance: provenance,
	}
	if !root.Valid() {
		return ProjectedRoot{}, fmt.Errorf("projected root is invalid")
	}
	return root, nil
}

func (root ProjectedRoot) Coordinate() OutputItemCoordinate {
	return root.coordinate
}

func (root ProjectedRoot) Reference() typedmemory.PersistedRef {
	return root.coordinate.Reference()
}

func (root ProjectedRoot) Text() ReadableItemText {
	return root.text
}

func (root ProjectedRoot) Postures() ItemPostures {
	return root.postures
}

func (root ProjectedRoot) Provenance() typedmemory.ProvenanceRef {
	return root.provenance
}

func (root ProjectedRoot) Valid() bool {
	provenance, err := typedmemory.NewProvenanceRef(
		root.provenance.String(),
	)
	return root.coordinate.Valid() &&
		root.coordinate.Scope() == OutputRootItem &&
		root.text.String() != "" &&
		root.postures.Valid() &&
		err == nil &&
		provenance == root.provenance
}

type NeighborhoodItem struct {
	coordinate  OutputItemCoordinate
	itemKind    ItemKind
	text        ReadableItemText
	postures    ItemPostures
	provenance  typedmemory.ProvenanceRef
	whyIncluded []RelationPathWitness
}

func NewNeighborhoodItem(
	coordinate OutputItemCoordinate,
	itemKind ItemKind,
	text ReadableItemText,
	postures ItemPostures,
	provenance typedmemory.ProvenanceRef,
	whyIncluded []RelationPathWitness,
) (NeighborhoodItem, error) {
	item := NeighborhoodItem{
		coordinate: coordinate,
		itemKind:   itemKind,
		text:       text,
		postures:   postures,
		provenance: provenance,
		whyIncluded: canonicalRelationWitnesses(
			whyIncluded,
		),
	}
	if !item.Valid() {
		return NeighborhoodItem{}, fmt.Errorf(
			"neighborhood item is invalid",
		)
	}
	return item, nil
}

func (item NeighborhoodItem) Coordinate() OutputItemCoordinate {
	return item.coordinate
}

func (item NeighborhoodItem) Reference() typedmemory.PersistedRef {
	return item.coordinate.Reference()
}

func (item NeighborhoodItem) ItemKind() ItemKind {
	return item.itemKind
}

func (item NeighborhoodItem) Text() ReadableItemText {
	return item.text
}

func (item NeighborhoodItem) Postures() ItemPostures {
	return item.postures
}

func (item NeighborhoodItem) Provenance() typedmemory.ProvenanceRef {
	return item.provenance
}

func (item NeighborhoodItem) WhyIncluded() []RelationPathWitness {
	return append([]RelationPathWitness{}, item.whyIncluded...)
}

func (item NeighborhoodItem) Valid() bool {
	facet, facetSet := item.coordinate.Facet()
	provenance, provenanceErr := typedmemory.NewProvenanceRef(
		item.provenance.String(),
	)
	return item.coordinate.Valid() &&
		facetSet &&
		item.itemKind.Valid() &&
		item.text.String() != "" &&
		item.postures.Valid() &&
		provenanceErr == nil &&
		provenance == item.provenance &&
		len(item.whyIncluded) > 0 &&
		allRelationWitnessesValid(item.whyIncluded) &&
		itemFacetMatches(item.itemKind, facet)
}

func itemFacetMatches(item ItemKind, facet FacetKind) bool {
	for _, rule := range canonicalItemFacetRulesV2 {
		if rule.item == item {
			return rule.facet == facet
		}
	}
	return false
}

type NeighborhoodFacet struct {
	kind     FacetKind
	coverage FacetCoverage
	items    []NeighborhoodItem
}

func newNeighborhoodFacet(
	kind FacetKind,
	coverage FacetCoverage,
	items []NeighborhoodItem,
) (NeighborhoodFacet, error) {
	facet := NeighborhoodFacet{
		kind:     kind,
		coverage: coverage,
		items:    canonicalNeighborhoodItems(items),
	}
	if !facet.Valid() {
		return NeighborhoodFacet{}, fmt.Errorf(
			"neighborhood facet %q is invalid",
			kind,
		)
	}
	return facet, nil
}

func (facet NeighborhoodFacet) Kind() FacetKind {
	return facet.kind
}

func (facet NeighborhoodFacet) Coverage() FacetCoverage {
	return facet.coverage
}

func (facet NeighborhoodFacet) Items() []NeighborhoodItem {
	return append([]NeighborhoodItem{}, facet.items...)
}

func (facet NeighborhoodFacet) Valid() bool {
	if !facet.kind.Valid() || facet.coverage == nil {
		return false
	}
	for _, item := range facet.items {
		itemFacet, found := item.Coordinate().Facet()
		if !item.Valid() || !found || itemFacet != facet.kind {
			return false
		}
	}
	if hasDuplicateNeighborhoodItems(facet.items) {
		return false
	}
	return facetCoverageMatchesItems(facet.coverage, facet.items)
}

func facetCoverageMatchesItems(
	coverage FacetCoverage,
	items []NeighborhoodItem,
) bool {
	itemCount := uint64(len(items))
	if coverage.Included() != itemCount {
		return false
	}
	switch value := coverage.(type) {
	case CompleteCoverage:
		return value.Kind() == CoverageComplete
	case PartialCoverage:
		return value.Kind() == CoveragePartial &&
			value.OmittedAtLeast() > 0 &&
			value.Cursor().Valid()
	case NotApplicableCoverage:
		return len(items) == 0 && value.Basis().String() != ""
	case UnavailableCoverage:
		return len(items) == 0 && value.MissingBasis().String() != ""
	case StaleCoverage:
		return len(items) == 0 && value.RetryBasis().String() != ""
	default:
		return false
	}
}

type CrossContextReference struct {
	source typedmemory.BoundedContextRef
	target typedmemory.BoundedContextRef
	bridge BridgeKnowledge
}

func newCrossContextReference(
	issue ExplicitBridgeRequiredIssue,
) CrossContextReference {
	return CrossContextReference{
		source: issue.SourceContext(),
		target: issue.TargetContext(),
		bridge: issue.Bridge(),
	}
}

func (reference CrossContextReference) SourceContext() typedmemory.BoundedContextRef {
	return reference.source
}

func (reference CrossContextReference) TargetContext() typedmemory.BoundedContextRef {
	return reference.target
}

func (reference CrossContextReference) Bridge() BridgeKnowledge {
	return reference.bridge
}

type NeighborhoodBoundaries struct {
	crossContextRefs []CrossContextReference
	unresolvedItems  []LegacyRecordRef
	omittedFacets    []FacetKind
	facetBasisIssues []FacetBasisIssue
}

func (boundaries NeighborhoodBoundaries) CrossContextRefs() []CrossContextReference {
	return append([]CrossContextReference{}, boundaries.crossContextRefs...)
}

func (boundaries NeighborhoodBoundaries) UnresolvedItems() []LegacyRecordRef {
	return append([]LegacyRecordRef{}, boundaries.unresolvedItems...)
}

func (boundaries NeighborhoodBoundaries) OmittedFacets() []FacetKind {
	return append([]FacetKind{}, boundaries.omittedFacets...)
}

func (boundaries NeighborhoodBoundaries) FacetBasisIssues() []FacetBasisIssue {
	return append([]FacetBasisIssue{}, boundaries.facetBasisIssues...)
}

type ReadAffordanceKind string

const (
	AffordanceExpandFacet         ReadAffordanceKind = "expand_facet"
	AffordanceInspectEntity       ReadAffordanceKind = "inspect_entity"
	AffordanceHydrateCarrier      ReadAffordanceKind = "hydrate_carrier"
	AffordanceFollowContextBridge ReadAffordanceKind = "follow_context_bridge"
)

type ReadAffordance interface {
	Kind() ReadAffordanceKind
	isReadAffordance()
}

type ExpandFacetAffordance struct {
	facet  FacetKind
	cursor SnapshotCursor
}

func newExpandFacetAffordance(
	facet FacetKind,
	cursor SnapshotCursor,
) ExpandFacetAffordance {
	return ExpandFacetAffordance{
		facet:  facet,
		cursor: cursor,
	}
}

func (ExpandFacetAffordance) Kind() ReadAffordanceKind {
	return AffordanceExpandFacet
}

func (affordance ExpandFacetAffordance) Facet() FacetKind {
	return affordance.facet
}

func (affordance ExpandFacetAffordance) Cursor() SnapshotCursor {
	return affordance.cursor
}

func (ExpandFacetAffordance) isReadAffordance() {}

type InspectEntityAffordance struct {
	entity  typedmemory.PersistedRef
	context typedmemory.BoundedContextRef
}

func newInspectEntityAffordance(
	entity typedmemory.PersistedRef,
	context typedmemory.BoundedContextRef,
) InspectEntityAffordance {
	return InspectEntityAffordance{
		entity:  entity,
		context: context,
	}
}

func (InspectEntityAffordance) Kind() ReadAffordanceKind {
	return AffordanceInspectEntity
}

func (affordance InspectEntityAffordance) Entity() typedmemory.PersistedRef {
	return affordance.entity
}

func (affordance InspectEntityAffordance) Context() typedmemory.BoundedContextRef {
	return affordance.context
}

func (InspectEntityAffordance) isReadAffordance() {}

type CarrierRef struct{ value string }
type CarrierEdition struct{ value string }

func NewCarrierRef(raw string) (CarrierRef, error) {
	value, err := exactReference("carrier", raw)
	if err != nil {
		return CarrierRef{}, err
	}
	return CarrierRef{value: value}, nil
}

func NewCarrierEdition(raw string) (CarrierEdition, error) {
	value, err := exactReference("carrier edition", raw)
	if err != nil {
		return CarrierEdition{}, err
	}
	return CarrierEdition{value: value}, nil
}

func (ref CarrierRef) String() string { return ref.value }
func (edition CarrierEdition) String() string {
	return edition.value
}

type HydrateCarrierAffordance struct {
	carrier CarrierRef
	edition CarrierEdition
}

func NewHydrateCarrierAffordance(
	carrier CarrierRef,
	edition CarrierEdition,
) (HydrateCarrierAffordance, error) {
	if carrier.String() == "" || edition.String() == "" {
		return HydrateCarrierAffordance{}, fmt.Errorf(
			"hydrate-carrier affordance is invalid",
		)
	}
	return HydrateCarrierAffordance{
		carrier: carrier,
		edition: edition,
	}, nil
}

func (HydrateCarrierAffordance) Kind() ReadAffordanceKind {
	return AffordanceHydrateCarrier
}

func (affordance HydrateCarrierAffordance) Carrier() CarrierRef {
	return affordance.carrier
}

func (affordance HydrateCarrierAffordance) Edition() CarrierEdition {
	return affordance.edition
}

func (HydrateCarrierAffordance) isReadAffordance() {}

type FollowContextBridgeAffordance struct {
	bridge ContextBridgeRef
	target typedmemory.BoundedContextRef
}

func newFollowContextBridgeAffordance(
	bridge ContextBridgeRef,
	target typedmemory.BoundedContextRef,
) FollowContextBridgeAffordance {
	return FollowContextBridgeAffordance{
		bridge: bridge,
		target: target,
	}
}

func (FollowContextBridgeAffordance) Kind() ReadAffordanceKind {
	return AffordanceFollowContextBridge
}

func (affordance FollowContextBridgeAffordance) Bridge() ContextBridgeRef {
	return affordance.bridge
}

func (affordance FollowContextBridgeAffordance) TargetContext() typedmemory.BoundedContextRef {
	return affordance.target
}

func (FollowContextBridgeAffordance) isReadAffordance() {}

type FacetBudgetApplication struct {
	facet         FacetKind
	included      uint64
	omittedItems  uint64
	filteredItems uint64
}

func (application FacetBudgetApplication) Facet() FacetKind {
	return application.facet
}

func (application FacetBudgetApplication) IncludedItems() uint64 {
	return application.included
}

func (application FacetBudgetApplication) OmittedItems() uint64 {
	return application.omittedItems
}

func (application FacetBudgetApplication) ProfileFilteredItems() uint64 {
	return application.filteredItems
}

type AppliedReadBudget struct {
	requested                DimensionedReadBudget
	applied                  DimensionedReadBudget
	perFacet                 []FacetBudgetApplication
	emittedRelationPaths     uint64
	omittedRelationPaths     uint64
	emittedExcerptCharacters uint64
	emittedProvenanceDepth   uint64
	boundedContentUTF8Bytes  uint64
	continuationCursors      []SnapshotCursor
}

func (budget AppliedReadBudget) RequestedLimits() DimensionedReadBudget {
	return budget.requested
}

func (budget AppliedReadBudget) AppliedLimits() DimensionedReadBudget {
	return budget.applied
}

func (budget AppliedReadBudget) PerFacet() []FacetBudgetApplication {
	return append([]FacetBudgetApplication{}, budget.perFacet...)
}

func (budget AppliedReadBudget) EmittedRelationPathCount() uint64 {
	return budget.emittedRelationPaths
}

func (budget AppliedReadBudget) OmittedRelationPathCount() uint64 {
	return budget.omittedRelationPaths
}

func (budget AppliedReadBudget) EmittedExcerptCharacterCount() uint64 {
	return budget.emittedExcerptCharacters
}

func (budget AppliedReadBudget) EmittedProvenanceDepth() uint64 {
	return budget.emittedProvenanceDepth
}

func (budget AppliedReadBudget) BoundedContentUTF8Bytes() uint64 {
	return budget.boundedContentUTF8Bytes
}

func (budget AppliedReadBudget) ContinuationCursors() []SnapshotCursor {
	return append([]SnapshotCursor{}, budget.continuationCursors...)
}

type ExactNeighborhood struct {
	view           MemoryViewContext
	snapshot       SnapshotBasis
	projection     ProjectionBasis
	root           ProjectedRoot
	facets         []NeighborhoodFacet
	boundaries     NeighborhoodBoundaries
	interpretation InterpretationContract
	affordances    []ReadAffordance
	budget         AppliedReadBudget
	canonical      []byte
	digest         typedmemory.SHA256Digest
}

func (ExactNeighborhood) Kind() NeighborhoodResultKind {
	return ResultExactNeighborhood
}

func (ExactNeighborhood) ContractVersion() string {
	return NeighborhoodContractV1
}

func (result ExactNeighborhood) ViewContext() MemoryViewContext {
	return result.view
}

func (result ExactNeighborhood) SnapshotBasis() SnapshotBasis {
	return result.snapshot
}

func (result ExactNeighborhood) ProjectionBasis() ProjectionBasis {
	return result.projection
}

func (result ExactNeighborhood) Root() ProjectedRoot {
	return result.root
}

func (result ExactNeighborhood) Facets() []NeighborhoodFacet {
	return append([]NeighborhoodFacet{}, result.facets...)
}

func (result ExactNeighborhood) Boundaries() NeighborhoodBoundaries {
	return NeighborhoodBoundaries{
		crossContextRefs: result.boundaries.CrossContextRefs(),
		unresolvedItems:  result.boundaries.UnresolvedItems(),
		omittedFacets:    result.boundaries.OmittedFacets(),
		facetBasisIssues: result.boundaries.FacetBasisIssues(),
	}
}

func (result ExactNeighborhood) Interpretation() InterpretationContract {
	return result.interpretation
}

func (result ExactNeighborhood) ReadAffordances() []ReadAffordance {
	return append([]ReadAffordance{}, result.affordances...)
}

func (result ExactNeighborhood) AppliedBudget() AppliedReadBudget {
	return result.budget
}

func (result ExactNeighborhood) CanonicalBytes() []byte {
	return append([]byte{}, result.canonical...)
}

func (result ExactNeighborhood) Digest() typedmemory.SHA256Digest {
	return result.digest
}

func (result ExactNeighborhood) Valid() bool {
	return result.valid()
}

func (ExactNeighborhood) isNeighborhoodResult() {}

func (result ExactNeighborhood) valid() bool {
	if !result.view.Valid() ||
		!result.snapshot.Valid() ||
		!result.projection.Valid() ||
		!result.root.Valid() ||
		len(result.facets) == 0 ||
		!result.interpretation.Valid() ||
		result.interpretation.Structure() != StructureExactAtSnapshot ||
		len(result.affordances) == 0 ||
		!result.budget.requested.Valid() ||
		!result.budget.applied.Valid() ||
		len(result.canonical) == 0 ||
		result.digest.String() == "" {
		return false
	}
	if result.view.Entity() != result.root.Reference() ||
		result.view.ProfileRef() != result.projection.ProfileRef() ||
		result.snapshot.TypeEnv() != result.view.Entity().RefKind().TypeEnv() {
		return false
	}
	if !allFacetsValid(result.facets) ||
		!facetsFollowView(result.facets, result.view.ProfileRef()) ||
		!projectionBasisIsTotal(result) {
		return false
	}
	canonical, err := encodeExactNeighborhoodCanonical(result)
	if err != nil || !slices.Equal(canonical, result.canonical) {
		return false
	}
	digest, err := digestExactNeighborhoodCanonical(canonical)
	return err == nil && digest == result.digest
}

func canonicalNeighborhoodItems(
	values []NeighborhoodItem,
) []NeighborhoodItem {
	result := append([]NeighborhoodItem{}, values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].Coordinate().key() <
			result[right].Coordinate().key()
	})
	return result
}

func hasDuplicateNeighborhoodItems(values []NeighborhoodItem) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		key := value.Coordinate().key()
		if _, found := seen[key]; found {
			return true
		}
		seen[key] = struct{}{}
	}
	return false
}

func allFacetsValid(values []NeighborhoodFacet) bool {
	seen := make(map[FacetKind]struct{}, len(values))
	for _, value := range values {
		if !value.Valid() {
			return false
		}
		if _, found := seen[value.Kind()]; found {
			return false
		}
		seen[value.Kind()] = struct{}{}
	}
	return true
}

func facetsFollowView(
	facets []NeighborhoodFacet,
	profileRef ProjectionProfileRef,
) bool {
	profile, found := LookupProjectionProfile(profileRef)
	if !found {
		return false
	}
	positions := make(map[FacetKind]int, len(profile.Facets()))
	for index, facet := range profile.Facets() {
		positions[facet] = index
	}
	for index := 1; index < len(facets); index++ {
		left := positions[facets[index-1].Kind()]
		right := positions[facets[index].Kind()]
		if left >= right {
			return false
		}
	}
	return true
}

func projectionBasisIsTotal(result ExactNeighborhood) bool {
	if _, found := result.projection.ItemBasisFor(
		result.root.Coordinate(),
	); !found {
		return false
	}
	expected := 1
	for _, facet := range result.facets {
		expected += len(facet.items)
		for _, item := range facet.items {
			if _, found := result.projection.ItemBasisFor(
				item.Coordinate(),
			); !found {
				return false
			}
		}
	}
	return expected == len(result.projection.ItemBases())
}
