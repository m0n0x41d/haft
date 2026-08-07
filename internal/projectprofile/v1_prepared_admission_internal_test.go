package projectprofile

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPreparedProfileAdmissionV1CanonicalMaterialIsExactlyDerived(t *testing.T) {
	prepared := internalPreparedProfileAdmissionV1(t)
	builder := NewProfileAdmissionPreparationV1Builder(prepared.plan, prepared.projectRoot)
	var err error
	builder, err = withPreparedProfileAdmissionMethodEdition(
		builder,
		prepared.workRecord,
		prepared.methodDescription,
		prepared.methodContract,
	)
	if err != nil {
		t.Fatalf("withPreparedProfileAdmissionMethodEdition: %v", err)
	}
	builder = builder.WithProfileAuthor(prepared.profileAuthorRoleAssignment, prepared.profileAuthorAssignmentSupport)
	builder = builder.WithObservedOutcome(prepared.observedProjectBasis, prepared.onboardingEffect, prepared.outcomeAssessment)
	exactInput, err := exactProfileAdmissionPreparationInputV1From(builder.input)
	if err != nil {
		t.Fatalf("exactProfileAdmissionPreparationInputV1From: %v", err)
	}
	requestJSON, err := encodeProfileDeclarationAdmissionRequestV1CanonicalJSON(exactInput)
	if err != nil {
		t.Fatalf("encodeProfileDeclarationAdmissionRequestV1CanonicalJSON: %v", err)
	}
	if !bytes.Equal(requestJSON, prepared.admissionRequestCanonicalJSON) {
		t.Fatal("prepared request differs from typed canonical request")
	}
	if digestProfileDeclarationAdmissionRequestV1(requestJSON) != prepared.admissionRequestDigest {
		t.Fatal("prepared request digest differs from canonical request bytes")
	}
	assignmentJSON, err := EncodeProfileAuthorRoleAssignmentV1CanonicalJSON(prepared.profileAuthorRoleAssignment)
	if err != nil {
		t.Fatalf("EncodeProfileAuthorRoleAssignmentV1CanonicalJSON: %v", err)
	}
	if !bytes.Equal(assignmentJSON, prepared.profileAuthorRoleAssignmentCanonicalJSON) {
		t.Fatal("prepared assignment differs from typed canonical assignment")
	}
	if !bytes.Equal(
		prepared.profileAuthorAssignmentSupport.SystemAdmissionCanonicalJSON(),
		prepared.ExecutorSystemAdmissionCanonicalJSON(),
	) {
		t.Fatal("prepared executor-system admission differs from exact support carrier")
	}
	if err := ValidatePreparedProfileAdmissionV1(prepared); err != nil {
		t.Fatalf("ValidatePreparedProfileAdmissionV1: %v", err)
	}
}

func TestPreparedProfileAdmissionV1ValidationRecomputesStoredMaterial(t *testing.T) {
	prepared := internalPreparedProfileAdmissionV1(t)
	tamperers := []struct {
		name   string
		tamper func(preparedProfileAdmissionV1) preparedProfileAdmissionV1
	}{
		{name: "request JSON", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.admissionRequestCanonicalJSON = tamperedPreparedBytesV1(value.admissionRequestCanonicalJSON)
			return value
		}},
		{name: "request digest", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.admissionRequestDigest = internalV1Digest("3")
			return value
		}},
		{name: "payload JSON", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.profilePayloadCanonicalJSON = tamperedPreparedBytesV1(value.profilePayloadCanonicalJSON)
			return value
		}},
		{name: "payload digest", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.profilePayloadDigest = internalV1Digest("4")
			return value
		}},
		{name: "provenance JSON", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.candidateProvenanceJSON = tamperedPreparedBytesV1(value.candidateProvenanceJSON)
			return value
		}},
		{name: "provenance digest", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.candidateProvenanceDigest = internalV1Digest("5")
			return value
		}},
		{name: "Work JSON", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.workRecordCanonicalJSON = tamperedPreparedBytesV1(value.workRecordCanonicalJSON)
			return value
		}},
		{name: "Work digest", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.workRecordDigest = internalV1Digest("6")
			return value
		}},
		{name: "MethodDescription JSON", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.methodDescriptionCanonicalJSON = tamperedPreparedBytesV1(value.methodDescriptionCanonicalJSON)
			return value
		}},
		{name: "MethodDescription digest", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.methodDescriptionDigest = internalV1Digest("7")
			return value
		}},
		{name: "MethodContract JSON", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.methodContractCanonicalJSON = tamperedPreparedBytesV1(value.methodContractCanonicalJSON)
			return value
		}},
		{name: "MethodContract digest", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.methodContractDigest = internalV1Digest("8")
			return value
		}},
		{name: "assignment JSON", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.profileAuthorRoleAssignmentCanonicalJSON = tamperedPreparedBytesV1(value.profileAuthorRoleAssignmentCanonicalJSON)
			return value
		}},
		{name: "assignment digest", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.profileAuthorRoleAssignmentDigest = internalV1Digest("9")
			return value
		}},
		{name: "system-admission JSON", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.profileAuthorAssignmentSupport.systemAdmissionJSON = tamperedPreparedBytesV1(value.profileAuthorAssignmentSupport.systemAdmissionJSON)
			return value
		}},
		{name: "system-admission digest", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.profileAuthorAssignmentSupport.systemAdmissionDigest = internalV1Digest("a")
			return value
		}},
		{name: "role-admission JSON", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.profileAuthorAssignmentSupport.roleAdmissionJSON = tamperedPreparedBytesV1(value.profileAuthorAssignmentSupport.roleAdmissionJSON)
			return value
		}},
		{name: "role-admission digest", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.profileAuthorAssignmentSupport.roleAdmissionDigest = internalV1Digest("b")
			return value
		}},
		{name: "justification JSON", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.profileAuthorAssignmentSupport.justificationJSON = tamperedPreparedBytesV1(value.profileAuthorAssignmentSupport.justificationJSON)
			return value
		}},
		{name: "justification digest", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.profileAuthorAssignmentSupport.justificationDigest = internalV1Digest("c")
			return value
		}},
		{name: "assignment-provenance JSON", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.profileAuthorAssignmentSupport.provenanceJSON = tamperedPreparedBytesV1(value.profileAuthorAssignmentSupport.provenanceJSON)
			return value
		}},
		{name: "assignment-provenance digest", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.profileAuthorAssignmentSupport.provenanceDigest = internalV1Digest("d")
			return value
		}},
		{name: "basis JSON", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.observedProjectBasisCanonicalJSON = tamperedPreparedBytesV1(value.observedProjectBasisCanonicalJSON)
			return value
		}},
		{name: "basis digest", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.observedProjectBasisDigest = internalV1Digest("e")
			return value
		}},
		{name: "effect JSON", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.onboardingEffectCanonicalJSON = tamperedPreparedBytesV1(value.onboardingEffectCanonicalJSON)
			return value
		}},
		{name: "effect digest", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.onboardingEffectDigest = internalV1Digest("f")
			return value
		}},
		{name: "assessment JSON", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.outcomeAssessmentCanonicalJSON = tamperedPreparedBytesV1(value.outcomeAssessmentCanonicalJSON)
			return value
		}},
		{name: "assessment digest", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.outcomeAssessmentDigest = internalV1Digest("0")
			return value
		}},
		{name: "commit plan", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.plan.digest = internalV1Digest("1")
			return value
		}},
		{name: "project root", tamper: func(value preparedProfileAdmissionV1) preparedProfileAdmissionV1 {
			value.projectRoot = ProjectRootV1{value: "/tmp/other-project"}
			return value
		}},
	}
	for _, test := range tamperers {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidatePreparedProfileAdmissionV1(test.tamper(prepared)); err == nil {
				t.Fatalf("validation accepted tampered %s", test.name)
			}
		})
	}
}

func TestTentativeProfileAdmissionValidationRecomputesFinalMaterial(t *testing.T) {
	prepared := internalPreparedProfileAdmissionV1(t)
	material := internalTentativeProfileAdmissionV1(t, prepared)
	tamperers := []struct {
		name   string
		tamper func(tentativeProfileAdmissionTransactionMaterialV1) tentativeProfileAdmissionTransactionMaterialV1
	}{
		{name: "receipt JSON", tamper: func(value tentativeProfileAdmissionTransactionMaterialV1) tentativeProfileAdmissionTransactionMaterialV1 {
			value.receiptCanonicalJSON = tamperedPreparedBytesV1(value.receiptCanonicalJSON)
			return value
		}},
		{name: "receipt digest", tamper: func(value tentativeProfileAdmissionTransactionMaterialV1) tentativeProfileAdmissionTransactionMaterialV1 {
			value.receiptDigest = internalV1Digest("9")
			return value
		}},
		{name: "admission JSON", tamper: func(value tentativeProfileAdmissionTransactionMaterialV1) tentativeProfileAdmissionTransactionMaterialV1 {
			value.admissionRecordCanonicalJSON = tamperedPreparedBytesV1(value.admissionRecordCanonicalJSON)
			return value
		}},
		{name: "admission digest", tamper: func(value tentativeProfileAdmissionTransactionMaterialV1) tentativeProfileAdmissionTransactionMaterialV1 {
			value.admissionRecordDigest = internalV1Digest("a")
			return value
		}},
		{name: "tentative admission ref", tamper: func(value tentativeProfileAdmissionTransactionMaterialV1) tentativeProfileAdmissionTransactionMaterialV1 {
			value.admissionRecordRef = ProfileDeclarationAdmissionRecordRef{
				v1Reference: v1Reference{value: "profile-admission:tampered"},
			}
			return value
		}},
		{name: "tentative revision", tamper: func(value tentativeProfileAdmissionTransactionMaterialV1) tentativeProfileAdmissionTransactionMaterialV1 {
			value.committedLedgerRevision = value.prepared.ExpectedLedgerRevision()
			return value
		}},
		{name: "tentative time", tamper: func(value tentativeProfileAdmissionTransactionMaterialV1) tentativeProfileAdmissionTransactionMaterialV1 {
			value.recordedAt = value.recordedAt.Add(time.Second)
			return value
		}},
	}
	for _, test := range tamperers {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateTentativeProfileAdmissionTransactionMaterialV1(test.tamper(material)); err == nil {
				t.Fatalf("validation accepted tampered %s", test.name)
			}
		})
	}
}

func TestTentativeProfileAdmissionRepresentationContainsNoFinalSemanticVariant(t *testing.T) {
	typeOfMaterial := reflect.TypeOf(tentativeProfileAdmissionTransactionMaterialV1{})
	forbiddenTypeFragments := []string{
		"declaredProjectProfileV1",
		"profileDeclarationReceiptV1",
		"profileDeclarationAdmissionRecord",
	}
	for fieldIndex := range typeOfMaterial.NumField() {
		field := typeOfMaterial.Field(fieldIndex)
		fieldType := field.Type.String()
		for _, forbidden := range forbiddenTypeFragments {
			if strings.Contains(fieldType, forbidden) {
				t.Fatalf("tentative material field %s retains final semantic type %s", field.Name, fieldType)
			}
		}
	}
}

func TestTentativeProfileAdmissionRejectsObservationWindowEndingAfterTransaction(t *testing.T) {
	workFrom := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	workUntil := workFrom.Add(time.Hour)
	workInterval, err := NewWorkIntervalV1(workFrom, workUntil)
	if err != nil {
		t.Fatalf("NewWorkIntervalV1: %v", err)
	}
	basisWindow, err := NewBasisObservationWindowV1(
		workFrom.Add(-time.Hour),
		workUntil.Add(time.Hour),
	)
	if err != nil {
		t.Fatalf("NewBasisObservationWindowV1: %v", err)
	}
	record := ProfileOnboardingWorkRecord{
		workInterval:           workInterval,
		basisObservationWindow: basisWindow,
	}
	err = validateAdmissionRecordingTimeV1(workUntil.Add(time.Minute), record)
	if err == nil || !strings.Contains(err.Error(), "basis-observation window") {
		t.Fatalf("recording-time validation accepted incomplete basis window: %v", err)
	}
}

func internalPreparedProfileAdmissionV1(t *testing.T) preparedProfileAdmissionV1 {
	t.Helper()
	fixture := internalV1AdmissionFixture(t)
	projectRoot := fixture.plan.inputs.candidate.provenance.projectRoot
	builder := NewProfileAdmissionPreparationV1Builder(fixture.plan, projectRoot)
	builder = builder.WithWork(fixture.record, fixture.description, fixture.contract)
	builder = builder.WithProfileAuthor(fixture.assignment, fixture.assignmentSupport)
	builder = builder.WithObservedOutcome(fixture.basis, fixture.effect, fixture.assessment)
	prepared, err := buildPreparedProfileAdmissionV1(builder.input)
	if err != nil {
		t.Fatalf("buildPreparedProfileAdmissionV1: %v", err)
	}
	return prepared
}

func internalTentativeProfileAdmissionV1(
	t *testing.T,
	prepared preparedProfileAdmissionV1,
) tentativeProfileAdmissionTransactionMaterialV1 {
	t.Helper()
	admissionRef, err := NewProfileDeclarationAdmissionRecordRef("profile-admission:1")
	if err != nil {
		t.Fatalf("NewProfileDeclarationAdmissionRecordRef: %v", err)
	}
	committed, err := prepared.ExpectedLedgerRevision().Next()
	if err != nil {
		t.Fatalf("LedgerRevision.Next: %v", err)
	}
	material, err := buildTentativeProfileAdmissionTransactionMaterialV1(
		prepared,
		committed,
		prepared.workRecord.workInterval.until.Add(time.Hour),
		admissionRef,
	)
	if err != nil {
		t.Fatalf("buildTentativeProfileAdmissionTransactionMaterialV1: %v", err)
	}
	return material
}

func tamperedPreparedBytesV1(value []byte) []byte {
	result := append([]byte{}, value...)
	result[0] ^= 1
	return result
}
