package typedmemory

import (
	"bytes"
	"testing"
	"time"
)

var _ DefinedMemberOfJudgement = MemberOfMember{}
var _ DefinedMemberOfJudgement = MemberOfNotMember{}

func TestMemberOfBasisCanonicalizesObservableInputSet(t *testing.T) {
	fixture := newMemberOfFixture(t)
	first := memberOfTestObservableInput(t, "observable:registry/vehicle-7", 0xb1)
	second := memberOfTestObservableInput(t, "observable:registry/schema", 0xb2)

	forward := fixture.basis(t, []MemberOfObservableInput{first, second, first})
	reverse := fixture.basis(t, []MemberOfObservableInput{second, first})

	if forward.Digest() != reverse.Digest() {
		t.Fatalf(
			"observable input order changed MemberOf basis digest: %s != %s",
			forward.Digest().String(),
			reverse.Digest().String(),
		)
	}
	if !bytes.Equal(forward.CanonicalBytes(), reverse.CanonicalBytes()) {
		t.Fatal("observable input order changed MemberOf basis canonical bytes")
	}
	if len(forward.ObservableInputs()) != 2 {
		t.Fatalf("observable input count = %d; want exact duplicate deduplication", len(forward.ObservableInputs()))
	}

	conflict := memberOfTestObservableInput(t, first.Reference().String(), 0xb3)
	_, err := NewMemberOfBasis(MemberOfBasisInput{
		Query:                fixture.query,
		EvaluationView:       fixture.evaluationView,
		KindSignature:        fixture.signature,
		EntitySet:            fixture.entitySet,
		ObservableInputs:     []MemberOfObservableInput{first, conflict},
		EvaluationProvenance: fixture.evaluationProvenance,
	})
	if err == nil {
		t.Fatal("one observable reference with conflicting digests was accepted")
	}
}

func TestMemberOfObservableInputSetDigestNormalizesOrderAndDuplicates(t *testing.T) {
	first := memberOfTestObservableInput(t, "observable:a", 0xa1)
	second := memberOfTestObservableInput(t, "observable:b", 0xb2)

	ordered, err := ComputeMemberOfObservableInputSetDigest(
		[]MemberOfObservableInput{first, second},
	)
	if err != nil {
		t.Fatalf("ComputeMemberOfObservableInputSetDigest(ordered): %v", err)
	}
	reversed, err := ComputeMemberOfObservableInputSetDigest(
		[]MemberOfObservableInput{second, first},
	)
	if err != nil {
		t.Fatalf("ComputeMemberOfObservableInputSetDigest(reversed): %v", err)
	}
	deduplicated, err := ComputeMemberOfObservableInputSetDigest(
		[]MemberOfObservableInput{first, second, first},
	)
	if err != nil {
		t.Fatalf("ComputeMemberOfObservableInputSetDigest(deduplicated): %v", err)
	}
	if ordered != reversed || ordered != deduplicated {
		t.Fatal("observable-input set digest changed under ordering or identical duplication")
	}
}

func TestMemberOfObservableInputSetDigestRejectsConflictingOrEmptySets(t *testing.T) {
	first := memberOfTestObservableInput(t, "observable:conflict", 0xa1)
	conflict := memberOfTestObservableInput(t, "observable:conflict", 0xb2)
	if _, err := ComputeMemberOfObservableInputSetDigest(
		[]MemberOfObservableInput{first, conflict},
	); err == nil {
		t.Fatal("observable-input set digest accepted one reference with conflicting digests")
	}
	if _, err := ComputeMemberOfObservableInputSetDigest(nil); err == nil {
		t.Fatal("observable-input set digest accepted an empty input set")
	}
}

func TestMemberOfCanonicalGolden(t *testing.T) {
	fixture := newMemberOfFixture(t)
	basis := fixture.basis(t, []MemberOfObservableInput{
		memberOfTestObservableInput(t, "observable:registry/vehicle-7", 0xb1),
	})
	member, err := NewMemberOfMember(fixture.query, basis)
	if err != nil {
		t.Fatalf("NewMemberOfMember() error = %v", err)
	}
	actual := []string{
		fixture.entitySet.Ref().Digest().String(),
		fixture.signature.Ref().Digest().String(),
		fixture.query.Digest().String(),
		basis.Digest().String(),
		member.Digest().String(),
	}
	want := []string{
		"sha256:f8a4ee1ce254ac7b52c80d701e519fee78b3a9ac3e35aea13fef3d3002a5adb2",
		"sha256:233efd73b818e4d2a2e8eac9eb1cc29455e61c5c7459c4922ddad8c924122e6a",
		"sha256:c9be4d91f0aeecf213b27686d3fee7d7c2ce1302a564499989f86a9e0804a385",
		"sha256:a6ba0fd826839f7cda7f69e9db72baba437829de2cec6aac7141f1642cbdc141",
		"sha256:3d67497d244365a322417c8f0a42995481ff2548cb9c043a0829cc71d7dc0862",
	}
	for index := range want {
		if actual[index] != want[index] {
			t.Fatalf("canonical golden = %#v", actual)
		}
	}
}

func TestMemberOfJudgementKeepsMemberNotMemberAndUndefinedDistinct(t *testing.T) {
	fixture := newMemberOfFixture(t)
	basis := fixture.basis(t, []MemberOfObservableInput{
		memberOfTestObservableInput(t, "observable:registry/vehicle-7", 0xb1),
	})

	member, err := NewMemberOfMember(fixture.query, basis)
	if err != nil {
		t.Fatalf("NewMemberOfMember() error = %v", err)
	}
	replayedMember, err := NewMemberOfMember(fixture.query, basis)
	if err != nil {
		t.Fatalf("NewMemberOfMember(replay) error = %v", err)
	}
	if member.Digest() != replayedMember.Digest() ||
		!bytes.Equal(member.CanonicalBytes(), replayedMember.CanonicalBytes()) {
		t.Fatal("same fixed MemberOf inputs did not reproduce one judgement")
	}
	notMember, err := NewMemberOfNotMember(fixture.query, basis)
	if err != nil {
		t.Fatalf("NewMemberOfNotMember() error = %v", err)
	}
	missing, err := MissingObservableInputForMemberOf(
		memberOfTestObservableInputRef(t, "observable:registry/vehicle-7"),
	)
	if err != nil {
		t.Fatalf("MissingObservableInputForMemberOf() error = %v", err)
	}
	undefined, err := NewMemberOfUndefined(
		fixture.request(t),
		[]MemberOfMissingBasis{missing, missing},
		memberOfTestRepair(t, "repair:materialize-registry-record"),
	)
	if err != nil {
		t.Fatalf("NewMemberOfUndefined() error = %v", err)
	}

	judgements := []MemberOfJudgement{member, notMember, undefined}
	wantKinds := []MemberOfJudgementKind{
		MemberJudgement,
		NotMemberJudgement,
		UndefinedMemberJudgement,
	}
	seenDigests := map[SHA256Digest]struct{}{}
	for index, judgement := range judgements {
		if judgement.Kind() != wantKinds[index] {
			t.Fatalf("judgement %d kind = %s; want %s", index, judgement.Kind().String(), wantKinds[index].String())
		}
		if !MemberOfJudgementMatchesRequest(fixture.request(t), judgement) {
			t.Fatalf("judgement %d did not match its exact evaluation request", index)
		}
		seenDigests[judgement.Digest()] = struct{}{}
	}
	if len(seenDigests) != 3 {
		t.Fatal("Member, NotMember, and Undefined collapsed to one canonical judgement")
	}
	if len(undefined.MissingBasis()) != 1 {
		t.Fatalf("undefined missing-basis count = %d; want exact duplicate deduplication", len(undefined.MissingBasis()))
	}
	missingSignature, err := MissingKindSignatureForMemberOf(fixture.query)
	if err != nil {
		t.Fatalf("MissingKindSignatureForMemberOf() error = %v", err)
	}
	forwardUndefined, err := NewMemberOfUndefined(
		fixture.request(t),
		[]MemberOfMissingBasis{missing, missingSignature},
		memberOfTestRepair(t, "repair:restore-membership-basis"),
	)
	if err != nil {
		t.Fatalf("NewMemberOfUndefined(forward) error = %v", err)
	}
	reverseUndefined, err := NewMemberOfUndefined(
		fixture.request(t),
		[]MemberOfMissingBasis{missingSignature, missing},
		memberOfTestRepair(t, "repair:restore-membership-basis"),
	)
	if err != nil {
		t.Fatalf("NewMemberOfUndefined(reverse) error = %v", err)
	}
	if forwardUndefined.Digest() != reverseUndefined.Digest() {
		t.Fatal("missing-basis order changed Undefined MemberOf identity")
	}

	_, err = NewMemberOfUndefined(
		fixture.request(t),
		nil,
		memberOfTestRepair(t, "repair:missing-basis"),
	)
	if err == nil {
		t.Fatal("Undefined MemberOf without a missing basis was accepted")
	}
}

func TestMemberOfUndefinedDistinguishesNoApplicableSourceExactly(t *testing.T) {
	fixture := newMemberOfFixture(t)
	noApplicableBasis, err := NoApplicableObservableSourceForMemberOf(fixture.query)
	if err != nil {
		t.Fatalf("NoApplicableObservableSourceForMemberOf() error = %v", err)
	}
	unusableBasis, err := MissingUniqueTrustedObservableSourceForMemberOf(fixture.query)
	if err != nil {
		t.Fatalf("MissingUniqueTrustedObservableSourceForMemberOf() error = %v", err)
	}
	missingSignature, err := MissingKindSignatureForMemberOf(fixture.query)
	if err != nil {
		t.Fatalf("MissingKindSignatureForMemberOf() error = %v", err)
	}
	noApplicable, err := NewMemberOfUndefined(
		fixture.request(t),
		[]MemberOfMissingBasis{noApplicableBasis},
		memberOfTestRepair(t, "repair:no-applicable-source"),
	)
	if err != nil {
		t.Fatalf("NewMemberOfUndefined(no applicable source) error = %v", err)
	}
	unusable, err := NewMemberOfUndefined(
		fixture.request(t),
		[]MemberOfMissingBasis{unusableBasis},
		memberOfTestRepair(t, "repair:unique-trusted-source"),
	)
	if err != nil {
		t.Fatalf("NewMemberOfUndefined(unusable source) error = %v", err)
	}
	missingPrerequisite, err := NewMemberOfUndefined(
		fixture.request(t),
		[]MemberOfMissingBasis{missingSignature},
		memberOfTestRepair(t, "repair:kind-signature"),
	)
	if err != nil {
		t.Fatalf("NewMemberOfUndefined(missing prerequisite) error = %v", err)
	}
	mixed, err := NewMemberOfUndefined(
		fixture.request(t),
		[]MemberOfMissingBasis{noApplicableBasis, missingSignature},
		memberOfTestRepair(t, "repair:mixed-source-and-signature"),
	)
	if err != nil {
		t.Fatalf("NewMemberOfUndefined(mixed basis) error = %v", err)
	}
	otherQuery := fixture.queryForEntity(t, "entity:vehicle-8")
	wrongQueryBasis, err := NoApplicableObservableSourceForMemberOf(otherQuery)
	if err != nil {
		t.Fatalf("NoApplicableObservableSourceForMemberOf(other query) error = %v", err)
	}
	wrongQuery, err := NewMemberOfUndefined(
		fixture.request(t),
		[]MemberOfMissingBasis{wrongQueryBasis},
		memberOfTestRepair(t, "repair:wrong-query-source-posture"),
	)
	if err != nil {
		t.Fatalf("NewMemberOfUndefined(wrong query basis) error = %v", err)
	}

	if !noApplicable.IsNoApplicableObservableSource() {
		t.Fatal("exact no-applicable-source Undefined lost its typed posture")
	}
	if unusable.IsNoApplicableObservableSource() {
		t.Fatal("unusable source material became no-applicable-source")
	}
	if missingPrerequisite.IsNoApplicableObservableSource() {
		t.Fatal("missing C.3.2 prerequisite became no-applicable-source")
	}
	if mixed.IsNoApplicableObservableSource() {
		t.Fatal("mixed missing basis became exact no-applicable-source")
	}
	if wrongQuery.IsNoApplicableObservableSource() {
		t.Fatal("no-applicable-source basis for another query became exact")
	}
	if noApplicable.Digest() == unusable.Digest() {
		t.Fatal("no-applicable and unusable source postures share one identity")
	}
}

func TestMemberOfBasisRejectsKindContextAndQueryMismatch(t *testing.T) {
	fixture := newMemberOfFixture(t)
	observable := memberOfTestObservableInput(t, "observable:registry/vehicle-7", 0xb1)

	otherKind := typeEnvTestValueKindRef(
		t,
		fixture.typeEnv.ref,
		fixture.typeEnv.claimGraphKind.ID(),
	)
	otherSignature := typeEnvTestKindSignatureDefinition(
		t,
		otherKind,
		SignatureF4,
		nil,
		"test:member-of/claim-graph/v1",
		fixture.entitySet.Ref(),
		fixture.typeEnv.provenance,
	)
	_, err := NewMemberOfBasis(MemberOfBasisInput{
		Query:                fixture.query,
		EvaluationView:       fixture.evaluationView,
		KindSignature:        otherSignature,
		EntitySet:            fixture.entitySet,
		ObservableInputs:     []MemberOfObservableInput{observable},
		EvaluationProvenance: fixture.evaluationProvenance,
	})
	if err == nil {
		t.Fatal("KindSignature for another ValueKind was accepted")
	}

	otherEntitySet := typeEnvTestEntitySetDefinition(
		t,
		fixture.typeEnv.ref,
		fixture.typeEnv.secondaryContext.Ref(),
		"test:entity-set/secondary/v1",
		fixture.typeEnv.provenance,
	)
	_, err = NewMemberOfBasis(MemberOfBasisInput{
		Query:                fixture.query,
		EvaluationView:       fixture.evaluationView,
		KindSignature:        fixture.signature,
		EntitySet:            otherEntitySet,
		ObservableInputs:     []MemberOfObservableInput{observable},
		EvaluationProvenance: fixture.evaluationProvenance,
	})
	if err == nil {
		t.Fatal("EntitySet for another bounded context was accepted")
	}

	basis := fixture.basis(t, []MemberOfObservableInput{observable})
	otherQuery := fixture.queryForEntity(t, "entity:vehicle-8")
	otherBasis := fixture.basisForQuery(t, otherQuery, []MemberOfObservableInput{observable})
	otherJudgement, err := NewMemberOfMember(otherQuery, otherBasis)
	if err != nil {
		t.Fatalf("NewMemberOfMember(other query) error = %v", err)
	}
	mismatches := memberOfJudgementBaseQueryMismatches(fixture.query, otherJudgement)
	if len(mismatches) != 1 || mismatches[0].Kind() != MemberOfJudgementQueryMismatch {
		t.Fatalf("query mismatches = %#v; want one judgement_query_mismatch", mismatches)
	}
	if len(basis.Mismatches(fixture.query)) != 0 {
		t.Fatal("a correctly correlated basis reported a mismatch")
	}
}

func TestMemberOfQueryAndBasisDigestsCommitToSliceAndEvaluationBasis(t *testing.T) {
	fixture := newMemberOfFixture(t)
	input := memberOfTestObservableInput(t, "observable:registry/vehicle-7", 0xb1)
	base := fixture.basis(t, []MemberOfObservableInput{input})

	laterSlice := memberOfTestContextSlice(
		t,
		fixture.typeEnv.primaryContext.Ref(),
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
	if laterQuery.Digest() == fixture.query.Digest() {
		t.Fatal("changing Gamma_time did not change MemberOf query digest")
	}

	changedInput := memberOfTestObservableInput(t, input.Reference().String(), 0xb4)
	changed := fixture.basis(t, []MemberOfObservableInput{changedInput})
	if changed.Digest() == base.Digest() {
		t.Fatal("changing observable bytes did not change MemberOf basis digest")
	}

	_, err = NewMemberOfEvaluationProvenance(MemberOfEvaluationProvenanceInput{
		Reference:         memberOfTestProvenanceRef(t, "prov:member-of/evaluation/latest"),
		EvaluatorArtifact: memberOfTestCarrierRef(t, "binary:member-of-evaluator"),
		EvaluatorEdition:  memberOfTestCarrierEdition(t, "latest"),
		EvaluatorDigest:   memberOfTestDigest(t, 0xb5),
	})
	if err == nil {
		t.Fatal("implicit latest evaluator edition was accepted")
	}

	_, err = NewMemberOfQuery(
		fixture.query.EntityID(),
		fixture.query.ValueKind(),
		ContextSlice{},
	)
	if err == nil {
		t.Fatal("MemberOf query without a complete ContextSlice was accepted")
	}
}

type memberOfFixture struct {
	typeEnv              typeEnvFixture
	query                MemberOfQuery
	entitySet            EntitySetDefinition
	signature            KindSignatureDefinition
	evaluationProvenance MemberOfEvaluationProvenance
	evaluationView       PersistedSnapshotView
}

func newMemberOfFixture(t *testing.T) memberOfFixture {
	t.Helper()
	typeEnv := newTypeEnvFixture(t)
	contextSlice := memberOfTestContextSlice(
		t,
		typeEnv.primaryContext.Ref(),
		time.Date(2026, time.July, 16, 9, 0, 0, 0, time.UTC),
	)
	query, err := NewMemberOfQuery(
		memberOfTestEntityID(t, "entity:vehicle-7"),
		typeEnv.entityValueKind,
		contextSlice,
	)
	if err != nil {
		t.Fatalf("NewMemberOfQuery() error = %v", err)
	}
	entitySet := typeEnvTestEntitySetDefinition(
		t,
		typeEnv.ref,
		typeEnv.primaryContext.Ref(),
		"test:entity-set/primary/v1",
		typeEnv.provenance,
	)
	signature := typeEnvTestKindSignatureDefinition(
		t,
		typeEnv.entityValueKind,
		SignatureF4,
		[]KindAssumptionPin{
			typeEnvTestKindAssumption(t, "standard:registry", "v2.3", 0xa1),
		},
		"test:member-of/entity/v1",
		entitySet.Ref(),
		typeEnv.provenance,
	)
	provenance, err := NewMemberOfEvaluationProvenance(MemberOfEvaluationProvenanceInput{
		Reference:         memberOfTestProvenanceRef(t, "prov:member-of/evaluation/v1"),
		EvaluatorArtifact: memberOfTestCarrierRef(t, "binary:member-of-evaluator"),
		EvaluatorEdition:  memberOfTestCarrierEdition(t, "build-20260716.1"),
		EvaluatorDigest:   memberOfTestDigest(t, 0xb0),
	})
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationProvenance() error = %v", err)
	}
	evaluationView, err := NewPersistedSnapshotView(typeEnv.ref, NewGraphRevision(42))
	if err != nil {
		t.Fatalf("NewPersistedSnapshotView() error = %v", err)
	}
	return memberOfFixture{
		typeEnv:              typeEnv,
		query:                query,
		entitySet:            entitySet,
		signature:            signature,
		evaluationProvenance: provenance,
		evaluationView:       evaluationView,
	}
}

func (fixture memberOfFixture) queryForEntity(
	t *testing.T,
	entity string,
) MemberOfQuery {
	t.Helper()
	query, err := NewMemberOfQuery(
		memberOfTestEntityID(t, entity),
		fixture.query.ValueKind(),
		fixture.query.ContextSlice(),
	)
	if err != nil {
		t.Fatalf("NewMemberOfQuery(%q) error = %v", entity, err)
	}
	return query
}

func (fixture memberOfFixture) request(t *testing.T) MemberOfEvaluationRequest {
	t.Helper()
	return fixture.requestForQuery(t, fixture.query)
}

func (fixture memberOfFixture) requestForQuery(
	t *testing.T,
	query MemberOfQuery,
) MemberOfEvaluationRequest {
	t.Helper()
	request, err := NewMemberOfEvaluationRequest(query, fixture.evaluationView)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest() error = %v", err)
	}
	return request
}

func (fixture memberOfFixture) basis(
	t *testing.T,
	inputs []MemberOfObservableInput,
) MemberOfBasis {
	t.Helper()
	return fixture.basisForQuery(t, fixture.query, inputs)
}

func (fixture memberOfFixture) basisForQuery(
	t *testing.T,
	query MemberOfQuery,
	inputs []MemberOfObservableInput,
) MemberOfBasis {
	t.Helper()
	basis, err := NewMemberOfBasis(MemberOfBasisInput{
		Query:                query,
		EvaluationView:       fixture.evaluationView,
		KindSignature:        fixture.signature,
		EntitySet:            fixture.entitySet,
		ObservableInputs:     inputs,
		EvaluationProvenance: fixture.evaluationProvenance,
	})
	if err != nil {
		t.Fatalf("NewMemberOfBasis() error = %v", err)
	}
	return basis
}

func memberOfTestContextSlice(
	t *testing.T,
	context BoundedContextRef,
	at time.Time,
) ContextSlice {
	t.Helper()
	gamma, err := NewGammaPoint(at)
	if err != nil {
		t.Fatalf("NewGammaPoint() error = %v", err)
	}
	contextSlice, err := NewContextSlice(ContextSliceInput{
		Context:   context,
		GammaTime: gamma,
	})
	if err != nil {
		t.Fatalf("NewContextSlice() error = %v", err)
	}
	return contextSlice
}

func memberOfTestEntityID(t *testing.T, raw string) EntityID {
	t.Helper()
	value, err := NewEntityID(raw)
	if err != nil {
		t.Fatalf("NewEntityID() error = %v", err)
	}
	return value
}

func memberOfTestObservableInputRef(t *testing.T, raw string) ObservableInputRef {
	t.Helper()
	value, err := NewObservableInputRef(raw)
	if err != nil {
		t.Fatalf("NewObservableInputRef() error = %v", err)
	}
	return value
}

func memberOfTestObservableInput(
	t *testing.T,
	raw string,
	fill byte,
) MemberOfObservableInput {
	t.Helper()
	value, err := NewMemberOfObservableInput(
		memberOfTestObservableInputRef(t, raw),
		memberOfTestDigest(t, fill),
	)
	if err != nil {
		t.Fatalf("NewMemberOfObservableInput() error = %v", err)
	}
	return value
}

func memberOfTestDigest(t *testing.T, fill byte) SHA256Digest {
	t.Helper()
	return typeEnvTestDigest(t, fill)
}

func memberOfTestProvenanceRef(t *testing.T, raw string) ProvenanceRef {
	t.Helper()
	return typeEnvTestProvenanceRef(t, raw)
}

func memberOfTestCarrierRef(t *testing.T, raw string) CarrierRef {
	t.Helper()
	return typeEnvTestCarrierRef(t, raw)
}

func memberOfTestCarrierEdition(t *testing.T, raw string) CarrierEdition {
	t.Helper()
	return typeEnvTestCarrierEdition(t, raw)
}

func memberOfTestRepair(t *testing.T, raw string) RepairPointer {
	t.Helper()
	value, err := NewRepairPointer(raw)
	if err != nil {
		t.Fatalf("NewRepairPointer() error = %v", err)
	}
	return value
}
