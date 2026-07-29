package projectprofile

import (
	"bytes"
	"testing"
)

func TestExactProfileOnboardingMethodDescriptionV1RejectsMutatedPackageValues(t *testing.T) {
	base := newProfileOnboardingMethodDescriptionV1()
	tests := []struct {
		name   string
		mutate func(profileOnboardingMethodDescriptionV1) profileOnboardingMethodDescriptionV1
	}{
		{
			name: "source revision",
			mutate: func(value profileOnboardingMethodDescriptionV1) profileOnboardingMethodDescriptionV1 {
				value.sourceRevision.value = "54dd88188a07646ef23aca32627a3f670525853f"
				return value
			},
		},
		{
			name: "method ref",
			mutate: func(value profileOnboardingMethodDescriptionV1) profileOnboardingMethodDescriptionV1 {
				value.describedMethodRef.v1Reference.value = "haft:method:other/v1"
				return value
			},
		},
		{
			name: "parameter declaration",
			mutate: func(value profileOnboardingMethodDescriptionV1) profileOnboardingMethodDescriptionV1 {
				value.parameterDeclarations = append(
					[]ProfileOnboardingParameterDeclarationV1{},
					value.parameterDeclarations...,
				)
				value.parameterDeclarations[0].name = "foreign_parameter"
				return value
			},
		},
		{
			name: "result union",
			mutate: func(value profileOnboardingMethodDescriptionV1) profileOnboardingMethodDescriptionV1 {
				value.acceptedResultKinds = append(
					[]ProfileOnboardingResultKindV1{},
					value.acceptedResultKinds...,
				)
				value.acceptedResultKinds[0].value = "FinalProfileAdmitted"
				return value
			},
		},
		{
			name: "system kind",
			mutate: func(value profileOnboardingMethodDescriptionV1) profileOnboardingMethodDescriptionV1 {
				value.requiredSystemKind.value = "U.Episteme"
				return value
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := test.mutate(base)
			if _, err := EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON(mutated); err == nil {
				t.Fatalf("encoder accepted mutated package-owned %s", test.name)
			}
		})
	}
	if _, err := EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON(profileOnboardingMethodDescriptionV1{}); err == nil {
		t.Fatal("encoder accepted zero MethodDescription")
	}
}

func TestExactProfileOnboardingMethodContractV1RejectsMutatedPackageValues(t *testing.T) {
	base, err := newProfileOnboardingMethodContractV1()
	if err != nil {
		t.Fatalf("newProfileOnboardingMethodContractV1: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(profileOnboardingMethodContractV1) profileOnboardingMethodContractV1
	}{
		{
			name: "description digest",
			mutate: func(value profileOnboardingMethodContractV1) profileOnboardingMethodContractV1 {
				writer := newCanonicalDigestWriter("foreign-description")
				value.methodDescriptionDigest = writer.digest()
				return value
			},
		},
		{
			name: "parameter digest",
			mutate: func(value profileOnboardingMethodContractV1) profileOnboardingMethodContractV1 {
				writer := newCanonicalDigestWriter("foreign-parameters")
				value.parameterSpecSetDigest = writer.digest()
				return value
			},
		},
		{
			name: "occurrence slot",
			mutate: func(value profileOnboardingMethodContractV1) profileOnboardingMethodContractV1 {
				value.requiredOccurrenceSlots = append(
					[]ProfileOnboardingOccurrenceSlotV1{},
					value.requiredOccurrenceSlots...,
				)
				value.requiredOccurrenceSlots[0].value = "calendar_plan"
				return value
			},
		},
		{
			name: "holder rule",
			mutate: func(value profileOnboardingMethodContractV1) profileOnboardingMethodContractV1 {
				value.holderEqualsExecutedWithinRule = holderEqualsExecutedWithinV1{}
				value.acceptanceStandardEdition = "v2"
				return value
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := test.mutate(base)
			if _, err := EncodeProfileOnboardingMethodContractV1CanonicalJSON(mutated); err == nil {
				t.Fatalf("encoder accepted mutated package-owned %s", test.name)
			}
		})
	}
	if _, err := EncodeProfileOnboardingMethodContractV1CanonicalJSON(profileOnboardingMethodContractV1{}); err == nil {
		t.Fatal("encoder accepted zero MethodContract")
	}
}

func TestProfileOnboardingMethodDigestsAreDomainSeparated(t *testing.T) {
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
	if descriptionDigest == contractDigest {
		t.Fatal("MethodDescription and MethodContract digests are not domain-separated")
	}
	descriptionJSON, err := EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON(description)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON: %v", err)
	}
	wrongDomainDigest := digestProfileOnboardingCanonicalJSONV1(
		profileOnboardingMethodContractDigestV1,
		descriptionJSON,
	)
	if descriptionDigest == wrongDomainDigest {
		t.Fatal("MethodDescription digest ignored its domain")
	}

	decoded, err := DecodeProfileOnboardingMethodDescriptionV1CanonicalJSON(descriptionJSON)
	if err != nil {
		t.Fatalf("DecodeProfileOnboardingMethodDescriptionV1CanonicalJSON: %v", err)
	}
	reencoded, err := EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON(decoded)
	if err != nil {
		t.Fatalf("re-encode MethodDescription: %v", err)
	}
	if !bytes.Equal(descriptionJSON, reencoded) {
		t.Fatal("MethodDescription canonical round trip changed bytes")
	}
}
