package typedmemory

import (
	"reflect"
	"testing"
)

func TestClassificationAdmissionBasisCarriesOnlyDirectCurrentJudgements(t *testing.T) {
	required := newKindClassificationFixture(
		t,
		"ctx:haft",
		"U.Entity",
		SignatureF4,
		true,
	)
	counter := newKindClassificationFixture(
		t,
		"ctx:haft",
		"U.ClaimGraph",
		SignatureF4,
		true,
	)
	environment, err := required.environment.builderWithoutBridge().
		AddKindClassificationSignatureDefinition(required.signature).
		AddKindClassificationSignatureDefinition(counter.signature).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	assertion := admissionTestAssertionID(
		t,
		"assertion:current-classification/vehicle-7",
	)
	coordinate, reference := classificationAdmissionTestCoordinate(
		t,
		required,
		environment,
		assertion,
		3,
		0,
	)
	resolution := admissionTestSnapshotReferenceResolution(
		t,
		reference,
		required.candidate.EntityID(),
		required.contextSlice.Context(),
		"snapshot:current-classification/vehicle-7",
	)
	requiredJudgement, err := NewTrueKindClassification(
		required.request,
		required.basis,
	)
	if err != nil {
		t.Fatalf("NewTrueKindClassification() error = %v", err)
	}
	counterJudgement, err := NewFalseKindClassification(
		counter.request,
		counter.basis,
	)
	if err != nil {
		t.Fatalf("NewFalseKindClassification() error = %v", err)
	}
	constraint := required.environment.constraint.(KindDisjointConstraint)
	disjoint, err := NewClassificationDisjointUse(
		constraint.ID(),
		counterJudgement,
	)
	if err != nil {
		t.Fatalf("NewClassificationDisjointUse() error = %v", err)
	}
	use, err := NewClassificationReferenceFillerAdmissionUse(
		ClassificationReferenceFillerAdmissionUseInput{
			TypeEnv:                 environment,
			Coordinate:              coordinate,
			Resolution:              resolution,
			RequiredClassification:  requiredJudgement,
			DisjointClassifications: []ClassificationDisjointUse{disjoint, disjoint},
		},
	)
	if err != nil {
		t.Fatalf("NewClassificationReferenceFillerAdmissionUse() error = %v", err)
	}
	if !validClassificationReferenceFillerAdmissionUse(use) ||
		len(use.DisjointClassifications()) != 1 {
		t.Fatal("current classification use was not sealed and normalized")
	}

	observation := admissionTestAssertionAbsentObservation(t, 3, assertion)
	basis, err := NewContextSliceClassificationBasis(
		ContextSliceClassificationBasisInput{
			TypeEnv:       environment.Ref(),
			GraphRevision: NewGraphRevision(0),
			Observations:  []AdmissionSnapshotObservation{observation},
			ClassificationReferenceFillerAdmissionUses: []ClassificationReferenceFillerAdmissionUse{
				use,
				use,
			},
		},
	)
	if err != nil {
		t.Fatalf("NewContextSliceClassificationBasis() error = %v", err)
	}
	if basis.Kind() != ContextSliceClassificationAdmissionBasis ||
		len(basis.ClassificationReferenceFillerAdmissionUses()) != 1 ||
		!validAdmissionBasis(basis) {
		t.Fatal("current classification AdmissionBasis was not sealed and normalized")
	}
	if _, historical := basis.(ContextSliceMembershipBasis); historical {
		t.Fatal("current classification basis also satisfies the historical membership variant")
	}
	if err := VerifyStoredAdmissionBasisCoordinates(
		basis.Kind(),
		environment.Ref(),
		NewGraphRevision(0),
		basis.CanonicalBytes(),
	); err != nil {
		t.Fatalf("VerifyStoredAdmissionBasisCoordinates() error = %v", err)
	}

	uses := basis.ClassificationReferenceFillerAdmissionUses()
	uses[0] = nil
	if basis.ClassificationReferenceFillerAdmissionUses()[0] == nil {
		t.Fatal("classification basis exposed mutable admission-use storage")
	}
	disjointCopy := use.DisjointClassifications()
	disjointCopy[0] = ClassificationDisjointUse{}
	if !use.DisjointClassifications()[0].valid() {
		t.Fatal("classification use exposed mutable disjoint-use storage")
	}
}

func TestClassificationAdmissionUseRejectsMismatchedOrDamagedProofs(t *testing.T) {
	required := newKindClassificationFixture(
		t,
		"ctx:haft",
		"U.Entity",
		SignatureF4,
		true,
	)
	counter := newKindClassificationFixture(
		t,
		"ctx:haft",
		"U.ClaimGraph",
		SignatureF4,
		true,
	)
	environment, err := required.environment.builderWithoutBridge().
		AddKindClassificationSignatureDefinition(required.signature).
		AddKindClassificationSignatureDefinition(counter.signature).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	assertion := admissionTestAssertionID(
		t,
		"assertion:current-classification/damage-test",
	)
	coordinate, reference := classificationAdmissionTestCoordinate(
		t,
		required,
		environment,
		assertion,
		5,
		0,
	)
	resolution := admissionTestSnapshotReferenceResolution(
		t,
		reference,
		required.candidate.EntityID(),
		required.contextSlice.Context(),
		"snapshot:current-classification/damage-test",
	)
	requiredJudgement, err := NewTrueKindClassification(
		required.request,
		required.basis,
	)
	if err != nil {
		t.Fatalf("NewTrueKindClassification() error = %v", err)
	}
	counterJudgement, err := NewFalseKindClassification(
		counter.request,
		counter.basis,
	)
	if err != nil {
		t.Fatalf("NewFalseKindClassification() error = %v", err)
	}
	constraint := required.environment.constraint.(KindDisjointConstraint)
	disjoint, err := NewClassificationDisjointUse(
		constraint.ID(),
		counterJudgement,
	)
	if err != nil {
		t.Fatalf("NewClassificationDisjointUse() error = %v", err)
	}
	use, err := NewClassificationReferenceFillerAdmissionUse(
		ClassificationReferenceFillerAdmissionUseInput{
			TypeEnv:                 environment,
			Coordinate:              coordinate,
			Resolution:              resolution,
			RequiredClassification:  requiredJudgement,
			DisjointClassifications: []ClassificationDisjointUse{disjoint},
		},
	)
	if err != nil {
		t.Fatalf("NewClassificationReferenceFillerAdmissionUse() error = %v", err)
	}

	otherReferenceID, err := NewReferenceID("entity:vehicle-7/other")
	if err != nil {
		t.Fatalf("NewReferenceID() error = %v", err)
	}
	otherReference, err := NewPersistedRef(
		required.environment.entityRefKind,
		otherReferenceID,
	)
	if err != nil {
		t.Fatalf("NewPersistedRef() error = %v", err)
	}
	otherResolution := admissionTestSnapshotReferenceResolution(
		t,
		otherReference,
		required.candidate.EntityID(),
		required.contextSlice.Context(),
		"snapshot:current-classification/other",
	)
	_, err = NewClassificationReferenceFillerAdmissionUse(
		ClassificationReferenceFillerAdmissionUseInput{
			TypeEnv:                 environment,
			Coordinate:              coordinate,
			Resolution:              otherResolution,
			RequiredClassification:  requiredJudgement,
			DisjointClassifications: []ClassificationDisjointUse{disjoint},
		},
	)
	if err == nil {
		t.Fatal("classification use accepted a resolution for another reference")
	}

	damaged := use.(classificationReferenceFillerAdmissionUse)
	damaged.resolution = otherResolution
	if validClassificationReferenceFillerAdmissionUse(damaged) {
		t.Fatal("classification use accepted internally damaged reference correlation")
	}

	withoutCounterSignature, err := required.environment.builderWithoutBridge().
		AddKindClassificationSignatureDefinition(required.signature).
		Build()
	if err != nil {
		t.Fatalf("Build(without counter signature) error = %v", err)
	}
	_, err = NewClassificationReferenceFillerAdmissionUse(
		ClassificationReferenceFillerAdmissionUseInput{
			TypeEnv:                 withoutCounterSignature,
			Coordinate:              coordinate,
			Resolution:              resolution,
			RequiredClassification:  requiredJudgement,
			DisjointClassifications: []ClassificationDisjointUse{disjoint},
		},
	)
	if err == nil {
		t.Fatal("classification use accepted a false counter with no exact current signature")
	}
}

func TestClassificationAdmissionCarrierCannotEncodeHistoricalMembership(t *testing.T) {
	inputType := reflect.TypeOf(ClassificationReferenceFillerAdmissionUseInput{})
	requiredField, exists := inputType.FieldByName("RequiredClassification")
	if !exists || requiredField.Type != reflect.TypeOf(TrueKindClassification{}) {
		t.Fatal("current admission input is not statically restricted to direct true classification")
	}
	if requiredField.Type == reflect.TypeOf(MemberOfMember{}) {
		t.Fatal("historical MemberOf can inhabit the current classification carrier")
	}
	if SnapshotOnlyAdmissionBasis != AdmissionBasisKind(1) ||
		ContextSliceMembershipAdmissionBasis != AdmissionBasisKind(2) ||
		ContextSliceClassificationAdmissionBasis != AdmissionBasisKind(3) {
		t.Fatal("adding current classification changed a historical AdmissionBasisKind value")
	}
	if contextSliceMembershipAdmissionBasisDomain == contextSliceClassificationBasisDomain {
		t.Fatal("current and historical admission bases share a canonical domain")
	}

	legacy := newMemberOfFixture(t)
	member := admissionTestMember(t, legacy)
	if _, current := any(member).(TrueKindClassification); current {
		t.Fatal("historical membership can inhabit the current classification result type")
	}
}

func classificationAdmissionTestCoordinate(
	t *testing.T,
	fixture kindClassificationFixture,
	environment TypeEnv,
	assertion AssertionID,
	changeOrdinal uint64,
	fillerOrdinal uint64,
) (RelationFillerCoordinate, PersistedRef) {
	t.Helper()
	entity := fixture.candidate.EntityID()
	reference := admissionTestPersistedRef(
		t,
		fixture.environment.entityRefKind,
		entity,
	)
	filler := newReferenceFiller(reference, entity)
	candidateFiller, err := NewByReferenceCandidate(reference)
	if err != nil {
		t.Fatalf("NewByReferenceCandidate() error = %v", err)
	}
	candidateBinding, err := NewCandidateSlotBinding(
		fixture.environment.entitySlot,
		[]CandidateSlotFiller{candidateFiller},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding() error = %v", err)
	}
	candidate, err := NewRelationInstantiation(
		assertion,
		fixture.environment.signature.Ref(),
		fixture.contextSlice,
		[]CandidateSlotBinding{candidateBinding},
		memberOfTestProvenanceRef(t, "prov:test/current-classification-relation"),
	)
	if err != nil {
		t.Fatalf("NewRelationInstantiation() error = %v", err)
	}
	binding := newSlotBinding(
		fixture.environment.entitySlot,
		[]SlotFiller{filler},
	)
	relation := newRelationInstance(candidate, []SlotBinding{binding})
	coordinate, err := NewRelationFillerCoordinate(RelationFillerCoordinateInput{
		TypeEnv:       environment,
		ChangeOrdinal: changeOrdinal,
		Relation:      relation,
		Slot:          fixture.environment.entitySlot,
		FillerOrdinal: fillerOrdinal,
	})
	if err != nil {
		t.Fatalf("NewRelationFillerCoordinate() error = %v", err)
	}
	return coordinate, reference
}
