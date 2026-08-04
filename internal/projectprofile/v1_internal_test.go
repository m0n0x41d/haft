package projectprofile

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

type structurallyUnknownScopeV1 struct {
	scopeID ScopeID
}

func (structurallyUnknownScopeV1) realizationScopeVariant() {}

func (scope structurallyUnknownScopeV1) ScopeID() ScopeID {
	return scope.scopeID
}

func TestFinalV1PayloadStructuralConsistencyHasExactBoundedPredicate(t *testing.T) {
	sharedEntity, err := NewEntityRef("entity:shared")
	if err != nil {
		t.Fatalf("NewEntityRef: %v", err)
	}
	leftID, err := NewScopeID("scope-left")
	if err != nil {
		t.Fatalf("NewScopeID(left): %v", err)
	}
	rightID, err := NewScopeID("scope-right")
	if err != nil {
		t.Fatalf("NewScopeID(right): %v", err)
	}
	left, err := NewSoftwareRealization(leftID, NewReferencedEntity(sharedEntity))
	if err != nil {
		t.Fatalf("NewSoftwareRealization(left): %v", err)
	}
	right, err := NewSoftwareRealization(rightID, NewReferencedEntity(sharedEntity))
	if err != nil {
		t.Fatalf("NewSoftwareRealization(right): %v", err)
	}
	contextualScopes, err := NewScopeSet([]RealizationScope{left, right})
	if err != nil {
		t.Fatalf("NewScopeSet(contextual): %v", err)
	}
	payload, err := NewProfileDeclarationPayload(contextualScopes)
	if err != nil {
		t.Fatalf("same EntityRef in distinct contextual scopes was rejected: %v", err)
	}
	err = ValidateProfileDeclarationPayloadStructuralConsistencyV1(payload)
	if err != nil {
		t.Fatalf("ValidateProfileDeclarationPayloadStructuralConsistencyV1: %v", err)
	}

	duplicateScopePayload := ProfileDeclarationPayload{
		scopes: ScopeSet{values: []RealizationScope{left, left}},
	}
	err = ValidateProfileDeclarationPayloadStructuralConsistencyV1(duplicateScopePayload)
	if err == nil {
		t.Fatal("structural consistency accepted duplicate ScopeID")
	}

	unknownVariantPayload := ProfileDeclarationPayload{
		scopes: ScopeSet{values: []RealizationScope{structurallyUnknownScopeV1{scopeID: leftID}}},
	}
	err = ValidateProfileDeclarationPayloadStructuralConsistencyV1(unknownVariantPayload)
	if err == nil {
		t.Fatal("structural consistency accepted an unknown realization variant")
	}

	invalidBindingPayload := ProfileDeclarationPayload{
		scopes: ScopeSet{values: []RealizationScope{SoftwareRealization{
			scopeID:   leftID,
			entityRef: ReferencedEntity{},
		}}},
	}
	err = ValidateProfileDeclarationPayloadStructuralConsistencyV1(invalidBindingPayload)
	if err == nil {
		t.Fatal("structural consistency accepted an invalid optional entity binding")
	}

	patternRef, err := NewSourceUnitRef("fpf:E.11.PUA")
	if err != nil {
		t.Fatalf("NewSourceUnitRef: %v", err)
	}
	duplicateReferencePayload := ProfileDeclarationPayload{
		scopes: ScopeSet{values: []RealizationScope{NonSoftwareRealization{
			scopeID:              rightID,
			entityRef:            NoEntityReference{},
			kindOrientation:      UnspecifiedKindOrientation{},
			governingPatternRefs: []SourceUnitRef{patternRef, patternRef},
			contractRefs:         []SpecSectionRef{},
		}}},
	}
	err = ValidateProfileDeclarationPayloadStructuralConsistencyV1(duplicateReferencePayload)
	if err == nil {
		t.Fatal("structural consistency accepted duplicate canonical refs")
	}
}

func TestFinalV1PipelineAdjacentVisitHandlesEmptyAndSingletonCollections(t *testing.T) {
	emptyVisits := 0
	err := visitAdjacentV1([]string{}, func(_ string, _ string) error {
		emptyVisits++
		return nil
	})
	if err != nil || emptyVisits != 0 {
		t.Fatalf("empty adjacent visit = (%d, %v), want (0, nil)", emptyVisits, err)
	}

	singletonVisits := 0
	err = visitAdjacentV1([]string{"only"}, func(_ string, _ string) error {
		singletonVisits++
		return nil
	})
	if err != nil || singletonVisits != 0 {
		t.Fatalf("singleton adjacent visit = (%d, %v), want (0, nil)", singletonVisits, err)
	}
}

func TestFinalV1AdmissionRecordRejectsEveryCrossBindingMismatch(t *testing.T) {
	prepared := internalPreparedProfileAdmissionV1(t)
	material := internalTentativeProfileAdmissionV1(t, prepared)
	decoded, err := decodeProfileDeclarationAdmissionRecordCanonicalJSON(
		material.admissionRecordCanonicalJSON,
	)
	if err != nil {
		t.Fatalf("decodeProfileDeclarationAdmissionRecordCanonicalJSON: %v", err)
	}
	original := decoded.(profileDeclarationAdmissionRecord)
	originalReceipt := original.receipt.(profileDeclarationReceiptV1)
	recordedAt := material.recordedAt

	assertRejected := func(name string, value profileDeclarationAdmissionRecord) {
		t.Helper()
		validationErr := validateProfileDeclarationAdmissionRecord(value)
		if validationErr == nil {
			t.Fatalf("admission record accepted %s mismatch", name)
		}
	}

	wrongWork := original
	wrongWork.classificationWorkRecordRef = ProfileOnboardingWorkRecordRef{
		v1Reference: v1Reference{value: "work-record:other"},
	}
	assertRejected("Work-record", wrongWork)

	wrongBasis := original
	wrongBasis.authorityBasisRef = ProfileDeclarationAuthorityBasisRef{
		v1Reference: v1Reference{value: "authority-basis:other"},
	}
	assertRejected("authority-basis", wrongBasis)

	wrongResolution := original
	wrongResolution.authorityResolutionRecordDigest = internalV1Digest("3")
	assertRejected("authority-resolution", wrongResolution)

	wrongReceiptPayload := originalReceipt
	wrongReceiptPayload.payloadDigest = internalV1Digest("4")
	wrongReceipt := original
	wrongReceipt.receipt = wrongReceiptPayload
	assertRejected("receipt payload", wrongReceipt)

	wrongReceiptProvenance := originalReceipt
	wrongReceiptProvenance.candidateProvenanceDigest = internalV1Digest("5")
	wrongReceipt = original
	wrongReceipt.receipt = wrongReceiptProvenance
	assertRejected("receipt provenance", wrongReceipt)

	wrongReceiptObservedBasis := originalReceipt
	wrongReceiptObservedBasis.observedBasisDigest = internalV1Digest("6")
	wrongReceipt = original
	wrongReceipt.receipt = wrongReceiptObservedBasis
	assertRejected("receipt observed basis", wrongReceipt)

	wrongReceiptRevision := originalReceipt
	wrongReceiptRevision.ledgerRevision = NewLedgerRevision(2)
	wrongReceipt = original
	wrongReceipt.receipt = wrongReceiptRevision
	assertRejected("receipt revision", wrongReceipt)

	wrongReceiptTime := originalReceipt
	wrongReceiptTime.recordedAt = recordedAt.Add(time.Second)
	wrongReceipt = original
	wrongReceipt.receipt = wrongReceiptTime
	assertRejected("receipt time", wrongReceipt)

	recordJSON, err := encodeProfileDeclarationAdmissionRecordCanonicalJSON(original)
	if err != nil {
		t.Fatalf("encodeProfileDeclarationAdmissionRecordCanonicalJSON: %v", err)
	}
	oldDigest := fmt.Sprintf(`"candidate_provenance_digest":"%s"`, originalReceipt.candidateProvenanceDigest.String())
	newDigest := fmt.Sprintf(`"candidate_provenance_digest":"%s"`, internalV1Digest("7").String())
	tamperedJSON := bytes.Replace(recordJSON, []byte(oldDigest), []byte(newDigest), 1)
	if bytes.Equal(recordJSON, tamperedJSON) {
		t.Fatal("test fixture did not alter receipt provenance digest")
	}
	_, err = decodeProfileDeclarationAdmissionRecordCanonicalJSON(tamperedJSON)
	if err == nil {
		t.Fatal("admission-record decoder accepted receipt/provenance mismatch")
	}
}

type internalV1AdmissionFixtureV1 struct {
	plan              ProfileDeclarationCommitPlan
	record            ProfileOnboardingWorkRecord
	description       ProfileOnboardingMethodDescriptionV1
	contract          ProfileOnboardingMethodContractV1
	assignment        ProfileAuthorRoleAssignmentV1
	assignmentSupport ProfileAuthorAssignmentSupportCarrierV1
	basis             ObservedProjectBasisV1
	effect            ProfileOnboardingEffectV1
	assessment        ProfileOnboardingOutcomeAssessmentV1
}

func internalV1CommitPlan(t *testing.T) ProfileDeclarationCommitPlan {
	t.Helper()
	return internalV1AdmissionFixture(t).plan
}

func internalV1AdmissionFixture(t *testing.T) internalV1AdmissionFixtureV1 {
	t.Helper()
	scopeID, err := NewScopeID("software-cli")
	if err != nil {
		t.Fatalf("NewScopeID: %v", err)
	}
	scope, err := NewSoftwareRealization(scopeID, NoEntityReference{})
	if err != nil {
		t.Fatalf("NewSoftwareRealization: %v", err)
	}
	scopes, err := NewScopeSet([]RealizationScope{scope})
	if err != nil {
		t.Fatalf("NewScopeSet: %v", err)
	}
	payload, err := NewProfileDeclarationPayload(scopes)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload: %v", err)
	}
	payloadDigest, err := DigestProfileDeclarationPayload(payload)
	if err != nil {
		t.Fatalf("DigestProfileDeclarationPayload: %v", err)
	}
	workSupport := exactProfileOnboardingWorkSupportFixtureV1(t)
	basisDigest, err := DigestObservedProjectBasisV1(workSupport.basis)
	if err != nil {
		t.Fatalf("DigestObservedProjectBasisV1: %v", err)
	}
	outcome, err := NewCandidatePayloadProduced(payloadDigest, basisDigest)
	if err != nil {
		t.Fatalf("NewCandidatePayloadProduced: %v", err)
	}
	workRecord := internalV1WorkRecordWithOutcome(t, workSupport.record, outcome)
	workRecordDigest, err := DigestProfileOnboardingWorkRecord(workRecord)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingWorkRecord: %v", err)
	}
	assignmentDigest, err := DigestProfileAuthorRoleAssignmentV1(workSupport.assignment)
	if err != nil {
		t.Fatalf("DigestProfileAuthorRoleAssignmentV1: %v", err)
	}
	affectedEntityRef, err := NewEntityRef(workRecord.affectedRefs[0].String())
	if err != nil {
		t.Fatalf("NewEntityRef: %v", err)
	}
	pbo := pboFixtureV1{
		basis:             workSupport.basis,
		basisDigest:       basisDigest,
		work:              workRecord,
		workDigest:        workRecordDigest,
		payloadDigest:     payloadDigest,
		outputRef:         workRecord.outputRefs[0],
		statePlaneRef:     workRecord.statePlaneRef,
		stateWitness:      workRecord.stateTransition,
		affectedEntityRef: affectedEntityRef,
	}
	effect := pboEffect(t, pbo, "evidence:path:effect:candidate")
	assessment := pboAssessment(t, effect, ProfileOnboardingAcceptancePassedV1Value())
	assessmentDigest, err := DigestProfileOnboardingOutcomeAssessmentV1(assessment)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingOutcomeAssessmentV1: %v", err)
	}
	classifierVersion, err := NewClassifierVersion("h-onboard/v9")
	if err != nil {
		t.Fatalf("NewClassifierVersion: %v", err)
	}
	policyVersion, err := NewPolicyVersion("profile-policy/v1")
	if err != nil {
		t.Fatalf("NewPolicyVersion: %v", err)
	}
	provenanceBuilder := NewCandidateProvenanceV1Builder(
		ProfileDeclarationAuthorityBasisRef{v1Reference: v1Reference{value: "authority-basis:1"}},
		workRecord.recordRef,
		workRecordDigest,
	)
	provenanceBuilder = provenanceBuilder.ForProfileAuthorRoleAssignment(
		workSupport.assignment.RoleAssignmentRef(),
		assignmentDigest,
	)
	provenanceBuilder = provenanceBuilder.ForProject(workSupport.basis.ProjectRoot())
	provenanceBuilder = provenanceBuilder.ClassifiedBy(classifierVersion, policyVersion)
	provenanceBuilder = provenanceBuilder.InSession(workSupport.assignmentSupport.Provenance().SessionRef())
	provenanceBuilder = provenanceBuilder.ForPayload(payloadDigest)
	provenanceBuilder = provenanceBuilder.ForObservedProjectBasis(workSupport.basis.Ref(), basisDigest)
	provenanceBuilder = provenanceBuilder.ForOutcomeAssessment(assessment.Ref(), assessmentDigest)
	provenance, err := provenanceBuilder.Build()
	if err != nil {
		t.Fatalf("CandidateProvenanceV1Builder.Build: %v", err)
	}
	candidate, err := NewProfileDeclarationCandidateV1(payload, provenance)
	if err != nil {
		t.Fatalf("NewProfileDeclarationCandidateV1: %v", err)
	}
	inputs, err := NewProfileDeclarationAdmissionInputs(candidate, NewLedgerRevision(0))
	if err != nil {
		t.Fatalf("NewProfileDeclarationAdmissionInputs: %v", err)
	}
	plan, err := NewProfileDeclarationCommitPlan(
		inputs,
		AuthorityResolutionRecordRef{v1Reference: v1Reference{value: "authority-resolution:1"}},
		internalV1Digest("2"),
		SingleUseKey{v1Reference: v1Reference{value: "single-use:1"}},
	)
	if err != nil {
		t.Fatalf("NewProfileDeclarationCommitPlan: %v", err)
	}
	return internalV1AdmissionFixtureV1{
		plan:              plan,
		record:            workRecord,
		description:       workSupport.description,
		contract:          workSupport.contract,
		assignment:        workSupport.assignment,
		assignmentSupport: workSupport.assignmentSupport,
		basis:             workSupport.basis,
		effect:            effect,
		assessment:        assessment,
	}
}

func internalV1WorkRecord(
	t *testing.T,
	payloadDigest ContentDigest,
	observedBasisDigest ContentDigest,
) ProfileOnboardingWorkRecord {
	t.Helper()
	description := ProfileOnboardingMethodDescriptionV1Value()
	descriptionDigest, err := DigestProfileOnboardingMethodDescriptionV1(description)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingMethodDescriptionV1: %v", err)
	}
	contract, err := ProfileOnboardingMethodContractV1Value()
	if err != nil {
		t.Fatalf("ProfileOnboardingMethodContractV1Value: %v", err)
	}
	contractDigest, err := DigestProfileOnboardingMethodContractV1(contract)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingMethodContractV1: %v", err)
	}
	rootBinding, err := NewMethodParameterBinding("project_root", "/tmp/haft-project")
	if err != nil {
		t.Fatalf("NewMethodParameterBinding(project_root): %v", err)
	}
	classifierBinding, err := NewMethodParameterBinding("classifier_version", "h-onboard/v9")
	if err != nil {
		t.Fatalf("NewMethodParameterBinding(classifier_version): %v", err)
	}
	policyBinding, err := NewMethodParameterBinding("policy_version", "profile-policy/v1")
	if err != nil {
		t.Fatalf("NewMethodParameterBinding(policy_version): %v", err)
	}
	sessionBinding, err := NewMethodParameterBinding("session_ref", "session:1")
	if err != nil {
		t.Fatalf("NewMethodParameterBinding(session_ref): %v", err)
	}
	bindings, err := NewMethodParameterBindings([]MethodParameterBinding{
		rootBinding,
		classifierBinding,
		policyBinding,
		sessionBinding,
	})
	if err != nil {
		t.Fatalf("NewMethodParameterBindings: %v", err)
	}
	from := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	workUntil := from.Add(time.Hour)
	workInterval, err := NewWorkIntervalV1(from, workUntil)
	if err != nil {
		t.Fatalf("NewWorkIntervalV1: %v", err)
	}
	basisFrom := from.Add(-time.Hour)
	basisUntil := from.Add(30 * time.Minute)
	basisWindow, err := NewBasisObservationWindowV1(basisFrom, basisUntil)
	if err != nil {
		t.Fatalf("NewBasisObservationWindowV1: %v", err)
	}
	transition, err := NewPrePostStateTransitionV1(
		StateRef{v1Reference: v1Reference{value: "state:auto"}},
		StateRef{v1Reference: v1Reference{value: "state:candidate"}},
	)
	if err != nil {
		t.Fatalf("NewPrePostStateTransitionV1: %v", err)
	}
	outcome, err := NewCandidatePayloadProduced(payloadDigest, observedBasisDigest)
	if err != nil {
		t.Fatalf("NewCandidatePayloadProduced: %v", err)
	}
	builder := NewProfileOnboardingWorkRecordBuilder(
		ProfileOnboardingWorkRecordRef{v1Reference: v1Reference{value: "work-record:1"}},
		WorkRef{v1Reference: v1Reference{value: "work:1"}},
	)
	methodRef := ProfileOnboardingMethodRefV1()
	methodDescriptionRef := ProfileOnboardingMethodDescriptionRefV1()
	builder = builder.Enacts(
		methodRef,
		methodDescriptionRef,
		bindings,
	)
	builder = builder.WithMethodDescriptionDigest(descriptionDigest)
	contractRef := contract.Ref()
	builder = builder.GovernedByMethodContract(contractRef, contractDigest)
	performedBy := RoleAssignmentRef{v1Reference: v1Reference{value: "role-assignment:1"}}
	builder = builder.PerformedBy(performedBy)
	assignmentDigest := internalV1Digest("assignment")
	builder = builder.WithProfileAuthorRoleAssignment(performedBy, assignmentDigest)
	executedWithin := SystemRef{v1Reference: v1Reference{value: "system:host-agent:1"}}
	builder = builder.ExecutedWithin(executedWithin)
	contextRef := ProfileOnboardingBoundedContextRefV1()
	builder = builder.InContext(contextRef)
	builder = builder.During(workInterval, basisWindow)
	basisRef := ObservedProjectBasisRefV1{v1Reference: v1Reference{value: "observed-project-basis:1"}}
	builder = builder.WithObservedProjectBasis(basisRef, observedBasisDigest)
	basisRefValue := basisRef.String()
	builder = builder.WithInputs([]WorkInputRef{{v1Reference: v1Reference{value: basisRefValue}}})
	builder = builder.WithOutputs([]WorkOutputRef{{v1Reference: v1Reference{value: "output:payload:1"}}})
	builder = builder.WithResources([]WorkResourceRef{{v1Reference: v1Reference{value: "resource:session:1"}}})
	affectedRefKind := description.AffectedRefKind()
	builder = builder.AffectingKind(affectedRefKind)
	builder = builder.Affecting([]AffectedReferentRef{{v1Reference: v1Reference{value: "episteme:classification:1"}}})
	statePlaneRef := description.StatePlaneRef()
	builder = builder.OnStatePlane(statePlaneRef, transition)
	builder = builder.WithOutcome(outcome)
	record, err := builder.Build()
	if err != nil {
		t.Fatalf("ProfileOnboardingWorkRecordBuilder.Build: %v", err)
	}
	return record
}

func internalV1WorkRecordWithOutcome(
	t *testing.T,
	record ProfileOnboardingWorkRecord,
	outcome ProfileOnboardingWorkOutcomeV1,
) ProfileOnboardingWorkRecord {
	t.Helper()
	builder := NewProfileOnboardingWorkRecordBuilder(record.recordRef, record.workRef)
	builder = builder.Enacts(record.enactsMethodRef, record.methodDescriptionRef, record.parameterBindings)
	builder = builder.WithMethodDescriptionDigest(record.methodDescriptionDigest)
	builder = builder.GovernedByMethodContract(record.methodContractRef, record.methodContractDigest)
	builder = builder.PerformedUnderAssignment(record.coveringRoleAssignment)
	builder = builder.WithProfileAuthorRoleAssignment(
		record.profileAuthorRoleAssignmentRef,
		record.profileAuthorRoleAssignmentDigest,
	)
	builder = builder.ActualPerformer(record.actualPerformerSystem)
	builder = builder.InContext(record.boundedContextRef)
	builder = builder.During(record.workInterval, record.basisObservationWindow)
	builder = builder.WithObservedProjectBasis(
		record.observedProjectBasisRef,
		record.observedProjectBasisDigest,
	)
	builder = builder.WithInputs(record.inputRefs)
	builder = builder.WithOutputs(record.outputRefs)
	builder = builder.WithResources(record.resourceRefs)
	builder = builder.AffectingKind(record.affectedRefKind)
	builder = builder.Affecting(record.affectedRefs)
	builder = builder.OnStatePlane(record.statePlaneRef, record.stateTransition)
	builder = builder.WithOutcome(outcome)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("ProfileOnboardingWorkRecordBuilder.Build: %v", err)
	}
	return result
}

func internalV1Digest(seed string) ContentDigest {
	value := strings.Repeat(seed, 64)
	return ContentDigest{value: "sha256:" + value[:64]}
}
