package identityreconciliation

import (
	"bytes"
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

type SQLiteService struct {
	database *sql.DB
	clock    Clock
	schema   schemaGate
}

func NewSQLiteService(database *sql.DB, clock Clock) (*SQLiteService, error) {
	if database == nil {
		return nil, ErrDatabaseRequired
	}
	if clock == nil {
		return nil, ErrClockRequired
	}
	return &SQLiteService{
		database: database,
		clock:    clock,
		schema:   newSQLiteSchemaGate(database),
	}, nil
}

func (service *SQLiteService) Commit(
	ctx context.Context,
	request Request,
) (Receipt, error) {
	if ctx == nil {
		return Receipt{}, fmt.Errorf("commit identity reconciliation: context is required")
	}
	if service == nil || service.database == nil || service.clock == nil || service.schema == nil {
		return Receipt{}, ErrDatabaseRequired
	}
	if err := service.schema.RequireCompatible(ctx); err != nil {
		return Receipt{}, fmt.Errorf("%w: %v", ErrSchemaUnavailable, err)
	}
	prepared, err := prepareRequest(request)
	if err != nil {
		return Receipt{}, err
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, service.database)
	if err != nil {
		return Receipt{}, fmt.Errorf("begin identity-reconciliation transaction: %w", err)
	}
	replay, found, err := loadReplay(ctx, transaction, prepared)
	if err != nil {
		return Receipt{}, rollbackWith(ctx, transaction, err)
	}
	if found {
		if err := rollbackSuccess(ctx, transaction); err != nil {
			return Receipt{}, err
		}
		return replay, nil
	}
	if err := service.revalidate(ctx, transaction, prepared); err != nil {
		return Receipt{}, rollbackWith(ctx, transaction, err)
	}
	recordedAt := service.clock.Now().UTC().Format(time.RFC3339Nano)
	if err := persist(ctx, transaction, prepared, recordedAt); err != nil {
		return Receipt{}, rollbackWith(ctx, transaction, err)
	}
	pending, found, err := loadReplay(ctx, transaction, prepared)
	if err != nil {
		return Receipt{}, rollbackWith(ctx, transaction, err)
	}
	if !found {
		return Receipt{}, rollbackWith(
			ctx,
			transaction,
			fmt.Errorf("%w: pending reconciliation rows are missing", ErrStoredIntegrity),
		)
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		durable, durableFound, durableErr := service.Replay(ctx, request)
		if durableErr == nil && durableFound {
			return durable, nil
		}
		return Receipt{}, fmt.Errorf(
			"%w for project %s and idempotency key %q: %v",
			ErrCommitOutcomeUnknown,
			request.Project().String(),
			request.IdempotencyKey().String(),
			errors.Join(finish.Err(), durableErr),
		)
	}
	pending.disposition = CommitApplied
	return pending, nil
}

func (service *SQLiteService) Replay(
	ctx context.Context,
	request Request,
) (Receipt, bool, error) {
	if ctx == nil {
		return Receipt{}, false, fmt.Errorf("replay identity reconciliation: context is required")
	}
	if service == nil || service.database == nil || service.schema == nil {
		return Receipt{}, false, ErrDatabaseRequired
	}
	if err := service.schema.RequireCompatible(ctx); err != nil {
		return Receipt{}, false, fmt.Errorf("%w: %v", ErrSchemaUnavailable, err)
	}
	prepared, err := prepareRequest(request)
	if err != nil {
		return Receipt{}, false, err
	}
	transaction, err := sqlitetransaction.BeginRead(ctx, service.database)
	if err != nil {
		return Receipt{}, false, fmt.Errorf("begin identity-reconciliation replay: %w", err)
	}
	receipt, found, err := loadReplay(ctx, transaction, prepared)
	finish := transaction.Rollback(ctx)
	if err != nil {
		return Receipt{}, false, errors.Join(err, finish.Err())
	}
	if !finish.Succeeded() {
		return Receipt{}, false, finish.Err()
	}
	return receipt, found, nil
}

func (service *SQLiteService) revalidate(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	prepared preparedRequest,
) error {
	var revision int64
	var typeEnvText string
	err := transaction.ScanOne(
		ctx,
		`SELECT graph_revision, active_type_env_ref
		FROM typed_memory_graph_heads WHERE project_id = ?`,
		[]any{prepared.request.Project().String()},
		[]any{&revision, &typeEnvText},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrEntityBasisMissing
	}
	if err != nil {
		return fmt.Errorf("load identity-reconciliation graph head: %w", err)
	}
	basis := prepared.request.Admission().Basis()
	basisRevision, err := reconciliationRevisionSQLiteValue(
		basis.GraphRevision(),
	)
	if err != nil {
		return err
	}
	if revision != basisRevision {
		return ErrStaleGraphRevision
	}
	if typeEnvText != basis.TypeEnvRef().String() {
		return ErrActiveTypeEnvChanged
	}
	entities := append([]typedmemory.EntityID{prepared.primary}, prepared.related...)
	for _, entity := range entities {
		if err := requireExactEntityContext(
			ctx,
			transaction,
			prepared.request.Project(),
			entity,
			prepared.context,
			basis.GraphRevision(),
		); err != nil {
			return err
		}
		if err := requireNoPriorIdentityRedirect(
			ctx,
			transaction,
			prepared.request.Project(),
			entity,
			prepared.context,
			basis.GraphRevision(),
		); err != nil {
			return err
		}
	}
	return nil
}

func requireExactEntityContext(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	revision typedmemory.GraphRevision,
) error {
	revisionValue, err := reconciliationRevisionSQLiteValue(revision)
	if err != nil {
		return err
	}
	var count int64
	err = transaction.ScanOne(
		ctx,
		`SELECT COUNT(*)
		FROM typed_memory_entity_contexts entity_context
		JOIN typed_memory_graph_events event
			ON event.project_id = entity_context.project_id
			AND event.event_ref = entity_context.declared_event_ref
		JOIN typed_memory_graph_commits commit_record
			ON commit_record.project_id = event.project_id
			AND commit_record.event_ref = event.event_ref
		WHERE entity_context.project_id = ?
			AND entity_context.entity_id = ?
			AND entity_context.bounded_context_ref = ?
			AND event.graph_revision <= ?`,
		[]any{project.String(), entity.String(), contextRef.String(), revisionValue},
		[]any{&count},
	)
	if err != nil {
		return fmt.Errorf("inspect exact identity participant: %w", err)
	}
	if count != 1 {
		return fmt.Errorf(
			"%w: entity=%s context=%s count=%d",
			ErrEntityBasisMissing,
			entity.String(),
			contextRef.String(),
			count,
		)
	}
	return nil
}

func requireNoPriorIdentityRedirect(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	revision typedmemory.GraphRevision,
) error {
	revisionValue, err := reconciliationRevisionSQLiteValue(revision)
	if err != nil {
		return err
	}
	var count int64
	err = transaction.ScanOne(
		ctx,
		`SELECT COUNT(*)
		FROM typed_memory_identity_redirects redirect
		JOIN typed_memory_graph_events event
			ON event.project_id = redirect.project_id
			AND event.event_ref = redirect.event_ref
		JOIN typed_memory_graph_commits commit_record
			ON commit_record.project_id = event.project_id
			AND commit_record.event_ref = event.event_ref
		WHERE redirect.project_id = ?
			AND redirect.bounded_context_ref = ?
			AND redirect.source_entity_id = ?
			AND event.graph_revision <= ?`,
		[]any{project.String(), contextRef.String(), entity.String(), revisionValue},
		[]any{&count},
	)
	if err != nil {
		return fmt.Errorf("inspect prior identity history: %w", err)
	}
	if count != 0 {
		return fmt.Errorf("%w: entity %s already has reviewed redirect history", ErrIdentityConflict, entity.String())
	}
	return nil
}

func persist(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	prepared preparedRequest,
	recordedAt string,
) error {
	basis := prepared.request.Admission().Basis()
	project := prepared.request.Project().String()
	expectedRevision, err := reconciliationRevisionSQLiteValue(
		basis.GraphRevision(),
	)
	if err != nil {
		return err
	}
	nextRevision, err := reconciliationRevisionSQLiteValue(
		prepared.nextRevision,
	)
	if err != nil {
		return err
	}
	statements := []sqlStatement{
		{
			query: `INSERT INTO typed_memory_graph_events (
				project_id, event_ref, commit_ref, event_digest,
				expected_revision, graph_revision, basis_type_env_ref,
				result_type_env_ref, change_set_digest,
				canonical_change_set_bytes, change_count, event_kind,
				authority_class, request_provenance_ref, recorded_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, 'non_binding_semantic_assertion', ?, ?)`,
			args: []any{
				project, prepared.eventRef, prepared.commitRef, prepared.eventDigest.String(),
				expectedRevision, nextRevision, basis.TypeEnvRef().String(),
				basis.TypeEnvRef().String(), prepared.changeSetDigest.String(),
				prepared.changeSetBytes, string(prepared.operation),
				basis.Provenance().String(), recordedAt,
			},
		},
		{
			query: `INSERT INTO typed_memory_identity_reconciliations (
				project_id, event_ref, commit_ref, reconciliation_ref,
				operation, bounded_context_ref, primary_entity_id,
				reconciliation_basis_ref, basis_type_env_ref,
				basis_graph_revision, review_payload_digest,
				review_provenance_ref, basis_digest, canonical_basis_bytes,
				change_digest, canonical_change_bytes, admission_digest,
				canonical_admission_bytes, reconciliation_digest,
				canonical_reconciliation_bytes, recorded_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			args: []any{
				project, prepared.eventRef, prepared.commitRef, prepared.reconciliationRef,
				string(prepared.operation), prepared.context.String(), prepared.primary.String(),
				basis.Basis().String(), basis.TypeEnvRef().String(), expectedRevision,
				basis.PayloadDigest().String(), basis.Provenance().String(),
				prepared.basisDigest.String(), prepared.basisBytes,
				prepared.changeSetDigest.String(), prepared.changeSetBytes,
				prepared.admissionDigest.String(), prepared.admissionBytes,
				prepared.reconciliationDigest.String(), prepared.reconciliationBytes,
				recordedAt,
			},
		},
	}
	for index, entity := range prepared.related {
		statements = append(
			statements,
			sqlStatement{
				query: `INSERT INTO typed_memory_identity_reconciliation_participants (
					project_id, event_ref, participant_ordinal, participant_role, entity_id
				) VALUES (?, ?, ?, ?, ?)`,
				args: []any{project, prepared.eventRef, int64(index), prepared.participantRole, entity.String()},
			},
			sqlStatement{
				query: `INSERT INTO typed_memory_identity_redirects (
					project_id, event_ref, redirect_ordinal, resolution_kind,
					bounded_context_ref, source_entity_id, target_entity_id,
					reconciliation_basis_ref
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				args: redirectArguments(prepared, index, entity),
			},
		)
	}
	statements = append(
		statements,
		sqlStatement{
			query: `INSERT INTO typed_memory_idempotency_history (
				project_id, idempotency_key, change_set_digest,
				event_ref, graph_revision, result_digest, recorded_at
			) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			args: []any{
				project, prepared.request.IdempotencyKey().String(),
				prepared.changeSetDigest.String(), prepared.eventRef, nextRevision,
				prepared.eventDigest.String(), recordedAt,
			},
		},
		sqlStatement{
			query: `INSERT INTO typed_memory_projection_jobs (
				project_id, projection_job_ref, semantic_event_ref,
				graph_revision, target_kind, input_event_digest, recorded_at
			) VALUES (?, ?, ?, ?, 'project_carriers', ?, ?)`,
			args: []any{
				project, prepared.projectionJobRef, prepared.eventRef,
				nextRevision, prepared.eventDigest.String(), recordedAt,
			},
		},
		sqlStatement{
			query: `INSERT INTO typed_memory_identity_reconciliation_closures (
				project_id, event_ref, commit_ref, event_digest, change_digest,
				reconciliation_digest, materialization_digest,
				canonical_materialization_bytes, participant_count,
				redirect_count, recorded_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			args: []any{
				project, prepared.eventRef, prepared.commitRef, prepared.eventDigest.String(),
				prepared.changeSetDigest.String(), prepared.reconciliationDigest.String(),
				prepared.materializationDigest.String(), prepared.materializationBytes,
				int64(len(prepared.related)), int64(len(prepared.related)), recordedAt,
			},
		},
		sqlStatement{
			query: `INSERT INTO typed_memory_graph_commits (
				project_id, commit_ref, event_ref, event_digest,
				expected_revision, graph_revision, change_set_digest,
				idempotency_key, projection_job_ref, entity_count,
				entity_context_count, recorded_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, ?)`,
			args: []any{
				project, prepared.commitRef, prepared.eventRef, prepared.eventDigest.String(),
				expectedRevision, nextRevision, prepared.changeSetDigest.String(),
				prepared.request.IdempotencyKey().String(), prepared.projectionJobRef, recordedAt,
			},
		},
	)
	for _, statement := range statements {
		if _, err := transaction.Execute(ctx, statement.query, statement.args); err != nil {
			return fmt.Errorf("persist reviewed identity reconciliation: %w", err)
		}
	}
	var revision int64
	var eventRef string
	var commitRef string
	err = transaction.ScanOne(
		ctx,
		`SELECT graph_revision, last_event_ref, last_commit_ref
		FROM typed_memory_graph_heads WHERE project_id = ?`,
		[]any{project},
		[]any{&revision, &eventRef, &commitRef},
	)
	if err != nil {
		return fmt.Errorf("verify identity-reconciliation graph head: %w", err)
	}
	if revision != nextRevision || eventRef != prepared.eventRef || commitRef != prepared.commitRef {
		return fmt.Errorf("%w: graph head did not advance to the exact reconciliation event", ErrStoredIntegrity)
	}
	return nil
}

type sqlStatement struct {
	query string
	args  []any
}

func redirectArguments(prepared preparedRequest, index int, participant typedmemory.EntityID) []any {
	source := participant
	target := prepared.primary
	if prepared.operation == typedmemory.ReconciliationSplitEntity {
		source = prepared.primary
		target = participant
	}
	return []any{
		prepared.request.Project().String(),
		prepared.eventRef,
		int64(index),
		prepared.redirectKind,
		prepared.context.String(),
		source.String(),
		target.String(),
		prepared.request.Admission().Basis().Basis().String(),
	}
}

func loadReplay(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	prepared preparedRequest,
) (Receipt, bool, error) {
	project := prepared.request.Project().String()
	key := prepared.request.IdempotencyKey().String()
	var storedChangeDigest string
	var storedEventRef string
	var storedRevision int64
	var storedResultDigest string
	err := transaction.ScanOne(
		ctx,
		`SELECT change_set_digest, event_ref, graph_revision, result_digest
		FROM typed_memory_idempotency_history
		WHERE project_id = ? AND idempotency_key = ?`,
		[]any{project, key},
		[]any{&storedChangeDigest, &storedEventRef, &storedRevision, &storedResultDigest},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Receipt{}, false, nil
	}
	if err != nil {
		return Receipt{}, false, fmt.Errorf("load identity-reconciliation replay key: %w", err)
	}
	nextRevision, err := reconciliationRevisionSQLiteValue(
		prepared.nextRevision,
	)
	if err != nil {
		return Receipt{}, true, err
	}
	identityMatches := storedChangeDigest == prepared.changeSetDigest.String() &&
		storedEventRef == prepared.eventRef &&
		storedRevision == nextRevision &&
		storedResultDigest == prepared.eventDigest.String()
	if !identityMatches {
		return Receipt{}, true, ErrIdempotencyConflict
	}
	if err := verifyStoredReplayRows(ctx, transaction, prepared); err != nil {
		return Receipt{}, true, err
	}
	return Receipt{
		disposition:       CommitReplay,
		reconciliationRef: prepared.reconciliationRef,
		eventRef:          prepared.eventRef,
		commitRef:         prepared.commitRef,
		revision:          prepared.nextRevision,
		resultDigest:      prepared.eventDigest,
	}, true, nil
}

func verifyStoredReplayRows(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	prepared preparedRequest,
) error {
	project := prepared.request.Project().String()
	var operation string
	var contextText string
	var primaryText string
	var basisRef string
	var basisTypeEnv string
	var basisRevision int64
	var payloadDigest string
	var provenance string
	var basisDigest string
	var basisBytes []byte
	var changeDigest string
	var changeBytes []byte
	var admissionDigest string
	var admissionBytes []byte
	var reconciliationDigest string
	var reconciliationBytes []byte
	var closureEventDigest string
	var closureChangeDigest string
	var closureReconciliationDigest string
	var materializationDigest string
	var materializationBytes []byte
	var participantCount int64
	var redirectCount int64
	err := transaction.ScanOne(
		ctx,
		`SELECT reconciliation.operation, reconciliation.bounded_context_ref,
			reconciliation.primary_entity_id, reconciliation.reconciliation_basis_ref,
			reconciliation.basis_type_env_ref, reconciliation.basis_graph_revision,
			reconciliation.review_payload_digest, reconciliation.review_provenance_ref,
			reconciliation.basis_digest, reconciliation.canonical_basis_bytes,
			reconciliation.change_digest, reconciliation.canonical_change_bytes,
			reconciliation.admission_digest, reconciliation.canonical_admission_bytes,
			reconciliation.reconciliation_digest,
			reconciliation.canonical_reconciliation_bytes,
			closure.event_digest, closure.change_digest,
			closure.reconciliation_digest, closure.materialization_digest,
			closure.canonical_materialization_bytes,
			closure.participant_count, closure.redirect_count
		FROM typed_memory_identity_reconciliations reconciliation
		JOIN typed_memory_identity_reconciliation_closures closure
			ON closure.project_id = reconciliation.project_id
			AND closure.event_ref = reconciliation.event_ref
		WHERE reconciliation.project_id = ?
			AND reconciliation.event_ref = ?
			AND reconciliation.commit_ref = ?
			AND reconciliation.reconciliation_ref = ?`,
		[]any{project, prepared.eventRef, prepared.commitRef, prepared.reconciliationRef},
		[]any{
			&operation, &contextText, &primaryText, &basisRef, &basisTypeEnv,
			&basisRevision, &payloadDigest, &provenance, &basisDigest, &basisBytes,
			&changeDigest, &changeBytes, &admissionDigest, &admissionBytes,
			&reconciliationDigest, &reconciliationBytes, &closureEventDigest,
			&closureChangeDigest, &closureReconciliationDigest,
			&materializationDigest, &materializationBytes,
			&participantCount, &redirectCount,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: reconciliation closure is missing", ErrStoredIntegrity)
	}
	if err != nil {
		return fmt.Errorf("load identity-reconciliation closure: %w", err)
	}
	basis := prepared.request.Admission().Basis()
	basisRevisionValue, err := reconciliationRevisionSQLiteValue(
		basis.GraphRevision(),
	)
	if err != nil {
		return err
	}
	matches := operation == string(prepared.operation) &&
		contextText == prepared.context.String() &&
		primaryText == prepared.primary.String() &&
		basisRef == basis.Basis().String() &&
		basisTypeEnv == basis.TypeEnvRef().String() &&
		basisRevision == basisRevisionValue &&
		payloadDigest == basis.PayloadDigest().String() &&
		provenance == basis.Provenance().String() &&
		basisDigest == prepared.basisDigest.String() &&
		bytes.Equal(basisBytes, prepared.basisBytes) &&
		changeDigest == prepared.changeSetDigest.String() &&
		bytes.Equal(changeBytes, prepared.changeSetBytes) &&
		admissionDigest == prepared.admissionDigest.String() &&
		bytes.Equal(admissionBytes, prepared.admissionBytes) &&
		reconciliationDigest == prepared.reconciliationDigest.String() &&
		bytes.Equal(reconciliationBytes, prepared.reconciliationBytes) &&
		closureEventDigest == prepared.eventDigest.String() &&
		closureChangeDigest == prepared.changeSetDigest.String() &&
		closureReconciliationDigest == prepared.reconciliationDigest.String() &&
		materializationDigest == prepared.materializationDigest.String() &&
		bytes.Equal(materializationBytes, prepared.materializationBytes) &&
		participantCount == int64(len(prepared.related)) &&
		redirectCount == int64(len(prepared.related))
	if !matches {
		return fmt.Errorf("%w: durable reconciliation carriers differ from the exact request", ErrStoredIntegrity)
	}
	for index, entity := range prepared.related {
		if err := verifyParticipantAndRedirect(ctx, transaction, prepared, index, entity); err != nil {
			return err
		}
	}
	return verifySpineRows(ctx, transaction, prepared)
}

func verifyParticipantAndRedirect(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	prepared preparedRequest,
	index int,
	entity typedmemory.EntityID,
) error {
	var role string
	var participant string
	var resolutionKind string
	var contextText string
	var source string
	var target string
	var basisRef string
	err := transaction.ScanOne(
		ctx,
		`SELECT participant.participant_role, participant.entity_id,
			redirect.resolution_kind, redirect.bounded_context_ref,
			redirect.source_entity_id, redirect.target_entity_id,
			redirect.reconciliation_basis_ref
		FROM typed_memory_identity_reconciliation_participants participant
		JOIN typed_memory_identity_redirects redirect
			ON redirect.project_id = participant.project_id
			AND redirect.event_ref = participant.event_ref
			AND redirect.redirect_ordinal = participant.participant_ordinal
		WHERE participant.project_id = ? AND participant.event_ref = ?
			AND participant.participant_ordinal = ?`,
		[]any{prepared.request.Project().String(), prepared.eventRef, int64(index)},
		[]any{&role, &participant, &resolutionKind, &contextText, &source, &target, &basisRef},
	)
	if err != nil {
		return fmt.Errorf("%w: load reconciliation participant %d: %v", ErrStoredIntegrity, index, err)
	}
	expectedSource := entity.String()
	expectedTarget := prepared.primary.String()
	if prepared.operation == typedmemory.ReconciliationSplitEntity {
		expectedSource = prepared.primary.String()
		expectedTarget = entity.String()
	}
	matches := role == prepared.participantRole &&
		participant == entity.String() &&
		resolutionKind == prepared.redirectKind &&
		contextText == prepared.context.String() &&
		source == expectedSource &&
		target == expectedTarget &&
		basisRef == prepared.request.Admission().Basis().Basis().String()
	if !matches {
		return fmt.Errorf("%w: reconciliation participant %d differs", ErrStoredIntegrity, index)
	}
	return nil
}

func verifySpineRows(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	prepared preparedRequest,
) error {
	nextRevision, err := reconciliationRevisionSQLiteValue(
		prepared.nextRevision,
	)
	if err != nil {
		return err
	}
	var count int64
	err = transaction.ScanOne(
		ctx,
		`SELECT COUNT(*)
		FROM typed_memory_graph_events event
		JOIN typed_memory_graph_commits commit_record
			ON commit_record.project_id = event.project_id
			AND commit_record.event_ref = event.event_ref
		JOIN typed_memory_projection_jobs job
			ON job.project_id = event.project_id
			AND job.semantic_event_ref = event.event_ref
		WHERE event.project_id = ? AND event.event_ref = ?
			AND event.commit_ref = ? AND event.event_digest = ?
			AND event.graph_revision = ? AND event.change_set_digest = ?
			AND event.event_kind = ? AND event.authority_class = 'non_binding_semantic_assertion'
			AND commit_record.commit_ref = ? AND commit_record.entity_count = 0
			AND commit_record.entity_context_count = 0
			AND job.projection_job_ref = ?`,
		[]any{
			prepared.request.Project().String(), prepared.eventRef, prepared.commitRef,
			prepared.eventDigest.String(), nextRevision,
			prepared.changeSetDigest.String(), string(prepared.operation), prepared.commitRef,
			prepared.projectionJobRef,
		},
		[]any{&count},
	)
	if err != nil {
		return fmt.Errorf("verify identity-reconciliation graph spine: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("%w: exact graph spine count=%d", ErrStoredIntegrity, count)
	}
	return nil
}

func reconciliationRevisionSQLiteValue(
	revision typedmemory.GraphRevision,
) (int64, error) {
	value := revision.Value()
	if value > math.MaxInt64 {
		return 0, fmt.Errorf(
			"%w: identity-reconciliation revision exceeds SQLite range",
			ErrStoredIntegrity,
		)
	}
	return int64(value), nil
}

func rollbackWith(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	cause error,
) error {
	finish := transaction.Rollback(ctx)
	return errors.Join(cause, finish.Err())
}

func rollbackSuccess(ctx context.Context, transaction *sqlitetransaction.Transaction) error {
	finish := transaction.Rollback(ctx)
	if !finish.Succeeded() {
		return finish.Err()
	}
	return nil
}
