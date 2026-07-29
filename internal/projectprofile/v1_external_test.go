package projectprofile_test

import (
	"bytes"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

type v1Fixture struct {
	payload           projectprofile.ProfileDeclarationPayload
	record            projectprofile.ProfileOnboardingWorkRecord
	description       projectprofile.ProfileOnboardingMethodDescriptionV1
	contract          projectprofile.ProfileOnboardingMethodContractV1
	assignment        projectprofile.ProfileAuthorRoleAssignmentV1
	assignmentSupport projectprofile.ProfileAuthorAssignmentSupportCarrierV1
	basis             projectprofile.ObservedProjectBasisV1
	effect            projectprofile.ProfileOnboardingEffectV1
	assessment        projectprofile.ProfileOnboardingOutcomeAssessmentV1
	candidate         projectprofile.ProfileDeclarationCandidateV1
	inputs            projectprofile.ProfileDeclarationAdmissionInputs
	commitPlan        projectprofile.ProfileDeclarationCommitPlan
}

func TestFinalV1PayloadDigestContainsScopesOnlyAndCanonicalizesSetOrder(t *testing.T) {
	software := mustSoftwareScope(t, "software-cli")
	nonSoftware := mustNonSoftwareScope(t, "knowledge-model")
	leftScopes := mustScopeSet(t, []projectprofile.RealizationScope{software, nonSoftware})
	rightScopes := mustScopeSet(t, []projectprofile.RealizationScope{nonSoftware, software})
	left, err := projectprofile.NewProfileDeclarationPayload(leftScopes)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload(left): %v", err)
	}
	right, err := projectprofile.NewProfileDeclarationPayload(rightScopes)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload(right): %v", err)
	}
	leftDigest, err := projectprofile.DigestProfileDeclarationPayload(left)
	if err != nil {
		t.Fatalf("DigestProfileDeclarationPayload(left): %v", err)
	}
	rightDigest, err := projectprofile.DigestProfileDeclarationPayload(right)
	if err != nil {
		t.Fatalf("DigestProfileDeclarationPayload(right): %v", err)
	}
	if leftDigest != rightDigest {
		t.Fatalf("scope-set order changed payload digest: %s != %s", leftDigest.String(), rightDigest.String())
	}
	payloadType := reflect.TypeOf(left)
	if payloadType.NumField() != 1 || payloadType.Field(0).Name != "scopes" {
		t.Fatalf("payload contains non-scope state: %#v", payloadType)
	}

	data, err := projectprofile.EncodeProfileDeclarationPayloadCanonicalJSON(left)
	if err != nil {
		t.Fatalf("EncodeProfileDeclarationPayloadCanonicalJSON: %v", err)
	}
	decoded, err := projectprofile.DecodeProfileDeclarationPayloadCanonicalJSON(data)
	if err != nil {
		t.Fatalf("DecodeProfileDeclarationPayloadCanonicalJSON: %v", err)
	}
	decodedDigest, err := projectprofile.DigestProfileDeclarationPayload(decoded)
	if err != nil {
		t.Fatalf("DigestProfileDeclarationPayload(decoded): %v", err)
	}
	if decodedDigest != leftDigest {
		t.Fatalf("payload digest changed after canonical JSON roundtrip")
	}
}

func TestFinalV1KindOrientationPreservesLegacyWireBytesAndDigest(t *testing.T) {
	payload := mustKindOrientationPayloadV1(t)
	wantJSON := []byte(`{"schema":"haft.project-profile.declaration-payload/v1","scopes":[{"kind":"non_software","scope_id":"knowledge-model","entity_reference":{"kind":"referenced","ref":"entity:knowledge-model"},"kind_admission":{"kind":"admitted","ref":"U.Episteme"},"governing_pattern_refs":["A.7","C.28"],"contract_refs":["TS.knowledge.001"]}]}`)

	encoded, err := projectprofile.EncodeProfileDeclarationPayloadCanonicalJSON(payload)
	if err != nil {
		t.Fatalf("EncodeProfileDeclarationPayloadCanonicalJSON: %v", err)
	}
	if !bytes.Equal(encoded, wantJSON) {
		t.Fatalf("canonical v1 JSON changed\n--- got ---\n%s\n--- want ---\n%s", encoded, wantJSON)
	}

	decoded, err := projectprofile.DecodeProfileDeclarationPayloadCanonicalJSON(wantJSON)
	if err != nil {
		t.Fatalf("DecodeProfileDeclarationPayloadCanonicalJSON: %v", err)
	}
	scopes := decoded.Scopes().Values()
	if len(scopes) != 1 {
		t.Fatalf("decoded scopes = %d, want 1", len(scopes))
	}
	nonSoftware, ok := scopes[0].(projectprofile.NonSoftwareRealization)
	if !ok {
		t.Fatalf("decoded scope = %T, want NonSoftwareRealization", scopes[0])
	}
	orientation, ok := nonSoftware.KindOrientation().(projectprofile.ReferencedKindOrientation)
	if !ok {
		t.Fatalf("decoded orientation = %T, want ReferencedKindOrientation", nonSoftware.KindOrientation())
	}
	if orientation.Ref().String() != "U.Episteme" {
		t.Fatalf("decoded orientation ref = %q", orientation.Ref().String())
	}

	roundTrip, err := projectprofile.EncodeProfileDeclarationPayloadCanonicalJSON(decoded)
	if err != nil {
		t.Fatalf("re-encode profile payload: %v", err)
	}
	if !bytes.Equal(roundTrip, wantJSON) {
		t.Fatalf("legacy v1 JSON changed after roundtrip\n--- got ---\n%s\n--- want ---\n%s", roundTrip, wantJSON)
	}
	digest, err := projectprofile.DigestProfileDeclarationPayload(decoded)
	if err != nil {
		t.Fatalf("DigestProfileDeclarationPayload: %v", err)
	}
	wantDigest := "sha256:2280af70c20c8313eea9f669dce50eb7cd3b8aa89bcb6e65c3384e06129c25a5"
	if digest.String() != wantDigest {
		t.Fatalf("legacy v1 digest = %q, want %q", digest.String(), wantDigest)
	}
}

func TestFinalV1UnspecifiedKindOrientationPreservesLegacyNoneBytesAndDigest(t *testing.T) {
	scopes := mustScopeSet(
		t,
		[]projectprofile.RealizationScope{mustNonSoftwareScope(t, "unclassified-documents")},
	)
	payload, err := projectprofile.NewProfileDeclarationPayload(scopes)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload: %v", err)
	}
	wantJSON := []byte(`{"schema":"haft.project-profile.declaration-payload/v1","scopes":[{"kind":"non_software","scope_id":"unclassified-documents","entity_reference":{"kind":"none"},"kind_admission":{"kind":"none"},"governing_pattern_refs":[],"contract_refs":[]}]}`)
	encoded, err := projectprofile.EncodeProfileDeclarationPayloadCanonicalJSON(payload)
	if err != nil {
		t.Fatalf("EncodeProfileDeclarationPayloadCanonicalJSON: %v", err)
	}
	if !bytes.Equal(encoded, wantJSON) {
		t.Fatalf("canonical v1 none JSON changed\n--- got ---\n%s\n--- want ---\n%s", encoded, wantJSON)
	}
	decoded, err := projectprofile.DecodeProfileDeclarationPayloadCanonicalJSON(wantJSON)
	if err != nil {
		t.Fatalf("DecodeProfileDeclarationPayloadCanonicalJSON: %v", err)
	}
	nonSoftware, ok := decoded.Scopes().Values()[0].(projectprofile.NonSoftwareRealization)
	if !ok {
		t.Fatalf("decoded scope = %T, want NonSoftwareRealization", decoded.Scopes().Values()[0])
	}
	if _, ok := nonSoftware.KindOrientation().(projectprofile.UnspecifiedKindOrientation); !ok {
		t.Fatalf("decoded orientation = %T, want UnspecifiedKindOrientation", nonSoftware.KindOrientation())
	}
	digest, err := projectprofile.DigestProfileDeclarationPayload(decoded)
	if err != nil {
		t.Fatalf("DigestProfileDeclarationPayload: %v", err)
	}
	wantDigest := "sha256:d45bd224ba4c16302047b80d3f35152b2c9a2c014ce945223044dd841689ab93"
	if digest.String() != wantDigest {
		t.Fatalf("legacy v1 none digest = %q, want %q", digest.String(), wantDigest)
	}
}

func mustKindOrientationPayloadV1(t *testing.T) projectprofile.ProfileDeclarationPayload {
	t.Helper()
	scopeID, err := projectprofile.NewScopeID("knowledge-model")
	if err != nil {
		t.Fatalf("NewScopeID: %v", err)
	}
	entityRef, err := projectprofile.NewEntityRef("entity:knowledge-model")
	if err != nil {
		t.Fatalf("NewEntityRef: %v", err)
	}
	kindRef, err := projectprofile.NewKindRef("U.Episteme")
	if err != nil {
		t.Fatalf("NewKindRef: %v", err)
	}
	patternC28, err := projectprofile.NewSourceUnitRef("C.28")
	if err != nil {
		t.Fatalf("NewSourceUnitRef(C.28): %v", err)
	}
	patternA7, err := projectprofile.NewSourceUnitRef("A.7")
	if err != nil {
		t.Fatalf("NewSourceUnitRef(A.7): %v", err)
	}
	contractRef, err := projectprofile.NewSpecSectionRef("TS.knowledge.001")
	if err != nil {
		t.Fatalf("NewSpecSectionRef: %v", err)
	}
	scope, err := projectprofile.NewNonSoftwareRealization(
		scopeID,
		projectprofile.NewReferencedEntity(entityRef),
		projectprofile.NewReferencedKindOrientation(kindRef),
		[]projectprofile.SourceUnitRef{patternC28, patternA7},
		[]projectprofile.SpecSectionRef{contractRef},
	)
	if err != nil {
		t.Fatalf("NewNonSoftwareRealization: %v", err)
	}
	scopeSet, err := projectprofile.NewScopeSet([]projectprofile.RealizationScope{scope})
	if err != nil {
		t.Fatalf("NewScopeSet: %v", err)
	}
	payload, err := projectprofile.NewProfileDeclarationPayload(scopeSet)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload: %v", err)
	}
	return payload
}

func TestFinalV1WorkRecordIsCompleteCycleFreeAndExactlySupported(t *testing.T) {
	fixture := mustV1Fixture(t)
	err := projectprofile.ValidateProfileOnboardingWorkRecordAgainstSupportV1(
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
	workType := reflect.TypeOf(fixture.record)
	forbiddenWorkFields := []string{"candidate", "provenance", "receipt", "admission"}
	hasCycleFormingField := slices.ContainsFunc(forbiddenWorkFields, func(fragment string) bool {
		return reflectedTypeContainsV1(workType, fragment, 0)
	})
	if hasCycleFormingField {
		t.Fatal("Work record has a cycle-forming field")
	}

	data, err := projectprofile.EncodeProfileOnboardingWorkRecordCanonicalJSON(fixture.record)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingWorkRecordCanonicalJSON: %v", err)
	}
	decoded, err := projectprofile.DecodeProfileOnboardingWorkRecordCanonicalJSON(data)
	if err != nil {
		t.Fatalf("DecodeProfileOnboardingWorkRecordCanonicalJSON: %v", err)
	}
	leftDigest, err := projectprofile.DigestProfileOnboardingWorkRecord(fixture.record)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingWorkRecord: %v", err)
	}
	rightDigest, err := projectprofile.DigestProfileOnboardingWorkRecord(decoded)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingWorkRecord(decoded): %v", err)
	}
	if leftDigest != rightDigest {
		t.Fatal("Work-record digest changed after canonical JSON roundtrip")
	}
}

func TestFinalV1WorkRejectsWrongAnchorsMissingBindingsAndInvalidState(t *testing.T) {
	fixture := mustV1Fixture(t)
	wrongMethod, err := projectprofile.NewMethodRef("method:wrong")
	if err != nil {
		t.Fatalf("NewMethodRef: %v", err)
	}
	builder := cloneV1WorkBuilder(t, fixture)
	builder = builder.Enacts(
		wrongMethod,
		projectprofile.ProfileOnboardingMethodDescriptionRefV1(),
		fixture.record.ParameterBindings(),
	)
	_, err = builder.Build()
	if err == nil {
		t.Fatal("Work accepted a non-onboarding Method anchor")
	}

	emptyBindings := projectprofile.MethodParameterBindings{}
	builder = cloneV1WorkBuilder(t, fixture)
	builder = builder.Enacts(
		projectprofile.ProfileOnboardingMethodRefV1(),
		projectprofile.ProfileOnboardingMethodDescriptionRefV1(),
		emptyBindings,
	)
	_, err = builder.Build()
	if err == nil {
		t.Fatal("Work accepted absent concrete parameter bindings")
	}

	builder = cloneV1WorkBuilder(t, fixture)
	builder = builder.WithOutputs(nil)
	_, err = builder.Build()
	if err == nil {
		t.Fatal("Work accepted absent explicit outputs")
	}

	builder = cloneV1WorkBuilder(t, fixture)
	builder = builder.OnStatePlane(fixture.record.StatePlaneRef(), nil)
	_, err = builder.Build()
	if err == nil {
		t.Fatal("Work accepted absent state transition")
	}

	preState := fixture.record.StateTransition().PreStateRef()
	equalTransition, err := projectprofile.NewPrePostStateTransitionV1(preState, preState)
	if err == nil || equalTransition != nil {
		t.Fatal("pre/post transition accepted an unflagged no-op occurrence")
	}
}

func TestFinalV1CandidateBindsExactWorkWithoutDuplicatingWorkFields(t *testing.T) {
	fixture := mustV1Fixture(t)
	err := projectprofile.ValidateProfileDeclarationCandidateV1AgainstSupports(
		fixture.candidate,
		fixture.record,
		fixture.description,
		fixture.contract,
		fixture.assignment,
		fixture.assignmentSupport,
		fixture.basis,
		fixture.effect,
		fixture.assessment,
	)
	if err != nil {
		t.Fatalf("ValidateProfileDeclarationCandidateV1AgainstSupports: %v", err)
	}
	provenanceType := reflect.TypeOf(fixture.candidate.Provenance())
	forbiddenProvenanceFields := []string{"executed", "window", "interval", "receipt", "revision"}
	duplicatesWorkOrAdmission := slices.ContainsFunc(forbiddenProvenanceFields, func(fragment string) bool {
		return reflectedTypeContainsV1(provenanceType, fragment, 0)
	})
	if duplicatesWorkOrAdmission {
		t.Fatal("CandidateProvenanceV1 duplicates a Work/admission field")
	}

	data, err := projectprofile.EncodeProfileDeclarationCandidateV1CanonicalJSON(fixture.candidate)
	if err != nil {
		t.Fatalf("EncodeProfileDeclarationCandidateV1CanonicalJSON: %v", err)
	}
	decoded, err := projectprofile.DecodeProfileDeclarationCandidateV1CanonicalJSON(data)
	if err != nil {
		t.Fatalf("DecodeProfileDeclarationCandidateV1CanonicalJSON: %v", err)
	}
	if decoded.Provenance().Digest() != fixture.candidate.Provenance().Digest() {
		t.Fatal("candidate provenance digest changed after canonical JSON roundtrip")
	}

	assertCandidateWorkParameterMismatchV1(t, fixture, "classifier_version", "h-onboard/other")
	assertCandidateWorkParameterMismatchV1(t, fixture, "policy_version", "profile-policy/other")
	assertCandidateWorkParameterMismatchV1(t, fixture, "project_root", "/tmp/another-project")
	assertCandidateWorkParameterMismatchV1(t, fixture, "session_ref", "session:other")
}

func TestFinalV1UnderdeterminedClassificationBindsMissingBasisToWorkOutcome(t *testing.T) {
	fixture := mustV1Fixture(t)
	missing, err := projectprofile.NewMissingProfileBasisSetV1(
		[]projectprofile.MissingProfileBasis{
			projectprofile.MissingClassificationBasis,
			projectprofile.MissingObservedProjectBasis,
		},
	)
	if err != nil {
		t.Fatalf("NewMissingProfileBasisSetV1: %v", err)
	}
	missingDigest, err := projectprofile.DigestMissingProfileBasisSetV1(missing)
	if err != nil {
		t.Fatalf("DigestMissingProfileBasisSetV1: %v", err)
	}
	outcome, err := projectprofile.NewClassificationUnderdetermined(missingDigest)
	if err != nil {
		t.Fatalf("NewClassificationUnderdetermined: %v", err)
	}
	builder := cloneV1WorkBuilder(t, fixture)
	builder = builder.WithOutcome(outcome)
	record, err := builder.Build()
	if err != nil {
		t.Fatalf("ProfileOnboardingWorkRecordBuilder.Build: %v", err)
	}
	err = projectprofile.ValidateProfileOnboardingWorkRecordAgainstSupportV1(
		record,
		fixture.description,
		fixture.contract,
		fixture.assignment,
		fixture.assignmentSupport,
		fixture.basis,
	)
	if err != nil {
		t.Fatalf("ValidateProfileOnboardingWorkRecordAgainstSupportV1: %v", err)
	}
	recordedOutcome, ok := record.Outcome().(projectprofile.ClassificationUnderdetermined)
	if !ok || recordedOutcome.MissingBasisDigest() != missingDigest {
		t.Fatal("Work record did not preserve ClassificationUnderdetermined outcome")
	}
	other, err := projectprofile.NewMissingProfileBasisSetV1(
		[]projectprofile.MissingProfileBasis{projectprofile.MissingStableScopeIdentity},
	)
	if err != nil {
		t.Fatalf("NewMissingProfileBasisSetV1(other): %v", err)
	}
	otherDigest, err := projectprofile.DigestMissingProfileBasisSetV1(other)
	if err != nil {
		t.Fatalf("DigestMissingProfileBasisSetV1(other): %v", err)
	}
	if otherDigest == missingDigest {
		t.Fatal("distinct missing-basis sets collapsed to one digest")
	}
}

func TestFinalV1AdmissionInputsAndCommitPlanRemainNonBinding(t *testing.T) {
	fixture := mustV1Fixture(t)
	rawInputs := any(fixture.inputs)
	_, ok := rawInputs.(projectprofile.ConfiguredProjectProfileV1)
	if ok {
		t.Fatal("pre-admission inputs implement ConfiguredProjectProfileV1")
	}
	rawPlan := any(fixture.commitPlan)
	_, ok = rawPlan.(projectprofile.ConfiguredProjectProfileV1)
	if ok {
		t.Fatal("commit plan implements ConfiguredProjectProfileV1")
	}
	_, ok = rawInputs.(projectprofile.ProfileDeclarationReceiptV1)
	if ok {
		t.Fatal("pre-admission inputs implement ProfileDeclarationReceiptV1")
	}
	_, ok = rawPlan.(projectprofile.ProfileDeclarationAdmissionRecord)
	if ok {
		t.Fatal("commit plan implements ProfileDeclarationAdmissionRecord")
	}

	inputsJSON, err := projectprofile.EncodeProfileDeclarationAdmissionInputsCanonicalJSON(fixture.inputs)
	if err != nil {
		t.Fatalf("EncodeProfileDeclarationAdmissionInputsCanonicalJSON: %v", err)
	}
	decodedInputs, err := projectprofile.DecodeProfileDeclarationAdmissionInputsCanonicalJSON(inputsJSON)
	if err != nil {
		t.Fatalf("DecodeProfileDeclarationAdmissionInputsCanonicalJSON: %v", err)
	}
	if decodedInputs.Digest() != fixture.inputs.Digest() {
		t.Fatal("admission-input digest changed after canonical JSON roundtrip")
	}

	planJSON, err := projectprofile.EncodeProfileDeclarationCommitPlanCanonicalJSON(fixture.commitPlan)
	if err != nil {
		t.Fatalf("EncodeProfileDeclarationCommitPlanCanonicalJSON: %v", err)
	}
	decodedPlan, err := projectprofile.DecodeProfileDeclarationCommitPlanCanonicalJSON(planJSON)
	if err != nil {
		t.Fatalf("DecodeProfileDeclarationCommitPlanCanonicalJSON: %v", err)
	}
	if decodedPlan.Digest() != fixture.commitPlan.Digest() {
		t.Fatal("commit-plan digest changed after canonical JSON roundtrip")
	}

	_, err = projectprofile.NewProfileDeclarationAdmissionInputs(
		fixture.candidate,
		projectprofile.NewLedgerRevision(math.MaxInt64),
	)
	if err == nil {
		t.Fatal("admission inputs accepted an exhausted expected ledger revision")
	}
}

func TestFinalV1CanonicalJSONRejectsNonCanonicalAndUnknownInput(t *testing.T) {
	fixture := mustV1Fixture(t)
	canonical, err := projectprofile.EncodeProfileOnboardingWorkRecordCanonicalJSON(fixture.record)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingWorkRecordCanonicalJSON: %v", err)
	}
	spaced := append([]byte(" "), canonical...)
	_, err = projectprofile.DecodeProfileOnboardingWorkRecordCanonicalJSON(spaced)
	if err == nil {
		t.Fatal("canonical JSON decoder accepted leading whitespace")
	}
	unknown := bytes.Replace(canonical, []byte(`"schema":`), []byte(`"unknown":1,"schema":`), 1)
	_, err = projectprofile.DecodeProfileOnboardingWorkRecordCanonicalJSON(unknown)
	if err == nil {
		t.Fatal("canonical JSON decoder accepted unknown field")
	}
	duplicate := bytes.Replace(canonical, []byte(`"schema":`), []byte(`"schema":"duplicate","schema":`), 1)
	_, err = projectprofile.DecodeProfileOnboardingWorkRecordCanonicalJSON(duplicate)
	if err == nil {
		t.Fatal("canonical JSON decoder accepted duplicate field")
	}
	withTrailing := append(append([]byte{}, canonical...), []byte(`{}`)...)
	_, err = projectprofile.DecodeProfileOnboardingWorkRecordCanonicalJSON(withTrailing)
	if err == nil {
		t.Fatal("canonical JSON decoder accepted trailing root")
	}
}

func FuzzFinalV1WorkRecordCanonicalJSONNeverPanics(f *testing.F) {
	fixture := mustV1Fixture(f)
	canonical, err := projectprofile.EncodeProfileOnboardingWorkRecordCanonicalJSON(fixture.record)
	if err != nil {
		f.Fatalf("EncodeProfileOnboardingWorkRecordCanonicalJSON: %v", err)
	}
	f.Add(canonical)
	f.Add([]byte(`null`))
	f.Add([]byte(`{"schema":"unknown"}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = projectprofile.DecodeProfileOnboardingWorkRecordCanonicalJSON(data)
	})
}

func mustV1Fixture(t testing.TB) v1Fixture {
	t.Helper()
	projectRoot, err := projectprofile.NewProjectRootV1("/tmp/haft-project")
	if err != nil {
		t.Fatalf("NewProjectRootV1: %v", err)
	}
	classifierVersion := mustClassifierVersionTB(t, "h-onboard/v9")
	policyVersion := mustPolicyVersionTB(t, "profile-policy/v1")
	scopes, err := projectprofile.NewScopeSet(
		[]projectprofile.RealizationScope{mustSoftwareScopeTB(t, "software-cli")},
	)
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
	assignmentFixture := mustProfileAuthorAssignmentSupportFixtureV1(t)
	basis := mustObservedProjectBasisForV1Fixture(t, projectRoot, classifierVersion)
	basisDigest, err := projectprofile.DigestObservedProjectBasisV1(basis)
	if err != nil {
		t.Fatalf("DigestObservedProjectBasisV1: %v", err)
	}
	description := projectprofile.ProfileOnboardingMethodDescriptionV1Value()
	contract, err := projectprofile.ProfileOnboardingMethodContractV1Value()
	if err != nil {
		t.Fatalf("ProfileOnboardingMethodContractV1Value: %v", err)
	}
	record := mustCandidateWorkRecordV1(
		t,
		payloadDigest,
		basis,
		basisDigest,
		assignmentFixture,
		projectRoot,
		classifierVersion,
		policyVersion,
	)
	workRecordDigest, err := projectprofile.DigestProfileOnboardingWorkRecord(record)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingWorkRecord: %v", err)
	}
	effect := mustProfileOnboardingEffectForV1Fixture(t, record, payloadDigest, basis, basisDigest)
	assessment := mustPassedOutcomeAssessmentForV1Fixture(t, effect, contract)
	assessmentDigest, err := projectprofile.DigestProfileOnboardingOutcomeAssessmentV1(assessment)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingOutcomeAssessmentV1: %v", err)
	}
	authorityBasisRef, err := projectprofile.NewProfileDeclarationAuthorityBasisRef("authority-basis:profile-onboarding:1")
	if err != nil {
		t.Fatalf("NewProfileDeclarationAuthorityBasisRef: %v", err)
	}
	sessionRef := assignmentFixture.sessionRef
	provenanceBuilder := projectprofile.NewCandidateProvenanceV1Builder(
		authorityBasisRef,
		record.RecordRef(),
		workRecordDigest,
	)
	assignmentDigest, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(assignmentFixture.assignment)
	if err != nil {
		t.Fatalf("DigestProfileAuthorRoleAssignmentV1: %v", err)
	}
	provenanceBuilder = provenanceBuilder.ForProfileAuthorRoleAssignment(
		assignmentFixture.assignment.RoleAssignmentRef(),
		assignmentDigest,
	)
	provenanceBuilder = provenanceBuilder.ForProject(projectRoot)
	provenanceBuilder = provenanceBuilder.ClassifiedBy(classifierVersion, policyVersion)
	provenanceBuilder = provenanceBuilder.InSession(sessionRef)
	provenanceBuilder = provenanceBuilder.ForPayload(payloadDigest)
	provenanceBuilder = provenanceBuilder.ForObservedProjectBasis(basis.Ref(), basisDigest)
	provenanceBuilder = provenanceBuilder.ForOutcomeAssessment(assessment.Ref(), assessmentDigest)
	provenance, err := provenanceBuilder.Build()
	if err != nil {
		t.Fatalf("CandidateProvenanceV1Builder.Build: %v", err)
	}
	candidate, err := projectprofile.NewProfileDeclarationCandidateV1(payload, provenance)
	if err != nil {
		t.Fatalf("NewProfileDeclarationCandidateV1: %v", err)
	}
	inputs, err := projectprofile.NewProfileDeclarationAdmissionInputs(
		candidate,
		projectprofile.NewLedgerRevision(0),
	)
	if err != nil {
		t.Fatalf("NewProfileDeclarationAdmissionInputs: %v", err)
	}
	resolutionRef, err := projectprofile.NewAuthorityResolutionRecordRef("authority-resolution:profile:1")
	if err != nil {
		t.Fatalf("NewAuthorityResolutionRecordRef: %v", err)
	}
	singleUseKey, err := projectprofile.NewSingleUseKey("single-use:profile:1")
	if err != nil {
		t.Fatalf("NewSingleUseKey: %v", err)
	}
	commitPlan, err := projectprofile.NewProfileDeclarationCommitPlan(
		inputs,
		resolutionRef,
		digestOfTB(t, "authority-resolution-record"),
		singleUseKey,
	)
	if err != nil {
		t.Fatalf("NewProfileDeclarationCommitPlan: %v", err)
	}
	return v1Fixture{
		payload:           payload,
		record:            record,
		description:       description,
		contract:          contract,
		assignment:        assignmentFixture.assignment,
		assignmentSupport: assignmentFixture.carrier,
		basis:             basis,
		effect:            effect,
		assessment:        assessment,
		candidate:         candidate,
		inputs:            inputs,
		commitPlan:        commitPlan,
	}
}

func mustCandidateWorkRecordV1(
	t testing.TB,
	payloadDigest projectprofile.ContentDigest,
	basis projectprofile.ObservedProjectBasisV1,
	observedBasisDigest projectprofile.ContentDigest,
	assignmentFixture profileAuthorAssignmentSupportFixtureV1,
	projectRoot projectprofile.ProjectRootV1,
	classifierVersion projectprofile.ClassifierVersion,
	policyVersion projectprofile.PolicyVersion,
) projectprofile.ProfileOnboardingWorkRecord {
	t.Helper()
	description := projectprofile.ProfileOnboardingMethodDescriptionV1Value()
	descriptionDigest, err := projectprofile.DigestProfileOnboardingMethodDescriptionV1(description)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingMethodDescriptionV1: %v", err)
	}
	contract, err := projectprofile.ProfileOnboardingMethodContractV1Value()
	if err != nil {
		t.Fatalf("ProfileOnboardingMethodContractV1Value: %v", err)
	}
	contractDigest, err := projectprofile.DigestProfileOnboardingMethodContractV1(contract)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingMethodContractV1: %v", err)
	}
	recordRef := mustProfileWorkRecordRefV1(t, "work-record:profile-classification:1")
	workRef := mustWorkRefV1(t, "work:profile-classification:1")
	bindings := mustParameterBindingsV1(
		t,
		projectRoot,
		classifierVersion,
		policyVersion,
		assignmentFixture.sessionRef,
	)
	performedBy := assignmentFixture.assignment.RoleAssignmentRef()
	executedWithin := assignmentFixture.assignment.HolderSystemRef()
	contextRef := projectprofile.ProfileOnboardingBoundedContextRefV1()
	observedBasisRef := basis.Ref()
	from := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	until := from.Add(time.Hour)
	workInterval, err := projectprofile.NewWorkIntervalV1(from, until)
	if err != nil {
		t.Fatalf("NewWorkIntervalV1: %v", err)
	}
	basisWindow := basis.ObservationWindow()
	transition := mustPrePostTransitionV1(t)
	outcome, err := projectprofile.NewCandidatePayloadProduced(payloadDigest, observedBasisDigest)
	if err != nil {
		t.Fatalf("NewCandidatePayloadProduced: %v", err)
	}
	builder := projectprofile.NewProfileOnboardingWorkRecordBuilder(recordRef, workRef)
	methodRef := projectprofile.ProfileOnboardingMethodRefV1()
	methodDescriptionRef := projectprofile.ProfileOnboardingMethodDescriptionRefV1()
	builder = builder.Enacts(
		methodRef,
		methodDescriptionRef,
		bindings,
	)
	builder = builder.WithMethodDescriptionDigest(descriptionDigest)
	contractRef := contract.Ref()
	builder = builder.GovernedByMethodContract(contractRef, contractDigest)
	builder = builder.PerformedBy(performedBy)
	assignmentDigest, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(assignmentFixture.assignment)
	if err != nil {
		t.Fatalf("DigestProfileAuthorRoleAssignmentV1: %v", err)
	}
	builder = builder.WithProfileAuthorRoleAssignment(
		performedBy,
		assignmentDigest,
	)
	builder = builder.ExecutedWithin(executedWithin)
	builder = builder.InContext(contextRef)
	builder = builder.During(workInterval, basisWindow)
	builder = builder.WithObservedProjectBasis(observedBasisRef, observedBasisDigest)
	observedBasisRefValue := observedBasisRef.String()
	inputRef := mustWorkInputRefV1(t, observedBasisRefValue)
	builder = builder.WithInputs([]projectprofile.WorkInputRef{inputRef})
	outputRef := mustWorkOutputRefV1(t, "output:classification-payload:1")
	builder = builder.WithOutputs([]projectprofile.WorkOutputRef{outputRef})
	resourceRef := mustWorkResourceRefV1(t, "resource:host-agent-session:1")
	builder = builder.WithResources([]projectprofile.WorkResourceRef{resourceRef})
	affectedRefKind := description.AffectedRefKind()
	builder = builder.AffectingKind(affectedRefKind)
	affectedRef := mustAffectedRefV1(t, "episteme:project-classification:1")
	builder = builder.Affecting([]projectprofile.AffectedReferentRef{affectedRef})
	statePlaneRef := description.StatePlaneRef()
	builder = builder.OnStatePlane(statePlaneRef, transition)
	builder = builder.WithOutcome(outcome)
	record, err := builder.Build()
	if err != nil {
		t.Fatalf("ProfileOnboardingWorkRecordBuilder.Build: %v", err)
	}
	return record
}

func mustObservedProjectBasisForV1Fixture(
	t testing.TB,
	projectRoot projectprofile.ProjectRootV1,
	classifierVersion projectprofile.ClassifierVersion,
) projectprofile.ObservedProjectBasisV1 {
	t.Helper()
	basisRef, err := projectprofile.NewObservedProjectBasisRefV1("observed-project-basis:profile-onboarding:1")
	if err != nil {
		t.Fatalf("NewObservedProjectBasisRefV1: %v", err)
	}
	window, err := projectprofile.NewBasisObservationWindowV1(
		time.Date(2026, 7, 14, 8, 30, 0, 0, time.UTC),
		time.Date(2026, 7, 14, 10, 30, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewBasisObservationWindowV1: %v", err)
	}
	signalKind, err := projectprofile.NewObservedProjectSignalKindV1("repository-shape")
	if err != nil {
		t.Fatalf("NewObservedProjectSignalKindV1: %v", err)
	}
	signalValue, err := projectprofile.NewObservedProjectSignalValueV1("software")
	if err != nil {
		t.Fatalf("NewObservedProjectSignalValueV1: %v", err)
	}
	sourceRef, err := projectprofile.NewSourceCarrierRefV1("carrier:repository-tree:1")
	if err != nil {
		t.Fatalf("NewSourceCarrierRefV1: %v", err)
	}
	evidenceRef, err := projectprofile.NewEvidenceProvenancePathRefV1("evidence:path:repository-tree:1")
	if err != nil {
		t.Fatalf("NewEvidenceProvenancePathRefV1: %v", err)
	}
	signal, err := projectprofile.NewObservedProjectSignalV1(
		signalKind,
		signalValue,
		sourceRef,
		[]projectprofile.EvidenceProvenancePathRefV1{evidenceRef},
	)
	if err != nil {
		t.Fatalf("NewObservedProjectSignalV1: %v", err)
	}
	detectorVersion, err := projectprofile.NewObservedProjectDetectorVersionV1("detector-v1")
	if err != nil {
		t.Fatalf("NewObservedProjectDetectorVersionV1: %v", err)
	}
	basis, err := projectprofile.NewObservedProjectBasisV1(
		basisRef,
		projectRoot,
		window,
		[]projectprofile.ObservedProjectSignalV1{signal},
		detectorVersion,
		classifierVersion,
	)
	if err != nil {
		t.Fatalf("NewObservedProjectBasisV1: %v", err)
	}
	return basis
}

func mustProfileOnboardingEffectForV1Fixture(
	t testing.TB,
	record projectprofile.ProfileOnboardingWorkRecord,
	payloadDigest projectprofile.ContentDigest,
	basis projectprofile.ObservedProjectBasisV1,
	basisDigest projectprofile.ContentDigest,
) projectprofile.ProfileOnboardingEffectV1 {
	t.Helper()
	outputs := record.OutputRefs()
	if len(outputs) != 1 {
		t.Fatalf("fixture Work outputs = %d, want 1", len(outputs))
	}
	result, err := projectprofile.NewProfileOnboardingCandidateResultV1(
		outputs[0],
		payloadDigest,
		basis.Ref(),
		basisDigest,
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingCandidateResultV1: %v", err)
	}
	affected := record.AffectedRefs()
	if len(affected) != 1 {
		t.Fatalf("fixture Work affected refs = %d, want 1", len(affected))
	}
	affectedEntity, err := projectprofile.NewEntityRef(affected[0].String())
	if err != nil {
		t.Fatalf("NewEntityRef: %v", err)
	}
	workDigest, err := projectprofile.DigestProfileOnboardingWorkRecord(record)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingWorkRecord: %v", err)
	}
	effectRef, err := projectprofile.NewProfileOnboardingEffectRefV1("effect:profile-onboarding:1")
	if err != nil {
		t.Fatalf("NewProfileOnboardingEffectRefV1: %v", err)
	}
	evidenceRef, err := projectprofile.NewEvidenceProvenancePathRefV1("evidence:path:profile-onboarding-effect:1")
	if err != nil {
		t.Fatalf("NewEvidenceProvenancePathRefV1: %v", err)
	}
	effect, err := projectprofile.NewProfileOnboardingEffectV1(
		effectRef,
		record.RecordRef(),
		record.WorkRef(),
		workDigest,
		result,
		[]projectprofile.EntityRef{affectedEntity},
		record.StatePlaneRef(),
		record.StateTransition(),
		[]projectprofile.EvidenceProvenancePathRefV1{evidenceRef},
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingEffectV1: %v", err)
	}
	return effect
}

func mustPassedOutcomeAssessmentForV1Fixture(
	t testing.TB,
	effect projectprofile.ProfileOnboardingEffectV1,
	contract projectprofile.ProfileOnboardingMethodContractV1,
) projectprofile.ProfileOnboardingOutcomeAssessmentV1 {
	t.Helper()
	assessmentRef, err := projectprofile.NewProfileOnboardingOutcomeAssessmentRefV1("assessment:profile-onboarding:1")
	if err != nil {
		t.Fatalf("NewProfileOnboardingOutcomeAssessmentRefV1: %v", err)
	}
	standardEdition, err := projectprofile.NewProfileOnboardingAcceptanceStandardEditionV1(
		contract.AcceptanceStandardEdition(),
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingAcceptanceStandardEditionV1: %v", err)
	}
	comparatorRef, err := projectprofile.NewProfileOnboardingComparatorRefV1("comparator:profile-onboarding:1")
	if err != nil {
		t.Fatalf("NewProfileOnboardingComparatorRefV1: %v", err)
	}
	comparatorEdition, err := projectprofile.NewProfileOnboardingComparatorEditionV1("v1")
	if err != nil {
		t.Fatalf("NewProfileOnboardingComparatorEditionV1: %v", err)
	}
	evidenceRef, err := projectprofile.NewEvidenceProvenancePathRefV1("evidence:path:profile-onboarding-assessment:1")
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
		projectprofile.ProfileOnboardingAcceptancePassedV1Value(),
		[]projectprofile.EvidenceProvenancePathRefV1{evidenceRef},
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingOutcomeAssessmentV1: %v", err)
	}
	return assessment
}

func cloneV1WorkBuilder(
	t testing.TB,
	fixture v1Fixture,
) projectprofile.ProfileOnboardingWorkRecordBuilder {
	t.Helper()
	record := fixture.record
	recordRef := record.RecordRef()
	workRef := record.WorkRef()
	builder := projectprofile.NewProfileOnboardingWorkRecordBuilder(recordRef, workRef)
	methodRef := record.EnactsMethodRef()
	methodDescriptionRef := record.MethodDescriptionRef()
	parameterBindings := record.ParameterBindings()
	builder = builder.Enacts(methodRef, methodDescriptionRef, parameterBindings)
	methodDescriptionDigest := record.MethodDescriptionDigest()
	builder = builder.WithMethodDescriptionDigest(methodDescriptionDigest)
	methodContractRef := record.MethodContractRef()
	methodContractDigest := record.MethodContractDigest()
	builder = builder.GovernedByMethodContract(methodContractRef, methodContractDigest)
	performedBy := record.PerformedBy()
	builder = builder.PerformedBy(performedBy)
	assignmentRef := record.ProfileAuthorRoleAssignmentRef()
	assignmentDigest := record.ProfileAuthorRoleAssignmentDigest()
	builder = builder.WithProfileAuthorRoleAssignment(
		assignmentRef,
		assignmentDigest,
	)
	executedWithin := record.ExecutedWithin()
	builder = builder.ExecutedWithin(executedWithin)
	boundedContextRef := record.BoundedContextRef()
	builder = builder.InContext(boundedContextRef)
	workInterval := record.WorkInterval()
	basisWindow := record.BasisObservationWindow()
	builder = builder.During(workInterval, basisWindow)
	basisRef := record.ObservedProjectBasisRef()
	basisDigest := record.ObservedProjectBasisDigest()
	builder = builder.WithObservedProjectBasis(
		basisRef,
		basisDigest,
	)
	inputRefs := record.InputRefs()
	builder = builder.WithInputs(inputRefs)
	outputRefs := record.OutputRefs()
	builder = builder.WithOutputs(outputRefs)
	resourceRefs := record.ResourceRefs()
	builder = builder.WithResources(resourceRefs)
	affectedRefKind := record.AffectedRefKind()
	builder = builder.AffectingKind(affectedRefKind)
	affectedRefs := record.AffectedRefs()
	builder = builder.Affecting(affectedRefs)
	statePlaneRef := record.StatePlaneRef()
	stateTransition := record.StateTransition()
	builder = builder.OnStatePlane(statePlaneRef, stateTransition)
	outcome := record.Outcome()
	builder = builder.WithOutcome(outcome)
	return builder
}

func assertCandidateWorkParameterMismatchV1(
	t testing.TB,
	fixture v1Fixture,
	parameterName string,
	wrongValue string,
) {
	t.Helper()
	originalProvenance := fixture.candidate.Provenance()
	parameterValues := map[string]string{
		"classifier_version": originalProvenance.ClassifierVersion().String(),
		"policy_version":     originalProvenance.PolicyVersion().String(),
		"project_root":       originalProvenance.ProjectRoot().String(),
		"session_ref":        originalProvenance.SessionRef().String(),
	}
	parameterValues[parameterName] = wrongValue
	bindings, err := projectprofile.NewMethodParameterBindings(
		[]projectprofile.MethodParameterBinding{
			mustMethodParameterBindingV1(t, "classifier_version", parameterValues["classifier_version"]),
			mustMethodParameterBindingV1(t, "policy_version", parameterValues["policy_version"]),
			mustMethodParameterBindingV1(t, "project_root", parameterValues["project_root"]),
			mustMethodParameterBindingV1(t, "session_ref", parameterValues["session_ref"]),
		},
	)
	if err != nil {
		t.Fatalf("NewMethodParameterBindings(%s): %v", parameterName, err)
	}
	wrongBuilder := cloneV1WorkBuilder(t, fixture)
	wrongBuilder = wrongBuilder.Enacts(
		fixture.record.EnactsMethodRef(),
		fixture.record.MethodDescriptionRef(),
		bindings,
	)
	wrongRecord, err := wrongBuilder.Build()
	if err != nil {
		t.Fatalf("ProfileOnboardingWorkRecordBuilder.Build(%s): %v", parameterName, err)
	}
	wrongWorkDigest, err := projectprofile.DigestProfileOnboardingWorkRecord(wrongRecord)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingWorkRecord(%s): %v", parameterName, err)
	}
	provenanceBuilder := projectprofile.NewCandidateProvenanceV1Builder(
		originalProvenance.AuthorityBasisRef(),
		wrongRecord.RecordRef(),
		wrongWorkDigest,
	)
	assignmentDigest, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(fixture.assignment)
	if err != nil {
		t.Fatalf("DigestProfileAuthorRoleAssignmentV1(%s): %v", parameterName, err)
	}
	provenanceBuilder = provenanceBuilder.ForProfileAuthorRoleAssignment(
		fixture.assignment.RoleAssignmentRef(),
		assignmentDigest,
	)
	provenanceBuilder = provenanceBuilder.ForProject(originalProvenance.ProjectRoot())
	provenanceBuilder = provenanceBuilder.ClassifiedBy(
		originalProvenance.ClassifierVersion(),
		originalProvenance.PolicyVersion(),
	)
	provenanceBuilder = provenanceBuilder.InSession(originalProvenance.SessionRef())
	provenanceBuilder = provenanceBuilder.ForPayload(originalProvenance.PayloadDigest())
	provenanceBuilder = provenanceBuilder.ForObservedProjectBasis(
		originalProvenance.ObservedProjectBasisRef(),
		originalProvenance.ObservedProjectBasisDigest(),
	)
	provenanceBuilder = provenanceBuilder.ForOutcomeAssessment(
		originalProvenance.OutcomeAssessmentRef(),
		originalProvenance.OutcomeAssessmentDigest(),
	)
	provenance, err := provenanceBuilder.Build()
	if err != nil {
		t.Fatalf("CandidateProvenanceV1Builder.Build(%s): %v", parameterName, err)
	}
	candidate, err := projectprofile.NewProfileDeclarationCandidateV1(
		fixture.candidate.Payload(),
		provenance,
	)
	if err != nil {
		t.Fatalf("NewProfileDeclarationCandidateV1(%s): %v", parameterName, err)
	}
	err = projectprofile.ValidateProfileDeclarationCandidateV1AgainstSupports(
		candidate,
		wrongRecord,
		fixture.description,
		fixture.contract,
		fixture.assignment,
		fixture.assignmentSupport,
		fixture.basis,
		fixture.effect,
		fixture.assessment,
	)
	if err == nil {
		t.Fatalf("candidate/Work validation did not reject exact %s binding: %v", parameterName, err)
	}
}

func mustMethodParameterBindingV1(
	t testing.TB,
	name string,
	value string,
) projectprofile.MethodParameterBinding {
	t.Helper()
	binding, err := projectprofile.NewMethodParameterBinding(name, value)
	if err != nil {
		t.Fatalf("NewMethodParameterBinding(%s): %v", name, err)
	}
	return binding
}

func mustParameterBindingsV1(
	t testing.TB,
	projectRoot projectprofile.ProjectRootV1,
	classifierVersion projectprofile.ClassifierVersion,
	policyVersion projectprofile.PolicyVersion,
	sessionRef projectprofile.SessionRef,
) projectprofile.MethodParameterBindings {
	t.Helper()
	root, err := projectprofile.NewMethodParameterBinding("project_root", projectRoot.String())
	if err != nil {
		t.Fatalf("NewMethodParameterBinding(project_root): %v", err)
	}
	policy, err := projectprofile.NewMethodParameterBinding("policy_version", policyVersion.String())
	if err != nil {
		t.Fatalf("NewMethodParameterBinding(policy_version): %v", err)
	}
	classifier, err := projectprofile.NewMethodParameterBinding("classifier_version", classifierVersion.String())
	if err != nil {
		t.Fatalf("NewMethodParameterBinding(classifier_version): %v", err)
	}
	session, err := projectprofile.NewMethodParameterBinding("session_ref", sessionRef.String())
	if err != nil {
		t.Fatalf("NewMethodParameterBinding(session_ref): %v", err)
	}
	bindings, err := projectprofile.NewMethodParameterBindings(
		[]projectprofile.MethodParameterBinding{root, policy, classifier, session},
	)
	if err != nil {
		t.Fatalf("NewMethodParameterBindings: %v", err)
	}
	return bindings
}

func mustPrePostTransitionV1(t testing.TB) projectprofile.WorkStateTransitionV1 {
	t.Helper()
	pre := mustStateRefV1(t, "state:profile:auto")
	post := mustStateRefV1(t, "state:profile:candidate-produced")
	transition, err := projectprofile.NewPrePostStateTransitionV1(pre, post)
	if err != nil {
		t.Fatalf("NewPrePostStateTransitionV1: %v", err)
	}
	return transition
}

func mustSoftwareScopeTB(t testing.TB, raw string) projectprofile.SoftwareRealization {
	t.Helper()
	id, err := projectprofile.NewScopeID(raw)
	if err != nil {
		t.Fatalf("NewScopeID: %v", err)
	}
	scope, err := projectprofile.NewSoftwareRealization(id, projectprofile.NoEntityReference{})
	if err != nil {
		t.Fatalf("NewSoftwareRealization: %v", err)
	}
	return scope
}

func digestOfTB(t testing.TB, seed string) projectprofile.ContentDigest {
	t.Helper()
	concrete, ok := t.(*testing.T)
	if ok {
		return digestOf(concrete, seed)
	}
	value := strings.Repeat("0", 64-len(seed)) + strings.Repeat("a", len(seed))
	digest, err := projectprofile.NewContentDigest("sha256:" + value)
	if err != nil {
		t.Fatalf("NewContentDigest: %v", err)
	}
	return digest
}

func mustClassifierVersionTB(t testing.TB, raw string) projectprofile.ClassifierVersion {
	t.Helper()
	value, err := projectprofile.NewClassifierVersion(raw)
	if err != nil {
		t.Fatalf("NewClassifierVersion: %v", err)
	}
	return value
}

func mustPolicyVersionTB(t testing.TB, raw string) projectprofile.PolicyVersion {
	t.Helper()
	value, err := projectprofile.NewPolicyVersion(raw)
	if err != nil {
		t.Fatalf("NewPolicyVersion: %v", err)
	}
	return value
}

func reflectedTypeContainsV1(recordType reflect.Type, fragment string, index int) bool {
	if index == recordType.NumField() {
		return false
	}
	fieldName := strings.ToLower(recordType.Field(index).Name)
	if strings.Contains(fieldName, fragment) {
		return true
	}
	return reflectedTypeContainsV1(recordType, fragment, index+1)
}

func mustProfileWorkRecordRefV1(t testing.TB, raw string) projectprofile.ProfileOnboardingWorkRecordRef {
	value, err := projectprofile.NewProfileOnboardingWorkRecordRef(raw)
	if err != nil {
		t.Fatalf("NewProfileOnboardingWorkRecordRef: %v", err)
	}
	return value
}

func mustWorkRefV1(t testing.TB, raw string) projectprofile.WorkRef {
	value, err := projectprofile.NewWorkRef(raw)
	if err != nil {
		t.Fatalf("NewWorkRef: %v", err)
	}
	return value
}

func mustRoleAssignmentRefV1(t testing.TB, raw string) projectprofile.RoleAssignmentRef {
	value, err := projectprofile.NewRoleAssignmentRef(raw)
	if err != nil {
		t.Fatalf("NewRoleAssignmentRef: %v", err)
	}
	return value
}

func mustSystemRefV1(t testing.TB, raw string) projectprofile.SystemRef {
	value, err := projectprofile.NewSystemRef(raw)
	if err != nil {
		t.Fatalf("NewSystemRef: %v", err)
	}
	return value
}

func mustBoundedContextRefV1(t testing.TB, raw string) projectprofile.BoundedContextRef {
	value, err := projectprofile.NewBoundedContextRef(raw)
	if err != nil {
		t.Fatalf("NewBoundedContextRef: %v", err)
	}
	return value
}

func mustWorkInputRefV1(t testing.TB, raw string) projectprofile.WorkInputRef {
	value, err := projectprofile.NewWorkInputRef(raw)
	if err != nil {
		t.Fatalf("NewWorkInputRef: %v", err)
	}
	return value
}

func mustWorkOutputRefV1(t testing.TB, raw string) projectprofile.WorkOutputRef {
	value, err := projectprofile.NewWorkOutputRef(raw)
	if err != nil {
		t.Fatalf("NewWorkOutputRef: %v", err)
	}
	return value
}

func mustWorkResourceRefV1(t testing.TB, raw string) projectprofile.WorkResourceRef {
	value, err := projectprofile.NewWorkResourceRef(raw)
	if err != nil {
		t.Fatalf("NewWorkResourceRef: %v", err)
	}
	return value
}

func mustAffectedRefV1(t testing.TB, raw string) projectprofile.AffectedReferentRef {
	value, err := projectprofile.NewAffectedReferentRef(raw)
	if err != nil {
		t.Fatalf("NewAffectedReferentRef: %v", err)
	}
	return value
}

func mustStatePlaneRefV1(t testing.TB, raw string) projectprofile.StatePlaneRef {
	value, err := projectprofile.NewStatePlaneRef(raw)
	if err != nil {
		t.Fatalf("NewStatePlaneRef: %v", err)
	}
	return value
}

func mustStateRefV1(t testing.TB, raw string) projectprofile.StateRef {
	value, err := projectprofile.NewStateRef(raw)
	if err != nil {
		t.Fatalf("NewStateRef: %v", err)
	}
	return value
}
