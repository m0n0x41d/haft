package projectprofile

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
)

const (
	profileOnboardingMethodRefV1Value            = "haft:method:profile-onboarding/v1"
	profileOnboardingMethodDescriptionRefV1Value = "haft:method-description:profile-onboarding/v1"
	profileAuthorRoleRefV1Value                  = "haft:role:profile-author/v1"
	profileOnboardingWorkRecordDigestDomainV1    = "haft.project-profile.onboarding-work-record/v1"
	profileOnboardingWorkRecordDigestDomainV2    = "haft.project-profile.onboarding-work-record/v2"
	profileOnboardingClassifierParameterV1       = "classifier_version"
	profileOnboardingPolicyParameterV1           = "policy_version"
	profileOnboardingProjectRootParameterV1      = "project_root"
	profileOnboardingSessionParameterV1          = "session_ref"
)

func ProfileOnboardingMethodRefV1() MethodRef {
	return MethodRef{v1Reference: v1Reference{value: profileOnboardingMethodRefV1Value}}
}

func ProfileOnboardingMethodDescriptionRefV1() MethodDescriptionRef {
	return MethodDescriptionRef{
		v1Reference: v1Reference{value: profileOnboardingMethodDescriptionRefV1Value},
	}
}

func ProfileAuthorRoleRefV1() RoleRef {
	return RoleRef{v1Reference: v1Reference{value: profileAuthorRoleRefV1Value}}
}

type WorkStateTransitionV1 interface {
	workStateTransitionV1Variant()
	PreStateRef() StateRef
}

type PrePostStateTransitionV1 interface {
	WorkStateTransitionV1
	PostStateRef() StateRef
	prePostStateTransitionV1Variant()
}

type prePostStateTransitionV1 struct {
	preStateRef  StateRef
	postStateRef StateRef
}

func NewPrePostStateTransitionV1(
	preStateRef StateRef,
	postStateRef StateRef,
) (PrePostStateTransitionV1, error) {
	if !preStateRef.valid() || !postStateRef.valid() {
		return nil, fmt.Errorf("pre-state and post-state refs are required")
	}
	if preStateRef == postStateRef {
		return nil, fmt.Errorf("equal pre/post state refs require an explicit no-op Work variant")
	}
	return prePostStateTransitionV1{
		preStateRef:  preStateRef,
		postStateRef: postStateRef,
	}, nil
}

func (prePostStateTransitionV1) workStateTransitionV1Variant()    {}
func (prePostStateTransitionV1) prePostStateTransitionV1Variant() {}

func (transition prePostStateTransitionV1) PreStateRef() StateRef {
	return transition.preStateRef
}

func (transition prePostStateTransitionV1) PostStateRef() StateRef {
	return transition.postStateRef
}

type DeltaStateTransitionV1 interface {
	WorkStateTransitionV1
	DeltaPredicateRef() DeltaPredicateRef
	deltaStateTransitionV1Variant()
}

type deltaStateTransitionV1 struct {
	preStateRef       StateRef
	deltaPredicateRef DeltaPredicateRef
}

func NewDeltaStateTransitionV1(
	preStateRef StateRef,
	deltaPredicateRef DeltaPredicateRef,
) (DeltaStateTransitionV1, error) {
	if !preStateRef.valid() || !deltaPredicateRef.valid() {
		return nil, fmt.Errorf("pre-state and delta-predicate refs are required")
	}
	return deltaStateTransitionV1{
		preStateRef:       preStateRef,
		deltaPredicateRef: deltaPredicateRef,
	}, nil
}

func (deltaStateTransitionV1) workStateTransitionV1Variant()  {}
func (deltaStateTransitionV1) deltaStateTransitionV1Variant() {}

func (transition deltaStateTransitionV1) PreStateRef() StateRef {
	return transition.preStateRef
}

func (transition deltaStateTransitionV1) DeltaPredicateRef() DeltaPredicateRef {
	return transition.deltaPredicateRef
}

type ProfileOnboardingWorkOutcomeV1 interface {
	profileOnboardingWorkOutcomeV1Variant()
	profileOnboardingWorkOutcomeOperationV1() profileOnboardingWorkOutcomeOperationV1
}

type profileOnboardingWorkOutcomeOperationV1 struct {
	resultKind          ProfileOnboardingResultKindV1
	canonicalKind       string
	payloadDigest       ContentDigest
	observedBasisDigest ContentDigest
	missingBasisDigest  ContentDigest
	digestFields        []string
	checks              []profileOnboardingWorkSupportCheckV1
}

type CandidatePayloadProduced interface {
	ProfileOnboardingWorkOutcomeV1
	PayloadDigest() ContentDigest
	ObservedBasisDigest() ContentDigest
	candidatePayloadProducedVariant()
}

type candidatePayloadProduced struct {
	payloadDigest       ContentDigest
	observedBasisDigest ContentDigest
}

func NewCandidatePayloadProduced(
	payloadDigest ContentDigest,
	observedBasisDigest ContentDigest,
) (CandidatePayloadProduced, error) {
	if !payloadDigest.valid() || !observedBasisDigest.valid() {
		return nil, fmt.Errorf("candidate-payload outcome requires payload and observed-basis digests")
	}
	return candidatePayloadProduced{
		payloadDigest:       payloadDigest,
		observedBasisDigest: observedBasisDigest,
	}, nil
}

func (candidatePayloadProduced) profileOnboardingWorkOutcomeV1Variant() {}
func (candidatePayloadProduced) candidatePayloadProducedVariant()       {}

func (outcome candidatePayloadProduced) profileOnboardingWorkOutcomeOperationV1() profileOnboardingWorkOutcomeOperationV1 {
	payloadDigest := outcome.payloadDigest.String()
	observedBasisDigest := outcome.observedBasisDigest.String()
	return profileOnboardingWorkOutcomeOperationV1{
		resultKind: ProfileOnboardingResultKindV1{
			value: profileOnboardingCandidateResultKindV1Value,
		},
		canonicalKind:       "candidate_payload_produced",
		payloadDigest:       outcome.payloadDigest,
		observedBasisDigest: outcome.observedBasisDigest,
		digestFields: []string{
			"candidate_payload_produced",
			payloadDigest,
			observedBasisDigest,
		},
		checks: []profileOnboardingWorkSupportCheckV1{
			{valid: outcome.payloadDigest.valid(), reason: "candidate-payload outcome payload digest is invalid"},
			{valid: outcome.observedBasisDigest.valid(), reason: "candidate-payload outcome observed-basis digest is invalid"},
		},
	}
}

func (outcome candidatePayloadProduced) PayloadDigest() ContentDigest {
	return outcome.payloadDigest
}

func (outcome candidatePayloadProduced) ObservedBasisDigest() ContentDigest {
	return outcome.observedBasisDigest
}

type ClassificationUnderdetermined interface {
	ProfileOnboardingWorkOutcomeV1
	MissingBasisDigest() ContentDigest
	classificationUnderdeterminedVariant()
}

type classificationUnderdetermined struct {
	missingBasisDigest ContentDigest
}

func NewClassificationUnderdetermined(
	missingBasisDigest ContentDigest,
) (ClassificationUnderdetermined, error) {
	if !missingBasisDigest.valid() {
		return nil, fmt.Errorf("classification-underdetermined outcome requires missing-basis digest")
	}
	return classificationUnderdetermined{missingBasisDigest: missingBasisDigest}, nil
}

func (classificationUnderdetermined) profileOnboardingWorkOutcomeV1Variant() {}
func (classificationUnderdetermined) classificationUnderdeterminedVariant()  {}

func (outcome classificationUnderdetermined) profileOnboardingWorkOutcomeOperationV1() profileOnboardingWorkOutcomeOperationV1 {
	missingBasisDigest := outcome.missingBasisDigest.String()
	return profileOnboardingWorkOutcomeOperationV1{
		resultKind: ProfileOnboardingResultKindV1{
			value: profileOnboardingUnderdeterminedKindV1Value,
		},
		canonicalKind:      "classification_underdetermined",
		missingBasisDigest: outcome.missingBasisDigest,
		digestFields: []string{
			"classification_underdetermined",
			missingBasisDigest,
		},
		checks: []profileOnboardingWorkSupportCheckV1{
			{valid: outcome.missingBasisDigest.valid(), reason: "classification-underdetermined outcome missing-basis digest is invalid"},
		},
	}
}

func (outcome classificationUnderdetermined) MissingBasisDigest() ContentDigest {
	return outcome.missingBasisDigest
}

// ProfileOnboardingWorkRecord is a cycle-free immutable description of one
// dated ProfileOnboardingMethod Work occurrence. It contains digest-only
// outcome data and therefore cannot point back to a candidate or provenance.
type ProfileOnboardingWorkRecord struct {
	recordRef                         ProfileOnboardingWorkRecordRef
	workRef                           WorkRef
	enactsMethodRef                   MethodRef
	methodDescriptionRef              MethodDescriptionRef
	methodDescriptionDigest           ContentDigest
	methodContractRef                 ProfileOnboardingMethodContractRef
	methodContractDigest              ContentDigest
	parameterBindings                 MethodParameterBindings
	performedBy                       RoleAssignmentRef
	profileAuthorRoleAssignmentRef    RoleAssignmentRef
	profileAuthorRoleAssignmentDigest ContentDigest
	executedWithin                    SystemRef
	boundedContextRef                 BoundedContextRef
	workInterval                      WorkIntervalV1
	basisObservationWindow            BasisObservationWindowV1
	observedProjectBasisRef           ObservedProjectBasisRefV1
	observedProjectBasisDigest        ContentDigest
	inputRefs                         []WorkInputRef
	outputRefs                        []WorkOutputRef
	resourceRefs                      []WorkResourceRef
	affectedRefKind                   ProfileOnboardingAffectedKindV1
	affectedRefs                      []AffectedReferentRef
	statePlaneRef                     StatePlaneRef
	stateTransition                   WorkStateTransitionV1
	outcome                           ProfileOnboardingWorkOutcomeV1
}

type ProfileOnboardingWorkRecordBuilder struct {
	value ProfileOnboardingWorkRecord
}

func NewProfileOnboardingWorkRecordBuilder(
	recordRef ProfileOnboardingWorkRecordRef,
	workRef WorkRef,
) ProfileOnboardingWorkRecordBuilder {
	return ProfileOnboardingWorkRecordBuilder{value: ProfileOnboardingWorkRecord{
		recordRef: recordRef,
		workRef:   workRef,
	}}
}

func (builder ProfileOnboardingWorkRecordBuilder) Enacts(
	methodRef MethodRef,
	methodDescriptionRef MethodDescriptionRef,
	parameterBindings MethodParameterBindings,
) ProfileOnboardingWorkRecordBuilder {
	builder.value.enactsMethodRef = methodRef
	builder.value.methodDescriptionRef = methodDescriptionRef
	builder.value.parameterBindings = parameterBindings
	return builder
}

func (builder ProfileOnboardingWorkRecordBuilder) WithMethodDescriptionDigest(
	digest ContentDigest,
) ProfileOnboardingWorkRecordBuilder {
	builder.value.methodDescriptionDigest = digest
	return builder
}

func (builder ProfileOnboardingWorkRecordBuilder) GovernedByMethodContract(
	ref ProfileOnboardingMethodContractRef,
	digest ContentDigest,
) ProfileOnboardingWorkRecordBuilder {
	builder.value.methodContractRef = ref
	builder.value.methodContractDigest = digest
	return builder
}

func (builder ProfileOnboardingWorkRecordBuilder) PerformedBy(
	ref RoleAssignmentRef,
) ProfileOnboardingWorkRecordBuilder {
	builder.value.performedBy = ref
	return builder
}

func (builder ProfileOnboardingWorkRecordBuilder) WithProfileAuthorRoleAssignment(
	ref RoleAssignmentRef,
	digest ContentDigest,
) ProfileOnboardingWorkRecordBuilder {
	builder.value.profileAuthorRoleAssignmentRef = ref
	builder.value.profileAuthorRoleAssignmentDigest = digest
	return builder
}

func (builder ProfileOnboardingWorkRecordBuilder) ExecutedWithin(
	ref SystemRef,
) ProfileOnboardingWorkRecordBuilder {
	builder.value.executedWithin = ref
	return builder
}

func (builder ProfileOnboardingWorkRecordBuilder) InContext(
	ref BoundedContextRef,
) ProfileOnboardingWorkRecordBuilder {
	builder.value.boundedContextRef = ref
	return builder
}

func (builder ProfileOnboardingWorkRecordBuilder) During(
	workInterval WorkIntervalV1,
	basisObservationWindow BasisObservationWindowV1,
) ProfileOnboardingWorkRecordBuilder {
	builder.value.workInterval = workInterval
	builder.value.basisObservationWindow = basisObservationWindow
	return builder
}

func (builder ProfileOnboardingWorkRecordBuilder) WithObservedProjectBasis(
	ref ObservedProjectBasisRefV1,
	digest ContentDigest,
) ProfileOnboardingWorkRecordBuilder {
	builder.value.observedProjectBasisRef = ref
	builder.value.observedProjectBasisDigest = digest
	return builder
}

func (builder ProfileOnboardingWorkRecordBuilder) WithInputs(
	refs []WorkInputRef,
) ProfileOnboardingWorkRecordBuilder {
	builder.value.inputRefs = append([]WorkInputRef{}, refs...)
	return builder
}

func (builder ProfileOnboardingWorkRecordBuilder) WithOutputs(
	refs []WorkOutputRef,
) ProfileOnboardingWorkRecordBuilder {
	builder.value.outputRefs = append([]WorkOutputRef{}, refs...)
	return builder
}

func (builder ProfileOnboardingWorkRecordBuilder) WithResources(
	refs []WorkResourceRef,
) ProfileOnboardingWorkRecordBuilder {
	builder.value.resourceRefs = append([]WorkResourceRef{}, refs...)
	return builder
}

func (builder ProfileOnboardingWorkRecordBuilder) Affecting(
	refs []AffectedReferentRef,
) ProfileOnboardingWorkRecordBuilder {
	builder.value.affectedRefs = append([]AffectedReferentRef{}, refs...)
	return builder
}

func (builder ProfileOnboardingWorkRecordBuilder) AffectingKind(
	kind ProfileOnboardingAffectedKindV1,
) ProfileOnboardingWorkRecordBuilder {
	builder.value.affectedRefKind = kind
	return builder
}

func (builder ProfileOnboardingWorkRecordBuilder) OnStatePlane(
	statePlaneRef StatePlaneRef,
	transition WorkStateTransitionV1,
) ProfileOnboardingWorkRecordBuilder {
	builder.value.statePlaneRef = statePlaneRef
	builder.value.stateTransition = transition
	return builder
}

func (builder ProfileOnboardingWorkRecordBuilder) WithOutcome(
	outcome ProfileOnboardingWorkOutcomeV1,
) ProfileOnboardingWorkRecordBuilder {
	builder.value.outcome = outcome
	return builder
}

func (builder ProfileOnboardingWorkRecordBuilder) Build() (ProfileOnboardingWorkRecord, error) {
	canonical, err := canonicalizeProfileOnboardingWorkRecord(builder.value)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	return canonical, nil
}

func (record ProfileOnboardingWorkRecord) RecordRef() ProfileOnboardingWorkRecordRef {
	return record.recordRef
}

func (record ProfileOnboardingWorkRecord) WorkRef() WorkRef {
	return record.workRef
}

func (record ProfileOnboardingWorkRecord) EnactsMethodRef() MethodRef {
	return record.enactsMethodRef
}

func (record ProfileOnboardingWorkRecord) MethodDescriptionRef() MethodDescriptionRef {
	return record.methodDescriptionRef
}

func (record ProfileOnboardingWorkRecord) MethodDescriptionDigest() ContentDigest {
	return record.methodDescriptionDigest
}

func (record ProfileOnboardingWorkRecord) MethodContractRef() ProfileOnboardingMethodContractRef {
	return record.methodContractRef
}

func (record ProfileOnboardingWorkRecord) MethodContractDigest() ContentDigest {
	return record.methodContractDigest
}

func (record ProfileOnboardingWorkRecord) ParameterBindings() MethodParameterBindings {
	return record.parameterBindings
}

func (record ProfileOnboardingWorkRecord) PerformedBy() RoleAssignmentRef {
	return record.performedBy
}

func (record ProfileOnboardingWorkRecord) ProfileAuthorRoleAssignmentRef() RoleAssignmentRef {
	return record.profileAuthorRoleAssignmentRef
}

func (record ProfileOnboardingWorkRecord) ProfileAuthorRoleAssignmentDigest() ContentDigest {
	return record.profileAuthorRoleAssignmentDigest
}

func (record ProfileOnboardingWorkRecord) ExecutedWithin() SystemRef {
	return record.executedWithin
}

func (record ProfileOnboardingWorkRecord) BoundedContextRef() BoundedContextRef {
	return record.boundedContextRef
}

func (record ProfileOnboardingWorkRecord) WorkInterval() WorkIntervalV1 {
	return record.workInterval
}

func (record ProfileOnboardingWorkRecord) BasisObservationWindow() BasisObservationWindowV1 {
	return record.basisObservationWindow
}

func (record ProfileOnboardingWorkRecord) ObservedProjectBasisRef() ObservedProjectBasisRefV1 {
	return record.observedProjectBasisRef
}

func (record ProfileOnboardingWorkRecord) ObservedProjectBasisDigest() ContentDigest {
	return record.observedProjectBasisDigest
}

func (record ProfileOnboardingWorkRecord) InputRefs() []WorkInputRef {
	return append([]WorkInputRef{}, record.inputRefs...)
}

func (record ProfileOnboardingWorkRecord) ProfileOnboardingWorkInputRefV2() (
	WorkInputRef,
	bool,
) {
	canonical, err := canonicalizeProfileOnboardingWorkRecord(record)
	if err != nil {
		return WorkInputRef{}, false
	}
	edition, err := resolveProfileOnboardingWorkMethodEdition(canonical)
	if err != nil {
		return WorkInputRef{}, false
	}
	if _, isV2 := edition.(profileOnboardingWorkMethodEditionV2); !isV2 {
		return WorkInputRef{}, false
	}
	basisRef := canonical.observedProjectBasisRef.String()
	index := slices.IndexFunc(canonical.inputRefs, func(ref WorkInputRef) bool {
		return ref.String() != basisRef
	})
	if index < 0 {
		return WorkInputRef{}, false
	}
	return canonical.inputRefs[index], true
}

func (record ProfileOnboardingWorkRecord) OutputRefs() []WorkOutputRef {
	return append([]WorkOutputRef{}, record.outputRefs...)
}

func (record ProfileOnboardingWorkRecord) ResourceRefs() []WorkResourceRef {
	return append([]WorkResourceRef{}, record.resourceRefs...)
}

func (record ProfileOnboardingWorkRecord) AffectedRefs() []AffectedReferentRef {
	return append([]AffectedReferentRef{}, record.affectedRefs...)
}

func (record ProfileOnboardingWorkRecord) AffectedRefKind() ProfileOnboardingAffectedKindV1 {
	return record.affectedRefKind
}

func (record ProfileOnboardingWorkRecord) StatePlaneRef() StatePlaneRef {
	return record.statePlaneRef
}

func (record ProfileOnboardingWorkRecord) StateTransition() WorkStateTransitionV1 {
	return record.stateTransition
}

func (record ProfileOnboardingWorkRecord) Outcome() ProfileOnboardingWorkOutcomeV1 {
	return record.outcome
}

// ValidateProfileOnboardingWorkRecordAgainstSupportV1 proves the direct,
// pure relation between one Work description and the exact support objects it
// names. It consumes the local occurrence contract but does not discharge the
// returned authority-coverage requirements. Binding callers must evaluate the
// occurrence contract and discharge those requirements against sealed
// authority. This function does not resolve carriers, perform Work, admit
// authority, or make an acceptance decision.
func ValidateProfileOnboardingWorkRecordAgainstSupportV1(
	record ProfileOnboardingWorkRecord,
	description ProfileOnboardingMethodDescriptionV1,
	contract ProfileOnboardingMethodContractV1,
	assignment ProfileAuthorRoleAssignmentV1,
	assignmentSupport ProfileAuthorAssignmentSupportCarrierV1,
	basis ObservedProjectBasisV1,
) error {
	work, err := canonicalizeProfileOnboardingWorkRecord(record)
	if err != nil {
		return err
	}
	exactDescription, err := exactProfileOnboardingMethodDescriptionV1(description)
	if err != nil {
		return err
	}
	exactContract, err := exactProfileOnboardingMethodContractV1(contract)
	if err != nil {
		return err
	}
	exactAssignment, err := canonicalProfileAuthorRoleAssignmentV1(assignment)
	if err != nil {
		return err
	}
	exactBasis, err := exactObservedProjectBasisV1(basis)
	if err != nil {
		return err
	}
	descriptionDigest, err := DigestProfileOnboardingMethodDescriptionV1(exactDescription)
	if err != nil {
		return err
	}
	contractDigest, err := DigestProfileOnboardingMethodContractV1(exactContract)
	if err != nil {
		return err
	}
	assignmentDigest, err := DigestProfileAuthorRoleAssignmentV1(exactAssignment)
	if err != nil {
		return err
	}
	basisDigest, err := DigestObservedProjectBasisV1(exactBasis)
	if err != nil {
		return err
	}
	err = ValidateProfileAuthorRoleAssignmentV1Support(exactAssignment, assignmentSupport)
	if err != nil {
		return err
	}
	systemAdmission := assignmentSupport.SystemAdmission()
	roleAdmission := assignmentSupport.RoleAdmission()
	justification := assignmentSupport.Justification()
	provenance := assignmentSupport.Provenance()
	validationContext := profileOnboardingWorkValidationContextV1{
		work:              work,
		description:       exactDescription,
		contract:          exactContract,
		assignment:        exactAssignment,
		systemAdmission:   systemAdmission,
		roleAdmission:     roleAdmission,
		justification:     justification,
		provenance:        provenance,
		basis:             exactBasis,
		descriptionDigest: descriptionDigest,
		contractDigest:    contractDigest,
		assignmentDigest:  assignmentDigest,
		basisDigest:       basisDigest,
	}
	occurrenceContext := profileOnboardingOccurrenceContractContextV1{
		work:        work,
		description: exactDescription,
		contract:    exactContract,
		assignment:  exactAssignment,
		basis:       exactBasis,
	}
	occurrenceEvaluation, err := evaluateProfileOnboardingOccurrenceContractV1(
		occurrenceContext,
	)
	if err != nil {
		return err
	}
	deferredRequirements := occurrenceEvaluation.DeferredAuthorityCoverageRequirements()
	err = validateProfileOnboardingDeferredCoverageRequirementsV1(deferredRequirements)
	if err != nil {
		return err
	}
	checks, err := profileOnboardingWorkSupportChecksV1(validationContext)
	if err != nil {
		return err
	}
	err = visitSliceV1(checks, validateProfileOnboardingWorkSupportCheckV1)
	if err != nil {
		return err
	}
	err = ValidateProfileOnboardingWorkRecordAgainstProfileAuthorRoleAssignmentV1(work, exactAssignment)
	if err != nil {
		return err
	}
	holderRule := exactContract.HolderEqualsExecutedWithinRule()
	err = ValidateHolderEqualsExecutedWithinV1(
		holderRule,
		exactAssignment,
		work.executedWithin,
	)
	if err != nil {
		return err
	}
	err = ValidateObservedProjectBasisV1AgainstWorkRecord(exactBasis, work)
	if err != nil {
		return err
	}
	err = validateProfileOnboardingOutcomeBasisDigestV1(work.outcome, basisDigest)
	return err
}

type profileOnboardingWorkSupportCheckV1 struct {
	valid  bool
	reason string
}

// profileOnboardingWorkValidationContextV1 is an ephemeral pure-validation
// input, not a stored support aggregate or ontology object.
type profileOnboardingWorkValidationContextV1 struct {
	work              ProfileOnboardingWorkRecord
	description       profileOnboardingMethodDescriptionV1
	contract          profileOnboardingMethodContractV1
	assignment        ProfileAuthorRoleAssignmentV1
	systemAdmission   ProfileOnboardingExecutorSystemAdmissionV1
	roleAdmission     ProfileAuthorRoleAdmissionV1
	justification     ProfileAuthorAssignmentJustificationV1
	provenance        ProfileAuthorAssignmentProvenanceV1
	basis             observedProjectBasisV1
	descriptionDigest ContentDigest
	contractDigest    ContentDigest
	assignmentDigest  ContentDigest
	basisDigest       ContentDigest
}

func profileOnboardingWorkSupportChecksV1(
	context profileOnboardingWorkValidationContextV1,
) ([]profileOnboardingWorkSupportCheckV1, error) {
	work := context.work
	description := context.description
	contract := context.contract
	assignment := context.assignment
	systemAdmission := context.systemAdmission
	roleAdmission := context.roleAdmission
	justification := context.justification
	provenance := context.provenance
	basis := context.basis
	descriptionDigest := context.descriptionDigest
	contractDigest := context.contractDigest
	assignmentDigest := context.assignmentDigest
	basisDigest := context.basisDigest
	parameterNames := methodParameterNamesV1(work.parameterBindings)
	parameterDeclarations := description.ParameterDeclarations()
	declaredParameterNames := profileOnboardingParameterDeclarationNamesV1(parameterDeclarations)
	resultKind, err := profileOnboardingWorkOutcomeResultKindV1(work.outcome)
	if err != nil {
		return nil, err
	}
	acceptedResultKinds := contract.AcceptedResultKinds()
	acceptedResult := slices.ContainsFunc(
		acceptedResultKinds,
		func(value ProfileOnboardingResultKindV1) bool {
			valueKind := value.String()
			return valueKind == resultKind
		},
	)
	sessionRef, hasSessionRef := work.parameterBindings.ValueFor(
		profileOnboardingSessionParameterV1,
	)
	describedMethodRef := description.DescribedMethodRef()
	descriptionRef := description.Ref()
	contractRef := contract.Ref()
	contractDescriptionRef := contract.MethodDescriptionRef()
	contractDescriptionDigest := contract.MethodDescriptionDigest()
	assignmentRef := assignment.RoleAssignmentRef()
	systemRef := systemAdmission.SystemRef()
	basisRef := basis.Ref()
	descriptionContext := description.BoundedContextRef()
	contractContext := contract.BoundedContextRef()
	assignmentContext := assignment.BoundedContextRef()
	systemContext := systemAdmission.BoundedContextRef()
	roleContext := roleAdmission.BoundedContextRef()
	systemDescriptionRef := systemAdmission.MethodDescriptionRef()
	systemDescriptionDigest := systemAdmission.MethodDescriptionDigest()
	roleDescriptionRef := roleAdmission.MethodDescriptionRef()
	roleDescriptionDigest := roleAdmission.MethodDescriptionDigest()
	systemContractRef := systemAdmission.MethodContractRef()
	systemContractDigest := systemAdmission.MethodContractDigest()
	roleContractRef := roleAdmission.MethodContractRef()
	roleContractDigest := roleAdmission.MethodContractDigest()
	justificationContractRef := justification.MethodContractRef()
	justificationContractDigest := justification.MethodContractDigest()
	parameterNamesMatch := slices.Equal(parameterNames, declaredParameterNames)
	systemSessionRef := systemAdmission.SessionRef()
	systemSession := systemSessionRef.String()
	provenanceSessionRef := provenance.SessionRef()
	provenanceSession := provenanceSessionRef.String()
	systemWindow := systemAdmission.ValidityWindow()
	systemCoversWork := systemWindow.CoversWork(work.workInterval)
	descriptionStatePlane := description.StatePlaneRef()
	descriptionAffectedKind := description.AffectedRefKind()
	checks := []profileOnboardingWorkSupportCheckV1{
		{valid: work.enactsMethodRef == describedMethodRef, reason: "Work method ref does not match MethodDescription"},
		{valid: work.methodDescriptionRef == descriptionRef, reason: "Work MethodDescription ref does not match exact MethodDescription"},
		{valid: work.methodDescriptionDigest == descriptionDigest, reason: "Work MethodDescription digest does not match exact MethodDescription"},
		{valid: work.methodContractRef == contractRef, reason: "Work MethodContract ref does not match exact MethodContract"},
		{valid: work.methodContractDigest == contractDigest, reason: "Work MethodContract digest does not match exact MethodContract"},
		{valid: contractDescriptionRef == descriptionRef, reason: "MethodContract does not bind the exact MethodDescription ref"},
		{valid: contractDescriptionDigest == descriptionDigest, reason: "MethodContract does not bind the exact MethodDescription digest"},
		{valid: work.profileAuthorRoleAssignmentRef == assignmentRef, reason: "Work ProfileAuthorRoleAssignment ref does not match exact assignment"},
		{valid: work.profileAuthorRoleAssignmentDigest == assignmentDigest, reason: "Work ProfileAuthorRoleAssignment digest does not match exact assignment"},
		{valid: work.executedWithin == systemRef, reason: "Work executedWithin does not match admitted executor system"},
		{valid: work.observedProjectBasisRef == basisRef, reason: "Work ObservedProjectBasis ref does not match exact basis"},
		{valid: work.observedProjectBasisDigest == basisDigest, reason: "Work ObservedProjectBasis digest does not match exact basis"},
		{valid: work.boundedContextRef == descriptionContext, reason: "Work context does not match MethodDescription"},
		{valid: work.boundedContextRef == contractContext, reason: "Work context does not match MethodContract"},
		{valid: work.boundedContextRef == assignmentContext, reason: "Work context does not match ProfileAuthorRoleAssignment"},
		{valid: work.boundedContextRef == systemContext, reason: "Work context does not match executor-system admission"},
		{valid: work.boundedContextRef == roleContext, reason: "Work context does not match ProfileAuthor role admission"},
		{valid: work.methodDescriptionRef == systemDescriptionRef, reason: "Work MethodDescription ref does not match executor-system admission"},
		{valid: work.methodDescriptionDigest == systemDescriptionDigest, reason: "Work MethodDescription digest does not match executor-system admission"},
		{valid: work.methodDescriptionRef == roleDescriptionRef, reason: "Work MethodDescription ref does not match ProfileAuthor role admission"},
		{valid: work.methodDescriptionDigest == roleDescriptionDigest, reason: "Work MethodDescription digest does not match ProfileAuthor role admission"},
		{valid: work.methodContractRef == systemContractRef, reason: "Work MethodContract ref does not match executor-system admission"},
		{valid: work.methodContractDigest == systemContractDigest, reason: "Work MethodContract digest does not match executor-system admission"},
		{valid: work.methodContractRef == roleContractRef, reason: "Work MethodContract ref does not match ProfileAuthor role admission"},
		{valid: work.methodContractDigest == roleContractDigest, reason: "Work MethodContract digest does not match ProfileAuthor role admission"},
		{valid: work.methodContractRef == justificationContractRef, reason: "Work MethodContract ref does not match assignment justification"},
		{valid: work.methodContractDigest == justificationContractDigest, reason: "Work MethodContract digest does not match assignment justification"},
		{valid: parameterNamesMatch, reason: "Work parameter names do not match exact MethodDescription declarations"},
		{valid: hasSessionRef, reason: "Work does not bind the onboarding session"},
		{valid: sessionRef == systemSession, reason: "Work session does not match executor-system admission"},
		{valid: sessionRef == provenanceSession, reason: "Work session does not match assignment provenance"},
		{valid: systemCoversWork, reason: "executor-system admission window does not cover Work"},
		{valid: work.statePlaneRef == descriptionStatePlane, reason: "Work StatePlane does not match MethodDescription"},
		{valid: work.affectedRefKind == descriptionAffectedKind, reason: "Work affected kind does not match MethodDescription"},
		{valid: acceptedResult, reason: "Work result kind is not accepted by MethodContract"},
	}
	return checks, nil
}

func profileOnboardingParameterDeclarationNamesV1(
	values []ProfileOnboardingParameterDeclarationV1,
) []string {
	result := mapSliceV1Pure(values, func(value ProfileOnboardingParameterDeclarationV1) string {
		return value.Name()
	})
	return result
}

func profileOnboardingWorkOutcomeResultKindV1(
	outcome ProfileOnboardingWorkOutcomeV1,
) (string, error) {
	operation, err := exactProfileOnboardingWorkOutcomeOperationV1(outcome)
	if err != nil {
		return "", err
	}
	return operation.resultKind.String(), nil
}

func validateProfileOnboardingWorkSupportCheckV1(
	_ int,
	check profileOnboardingWorkSupportCheckV1,
) error {
	if !check.valid {
		return fmt.Errorf("%s", check.reason)
	}
	return nil
}

func validateProfileOnboardingOutcomeBasisDigestV1(
	outcome ProfileOnboardingWorkOutcomeV1,
	basisDigest ContentDigest,
) error {
	operation, err := exactProfileOnboardingWorkOutcomeOperationV1(outcome)
	if err != nil {
		return err
	}
	if !operation.observedBasisDigest.valid() {
		return nil
	}
	if operation.observedBasisDigest != basisDigest {
		return fmt.Errorf("candidate outcome does not bind the exact ObservedProjectBasis digest")
	}
	return nil
}

func DigestProfileOnboardingWorkRecord(
	record ProfileOnboardingWorkRecord,
) (ContentDigest, error) {
	validated, err := canonicalizeProfileOnboardingWorkRecord(record)
	if err != nil {
		return ContentDigest{}, err
	}
	edition, err := resolveProfileOnboardingWorkMethodEdition(validated)
	if err != nil {
		return ContentDigest{}, err
	}
	domain := profileOnboardingWorkRecordDigestDomain(edition)
	writer := newCanonicalDigestWriter(domain)
	err = addProfileOnboardingWorkRecordDigestFields(writer, validated)
	if err != nil {
		return ContentDigest{}, err
	}
	return writer.digest(), nil
}

func canonicalizeProfileOnboardingWorkRecord(
	record ProfileOnboardingWorkRecord,
) (ProfileOnboardingWorkRecord, error) {
	recordRefValid := record.recordRef.valid()
	workRefValid := record.workRef.valid()
	if !recordRefValid || !workRefValid {
		return ProfileOnboardingWorkRecord{}, fmt.Errorf("work record and Work refs are required")
	}
	edition, err := resolveProfileOnboardingWorkMethodEdition(record)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	methodDescriptionDigestValid := record.methodDescriptionDigest.valid()
	if !methodDescriptionDigestValid {
		return ProfileOnboardingWorkRecord{}, fmt.Errorf("work MethodDescription digest is required")
	}
	methodContractDigestValid := record.methodContractDigest.valid()
	if !methodContractDigestValid {
		return ProfileOnboardingWorkRecord{}, fmt.Errorf("work MethodContract digest is required")
	}
	if !record.parameterBindings.valid() {
		return ProfileOnboardingWorkRecord{}, fmt.Errorf("work concrete parameter bindings are invalid")
	}
	parameterNames := methodParameterNamesV1(record.parameterBindings)
	if !validProfileOnboardingParameterNamesV1(parameterNames) {
		return ProfileOnboardingWorkRecord{}, fmt.Errorf("work parameter bindings do not match ProfileOnboardingMethodDescription v1")
	}
	performedByValid := record.performedBy.valid()
	executedWithinValid := record.executedWithin.valid()
	boundedContextValid := record.boundedContextRef.valid()
	if !performedByValid || !executedWithinValid || !boundedContextValid {
		return ProfileOnboardingWorkRecord{}, fmt.Errorf("work performer, executing system, and context refs are required")
	}
	assignmentRefValid := record.profileAuthorRoleAssignmentRef.valid()
	assignmentDigestValid := record.profileAuthorRoleAssignmentDigest.valid()
	if !assignmentRefValid || !assignmentDigestValid {
		return ProfileOnboardingWorkRecord{}, fmt.Errorf("work ProfileAuthorRoleAssignment ref and digest are required")
	}
	if record.profileAuthorRoleAssignmentRef != record.performedBy {
		return ProfileOnboardingWorkRecord{}, fmt.Errorf("work ProfileAuthorRoleAssignment ref must equal performedBy")
	}
	workIntervalValid := record.workInterval.valid()
	basisWindowValid := record.basisObservationWindow.valid()
	if !workIntervalValid || !basisWindowValid {
		return ProfileOnboardingWorkRecord{}, fmt.Errorf("work and basis-observation windows are invalid")
	}
	basisRefValid := record.observedProjectBasisRef.valid()
	basisDigestValid := record.observedProjectBasisDigest.valid()
	if !basisRefValid || !basisDigestValid {
		return ProfileOnboardingWorkRecord{}, fmt.Errorf("work ObservedProjectBasis ref and digest are required")
	}
	inputRefs, err := canonicalWorkInputRefs(record.inputRefs)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	err = validateProfileOnboardingWorkInputCardinality(edition, record.observedProjectBasisRef, inputRefs)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	outputRefs, err := canonicalWorkOutputRefs(record.outputRefs)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	resourceRefs, err := canonicalWorkResourceRefs(record.resourceRefs)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	affectedRefs, err := canonicalAffectedRefs(record.affectedRefs)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	if record.affectedRefKind.String() != profileOnboardingAffectedKindV1Value {
		return ProfileOnboardingWorkRecord{}, fmt.Errorf("work affected kind must be ProfileClassificationEpistemeV1")
	}
	if !record.statePlaneRef.valid() {
		return ProfileOnboardingWorkRecord{}, fmt.Errorf("work StatePlane ref is required")
	}
	err = validateWorkStateTransition(record.stateTransition)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	err = validateProfileOnboardingWorkOutcome(record.outcome)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	record.inputRefs = inputRefs
	record.outputRefs = outputRefs
	record.resourceRefs = resourceRefs
	record.affectedRefs = affectedRefs
	return record, nil
}

type profileOnboardingWorkMethodEditionV1 struct{}
type profileOnboardingWorkMethodEditionV2 struct{}

type profileOnboardingWorkMethodEdition interface {
	profileOnboardingWorkMethodEdition()
}

func (profileOnboardingWorkMethodEditionV1) profileOnboardingWorkMethodEdition() {}
func (profileOnboardingWorkMethodEditionV2) profileOnboardingWorkMethodEdition() {}

func resolveProfileOnboardingWorkMethodEdition(
	record ProfileOnboardingWorkRecord,
) (profileOnboardingWorkMethodEdition, error) {
	if record.methodContractRef == nil {
		return nil, fmt.Errorf("work MethodContract ref is required")
	}
	contractRef := record.methodContractRef.String()
	if record.enactsMethodRef == ProfileOnboardingMethodRefV1() &&
		record.methodDescriptionRef == ProfileOnboardingMethodDescriptionRefV1() &&
		contractRef == profileOnboardingMethodContractRefV1Value {
		return profileOnboardingWorkMethodEditionV1{}, nil
	}
	if record.enactsMethodRef == ProfileOnboardingMethodRefV2() &&
		record.methodDescriptionRef == ProfileOnboardingMethodDescriptionRefV2() &&
		contractRef == profileOnboardingMethodContractRefV2Value {
		return profileOnboardingWorkMethodEditionV2{}, nil
	}
	return nil, fmt.Errorf("work method, MethodDescription, and MethodContract refs do not form an exact supported v1 or v2 edition")
}

func profileOnboardingWorkRecordDigestDomain(
	edition profileOnboardingWorkMethodEdition,
) string {
	switch edition.(type) {
	case profileOnboardingWorkMethodEditionV1:
		return profileOnboardingWorkRecordDigestDomainV1
	case profileOnboardingWorkMethodEditionV2:
		return profileOnboardingWorkRecordDigestDomainV2
	default:
		return ""
	}
}

func validateProfileOnboardingWorkInputCardinality(
	edition profileOnboardingWorkMethodEdition,
	basisRef ObservedProjectBasisRefV1,
	inputRefs []WorkInputRef,
) error {
	if _, isV1 := edition.(profileOnboardingWorkMethodEditionV1); isV1 {
		return nil
	}
	if len(inputRefs) != 2 {
		return fmt.Errorf("ProfileOnboarding Method v2 Work requires exactly ObservedProjectBasisV1 and ProfileOnboardingWorkInputV1 refs")
	}
	basis := basisRef.String()
	containsBasis := slices.ContainsFunc(inputRefs, func(ref WorkInputRef) bool {
		return ref.String() == basis
	})
	if !containsBasis {
		return fmt.Errorf("ProfileOnboarding Method v2 Work inputs omit the exact ObservedProjectBasisV1 ref")
	}
	return nil
}

func validProfileOnboardingParameterNamesV1(values []string) bool {
	if len(values) != 4 {
		return false
	}
	return values[0] == profileOnboardingClassifierParameterV1 &&
		values[1] == profileOnboardingPolicyParameterV1 &&
		values[2] == profileOnboardingProjectRootParameterV1 &&
		values[3] == profileOnboardingSessionParameterV1
}

func methodParameterNamesV1(bindings MethodParameterBindings) []string {
	values := bindings.Values()
	return mapSliceV1Pure(values, func(value MethodParameterBinding) string {
		return value.Name()
	})
}

func validateWorkStateTransition(transition WorkStateTransitionV1) error {
	switch value := transition.(type) {
	case prePostStateTransitionV1:
		if value.preStateRef.valid() && value.postStateRef.valid() {
			return nil
		}
	case deltaStateTransitionV1:
		if value.preStateRef.valid() && value.deltaPredicateRef.valid() {
			return nil
		}
	}
	return fmt.Errorf("work needs exactly one valid pre/post or pre/delta state transition")
}

func validateProfileOnboardingWorkOutcome(outcome ProfileOnboardingWorkOutcomeV1) error {
	_, err := exactProfileOnboardingWorkOutcomeOperationV1(outcome)
	return err
}

func exactProfileOnboardingWorkOutcomeOperationV1(
	outcome ProfileOnboardingWorkOutcomeV1,
) (profileOnboardingWorkOutcomeOperationV1, error) {
	if outcome == nil {
		return profileOnboardingWorkOutcomeOperationV1{}, fmt.Errorf("work outcome is invalid or unknown")
	}
	operation := outcome.profileOnboardingWorkOutcomeOperationV1()
	err := visitSliceV1(operation.checks, validateProfileOnboardingWorkSupportCheckV1)
	if err != nil {
		return profileOnboardingWorkOutcomeOperationV1{}, err
	}
	if len(operation.digestFields) <= 1 {
		return profileOnboardingWorkOutcomeOperationV1{}, fmt.Errorf("work outcome digest fields are incomplete")
	}
	checks := []profileOnboardingWorkSupportCheckV1{
		{valid: operation.resultKind.String() != "", reason: "Work outcome result kind is missing"},
		{valid: operation.canonicalKind != "", reason: "Work outcome canonical kind is missing"},
		{valid: operation.digestFields[0] == operation.canonicalKind, reason: "Work outcome digest kind is inconsistent"},
	}
	err = visitSliceV1(checks, validateProfileOnboardingWorkSupportCheckV1)
	if err != nil {
		return profileOnboardingWorkOutcomeOperationV1{}, err
	}
	return operation, nil
}

func canonicalWorkInputRefs(values []WorkInputRef) ([]WorkInputRef, error) {
	result := append([]WorkInputRef{}, values...)
	err := canonicalizeV1Refs("Work input refs", result, func(value WorkInputRef) string {
		return value.String()
	}, func(value WorkInputRef) bool {
		return value.valid()
	})
	return result, err
}

func canonicalWorkOutputRefs(values []WorkOutputRef) ([]WorkOutputRef, error) {
	result := append([]WorkOutputRef{}, values...)
	err := canonicalizeV1Refs("Work output refs", result, func(value WorkOutputRef) string {
		return value.String()
	}, func(value WorkOutputRef) bool {
		return value.valid()
	})
	return result, err
}

func canonicalWorkResourceRefs(values []WorkResourceRef) ([]WorkResourceRef, error) {
	result := append([]WorkResourceRef{}, values...)
	err := canonicalizeV1Refs("Work resource refs", result, func(value WorkResourceRef) string {
		return value.String()
	}, func(value WorkResourceRef) bool {
		return value.valid()
	})
	return result, err
}

func canonicalAffectedRefs(values []AffectedReferentRef) ([]AffectedReferentRef, error) {
	result := append([]AffectedReferentRef{}, values...)
	err := canonicalizeV1Refs("affected referent refs", result, func(value AffectedReferentRef) string {
		return value.String()
	}, func(value AffectedReferentRef) bool {
		return value.valid()
	})
	return result, err
}

func canonicalizeV1Refs[T any](
	name string,
	values []T,
	key func(T) string,
	valid func(T) bool,
) error {
	if len(values) == 0 {
		return fmt.Errorf("%s must not be empty", name)
	}
	err := visitSliceV1(values, func(index int, value T) error {
		if !valid(value) {
			return fmt.Errorf("%s contains invalid value at index %d", name, index)
		}
		return nil
	})
	if err != nil {
		return err
	}
	compare := compareV1RefByKey(key)
	slices.SortFunc(values, compare)
	return visitAdjacentV1(values, func(previous T, current T) error {
		previousKey := key(previous)
		currentKey := key(current)
		if previousKey == currentKey {
			return fmt.Errorf("%s contains duplicate %q", name, currentKey)
		}
		return nil
	})
}

func compareV1RefByKey[T any](key func(T) string) func(T, T) int {
	return func(left T, right T) int {
		leftKey := key(left)
		rightKey := key(right)
		return cmp.Compare(leftKey, rightKey)
	}
}

func addProfileOnboardingWorkRecordDigestFields(
	writer canonicalDigestWriter,
	record ProfileOnboardingWorkRecord,
) error {
	recordRef := record.recordRef.String()
	workRef := record.workRef.String()
	methodRef := record.enactsMethodRef.String()
	methodDescriptionRef := record.methodDescriptionRef.String()
	methodDescriptionDigest := record.methodDescriptionDigest.String()
	methodContractRef := record.methodContractRef.String()
	methodContractDigest := record.methodContractDigest.String()
	performedBy := record.performedBy.String()
	profileAuthorRoleAssignmentRef := record.profileAuthorRoleAssignmentRef.String()
	profileAuthorRoleAssignmentDigest := record.profileAuthorRoleAssignmentDigest.String()
	executedWithin := record.executedWithin.String()
	boundedContextRef := record.boundedContextRef.String()
	observedProjectBasisRef := record.observedProjectBasisRef.String()
	observedProjectBasisDigest := record.observedProjectBasisDigest.String()
	inputRefs := workInputStrings(record.inputRefs)
	outputRefs := workOutputStrings(record.outputRefs)
	resourceRefs := workResourceStrings(record.resourceRefs)
	affectedRefs := affectedReferentStrings(record.affectedRefs)
	affectedRefKind := record.affectedRefKind.String()
	statePlaneRef := record.statePlaneRef.String()
	writer.add(recordRef)
	writer.add(workRef)
	writer.add(methodRef)
	writer.add(methodDescriptionRef)
	writer.add(methodDescriptionDigest)
	writer.add(methodContractRef)
	writer.add(methodContractDigest)
	addMethodParameterBindings(writer, record.parameterBindings)
	writer.add(performedBy)
	writer.add(profileAuthorRoleAssignmentRef)
	writer.add(profileAuthorRoleAssignmentDigest)
	writer.add(executedWithin)
	writer.add(boundedContextRef)
	addClosedIntervalV1(writer, record.workInterval.closedIntervalV1)
	addClosedIntervalV1(writer, record.basisObservationWindow.closedIntervalV1)
	writer.add(observedProjectBasisRef)
	writer.add(observedProjectBasisDigest)
	addReferenceStrings(writer, inputRefs)
	addReferenceStrings(writer, outputRefs)
	addReferenceStrings(writer, resourceRefs)
	writer.add(affectedRefKind)
	addReferenceStrings(writer, affectedRefs)
	writer.add(statePlaneRef)
	addWorkStateTransition(writer, record.stateTransition)
	return addProfileOnboardingWorkOutcome(writer, record.outcome)
}

func addMethodParameterBindings(
	writer canonicalDigestWriter,
	bindings MethodParameterBindings,
) {
	values := bindings.Values()
	count := len(values)
	valueCount := strconv.Itoa(count)
	writer.add(valueCount)
	visitSliceV1Pure(values, func(binding MethodParameterBinding) {
		name := binding.Name()
		value := binding.Value()
		writer.add(name)
		writer.add(value)
	})
}

func addClosedIntervalV1(writer canonicalDigestWriter, interval closedIntervalV1) {
	from := canonicalTime(interval.from)
	until := canonicalTime(interval.until)
	writer.add(from)
	writer.add(until)
}

func addReferenceStrings(writer canonicalDigestWriter, values []string) {
	count := len(values)
	valueCount := strconv.Itoa(count)
	writer.add(valueCount)
	visitSliceV1Pure(values, func(value string) {
		writer.add(value)
	})
}

func addWorkStateTransition(writer canonicalDigestWriter, transition WorkStateTransitionV1) {
	switch value := transition.(type) {
	case prePostStateTransitionV1:
		preStateRef := value.preStateRef.String()
		postStateRef := value.postStateRef.String()
		writer.add("pre_post")
		writer.add(preStateRef)
		writer.add(postStateRef)
	case deltaStateTransitionV1:
		preStateRef := value.preStateRef.String()
		deltaPredicateRef := value.deltaPredicateRef.String()
		writer.add("delta_predicate")
		writer.add(preStateRef)
		writer.add(deltaPredicateRef)
	}
}

func addProfileOnboardingWorkOutcome(
	writer canonicalDigestWriter,
	outcome ProfileOnboardingWorkOutcomeV1,
) error {
	operation, err := exactProfileOnboardingWorkOutcomeOperationV1(outcome)
	if err != nil {
		return err
	}
	visitSliceV1Pure(operation.digestFields, func(value string) {
		writer.add(value)
	})
	return nil
}

func workInputStrings(values []WorkInputRef) []string {
	return mapSliceV1Pure(values, func(value WorkInputRef) string {
		return value.String()
	})
}

func workOutputStrings(values []WorkOutputRef) []string {
	return mapSliceV1Pure(values, func(value WorkOutputRef) string {
		return value.String()
	})
}

func workResourceStrings(values []WorkResourceRef) []string {
	return mapSliceV1Pure(values, func(value WorkResourceRef) string {
		return value.String()
	})
}

func affectedReferentStrings(values []AffectedReferentRef) []string {
	return mapSliceV1Pure(values, func(value AffectedReferentRef) string {
		return value.String()
	})
}
