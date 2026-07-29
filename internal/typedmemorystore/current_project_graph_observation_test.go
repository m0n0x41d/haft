package typedmemorystore

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestCurrentProjectGraphObservationLoadsExactClosureAndActiveRelations(
	t *testing.T,
) {
	fixture := newGenericMixedStoreFixture(t)
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
	basis := observation.GraphSnapshotBasis()
	if basis.Project() != fixture.base.project {
		t.Fatalf(
			"basis project = %q; want %q",
			basis.Project().String(),
			fixture.base.project.String(),
		)
	}
	if basis.GraphRevision().Value() != 2 {
		t.Fatalf("basis graph revision = %d; want 2", basis.GraphRevision().Value())
	}
	closure, ok := basis.Closure().(projecttypeenvselection.CommittedProjectGraphClosure)
	if !ok {
		t.Fatalf("basis closure = %T; want CommittedProjectGraphClosure", basis.Closure())
	}
	if closure.Event().String() == "" ||
		closure.Commit().String() == "" ||
		closure.MaterializationDigest().String() == "" {
		t.Fatal("committed graph closure lost an exact coordinate")
	}
	var expectedEvent string
	var expectedCommit string
	var expectedMaterializationDigest string
	err = transaction.ScanOne(
		ctx,
		`SELECT head.last_event_ref, head.last_commit_ref,
			closure.materialization_digest
		FROM typed_memory_graph_heads head
		JOIN typed_memory_commit_materialization_closures closure
			ON closure.project_id = head.project_id
			AND closure.event_ref = head.last_event_ref
			AND closure.commit_ref = head.last_commit_ref
		WHERE head.project_id = ?`,
		[]any{fixture.base.project.String()},
		[]any{
			&expectedEvent,
			&expectedCommit,
			&expectedMaterializationDigest,
		},
	)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("load exact durable graph closure: %v", err)
	}
	if closure.Event().String() != expectedEvent ||
		closure.Commit().String() != expectedCommit ||
		closure.MaterializationDigest().String() !=
			expectedMaterializationDigest {
		t.Fatal("graph observation closure differs from the exact durable head closure")
	}
	if observation.ActiveTypeEnv() != fixture.environment.Ref() {
		t.Fatal("graph observation active TypeEnv differs from the durable head")
	}
	active := observation.ActiveAssertions()
	if active.Project() != fixture.base.project ||
		active.GraphRevision().Value() != 2 {
		t.Fatal("active assertion set is not correlated to the graph basis")
	}
	relations := active.Relations()
	if len(relations) != 1 {
		t.Fatalf("active relation count = %d; want 1", len(relations))
	}
	relation := relations[0]
	assertion, v3 := relation.RelationalAssertion()
	if relation.AssertionID() != fixture.oldAssertion ||
		!v3 ||
		assertion.Assertion() != fixture.oldAssertion {
		t.Fatal("active relation does not preserve the exact assertion identity")
	}
	if _, legacy := relation.LegacyRelation(); legacy {
		t.Fatal("v3 assertion was inferred to be a legacy relation occurrence")
	}
	if relation.OriginRevision().Value() != 2 ||
		relation.OriginEvent().String() != closure.Event().String() {
		t.Fatal("active relation lost its exact durable origin")
	}
	if relation.Posture().String() != "active_at_observed_revision" {
		t.Fatalf("active relation posture = %q", relation.Posture().String())
	}
	modality, explicit := relation.Posture().ExplicitModality()
	if !explicit || modality != typedmemory.AssertionModalityAffirmsObtaining {
		t.Fatalf(
			"active assertion modality = (%q, %t); want explicit affirms_obtaining",
			modality.String(),
			explicit,
		)
	}
	digest, err := assertion.Digest()
	if err != nil {
		t.Fatalf("RelationalAssertion.Digest: %v", err)
	}
	if digest != relation.Digest() {
		t.Fatal("active relation digest differs from its canonical relation")
	}

	relations[0] = CurrentActiveAssertion{}
	canonical := relation.CanonicalBytes()
	canonical[0] ^= 0xff
	reloaded := observation.ActiveAssertions().Relations()[0]
	if reloaded.AssertionID() != fixture.oldAssertion ||
		reloaded.CanonicalBytes()[0] == canonical[0] {
		t.Fatal("graph observation leaked mutable active-relation storage")
	}

	var count int64
	if err := transaction.ScanOne(
		ctx,
		`SELECT COUNT(*) FROM typed_memory_graph_heads WHERE project_id = ?`,
		[]any{fixture.base.project.String()},
		[]any{&count},
	); err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("caller transaction was finished by observation reader: %v", err)
	}
	if count != 1 {
		t.Fatalf("graph head count = %d; want 1", count)
	}
	if result := transaction.Rollback(ctx); !result.Succeeded() {
		t.Fatalf("Rollback: %v", result.Err())
	}
}

func TestCurrentProjectGraphObservationFiltersRetractedAssertions(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	candidate := fixture.finalCandidate(t, "Replacement entity", "replacement payload")
	request := fixture.finalRequest(t, "graph-observation-active-filter", candidate)
	if _, err := fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		request,
	); err != nil {
		t.Fatalf("CommitMemoryChangeSet: %v", err)
	}

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
	relations := observation.ActiveAssertions().Relations()
	newAssertion := mustGenericAssertionID(t, "assertion:new")
	if len(relations) != 1 || relations[0].AssertionID() != newAssertion {
		_ = transaction.Rollback(ctx)
		t.Fatalf(
			"active assertions = %#v; want only %s",
			relations,
			newAssertion.String(),
		)
	}
	if relations[0].AssertionID() == fixture.oldAssertion {
		_ = transaction.Rollback(ctx)
		t.Fatal("retracted assertion remained in the current active view")
	}
	if result := transaction.Rollback(ctx); !result.Succeeded() {
		t.Fatalf("Rollback: %v", result.Err())
	}
}

func TestCurrentProjectGraphObservationRepresentsRevisionZeroWithoutClosure(
	t *testing.T,
) {
	fixture := newSQLiteStoreFixture(t)
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead: %v", err)
	}
	observation, err := LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		fixture.project,
	)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("LoadCurrentGraphRevalidationBasisTx: %v", err)
	}
	basis := observation.GraphSnapshotBasis()
	if basis.GraphRevision().Value() != 0 {
		t.Fatalf("revision-zero basis = %d; want 0", basis.GraphRevision().Value())
	}
	if _, ok := basis.Closure().(projecttypeenvselection.EmptyProjectGraphClosure); !ok {
		t.Fatalf("revision-zero closure = %T; want EmptyProjectGraphClosure", basis.Closure())
	}
	if len(observation.ActiveAssertions().Relations()) != 0 {
		t.Fatal("revision-zero graph has active assertions")
	}
	if result := transaction.Rollback(ctx); !result.Succeeded() {
		t.Fatalf("Rollback: %v", result.Err())
	}
}

func TestCurrentActiveAssertionsRejectsAStaleGraphBasis(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	ctx := context.Background()
	beforeTransaction, err := sqlitetransaction.BeginRead(ctx, fixture.base.database)
	if err != nil {
		t.Fatalf("BeginRead(before): %v", err)
	}
	before, err := LoadCurrentGraphRevalidationBasisTx(
		ctx,
		beforeTransaction,
		fixture.base.project,
	)
	if err != nil {
		_ = beforeTransaction.Rollback(ctx)
		t.Fatalf("LoadCurrentGraphRevalidationBasisTx(before): %v", err)
	}
	if result := beforeTransaction.Rollback(ctx); !result.Succeeded() {
		t.Fatalf("Rollback(before): %v", result.Err())
	}

	candidate := fixture.finalCandidate(t, "New entity", "new payload")
	request := fixture.finalRequest(t, "graph-observation-stale-basis", candidate)
	if _, err := fixture.adapter.CommitMemoryChangeSet(ctx, request); err != nil {
		t.Fatalf("CommitMemoryChangeSet: %v", err)
	}

	afterTransaction, err := sqlitetransaction.BeginRead(ctx, fixture.base.database)
	if err != nil {
		t.Fatalf("BeginRead(after): %v", err)
	}
	_, err = LoadCurrentActiveAssertionsTx(
		ctx,
		afterTransaction,
		before.GraphSnapshotBasis(),
	)
	if !errors.Is(err, ErrStaleGraphRevision) {
		_ = afterTransaction.Rollback(ctx)
		t.Fatalf("stale graph basis error = %v; want ErrStaleGraphRevision", err)
	}
	if result := afterTransaction.Rollback(ctx); !result.Succeeded() {
		t.Fatalf("Rollback(after): %v", result.Err())
	}
}

func TestCurrentProjectGraphObservationRejectsProjectionDrift(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	updates := []decomposedProjectionColumnUpdate{
		{
			table:        "typed_memory_relational_assertions_v3",
			assignment:   "provenance_ref = 'memory:test:observation-drift'",
			wantAffected: 1,
		},
	}
	allowDecomposedProjectionMutation(t, fixture.base.database, updates)
	result, err := fixture.base.database.Exec(
		`UPDATE typed_memory_relational_assertions_v3
		SET provenance_ref = 'memory:test:observation-drift'
		WHERE project_id = ? AND assertion_id = ?`,
		fixture.base.project.String(),
		fixture.oldAssertion.String(),
	)
	if err != nil {
		t.Fatalf("mutate durable relation projection: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "current observation relation provenance")

	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.base.database)
	if err != nil {
		t.Fatalf("BeginRead: %v", err)
	}
	_, err = LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		fixture.base.project,
	)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		_ = transaction.Rollback(ctx)
		t.Fatalf("projection drift error = %v; want ErrStoredAdmissionIntegrity", err)
	}
	if result := transaction.Rollback(ctx); !result.Succeeded() {
		t.Fatalf("Rollback: %v", result.Err())
	}
}

func TestCurrentRelationOriginsRejectV3RowsOwnedByLegacyWriter(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	const immutabilityTrigger = "typed_memory_event_writer_generations_v46_no_update"
	var exactTriggerSQL string
	err := fixture.base.database.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type = 'trigger' AND name = ?",
		immutabilityTrigger,
	).Scan(&exactTriggerSQL)
	if err != nil || exactTriggerSQL == "" {
		t.Fatalf("load writer-generation immutability trigger: %v", err)
	}
	if _, err := fixture.base.database.Exec(
		"DROP TRIGGER " + immutabilityTrigger,
	); err != nil {
		t.Fatalf("drop writer-generation immutability trigger: %v", err)
	}
	result, err := fixture.base.database.Exec(
		`UPDATE typed_memory_event_writer_generations
		SET writer_generation = 46, provenance_kind = 'writer_v46'
		WHERE project_id = ? AND event_ref = (
			SELECT event_ref
			FROM typed_memory_relational_assertions_v3
			WHERE project_id = ? AND assertion_id = ?
		)`,
		fixture.base.project.String(),
		fixture.base.project.String(),
		fixture.oldAssertion.String(),
	)
	if err != nil {
		t.Fatalf("inject v3/legacy writer mismatch: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "v3 assertion writer marker")
	if _, err := fixture.base.database.Exec(exactTriggerSQL); err != nil {
		t.Fatalf("restore writer-generation immutability trigger: %v", err)
	}
	var restoredTriggerSQL string
	err = fixture.base.database.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type = 'trigger' AND name = ?",
		immutabilityTrigger,
	).Scan(&restoredTriggerSQL)
	if err != nil || restoredTriggerSQL != exactTriggerSQL {
		t.Fatalf(
			"writer-generation immutability trigger was not restored byte-identically: %v",
			err,
		)
	}

	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.base.database)
	if err != nil {
		t.Fatalf("BeginRead: %v", err)
	}
	head, err := loadHeadWithScanner(ctx, transaction, fixture.base.project)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("loadHeadWithScanner: %v", err)
	}
	_, err = loadCurrentRelationOrigins(ctx, transaction, head)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		_ = transaction.Rollback(ctx)
		t.Fatalf(
			"v3/legacy writer mismatch error = %v; want ErrStoredAdmissionIntegrity",
			err,
		)
	}
	if result := transaction.Rollback(ctx); !result.Succeeded() {
		t.Fatalf("Rollback: %v", result.Err())
	}
}

func TestCurrentAssertionCarrierRejectsOriginTypeEnvDifferentFromSignature(
	t *testing.T,
) {
	fixture := newGenericMixedStoreFixture(t)
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.base.database)
	if err != nil {
		t.Fatalf("BeginRead: %v", err)
	}
	head, err := loadHeadWithScanner(ctx, transaction, fixture.base.project)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("loadHeadWithScanner: %v", err)
	}
	origins, err := loadCurrentRelationOrigins(ctx, transaction, head)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("loadCurrentRelationOrigins: %v", err)
	}
	if len(origins) != 1 {
		_ = transaction.Rollback(ctx)
		t.Fatalf("current assertion origins = %d; want 1", len(origins))
	}
	origin := origins[0]
	canonical, err := hex.DecodeString(origin.CanonicalCarrier)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("decode current assertion carrier: %v", err)
	}
	assertion, err := typedmemory.DecodeCanonicalRelationalAssertion(canonical)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("DecodeCanonicalRelationalAssertion: %v", err)
	}
	digest, err := assertion.Digest()
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("RelationalAssertion.Digest: %v", err)
	}
	wrongTypeEnv, err := typedmemory.ParseTypeEnvRef(
		"typeenv:sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("ParseTypeEnvRef(wrong origin): %v", err)
	}
	if wrongTypeEnv == assertion.Signature().TypeEnv() {
		_ = transaction.Rollback(ctx)
		t.Fatal("wrong origin TypeEnv unexpectedly equals the carrier TypeEnv")
	}
	origin.EventBasisTypeEnv = wrongTypeEnv.String()
	origin.AdmissionTypeEnv = wrongTypeEnv.String()
	if err := verifyCurrentAssertionOriginCoordinates(origin); err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("internally consistent corrupt origin coordinates: %v", err)
	}
	_, err = verifyCurrentAssertionCarrierProjection(
		ctx,
		transaction,
		head,
		origin,
		assertion,
		digest,
	)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		_ = transaction.Rollback(ctx)
		t.Fatalf(
			"carrier/origin TypeEnv mismatch error = %v; want ErrStoredAdmissionIntegrity",
			err,
		)
	}
	if result := transaction.Rollback(ctx); !result.Succeeded() {
		t.Fatalf("Rollback: %v", result.Err())
	}
}

func TestCurrentRelationContextSliceClassifiesOnlyMissingRowsAsIntegrity(
	t *testing.T,
) {
	fixture := newGenericMixedStoreFixture(t)
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.base.database)
	if err != nil {
		t.Fatalf("BeginRead: %v", err)
	}
	var eventRef string
	err = transaction.ScanOne(
		ctx,
		`SELECT event_ref FROM typed_memory_relational_assertions_v3
		WHERE project_id = ? AND assertion_id = ?`,
		[]any{fixture.base.project.String(), fixture.oldAssertion.String()},
		[]any{&eventRef},
	)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("load relation event: %v", err)
	}
	err = verifyCurrentRelationContextSlice(
		ctx,
		transaction,
		fixture.base.project,
		"typed-memory-event:missing",
		fixture.contextSlice,
	)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		_ = transaction.Rollback(ctx)
		t.Fatalf("missing ContextSlice error = %v; want integrity failure", err)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	err = verifyCurrentRelationContextSlice(
		cancelled,
		transaction,
		fixture.base.project,
		eventRef,
		fixture.contextSlice,
	)
	if !errors.Is(err, context.Canceled) {
		_ = transaction.Rollback(ctx)
		t.Fatalf("cancelled ContextSlice read error = %v; want context.Canceled", err)
	}
	if errors.Is(err, ErrStoredAdmissionIntegrity) {
		_ = transaction.Rollback(ctx)
		t.Fatalf("operational ContextSlice read was classified as corruption: %v", err)
	}
	if result := transaction.Rollback(ctx); !result.Succeeded() {
		t.Fatalf("Rollback: %v", result.Err())
	}
}

func TestCurrentReferenceFillerRequiresExactAuthenticatedMemberUse(
	t *testing.T,
) {
	fixture := newReferenceProjectionTestFixture(
		t,
		"graph-observation-required-kind",
	)
	if _, err := fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		fixture.request,
	); err != nil {
		t.Fatalf("CommitMemoryChangeSet: %v", err)
	}
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead: %v", err)
	}
	head, err := loadHeadWithScanner(ctx, transaction, fixture.project)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("loadHeadWithScanner: %v", err)
	}
	origins, err := loadCurrentRelationOrigins(ctx, transaction, head)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("loadCurrentRelationOrigins: %v", err)
	}
	if len(origins) != 1 {
		_ = transaction.Rollback(ctx)
		t.Fatalf("relation origins = %d; want 1", len(origins))
	}
	raw, err := hex.DecodeString(origins[0].CanonicalCarrier)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("decode relation carrier: %v", err)
	}
	relation, err := typedmemory.DecodeCanonicalRelationalAssertion(raw)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("DecodeCanonicalRelationalAssertion: %v", err)
	}
	rows, err := loadCurrentRelationFillers(
		ctx,
		transaction,
		fixture.project,
		origins[0],
	)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("loadCurrentRelationFillers: %v", err)
	}
	if err := verifyCurrentRelationFillerRows(relation, rows); err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("exact reference filler verification: %v", err)
	}
	if len(rows) != 1 || rows[0].RequiredMemberUseCount != 1 ||
		rows[0].RequiredValueKind != rows[0].AdmittedRequiredValueKind {
		_ = transaction.Rollback(ctx)
		t.Fatalf("required-member derivation = %#v; want one exact matching use", rows)
	}

	mismatch := append([]storedCurrentRelationFiller(nil), rows...)
	mismatch[0].AdmittedRequiredValueKind =
		"typeenv:sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/kind/U.Wrong"
	if err := verifyCurrentRelationFillerRows(relation, mismatch); !errors.Is(
		err,
		ErrStoredAdmissionIntegrity,
	) {
		_ = transaction.Rollback(ctx)
		t.Fatalf("required-kind mismatch error = %v; want integrity failure", err)
	}

	missing := append([]storedCurrentRelationFiller(nil), rows...)
	missing[0].RequiredMemberUseCount = 0
	missing[0].AdmittedRequiredValueKind = ""
	if err := verifyCurrentRelationFillerRows(relation, missing); !errors.Is(
		err,
		ErrStoredAdmissionIntegrity,
	) {
		_ = transaction.Rollback(ctx)
		t.Fatalf("missing required-member use error = %v; want integrity failure", err)
	}

	duplicate := append([]storedCurrentRelationFiller(nil), rows...)
	duplicate[0].RequiredMemberUseCount = 2
	if err := verifyCurrentRelationFillerRows(relation, duplicate); !errors.Is(
		err,
		ErrStoredAdmissionIntegrity,
	) {
		_ = transaction.Rollback(ctx)
		t.Fatalf("duplicate required-member use error = %v; want integrity failure", err)
	}
	if result := transaction.Rollback(ctx); !result.Succeeded() {
		t.Fatalf("Rollback: %v", result.Err())
	}
}

func TestCurrentGraphObservationRejectsLegacyPositiveHeadWithoutExactClosure(
	t *testing.T,
) {
	fixture := newSQLiteStoreFixture(t)
	candidate := fixture.declaration(t, "legacy-head", "Legacy head")
	request := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"declare:legacy-positive-head",
		candidate,
	)
	receipt, err := fixture.adapter.commitDeclareEntity(context.Background(), request)
	if err != nil {
		t.Fatalf("commit exact declaration: %v", err)
	}
	dropStatements := []string{
		"DROP TRIGGER typed_memory_commit_materialization_closures_v46_no_delete",
		"DROP TRIGGER typed_memory_event_admission_bases_v46_no_delete",
		"DROP TRIGGER typed_memory_event_writer_generations_v46_no_update",
	}
	for _, statement := range dropStatements {
		if _, err := fixture.database.Exec(statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
	if _, err := fixture.database.Exec(
		`DELETE FROM typed_memory_commit_materialization_closures
		WHERE project_id = ? AND event_ref = ?`,
		fixture.project.String(),
		receipt.EventRef(),
	); err != nil {
		t.Fatalf("remove exact materialization closure: %v", err)
	}
	if _, err := fixture.database.Exec(
		`DELETE FROM typed_memory_event_admission_bases
		WHERE project_id = ? AND event_ref = ?`,
		fixture.project.String(),
		receipt.EventRef(),
	); err != nil {
		t.Fatalf("remove exact admission carrier: %v", err)
	}
	if _, err := fixture.database.Exec(
		`UPDATE typed_memory_event_writer_generations
		SET writer_generation = 45, provenance_kind = 'migration_v45_backfill'
		WHERE project_id = ? AND event_ref = ?`,
		fixture.project.String(),
		receipt.EventRef(),
	); err != nil {
		t.Fatalf("mark event as legacy-v45: %v", err)
	}
	if _, err := fixture.adapter.LoadCurrentProjectSnapshot(
		context.Background(),
		fixture.project,
	); err != nil {
		t.Fatalf("legacy-compatible snapshot read: %v", err)
	}

	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead: %v", err)
	}
	_, err = LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		fixture.project,
	)
	if !errors.Is(err, ErrStorageGenerationUnavailable) {
		_ = transaction.Rollback(ctx)
		t.Fatalf(
			"legacy positive-head graph observation error = %v; want ErrStorageGenerationUnavailable",
			err,
		)
	}
	if errors.Is(err, ErrStoredAdmissionIntegrity) {
		_ = transaction.Rollback(ctx)
		t.Fatalf("supported legacy head was misclassified as corruption: %v", err)
	}
	if result := transaction.Rollback(ctx); !result.Succeeded() {
		t.Fatalf("Rollback: %v", result.Err())
	}
}
