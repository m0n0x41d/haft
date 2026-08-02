package typedmemorystore

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// These cases complement generic_decomposed_projection_integrity_test.go.
// Every mutation preserves the sealed carriers and their digests while
// changing one query-visible projection, or a correlated set of projections.
// The exact materialization verifier must therefore reject the row set both
// before COMMIT and during durable replay.
func TestGenericCommitRejectsSupplementalProjectedColumnDriftBeforeCommit(
	t *testing.T,
) {
	for _, fault := range supplementalProjectedColumnFaults() {
		t.Run(fault.name, func(t *testing.T) {
			t.Parallel()

			fixture := fault.newFixture(t, "same-tx-"+fault.name)
			allowDecomposedProjectionMutation(t, fixture.database, fault.updates)
			installDecomposedProjectionFaultTrigger(t, fixture.database, fault)

			_, err := fixture.adapter.CommitMemoryChangeSet(
				context.Background(),
				fixture.request,
			)
			if !errors.Is(err, ErrStoredAdmissionIntegrity) {
				t.Fatalf(
					"same-transaction supplemental projection drift error = %v; want ErrStoredAdmissionIntegrity",
					err,
				)
			}
			if errors.Is(err, ErrCommitOutcomeUnknown) {
				t.Fatalf("supplemental projection drift was first detected after COMMIT: %v", err)
			}
			assertDecomposedProjectionHead(t, fixture)
		})
	}
}

func TestGenericReplayRejectsSupplementalDurableProjectedColumnDrift(
	t *testing.T,
) {
	for _, fault := range supplementalProjectedColumnFaults() {
		t.Run(fault.name, func(t *testing.T) {
			t.Parallel()

			fixture := fault.newFixture(t, "replay-"+fault.name)
			receipt, err := fixture.adapter.CommitMemoryChangeSet(
				context.Background(),
				fixture.request,
			)
			if err != nil {
				t.Fatalf("seed supplemental exact materialization: %v", err)
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
					"durable supplemental projection drift error = %v; want ErrStoredAdmissionIntegrity",
					err,
				)
			}
			if errors.Is(err, ErrIdempotencyConflict) {
				t.Fatalf("stored supplemental drift was misclassified as caller conflict: %v", err)
			}
		})
	}
}

func supplementalProjectedColumnFaults() []decomposedProjectionColumnFault {
	return []decomposedProjectionColumnFault{
		{
			name:       "relation_context_slice_ref",
			newFixture: newTwoSliceProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table: "typed_memory_relational_assertions_v3",
					assignment: `context_slice_ref = (
						SELECT peer.context_slice_ref
						FROM typed_memory_relational_assertions_v3 peer
						WHERE peer.project_id = typed_memory_relational_assertions_v3.project_id
							AND peer.event_ref = typed_memory_relational_assertions_v3.event_ref
							AND peer.change_ordinal = 1
					)`,
					predicate:    " AND change_ordinal = 0",
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
					assignment:   "reference_kind_ref = 'typeenv:projection-drift/ref-kind/U.WrongRef'",
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
					assignment:   "required_value_kind_ref = 'typeenv:projection-drift/kind/U.WrongKind'",
					predicate:    " AND filler_kind = 'by_reference'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "reference_filler_discriminant_to_by_value",
			newFixture: newReferenceProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table: "typed_memory_relational_assertion_fillers_v3",
					assignment: `filler_kind = 'by_value',
						reference_kind_ref = '', reference_id = '', entity_id = '',
						required_value_kind_ref = '', value_ref = 'value:projection-drift'`,
					predicate:    " AND filler_kind = 'by_reference'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "correlated_value_blob_and_filler_value_ref",
			newFixture: newValueProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_value_blobs",
					assignment:   "value_ref = 'value:correlated-projection-drift'",
					wantAffected: 1,
				},
				{
					table:        "typed_memory_relational_assertion_fillers_v3",
					assignment:   "value_ref = 'value:correlated-projection-drift'",
					predicate:    " AND filler_kind = 'by_value'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "alias_change_ref",
			newFixture: newValueProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_alias_changes",
					assignment:   "alias_change_ref = 'alias-change:projection-drift'",
					predicate:    " AND change_kind = 'admit_alias'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "alias_bounded_context_ref",
			newFixture: newValueProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_alias_changes",
					assignment:   "bounded_context_ref = 'ctx:projection-drift'",
					predicate:    " AND change_kind = 'admit_alias'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "alias_name",
			newFixture: newValueProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_alias_changes",
					assignment:   "alias = 'projection-drift-alias'",
					predicate:    " AND change_kind = 'admit_alias'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "alias_entity_id",
			newFixture: newValueProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_alias_changes",
					assignment:   "entity_id = 'entity:projection-drift'",
					predicate:    " AND change_kind = 'admit_alias'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "alias_replacement_alias",
			newFixture: newValueProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_alias_changes",
					assignment:   "replacement_alias = 'projection-drift-replacement'",
					predicate:    " AND change_kind = 'supersede_alias'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "alias_supersedes_alias_change_ref",
			newFixture: newValueProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_alias_changes",
					assignment:   "supersedes_alias_change_ref = 'alias-change:projection-drift/prior'",
					predicate:    " AND change_kind = 'supersede_alias'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "alias_discriminant_to_admit",
			newFixture: newValueProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table: "typed_memory_alias_changes",
					assignment: `change_kind = 'admit_alias',
						replacement_alias = NULL, supersedes_alias_change_ref = ''`,
					predicate:    " AND change_kind = 'supersede_alias'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "retraction_ref",
			newFixture: newValueProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_assertion_retractions",
					assignment:   "retraction_ref = 'retraction:projection-drift'",
					wantAffected: 1,
				},
			},
		},
		{
			name:       "retraction_assertion_id",
			newFixture: newValueProjectionTestFixture,
			updates: []decomposedProjectionColumnUpdate{
				{
					table:        "typed_memory_assertion_retractions",
					assignment:   "assertion_id = 'assertion:projection-drift'",
					wantAffected: 1,
				},
			},
		},
	}
}

func newTwoSliceProjectionTestFixture(
	t *testing.T,
	key string,
) decomposedProjectionTestFixture {
	t.Helper()
	fixture := newGenericMixedStoreFixture(t)
	secondSlice := supplementalContextSlice(t, fixture.primary)
	firstAssertion := mustGenericAssertionID(t, "assertion:projection-context-primary")
	secondAssertion := mustGenericAssertionID(t, "assertion:projection-context-secondary")
	first := fixture.relationChange(
		t,
		firstAssertion,
		"primary context-slice payload",
		"memory:test:projection-context-primary",
	)
	second := supplementalRelationChange(
		t,
		fixture,
		secondAssertion,
		secondSlice,
		"secondary context-slice payload",
		"memory:test:projection-context-secondary",
	)
	candidate, err := typedmemory.NewMemoryChangeSet([]typedmemory.MemoryChange{
		first,
		second,
	})
	if err != nil {
		t.Fatalf("two-slice NewMemoryChangeSet: %v", err)
	}
	request := fixture.requestAt(
		t,
		2,
		"supplemental-projection-"+key,
		candidate,
		func(snapshot *genericMixedSnapshot) {
			snapshot.assertionAbsent(t, firstAssertion)
			snapshot.assertionAbsent(t, secondAssertion)
		},
	)
	return decomposedProjectionTestFixture{
		database:        fixture.base.database,
		adapter:         fixture.adapter,
		project:         fixture.base.project,
		request:         request,
		initialRevision: 2,
	}
}

func supplementalContextSlice(
	t *testing.T,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.ContextSlice {
	t.Helper()
	gamma, err := typedmemory.NewGammaPoint(
		time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("supplemental NewGammaPoint: %v", err)
	}
	slice, err := typedmemory.NewContextSlice(typedmemory.ContextSliceInput{
		Context:   contextRef,
		GammaTime: gamma,
	})
	if err != nil {
		t.Fatalf("supplemental NewContextSlice: %v", err)
	}
	return slice
}

func supplementalRelationChange(
	t *testing.T,
	fixture genericMixedStoreFixture,
	assertion typedmemory.AssertionID,
	slice typedmemory.ContextSlice,
	payload string,
	provenance string,
) typedmemory.AssertRelation {
	t.Helper()
	value, err := typedmemory.NewTypedValueCandidate(
		fixture.textKind,
		fixture.shape,
		fixture.codec,
		[]byte(payload),
		typedmemory.NoAssertedDigest{},
	)
	if err != nil {
		t.Fatalf("supplemental NewTypedValueCandidate: %v", err)
	}
	filler, err := typedmemory.NewByValueCandidate(value)
	if err != nil {
		t.Fatalf("supplemental NewByValueCandidate: %v", err)
	}
	binding, err := typedmemory.NewCandidateSlotBinding(
		fixture.payloadSlot,
		[]typedmemory.CandidateSlotFiller{filler},
	)
	if err != nil {
		t.Fatalf("supplemental NewCandidateSlotBinding: %v", err)
	}
	relation, err := typedmemory.NewRelationalAssertionCandidate(
		typedmemory.RelationalAssertionCandidateInput{
			Assertion:  assertion,
			Signature:  fixture.signature,
			Slice:      slice,
			Modality:   typedmemory.NewAffirmsObtaining(),
			Bindings:   []typedmemory.CandidateSlotBinding{binding},
			Provenance: mustGenericProvenanceRef(t, provenance),
		},
	)
	if err != nil {
		t.Fatalf("supplemental NewRelationalAssertionCandidate: %v", err)
	}
	change, err := typedmemory.NewAssertRelation(relation)
	if err != nil {
		t.Fatalf("supplemental NewAssertRelation: %v", err)
	}
	return change
}

func TestGenericCommitRejectsEntityContextRevisionDriftBeforeCommit(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	candidate := fixture.finalCandidate(t, "Context revision", "context revision payload")
	request := fixture.finalRequest(t, "entity-context-revision-same-tx", candidate)
	allowSupplementalLegacyMutation(t, fixture.base.database, "typed_memory_entity_contexts")
	_, err := fixture.base.database.Exec(`CREATE TRIGGER supplemental_entity_context_revision_drift
		AFTER INSERT ON typed_memory_commit_materialization_closures
		BEGIN
			UPDATE typed_memory_entity_contexts
			SET declared_revision = declared_revision + 100
			WHERE project_id = NEW.project_id AND declared_event_ref = NEW.event_ref;
		END`)
	if err != nil {
		t.Fatalf("install entity-context revision fault: %v", err)
	}

	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("same-transaction entity-context revision error = %v; want ErrStoredAdmissionIntegrity", err)
	}
	if errors.Is(err, ErrCommitOutcomeUnknown) {
		t.Fatalf("entity-context revision drift was first detected after COMMIT: %v", err)
	}
	assertSupplementalHeadRevision(t, fixture.adapter, fixture.base.project.String(), 2)
}

func TestGenericReplayRejectsEntityContextRevisionDrift(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	candidate := fixture.finalCandidate(t, "Context revision", "context revision payload")
	request := fixture.finalRequest(t, "entity-context-revision-replay", candidate)
	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("seed entity-context revision materialization: %v", err)
	}
	allowSupplementalLegacyMutation(t, fixture.base.database, "typed_memory_entity_contexts")
	result, err := fixture.base.database.Exec(`UPDATE typed_memory_entity_contexts
		SET declared_revision = declared_revision + 100
		WHERE project_id = ? AND declared_event_ref = ?`,
		fixture.base.project.String(),
		receipt.EventRef(),
	)
	if err != nil {
		t.Fatalf("inject durable entity-context revision drift: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "entity-context revision rows")

	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("durable entity-context revision error = %v; want ErrStoredAdmissionIntegrity", err)
	}
	if errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("entity-context revision drift was misclassified as caller conflict: %v", err)
	}
}

func TestConditionalGlobalEntityOwnerComesFromDeclarationHistory(t *testing.T) {
	fixture, expected, receipt := supplementalCommittedMixedManifest(
		t,
		"conditional-global-owner",
	)
	allowSupplementalLegacyMutation(t, fixture.base.database, "typed_memory_entities")
	var firstEventRef string
	err := fixture.base.database.QueryRow(
		`SELECT event_ref FROM typed_memory_graph_events
		WHERE project_id = ? AND graph_revision = 1`,
		fixture.base.project.String(),
	).Scan(&firstEventRef)
	if err != nil {
		t.Fatalf("load first declaration event: %v", err)
	}
	result, err := fixture.base.database.Exec(`UPDATE typed_memory_entities
		SET first_declared_event_ref = ?, first_declared_revision = 1
		WHERE project_id = ? AND entity_id = 'entity:new'`,
		firstEventRef,
		fixture.base.project.String(),
	)
	if err != nil {
		t.Fatalf("inject wrong global entity owner: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "mis-owned global entities")

	err = verifyConditionalSemanticRows(
		context.Background(),
		supplementalSQLScanner{database: fixture.base.database},
		fixture.base.project,
		receipt.EventRef(),
		expected,
	)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("conditional global owner verification = %v; want ErrStoredAdmissionIntegrity", err)
	}
}

func TestConditionalContextSliceCatalogOwnerComesFromCommittedUseHistory(t *testing.T) {
	fixture, expected, receipt := supplementalCommittedMixedManifest(
		t,
		"conditional-catalog-owner",
	)
	allowSupplementalV46Mutation(
		t,
		fixture.base.database,
		"typed_memory_context_slice_catalog",
	)
	result, err := fixture.base.database.Exec(`UPDATE typed_memory_context_slice_catalog
		SET event_ref = ?
		WHERE project_id = ? AND context_slice_ref = ?`,
		receipt.EventRef(),
		fixture.base.project.String(),
		fixture.contextSlice.Ref().String(),
	)
	if err != nil {
		t.Fatalf("inject stolen ContextSlice catalog owner: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "mis-owned ContextSlice catalog rows")

	err = verifyConditionalSemanticRows(
		context.Background(),
		supplementalSQLScanner{database: fixture.base.database},
		fixture.base.project,
		receipt.EventRef(),
		expected,
	)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("conditional catalog owner verification = %v; want ErrStoredAdmissionIntegrity", err)
	}
}

func supplementalCommittedMixedManifest(
	t *testing.T,
	key string,
) (
	genericMixedStoreFixture,
	expectedMaterializationManifest,
	CommitReceipt,
) {
	t.Helper()
	fixture := newGenericMixedStoreFixture(t)
	candidate := fixture.finalCandidate(t, "Conditional owner", "conditional owner payload")
	request := fixture.finalRequest(t, key, candidate)
	prepared, err := prepareGenericAdmission(request)
	if err != nil {
		t.Fatalf("prepare conditional-owner admission: %v", err)
	}
	expected, err := buildExpectedMaterializationManifest(prepared)
	if err != nil {
		t.Fatalf("build conditional-owner manifest: %v", err)
	}
	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("seed conditional-owner materialization: %v", err)
	}
	return fixture, expected, receipt
}

func allowSupplementalV46Mutation(t *testing.T, database *sql.DB, table string) {
	t.Helper()
	trigger := table + "_v46_no_update"
	if _, err := database.Exec("DROP TRIGGER " + trigger); err != nil {
		t.Fatalf("drop %s: %v", trigger, err)
	}
}

func allowSupplementalLegacyMutation(t *testing.T, database *sql.DB, table string) {
	t.Helper()
	trigger := table + "_no_update"
	if _, err := database.Exec("DROP TRIGGER " + trigger); err != nil {
		t.Fatalf("drop %s: %v", trigger, err)
	}
}

func assertSupplementalHeadRevision(
	t *testing.T,
	adapter *SQLiteAdapter,
	projectText string,
	want uint64,
) {
	t.Helper()
	project := mustProjectID(t, projectText)
	head, err := adapter.LoadHead(context.Background(), project)
	if err != nil {
		t.Fatalf("LoadHead after supplemental rejection: %v", err)
	}
	if head.Revision().Value() != want {
		t.Fatalf("graph revision after supplemental rejection = %d; want %d", head.Revision().Value(), want)
	}
}

type supplementalSQLScanner struct {
	database *sql.DB
}

func (source supplementalSQLScanner) ScanOne(
	ctx context.Context,
	statement string,
	args []any,
	dest []any,
) error {
	return source.database.QueryRowContext(ctx, statement, args...).Scan(dest...)
}

var _ scanner = supplementalSQLScanner{}
