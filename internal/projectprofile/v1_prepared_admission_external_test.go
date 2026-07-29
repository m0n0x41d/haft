package projectprofile_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

type foreignPreparedProfileAdmissionV1 struct {
	projectprofile.PreparedProfileAdmissionV1
	rawJSON []byte
	digest  projectprofile.ContentDigest
}

type foreignTentativeProfileAdmissionV1 struct {
	projectprofile.TentativeProfileAdmissionTransactionMaterialV1
}

type foreignProfileDeclarationReceiptV1 struct {
	projectprofile.ProfileDeclarationReceiptV1
	rawJSON []byte
	digest  projectprofile.ContentDigest
}

type foreignProfileDeclarationAdmissionRecord struct {
	projectprofile.ProfileDeclarationAdmissionRecord
	rawJSON []byte
	digest  projectprofile.ContentDigest
}

func TestProfileAdmissionPreparationV1BuilderRejectsPartialInputs(t *testing.T) {
	fixture := mustV1Fixture(t)
	provenance := fixture.candidate.Provenance()
	projectRoot := provenance.ProjectRoot()
	planOnly := projectprofile.NewProfileAdmissionPreparationV1Builder(fixture.commitPlan, projectRoot)
	withWork := planOnly.WithWork(fixture.record, fixture.description, fixture.contract)
	withAuthor := withWork.WithProfileAuthor(fixture.assignment, fixture.assignmentSupport)
	withOutcomeWithoutWork := planOnly.WithObservedOutcome(fixture.basis, fixture.effect, fixture.assessment)
	withAuthorWithoutWork := withOutcomeWithoutWork.WithProfileAuthor(fixture.assignment, fixture.assignmentSupport)
	cases := []struct {
		name    string
		builder projectprofile.ProfileAdmissionPreparationV1Builder
	}{
		{name: "zero", builder: projectprofile.ProfileAdmissionPreparationV1Builder{}},
		{name: "plan only", builder: planOnly},
		{name: "missing author and outcome", builder: withWork},
		{name: "missing outcome", builder: withAuthor},
		{name: "missing Work support", builder: withAuthorWithoutWork},
	}
	for _, testCase := range cases {
		_, err := testCase.builder.Build()
		if err == nil {
			t.Fatalf("%s builder unexpectedly prepared admission material", testCase.name)
		}
	}
}

func TestPreparedProfileAdmissionV1IsOpaquePreCommitMaterialOnly(t *testing.T) {
	fixture := mustV1Fixture(t)
	prepared := mustPreparedProfileAdmissionV1(t, fixture)
	err := projectprofile.ValidatePreparedProfileAdmissionV1(prepared)
	if err != nil {
		t.Fatalf("ValidatePreparedProfileAdmissionV1: %v", err)
	}
	preparedType := reflect.TypeOf((*projectprofile.PreparedProfileAdmissionV1)(nil)).Elem()
	forbiddenMethods := []string{
		"DeclaredProfile",
		"Receipt",
		"AdmissionRecord",
		"AdmissionRecordRef",
		"CommittedLedgerRevision",
		"RecordedAt",
		"ReceiptCanonicalJSON",
		"AdmissionRecordCanonicalJSON",
	}
	for _, methodName := range forbiddenMethods {
		if _, found := preparedType.MethodByName(methodName); found {
			t.Fatalf("pre-commit interface exposes %s", methodName)
		}
	}
	raw := any(prepared)
	if _, ok := raw.(projectprofile.DeclaredProjectProfileV1); ok {
		t.Fatal("prepared request implements DeclaredProjectProfileV1")
	}
	if _, ok := raw.(projectprofile.ProfileDeclarationReceiptV1); ok {
		t.Fatal("prepared request implements ProfileDeclarationReceiptV1")
	}
	if _, ok := raw.(projectprofile.ProfileDeclarationAdmissionRecord); ok {
		t.Fatal("prepared request implements ProfileDeclarationAdmissionRecord")
	}
	if prepared.ProjectRoot() != fixture.candidate.Provenance().ProjectRoot() {
		t.Fatal("prepared request changed project root")
	}
	if prepared.ExpectedLedgerRevision() != fixture.inputs.ExpectedLedgerRevision() {
		t.Fatal("prepared request changed expected revision")
	}
	wantPayloadJSON, err := projectprofile.EncodeProfileDeclarationPayloadCanonicalJSON(fixture.payload)
	if err != nil {
		t.Fatalf("EncodeProfileDeclarationPayloadCanonicalJSON: %v", err)
	}
	wantWorkJSON, err := projectprofile.EncodeProfileOnboardingWorkRecordCanonicalJSON(fixture.record)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingWorkRecordCanonicalJSON: %v", err)
	}
	wantAssignmentJSON, err := projectprofile.EncodeProfileAuthorRoleAssignmentV1CanonicalJSON(fixture.assignment)
	if err != nil {
		t.Fatalf("EncodeProfileAuthorRoleAssignmentV1CanonicalJSON: %v", err)
	}
	if !bytes.Equal(prepared.ProfilePayloadCanonicalJSON(), wantPayloadJSON) {
		t.Fatal("prepared payload JSON is not canonical typed output")
	}
	if !bytes.Equal(prepared.WorkRecordCanonicalJSON(), wantWorkJSON) {
		t.Fatal("prepared Work JSON is not canonical typed output")
	}
	if !bytes.Equal(prepared.ProfileAuthorRoleAssignmentCanonicalJSON(), wantAssignmentJSON) {
		t.Fatal("prepared assignment JSON is not canonical typed output")
	}
	canonicalValues := [][]byte{
		prepared.AdmissionRequestCanonicalJSON(),
		prepared.ProfilePayloadCanonicalJSON(),
		prepared.CandidateProvenanceCanonicalJSON(),
		prepared.WorkRecordCanonicalJSON(),
		prepared.MethodDescriptionCanonicalJSON(),
		prepared.MethodContractCanonicalJSON(),
		prepared.ProfileAuthorRoleAssignmentCanonicalJSON(),
		prepared.ExecutorSystemAdmissionCanonicalJSON(),
		prepared.ProfileAuthorRoleAdmissionCanonicalJSON(),
		prepared.AssignmentJustificationCanonicalJSON(),
		prepared.AssignmentProvenanceCanonicalJSON(),
		prepared.ObservedProjectBasisCanonicalJSON(),
		prepared.OnboardingEffectCanonicalJSON(),
		prepared.OutcomeAssessmentCanonicalJSON(),
	}
	for index, value := range canonicalValues {
		if !json.Valid(value) || bytes.ContainsAny(value, "\n\t") {
			t.Fatalf("canonical pre-commit material %d is invalid", index)
		}
	}
	requestJSON := string(prepared.AdmissionRequestCanonicalJSON())
	for _, binding := range []string{
		`"schema":"haft.project-profile.declaration-admission-request/v1"`,
		`"project_root":"/tmp/haft-project"`,
		`"candidate":`,
		`"work_record":`,
		`"method_description":`,
		`"method_contract":`,
		`"profile_author_role_assignment":`,
		`"executor_system_admission":`,
		`"profile_author_role_admission":`,
		`"assignment_justification":`,
		`"assignment_provenance":`,
		`"observed_project_basis":`,
		`"onboarding_effect":`,
		`"outcome_assessment":`,
		`"authority_resolution_record_ref":`,
		`"authority_resolution_record_digest":`,
		`"single_use_key":`,
	} {
		if !strings.Contains(requestJSON, binding) {
			t.Fatalf("canonical request is missing %s", binding)
		}
	}
	for _, forbidden := range []string{
		`"commit_plan":`,
		`"inputs":`,
		`"inputs_digest":`,
		`"plan_digest":`,
		`"expected_ledger_revision":`,
		`"admission_record_ref":`,
		`"committed_ledger_revision":`,
		`"ledger_revision":`,
		`"committed_at":`,
		`"receipt":`,
	} {
		if strings.Contains(requestJSON, forbidden) {
			t.Fatalf("pre-commit request contains transaction-selected fact %s", forbidden)
		}
	}
	mutated := prepared.AdmissionRequestCanonicalJSON()
	mutated[0] = '['
	if bytes.Equal(mutated, prepared.AdmissionRequestCanonicalJSON()) {
		t.Fatal("prepared request leaked mutable bytes")
	}
	if err := projectprofile.ValidatePreparedProfileAdmissionV1(prepared); err != nil {
		t.Fatalf("caller byte mutation corrupted prepared request: %v", err)
	}
}

func TestAdmissionRequestIntentIsStableAcrossLedgerHeadChanges(t *testing.T) {
	fixture := mustV1Fixture(t)
	first := mustPreparedProfileAdmissionV1(t, fixture)
	nextExpected := mustNextLedgerRevisionV1(t, fixture.inputs.ExpectedLedgerRevision())
	nextInputs, err := projectprofile.NewProfileDeclarationAdmissionInputs(
		fixture.candidate,
		nextExpected,
	)
	if err != nil {
		t.Fatalf("NewProfileDeclarationAdmissionInputs(next): %v", err)
	}
	nextPlan, err := projectprofile.NewProfileDeclarationCommitPlan(
		nextInputs,
		fixture.commitPlan.AuthorityResolutionRecordRef(),
		fixture.commitPlan.AuthorityResolutionRecordDigest(),
		fixture.commitPlan.SingleUseKey(),
	)
	if err != nil {
		t.Fatalf("NewProfileDeclarationCommitPlan(next): %v", err)
	}
	provenance := fixture.candidate.Provenance()
	projectRoot := provenance.ProjectRoot()
	builder := profileAdmissionPreparationBuilderV1(fixture, nextPlan, fixture.assessment, projectRoot)
	second, err := builder.Build()
	if err != nil {
		t.Fatalf("ProfileAdmissionPreparationV1Builder.Build(next): %v", err)
	}
	if !bytes.Equal(first.AdmissionRequestCanonicalJSON(), second.AdmissionRequestCanonicalJSON()) {
		t.Fatal("ledger-head change changed stable admission-request intent bytes")
	}
	if first.AdmissionRequestDigest() != second.AdmissionRequestDigest() {
		t.Fatal("ledger-head change changed stable admission-request intent digest")
	}
	if first.CommitPlan().Digest() == second.CommitPlan().Digest() {
		t.Fatal("revision-bound commit plans collapsed across ledger heads")
	}
	firstTentative := mustTentativeProfileAdmissionV1(t, fixture, first)
	admissionRef, err := projectprofile.NewProfileDeclarationAdmissionRecordRef(
		"profile-admission:profile-onboarding:1",
	)
	if err != nil {
		t.Fatalf("NewProfileDeclarationAdmissionRecordRef: %v", err)
	}
	secondTentative, err := projectprofile.PrepareTentativeProfileAdmissionTransactionMaterialV1(
		second,
		mustNextLedgerRevisionV1(t, second.ExpectedLedgerRevision()),
		fixture.record.WorkInterval().Until().Add(time.Hour),
		admissionRef,
	)
	if err != nil {
		t.Fatalf("PrepareTentativeProfileAdmissionTransactionMaterialV1(next): %v", err)
	}
	if firstTentative.TentativeAdmissionRecordDigest() == secondTentative.TentativeAdmissionRecordDigest() {
		t.Fatal("revision-bound tentative material collapsed across ledger heads")
	}
}

func TestTentativeTransactionMaterialDoesNotExposeCommittedSemanticTypes(t *testing.T) {
	fixture := mustV1Fixture(t)
	prepared := mustPreparedProfileAdmissionV1(t, fixture)
	material := mustTentativeProfileAdmissionV1(t, fixture, prepared)
	err := projectprofile.ValidateTentativeProfileAdmissionTransactionMaterialV1(material)
	if err != nil {
		t.Fatalf("ValidateTentativeProfileAdmissionTransactionMaterialV1: %v", err)
	}
	tentativeType := reflect.TypeOf((*projectprofile.TentativeProfileAdmissionTransactionMaterialV1)(nil)).Elem()
	for _, methodName := range []string{
		"DeclaredProfile",
		"Receipt",
		"AdmissionRecord",
		"AdmissionRecordRef",
		"CommittedLedgerRevision",
		"RecordedAt",
	} {
		if _, found := tentativeType.MethodByName(methodName); found {
			t.Fatalf("tentative interface exposes committed semantic accessor %s", methodName)
		}
	}
	raw := any(material)
	if _, ok := raw.(projectprofile.DeclaredProjectProfileV1); ok {
		t.Fatal("tentative transaction material implements DeclaredProjectProfileV1")
	}
	if _, ok := raw.(projectprofile.ProfileDeclarationReceiptV1); ok {
		t.Fatal("tentative transaction material implements ProfileDeclarationReceiptV1")
	}
	if _, ok := raw.(projectprofile.ProfileDeclarationAdmissionRecord); ok {
		t.Fatal("tentative transaction material implements ProfileDeclarationAdmissionRecord")
	}
	if material.Prepared().AdmissionRequestDigest() != prepared.AdmissionRequestDigest() {
		t.Fatal("tentative material changed its pre-commit request")
	}
	if !json.Valid(material.TentativeReceiptCanonicalJSON()) || !json.Valid(material.TentativeAdmissionRecordCanonicalJSON()) {
		t.Fatal("tentative transaction material is not canonical JSON")
	}
	mutated := material.TentativeAdmissionRecordCanonicalJSON()
	mutated[0] = '['
	if bytes.Equal(mutated, material.TentativeAdmissionRecordCanonicalJSON()) {
		t.Fatal("tentative transaction material leaked mutable bytes")
	}
	if err := projectprofile.ValidateTentativeProfileAdmissionTransactionMaterialV1(material); err != nil {
		t.Fatalf("caller byte mutation corrupted tentative material: %v", err)
	}
}

func TestPreparedAndTentativeProfileAdmissionRejectForeignEmbeddingForgeries(t *testing.T) {
	fixture := mustV1Fixture(t)
	prepared := mustPreparedProfileAdmissionV1(t, fixture)
	forgedPrepared := foreignPreparedProfileAdmissionV1{
		PreparedProfileAdmissionV1: prepared,
		rawJSON:                    prepared.AdmissionRequestCanonicalJSON(),
		digest:                     prepared.AdmissionRequestDigest(),
	}
	if err := projectprofile.ValidatePreparedProfileAdmissionV1(forgedPrepared); err == nil {
		t.Fatal("prepared validation accepted a foreign embedded implementation")
	}
	material := mustTentativeProfileAdmissionV1(t, fixture, prepared)
	forgedTentative := foreignTentativeProfileAdmissionV1{
		TentativeProfileAdmissionTransactionMaterialV1: material,
	}
	if err := projectprofile.ValidateTentativeProfileAdmissionTransactionMaterialV1(forgedTentative); err == nil {
		t.Fatal("tentative validation accepted a foreign embedded implementation")
	}
	if err := projectprofile.ValidatePreparedProfileAdmissionV1(nil); err == nil {
		t.Fatal("prepared validation accepted nil")
	}
	if err := projectprofile.ValidateTentativeProfileAdmissionTransactionMaterialV1(nil); err == nil {
		t.Fatal("tentative validation accepted nil")
	}
	_, err := projectprofile.DigestProfileDeclarationReceiptV1(
		foreignProfileDeclarationReceiptV1{
			rawJSON: material.TentativeReceiptCanonicalJSON(),
			digest:  material.TentativeReceiptDigest(),
		},
	)
	if err == nil {
		t.Fatal("receipt digest accepted raw fields in a foreign sealed implementation")
	}
	_, err = projectprofile.DigestProfileDeclarationAdmissionRecord(
		foreignProfileDeclarationAdmissionRecord{
			rawJSON: material.TentativeAdmissionRecordCanonicalJSON(),
			digest:  material.TentativeAdmissionRecordDigest(),
		},
	)
	if err == nil {
		t.Fatal("admission digest accepted raw fields in a foreign sealed implementation")
	}
}

func TestPreparedProfileAdmissionV1BindsExactSupportClosureAndPassedAssessment(t *testing.T) {
	fixture := mustV1Fixture(t)
	prepared := mustPreparedProfileAdmissionV1(t, fixture)
	if prepared.ProfileAuthorRoleAssignmentDigest() != fixture.candidate.Provenance().ProfileAuthorRoleAssignmentDigest() {
		t.Fatal("prepared assignment digest differs from candidate provenance")
	}
	if prepared.ObservedProjectBasisDigest() != fixture.candidate.Provenance().ObservedProjectBasisDigest() {
		t.Fatal("prepared basis digest differs from candidate provenance")
	}
	if prepared.OutcomeAssessmentDigest() != fixture.candidate.Provenance().OutcomeAssessmentDigest() {
		t.Fatal("prepared assessment digest differs from candidate provenance")
	}
	reasonRef, err := projectprofile.NewProfileOnboardingAcceptanceReasonRefV1(
		"acceptance-reason:failed",
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingAcceptanceReasonRefV1: %v", err)
	}
	failedVerdict, err := projectprofile.NewProfileOnboardingAcceptanceFailedV1(reasonRef)
	if err != nil {
		t.Fatalf("NewProfileOnboardingAcceptanceFailedV1: %v", err)
	}
	failedAssessment := mustOutcomeAssessmentWithVerdictV1(t, fixture.effect, fixture.contract, failedVerdict)
	provenance := fixture.candidate.Provenance()
	projectRoot := provenance.ProjectRoot()
	builder := profileAdmissionPreparationBuilderV1(fixture, fixture.commitPlan, failedAssessment, projectRoot)
	_, err = builder.Build()
	if err == nil || !strings.Contains(err.Error(), "passed outcome assessment") {
		t.Fatalf("preparation accepted failed outcome assessment: %v", err)
	}
}

func TestTentativeTransactionFactsDoNotEnterAdmissionRequestDigest(t *testing.T) {
	fixture := mustV1Fixture(t)
	prepared := mustPreparedProfileAdmissionV1(t, fixture)
	first := mustTentativeProfileAdmissionV1(t, fixture, prepared)
	otherRef, err := projectprofile.NewProfileDeclarationAdmissionRecordRef("profile-admission:other")
	if err != nil {
		t.Fatalf("NewProfileDeclarationAdmissionRecordRef: %v", err)
	}
	other, err := projectprofile.PrepareTentativeProfileAdmissionTransactionMaterialV1(
		prepared,
		mustNextLedgerRevisionV1(t, prepared.ExpectedLedgerRevision()),
		fixture.record.WorkInterval().Until().Add(time.Hour+time.Second),
		otherRef,
	)
	if err != nil {
		t.Fatalf("PrepareTentativeProfileAdmissionTransactionMaterialV1(other): %v", err)
	}
	if first.Prepared().AdmissionRequestDigest() != other.Prepared().AdmissionRequestDigest() {
		t.Fatal("transaction facts changed the non-binding request digest")
	}
	if first.TentativeAdmissionRecordDigest() == other.TentativeAdmissionRecordDigest() {
		t.Fatal("distinct transaction facts collapsed to one tentative admission record")
	}
}

func TestProfileAdmissionPreparationRejectsSourceAndTransactionMismatches(t *testing.T) {
	fixture := mustV1Fixture(t)
	wrongRoot, err := projectprofile.NewProjectRootV1("/tmp/another-project")
	if err != nil {
		t.Fatalf("NewProjectRootV1: %v", err)
	}
	builder := profileAdmissionPreparationBuilderV1(fixture, fixture.commitPlan, fixture.assessment, wrongRoot)
	_, err = builder.Build()
	if err == nil {
		t.Fatal("preparation accepted a different project root")
	}
	prepared := mustPreparedProfileAdmissionV1(t, fixture)
	admissionRef, err := projectprofile.NewProfileDeclarationAdmissionRecordRef("profile-admission:bad")
	if err != nil {
		t.Fatalf("NewProfileDeclarationAdmissionRecordRef: %v", err)
	}
	_, err = projectprofile.PrepareTentativeProfileAdmissionTransactionMaterialV1(
		prepared,
		fixture.inputs.ExpectedLedgerRevision(),
		fixture.record.WorkInterval().Until().Add(time.Hour),
		admissionRef,
	)
	if err == nil {
		t.Fatal("tentative material accepted a stale ledger revision")
	}
	committedRevision, err := fixture.inputs.ExpectedLedgerRevision().Next()
	if err != nil {
		t.Fatalf("LedgerRevision.Next: %v", err)
	}
	_, err = projectprofile.PrepareTentativeProfileAdmissionTransactionMaterialV1(
		prepared,
		committedRevision,
		fixture.record.WorkInterval().Until().Add(-time.Nanosecond),
		admissionRef,
	)
	if err == nil {
		t.Fatal("tentative material accepted a time before completed Work")
	}
}

func TestFinalV1CandidateProvenanceBindsExactProfileAuthorRoleAssignment(t *testing.T) {
	fixture := mustV1Fixture(t)
	provenance := fixture.candidate.Provenance()
	assignmentDigest, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(fixture.assignment)
	if err != nil {
		t.Fatalf("DigestProfileAuthorRoleAssignmentV1: %v", err)
	}
	if provenance.ProfileAuthorRoleAssignmentRef() != fixture.assignment.RoleAssignmentRef() ||
		provenance.ProfileAuthorRoleAssignmentDigest() != assignmentDigest {
		t.Fatal("candidate provenance does not bind the exact ProfileAuthorRoleAssignment")
	}
}

func mustOutcomeAssessmentWithVerdictV1(
	t testing.TB,
	effect projectprofile.ProfileOnboardingEffectV1,
	contract projectprofile.ProfileOnboardingMethodContractV1,
	verdict projectprofile.ProfileOnboardingAcceptanceVerdictV1,
) projectprofile.ProfileOnboardingOutcomeAssessmentV1 {
	t.Helper()
	assessmentRef, err := projectprofile.NewProfileOnboardingOutcomeAssessmentRefV1(
		"assessment:profile-onboarding:alternate",
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingOutcomeAssessmentRefV1: %v", err)
	}
	standardEdition, err := projectprofile.NewProfileOnboardingAcceptanceStandardEditionV1(
		contract.AcceptanceStandardEdition(),
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingAcceptanceStandardEditionV1: %v", err)
	}
	comparatorRef, err := projectprofile.NewProfileOnboardingComparatorRefV1(
		"comparator:profile-onboarding:alternate",
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingComparatorRefV1: %v", err)
	}
	comparatorEdition, err := projectprofile.NewProfileOnboardingComparatorEditionV1("v1")
	if err != nil {
		t.Fatalf("NewProfileOnboardingComparatorEditionV1: %v", err)
	}
	evidenceRef, err := projectprofile.NewEvidenceProvenancePathRefV1(
		"evidence:path:profile-onboarding-assessment:alternate",
	)
	if err != nil {
		t.Fatalf("NewEvidenceProvenancePathRefV1: %v", err)
	}
	assessment, err := projectprofile.NewProfileOnboardingOutcomeAssessmentV1(
		assessmentRef,
		effect,
		contract.AcceptanceStandardRef(),
		standardEdition,
		comparatorRef,
		comparatorEdition,
		verdict,
		[]projectprofile.EvidenceProvenancePathRefV1{evidenceRef},
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingOutcomeAssessmentV1: %v", err)
	}
	return assessment
}

func mustPreparedProfileAdmissionV1(
	t testing.TB,
	fixture v1Fixture,
) projectprofile.PreparedProfileAdmissionV1 {
	t.Helper()
	provenance := fixture.candidate.Provenance()
	projectRoot := provenance.ProjectRoot()
	builder := profileAdmissionPreparationBuilderV1(fixture, fixture.commitPlan, fixture.assessment, projectRoot)
	prepared, err := builder.Build()
	if err != nil {
		t.Fatalf("ProfileAdmissionPreparationV1Builder.Build: %v", err)
	}
	return prepared
}

func profileAdmissionPreparationBuilderV1(
	fixture v1Fixture,
	plan projectprofile.ProfileDeclarationCommitPlan,
	assessment projectprofile.ProfileOnboardingOutcomeAssessmentV1,
	projectRoot projectprofile.ProjectRootV1,
) projectprofile.ProfileAdmissionPreparationV1Builder {
	builder := projectprofile.NewProfileAdmissionPreparationV1Builder(plan, projectRoot)
	builder = builder.WithWork(fixture.record, fixture.description, fixture.contract)
	builder = builder.WithProfileAuthor(fixture.assignment, fixture.assignmentSupport)
	builder = builder.WithObservedOutcome(fixture.basis, fixture.effect, assessment)
	return builder
}

func mustTentativeProfileAdmissionV1(
	t testing.TB,
	fixture v1Fixture,
	prepared projectprofile.PreparedProfileAdmissionV1,
) projectprofile.TentativeProfileAdmissionTransactionMaterialV1 {
	t.Helper()
	admissionRef, err := projectprofile.NewProfileDeclarationAdmissionRecordRef(
		"profile-admission:profile-onboarding:1",
	)
	if err != nil {
		t.Fatalf("NewProfileDeclarationAdmissionRecordRef: %v", err)
	}
	committedRevision, err := fixture.inputs.ExpectedLedgerRevision().Next()
	if err != nil {
		t.Fatalf("LedgerRevision.Next: %v", err)
	}
	recordedAt := fixture.record.WorkInterval().Until().Add(time.Hour)
	material, err := projectprofile.PrepareTentativeProfileAdmissionTransactionMaterialV1(
		prepared,
		committedRevision,
		recordedAt,
		admissionRef,
	)
	if err != nil {
		t.Fatalf("PrepareTentativeProfileAdmissionTransactionMaterialV1: %v", err)
	}
	return material
}

func mustNextLedgerRevisionV1(
	t testing.TB,
	revision projectprofile.LedgerRevision,
) projectprofile.LedgerRevision {
	t.Helper()
	next, err := revision.Next()
	if err != nil {
		t.Fatalf("LedgerRevision.Next: %v", err)
	}
	return next
}
