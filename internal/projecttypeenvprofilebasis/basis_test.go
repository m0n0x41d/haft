package projecttypeenvprofilebasis

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func TestNoCanonicalProjectProfileIsDeterministicExactAbsence(t *testing.T) {
	root := mustProjectRoot(t, "/tmp/haft-profile-basis")
	left, err := NewNoCanonicalProjectProfile(root)
	if err != nil {
		t.Fatalf("NewNoCanonicalProjectProfile(left): %v", err)
	}
	right, err := NewNoCanonicalProjectProfile(root)
	if err != nil {
		t.Fatalf("NewNoCanonicalProjectProfile(right): %v", err)
	}
	if left.LedgerRevision().Value() != 0 {
		t.Fatalf("absence revision = %d, want 0", left.LedgerRevision().Value())
	}
	if left.Digest() != right.Digest() ||
		left.ProfileLedgerDigest() != right.ProfileLedgerDigest() ||
		left.ProfileBasisRef() != right.ProfileBasisRef() {
		t.Fatal("same exact absence did not produce identical coordinates")
	}
	if err := left.Verify(); err != nil {
		t.Fatalf("Verify(): %v", err)
	}
}

func TestDeclaredCanonicalProjectProfileBindsPayloadRevisionAndSupportDAG(t *testing.T) {
	input := declaredBasisInput(t, softwarePayload(t), 1)
	left, err := NewDeclaredCanonicalProjectProfile(input)
	if err != nil {
		t.Fatalf("NewDeclaredCanonicalProjectProfile(left): %v", err)
	}
	right, err := NewDeclaredCanonicalProjectProfile(input)
	if err != nil {
		t.Fatalf("NewDeclaredCanonicalProjectProfile(right): %v", err)
	}
	if left.Digest() != right.Digest() ||
		left.SupportDAGDigest() != right.SupportDAGDigest() {
		t.Fatal("same declared basis did not produce identical coordinates")
	}
	if err := left.Verify(); err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	payloadBytes := left.PayloadCanonicalJSON()
	payloadBytes[0] ^= 0xff
	if string(payloadBytes) == string(left.PayloadCanonicalJSON()) {
		t.Fatal("PayloadCanonicalJSON leaked mutable storage")
	}
	admissionBytes := left.AdmissionRecordCanonicalJSON()
	admissionBytes[0] ^= 0xff
	if string(admissionBytes) == string(left.AdmissionRecordCanonicalJSON()) {
		t.Fatal("AdmissionRecordCanonicalJSON leaked mutable storage")
	}
	receiptBytes := left.ReceiptCanonicalJSON()
	receiptBytes[0] ^= 0xff
	if string(receiptBytes) == string(left.ReceiptCanonicalJSON()) {
		t.Fatal("ReceiptCanonicalJSON leaked mutable storage")
	}

	changedRevision := input
	changedRevision.LedgerRevision = projectprofile.NewLedgerRevision(2)
	revisionBasis, err := NewDeclaredCanonicalProjectProfile(changedRevision)
	if err != nil {
		t.Fatalf("NewDeclaredCanonicalProjectProfile(revision): %v", err)
	}
	if left.Digest() == revisionBasis.Digest() ||
		left.ProfileLedgerDigest() == revisionBasis.ProfileLedgerDigest() {
		t.Fatal("ledger revision did not change basis and ledger identities")
	}

	changedSupport := input
	changedSupport.WorkRecordDigest = mustContentDigest(t, "b")
	supportBasis, err := NewDeclaredCanonicalProjectProfile(changedSupport)
	if err != nil {
		t.Fatalf("NewDeclaredCanonicalProjectProfile(support): %v", err)
	}
	if left.SupportDAGDigest() == supportBasis.SupportDAGDigest() ||
		left.Digest() == supportBasis.Digest() {
		t.Fatal("support-DAG change did not change support and basis identities")
	}
	if left.ProfileLedgerDigest() != supportBasis.ProfileLedgerDigest() {
		t.Fatal("support-DAG change unexpectedly changed exact ledger-head identity")
	}
}

func TestDeclaredCanonicalProjectProfileRejectsZeroRevisionAndInvalidCarrier(t *testing.T) {
	input := declaredBasisInput(t, softwarePayload(t), 0)
	if _, err := NewDeclaredCanonicalProjectProfile(input); err == nil {
		t.Fatal("zero declared ledger revision was accepted")
	}
	input = declaredBasisInput(t, softwarePayload(t), 1)
	input.ReceiptCanonicalJSON = []byte("not-json")
	if _, err := NewDeclaredCanonicalProjectProfile(input); err == nil {
		t.Fatal("invalid receipt carrier was accepted")
	}
}

func declaredBasisInput(
	t *testing.T,
	payload projectprofile.ProfileDeclarationPayload,
	revision uint64,
) DeclaredProjectProfileBasisInput {
	t.Helper()
	return DeclaredProjectProfileBasisInput{
		ProjectRoot:                       mustProjectRoot(t, "/tmp/haft-profile-basis"),
		LedgerRevision:                    projectprofile.NewLedgerRevision(revision),
		Payload:                           payload,
		AdmissionRecordRef:                mustAdmissionRef(t, "profile-admission:test"),
		AdmissionRecordDigest:             mustContentDigest(t, "1"),
		AdmissionRecordCanonicalJSON:      []byte(`{"schema":"test-admission/v1"}`),
		ReceiptDigest:                     mustContentDigest(t, "2"),
		ReceiptCanonicalJSON:              []byte(`{"schema":"test-receipt/v1"}`),
		CandidateProvenanceDigest:         mustContentDigest(t, "3"),
		WorkRecordRef:                     mustWorkRef(t, "work:test"),
		WorkRecordDigest:                  mustContentDigest(t, "4"),
		AuthorityBasisRef:                 mustAuthorityBasisRef(t, "authority-basis:test"),
		AuthorityBasisDigest:              mustContentDigest(t, "5"),
		AuthorityResolutionRef:            mustAuthorityResolutionRef(t, "authority-resolution:test"),
		AuthorityResolutionDigest:         mustContentDigest(t, "6"),
		ProfileAuthorRoleAssignmentRef:    mustRoleAssignmentRef(t, "role-assignment:test"),
		ProfileAuthorRoleAssignmentDigest: mustContentDigest(t, "7"),
		ObservedProjectBasisRef:           mustObservedBasisRef(t, "observed-basis:test"),
		ObservedProjectBasisDigest:        mustContentDigest(t, "8"),
		OutcomeAssessmentRef:              mustOutcomeAssessmentRef(t, "outcome:test"),
		OutcomeAssessmentDigest:           mustContentDigest(t, "9"),
	}
}

func softwarePayload(t *testing.T) projectprofile.ProfileDeclarationPayload {
	t.Helper()
	scopeID, err := projectprofile.NewScopeID("software")
	if err != nil {
		t.Fatal(err)
	}
	scope, err := projectprofile.NewSoftwareRealization(
		scopeID,
		projectprofile.NoEntityReference{},
	)
	if err != nil {
		t.Fatal(err)
	}
	scopes, err := projectprofile.NewScopeSet([]projectprofile.RealizationScope{scope})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := projectprofile.NewProfileDeclarationPayload(scopes)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func mustProjectRoot(t *testing.T, raw string) projectprofile.ProjectRootV1 {
	t.Helper()
	value, err := projectprofile.NewProjectRootV1(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustContentDigest(t *testing.T, fill string) projectprofile.ContentDigest {
	t.Helper()
	value, err := projectprofile.NewContentDigest("sha256:" + strings.Repeat(fill, 64))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustAdmissionRef(t *testing.T, raw string) projectprofile.ProfileDeclarationAdmissionRecordRef {
	t.Helper()
	value, err := projectprofile.NewProfileDeclarationAdmissionRecordRef(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustWorkRef(t *testing.T, raw string) projectprofile.ProfileOnboardingWorkRecordRef {
	t.Helper()
	value, err := projectprofile.NewProfileOnboardingWorkRecordRef(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustAuthorityBasisRef(t *testing.T, raw string) projectprofile.ProfileDeclarationAuthorityBasisRef {
	t.Helper()
	value, err := projectprofile.NewProfileDeclarationAuthorityBasisRef(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustAuthorityResolutionRef(t *testing.T, raw string) projectprofile.AuthorityResolutionRecordRef {
	t.Helper()
	value, err := projectprofile.NewAuthorityResolutionRecordRef(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustRoleAssignmentRef(t *testing.T, raw string) projectprofile.RoleAssignmentRef {
	t.Helper()
	value, err := projectprofile.NewRoleAssignmentRef(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustObservedBasisRef(t *testing.T, raw string) projectprofile.ObservedProjectBasisRefV1 {
	t.Helper()
	value, err := projectprofile.NewObservedProjectBasisRefV1(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustOutcomeAssessmentRef(
	t *testing.T,
	raw string,
) projectprofile.ProfileOnboardingOutcomeAssessmentRefV1 {
	t.Helper()
	value, err := projectprofile.NewProfileOnboardingOutcomeAssessmentRefV1(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
