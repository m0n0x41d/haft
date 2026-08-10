package projectledgermigration

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/projectledger"
)

// ServeActivationOutcome preserves the exact startup result. Only the first
// three outcomes make project-backed MCP surfaces available in this process.
type ServeActivationOutcome string

const (
	ServeActivationCurrent          ServeActivationOutcome = "current"
	ServeActivationMigrated         ServeActivationOutcome = "migrated"
	ServeActivationCurrentAfterWait ServeActivationOutcome = "current_after_wait"
	ServeActivationManualRequired   ServeActivationOutcome = "manual_required"
	ServeActivationRetryRequired    ServeActivationOutcome = "retry_required"
	ServeActivationBlocked          ServeActivationOutcome = "blocked"
)

type ServeActivationBlocker string

const (
	ServeBlockerNone             ServeActivationBlocker = ""
	ServeBlockerManualChain      ServeActivationBlocker = "manual_chain"
	ServeBlockerMissingBinding   ServeActivationBlocker = "missing_binding"
	ServeBlockerFutureSchema     ServeActivationBlocker = "future_schema"
	ServeBlockerInvalidSchema    ServeActivationBlocker = "invalid_schema"
	ServeBlockerLeaseTimeout     ServeActivationBlocker = "lease_timeout"
	ServeBlockerLeaseUnavailable ServeActivationBlocker = "lease_unavailable"
	ServeBlockerStaleSidecar     ServeActivationBlocker = "stale_sidecar"
	ServeBlockerSnapshot         ServeActivationBlocker = "snapshot"
	ServeBlockerMigration        ServeActivationBlocker = "migration"
)

type ServeActivationResult struct {
	Outcome             ServeActivationOutcome
	Blocker             ServeActivationBlocker
	ProjectRoot         string
	ProjectID           string
	DatabasePath        string
	BeforeSchema        int
	AfterSchema         int
	PendingVersions     []int
	FirstBlockedVersion int
	WaitDuration        time.Duration
	BackupPath          string
	BackupDigest        string
}

func (result ServeActivationResult) Ready() bool {
	return result.Outcome == ServeActivationCurrent ||
		result.Outcome == ServeActivationMigrated ||
		result.Outcome == ServeActivationCurrentAfterWait
}

// EnsureCurrentForServe activates only the suffix explicitly admitted by the
// compiled migration catalog. Policy blocks are returned as closed results
// plus an error so a caller cannot accidentally continue into DB-backed
// surfaces while still retaining an exact recovery classification.
func EnsureCurrentForServe(
	ctx context.Context,
	request Request,
	at time.Time,
) (ServeActivationResult, error) {
	base := newServeActivationBase(request)
	if ctx == nil {
		base.Outcome = ServeActivationBlocked
		base.Blocker = ServeBlockerInvalidSchema
		return base, fmt.Errorf("activate serve project ledger: context is required")
	}
	observation, err := Observe(ctx, request)
	if err != nil {
		return blockedServeObservation(base, err)
	}
	base = serveActivationFromObservation(base, observation)
	plan, err := db.CompileServeMigrationPlan(observation.ObservedSchema)
	if err != nil {
		base.Outcome = ServeActivationBlocked
		base.Blocker = ServeBlockerInvalidSchema
		return base, err
	}
	base = serveActivationFromPlan(base, plan)
	if plan.Kind != db.ServeMigrationAutomatic {
		return finishServeActivationPlan(base, plan)
	}
	if plan.SnapshotRequired && at.IsZero() {
		base.Outcome = ServeActivationBlocked
		base.Blocker = ServeBlockerSnapshot
		return base, fmt.Errorf(
			"activate serve project ledger: snapshot timestamp is required",
		)
	}
	coordinator, err := newMigrationCoordinator(observation)
	if err != nil {
		base.Outcome = ServeActivationBlocked
		base.Blocker = ServeBlockerLeaseUnavailable
		return base, err
	}
	waitContext, cancel := boundedMigrationWaitContext(ctx)
	defer cancel()
	lease, waitDuration, err := coordinator.acquire(waitContext)
	base.WaitDuration = waitDuration
	if err != nil {
		base.Outcome = ServeActivationBlocked
		base.Blocker = ServeBlockerLeaseUnavailable
		if errors.Is(err, ErrMigrationLeaseTimeout) {
			base.Outcome = ServeActivationRetryRequired
			base.Blocker = ServeBlockerLeaseTimeout
		}
		return base, err
	}
	result, activationErr := activateServeUnderLease(
		ctx,
		request,
		at,
		base,
	)
	releaseErr := lease.release()
	if err := errors.Join(activationErr, releaseErr); err != nil {
		if result.Blocker == ServeBlockerNone {
			result.Outcome = ServeActivationBlocked
			result.Blocker = ServeBlockerLeaseUnavailable
		}
		return result, err
	}
	return result, nil
}

func activateServeUnderLease(
	ctx context.Context,
	request Request,
	at time.Time,
	base ServeActivationResult,
) (ServeActivationResult, error) {
	observation, err := Observe(ctx, request)
	if err != nil {
		return blockedServeObservation(base, err)
	}
	base = serveActivationFromObservation(base, observation)
	plan, err := db.CompileServeMigrationPlan(observation.ObservedSchema)
	if err != nil {
		base.Outcome = ServeActivationBlocked
		base.Blocker = ServeBlockerInvalidSchema
		return base, err
	}
	base = serveActivationFromPlan(base, plan)
	if plan.Kind != db.ServeMigrationAutomatic {
		result, planErr := finishServeActivationPlan(base, plan)
		if result.Outcome == ServeActivationCurrent {
			result.Outcome = ServeActivationCurrentAfterWait
		}
		return result, planErr
	}
	handle, err := projectledger.OpenForExplicitMigration(
		ctx,
		request.root.String(),
		projectledger.ReadWrite,
	)
	if err != nil {
		base.Outcome = ServeActivationBlocked
		base.Blocker = classifyServeObservationBlocker(err)
		return base, fmt.Errorf("open project ledger for serve activation: %w", err)
	}
	result, activationErr := migrateServeHandle(
		ctx,
		request,
		handle,
		observation,
		plan,
		at,
		base,
	)
	closeErr := handle.Close()
	if err := errors.Join(activationErr, closeErr); err != nil {
		return result, err
	}
	return result, nil
}

func migrateServeHandle(
	ctx context.Context,
	request Request,
	handle *projectledger.Handle,
	observation SchemaObservation,
	plan db.ServeMigrationPlan,
	at time.Time,
	result ServeActivationResult,
) (ServeActivationResult, error) {
	if handle.ProjectID() != request.project {
		result.Outcome = ServeActivationBlocked
		result.Blocker = ServeBlockerInvalidSchema
		return result, fmt.Errorf(
			"serve activation opened project %s, expected %s",
			handle.ProjectID().String(),
			request.project.String(),
		)
	}
	migrated, err := applyToHandleAt(
		ctx,
		request,
		handle,
		transitionExpectation{
			kind:   transitionExactAdditive,
			before: observation.ObservedSchema,
			after:  observation.CompiledSchema,
		},
		at,
	)
	result.BackupPath = migrated.BackupPath
	result.BackupDigest = migrated.BackupDigest
	if err != nil {
		result.Outcome = ServeActivationBlocked
		result.Blocker = ServeBlockerSnapshot
		if result.BackupPath != "" {
			result.Blocker = ServeBlockerMigration
		}
		return result, err
	}
	if plan.SnapshotRequired && result.BackupPath == "" {
		result.Outcome = ServeActivationBlocked
		result.Blocker = ServeBlockerSnapshot
		return result, fmt.Errorf(
			"migration plan required a verified snapshot but none was published",
		)
	}
	result.Outcome = ServeActivationMigrated
	result.Blocker = ServeBlockerNone
	result.BeforeSchema = migrated.BeforeSchema
	result.AfterSchema = migrated.AfterSchema
	return result, nil
}

func newServeActivationBase(request Request) ServeActivationResult {
	return ServeActivationResult{
		ProjectRoot: request.root.String(),
		ProjectID:   request.project.String(),
	}
}

func serveActivationFromObservation(
	result ServeActivationResult,
	observation SchemaObservation,
) ServeActivationResult {
	result.ProjectRoot = observation.ProjectRoot
	result.ProjectID = observation.ProjectID
	result.DatabasePath = observation.DatabasePath
	result.BeforeSchema = observation.ObservedSchema
	result.AfterSchema = observation.CompiledSchema
	return result
}

func serveActivationFromPlan(
	result ServeActivationResult,
	plan db.ServeMigrationPlan,
) ServeActivationResult {
	result.PendingVersions = append(
		[]int(nil),
		plan.PendingVersions...,
	)
	result.FirstBlockedVersion = plan.FirstBlockedVersion
	return result
}

func finishServeActivationPlan(
	result ServeActivationResult,
	plan db.ServeMigrationPlan,
) (ServeActivationResult, error) {
	switch plan.Kind {
	case db.ServeMigrationCurrent:
		result.Outcome = ServeActivationCurrent
		result.Blocker = ServeBlockerNone
		return result, nil
	case db.ServeMigrationManualRequired:
		result.Outcome = ServeActivationManualRequired
		result.Blocker = ServeBlockerManualChain
		return result, fmt.Errorf(
			"project schema %d requires manual migration at version %d before compiled schema %d",
			plan.ObservedSchema,
			plan.FirstBlockedVersion,
			plan.CompiledSchema,
		)
	case db.ServeMigrationFutureSchema:
		result.Outcome = ServeActivationBlocked
		result.Blocker = ServeBlockerFutureSchema
		return result, fmt.Errorf(
			"project schema %d is newer than this Haft binary schema %d",
			plan.ObservedSchema,
			plan.CompiledSchema,
		)
	case db.ServeMigrationInvalidCatalog:
		result.Outcome = ServeActivationBlocked
		result.Blocker = ServeBlockerInvalidSchema
		return result, fmt.Errorf(
			"project schema %d cannot be mapped to the compiled migration catalog ending at %d",
			plan.ObservedSchema,
			plan.CompiledSchema,
		)
	default:
		result.Outcome = ServeActivationBlocked
		result.Blocker = ServeBlockerInvalidSchema
		return result, fmt.Errorf("unknown serve migration plan %q", plan.Kind)
	}
}

func blockedServeObservation(
	result ServeActivationResult,
	cause error,
) (ServeActivationResult, error) {
	result.Outcome = ServeActivationBlocked
	result.Blocker = classifyServeObservationBlocker(cause)
	return result, cause
}

func classifyServeObservationBlocker(cause error) ServeActivationBlocker {
	if errors.Is(cause, projectledger.ErrBindingMissing) {
		return ServeBlockerMissingBinding
	}
	if errors.Is(cause, projectledger.ErrSQLiteSidecarGenerationChanged) {
		return ServeBlockerStaleSidecar
	}
	return ServeBlockerInvalidSchema
}
