package profileonboarding

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/m0n0x41d/haft/internal/operatorrequest"
	"github.com/m0n0x41d/haft/internal/profileadmission"
	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
	profiledeclarationpreparationsqlite "github.com/m0n0x41d/haft/internal/profiledeclarationpreparation/sqlite"
	"github.com/m0n0x41d/haft/internal/profiledetector"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

// RunAutomaticInitialProfileBootstrap applies only the exact complete,
// supported singleton suggestion supplied by haft init. The automatic policy
// constructor rechecks that closed condition before any durable preparation.
func RunAutomaticInitialProfileBootstrap(
	ctx context.Context,
	database *sql.DB,
	projectRoot string,
	suggestion profiledetector.Suggestion,
	revalidate profileLedgerRevalidator,
) (Result, error) {
	content, err := ProposeProfileOnboardingWorkInput(suggestion)
	if err != nil {
		return Result{}, err
	}
	input, err := DecodeProfileOnboardingWorkInput(content, suggestion)
	if err != nil {
		return Result{}, err
	}
	policy, err := profiledeclarationpreparation.
		NewAutomaticSupportedSingletonPolicy(suggestion)
	if err != nil {
		return Result{}, err
	}
	service, err := newService(database, revalidate)
	if err != nil {
		return Result{}, err
	}
	return service.DeclareProfile(ctx, projectRoot, input, policy), nil
}

// RunProfileDeclaration is the public application boundary for declaring the
// exact, operator-reviewed profile input. The caller supplies semantic input
// and the effective project policy, never database refs, digests, or an
// authority packet.
func RunProfileDeclaration(
	ctx context.Context,
	database *sql.DB,
	projectRoot string,
	input ProfileOnboardingWorkInput,
	policy ProfileDeclarationPolicy,
	revalidate profileLedgerRevalidator,
) (Result, error) {
	service, err := newService(database, revalidate)
	if err != nil {
		return Result{}, err
	}
	result := service.DeclareProfile(ctx, projectRoot, input, policy)
	return result, nil
}

// RunProfileEntityRelationChange applies one exact predecessor-pinned review.
// It is intentionally separate from initial declaration and can express only
// a single existing scope's entity_ref replacement.
func RunProfileEntityRelationChange(
	ctx context.Context,
	database *sql.DB,
	projectRoot string,
	input ProfileOnboardingWorkInput,
	policy ProfileDeclarationPolicy,
	revalidate profileLedgerRevalidator,
) (Result, error) {
	service, err := newService(database, revalidate)
	if err != nil {
		return Result{}, err
	}
	result := service.ChangeProfileEntityRelation(
		ctx,
		projectRoot,
		input,
		policy,
	)
	return result, nil
}

// DeclareProfile turns one exact reviewed Work input into performed Work and a
// canonical profile admission. It is intentionally an initial-declaration
// operation; changing an already-declared payload is a separate contract.
func (service Service) DeclareProfile(
	ctx context.Context,
	projectRoot string,
	input ProfileOnboardingWorkInput,
	policy ProfileDeclarationPolicy,
) Result {
	root, failure := service.validateProfileDeclaration(
		ctx,
		projectRoot,
		input,
		policy,
	)
	if failure.Kind() != "" {
		return failure
	}
	if !policyCarriesEffect(policy, operatorrequest.ProfileDeclaration) &&
		policy.Mode() != ProfileDeclarationModeAutomaticSupportedSingleton {
		return rejectedResult(
			"profile_declaration_authority_mismatch",
			"initial profile declaration requires its own exact operator effect",
		)
	}
	current := service.state.admission.ResolveCurrent(ctx, root)
	currentPresent := admissionResultPresent(current)
	overrideDetectorDefault := false
	if currentPresent {
		admission, _ := current.Admission()
		overrideDetectorDefault = explicitPolicyMayOverrideDetectorDefault(
			policy,
			admission,
		)
		if admission.PayloadDigest() == input.PayloadDigest() &&
			!overrideDetectorDefault {
			return service.projectAdmission(ctx, current)
		}
		if !overrideDetectorDefault {
			return rejectedResult(
				"profile_change_requires_separate_action",
				"the current profile has another payload; only a direct host-routed operator request may supersede an automatic detector_default profile",
			)
		}
	}
	if !currentPresent && !admissionResultAbsent(current) {
		return service.projectAdmission(ctx, current)
	}
	if !overrideDetectorDefault {
		committed := service.state.admission.ResolveCommittedForPayload(
			ctx,
			root,
			input.PayloadDigest(),
		)
		if admissionResultPresent(committed) {
			return service.projectExistingAdmissionForPayload(
				ctx,
				committed,
				input.PayloadDigest(),
			)
		}
		if !admissionResultMissingCommitted(committed) {
			return service.projectAdmission(ctx, committed)
		}
	}
	if err := service.state.revalidate(ctx); err != nil {
		return failedResult(
			"ledger_revalidation",
			"pre_authority_revalidation_failed",
			err.Error(),
		)
	}
	preparation, err := profiledeclarationpreparationsqlite.PrepareBeforeAdmission(
		ctx,
		service.state.database,
		root.String(),
		input,
		policy,
		service.state.now,
		profiledeclarationpreparationsqlite.Revalidator(service.state.revalidate),
	)
	if err != nil {
		return failedResult(
			"pre_admission_preparation",
			"profile_declaration_preparation_failed",
			err.Error(),
		)
	}
	if preparation.Kind() == profiledeclarationpreparationsqlite.OutcomeConflict {
		detail, ok := preparation.ConflictDetail()
		if !ok {
			detail = "profile declaration preparation found an unnamed durable conflict"
		}
		return rejectedResult("profile_declaration_preparation_conflict", detail)
	}
	prepared, ok := preparation.Prepared()
	if !ok {
		return failedResult(
			"pre_admission_preparation",
			"prepared_result_missing",
			"profile declaration preparation omitted its sealed prepared result",
		)
	}
	if err := service.state.revalidate(ctx); err != nil {
		return failedResult(
			"ledger_revalidation",
			"post_work_revalidation_failed",
			err.Error(),
		)
	}
	candidate, ok := prepared.Candidate()
	if !ok {
		return failedResult(
			"onboarding_work",
			"candidate_missing",
			"durable profile-declaration Work omitted its typed candidate",
		)
	}
	request, err := profileadmission.NewProfileDeclarationAdmissionRequest(candidate)
	if err != nil {
		return rejectedResult("invalid_admission_request", err.Error())
	}
	admission := service.state.admission.Admit(ctx, request)
	return service.projectAdmission(ctx, admission)
}

func (service Service) ChangeProfileEntityRelation(
	ctx context.Context,
	projectRoot string,
	input ProfileOnboardingWorkInput,
	policy ProfileDeclarationPolicy,
) Result {
	root, failure := service.validateProfileDeclaration(
		ctx,
		projectRoot,
		input,
		policy,
	)
	if failure.Kind() != "" {
		return failure
	}
	if policy.Mode() != ProfileDeclarationModeHostRoutedOperatorRequest {
		return rejectedResult(
			"profile_change_requires_operator_request",
			"profile relation change requires an exact host-routed operator request",
		)
	}
	if !policyCarriesEffect(policy, operatorrequest.ProfileChange) {
		return rejectedResult(
			"profile_change_authority_mismatch",
			"profile relation change requires the separate profile.change operator effect",
		)
	}
	basis, basisPresent := input.ProfileChangeBasis()
	if !basisPresent {
		return rejectedResult(
			"profile_change_review_required",
			"profile relation change requires its own predecessor-pinned review carrier",
		)
	}
	current := service.state.admission.ResolveCurrent(ctx, root)
	if !admissionResultPresent(current) {
		if admissionResultAbsent(current) {
			return rejectedResult(
				"profile_not_declared",
				"profile relation change requires an existing canonical profile",
			)
		}
		return service.projectAdmission(ctx, current)
	}
	admission, _ := current.Admission()
	if admission.PayloadDigest() == input.PayloadDigest() {
		return service.projectAdmission(ctx, current)
	}
	if err := validateProfileChangeBasis(admission, basis); err != nil {
		return rejectedResult("profile_change_basis_stale", err.Error())
	}
	if err := input.ValidateProfileEntityRelationChange(
		admission.Payload(),
	); err != nil {
		return rejectedResult("profile_change_delta_invalid", err.Error())
	}
	if err := service.state.revalidate(ctx); err != nil {
		return failedResult(
			"ledger_revalidation",
			"pre_authority_revalidation_failed",
			err.Error(),
		)
	}
	preparation, err := profiledeclarationpreparationsqlite.PrepareBeforeAdmission(
		ctx,
		service.state.database,
		root.String(),
		input,
		policy,
		service.state.now,
		profiledeclarationpreparationsqlite.Revalidator(service.state.revalidate),
	)
	if err != nil {
		return failedResult(
			"pre_admission_preparation",
			"profile_change_preparation_failed",
			err.Error(),
		)
	}
	if preparation.Kind() == profiledeclarationpreparationsqlite.OutcomeConflict {
		detail, ok := preparation.ConflictDetail()
		if !ok {
			detail = "profile relation change preparation found an unnamed durable conflict"
		}
		return rejectedResult("profile_change_preparation_conflict", detail)
	}
	prepared, ok := preparation.Prepared()
	if !ok {
		return failedResult(
			"pre_admission_preparation",
			"prepared_result_missing",
			"profile relation change preparation omitted its sealed result",
		)
	}
	if err := service.state.revalidate(ctx); err != nil {
		return failedResult(
			"ledger_revalidation",
			"post_work_revalidation_failed",
			err.Error(),
		)
	}
	candidate, ok := prepared.Candidate()
	if !ok {
		return failedResult(
			"onboarding_work",
			"candidate_missing",
			"durable profile-change Work omitted its typed candidate",
		)
	}
	request, err := profileadmission.NewProfileChangeAdmissionRequest(
		candidate,
		basis.LedgerRevision(),
	)
	if err != nil {
		return rejectedResult("invalid_profile_change_request", err.Error())
	}
	result := service.state.admission.Admit(ctx, request)
	return service.projectAdmission(ctx, result)
}

func validateProfileChangeBasis(
	admission profileadmissionsqlite.CanonicalProfileAdmission,
	basis ProfileChangeBasis,
) error {
	checks := []struct {
		matches bool
		name    string
	}{
		{
			matches: admission.AdmissionRecordRef() == basis.AdmissionRecordRef(),
			name:    "admission_record_ref",
		},
		{
			matches: admission.AdmissionRecordDigest() == basis.AdmissionRecordDigest(),
			name:    "admission_record_digest",
		},
		{
			matches: admission.PayloadDigest() == basis.PayloadDigest(),
			name:    "payload_digest",
		},
		{
			matches: admission.LedgerRevision() == basis.LedgerRevision(),
			name:    "ledger_revision",
		},
	}
	for _, check := range checks {
		if !check.matches {
			return fmt.Errorf(
				"profile relation change %s no longer matches the current canonical profile",
				check.name,
			)
		}
	}
	return nil
}

func explicitPolicyMayOverrideDetectorDefault(
	policy ProfileDeclarationPolicy,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
) bool {
	return policy.Mode() == ProfileDeclarationModeHostRoutedOperatorRequest &&
		admission.Origin() == projectprofile.ProfileAdmissionOriginDetectorDefault
}

func policyCarriesEffect(
	policy ProfileDeclarationPolicy,
	effect operatorrequest.Effect,
) bool {
	request, ok := policy.OperatorRequest()
	return ok && request.Effect() == effect
}

func (service Service) validateProfileDeclaration(
	ctx context.Context,
	projectRoot string,
	input ProfileOnboardingWorkInput,
	policy ProfileDeclarationPolicy,
) (projectprofile.ProjectRootV1, Result) {
	if ctx == nil {
		return projectprofile.ProjectRootV1{}, failedResult(
			"orchestration",
			"context_required",
			"profile declaration requires a context",
		)
	}
	if service.state == nil {
		return projectprofile.ProjectRootV1{}, failedResult(
			"orchestration",
			"service_not_open",
			"profile declaration service is not open",
		)
	}
	if !input.Valid() {
		return projectprofile.ProjectRootV1{}, failedResult(
			"orchestration",
			"work_input_required",
			"profile declaration requires an exact reviewed Work input",
		)
	}
	if policy.Mode() != ProfileDeclarationModeHostRoutedOperatorRequest &&
		policy.Mode() != ProfileDeclarationModeAutomaticSupportedSingleton {
		return projectprofile.ProjectRootV1{}, failedResult(
			"orchestration",
			"authority_policy_required",
			"profile declaration requires an effective project authority policy",
		)
	}
	root, err := canonicalProfileProjectRoot(projectRoot)
	if err != nil {
		return projectprofile.ProjectRootV1{}, failedResult(
			"orchestration",
			"invalid_project_root",
			err.Error(),
		)
	}
	if root != input.ProjectRoot() {
		return projectprofile.ProjectRootV1{}, failedResult(
			"orchestration",
			"work_input_project_mismatch",
			"reviewed profile input belongs to another physical project root",
		)
	}
	return root, Result{}
}

func (service Service) projectExistingAdmissionForPayload(
	ctx context.Context,
	result profileadmissionsqlite.AdmissionResult,
	expected projectprofile.ContentDigest,
) Result {
	admission, ok := result.Admission()
	if !ok {
		return service.projectAdmission(ctx, result)
	}
	if admission.PayloadDigest() != expected {
		return rejectedResult(
			"profile_change_requires_separate_action",
			"the current profile has another payload; changing it requires a separate explicit action",
		)
	}
	return service.projectAdmission(ctx, result)
}
