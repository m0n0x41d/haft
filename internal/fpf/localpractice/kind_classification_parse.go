package localpractice

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func parseKindClassificationSignatureDeclaration(
	node *yaml.Node,
	context string,
) (Declaration, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{
			"kind",
			"symbol",
			"local_kind",
			"candidate_value_kind",
			"formality",
			"criterion_rule",
			"slice_conditions_rule",
			"reference_scheme",
			"dependencies",
			"extent_rule",
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
	localKind, err := valueKindText(fields["local_kind"], context+".local_kind")
	if err != nil {
		return nil, err
	}
	candidateValueKind, err := valueKindText(
		fields["candidate_value_kind"],
		context+".candidate_value_kind",
	)
	if err != nil {
		return nil, err
	}
	formality, err := parseSignatureFormality(fields["formality"], context+".formality")
	if err != nil {
		return nil, err
	}
	criterionRule, err := sourceText(fields["criterion_rule"], context+".criterion_rule")
	if err != nil {
		return nil, err
	}
	sliceConditions, err := sourceText(
		fields["slice_conditions_rule"],
		context+".slice_conditions_rule",
	)
	if err != nil {
		return nil, err
	}
	referenceScheme, err := parseKindClassificationReferenceScheme(
		fields["reference_scheme"],
		context+".reference_scheme",
	)
	if err != nil {
		return nil, err
	}
	dependencies, err := parseKindClassificationDependencies(
		fields["dependencies"],
		context+".dependencies",
	)
	if err != nil {
		return nil, err
	}
	extentRule, err := parseKindClassificationExtentRule(
		fields["extent_rule"],
		context+".extent_rule",
	)
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	return KindClassificationSignatureDeclaration{
		symbol:             symbol,
		localKind:          localKind,
		candidateValueKind: candidateValueKind,
		formality:          formality,
		criterionRule:      criterionRule,
		sliceConditions:    sliceConditions,
		referenceScheme:    referenceScheme,
		dependencies:       dependencies,
		extentRule:         extentRule,
		span:               span,
	}, nil
}

func parseKindClassificationReferenceScheme(
	node *yaml.Node,
	context string,
) (KindClassificationReferenceSchemePin, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{"carrier_ref", "edition", "digest"},
		nil,
	)
	if err != nil {
		return KindClassificationReferenceSchemePin{}, err
	}
	carrierRef, err := sourceText(fields["carrier_ref"], context+".carrier_ref")
	if err != nil {
		return KindClassificationReferenceSchemePin{}, err
	}
	edition, err := sourceText(fields["edition"], context+".edition")
	if err != nil {
		return KindClassificationReferenceSchemePin{}, err
	}
	digest, err := sourceText(fields["digest"], context+".digest")
	if err != nil {
		return KindClassificationReferenceSchemePin{}, err
	}
	if err := validateSHA256Digest(digest.value); err != nil {
		return KindClassificationReferenceSchemePin{}, fmt.Errorf(
			"%s.digest %q %w",
			context,
			digest.value,
			err,
		)
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return KindClassificationReferenceSchemePin{}, fmt.Errorf(
			"%s source span: %w",
			context,
			err,
		)
	}
	return KindClassificationReferenceSchemePin{
		carrierRef: carrierRef,
		edition:    edition,
		digest:     digest,
		span:       span,
	}, nil
}

func parseKindClassificationDependencies(
	node *yaml.Node,
	context string,
) ([]KindClassificationDependency, error) {
	items, err := sequenceItems(node, context, true)
	if err != nil {
		return nil, err
	}
	dependencies := make([]KindClassificationDependency, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for index, item := range items {
		itemContext := fmt.Sprintf("%s[%d]", context, index)
		fields, fieldsErr := mappingFields(
			item,
			itemContext,
			[]string{"kind", "carrier_ref", "edition", "digest"},
			nil,
		)
		if fieldsErr != nil {
			return nil, fieldsErr
		}
		kindSource, parseErr := sourceText(fields["kind"], itemContext+".kind")
		if parseErr != nil {
			return nil, parseErr
		}
		kind := KindClassificationDependencyKind(kindSource.value)
		if !kind.valid() {
			return nil, fmt.Errorf(
				"%s.kind %q is not supported",
				itemContext,
				kindSource.value,
			)
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
			return nil, fmt.Errorf(
				"%s.digest %q %w",
				itemContext,
				digest.value,
				digestErr,
			)
		}
		key := kindSource.value + "\x00" + carrierRef.value
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf(
				"%s repeats dependency %q:%q",
				context,
				kindSource.value,
				carrierRef.value,
			)
		}
		seen[key] = struct{}{}
		span, rangeErr := nodeLineRange(item)
		if rangeErr != nil {
			return nil, fmt.Errorf("%s source span: %w", itemContext, rangeErr)
		}
		dependencies = append(dependencies, KindClassificationDependency{
			kind:       kind,
			kindSource: kindSource,
			carrierRef: carrierRef,
			edition:    edition,
			digest:     digest,
			span:       span,
		})
	}
	return dependencies, nil
}

func parseKindClassificationExtentRule(
	node *yaml.Node,
	context string,
) (KindClassificationExtentRule, error) {
	initial, err := mappingFields(node, context, []string{"kind"}, []string{"rule_ref"})
	if err != nil {
		return nil, err
	}
	kindSource, err := sourceText(initial["kind"], context+".kind")
	if err != nil {
		return nil, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return nil, fmt.Errorf("%s source span: %w", context, err)
	}
	switch KindClassificationExtentRuleKind(kindSource.value) {
	case KindClassificationNoExtentRule:
		if _, exactErr := mappingFields(node, context, []string{"kind"}, nil); exactErr != nil {
			return nil, exactErr
		}
		return NoKindClassificationExtentRule{span: span}, nil
	case KindClassificationDeclaredExtentRule:
		fields, exactErr := mappingFields(
			node,
			context,
			[]string{"kind", "rule_ref"},
			nil,
		)
		if exactErr != nil {
			return nil, exactErr
		}
		ruleRef, parseErr := sourceText(fields["rule_ref"], context+".rule_ref")
		if parseErr != nil {
			return nil, parseErr
		}
		return DeclaredKindClassificationExtentRule{
			ruleRef: ruleRef,
			span:    span,
		}, nil
	default:
		return nil, fmt.Errorf(
			"%s.kind %q is not supported; want none or declared",
			context,
			kindSource.value,
		)
	}
}
