// Package sqlite persists the source-native profile authority closure introduced
// by schema v43. It never reconstructs a generic SpeechAct completion capability
// and does not write the legacy Presentation/AuthorityResolution chain.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileauthority"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const (
	contentTable         = "profile_declaration_authorization_contents_v2"
	preparationTable     = "profile_declaration_authorization_preparations_v2"
	permissionTable      = "profile_declaration_permissions_v2"
	effectTable          = "profile_declaration_instituted_effects_v2"
	basisTable           = "profile_declaration_authority_bases_v2"
	preparationSavepoint = "profile_authority_preparation_source"
)

type WriteKind string

const (
	WriteStaged      WriteKind = "staged"
	WriteExactReplay WriteKind = "exact_replay"
	WriteRecovered   WriteKind = "recovered"
	WriteRejected    WriteKind = "rejected"
)

type PreparationSourceWriteResult struct {
	kind     WriteKind
	prepared profileauthority.PreparedAuthorization
	recorded authority.RecordedSpeechActSource
	detail   string
}

func (result PreparationSourceWriteResult) Kind() WriteKind {
	return result.kind
}

func (result PreparationSourceWriteResult) Prepared() (
	profileauthority.PreparedAuthorization,
	bool,
) {
	usable := result.kind == WriteStaged || result.kind == WriteExactReplay
	if !usable {
		return profileauthority.PreparedAuthorization{}, false
	}
	_, ok := result.prepared.Digest()
	return result.prepared, ok
}

func (result PreparationSourceWriteResult) RecordedSource() (
	authority.RecordedSpeechActSource,
	bool,
) {
	usable := result.kind == WriteStaged || result.kind == WriteExactReplay
	return result.recorded, usable && result.recorded.Valid()
}

func (result PreparationSourceWriteResult) RejectionDetail() (string, bool) {
	return result.detail, result.kind == WriteRejected && result.detail != ""
}

type Store struct {
	database     *sql.DB
	sourceWriter *authority.SpeechActSourceWriter
	now          func() time.Time
}

func Open(database *sql.DB) (*Store, error) {
	return openWithClock(database, time.Now)
}

// OpenWithClock is the deterministic construction seam for integration
// fixtures that must prove pre-Work and post-Work chronology. Production code
// uses Open. The supplied clock is sampled only inside validated boundaries.
func OpenWithClock(database *sql.DB, now func() time.Time) (*Store, error) {
	return openWithClock(database, now)
}

func openWithClock(database *sql.DB, now func() time.Time) (*Store, error) {
	if database == nil || now == nil {
		return nil, fmt.Errorf("profile authority SQLite store requires a database and clock")
	}
	if err := requireV43(database); err != nil {
		return nil, err
	}
	if err := requireV44(database); err != nil {
		return nil, err
	}
	sourceWriter, err := authority.OpenSpeechActSourceWriter(database)
	if err != nil {
		return nil, err
	}
	return &Store{
		database:     database,
		sourceWriter: sourceWriter,
		now:          now,
	}, nil
}

// RecordPreparationAndSourceInTransaction stores only already-successful TTY
// material. Cancellation occurs before this boundary and therefore writes no
// content, preparation, capture, or SpeechAct row. The caller owns the enclosing
// BEGIN IMMEDIATE and its final commit/rollback.
func (store *Store) RecordPreparationAndSourceInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	prepared profileauthority.PreparedAuthorization,
	source authority.VerifiedSpeechActSource,
) (PreparationSourceWriteResult, error) {
	if err := store.validateMutation(ctx, transaction); err != nil {
		return PreparationSourceWriteResult{}, err
	}
	if _, ok := prepared.Digest(); !ok {
		return PreparationSourceWriteResult{}, fmt.Errorf(
			"profile authority source write requires canonical prepared authorization",
		)
	}
	if err := beginSavepoint(ctx, transaction, preparationSavepoint); err != nil {
		return PreparationSourceWriteResult{}, err
	}
	preparationKind, err := persistPreparation(
		ctx,
		transaction,
		prepared,
		canonicalTime(store.now()),
	)
	if err != nil {
		return PreparationSourceWriteResult{}, rollbackSavepointError(
			ctx,
			transaction,
			preparationSavepoint,
			err,
		)
	}
	if preparationKind == mutationRejected {
		detail := "profile authorization preparation identity collides with different canonical material"
		return rejectedPreparationResult(
			ctx,
			transaction,
			preparationSavepoint,
			detail,
		)
	}
	sourceResult, err := store.sourceWriter.RecordInTransaction(
		ctx,
		transaction,
		source,
	)
	if err != nil {
		return PreparationSourceWriteResult{}, rollbackSavepointError(
			ctx,
			transaction,
			preparationSavepoint,
			err,
		)
	}
	if sourceResult.Kind() == authority.SpeechActSourceWriteRejected {
		detail, _ := sourceResult.RejectionDetail()
		return rejectedPreparationResult(
			ctx,
			transaction,
			preparationSavepoint,
			detail,
		)
	}
	recorded, ok := sourceResult.RecordedSource()
	if !ok {
		err = fmt.Errorf("generic SpeechAct source writer returned no exact source")
		return PreparationSourceWriteResult{}, rollbackSavepointError(
			ctx,
			transaction,
			preparationSavepoint,
			err,
		)
	}
	if err := profileauthority.ValidateRecordedSource(prepared, recorded); err != nil {
		return PreparationSourceWriteResult{}, rollbackSavepointError(
			ctx,
			transaction,
			preparationSavepoint,
			err,
		)
	}
	if err := releaseSavepoint(ctx, transaction, preparationSavepoint); err != nil {
		return PreparationSourceWriteResult{}, err
	}
	kind := WriteStaged
	if preparationKind == mutationExact &&
		sourceResult.Kind() == authority.SpeechActSourceWriteExactReplay {
		kind = WriteExactReplay
	}
	return PreparationSourceWriteResult{
		kind:     kind,
		prepared: prepared,
		recorded: recorded,
	}, nil
}

func (store *Store) validateMutation(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
) error {
	if store == nil || store.database == nil || store.sourceWriter == nil || store.now == nil {
		return fmt.Errorf("profile authority SQLite store is not open")
	}
	if ctx == nil {
		return fmt.Errorf("profile authority SQLite mutation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := transaction.RequireImmediate(); err != nil {
		return fmt.Errorf("profile authority SQLite mutation requires BEGIN IMMEDIATE: %w", err)
	}
	return nil
}

func requireV43(database *sql.DB) error {
	var versionCount int
	err := database.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = 43",
	).Scan(&versionCount)
	if err != nil || versionCount != 1 {
		return errors.Join(fmt.Errorf("profile authority schema v43 is unavailable"), err)
	}
	var tableCount int
	err = database.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master
		 WHERE type = 'table' AND name IN (?, ?, ?, ?, ?)`,
		contentTable,
		preparationTable,
		permissionTable,
		effectTable,
		basisTable,
	).Scan(&tableCount)
	if err != nil || tableCount != 5 {
		return errors.Join(fmt.Errorf("profile authority schema v43 tables are incomplete"), err)
	}
	return nil
}

func beginSavepoint(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	name string,
) error {
	_, err := transaction.Execute(ctx, "SAVEPOINT "+name, nil)
	return err
}

func releaseSavepoint(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	name string,
) error {
	_, err := transaction.Execute(ctx, "RELEASE SAVEPOINT "+name, nil)
	return err
}

func rollbackSavepointError(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	name string,
	cause error,
) error {
	_, rollbackErr := transaction.Execute(ctx, "ROLLBACK TO SAVEPOINT "+name, nil)
	_, releaseErr := transaction.Execute(ctx, "RELEASE SAVEPOINT "+name, nil)
	return errors.Join(cause, rollbackErr, releaseErr)
}

func rejectedPreparationResult(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	name string,
	detail string,
) (PreparationSourceWriteResult, error) {
	err := rollbackSavepointError(ctx, transaction, name, nil)
	if err != nil {
		return PreparationSourceWriteResult{}, err
	}
	return PreparationSourceWriteResult{
		kind:   WriteRejected,
		detail: detail,
	}, nil
}

func canonicalTime(value time.Time) time.Time {
	return value.UTC().Round(0)
}

func formatTime(value time.Time) string {
	return canonicalTime(value).Format(time.RFC3339Nano)
}
