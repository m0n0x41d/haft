package profileonboarding

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/profileprojection"
)

type profileLedgerRevalidator func(context.Context) error

// Service composes the source-native profile authority stages, separate
// ProfileOnboarding Work, candidate-only admission, and readable projection.
// It owns no SQL and cannot accept a caller-supplied authority address.
type Service struct {
	state *serviceState
}

type serviceState struct {
	database   *sql.DB
	admission  profileadmissionsqlite.Service
	projection profileprojection.Service
	now        func() time.Time
	revalidate profileLedgerRevalidator
}

func newService(
	database *sql.DB,
	revalidate profileLedgerRevalidator,
) (Service, error) {
	if database == nil || revalidate == nil {
		return Service{}, fmt.Errorf(
			"profile onboarding requires database and ledger revalidation",
		)
	}
	admission, err := profileadmissionsqlite.NewService(database)
	if err != nil {
		return Service{}, err
	}
	projection, err := profileprojection.NewService(database)
	if err != nil {
		return Service{}, err
	}
	return Service{state: &serviceState{
		database:   database,
		admission:  admission,
		projection: projection,
		now:        time.Now,
		revalidate: revalidate,
	}}, nil
}

func (service Service) projectAdmission(
	ctx context.Context,
	admissionResult profileadmissionsqlite.AdmissionResult,
) Result {
	admissionKind := admissionResult.Kind()
	denials, hasDenials := admissionResult.Denials()
	if admissionKind == profileadmissionsqlite.AdmissionResultNotAdmitted && !hasDenials {
		return failedResult("admission", "invalid_admission_result", "not-admitted result omitted denials")
	}
	if admissionKind == profileadmissionsqlite.AdmissionResultNotAdmitted {
		return admissionDeniedResult(denials)
	}
	failure, hasFailure := admissionResult.Failure()
	if admissionKind == profileadmissionsqlite.AdmissionResultWriteFailed && !hasFailure {
		return failedResult("admission", "invalid_admission_result", "write-failed result omitted failure evidence")
	}
	if admissionKind == profileadmissionsqlite.AdmissionResultWriteFailed {
		return admissionFailedResult(failure)
	}
	admission, ok := admissionResult.Admission()
	if admissionKind != profileadmissionsqlite.AdmissionResultAdmitted || !ok {
		return failedResult("admission", "invalid_admission_result", "admission service returned no canonical admission")
	}
	if err := service.state.revalidate(ctx); err != nil {
		return projectionFailedResult(
			admission,
			"pre_projection_revalidation_failed",
			err.Error(),
		)
	}
	projection, projectionErr := service.state.projection.Project(ctx, admission)
	revalidationErr := service.state.revalidate(ctx)
	if projection.Kind() == profileprojection.ResultProjectionDebt {
		return projectionDebtResult(
			admission,
			projection,
			errors.Join(projectionErr, revalidationErr),
		)
	}
	if projectionErr != nil || revalidationErr != nil {
		code := "projection_failed"
		if projectionErr == nil {
			code = "post_projection_revalidation_failed"
		}
		return projectionFailedResult(
			admission,
			code,
			errors.Join(projectionErr, revalidationErr).Error(),
		)
	}
	if projection.Kind() != profileprojection.ResultSynchronized {
		detail := fmt.Sprintf("unexpected projection result %q", projection.Kind())
		return projectionFailedResult(admission, "invalid_projection_result", detail)
	}
	return synchronizedResult(admission, projection)
}

func admissionResultPresent(result profileadmissionsqlite.AdmissionResult) bool {
	_, ok := result.Admission()
	return result.Kind() == profileadmissionsqlite.AdmissionResultAdmitted && ok
}

func admissionResultAbsent(result profileadmissionsqlite.AdmissionResult) bool {
	if result.Kind() != profileadmissionsqlite.AdmissionResultNotAdmitted {
		return false
	}
	denials, ok := result.Denials()
	return ok && len(denials) == 1 && denials[0].Code() == "profile_not_declared"
}

func admissionResultMissingCommitted(
	result profileadmissionsqlite.AdmissionResult,
) bool {
	if result.Kind() != profileadmissionsqlite.AdmissionResultNotAdmitted {
		return false
	}
	denials, ok := result.Denials()
	return ok && len(denials) == 1 && denials[0].Code() == "committed_profile_not_found"
}
