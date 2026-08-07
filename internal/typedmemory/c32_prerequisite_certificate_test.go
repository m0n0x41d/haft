package typedmemory

import (
	"bytes"
	"testing"
)

func TestC32CertificateEnforcesVisibilityShape(t *testing.T) {
	fixture := newMemberOfFixture(t)
	request, err := NewMemberOfEvaluationRequest(
		fixture.query,
		fixture.evaluationView,
	)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest(): %v", err)
	}
	input := c32CertificateTestInput(t, fixture, request)
	prospective, err := NewC32ProspectiveVisibilityCoordinate(
		C32ProspectiveVisibilityCoordinateInput{
			RequestDigest: memberOfTestDigest(t, 0xc1),
			ResultDigest:  memberOfTestDigest(t, 0xc2),
			BasisDigest:   memberOfTestDigest(t, 0xc3),
			Rule:          typeEnvTestRuleRef(t, "test:c32/candidate/v1"),
			Mechanism:     c32TestMechanism(t, 0xc4),
		},
	)
	if err != nil {
		t.Fatalf("NewC32ProspectiveVisibilityCoordinate(): %v", err)
	}
	input.CandidateVisibility = prospective
	if _, err := NewC32PrerequisiteCertificate(input); err == nil {
		t.Fatal("persisted evaluation accepted prospective candidate coordinates")
	}

	prospectiveView := prospectiveMemberOfFixtureView(t, fixture)
	prospectiveRequest, err := NewMemberOfEvaluationRequest(
		fixture.query,
		prospectiveView,
	)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest(prospective): %v", err)
	}
	input = c32CertificateTestInput(t, fixture, prospectiveRequest)
	input.CandidateVisibility = NewC32PersistedVisibilityCoordinate()
	if _, err := NewC32PrerequisiteCertificate(input); err == nil {
		t.Fatal("prospective evaluation accepted persisted-only candidate coordinates")
	}
}

func TestMemberOfBasisV3RequiresMatchingC32CertificateAndPreservesV2(t *testing.T) {
	fixture := newMemberOfFixture(t)
	observable := memberOfTestObservableInput(
		t,
		"observable:c32/member-of",
		0xd1,
	)
	legacyInput := MemberOfBasisInput{
		Query:                fixture.query,
		EvaluationView:       fixture.evaluationView,
		KindSignature:        fixture.signature,
		EntitySet:            fixture.entitySet,
		ObservableInputs:     []MemberOfObservableInput{observable},
		EvaluationProvenance: fixture.evaluationProvenance,
	}
	legacy, err := NewMemberOfBasis(legacyInput)
	if err != nil {
		t.Fatalf("NewMemberOfBasis(v2): %v", err)
	}
	explicitLegacy, err := NewLegacyMemberOfBasisV2(legacyInput)
	if err != nil {
		t.Fatalf("NewLegacyMemberOfBasisV2(): %v", err)
	}
	if legacy.Posture().Version() != LegacyMemberOfBasisVersionV2 ||
		legacy.Digest() != explicitLegacy.Digest() ||
		!bytes.Equal(legacy.CanonicalBytes(), explicitLegacy.CanonicalBytes()) {
		t.Fatal("legacy constructor no longer reproduces exact v2 canonical bytes")
	}
	if _, present := legacy.MemberOfRequestDigest(); present {
		t.Fatal("legacy basis fabricated a v3 MemberOf request coordinate")
	}

	request, err := NewMemberOfEvaluationRequest(
		fixture.query,
		fixture.evaluationView,
	)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest(): %v", err)
	}
	certificate, err := NewC32PrerequisiteCertificate(
		c32CertificateTestInput(t, fixture, request),
	)
	if err != nil {
		t.Fatalf("NewC32PrerequisiteCertificate(): %v", err)
	}
	v3, err := NewMemberOfBasisV3(MemberOfBasisV3Input{
		Basis:         legacyInput,
		Prerequisites: certificate,
	})
	if err != nil {
		t.Fatalf("NewMemberOfBasisV3(): %v", err)
	}
	if v3.Posture().Version() != C32PrerequisiteMemberOfBasisVersionV3 {
		t.Fatalf("v3 posture = %s", v3.Posture().Version().String())
	}
	requestDigest, present := v3.MemberOfRequestDigest()
	if !present || requestDigest != request.Digest() {
		t.Fatal("v3 basis lost its exact MemberOf request coordinate")
	}
	if legacy.Digest() == v3.Digest() ||
		bytes.Equal(legacy.CanonicalBytes(), v3.CanonicalBytes()) {
		t.Fatal("v2 and v3 MemberOf bases collapsed to one canonical contract")
	}

	legacyMember, err := NewMemberOfMember(fixture.query, legacy)
	if err != nil {
		t.Fatalf("NewMemberOfMember(v2): %v", err)
	}
	v3Member, err := NewMemberOfMember(fixture.query, v3)
	if err != nil {
		t.Fatalf("NewMemberOfMember(v3): %v", err)
	}
	if legacyMember.Digest() == v3Member.Digest() {
		t.Fatal("v2 and v3 defined judgements reused one canonical domain")
	}

	mismatchedInput := c32CertificateTestInput(t, fixture, request)
	mismatchedInput.MemberOfRequestDigest = memberOfTestDigest(t, 0xee)
	mismatched, err := NewC32PrerequisiteCertificate(mismatchedInput)
	if err != nil {
		t.Fatalf("NewC32PrerequisiteCertificate(mismatched): %v", err)
	}
	if _, err := NewMemberOfBasisV3(MemberOfBasisV3Input{
		Basis:         legacyInput,
		Prerequisites: mismatched,
	}); err == nil {
		t.Fatal("MemberOf v3 accepted certificate for another exact request")
	}
}

func c32CertificateTestInput(
	t *testing.T,
	fixture memberOfFixture,
	request MemberOfEvaluationRequest,
) C32PrerequisiteCertificateInput {
	t.Helper()
	return C32PrerequisiteCertificateInput{
		TypeEnv:                  fixture.typeEnv.ref,
		KindSignature:            fixture.signature.Ref(),
		EntitySet:                fixture.entitySet.Ref(),
		ContextSlice:             fixture.query.ContextSlice().Ref(),
		EvaluationView:           request.View(),
		MemberOfRequestDigest:    request.Digest(),
		EnumerationRequestDigest: memberOfTestDigest(t, 0xa1),
		EnumerationResultDigest:  memberOfTestDigest(t, 0xa2),
		EnumerationBasisDigest:   memberOfTestDigest(t, 0xa3),
		EnumerationRule:          fixture.entitySet.EnumerationRule(),
		EnumerationMechanism:     c32TestMechanism(t, 0xa4),
		DefinednessRequestDigest: memberOfTestDigest(t, 0xb1),
		DefinednessResultDigest:  memberOfTestDigest(t, 0xb2),
		DefinednessBasisDigest:   memberOfTestDigest(t, 0xb3),
		DefinednessRule:          fixture.signature.DefinednessRule(),
		DefinednessMechanism:     c32TestMechanism(t, 0xb4),
		CandidateVisibility:      NewC32PersistedVisibilityCoordinate(),
	}
}

func c32TestMechanism(
	t *testing.T,
	seed byte,
) C32EvaluationMechanismIdentity {
	t.Helper()
	identity, err := NewC32EvaluationMechanismIdentity(
		memberOfTestCarrierRef(t, "binary:c32-prerequisite-evaluator"),
		memberOfTestCarrierEdition(t, "build-20260717.1"),
		memberOfTestDigest(t, seed),
	)
	if err != nil {
		t.Fatalf("NewC32EvaluationMechanismIdentity(): %v", err)
	}
	return identity
}
