package projectprofile

import (
	"fmt"
	"slices"
	"strconv"
)

const (
	candidateProvenanceDigestDomainV1 = "haft.project-profile.candidate-provenance/final-v1"
	missingProfileBasisDigestDomainV1 = "haft.project-profile.missing-basis/v1"
)

// CandidateProvenanceV1 binds every reliance-bearing support object by exact
// ref+digest. It remains a provenance claim: admission resolves the objects
// and validates their relations before relying on the candidate.
type CandidateProvenanceV1 struct {
	authorityBasisRef                 ProfileDeclarationAuthorityBasisRef
	workRecordRef                     ProfileOnboardingWorkRecordRef
	workRecordDigest                  ContentDigest
	profileAuthorRoleAssignmentRef    RoleAssignmentRef
	profileAuthorRoleAssignmentDigest ContentDigest
	observedProjectBasisRef           ObservedProjectBasisRefV1
	observedBasisDigest               ContentDigest
	outcomeAssessmentRef              ProfileOnboardingOutcomeAssessmentRefV1
	outcomeAssessmentDigest           ContentDigest
	projectRoot                       ProjectRootV1
	classifierVersion                 ClassifierVersion
	policyVersion                     PolicyVersion
	sessionRef                        SessionRef
	payloadDigest                     ContentDigest
	candidateProvenanceHash           ContentDigest
}

type CandidateProvenanceV1Builder struct {
	value CandidateProvenanceV1
}

func NewCandidateProvenanceV1Builder(
	authorityBasisRef ProfileDeclarationAuthorityBasisRef,
	workRecordRef ProfileOnboardingWorkRecordRef,
	workRecordDigest ContentDigest,
) CandidateProvenanceV1Builder {
	return CandidateProvenanceV1Builder{value: CandidateProvenanceV1{
		authorityBasisRef: authorityBasisRef,
		workRecordRef:     workRecordRef,
		workRecordDigest:  workRecordDigest,
	}}
}

func (builder CandidateProvenanceV1Builder) ForProject(
	root ProjectRootV1,
) CandidateProvenanceV1Builder {
	builder.value.projectRoot = root
	return builder
}

func (builder CandidateProvenanceV1Builder) ForProfileAuthorRoleAssignment(
	ref RoleAssignmentRef,
	digest ContentDigest,
) CandidateProvenanceV1Builder {
	builder.value.profileAuthorRoleAssignmentRef = ref
	builder.value.profileAuthorRoleAssignmentDigest = digest
	return builder
}

func (builder CandidateProvenanceV1Builder) ClassifiedBy(
	classifierVersion ClassifierVersion,
	policyVersion PolicyVersion,
) CandidateProvenanceV1Builder {
	builder.value.classifierVersion = classifierVersion
	builder.value.policyVersion = policyVersion
	return builder
}

func (builder CandidateProvenanceV1Builder) InSession(
	sessionRef SessionRef,
) CandidateProvenanceV1Builder {
	builder.value.sessionRef = sessionRef
	return builder
}

func (builder CandidateProvenanceV1Builder) ForPayload(
	payloadDigest ContentDigest,
) CandidateProvenanceV1Builder {
	builder.value.payloadDigest = payloadDigest
	return builder
}

func (builder CandidateProvenanceV1Builder) ForObservedProjectBasis(
	ref ObservedProjectBasisRefV1,
	digest ContentDigest,
) CandidateProvenanceV1Builder {
	builder.value.observedProjectBasisRef = ref
	builder.value.observedBasisDigest = digest
	return builder
}

func (builder CandidateProvenanceV1Builder) ForOutcomeAssessment(
	ref ProfileOnboardingOutcomeAssessmentRefV1,
	digest ContentDigest,
) CandidateProvenanceV1Builder {
	builder.value.outcomeAssessmentRef = ref
	builder.value.outcomeAssessmentDigest = digest
	return builder
}

func (builder CandidateProvenanceV1Builder) Build() (CandidateProvenanceV1, error) {
	err := validateCandidateProvenanceV1Fields(builder.value)
	if err != nil {
		return CandidateProvenanceV1{}, err
	}
	digest := digestCandidateProvenanceV1Fields(builder.value)
	builder.value.candidateProvenanceHash = digest
	return builder.value, nil
}

func (value CandidateProvenanceV1) AuthorityBasisRef() ProfileDeclarationAuthorityBasisRef {
	return value.authorityBasisRef
}

func (value CandidateProvenanceV1) WorkRecordRef() ProfileOnboardingWorkRecordRef {
	return value.workRecordRef
}

func (value CandidateProvenanceV1) WorkRecordDigest() ContentDigest {
	return value.workRecordDigest
}

func (value CandidateProvenanceV1) ProfileAuthorRoleAssignmentRef() RoleAssignmentRef {
	return value.profileAuthorRoleAssignmentRef
}

func (value CandidateProvenanceV1) ProfileAuthorRoleAssignmentDigest() ContentDigest {
	return value.profileAuthorRoleAssignmentDigest
}

func (value CandidateProvenanceV1) ProjectRoot() ProjectRootV1 {
	return value.projectRoot
}

func (value CandidateProvenanceV1) ClassifierVersion() ClassifierVersion {
	return value.classifierVersion
}

func (value CandidateProvenanceV1) PolicyVersion() PolicyVersion {
	return value.policyVersion
}

func (value CandidateProvenanceV1) SessionRef() SessionRef {
	return value.sessionRef
}

func (value CandidateProvenanceV1) PayloadDigest() ContentDigest {
	return value.payloadDigest
}

func (value CandidateProvenanceV1) ObservedProjectBasisRef() ObservedProjectBasisRefV1 {
	return value.observedProjectBasisRef
}

func (value CandidateProvenanceV1) ObservedProjectBasisDigest() ContentDigest {
	return value.observedBasisDigest
}

func (value CandidateProvenanceV1) OutcomeAssessmentRef() ProfileOnboardingOutcomeAssessmentRefV1 {
	return value.outcomeAssessmentRef
}

func (value CandidateProvenanceV1) OutcomeAssessmentDigest() ContentDigest {
	return value.outcomeAssessmentDigest
}

func (value CandidateProvenanceV1) Digest() ContentDigest {
	return value.candidateProvenanceHash
}

func DigestCandidateProvenanceV1(value CandidateProvenanceV1) (ContentDigest, error) {
	err := validateCandidateProvenanceV1(value)
	if err != nil {
		return ContentDigest{}, err
	}
	return value.candidateProvenanceHash, nil
}

func validateCandidateProvenanceV1(value CandidateProvenanceV1) error {
	err := validateCandidateProvenanceV1Fields(value)
	if err != nil {
		return err
	}
	expected := digestCandidateProvenanceV1Fields(value)
	if value.candidateProvenanceHash != expected {
		return fmt.Errorf("candidate provenance digest is not canonical")
	}
	return nil
}

func validateCandidateProvenanceV1Fields(value CandidateProvenanceV1) error {
	if !value.authorityBasisRef.valid() {
		return fmt.Errorf("candidate provenance authority-basis ref is invalid")
	}
	if !value.workRecordRef.valid() || !value.workRecordDigest.valid() {
		return fmt.Errorf("candidate provenance Work-record ref and digest are required")
	}
	if !value.profileAuthorRoleAssignmentRef.valid() || !value.profileAuthorRoleAssignmentDigest.valid() {
		return fmt.Errorf("candidate provenance ProfileAuthorRoleAssignment ref and digest are required")
	}
	if !value.observedProjectBasisRef.valid() || !value.observedBasisDigest.valid() {
		return fmt.Errorf("candidate provenance ObservedProjectBasis ref and digest are required")
	}
	if !value.outcomeAssessmentRef.valid() || !value.outcomeAssessmentDigest.valid() {
		return fmt.Errorf("candidate provenance outcome-assessment ref and digest are required")
	}
	if !value.projectRoot.valid() {
		return fmt.Errorf("candidate provenance project root is invalid")
	}
	if !value.classifierVersion.valid() || !value.policyVersion.valid() {
		return fmt.Errorf("candidate provenance classifier and policy versions are invalid")
	}
	if !value.sessionRef.valid() {
		return fmt.Errorf("candidate provenance session ref is invalid")
	}
	if !value.payloadDigest.valid() {
		return fmt.Errorf("candidate provenance payload digest is required")
	}
	return nil
}

func digestCandidateProvenanceV1Fields(value CandidateProvenanceV1) ContentDigest {
	writer := newCanonicalDigestWriter(candidateProvenanceDigestDomainV1)
	authorityBasisRef := value.authorityBasisRef.String()
	workRecordRef := value.workRecordRef.String()
	workRecordDigest := value.workRecordDigest.String()
	assignmentRef := value.profileAuthorRoleAssignmentRef.String()
	assignmentDigest := value.profileAuthorRoleAssignmentDigest.String()
	observedProjectBasisRef := value.observedProjectBasisRef.String()
	observedProjectBasisDigest := value.observedBasisDigest.String()
	outcomeAssessmentRef := value.outcomeAssessmentRef.String()
	outcomeAssessmentDigest := value.outcomeAssessmentDigest.String()
	projectRoot := value.projectRoot.String()
	classifierVersion := value.classifierVersion.String()
	policyVersion := value.policyVersion.String()
	sessionRef := value.sessionRef.String()
	payloadDigest := value.payloadDigest.String()
	writer.add(authorityBasisRef)
	writer.add(workRecordRef)
	writer.add(workRecordDigest)
	writer.add(assignmentRef)
	writer.add(assignmentDigest)
	writer.add(observedProjectBasisRef)
	writer.add(observedProjectBasisDigest)
	writer.add(outcomeAssessmentRef)
	writer.add(outcomeAssessmentDigest)
	writer.add(projectRoot)
	writer.add(classifierVersion)
	writer.add(policyVersion)
	writer.add(sessionRef)
	writer.add(payloadDigest)
	return writer.digest()
}

type ProfileDeclarationCandidateV1 struct {
	payload    ProfileDeclarationPayload
	provenance CandidateProvenanceV1
}

func NewProfileDeclarationCandidateV1(
	payload ProfileDeclarationPayload,
	provenance CandidateProvenanceV1,
) (ProfileDeclarationCandidateV1, error) {
	if !payload.valid() {
		return ProfileDeclarationCandidateV1{}, fmt.Errorf("candidate payload is invalid")
	}
	err := validateCandidateProvenanceV1(provenance)
	if err != nil {
		return ProfileDeclarationCandidateV1{}, err
	}
	payloadDigest, err := DigestProfileDeclarationPayload(payload)
	if err != nil {
		return ProfileDeclarationCandidateV1{}, err
	}
	if payloadDigest != provenance.payloadDigest {
		return ProfileDeclarationCandidateV1{}, fmt.Errorf("candidate provenance does not bind the exact payload")
	}
	return ProfileDeclarationCandidateV1{payload: payload, provenance: provenance}, nil
}

func (candidate ProfileDeclarationCandidateV1) Payload() ProfileDeclarationPayload {
	return candidate.payload
}

func (candidate ProfileDeclarationCandidateV1) Provenance() CandidateProvenanceV1 {
	return candidate.provenance
}

func validateProfileDeclarationCandidateV1(candidate ProfileDeclarationCandidateV1) error {
	_, err := NewProfileDeclarationCandidateV1(candidate.payload, candidate.provenance)
	return err
}

// ValidateProfileDeclarationCandidateV1AgainstSupports is the reliance gate
// for an admission-bound candidate. Structural candidate JSON alone is not
// sufficient: all exact support objects must resolve, agree, and terminate in
// a passed outcome assessment.
func ValidateProfileDeclarationCandidateV1AgainstSupports(
	candidate ProfileDeclarationCandidateV1,
	record ProfileOnboardingWorkRecord,
	description ProfileOnboardingMethodDescriptionV1,
	contract ProfileOnboardingMethodContractV1,
	assignment ProfileAuthorRoleAssignmentV1,
	assignmentSupport ProfileAuthorAssignmentSupportCarrierV1,
	basis ObservedProjectBasisV1,
	effect ProfileOnboardingEffectV1,
	assessment ProfileOnboardingOutcomeAssessmentV1,
) error {
	err := validateProfileDeclarationCandidateV1(candidate)
	if err != nil {
		return err
	}
	err = ValidateProfileOnboardingWorkRecordAgainstSupportV1(
		record,
		description,
		contract,
		assignment,
		assignmentSupport,
		basis,
	)
	if err != nil {
		return err
	}
	err = matchCandidateProvenanceToWorkParameters(candidate.provenance, record.parameterBindings)
	if err != nil {
		return err
	}
	err = ValidateProfileOnboardingEffectV1AgainstWorkRecord(effect, record)
	if err != nil {
		return err
	}
	err = ValidateProfileOnboardingEffectV1AgainstObservedProjectBasis(effect, basis)
	if err != nil {
		return err
	}
	err = ValidateProfileOnboardingOutcomeAssessmentV1AgainstEffect(assessment, effect)
	if err != nil {
		return err
	}
	verdict := assessment.Verdict()
	_, passed := verdict.(profileOnboardingAcceptancePassedV1)
	if !passed {
		return fmt.Errorf("admission-bound candidate requires an exact passed outcome assessment")
	}
	if candidate.provenance.workRecordRef != record.recordRef {
		return fmt.Errorf("candidate provenance points to another Work record")
	}
	workRecordDigest, err := DigestProfileOnboardingWorkRecord(record)
	if err != nil {
		return err
	}
	if candidate.provenance.workRecordDigest != workRecordDigest {
		return fmt.Errorf("candidate provenance Work-record digest does not match")
	}
	assignmentRef := assignment.RoleAssignmentRef()
	if candidate.provenance.profileAuthorRoleAssignmentRef != assignmentRef {
		return fmt.Errorf("candidate provenance points to another ProfileAuthorRoleAssignment")
	}
	assignmentDigest, err := DigestProfileAuthorRoleAssignmentV1(assignment)
	if err != nil {
		return err
	}
	if candidate.provenance.profileAuthorRoleAssignmentDigest != assignmentDigest {
		return fmt.Errorf("candidate provenance ProfileAuthorRoleAssignment digest does not match")
	}
	exactBasis, err := exactObservedProjectBasisV1(basis)
	if err != nil {
		return err
	}
	basisDigest, err := DigestObservedProjectBasisV1(exactBasis)
	if err != nil {
		return err
	}
	if candidate.provenance.observedProjectBasisRef != exactBasis.ref {
		return fmt.Errorf("candidate provenance points to another ObservedProjectBasis")
	}
	if candidate.provenance.observedBasisDigest != basisDigest {
		return fmt.Errorf("candidate provenance ObservedProjectBasis digest does not match")
	}
	exactAssessment, err := exactProfileOnboardingOutcomeAssessmentV1(assessment)
	if err != nil {
		return err
	}
	assessmentDigest, err := DigestProfileOnboardingOutcomeAssessmentV1(exactAssessment)
	if err != nil {
		return err
	}
	if candidate.provenance.outcomeAssessmentRef != exactAssessment.ref {
		return fmt.Errorf("candidate provenance points to another outcome assessment")
	}
	if candidate.provenance.outcomeAssessmentDigest != assessmentDigest {
		return fmt.Errorf("candidate provenance outcome-assessment digest does not match")
	}
	outcome, ok := record.outcome.(CandidatePayloadProduced)
	if !ok {
		return fmt.Errorf("candidate requires CandidatePayloadProduced Work outcome")
	}
	outcomePayloadDigest := outcome.PayloadDigest()
	if outcomePayloadDigest != candidate.provenance.payloadDigest {
		return fmt.Errorf("work outcome payload digest does not match candidate")
	}
	outcomeObservedBasisDigest := outcome.ObservedBasisDigest()
	if outcomeObservedBasisDigest != candidate.provenance.observedBasisDigest {
		return fmt.Errorf("work outcome observed-basis digest does not match candidate")
	}
	effectResult := effect.Result()
	result, ok := effectResult.(profileOnboardingCandidateResultV1)
	if !ok {
		return fmt.Errorf("candidate requires CandidatePayloadProduced effect result")
	}
	if result.payloadDigest != candidate.provenance.payloadDigest {
		return fmt.Errorf("effect result payload digest does not match candidate")
	}
	resultBasisRefMatches := result.observedProjectBasisRef == candidate.provenance.observedProjectBasisRef
	resultBasisDigestMatches := result.observedProjectBasisDigest == candidate.provenance.observedBasisDigest
	if !resultBasisRefMatches || !resultBasisDigestMatches {
		return fmt.Errorf("effect result ObservedProjectBasis binding does not match candidate")
	}
	return nil
}

func ValidateProfileDeclarationCandidateV1AgainstSupportsV2(
	candidate ProfileDeclarationCandidateV1,
	record ProfileOnboardingWorkRecord,
	description ProfileOnboardingMethodDescriptionV2,
	contract ProfileOnboardingMethodContractV2,
	assignment ProfileAuthorRoleAssignmentV1,
	assignmentSupport ProfileAuthorAssignmentSupportCarrierV1,
	basis ObservedProjectBasisV1,
	effect ProfileOnboardingEffectV1,
	assessment ProfileOnboardingOutcomeAssessmentV1,
	workInputRef WorkInputRef,
) error {
	if err := validateProfileDeclarationCandidateV1(candidate); err != nil {
		return err
	}
	if err := ValidateProfileOnboardingWorkRecordAgainstSupportV2(
		record,
		description,
		contract,
		assignment,
		assignmentSupport,
		basis,
		workInputRef,
	); err != nil {
		return err
	}
	return validateProfileDeclarationCandidateAfterWorkSupportV2(
		candidate,
		record,
		assignment,
		basis,
		effect,
		assessment,
	)
}

func validateProfileDeclarationCandidateAfterWorkSupportV2(
	candidate ProfileDeclarationCandidateV1,
	record ProfileOnboardingWorkRecord,
	assignment ProfileAuthorRoleAssignmentV1,
	basis ObservedProjectBasisV1,
	effect ProfileOnboardingEffectV1,
	assessment ProfileOnboardingOutcomeAssessmentV1,
) error {
	if err := matchCandidateProvenanceToWorkParameters(candidate.provenance, record.parameterBindings); err != nil {
		return err
	}
	if err := ValidateProfileOnboardingEffectV1AgainstWorkRecord(effect, record); err != nil {
		return err
	}
	if err := ValidateProfileOnboardingEffectV1AgainstObservedProjectBasis(effect, basis); err != nil {
		return err
	}
	if err := ValidateProfileOnboardingOutcomeAssessmentV1AgainstEffect(assessment, effect); err != nil {
		return err
	}
	verdict := assessment.Verdict()
	if _, passed := verdict.(profileOnboardingAcceptancePassedV1); !passed {
		return fmt.Errorf("admission-bound candidate requires an exact passed outcome assessment")
	}
	if candidate.provenance.workRecordRef != record.recordRef {
		return fmt.Errorf("candidate provenance points to another Work record")
	}
	workRecordDigest, err := DigestProfileOnboardingWorkRecord(record)
	if err != nil {
		return err
	}
	if candidate.provenance.workRecordDigest != workRecordDigest {
		return fmt.Errorf("candidate provenance Work-record digest does not match")
	}
	assignmentRef := assignment.RoleAssignmentRef()
	if candidate.provenance.profileAuthorRoleAssignmentRef != assignmentRef {
		return fmt.Errorf("candidate provenance points to another ProfileAuthorRoleAssignment")
	}
	assignmentDigest, err := DigestProfileAuthorRoleAssignmentV1(assignment)
	if err != nil {
		return err
	}
	if candidate.provenance.profileAuthorRoleAssignmentDigest != assignmentDigest {
		return fmt.Errorf("candidate provenance ProfileAuthorRoleAssignment digest does not match")
	}
	exactBasis, err := exactObservedProjectBasisV1(basis)
	if err != nil {
		return err
	}
	basisDigest, err := DigestObservedProjectBasisV1(exactBasis)
	if err != nil {
		return err
	}
	if candidate.provenance.observedProjectBasisRef != exactBasis.ref {
		return fmt.Errorf("candidate provenance points to another ObservedProjectBasis")
	}
	if candidate.provenance.observedBasisDigest != basisDigest {
		return fmt.Errorf("candidate provenance ObservedProjectBasis digest does not match")
	}
	exactAssessment, err := exactProfileOnboardingOutcomeAssessmentV1(assessment)
	if err != nil {
		return err
	}
	assessmentDigest, err := DigestProfileOnboardingOutcomeAssessmentV1(exactAssessment)
	if err != nil {
		return err
	}
	if candidate.provenance.outcomeAssessmentRef != exactAssessment.ref {
		return fmt.Errorf("candidate provenance points to another outcome assessment")
	}
	if candidate.provenance.outcomeAssessmentDigest != assessmentDigest {
		return fmt.Errorf("candidate provenance outcome-assessment digest does not match")
	}
	outcome, ok := record.outcome.(CandidatePayloadProduced)
	if !ok {
		return fmt.Errorf("candidate requires CandidatePayloadProduced Work outcome")
	}
	if outcome.PayloadDigest() != candidate.provenance.payloadDigest {
		return fmt.Errorf("work outcome payload digest does not match candidate")
	}
	if outcome.ObservedBasisDigest() != candidate.provenance.observedBasisDigest {
		return fmt.Errorf("work outcome observed-basis digest does not match candidate")
	}
	effectResult := effect.Result()
	result, ok := effectResult.(profileOnboardingCandidateResultV1)
	if !ok {
		return fmt.Errorf("candidate requires CandidatePayloadProduced effect result")
	}
	if result.payloadDigest != candidate.provenance.payloadDigest {
		return fmt.Errorf("effect result payload digest does not match candidate")
	}
	resultBasisRefMatches := result.observedProjectBasisRef == candidate.provenance.observedProjectBasisRef
	resultBasisDigestMatches := result.observedProjectBasisDigest == candidate.provenance.observedBasisDigest
	if !resultBasisRefMatches || !resultBasisDigestMatches {
		return fmt.Errorf("effect result ObservedProjectBasis binding does not match candidate")
	}
	return nil
}

func matchCandidateProvenanceToWorkParameters(
	provenance CandidateProvenanceV1,
	bindings MethodParameterBindings,
) error {
	classifierVersion := provenance.classifierVersion.String()
	err := matchWorkParameterV1(bindings, profileOnboardingClassifierParameterV1, classifierVersion)
	if err != nil {
		return err
	}
	policyVersion := provenance.policyVersion.String()
	err = matchWorkParameterV1(bindings, profileOnboardingPolicyParameterV1, policyVersion)
	if err != nil {
		return err
	}
	projectRoot := provenance.projectRoot.String()
	err = matchWorkParameterV1(bindings, profileOnboardingProjectRootParameterV1, projectRoot)
	if err != nil {
		return err
	}
	sessionRef := provenance.sessionRef.String()
	return matchWorkParameterV1(bindings, profileOnboardingSessionParameterV1, sessionRef)
}

func matchWorkParameterV1(
	bindings MethodParameterBindings,
	name string,
	expectedValue string,
) error {
	actualValue, found := bindings.ValueFor(name)
	if !found || actualValue != expectedValue {
		return fmt.Errorf("work parameter %q does not match candidate provenance", name)
	}
	return nil
}

type MissingProfileBasisSetV1 struct {
	values []MissingProfileBasis
}

func NewMissingProfileBasisSetV1(
	values []MissingProfileBasis,
) (MissingProfileBasisSetV1, error) {
	if len(values) == 0 {
		return MissingProfileBasisSetV1{}, fmt.Errorf("missing profile basis must not be empty")
	}
	canonical := append([]MissingProfileBasis{}, values...)
	slices.Sort(canonical)
	err := visitSliceV1(canonical, func(_ int, value MissingProfileBasis) error {
		if !knownMissingProfileBasis(value) {
			return fmt.Errorf("unknown missing profile basis %q", value)
		}
		return nil
	})
	if err != nil {
		return MissingProfileBasisSetV1{}, err
	}
	err = visitAdjacentV1(canonical, func(previous MissingProfileBasis, current MissingProfileBasis) error {
		if previous == current {
			return fmt.Errorf("duplicate missing profile basis %q", current)
		}
		return nil
	})
	if err != nil {
		return MissingProfileBasisSetV1{}, err
	}
	return MissingProfileBasisSetV1{values: canonical}, nil
}

func (set MissingProfileBasisSetV1) Values() []MissingProfileBasis {
	return append([]MissingProfileBasis{}, set.values...)
}

func DigestMissingProfileBasisSetV1(set MissingProfileBasisSetV1) (ContentDigest, error) {
	validated, err := NewMissingProfileBasisSetV1(set.values)
	if err != nil {
		return ContentDigest{}, err
	}
	writer := newCanonicalDigestWriter(missingProfileBasisDigestDomainV1)
	values := validated.Values()
	count := len(values)
	valueCount := strconv.Itoa(count)
	writer.add(valueCount)
	visitSliceV1Pure(values, func(value MissingProfileBasis) {
		text := string(value)
		writer.add(text)
	})
	return writer.digest(), nil
}

func knownMissingProfileBasis(value MissingProfileBasis) bool {
	switch value {
	case MissingObservedProjectBasis:
		return true
	case MissingStableScopeIdentity:
		return true
	case MissingClassificationBasis:
		return true
	default:
		return false
	}
}

type ProfileClassificationResult interface {
	profileClassificationResultVariant()
}

type ProfileClassificationCandidate interface {
	ProfileClassificationResult
	Candidate() ProfileDeclarationCandidateV1
	profileClassificationCandidateVariant()
}

type ProfileClassificationUnderdetermined interface {
	ProfileClassificationResult
	WorkRecordRef() ProfileOnboardingWorkRecordRef
	MissingBasis() MissingProfileBasisSetV1
	profileClassificationUnderdeterminedVariant()
}
