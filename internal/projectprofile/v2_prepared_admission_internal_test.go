package projectprofile

import (
	"strings"
	"testing"
)

type internalV2AdmissionFixture struct {
	plan              ProfileDeclarationCommitPlan
	record            ProfileOnboardingWorkRecord
	description       ProfileOnboardingMethodDescriptionV2
	contract          ProfileOnboardingMethodContractV2
	assignment        ProfileAuthorRoleAssignmentV1
	assignmentSupport ProfileAuthorAssignmentSupportCarrierV1
	basis             ObservedProjectBasisV1
	effect            ProfileOnboardingEffectV1
	assessment        ProfileOnboardingOutcomeAssessmentV1
	workInputRef      WorkInputRef
}

func TestProfileAdmissionPreparationV2AcceptsOneExactEditionClosure(t *testing.T) {
	fixture := internalProfileAdmissionFixtureV2(t)
	projectRoot := fixture.plan.Inputs().Candidate().Provenance().ProjectRoot()
	builder := NewProfileAdmissionPreparationV1Builder(fixture.plan, projectRoot)
	builder = builder.WithWorkV2(fixture.record, fixture.description, fixture.contract)
	builder = builder.WithProfileAuthor(fixture.assignment, fixture.assignmentSupport)
	builder = builder.WithObservedOutcome(fixture.basis, fixture.effect, fixture.assessment)
	prepared, err := builder.Build()
	if err != nil {
		t.Fatalf("ProfileAdmissionPreparationV1Builder.Build(v2): %v", err)
	}
	if err := ValidatePreparedProfileAdmissionV1(prepared); err != nil {
		t.Fatalf("ValidatePreparedProfileAdmissionV1(v2): %v", err)
	}
	description, descriptionOK := prepared.MethodDescriptionV2()
	contract, contractOK := prepared.MethodContractV2()
	if !descriptionOK || !contractOK {
		t.Fatal("prepared v2 admission did not retain its exact method edition")
	}
	if description.Ref() != fixture.description.Ref() || contract.Ref() != fixture.contract.Ref() {
		t.Fatal("prepared v2 admission changed exact method refs")
	}
	actualWorkInputRef, ok := prepared.WorkRecord().ProfileOnboardingWorkInputRefV2()
	if !ok || actualWorkInputRef != fixture.workInputRef {
		t.Fatal("prepared v2 admission changed its reviewed WorkInput ref")
	}
}

func TestProfileAdmissionPreparationV2RejectsMixedMethodOrActorEditions(t *testing.T) {
	fixture := internalProfileAdmissionFixtureV2(t)
	projectRoot := fixture.plan.Inputs().Candidate().Provenance().ProjectRoot()
	v1 := internalV1AdmissionFixture(t)

	mixedMethodBuilder := NewProfileAdmissionPreparationV1Builder(fixture.plan, projectRoot)
	mixedMethodBuilder.input.workRecord = fixture.record
	mixedMethodBuilder.input.methodDescription = fixture.description
	mixedMethodBuilder.input.methodContract = v1.contract
	mixedMethodBuilder = mixedMethodBuilder.WithProfileAuthor(
		fixture.assignment,
		fixture.assignmentSupport,
	)
	mixedMethodBuilder = mixedMethodBuilder.WithObservedOutcome(
		fixture.basis,
		fixture.effect,
		fixture.assessment,
	)
	_, err := mixedMethodBuilder.Build()
	if err == nil || !strings.Contains(err.Error(), "editions differ") {
		t.Fatalf("mixed v2 description/v1 contract was not rejected exactly: %v", err)
	}

	mixedActorBuilder := NewProfileAdmissionPreparationV1Builder(fixture.plan, projectRoot)
	mixedActorBuilder = mixedActorBuilder.WithWorkV2(
		fixture.record,
		fixture.description,
		fixture.contract,
	)
	mixedActorBuilder = mixedActorBuilder.WithProfileAuthor(v1.assignment, v1.assignmentSupport)
	mixedActorBuilder = mixedActorBuilder.WithObservedOutcome(
		fixture.basis,
		fixture.effect,
		fixture.assessment,
	)
	_, err = mixedActorBuilder.Build()
	if err == nil || !strings.Contains(err.Error(), "v2") {
		t.Fatalf("v2 Work with v1 actor pins was not rejected exactly: %v", err)
	}
}

func internalProfileAdmissionFixtureV2(t *testing.T) internalV2AdmissionFixture {
	t.Helper()
	base := internalV1AdmissionFixture(t)
	description := ProfileOnboardingMethodDescriptionV2Value()
	descriptionDigest, err := DigestProfileOnboardingMethodDescriptionV2(description)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingMethodDescriptionV2: %v", err)
	}
	contract, err := ProfileOnboardingMethodContractV2Value()
	if err != nil {
		t.Fatalf("ProfileOnboardingMethodContractV2Value: %v", err)
	}
	contractDigest, err := DigestProfileOnboardingMethodContractV2(contract)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingMethodContractV2: %v", err)
	}
	assignment, support := internalProfileAuthorSupportV2(t, base)
	assignmentDigest, err := DigestProfileAuthorRoleAssignmentV1(assignment)
	if err != nil {
		t.Fatalf("DigestProfileAuthorRoleAssignmentV1(v2 pins): %v", err)
	}
	workInputRef, err := NewWorkInputRef("profile-onboarding-work-input:reviewed-v3")
	if err != nil {
		t.Fatalf("NewWorkInputRef: %v", err)
	}
	record := base.record
	record.enactsMethodRef = description.DescribedMethodRef()
	record.methodDescriptionRef = description.Ref()
	record.methodDescriptionDigest = descriptionDigest
	record.methodContractRef = contract.Ref()
	record.methodContractDigest = contractDigest
	record.profileAuthorRoleAssignmentRef = assignment.RoleAssignmentRef()
	record.profileAuthorRoleAssignmentDigest = assignmentDigest
	basisInputRef, err := NewWorkInputRef(base.basis.Ref().String())
	if err != nil {
		t.Fatalf("NewWorkInputRef(ObservedProjectBasis): %v", err)
	}
	record.inputRefs = []WorkInputRef{basisInputRef, workInputRef}
	record, err = canonicalizeProfileOnboardingWorkRecord(record)
	if err != nil {
		t.Fatalf("canonicalizeProfileOnboardingWorkRecord(v2): %v", err)
	}
	workDigest, err := DigestProfileOnboardingWorkRecord(record)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingWorkRecord(v2): %v", err)
	}
	effect := internalProfileOnboardingEffectForWorkV2(t, base.effect, record, workDigest)
	assessment := internalProfileOnboardingAssessmentForEffectV2(t, base.assessment, effect)
	assessmentDigest, err := DigestProfileOnboardingOutcomeAssessmentV1(assessment)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingOutcomeAssessmentV1(v2): %v", err)
	}
	candidate := base.plan.Inputs().Candidate()
	provenance := candidate.Provenance()
	provenanceBuilder := NewCandidateProvenanceV1Builder(
		provenance.AuthorityBasisRef(),
		record.RecordRef(),
		workDigest,
	)
	provenanceBuilder = provenanceBuilder.ForProfileAuthorRoleAssignment(
		assignment.RoleAssignmentRef(),
		assignmentDigest,
	)
	provenanceBuilder = provenanceBuilder.ForProject(provenance.ProjectRoot())
	provenanceBuilder = provenanceBuilder.ClassifiedBy(
		provenance.ClassifierVersion(),
		provenance.PolicyVersion(),
	)
	provenanceBuilder = provenanceBuilder.InSession(provenance.SessionRef())
	provenanceBuilder = provenanceBuilder.ForPayload(provenance.PayloadDigest())
	provenanceBuilder = provenanceBuilder.ForObservedProjectBasis(
		base.basis.Ref(),
		provenance.ObservedProjectBasisDigest(),
	)
	provenanceBuilder = provenanceBuilder.ForOutcomeAssessment(
		assessment.Ref(),
		assessmentDigest,
	)
	provenance, err = provenanceBuilder.Build()
	if err != nil {
		t.Fatalf("CandidateProvenanceV1Builder.Build(v2): %v", err)
	}
	candidate, err = NewProfileDeclarationCandidateV1(candidate.Payload(), provenance)
	if err != nil {
		t.Fatalf("NewProfileDeclarationCandidateV1(v2): %v", err)
	}
	inputs, err := NewProfileDeclarationAdmissionInputs(
		candidate,
		base.plan.Inputs().ExpectedLedgerRevision(),
	)
	if err != nil {
		t.Fatalf("NewProfileDeclarationAdmissionInputs(v2): %v", err)
	}
	plan, err := NewProfileDeclarationCommitPlan(
		inputs,
		base.plan.AuthorityResolutionRecordRef(),
		base.plan.AuthorityResolutionRecordDigest(),
		base.plan.SingleUseKey(),
	)
	if err != nil {
		t.Fatalf("NewProfileDeclarationCommitPlan(v2): %v", err)
	}
	return internalV2AdmissionFixture{
		plan:              plan,
		record:            record,
		description:       description,
		contract:          contract,
		assignment:        assignment,
		assignmentSupport: support,
		basis:             base.basis,
		effect:            effect,
		assessment:        assessment,
		workInputRef:      workInputRef,
	}
}

func internalProfileAuthorSupportV2(
	t *testing.T,
	base internalV1AdmissionFixtureV1,
) (ProfileAuthorRoleAssignmentV1, ProfileAuthorAssignmentSupportCarrierV1) {
	t.Helper()
	oldSystemAdmission := base.assignmentSupport.SystemAdmission()
	systemAdmissionBuilder := NewProfileOnboardingExecutorSystemAdmissionV1Builder(
		oldSystemAdmission.Ref(),
		oldSystemAdmission.SystemRef(),
	)
	systemAdmissionBuilder = systemAdmissionBuilder.IdentifiedBy(oldSystemAdmission.IdentityBasis())
	systemAdmissionBuilder = systemAdmissionBuilder.AdmittedToActBy(
		oldSystemAdmission.ActingEligibilityBasisRef(),
		oldSystemAdmission.ActingEligibilityBasisDigest(),
	)
	systemAdmissionBuilder = systemAdmissionBuilder.InSession(oldSystemAdmission.SessionRef())
	systemAdmissionBuilder = systemAdmissionBuilder.ValidDuring(oldSystemAdmission.ValidityWindow())
	systemAdmissionBuilder = systemAdmissionBuilder.UsingMethodEditionV2()
	systemAdmission, err := systemAdmissionBuilder.Build()
	if err != nil {
		t.Fatalf("ProfileOnboardingExecutorSystemAdmissionV1Builder.Build(v2): %v", err)
	}
	oldRoleAdmission := base.assignmentSupport.RoleAdmission()
	roleAdmission, err := NewProfileAuthorRoleAdmissionV2(oldRoleAdmission.Ref())
	if err != nil {
		t.Fatalf("NewProfileAuthorRoleAdmissionV2: %v", err)
	}
	oldJustification := base.assignmentSupport.Justification()
	justificationBuilder := NewProfileAuthorAssignmentJustificationV1Builder(
		oldJustification.Ref(),
	)
	justificationBuilder = justificationBuilder.ApplyingAdmissions(systemAdmission, roleAdmission)
	justificationBuilder = justificationBuilder.ValidDuring(oldJustification.AssignmentWindow())
	justification, err := justificationBuilder.Build()
	if err != nil {
		t.Fatalf("ProfileAuthorAssignmentJustificationV1Builder.Build(v2): %v", err)
	}
	oldProvenance := base.assignmentSupport.Provenance()
	provenanceBuilder := NewProfileAuthorAssignmentProvenanceV1Builder(
		oldProvenance.Ref(),
		justification,
	)
	provenanceBuilder = provenanceBuilder.InSession(oldProvenance.SessionRef())
	provenanceBuilder = provenanceBuilder.ProducedBy(
		oldProvenance.Kernel(),
		oldProvenance.Runtime(),
	)
	provenanceBuilder = provenanceBuilder.RecordedAt(oldProvenance.RecordedAt())
	provenance, err := provenanceBuilder.Build()
	if err != nil {
		t.Fatalf("ProfileAuthorAssignmentProvenanceV1Builder.Build(v2): %v", err)
	}
	support, err := CarryProfileAuthorAssignmentSupportV1(
		systemAdmission,
		roleAdmission,
		justification,
		provenance,
	)
	if err != nil {
		t.Fatalf("CarryProfileAuthorAssignmentSupportV1(v2): %v", err)
	}
	oldAssignment := base.assignment
	assignmentBuilder := NewProfileAuthorRoleAssignmentV1Builder(oldAssignment.RoleAssignmentRef())
	assignmentBuilder = assignmentBuilder.HeldBy(oldAssignment.HolderSystemRef())
	assignmentBuilder = assignmentBuilder.Assigning(oldAssignment.AdmittedRoleRef())
	assignmentBuilder = assignmentBuilder.InContext(oldAssignment.BoundedContextRef())
	assignmentBuilder = assignmentBuilder.ValidDuring(oldAssignment.ValidityWindow())
	assignmentBuilder = assignmentBuilder.WithSystemAdmission(
		systemAdmission.Ref(),
		support.SystemAdmissionDigest(),
	)
	assignmentBuilder = assignmentBuilder.WithRoleAdmission(
		roleAdmission.Ref(),
		support.RoleAdmissionDigest(),
	)
	assignmentBuilder = assignmentBuilder.JustifiedBy(
		justification.Ref(),
		support.JustificationDigest(),
	)
	assignmentBuilder = assignmentBuilder.WithProvenance(
		provenance.Ref(),
		support.ProvenanceDigest(),
	)
	assignment, err := assignmentBuilder.Build()
	if err != nil {
		t.Fatalf("ProfileAuthorRoleAssignmentV1Builder.Build(v2): %v", err)
	}
	if err := ValidateProfileAuthorRoleAssignmentV1Support(assignment, support); err != nil {
		t.Fatalf("ValidateProfileAuthorRoleAssignmentV1Support(v2): %v", err)
	}
	return assignment, support
}

func internalProfileOnboardingEffectForWorkV2(
	t *testing.T,
	base ProfileOnboardingEffectV1,
	record ProfileOnboardingWorkRecord,
	workDigest ContentDigest,
) ProfileOnboardingEffectV1 {
	t.Helper()
	effect, err := NewProfileOnboardingEffectV1(
		base.Ref(),
		record.RecordRef(),
		record.WorkRef(),
		workDigest,
		base.Result(),
		base.AffectedEntityRefs(),
		base.StatePlaneRef(),
		base.StateWitness(),
		base.EvidencePathRefs(),
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingEffectV1(v2 Work): %v", err)
	}
	return effect
}

func internalProfileOnboardingAssessmentForEffectV2(
	t *testing.T,
	base ProfileOnboardingOutcomeAssessmentV1,
	effect ProfileOnboardingEffectV1,
) ProfileOnboardingOutcomeAssessmentV1 {
	t.Helper()
	assessment, err := NewProfileOnboardingOutcomeAssessmentV1(
		base.Ref(),
		effect,
		base.AcceptanceStandardRef(),
		base.AcceptanceStandardEdition(),
		base.ComparatorRef(),
		base.ComparatorEdition(),
		base.Verdict(),
		base.EvidencePathRefs(),
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingOutcomeAssessmentV1(v2): %v", err)
	}
	return assessment
}
