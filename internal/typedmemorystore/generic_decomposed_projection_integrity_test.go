package typedmemorystore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectledger"
)

// These cases mutate query-visible columns after every semantic row has been
// inserted but before the pending admission is verified. Canonical carriers
// and their digests remain unchanged. The commit may therefore be accepted
// only when the exact materialization verifier binds every decomposed column
// back to the sealed candidate and admission basis.
func TestGenericCommitRejectsDecomposedProjectionColumnDriftBeforeCommit(
	t *testing.T,
) {
	for _, fault := range decomposedProjectionColumnFaults() {
		t.Run(fault.name, func(t *testing.T) {
			fixture := fault.newFixture(t, "same-tx-"+fault.name)
			allowDecomposedProjectionMutation(t, fixture.database, fault.updates)
			installDecomposedProjectionFaultTrigger(t, fixture.database, fault)

			_, err := fixture.adapter.CommitMemoryChangeSet(
				context.Background(),
				fixture.request,
			)
			if !errors.Is(err, ErrStoredAdmissionIntegrity) {
				t.Fatalf(
					"same-transaction projection drift error = %v; want ErrStoredAdmissionIntegrity",
					err,
				)
			}
			if errors.Is(err, ErrCommitOutcomeUnknown) {
				t.Fatalf("projection drift was first detected after COMMIT: %v", err)
			}
			assertDecomposedProjectionHead(t, fixture)
		})
	}
}

func TestGenericReplayRejectsDurableDecomposedProjectionColumnDrift(
	t *testing.T,
) {
	for _, fault := range decomposedProjectionColumnFaults() {
		t.Run(fault.name, func(t *testing.T) {
			fixture := fault.newFixture(t, "replay-"+fault.name)
			receipt, err := fixture.adapter.CommitMemoryChangeSet(
				context.Background(),
				fixture.request,
			)
			if err != nil {
				t.Fatalf("seed exact materialization: %v", err)
			}

			allowDecomposedProjectionMutation(t, fixture.database, fault.updates)
			applyDurableDecomposedProjectionFault(
				t,
				fixture,
				receipt.EventRef(),
				fault,
			)

			_, err = fixture.adapter.CommitMemoryChangeSet(
				context.Background(),
				fixture.request,
			)
			if !errors.Is(err, ErrStoredAdmissionIntegrity) {
				t.Fatalf(
					"durable projection drift error = %v; want ErrStoredAdmissionIntegrity",
					err,
				)
			}
			if errors.Is(err, ErrIdempotencyConflict) {
				t.Fatalf("stored projection drift was misclassified as caller conflict: %v", err)
			}
		})
	}
}

type decomposedProjectionTestFixture struct {
	database        *sql.DB
	adapter         *SQLiteAdapter
	project         projectledger.ProjectID
	request         CommitRequest
	initialRevision uint64
}

type decomposedProjectionColumnFault struct {
	name       string
	newFixture func(*testing.T, string) decomposedProjectionTestFixture
	updates    []decomposedProjectionColumnUpdate
}

type decomposedProjectionColumnUpdate struct {
	table        string
	assignment   string
	predicate    string
	wantAffected int64
}

func decomposedProjectionColumnFaults() []decomposedProjectionColumnFault {
	return []decomposedProjectionColumnFault{
		{
			name:       "context_slice_bounded_context_ref",
			newFixture: newReferenceProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_context_slice_catalog",
					assignment:   "bounded_context_ref = 'ctx:projection-drift'",
					wantAffected: 1,
				},
				{
					table:        "typed_memory_context_slices",
					assignment:   "bounded_context_ref = 'ctx:projection-drift'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "relation_signature_ref",
			newFixture: newValueProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_relational_assertions_v3",
					assignment:   "signature_ref = 'typeenv:projection-drift/signature'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "relation_provenance_ref",
			newFixture: newValueProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_relational_assertions_v3",
					assignment:   "provenance_ref = 'memory:test:projection-drift/relation'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "relation_slot_kind_ref",
			newFixture: newValueProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_relational_assertion_slots_v3",
					assignment:   "slot_kind_ref = 'projection-drift-slot'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "reference_filler_reference_kind_ref",
			newFixture: newReferenceProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_relational_assertion_fillers_v3",
					assignment:   "reference_kind_ref = 'typeenv:projection-drift/ref-kind'",
					predicate:    " AND filler_kind = 'by_reference'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "reference_filler_reference_id",
			newFixture: newReferenceProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_relational_assertion_fillers_v3",
					assignment:   "reference_id = 'entity:projection-drift'",
					predicate:    " AND filler_kind = 'by_reference'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "reference_filler_entity_id",
			newFixture: newReferenceProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_relational_assertion_fillers_v3",
					assignment:   "entity_id = 'entity:projection-drift'",
					predicate:    " AND filler_kind = 'by_reference'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "reference_filler_required_value_kind_ref",
			newFixture: newReferenceProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_relational_assertion_fillers_v3",
					assignment:   "required_value_kind_ref = 'typeenv:projection-drift/value-kind'",
					predicate:    " AND filler_kind = 'by_reference'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "value_filler_value_ref",
			newFixture: newValueProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_relational_assertion_fillers_v3",
					assignment:   "value_ref = 'value:projection-drift'",
					predicate:    " AND filler_kind = 'by_value'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "alias_provenance_ref",
			newFixture: newValueProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_alias_changes",
					assignment:   "provenance_ref = 'memory:test:projection-drift/alias'",
					predicate:    " AND change_kind = 'admit_alias'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "retraction_reason",
			newFixture: newValueProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_assertion_retractions",
					assignment:   "reason = 'projection drift reason'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "retraction_provenance_ref",
			newFixture: newValueProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_assertion_retractions",
					assignment:   "provenance_ref = 'memory:test:projection-drift/retraction'",
					wantAffected: 1,
				},
			},
		},
	}
}

func newReferenceProjectionTestFixture(
	t *testing.T,
	key string,
) decomposedProjectionTestFixture {
	t.Helper()
	fixture := newExactBasisStoreFixture(t)
	return decomposedProjectionTestFixture{
		database:        fixture.base.database,
		adapter:         fixture.adapter,
		project:         fixture.base.project,
		request:         fixture.request(t, "decomposed-projection-"+key),
		initialRevision: 0,
	}
}

func newValueProjectionTestFixture(
	t *testing.T,
	key string,
) decomposedProjectionTestFixture {
	t.Helper()
	fixture := newGenericMixedStoreFixture(t)
	candidate := fixture.finalCandidate(t, "Projection entity", "projection payload")
	return decomposedProjectionTestFixture{
		database:        fixture.base.database,
		adapter:         fixture.adapter,
		project:         fixture.base.project,
		request:         fixture.finalRequest(t, "decomposed-projection-"+key, candidate),
		initialRevision: 2,
	}
}

func allowDecomposedProjectionMutation(
	t *testing.T,
	database *sql.DB,
	updates []decomposedProjectionColumnUpdate,
) {
	t.Helper()
	seen := make(map[string]struct{}, len(updates))
	for _, update := range updates {
		if _, exists := seen[update.table]; exists {
			continue
		}
		seen[update.table] = struct{}{}
		trigger := update.table + "_v46_no_update"
		if strings.HasSuffix(update.table, "_v3") {
			trigger = update.table + "_v53_no_update"
		}
		statement := "DROP TRIGGER " + trigger
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("drop %s: %v", trigger, err)
		}
	}
}

func installDecomposedProjectionFaultTrigger(
	t *testing.T,
	database *sql.DB,
	fault decomposedProjectionColumnFault,
) {
	t.Helper()
	body := make([]string, 0, len(fault.updates))
	for _, update := range fault.updates {
		statement := fmt.Sprintf(
			"UPDATE %s SET %s WHERE project_id = NEW.project_id AND event_ref = NEW.event_ref%s;",
			update.table,
			update.assignment,
			update.predicate,
		)
		body = append(body, statement)
	}
	trigger := fmt.Sprintf(
		`CREATE TRIGGER decomposed_projection_test_%s
		AFTER INSERT ON typed_memory_commit_materialization_closures
		BEGIN
			%s
		END`,
		fault.name,
		strings.Join(body, "\n"),
	)
	if _, err := database.Exec(trigger); err != nil {
		t.Fatalf("install %s projection fault: %v", fault.name, err)
	}
}

func applyDurableDecomposedProjectionFault(
	t *testing.T,
	fixture decomposedProjectionTestFixture,
	eventRef string,
	fault decomposedProjectionColumnFault,
) {
	t.Helper()
	transaction, err := fixture.database.Begin()
	if err != nil {
		t.Fatalf("begin %s durable projection mutation: %v", fault.name, err)
	}
	if _, err := transaction.Exec(`PRAGMA defer_foreign_keys = ON`); err != nil {
		_ = transaction.Rollback()
		t.Fatalf("defer %s projection foreign keys: %v", fault.name, err)
	}
	for _, update := range fault.updates {
		statement := fmt.Sprintf(
			"UPDATE %s SET %s WHERE project_id = ? AND event_ref = ?%s",
			update.table,
			update.assignment,
			update.predicate,
		)
		result, updateErr := transaction.Exec(
			statement,
			fixture.project.String(),
			eventRef,
		)
		if updateErr != nil {
			_ = transaction.Rollback()
			t.Fatalf("apply %s projection mutation to %s: %v", fault.name, update.table, updateErr)
		}
		assertExactBasisRowsAffected(
			t,
			result,
			update.wantAffected,
			fault.name+" "+update.table,
		)
	}
	if err := transaction.Commit(); err != nil {
		t.Fatalf("commit %s durable projection mutation: %v", fault.name, err)
	}
}

func assertDecomposedProjectionHead(
	t *testing.T,
	fixture decomposedProjectionTestFixture,
) {
	t.Helper()
	head, err := fixture.adapter.LoadHead(context.Background(), fixture.project)
	if err != nil {
		t.Fatalf("LoadHead after rejected projection drift: %v", err)
	}
	if head.Revision().Value() != fixture.initialRevision {
		t.Fatalf(
			"graph revision after rejected projection drift = %d; want %d",
			head.Revision().Value(),
			fixture.initialRevision,
		)
	}
}
