package typedmemorystore

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	_ "modernc.org/sqlite"
)

func TestRebuildAdmissionObservationsUsesOnlyExactCommittedRevision(t *testing.T) {
	database := newAdmissionObservationDatabase(t)
	project := mustProjectID(t, "qnt_a7f3b2c1")
	contextRef := mustContextRef(t, "ctx:test")
	otherContext := mustContextRef(t, "ctx:other")
	entity := mustObservationEntityID(t, "entity:authorization")
	assertion := mustObservationAssertionID(t, "assertion:active")

	seedAdmissionObservationEvent(t, database, project, "event:one", 1, true)
	seedAdmissionObservationEvent(t, database, project, "event:two", 2, true)
	seedAdmissionObservationEvent(t, database, project, "event:open", 3, false)
	executeAdmissionObservationSQL(
		t,
		database,
		`INSERT INTO typed_memory_entity_contexts VALUES (?, ?, ?, ?, ?)`,
		project.String(), entity.String(), contextRef.String(), "event:one", 1,
	)
	executeAdmissionObservationSQL(
		t,
		database,
		`INSERT INTO typed_memory_entity_contexts VALUES (?, ?, ?, ?, ?)`,
		project.String(), "entity:open", contextRef.String(), "event:open", 3,
	)
	executeAdmissionObservationSQL(
		t,
		database,
		`INSERT INTO typed_memory_alias_changes VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		project.String(), "event:one", "alias-change:one", "", "admit_alias",
		contextRef.String(), "authorization", nil, entity.String(),
	)
	executeAdmissionObservationSQL(
		t,
		database,
		`INSERT INTO typed_memory_alias_changes VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		project.String(), "event:two", "alias-change:two", "alias-change:one", "supersede_alias",
		contextRef.String(), "authorization", "auth-service", entity.String(),
	)
	executeAdmissionObservationSQL(
		t,
		database,
		`INSERT INTO typed_memory_alias_changes VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		project.String(), "event:open", "alias-change:open", "", "admit_alias",
		contextRef.String(), "open-alias", nil, entity.String(),
	)
	executeAdmissionObservationSQL(
		t,
		database,
		`INSERT INTO typed_memory_relation_instances VALUES (?, ?, ?)`,
		project.String(), "event:one", assertion.String(),
	)
	executeAdmissionObservationSQL(
		t,
		database,
		`INSERT INTO typed_memory_assertion_retractions VALUES (?, ?, ?)`,
		project.String(), "event:two", assertion.String(),
	)
	executeAdmissionObservationSQL(
		t,
		database,
		`INSERT INTO typed_memory_relation_instances VALUES (?, ?, ?)`,
		project.String(), "event:open", "assertion:open",
	)

	transaction, err := sqlitetransaction.BeginImmediate(context.Background(), database)
	if err != nil {
		t.Fatalf("BeginImmediate: %v", err)
	}
	t.Cleanup(func() {
		finish := transaction.Rollback(context.Background())
		if !finish.Succeeded() {
			t.Fatalf("rollback observation transaction: %v", finish.Err())
		}
	})

	assertEntityObservationKind(
		t,
		transaction,
		project,
		1,
		entity,
		contextRef,
		typedmemory.EntityExactAdmissionObservation,
	)
	assertEntityObservationKind(
		t,
		transaction,
		project,
		1,
		entity,
		otherContext,
		typedmemory.EntityAbsentAdmissionObservation,
	)
	assertEntityObservationKind(
		t,
		transaction,
		project,
		3,
		mustObservationEntityID(t, "entity:open"),
		contextRef,
		typedmemory.EntityAbsentAdmissionObservation,
	)

	assertAliasObservationKind(
		t,
		transaction,
		project,
		1,
		"authorization",
		contextRef,
		typedmemory.AliasBoundAdmissionObservation,
	)
	assertAliasObservationKind(
		t,
		transaction,
		project,
		2,
		"authorization",
		contextRef,
		typedmemory.AliasUnboundAdmissionObservation,
	)
	assertAliasObservationKind(
		t,
		transaction,
		project,
		2,
		"auth-service",
		contextRef,
		typedmemory.AliasBoundAdmissionObservation,
	)
	assertAliasObservationKind(
		t,
		transaction,
		project,
		3,
		"open-alias",
		contextRef,
		typedmemory.AliasUnboundAdmissionObservation,
	)

	active := rebuildAssertionObservationFixture(
		t,
		transaction,
		project,
		1,
		assertion,
	)
	if active.Kind() != typedmemory.AssertionActiveAdmissionObservation {
		t.Fatalf("assertion observation at revision 1 = %s; want active", active.Kind())
	}
	_, err = rebuildAssertionObservationFixtureResult(
		transaction,
		project,
		2,
		assertion,
	)
	if !errors.Is(err, ErrRevalidationRejected) {
		t.Fatalf("retracted assertion error = %v; want ErrRevalidationRejected", err)
	}
	openAssertion := rebuildAssertionObservationFixture(
		t,
		transaction,
		project,
		3,
		mustObservationAssertionID(t, "assertion:open"),
	)
	if openAssertion.Kind() != typedmemory.AssertionAbsentAdmissionObservation {
		t.Fatalf("open-event assertion observation = %s; want absent", openAssertion.Kind())
	}
}

func TestRebuildAdmissionObservationsReadsEveryV3AssertionModalityAsActive(
	t *testing.T,
) {
	database := newAdmissionObservationDatabase(t)
	project := mustProjectID(t, "qnt_53aa0001")
	modalities := []string{
		"affirms_obtaining",
		"denies_obtaining",
		"obtaining_unknown",
	}
	assertions := make([]typedmemory.AssertionID, 0, len(modalities))
	for index, modality := range modalities {
		assertion := mustObservationAssertionID(
			t,
			"assertion:v3:"+modality,
		)
		eventRef := "event:v3:" + modality
		seedAdmissionObservationEventWithWriter(
			t,
			database,
			project,
			eventRef,
			int64(index+1),
			true,
			53,
			"writer_v53",
		)
		executeAdmissionObservationSQL(
			t,
			database,
			`INSERT INTO typed_memory_relational_assertions_v3 VALUES (?, ?, ?, ?, ?)`,
			project.String(),
			eventRef,
			assertion.String(),
			modality,
			"memory:test:"+modality,
		)
		assertions = append(assertions, assertion)
	}

	transaction := beginAdmissionObservationTransaction(t, database)
	revision := typedmemory.NewGraphRevision(uint64(len(assertions)))
	for _, assertion := range assertions {
		callerRule, err := typedmemory.NewRuleRef("caller-untrusted-rule")
		if err != nil {
			t.Fatalf("NewRuleRef: %v", err)
		}
		callerState, err := typedmemory.NewAbsentAssertionState(assertion, callerRule)
		if err != nil {
			t.Fatalf("NewAbsentAssertionState: %v", err)
		}
		expected, err := typedmemory.NewAssertionAbsentObservation(0, callerState)
		if err != nil {
			t.Fatalf("NewAssertionAbsentObservation: %v", err)
		}
		observations, err := rebuildAdmissionObservations(
			context.Background(),
			transaction,
			project,
			revision,
			[]typedmemory.AdmissionSnapshotObservation{expected},
		)
		if err != nil {
			t.Fatalf("rebuildAdmissionObservations(%s): %v", assertion.String(), err)
		}
		if len(observations) != 1 {
			t.Fatalf("rebuilt v3 observations = %d; want 1", len(observations))
		}
		active, ok := observations[0].(typedmemory.AssertionActiveObservation)
		if !ok {
			t.Fatalf(
				"rebuilt v3 observation = %T (%s); want active",
				observations[0],
				observations[0].Kind(),
			)
		}
		wantRule, err := snapshotAssertionRule(project, revision)
		if err != nil {
			t.Fatalf("snapshotAssertionRule: %v", err)
		}
		if active.State().Basis() != wantRule {
			t.Fatalf(
				"rebuilt v3 observation basis = %s; want %s",
				active.State().Basis().String(),
				wantRule.String(),
			)
		}
	}
}

func TestRebuildAssertionObservationRejectsRetractedV3Assertion(t *testing.T) {
	database := newAdmissionObservationDatabase(t)
	project := mustProjectID(t, "qnt_53aa0002")
	assertion := mustObservationAssertionID(t, "assertion:v3:retracted")
	seedAdmissionObservationEventWithWriter(
		t,
		database,
		project,
		"event:v3:origin",
		1,
		true,
		53,
		"writer_v53",
	)
	seedAdmissionObservationEventWithWriter(
		t,
		database,
		project,
		"event:v3:retraction",
		2,
		true,
		53,
		"writer_v53",
	)
	executeAdmissionObservationSQL(
		t,
		database,
		`INSERT INTO typed_memory_relational_assertions_v3 VALUES (?, ?, ?, ?, ?)`,
		project.String(),
		"event:v3:origin",
		assertion.String(),
		"affirms_obtaining",
		"memory:test:v3-origin",
	)
	executeAdmissionObservationSQL(
		t,
		database,
		`INSERT INTO typed_memory_assertion_retractions VALUES (?, ?, ?)`,
		project.String(),
		"event:v3:retraction",
		assertion.String(),
	)

	transaction := beginAdmissionObservationTransaction(t, database)
	active := rebuildAssertionObservationFixture(t, transaction, project, 1, assertion)
	if active.Kind() != typedmemory.AssertionActiveAdmissionObservation {
		t.Fatalf("v3 assertion at revision 1 = %s; want active", active.Kind())
	}
	_, err := rebuildAssertionObservationFixtureResult(
		transaction,
		project,
		2,
		assertion,
	)
	if !errors.Is(err, ErrRevalidationRejected) {
		t.Fatalf("retracted v3 assertion error = %v; want ErrRevalidationRejected", err)
	}
}

func TestRebuildAssertionObservationRejectsInvalidOriginUnion(t *testing.T) {
	t.Run("cross-lane duplicate", func(t *testing.T) {
		database := newAdmissionObservationDatabase(t)
		project := mustProjectID(t, "qnt_53aa0003")
		assertion := mustObservationAssertionID(t, "assertion:cross-lane")
		seedAdmissionObservationEvent(
			t,
			database,
			project,
			"event:legacy-origin",
			1,
			true,
		)
		seedAdmissionObservationEventWithWriter(
			t,
			database,
			project,
			"event:v3-origin",
			2,
			true,
			53,
			"writer_v53",
		)
		executeAdmissionObservationSQL(
			t,
			database,
			`INSERT INTO typed_memory_relation_instances VALUES (?, ?, ?)`,
			project.String(),
			"event:legacy-origin",
			assertion.String(),
		)
		executeAdmissionObservationSQL(
			t,
			database,
			`INSERT INTO typed_memory_relational_assertions_v3 VALUES (?, ?, ?, ?, ?)`,
			project.String(),
			"event:v3-origin",
			assertion.String(),
			"obtaining_unknown",
			"memory:test:v3-duplicate",
		)

		transaction := beginAdmissionObservationTransaction(t, database)
		_, err := rebuildAssertionObservationFixtureResult(
			transaction,
			project,
			2,
			assertion,
		)
		if !errors.Is(err, ErrRevalidationRejected) {
			t.Fatalf("cross-lane duplicate error = %v; want ErrRevalidationRejected", err)
		}
	})

	t.Run("writer-generation mismatch", func(t *testing.T) {
		database := newAdmissionObservationDatabase(t)
		project := mustProjectID(t, "qnt_53aa0004")
		assertion := mustObservationAssertionID(t, "assertion:v3:wrong-writer")
		seedAdmissionObservationEvent(
			t,
			database,
			project,
			"event:v3:wrong-writer",
			1,
			true,
		)
		executeAdmissionObservationSQL(
			t,
			database,
			`INSERT INTO typed_memory_relational_assertions_v3 VALUES (?, ?, ?, ?, ?)`,
			project.String(),
			"event:v3:wrong-writer",
			assertion.String(),
			"affirms_obtaining",
			"memory:test:v3-wrong-writer",
		)

		transaction := beginAdmissionObservationTransaction(t, database)
		_, err := rebuildAssertionObservationFixtureResult(
			transaction,
			project,
			1,
			assertion,
		)
		if !errors.Is(err, ErrStoredAdmissionIntegrity) {
			t.Fatalf("wrong-writer v3 error = %v; want ErrStoredAdmissionIntegrity", err)
		}
	})
}

func newAdmissionObservationDatabase(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open observation database: %v", err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close observation database: %v", err)
		}
	})
	statements := []string{
		`CREATE TABLE typed_memory_graph_events (
			project_id TEXT, event_ref TEXT, graph_revision INTEGER
		)`,
		`CREATE TABLE typed_memory_graph_commits (
			project_id TEXT, event_ref TEXT
		)`,
		`CREATE TABLE typed_memory_event_writer_generations (
			project_id TEXT, event_ref TEXT, writer_generation INTEGER,
			provenance_kind TEXT
		)`,
		`CREATE TABLE typed_memory_entity_contexts (
			project_id TEXT, entity_id TEXT, bounded_context_ref TEXT,
			declared_event_ref TEXT, declared_revision INTEGER
		)`,
		`CREATE TABLE typed_memory_alias_changes (
			project_id TEXT, event_ref TEXT, alias_change_ref TEXT,
			supersedes_alias_change_ref TEXT, change_kind TEXT,
			bounded_context_ref TEXT, alias TEXT, replacement_alias TEXT,
			entity_id TEXT
		)`,
		`CREATE TABLE typed_memory_relation_instances (
			project_id TEXT, event_ref TEXT, assertion_id TEXT
		)`,
		`CREATE TABLE typed_memory_relational_assertions_v3 (
			project_id TEXT, event_ref TEXT, assertion_id TEXT,
			modality TEXT, provenance_ref TEXT
		)`,
		`CREATE TABLE typed_memory_assertion_retractions (
			project_id TEXT, event_ref TEXT, assertion_id TEXT
		)`,
	}
	for _, statement := range statements {
		executeAdmissionObservationSQL(t, database, statement)
	}
	return database
}

func seedAdmissionObservationEvent(
	t *testing.T,
	database *sql.DB,
	project projectledger.ProjectID,
	eventRef string,
	revision int64,
	committed bool,
) {
	t.Helper()
	seedAdmissionObservationEventWithWriter(
		t,
		database,
		project,
		eventRef,
		revision,
		committed,
		46,
		"writer_v46",
	)
}

func seedAdmissionObservationEventWithWriter(
	t *testing.T,
	database *sql.DB,
	project projectledger.ProjectID,
	eventRef string,
	revision int64,
	committed bool,
	writerGeneration int64,
	writerProvenance string,
) {
	t.Helper()
	executeAdmissionObservationSQL(
		t,
		database,
		`INSERT INTO typed_memory_graph_events VALUES (?, ?, ?)`,
		project.String(),
		eventRef,
		revision,
	)
	executeAdmissionObservationSQL(
		t,
		database,
		`INSERT INTO typed_memory_event_writer_generations VALUES (?, ?, ?, ?)`,
		project.String(),
		eventRef,
		writerGeneration,
		writerProvenance,
	)
	if !committed {
		return
	}
	executeAdmissionObservationSQL(
		t,
		database,
		`INSERT INTO typed_memory_graph_commits VALUES (?, ?)`,
		project.String(),
		eventRef,
	)
}

func beginAdmissionObservationTransaction(
	t *testing.T,
	database *sql.DB,
) *sqlitetransaction.Transaction {
	t.Helper()
	transaction, err := sqlitetransaction.BeginImmediate(context.Background(), database)
	if err != nil {
		t.Fatalf("BeginImmediate: %v", err)
	}
	t.Cleanup(func() {
		finish := transaction.Rollback(context.Background())
		if !finish.Succeeded() {
			t.Fatalf("rollback observation transaction: %v", finish.Err())
		}
	})
	return transaction
}

func executeAdmissionObservationSQL(
	t *testing.T,
	database *sql.DB,
	statement string,
	arguments ...any,
) {
	t.Helper()
	_, err := database.Exec(statement, arguments...)
	if err != nil {
		t.Fatalf("execute observation fixture SQL: %v", err)
	}
}

func assertEntityObservationKind(
	t *testing.T,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	revision uint64,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	want typedmemory.AdmissionSnapshotObservationKind,
) {
	t.Helper()
	callerBasis, err := typedmemory.NewResolutionBasisRef("caller-untrusted-basis")
	if err != nil {
		t.Fatalf("NewResolutionBasisRef: %v", err)
	}
	resolution, err := typedmemory.NewAbsentEntityResolution(entity, contextRef, callerBasis)
	if err != nil {
		t.Fatalf("NewAbsentEntityResolution: %v", err)
	}
	expected, err := typedmemory.NewEntityAbsentObservation(0, resolution)
	if err != nil {
		t.Fatalf("NewEntityAbsentObservation: %v", err)
	}
	observations, err := rebuildAdmissionObservations(
		context.Background(),
		transaction,
		project,
		typedmemory.NewGraphRevision(revision),
		[]typedmemory.AdmissionSnapshotObservation{expected},
	)
	if err != nil {
		t.Fatalf("rebuildAdmissionObservations: %v", err)
	}
	if len(observations) != 1 || observations[0].Kind() != want {
		t.Fatalf("rebuilt entity observations = %#v; want one %s", observations, want)
	}
}

func assertAliasObservationKind(
	t *testing.T,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	revision uint64,
	aliasText string,
	contextRef typedmemory.BoundedContextRef,
	want typedmemory.AdmissionSnapshotObservationKind,
) {
	t.Helper()
	alias, err := typedmemory.NewEntityAlias(aliasText)
	if err != nil {
		t.Fatalf("NewEntityAlias: %v", err)
	}
	basis, err := typedmemory.NewResolutionBasisRef("caller-untrusted-basis")
	if err != nil {
		t.Fatalf("NewResolutionBasisRef: %v", err)
	}
	resolution, err := typedmemory.NewUnboundAliasResolution(alias, contextRef, basis)
	if err != nil {
		t.Fatalf("NewUnboundAliasResolution: %v", err)
	}
	expected, err := typedmemory.NewAliasUnboundObservation(0, resolution)
	if err != nil {
		t.Fatalf("NewAliasUnboundObservation: %v", err)
	}
	observations, err := rebuildAdmissionObservations(
		context.Background(),
		transaction,
		project,
		typedmemory.NewGraphRevision(revision),
		[]typedmemory.AdmissionSnapshotObservation{expected},
	)
	if err != nil {
		t.Fatalf("rebuildAdmissionObservations: %v", err)
	}
	if len(observations) != 1 || observations[0].Kind() != want {
		t.Fatalf("rebuilt alias observations = %#v; want one %s", observations, want)
	}
}

func rebuildAssertionObservationFixture(
	t *testing.T,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	revision uint64,
	assertion typedmemory.AssertionID,
) typedmemory.AdmissionSnapshotObservation {
	t.Helper()
	observation, err := rebuildAssertionObservationFixtureResult(
		transaction,
		project,
		revision,
		assertion,
	)
	if err != nil {
		t.Fatalf("rebuild assertion observation: %v", err)
	}
	return observation
}

func rebuildAssertionObservationFixtureResult(
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	revision uint64,
	assertion typedmemory.AssertionID,
) (typedmemory.AdmissionSnapshotObservation, error) {
	rule, err := snapshotAssertionRule(project, typedmemory.NewGraphRevision(revision))
	if err != nil {
		return nil, err
	}
	return rebuildAssertionObservation(
		context.Background(),
		transaction,
		project,
		typedmemory.NewGraphRevision(revision),
		0,
		assertion,
		rule,
	)
}

func mustObservationEntityID(t *testing.T, raw string) typedmemory.EntityID {
	t.Helper()
	entity, err := typedmemory.NewEntityID(raw)
	if err != nil {
		t.Fatalf("NewEntityID: %v", err)
	}
	return entity
}

func mustObservationAssertionID(t *testing.T, raw string) typedmemory.AssertionID {
	t.Helper()
	assertion, err := typedmemory.NewAssertionID(raw)
	if err != nil {
		t.Fatalf("NewAssertionID: %v", err)
	}
	return assertion
}
