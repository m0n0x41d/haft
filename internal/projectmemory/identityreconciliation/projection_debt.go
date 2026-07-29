package identityreconciliation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// OpenProjectionDebt records drift against an already committed reconciliation
// projection. It appends only to the debt ledger; it cannot alter semantic
// graph events, entity identity, or the graph head.
func (service *SQLiteService) OpenProjectionDebt(
	ctx context.Context,
	project projectledger.ProjectID,
	reconciliation Receipt,
	reason ProjectionDebtReason,
	detail ProjectionDebtDetail,
	expected typedmemory.SHA256Digest,
) (ProjectionDebtReceipt, error) {
	if ctx == nil {
		return ProjectionDebtReceipt{}, fmt.Errorf("open identity projection debt: context is required")
	}
	if service == nil || service.database == nil || service.clock == nil || service.schema == nil {
		return ProjectionDebtReceipt{}, ErrDatabaseRequired
	}
	if reason.String() == "" || detail.String() == "" || expected.String() == "" ||
		reconciliation.EventRef() == "" || reconciliation.CommitRef() == "" ||
		reconciliation.GraphRevision().Value() == 0 {
		return ProjectionDebtReceipt{}, ErrProjectionBasis
	}
	graphRevision, err := projectionDebtRevisionSQLiteValue(
		reconciliation.GraphRevision(),
	)
	if err != nil {
		return ProjectionDebtReceipt{}, err
	}
	if err := service.schema.RequireCompatible(ctx); err != nil {
		return ProjectionDebtReceipt{}, fmt.Errorf("%w: %v", ErrSchemaUnavailable, err)
	}
	projectionJobRef := derivedRef("typed-memory-projection-job", reconciliation.CommitRef())
	debtRef := derivedRef(
		"typed-memory-projection-debt",
		project.String(),
		reconciliation.EventRef(),
		expected.String(),
		reason.String(),
	)
	debtEventRef := derivedRef(
		"typed-memory-projection-debt-event",
		debtRef,
		detail.String(),
	)
	transaction, err := sqlitetransaction.BeginImmediate(ctx, service.database)
	if err != nil {
		return ProjectionDebtReceipt{}, fmt.Errorf("begin identity projection-debt transaction: %w", err)
	}
	existing, found, err := loadProjectionDebt(
		ctx,
		transaction,
		project,
		debtEventRef,
		debtRef,
		projectionJobRef,
		reconciliation,
		graphRevision,
		reason,
		detail,
		expected,
	)
	if err != nil {
		return ProjectionDebtReceipt{}, rollbackWith(ctx, transaction, err)
	}
	if found {
		if err := rollbackSuccess(ctx, transaction); err != nil {
			return ProjectionDebtReceipt{}, err
		}
		return existing, nil
	}
	if err := requireProjectionBasis(
		ctx,
		transaction,
		project,
		projectionJobRef,
		reconciliation,
		graphRevision,
	); err != nil {
		return ProjectionDebtReceipt{}, rollbackWith(ctx, transaction, err)
	}
	recordedAt := service.clock.Now().UTC().Format(time.RFC3339Nano)
	_, err = transaction.Execute(
		ctx,
		`INSERT INTO typed_memory_projection_debt_events (
			project_id, debt_event_ref, debt_ref, projection_job_ref,
			semantic_event_ref, graph_revision, event_kind, reason_code,
			detail, expected_projection_digest, observed_projection_digest,
			supersedes_debt_event_ref, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, 'opened', ?, ?, ?, '', NULL, ?)`,
		[]any{
			project.String(),
			debtEventRef,
			debtRef,
			projectionJobRef,
			reconciliation.EventRef(),
			graphRevision,
			reason.String(),
			detail.String(),
			expected.String(),
			recordedAt,
		},
	)
	if err != nil {
		return ProjectionDebtReceipt{}, rollbackWith(
			ctx,
			transaction,
			fmt.Errorf("persist identity projection debt: %w", err),
		)
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		return ProjectionDebtReceipt{}, fmt.Errorf("commit identity projection debt: %w", finish.Err())
	}
	return ProjectionDebtReceipt{debtRef: debtRef, debtEventRef: debtEventRef}, nil
}

func requireProjectionBasis(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	projectionJobRef string,
	reconciliation Receipt,
	graphRevision int64,
) error {
	var count int64
	err := transaction.ScanOne(
		ctx,
		`SELECT COUNT(*)
		FROM typed_memory_identity_reconciliations identity_event
		JOIN typed_memory_identity_reconciliation_closures closure
			ON closure.project_id = identity_event.project_id
			AND closure.event_ref = identity_event.event_ref
		JOIN typed_memory_projection_jobs job
			ON job.project_id = identity_event.project_id
			AND job.semantic_event_ref = identity_event.event_ref
		WHERE identity_event.project_id = ?
			AND identity_event.event_ref = ?
			AND identity_event.commit_ref = ?
			AND job.projection_job_ref = ?
			AND job.graph_revision = ?`,
		[]any{
			project.String(),
			reconciliation.EventRef(),
			reconciliation.CommitRef(),
			projectionJobRef,
			graphRevision,
		},
		[]any{&count},
	)
	if err != nil {
		return fmt.Errorf("inspect identity projection basis: %w", err)
	}
	if count != 1 {
		return ErrProjectionBasis
	}
	return nil
}

func loadProjectionDebt(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	debtEventRef string,
	debtRef string,
	projectionJobRef string,
	reconciliation Receipt,
	graphRevision int64,
	reason ProjectionDebtReason,
	detail ProjectionDebtDetail,
	expected typedmemory.SHA256Digest,
) (ProjectionDebtReceipt, bool, error) {
	var storedDebtRef string
	var storedJobRef string
	var storedEventRef string
	var storedRevision int64
	var storedKind string
	var storedReason string
	var storedDetail string
	var storedExpected string
	var storedObserved string
	var storedSupersedes sql.NullString
	err := transaction.ScanOne(
		ctx,
		`SELECT debt_ref, projection_job_ref, semantic_event_ref,
			graph_revision, event_kind, reason_code, detail,
			expected_projection_digest, observed_projection_digest,
			supersedes_debt_event_ref
		FROM typed_memory_projection_debt_events
		WHERE project_id = ? AND debt_event_ref = ?`,
		[]any{project.String(), debtEventRef},
		[]any{
			&storedDebtRef,
			&storedJobRef,
			&storedEventRef,
			&storedRevision,
			&storedKind,
			&storedReason,
			&storedDetail,
			&storedExpected,
			&storedObserved,
			&storedSupersedes,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectionDebtReceipt{}, false, nil
	}
	if err != nil {
		return ProjectionDebtReceipt{}, false, fmt.Errorf("load identity projection debt: %w", err)
	}
	matches := storedDebtRef == debtRef &&
		storedJobRef == projectionJobRef &&
		storedEventRef == reconciliation.EventRef() &&
		storedRevision == graphRevision &&
		storedKind == "opened" &&
		storedReason == reason.String() &&
		storedDetail == detail.String() &&
		storedExpected == expected.String() &&
		storedObserved == "" &&
		!storedSupersedes.Valid
	if !matches {
		return ProjectionDebtReceipt{}, true, fmt.Errorf("%w: projection-debt replay differs", ErrStoredIntegrity)
	}
	return ProjectionDebtReceipt{debtRef: debtRef, debtEventRef: debtEventRef}, true, nil
}

func projectionDebtRevisionSQLiteValue(
	revision typedmemory.GraphRevision,
) (int64, error) {
	value := revision.Value()
	if value == 0 || value > math.MaxInt64 {
		return 0, fmt.Errorf(
			"%w: identity projection-debt revision exceeds SQLite range",
			ErrProjectionBasis,
		)
	}
	return int64(value), nil
}
