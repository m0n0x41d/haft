package typedmemory

import (
	"bytes"
	"fmt"
	"sort"
)

const classificationDisjointUseDomain = "admission-classification-disjoint-use.v1"

// ClassificationDisjointUse retains one direct false classification used to
// prove that the same reference filler does not satisfy another operand of an
// active KindDisjoint constraint. It is not an inferred false judgement.
type ClassificationDisjointUse struct {
	constraint ConstraintID
	judgement  FalseKindClassification
	canonical  []byte
	digest     SHA256Digest
}

func NewClassificationDisjointUse(
	constraint ConstraintID,
	judgement FalseKindClassification,
) (ClassificationDisjointUse, error) {
	if !constraint.valid() || !KindClassificationJudgementValid(judgement) {
		return ClassificationDisjointUse{}, fmt.Errorf(
			"classification disjoint use requires an exact constraint and false judgement",
		)
	}
	writer := newCanonicalWriter(classificationDisjointUseDomain)
	writer.addString(constraint.String())
	writer.addBytes(judgement.CanonicalBytes())
	return ClassificationDisjointUse{
		constraint: constraint,
		judgement:  judgement,
		canonical:  writer.bytes(),
		digest:     writer.digest(),
	}, nil
}

func (use ClassificationDisjointUse) Constraint() ConstraintID {
	return use.constraint
}

func (use ClassificationDisjointUse) Judgement() FalseKindClassification {
	return use.judgement
}

func (use ClassificationDisjointUse) CanonicalBytes() []byte {
	return append([]byte(nil), use.canonical...)
}

func (use ClassificationDisjointUse) Digest() SHA256Digest { return use.digest }

func (use ClassificationDisjointUse) valid() bool {
	rebuilt, err := NewClassificationDisjointUse(use.constraint, use.judgement)
	return err == nil && rebuilt.digest == use.digest &&
		bytes.Equal(rebuilt.canonical, use.canonical)
}

// ClassificationReferenceFillerAdmissionUse is the current C.3.2 admission
// certificate for one final relation filler. Its required result is a direct
// true classification; historical MemberOf evidence has a separate sealed
// carrier and cannot inhabit this interface.
type ClassificationReferenceFillerAdmissionUse interface {
	Coordinate() RelationFillerCoordinate
	Resolution() AdmissionReferenceResolution
	RequiredClassification() TrueKindClassification
	DisjointClassifications() []ClassificationDisjointUse
	CanonicalBytes() []byte
	Digest() SHA256Digest
	classificationReferenceFillerAdmissionUseVariant()
}

type ClassificationReferenceFillerAdmissionUseInput struct {
	TypeEnv                 TypeEnv
	Coordinate              RelationFillerCoordinate
	Resolution              AdmissionReferenceResolution
	RequiredClassification  TrueKindClassification
	DisjointClassifications []ClassificationDisjointUse
}

type classificationReferenceFillerAdmissionUse struct {
	coordinate RelationFillerCoordinate
	resolution AdmissionReferenceResolution
	required   TrueKindClassification
	disjoint   []ClassificationDisjointUse
	canonical  []byte
	digest     SHA256Digest
}

func NewClassificationReferenceFillerAdmissionUse(
	input ClassificationReferenceFillerAdmissionUseInput,
) (ClassificationReferenceFillerAdmissionUse, error) {
	if err := validateTypeEnv(input.TypeEnv); err != nil {
		return nil, fmt.Errorf(
			"classification reference-filler use requires a valid exact TypeEnv: %w",
			err,
		)
	}
	if !validRelationFillerCoordinate(input.Coordinate) {
		return nil, fmt.Errorf(
			"classification reference-filler use requires an exact final relation-filler coordinate",
		)
	}
	if !validAdmissionReferenceResolution(input.Resolution) {
		return nil, fmt.Errorf(
			"classification reference-filler use requires a defined reference resolution",
		)
	}
	if !KindClassificationJudgementValid(input.RequiredClassification) {
		return nil, fmt.Errorf(
			"classification reference-filler use requires a direct true classification",
		)
	}
	disjoint, err := normalizeClassificationDisjointUses(
		input.DisjointClassifications,
	)
	if err != nil {
		return nil, err
	}
	if err := validateClassificationReferenceFillerUse(
		input.TypeEnv,
		input.Coordinate,
		input.Resolution,
		input.RequiredClassification,
		disjoint,
	); err != nil {
		return nil, err
	}
	writer := canonicalClassificationReferenceFillerUse(
		input.Coordinate,
		input.Resolution,
		input.RequiredClassification,
		disjoint,
	)
	return classificationReferenceFillerAdmissionUse{
		coordinate: input.Coordinate,
		resolution: input.Resolution,
		required:   input.RequiredClassification,
		disjoint:   disjoint,
		canonical:  writer.bytes(),
		digest:     writer.digest(),
	}, nil
}

func (use classificationReferenceFillerAdmissionUse) Coordinate() RelationFillerCoordinate {
	return use.coordinate
}

func (use classificationReferenceFillerAdmissionUse) Resolution() AdmissionReferenceResolution {
	return use.resolution
}

func (use classificationReferenceFillerAdmissionUse) RequiredClassification() TrueKindClassification {
	return use.required
}

func (use classificationReferenceFillerAdmissionUse) DisjointClassifications() []ClassificationDisjointUse {
	return append([]ClassificationDisjointUse(nil), use.disjoint...)
}

func (use classificationReferenceFillerAdmissionUse) CanonicalBytes() []byte {
	return append([]byte(nil), use.canonical...)
}

func (use classificationReferenceFillerAdmissionUse) Digest() SHA256Digest {
	return use.digest
}

func (classificationReferenceFillerAdmissionUse) classificationReferenceFillerAdmissionUseVariant() {
}

func canonicalClassificationReferenceFillerUse(
	coordinate RelationFillerCoordinate,
	resolution AdmissionReferenceResolution,
	required TrueKindClassification,
	disjoint []ClassificationDisjointUse,
) canonicalWriter {
	writer := newCanonicalWriter(classificationReferenceFillerUseDomain)
	writer.addBytes(coordinate.CanonicalBytes())
	writer.addBytes(resolution.CanonicalBytes())
	writer.addBytes(required.CanonicalBytes())
	writer.addUint64(uint64(len(disjoint)))
	for _, use := range disjoint {
		writer.addBytes(use.CanonicalBytes())
	}
	return writer
}

func validateClassificationReferenceFillerUse(
	environment TypeEnv,
	coordinate RelationFillerCoordinate,
	resolution AdmissionReferenceResolution,
	required TrueKindClassification,
	disjoint []ClassificationDisjointUse,
) error {
	if coordinate.Reference() != resolution.PersistedReference() ||
		coordinate.Entity() != resolution.Entity() {
		return fmt.Errorf(
			"classification reference resolution does not identify the exact admitted relation filler",
		)
	}
	request := required.Request()
	if request.LocalKind().TypeEnv() != environment.Ref() ||
		request.LocalKind().ValueKind() != coordinate.RequiredValueKind() {
		return fmt.Errorf(
			"required classification local kind does not match the final slot ValueKind",
		)
	}
	if request.ContextSlice().Context() != resolution.Context() ||
		!sameContextSlice(request.ContextSlice(), coordinate.ContextSlice()) {
		return fmt.Errorf(
			"required classification does not use the resolved final-relation ContextSlice",
		)
	}
	if err := validateClassificationEntityCandidate(
		request,
		coordinate.Entity(),
	); err != nil {
		return err
	}
	signature, exists := environment.KindClassificationSignatureDefinition(
		request.LocalKind(),
	)
	if !exists || signature.Ref() != request.SignatureEdition() {
		return fmt.Errorf(
			"required classification does not use the exact current KindSignature",
		)
	}
	return validateClassificationDisjointUseSet(
		environment,
		coordinate,
		required,
		disjoint,
	)
}

func validateClassificationEntityCandidate(
	request KindClassificationRequest,
	entity EntityID,
) error {
	candidate, entityCandidate := request.Candidate().(ExactKindEntityCandidate)
	if !entityCandidate || candidate.EntityID() != entity {
		return fmt.Errorf(
			"required classification is not for the exact reference-filler entity candidate",
		)
	}
	return nil
}

func normalizeClassificationDisjointUses(
	values []ClassificationDisjointUse,
) ([]ClassificationDisjointUse, error) {
	result := append([]ClassificationDisjointUse(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		return classificationDisjointPosition(result[left]) <
			classificationDisjointPosition(result[right])
	})
	normalized := make([]ClassificationDisjointUse, 0, len(result))
	for _, use := range result {
		if !use.valid() {
			return nil, fmt.Errorf(
				"classification reference-filler use contains an invalid disjoint classification",
			)
		}
		if len(normalized) == 0 ||
			classificationDisjointPosition(normalized[len(normalized)-1]) !=
				classificationDisjointPosition(use) {
			normalized = append(normalized, use)
			continue
		}
		previous := normalized[len(normalized)-1]
		if bytes.Equal(previous.CanonicalBytes(), use.CanonicalBytes()) {
			continue
		}
		return nil, fmt.Errorf(
			"one disjoint classification position has conflicting direct false judgements",
		)
	}
	return normalized, nil
}

func classificationDisjointPosition(use ClassificationDisjointUse) string {
	return exactTupleKey(
		"classification-disjoint-position",
		use.Constraint().String(),
		use.Judgement().Request().LocalKind().ValueKind().ID().String(),
	)
}

func validateClassificationDisjointUseSet(
	environment TypeEnv,
	coordinate RelationFillerCoordinate,
	required TrueKindClassification,
	uses []ClassificationDisjointUse,
) error {
	expected, err := expectedClassificationDisjointPositions(
		environment,
		coordinate.RequiredValueKind().ID(),
	)
	if err != nil {
		return err
	}
	actual := make([]string, 0, len(uses))
	for _, use := range uses {
		request := use.Judgement().Request()
		if request.LocalKind().TypeEnv() != environment.Ref() ||
			request.LocalKind().Context() != required.Request().LocalKind().Context() ||
			!sameContextSlice(request.ContextSlice(), required.Request().ContextSlice()) {
			return fmt.Errorf(
				"disjoint false classification belongs to another TypeEnv, context, or slice",
			)
		}
		if err := validateClassificationEntityCandidate(
			request,
			coordinate.Entity(),
		); err != nil {
			return err
		}
		signature, exists := environment.KindClassificationSignatureDefinition(
			request.LocalKind(),
		)
		if !exists || signature.Ref() != request.SignatureEdition() {
			return fmt.Errorf(
				"disjoint false classification does not use the exact current KindSignature",
			)
		}
		actual = append(actual, classificationDisjointPosition(use))
	}
	sort.Strings(actual)
	if len(expected) != len(actual) {
		return fmt.Errorf(
			"disjoint classifications do not cover the exact expected counter-kind set",
		)
	}
	for index := range expected {
		if expected[index] != actual[index] {
			return fmt.Errorf(
				"disjoint classifications do not cover the exact expected counter-kind set",
			)
		}
	}
	return nil
}

func expectedClassificationDisjointPositions(
	environment TypeEnv,
	required KindID,
) ([]string, error) {
	positions := make([]string, 0)
	for _, rule := range environment.Constraints() {
		constraint, disjoint := rule.(KindDisjointConstraint)
		if !disjoint {
			continue
		}
		matched := make([]KindID, 0, 1)
		for _, operand := range constraint.Kinds() {
			if environment.IsSubkind(required, operand) {
				matched = append(matched, operand)
			}
		}
		if len(matched) > 1 {
			return nil, fmt.Errorf(
				"required ValueKind is below multiple operands of one disjoint constraint",
			)
		}
		if len(matched) == 0 {
			continue
		}
		for _, operand := range constraint.Kinds() {
			if operand == matched[0] {
				continue
			}
			positions = append(positions, exactTupleKey(
				"classification-disjoint-position",
				constraint.ID().String(),
				operand.String(),
			))
		}
	}
	sort.Strings(positions)
	return positions, nil
}

func validClassificationReferenceFillerAdmissionUse(
	use ClassificationReferenceFillerAdmissionUse,
) bool {
	value, supported := use.(classificationReferenceFillerAdmissionUse)
	if !supported {
		return false
	}
	disjoint, err := normalizeClassificationDisjointUses(value.disjoint)
	if err != nil || len(disjoint) != len(value.disjoint) {
		return false
	}
	if err := validateClassificationReferenceFillerUseStructure(
		value.coordinate,
		value.resolution,
		value.required,
		disjoint,
	); err != nil {
		return false
	}
	writer := canonicalClassificationReferenceFillerUse(
		value.coordinate,
		value.resolution,
		value.required,
		disjoint,
	)
	return validRelationFillerCoordinate(value.coordinate) &&
		validAdmissionReferenceResolution(value.resolution) &&
		KindClassificationJudgementValid(value.required) &&
		canonicalValueMatches(writer, value.canonical, value.digest)
}

func validateClassificationReferenceFillerUseStructure(
	coordinate RelationFillerCoordinate,
	resolution AdmissionReferenceResolution,
	required TrueKindClassification,
	disjoint []ClassificationDisjointUse,
) error {
	if coordinate.Reference() != resolution.PersistedReference() ||
		coordinate.Entity() != resolution.Entity() {
		return fmt.Errorf(
			"classification reference resolution does not identify the exact admitted relation filler",
		)
	}
	request := required.Request()
	if request.LocalKind().ValueKind() != coordinate.RequiredValueKind() ||
		request.LocalKind().TypeEnv() != coordinate.Reference().RefKind().TypeEnv() {
		return fmt.Errorf(
			"required classification local kind does not match the final slot ValueKind",
		)
	}
	if request.ContextSlice().Context() != resolution.Context() ||
		!sameContextSlice(request.ContextSlice(), coordinate.ContextSlice()) {
		return fmt.Errorf(
			"required classification does not use the resolved final-relation ContextSlice",
		)
	}
	if err := validateClassificationEntityCandidate(request, coordinate.Entity()); err != nil {
		return err
	}
	for _, use := range disjoint {
		counter := use.Judgement().Request()
		if counter.LocalKind().TypeEnv() != request.LocalKind().TypeEnv() ||
			counter.LocalKind().Context() != request.LocalKind().Context() ||
			!sameContextSlice(counter.ContextSlice(), request.ContextSlice()) {
			return fmt.Errorf(
				"disjoint false classification belongs to another TypeEnv, context, or slice",
			)
		}
		if err := validateClassificationEntityCandidate(counter, coordinate.Entity()); err != nil {
			return err
		}
	}
	return nil
}

func normalizeClassificationReferenceFillerAdmissionUses(
	values []ClassificationReferenceFillerAdmissionUse,
) ([]ClassificationReferenceFillerAdmissionUse, error) {
	result := append([]ClassificationReferenceFillerAdmissionUse(nil), values...)
	for _, use := range result {
		if !validClassificationReferenceFillerAdmissionUse(use) {
			return nil, fmt.Errorf(
				"context-slice classification basis contains an invalid reference-filler use",
			)
		}
	}
	sort.Slice(result, func(left, right int) bool {
		leftPosition := relationFillerPositionKey(result[left].Coordinate())
		rightPosition := relationFillerPositionKey(result[right].Coordinate())
		if leftPosition != rightPosition {
			return leftPosition < rightPosition
		}
		return bytes.Compare(
			result[left].CanonicalBytes(),
			result[right].CanonicalBytes(),
		) < 0
	})
	normalized := make([]ClassificationReferenceFillerAdmissionUse, 0, len(result))
	for _, use := range result {
		if len(normalized) == 0 ||
			relationFillerPositionKey(normalized[len(normalized)-1].Coordinate()) !=
				relationFillerPositionKey(use.Coordinate()) {
			normalized = append(normalized, use)
			continue
		}
		previous := normalized[len(normalized)-1]
		if !sameRelationFillerCoordinate(previous.Coordinate(), use.Coordinate()) {
			return nil, fmt.Errorf(
				"one classification relation-filler position has conflicting final coordinates",
			)
		}
		if bytes.Equal(previous.CanonicalBytes(), use.CanonicalBytes()) {
			continue
		}
		return nil, fmt.Errorf(
			"one classification relation-filler coordinate has conflicting admission evidence",
		)
	}
	return normalized, nil
}

type ContextSliceClassificationBasisInput struct {
	TypeEnv                                    TypeEnvRef
	GraphRevision                              GraphRevision
	Observations                               []AdmissionSnapshotObservation
	ClassificationReferenceFillerAdmissionUses []ClassificationReferenceFillerAdmissionUse
}

type contextSliceClassificationBasis struct {
	snapshot  admissionSnapshotBasis
	uses      []ClassificationReferenceFillerAdmissionUse
	canonical []byte
	digest    SHA256Digest
}

func NewContextSliceClassificationBasis(
	input ContextSliceClassificationBasisInput,
) (ContextSliceClassificationBasis, error) {
	snapshot, err := newAdmissionSnapshotBasis(
		input.TypeEnv,
		input.GraphRevision,
		input.Observations,
	)
	if err != nil {
		return nil, err
	}
	uses, err := normalizeClassificationReferenceFillerAdmissionUses(
		input.ClassificationReferenceFillerAdmissionUses,
	)
	if err != nil {
		return nil, err
	}
	if len(uses) == 0 {
		return nil, fmt.Errorf(
			"context-slice classification admission basis requires a reference-filler use",
		)
	}
	if err := validateClassificationAdmissionUseSet(
		input.TypeEnv,
		snapshot.observations,
		uses,
	); err != nil {
		return nil, err
	}
	writer := newCanonicalWriter(contextSliceClassificationBasisDomain)
	writer.addBytes(snapshot.canonical)
	writer.addUint64(uint64(len(uses)))
	for _, use := range uses {
		writer.addBytes(use.CanonicalBytes())
	}
	return contextSliceClassificationBasis{
		snapshot:  snapshot,
		uses:      uses,
		canonical: writer.bytes(),
		digest:    writer.digest(),
	}, nil
}

func (contextSliceClassificationBasis) Kind() AdmissionBasisKind {
	return ContextSliceClassificationAdmissionBasis
}

func (basis contextSliceClassificationBasis) TypeEnv() TypeEnvRef {
	return basis.snapshot.typeEnv
}

func (basis contextSliceClassificationBasis) GraphRevision() GraphRevision {
	return basis.snapshot.revision
}

func (basis contextSliceClassificationBasis) SnapshotObservations() []AdmissionSnapshotObservation {
	return append([]AdmissionSnapshotObservation(nil), basis.snapshot.observations...)
}

func (basis contextSliceClassificationBasis) ClassificationReferenceFillerAdmissionUses() []ClassificationReferenceFillerAdmissionUse {
	return append([]ClassificationReferenceFillerAdmissionUse(nil), basis.uses...)
}

func (basis contextSliceClassificationBasis) CanonicalBytes() []byte {
	return append([]byte(nil), basis.canonical...)
}

func (basis contextSliceClassificationBasis) Digest() SHA256Digest {
	return basis.digest
}

func (contextSliceClassificationBasis) admissionBasisVariant() {}

func (contextSliceClassificationBasis) contextSliceClassificationBasisVariant() {}

func validateClassificationAdmissionUseSet(
	typeEnv TypeEnvRef,
	observations []AdmissionSnapshotObservation,
	uses []ClassificationReferenceFillerAdmissionUse,
) error {
	for _, use := range uses {
		request := use.RequiredClassification().Request()
		if request.LocalKind().TypeEnv() != typeEnv ||
			use.Coordinate().Reference().RefKind().TypeEnv() != typeEnv {
			return fmt.Errorf(
				"classification reference-filler admission use belongs to another TypeEnv",
			)
		}
		if !hasSupportingAssertionAbsence(observations, use.Coordinate()) {
			return fmt.Errorf(
				"classification reference-filler use lacks the correlated assertion-absence observation",
			)
		}
		if _, snapshotResolution := use.Resolution().(snapshotReferenceResolution); snapshotResolution && hasEntityAbsenceAtSnapshot(
			observations,
			use.Resolution().Entity(),
			use.Resolution().Context(),
		) {
			return fmt.Errorf(
				"persisted classification reference contradicts entity absence in the same snapshot",
			)
		}
		if err := validateSameBatchDeclarationObservation(
			observations,
			use.Resolution(),
		); err != nil {
			return err
		}
	}
	return nil
}
