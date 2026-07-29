package typedmemory

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestLoweredTypeEnvExtensionProposalContentAddressedRoundTrip(t *testing.T) {
	proposal := newExtensionFixture(t).build(t)
	canonical := proposal.CanonicalBytes()
	const goldenRef = "typeenv-extension:haft.typed-memory.v1@sha256:4d88ce65aaeb8fba10477329494381aafe29d0833a3357b4c140da7e50d0b923"
	if proposal.Ref().String() != goldenRef {
		t.Fatalf("lowered extension ref = %q; want golden %q", proposal.Ref().String(), goldenRef)
	}

	if proposal.Ref().Digest() != digestCanonicalBytes(canonical) {
		t.Fatal("proposal digest is not SHA-256 of exact canonical bytes")
	}
	parsed, err := ParseTypeEnvExtensionRef(proposal.Ref().String())
	if err != nil {
		t.Fatalf("ParseTypeEnvExtensionRef() error = %v", err)
	}
	if parsed != proposal.Ref() {
		t.Fatal("parsed reference differs from derived reference")
	}
	decoded, err := DecodeLoweredTypeEnvExtensionProposal(canonical)
	if err != nil {
		t.Fatalf("DecodeLoweredTypeEnvExtensionProposal() error = %v", err)
	}
	if decoded.Ref() != proposal.Ref() {
		t.Fatal("decoded proposal changed its content-derived reference")
	}
	if !bytes.Equal(decoded.CanonicalBytes(), canonical) {
		t.Fatal("decoded proposal changed exact canonical bytes")
	}
	verified, err := VerifyLoweredTypeEnvExtensionProposal(proposal.Ref(), canonical)
	if err != nil {
		t.Fatalf("VerifyLoweredTypeEnvExtensionProposal() error = %v", err)
	}
	if !reflect.DeepEqual(verified, proposal) {
		t.Fatal("verified proposal lost semantic fields")
	}

	canonical[0] ^= 0xff
	if bytes.Equal(canonical, proposal.CanonicalBytes()) {
		t.Fatal("CanonicalBytes accessor leaked mutable storage")
	}
}

func TestLoweredTypeEnvExtensionProposalRejectsForgedAndMalformedArtifacts(t *testing.T) {
	proposal := newExtensionFixture(t).build(t)
	canonical := proposal.CanonicalBytes()
	forgedRaw := "typeenv-extension:" + proposal.Ref().ID().String() + "@" + typeEnvTestDigest(t, 0xee).String()
	forged, err := ParseTypeEnvExtensionRef(forgedRaw)
	if err != nil {
		t.Fatalf("ParseTypeEnvExtensionRef(forged) error = %v", err)
	}
	if _, err := VerifyLoweredTypeEnvExtensionProposal(forged, canonical); err == nil {
		t.Fatal("Verify accepted a forged reference")
	}

	mutated := append([]byte(nil), canonical...)
	mutated[len(mutated)-1] ^= 1
	if _, err := VerifyLoweredTypeEnvExtensionProposal(proposal.Ref(), mutated); err == nil {
		t.Fatal("Verify accepted byte mutation under the old reference")
	}
	trailing := append(append([]byte(nil), canonical...), 0)
	if _, err := DecodeLoweredTypeEnvExtensionProposal(trailing); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("trailing-byte error = %v; want explicit trailing rejection", err)
	}

	unknownVariant := rewriteExtensionPayload(t, canonical, func(encoded *extensionCanonicalV1) {
		encoded.SchemaChanges[0].Kind = "future_unsealed_change"
	})
	if _, err := DecodeLoweredTypeEnvExtensionProposal(unknownVariant); err == nil || !strings.Contains(err.Error(), "unknown SchemaChange") {
		t.Fatalf("unknown-variant error = %v; want closed-union rejection", err)
	}

	payload, err := decodeExtensionCanonicalEnvelope(canonical)
	if err != nil {
		t.Fatalf("decodeExtensionCanonicalEnvelope() error = %v", err)
	}
	var withUnknown map[string]any
	if err := json.Unmarshal(payload, &withUnknown); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	withUnknown["unknown_future_field"] = true
	unknownPayload, err := json.Marshal(withUnknown)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	unknownField := wrapExtensionPayload(unknownPayload)
	if _, err := DecodeLoweredTypeEnvExtensionProposal(unknownField); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown-field error = %v; want strict schema rejection", err)
	}
}

func TestLoweredTypeEnvExtensionProposalRejectsInvalidUTF8BeforeHashing(t *testing.T) {
	fixture := newExtensionFixture(t)
	invalid := ExtensionID{value: string([]byte{0xff, 'x'})}

	_, err := NewLoweredTypeEnvExtensionProposalBuilder(invalid).
		SetSourceCarrier(fixture.carrier, fixture.edition, fixture.carrierHash).
		SetBaseTypeEnv(fixture.base).
		SetBoundedContext(fixture.context).
		SetSignatureManifest(fixture.manifest).
		SetSchemaChanges(fixture.changes).
		SetCompilerSchemaVersion(fixture.compiler).
		SetCompatibilityDiff(fixture.compatibility).
		SetRevalidationReport(fixture.revalidation).
		Build()
	if err == nil || !strings.Contains(err.Error(), "invalid UTF-8") {
		t.Fatalf("invalid-UTF8 error = %v; want pre-hash rejection", err)
	}
}

func TestDecodeLoweredTypeEnvExtensionProposalRejectsInvalidUTF8Payload(t *testing.T) {
	canonical := newExtensionFixture(t).build(t).CanonicalBytes()
	payload, err := decodeExtensionCanonicalEnvelope(canonical)
	if err != nil {
		t.Fatalf("decodeExtensionCanonicalEnvelope() error = %v", err)
	}
	marker := []byte("haft.typed-memory.v1")
	markerIndex := bytes.Index(payload, marker)
	if markerIndex < 0 {
		t.Fatalf("canonical payload does not contain marker %q", marker)
	}
	payload[markerIndex] = 0xff
	invalid := wrapExtensionPayload(payload)

	_, err = DecodeLoweredTypeEnvExtensionProposal(invalid)
	if err == nil || !strings.Contains(err.Error(), "invalid UTF-8") {
		t.Fatalf("decode invalid-UTF8 error = %v; want direct byte-path rejection", err)
	}
}

func TestBindValueKindSchemaChangeProvidesOnlyCodec(t *testing.T) {
	factory := newExhaustiveExtensionFactory(t)
	shape := typeEnvTestShapeRef(t, "Haft.TextShape", 0xc5)
	change := factory.valueBindingChange(shape)
	provided := change.ProvidedSymbols()

	if len(provided) != 1 {
		t.Fatalf("binding provides %d symbols; want codec only", len(provided))
	}
	if provided[0].Kind() != CodecSymbol {
		t.Fatalf("binding provided symbol kind = %q; want codec", provided[0].Kind().String())
	}
}

func TestLoweredTypeEnvExtensionProposalRoundTripsEverySupportedVariant(t *testing.T) {
	forward := buildExhaustiveLoweredExtension(t, false)
	reverse := buildExhaustiveLoweredExtension(t, true)

	if !bytes.Equal(forward.CanonicalBytes(), reverse.CanonicalBytes()) {
		t.Fatal("permuting unordered semantic inputs changed canonical bytes")
	}
	if forward.Ref() != reverse.Ref() {
		t.Fatal("permuting unordered semantic inputs changed content identity")
	}
	decoded, err := DecodeLoweredTypeEnvExtensionProposal(forward.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeLoweredTypeEnvExtensionProposal(exhaustive) error = %v", err)
	}
	if !reflect.DeepEqual(decoded, forward) {
		t.Fatal("exhaustive round trip lost a supported variant or nested field")
	}
	assertExhaustiveConstraintVariants(t, decoded)
	assertExhaustiveDefinitionVariants(t, decoded)
	assertExhaustiveContextBridgeContract(t, decoded)
	assertExactManifestClosure(t, decoded)

	encoded := rewriteExtensionPayload(t, forward.CanonicalBytes(), func(value *extensionCanonicalV1) {
		reverseSchemaChangeEnvelopes(value.SchemaChanges)
	})
	if _, err := DecodeLoweredTypeEnvExtensionProposal(encoded); err == nil || !strings.Contains(err.Error(), "not canonical") {
		t.Fatalf("noncanonical-order error = %v; want exact re-encode rejection", err)
	}
}

func TestContextBridgeCodecFailsClosedAndCommitsToFullContract(t *testing.T) {
	proposal := buildExhaustiveLoweredExtension(t, false)
	canonical := proposal.CanonicalBytes()

	missingCongruence := rewriteContextBridgeRawPayloadMap(t, canonical, func(payload map[string]any) {
		delete(payload, "kind_congruence")
	})
	if _, err := decodeContextBridgeChange(missingCongruence); err == nil ||
		!strings.Contains(err.Error(), "kind_congruence is required") {
		t.Fatalf("missing kind_congruence error = %v; want explicit presence rejection", err)
	}
	nullCongruence := rewriteContextBridgeRawPayloadMap(t, canonical, func(payload map[string]any) {
		payload["kind_congruence"] = nil
	})
	if _, err := decodeContextBridgeChange(nullCongruence); err == nil ||
		!strings.Contains(err.Error(), "kind_congruence is required") {
		t.Fatalf("null kind_congruence error = %v; want explicit presence rejection", err)
	}

	invalidMutations := []struct {
		name   string
		mutate func(*contextBridgeChangeV1)
		want   string
	}{
		{"moving source edition", func(value *contextBridgeChangeV1) {
			value.Source.Edition = "latest"
		}, "exact rather than moving"},
		{"unknown mapping", func(value *contextBridgeChangeV1) {
			value.Mapping.Kind = "signature_translation"
		}, "unknown context-bridge mapping kind"},
		{"unknown direction", func(value *contextBridgeChangeV1) {
			value.Direction = "reverse"
		}, "unknown context-bridge direction"},
		{"unsupported order claim", func(value *contextBridgeChangeV1) {
			value.OrderCoverage = "preserved"
		}, "unknown context-bridge order coverage"},
		{"CL outside ladder", func(value *contextBridgeChangeV1) {
			level := uint8(4)
			value.KindCongruence = &level
		}, "closed CL^k 0..3 ladder"},
		{"empty loss notes", func(value *contextBridgeChangeV1) {
			value.LossNotes = []string{}
		}, "at least one loss note"},
		{"empty definedness", func(value *contextBridgeChangeV1) {
			value.DefinednessArea = []string{}
		}, "at least one definedness condition"},
	}
	for _, test := range invalidMutations {
		t.Run(test.name, func(t *testing.T) {
			payload := rewriteContextBridgeRawPayload(t, canonical, test.mutate)
			if _, err := decodeContextBridgeChange(payload); err == nil ||
				!strings.Contains(err.Error(), test.want) {
				t.Fatalf("decode error = %v; want %q", err, test.want)
			}
		})
	}

	unknownTopLevel := rewriteContextBridgeRawPayloadMap(t, canonical, func(payload map[string]any) {
		payload["future_scope"] = "project-wide"
	})
	if _, err := decodeContextBridgeChange(unknownTopLevel); err == nil ||
		!strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown bridge field error = %v; want strict rejection", err)
	}
	unknownEndpoint := rewriteContextBridgeRawPayloadMap(t, canonical, func(payload map[string]any) {
		source := payload["source"].(map[string]any)
		source["floating"] = true
	})
	if _, err := decodeContextBridgeChange(unknownEndpoint); err == nil ||
		!strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown endpoint field error = %v; want strict rejection", err)
	}
	trailing := append(contextBridgeRawPayload(t, canonical), []byte(" {}")...)
	if _, err := decodeContextBridgeChange(trailing); err == nil ||
		!strings.Contains(err.Error(), "trailing") {
		t.Fatalf("nested trailing-value error = %v; want strict rejection", err)
	}

	noncanonicalSetOrder := rewriteContextBridgePayload(t, canonical, func(value *contextBridgeChangeV1) {
		value.LossNotes = []string{"Zulu loss.", "Alpha loss."}
		value.DefinednessArea = []string{"Zulu condition.", "Alpha condition."}
	})
	if _, err := DecodeLoweredTypeEnvExtensionProposal(noncanonicalSetOrder); err == nil ||
		!strings.Contains(err.Error(), "not canonical") {
		t.Fatalf("noncanonical bridge set order error = %v; want exact re-encode rejection", err)
	}

	validMutation := rewriteContextBridgePayload(t, canonical, func(value *contextBridgeChangeV1) {
		value.Direction = OneWayBridge.String()
	})
	mutated, err := DecodeLoweredTypeEnvExtensionProposal(validMutation)
	if err != nil {
		t.Fatalf("Decode valid bridge mutation: %v", err)
	}
	if mutated.Ref() == proposal.Ref() {
		t.Fatal("changing bridge direction did not change extension content identity")
	}
	if _, err := VerifyLoweredTypeEnvExtensionProposal(proposal.Ref(), validMutation); err == nil {
		t.Fatal("Verify accepted changed bridge semantics under the old extension reference")
	}
}

func assertExhaustiveContextBridgeContract(
	t *testing.T,
	proposal TypeEnvExtensionProposal,
) {
	t.Helper()
	var bridge ContextBridge
	found := false
	for _, change := range proposal.SchemaChanges().Changes() {
		add, ok := change.(AddContextBridgeSchemaChange)
		if !ok {
			continue
		}
		bridge = add.Bridge()
		found = true
		break
	}
	if !found {
		t.Fatal("exhaustive extension omitted ContextBridge")
	}
	if bridge.ID().String() != "Haft.ProjectDeliveryBridge" {
		t.Fatalf("ContextBridge ID = %q", bridge.ID().String())
	}
	if bridge.Source().Context().String() != "ctx:haft" ||
		bridge.Source().Edition().String() != "1.0.0-source" ||
		bridge.Target().Context().String() != "ctx:delivery" ||
		bridge.Target().Edition().String() != "1.0.0-target" {
		t.Fatalf("ContextBridge exact endpoints changed: %#v -> %#v", bridge.Source(), bridge.Target())
	}
	if bridge.Mapping().SourceKind().String() != "Haft.ProjectMemory" ||
		bridge.Mapping().TargetKind().String() != "Haft.SpecialProjectMemory" {
		t.Fatalf("ContextBridge named mapping changed: %#v", bridge.Mapping())
	}
	if bridge.Direction() != TwoWayBridge ||
		bridge.OrderCoverage() != NoOrderLinksCovered ||
		bridge.KindCongruence().Value() != 2 {
		t.Fatal("ContextBridge direction, order coverage, or CL^k changed")
	}
	if !reflect.DeepEqual(bridge.LossNotes().Values(), []string{
		"No source SubkindOf links are covered by this v1 bridge.",
	}) || !reflect.DeepEqual(bridge.DefinednessArea().Values(), []string{
		"The exact pinned context editions are active.",
	}) {
		t.Fatal("ContextBridge loss notes or definedness area changed")
	}
	provenance, ok := bridge.Provenance().(ProjectSourceProvenance)
	if !ok || provenance.SignatureBlockRow() != LawsRow ||
		provenance.ManifestBasis().Direction() != ManifestProvide {
		t.Fatalf("ContextBridge exact project provenance changed: %T", bridge.Provenance())
	}
}

func assertExhaustiveDefinitionVariants(
	t *testing.T,
	proposal TypeEnvExtensionProposal,
) {
	t.Helper()
	var entitySetObserved bool
	var kindSignatureObserved bool
	for _, change := range proposal.SchemaChanges().Changes() {
		switch value := change.(type) {
		case DefineEntitySetSchemaChange:
			entitySetObserved = true
			if value.ExportedSymbolID().String() != "Haft.ProjectMemoryEntitySet" {
				t.Fatalf("EntitySet exported symbol changed: %q", value.ExportedSymbolID().String())
			}
			if value.Definition().Ref().TypeEnv() != proposal.BaseTypeEnv() {
				t.Fatal("EntitySet exact TypeEnvRef changed during round trip")
			}
			if _, ok := value.Definition().CandidatePolicy().(PriorBatchDeclarationsVisible); !ok {
				t.Fatalf("EntitySet candidate policy = %T; want PriorBatchDeclarationsVisible", value.Definition().CandidatePolicy())
			}
		case DefineKindSignatureSchemaChange:
			kindSignatureObserved = true
			if value.ExportedSymbolID().String() != "Haft.ProjectMemoryKindSignature" {
				t.Fatalf("KindSignature exported symbol changed: %q", value.ExportedSymbolID().String())
			}
			if value.Definition().Ref().TypeEnv() != proposal.BaseTypeEnv() ||
				value.Definition().EntitySet().TypeEnv() != proposal.BaseTypeEnv() {
				t.Fatal("KindSignature exact TypeEnvRef changed during round trip")
			}
		}
	}
	if !entitySetObserved || !kindSignatureObserved {
		t.Fatalf(
			"exhaustive definition variants: EntitySet=%v KindSignature=%v; want both",
			entitySetObserved,
			kindSignatureObserved,
		)
	}
}

func TestLoweredTypeEnvExtensionProposalRejectsUnknownFieldsInNestedConstraintPayloads(t *testing.T) {
	canonical := buildExhaustiveLoweredExtension(t, false).CanonicalBytes()
	variants := []string{
		"slot_cardinality",
		"reference_slot_subset",
		"reference_slot_partition",
	}

	for _, variant := range variants {
		t.Run(variant, func(t *testing.T) {
			mutated := rewriteConstraintRulePayload(t, canonical, variant, func(payload map[string]any) {
				payload["unknown_future_field"] = true
			})
			_, err := DecodeLoweredTypeEnvExtensionProposal(mutated)
			if err == nil || !strings.Contains(err.Error(), "unknown field") {
				t.Fatalf("nested-payload error = %v; want strict unknown-field rejection", err)
			}
		})
	}
}

func TestExhaustiveLoweredTypeEnvExtensionRejectsIncompleteConstraintManifest(t *testing.T) {
	proposal := buildExhaustiveLoweredExtension(t, false)
	manifest := proposal.SignatureManifest()
	partitionID := typeEnvTestConstraintID(t, "Haft.ProjectMemoryReferencePartition")
	partitionSymbol := mustExtensionValue(ConstraintSymbolRef(partitionID))
	provides := manifest.Provides()
	filtered := make([]SchemaSymbolRef, 0, len(provides)-1)
	for _, symbol := range provides {
		if symbol != partitionSymbol {
			filtered = append(filtered, symbol)
		}
	}
	if len(filtered) != len(provides)-1 {
		t.Fatalf("partition constraint symbol occurs %d times; want exactly once", len(provides)-len(filtered))
	}
	incomplete := mustExtensionValue(NewSignatureManifest(manifest.Ref(), manifest.Imports(), filtered))

	_, err := NewLoweredTypeEnvExtensionProposalBuilder(proposal.Ref().ID()).
		SetSourceCarrier(proposal.SourceCarrier(), proposal.Edition(), proposal.SourceCarrierHash()).
		SetBaseTypeEnv(proposal.BaseTypeEnv()).
		SetBoundedContext(proposal.BoundedContext()).
		SetSignatureManifest(incomplete).
		SetSchemaChanges(proposal.SchemaChanges()).
		SetCompilerSchemaVersion(proposal.CompilerSchemaVersion()).
		SetCompatibilityDiff(proposal.CompatibilityDiff()).
		SetRevalidationReport(proposal.RevalidationReport()).
		Build()
	if err == nil || !strings.Contains(err.Error(), "SchemaChanges realize") {
		t.Fatalf("incomplete-manifest error = %v; want exact closure rejection", err)
	}
}

func assertExhaustiveConstraintVariants(
	t *testing.T,
	proposal TypeEnvExtensionProposal,
) {
	t.Helper()
	want := map[string]bool{
		"kind_disjoint":            false,
		"slot_group":               false,
		"slot_cardinality":         false,
		"reference_slot_subset":    false,
		"reference_slot_partition": false,
	}
	for _, change := range proposal.SchemaChanges().Changes() {
		constraint, ok := change.(AddConstraintSchemaChange)
		if !ok {
			continue
		}
		switch rule := constraint.Rule().(type) {
		case KindDisjointConstraint:
			want["kind_disjoint"] = true
		case SlotGroupConstraint:
			want["slot_group"] = true
		case SlotCardinalityConstraint:
			want["slot_cardinality"] = true
			if rule.Slot().String() != "Related" || !equalCardinality(rule.Cardinality(), NewUnboundedCardinality(1)) {
				t.Fatalf("decoded slot-cardinality coordinates changed: slot=%q", rule.Slot().String())
			}
		case ReferenceSlotSubsetConstraint:
			want["reference_slot_subset"] = true
			if rule.Subset().String() != "RelatedLeft" || rule.Superset().String() != "Related" {
				t.Fatalf(
					"decoded subset coordinates = %q subset of %q; want RelatedLeft subset of Related",
					rule.Subset().String(),
					rule.Superset().String(),
				)
			}
		case ReferenceSlotPartitionConstraint:
			want["reference_slot_partition"] = true
			parts := rule.Parts()
			if len(parts) != 2 || parts[0].String() != "RelatedLeft" || parts[1].String() != "RelatedRight" {
				t.Fatalf("decoded canonical partition parts = %v; want RelatedLeft, RelatedRight", parts)
			}
		default:
			t.Fatalf("exhaustive fixture contains unknown constraint variant %T", rule)
		}
	}
	for variant, observed := range want {
		if !observed {
			t.Fatalf("exhaustive round trip omitted %s", variant)
		}
	}
}

func assertExactManifestClosure(
	t *testing.T,
	proposal TypeEnvExtensionProposal,
) {
	t.Helper()
	realized := make([]SchemaSymbolRef, 0)
	for _, change := range proposal.SchemaChanges().Changes() {
		realized = append(realized, change.ProvidedSymbols()...)
	}
	canonical := mustExtensionValue(canonicalSchemaSymbols("test realized provide", realized))
	if !reflect.DeepEqual(proposal.SignatureManifest().Provides(), canonical) {
		t.Fatal("exhaustive manifest does not exactly equal the realized SchemaChange symbol closure")
	}
}

func rewriteConstraintRulePayload(
	t *testing.T,
	canonical []byte,
	variant string,
	rewrite func(map[string]any),
) []byte {
	t.Helper()
	found := false
	mutated := rewriteExtensionPayload(t, canonical, func(value *extensionCanonicalV1) {
		for index, change := range value.SchemaChanges {
			if change.Kind != "add_constraint" {
				continue
			}
			var envelope constraintChangeEnvelopeV1
			if err := decodeStrictJSON(change.Payload, &envelope); err != nil {
				t.Fatalf("decode add_constraint envelope: %v", err)
			}
			if envelope.Kind != variant {
				continue
			}
			var payload map[string]any
			if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
				t.Fatalf("decode %s payload: %v", variant, err)
			}
			rewrite(payload)
			envelope.Payload = mustExtensionValue(json.Marshal(payload))
			value.SchemaChanges[index].Payload = mustExtensionValue(json.Marshal(envelope))
			found = true
		}
	})
	if !found {
		t.Fatalf("exhaustive fixture has no %s constraint", variant)
	}
	return mutated
}

func contextBridgeRawPayload(t *testing.T, canonical []byte) []byte {
	t.Helper()
	payload, err := decodeExtensionCanonicalEnvelope(canonical)
	if err != nil {
		t.Fatalf("decode extension envelope: %v", err)
	}
	var encoded extensionCanonicalV1
	if err := decodeStrictJSON(payload, &encoded); err != nil {
		t.Fatalf("decode extension payload: %v", err)
	}
	for _, change := range encoded.SchemaChanges {
		if change.Kind == "add_context_bridge" {
			return append([]byte(nil), change.Payload...)
		}
	}
	t.Fatal("exhaustive fixture has no add_context_bridge change")
	return nil
}

func rewriteContextBridgeRawPayload(
	t *testing.T,
	canonical []byte,
	rewrite func(*contextBridgeChangeV1),
) []byte {
	t.Helper()
	payload := contextBridgeRawPayload(t, canonical)
	var encoded contextBridgeChangeV1
	if err := decodeStrictJSON(payload, &encoded); err != nil {
		t.Fatalf("decode ContextBridge payload: %v", err)
	}
	rewrite(&encoded)
	return mustExtensionValue(json.Marshal(encoded))
}

func rewriteContextBridgeRawPayloadMap(
	t *testing.T,
	canonical []byte,
	rewrite func(map[string]any),
) []byte {
	t.Helper()
	payload := contextBridgeRawPayload(t, canonical)
	var encoded map[string]any
	if err := json.Unmarshal(payload, &encoded); err != nil {
		t.Fatalf("decode ContextBridge payload map: %v", err)
	}
	rewrite(encoded)
	return mustExtensionValue(json.Marshal(encoded))
}

func rewriteContextBridgePayload(
	t *testing.T,
	canonical []byte,
	rewrite func(*contextBridgeChangeV1),
) []byte {
	t.Helper()
	found := false
	mutated := rewriteExtensionPayload(t, canonical, func(value *extensionCanonicalV1) {
		for index, change := range value.SchemaChanges {
			if change.Kind != "add_context_bridge" {
				continue
			}
			var payload contextBridgeChangeV1
			if err := decodeStrictJSON(change.Payload, &payload); err != nil {
				t.Fatalf("decode ContextBridge payload: %v", err)
			}
			rewrite(&payload)
			value.SchemaChanges[index].Payload = mustExtensionValue(json.Marshal(payload))
			found = true
		}
	})
	if !found {
		t.Fatal("exhaustive fixture has no add_context_bridge change")
	}
	return mutated
}

func rewriteExtensionPayload(
	t *testing.T,
	canonical []byte,
	rewrite func(*extensionCanonicalV1),
) []byte {
	t.Helper()
	payload, err := decodeExtensionCanonicalEnvelope(canonical)
	if err != nil {
		t.Fatalf("decodeExtensionCanonicalEnvelope() error = %v", err)
	}
	var encoded extensionCanonicalV1
	if err := decodeStrictJSON(payload, &encoded); err != nil {
		t.Fatalf("decodeStrictJSON() error = %v", err)
	}
	rewrite(&encoded)
	updated, err := json.Marshal(encoded)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return wrapExtensionPayload(updated)
}

func wrapExtensionPayload(payload []byte) []byte {
	writer := newCanonicalWriter(typeEnvExtensionProposalDomain)
	writer.addBytes(payload)
	return writer.bytes()
}

func reverseSchemaChangeEnvelopes(values []schemaChangeEnvelopeV1) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

type exhaustiveExtensionFactory struct {
	t            *testing.T
	base         TypeEnvRef
	context      BoundedContextRef
	otherContext BoundedContextRef
	carrier      CarrierRef
	edition      CarrierEdition
	carrierHash  SHA256Digest
	manifest     SignatureManifestRef
	imported     SchemaSymbolRef
}

func buildExhaustiveLoweredExtension(t *testing.T, reverse bool) TypeEnvExtensionProposal {
	t.Helper()
	factory := newExhaustiveExtensionFactory(t)
	changes := factory.allSchemaChanges(reverse)
	provides := make([]SchemaSymbolRef, 0)
	for _, change := range changes {
		provides = append(provides, change.ProvidedSymbols()...)
	}
	imports := []SchemaSymbolRef{factory.imported}
	if reverse {
		reverseSchemaChanges(changes)
		reverseSchemaSymbols(provides)
	}
	manifest := mustExtensionValue(NewSignatureManifest(factory.manifest, imports, provides))
	changeSet := mustExtensionValue(NewSchemaChangeSet(changes))
	canonicalProvides := mustExtensionValue(canonicalSchemaSymbols("fixture provide", provides))
	compatibilitySymbols := []SchemaSymbolRef{
		canonicalProvides[0],
		canonicalProvides[len(canonicalProvides)/2],
		canonicalProvides[len(canonicalProvides)-1],
	}
	compatibilityKinds := []CompatibilityChangeKind{
		CompatibilityAdded,
		CompatibilityChanged,
		CompatibilityRemoved,
	}
	compatibilityChanges := make([]CompatibilityChange, 0, len(compatibilitySymbols))
	for index, symbol := range compatibilitySymbols {
		change := mustExtensionValue(NewCompatibilityChange(
			symbol,
			compatibilityKinds[index],
			"compatibility rationale "+symbol.String(),
		))
		compatibilityChanges = append(compatibilityChanges, change)
	}
	if reverse {
		reverseCompatibilityChanges(compatibilityChanges)
	}
	compatibility := mustExtensionValue(NewTypeEnvCompatibilityDiff(factory.base, compatibilityChanges))
	assertions := []AssertionID{
		mustExtensionValue(NewAssertionID("assertion:alpha")),
		mustExtensionValue(NewAssertionID("assertion:omega")),
	}
	if reverse {
		assertions[0], assertions[1] = assertions[1], assertions[0]
	}
	revalidation := mustExtensionValue(NewExistingAssertionRevalidationReport(
		RevalidationConflict,
		NewGraphRevision(73),
		assertions,
		typeEnvTestDigest(t, 0xdc),
	))
	id := mustExtensionValue(NewExtensionID("haft.typed-memory.exhaustive.v1"))
	proposal, err := NewLoweredTypeEnvExtensionProposalBuilder(id).
		SetSourceCarrier(factory.carrier, factory.edition, factory.carrierHash).
		SetBaseTypeEnv(factory.base).
		SetBoundedContext(factory.context).
		SetSignatureManifest(manifest).
		SetSchemaChanges(changeSet).
		SetCompilerSchemaVersion(typeEnvTestCompilerVersion(t, "local-practice.exhaustive.v1")).
		SetCompatibilityDiff(compatibility).
		SetRevalidationReport(revalidation).
		Build()
	if err != nil {
		t.Fatalf("exhaustive extension Build() error = %v", err)
	}
	return proposal
}

func newExhaustiveExtensionFactory(t *testing.T) exhaustiveExtensionFactory {
	t.Helper()
	imported := mustExtensionValue(KindSymbolRef(typeEnvTestKindID(t, "FPF.ExternalImportedKind")))
	return exhaustiveExtensionFactory{
		t:            t,
		base:         typeEnvTestTypeEnvRef(t, 0xc0),
		context:      typeEnvTestContextRef(t, "ctx:haft"),
		otherContext: typeEnvTestContextRef(t, "ctx:delivery"),
		carrier:      typeEnvTestCarrierRef(t, "carrier:.haft/typed-memory-local-practice.md"),
		edition:      typeEnvTestCarrierEdition(t, "2026-07-16.exhaustive"),
		carrierHash:  typeEnvTestDigest(t, 0xc1),
		manifest:     mustExtensionValue(NewSignatureManifestRef("haft.local-practice.exhaustive", "v1")),
		imported:     imported,
	}
}

func (factory exhaustiveExtensionFactory) projectProvenance(
	symbol SchemaSymbolRef,
	row SignatureBlockRow,
	direction ManifestDirection,
) ProjectSourceProvenance {
	factory.t.Helper()
	basis := mustExtensionValue(NewManifestSymbolBasis(factory.manifest, direction, symbol))
	reference := typeEnvTestProvenanceRef(factory.t, "prov:project:"+symbol.String()+":"+row.String())
	return mustExtensionValue(NewProjectSourceProvenanceBuilder(
		reference,
		factory.carrier,
		factory.edition,
		factory.carrierHash,
	).
		SetDeclarationRange(typeEnvTestLineRange(factory.t, 40, 48)).
		SetCompilerRule(typeEnvTestCompilerRuleID(factory.t, "local-practice.exhaustive.v1")).
		SetBoundedContext(factory.context).
		SetBaseTypeEnv(factory.base).
		SetSignatureBlockRow(row).
		SetManifestBasis(basis).
		Build())
}

func (factory exhaustiveExtensionFactory) allSchemaChanges(reverse bool) []SchemaChange {
	factory.t.Helper()
	contextSymbol := mustExtensionValue(BoundedContextSymbolRef(factory.context))
	context := mustExtensionValue(NewBoundedContext(
		factory.context,
		factory.projectProvenance(contextSymbol, SubjectBlockRow, ManifestProvide),
	))
	addContext := mustExtensionValue(NewAddBoundedContextSchemaChange(context))

	kindID := typeEnvTestKindID(factory.t, "Haft.ProjectMemory")
	kindSymbol := mustExtensionValue(KindSymbolRef(kindID))
	kindDefinition := mustExtensionValue(NewKindDefinition(
		kindID,
		factory.projectProvenance(kindSymbol, VocabularyRow, ManifestProvide),
	))
	defineKind := mustExtensionValue(NewDefineKindSchemaChange(kindDefinition))

	refKindID := typeEnvTestRefKindID(factory.t, "Haft.ProjectMemoryRef")
	refKind := typeEnvTestRefKindRef(factory.t, factory.base, refKindID)
	refKindSymbol := mustExtensionValue(RefKindSymbolRef(refKindID))
	valueKind := typeEnvTestValueKindRef(factory.t, factory.base, kindID)
	refDefinition := mustExtensionValue(NewRefKindDefinition(
		refKind,
		valueKind,
		factory.projectProvenance(refKindSymbol, VocabularyRow, ManifestProvide),
	))
	defineRefKind := mustExtensionValue(NewDefineRefKindSchemaChange(refDefinition))

	subkindID := typeEnvTestKindID(factory.t, "Haft.SpecialProjectMemory")
	subkind := mustExtensionValue(NewSubkindRelation(
		subkindID,
		kindID,
		factory.projectProvenance(kindSymbol, LawsRow, ManifestProvide),
	))
	defineSubkind := mustExtensionValue(NewDefineSubkindSchemaChange(subkind))

	bridgeID := typeEnvTestBridgeID(factory.t, "Haft.ProjectDeliveryBridge")
	bridgeSymbol := mustExtensionValue(ContextBridgeSymbolRef(bridgeID))
	bridge := typeEnvTestContextBridge(
		factory.t,
		bridgeID,
		factory.context,
		factory.otherContext,
		kindID,
		subkindID,
		TwoWayBridge,
		factory.projectProvenance(bridgeSymbol, LawsRow, ManifestProvide),
	)
	addBridge := mustExtensionValue(NewAddContextBridgeSchemaChange(bridge))

	relationChange := factory.relationSignatureChange(kindID, refKind, reverse)
	shapeChanges, shapeRefs := factory.valueShapeChanges(reverse)
	bindingChange := factory.valueBindingChange(shapeRefs[0])
	constraintChanges := factory.constraintChanges(kindID, subkindID, reverse)
	membershipDefinitionChanges := factory.membershipDefinitionChanges(kindID, reverse)

	changes := []SchemaChange{
		addContext,
		defineKind,
		defineRefKind,
		defineSubkind,
		addBridge,
		relationChange,
	}
	changes = append(changes, shapeChanges...)
	changes = append(changes, bindingChange)
	changes = append(changes, constraintChanges...)
	changes = append(changes, membershipDefinitionChanges...)
	return changes
}

func (factory exhaustiveExtensionFactory) membershipDefinitionChanges(
	kindID KindID,
	reverse bool,
) []SchemaChange {
	factory.t.Helper()
	entitySetSymbolID := mustExtensionValue(NewEntitySetSymbolID("Haft.ProjectMemoryEntitySet"))
	entitySetSymbol := mustExtensionValue(EntitySetSymbolRef(entitySetSymbolID))
	policy := mustExtensionValue(NewPriorBatchDeclarationsVisible(
		typeEnvTestRuleRef(factory.t, "rule:entity-set:prospective"),
	))
	entitySet := mustExtensionValue(NewEntitySetDefinition(EntitySetDefinitionInput{
		TypeEnv:         factory.base,
		Context:         factory.context,
		EnumerationRule: typeEnvTestRuleRef(factory.t, "rule:entity-set:persisted"),
		CandidatePolicy: policy,
		Provenance:      factory.projectProvenance(entitySetSymbol, VocabularyRow, ManifestProvide),
	}))
	entitySetChange := mustExtensionValue(NewDefineEntitySetSchemaChange(
		entitySetSymbolID,
		entitySet,
	))

	kindSignatureSymbolID := mustExtensionValue(NewKindSignatureSymbolID(
		"Haft.ProjectMemoryKindSignature",
	))
	kindSignatureSymbol := mustExtensionValue(KindSignatureSymbolRef(kindSignatureSymbolID))
	assumptions := []KindAssumptionPin{
		typeEnvTestKindAssumption(factory.t, "carrier:assumption-alpha", "v1", 0xcd),
		typeEnvTestKindAssumption(factory.t, "carrier:assumption-omega", "v2", 0xce),
	}
	if reverse {
		assumptions[0], assumptions[1] = assumptions[1], assumptions[0]
	}
	kindSignature := mustExtensionValue(NewKindSignatureDefinition(KindSignatureDefinitionInput{
		ValueKind:       typeEnvTestValueKindRef(factory.t, factory.base, kindID),
		Formality:       SignatureF7,
		Assumptions:     assumptions,
		DefinednessRule: typeEnvTestRuleRef(factory.t, "rule:kind-signature:definedness"),
		Evaluator:       typeEnvTestRuleRef(factory.t, "rule:kind-signature:evaluator"),
		EntitySet:       entitySet.Ref(),
		Provenance:      factory.projectProvenance(kindSignatureSymbol, VocabularyRow, ManifestProvide),
	}))
	kindSignatureChange := mustExtensionValue(NewDefineKindSignatureSchemaChange(
		kindSignatureSymbolID,
		kindSignature,
	))
	return []SchemaChange{entitySetChange, kindSignatureChange}
}

func (factory exhaustiveExtensionFactory) relationSignatureChange(
	kindID KindID,
	refKind RefKindRef,
	reverse bool,
) SchemaChange {
	factory.t.Helper()
	signatureID := mustExtensionValue(NewSignatureID("Haft.ProjectMemoryRelation"))
	signatureRef := mustExtensionValue(NewRelationSignatureRef(factory.base, signatureID))
	relationSymbol := mustExtensionValue(RelationSymbolRef(signatureID))
	valueKind := typeEnvTestValueKindRef(factory.t, factory.base, kindID)
	valueTarget := mustExtensionValue(NewValueSlotTarget(valueKind))
	referenceTarget := mustExtensionValue(NewReferenceSlotTarget(valueKind, refKind))
	firstSlotID := typeEnvTestSlotKindID(factory.t, "Concern")
	secondSlotID := typeEnvTestSlotKindID(factory.t, "Related")
	thirdSlotID := typeEnvTestSlotKindID(factory.t, "Evidence")
	leftSlotID := typeEnvTestSlotKindID(factory.t, "RelatedLeft")
	rightSlotID := typeEnvTestSlotKindID(factory.t, "RelatedRight")
	thirdSlotSymbol := mustExtensionValue(SlotKindSymbolRef(signatureID, thirdSlotID))
	projectSlot := factory.projectProvenance(thirdSlotSymbol, VocabularyRow, ManifestProvide)
	fpfProvenance := typeEnvTestFPFProvenance(factory.t, "prov:fpf:lowered-slot", 0xc2)
	compilerProvenance := mustExtensionValue(NewCompilerDerivedProvenance(
		typeEnvTestProvenanceRef(factory.t, "prov:compiler:lowered-slot"),
		[]SourceLocation{
			typeEnvTestSourceLocation(factory.t, 0xc3),
			typeEnvTestSourceLocation(factory.t, 0xc4),
		},
		typeEnvTestCompilerRuleID(factory.t, "compiler.lowered-slot.v1"),
	))
	firstSlot := mustExtensionValue(NewSlotSpec(
		firstSlotID,
		valueTarget,
		mustExtensionValue(NewBoundedCardinality(0, 0)),
		fpfProvenance,
	))
	secondSlot := mustExtensionValue(NewSlotSpec(
		secondSlotID,
		referenceTarget,
		NewUnboundedCardinality(1),
		compilerProvenance,
	))
	thirdSlot := mustExtensionValue(NewSlotSpec(
		thirdSlotID,
		valueTarget,
		mustExtensionValue(NewBoundedCardinality(1, 2)),
		projectSlot,
	))
	leftSlot := mustExtensionValue(NewSlotSpec(
		leftSlotID,
		referenceTarget,
		NewUnboundedCardinality(0),
		compilerProvenance,
	))
	rightSlot := mustExtensionValue(NewSlotSpec(
		rightSlotID,
		referenceTarget,
		NewUnboundedCardinality(0),
		compilerProvenance,
	))
	contexts := []BoundedContextRef{factory.context, factory.otherContext}
	slots := []SlotSpec{firstSlot, secondSlot, thirdSlot, leftSlot, rightSlot}
	if reverse {
		contexts[0], contexts[1] = contexts[1], contexts[0]
		for left, right := 0, len(slots)-1; left < right; left, right = left+1, right-1 {
			slots[left], slots[right] = slots[right], slots[left]
		}
	}
	fragment := mustExtensionValue(NewTypedRelationDeclarationFragment(
		signatureRef,
		contexts,
		slots,
		factory.projectProvenance(relationSymbol, VocabularyRow, ManifestProvide),
	))
	return mustExtensionValue(
		NewDefineTypedRelationDeclarationFragmentSchemaChange(fragment),
	)
}

func (factory exhaustiveExtensionFactory) valueShapeChanges(
	reverse bool,
) ([]SchemaChange, []ValueShapeRef) {
	factory.t.Helper()
	scalar := mustExtensionValue(NewScalarShape(ScalarText))
	scalarRef := typeEnvTestDerivedShapeRef(factory.t, "Haft.TextShape", scalar)
	claimGraph := NewClaimGraphShape()
	claimGraphRef := typeEnvTestDerivedShapeRef(
		factory.t,
		"Haft.ClaimGraphShape",
		claimGraph,
	)
	recordFields := []RecordFieldShape{
		mustExtensionValue(NewRecordFieldShape(
			mustExtensionValue(NewValueMemberName("alpha")),
			scalarRef,
		)),
		mustExtensionValue(NewRecordFieldShape(
			mustExtensionValue(NewValueMemberName("omega")),
			claimGraphRef,
		)),
	}
	if reverse {
		recordFields[0], recordFields[1] = recordFields[1], recordFields[0]
	}
	record := mustExtensionValue(NewRecordShape(recordFields))
	recordRef := typeEnvTestDerivedShapeRef(factory.t, "Haft.RecordShape", record)
	sumVariants := []SumVariantShape{
		mustExtensionValue(NewSumVariantShape(
			mustExtensionValue(NewValueMemberName("left")),
			scalarRef,
		)),
		mustExtensionValue(NewSumVariantShape(
			mustExtensionValue(NewValueMemberName("right")),
			recordRef,
		)),
	}
	if reverse {
		sumVariants[0], sumVariants[1] = sumVariants[1], sumVariants[0]
	}
	sum := mustExtensionValue(NewSumShape(sumVariants))
	sumRef := typeEnvTestDerivedShapeRef(factory.t, "Haft.SumShape", sum)
	sequence := mustExtensionValue(NewOrderedSequenceShape(recordRef))
	sequenceRef := typeEnvTestDerivedShapeRef(
		factory.t,
		"Haft.SequenceShape",
		sequence,
	)
	set := mustExtensionValue(NewUnorderedSetShape(sumRef))
	setRef := typeEnvTestDerivedShapeRef(factory.t, "Haft.SetShape", set)
	shapes := []ValueShape{
		scalar,
		record,
		sum,
		sequence,
		set,
		claimGraph,
	}
	refs := []ValueShapeRef{
		scalarRef,
		recordRef,
		sumRef,
		sequenceRef,
		setRef,
		claimGraphRef,
	}
	changes := make([]SchemaChange, 0, len(shapes))
	for index, shape := range shapes {
		symbol := mustExtensionValue(ValueShapeSymbolRef(refs[index].ID()))
		declaration := mustExtensionValue(NewValueShapeDeclaration(
			refs[index],
			shape,
			factory.projectProvenance(symbol, VocabularyRow, ManifestProvide),
		))
		change := mustExtensionValue(NewDeclareValueShapeSchemaChange(declaration))
		changes = append(changes, change)
	}
	return changes, refs
}

func (factory exhaustiveExtensionFactory) valueBindingChange(shape ValueShapeRef) SchemaChange {
	factory.t.Helper()
	kindID := typeEnvTestKindID(factory.t, "Haft.StoredClaim")
	kind := typeEnvTestValueKindRef(factory.t, factory.base, kindID)
	codec := typeEnvTestCodecRef(factory.t, "Haft.StoredClaimCodec", 0xcb)
	codecSymbol := mustExtensionValue(CodecSymbolRef(codec.ID()))
	binding := mustExtensionValue(NewValueBinding(
		kind,
		shape,
		codec,
		factory.projectProvenance(codecSymbol, VocabularyRow, ManifestProvide),
	))
	return mustExtensionValue(NewBindValueKindSchemaChange(binding))
}

func (factory exhaustiveExtensionFactory) constraintChanges(
	kindID KindID,
	otherKindID KindID,
	reverse bool,
) []SchemaChange {
	factory.t.Helper()
	disjointID := typeEnvTestConstraintID(factory.t, "Haft.ProjectMemoryDisjoint")
	disjointSymbol := mustExtensionValue(ConstraintSymbolRef(disjointID))
	disjoint := mustExtensionValue(NewKindDisjointConstraint(
		disjointID,
		[]KindID{kindID, otherKindID},
		factory.projectProvenance(disjointSymbol, LawsRow, ManifestProvide),
	))
	disjointChange := mustExtensionValue(NewAddConstraintSchemaChange(disjoint))

	signature := typeEnvTestSignatureRef(factory.t, factory.base, "Haft.ProjectMemoryRelation")
	groupID := typeEnvTestConstraintID(factory.t, "Haft.ProjectMemorySlotGroup")
	groupSymbol := mustExtensionValue(ConstraintSymbolRef(groupID))
	group := mustExtensionValue(NewSlotGroupConstraint(
		groupID,
		signature,
		[]SlotKindID{
			typeEnvTestSlotKindID(factory.t, "Concern"),
			typeEnvTestSlotKindID(factory.t, "Related"),
		},
		SlotGroupExactlyOne,
		factory.projectProvenance(groupSymbol, LawsRow, ManifestProvide),
	))
	groupChange := mustExtensionValue(NewAddConstraintSchemaChange(group))

	cardinalityID := typeEnvTestConstraintID(factory.t, "Haft.ProjectMemoryRelatedCardinality")
	cardinalitySymbol := mustExtensionValue(ConstraintSymbolRef(cardinalityID))
	cardinality := mustExtensionValue(NewSlotCardinalityConstraint(
		cardinalityID,
		signature,
		typeEnvTestSlotKindID(factory.t, "Related"),
		NewUnboundedCardinality(1),
		factory.projectProvenance(cardinalitySymbol, LawsRow, ManifestProvide),
	))
	cardinalityChange := mustExtensionValue(NewAddConstraintSchemaChange(cardinality))

	subsetID := typeEnvTestConstraintID(factory.t, "Haft.ProjectMemoryReferenceSubset")
	subsetSymbol := mustExtensionValue(ConstraintSymbolRef(subsetID))
	subset := mustExtensionValue(NewReferenceSlotSubsetConstraint(
		subsetID,
		signature,
		typeEnvTestSlotKindID(factory.t, "RelatedLeft"),
		typeEnvTestSlotKindID(factory.t, "Related"),
		factory.projectProvenance(subsetSymbol, LawsRow, ManifestProvide),
	))
	subsetChange := mustExtensionValue(NewAddConstraintSchemaChange(subset))

	partitionID := typeEnvTestConstraintID(factory.t, "Haft.ProjectMemoryReferencePartition")
	partitionSymbol := mustExtensionValue(ConstraintSymbolRef(partitionID))
	parts := []SlotKindID{
		typeEnvTestSlotKindID(factory.t, "RelatedLeft"),
		typeEnvTestSlotKindID(factory.t, "RelatedRight"),
	}
	if reverse {
		parts[0], parts[1] = parts[1], parts[0]
	}
	partition := mustExtensionValue(NewReferenceSlotPartitionConstraint(
		partitionID,
		signature,
		typeEnvTestSlotKindID(factory.t, "Related"),
		parts,
		factory.projectProvenance(partitionSymbol, LawsRow, ManifestProvide),
	))
	partitionChange := mustExtensionValue(NewAddConstraintSchemaChange(partition))

	return []SchemaChange{
		disjointChange,
		groupChange,
		cardinalityChange,
		subsetChange,
		partitionChange,
	}
}

func mustExtensionValue[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

func reverseSchemaChanges(values []SchemaChange) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func reverseSchemaSymbols(values []SchemaSymbolRef) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func reverseCompatibilityChanges(values []CompatibilityChange) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}
