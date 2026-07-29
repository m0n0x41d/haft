package typedmemorystore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestGenericStorageAvailabilityDoesNotDowngradeInstalledV46WithoutTables(
	t *testing.T,
) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open storage-availability fixture: %v", err)
	}
	defer database.Close()
	if _, err := database.Exec(`CREATE TABLE schema_version (version INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create schema-version fixture: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO schema_version (version) VALUES (46)`); err != nil {
		t.Fatalf("insert v46 schema version: %v", err)
	}
	availability, err := loadGenericStorageAvailability(
		context.Background(),
		databaseScanner{database: database},
	)
	if err != nil {
		t.Fatalf("loadGenericStorageAvailability: %v", err)
	}
	if availability != genericStoragePartial {
		t.Fatalf("v46 without core tables = %d; want genericStoragePartial", availability)
	}
}

func TestDecodeStoredV46DigestRowClassifiesStoredCorruption(t *testing.T) {
	validDigest := mustDigest(t, []byte("expected canonical materialization"))
	cases := map[string]string{
		"aggregate shape":       "missing-separator",
		"unknown row kind":      "unknown:" + validDigest.String() + ",00",
		"invalid canonical hex": "relation:" + validDigest.String() + ",NOT-HEX",
		"canonical digest mismatch": "relation:" + validDigest.String() + "," +
			strings.ToUpper("77726f6e672063616e6f6e6963616c206279746573"),
	}
	for name, encoded := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := decodeStoredV46DigestRow(encoded)
			if !errors.Is(err, ErrStoredAdmissionIntegrity) {
				t.Fatalf("decodeStoredV46DigestRow error = %v; want ErrStoredAdmissionIntegrity", err)
			}
		})
	}
}

func TestParseCanonicalGenericRecordedAtRejectsNonCanonicalUTCOffset(t *testing.T) {
	_, err := parseCanonicalGenericRecordedAt("2026-07-16T08:30:00.123456789+00:00")
	if err == nil {
		t.Fatal("non-canonical UTC offset was accepted as generic recorded_at")
	}
}

func TestGenericCommitRejectsCarrierRecordedAtDriftBeforeCommit(t *testing.T) {
	for _, testCase := range genericRecordedAtCarrierCases() {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newExactBasisStoreFixture(t)
			fixture.allowTestMutation(t, testCase.table)
			driftedRecordedAt := canonicalTime(fixture.base.clock.Now().Add(time.Second))
			trigger := fmt.Sprintf(
				`CREATE TRIGGER generic_recorded_at_fault
				AFTER INSERT ON typed_memory_commit_materialization_closures
				BEGIN
					UPDATE %s SET recorded_at = '%s'
					WHERE project_id = NEW.project_id AND event_ref = NEW.event_ref;
				END`,
				testCase.table,
				driftedRecordedAt,
			)
			if _, err := fixture.base.database.Exec(trigger); err != nil {
				t.Fatalf("install %s recorded_at fault: %v", testCase.name, err)
			}
			request := fixture.request(t, "same-tx-recorded-at-"+testCase.name)

			_, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
			if !errors.Is(err, ErrStoredAdmissionIntegrity) {
				t.Fatalf(
					"same-transaction %s recorded_at error = %v; want ErrStoredAdmissionIntegrity",
					testCase.name,
					err,
				)
			}
			if errors.Is(err, ErrCommitOutcomeUnknown) {
				t.Fatalf("%s recorded_at drift crossed COMMIT: %v", testCase.name, err)
			}
			fixture.assertNoSemanticCommit(t)
		})
	}
}

func TestGenericReplayRejectsDurableCarrierRecordedAtDrift(t *testing.T) {
	for _, testCase := range genericRecordedAtCarrierCases() {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newExactBasisStoreFixture(t)
			request := fixture.request(t, "durable-recorded-at-"+testCase.name)
			receipt, err := fixture.adapter.CommitMemoryChangeSet(
				context.Background(),
				request,
			)
			if err != nil {
				t.Fatalf("seed exact-basis commit: %v", err)
			}
			fixture.allowTestMutation(t, testCase.table)
			driftedRecordedAt := canonicalTime(fixture.base.clock.Now().Add(time.Second))
			result, err := fixture.base.database.Exec(
				"UPDATE "+testCase.table+" SET recorded_at = ? WHERE project_id = ? AND event_ref = ?",
				driftedRecordedAt,
				fixture.base.project.String(),
				receipt.EventRef(),
			)
			if err != nil {
				t.Fatalf("mutate durable %s recorded_at: %v", testCase.name, err)
			}
			assertExactBasisRowsAffected(t, result, 1, testCase.name+" recorded_at")

			_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
			if !errors.Is(err, ErrStoredAdmissionIntegrity) {
				t.Fatalf(
					"durable %s recorded_at error = %v; want ErrStoredAdmissionIntegrity",
					testCase.name,
					err,
				)
			}
			if errors.Is(err, ErrIdempotencyConflict) {
				t.Fatalf("%s recorded_at drift was misclassified as caller conflict: %v", testCase.name, err)
			}
		})
	}
}

type genericRecordedAtCarrierCase struct {
	name  string
	table string
}

func genericRecordedAtCarrierCases() []genericRecordedAtCarrierCase {
	return []genericRecordedAtCarrierCase{
		{
			name:  "admission",
			table: "typed_memory_event_admission_bases",
		},
		{
			name:  "closure",
			table: "typed_memory_commit_materialization_closures",
		},
	}
}

func TestGenericReplayClassifiesPartialV46AdmissionPairAsStoredCorruption(
	t *testing.T,
) {
	cases := []struct {
		name         string
		missingTable string
	}{
		{
			name:         "admission missing",
			missingTable: "typed_memory_event_admission_bases",
		},
		{
			name:         "closure missing",
			missingTable: "typed_memory_commit_materialization_closures",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newExactBasisStoreFixture(t)
			request := fixture.request(t, "generic-partial-v46-pair-"+testCase.name)
			receipt, err := fixture.adapter.CommitMemoryChangeSet(
				context.Background(),
				request,
			)
			if err != nil {
				t.Fatalf("seed exact-basis commit: %v", err)
			}

			connection, err := fixture.base.database.Conn(context.Background())
			if err != nil {
				t.Fatalf("open corruption-fixture connection: %v", err)
			}
			t.Cleanup(func() { _ = connection.Close() })
			if _, err := connection.ExecContext(
				context.Background(),
				"PRAGMA foreign_keys = OFF",
			); err != nil {
				t.Fatalf("disable fixture foreign keys: %v", err)
			}
			trigger := testCase.missingTable + "_v46_no_delete"
			if _, err := connection.ExecContext(
				context.Background(),
				"DROP TRIGGER "+trigger,
			); err != nil {
				t.Fatalf("drop %s: %v", trigger, err)
			}
			result, err := connection.ExecContext(
				context.Background(),
				"DELETE FROM "+testCase.missingTable+" WHERE project_id = ? AND event_ref = ?",
				fixture.base.project.String(),
				receipt.EventRef(),
			)
			if err != nil {
				t.Fatalf("remove %s: %v", testCase.missingTable, err)
			}
			assertExactBasisRowsAffected(t, result, 1, testCase.missingTable)
			if _, err := connection.ExecContext(
				context.Background(),
				"PRAGMA foreign_keys = ON",
			); err != nil {
				t.Fatalf("restore fixture foreign keys: %v", err)
			}
			if err := connection.Close(); err != nil {
				t.Fatalf("close corruption-fixture connection: %v", err)
			}

			_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
			if !errors.Is(err, ErrStoredAdmissionIntegrity) {
				t.Fatalf(
					"partial v46 pair error = %v; want ErrStoredAdmissionIntegrity",
					err,
				)
			}
			if errors.Is(err, ErrStorageGenerationUnavailable) {
				t.Fatalf("partial v46 pair was misclassified as unavailable: %v", err)
			}
		})
	}
}

func TestGenericReplayRejectsMissingV46CompanionsDespiteV46WriterMarker(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	request := fixture.request(t, "generic-missing-v46-companions")
	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("seed exact-basis commit: %v", err)
	}
	connection, err := fixture.base.database.Conn(context.Background())
	if err != nil {
		t.Fatalf("open missing-companions fixture connection: %v", err)
	}
	if _, err := connection.ExecContext(context.Background(), "PRAGMA foreign_keys = OFF"); err != nil {
		_ = connection.Close()
		t.Fatalf("disable missing-companions fixture foreign keys: %v", err)
	}
	for _, table := range []string{
		"typed_memory_commit_materialization_closures",
		"typed_memory_event_admission_bases",
	} {
		if _, err := connection.ExecContext(
			context.Background(),
			"DROP TRIGGER "+table+"_v46_no_delete",
		); err != nil {
			_ = connection.Close()
			t.Fatalf("drop %s delete guard: %v", table, err)
		}
		result, err := connection.ExecContext(
			context.Background(),
			"DELETE FROM "+table+" WHERE project_id = ? AND event_ref = ?",
			fixture.base.project.String(),
			receipt.EventRef(),
		)
		if err != nil {
			_ = connection.Close()
			t.Fatalf("delete %s: %v", table, err)
		}
		assertExactBasisRowsAffected(t, result, 1, table)
	}
	if err := connection.Close(); err != nil {
		t.Fatalf("close missing-companions fixture connection: %v", err)
	}

	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("missing v46 companions error = %v; want ErrStoredAdmissionIntegrity", err)
	}
}

func TestGenericReplayRejectsMissingWriterGenerationMarker(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	request := fixture.request(t, "generic-missing-writer-generation")
	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("seed exact-basis commit: %v", err)
	}
	if _, err := fixture.base.database.Exec(
		`DROP TRIGGER typed_memory_event_writer_generations_v46_no_delete`,
	); err != nil {
		t.Fatalf("drop writer-generation delete guard: %v", err)
	}
	result, err := fixture.base.database.Exec(
		`DELETE FROM typed_memory_event_writer_generations
		WHERE project_id = ? AND event_ref = ?`,
		fixture.base.project.String(),
		receipt.EventRef(),
	)
	if err != nil {
		t.Fatalf("delete writer-generation marker: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "writer-generation marker")

	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("missing writer-generation marker error = %v; want ErrStoredAdmissionIntegrity", err)
	}
}

func TestGenericReplayRejectsCorrelatedStoredEventDigestWithNonCanonicalRef(
	t *testing.T,
) {
	fixture := newExactBasisStoreFixture(t)
	request := fixture.request(t, "generic-correlated-event-corruption")
	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("seed exact-basis commit: %v", err)
	}
	common, found, err := loadDurableGenericCommonRow(
		context.Background(),
		databaseScanner{database: fixture.base.database},
		fixture.base.project,
		request.idempotencyKey,
	)
	if err != nil || !found {
		t.Fatalf("load stored generic event: found=%t err=%v", found, err)
	}
	corruptedProvenance := "memory:test:correlated-stored-event-corruption"
	correlatedDigest, err := digestFields(
		"typed-memory-graph-event.v1",
		fixture.base.project.String(),
		common.eventCommitRef,
		strconv.FormatInt(common.eventExpectedRevision, 10),
		strconv.FormatInt(common.eventRevision, 10),
		common.eventBasisTypeEnv,
		common.eventChangeDigest,
		string(common.eventCanonicalBytes),
		common.eventKind,
		common.eventAuthorityClass,
		corruptedProvenance,
	)
	if err != nil {
		t.Fatalf("digest correlated stored-event corruption: %v", err)
	}
	if derivedRef("typed-memory-event", correlatedDigest.String()) == receipt.EventRef() {
		t.Fatal("correlated corruption fixture did not change the canonical event ref")
	}
	for _, trigger := range []string{
		"typed_memory_graph_events_no_update",
		"typed_memory_idempotency_history_no_update",
		"typed_memory_graph_commits_no_update",
	} {
		if _, err := fixture.base.database.Exec("DROP TRIGGER " + trigger); err != nil {
			t.Fatalf("drop %s: %v", trigger, err)
		}
	}
	transaction, err := fixture.base.database.Begin()
	if err != nil {
		t.Fatalf("begin correlated event-corruption transaction: %v", err)
	}
	updates := []struct {
		statement string
		arguments []any
		label     string
	}{
		{
			statement: `UPDATE typed_memory_graph_events
				SET request_provenance_ref = ?, event_digest = ?
				WHERE project_id = ? AND event_ref = ?`,
			arguments: []any{
				corruptedProvenance,
				correlatedDigest.String(),
				fixture.base.project.String(),
				receipt.EventRef(),
			},
			label: "graph event",
		},
		{
			statement: `UPDATE typed_memory_idempotency_history
				SET result_digest = ?
				WHERE project_id = ? AND idempotency_key = ?`,
			arguments: []any{
				correlatedDigest.String(),
				fixture.base.project.String(),
				request.idempotencyKey.String(),
			},
			label: "idempotency history",
		},
		{
			statement: `UPDATE typed_memory_graph_commits
				SET event_digest = ?
				WHERE project_id = ? AND event_ref = ?`,
			arguments: []any{
				correlatedDigest.String(),
				fixture.base.project.String(),
				receipt.EventRef(),
			},
			label: "graph commit",
		},
	}
	for _, update := range updates {
		result, updateErr := transaction.Exec(update.statement, update.arguments...)
		if updateErr != nil {
			_ = transaction.Rollback()
			t.Fatalf("mutate correlated %s: %v", update.label, updateErr)
		}
		assertExactBasisRowsAffected(t, result, 1, update.label)
	}
	if err := transaction.Commit(); err != nil {
		t.Fatalf("commit correlated event-corruption fixture: %v", err)
	}

	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("correlated stored-event error = %v; want ErrStoredAdmissionIntegrity", err)
	}
	if errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("correlated stored-event corruption was misclassified as caller conflict: %v", err)
	}
}

func TestGenericReplayRejectsStoredRequestBytesBeforeCallerComparison(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	request := fixture.request(t, "generic-corrupt-request-bytes")
	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("seed exact-basis commit: %v", err)
	}
	fixture.allowTestMutation(t, "typed_memory_event_admission_bases")
	result, err := fixture.base.database.Exec(
		`UPDATE typed_memory_event_admission_bases
		SET canonical_request_bytes = ?
		WHERE project_id = ? AND event_ref = ?`,
		[]byte("corrupted canonical request bytes"),
		fixture.base.project.String(),
		receipt.EventRef(),
	)
	if err != nil {
		t.Fatalf("inject stored request-byte corruption: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "stored canonical request bytes")

	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("stored request-byte error = %v; want ErrStoredAdmissionIntegrity", err)
	}
	if errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("stored request-byte corruption was misclassified as caller conflict: %v", err)
	}
}

func TestGenericReplayRejectsJointlyRehashedStoredAdmissionCarriers(t *testing.T) {
	cases := []struct {
		name      string
		statement string
	}{
		{
			name: "semantic change set",
			statement: `UPDATE typed_memory_event_admission_bases
				SET canonical_semantic_bytes = ?, semantic_digest = ?
				WHERE project_id = ? AND event_ref = ?`,
		},
		{
			name: "admission envelope",
			statement: `UPDATE typed_memory_event_admission_bases
				SET canonical_admission_envelope_bytes = ?, admission_envelope_digest = ?
				WHERE project_id = ? AND event_ref = ?`,
		},
		{
			name: "admission basis",
			statement: `UPDATE typed_memory_event_admission_bases
				SET canonical_admission_basis_bytes = ?, admission_basis_digest = ?
				WHERE project_id = ? AND event_ref = ?`,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newExactBasisStoreFixture(t)
			request := fixture.request(t, "generic-correlated-carrier-"+testCase.name)
			receipt, err := fixture.adapter.CommitMemoryChangeSet(
				context.Background(),
				request,
			)
			if err != nil {
				t.Fatalf("seed exact-basis commit: %v", err)
			}
			fixture.allowTestMutation(t, "typed_memory_event_admission_bases")
			corruptedBytes := []byte("correlated self-hashed " + testCase.name)
			corruptedDigest := mustDigest(t, corruptedBytes)
			result, err := fixture.base.database.Exec(
				testCase.statement,
				corruptedBytes,
				corruptedDigest.String(),
				fixture.base.project.String(),
				receipt.EventRef(),
			)
			if err != nil {
				t.Fatalf("inject correlated %s corruption: %v", testCase.name, err)
			}
			assertExactBasisRowsAffected(t, result, 1, testCase.name)

			_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
			if !errors.Is(err, ErrStoredAdmissionIntegrity) {
				t.Fatalf("correlated %s error = %v; want ErrStoredAdmissionIntegrity", testCase.name, err)
			}
			if errors.Is(err, ErrIdempotencyConflict) {
				t.Fatalf("correlated %s corruption was misclassified as caller conflict: %v", testCase.name, err)
			}
		})
	}
}

func TestGenericReplayRejectsJointlyRehashedStoredRequestCarrier(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	candidate := fixture.finalCandidate(t, "New entity", "new payload")
	request := fixture.finalRequest(t, "generic-correlated-request-carrier", candidate)
	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("seed mixed commit: %v", err)
	}
	if _, err := fixture.base.database.Exec(
		"DROP TRIGGER typed_memory_event_admission_bases_v46_no_update",
	); err != nil {
		t.Fatalf("allow stored request-carrier corruption fixture: %v", err)
	}
	corruptedBytes := []byte("correlated self-hashed request carrier")
	corruptedDigest := mustDigest(t, corruptedBytes)
	result, err := fixture.base.database.Exec(
		`UPDATE typed_memory_event_admission_bases
		SET canonical_request_bytes = ?, request_digest = ?
		WHERE project_id = ? AND event_ref = ?`,
		corruptedBytes,
		corruptedDigest.String(),
		fixture.base.project.String(),
		receipt.EventRef(),
	)
	if err != nil {
		t.Fatalf("inject correlated request-carrier corruption: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "request carrier")

	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("correlated request-carrier error = %v; want ErrStoredAdmissionIntegrity", err)
	}
	if errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("correlated request-carrier corruption was misclassified as caller conflict: %v", err)
	}
}

func TestGenericReplayRejectsClosureOwnedAdmissionDigestDrift(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	request := fixture.request(t, "generic-closure-owned-basis-digest-drift")
	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("seed exact-basis commit: %v", err)
	}
	fixture.allowTestMutation(t, "typed_memory_commit_materialization_closures")
	wrongDigest := mustDigest(t, []byte("wrong closure-owned admission-basis digest"))
	result, err := fixture.base.database.Exec(
		`UPDATE typed_memory_commit_materialization_closures
		SET admission_basis_digest = ?
		WHERE project_id = ? AND event_ref = ?`,
		wrongDigest.String(),
		fixture.base.project.String(),
		receipt.EventRef(),
	)
	if err != nil {
		t.Fatalf("inject closure-owned admission-basis drift: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "closure-owned admission-basis digest")

	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("closure-owned admission-basis drift error = %v; want ErrStoredAdmissionIntegrity", err)
	}
	if errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("closure-owned admission drift was misclassified as caller conflict: %v", err)
	}
}

func TestGenericReplayRejectsCorrelatedStoredAdmissionBasisKindDrift(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	request := fixture.request(t, "generic-correlated-admission-basis-kind-drift")
	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("seed exact-basis commit: %v", err)
	}
	admission, found, err := loadDurableV46AdmissionRow(
		context.Background(),
		databaseScanner{database: fixture.base.database},
		fixture.base.project,
		receipt.EventRef(),
	)
	if err != nil || !found {
		t.Fatalf("load stored admission basis: found=%t err=%v", found, err)
	}
	storedKind, err := typedmemory.ParseAdmissionBasisKind(admission.basisKind)
	if err != nil {
		t.Fatalf("parse stored admission-basis kind: %v", err)
	}
	mutatedKind := oppositeAdmissionBasisKind(t, storedKind)
	fixture.allowTestMutation(t, "typed_memory_event_admission_bases")
	fixture.allowTestMutation(t, "typed_memory_commit_materialization_closures")
	transaction, err := fixture.base.database.Begin()
	if err != nil {
		t.Fatalf("begin correlated basis-kind transaction: %v", err)
	}
	if _, err := transaction.Exec(`PRAGMA defer_foreign_keys = ON`); err != nil {
		_ = transaction.Rollback()
		t.Fatalf("defer correlated basis-kind foreign keys: %v", err)
	}
	for _, table := range []string{
		"typed_memory_event_admission_bases",
		"typed_memory_commit_materialization_closures",
	} {
		result, updateErr := transaction.Exec(
			"UPDATE "+table+" SET admission_basis_kind = ? WHERE project_id = ? AND event_ref = ?",
			mutatedKind.String(),
			fixture.base.project.String(),
			receipt.EventRef(),
		)
		if updateErr != nil {
			_ = transaction.Rollback()
			t.Fatalf("mutate correlated admission-basis kind in %s: %v", table, updateErr)
		}
		assertExactBasisRowsAffected(t, result, 1, table+" admission-basis kind")
	}
	if err := transaction.Commit(); err != nil {
		t.Fatalf("commit correlated basis-kind fixture: %v", err)
	}

	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("correlated admission-basis kind error = %v; want ErrStoredAdmissionIntegrity", err)
	}
	if errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("correlated admission-basis kind drift was misclassified as caller conflict: %v", err)
	}
}

func oppositeAdmissionBasisKind(
	t *testing.T,
	kind typedmemory.AdmissionBasisKind,
) typedmemory.AdmissionBasisKind {
	t.Helper()
	switch kind {
	case typedmemory.SnapshotOnlyAdmissionBasis:
		return typedmemory.ContextSliceMembershipAdmissionBasis
	case typedmemory.ContextSliceMembershipAdmissionBasis:
		return typedmemory.SnapshotOnlyAdmissionBasis
	default:
		t.Fatalf("unsupported admission-basis kind %d", kind)
		return 0
	}
}

func TestGenericReplayRejectsCorrelatedValidBasisCoordinateDrift(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	candidate := fixture.finalCandidate(t, "Coordinate-bound entity", "coordinate-bound payload")
	key := "generic-correlated-valid-basis-coordinate-drift"
	request := fixture.finalRequest(t, key, candidate)
	alternate := fixture.requestAt(
		t,
		3,
		key,
		candidate,
		func(snapshot *genericMixedSnapshot) {
			newEntity := mustGenericEntityID(t, "entity:new")
			newAssertion := mustGenericAssertionID(t, "assertion:new")
			snapshot.entityAbsent(t, newEntity, fixture.primary)
			snapshot.entityExact(t, fixture.anchor, fixture.primary)
			snapshot.aliasUnbound(t, mustGenericAlias(t, "fresh-anchor"), fixture.primary)
			snapshot.aliasBound(t, fixture.oldAlias, fixture.anchor, fixture.primary)
			snapshot.aliasUnbound(t, mustGenericAlias(t, "replacement-anchor"), fixture.primary)
			snapshot.assertionAbsent(t, newAssertion)
			snapshot.assertionActive(t, fixture.oldAssertion)
		},
	)
	originalBasis := request.admissionBatch.Basis()
	alternateBasis := alternate.admissionBatch.Basis()
	if originalBasis.Kind() != alternateBasis.Kind() {
		t.Fatalf(
			"alternate basis kind = %s; want same kind %s",
			alternateBasis.Kind().String(),
			originalBasis.Kind().String(),
		)
	}
	if originalBasis.GraphRevision() == alternateBasis.GraphRevision() {
		t.Fatal("alternate basis fixture did not change the graph revision")
	}
	if request.admissionBatch.RequestDigest() != alternate.admissionBatch.RequestDigest() ||
		request.admissionBatch.SemanticChangeDigest() != alternate.admissionBatch.SemanticChangeDigest() {
		t.Fatal("alternate basis fixture changed request or semantic identity")
	}
	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("seed coordinate-bound admission: %v", err)
	}
	for _, table := range []string{
		"typed_memory_event_admission_bases",
		"typed_memory_commit_materialization_closures",
	} {
		trigger := table + "_v46_no_update"
		if _, err := fixture.base.database.Exec("DROP TRIGGER " + trigger); err != nil {
			t.Fatalf("drop %s: %v", trigger, err)
		}
	}
	transaction, err := fixture.base.database.Begin()
	if err != nil {
		t.Fatalf("begin valid basis-coordinate mutation: %v", err)
	}
	result, err := transaction.Exec(
		`UPDATE typed_memory_event_admission_bases
		SET canonical_admission_basis_bytes = ?, admission_basis_digest = ?,
			canonical_admission_envelope_bytes = ?, admission_envelope_digest = ?
		WHERE project_id = ? AND event_ref = ?`,
		alternateBasis.CanonicalBytes(),
		alternateBasis.Digest().String(),
		alternate.admissionBatch.CanonicalEnvelopeBytes(),
		alternate.admissionBatch.AdmissionEnvelopeDigest().String(),
		fixture.base.project.String(),
		receipt.EventRef(),
	)
	if err != nil {
		_ = transaction.Rollback()
		t.Fatalf("replace stored admission carriers with alternate valid basis: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "alternate valid admission basis")
	result, err = transaction.Exec(
		`UPDATE typed_memory_commit_materialization_closures
		SET admission_basis_digest = ?, admission_envelope_digest = ?
		WHERE project_id = ? AND event_ref = ?`,
		alternateBasis.Digest().String(),
		alternate.admissionBatch.AdmissionEnvelopeDigest().String(),
		fixture.base.project.String(),
		receipt.EventRef(),
	)
	if err != nil {
		_ = transaction.Rollback()
		t.Fatalf("propagate alternate admission carrier digests to closure: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "alternate closure admission carriers")
	if err := transaction.Commit(); err != nil {
		t.Fatalf("commit valid basis-coordinate mutation: %v", err)
	}

	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("valid basis-coordinate drift error = %v; want ErrStoredAdmissionIntegrity", err)
	}
	if !strings.Contains(err.Error(), "stored admission-basis coordinates") {
		t.Fatalf("valid basis-coordinate drift error = %v; want coordinate verification detail", err)
	}
	if errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("valid basis-coordinate drift was misclassified as caller conflict: %v", err)
	}
}
