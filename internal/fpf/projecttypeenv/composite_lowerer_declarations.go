package projecttypeenv

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const compositeDeclarationLoweringRulePrefix = "haft.projecttypeenv.composite-lower."

func lowerCompositeDeclarations(
	linked LinkedProjectTypeEnvCompositeIR,
	target typedmemory.TypeEnvRef,
	base typedmemory.TypeEnv,
	sources []compositeSourceDeclaration,
) (compositeLoweredDeclarations, error) {
	provenance := func(
		source compositeSourceDeclaration,
		semantic string,
	) (typedmemory.ProjectSourceProvenance, error) {
		return compositeDeclarationProvenance(linked, source, semantic)
	}

	contexts, err := lowerCompositeBoundedContexts(sources, provenance)
	if err != nil {
		return compositeLoweredDeclarations{}, err
	}
	kinds, err := lowerCompositeKindDefinitions(sources, provenance)
	if err != nil {
		return compositeLoweredDeclarations{}, err
	}
	subkinds, err := lowerCompositeSubkindRelations(linked, sources, provenance)
	if err != nil {
		return compositeLoweredDeclarations{}, err
	}
	refKinds, err := lowerCompositeRefKindDefinitions(target, sources, provenance)
	if err != nil {
		return compositeLoweredDeclarations{}, err
	}
	entitySets, entitySetsBySymbol, err := lowerCompositeEntitySetDefinitions(
		target,
		sources,
		provenance,
	)
	if err != nil {
		return compositeLoweredDeclarations{}, err
	}
	kindSignatures, err := lowerCompositeKindSignatureDefinitions(
		target,
		sources,
		entitySetsBySymbol,
		provenance,
	)
	if err != nil {
		return compositeLoweredDeclarations{}, err
	}
	classificationSignatures, err := lowerCompositeKindClassificationSignatureDefinitions(
		target,
		sources,
		provenance,
	)
	if err != nil {
		return compositeLoweredDeclarations{}, err
	}
	inheritedShapeRefs := make(map[string]typedmemory.ValueShapeRef)
	for _, declaration := range base.ValueShapes() {
		inheritedShapeRefs[declaration.Ref().ID().String()] = declaration.Ref()
	}
	shapes, shapeRefs, err := lowerCompositeValueShapes(
		sources,
		inheritedShapeRefs,
		provenance,
	)
	if err != nil {
		return compositeLoweredDeclarations{}, err
	}
	bindings, err := lowerCompositeValueBindings(
		target,
		sources,
		shapeRefs,
		provenance,
	)
	if err != nil {
		return compositeLoweredDeclarations{}, err
	}
	fragments, constraints, err := lowerCompositeRelationsAndConstraints(
		target,
		sources,
		provenance,
	)
	if err != nil {
		return compositeLoweredDeclarations{}, err
	}
	bridges, err := lowerCompositeContextBridges(sources)
	if err != nil {
		return compositeLoweredDeclarations{}, err
	}
	return compositeLoweredDeclarations{
		contexts:                 contexts,
		kinds:                    kinds,
		entitySets:               entitySets,
		kindSignatures:           kindSignatures,
		classificationSignatures: classificationSignatures,
		refKinds:                 refKinds,
		subkinds:                 subkinds,
		bridges:                  bridges,
		relationFragments:        fragments,
		shapes:                   shapes,
		bindings:                 bindings,
		constraints:              constraints,
	}, nil
}

func lowerCompositeBoundedContexts(
	sources []compositeSourceDeclaration,
	provenance func(compositeSourceDeclaration, string) (typedmemory.ProjectSourceProvenance, error),
) ([]typedmemory.BoundedContext, error) {
	byExtension := make(map[string]int)
	result := make([]typedmemory.BoundedContext, 0)
	for _, source := range declarationsOfKind(sources, localpractice.DeclarationBoundedContext) {
		byExtension[source.extension.Ref().String()]++
		context, err := typedmemory.NewBoundedContextRef(source.value.Symbol().Value())
		if err != nil {
			return nil, compositeDeclarationError(source, "bounded context", err)
		}
		basis, err := provenance(source, "bounded-context:"+context.String())
		if err != nil {
			return nil, compositeDeclarationError(source, "bounded-context provenance", err)
		}
		definition, err := typedmemory.NewBoundedContext(context, basis)
		if err != nil {
			return nil, compositeDeclarationError(source, "bounded context", err)
		}
		result = append(result, definition)
	}
	for _, extension := range uniqueCompositeExtensions(sources) {
		count := byExtension[extension.Ref().String()]
		if count != 1 {
			return nil, fmt.Errorf(
				"extension %q has %d explicit bounded_context declarations; final lowering requires exactly one",
				extension.Ref().String(),
				count,
			)
		}
	}
	return result, nil
}

func lowerCompositeKindDefinitions(
	sources []compositeSourceDeclaration,
	provenance func(compositeSourceDeclaration, string) (typedmemory.ProjectSourceProvenance, error),
) ([]typedmemory.KindDefinition, error) {
	result := make([]typedmemory.KindDefinition, 0)
	for _, source := range declarationsOfKind(sources, localpractice.DeclarationValueKind) {
		id, err := typedmemory.NewKindID(source.value.Symbol().Value())
		if err != nil {
			return nil, compositeDeclarationError(source, "ValueKind ID", err)
		}
		basis, err := provenance(source, "value-kind:"+id.String())
		if err != nil {
			return nil, compositeDeclarationError(source, "ValueKind provenance", err)
		}
		definition, err := typedmemory.NewKindDefinition(id, basis)
		if err != nil {
			return nil, compositeDeclarationError(source, "ValueKind", err)
		}
		result = append(result, definition)
	}
	return result, nil
}

func lowerCompositeSubkindRelations(
	linked LinkedProjectTypeEnvCompositeIR,
	sources []compositeSourceDeclaration,
	provenance func(compositeSourceDeclaration, string) (typedmemory.ProjectSourceProvenance, error),
) ([]typedmemory.SubkindRelation, error) {
	result := make([]typedmemory.SubkindRelation, 0)
	for _, source := range declarationsOfKind(sources, localpractice.DeclarationSubkind) {
		childSource, err := requiredDeclarationFact(source.value, "child_kind")
		if err != nil {
			return nil, compositeDeclarationError(source, "subkind child", err)
		}
		superSource, err := requiredDeclarationFact(source.value, "super_kind")
		if err != nil {
			return nil, compositeDeclarationError(source, "subkind superkind", err)
		}
		child, err := typedmemory.NewKindID(childSource.Value())
		if err != nil {
			return nil, compositeDeclarationError(source, "subkind child", err)
		}
		superkind, err := typedmemory.NewKindID(superSource.Value())
		if err != nil {
			return nil, compositeDeclarationError(source, "subkind superkind", err)
		}
		if _, err := compositeSubkindManifestBasis(linked, source); err != nil {
			return nil, compositeDeclarationError(source, "subkind manifest basis", err)
		}
		basis, err := provenance(
			source,
			"subkind:"+child.String()+":"+superkind.String(),
		)
		if err != nil {
			return nil, compositeDeclarationError(source, "subkind provenance", err)
		}
		relation, err := typedmemory.NewSubkindRelation(child, superkind, basis)
		if err != nil {
			return nil, compositeDeclarationError(source, "subkind relation", err)
		}
		result = append(result, relation)
	}
	return result, nil
}

func lowerCompositeRefKindDefinitions(
	target typedmemory.TypeEnvRef,
	sources []compositeSourceDeclaration,
	provenance func(compositeSourceDeclaration, string) (typedmemory.ProjectSourceProvenance, error),
) ([]typedmemory.RefKindDefinition, error) {
	result := make([]typedmemory.RefKindDefinition, 0)
	for _, source := range declarationsOfKind(sources, localpractice.DeclarationRefKind) {
		id, err := typedmemory.NewRefKindID(source.value.Symbol().Value())
		if err != nil {
			return nil, compositeDeclarationError(source, "RefKind ID", err)
		}
		ref, err := typedmemory.NewRefKindRef(target, id)
		if err != nil {
			return nil, compositeDeclarationError(source, "RefKind reference", err)
		}
		valueSource, err := requiredDeclarationFact(source.value, "value_kind")
		if err != nil {
			return nil, compositeDeclarationError(source, "RefKind ValueKind", err)
		}
		valueID, err := typedmemory.NewKindID(valueSource.Value())
		if err != nil {
			return nil, compositeDeclarationError(source, "RefKind ValueKind", err)
		}
		valueRef, err := typedmemory.NewValueKindRef(target, valueID)
		if err != nil {
			return nil, compositeDeclarationError(source, "RefKind ValueKind reference", err)
		}
		basis, err := provenance(source, "ref-kind:"+id.String())
		if err != nil {
			return nil, compositeDeclarationError(source, "RefKind provenance", err)
		}
		definition, err := typedmemory.NewRefKindDefinition(ref, valueRef, basis)
		if err != nil {
			return nil, compositeDeclarationError(source, "RefKind", err)
		}
		result = append(result, definition)
	}
	return result, nil
}

func lowerCompositeEntitySetDefinitions(
	target typedmemory.TypeEnvRef,
	sources []compositeSourceDeclaration,
	provenance func(compositeSourceDeclaration, string) (typedmemory.ProjectSourceProvenance, error),
) (
	[]typedmemory.EntitySetDefinition,
	map[string]typedmemory.EntitySetDefinition,
	error,
) {
	result := make([]typedmemory.EntitySetDefinition, 0)
	bySymbol := make(map[string]typedmemory.EntitySetDefinition)
	for _, source := range declarationsOfKind(sources, localpractice.DeclarationEntitySet) {
		ir := source.extension.Artifact().IR()
		context, err := typedmemory.NewBoundedContextRef(ir.BoundedContext().Value())
		if err != nil {
			return nil, nil, compositeDeclarationError(source, "EntitySet context", err)
		}
		ruleSource, err := requiredDeclarationFact(source.value, "enumeration_rule")
		if err != nil {
			return nil, nil, compositeDeclarationError(source, "EntitySet enumeration rule", err)
		}
		rule, err := typedmemory.NewRuleRef(ruleSource.Value())
		if err != nil {
			return nil, nil, compositeDeclarationError(source, "EntitySet enumeration rule", err)
		}
		policy, err := compositeEntitySetPolicy(source)
		if err != nil {
			return nil, nil, err
		}
		basis, err := provenance(source, "entity-set:"+source.value.Symbol().Value())
		if err != nil {
			return nil, nil, compositeDeclarationError(source, "EntitySet provenance", err)
		}
		definition, err := typedmemory.NewEntitySetDefinition(typedmemory.EntitySetDefinitionInput{
			TypeEnv:         target,
			Context:         context,
			EnumerationRule: rule,
			CandidatePolicy: policy,
			Provenance:      basis,
		})
		if err != nil {
			return nil, nil, compositeDeclarationError(source, "EntitySet", err)
		}
		result = append(result, definition)
		bySymbol[source.value.Symbol().Value()] = definition
	}
	return result, bySymbol, nil
}

func compositeEntitySetPolicy(
	source compositeSourceDeclaration,
) (typedmemory.EntitySetCandidatePolicy, error) {
	policySource, err := requiredDeclarationFact(source.value, "candidate_policy.kind")
	if err != nil {
		return nil, compositeDeclarationError(source, "EntitySet candidate policy", err)
	}
	switch localpractice.EntitySetPolicyKind(policySource.Value()) {
	case localpractice.EntitySetPersistedOnly:
		return typedmemory.PersistedEntitiesOnly{}, nil
	case localpractice.EntitySetPriorBatch:
		ruleSource, err := requiredDeclarationFact(
			source.value,
			"candidate_policy.evaluation_rule",
		)
		if err != nil {
			return nil, compositeDeclarationError(source, "EntitySet prior-batch rule", err)
		}
		rule, err := typedmemory.NewRuleRef(ruleSource.Value())
		if err != nil {
			return nil, compositeDeclarationError(source, "EntitySet prior-batch rule", err)
		}
		policy, err := typedmemory.NewPriorBatchDeclarationsVisible(rule)
		if err != nil {
			return nil, compositeDeclarationError(source, "EntitySet prior-batch policy", err)
		}
		return policy, nil
	default:
		return nil, compositeDeclarationError(
			source,
			"EntitySet candidate policy",
			fmt.Errorf("unsupported policy %q", policySource.Value()),
		)
	}
}

func lowerCompositeKindSignatureDefinitions(
	target typedmemory.TypeEnvRef,
	sources []compositeSourceDeclaration,
	entitySets map[string]typedmemory.EntitySetDefinition,
	provenance func(compositeSourceDeclaration, string) (typedmemory.ProjectSourceProvenance, error),
) ([]typedmemory.KindSignatureDefinition, error) {
	result := make([]typedmemory.KindSignatureDefinition, 0)
	for _, source := range declarationsOfKind(sources, localpractice.DeclarationKindSignature) {
		valueSource, err := requiredDeclarationFact(source.value, "value_kind")
		if err != nil {
			return nil, compositeDeclarationError(source, "KindSignature ValueKind", err)
		}
		kindID, err := typedmemory.NewKindID(valueSource.Value())
		if err != nil {
			return nil, compositeDeclarationError(source, "KindSignature ValueKind", err)
		}
		kind, err := typedmemory.NewValueKindRef(target, kindID)
		if err != nil {
			return nil, compositeDeclarationError(source, "KindSignature ValueKind reference", err)
		}
		formality, err := compositeSignatureFormality(source.value)
		if err != nil {
			return nil, compositeDeclarationError(source, "KindSignature formality", err)
		}
		assumptions, err := compositeKindAssumptions(source.value)
		if err != nil {
			return nil, compositeDeclarationError(source, "KindSignature assumptions", err)
		}
		definedness, err := compositeRuleFact(source.value, "definedness_rule")
		if err != nil {
			return nil, compositeDeclarationError(source, "KindSignature definedness", err)
		}
		evaluator, err := compositeRuleFact(source.value, "evaluator_rule")
		if err != nil {
			return nil, compositeDeclarationError(source, "KindSignature evaluator", err)
		}
		entitySetSource, err := requiredDeclarationFact(source.value, "entity_set")
		if err != nil {
			return nil, compositeDeclarationError(source, "KindSignature EntitySet", err)
		}
		entitySet, exists := entitySets[entitySetSource.Value()]
		if !exists {
			return nil, compositeDeclarationError(
				source,
				"KindSignature EntitySet",
				fmt.Errorf("resolved source symbol %q did not lower to an EntitySet definition", entitySetSource.Value()),
			)
		}
		basis, err := provenance(source, "kind-signature:"+source.value.Symbol().Value())
		if err != nil {
			return nil, compositeDeclarationError(source, "KindSignature provenance", err)
		}
		definition, err := typedmemory.NewKindSignatureDefinition(
			typedmemory.KindSignatureDefinitionInput{
				ValueKind:       kind,
				Formality:       formality,
				Assumptions:     assumptions,
				DefinednessRule: definedness,
				Evaluator:       evaluator,
				EntitySet:       entitySet.Ref(),
				Provenance:      basis,
			},
		)
		if err != nil {
			return nil, compositeDeclarationError(source, "KindSignature", err)
		}
		result = append(result, definition)
	}
	return result, nil
}

func lowerCompositeKindClassificationSignatureDefinitions(
	target typedmemory.TypeEnvRef,
	sources []compositeSourceDeclaration,
	provenance func(compositeSourceDeclaration, string) (typedmemory.ProjectSourceProvenance, error),
) ([]typedmemory.KindClassificationSignatureDefinition, error) {
	result := make([]typedmemory.KindClassificationSignatureDefinition, 0)
	for _, source := range declarationsOfKind(
		sources,
		localpractice.DeclarationKindClassificationSignature,
	) {
		localKind, err := compositeLocalKindRef(target, source)
		if err != nil {
			return nil, compositeDeclarationError(source, "current KindSignature local kind", err)
		}
		candidateValueKind, err := compositeValueKindFact(
			target,
			source.value,
			"candidate_value_kind",
		)
		if err != nil {
			return nil, compositeDeclarationError(source, "current KindSignature candidate ValueKind", err)
		}
		formality, err := compositeSignatureFormality(source.value)
		if err != nil {
			return nil, compositeDeclarationError(source, "current KindSignature formality", err)
		}
		criterion, err := compositeRuleFact(source.value, "criterion_rule")
		if err != nil {
			return nil, compositeDeclarationError(source, "current KindSignature criterion", err)
		}
		sliceConditions, err := compositeRuleFact(source.value, "slice_conditions_rule")
		if err != nil {
			return nil, compositeDeclarationError(source, "current KindSignature slice conditions", err)
		}
		referenceScheme, err := compositeKindReferenceScheme(source.value)
		if err != nil {
			return nil, compositeDeclarationError(source, "current KindSignature ReferenceScheme", err)
		}
		dependencies, err := compositeKindClassificationDependencies(source.value)
		if err != nil {
			return nil, compositeDeclarationError(source, "current KindSignature dependencies", err)
		}
		extentRule, err := compositeKindExtentRule(source.value)
		if err != nil {
			return nil, compositeDeclarationError(source, "current KindSignature ExtentRule", err)
		}
		basis, err := provenance(
			source,
			"kind-classification-signature:"+source.value.Symbol().Value(),
		)
		if err != nil {
			return nil, compositeDeclarationError(source, "current KindSignature provenance", err)
		}
		definition, err := typedmemory.NewKindClassificationSignatureDefinition(
			typedmemory.KindClassificationSignatureDefinitionInput{
				LocalKind:          localKind,
				CandidateValueKind: candidateValueKind,
				Criterion:          criterion,
				SliceConditions:    sliceConditions,
				ReferenceScheme:    referenceScheme,
				Dependencies:       dependencies,
				Formality:          formality,
				ExtentRule:         extentRule,
				Provenance:         basis,
			},
		)
		if err != nil {
			return nil, compositeDeclarationError(source, "current KindSignature", err)
		}
		result = append(result, definition)
	}
	return result, nil
}

func compositeLocalKindRef(
	target typedmemory.TypeEnvRef,
	source compositeSourceDeclaration,
) (typedmemory.LocalKindRef, error) {
	valueKind, err := compositeValueKindFact(target, source.value, "local_kind")
	if err != nil {
		return typedmemory.LocalKindRef{}, err
	}
	context, err := typedmemory.NewBoundedContextRef(
		source.extension.Artifact().IR().BoundedContext().Value(),
	)
	if err != nil {
		return typedmemory.LocalKindRef{}, err
	}
	return typedmemory.NewLocalKindRef(valueKind, context)
}

func compositeValueKindFact(
	target typedmemory.TypeEnvRef,
	declaration SymbolicDeclaration,
	path string,
) (typedmemory.ValueKindRef, error) {
	source, err := requiredDeclarationFact(declaration, path)
	if err != nil {
		return typedmemory.ValueKindRef{}, err
	}
	id, err := typedmemory.NewKindID(source.Value())
	if err != nil {
		return typedmemory.ValueKindRef{}, err
	}
	return typedmemory.NewValueKindRef(target, id)
}

func compositeKindReferenceScheme(
	declaration SymbolicDeclaration,
) (typedmemory.KindReferenceSchemePin, error) {
	return compositeKindReferenceSchemeAt(declaration, "reference_scheme")
}

func compositeKindReferenceSchemeAt(
	declaration SymbolicDeclaration,
	prefix string,
) (typedmemory.KindReferenceSchemePin, error) {
	carrierSource, err := requiredDeclarationFact(declaration, prefix+".carrier_ref")
	if err != nil {
		return typedmemory.KindReferenceSchemePin{}, err
	}
	editionSource, err := requiredDeclarationFact(declaration, prefix+".edition")
	if err != nil {
		return typedmemory.KindReferenceSchemePin{}, err
	}
	digestSource, err := requiredDeclarationFact(declaration, prefix+".digest")
	if err != nil {
		return typedmemory.KindReferenceSchemePin{}, err
	}
	carrier, err := typedmemory.NewCarrierRef(carrierSource.Value())
	if err != nil {
		return typedmemory.KindReferenceSchemePin{}, err
	}
	edition, err := typedmemory.NewCarrierEdition(editionSource.Value())
	if err != nil {
		return typedmemory.KindReferenceSchemePin{}, err
	}
	digest, err := typedmemory.NewSHA256Digest(digestSource.Value())
	if err != nil {
		return typedmemory.KindReferenceSchemePin{}, err
	}
	return typedmemory.NewKindReferenceSchemePin(carrier, edition, digest)
}

func compositeKindClassificationDependencies(
	declaration SymbolicDeclaration,
) ([]typedmemory.KindSignatureDependencyPin, error) {
	type parts struct {
		kind    string
		carrier string
		edition string
		digest  string
	}
	byIndex := make(map[int]parts)
	for _, fact := range declaration.Facts() {
		index, field, matches := parseCompositeIndexedRecordPath(fact.Path(), "dependencies")
		if !matches {
			continue
		}
		value := byIndex[index]
		switch field {
		case "kind":
			value.kind = fact.Value().Value()
		case "carrier_ref":
			value.carrier = fact.Value().Value()
		case "edition":
			value.edition = fact.Value().Value()
		case "digest":
			value.digest = fact.Value().Value()
		}
		byIndex[index] = value
	}
	result := make([]typedmemory.KindSignatureDependencyPin, 0, len(byIndex))
	for index := 0; index < len(byIndex); index++ {
		value, exists := byIndex[index]
		if !exists || value.kind == "" || value.carrier == "" ||
			value.edition == "" || value.digest == "" {
			return nil, fmt.Errorf("dependency indices are not dense or complete at %d", index)
		}
		kind, err := kindClassificationDependencyKind(value.kind)
		if err != nil {
			return nil, err
		}
		carrier, err := typedmemory.NewCarrierRef(value.carrier)
		if err != nil {
			return nil, err
		}
		edition, err := typedmemory.NewCarrierEdition(value.edition)
		if err != nil {
			return nil, err
		}
		digest, err := typedmemory.NewSHA256Digest(value.digest)
		if err != nil {
			return nil, err
		}
		pin, err := typedmemory.NewKindSignatureDependencyPin(
			kind,
			carrier,
			edition,
			digest,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, pin)
	}
	return result, nil
}

func compositeKindExtentRule(
	declaration SymbolicDeclaration,
) (typedmemory.KindExtentRuleOption, error) {
	kindSource, err := requiredDeclarationFact(declaration, "extent_rule.kind")
	if err != nil {
		return nil, err
	}
	switch localpractice.KindClassificationExtentRuleKind(kindSource.Value()) {
	case localpractice.KindClassificationNoExtentRule:
		return typedmemory.NoKindExtentRule{}, nil
	case localpractice.KindClassificationDeclaredExtentRule:
		rule, err := compositeRuleFact(declaration, "extent_rule.rule_ref")
		if err != nil {
			return nil, err
		}
		return typedmemory.NewDeclaredKindExtentRule(rule)
	default:
		return nil, fmt.Errorf("unsupported current KindSignature ExtentRule %q", kindSource.Value())
	}
}

func compositeSignatureFormality(
	declaration SymbolicDeclaration,
) (typedmemory.SignatureFormality, error) {
	source, err := requiredDeclarationFact(declaration, "formality")
	if err != nil {
		return 0, err
	}
	raw := strings.TrimPrefix(source.Value(), "F")
	value, err := strconv.ParseUint(raw, 10, 8)
	if err != nil {
		return 0, err
	}
	return typedmemory.NewSignatureFormality(uint8(value))
}

func compositeKindAssumptions(
	declaration SymbolicDeclaration,
) ([]typedmemory.KindAssumptionPin, error) {
	type parts struct {
		carrier string
		edition string
		digest  string
	}
	byIndex := make(map[int]parts)
	for _, fact := range declaration.Facts() {
		index, field, matches := parseCompositeIndexedRecordPath(fact.Path(), "assumptions")
		if !matches {
			continue
		}
		value := byIndex[index]
		switch field {
		case "carrier_ref":
			value.carrier = fact.Value().Value()
		case "edition":
			value.edition = fact.Value().Value()
		case "digest":
			value.digest = fact.Value().Value()
		}
		byIndex[index] = value
	}
	result := make([]typedmemory.KindAssumptionPin, 0, len(byIndex))
	for index := 0; index < len(byIndex); index++ {
		value, exists := byIndex[index]
		if !exists || value.carrier == "" || value.edition == "" || value.digest == "" {
			return nil, fmt.Errorf("assumption indices are not dense or complete at %d", index)
		}
		carrier, err := typedmemory.NewCarrierRef(value.carrier)
		if err != nil {
			return nil, err
		}
		edition, err := typedmemory.NewCarrierEdition(value.edition)
		if err != nil {
			return nil, err
		}
		digest, err := typedmemory.NewSHA256Digest(value.digest)
		if err != nil {
			return nil, err
		}
		pin, err := typedmemory.NewKindAssumptionPin(carrier, edition, digest)
		if err != nil {
			return nil, err
		}
		result = append(result, pin)
	}
	return result, nil
}

func compositeRuleFact(
	declaration SymbolicDeclaration,
	path string,
) (typedmemory.RuleRef, error) {
	source, err := requiredDeclarationFact(declaration, path)
	if err != nil {
		return typedmemory.RuleRef{}, err
	}
	return typedmemory.NewRuleRef(source.Value())
}

func lowerCompositeContextBridges(
	sources []compositeSourceDeclaration,
) ([]typedmemory.ContextBridge, error) {
	result := make([]typedmemory.ContextBridge, 0)
	for _, source := range declarationsOfKind(sources, localpractice.DeclarationKindBridge) {
		extension, err := contextKindAvailabilityExtensionFromLinked(source.extension)
		if err != nil {
			return nil, compositeDeclarationError(source, "KindBridge extension", err)
		}
		bridge, err := lowerContextKindAvailabilityBridge(extension, source.value)
		if err != nil {
			return nil, compositeDeclarationError(source, "KindBridge", err)
		}
		result = append(result, bridge)
	}
	return result, nil
}

func compositeDeclarationProvenance(
	linked LinkedProjectTypeEnvCompositeIR,
	source compositeSourceDeclaration,
	semantic string,
) (typedmemory.ProjectSourceProvenance, error) {
	ir := source.extension.Artifact().IR()
	reference, err := typedmemory.NewProvenanceRef(
		source.extension.Ref().String() + "#composite-lower:" + semantic,
	)
	if err != nil {
		return typedmemory.ProjectSourceProvenance{}, err
	}
	carrier, err := typedmemory.NewCarrierRef(ir.Carrier().ID().Value())
	if err != nil {
		return typedmemory.ProjectSourceProvenance{}, err
	}
	edition, err := typedmemory.NewCarrierEdition(ir.Carrier().Edition().Value())
	if err != nil {
		return typedmemory.ProjectSourceProvenance{}, err
	}
	lineRange, err := typedmemory.NewSourceLineRange(
		source.value.Span().Start(),
		source.value.Span().End(),
	)
	if err != nil {
		return typedmemory.ProjectSourceProvenance{}, err
	}
	rule, err := typedmemory.NewCompilerRuleID(
		compositeDeclarationLoweringRulePrefix + string(source.value.Kind()) + ".v1",
	)
	if err != nil {
		return typedmemory.ProjectSourceProvenance{}, err
	}
	context, err := typedmemory.NewBoundedContextRef(ir.BoundedContext().Value())
	if err != nil {
		return typedmemory.ProjectSourceProvenance{}, err
	}
	manifest := ir.Manifest()
	manifestRef, err := typedmemory.NewSignatureManifestRef(
		manifest.ID().Value(),
		manifest.Version().Value(),
	)
	if err != nil {
		return typedmemory.ProjectSourceProvenance{}, err
	}
	basis, err := compositeDeclarationManifestBasis(linked, source, manifestRef)
	if err != nil {
		return typedmemory.ProjectSourceProvenance{}, err
	}
	builder := typedmemory.NewProjectSourceProvenanceBuilder(
		reference,
		carrier,
		edition,
		ir.Carrier().Digest(),
	)
	builder = builder.SetDeclarationRange(lineRange)
	builder = builder.SetCompilerRule(rule)
	builder = builder.SetBoundedContext(context)
	builder = builder.SetBaseTypeEnv(ir.BaseTypeEnvRef())
	builder = builder.SetSignatureBlockRow(typedmemory.VocabularyRow)
	builder = builder.SetManifestBasis(basis)
	return builder.Build()
}

func compositeDeclarationManifestBasis(
	linked LinkedProjectTypeEnvCompositeIR,
	source compositeSourceDeclaration,
	manifest typedmemory.SignatureManifestRef,
) (typedmemory.ManifestSymbolBasis, error) {
	if source.value.Kind() == localpractice.DeclarationSubkind {
		return compositeSubkindManifestBasisWithManifest(linked, source, manifest)
	}
	symbol, err := extensionExportSymbol(source.value, source.value.Symbol().Value())
	if err != nil {
		return typedmemory.ManifestSymbolBasis{}, err
	}
	if !compositeSchemaSymbolContains(source.extension.Provides(), symbol) {
		return typedmemory.ManifestSymbolBasis{}, fmt.Errorf(
			"declaration symbol %q is not an exact manifest provide",
			symbol.String(),
		)
	}
	return typedmemory.NewManifestSymbolBasis(
		manifest,
		typedmemory.ManifestProvide,
		symbol,
	)
}

func compositeSubkindManifestBasis(
	linked LinkedProjectTypeEnvCompositeIR,
	source compositeSourceDeclaration,
) (typedmemory.ManifestSymbolBasis, error) {
	ir := source.extension.Artifact().IR()
	manifest, err := typedmemory.NewSignatureManifestRef(
		ir.Manifest().ID().Value(),
		ir.Manifest().Version().Value(),
	)
	if err != nil {
		return typedmemory.ManifestSymbolBasis{}, err
	}
	return compositeSubkindManifestBasisWithManifest(linked, source, manifest)
}

func compositeSubkindManifestBasisWithManifest(
	linked LinkedProjectTypeEnvCompositeIR,
	source compositeSourceDeclaration,
	manifest typedmemory.SignatureManifestRef,
) (typedmemory.ManifestSymbolBasis, error) {
	origin := "declaration:" + source.value.Symbol().Value()
	roles := []string{"child_kind", "super_kind"}
	for _, role := range roles {
		for _, resolution := range linked.DependencyResolutions() {
			if resolution.ConsumerRef() != source.extension.Ref() ||
				resolution.Origin() != origin ||
				resolution.Role() != role {
				continue
			}
			switch resolution.Scope() {
			case CompositeDependencyOwn:
				return typedmemory.NewManifestSymbolBasis(
					manifest,
					typedmemory.ManifestProvide,
					resolution.Target(),
				)
			case CompositeDependencyImported:
				return typedmemory.NewManifestSymbolBasis(
					manifest,
					typedmemory.ManifestImport,
					resolution.Target(),
				)
			case CompositeDependencyBase:
				continue
			default:
				continue
			}
		}
	}
	return typedmemory.ManifestSymbolBasis{}, fmt.Errorf(
		"subkind declaration has no exact local-provide or imported endpoint for provenance basis",
	)
}

func compositeSchemaSymbolContains(
	values []typedmemory.SchemaSymbolRef,
	want typedmemory.SchemaSymbolRef,
) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func uniqueCompositeExtensions(
	sources []compositeSourceDeclaration,
) []LinkedCompositeExtension {
	result := make([]LinkedCompositeExtension, 0)
	seen := make(map[string]struct{})
	for _, source := range sources {
		key := source.extension.Ref().String()
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, source.extension)
	}
	return result
}

func parseCompositeIndexedRecordPath(
	path string,
	prefix string,
) (int, string, bool) {
	start := prefix + "["
	if !strings.HasPrefix(path, start) {
		return 0, "", false
	}
	remaining := strings.TrimPrefix(path, start)
	indexText, fieldWithDot, found := strings.Cut(remaining, "]")
	if !found || !strings.HasPrefix(fieldWithDot, ".") {
		return 0, "", false
	}
	index, err := strconv.Atoi(indexText)
	if err != nil || index < 0 {
		return 0, "", false
	}
	return index, strings.TrimPrefix(fieldWithDot, "."), true
}

func compositeDeclarationError(
	source compositeSourceDeclaration,
	operation string,
	err error,
) error {
	return fmt.Errorf(
		"lower %s declaration %q from %s: %w",
		operation,
		source.value.Symbol().Value(),
		source.extension.Ref().String(),
		err,
	)
}
