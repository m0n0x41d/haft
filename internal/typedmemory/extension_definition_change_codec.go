package typedmemory

import (
	"encoding/json"
	"fmt"
)

type entitySetDefinitionRefV1 struct {
	TypeEnv string `json:"type_env"`
	Context string `json:"context"`
	Digest  string `json:"digest"`
}

type entitySetDefinitionChangeV1 struct {
	ExportedSymbol  schemaSymbolV1                     `json:"exported_symbol"`
	Reference       entitySetDefinitionRefV1           `json:"reference"`
	EnumerationRule string                             `json:"enumeration_rule"`
	CandidatePolicy entitySetCandidatePolicyEnvelopeV1 `json:"candidate_policy"`
	Provenance      declarationProvenanceEnvelopeV1    `json:"provenance"`
}

type entitySetCandidatePolicyEnvelopeV1 struct {
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

type persistedEntitiesOnlyV1 struct{}

type priorBatchDeclarationsVisibleV1 struct {
	EvaluationRule string `json:"evaluation_rule"`
}

type kindSignatureRefV1 struct {
	ValueKind typeEnvIDRefV1 `json:"value_kind"`
	Context   string         `json:"context"`
	Digest    string         `json:"digest"`
}

type kindAssumptionPinV1 struct {
	Carrier string `json:"carrier"`
	Edition string `json:"edition"`
	Digest  string `json:"digest"`
}

type kindSignatureDefinitionChangeV1 struct {
	ExportedSymbol  schemaSymbolV1                  `json:"exported_symbol"`
	Reference       kindSignatureRefV1              `json:"reference"`
	Formality       string                          `json:"formality"`
	Assumptions     []kindAssumptionPinV1           `json:"assumptions"`
	DefinednessRule string                          `json:"definedness_rule"`
	Evaluator       string                          `json:"evaluator"`
	EntitySet       entitySetDefinitionRefV1        `json:"entity_set"`
	Provenance      declarationProvenanceEnvelopeV1 `json:"provenance"`
}

func encodeEntitySetDefinitionChange(
	change DefineEntitySetSchemaChange,
) (entitySetDefinitionChangeV1, error) {
	definition := change.Definition()
	provenance, err := encodeDeclarationProvenance(definition.Provenance())
	if err != nil {
		return entitySetDefinitionChangeV1{}, err
	}
	policy, err := encodeEntitySetCandidatePolicy(definition.CandidatePolicy())
	if err != nil {
		return entitySetDefinitionChangeV1{}, err
	}
	return entitySetDefinitionChangeV1{
		ExportedSymbol:  encodeSchemaSymbol(change.ExportedSymbol()),
		Reference:       encodeEntitySetDefinitionRef(definition.Ref()),
		EnumerationRule: definition.EnumerationRule().String(),
		CandidatePolicy: policy,
		Provenance:      provenance,
	}, nil
}

func decodeEntitySetDefinitionChange(payload []byte) (SchemaChange, error) {
	encoded, err := decodeVariantPayload[entitySetDefinitionChangeV1](
		payload,
		"define_entity_set",
	)
	if err != nil {
		return nil, err
	}
	exportedSymbol, err := decodeEntitySetSymbolID(encoded.ExportedSymbol)
	if err != nil {
		return nil, fmt.Errorf("EntitySet exported symbol: %w", err)
	}
	expectedRef, err := decodeEntitySetDefinitionRef(encoded.Reference)
	if err != nil {
		return nil, fmt.Errorf("EntitySet definition reference: %w", err)
	}
	enumerationRule, err := NewRuleRef(encoded.EnumerationRule)
	if err != nil {
		return nil, fmt.Errorf("EntitySet enumeration rule: %w", err)
	}
	policy, err := decodeEntitySetCandidatePolicy(encoded.CandidatePolicy)
	if err != nil {
		return nil, err
	}
	provenance, err := decodeDeclarationProvenance(encoded.Provenance)
	if err != nil {
		return nil, err
	}
	definition, err := NewEntitySetDefinition(EntitySetDefinitionInput{
		TypeEnv:         expectedRef.TypeEnv(),
		Context:         expectedRef.Context(),
		EnumerationRule: enumerationRule,
		CandidatePolicy: policy,
		Provenance:      provenance,
	})
	if err != nil {
		return nil, err
	}
	if definition.Ref() != expectedRef {
		return nil, fmt.Errorf("EntitySet definition reference does not match decoded content")
	}
	change, err := NewDefineEntitySetSchemaChange(exportedSymbol, definition)
	if err != nil {
		return nil, err
	}
	return change, nil
}

func encodeKindSignatureDefinitionChange(
	change DefineKindSignatureSchemaChange,
) (kindSignatureDefinitionChangeV1, error) {
	definition := change.Definition()
	provenance, err := encodeDeclarationProvenance(definition.Provenance())
	if err != nil {
		return kindSignatureDefinitionChangeV1{}, err
	}
	assumptions := make([]kindAssumptionPinV1, 0, len(definition.Assumptions()))
	for _, assumption := range definition.Assumptions() {
		assumptions = append(assumptions, encodeKindAssumptionPin(assumption))
	}
	return kindSignatureDefinitionChangeV1{
		ExportedSymbol:  encodeSchemaSymbol(change.ExportedSymbol()),
		Reference:       encodeKindSignatureRef(definition.Ref()),
		Formality:       definition.Formality().String(),
		Assumptions:     assumptions,
		DefinednessRule: definition.DefinednessRule().String(),
		Evaluator:       definition.Evaluator().String(),
		EntitySet:       encodeEntitySetDefinitionRef(definition.EntitySet()),
		Provenance:      provenance,
	}, nil
}

func decodeKindSignatureDefinitionChange(payload []byte) (SchemaChange, error) {
	encoded, err := decodeVariantPayload[kindSignatureDefinitionChangeV1](
		payload,
		"define_kind_signature",
	)
	if err != nil {
		return nil, err
	}
	exportedSymbol, err := decodeKindSignatureSymbolID(encoded.ExportedSymbol)
	if err != nil {
		return nil, fmt.Errorf("KindSignature exported symbol: %w", err)
	}
	expectedRef, err := decodeKindSignatureRef(encoded.Reference)
	if err != nil {
		return nil, fmt.Errorf("KindSignature reference: %w", err)
	}
	formality, err := parseSignatureFormality(encoded.Formality)
	if err != nil {
		return nil, err
	}
	assumptions, err := decodeKindAssumptionPins(encoded.Assumptions)
	if err != nil {
		return nil, err
	}
	definednessRule, err := NewRuleRef(encoded.DefinednessRule)
	if err != nil {
		return nil, fmt.Errorf("KindSignature definedness rule: %w", err)
	}
	evaluator, err := NewRuleRef(encoded.Evaluator)
	if err != nil {
		return nil, fmt.Errorf("KindSignature evaluator: %w", err)
	}
	entitySet, err := decodeEntitySetDefinitionRef(encoded.EntitySet)
	if err != nil {
		return nil, fmt.Errorf("KindSignature EntitySet reference: %w", err)
	}
	provenance, err := decodeDeclarationProvenance(encoded.Provenance)
	if err != nil {
		return nil, err
	}
	definition, err := NewKindSignatureDefinition(KindSignatureDefinitionInput{
		ValueKind:       expectedRef.ValueKind(),
		Formality:       formality,
		Assumptions:     assumptions,
		DefinednessRule: definednessRule,
		Evaluator:       evaluator,
		EntitySet:       entitySet,
		Provenance:      provenance,
	})
	if err != nil {
		return nil, err
	}
	if definition.Ref() != expectedRef {
		return nil, fmt.Errorf("KindSignature reference does not match decoded content")
	}
	change, err := NewDefineKindSignatureSchemaChange(exportedSymbol, definition)
	if err != nil {
		return nil, err
	}
	return change, nil
}

func encodeEntitySetCandidatePolicy(
	policy EntitySetCandidatePolicy,
) (entitySetCandidatePolicyEnvelopeV1, error) {
	var kind string
	var payload any
	switch value := policy.(type) {
	case PersistedEntitiesOnly:
		kind = "persisted_entities_only"
		payload = persistedEntitiesOnlyV1{}
	case PriorBatchDeclarationsVisible:
		kind = "prior_batch_declarations_visible"
		payload = priorBatchDeclarationsVisibleV1{
			EvaluationRule: value.EvaluationRule().String(),
		}
	default:
		return entitySetCandidatePolicyEnvelopeV1{}, fmt.Errorf(
			"unknown EntitySet candidate policy %T",
			policy,
		)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return entitySetCandidatePolicyEnvelopeV1{}, err
	}
	return entitySetCandidatePolicyEnvelopeV1{Kind: kind, Payload: encoded}, nil
}

func decodeEntitySetCandidatePolicy(
	envelope entitySetCandidatePolicyEnvelopeV1,
) (EntitySetCandidatePolicy, error) {
	switch envelope.Kind {
	case "persisted_entities_only":
		if _, err := decodeVariantPayload[persistedEntitiesOnlyV1](
			envelope.Payload,
			"persisted_entities_only EntitySet candidate policy",
		); err != nil {
			return nil, err
		}
		return PersistedEntitiesOnly{}, nil
	case "prior_batch_declarations_visible":
		payload, err := decodeVariantPayload[priorBatchDeclarationsVisibleV1](
			envelope.Payload,
			"prior_batch_declarations_visible EntitySet candidate policy",
		)
		if err != nil {
			return nil, err
		}
		rule, err := NewRuleRef(payload.EvaluationRule)
		if err != nil {
			return nil, err
		}
		policy, err := NewPriorBatchDeclarationsVisible(rule)
		if err != nil {
			return nil, err
		}
		return policy, nil
	default:
		return nil, fmt.Errorf("unknown EntitySet candidate policy %q", envelope.Kind)
	}
}

func encodeEntitySetDefinitionRef(ref EntitySetDefinitionRef) entitySetDefinitionRefV1 {
	return entitySetDefinitionRefV1{
		TypeEnv: ref.TypeEnv().String(),
		Context: ref.Context().String(),
		Digest:  ref.Digest().String(),
	}
}

func decodeEntitySetDefinitionRef(
	encoded entitySetDefinitionRefV1,
) (EntitySetDefinitionRef, error) {
	typeEnv, err := ParseTypeEnvRef(encoded.TypeEnv)
	if err != nil {
		return EntitySetDefinitionRef{}, err
	}
	context, err := NewBoundedContextRef(encoded.Context)
	if err != nil {
		return EntitySetDefinitionRef{}, err
	}
	digest, err := NewSHA256Digest(encoded.Digest)
	if err != nil {
		return EntitySetDefinitionRef{}, err
	}
	ref, err := NewEntitySetDefinitionRef(typeEnv, context, digest)
	if err != nil {
		return EntitySetDefinitionRef{}, err
	}
	return ref, nil
}

func encodeKindSignatureRef(ref KindSignatureRef) kindSignatureRefV1 {
	return kindSignatureRefV1{
		ValueKind: encodeValueKindRef(ref.ValueKind()),
		Context:   ref.Context().String(),
		Digest:    ref.Digest().String(),
	}
}

func decodeKindSignatureRef(encoded kindSignatureRefV1) (KindSignatureRef, error) {
	valueKind, err := decodeValueKindRef(encoded.ValueKind)
	if err != nil {
		return KindSignatureRef{}, err
	}
	context, err := NewBoundedContextRef(encoded.Context)
	if err != nil {
		return KindSignatureRef{}, err
	}
	digest, err := NewSHA256Digest(encoded.Digest)
	if err != nil {
		return KindSignatureRef{}, err
	}
	ref, err := NewKindSignatureRef(valueKind, context, digest)
	if err != nil {
		return KindSignatureRef{}, err
	}
	return ref, nil
}

func encodeKindAssumptionPin(pin KindAssumptionPin) kindAssumptionPinV1 {
	return kindAssumptionPinV1{
		Carrier: pin.Reference().String(),
		Edition: pin.Edition().String(),
		Digest:  pin.Digest().String(),
	}
}

func decodeKindAssumptionPins(
	encoded []kindAssumptionPinV1,
) ([]KindAssumptionPin, error) {
	result := make([]KindAssumptionPin, 0, len(encoded))
	seen := make(map[KindAssumptionPin]struct{}, len(encoded))
	for index, value := range encoded {
		carrier, err := NewCarrierRef(value.Carrier)
		if err != nil {
			return nil, fmt.Errorf("KindSignature assumption %d carrier: %w", index, err)
		}
		edition, err := NewCarrierEdition(value.Edition)
		if err != nil {
			return nil, fmt.Errorf("KindSignature assumption %d edition: %w", index, err)
		}
		digest, err := NewSHA256Digest(value.Digest)
		if err != nil {
			return nil, fmt.Errorf("KindSignature assumption %d digest: %w", index, err)
		}
		pin, err := NewKindAssumptionPin(carrier, edition, digest)
		if err != nil {
			return nil, fmt.Errorf("KindSignature assumption %d: %w", index, err)
		}
		if _, duplicate := seen[pin]; duplicate {
			return nil, fmt.Errorf("duplicate KindSignature assumption at index %d", index)
		}
		seen[pin] = struct{}{}
		result = append(result, pin)
	}
	return result, nil
}

func parseSignatureFormality(raw string) (SignatureFormality, error) {
	for candidate := SignatureF0; candidate <= SignatureF9; candidate++ {
		if candidate.String() == raw {
			return candidate, nil
		}
	}
	return 0, fmt.Errorf("unknown KindSignature formality %q", raw)
}

func decodeEntitySetSymbolID(encoded schemaSymbolV1) (EntitySetSymbolID, error) {
	symbol, err := decodeSchemaSymbol(encoded)
	if err != nil {
		return EntitySetSymbolID{}, err
	}
	if symbol.Kind() != EntitySetSymbol {
		return EntitySetSymbolID{}, fmt.Errorf("symbol kind must be entity_set")
	}
	id, err := NewEntitySetSymbolID(symbol.Key())
	if err != nil {
		return EntitySetSymbolID{}, err
	}
	return id, nil
}

func decodeKindSignatureSymbolID(encoded schemaSymbolV1) (KindSignatureSymbolID, error) {
	symbol, err := decodeSchemaSymbol(encoded)
	if err != nil {
		return KindSignatureSymbolID{}, err
	}
	if symbol.Kind() != KindSignatureSymbol {
		return KindSignatureSymbolID{}, fmt.Errorf("symbol kind must be kind_signature")
	}
	id, err := NewKindSignatureSymbolID(symbol.Key())
	if err != nil {
		return KindSignatureSymbolID{}, err
	}
	return id, nil
}
