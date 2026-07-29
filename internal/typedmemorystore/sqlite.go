package typedmemorystore

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type SQLiteAdapter struct {
	database                  *sql.DB
	loader                    TypeEnvLoader
	selectedProjectRuntime    SelectedProjectTypeEnvRuntimeResolver
	clock                     Clock
	memberOfEngine            MemberOfEvaluationEngine
	kindClassificationEngine  KindClassificationAdmissionEngine
	kindClassificationSources KindClassificationSourceProvider
	referenceEngine           StrongReferenceResolutionEngine
	observableInputs          ObservableInputContentProvider
	finisher                  transactionFinisher
}

type transactionFinishEvidence interface {
	Succeeded() bool
	Err() error
}

type transactionFinisher interface {
	Commit(
		context.Context,
		*sqlitetransaction.Transaction,
	) transactionFinishEvidence
}

type defaultTransactionFinisher struct{}

func (defaultTransactionFinisher) Commit(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
) transactionFinishEvidence {
	return transaction.Commit(ctx)
}

func newDeclarationSQLiteAdapter(
	database *sql.DB,
	loader TypeEnvLoader,
	clock Clock,
) (*SQLiteAdapter, error) {
	if database == nil {
		return nil, ErrDatabaseRequired
	}
	if loader == nil {
		return nil, ErrTypeEnvLoaderRequired
	}
	if clock == nil {
		return nil, ErrClockRequired
	}
	return &SQLiteAdapter{
		database: database,
		loader:   loader,
		clock:    clock,
		finisher: defaultTransactionFinisher{},
	}, nil
}

// NewSQLiteSnapshotPort exposes only immutable TypeEnv snapshot persistence
// and loading. It cannot initialize a graph, admit memory, or select a project
// TypeEnv head.
func NewSQLiteSnapshotPort(
	database *sql.DB,
	loader TypeEnvLoader,
	clock Clock,
) (SnapshotPort, error) {
	return newDeclarationSQLiteAdapter(database, loader, clock)
}

// NewGenericSQLiteAdapter constructs the v46 generic admission adapter. The
// legacy constructor intentionally leaves the store-owned MemberOf evaluator
// absent and remains usable only through the bounded declaration compatibility
// surface.
func NewGenericSQLiteAdapter(
	database *sql.DB,
	loader TypeEnvLoader,
	clock Clock,
	memberOfEngine MemberOfEvaluationEngine,
	referenceEngine StrongReferenceResolutionEngine,
	observableInputs ObservableInputContentProvider,
) (*SQLiteAdapter, error) {
	if memberOfEngine == nil {
		return nil, fmt.Errorf("generic typed-memory adapter requires a store-owned MemberOf evaluator")
	}
	if observableInputs == nil {
		return nil, fmt.Errorf("generic typed-memory adapter requires an immutable observable-input provider")
	}
	if referenceEngine == nil {
		return nil, fmt.Errorf("generic typed-memory adapter requires a store-owned strong-reference resolver")
	}
	adapter, err := newDeclarationSQLiteAdapter(database, loader, clock)
	if err != nil {
		return nil, err
	}
	adapter.memberOfEngine = memberOfEngine
	adapter.referenceEngine = referenceEngine
	adapter.observableInputs = observableInputs
	return adapter, nil
}

// NewProjectAwareGenericSQLiteAdapter adds the exact project-executable
// TypeEnv branch while retaining the legacy generic-snapshot branch. The
// resolver is consulted only when the transaction-local coordinate owner is
// project_executable; absence or failure never falls back to generic storage.
func NewProjectAwareGenericSQLiteAdapter(
	database *sql.DB,
	loader TypeEnvLoader,
	clock Clock,
	memberOfEngine MemberOfEvaluationEngine,
	referenceEngine StrongReferenceResolutionEngine,
	observableInputs ObservableInputContentProvider,
	selectedProjectRuntime SelectedProjectTypeEnvRuntimeResolver,
) (*SQLiteAdapter, error) {
	if !selectedProjectTypeEnvRuntimeResolverIsPresent(
		selectedProjectRuntime,
	) {
		return nil, ErrSelectedProjectTypeEnvRuntimeResolverRequired
	}
	adapter, err := NewGenericSQLiteAdapter(
		database,
		loader,
		clock,
		memberOfEngine,
		referenceEngine,
		observableInputs,
	)
	if err != nil {
		return nil, err
	}
	adapter.selectedProjectRuntime = selectedProjectRuntime
	return adapter, nil
}

// ProjectExecutableGenericSQLiteAdapterBuilder constructs the admission shell
// for a project whose active TypeEnv coordinate is owned by the dedicated
// project-executable runtime. MemberOf is recovered from that exact selected
// C/X closure inside the caller-owned transaction, so this builder exposes no
// process-global membership-evaluator slot.
type ProjectExecutableGenericSQLiteAdapterBuilder struct {
	database                  *sql.DB
	loader                    TypeEnvLoader
	clock                     Clock
	referenceEngine           StrongReferenceResolutionEngine
	observableInputs          ObservableInputContentProvider
	kindClassificationSources KindClassificationSourceProvider
	selectedProjectRuntime    SelectedProjectTypeEnvRuntimeResolver
}

func NewProjectExecutableGenericSQLiteAdapterBuilder(
	database *sql.DB,
) ProjectExecutableGenericSQLiteAdapterBuilder {
	return ProjectExecutableGenericSQLiteAdapterBuilder{database: database}
}

func (builder ProjectExecutableGenericSQLiteAdapterBuilder) SetTypeEnvLoader(
	loader TypeEnvLoader,
) ProjectExecutableGenericSQLiteAdapterBuilder {
	builder.loader = loader
	return builder
}

func (builder ProjectExecutableGenericSQLiteAdapterBuilder) SetClock(
	clock Clock,
) ProjectExecutableGenericSQLiteAdapterBuilder {
	builder.clock = clock
	return builder
}

func (builder ProjectExecutableGenericSQLiteAdapterBuilder) SetReferenceEngine(
	engine StrongReferenceResolutionEngine,
) ProjectExecutableGenericSQLiteAdapterBuilder {
	builder.referenceEngine = engine
	return builder
}

func (builder ProjectExecutableGenericSQLiteAdapterBuilder) SetObservableInputs(
	inputs ObservableInputContentProvider,
) ProjectExecutableGenericSQLiteAdapterBuilder {
	builder.observableInputs = inputs
	return builder
}

func (builder ProjectExecutableGenericSQLiteAdapterBuilder) SetKindClassificationSources(
	sources KindClassificationSourceProvider,
) ProjectExecutableGenericSQLiteAdapterBuilder {
	builder.kindClassificationSources = sources
	return builder
}

func (builder ProjectExecutableGenericSQLiteAdapterBuilder) SetSelectedProjectRuntime(
	resolver SelectedProjectTypeEnvRuntimeResolver,
) ProjectExecutableGenericSQLiteAdapterBuilder {
	builder.selectedProjectRuntime = resolver
	return builder
}

// Build returns a project-executable adapter with no static MemberOf
// evaluator. A generic-snapshot read or membership-free compatibility
// admission remains possible, while any generic admission that actually
// requires MemberOf fails closed.
func (builder ProjectExecutableGenericSQLiteAdapterBuilder) Build() (
	*SQLiteAdapter,
	error,
) {
	if !selectedProjectTypeEnvRuntimeResolverIsPresent(
		builder.selectedProjectRuntime,
	) {
		return nil, ErrSelectedProjectTypeEnvRuntimeResolverRequired
	}
	if builder.referenceEngine == nil {
		return nil, fmt.Errorf(
			"project-executable typed-memory adapter requires a store-owned strong-reference resolver",
		)
	}
	if builder.observableInputs == nil {
		return nil, fmt.Errorf(
			"project-executable typed-memory adapter requires an immutable observable-input provider",
		)
	}
	adapter, err := newDeclarationSQLiteAdapter(
		builder.database,
		builder.loader,
		builder.clock,
	)
	if err != nil {
		return nil, err
	}
	adapter.selectedProjectRuntime = builder.selectedProjectRuntime
	adapter.referenceEngine = builder.referenceEngine
	adapter.observableInputs = builder.observableInputs
	adapter.kindClassificationSources = builder.kindClassificationSources
	return adapter, nil
}

func (adapter *SQLiteAdapter) PutTypeEnvSnapshot(
	ctx context.Context,
	snapshot TypeEnvSnapshot,
) error {
	if ctx == nil {
		return fmt.Errorf("store TypeEnv snapshot: context is required")
	}
	verified, err := NewTypeEnvSnapshotBuilder(snapshot.Ref()).
		SetFormat(snapshot.Format()).
		SetCanonicalBytes(snapshot.CanonicalBytes()).
		SetSourceRevision(snapshot.SourceRevision()).
		SetCompilerSchemaVersion(snapshot.CompilerSchemaVersion()).
		Build()
	if err != nil {
		return fmt.Errorf("store TypeEnv snapshot: %w", err)
	}
	environment, _, err := adapter.loader.LoadTypeEnv(verified)
	if err != nil {
		return fmt.Errorf("store executable TypeEnv snapshot: %w", err)
	}
	if !loadedEnvironmentMatchesSnapshot(environment, verified) {
		return fmt.Errorf("store executable TypeEnv snapshot: loaded environment does not match immutable snapshot metadata")
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, adapter.database)
	if err != nil {
		return fmt.Errorf("begin TypeEnv snapshot transaction: %w", err)
	}
	existing, found, err := loadTypeEnvSnapshotWithScanner(
		ctx,
		transaction,
		verified.Ref(),
	)
	if err != nil {
		return rollbackError(ctx, transaction, err)
	}
	if found {
		if !equalTypeEnvSnapshots(existing, verified) {
			return rollbackError(ctx, transaction, ErrSnapshotConflict)
		}
		return rollbackSuccess(ctx, transaction)
	}
	recordedAt := canonicalTime(adapter.clock.Now())
	_, err = transaction.Execute(
		ctx,
		`INSERT INTO typed_memory_type_env_snapshots (
			type_env_ref, artifact_digest, snapshot_format, canonical_bytes,
			source_revision, compiler_schema_version, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		[]any{
			verified.Ref().String(),
			verified.ArtifactDigest().String(),
			verified.Format().String(),
			verified.CanonicalBytes(),
			verified.SourceRevision().String(),
			verified.CompilerSchemaVersion().String(),
			recordedAt,
		},
	)
	if err != nil {
		return rollbackError(ctx, transaction, fmt.Errorf("insert TypeEnv snapshot: %w", err))
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		return fmt.Errorf("commit TypeEnv snapshot: %w", finish.Err())
	}
	return nil
}

func (adapter *SQLiteAdapter) LoadTypeEnvSnapshot(
	ctx context.Context,
	ref typedmemory.TypeEnvRef,
) (TypeEnvSnapshot, error) {
	if ctx == nil {
		return TypeEnvSnapshot{}, fmt.Errorf("load TypeEnv snapshot: context is required")
	}
	snapshot, found, err := loadTypeEnvSnapshotWithScanner(
		ctx,
		databaseScanner{database: adapter.database},
		ref,
	)
	if err != nil {
		return TypeEnvSnapshot{}, err
	}
	if !found {
		return TypeEnvSnapshot{}, sql.ErrNoRows
	}
	return snapshot, nil
}

func (adapter *SQLiteAdapter) LoadHead(
	ctx context.Context,
	project projectledger.ProjectID,
) (GraphHead, error) {
	if ctx == nil {
		return GraphHead{}, fmt.Errorf("load typed-memory graph head: context is required")
	}
	return loadHeadWithScanner(ctx, databaseScanner{database: adapter.database}, project)
}

func (adapter *SQLiteAdapter) LoadEntity(
	ctx context.Context,
	project projectledger.ProjectID,
	entity typedmemory.EntityID,
) (StoredEntity, error) {
	if ctx == nil {
		return StoredEntity{}, fmt.Errorf("load typed-memory entity: context is required")
	}
	var contextText string
	var labelText string
	var provenanceText string
	var eventRef string
	var revision int64
	err := adapter.database.QueryRowContext(
		ctx,
		`SELECT context.bounded_context_ref, context.label, context.provenance_ref,
			context.declared_event_ref, context.declared_revision
		FROM typed_memory_entities entity
		JOIN typed_memory_entity_contexts context
			ON context.project_id = entity.project_id AND context.entity_id = entity.entity_id
		JOIN typed_memory_graph_commits commit_record
			ON commit_record.project_id = context.project_id
			AND commit_record.event_ref = context.declared_event_ref
		WHERE entity.project_id = ? AND entity.entity_id = ?
		ORDER BY context.declared_revision, context.bounded_context_ref
		LIMIT 1`,
		project.String(),
		entity.String(),
	).Scan(&contextText, &labelText, &provenanceText, &eventRef, &revision)
	if err != nil {
		return StoredEntity{}, fmt.Errorf("load typed-memory entity: %w", err)
	}
	contextRef, err := typedmemory.NewBoundedContextRef(contextText)
	if err != nil {
		return StoredEntity{}, err
	}
	label, err := typedmemory.NewEntityLabel(labelText)
	if err != nil {
		return StoredEntity{}, err
	}
	provenance, err := typedmemory.NewProvenanceRef(provenanceText)
	if err != nil {
		return StoredEntity{}, err
	}
	graphRevision, err := graphRevisionFromSQLite(revision)
	if err != nil {
		return StoredEntity{}, err
	}
	return StoredEntity{
		project:    project,
		entity:     entity,
		context:    contextRef,
		label:      label,
		provenance: provenance,
		eventRef:   eventRef,
		revision:   graphRevision,
	}, nil
}

func (adapter *SQLiteAdapter) LoadEntityContext(
	ctx context.Context,
	project projectledger.ProjectID,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) (StoredEntity, error) {
	if ctx == nil {
		return StoredEntity{}, fmt.Errorf(
			"load typed-memory entity context: context is required",
		)
	}
	if adapter == nil || adapter.database == nil {
		return StoredEntity{}, ErrDatabaseRequired
	}
	var labelText string
	var provenanceText string
	var eventRef string
	var revision int64
	err := adapter.database.QueryRowContext(
		ctx,
		`SELECT context.label, context.provenance_ref,
			context.declared_event_ref, context.declared_revision
		FROM typed_memory_entity_contexts context
		JOIN typed_memory_graph_commits commit_record
			ON commit_record.project_id = context.project_id
			AND commit_record.event_ref = context.declared_event_ref
		WHERE context.project_id = ?
			AND context.entity_id = ?
			AND context.bounded_context_ref = ?`,
		project.String(),
		entity.String(),
		contextRef.String(),
	).Scan(&labelText, &provenanceText, &eventRef, &revision)
	if err != nil {
		return StoredEntity{}, fmt.Errorf(
			"load typed-memory entity context: %w",
			err,
		)
	}
	label, err := typedmemory.NewEntityLabel(labelText)
	if err != nil {
		return StoredEntity{}, err
	}
	provenance, err := typedmemory.NewProvenanceRef(provenanceText)
	if err != nil {
		return StoredEntity{}, err
	}
	graphRevision, err := graphRevisionFromSQLite(revision)
	if err != nil {
		return StoredEntity{}, err
	}
	return StoredEntity{
		project:    project,
		entity:     entity,
		context:    contextRef,
		label:      label,
		provenance: provenance,
		eventRef:   eventRef,
		revision:   graphRevision,
	}, nil
}

type scanner interface {
	ScanOne(context.Context, string, []any, []any) error
}

type databaseScanner struct {
	database *sql.DB
}

func (scanner databaseScanner) ScanOne(
	ctx context.Context,
	statement string,
	arguments []any,
	destinations []any,
) error {
	return scanner.database.QueryRowContext(ctx, statement, arguments...).Scan(destinations...)
}

func loadTypeEnvSnapshotWithScanner(
	ctx context.Context,
	scanner scanner,
	ref typedmemory.TypeEnvRef,
) (TypeEnvSnapshot, bool, error) {
	var artifactDigest string
	var formatText string
	var canonicalBytes []byte
	var sourceRevisionText string
	var compilerVersionText string
	err := scanner.ScanOne(
		ctx,
		`SELECT artifact_digest, snapshot_format, canonical_bytes,
			source_revision, compiler_schema_version
		FROM typed_memory_type_env_snapshots WHERE type_env_ref = ?`,
		[]any{ref.String()},
		[]any{
			&artifactDigest,
			&formatText,
			&canonicalBytes,
			&sourceRevisionText,
			&compilerVersionText,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return TypeEnvSnapshot{}, false, nil
	}
	if err != nil {
		return TypeEnvSnapshot{}, false, fmt.Errorf("load TypeEnv snapshot: %w", err)
	}
	format, err := NewSnapshotFormat(formatText)
	if err != nil {
		return TypeEnvSnapshot{}, false, err
	}
	sourceRevision, err := typedmemory.NewSourceRevision(sourceRevisionText)
	if err != nil {
		return TypeEnvSnapshot{}, false, err
	}
	compilerVersion, err := typedmemory.NewCompilerSchemaVersion(compilerVersionText)
	if err != nil {
		return TypeEnvSnapshot{}, false, err
	}
	snapshot, err := NewTypeEnvSnapshotBuilder(ref).
		SetFormat(format).
		SetCanonicalBytes(canonicalBytes).
		SetSourceRevision(sourceRevision).
		SetCompilerSchemaVersion(compilerVersion).
		Build()
	if err != nil {
		return TypeEnvSnapshot{}, false, fmt.Errorf("verify stored TypeEnv snapshot: %w", err)
	}
	if snapshot.ArtifactDigest().String() != artifactDigest {
		return TypeEnvSnapshot{}, false, fmt.Errorf("stored TypeEnv artifact digest does not match canonical bytes")
	}
	return snapshot, true, nil
}

func loadHeadWithScanner(
	ctx context.Context,
	scanner scanner,
	project projectledger.ProjectID,
) (GraphHead, error) {
	var revision int64
	var typeEnvText string
	var lastEventRef string
	var lastCommitRef string
	err := scanner.ScanOne(
		ctx,
		`SELECT graph_revision, active_type_env_ref, last_event_ref, last_commit_ref
		FROM typed_memory_graph_heads WHERE project_id = ?`,
		[]any{project.String()},
		[]any{&revision, &typeEnvText, &lastEventRef, &lastCommitRef},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return GraphHead{}, ErrProjectNotInitialized
	}
	if err != nil {
		return GraphHead{}, fmt.Errorf("load typed-memory graph head: %w", err)
	}
	graphRevision, err := graphRevisionFromSQLite(revision)
	if err != nil {
		return GraphHead{}, err
	}
	typeEnvRef, err := parseTypeEnvRef(typeEnvText)
	if err != nil {
		return GraphHead{}, err
	}
	return GraphHead{
		project:       project,
		revision:      graphRevision,
		activeTypeEnv: typeEnvRef,
		lastEventRef:  lastEventRef,
		lastCommitRef: lastCommitRef,
	}, nil
}

type statement struct {
	query     string
	arguments []any
}

func executeStatements(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	statements []statement,
	index int,
) error {
	if index >= len(statements) {
		return nil
	}
	current := statements[index]
	_, err := transaction.Execute(ctx, current.query, current.arguments)
	if err != nil {
		return fmt.Errorf("persist typed-memory statement %d: %w", index+1, err)
	}
	return executeStatements(ctx, transaction, statements, index+1)
}

func rollbackError(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	cause error,
) error {
	finish := transaction.Rollback(ctx)
	return errors.Join(cause, finish.Err())
}

func rollbackSuccess(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
) error {
	finish := transaction.Rollback(ctx)
	if !finish.Succeeded() {
		return fmt.Errorf("close read-only typed-memory transaction: %w", finish.Err())
	}
	return nil
}

func equalTypeEnvSnapshots(left TypeEnvSnapshot, right TypeEnvSnapshot) bool {
	return left.Ref() == right.Ref() &&
		left.ArtifactDigest() == right.ArtifactDigest() &&
		left.Format() == right.Format() &&
		left.SourceRevision() == right.SourceRevision() &&
		left.CompilerSchemaVersion() == right.CompilerSchemaVersion() &&
		bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes())
}

func loadedEnvironmentMatchesSnapshot(
	environment typedmemory.TypeEnv,
	snapshot TypeEnvSnapshot,
) bool {
	return environment.Ref() == snapshot.Ref() &&
		environment.SourceRevision() == snapshot.SourceRevision() &&
		environment.CompilerSchemaVersion() == snapshot.CompilerSchemaVersion()
}

func canonicalTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func graphRevisionFromSQLite(value int64) (typedmemory.GraphRevision, error) {
	if value < 0 {
		return typedmemory.GraphRevision{}, fmt.Errorf("stored graph revision is negative")
	}
	return typedmemory.NewGraphRevision(uint64(value)), nil
}

func parseTypeEnvRef(raw string) (typedmemory.TypeEnvRef, error) {
	if !strings.HasPrefix(raw, "typeenv:sha256:") {
		return typedmemory.TypeEnvRef{}, fmt.Errorf("stored TypeEnv ref is malformed")
	}
	digest, err := typedmemory.NewSHA256Digest(strings.TrimPrefix(raw, "typeenv:"))
	if err != nil {
		return typedmemory.TypeEnvRef{}, err
	}
	return typedmemory.NewTypeEnvRef(digest)
}

func digestFields(domain string, fields ...string) (typedmemory.SHA256Digest, error) {
	buffer := make([]byte, 0)
	appendField := func(value string) {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		buffer = append(buffer, length[:]...)
		buffer = append(buffer, value...)
	}
	appendField("haft.typedmemorystore.digest.v1")
	appendField(domain)
	for _, field := range fields {
		appendField(field)
	}
	return digestBytes(buffer)
}

func derivedRef(domain string, fields ...string) string {
	digest, _ := digestFields(domain, fields...)
	hexDigest := strings.TrimPrefix(digest.String(), "sha256:")
	return domain + ":" + hexDigest
}

const mathMaxSQLiteRevision = ^uint64(0) >> 1
