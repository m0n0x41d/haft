package typedmemorycandidatecodec

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	textShapeID                       = "Haft.Shape.TextV1"
	evidencePolarityShapeID           = "Haft.Shape.EvidencePolarityV1"
	canonicalInstantShapeID           = "Haft.Shape.CanonicalInstantV1"
	evidenceUseQualifierShapeID       = "Haft.Shape.EvidenceUseQualifierV1"
	completedPerformedIntervalShapeID = "Haft.Shape.CompletedPerformedIntervalV1"
	inFlightPerformedIntervalShapeID  = "Haft.Shape.InFlightPerformedIntervalV1"
	performedIntervalStateShapeID     = "Haft.Shape.PerformedIntervalStateV1"
	performedIntervalShapeID          = "Haft.Shape.PerformedIntervalV1"
	fileCodeAnchorTargetShapeID       = "Haft.Shape.FileCodeAnchorTargetV1"
	symbolCodeAnchorTargetShapeID     = "Haft.Shape.SymbolCodeAnchorTargetV1"
	codeAnchorTargetShapeID           = "Haft.Shape.CodeAnchorTargetV1"
	codeAnchorLocatorShapeID          = "Haft.Shape.CodeAnchorLocatorV1"
)

var candidateShapeIDs = []string{
	textShapeID,
	evidencePolarityShapeID,
	canonicalInstantShapeID,
	evidenceUseQualifierShapeID,
	completedPerformedIntervalShapeID,
	inFlightPerformedIntervalShapeID,
	performedIntervalStateShapeID,
	performedIntervalShapeID,
	fileCodeAnchorTargetShapeID,
	symbolCodeAnchorTargetShapeID,
	codeAnchorTargetShapeID,
	codeAnchorLocatorShapeID,
}

// ShapeSet is the sealed exact shape basis for the candidate suite. It can
// only be built from declarations whose content-derived references match the
// twelve structures in the candidate local-practice carrier.
type ShapeSet struct {
	text                       typedmemory.ValueShapeRef
	evidencePolarity           typedmemory.ValueShapeRef
	canonicalInstant           typedmemory.ValueShapeRef
	evidenceUseQualifier       typedmemory.ValueShapeRef
	completedPerformedInterval typedmemory.ValueShapeRef
	inFlightPerformedInterval  typedmemory.ValueShapeRef
	performedIntervalState     typedmemory.ValueShapeRef
	performedInterval          typedmemory.ValueShapeRef
	fileCodeAnchorTarget       typedmemory.ValueShapeRef
	symbolCodeAnchorTarget     typedmemory.ValueShapeRef
	codeAnchorTarget           typedmemory.ValueShapeRef
	codeAnchorLocator          typedmemory.ValueShapeRef
}

func NewShapeSet(
	declarations []typedmemory.ValueShapeDeclaration,
) (ShapeSet, error) {
	indexed, err := indexCandidateShapeDeclarations(declarations)
	if err != nil {
		return ShapeSet{}, err
	}
	refs := make(map[string]typedmemory.ValueShapeRef, len(indexed))
	for _, id := range candidateShapeIDs {
		declaration, exists := indexed[id]
		if !exists {
			return ShapeSet{}, fmt.Errorf("candidate ValueShape %q is missing", id)
		}
		if err := typedmemory.VerifyValueShapeRef(
			declaration.Ref(),
			declaration.Shape(),
		); err != nil {
			return ShapeSet{}, fmt.Errorf("candidate ValueShape %q: %w", id, err)
		}
		refs[id] = declaration.Ref()
	}
	shapes := ShapeSet{
		text:                       refs[textShapeID],
		evidencePolarity:           refs[evidencePolarityShapeID],
		canonicalInstant:           refs[canonicalInstantShapeID],
		evidenceUseQualifier:       refs[evidenceUseQualifierShapeID],
		completedPerformedInterval: refs[completedPerformedIntervalShapeID],
		inFlightPerformedInterval:  refs[inFlightPerformedIntervalShapeID],
		performedIntervalState:     refs[performedIntervalStateShapeID],
		performedInterval:          refs[performedIntervalShapeID],
		fileCodeAnchorTarget:       refs[fileCodeAnchorTargetShapeID],
		symbolCodeAnchorTarget:     refs[symbolCodeAnchorTargetShapeID],
		codeAnchorTarget:           refs[codeAnchorTargetShapeID],
		codeAnchorLocator:          refs[codeAnchorLocatorShapeID],
	}
	if err := shapes.verifyExactStructures(); err != nil {
		return ShapeSet{}, err
	}
	return shapes, nil
}

func indexCandidateShapeDeclarations(
	declarations []typedmemory.ValueShapeDeclaration,
) (map[string]typedmemory.ValueShapeDeclaration, error) {
	required := make(map[string]struct{}, len(candidateShapeIDs))
	for _, id := range candidateShapeIDs {
		required[id] = struct{}{}
	}
	indexed := make(map[string]typedmemory.ValueShapeDeclaration, len(required))
	for _, declaration := range declarations {
		id := declaration.Ref().ID().String()
		if _, relevant := required[id]; !relevant {
			continue
		}
		if _, duplicate := indexed[id]; duplicate {
			return nil, fmt.Errorf("candidate ValueShape %q is duplicated", id)
		}
		indexed[id] = declaration
	}
	return indexed, nil
}

func (shapes ShapeSet) verifyExactStructures() error {
	checks := []struct {
		ref   typedmemory.ValueShapeRef
		shape typedmemory.ValueShape
	}{
		{ref: shapes.text, shape: mustScalarShape(typedmemory.ScalarText)},
		{ref: shapes.evidencePolarity, shape: mustScalarShape(typedmemory.ScalarText)},
		{ref: shapes.canonicalInstant, shape: mustScalarShape(typedmemory.ScalarText)},
		{ref: shapes.evidenceUseQualifier, shape: mustRecordShape([]shapeField{
			{name: "polarity", ref: shapes.evidencePolarity},
		})},
		{ref: shapes.completedPerformedInterval, shape: mustRecordShape([]shapeField{
			{name: "start", ref: shapes.canonicalInstant},
			{name: "end", ref: shapes.canonicalInstant},
		})},
		{ref: shapes.inFlightPerformedInterval, shape: mustRecordShape([]shapeField{
			{name: "start", ref: shapes.canonicalInstant},
		})},
		{ref: shapes.performedIntervalState, shape: mustSumShape([]shapeField{
			{name: "Completed", ref: shapes.completedPerformedInterval},
			{name: "InFlight", ref: shapes.inFlightPerformedInterval},
		})},
		{ref: shapes.performedInterval, shape: mustRecordShape([]shapeField{
			{name: "state", ref: shapes.performedIntervalState},
		})},
		{ref: shapes.fileCodeAnchorTarget, shape: mustRecordShape([]shapeField{
			{name: "path", ref: shapes.text},
		})},
		{ref: shapes.symbolCodeAnchorTarget, shape: mustRecordShape([]shapeField{
			{name: "path", ref: shapes.text},
			{name: "symbol", ref: shapes.text},
		})},
		{ref: shapes.codeAnchorTarget, shape: mustSumShape([]shapeField{
			{name: "File", ref: shapes.fileCodeAnchorTarget},
			{name: "Symbol", ref: shapes.symbolCodeAnchorTarget},
		})},
		{ref: shapes.codeAnchorLocator, shape: mustRecordShape([]shapeField{
			{name: "repository", ref: shapes.text},
			{name: "revision", ref: shapes.text},
			{name: "target", ref: shapes.codeAnchorTarget},
		})},
	}
	for _, check := range checks {
		if err := typedmemory.VerifyValueShapeRef(check.ref, check.shape); err != nil {
			return fmt.Errorf("candidate shape %q does not match its exact contract: %w", check.ref.ID().String(), err)
		}
	}
	return nil
}

type shapeField struct {
	name string
	ref  typedmemory.ValueShapeRef
}

func mustScalarShape(kind typedmemory.ScalarKind) typedmemory.ValueShape {
	shape, _ := typedmemory.NewScalarShape(kind)
	return shape
}

func mustRecordShape(fields []shapeField) typedmemory.ValueShape {
	members := make([]typedmemory.RecordFieldShape, 0, len(fields))
	for _, field := range fields {
		name, _ := typedmemory.NewValueMemberName(field.name)
		member, _ := typedmemory.NewRecordFieldShape(name, field.ref)
		members = append(members, member)
	}
	shape, _ := typedmemory.NewRecordShape(members)
	return shape
}

func mustSumShape(variants []shapeField) typedmemory.ValueShape {
	members := make([]typedmemory.SumVariantShape, 0, len(variants))
	for _, variant := range variants {
		name, _ := typedmemory.NewValueMemberName(variant.name)
		member, _ := typedmemory.NewSumVariantShape(name, variant.ref)
		members = append(members, member)
	}
	shape, _ := typedmemory.NewSumShape(members)
	return shape
}

func (shapes ShapeSet) Text() typedmemory.ValueShapeRef { return shapes.text }

func (shapes ShapeSet) EvidencePolarity() typedmemory.ValueShapeRef {
	return shapes.evidencePolarity
}

func (shapes ShapeSet) CanonicalInstant() typedmemory.ValueShapeRef {
	return shapes.canonicalInstant
}

func (shapes ShapeSet) EvidenceUseQualifier() typedmemory.ValueShapeRef {
	return shapes.evidenceUseQualifier
}

func (shapes ShapeSet) PerformedInterval() typedmemory.ValueShapeRef {
	return shapes.performedInterval
}

func (shapes ShapeSet) CodeAnchorLocator() typedmemory.ValueShapeRef {
	return shapes.codeAnchorLocator
}
