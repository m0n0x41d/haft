package typedmemory

import (
	"bytes"
	"reflect"
	"testing"
)

func TestValidateMemoryChangeSetAcceptsExactTypedRelation(t *testing.T) {
	fixture := newValidationFixture(t)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	valid, ok := verdict.(Valid)
	if !ok {
		t.Fatalf("verdict = %T (%s); want Valid", verdict, verdict.Kind())
	}
	changes := valid.ChangeSet().Changes()
	if len(changes) != 1 {
		t.Fatalf("validated changes = %d; want 1", len(changes))
	}
	if _, ok := changes[0].(ValidatedRelationInstance); !ok {
		t.Fatalf("validated change = %T; want ValidatedRelationInstance", changes[0])
	}
	batch := valid.AdmissionBatch()
	if !batch.IsValid() {
		t.Fatal("valid relation verdict returned a non-admissible batch")
	}
	basis, ok := batch.Basis().(ContextSliceMembershipBasis)
	if !ok {
		t.Fatalf("admission basis = %T; want ContextSliceMembershipBasis", batch.Basis())
	}
	if basis.TypeEnv() != fixture.environment.Ref() ||
		basis.GraphRevision() != fixture.snapshot.GraphRevision() {
		t.Fatal("admission basis lost the exact TypeEnv or graph revision")
	}
	observations := basis.SnapshotObservations()
	if len(observations) != 1 {
		t.Fatalf("snapshot observations = %d; want assertion absence", len(observations))
	}
	assertionAbsent, ok := observations[0].(AssertionAbsentObservation)
	if !ok || assertionAbsent.ChangeOrdinal() != 0 || assertionAbsent.State().Assertion() != fixture.assertion {
		t.Fatalf("snapshot observation = %T; want exact assertion absence at change 0", observations[0])
	}
	uses := basis.ReferenceFillerAdmissionUses()
	if len(uses) != 1 {
		t.Fatalf("reference-filler uses = %d; want 1", len(uses))
	}
	use := uses[0]
	coordinate := use.Coordinate()
	if coordinate.ChangeOrdinal() != 0 ||
		coordinate.Assertion() != fixture.assertion ||
		coordinate.Slot() != fixture.typeEnv.entitySlot ||
		coordinate.FillerOrdinal() != 0 ||
		coordinate.Reference() != fixture.reference ||
		coordinate.Entity() != fixture.referenceEntity ||
		!sameContextSlice(coordinate.ContextSlice(), use.RequiredMembership().Query().ContextSlice()) {
		t.Fatal("reference-filler admission use is not correlated with the final canonical relation")
	}
	if _, ok := use.Resolution().(SnapshotReferenceResolution); !ok {
		t.Fatalf("reference resolution = %T; want SnapshotReferenceResolution", use.Resolution())
	}
	if len(use.DisjointMemberships()) != 1 {
		t.Fatalf("disjoint membership uses = %d; want 1", len(use.DisjointMemberships()))
	}
	if use.DisjointMemberships()[0].Kind() != DirectNotMemberDisjointCounterUse {
		t.Fatal("defined negative evaluator result was not preserved as a direct disjoint counter use")
	}
}

func TestValidateMemoryChangeSetUsesExactDisjointEntailmentWhenNoCounterSourceApplies(
	t *testing.T,
) {
	fixture := newValidationFixture(t)
	relation := fixture.changeSet.Changes()[0].(InstantiateRelation).Relation()
	counterQuery := validationTestMemberOfQuery(
		t,
		fixture.referenceEntity,
		fixture.typeEnv.claimGraphValueKind,
		relation.Slice(),
	)
	missing, err := NoApplicableObservableSourceForMemberOf(counterQuery)
	if err != nil {
		t.Fatalf("NoApplicableObservableSourceForMemberOf(): %v", err)
	}
	repair, err := NewRepairPointer("repair:test/no-applicable-counter-source")
	if err != nil {
		t.Fatalf("NewRepairPointer(): %v", err)
	}
	undefined, err := NewMemberOfUndefined(
		validationTestMemberOfRequest(t, counterQuery),
		[]MemberOfMissingBasis{missing},
		repair,
	)
	if err != nil {
		t.Fatalf("NewMemberOfUndefined(): %v", err)
	}
	validationTestSetMemberOf(t, &fixture.snapshot, undefined)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	valid, ok := verdict.(Valid)
	if !ok {
		t.Fatalf("verdict = %T (%s); want Valid", verdict, verdict.Kind())
	}
	basis := valid.AdmissionBatch().Basis().(ContextSliceMembershipBasis)
	uses := basis.ReferenceFillerAdmissionUses()
	if len(uses) != 1 || len(uses[0].DisjointMemberships()) != 1 {
		t.Fatalf("reference/disjoint uses = %d/%d; want 1/1", len(uses), len(uses[0].DisjointMemberships()))
	}
	counter := uses[0].DisjointMemberships()[0]
	entailed, ok := counter.(DisjointEntailmentUse)
	if !ok || counter.Kind() != EntailedDisjointCounterUse {
		t.Fatalf("counter use = %T/%s; want exact DisjointEntailmentUse", counter, counter.Kind())
	}
	if entailed.SupportingMembership().Digest() != uses[0].RequiredMembership().Digest() ||
		entailed.CounterQuery().Digest() != counterQuery.Digest() {
		t.Fatal("disjoint entailment lost the required membership or exact counter query")
	}
}

func TestValidateMemoryChangeSetReusesOnlyExactRevalidatedDisjointEntailment(
	t *testing.T,
) {
	fixture := newValidationFixture(t)
	relation := fixture.changeSet.Changes()[0].(InstantiateRelation).Relation()
	counterQuery := validationTestMemberOfQuery(
		t,
		fixture.referenceEntity,
		fixture.typeEnv.claimGraphValueKind,
		relation.Slice(),
	)
	missing, err := NoApplicableObservableSourceForMemberOf(counterQuery)
	if err != nil {
		t.Fatalf("NoApplicableObservableSourceForMemberOf(): %v", err)
	}
	repair, err := NewRepairPointer("repair:test/revalidated-disjoint-entailment")
	if err != nil {
		t.Fatalf("NewRepairPointer(): %v", err)
	}
	undefined, err := NewMemberOfUndefined(
		validationTestMemberOfRequest(t, counterQuery),
		[]MemberOfMissingBasis{missing},
		repair,
	)
	if err != nil {
		t.Fatalf("NewMemberOfUndefined(): %v", err)
	}
	validationTestSetMemberOf(t, &fixture.snapshot, undefined)

	initial := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	valid, ok := initial.(Valid)
	if !ok {
		t.Fatalf("initial verdict = %T (%s); want Valid", initial, initial.Kind())
	}
	use := valid.AdmissionBatch().Basis().(ContextSliceMembershipBasis).
		ReferenceFillerAdmissionUses()[0]
	proof := use.DisjointMemberships()[0].(DisjointEntailmentUse)
	request, err := NewMemberOfEvaluationRequest(
		proof.CounterQuery(),
		proof.EvaluationView(),
	)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest(): %v", err)
	}
	fixture.snapshot.disjointEntailments = map[string]DisjointEntailmentUse{
		validationDisjointEntailmentKey(
			request,
			proof.Constraint(),
			proof.SupportingMembership(),
		): proof,
	}
	fixture.snapshot.memberOfJudgements[counterQuery.Digest().String()] =
		validationTestMemberOfMemberWithView(
			t,
			counterQuery,
			fixture.typeEnv.provenance,
			proof.EvaluationView(),
		)

	replayed := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	replayedValid, ok := replayed.(Valid)
	if !ok {
		t.Fatalf("replayed verdict = %T (%s); want Valid", replayed, replayed.Kind())
	}
	replayedUse := replayedValid.AdmissionBatch().Basis().(ContextSliceMembershipBasis).
		ReferenceFillerAdmissionUses()[0].DisjointMemberships()[0]
	if replayedUse.Digest() != proof.Digest() ||
		!bytes.Equal(replayedUse.CanonicalBytes(), proof.CanonicalBytes()) {
		t.Fatal("replayed disjoint entailment differs from the exact revalidated proof")
	}

	fixture.snapshot.disjointEntailments[validationDisjointEntailmentKey(
		request,
		proof.Constraint(),
		proof.SupportingMembership(),
	)] =
		corruptDisjointEntailmentUse{DisjointEntailmentUse: proof}
	rejected := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	assertValidationDiagnostic(
		t,
		rejected,
		ValidationUnderdetermined,
		DiagnosticTypeRuleUnavailable,
	)
}

func TestValidateMemoryChangeSetEntailsDisjointnessIndependentlyOfCounterSourcePosture(
	t *testing.T,
) {
	fixture := newValidationFixture(t)
	relation := fixture.changeSet.Changes()[0].(InstantiateRelation).Relation()
	counterQuery := validationTestMemberOfQuery(
		t,
		fixture.referenceEntity,
		fixture.typeEnv.claimGraphValueKind,
		relation.Slice(),
	)
	missing, err := MissingUniqueTrustedObservableSourceForMemberOf(counterQuery)
	if err != nil {
		t.Fatalf("MissingUniqueTrustedObservableSourceForMemberOf(): %v", err)
	}
	repair, err := NewRepairPointer("repair:test/unusable-counter-source")
	if err != nil {
		t.Fatalf("NewRepairPointer(): %v", err)
	}
	undefined, err := NewMemberOfUndefined(
		validationTestMemberOfRequest(t, counterQuery),
		[]MemberOfMissingBasis{missing},
		repair,
	)
	if err != nil {
		t.Fatalf("NewMemberOfUndefined(): %v", err)
	}
	validationTestSetMemberOf(t, &fixture.snapshot, undefined)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	valid, ok := verdict.(Valid)
	if !ok {
		t.Fatalf("verdict = %T (%s); want Valid", verdict, verdict.Kind())
	}
	uses := valid.AdmissionBatch().Basis().(ContextSliceMembershipBasis).
		ReferenceFillerAdmissionUses()
	if len(uses) != 1 || len(uses[0].DisjointMemberships()) != 1 {
		t.Fatalf(
			"reference/disjoint uses = %d/%d; want 1/1",
			len(uses),
			len(uses[0].DisjointMemberships()),
		)
	}
	if _, ok := uses[0].DisjointMemberships()[0].(DisjointEntailmentUse); !ok {
		t.Fatalf(
			"counter use = %T; want DisjointEntailmentUse",
			uses[0].DisjointMemberships()[0],
		)
	}
}

func TestValidateMemoryChangeSetCapturesSnapshotOnlyDeclarationBasis(t *testing.T) {
	fixture := newValidationFixture(t)
	entity := mustEntityID(t, "entity:new-service")
	context := fixture.typeEnv.primaryContext.Ref()
	absent, err := NewAbsentEntityResolution(
		entity,
		context,
		mustResolutionBasisRef(t, "snapshot:entity-absence"),
	)
	if err != nil {
		t.Fatalf("NewAbsentEntityResolution: %v", err)
	}
	fixture.snapshot.entityResolution = absent
	changeSet := validationTestDeclareEntityChangeSet(t, entity, context)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	valid, ok := verdict.(Valid)
	if !ok {
		t.Fatalf("verdict = %T (%s); want Valid", verdict, verdict.Kind())
	}
	basis, ok := valid.AdmissionBatch().Basis().(SnapshotOnlyBasis)
	if !ok {
		t.Fatalf("admission basis = %T; want SnapshotOnlyBasis", valid.AdmissionBatch().Basis())
	}
	observations := basis.SnapshotObservations()
	if len(observations) != 1 {
		t.Fatalf("snapshot observations = %d; want one entity absence", len(observations))
	}
	entityAbsent, ok := observations[0].(EntityAbsentObservation)
	if !ok ||
		entityAbsent.ChangeOrdinal() != 0 ||
		entityAbsent.Resolution().Entity() != entity ||
		entityAbsent.Resolution().Context() != context {
		t.Fatalf("snapshot observation = %T; want exact entity absence at change 0", observations[0])
	}
}

func TestValidateMemoryChangeSetRejectsAbsentEntityThroughPersistedFillerInEveryPermutation(t *testing.T) {
	fixture := newValidationFixture(t)
	cardinality, err := NewBoundedCardinality(2, 2)
	if err != nil {
		t.Fatalf("NewBoundedCardinality: %v", err)
	}
	entityTarget, err := NewReferenceSlotTarget(
		fixture.typeEnv.entityValueKind,
		fixture.typeEnv.entityRefKind,
	)
	if err != nil {
		t.Fatalf("NewReferenceSlotTarget: %v", err)
	}
	entitySlot, err := NewSlotSpec(
		fixture.typeEnv.entitySlot,
		entityTarget,
		cardinality,
		fixture.typeEnv.provenance,
	)
	if err != nil {
		t.Fatalf("NewSlotSpec: %v", err)
	}
	claimTarget, err := NewValueSlotTarget(fixture.typeEnv.claimGraphValueKind)
	if err != nil {
		t.Fatalf("NewValueSlotTarget: %v", err)
	}
	claimSlot, err := NewSlotSpec(
		fixture.typeEnv.claimGraphSlot,
		claimTarget,
		ExactlyOneCardinality(),
		fixture.typeEnv.provenance,
	)
	if err != nil {
		t.Fatalf("NewSlotSpec(claim): %v", err)
	}
	signatureRef := typeEnvTestSignatureRef(
		t,
		fixture.environment.Ref(),
		"test.DuplicateReferenceEvidence",
	)
	signature, err := NewRelationSignature(
		signatureRef,
		[]BoundedContextRef{fixture.typeEnv.primaryContext.Ref()},
		[]SlotSpec{entitySlot, claimSlot},
		fixture.typeEnv.provenance,
	)
	if err != nil {
		t.Fatalf("NewRelationSignature: %v", err)
	}
	environment, err := fixture.typeEnv.builder().
		AddRelationSignature(signature).
		Build()
	if err != nil {
		t.Fatalf("Build TypeEnv: %v", err)
	}

	localID, err := NewBatchLocalRef("local:duplicate-reference-evidence")
	if err != nil {
		t.Fatalf("NewBatchLocalRef: %v", err)
	}
	localRef, err := NewLocalRef(fixture.typeEnv.entityRefKind, localID)
	if err != nil {
		t.Fatalf("NewLocalRef: %v", err)
	}
	localCandidate, err := NewByReferenceCandidate(localRef)
	if err != nil {
		t.Fatalf("NewByReferenceCandidate(local): %v", err)
	}
	persistedCandidate, err := NewByReferenceCandidate(fixture.reference)
	if err != nil {
		t.Fatalf("NewByReferenceCandidate(persisted): %v", err)
	}
	label, err := NewEntityLabel("Duplicate evidence entity")
	if err != nil {
		t.Fatalf("NewEntityLabel: %v", err)
	}
	declaration, err := NewDeclareEntity(
		fixture.referenceEntity,
		localID,
		fixture.typeEnv.primaryContext.Ref(),
		label,
		typeEnvTestProvenanceRef(t, "memory:duplicate-evidence-declaration"),
	)
	if err != nil {
		t.Fatalf("NewDeclareEntity: %v", err)
	}
	absentEntity, err := NewAbsentEntityResolution(
		fixture.referenceEntity,
		fixture.typeEnv.primaryContext.Ref(),
		mustResolutionBasisRef(t, "snapshot:duplicate-evidence-entity-absence"),
	)
	if err != nil {
		t.Fatalf("NewAbsentEntityResolution: %v", err)
	}
	fixture.snapshot.entityResolution = absentEntity
	fixture.snapshot.typeEnv = environment.Ref()

	validate := func(order []CandidateSlotFiller) ValidationVerdict {
		t.Helper()
		entityBinding, bindingErr := NewCandidateSlotBinding(fixture.typeEnv.entitySlot, order)
		if bindingErr != nil {
			t.Fatalf("NewCandidateSlotBinding: %v", bindingErr)
		}
		relation := validationTestRelation(
			t,
			fixture,
			signature.Ref(),
			[]CandidateSlotBinding{entityBinding, fixture.claimBinding},
		)
		instantiation, changeErr := NewInstantiateRelation(relation)
		if changeErr != nil {
			t.Fatalf("NewInstantiateRelation: %v", changeErr)
		}
		changeSet, changeErr := NewMemoryChangeSet([]MemoryChange{declaration, instantiation})
		if changeErr != nil {
			t.Fatalf("NewMemoryChangeSet: %v", changeErr)
		}
		prefix, prefixErr := ComputeOrderedCandidatePrefix(changeSet, 1)
		if prefixErr != nil {
			t.Fatalf("ComputeOrderedCandidatePrefix: %v", prefixErr)
		}
		prospectiveView, viewErr := NewProspectiveBatchView(ProspectiveBatchViewInput{
			TypeEnv:                  environment.Ref(),
			PreStateGraphRevision:    fixture.snapshot.GraphRevision(),
			EvaluationChangeOrdinal:  1,
			DeclarationChangeOrdinal: 0,
			Declaration:              declaration,
			LocalReference:           localRef,
			PersistedReference:       fixture.reference,
			OrderedCandidatePrefix:   prefix,
		})
		if viewErr != nil {
			t.Fatalf("NewProspectiveBatchView: %v", viewErr)
		}
		validationTestSetMemberOf(
			t,
			&fixture.snapshot,
			validationTestMemberOfMemberWithView(
				t,
				validationTestMemberOfQuery(t, fixture.referenceEntity, fixture.typeEnv.entityValueKind, relation.Slice()),
				fixture.typeEnv.provenance,
				prospectiveView,
			),
		)
		validationTestSetMemberOf(
			t,
			&fixture.snapshot,
			validationTestMemberOfNotMemberWithView(
				t,
				validationTestMemberOfQuery(t, fixture.referenceEntity, fixture.typeEnv.claimGraphValueKind, relation.Slice()),
				fixture.typeEnv.provenance,
				prospectiveView,
			),
		)
		verdict := ValidateMemoryChangeSet(
			environment,
			fixture.registry,
			fixture.snapshot,
			changeSet,
		)
		return verdict
	}
	forward := validate([]CandidateSlotFiller{localCandidate, persistedCandidate})
	reverse := validate([]CandidateSlotFiller{persistedCandidate, localCandidate})
	assertValidationDiagnostic(t, forward, ValidationInvalid, DiagnosticReferenceUnresolved)
	assertValidationDiagnostic(t, reverse, ValidationInvalid, DiagnosticReferenceUnresolved)
}

func TestValidateMemoryChangeSetEnforcesReferenceSlotSubsetConstraint(t *testing.T) {
	fixture := newValidationFixture(t)
	subsetSlot := typeEnvTestSlotKindID(t, "SelectedOptionSlot")
	supersetSlot := typeEnvTestSlotKindID(t, "ComparedOptionSlot")
	referenceTarget, err := NewReferenceSlotTarget(
		fixture.typeEnv.entityValueKind,
		fixture.typeEnv.entityRefKind,
	)
	if err != nil {
		t.Fatalf("NewReferenceSlotTarget: %v", err)
	}
	subsetSpec, err := NewSlotSpec(
		subsetSlot,
		referenceTarget,
		ExactlyOneCardinality(),
		fixture.typeEnv.provenance,
	)
	if err != nil {
		t.Fatalf("NewSlotSpec(subset): %v", err)
	}
	supersetSpec, err := NewSlotSpec(
		supersetSlot,
		referenceTarget,
		NewUnboundedCardinality(0),
		fixture.typeEnv.provenance,
	)
	if err != nil {
		t.Fatalf("NewSlotSpec(superset): %v", err)
	}
	signatureRef := typeEnvTestSignatureRef(
		t,
		fixture.environment.Ref(),
		"test.PortfolioComparison",
	)
	signature, err := NewRelationSignature(
		signatureRef,
		[]BoundedContextRef{fixture.typeEnv.primaryContext.Ref()},
		[]SlotSpec{subsetSpec, supersetSpec},
		fixture.typeEnv.provenance,
	)
	if err != nil {
		t.Fatalf("NewRelationSignature: %v", err)
	}
	constraint, err := NewReferenceSlotSubsetConstraint(
		typeEnvTestConstraintID(t, "constraint:selected-subset-compared"),
		signature.Ref(),
		subsetSlot,
		supersetSlot,
		fixture.typeEnv.provenance,
	)
	if err != nil {
		t.Fatalf("NewReferenceSlotSubsetConstraint: %v", err)
	}
	environment, err := fixture.typeEnv.builder().
		AddRelationSignature(signature).
		AddConstraint(constraint).
		Build()
	if err != nil {
		t.Fatalf("Build TypeEnv: %v", err)
	}
	fixture.snapshot.typeEnv = environment.Ref()

	referenceFiller, err := NewByReferenceCandidate(fixture.reference)
	if err != nil {
		t.Fatalf("NewByReferenceCandidate: %v", err)
	}
	subsetBinding, err := NewCandidateSlotBinding(
		subsetSlot,
		[]CandidateSlotFiller{referenceFiller},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding(subset): %v", err)
	}
	relation := validationTestRelation(
		t,
		fixture,
		signature.Ref(),
		[]CandidateSlotBinding{subsetBinding},
	)
	changeSet := validationTestChangeSet(t, relation)

	verdict := ValidateMemoryChangeSet(
		environment,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	assertValidationDiagnostic(
		t,
		verdict,
		ValidationInvalid,
		DiagnosticReferenceSubsetMismatch,
	)
}

func TestValidateMemoryChangeSetCapturesEveryNaryDisjointCounterKind(t *testing.T) {
	fixture := newValidationFixture(t)
	thirdKind := typeEnvTestKindDefinition(
		t,
		"U.Role",
		fixture.typeEnv.provenance,
	)
	constraintID := typeEnvTestConstraintID(t, "constraint:entity-claim-role-disjoint")
	constraint, err := NewKindDisjointConstraint(
		constraintID,
		[]KindID{
			fixture.typeEnv.entityKind.ID(),
			fixture.typeEnv.claimGraphKind.ID(),
			thirdKind.ID(),
		},
		fixture.typeEnv.provenance,
	)
	if err != nil {
		t.Fatalf("NewKindDisjointConstraint: %v", err)
	}
	environment, err := fixture.typeEnv.builder().
		AddKindDefinition(thirdKind).
		AddContextKindAvailability(typeEnvTestKindAvailability(
			fixture.typeEnv.primaryContext.Ref(),
			thirdKind.ID(),
			fixture.typeEnv.provenance,
		)).
		AddConstraint(constraint).
		Build()
	if err != nil {
		t.Fatalf("Build n-ary TypeEnv: %v", err)
	}
	relation := validationTestRelation(
		t,
		fixture,
		fixture.typeEnv.signature.Ref(),
		fixture.bindings,
	)
	thirdKindRef := typeEnvTestValueKindRef(t, environment.Ref(), thirdKind.ID())
	validationTestSetMemberOf(
		t,
		&fixture.snapshot,
		validationTestMemberOfNotMember(
			t,
			validationTestMemberOfQuery(
				t,
				fixture.referenceEntity,
				thirdKindRef,
				relation.Slice(),
			),
			fixture.typeEnv.provenance,
		),
	)
	fixture.snapshot.typeEnv = environment.Ref()

	verdict := ValidateMemoryChangeSet(
		environment,
		fixture.registry,
		fixture.snapshot,
		validationTestChangeSet(t, relation),
	)
	valid, ok := verdict.(Valid)
	if !ok {
		t.Fatalf("verdict = %T (%s); want Valid", verdict, verdict.Kind())
	}
	basis, ok := valid.AdmissionBatch().Basis().(ContextSliceMembershipBasis)
	if !ok {
		t.Fatalf("admission basis = %T; want ContextSliceMembershipBasis", valid.AdmissionBatch().Basis())
	}
	uses := basis.ReferenceFillerAdmissionUses()
	if len(uses) != 1 {
		t.Fatalf("reference-filler uses = %d; want 1", len(uses))
	}
	disjoint := uses[0].DisjointMemberships()
	if len(disjoint) != 3 {
		t.Fatalf("disjoint uses = %d; want binary counter plus both n-ary counters", len(disjoint))
	}
	want := map[string]bool{
		disjointUsePositionKey(
			fixture.typeEnv.constraint.ID(),
			fixture.typeEnv.claimGraphValueKind,
		): true,
		disjointUsePositionKey(
			constraintID,
			fixture.typeEnv.claimGraphValueKind,
		): true,
		disjointUsePositionKey(constraintID, thirdKindRef): true,
	}
	for _, use := range disjoint {
		key := disjointUsePositionKey(use.Constraint(), use.CounterQuery().ValueKind())
		if !want[key] {
			t.Fatalf("unexpected disjoint admission use %q", key)
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatalf("missing exact n-ary counter-kind admission uses: %v", want)
	}
}

func TestValidateMemoryChangeSetAbstainsWhenSignatureBasisIsMissing(t *testing.T) {
	fixture := newValidationFixture(t)
	unknownSignature := typeEnvTestSignatureRef(t, fixture.environment.Ref(), "U.UnknownRelation")
	relation := validationTestRelation(
		t,
		fixture,
		unknownSignature,
		fixture.bindings,
	)
	changeSet := validationTestChangeSet(t, relation)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	assertValidationDiagnostic(t, verdict, ValidationUnderdetermined, DiagnosticSignatureNotActive)
}

func TestEntityValidationPreservesExactUnknownResolutionBasis(t *testing.T) {
	fixture := newValidationFixture(t)
	entity := mustEntityID(t, "entity:unknown-service")
	context := fixture.typeEnv.primaryContext.Ref()
	identityIndex, _ := NewMissingBasis("identity-index")
	aliasIndex, _ := NewMissingBasis("alias-index")
	unknown, err := NewUnknownEntityResolution(
		entity,
		context,
		[]MissingBasis{identityIndex, aliasIndex},
	)
	if err != nil {
		t.Fatalf("NewUnknownEntityResolution: %v", err)
	}
	fixture.snapshot.entityResolution = unknown

	declareVerdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		validationTestDeclareEntityChangeSet(t, entity, context),
	)
	declareDiagnostic := findValidationDiagnostic(
		t,
		declareVerdict,
		DiagnosticIdentityBasisMissing,
	)
	assertMissingResolutionWitness(t, declareDiagnostic, []string{"alias-index", "identity-index"})

	exactAccumulator := validationAccumulator{}
	validateExactEntity(&exactAccumulator, fixture.snapshot, entity, context, 0, "entity")
	if len(exactAccumulator.diagnostics) != 1 {
		t.Fatalf("validateExactEntity diagnostics = %d", len(exactAccumulator.diagnostics))
	}
	assertMissingResolutionWitness(
		t,
		exactAccumulator.diagnostics[0],
		[]string{"alias-index", "identity-index"},
	)
}

func TestUnknownEntityResolutionMismatchRetainsExpectedAndActualQuery(t *testing.T) {
	fixture := newValidationFixture(t)
	requested := mustEntityID(t, "entity:requested")
	returned := mustEntityID(t, "entity:returned")
	context := fixture.typeEnv.primaryContext.Ref()
	missing, _ := NewMissingBasis("identity-index")
	unknown, err := NewUnknownEntityResolution(returned, context, []MissingBasis{missing})
	if err != nil {
		t.Fatalf("NewUnknownEntityResolution: %v", err)
	}
	fixture.snapshot.entityResolution = unknown

	accumulator := validationAccumulator{}
	validateExactEntity(&accumulator, fixture.snapshot, requested, context, 0, "entity")
	if len(accumulator.diagnostics) != 1 {
		t.Fatalf("diagnostics = %d", len(accumulator.diagnostics))
	}
	witness, ok := accumulator.diagnostics[0].Witness().(MissingBasisWitness)
	if !ok {
		t.Fatalf("witness = %T", accumulator.diagnostics[0].Witness())
	}
	if witness.Expected().Kind() != DiagnosticDatumSet ||
		witness.Actual().Kind() != DiagnosticDatumSet {
		t.Fatalf("witness kinds = %q/%q", witness.Expected().Kind(), witness.Actual().Kind())
	}
	if reflect.DeepEqual(witness.Expected().Values(), witness.Actual().Values()) {
		t.Fatal("uncorrelated resolution lost its expected/actual distinction")
	}
}

func TestValidateMemoryChangeSetRejectsSlotReferenceModeMismatch(t *testing.T) {
	fixture := newValidationFixture(t)
	wrongEntityBinding, err := NewCandidateSlotBinding(
		fixture.typeEnv.entitySlot,
		[]CandidateSlotFiller{fixture.valueFiller},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding(): %v", err)
	}
	relation := validationTestRelation(
		t,
		fixture,
		fixture.typeEnv.signature.Ref(),
		[]CandidateSlotBinding{wrongEntityBinding, fixture.claimBinding},
	)
	changeSet := validationTestChangeSet(t, relation)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	assertValidationDiagnostic(t, verdict, ValidationInvalid, DiagnosticReferenceModeMismatch)
}

func TestValidateMemoryChangeSetKeepsKnownByValueMismatchInvalidWithoutKindAvailability(t *testing.T) {
	fixture := newValidationFixture(t)
	availabilities := make(
		[]ContextKindAvailability,
		0,
		len(fixture.environment.kindAvailabilities),
	)
	for _, availability := range fixture.environment.kindAvailabilities {
		if availability.kindID != fixture.typeEnv.claimGraphKind.ID() {
			availabilities = append(availabilities, availability)
		}
	}
	fixture.environment.kindAvailabilities = availabilities
	wrongKind := typeEnvTestValueKindRef(
		t,
		fixture.environment.Ref(),
		fixture.typeEnv.systemKind.ID(),
	)
	graphBytes := fixture.valueFiller.Value().InputBytes()
	wrongCandidate, err := NewTypedValueCandidate(
		wrongKind,
		fixture.typeEnv.binding.ValueShape(),
		fixture.typeEnv.binding.Codec(),
		graphBytes,
		NoAssertedDigest{},
	)
	if err != nil {
		t.Fatalf("NewTypedValueCandidate(): %v", err)
	}
	wrongFiller, err := NewByValueCandidate(wrongCandidate)
	if err != nil {
		t.Fatalf("NewByValueCandidate(): %v", err)
	}
	wrongBinding, err := NewCandidateSlotBinding(
		fixture.typeEnv.claimGraphSlot,
		[]CandidateSlotFiller{wrongFiller},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding(): %v", err)
	}
	relation := validationTestRelation(
		t,
		fixture,
		fixture.typeEnv.signature.Ref(),
		[]CandidateSlotBinding{fixture.entityBinding, wrongBinding},
	)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		validationTestChangeSet(t, relation),
	)
	assertValidationDiagnostic(t, verdict, ValidationInvalid, DiagnosticValueKindMismatch)
}

func TestValidateByValueAcceptsOnlyContextAdmittedSubkind(t *testing.T) {
	fixture := newValidationFixture(t)
	specialKind := typeEnvTestKindDefinition(
		t,
		"U.SpecialClaimGraph",
		fixture.typeEnv.provenance,
	)
	specialValueKind := typeEnvTestValueKindRef(
		t,
		fixture.environment.Ref(),
		specialKind.ID(),
	)
	subkind, err := NewSubkindRelation(
		specialKind.ID(),
		fixture.typeEnv.claimGraphKind.ID(),
		fixture.typeEnv.provenance,
	)
	if err != nil {
		t.Fatalf("NewSubkindRelation: %v", err)
	}
	binding, err := NewValueBinding(
		specialValueKind,
		fixture.typeEnv.binding.ValueShape(),
		fixture.typeEnv.binding.Codec(),
		fixture.typeEnv.provenance,
	)
	if err != nil {
		t.Fatalf("NewValueBinding: %v", err)
	}
	availability := typeEnvTestKindAvailability(
		fixture.typeEnv.primaryContext.Ref(),
		specialKind.ID(),
		fixture.typeEnv.provenance,
	)
	environment, err := fixture.typeEnv.builder().
		AddKindDefinition(specialKind).
		AddSubkindRelation(subkind).
		AddValueBinding(binding).
		AddContextKindAvailability(availability).
		Build()
	if err != nil {
		t.Fatalf("Build TypeEnv with available subkind: %v", err)
	}
	candidate, err := NewTypedValueCandidate(
		specialValueKind,
		fixture.typeEnv.binding.ValueShape(),
		fixture.typeEnv.binding.Codec(),
		fixture.valueFiller.Value().InputBytes(),
		NoAssertedDigest{},
	)
	if err != nil {
		t.Fatalf("NewTypedValueCandidate: %v", err)
	}
	filler, err := NewByValueCandidate(candidate)
	if err != nil {
		t.Fatalf("NewByValueCandidate: %v", err)
	}
	claimBinding, err := NewCandidateSlotBinding(
		fixture.typeEnv.claimGraphSlot,
		[]CandidateSlotFiller{filler},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding: %v", err)
	}
	relation := validationTestRelation(
		t,
		fixture,
		fixture.typeEnv.signature.Ref(),
		[]CandidateSlotBinding{fixture.entityBinding, claimBinding},
	)
	changeSet := validationTestChangeSet(t, relation)

	verdict := ValidateMemoryChangeSet(
		environment,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	if _, ok := verdict.(Valid); !ok {
		t.Fatalf("admitted ByValue subkind verdict = %T (%s); want Valid", verdict, verdict.Kind())
	}

	withoutAdmission, err := fixture.typeEnv.builder().
		AddKindDefinition(specialKind).
		AddSubkindRelation(subkind).
		AddValueBinding(binding).
		Build()
	if err != nil {
		t.Fatalf("Build TypeEnv with unavailable subkind: %v", err)
	}
	verdict = ValidateMemoryChangeSet(
		withoutAdmission,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	assertValidationDiagnostic(t, verdict, ValidationUnderdetermined, DiagnosticKindUnavailableInContext)
}

func TestValidateMemoryChangeSetRejectsMissingRequiredSlot(t *testing.T) {
	fixture := newValidationFixture(t)
	relation := validationTestRelation(
		t,
		fixture,
		fixture.typeEnv.signature.Ref(),
		[]CandidateSlotBinding{fixture.claimBinding},
	)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		validationTestChangeSet(t, relation),
	)
	assertValidationDiagnostic(t, verdict, ValidationInvalid, DiagnosticMissingSlot)
}

func TestRelationInstantiationRejectsDuplicateNamedSlotBeforeValidation(t *testing.T) {
	fixture := newValidationFixture(t)
	_, err := NewRelationInstantiation(
		fixture.assertion,
		fixture.typeEnv.signature.Ref(),
		validationTestContextSlice(t, fixture.typeEnv.primaryContext.Ref()),
		[]CandidateSlotBinding{fixture.entityBinding, fixture.entityBinding},
		typeEnvTestProvenanceRef(t, "memory:duplicate-slot"),
	)
	if err == nil {
		t.Fatal("NewRelationInstantiation() accepted duplicate named slots")
	}
}

func TestValidateMemoryChangeSetRejectsUnknownSlot(t *testing.T) {
	fixture := newValidationFixture(t)
	unknownSlot := typeEnvTestSlotKindID(t, "UnknownSlot")
	unknownBinding, err := NewCandidateSlotBinding(
		unknownSlot,
		[]CandidateSlotFiller{fixture.valueFiller},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding(): %v", err)
	}
	relation := validationTestRelation(
		t,
		fixture,
		fixture.typeEnv.signature.Ref(),
		[]CandidateSlotBinding{fixture.entityBinding, fixture.claimBinding, unknownBinding},
	)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		validationTestChangeSet(t, relation),
	)
	assertValidationDiagnostic(t, verdict, ValidationInvalid, DiagnosticUnknownSlot)
}

func TestValidateMemoryChangeSetRejectsCardinalityOverflow(t *testing.T) {
	fixture := newValidationFixture(t)
	fillers := fixture.entityBinding.Fillers()
	overflowBinding, err := NewCandidateSlotBinding(
		fixture.typeEnv.entitySlot,
		[]CandidateSlotFiller{fillers[0], fillers[0]},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding(): %v", err)
	}
	relation := validationTestRelation(
		t,
		fixture,
		fixture.typeEnv.signature.Ref(),
		[]CandidateSlotBinding{overflowBinding, fixture.claimBinding},
	)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		validationTestChangeSet(t, relation),
	)
	assertValidationDiagnostic(t, verdict, ValidationInvalid, DiagnosticCardinalityMismatch)
}

func TestValidateMemoryChangeSetKeepsMissingEntityKindBasisUnderdetermined(t *testing.T) {
	fixture := newValidationFixture(t)
	query := validationFixtureMemberOfQuery(
		t,
		fixture,
		fixture.typeEnv.entityValueKind,
	)
	validationTestSetMemberOf(
		t,
		&fixture.snapshot,
		validationTestMemberOfUndefined(t, query, "recover-memberof-basis"),
	)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	assertValidationDiagnostic(t, verdict, ValidationUnderdetermined, DiagnosticTypeRuleUnavailable)
}

func TestValidateMemoryChangeSetRejectsKnownEntityKindContradiction(t *testing.T) {
	fixture := newValidationFixture(t)
	query := validationFixtureMemberOfQuery(
		t,
		fixture,
		fixture.typeEnv.entityValueKind,
	)
	validationTestSetMemberOf(
		t,
		&fixture.snapshot,
		validationTestMemberOfNotMember(t, query, fixture.typeEnv.provenance),
	)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	assertValidationDiagnostic(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
}

func TestValidateMemoryChangeSetEnforcesKindDisjointConstraint(t *testing.T) {
	fixture := newValidationFixture(t)
	query := validationFixtureMemberOfQuery(
		t,
		fixture,
		fixture.typeEnv.claimGraphValueKind,
	)
	validationTestSetMemberOf(
		t,
		&fixture.snapshot,
		validationTestMemberOfMember(t, query, fixture.typeEnv.provenance),
	)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	assertValidationDiagnostic(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
}

func TestValidateMemoryChangeSetUsesExactDisjointEntailmentForUndefinedCounterMembership(
	t *testing.T,
) {
	fixture := newValidationFixture(t)
	query := validationFixtureMemberOfQuery(
		t,
		fixture,
		fixture.typeEnv.claimGraphValueKind,
	)
	validationTestSetMemberOf(
		t,
		&fixture.snapshot,
		validationTestMemberOfUndefined(t, query, "recover-disjoint-memberof-basis"),
	)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	valid, ok := verdict.(Valid)
	if !ok {
		t.Fatalf("verdict = %T (%s); want Valid", verdict, verdict.Kind())
	}
	uses := valid.AdmissionBatch().Basis().(ContextSliceMembershipBasis).
		ReferenceFillerAdmissionUses()
	if len(uses) != 1 || len(uses[0].DisjointMemberships()) != 1 {
		t.Fatalf(
			"reference/disjoint uses = %d/%d; want 1/1",
			len(uses),
			len(uses[0].DisjointMemberships()),
		)
	}
	if _, ok := uses[0].DisjointMemberships()[0].(DisjointEntailmentUse); !ok {
		t.Fatalf(
			"counter use = %T; want DisjointEntailmentUse",
			uses[0].DisjointMemberships()[0],
		)
	}
}

func TestValidateMemoryChangeSetRejectsMismatchedDefinedMemberOfJudgement(t *testing.T) {
	fixture := newValidationFixture(t)
	requested := validationFixtureMemberOfQuery(
		t,
		fixture,
		fixture.typeEnv.entityValueKind,
	)
	otherQuery := validationTestMemberOfQuery(
		t,
		mustEntityID(t, "entity:another-service"),
		requested.ValueKind(),
		requested.ContextSlice(),
	)
	fixture.snapshot.memberOfJudgements[requested.Digest().String()] = validationTestMemberOfMember(
		t,
		otherQuery,
		fixture.typeEnv.provenance,
	)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	assertValidationDiagnostic(t, verdict, ValidationInvalid, DiagnosticTypeRuleUnavailable)
}

func TestValidateMemoryChangeSetRejectsMismatchedDefinedDisjointJudgement(t *testing.T) {
	fixture := newValidationFixture(t)
	requested := validationFixtureMemberOfQuery(
		t,
		fixture,
		fixture.typeEnv.claimGraphValueKind,
	)
	otherQuery := validationTestMemberOfQuery(
		t,
		mustEntityID(t, "entity:another-service"),
		requested.ValueKind(),
		requested.ContextSlice(),
	)
	fixture.snapshot.memberOfJudgements[requested.Digest().String()] = validationTestMemberOfNotMember(
		t,
		otherQuery,
		fixture.typeEnv.provenance,
	)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	assertValidationDiagnostic(t, verdict, ValidationInvalid, DiagnosticTypeRuleUnavailable)
}

func TestValidateMemoryChangeSetKeepsMissingMemberOfImplementationUnderdetermined(t *testing.T) {
	fixture := newValidationFixture(t)
	query := validationFixtureMemberOfQuery(
		t,
		fixture,
		fixture.typeEnv.entityValueKind,
	)
	fixture.snapshot.memberOfJudgements[query.Digest().String()] = nil

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	assertValidationDiagnostic(t, verdict, ValidationUnderdetermined, DiagnosticTypeRuleUnavailable)
}

func TestValidateMemoryChangeSetMemberOfLookupUsesTheFullContextSlice(t *testing.T) {
	fixture := newValidationFixture(t)
	changedSlice := mustContextSliceBuild(t, ContextSliceInput{
		Context:   fixture.typeEnv.primaryContext.Ref(),
		GammaTime: mustContextSlicePoint(t, "2026-07-16T08:00:01Z"),
	})
	relation, err := NewRelationInstantiation(
		fixture.assertion,
		fixture.typeEnv.signature.Ref(),
		changedSlice,
		fixture.bindings,
		typeEnvTestProvenanceRef(t, "memory:relation-instantiation/changed-slice"),
	)
	if err != nil {
		t.Fatalf("NewRelationInstantiation(): %v", err)
	}

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		validationTestChangeSet(t, relation),
	)
	assertValidationDiagnostic(t, verdict, ValidationUnderdetermined, DiagnosticTypeRuleUnavailable)
}

func TestStaticKindCannotInheritTwoDisjointOperands(t *testing.T) {
	fixture := newValidationFixture(t)
	hybrid := typeEnvTestKindDefinition(t, "U.ImpossibleHybrid", fixture.typeEnv.provenance)
	entitySubkind, err := NewSubkindRelation(
		hybrid.ID(),
		fixture.typeEnv.entityKind.ID(),
		fixture.typeEnv.provenance,
	)
	if err != nil {
		t.Fatalf("NewSubkindRelation(entity): %v", err)
	}
	claimSubkind, err := NewSubkindRelation(
		hybrid.ID(),
		fixture.typeEnv.claimGraphKind.ID(),
		fixture.typeEnv.provenance,
	)
	if err != nil {
		t.Fatalf("NewSubkindRelation(claim): %v", err)
	}
	_, err = fixture.typeEnv.builder().
		AddKindDefinition(hybrid).
		AddSubkindRelation(entitySubkind).
		AddSubkindRelation(claimSubkind).
		Build()
	if err == nil {
		t.Fatal("TypeEnv accepted a kind below two mutually disjoint operands")
	}

	environment := fixture.environment
	environment.kinds = append(environment.kinds, hybrid)
	environment.subkinds = append(environment.subkinds, entitySubkind, claimSubkind)
	canonicalizeTypeEnv(&environment)
	accumulator := validationAccumulator{}
	hybridRef := typeEnvTestValueKindRef(t, environment.Ref(), hybrid.ID())
	if validateStaticKindDisjointness(
		&accumulator,
		environment,
		hybridRef,
		"typed_value.value_kind_ref",
	) {
		t.Fatal("runtime defense accepted a kind below two disjoint operands")
	}
	verdict := accumulator.verdict(
		MemoryChangeSet{},
		fixture.environment.Ref(),
		fixture.snapshot.GraphRevision(),
	)
	assertValidationDiagnostic(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
}

func TestValidateMemoryChangeSetRejectsRefKindMismatch(t *testing.T) {
	fixture := newValidationFixture(t)
	otherRefKind := typeEnvTestRefKindRef(
		t,
		fixture.environment.Ref(),
		typeEnvTestRefKindID(t, "U.OtherEntityRef"),
	)
	referenceID, err := NewReferenceID("entity:authorization-service")
	if err != nil {
		t.Fatalf("NewReferenceID(): %v", err)
	}
	wrongReference, err := NewPersistedRef(otherRefKind, referenceID)
	if err != nil {
		t.Fatalf("NewPersistedRef(): %v", err)
	}
	wrongFiller, err := NewByReferenceCandidate(wrongReference)
	if err != nil {
		t.Fatalf("NewByReferenceCandidate(): %v", err)
	}
	wrongBinding, err := NewCandidateSlotBinding(
		fixture.typeEnv.entitySlot,
		[]CandidateSlotFiller{wrongFiller},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding(): %v", err)
	}
	relation := validationTestRelation(
		t,
		fixture,
		fixture.typeEnv.signature.Ref(),
		[]CandidateSlotBinding{wrongBinding, fixture.claimBinding},
	)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		validationTestChangeSet(t, relation),
	)
	assertValidationDiagnostic(t, verdict, ValidationInvalid, DiagnosticReferenceKindMismatch)
}

func TestValidateMemoryChangeSetAcceptsSlotSubkindThroughRefKindSupertype(t *testing.T) {
	fixture := newValidationFixture(t)
	systemKind := typeEnvTestValueKindRef(
		t,
		fixture.environment.Ref(),
		fixture.typeEnv.systemKind.ID(),
	)
	target, err := NewReferenceSlotTarget(systemKind, fixture.typeEnv.entityRefKind)
	if err != nil {
		t.Fatalf("NewReferenceSlotTarget(): %v", err)
	}
	slotKind := typeEnvTestSlotKindID(t, "SystemOfConcernSlot")
	slot, err := NewSlotSpec(
		slotKind,
		target,
		ExactlyOneCardinality(),
		fixture.typeEnv.provenance,
	)
	if err != nil {
		t.Fatalf("NewSlotSpec(): %v", err)
	}
	signatureRef := typeEnvTestSignatureRef(
		t,
		fixture.environment.Ref(),
		"U.SystemOfConcernRelation",
	)
	signature, err := NewRelationSignature(
		signatureRef,
		[]BoundedContextRef{fixture.typeEnv.primaryContext.Ref()},
		[]SlotSpec{slot},
		fixture.typeEnv.provenance,
	)
	if err != nil {
		t.Fatalf("NewRelationSignature(): %v", err)
	}
	environment, err := fixture.typeEnv.builder().AddRelationSignature(signature).Build()
	if err != nil {
		t.Fatalf("TypeEnv Build(): %v", err)
	}
	fixture.environment = environment
	fixture.snapshot.typeEnv = environment.Ref()
	fillers := fixture.entityBinding.Fillers()
	binding, err := NewCandidateSlotBinding(slotKind, fillers)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding(): %v", err)
	}
	relation := validationTestRelation(
		t,
		fixture,
		signatureRef,
		[]CandidateSlotBinding{binding},
	)
	validationTestSetMemberOf(
		t,
		&fixture.snapshot,
		validationTestMemberOfMember(
			t,
			validationTestMemberOfQuery(
				t,
				fixture.referenceEntity,
				systemKind,
				relation.Slice(),
			),
			fixture.typeEnv.provenance,
		),
	)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		validationTestChangeSet(t, relation),
	)
	if _, ok := verdict.(Valid); !ok {
		t.Fatalf("subkind relation verdict = %T (%s); want Valid", verdict, verdict.Kind())
	}
}

func TestValidateMemoryChangeSetRejectsMismatchedResolutionBasis(t *testing.T) {
	fixture := newValidationFixture(t)
	otherID, err := NewReferenceID("entity:different-service")
	if err != nil {
		t.Fatalf("NewReferenceID(): %v", err)
	}
	otherReference, err := NewPersistedRef(fixture.typeEnv.entityRefKind, otherID)
	if err != nil {
		t.Fatalf("NewPersistedRef(): %v", err)
	}
	otherEntity := mustEntityID(t, "entity:different-service")
	resolution, err := NewResolvedStrongReference(
		otherReference,
		otherEntity,
		fixture.typeEnv.primaryContext.Ref(),
		mustResolutionBasisRef(t, "snapshot:mismatched-reference-resolution"),
	)
	if err != nil {
		t.Fatalf("NewResolvedStrongReference(): %v", err)
	}
	fixture.snapshot.referenceResolution = resolution

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	assertValidationDiagnostic(t, verdict, ValidationUnderdetermined, DiagnosticReferenceUnresolved)
}

func TestValidateMemoryChangeSetReportsMissingContextBridge(t *testing.T) {
	fixture := newValidationFixture(t)
	repair, err := NewRepairPointer("activate-an-exact-context-bridge")
	if err != nil {
		t.Fatalf("NewRepairPointer(): %v", err)
	}
	resolution, err := NewMissingContextBridgeResolution(
		fixture.reference,
		fixture.typeEnv.secondaryContext.Ref(),
		fixture.typeEnv.primaryContext.Ref(),
		fixture.typeEnv.systemKind.ID(),
		fixture.typeEnv.entityKind.ID(),
		repair,
	)
	if err != nil {
		t.Fatalf("NewMissingContextBridgeResolution(): %v", err)
	}
	fixture.snapshot.referenceResolution = resolution

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	assertValidationDiagnostic(t, verdict, ValidationUnderdetermined, DiagnosticContextBridgeMissing)
}

func TestValidateMemoryChangeSetRejectsSnapshotFromAnotherTypeEnv(t *testing.T) {
	fixture := newValidationFixture(t)
	fixture.snapshot.typeEnv = typeEnvTestTypeEnvRef(t, 0x7e)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	assertValidationDiagnostic(t, verdict, ValidationUnderdetermined, DiagnosticTypeRuleUnavailable)
}

func TestRelationDigestIsInvariantUnderNamedSlotPermutation(t *testing.T) {
	fixture := newValidationFixture(t)
	forward := validationTestRelation(
		t,
		fixture,
		fixture.typeEnv.signature.Ref(),
		[]CandidateSlotBinding{fixture.entityBinding, fixture.claimBinding},
	)
	reverse := validationTestRelation(
		t,
		fixture,
		fixture.typeEnv.signature.Ref(),
		[]CandidateSlotBinding{fixture.claimBinding, fixture.entityBinding},
	)
	forwardDigest := validationTestDigest(t, validationTestChangeSet(t, forward))
	reverseDigest := validationTestDigest(t, validationTestChangeSet(t, reverse))
	if forwardDigest != reverseDigest {
		t.Fatalf("slot permutation changed digest: %s != %s", forwardDigest.String(), reverseDigest.String())
	}
}

func TestValidatedDigestCollapsesSemanticallyEquivalentCodecInputs(t *testing.T) {
	fixture := newValidationFixture(t)
	nodeA, nodeB, nodeC, edgeAB, edgeBC := valueTestGraphParts(t)
	forwardGraph := valueTestClaimGraph(
		t,
		[]ClaimNode{nodeA, nodeB, nodeC},
		[]ClaimEdge{edgeAB, edgeBC},
	)
	reverseGraph := valueTestClaimGraph(
		t,
		[]ClaimNode{nodeC, nodeB, nodeA},
		[]ClaimEdge{edgeBC, edgeAB},
	)
	forwardInput := encodeClaimGraphInCallerOrder(t, forwardGraph)
	reverseInput := encodeClaimGraphInCallerOrder(t, reverseGraph)
	if bytes.Equal(forwardInput, reverseInput) {
		t.Fatal("test inputs are byte-identical; semantic normalization was not exercised")
	}
	codec, available := fixture.registry.Resolve(fixture.typeEnv.binding.Codec())
	if !available {
		t.Fatal("fixture codec is unavailable")
	}
	canonicalized, ok := codec.Canonicalize(
		fixture.typeEnv.binding.ValueShape(),
		reverseInput,
	).(CanonicalizedCodecValue)
	if !ok {
		t.Fatal("semantically valid reverse input did not canonicalize")
	}
	canonicalValueDigest := digestTypedValue(
		fixture.typeEnv.claimGraphValueKind,
		fixture.typeEnv.binding.ValueShape(),
		fixture.typeEnv.binding.Codec(),
		canonicalized.CanonicalBytes(),
	)
	assertedDigest, err := NewExactAssertedDigest(canonicalValueDigest)
	if err != nil {
		t.Fatalf("NewExactAssertedDigest: %v", err)
	}

	makeChangeSet := func(
		input []byte,
		asserted AssertedTypedValueDigest,
	) MemoryChangeSet {
		candidate, err := NewTypedValueCandidate(
			fixture.typeEnv.claimGraphValueKind,
			fixture.typeEnv.binding.ValueShape(),
			fixture.typeEnv.binding.Codec(),
			input,
			asserted,
		)
		if err != nil {
			t.Fatalf("NewTypedValueCandidate: %v", err)
		}
		filler, err := NewByValueCandidate(candidate)
		if err != nil {
			t.Fatalf("NewByValueCandidate: %v", err)
		}
		binding, err := NewCandidateSlotBinding(
			fixture.typeEnv.claimGraphSlot,
			[]CandidateSlotFiller{filler},
		)
		if err != nil {
			t.Fatalf("NewCandidateSlotBinding: %v", err)
		}
		relation := validationTestRelation(
			t,
			fixture,
			fixture.typeEnv.signature.Ref(),
			[]CandidateSlotBinding{fixture.entityBinding, binding},
		)
		return validationTestChangeSet(t, relation)
	}

	forwardSet := makeChangeSet(forwardInput, NoAssertedDigest{})
	reverseSet := makeChangeSet(reverseInput, assertedDigest)
	if validationTestDigest(t, forwardSet) == validationTestDigest(t, reverseSet) {
		t.Fatal("raw candidate digest hid the distinct input encodings")
	}

	forwardVerdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		forwardSet,
	)
	reverseVerdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		reverseSet,
	)
	forwardDigest := validationTestCanonicalDigest(t, forwardVerdict)
	reverseDigest := validationTestCanonicalDigest(t, reverseVerdict)
	if forwardDigest != reverseDigest {
		t.Fatalf(
			"semantically equivalent admitted values have different digests: %s != %s",
			forwardDigest.String(),
			reverseDigest.String(),
		)
	}
}

func TestFillerPermutationHasOneDigestAndOneAdmittedRepresentation(t *testing.T) {
	fixture := newValidationFixture(t)
	cardinality, err := NewBoundedCardinality(2, 2)
	if err != nil {
		t.Fatalf("NewBoundedCardinality: %v", err)
	}
	target, err := NewValueSlotTarget(fixture.typeEnv.claimGraphValueKind)
	if err != nil {
		t.Fatalf("NewValueSlotTarget: %v", err)
	}
	slot, err := NewSlotSpec(
		fixture.typeEnv.claimGraphSlot,
		target,
		cardinality,
		fixture.typeEnv.provenance,
	)
	if err != nil {
		t.Fatalf("NewSlotSpec: %v", err)
	}
	signatureRef := typeEnvTestSignatureRef(
		t,
		fixture.environment.Ref(),
		"test.TwoClaimGraphs",
	)
	signature, err := NewRelationSignature(
		signatureRef,
		[]BoundedContextRef{fixture.typeEnv.primaryContext.Ref()},
		[]SlotSpec{slot},
		fixture.typeEnv.provenance,
	)
	if err != nil {
		t.Fatalf("NewRelationSignature: %v", err)
	}
	environment, err := fixture.typeEnv.builder().
		AddRelationSignature(signature).
		Build()
	if err != nil {
		t.Fatalf("Build two-filler TypeEnv: %v", err)
	}

	emptyGraph, err := NewClaimGraphValue(nil, nil)
	if err != nil {
		t.Fatalf("NewClaimGraphValue(empty): %v", err)
	}
	nonEmptyGraph := valueTestGraph(t)
	makeFiller := func(graph ClaimGraphValue) CandidateSlotFiller {
		codec, ok := fixture.registry.Resolve(fixture.typeEnv.binding.Codec())
		if !ok {
			t.Fatal("fixture codec is unavailable")
		}
		claimCodec, ok := codec.(ClaimGraphCodecV1)
		if !ok {
			t.Fatalf("fixture codec = %T; want ClaimGraphCodecV1", codec)
		}
		result := claimCodec.EncodeInput(graph)
		canonical, ok := result.(CanonicalizedCodecValue)
		if !ok {
			t.Fatalf("EncodeInput = %T; want CanonicalizedCodecValue", result)
		}
		candidate, candidateErr := NewTypedValueCandidate(
			fixture.typeEnv.claimGraphValueKind,
			fixture.typeEnv.binding.ValueShape(),
			fixture.typeEnv.binding.Codec(),
			canonical.CanonicalBytes(),
			NoAssertedDigest{},
		)
		if candidateErr != nil {
			t.Fatalf("NewTypedValueCandidate: %v", candidateErr)
		}
		filler, fillerErr := NewByValueCandidate(candidate)
		if fillerErr != nil {
			t.Fatalf("NewByValueCandidate: %v", fillerErr)
		}
		return filler
	}
	first := makeFiller(emptyGraph)
	second := makeFiller(nonEmptyGraph)
	forwardBinding, err := NewCandidateSlotBinding(
		fixture.typeEnv.claimGraphSlot,
		[]CandidateSlotFiller{first, second},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding(forward): %v", err)
	}
	reverseBinding, err := NewCandidateSlotBinding(
		fixture.typeEnv.claimGraphSlot,
		[]CandidateSlotFiller{second, first},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding(reverse): %v", err)
	}
	forwardRelation := validationTestRelation(
		t,
		fixture,
		signature.Ref(),
		[]CandidateSlotBinding{forwardBinding},
	)
	reverseRelation := validationTestRelation(
		t,
		fixture,
		signature.Ref(),
		[]CandidateSlotBinding{reverseBinding},
	)
	forwardSet := validationTestChangeSet(t, forwardRelation)
	reverseSet := validationTestChangeSet(t, reverseRelation)
	if validationTestDigest(t, forwardSet) != validationTestDigest(t, reverseSet) {
		t.Fatal("filler permutation changed MemoryChangeSet digest")
	}

	forwardVerdict := ValidateMemoryChangeSet(
		environment,
		fixture.registry,
		fixture.snapshot,
		forwardSet,
	)
	reverseVerdict := ValidateMemoryChangeSet(
		environment,
		fixture.registry,
		fixture.snapshot,
		reverseSet,
	)
	forwardDigests := admittedValueFillerDigests(t, forwardVerdict)
	reverseDigests := admittedValueFillerDigests(t, reverseVerdict)
	if validationTestCanonicalDigest(t, forwardVerdict) != validationTestCanonicalDigest(t, reverseVerdict) {
		t.Fatal("filler permutation changed validated canonical digest")
	}
	if len(forwardDigests) != len(reverseDigests) {
		t.Fatal("filler permutation changed admitted filler count")
	}
	for index := range forwardDigests {
		if forwardDigests[index] != reverseDigests[index] {
			t.Fatal("filler permutation changed admitted RelationInstance order")
		}
	}
}

func encodeClaimGraphInCallerOrder(t *testing.T, value ClaimGraphValue) []byte {
	t.Helper()
	graph, ok := value.(claimGraphValue)
	if !ok {
		t.Fatalf("ClaimGraphValue = %T; want exact claimGraphValue", value)
	}
	writer := newCanonicalWriter(claimGraphCodecDomain)
	writer.addUint64(uint64(len(graph.nodes)))
	for _, node := range graph.nodes {
		encoded, issues := encodeClaimNode(node)
		if len(issues) > 0 {
			t.Fatalf("encode claim node: %s", issues[0].Message())
		}
		writer.addBytes(encoded)
	}
	writer.addUint64(uint64(len(graph.edges)))
	for _, edge := range graph.edges {
		writer.addBytes(encodeClaimEdge(edge))
	}
	return writer.bytes()
}

func validationTestCanonicalDigest(
	t *testing.T,
	verdict ValidationVerdict,
) SHA256Digest {
	t.Helper()
	valid, ok := verdict.(Valid)
	if !ok {
		t.Fatalf("verdict = %T (%s); want Valid", verdict, verdict.Kind())
	}
	digest := valid.SemanticChangeDigest()
	if !digest.valid() {
		t.Fatal("Valid verdict returned an empty canonical digest")
	}
	projected, err := valid.ChangeSet().CanonicalDigest()
	if err != nil {
		t.Fatalf("ValidatedMemoryChangeSet.CanonicalDigest: %v", err)
	}
	if projected != digest {
		t.Fatal("Valid verdict digest differs from its validated change-set digest")
	}
	return digest
}

func admittedValueFillerDigests(
	t *testing.T,
	verdict ValidationVerdict,
) []SHA256Digest {
	t.Helper()
	valid, ok := verdict.(Valid)
	if !ok {
		t.Fatalf("verdict = %T (%s); want Valid", verdict, verdict.Kind())
	}
	changes := valid.ChangeSet().Changes()
	if len(changes) != 1 {
		t.Fatalf("validated changes = %d; want 1", len(changes))
	}
	relation, ok := changes[0].(ValidatedRelationInstance)
	if !ok {
		t.Fatalf("validated change = %T; want ValidatedRelationInstance", changes[0])
	}
	bindings := relation.Relation().Bindings()
	if len(bindings) != 1 {
		t.Fatalf("validated bindings = %d; want 1", len(bindings))
	}
	fillers := bindings[0].Fillers()
	digests := make([]SHA256Digest, 0, len(fillers))
	for _, filler := range fillers {
		value, ok := filler.(ValueFiller)
		if !ok {
			t.Fatalf("validated filler = %T; want ValueFiller", filler)
		}
		digests = append(digests, value.Value().Digest())
	}
	return digests
}

func TestAliasCandidatesRemainUnderdeterminedAndNeverBecomeMergeAuthority(t *testing.T) {
	fixture := newValidationFixture(t)
	alias := mustEntityAlias(t, "authorization")
	candidateA := validationTestEntityCandidate(t, "entity:auth-service", alias, fixture.typeEnv.primaryContext.Ref())
	candidateB := validationTestEntityCandidate(t, "entity:auth-policy", alias, fixture.typeEnv.primaryContext.Ref())
	resolution, err := NewCandidateAliasResolution(
		alias,
		fixture.typeEnv.primaryContext.Ref(),
		[]EntityCandidate{candidateA, candidateB},
	)
	if err != nil {
		t.Fatalf("NewCandidateAliasResolution(): %v", err)
	}
	fixture.snapshot.aliasResolution = resolution
	change, err := NewAdmitAlias(
		mustEntityID(t, "entity:auth-service"),
		alias,
		fixture.typeEnv.primaryContext.Ref(),
		typeEnvTestProvenanceRef(t, "memory:alias-admission"),
	)
	if err != nil {
		t.Fatalf("NewAdmitAlias(): %v", err)
	}
	identityChange, err := NewApplyIdentityChange(change)
	if err != nil {
		t.Fatalf("NewApplyIdentityChange(): %v", err)
	}
	changeSet, err := NewMemoryChangeSet([]MemoryChange{identityChange})
	if err != nil {
		t.Fatalf("NewMemoryChangeSet(): %v", err)
	}

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	assertValidationDiagnostic(t, verdict, ValidationUnderdetermined, DiagnosticIdentityBasisMissing)
}

func TestMergeEntitiesRequiresManualReconciliationWithMissingBasis(t *testing.T) {
	fixture := newValidationFixture(t)
	contextRef := fixture.typeEnv.primaryContext.Ref()
	survivor := mustEntityID(t, "entity:canonical-auth")
	merged := []EntityID{
		mustEntityID(t, "entity:legacy-auth-a"),
		mustEntityID(t, "entity:legacy-auth-b"),
	}
	basis := mustReconciliationBasisRef(t, "reconciliation:auth-merge")
	fixture.snapshot.entityResolutions = validationTestExactEntityResolutions(
		t,
		contextRef,
		append([]EntityID{survivor}, merged...),
	)
	missing, err := NewMissingReconciliationBasis(basis, contextRef)
	if err != nil {
		t.Fatalf("NewMissingReconciliationBasis: %v", err)
	}
	fixture.snapshot.reconciliationBasisResolution = missing
	change, err := NewMergeEntities(survivor, merged, contextRef, basis)
	if err != nil {
		t.Fatalf("NewMergeEntities: %v", err)
	}
	changeSet := validationTestIdentityChangeSet(t, change)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	assertValidationDiagnostic(
		t,
		verdict,
		ValidationUnderdetermined,
		DiagnosticReconciliationBasisUnresolved,
	)
	assertSingleValidationDiagnostic(t, verdict, DiagnosticReconciliationBasisUnresolved)
	assertValidationRepair(
		t,
		verdict,
		DiagnosticReconciliationBasisUnresolved,
		"manual_identity_reconciliation_required",
	)
}

func TestMergeEntitiesIgnoresArbitraryBasisUntilManualReconciliation(t *testing.T) {
	fixture := newValidationFixture(t)
	contextRef := fixture.typeEnv.primaryContext.Ref()
	survivor := mustEntityID(t, "entity:canonical-auth")
	merged := []EntityID{mustEntityID(t, "entity:legacy-auth")}
	basis := mustReconciliationBasisRef(t, "reconciliation:auth-merge")
	fixture.snapshot.entityResolutions = validationTestExactEntityResolutions(
		t,
		contextRef,
		append([]EntityID{survivor}, merged...),
	)
	resolved, err := NewResolvedReconciliationBasis(
		basis,
		ReconciliationMergeEntities,
		contextRef,
		survivor,
		[]EntityID{mustEntityID(t, "entity:different-legacy-auth")},
		fixture.snapshot.GraphRevision(),
		fixture.snapshot.TypeEnvRef(),
		typeEnvTestDigest(t, 0x91),
		typeEnvTestProvenanceRef(t, "memory:auth-merge-review"),
	)
	if err != nil {
		t.Fatalf("NewResolvedReconciliationBasis: %v", err)
	}
	fixture.snapshot.reconciliationBasisResolution = resolved
	change, err := NewMergeEntities(survivor, merged, contextRef, basis)
	if err != nil {
		t.Fatalf("NewMergeEntities: %v", err)
	}
	changeSet := validationTestIdentityChangeSet(t, change)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	assertValidationDiagnostic(
		t,
		verdict,
		ValidationUnderdetermined,
		DiagnosticReconciliationBasisUnresolved,
	)
	assertSingleValidationDiagnostic(t, verdict, DiagnosticReconciliationBasisUnresolved)
	assertValidationRepair(
		t,
		verdict,
		DiagnosticReconciliationBasisUnresolved,
		"manual_identity_reconciliation_required",
	)
}

func TestMergeEntitiesRequiresManualReconciliationEvenWithExactSemanticBasis(t *testing.T) {
	fixture := newValidationFixture(t)
	contextRef := fixture.typeEnv.primaryContext.Ref()
	survivor := mustEntityID(t, "entity:canonical-auth")
	merged := []EntityID{mustEntityID(t, "entity:legacy-auth")}
	basis := mustReconciliationBasisRef(t, "reconciliation:auth-merge")
	fixture.snapshot.entityResolutions = validationTestExactEntityResolutions(
		t,
		contextRef,
		append([]EntityID{survivor}, merged...),
	)
	fixture.snapshot.reconciliationBasisResolution = validationTestResolvedReconciliationBasis(
		t,
		fixture.snapshot,
		basis,
		ReconciliationMergeEntities,
		contextRef,
		survivor,
		merged,
	)
	change, err := NewMergeEntities(survivor, merged, contextRef, basis)
	if err != nil {
		t.Fatalf("NewMergeEntities: %v", err)
	}
	changeSet := validationTestIdentityChangeSet(t, change)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	assertValidationDiagnostic(
		t,
		verdict,
		ValidationUnderdetermined,
		DiagnosticReconciliationBasisUnresolved,
	)
	assertValidationRepair(
		t,
		verdict,
		DiagnosticReconciliationBasisUnresolved,
		"manual_identity_reconciliation_required",
	)
}

func TestSplitEntityRequiresManualReconciliationEvenWithExactSemanticBasis(t *testing.T) {
	fixture := newValidationFixture(t)
	contextRef := fixture.typeEnv.primaryContext.Ref()
	source := mustEntityID(t, "entity:legacy-auth")
	targets := []EntityID{
		mustEntityID(t, "entity:auth-service"),
		mustEntityID(t, "entity:auth-policy"),
	}
	basis := mustReconciliationBasisRef(t, "reconciliation:auth-split")
	fixture.snapshot.entityResolutions = validationTestSplitEntityResolutions(
		t,
		contextRef,
		source,
		targets,
	)
	fixture.snapshot.reconciliationBasisResolution = validationTestResolvedReconciliationBasis(
		t,
		fixture.snapshot,
		basis,
		ReconciliationSplitEntity,
		contextRef,
		source,
		targets,
	)
	change, err := NewSplitEntity(source, targets, contextRef, basis)
	if err != nil {
		t.Fatalf("NewSplitEntity: %v", err)
	}
	changeSet := validationTestIdentityChangeSet(t, change)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	assertValidationDiagnostic(
		t,
		verdict,
		ValidationUnderdetermined,
		DiagnosticReconciliationBasisUnresolved,
	)
	assertValidationRepair(
		t,
		verdict,
		DiagnosticReconciliationBasisUnresolved,
		"manual_identity_reconciliation_required",
	)
}

func TestDeclareEntityDoesNotRequireReconciliationBasis(t *testing.T) {
	fixture := newValidationFixture(t)
	contextRef := fixture.typeEnv.primaryContext.Ref()
	entity := mustEntityID(t, "entity:new-service")
	absent, err := NewAbsentEntityResolution(
		entity,
		contextRef,
		mustResolutionBasisRef(t, "snapshot:known-absent"),
	)
	if err != nil {
		t.Fatalf("NewAbsentEntityResolution: %v", err)
	}
	fixture.snapshot.entityResolution = absent
	fixture.snapshot.reconciliationBasisResolution = nil
	changeSet := validationTestDeclareEntityChangeSet(t, entity, contextRef)

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	if _, ok := verdict.(Valid); !ok {
		t.Fatalf("DeclareEntity verdict = %T (%s); want Valid", verdict, verdict.Kind())
	}
}

func TestAliasAdmissionRequiresExactEntityAndKnownUnboundAlias(t *testing.T) {
	fixture := newValidationFixture(t)
	entity := mustEntityID(t, "entity:auth-service")
	alias := mustEntityAlias(t, "authorization service")
	basis := mustResolutionBasisRef(t, "snapshot:identity-index")
	exact, err := NewExactEntityResolution(
		entity,
		fixture.typeEnv.primaryContext.Ref(),
		basis,
	)
	if err != nil {
		t.Fatalf("NewExactEntityResolution(): %v", err)
	}
	unbound, err := NewUnboundAliasResolution(
		alias,
		fixture.typeEnv.primaryContext.Ref(),
		basis,
	)
	if err != nil {
		t.Fatalf("NewUnboundAliasResolution(): %v", err)
	}
	fixture.snapshot.entityResolution = exact
	fixture.snapshot.aliasResolution = unbound
	change, err := NewAdmitAlias(
		entity,
		alias,
		fixture.typeEnv.primaryContext.Ref(),
		typeEnvTestProvenanceRef(t, "memory:alias-admission"),
	)
	if err != nil {
		t.Fatalf("NewAdmitAlias(): %v", err)
	}
	effect, err := NewApplyIdentityChange(change)
	if err != nil {
		t.Fatalf("NewApplyIdentityChange(): %v", err)
	}
	changeSet, err := NewMemoryChangeSet([]MemoryChange{effect})
	if err != nil {
		t.Fatalf("NewMemoryChangeSet(): %v", err)
	}

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	if _, ok := verdict.(Valid); !ok {
		t.Fatalf("alias admission verdict = %T (%s); want Valid", verdict, verdict.Kind())
	}

	bound, err := NewBoundAliasResolution(
		alias,
		entity,
		fixture.typeEnv.primaryContext.Ref(),
		basis,
	)
	if err != nil {
		t.Fatalf("NewBoundAliasResolution(): %v", err)
	}
	fixture.snapshot.aliasResolution = bound
	verdict = ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	assertValidationDiagnostic(t, verdict, ValidationInvalid, DiagnosticAliasAlreadyBound)
}

func TestAliasAdmissionCanTargetPriorSameBatchDeclaration(t *testing.T) {
	fixture := newValidationFixture(t)
	entity := mustEntityID(t, "entity:first-concern")
	contextRef := fixture.typeEnv.primaryContext.Ref()
	alias := mustEntityAlias(t, "first concern")
	basis := mustResolutionBasisRef(t, "snapshot:first-concern-identity")
	absent, err := NewAbsentEntityResolution(entity, contextRef, basis)
	if err != nil {
		t.Fatalf("NewAbsentEntityResolution: %v", err)
	}
	unbound, err := NewUnboundAliasResolution(alias, contextRef, basis)
	if err != nil {
		t.Fatalf("NewUnboundAliasResolution: %v", err)
	}
	fixture.snapshot.entityResolution = absent
	fixture.snapshot.aliasResolution = unbound

	declarationSet := validationTestDeclareEntityChangeSet(
		t,
		entity,
		contextRef,
	)
	declaration := declarationSet.Changes()[0]
	aliasChange, err := NewAdmitAlias(
		entity,
		alias,
		contextRef,
		typeEnvTestProvenanceRef(t, "memory:first-concern-alias"),
	)
	if err != nil {
		t.Fatalf("NewAdmitAlias: %v", err)
	}
	aliasEffect, err := NewApplyIdentityChange(aliasChange)
	if err != nil {
		t.Fatalf("NewApplyIdentityChange: %v", err)
	}
	changeSet, err := NewMemoryChangeSet(
		[]MemoryChange{declaration, aliasEffect},
	)
	if err != nil {
		t.Fatalf("NewMemoryChangeSet: %v", err)
	}

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	valid, ok := verdict.(Valid)
	if !ok {
		t.Fatalf(
			"same-batch declaration+alias verdict = %T (%s); want Valid",
			verdict,
			verdict.Kind(),
		)
	}
	validated := valid.ChangeSet().Changes()
	if len(validated) != 2 {
		t.Fatalf("validated changes = %d; want 2", len(validated))
	}
	if _, ok := validated[0].(ValidatedDeclareEntity); !ok {
		t.Fatalf(
			"validated change[0] = %T; want ValidatedDeclareEntity",
			validated[0],
		)
	}
	identity, ok := validated[1].(ValidatedIdentityChange)
	if !ok {
		t.Fatalf(
			"validated change[1] = %T; want ValidatedIdentityChange",
			validated[1],
		)
	}
	admitted, ok := identity.Change().(AdmitAlias)
	if !ok || admitted.Entity() != entity || admitted.Alias() != alias {
		t.Fatalf("validated alias change = %#v; want exact requested alias", identity.Change())
	}
}

func TestAliasAdmissionRejectsDeclarationThatFollowsItInSameBatch(t *testing.T) {
	fixture := newValidationFixture(t)
	entity := mustEntityID(t, "entity:late-concern")
	contextRef := fixture.typeEnv.primaryContext.Ref()
	alias := mustEntityAlias(t, "late concern")
	basis := mustResolutionBasisRef(t, "snapshot:late-concern-identity")
	absent, err := NewAbsentEntityResolution(entity, contextRef, basis)
	if err != nil {
		t.Fatalf("NewAbsentEntityResolution: %v", err)
	}
	unbound, err := NewUnboundAliasResolution(alias, contextRef, basis)
	if err != nil {
		t.Fatalf("NewUnboundAliasResolution: %v", err)
	}
	fixture.snapshot.entityResolution = absent
	fixture.snapshot.aliasResolution = unbound

	declarationSet := validationTestDeclareEntityChangeSet(
		t,
		entity,
		contextRef,
	)
	declaration := declarationSet.Changes()[0]
	aliasChange, err := NewAdmitAlias(
		entity,
		alias,
		contextRef,
		typeEnvTestProvenanceRef(t, "memory:late-concern-alias"),
	)
	if err != nil {
		t.Fatalf("NewAdmitAlias: %v", err)
	}
	aliasEffect, err := NewApplyIdentityChange(aliasChange)
	if err != nil {
		t.Fatalf("NewApplyIdentityChange: %v", err)
	}
	changeSet, err := NewMemoryChangeSet(
		[]MemoryChange{aliasEffect, declaration},
	)
	if err != nil {
		t.Fatalf("NewMemoryChangeSet: %v", err)
	}

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	assertValidationDiagnostic(
		t,
		verdict,
		ValidationUnderdetermined,
		DiagnosticIdentityBasisMissing,
	)
}

func TestAliasSupersessionTreatsUncorrelatedAvailabilityAsMissingBasis(t *testing.T) {
	fixture := newValidationFixture(t)
	entity := mustEntityID(t, "entity:alias-owner")
	contextRef := fixture.typeEnv.primaryContext.Ref()
	basis := mustResolutionBasisRef(t, "snapshot:alias-supersession")
	exactEntity, err := NewExactEntityResolution(entity, contextRef, basis)
	if err != nil {
		t.Fatalf("NewExactEntityResolution: %v", err)
	}
	fixture.snapshot.entityResolution = exactEntity
	wrongUnbound, err := NewUnboundAliasResolution(
		mustEntityAlias(t, "different alias"),
		contextRef,
		basis,
	)
	if err != nil {
		t.Fatalf("NewUnboundAliasResolution: %v", err)
	}
	fixture.snapshot.aliasResolution = wrongUnbound
	change, err := NewSupersedeAlias(
		entity,
		mustEntityAlias(t, "old alias"),
		mustEntityAlias(t, "replacement alias"),
		contextRef,
		typeEnvTestProvenanceRef(t, "memory:alias-supersession"),
	)
	if err != nil {
		t.Fatalf("NewSupersedeAlias: %v", err)
	}
	effect, err := NewApplyIdentityChange(change)
	if err != nil {
		t.Fatalf("NewApplyIdentityChange: %v", err)
	}
	changeSet, err := NewMemoryChangeSet([]MemoryChange{effect})
	if err != nil {
		t.Fatalf("NewMemoryChangeSet: %v", err)
	}

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	assertValidationDiagnostic(t, verdict, ValidationUnderdetermined, DiagnosticIdentityBasisMissing)
}

func TestCrossContextBatchLocalReferenceCannotBypassBridgeBasis(t *testing.T) {
	fixture := newValidationFixture(t)
	secondary := fixture.typeEnv.secondaryContext.Ref()
	secondaryEntityAvailability := typeEnvTestKindAvailability(
		secondary,
		fixture.typeEnv.entityKind.ID(),
		fixture.typeEnv.provenance,
	)
	localSignatureRef := typeEnvTestSignatureRef(
		t,
		fixture.environment.Ref(),
		"test.CrossContextLocalReference",
	)
	entityTarget, err := NewReferenceSlotTarget(
		fixture.typeEnv.entityValueKind,
		fixture.typeEnv.entityRefKind,
	)
	if err != nil {
		t.Fatalf("NewReferenceSlotTarget: %v", err)
	}
	entitySlot, err := NewSlotSpec(
		fixture.typeEnv.entitySlot,
		entityTarget,
		ExactlyOneCardinality(),
		fixture.typeEnv.provenance,
	)
	if err != nil {
		t.Fatalf("NewSlotSpec: %v", err)
	}
	localSignature, err := NewRelationSignature(
		localSignatureRef,
		[]BoundedContextRef{secondary},
		[]SlotSpec{entitySlot},
		fixture.typeEnv.provenance,
	)
	if err != nil {
		t.Fatalf("NewRelationSignature: %v", err)
	}
	environment, err := fixture.typeEnv.builder().
		AddContextKindAvailability(secondaryEntityAvailability).
		AddRelationSignature(localSignature).
		Build()
	if err != nil {
		t.Fatalf("Build cross-context TypeEnv: %v", err)
	}

	entity := mustEntityID(t, "entity:batch-local-cross-context")
	localID, err := NewBatchLocalRef("local:batch-local-cross-context")
	if err != nil {
		t.Fatalf("NewBatchLocalRef: %v", err)
	}
	localReference, err := NewLocalRef(fixture.typeEnv.entityRefKind, localID)
	if err != nil {
		t.Fatalf("NewLocalRef: %v", err)
	}
	localCandidate, err := NewByReferenceCandidate(localReference)
	if err != nil {
		t.Fatalf("NewByReferenceCandidate: %v", err)
	}
	localBinding, err := NewCandidateSlotBinding(
		fixture.typeEnv.entitySlot,
		[]CandidateSlotFiller{localCandidate},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding: %v", err)
	}
	assertion, err := NewAssertionID("assertion:batch-local-cross-context")
	if err != nil {
		t.Fatalf("NewAssertionID: %v", err)
	}
	relation, err := NewRelationInstantiation(
		assertion,
		localSignature.Ref(),
		validationTestContextSlice(t, secondary),
		[]CandidateSlotBinding{localBinding},
		typeEnvTestProvenanceRef(t, "memory:cross-context-local-relation"),
	)
	if err != nil {
		t.Fatalf("NewRelationInstantiation: %v", err)
	}
	label, err := NewEntityLabel("Cross-context local entity")
	if err != nil {
		t.Fatalf("NewEntityLabel: %v", err)
	}
	declaration, err := NewDeclareEntity(
		entity,
		localID,
		fixture.typeEnv.primaryContext.Ref(),
		label,
		typeEnvTestProvenanceRef(t, "memory:cross-context-local-declaration"),
	)
	if err != nil {
		t.Fatalf("NewDeclareEntity: %v", err)
	}
	instantiation, err := NewInstantiateRelation(relation)
	if err != nil {
		t.Fatalf("NewInstantiateRelation: %v", err)
	}
	changeSet, err := NewMemoryChangeSet([]MemoryChange{declaration, instantiation})
	if err != nil {
		t.Fatalf("NewMemoryChangeSet: %v", err)
	}

	rule := validationTestRule(t)
	absentEntity, err := NewAbsentEntityResolution(
		entity,
		fixture.typeEnv.primaryContext.Ref(),
		mustResolutionBasisRef(t, "snapshot:cross-context-entity-absent"),
	)
	if err != nil {
		t.Fatalf("NewAbsentEntityResolution: %v", err)
	}
	absentAssertion, err := NewAbsentAssertionState(assertion, rule)
	if err != nil {
		t.Fatalf("NewAbsentAssertionState: %v", err)
	}
	fixture.snapshot.typeEnv = environment.Ref()
	fixture.snapshot.entityResolution = absentEntity
	fixture.snapshot.assertionStates[assertion.String()] = absentAssertion

	verdict := ValidateMemoryChangeSet(
		environment,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	assertValidationDiagnostic(t, verdict, ValidationUnderdetermined, DiagnosticContextBridgeMissing)
}

func TestUndeclaredBatchLocalReferenceCannotResolveThroughSnapshot(t *testing.T) {
	fixture := newValidationFixture(t)
	localID, err := NewBatchLocalRef("local:undeclared")
	if err != nil {
		t.Fatalf("NewBatchLocalRef: %v", err)
	}
	localReference, err := NewLocalRef(fixture.typeEnv.entityRefKind, localID)
	if err != nil {
		t.Fatalf("NewLocalRef: %v", err)
	}
	localCandidate, err := NewByReferenceCandidate(localReference)
	if err != nil {
		t.Fatalf("NewByReferenceCandidate: %v", err)
	}
	localBinding, err := NewCandidateSlotBinding(
		fixture.typeEnv.entitySlot,
		[]CandidateSlotFiller{localCandidate},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding: %v", err)
	}
	relation := validationTestRelation(
		t,
		fixture,
		fixture.typeEnv.signature.Ref(),
		[]CandidateSlotBinding{localBinding, fixture.claimBinding},
	)
	resolved, err := NewResolvedStrongReference(
		localReference,
		mustEntityID(t, "entity:undeclared-local-reference"),
		fixture.typeEnv.primaryContext.Ref(),
		mustResolutionBasisRef(t, "snapshot:undeclared-local-reference-resolution"),
	)
	if err != nil {
		t.Fatalf("NewResolvedStrongReference: %v", err)
	}
	fixture.snapshot.referenceResolution = resolved

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		validationTestChangeSet(t, relation),
	)
	assertValidationDiagnostic(t, verdict, ValidationUnderdetermined, DiagnosticReferenceUnresolved)
}

func TestDeclareEntityRequiresKnownAbsenceRatherThanUnsettledIdentity(t *testing.T) {
	fixture := newValidationFixture(t)
	entity := mustEntityID(t, "entity:new-service")
	changeSet := validationTestDeclareEntityChangeSet(t, entity, fixture.typeEnv.primaryContext.Ref())

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	assertValidationDiagnostic(t, verdict, ValidationUnderdetermined, DiagnosticIdentityBasisMissing)

	absent, err := NewAbsentEntityResolution(
		entity,
		fixture.typeEnv.primaryContext.Ref(),
		mustResolutionBasisRef(t, "snapshot:known-absent"),
	)
	if err != nil {
		t.Fatalf("NewAbsentEntityResolution(): %v", err)
	}
	fixture.snapshot.entityResolution = absent
	verdict = ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		changeSet,
	)
	if _, ok := verdict.(Valid); !ok {
		t.Fatalf("known-absent declaration verdict = %T (%s); want Valid", verdict, verdict.Kind())
	}
}

type validationFixture struct {
	typeEnv         typeEnvFixture
	environment     TypeEnv
	registry        CodecRegistry
	snapshot        validationSnapshot
	assertion       AssertionID
	reference       PersistedRef
	referenceEntity EntityID
	valueFiller     ByValueCandidate
	entityBinding   CandidateSlotBinding
	claimBinding    CandidateSlotBinding
	bindings        []CandidateSlotBinding
	changeSet       MemoryChangeSet
}

func newValidationFixture(t *testing.T) validationFixture {
	t.Helper()
	typeEnv := newTypeEnvFixture(t)
	environment := typeEnv.build(t)
	codec, err := NewClaimGraphCodecV1(typeEnv.binding.ValueShape())
	if err != nil {
		t.Fatalf("NewClaimGraphCodecV1(): %v", err)
	}
	registry, err := NewCodecRegistry().Register(typeEnv.binding.Codec(), codec)
	if err != nil {
		t.Fatalf("CodecRegistry.Register(): %v", err)
	}
	graphBytes := valueTestEncodedGraph(t, codec, valueTestGraph(t))
	valueCandidate, err := NewTypedValueCandidate(
		typeEnv.claimGraphValueKind,
		typeEnv.binding.ValueShape(),
		typeEnv.binding.Codec(),
		graphBytes,
		NoAssertedDigest{},
	)
	if err != nil {
		t.Fatalf("NewTypedValueCandidate(): %v", err)
	}
	valueFiller, err := NewByValueCandidate(valueCandidate)
	if err != nil {
		t.Fatalf("NewByValueCandidate(): %v", err)
	}
	referenceID, err := NewReferenceID("entity:authorization-service")
	if err != nil {
		t.Fatalf("NewReferenceID(): %v", err)
	}
	referenceEntity := mustEntityID(t, "entity:authorization-service")
	reference, err := NewPersistedRef(typeEnv.entityRefKind, referenceID)
	if err != nil {
		t.Fatalf("NewPersistedRef(): %v", err)
	}
	referenceFiller, err := NewByReferenceCandidate(reference)
	if err != nil {
		t.Fatalf("NewByReferenceCandidate(): %v", err)
	}
	entityBinding, err := NewCandidateSlotBinding(
		typeEnv.entitySlot,
		[]CandidateSlotFiller{referenceFiller},
	)
	if err != nil {
		t.Fatalf("entity NewCandidateSlotBinding(): %v", err)
	}
	claimBinding, err := NewCandidateSlotBinding(
		typeEnv.claimGraphSlot,
		[]CandidateSlotFiller{valueFiller},
	)
	if err != nil {
		t.Fatalf("claim NewCandidateSlotBinding(): %v", err)
	}
	assertion, err := NewAssertionID("assertion:episteme-slot-relation")
	if err != nil {
		t.Fatalf("NewAssertionID(): %v", err)
	}
	rule := validationTestRule(t)
	resolved, err := NewResolvedStrongReference(
		reference,
		referenceEntity,
		typeEnv.primaryContext.Ref(),
		mustResolutionBasisRef(t, "snapshot:reference-resolution-index"),
	)
	if err != nil {
		t.Fatalf("NewResolvedStrongReference(): %v", err)
	}
	absent, err := NewAbsentAssertionState(assertion, rule)
	if err != nil {
		t.Fatalf("NewAbsentAssertionState(): %v", err)
	}
	missing, err := NewMissingBasis("identity has not been declared")
	if err != nil {
		t.Fatalf("NewMissingBasis(): %v", err)
	}
	unsettled, err := NewUnsettledEntityResolution("unknown entity", []MissingBasis{missing})
	if err != nil {
		t.Fatalf("NewUnsettledEntityResolution(): %v", err)
	}
	unsettledAlias, err := NewUnsettledAliasResolution(
		mustEntityAlias(t, "unresolved alias"),
		typeEnv.primaryContext.Ref(),
		[]MissingBasis{missing},
	)
	if err != nil {
		t.Fatalf("NewUnsettledAliasResolution(): %v", err)
	}
	snapshot := validationSnapshot{
		revision:                     NewGraphRevision(7),
		typeEnv:                      environment.Ref(),
		entityResolution:             unsettled,
		referenceResolution:          resolved,
		memberOfJudgements:           map[string]MemberOfJudgement{},
		memberOfRequestJudgements:    map[string]MemberOfJudgement{},
		kindClassificationJudgements: map[string]KindClassificationJudgement{},
		assertionStates:              map[string]AssertionState{assertion.String(): absent},
		aliasResolution:              unsettledAlias,
	}
	partial := validationFixture{
		typeEnv:         typeEnv,
		environment:     environment,
		registry:        registry,
		snapshot:        snapshot,
		assertion:       assertion,
		reference:       reference,
		referenceEntity: referenceEntity,
		valueFiller:     valueFiller,
		entityBinding:   entityBinding,
		claimBinding:    claimBinding,
		bindings:        []CandidateSlotBinding{entityBinding, claimBinding},
	}
	relation := validationTestRelation(t, partial, typeEnv.signature.Ref(), partial.bindings)
	validationTestSetMemberOf(
		t,
		&partial.snapshot,
		validationTestMemberOfMember(
			t,
			validationTestMemberOfQuery(t, referenceEntity, typeEnv.entityValueKind, relation.Slice()),
			typeEnv.provenance,
		),
	)
	validationTestSetMemberOf(
		t,
		&partial.snapshot,
		validationTestMemberOfNotMember(
			t,
			validationTestMemberOfQuery(t, referenceEntity, typeEnv.claimGraphValueKind, relation.Slice()),
			typeEnv.provenance,
		),
	)
	partial.snapshot.memberOfEvaluator = func(request MemberOfEvaluationRequest) MemberOfJudgement {
		if _, prospective := request.View().(ProspectiveBatchView); !prospective {
			return nil
		}
		if request.Query().ValueKind() == typeEnv.entityValueKind {
			return validationTestMemberOfMemberWithView(
				t,
				request.Query(),
				typeEnv.provenance,
				request.View(),
			)
		}
		if request.Query().ValueKind() == typeEnv.claimGraphValueKind {
			return validationTestMemberOfNotMemberWithView(
				t,
				request.Query(),
				typeEnv.provenance,
				request.View(),
			)
		}
		return nil
	}
	partial.changeSet = validationTestChangeSet(t, relation)
	return partial
}

func validationTestRelation(
	t *testing.T,
	fixture validationFixture,
	signature RelationSignatureRef,
	bindings []CandidateSlotBinding,
) RelationInstantiation {
	t.Helper()
	relation, err := NewRelationInstantiation(
		fixture.assertion,
		signature,
		validationTestContextSlice(t, fixture.typeEnv.primaryContext.Ref()),
		bindings,
		typeEnvTestProvenanceRef(t, "memory:relation-instantiation"),
	)
	if err != nil {
		t.Fatalf("NewRelationInstantiation(): %v", err)
	}
	return relation
}

func validationTestContextSlice(t *testing.T, context BoundedContextRef) ContextSlice {
	t.Helper()
	return mustContextSliceBuild(t, ContextSliceInput{
		Context:   context,
		GammaTime: mustContextSlicePoint(t, "2026-07-16T08:00:00Z"),
	})
}

func validationTestChangeSet(t *testing.T, relation RelationInstantiation) MemoryChangeSet {
	t.Helper()
	change, err := NewInstantiateRelation(relation)
	if err != nil {
		t.Fatalf("NewInstantiateRelation(): %v", err)
	}
	changeSet, err := NewMemoryChangeSet([]MemoryChange{change})
	if err != nil {
		t.Fatalf("NewMemoryChangeSet(): %v", err)
	}
	return changeSet
}

func validationTestDeclareEntityChangeSet(
	t *testing.T,
	entity EntityID,
	context BoundedContextRef,
) MemoryChangeSet {
	t.Helper()
	localRef, err := NewBatchLocalRef("local:new-service")
	if err != nil {
		t.Fatalf("NewBatchLocalRef(): %v", err)
	}
	label, err := NewEntityLabel("New service")
	if err != nil {
		t.Fatalf("NewEntityLabel(): %v", err)
	}
	declaration, err := NewDeclareEntity(
		entity,
		localRef,
		context,
		label,
		typeEnvTestProvenanceRef(t, "memory:declare-entity"),
	)
	if err != nil {
		t.Fatalf("NewDeclareEntity(): %v", err)
	}
	changeSet, err := NewMemoryChangeSet([]MemoryChange{declaration})
	if err != nil {
		t.Fatalf("NewMemoryChangeSet(): %v", err)
	}
	return changeSet
}

func validationTestIdentityChangeSet(
	t *testing.T,
	change IdentityChange,
) MemoryChangeSet {
	t.Helper()
	effect, err := NewApplyIdentityChange(change)
	if err != nil {
		t.Fatalf("NewApplyIdentityChange: %v", err)
	}
	changeSet, err := NewMemoryChangeSet([]MemoryChange{effect})
	if err != nil {
		t.Fatalf("NewMemoryChangeSet: %v", err)
	}
	return changeSet
}

func validationTestExactEntityResolutions(
	t *testing.T,
	context BoundedContextRef,
	entities []EntityID,
) map[string]EntityResolution {
	t.Helper()
	result := make(map[string]EntityResolution, len(entities))
	basis := mustResolutionBasisRef(t, "snapshot:exact-identity-index")
	for _, entity := range entities {
		resolution, err := NewExactEntityResolution(entity, context, basis)
		if err != nil {
			t.Fatalf("NewExactEntityResolution(%s): %v", entity.String(), err)
		}
		result[entityResolutionTestKey(entity, context)] = resolution
	}
	return result
}

func validationTestSplitEntityResolutions(
	t *testing.T,
	context BoundedContextRef,
	source EntityID,
	targets []EntityID,
) map[string]EntityResolution {
	t.Helper()
	result := validationTestExactEntityResolutions(t, context, []EntityID{source})
	basis := mustResolutionBasisRef(t, "snapshot:exact-identity-absence-index")
	for _, target := range targets {
		resolution, err := NewAbsentEntityResolution(target, context, basis)
		if err != nil {
			t.Fatalf("NewAbsentEntityResolution(%s): %v", target.String(), err)
		}
		result[entityResolutionTestKey(target, context)] = resolution
	}
	return result
}

func validationTestResolvedReconciliationBasis(
	t *testing.T,
	snapshot validationSnapshot,
	basis ReconciliationBasisRef,
	operation IdentityReconciliationOperation,
	context BoundedContextRef,
	primary EntityID,
	related []EntityID,
) ResolvedReconciliationBasis {
	t.Helper()
	resolution, err := NewResolvedReconciliationBasis(
		basis,
		operation,
		context,
		primary,
		related,
		snapshot.GraphRevision(),
		snapshot.TypeEnvRef(),
		typeEnvTestDigest(t, 0x92),
		typeEnvTestProvenanceRef(t, "memory:identity-reconciliation-review"),
	)
	if err != nil {
		t.Fatalf("NewResolvedReconciliationBasis: %v", err)
	}
	return resolution
}

func assertValidatedIdentityChange(
	t *testing.T,
	verdict ValidationVerdict,
	want IdentityChange,
) {
	t.Helper()
	valid, ok := verdict.(Valid)
	if !ok {
		t.Fatalf("verdict = %T (%s); want Valid", verdict, verdict.Kind())
	}
	changes := valid.ChangeSet().Changes()
	if len(changes) != 1 {
		t.Fatalf("validated changes = %d; want 1", len(changes))
	}
	identity, ok := changes[0].(ValidatedIdentityChange)
	if !ok {
		t.Fatalf("validated change = %T; want ValidatedIdentityChange", changes[0])
	}
	if !reflect.DeepEqual(identity.Change(), want) {
		t.Fatalf("validated identity change = %#v; want %#v", identity.Change(), want)
	}
}

func validationTestDigest(t *testing.T, changeSet MemoryChangeSet) SHA256Digest {
	t.Helper()
	digest, err := changeSet.Digest()
	if err != nil {
		t.Fatalf("MemoryChangeSet.Digest(): %v", err)
	}
	return digest
}

func validationTestRule(t *testing.T) RuleRef {
	t.Helper()
	rule, err := NewRuleRef("test:typed-memory-rule")
	if err != nil {
		t.Fatalf("NewRuleRef(): %v", err)
	}
	return rule
}

func validationTestMemberOfQuery(
	t *testing.T,
	entity EntityID,
	valueKind ValueKindRef,
	contextSlice ContextSlice,
) MemberOfQuery {
	t.Helper()
	query, err := NewMemberOfQuery(entity, valueKind, contextSlice)
	if err != nil {
		t.Fatalf("NewMemberOfQuery(): %v", err)
	}
	return query
}

func validationFixtureMemberOfQuery(
	t *testing.T,
	fixture validationFixture,
	valueKind ValueKindRef,
) MemberOfQuery {
	t.Helper()
	return validationTestMemberOfQuery(
		t,
		fixture.referenceEntity,
		valueKind,
		validationTestContextSlice(t, fixture.typeEnv.primaryContext.Ref()),
	)
}

func validationTestMemberOfBasis(
	t *testing.T,
	query MemberOfQuery,
	provenance DeclarationProvenance,
) MemberOfBasis {
	t.Helper()
	evaluationView, err := NewPersistedSnapshotView(query.ValueKind().TypeEnv(), NewGraphRevision(7))
	if err != nil {
		t.Fatalf("NewPersistedSnapshotView(): %v", err)
	}
	return validationTestMemberOfBasisWithView(t, query, provenance, evaluationView)
}

func validationTestMemberOfBasisWithView(
	t *testing.T,
	query MemberOfQuery,
	provenance DeclarationProvenance,
	evaluationView MemberOfEvaluationView,
) MemberOfBasis {
	t.Helper()
	policy := EntitySetCandidatePolicy(PersistedEntitiesOnly{})
	if _, prospective := evaluationView.(ProspectiveBatchView); prospective {
		visible, err := NewPriorBatchDeclarationsVisible(typeEnvTestRuleRef(t, "test:entity-set/candidate-evaluation/v1"))
		if err != nil {
			t.Fatalf("NewPriorBatchDeclarationsVisible(): %v", err)
		}
		policy = visible
	}
	entitySet := typeEnvTestEntitySetDefinitionWithPolicy(
		t,
		query.ValueKind().TypeEnv(),
		query.ContextSlice().Context(),
		"test:entity-set/validation/v1",
		policy,
		provenance,
	)
	signature := typeEnvTestKindSignatureDefinition(
		t,
		query.ValueKind(),
		SignatureF4,
		nil,
		"test:member-of/validation/v1",
		entitySet.Ref(),
		provenance,
	)
	evaluationProvenance, err := NewMemberOfEvaluationProvenance(
		MemberOfEvaluationProvenanceInput{
			Reference:         typeEnvTestProvenanceRef(t, "prov:member-of/validation/v1"),
			EvaluatorArtifact: typeEnvTestCarrierRef(t, "binary:member-of-validation-evaluator"),
			EvaluatorEdition:  typeEnvTestCarrierEdition(t, "build-20260716.1"),
			EvaluatorDigest:   typeEnvTestDigest(t, 0xb7),
		},
	)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationProvenance(): %v", err)
	}
	inputRef, err := NewObservableInputRef("observable:member-of/validation-index")
	if err != nil {
		t.Fatalf("NewObservableInputRef(): %v", err)
	}
	input, err := NewMemberOfObservableInput(inputRef, typeEnvTestDigest(t, 0xb8))
	if err != nil {
		t.Fatalf("NewMemberOfObservableInput(): %v", err)
	}
	basis, err := NewMemberOfBasis(MemberOfBasisInput{
		Query:                query,
		EvaluationView:       evaluationView,
		KindSignature:        signature,
		EntitySet:            entitySet,
		ObservableInputs:     []MemberOfObservableInput{input},
		EvaluationProvenance: evaluationProvenance,
	})
	if err != nil {
		t.Fatalf("NewMemberOfBasis(): %v", err)
	}
	return basis
}

func validationTestMemberOfMember(
	t *testing.T,
	query MemberOfQuery,
	provenance DeclarationProvenance,
) MemberOfMember {
	t.Helper()
	judgement, err := NewMemberOfMember(query, validationTestMemberOfBasis(t, query, provenance))
	if err != nil {
		t.Fatalf("NewMemberOfMember(): %v", err)
	}
	return judgement
}

func validationTestMemberOfMemberWithView(
	t *testing.T,
	query MemberOfQuery,
	provenance DeclarationProvenance,
	evaluationView MemberOfEvaluationView,
) MemberOfMember {
	t.Helper()
	judgement, err := NewMemberOfMember(
		query,
		validationTestMemberOfBasisWithView(t, query, provenance, evaluationView),
	)
	if err != nil {
		t.Fatalf("NewMemberOfMember(view): %v", err)
	}
	return judgement
}

func validationTestMemberOfNotMember(
	t *testing.T,
	query MemberOfQuery,
	provenance DeclarationProvenance,
) MemberOfNotMember {
	t.Helper()
	judgement, err := NewMemberOfNotMember(query, validationTestMemberOfBasis(t, query, provenance))
	if err != nil {
		t.Fatalf("NewMemberOfNotMember(): %v", err)
	}
	return judgement
}

func validationTestMemberOfNotMemberWithView(
	t *testing.T,
	query MemberOfQuery,
	provenance DeclarationProvenance,
	evaluationView MemberOfEvaluationView,
) MemberOfNotMember {
	t.Helper()
	judgement, err := NewMemberOfNotMember(
		query,
		validationTestMemberOfBasisWithView(t, query, provenance, evaluationView),
	)
	if err != nil {
		t.Fatalf("NewMemberOfNotMember(view): %v", err)
	}
	return judgement
}

func validationTestMemberOfUndefined(
	t *testing.T,
	query MemberOfQuery,
	repair string,
) MemberOfUndefined {
	t.Helper()
	missing, err := MissingKindSignatureForMemberOf(query)
	if err != nil {
		t.Fatalf("MissingKindSignatureForMemberOf(): %v", err)
	}
	pointer, err := NewRepairPointer(repair)
	if err != nil {
		t.Fatalf("NewRepairPointer(): %v", err)
	}
	judgement, err := NewMemberOfUndefined(
		validationTestMemberOfRequest(t, query),
		[]MemberOfMissingBasis{missing},
		pointer,
	)
	if err != nil {
		t.Fatalf("NewMemberOfUndefined(): %v", err)
	}
	return judgement
}

func validationTestSetMemberOf(
	t *testing.T,
	snapshot *validationSnapshot,
	judgement MemberOfJudgement,
) {
	t.Helper()
	request := judgement.EvaluationRequest()
	if !MemberOfJudgementMatchesRequest(request, judgement) {
		t.Fatal("test MemberOf judgement is not self-correlated")
	}
	if snapshot.memberOfRequestJudgements == nil {
		snapshot.memberOfRequestJudgements = map[string]MemberOfJudgement{}
	}
	snapshot.memberOfRequestJudgements[request.Digest().String()] = judgement
}

func validationTestMemberOfRequest(
	t *testing.T,
	query MemberOfQuery,
) MemberOfEvaluationRequest {
	t.Helper()
	view, err := NewPersistedSnapshotView(
		query.ValueKind().TypeEnv(),
		NewGraphRevision(7),
	)
	if err != nil {
		t.Fatalf("NewPersistedSnapshotView(): %v", err)
	}
	request, err := NewMemberOfEvaluationRequest(query, view)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest(): %v", err)
	}
	return request
}

func validationTestEntityCandidate(
	t *testing.T,
	entityRaw string,
	alias EntityAlias,
	context BoundedContextRef,
) EntityCandidate {
	t.Helper()
	candidate, err := NewEntityCandidate(
		mustEntityID(t, entityRaw),
		[]EntityAlias{alias},
		[]BoundedContextRef{context},
		mustResolutionBasisRef(t, "test:alias-index"),
	)
	if err != nil {
		t.Fatalf("NewEntityCandidate(): %v", err)
	}
	return candidate
}

func assertValidationDiagnostic(
	t *testing.T,
	verdict ValidationVerdict,
	wantKind ValidationVerdictKind,
	wantCode DiagnosticCode,
) {
	t.Helper()
	if verdict.Kind() != wantKind {
		t.Fatalf("verdict kind = %s (%T); want %s", verdict.Kind(), verdict, wantKind)
	}
	var diagnostics []Diagnostic
	switch value := verdict.(type) {
	case Invalid:
		diagnostics = value.Diagnostics()
	case Underdetermined:
		diagnostics = value.Diagnostics()
	default:
		t.Fatalf("verdict %T does not expose diagnostics", verdict)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code() == wantCode {
			return
		}
	}
	t.Fatalf("diagnostics = %#v; want code %s", diagnostics, wantCode)
}

func assertValidationRepair(
	t *testing.T,
	verdict ValidationVerdict,
	wantCode DiagnosticCode,
	wantRepair string,
) {
	t.Helper()
	diagnostic := findValidationDiagnostic(t, verdict, wantCode)
	repair, present := diagnostic.Repair()
	if !present {
		t.Fatalf("diagnostic %s has no repair pointer", wantCode)
	}
	if repair.String() != wantRepair {
		t.Fatalf("diagnostic %s repair = %q; want %q", wantCode, repair.String(), wantRepair)
	}
}

func assertSingleValidationDiagnostic(
	t *testing.T,
	verdict ValidationVerdict,
	wantCode DiagnosticCode,
) {
	t.Helper()
	underdetermined, ok := verdict.(Underdetermined)
	if !ok {
		t.Fatalf("verdict %T is not Underdetermined", verdict)
	}
	diagnostics := underdetermined.Diagnostics()
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostic count = %d; want exactly one", len(diagnostics))
	}
	if diagnostics[0].Code() != wantCode {
		t.Fatalf("diagnostic code = %s; want %s", diagnostics[0].Code(), wantCode)
	}
}

func findValidationDiagnostic(
	t *testing.T,
	verdict ValidationVerdict,
	wantCode DiagnosticCode,
) Diagnostic {
	t.Helper()
	var diagnostics []Diagnostic
	switch value := verdict.(type) {
	case Invalid:
		diagnostics = value.Diagnostics()
	case Underdetermined:
		diagnostics = value.Diagnostics()
	default:
		t.Fatalf("verdict %T has no diagnostics", verdict)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code() == wantCode {
			return diagnostic
		}
	}
	t.Fatalf("diagnostics = %#v; want code %s", diagnostics, wantCode)
	return Diagnostic{}
}

func assertMissingResolutionWitness(
	t *testing.T,
	diagnostic Diagnostic,
	wantBasis []string,
) {
	t.Helper()
	if diagnostic.Posture() != DiagnosticUnderdetermined {
		t.Fatalf("posture = %q", diagnostic.Posture())
	}
	basis, ok := diagnostic.GoverningBasis().(MissingRuntimeBasis)
	if !ok || basis.MissingKind() != MissingRuntimeResolution {
		t.Fatalf("governing basis = %T/%v", diagnostic.GoverningBasis(), basis.MissingKind())
	}
	witness, ok := diagnostic.Witness().(MissingBasisWitness)
	if !ok {
		t.Fatalf("witness = %T", diagnostic.Witness())
	}
	actual := witness.Actual()
	if actual.Kind() != DiagnosticDatumSet {
		t.Fatalf("actual kind = %q", actual.Kind())
	}
	if !reflect.DeepEqual(actual.Values(), wantBasis) {
		t.Fatalf("missing basis = %v, want %v", actual.Values(), wantBasis)
	}
}

type validationSnapshot struct {
	revision                      GraphRevision
	typeEnv                       TypeEnvRef
	entityResolution              EntityResolution
	entityResolutions             map[string]EntityResolution
	referenceResolution           StrongReferenceResolution
	memberOfJudgements            map[string]MemberOfJudgement
	memberOfRequestJudgements     map[string]MemberOfJudgement
	memberOfEvaluator             func(MemberOfEvaluationRequest) MemberOfJudgement
	kindClassificationJudgements  map[string]KindClassificationJudgement
	kindClassificationEvaluator   func(KindClassificationRequest) KindClassificationJudgement
	assertionStates               map[string]AssertionState
	aliasResolution               AliasAvailability
	reconciliationBasisResolution ReconciliationBasisResolution
	disjointEntailments           map[string]DisjointEntailmentUse
}

func (snapshot validationSnapshot) GraphRevision() GraphRevision {
	return snapshot.revision
}

func (snapshot validationSnapshot) TypeEnvRef() TypeEnvRef {
	return snapshot.typeEnv
}

func (snapshot validationSnapshot) ResolveEntity(
	entity EntityID,
	context BoundedContextRef,
) EntityResolution {
	resolution, exists := snapshot.entityResolutions[entityResolutionTestKey(entity, context)]
	if exists {
		return resolution
	}
	return snapshot.entityResolution
}

func (snapshot validationSnapshot) ResolveReference(
	StrongRef,
	BoundedContextRef,
) StrongReferenceResolution {
	return snapshot.referenceResolution
}

func (snapshot validationSnapshot) EvaluateMemberOf(
	request MemberOfEvaluationRequest,
) MemberOfJudgement {
	query := request.Query()
	judgement, exists := snapshot.memberOfJudgements[query.Digest().String()]
	if exists {
		return judgement
	}
	judgement, exists = snapshot.memberOfRequestJudgements[request.Digest().String()]
	if exists {
		return judgement
	}
	if snapshot.memberOfEvaluator != nil {
		judgement = snapshot.memberOfEvaluator(request)
		if judgement != nil {
			return judgement
		}
	}
	missing, _ := MissingKindSignatureForMemberOf(query)
	repair, _ := NewRepairPointer("recover-memberof-basis")
	undefined, _ := NewMemberOfUndefined(request, []MemberOfMissingBasis{missing}, repair)
	return undefined
}

func (snapshot validationSnapshot) EvaluateKindClassification(
	request KindClassificationRequest,
) KindClassificationJudgement {
	judgement, exists := snapshot.kindClassificationJudgements[request.Digest().String()]
	if exists {
		return judgement
	}
	if snapshot.kindClassificationEvaluator != nil {
		judgement = snapshot.kindClassificationEvaluator(request)
		if judgement != nil {
			return judgement
		}
	}
	reason, _ := NewKindClassificationUnknownReason(
		KindUnknownCriterionUnavailable,
		RepairPointer{value: "provide-kind-classification-evaluator"},
	)
	unknown, _ := NewUnknownKindClassification(
		request,
		[]KindClassificationUnknownReason{reason},
	)
	return unknown
}

func (snapshot validationSnapshot) ResolveDisjointEntailment(
	request MemberOfEvaluationRequest,
	constraint ConstraintID,
	supporting MemberOfMember,
) (DisjointEntailmentUse, bool) {
	use, exists := snapshot.disjointEntailments[validationDisjointEntailmentKey(
		request,
		constraint,
		supporting,
	)]
	return use, exists
}

func (snapshot validationSnapshot) AssertionState(assertion AssertionID) AssertionState {
	state, exists := snapshot.assertionStates[assertion.String()]
	if exists {
		return state
	}
	repair, _ := NewRepairPointer("inspect-assertion-index")
	unknown, _ := NewUnknownAssertionState(assertion, repair)
	return unknown
}

func (snapshot validationSnapshot) ResolveAlias(
	EntityAlias,
	BoundedContextRef,
) AliasAvailability {
	return snapshot.aliasResolution
}

func (snapshot validationSnapshot) ResolveReconciliationBasis(
	ReconciliationBasisRef,
	BoundedContextRef,
) ReconciliationBasisResolution {
	return snapshot.reconciliationBasisResolution
}

func entityResolutionTestKey(entity EntityID, context BoundedContextRef) string {
	return exactTupleKey("test-entity-resolution", entity.String(), context.String())
}

func validationDisjointEntailmentKey(
	request MemberOfEvaluationRequest,
	constraint ConstraintID,
	supporting MemberOfMember,
) string {
	return constraint.String() + "\x00" +
		request.Digest().String() + "\x00" +
		supporting.Digest().String()
}

type corruptDisjointEntailmentUse struct {
	DisjointEntailmentUse
}

func (use corruptDisjointEntailmentUse) ConstraintDigest() SHA256Digest {
	return SHA256Digest{}
}

func (corruptDisjointEntailmentUse) disjointCounterUseVariant() {}

func (corruptDisjointEntailmentUse) disjointEntailmentUseVariant() {}
