package typedmemory

import "testing"

func TestBatchLocalLabelChangesCandidateDigestButNotAdmittedDigest(t *testing.T) {
	first := newLocalReferenceAdmissionFixture(t, "local:first-label", true)
	second := newLocalReferenceAdmissionFixture(t, "local:second-label", true)

	firstCandidate := validationTestDigest(t, first.changeSet)
	secondCandidate := validationTestDigest(t, second.changeSet)
	if firstCandidate == secondCandidate {
		t.Fatal("candidate digest erased distinct batch-local labels")
	}

	firstAdmitted := validationTestCanonicalDigest(t, first.verdict(t))
	secondAdmitted := validationTestCanonicalDigest(t, second.verdict(t))
	if firstAdmitted != secondAdmitted {
		t.Fatalf(
			"batch-local label changed admitted digest: %s != %s",
			firstAdmitted.String(),
			secondAdmitted.String(),
		)
	}
}

func TestPersistedReferenceCannotMasqueradeAsSameBatchLocalReference(t *testing.T) {
	local := newLocalReferenceAdmissionFixture(t, "local:temporary-label", true)
	persisted := newLocalReferenceAdmissionFixture(t, "local:temporary-label", false)

	localCandidate := validationTestDigest(t, local.changeSet)
	persistedCandidate := validationTestDigest(t, persisted.changeSet)
	if localCandidate == persistedCandidate {
		t.Fatal("candidate digest erased local-versus-persisted reference evidence")
	}

	_ = validationTestCanonicalDigest(t, local.verdict(t))
	assertValidationDiagnostic(
		t,
		persisted.verdict(t),
		ValidationInvalid,
		DiagnosticReferenceUnresolved,
	)
}

func TestAdmissionBatchContainsOnlyStableEntityReferences(t *testing.T) {
	fixture := newLocalReferenceAdmissionFixture(t, "local:must-not-escape", true)
	verdict := fixture.verdict(t)
	valid, ok := verdict.(Valid)
	if !ok {
		t.Fatalf("verdict = %T (%s); want Valid", verdict, verdict.Kind())
	}

	changes := valid.AdmissionBatch().ChangeSet().Changes()
	if len(changes) != 2 {
		t.Fatalf("admitted changes = %d; want declaration and relation", len(changes))
	}
	declaration, ok := changes[0].(ValidatedDeclareEntity)
	if !ok {
		t.Fatalf("admitted change[0] = %T; want ValidatedDeclareEntity", changes[0])
	}
	if declaration.Change().Entity() != fixture.entity {
		t.Fatal("admitted declaration changed stable EntityID")
	}

	relation, ok := changes[1].(ValidatedRelationInstance)
	if !ok {
		t.Fatalf("admitted change[1] = %T; want ValidatedRelationInstance", changes[1])
	}
	reference := admittedEntityReference(t, relation.Relation())
	entity := admittedEntityID(t, relation.Relation())
	if entity != fixture.entity {
		t.Fatalf("admitted stable EntityID = %q; want %q", entity.String(), fixture.entity.String())
	}
	if reference.RefKind() != fixture.stableReference.RefKind() {
		t.Fatal("admitted reference changed exact RefKindRef")
	}
	if reference.ReferenceKey() != fixture.stableReference.ReferenceKey() {
		t.Fatalf(
			"admitted reference = %q; want %q",
			reference.ReferenceKey(),
			fixture.stableReference.ReferenceKey(),
		)
	}
	if reference.ReferenceID().String() != fixture.entity.String() {
		t.Fatalf(
			"admitted ReferenceID = %q; want stable EntityID %q",
			reference.ReferenceID().String(),
			fixture.entity.String(),
		)
	}
	if _, escaped := any(reference).(LocalRef); escaped {
		t.Fatal("LocalRef escaped into AdmissionBatch")
	}
}

type localReferenceAdmissionFixture struct {
	environment     TypeEnv
	registry        CodecRegistry
	snapshot        validationSnapshot
	changeSet       MemoryChangeSet
	entity          EntityID
	stableReference PersistedRef
}

func newLocalReferenceAdmissionFixture(
	t *testing.T,
	localRaw string,
	useLocalReference bool,
) localReferenceAdmissionFixture {
	t.Helper()
	base := newValidationFixture(t)
	context := base.typeEnv.primaryContext.Ref()
	entity := mustEntityID(t, "entity:canonical-local-reference")
	local, err := NewBatchLocalRef(localRaw)
	if err != nil {
		t.Fatalf("NewBatchLocalRef: %v", err)
	}
	referenceID, err := NewReferenceID(entity.String())
	if err != nil {
		t.Fatalf("NewReferenceID: %v", err)
	}
	stableReference, err := NewPersistedRef(base.typeEnv.entityRefKind, referenceID)
	if err != nil {
		t.Fatalf("NewPersistedRef: %v", err)
	}

	var candidateReference StrongRef = stableReference
	if useLocalReference {
		candidateReference, err = NewLocalRef(base.typeEnv.entityRefKind, local)
		if err != nil {
			t.Fatalf("NewLocalRef: %v", err)
		}
	}
	referenceCandidate, err := NewByReferenceCandidate(candidateReference)
	if err != nil {
		t.Fatalf("NewByReferenceCandidate: %v", err)
	}
	entityBinding, err := NewCandidateSlotBinding(
		base.typeEnv.entitySlot,
		[]CandidateSlotFiller{referenceCandidate},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding: %v", err)
	}
	relation := validationTestRelation(
		t,
		base,
		base.typeEnv.signature.Ref(),
		[]CandidateSlotBinding{entityBinding, base.claimBinding},
	)
	instantiation, err := NewInstantiateRelation(relation)
	if err != nil {
		t.Fatalf("NewInstantiateRelation: %v", err)
	}
	label, err := NewEntityLabel("Canonical local-reference entity")
	if err != nil {
		t.Fatalf("NewEntityLabel: %v", err)
	}
	declaration, err := NewDeclareEntity(
		entity,
		local,
		context,
		label,
		typeEnvTestProvenanceRef(t, "memory:canonical-local-reference-declaration"),
	)
	if err != nil {
		t.Fatalf("NewDeclareEntity: %v", err)
	}
	changeSet, err := NewMemoryChangeSet([]MemoryChange{declaration, instantiation})
	if err != nil {
		t.Fatalf("NewMemoryChangeSet: %v", err)
	}

	absent, err := NewAbsentEntityResolution(
		entity,
		context,
		mustResolutionBasisRef(t, "snapshot:canonical-local-reference-absent"),
	)
	if err != nil {
		t.Fatalf("NewAbsentEntityResolution: %v", err)
	}
	resolved, err := NewResolvedStrongReference(
		candidateReference,
		entity,
		context,
		mustResolutionBasisRef(t, "snapshot:canonical-local-reference-resolution"),
	)
	if err != nil {
		t.Fatalf("NewResolvedStrongReference: %v", err)
	}
	snapshot := base.snapshot
	snapshot.entityResolution = absent
	snapshot.referenceResolution = resolved
	validationTestSetMemberOf(
		t,
		&snapshot,
		validationTestMemberOfMember(
			t,
			validationTestMemberOfQuery(
				t,
				entity,
				base.typeEnv.entityValueKind,
				relation.Slice(),
			),
			base.typeEnv.provenance,
		),
	)
	validationTestSetMemberOf(
		t,
		&snapshot,
		validationTestMemberOfNotMember(
			t,
			validationTestMemberOfQuery(
				t,
				entity,
				base.typeEnv.claimGraphValueKind,
				relation.Slice(),
			),
			base.typeEnv.provenance,
		),
	)

	return localReferenceAdmissionFixture{
		environment:     base.environment,
		registry:        base.registry,
		snapshot:        snapshot,
		changeSet:       changeSet,
		entity:          entity,
		stableReference: stableReference,
	}
}

func (fixture localReferenceAdmissionFixture) verdict(t *testing.T) ValidationVerdict {
	t.Helper()
	return ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
}

func admittedEntityReference(t *testing.T, relation RelationInstance) PersistedRef {
	t.Helper()
	for _, binding := range relation.Bindings() {
		for _, filler := range binding.Fillers() {
			reference, ok := filler.(ReferenceFiller)
			if ok {
				return reference.Reference()
			}
		}
	}
	t.Fatal("admitted relation has no reference filler")
	return PersistedRef{}
}

func admittedEntityID(t *testing.T, relation RelationInstance) EntityID {
	t.Helper()
	for _, binding := range relation.Bindings() {
		for _, filler := range binding.Fillers() {
			reference, ok := filler.(ReferenceFiller)
			if ok {
				return reference.Entity()
			}
		}
	}
	t.Fatal("admitted relation has no stable EntityID filler")
	return EntityID{}
}
