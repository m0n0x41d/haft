package typedmemoryvalidation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestValidOutcomeRetainsExactCandidateBatchAndBasis(t *testing.T) {
	environment := testEnvironment(t, "6")
	snapshot := newTestSnapshot(t, environment.Ref(), 23, entityAbsent)
	resolved, err := NewResolvedProjectBasis(
		environment,
		typedmemory.NewCodecRegistry(),
		snapshot,
	)
	if err != nil {
		t.Fatalf("NewResolvedProjectBasis() error = %v", err)
	}
	resolver := &fixedResolver{resolution: resolved}
	service := mustService(t, resolver)
	request := decodeRequest(t, `{"kind":"project_current"}`)
	expectedCandidate, err := request.BindChangeSet(environment.Ref())
	if err != nil {
		t.Fatalf("BindChangeSet() error = %v", err)
	}
	expectedCandidateBytes, err := expectedCandidate.CanonicalBytes()
	if err != nil {
		t.Fatalf("candidate CanonicalBytes() error = %v", err)
	}
	expectedCandidateDigest, err := expectedCandidate.Digest()
	if err != nil {
		t.Fatalf("candidate Digest() error = %v", err)
	}

	outcome := service.Evaluate(request)
	valid, isValid := outcome.(ValidOutcome)
	if !isValid {
		t.Fatalf("outcome type = %T, want ValidOutcome", outcome)
	}
	actualCandidateBytes, err := valid.Candidate().CanonicalBytes()
	if err != nil {
		t.Fatalf("outcome candidate CanonicalBytes() error = %v", err)
	}
	if !bytes.Equal(actualCandidateBytes, expectedCandidateBytes) {
		t.Fatal("ValidOutcome did not retain the exact bound candidate")
	}
	actualCandidateDigest, err := valid.Candidate().Digest()
	if err != nil {
		t.Fatalf("outcome candidate Digest() error = %v", err)
	}
	if actualCandidateDigest != expectedCandidateDigest {
		t.Fatalf(
			"candidate digest = %s, want %s",
			actualCandidateDigest.String(),
			expectedCandidateDigest.String(),
		)
	}

	batch := valid.AdmissionBatch()
	if !batch.IsValid() {
		t.Fatal("ValidOutcome retained a non-admissible AdmissionBatch")
	}
	if batch.RequestDigest() != expectedCandidateDigest {
		t.Fatal("AdmissionBatch does not correlate the retained candidate")
	}
	if batch.SemanticChangeDigest() != valid.SemanticChangeDigest() {
		t.Fatal("AdmissionBatch semantic digest drifted from ValidOutcome")
	}
	batchBasis := batch.Basis()
	outcomeBasis := valid.AdmissionBasis()
	if batchBasis == nil || outcomeBasis == nil {
		t.Fatal("ValidOutcome did not retain an exact AdmissionBasis")
	}
	if outcomeBasis.Kind() != batchBasis.Kind() ||
		outcomeBasis.Digest() != batchBasis.Digest() ||
		!bytes.Equal(outcomeBasis.CanonicalBytes(), batchBasis.CanonicalBytes()) {
		t.Fatal("ValidOutcome AdmissionBasis drifted from AdmissionBatch")
	}
	if outcomeBasis.TypeEnv() != environment.Ref() {
		t.Fatalf(
			"AdmissionBasis TypeEnv = %s, want %s",
			outcomeBasis.TypeEnv().String(),
			environment.Ref().String(),
		)
	}
	if outcomeBasis.GraphRevision() != snapshot.GraphRevision() {
		t.Fatalf(
			"AdmissionBasis graph revision = %d, want %d",
			outcomeBasis.GraphRevision().Value(),
			snapshot.GraphRevision().Value(),
		)
	}

	changes := valid.Candidate().Changes()
	changes[0] = nil
	retained := valid.Candidate().Changes()
	if len(retained) != 1 || retained[0] == nil {
		t.Fatal("Candidate() exposed mutable retained candidate state")
	}
}

func TestNonValidOutcomesCannotExposeAdmissionCapability(t *testing.T) {
	resolver := &fixedResolver{resolution: NewProjectBasisUnavailable()}
	service := mustService(t, resolver)
	tests := []struct {
		name    string
		request typedmemorywire.ValidateRequest
		kind    typedmemory.ValidationVerdictKind
	}{
		{
			name:    "invalid",
			request: typedmemorywire.ValidateRequest{},
			kind:    typedmemory.ValidationInvalid,
		},
		{
			name:    "underdetermined",
			request: decodeRequest(t, `{"kind":"project_current"}`),
			kind:    typedmemory.ValidationUnderdetermined,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outcome := service.Evaluate(test.request)
			if outcome.Verdict() != test.kind {
				t.Fatalf("Verdict() = %q, want %q", outcome.Verdict(), test.kind)
			}
			if _, exposesAdmission := outcome.(ValidOutcome); exposesAdmission {
				t.Fatalf("non-valid outcome type %T satisfies ValidOutcome", outcome)
			}
			assertOutcomeHasNoAdmissionCapability(t, outcome)
		})
	}
}

func TestValidatePresentsOneEvaluationWithoutResponseDrift(t *testing.T) {
	environment := testEnvironment(t, "7")
	snapshot := newTestSnapshot(t, environment.Ref(), 37, entityAbsent)
	resolved, err := NewResolvedProjectBasis(
		environment,
		typedmemory.NewCodecRegistry(),
		snapshot,
	)
	if err != nil {
		t.Fatalf("NewResolvedProjectBasis() error = %v", err)
	}
	resolver := &fixedResolver{resolution: resolved}
	service := mustService(t, resolver)
	wireRequest := decodeRequest(t, `{"kind":"project_current"}`)
	request := &trackingRequest{request: wireRequest}

	response := presentOutcome(service.evaluate(request))

	if request.bindCount != 1 {
		t.Fatalf("Validate bound the candidate %d times, want exactly once", request.bindCount)
	}
	if resolver.callCount != 1 {
		t.Fatalf("Validate resolved the basis %d times, want exactly once", resolver.callCount)
	}
	if snapshot.entityCalls != 1 {
		t.Fatalf("Validate queried entity state %d times, want exactly once", snapshot.entityCalls)
	}
	valid, isValid := response.(ValidResponse)
	if !isValid {
		t.Fatalf("response type = %T, want ValidResponse", response)
	}
	publicResolver := &fixedResolver{resolution: resolved}
	publicService := mustService(t, publicResolver)
	publicResponse := publicService.Validate(wireRequest)
	publicValid, isPublicValid := publicResponse.(ValidResponse)
	if !isPublicValid {
		t.Fatalf("public response type = %T, want ValidResponse", publicResponse)
	}
	if publicResolver.callCount != 1 {
		t.Fatalf(
			"Service.Validate resolved the basis %d times, want exactly once",
			publicResolver.callCount,
		)
	}
	payload, err := json.Marshal(publicResponse)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	expected := fmt.Sprintf(
		`{"contract_version":%q,"action":"validate","verdict":"valid","basis":{"requested_kind":"project_current","resolution_kind":"resolved_project_basis","type_env_ref":%q,"graph_revision":37},"persistence_disposition":{"mode":"validation_only_no_write","rows_written":0,"authority_granted":false},"diagnostics":[],"normalized_digest":%q}`,
		typedmemorywire.ContractVersion,
		environment.Ref().String(),
		publicValid.NormalizedDigest().String(),
	)
	if string(payload) != expected {
		t.Fatalf("public response bytes drifted:\n got: %s\nwant: %s", payload, expected)
	}
	if valid.NormalizedDigest() != publicValid.NormalizedDigest() {
		t.Fatal("private compatibility wrapper and Service.Validate semantic digests differ")
	}
	assertNoAdmissionCapability(t, publicResponse)
}

func TestValidateAndOutcomePresentationRemainByteEquivalent(t *testing.T) {
	tests := []struct {
		name    string
		posture entityPosture
	}{
		{
			name:    "valid",
			posture: entityAbsent,
		},
		{
			name:    "invalid",
			posture: entityExact,
		},
		{
			name:    "underdetermined",
			posture: entityUnsettled,
		},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			digit := fmt.Sprintf("%x", index+8)
			environment := testEnvironment(t, digit)
			snapshot := newTestSnapshot(t, environment.Ref(), 41, test.posture)
			resolution, err := NewResolvedProjectBasis(
				environment,
				typedmemory.NewCodecRegistry(),
				snapshot,
			)
			if err != nil {
				t.Fatalf("NewResolvedProjectBasis() error = %v", err)
			}
			request := decodeRequest(t, `{"kind":"project_current"}`)
			evaluateService := mustService(t, &fixedResolver{resolution: resolution})
			validateService := mustService(t, &fixedResolver{resolution: resolution})

			outcome := evaluateService.Evaluate(request)
			projectedPayload, err := json.Marshal(presentOutcome(outcome))
			if err != nil {
				t.Fatalf("json.Marshal(projected outcome) error = %v", err)
			}
			publicPayload, err := json.Marshal(validateService.Validate(request))
			if err != nil {
				t.Fatalf("json.Marshal(Validate response) error = %v", err)
			}
			if !bytes.Equal(publicPayload, projectedPayload) {
				t.Fatalf(
					"Validate response drifted from outcome projection:\npublic=%s\noutcome=%s",
					publicPayload,
					projectedPayload,
				)
			}
		})
	}
}

func assertOutcomeHasNoAdmissionCapability(t *testing.T, outcome Outcome) {
	t.Helper()
	typeOf := reflect.TypeOf(outcome)
	for _, method := range []string{"Candidate", "AdmissionBatch", "AdmissionBasis"} {
		if _, exists := typeOf.MethodByName(method); exists {
			t.Fatalf("outcome type %T exposes %s", outcome, method)
		}
	}
	admissionBatchType := reflect.TypeOf(typedmemory.AdmissionBatch{})
	memoryChangeSetType := reflect.TypeOf(typedmemory.MemoryChangeSet{})
	assertTypeContainsNoCapability(t, typeOf, admissionBatchType, memoryChangeSetType)
}

func assertTypeContainsNoCapability(
	t *testing.T,
	value reflect.Type,
	admissionBatch reflect.Type,
	memoryChangeSet reflect.Type,
) {
	t.Helper()
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if value == admissionBatch || value == memoryChangeSet {
		t.Fatalf("non-valid outcome contains capability field %s", value)
	}
	if value.Kind() != reflect.Struct {
		return
	}
	for index := 0; index < value.NumField(); index++ {
		field := value.Field(index)
		assertTypeContainsNoCapability(
			t,
			field.Type,
			admissionBatch,
			memoryChangeSet,
		)
	}
}
