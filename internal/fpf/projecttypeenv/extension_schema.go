package projecttypeenv

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// symbolicDeclarationSchema proves that an untrusted canonical declaration is
// one of the closed Local-Practice source variants. It does not resolve names
// or lower them into C-bound runtime references.
type symbolicDeclarationSchema struct {
	declaration     SymbolicDeclaration
	facts           map[string]SourceScalar
	dependencies    map[string]SourceScalar
	expectedFacts   map[string]struct{}
	expectedDeps    map[string]SourceScalar
	expectedExports map[string]SourceScalar
}

func newSymbolicDeclarationSchema(
	declaration SymbolicDeclaration,
) symbolicDeclarationSchema {
	facts := make(map[string]SourceScalar, len(declaration.facts))
	for _, fact := range declaration.facts {
		facts[fact.path] = fact.value
	}
	dependencies := make(map[string]SourceScalar, len(declaration.dependencies))
	for _, dependency := range declaration.dependencies {
		dependencies[dependency.role] = dependency.target
	}
	expectedExports := make(map[string]SourceScalar)
	if declaration.kind != localpractice.DeclarationSubkind {
		expectedExports[declaration.symbol.value] = declaration.symbol
	}
	return symbolicDeclarationSchema{
		declaration:     declaration,
		facts:           facts,
		dependencies:    dependencies,
		expectedFacts:   make(map[string]struct{}),
		expectedDeps:    make(map[string]SourceScalar),
		expectedExports: expectedExports,
	}
}

func (schema *symbolicDeclarationSchema) requireFact(path string) (SourceScalar, error) {
	value, exists := schema.facts[path]
	if !exists {
		return SourceScalar{}, fmt.Errorf(
			"symbolic declaration %q requires source fact %q",
			schema.declaration.symbol.value,
			path,
		)
	}
	schema.expectedFacts[path] = struct{}{}
	return value, nil
}

func (schema *symbolicDeclarationSchema) admitFact(path string) {
	schema.expectedFacts[path] = struct{}{}
}

func (schema *symbolicDeclarationSchema) expectDependency(
	role string,
	target SourceScalar,
) {
	schema.expectedDeps[role] = target
}

func (schema *symbolicDeclarationSchema) expectExport(value SourceScalar) {
	schema.expectedExports[value.value] = value
}

func (schema symbolicDeclarationSchema) finish() error {
	for _, path := range sortedSourceScalarKeys(schema.facts) {
		if _, expected := schema.expectedFacts[path]; !expected {
			return fmt.Errorf(
				"symbolic declaration %q contains source fact %q outside the %s schema",
				schema.declaration.symbol.value,
				path,
				schema.declaration.kind,
			)
		}
	}
	for _, role := range sortedSourceScalarKeys(schema.dependencies) {
		expected, exists := schema.expectedDeps[role]
		if !exists {
			return fmt.Errorf(
				"symbolic declaration %q contains dependency role %q outside the %s schema",
				schema.declaration.symbol.value,
				role,
				schema.declaration.kind,
			)
		}
		if schema.dependencies[role] != expected {
			return fmt.Errorf(
				"symbolic declaration %q dependency %q does not exactly match its source fact",
				schema.declaration.symbol.value,
				role,
			)
		}
	}
	for _, role := range sortedSourceScalarKeys(schema.expectedDeps) {
		if _, exists := schema.dependencies[role]; !exists {
			return fmt.Errorf(
				"symbolic declaration %q requires dependency role %q",
				schema.declaration.symbol.value,
				role,
			)
		}
	}
	actualExports := make(map[string]SourceScalar, len(schema.declaration.exports))
	for _, exported := range schema.declaration.exports {
		actualExports[exported.value] = exported
	}
	for _, symbol := range sortedSourceScalarKeys(actualExports) {
		expected, exists := schema.expectedExports[symbol]
		if !exists {
			return fmt.Errorf(
				"symbolic declaration %q exports %q outside the %s schema",
				schema.declaration.symbol.value,
				symbol,
				schema.declaration.kind,
			)
		}
		if actualExports[symbol] != expected {
			return fmt.Errorf(
				"symbolic declaration %q export %q does not exactly match its source symbol",
				schema.declaration.symbol.value,
				symbol,
			)
		}
	}
	for _, symbol := range sortedSourceScalarKeys(schema.expectedExports) {
		if _, exists := actualExports[symbol]; !exists {
			return fmt.Errorf(
				"symbolic declaration %q requires export %q",
				schema.declaration.symbol.value,
				symbol,
			)
		}
	}
	return nil
}

func validateSymbolicDeclarationSchema(declaration SymbolicDeclaration) error {
	if err := validateDeclarationSymbol(declaration.kind, declaration.symbol.value); err != nil {
		return fmt.Errorf("symbolic declaration %q: %w", declaration.symbol.value, err)
	}
	schema := newSymbolicDeclarationSchema(declaration)
	var err error
	switch declaration.kind {
	case localpractice.DeclarationBoundedContext:
		err = validateBoundedContextDeclarationSchema(&schema)
	case localpractice.DeclarationValueKind:
		err = validateValueKindDeclarationSchema(&schema)
	case localpractice.DeclarationSubkind:
		err = validateSubkindDeclarationSchema(&schema)
	case localpractice.DeclarationRefKind:
		err = validateRefKindDeclarationSchema(&schema)
	case localpractice.DeclarationEntitySet:
		err = validateEntitySetDeclarationSchema(&schema)
	case localpractice.DeclarationKindSignature:
		err = validateKindSignatureDeclarationSchema(&schema)
	case localpractice.DeclarationKindClassificationSignature:
		err = validateKindClassificationSignatureDeclarationSchema(&schema)
	case localpractice.DeclarationRelationSignature:
		err = validateRelationSignatureDeclarationSchema(&schema)
	case localpractice.DeclarationRuntimeEvaluatorInput:
		err = validateRuntimeEvaluatorInputDeclarationSchema(&schema)
	case localpractice.DeclarationValueShape:
		err = validateValueShapeDeclarationSchema(&schema)
	case localpractice.DeclarationCodecBinding:
		err = validateCodecBindingDeclarationSchema(&schema)
	case localpractice.DeclarationRuntimeEvaluatorRequirement:
		err = validateRuntimeEvaluatorRequirementDeclarationSchema(&schema)
	case localpractice.DeclarationConstraint:
		err = validateConstraintDeclarationSchema(&schema)
	case localpractice.DeclarationKindBridge:
		err = validateKindBridgeDeclarationSchema(&schema)
	default:
		err = fmt.Errorf("unsupported symbolic declaration kind %q", declaration.kind)
	}
	if err != nil {
		return err
	}
	return schema.finish()
}

func validateBoundedContextDeclarationSchema(schema *symbolicDeclarationSchema) error {
	contextRef, err := typedmemory.NewBoundedContextRef(schema.declaration.symbol.value)
	if err != nil || contextRef.String() != schema.declaration.symbol.value {
		return fmt.Errorf("bounded_context symbol is not a canonical BoundedContextRef")
	}
	return nil
}

func validateSubkindDeclarationSchema(schema *symbolicDeclarationSchema) error {
	childKind, err := schema.requireFact("child_kind")
	if err != nil {
		return err
	}
	superKind, err := schema.requireFact("super_kind")
	if err != nil {
		return err
	}
	parsedChild, err := typedmemory.NewKindID(childKind.value)
	if err != nil || parsedChild.String() != childKind.value {
		return fmt.Errorf("subkind child_kind %q is not a canonical KindID", childKind.value)
	}
	parsedSuper, err := typedmemory.NewKindID(superKind.value)
	if err != nil || parsedSuper.String() != superKind.value {
		return fmt.Errorf("subkind super_kind %q is not a canonical KindID", superKind.value)
	}
	if parsedChild == parsedSuper {
		return fmt.Errorf("subkind child_kind and super_kind must be distinct")
	}
	schema.expectDependency("child_kind", childKind)
	schema.expectDependency("super_kind", superKind)
	return nil
}

func validateKindBridgeDeclarationSchema(schema *symbolicDeclarationSchema) error {
	bridgeID, err := typedmemory.NewContextBridgeID(schema.declaration.symbol.value)
	if err != nil || bridgeID.String() != schema.declaration.symbol.value {
		return fmt.Errorf("kind_bridge symbol is not a canonical ContextBridgeID")
	}
	sourceContext, err := schema.requireFact("endpoints.source.bounded_context_ref")
	if err != nil {
		return err
	}
	targetContext, err := schema.requireFact("endpoints.target.bounded_context_ref")
	if err != nil {
		return err
	}
	parsedSourceContext, err := typedmemory.NewBoundedContextRef(sourceContext.value)
	if err != nil || parsedSourceContext.String() != sourceContext.value {
		return fmt.Errorf("kind_bridge source bounded context is invalid")
	}
	parsedTargetContext, err := typedmemory.NewBoundedContextRef(targetContext.value)
	if err != nil || parsedTargetContext.String() != targetContext.value {
		return fmt.Errorf("kind_bridge target bounded context is invalid")
	}
	if parsedSourceContext == parsedTargetContext {
		return fmt.Errorf("kind_bridge endpoints must name distinct bounded contexts")
	}
	sourceEdition, err := schema.requireFact("endpoints.source.edition")
	if err != nil {
		return err
	}
	targetEdition, err := schema.requireFact("endpoints.target.edition")
	if err != nil {
		return err
	}
	if !validExactKindBridgeEdition(sourceEdition.value) ||
		!validExactKindBridgeEdition(targetEdition.value) {
		return fmt.Errorf("kind_bridge endpoint editions must be exact and must not use latest/current/head selectors")
	}
	mappingKind, err := schema.requireFact("mapping.kind")
	if err != nil {
		return err
	}
	if localpractice.KindBridgeMappingKind(mappingKind.value) != localpractice.KindBridgeNamedTarget {
		return fmt.Errorf("kind_bridge mapping kind %q is unsupported", mappingKind.value)
	}
	sourceKind, err := schema.requireFact("mapping.source_kind")
	if err != nil {
		return err
	}
	targetKind, err := schema.requireFact("mapping.target_kind")
	if err != nil {
		return err
	}
	if err := validateKindBridgeKind(sourceKind.value, "source"); err != nil {
		return err
	}
	if err := validateKindBridgeKind(targetKind.value, "target"); err != nil {
		return err
	}
	schema.expectDependency("mapping.source_kind", sourceKind)
	schema.expectDependency("mapping.target_kind", targetKind)
	direction, err := schema.requireFact("direction")
	if err != nil {
		return err
	}
	if localpractice.KindBridgeDirectionKind(direction.value) != localpractice.KindBridgeOneWay &&
		localpractice.KindBridgeDirectionKind(direction.value) != localpractice.KindBridgeTwoWay {
		return fmt.Errorf("kind_bridge direction %q is unsupported", direction.value)
	}
	orderPreservation, err := schema.requireFact("order_preservation")
	if err != nil {
		return err
	}
	if localpractice.KindBridgeOrderPreservationKind(orderPreservation.value) !=
		localpractice.KindBridgeNoOrderLinksCovered {
		return fmt.Errorf("kind_bridge order preservation %q is unsupported", orderPreservation.value)
	}
	congruence, err := schema.requireFact("kind_congruence")
	if err != nil {
		return err
	}
	level, err := parseCanonicalUnsigned(congruence.value)
	if err != nil || level > 3 {
		return fmt.Errorf("kind_bridge kind_congruence %q is outside canonical CL^k 0..3", congruence.value)
	}
	lossNotes, err := collectIndexedScalarFacts(schema, "loss_notes")
	if err != nil {
		return err
	}
	if len(lossNotes) == 0 {
		return fmt.Errorf("kind_bridge requires at least one loss note")
	}
	if duplicate := duplicateScalarValue(lossNotes); duplicate != "" {
		return fmt.Errorf("kind_bridge repeats loss note %q", duplicate)
	}
	definednessArea, err := collectIndexedScalarFacts(schema, "definedness_area")
	if err != nil {
		return err
	}
	if len(definednessArea) == 0 {
		return fmt.Errorf("kind_bridge requires a nonempty definedness area")
	}
	if duplicate := duplicateScalarValue(definednessArea); duplicate != "" {
		return fmt.Errorf("kind_bridge repeats definedness condition %q", duplicate)
	}
	return nil
}

func validateKindBridgeKind(value string, endpoint string) error {
	id, err := typedmemory.NewKindID(value)
	if err != nil || id.String() != value {
		return fmt.Errorf("kind_bridge %s kind %q is not a canonical KindID", endpoint, value)
	}
	return nil
}

func validExactKindBridgeEdition(value string) bool {
	if value == "" {
		return false
	}
	switch strings.ToLower(value) {
	case "latest", "current", "head", "*":
		return false
	default:
		return true
	}
}

func validateValueKindDeclarationSchema(schema *symbolicDeclarationSchema) error {
	return validateValueKindName(schema.declaration.symbol.value)
}

func validateRefKindDeclarationSchema(schema *symbolicDeclarationSchema) error {
	if err := validateRefKindName(schema.declaration.symbol.value); err != nil {
		return err
	}
	valueKind, err := schema.requireFact("value_kind")
	if err != nil {
		return err
	}
	if err := validateValueKindName(valueKind.value); err != nil {
		return fmt.Errorf("ref_kind value_kind: %w", err)
	}
	schema.expectDependency("value_kind", valueKind)
	return nil
}

func validateEntitySetDeclarationSchema(schema *symbolicDeclarationSchema) error {
	enumerationRule, err := schema.requireFact("enumeration_rule")
	if err != nil {
		return err
	}
	policy, err := schema.requireFact("candidate_policy.kind")
	if err != nil {
		return err
	}
	schema.expectDependency("enumeration_rule", enumerationRule)
	switch localpractice.EntitySetPolicyKind(policy.value) {
	case localpractice.EntitySetPersistedOnly:
		return nil
	case localpractice.EntitySetPriorBatch:
		evaluationRule, factErr := schema.requireFact("candidate_policy.evaluation_rule")
		if factErr != nil {
			return factErr
		}
		schema.expectDependency("candidate_policy.evaluation_rule", evaluationRule)
		return nil
	default:
		return fmt.Errorf("entity_set_definition candidate policy %q is unsupported", policy.value)
	}
}

func validateKindSignatureDeclarationSchema(schema *symbolicDeclarationSchema) error {
	valueKind, err := schema.requireFact("value_kind")
	if err != nil {
		return err
	}
	if err := validateValueKindName(valueKind.value); err != nil {
		return fmt.Errorf("kind_signature_definition value_kind: %w", err)
	}
	formality, err := schema.requireFact("formality")
	if err != nil {
		return err
	}
	if !validSignatureFormality(formality.value) {
		return fmt.Errorf("kind_signature_definition formality %q is not F0..F9", formality.value)
	}
	definednessRule, err := schema.requireFact("definedness_rule")
	if err != nil {
		return err
	}
	evaluatorRule, err := schema.requireFact("evaluator_rule")
	if err != nil {
		return err
	}
	membershipBasis, err := schema.requireFact("membership_basis.kind")
	if err != nil {
		return err
	}
	switch localpractice.KindSignatureMembershipBasisKind(membershipBasis.value) {
	case localpractice.KindSignatureDirectObservableInputs:
	case localpractice.KindSignatureCarrierFirst:
		adapterRule, adapterErr := schema.requireFact("membership_basis.adapter_rule")
		if adapterErr != nil {
			return adapterErr
		}
		parsedAdapter, adapterErr := typedmemory.NewRuleRef(adapterRule.value)
		if adapterErr != nil || parsedAdapter.String() != adapterRule.value {
			return fmt.Errorf(
				"kind_signature_definition membership adapter %q is not a canonical RuleRef",
				adapterRule.value,
			)
		}
		schema.expectDependency("membership_basis.adapter_rule", adapterRule)
	default:
		return fmt.Errorf(
			"kind_signature_definition membership basis %q is unsupported",
			membershipBasis.value,
		)
	}
	entitySet, err := schema.requireFact("entity_set")
	if err != nil {
		return err
	}
	if !validQualifiedName(entitySet.value) {
		return fmt.Errorf("kind_signature_definition entity_set %q is not a qualified source name", entitySet.value)
	}
	schema.expectDependency("value_kind", valueKind)
	schema.expectDependency("definedness_rule", definednessRule)
	schema.expectDependency("evaluator_rule", evaluatorRule)
	schema.expectDependency("entity_set", entitySet)
	records, err := collectIndexedFactRecords(
		schema,
		"assumptions",
		[]string{"carrier_ref", "edition", "digest"},
	)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(records))
	for index, record := range records {
		digest := record["digest"]
		parsedDigest, digestErr := typedmemory.NewSHA256Digest(digest.value)
		if digestErr != nil || parsedDigest.String() != digest.value {
			return fmt.Errorf("kind_signature_definition assumption %d digest is invalid", index)
		}
		coordinate := record["carrier_ref"].value + "\x00" + record["edition"].value
		if _, exists := seen[coordinate]; exists {
			return fmt.Errorf("kind_signature_definition repeats assumption carrier edition %q", coordinate)
		}
		seen[coordinate] = struct{}{}
		path := indexedPath("assumptions", index) + ".carrier_ref"
		schema.expectDependency(path, record["carrier_ref"])
	}
	return nil
}

func validateKindClassificationSignatureDeclarationSchema(
	schema *symbolicDeclarationSchema,
) error {
	localKind, err := schema.requireFact("local_kind")
	if err != nil {
		return err
	}
	if err := validateValueKindName(localKind.value); err != nil {
		return fmt.Errorf("kind_classification_signature_definition local_kind: %w", err)
	}
	candidateValueKind, err := schema.requireFact("candidate_value_kind")
	if err != nil {
		return err
	}
	if err := validateValueKindName(candidateValueKind.value); err != nil {
		return fmt.Errorf(
			"kind_classification_signature_definition candidate_value_kind: %w",
			err,
		)
	}
	formality, err := schema.requireFact("formality")
	if err != nil {
		return err
	}
	if !validSignatureFormality(formality.value) {
		return fmt.Errorf(
			"kind_classification_signature_definition formality %q is not F0..F9",
			formality.value,
		)
	}
	criterion, err := requireCanonicalRuleFact(schema, "criterion_rule")
	if err != nil {
		return err
	}
	sliceConditions, err := requireCanonicalRuleFact(schema, "slice_conditions_rule")
	if err != nil {
		return err
	}
	referenceScheme, err := requireExactKindClassificationPin(
		schema,
		"reference_scheme",
	)
	if err != nil {
		return err
	}
	dependencies, err := collectIndexedFactRecords(
		schema,
		"dependencies",
		[]string{"kind", "carrier_ref", "edition", "digest"},
	)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(dependencies))
	for index, dependency := range dependencies {
		kind, kindErr := kindClassificationDependencyKind(dependency["kind"].value)
		if kindErr != nil {
			return fmt.Errorf(
				"kind_classification_signature_definition dependency %d: %w",
				index,
				kindErr,
			)
		}
		pin, pinErr := exactKindClassificationPin(dependency)
		if pinErr != nil {
			return fmt.Errorf(
				"kind_classification_signature_definition dependency %d: %w",
				index,
				pinErr,
			)
		}
		coordinate := kind.String() + "\x00" + pin.Reference().String()
		if _, exists := seen[coordinate]; exists {
			return fmt.Errorf(
				"kind_classification_signature_definition repeats dependency %q",
				coordinate,
			)
		}
		seen[coordinate] = struct{}{}
		path := indexedPath("dependencies", index) + ".carrier_ref"
		schema.expectDependency(path, dependency["carrier_ref"])
	}
	extentKind, err := schema.requireFact("extent_rule.kind")
	if err != nil {
		return err
	}
	switch localpractice.KindClassificationExtentRuleKind(extentKind.value) {
	case localpractice.KindClassificationNoExtentRule:
	case localpractice.KindClassificationDeclaredExtentRule:
		extentRule, ruleErr := requireCanonicalRuleFact(schema, "extent_rule.rule_ref")
		if ruleErr != nil {
			return ruleErr
		}
		schema.expectDependency("extent_rule.rule_ref", extentRule)
	default:
		return fmt.Errorf(
			"kind_classification_signature_definition extent rule %q is unsupported",
			extentKind.value,
		)
	}
	schema.expectDependency("local_kind", localKind)
	schema.expectDependency("candidate_value_kind", candidateValueKind)
	schema.expectDependency("criterion_rule", criterion)
	schema.expectDependency("slice_conditions_rule", sliceConditions)
	schema.expectDependency("reference_scheme.carrier_ref", referenceScheme["carrier_ref"])
	return nil
}

func requireCanonicalRuleFact(
	schema *symbolicDeclarationSchema,
	path string,
) (SourceScalar, error) {
	source, err := schema.requireFact(path)
	if err != nil {
		return SourceScalar{}, err
	}
	rule, err := typedmemory.NewRuleRef(source.value)
	if err != nil || rule.String() != source.value {
		return SourceScalar{}, fmt.Errorf(
			"kind_classification_signature_definition %s %q is not a canonical RuleRef",
			path,
			source.value,
		)
	}
	return source, nil
}

func requireExactKindClassificationPin(
	schema *symbolicDeclarationSchema,
	prefix string,
) (map[string]SourceScalar, error) {
	record := make(map[string]SourceScalar, 3)
	for _, field := range []string{"carrier_ref", "edition", "digest"} {
		value, err := schema.requireFact(prefix + "." + field)
		if err != nil {
			return nil, err
		}
		record[field] = value
	}
	if _, err := exactKindClassificationPin(record); err != nil {
		return nil, fmt.Errorf(
			"kind_classification_signature_definition %s: %w",
			prefix,
			err,
		)
	}
	return record, nil
}

func exactKindClassificationPin(
	record map[string]SourceScalar,
) (typedmemory.KindReferenceSchemePin, error) {
	reference, err := typedmemory.NewCarrierRef(record["carrier_ref"].value)
	if err != nil {
		return typedmemory.KindReferenceSchemePin{}, err
	}
	edition, err := typedmemory.NewCarrierEdition(record["edition"].value)
	if err != nil {
		return typedmemory.KindReferenceSchemePin{}, err
	}
	digest, err := typedmemory.NewSHA256Digest(record["digest"].value)
	if err != nil {
		return typedmemory.KindReferenceSchemePin{}, err
	}
	return typedmemory.NewKindReferenceSchemePin(reference, edition, digest)
}

func kindClassificationDependencyKind(
	raw string,
) (typedmemory.KindSignatureDependencyKind, error) {
	switch localpractice.KindClassificationDependencyKind(raw) {
	case localpractice.KindClassificationDependencyAssumption:
		return typedmemory.KindDependencyAssumption, nil
	case localpractice.KindClassificationDependencyExternal:
		return typedmemory.KindDependencyExternal, nil
	case localpractice.KindClassificationDependencyStandard:
		return typedmemory.KindDependencyStandard, nil
	case localpractice.KindClassificationDependencyVersion:
		return typedmemory.KindDependencyVersion, nil
	case localpractice.KindClassificationDependencyUnit:
		return typedmemory.KindDependencyUnit, nil
	case localpractice.KindClassificationDependencyTemporalPolicy:
		return typedmemory.KindDependencyTemporalPolicy, nil
	default:
		return 0, fmt.Errorf("dependency kind %q is unsupported", raw)
	}
}

func validateRelationSignatureDeclarationSchema(schema *symbolicDeclarationSchema) error {
	return validateSlottedDeclarationSchema(schema, "relation_signature")
}

func validateRuntimeEvaluatorInputDeclarationSchema(
	schema *symbolicDeclarationSchema,
) error {
	evaluatorRequirement, err := schema.requireFact("evaluator_requirement")
	if err != nil {
		return err
	}
	if !validQualifiedName(evaluatorRequirement.value) {
		return fmt.Errorf(
			"runtime_evaluator_input evaluator_requirement %q is not a qualified source name",
			evaluatorRequirement.value,
		)
	}
	schema.expectDependency("evaluator_requirement", evaluatorRequirement)
	return validateSlottedDeclarationSchema(schema, "runtime_evaluator_input")
}

func validateSlottedDeclarationSchema(
	schema *symbolicDeclarationSchema,
	declarationKind string,
) error {
	records, err := collectKeyedFactRecords(
		schema,
		"slots",
		[]string{"slot_kind", "value_kind", "ref_mode.kind"},
		[]string{"ref_mode.ref_kind"},
	)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return fmt.Errorf("%s requires at least one SlotSpec", declarationKind)
	}
	for _, record := range records {
		slotKind := record.facts["slot_kind"]
		if slotKind.value != record.key {
			return fmt.Errorf(
				"%s slot key %q does not match SlotKind %q",
				declarationKind,
				record.key,
				slotKind.value,
			)
		}
		if err := validateSlotKindName(slotKind.value); err != nil {
			return err
		}
		valueKind := record.facts["value_kind"]
		if err := validateValueKindName(valueKind.value); err != nil {
			return fmt.Errorf(
				"%s slot %q value_kind: %w",
				declarationKind,
				record.key,
				err,
			)
		}
		schema.expectExport(slotKind)
		prefix := keyedPath("slots", record.key)
		schema.expectDependency(prefix+".value_kind", valueKind)
		mode := record.facts["ref_mode.kind"]
		switch localpractice.ReferenceModeKind(mode.value) {
		case localpractice.ReferenceByValue:
			if _, exists := record.facts["ref_mode.ref_kind"]; exists {
				return fmt.Errorf(
					"%s by_value slot %q must not declare ref_kind",
					declarationKind,
					record.key,
				)
			}
		case localpractice.ReferenceByKind:
			refKind, exists := record.facts["ref_mode.ref_kind"]
			if !exists {
				return fmt.Errorf(
					"%s reference slot %q requires ref_kind",
					declarationKind,
					record.key,
				)
			}
			if err := validateRefKindName(refKind.value); err != nil {
				return err
			}
			schema.expectDependency(prefix+".ref_mode.ref_kind", refKind)
		default:
			return fmt.Errorf(
				"%s slot %q reference mode %q is unsupported",
				declarationKind,
				record.key,
				mode.value,
			)
		}
	}
	return nil
}

func validateRuntimeEvaluatorRequirementDeclarationSchema(
	schema *symbolicDeclarationSchema,
) error {
	ruleSource, err := schema.requireFact("rule_ref")
	if err != nil {
		return err
	}
	rule, err := typedmemory.NewRuleRef(ruleSource.value)
	if err != nil || rule.String() != ruleSource.value {
		return fmt.Errorf(
			"runtime_evaluator_requirement rule_ref %q is not a canonical RuleRef",
			ruleSource.value,
		)
	}
	contractSource, err := schema.requireFact("invocation_contract")
	if err != nil {
		return err
	}
	contract, err := parseEvaluatorRuntimeMechanismContract(contractSource.value)
	if err != nil || contract.String() != contractSource.value {
		return fmt.Errorf(
			"runtime_evaluator_requirement invocation_contract %q is not a closed evaluator contract",
			contractSource.value,
		)
	}
	return nil
}

func validateValueShapeDeclarationSchema(schema *symbolicDeclarationSchema) error {
	kind, err := schema.requireFact("shape.kind")
	if err != nil {
		return err
	}
	switch localpractice.ValueShapeKind(kind.value) {
	case localpractice.ValueShapeScalar:
		scalar, factErr := schema.requireFact("shape.scalar_kind")
		if factErr != nil {
			return factErr
		}
		if !validScalarKind(scalar.value) {
			return fmt.Errorf("value_shape scalar kind %q is unsupported", scalar.value)
		}
		return nil
	case localpractice.ValueShapeRecord:
		return validateShapeMembers(schema, "shape.fields")
	case localpractice.ValueShapeSum:
		return validateShapeMembers(schema, "shape.variants")
	case localpractice.ValueShapeOrderedSequence, localpractice.ValueShapeUnorderedSet:
		element, factErr := schema.requireFact("shape.element")
		if factErr != nil {
			return factErr
		}
		if !validQualifiedName(element.value) {
			return fmt.Errorf("value_shape element %q is not a qualified source name", element.value)
		}
		schema.expectDependency("shape.element", element)
		return nil
	case localpractice.ValueShapeClaimGraph:
		return nil
	default:
		return fmt.Errorf("value_shape kind %q is unsupported", kind.value)
	}
}

func validateShapeMembers(
	schema *symbolicDeclarationSchema,
	prefix string,
) error {
	records, err := collectKeyedFactRecords(
		schema,
		prefix,
		[]string{"name", "shape"},
		nil,
	)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return fmt.Errorf("value_shape %s requires at least one member", prefix)
	}
	for _, record := range records {
		name := record.facts["name"]
		if name.value != record.key || !validMemberName(name.value) {
			return fmt.Errorf("value_shape member key %q does not match valid name %q", record.key, name.value)
		}
		shape := record.facts["shape"]
		if !validQualifiedName(shape.value) {
			return fmt.Errorf("value_shape member %q shape %q is not a qualified source name", record.key, shape.value)
		}
		path := keyedPath(prefix, record.key) + ".shape"
		schema.expectDependency(path, shape)
	}
	return nil
}

func validateCodecBindingDeclarationSchema(schema *symbolicDeclarationSchema) error {
	valueKind, err := schema.requireFact("value_kind")
	if err != nil {
		return err
	}
	if err := validateValueKindName(valueKind.value); err != nil {
		return fmt.Errorf("codec_binding value_kind: %w", err)
	}
	valueShape, err := schema.requireFact("value_shape")
	if err != nil {
		return err
	}
	if !validQualifiedName(valueShape.value) {
		return fmt.Errorf("codec_binding value_shape %q is not a qualified source name", valueShape.value)
	}
	if _, err := schema.requireFact("canonicalization_version"); err != nil {
		return err
	}
	contract, err := collectIndexedScalarFacts(schema, "contract")
	if err != nil {
		return err
	}
	if len(contract) == 0 {
		return fmt.Errorf("codec_binding requires at least one contract statement")
	}
	if duplicate := duplicateScalarValue(contract); duplicate != "" {
		return fmt.Errorf("codec_binding repeats contract statement %q", duplicate)
	}
	schema.expectDependency("value_kind", valueKind)
	schema.expectDependency("value_shape", valueShape)
	return nil
}

func validateConstraintDeclarationSchema(schema *symbolicDeclarationSchema) error {
	kind, err := schema.requireFact("rule.kind")
	if err != nil {
		return err
	}
	switch localpractice.ConstraintKind(kind.value) {
	case localpractice.ConstraintKindDisjoint:
		return validateDisjointConstraintSchema(schema)
	case localpractice.ConstraintSlotGroup:
		return validateSlotGroupConstraintSchema(schema)
	case localpractice.ConstraintSlotCardinality:
		return validateSlotCardinalityConstraintSchema(schema)
	case localpractice.ConstraintReferenceSlotSubset:
		return validateReferenceSlotSubsetConstraintSchema(schema)
	case localpractice.ConstraintReferenceSlotPartition:
		return validateReferenceSlotPartitionConstraintSchema(schema)
	default:
		return fmt.Errorf("constraint rule kind %q is unsupported", kind.value)
	}
}

func validateDisjointConstraintSchema(schema *symbolicDeclarationSchema) error {
	kinds, err := collectIndexedScalarFacts(schema, "rule.disjoint_kinds")
	if err != nil {
		return err
	}
	if len(kinds) < 2 {
		return fmt.Errorf("kind_disjoint constraint requires at least two kinds")
	}
	if duplicate := duplicateScalarValue(kinds); duplicate != "" {
		return fmt.Errorf("kind_disjoint constraint repeats kind %q", duplicate)
	}
	for index, kind := range kinds {
		if err := validateValueKindName(kind.value); err != nil {
			return err
		}
		path := indexedPath("rule.disjoint_kinds", index)
		schema.expectDependency(path, kind)
	}
	return nil
}

func validateSlotGroupConstraintSchema(schema *symbolicDeclarationSchema) error {
	relation, err := schema.requireFact("rule.relation")
	if err != nil {
		return err
	}
	if !validQualifiedName(relation.value) {
		return fmt.Errorf("slot_group relation %q is not a qualified source name", relation.value)
	}
	mode, err := schema.requireFact("rule.mode")
	if err != nil {
		return err
	}
	if !validSlotGroupMode(mode.value) {
		return fmt.Errorf("slot_group constraint mode %q is unsupported", mode.value)
	}
	slots, err := collectIndexedScalarFacts(schema, "rule.slots")
	if err != nil {
		return err
	}
	if len(slots) < 2 {
		return fmt.Errorf("slot_group constraint requires at least two SlotKinds")
	}
	if duplicate := duplicateScalarValue(slots); duplicate != "" {
		return fmt.Errorf("slot_group constraint repeats SlotKind %q", duplicate)
	}
	schema.expectDependency("constraint.relation", relation)
	for index, slot := range slots {
		if err := validateSlotKindName(slot.value); err != nil {
			return err
		}
		path := indexedPath("rule.slots", index)
		schema.expectDependency(path, slot)
	}
	return nil
}

func validateSlotCardinalityConstraintSchema(schema *symbolicDeclarationSchema) error {
	relation, err := schema.requireFact("rule.relation")
	if err != nil {
		return err
	}
	if !validQualifiedName(relation.value) {
		return fmt.Errorf("slot_cardinality relation %q is not a qualified source name", relation.value)
	}
	slot, err := schema.requireFact("rule.slot")
	if err != nil {
		return err
	}
	if err := validateSlotKindName(slot.value); err != nil {
		return err
	}
	minimum, err := schema.requireFact("rule.cardinality.minimum")
	if err != nil {
		return err
	}
	minimumValue, err := parseCanonicalUnsigned(minimum.value)
	if err != nil {
		return fmt.Errorf("slot_cardinality minimum: %w", err)
	}
	maximum, err := schema.requireFact("rule.cardinality.maximum")
	if err != nil {
		return err
	}
	if maximum.value != "unbounded" {
		maximumValue, parseErr := parseCanonicalUnsigned(maximum.value)
		if parseErr != nil {
			return fmt.Errorf("slot_cardinality maximum: %w", parseErr)
		}
		if maximumValue < minimumValue {
			return fmt.Errorf("slot_cardinality maximum %d precedes minimum %d", maximumValue, minimumValue)
		}
	}
	schema.expectDependency("constraint.relation", relation)
	schema.expectDependency("constraint.slot", slot)
	return nil
}

func validateReferenceSlotSubsetConstraintSchema(
	schema *symbolicDeclarationSchema,
) error {
	relation, err := schema.requireFact("rule.relation")
	if err != nil {
		return err
	}
	if !validQualifiedName(relation.value) {
		return fmt.Errorf(
			"reference_slot_subset relation %q is not a qualified source name",
			relation.value,
		)
	}
	subset, err := schema.requireFact("rule.subset")
	if err != nil {
		return err
	}
	if err := validateSlotKindName(subset.value); err != nil {
		return err
	}
	superset, err := schema.requireFact("rule.superset")
	if err != nil {
		return err
	}
	if err := validateSlotKindName(superset.value); err != nil {
		return err
	}
	if subset.value == superset.value {
		return fmt.Errorf("reference_slot_subset coordinates must be distinct")
	}
	schema.expectDependency("constraint.relation", relation)
	schema.expectDependency("constraint.subset", subset)
	schema.expectDependency("constraint.superset", superset)
	return nil
}

func validateReferenceSlotPartitionConstraintSchema(
	schema *symbolicDeclarationSchema,
) error {
	relation, err := schema.requireFact("rule.relation")
	if err != nil {
		return err
	}
	if !validQualifiedName(relation.value) {
		return fmt.Errorf(
			"reference_slot_partition relation %q is not a qualified source name",
			relation.value,
		)
	}
	whole, err := schema.requireFact("rule.whole")
	if err != nil {
		return err
	}
	if err := validateSlotKindName(whole.value); err != nil {
		return err
	}
	parts, err := collectIndexedScalarFacts(schema, "rule.parts")
	if err != nil {
		return err
	}
	if len(parts) < 2 {
		return fmt.Errorf("reference_slot_partition requires at least two part SlotKinds")
	}
	if duplicate := duplicateScalarValue(parts); duplicate != "" {
		return fmt.Errorf("reference_slot_partition repeats part SlotKind %q", duplicate)
	}
	schema.expectDependency("constraint.relation", relation)
	schema.expectDependency("constraint.whole", whole)
	for index, part := range parts {
		if err := validateSlotKindName(part.value); err != nil {
			return err
		}
		if part.value == whole.value {
			return fmt.Errorf("reference_slot_partition whole cannot also be a part")
		}
		path := indexedPath("rule.parts", index)
		schema.expectDependency(path, part)
	}
	return nil
}

type keyedFactRecord struct {
	key   string
	facts map[string]SourceScalar
}

func collectKeyedFactRecords(
	schema *symbolicDeclarationSchema,
	prefix string,
	required []string,
	optional []string,
) ([]keyedFactRecord, error) {
	allowedLeaves := make(map[string]struct{}, len(required)+len(optional))
	for _, leaf := range required {
		allowedLeaves[leaf] = struct{}{}
	}
	for _, leaf := range optional {
		allowedLeaves[leaf] = struct{}{}
	}
	byKey := make(map[string]map[string]SourceScalar)
	for path, value := range schema.facts {
		key, leaf, matches := parseKeyedFactPath(path, prefix)
		if !matches {
			continue
		}
		if _, allowed := allowedLeaves[leaf]; !allowed {
			return nil, fmt.Errorf("symbolic declaration %q contains unknown %s field %q", schema.declaration.symbol.value, prefix, leaf)
		}
		fields, exists := byKey[key]
		if !exists {
			fields = make(map[string]SourceScalar)
			byKey[key] = fields
		}
		fields[leaf] = value
		schema.admitFact(path)
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]keyedFactRecord, 0, len(keys))
	for _, key := range keys {
		fields := byKey[key]
		for _, leaf := range required {
			if _, exists := fields[leaf]; !exists {
				return nil, fmt.Errorf("symbolic declaration %q %s %q requires field %q", schema.declaration.symbol.value, prefix, key, leaf)
			}
		}
		result = append(result, keyedFactRecord{key: key, facts: fields})
	}
	return result, nil
}

func collectIndexedFactRecords(
	schema *symbolicDeclarationSchema,
	prefix string,
	required []string,
) ([]map[string]SourceScalar, error) {
	allowedLeaves := make(map[string]struct{}, len(required))
	for _, leaf := range required {
		allowedLeaves[leaf] = struct{}{}
	}
	byIndex := make(map[int]map[string]SourceScalar)
	for path, value := range schema.facts {
		index, leaf, matches := parseIndexedFactPath(path, prefix)
		if !matches {
			continue
		}
		if _, allowed := allowedLeaves[leaf]; !allowed {
			return nil, fmt.Errorf("symbolic declaration %q contains unknown %s field %q", schema.declaration.symbol.value, prefix, leaf)
		}
		fields, exists := byIndex[index]
		if !exists {
			fields = make(map[string]SourceScalar)
			byIndex[index] = fields
		}
		fields[leaf] = value
		schema.admitFact(path)
	}
	result := make([]map[string]SourceScalar, 0, len(byIndex))
	for index := 0; index < len(byIndex); index++ {
		fields, exists := byIndex[index]
		if !exists {
			return nil, fmt.Errorf("symbolic declaration %q %s indices are not dense", schema.declaration.symbol.value, prefix)
		}
		for _, leaf := range required {
			if _, exists := fields[leaf]; !exists {
				return nil, fmt.Errorf("symbolic declaration %q %s[%d] requires field %q", schema.declaration.symbol.value, prefix, index, leaf)
			}
		}
		result = append(result, fields)
	}
	return result, nil
}

func collectIndexedScalarFacts(
	schema *symbolicDeclarationSchema,
	prefix string,
) ([]SourceScalar, error) {
	byIndex := make(map[int]SourceScalar)
	for path, value := range schema.facts {
		index, matches := parseIndexedScalarPath(path, prefix)
		if !matches {
			continue
		}
		byIndex[index] = value
		schema.admitFact(path)
	}
	result := make([]SourceScalar, 0, len(byIndex))
	for index := 0; index < len(byIndex); index++ {
		value, exists := byIndex[index]
		if !exists {
			return nil, fmt.Errorf("symbolic declaration %q %s indices are not dense", schema.declaration.symbol.value, prefix)
		}
		result = append(result, value)
	}
	return result, nil
}

func validateSubjectRowSemantics(row SignatureRowIR) error {
	valueKindPaths := []string{"subject_kind", "ranged_value_kind", "result_kind"}
	for _, path := range valueKindPaths {
		values := factsAtPath(row.facts, path)
		for _, value := range values {
			if err := validateValueKindName(value.value); err != nil {
				return fmt.Errorf("subject_block %s: %w", path, err)
			}
		}
	}
	sliceSet := factsAtPath(row.facts, "slice_set")[0]
	if !validQualifiedName(sliceSet.value) {
		return fmt.Errorf("subject_block slice_set %q is not a qualified source name", sliceSet.value)
	}
	return nil
}

func validateLawsRowSemantics(row SignatureRowIR) error {
	constraintRefs, err := collectRowIndexedFacts(row, "constraint_refs")
	if err != nil {
		return err
	}
	for _, reference := range constraintRefs {
		if !validQualifiedName(reference.value) {
			return fmt.Errorf("laws constraint_ref %q is not a qualified source name", reference.value)
		}
	}
	if duplicate := duplicateScalarValue(constraintRefs); duplicate != "" {
		return fmt.Errorf("laws repeats constraint_ref %q", duplicate)
	}
	invariants, err := collectRowIndexedFacts(row, "invariants")
	if err != nil {
		return err
	}
	if len(invariants) == 0 {
		return fmt.Errorf("laws requires at least one invariant")
	}
	if duplicate := duplicateScalarValue(invariants); duplicate != "" {
		return fmt.Errorf("laws repeats invariant %q", duplicate)
	}
	if len(constraintRefs)+len(invariants) != len(row.facts) {
		return fmt.Errorf("laws contains a source fact outside its closed schema")
	}
	return nil
}

func validateApplicabilityRowSemantics(row SignatureRowIR) error {
	assumptions, err := collectRowIndexedFacts(row, "assumptions")
	if err != nil {
		return err
	}
	if len(assumptions) == 0 {
		return fmt.Errorf("applicability requires at least one assumption")
	}
	if duplicate := duplicateScalarValue(assumptions); duplicate != "" {
		return fmt.Errorf("applicability repeats assumption %q", duplicate)
	}
	if len(assumptions)+1 != len(row.facts) {
		return fmt.Errorf("applicability contains a source fact outside its closed schema")
	}
	return nil
}

func collectRowIndexedFacts(
	row SignatureRowIR,
	prefix string,
) ([]SourceScalar, error) {
	byIndex := make(map[int]SourceScalar)
	for _, fact := range row.facts {
		index, matches := parseIndexedScalarPath(fact.path, prefix)
		if !matches {
			continue
		}
		byIndex[index] = fact.value
	}
	result := make([]SourceScalar, 0, len(byIndex))
	for index := 0; index < len(byIndex); index++ {
		value, exists := byIndex[index]
		if !exists {
			return nil, fmt.Errorf("%s row %s indices are not dense", row.name, prefix)
		}
		result = append(result, value)
	}
	return result, nil
}

func parseKeyedFactPath(path string, prefix string) (string, string, bool) {
	opening := prefix + "["
	if !strings.HasPrefix(path, opening) {
		return "", "", false
	}
	remainder := strings.TrimPrefix(path, opening)
	colon := strings.IndexByte(remainder, ':')
	if colon <= 0 {
		return "", "", false
	}
	lengthText := remainder[:colon]
	length, err := strconv.Atoi(lengthText)
	if err != nil || length < 0 || strconv.Itoa(length) != lengthText {
		return "", "", false
	}
	keyStart := colon + 1
	if length > len(remainder)-keyStart {
		return "", "", false
	}
	keyEnd := keyStart + length
	if keyEnd >= len(remainder) || remainder[keyEnd] != ']' {
		return "", "", false
	}
	leafStart := keyEnd + 1
	if leafStart >= len(remainder) || remainder[leafStart] != '.' {
		return "", "", false
	}
	key := remainder[keyStart:keyEnd]
	leaf := remainder[leafStart+1:]
	if key == "" || leaf == "" || keyedPath(prefix, key)+"."+leaf != path {
		return "", "", false
	}
	return key, leaf, true
}

func parseIndexedFactPath(path string, prefix string) (int, string, bool) {
	opening := prefix + "["
	if !strings.HasPrefix(path, opening) {
		return 0, "", false
	}
	remainder := strings.TrimPrefix(path, opening)
	closing := strings.IndexByte(remainder, ']')
	if closing <= 0 || closing+1 >= len(remainder) || remainder[closing+1] != '.' {
		return 0, "", false
	}
	indexText := remainder[:closing]
	index, err := strconv.Atoi(indexText)
	if err != nil || index < 0 {
		return 0, "", false
	}
	leaf := remainder[closing+2:]
	if leaf == "" || indexedPath(prefix, index)+"."+leaf != path {
		return 0, "", false
	}
	return index, leaf, true
}

func parseIndexedScalarPath(path string, prefix string) (int, bool) {
	opening := prefix + "["
	if !strings.HasPrefix(path, opening) || !strings.HasSuffix(path, "]") {
		return 0, false
	}
	indexText := path[len(opening) : len(path)-1]
	index, err := strconv.Atoi(indexText)
	if err != nil || index < 0 || indexedPath(prefix, index) != path {
		return 0, false
	}
	return index, true
}

func validateDeclarationSymbol(kind localpractice.DeclarationKind, value string) error {
	if kind == localpractice.DeclarationBoundedContext {
		contextRef, err := typedmemory.NewBoundedContextRef(value)
		if err != nil || contextRef.String() != value {
			return fmt.Errorf("declaration symbol is not a canonical BoundedContextRef")
		}
		return nil
	}
	if !validQualifiedName(value) {
		return fmt.Errorf("declaration symbol is not a qualified source name")
	}
	if kind == localpractice.DeclarationValueKind {
		return validateValueKindName(value)
	}
	if kind == localpractice.DeclarationRefKind {
		return validateRefKindName(value)
	}
	return nil
}

func validateValueKindName(value string) error {
	if !validQualifiedName(value) {
		return fmt.Errorf("ValueKind %q is not a qualified source name", value)
	}
	if strings.HasSuffix(value, "Slot") || strings.HasSuffix(value, "Ref") {
		return fmt.Errorf("ValueKind %q violates A.6.5 suffix discipline", value)
	}
	return nil
}

func validateRefKindName(value string) error {
	if !validQualifiedName(value) || !strings.HasSuffix(value, "Ref") {
		return fmt.Errorf("RefKind %q must be a qualified source name ending with Ref", value)
	}
	return nil
}

func validateSlotKindName(value string) error {
	if !validQualifiedName(value) || !strings.HasSuffix(value, "Slot") {
		return fmt.Errorf("SlotKind %q must be a qualified source name ending with Slot", value)
	}
	return nil
}

func validQualifiedName(value string) bool {
	return value != "" && !strings.ContainsAny(value, " /\\")
}

func validMemberName(value string) bool {
	return value != "" && !strings.ContainsAny(value, " /\\")
}

func validSignatureFormality(value string) bool {
	return len(value) == 2 && value[0] == 'F' && value[1] >= '0' && value[1] <= '9'
}

func validScalarKind(value string) bool {
	switch value {
	case "text", "boolean", "signed_integer", "unsigned_integer", "bytes":
		return true
	default:
		return false
	}
}

func validSlotGroupMode(value string) bool {
	switch localpractice.SlotGroupMode(value) {
	case localpractice.SlotGroupAllOrNone,
		localpractice.SlotGroupAtLeastOne,
		localpractice.SlotGroupExactlyOne:
		return true
	default:
		return false
	}
}

func parseCanonicalUnsigned(value string) (uint64, error) {
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return 0, fmt.Errorf("%q is not canonical unsigned decimal", value)
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, fmt.Errorf("%q is not canonical unsigned decimal", value)
		}
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not uint64: %w", value, err)
	}
	return parsed, nil
}

func duplicateScalarValue(values []SourceScalar) string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, exists := seen[value.value]; exists {
			return value.value
		}
		seen[value.value] = struct{}{}
	}
	return ""
}

func sortedSourceScalarKeys(values map[string]SourceScalar) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
