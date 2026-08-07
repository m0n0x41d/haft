package typedmemorystore

import (
	"context"
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/db"
)

func TestFreshV2AdmissionsUseWriter53AcrossConsecutiveCommitsAndSnapshotRead(
	t *testing.T,
) {
	fixture := newSQLiteStoreFixture(t)
	adapter := newGenericFixtureAdapter(t, fixture)

	firstCandidate := fixture.declaration(t, "writer53-first", "Writer 53 first")
	first := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"writer53:first",
		firstCandidate,
	)
	first.admissionBatch = sealGenericDeclaration(t, fixture, firstCandidate, 0)
	firstReceipt, err := adapter.CommitMemoryChangeSet(context.Background(), first)
	if err != nil {
		t.Fatalf("first writer-53 commit: %v", err)
	}

	secondCandidate := fixture.declaration(t, "writer53-second", "Writer 53 second")
	second := fixture.request(
		t,
		1,
		fixture.environment.Ref(),
		"writer53:second",
		secondCandidate,
	)
	second.admissionBatch = sealGenericDeclaration(t, fixture, secondCandidate, 1)
	secondReceipt, err := adapter.CommitMemoryChangeSet(context.Background(), second)
	if err != nil {
		t.Fatalf("second writer-53 commit: %v", err)
	}
	if firstReceipt.GraphRevision().Value() != 1 ||
		secondReceipt.GraphRevision().Value() != 2 {
		t.Fatalf(
			"writer-53 revisions = %d, %d; want 1, 2",
			firstReceipt.GraphRevision().Value(),
			secondReceipt.GraphRevision().Value(),
		)
	}

	rows, err := fixture.database.Query(
		`SELECT writer_generation, provenance_kind
		FROM typed_memory_event_writer_generations
		WHERE project_id = ? ORDER BY event_ref`,
		fixture.project.String(),
	)
	if err != nil {
		t.Fatalf("query writer generations: %v", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var generation int64
		var provenance string
		if err := rows.Scan(&generation, &provenance); err != nil {
			t.Fatalf("scan writer generation: %v", err)
		}
		if generation != relationalAssertionWriterGeneration || provenance != "writer_v53" {
			t.Fatalf(
				"writer marker = (%d, %q); want (53, writer_v53)",
				generation,
				provenance,
			)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate writer generations: %v", err)
	}
	if count != 2 {
		t.Fatalf("writer-53 marker count = %d; want 2", count)
	}

	snapshot, err := adapter.LoadCurrentProjectSnapshot(
		context.Background(),
		fixture.project,
	)
	if err != nil {
		t.Fatalf("load writer-53 current snapshot: %v", err)
	}
	if snapshot.Snapshot().GraphRevision().Value() != 2 {
		t.Fatalf(
			"snapshot revision = %d; want 2",
			snapshot.Snapshot().GraphRevision().Value(),
		)
	}
}

func TestFreshV1ReplayMissFailsBeforeCASWithZeroMutation(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	adapter := newGenericFixtureAdapter(t, fixture)
	candidate := fixture.declaration(t, "v1-miss", "V1 replay miss")
	request := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"v1:replay-miss",
		candidate,
	)
	request.admissionBatch = sealGenericDeclaration(t, fixture, candidate, 0)
	request = requestWithContractVersion(t, request, AdmissionContractV1())

	_, err := adapter.CommitMemoryChangeSet(context.Background(), request)
	if !errors.Is(err, ErrLegacyAdmissionReplayOnly) {
		t.Fatalf("fresh v1 miss error = %v; want ErrLegacyAdmissionReplayOnly", err)
	}
	assertTypedMemoryRowCounts(t, fixture.database, map[string]int64{
		"typed_memory_graph_events":                    0,
		"typed_memory_graph_commits":                   0,
		"typed_memory_event_admission_bases":           0,
		"typed_memory_commit_materialization_closures": 0,
		"typed_memory_event_writer_generations":        0,
		"typed_memory_entities":                        0,
		"typed_memory_entity_contexts":                 0,
		"typed_memory_idempotency_history":             0,
	})
}

func TestHistoricalWriter46V1ExactReplaySurvivesRestart(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	adapter := newGenericFixtureAdapter(t, fixture)
	candidate := fixture.declaration(t, "historical-v1", "Historical V1")
	request := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"historical:v1",
		candidate,
	)
	request.admissionBatch = sealGenericDeclaration(t, fixture, candidate, 0)
	receipt, err := adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("seed exact admission: %v", err)
	}
	seedHistoricalWriter46(t, fixture, receipt.EventRef())

	if err := fixture.store.Close(); err != nil {
		t.Fatalf("close pre-restart store: %v", err)
	}
	restarted, err := db.NewStore(fixture.databasePath)
	if err != nil {
		t.Fatalf("restart migrated store: %v", err)
	}
	t.Cleanup(func() { _ = restarted.Close() })
	fixture.store = restarted
	fixture.database = restarted.GetRawDB()
	restartedAdapter := newGenericFixtureAdapter(t, fixture)

	v1Commit := requestWithContractVersion(t, request, AdmissionContractV1())
	replayed, err := restartedAdapter.CommitMemoryChangeSet(
		context.Background(),
		v1Commit,
	)
	if err != nil {
		t.Fatalf("historical v1 commit replay after restart: %v", err)
	}
	if replayed.Disposition() != CommitReplay ||
		replayed.EventRef() != receipt.EventRef() ||
		replayed.CommitRef() != receipt.CommitRef() {
		t.Fatalf("historical v1 replay = %#v; want exact original %#v", replayed, receipt)
	}

	probe := replayRequestFromCommit(t, v1Commit)
	publicReplay, found, err := restartedAdapter.ReplayMemoryChangeSet(
		context.Background(),
		probe,
	)
	if err != nil {
		t.Fatalf("public historical v1 replay: %v", err)
	}
	if !found || publicReplay.EventRef() != receipt.EventRef() {
		t.Fatalf("public historical v1 replay = (%#v, %v); want exact hit", publicReplay, found)
	}
}

func TestOccupiedDifferentWriterGenerationIsReplayConflict(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	adapter := newGenericFixtureAdapter(t, fixture)
	candidate := fixture.declaration(t, "generation-conflict", "Generation conflict")
	v2 := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"generation:conflict",
		candidate,
	)
	v2.admissionBatch = sealGenericDeclaration(t, fixture, candidate, 0)
	if _, err := adapter.CommitMemoryChangeSet(context.Background(), v2); err != nil {
		t.Fatalf("seed writer-53 admission: %v", err)
	}
	v1 := requestWithContractVersion(t, v2, AdmissionContractV1())
	_, found, err := adapter.ReplayMemoryChangeSet(
		context.Background(),
		replayRequestFromCommit(t, v1),
	)
	if !found || !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf(
			"cross-generation replay = (found %v, error %v); want occupied conflict",
			found,
			err,
		)
	}
}

func TestMissingWriterGenerationMarkerIsStoredIntegrityFailure(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	adapter := newGenericFixtureAdapter(t, fixture)
	candidate := fixture.declaration(t, "missing-generation", "Missing generation")
	request := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"generation:missing",
		candidate,
	)
	request.admissionBatch = sealGenericDeclaration(t, fixture, candidate, 0)
	receipt, err := adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("seed writer-53 admission: %v", err)
	}
	if _, err := fixture.database.Exec(
		`DROP TRIGGER typed_memory_event_writer_generations_v46_no_delete`,
	); err != nil {
		t.Fatalf("open writer-marker corruption seam: %v", err)
	}
	if _, err := fixture.database.Exec(
		`DELETE FROM typed_memory_event_writer_generations
		WHERE project_id = ? AND event_ref = ?`,
		fixture.project.String(),
		receipt.EventRef(),
	); err != nil {
		t.Fatalf("remove writer marker: %v", err)
	}

	_, found, err := adapter.ReplayMemoryChangeSet(
		context.Background(),
		replayRequestFromCommit(t, request),
	)
	if !found || !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf(
			"missing-marker replay = (found %v, error %v); want occupied integrity failure",
			found,
			err,
		)
	}
}

func requestWithContractVersion(
	t *testing.T,
	request CommitRequest,
	version AdmissionContractVersion,
) CommitRequest {
	t.Helper()
	result, err := NewCommitRequestBuilder().
		SetContractVersion(version).
		SetProject(request.project).
		SetExpectedRevision(request.expectedRevision).
		SetExpectedTypeEnv(request.expectedTypeEnv).
		SetIdempotencyKey(request.idempotencyKey).
		SetRequestProvenance(request.requestProvenance).
		SetCandidate(request.candidate).
		SetAdmissionBatch(request.admissionBatch).
		Build()
	if err != nil {
		t.Fatalf("build versioned commit request: %v", err)
	}
	return result
}

func replayRequestFromCommit(
	t *testing.T,
	request CommitRequest,
) ReplayRequest {
	t.Helper()
	replay, err := NewReplayRequestBuilder().
		SetContractVersion(request.ContractVersion()).
		SetProject(request.project).
		SetExpectedRevision(request.expectedRevision).
		SetExpectedTypeEnv(request.expectedTypeEnv).
		SetIdempotencyKey(request.idempotencyKey).
		SetRequestProvenance(request.requestProvenance).
		SetCandidate(request.candidate).
		Build()
	if err != nil {
		t.Fatalf("build replay request: %v", err)
	}
	return replay
}

func seedHistoricalWriter46(
	t *testing.T,
	fixture sqliteStoreFixture,
	eventRef string,
) {
	t.Helper()
	const triggerName = "typed_memory_event_writer_generations_v46_no_update"
	var triggerSQL string
	if err := fixture.database.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type = 'trigger' AND name = ?`,
		triggerName,
	).Scan(&triggerSQL); err != nil {
		t.Fatalf("load exact historical writer marker trigger: %v", err)
	}
	if _, err := fixture.database.Exec(
		`DROP TRIGGER typed_memory_event_writer_generations_v46_no_update`,
	); err != nil {
		t.Fatalf("open historical writer marker seam: %v", err)
	}
	restoreTrigger := func() {
		t.Helper()
		if _, err := fixture.database.Exec(triggerSQL); err != nil {
			t.Fatalf("restore exact historical writer marker trigger: %v", err)
		}
	}
	if _, err := fixture.database.Exec(
		`UPDATE typed_memory_event_writer_generations
		SET writer_generation = 46, provenance_kind = 'writer_v46'
		WHERE project_id = ? AND event_ref = ?`,
		fixture.project.String(),
		eventRef,
	); err != nil {
		restoreTrigger()
		t.Fatalf("seed historical writer-46 marker: %v", err)
	}
	restoreTrigger()
}
