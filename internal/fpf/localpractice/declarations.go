package localpractice

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type runtimeEvaluatorRequirementCoordinate struct {
	invocationContract string
	ruleRef            string
}

func parseVocabulary(node *yaml.Node) (Vocabulary, error) {
	fields, err := mappingFields(
		node,
		"signature.vocabulary",
		[]string{"declarations"},
		nil,
	)
	if err != nil {
		return Vocabulary{}, err
	}
	items, err := sequenceItems(
		fields["declarations"],
		"signature.vocabulary.declarations",
		false,
	)
	if err != nil {
		return Vocabulary{}, err
	}
	declarations := make([]Declaration, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	seenRuntimeRequirements := make(map[runtimeEvaluatorRequirementCoordinate]struct{})
	for index, item := range items {
		context := fmt.Sprintf("signature.vocabulary.declarations[%d]", index)
		declaration, parseErr := parseDeclaration(item, context)
		if parseErr != nil {
			return Vocabulary{}, parseErr
		}
		symbol := declaration.Symbol().value
		if _, exists := seen[symbol]; exists {
			return Vocabulary{}, fmt.Errorf(
				"signature.vocabulary.declarations contains duplicate symbol %q",
				symbol,
			)
		}
		seen[symbol] = struct{}{}
		requirement, isRuntimeRequirement := declaration.(RuntimeEvaluatorRequirementDeclaration)
		if isRuntimeRequirement {
			coordinate := runtimeEvaluatorRequirementCoordinate{
				invocationContract: requirement.invocationContract.value,
				ruleRef:            requirement.ruleRef.value,
			}
			if _, exists := seenRuntimeRequirements[coordinate]; exists {
				return Vocabulary{}, fmt.Errorf(
					"signature.vocabulary.declarations contains duplicate runtime evaluator requirement contract %q for RuleRef %q",
					requirement.invocationContract.value,
					requirement.ruleRef.value,
				)
			}
			seenRuntimeRequirements[coordinate] = struct{}{}
		}
		declarations = append(declarations, declaration)
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return Vocabulary{}, fmt.Errorf("signature.vocabulary source span: %w", err)
	}
	return Vocabulary{declarations: declarations, span: span}, nil
}

func parseDeclaration(node *yaml.Node, context string) (Declaration, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{"kind", "symbol"},
		[]string{
			"child_kind",
			"super_kind",
			"value_kind",
			"enumeration_rule",
			"candidate_policy",
			"formality",
			"assumptions",
			"definedness_rule",
			"evaluator_rule",
			"membership_basis",
			"entity_set",
			"local_kind",
			"candidate_value_kind",
			"criterion_rule",
			"slice_conditions_rule",
			"reference_scheme",
			"dependencies",
			"extent_rule",
			"slots",
			"evaluator_requirement",
			"shape",
			"value_shape",
			"canonicalization_version",
			"contract",
			"rule_ref",
			"invocation_contract",
			"rule",
			"endpoints",
			"mapping",
			"direction",
			"order_preservation",
			"kind_congruence",
			"loss_notes",
			"definedness_area",
		},
	)
	if err != nil {
		return nil, err
	}
	kind, err := sourceText(fields["kind"], context+".kind")
	if err != nil {
		return nil, err
	}
	switch DeclarationKind(kind.value) {
	case DeclarationBoundedContext:
		return parseBoundedContextDeclaration(node, context)
	case DeclarationValueKind:
		return parseValueKindDeclaration(node, context)
	case DeclarationSubkind:
		return parseSubkindDeclaration(node, context)
	case DeclarationRefKind:
		return parseRefKindDeclaration(node, context)
	case DeclarationEntitySet:
		return parseEntitySetDefinitionDeclaration(node, context)
	case DeclarationKindSignature:
		return parseKindSignatureDefinitionDeclaration(node, context)
	case DeclarationKindClassificationSignature:
		return parseKindClassificationSignatureDeclaration(node, context)
	case DeclarationRelationSignature:
		return parseTypedRelationDeclarationFragment(node, context)
	case DeclarationRuntimeEvaluatorInput:
		return parseRuntimeEvaluatorInputDeclaration(node, context)
	case DeclarationValueShape:
		return parseValueShapeDeclaration(node, context)
	case DeclarationCodecBinding:
		return parseCodecBindingDeclaration(node, context)
	case DeclarationRuntimeEvaluatorRequirement:
		return parseRuntimeEvaluatorRequirementDeclaration(node, context)
	case DeclarationConstraint:
		return parseConstraintDeclaration(node, context)
	case DeclarationKindBridge:
		return parseKindBridgeDeclaration(node, context)
	default:
		return nil, fmt.Errorf("%s.kind %q is not supported", context, kind.value)
	}
}

func parseBoundedContextDeclaration(node *yaml.Node, context string) (Declaration, error) {
	fields, err := mappingFields(node, context, []string{"kind", "symbol"}, nil)
	if err != nil {
		return nil, err
	}
	symbol, err := sourceText(fields["symbol"], context+".symbol")
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return BoundedContextDeclaration{symbol: symbol, span: span}, nil
}

func parseSubkindDeclaration(node *yaml.Node, context string) (Declaration, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{"kind", "symbol", "child_kind", "super_kind"},
		nil,
	)
	if err != nil {
		return nil, err
	}
	symbol, err := qualifiedSourceText(fields["symbol"], context+".symbol")
	if err != nil {
		return nil, err
	}
	childKind, err := qualifiedSourceText(fields["child_kind"], context+".child_kind")
	if err != nil {
		return nil, err
	}
	superKind, err := qualifiedSourceText(fields["super_kind"], context+".super_kind")
	if err != nil {
		return nil, err
	}
	if childKind.value == superKind.value {
		return nil, fmt.Errorf("%s child_kind and super_kind must be distinct", context)
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return SubkindDeclaration{
		symbol:    symbol,
		childKind: childKind,
		superKind: superKind,
		span:      span,
	}, nil
}

func parseKindBridgeDeclaration(node *yaml.Node, context string) (Declaration, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{
			"kind",
			"symbol",
			"endpoints",
			"mapping",
			"direction",
			"order_preservation",
			"kind_congruence",
			"loss_notes",
			"definedness_area",
		},
		nil,
	)
	if err != nil {
		return nil, err
	}
	symbol, err := qualifiedSourceText(fields["symbol"], context+".symbol")
	if err != nil {
		return nil, err
	}
	source, target, err := parseKindBridgeEndpoints(
		fields["endpoints"],
		context+".endpoints",
	)
	if err != nil {
		return nil, err
	}
	if source.boundedContextRef.value == target.boundedContextRef.value {
		return nil, fmt.Errorf("%s endpoints must name distinct bounded contexts", context)
	}
	mapping, err := parseKindBridgeMapping(fields["mapping"], context+".mapping")
	if err != nil {
		return nil, err
	}
	direction, err := parseKindBridgeDirection(
		fields["direction"],
		context+".direction",
	)
	if err != nil {
		return nil, err
	}
	orderPreservation, err := parseKindBridgeOrderPreservation(
		fields["order_preservation"],
		context+".order_preservation",
	)
	if err != nil {
		return nil, err
	}
	kindCongruence, err := parseKindCongruenceLevel(
		fields["kind_congruence"],
		context+".kind_congruence",
	)
	if err != nil {
		return nil, err
	}
	lossNotes, err := parseUniqueSourceTextSequence(
		fields["loss_notes"],
		context+".loss_notes",
		false,
		sourceText,
	)
	if err != nil {
		return nil, err
	}
	definednessArea, err := parseUniqueSourceTextSequence(
		fields["definedness_area"],
		context+".definedness_area",
		false,
		sourceText,
	)
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return KindBridgeDeclaration{
		symbol:            symbol,
		source:            source,
		target:            target,
		mapping:           mapping,
		direction:         direction,
		orderPreservation: orderPreservation,
		kindCongruence:    kindCongruence,
		lossNotes:         lossNotes,
		definednessArea:   definednessArea,
		span:              span,
	}, nil
}

func parseKindBridgeEndpoints(
	node *yaml.Node,
	context string,
) (KindBridgeEndpoint, KindBridgeEndpoint, error) {
	fields, err := mappingFields(node, context, []string{"source", "target"}, nil)
	if err != nil {
		return KindBridgeEndpoint{}, KindBridgeEndpoint{}, err
	}
	source, err := parseKindBridgeEndpoint(fields["source"], context+".source")
	if err != nil {
		return KindBridgeEndpoint{}, KindBridgeEndpoint{}, err
	}
	target, err := parseKindBridgeEndpoint(fields["target"], context+".target")
	if err != nil {
		return KindBridgeEndpoint{}, KindBridgeEndpoint{}, err
	}
	return source, target, nil
}

func parseKindBridgeEndpoint(
	node *yaml.Node,
	context string,
) (KindBridgeEndpoint, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{"bounded_context_ref", "edition"},
		nil,
	)
	if err != nil {
		return KindBridgeEndpoint{}, err
	}
	boundedContextRef, err := sourceText(
		fields["bounded_context_ref"],
		context+".bounded_context_ref",
	)
	if err != nil {
		return KindBridgeEndpoint{}, err
	}
	edition, err := sourceText(fields["edition"], context+".edition")
	if err != nil {
		return KindBridgeEndpoint{}, err
	}
	if err := validatePinnedKindBridgeEdition(edition.value); err != nil {
		return KindBridgeEndpoint{}, fmt.Errorf("%s.edition: %w", context, err)
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return KindBridgeEndpoint{}, fmt.Errorf("%s source span: %w", context, err)
	}
	return KindBridgeEndpoint{
		boundedContextRef: boundedContextRef,
		edition:           edition,
		span:              span,
	}, nil
}

func validatePinnedKindBridgeEdition(value string) error {
	switch strings.ToLower(value) {
	case "latest", "current", "head", "*":
		return fmt.Errorf("must pin an exact edition rather than %q", value)
	default:
		return nil
	}
}

func parseKindBridgeMapping(
	node *yaml.Node,
	context string,
) (NamedTargetKindMapping, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{"kind", "source_kind", "target_kind"},
		nil,
	)
	if err != nil {
		return NamedTargetKindMapping{}, err
	}
	kind, err := sourceText(fields["kind"], context+".kind")
	if err != nil {
		return NamedTargetKindMapping{}, err
	}
	if KindBridgeMappingKind(kind.value) != KindBridgeNamedTarget {
		return NamedTargetKindMapping{}, fmt.Errorf(
			"%s.kind %q is not supported; v1 requires an exact named_target mapping",
			context,
			kind.value,
		)
	}
	sourceKind, err := qualifiedSourceText(fields["source_kind"], context+".source_kind")
	if err != nil {
		return NamedTargetKindMapping{}, err
	}
	targetKind, err := qualifiedSourceText(fields["target_kind"], context+".target_kind")
	if err != nil {
		return NamedTargetKindMapping{}, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return NamedTargetKindMapping{}, fmt.Errorf("%s source span: %w", context, err)
	}
	return NamedTargetKindMapping{
		kindSource: kind,
		sourceKind: sourceKind,
		targetKind: targetKind,
		span:       span,
	}, nil
}

func parseKindBridgeDirection(
	node *yaml.Node,
	context string,
) (KindBridgeDirection, error) {
	value, err := sourceText(node, context)
	if err != nil {
		return KindBridgeDirection{}, err
	}
	direction := KindBridgeDirectionKind(value.value)
	if direction != KindBridgeOneWay && direction != KindBridgeTwoWay {
		return KindBridgeDirection{}, fmt.Errorf(
			"%s %q is not supported; want one_way or two_way",
			context,
			value.value,
		)
	}
	return KindBridgeDirection{kind: direction, span: value.span}, nil
}

func parseKindBridgeOrderPreservation(
	node *yaml.Node,
	context string,
) (KindBridgeOrderPreservation, error) {
	value, err := sourceText(node, context)
	if err != nil {
		return KindBridgeOrderPreservation{}, err
	}
	preservation := KindBridgeOrderPreservationKind(value.value)
	if preservation != KindBridgeNoOrderLinksCovered {
		return KindBridgeOrderPreservation{}, fmt.Errorf(
			"%s %q is unsupported; v1 can only declare no_links_covered",
			context,
			value.value,
		)
	}
	return KindBridgeOrderPreservation{kind: preservation, span: value.span}, nil
}

func parseKindCongruenceLevel(
	node *yaml.Node,
	context string,
) (KindCongruenceLevel, error) {
	value, err := unsignedScalar(node, context)
	if err != nil {
		return KindCongruenceLevel{}, err
	}
	if value > 3 {
		return KindCongruenceLevel{}, fmt.Errorf("%s must be in the closed CL^k range 0..3", context)
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return KindCongruenceLevel{}, fmt.Errorf("%s source span: %w", context, err)
	}
	return KindCongruenceLevel{value: uint8(value), span: span}, nil
}

func parseValueKindDeclaration(node *yaml.Node, context string) (Declaration, error) {
	fields, err := mappingFields(node, context, []string{"kind", "symbol"}, nil)
	if err != nil {
		return nil, err
	}
	symbol, err := valueKindText(fields["symbol"], context+".symbol")
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return ValueKindDeclaration{symbol: symbol, span: span}, nil
}

func parseRefKindDeclaration(node *yaml.Node, context string) (Declaration, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{"kind", "symbol", "value_kind"},
		nil,
	)
	if err != nil {
		return nil, err
	}
	symbol, err := refKindText(fields["symbol"], context+".symbol")
	if err != nil {
		return nil, err
	}
	valueKind, err := valueKindText(fields["value_kind"], context+".value_kind")
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return RefKindDeclaration{symbol: symbol, valueKind: valueKind, span: span}, nil
}

func parseEntitySetDefinitionDeclaration(node *yaml.Node, context string) (Declaration, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{"kind", "symbol", "enumeration_rule", "candidate_policy"},
		nil,
	)
	if err != nil {
		return nil, err
	}
	symbol, err := qualifiedSourceText(fields["symbol"], context+".symbol")
	if err != nil {
		return nil, err
	}
	enumerationRule, err := sourceText(
		fields["enumeration_rule"],
		context+".enumeration_rule",
	)
	if err != nil {
		return nil, err
	}
	policy, err := parseEntitySetCandidatePolicy(
		fields["candidate_policy"],
		context+".candidate_policy",
	)
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return EntitySetDefinitionDeclaration{
		symbol:          symbol,
		enumerationRule: enumerationRule,
		candidatePolicy: policy,
		span:            span,
	}, nil
}

func parseEntitySetCandidatePolicy(
	node *yaml.Node,
	context string,
) (EntitySetCandidatePolicy, error) {
	initial, err := mappingFields(node, context, []string{"kind"}, []string{"evaluation_rule"})
	if err != nil {
		return nil, err
	}
	kind, err := sourceText(initial["kind"], context+".kind")
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	policyKind := EntitySetPolicyKind(kind.value)
	if policyKind == EntitySetPersistedOnly {
		_, exactErr := mappingFields(node, context, []string{"kind"}, nil)
		if exactErr != nil {
			return nil, exactErr
		}
		return PersistedEntitiesOnlyPolicy{span: span}, nil
	}
	if policyKind == EntitySetPriorBatch {
		fields, exactErr := mappingFields(
			node,
			context,
			[]string{"kind", "evaluation_rule"},
			nil,
		)
		if exactErr != nil {
			return nil, exactErr
		}
		evaluationRule, parseErr := sourceText(
			fields["evaluation_rule"],
			context+".evaluation_rule",
		)
		if parseErr != nil {
			return nil, parseErr
		}
		return PriorBatchDeclarationsVisiblePolicy{
			evaluationRule: evaluationRule,
			span:           span,
		}, nil
	}
	return nil, fmt.Errorf("%s.kind %q is not supported", context, kind.value)
}

func parseKindSignatureDefinitionDeclaration(
	node *yaml.Node,
	context string,
) (Declaration, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{
			"kind",
			"symbol",
			"value_kind",
			"formality",
			"assumptions",
			"definedness_rule",
			"evaluator_rule",
			"membership_basis",
			"entity_set",
		},
		nil,
	)
	if err != nil {
		return nil, err
	}
	symbol, err := qualifiedSourceText(fields["symbol"], context+".symbol")
	if err != nil {
		return nil, err
	}
	valueKind, err := valueKindText(fields["value_kind"], context+".value_kind")
	if err != nil {
		return nil, err
	}
	formality, err := parseSignatureFormality(fields["formality"], context+".formality")
	if err != nil {
		return nil, err
	}
	assumptions, err := parseKindSignatureAssumptions(
		fields["assumptions"],
		context+".assumptions",
	)
	if err != nil {
		return nil, err
	}
	definednessRule, err := sourceText(
		fields["definedness_rule"],
		context+".definedness_rule",
	)
	if err != nil {
		return nil, err
	}
	evaluatorRule, err := sourceText(
		fields["evaluator_rule"],
		context+".evaluator_rule",
	)
	if err != nil {
		return nil, err
	}
	membershipBasis, err := parseKindSignatureMembershipBasis(
		fields["membership_basis"],
		context+".membership_basis",
	)
	if err != nil {
		return nil, err
	}
	entitySet, err := qualifiedSourceText(fields["entity_set"], context+".entity_set")
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return KindSignatureDefinitionDeclaration{
		symbol:          symbol,
		valueKind:       valueKind,
		formality:       formality,
		assumptions:     assumptions,
		definednessRule: definednessRule,
		evaluatorRule:   evaluatorRule,
		membershipBasis: membershipBasis,
		entitySet:       entitySet,
		span:            span,
	}, nil
}

func parseKindSignatureMembershipBasis(
	node *yaml.Node,
	context string,
) (KindSignatureMembershipBasis, error) {
	initial, err := mappingFields(
		node,
		context,
		[]string{"kind"},
		[]string{"adapter_rule"},
	)
	if err != nil {
		return nil, err
	}
	kind, err := sourceText(initial["kind"], context+".kind")
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	basisKind := KindSignatureMembershipBasisKind(kind.value)
	if basisKind == KindSignatureDirectObservableInputs {
		_, exactErr := mappingFields(node, context, []string{"kind"}, nil)
		if exactErr != nil {
			return nil, exactErr
		}
		return DirectObservableInputsMembershipBasis{
			kindSource: kind,
			span:       span,
		}, nil
	}
	if basisKind == KindSignatureCarrierFirst {
		fields, exactErr := mappingFields(
			node,
			context,
			[]string{"kind", "adapter_rule"},
			nil,
		)
		if exactErr != nil {
			return nil, exactErr
		}
		adapterRule, parseErr := sourceText(
			fields["adapter_rule"],
			context+".adapter_rule",
		)
		if parseErr != nil {
			return nil, parseErr
		}
		return CarrierFirstMembershipBasis{
			kindSource:  kind,
			adapterRule: adapterRule,
			span:        span,
		}, nil
	}
	return nil, fmt.Errorf(
		"%s.kind %q is not supported; want direct_observable_inputs or carrier_first",
		context,
		kind.value,
	)
}

func parseSignatureFormality(node *yaml.Node, context string) (SignatureFormality, error) {
	text, err := sourceText(node, context)
	if err != nil {
		return SignatureF0, err
	}
	if len(text.value) != 2 || text.value[0] != 'F' || text.value[1] < '0' || text.value[1] > '9' {
		return SignatureF0, fmt.Errorf("%s must be one of F0..F9", context)
	}
	return SignatureFormality(text.value[1] - '0'), nil
}

func parseKindSignatureAssumptions(
	node *yaml.Node,
	context string,
) ([]KindSignatureAssumption, error) {
	items, err := sequenceItems(node, context, true)
	if err != nil {
		return nil, err
	}
	assumptions := make([]KindSignatureAssumption, 0, len(items))
	seen := make(map[kindSignatureAssumptionCoordinate]struct{}, len(items))
	for index, item := range items {
		itemContext := fmt.Sprintf("%s[%d]", context, index)
		fields, fieldsErr := mappingFields(
			item,
			itemContext,
			[]string{"carrier_ref", "edition", "digest"},
			nil,
		)
		if fieldsErr != nil {
			return nil, fieldsErr
		}
		carrierRef, parseErr := sourceText(fields["carrier_ref"], itemContext+".carrier_ref")
		if parseErr != nil {
			return nil, parseErr
		}
		edition, parseErr := sourceText(fields["edition"], itemContext+".edition")
		if parseErr != nil {
			return nil, parseErr
		}
		digest, parseErr := sourceText(fields["digest"], itemContext+".digest")
		if parseErr != nil {
			return nil, parseErr
		}
		if digestErr := validateSHA256Digest(digest.value); digestErr != nil {
			return nil, fmt.Errorf("%s.digest: %w", itemContext, digestErr)
		}
		key := kindSignatureAssumptionCoordinate{
			carrierRef: carrierRef.value,
			edition:    edition.value,
		}
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf(
				"%s contains duplicate carrier edition %q at %q",
				context,
				carrierRef.value,
				edition.value,
			)
		}
		seen[key] = struct{}{}
		span, spanErr := nodeLineRange(item)
		if spanErr != nil {
			return nil, fmt.Errorf("%s source span: %w", itemContext, spanErr)
		}
		assumptions = append(assumptions, KindSignatureAssumption{
			carrierRef: carrierRef,
			edition:    edition,
			digest:     digest,
			span:       span,
		})
	}
	return assumptions, nil
}

type kindSignatureAssumptionCoordinate struct {
	carrierRef string
	edition    string
}

func parseTypedRelationDeclarationFragment(
	node *yaml.Node,
	context string,
) (Declaration, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{"kind", "symbol", "slots"},
		nil,
	)
	if err != nil {
		return nil, err
	}
	symbol, err := qualifiedSourceText(fields["symbol"], context+".symbol")
	if err != nil {
		return nil, err
	}
	slots, err := parseSlots(fields["slots"], context+".slots")
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return TypedRelationDeclarationFragmentDeclaration{
		symbol: symbol,
		slots:  slots,
		span:   span,
	}, nil
}

func parseRuntimeEvaluatorInputDeclaration(
	node *yaml.Node,
	context string,
) (Declaration, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{"kind", "symbol", "evaluator_requirement", "slots"},
		nil,
	)
	if err != nil {
		return nil, err
	}
	symbol, err := qualifiedSourceText(fields["symbol"], context+".symbol")
	if err != nil {
		return nil, err
	}
	evaluatorRequirement, err := qualifiedSourceText(
		fields["evaluator_requirement"],
		context+".evaluator_requirement",
	)
	if err != nil {
		return nil, err
	}
	slots, err := parseSlots(fields["slots"], context+".slots")
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return RuntimeEvaluatorInputDeclaration{
		symbol:               symbol,
		evaluatorRequirement: evaluatorRequirement,
		slots:                slots,
		span:                 span,
	}, nil
}

func parseSlots(node *yaml.Node, context string) ([]SlotSpec, error) {
	items, err := sequenceItems(node, context, false)
	if err != nil {
		return nil, err
	}
	slots := make([]SlotSpec, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for index, item := range items {
		itemContext := fmt.Sprintf("%s[%d]", context, index)
		slot, parseErr := parseSlot(item, itemContext)
		if parseErr != nil {
			return nil, parseErr
		}
		key := slot.slotKind.value
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("%s contains duplicate SlotKind %q", context, key)
		}
		seen[key] = struct{}{}
		slots = append(slots, slot)
	}
	return slots, nil
}

func parseSlot(node *yaml.Node, context string) (SlotSpec, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{"slot_kind", "value_kind", "ref_mode"},
		nil,
	)
	if err != nil {
		return SlotSpec{}, err
	}
	slotKind, err := slotKindText(fields["slot_kind"], context+".slot_kind")
	if err != nil {
		return SlotSpec{}, err
	}
	valueKind, err := valueKindText(fields["value_kind"], context+".value_kind")
	if err != nil {
		return SlotSpec{}, err
	}
	reference, err := parseReferenceMode(fields["ref_mode"], context+".ref_mode")
	if err != nil {
		return SlotSpec{}, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return SlotSpec{}, fmt.Errorf("%s source span: %w", context, err)
	}
	return SlotSpec{
		slotKind:  slotKind,
		valueKind: valueKind,
		reference: reference,
		span:      span,
	}, nil
}

func parseReferenceMode(node *yaml.Node, context string) (ReferenceMode, error) {
	initial, err := mappingFields(node, context, []string{"kind"}, []string{"ref_kind"})
	if err != nil {
		return nil, err
	}
	kind, err := sourceText(initial["kind"], context+".kind")
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	mode := ReferenceModeKind(kind.value)
	if mode == ReferenceByValue {
		_, exactErr := mappingFields(node, context, []string{"kind"}, nil)
		if exactErr != nil {
			return nil, exactErr
		}
		return ByValueReferenceMode{span: span}, nil
	}
	if mode == ReferenceByKind {
		fields, exactErr := mappingFields(
			node,
			context,
			[]string{"kind", "ref_kind"},
			nil,
		)
		if exactErr != nil {
			return nil, exactErr
		}
		refKind, parseErr := refKindText(fields["ref_kind"], context+".ref_kind")
		if parseErr != nil {
			return nil, parseErr
		}
		return RefKindReferenceMode{refKind: refKind, span: span}, nil
	}
	return nil, fmt.Errorf("%s.kind %q is not supported", context, kind.value)
}

func parseCardinality(node *yaml.Node, context string) (Cardinality, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{"minimum", "maximum"},
		nil,
	)
	if err != nil {
		return Cardinality{}, err
	}
	minimum, err := unsignedScalar(fields["minimum"], context+".minimum")
	if err != nil {
		return Cardinality{}, err
	}
	maximum, err := parseMaximum(fields["maximum"], context+".maximum")
	if err != nil {
		return Cardinality{}, err
	}
	maximumValue, bounded := maximum.Value()
	if bounded && maximumValue < minimum {
		return Cardinality{}, fmt.Errorf(
			"%s maximum %d precedes minimum %d",
			context,
			maximumValue,
			minimum,
		)
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return Cardinality{}, fmt.Errorf("%s source span: %w", context, err)
	}
	return Cardinality{minimum: minimum, maximum: maximum, span: span}, nil
}

func parseMaximum(node *yaml.Node, context string) (OptionalMaximum, error) {
	if node.Kind == yaml.ScalarNode && node.Tag == "!!str" {
		text, err := sourceText(node, context)
		if err != nil {
			return OptionalMaximum{}, err
		}
		if text.value != "unbounded" {
			return OptionalMaximum{}, fmt.Errorf("%s string value must be %q", context, "unbounded")
		}
		return OptionalMaximum{unbounded: true}, nil
	}
	value, err := unsignedScalar(node, context)
	if err != nil {
		return OptionalMaximum{}, err
	}
	return OptionalMaximum{value: value}, nil
}

func parseValueShapeDeclaration(node *yaml.Node, context string) (Declaration, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{"kind", "symbol", "shape"},
		nil,
	)
	if err != nil {
		return nil, err
	}
	symbol, err := qualifiedSourceText(fields["symbol"], context+".symbol")
	if err != nil {
		return nil, err
	}
	shape, err := parseValueShape(fields["shape"], context+".shape")
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return ValueShapeDeclaration{symbol: symbol, shape: shape, span: span}, nil
}

func parseValueShape(node *yaml.Node, context string) (ValueShape, error) {
	initial, err := mappingFields(
		node,
		context,
		[]string{"kind"},
		[]string{"scalar_kind", "fields", "variants", "element_shape"},
	)
	if err != nil {
		return nil, err
	}
	kind, err := sourceText(initial["kind"], context+".kind")
	if err != nil {
		return nil, err
	}
	switch ValueShapeKind(kind.value) {
	case ValueShapeScalar:
		return parseScalarShape(node, context)
	case ValueShapeRecord:
		return parseRecordShape(node, context)
	case ValueShapeSum:
		return parseSumShape(node, context)
	case ValueShapeOrderedSequence:
		return parseCollectionShape(node, context, ValueShapeOrderedSequence)
	case ValueShapeUnorderedSet:
		return parseCollectionShape(node, context, ValueShapeUnorderedSet)
	case ValueShapeClaimGraph:
		return parseClaimGraphShape(node, context)
	default:
		return nil, fmt.Errorf("%s.kind %q is not supported", context, kind.value)
	}
}

func parseScalarShape(node *yaml.Node, context string) (ValueShape, error) {
	fields, err := mappingFields(node, context, []string{"kind", "scalar_kind"}, nil)
	if err != nil {
		return nil, err
	}
	scalarKind, err := sourceText(fields["scalar_kind"], context+".scalar_kind")
	if err != nil {
		return nil, err
	}
	if !supportedScalarKind(scalarKind.value) {
		return nil, fmt.Errorf("%s.scalar_kind %q is not supported", context, scalarKind.value)
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return ScalarValueShape{scalarKind: scalarKind, span: span}, nil
}

func supportedScalarKind(value string) bool {
	switch value {
	case "text", "boolean", "signed_integer", "unsigned_integer", "bytes":
		return true
	default:
		return false
	}
}

func parseRecordShape(node *yaml.Node, context string) (ValueShape, error) {
	fields, err := mappingFields(node, context, []string{"kind", "fields"}, nil)
	if err != nil {
		return nil, err
	}
	members, err := parseShapeMembers(fields["fields"], context+".fields")
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return RecordValueShape{fields: members, span: span}, nil
}

func parseSumShape(node *yaml.Node, context string) (ValueShape, error) {
	fields, err := mappingFields(node, context, []string{"kind", "variants"}, nil)
	if err != nil {
		return nil, err
	}
	members, err := parseShapeMembers(fields["variants"], context+".variants")
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return SumValueShape{variants: members, span: span}, nil
}

func parseShapeMembers(node *yaml.Node, context string) ([]ShapeMember, error) {
	items, err := sequenceItems(node, context, false)
	if err != nil {
		return nil, err
	}
	members := make([]ShapeMember, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for index, item := range items {
		itemContext := fmt.Sprintf("%s[%d]", context, index)
		fields, fieldsErr := mappingFields(item, itemContext, []string{"name", "shape"}, nil)
		if fieldsErr != nil {
			return nil, fieldsErr
		}
		name, parseErr := sourceText(fields["name"], itemContext+".name")
		if parseErr != nil {
			return nil, parseErr
		}
		if strings.ContainsAny(name.value, " /\\") {
			return nil, fmt.Errorf("%s.name must not contain whitespace, slash, or backslash", itemContext)
		}
		shape, parseErr := qualifiedSourceText(fields["shape"], itemContext+".shape")
		if parseErr != nil {
			return nil, parseErr
		}
		if _, exists := seen[name.value]; exists {
			return nil, fmt.Errorf("%s contains duplicate member %q", context, name.value)
		}
		seen[name.value] = struct{}{}
		span, spanErr := nodeLineRange(item)
		if spanErr != nil {
			return nil, fmt.Errorf("%s source span: %w", itemContext, spanErr)
		}
		members = append(members, ShapeMember{name: name, shape: shape, span: span})
	}
	return members, nil
}

func parseCollectionShape(
	node *yaml.Node,
	context string,
	kind ValueShapeKind,
) (ValueShape, error) {
	fields, err := mappingFields(node, context, []string{"kind", "element_shape"}, nil)
	if err != nil {
		return nil, err
	}
	element, err := qualifiedSourceText(fields["element_shape"], context+".element_shape")
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return CollectionValueShape{kind: kind, element: element, span: span}, nil
}

func parseClaimGraphShape(node *yaml.Node, context string) (ValueShape, error) {
	_, err := mappingFields(node, context, []string{"kind"}, nil)
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return ClaimGraphValueShape{span: span}, nil
}

func parseCodecBindingDeclaration(node *yaml.Node, context string) (Declaration, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{
			"kind",
			"symbol",
			"value_kind",
			"value_shape",
			"canonicalization_version",
			"contract",
		},
		nil,
	)
	if err != nil {
		return nil, err
	}
	symbol, err := qualifiedSourceText(fields["symbol"], context+".symbol")
	if err != nil {
		return nil, err
	}
	valueKind, err := valueKindText(fields["value_kind"], context+".value_kind")
	if err != nil {
		return nil, err
	}
	valueShape, err := qualifiedSourceText(fields["value_shape"], context+".value_shape")
	if err != nil {
		return nil, err
	}
	version, err := sourceText(
		fields["canonicalization_version"],
		context+".canonicalization_version",
	)
	if err != nil {
		return nil, err
	}
	contract, err := parseUniqueSourceTextSequence(
		fields["contract"],
		context+".contract",
		false,
		sourceText,
	)
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return CodecBindingDeclaration{
		symbol:                  symbol,
		valueKind:               valueKind,
		valueShape:              valueShape,
		canonicalizationVersion: version,
		contract:                contract,
		span:                    span,
	}, nil
}

func parseRuntimeEvaluatorRequirementDeclaration(
	node *yaml.Node,
	context string,
) (Declaration, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{
			"kind",
			"symbol",
			"rule_ref",
			"invocation_contract",
		},
		nil,
	)
	if err != nil {
		return nil, err
	}
	symbol, err := qualifiedSourceText(fields["symbol"], context+".symbol")
	if err != nil {
		return nil, err
	}
	ruleRef, err := sourceText(fields["rule_ref"], context+".rule_ref")
	if err != nil {
		return nil, err
	}
	invocationContract, err := qualifiedSourceText(
		fields["invocation_contract"],
		context+".invocation_contract",
	)
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return RuntimeEvaluatorRequirementDeclaration{
		symbol:             symbol,
		ruleRef:            ruleRef,
		invocationContract: invocationContract,
		span:               span,
	}, nil
}

func parseConstraintDeclaration(node *yaml.Node, context string) (Declaration, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{"kind", "symbol", "rule"},
		nil,
	)
	if err != nil {
		return nil, err
	}
	symbol, err := qualifiedSourceText(fields["symbol"], context+".symbol")
	if err != nil {
		return nil, err
	}
	rule, err := parseConstraintRule(fields["rule"], context+".rule")
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return ConstraintDeclaration{symbol: symbol, rule: rule, span: span}, nil
}

func parseConstraintRule(node *yaml.Node, context string) (ConstraintRule, error) {
	initial, err := mappingFields(
		node,
		context,
		[]string{"kind"},
		[]string{
			"kinds",
			"relation",
			"slots",
			"slot",
			"mode",
			"cardinality",
			"subset",
			"superset",
			"whole",
			"parts",
		},
	)
	if err != nil {
		return nil, err
	}
	kind, err := sourceText(initial["kind"], context+".kind")
	if err != nil {
		return nil, err
	}
	switch ConstraintKind(kind.value) {
	case ConstraintKindDisjoint:
		return parseKindDisjointConstraint(node, context)
	case ConstraintSlotGroup:
		return parseSlotGroupConstraint(node, context)
	case ConstraintSlotCardinality:
		return parseSlotCardinalityConstraint(node, context)
	case ConstraintReferenceSlotSubset:
		return parseReferenceSlotSubsetConstraint(node, context)
	case ConstraintReferenceSlotPartition:
		return parseReferenceSlotPartitionConstraint(node, context)
	default:
		return nil, fmt.Errorf("%s.kind %q is not supported", context, kind.value)
	}
}

func parseKindDisjointConstraint(node *yaml.Node, context string) (ConstraintRule, error) {
	fields, err := mappingFields(node, context, []string{"kind", "kinds"}, nil)
	if err != nil {
		return nil, err
	}
	kinds, err := parseUniqueSourceTextSequence(
		fields["kinds"],
		context+".kinds",
		false,
		valueKindText,
	)
	if err != nil {
		return nil, err
	}
	if len(kinds) < 2 {
		return nil, fmt.Errorf("%s.kinds must contain at least two kinds", context)
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return KindDisjointConstraint{kinds: kinds, span: span}, nil
}

func parseSlotGroupConstraint(node *yaml.Node, context string) (ConstraintRule, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{"kind", "relation", "slots", "mode"},
		nil,
	)
	if err != nil {
		return nil, err
	}
	relation, err := qualifiedSourceText(fields["relation"], context+".relation")
	if err != nil {
		return nil, err
	}
	slots, err := parseUniqueSourceTextSequence(
		fields["slots"],
		context+".slots",
		false,
		slotKindText,
	)
	if err != nil {
		return nil, err
	}
	if len(slots) < 2 {
		return nil, fmt.Errorf("%s.slots must contain at least two SlotKinds", context)
	}
	modeText, err := sourceText(fields["mode"], context+".mode")
	if err != nil {
		return nil, err
	}
	mode := SlotGroupMode(modeText.value)
	if !supportedSlotGroupMode(mode) {
		return nil, fmt.Errorf("%s.mode %q is not supported", context, modeText.value)
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return SlotGroupConstraint{
		relation: relation,
		slots:    slots,
		mode:     mode,
		span:     span,
	}, nil
}

func parseSlotCardinalityConstraint(node *yaml.Node, context string) (ConstraintRule, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{"kind", "relation", "slot", "cardinality"},
		nil,
	)
	if err != nil {
		return nil, err
	}
	relation, err := qualifiedSourceText(fields["relation"], context+".relation")
	if err != nil {
		return nil, err
	}
	slot, err := slotKindText(fields["slot"], context+".slot")
	if err != nil {
		return nil, err
	}
	cardinality, err := parseCardinality(fields["cardinality"], context+".cardinality")
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return SlotCardinalityConstraint{
		relation:    relation,
		slot:        slot,
		cardinality: cardinality,
		span:        span,
	}, nil
}

func parseReferenceSlotSubsetConstraint(
	node *yaml.Node,
	context string,
) (ConstraintRule, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{"kind", "relation", "subset", "superset"},
		nil,
	)
	if err != nil {
		return nil, err
	}
	relation, err := qualifiedSourceText(fields["relation"], context+".relation")
	if err != nil {
		return nil, err
	}
	subset, err := slotKindText(fields["subset"], context+".subset")
	if err != nil {
		return nil, err
	}
	superset, err := slotKindText(fields["superset"], context+".superset")
	if err != nil {
		return nil, err
	}
	if subset.value == superset.value {
		return nil, fmt.Errorf("%s subset and superset coordinates must be distinct", context)
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return ReferenceSlotSubsetConstraint{
		relation: relation,
		subset:   subset,
		superset: superset,
		span:     span,
	}, nil
}

func parseReferenceSlotPartitionConstraint(
	node *yaml.Node,
	context string,
) (ConstraintRule, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{"kind", "relation", "whole", "parts"},
		nil,
	)
	if err != nil {
		return nil, err
	}
	relation, err := qualifiedSourceText(fields["relation"], context+".relation")
	if err != nil {
		return nil, err
	}
	whole, err := slotKindText(fields["whole"], context+".whole")
	if err != nil {
		return nil, err
	}
	parts, err := parseUniqueSourceTextSequence(
		fields["parts"],
		context+".parts",
		false,
		slotKindText,
	)
	if err != nil {
		return nil, err
	}
	if len(parts) < 2 {
		return nil, fmt.Errorf("%s.parts must contain at least two SlotKinds", context)
	}
	for _, part := range parts {
		if part.value == whole.value {
			return nil, fmt.Errorf("%s whole cannot also be a part", context)
		}
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return ReferenceSlotPartitionConstraint{
		relation: relation,
		whole:    whole,
		parts:    parts,
		span:     span,
	}, nil
}

func supportedSlotGroupMode(mode SlotGroupMode) bool {
	switch mode {
	case SlotGroupAllOrNone, SlotGroupAtLeastOne, SlotGroupExactlyOne:
		return true
	default:
		return false
	}
}

func slotKindText(node *yaml.Node, context string) (SourceText, error) {
	value, err := qualifiedSourceText(node, context)
	if err != nil {
		return SourceText{}, err
	}
	if !strings.HasSuffix(value.value, "Slot") {
		return SourceText{}, fmt.Errorf("%s %q must end with Slot under A.6.5", context, value.value)
	}
	return value, nil
}

func refKindText(node *yaml.Node, context string) (SourceText, error) {
	value, err := qualifiedSourceText(node, context)
	if err != nil {
		return SourceText{}, err
	}
	if !strings.HasSuffix(value.value, "Ref") {
		return SourceText{}, fmt.Errorf("%s %q must end with Ref under A.6.5", context, value.value)
	}
	return value, nil
}
