package projectprofile

import (
	"fmt"
	"slices"
)

const (
	profileOnboardingEffectJSONSchemaV1            = "haft.project-profile.profile-onboarding-effect/v1"
	profileOnboardingEffectDigestV1                = "haft.project-profile.profile-onboarding-effect/v1"
	profileOnboardingOutcomeAssessmentJSONSchemaV1 = "haft.project-profile.profile-onboarding-outcome-assessment/v1"
	profileOnboardingOutcomeAssessmentDigestV1     = "haft.project-profile.profile-onboarding-outcome-assessment/v1"
)

type ProfileOnboardingEffectRefV1 struct{ v1Reference }

func NewProfileOnboardingEffectRefV1(raw string) (ProfileOnboardingEffectRefV1, error) {
	ref, err := newV1Reference("ProfileOnboardingEffect ref", raw)
	return ProfileOnboardingEffectRefV1{v1Reference: ref}, err
}

type ProfileOnboardingOutcomeAssessmentRefV1 struct{ v1Reference }

func NewProfileOnboardingOutcomeAssessmentRefV1(
	raw string,
) (ProfileOnboardingOutcomeAssessmentRefV1, error) {
	ref, err := newV1Reference("ProfileOnboardingOutcomeAssessment ref", raw)
	return ProfileOnboardingOutcomeAssessmentRefV1{v1Reference: ref}, err
}

type ProfileOnboardingComparatorRefV1 struct{ v1Reference }

func NewProfileOnboardingComparatorRefV1(
	raw string,
) (ProfileOnboardingComparatorRefV1, error) {
	ref, err := newV1Reference("profile-onboarding comparator ref", raw)
	return ProfileOnboardingComparatorRefV1{v1Reference: ref}, err
}

type ProfileOnboardingAcceptanceStandardEditionV1 struct{ value string }

func NewProfileOnboardingAcceptanceStandardEditionV1(
	raw string,
) (ProfileOnboardingAcceptanceStandardEditionV1, error) {
	value, err := requireText("profile-onboarding acceptance-standard edition", raw)
	return ProfileOnboardingAcceptanceStandardEditionV1{value: value}, err
}

func (edition ProfileOnboardingAcceptanceStandardEditionV1) String() string {
	return edition.value
}

func (edition ProfileOnboardingAcceptanceStandardEditionV1) valid() bool {
	_, err := requireText("profile-onboarding acceptance-standard edition", edition.value)
	return err == nil
}

type ProfileOnboardingComparatorEditionV1 struct{ value string }

func NewProfileOnboardingComparatorEditionV1(
	raw string,
) (ProfileOnboardingComparatorEditionV1, error) {
	value, err := requireText("profile-onboarding comparator edition", raw)
	return ProfileOnboardingComparatorEditionV1{value: value}, err
}

func (edition ProfileOnboardingComparatorEditionV1) String() string {
	return edition.value
}

func (edition ProfileOnboardingComparatorEditionV1) valid() bool {
	_, err := requireText("profile-onboarding comparator edition", edition.value)
	return err == nil
}

type ProfileOnboardingAcceptanceReasonRefV1 struct{ v1Reference }

func NewProfileOnboardingAcceptanceReasonRefV1(
	raw string,
) (ProfileOnboardingAcceptanceReasonRefV1, error) {
	ref, err := newV1Reference("profile-onboarding acceptance reason ref", raw)
	return ProfileOnboardingAcceptanceReasonRefV1{v1Reference: ref}, err
}

// ProfileOnboardingEffectResultV1 is the result produced by one Work
// occurrence. It is not an acceptance verdict.
type ProfileOnboardingEffectResultV1 interface {
	Kind() ProfileOnboardingResultKindV1
	OutputRef() WorkOutputRef
	profileOnboardingEffectResultV1()
}

type ProfileOnboardingCandidateResultV1 interface {
	ProfileOnboardingEffectResultV1
	PayloadDigest() ContentDigest
	ObservedProjectBasisRef() ObservedProjectBasisRefV1
	ObservedProjectBasisDigest() ContentDigest
	profileOnboardingCandidateResultV1()
}

type profileOnboardingCandidateResultV1 struct {
	outputRef                  WorkOutputRef
	payloadDigest              ContentDigest
	observedProjectBasisRef    ObservedProjectBasisRefV1
	observedProjectBasisDigest ContentDigest
}

func (profileOnboardingCandidateResultV1) profileOnboardingEffectResultV1()    {}
func (profileOnboardingCandidateResultV1) profileOnboardingCandidateResultV1() {}

func NewProfileOnboardingCandidateResultV1(
	outputRef WorkOutputRef,
	payloadDigest ContentDigest,
	observedProjectBasisRef ObservedProjectBasisRefV1,
	observedProjectBasisDigest ContentDigest,
) (ProfileOnboardingCandidateResultV1, error) {
	value := profileOnboardingCandidateResultV1{
		outputRef:                  outputRef,
		payloadDigest:              payloadDigest,
		observedProjectBasisRef:    observedProjectBasisRef,
		observedProjectBasisDigest: observedProjectBasisDigest,
	}
	return canonicalProfileOnboardingCandidateResultV1(value)
}

func (profileOnboardingCandidateResultV1) Kind() ProfileOnboardingResultKindV1 {
	return ProfileOnboardingResultKindV1{value: profileOnboardingCandidateResultKindV1Value}
}

func (result profileOnboardingCandidateResultV1) OutputRef() WorkOutputRef {
	return result.outputRef
}

func (result profileOnboardingCandidateResultV1) PayloadDigest() ContentDigest {
	return result.payloadDigest
}

func (result profileOnboardingCandidateResultV1) ObservedProjectBasisRef() ObservedProjectBasisRefV1 {
	return result.observedProjectBasisRef
}

func (result profileOnboardingCandidateResultV1) ObservedProjectBasisDigest() ContentDigest {
	return result.observedProjectBasisDigest
}

type ProfileOnboardingUnderdeterminedResultV1 interface {
	ProfileOnboardingEffectResultV1
	MissingBasisDigest() ContentDigest
	profileOnboardingUnderdeterminedResultV1()
}

type profileOnboardingUnderdeterminedResultV1 struct {
	outputRef          WorkOutputRef
	missingBasisDigest ContentDigest
}

func (profileOnboardingUnderdeterminedResultV1) profileOnboardingEffectResultV1()          {}
func (profileOnboardingUnderdeterminedResultV1) profileOnboardingUnderdeterminedResultV1() {}

func NewProfileOnboardingUnderdeterminedResultV1(
	outputRef WorkOutputRef,
	missingBasisDigest ContentDigest,
) (ProfileOnboardingUnderdeterminedResultV1, error) {
	value := profileOnboardingUnderdeterminedResultV1{
		outputRef:          outputRef,
		missingBasisDigest: missingBasisDigest,
	}
	return canonicalProfileOnboardingUnderdeterminedResultV1(value)
}

func (profileOnboardingUnderdeterminedResultV1) Kind() ProfileOnboardingResultKindV1 {
	return ProfileOnboardingResultKindV1{value: profileOnboardingUnderdeterminedKindV1Value}
}

func (result profileOnboardingUnderdeterminedResultV1) OutputRef() WorkOutputRef {
	return result.outputRef
}

func (result profileOnboardingUnderdeterminedResultV1) MissingBasisDigest() ContentDigest {
	return result.missingBasisDigest
}

// ProfileOnboardingEffectV1 is the local Work-effect claim. The Work remains
// the dated occurrence; this object binds its result and state-change witness
// to affected EntityOfConcern refs and A.10 evidence paths.
type ProfileOnboardingEffectV1 interface {
	Ref() ProfileOnboardingEffectRefV1
	WorkRecordRef() ProfileOnboardingWorkRecordRef
	WorkRef() WorkRef
	WorkRecordDigest() ContentDigest
	Result() ProfileOnboardingEffectResultV1
	AffectedEntityRefs() []EntityRef
	StatePlaneRef() StatePlaneRef
	StateWitness() WorkStateTransitionV1
	EvidencePathRefs() []EvidenceProvenancePathRefV1
	profileOnboardingEffectV1()
}

type profileOnboardingEffectV1 struct {
	ref                ProfileOnboardingEffectRefV1
	workRecordRef      ProfileOnboardingWorkRecordRef
	workRef            WorkRef
	workRecordDigest   ContentDigest
	result             ProfileOnboardingEffectResultV1
	affectedEntityRefs []EntityRef
	statePlaneRef      StatePlaneRef
	stateWitness       WorkStateTransitionV1
	evidencePathRefs   []EvidenceProvenancePathRefV1
}

func (profileOnboardingEffectV1) profileOnboardingEffectV1() {}

func NewProfileOnboardingEffectV1(
	ref ProfileOnboardingEffectRefV1,
	workRecordRef ProfileOnboardingWorkRecordRef,
	workRef WorkRef,
	workRecordDigest ContentDigest,
	result ProfileOnboardingEffectResultV1,
	affectedEntityRefs []EntityRef,
	statePlaneRef StatePlaneRef,
	stateWitness WorkStateTransitionV1,
	evidencePathRefs []EvidenceProvenancePathRefV1,
) (ProfileOnboardingEffectV1, error) {
	value := profileOnboardingEffectV1{
		ref:                ref,
		workRecordRef:      workRecordRef,
		workRef:            workRef,
		workRecordDigest:   workRecordDigest,
		result:             result,
		affectedEntityRefs: append([]EntityRef{}, affectedEntityRefs...),
		statePlaneRef:      statePlaneRef,
		stateWitness:       stateWitness,
		evidencePathRefs:   append([]EvidenceProvenancePathRefV1{}, evidencePathRefs...),
	}
	return canonicalProfileOnboardingEffectV1(value)
}

func (effect profileOnboardingEffectV1) Ref() ProfileOnboardingEffectRefV1 {
	return effect.ref
}

func (effect profileOnboardingEffectV1) WorkRecordRef() ProfileOnboardingWorkRecordRef {
	return effect.workRecordRef
}

func (effect profileOnboardingEffectV1) WorkRef() WorkRef { return effect.workRef }

func (effect profileOnboardingEffectV1) WorkRecordDigest() ContentDigest {
	return effect.workRecordDigest
}

func (effect profileOnboardingEffectV1) Result() ProfileOnboardingEffectResultV1 {
	return effect.result
}

func (effect profileOnboardingEffectV1) AffectedEntityRefs() []EntityRef {
	return append([]EntityRef{}, effect.affectedEntityRefs...)
}

func (effect profileOnboardingEffectV1) StatePlaneRef() StatePlaneRef {
	return effect.statePlaneRef
}

func (effect profileOnboardingEffectV1) StateWitness() WorkStateTransitionV1 {
	return effect.stateWitness
}

func (effect profileOnboardingEffectV1) EvidencePathRefs() []EvidenceProvenancePathRefV1 {
	return append([]EvidenceProvenancePathRefV1{}, effect.evidencePathRefs...)
}

// ProfileOnboardingAcceptanceVerdictV1 is a comparator result. It is neither
// the Work result nor evidence for itself.
type ProfileOnboardingAcceptanceVerdictV1 interface {
	Kind() string
	profileOnboardingAcceptanceVerdictV1()
}

type ProfileOnboardingAcceptancePassedV1 interface {
	ProfileOnboardingAcceptanceVerdictV1
	profileOnboardingAcceptancePassedV1()
}

type profileOnboardingAcceptancePassedV1 struct{}

func (profileOnboardingAcceptancePassedV1) profileOnboardingAcceptanceVerdictV1() {}
func (profileOnboardingAcceptancePassedV1) profileOnboardingAcceptancePassedV1()  {}
func (profileOnboardingAcceptancePassedV1) Kind() string                          { return "passed" }

func ProfileOnboardingAcceptancePassedV1Value() ProfileOnboardingAcceptancePassedV1 {
	return profileOnboardingAcceptancePassedV1{}
}

type ProfileOnboardingAcceptanceFailedV1 interface {
	ProfileOnboardingAcceptanceVerdictV1
	ReasonRef() ProfileOnboardingAcceptanceReasonRefV1
	profileOnboardingAcceptanceFailedV1()
}

type profileOnboardingAcceptanceFailedV1 struct {
	reasonRef ProfileOnboardingAcceptanceReasonRefV1
}

func (profileOnboardingAcceptanceFailedV1) profileOnboardingAcceptanceVerdictV1() {}
func (profileOnboardingAcceptanceFailedV1) profileOnboardingAcceptanceFailedV1()  {}
func (profileOnboardingAcceptanceFailedV1) Kind() string                          { return "failed" }
func (verdict profileOnboardingAcceptanceFailedV1) ReasonRef() ProfileOnboardingAcceptanceReasonRefV1 {
	return verdict.reasonRef
}

func NewProfileOnboardingAcceptanceFailedV1(
	reasonRef ProfileOnboardingAcceptanceReasonRefV1,
) (ProfileOnboardingAcceptanceFailedV1, error) {
	if !reasonRef.valid() {
		return nil, fmt.Errorf("failed acceptance verdict requires a reason ref")
	}
	return profileOnboardingAcceptanceFailedV1{reasonRef: reasonRef}, nil
}

type ProfileOnboardingAcceptanceUndeterminedV1 interface {
	ProfileOnboardingAcceptanceVerdictV1
	MissingBasisDigest() ContentDigest
	profileOnboardingAcceptanceUndeterminedV1()
}

type profileOnboardingAcceptanceUndeterminedV1 struct {
	missingBasisDigest ContentDigest
}

func (profileOnboardingAcceptanceUndeterminedV1) profileOnboardingAcceptanceVerdictV1() {}
func (profileOnboardingAcceptanceUndeterminedV1) profileOnboardingAcceptanceUndeterminedV1() {
}
func (profileOnboardingAcceptanceUndeterminedV1) Kind() string { return "undetermined" }
func (verdict profileOnboardingAcceptanceUndeterminedV1) MissingBasisDigest() ContentDigest {
	return verdict.missingBasisDigest
}

func NewProfileOnboardingAcceptanceUndeterminedV1(
	missingBasisDigest ContentDigest,
) (ProfileOnboardingAcceptanceUndeterminedV1, error) {
	if !missingBasisDigest.valid() {
		return nil, fmt.Errorf("undetermined acceptance verdict requires a missing-basis digest")
	}
	return profileOnboardingAcceptanceUndeterminedV1{
		missingBasisDigest: missingBasisDigest,
	}, nil
}

// ProfileOnboardingOutcomeAssessmentV1 binds one effect to a pinned
// acceptance standard, comparator edition, verdict, and evidence paths. It is
// not the effect, the Work, the standard, the comparator, or the evidence.
type ProfileOnboardingOutcomeAssessmentV1 interface {
	Ref() ProfileOnboardingOutcomeAssessmentRefV1
	EffectRef() ProfileOnboardingEffectRefV1
	EffectDigest() ContentDigest
	WorkRecordRef() ProfileOnboardingWorkRecordRef
	WorkRef() WorkRef
	WorkRecordDigest() ContentDigest
	AcceptanceStandardRef() ProfileOnboardingAcceptanceStandardRefV1
	AcceptanceStandardEdition() ProfileOnboardingAcceptanceStandardEditionV1
	ComparatorRef() ProfileOnboardingComparatorRefV1
	ComparatorEdition() ProfileOnboardingComparatorEditionV1
	Verdict() ProfileOnboardingAcceptanceVerdictV1
	EvidencePathRefs() []EvidenceProvenancePathRefV1
	profileOnboardingOutcomeAssessmentV1()
}

type profileOnboardingOutcomeAssessmentV1 struct {
	ref                       ProfileOnboardingOutcomeAssessmentRefV1
	effectRef                 ProfileOnboardingEffectRefV1
	effectDigest              ContentDigest
	workRecordRef             ProfileOnboardingWorkRecordRef
	workRef                   WorkRef
	workRecordDigest          ContentDigest
	acceptanceStandardRef     ProfileOnboardingAcceptanceStandardRefV1
	acceptanceStandardEdition ProfileOnboardingAcceptanceStandardEditionV1
	comparatorRef             ProfileOnboardingComparatorRefV1
	comparatorEdition         ProfileOnboardingComparatorEditionV1
	verdict                   ProfileOnboardingAcceptanceVerdictV1
	evidencePathRefs          []EvidenceProvenancePathRefV1
}

func (profileOnboardingOutcomeAssessmentV1) profileOnboardingOutcomeAssessmentV1() {}

func NewProfileOnboardingOutcomeAssessmentV1(
	ref ProfileOnboardingOutcomeAssessmentRefV1,
	effect ProfileOnboardingEffectV1,
	acceptanceStandardRef ProfileOnboardingAcceptanceStandardRefV1,
	acceptanceStandardEdition ProfileOnboardingAcceptanceStandardEditionV1,
	comparatorRef ProfileOnboardingComparatorRefV1,
	comparatorEdition ProfileOnboardingComparatorEditionV1,
	verdict ProfileOnboardingAcceptanceVerdictV1,
	evidencePathRefs []EvidenceProvenancePathRefV1,
) (ProfileOnboardingOutcomeAssessmentV1, error) {
	exactEffect, err := exactProfileOnboardingEffectV1(effect)
	if err != nil {
		return nil, err
	}
	effectDigest, err := DigestProfileOnboardingEffectV1(exactEffect)
	if err != nil {
		return nil, err
	}
	value := profileOnboardingOutcomeAssessmentV1{
		ref:                       ref,
		effectRef:                 exactEffect.ref,
		effectDigest:              effectDigest,
		workRecordRef:             exactEffect.workRecordRef,
		workRef:                   exactEffect.workRef,
		workRecordDigest:          exactEffect.workRecordDigest,
		acceptanceStandardRef:     acceptanceStandardRef,
		acceptanceStandardEdition: acceptanceStandardEdition,
		comparatorRef:             comparatorRef,
		comparatorEdition:         comparatorEdition,
		verdict:                   verdict,
		evidencePathRefs:          append([]EvidenceProvenancePathRefV1{}, evidencePathRefs...),
	}
	canonical, err := canonicalProfileOnboardingOutcomeAssessmentV1(value)
	if err != nil {
		return nil, err
	}
	err = validateOutcomeAssessmentVerdictAgainstEffectV1(canonical.verdict, exactEffect.result)
	if err != nil {
		return nil, err
	}
	return canonical, nil
}

func (assessment profileOnboardingOutcomeAssessmentV1) Ref() ProfileOnboardingOutcomeAssessmentRefV1 {
	return assessment.ref
}
func (assessment profileOnboardingOutcomeAssessmentV1) EffectRef() ProfileOnboardingEffectRefV1 {
	return assessment.effectRef
}
func (assessment profileOnboardingOutcomeAssessmentV1) EffectDigest() ContentDigest {
	return assessment.effectDigest
}
func (assessment profileOnboardingOutcomeAssessmentV1) WorkRecordRef() ProfileOnboardingWorkRecordRef {
	return assessment.workRecordRef
}
func (assessment profileOnboardingOutcomeAssessmentV1) WorkRef() WorkRef {
	return assessment.workRef
}
func (assessment profileOnboardingOutcomeAssessmentV1) WorkRecordDigest() ContentDigest {
	return assessment.workRecordDigest
}
func (assessment profileOnboardingOutcomeAssessmentV1) AcceptanceStandardRef() ProfileOnboardingAcceptanceStandardRefV1 {
	return assessment.acceptanceStandardRef
}
func (assessment profileOnboardingOutcomeAssessmentV1) AcceptanceStandardEdition() ProfileOnboardingAcceptanceStandardEditionV1 {
	return assessment.acceptanceStandardEdition
}
func (assessment profileOnboardingOutcomeAssessmentV1) ComparatorRef() ProfileOnboardingComparatorRefV1 {
	return assessment.comparatorRef
}
func (assessment profileOnboardingOutcomeAssessmentV1) ComparatorEdition() ProfileOnboardingComparatorEditionV1 {
	return assessment.comparatorEdition
}
func (assessment profileOnboardingOutcomeAssessmentV1) Verdict() ProfileOnboardingAcceptanceVerdictV1 {
	return assessment.verdict
}
func (assessment profileOnboardingOutcomeAssessmentV1) EvidencePathRefs() []EvidenceProvenancePathRefV1 {
	return append([]EvidenceProvenancePathRefV1{}, assessment.evidencePathRefs...)
}

func ValidateProfileOnboardingEffectV1AgainstWorkRecord(
	effect ProfileOnboardingEffectV1,
	record ProfileOnboardingWorkRecord,
) error {
	exact, err := exactProfileOnboardingEffectV1(effect)
	if err != nil {
		return err
	}
	canonicalRecord, err := canonicalizeProfileOnboardingWorkRecord(record)
	if err != nil {
		return err
	}
	recordDigest, err := DigestProfileOnboardingWorkRecord(canonicalRecord)
	if err != nil {
		return err
	}
	if exact.workRecordRef != canonicalRecord.recordRef || exact.workRef != canonicalRecord.workRef {
		return fmt.Errorf("ProfileOnboardingEffect points to another Work occurrence")
	}
	if exact.workRecordDigest != recordDigest {
		return fmt.Errorf("ProfileOnboardingEffect Work-record digest does not match")
	}
	affected := entityOfConcernRefStringsV1(exact.affectedEntityRefs)
	workAffected := affectedReferentStrings(canonicalRecord.affectedRefs)
	if !slices.Equal(affected, workAffected) {
		return fmt.Errorf("ProfileOnboardingEffect affected EntityOfConcern refs do not match Work")
	}
	if exact.statePlaneRef != canonicalRecord.statePlaneRef {
		return fmt.Errorf("ProfileOnboardingEffect state plane does not match Work")
	}
	if !sameWorkStateTransitionV1(exact.stateWitness, canonicalRecord.stateTransition) {
		return fmt.Errorf("ProfileOnboardingEffect state witness does not match Work")
	}
	outputRefs := workOutputStrings(canonicalRecord.outputRefs)
	if !slices.Contains(outputRefs, exact.result.OutputRef().String()) {
		return fmt.Errorf("ProfileOnboardingEffect result output is not a Work output")
	}
	return validateEffectResultAgainstWorkOutcomeV1(exact.result, canonicalRecord.outcome)
}

func ValidateProfileOnboardingEffectV1AgainstObservedProjectBasis(
	effect ProfileOnboardingEffectV1,
	basis ObservedProjectBasisV1,
) error {
	exactEffect, err := exactProfileOnboardingEffectV1(effect)
	if err != nil {
		return err
	}
	exactBasis, err := exactObservedProjectBasisV1(basis)
	if err != nil {
		return err
	}
	result, ok := exactEffect.result.(profileOnboardingCandidateResultV1)
	if !ok {
		return fmt.Errorf("underdetermined ProfileOnboardingEffect does not bind an ObservedProjectBasis")
	}
	basisDigest, err := DigestObservedProjectBasisV1(exactBasis)
	if err != nil {
		return err
	}
	if result.observedProjectBasisRef != exactBasis.ref {
		return fmt.Errorf("ProfileOnboardingEffect points to another ObservedProjectBasis")
	}
	if result.observedProjectBasisDigest != basisDigest {
		return fmt.Errorf("ProfileOnboardingEffect ObservedProjectBasis digest does not match")
	}
	return nil
}

func ValidateProfileOnboardingOutcomeAssessmentV1AgainstEffect(
	assessment ProfileOnboardingOutcomeAssessmentV1,
	effect ProfileOnboardingEffectV1,
) error {
	exactAssessment, err := exactProfileOnboardingOutcomeAssessmentV1(assessment)
	if err != nil {
		return err
	}
	exactEffect, err := exactProfileOnboardingEffectV1(effect)
	if err != nil {
		return err
	}
	effectDigest, err := DigestProfileOnboardingEffectV1(exactEffect)
	if err != nil {
		return err
	}
	if exactAssessment.effectRef != exactEffect.ref || exactAssessment.effectDigest != effectDigest {
		return fmt.Errorf("outcome assessment does not bind the exact ProfileOnboardingEffect")
	}
	if exactAssessment.workRecordRef != exactEffect.workRecordRef || exactAssessment.workRef != exactEffect.workRef {
		return fmt.Errorf("outcome assessment Work identity does not match effect")
	}
	if exactAssessment.workRecordDigest != exactEffect.workRecordDigest {
		return fmt.Errorf("outcome assessment Work-record digest does not match effect")
	}
	return validateOutcomeAssessmentVerdictAgainstEffectV1(
		exactAssessment.verdict,
		exactEffect.result,
	)
}

func canonicalProfileOnboardingCandidateResultV1(
	value profileOnboardingCandidateResultV1,
) (profileOnboardingCandidateResultV1, error) {
	if !value.outputRef.valid() || !value.payloadDigest.valid() {
		return profileOnboardingCandidateResultV1{}, fmt.Errorf("candidate effect result output ref and payload digest are required")
	}
	if !value.observedProjectBasisRef.valid() || !value.observedProjectBasisDigest.valid() {
		return profileOnboardingCandidateResultV1{}, fmt.Errorf("candidate effect result ObservedProjectBasis ref and digest are required")
	}
	return value, nil
}

func canonicalProfileOnboardingUnderdeterminedResultV1(
	value profileOnboardingUnderdeterminedResultV1,
) (profileOnboardingUnderdeterminedResultV1, error) {
	if !value.outputRef.valid() || !value.missingBasisDigest.valid() {
		return profileOnboardingUnderdeterminedResultV1{}, fmt.Errorf("underdetermined effect result output ref and missing-basis digest are required")
	}
	return value, nil
}

func exactProfileOnboardingEffectResultV1(
	value ProfileOnboardingEffectResultV1,
) (ProfileOnboardingEffectResultV1, error) {
	switch exact := value.(type) {
	case profileOnboardingCandidateResultV1:
		return canonicalProfileOnboardingCandidateResultV1(exact)
	case profileOnboardingUnderdeterminedResultV1:
		return canonicalProfileOnboardingUnderdeterminedResultV1(exact)
	default:
		return nil, fmt.Errorf("ProfileOnboardingEffect result must be a package-owned v1 variant")
	}
}

func canonicalProfileOnboardingEffectV1(
	value profileOnboardingEffectV1,
) (profileOnboardingEffectV1, error) {
	if !value.ref.valid() || !value.workRecordRef.valid() || !value.workRef.valid() {
		return profileOnboardingEffectV1{}, fmt.Errorf("ProfileOnboardingEffect and Work refs are required")
	}
	if !value.workRecordDigest.valid() {
		return profileOnboardingEffectV1{}, fmt.Errorf("ProfileOnboardingEffect Work-record digest is required")
	}
	result, err := exactProfileOnboardingEffectResultV1(value.result)
	if err != nil {
		return profileOnboardingEffectV1{}, err
	}
	affectedRefs := append([]EntityRef{}, value.affectedEntityRefs...)
	err = canonicalizeV1Refs(
		"ProfileOnboardingEffect affected EntityOfConcern refs",
		affectedRefs,
		func(ref EntityRef) string { return ref.String() },
		func(ref EntityRef) bool { return ref.valid() },
	)
	if err != nil {
		return profileOnboardingEffectV1{}, err
	}
	if !value.statePlaneRef.valid() {
		return profileOnboardingEffectV1{}, fmt.Errorf("ProfileOnboardingEffect StatePlane ref is required")
	}
	err = validateWorkStateTransition(value.stateWitness)
	if err != nil {
		return profileOnboardingEffectV1{}, err
	}
	evidenceRefs := append([]EvidenceProvenancePathRefV1{}, value.evidencePathRefs...)
	err = canonicalizeV1Refs(
		"ProfileOnboardingEffect evidence-provenance path refs",
		evidenceRefs,
		func(ref EvidenceProvenancePathRefV1) string { return ref.String() },
		func(ref EvidenceProvenancePathRefV1) bool { return ref.valid() },
	)
	if err != nil {
		return profileOnboardingEffectV1{}, err
	}
	value.result = result
	value.affectedEntityRefs = affectedRefs
	value.evidencePathRefs = evidenceRefs
	return value, nil
}

func exactProfileOnboardingEffectV1(
	value ProfileOnboardingEffectV1,
) (profileOnboardingEffectV1, error) {
	exact, ok := value.(profileOnboardingEffectV1)
	if !ok {
		return profileOnboardingEffectV1{}, fmt.Errorf("ProfileOnboardingEffect must be the package-owned v1 value")
	}
	return canonicalProfileOnboardingEffectV1(exact)
}

func exactProfileOnboardingAcceptanceVerdictV1(
	value ProfileOnboardingAcceptanceVerdictV1,
) (ProfileOnboardingAcceptanceVerdictV1, error) {
	switch exact := value.(type) {
	case profileOnboardingAcceptancePassedV1:
		return exact, nil
	case profileOnboardingAcceptanceFailedV1:
		if !exact.reasonRef.valid() {
			return nil, fmt.Errorf("failed acceptance verdict reason ref is invalid")
		}
		return exact, nil
	case profileOnboardingAcceptanceUndeterminedV1:
		if !exact.missingBasisDigest.valid() {
			return nil, fmt.Errorf("undetermined acceptance verdict missing-basis digest is invalid")
		}
		return exact, nil
	default:
		return nil, fmt.Errorf("acceptance verdict must be a package-owned v1 variant")
	}
}

func canonicalProfileOnboardingOutcomeAssessmentV1(
	value profileOnboardingOutcomeAssessmentV1,
) (profileOnboardingOutcomeAssessmentV1, error) {
	if !value.ref.valid() || !value.effectRef.valid() || !value.effectDigest.valid() {
		return profileOnboardingOutcomeAssessmentV1{}, fmt.Errorf("outcome-assessment and effect refs/digest are required")
	}
	if !value.workRecordRef.valid() || !value.workRef.valid() || !value.workRecordDigest.valid() {
		return profileOnboardingOutcomeAssessmentV1{}, fmt.Errorf("outcome assessment Work refs/digest are required")
	}
	contract, err := ProfileOnboardingMethodContractV1Value()
	if err != nil {
		return profileOnboardingOutcomeAssessmentV1{}, err
	}
	if value.acceptanceStandardRef != contract.AcceptanceStandardRef() {
		return profileOnboardingOutcomeAssessmentV1{}, fmt.Errorf("outcome assessment must use the v1 method-contract acceptance standard")
	}
	if !value.acceptanceStandardEdition.valid() {
		return profileOnboardingOutcomeAssessmentV1{}, fmt.Errorf("outcome assessment acceptance-standard edition is invalid")
	}
	if value.acceptanceStandardEdition.String() != contract.AcceptanceStandardEdition() {
		return profileOnboardingOutcomeAssessmentV1{}, fmt.Errorf("outcome assessment must pin the v1 method-contract acceptance-standard edition")
	}
	if !value.comparatorRef.valid() || !value.comparatorEdition.valid() {
		return profileOnboardingOutcomeAssessmentV1{}, fmt.Errorf("outcome assessment comparator ref and edition are required")
	}
	verdict, err := exactProfileOnboardingAcceptanceVerdictV1(value.verdict)
	if err != nil {
		return profileOnboardingOutcomeAssessmentV1{}, err
	}
	evidenceRefs := append([]EvidenceProvenancePathRefV1{}, value.evidencePathRefs...)
	err = canonicalizeV1Refs(
		"outcome assessment evidence-provenance path refs",
		evidenceRefs,
		func(ref EvidenceProvenancePathRefV1) string { return ref.String() },
		func(ref EvidenceProvenancePathRefV1) bool { return ref.valid() },
	)
	if err != nil {
		return profileOnboardingOutcomeAssessmentV1{}, err
	}
	value.verdict = verdict
	value.evidencePathRefs = evidenceRefs
	return value, nil
}

func exactProfileOnboardingOutcomeAssessmentV1(
	value ProfileOnboardingOutcomeAssessmentV1,
) (profileOnboardingOutcomeAssessmentV1, error) {
	exact, ok := value.(profileOnboardingOutcomeAssessmentV1)
	if !ok {
		return profileOnboardingOutcomeAssessmentV1{}, fmt.Errorf("outcome assessment must be the package-owned v1 value")
	}
	return canonicalProfileOnboardingOutcomeAssessmentV1(exact)
}

func validateEffectResultAgainstWorkOutcomeV1(
	result ProfileOnboardingEffectResultV1,
	outcome ProfileOnboardingWorkOutcomeV1,
) error {
	operation, err := exactProfileOnboardingWorkOutcomeOperationV1(outcome)
	if err != nil {
		return err
	}
	switch exactResult := result.(type) {
	case profileOnboardingCandidateResultV1:
		if operation.resultKind.String() != profileOnboardingCandidateResultKindV1Value {
			return fmt.Errorf("candidate effect result does not match Work outcome variant")
		}
		if exactResult.payloadDigest != operation.payloadDigest {
			return fmt.Errorf("candidate effect payload digest does not match Work outcome")
		}
		if exactResult.observedProjectBasisDigest != operation.observedBasisDigest {
			return fmt.Errorf("candidate effect ObservedProjectBasis digest does not match Work outcome")
		}
		return nil
	case profileOnboardingUnderdeterminedResultV1:
		if operation.resultKind.String() != profileOnboardingUnderdeterminedKindV1Value {
			return fmt.Errorf("underdetermined effect result does not match Work outcome variant")
		}
		if exactResult.missingBasisDigest != operation.missingBasisDigest {
			return fmt.Errorf("underdetermined effect missing-basis digest does not match Work outcome")
		}
		return nil
	default:
		return fmt.Errorf("unknown ProfileOnboardingEffect result variant")
	}
}

func validateOutcomeAssessmentVerdictAgainstEffectV1(
	verdict ProfileOnboardingAcceptanceVerdictV1,
	result ProfileOnboardingEffectResultV1,
) error {
	switch exactResult := result.(type) {
	case profileOnboardingCandidateResultV1:
		switch verdict.(type) {
		case profileOnboardingAcceptancePassedV1, profileOnboardingAcceptanceFailedV1:
			return nil
		default:
			return fmt.Errorf("candidate result requires a passed or failed acceptance verdict")
		}
	case profileOnboardingUnderdeterminedResultV1:
		exactVerdict, ok := verdict.(profileOnboardingAcceptanceUndeterminedV1)
		if !ok {
			return fmt.Errorf("underdetermined result requires an undetermined acceptance verdict")
		}
		if exactVerdict.missingBasisDigest != exactResult.missingBasisDigest {
			return fmt.Errorf("undetermined verdict missing-basis digest does not match effect result")
		}
		return nil
	default:
		return fmt.Errorf("unknown ProfileOnboardingEffect result variant")
	}
}

func sameWorkStateTransitionV1(
	left WorkStateTransitionV1,
	right WorkStateTransitionV1,
) bool {
	switch leftValue := left.(type) {
	case prePostStateTransitionV1:
		rightValue, ok := right.(prePostStateTransitionV1)
		return ok && leftValue == rightValue
	case deltaStateTransitionV1:
		rightValue, ok := right.(deltaStateTransitionV1)
		return ok && leftValue == rightValue
	default:
		return false
	}
}

func entityOfConcernRefStringsV1(values []EntityRef) []string {
	return mapSliceV1Pure(values, func(value EntityRef) string {
		return value.String()
	})
}
