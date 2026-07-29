package neighborhood

import (
	"fmt"
	"slices"
	"sort"
)

// RootProjectionSource couples exact source-owned root content with its
// mandatory projection basis.
type RootProjectionSource struct {
	root  ProjectedRoot
	basis ProjectionItemBasis
}

func NewRootProjectionSource(
	root ProjectedRoot,
	basis ProjectionItemBasis,
) (RootProjectionSource, error) {
	source := RootProjectionSource{
		root:  root,
		basis: basis,
	}
	if !source.valid() {
		return RootProjectionSource{}, fmt.Errorf(
			"root projection source is invalid",
		)
	}
	return source, nil
}

func (source RootProjectionSource) Root() ProjectedRoot {
	return source.root
}

func (source RootProjectionSource) Basis() ProjectionItemBasis {
	return source.basis
}

func (source RootProjectionSource) valid() bool {
	return source.root.Valid() &&
		source.basis != nil &&
		source.basis.Output() == source.root.Coordinate()
}

// ItemProjectionSource couples source-owned text/postures/path witnesses with
// the exact direct or correspondence basis for that output item.
type ItemProjectionSource struct {
	item  NeighborhoodItem
	basis ProjectionItemBasis
}

func NewItemProjectionSource(
	item NeighborhoodItem,
	basis ProjectionItemBasis,
) (ItemProjectionSource, error) {
	source := ItemProjectionSource{
		item:  item,
		basis: basis,
	}
	if !source.valid() {
		return ItemProjectionSource{}, fmt.Errorf(
			"item projection source is invalid",
		)
	}
	return source, nil
}

func (source ItemProjectionSource) Item() NeighborhoodItem {
	return source.item
}

func (source ItemProjectionSource) Basis() ProjectionItemBasis {
	return source.basis
}

func (source ItemProjectionSource) valid() bool {
	return source.item.Valid() &&
		source.basis != nil &&
		source.basis.Output() == source.item.Coordinate()
}

type FacetInputKind string

const (
	FacetInputExact         FacetInputKind = "exact"
	FacetInputNotApplicable FacetInputKind = "not_applicable"
	FacetInputUnavailable   FacetInputKind = "unavailable"
	FacetInputStale         FacetInputKind = "stale"
)

type FacetProjectionInput interface {
	Kind() FacetInputKind
	Facet() FacetKind
	isFacetProjectionInput()
}

// ExactFacetInput carries an explicit completeness input even when Items is
// empty. This is what makes Complete(0) different from missing basis.
type ExactFacetInput struct {
	facet             FacetKind
	completenessInput ProjectionInputCoordinate
	items             []ItemProjectionSource
}

func NewExactFacetInput(
	facet FacetKind,
	completenessInput ProjectionInputCoordinate,
	items []ItemProjectionSource,
) (ExactFacetInput, error) {
	input := ExactFacetInput{
		facet:             facet,
		completenessInput: completenessInput,
		items:             canonicalItemProjectionSources(items),
	}
	if !input.valid() {
		return ExactFacetInput{}, fmt.Errorf(
			"exact facet input %q is invalid",
			facet,
		)
	}
	return input, nil
}

func (ExactFacetInput) Kind() FacetInputKind {
	return FacetInputExact
}

func (input ExactFacetInput) Facet() FacetKind {
	return input.facet
}

func (input ExactFacetInput) CompletenessInput() ProjectionInputCoordinate {
	return input.completenessInput
}

func (input ExactFacetInput) Items() []ItemProjectionSource {
	return append([]ItemProjectionSource{}, input.items...)
}

func (ExactFacetInput) isFacetProjectionInput() {}

func (input ExactFacetInput) valid() bool {
	if !input.facet.Valid() || !input.completenessInput.Valid() {
		return false
	}
	seen := make(map[string]struct{}, len(input.items))
	for _, item := range input.items {
		itemFacet, found := item.Item().Coordinate().Facet()
		if !item.valid() || !found || itemFacet != input.facet {
			return false
		}
		key := item.Item().Coordinate().key()
		if _, found := seen[key]; found {
			return false
		}
		seen[key] = struct{}{}
	}
	return true
}

type NotApplicableFacetInput struct {
	facet FacetKind
	basis ApplicabilityBasisRef
}

func NewNotApplicableFacetInput(
	facet FacetKind,
	basis ApplicabilityBasisRef,
) (NotApplicableFacetInput, error) {
	input := NotApplicableFacetInput{
		facet: facet,
		basis: basis,
	}
	if !facet.Valid() || basis.String() == "" {
		return NotApplicableFacetInput{}, fmt.Errorf(
			"not-applicable facet input is invalid",
		)
	}
	return input, nil
}

func (NotApplicableFacetInput) Kind() FacetInputKind {
	return FacetInputNotApplicable
}

func (input NotApplicableFacetInput) Facet() FacetKind {
	return input.facet
}

func (input NotApplicableFacetInput) Basis() ApplicabilityBasisRef {
	return input.basis
}

func (NotApplicableFacetInput) isFacetProjectionInput() {}

type UnavailableFacetInput struct {
	facet   FacetKind
	missing MissingBasisRef
	issue   FacetBasisIssue
}

func NewUnavailableFacetInput(
	facet FacetKind,
	missing MissingBasisRef,
	issue FacetBasisIssue,
) (UnavailableFacetInput, error) {
	input := UnavailableFacetInput{
		facet:   facet,
		missing: missing,
		issue:   issue,
	}
	if !input.valid() {
		return UnavailableFacetInput{}, fmt.Errorf(
			"unavailable facet input is invalid",
		)
	}
	return input, nil
}

func (UnavailableFacetInput) Kind() FacetInputKind {
	return FacetInputUnavailable
}

func (input UnavailableFacetInput) Facet() FacetKind {
	return input.facet
}

func (input UnavailableFacetInput) MissingBasis() MissingBasisRef {
	return input.missing
}

func (input UnavailableFacetInput) Issue() FacetBasisIssue {
	return input.issue
}

func (UnavailableFacetInput) isFacetProjectionInput() {}

func (input UnavailableFacetInput) valid() bool {
	return input.facet.Valid() &&
		input.missing.String() != "" &&
		validFacetBasisIssue(input.issue) &&
		input.issue.Facet() == input.facet &&
		input.issue.Kind() != IssueStaleDerivedProjection
}

type StaleFacetInput struct {
	facet FacetKind
	retry RetryBasisRef
	issue StaleDerivedProjectionIssue
}

func NewStaleFacetInput(
	facet FacetKind,
	retry RetryBasisRef,
	issue StaleDerivedProjectionIssue,
) (StaleFacetInput, error) {
	input := StaleFacetInput{
		facet: facet,
		retry: retry,
		issue: issue,
	}
	if !input.valid() {
		return StaleFacetInput{}, fmt.Errorf(
			"stale facet input is invalid",
		)
	}
	return input, nil
}

func (StaleFacetInput) Kind() FacetInputKind {
	return FacetInputStale
}

func (input StaleFacetInput) Facet() FacetKind {
	return input.facet
}

func (input StaleFacetInput) RetryBasis() RetryBasisRef {
	return input.retry
}

func (input StaleFacetInput) Issue() StaleDerivedProjectionIssue {
	return input.issue
}

func (StaleFacetInput) isFacetProjectionInput() {}

func (input StaleFacetInput) valid() bool {
	return input.facet.Valid() &&
		input.retry.String() != "" &&
		validFacetBasisIssue(input.issue) &&
		input.issue.Facet() == input.facet
}

// PinnedNeighborhoodInput contains only already-loaded exact values. Building
// or assembling it performs no graph, DB, file, code-index, cache, clock, or
// evidence IO.
type PinnedNeighborhoodInput struct {
	request         NeighborhoodRequest
	snapshot        SnapshotBasis
	root            RootProjectionSource
	canonicalInputs []CanonicalInputCoordinate
	derivedInputs   []DerivedProjectionCoordinate
	manifests       []ProjectionCorrespondenceManifest
	facets          []FacetProjectionInput
}

type PinnedNeighborhoodInputBuilder struct {
	input       PinnedNeighborhoodInput
	requestSet  bool
	snapshotSet bool
	rootSet     bool
}

func NewPinnedNeighborhoodInputBuilder() *PinnedNeighborhoodInputBuilder {
	return &PinnedNeighborhoodInputBuilder{}
}

func (builder *PinnedNeighborhoodInputBuilder) SetRequest(
	value NeighborhoodRequest,
) *PinnedNeighborhoodInputBuilder {
	builder.input.request = value
	builder.requestSet = true
	return builder
}

func (builder *PinnedNeighborhoodInputBuilder) SetSnapshot(
	value SnapshotBasis,
) *PinnedNeighborhoodInputBuilder {
	builder.input.snapshot = value
	builder.snapshotSet = true
	return builder
}

func (builder *PinnedNeighborhoodInputBuilder) SetRoot(
	value RootProjectionSource,
) *PinnedNeighborhoodInputBuilder {
	builder.input.root = value
	builder.rootSet = true
	return builder
}

func (builder *PinnedNeighborhoodInputBuilder) AddCanonicalInput(
	value CanonicalInputCoordinate,
) *PinnedNeighborhoodInputBuilder {
	builder.input.canonicalInputs = append(
		builder.input.canonicalInputs,
		value,
	)
	return builder
}

func (builder *PinnedNeighborhoodInputBuilder) AddDerivedInput(
	value DerivedProjectionCoordinate,
) *PinnedNeighborhoodInputBuilder {
	builder.input.derivedInputs = append(builder.input.derivedInputs, value)
	return builder
}

func (builder *PinnedNeighborhoodInputBuilder) AddCorrespondenceManifest(
	value ProjectionCorrespondenceManifest,
) *PinnedNeighborhoodInputBuilder {
	builder.input.manifests = append(builder.input.manifests, value)
	return builder
}

func (builder *PinnedNeighborhoodInputBuilder) AddFacet(
	value FacetProjectionInput,
) *PinnedNeighborhoodInputBuilder {
	builder.input.facets = append(builder.input.facets, value)
	return builder
}

func (builder *PinnedNeighborhoodInputBuilder) Build() (
	PinnedNeighborhoodInput,
	error,
) {
	if builder == nil ||
		!builder.requestSet ||
		!builder.snapshotSet ||
		!builder.rootSet {
		return PinnedNeighborhoodInput{}, fmt.Errorf(
			"pinned neighborhood input requires request, snapshot, and root",
		)
	}
	if hasDuplicateFacetInputs(builder.input.facets) {
		return PinnedNeighborhoodInput{}, fmt.Errorf(
			"pinned neighborhood input repeats a facet source",
		)
	}
	input := builder.input
	input.canonicalInputs = canonicalCanonicalInputs(input.canonicalInputs)
	input.derivedInputs = canonicalDerivedInputs(input.derivedInputs)
	input.manifests = canonicalCorrespondenceManifests(input.manifests)
	input.facets = canonicalFacetInputs(input.request.View(), input.facets)
	if err := validatePinnedNeighborhoodInput(input); err != nil {
		return PinnedNeighborhoodInput{}, err
	}
	return input, nil
}

func hasDuplicateFacetInputs(values []FacetProjectionInput) bool {
	seen := make(map[FacetKind]struct{}, len(values))
	for _, value := range values {
		if value == nil {
			return true
		}
		if _, found := seen[value.Facet()]; found {
			return true
		}
		seen[value.Facet()] = struct{}{}
	}
	return false
}

func validatePinnedNeighborhoodInput(
	input PinnedNeighborhoodInput,
) error {
	if !input.request.Valid() ||
		!input.snapshot.Valid() ||
		!input.root.valid() {
		return fmt.Errorf("pinned neighborhood coordinate is invalid")
	}
	if input.request.GraphRevision() != input.snapshot.GraphRevision() ||
		input.request.TypeEnv() != input.snapshot.TypeEnv() ||
		input.request.Entity() != input.root.Root().Reference() {
		return fmt.Errorf(
			"pinned neighborhood request, snapshot, and root disagree",
		)
	}
	if len(input.canonicalInputs) == 0 ||
		!allCanonicalInputsValid(input.canonicalInputs) ||
		!allDerivedInputsValid(input.derivedInputs) ||
		!allCorrespondenceManifestsValid(input.manifests) {
		return fmt.Errorf("pinned neighborhood input basis is incomplete")
	}
	if hasDuplicateCanonicalInputRefs(input.canonicalInputs) ||
		hasDuplicateDerivedProjectionRefs(input.derivedInputs) ||
		hasDuplicateManifestRefs(input.manifests) {
		return fmt.Errorf("pinned neighborhood input repeats a basis coordinate")
	}
	requested := input.request.View().RequestedFacets()
	if len(input.facets) != len(requested) {
		return fmt.Errorf(
			"pinned neighborhood requires one input per requested facet",
		)
	}
	for index, facet := range input.facets {
		if facet == nil ||
			facet.Facet() != requested[index] ||
			!validFacetProjectionInput(facet) {
			return fmt.Errorf(
				"pinned neighborhood facet input differs from view",
			)
		}
		if err := validateFacetInputTypeEnv(
			input.request,
			facet,
		); err != nil {
			return err
		}
	}
	if input.root.Root().Reference().RefKind().TypeEnv() !=
		input.request.TypeEnv() {
		return fmt.Errorf("pinned neighborhood root uses another TypeEnv")
	}
	return nil
}

func validateFacetInputTypeEnv(
	request NeighborhoodRequest,
	input FacetProjectionInput,
) error {
	exact, ok := input.(ExactFacetInput)
	if !ok {
		return nil
	}
	for _, source := range exact.Items() {
		item := source.Item()
		if item.Reference().RefKind().TypeEnv() != request.TypeEnv() {
			return fmt.Errorf(
				"facet %q item uses another TypeEnv",
				input.Facet(),
			)
		}
		for _, witness := range item.WhyIncluded() {
			if witness.Context() != request.Context() {
				return fmt.Errorf(
					"facet %q inclusion path crosses context without boundary",
					input.Facet(),
				)
			}
		}
	}
	return nil
}

func validFacetProjectionInput(input FacetProjectionInput) bool {
	switch value := input.(type) {
	case ExactFacetInput:
		return value.valid()
	case NotApplicableFacetInput:
		return value.Facet().Valid() && value.Basis().String() != ""
	case UnavailableFacetInput:
		return value.valid()
	case StaleFacetInput:
		return value.valid()
	default:
		return false
	}
}

// Assemble is a pure deterministic projection over PinnedNeighborhoodInput.
func Assemble(input PinnedNeighborhoodInput) (ExactNeighborhood, error) {
	if err := validatePinnedNeighborhoodInput(input); err != nil {
		return ExactNeighborhood{}, err
	}
	profile, found := LookupProjectionProfile(
		input.request.View().ProfileRef(),
	)
	if !found {
		return ExactNeighborhood{}, fmt.Errorf(
			"projection profile is unavailable",
		)
	}
	view, err := NewMemoryViewContext(
		input.request.Entity(),
		input.request.Context(),
		profile.Ref(),
	)
	if err != nil {
		return ExactNeighborhood{}, err
	}
	assembly, err := assembleFacets(input, profile)
	if err != nil {
		return ExactNeighborhood{}, err
	}
	projection, err := assembleProjectionBasis(
		input,
		profile,
		assembly.itemBases,
	)
	if err != nil {
		return ExactNeighborhood{}, err
	}
	boundaries := assembleBoundaries(
		profile,
		input.request.View(),
		assembly.issues,
	)
	affordances := assembleReadAffordances(
		view,
		assembly.facets,
		boundaries,
	)
	postures := collectResultPostures(input.root.Root(), assembly.facets)
	relationalPostures := collectRelationalRecordPostures(assembly.facets)
	interpretation := interpretationForExactNeighborhood(
		postures,
		relationalPostures,
	)
	appliedBudget := assembleAppliedBudget(
		input.request.Budget(),
		assembly,
	)
	result := ExactNeighborhood{
		view:           view,
		snapshot:       input.snapshot,
		projection:     projection,
		root:           input.root.Root(),
		facets:         assembly.facets,
		boundaries:     boundaries,
		interpretation: interpretation,
		affordances:    affordances,
		budget:         appliedBudget,
	}
	boundedBytes, err := encodeExactNeighborhoodBoundedContent(result)
	if err != nil {
		return ExactNeighborhood{}, err
	}
	result.budget.boundedContentUTF8Bytes = uint64(len(boundedBytes))
	canonical, err := encodeExactNeighborhoodCanonical(result)
	if err != nil {
		return ExactNeighborhood{}, err
	}
	digest, err := digestExactNeighborhoodCanonical(canonical)
	if err != nil {
		return ExactNeighborhood{}, err
	}
	result.canonical = canonical
	result.digest = digest
	if !result.valid() {
		return ExactNeighborhood{}, fmt.Errorf(
			"assembled exact neighborhood is invalid",
		)
	}
	return result, nil
}

type facetAssembly struct {
	facets               []NeighborhoodFacet
	itemBases            []ProjectionItemBasis
	issues               []FacetBasisIssue
	perFacetBudget       []FacetBudgetApplication
	emittedRelationPaths uint64
	omittedRelationPaths uint64
	continuationCursors  []SnapshotCursor
}

func assembleFacets(
	input PinnedNeighborhoodInput,
	profile ProjectionProfileDefinition,
) (facetAssembly, error) {
	result := facetAssembly{
		facets:              make([]NeighborhoodFacet, 0, len(input.facets)),
		itemBases:           []ProjectionItemBasis{input.root.Basis()},
		issues:              make([]FacetBasisIssue, 0),
		perFacetBudget:      make([]FacetBudgetApplication, 0, len(input.facets)),
		continuationCursors: make([]SnapshotCursor, 0),
	}
	for _, source := range input.facets {
		current, err := assembleFacet(input, profile, source)
		if err != nil {
			return facetAssembly{}, err
		}
		result.facets = append(result.facets, current.facet)
		result.itemBases = append(result.itemBases, current.itemBases...)
		result.issues = append(result.issues, current.issues...)
		result.perFacetBudget = append(
			result.perFacetBudget,
			current.budget,
		)
		result.emittedRelationPaths += current.emittedRelationPaths
		result.omittedRelationPaths += current.omittedRelationPaths
		if current.cursor.Valid() {
			result.continuationCursors = append(
				result.continuationCursors,
				current.cursor,
			)
		}
	}
	return result, nil
}

type oneFacetAssembly struct {
	facet                NeighborhoodFacet
	itemBases            []ProjectionItemBasis
	issues               []FacetBasisIssue
	budget               FacetBudgetApplication
	emittedRelationPaths uint64
	omittedRelationPaths uint64
	cursor               SnapshotCursor
}

func assembleFacet(
	input PinnedNeighborhoodInput,
	profile ProjectionProfileDefinition,
	source FacetProjectionInput,
) (oneFacetAssembly, error) {
	switch value := source.(type) {
	case ExactFacetInput:
		return assembleExactFacet(input, profile, value)
	case NotApplicableFacetInput:
		return assembleNotApplicableFacet(value)
	case UnavailableFacetInput:
		return assembleUnavailableFacet(value)
	case StaleFacetInput:
		return assembleStaleFacet(value)
	default:
		return oneFacetAssembly{}, fmt.Errorf(
			"unsupported facet projection input",
		)
	}
}

func assembleExactFacet(
	input PinnedNeighborhoodInput,
	profile ProjectionProfileDefinition,
	source ExactFacetInput,
) (oneFacetAssembly, error) {
	filtered, filteredCount := filterFacetHistory(
		source.Items(),
		input.request.View().IncludeHistory(),
	)
	maxItems := int(input.request.Budget().MaxItemsPerFacet())
	includedSources := filtered
	if len(includedSources) > maxItems {
		includedSources = includedSources[:maxItems]
	}
	omittedItems, omittedItemsOK := checkedSuffixCount(
		filtered,
		len(includedSources),
	)
	if !omittedItemsOK {
		return oneFacetAssembly{}, fmt.Errorf(
			"included facet items exceed filtered facet items",
		)
	}
	items := make([]NeighborhoodItem, 0, len(includedSources))
	itemBases := make([]ProjectionItemBasis, 0, len(includedSources))
	emittedPaths := uint64(0)
	omittedPaths := uint64(0)
	for _, projected := range includedSources {
		item, emitted, omitted, err := applyRelationPathBudget(
			projected.Item(),
			input.request.Budget().MaxRelationPathsPerItem(),
		)
		if err != nil {
			return oneFacetAssembly{}, err
		}
		items = append(items, item)
		itemBases = append(itemBases, projected.Basis())
		emittedPaths += emitted
		omittedPaths += omitted
	}
	coverage := FacetCoverage(NewCompleteCoverage(uint64(len(items))))
	cursor := SnapshotCursor{}
	if omittedItems > 0 {
		nextOffset := uint64(len(items))
		cursorValue, err := NewSnapshotCursor(
			input.snapshot.GraphRevision(),
			input.snapshot.TypeEnv(),
			profile,
			source.Facet(),
			nextOffset,
		)
		if err != nil {
			return oneFacetAssembly{}, err
		}
		partial, err := NewPartialCoverage(
			uint64(len(items)),
			omittedItems,
			cursorValue,
		)
		if err != nil {
			return oneFacetAssembly{}, err
		}
		coverage = partial
		cursor = cursorValue
	}
	facet, err := newNeighborhoodFacet(source.Facet(), coverage, items)
	if err != nil {
		return oneFacetAssembly{}, err
	}
	return oneFacetAssembly{
		facet:     facet,
		itemBases: itemBases,
		budget: FacetBudgetApplication{
			facet:         source.Facet(),
			included:      uint64(len(items)),
			omittedItems:  omittedItems,
			filteredItems: filteredCount,
		},
		emittedRelationPaths: emittedPaths,
		omittedRelationPaths: omittedPaths,
		cursor:               cursor,
	}, nil
}

func assembleNotApplicableFacet(
	source NotApplicableFacetInput,
) (oneFacetAssembly, error) {
	coverage, err := NewNotApplicableCoverage(source.Basis())
	if err != nil {
		return oneFacetAssembly{}, err
	}
	facet, err := newNeighborhoodFacet(source.Facet(), coverage, nil)
	if err != nil {
		return oneFacetAssembly{}, err
	}
	return emptyFacetAssembly(facet), nil
}

func assembleUnavailableFacet(
	source UnavailableFacetInput,
) (oneFacetAssembly, error) {
	coverage, err := NewUnavailableCoverage(source.MissingBasis())
	if err != nil {
		return oneFacetAssembly{}, err
	}
	facet, err := newNeighborhoodFacet(source.Facet(), coverage, nil)
	if err != nil {
		return oneFacetAssembly{}, err
	}
	result := emptyFacetAssembly(facet)
	result.issues = []FacetBasisIssue{source.Issue()}
	return result, nil
}

func assembleStaleFacet(
	source StaleFacetInput,
) (oneFacetAssembly, error) {
	coverage, err := NewStaleCoverage(source.RetryBasis())
	if err != nil {
		return oneFacetAssembly{}, err
	}
	facet, err := newNeighborhoodFacet(source.Facet(), coverage, nil)
	if err != nil {
		return oneFacetAssembly{}, err
	}
	result := emptyFacetAssembly(facet)
	result.issues = []FacetBasisIssue{source.Issue()}
	return result, nil
}

func emptyFacetAssembly(
	facet NeighborhoodFacet,
) oneFacetAssembly {
	return oneFacetAssembly{
		facet: facet,
		budget: FacetBudgetApplication{
			facet: facet.Kind(),
		},
	}
}

func applyRelationPathBudget(
	item NeighborhoodItem,
	max uint32,
) (NeighborhoodItem, uint64, uint64, error) {
	paths := item.WhyIncluded()
	included := paths
	if len(included) > int(max) {
		included = included[:max]
	}
	omitted, omittedOK := checkedSuffixCount(paths, len(included))
	if !omittedOK {
		return NeighborhoodItem{}, 0, 0, fmt.Errorf(
			"included relation paths exceed available relation paths",
		)
	}
	result, err := NewNeighborhoodItem(
		item.Coordinate(),
		item.ItemKind(),
		item.Text(),
		item.Postures(),
		item.Provenance(),
		included,
	)
	if err != nil {
		return NeighborhoodItem{}, 0, 0, err
	}
	emitted := uint64(len(included))
	return result, emitted, omitted, nil
}

func filterFacetHistory(
	values []ItemProjectionSource,
	includeHistory bool,
) ([]ItemProjectionSource, uint64) {
	if includeHistory {
		return values, 0
	}
	result := make([]ItemProjectionSource, 0, len(values))
	filteredCount := uint64(0)
	for _, value := range values {
		if value.Item().Postures().Lifecycle() != LifecycleActive {
			filteredCount++
			continue
		}
		result = append(result, value)
	}
	return result, filteredCount
}

func checkedSuffixCount[T any](
	values []T,
	prefixLength int,
) (uint64, bool) {
	if prefixLength < 0 || prefixLength > len(values) {
		return 0, false
	}
	return uint64(len(values[prefixLength:])), true
}

func assembleProjectionBasis(
	input PinnedNeighborhoodInput,
	profile ProjectionProfileDefinition,
	itemBases []ProjectionItemBasis,
) (ProjectionBasis, error) {
	builder := NewProjectionBasisBuilder(profile)
	for _, value := range input.canonicalInputs {
		builder.AddCanonicalInput(value)
	}
	for _, value := range input.derivedInputs {
		builder.AddDerivedInput(value)
	}
	requiredManifests := requiredCorrespondenceManifestRefs(itemBases)
	for _, value := range input.manifests {
		if !slices.Contains(requiredManifests, value.Ref()) {
			continue
		}
		builder.AddCorrespondenceManifest(value)
	}
	for _, value := range itemBases {
		builder.AddItemBasis(value)
	}
	return builder.Build()
}

func requiredCorrespondenceManifestRefs(
	values []ProjectionItemBasis,
) []ProjectionCorrespondenceManifestRef {
	result := make([]ProjectionCorrespondenceManifestRef, 0)
	for _, value := range values {
		correspondence, ok := value.(CorrespondenceProjectionItemBasis)
		if !ok {
			continue
		}
		result = append(result, correspondence.ManifestRef())
	}
	sort.Slice(result, func(left int, right int) bool {
		return result[left].String() < result[right].String()
	})
	return slices.Compact(result)
}

func assembleBoundaries(
	profile ProjectionProfileDefinition,
	view NeighborhoodViewSpec,
	issues []FacetBasisIssue,
) NeighborhoodBoundaries {
	requested := view.RequestedFacets()
	omitted := make([]FacetKind, 0)
	for _, facet := range profile.Facets() {
		if slices.Contains(requested, facet) {
			continue
		}
		omitted = append(omitted, facet)
	}
	unresolved := make([]LegacyRecordRef, 0)
	crossContext := make([]CrossContextReference, 0)
	for _, issue := range issues {
		legacy, legacyOK := issue.(UnresolvedLegacyIdentityIssue)
		if legacyOK {
			unresolved = append(unresolved, legacy.LegacyRef())
		}
		bridge, bridgeOK := issue.(ExplicitBridgeRequiredIssue)
		if bridgeOK {
			crossContext = append(
				crossContext,
				newCrossContextReference(bridge),
			)
		}
	}
	return NeighborhoodBoundaries{
		crossContextRefs: crossContext,
		unresolvedItems:  unresolved,
		omittedFacets:    omitted,
		facetBasisIssues: append([]FacetBasisIssue{}, issues...),
	}
}

func assembleReadAffordances(
	view MemoryViewContext,
	facets []NeighborhoodFacet,
	boundaries NeighborhoodBoundaries,
) []ReadAffordance {
	result := []ReadAffordance{
		newInspectEntityAffordance(view.Entity(), view.Context()),
	}
	for _, facet := range facets {
		partial, ok := facet.Coverage().(PartialCoverage)
		if !ok {
			continue
		}
		result = append(
			result,
			newExpandFacetAffordance(facet.Kind(), partial.Cursor()),
		)
	}
	for _, boundary := range boundaries.CrossContextRefs() {
		known, ok := boundary.Bridge().(KnownBridge)
		if !ok {
			continue
		}
		result = append(
			result,
			newFollowContextBridgeAffordance(
				known.Ref(),
				boundary.TargetContext(),
			),
		)
	}
	return result
}

func collectResultPostures(
	root ProjectedRoot,
	facets []NeighborhoodFacet,
) []ItemPostures {
	result := []ItemPostures{root.Postures()}
	for _, facet := range facets {
		for _, item := range facet.Items() {
			result = append(result, item.Postures())
		}
	}
	return result
}

func collectRelationalRecordPostures(
	facets []NeighborhoodFacet,
) []RelationalRecordItemPosture {
	result := make([]RelationalRecordItemPosture, 0)
	for _, facet := range facets {
		for _, item := range facet.Items() {
			for _, witness := range item.WhyIncluded() {
				result = append(
					result,
					witness.RelationalRecordPosture(),
				)
			}
		}
	}
	return result
}

func assembleAppliedBudget(
	requested DimensionedReadBudget,
	assembly facetAssembly,
) AppliedReadBudget {
	return AppliedReadBudget{
		requested:                requested,
		applied:                  requested,
		perFacet:                 assembly.perFacetBudget,
		emittedRelationPaths:     assembly.emittedRelationPaths,
		omittedRelationPaths:     assembly.omittedRelationPaths,
		emittedExcerptCharacters: 0,
		emittedProvenanceDepth:   1,
		continuationCursors: append(
			[]SnapshotCursor{},
			assembly.continuationCursors...,
		),
	}
}

func canonicalItemProjectionSources(
	values []ItemProjectionSource,
) []ItemProjectionSource {
	result := append([]ItemProjectionSource{}, values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].Item().Coordinate().key() <
			result[right].Item().Coordinate().key()
	})
	return result
}

func canonicalFacetInputs(
	view NeighborhoodViewSpec,
	values []FacetProjectionInput,
) []FacetProjectionInput {
	byFacet := make(map[FacetKind]FacetProjectionInput, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		if _, found := byFacet[value.Facet()]; found {
			continue
		}
		byFacet[value.Facet()] = value
	}
	result := make([]FacetProjectionInput, 0, len(values))
	for _, facet := range view.RequestedFacets() {
		value, found := byFacet[facet]
		if !found {
			continue
		}
		result = append(result, value)
	}
	return result
}
