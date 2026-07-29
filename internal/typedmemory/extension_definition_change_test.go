package typedmemory

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestDefinitionSchemaChangesSeparateExportedSymbolsFromRuntimeCoordinates(t *testing.T) {
	factory := newExhaustiveExtensionFactory(t)
	changes := factory.membershipDefinitionChanges(
		typeEnvTestKindID(t, "Haft.ProjectMemory"),
		false,
	)
	entitySet := changes[0].(DefineEntitySetSchemaChange)
	kindSignature := changes[1].(DefineKindSignatureSchemaChange)

	t.Run("entity-set aliases cannot conceal one coordinate", func(t *testing.T) {
		alias := mustExtensionValue(NewEntitySetSymbolID("Haft.ProjectMemoryEntitySetAlias"))
		otherTypeEnv := typeEnvTestTypeEnvRef(t, 0xcf)
		otherDefinition := mustExtensionValue(NewEntitySetDefinition(EntitySetDefinitionInput{
			TypeEnv:         otherTypeEnv,
			Context:         entitySet.Definition().Ref().Context(),
			EnumerationRule: typeEnvTestRuleRef(t, "rule:entity-set:aliased"),
			CandidatePolicy: PersistedEntitiesOnly{},
			Provenance:      typeEnvTestFPFProvenance(t, "prov:fpf:entity-set:aliased", 0xcf),
		}))
		aliased := mustExtensionValue(NewDefineEntitySetSchemaChange(alias, otherDefinition))
		_, err := NewSchemaChangeSet([]SchemaChange{entitySet, aliased})
		if err == nil || !strings.Contains(err.Error(), "duplicate schema change") {
			t.Fatalf("duplicate-coordinate error = %v; want semantic-coordinate rejection", err)
		}
	})

	t.Run("entity-set symbol cannot bind two coordinates", func(t *testing.T) {
		other := mustExtensionValue(NewEntitySetDefinition(EntitySetDefinitionInput{
			TypeEnv:         factory.base,
			Context:         factory.otherContext,
			EnumerationRule: typeEnvTestRuleRef(t, "rule:entity-set:other"),
			CandidatePolicy: PersistedEntitiesOnly{},
			Provenance:      typeEnvTestFPFProvenance(t, "prov:fpf:entity-set:other", 0xd0),
		}))
		conflict := mustExtensionValue(NewDefineEntitySetSchemaChange(
			entitySet.ExportedSymbolID(),
			other,
		))
		_, err := NewSchemaChangeSet([]SchemaChange{entitySet, conflict})
		if err == nil || !strings.Contains(err.Error(), "cannot bind both") {
			t.Fatalf("duplicate-symbol error = %v; want exported-symbol conflict", err)
		}
	})

	t.Run("kind-signature aliases cannot conceal one coordinate", func(t *testing.T) {
		alias := mustExtensionValue(NewKindSignatureSymbolID(
			"Haft.ProjectMemoryKindSignatureAlias",
		))
		definition := kindSignature.Definition()
		otherTypeEnv := typeEnvTestTypeEnvRef(t, 0xd4)
		otherEntitySet := mustExtensionValue(NewEntitySetDefinition(EntitySetDefinitionInput{
			TypeEnv:         otherTypeEnv,
			Context:         definition.Ref().Context(),
			EnumerationRule: typeEnvTestRuleRef(t, "rule:entity-set:kind-signature-alias"),
			CandidatePolicy: PersistedEntitiesOnly{},
			Provenance:      typeEnvTestFPFProvenance(t, "prov:fpf:kind-signature-alias-set", 0xd4),
		}))
		otherDefinition := mustExtensionValue(NewKindSignatureDefinition(KindSignatureDefinitionInput{
			ValueKind: typeEnvTestValueKindRef(
				t,
				otherTypeEnv,
				definition.ValueKind().ID(),
			),
			Formality:       definition.Formality(),
			Assumptions:     definition.Assumptions(),
			DefinednessRule: definition.DefinednessRule(),
			Evaluator:       definition.Evaluator(),
			EntitySet:       otherEntitySet.Ref(),
			Provenance:      typeEnvTestFPFProvenance(t, "prov:fpf:kind-signature:aliased", 0xd5),
		}))
		aliased := mustExtensionValue(NewDefineKindSignatureSchemaChange(
			alias,
			otherDefinition,
		))
		_, err := NewSchemaChangeSet([]SchemaChange{kindSignature, aliased})
		if err == nil || !strings.Contains(err.Error(), "duplicate schema change") {
			t.Fatalf("duplicate-coordinate error = %v; want semantic-coordinate rejection", err)
		}
	})

	t.Run("kind-signature symbol cannot bind two coordinates", func(t *testing.T) {
		definition := kindSignature.Definition()
		otherKind := typeEnvTestValueKindRef(
			t,
			factory.base,
			typeEnvTestKindID(t, "Haft.OtherProjectMemory"),
		)
		other := mustExtensionValue(NewKindSignatureDefinition(KindSignatureDefinitionInput{
			ValueKind:       otherKind,
			Formality:       definition.Formality(),
			Assumptions:     definition.Assumptions(),
			DefinednessRule: definition.DefinednessRule(),
			Evaluator:       definition.Evaluator(),
			EntitySet:       definition.EntitySet(),
			Provenance:      typeEnvTestFPFProvenance(t, "prov:fpf:kind-signature:other", 0xd1),
		}))
		conflict := mustExtensionValue(NewDefineKindSignatureSchemaChange(
			kindSignature.ExportedSymbolID(),
			other,
		))
		_, err := NewSchemaChangeSet([]SchemaChange{kindSignature, conflict})
		if err == nil || !strings.Contains(err.Error(), "cannot bind both") {
			t.Fatalf("duplicate-symbol error = %v; want exported-symbol conflict", err)
		}
	})

	entitySetProvided := entitySet.ProvidedSymbols()
	if len(entitySetProvided) != 1 ||
		entitySetProvided[0].Key() != entitySet.ExportedSymbolID().String() ||
		entitySetProvided[0].Key() == entitySet.Definition().Ref().Context().String() {
		t.Fatalf("EntitySet provide %v is not its explicit exported symbol", entitySetProvided)
	}
	kindSignatureProvided := kindSignature.ProvidedSymbols()
	if len(kindSignatureProvided) != 1 ||
		kindSignatureProvided[0].Key() != kindSignature.ExportedSymbolID().String() ||
		kindSignatureProvided[0].Key() == kindSignature.Definition().Ref().key() {
		t.Fatalf("KindSignature provide %v is not its explicit exported symbol", kindSignatureProvided)
	}
}

func TestDefinitionSchemaChangeCodecRejectsMalformedNestedContent(t *testing.T) {
	factory := newExhaustiveExtensionFactory(t)
	changes := factory.membershipDefinitionChanges(
		typeEnvTestKindID(t, "Haft.ProjectMemory"),
		false,
	)

	for _, change := range changes {
		t.Run(change.ChangeKey(), func(t *testing.T) {
			envelope := mustExtensionValue(encodeSchemaChange(change))
			decoded, err := decodeSchemaChange(envelope)
			if err != nil {
				t.Fatalf("decodeSchemaChange() error = %v", err)
			}
			if !reflect.DeepEqual(decoded, change) {
				t.Fatal("definition SchemaChange round trip changed semantic content")
			}

			unknown := addUnknownJSONField(t, envelope.Payload)
			_, err = decodeSchemaChange(schemaChangeEnvelopeV1{
				Kind:    envelope.Kind,
				Payload: unknown,
			})
			if err == nil || !strings.Contains(err.Error(), "unknown field") {
				t.Fatalf("unknown-field error = %v; want strict rejection", err)
			}

			duplicate := duplicateFirstJSONField(t, envelope.Payload)
			_, err = decodeSchemaChange(schemaChangeEnvelopeV1{
				Kind:    envelope.Kind,
				Payload: duplicate,
			})
			if err == nil || !strings.Contains(err.Error(), "duplicate field") {
				t.Fatalf("duplicate-field error = %v; want strict rejection", err)
			}

			_, err = decodeSchemaChange(schemaChangeEnvelopeV1{
				Kind:    envelope.Kind,
				Payload: append(append([]byte(nil), envelope.Payload...), []byte(` {}`)...),
			})
			if err == nil || !strings.Contains(err.Error(), "trailing") {
				t.Fatalf("trailing-value error = %v; want strict rejection", err)
			}
		})
	}

	entityEnvelope := mustExtensionValue(encodeSchemaChange(changes[0]))
	var entityEncoded entitySetDefinitionChangeV1
	if err := decodeStrictJSON(entityEnvelope.Payload, &entityEncoded); err != nil {
		t.Fatalf("decode EntitySet fixture: %v", err)
	}
	entityEncoded.Reference.Digest = typeEnvTestDigest(t, 0xd2).String()
	forgedEntity := mustExtensionValue(json.Marshal(entityEncoded))
	if _, err := decodeEntitySetDefinitionChange(forgedEntity); err == nil ||
		!strings.Contains(err.Error(), "does not match decoded content") {
		t.Fatalf("forged EntitySet reference error = %v; want exact-content mismatch", err)
	}

	var policy map[string]any
	if err := json.Unmarshal(entityEncoded.CandidatePolicy.Payload, &policy); err != nil {
		t.Fatalf("decode EntitySet policy: %v", err)
	}
	policy["unknown_future_field"] = true
	entityEncoded.Reference = encodeEntitySetDefinitionRef(
		changes[0].(DefineEntitySetSchemaChange).Definition().Ref(),
	)
	entityEncoded.CandidatePolicy.Payload = mustExtensionValue(json.Marshal(policy))
	unknownPolicy := mustExtensionValue(json.Marshal(entityEncoded))
	if _, err := decodeEntitySetDefinitionChange(unknownPolicy); err == nil ||
		!strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown EntitySet-policy field error = %v; want strict nested rejection", err)
	}

	kindEnvelope := mustExtensionValue(encodeSchemaChange(changes[1]))
	var kindEncoded kindSignatureDefinitionChangeV1
	if err := decodeStrictJSON(kindEnvelope.Payload, &kindEncoded); err != nil {
		t.Fatalf("decode KindSignature fixture: %v", err)
	}
	kindEncoded.Reference.Digest = typeEnvTestDigest(t, 0xd3).String()
	forgedKind := mustExtensionValue(json.Marshal(kindEncoded))
	if _, err := decodeKindSignatureDefinitionChange(forgedKind); err == nil ||
		!strings.Contains(err.Error(), "does not match decoded content") {
		t.Fatalf("forged KindSignature reference error = %v; want exact-content mismatch", err)
	}

	if len(kindEncoded.Assumptions) == 0 {
		t.Fatal("KindSignature fixture requires at least one assumption")
	}
	kindEncoded.Reference = encodeKindSignatureRef(
		changes[1].(DefineKindSignatureSchemaChange).Definition().Ref(),
	)
	kindEncoded.Assumptions = append(kindEncoded.Assumptions, kindEncoded.Assumptions[0])
	duplicateAssumption := mustExtensionValue(json.Marshal(kindEncoded))
	if _, err := decodeKindSignatureDefinitionChange(duplicateAssumption); err == nil ||
		!strings.Contains(err.Error(), "duplicate KindSignature assumption") {
		t.Fatalf("duplicate-assumption error = %v; want exact duplicate rejection", err)
	}
}

func TestDefinitionSchemaChangeCodecRoundTripsPersistedEntitySetPolicy(t *testing.T) {
	factory := newExhaustiveExtensionFactory(t)
	symbolID := mustExtensionValue(NewEntitySetSymbolID("Haft.PersistedEntitySet"))
	symbol := mustExtensionValue(EntitySetSymbolRef(symbolID))
	definition := mustExtensionValue(NewEntitySetDefinition(EntitySetDefinitionInput{
		TypeEnv:         factory.base,
		Context:         factory.context,
		EnumerationRule: typeEnvTestRuleRef(t, "rule:entity-set:persisted-only"),
		CandidatePolicy: PersistedEntitiesOnly{},
		Provenance:      factory.projectProvenance(symbol, VocabularyRow, ManifestProvide),
	}))
	change := mustExtensionValue(NewDefineEntitySetSchemaChange(symbolID, definition))
	envelope := mustExtensionValue(encodeSchemaChange(change))
	decoded, err := decodeSchemaChange(envelope)
	if err != nil {
		t.Fatalf("decodeSchemaChange() error = %v", err)
	}
	if !reflect.DeepEqual(decoded, change) {
		t.Fatal("persisted-only EntitySet policy changed during round trip")
	}
}

func TestDefinitionSchemaChangesAreDefensiveAndNonBinding(t *testing.T) {
	factory := newExhaustiveExtensionFactory(t)
	changes := factory.membershipDefinitionChanges(
		typeEnvTestKindID(t, "Haft.ProjectMemory"),
		false,
	)
	for _, change := range changes {
		provided := change.ProvidedSymbols()
		provided[0] = SchemaSymbolRef{}
		if !change.ProvidedSymbols()[0].valid() {
			t.Fatalf("%T leaked mutable provided-symbol storage", change)
		}
		if _, isMemoryChange := any(change).(MemoryChange); isMemoryChange {
			t.Fatalf("%T is also a MemoryChange", change)
		}
		changeType := reflect.TypeOf(change)
		for _, forbidden := range []string{"Activate", "Admit", "Apply"} {
			if _, exists := changeType.MethodByName(forbidden); exists {
				t.Fatalf("non-binding %T exposes %s", change, forbidden)
			}
		}
	}

	entitySet := changes[0].(DefineEntitySetSchemaChange)
	entityBytes := entitySet.Definition().CanonicalBytes()
	entityBytes[0] ^= 0xff
	if reflect.DeepEqual(entityBytes, entitySet.Definition().CanonicalBytes()) {
		t.Fatal("EntitySet definition leaked mutable canonical bytes")
	}

	kindSignature := changes[1].(DefineKindSignatureSchemaChange)
	assumptions := kindSignature.Definition().Assumptions()
	assumptions[0] = KindAssumptionPin{}
	if !kindSignature.Definition().Assumptions()[0].valid() {
		t.Fatal("KindSignature definition leaked mutable assumptions")
	}
}

func addUnknownJSONField(t *testing.T, payload []byte) []byte {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(payload, &value); err != nil {
		t.Fatalf("json.Unmarshal(): %v", err)
	}
	value["unknown_future_field"] = true
	return mustExtensionValue(json.Marshal(value))
}

func duplicateFirstJSONField(t *testing.T, payload []byte) []byte {
	t.Helper()
	var value map[string]json.RawMessage
	if err := json.Unmarshal(payload, &value); err != nil {
		t.Fatalf("json.Unmarshal(): %v", err)
	}
	raw, exists := value["exported_symbol"]
	if !exists {
		t.Fatal("definition payload has no exported_symbol")
	}
	prefix := []byte(`{"exported_symbol":`)
	result := append([]byte(nil), prefix...)
	result = append(result, raw...)
	result = append(result, ',')
	result = append(result, payload[1:]...)
	return result
}
