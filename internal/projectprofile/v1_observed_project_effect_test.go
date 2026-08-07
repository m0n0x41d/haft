package projectprofile

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

type pboFixtureV1 struct {
	basis             ObservedProjectBasisV1
	basisDigest       ContentDigest
	work              ProfileOnboardingWorkRecord
	workDigest        ContentDigest
	payloadDigest     ContentDigest
	outputRef         WorkOutputRef
	statePlaneRef     StatePlaneRef
	stateWitness      WorkStateTransitionV1
	affectedEntityRef EntityRef
}

type pboForeignEffectResultV1 struct{}

func (pboForeignEffectResultV1) profileOnboardingEffectResultV1() {}
func (pboForeignEffectResultV1) Kind() ProfileOnboardingResultKindV1 {
	return ProfileOnboardingResultKindV1{value: profileOnboardingCandidateResultKindV1Value}
}
func (pboForeignEffectResultV1) OutputRef() WorkOutputRef { return WorkOutputRef{} }

type pboForeignAcceptanceVerdictV1 struct{}

func (pboForeignAcceptanceVerdictV1) profileOnboardingAcceptanceVerdictV1() {}
func (pboForeignAcceptanceVerdictV1) Kind() string                          { return "passed" }

func TestObservedProjectBasisV1CanonicalRelationAndCarrier(t *testing.T) {
	fixture := pboFixture(t)
	basis := fixture.basis

	signals := basis.Signals()
	if len(signals) != 2 {
		t.Fatalf("Signals length = %d, want 2", len(signals))
	}
	if signals[0].Kind().String() != "manifest" {
		t.Fatalf("signals were not canonically sorted: %q", signals[0].Kind().String())
	}
	signals[0] = signals[1]
	if basis.Signals()[0].Kind().String() != "manifest" {
		t.Fatal("mutating returned signals changed ObservedProjectBasis")
	}

	canonical, err := EncodeObservedProjectBasisV1CanonicalJSON(basis)
	if err != nil {
		t.Fatalf("EncodeObservedProjectBasisV1CanonicalJSON: %v", err)
	}
	decoded, err := DecodeObservedProjectBasisV1CanonicalJSON(canonical)
	if err != nil {
		t.Fatalf("DecodeObservedProjectBasisV1CanonicalJSON: %v", err)
	}
	decodedDigest, err := DigestObservedProjectBasisV1(decoded)
	if err != nil {
		t.Fatalf("DigestObservedProjectBasisV1(decoded): %v", err)
	}
	if decodedDigest != fixture.basisDigest {
		t.Fatal("ObservedProjectBasis digest changed after canonical roundtrip")
	}

	carrier, err := CarryObservedProjectBasisV1(basis)
	if err != nil {
		t.Fatalf("CarryObservedProjectBasisV1: %v", err)
	}
	carrierBytes := carrier.CanonicalJSON()
	carrierBytes[0] = '['
	if bytes.Equal(carrierBytes, carrier.CanonicalJSON()) {
		t.Fatal("ObservedProjectBasis carrier leaked mutable bytes")
	}
	if carrier.ContentDigest() != fixture.basisDigest {
		t.Fatal("ObservedProjectBasis carrier digest does not bind its canonical bytes")
	}

	err = ValidateObservedProjectBasisV1AgainstWorkRecord(basis, fixture.work)
	if err != nil {
		t.Fatalf("ValidateObservedProjectBasisV1AgainstWorkRecord: %v", err)
	}
}

func TestProfileOnboardingEffectAndAssessmentBindExactRelations(t *testing.T) {
	fixture := pboFixture(t)
	effect := pboEffect(t, fixture, "evidence:path:effect:2", "evidence:path:effect:1")

	err := ValidateProfileOnboardingEffectV1AgainstWorkRecord(effect, fixture.work)
	if err != nil {
		t.Fatalf("ValidateProfileOnboardingEffectV1AgainstWorkRecord: %v", err)
	}
	err = ValidateProfileOnboardingEffectV1AgainstObservedProjectBasis(effect, fixture.basis)
	if err != nil {
		t.Fatalf("ValidateProfileOnboardingEffectV1AgainstObservedProjectBasis: %v", err)
	}
	if effect.EvidencePathRefs()[0].String() != "evidence:path:effect:1" {
		t.Fatal("ProfileOnboardingEffect evidence refs were not canonically sorted")
	}

	effectJSON, err := EncodeProfileOnboardingEffectV1CanonicalJSON(effect)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingEffectV1CanonicalJSON: %v", err)
	}
	decodedEffect, err := DecodeProfileOnboardingEffectV1CanonicalJSON(effectJSON)
	if err != nil {
		t.Fatalf("DecodeProfileOnboardingEffectV1CanonicalJSON: %v", err)
	}
	effectDigest, err := DigestProfileOnboardingEffectV1(effect)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingEffectV1: %v", err)
	}
	decodedEffectDigest, err := DigestProfileOnboardingEffectV1(decodedEffect)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingEffectV1(decoded): %v", err)
	}
	if effectDigest != decodedEffectDigest {
		t.Fatal("ProfileOnboardingEffect digest changed after canonical roundtrip")
	}

	assessment := pboAssessment(t, effect, ProfileOnboardingAcceptancePassedV1Value())
	err = ValidateProfileOnboardingOutcomeAssessmentV1AgainstEffect(assessment, effect)
	if err != nil {
		t.Fatalf("ValidateProfileOnboardingOutcomeAssessmentV1AgainstEffect: %v", err)
	}
	assessmentJSON, err := EncodeProfileOnboardingOutcomeAssessmentV1CanonicalJSON(assessment)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingOutcomeAssessmentV1CanonicalJSON: %v", err)
	}
	decodedAssessment, err := DecodeProfileOnboardingOutcomeAssessmentV1CanonicalJSON(
		assessmentJSON,
		effect,
	)
	if err != nil {
		t.Fatalf("DecodeProfileOnboardingOutcomeAssessmentV1CanonicalJSON: %v", err)
	}
	assessmentDigest, err := DigestProfileOnboardingOutcomeAssessmentV1(assessment)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingOutcomeAssessmentV1: %v", err)
	}
	decodedAssessmentDigest, err := DigestProfileOnboardingOutcomeAssessmentV1(decodedAssessment)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingOutcomeAssessmentV1(decoded): %v", err)
	}
	if assessmentDigest != decodedAssessmentDigest {
		t.Fatal("outcome-assessment digest changed after canonical roundtrip")
	}

	carrier, err := CarryProfileOnboardingOutcomeAssessmentV1(assessment)
	if err != nil {
		t.Fatalf("CarryProfileOnboardingOutcomeAssessmentV1: %v", err)
	}
	carrierBytes := carrier.CanonicalJSON()
	carrierBytes[len(carrierBytes)-1] = ']'
	if bytes.Equal(carrierBytes, carrier.CanonicalJSON()) {
		t.Fatal("outcome-assessment carrier leaked mutable bytes")
	}
}

func TestObservedProjectBasisV1RejectsUnboundOrNoncanonicalClaims(t *testing.T) {
	fixture := pboFixture(t)
	concrete := fixture.basis.(observedProjectBasisV1)

	_, err := NewObservedProjectBasisV1(
		concrete.ref,
		concrete.projectRoot,
		concrete.observationWindow,
		nil,
		concrete.detectorVersion,
		concrete.classifierVersion,
	)
	if err == nil {
		t.Fatal("ObservedProjectBasis accepted an empty signal set")
	}

	signal := concrete.signals[0]
	_, err = NewObservedProjectSignalV1(
		signal.kind,
		signal.value,
		signal.sourceCarrierRef,
		nil,
	)
	if err == nil {
		t.Fatal("observed signal accepted no evidence-provenance path")
	}

	_, err = NewObservedProjectBasisV1(
		concrete.ref,
		concrete.projectRoot,
		concrete.observationWindow,
		[]ObservedProjectSignalV1{signal, signal},
		concrete.detectorVersion,
		concrete.classifierVersion,
	)
	if err == nil {
		t.Fatal("ObservedProjectBasis accepted duplicate signals")
	}

	canonical, err := EncodeObservedProjectBasisV1CanonicalJSON(fixture.basis)
	if err != nil {
		t.Fatalf("EncodeObservedProjectBasisV1CanonicalJSON: %v", err)
	}
	noncanonical := append([]byte(" "), canonical...)
	_, err = DecodeObservedProjectBasisV1CanonicalJSON(noncanonical)
	if err == nil {
		t.Fatal("ObservedProjectBasis decoder accepted noncanonical JSON")
	}

	unknownField := append([]byte{}, canonical[:len(canonical)-1]...)
	unknownField = append(unknownField, []byte(`,"claim_is_true":true}`)...)
	_, err = DecodeObservedProjectBasisV1CanonicalJSON(unknownField)
	if err == nil {
		t.Fatal("ObservedProjectBasis decoder accepted an evidence-truth field")
	}
}

func TestProfileOnboardingEffectRejectsMismatchedWorkBasisAndAssessment(t *testing.T) {
	fixture := pboFixture(t)
	effect := pboEffect(t, fixture, "evidence:path:effect:1")

	wrongDigest := pboDigest(t, "d")
	wrongEffect := pboEffectWithWorkDigest(t, fixture, wrongDigest)
	err := ValidateProfileOnboardingEffectV1AgainstWorkRecord(wrongEffect, fixture.work)
	if err == nil {
		t.Fatal("effect validation accepted another Work-record digest")
	}

	otherBasis := pboBasisWithRef(t, fixture.basis, "observed-basis:other")
	err = ValidateProfileOnboardingEffectV1AgainstObservedProjectBasis(effect, otherBasis)
	if err == nil {
		t.Fatal("effect validation accepted another ObservedProjectBasis")
	}

	effectJSON, err := EncodeProfileOnboardingEffectV1CanonicalJSON(effect)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingEffectV1CanonicalJSON: %v", err)
	}
	unknownField := append([]byte{}, effectJSON[:len(effectJSON)-1]...)
	unknownField = append(unknownField, []byte(`,"evidence_is_true":true}`)...)
	_, err = DecodeProfileOnboardingEffectV1CanonicalJSON(unknownField)
	if err == nil {
		t.Fatal("effect decoder accepted an evidence-truth field")
	}

	assessment := pboAssessment(t, effect, ProfileOnboardingAcceptancePassedV1Value())
	assessmentJSON, err := EncodeProfileOnboardingOutcomeAssessmentV1CanonicalJSON(assessment)
	if err != nil {
		t.Fatalf("EncodeProfileOnboardingOutcomeAssessmentV1CanonicalJSON: %v", err)
	}
	otherEffect := pboEffect(t, fixture, "evidence:path:effect:other")
	_, err = DecodeProfileOnboardingOutcomeAssessmentV1CanonicalJSON(assessmentJSON, otherEffect)
	if err == nil {
		t.Fatal("outcome-assessment decoder accepted another effect")
	}

	missingDigest := pboDigest(t, "e")
	outputRef := fixture.outputRef
	underdetermined, err := NewProfileOnboardingUnderdeterminedResultV1(outputRef, missingDigest)
	if err != nil {
		t.Fatalf("NewProfileOnboardingUnderdeterminedResultV1: %v", err)
	}
	underdeterminedEffect := pboEffectWithResult(t, fixture, underdetermined)
	_, err = pboNewAssessment(
		t,
		underdeterminedEffect,
		ProfileOnboardingAcceptancePassedV1Value(),
	)
	if err == nil {
		t.Fatal("outcome assessment accepted passed verdict for underdetermined result")
	}

	undeterminedVerdict, err := NewProfileOnboardingAcceptanceUndeterminedV1(missingDigest)
	if err != nil {
		t.Fatalf("NewProfileOnboardingAcceptanceUndeterminedV1: %v", err)
	}
	_, err = pboNewAssessment(t, underdeterminedEffect, undeterminedVerdict)
	if err != nil {
		t.Fatalf("undetermined result/verdict pair was rejected: %v", err)
	}

	_, err = NewProfileOnboardingEffectV1(
		pboEffectRef(t, "effect:foreign-result"),
		fixture.work.RecordRef(),
		fixture.work.WorkRef(),
		fixture.workDigest,
		pboForeignEffectResultV1{},
		[]EntityRef{fixture.affectedEntityRef},
		fixture.statePlaneRef,
		fixture.stateWitness,
		[]EvidenceProvenancePathRefV1{pboEvidencePathRef(t, "evidence:path:foreign")},
	)
	if err == nil {
		t.Fatal("ProfileOnboardingEffect accepted a foreign result variant")
	}

	_, err = pboNewAssessment(t, effect, pboForeignAcceptanceVerdictV1{})
	if err == nil {
		t.Fatal("outcome assessment accepted a foreign verdict variant")
	}
}

func pboFixture(t testing.TB) pboFixtureV1 {
	t.Helper()
	root, err := NewProjectRootV1("/tmp/haft-profile-basis")
	if err != nil {
		t.Fatalf("NewProjectRootV1: %v", err)
	}
	window, err := NewBasisObservationWindowV1(
		time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 14, 8, 30, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewBasisObservationWindowV1: %v", err)
	}
	basisRef := pboObservedBasisRef(t, "observed-basis:profile-onboarding:1")
	detectorVersion, err := NewObservedProjectDetectorVersionV1("detector-v1")
	if err != nil {
		t.Fatalf("NewObservedProjectDetectorVersionV1: %v", err)
	}
	classifierVersion, err := NewClassifierVersion("classifier-v1")
	if err != nil {
		t.Fatalf("NewClassifierVersion: %v", err)
	}
	signals := []ObservedProjectSignalV1{
		pboSignal(t, "repository-shape", "software", "carrier:tree:1", "evidence:path:tree:1"),
		pboSignal(t, "manifest", "go.mod", "carrier:file:go.mod", "evidence:path:manifest:2", "evidence:path:manifest:1"),
	}
	basis, err := NewObservedProjectBasisV1(
		basisRef,
		root,
		window,
		signals,
		detectorVersion,
		classifierVersion,
	)
	if err != nil {
		t.Fatalf("NewObservedProjectBasisV1: %v", err)
	}
	basisDigest, err := DigestObservedProjectBasisV1(basis)
	if err != nil {
		t.Fatalf("DigestObservedProjectBasisV1: %v", err)
	}
	payloadDigest := pboDigest(t, "a")
	outputRef := pboWorkOutputRef(t, "output:profile-candidate:1")
	affectedEntityRef := pboEntityRef(t, "eoc:profile-classification:1")
	statePlaneRef := ProfileOnboardingMethodDescriptionV1Value().StatePlaneRef()
	stateWitness := pboStateWitness(t)
	work := pboWorkRecord(
		t,
		root,
		window,
		basisRef,
		basisDigest,
		payloadDigest,
		outputRef,
		affectedEntityRef,
		statePlaneRef,
		stateWitness,
	)
	workDigest, err := DigestProfileOnboardingWorkRecord(work)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingWorkRecord: %v", err)
	}
	return pboFixtureV1{
		basis:             basis,
		basisDigest:       basisDigest,
		work:              work,
		workDigest:        workDigest,
		payloadDigest:     payloadDigest,
		outputRef:         outputRef,
		statePlaneRef:     statePlaneRef,
		stateWitness:      stateWitness,
		affectedEntityRef: affectedEntityRef,
	}
}

func pboSignal(
	t testing.TB,
	kindRaw string,
	valueRaw string,
	sourceRaw string,
	evidenceRaws ...string,
) ObservedProjectSignalV1 {
	t.Helper()
	kind, err := NewObservedProjectSignalKindV1(kindRaw)
	if err != nil {
		t.Fatalf("NewObservedProjectSignalKindV1: %v", err)
	}
	value, err := NewObservedProjectSignalValueV1(valueRaw)
	if err != nil {
		t.Fatalf("NewObservedProjectSignalValueV1: %v", err)
	}
	sourceRef, err := NewSourceCarrierRefV1(sourceRaw)
	if err != nil {
		t.Fatalf("NewSourceCarrierRefV1: %v", err)
	}
	evidenceRefs := make([]EvidenceProvenancePathRefV1, 0, len(evidenceRaws))
	for _, raw := range evidenceRaws {
		ref, refErr := NewEvidenceProvenancePathRefV1(raw)
		if refErr != nil {
			t.Fatalf("NewEvidenceProvenancePathRefV1: %v", refErr)
		}
		evidenceRefs = append(evidenceRefs, ref)
	}
	signal, err := NewObservedProjectSignalV1(kind, value, sourceRef, evidenceRefs)
	if err != nil {
		t.Fatalf("NewObservedProjectSignalV1: %v", err)
	}
	return signal
}

func pboWorkRecord(
	t testing.TB,
	root ProjectRootV1,
	basisWindow BasisObservationWindowV1,
	basisRef ObservedProjectBasisRefV1,
	basisDigest ContentDigest,
	payloadDigest ContentDigest,
	outputRef WorkOutputRef,
	affectedEntityRef EntityRef,
	statePlaneRef StatePlaneRef,
	stateWitness WorkStateTransitionV1,
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
	bindings := pboParameterBindings(t, root)
	workFrom := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	workUntil := time.Date(2026, 7, 14, 9, 5, 0, 0, time.UTC)
	workInterval, err := NewWorkIntervalV1(workFrom, workUntil)
	if err != nil {
		t.Fatalf("NewWorkIntervalV1: %v", err)
	}
	outcome, err := NewCandidatePayloadProduced(payloadDigest, basisDigest)
	if err != nil {
		t.Fatalf("NewCandidatePayloadProduced: %v", err)
	}
	recordRef := pboWorkRecordRef(t, "work-record:profile-onboarding:1")
	workRef := pboWorkRef(t, "work:profile-onboarding:1")
	builder := NewProfileOnboardingWorkRecordBuilder(
		recordRef,
		workRef,
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
	performedBy := pboRoleAssignmentRef(t, "role-assignment:profile-author:1")
	builder = builder.PerformedBy(performedBy)
	assignmentDigest := pboDigest(t, "b")
	builder = builder.WithProfileAuthorRoleAssignment(
		performedBy,
		assignmentDigest,
	)
	executedWithin := pboSystemRef(t, "system:onboarding-agent:1")
	builder = builder.ExecutedWithin(executedWithin)
	contextRef := ProfileOnboardingBoundedContextRefV1()
	builder = builder.InContext(contextRef)
	builder = builder.During(workInterval, basisWindow)
	builder = builder.WithObservedProjectBasis(basisRef, basisDigest)
	basisRefValue := basisRef.String()
	inputRef := pboWorkInputRef(t, basisRefValue)
	builder = builder.WithInputs([]WorkInputRef{inputRef})
	builder = builder.WithOutputs([]WorkOutputRef{outputRef})
	resourceRef := pboWorkResourceRef(t, "resource:cpu:1")
	builder = builder.WithResources([]WorkResourceRef{resourceRef})
	affectedRefKind := description.AffectedRefKind()
	builder = builder.AffectingKind(affectedRefKind)
	affectedEntityRefValue := affectedEntityRef.String()
	affectedRef := pboAffectedRef(t, affectedEntityRefValue)
	builder = builder.Affecting([]AffectedReferentRef{affectedRef})
	builder = builder.OnStatePlane(statePlaneRef, stateWitness)
	builder = builder.WithOutcome(outcome)
	record, err := builder.Build()
	if err != nil {
		t.Fatalf("ProfileOnboardingWorkRecordBuilder.Build: %v", err)
	}
	return record
}

func pboParameterBindings(t testing.TB, root ProjectRootV1) MethodParameterBindings {
	t.Helper()
	values := []struct {
		name  string
		value string
	}{
		{name: profileOnboardingClassifierParameterV1, value: "classifier-v1"},
		{name: profileOnboardingPolicyParameterV1, value: "policy-v1"},
		{name: profileOnboardingProjectRootParameterV1, value: root.String()},
		{name: profileOnboardingSessionParameterV1, value: "session:profile-onboarding:1"},
	}
	bindings := make([]MethodParameterBinding, 0, len(values))
	for _, value := range values {
		binding, err := NewMethodParameterBinding(value.name, value.value)
		if err != nil {
			t.Fatalf("NewMethodParameterBinding: %v", err)
		}
		bindings = append(bindings, binding)
	}
	result, err := NewMethodParameterBindings(bindings)
	if err != nil {
		t.Fatalf("NewMethodParameterBindings: %v", err)
	}
	return result
}

func pboStateWitness(t testing.TB) WorkStateTransitionV1 {
	t.Helper()
	preState := pboStateRef(t, "state:profile:before")
	postState := pboStateRef(t, "state:profile:after")
	witness, err := NewPrePostStateTransitionV1(preState, postState)
	if err != nil {
		t.Fatalf("NewPrePostStateTransitionV1: %v", err)
	}
	return witness
}

func pboEffect(
	t testing.TB,
	fixture pboFixtureV1,
	evidenceRaws ...string,
) ProfileOnboardingEffectV1 {
	t.Helper()
	return pboEffectWithDigestAndResult(t, fixture, fixture.workDigest, nil, evidenceRaws...)
}

func pboEffectWithWorkDigest(
	t testing.TB,
	fixture pboFixtureV1,
	digest ContentDigest,
) ProfileOnboardingEffectV1 {
	t.Helper()
	return pboEffectWithDigestAndResult(t, fixture, digest, nil, "evidence:path:effect:1")
}

func pboEffectWithResult(
	t testing.TB,
	fixture pboFixtureV1,
	result ProfileOnboardingEffectResultV1,
) ProfileOnboardingEffectV1 {
	t.Helper()
	return pboEffectWithDigestAndResult(
		t,
		fixture,
		fixture.workDigest,
		result,
		"evidence:path:effect:1",
	)
}

func pboEffectWithDigestAndResult(
	t testing.TB,
	fixture pboFixtureV1,
	workDigest ContentDigest,
	result ProfileOnboardingEffectResultV1,
	evidenceRaws ...string,
) ProfileOnboardingEffectV1 {
	t.Helper()
	if result == nil {
		candidate, err := NewProfileOnboardingCandidateResultV1(
			fixture.outputRef,
			fixture.payloadDigest,
			fixture.basis.Ref(),
			fixture.basisDigest,
		)
		if err != nil {
			t.Fatalf("NewProfileOnboardingCandidateResultV1: %v", err)
		}
		result = candidate
	}
	evidenceRefs := make([]EvidenceProvenancePathRefV1, 0, len(evidenceRaws))
	for _, raw := range evidenceRaws {
		ref, err := NewEvidenceProvenancePathRefV1(raw)
		if err != nil {
			t.Fatalf("NewEvidenceProvenancePathRefV1: %v", err)
		}
		evidenceRefs = append(evidenceRefs, ref)
	}
	effect, err := NewProfileOnboardingEffectV1(
		pboEffectRef(t, "effect:profile-onboarding:1"),
		fixture.work.RecordRef(),
		fixture.work.WorkRef(),
		workDigest,
		result,
		[]EntityRef{fixture.affectedEntityRef},
		fixture.statePlaneRef,
		fixture.stateWitness,
		evidenceRefs,
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingEffectV1: %v", err)
	}
	return effect
}

func pboAssessment(
	t testing.TB,
	effect ProfileOnboardingEffectV1,
	verdict ProfileOnboardingAcceptanceVerdictV1,
) ProfileOnboardingOutcomeAssessmentV1 {
	t.Helper()
	assessment, err := pboNewAssessment(t, effect, verdict)
	if err != nil {
		t.Fatalf("NewProfileOnboardingOutcomeAssessmentV1: %v", err)
	}
	return assessment
}

func pboNewAssessment(
	t testing.TB,
	effect ProfileOnboardingEffectV1,
	verdict ProfileOnboardingAcceptanceVerdictV1,
) (ProfileOnboardingOutcomeAssessmentV1, error) {
	t.Helper()
	contract, err := ProfileOnboardingMethodContractV1Value()
	if err != nil {
		t.Fatalf("ProfileOnboardingMethodContractV1Value: %v", err)
	}
	standardEdition, err := NewProfileOnboardingAcceptanceStandardEditionV1(
		contract.AcceptanceStandardEdition(),
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingAcceptanceStandardEditionV1: %v", err)
	}
	comparatorRef, err := NewProfileOnboardingComparatorRefV1("comparator:profile-onboarding:1")
	if err != nil {
		t.Fatalf("NewProfileOnboardingComparatorRefV1: %v", err)
	}
	comparatorEdition, err := NewProfileOnboardingComparatorEditionV1("v1")
	if err != nil {
		t.Fatalf("NewProfileOnboardingComparatorEditionV1: %v", err)
	}
	evidenceRef, err := NewEvidenceProvenancePathRefV1("evidence:path:acceptance:1")
	if err != nil {
		t.Fatalf("NewEvidenceProvenancePathRefV1: %v", err)
	}
	return NewProfileOnboardingOutcomeAssessmentV1(
		pboAssessmentRef(t, "assessment:profile-onboarding:1"),
		effect,
		contract.AcceptanceStandardRef(),
		standardEdition,
		comparatorRef,
		comparatorEdition,
		verdict,
		[]EvidenceProvenancePathRefV1{evidenceRef},
	)
}

func pboBasisWithRef(
	t testing.TB,
	basis ObservedProjectBasisV1,
	raw string,
) ObservedProjectBasisV1 {
	t.Helper()
	exact := basis.(observedProjectBasisV1)
	other, err := NewObservedProjectBasisV1(
		pboObservedBasisRef(t, raw),
		exact.projectRoot,
		exact.observationWindow,
		exact.signals,
		exact.detectorVersion,
		exact.classifierVersion,
	)
	if err != nil {
		t.Fatalf("NewObservedProjectBasisV1(other): %v", err)
	}
	return other
}

func pboDigest(t testing.TB, character string) ContentDigest {
	t.Helper()
	digest, err := NewContentDigest("sha256:" + strings.Repeat(character, 64))
	if err != nil {
		t.Fatalf("NewContentDigest: %v", err)
	}
	return digest
}

func pboObservedBasisRef(t testing.TB, raw string) ObservedProjectBasisRefV1 {
	t.Helper()
	ref, err := NewObservedProjectBasisRefV1(raw)
	if err != nil {
		t.Fatalf("NewObservedProjectBasisRefV1: %v", err)
	}
	return ref
}

func pboWorkRecordRef(t testing.TB, raw string) ProfileOnboardingWorkRecordRef {
	t.Helper()
	ref, err := NewProfileOnboardingWorkRecordRef(raw)
	if err != nil {
		t.Fatalf("NewProfileOnboardingWorkRecordRef: %v", err)
	}
	return ref
}

func pboWorkRef(t testing.TB, raw string) WorkRef {
	t.Helper()
	ref, err := NewWorkRef(raw)
	if err != nil {
		t.Fatalf("NewWorkRef: %v", err)
	}
	return ref
}

func pboRoleAssignmentRef(t testing.TB, raw string) RoleAssignmentRef {
	t.Helper()
	ref, err := NewRoleAssignmentRef(raw)
	if err != nil {
		t.Fatalf("NewRoleAssignmentRef: %v", err)
	}
	return ref
}

func pboSystemRef(t testing.TB, raw string) SystemRef {
	t.Helper()
	ref, err := NewSystemRef(raw)
	if err != nil {
		t.Fatalf("NewSystemRef: %v", err)
	}
	return ref
}

func pboWorkInputRef(t testing.TB, raw string) WorkInputRef {
	t.Helper()
	ref, err := NewWorkInputRef(raw)
	if err != nil {
		t.Fatalf("NewWorkInputRef: %v", err)
	}
	return ref
}

func pboWorkOutputRef(t testing.TB, raw string) WorkOutputRef {
	t.Helper()
	ref, err := NewWorkOutputRef(raw)
	if err != nil {
		t.Fatalf("NewWorkOutputRef: %v", err)
	}
	return ref
}

func pboWorkResourceRef(t testing.TB, raw string) WorkResourceRef {
	t.Helper()
	ref, err := NewWorkResourceRef(raw)
	if err != nil {
		t.Fatalf("NewWorkResourceRef: %v", err)
	}
	return ref
}

func pboAffectedRef(t testing.TB, raw string) AffectedReferentRef {
	t.Helper()
	ref, err := NewAffectedReferentRef(raw)
	if err != nil {
		t.Fatalf("NewAffectedReferentRef: %v", err)
	}
	return ref
}

func pboEntityRef(t testing.TB, raw string) EntityRef {
	t.Helper()
	ref, err := NewEntityRef(raw)
	if err != nil {
		t.Fatalf("NewEntityRef: %v", err)
	}
	return ref
}

func pboStateRef(t testing.TB, raw string) StateRef {
	t.Helper()
	ref, err := NewStateRef(raw)
	if err != nil {
		t.Fatalf("NewStateRef: %v", err)
	}
	return ref
}

func pboEffectRef(t testing.TB, raw string) ProfileOnboardingEffectRefV1 {
	t.Helper()
	ref, err := NewProfileOnboardingEffectRefV1(raw)
	if err != nil {
		t.Fatalf("NewProfileOnboardingEffectRefV1: %v", err)
	}
	return ref
}

func pboAssessmentRef(t testing.TB, raw string) ProfileOnboardingOutcomeAssessmentRefV1 {
	t.Helper()
	ref, err := NewProfileOnboardingOutcomeAssessmentRefV1(raw)
	if err != nil {
		t.Fatalf("NewProfileOnboardingOutcomeAssessmentRefV1: %v", err)
	}
	return ref
}

func pboEvidencePathRef(t testing.TB, raw string) EvidenceProvenancePathRefV1 {
	t.Helper()
	ref, err := NewEvidenceProvenancePathRefV1(raw)
	if err != nil {
		t.Fatalf("NewEvidenceProvenancePathRefV1: %v", err)
	}
	return ref
}
