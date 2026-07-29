package typedmemory

import (
	"bytes"
	"testing"
)

func TestProspectiveMemberOfViewBindsExactPriorDeclarationAndPrefix(t *testing.T) {
	fixture := newLocalReferenceAdmissionFixture(t, "local:prospective-view", true)
	declaration, localReference, prefix := prospectiveViewTestBasis(t, fixture)

	view, err := NewProspectiveBatchView(ProspectiveBatchViewInput{
		TypeEnv:                  fixture.environment.Ref(),
		PreStateGraphRevision:    fixture.snapshot.GraphRevision(),
		EvaluationChangeOrdinal:  1,
		DeclarationChangeOrdinal: 0,
		Declaration:              declaration,
		LocalReference:           localReference,
		PersistedReference:       fixture.stableReference,
		OrderedCandidatePrefix:   prefix,
	})
	if err != nil {
		t.Fatalf("NewProspectiveBatchView(): %v", err)
	}
	if view.Kind() != ProspectiveBatchEvaluationView {
		t.Fatalf("view kind = %s; want prospective_batch", view.Kind().String())
	}
	if view.DeclarationChangeOrdinal() != 0 || view.EvaluationChangeOrdinal() != 1 {
		t.Fatal("prospective view lost exact declaration/evaluation ordinals")
	}
	if !bytes.Equal(view.DeclarationCanonicalBytes(), mustCanonicalMemoryChange(t, declaration)) {
		t.Fatal("prospective view changed the exact DeclareEntity candidate bytes")
	}
	if view.OrderedCandidatePrefix().Digest() != prefix.Digest() {
		t.Fatal("prospective view changed the ordered candidate prefix")
	}
	query := prospectiveViewTestQuery(t, fixture)
	request, err := NewMemberOfEvaluationRequest(query, view)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest(): %v", err)
	}
	if request.View().Digest() != view.Digest() {
		t.Fatal("MemberOf request did not retain the exact prospective view")
	}
}

func TestProspectiveMemberOfViewRejectsDeclarationAtOrAfterUse(t *testing.T) {
	fixture := newLocalReferenceAdmissionFixture(t, "local:late-declaration", true)
	declaration, localReference, prefix := prospectiveViewTestBasis(t, fixture)

	_, err := NewProspectiveBatchView(ProspectiveBatchViewInput{
		TypeEnv:                  fixture.environment.Ref(),
		PreStateGraphRevision:    fixture.snapshot.GraphRevision(),
		EvaluationChangeOrdinal:  0,
		DeclarationChangeOrdinal: 0,
		Declaration:              declaration,
		LocalReference:           localReference,
		PersistedReference:       fixture.stableReference,
		OrderedCandidatePrefix:   prefix,
	})
	if err == nil {
		t.Fatal("prospective view accepted a declaration that did not precede evaluation")
	}
}

func TestProspectiveMemberOfViewRejectsCrossTypeEnvReferenceLowering(t *testing.T) {
	fixture := newLocalReferenceAdmissionFixture(t, "local:cross-typeenv", true)
	declaration, _, prefix := prospectiveViewTestBasis(t, fixture)
	otherTypeEnv := typeEnvTestTypeEnvRef(t, 0xee)
	if otherTypeEnv == fixture.environment.Ref() {
		t.Fatal("cross-TypeEnv fixture unexpectedly reused the view TypeEnv")
	}
	otherRefKind := typeEnvTestRefKindRef(
		t,
		otherTypeEnv,
		fixture.stableReference.RefKind().ID(),
	)
	localReference, err := NewLocalRef(otherRefKind, declaration.LocalRef())
	if err != nil {
		t.Fatalf("NewLocalRef(): %v", err)
	}
	persistedReference, err := NewPersistedRef(
		otherRefKind,
		fixture.stableReference.ReferenceID(),
	)
	if err != nil {
		t.Fatalf("NewPersistedRef(): %v", err)
	}
	if localReference.RefKind() != persistedReference.RefKind() {
		t.Fatal("cross-TypeEnv fixture must keep both lowering endpoints on one RefKind")
	}

	_, err = NewProspectiveBatchView(ProspectiveBatchViewInput{
		TypeEnv:                  fixture.environment.Ref(),
		PreStateGraphRevision:    fixture.snapshot.GraphRevision(),
		EvaluationChangeOrdinal:  1,
		DeclarationChangeOrdinal: 0,
		Declaration:              declaration,
		LocalReference:           localReference,
		PersistedReference:       persistedReference,
		OrderedCandidatePrefix:   prefix,
	})
	if err == nil {
		t.Fatal("prospective view accepted a lowering whose RefKind belongs to another TypeEnv")
	}
}

func TestProspectiveMemberOfViewCommitsPrefixWithoutEmbeddingItsPayload(t *testing.T) {
	fixture := newLocalReferenceAdmissionFixture(t, "local:prefix-target", true)
	declaration, localReference, _ := prospectiveViewTestBasis(t, fixture)
	firstExtra := prospectiveViewTestDeclaration(
		t,
		"entity:prefix-extra-a",
		"local:prefix-extra-a",
		"Prefix extra A",
		"memory:prefix-extra-a",
		declaration.Context(),
	)
	secondExtra := prospectiveViewTestDeclaration(
		t,
		"entity:prefix-extra-b",
		"local:prefix-extra-b",
		"A much longer prefix payload that must not be copied into the view",
		"memory:prefix-extra-b",
		declaration.Context(),
	)
	firstPrefix := prospectiveViewTestPrefix(t, []MemoryChange{declaration, firstExtra})
	secondPrefix := prospectiveViewTestPrefix(t, []MemoryChange{declaration, secondExtra})
	firstView := prospectiveViewTestView(
		t,
		fixture,
		declaration,
		localReference,
		firstPrefix,
		2,
		0,
	)
	secondView := prospectiveViewTestView(
		t,
		fixture,
		declaration,
		localReference,
		secondPrefix,
		2,
		0,
	)

	if firstPrefix.Digest() == secondPrefix.Digest() {
		t.Fatal("different ordered candidate prefixes collapsed to one digest")
	}
	if firstView.Digest() == secondView.Digest() {
		t.Fatal("prospective view did not commit the exact prefix digest")
	}
	query := prospectiveViewTestQuery(t, fixture)
	firstRequest, err := NewMemberOfEvaluationRequest(query, firstView)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest(first): %v", err)
	}
	secondRequest, err := NewMemberOfEvaluationRequest(query, secondView)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest(second): %v", err)
	}
	if firstRequest.Digest() == secondRequest.Digest() {
		t.Fatal("different prospective prefixes collapsed to one extended MemberOf slice")
	}
	if len(firstView.CanonicalBytes()) != len(secondView.CanonicalBytes()) {
		t.Fatal("prospective view embedded variable-size prefix payload instead of its exact digest")
	}
	if bytes.Contains(secondView.CanonicalBytes(), []byte(secondExtra.Label().String())) {
		t.Fatal("prospective view copied an unrelated prior declaration payload")
	}
	if len(secondView.OrderedCandidatePrefix().Changes()) != 2 {
		t.Fatal("evaluator-visible view lost exact ordered prefix content")
	}
}

func TestProspectiveMemberOfBasisRequiresExplicitCandidateVisibilityPolicy(t *testing.T) {
	fixture := newMemberOfFixture(t)
	view := prospectiveMemberOfFixtureView(t, fixture)
	input := MemberOfBasisInput{
		Query:                fixture.query,
		EvaluationView:       view,
		KindSignature:        fixture.signature,
		EntitySet:            fixture.entitySet,
		ObservableInputs:     []MemberOfObservableInput{memberOfTestObservableInput(t, "observable:prospective/policy", 0xd1)},
		EvaluationProvenance: fixture.evaluationProvenance,
	}
	if _, err := NewMemberOfBasis(input); err == nil {
		t.Fatal("prospective MemberOf basis accepted persisted-only EntitySet visibility")
	}

	missing, err := MissingCandidateVisibilityForMemberOf(fixture.entitySet.Ref())
	if err != nil {
		t.Fatalf("MissingCandidateVisibilityForMemberOf(): %v", err)
	}
	repair, err := NewRepairPointer("declare-prior-batch-candidate-visibility")
	if err != nil {
		t.Fatalf("NewRepairPointer(): %v", err)
	}
	request, err := NewMemberOfEvaluationRequest(fixture.query, view)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest(): %v", err)
	}
	undefined, err := NewMemberOfUndefined(
		request,
		[]MemberOfMissingBasis{missing},
		repair,
	)
	if err != nil {
		t.Fatalf("NewMemberOfUndefined(): %v", err)
	}
	if undefined.MissingBasis()[0].Kind() != MissingMemberOfCandidateVisibility {
		t.Fatal("undefined MemberOf lost the exact candidate-visibility missing basis")
	}
	if !MemberOfJudgementMatchesRequest(request, undefined) {
		t.Fatal("Undefined lost the exact prospective MemberOf evaluation request")
	}
	if MemberOfJudgementMatchesRequest(fixture.request(t), undefined) {
		t.Fatal("prospective Undefined satisfied a persisted-snapshot request")
	}
}

func TestProspectiveMemberOfBasisAcceptsExplicitPriorDeclarationPolicy(t *testing.T) {
	fixture := newMemberOfFixture(t)
	view := prospectiveMemberOfFixtureView(t, fixture)
	policy, err := NewPriorBatchDeclarationsVisible(
		typeEnvTestRuleRef(t, "test:entity-set/prior-batch-candidates/v1"),
	)
	if err != nil {
		t.Fatalf("NewPriorBatchDeclarationsVisible(): %v", err)
	}
	entitySet := typeEnvTestEntitySetDefinitionWithPolicy(
		t,
		fixture.typeEnv.ref,
		fixture.query.ContextSlice().Context(),
		"test:entity-set/prospective/v1",
		policy,
		fixture.typeEnv.provenance,
	)
	signature := typeEnvTestKindSignatureDefinition(
		t,
		fixture.query.ValueKind(),
		SignatureF4,
		nil,
		"test:member-of/prospective/v1",
		entitySet.Ref(),
		fixture.typeEnv.provenance,
	)
	basis, err := NewMemberOfBasis(MemberOfBasisInput{
		Query:                fixture.query,
		EvaluationView:       view,
		KindSignature:        signature,
		EntitySet:            entitySet,
		ObservableInputs:     []MemberOfObservableInput{memberOfTestObservableInput(t, "observable:prospective/visible", 0xd2)},
		EvaluationProvenance: fixture.evaluationProvenance,
	})
	if err != nil {
		t.Fatalf("NewMemberOfBasis(): %v", err)
	}
	member, err := NewMemberOfMember(fixture.query, basis)
	if err != nil {
		t.Fatalf("NewMemberOfMember(): %v", err)
	}
	request, err := NewMemberOfEvaluationRequest(fixture.query, view)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest(): %v", err)
	}
	if !MemberOfJudgementMatchesRequest(request, member) {
		t.Fatal("defined prospective MemberOf judgement lost its exact request view")
	}
}

func prospectiveViewTestBasis(
	t *testing.T,
	fixture localReferenceAdmissionFixture,
) (DeclareEntity, LocalRef, OrderedCandidatePrefix) {
	t.Helper()
	changes := fixture.changeSet.Changes()
	declaration, ok := changes[0].(DeclareEntity)
	if !ok {
		t.Fatalf("change[0] = %T; want DeclareEntity", changes[0])
	}
	localReference, err := NewLocalRef(
		fixture.stableReference.RefKind(),
		declaration.LocalRef(),
	)
	if err != nil {
		t.Fatalf("NewLocalRef(): %v", err)
	}
	prefix, err := ComputeOrderedCandidatePrefix(fixture.changeSet, 1)
	if err != nil {
		t.Fatalf("ComputeOrderedCandidatePrefix(): %v", err)
	}
	return declaration, localReference, prefix
}

func prospectiveViewTestQuery(
	t *testing.T,
	fixture localReferenceAdmissionFixture,
) MemberOfQuery {
	t.Helper()
	changes := fixture.changeSet.Changes()
	instantiation, ok := changes[1].(InstantiateRelation)
	if !ok {
		t.Fatalf("change[1] = %T; want InstantiateRelation", changes[1])
	}
	signature, ok := fixture.environment.RelationSignature(instantiation.Relation().Signature())
	if !ok {
		t.Fatal("fixture relation signature is missing")
	}
	var target ReferenceSlotTarget
	found := false
	for _, slot := range signature.Slots() {
		candidate, ok := slot.Target().(ReferenceSlotTarget)
		if !ok {
			continue
		}
		target = candidate
		found = true
		break
	}
	if !found {
		t.Fatal("fixture relation has no reference target")
	}
	return validationTestMemberOfQuery(
		t,
		fixture.entity,
		target.ValueKind(),
		instantiation.Relation().Slice(),
	)
}

func prospectiveViewTestPrefix(
	t *testing.T,
	changes []MemoryChange,
) OrderedCandidatePrefix {
	t.Helper()
	changeSet, err := NewMemoryChangeSet(changes)
	if err != nil {
		t.Fatalf("NewMemoryChangeSet(): %v", err)
	}
	prefix, err := ComputeOrderedCandidatePrefix(changeSet, uint64(len(changes)))
	if err != nil {
		t.Fatalf("ComputeOrderedCandidatePrefix(): %v", err)
	}
	return prefix
}

func prospectiveViewTestView(
	t *testing.T,
	fixture localReferenceAdmissionFixture,
	declaration DeclareEntity,
	localReference LocalRef,
	prefix OrderedCandidatePrefix,
	evaluationOrdinal uint64,
	declarationOrdinal uint64,
) ProspectiveBatchView {
	t.Helper()
	view, err := NewProspectiveBatchView(ProspectiveBatchViewInput{
		TypeEnv:                  fixture.environment.Ref(),
		PreStateGraphRevision:    fixture.snapshot.GraphRevision(),
		EvaluationChangeOrdinal:  evaluationOrdinal,
		DeclarationChangeOrdinal: declarationOrdinal,
		Declaration:              declaration,
		LocalReference:           localReference,
		PersistedReference:       fixture.stableReference,
		OrderedCandidatePrefix:   prefix,
	})
	if err != nil {
		t.Fatalf("NewProspectiveBatchView(): %v", err)
	}
	return view
}

func prospectiveViewTestDeclaration(
	t *testing.T,
	entityRaw string,
	localRaw string,
	labelRaw string,
	provenanceRaw string,
	context BoundedContextRef,
) DeclareEntity {
	t.Helper()
	local, err := NewBatchLocalRef(localRaw)
	if err != nil {
		t.Fatalf("NewBatchLocalRef(): %v", err)
	}
	label, err := NewEntityLabel(labelRaw)
	if err != nil {
		t.Fatalf("NewEntityLabel(): %v", err)
	}
	declaration, err := NewDeclareEntity(
		mustEntityID(t, entityRaw),
		local,
		context,
		label,
		typeEnvTestProvenanceRef(t, provenanceRaw),
	)
	if err != nil {
		t.Fatalf("NewDeclareEntity(): %v", err)
	}
	return declaration
}

func prospectiveMemberOfFixtureView(
	t *testing.T,
	fixture memberOfFixture,
) ProspectiveBatchView {
	t.Helper()
	local, err := NewBatchLocalRef("local:prospective-memberof-fixture")
	if err != nil {
		t.Fatalf("NewBatchLocalRef(): %v", err)
	}
	label, err := NewEntityLabel("Prospective MemberOf fixture")
	if err != nil {
		t.Fatalf("NewEntityLabel(): %v", err)
	}
	declaration, err := NewDeclareEntity(
		fixture.query.EntityID(),
		local,
		fixture.query.ContextSlice().Context(),
		label,
		typeEnvTestProvenanceRef(t, "memory:prospective-memberof-fixture"),
	)
	if err != nil {
		t.Fatalf("NewDeclareEntity(): %v", err)
	}
	changeSet, err := NewMemoryChangeSet([]MemoryChange{declaration})
	if err != nil {
		t.Fatalf("NewMemoryChangeSet(): %v", err)
	}
	prefix, err := ComputeOrderedCandidatePrefix(changeSet, 1)
	if err != nil {
		t.Fatalf("ComputeOrderedCandidatePrefix(): %v", err)
	}
	localReference, err := NewLocalRef(fixture.typeEnv.entityRefKind, local)
	if err != nil {
		t.Fatalf("NewLocalRef(): %v", err)
	}
	referenceID, err := NewReferenceID(fixture.query.EntityID().String())
	if err != nil {
		t.Fatalf("NewReferenceID(): %v", err)
	}
	persistedReference, err := NewPersistedRef(fixture.typeEnv.entityRefKind, referenceID)
	if err != nil {
		t.Fatalf("NewPersistedRef(): %v", err)
	}
	view, err := NewProspectiveBatchView(ProspectiveBatchViewInput{
		TypeEnv:                  fixture.typeEnv.ref,
		PreStateGraphRevision:    NewGraphRevision(42),
		EvaluationChangeOrdinal:  1,
		DeclarationChangeOrdinal: 0,
		Declaration:              declaration,
		LocalReference:           localReference,
		PersistedReference:       persistedReference,
		OrderedCandidatePrefix:   prefix,
	})
	if err != nil {
		t.Fatalf("NewProspectiveBatchView(): %v", err)
	}
	return view
}
