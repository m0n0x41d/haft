package typedmemorystore

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestSQLiteProjectGraphInitializerInitializesAndReplaysExactBase(
	t *testing.T,
) {
	fixture := newProjectGraphInitializationFixture(
		t,
		[]byte(`{"schema":"test.graph-initialization.typeenv/v1","base":"a"}`),
		"test-graph-initialization-a",
		"test.graph-initialization.a.v1",
	)
	ctx := context.Background()
	result, err := fixture.initializer.InitializeProjectGraphAtBaseTypeEnv(
		ctx,
		fixture.project,
		fixture.snapshot,
	)
	if err != nil {
		t.Fatalf("InitializeProjectGraphAtBaseTypeEnv(fresh): %v", err)
	}
	initialized, ok := result.(InitializedAtBase)
	if !ok {
		t.Fatalf("fresh result = %T; want InitializedAtBase", result)
	}
	if initialized.GraphRevision().Value() != 0 {
		t.Fatalf(
			"fresh graph revision = %d; want 0",
			initialized.GraphRevision().Value(),
		)
	}
	assertGraphInitializationBaseCoordinate(
		t,
		initialized.Project(),
		initialized.BaseTypeEnv(),
		initialized.Snapshot(),
		fixture,
	)
	assertProjectGraphInitializationRows(t, fixture.database, 1, 1, 0)
	initialSnapshotTime, initialHeadTime := loadProjectGraphInitializationTimes(
		t,
		fixture.database,
		fixture.project,
		fixture.snapshot.Ref(),
	)

	replayed, err := fixture.initializer.InitializeProjectGraphAtBaseTypeEnv(
		ctx,
		fixture.project,
		fixture.snapshot,
	)
	if err != nil {
		t.Fatalf("InitializeProjectGraphAtBaseTypeEnv(replay): %v", err)
	}
	exact, ok := replayed.(AlreadyExactAtBase)
	if !ok {
		t.Fatalf("replay result = %T; want AlreadyExactAtBase", replayed)
	}
	if exact.GraphRevision().Value() != 0 {
		t.Fatalf(
			"replay graph revision = %d; want 0",
			exact.GraphRevision().Value(),
		)
	}
	assertGraphInitializationBaseCoordinate(
		t,
		exact.Project(),
		exact.BaseTypeEnv(),
		exact.Snapshot(),
		fixture,
	)
	replayedSnapshotTime, replayedHeadTime := loadProjectGraphInitializationTimes(
		t,
		fixture.database,
		fixture.project,
		fixture.snapshot.Ref(),
	)
	if replayedSnapshotTime != initialSnapshotTime ||
		replayedHeadTime != initialHeadTime {
		t.Fatal("exact replay changed snapshot or graph-head persistence time")
	}
	assertProjectGraphInitializationRows(t, fixture.database, 1, 1, 0)
}

func TestSQLiteProjectGraphInitializerReturnsConflictWithoutPersistingPresentedBase(
	t *testing.T,
) {
	fixture := newProjectGraphInitializationFixture(
		t,
		[]byte(`{"schema":"test.graph-initialization.typeenv/v1","base":"a"}`),
		"test-graph-initialization-a",
		"test.graph-initialization.a.v1",
	)
	ctx := context.Background()
	if _, err := fixture.initializer.InitializeProjectGraphAtBaseTypeEnv(
		ctx,
		fixture.project,
		fixture.snapshot,
	); err != nil {
		t.Fatalf("seed revision-zero graph: %v", err)
	}
	presented := newProjectGraphInitializationSnapshot(
		t,
		[]byte(`{"schema":"test.graph-initialization.typeenv/v1","base":"b"}`),
		"test-graph-initialization-b",
		"test.graph-initialization.b.v1",
	)
	presentedEnvironment := newLoaderContractEnvironment(
		t,
		presented.Ref(),
		presented.SourceRevision(),
		presented.CompilerSchemaVersion(),
	)
	presentedInitializer := newProjectGraphInitializer(
		t,
		fixture.database,
		presented,
		presentedEnvironment,
		fixture.clock,
	)

	result, err := presentedInitializer.InitializeProjectGraphAtBaseTypeEnv(
		ctx,
		fixture.project,
		presented,
	)
	if err != nil {
		t.Fatalf("InitializeProjectGraphAtBaseTypeEnv(conflict): %v", err)
	}
	conflict, ok := result.(Conflict)
	if !ok {
		t.Fatalf("conflict result = %T; want Conflict", result)
	}
	if conflict.Project() != fixture.project ||
		!equalTypeEnvSnapshots(conflict.ExistingSnapshot(), fixture.snapshot) ||
		!equalTypeEnvSnapshots(conflict.PresentedSnapshot(), presented) {
		t.Fatal("conflict did not retain the exact existing and presented bases")
	}
	assertProjectGraphInitializationRows(t, fixture.database, 1, 1, 0)
	var presentedCount int
	err = fixture.database.QueryRow(
		`SELECT COUNT(*) FROM typed_memory_type_env_snapshots
		 WHERE type_env_ref = ?`,
		presented.Ref().String(),
	).Scan(&presentedCount)
	if err != nil {
		t.Fatalf("count conflicting presented snapshot: %v", err)
	}
	if presentedCount != 0 {
		t.Fatal("conflicting presented base snapshot was persisted")
	}
}

func TestSQLiteProjectGraphInitializerRollsBackSnapshotWhenHeadInsertFails(
	t *testing.T,
) {
	fixture := newProjectGraphInitializationFixture(
		t,
		[]byte(`{"schema":"test.graph-initialization.typeenv/v1","base":"rollback"}`),
		"test-graph-initialization-rollback",
		"test.graph-initialization.rollback.v1",
	)
	_, err := fixture.database.Exec(
		`CREATE TRIGGER reject_project_graph_initialization_head
		 BEFORE INSERT ON typed_memory_graph_heads
		 BEGIN
			SELECT RAISE(ABORT, 'injected graph initialization failure');
		 END`,
	)
	if err != nil {
		t.Fatalf("install graph-initialization failure trigger: %v", err)
	}

	result, err := fixture.initializer.InitializeProjectGraphAtBaseTypeEnv(
		context.Background(),
		fixture.project,
		fixture.snapshot,
	)
	if err == nil {
		t.Fatal("head-insert failure unexpectedly initialized project graph")
	}
	if result != nil {
		t.Fatalf("head-insert failure result = %T; want nil", result)
	}
	assertProjectGraphInitializationRows(t, fixture.database, 0, 0, 0)
}

func TestSQLiteProjectGraphInitializerRequiresExactPersistedProjectBinding(
	t *testing.T,
) {
	fixture := newProjectGraphInitializationFixture(
		t,
		[]byte(`{"schema":"test.graph-initialization.typeenv/v1","base":"binding"}`),
		"test-graph-initialization-binding",
		"test.graph-initialization.binding.v1",
	)
	foreign := mustProjectID(t, "qnt_b8e4c3d2")

	result, err := fixture.initializer.InitializeProjectGraphAtBaseTypeEnv(
		context.Background(),
		foreign,
		fixture.snapshot,
	)
	if err == nil {
		t.Fatal("foreign project unexpectedly initialized project graph")
	}
	if result != nil {
		t.Fatalf("foreign-project result = %T; want nil", result)
	}
	assertProjectGraphInitializationRows(t, fixture.database, 0, 0, 0)
}

func TestSQLiteProjectGraphInitializerLeavesActiveGraphUntouched(
	t *testing.T,
) {
	fixture := newSQLiteStoreFixture(t)
	candidate := fixture.declaration(
		t,
		"active-initialization-check",
		"Active graph initialization check",
	)
	request := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"declare:active-initialization-check",
		candidate,
	)
	if _, err := fixture.adapter.commitDeclareEntity(
		context.Background(),
		request,
	); err != nil {
		t.Fatalf("advance graph before initialization check: %v", err)
	}
	initializer := newProjectGraphInitializer(
		t,
		fixture.database,
		fixture.snapshot,
		fixture.environment,
		fixture.clock,
	)
	before := projectGraphInitializationTableCounts(t, fixture.database)

	result, err := initializer.InitializeProjectGraphAtBaseTypeEnv(
		context.Background(),
		fixture.project,
		fixture.snapshot,
	)
	if err != nil {
		t.Fatalf("InitializeProjectGraphAtBaseTypeEnv(active): %v", err)
	}
	active, ok := result.(AlreadyActive)
	if !ok {
		t.Fatalf("active result = %T; want AlreadyActive", result)
	}
	if active.Project() != fixture.project ||
		active.GraphRevision().Value() != 1 ||
		active.ActiveTypeEnv() != fixture.environment.Ref() {
		t.Fatal("AlreadyActive lost the exact live graph coordinate")
	}
	after := projectGraphInitializationTableCounts(t, fixture.database)
	if before != after {
		t.Fatalf("active graph initialization changed rows: before=%v after=%v", before, after)
	}
}

type projectGraphInitializationFixture struct {
	database    *sql.DB
	project     projectledger.ProjectID
	snapshot    TypeEnvSnapshot
	clock       fixedClock
	initializer ProjectGraphInitializationPort
}

func newProjectGraphInitializationFixture(
	t *testing.T,
	canonical []byte,
	sourceRevision string,
	compilerVersion string,
) projectGraphInitializationFixture {
	t.Helper()
	root := t.TempDir()
	store := openStoreAt(t, filepath.Join(root, "typed-memory.db"))
	database := store.GetRawDB()
	project := mustProjectID(t, "qnt_a7f3b2c1")
	insertProjectBinding(t, database, project, root)
	snapshot := newProjectGraphInitializationSnapshot(
		t,
		canonical,
		sourceRevision,
		compilerVersion,
	)
	environment := newLoaderContractEnvironment(
		t,
		snapshot.Ref(),
		snapshot.SourceRevision(),
		snapshot.CompilerSchemaVersion(),
	)
	clock := fixedClock{
		value: time.Date(2026, 7, 18, 8, 30, 0, 123456789, time.UTC),
	}
	initializer := newProjectGraphInitializer(
		t,
		database,
		snapshot,
		environment,
		clock,
	)
	return projectGraphInitializationFixture{
		database:    database,
		project:     project,
		snapshot:    snapshot,
		clock:       clock,
		initializer: initializer,
	}
}

func newProjectGraphInitializer(
	t *testing.T,
	database *sql.DB,
	snapshot TypeEnvSnapshot,
	environment typedmemory.TypeEnv,
	clock Clock,
) ProjectGraphInitializationPort {
	t.Helper()
	loader := staticTypeEnvLoader{
		reference:   snapshot.Ref(),
		environment: environment,
		registry:    typedmemory.NewCodecRegistry(),
	}
	initializer, err := NewSQLiteProjectGraphInitializer(
		database,
		loader,
		clock,
	)
	if err != nil {
		t.Fatalf("NewSQLiteProjectGraphInitializer: %v", err)
	}
	return initializer
}

func newProjectGraphInitializationSnapshot(
	t *testing.T,
	canonical []byte,
	sourceRevision string,
	compilerVersion string,
) TypeEnvSnapshot {
	t.Helper()
	reference := mustTypeEnvRef(t, canonical)
	format, err := NewSnapshotFormat(BaseTypeEnvSnapshotFormat)
	if err != nil {
		t.Fatalf("NewSnapshotFormat: %v", err)
	}
	snapshot, err := NewTypeEnvSnapshotBuilder(reference).
		SetFormat(format).
		SetCanonicalBytes(canonical).
		SetSourceRevision(mustSourceRevision(t, sourceRevision)).
		SetCompilerSchemaVersion(mustCompilerVersion(t, compilerVersion)).
		Build()
	if err != nil {
		t.Fatalf("build graph-initialization snapshot: %v", err)
	}
	return snapshot
}

func assertGraphInitializationBaseCoordinate(
	t *testing.T,
	project projectledger.ProjectID,
	base typedmemory.TypeEnvRef,
	snapshot TypeEnvSnapshot,
	fixture projectGraphInitializationFixture,
) {
	t.Helper()
	if project != fixture.project ||
		base != fixture.snapshot.Ref() ||
		snapshot.Ref() != fixture.snapshot.Ref() ||
		!equalTypeEnvSnapshots(snapshot, fixture.snapshot) {
		t.Fatal("graph-initialization result lost the exact base coordinate")
	}
}

func assertProjectGraphInitializationRows(
	t *testing.T,
	database *sql.DB,
	snapshotCount int64,
	headCount int64,
	eventCount int64,
) {
	t.Helper()
	assertTypedMemoryRowCounts(t, database, map[string]int64{
		"typed_memory_type_env_snapshots":  snapshotCount,
		"typed_memory_graph_heads":         headCount,
		"typed_memory_graph_events":        eventCount,
		"typed_memory_graph_commits":       0,
		"typed_memory_idempotency_history": 0,
		"typed_memory_projection_jobs":     0,
	})
}

func loadProjectGraphInitializationTimes(
	t *testing.T,
	database *sql.DB,
	project projectledger.ProjectID,
	snapshot typedmemory.TypeEnvRef,
) (string, string) {
	t.Helper()
	var snapshotTime string
	var headTime string
	err := database.QueryRow(
		`SELECT snapshot.recorded_at, head.updated_at
		 FROM typed_memory_type_env_snapshots snapshot
		 JOIN typed_memory_graph_heads head
			ON head.active_type_env_ref = snapshot.type_env_ref
		 WHERE head.project_id = ? AND snapshot.type_env_ref = ?`,
		project.String(),
		snapshot.String(),
	).Scan(&snapshotTime, &headTime)
	if err != nil {
		t.Fatalf("load graph-initialization persistence times: %v", err)
	}
	return snapshotTime, headTime
}

type projectGraphInitializationCounts struct {
	snapshots int64
	heads     int64
	events    int64
	commits   int64
}

func projectGraphInitializationTableCounts(
	t *testing.T,
	database *sql.DB,
) projectGraphInitializationCounts {
	t.Helper()
	counts := projectGraphInitializationCounts{}
	queries := []struct {
		table string
		value *int64
	}{
		{table: "typed_memory_type_env_snapshots", value: &counts.snapshots},
		{table: "typed_memory_graph_heads", value: &counts.heads},
		{table: "typed_memory_graph_events", value: &counts.events},
		{table: "typed_memory_graph_commits", value: &counts.commits},
	}
	for _, query := range queries {
		err := database.QueryRow(
			"SELECT COUNT(*) FROM " + query.table,
		).Scan(query.value)
		if err != nil {
			t.Fatalf("count %s: %v", query.table, err)
		}
	}
	return counts
}

func TestSQLiteProjectGraphInitializerRejectsInvalidSnapshotBeforeWrites(
	t *testing.T,
) {
	fixture := newProjectGraphInitializationFixture(
		t,
		[]byte(`{"schema":"test.graph-initialization.typeenv/v1","base":"invalid"}`),
		"test-graph-initialization-invalid",
		"test.graph-initialization.invalid.v1",
	)
	invalid := fixture.snapshot
	invalid.canonicalBytes = []byte(`{"changed":true}`)

	result, err := fixture.initializer.InitializeProjectGraphAtBaseTypeEnv(
		context.Background(),
		fixture.project,
		invalid,
	)
	if err == nil {
		t.Fatal("invalid snapshot unexpectedly initialized project graph")
	}
	if result != nil {
		t.Fatalf("invalid snapshot result = %T; want nil", result)
	}
	assertProjectGraphInitializationRows(t, fixture.database, 0, 0, 0)
}

func TestSQLiteProjectGraphInitializerRejectsCorruptPersistedBinding(
	t *testing.T,
) {
	fixture := newProjectGraphInitializationFixture(
		t,
		[]byte(`{"schema":"test.graph-initialization.typeenv/v1","base":"corrupt-binding"}`),
		"test-graph-initialization-corrupt-binding",
		"test.graph-initialization.corrupt-binding.v1",
	)
	if _, err := fixture.database.Exec(
		"DROP TRIGGER project_ledger_binding_no_update",
	); err != nil {
		t.Fatalf("allow project-binding corruption fixture: %v", err)
	}
	if _, err := fixture.database.Exec(
		`UPDATE project_ledger_binding
		 SET binding_digest = ?`,
		"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	); err != nil {
		t.Fatalf("corrupt persisted project binding: %v", err)
	}

	result, err := fixture.initializer.InitializeProjectGraphAtBaseTypeEnv(
		context.Background(),
		fixture.project,
		fixture.snapshot,
	)
	if err == nil {
		t.Fatal("corrupt persisted binding unexpectedly initialized project graph")
	}
	if result != nil {
		t.Fatalf("corrupt-binding result = %T; want nil", result)
	}
	assertProjectGraphInitializationRows(t, fixture.database, 0, 0, 0)
}

func TestSQLiteProjectGraphInitializerConcurrentExactCallsConverge(
	t *testing.T,
) {
	fixture := newProjectGraphInitializationFixture(
		t,
		[]byte(`{"schema":"test.graph-initialization.typeenv/v1","base":"concurrent"}`),
		"test-graph-initialization-concurrent",
		"test.graph-initialization.concurrent.v1",
	)
	type outcome struct {
		result ProjectGraphInitializationResult
		err    error
	}
	start := make(chan struct{})
	results := make(chan outcome, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	for range 2 {
		go func() {
			defer workers.Done()
			<-start
			result, err := fixture.initializer.InitializeProjectGraphAtBaseTypeEnv(
				context.Background(),
				fixture.project,
				fixture.snapshot,
			)
			results <- outcome{result: result, err: err}
		}()
	}
	close(start)
	workers.Wait()
	close(results)

	initialized := 0
	alreadyExact := 0
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent initialization: %v", result.err)
		}
		switch result.result.(type) {
		case InitializedAtBase:
			initialized++
		case AlreadyExactAtBase:
			alreadyExact++
		default:
			t.Fatalf(
				"concurrent initialization result = %T",
				result.result,
			)
		}
	}
	if initialized != 1 || alreadyExact != 1 {
		t.Fatalf(
			"concurrent outcomes initialized=%d already-exact=%d; want 1/1",
			initialized,
			alreadyExact,
		)
	}
	assertProjectGraphInitializationRows(t, fixture.database, 1, 1, 0)
}

func TestNewSQLiteProjectGraphInitializerRequiresDependencies(t *testing.T) {
	snapshot := newProjectGraphInitializationSnapshot(
		t,
		[]byte(`{"schema":"test.graph-initialization.typeenv/v1","base":"dependencies"}`),
		"test-graph-initialization-dependencies",
		"test.graph-initialization.dependencies.v1",
	)
	environment := newLoaderContractEnvironment(
		t,
		snapshot.Ref(),
		snapshot.SourceRevision(),
		snapshot.CompilerSchemaVersion(),
	)
	loader := staticTypeEnvLoader{
		reference:   snapshot.Ref(),
		environment: environment,
		registry:    typedmemory.NewCodecRegistry(),
	}
	clock := fixedClock{value: time.Now().UTC()}
	tests := []struct {
		name     string
		database *sql.DB
		loader   TypeEnvLoader
		clock    Clock
		want     error
	}{
		{name: "database", loader: loader, clock: clock, want: ErrDatabaseRequired},
		{name: "loader", database: &sql.DB{}, clock: clock, want: ErrTypeEnvLoaderRequired},
		{name: "clock", database: &sql.DB{}, loader: loader, want: ErrClockRequired},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := NewSQLiteProjectGraphInitializer(
				test.database,
				test.loader,
				test.clock,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("constructor error = %v; want %v", err, test.want)
			}
			if result != nil {
				t.Fatalf("constructor result = %T; want nil", result)
			}
		})
	}
}
