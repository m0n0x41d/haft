package projectmemory

import (
	"fmt"
	"slices"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

type currentNeighborhoodRoleRule struct {
	signature string
	slot      string
	item      neighborhood.ItemKind
}

var currentNeighborhoodProjectRecordRoleRules = []currentNeighborhoodRoleRule{
	{
		signature: "Haft.NoteAtConcern",
		slot:      "Haft.NoteAtConcern.NoteSlot",
		item:      neighborhood.ItemNoteRecord,
	},
	{
		signature: "Haft.DecisionChoiceAtConcern",
		slot:      "Haft.DecisionChoiceAtConcern.ComparisonRecordSlot",
		item:      neighborhood.ItemPortfolioComparison,
	},
	{
		signature: "Haft.DecisionChoiceAtConcern",
		slot:      "Haft.DecisionChoiceAtConcern.OptionSlot",
		item:      neighborhood.ItemSolutionOption,
	},
	{
		signature: "Haft.DecisionChoiceAtConcern",
		slot:      "Haft.DecisionChoiceAtConcern.ChosenOptionSlot",
		item:      neighborhood.ItemSolutionOption,
	},
	{
		signature: "Haft.DecisionChoiceAtConcern",
		slot:      "Haft.DecisionChoiceAtConcern.RejectedOptionSlot",
		item:      neighborhood.ItemSolutionOption,
	},
	{
		signature: "Haft.DecisionChoiceAtConcern",
		slot:      "Haft.DecisionChoiceAtConcern.PortfolioRecordSlot",
		item:      neighborhood.ItemSolutionPortfolio,
	},
	{
		signature: "Haft.DecisionChoiceAtConcern",
		slot:      "Haft.DecisionChoiceAtConcern.ProblemRecordSlot",
		item:      neighborhood.ItemProblemCard,
	},
	{
		signature: "Haft.PortfolioComparison",
		slot:      "Haft.PortfolioComparison.ComparedOptionSlot",
		item:      neighborhood.ItemSolutionOption,
	},
	{
		signature: "Haft.PortfolioComparison",
		slot:      "Haft.PortfolioComparison.ComparisonSlot",
		item:      neighborhood.ItemPortfolioComparison,
	},
	{
		signature: "Haft.PortfolioComparison",
		slot:      "Haft.PortfolioComparison.NonDominatedOptionSlot",
		item:      neighborhood.ItemSolutionOption,
	},
	{
		signature: "Haft.PortfolioComparison",
		slot:      "Haft.PortfolioComparison.PortfolioSlot",
		item:      neighborhood.ItemSolutionPortfolio,
	},
	{
		signature: "Haft.ProblemCardAtConcern",
		slot:      "Haft.ProblemCardAtConcern.ProblemCardSlot",
		item:      neighborhood.ItemProblemCard,
	},
	{
		signature: "Haft.SolutionPortfolioAtConcern",
		slot:      "Haft.SolutionPortfolioAtConcern.OptionSlot",
		item:      neighborhood.ItemSolutionOption,
	},
	{
		signature: "Haft.SolutionPortfolioAtConcern",
		slot:      "Haft.SolutionPortfolioAtConcern.PortfolioSlot",
		item:      neighborhood.ItemSolutionPortfolio,
	},
}

type currentNeighborhoodReference struct {
	reference typedmemory.PersistedRef
	slot      typedmemory.SlotKindID
}

type currentNeighborhoodEdge struct {
	target  typedmemory.PersistedRef
	witness neighborhood.RelationPathWitness
	input   neighborhood.ProjectionInputCoordinate
}

type currentNeighborhoodPath struct {
	witnesses []neighborhood.RelationPathWitness
	inputs    []neighborhood.ProjectionInputCoordinate
}

type currentNeighborhoodGraph struct {
	roles     map[string][]neighborhood.ItemKind
	adjacency map[string][]currentNeighborhoodEdge
}

type currentNeighborhoodItemKey struct {
	facet     neighborhood.FacetKind
	reference string
}

// BuildCurrentNeighborhoodInput creates one conservative pure-assembler input
// from the exact current read frame. It traverses only admitted references
// connected by active relation instances in the requested bounded context.
// Relation co-membership is an inclusion path, never direction, causality,
// applicability, truth, authority, or Work order.
func BuildCurrentNeighborhoodInput(
	frame typedmemorystore.CurrentProjectReadFrame,
	request neighborhood.NeighborhoodRequest,
) (neighborhood.PinnedNeighborhoodInput, bool, error) {
	correlated, err := typedmemorystore.NewCurrentProjectReadFrame(
		frame.Snapshot(),
		frame.EntityDirectory(),
		frame.GraphObservation(),
	)
	if err != nil {
		return neighborhood.PinnedNeighborhoodInput{}, false, fmt.Errorf(
			"%w: %v",
			ErrCurrentResolutionFrameInvalid,
			err,
		)
	}
	if !request.Valid() {
		return neighborhood.PinnedNeighborhoodInput{}, false, fmt.Errorf(
			"current neighborhood request is invalid",
		)
	}
	snapshot, err := currentResolutionSnapshotBasis(
		correlated.EntityDirectory(),
	)
	if err != nil {
		return neighborhood.PinnedNeighborhoodInput{}, false, err
	}
	if request.TypeEnv() != snapshot.TypeEnv() ||
		request.GraphRevision() != snapshot.GraphRevision() {
		return neighborhood.PinnedNeighborhoodInput{}, false, fmt.Errorf(
			"current neighborhood request uses another snapshot",
		)
	}
	rootEntry, found := currentDirectoryEntry(
		correlated.EntityDirectory(),
		request.Entity(),
		request.Context(),
	)
	if !found {
		return neighborhood.PinnedNeighborhoodInput{}, false, nil
	}
	profile, found := neighborhood.LookupProjectionProfile(
		request.View().ProfileRef(),
	)
	if !found {
		return neighborhood.PinnedNeighborhoodInput{}, false, fmt.Errorf(
			"current neighborhood projection profile is unavailable",
		)
	}
	directoryCanonical, directoryInput, err :=
		currentDirectoryProjectionInput(correlated.EntityDirectory())
	if err != nil {
		return neighborhood.PinnedNeighborhoodInput{}, false, err
	}
	graphCanonical, graphInput, err := currentGraphProjectionInput(
		correlated.GraphObservation(),
	)
	if err != nil {
		return neighborhood.PinnedNeighborhoodInput{}, false, err
	}
	root, err := currentNeighborhoodRoot(
		request,
		rootEntry,
		directoryInput,
		profile,
	)
	if err != nil {
		return neighborhood.PinnedNeighborhoodInput{}, false, err
	}
	graph, relationCanonicals, err := currentNeighborhoodRelationGraph(
		correlated,
		request.Context(),
	)
	if err != nil {
		return neighborhood.PinnedNeighborhoodInput{}, false, err
	}
	paths := currentNeighborhoodPaths(request.Entity(), graph)
	facets, err := currentNeighborhoodFacets(
		correlated.EntityDirectory(),
		request,
		profile,
		graph,
		paths,
		directoryInput,
		graphInput,
	)
	if err != nil {
		return neighborhood.PinnedNeighborhoodInput{}, false, err
	}
	builder := neighborhood.NewPinnedNeighborhoodInputBuilder().
		SetRequest(request).
		SetSnapshot(snapshot).
		SetRoot(root).
		AddCanonicalInput(directoryCanonical).
		AddCanonicalInput(graphCanonical)
	for _, input := range relationCanonicals {
		builder.AddCanonicalInput(input)
	}
	for _, facet := range facets {
		builder.AddFacet(facet)
	}
	input, err := builder.Build()
	if err != nil {
		return neighborhood.PinnedNeighborhoodInput{}, false, fmt.Errorf(
			"build current neighborhood pinned input: %w",
			err,
		)
	}
	return input, true, nil
}

func currentDirectoryEntry(
	directory typedmemorystore.CurrentEntityDirectory,
	reference typedmemory.PersistedRef,
	context typedmemory.BoundedContextRef,
) (typedmemorystore.CurrentEntityDirectoryEntry, bool) {
	for _, entry := range directory.Entries() {
		if entry.Entity().String() != reference.ReferenceID().String() ||
			entry.Context() != context {
			continue
		}
		return entry, true
	}
	return typedmemorystore.CurrentEntityDirectoryEntry{}, false
}

func currentDirectoryProjectionInput(
	directory typedmemorystore.CurrentEntityDirectory,
) (
	neighborhood.CanonicalInputCoordinate,
	neighborhood.ProjectionInputCoordinate,
	error,
) {
	raw := "current-entity-directory:" + directory.ProjectID().String()
	return currentProjectionInput(raw, directory.Digest())
}

func currentGraphProjectionInput(
	graph typedmemorystore.CurrentProjectGraphObservation,
) (
	neighborhood.CanonicalInputCoordinate,
	neighborhood.ProjectionInputCoordinate,
	error,
) {
	basis := graph.GraphSnapshotBasis()
	raw := "current-project-graph:" + basis.Ref().String()
	return currentProjectionInput(raw, basis.Ref().Digest())
}

func currentRelationProjectionInput(
	relation typedmemorystore.CurrentActiveAssertion,
) (
	neighborhood.CanonicalInputCoordinate,
	neighborhood.ProjectionInputCoordinate,
	error,
) {
	raw := "current-active-relation:" + relation.AssertionID().String()
	return currentProjectionInput(raw, relation.Digest())
}

func currentProjectionInput(
	raw string,
	digest typedmemory.SHA256Digest,
) (
	neighborhood.CanonicalInputCoordinate,
	neighborhood.ProjectionInputCoordinate,
	error,
) {
	ref, err := neighborhood.NewProjectionInputRef(raw)
	if err != nil {
		return neighborhood.CanonicalInputCoordinate{},
			neighborhood.ProjectionInputCoordinate{},
			err
	}
	canonical, err := neighborhood.NewCanonicalInputCoordinate(ref, digest)
	if err != nil {
		return neighborhood.CanonicalInputCoordinate{},
			neighborhood.ProjectionInputCoordinate{},
			err
	}
	input, err := neighborhood.NewProjectionInputCoordinate(ref, digest)
	if err != nil {
		return neighborhood.CanonicalInputCoordinate{},
			neighborhood.ProjectionInputCoordinate{},
			err
	}
	return canonical, input, nil
}

func currentNeighborhoodRoot(
	request neighborhood.NeighborhoodRequest,
	entry typedmemorystore.CurrentEntityDirectoryEntry,
	input neighborhood.ProjectionInputCoordinate,
	profile neighborhood.ProjectionProfileDefinition,
) (neighborhood.RootProjectionSource, error) {
	coordinate, err := neighborhood.NewRootOutputCoordinate(request.Entity())
	if err != nil {
		return neighborhood.RootProjectionSource{}, err
	}
	text, err := neighborhood.NewReadableItemText(entry.Label().String())
	if err != nil {
		return neighborhood.RootProjectionSource{}, err
	}
	postures := currentNeighborhoodPostures()
	root, err := neighborhood.NewProjectedRoot(
		coordinate,
		text,
		postures,
		entry.Provenance(),
	)
	if err != nil {
		return neighborhood.RootProjectionSource{}, err
	}
	basis, err := neighborhood.NewDirectProjectionItemBasis(
		coordinate,
		[]neighborhood.ProjectionInputCoordinate{input},
		neighborhood.TransformFieldSelection,
		profile.IntentionalLosses(),
	)
	if err != nil {
		return neighborhood.RootProjectionSource{}, err
	}
	return neighborhood.NewRootProjectionSource(root, basis)
}

func currentNeighborhoodPostures() neighborhood.ItemPostures {
	postures, valid := neighborhood.NewItemPostures(
		neighborhood.SemanticTypedActive,
		neighborhood.LifecycleActive,
		neighborhood.EvidenceUnknown,
		neighborhood.ProjectionCurrent,
	)
	if !valid {
		panic("static current neighborhood postures are invalid")
	}
	return postures
}

func currentNeighborhoodRelationGraph(
	frame typedmemorystore.CurrentProjectReadFrame,
	context typedmemory.BoundedContextRef,
) (
	currentNeighborhoodGraph,
	[]neighborhood.CanonicalInputCoordinate,
	error,
) {
	relations := frame.GraphObservation().ActiveAssertions().Relations()
	referencesByAssertion := make(
		map[string][]currentNeighborhoodReference,
	)
	inputsByAssertion := make(
		map[string]neighborhood.ProjectionInputCoordinate,
	)
	canonicalInputs := make(
		[]neighborhood.CanonicalInputCoordinate,
		0,
		len(relations),
	)
	roles := make(map[string][]neighborhood.ItemKind)
	for _, active := range relations {
		relation := active.Carrier()
		if relation.Context() != context {
			continue
		}
		canonical, input, err := currentRelationProjectionInput(active)
		if err != nil {
			return currentNeighborhoodGraph{}, nil, err
		}
		references := currentRelationReferences(relation)
		key := active.AssertionID().String()
		referencesByAssertion[key] = references
		inputsByAssertion[key] = input
		canonicalInputs = append(canonicalInputs, canonical)
		currentNeighborhoodCollectRoles(relation, references, roles)
	}
	adjacency, err := currentNeighborhoodAdjacency(
		relations,
		context,
		referencesByAssertion,
		inputsByAssertion,
		roles,
	)
	if err != nil {
		return currentNeighborhoodGraph{}, nil, err
	}
	return currentNeighborhoodGraph{
		roles:     roles,
		adjacency: adjacency,
	}, canonicalInputs, nil
}

func currentRelationReferences(
	relation typedmemorystore.CurrentAssertionCarrier,
) []currentNeighborhoodReference {
	result := make([]currentNeighborhoodReference, 0)
	for _, binding := range relation.Bindings() {
		for _, filler := range binding.Fillers() {
			reference, ok := filler.(typedmemory.ReferenceFiller)
			if !ok {
				continue
			}
			result = append(result, currentNeighborhoodReference{
				reference: reference.Reference(),
				slot:      binding.Name(),
			})
		}
	}
	sort.Slice(result, func(left int, right int) bool {
		leftKey := currentNeighborhoodReferenceKey(result[left])
		rightKey := currentNeighborhoodReferenceKey(result[right])
		return leftKey < rightKey
	})
	return result
}

func currentNeighborhoodReferenceKey(
	value currentNeighborhoodReference,
) string {
	return currentPersistedReferenceKey(value.reference) +
		"|" +
		value.slot.String()
}

func currentNeighborhoodCollectRoles(
	relation typedmemorystore.CurrentAssertionCarrier,
	references []currentNeighborhoodReference,
	roles map[string][]neighborhood.ItemKind,
) {
	signature := relation.Signature().ID().String()
	for _, reference := range references {
		items := currentNeighborhoodRoles(
			signature,
			reference.slot.String(),
			reference.reference.RefKind().ID().String(),
		)
		key := currentNeighborhoodTraversalKey(reference.reference)
		roles[key] = canonicalCurrentNeighborhoodRoles(
			append(roles[key], items...),
		)
	}
}

func currentNeighborhoodRoles(
	signature string,
	slot string,
	refKind string,
) []neighborhood.ItemKind {
	if item, found := currentNeighborhoodRefKindRole(refKind); found {
		return []neighborhood.ItemKind{item}
	}
	if refKind != "Haft.ProjectRecordRef" {
		return nil
	}
	result := make([]neighborhood.ItemKind, 0)
	for _, rule := range currentNeighborhoodProjectRecordRoleRules {
		if rule.signature != signature || rule.slot != slot {
			continue
		}
		result = append(result, rule.item)
	}
	return canonicalCurrentNeighborhoodRoles(result)
}

func currentNeighborhoodRefKindRole(
	refKind string,
) (neighborhood.ItemKind, bool) {
	values := map[string]neighborhood.ItemKind{
		"Haft.CodeAnchorRef":              neighborhood.ItemCodeAnchor,
		"Haft.DecisionRecordRef":          neighborhood.ItemDecisionRecord,
		"Haft.EvidenceRecordRef":          neighborhood.ItemEvidenceRecord,
		"Haft.PerformedWorkOccurrenceRef": neighborhood.ItemPerformedWorkOccurrence,
		"Haft.ProjectClaimRef":            neighborhood.ItemProjectClaim,
		"Haft.SpecSectionRecordRef":       neighborhood.ItemSpecSection,
		"Haft.SupportingEpistemeRecordRef": neighborhood.
			ItemSupportingEpisteme,
		"Haft.WorkRecordRef": neighborhood.ItemWorkRecord,
	}
	item, found := values[refKind]
	return item, found
}

func canonicalCurrentNeighborhoodRoles(
	values []neighborhood.ItemKind,
) []neighborhood.ItemKind {
	result := append([]neighborhood.ItemKind{}, values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left] < result[right]
	})
	return slices.Compact(result)
}

func currentNeighborhoodAdjacency(
	relations []typedmemorystore.CurrentActiveAssertion,
	context typedmemory.BoundedContextRef,
	referencesByAssertion map[string][]currentNeighborhoodReference,
	inputsByAssertion map[string]neighborhood.ProjectionInputCoordinate,
	roles map[string][]neighborhood.ItemKind,
) (map[string][]currentNeighborhoodEdge, error) {
	result := make(map[string][]currentNeighborhoodEdge)
	seen := make(map[string]struct{})
	for _, active := range relations {
		relation := active.Carrier()
		if relation.Context() != context {
			continue
		}
		assertionKey := active.AssertionID().String()
		references := referencesByAssertion[assertionKey]
		input, found := inputsByAssertion[assertionKey]
		if !found {
			return nil, fmt.Errorf(
				"current neighborhood relation input is missing",
			)
		}
		for _, source := range references {
			sourceKey := currentNeighborhoodTraversalKey(source.reference)
			if !currentNeighborhoodTraversable(source.reference, roles) {
				continue
			}
			for _, target := range references {
				targetKey := currentNeighborhoodTraversalKey(target.reference)
				if sourceKey == targetKey ||
					!currentNeighborhoodTraversable(target.reference, roles) {
					continue
				}
				witness, err := currentNeighborhoodRelationPathWitness(
					active,
					relation,
					target,
				)
				if err != nil {
					return nil, err
				}
				edgeKey := sourceKey + "|" + witnessKey(witness)
				if _, found := seen[edgeKey]; found {
					continue
				}
				seen[edgeKey] = struct{}{}
				result[sourceKey] = append(
					result[sourceKey],
					currentNeighborhoodEdge{
						target:  target.reference,
						witness: witness,
						input:   input,
					},
				)
			}
		}
	}
	for key := range result {
		sort.Slice(result[key], func(left int, right int) bool {
			leftKey := currentNeighborhoodEdgeKey(result[key][left])
			rightKey := currentNeighborhoodEdgeKey(result[key][right])
			return leftKey < rightKey
		})
	}
	return result, nil
}

func currentNeighborhoodRelationPathWitness(
	active typedmemorystore.CurrentActiveAssertion,
	carrier typedmemorystore.CurrentAssertionCarrier,
	target currentNeighborhoodReference,
) (neighborhood.RelationPathWitness, error) {
	switch value := carrier.(type) {
	case typedmemorystore.CurrentLegacyRelation:
		return neighborhood.NewRelationPathWitness(
			carrier.AssertionID(),
			carrier.Signature().ID(),
			carrier.Context(),
			target.slot,
			target.reference,
			carrier.Provenance(),
			active.OriginEvent().String(),
		)
	case typedmemorystore.CurrentRelationalAssertion:
		return neighborhood.NewRelationalAssertionPathWitness(
			carrier.AssertionID(),
			carrier.Signature().ID(),
			carrier.Context(),
			target.slot,
			target.reference,
			carrier.Provenance(),
			active.OriginEvent().String(),
			value.Assertion().Modality().Kind(),
		)
	default:
		return neighborhood.RelationPathWitness{}, fmt.Errorf(
			"current neighborhood assertion carrier %T is unsupported",
			carrier,
		)
	}
}

func currentNeighborhoodTraversable(
	reference typedmemory.PersistedRef,
	roles map[string][]neighborhood.ItemKind,
) bool {
	if reference.RefKind().ID().String() == "U.EntityRef" {
		return true
	}
	key := currentNeighborhoodTraversalKey(reference)
	return len(roles[key]) > 0
}

func currentNeighborhoodPaths(
	root typedmemory.PersistedRef,
	graph currentNeighborhoodGraph,
) map[string]currentNeighborhoodPath {
	rootKey := currentNeighborhoodTraversalKey(root)
	paths := map[string]currentNeighborhoodPath{
		rootKey: {},
	}
	queue := []typedmemory.PersistedRef{root}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		currentKey := currentNeighborhoodTraversalKey(current)
		currentPath := paths[currentKey]
		for _, edge := range graph.adjacency[currentKey] {
			targetKey := currentNeighborhoodTraversalKey(edge.target)
			if _, found := paths[targetKey]; found {
				continue
			}
			next := currentNeighborhoodPath{
				witnesses: append(
					append(
						[]neighborhood.RelationPathWitness{},
						currentPath.witnesses...,
					),
					edge.witness,
				),
				inputs: append(
					append(
						[]neighborhood.ProjectionInputCoordinate{},
						currentPath.inputs...,
					),
					edge.input,
				),
			}
			next.inputs = canonicalCurrentProjectionInputs(next.inputs)
			paths[targetKey] = next
			queue = append(queue, edge.target)
		}
	}
	return paths
}

func currentNeighborhoodFacets(
	directory typedmemorystore.CurrentEntityDirectory,
	request neighborhood.NeighborhoodRequest,
	profile neighborhood.ProjectionProfileDefinition,
	graph currentNeighborhoodGraph,
	paths map[string]currentNeighborhoodPath,
	directoryInput neighborhood.ProjectionInputCoordinate,
	graphInput neighborhood.ProjectionInputCoordinate,
) ([]neighborhood.FacetProjectionInput, error) {
	grouped := make(
		map[neighborhood.FacetKind][]neighborhood.ItemProjectionSource,
	)
	seen := make(map[currentNeighborhoodItemKey]neighborhood.ItemKind)
	keys := make([]string, 0, len(paths))
	for key := range paths {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rootKey := currentNeighborhoodTraversalKey(request.Entity())
	for _, key := range keys {
		if key == rootKey {
			continue
		}
		path := paths[key]
		for _, itemKind := range graph.roles[key] {
			facet, admitted := profile.FacetForItem(itemKind)
			if !admitted ||
				!slices.Contains(request.View().RequestedFacets(), facet) {
				continue
			}
			historicalReference := path.witnesses[len(path.witnesses)-1].Target()
			reference, err := currentNeighborhoodProjectionReference(
				historicalReference,
				request.TypeEnv(),
			)
			if err != nil {
				return nil, err
			}
			itemKey := currentNeighborhoodItemKey{
				facet:     facet,
				reference: currentPersistedReferenceKey(reference),
			}
			if previous, found := seen[itemKey]; found &&
				previous != itemKind {
				return nil, fmt.Errorf(
					"current neighborhood reference has conflicting roles in facet %q",
					facet,
				)
			}
			if _, found := seen[itemKey]; found {
				continue
			}
			source, found, err := currentNeighborhoodItem(
				directory,
				request.Context(),
				reference,
				itemKind,
				facet,
				path,
				directoryInput,
				profile,
			)
			if err != nil {
				return nil, err
			}
			if !found {
				continue
			}
			seen[itemKey] = itemKind
			grouped[facet] = append(grouped[facet], source)
		}
	}
	result := make(
		[]neighborhood.FacetProjectionInput,
		0,
		len(request.View().RequestedFacets()),
	)
	for _, facet := range request.View().RequestedFacets() {
		input, err := neighborhood.NewExactFacetInput(
			facet,
			graphInput,
			grouped[facet],
		)
		if err != nil {
			return nil, err
		}
		result = append(result, input)
	}
	return result, nil
}

func currentNeighborhoodProjectionReference(
	reference typedmemory.PersistedRef,
	typeEnv typedmemory.TypeEnvRef,
) (typedmemory.PersistedRef, error) {
	refKind, err := typedmemory.NewRefKindRef(
		typeEnv,
		reference.RefKind().ID(),
	)
	if err != nil {
		return typedmemory.PersistedRef{}, err
	}
	projected, err := typedmemory.NewPersistedRef(
		refKind,
		reference.ReferenceID(),
	)
	if err != nil {
		return typedmemory.PersistedRef{}, err
	}
	return projected, nil
}

func currentNeighborhoodItem(
	directory typedmemorystore.CurrentEntityDirectory,
	context typedmemory.BoundedContextRef,
	reference typedmemory.PersistedRef,
	itemKind neighborhood.ItemKind,
	facet neighborhood.FacetKind,
	path currentNeighborhoodPath,
	directoryInput neighborhood.ProjectionInputCoordinate,
	profile neighborhood.ProjectionProfileDefinition,
) (neighborhood.ItemProjectionSource, bool, error) {
	entry, found := currentDirectoryEntry(directory, reference, context)
	if !found {
		return neighborhood.ItemProjectionSource{}, false, nil
	}
	coordinate, err := neighborhood.NewFacetOutputCoordinate(
		facet,
		reference,
	)
	if err != nil {
		return neighborhood.ItemProjectionSource{}, false, err
	}
	text, err := neighborhood.NewReadableItemText(entry.Label().String())
	if err != nil {
		return neighborhood.ItemProjectionSource{}, false, err
	}
	item, err := neighborhood.NewNeighborhoodItem(
		coordinate,
		itemKind,
		text,
		currentNeighborhoodPostures(),
		entry.Provenance(),
		path.witnesses,
	)
	if err != nil {
		return neighborhood.ItemProjectionSource{}, false, err
	}
	inputs := append(
		[]neighborhood.ProjectionInputCoordinate{directoryInput},
		path.inputs...,
	)
	inputs = canonicalCurrentProjectionInputs(inputs)
	basis, err := neighborhood.NewDirectProjectionItemBasis(
		coordinate,
		inputs,
		neighborhood.TransformFieldSelection,
		profile.IntentionalLosses(),
	)
	if err != nil {
		return neighborhood.ItemProjectionSource{}, false, err
	}
	source, err := neighborhood.NewItemProjectionSource(item, basis)
	if err != nil {
		return neighborhood.ItemProjectionSource{}, false, err
	}
	return source, true, nil
}

func canonicalCurrentProjectionInputs(
	values []neighborhood.ProjectionInputCoordinate,
) []neighborhood.ProjectionInputCoordinate {
	result := append([]neighborhood.ProjectionInputCoordinate{}, values...)
	sort.Slice(result, func(left int, right int) bool {
		leftKey := result[left].Ref().String() +
			"@" +
			result[left].Digest().String()
		rightKey := result[right].Ref().String() +
			"@" +
			result[right].Digest().String()
		return leftKey < rightKey
	})
	return slices.CompactFunc(
		result,
		func(
			left neighborhood.ProjectionInputCoordinate,
			right neighborhood.ProjectionInputCoordinate,
		) bool {
			return left.Ref() == right.Ref() &&
				left.Digest() == right.Digest()
		},
	)
}

func currentPersistedReferenceKey(
	reference typedmemory.PersistedRef,
) string {
	return reference.RefKind().String() +
		"/reference/" +
		reference.ReferenceID().String()
}

func currentNeighborhoodTraversalKey(
	reference typedmemory.PersistedRef,
) string {
	return reference.RefKind().ID().String() +
		"/reference/" +
		reference.ReferenceID().String()
}

func currentNeighborhoodEdgeKey(edge currentNeighborhoodEdge) string {
	return currentPersistedReferenceKey(edge.target) +
		"|" +
		witnessKey(edge.witness)
}

func witnessKey(witness neighborhood.RelationPathWitness) string {
	return witness.Assertion().String() +
		"|" +
		witness.Signature().String() +
		"|" +
		witness.Context().String() +
		"|" +
		witness.Slot().String() +
		"|" +
		currentPersistedReferenceKey(witness.Target()) +
		"|" +
		witness.Provenance().String() +
		"|" +
		witness.AdmissionEventRef()
}
