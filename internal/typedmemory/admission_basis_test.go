package typedmemory

import (
	"bytes"
	"testing"
	"time"
)

func TestSnapshotOnlyBasisIsSealedNormalizedAndDefensive(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	entity := mustEntityID(t, "entity:authorization-service")
	context := fixture.primaryContext.Ref()
	entityBasis := mustResolutionBasisRef(t, "snapshot:entity-absence")
	aliasBasis := mustResolutionBasisRef(t, "snapshot:alias-availability")
	absent, err := NewAbsentEntityResolution(entity, context, entityBasis)
	if err != nil {
		t.Fatalf("NewAbsentEntityResolution() error = %v", err)
	}
	alias := mustEntityAlias(t, "authorization-service")
	unbound, err := NewUnboundAliasResolution(alias, context, aliasBasis)
	if err != nil {
		t.Fatalf("NewUnboundAliasResolution() error = %v", err)
	}
	entityObservation, err := NewEntityAbsentObservation(0, absent)
	if err != nil {
		t.Fatalf("NewEntityAbsentObservation() error = %v", err)
	}
	aliasObservation, err := NewAliasUnboundObservation(1, unbound)
	if err != nil {
		t.Fatalf("NewAliasUnboundObservation() error = %v", err)
	}

	forward := admissionTestSnapshotOnlyBasis(
		t,
		fixture.ref,
		NewGraphRevision(19),
		[]AdmissionSnapshotObservation{
			entityObservation,
			aliasObservation,
			entityObservation,
		},
	)
	reverse := admissionTestSnapshotOnlyBasis(
		t,
		fixture.ref,
		NewGraphRevision(19),
		[]AdmissionSnapshotObservation{aliasObservation, entityObservation},
	)
	if forward.Digest() != reverse.Digest() ||
		!bytes.Equal(forward.CanonicalBytes(), reverse.CanonicalBytes()) {
		t.Fatal("observation order or exact duplication changed snapshot-only basis identity")
	}
	if forward.Kind() != SnapshotOnlyAdmissionBasis ||
		forward.TypeEnv() != fixture.ref ||
		forward.GraphRevision() != NewGraphRevision(19) {
		t.Fatal("snapshot-only basis lost its exact common snapshot coordinates")
	}
	if len(forward.SnapshotObservations()) != 2 {
		t.Fatalf("normalized observation count = %d; want 2", len(forward.SnapshotObservations()))
	}
	if !validAdmissionBasis(forward) {
		t.Fatal("constructor produced an invalid sealed AdmissionBasis")
	}
	if _, membership := forward.(ContextSliceMembershipBasis); membership {
		t.Fatal("snapshot-only basis also satisfied the membership-bearing variant")
	}

	canonical := forward.CanonicalBytes()
	canonical[0] ^= 0xff
	if bytes.Equal(canonical, forward.CanonicalBytes()) {
		t.Fatal("CanonicalBytes exposed mutable admission-basis storage")
	}
	observations := forward.SnapshotObservations()
	observations[0] = nil
	if forward.SnapshotObservations()[0] == nil {
		t.Fatal("SnapshotObservations exposed mutable admission-basis storage")
	}

	_, err = NewSnapshotOnlyBasis(SnapshotOnlyBasisInput{
		TypeEnv:       fixture.ref,
		GraphRevision: NewGraphRevision(19),
	})
	if err == nil {
		t.Fatal("snapshot-only basis accepted zero positive observations")
	}
	_, err = NewSnapshotOnlyBasis(SnapshotOnlyBasisInput{
		GraphRevision: NewGraphRevision(19),
		Observations:  []AdmissionSnapshotObservation{entityObservation},
	})
	if err == nil {
		t.Fatal("snapshot-only basis accepted a missing TypeEnv")
	}
	_, err = NewEntityAbsentObservation(0, AbsentEntityResolution{})
	if err == nil {
		t.Fatal("positive observation constructor accepted an unresolved/zero outcome")
	}
	exact, err := NewExactEntityResolution(
		entity,
		context,
		mustResolutionBasisRef(t, "snapshot:entity-exact"),
	)
	if err != nil {
		t.Fatalf("NewExactEntityResolution() error = %v", err)
	}
	exactObservation, err := NewEntityExactObservation(0, exact)
	if err != nil {
		t.Fatalf("NewEntityExactObservation() error = %v", err)
	}
	_, err = NewSnapshotOnlyBasis(SnapshotOnlyBasisInput{
		TypeEnv:       fixture.ref,
		GraphRevision: NewGraphRevision(19),
		Observations: []AdmissionSnapshotObservation{
			entityObservation,
			exactObservation,
		},
	})
	if err == nil {
		t.Fatal("snapshot basis accepted mutually exclusive facts for one entity observation position")
	}
	var absentBasis AdmissionBasis
	if validAdmissionBasis(absentBasis) {
		t.Fatal("nil AdmissionBasis was accepted")
	}
}

func TestReferenceFillerAdmissionUseRequiresCorrelatedDefinedMembership(t *testing.T) {
	fixture := newMemberOfFixture(t)
	assertion := admissionTestAssertionID(t, "assertion:episteme-about-vehicle-7")
	coordinate, reference := admissionTestCoordinate(t, fixture, assertion, 0, 0)
	resolution := admissionTestSnapshotReferenceResolution(
		t,
		reference,
		fixture.query.EntityID(),
		fixture.query.ContextSlice().Context(),
		"snapshot:reference-resolution-index",
	)
	required := admissionTestMember(t, fixture)
	disjoint := admissionTestDisjointNotMember(t, fixture)

	use, err := NewReferenceFillerAdmissionUse(ReferenceFillerAdmissionUseInput{
		TypeEnv:             fixture.typeEnv.build(t),
		Coordinate:          coordinate,
		Resolution:          resolution,
		RequiredMembership:  required,
		DisjointMemberships: []DisjointCounterUse{disjoint, disjoint},
	})
	if err != nil {
		t.Fatalf("NewReferenceFillerAdmissionUse() error = %v", err)
	}
	if !validReferenceFillerAdmissionUse(use) {
		t.Fatal("constructor produced an invalid reference-filler admission use")
	}
	if len(use.DisjointMemberships()) != 1 {
		t.Fatalf("normalized disjoint use count = %d; want 1", len(use.DisjointMemberships()))
	}
	disjointCopy := use.DisjointMemberships()
	disjointCopy[0] = nil
	if use.DisjointMemberships()[0] == nil {
		t.Fatal("DisjointMemberships exposed mutable admission-use storage")
	}

	otherEntityQuery := fixture.queryForEntity(t, "entity:vehicle-8")
	otherEntityBasis := fixture.basisForQuery(
		t,
		otherEntityQuery,
		[]MemberOfObservableInput{
			memberOfTestObservableInput(t, "observable:registry/vehicle-8", 0xc1),
		},
	)
	otherMember, err := NewMemberOfMember(otherEntityQuery, otherEntityBasis)
	if err != nil {
		t.Fatalf("NewMemberOfMember(other entity) error = %v", err)
	}
	_, err = NewReferenceFillerAdmissionUse(ReferenceFillerAdmissionUseInput{
		TypeEnv:            fixture.typeEnv.build(t),
		Coordinate:         coordinate,
		Resolution:         resolution,
		RequiredMembership: otherMember,
	})
	if err == nil {
		t.Fatal("reference-filler use accepted MemberOf evidence for another entity")
	}

	_, err = NewReferenceFillerAdmissionUse(ReferenceFillerAdmissionUseInput{
		TypeEnv:            fixture.typeEnv.build(t),
		Coordinate:         coordinate,
		Resolution:         resolution,
		RequiredMembership: MemberOfMember{},
	})
	if err == nil {
		t.Fatal("reference-filler use accepted an undefined/zero positive membership")
	}
	wrongKindMember, err := NewMemberOfMember(
		disjoint.Judgement().Query(),
		disjoint.Judgement().Basis(),
	)
	if err != nil {
		t.Fatalf("NewMemberOfMember(wrong slot kind) error = %v", err)
	}
	_, err = NewReferenceFillerAdmissionUse(ReferenceFillerAdmissionUseInput{
		TypeEnv:            fixture.typeEnv.build(t),
		Coordinate:         coordinate,
		Resolution:         resolution,
		RequiredMembership: wrongKindMember,
	})
	if err == nil {
		t.Fatal("reference-filler use accepted MemberOf evidence for another slot ValueKind")
	}
	laterSlice := memberOfTestContextSlice(
		t,
		fixture.query.ContextSlice().Context(),
		time.Date(2026, time.July, 17, 9, 0, 0, 0, time.UTC),
	)
	laterQuery, err := NewMemberOfQuery(
		fixture.query.EntityID(),
		fixture.query.ValueKind(),
		laterSlice,
	)
	if err != nil {
		t.Fatalf("NewMemberOfQuery(later slice) error = %v", err)
	}
	laterBasis := fixture.basisForQuery(
		t,
		laterQuery,
		[]MemberOfObservableInput{
			memberOfTestObservableInput(t, "observable:registry/vehicle-7/later", 0xc4),
		},
	)
	laterMember, err := NewMemberOfMember(laterQuery, laterBasis)
	if err != nil {
		t.Fatalf("NewMemberOfMember(later slice) error = %v", err)
	}
	_, err = NewReferenceFillerAdmissionUse(ReferenceFillerAdmissionUseInput{
		TypeEnv:            fixture.typeEnv.build(t),
		Coordinate:         coordinate,
		Resolution:         resolution,
		RequiredMembership: laterMember,
	})
	if err == nil {
		t.Fatal("reference-filler use accepted another full ContextSlice with the same bounded context")
	}
}

func TestContextSliceMembershipBasisRequiresUsesAndExactCoordinateCoverage(t *testing.T) {
	fixture := newMemberOfFixture(t)
	assertion := admissionTestAssertionID(t, "assertion:episteme-about-vehicle-7")
	coordinate, reference := admissionTestCoordinate(t, fixture, assertion, 3, 0)
	resolution := admissionTestSnapshotReferenceResolution(
		t,
		reference,
		fixture.query.EntityID(),
		fixture.query.ContextSlice().Context(),
		"snapshot:reference-resolution-index",
	)
	use := admissionTestReferenceUse(t, fixture, coordinate, resolution, admissionTestMember(t, fixture))
	observation := admissionTestAssertionAbsentObservation(t, 3, assertion)

	basis, err := NewContextSliceMembershipBasis(ContextSliceMembershipBasisInput{
		TypeEnv:                      fixture.typeEnv.ref,
		GraphRevision:                NewGraphRevision(42),
		Observations:                 []AdmissionSnapshotObservation{observation},
		ReferenceFillerAdmissionUses: []ReferenceFillerAdmissionUse{use, use},
	})
	if err != nil {
		t.Fatalf("NewContextSliceMembershipBasis() error = %v", err)
	}
	if basis.Kind() != ContextSliceMembershipAdmissionBasis ||
		len(basis.ReferenceFillerAdmissionUses()) != 1 ||
		!validAdmissionBasis(basis) {
		t.Fatal("membership-bearing AdmissionBasis was not sealed and normalized")
	}
	uses := basis.ReferenceFillerAdmissionUses()
	uses[0] = nil
	if basis.ReferenceFillerAdmissionUses()[0] == nil {
		t.Fatal("ReferenceFillerAdmissionUses exposed mutable basis storage")
	}

	_, err = NewContextSliceMembershipBasis(ContextSliceMembershipBasisInput{
		TypeEnv:       fixture.typeEnv.ref,
		GraphRevision: NewGraphRevision(42),
		Observations:  []AdmissionSnapshotObservation{observation},
	})
	if err == nil {
		t.Fatal("context-slice membership basis accepted zero reference-filler uses")
	}
	otherAssertion := admissionTestAssertionID(t, "assertion:another")
	wrongObservation := admissionTestAssertionAbsentObservation(t, 3, otherAssertion)
	_, err = NewContextSliceMembershipBasis(ContextSliceMembershipBasisInput{
		TypeEnv:                      fixture.typeEnv.ref,
		GraphRevision:                NewGraphRevision(42),
		Observations:                 []AdmissionSnapshotObservation{wrongObservation},
		ReferenceFillerAdmissionUses: []ReferenceFillerAdmissionUse{use},
	})
	if err == nil {
		t.Fatal("membership basis accepted a filler use without matching assertion-absence evidence")
	}

	alternativeResolution := admissionTestSnapshotReferenceResolution(
		t,
		reference,
		fixture.query.EntityID(),
		fixture.query.ContextSlice().Context(),
		"snapshot:alternative-resolution-index",
	)
	alternativeUse := admissionTestReferenceUse(
		t,
		fixture,
		coordinate,
		alternativeResolution,
		admissionTestMember(t, fixture),
	)
	_, err = NewContextSliceMembershipBasis(ContextSliceMembershipBasisInput{
		TypeEnv:                      fixture.typeEnv.ref,
		GraphRevision:                NewGraphRevision(42),
		Observations:                 []AdmissionSnapshotObservation{observation},
		ReferenceFillerAdmissionUses: []ReferenceFillerAdmissionUse{use, alternativeUse},
	})
	if err == nil {
		t.Fatal("one relation-filler coordinate accepted conflicting admission evidence")
	}
	alternateReferenceID, err := NewReferenceID("entity:vehicle-7/alternate-reference")
	if err != nil {
		t.Fatalf("NewReferenceID(alternate) error = %v", err)
	}
	alternateReference, err := NewPersistedRef(
		fixture.typeEnv.entityRefKind,
		alternateReferenceID,
	)
	if err != nil {
		t.Fatalf("NewPersistedRef(alternate) error = %v", err)
	}
	alternateCoordinate, _ := admissionTestCoordinateForReference(
		t,
		fixture,
		assertion,
		alternateReference,
		fixture.query.EntityID(),
		3,
		0,
	)
	alternateFillerResolution := admissionTestSnapshotReferenceResolution(
		t,
		alternateReference,
		fixture.query.EntityID(),
		fixture.query.ContextSlice().Context(),
		"snapshot:alternate-filler-resolution",
	)
	alternateFillerUse := admissionTestReferenceUse(
		t,
		fixture,
		alternateCoordinate,
		alternateFillerResolution,
		admissionTestMember(t, fixture),
	)
	_, err = NewContextSliceMembershipBasis(ContextSliceMembershipBasisInput{
		TypeEnv:                      fixture.typeEnv.ref,
		GraphRevision:                NewGraphRevision(42),
		Observations:                 []AdmissionSnapshotObservation{observation},
		ReferenceFillerAdmissionUses: []ReferenceFillerAdmissionUse{use, alternateFillerUse},
	})
	if err == nil {
		t.Fatal("one relation-filler position accepted two different final fillers")
	}
}

func TestSameBatchResolutionRequiresDeclarationCorrelationAndBasisObservation(t *testing.T) {
	fixture := newMemberOfFixture(t)
	entity := fixture.query.EntityID()
	localID, err := NewBatchLocalRef("local:vehicle-7")
	if err != nil {
		t.Fatalf("NewBatchLocalRef() error = %v", err)
	}
	localReference, err := NewLocalRef(fixture.typeEnv.entityRefKind, localID)
	if err != nil {
		t.Fatalf("NewLocalRef() error = %v", err)
	}
	declaration, err := NewDeclareEntity(
		entity,
		localID,
		fixture.query.ContextSlice().Context(),
		admissionTestEntityLabel(t, "Vehicle 7"),
		memberOfTestProvenanceRef(t, "prov:test/declaration"),
	)
	if err != nil {
		t.Fatalf("NewDeclareEntity() error = %v", err)
	}
	persisted := admissionTestPersistedRef(t, fixture.typeEnv.entityRefKind, entity)
	resolution, err := NewSameBatchDeclarationResolution(SameBatchDeclarationResolutionInput{
		LocalReference:           localReference,
		DeclarationChangeOrdinal: 0,
		Declaration:              declaration,
		PersistedReference:       persisted,
	})
	if err != nil {
		t.Fatalf("NewSameBatchDeclarationResolution() error = %v", err)
	}
	assertion := admissionTestAssertionID(t, "assertion:local-vehicle")
	coordinate, _ := admissionTestCoordinateForReference(
		t,
		fixture,
		assertion,
		persisted,
		entity,
		1,
		0,
	)
	prefixSet, err := NewMemoryChangeSet([]MemoryChange{declaration})
	if err != nil {
		t.Fatalf("NewMemoryChangeSet(prefix) error = %v", err)
	}
	prefix, err := ComputeOrderedCandidatePrefix(prefixSet, 1)
	if err != nil {
		t.Fatalf("ComputeOrderedCandidatePrefix() error = %v", err)
	}
	view, err := NewProspectiveBatchView(ProspectiveBatchViewInput{
		TypeEnv:                  fixture.typeEnv.ref,
		PreStateGraphRevision:    NewGraphRevision(7),
		EvaluationChangeOrdinal:  1,
		DeclarationChangeOrdinal: 0,
		Declaration:              declaration,
		LocalReference:           localReference,
		PersistedReference:       persisted,
		OrderedCandidatePrefix:   prefix,
	})
	if err != nil {
		t.Fatalf("NewProspectiveBatchView() error = %v", err)
	}
	required := validationTestMemberOfMemberWithView(
		t,
		fixture.query,
		fixture.typeEnv.provenance,
		view,
	)
	disjointQuery, err := NewMemberOfQuery(
		entity,
		fixture.typeEnv.claimGraphValueKind,
		fixture.query.ContextSlice(),
	)
	if err != nil {
		t.Fatalf("NewMemberOfQuery(disjoint) error = %v", err)
	}
	disjointJudgement := validationTestMemberOfNotMemberWithView(
		t,
		disjointQuery,
		fixture.typeEnv.provenance,
		view,
	)
	constraint := fixture.typeEnv.constraint.(KindDisjointConstraint)
	disjointUse, err := NewDisjointNotMemberUse(constraint.ID(), disjointJudgement)
	if err != nil {
		t.Fatalf("NewDisjointNotMemberUse() error = %v", err)
	}
	use, err := NewReferenceFillerAdmissionUse(ReferenceFillerAdmissionUseInput{
		TypeEnv:             fixture.typeEnv.build(t),
		Coordinate:          coordinate,
		Resolution:          resolution,
		RequiredMembership:  required,
		DisjointMemberships: []DisjointCounterUse{disjointUse},
	})
	if err != nil {
		t.Fatalf("NewReferenceFillerAdmissionUse() error = %v", err)
	}
	assertionObservation := admissionTestAssertionAbsentObservation(t, 1, assertion)
	absence, err := NewAbsentEntityResolution(
		entity,
		fixture.query.ContextSlice().Context(),
		mustResolutionBasisRef(t, "snapshot:entity-absence"),
	)
	if err != nil {
		t.Fatalf("NewAbsentEntityResolution() error = %v", err)
	}
	declarationObservation, err := NewEntityAbsentObservation(0, absence)
	if err != nil {
		t.Fatalf("NewEntityAbsentObservation() error = %v", err)
	}

	_, err = NewContextSliceMembershipBasis(ContextSliceMembershipBasisInput{
		TypeEnv:       fixture.typeEnv.ref,
		GraphRevision: NewGraphRevision(7),
		Observations: []AdmissionSnapshotObservation{
			assertionObservation,
			declarationObservation,
		},
		ReferenceFillerAdmissionUses: []ReferenceFillerAdmissionUse{use},
	})
	if err != nil {
		t.Fatalf("correlated same-batch admission basis error = %v", err)
	}

	_, err = NewContextSliceMembershipBasis(ContextSliceMembershipBasisInput{
		TypeEnv:                      fixture.typeEnv.ref,
		GraphRevision:                NewGraphRevision(7),
		Observations:                 []AdmissionSnapshotObservation{assertionObservation},
		ReferenceFillerAdmissionUses: []ReferenceFillerAdmissionUse{use},
	})
	if err == nil {
		t.Fatal("same-batch resolution was admitted without declaration absence evidence")
	}

	otherLocalID, err := NewBatchLocalRef("local:vehicle-8")
	if err != nil {
		t.Fatalf("NewBatchLocalRef(other) error = %v", err)
	}
	otherLocal, err := NewLocalRef(fixture.typeEnv.entityRefKind, otherLocalID)
	if err != nil {
		t.Fatalf("NewLocalRef(other) error = %v", err)
	}
	_, err = NewSameBatchDeclarationResolution(SameBatchDeclarationResolutionInput{
		LocalReference:           otherLocal,
		DeclarationChangeOrdinal: 0,
		Declaration:              declaration,
		PersistedReference:       persisted,
	})
	if err == nil {
		t.Fatal("same-batch resolution accepted a local ref for another declaration")
	}
	resolvedLocal, err := NewResolvedStrongReference(
		localReference,
		entity,
		fixture.query.ContextSlice().Context(),
		mustResolutionBasisRef(t, "snapshot:invalid-local-resolution"),
	)
	if err != nil {
		t.Fatalf("NewResolvedStrongReference(local) error = %v", err)
	}
	_, err = NewSnapshotReferenceResolution(resolvedLocal)
	if err == nil {
		t.Fatal("snapshot admission resolution accepted a batch-local reference")
	}
}

func admissionTestSnapshotOnlyBasis(
	t *testing.T,
	typeEnv TypeEnvRef,
	revision GraphRevision,
	observations []AdmissionSnapshotObservation,
) SnapshotOnlyBasis {
	t.Helper()
	basis, err := NewSnapshotOnlyBasis(SnapshotOnlyBasisInput{
		TypeEnv:       typeEnv,
		GraphRevision: revision,
		Observations:  observations,
	})
	if err != nil {
		t.Fatalf("NewSnapshotOnlyBasis() error = %v", err)
	}
	return basis
}

func admissionTestAssertionID(t *testing.T, raw string) AssertionID {
	t.Helper()
	value, err := NewAssertionID(raw)
	if err != nil {
		t.Fatalf("NewAssertionID(%q) error = %v", raw, err)
	}
	return value
}

func admissionTestEntityLabel(t *testing.T, raw string) EntityLabel {
	t.Helper()
	value, err := NewEntityLabel(raw)
	if err != nil {
		t.Fatalf("NewEntityLabel(%q) error = %v", raw, err)
	}
	return value
}

func admissionTestPersistedRef(
	t *testing.T,
	kind RefKindRef,
	entity EntityID,
) PersistedRef {
	t.Helper()
	referenceID, err := NewReferenceID(entity.String())
	if err != nil {
		t.Fatalf("NewReferenceID() error = %v", err)
	}
	reference, err := NewPersistedRef(kind, referenceID)
	if err != nil {
		t.Fatalf("NewPersistedRef() error = %v", err)
	}
	return reference
}

func admissionTestCoordinate(
	t *testing.T,
	fixture memberOfFixture,
	assertion AssertionID,
	changeOrdinal uint64,
	fillerOrdinal uint64,
) (RelationFillerCoordinate, PersistedRef) {
	t.Helper()
	reference := admissionTestPersistedRef(
		t,
		fixture.typeEnv.entityRefKind,
		fixture.query.EntityID(),
	)
	coordinate, fillerReference := admissionTestCoordinateForReference(
		t,
		fixture,
		assertion,
		reference,
		fixture.query.EntityID(),
		changeOrdinal,
		fillerOrdinal,
	)
	return coordinate, fillerReference
}

func admissionTestCoordinateForReference(
	t *testing.T,
	fixture memberOfFixture,
	assertion AssertionID,
	reference PersistedRef,
	entity EntityID,
	changeOrdinal uint64,
	fillerOrdinal uint64,
) (RelationFillerCoordinate, PersistedRef) {
	t.Helper()
	filler := newReferenceFiller(reference, entity)
	candidateFiller, err := NewByReferenceCandidate(reference)
	if err != nil {
		t.Fatalf("NewByReferenceCandidate() error = %v", err)
	}
	candidateBinding, err := NewCandidateSlotBinding(
		fixture.typeEnv.entitySlot,
		[]CandidateSlotFiller{candidateFiller},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding() error = %v", err)
	}
	candidate, err := NewRelationInstantiation(
		assertion,
		fixture.typeEnv.signature.Ref(),
		fixture.query.ContextSlice(),
		[]CandidateSlotBinding{candidateBinding},
		memberOfTestProvenanceRef(t, "prov:test/relation"),
	)
	if err != nil {
		t.Fatalf("NewRelationInstantiation() error = %v", err)
	}
	relation := newRelationInstance(
		candidate,
		[]SlotBinding{newSlotBinding(fixture.typeEnv.entitySlot, []SlotFiller{filler})},
	)
	coordinate, err := NewRelationFillerCoordinate(RelationFillerCoordinateInput{
		TypeEnv:       fixture.typeEnv.build(t),
		ChangeOrdinal: changeOrdinal,
		Relation:      relation,
		Slot:          fixture.typeEnv.entitySlot,
		FillerOrdinal: fillerOrdinal,
	})
	if err != nil {
		t.Fatalf("NewRelationFillerCoordinate() error = %v", err)
	}
	return coordinate, reference
}

func admissionTestSnapshotReferenceResolution(
	t *testing.T,
	reference PersistedRef,
	entity EntityID,
	context BoundedContextRef,
	basis string,
) SnapshotReferenceResolution {
	t.Helper()
	resolved, err := NewResolvedStrongReference(
		reference,
		entity,
		context,
		mustResolutionBasisRef(t, basis),
	)
	if err != nil {
		t.Fatalf("NewResolvedStrongReference() error = %v", err)
	}
	admissionResolution, err := NewSnapshotReferenceResolution(resolved)
	if err != nil {
		t.Fatalf("NewSnapshotReferenceResolution() error = %v", err)
	}
	return admissionResolution
}

func admissionTestMember(t *testing.T, fixture memberOfFixture) MemberOfMember {
	t.Helper()
	basis := fixture.basis(t, []MemberOfObservableInput{
		memberOfTestObservableInput(t, "observable:registry/vehicle-7", 0xc2),
	})
	member, err := NewMemberOfMember(fixture.query, basis)
	if err != nil {
		t.Fatalf("NewMemberOfMember() error = %v", err)
	}
	return member
}

func admissionTestDisjointNotMember(
	t *testing.T,
	fixture memberOfFixture,
) DisjointNotMemberUse {
	t.Helper()
	constraint := fixture.typeEnv.constraint.(KindDisjointConstraint)
	return admissionTestNotMemberUse(
		t,
		fixture,
		fixture.typeEnv.claimGraphValueKind,
		constraint.ID(),
		"claim-graph",
	)
}

func admissionTestNotMemberUse(
	t *testing.T,
	fixture memberOfFixture,
	valueKind ValueKindRef,
	constraint ConstraintID,
	suffix string,
) DisjointNotMemberUse {
	t.Helper()
	query, err := NewMemberOfQuery(
		fixture.query.EntityID(),
		valueKind,
		fixture.query.ContextSlice(),
	)
	if err != nil {
		t.Fatalf("NewMemberOfQuery(disjoint) error = %v", err)
	}
	entitySet := typeEnvTestEntitySetDefinition(
		t,
		fixture.typeEnv.ref,
		fixture.query.ContextSlice().Context(),
		"test:entity-set/"+suffix+"/v1",
		fixture.typeEnv.provenance,
	)
	signature := typeEnvTestKindSignatureDefinition(
		t,
		valueKind,
		SignatureF4,
		nil,
		"test:member-of/"+suffix+"/v1",
		entitySet.Ref(),
		fixture.typeEnv.provenance,
	)
	basis, err := NewMemberOfBasis(MemberOfBasisInput{
		Query:          query,
		EvaluationView: fixture.evaluationView,
		KindSignature:  signature,
		EntitySet:      entitySet,
		ObservableInputs: []MemberOfObservableInput{
			memberOfTestObservableInput(t, "observable:registry/"+suffix, 0xc3),
		},
		EvaluationProvenance: fixture.evaluationProvenance,
	})
	if err != nil {
		t.Fatalf("NewMemberOfBasis(disjoint) error = %v", err)
	}
	judgement, err := NewMemberOfNotMember(query, basis)
	if err != nil {
		t.Fatalf("NewMemberOfNotMember() error = %v", err)
	}
	use, err := NewDisjointNotMemberUse(constraint, judgement)
	if err != nil {
		t.Fatalf("NewDisjointNotMemberUse() error = %v", err)
	}
	return use
}

func admissionTestReferenceUse(
	t *testing.T,
	fixture memberOfFixture,
	coordinate RelationFillerCoordinate,
	resolution AdmissionReferenceResolution,
	required MemberOfMember,
) ReferenceFillerAdmissionUse {
	t.Helper()
	use, err := NewReferenceFillerAdmissionUse(ReferenceFillerAdmissionUseInput{
		TypeEnv:            fixture.typeEnv.build(t),
		Coordinate:         coordinate,
		Resolution:         resolution,
		RequiredMembership: required,
		DisjointMemberships: []DisjointCounterUse{
			admissionTestDisjointNotMember(t, fixture),
		},
	})
	if err != nil {
		t.Fatalf("NewReferenceFillerAdmissionUse() error = %v", err)
	}
	return use
}

func admissionTestAssertionAbsentObservation(
	t *testing.T,
	changeOrdinal uint64,
	assertion AssertionID,
) AssertionAbsentObservation {
	t.Helper()
	state, err := NewAbsentAssertionState(
		assertion,
		typeEnvTestRuleRef(t, "test:snapshot/assertion-absence/v1"),
	)
	if err != nil {
		t.Fatalf("NewAbsentAssertionState() error = %v", err)
	}
	observation, err := NewAssertionAbsentObservation(changeOrdinal, state)
	if err != nil {
		t.Fatalf("NewAssertionAbsentObservation() error = %v", err)
	}
	return observation
}
