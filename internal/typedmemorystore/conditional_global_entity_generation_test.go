package typedmemorystore

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestConditionalGlobalEntityOwnerRejectsHistoricalV46TripleLoss(
	t *testing.T,
) {
	fixture := newGenericMixedStoreFixture(t)
	candidate := fixture.finalCandidate(t, "Generation owner", "generation owner payload")
	request := fixture.finalRequest(t, "conditional-owner-generation", candidate)
	prepared, err := prepareGenericAdmission(request)
	if err != nil {
		t.Fatalf("prepare v46 declaration admission: %v", err)
	}
	expected, err := buildExpectedMaterializationManifest(prepared)
	if err != nil {
		t.Fatalf("build v46 declaration manifest: %v", err)
	}
	globalCandidate := expectedGlobalEntityCandidate(t, expected, "entity:new")
	ownerReceipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("commit v46 declaration owner: %v", err)
	}

	laterAlias := mustGenericAlias(t, "post-declaration-owner-check")
	laterChange := fixture.admitAliasChange(
		t,
		laterAlias,
		"memory:test:post-declaration-owner-check",
	)
	laterCandidate, err := typedmemory.NewMemoryChangeSet(
		[]typedmemory.MemoryChange{laterChange},
	)
	if err != nil {
		t.Fatalf("build later conditional candidate: %v", err)
	}
	laterRequest := fixture.requestAt(
		t,
		ownerReceipt.GraphRevision().Value(),
		"conditional-owner-generation-later",
		laterCandidate,
		func(snapshot *genericMixedSnapshot) {
			snapshot.entityExact(t, fixture.anchor, fixture.primary)
			snapshot.aliasUnbound(t, laterAlias, fixture.primary)
		},
	)
	laterReceipt, err := fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		laterRequest,
	)
	if err != nil {
		t.Fatalf("commit later conditional event: %v", err)
	}
	laterExpected := expectedMaterializationManifest{
		basisRevision: ownerReceipt.GraphRevision().Value(),
		semanticRows:  []expectedSemanticRowIdentity{globalCandidate},
	}
	source := databaseScanner{database: fixture.base.database}
	if err := verifyConditionalSemanticRows(
		context.Background(),
		source,
		fixture.base.project,
		laterReceipt.EventRef(),
		laterExpected,
	); err != nil {
		t.Fatalf("verify intact historical v46 owner: %v", err)
	}

	removeV46DeclarationGeneration(
		t,
		fixture.base.database,
		fixture.base.project.String(),
		ownerReceipt.EventRef(),
		"entity:new",
	)

	err = verifyConditionalSemanticRows(
		context.Background(),
		source,
		fixture.base.project,
		laterReceipt.EventRef(),
		laterExpected,
	)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf(
			"historical v46 triple-loss error = %v; want ErrStoredAdmissionIntegrity",
			err,
		)
	}
}

func TestConditionalGlobalEntityOwnerRequiresP9ImportForLegacyShapedV46State(
	t *testing.T,
) {
	fixture := newSQLiteStoreFixture(t)
	candidate := fixture.declaration(t, "legacy-owner", "Legacy owner")
	request := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"declare:legacy-owner",
		candidate,
	)
	receipt, err := fixture.adapter.commitDeclareEntity(context.Background(), request)
	if err != nil {
		t.Fatalf("commit legacy declaration owner: %v", err)
	}
	declaration, ok := candidate.Changes()[0].(typedmemory.DeclareEntity)
	if !ok {
		t.Fatalf("legacy candidate change = %T; want DeclareEntity", candidate.Changes()[0])
	}
	availability, err := loadGenericStorageAvailability(
		context.Background(),
		databaseScanner{database: fixture.database},
	)
	if err != nil {
		t.Fatalf("load upgraded storage capability: %v", err)
	}
	if availability != genericStorageExact {
		t.Fatalf("storage availability = %d; want exact v46", availability)
	}
	removeV46DeclarationGeneration(
		t,
		fixture.database,
		fixture.project.String(),
		receipt.EventRef(),
		declaration.Entity().String(),
	)

	_, found, err := loadPriorGlobalEntityOwner(
		context.Background(),
		databaseScanner{database: fixture.database},
		fixture.project,
		declaration.Entity().String(),
		receipt.GraphRevision().Value(),
	)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf(
			"legacy-shaped v46 owner error = %v; want ErrStoredAdmissionIntegrity pending P9 import",
			err,
		)
	}
	if found {
		t.Fatal("legacy-shaped state in an exact v46 store was accepted without P9 import")
	}
}

func removeV46DeclarationGeneration(
	t *testing.T,
	database *sql.DB,
	projectID string,
	eventRef string,
	entityID string,
) {
	t.Helper()
	dropV46DeleteGuard(
		t,
		database,
		"typed_memory_commit_materialization_closures_v46_no_delete",
	)
	dropV46DeleteGuard(
		t,
		database,
		"typed_memory_event_admission_bases_v46_no_delete",
	)
	dropV46DeleteGuard(
		t,
		database,
		"typed_memory_entity_declarations_v46_no_delete",
	)
	closureResult, err := database.Exec(
		`DELETE FROM typed_memory_commit_materialization_closures
		 WHERE project_id = ? AND event_ref = ?`,
		projectID,
		eventRef,
	)
	if err != nil {
		t.Fatalf("delete historical v46 closure witness: %v", err)
	}
	assertExactBasisRowsAffected(t, closureResult, 1, "historical v46 closure witness")
	admissionResult, err := database.Exec(
		`DELETE FROM typed_memory_event_admission_bases
		 WHERE project_id = ? AND event_ref = ?`,
		projectID,
		eventRef,
	)
	if err != nil {
		t.Fatalf("delete historical v46 admission witness: %v", err)
	}
	assertExactBasisRowsAffected(t, admissionResult, 1, "historical v46 admission witness")
	declarationResult, err := database.Exec(
		`DELETE FROM typed_memory_entity_declarations
		 WHERE project_id = ? AND event_ref = ? AND entity_id = ?`,
		projectID,
		eventRef,
		entityID,
	)
	if err != nil {
		t.Fatalf("delete historical v46 declaration witness: %v", err)
	}
	assertExactBasisRowsAffected(
		t,
		declarationResult,
		1,
		"historical v46 declaration witness",
	)
}

func dropV46DeleteGuard(t *testing.T, database *sql.DB, trigger string) {
	t.Helper()
	if _, err := database.Exec("DROP TRIGGER " + trigger); err != nil {
		t.Fatalf("drop %s: %v", trigger, err)
	}
}

func expectedGlobalEntityCandidate(
	t *testing.T,
	manifest expectedMaterializationManifest,
	entityID string,
) expectedSemanticRowIdentity {
	t.Helper()
	for _, row := range manifest.semanticRows {
		matches := row.rowKind == "global_entity_candidate" &&
			len(row.coordinate) == 1 &&
			row.coordinate[0] == entityID
		if matches {
			return row
		}
	}
	t.Fatalf("manifest has no global entity candidate for %q", entityID)
	return expectedSemanticRowIdentity{}
}
