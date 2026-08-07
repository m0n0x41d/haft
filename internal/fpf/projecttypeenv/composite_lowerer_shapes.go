package projecttypeenv

import (
	"container/heap"
	"fmt"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	lowerCompositeValueShapeCompilerRule   = "haft.projecttypeenv.value-shape.v1"
	lowerCompositeCodecBindingCompilerRule = "haft.projecttypeenv.codec-binding.v1"
)

type lowerCompositeShapeNode struct {
	source       compositeSourceDeclaration
	dependencies []string
}

type lowerCompositeStringHeap []string

func (values lowerCompositeStringHeap) Len() int { return len(values) }

func (values lowerCompositeStringHeap) Less(left, right int) bool {
	return values[left] < values[right]
}

func (values lowerCompositeStringHeap) Swap(left, right int) {
	values[left], values[right] = values[right], values[left]
}

func (values *lowerCompositeStringHeap) Push(value any) {
	*values = append(*values, value.(string))
}

func (values *lowerCompositeStringHeap) Pop() any {
	owned := *values
	last := len(owned) - 1
	value := owned[last]
	*values = owned[:last]
	return value
}

func lowerCompositePartitionValueDeclarations(
	sources []compositeSourceDeclaration,
) (map[string]lowerCompositeShapeNode, []compositeSourceDeclaration, error) {
	shapes := make(map[string]lowerCompositeShapeNode)
	codecs := make([]compositeSourceDeclaration, 0)
	for _, source := range sources {
		declaration := source.value
		switch declaration.Kind() {
		case localpractice.DeclarationValueShape:
			symbol := declaration.Symbol().Value()
			if _, exists := shapes[symbol]; exists {
				return nil, nil, fmt.Errorf("lower value shapes: duplicate declaration %q", symbol)
			}
			dependencies, err := lowerCompositeShapeDependencies(declaration)
			if err != nil {
				return nil, nil, err
			}
			shapes[symbol] = lowerCompositeShapeNode{
				source:       source,
				dependencies: dependencies,
			}
		case localpractice.DeclarationCodecBinding:
			codecs = append(codecs, source)
		}
	}
	sort.Slice(codecs, func(left, right int) bool {
		return codecs[left].value.Symbol().Value() < codecs[right].value.Symbol().Value()
	})
	for index := 1; index < len(codecs); index++ {
		if codecs[index-1].value.Symbol().Value() == codecs[index].value.Symbol().Value() {
			return nil, nil, fmt.Errorf(
				"lower codec bindings: duplicate declaration %q",
				codecs[index].value.Symbol().Value(),
			)
		}
	}
	return shapes, codecs, nil
}

func lowerCompositeShapeDependencies(
	declaration SymbolicDeclaration,
) ([]string, error) {
	kindFact, err := requiredDeclarationFact(declaration, "shape.kind")
	if err != nil {
		return nil, err
	}
	dependencies := make([]string, 0)
	switch localpractice.ValueShapeKind(kindFact.Value()) {
	case localpractice.ValueShapeScalar, localpractice.ValueShapeClaimGraph:
	case localpractice.ValueShapeRecord:
		members, memberErr := lowerCompositeShapeMembers(declaration, "shape.fields")
		if memberErr != nil {
			return nil, memberErr
		}
		dependencies = lowerCompositeShapeMemberDependencies(members)
	case localpractice.ValueShapeSum:
		members, memberErr := lowerCompositeShapeMembers(declaration, "shape.variants")
		if memberErr != nil {
			return nil, memberErr
		}
		dependencies = lowerCompositeShapeMemberDependencies(members)
	case localpractice.ValueShapeOrderedSequence, localpractice.ValueShapeUnorderedSet:
		element, elementErr := requiredDeclarationFact(declaration, "shape.element")
		if elementErr != nil {
			return nil, elementErr
		}
		dependencies = append(dependencies, element.Value())
	default:
		return nil, fmt.Errorf(
			"lower value shape %q: unsupported shape kind %q",
			declaration.Symbol().Value(),
			kindFact.Value(),
		)
	}
	sort.Strings(dependencies)
	dependencies = lowerCompositeUniqueStrings(dependencies)
	return dependencies, nil
}

func lowerCompositeShapeMemberDependencies(
	members []lowerCompositeShapeMember,
) []string {
	result := make([]string, 0, len(members))
	for _, member := range members {
		result = append(result, member.shape)
	}
	return result
}

func lowerCompositeValueShapes(
	sources []compositeSourceDeclaration,
	inherited map[string]typedmemory.ValueShapeRef,
	provenance func(
		compositeSourceDeclaration,
		string,
	) (typedmemory.ProjectSourceProvenance, error),
) ([]typedmemory.ValueShapeDeclaration, map[string]typedmemory.ValueShapeRef, error) {
	if provenance == nil {
		return nil, nil, fmt.Errorf("lower value shapes: provenance constructor is required")
	}
	nodes, _, err := lowerCompositePartitionValueDeclarations(sources)
	if err != nil {
		return nil, nil, err
	}
	resolved := lowerCompositeCloneShapeRefs(inherited)
	for symbol := range nodes {
		if _, exists := resolved[symbol]; exists {
			return nil, nil, fmt.Errorf(
				"lower value shapes: declaration %q collides with an inherited shape",
				symbol,
			)
		}
	}
	missing := lowerCompositeMissingShapeDependencies(nodes, resolved)
	if len(missing) > 0 {
		return nil, nil, fmt.Errorf(
			"lower value shapes: missing child references: %s",
			strings.Join(missing, ", "),
		)
	}

	pending := make(map[string]lowerCompositeShapeNode, len(nodes))
	for symbol, node := range nodes {
		pending[symbol] = node
	}
	unresolved, dependents := lowerCompositeShapeDependencyIndex(nodes, resolved)
	ready := make(lowerCompositeStringHeap, 0)
	for symbol, count := range unresolved {
		if count == 0 {
			ready = append(ready, symbol)
		}
	}
	heap.Init(&ready)
	result := make([]typedmemory.ValueShapeDeclaration, 0, len(nodes))
	for ready.Len() > 0 {
		symbol := heap.Pop(&ready).(string)
		node := pending[symbol]
		declaration, ref, err := lowerCompositeValueShape(
			node.source,
			resolved,
			provenance,
		)
		if err != nil {
			return nil, nil, err
		}
		result = append(result, declaration)
		resolved[symbol] = ref
		delete(pending, symbol)
		for _, dependent := range dependents[symbol] {
			unresolved[dependent]--
			if unresolved[dependent] == 0 {
				heap.Push(&ready, dependent)
			}
		}
	}
	if len(pending) > 0 {
		cycle := lowerCompositeShapeCycleParticipants(pending)
		blocked := lowerCompositeShapeBlockedSymbols(pending, cycle)
		detail := strings.Join(cycle, ", ")
		if len(blocked) > 0 {
			detail += "; blocked dependents: " + strings.Join(blocked, ", ")
		}
		return nil, nil, fmt.Errorf(
			"lower value shapes: dependency cycle among: %s",
			detail,
		)
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].Ref().ID().String() < result[right].Ref().ID().String()
	})
	return result, resolved, nil
}

func lowerCompositeMissingShapeDependencies(
	nodes map[string]lowerCompositeShapeNode,
	resolved map[string]typedmemory.ValueShapeRef,
) []string {
	missing := make([]string, 0)
	for symbol, node := range nodes {
		for _, dependency := range node.dependencies {
			_, local := nodes[dependency]
			_, inherited := resolved[dependency]
			if local || inherited {
				continue
			}
			missing = append(missing, symbol+" -> "+dependency)
		}
	}
	sort.Strings(missing)
	return lowerCompositeUniqueStrings(missing)
}

func lowerCompositeShapeDependencyIndex(
	nodes map[string]lowerCompositeShapeNode,
	resolved map[string]typedmemory.ValueShapeRef,
) (map[string]int, map[string][]string) {
	unresolved := make(map[string]int, len(nodes))
	dependents := make(map[string][]string, len(nodes))
	for symbol, node := range nodes {
		for _, dependency := range node.dependencies {
			if _, exists := resolved[dependency]; exists {
				continue
			}
			unresolved[symbol]++
			dependents[dependency] = append(dependents[dependency], symbol)
		}
	}
	for symbol := range nodes {
		if _, exists := unresolved[symbol]; !exists {
			unresolved[symbol] = 0
		}
	}
	for dependency, values := range dependents {
		sort.Strings(values)
		dependents[dependency] = lowerCompositeUniqueStrings(values)
	}
	return unresolved, dependents
}

func lowerCompositeValueShape(
	sourceDeclaration compositeSourceDeclaration,
	resolved map[string]typedmemory.ValueShapeRef,
	provenance func(
		compositeSourceDeclaration,
		string,
	) (typedmemory.ProjectSourceProvenance, error),
) (typedmemory.ValueShapeDeclaration, typedmemory.ValueShapeRef, error) {
	declaration := sourceDeclaration.value
	shape, err := lowerCompositeValueShapePayload(declaration, resolved)
	if err != nil {
		return typedmemory.ValueShapeDeclaration{}, typedmemory.ValueShapeRef{}, err
	}
	shapeID, err := typedmemory.NewShapeID(declaration.Symbol().Value())
	if err != nil {
		return typedmemory.ValueShapeDeclaration{}, typedmemory.ValueShapeRef{}, fmt.Errorf(
			"lower value shape %q ID: %w",
			declaration.Symbol().Value(),
			err,
		)
	}
	ref, err := typedmemory.DeriveValueShapeRef(shapeID, shape)
	if err != nil {
		return typedmemory.ValueShapeDeclaration{}, typedmemory.ValueShapeRef{}, fmt.Errorf(
			"lower value shape %q identity: %w",
			declaration.Symbol().Value(),
			err,
		)
	}
	source, err := provenance(sourceDeclaration, lowerCompositeValueShapeCompilerRule)
	if err != nil {
		return typedmemory.ValueShapeDeclaration{}, typedmemory.ValueShapeRef{}, fmt.Errorf(
			"lower value shape %q provenance: %w",
			declaration.Symbol().Value(),
			err,
		)
	}
	result, err := typedmemory.NewValueShapeDeclaration(ref, shape, source)
	if err != nil {
		return typedmemory.ValueShapeDeclaration{}, typedmemory.ValueShapeRef{}, fmt.Errorf(
			"lower value shape %q declaration: %w",
			declaration.Symbol().Value(),
			err,
		)
	}
	return result, ref, nil
}

func lowerCompositeValueShapePayload(
	declaration SymbolicDeclaration,
	resolved map[string]typedmemory.ValueShapeRef,
) (typedmemory.ValueShape, error) {
	kindFact, err := requiredDeclarationFact(declaration, "shape.kind")
	if err != nil {
		return nil, err
	}
	switch localpractice.ValueShapeKind(kindFact.Value()) {
	case localpractice.ValueShapeScalar:
		return lowerCompositeScalarShape(declaration)
	case localpractice.ValueShapeRecord:
		return lowerCompositeRecordShape(declaration, resolved)
	case localpractice.ValueShapeSum:
		return lowerCompositeSumShape(declaration, resolved)
	case localpractice.ValueShapeOrderedSequence:
		return lowerCompositeOrderedShape(declaration, resolved)
	case localpractice.ValueShapeUnorderedSet:
		return lowerCompositeUnorderedShape(declaration, resolved)
	case localpractice.ValueShapeClaimGraph:
		return typedmemory.NewClaimGraphShape(), nil
	default:
		return nil, fmt.Errorf(
			"lower value shape %q: unsupported shape kind %q",
			declaration.Symbol().Value(),
			kindFact.Value(),
		)
	}
}

func lowerCompositeScalarShape(
	declaration SymbolicDeclaration,
) (typedmemory.ValueShape, error) {
	scalarFact, err := requiredDeclarationFact(declaration, "shape.scalar_kind")
	if err != nil {
		return nil, err
	}
	shape, err := typedmemory.NewScalarShape(typedmemory.ScalarKind(scalarFact.Value()))
	if err != nil {
		return nil, fmt.Errorf(
			"lower value shape %q scalar: %w",
			declaration.Symbol().Value(),
			err,
		)
	}
	return shape, nil
}

func lowerCompositeRecordShape(
	declaration SymbolicDeclaration,
	resolved map[string]typedmemory.ValueShapeRef,
) (typedmemory.ValueShape, error) {
	members, err := lowerCompositeShapeMembers(declaration, "shape.fields")
	if err != nil {
		return nil, err
	}
	fields := make([]typedmemory.RecordFieldShape, 0, len(members))
	for _, member := range members {
		name, nameErr := typedmemory.NewValueMemberName(member.name)
		if nameErr != nil {
			return nil, fmt.Errorf("lower value shape %q field: %w", declaration.Symbol().Value(), nameErr)
		}
		child := resolved[member.shape]
		field, fieldErr := typedmemory.NewRecordFieldShape(name, child)
		if fieldErr != nil {
			return nil, fmt.Errorf("lower value shape %q field %q: %w", declaration.Symbol().Value(), member.name, fieldErr)
		}
		fields = append(fields, field)
	}
	shape, err := typedmemory.NewRecordShape(fields)
	if err != nil {
		return nil, fmt.Errorf("lower value shape %q record: %w", declaration.Symbol().Value(), err)
	}
	return shape, nil
}

func lowerCompositeSumShape(
	declaration SymbolicDeclaration,
	resolved map[string]typedmemory.ValueShapeRef,
) (typedmemory.ValueShape, error) {
	members, err := lowerCompositeShapeMembers(declaration, "shape.variants")
	if err != nil {
		return nil, err
	}
	variants := make([]typedmemory.SumVariantShape, 0, len(members))
	for _, member := range members {
		name, nameErr := typedmemory.NewValueMemberName(member.name)
		if nameErr != nil {
			return nil, fmt.Errorf("lower value shape %q variant: %w", declaration.Symbol().Value(), nameErr)
		}
		child := resolved[member.shape]
		variant, variantErr := typedmemory.NewSumVariantShape(name, child)
		if variantErr != nil {
			return nil, fmt.Errorf("lower value shape %q variant %q: %w", declaration.Symbol().Value(), member.name, variantErr)
		}
		variants = append(variants, variant)
	}
	shape, err := typedmemory.NewSumShape(variants)
	if err != nil {
		return nil, fmt.Errorf("lower value shape %q sum: %w", declaration.Symbol().Value(), err)
	}
	return shape, nil
}

func lowerCompositeOrderedShape(
	declaration SymbolicDeclaration,
	resolved map[string]typedmemory.ValueShapeRef,
) (typedmemory.ValueShape, error) {
	element, err := lowerCompositeShapeElement(declaration, resolved)
	if err != nil {
		return nil, err
	}
	shape, err := typedmemory.NewOrderedSequenceShape(element)
	if err != nil {
		return nil, fmt.Errorf("lower value shape %q ordered sequence: %w", declaration.Symbol().Value(), err)
	}
	return shape, nil
}

func lowerCompositeUnorderedShape(
	declaration SymbolicDeclaration,
	resolved map[string]typedmemory.ValueShapeRef,
) (typedmemory.ValueShape, error) {
	element, err := lowerCompositeShapeElement(declaration, resolved)
	if err != nil {
		return nil, err
	}
	shape, err := typedmemory.NewUnorderedSetShape(element)
	if err != nil {
		return nil, fmt.Errorf("lower value shape %q unordered set: %w", declaration.Symbol().Value(), err)
	}
	return shape, nil
}

func lowerCompositeShapeElement(
	declaration SymbolicDeclaration,
	resolved map[string]typedmemory.ValueShapeRef,
) (typedmemory.ValueShapeRef, error) {
	elementFact, err := requiredDeclarationFact(declaration, "shape.element")
	if err != nil {
		return typedmemory.ValueShapeRef{}, err
	}
	element, exists := resolved[elementFact.Value()]
	if !exists {
		return typedmemory.ValueShapeRef{}, fmt.Errorf(
			"lower value shape %q: unresolved element %q",
			declaration.Symbol().Value(),
			elementFact.Value(),
		)
	}
	return element, nil
}

type lowerCompositeShapeMember struct {
	name  string
	shape string
}

func lowerCompositeShapeMembers(
	declaration SymbolicDeclaration,
	prefix string,
) ([]lowerCompositeShapeMember, error) {
	type partialMember struct {
		name     string
		shape    string
		hasName  bool
		hasShape bool
	}
	byKey := make(map[string]partialMember)
	for _, fact := range declaration.Facts() {
		key, leaf, matches := parseKeyedFactPath(fact.Path(), prefix)
		if !matches {
			continue
		}
		member := byKey[key]
		switch leaf {
		case "name":
			if member.hasName {
				return nil, fmt.Errorf("lower value shape %q: duplicate %s name %q", declaration.Symbol().Value(), prefix, key)
			}
			member.name = fact.Value().Value()
			member.hasName = true
		case "shape":
			if member.hasShape {
				return nil, fmt.Errorf("lower value shape %q: duplicate %s shape %q", declaration.Symbol().Value(), prefix, key)
			}
			member.shape = fact.Value().Value()
			member.hasShape = true
		default:
			return nil, fmt.Errorf("lower value shape %q: unknown %s field %q", declaration.Symbol().Value(), prefix, leaf)
		}
		byKey[key] = member
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]lowerCompositeShapeMember, 0, len(keys))
	for _, key := range keys {
		member := byKey[key]
		if !member.hasName || !member.hasShape {
			return nil, fmt.Errorf("lower value shape %q: incomplete %s member %q", declaration.Symbol().Value(), prefix, key)
		}
		if member.name != key {
			return nil, fmt.Errorf(
				"lower value shape %q: %s key %q differs from authored name %q",
				declaration.Symbol().Value(),
				prefix,
				key,
				member.name,
			)
		}
		result = append(result, lowerCompositeShapeMember{name: member.name, shape: member.shape})
	}
	return result, nil
}

func lowerCompositeValueBindings(
	target typedmemory.TypeEnvRef,
	sources []compositeSourceDeclaration,
	shapes map[string]typedmemory.ValueShapeRef,
	provenance func(
		compositeSourceDeclaration,
		string,
	) (typedmemory.ProjectSourceProvenance, error),
) ([]typedmemory.ValueBinding, error) {
	bindings, _, err := lowerCompositeValueBindingsWithSpecifications(
		target,
		sources,
		shapes,
		provenance,
	)
	return bindings, err
}

func lowerCompositeValueBindingsWithSpecifications(
	target typedmemory.TypeEnvRef,
	sources []compositeSourceDeclaration,
	shapes map[string]typedmemory.ValueShapeRef,
	provenance func(
		compositeSourceDeclaration,
		string,
	) (typedmemory.ProjectSourceProvenance, error),
) ([]typedmemory.ValueBinding, []CodecSpecificationV1, error) {
	verifiedTarget, err := typedmemory.ParseTypeEnvRef(target.String())
	if err != nil || verifiedTarget != target {
		return nil, nil, fmt.Errorf("lower codec bindings: target TypeEnvRef is invalid")
	}
	if provenance == nil {
		return nil, nil, fmt.Errorf("lower codec bindings: provenance constructor is required")
	}
	_, declarations, err := lowerCompositePartitionValueDeclarations(sources)
	if err != nil {
		return nil, nil, err
	}
	bindings := make([]typedmemory.ValueBinding, 0, len(declarations))
	specifications := make([]CodecSpecificationV1, 0, len(declarations))
	boundKinds := make(map[string]string)
	for _, sourceDeclaration := range declarations {
		declaration := sourceDeclaration.value
		binding, specification, valueKind, err := lowerCompositeCodecBinding(
			sourceDeclaration,
			verifiedTarget,
			shapes,
			provenance,
		)
		if err != nil {
			return nil, nil, err
		}
		if previous, exists := boundKinds[valueKind.String()]; exists {
			return nil, nil, fmt.Errorf(
				"lower codec binding %q: value kind %q is already bound by %q",
				declaration.Symbol().Value(),
				valueKind.String(),
				previous,
			)
		}
		boundKinds[valueKind.String()] = declaration.Symbol().Value()
		bindings = append(bindings, binding)
		specifications = append(specifications, specification)
	}
	sort.Slice(bindings, func(left, right int) bool {
		return bindings[left].ValueKind().String() < bindings[right].ValueKind().String()
	})
	sort.Slice(specifications, func(left, right int) bool {
		return specifications[left].Ref().String() < specifications[right].Ref().String()
	})
	return bindings, specifications, nil
}

func lowerCompositeCodecBinding(
	sourceDeclaration compositeSourceDeclaration,
	target typedmemory.TypeEnvRef,
	shapes map[string]typedmemory.ValueShapeRef,
	provenance func(
		compositeSourceDeclaration,
		string,
	) (typedmemory.ProjectSourceProvenance, error),
) (typedmemory.ValueBinding, CodecSpecificationV1, typedmemory.ValueKindRef, error) {
	declaration := sourceDeclaration.value
	valueKindFact, err := requiredDeclarationFact(declaration, "value_kind")
	if err != nil {
		return typedmemory.ValueBinding{}, CodecSpecificationV1{}, typedmemory.ValueKindRef{}, err
	}
	kindID, err := typedmemory.NewKindID(valueKindFact.Value())
	if err != nil {
		return typedmemory.ValueBinding{}, CodecSpecificationV1{}, typedmemory.ValueKindRef{}, fmt.Errorf("lower codec binding %q value kind: %w", declaration.Symbol().Value(), err)
	}
	valueKind, err := typedmemory.NewValueKindRef(target, kindID)
	if err != nil {
		return typedmemory.ValueBinding{}, CodecSpecificationV1{}, typedmemory.ValueKindRef{}, fmt.Errorf("lower codec binding %q value-kind reference: %w", declaration.Symbol().Value(), err)
	}
	specification, shape, err := deriveCompositeCodecSpecification(
		sourceDeclaration,
		shapes,
	)
	if err != nil {
		return typedmemory.ValueBinding{}, CodecSpecificationV1{}, typedmemory.ValueKindRef{}, err
	}
	source, err := provenance(sourceDeclaration, lowerCompositeCodecBindingCompilerRule)
	if err != nil {
		return typedmemory.ValueBinding{}, CodecSpecificationV1{}, typedmemory.ValueKindRef{}, fmt.Errorf("lower codec binding %q provenance: %w", declaration.Symbol().Value(), err)
	}
	binding, err := typedmemory.NewValueBinding(valueKind, shape, specification.Ref(), source)
	if err != nil {
		return typedmemory.ValueBinding{}, CodecSpecificationV1{}, typedmemory.ValueKindRef{}, fmt.Errorf("lower codec binding %q: %w", declaration.Symbol().Value(), err)
	}
	return binding, specification, valueKind, nil
}

// deriveCompositeCodecSpecification recovers the content-addressed runtime
// codec coordinate from source declarations and already-derived value shapes.
// It deliberately accepts no TypeEnvRef: codec requirements are authored by B
// and E before X exists, while a ValueBinding is materialized only after the
// final composite C has been derived.
func deriveCompositeCodecSpecification(
	sourceDeclaration compositeSourceDeclaration,
	shapes map[string]typedmemory.ValueShapeRef,
) (CodecSpecificationV1, typedmemory.ValueShapeRef, error) {
	declaration := sourceDeclaration.value
	shapeFact, err := requiredDeclarationFact(declaration, "value_shape")
	if err != nil {
		return CodecSpecificationV1{}, typedmemory.ValueShapeRef{}, err
	}
	shape, exists := shapes[shapeFact.Value()]
	if !exists {
		return CodecSpecificationV1{}, typedmemory.ValueShapeRef{}, fmt.Errorf("lower codec binding %q: unresolved value shape %q", declaration.Symbol().Value(), shapeFact.Value())
	}
	codecID, err := typedmemory.NewCodecID(declaration.Symbol().Value())
	if err != nil {
		return CodecSpecificationV1{}, typedmemory.ValueShapeRef{}, fmt.Errorf("lower codec binding %q codec ID: %w", declaration.Symbol().Value(), err)
	}
	versionFact, err := requiredDeclarationFact(declaration, "canonicalization_version")
	if err != nil {
		return CodecSpecificationV1{}, typedmemory.ValueShapeRef{}, err
	}
	version, err := typedmemory.NewCanonicalizationVersion(versionFact.Value())
	if err != nil {
		return CodecSpecificationV1{}, typedmemory.ValueShapeRef{}, fmt.Errorf("lower codec binding %q canonicalization version: %w", declaration.Symbol().Value(), err)
	}
	contractFacts, err := indexedDeclarationFacts(declaration, "contract")
	if err != nil {
		return CodecSpecificationV1{}, typedmemory.ValueShapeRef{}, err
	}
	contract := make([]string, 0, len(contractFacts))
	for _, fact := range contractFacts {
		contract = append(contract, fact.Value())
	}
	specification, err := DeriveCodecSpecificationV1(codecID, version, shape, contract)
	if err != nil {
		return CodecSpecificationV1{}, typedmemory.ValueShapeRef{}, fmt.Errorf("lower codec binding %q specification: %w", declaration.Symbol().Value(), err)
	}
	if err := specification.Verify(); err != nil {
		return CodecSpecificationV1{}, typedmemory.ValueShapeRef{}, fmt.Errorf("lower codec binding %q verify specification: %w", declaration.Symbol().Value(), err)
	}
	return specification, shape, nil
}

func lowerCompositeSortedShapeNodeKeys(
	values map[string]lowerCompositeShapeNode,
) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func lowerCompositeShapeCycleParticipants(
	pending map[string]lowerCompositeShapeNode,
) []string {
	adjacency := lowerCompositeShapeAdjacency(pending)
	finishOrder := lowerCompositeShapeFinishOrder(adjacency)
	reverse := lowerCompositeReverseShapeAdjacency(adjacency)
	assigned := make(map[string]struct{}, len(pending))
	participants := make([]string, 0)
	for index := len(finishOrder) - 1; index >= 0; index-- {
		symbol := finishOrder[index]
		if _, exists := assigned[symbol]; exists {
			continue
		}
		component := lowerCompositeCollectShapeComponent(symbol, reverse, assigned)
		if len(component) > 1 || lowerCompositeShapeHasSelfEdge(component[0], adjacency) {
			participants = append(participants, component...)
		}
	}
	sort.Strings(participants)
	return participants
}

func lowerCompositeShapeAdjacency(
	pending map[string]lowerCompositeShapeNode,
) map[string][]string {
	result := make(map[string][]string, len(pending))
	for symbol, node := range pending {
		dependencies := make([]string, 0, len(node.dependencies))
		for _, dependency := range node.dependencies {
			if _, exists := pending[dependency]; exists {
				dependencies = append(dependencies, dependency)
			}
		}
		sort.Strings(dependencies)
		result[symbol] = lowerCompositeUniqueStrings(dependencies)
	}
	return result
}

type lowerCompositeShapeDFSFrame struct {
	symbol string
	next   int
}

func lowerCompositeShapeFinishOrder(
	adjacency map[string][]string,
) []string {
	visited := make(map[string]struct{}, len(adjacency))
	result := make([]string, 0, len(adjacency))
	for _, start := range lowerCompositeSortedStringSliceMapKeys(adjacency) {
		if _, exists := visited[start]; exists {
			continue
		}
		visited[start] = struct{}{}
		stack := []lowerCompositeShapeDFSFrame{{symbol: start}}
		for len(stack) > 0 {
			last := len(stack) - 1
			frame := stack[last]
			neighbors := adjacency[frame.symbol]
			if frame.next < len(neighbors) {
				neighbor := neighbors[frame.next]
				stack[last].next++
				if _, exists := visited[neighbor]; exists {
					continue
				}
				visited[neighbor] = struct{}{}
				stack = append(stack, lowerCompositeShapeDFSFrame{symbol: neighbor})
				continue
			}
			result = append(result, frame.symbol)
			stack = stack[:last]
		}
	}
	return result
}

func lowerCompositeReverseShapeAdjacency(
	adjacency map[string][]string,
) map[string][]string {
	result := make(map[string][]string, len(adjacency))
	for symbol := range adjacency {
		result[symbol] = nil
	}
	for symbol, dependencies := range adjacency {
		for _, dependency := range dependencies {
			result[dependency] = append(result[dependency], symbol)
		}
	}
	for symbol, dependencies := range result {
		sort.Strings(dependencies)
		result[symbol] = lowerCompositeUniqueStrings(dependencies)
	}
	return result
}

func lowerCompositeCollectShapeComponent(
	start string,
	reverse map[string][]string,
	assigned map[string]struct{},
) []string {
	assigned[start] = struct{}{}
	stack := []string{start}
	result := make([]string, 0)
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		result = append(result, current)
		neighbors := reverse[current]
		for index := len(neighbors) - 1; index >= 0; index-- {
			neighbor := neighbors[index]
			if _, exists := assigned[neighbor]; exists {
				continue
			}
			assigned[neighbor] = struct{}{}
			stack = append(stack, neighbor)
		}
	}
	sort.Strings(result)
	return result
}

func lowerCompositeShapeHasSelfEdge(
	symbol string,
	adjacency map[string][]string,
) bool {
	neighbors := adjacency[symbol]
	index := sort.SearchStrings(neighbors, symbol)
	return index < len(neighbors) && neighbors[index] == symbol
}

func lowerCompositeShapeBlockedSymbols(
	pending map[string]lowerCompositeShapeNode,
	cycle []string,
) []string {
	cycleSet := make(map[string]struct{}, len(cycle))
	for _, symbol := range cycle {
		cycleSet[symbol] = struct{}{}
	}
	blocked := make([]string, 0)
	for _, symbol := range lowerCompositeSortedShapeNodeKeys(pending) {
		if _, participates := cycleSet[symbol]; !participates {
			blocked = append(blocked, symbol)
		}
	}
	return blocked
}

func lowerCompositeCloneShapeRefs(
	values map[string]typedmemory.ValueShapeRef,
) map[string]typedmemory.ValueShapeRef {
	result := make(map[string]typedmemory.ValueShapeRef, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func lowerCompositeSortedStringSliceMapKeys(
	values map[string][]string,
) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func lowerCompositeUniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if len(result) > 0 && result[len(result)-1] == value {
			continue
		}
		result = append(result, value)
	}
	return result
}
