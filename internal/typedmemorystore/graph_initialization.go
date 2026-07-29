package typedmemorystore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// ProjectGraphInitializationResult is a closed outcome. None of its variants
// is a graph event, a project-TypeEnv selection, or an authority receipt.
type ProjectGraphInitializationResult interface {
	projectGraphInitializationResultVariant()
}

type projectGraphBaseCoordinate struct {
	project  projectledger.ProjectID
	snapshot TypeEnvSnapshot
}

func newProjectGraphBaseCoordinate(
	project projectledger.ProjectID,
	snapshot TypeEnvSnapshot,
) (projectGraphBaseCoordinate, error) {
	canonicalProject, err := projectledger.ParseProjectID(project.String())
	if err != nil || canonicalProject != project {
		return projectGraphBaseCoordinate{},
			fmt.Errorf("project graph base coordinate requires a project")
	}
	verified, err := canonicalTypeEnvSnapshot(snapshot)
	if err != nil {
		return projectGraphBaseCoordinate{}, err
	}
	return projectGraphBaseCoordinate{
		project:  canonicalProject,
		snapshot: verified,
	}, nil
}

func (coordinate projectGraphBaseCoordinate) Project() projectledger.ProjectID {
	return coordinate.project
}

func (coordinate projectGraphBaseCoordinate) BaseTypeEnv() typedmemory.TypeEnvRef {
	return coordinate.snapshot.Ref()
}

func (coordinate projectGraphBaseCoordinate) GraphRevision() typedmemory.GraphRevision {
	return typedmemory.NewGraphRevision(0)
}

func (coordinate projectGraphBaseCoordinate) Snapshot() TypeEnvSnapshot {
	return cloneSnapshot(coordinate.snapshot)
}

// InitializedAtBase means this invocation atomically created the revision-zero
// graph head at the exact supplied base snapshot.
type InitializedAtBase struct {
	coordinate projectGraphBaseCoordinate
}

func (InitializedAtBase) projectGraphInitializationResultVariant() {}

func (result InitializedAtBase) Project() projectledger.ProjectID {
	return result.coordinate.Project()
}

func (result InitializedAtBase) BaseTypeEnv() typedmemory.TypeEnvRef {
	return result.coordinate.BaseTypeEnv()
}

func (result InitializedAtBase) GraphRevision() typedmemory.GraphRevision {
	return result.coordinate.GraphRevision()
}

func (result InitializedAtBase) Snapshot() TypeEnvSnapshot {
	return result.coordinate.Snapshot()
}

// AlreadyExactAtBase means the exact revision-zero coordinate already existed
// and this invocation performed no semantic write.
type AlreadyExactAtBase struct {
	coordinate projectGraphBaseCoordinate
}

func (AlreadyExactAtBase) projectGraphInitializationResultVariant() {}

func (result AlreadyExactAtBase) Project() projectledger.ProjectID {
	return result.coordinate.Project()
}

func (result AlreadyExactAtBase) BaseTypeEnv() typedmemory.TypeEnvRef {
	return result.coordinate.BaseTypeEnv()
}

func (result AlreadyExactAtBase) GraphRevision() typedmemory.GraphRevision {
	return result.coordinate.GraphRevision()
}

func (result AlreadyExactAtBase) Snapshot() TypeEnvSnapshot {
	return result.coordinate.Snapshot()
}

// AlreadyActive means the graph has advanced beyond revision zero. The
// initializer leaves the active graph untouched.
type AlreadyActive struct {
	project       projectledger.ProjectID
	revision      typedmemory.GraphRevision
	activeTypeEnv typedmemory.TypeEnvRef
}

func (AlreadyActive) projectGraphInitializationResultVariant() {}

func (result AlreadyActive) Project() projectledger.ProjectID {
	return result.project
}

func (result AlreadyActive) GraphRevision() typedmemory.GraphRevision {
	return result.revision
}

func (result AlreadyActive) ActiveTypeEnv() typedmemory.TypeEnvRef {
	return result.activeTypeEnv
}

// Conflict means a revision-zero graph already pins a different exact base
// snapshot. The presented snapshot is not persisted.
type Conflict struct {
	project   projectledger.ProjectID
	existing  TypeEnvSnapshot
	presented TypeEnvSnapshot
}

func (Conflict) projectGraphInitializationResultVariant() {}

func (result Conflict) Project() projectledger.ProjectID {
	return result.project
}

func (result Conflict) ExistingSnapshot() TypeEnvSnapshot {
	return cloneSnapshot(result.existing)
}

func (result Conflict) PresentedSnapshot() TypeEnvSnapshot {
	return cloneSnapshot(result.presented)
}

type sqliteProjectGraphInitializer struct {
	database *sql.DB
	loader   TypeEnvLoader
	clock    Clock
}

var _ ProjectGraphInitializationPort = (*sqliteProjectGraphInitializer)(nil)

// NewSQLiteProjectGraphInitializer returns only the narrow initialization
// capability, not the generic typed-memory adapter.
func NewSQLiteProjectGraphInitializer(
	database *sql.DB,
	loader TypeEnvLoader,
	clock Clock,
) (ProjectGraphInitializationPort, error) {
	if database == nil {
		return nil, ErrDatabaseRequired
	}
	if loader == nil {
		return nil, ErrTypeEnvLoaderRequired
	}
	if clock == nil {
		return nil, ErrClockRequired
	}
	return &sqliteProjectGraphInitializer{
		database: database,
		loader:   loader,
		clock:    clock,
	}, nil
}

func (initializer *sqliteProjectGraphInitializer) InitializeProjectGraphAtBaseTypeEnv(
	ctx context.Context,
	project projectledger.ProjectID,
	snapshot TypeEnvSnapshot,
) (ProjectGraphInitializationResult, error) {
	if ctx == nil {
		return nil, fmt.Errorf(
			"initialize project graph at base TypeEnv: context is required",
		)
	}
	if initializer == nil || initializer.database == nil {
		return nil, ErrDatabaseRequired
	}
	verified, err := initializer.verifyBaseSnapshot(snapshot)
	if err != nil {
		return nil, err
	}
	coordinate, err := newProjectGraphBaseCoordinate(project, verified)
	if err != nil {
		return nil, err
	}
	transaction, err := sqlitetransaction.BeginImmediate(
		ctx,
		initializer.database,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"begin project graph initialization: %w",
			err,
		)
	}
	result, changed, err := initializer.initializeInTransaction(
		ctx,
		transaction,
		coordinate,
	)
	if err != nil {
		return nil, rollbackError(ctx, transaction, err)
	}
	if !changed {
		if err := finishProjectGraphInitializationRead(ctx, transaction); err != nil {
			return nil, err
		}
		return result, nil
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		return nil, fmt.Errorf(
			"%w: commit project graph initialization: %v",
			ErrCommitOutcomeUnknown,
			finish.Err(),
		)
	}
	return result, nil
}

func (initializer *sqliteProjectGraphInitializer) verifyBaseSnapshot(
	snapshot TypeEnvSnapshot,
) (TypeEnvSnapshot, error) {
	verified, err := canonicalTypeEnvSnapshot(snapshot)
	if err != nil {
		return TypeEnvSnapshot{}, fmt.Errorf(
			"initialize project graph at base TypeEnv: %w",
			err,
		)
	}
	if verified.Format().String() != BaseTypeEnvSnapshotFormat {
		return TypeEnvSnapshot{}, fmt.Errorf(
			"initialize project graph at base TypeEnv: snapshot format %q is not %q",
			verified.Format().String(),
			BaseTypeEnvSnapshotFormat,
		)
	}
	environment, _, err := initializer.loader.LoadTypeEnv(verified)
	if err != nil {
		return TypeEnvSnapshot{}, fmt.Errorf(
			"initialize project graph at executable base TypeEnv: %w",
			err,
		)
	}
	if !loadedEnvironmentMatchesSnapshot(environment, verified) {
		return TypeEnvSnapshot{}, fmt.Errorf(
			"initialize project graph at executable base TypeEnv: loaded environment does not match immutable snapshot metadata",
		)
	}
	return verified, nil
}

func (initializer *sqliteProjectGraphInitializer) initializeInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	coordinate projectGraphBaseCoordinate,
) (ProjectGraphInitializationResult, bool, error) {
	if err := requireGenericStorageCapability(ctx, transaction); err != nil {
		return nil, false, err
	}
	if err := projectledger.RequireExactPersistedBinding(
		ctx,
		transaction,
		coordinate.Project(),
	); err != nil {
		return nil, false, err
	}
	head, found, err := loadOptionalProjectGraphHead(
		ctx,
		transaction,
		coordinate.Project(),
	)
	if err != nil {
		return nil, false, err
	}
	if found {
		return initializer.classifyExistingHead(
			ctx,
			transaction,
			coordinate,
			head,
		)
	}
	recordedAt := canonicalTime(initializer.clock.Now())
	if err := putExactTypeEnvSnapshotTx(
		ctx,
		transaction,
		coordinate.Snapshot(),
		recordedAt,
	); err != nil {
		return nil, false, err
	}
	_, err = transaction.Execute(
		ctx,
		`INSERT INTO typed_memory_graph_heads (
			project_id,
			graph_revision,
			active_type_env_ref,
			last_event_ref,
			last_commit_ref,
			updated_at
		) VALUES (?, 0, ?, '', '', ?)`,
		[]any{
			coordinate.Project().String(),
			coordinate.BaseTypeEnv().String(),
			recordedAt,
		},
	)
	if err != nil {
		return nil, false, fmt.Errorf(
			"insert revision-zero project graph head: %w",
			err,
		)
	}
	if err := verifyInitializedProjectGraphTx(
		ctx,
		transaction,
		coordinate,
	); err != nil {
		return nil, false, err
	}
	return InitializedAtBase{coordinate: coordinate}, true, nil
}

func (initializer *sqliteProjectGraphInitializer) classifyExistingHead(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	coordinate projectGraphBaseCoordinate,
	head GraphHead,
) (ProjectGraphInitializationResult, bool, error) {
	if err := verifyCurrentHeadClosure(ctx, transaction, head); err != nil {
		return nil, false, err
	}
	if _, err := verifyExactV46AdmissionIntegrity(
		ctx,
		transaction,
		head,
	); err != nil {
		return nil, false, err
	}
	if head.Revision().Value() > 0 {
		return AlreadyActive{
			project:       head.Project(),
			revision:      head.Revision(),
			activeTypeEnv: head.ActiveTypeEnv(),
		}, false, nil
	}
	existing, found, err := loadTypeEnvSnapshotWithScanner(
		ctx,
		transaction,
		head.ActiveTypeEnv(),
	)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, fmt.Errorf(
			"revision-zero graph base TypeEnv snapshot is missing",
		)
	}
	presented := coordinate.Snapshot()
	if head.ActiveTypeEnv() != coordinate.BaseTypeEnv() ||
		!equalTypeEnvSnapshots(existing, presented) {
		return Conflict{
			project:   coordinate.Project(),
			existing:  existing,
			presented: presented,
		}, false, nil
	}
	return AlreadyExactAtBase{coordinate: coordinate}, false, nil
}

func canonicalTypeEnvSnapshot(
	snapshot TypeEnvSnapshot,
) (TypeEnvSnapshot, error) {
	return NewTypeEnvSnapshotBuilder(snapshot.Ref()).
		SetFormat(snapshot.Format()).
		SetCanonicalBytes(snapshot.CanonicalBytes()).
		SetSourceRevision(snapshot.SourceRevision()).
		SetCompilerSchemaVersion(snapshot.CompilerSchemaVersion()).
		Build()
}

func loadOptionalProjectGraphHead(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
) (GraphHead, bool, error) {
	head, err := loadHeadWithScanner(ctx, transaction, project)
	if errors.Is(err, ErrProjectNotInitialized) {
		return GraphHead{}, false, nil
	}
	if err != nil {
		return GraphHead{}, false, err
	}
	return head, true, nil
}

func putExactTypeEnvSnapshotTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	snapshot TypeEnvSnapshot,
	recordedAt string,
) error {
	existing, found, err := loadTypeEnvSnapshotWithScanner(
		ctx,
		transaction,
		snapshot.Ref(),
	)
	if err != nil {
		return err
	}
	if found {
		if !equalTypeEnvSnapshots(existing, snapshot) {
			return ErrSnapshotConflict
		}
		return nil
	}
	_, err = transaction.Execute(
		ctx,
		`INSERT INTO typed_memory_type_env_snapshots (
			type_env_ref,
			artifact_digest,
			snapshot_format,
			canonical_bytes,
			source_revision,
			compiler_schema_version,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		[]any{
			snapshot.Ref().String(),
			snapshot.ArtifactDigest().String(),
			snapshot.Format().String(),
			snapshot.CanonicalBytes(),
			snapshot.SourceRevision().String(),
			snapshot.CompilerSchemaVersion().String(),
			recordedAt,
		},
	)
	if err != nil {
		return fmt.Errorf("insert project graph base TypeEnv snapshot: %w", err)
	}
	stored, found, err := loadTypeEnvSnapshotWithScanner(
		ctx,
		transaction,
		snapshot.Ref(),
	)
	if err != nil {
		return err
	}
	if !found || !equalTypeEnvSnapshots(stored, snapshot) {
		return fmt.Errorf(
			"project graph base TypeEnv snapshot exact reread failed",
		)
	}
	return nil
}

func verifyInitializedProjectGraphTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	coordinate projectGraphBaseCoordinate,
) error {
	head, err := loadHeadWithScanner(
		ctx,
		transaction,
		coordinate.Project(),
	)
	if err != nil {
		return err
	}
	if head.Revision().Value() != 0 ||
		head.ActiveTypeEnv() != coordinate.BaseTypeEnv() ||
		head.LastEventRef() != "" ||
		head.LastCommitRef() != "" {
		return fmt.Errorf(
			"revision-zero project graph exact reread failed",
		)
	}
	if err := verifyCurrentHeadClosure(ctx, transaction, head); err != nil {
		return err
	}
	return nil
}

func finishProjectGraphInitializationRead(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
) error {
	finish := transaction.Rollback(ctx)
	if !finish.Succeeded() {
		return fmt.Errorf(
			"finish project graph initialization without writes: %w",
			finish.Err(),
		)
	}
	return nil
}
