package profileonboarding

import (
	"context"
	"database/sql"

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

func explicitPolicyMayOverrideDetectorDefault(
	policy ProfileDeclarationPolicy,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
) bool {
	return policy.Mode() == ProfileDeclarationModeHostRoutedOperatorRequest &&
		admission.Origin() == projectprofile.ProfileAdmissionOriginDetectorDefault
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
