package typeenv

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	claimGraphValueKindID = "U.ClaimGraph"
	claimGraphSlotKindID  = "ClaimGraphSlot"
	claimGraphOwnerID     = "C.2.1"
)

type claimGraphLinkedRepresentation struct {
	shape LinkedDeclaration
	codec LinkedDeclaration
}

type claimGraphSourceFragment struct {
	source    fpf.SourceUnit
	owner     string
	slotKind  string
	valueKind string
	reference ReferenceModeEvidence
}

func linkClaimGraphRepresentation(
	structural []StructuralDeclaration,
) (claimGraphLinkedRepresentation, bool, error) {
	fragments := make([]claimGraphSourceFragment, 0, 1)
	for _, declaration := range structural {
		switch typed := declaration.(type) {
		case SlotDeclarationFragment:
			if typed.ValueKind() != claimGraphValueKindID {
				continue
			}
			fragments = append(fragments, claimGraphSourceFragment{
				source:    typed.Source(),
				owner:     typed.OwnerPatternID(),
				slotKind:  typed.SlotKind(),
				valueKind: typed.ValueKind(),
				reference: typed.ReferenceMode(),
			})
		case SymbolicRelationSignatureDeclaration:
			for _, slot := range typed.Slots() {
				if slot.ValueKind() != claimGraphValueKindID {
					continue
				}
				fragments = append(fragments, claimGraphSourceFragment{
					source:    typed.Source(),
					owner:     typed.OwnerPatternID(),
					slotKind:  slot.SlotKind(),
					valueKind: slot.ValueKind(),
					reference: slot.ReferenceMode(),
				})
			}
		}
	}
	if len(fragments) == 0 {
		return claimGraphLinkedRepresentation{}, false, nil
	}
	if len(fragments) != 1 {
		return claimGraphLinkedRepresentation{}, false, fmt.Errorf(
			"source declares %d U.ClaimGraph slot fragments; exactly one is required",
			len(fragments),
		)
	}
	fragment := fragments[0]
	if err := validateClaimGraphSourceFragment(fragment); err != nil {
		return claimGraphLinkedRepresentation{}, false, err
	}
	units := []fpf.SourceUnit{fragment.source}
	locations, err := sourceLocationsFromUnits(units)
	if err != nil {
		return claimGraphLinkedRepresentation{}, false, err
	}
	valueSymbol, err := kindSymbol(claimGraphValueKindID)
	if err != nil {
		return claimGraphLinkedRepresentation{}, false, err
	}
	shapeRef, err := newClaimGraphShapeRef()
	if err != nil {
		return claimGraphLinkedRepresentation{}, false, err
	}
	budget := P6ClaimGraphDecodeBudget()
	codecRef, err := newClaimGraphCodecRef(shapeRef, budget)
	if err != nil {
		return claimGraphLinkedRepresentation{}, false, err
	}
	bodies, err := claimGraphDeclarationBodies(
		valueSymbol,
		shapeRef,
		codecRef,
		budget,
		locations,
	)
	if err != nil {
		return claimGraphLinkedRepresentation{}, false, err
	}
	shapeSymbol, err := typedmemory.ValueShapeSymbolRef(shapeRef.ID())
	if err != nil {
		return claimGraphLinkedRepresentation{}, false, err
	}
	shape, err := compilerDerivedLinkedDeclaration(
		shapeSymbol,
		claimGraphRepresentationRule,
		bodies.shape,
		units,
	)
	if err != nil {
		return claimGraphLinkedRepresentation{}, false, err
	}
	codecSymbol, err := typedmemory.CodecSymbolRef(codecRef.ID())
	if err != nil {
		return claimGraphLinkedRepresentation{}, false, err
	}
	codec, err := compilerDerivedLinkedDeclaration(
		codecSymbol,
		claimGraphRepresentationRule,
		bodies.codec,
		units,
	)
	if err != nil {
		return claimGraphLinkedRepresentation{}, false, err
	}
	linked := claimGraphLinkedRepresentation{shape: shape, codec: codec}
	return linked, true, nil
}

func validateClaimGraphSourceFragment(fragment claimGraphSourceFragment) error {
	if fragment.owner != claimGraphOwnerID {
		return fmt.Errorf(
			"U.ClaimGraph representation source owner is %q, want %q",
			fragment.owner,
			claimGraphOwnerID,
		)
	}
	if fragment.slotKind != claimGraphSlotKindID {
		return fmt.Errorf(
			"U.ClaimGraph source SlotKind is %q, want %q",
			fragment.slotKind,
			claimGraphSlotKindID,
		)
	}
	if fragment.valueKind != claimGraphValueKindID {
		return fmt.Errorf(
			"U.ClaimGraph source ValueKind is %q, want %q",
			fragment.valueKind,
			claimGraphValueKindID,
		)
	}
	if _, ok := fragment.reference.(ByValueEvidence); !ok {
		return fmt.Errorf("U.ClaimGraph source is not explicitly ByValue")
	}
	return nil
}

type claimGraphBodies struct {
	shape DeclarationBody
	codec DeclarationBody
}

func claimGraphDeclarationBodies(
	valueSymbol typedmemory.SchemaSymbolRef,
	shapeRef typedmemory.ValueShapeRef,
	codecRef typedmemory.CodecRef,
	budget ClaimGraphDecodeBudget,
	locations []typedmemory.SourceLocation,
) (claimGraphBodies, error) {
	valueReference, err := NewSymbolValue(valueSymbol)
	if err != nil {
		return claimGraphBodies{}, err
	}
	sourceInputs, err := claimGraphSourceInputSet(locations)
	if err != nil {
		return claimGraphBodies{}, err
	}
	shapeBody, err := newDeclarationBody([]fieldInput{
		{name: "shape_id", value: NewTextValue(shapeRef.ID().String())},
		{name: "shape_digest", value: NewTextValue(shapeRef.Digest().String())},
		{name: "shape_kind", value: NewTextValue(string(typedmemory.ValueShapeClaimGraph))},
		{name: "value_kind", value: valueReference},
		{name: "closed_value_algebra", value: NewBooleanValue(true)},
		{name: "source_inputs", value: sourceInputs},
	})
	if err != nil {
		return claimGraphBodies{}, err
	}
	shapeSymbol, err := typedmemory.ValueShapeSymbolRef(shapeRef.ID())
	if err != nil {
		return claimGraphBodies{}, err
	}
	shapeReference, err := NewSymbolValue(shapeSymbol)
	if err != nil {
		return claimGraphBodies{}, err
	}
	budgetValue, err := claimGraphBudgetValue(budget)
	if err != nil {
		return claimGraphBodies{}, err
	}
	domains, err := claimGraphTextSet([]string{
		claimGraphCanonicalEnvelopeDomain,
		claimGraphCanonicalValueDomain,
		claimGraphCanonicalNodeDomain,
		claimGraphCanonicalEdgeDomain,
		claimGraphCanonicalTypedDomain,
	})
	if err != nil {
		return claimGraphBodies{}, err
	}
	invariants, err := claimGraphTextSet([]string{
		"closed_value_algebra",
		"no_arbitrary_json",
		"unordered_node_set",
		"unordered_edge_set",
		"duplicate_node_id_rejected",
		"duplicate_edge_id_rejected",
		"dangling_endpoint_rejected",
		"exact_typed_value_digest",
	})
	if err != nil {
		return claimGraphBodies{}, err
	}
	codecBody, err := newDeclarationBody([]fieldInput{
		{name: "codec_id", value: NewTextValue(codecRef.ID().String())},
		{name: "canonicalization_version", value: NewTextValue(codecRef.Version().String())},
		{name: "codec_spec_digest", value: NewTextValue(codecRef.SpecificationDigest().String())},
		{name: "shape", value: shapeReference},
		{name: "value_kind", value: valueReference},
		{name: "decode_budget", value: budgetValue},
		{name: "canonical_domains", value: domains},
		{name: "invariants", value: invariants},
		{name: "source_inputs", value: sourceInputs},
	})
	if err != nil {
		return claimGraphBodies{}, err
	}
	return claimGraphBodies{shape: shapeBody, codec: codecBody}, nil
}

func claimGraphBudgetValue(
	budget ClaimGraphDecodeBudget,
) (RecordValue, error) {
	fields := []fieldInput{
		{name: "max_canonical_bytes", value: NewUnsignedValue(budget.MaxCanonicalBytes())},
		{name: "max_nodes", value: NewUnsignedValue(budget.MaxNodes())},
		{name: "max_edges", value: NewUnsignedValue(budget.MaxEdges())},
		{name: "max_value_depth", value: NewUnsignedValue(budget.MaxValueDepth())},
		{name: "max_value_items", value: NewUnsignedValue(budget.MaxValueItems())},
	}
	declarationFields := make([]DeclarationField, 0, len(fields))
	for _, input := range fields {
		field, err := NewDeclarationField(input.name, input.value)
		if err != nil {
			return RecordValue{}, err
		}
		declarationFields = append(declarationFields, field)
	}
	return NewRecordValue(declarationFields)
}

func claimGraphTextSet(values []string) (SetValue, error) {
	items := make([]DeclarationValue, 0, len(values))
	for _, value := range values {
		items = append(items, NewTextValue(value))
	}
	return NewSetValue(items)
}

func sourceLocationsFromUnits(
	units []fpf.SourceUnit,
) ([]typedmemory.SourceLocation, error) {
	locations := make([]typedmemory.SourceLocation, 0, len(units))
	for _, unit := range units {
		location, err := sourceLocation(unit)
		if err != nil {
			return nil, err
		}
		locations = append(locations, location)
	}
	return locations, nil
}

func claimGraphSourceInputSet(
	locations []typedmemory.SourceLocation,
) (SetValue, error) {
	items := make([]DeclarationValue, 0, len(locations))
	for _, location := range locations {
		value, err := claimGraphSourceInputValue(location)
		if err != nil {
			return SetValue{}, err
		}
		items = append(items, value)
	}
	return NewSetValue(items)
}

func claimGraphSourceInputValue(
	location typedmemory.SourceLocation,
) (RecordValue, error) {
	lineRange := location.LineRange()
	patternID, hasPattern := location.PatternID()
	pattern := ""
	if hasPattern {
		pattern = patternID.String()
	}
	inputs := []fieldInput{
		{name: "unit_id", value: NewTextValue(location.UnitID().String())},
		{name: "revision", value: NewTextValue(location.Revision().String())},
		{name: "content_hash", value: NewTextValue(location.ContentHash().String())},
		{name: "start_line", value: NewUnsignedValue(lineRange.Start())},
		{name: "end_line", value: NewUnsignedValue(lineRange.End())},
		{name: "pattern_id", value: NewTextValue(pattern)},
	}
	fields := make([]DeclarationField, 0, len(inputs))
	for _, input := range inputs {
		field, err := NewDeclarationField(input.name, input.value)
		if err != nil {
			return RecordValue{}, err
		}
		fields = append(fields, field)
	}
	return NewRecordValue(fields)
}

func lowerClaimGraphRepresentation(
	artifact BaseTypeEnvArtifact,
	ref typedmemory.TypeEnvRef,
) (P6ClaimGraphRepresentation, bool, error) {
	shapeID, err := typedmemory.NewShapeID(claimGraphShapeID)
	if err != nil {
		return P6ClaimGraphRepresentation{}, false, err
	}
	shapeSymbol, err := typedmemory.ValueShapeSymbolRef(shapeID)
	if err != nil {
		return P6ClaimGraphRepresentation{}, false, err
	}
	codecID, err := typedmemory.NewCodecID(claimGraphCodecID)
	if err != nil {
		return P6ClaimGraphRepresentation{}, false, err
	}
	codecSymbol, err := typedmemory.CodecSymbolRef(codecID)
	if err != nil {
		return P6ClaimGraphRepresentation{}, false, err
	}
	shape, hasShape := artifactDeclaration(artifact, shapeSymbol)
	codec, hasCodec := artifactDeclaration(artifact, codecSymbol)
	if !hasShape && !hasCodec {
		return P6ClaimGraphRepresentation{}, false, nil
	}
	if !hasShape || !hasCodec {
		return P6ClaimGraphRepresentation{}, false, fmt.Errorf(
			"ClaimGraph representation requires both shape and codec declarations",
		)
	}
	if !bytes.Equal(
		canonicalDeclarationBasis(shape.Basis()),
		canonicalDeclarationBasis(codec.Basis()),
	) {
		return P6ClaimGraphRepresentation{}, false, fmt.Errorf(
			"ClaimGraph shape and codec have different exact source inputs",
		)
	}
	valueID, err := typedmemory.NewKindID(claimGraphValueKindID)
	if err != nil {
		return P6ClaimGraphRepresentation{}, false, err
	}
	valueKind, err := typedmemory.NewValueKindRef(ref, valueID)
	if err != nil {
		return P6ClaimGraphRepresentation{}, false, err
	}
	representation, err := NewP6ClaimGraphRepresentation(
		valueKind,
		shape.Basis().SourceLocations(),
	)
	if err != nil {
		return P6ClaimGraphRepresentation{}, false, err
	}
	if representation.ShapeRef().ID() != shapeID {
		return P6ClaimGraphRepresentation{}, false, fmt.Errorf(
			"lowered ClaimGraph shape differs from linked declaration",
		)
	}
	if representation.CodecRef().ID() != codecID {
		return P6ClaimGraphRepresentation{}, false, fmt.Errorf(
			"lowered ClaimGraph codec differs from linked declaration",
		)
	}
	return representation, true, nil
}

func artifactDeclaration(
	artifact BaseTypeEnvArtifact,
	symbol typedmemory.SchemaSymbolRef,
) (LinkedDeclaration, bool) {
	for _, declaration := range artifact.Declarations() {
		if declaration.Symbol() == symbol {
			return declaration, true
		}
	}
	return LinkedDeclaration{}, false
}

func validateExecutableDeclarationSchemas(
	artifact BaseTypeEnvArtifact,
) error {
	for _, declaration := range artifact.Declarations() {
		if !linkedDeclarationIsCompiled(artifact, declaration) {
			continue
		}
		var err error
		switch declaration.Symbol().Kind() {
		case typedmemory.ContextSymbol:
			err = validateContextDeclarationSchema(artifact, declaration)
		case typedmemory.KindSymbol:
			err = validateKindDeclarationSchema(declaration)
		case typedmemory.RefKindSymbol:
			err = validateRefKindDeclarationSchema(declaration)
		case typedmemory.SignatureSymbol:
			err = validateTypedRelationDeclarationFragmentSchema(declaration)
		case typedmemory.SlotKindSymbol:
			err = validateSlotDeclarationSchema(declaration)
		case typedmemory.ShapeSymbol, typedmemory.CodecSymbol:
			err = validateClaimGraphDeclarationSchema(artifact, declaration)
		case typedmemory.BridgeSymbol, typedmemory.ConstraintSymbol:
			err = fmt.Errorf(
				"materialized symbol %q has no executable body schema",
				declaration.Symbol().String(),
			)
		default:
			err = fmt.Errorf(
				"materialized symbol %q has unknown body schema",
				declaration.Symbol().String(),
			)
		}
		if err != nil {
			return fmt.Errorf(
				"declaration %q is not exactly lowerable: %w",
				declaration.Symbol().String(),
				err,
			)
		}
	}
	return nil
}

func validateContextDeclarationSchema(
	artifact BaseTypeEnvArtifact,
	declaration LinkedDeclaration,
) error {
	if declaration.RuleID().String() != publicationContextCompilerRule {
		return fmt.Errorf("context compiler rule is not %q", publicationContextCompilerRule)
	}
	expected, err := newDeclarationBody([]fieldInput{
		{name: "context_ref", value: NewTextValue(declaration.Symbol().Key())},
		{name: "source_revision", value: NewTextValue(artifact.SourceRevision().String())},
	})
	if err != nil {
		return err
	}
	return requireExactDeclarationBody(declaration, expected)
}

func validateKindDeclarationSchema(declaration LinkedDeclaration) error {
	if declaration.RuleID().String() != valueKindCompilerRule {
		return fmt.Errorf("kind compiler rule is not %q", valueKindCompilerRule)
	}
	expected, err := newDeclarationBody([]fieldInput{
		{name: "kind_id", value: NewTextValue(declaration.Symbol().Key())},
		{name: "semantic_role", value: NewTextValue("value_kind")},
	})
	if err != nil {
		return err
	}
	return requireExactDeclarationBody(declaration, expected)
}

func validateRefKindDeclarationSchema(declaration LinkedDeclaration) error {
	if declaration.RuleID().String() != refKindCompilerRule {
		return fmt.Errorf("RefKind compiler rule is not %q", refKindCompilerRule)
	}
	referent, err := declarationSymbolField(declaration, "referent_value_kind")
	if err != nil {
		return err
	}
	if referent.Kind() != typedmemory.KindSymbol {
		return fmt.Errorf("referent_value_kind is not a kind symbol")
	}
	reference, err := NewSymbolValue(referent)
	if err != nil {
		return err
	}
	expected, err := newDeclarationBody([]fieldInput{
		{name: "ref_kind", value: NewTextValue(declaration.Symbol().Key())},
		{name: "referent_value_kind", value: reference},
	})
	if err != nil {
		return err
	}
	return requireExactDeclarationBody(declaration, expected)
}

func validateTypedRelationDeclarationFragmentSchema(
	declaration LinkedDeclaration,
) error {
	if declaration.RuleID().String() != typedRelationFragmentRule {
		return fmt.Errorf(
			"typed relation declaration fragment compiler rule is not %q",
			typedRelationFragmentRule,
		)
	}
	subject, err := declarationTextField(declaration, "subject_kind")
	if err != nil {
		return err
	}
	if _, err := typedmemory.NewKindID(subject); err != nil {
		return fmt.Errorf("subject_kind is invalid: %w", err)
	}
	slots, err := declarationSymbolSetField(declaration, "slots")
	if err != nil {
		return err
	}
	slotValues := make([]DeclarationValue, 0, len(slots))
	owner := declaration.Symbol().Key() + "/slot/"
	for _, slot := range slots {
		if slot.Kind() != typedmemory.SlotKindSymbol || !strings.HasPrefix(slot.Key(), owner) {
			return fmt.Errorf(
				"slot %q belongs to another typed relation declaration fragment",
				slot.String(),
			)
		}
		value, valueErr := NewSymbolValue(slot)
		if valueErr != nil {
			return valueErr
		}
		slotValues = append(slotValues, value)
	}
	slotSet, err := NewSetValue(slotValues)
	if err != nil {
		return err
	}
	expected, err := newDeclarationBody([]fieldInput{
		{name: "carrier_kind", value: NewTextValue("typed_relation_declaration_fragment")},
		{name: "relation_designator", value: NewTextValue(declaration.Symbol().Key())},
		{name: "subject_kind", value: NewTextValue(subject)},
		{name: "slots", value: slotSet},
		{name: "structural_check_scope", value: NewTextValue("local_structural_assertion_checks_only")},
	})
	if err != nil {
		return err
	}
	return requireExactDeclarationBody(declaration, expected)
}

func validateSlotDeclarationSchema(declaration LinkedDeclaration) error {
	if declaration.RuleID().String() != slotKindCompilerRule {
		return fmt.Errorf("SlotKind compiler rule is not %q", slotKindCompilerRule)
	}
	owner, slot, err := splitSlotSymbolKey(declaration.Symbol().Key())
	if err != nil {
		return err
	}
	valueKind, err := declarationSymbolField(declaration, "value_kind")
	if err != nil {
		return err
	}
	if valueKind.Kind() != typedmemory.KindSymbol {
		return fmt.Errorf("value_kind is not a kind symbol")
	}
	valueReference, err := NewSymbolValue(valueKind)
	if err != nil {
		return err
	}
	mode, err := declarationTextField(declaration, "reference_mode")
	if err != nil {
		return err
	}
	cardinality, err := declarationRecordField(declaration, "cardinality")
	if err != nil {
		return err
	}
	if err := validateExactCardinalityRecord(cardinality); err != nil {
		return err
	}
	fields := []fieldInput{
		{name: "governing_relation", value: NewTextValue(owner)},
		{name: "slot_kind", value: NewTextValue(slot)},
		{name: "value_kind", value: valueReference},
		{name: "reference_mode", value: NewTextValue(mode)},
		{name: "cardinality", value: cardinality},
	}
	if mode == "by_value" {
		expected, buildErr := newDeclarationBody(fields)
		if buildErr != nil {
			return buildErr
		}
		return requireExactDeclarationBody(declaration, expected)
	}
	if !strings.HasPrefix(mode, "by_reference:") {
		return fmt.Errorf("reference_mode %q is unknown", mode)
	}
	refKind, err := declarationSymbolField(declaration, "ref_kind")
	if err != nil {
		return err
	}
	if refKind.Kind() != typedmemory.RefKindSymbol {
		return fmt.Errorf("ref_kind is not a RefKind symbol")
	}
	wantRef := strings.TrimPrefix(mode, "by_reference:")
	if refKind.Key() != wantRef {
		return fmt.Errorf("reference_mode and ref_kind disagree")
	}
	refReference, err := NewSymbolValue(refKind)
	if err != nil {
		return err
	}
	fields = append(fields, fieldInput{name: "ref_kind", value: refReference})
	expected, err := newDeclarationBody(fields)
	if err != nil {
		return err
	}
	return requireExactDeclarationBody(declaration, expected)
}

func splitSlotSymbolKey(key string) (string, string, error) {
	parts := strings.Split(key, "/slot/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("SlotKind symbol key is not owner-qualified")
	}
	return parts[0], parts[1], nil
}

func validateExactCardinalityRecord(record RecordValue) error {
	fields := record.Fields()
	if len(fields) != 2 || fields[0].Name() != "maximum" || fields[1].Name() != "minimum" {
		return fmt.Errorf("cardinality must contain exactly maximum and minimum")
	}
	minimum, ok := fields[1].Value().(UnsignedValue)
	if !ok {
		return fmt.Errorf("cardinality minimum is not unsigned")
	}
	switch maximum := fields[0].Value().(type) {
	case UnsignedValue:
		if maximum.Value() < minimum.Value() {
			return fmt.Errorf("cardinality maximum precedes minimum")
		}
	case TextValue:
		if maximum.Value() != "unbounded" {
			return fmt.Errorf("cardinality maximum text is not unbounded")
		}
	default:
		return fmt.Errorf("cardinality maximum has unknown type")
	}
	return nil
}

func validateClaimGraphDeclarationSchema(
	artifact BaseTypeEnvArtifact,
	declaration LinkedDeclaration,
) error {
	if declaration.RuleID().String() != claimGraphRepresentationRule {
		return fmt.Errorf("ClaimGraph representation rule is not %q", claimGraphRepresentationRule)
	}
	valueSymbol, err := kindSymbol(claimGraphValueKindID)
	if err != nil {
		return err
	}
	shapeRef, err := newClaimGraphShapeRef()
	if err != nil {
		return err
	}
	budget := P6ClaimGraphDecodeBudget()
	codecRef, err := newClaimGraphCodecRef(shapeRef, budget)
	if err != nil {
		return err
	}
	bodies, err := claimGraphDeclarationBodies(
		valueSymbol,
		shapeRef,
		codecRef,
		budget,
		declaration.Basis().SourceLocations(),
	)
	if err != nil {
		return err
	}
	var expected DeclarationBody
	switch declaration.Symbol().Kind() {
	case typedmemory.ShapeSymbol:
		if declaration.Symbol().Key() != shapeRef.ID().String() {
			return fmt.Errorf("unsupported value-shape symbol")
		}
		expected = bodies.shape
	case typedmemory.CodecSymbol:
		if declaration.Symbol().Key() != codecRef.ID().String() {
			return fmt.Errorf("unsupported codec symbol")
		}
		expected = bodies.codec
	}
	if err := requireExactDeclarationBody(declaration, expected); err != nil {
		return err
	}
	ref, exists := artifact.TypeEnvRef()
	if !exists {
		return fmt.Errorf("compiled ClaimGraph representation has no TypeEnvRef")
	}
	_, _, err = lowerClaimGraphRepresentation(artifact, ref)
	return err
}

func requireExactDeclarationBody(
	declaration LinkedDeclaration,
	expected DeclarationBody,
) error {
	actualBytes := canonicalFields(
		declaration.Body().fields,
		"declaration-body-fields.v1",
	)
	expectedBytes := canonicalFields(
		expected.fields,
		"declaration-body-fields.v1",
	)
	if !bytes.Equal(actualBytes, expectedBytes) {
		return fmt.Errorf("declaration body does not match its exact executable schema")
	}
	return nil
}
