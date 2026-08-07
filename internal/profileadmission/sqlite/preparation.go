package sqlite

import (
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileauthority"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	projectprofilesqlite "github.com/m0n0x41d/haft/internal/projectprofile/sqlite"
)

type authorityMaterial struct {
	admittedUse              profileauthority.AdmittedUse
	resolution               profileauthority.AuthorityResolutionRecord
	resolutionRef            profileauthority.ProfileDeclarationAuthorityResolutionRef
	resolutionDigest         authority.Digest
	authorityBasisRef        profileauthority.BasisRef
	authorityBasisDigest     authority.Digest
	permissionRef            authority.PermissionRef
	permissionDigest         authority.Digest
	authorizationContentRef  authority.AuthorizationContentRef
	authorizationContentHash authority.Digest
	projectRoot              authority.ProjectRoot
	actionKind               authority.ActionKind
	projectBindingHash       authority.Digest
	actionEnvelopeHash       authority.Digest
	profileAuthorRef         authority.RoleAssignmentRef
	profileAuthorDigest      authority.Digest
	methodDescriptionRef     authority.MethodDescriptionRef
	methodDescriptionDigest  authority.Digest
	methodContractRef        authority.MethodContractRef
	methodContractDigest     authority.Digest
	classifierVersion        authority.ClassifierVersion
	policyVersion            authority.PolicyVersion
	futureWorkSession        authority.SessionRef
	allowedWork              authority.TimeWindow
	allowedBasisObservation  authority.TimeWindow
	permissionValidity       authority.TimeWindow
	singleUseKey             authority.SingleUseKey
	verifierIdentity         authority.VerifierIdentity
	verifierVersion          authority.VerifierVersion
	checkedAt                time.Time
	judgementTime            time.Time
	authorityMode            string
	resolutionKind           string
	workInputRef             string
	workInputDigest          string
	permissionRequired       bool
}

func materializeAuthority(
	use profileauthority.AdmittedUse,
) (authorityMaterial, error) {
	resolution, resolutionOK := use.AuthorityResolutionRecord()
	resolutionRef, resolutionDigest, resolutionIdentityOK := use.Resolution()
	authorityBasisRef, authorityBasisDigest, basisOK := use.Basis()
	permissionRef, permissionDigest, permissionOK := use.Permission()
	contentRef, contentDigest, contentOK := use.AuthorizationContent()
	projectRoot, actionKind, projectBindingHash, projectOK := use.ProjectBinding()
	actionEnvelopeHash, envelopeOK := use.ActionEnvelopeDigest()
	singleUseKey, singleUseOK := use.SingleUseKey()
	judgementTime, judgementOK := use.JudgedAt()
	profileAuthorRef, profileAuthorDigest, authorOK := resolution.ProfileAuthor()
	methodDescriptionRef, methodDescriptionDigest, methodDescriptionOK := resolution.MethodDescription()
	methodContractRef, methodContractDigest, methodContractOK := resolution.MethodContract()
	classifierVersion, policyVersion, versionsOK := resolution.Versions()
	futureWorkSession, sessionOK := resolution.FutureWorkSession()
	allowedWork, allowedWorkOK := resolution.AllowedWorkWindow()
	allowedBasis, allowedBasisOK := resolution.AllowedBasisObservationWindow()
	permissionValidity, permissionValidityOK := resolution.PermissionValidity()
	verifierIdentity, verifierVersion, _, _, verifierOK := resolution.Verifier()
	checkedAt, checkedAtOK := resolution.CheckedAt()
	allPresent := resolutionOK && resolutionIdentityOK && basisOK && permissionOK &&
		contentOK && projectOK && envelopeOK && singleUseOK && judgementOK &&
		authorOK && methodDescriptionOK && methodContractOK && versionsOK &&
		sessionOK && allowedWorkOK && allowedBasisOK && permissionValidityOK &&
		verifierOK && checkedAtOK
	if !allPresent {
		return authorityMaterial{}, fmt.Errorf("sealed profile authority use is incomplete")
	}
	return authorityMaterial{
		admittedUse:              use,
		resolution:               resolution,
		resolutionRef:            resolutionRef,
		resolutionDigest:         resolutionDigest,
		authorityBasisRef:        authorityBasisRef,
		authorityBasisDigest:     authorityBasisDigest,
		permissionRef:            permissionRef,
		permissionDigest:         permissionDigest,
		authorizationContentRef:  contentRef,
		authorizationContentHash: contentDigest,
		projectRoot:              projectRoot,
		actionKind:               actionKind,
		projectBindingHash:       projectBindingHash,
		actionEnvelopeHash:       actionEnvelopeHash,
		profileAuthorRef:         profileAuthorRef,
		profileAuthorDigest:      profileAuthorDigest,
		methodDescriptionRef:     methodDescriptionRef,
		methodDescriptionDigest:  methodDescriptionDigest,
		methodContractRef:        methodContractRef,
		methodContractDigest:     methodContractDigest,
		classifierVersion:        classifierVersion,
		policyVersion:            policyVersion,
		futureWorkSession:        futureWorkSession,
		allowedWork:              allowedWork,
		allowedBasisObservation:  allowedBasis,
		permissionValidity:       permissionValidity,
		singleUseKey:             singleUseKey,
		verifierIdentity:         verifierIdentity,
		verifierVersion:          verifierVersion,
		checkedAt:                checkedAt.UTC(),
		judgementTime:            judgementTime.UTC(),
	}, nil
}

func prepareAdmission(
	candidate projectprofile.ProfileDeclarationCandidateV1,
	values projectprofilesqlite.ProfileOnboardingValueSetV1,
	authorityValue authorityMaterial,
	expectedRevision projectprofile.LedgerRevision,
) (projectprofile.PreparedProfileAdmissionV1, error) {
	err := validateCandidateAgainstAuthority(candidate, values, authorityValue)
	if err != nil {
		return nil, err
	}
	inputs, err := projectprofile.NewProfileDeclarationAdmissionInputs(
		candidate,
		expectedRevision,
	)
	if err != nil {
		return nil, fmt.Errorf("construct profile-admission inputs: %w", err)
	}
	resolutionRef, err := projectprofile.NewAuthorityResolutionRecordRef(
		authorityValue.resolutionRef.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("convert authority-resolution ref: %w", err)
	}
	resolutionDigest, err := projectprofile.NewContentDigest(
		authorityValue.resolutionDigest.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("convert authority-resolution digest: %w", err)
	}
	authoritySingleUseKey := authorityValue.singleUseKey
	singleUseKeyText := authoritySingleUseKey.String()
	singleUseKey, err := projectprofile.NewSingleUseKey(singleUseKeyText)
	if err != nil {
		return nil, fmt.Errorf("convert single-use key: %w", err)
	}
	plan, err := projectprofile.NewProfileDeclarationCommitPlan(
		inputs,
		resolutionRef,
		resolutionDigest,
		singleUseKey,
	)
	if err != nil {
		return nil, fmt.Errorf("construct profile-admission commit plan: %w", err)
	}
	systemAdmission := values.SystemAdmission()
	roleAdmission := values.RoleAdmission()
	assignmentJustification := values.AssignmentJustification()
	assignmentProvenance := values.AssignmentProvenance()
	support, err := projectprofile.CarryProfileAuthorAssignmentSupportV1(
		systemAdmission,
		roleAdmission,
		assignmentJustification,
		assignmentProvenance,
	)
	if err != nil {
		return nil, fmt.Errorf("reconstruct ProfileAuthor assignment support: %w", err)
	}
	provenance := candidate.Provenance()
	projectRoot := provenance.ProjectRoot()
	builder := projectprofile.NewProfileAdmissionPreparationV1Builder(plan, projectRoot)
	workRecord := values.WorkRecord()
	builder, err = withPreparedAdmissionWorkEdition(builder, workRecord, values)
	if err != nil {
		return nil, err
	}
	roleAssignment := values.RoleAssignment()
	builder = builder.WithProfileAuthor(roleAssignment, support)
	observedBasis := values.ObservedBasis()
	effect := values.Effect()
	assessment := values.Assessment()
	builder = builder.WithObservedOutcome(observedBasis, effect, assessment)
	prepared, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("prepare canonical profile admission: %w", err)
	}
	return prepared, nil
}

func withPreparedAdmissionWorkEdition(
	builder projectprofile.ProfileAdmissionPreparationV1Builder,
	work projectprofile.ProfileOnboardingWorkRecord,
	values projectprofilesqlite.ProfileOnboardingValueSetV1,
) (projectprofile.ProfileAdmissionPreparationV1Builder, error) {
	switch description := values.MethodDescriptionEdition().(type) {
	case projectprofile.ProfileOnboardingMethodDescriptionV1:
		contract, ok := values.MethodContractEdition().(projectprofile.ProfileOnboardingMethodContractV1)
		if !ok {
			return builder, fmt.Errorf("prepare profile admission: profile-onboarding method editions differ")
		}
		return builder.WithWork(work, description, contract), nil
	case projectprofile.ProfileOnboardingMethodDescriptionV2:
		contract, ok := values.MethodContractEdition().(projectprofile.ProfileOnboardingMethodContractV2)
		if !ok {
			return builder, fmt.Errorf("prepare profile admission: profile-onboarding method editions differ")
		}
		return builder.WithWorkV2(work, description, contract), nil
	default:
		return builder, fmt.Errorf("prepare profile admission: profile-onboarding method edition is unsupported")
	}
}

func validateCandidateAgainstAuthority(
	candidate projectprofile.ProfileDeclarationCandidateV1,
	values projectprofilesqlite.ProfileOnboardingValueSetV1,
	authorityValue authorityMaterial,
) error {
	provenance := candidate.Provenance()
	work := values.WorkRecord()
	assignmentDigest := provenance.ProfileAuthorRoleAssignmentDigest()
	methodDescriptionDigest := work.MethodDescriptionDigest()
	methodContractRef := work.MethodContractRef()
	methodContractDigest := work.MethodContractDigest()
	workWindow := work.WorkInterval()
	basisWindow := work.BasisObservationWindow()
	deferredRequirements, err := evaluateProfileOnboardingOccurrenceAgainstEdition(
		values,
		authorityValue,
	)
	if err != nil {
		return fmt.Errorf("evaluate profile-onboarding occurrence contract: %w", err)
	}
	requirements := authorityCoverageRequirements(deferredRequirements, 0)
	workFrom := workWindow.From()
	workUntil := workWindow.Until()
	basisFrom := basisWindow.From()
	basisUntil := basisWindow.Until()
	err = dischargeAuthorityCoverageRequirements(
		requirements,
		workFrom,
		workUntil,
		basisFrom,
		basisUntil,
		authorityValue.allowedWork,
		authorityValue.allowedBasisObservation,
	)
	if err != nil {
		return err
	}
	authorityBasisRef := provenance.AuthorityBasisRef()
	projectRoot := provenance.ProjectRoot()
	authorizedRoot := authorityValue.projectRoot
	assignmentRef := provenance.ProfileAuthorRoleAssignmentRef()
	authorizedAssignmentRef := authorityValue.profileAuthorRef
	authorizedAssignmentDigest := authorityValue.profileAuthorDigest
	methodDescriptionRef := work.MethodDescriptionRef()
	authorizedMethodDescriptionRef := authorityValue.methodDescriptionRef
	authorizedMethodDescriptionDigest := authorityValue.methodDescriptionDigest
	authorizedMethodContractRef := authorityValue.methodContractRef
	authorizedMethodContractDigest := authorityValue.methodContractDigest
	classifierVersion := provenance.ClassifierVersion()
	authorizedClassifierVersion := authorityValue.classifierVersion
	policyVersion := provenance.PolicyVersion()
	authorizedPolicyVersion := authorityValue.policyVersion
	sessionRef := provenance.SessionRef()
	authorizedSessionRef := authorityValue.futureWorkSession
	checks := []struct {
		matches bool
		name    string
	}{
		{
			matches: authorityBasisRef.String() == authorityValue.authorityBasisRef.String(),
			name:    "authority-basis ref",
		},
		{
			matches: projectRoot.String() == authorizedRoot.String(),
			name:    "project root",
		},
		{
			matches: assignmentRef.String() == authorizedAssignmentRef.String(),
			name:    "ProfileAuthor RoleAssignment",
		},
		{
			matches: assignmentDigest.String() == authorizedAssignmentDigest.String(),
			name:    "ProfileAuthor RoleAssignment digest",
		},
		{
			matches: methodDescriptionRef.String() == authorizedMethodDescriptionRef.String(),
			name:    "MethodDescription",
		},
		{
			matches: methodDescriptionDigest.String() == authorizedMethodDescriptionDigest.String(),
			name:    "MethodDescription digest",
		},
		{
			matches: methodContractRef.String() == authorizedMethodContractRef.String(),
			name:    "MethodContract",
		},
		{
			matches: methodContractDigest.String() == authorizedMethodContractDigest.String(),
			name:    "MethodContract digest",
		},
		{
			matches: classifierVersion.String() == authorizedClassifierVersion.String(),
			name:    "classifier version",
		},
		{
			matches: policyVersion.String() == authorizedPolicyVersion.String(),
			name:    "policy version",
		},
		{
			matches: sessionRef.String() == authorizedSessionRef.String(),
			name:    "session ref",
		},
		{
			matches: !workFrom.Before(authorityValue.checkedAt),
			name:    "Work start after authority resolution",
		},
		{
			matches: !authorityValue.judgementTime.Before(workUntil),
			name:    "authority-use time after Work completion",
		},
		{
			matches: !authorityValue.permissionRequired ||
				authorityValue.permissionValidity.Contains(authorityValue.judgementTime),
			name: "permission current at authority use",
		},
	}
	return firstMismatch(checks, "candidate authority resolution")
}

func evaluateProfileOnboardingOccurrenceAgainstEdition(
	values projectprofilesqlite.ProfileOnboardingValueSetV1,
	authorityValue authorityMaterial,
) ([]projectprofile.ProfileOnboardingDeferredAuthorityCoverageRequirementV1, error) {
	work := values.WorkRecord()
	assignment := values.RoleAssignment()
	basis := values.ObservedBasis()
	switch description := values.MethodDescriptionEdition().(type) {
	case projectprofile.ProfileOnboardingMethodDescriptionV1:
		contract, ok := values.MethodContractEdition().(projectprofile.ProfileOnboardingMethodContractV1)
		if !ok {
			return nil, fmt.Errorf("profile-onboarding MethodDescription and MethodContract editions differ")
		}
		evaluation, err := projectprofile.EvaluateProfileOnboardingOccurrenceContractV1(
			work,
			description,
			contract,
			assignment,
			basis,
		)
		if err != nil {
			return nil, err
		}
		return evaluation.DeferredAuthorityCoverageRequirements(), nil
	case projectprofile.ProfileOnboardingMethodDescriptionV2:
		contract, ok := values.MethodContractEdition().(projectprofile.ProfileOnboardingMethodContractV2)
		if !ok {
			return nil, fmt.Errorf("profile-onboarding MethodDescription and MethodContract editions differ")
		}
		workInputRef, err := projectprofile.NewWorkInputRef(authorityValue.workInputRef)
		if err != nil {
			return nil, fmt.Errorf("parse authority-bound ProfileOnboardingWorkInput ref: %w", err)
		}
		evaluation, err := projectprofile.EvaluateProfileOnboardingOccurrenceContractV2(
			work,
			description,
			contract,
			assignment,
			basis,
			workInputRef,
		)
		if err != nil {
			return nil, err
		}
		return evaluation.DeferredAuthorityCoverageRequirements(), nil
	default:
		return nil, fmt.Errorf("profile-onboarding MethodDescription edition is unsupported")
	}
}

const (
	authorityCoversWorkRule  = "haft:rule:profile-onboarding/authority-covers-work/v1"
	authorityCoversBasisRule = "haft:rule:profile-onboarding/authority-covers-basis-observation/v1"
	workIntervalSlot         = "work_interval"
	basisObservationSlot     = "basis_observation_window"
)

type authorityCoverageRequirement struct {
	rule string
	slot string
}

type authorityCoverageDischarger struct {
	slot      string
	discharge func() bool
}

func authorityCoverageRequirements(
	values []projectprofile.ProfileOnboardingDeferredAuthorityCoverageRequirementV1,
	index int,
) []authorityCoverageRequirement {
	if index >= len(values) {
		return []authorityCoverageRequirement{}
	}
	ruleRef := values[index].RuleRef()
	occurrenceSlot := values[index].OccurrenceSlot()
	current := authorityCoverageRequirement{
		rule: ruleRef.String(),
		slot: occurrenceSlot.String(),
	}
	rest := authorityCoverageRequirements(values, index+1)
	return append([]authorityCoverageRequirement{current}, rest...)
}

func dischargeAuthorityCoverageRequirements(
	requirements []authorityCoverageRequirement,
	workFrom time.Time,
	workUntil time.Time,
	basisFrom time.Time,
	basisUntil time.Time,
	allowedWork authority.TimeWindow,
	allowedBasis authority.TimeWindow,
) error {
	dischargers := map[string]authorityCoverageDischarger{
		authorityCoversWorkRule: {
			slot: workIntervalSlot,
			discharge: func() bool {
				startsWithin := !workFrom.Before(allowedWork.From())
				endsWithin := !workUntil.After(allowedWork.Until())
				return startsWithin && endsWithin
			},
		},
		authorityCoversBasisRule: {
			slot: basisObservationSlot,
			discharge: func() bool {
				startsWithin := !basisFrom.Before(allowedBasis.From())
				endsWithin := !basisUntil.After(allowedBasis.Until())
				return startsWithin && endsWithin
			},
		},
	}
	consumed := map[string]bool{}
	err := consumeAuthorityCoverageRequirements(requirements, dischargers, consumed, 0)
	if err != nil {
		return err
	}
	if len(consumed) != len(dischargers) {
		return fmt.Errorf("authority envelope does not discharge the exact final-v1 deferred requirement set")
	}
	return nil
}

func consumeAuthorityCoverageRequirements(
	requirements []authorityCoverageRequirement,
	dischargers map[string]authorityCoverageDischarger,
	consumed map[string]bool,
	index int,
) error {
	if index >= len(requirements) {
		return nil
	}
	requirement := requirements[index]
	discharger, known := dischargers[requirement.rule]
	if !known {
		return fmt.Errorf("unknown deferred authority-coverage rule %q", requirement.rule)
	}
	if consumed[requirement.rule] {
		return fmt.Errorf("duplicate deferred authority-coverage rule %q", requirement.rule)
	}
	if requirement.slot != discharger.slot {
		return fmt.Errorf("deferred authority-coverage rule %q names unexpected slot %q", requirement.rule, requirement.slot)
	}
	if !discharger.discharge() {
		return fmt.Errorf("authority envelope does not cover deferred occurrence slot %q", requirement.slot)
	}
	consumed[requirement.rule] = true
	return consumeAuthorityCoverageRequirements(requirements, dischargers, consumed, index+1)
}
