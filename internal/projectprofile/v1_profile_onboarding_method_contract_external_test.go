package projectprofile_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

type foreignProfileOnboardingMethodDescriptionV1 struct {
	projectprofile.ProfileOnboardingMethodDescriptionV1
}

type foreignProfileOnboardingMethodContractV1 struct {
	projectprofile.ProfileOnboardingMethodContractV1
}

type foreignHolderEqualsExecutedWithinV1 struct {
	projectprofile.HolderEqualsExecutedWithinV1
}

func TestProfileOnboardingMethodDescriptionV1PinsExactLocalEpisteme(t *testing.T) {
	description := projectprofile.ProfileOnboardingMethodDescriptionV1Value()

	if description.FPFKindName() != "U.MethodDescription" {
		t.Fatalf("MethodDescription kind = %q, want U.MethodDescription", description.FPFKindName())
	}
	if description.Ref() != projectprofile.ProfileOnboardingMethodDescriptionRefV1() {
		t.Fatal("MethodDescription ref is not the existing ProfileOnboarding ref")
	}
	if description.DescribedMethodRef() != projectprofile.ProfileOnboardingMethodRefV1() {
		t.Fatal("MethodDescription does not describe the existing ProfileOnboarding Method")
	}
	if description.BoundedContextRef() != projectprofile.ProfileOnboardingBoundedContextRefV1() {
		t.Fatal("MethodDescription has an unexpected bounded context")
	}
	if description.FPFSourceRevision().String() != "44dd88188a07646ef23aca32627a3f670525853f" {
		t.Fatal("MethodDescription does not pin the designated FPF source revision")
	}
	if description.Edition() != "v1" {
		t.Fatalf("MethodDescription edition = %q, want v1", description.Edition())
	}

	wantParameters := []string{
		"classifier_version",
		"policy_version",
		"project_root",
		"session_ref",
	}
	wantParameterKinds := []string{
		"ClassifierVersion",
		"PolicyVersion",
		"ProjectRootV1",
		"SessionRef",
	}
	parameters := description.ParameterDeclarations()
	if len(parameters) != len(wantParameters) {
		t.Fatalf("parameter count = %d, want %d", len(parameters), len(wantParameters))
	}
	for index, want := range wantParameters {
		if parameters[index].Name() != want {
			t.Fatalf("parameter %d = %q, want %q", index, parameters[index].Name(), want)
		}
		if parameters[index].BindingLocus().String() != "work.parameter_bindings" {
			t.Fatalf("parameter %q has unexpected binding locus", want)
		}
		if parameters[index].ValueKind().String() != wantParameterKinds[index] {
			t.Fatalf(
				"parameter %q kind = %q, want %q",
				want,
				parameters[index].ValueKind().String(),
				wantParameterKinds[index],
			)
		}
		if !parameters[index].Required() {
			t.Fatalf("parameter %q is not required", want)
		}
	}

	inputs := description.AcceptedInputKinds()
	if len(inputs) != 1 || inputs[0].String() != "ObservedProjectBasisV1" {
		t.Fatalf("accepted inputs = %#v, want ObservedProjectBasisV1", inputKindStrings(inputs))
	}
	wantResults := []string{"CandidatePayloadProduced", "ClassificationUnderdetermined"}
	if got := resultKindStrings(description.AcceptedResultKinds()); !equalStrings(got, wantResults) {
		t.Fatalf("accepted results = %#v, want %#v", got, wantResults)
	}
	if description.AffectedRefKind().String() != "ProfileClassificationEpistemeV1" {
		t.Fatal("MethodDescription has an unexpected affected-ref kind")
	}
	if description.StatePlaneRef().String() != "haft:state-plane:project-profile-classification/v1" {
		t.Fatal("MethodDescription has an unexpected StatePlane ref")
	}
	witnessRequirement := description.EffectWitnessRequirement().Statement()
	if !strings.Contains(witnessRequirement, "pre-state plus post-state") ||
		!strings.Contains(witnessRequirement, "pre-state plus delta-predicate") {
		t.Fatal("MethodDescription does not state the exact pre/post-or-delta requirement")
	}
	if description.RequiredRoleRef() != projectprofile.ProfileAuthorRoleRefV1() {
		t.Fatal("MethodDescription does not require ProfileAuthorRole v1")
	}
	if description.RequiredSystemKind().String() != "U.System" {
		t.Fatal("MethodDescription does not require a U.System holder/executor kind")
	}
	if description.SuccessCriterion().Statement() == "" ||
		description.FailureStopPolicy().Statement() == "" ||
		description.AcceptanceCriterion().Statement() == "" {
		t.Fatal("MethodDescription criteria must be explicit")
	}
}

func TestProfileOnboardingMethodContractV1BindsDescriptionAndOccurrenceRules(t *testing.T) {
	description := projectprofile.ProfileOnboardingMethodDescriptionV1Value()
	descriptionDigest, err := projectprofile.DigestProfileOnboardingMethodDescriptionV1(description)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingMethodDescriptionV1: %v", err)
	}
	contract, err := projectprofile.ProfileOnboardingMethodContractV1Value()
	if err != nil {
		t.Fatalf("ProfileOnboardingMethodContractV1Value: %v", err)
	}
	if contract.MethodDescriptionRef() != description.Ref() {
		t.Fatal("MethodContract points to another MethodDescription ref")
	}
	if contract.MethodDescriptionDigest() != descriptionDigest {
		t.Fatal("MethodContract does not bind the exact MethodDescription digest")
	}
	if contract.BoundedContextRef() != description.BoundedContextRef() {
		t.Fatal("MethodContract and MethodDescription contexts differ")
	}
	parameterSpecDigest := contract.ParameterSpecSetDigest().String()
	if len(parameterSpecDigest) != 71 || !strings.HasPrefix(parameterSpecDigest, "sha256:") {
		t.Fatal("parameter-spec set digest is not canonical")
	}
	wantSlots := []string{"work_interval", "basis_observation_window"}
	if got := occurrenceSlotStrings(contract.RequiredOccurrenceSlots()); !equalStrings(got, wantSlots) {
		t.Fatalf("required occurrence slots = %#v, want %#v", got, wantSlots)
	}
	wantCoverageRules := []string{
		"haft:rule:profile-onboarding/role-assignment-covers-work/v1",
		"haft:rule:profile-onboarding/authority-covers-work/v1",
		"haft:rule:profile-onboarding/authority-covers-basis-observation/v1",
	}
	if got := occurrenceRuleStrings(contract.OccurrenceCoverageRuleRefs()); !equalStrings(got, wantCoverageRules) {
		t.Fatalf("occurrence coverage rules = %#v, want %#v", got, wantCoverageRules)
	}
	if contract.RoleAdmissionPolicyRef().String() == contract.SystemAdmissionPolicyRef().String() {
		t.Fatal("role and system admission policies collapsed into one ref")
	}
	if contract.AcceptanceStandardRef().String() != "haft:acceptance-standard:profile-onboarding/v1" ||
		contract.AcceptanceStandardEdition() != "v1" {
		t.Fatal("MethodContract does not pin its acceptance standard and edition")
	}
	if contract.HolderEqualsExecutedWithinRule().Ref().String() != "haft:rule:profile-onboarding/holder-equals-executed-within/v1" {
		t.Fatal("MethodContract does not bind HolderEqualsExecutedWithinV1")
	}
}

func TestProfileOnboardingMethodCarriersAreDeterministicAndDistinct(t *testing.T) {
	description := projectprofile.ProfileOnboardingMethodDescriptionV1Value()
	firstDescriptionJSON, err := projectprofile.EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON(description)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON(first): %v", err)
	}
	secondDescriptionJSON, err := projectprofile.EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON(
		projectprofile.ProfileOnboardingMethodDescriptionV1Value(),
	)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON(second): %v", err)
	}
	if !bytes.Equal(firstDescriptionJSON, secondDescriptionJSON) {
		t.Fatal("MethodDescription canonical JSON is not deterministic")
	}
	descriptionCarrier, err := projectprofile.CarryProfileOnboardingMethodDescriptionV1(description)
	if err != nil {
		t.Fatalf("CarryProfileOnboardingMethodDescriptionV1: %v", err)
	}
	if _, isDescription := any(descriptionCarrier).(projectprofile.ProfileOnboardingMethodDescriptionV1); isDescription {
		t.Fatal("MethodDescription JSON carrier also implements the episteme")
	}
	if _, isDescription := any(description.DescribedMethodRef()).(projectprofile.ProfileOnboardingMethodDescriptionV1); isDescription {
		t.Fatal("Method ref also implements the MethodDescription episteme")
	}
	mutatedCarrierBytes := descriptionCarrier.CanonicalJSON()
	mutatedCarrierBytes[0] = '['
	if bytes.Equal(mutatedCarrierBytes, descriptionCarrier.CanonicalJSON()) {
		t.Fatal("MethodDescription carrier leaked mutable canonical bytes")
	}

	contract, err := projectprofile.ProfileOnboardingMethodContractV1Value()
	if err != nil {
		t.Fatalf("ProfileOnboardingMethodContractV1Value: %v", err)
	}
	firstContractJSON, err := projectprofile.EncodeProfileOnboardingMethodContractV1CanonicalJSON(contract)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingMethodContractV1CanonicalJSON(first): %v", err)
	}
	secondContract, err := projectprofile.ProfileOnboardingMethodContractV1Value()
	if err != nil {
		t.Fatalf("ProfileOnboardingMethodContractV1Value(second): %v", err)
	}
	secondContractJSON, err := projectprofile.EncodeProfileOnboardingMethodContractV1CanonicalJSON(secondContract)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingMethodContractV1CanonicalJSON(second): %v", err)
	}
	if !bytes.Equal(firstContractJSON, secondContractJSON) {
		t.Fatal("MethodContract canonical JSON is not deterministic")
	}
	contractCarrier, err := projectprofile.CarryProfileOnboardingMethodContractV1(contract)
	if err != nil {
		t.Fatalf("CarryProfileOnboardingMethodContractV1: %v", err)
	}
	if _, isContract := any(contractCarrier).(projectprofile.ProfileOnboardingMethodContractV1); isContract {
		t.Fatal("MethodContract JSON carrier also implements the contract")
	}
	if descriptionCarrier.ContentDigest() != contract.MethodDescriptionDigest() {
		t.Fatal("MethodContract does not bind the exact MethodDescription carrier digest")
	}
	contractDigest, err := projectprofile.DigestProfileOnboardingMethodContractV1(contract)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingMethodContractV1: %v", err)
	}
	if contractCarrier.ContentDigest() != contractDigest {
		t.Fatal("MethodContract carrier digest differs from semantic digest")
	}
}

func TestProfileOnboardingMethodAlgebraRejectsForeignEmbeddingAndTamper(t *testing.T) {
	description := projectprofile.ProfileOnboardingMethodDescriptionV1Value()
	foreignDescription := foreignProfileOnboardingMethodDescriptionV1{
		ProfileOnboardingMethodDescriptionV1: description,
	}
	if _, err := projectprofile.EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON(foreignDescription); err == nil {
		t.Fatal("MethodDescription encoder accepted a foreign embedded implementation")
	}
	if _, err := projectprofile.DigestProfileOnboardingMethodDescriptionV1(foreignDescription); err == nil {
		t.Fatal("MethodDescription digest accepted a foreign embedded implementation")
	}

	contract, err := projectprofile.ProfileOnboardingMethodContractV1Value()
	if err != nil {
		t.Fatalf("ProfileOnboardingMethodContractV1Value: %v", err)
	}
	foreignContract := foreignProfileOnboardingMethodContractV1{
		ProfileOnboardingMethodContractV1: contract,
	}
	if _, err := projectprofile.EncodeProfileOnboardingMethodContractV1CanonicalJSON(foreignContract); err == nil {
		t.Fatal("MethodContract encoder accepted a foreign embedded implementation")
	}
	if _, err := projectprofile.DigestProfileOnboardingMethodContractV1(foreignContract); err == nil {
		t.Fatal("MethodContract digest accepted a foreign embedded implementation")
	}

	descriptionJSON, err := projectprofile.EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON(description)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON: %v", err)
	}
	wrongRevision := bytes.Replace(
		descriptionJSON,
		[]byte("44dd88188a07646ef23aca32627a3f670525853f"),
		[]byte("54dd88188a07646ef23aca32627a3f670525853f"),
		1,
	)
	if _, err := projectprofile.DecodeProfileOnboardingMethodDescriptionV1CanonicalJSON(wrongRevision); err == nil {
		t.Fatal("MethodDescription decoder accepted a different source revision")
	}
	if _, err := projectprofile.DecodeProfileOnboardingMethodDescriptionV1CanonicalJSON(append([]byte(" "), descriptionJSON...)); err == nil {
		t.Fatal("MethodDescription decoder accepted non-canonical whitespace")
	}
	unknownField := append([]byte{}, descriptionJSON[:len(descriptionJSON)-1]...)
	unknownField = append(unknownField, []byte(`,"unknown":true}`)...)
	if _, err := projectprofile.DecodeProfileOnboardingMethodDescriptionV1CanonicalJSON(unknownField); err == nil {
		t.Fatal("MethodDescription decoder accepted an unknown field")
	}

	contractJSON, err := projectprofile.EncodeProfileOnboardingMethodContractV1CanonicalJSON(contract)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingMethodContractV1CanonicalJSON: %v", err)
	}
	wrongRule := bytes.Replace(
		contractJSON,
		[]byte("holder-equals-executed-within/v1"),
		[]byte("holder-equals-executed-within/v2"),
		1,
	)
	if _, err := projectprofile.DecodeProfileOnboardingMethodContractV1CanonicalJSON(wrongRule); err == nil {
		t.Fatal("MethodContract decoder accepted a different local rule")
	}
}

func TestProfileOnboardingMethodDescriptionAndContractCarryNoPerformedOrFinalFacts(t *testing.T) {
	description := projectprofile.ProfileOnboardingMethodDescriptionV1Value()
	descriptionJSON, err := projectprofile.EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON(description)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON: %v", err)
	}
	contract, err := projectprofile.ProfileOnboardingMethodContractV1Value()
	if err != nil {
		t.Fatalf("ProfileOnboardingMethodContractV1Value: %v", err)
	}
	contractJSON, err := projectprofile.EncodeProfileOnboardingMethodContractV1CanonicalJSON(contract)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingMethodContractV1CanonicalJSON: %v", err)
	}
	joined := string(descriptionJSON) + string(contractJSON)
	forbidden := []string{
		`"work_ref"`,
		`"performed_by_role_assignment_ref"`,
		`"executed_within_system_ref"`,
		`"parameter_bindings"`,
		`"recorded_at"`,
		`"committed_ledger_revision"`,
		`"admission_record_ref"`,
		`"receipt"`,
		`"permission_ref"`,
		`"result_ref"`,
		`"result_digest"`,
	}
	for _, key := range forbidden {
		if strings.Contains(joined, key) {
			t.Fatalf("description/contract smuggles performed or final fact key %s", key)
		}
	}
}

func TestHolderEqualsExecutedWithinV1IsExactLocalRule(t *testing.T) {
	from := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	until := from.Add(2 * time.Hour)
	window, err := projectprofile.NewRoleAssignmentWindowV1(from, until)
	if err != nil {
		t.Fatalf("NewRoleAssignmentWindowV1: %v", err)
	}
	assignmentRef, err := projectprofile.NewRoleAssignmentRef("role-assignment:profile-author:test")
	if err != nil {
		t.Fatalf("NewRoleAssignmentRef: %v", err)
	}
	holder, err := projectprofile.NewSystemRef("system:host-agent:test")
	if err != nil {
		t.Fatalf("NewSystemRef(holder): %v", err)
	}
	systemAdmissionRef, err := projectprofile.NewSystemAdmissionRef("system-admission:profile-agent:test")
	if err != nil {
		t.Fatalf("NewSystemAdmissionRef: %v", err)
	}
	roleAdmissionRef, err := projectprofile.NewRoleAdmissionRef("role-admission:profile-author:test")
	if err != nil {
		t.Fatalf("NewRoleAdmissionRef: %v", err)
	}
	justificationRef, err := projectprofile.NewRoleAssignmentJustificationRef("role-assignment-justification:test")
	if err != nil {
		t.Fatalf("NewRoleAssignmentJustificationRef: %v", err)
	}
	provenanceRef, err := projectprofile.NewRoleAssignmentProvenanceRef("role-assignment-provenance:test")
	if err != nil {
		t.Fatalf("NewRoleAssignmentProvenanceRef: %v", err)
	}
	builder := projectprofile.NewProfileAuthorRoleAssignmentV1Builder(assignmentRef)
	builder = builder.HeldBy(holder)
	builder = builder.Assigning(projectprofile.ProfileAuthorRoleRefV1())
	builder = builder.InContext(projectprofile.ProfileOnboardingBoundedContextRefV1())
	builder = builder.ValidDuring(window)
	builder = builder.WithSystemAdmission(
		systemAdmissionRef,
		methodContractDigestOf(t, "system-admission"),
	)
	builder = builder.WithRoleAdmission(
		roleAdmissionRef,
		methodContractDigestOf(t, "role-admission"),
	)
	builder = builder.JustifiedBy(
		justificationRef,
		methodContractDigestOf(t, "justification"),
	)
	builder = builder.WithProvenance(
		provenanceRef,
		methodContractDigestOf(t, "provenance"),
	)
	assignment, err := builder.Build()
	if err != nil {
		t.Fatalf("ProfileAuthorRoleAssignmentV1Builder.Build: %v", err)
	}
	rule := projectprofile.ProfileOnboardingHolderEqualsExecutedWithinV1()
	if err := projectprofile.ValidateHolderEqualsExecutedWithinV1(rule, assignment, holder); err != nil {
		t.Fatalf("matching holder/executedWithin rejected: %v", err)
	}
	otherSystem, err := projectprofile.NewSystemRef("system:other:test")
	if err != nil {
		t.Fatalf("NewSystemRef(other): %v", err)
	}
	if err := projectprofile.ValidateHolderEqualsExecutedWithinV1(rule, assignment, otherSystem); err == nil {
		t.Fatal("local rule accepted a holder/executedWithin mismatch")
	}
	foreignRule := foreignHolderEqualsExecutedWithinV1{HolderEqualsExecutedWithinV1: rule}
	if err := projectprofile.ValidateHolderEqualsExecutedWithinV1(foreignRule, assignment, holder); err == nil {
		t.Fatal("local rule validator accepted a foreign embedded implementation")
	}
}

func methodContractDigestOf(t testing.TB, seed string) projectprofile.ContentDigest {
	t.Helper()
	sum := sha256.Sum256([]byte(seed))
	raw := "sha256:" + hex.EncodeToString(sum[:])
	digest, err := projectprofile.NewContentDigest(raw)
	if err != nil {
		t.Fatalf("NewContentDigest: %v", err)
	}
	return digest
}

func inputKindStrings(values []projectprofile.ProfileOnboardingInputKindV1) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}

func resultKindStrings(values []projectprofile.ProfileOnboardingResultKindV1) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}

func occurrenceSlotStrings(values []projectprofile.ProfileOnboardingOccurrenceSlotV1) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}

func occurrenceRuleStrings(values []projectprofile.ProfileOnboardingOccurrenceCoverageRuleRefV1) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}

func equalStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
