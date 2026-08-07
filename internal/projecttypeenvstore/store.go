package projecttypeenvstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type Store struct {
	database *sql.DB
}

func New(ctx context.Context, database *sql.DB) (*Store, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	if database == nil {
		return nil, ErrStoreRequired
	}
	if err := ensureSchema(ctx, database); err != nil {
		return nil, err
	}
	return &Store{database: database}, nil
}

func (store *Store) PutBaseTypeEnvArtifact(
	ctx context.Context,
	artifact typeenv.BaseTypeEnvArtifact,
) error {
	record, _, err := prepareBaseArtifact(artifact)
	if err != nil {
		return err
	}
	return store.putOne(ctx, record)
}

func (store *Store) PutProjectTypeEnvExtensionArtifact(
	ctx context.Context,
	artifact projecttypeenv.ProjectTypeEnvExtensionArtifact,
) error {
	record, _, err := prepareExtensionArtifact(artifact)
	if err != nil {
		return err
	}
	return store.putOne(ctx, record)
}

func (store *Store) PutRuntimeEvaluationBasisArtifact(
	ctx context.Context,
	artifact projecttypeenv.RuntimeEvaluationBasisArtifact,
) error {
	if len(artifact.AllPins()) != 0 {
		return fmt.Errorf(
			"%w: use PutRuntimeEvaluationBasisResolvedClosure for non-empty X %q",
			ErrRuntimeClosureRequired,
			artifact.Ref().String(),
		)
	}
	return store.PutRuntimeEvaluationBasisClosure(ctx, artifact, nil)
}

// PutRuntimeEvaluationBasisClosure atomically persists X together with every
// exact runtime mechanism artifact needed to verify X after reread.
func (store *Store) PutRuntimeEvaluationBasisClosure(
	ctx context.Context,
	artifact projecttypeenv.RuntimeEvaluationBasisArtifact,
	mechanisms []runtimemechanism.RuntimeMechanismArtifactV1,
) error {
	return store.PutRuntimeEvaluationBasisResolvedClosure(
		ctx,
		artifact,
		mechanisms,
		nil,
	)
}

// PutRuntimeEvaluationBasisResolvedClosure atomically persists X with every
// exact mechanism catalog and registration policy required for transitive
// reread verification.
func (store *Store) PutRuntimeEvaluationBasisResolvedClosure(
	ctx context.Context,
	artifact projecttypeenv.RuntimeEvaluationBasisArtifact,
	mechanisms []runtimemechanism.RuntimeMechanismArtifactV1,
	policies []projecttypeenv.RegistrationPolicyArtifact,
) error {
	record, decoded, err := prepareRuntimeBasisArtifact(artifact)
	if err != nil {
		return err
	}
	mechanismRecords, verifiedMechanisms, err := prepareRuntimeMechanismArtifacts(mechanisms)
	if err != nil {
		return err
	}
	policyRecords, verifiedPolicies, err := prepareRegistrationPolicyArtifacts(policies)
	if err != nil {
		return err
	}
	resolved, err := projecttypeenv.ResolveRuntimeEvaluationBasisClosure(
		decoded,
		verifiedMechanisms,
		verifiedPolicies,
	)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRuntimeClosureRequired, err)
	}
	if err := resolved.VerifyResolvedClosure(); err != nil {
		return fmt.Errorf("%w: %v", ErrRuntimeClosureRequired, err)
	}
	if ctx == nil {
		return ErrContextRequired
	}
	if store == nil || store.database == nil {
		return ErrStoreRequired
	}
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin runtime evaluation basis closure transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()
	for _, mechanismRecord := range mechanismRecords {
		if err := putRuntimeMechanismRecord(ctx, transaction, mechanismRecord); err != nil {
			return err
		}
	}
	for _, policyRecord := range policyRecords {
		if err := putRegistrationPolicyRecord(ctx, transaction, policyRecord); err != nil {
			return err
		}
	}
	if err := putRecord(ctx, transaction, record); err != nil {
		return err
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit runtime evaluation basis closure transaction: %w", err)
	}
	return nil
}

func (store *Store) PutProjectTypeEnvCompositeArtifact(
	ctx context.Context,
	artifact projecttypeenv.ProjectTypeEnvCompositeArtifact,
) error {
	record, _, err := prepareCompositeArtifact(artifact)
	if err != nil {
		return err
	}
	return store.putOne(ctx, record)
}

// PutArtifactClosure atomically writes every exact artifact in a previously
// verified recipe closure. It re-verifies the closure before opening the
// transaction and makes no final-lowerability claim.
func (store *Store) PutArtifactClosure(
	ctx context.Context,
	closure ArtifactClosure,
) error {
	if ctx == nil {
		return ErrContextRequired
	}
	if store == nil || store.database == nil {
		return ErrStoreRequired
	}
	verified, err := verifyArtifactClosure(closure)
	if err != nil {
		return err
	}
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin project TypeEnv closure transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()
	for _, record := range verified.records {
		if record.kind == ArtifactRuntimeBasis {
			for _, mechanismRecord := range verified.mechanismRecords {
				if err := putRuntimeMechanismRecord(ctx, transaction, mechanismRecord); err != nil {
					return err
				}
			}
			for _, policyRecord := range verified.policyRecords {
				if err := putRegistrationPolicyRecord(ctx, transaction, policyRecord); err != nil {
					return err
				}
			}
		}
		if err := putRecord(ctx, transaction, record); err != nil {
			return err
		}
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit project TypeEnv closure transaction: %w", err)
	}
	return nil
}

func (store *Store) GetBaseTypeEnvArtifact(
	ctx context.Context,
	ref typedmemory.TypeEnvRef,
) (typeenv.BaseTypeEnvArtifact, error) {
	if err := validateTypeEnvRef(ref); err != nil {
		return typeenv.BaseTypeEnvArtifact{}, err
	}
	value, err := store.get(ctx, ArtifactBaseTypeEnv, ref.String())
	if err != nil {
		return typeenv.BaseTypeEnvArtifact{}, err
	}
	artifact, ok := value.(typeenv.BaseTypeEnvArtifact)
	if !ok {
		return typeenv.BaseTypeEnvArtifact{}, integrityError(
			ArtifactBaseTypeEnv,
			ref.String(),
			fmt.Errorf("decoded artifact has type %T", value),
		)
	}
	return artifact, nil
}

func (store *Store) GetProjectTypeEnvExtensionArtifact(
	ctx context.Context,
	ref typedmemory.TypeEnvExtensionRef,
) (projecttypeenv.ProjectTypeEnvExtensionArtifact, error) {
	if err := validateExtensionRef(ref); err != nil {
		return projecttypeenv.ProjectTypeEnvExtensionArtifact{}, err
	}
	value, err := store.get(ctx, ArtifactExtensionTypeEnv, ref.String())
	if err != nil {
		return projecttypeenv.ProjectTypeEnvExtensionArtifact{}, err
	}
	artifact, ok := value.(projecttypeenv.ProjectTypeEnvExtensionArtifact)
	if !ok {
		return projecttypeenv.ProjectTypeEnvExtensionArtifact{}, integrityError(
			ArtifactExtensionTypeEnv,
			ref.String(),
			fmt.Errorf("decoded artifact has type %T", value),
		)
	}
	return artifact, nil
}

func (store *Store) GetRuntimeEvaluationBasisArtifact(
	ctx context.Context,
	ref projecttypeenv.RuntimeEvaluationBasisRef,
) (projecttypeenv.RuntimeEvaluationBasisArtifact, error) {
	if err := validateRuntimeBasisRef(ref); err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	if ctx == nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, ErrContextRequired
	}
	if store == nil || store.database == nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, ErrStoreRequired
	}
	scanner := sqlOneRowScanner{query: store.database}
	record, err := loadRecord(ctx, scanner, ArtifactRuntimeBasis, ref.String())
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	value, err := decodeStoredRecord(record)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, storedRecordReadError(
			ArtifactRuntimeBasis,
			ref.String(),
			err,
		)
	}
	artifact, ok := value.(projecttypeenv.RuntimeEvaluationBasisArtifact)
	if !ok {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, integrityError(
			ArtifactRuntimeBasis,
			ref.String(),
			fmt.Errorf("decoded artifact has type %T", value),
		)
	}
	return resolveStoredRuntimeBasis(ctx, scanner, artifact)
}

func (store *Store) GetProjectTypeEnvCompositeArtifact(
	ctx context.Context,
	ref typedmemory.TypeEnvRef,
) (projecttypeenv.ProjectTypeEnvCompositeArtifact, error) {
	if err := validateTypeEnvRef(ref); err != nil {
		return projecttypeenv.ProjectTypeEnvCompositeArtifact{}, err
	}
	value, err := store.get(ctx, ArtifactCompositeTypeEnv, ref.String())
	if err != nil {
		return projecttypeenv.ProjectTypeEnvCompositeArtifact{}, err
	}
	artifact, ok := value.(projecttypeenv.ProjectTypeEnvCompositeArtifact)
	if !ok {
		return projecttypeenv.ProjectTypeEnvCompositeArtifact{}, integrityError(
			ArtifactCompositeTypeEnv,
			ref.String(),
			fmt.Errorf("decoded artifact has type %T", value),
		)
	}
	return artifact, nil
}

func (store *Store) putOne(ctx context.Context, record artifactRecord) error {
	if ctx == nil {
		return ErrContextRequired
	}
	if store == nil || store.database == nil {
		return ErrStoreRequired
	}
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin project TypeEnv artifact transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()
	if err := putRecord(ctx, transaction, record); err != nil {
		return err
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit project TypeEnv artifact transaction: %w", err)
	}
	return nil
}

func putRecord(ctx context.Context, transaction *sql.Tx, record artifactRecord) error {
	if _, err := decodeStoredRecord(record); err != nil {
		return integrityError(record.kind, record.ref, fmt.Errorf("prepare write: %w", err))
	}
	_, err := transaction.ExecContext(
		ctx,
		`INSERT INTO project_typeenv_artifacts (
			artifact_kind,
			artifact_ref,
			artifact_digest,
			canonical_schema_version,
			producer_schema_version,
			canonical_bytes
		)
		SELECT ?, ?, ?, ?, ?, ?
		WHERE NOT EXISTS (
			SELECT 1
			FROM project_typeenv_artifacts
			WHERE artifact_kind = ?
			  AND (artifact_ref = ? OR artifact_digest = ?)
		)`,
		string(record.kind),
		record.ref,
		record.digest,
		record.canonicalSchema,
		record.producerSchema,
		record.canonical,
		string(record.kind),
		record.ref,
		record.digest,
	)
	if err != nil {
		return fmt.Errorf("insert %s %q: %w", record.kind, record.ref, err)
	}
	stored, err := loadRecord(
		ctx,
		sqlOneRowScanner{query: transaction},
		record.kind,
		record.ref,
	)
	if err != nil {
		if errors.Is(err, ErrArtifactNotFound) {
			return fmt.Errorf("%w: %s %q", ErrArtifactConflict, record.kind, record.ref)
		}
		return err
	}
	if !stored.exactEqual(record) {
		return fmt.Errorf("%w: %s %q", ErrArtifactConflict, record.kind, record.ref)
	}
	return nil
}

func (store *Store) get(ctx context.Context, kind ArtifactKind, ref string) (any, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	if store == nil || store.database == nil {
		return nil, ErrStoreRequired
	}
	return readArtifactWithScanner(
		ctx,
		sqlOneRowScanner{query: store.database},
		kind,
		ref,
	)
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type oneRowScanner interface {
	ScanOne(context.Context, string, []any, []any) error
}

type sqlOneRowScanner struct {
	query queryRower
}

func (scanner sqlOneRowScanner) ScanOne(
	ctx context.Context,
	statement string,
	arguments []any,
	destinations []any,
) error {
	if scanner.query == nil {
		return fmt.Errorf("SQL row scanner is required")
	}
	return scanner.query.QueryRowContext(ctx, statement, arguments...).Scan(destinations...)
}

func readArtifactWithScanner(
	ctx context.Context,
	scanner oneRowScanner,
	kind ArtifactKind,
	ref string,
) (any, error) {
	record, err := loadRecord(ctx, scanner, kind, ref)
	if err != nil {
		return nil, err
	}
	value, err := decodeStoredRecord(record)
	if err != nil {
		return nil, storedRecordReadError(kind, ref, err)
	}
	return value, nil
}

func loadRecord(
	ctx context.Context,
	scanner oneRowScanner,
	kind ArtifactKind,
	ref string,
) (artifactRecord, error) {
	if !kind.valid() {
		return artifactRecord{}, fmt.Errorf("artifact kind %q is invalid", kind)
	}
	record := artifactRecord{}
	var storedKind string
	err := scanner.ScanOne(
		ctx,
		`SELECT
			artifact_kind,
			artifact_ref,
			artifact_digest,
			canonical_schema_version,
			producer_schema_version,
			canonical_bytes
		 FROM project_typeenv_artifacts
		 WHERE artifact_kind = ? AND artifact_ref = ?`,
		[]any{string(kind), ref},
		[]any{
			&storedKind,
			&record.ref,
			&record.digest,
			&record.canonicalSchema,
			&record.producerSchema,
			&record.canonical,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return artifactRecord{}, fmt.Errorf("%w: %s %q", ErrArtifactNotFound, kind, ref)
	}
	if err != nil {
		return artifactRecord{}, fmt.Errorf("load %s %q: %w", kind, ref, err)
	}
	record.kind = ArtifactKind(storedKind)
	if record.kind != kind || record.ref != ref {
		return artifactRecord{}, integrityError(
			kind,
			ref,
			fmt.Errorf("selected row coordinate changed to %s %q", record.kind, record.ref),
		)
	}
	return record.clone(), nil
}
