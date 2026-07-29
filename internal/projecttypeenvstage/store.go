package projecttypeenvstage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstore"
)

type Store struct {
	database  *sql.DB
	artifacts *projecttypeenvstore.Store
}

func New(ctx context.Context, database *sql.DB) (*Store, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	if database == nil {
		return nil, ErrStoreRequired
	}
	artifacts, err := projecttypeenvstore.New(ctx, database)
	if err != nil {
		return nil, fmt.Errorf("open project TypeEnv artifact store: %w", err)
	}
	if err := ensureSchema(ctx, database); err != nil {
		return nil, err
	}
	return &Store{database: database, artifacts: artifacts}, nil
}

// PutArtifactClosure delegates immutable B/E/X/C storage to the owning
// artifact store. The closure and Stage record remain separate identities;
// storing either one does not select a head.
func (store *Store) PutArtifactClosure(
	ctx context.Context,
	closure projecttypeenvstore.ArtifactClosure,
) error {
	if ctx == nil {
		return ErrContextRequired
	}
	if store == nil || store.database == nil || store.artifacts == nil {
		return ErrStoreRequired
	}
	return store.artifacts.PutArtifactClosure(ctx, closure)
}

// Put atomically persists the exact Stage bytes, final-lowerer verification,
// and executable-snapshot record. Existing identical rows make the operation
// idempotent; an occupied coordinate with different bytes fails closed.
func (store *Store) Put(
	ctx context.Context,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	verification projecttypeenv.ProjectTypeEnvCompositeVerificationRecord,
	snapshot projecttypeenv.ProjectTypeEnvExecutableSnapshotRecord,
) error {
	if ctx == nil {
		return ErrContextRequired
	}
	if store == nil || store.database == nil || store.artifacts == nil {
		return ErrStoreRequired
	}
	stageRow, verificationRow, snapshotRow, err := preparePersistedStage(
		stage,
		verification,
		snapshot,
	)
	if err != nil {
		return err
	}
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin project TypeEnv Stage transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()
	if err := putVerificationRecord(ctx, transaction, verificationRow); err != nil {
		return err
	}
	if err := putExecutableSnapshotRecord(ctx, transaction, snapshotRow); err != nil {
		return err
	}
	if err := putStageRecord(ctx, transaction, stageRow); err != nil {
		return err
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit project TypeEnv Stage transaction: %w", err)
	}
	return nil
}

// Get returns only immutable decoded data. It does not load B/E/X/C, restore
// the executable TypeEnv, or recreate a final-lowerer capability.
func (store *Store) Get(
	ctx context.Context,
	ref projecttypeenvselection.ProjectTypeEnvStageRef,
) (PersistedStage, error) {
	if ctx == nil {
		return PersistedStage{}, ErrContextRequired
	}
	if store == nil || store.database == nil || store.artifacts == nil {
		return PersistedStage{}, ErrStoreRequired
	}
	parsed, err := projecttypeenvselection.ParseProjectTypeEnvStageRef(ref.String())
	if err != nil || parsed != ref {
		return PersistedStage{}, fmt.Errorf("project TypeEnv Stage reference is required")
	}
	transaction, err := store.database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return PersistedStage{}, fmt.Errorf("begin project TypeEnv Stage read transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()
	stageRow, err := loadStageRecord(ctx, transaction, ref.String())
	if err != nil {
		return PersistedStage{}, err
	}
	snapshotRow, err := loadExecutableSnapshotRecord(
		ctx,
		transaction,
		stageRow.executableRef,
	)
	if err != nil {
		return PersistedStage{}, err
	}
	verificationRow, err := loadVerificationRecord(
		ctx,
		transaction,
		stageRow.verificationRef,
	)
	if err != nil {
		return PersistedStage{}, err
	}
	persisted, err := decodePersistedStage(stageRow, verificationRow, snapshotRow)
	if err != nil {
		return PersistedStage{}, err
	}
	if err := transaction.Commit(); err != nil {
		return PersistedStage{}, fmt.Errorf("commit project TypeEnv Stage read transaction: %w", err)
	}
	return persisted, nil
}

func putVerificationRecord(
	ctx context.Context,
	transaction *sql.Tx,
	record verificationRecord,
) error {
	_, err := transaction.ExecContext(
		ctx,
		`INSERT INTO project_typeenv_composite_verifications (
			verification_ref,
			verification_digest,
			lowerer_schema_version,
			canonical_schema_version,
			canonical_bytes
		)
		SELECT ?, ?, ?, ?, ?
		WHERE NOT EXISTS (
			SELECT 1
			FROM project_typeenv_composite_verifications
			WHERE verification_ref = ? OR verification_digest = ?
		)`,
		record.ref,
		record.digest,
		record.lowererSchema,
		record.canonicalSchema,
		record.canonical,
		record.ref,
		record.digest,
	)
	if err != nil {
		return fmt.Errorf("insert final-lowerer verification %q: %w", record.ref, err)
	}
	stored, err := loadVerificationRecord(ctx, transaction, record.ref)
	if err != nil {
		if errors.Is(err, ErrStageNotFound) {
			return fmt.Errorf(
				"%w: final-lowerer verification %q",
				ErrStageConflict,
				record.ref,
			)
		}
		return err
	}
	if !stored.exactEqual(record) {
		return fmt.Errorf("%w: final-lowerer verification %q", ErrStageConflict, record.ref)
	}
	return nil
}

func putExecutableSnapshotRecord(
	ctx context.Context,
	transaction *sql.Tx,
	record executableSnapshotRecord,
) error {
	_, err := transaction.ExecContext(
		ctx,
		`INSERT INTO project_typeenv_executable_snapshots (
			type_env_ref,
			snapshot_digest,
			lowered_environment_digest,
			source_revision,
			compiler_schema_version,
			lowerer_schema_version,
			verification_ref,
			canonical_schema_version,
			canonical_bytes
		)
		SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?
		WHERE NOT EXISTS (
			SELECT 1
			FROM project_typeenv_executable_snapshots
			WHERE type_env_ref = ? OR snapshot_digest = ?
		)`,
		record.typeEnvRef,
		record.snapshotDigest,
		record.loweredDigest,
		record.sourceRevision,
		record.compilerSchema,
		record.lowererSchema,
		record.verificationRef,
		record.canonicalSchema,
		record.canonical,
		record.typeEnvRef,
		record.snapshotDigest,
	)
	if err != nil {
		return fmt.Errorf(
			"insert project TypeEnv executable snapshot %q: %w",
			record.typeEnvRef,
			err,
		)
	}
	stored, err := loadExecutableSnapshotRecord(ctx, transaction, record.typeEnvRef)
	if err != nil {
		if errors.Is(err, ErrStageNotFound) {
			return fmt.Errorf(
				"%w: executable snapshot %q",
				ErrStageConflict,
				record.typeEnvRef,
			)
		}
		return err
	}
	if !stored.exactEqual(record) {
		return fmt.Errorf(
			"%w: executable snapshot %q",
			ErrStageConflict,
			record.typeEnvRef,
		)
	}
	return nil
}

func putStageRecord(ctx context.Context, transaction *sql.Tx, record stageRecord) error {
	_, err := transaction.ExecContext(
		ctx,
		`INSERT INTO project_typeenv_stages (
			stage_ref,
			stage_digest,
			project_id,
			composite_verification_ref,
			executable_type_env_ref,
			canonical_schema_version,
			canonical_bytes
		)
		SELECT ?, ?, ?, ?, ?, ?, ?
		WHERE NOT EXISTS (
			SELECT 1
			FROM project_typeenv_stages
			WHERE stage_ref = ? OR stage_digest = ?
		)`,
		record.ref,
		record.digest,
		record.project,
		record.verificationRef,
		record.executableRef,
		record.canonicalSchema,
		record.canonical,
		record.ref,
		record.digest,
	)
	if err != nil {
		return fmt.Errorf("insert project TypeEnv Stage %q: %w", record.ref, err)
	}
	stored, err := loadStageRecord(ctx, transaction, record.ref)
	if err != nil {
		if errors.Is(err, ErrStageNotFound) {
			return fmt.Errorf("%w: Stage %q", ErrStageConflict, record.ref)
		}
		return err
	}
	if !stageRecordMatchesCanonical(record, stored, record.canonicalSchema) {
		return fmt.Errorf("%w: Stage %q", ErrStageConflict, record.ref)
	}
	return nil
}

type rowScanner interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func loadStageRecord(
	ctx context.Context,
	scanner rowScanner,
	ref string,
) (stageRecord, error) {
	record := stageRecord{}
	err := scanner.QueryRowContext(
		ctx,
		`SELECT
			stage_ref,
			stage_digest,
			project_id,
			composite_verification_ref,
			executable_type_env_ref,
			canonical_schema_version,
			canonical_bytes
		 FROM project_typeenv_stages
		 WHERE stage_ref = ?`,
		ref,
	).Scan(
		&record.ref,
		&record.digest,
		&record.project,
		&record.verificationRef,
		&record.executableRef,
		&record.canonicalSchema,
		&record.canonical,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return stageRecord{}, fmt.Errorf("%w: %q", ErrStageNotFound, ref)
	}
	if err != nil {
		return stageRecord{}, fmt.Errorf("load project TypeEnv Stage %q: %w", ref, err)
	}
	if record.ref != ref {
		return stageRecord{}, integrityError(
			ref,
			fmt.Errorf("selected Stage row coordinate changed to %q", record.ref),
		)
	}
	if record.executableRef == "" {
		return stageRecord{}, integrityError(
			ref,
			fmt.Errorf("stored Stage has no executable snapshot coordinate"),
		)
	}
	return record.clone(), nil
}

func loadVerificationRecord(
	ctx context.Context,
	scanner rowScanner,
	ref string,
) (verificationRecord, error) {
	record := verificationRecord{}
	err := scanner.QueryRowContext(
		ctx,
		`SELECT
			verification_ref,
			verification_digest,
			lowerer_schema_version,
			canonical_schema_version,
			canonical_bytes
		 FROM project_typeenv_composite_verifications
		 WHERE verification_ref = ?`,
		ref,
	).Scan(
		&record.ref,
		&record.digest,
		&record.lowererSchema,
		&record.canonicalSchema,
		&record.canonical,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return verificationRecord{}, fmt.Errorf(
			"%w: final-lowerer verification %q",
			ErrStageNotFound,
			ref,
		)
	}
	if err != nil {
		return verificationRecord{}, fmt.Errorf(
			"load final-lowerer verification %q: %w",
			ref,
			err,
		)
	}
	if record.ref != ref {
		return verificationRecord{}, integrityError(
			ref,
			fmt.Errorf("selected verification row coordinate changed to %q", record.ref),
		)
	}
	return record.clone(), nil
}

func loadExecutableSnapshotRecord(
	ctx context.Context,
	scanner rowScanner,
	typeEnvRef string,
) (executableSnapshotRecord, error) {
	if typeEnvRef == "" {
		return executableSnapshotRecord{}, fmt.Errorf(
			"%w: executable snapshot coordinate is missing",
			ErrStageNotFound,
		)
	}
	record := executableSnapshotRecord{}
	err := scanner.QueryRowContext(
		ctx,
		`SELECT
			type_env_ref,
			snapshot_digest,
			lowered_environment_digest,
			source_revision,
			compiler_schema_version,
			lowerer_schema_version,
			verification_ref,
			canonical_schema_version,
			canonical_bytes
		 FROM project_typeenv_executable_snapshots
		 WHERE type_env_ref = ?`,
		typeEnvRef,
	).Scan(
		&record.typeEnvRef,
		&record.snapshotDigest,
		&record.loweredDigest,
		&record.sourceRevision,
		&record.compilerSchema,
		&record.lowererSchema,
		&record.verificationRef,
		&record.canonicalSchema,
		&record.canonical,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return executableSnapshotRecord{}, fmt.Errorf(
			"%w: executable snapshot %q",
			ErrStageNotFound,
			typeEnvRef,
		)
	}
	if err != nil {
		return executableSnapshotRecord{}, fmt.Errorf(
			"load project TypeEnv executable snapshot %q: %w",
			typeEnvRef,
			err,
		)
	}
	if record.typeEnvRef != typeEnvRef {
		return executableSnapshotRecord{}, integrityError(
			typeEnvRef,
			fmt.Errorf(
				"selected executable snapshot coordinate changed to %q",
				record.typeEnvRef,
			),
		)
	}
	return record.clone(), nil
}
