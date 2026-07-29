package typedmemory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"
)

const (
	typeEnvExtensionProposalDomain   = "haft.typedmemory.lowered-typeenv-extension-proposal.v1"
	maxTypeEnvExtensionCanonicalSize = 16 << 20
)

type extensionCanonicalV1 struct {
	ExtensionID       string                   `json:"extension_id"`
	Source            extensionSourceV1        `json:"source"`
	BaseTypeEnv       string                   `json:"base_type_env"`
	BoundedContext    string                   `json:"bounded_context"`
	SignatureManifest signatureManifestV1      `json:"signature_manifest"`
	SchemaChanges     []schemaChangeEnvelopeV1 `json:"schema_changes"`
	CompilerSchema    string                   `json:"compiler_schema_version"`
	Compatibility     compatibilityDiffV1      `json:"compatibility"`
	Revalidation      assertionRevalidationV1  `json:"revalidation"`
}

type extensionSourceV1 struct {
	Carrier string `json:"carrier"`
	Edition string `json:"edition"`
	Digest  string `json:"digest"`
}

type signatureManifestV1 struct {
	ID       string           `json:"id"`
	Version  string           `json:"version"`
	Imports  []schemaSymbolV1 `json:"imports"`
	Provides []schemaSymbolV1 `json:"provides"`
}

type schemaSymbolV1 struct {
	Kind string `json:"kind"`
	Key  string `json:"key"`
}

type schemaChangeEnvelopeV1 struct {
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

type declarationProvenanceEnvelopeV1 struct {
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

type fpfSourceProvenanceV1 struct {
	Reference    string           `json:"reference"`
	Location     sourceLocationV1 `json:"location"`
	CompilerRule string           `json:"compiler_rule"`
}

type compilerDerivedProvenanceV1 struct {
	Reference    string             `json:"reference"`
	Inputs       []sourceLocationV1 `json:"inputs"`
	CompilerRule string             `json:"compiler_rule"`
}

type projectSourceProvenanceV1 struct {
	Reference      string                `json:"reference"`
	Carrier        string                `json:"carrier"`
	Edition        string                `json:"edition"`
	ContentDigest  string                `json:"content_digest"`
	LineStart      uint64                `json:"line_start"`
	LineEnd        uint64                `json:"line_end"`
	CompilerRule   string                `json:"compiler_rule"`
	BoundedContext string                `json:"bounded_context"`
	BaseTypeEnv    string                `json:"base_type_env"`
	SignatureRow   string                `json:"signature_row"`
	ManifestBasis  manifestSymbolBasisV1 `json:"manifest_basis"`
}

type sourceLocationV1 struct {
	UnitID        string `json:"unit_id"`
	Revision      string `json:"revision"`
	ContentDigest string `json:"content_digest"`
	LineStart     uint64 `json:"line_start"`
	LineEnd       uint64 `json:"line_end"`
	PatternID     string `json:"pattern_id"`
}

type manifestSymbolBasisV1 struct {
	ManifestID      string         `json:"manifest_id"`
	ManifestVersion string         `json:"manifest_version"`
	Direction       string         `json:"direction"`
	Symbol          schemaSymbolV1 `json:"symbol"`
}

type typeEnvIDRefV1 struct {
	TypeEnv string `json:"type_env"`
	ID      string `json:"id"`
}

type valueShapeRefV1 struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

type codecRefV1 struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

type compatibilityDiffV1 struct {
	Base    string                  `json:"base"`
	Changes []compatibilityChangeV1 `json:"changes"`
}

type compatibilityChangeV1 struct {
	Symbol    schemaSymbolV1 `json:"symbol"`
	Kind      string         `json:"kind"`
	Rationale string         `json:"rationale"`
}

type assertionRevalidationV1 struct {
	Posture            string   `json:"posture"`
	GraphRevision      uint64   `json:"graph_revision"`
	AffectedAssertions []string `json:"affected_assertions"`
	ReportDigest       string   `json:"report_digest"`
}

type boundedContextChangeV1 struct {
	Reference  string                          `json:"reference"`
	Provenance declarationProvenanceEnvelopeV1 `json:"provenance"`
}

type kindDefinitionChangeV1 struct {
	KindID     string                          `json:"kind_id"`
	Provenance declarationProvenanceEnvelopeV1 `json:"provenance"`
}

type refKindDefinitionChangeV1 struct {
	Reference  typeEnvIDRefV1                  `json:"reference"`
	ValueKind  typeEnvIDRefV1                  `json:"value_kind"`
	Provenance declarationProvenanceEnvelopeV1 `json:"provenance"`
}

type subkindChangeV1 struct {
	Subkind    string                          `json:"subkind"`
	Superkind  string                          `json:"superkind"`
	Provenance declarationProvenanceEnvelopeV1 `json:"provenance"`
}

type contextBridgeChangeV1 struct {
	ID              string                          `json:"id"`
	Source          contextBridgeEndpointV1         `json:"source"`
	Target          contextBridgeEndpointV1         `json:"target"`
	Mapping         namedTargetKindMappingV1        `json:"mapping"`
	Direction       string                          `json:"direction"`
	OrderCoverage   string                          `json:"order_coverage"`
	KindCongruence  *uint8                          `json:"kind_congruence"`
	LossNotes       []string                        `json:"loss_notes"`
	DefinednessArea []string                        `json:"definedness_area"`
	Provenance      declarationProvenanceEnvelopeV1 `json:"provenance"`
}

type contextBridgeEndpointV1 struct {
	BoundedContext string `json:"bounded_context_ref"`
	Edition        string `json:"edition"`
}

type namedTargetKindMappingV1 struct {
	Kind       string `json:"kind"`
	SourceKind string `json:"source_kind"`
	TargetKind string `json:"target_kind"`
}

type relationSignatureChangeV1 struct {
	Reference  typeEnvIDRefV1                  `json:"reference"`
	Contexts   []string                        `json:"contexts"`
	Slots      []slotSpecV1                    `json:"slots"`
	Provenance declarationProvenanceEnvelopeV1 `json:"provenance"`
}

type slotSpecV1 struct {
	SlotKind    string                          `json:"slot_kind"`
	Target      slotTargetEnvelopeV1            `json:"target"`
	Cardinality cardinalityV1                   `json:"cardinality"`
	Provenance  declarationProvenanceEnvelopeV1 `json:"provenance"`
}

type slotTargetEnvelopeV1 struct {
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

type valueSlotTargetV1 struct {
	ValueKind typeEnvIDRefV1 `json:"value_kind"`
}

type referenceSlotTargetV1 struct {
	ValueKind     typeEnvIDRefV1 `json:"value_kind"`
	ReferenceKind typeEnvIDRefV1 `json:"reference_kind"`
}

type cardinalityV1 struct {
	Minimum     uint64 `json:"minimum"`
	MaximumKind string `json:"maximum_kind"`
	Maximum     uint64 `json:"maximum"`
}

type valueShapeDeclarationChangeV1 struct {
	Reference  valueShapeRefV1                 `json:"reference"`
	Shape      valueShapeEnvelopeV1            `json:"shape"`
	Provenance declarationProvenanceEnvelopeV1 `json:"provenance"`
}

type valueShapeEnvelopeV1 struct {
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

type scalarValueShapeV1 struct {
	ScalarKind string `json:"scalar_kind"`
}

type namedValueShapeRefV1 struct {
	Name  string          `json:"name"`
	Shape valueShapeRefV1 `json:"shape"`
}

type recordValueShapeV1 struct {
	Fields []namedValueShapeRefV1 `json:"fields"`
}

type sumValueShapeV1 struct {
	Variants []namedValueShapeRefV1 `json:"variants"`
}

type elementValueShapeV1 struct {
	Element valueShapeRefV1 `json:"element"`
}

type emptyValueShapeV1 struct{}

type valueBindingChangeV1 struct {
	ValueKind  typeEnvIDRefV1                  `json:"value_kind"`
	ValueShape valueShapeRefV1                 `json:"value_shape"`
	Codec      codecRefV1                      `json:"codec"`
	Provenance declarationProvenanceEnvelopeV1 `json:"provenance"`
}

type constraintChangeEnvelopeV1 struct {
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

type kindDisjointConstraintV1 struct {
	ID         string                          `json:"id"`
	Kinds      []string                        `json:"kinds"`
	Provenance declarationProvenanceEnvelopeV1 `json:"provenance"`
}

type slotGroupConstraintV1 struct {
	ID         string                          `json:"id"`
	Signature  typeEnvIDRefV1                  `json:"signature"`
	Slots      []string                        `json:"slots"`
	Mode       string                          `json:"mode"`
	Provenance declarationProvenanceEnvelopeV1 `json:"provenance"`
}

type slotCardinalityConstraintV1 struct {
	ID          string                          `json:"id"`
	Signature   typeEnvIDRefV1                  `json:"signature"`
	Slot        string                          `json:"slot"`
	Cardinality cardinalityV1                   `json:"cardinality"`
	Provenance  declarationProvenanceEnvelopeV1 `json:"provenance"`
}

type referenceSlotSubsetConstraintV1 struct {
	ID         string                          `json:"id"`
	Signature  typeEnvIDRefV1                  `json:"signature"`
	Subset     string                          `json:"subset"`
	Superset   string                          `json:"superset"`
	Provenance declarationProvenanceEnvelopeV1 `json:"provenance"`
}

type referenceSlotPartitionConstraintV1 struct {
	ID         string                          `json:"id"`
	Signature  typeEnvIDRefV1                  `json:"signature"`
	Whole      string                          `json:"whole"`
	Parts      []string                        `json:"parts"`
	Provenance declarationProvenanceEnvelopeV1 `json:"provenance"`
}

func encodeExtensionCandidate(
	candidate loweredTypeEnvExtensionCandidate,
) (extensionCanonicalV1, error) {
	changes := make([]schemaChangeEnvelopeV1, 0, len(candidate.changes.changes))
	for _, change := range candidate.changes.changes {
		encoded, err := encodeSchemaChange(change)
		if err != nil {
			return extensionCanonicalV1{}, err
		}
		changes = append(changes, encoded)
	}
	compatibility := make([]compatibilityChangeV1, 0, len(candidate.compatibility.changes))
	for _, change := range candidate.compatibility.changes {
		compatibility = append(compatibility, compatibilityChangeV1{
			Symbol:    encodeSchemaSymbol(change.symbol),
			Kind:      change.kind.String(),
			Rationale: change.rationale,
		})
	}
	affected := make([]string, 0, len(candidate.revalidation.affectedAssertions))
	for _, assertion := range candidate.revalidation.affectedAssertions {
		affected = append(affected, assertion.String())
	}
	return extensionCanonicalV1{
		ExtensionID: candidate.id.String(),
		Source: extensionSourceV1{
			Carrier: candidate.carrier.String(),
			Edition: candidate.edition.String(),
			Digest:  candidate.carrierHash.String(),
		},
		BaseTypeEnv:    candidate.baseTypeEnv.String(),
		BoundedContext: candidate.context.String(),
		SignatureManifest: signatureManifestV1{
			ID:       candidate.manifest.ref.ID(),
			Version:  candidate.manifest.ref.Version(),
			Imports:  encodeSchemaSymbols(candidate.manifest.imports),
			Provides: encodeSchemaSymbols(candidate.manifest.provides),
		},
		SchemaChanges:  changes,
		CompilerSchema: candidate.compiler.String(),
		Compatibility: compatibilityDiffV1{
			Base:    candidate.compatibility.base.String(),
			Changes: compatibility,
		},
		Revalidation: assertionRevalidationV1{
			Posture:            candidate.revalidation.posture.String(),
			GraphRevision:      candidate.revalidation.graphRevision.Value(),
			AffectedAssertions: affected,
			ReportDigest:       candidate.revalidation.reportDigest.String(),
		},
	}, nil
}

func decodeExtensionCandidate(encoded extensionCanonicalV1) (loweredTypeEnvExtensionCandidate, error) {
	id, err := NewExtensionID(encoded.ExtensionID)
	if err != nil {
		return loweredTypeEnvExtensionCandidate{}, fmt.Errorf("TypeEnv extension ID: %w", err)
	}
	carrier, err := NewCarrierRef(encoded.Source.Carrier)
	if err != nil {
		return loweredTypeEnvExtensionCandidate{}, fmt.Errorf("TypeEnv extension carrier: %w", err)
	}
	edition, err := NewCarrierEdition(encoded.Source.Edition)
	if err != nil {
		return loweredTypeEnvExtensionCandidate{}, fmt.Errorf("TypeEnv extension edition: %w", err)
	}
	carrierHash, err := NewSHA256Digest(encoded.Source.Digest)
	if err != nil {
		return loweredTypeEnvExtensionCandidate{}, fmt.Errorf("TypeEnv extension source digest: %w", err)
	}
	base, err := ParseTypeEnvRef(encoded.BaseTypeEnv)
	if err != nil {
		return loweredTypeEnvExtensionCandidate{}, fmt.Errorf("TypeEnv extension base: %w", err)
	}
	context, err := NewBoundedContextRef(encoded.BoundedContext)
	if err != nil {
		return loweredTypeEnvExtensionCandidate{}, fmt.Errorf("TypeEnv extension bounded context: %w", err)
	}
	manifest, err := decodeSignatureManifest(encoded.SignatureManifest)
	if err != nil {
		return loweredTypeEnvExtensionCandidate{}, err
	}
	changes, err := decodeSchemaChangeSet(encoded.SchemaChanges)
	if err != nil {
		return loweredTypeEnvExtensionCandidate{}, err
	}
	compiler, err := NewCompilerSchemaVersion(encoded.CompilerSchema)
	if err != nil {
		return loweredTypeEnvExtensionCandidate{}, fmt.Errorf("TypeEnv extension compiler schema: %w", err)
	}
	compatibility, err := decodeCompatibilityDiff(encoded.Compatibility)
	if err != nil {
		return loweredTypeEnvExtensionCandidate{}, err
	}
	revalidation, err := decodeAssertionRevalidation(encoded.Revalidation)
	if err != nil {
		return loweredTypeEnvExtensionCandidate{}, err
	}
	return loweredTypeEnvExtensionCandidate{
		id:            id,
		edition:       edition,
		baseTypeEnv:   base,
		context:       context,
		carrier:       carrier,
		carrierHash:   carrierHash,
		manifest:      manifest,
		changes:       changes,
		compiler:      compiler,
		compatibility: compatibility,
		revalidation:  revalidation,
	}, nil
}

func encodeSchemaSymbol(symbol SchemaSymbolRef) schemaSymbolV1 {
	return schemaSymbolV1{Kind: symbol.Kind().String(), Key: symbol.Key()}
}

func encodeSchemaSymbols(symbols []SchemaSymbolRef) []schemaSymbolV1 {
	result := make([]schemaSymbolV1, 0, len(symbols))
	for _, symbol := range symbols {
		result = append(result, encodeSchemaSymbol(symbol))
	}
	return result
}

func decodeSchemaSymbol(encoded schemaSymbolV1) (SchemaSymbolRef, error) {
	var ref SchemaSymbolRef
	var err error
	switch encoded.Kind {
	case "context":
		var context BoundedContextRef
		context, err = NewBoundedContextRef(encoded.Key)
		if err == nil {
			ref, err = BoundedContextSymbolRef(context)
		}
	case "kind":
		var kindID KindID
		kindID, err = NewKindID(encoded.Key)
		if err == nil {
			ref, err = KindSymbolRef(kindID)
		}
	case "slot_kind":
		ref, err = decodeSlotKindSymbol(encoded.Key)
	case "ref_kind":
		var refKindID RefKindID
		refKindID, err = NewRefKindID(encoded.Key)
		if err == nil {
			ref, err = RefKindSymbolRef(refKindID)
		}
	case "bridge":
		var bridgeID ContextBridgeID
		bridgeID, err = NewContextBridgeID(encoded.Key)
		if err == nil {
			ref, err = ContextBridgeSymbolRef(bridgeID)
		}
	case "signature":
		var signatureID SignatureID
		signatureID, err = NewSignatureID(encoded.Key)
		if err == nil {
			ref, err = RelationSymbolRef(signatureID)
		}
	case "shape":
		var shapeID ShapeID
		shapeID, err = NewShapeID(encoded.Key)
		if err == nil {
			ref, err = ValueShapeSymbolRef(shapeID)
		}
	case "codec":
		var codecID CodecID
		codecID, err = NewCodecID(encoded.Key)
		if err == nil {
			ref, err = CodecSymbolRef(codecID)
		}
	case "constraint":
		var constraintID ConstraintID
		constraintID, err = NewConstraintID(encoded.Key)
		if err == nil {
			ref, err = ConstraintSymbolRef(constraintID)
		}
	case "entity_set":
		var entitySetID EntitySetSymbolID
		entitySetID, err = NewEntitySetSymbolID(encoded.Key)
		if err == nil {
			ref, err = EntitySetSymbolRef(entitySetID)
		}
	case "kind_signature":
		var kindSignatureID KindSignatureSymbolID
		kindSignatureID, err = NewKindSignatureSymbolID(encoded.Key)
		if err == nil {
			ref, err = KindSignatureSymbolRef(kindSignatureID)
		}
	default:
		return SchemaSymbolRef{}, fmt.Errorf("unknown schema-symbol kind %q", encoded.Kind)
	}
	if err != nil {
		return SchemaSymbolRef{}, err
	}
	if ref.Kind().String() != encoded.Kind || ref.Key() != encoded.Key {
		return SchemaSymbolRef{}, fmt.Errorf("schema-symbol is not canonical")
	}
	return ref, nil
}

func decodeSlotKindSymbol(key string) (SchemaSymbolRef, error) {
	signatureRaw, slotRaw, found := strings.Cut(key, "/slot/")
	if !found || signatureRaw == "" || slotRaw == "" {
		return SchemaSymbolRef{}, fmt.Errorf("SlotKind symbol key is malformed")
	}
	signature, err := NewSignatureID(signatureRaw)
	if err != nil {
		return SchemaSymbolRef{}, err
	}
	slot, err := NewSlotKindID(slotRaw)
	if err != nil {
		return SchemaSymbolRef{}, err
	}
	ref, err := SlotKindSymbolRef(signature, slot)
	if err != nil {
		return SchemaSymbolRef{}, err
	}
	return ref, nil
}

func decodeSchemaSymbols(encoded []schemaSymbolV1) ([]SchemaSymbolRef, error) {
	result := make([]SchemaSymbolRef, 0, len(encoded))
	for index, symbol := range encoded {
		decoded, err := decodeSchemaSymbol(symbol)
		if err != nil {
			return nil, fmt.Errorf("schema symbol %d: %w", index, err)
		}
		result = append(result, decoded)
	}
	return result, nil
}

func decodeSignatureManifest(encoded signatureManifestV1) (SignatureManifest, error) {
	ref, err := NewSignatureManifestRef(encoded.ID, encoded.Version)
	if err != nil {
		return SignatureManifest{}, fmt.Errorf("signature manifest reference: %w", err)
	}
	imports, err := decodeSchemaSymbols(encoded.Imports)
	if err != nil {
		return SignatureManifest{}, fmt.Errorf("signature manifest imports: %w", err)
	}
	provides, err := decodeSchemaSymbols(encoded.Provides)
	if err != nil {
		return SignatureManifest{}, fmt.Errorf("signature manifest provides: %w", err)
	}
	manifest, err := NewSignatureManifest(ref, imports, provides)
	if err != nil {
		return SignatureManifest{}, err
	}
	return manifest, nil
}

func decodeCompatibilityDiff(encoded compatibilityDiffV1) (TypeEnvCompatibilityDiff, error) {
	base, err := ParseTypeEnvRef(encoded.Base)
	if err != nil {
		return TypeEnvCompatibilityDiff{}, fmt.Errorf("compatibility base: %w", err)
	}
	changes := make([]CompatibilityChange, 0, len(encoded.Changes))
	for index, encodedChange := range encoded.Changes {
		symbol, symbolErr := decodeSchemaSymbol(encodedChange.Symbol)
		if symbolErr != nil {
			return TypeEnvCompatibilityDiff{}, fmt.Errorf("compatibility change %d: %w", index, symbolErr)
		}
		kind, kindErr := parseCompatibilityChangeKind(encodedChange.Kind)
		if kindErr != nil {
			return TypeEnvCompatibilityDiff{}, fmt.Errorf("compatibility change %d: %w", index, kindErr)
		}
		change, changeErr := NewCompatibilityChange(symbol, kind, encodedChange.Rationale)
		if changeErr != nil {
			return TypeEnvCompatibilityDiff{}, fmt.Errorf("compatibility change %d: %w", index, changeErr)
		}
		changes = append(changes, change)
	}
	diff, err := NewTypeEnvCompatibilityDiff(base, changes)
	if err != nil {
		return TypeEnvCompatibilityDiff{}, err
	}
	return diff, nil
}

func parseCompatibilityChangeKind(raw string) (CompatibilityChangeKind, error) {
	for candidate := CompatibilityAdded; candidate <= CompatibilityRemoved; candidate++ {
		if candidate.String() == raw {
			return candidate, nil
		}
	}
	return 0, fmt.Errorf("unknown compatibility change kind %q", raw)
}

func decodeAssertionRevalidation(
	encoded assertionRevalidationV1,
) (ExistingAssertionRevalidationReport, error) {
	posture, err := parseRevalidationPosture(encoded.Posture)
	if err != nil {
		return ExistingAssertionRevalidationReport{}, err
	}
	assertions := make([]AssertionID, 0, len(encoded.AffectedAssertions))
	for index, raw := range encoded.AffectedAssertions {
		assertion, assertionErr := NewAssertionID(raw)
		if assertionErr != nil {
			return ExistingAssertionRevalidationReport{}, fmt.Errorf("revalidation assertion %d: %w", index, assertionErr)
		}
		assertions = append(assertions, assertion)
	}
	digest, err := NewSHA256Digest(encoded.ReportDigest)
	if err != nil {
		return ExistingAssertionRevalidationReport{}, fmt.Errorf("revalidation report digest: %w", err)
	}
	report, err := NewExistingAssertionRevalidationReport(
		posture,
		NewGraphRevision(encoded.GraphRevision),
		assertions,
		digest,
	)
	if err != nil {
		return ExistingAssertionRevalidationReport{}, err
	}
	return report, nil
}

func parseRevalidationPosture(raw string) (RevalidationPosture, error) {
	for candidate := RevalidationClean; candidate <= RevalidationUnderdetermined; candidate++ {
		if candidate.String() == raw {
			return candidate, nil
		}
	}
	return 0, fmt.Errorf("unknown revalidation posture %q", raw)
}

func encodeSchemaChange(change SchemaChange) (schemaChangeEnvelopeV1, error) {
	switch value := change.(type) {
	case AddBoundedContextSchemaChange:
		provenance, err := encodeDeclarationProvenance(value.context.provenance)
		if err != nil {
			return schemaChangeEnvelopeV1{}, err
		}
		payload := boundedContextChangeV1{
			Reference:  value.context.ref.String(),
			Provenance: provenance,
		}
		return newSchemaChangeEnvelope("add_bounded_context", payload)
	case DefineKindSchemaChange:
		provenance, err := encodeDeclarationProvenance(value.definition.provenance)
		if err != nil {
			return schemaChangeEnvelopeV1{}, err
		}
		payload := kindDefinitionChangeV1{
			KindID:     value.definition.id.String(),
			Provenance: provenance,
		}
		return newSchemaChangeEnvelope("define_kind", payload)
	case DefineRefKindSchemaChange:
		provenance, err := encodeDeclarationProvenance(value.definition.provenance)
		if err != nil {
			return schemaChangeEnvelopeV1{}, err
		}
		payload := refKindDefinitionChangeV1{
			Reference:  encodeRefKindRef(value.definition.ref),
			ValueKind:  encodeValueKindRef(value.definition.valueKind),
			Provenance: provenance,
		}
		return newSchemaChangeEnvelope("define_ref_kind", payload)
	case DefineSubkindSchemaChange:
		provenance, err := encodeDeclarationProvenance(value.relation.provenance)
		if err != nil {
			return schemaChangeEnvelopeV1{}, err
		}
		payload := subkindChangeV1{
			Subkind:    value.relation.subkind.String(),
			Superkind:  value.relation.superkind.String(),
			Provenance: provenance,
		}
		return newSchemaChangeEnvelope("define_subkind", payload)
	case AddContextBridgeSchemaChange:
		provenance, err := encodeDeclarationProvenance(value.bridge.provenance)
		if err != nil {
			return schemaChangeEnvelopeV1{}, err
		}
		kindCongruence := value.bridge.kindCongruence.Value()
		payload := contextBridgeChangeV1{
			ID: value.bridge.id.String(),
			Source: contextBridgeEndpointV1{
				BoundedContext: value.bridge.source.Context().String(),
				Edition:        value.bridge.source.Edition().String(),
			},
			Target: contextBridgeEndpointV1{
				BoundedContext: value.bridge.target.Context().String(),
				Edition:        value.bridge.target.Edition().String(),
			},
			Mapping: namedTargetKindMappingV1{
				Kind:       "named_target",
				SourceKind: value.bridge.mapping.SourceKind().String(),
				TargetKind: value.bridge.mapping.TargetKind().String(),
			},
			Direction:       value.bridge.direction.String(),
			OrderCoverage:   value.bridge.orderCoverage.String(),
			KindCongruence:  &kindCongruence,
			LossNotes:       value.bridge.lossNotes.Values(),
			DefinednessArea: value.bridge.definednessArea.Values(),
			Provenance:      provenance,
		}
		return newSchemaChangeEnvelope("add_context_bridge", payload)
	case DefineTypedRelationDeclarationFragmentSchemaChange:
		payload, err := encodeRelationSignatureChange(value.fragment)
		if err != nil {
			return schemaChangeEnvelopeV1{}, err
		}
		return newSchemaChangeEnvelope("define_relation_signature", payload)
	case DeclareValueShapeSchemaChange:
		payload, err := encodeValueShapeDeclarationChange(value.declaration)
		if err != nil {
			return schemaChangeEnvelopeV1{}, err
		}
		return newSchemaChangeEnvelope("declare_value_shape", payload)
	case BindValueKindSchemaChange:
		provenance, err := encodeDeclarationProvenance(value.binding.provenance)
		if err != nil {
			return schemaChangeEnvelopeV1{}, err
		}
		payload := valueBindingChangeV1{
			ValueKind:  encodeValueKindRef(value.binding.valueKind),
			ValueShape: encodeValueShapeRef(value.binding.valueShape),
			Codec:      encodeCodecRef(value.binding.codec),
			Provenance: provenance,
		}
		return newSchemaChangeEnvelope("bind_value_kind", payload)
	case AddConstraintSchemaChange:
		payload, err := encodeConstraintChange(value.rule)
		if err != nil {
			return schemaChangeEnvelopeV1{}, err
		}
		return newSchemaChangeEnvelope("add_constraint", payload)
	case DefineEntitySetSchemaChange:
		payload, err := encodeEntitySetDefinitionChange(value)
		if err != nil {
			return schemaChangeEnvelopeV1{}, err
		}
		return newSchemaChangeEnvelope("define_entity_set", payload)
	case DefineKindSignatureSchemaChange:
		payload, err := encodeKindSignatureDefinitionChange(value)
		if err != nil {
			return schemaChangeEnvelopeV1{}, err
		}
		return newSchemaChangeEnvelope("define_kind_signature", payload)
	default:
		return schemaChangeEnvelopeV1{}, fmt.Errorf("unknown SchemaChange variant %T", change)
	}
}

func newSchemaChangeEnvelope(kind string, payload any) (schemaChangeEnvelopeV1, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return schemaChangeEnvelopeV1{}, fmt.Errorf("encode %s SchemaChange: %w", kind, err)
	}
	return schemaChangeEnvelopeV1{Kind: kind, Payload: encoded}, nil
}

func decodeSchemaChangeSet(encoded []schemaChangeEnvelopeV1) (SchemaChangeSet, error) {
	changes := make([]SchemaChange, 0, len(encoded))
	for index, envelope := range encoded {
		change, err := decodeSchemaChange(envelope)
		if err != nil {
			return SchemaChangeSet{}, fmt.Errorf("schema change %d: %w", index, err)
		}
		changes = append(changes, change)
	}
	set, err := NewSchemaChangeSet(changes)
	if err != nil {
		return SchemaChangeSet{}, err
	}
	return set, nil
}

func decodeSchemaChange(envelope schemaChangeEnvelopeV1) (SchemaChange, error) {
	switch envelope.Kind {
	case "add_bounded_context":
		return decodeBoundedContextChange(envelope.Payload)
	case "define_kind":
		return decodeKindDefinitionChange(envelope.Payload)
	case "define_ref_kind":
		return decodeRefKindDefinitionChange(envelope.Payload)
	case "define_subkind":
		return decodeSubkindChange(envelope.Payload)
	case "add_context_bridge":
		return decodeContextBridgeChange(envelope.Payload)
	case "define_relation_signature":
		return decodeRelationSignatureChange(envelope.Payload)
	case "declare_value_shape":
		return decodeValueShapeDeclarationChange(envelope.Payload)
	case "bind_value_kind":
		return decodeValueBindingChange(envelope.Payload)
	case "add_constraint":
		return decodeConstraintSchemaChange(envelope.Payload)
	case "define_entity_set":
		return decodeEntitySetDefinitionChange(envelope.Payload)
	case "define_kind_signature":
		return decodeKindSignatureDefinitionChange(envelope.Payload)
	default:
		return nil, fmt.Errorf("unknown SchemaChange variant %q", envelope.Kind)
	}
}

func decodeBoundedContextChange(payload []byte) (SchemaChange, error) {
	encoded, err := decodeVariantPayload[boundedContextChangeV1](payload, "add_bounded_context")
	if err != nil {
		return nil, err
	}
	ref, err := NewBoundedContextRef(encoded.Reference)
	if err != nil {
		return nil, err
	}
	provenance, err := decodeDeclarationProvenance(encoded.Provenance)
	if err != nil {
		return nil, err
	}
	context, err := NewBoundedContext(ref, provenance)
	if err != nil {
		return nil, err
	}
	change, err := NewAddBoundedContextSchemaChange(context)
	if err != nil {
		return nil, err
	}
	return change, nil
}

func decodeKindDefinitionChange(payload []byte) (SchemaChange, error) {
	encoded, err := decodeVariantPayload[kindDefinitionChangeV1](payload, "define_kind")
	if err != nil {
		return nil, err
	}
	kindID, err := NewKindID(encoded.KindID)
	if err != nil {
		return nil, err
	}
	provenance, err := decodeDeclarationProvenance(encoded.Provenance)
	if err != nil {
		return nil, err
	}
	definition, err := NewKindDefinition(kindID, provenance)
	if err != nil {
		return nil, err
	}
	change, err := NewDefineKindSchemaChange(definition)
	if err != nil {
		return nil, err
	}
	return change, nil
}

func decodeRefKindDefinitionChange(payload []byte) (SchemaChange, error) {
	encoded, err := decodeVariantPayload[refKindDefinitionChangeV1](payload, "define_ref_kind")
	if err != nil {
		return nil, err
	}
	ref, err := decodeRefKindRef(encoded.Reference)
	if err != nil {
		return nil, err
	}
	valueKind, err := decodeValueKindRef(encoded.ValueKind)
	if err != nil {
		return nil, err
	}
	provenance, err := decodeDeclarationProvenance(encoded.Provenance)
	if err != nil {
		return nil, err
	}
	definition, err := NewRefKindDefinition(ref, valueKind, provenance)
	if err != nil {
		return nil, err
	}
	change, err := NewDefineRefKindSchemaChange(definition)
	if err != nil {
		return nil, err
	}
	return change, nil
}

func decodeSubkindChange(payload []byte) (SchemaChange, error) {
	encoded, err := decodeVariantPayload[subkindChangeV1](payload, "define_subkind")
	if err != nil {
		return nil, err
	}
	subkind, err := NewKindID(encoded.Subkind)
	if err != nil {
		return nil, err
	}
	superkind, err := NewKindID(encoded.Superkind)
	if err != nil {
		return nil, err
	}
	provenance, err := decodeDeclarationProvenance(encoded.Provenance)
	if err != nil {
		return nil, err
	}
	relation, err := NewSubkindRelation(subkind, superkind, provenance)
	if err != nil {
		return nil, err
	}
	change, err := NewDefineSubkindSchemaChange(relation)
	if err != nil {
		return nil, err
	}
	return change, nil
}

func decodeContextBridgeChange(payload []byte) (SchemaChange, error) {
	encoded, err := decodeVariantPayload[contextBridgeChangeV1](payload, "add_context_bridge")
	if err != nil {
		return nil, err
	}
	id, err := NewContextBridgeID(encoded.ID)
	if err != nil {
		return nil, err
	}
	source, err := decodeContextBridgeEndpoint(encoded.Source, "source")
	if err != nil {
		return nil, err
	}
	target, err := decodeContextBridgeEndpoint(encoded.Target, "target")
	if err != nil {
		return nil, err
	}
	if encoded.Mapping.Kind != "named_target" {
		return nil, fmt.Errorf(
			"unknown context-bridge mapping kind %q",
			encoded.Mapping.Kind,
		)
	}
	sourceKind, err := NewKindID(encoded.Mapping.SourceKind)
	if err != nil {
		return nil, err
	}
	targetKind, err := NewKindID(encoded.Mapping.TargetKind)
	if err != nil {
		return nil, err
	}
	mapping, err := NewNamedTargetKindMapping(sourceKind, targetKind)
	if err != nil {
		return nil, err
	}
	direction, err := parseBridgeDirection(encoded.Direction)
	if err != nil {
		return nil, err
	}
	orderCoverage, err := parseKindBridgeOrderCoverage(encoded.OrderCoverage)
	if err != nil {
		return nil, err
	}
	if encoded.KindCongruence == nil {
		return nil, fmt.Errorf("context-bridge kind_congruence is required")
	}
	kindCongruence, err := NewKindCongruenceLevel(*encoded.KindCongruence)
	if err != nil {
		return nil, err
	}
	lossNotes, err := NewKindBridgeLossNotes(encoded.LossNotes)
	if err != nil {
		return nil, err
	}
	definednessArea, err := NewKindBridgeDefinednessArea(encoded.DefinednessArea)
	if err != nil {
		return nil, err
	}
	provenance, err := decodeDeclarationProvenance(encoded.Provenance)
	if err != nil {
		return nil, err
	}
	bridge, err := NewContextBridge(ContextBridgeInput{
		ID:              id,
		Source:          source,
		Target:          target,
		Mapping:         mapping,
		Direction:       direction,
		OrderCoverage:   orderCoverage,
		KindCongruence:  kindCongruence,
		LossNotes:       lossNotes,
		DefinednessArea: definednessArea,
		Provenance:      provenance,
	})
	if err != nil {
		return nil, err
	}
	change, err := NewAddContextBridgeSchemaChange(bridge)
	if err != nil {
		return nil, err
	}
	return change, nil
}

func decodeContextBridgeEndpoint(
	encoded contextBridgeEndpointV1,
	label string,
) (ContextBridgeEndpoint, error) {
	context, err := NewBoundedContextRef(encoded.BoundedContext)
	if err != nil {
		return ContextBridgeEndpoint{}, fmt.Errorf(
			"decode context-bridge %s context: %w",
			label,
			err,
		)
	}
	edition, err := NewContextEdition(encoded.Edition)
	if err != nil {
		return ContextBridgeEndpoint{}, fmt.Errorf(
			"decode context-bridge %s edition: %w",
			label,
			err,
		)
	}
	endpoint, err := NewContextBridgeEndpoint(context, edition)
	if err != nil {
		return ContextBridgeEndpoint{}, err
	}
	return endpoint, nil
}

func parseBridgeDirection(raw string) (BridgeDirection, error) {
	for candidate := OneWayBridge; candidate <= TwoWayBridge; candidate++ {
		if candidate.String() == raw {
			return candidate, nil
		}
	}
	return 0, fmt.Errorf("unknown context-bridge direction %q", raw)
}

func decodeVariantPayload[T any](payload []byte, kind string) (T, error) {
	var result T
	if err := decodeStrictJSON(payload, &result); err != nil {
		return result, fmt.Errorf("%s payload: %w", kind, err)
	}
	return result, nil
}

func encodeRelationSignatureChange(
	signature RelationSignature,
) (relationSignatureChangeV1, error) {
	contexts := make([]string, 0, len(signature.contexts))
	for _, context := range signature.contexts {
		contexts = append(contexts, context.String())
	}
	slots := make([]slotSpecV1, 0, len(signature.slots))
	for _, slot := range signature.slots {
		encoded, err := encodeSlotSpec(slot)
		if err != nil {
			return relationSignatureChangeV1{}, err
		}
		slots = append(slots, encoded)
	}
	provenance, err := encodeDeclarationProvenance(signature.provenance)
	if err != nil {
		return relationSignatureChangeV1{}, err
	}
	return relationSignatureChangeV1{
		Reference:  encodeRelationSignatureRef(signature.ref),
		Contexts:   contexts,
		Slots:      slots,
		Provenance: provenance,
	}, nil
}

func encodeSlotSpec(slot SlotSpec) (slotSpecV1, error) {
	target, err := encodeSlotTarget(slot.target)
	if err != nil {
		return slotSpecV1{}, err
	}
	provenance, err := encodeDeclarationProvenance(slot.provenance)
	if err != nil {
		return slotSpecV1{}, err
	}
	return slotSpecV1{
		SlotKind:    slot.slotKind.String(),
		Target:      target,
		Cardinality: encodeCardinality(slot.cardinality),
		Provenance:  provenance,
	}, nil
}

func encodeCardinality(cardinality Cardinality) cardinalityV1 {
	maximum, bounded := cardinality.maximum.BoundedValue()
	maximumKind := "finite"
	if !bounded {
		maximumKind = "unbounded"
		maximum = 0
	}
	return cardinalityV1{
		Minimum:     cardinality.minimum,
		MaximumKind: maximumKind,
		Maximum:     maximum,
	}
}

func encodeSlotTarget(target SlotTarget) (slotTargetEnvelopeV1, error) {
	switch value := target.(type) {
	case ValueSlotTarget:
		payload := valueSlotTargetV1{ValueKind: encodeValueKindRef(value.kind)}
		return newSlotTargetEnvelope("value", payload)
	case ReferenceSlotTarget:
		payload := referenceSlotTargetV1{
			ValueKind:     encodeValueKindRef(value.valueKind),
			ReferenceKind: encodeRefKindRef(value.referenceKind),
		}
		return newSlotTargetEnvelope("reference", payload)
	default:
		return slotTargetEnvelopeV1{}, fmt.Errorf("unknown SlotTarget variant %T", target)
	}
}

func newSlotTargetEnvelope(kind string, payload any) (slotTargetEnvelopeV1, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return slotTargetEnvelopeV1{}, fmt.Errorf("encode %s slot target: %w", kind, err)
	}
	return slotTargetEnvelopeV1{Kind: kind, Payload: encoded}, nil
}

func decodeRelationSignatureChange(payload []byte) (SchemaChange, error) {
	encoded, err := decodeVariantPayload[relationSignatureChangeV1](payload, "define_relation_signature")
	if err != nil {
		return nil, err
	}
	ref, err := decodeRelationSignatureRef(encoded.Reference)
	if err != nil {
		return nil, err
	}
	contexts := make([]BoundedContextRef, 0, len(encoded.Contexts))
	for index, raw := range encoded.Contexts {
		context, contextErr := NewBoundedContextRef(raw)
		if contextErr != nil {
			return nil, fmt.Errorf("relation context %d: %w", index, contextErr)
		}
		contexts = append(contexts, context)
	}
	slots := make([]SlotSpec, 0, len(encoded.Slots))
	for index, encodedSlot := range encoded.Slots {
		slot, slotErr := decodeSlotSpec(encodedSlot)
		if slotErr != nil {
			return nil, fmt.Errorf("relation slot %d: %w", index, slotErr)
		}
		slots = append(slots, slot)
	}
	provenance, err := decodeDeclarationProvenance(encoded.Provenance)
	if err != nil {
		return nil, err
	}
	signature, err := NewRelationSignature(ref, contexts, slots, provenance)
	if err != nil {
		return nil, err
	}
	change, err := NewDefineRelationSignatureSchemaChange(signature)
	if err != nil {
		return nil, err
	}
	return change, nil
}

func decodeSlotSpec(encoded slotSpecV1) (SlotSpec, error) {
	slotKind, err := NewSlotKindID(encoded.SlotKind)
	if err != nil {
		return SlotSpec{}, err
	}
	target, err := decodeSlotTarget(encoded.Target)
	if err != nil {
		return SlotSpec{}, err
	}
	cardinality, err := decodeCardinality(encoded.Cardinality)
	if err != nil {
		return SlotSpec{}, err
	}
	provenance, err := decodeDeclarationProvenance(encoded.Provenance)
	if err != nil {
		return SlotSpec{}, err
	}
	slot, err := NewSlotSpec(slotKind, target, cardinality, provenance)
	if err != nil {
		return SlotSpec{}, err
	}
	return slot, nil
}

func decodeSlotTarget(encoded slotTargetEnvelopeV1) (SlotTarget, error) {
	switch encoded.Kind {
	case "value":
		payload, err := decodeVariantPayload[valueSlotTargetV1](encoded.Payload, "value slot target")
		if err != nil {
			return nil, err
		}
		kind, err := decodeValueKindRef(payload.ValueKind)
		if err != nil {
			return nil, err
		}
		target, err := NewValueSlotTarget(kind)
		if err != nil {
			return nil, err
		}
		return target, nil
	case "reference":
		payload, err := decodeVariantPayload[referenceSlotTargetV1](encoded.Payload, "reference slot target")
		if err != nil {
			return nil, err
		}
		valueKind, err := decodeValueKindRef(payload.ValueKind)
		if err != nil {
			return nil, err
		}
		refKind, err := decodeRefKindRef(payload.ReferenceKind)
		if err != nil {
			return nil, err
		}
		target, err := NewReferenceSlotTarget(valueKind, refKind)
		if err != nil {
			return nil, err
		}
		return target, nil
	default:
		return nil, fmt.Errorf("unknown SlotTarget variant %q", encoded.Kind)
	}
}

func decodeCardinality(encoded cardinalityV1) (Cardinality, error) {
	switch encoded.MaximumKind {
	case "finite":
		return NewBoundedCardinality(encoded.Minimum, encoded.Maximum)
	case "unbounded":
		if encoded.Maximum != 0 {
			return Cardinality{}, fmt.Errorf("unbounded cardinality cannot carry a finite maximum")
		}
		return NewUnboundedCardinality(encoded.Minimum), nil
	default:
		return Cardinality{}, fmt.Errorf("unknown cardinality maximum kind %q", encoded.MaximumKind)
	}
}

func encodeValueShapeDeclarationChange(
	declaration ValueShapeDeclaration,
) (valueShapeDeclarationChangeV1, error) {
	shape, err := encodeValueShape(declaration.shape)
	if err != nil {
		return valueShapeDeclarationChangeV1{}, err
	}
	provenance, err := encodeDeclarationProvenance(declaration.provenance)
	if err != nil {
		return valueShapeDeclarationChangeV1{}, err
	}
	return valueShapeDeclarationChangeV1{
		Reference:  encodeValueShapeRef(declaration.ref),
		Shape:      shape,
		Provenance: provenance,
	}, nil
}

func encodeValueShape(shape ValueShape) (valueShapeEnvelopeV1, error) {
	switch value := shape.(type) {
	case ScalarValueShape:
		payload := scalarValueShapeV1{ScalarKind: string(value.ScalarKind())}
		return newValueShapeEnvelope(string(ValueShapeScalar), payload)
	case RecordValueShape:
		fields := make([]namedValueShapeRefV1, 0, len(value.Fields()))
		for _, field := range value.Fields() {
			fields = append(fields, namedValueShapeRefV1{
				Name:  field.Name().String(),
				Shape: encodeValueShapeRef(field.Shape()),
			})
		}
		return newValueShapeEnvelope(string(ValueShapeRecord), recordValueShapeV1{Fields: fields})
	case SumValueShape:
		variants := make([]namedValueShapeRefV1, 0, len(value.Variants()))
		for _, variant := range value.Variants() {
			variants = append(variants, namedValueShapeRefV1{
				Name:  variant.Name().String(),
				Shape: encodeValueShapeRef(variant.Shape()),
			})
		}
		return newValueShapeEnvelope(string(ValueShapeSum), sumValueShapeV1{Variants: variants})
	case OrderedSequenceValueShape:
		payload := elementValueShapeV1{Element: encodeValueShapeRef(value.ElementShape())}
		return newValueShapeEnvelope(string(ValueShapeOrderedSequence), payload)
	case UnorderedSetValueShape:
		payload := elementValueShapeV1{Element: encodeValueShapeRef(value.ElementShape())}
		return newValueShapeEnvelope(string(ValueShapeUnorderedSet), payload)
	case ClaimGraphValueShape:
		return newValueShapeEnvelope(string(ValueShapeClaimGraph), emptyValueShapeV1{})
	default:
		return valueShapeEnvelopeV1{}, fmt.Errorf("unknown ValueShape variant %T", shape)
	}
}

func newValueShapeEnvelope(kind string, payload any) (valueShapeEnvelopeV1, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return valueShapeEnvelopeV1{}, fmt.Errorf("encode %s ValueShape: %w", kind, err)
	}
	return valueShapeEnvelopeV1{Kind: kind, Payload: encoded}, nil
}

func decodeValueShapeDeclarationChange(payload []byte) (SchemaChange, error) {
	encoded, err := decodeVariantPayload[valueShapeDeclarationChangeV1](payload, "declare_value_shape")
	if err != nil {
		return nil, err
	}
	ref, err := decodeValueShapeRef(encoded.Reference)
	if err != nil {
		return nil, err
	}
	shape, err := decodeValueShape(encoded.Shape)
	if err != nil {
		return nil, err
	}
	provenance, err := decodeDeclarationProvenance(encoded.Provenance)
	if err != nil {
		return nil, err
	}
	declaration, err := NewValueShapeDeclaration(ref, shape, provenance)
	if err != nil {
		return nil, err
	}
	change, err := NewDeclareValueShapeSchemaChange(declaration)
	if err != nil {
		return nil, err
	}
	return change, nil
}

func decodeValueShape(encoded valueShapeEnvelopeV1) (ValueShape, error) {
	switch ValueShapeKind(encoded.Kind) {
	case ValueShapeScalar:
		payload, err := decodeVariantPayload[scalarValueShapeV1](encoded.Payload, "scalar ValueShape")
		if err != nil {
			return nil, err
		}
		shape, err := NewScalarShape(ScalarKind(payload.ScalarKind))
		if err != nil {
			return nil, err
		}
		return shape, nil
	case ValueShapeRecord:
		payload, err := decodeVariantPayload[recordValueShapeV1](encoded.Payload, "record ValueShape")
		if err != nil {
			return nil, err
		}
		fields := make([]RecordFieldShape, 0, len(payload.Fields))
		for index, encodedField := range payload.Fields {
			name, nameErr := NewValueMemberName(encodedField.Name)
			if nameErr != nil {
				return nil, fmt.Errorf("record field %d: %w", index, nameErr)
			}
			ref, refErr := decodeValueShapeRef(encodedField.Shape)
			if refErr != nil {
				return nil, fmt.Errorf("record field %d: %w", index, refErr)
			}
			field, fieldErr := NewRecordFieldShape(name, ref)
			if fieldErr != nil {
				return nil, fmt.Errorf("record field %d: %w", index, fieldErr)
			}
			fields = append(fields, field)
		}
		shape, err := NewRecordShape(fields)
		if err != nil {
			return nil, err
		}
		return shape, nil
	case ValueShapeSum:
		payload, err := decodeVariantPayload[sumValueShapeV1](encoded.Payload, "sum ValueShape")
		if err != nil {
			return nil, err
		}
		variants := make([]SumVariantShape, 0, len(payload.Variants))
		for index, encodedVariant := range payload.Variants {
			name, nameErr := NewValueMemberName(encodedVariant.Name)
			if nameErr != nil {
				return nil, fmt.Errorf("sum variant %d: %w", index, nameErr)
			}
			ref, refErr := decodeValueShapeRef(encodedVariant.Shape)
			if refErr != nil {
				return nil, fmt.Errorf("sum variant %d: %w", index, refErr)
			}
			variant, variantErr := NewSumVariantShape(name, ref)
			if variantErr != nil {
				return nil, fmt.Errorf("sum variant %d: %w", index, variantErr)
			}
			variants = append(variants, variant)
		}
		shape, err := NewSumShape(variants)
		if err != nil {
			return nil, err
		}
		return shape, nil
	case ValueShapeOrderedSequence:
		payload, err := decodeVariantPayload[elementValueShapeV1](encoded.Payload, "ordered-sequence ValueShape")
		if err != nil {
			return nil, err
		}
		element, err := decodeValueShapeRef(payload.Element)
		if err != nil {
			return nil, err
		}
		shape, err := NewOrderedSequenceShape(element)
		if err != nil {
			return nil, err
		}
		return shape, nil
	case ValueShapeUnorderedSet:
		payload, err := decodeVariantPayload[elementValueShapeV1](encoded.Payload, "unordered-set ValueShape")
		if err != nil {
			return nil, err
		}
		element, err := decodeValueShapeRef(payload.Element)
		if err != nil {
			return nil, err
		}
		shape, err := NewUnorderedSetShape(element)
		if err != nil {
			return nil, err
		}
		return shape, nil
	case ValueShapeClaimGraph:
		if _, err := decodeVariantPayload[emptyValueShapeV1](encoded.Payload, "ClaimGraph ValueShape"); err != nil {
			return nil, err
		}
		return NewClaimGraphShape(), nil
	default:
		return nil, fmt.Errorf("unknown ValueShape variant %q", encoded.Kind)
	}
}

func decodeValueBindingChange(payload []byte) (SchemaChange, error) {
	encoded, err := decodeVariantPayload[valueBindingChangeV1](payload, "bind_value_kind")
	if err != nil {
		return nil, err
	}
	valueKind, err := decodeValueKindRef(encoded.ValueKind)
	if err != nil {
		return nil, err
	}
	shape, err := decodeValueShapeRef(encoded.ValueShape)
	if err != nil {
		return nil, err
	}
	codec, err := decodeCodecRef(encoded.Codec)
	if err != nil {
		return nil, err
	}
	provenance, err := decodeDeclarationProvenance(encoded.Provenance)
	if err != nil {
		return nil, err
	}
	binding, err := NewValueBinding(valueKind, shape, codec, provenance)
	if err != nil {
		return nil, err
	}
	change, err := NewBindValueKindSchemaChange(binding)
	if err != nil {
		return nil, err
	}
	return change, nil
}

func encodeConstraintChange(rule ConstraintRule) (constraintChangeEnvelopeV1, error) {
	switch value := rule.(type) {
	case KindDisjointConstraint:
		provenance, err := encodeDeclarationProvenance(value.provenance)
		if err != nil {
			return constraintChangeEnvelopeV1{}, err
		}
		kinds := make([]string, 0, len(value.kinds))
		for _, kindID := range value.kinds {
			kinds = append(kinds, kindID.String())
		}
		payload := kindDisjointConstraintV1{
			ID:         value.id.String(),
			Kinds:      kinds,
			Provenance: provenance,
		}
		return newConstraintEnvelope("kind_disjoint", payload)
	case SlotGroupConstraint:
		provenance, err := encodeDeclarationProvenance(value.provenance)
		if err != nil {
			return constraintChangeEnvelopeV1{}, err
		}
		slots := make([]string, 0, len(value.slots))
		for _, slot := range value.slots {
			slots = append(slots, slot.String())
		}
		payload := slotGroupConstraintV1{
			ID:         value.id.String(),
			Signature:  encodeRelationSignatureRef(value.signature),
			Slots:      slots,
			Mode:       value.mode.String(),
			Provenance: provenance,
		}
		return newConstraintEnvelope("slot_group", payload)
	case SlotCardinalityConstraint:
		provenance, err := encodeDeclarationProvenance(value.provenance)
		if err != nil {
			return constraintChangeEnvelopeV1{}, err
		}
		payload := slotCardinalityConstraintV1{
			ID:          value.id.String(),
			Signature:   encodeRelationSignatureRef(value.signature),
			Slot:        value.slot.String(),
			Cardinality: encodeCardinality(value.cardinality),
			Provenance:  provenance,
		}
		return newConstraintEnvelope("slot_cardinality", payload)
	case ReferenceSlotSubsetConstraint:
		provenance, err := encodeDeclarationProvenance(value.provenance)
		if err != nil {
			return constraintChangeEnvelopeV1{}, err
		}
		payload := referenceSlotSubsetConstraintV1{
			ID:         value.id.String(),
			Signature:  encodeRelationSignatureRef(value.signature),
			Subset:     value.subset.String(),
			Superset:   value.superset.String(),
			Provenance: provenance,
		}
		return newConstraintEnvelope("reference_slot_subset", payload)
	case ReferenceSlotPartitionConstraint:
		provenance, err := encodeDeclarationProvenance(value.provenance)
		if err != nil {
			return constraintChangeEnvelopeV1{}, err
		}
		parts := make([]string, 0, len(value.parts))
		for _, part := range value.parts {
			parts = append(parts, part.String())
		}
		payload := referenceSlotPartitionConstraintV1{
			ID:         value.id.String(),
			Signature:  encodeRelationSignatureRef(value.signature),
			Whole:      value.whole.String(),
			Parts:      parts,
			Provenance: provenance,
		}
		return newConstraintEnvelope("reference_slot_partition", payload)
	default:
		return constraintChangeEnvelopeV1{}, fmt.Errorf("unknown ConstraintRule variant %T", rule)
	}
}

func newConstraintEnvelope(kind string, payload any) (constraintChangeEnvelopeV1, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return constraintChangeEnvelopeV1{}, fmt.Errorf("encode %s constraint: %w", kind, err)
	}
	return constraintChangeEnvelopeV1{Kind: kind, Payload: encoded}, nil
}

func decodeConstraintSchemaChange(payload []byte) (SchemaChange, error) {
	envelope, err := decodeVariantPayload[constraintChangeEnvelopeV1](payload, "add_constraint")
	if err != nil {
		return nil, err
	}
	rule, err := decodeConstraintRule(envelope)
	if err != nil {
		return nil, err
	}
	change, err := NewAddConstraintSchemaChange(rule)
	if err != nil {
		return nil, err
	}
	return change, nil
}

func decodeConstraintRule(envelope constraintChangeEnvelopeV1) (ConstraintRule, error) {
	switch envelope.Kind {
	case "kind_disjoint":
		return decodeKindDisjointConstraint(envelope.Payload)
	case "slot_group":
		return decodeSlotGroupConstraint(envelope.Payload)
	case "slot_cardinality":
		return decodeSlotCardinalityConstraint(envelope.Payload)
	case "reference_slot_subset":
		return decodeReferenceSlotSubsetConstraint(envelope.Payload)
	case "reference_slot_partition":
		return decodeReferenceSlotPartitionConstraint(envelope.Payload)
	default:
		return nil, fmt.Errorf("unknown ConstraintRule variant %q", envelope.Kind)
	}
}

func decodeKindDisjointConstraint(payload []byte) (ConstraintRule, error) {
	encoded, err := decodeVariantPayload[kindDisjointConstraintV1](payload, "kind_disjoint constraint")
	if err != nil {
		return nil, err
	}
	id, err := NewConstraintID(encoded.ID)
	if err != nil {
		return nil, err
	}
	kinds := make([]KindID, 0, len(encoded.Kinds))
	for index, raw := range encoded.Kinds {
		kindID, kindErr := NewKindID(raw)
		if kindErr != nil {
			return nil, fmt.Errorf("kind-disjoint operand %d: %w", index, kindErr)
		}
		kinds = append(kinds, kindID)
	}
	provenance, err := decodeDeclarationProvenance(encoded.Provenance)
	if err != nil {
		return nil, err
	}
	constraint, err := NewKindDisjointConstraint(id, kinds, provenance)
	if err != nil {
		return nil, err
	}
	return constraint, nil
}

func decodeSlotGroupConstraint(payload []byte) (ConstraintRule, error) {
	encoded, err := decodeVariantPayload[slotGroupConstraintV1](payload, "slot_group constraint")
	if err != nil {
		return nil, err
	}
	id, err := NewConstraintID(encoded.ID)
	if err != nil {
		return nil, err
	}
	signature, err := decodeRelationSignatureRef(encoded.Signature)
	if err != nil {
		return nil, err
	}
	slots := make([]SlotKindID, 0, len(encoded.Slots))
	for index, raw := range encoded.Slots {
		slot, slotErr := NewSlotKindID(raw)
		if slotErr != nil {
			return nil, fmt.Errorf("slot-group operand %d: %w", index, slotErr)
		}
		slots = append(slots, slot)
	}
	mode, err := parseSlotGroupMode(encoded.Mode)
	if err != nil {
		return nil, err
	}
	provenance, err := decodeDeclarationProvenance(encoded.Provenance)
	if err != nil {
		return nil, err
	}
	constraint, err := NewSlotGroupConstraint(id, signature, slots, mode, provenance)
	if err != nil {
		return nil, err
	}
	return constraint, nil
}

func parseSlotGroupMode(raw string) (SlotGroupMode, error) {
	for candidate := SlotGroupAllOrNone; candidate <= SlotGroupExactlyOne; candidate++ {
		if candidate.String() == raw {
			return candidate, nil
		}
	}
	return 0, fmt.Errorf("unknown slot-group mode %q", raw)
}

func decodeSlotCardinalityConstraint(payload []byte) (ConstraintRule, error) {
	encoded, err := decodeVariantPayload[slotCardinalityConstraintV1](payload, "slot_cardinality constraint")
	if err != nil {
		return nil, err
	}
	id, err := NewConstraintID(encoded.ID)
	if err != nil {
		return nil, err
	}
	signature, err := decodeRelationSignatureRef(encoded.Signature)
	if err != nil {
		return nil, err
	}
	slot, err := NewSlotKindID(encoded.Slot)
	if err != nil {
		return nil, err
	}
	cardinality, err := decodeCardinality(encoded.Cardinality)
	if err != nil {
		return nil, err
	}
	provenance, err := decodeDeclarationProvenance(encoded.Provenance)
	if err != nil {
		return nil, err
	}
	constraint, err := NewSlotCardinalityConstraint(
		id,
		signature,
		slot,
		cardinality,
		provenance,
	)
	if err != nil {
		return nil, err
	}
	return constraint, nil
}

func decodeReferenceSlotSubsetConstraint(payload []byte) (ConstraintRule, error) {
	encoded, err := decodeVariantPayload[referenceSlotSubsetConstraintV1](payload, "reference_slot_subset constraint")
	if err != nil {
		return nil, err
	}
	id, err := NewConstraintID(encoded.ID)
	if err != nil {
		return nil, err
	}
	signature, err := decodeRelationSignatureRef(encoded.Signature)
	if err != nil {
		return nil, err
	}
	subset, err := NewSlotKindID(encoded.Subset)
	if err != nil {
		return nil, err
	}
	superset, err := NewSlotKindID(encoded.Superset)
	if err != nil {
		return nil, err
	}
	provenance, err := decodeDeclarationProvenance(encoded.Provenance)
	if err != nil {
		return nil, err
	}
	constraint, err := NewReferenceSlotSubsetConstraint(
		id,
		signature,
		subset,
		superset,
		provenance,
	)
	if err != nil {
		return nil, err
	}
	return constraint, nil
}

func decodeReferenceSlotPartitionConstraint(payload []byte) (ConstraintRule, error) {
	encoded, err := decodeVariantPayload[referenceSlotPartitionConstraintV1](payload, "reference_slot_partition constraint")
	if err != nil {
		return nil, err
	}
	id, err := NewConstraintID(encoded.ID)
	if err != nil {
		return nil, err
	}
	signature, err := decodeRelationSignatureRef(encoded.Signature)
	if err != nil {
		return nil, err
	}
	whole, err := NewSlotKindID(encoded.Whole)
	if err != nil {
		return nil, err
	}
	parts := make([]SlotKindID, 0, len(encoded.Parts))
	for index, raw := range encoded.Parts {
		part, partErr := NewSlotKindID(raw)
		if partErr != nil {
			return nil, fmt.Errorf("reference-slot-partition part %d: %w", index, partErr)
		}
		parts = append(parts, part)
	}
	provenance, err := decodeDeclarationProvenance(encoded.Provenance)
	if err != nil {
		return nil, err
	}
	constraint, err := NewReferenceSlotPartitionConstraint(
		id,
		signature,
		whole,
		parts,
		provenance,
	)
	if err != nil {
		return nil, err
	}
	return constraint, nil
}

func encodeDeclarationProvenance(
	provenance DeclarationProvenance,
) (declarationProvenanceEnvelopeV1, error) {
	switch value := provenance.(type) {
	case FPFSourceProvenance:
		payload := fpfSourceProvenanceV1{
			Reference:    value.reference.String(),
			Location:     encodeSourceLocation(value.location),
			CompilerRule: value.ruleID.String(),
		}
		return newProvenanceEnvelope("fpf_source", payload)
	case CompilerDerivedProvenance:
		inputs := make([]sourceLocationV1, 0, len(value.inputs))
		for _, input := range value.inputs {
			inputs = append(inputs, encodeSourceLocation(input))
		}
		payload := compilerDerivedProvenanceV1{
			Reference:    value.reference.String(),
			Inputs:       inputs,
			CompilerRule: value.ruleID.String(),
		}
		return newProvenanceEnvelope("compiler_derived", payload)
	case ProjectSourceProvenance:
		payload := projectSourceProvenanceV1{
			Reference:      value.reference.String(),
			Carrier:        value.carrier.String(),
			Edition:        value.edition.String(),
			ContentDigest:  value.contentHash.String(),
			LineStart:      value.lineRange.Start(),
			LineEnd:        value.lineRange.End(),
			CompilerRule:   value.ruleID.String(),
			BoundedContext: value.context.String(),
			BaseTypeEnv:    value.baseTypeEnv.String(),
			SignatureRow:   value.signatureRow.String(),
			ManifestBasis: manifestSymbolBasisV1{
				ManifestID:      value.manifestBasis.manifest.ID(),
				ManifestVersion: value.manifestBasis.manifest.Version(),
				Direction:       value.manifestBasis.direction.String(),
				Symbol:          encodeSchemaSymbol(value.manifestBasis.symbol),
			},
		}
		return newProvenanceEnvelope("project_source", payload)
	default:
		return declarationProvenanceEnvelopeV1{}, fmt.Errorf(
			"unknown DeclarationProvenance variant %T",
			provenance,
		)
	}
}

func encodeSourceLocation(location SourceLocation) sourceLocationV1 {
	patternID, hasPattern := location.PatternID()
	pattern := ""
	if hasPattern {
		pattern = patternID.String()
	}
	return sourceLocationV1{
		UnitID:        location.unitID.String(),
		Revision:      location.revision.String(),
		ContentDigest: location.contentHash.String(),
		LineStart:     location.lineRange.Start(),
		LineEnd:       location.lineRange.End(),
		PatternID:     pattern,
	}
}

func newProvenanceEnvelope(
	kind string,
	payload any,
) (declarationProvenanceEnvelopeV1, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return declarationProvenanceEnvelopeV1{}, fmt.Errorf("encode %s provenance: %w", kind, err)
	}
	return declarationProvenanceEnvelopeV1{Kind: kind, Payload: encoded}, nil
}

func decodeDeclarationProvenance(
	envelope declarationProvenanceEnvelopeV1,
) (DeclarationProvenance, error) {
	switch envelope.Kind {
	case "fpf_source":
		return decodeFPFSourceProvenance(envelope.Payload)
	case "compiler_derived":
		return decodeCompilerDerivedProvenance(envelope.Payload)
	case "project_source":
		return decodeProjectSourceProvenance(envelope.Payload)
	default:
		return nil, fmt.Errorf("unknown DeclarationProvenance variant %q", envelope.Kind)
	}
}

func decodeFPFSourceProvenance(payload []byte) (DeclarationProvenance, error) {
	encoded, err := decodeVariantPayload[fpfSourceProvenanceV1](payload, "FPF source provenance")
	if err != nil {
		return nil, err
	}
	reference, err := NewProvenanceRef(encoded.Reference)
	if err != nil {
		return nil, err
	}
	location, err := decodeSourceLocation(encoded.Location)
	if err != nil {
		return nil, err
	}
	rule, err := NewCompilerRuleID(encoded.CompilerRule)
	if err != nil {
		return nil, err
	}
	provenance, err := NewFPFSourceProvenance(reference, location, rule)
	if err != nil {
		return nil, err
	}
	return provenance, nil
}

func decodeCompilerDerivedProvenance(payload []byte) (DeclarationProvenance, error) {
	encoded, err := decodeVariantPayload[compilerDerivedProvenanceV1](payload, "compiler-derived provenance")
	if err != nil {
		return nil, err
	}
	reference, err := NewProvenanceRef(encoded.Reference)
	if err != nil {
		return nil, err
	}
	inputs := make([]SourceLocation, 0, len(encoded.Inputs))
	for index, encodedInput := range encoded.Inputs {
		input, inputErr := decodeSourceLocation(encodedInput)
		if inputErr != nil {
			return nil, fmt.Errorf("compiler-derived input %d: %w", index, inputErr)
		}
		inputs = append(inputs, input)
	}
	rule, err := NewCompilerRuleID(encoded.CompilerRule)
	if err != nil {
		return nil, err
	}
	provenance, err := NewCompilerDerivedProvenance(reference, inputs, rule)
	if err != nil {
		return nil, err
	}
	return provenance, nil
}

func decodeProjectSourceProvenance(payload []byte) (DeclarationProvenance, error) {
	encoded, err := decodeVariantPayload[projectSourceProvenanceV1](payload, "project-source provenance")
	if err != nil {
		return nil, err
	}
	reference, err := NewProvenanceRef(encoded.Reference)
	if err != nil {
		return nil, err
	}
	carrier, err := NewCarrierRef(encoded.Carrier)
	if err != nil {
		return nil, err
	}
	edition, err := NewCarrierEdition(encoded.Edition)
	if err != nil {
		return nil, err
	}
	digest, err := NewSHA256Digest(encoded.ContentDigest)
	if err != nil {
		return nil, err
	}
	lineRange, err := NewSourceLineRange(encoded.LineStart, encoded.LineEnd)
	if err != nil {
		return nil, err
	}
	rule, err := NewCompilerRuleID(encoded.CompilerRule)
	if err != nil {
		return nil, err
	}
	context, err := NewBoundedContextRef(encoded.BoundedContext)
	if err != nil {
		return nil, err
	}
	base, err := ParseTypeEnvRef(encoded.BaseTypeEnv)
	if err != nil {
		return nil, err
	}
	row, err := parseSignatureBlockRow(encoded.SignatureRow)
	if err != nil {
		return nil, err
	}
	basis, err := decodeManifestSymbolBasis(encoded.ManifestBasis)
	if err != nil {
		return nil, err
	}
	provenance, err := NewProjectSourceProvenanceBuilder(reference, carrier, edition, digest).
		SetDeclarationRange(lineRange).
		SetCompilerRule(rule).
		SetBoundedContext(context).
		SetBaseTypeEnv(base).
		SetSignatureBlockRow(row).
		SetManifestBasis(basis).
		Build()
	if err != nil {
		return nil, err
	}
	return provenance, nil
}

func decodeSourceLocation(encoded sourceLocationV1) (SourceLocation, error) {
	unitID, err := NewSourceUnitID(encoded.UnitID)
	if err != nil {
		return SourceLocation{}, err
	}
	revision, err := NewSourceRevision(encoded.Revision)
	if err != nil {
		return SourceLocation{}, err
	}
	digest, err := NewSHA256Digest(encoded.ContentDigest)
	if err != nil {
		return SourceLocation{}, err
	}
	lineRange, err := NewSourceLineRange(encoded.LineStart, encoded.LineEnd)
	if err != nil {
		return SourceLocation{}, err
	}
	if encoded.PatternID == "" {
		return NewUnpatternedSourceLocation(unitID, revision, digest, lineRange)
	}
	patternID, err := NewPatternID(encoded.PatternID)
	if err != nil {
		return SourceLocation{}, err
	}
	return NewPatternedSourceLocation(unitID, revision, digest, lineRange, patternID)
}

func decodeManifestSymbolBasis(encoded manifestSymbolBasisV1) (ManifestSymbolBasis, error) {
	manifest, err := NewSignatureManifestRef(encoded.ManifestID, encoded.ManifestVersion)
	if err != nil {
		return ManifestSymbolBasis{}, err
	}
	direction, err := parseManifestDirection(encoded.Direction)
	if err != nil {
		return ManifestSymbolBasis{}, err
	}
	symbol, err := decodeSchemaSymbol(encoded.Symbol)
	if err != nil {
		return ManifestSymbolBasis{}, err
	}
	basis, err := NewManifestSymbolBasis(manifest, direction, symbol)
	if err != nil {
		return ManifestSymbolBasis{}, err
	}
	return basis, nil
}

func parseSignatureBlockRow(raw string) (SignatureBlockRow, error) {
	for candidate := SubjectBlockRow; candidate <= ApplicabilityRow; candidate++ {
		if candidate.String() == raw {
			return candidate, nil
		}
	}
	return 0, fmt.Errorf("unknown signature-block row %q", raw)
}

func parseManifestDirection(raw string) (ManifestDirection, error) {
	for candidate := ManifestImport; candidate <= ManifestProvide; candidate++ {
		if candidate.String() == raw {
			return candidate, nil
		}
	}
	return 0, fmt.Errorf("unknown manifest direction %q", raw)
}

func encodeRelationSignatureRef(ref RelationSignatureRef) typeEnvIDRefV1 {
	return typeEnvIDRefV1{TypeEnv: ref.typeEnv.String(), ID: ref.id.String()}
}

func decodeRelationSignatureRef(encoded typeEnvIDRefV1) (RelationSignatureRef, error) {
	typeEnv, err := ParseTypeEnvRef(encoded.TypeEnv)
	if err != nil {
		return RelationSignatureRef{}, err
	}
	id, err := NewSignatureID(encoded.ID)
	if err != nil {
		return RelationSignatureRef{}, err
	}
	ref, err := NewRelationSignatureRef(typeEnv, id)
	if err != nil {
		return RelationSignatureRef{}, err
	}
	return ref, nil
}

func encodeValueKindRef(ref ValueKindRef) typeEnvIDRefV1 {
	return typeEnvIDRefV1{TypeEnv: ref.typeEnv.String(), ID: ref.id.String()}
}

func decodeValueKindRef(encoded typeEnvIDRefV1) (ValueKindRef, error) {
	typeEnv, err := ParseTypeEnvRef(encoded.TypeEnv)
	if err != nil {
		return ValueKindRef{}, err
	}
	id, err := NewKindID(encoded.ID)
	if err != nil {
		return ValueKindRef{}, err
	}
	ref, err := NewValueKindRef(typeEnv, id)
	if err != nil {
		return ValueKindRef{}, err
	}
	return ref, nil
}

func encodeRefKindRef(ref RefKindRef) typeEnvIDRefV1 {
	return typeEnvIDRefV1{TypeEnv: ref.typeEnv.String(), ID: ref.id.String()}
}

func decodeRefKindRef(encoded typeEnvIDRefV1) (RefKindRef, error) {
	typeEnv, err := ParseTypeEnvRef(encoded.TypeEnv)
	if err != nil {
		return RefKindRef{}, err
	}
	id, err := NewRefKindID(encoded.ID)
	if err != nil {
		return RefKindRef{}, err
	}
	ref, err := NewRefKindRef(typeEnv, id)
	if err != nil {
		return RefKindRef{}, err
	}
	return ref, nil
}

func encodeValueShapeRef(ref ValueShapeRef) valueShapeRefV1 {
	return valueShapeRefV1{ID: ref.id.String(), Digest: ref.digest.String()}
}

func decodeValueShapeRef(encoded valueShapeRefV1) (ValueShapeRef, error) {
	id, err := NewShapeID(encoded.ID)
	if err != nil {
		return ValueShapeRef{}, err
	}
	digest, err := NewSHA256Digest(encoded.Digest)
	if err != nil {
		return ValueShapeRef{}, err
	}
	ref, err := NewValueShapeRef(id, digest)
	if err != nil {
		return ValueShapeRef{}, err
	}
	return ref, nil
}

func encodeCodecRef(ref CodecRef) codecRefV1 {
	return codecRefV1{
		ID:      ref.id.String(),
		Version: ref.version.String(),
		Digest:  ref.digest.String(),
	}
}

func decodeCodecRef(encoded codecRefV1) (CodecRef, error) {
	id, err := NewCodecID(encoded.ID)
	if err != nil {
		return CodecRef{}, err
	}
	version, err := NewCanonicalizationVersion(encoded.Version)
	if err != nil {
		return CodecRef{}, err
	}
	digest, err := NewSHA256Digest(encoded.Digest)
	if err != nil {
		return CodecRef{}, err
	}
	ref, err := NewCodecRef(id, version, digest)
	if err != nil {
		return CodecRef{}, err
	}
	return ref, nil
}

// DecodeLoweredTypeEnvExtensionProposal accepts only exact canonical bytes,
// rebuilds every strong domain value, and derives proposal identity from those
// bytes. Embedded TypeEnvRefs are exact inputs; this function never derives or
// substitutes a composite TypeEnv.
func DecodeLoweredTypeEnvExtensionProposal(canonical []byte) (TypeEnvExtensionProposal, error) {
	if len(canonical) == 0 {
		return TypeEnvExtensionProposal{}, fmt.Errorf("TypeEnv extension canonical bytes are required")
	}
	if len(canonical) > maxTypeEnvExtensionCanonicalSize {
		return TypeEnvExtensionProposal{}, fmt.Errorf(
			"TypeEnv extension canonical bytes exceed %d-byte limit",
			maxTypeEnvExtensionCanonicalSize,
		)
	}
	payload, err := decodeExtensionCanonicalEnvelope(canonical)
	if err != nil {
		return TypeEnvExtensionProposal{}, err
	}
	if !utf8.Valid(payload) {
		return TypeEnvExtensionProposal{}, fmt.Errorf("TypeEnv extension canonical payload contains invalid UTF-8")
	}
	var encoded extensionCanonicalV1
	if err := decodeStrictJSON(payload, &encoded); err != nil {
		return TypeEnvExtensionProposal{}, fmt.Errorf("TypeEnv extension payload: %w", err)
	}
	candidate, err := decodeExtensionCandidate(encoded)
	if err != nil {
		return TypeEnvExtensionProposal{}, err
	}
	if err := validateLoweredTypeEnvExtensionCandidate(candidate); err != nil {
		return TypeEnvExtensionProposal{}, err
	}
	reencoded, err := encodeLoweredTypeEnvExtensionCandidate(candidate)
	if err != nil {
		return TypeEnvExtensionProposal{}, err
	}
	if !bytes.Equal(reencoded, canonical) {
		return TypeEnvExtensionProposal{}, fmt.Errorf("TypeEnv extension payload is not canonical")
	}
	ref, err := newTypeEnvExtensionRef(candidate.id, digestCanonicalBytes(reencoded))
	if err != nil {
		return TypeEnvExtensionProposal{}, err
	}
	proposal := proposalFromCandidate(candidate, ref, reencoded)
	if err := verifyTypeEnvExtensionProposal(proposal); err != nil {
		return TypeEnvExtensionProposal{}, err
	}
	return cloneTypeEnvExtensionProposal(proposal), nil
}

// VerifyLoweredTypeEnvExtensionProposal proves that external lowered bytes
// have exactly the expected content-derived coordinate and returns the sealed
// decoded value.
func VerifyLoweredTypeEnvExtensionProposal(
	expected TypeEnvExtensionRef,
	canonical []byte,
) (TypeEnvExtensionProposal, error) {
	if !expected.valid() {
		return TypeEnvExtensionProposal{}, fmt.Errorf("expected TypeEnv extension reference is required")
	}
	proposal, err := DecodeLoweredTypeEnvExtensionProposal(canonical)
	if err != nil {
		return TypeEnvExtensionProposal{}, err
	}
	if proposal.ref != expected {
		return TypeEnvExtensionProposal{}, fmt.Errorf(
			"TypeEnv extension reference %q does not match canonical bytes %q",
			expected.String(),
			proposal.ref.String(),
		)
	}
	return proposal, nil
}

func encodeLoweredTypeEnvExtensionCandidate(candidate loweredTypeEnvExtensionCandidate) ([]byte, error) {
	if err := validateCanonicalUTF8(reflect.ValueOf(candidate), "extension"); err != nil {
		return nil, err
	}
	encoded, err := encodeExtensionCandidate(candidate)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(encoded)
	if err != nil {
		return nil, fmt.Errorf("encode TypeEnv extension payload: %w", err)
	}
	writer := newCanonicalWriter(typeEnvExtensionProposalDomain)
	writer.addBytes(payload)
	return writer.bytes(), nil
}

func validateCanonicalUTF8(value reflect.Value, path string) error {
	if !value.IsValid() {
		return nil
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return nil
		}
		return validateCanonicalUTF8(value.Elem(), path)
	case reflect.String:
		if !utf8.ValidString(value.String()) {
			return fmt.Errorf("%s contains invalid UTF-8", path)
		}
		return nil
	case reflect.Struct:
		valueType := value.Type()
		for index := 0; index < value.NumField(); index++ {
			fieldPath := path + "." + valueType.Field(index).Name
			if err := validateCanonicalUTF8(value.Field(index), fieldPath); err != nil {
				return err
			}
		}
		return nil
	case reflect.Array, reflect.Slice:
		for index := 0; index < value.Len(); index++ {
			itemPath := fmt.Sprintf("%s[%d]", path, index)
			if err := validateCanonicalUTF8(value.Index(index), itemPath); err != nil {
				return err
			}
		}
		return nil
	default:
		return nil
	}
}

func decodeExtensionCanonicalEnvelope(canonical []byte) ([]byte, error) {
	reader, err := newDomainReader(canonical, typeEnvExtensionProposalDomain)
	if err != nil {
		return nil, fmt.Errorf("TypeEnv extension canonical envelope: %w", err)
	}
	payload, err := reader.readBytes()
	if err != nil {
		return nil, fmt.Errorf("TypeEnv extension canonical payload: %w", err)
	}
	if err := reader.requireEnd(); err != nil {
		return nil, fmt.Errorf("TypeEnv extension canonical envelope: %w", err)
	}
	return payload, nil
}

func decodeStrictJSON(payload []byte, destination any) error {
	if err := rejectDuplicateJSONKeys(payload); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("JSON payload has trailing value")
}

func rejectDuplicateJSONKeys(payload []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var consumeValue func() error
	consumeValue = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, composite := token.(json.Delim)
		if !composite {
			return nil
		}
		switch delimiter {
		case '{':
			seen := make(map[string]struct{})
			for decoder.More() {
				keyToken, keyErr := decoder.Token()
				if keyErr != nil {
					return keyErr
				}
				key, ok := keyToken.(string)
				if !ok {
					return fmt.Errorf("JSON object key is not a string")
				}
				if _, duplicate := seen[key]; duplicate {
					return fmt.Errorf("JSON object has duplicate field %q", key)
				}
				seen[key] = struct{}{}
				if valueErr := consumeValue(); valueErr != nil {
					return valueErr
				}
			}
			closing, closingErr := decoder.Token()
			if closingErr != nil {
				return closingErr
			}
			if closing != json.Delim('}') {
				return fmt.Errorf("JSON object is not closed")
			}
			return nil
		case '[':
			for decoder.More() {
				if valueErr := consumeValue(); valueErr != nil {
					return valueErr
				}
			}
			closing, closingErr := decoder.Token()
			if closingErr != nil {
				return closingErr
			}
			if closing != json.Delim(']') {
				return fmt.Errorf("JSON array is not closed")
			}
			return nil
		default:
			return fmt.Errorf("unknown JSON delimiter %q", delimiter)
		}
	}
	if err := consumeValue(); err != nil {
		return err
	}
	if _, err := decoder.Token(); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("JSON payload has trailing value")
}
