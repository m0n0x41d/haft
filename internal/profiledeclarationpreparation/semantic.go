package profiledeclarationpreparation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

type declarationRefs struct {
	token                string
	session              string
	system               string
	systemAdmission      string
	actingEligibility    string
	roleAdmission        string
	assignmentReason     string
	assignmentProvenance string
	roleAssignment       string
	observedBasis        string
	workRecord           string
	work                 string
	output               string
	resource             string
	affected             string
	preState             string
	postState            string
	effect               string
	effectEvidence       string
	assessment           string
	assessmentEvidence   string
	comparator           string
	singleUse            string
}

type AuthoritySupport struct {
	description   projectprofile.ProfileOnboardingMethodDescriptionV2
	contract      projectprofile.ProfileOnboardingMethodContractV2
	actors        declarationActors
	allowedWork   authority.TimeWindow
	allowedBasis  authority.TimeWindow
	validUntil    time.Time
	preparedAt    time.Time
	declarationID string
}

type declarationActors struct {
	session         projectprofile.SessionRef
	classifier      projectprofile.ClassifierVersion
	policy          projectprofile.PolicyVersion
	systemAdmission projectprofile.ProfileOnboardingExecutorSystemAdmissionV1
	roleAdmission   projectprofile.ProfileAuthorRoleAdmissionV1
	justification   projectprofile.ProfileAuthorAssignmentJustificationV1
	provenance      projectprofile.ProfileAuthorAssignmentProvenanceV1
	assignment      projectprofile.ProfileAuthorRoleAssignmentV1
}

type Plan struct {
	root    projectprofile.ProjectRootV1
	input   ProfileOnboardingWorkInput
	policy  Policy
	refs    declarationRefs
	support AuthoritySupport
}

type OccurrenceTimes struct {
	workFrom  time.Time
	basisFrom time.Time
	basisTo   time.Time
	workTo    time.Time
}

type ValueSet struct {
	description projectprofile.ProfileOnboardingMethodDescriptionV2
	contract    projectprofile.ProfileOnboardingMethodContractV2
	actors      declarationActors
	basis       projectprofile.ObservedProjectBasisV1
	work        projectprofile.ProfileOnboardingWorkRecord
	effect      projectprofile.ProfileOnboardingEffectV1
	assessment  projectprofile.ProfileOnboardingOutcomeAssessmentV1
}

func NewPlan(
	projectRoot string,
	input ProfileOnboardingWorkInput,
	policy Policy,
	checkedAt time.Time,
) (Plan, error) {
	if !input.Valid() {
		return Plan{}, fmt.Errorf("profile declaration preparation requires an exact reviewed Work input")
	}
	if policy.Mode() != ModeHostRoutedOperatorRequest &&
		policy.Mode() != ModeAutomaticSupportedSingleton {
		return Plan{}, fmt.Errorf(
			"profile declaration provenance policy is unsupported",
		)
	}
	if policy.Mode() == ModeHostRoutedOperatorRequest {
		request, ok := policy.OperatorRequest()
		if !ok || !request.MatchesPayload(input.CanonicalJSON()) {
			return Plan{}, fmt.Errorf(
				"host-routed profile request does not bind the exact reviewed WorkInput",
			)
		}
	}
	if policy.Mode() == ModeAutomaticSupportedSingleton {
		detectorVersion,
			policyVersion,
			suggestionRef,
			observationDigest,
			ok := policy.AutomaticProvenance()
		if !ok ||
			input.UsesManualScopeBasis() ||
			input.DetectorVersion() != detectorVersion ||
			input.PolicyVersion() != policyVersion ||
			input.SuggestionRef() != suggestionRef ||
			input.ObservationDigest() != observationDigest {
			return Plan{}, fmt.Errorf(
				"automatic profile policy does not match the exact detector WorkInput",
			)
		}
	}
	root, err := projectprofile.NewProjectRootV1(projectRoot)
	if err != nil {
		return Plan{}, err
	}
	if root != input.ProjectRoot() {
		return Plan{}, fmt.Errorf("reviewed profile input belongs to another physical project root")
	}
	canonicalCheckedAt := checkedAt.UTC().Round(0)
	if canonicalCheckedAt.IsZero() {
		return Plan{}, fmt.Errorf("profile declaration preparation time is required")
	}
	identity := declarationIdentity(input, policy.Mode())
	refs, err := newDeclarationRefs(identity, input)
	if err != nil {
		return Plan{}, err
	}
	allowedWork, err := authority.NewTimeWindow(
		canonicalCheckedAt,
		canonicalCheckedAt.Add(25*time.Minute),
	)
	if err != nil {
		return Plan{}, err
	}
	allowedBasis, err := authority.NewTimeWindow(
		canonicalCheckedAt,
		canonicalCheckedAt.Add(20*time.Minute),
	)
	if err != nil {
		return Plan{}, err
	}
	support, err := buildAuthoritySupport(
		refs,
		canonicalCheckedAt,
		canonicalCheckedAt.Add(30*time.Minute),
		input.ClassifierVersion(),
		input.ClassificationPolicyVersion(),
		allowedWork,
		allowedBasis,
		identity,
	)
	if err != nil {
		return Plan{}, err
	}
	return Plan{
		root:    root,
		input:   input,
		policy:  policy,
		refs:    refs,
		support: support,
	}, nil
}

func NewOccurrenceTimes(
	workFrom time.Time,
	basisFrom time.Time,
	basisTo time.Time,
	workTo time.Time,
) (OccurrenceTimes, error) {
	canonical := OccurrenceTimes{
		workFrom:  workFrom.UTC().Round(0),
		basisFrom: basisFrom.UTC().Round(0),
		basisTo:   basisTo.UTC().Round(0),
		workTo:    workTo.UTC().Round(0),
	}
	workWindow, err := canonical.WorkWindow()
	if err != nil {
		return OccurrenceTimes{}, err
	}
	basisWindow, err := canonical.BasisWindow()
	if err != nil {
		return OccurrenceTimes{}, err
	}
	inside := !basisWindow.From().Before(workWindow.From()) &&
		!basisWindow.Until().After(workWindow.Until())
	if !inside {
		return OccurrenceTimes{}, fmt.Errorf("basis observation must occur inside actual onboarding Work")
	}
	return canonical, nil
}

func (times OccurrenceTimes) WorkWindow() (projectprofile.WorkIntervalV1, error) {
	return projectprofile.NewWorkIntervalV1(times.workFrom, times.workTo)
}

func (times OccurrenceTimes) BasisWindow() (projectprofile.BasisObservationWindowV1, error) {
	return projectprofile.NewBasisObservationWindowV1(times.basisFrom, times.basisTo)
}

func (plan Plan) BuildValueSet(times OccurrenceTimes) (ValueSet, error) {
	workWindow, err := times.WorkWindow()
	if err != nil {
		return ValueSet{}, err
	}
	basisWindow, err := times.BasisWindow()
	if err != nil {
		return ValueSet{}, err
	}
	workAuthorized := plan.support.allowedWork.Contains(workWindow.From()) &&
		!workWindow.Until().After(plan.support.allowedWork.Until())
	basisAuthorized := plan.support.allowedBasis.Contains(basisWindow.From()) &&
		!basisWindow.Until().After(plan.support.allowedBasis.Until())
	if !workAuthorized || !basisAuthorized {
		return ValueSet{}, fmt.Errorf("actual onboarding Work falls outside the authorized future windows")
	}
	if workWindow.From().Before(plan.support.preparedAt) {
		return ValueSet{}, fmt.Errorf("profile-onboarding Work starts before its authority resolution")
	}
	basis, err := buildObservedBasis(plan, basisWindow)
	if err != nil {
		return ValueSet{}, err
	}
	work, effect, err := buildWorkAndEffect(plan, workWindow, basisWindow, basis)
	if err != nil {
		return ValueSet{}, err
	}
	assessment, err := buildAssessment(plan.refs, plan.support.contract, effect)
	if err != nil {
		return ValueSet{}, err
	}
	return ValueSet{
		description: plan.support.description,
		contract:    plan.support.contract,
		actors:      plan.support.actors,
		basis:       basis,
		work:        *work,
		effect:      effect,
		assessment:  assessment,
	}, nil
}

func (plan Plan) Candidate(
	values ValueSet,
	authorityBasis projectprofile.ProfileDeclarationAuthorityBasisRef,
) (projectprofile.ProfileDeclarationCandidateV1, error) {
	workDigest, err := projectprofile.DigestProfileOnboardingWorkRecord(values.work)
	if err != nil {
		return projectprofile.ProfileDeclarationCandidateV1{}, err
	}
	assignmentDigest, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(values.actors.assignment)
	if err != nil {
		return projectprofile.ProfileDeclarationCandidateV1{}, err
	}
	basisDigest, err := projectprofile.DigestObservedProjectBasisV1(values.basis)
	if err != nil {
		return projectprofile.ProfileDeclarationCandidateV1{}, err
	}
	assessmentDigest, err := projectprofile.DigestProfileOnboardingOutcomeAssessmentV1(values.assessment)
	if err != nil {
		return projectprofile.ProfileDeclarationCandidateV1{}, err
	}
	provenanceBuilder := projectprofile.NewCandidateProvenanceV1Builder(
		authorityBasis,
		values.work.RecordRef(),
		workDigest,
	)
	provenanceBuilder = provenanceBuilder.ForProject(plan.root)
	provenanceBuilder = provenanceBuilder.ForProfileAuthorRoleAssignment(
		values.actors.assignment.RoleAssignmentRef(),
		assignmentDigest,
	)
	provenanceBuilder = provenanceBuilder.ClassifiedBy(
		values.basis.ClassifierVersion(),
		values.actors.policy,
	)
	provenanceBuilder = provenanceBuilder.InSession(values.actors.provenance.SessionRef())
	provenanceBuilder = provenanceBuilder.ForPayload(plan.input.PayloadDigest())
	provenanceBuilder = provenanceBuilder.ForObservedProjectBasis(
		values.basis.Ref(),
		basisDigest,
	)
	provenanceBuilder = provenanceBuilder.ForOutcomeAssessment(
		values.assessment.Ref(),
		assessmentDigest,
	)
	provenance, err := provenanceBuilder.Build()
	if err != nil {
		return projectprofile.ProfileDeclarationCandidateV1{}, err
	}
	return projectprofile.NewProfileDeclarationCandidateV1(
		plan.input.Payload(),
		provenance,
	)
}

func (plan Plan) Root() projectprofile.ProjectRootV1 { return plan.root }
func (plan Plan) Input() ProfileOnboardingWorkInput  { return plan.input }
func (plan Plan) Policy() Policy                     { return plan.policy }
func (plan Plan) PreparedAt() time.Time              { return plan.support.preparedAt }
func (plan Plan) AllowedWork() authority.TimeWindow  { return plan.support.allowedWork }
func (plan Plan) AllowedBasis() authority.TimeWindow { return plan.support.allowedBasis }
func (plan Plan) DeclarationID() string              { return plan.support.declarationID }
func (plan Plan) SingleUseKey() string               { return plan.refs.singleUse }

func (plan Plan) AuthorityBasisRef() (
	projectprofile.ProfileDeclarationAuthorityBasisRef,
	error,
) {
	return projectprofile.NewProfileDeclarationAuthorityBasisRef(
		"profile-authority-basis:" + plan.support.declarationID,
	)
}

func (plan Plan) AuthorityResolutionRef() string {
	return "profile-authority-resolution:" + plan.support.declarationID
}

func (plan Plan) WorkRecordRef() (
	projectprofile.ProfileOnboardingWorkRecordRef,
	error,
) {
	return projectprofile.NewProfileOnboardingWorkRecordRef(plan.refs.workRecord)
}

func (plan Plan) AssessmentRef() (
	projectprofile.ProfileOnboardingOutcomeAssessmentRefV1,
	error,
) {
	return projectprofile.NewProfileOnboardingOutcomeAssessmentRefV1(
		plan.refs.assessment,
	)
}

func (support AuthoritySupport) MethodDescription() projectprofile.ProfileOnboardingMethodDescriptionV2 {
	return support.description
}

func (support AuthoritySupport) MethodContract() projectprofile.ProfileOnboardingMethodContractV2 {
	return support.contract
}

func (support AuthoritySupport) SystemAdmission() projectprofile.ProfileOnboardingExecutorSystemAdmissionV1 {
	return support.actors.systemAdmission
}

func (support AuthoritySupport) RoleAdmission() projectprofile.ProfileAuthorRoleAdmissionV1 {
	return support.actors.roleAdmission
}

func (support AuthoritySupport) AssignmentJustification() projectprofile.ProfileAuthorAssignmentJustificationV1 {
	return support.actors.justification
}

func (support AuthoritySupport) AssignmentProvenance() projectprofile.ProfileAuthorAssignmentProvenanceV1 {
	return support.actors.provenance
}

func (support AuthoritySupport) RoleAssignment() projectprofile.ProfileAuthorRoleAssignmentV1 {
	return support.actors.assignment
}

func (support AuthoritySupport) ClassifierVersion() projectprofile.ClassifierVersion {
	return support.actors.classifier
}

func (support AuthoritySupport) PolicyVersion() projectprofile.PolicyVersion {
	return support.actors.policy
}

func (support AuthoritySupport) SessionRef() projectprofile.SessionRef {
	return support.actors.session
}

func (plan Plan) Support() AuthoritySupport { return plan.support }

func (values ValueSet) MethodDescription() projectprofile.ProfileOnboardingMethodDescriptionV2 {
	return values.description
}

func (values ValueSet) MethodContract() projectprofile.ProfileOnboardingMethodContractV2 {
	return values.contract
}

func (values ValueSet) SystemAdmission() projectprofile.ProfileOnboardingExecutorSystemAdmissionV1 {
	return values.actors.systemAdmission
}

func (values ValueSet) RoleAdmission() projectprofile.ProfileAuthorRoleAdmissionV1 {
	return values.actors.roleAdmission
}

func (values ValueSet) AssignmentJustification() projectprofile.ProfileAuthorAssignmentJustificationV1 {
	return values.actors.justification
}

func (values ValueSet) AssignmentProvenance() projectprofile.ProfileAuthorAssignmentProvenanceV1 {
	return values.actors.provenance
}

func (values ValueSet) RoleAssignment() projectprofile.ProfileAuthorRoleAssignmentV1 {
	return values.actors.assignment
}

func (values ValueSet) ObservedBasis() projectprofile.ObservedProjectBasisV1 {
	return values.basis
}

func (values ValueSet) WorkRecord() projectprofile.ProfileOnboardingWorkRecord {
	return values.work
}

func (values ValueSet) Effect() projectprofile.ProfileOnboardingEffectV1 {
	return values.effect
}

func (values ValueSet) Assessment() projectprofile.ProfileOnboardingOutcomeAssessmentV1 {
	return values.assessment
}

func declarationIdentity(input ProfileOnboardingWorkInput, mode string) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte("haft.profile-declaration.identity/v3\x00"))
	_, _ = hash.Write([]byte(input.Digest().String()))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(mode))
	return hex.EncodeToString(hash.Sum(nil))
}

func newDeclarationRefs(
	identity string,
	input ProfileOnboardingWorkInput,
) (declarationRefs, error) {
	if identity == "" || !input.Valid() {
		return declarationRefs{}, fmt.Errorf("profile declaration identity and Work input are required")
	}
	return declarationRefs{
		token:                identity,
		session:              "session:profile-onboarding:" + identity,
		system:               "system:haft-kernel:" + identity,
		systemAdmission:      "system-admission:profile-onboarding:" + identity,
		actingEligibility:    "acting-eligibility:haft-profile-onboarding:" + identity,
		roleAdmission:        "role-admission:profile-author:" + identity,
		assignmentReason:     "assignment-justification:profile-author:" + identity,
		assignmentProvenance: "assignment-provenance:profile-author:" + identity,
		roleAssignment:       "role-assignment:profile-author:" + identity,
		observedBasis:        "observed-basis:explicit-profile-request:" + identity,
		workRecord:           "work-record:profile-onboarding:" + identity,
		work:                 "work:profile-onboarding:" + identity,
		output:               "output:profile-candidate:" + identity,
		resource:             "resource:profile-onboarding-cli:" + identity,
		affected:             "entity:project-profile:" + identity,
		preState:             "state:project-profile:unbound:" + identity,
		postState:            "state:project-profile:candidate-prepared:" + identity,
		effect:               "effect:profile-candidate-prepared:" + identity,
		effectEvidence:       "evidence:path:profile-candidate-effect:" + identity,
		assessment:           "assessment:profile-candidate:" + identity,
		assessmentEvidence:   "evidence:path:profile-candidate-assessment:" + identity,
		comparator:           "comparator:profile-onboarding-contract-v1",
		singleUse:            "single-use:profile-onboarding:" + identity,
	}, nil
}

func buildAuthoritySupport(
	refs declarationRefs,
	validFrom time.Time,
	validUntil time.Time,
	classifierVersion string,
	policyVersion string,
	allowedWork authority.TimeWindow,
	allowedBasis authority.TimeWindow,
	declarationID string,
) (AuthoritySupport, error) {
	description := projectprofile.ProfileOnboardingMethodDescriptionV2Value()
	contract, err := projectprofile.ProfileOnboardingMethodContractV2Value()
	if err != nil {
		return AuthoritySupport{}, err
	}
	actors, err := buildActors(
		refs,
		validFrom,
		validUntil,
		classifierVersion,
		policyVersion,
	)
	if err != nil {
		return AuthoritySupport{}, err
	}
	return AuthoritySupport{
		description:   description,
		contract:      contract,
		actors:        actors,
		allowedWork:   allowedWork,
		allowedBasis:  allowedBasis,
		validUntil:    validUntil.UTC().Round(0),
		preparedAt:    validFrom.UTC().Round(0),
		declarationID: declarationID,
	}, nil
}

func buildActors(
	refs declarationRefs,
	validFrom time.Time,
	validUntil time.Time,
	classifierVersion string,
	policyVersion string,
) (declarationActors, error) {
	session, err := parse(refs.session, projectprofile.NewSessionRef)
	if err != nil {
		return declarationActors{}, err
	}
	system, err := parse(refs.system, projectprofile.NewSystemRef)
	if err != nil {
		return declarationActors{}, err
	}
	classifier, err := parse(classifierVersion, projectprofile.NewClassifierVersion)
	if err != nil {
		return declarationActors{}, err
	}
	policy, err := parse(policyVersion, projectprofile.NewPolicyVersion)
	if err != nil {
		return declarationActors{}, err
	}
	kernel, err := projectprofile.NewProfileOnboardingKernelIdentityV1("haft-kernel", "v9")
	if err != nil {
		return declarationActors{}, err
	}
	runtime, err := projectprofile.NewProfileOnboardingRuntimeIdentityV1("haft-cli", "v9")
	if err != nil {
		return declarationActors{}, err
	}
	identityBasis, err := projectprofile.NewProfileOnboardingKernelExecutorIdentityBasisV1(system, kernel)
	if err != nil {
		return declarationActors{}, err
	}
	systemWindow, err := projectprofile.NewProfileOnboardingExecutorAdmissionWindowV1(
		validFrom,
		validUntil,
	)
	if err != nil {
		return declarationActors{}, err
	}
	actingRef, err := parse(
		refs.actingEligibility,
		projectprofile.NewProfileOnboardingSystemActingEligibilityBasisRefV1,
	)
	if err != nil {
		return declarationActors{}, err
	}
	actingDigest, err := digestStrings(
		"haft.profile-onboarding.acting-eligibility/v1",
		[]string{system.String(), session.String(), "local-haft-cli"},
	)
	if err != nil {
		return declarationActors{}, err
	}
	systemAdmissionRef, err := parse(refs.systemAdmission, projectprofile.NewSystemAdmissionRef)
	if err != nil {
		return declarationActors{}, err
	}
	systemBuilder := projectprofile.NewProfileOnboardingExecutorSystemAdmissionV1Builder(
		systemAdmissionRef,
		system,
	)
	systemBuilder = systemBuilder.IdentifiedBy(identityBasis)
	systemBuilder = systemBuilder.AdmittedToActBy(actingRef, actingDigest)
	systemBuilder = systemBuilder.InSession(session)
	systemBuilder = systemBuilder.ValidDuring(systemWindow)
	systemBuilder = systemBuilder.UsingMethodEditionV2()
	systemAdmission, err := systemBuilder.Build()
	if err != nil {
		return declarationActors{}, err
	}
	roleAdmissionRef, err := parse(refs.roleAdmission, projectprofile.NewRoleAdmissionRef)
	if err != nil {
		return declarationActors{}, err
	}
	roleAdmission, err := projectprofile.NewProfileAuthorRoleAdmissionV2(roleAdmissionRef)
	if err != nil {
		return declarationActors{}, err
	}
	assignmentWindow, err := projectprofile.NewRoleAssignmentWindowV1(validFrom, validUntil)
	if err != nil {
		return declarationActors{}, err
	}
	justificationRef, err := parse(
		refs.assignmentReason,
		projectprofile.NewRoleAssignmentJustificationRef,
	)
	if err != nil {
		return declarationActors{}, err
	}
	justificationBuilder := projectprofile.NewProfileAuthorAssignmentJustificationV1Builder(justificationRef)
	justificationBuilder = justificationBuilder.ApplyingAdmissions(systemAdmission, roleAdmission)
	justificationBuilder = justificationBuilder.ValidDuring(assignmentWindow)
	justification, err := justificationBuilder.Build()
	if err != nil {
		return declarationActors{}, err
	}
	provenanceRef, err := parse(
		refs.assignmentProvenance,
		projectprofile.NewRoleAssignmentProvenanceRef,
	)
	if err != nil {
		return declarationActors{}, err
	}
	provenanceBuilder := projectprofile.NewProfileAuthorAssignmentProvenanceV1Builder(
		provenanceRef,
		justification,
	)
	provenanceBuilder = provenanceBuilder.InSession(session)
	provenanceBuilder = provenanceBuilder.ProducedBy(kernel, runtime)
	provenanceBuilder = provenanceBuilder.RecordedAt(validFrom)
	provenance, err := provenanceBuilder.Build()
	if err != nil {
		return declarationActors{}, err
	}
	assignmentSupport, err := projectprofile.CarryProfileAuthorAssignmentSupportV1(
		systemAdmission,
		roleAdmission,
		justification,
		provenance,
	)
	if err != nil {
		return declarationActors{}, err
	}
	assignmentRef, err := parse(refs.roleAssignment, projectprofile.NewRoleAssignmentRef)
	if err != nil {
		return declarationActors{}, err
	}
	assignmentBuilder := projectprofile.NewProfileAuthorRoleAssignmentV1Builder(assignmentRef)
	assignmentBuilder = assignmentBuilder.HeldBy(system)
	assignmentBuilder = assignmentBuilder.Assigning(projectprofile.ProfileAuthorRoleRefV1())
	assignmentBuilder = assignmentBuilder.InContext(projectprofile.ProfileOnboardingBoundedContextRefV1())
	assignmentBuilder = assignmentBuilder.ValidDuring(assignmentWindow)
	assignmentBuilder = assignmentBuilder.WithSystemAdmission(
		systemAdmission.Ref(),
		assignmentSupport.SystemAdmissionDigest(),
	)
	assignmentBuilder = assignmentBuilder.WithRoleAdmission(
		roleAdmission.Ref(),
		assignmentSupport.RoleAdmissionDigest(),
	)
	assignmentBuilder = assignmentBuilder.JustifiedBy(
		justification.Ref(),
		assignmentSupport.JustificationDigest(),
	)
	assignmentBuilder = assignmentBuilder.WithProvenance(
		provenance.Ref(),
		assignmentSupport.ProvenanceDigest(),
	)
	assignment, err := assignmentBuilder.Build()
	if err != nil {
		return declarationActors{}, err
	}
	return declarationActors{
		session:         session,
		classifier:      classifier,
		policy:          policy,
		systemAdmission: systemAdmission,
		roleAdmission:   roleAdmission,
		justification:   justification,
		provenance:      provenance,
		assignment:      assignment,
	}, nil
}

func buildObservedBasis(
	plan Plan,
	window projectprofile.BasisObservationWindowV1,
) (projectprofile.ObservedProjectBasisV1, error) {
	var noBasis projectprofile.ObservedProjectBasisV1
	kind, err := parse(
		"profile-onboarding-work-input",
		projectprofile.NewObservedProjectSignalKindV1,
	)
	if err != nil {
		return noBasis, err
	}
	value, err := parse(
		profileObservedSignalValue(plan.input),
		projectprofile.NewObservedProjectSignalValueV1,
	)
	if err != nil {
		return noBasis, err
	}
	carrier, err := parse(plan.input.Ref().String(), projectprofile.NewSourceCarrierRefV1)
	if err != nil {
		return noBasis, err
	}
	evidence, err := parse(
		"evidence:path:"+plan.input.Ref().String(),
		projectprofile.NewEvidenceProvenancePathRefV1,
	)
	if err != nil {
		return noBasis, err
	}
	signal, err := projectprofile.NewObservedProjectSignalV1(
		kind,
		value,
		carrier,
		[]projectprofile.EvidenceProvenancePathRefV1{evidence},
	)
	if err != nil {
		return noBasis, err
	}
	basisRef, err := parse(plan.refs.observedBasis, projectprofile.NewObservedProjectBasisRefV1)
	if err != nil {
		return noBasis, err
	}
	detector, err := parse(
		plan.input.ObservationDetectorVersion(),
		projectprofile.NewObservedProjectDetectorVersionV1,
	)
	if err != nil {
		return noBasis, err
	}
	return projectprofile.NewObservedProjectBasisV1(
		basisRef,
		plan.root,
		window,
		[]projectprofile.ObservedProjectSignalV1{signal},
		detector,
		plan.support.actors.classifier,
	)
}

func profileObservedSignalValue(
	input ProfileOnboardingWorkInput,
) string {
	if input.UsesManualScopeBasis() {
		return "complete repository observation " +
			input.ObservationDigest() +
			" mapped by manual scope proposal " +
			input.Ref().String()
	}
	return "detector observation " +
		input.ObservationDigest() +
		" mapped by " +
		input.Ref().String()
}

func buildWorkAndEffect(
	plan Plan,
	workWindow projectprofile.WorkIntervalV1,
	basisWindow projectprofile.BasisObservationWindowV1,
	basis projectprofile.ObservedProjectBasisV1,
) (*projectprofile.ProfileOnboardingWorkRecord, projectprofile.ProfileOnboardingEffectV1, error) {
	description := plan.support.description
	contract := plan.support.contract
	descriptionDigest, err := projectprofile.DigestProfileOnboardingMethodDescriptionV2(description)
	if err != nil {
		return nil, nil, err
	}
	contractDigest, err := projectprofile.DigestProfileOnboardingMethodContractV2(contract)
	if err != nil {
		return nil, nil, err
	}
	assignmentDigest, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(plan.support.actors.assignment)
	if err != nil {
		return nil, nil, err
	}
	basisDigest, err := projectprofile.DigestObservedProjectBasisV1(basis)
	if err != nil {
		return nil, nil, err
	}
	bindings, err := newMethodBindings(plan.root, plan.support.actors)
	if err != nil {
		return nil, nil, err
	}
	preState, err := parse(plan.refs.preState, projectprofile.NewStateRef)
	if err != nil {
		return nil, nil, err
	}
	postState, err := parse(plan.refs.postState, projectprofile.NewStateRef)
	if err != nil {
		return nil, nil, err
	}
	transition, err := projectprofile.NewPrePostStateTransitionV1(preState, postState)
	if err != nil {
		return nil, nil, err
	}
	outcome, err := projectprofile.NewCandidatePayloadProduced(
		plan.input.PayloadDigest(),
		basisDigest,
	)
	if err != nil {
		return nil, nil, err
	}
	recordRef, err := parse(plan.refs.workRecord, projectprofile.NewProfileOnboardingWorkRecordRef)
	if err != nil {
		return nil, nil, err
	}
	workRef, err := parse(plan.refs.work, projectprofile.NewWorkRef)
	if err != nil {
		return nil, nil, err
	}
	basisInputRef, err := parse(basis.Ref().String(), projectprofile.NewWorkInputRef)
	if err != nil {
		return nil, nil, err
	}
	outputRef, err := parse(plan.refs.output, projectprofile.NewWorkOutputRef)
	if err != nil {
		return nil, nil, err
	}
	resourceRef, err := parse(plan.refs.resource, projectprofile.NewWorkResourceRef)
	if err != nil {
		return nil, nil, err
	}
	affectedRef, err := parse(plan.refs.affected, projectprofile.NewAffectedReferentRef)
	if err != nil {
		return nil, nil, err
	}
	workBuilder := projectprofile.NewProfileOnboardingWorkRecordBuilder(recordRef, workRef)
	workBuilder = workBuilder.Enacts(description.DescribedMethodRef(), description.Ref(), bindings)
	workBuilder = workBuilder.WithMethodDescriptionDigest(descriptionDigest)
	workBuilder = workBuilder.GovernedByMethodContract(contract.Ref(), contractDigest)
	workBuilder = workBuilder.PerformedUnderAssignment(plan.support.actors.assignment.RoleAssignmentRef())
	workBuilder = workBuilder.WithProfileAuthorRoleAssignment(
		plan.support.actors.assignment.RoleAssignmentRef(),
		assignmentDigest,
	)
	workBuilder = workBuilder.ActualPerformer(plan.support.actors.assignment.HolderSystemRef())
	workBuilder = workBuilder.InContext(description.BoundedContextRef())
	workBuilder = workBuilder.During(workWindow, basisWindow)
	workBuilder = workBuilder.WithObservedProjectBasis(basis.Ref(), basisDigest)
	workBuilder = workBuilder.WithInputs([]projectprofile.WorkInputRef{
		basisInputRef,
		plan.input.Ref(),
	})
	workBuilder = workBuilder.WithOutputs([]projectprofile.WorkOutputRef{outputRef})
	workBuilder = workBuilder.WithResources([]projectprofile.WorkResourceRef{resourceRef})
	workBuilder = workBuilder.AffectingKind(description.AffectedRefKind())
	workBuilder = workBuilder.Affecting([]projectprofile.AffectedReferentRef{affectedRef})
	workBuilder = workBuilder.OnStatePlane(description.StatePlaneRef(), transition)
	workBuilder = workBuilder.WithOutcome(outcome)
	work, err := workBuilder.Build()
	if err != nil {
		return nil, nil, err
	}
	workDigest, err := projectprofile.DigestProfileOnboardingWorkRecord(work)
	if err != nil {
		return nil, nil, err
	}
	result, err := projectprofile.NewProfileOnboardingCandidateResultV1(
		outputRef,
		plan.input.PayloadDigest(),
		basis.Ref(),
		basisDigest,
	)
	if err != nil {
		return nil, nil, err
	}
	effectRef, err := parse(plan.refs.effect, projectprofile.NewProfileOnboardingEffectRefV1)
	if err != nil {
		return nil, nil, err
	}
	entityRef, err := parse(plan.refs.affected, projectprofile.NewEntityRef)
	if err != nil {
		return nil, nil, err
	}
	effectEvidence, err := parse(
		plan.refs.effectEvidence,
		projectprofile.NewEvidenceProvenancePathRefV1,
	)
	if err != nil {
		return nil, nil, err
	}
	effect, err := projectprofile.NewProfileOnboardingEffectV1(
		effectRef,
		work.RecordRef(),
		work.WorkRef(),
		workDigest,
		result,
		[]projectprofile.EntityRef{entityRef},
		description.StatePlaneRef(),
		transition,
		[]projectprofile.EvidenceProvenancePathRefV1{effectEvidence},
	)
	if err != nil {
		return nil, nil, err
	}
	return &work, effect, nil
}

func buildAssessment(
	refs declarationRefs,
	contract projectprofile.ProfileOnboardingMethodContractV2,
	effect projectprofile.ProfileOnboardingEffectV1,
) (projectprofile.ProfileOnboardingOutcomeAssessmentV1, error) {
	assessmentRef, err := parse(
		refs.assessment,
		projectprofile.NewProfileOnboardingOutcomeAssessmentRefV1,
	)
	if err != nil {
		return nil, err
	}
	standardEdition, err := projectprofile.NewProfileOnboardingAcceptanceStandardEditionV1(
		contract.AcceptanceStandardEdition(),
	)
	if err != nil {
		return nil, err
	}
	comparatorRef, err := parse(refs.comparator, projectprofile.NewProfileOnboardingComparatorRefV1)
	if err != nil {
		return nil, err
	}
	comparatorEdition, err := projectprofile.NewProfileOnboardingComparatorEditionV1("v1")
	if err != nil {
		return nil, err
	}
	evidence, err := parse(
		refs.assessmentEvidence,
		projectprofile.NewEvidenceProvenancePathRefV1,
	)
	if err != nil {
		return nil, err
	}
	return projectprofile.NewProfileOnboardingOutcomeAssessmentV1(
		assessmentRef,
		effect,
		contract.AcceptanceStandardRef(),
		standardEdition,
		comparatorRef,
		comparatorEdition,
		projectprofile.ProfileOnboardingAcceptancePassedV1Value(),
		[]projectprofile.EvidenceProvenancePathRefV1{evidence},
	)
}

func newMethodBindings(
	root projectprofile.ProjectRootV1,
	actors declarationActors,
) (projectprofile.MethodParameterBindings, error) {
	raw := []struct {
		name  string
		value string
	}{
		{name: "classifier_version", value: actors.classifier.String()},
		{name: "policy_version", value: actors.policy.String()},
		{name: "project_root", value: root.String()},
		{name: "session_ref", value: actors.session.String()},
	}
	bindings := make([]projectprofile.MethodParameterBinding, len(raw))
	if err := buildMethodBindings(raw, bindings, 0); err != nil {
		return projectprofile.MethodParameterBindings{}, err
	}
	return projectprofile.NewMethodParameterBindings(bindings)
}

func buildMethodBindings(
	raw []struct {
		name  string
		value string
	},
	bindings []projectprofile.MethodParameterBinding,
	index int,
) error {
	if index == len(raw) {
		return nil
	}
	value, err := projectprofile.NewMethodParameterBinding(raw[index].name, raw[index].value)
	if err != nil {
		return err
	}
	bindings[index] = value
	return buildMethodBindings(raw, bindings, index+1)
}

type digestWriter struct{ hash hash.Hash }

func newDigestWriter(domain string) digestWriter {
	writer := digestWriter{hash: sha256.New()}
	writer.add(domain)
	return writer
}

func (writer digestWriter) add(value string) {
	_, _ = writer.hash.Write([]byte(fmt.Sprintf("%d:%s", len(value), value)))
}

func digestStrings(domain string, values []string) (projectprofile.ContentDigest, error) {
	writer := newDigestWriter(domain)
	addDigestStrings(writer, values, 0)
	raw := "sha256:" + hex.EncodeToString(writer.hash.Sum(nil))
	return projectprofile.NewContentDigest(raw)
}

func addDigestStrings(writer digestWriter, values []string, index int) {
	if index == len(values) {
		return
	}
	writer.add(values[index])
	addDigestStrings(writer, values, index+1)
}

func parse[T any](raw string, parser func(string) (T, error)) (T, error) {
	value, err := parser(raw)
	if err != nil {
		var zero T
		return zero, err
	}
	return value, nil
}
