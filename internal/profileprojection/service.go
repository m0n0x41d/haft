package profileprojection

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const projectionRelativePath = ".haft/project-profile.yaml"

const rebuildAttemptLimit = 3

var projectLocks sync.Map

var errLedgerHeadChanged = errors.New("canonical profile ledger head changed")

type clock func() time.Time
type identifierSource func(string) (string, error)

// Service composes the canonical admission resolver, the projection effect,
// and the append-only projection-debt ledger. It owns no profile authority.
type Service struct {
	database         *sql.DB
	admissionService profileadmissionsqlite.Service
	now              clock
	newIdentifier    identifierSource
}

func NewService(database *sql.DB) (Service, error) {
	if database == nil {
		return Service{}, fmt.Errorf("profile-projection database is required")
	}
	admissionService, err := profileadmissionsqlite.NewService(database)
	if err != nil {
		return Service{}, err
	}
	return Service{
		database:         database,
		admissionService: admissionService,
		now:              time.Now,
		newIdentifier:    randomIdentifier,
	}, nil
}

// ProjectionPath returns the one final-v1 human-readable carrier path.
func ProjectionPath(projectRoot projectprofile.ProjectRootV1) (string, error) {
	validated, err := projectprofile.NewProjectRootV1(projectRoot.String())
	if err != nil || validated != projectRoot {
		return "", fmt.Errorf("canonical project root is required")
	}
	return filepath.Join(projectRoot.String(), filepath.FromSlash(projectionRelativePath)), nil
}

// Project synchronizes the projection for one sealed canonical admission.
// A stale token cannot overwrite the projection of a newer ledger revision.
func (service Service) Project(
	ctx context.Context,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("profile-projection context is required")
	}
	if err := service.validate(); err != nil {
		return Result{}, err
	}
	if !admission.Valid() {
		return Result{}, fmt.Errorf("sealed canonical profile admission is required")
	}
	unlock := lockProject(admission.ProjectRoot().String())
	defer unlock()
	current, err := service.resolveCurrent(ctx, admission.ProjectRoot())
	if err != nil {
		return Result{}, err
	}
	if !sameAdmission(current, admission) {
		return Result{}, fmt.Errorf("profile projection rejected a stale canonical admission")
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, service.database)
	if err != nil {
		return Result{}, err
	}
	source, err := requireExactLedgerHead(transaction, ctx, current)
	if err != nil {
		finish := transaction.Rollback(ctx)
		return Result{}, fmt.Errorf("profile ledger changed before projection: %w", joinErrors(err, finish.Err()))
	}
	return service.projectCurrent(ctx, transaction, current, source)
}

// Rebuild resolves profile state from SQLite without a caller-provided token.
// It is also the crash-reconciliation boundary: missing or drifted projection
// bytes are rediscovered from the canonical ledger even if a process stopped
// before it could commit a projection-debt event. No ledger and no projection
// is the backward-compatible Auto state.
func (service Service) Rebuild(
	ctx context.Context,
	projectRoot projectprofile.ProjectRootV1,
) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("profile-projection context is required")
	}
	if err := service.validate(); err != nil {
		return Result{}, err
	}
	path, err := ProjectionPath(projectRoot)
	if err != nil {
		return Result{}, err
	}
	unlock := lockProject(projectRoot.String())
	defer unlock()
	var lastRetryErr error
	for attempt := 0; attempt < rebuildAttemptLimit; attempt++ {
		result, retry, rebuildErr := service.rebuildAttempt(ctx, projectRoot, path)
		if !retry {
			return result, rebuildErr
		}
		lastRetryErr = rebuildErr
	}
	return Result{}, fmt.Errorf(
		"profile ledger changed during %d projection rebuild attempts: %w",
		rebuildAttemptLimit,
		lastRetryErr,
	)
}

func (service Service) rebuildAttempt(
	ctx context.Context,
	projectRoot projectprofile.ProjectRootV1,
	path string,
) (Result, bool, error) {
	resolution := service.admissionService.ResolveCurrent(ctx, projectRoot)
	admission, ok := resolution.Admission()
	if ok {
		transaction, err := sqlitetransaction.BeginImmediate(ctx, service.database)
		if err != nil {
			return Result{}, false, err
		}
		source, err := requireExactLedgerHead(transaction, ctx, admission)
		if err != nil {
			finish := transaction.Rollback(ctx)
			combined := joinErrors(err, finish.Err())
			if errors.Is(err, errLedgerHeadChanged) && finish.Succeeded() {
				return Result{}, true, combined
			}
			return Result{}, false, combined
		}
		result, projectErr := service.projectCurrent(
			ctx,
			transaction,
			admission,
			source,
		)
		return result, false, projectErr
	}
	if !profileAbsent(resolution) {
		return Result{}, false, resolutionError(resolution)
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, service.database)
	if err != nil {
		return Result{}, false, err
	}
	exists, err := canonicalLedgerHeadExists(transaction, ctx, projectRoot)
	if err != nil {
		finish := transaction.Rollback(ctx)
		return Result{}, false, joinErrors(err, finish.Err())
	}
	if exists {
		finish := transaction.Rollback(ctx)
		if !finish.Succeeded() {
			return Result{}, false, finish.Err()
		}
		return Result{}, true, errLedgerHeadChanged
	}
	result, resultErr := resultWithoutCanonicalLedger(path)
	finish := transaction.Rollback(ctx)
	return result, false, joinErrors(resultErr, finish.Err())
}

func (service Service) projectCurrent(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
	source exactAdmissionSource,
) (Result, error) {
	material, err := projectionFromAdmission(admission)
	if err != nil {
		finish := transaction.Rollback(ctx)
		return Result{}, joinErrors(err, finish.Err())
	}
	expected, err := buildProjection(material)
	if err != nil {
		finish := transaction.Rollback(ctx)
		return Result{}, joinErrors(err, finish.Err())
	}
	path, err := ProjectionPath(material.projectRoot)
	if err != nil {
		finish := transaction.Rollback(ctx)
		return Result{}, joinErrors(err, finish.Err())
	}
	directory, err := openProjectionDirectory(material.projectRoot.String())
	if err != nil {
		observation := unreadableProjectionObservation(err)
		debt, debtErr := service.openDebt(
			transaction,
			ctx,
			admission,
			source,
			expected,
			path,
			observation,
		)
		if debtErr != nil {
			finish := transaction.Rollback(ctx)
			return Result{}, joinErrors(debtErr, finish.Err())
		}
		return service.commitOutstandingDebt(
			ctx,
			transaction,
			path,
			expected,
			observation,
			debt,
			"projection_directory_unavailable",
			err,
		)
	}
	defer directory.Close()
	err = directory.reconcileStages()
	if err != nil {
		observation := unreadableProjectionObservation(err)
		debt, debtErr := service.openDebt(
			transaction,
			ctx,
			admission,
			source,
			expected,
			path,
			observation,
		)
		if debtErr != nil {
			finish := transaction.Rollback(ctx)
			return Result{}, joinErrors(debtErr, finish.Err())
		}
		return service.commitOutstandingDebt(
			ctx,
			transaction,
			path,
			expected,
			observation,
			debt,
			"projection_stage_reconciliation_failed",
			err,
		)
	}
	observation := directory.observe(expected.content)
	if observation.kind == observationMatched {
		return service.finishVerifiedProjection(
			ctx,
			transaction,
			admission,
			source,
			expected,
			path,
			observation,
		)
	}
	debt, err := service.openDebt(
		transaction,
		ctx,
		admission,
		source,
		expected,
		path,
		observation,
	)
	if err != nil {
		finish := transaction.Rollback(ctx)
		return Result{}, fmt.Errorf("record profile projection debt before repair: %w", joinErrors(err, finish.Err()))
	}
	temporaryID, err := service.newIdentifier("project-profile-projection-stage")
	if err != nil {
		return service.commitOutstandingDebt(
			ctx,
			transaction,
			path,
			expected,
			observation,
			debt,
			"projection_stage_id_failed",
			err,
		)
	}
	err = directory.writeAtomic(expected.content, temporaryID)
	if err != nil {
		return service.commitOutstandingDebt(
			ctx,
			transaction,
			path,
			expected,
			observation,
			debt,
			"projection_write_failed",
			fmt.Errorf("write profile projection: %w", err),
		)
	}
	verified := directory.observe(expected.content)
	if verified.kind != observationMatched {
		return service.commitOutstandingDebt(
			ctx,
			transaction,
			path,
			expected,
			verified,
			debt,
			"projection_verify_failed",
			fmt.Errorf("verify profile projection: %s", verified.detail),
		)
	}
	return service.finishVerifiedProjection(
		ctx,
		transaction,
		admission,
		source,
		expected,
		path,
		verified,
	)
}

func (service Service) finishVerifiedProjection(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
	source exactAdmissionSource,
	expected projection,
	path string,
	observation projectionObservation,
) (Result, error) {
	debt, found, err := scanExactOpenDebt(
		transaction,
		ctx,
		admission,
		source,
		path,
		expected.digest,
	)
	if err != nil {
		finish := transaction.Rollback(ctx)
		result := Result{
			kind:           ResultProjectionDebt,
			projectionPath: path,
			expectedDigest: expected.digest,
			observedDigest: observation.digest,
			diagnosticCode: "projection_debt_resolution_failed",
			detail:         "projection bytes are current but durable projection debt could not be resolved",
		}
		return result, joinErrors(err, finish.Err())
	}
	if found {
		err = service.resolveExactDebt(
			transaction,
			ctx,
			source,
			debt,
			observation.digest,
		)
		if err != nil {
			finish := transaction.Rollback(ctx)
			result := outstandingDebtResult(
				path,
				expected,
				observation,
				debt,
				"projection_debt_resolution_failed",
			)
			return result, joinErrors(err, finish.Err())
		}
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		return Result{
			kind:           ResultProjectionDebt,
			projectionPath: path,
			expectedDigest: expected.digest,
			observedDigest: observation.digest,
			diagnosticCode: "projection_debt_commit_unknown",
			detail:         "projection bytes are current but debt-event commit was not proved",
		}, finish.Err()
	}
	return Result{
		kind:           ResultSynchronized,
		projectionPath: path,
		expectedDigest: expected.digest,
		observedDigest: observation.digest,
		diagnosticCode: "projection_synchronized",
		detail:         "human-readable projection matches the canonical profile ledger revision",
	}, nil
}

func (service Service) commitOutstandingDebt(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	path string,
	expected projection,
	observation projectionObservation,
	debt debtRecord,
	diagnosticCode string,
	cause error,
) (Result, error) {
	finish := transaction.Commit(ctx)
	result := outstandingDebtResult(path, expected, observation, debt, diagnosticCode)
	if !finish.Succeeded() {
		return result, fmt.Errorf(
			"profile projection failed and durable debt commit was not proved: %w",
			joinErrors(cause, finish.Err()),
		)
	}
	return result, fmt.Errorf(
		"profile projection failed; retryable debt %s was recorded: %w",
		debt.debtID,
		cause,
	)
}

func (service Service) resolveCurrent(
	ctx context.Context,
	projectRoot projectprofile.ProjectRootV1,
) (profileadmissionsqlite.CanonicalProfileAdmission, error) {
	result := service.admissionService.ResolveCurrent(ctx, projectRoot)
	admission, ok := result.Admission()
	if ok {
		return admission, nil
	}
	return profileadmissionsqlite.CanonicalProfileAdmission{}, resolutionError(result)
}

func (service Service) validate() error {
	if service.database == nil {
		return fmt.Errorf("profile-projection service has no database")
	}
	if service.now == nil {
		return fmt.Errorf("profile-projection service has no clock")
	}
	if service.newIdentifier == nil {
		return fmt.Errorf("profile-projection service has no identifier source")
	}
	return nil
}

func sameAdmission(
	left profileadmissionsqlite.CanonicalProfileAdmission,
	right profileadmissionsqlite.CanonicalProfileAdmission,
) bool {
	return left.Valid() &&
		right.Valid() &&
		left.ProjectRoot() == right.ProjectRoot() &&
		left.LedgerRevision() == right.LedgerRevision() &&
		left.AdmissionRecordRef() == right.AdmissionRecordRef() &&
		left.AdmissionRecordDigest() == right.AdmissionRecordDigest() &&
		left.PayloadDigest() == right.PayloadDigest()
}

func profileAbsent(result profileadmissionsqlite.AdmissionResult) bool {
	denials, ok := result.Denials()
	if !ok {
		return false
	}
	for _, denial := range denials {
		if denial.Code() == "profile_not_declared" {
			return true
		}
	}
	return false
}

func resultWithoutCanonicalLedger(path string) (Result, error) {
	exists, detail, err := projectionExists(path)
	if err != nil {
		return Result{}, fmt.Errorf("inspect profile projection without canonical ledger: %w", err)
	}
	if exists {
		return Result{
			kind:           ResultProjectionWithoutLedger,
			projectionPath: path,
			diagnosticCode: "projection_without_ledger",
			detail:         detail,
		}, nil
	}
	return Result{
		kind:           ResultAuto,
		projectionPath: path,
		diagnosticCode: "auto_no_profile_state",
		detail:         "no canonical profile revision or YAML projection exists; use Auto",
	}, nil
}

func resolutionError(result profileadmissionsqlite.AdmissionResult) error {
	denials, ok := result.Denials()
	if ok {
		details := ""
		for _, denial := range denials {
			details = details + denial.Code() + ": " + denial.Detail() + "; "
		}
		return fmt.Errorf("canonical profile resolution was denied: %s", details)
	}
	failure, ok := result.Failure()
	if ok {
		return fmt.Errorf(
			"canonical profile resolution failed at %s with posture %s",
			failure.FailureRef(),
			failure.CommitPosture(),
		)
	}
	return fmt.Errorf("canonical profile resolver returned invalid result kind %q", result.Kind())
}

func outstandingDebtResult(
	path string,
	expected projection,
	observation projectionObservation,
	debt debtRecord,
	diagnosticCode string,
) Result {
	return Result{
		kind:           ResultProjectionDebt,
		projectionPath: path,
		expectedDigest: expected.digest,
		observedDigest: observation.digest,
		debtID:         debt.debtID,
		diagnosticCode: diagnosticCode,
		detail:         observation.detail,
	}
}

func randomIdentifier(prefix string) (string, error) {
	var value [16]byte
	_, err := rand.Read(value[:])
	if err != nil {
		return "", err
	}
	encoded := hex.EncodeToString(value[:])
	return prefix + "-" + encoded, nil
}

func lockProject(projectRoot string) func() {
	value, _ := projectLocks.LoadOrStore(projectRoot, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock
}

func joinErrors(left error, right error) error {
	return errors.Join(left, right)
}
