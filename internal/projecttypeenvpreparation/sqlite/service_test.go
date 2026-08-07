package sqlite

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projecttypeenvpreparation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	profilebasissqlite "github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis/sqlite"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	_ "modernc.org/sqlite"
)

type preparationClock struct {
	value time.Time
}

func (clock preparationClock) Now() time.Time {
	return clock.value
}

type preparationTypeEnvLoader struct {
	reference   typedmemory.TypeEnvRef
	environment typedmemory.TypeEnv
}

func (loader preparationTypeEnvLoader) LoadTypeEnv(
	snapshot typedmemorystore.TypeEnvSnapshot,
) (typedmemory.TypeEnv, typedmemory.CodecRegistry, error) {
	if snapshot.Ref() != loader.reference {
		return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, fmt.Errorf(
			"unexpected preparation test TypeEnv snapshot %q",
			snapshot.Ref().String(),
		)
	}
	return loader.environment, typedmemory.NewCodecRegistry(), nil
}

type preparationFixture struct {
	harness *profileadmissionfixture.Harness
	ledger  *projectledger.Handle
	service *Service
	base    typeenv.BaseTypeEnvArtifact
}

func TestPreparationServicePersistsExactGenesisCandidateAndReplaysWithoutWrites(
	t *testing.T,
) {
	fixture := newPreparationFixture(t)
	ctx := context.Background()

	result, err := fixture.service.PrepareAtBase(ctx, fixture.base)
	if err != nil {
		t.Fatalf("PrepareAtBase(fresh): %v", err)
	}
	fresh, ok := result.(PreparedAtNewBase)
	if !ok {
		t.Fatalf("PrepareAtBase(fresh) = %T, want PreparedAtNewBase", result)
	}
	candidate := fresh.Candidate()
	if err := candidate.Verify(); err != nil {
		t.Fatalf("fresh Genesis candidate Verify(): %v", err)
	}
	assertPreparationCandidateAtExactBase(t, fixture, candidate)
	assertPreparationArtifactFootprint(t, fixture.harness.Database(), candidate)
	assertPreparationHasNoBindingEffects(t, fixture.harness.Database())
	beforeReplay := preparationRelevantRowCounts(t, fixture.harness.Database())

	replayedResult, err := fixture.service.PrepareAtBase(ctx, fixture.base)
	if err != nil {
		t.Fatalf("PrepareAtBase(replay): %v", err)
	}
	replayed, ok := replayedResult.(PreparedAtExistingExactBase)
	if !ok {
		t.Fatalf(
			"PrepareAtBase(replay) = %T, want PreparedAtExistingExactBase",
			replayedResult,
		)
	}
	assertSamePreparationCandidate(t, candidate, replayed.Candidate())
	afterReplay := preparationRelevantRowCounts(t, fixture.harness.Database())
	if !reflect.DeepEqual(afterReplay, beforeReplay) {
		t.Fatalf(
			"exact preparation replay changed rows:\nbefore=%v\nafter=%v",
			beforeReplay,
			afterReplay,
		)
	}
	assertPreparationHasNoBindingEffects(t, fixture.harness.Database())
}

func TestPreparationServiceBindsDeclaredSoftwareProfileIntoGenesisStage(
	t *testing.T,
) {
	root := canonicalPreparationTempDir(t)
	harness := profileadmissionfixture.New(t, root)
	admission := harness.AdmitSoftwareRevision(t, "genesis-profile")
	fixture := openPreparationFixture(t, harness)

	result, err := fixture.service.PrepareAtBase(
		context.Background(),
		fixture.base,
	)
	if err != nil {
		t.Fatalf("PrepareAtBase(declared profile): %v", err)
	}
	prepared, ok := result.(PreparedAtNewBase)
	if !ok {
		t.Fatalf(
			"PrepareAtBase(declared profile) = %T, want PreparedAtNewBase",
			result,
		)
	}
	profile, err := profilebasissqlite.FromCanonicalAdmission(admission)
	if err != nil {
		t.Fatalf("FromCanonicalAdmission(): %v", err)
	}
	stage := prepared.Candidate().Stage()
	if stage.ProfileLedgerRevision() != profile.LedgerRevision() ||
		stage.ProfileLedgerDigest() != profile.ProfileLedgerDigest() {
		t.Fatal("Genesis Stage did not retain the exact declared profile basis")
	}
	if err := prepared.Candidate().Verify(); err != nil {
		t.Fatalf("declared-profile candidate Verify(): %v", err)
	}
	assertPreparationHasNoBindingEffects(t, harness.Database())
}

func TestPreparationServiceReturnsBaseConflictWithoutCandidatePersistence(
	t *testing.T,
) {
	fixture := newPreparationFixture(t)
	ctx := context.Background()
	existing := newConflictingPreparationSnapshot(t)
	environment := newPreparationTypeEnv(t, existing)
	initializer, err := typedmemorystore.NewSQLiteProjectGraphInitializer(
		fixture.harness.Database(),
		preparationTypeEnvLoader{
			reference:   existing.Ref(),
			environment: environment,
		},
		preparationTestClock(),
	)
	if err != nil {
		t.Fatalf("NewSQLiteProjectGraphInitializer(conflict seed): %v", err)
	}
	seeded, err := initializer.InitializeProjectGraphAtBaseTypeEnv(
		ctx,
		fixture.ledger.ProjectID(),
		existing,
	)
	if err != nil {
		t.Fatalf("initialize conflicting graph base: %v", err)
	}
	if _, ok := seeded.(typedmemorystore.InitializedAtBase); !ok {
		t.Fatalf("conflict seed result = %T, want InitializedAtBase", seeded)
	}
	before := preparationRelevantRowCounts(t, fixture.harness.Database())

	result, err := fixture.service.PrepareAtBase(ctx, fixture.base)
	if err != nil {
		t.Fatalf("PrepareAtBase(conflict): %v", err)
	}
	conflict, ok := result.(BaseSnapshotConflict)
	if !ok {
		t.Fatalf("PrepareAtBase(conflict) = %T, want BaseSnapshotConflict", result)
	}
	presented, err := projectmemory.NewBaseTypeEnvSnapshot(fixture.base)
	if err != nil {
		t.Fatalf("NewBaseTypeEnvSnapshot(): %v", err)
	}
	observation := conflict.Observation()
	if observation.ExistingSnapshot().Ref() != existing.Ref() ||
		observation.PresentedSnapshot().Ref() != presented.Ref() {
		t.Fatal("base conflict lost its exact existing or presented coordinate")
	}
	after := preparationRelevantRowCounts(t, fixture.harness.Database())
	if !reflect.DeepEqual(after, before) {
		t.Fatalf(
			"base conflict changed rows:\nbefore=%v\nafter=%v",
			before,
			after,
		)
	}
	assertPreparationCandidateRows(t, fixture.harness.Database(), 0)
	assertPreparationHasNoBindingEffects(t, fixture.harness.Database())
}

func TestPreparationServiceRejectsStorageFootprintDriftWithoutRepairOrGraphWrite(
	t *testing.T,
) {
	fixture := newPreparationFixture(t)
	const missingTrigger = "project_typeenv_stages_v47_no_insert"
	if _, err := fixture.harness.Database().Exec(
		"DROP TRIGGER " + missingTrigger,
	); err != nil {
		t.Fatalf("drop exact immutability trigger: %v", err)
	}

	result, err := fixture.service.PrepareAtBase(
		context.Background(),
		fixture.base,
	)
	if result != nil ||
		err == nil ||
		!strings.Contains(err.Error(), missingTrigger) {
		t.Fatalf(
			"PrepareAtBase(drift) = (%T, %v), want nil and exact trigger error",
			result,
			err,
		)
	}
	assertPreparationCandidateRows(t, fixture.harness.Database(), 0)
	assertPreparationTableCount(
		t,
		fixture.harness.Database(),
		"typed_memory_graph_heads",
		0,
	)
	var triggerCount int64
	if err := fixture.harness.Database().QueryRow(
		`SELECT COUNT(*)
		 FROM sqlite_schema
		 WHERE type = 'trigger' AND name = ?`,
		missingTrigger,
	).Scan(&triggerCount); err != nil {
		t.Fatalf("count missing immutability trigger: %v", err)
	}
	if triggerCount != 0 {
		t.Fatal("PrepareAtBase repaired the missing immutability trigger")
	}
}

func TestPreparationServiceRejectsAlteredTriggerBodyWithoutRepairOrGraphWrite(
	t *testing.T,
) {
	fixture := newPreparationFixture(t)
	const trigger = "project_typeenv_stages_v47_no_insert"
	const altered = `CREATE TRIGGER project_typeenv_stages_v47_no_insert
		BEFORE INSERT ON project_typeenv_stages
		BEGIN
			SELECT RAISE(
				ABORT,
				'project TypeEnv candidate store is immutable'
			) WHERE 0;
		END`
	if _, err := fixture.harness.Database().Exec(
		"DROP TRIGGER " + trigger,
	); err != nil {
		t.Fatalf("drop exact immutability trigger: %v", err)
	}
	if _, err := fixture.harness.Database().Exec(altered); err != nil {
		t.Fatalf("install altered immutability trigger: %v", err)
	}
	before := preparationSQLiteObjectSQL(
		t,
		fixture.harness.Database(),
		"trigger",
		trigger,
	)

	result, err := fixture.service.PrepareAtBase(
		context.Background(),
		fixture.base,
	)
	if result != nil ||
		err == nil ||
		!strings.Contains(err.Error(), trigger) {
		t.Fatalf(
			"PrepareAtBase(altered DDL) = (%T, %v), want nil and exact trigger error",
			result,
			err,
		)
	}
	assertPreparationCandidateRows(t, fixture.harness.Database(), 0)
	assertPreparationTableCount(
		t,
		fixture.harness.Database(),
		"typed_memory_graph_heads",
		0,
	)
	after := preparationSQLiteObjectSQL(
		t,
		fixture.harness.Database(),
		"trigger",
		trigger,
	)
	if after != before {
		t.Fatal("PrepareAtBase repaired the altered immutability trigger")
	}
}

func TestPreparationServiceRejectsMissingProjectExecutableCoordinateRegistrationWithoutRepairOrGraphWrite(
	t *testing.T,
) {
	fixture := newPreparationFixture(t)
	const trigger = "project_typeenv_executable_snapshots_v47_register_coordinate"
	if _, err := fixture.harness.Database().Exec(
		"DROP TRIGGER " + trigger,
	); err != nil {
		t.Fatalf("drop executable-coordinate registration trigger: %v", err)
	}

	assertPreparationRejectedBeforeGraphWrite(t, fixture, trigger)
	if count := preparationSQLiteObjectCount(
		t,
		fixture.harness.Database(),
		"trigger",
		trigger,
	); count != 0 {
		t.Fatal("PrepareAtBase repaired the missing coordinate-registration trigger")
	}
}

func TestPreparationServiceRejectsAlteredProjectExecutableCoordinateRegistrationWithoutRepairOrGraphWrite(
	t *testing.T,
) {
	fixture := newPreparationFixture(t)
	const trigger = "project_typeenv_executable_snapshots_v47_register_coordinate"
	const altered = `CREATE TRIGGER project_typeenv_executable_snapshots_v47_register_coordinate
		AFTER INSERT ON project_typeenv_executable_snapshots
		BEGIN
			SELECT 1;
		END`
	if _, err := fixture.harness.Database().Exec(
		"DROP TRIGGER " + trigger,
	); err != nil {
		t.Fatalf("drop executable-coordinate registration trigger: %v", err)
	}
	if _, err := fixture.harness.Database().Exec(altered); err != nil {
		t.Fatalf("install altered coordinate-registration trigger: %v", err)
	}
	before := preparationSQLiteObjectSQL(
		t,
		fixture.harness.Database(),
		"trigger",
		trigger,
	)

	assertPreparationRejectedBeforeGraphWrite(t, fixture, trigger)
	after := preparationSQLiteObjectSQL(
		t,
		fixture.harness.Database(),
		"trigger",
		trigger,
	)
	if after != before {
		t.Fatal("PrepareAtBase repaired the altered coordinate-registration trigger")
	}
}

func TestPreparationServiceRejectsUnknownTouchedTableTriggerWithoutRepairOrGraphWrite(
	t *testing.T,
) {
	fixture := newPreparationFixture(t)
	const trigger = "project_typeenv_stages_unknown_side_effect"
	const statement = `CREATE TRIGGER project_typeenv_stages_unknown_side_effect
		AFTER INSERT ON project_typeenv_stages
		BEGIN
			SELECT 1;
		END`
	if _, err := fixture.harness.Database().Exec(statement); err != nil {
		t.Fatalf("install unknown Stage trigger: %v", err)
	}
	before := preparationSQLiteObjectSQL(
		t,
		fixture.harness.Database(),
		"trigger",
		trigger,
	)

	assertPreparationRejectedBeforeGraphWrite(t, fixture, trigger)
	after := preparationSQLiteObjectSQL(
		t,
		fixture.harness.Database(),
		"trigger",
		trigger,
	)
	if after != before {
		t.Fatal("PrepareAtBase removed or rewrote the unknown Stage trigger")
	}
}

func TestPreparationServiceLeavesAdvancedGraphWithoutCandidatePersistence(
	t *testing.T,
) {
	fixture := newPreparationFixture(t)
	ctx := context.Background()
	baseSnapshot, err := projectmemory.NewBaseTypeEnvSnapshot(fixture.base)
	if err != nil {
		t.Fatalf("NewBaseTypeEnvSnapshot(): %v", err)
	}
	initializer, err := typedmemorystore.NewSQLiteProjectGraphInitializer(
		fixture.harness.Database(),
		projectmemory.NewBaseTypeEnvLoader(),
		preparationTestClock(),
	)
	if err != nil {
		t.Fatalf("NewSQLiteProjectGraphInitializer(): %v", err)
	}
	initialized, err := initializer.InitializeProjectGraphAtBaseTypeEnv(
		ctx,
		fixture.ledger.ProjectID(),
		baseSnapshot,
	)
	if err != nil {
		t.Fatalf("initialize exact base graph: %v", err)
	}
	if _, ok := initialized.(typedmemorystore.InitializedAtBase); !ok {
		t.Fatalf("base initialization = %T, want InitializedAtBase", initialized)
	}
	advancePreparationGraph(t, fixture, baseSnapshot)
	before := preparationRelevantRowCounts(t, fixture.harness.Database())

	result, err := fixture.service.PrepareAtBase(ctx, fixture.base)
	if err != nil {
		t.Fatalf("PrepareAtBase(active): %v", err)
	}
	active, ok := result.(GraphAlreadyActive)
	if !ok {
		t.Fatalf("PrepareAtBase(active) = %T, want GraphAlreadyActive", result)
	}
	observation := active.Observation()
	if observation.Project() != fixture.ledger.ProjectID() ||
		observation.GraphRevision().Value() != 1 ||
		observation.ActiveTypeEnv() != baseSnapshot.Ref() {
		t.Fatal("GraphAlreadyActive lost the exact advanced graph coordinate")
	}
	after := preparationRelevantRowCounts(t, fixture.harness.Database())
	if !reflect.DeepEqual(after, before) {
		t.Fatalf(
			"already-active preparation changed rows:\nbefore=%v\nafter=%v",
			before,
			after,
		)
	}
	assertPreparationCandidateRows(t, fixture.harness.Database(), 0)
	assertPreparationHasNoProjectTypeEnvBindingEffects(
		t,
		fixture.harness.Database(),
	)
}

func assertPreparationRejectedBeforeGraphWrite(
	t *testing.T,
	fixture preparationFixture,
	errorFragment string,
) {
	t.Helper()
	result, err := fixture.service.PrepareAtBase(
		context.Background(),
		fixture.base,
	)
	if result != nil ||
		err == nil ||
		!strings.Contains(err.Error(), errorFragment) {
		t.Fatalf(
			"PrepareAtBase(storage drift) = (%T, %v), want nil and error containing %q",
			result,
			err,
			errorFragment,
		)
	}
	assertPreparationCandidateRows(t, fixture.harness.Database(), 0)
	assertPreparationTableCount(
		t,
		fixture.harness.Database(),
		"typed_memory_graph_heads",
		0,
	)
}

func preparationSQLiteObjectSQL(
	t *testing.T,
	database *sql.DB,
	kind string,
	name string,
) string {
	t.Helper()
	var statement string
	err := database.
		QueryRow(
			`SELECT sql
			 FROM sqlite_schema
			 WHERE type = ? AND name = ?`,
			kind,
			name,
		).
		Scan(&statement)
	if err != nil {
		t.Fatal(err)
	}
	return statement
}

func preparationSQLiteObjectCount(
	t *testing.T,
	database *sql.DB,
	kind string,
	name string,
) int64 {
	t.Helper()
	var count int64
	err := database.
		QueryRow(
			`SELECT COUNT(*)
			 FROM sqlite_schema
			 WHERE type = ? AND name = ?`,
			kind,
			name,
		).
		Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	return count
}

func newPreparationFixture(t *testing.T) preparationFixture {
	t.Helper()
	root := canonicalPreparationTempDir(t)
	harness := profileadmissionfixture.New(t, root)
	return openPreparationFixture(t, harness)
}

func openPreparationFixture(
	t *testing.T,
	harness *profileadmissionfixture.Harness,
) preparationFixture {
	t.Helper()
	ledger, err := projectledger.OpenExisting(
		context.Background(),
		harness.Root().String(),
		projectledger.ReadWrite,
	)
	if err != nil {
		t.Fatalf("projectledger.OpenExisting(): %v", err)
	}
	t.Cleanup(func() {
		if err := ledger.Close(); err != nil {
			t.Errorf("close project ledger: %v", err)
		}
	})
	service, err := NewService(
		context.Background(),
		ledger,
		preparationTestClock(),
	)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	return preparationFixture{
		harness: harness,
		ledger:  ledger,
		service: service,
		base:    preparationBaseArtifact(t),
	}
}

func preparationTestClock() preparationClock {
	return preparationClock{
		value: time.Date(2026, 7, 18, 10, 30, 0, 123456789, time.UTC),
	}
}

func preparationBaseArtifact(t *testing.T) typeenv.BaseTypeEnvArtifact {
	t.Helper()
	path := filepath.Join("..", "..", "cli", "fpf.db")
	database, err := sql.Open(
		"sqlite",
		"file:"+filepath.ToSlash(path)+"?mode=ro&immutable=1",
	)
	if err != nil {
		t.Fatalf("open bundled FPF database: %v", err)
	}
	defer func() { _ = database.Close() }()
	artifact, err := typeenvsql.LoadArtifactReadOnlyDB(
		context.Background(),
		database,
	)
	if err != nil {
		t.Fatalf("LoadArtifactReadOnlyDB(): %v", err)
	}
	return artifact
}

func assertPreparationCandidateAtExactBase(
	t *testing.T,
	fixture preparationFixture,
	candidate projecttypeenvpreparation.GenesisCandidate,
) {
	t.Helper()
	baseRef, executable := fixture.base.TypeEnvRef()
	if !executable {
		t.Fatal("bundled base is not executable")
	}
	stage := candidate.Stage()
	if candidate.BaseSnapshot().Ref() != baseRef ||
		stage.Project() != fixture.ledger.ProjectID() ||
		stage.Base() != baseRef ||
		stage.GraphRevision().Value() != 0 ||
		stage.VerifiedComposite() != candidate.Target().Composite().Ref() ||
		stage.RuntimeBasis() != candidate.Target().RuntimeBasis().Ref() {
		t.Fatal("prepared candidate does not retain exact B/E/X/C/revision-zero coordinates")
	}
	profile, err := projecttypeenvprofilebasis.NewNoCanonicalProjectProfile(
		fixture.harness.Root(),
	)
	if err != nil {
		t.Fatalf("NewNoCanonicalProjectProfile(): %v", err)
	}
	if stage.ProfileLedgerRevision() != profile.LedgerRevision() ||
		stage.ProfileLedgerDigest() != profile.ProfileLedgerDigest() {
		t.Fatal("prepared candidate does not retain exact absent-profile basis")
	}
}

func assertPreparationArtifactFootprint(
	t *testing.T,
	database *sql.DB,
	candidate projecttypeenvpreparation.GenesisCandidate,
) {
	t.Helper()
	closure := candidate.ArtifactClosure()
	baseRef, executable := closure.Base().TypeEnvRef()
	if !executable {
		t.Fatal("prepared closure base is not executable")
	}
	assertPreparationTableCount(t, database, "project_typeenv_artifacts", 4)
	assertPreparationTableCount(
		t,
		database,
		"project_typeenv_runtime_mechanisms",
		int64(len(closure.RuntimeMechanisms())),
	)
	assertPreparationTableCount(
		t,
		database,
		"project_typeenv_registration_policies",
		int64(len(closure.RegistrationPolicies())),
	)
	assertPreparationTableCount(
		t,
		database,
		"project_typeenv_composite_verifications",
		1,
	)
	assertPreparationTableCount(t, database, "project_typeenv_stages", 1)
	assertPreparationTableCount(
		t,
		database,
		"project_typeenv_executable_snapshots",
		1,
	)
	assertPreparationTableCount(
		t,
		database,
		"typed_memory_type_env_snapshots",
		1,
	)
	assertPreparationTableCount(t, database, "typed_memory_graph_heads", 1)
	for _, exact := range []struct {
		kind string
		ref  string
	}{
		{kind: "base_type_env", ref: baseRef.String()},
		{
			kind: "project_type_env_extension",
			ref:  closure.Extensions()[0].Ref().String(),
		},
		{
			kind: "runtime_evaluation_basis",
			ref:  closure.RuntimeBasis().Ref().String(),
		},
		{
			kind: "project_type_env_composite",
			ref:  closure.Composite().Ref().String(),
		},
	} {
		var count int64
		err := database.QueryRow(
			`SELECT COUNT(*)
			 FROM project_typeenv_artifacts
			 WHERE artifact_kind = ? AND artifact_ref = ?`,
			exact.kind,
			exact.ref,
		).Scan(&count)
		if err != nil {
			t.Fatalf("count exact %s artifact: %v", exact.kind, err)
		}
		if count != 1 {
			t.Fatalf(
				"exact %s artifact %q count = %d, want 1",
				exact.kind,
				exact.ref,
				count,
			)
		}
	}
}

func assertSamePreparationCandidate(
	t *testing.T,
	left projecttypeenvpreparation.GenesisCandidate,
	right projecttypeenvpreparation.GenesisCandidate,
) {
	t.Helper()
	if err := left.Verify(); err != nil {
		t.Fatalf("left candidate Verify(): %v", err)
	}
	if err := right.Verify(); err != nil {
		t.Fatalf("right candidate Verify(): %v", err)
	}
	if left.BaseSnapshot().Ref() != right.BaseSnapshot().Ref() ||
		left.ArtifactClosure().Composite().Ref() !=
			right.ArtifactClosure().Composite().Ref() ||
		left.Stage().Ref() != right.Stage().Ref() ||
		left.Verification().Ref() != right.Verification().Ref() ||
		left.ExecutableSnapshot().TypeEnvRef() !=
			right.ExecutableSnapshot().TypeEnvRef() ||
		!bytes.Equal(left.Stage().CanonicalBytes(), right.Stage().CanonicalBytes()) ||
		!bytes.Equal(
			left.Verification().CanonicalBytes(),
			right.Verification().CanonicalBytes(),
		) ||
		!bytes.Equal(
			left.ExecutableSnapshot().Record().CanonicalBytes(),
			right.ExecutableSnapshot().Record().CanonicalBytes(),
		) {
		t.Fatal("exact preparation replay returned a different candidate")
	}
}

func assertPreparationCandidateRows(
	t *testing.T,
	database *sql.DB,
	expected int64,
) {
	t.Helper()
	for _, table := range []string{
		"project_typeenv_artifacts",
		"project_typeenv_runtime_mechanisms",
		"project_typeenv_registration_policies",
		"project_typeenv_composite_verifications",
		"project_typeenv_stages",
		"project_typeenv_executable_snapshots",
	} {
		assertPreparationTableCount(t, database, table, expected)
	}
}

func assertPreparationHasNoBindingEffects(t *testing.T, database *sql.DB) {
	t.Helper()
	assertPreparationHasNoProjectTypeEnvBindingEffects(t, database)
	for _, table := range []string{
		"typed_memory_graph_events",
		"typed_memory_graph_commits",
	} {
		assertPreparationTableCount(t, database, table, 0)
	}
}

func assertPreparationHasNoProjectTypeEnvBindingEffects(
	t *testing.T,
	database *sql.DB,
) {
	t.Helper()
	for _, table := range []string{
		"project_typeenv_heads",
		"project_typeenv_head_states",
		"project_typeenv_head_selection_requests",
		"project_typeenv_head_selection_authorization_contents",
		"project_typeenv_head_selection_speech_act_records",
		"project_typeenv_head_selection_authority_resolution_bases",
		"project_typeenv_head_selection_authority_resolutions",
		"project_typeenv_head_selection_authority_uses",
		"project_typeenv_head_cas_work_records",
		"typed_memory_type_env_activations",
		"project_typeenv_head_history",
		"project_typeenv_head_selection_receipts",
		"project_typeenv_head_selection_closures",
	} {
		assertPreparationTableCount(t, database, table, 0)
	}
}

func advancePreparationGraph(
	t *testing.T,
	fixture preparationFixture,
	baseSnapshot typedmemorystore.TypeEnvSnapshot,
) {
	t.Helper()
	loader := projectmemory.NewBaseTypeEnvLoader()
	currentLoader, err := typedmemorystore.NewSQLiteCurrentProjectSnapshotLoader(
		fixture.harness.Database(),
		loader,
	)
	if err != nil {
		t.Fatalf("NewSQLiteCurrentProjectSnapshotLoader(): %v", err)
	}
	current, err := currentLoader.LoadCurrentProjectSnapshot(
		context.Background(),
		fixture.ledger.ProjectID(),
	)
	if err != nil {
		t.Fatalf("LoadCurrentProjectSnapshot(): %v", err)
	}
	contexts := current.Environment().BoundedContexts()
	if len(contexts) == 0 {
		t.Fatal("bundled base TypeEnv has no bounded context for active-graph fixture")
	}
	entity, err := typedmemory.NewEntityID("entity:genesis-preparation-active")
	if err != nil {
		t.Fatalf("NewEntityID(): %v", err)
	}
	local, err := typedmemory.NewBatchLocalRef(
		"local:genesis-preparation-active",
	)
	if err != nil {
		t.Fatalf("NewBatchLocalRef(): %v", err)
	}
	label, err := typedmemory.NewEntityLabel("Genesis preparation active fixture")
	if err != nil {
		t.Fatalf("NewEntityLabel(): %v", err)
	}
	provenance, err := typedmemory.NewProvenanceRef(
		"memory:test:genesis-preparation-active",
	)
	if err != nil {
		t.Fatalf("NewProvenanceRef(): %v", err)
	}
	declaration, err := typedmemory.NewDeclareEntity(
		entity,
		local,
		contexts[0].Ref(),
		label,
		provenance,
	)
	if err != nil {
		t.Fatalf("NewDeclareEntity(): %v", err)
	}
	candidate, err := typedmemory.NewMemoryChangeSet(
		[]typedmemory.MemoryChange{declaration},
	)
	if err != nil {
		t.Fatalf("NewMemoryChangeSet(): %v", err)
	}
	verdict := typedmemory.ValidateMemoryChangeSet(
		current.Environment(),
		current.Codecs(),
		current.Snapshot(),
		candidate,
	)
	valid, ok := verdict.(typedmemory.Valid)
	if !ok {
		t.Fatalf("ValidateMemoryChangeSet() = %T, want Valid", verdict)
	}
	key, err := typedmemorystore.NewIdempotencyKey(
		"genesis-preparation-active",
	)
	if err != nil {
		t.Fatalf("NewIdempotencyKey(): %v", err)
	}
	request, err := typedmemorystore.NewCommitRequestBuilder().
		SetContractVersion(typedmemorystore.AdmissionContractV2()).
		SetProject(fixture.ledger.ProjectID()).
		SetExpectedRevision(typedmemory.NewGraphRevision(0)).
		SetExpectedTypeEnv(baseSnapshot.Ref()).
		SetIdempotencyKey(key).
		SetRequestProvenance(provenance).
		SetCandidate(candidate).
		SetAdmissionBatch(valid.AdmissionBatch()).
		Build()
	if err != nil {
		t.Fatalf("build active-graph CommitRequest: %v", err)
	}
	adapter, err := typedmemorystore.NewGenericSQLiteAdapter(
		fixture.harness.Database(),
		loader,
		preparationTestClock(),
		preparationUnexpectedMemberOfEngine{},
		preparationUnexpectedReferenceEngine{},
		preparationUnexpectedObservableProvider{},
	)
	if err != nil {
		t.Fatalf("NewGenericSQLiteAdapter(): %v", err)
	}
	receipt, err := adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("CommitMemoryChangeSet(): %v", err)
	}
	if receipt.GraphRevision().Value() != 1 {
		t.Fatalf(
			"active-graph fixture revision = %d, want 1",
			receipt.GraphRevision().Value(),
		)
	}
}

type preparationUnexpectedMemberOfEngine struct{}

func (preparationUnexpectedMemberOfEngine) EvaluateMemberOf(
	context.Context,
	typedmemorystore.MemberOfEvaluationInput,
) (typedmemory.MemberOfJudgement, error) {
	return nil, fmt.Errorf("unexpected MemberOf evaluation")
}

type preparationUnexpectedReferenceEngine struct{}

func (preparationUnexpectedReferenceEngine) ResolveStrongReference(
	context.Context,
	typedmemorystore.StrongReferenceResolutionInput,
) (typedmemory.StrongReferenceResolution, error) {
	return nil, fmt.Errorf("unexpected strong-reference resolution")
}

type preparationUnexpectedObservableProvider struct{}

func (preparationUnexpectedObservableProvider) LoadObservableInput(
	context.Context,
	projectledger.ProjectID,
	typedmemory.ObservableInputRef,
	typedmemory.SHA256Digest,
) (typedmemorystore.ObservableInputBlob, error) {
	return typedmemorystore.ObservableInputBlob{},
		fmt.Errorf("unexpected observable-input load")
}

func assertPreparationTableCount(
	t *testing.T,
	database *sql.DB,
	table string,
	expected int64,
) {
	t.Helper()
	var actual int64
	query := "SELECT COUNT(*) FROM " + table
	if err := database.QueryRow(query).Scan(&actual); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if actual != expected {
		t.Fatalf("%s row count = %d, want %d", table, actual, expected)
	}
}

func preparationRelevantRowCounts(
	t *testing.T,
	database *sql.DB,
) map[string]int64 {
	t.Helper()
	rows, err := database.Query(
		`SELECT name
		 FROM sqlite_schema
		 WHERE type = 'table'
		   AND (
				name LIKE 'project_typeenv_%'
				OR name LIKE 'typed_memory_%'
		   )
		 ORDER BY name`,
	)
	if err != nil {
		t.Fatalf("list preparation tables: %v", err)
	}
	defer func() { _ = rows.Close() }()
	tables := make([]string, 0)
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatalf("scan preparation table: %v", err)
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate preparation tables: %v", err)
	}
	sort.Strings(tables)
	counts := make(map[string]int64, len(tables))
	for _, table := range tables {
		var count int64
		query := "SELECT COUNT(*) FROM " + table
		if err := database.QueryRow(query).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		counts[table] = count
	}
	return counts
}

func newConflictingPreparationSnapshot(
	t *testing.T,
) typedmemorystore.TypeEnvSnapshot {
	t.Helper()
	canonical := []byte(
		`{"schema":"test.project-typeenv-preparation.conflict/v1"}`,
	)
	reference := preparationTypeEnvRef(t, canonical)
	format, err := typedmemorystore.NewSnapshotFormat(
		typedmemorystore.BaseTypeEnvSnapshotFormat,
	)
	if err != nil {
		t.Fatalf("NewSnapshotFormat(): %v", err)
	}
	snapshot, err := typedmemorystore.NewTypeEnvSnapshotBuilder(reference).
		SetFormat(format).
		SetCanonicalBytes(canonical).
		SetSourceRevision(preparationSourceRevision(t)).
		SetCompilerSchemaVersion(preparationCompilerVersion(t)).
		Build()
	if err != nil {
		t.Fatalf("build conflicting preparation snapshot: %v", err)
	}
	return snapshot
}

func newPreparationTypeEnv(
	t *testing.T,
	snapshot typedmemorystore.TypeEnvSnapshot,
) typedmemory.TypeEnv {
	t.Helper()
	contextRef, err := typedmemory.NewBoundedContextRef(
		"ctx:test-project-typeenv-preparation",
	)
	if err != nil {
		t.Fatalf("NewBoundedContextRef(): %v", err)
	}
	provenance := preparationFPFProvenance(t, snapshot.SourceRevision())
	contextValue, err := typedmemory.NewBoundedContext(contextRef, provenance)
	if err != nil {
		t.Fatalf("NewBoundedContext(): %v", err)
	}
	subject, err := typedmemory.SourceUnitCoverage(
		provenance.Location().UnitID(),
	)
	if err != nil {
		t.Fatalf("SourceUnitCoverage(): %v", err)
	}
	entry, err := typedmemory.NewCompiledCoverageEntry(
		subject,
		provenance.Location(),
	)
	if err != nil {
		t.Fatalf("NewCompiledCoverageEntry(): %v", err)
	}
	coverage, err := typedmemory.NewCoverageManifest(
		[]typedmemory.CoverageEntry{entry},
	)
	if err != nil {
		t.Fatalf("NewCoverageManifest(): %v", err)
	}
	environment, err := typedmemory.NewTypeEnvBuilder(snapshot.Ref()).
		SetSourceRevision(snapshot.SourceRevision()).
		SetCompilerSchemaVersion(snapshot.CompilerSchemaVersion()).
		SetCoverageManifest(coverage).
		AddBoundedContext(contextValue).
		Build()
	if err != nil {
		t.Fatalf("build conflicting preparation TypeEnv: %v", err)
	}
	return environment
}

func preparationFPFProvenance(
	t *testing.T,
	revision typedmemory.SourceRevision,
) typedmemory.FPFSourceProvenance {
	t.Helper()
	unit, err := typedmemory.NewSourceUnitID(
		"spec:test-project-typeenv-preparation",
	)
	if err != nil {
		t.Fatalf("NewSourceUnitID(): %v", err)
	}
	lineRange, err := typedmemory.NewSourceLineRange(1, 1)
	if err != nil {
		t.Fatalf("NewSourceLineRange(): %v", err)
	}
	location, err := typedmemory.NewUnpatternedSourceLocation(
		unit,
		revision,
		preparationDigest(t, []byte("preparation conflict source")),
		lineRange,
	)
	if err != nil {
		t.Fatalf("NewUnpatternedSourceLocation(): %v", err)
	}
	reference, err := typedmemory.NewProvenanceRef(
		"prov:fpf:test-project-typeenv-preparation",
	)
	if err != nil {
		t.Fatalf("NewProvenanceRef(): %v", err)
	}
	rule, err := typedmemory.NewCompilerRuleID(
		"test.project-typeenv-preparation.context.v1",
	)
	if err != nil {
		t.Fatalf("NewCompilerRuleID(): %v", err)
	}
	provenance, err := typedmemory.NewFPFSourceProvenance(
		reference,
		location,
		rule,
	)
	if err != nil {
		t.Fatalf("NewFPFSourceProvenance(): %v", err)
	}
	return provenance
}

func preparationTypeEnvRef(
	t *testing.T,
	canonical []byte,
) typedmemory.TypeEnvRef {
	t.Helper()
	reference, err := typedmemory.NewTypeEnvRef(
		preparationDigest(t, canonical),
	)
	if err != nil {
		t.Fatalf("NewTypeEnvRef(): %v", err)
	}
	return reference
}

func preparationDigest(
	t *testing.T,
	value []byte,
) typedmemory.SHA256Digest {
	t.Helper()
	sum := sha256.Sum256(value)
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + hex.EncodeToString(sum[:]),
	)
	if err != nil {
		t.Fatalf("NewSHA256Digest(): %v", err)
	}
	return digest
}

func preparationSourceRevision(t *testing.T) typedmemory.SourceRevision {
	t.Helper()
	revision, err := typedmemory.NewSourceRevision(
		"test-project-typeenv-preparation-source",
	)
	if err != nil {
		t.Fatalf("NewSourceRevision(): %v", err)
	}
	return revision
}

func preparationCompilerVersion(
	t *testing.T,
) typedmemory.CompilerSchemaVersion {
	t.Helper()
	version, err := typedmemory.NewCompilerSchemaVersion(
		"test.project-typeenv-preparation.compiler.v1",
	)
	if err != nil {
		t.Fatalf("NewCompilerSchemaVersion(): %v", err)
	}
	return version
}

func canonicalPreparationTempDir(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve preparation temp dir: %v", err)
	}
	return filepath.Clean(root)
}
