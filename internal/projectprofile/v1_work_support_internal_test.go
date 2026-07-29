package projectprofile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

type exactWorkSupportFixtureV1 struct {
	record            ProfileOnboardingWorkRecord
	description       ProfileOnboardingMethodDescriptionV1
	contract          ProfileOnboardingMethodContractV1
	assignment        ProfileAuthorRoleAssignmentV1
	assignmentSupport ProfileAuthorAssignmentSupportCarrierV1
	basis             ObservedProjectBasisV1
}

func TestProfileOnboardingWorkRecordValidatesExactSupportClosure(t *testing.T) {
	fixture := exactProfileOnboardingWorkSupportFixtureV1(t)
	err := ValidateProfileOnboardingWorkRecordAgainstSupportV1(
		fixture.record,
		fixture.description,
		fixture.contract,
		fixture.assignment,
		fixture.assignmentSupport,
		fixture.basis,
	)
	if err != nil {
		t.Fatalf("ValidateProfileOnboardingWorkRecordAgainstSupportV1: %v", err)
	}
}

func TestProfileOnboardingWorkRecordRejectsBrokenExactSupportRelations(t *testing.T) {
	fixture := exactProfileOnboardingWorkSupportFixtureV1(t)
	cases := []struct {
		name   string
		mutate func(ProfileOnboardingWorkRecord) ProfileOnboardingWorkRecord
		error  string
	}{
		{
			name: "method description digest",
			mutate: func(value ProfileOnboardingWorkRecord) ProfileOnboardingWorkRecord {
				value.methodDescriptionDigest = workSupportDigestV1(t, "foreign-description")
				return value
			},
			error: "MethodDescription digest",
		},
		{
			name: "assignment digest",
			mutate: func(value ProfileOnboardingWorkRecord) ProfileOnboardingWorkRecord {
				value.profileAuthorRoleAssignmentDigest = workSupportDigestV1(t, "foreign-assignment")
				return value
			},
			error: "ProfileAuthorRoleAssignment digest",
		},
		{
			name: "observed basis digest",
			mutate: func(value ProfileOnboardingWorkRecord) ProfileOnboardingWorkRecord {
				value.observedProjectBasisDigest = workSupportDigestV1(t, "foreign-basis")
				return value
			},
			error: "ObservedProjectBasis digest",
		},
		{
			name: "session",
			mutate: func(value ProfileOnboardingWorkRecord) ProfileOnboardingWorkRecord {
				value.parameterBindings = exactWorkSupportBindingsV1(t, "session:other")
				return value
			},
			error: "session",
		},
		{
			name: "input ref",
			mutate: func(value ProfileOnboardingWorkRecord) ProfileOnboardingWorkRecord {
				value.inputRefs = []WorkInputRef{{v1Reference: v1Reference{value: "observed-project-basis:other"}}}
				return value
			},
			error: "inputs do not reference ObservedProjectBasis",
		},
		{
			name: "state plane",
			mutate: func(value ProfileOnboardingWorkRecord) ProfileOnboardingWorkRecord {
				value.statePlaneRef = StatePlaneRef{v1Reference: v1Reference{value: "state-plane:other"}}
				return value
			},
			error: "StatePlane",
		},
		{
			name: "work outside assignment and system admission",
			mutate: func(value ProfileOnboardingWorkRecord) ProfileOnboardingWorkRecord {
				from := time.Date(2026, 7, 14, 13, 0, 0, 0, time.UTC)
				value.workInterval = WorkIntervalV1{closedIntervalV1: closedIntervalV1{
					from:  from,
					until: from.Add(time.Hour),
				}}
				return value
			},
			error: "window",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			record := testCase.mutate(fixture.record)
			err := ValidateProfileOnboardingWorkRecordAgainstSupportV1(
				record,
				fixture.description,
				fixture.contract,
				fixture.assignment,
				fixture.assignmentSupport,
				fixture.basis,
			)
			if err == nil {
				t.Fatal("broken exact support relation accepted")
			}
			message := err.Error()
			if !strings.Contains(message, testCase.error) {
				t.Fatalf("broken exact support relation accepted: %v", err)
			}
		})
	}
}

func TestProfileOnboardingWorkRecordCanonicalCarrierBindsExactSupportPairs(t *testing.T) {
	record := exactProfileOnboardingWorkSupportFixtureV1(t).record
	canonical, err := EncodeProfileOnboardingWorkRecordCanonicalJSON(record)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingWorkRecordCanonicalJSON: %v", err)
	}
	requiredKeys := []string{
		`"method_description_digest"`,
		`"method_contract_ref"`,
		`"method_contract_digest"`,
		`"profile_author_role_assignment_ref"`,
		`"profile_author_role_assignment_digest"`,
		`"observed_project_basis_ref"`,
		`"observed_project_basis_digest"`,
		`"affected_ref_kind"`,
	}
	for _, key := range requiredKeys {
		keyBytes := []byte(key)
		if !bytes.Contains(canonical, keyBytes) {
			t.Fatalf("canonical Work JSON omits support field %s", key)
		}
	}
	decoded, err := DecodeProfileOnboardingWorkRecordCanonicalJSON(canonical)
	if err != nil {
		t.Fatalf("DecodeProfileOnboardingWorkRecordCanonicalJSON: %v", err)
	}
	left, err := DigestProfileOnboardingWorkRecord(record)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingWorkRecord(original): %v", err)
	}
	right, err := DigestProfileOnboardingWorkRecord(decoded)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingWorkRecord(decoded): %v", err)
	}
	if left != right {
		t.Fatal("Work digest changed across canonical support-pair roundtrip")
	}
}

func TestProfileOnboardingWorkRecordRejectsMissingSupportBindings(t *testing.T) {
	record := exactProfileOnboardingWorkSupportFixtureV1(t).record
	cases := []struct {
		name   string
		mutate func(ProfileOnboardingWorkRecord) ProfileOnboardingWorkRecord
		error  string
	}{
		{
			name: "method description digest",
			mutate: func(value ProfileOnboardingWorkRecord) ProfileOnboardingWorkRecord {
				value.methodDescriptionDigest = ContentDigest{}
				return value
			},
			error: "MethodDescription digest",
		},
		{
			name: "method contract",
			mutate: func(value ProfileOnboardingWorkRecord) ProfileOnboardingWorkRecord {
				value.methodContractRef = ProfileOnboardingMethodContractRefV1{}
				return value
			},
			error: "MethodContract",
		},
		{
			name: "role assignment digest",
			mutate: func(value ProfileOnboardingWorkRecord) ProfileOnboardingWorkRecord {
				value.profileAuthorRoleAssignmentDigest = ContentDigest{}
				return value
			},
			error: "ProfileAuthorRoleAssignment",
		},
		{
			name: "observed basis ref",
			mutate: func(value ProfileOnboardingWorkRecord) ProfileOnboardingWorkRecord {
				value.observedProjectBasisRef = ObservedProjectBasisRefV1{}
				return value
			},
			error: "ObservedProjectBasis",
		},
		{
			name: "affected kind",
			mutate: func(value ProfileOnboardingWorkRecord) ProfileOnboardingWorkRecord {
				value.affectedRefKind = ProfileOnboardingAffectedKindV1{}
				return value
			},
			error: "affected kind",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			mutated := testCase.mutate(record)
			_, err := canonicalizeProfileOnboardingWorkRecord(mutated)
			if err == nil {
				t.Fatal("missing support binding accepted")
			}
			message := err.Error()
			if !strings.Contains(message, testCase.error) {
				t.Fatalf("missing support binding accepted: %v", err)
			}
		})
	}
}

func TestProfileOnboardingWorkDigestBindsEverySupportDigest(t *testing.T) {
	record := exactProfileOnboardingWorkSupportFixtureV1(t).record
	baseline, err := DigestProfileOnboardingWorkRecord(record)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingWorkRecord: %v", err)
	}
	mutations := []func(ProfileOnboardingWorkRecord) ProfileOnboardingWorkRecord{
		func(value ProfileOnboardingWorkRecord) ProfileOnboardingWorkRecord {
			value.methodDescriptionDigest = workSupportDigestV1(t, "other-description")
			return value
		},
		func(value ProfileOnboardingWorkRecord) ProfileOnboardingWorkRecord {
			value.methodContractDigest = workSupportDigestV1(t, "other-contract")
			return value
		},
		func(value ProfileOnboardingWorkRecord) ProfileOnboardingWorkRecord {
			value.profileAuthorRoleAssignmentDigest = workSupportDigestV1(t, "other-assignment")
			return value
		},
		func(value ProfileOnboardingWorkRecord) ProfileOnboardingWorkRecord {
			value.observedProjectBasisDigest = workSupportDigestV1(t, "other-basis")
			return value
		},
	}
	for index, mutate := range mutations {
		mutated := mutate(record)
		changed, err := DigestProfileOnboardingWorkRecord(mutated)
		if err != nil {
			t.Fatalf("DigestProfileOnboardingWorkRecord(mutation %d): %v", index, err)
		}
		if changed == baseline {
			t.Fatalf("support digest mutation %d did not change Work digest", index)
		}
	}
}

func exactProfileOnboardingWorkSupportFixtureV1(t *testing.T) exactWorkSupportFixtureV1 {
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
	sessionRef := SessionRef{v1Reference: v1Reference{value: "session:1"}}
	systemRef := SystemRef{v1Reference: v1Reference{value: "system:host-agent:1"}}
	kernel, err := NewProfileOnboardingKernelIdentityV1("haft-kernel", "v1")
	if err != nil {
		t.Fatalf("NewProfileOnboardingKernelIdentityV1: %v", err)
	}
	runtime, err := NewProfileOnboardingRuntimeIdentityV1("codex", "v1")
	if err != nil {
		t.Fatalf("NewProfileOnboardingRuntimeIdentityV1: %v", err)
	}
	identityBasis, err := NewProfileOnboardingKernelExecutorIdentityBasisV1(systemRef, kernel)
	if err != nil {
		t.Fatalf("NewProfileOnboardingKernelExecutorIdentityBasisV1: %v", err)
	}
	systemWindowFrom := time.Date(2026, 7, 14, 7, 0, 0, 0, time.UTC)
	systemWindowUntil := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	systemWindow, err := NewProfileOnboardingExecutorAdmissionWindowV1(
		systemWindowFrom,
		systemWindowUntil,
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingExecutorAdmissionWindowV1: %v", err)
	}
	systemAdmissionBuilder := NewProfileOnboardingExecutorSystemAdmissionV1Builder(
		SystemAdmissionRef{v1Reference: v1Reference{value: "system-admission:executor:1"}},
		systemRef,
	)
	systemAdmissionBuilder = systemAdmissionBuilder.IdentifiedBy(identityBasis)
	systemAdmissionBuilder = systemAdmissionBuilder.AdmittedToActBy(
		ProfileOnboardingSystemActingEligibilityBasisRefV1{
			v1Reference: v1Reference{value: "acting-eligibility:executor:1"},
		},
		workSupportDigestV1(t, "acting-eligibility"),
	)
	systemAdmissionBuilder = systemAdmissionBuilder.InSession(sessionRef)
	systemAdmissionBuilder = systemAdmissionBuilder.ValidDuring(systemWindow)
	systemAdmission, err := systemAdmissionBuilder.Build()
	if err != nil {
		t.Fatalf("ProfileOnboardingExecutorSystemAdmissionV1Builder.Build: %v", err)
	}
	roleAdmission, err := NewProfileAuthorRoleAdmissionV1(
		RoleAdmissionRef{v1Reference: v1Reference{value: "role-admission:profile-author:1"}},
	)
	if err != nil {
		t.Fatalf("NewProfileAuthorRoleAdmissionV1: %v", err)
	}
	assignmentWindowFrom := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	assignmentWindowUntil := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	assignmentWindow, err := NewRoleAssignmentWindowV1(
		assignmentWindowFrom,
		assignmentWindowUntil,
	)
	if err != nil {
		t.Fatalf("NewRoleAssignmentWindowV1: %v", err)
	}
	justificationBuilder := NewProfileAuthorAssignmentJustificationV1Builder(
		RoleAssignmentJustificationRef{
			v1Reference: v1Reference{value: "assignment-justification:profile-author:1"},
		},
	)
	justificationBuilder = justificationBuilder.ApplyingAdmissions(systemAdmission, roleAdmission)
	justificationBuilder = justificationBuilder.ValidDuring(assignmentWindow)
	justification, err := justificationBuilder.Build()
	if err != nil {
		t.Fatalf("ProfileAuthorAssignmentJustificationV1Builder.Build: %v", err)
	}
	provenanceBuilder := NewProfileAuthorAssignmentProvenanceV1Builder(
		RoleAssignmentProvenanceRef{
			v1Reference: v1Reference{value: "assignment-provenance:profile-author:1"},
		},
		justification,
	)
	provenanceBuilder = provenanceBuilder.InSession(sessionRef)
	provenanceBuilder = provenanceBuilder.ProducedBy(kernel, runtime)
	provenanceRecordedAt := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	provenanceBuilder = provenanceBuilder.RecordedAt(provenanceRecordedAt)
	provenance, err := provenanceBuilder.Build()
	if err != nil {
		t.Fatalf("ProfileAuthorAssignmentProvenanceV1Builder.Build: %v", err)
	}
	assignmentSupport, err := CarryProfileAuthorAssignmentSupportV1(
		systemAdmission,
		roleAdmission,
		justification,
		provenance,
	)
	if err != nil {
		t.Fatalf("CarryProfileAuthorAssignmentSupportV1: %v", err)
	}
	assignmentRef := RoleAssignmentRef{
		v1Reference: v1Reference{value: "role-assignment:1"},
	}
	assignmentBuilder := NewProfileAuthorRoleAssignmentV1Builder(assignmentRef)
	assignmentBuilder = assignmentBuilder.HeldBy(systemRef)
	profileAuthorRoleRef := ProfileAuthorRoleRefV1()
	assignmentBuilder = assignmentBuilder.Assigning(profileAuthorRoleRef)
	boundedContextRef := ProfileOnboardingBoundedContextRefV1()
	assignmentBuilder = assignmentBuilder.InContext(boundedContextRef)
	assignmentBuilder = assignmentBuilder.ValidDuring(assignmentWindow)
	systemAdmissionRef := systemAdmission.Ref()
	systemAdmissionDigest := assignmentSupport.SystemAdmissionDigest()
	assignmentBuilder = assignmentBuilder.WithSystemAdmission(
		systemAdmissionRef,
		systemAdmissionDigest,
	)
	roleAdmissionRef := roleAdmission.Ref()
	roleAdmissionDigest := assignmentSupport.RoleAdmissionDigest()
	assignmentBuilder = assignmentBuilder.WithRoleAdmission(
		roleAdmissionRef,
		roleAdmissionDigest,
	)
	justificationRef := justification.Ref()
	justificationDigest := assignmentSupport.JustificationDigest()
	assignmentBuilder = assignmentBuilder.JustifiedBy(
		justificationRef,
		justificationDigest,
	)
	provenanceRef := provenance.Ref()
	provenanceDigest := assignmentSupport.ProvenanceDigest()
	assignmentBuilder = assignmentBuilder.WithProvenance(
		provenanceRef,
		provenanceDigest,
	)
	assignment, err := assignmentBuilder.Build()
	if err != nil {
		t.Fatalf("ProfileAuthorRoleAssignmentV1Builder.Build: %v", err)
	}
	assignmentDigest, err := DigestProfileAuthorRoleAssignmentV1(assignment)
	if err != nil {
		t.Fatalf("DigestProfileAuthorRoleAssignmentV1: %v", err)
	}
	basisWindowFrom := time.Date(2026, 7, 14, 8, 30, 0, 0, time.UTC)
	basisWindowUntil := time.Date(2026, 7, 14, 9, 30, 0, 0, time.UTC)
	basisWindow, err := NewBasisObservationWindowV1(
		basisWindowFrom,
		basisWindowUntil,
	)
	if err != nil {
		t.Fatalf("NewBasisObservationWindowV1: %v", err)
	}
	basis := exactObservedProjectBasisForWorkV1(t, basisWindow)
	basisDigest, err := DigestObservedProjectBasisV1(basis)
	if err != nil {
		t.Fatalf("DigestObservedProjectBasisV1: %v", err)
	}
	workIntervalFrom := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	workIntervalUntil := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	workInterval, err := NewWorkIntervalV1(
		workIntervalFrom,
		workIntervalUntil,
	)
	if err != nil {
		t.Fatalf("NewWorkIntervalV1: %v", err)
	}
	transition, err := NewPrePostStateTransitionV1(
		StateRef{v1Reference: v1Reference{value: "state:before"}},
		StateRef{v1Reference: v1Reference{value: "state:after"}},
	)
	if err != nil {
		t.Fatalf("NewPrePostStateTransitionV1: %v", err)
	}
	payloadDigest := workSupportDigestV1(t, "payload")
	outcome, err := NewCandidatePayloadProduced(payloadDigest, basisDigest)
	if err != nil {
		t.Fatalf("NewCandidatePayloadProduced: %v", err)
	}
	workBuilder := NewProfileOnboardingWorkRecordBuilder(
		ProfileOnboardingWorkRecordRef{v1Reference: v1Reference{value: "work-record:exact-support:1"}},
		WorkRef{v1Reference: v1Reference{value: "work:exact-support:1"}},
	)
	methodRef := description.DescribedMethodRef()
	methodDescriptionRef := description.Ref()
	sessionRefValue := sessionRef.String()
	parameterBindings := exactWorkSupportBindingsV1(t, sessionRefValue)
	workBuilder = workBuilder.Enacts(
		methodRef,
		methodDescriptionRef,
		parameterBindings,
	)
	workBuilder = workBuilder.WithMethodDescriptionDigest(descriptionDigest)
	methodContractRef := contract.Ref()
	workBuilder = workBuilder.GovernedByMethodContract(methodContractRef, contractDigest)
	workBuilder = workBuilder.PerformedBy(assignmentRef)
	workBuilder = workBuilder.WithProfileAuthorRoleAssignment(assignmentRef, assignmentDigest)
	workBuilder = workBuilder.ExecutedWithin(systemRef)
	descriptionContext := description.BoundedContextRef()
	workBuilder = workBuilder.InContext(descriptionContext)
	workBuilder = workBuilder.During(workInterval, basisWindow)
	basisRef := basis.Ref()
	workBuilder = workBuilder.WithObservedProjectBasis(basisRef, basisDigest)
	basisRefValue := basisRef.String()
	workBuilder = workBuilder.WithInputs([]WorkInputRef{{
		v1Reference: v1Reference{value: basisRefValue},
	}})
	workBuilder = workBuilder.WithOutputs([]WorkOutputRef{{
		v1Reference: v1Reference{value: "output:profile-candidate:1"},
	}})
	workBuilder = workBuilder.WithResources([]WorkResourceRef{{
		v1Reference: v1Reference{value: "resource:profile-onboarding:1"},
	}})
	affectedRefKind := description.AffectedRefKind()
	workBuilder = workBuilder.AffectingKind(affectedRefKind)
	workBuilder = workBuilder.Affecting([]AffectedReferentRef{{
		v1Reference: v1Reference{value: "episteme:profile-classification:1"},
	}})
	statePlaneRef := description.StatePlaneRef()
	workBuilder = workBuilder.OnStatePlane(statePlaneRef, transition)
	workBuilder = workBuilder.WithOutcome(outcome)
	record, err := workBuilder.Build()
	if err != nil {
		t.Fatalf("ProfileOnboardingWorkRecordBuilder.Build: %v", err)
	}
	return exactWorkSupportFixtureV1{
		record:            record,
		description:       description,
		contract:          contract,
		assignment:        assignment,
		assignmentSupport: assignmentSupport,
		basis:             basis,
	}
}

func exactObservedProjectBasisForWorkV1(
	t *testing.T,
	window BasisObservationWindowV1,
) ObservedProjectBasisV1 {
	t.Helper()
	signal, err := NewObservedProjectSignalV1(
		ObservedProjectSignalKindV1{value: "repository-shape"},
		ObservedProjectSignalValueV1{value: "software"},
		SourceCarrierRefV1{v1Reference: v1Reference{value: "carrier:repository-tree:1"}},
		[]EvidenceProvenancePathRefV1{{
			v1Reference: v1Reference{value: "evidence-path:repository-tree:1"},
		}},
	)
	if err != nil {
		t.Fatalf("NewObservedProjectSignalV1: %v", err)
	}
	basis, err := NewObservedProjectBasisV1(
		ObservedProjectBasisRefV1{v1Reference: v1Reference{value: "observed-project-basis:1"}},
		ProjectRootV1{value: "/tmp/haft-project"},
		window,
		[]ObservedProjectSignalV1{signal},
		ObservedProjectDetectorVersionV1{value: "detector-v1"},
		ClassifierVersion{value: "h-onboard/v9"},
	)
	if err != nil {
		t.Fatalf("NewObservedProjectBasisV1: %v", err)
	}
	return basis
}

func exactWorkSupportBindingsV1(t *testing.T, sessionRef string) MethodParameterBindings {
	t.Helper()
	values := []struct {
		name  string
		value string
	}{
		{name: profileOnboardingClassifierParameterV1, value: "h-onboard/v9"},
		{name: profileOnboardingPolicyParameterV1, value: "profile-policy/v1"},
		{name: profileOnboardingProjectRootParameterV1, value: "/tmp/haft-project"},
		{name: profileOnboardingSessionParameterV1, value: sessionRef},
	}
	bindings, err := mapSliceV1(values, func(_ int, value struct {
		name  string
		value string
	}) (MethodParameterBinding, error) {
		return NewMethodParameterBinding(value.name, value.value)
	})
	if err != nil {
		t.Fatalf("NewMethodParameterBinding: %v", err)
	}
	result, err := NewMethodParameterBindings(bindings)
	if err != nil {
		t.Fatalf("NewMethodParameterBindings: %v", err)
	}
	return result
}

func workSupportDigestV1(t testing.TB, seed string) ContentDigest {
	t.Helper()
	seedBytes := []byte(seed)
	sum := sha256.Sum256(seedBytes)
	raw := "sha256:" + hex.EncodeToString(sum[:])
	digest, err := NewContentDigest(raw)
	if err != nil {
		t.Fatalf("NewContentDigest: %v", err)
	}
	return digest
}
