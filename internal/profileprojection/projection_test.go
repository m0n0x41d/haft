package profileprojection

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectprofile"
	"gopkg.in/yaml.v3"
)

func TestBuildProjectionIsDeterministicHumanReadableAndNonAuthoritative(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "project")
	material := projectionMaterialFixture(t, rootPath)
	first, err := buildProjection(material)
	if err != nil {
		t.Fatalf("build first projection: %v", err)
	}
	second, err := buildProjection(material)
	if err != nil {
		t.Fatalf("build second projection: %v", err)
	}
	if !bytes.Equal(first.content, second.content) {
		t.Fatal("same canonical admission material produced different projection bytes")
	}
	if first.digest != second.digest {
		t.Fatal("same canonical admission material produced different projection digests")
	}
	text := string(first.content)
	for _, expected := range []string{
		"Human-readable projection only: not authority and not admission proof",
		"schema: haft.project-profile-projection/v1",
		"canonical_source: sqlite_profile_admission_ledger",
		"profile_kind: Declared",
		"ledger_revision: 7",
		"kind: software",
		"scope_id: software.product",
		"kind: non_software",
		"scope_id: documents.model",
		"kind_admission:\n      kind: admitted\n      ref: U.Model",
		"governing_pattern_refs:",
		"contract_refs:",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("projection is missing %q:\n%s", expected, text)
		}
	}
	for _, forbidden := range []string{
		"single_use_key",
		"admission_record_canonical_json",
		"authorization_content",
		"kind_orientation",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("projection leaked forbidden admission field %q", forbidden)
		}
	}
	decoder := yaml.NewDecoder(bytes.NewReader(first.content))
	decoder.KnownFields(true)
	var document projectionDocumentV1
	if err := decoder.Decode(&document); err != nil {
		t.Fatalf("decode generated projection strictly: %v", err)
	}
	if document.Schema != ProjectionSchemaV1 {
		t.Fatalf("schema = %q, want %q", document.Schema, ProjectionSchemaV1)
	}
	if document.SemanticRole != "human_readable_projection" {
		t.Fatalf("semantic role = %q", document.SemanticRole)
	}
}

func TestExactProjectionVerificationRejectsWholeCarrierDrift(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "project")
	expected, err := buildProjection(projectionMaterialFixture(t, rootPath))
	if err != nil {
		t.Fatalf("build projection: %v", err)
	}
	if err := verifyExactProjectionBytes(expected, expected.content); err != nil {
		t.Fatalf("exact projection rejected: %v", err)
	}
	drifted := bytes.Replace(
		expected.content,
		[]byte("profile_kind: Declared"),
		[]byte("profile_kind: Auto"),
		1,
	)
	if bytes.Equal(drifted, expected.content) {
		t.Fatal("test setup did not change projection bytes")
	}
	if err := verifyExactProjectionBytes(expected, drifted); err == nil {
		t.Fatal("whole-carrier projection drift unexpectedly passed")
	}
}

func TestProjectionPathIsCanonicalAndRejectsZeroRoot(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "project")
	root := mustProjectRoot(t, rootPath)
	path, err := ProjectionPath(root)
	if err != nil {
		t.Fatalf("ProjectionPath: %v", err)
	}
	expectedPath := filepath.Join(rootPath, ".haft", "project-profile.yaml")
	if path != expectedPath {
		t.Fatalf("projection path = %q", path)
	}
	if _, err := ProjectionPath(projectprofile.ProjectRootV1{}); err == nil {
		t.Fatal("zero project root produced a projection path")
	}
}

func projectionMaterialFixture(t *testing.T, rootText string) projectionMaterial {
	t.Helper()
	root := mustProjectRoot(t, rootText)
	scopeID, err := projectprofile.NewScopeID("software.product")
	if err != nil {
		t.Fatalf("NewScopeID: %v", err)
	}
	scope, err := projectprofile.NewSoftwareRealization(
		scopeID,
		projectprofile.NoEntityReference{},
	)
	if err != nil {
		t.Fatalf("NewSoftwareRealization: %v", err)
	}
	documentScopeID, err := projectprofile.NewScopeID("documents.model")
	if err != nil {
		t.Fatalf("NewScopeID documents: %v", err)
	}
	kindRef, err := projectprofile.NewKindRef("U.Model")
	if err != nil {
		t.Fatalf("NewKindRef: %v", err)
	}
	patternRef, err := projectprofile.NewSourceUnitRef("A.1.1")
	if err != nil {
		t.Fatalf("NewSourceUnitRef: %v", err)
	}
	contractRef, err := projectprofile.NewSpecSectionRef("TS.boundary.001")
	if err != nil {
		t.Fatalf("NewSpecSectionRef: %v", err)
	}
	documentScope, err := projectprofile.NewNonSoftwareRealization(
		documentScopeID,
		projectprofile.NoEntityReference{},
		projectprofile.NewReferencedKindOrientation(kindRef),
		[]projectprofile.SourceUnitRef{patternRef},
		[]projectprofile.SpecSectionRef{contractRef},
	)
	if err != nil {
		t.Fatalf("NewNonSoftwareRealization: %v", err)
	}
	scopes, err := projectprofile.NewScopeSet([]projectprofile.RealizationScope{
		scope,
		documentScope,
	})
	if err != nil {
		t.Fatalf("NewScopeSet: %v", err)
	}
	payload, err := projectprofile.NewProfileDeclarationPayload(scopes)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload: %v", err)
	}
	payloadDigest, err := projectprofile.DigestProfileDeclarationPayload(payload)
	if err != nil {
		t.Fatalf("DigestProfileDeclarationPayload: %v", err)
	}
	digest := mustContentDigest(t, "sha256:"+strings.Repeat("1", 64))
	admissionRef, err := projectprofile.NewProfileDeclarationAdmissionRecordRef("admission/profile-7")
	if err != nil {
		t.Fatalf("NewProfileDeclarationAdmissionRecordRef: %v", err)
	}
	workRef, err := projectprofile.NewProfileOnboardingWorkRecordRef("work/profile-7")
	if err != nil {
		t.Fatalf("NewProfileOnboardingWorkRecordRef: %v", err)
	}
	authorityRef, err := projectprofile.NewProfileDeclarationAuthorityBasisRef("authority/profile-7")
	if err != nil {
		t.Fatalf("NewProfileDeclarationAuthorityBasisRef: %v", err)
	}
	resolutionRef, err := projectprofile.NewAuthorityResolutionRecordRef("authority-resolution/profile-7")
	if err != nil {
		t.Fatalf("NewAuthorityResolutionRecordRef: %v", err)
	}
	assignmentRef, err := projectprofile.NewRoleAssignmentRef("role-assignment/profile-author")
	if err != nil {
		t.Fatalf("NewRoleAssignmentRef: %v", err)
	}
	basisRef, err := projectprofile.NewObservedProjectBasisRefV1("observed-basis/profile-7")
	if err != nil {
		t.Fatalf("NewObservedProjectBasisRefV1: %v", err)
	}
	assessmentRef, err := projectprofile.NewProfileOnboardingOutcomeAssessmentRefV1("outcome/profile-7")
	if err != nil {
		t.Fatalf("NewProfileOnboardingOutcomeAssessmentRefV1: %v", err)
	}
	return projectionMaterial{
		projectRoot:                       root,
		payload:                           payload,
		payloadDigest:                     payloadDigest,
		admissionRecordRef:                admissionRef,
		admissionRecordDigest:             digest,
		receiptCanonicalJSON:              []byte(`{"schema":"haft.project-profile.declaration-receipt/v1"}`),
		receiptDigest:                     digest,
		candidateProvenanceDigest:         digest,
		workRecordRef:                     workRef,
		workRecordDigest:                  digest,
		authorityBasisRef:                 authorityRef,
		authorityBasisDigest:              digest,
		authorityResolutionRef:            resolutionRef,
		authorityResolutionDigest:         digest,
		profileAuthorRoleAssignmentRef:    assignmentRef,
		profileAuthorRoleAssignmentDigest: digest,
		observedProjectBasisRef:           basisRef,
		observedProjectBasisDigest:        digest,
		outcomeAssessmentRef:              assessmentRef,
		outcomeAssessmentDigest:           digest,
		ledgerRevision:                    projectprofile.NewLedgerRevision(7),
		recordedAt:                        time.Date(2026, time.July, 15, 4, 5, 6, 7, time.UTC),
	}
}

func mustProjectRoot(t *testing.T, value string) projectprofile.ProjectRootV1 {
	t.Helper()
	root, err := projectprofile.NewProjectRootV1(value)
	if err != nil {
		t.Fatalf("NewProjectRootV1(%q): %v", value, err)
	}
	return root
}

func mustContentDigest(t *testing.T, value string) projectprofile.ContentDigest {
	t.Helper()
	digest, err := projectprofile.NewContentDigest(value)
	if err != nil {
		t.Fatalf("NewContentDigest: %v", err)
	}
	return digest
}
