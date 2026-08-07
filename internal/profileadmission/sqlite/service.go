package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/m0n0x41d/haft/internal/profileadmission"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

// Service is the single public effect-facing profile-admission boundary. It
// owns a concrete SQLite adapter; callers cannot substitute a port that mints
// canonical admissions from non-durable values.
type Service struct {
	adapter adapter
}

func NewService(database *sql.DB) (Service, error) {
	adapter, err := newAdapter(database)
	if err != nil {
		return Service{}, err
	}
	return Service{adapter: adapter}, nil
}

// Admit performs the canonical effect and then resolves the committed record
// again through the request-free durable resolver before minting a token.
func (service Service) Admit(
	ctx context.Context,
	request profileadmission.ProfileDeclarationAdmissionRequest,
) AdmissionResult {
	if ctx == nil {
		return admissionDenied([]AdmissionDenial{{
			code:   "invalid_request",
			detail: "profile-admission context is required",
		}})
	}
	if service.adapter.database == nil {
		return admissionFailed(
			AdmissionCommitOutcomeUnknown,
			failureStageServiceContract,
		)
	}
	outcome := service.adapter.Admit(ctx, request)
	if outcome.kind == AdmissionResultAdmitted {
		return service.resolveAdmittedOutcome(ctx, outcome)
	}
	if outcome.kind == AdmissionResultNotAdmitted && len(outcome.denials) > 0 {
		return admissionDenied(outcome.denials)
	}
	if outcome.kind == AdmissionResultWriteFailed && outcome.failure.valid() {
		return AdmissionResult{
			kind:    AdmissionResultWriteFailed,
			failure: outcome.failure,
		}
	}
	return admissionFailed(
		AdmissionCommitOutcomeUnknown,
		failureStageAdapterResultContract,
	)
}

// ResolveCurrent reconstructs the current canonical admission solely from the
// durable project root. It does not require the original one-pass authority
// request and does not require permission or evidence windows to remain open.
func (service Service) ResolveCurrent(
	ctx context.Context,
	projectRoot projectprofile.ProjectRootV1,
) AdmissionResult {
	if ctx == nil {
		return admissionDenied([]AdmissionDenial{{
			code:   "invalid_request",
			detail: "profile-admission context is required",
		}})
	}
	if service.adapter.database == nil {
		return admissionFailed(
			AdmissionCommitOutcomeUnknown,
			failureStageServiceContract,
		)
	}
	material, err := service.adapter.resolveCurrentCanonical(ctx, projectRoot)
	if err != nil {
		if isNoCurrentAdmission(err) {
			return admissionDenied([]AdmissionDenial{{
				code:   "profile_not_declared",
				detail: "no canonical profile admission exists for the project root",
			}})
		}
		return admissionFailed(
			AdmissionCommitOutcomeUnknown,
			failureStageRestartReread,
		)
	}
	admission, err := newCanonicalProfileAdmission(
		material,
		CanonicalAdmissionResolvedAfterRestart,
	)
	if err != nil {
		return admissionFailed(
			AdmissionCommitOutcomeUnknown,
			failureStageRestartTokenContract,
		)
	}
	return admissionSucceeded(admission)
}

// ResolveCommittedForAuthorityBasis recovers an already-consumed v2 profile
// authority use without replaying onboarding Work or asking an expired
// permission to become current again.
func (service Service) ResolveCommittedForAuthorityBasis(
	ctx context.Context,
	projectRoot projectprofile.ProjectRootV1,
	basisRef projectprofile.ProfileDeclarationAuthorityBasisRef,
) AdmissionResult {
	if ctx == nil {
		return admissionDenied([]AdmissionDenial{{
			code:   "invalid_request",
			detail: "profile-admission context is required",
		}})
	}
	if service.adapter.database == nil {
		return admissionFailed(
			AdmissionCommitOutcomeUnknown,
			failureStageServiceContract,
		)
	}
	material, err := service.adapter.resolveCommittedForAuthorityBasis(
		ctx,
		projectRoot,
		basisRef,
	)
	if errors.Is(err, errNoCommittedAuthorityBasis) {
		return admissionDenied([]AdmissionDenial{{
			code:   "authority_not_consumed",
			detail: "no committed profile admission exists for the authority basis",
		}})
	}
	if err != nil {
		return admissionFailed(
			AdmissionCommitOutcomeUnknown,
			failureStageRestartReread,
		)
	}
	admission, err := newCanonicalProfileAdmission(
		material,
		CanonicalAdmissionResolvedAfterRestart,
	)
	if err != nil {
		return admissionFailed(
			AdmissionCommitOutcomeUnknown,
			failureStageRestartTokenContract,
		)
	}
	return admissionSucceeded(admission)
}

// ResolveCommittedForPayload recovers one exact v2 committed admission for a
// semantic target known before project-attempt selection. A matching current
// admission wins explicitly; multiple historical non-current matches are
// reported as ambiguity rather than resolved by row order.
func (service Service) ResolveCommittedForPayload(
	ctx context.Context,
	projectRoot projectprofile.ProjectRootV1,
	payloadDigest projectprofile.ContentDigest,
) AdmissionResult {
	if ctx == nil {
		return admissionDenied([]AdmissionDenial{{
			code:   "invalid_request",
			detail: "profile-admission context is required",
		}})
	}
	if service.adapter.database == nil {
		return admissionFailed(
			AdmissionCommitOutcomeUnknown,
			failureStageServiceContract,
		)
	}
	material, err := service.adapter.resolveCommittedForPayload(
		ctx,
		projectRoot,
		payloadDigest,
	)
	if errors.Is(err, errNoCommittedAuthorityBasis) {
		return admissionDenied([]AdmissionDenial{{
			code:   "committed_profile_not_found",
			detail: "no committed profile admission matches the project and payload",
		}})
	}
	if errors.Is(err, errAmbiguousCommittedPayload) {
		return admissionDenied([]AdmissionDenial{{
			code:   "committed_profile_ambiguous",
			detail: "multiple historical committed admissions match the project and payload",
		}})
	}
	if err != nil {
		return admissionFailed(
			AdmissionCommitOutcomeUnknown,
			failureStageRestartReread,
		)
	}
	admission, err := newCanonicalProfileAdmission(
		material,
		CanonicalAdmissionResolvedAfterRestart,
	)
	if err != nil {
		return admissionFailed(
			AdmissionCommitOutcomeUnknown,
			failureStageRestartTokenContract,
		)
	}
	return admissionSucceeded(admission)
}

func (service Service) resolveAdmittedOutcome(
	ctx context.Context,
	outcome adapterOutcome,
) AdmissionResult {
	projectRoot := outcome.admission.projectRoot
	admissionRef := outcome.admission.admissionRef
	admissionDigest := outcome.admission.admissionDigest
	material, err := service.adapter.resolveCanonicalByReference(
		ctx,
		projectRoot,
		admissionRef,
		admissionDigest,
	)
	if err != nil {
		return admissionFailed(
			AdmissionCommitOutcomeUnknown,
			failureStageTokenReread,
		)
	}
	admission, err := newCanonicalProfileAdmission(material, outcome.delivery)
	if err != nil {
		return admissionFailed(
			AdmissionCommitOutcomeUnknown,
			failureStageTokenContract,
		)
	}
	return admissionSucceeded(admission)
}
