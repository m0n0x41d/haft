package neighborhood

import (
	"fmt"
	"slices"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectmemory/goldenconcernbundle"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// ExactPostureBinding is required adapter input. Snapshot presence does not
// imply lifecycle activity, evidence currentness, or projection currentness.
type ExactPostureBinding struct {
	reference typedmemory.PersistedRef
	postures  ItemPostures
}

func NewExactPostureBinding(
	reference typedmemory.PersistedRef,
	postures ItemPostures,
) (ExactPostureBinding, error) {
	binding := ExactPostureBinding{
		reference: reference,
		postures:  postures,
	}
	if !validPersistedRef(reference) || !postures.Valid() {
		return ExactPostureBinding{}, fmt.Errorf(
			"exact posture binding is invalid",
		)
	}
	return binding, nil
}

func (binding ExactPostureBinding) Reference() typedmemory.PersistedRef {
	return binding.reference
}

func (binding ExactPostureBinding) Postures() ItemPostures {
	return binding.postures
}

type ExactPostureSet struct {
	bindings []ExactPostureBinding
}

func NewExactPostureSet(
	values []ExactPostureBinding,
) (ExactPostureSet, error) {
	bindings := append([]ExactPostureBinding{}, values...)
	sort.Slice(bindings, func(left int, right int) bool {
		return persistedReferenceKey(bindings[left].Reference()) <
			persistedReferenceKey(bindings[right].Reference())
	})
	if len(bindings) == 0 {
		return ExactPostureSet{}, fmt.Errorf(
			"exact posture set cannot be empty",
		)
	}
	seen := make(map[string]struct{}, len(bindings))
	for _, binding := range bindings {
		key := persistedReferenceKey(binding.Reference())
		if !validPersistedRef(binding.Reference()) ||
			!binding.Postures().Valid() {
			return ExactPostureSet{}, fmt.Errorf(
				"exact posture set contains invalid binding",
			)
		}
		if _, found := seen[key]; found {
			return ExactPostureSet{}, fmt.Errorf(
				"exact posture set repeats %q",
				key,
			)
		}
		seen[key] = struct{}{}
	}
	return ExactPostureSet{bindings: bindings}, nil
}

func (set ExactPostureSet) posturesFor(
	reference typedmemory.PersistedRef,
) (ItemPostures, bool) {
	key := persistedReferenceKey(reference)
	index := sort.Search(len(set.bindings), func(index int) bool {
		current := persistedReferenceKey(set.bindings[index].Reference())
		return current >= key
	})
	if index >= len(set.bindings) {
		return ItemPostures{}, false
	}
	binding := set.bindings[index]
	return binding.Postures(),
		persistedReferenceKey(binding.Reference()) == key
}

// AdaptGoldenConcernBundleAcceptance turns the exact P8G acceptance proof into
// pinned input for the general pure assembler. It is intentionally named as an
// acceptance adapter: GoldenConcernBundle is not the public P10 read model.
func AdaptGoldenConcernBundleAcceptance(
	bundle goldenconcernbundle.Bundle,
	request NeighborhoodRequest,
	postures ExactPostureSet,
) (PinnedNeighborhoodInput, error) {
	if err := bundle.Verify(); err != nil {
		return PinnedNeighborhoodInput{}, fmt.Errorf(
			"verify GoldenConcernBundle acceptance input: %w",
			err,
		)
	}
	if err := validateGoldenBundleRequest(bundle, request); err != nil {
		return PinnedNeighborhoodInput{}, err
	}
	if err := requireEveryGoldenItemPosture(bundle, postures); err != nil {
		return PinnedNeighborhoodInput{}, err
	}
	profile, found := LookupProjectionProfile(request.View().ProfileRef())
	if !found {
		return PinnedNeighborhoodInput{}, fmt.Errorf(
			"GoldenConcernBundle projection profile is unavailable",
		)
	}
	canonicalInput, itemInput, err := goldenBundleInputCoordinate(bundle)
	if err != nil {
		return PinnedNeighborhoodInput{}, err
	}
	root, err := adaptGoldenRoot(bundle, postures, itemInput, profile)
	if err != nil {
		return PinnedNeighborhoodInput{}, err
	}
	facets, err := adaptGoldenFacets(
		bundle,
		request,
		postures,
		itemInput,
		profile,
	)
	if err != nil {
		return PinnedNeighborhoodInput{}, err
	}
	snapshot, err := NewSnapshotBasis(
		bundle.Snapshot().GraphRevision(),
		bundle.Snapshot().TypeEnv(),
		bundle.Snapshot().TypeEnv().Digest(),
	)
	if err != nil {
		return PinnedNeighborhoodInput{}, err
	}
	builder := NewPinnedNeighborhoodInputBuilder().
		SetRequest(request).
		SetSnapshot(snapshot).
		SetRoot(root).
		AddCanonicalInput(canonicalInput)
	for _, facet := range facets {
		builder.AddFacet(facet)
	}
	return builder.Build()
}

func validateGoldenBundleRequest(
	bundle goldenconcernbundle.Bundle,
	request NeighborhoodRequest,
) error {
	if !request.Valid() {
		return fmt.Errorf("GoldenConcernBundle request is invalid")
	}
	snapshot := bundle.Snapshot()
	if request.Entity() != bundle.Concern().Reference() ||
		request.Context() != snapshot.Context() ||
		request.TypeEnv() != snapshot.TypeEnv() ||
		request.GraphRevision() != snapshot.GraphRevision() {
		return fmt.Errorf(
			"GoldenConcernBundle and neighborhood request coordinates disagree",
		)
	}
	return nil
}

func requireEveryGoldenItemPosture(
	bundle goldenconcernbundle.Bundle,
	postures ExactPostureSet,
) error {
	for _, item := range bundle.Items() {
		if _, found := postures.posturesFor(item.Reference()); found {
			continue
		}
		return fmt.Errorf(
			"GoldenConcernBundle item %q has no exact independent postures",
			persistedReferenceKey(item.Reference()),
		)
	}
	return nil
}

func goldenBundleInputCoordinate(
	bundle goldenconcernbundle.Bundle,
) (
	CanonicalInputCoordinate,
	ProjectionInputCoordinate,
	error,
) {
	raw := "golden-concern-bundle:" + bundle.Digest().String()
	ref, err := NewProjectionInputRef(raw)
	if err != nil {
		return CanonicalInputCoordinate{}, ProjectionInputCoordinate{}, err
	}
	canonical, err := NewCanonicalInputCoordinate(ref, bundle.Digest())
	if err != nil {
		return CanonicalInputCoordinate{}, ProjectionInputCoordinate{}, err
	}
	item, err := NewProjectionInputCoordinate(ref, bundle.Digest())
	if err != nil {
		return CanonicalInputCoordinate{}, ProjectionInputCoordinate{}, err
	}
	return canonical, item, nil
}

func adaptGoldenRoot(
	bundle goldenconcernbundle.Bundle,
	postures ExactPostureSet,
	input ProjectionInputCoordinate,
	profile ProjectionProfileDefinition,
) (RootProjectionSource, error) {
	var rootItem goldenconcernbundle.BundleItem
	rootFound := false
	for _, item := range bundle.Items() {
		if item.Role() != goldenconcernbundle.ItemEntityOfConcern {
			continue
		}
		if rootFound {
			return RootProjectionSource{}, fmt.Errorf(
				"GoldenConcernBundle repeats EntityOfConcern root",
			)
		}
		rootItem = item
		rootFound = true
	}
	if !rootFound {
		return RootProjectionSource{}, fmt.Errorf(
			"GoldenConcernBundle has no EntityOfConcern root",
		)
	}
	itemPostures, found := postures.posturesFor(rootItem.Reference())
	if !found {
		return RootProjectionSource{}, fmt.Errorf(
			"GoldenConcernBundle root has no exact postures",
		)
	}
	coordinate, err := NewRootOutputCoordinate(rootItem.Reference())
	if err != nil {
		return RootProjectionSource{}, err
	}
	text, err := NewReadableItemText(rootItem.Label().String())
	if err != nil {
		return RootProjectionSource{}, err
	}
	root, err := NewProjectedRoot(
		coordinate,
		text,
		itemPostures,
		rootItem.Provenance(),
	)
	if err != nil {
		return RootProjectionSource{}, err
	}
	basis, err := NewDirectProjectionItemBasis(
		coordinate,
		[]ProjectionInputCoordinate{input},
		TransformFieldSelection,
		profile.IntentionalLosses(),
	)
	if err != nil {
		return RootProjectionSource{}, err
	}
	return NewRootProjectionSource(root, basis)
}

func adaptGoldenFacets(
	bundle goldenconcernbundle.Bundle,
	request NeighborhoodRequest,
	postures ExactPostureSet,
	input ProjectionInputCoordinate,
	profile ProjectionProfileDefinition,
) ([]FacetProjectionInput, error) {
	grouped := make(map[FacetKind][]ItemProjectionSource)
	for _, item := range bundle.Items() {
		itemKind, mapped := goldenRoleItemKind(item.Role())
		if !mapped {
			continue
		}
		facet, admitted := profile.FacetForItem(itemKind)
		if !admitted {
			continue
		}
		if !slices.Contains(request.View().RequestedFacets(), facet) {
			continue
		}
		source, err := adaptGoldenItem(
			bundle,
			item,
			itemKind,
			facet,
			postures,
			input,
			profile,
		)
		if err != nil {
			return nil, err
		}
		grouped[facet] = append(grouped[facet], source)
	}
	result := make(
		[]FacetProjectionInput,
		0,
		len(request.View().RequestedFacets()),
	)
	for _, facet := range request.View().RequestedFacets() {
		exact, err := NewExactFacetInput(
			facet,
			input,
			grouped[facet],
		)
		if err != nil {
			return nil, err
		}
		result = append(result, exact)
	}
	return result, nil
}

func adaptGoldenItem(
	bundle goldenconcernbundle.Bundle,
	item goldenconcernbundle.BundleItem,
	itemKind ItemKind,
	facet FacetKind,
	postures ExactPostureSet,
	input ProjectionInputCoordinate,
	profile ProjectionProfileDefinition,
) (ItemProjectionSource, error) {
	itemPostures, found := postures.posturesFor(item.Reference())
	if !found {
		return ItemProjectionSource{}, fmt.Errorf(
			"GoldenConcernBundle item %q has no exact postures",
			persistedReferenceKey(item.Reference()),
		)
	}
	coordinate, err := NewFacetOutputCoordinate(facet, item.Reference())
	if err != nil {
		return ItemProjectionSource{}, err
	}
	text, err := NewReadableItemText(item.Label().String())
	if err != nil {
		return ItemProjectionSource{}, err
	}
	witnesses, err := goldenItemWitnesses(bundle, item.Reference())
	if err != nil {
		return ItemProjectionSource{}, err
	}
	projected, err := NewNeighborhoodItem(
		coordinate,
		itemKind,
		text,
		itemPostures,
		item.Provenance(),
		witnesses,
	)
	if err != nil {
		return ItemProjectionSource{}, err
	}
	basis, err := NewDirectProjectionItemBasis(
		coordinate,
		[]ProjectionInputCoordinate{input},
		TransformFieldSelection,
		profile.IntentionalLosses(),
	)
	if err != nil {
		return ItemProjectionSource{}, err
	}
	return NewItemProjectionSource(projected, basis)
}

func goldenItemWitnesses(
	bundle goldenconcernbundle.Bundle,
	reference typedmemory.PersistedRef,
) ([]RelationPathWitness, error) {
	result := make([]RelationPathWitness, 0)
	for _, path := range bundle.ExpectedRelationPaths() {
		if path.Target() != reference {
			continue
		}
		witness, err := NewRelationPathWitness(
			path.Assertion(),
			path.Signature(),
			path.Context(),
			path.Slot(),
			path.Target(),
			path.Provenance(),
			path.AdmissionEventRef(),
		)
		if err != nil {
			return nil, err
		}
		result = append(result, witness)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf(
			"GoldenConcernBundle item %q has no exact inclusion path",
			persistedReferenceKey(reference),
		)
	}
	return canonicalRelationWitnesses(result), nil
}

func goldenRoleItemKind(
	role goldenconcernbundle.ItemRole,
) (ItemKind, bool) {
	values := map[goldenconcernbundle.ItemRole]ItemKind{
		goldenconcernbundle.ItemProblemCard:              ItemProblemCard,
		goldenconcernbundle.ItemSolutionOption:           ItemSolutionOption,
		goldenconcernbundle.ItemSolutionPortfolio:        ItemSolutionPortfolio,
		goldenconcernbundle.ItemPortfolioComparison:      ItemPortfolioComparison,
		goldenconcernbundle.ItemDecisionRecord:           ItemDecisionRecord,
		goldenconcernbundle.ItemSpecSection:              ItemSpecSection,
		goldenconcernbundle.ItemProjectClaim:             ItemProjectClaim,
		goldenconcernbundle.ItemEvidenceRecord:           ItemEvidenceRecord,
		goldenconcernbundle.ItemSupportingEpistemeRecord: ItemSupportingEpisteme,
		goldenconcernbundle.ItemWorkRecord:               ItemWorkRecord,
		goldenconcernbundle.ItemPerformedWorkOccurrence:  ItemPerformedWorkOccurrence,
		goldenconcernbundle.ItemCodeAnchor:               ItemCodeAnchor,
	}
	value, found := values[role]
	return value, found
}

func persistedReferenceKey(reference typedmemory.PersistedRef) string {
	return reference.RefKind().String() +
		"/reference/" +
		reference.ReferenceID().String()
}
