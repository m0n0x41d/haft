package profileonboarding

import (
	"context"
	"database/sql"

	"github.com/m0n0x41d/haft/internal/profileadmission"
	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	profiledeclarationpreparationsqlite "github.com/m0n0x41d/haft/internal/profiledeclarationpreparation/sqlite"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

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
	if admissionResultPresent(current) {
		return service.projectExistingAdmissionForPayload(
			ctx,
			current,
			input.PayloadDigest(),
		)
	}
	if !admissionResultAbsent(current) {
		return service.projectAdmission(ctx, current)
	}
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
	if policy.Mode() == ProfileDeclarationModeStrictSpeechAct {
		return failedResult(
			"authority",
			"strict_profile_authority_not_available",
			"strict_cli_speech_act requires a native v3 profile authority source; this build will not write sealed v1/v2 authority rows; use explicit_h_onboard or upgrade when native strict support is available",
		)
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
	if policy.Mode() != ProfileDeclarationModeExplicitHOnboard &&
		policy.Mode() != ProfileDeclarationModeStrictSpeechAct {
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
