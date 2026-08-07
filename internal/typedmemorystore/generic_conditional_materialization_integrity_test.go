package typedmemorystore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestGenericCommitRejectsConditionalRowOwnershipDriftBeforeCommit(t *testing.T) {
	for _, fault := range conditionalOwnershipFaults() {
		t.Run(fault.name, func(t *testing.T) {
			t.Parallel()

			fixture := fault.newFixture(t, "same-tx-"+fault.name)
			dropConditionalOwnershipGuard(t, fixture.database, fault.guardTrigger)
			installConditionalOwnershipFaultTrigger(t, fixture.database, fault)

			_, err := fixture.adapter.CommitMemoryChangeSet(
				context.Background(),
				fixture.request,
			)
			if err == nil {
				t.Fatal("conditional ownership drift committed")
			}
			if errors.Is(err, ErrCommitOutcomeUnknown) {
				t.Fatalf("conditional ownership drift was first detected after COMMIT: %v", err)
			}
			assertConditionalOwnershipHead(t, fixture)
		})
	}
}

func TestGenericReplayRejectsConditionalRowOwnershipDrift(t *testing.T) {
	for _, fault := range conditionalOwnershipFaults() {
		t.Run(fault.name, func(t *testing.T) {
			t.Parallel()

			fixture := fault.newFixture(t, "replay-"+fault.name)
			receipt, err := fixture.adapter.CommitMemoryChangeSet(
				context.Background(),
				fixture.request,
			)
			if err != nil {
				t.Fatalf("seed conditional materialization: %v", err)
			}

			dropConditionalOwnershipGuard(t, fixture.database, fault.guardTrigger)
			result, err := fixture.database.Exec(
				fault.durableUpdate,
				fixture.project.String(),
				receipt.EventRef(),
			)
			if err != nil {
				t.Fatalf("apply %s ownership drift: %v", fault.name, err)
			}
			assertExactBasisRowsAffected(t, result, 1, fault.name+" ownership")

			_, err = fixture.adapter.CommitMemoryChangeSet(
				context.Background(),
				fixture.request,
			)
			if !errors.Is(err, ErrStoredAdmissionIntegrity) {
				t.Fatalf("durable conditional ownership drift error = %v; want ErrStoredAdmissionIntegrity", err)
			}
			if errors.Is(err, ErrIdempotencyConflict) {
				t.Fatalf("stored ownership drift was misclassified as caller conflict: %v", err)
			}
		})
	}
}

type conditionalOwnershipFixture struct {
	database        *sql.DB
	adapter         *SQLiteAdapter
	project         projectledger.ProjectID
	request         CommitRequest
	initialRevision uint64
}

type conditionalOwnershipFault struct {
	name          string
	guardTrigger  string
	newFixture    func(*testing.T, string) conditionalOwnershipFixture
	triggerUpdate string
	durableUpdate string
}

func conditionalOwnershipFaults() []conditionalOwnershipFault {
	return []conditionalOwnershipFault{
		{
			name:         "entity_context_declared_revision",
			guardTrigger: "typed_memory_entity_contexts_no_update",
			newFixture:   newGlobalEntityOwnershipFixture,
			triggerUpdate: `UPDATE typed_memory_entity_contexts
				SET declared_revision = 1
				WHERE project_id = NEW.project_id
					AND declared_event_ref = NEW.event_ref;`,
			durableUpdate: `UPDATE typed_memory_entity_contexts
				SET declared_revision = 1
				WHERE project_id = ?1 AND declared_event_ref = ?2`,
		},
		{
			name:         "global_entity_owner",
			guardTrigger: "typed_memory_entities_no_update",
			newFixture:   newGlobalEntityOwnershipFixture,
			triggerUpdate: `UPDATE typed_memory_entities
				SET first_declared_event_ref = (
					SELECT event_ref FROM typed_memory_graph_events
					WHERE project_id = NEW.project_id AND graph_revision = 1
				), first_declared_revision = 1
				WHERE project_id = NEW.project_id
					AND first_declared_event_ref = NEW.event_ref;`,
			durableUpdate: `UPDATE typed_memory_entities
				SET first_declared_event_ref = (
					SELECT event_ref FROM typed_memory_graph_events
					WHERE project_id = ?1 AND graph_revision = 1
				), first_declared_revision = 1
				WHERE project_id = ?1 AND first_declared_event_ref = ?2`,
		},
		{
			name:         "context_slice_catalog_owner",
			guardTrigger: "typed_memory_context_slice_catalog_v46_no_update",
			newFixture:   newContextSliceCatalogOwnershipFixture,
			triggerUpdate: `UPDATE typed_memory_context_slice_catalog
				SET event_ref = (
					SELECT event_ref FROM typed_memory_graph_events
					WHERE project_id = NEW.project_id AND graph_revision = 1
				)
				WHERE project_id = NEW.project_id AND event_ref = NEW.event_ref;`,
			durableUpdate: `UPDATE typed_memory_context_slice_catalog
				SET event_ref = (
					SELECT event_ref FROM typed_memory_graph_events
					WHERE project_id = ?1 AND graph_revision = 1
				)
				WHERE project_id = ?1 AND event_ref = ?2`,
		},
	}
}

func newGlobalEntityOwnershipFixture(
	t *testing.T,
	key string,
) conditionalOwnershipFixture {
	t.Helper()
	fixture := newGenericMixedStoreFixture(t)
	candidate := fixture.finalCandidate(t, "Ownership entity", "ownership payload")
	return conditionalOwnershipFixture{
		database:        fixture.base.database,
		adapter:         fixture.adapter,
		project:         fixture.base.project,
		request:         fixture.finalRequest(t, "conditional-"+key, candidate),
		initialRevision: 2,
	}
}

func newContextSliceCatalogOwnershipFixture(
	t *testing.T,
	key string,
) conditionalOwnershipFixture {
	t.Helper()
	fixture := newGenericMixedStoreFixture(t)
	assertion := mustGenericAssertionID(t, "assertion:catalog-ownership")
	gamma, err := typedmemory.NewGammaPoint(time.Date(2026, 7, 17, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewGammaPoint(catalog ownership): %v", err)
	}
	contextSlice, err := typedmemory.NewContextSlice(typedmemory.ContextSliceInput{
		Context:   fixture.primary,
		GammaTime: gamma,
	})
	if err != nil {
		t.Fatalf("NewContextSlice(catalog ownership): %v", err)
	}
	value, err := typedmemory.NewTypedValueCandidate(
		fixture.textKind,
		fixture.shape,
		fixture.codec,
		[]byte("catalog ownership payload"),
		typedmemory.NoAssertedDigest{},
	)
	if err != nil {
		t.Fatalf("NewTypedValueCandidate(catalog ownership): %v", err)
	}
	filler, err := typedmemory.NewByValueCandidate(value)
	if err != nil {
		t.Fatalf("NewByValueCandidate(catalog ownership): %v", err)
	}
	binding, err := typedmemory.NewCandidateSlotBinding(
		fixture.payloadSlot,
		[]typedmemory.CandidateSlotFiller{filler},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding(catalog ownership): %v", err)
	}
	relation, err := typedmemory.NewRelationalAssertionCandidate(
		typedmemory.RelationalAssertionCandidateInput{
			Assertion:  assertion,
			Signature:  fixture.signature,
			Slice:      contextSlice,
			Modality:   typedmemory.NewAffirmsObtaining(),
			Bindings:   []typedmemory.CandidateSlotBinding{binding},
			Provenance: mustGenericProvenanceRef(t, "memory:test:catalog-ownership"),
		},
	)
	if err != nil {
		t.Fatalf("NewRelationalAssertionCandidate(catalog ownership): %v", err)
	}
	change, err := typedmemory.NewAssertRelation(relation)
	if err != nil {
		t.Fatalf("NewAssertRelation(catalog ownership): %v", err)
	}
	candidate, err := typedmemory.NewMemoryChangeSet([]typedmemory.MemoryChange{change})
	if err != nil {
		t.Fatalf("NewMemoryChangeSet(catalog ownership): %v", err)
	}
	request := fixture.requestAt(
		t,
		2,
		"conditional-"+key,
		candidate,
		func(snapshot *genericMixedSnapshot) {
			snapshot.assertionAbsent(t, assertion)
		},
	)
	return conditionalOwnershipFixture{
		database:        fixture.base.database,
		adapter:         fixture.adapter,
		project:         fixture.base.project,
		request:         request,
		initialRevision: 2,
	}
}

func dropConditionalOwnershipGuard(
	t *testing.T,
	database *sql.DB,
	trigger string,
) {
	t.Helper()
	if _, err := database.Exec("DROP TRIGGER " + trigger); err != nil {
		t.Fatalf("drop %s: %v", trigger, err)
	}
}

func installConditionalOwnershipFaultTrigger(
	t *testing.T,
	database *sql.DB,
	fault conditionalOwnershipFault,
) {
	t.Helper()
	statement := fmt.Sprintf(
		`CREATE TRIGGER conditional_ownership_test_%s
		AFTER INSERT ON typed_memory_commit_materialization_closures
		BEGIN
			%s
		END`,
		fault.name,
		fault.triggerUpdate,
	)
	if _, err := database.Exec(statement); err != nil {
		t.Fatalf("install %s ownership fault: %v", fault.name, err)
	}
}

func assertConditionalOwnershipHead(
	t *testing.T,
	fixture conditionalOwnershipFixture,
) {
	t.Helper()
	head, err := fixture.adapter.LoadHead(context.Background(), fixture.project)
	if err != nil {
		t.Fatalf("LoadHead after rejected conditional ownership drift: %v", err)
	}
	if head.Revision().Value() != fixture.initialRevision {
		t.Fatalf(
			"graph revision after rejected conditional ownership drift = %d; want %d",
			head.Revision().Value(),
			fixture.initialRevision,
		)
	}
}
