package typedmemorystore

import (
	"context"
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectgraphobservation"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestCurrentProjectGraphObservationPreservesLegacyAndV3AssertionCarriers(
	t *testing.T,
) {
	mixed := newMixedLegacyAndV3AssertionFixture(t)
	fixture := mixed.fixture
	legacyID := mixed.legacyID
	v3ID := mixed.v3ID

	active := loadCurrentActiveAssertionsForTest(t, fixture)
	if len(active) != 2 {
		t.Fatalf("mixed active assertion count = %d; want 2", len(active))
	}
	byID := make(map[string]CurrentActiveAssertion, len(active))
	for _, assertion := range active {
		byID[assertion.AssertionID().String()] = assertion
	}
	assertExactLegacyCurrentCarrier(t, byID[legacyID.String()])
	assertExactV3CurrentCarrier(t, byID[v3ID.String()])

	reason, err := typedmemory.NewRetractionReason("historical assertion is superseded")
	if err != nil {
		t.Fatalf("NewRetractionReason: %v", err)
	}
	retraction, err := typedmemory.NewRetractAssertion(
		legacyID,
		reason,
		mustGenericProvenanceRef(t, "memory:test:retract-historical-legacy"),
	)
	if err != nil {
		t.Fatalf("NewRetractAssertion: %v", err)
	}
	retractionCandidate := mustMemoryChangeSet(t, retraction)
	retractionRequest := fixture.requestAt(
		t,
		2,
		"current-v2:retract-legacy",
		retractionCandidate,
		func(snapshot *genericMixedSnapshot) {
			snapshot.assertionActive(t, legacyID)
		},
	)
	if _, err := fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		retractionRequest,
	); err != nil {
		t.Fatalf("retract historical legacy assertion: %v", err)
	}

	survivors := loadCurrentActiveAssertionsForTest(t, fixture)
	if len(survivors) != 1 || survivors[0].AssertionID() != v3ID {
		t.Fatalf("active assertions after legacy retraction = %#v; want only %s", survivors, v3ID)
	}
	assertExactV3CurrentCarrier(t, survivors[0])
}

func TestCurrentRelationOriginsRejectCrossLaneDuplicateAssertionIdentity(
	t *testing.T,
) {
	mixed := newMixedLegacyAndV3AssertionFixture(t)
	ctx := context.Background()
	database := mixed.fixture.base.database
	connection, err := database.Conn(ctx)
	if err != nil {
		t.Fatalf("open cross-lane corruption fixture: %v", err)
	}
	closed := false
	defer func() {
		if closed {
			return
		}
		_, _ = connection.ExecContext(ctx, "PRAGMA foreign_keys = ON")
		_ = connection.Close()
	}()
	if _, err := connection.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable cross-lane fixture foreign keys: %v", err)
	}
	const updateGuard = "typed_memory_relational_assertions_v3_v53_no_update"
	var exactGuardSQL string
	err = connection.QueryRowContext(
		ctx,
		"SELECT sql FROM sqlite_master WHERE type = 'trigger' AND name = ?",
		updateGuard,
	).Scan(&exactGuardSQL)
	if err != nil || exactGuardSQL == "" {
		t.Fatalf("load exact v3 update guard: %v", err)
	}
	if _, err := connection.ExecContext(ctx, "DROP TRIGGER "+updateGuard); err != nil {
		t.Fatalf("open cross-lane assertion fixture seam: %v", err)
	}
	result, err := connection.ExecContext(
		ctx,
		`UPDATE typed_memory_relational_assertions_v3
		SET assertion_id = ?
		WHERE project_id = ? AND assertion_id = ?`,
		mixed.legacyID.String(),
		mixed.fixture.base.project.String(),
		mixed.v3ID.String(),
	)
	if err != nil {
		t.Fatalf("inject cross-lane assertion identity: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "cross-lane v3 assertion origins")
	if _, err := connection.ExecContext(ctx, exactGuardSQL); err != nil {
		t.Fatalf("restore exact v3 update guard: %v", err)
	}
	var restoredGuardSQL string
	err = connection.QueryRowContext(
		ctx,
		"SELECT sql FROM sqlite_master WHERE type = 'trigger' AND name = ?",
		updateGuard,
	).Scan(&restoredGuardSQL)
	if err != nil || restoredGuardSQL != exactGuardSQL {
		t.Fatalf("v3 update guard was not restored byte-identically: %v", err)
	}
	if _, err := connection.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("restore cross-lane fixture foreign keys: %v", err)
	}
	if err := connection.Close(); err != nil {
		t.Fatalf("close cross-lane corruption fixture: %v", err)
	}
	closed = true

	transaction, err := sqlitetransaction.BeginRead(ctx, database)
	if err != nil {
		t.Fatalf("BeginRead(cross-lane): %v", err)
	}
	head, err := loadHeadWithScanner(ctx, transaction, mixed.fixture.base.project)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("load cross-lane graph head: %v", err)
	}
	_, err = loadCurrentRelationOrigins(ctx, transaction, head)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		_ = transaction.Rollback(ctx)
		t.Fatalf("cross-lane duplicate error = %v; want ErrStoredAdmissionIntegrity", err)
	}
	if result := transaction.Rollback(ctx); !result.Succeeded() {
		t.Fatalf("Rollback(cross-lane): %v", result.Err())
	}
}

type mixedLegacyAndV3AssertionFixture struct {
	fixture  genericMixedStoreFixture
	legacyID typedmemory.AssertionID
	v3ID     typedmemory.AssertionID
}

func newMixedLegacyAndV3AssertionFixture(
	t *testing.T,
) mixedLegacyAndV3AssertionFixture {
	t.Helper()
	fixture := newFrozenLegacyV1GenericMixedStoreFixture(t)
	legacyID := mustGenericAssertionID(t, "assertion:historical-legacy")

	v3ID := mustGenericAssertionID(t, "assertion:current-v3")
	v3Change := fixture.relationChange(
		t,
		v3ID,
		"current v3 payload",
		"memory:test:current-v3",
	)
	v3Candidate := mustMemoryChangeSet(t, v3Change)
	v3Request := fixture.requestAt(
		t,
		1,
		"current-v2:v3-assertion",
		v3Candidate,
		func(snapshot *genericMixedSnapshot) {
			snapshot.assertionAbsent(t, v3ID)
		},
	)
	if _, err := fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		v3Request,
	); err != nil {
		t.Fatalf("commit fresh v3 assertion: %v", err)
	}
	return mixedLegacyAndV3AssertionFixture{
		fixture:  fixture,
		legacyID: legacyID,
		v3ID:     v3ID,
	}
}

func mustMemoryChangeSet(
	t *testing.T,
	changes ...typedmemory.MemoryChange,
) typedmemory.MemoryChangeSet {
	t.Helper()
	candidate, err := typedmemory.NewMemoryChangeSet(changes)
	if err != nil {
		t.Fatalf("NewMemoryChangeSet: %v", err)
	}
	return candidate
}

func loadCurrentActiveAssertionsForTest(
	t *testing.T,
	fixture genericMixedStoreFixture,
) []CurrentActiveAssertion {
	t.Helper()
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.base.database)
	if err != nil {
		t.Fatalf("BeginRead: %v", err)
	}
	observation, err := LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		fixture.base.project,
	)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("LoadCurrentGraphRevalidationBasisTx: %v", err)
	}
	active := observation.ActiveAssertions().Relations()
	if result := transaction.Rollback(ctx); !result.Succeeded() {
		t.Fatalf("Rollback: %v", result.Err())
	}
	return active
}

func assertExactLegacyCurrentCarrier(
	t *testing.T,
	assertion CurrentActiveAssertion,
) {
	t.Helper()
	if assertion.Carrier().Kind() != projectgraphobservation.CurrentLegacyRelationCarrier {
		t.Fatalf("legacy carrier kind = %q", assertion.Carrier().Kind())
	}
	if _, ok := assertion.LegacyRelation(); !ok {
		t.Fatal("legacy current assertion lost its exact RelationInstance carrier")
	}
	if _, ok := assertion.RelationalAssertion(); ok {
		t.Fatal("legacy current assertion was reinterpreted as a v3 assertion")
	}
	if modality, ok := assertion.Posture().ExplicitModality(); ok {
		t.Fatalf("legacy current assertion acquired v3 modality %q", modality)
	}
}

func assertExactV3CurrentCarrier(
	t *testing.T,
	assertion CurrentActiveAssertion,
) {
	t.Helper()
	if assertion.Carrier().Kind() != projectgraphobservation.CurrentRelationalAssertionV3Carrier {
		t.Fatalf("v3 carrier kind = %q", assertion.Carrier().Kind())
	}
	if _, ok := assertion.LegacyRelation(); ok {
		t.Fatal("v3 current assertion was coerced into a legacy RelationInstance")
	}
	current, ok := assertion.RelationalAssertion()
	if !ok {
		t.Fatal("v3 current assertion lost its exact RelationalAssertion carrier")
	}
	modality, ok := assertion.Posture().ExplicitModality()
	if !ok || modality != typedmemory.AssertionModalityAffirmsObtaining {
		t.Fatalf("v3 explicit modality = (%q, %v); want affirms_obtaining", modality, ok)
	}
	if current.Modality().Kind() != modality {
		t.Fatal("v3 posture modality differs from the exact assertion carrier")
	}
}
