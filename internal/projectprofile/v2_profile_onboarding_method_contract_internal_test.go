package projectprofile

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"testing"
)

func TestProfileOnboardingMethodV2RequiresExactBasisAndReviewedWorkInput(t *testing.T) {
	fixture := exactProfileOnboardingWorkSupportFixtureV1(t)
	record, description, contract, workInputRef := exactProfileOnboardingWorkV2Fixture(
		t,
		fixture,
	)

	_, err := EvaluateProfileOnboardingOccurrenceContractV2(
		record,
		description,
		contract,
		fixture.assignment,
		fixture.basis,
		workInputRef,
	)
	if err != nil {
		t.Fatalf("EvaluateProfileOnboardingOccurrenceContractV2: %v", err)
	}

	basisInputRef := WorkInputRef{
		v1Reference: v1Reference{value: fixture.basis.Ref().String()},
	}
	extraInputRef := WorkInputRef{
		v1Reference: v1Reference{value: "profile-onboarding-work-input:extra"},
	}
	tests := []struct {
		name      string
		inputRefs []WorkInputRef
	}{
		{name: "missing reviewed WorkInput", inputRefs: []WorkInputRef{basisInputRef}},
		{name: "missing observed basis", inputRefs: []WorkInputRef{workInputRef, extraInputRef}},
		{name: "extra input", inputRefs: []WorkInputRef{basisInputRef, workInputRef, extraInputRef}},
		{name: "duplicate input", inputRefs: []WorkInputRef{basisInputRef, workInputRef, workInputRef}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := record
			mutated.inputRefs = test.inputRefs
			if _, err := canonicalizeProfileOnboardingWorkRecord(mutated); err == nil {
				t.Fatal("v2 Work accepted an invalid input set")
			}
		})
	}

	mixed := record
	mixed.methodDescriptionRef = ProfileOnboardingMethodDescriptionRefV1()
	if _, err := canonicalizeProfileOnboardingWorkRecord(mixed); err == nil {
		t.Fatal("Work accepted mixed v1/v2 method refs")
	}
}

func TestProfileOnboardingMethodV2CanonicalRoundTripAndDigest(t *testing.T) {
	fixture := exactProfileOnboardingWorkSupportFixtureV1(t)
	record, description, contract, _ := exactProfileOnboardingWorkV2Fixture(t, fixture)

	descriptionJSON, err := EncodeProfileOnboardingMethodDescriptionV2CanonicalJSON(description)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingMethodDescriptionV2CanonicalJSON: %v", err)
	}
	decodedDescription, err := DecodeProfileOnboardingMethodDescriptionV2CanonicalJSON(descriptionJSON)
	if err != nil {
		t.Fatalf("DecodeProfileOnboardingMethodDescriptionV2CanonicalJSON: %v", err)
	}
	if decodedDescription.Edition() != "v2" {
		t.Fatalf("decoded MethodDescription edition = %q", decodedDescription.Edition())
	}
	inputKinds := profileOnboardingInputKindsToStringsV1(decodedDescription.AcceptedInputKinds())
	slices.Sort(inputKinds)
	wantInputKinds := []string{"ObservedProjectBasisV1", "ProfileOnboardingWorkInputV1"}
	slices.Sort(wantInputKinds)
	if !slices.Equal(inputKinds, wantInputKinds) {
		t.Fatalf("v2 accepted inputs = %#v", inputKinds)
	}

	contractJSON, err := EncodeProfileOnboardingMethodContractV2CanonicalJSON(contract)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingMethodContractV2CanonicalJSON: %v", err)
	}
	decodedContract, err := DecodeProfileOnboardingMethodContractV2CanonicalJSON(contractJSON)
	if err != nil {
		t.Fatalf("DecodeProfileOnboardingMethodContractV2CanonicalJSON: %v", err)
	}
	if decodedContract.Ref() != contract.Ref() {
		t.Fatal("v2 MethodContract ref changed across roundtrip")
	}

	recordJSON, err := EncodeProfileOnboardingWorkRecordCanonicalJSON(record)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingWorkRecordCanonicalJSON(v2): %v", err)
	}
	decodedRecord, err := DecodeProfileOnboardingWorkRecordCanonicalJSON(recordJSON)
	if err != nil {
		t.Fatalf("DecodeProfileOnboardingWorkRecordCanonicalJSON(v2): %v", err)
	}
	firstDigest, err := DigestProfileOnboardingWorkRecord(record)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingWorkRecord(v2): %v", err)
	}
	secondDigest, err := DigestProfileOnboardingWorkRecord(decodedRecord)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingWorkRecord(decoded v2): %v", err)
	}
	if firstDigest != secondDigest {
		t.Fatal("v2 Work digest changed across canonical roundtrip")
	}
}

func TestHistoricalProfileOnboardingV1CanonicalBytesRemainFrozen(t *testing.T) {
	fixture := exactProfileOnboardingWorkSupportFixtureV1(t)
	descriptionJSON, err := EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON(fixture.description)
	if err != nil {
		t.Fatal(err)
	}
	contractJSON, err := EncodeProfileOnboardingMethodContractV1CanonicalJSON(fixture.contract)
	if err != nil {
		t.Fatal(err)
	}
	recordJSON, err := EncodeProfileOnboardingWorkRecordCanonicalJSON(fixture.record)
	if err != nil {
		t.Fatal(err)
	}
	assertFrozenProfileOnboardingBytes(t, "MethodDescription v1", descriptionJSON, "58d12536d3326d3f45ed28eb3f38180b04c2e44460729a9e83201bc12a414dff")
	assertFrozenProfileOnboardingBytes(t, "MethodContract v1", contractJSON, "433328c9fb275a7d88c95c62be6c09e2db034823c7d1ca2f77b713e15a9081d7")
	assertFrozenProfileOnboardingBytes(t, "Work v1", recordJSON, "0c483a177ea40922fe6549376aa1aace8ed8d1a9db3d7eb321e411ce89073ab6")
}

func exactProfileOnboardingWorkV2Fixture(
	t *testing.T,
	fixture exactWorkSupportFixtureV1,
) (
	ProfileOnboardingWorkRecord,
	ProfileOnboardingMethodDescriptionV2,
	ProfileOnboardingMethodContractV2,
	WorkInputRef,
) {
	t.Helper()
	description := ProfileOnboardingMethodDescriptionV2Value()
	descriptionDigest, err := DigestProfileOnboardingMethodDescriptionV2(description)
	if err != nil {
		t.Fatal(err)
	}
	contract, err := ProfileOnboardingMethodContractV2Value()
	if err != nil {
		t.Fatal(err)
	}
	contractDigest, err := DigestProfileOnboardingMethodContractV2(contract)
	if err != nil {
		t.Fatal(err)
	}
	workInputRef := WorkInputRef{
		v1Reference: v1Reference{value: "profile-onboarding-work-input:reviewed-v3"},
	}
	basisInputRef := WorkInputRef{
		v1Reference: v1Reference{value: fixture.basis.Ref().String()},
	}
	record := fixture.record
	record.enactsMethodRef = description.DescribedMethodRef()
	record.methodDescriptionRef = description.Ref()
	record.methodDescriptionDigest = descriptionDigest
	record.methodContractRef = contract.Ref()
	record.methodContractDigest = contractDigest
	record.inputRefs = []WorkInputRef{workInputRef, basisInputRef}
	canonical, err := canonicalizeProfileOnboardingWorkRecord(record)
	if err != nil {
		t.Fatalf("canonicalize v2 Work: %v", err)
	}
	return canonical, description, contract, workInputRef
}

func assertFrozenProfileOnboardingBytes(
	t *testing.T,
	name string,
	data []byte,
	want string,
) {
	t.Helper()
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("%s canonical bytes hash = %s, want %s", name, got, want)
	}
}
