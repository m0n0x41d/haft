package identityreconciliation_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/identityreconciliation"
	"github.com/m0n0x41d/haft/internal/projectmemory/memoryresolve"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

func TestReviewedIdentityMergeCommitsReplaysResolvesAndLoadsCurrentSnapshot(
	t *testing.T,
) {
	fixture := newIdentityFixture(t)
	survivor := fixture.commitEntity(t, 0, "auth-service", "Auth service")
	legacyA := fixture.commitEntity(t, 1, "legacy-auth-a", "Legacy auth A")
	legacyB := fixture.commitEntity(t, 2, "legacy-auth-b", "Legacy auth B")
	service := fixture.service(t)
	request := fixture.reconciliationRequest(
		t,
		"reviewed-auth-merge",
		typedmemory.ReconciliationMergeEntities,
		survivor,
		[]typedmemory.EntityID{legacyB, legacyA},
		3,
		fixture.environment.Ref(),
	)

	receipt, err := service.Commit(context.Background(), request)
	if err != nil {
		t.Fatalf("Commit(merge): %v", err)
	}
	if receipt.Disposition() != identityreconciliation.CommitApplied ||
		receipt.GraphRevision().Value() != 4 {
		t.Fatalf("merge receipt = %#v; want applied revision 4", receipt)
	}
	fixture.assertIdentityLedgerRowsAreImmutable(t, receipt.EventRef())
	replay, err := service.Commit(context.Background(), request)
	if err != nil {
		t.Fatalf("Commit(exact replay): %v", err)
	}
	if replay.Disposition() != identityreconciliation.CommitReplay ||
		replay.EventRef() != receipt.EventRef() ||
		replay.CommitRef() != receipt.CommitRef() ||
		replay.ResultDigest() != receipt.ResultDigest() {
		t.Fatalf("merge replay = %#v; want exact receipt %#v", replay, receipt)
	}

	for _, historical := range []typedmemory.EntityID{legacyA, legacyB} {
		resolution, err := service.ResolveHistorical(
			context.Background(),
			fixture.project,
			historical,
			fixture.context,
		)
		if err != nil {
			t.Fatalf("ResolveHistorical(%s): %v", historical.String(), err)
		}
		merged, ok := resolution.(identityreconciliation.MergedIdentity)
		if !ok {
			t.Fatalf("ResolveHistorical(%s) = %T; want MergedIdentity", historical.String(), resolution)
		}
		if merged.Entity() != historical ||
			merged.Current() != survivor ||
			len(merged.ReconciliationHistory()) != 1 {
			t.Fatalf("merged resolution lost historical/current identity: %#v", merged)
		}
	}
	fixture.assertHistoricalRows(t, []typedmemory.EntityID{survivor, legacyA, legacyB})

	reader, err := typedmemorystore.NewSQLiteCurrentProjectSnapshotLoader(
		fixture.database,
		fixture.loader,
	)
	if err != nil {
		t.Fatalf("NewSQLiteCurrentProjectSnapshotLoader: %v", err)
	}
	loaded, err := reader.LoadCurrentProjectSnapshot(context.Background(), fixture.project)
	if err != nil {
		t.Fatalf("LoadCurrentProjectSnapshot(v52 head): %v", err)
	}
	if loaded.Snapshot().GraphRevision().Value() != 4 {
		t.Fatalf("current snapshot revision = %d; want 4", loaded.Snapshot().GraphRevision().Value())
	}
	fixture.assertRows(t, map[string]int64{
		"typed_memory_identity_reconciliations":             1,
		"typed_memory_identity_reconciliation_participants": 2,
		"typed_memory_identity_redirects":                   2,
		"typed_memory_identity_reconciliation_closures":     1,
		"typed_memory_graph_events":                         4,
		"typed_memory_graph_commits":                        4,
	})

	fixture.assertCASFailuresDoNotWrite(
		t,
		service,
		survivor,
		[]typedmemory.EntityID{legacyA, legacyB},
	)
}

func TestReviewedIdentitySplitReturnsCandidatesAndProjectionDebtDoesNotMutateGraph(
	t *testing.T,
) {
	fixture := newIdentityFixture(t)
	source := fixture.commitEntity(t, 0, "auth-boundary", "Auth boundary")
	token := fixture.commitEntity(t, 1, "token-service", "Token service")
	policy := fixture.commitEntity(t, 2, "policy-service", "Policy service")
	service := fixture.service(t)
	request := fixture.reconciliationRequest(
		t,
		"reviewed-auth-split",
		typedmemory.ReconciliationSplitEntity,
		source,
		[]typedmemory.EntityID{token, policy},
		3,
		fixture.environment.Ref(),
	)
	receipt, err := service.Commit(context.Background(), request)
	if err != nil {
		t.Fatalf("Commit(split): %v", err)
	}
	resolution, err := service.ResolveHistorical(
		context.Background(),
		fixture.project,
		source,
		fixture.context,
	)
	if err != nil {
		t.Fatalf("ResolveHistorical(split): %v", err)
	}
	candidates, ok := resolution.(identityreconciliation.SplitIdentityCandidates)
	if !ok {
		t.Fatalf("split resolution = %T; want SplitIdentityCandidates", resolution)
	}
	got := candidates.Candidates()
	if len(got) != 2 || got[0] != policy || got[1] != token {
		t.Fatalf("split candidates = %#v; want exact sorted targets", got)
	}

	headBefore := fixture.head(t)
	reason, err := identityreconciliation.NewProjectionDebtReason("carrier_projection_failed")
	if err != nil {
		t.Fatalf("NewProjectionDebtReason: %v", err)
	}
	detail, err := identityreconciliation.NewProjectionDebtDetail("injected carrier renderer failure")
	if err != nil {
		t.Fatalf("NewProjectionDebtDetail: %v", err)
	}
	debt, err := service.OpenProjectionDebt(
		context.Background(),
		fixture.project,
		receipt,
		reason,
		detail,
		identityDigest(t, "expected-split-projection"),
	)
	if err != nil {
		t.Fatalf("OpenProjectionDebt: %v", err)
	}
	if debt.DebtRef() == "" || debt.DebtEventRef() == "" {
		t.Fatal("projection debt omitted durable identity")
	}
	replayDebt, err := service.OpenProjectionDebt(
		context.Background(),
		fixture.project,
		receipt,
		reason,
		detail,
		identityDigest(t, "expected-split-projection"),
	)
	if err != nil {
		t.Fatalf("OpenProjectionDebt(exact replay): %v", err)
	}
	if replayDebt.DebtRef() != debt.DebtRef() ||
		replayDebt.DebtEventRef() != debt.DebtEventRef() {
		t.Fatalf("projection-debt replay = %#v; want exact receipt %#v", replayDebt, debt)
	}
	_, err = service.OpenProjectionDebt(
		context.Background(),
		fixture.project,
		identityreconciliation.Receipt{},
		reason,
		detail,
		identityDigest(t, "invalid-projection-basis"),
	)
	if !errors.Is(err, identityreconciliation.ErrProjectionBasis) {
		t.Fatalf("OpenProjectionDebt(invalid basis) = %v; want ErrProjectionBasis", err)
	}
	headAfter := fixture.head(t)
	if headAfter != headBefore {
		t.Fatalf("projection debt mutated graph head: before=%#v after=%#v", headBefore, headAfter)
	}
	fixture.assertRows(t, map[string]int64{
		"typed_memory_identity_reconciliations":         1,
		"typed_memory_projection_debt_events":           1,
		"typed_memory_graph_events":                     4,
		"typed_memory_graph_commits":                    4,
		"typed_memory_identity_reconciliation_closures": 1,
	})
}

func TestCurrentPublicResolutionConsumesOnlyExactCommittedMergeState(
	t *testing.T,
) {
	fixture := newIdentityFixture(t)
	survivor := fixture.commitEntity(t, 0, "read-auth", "Read auth")
	historical := fixture.commitEntity(t, 1, "read-legacy", "Read legacy")
	other := fixture.commitEntity(t, 2, "read-other", "Read other")
	service := fixture.service(t)
	request := fixture.reconciliationRequest(
		t,
		"reviewed-read-merge",
		typedmemory.ReconciliationMergeEntities,
		survivor,
		[]typedmemory.EntityID{historical, other},
		3,
		fixture.environment.Ref(),
	)
	receipt, err := service.Commit(context.Background(), request)
	if err != nil {
		t.Fatalf("Commit(read merge): %v", err)
	}
	headBefore := fixture.head(t)
	runtime := fixture.strictReadRuntime(t)
	result := fixture.resolveEntity(t, runtime, historical)
	exact, ok := result.(memoryresolve.ExactEntity)
	if !ok {
		t.Fatalf("Resolve(historical merge) = %T; want ExactEntity", result)
	}
	if exact.Entity().Entity().ReferenceID().String() != survivor.String() {
		t.Fatalf(
			"historical merge resolved to %s; want %s",
			exact.Entity().Entity().ReferenceID().String(),
			survivor.String(),
		)
	}
	witnesses := exact.ResolutionWitnesses()
	if len(witnesses) != 1 ||
		witnesses[0].Kind() != memoryresolve.WitnessReviewedMerge ||
		witnesses[0].Matched() != historical.String() ||
		witnesses[0].Basis().String() != receipt.ReconciliationRef() {
		t.Fatalf("reviewed merge witnesses = %#v; want exact durable history", witnesses)
	}
	basis, err := identityreconciliation.NewCommittedResolutionBasis(
		fixture.project,
		typedmemory.NewGraphRevision(3),
		fixture.environment.Ref(),
	)
	if err != nil {
		t.Fatalf("NewCommittedResolutionBasis(stale): %v", err)
	}
	_, err = service.LoadCommittedResolutionState(context.Background(), basis)
	if !errors.Is(err, identityreconciliation.ErrStaleGraphRevision) {
		t.Fatalf(
			"LoadCommittedResolutionState(stale) = %v; want ErrStaleGraphRevision",
			err,
		)
	}
	if headAfter := fixture.head(t); headAfter != headBefore {
		t.Fatalf(
			"strict identity resolution mutated graph head: before=%#v after=%#v",
			headBefore,
			headAfter,
		)
	}
}

func TestCurrentPublicResolutionPreservesReviewedSplitAsUnsettledCandidates(
	t *testing.T,
) {
	fixture := newIdentityFixture(t)
	source := fixture.commitEntity(t, 0, "read-boundary", "Read boundary")
	token := fixture.commitEntity(t, 1, "read-token", "Read token")
	policy := fixture.commitEntity(t, 2, "read-policy", "Read policy")
	service := fixture.service(t)
	request := fixture.reconciliationRequest(
		t,
		"reviewed-read-split",
		typedmemory.ReconciliationSplitEntity,
		source,
		[]typedmemory.EntityID{token, policy},
		3,
		fixture.environment.Ref(),
	)
	receipt, err := service.Commit(context.Background(), request)
	if err != nil {
		t.Fatalf("Commit(read split): %v", err)
	}
	runtime := fixture.strictReadRuntime(t)
	result := fixture.resolveEntity(t, runtime, source)
	unsettled, ok := result.(memoryresolve.ResolutionUnsettled)
	if !ok {
		t.Fatalf("Resolve(historical split) = %T; want ResolutionUnsettled", result)
	}
	issues := unsettled.Issues()
	if len(issues) != 1 {
		t.Fatalf("split issues = %#v; want one reviewed split issue", issues)
	}
	issue, ok := issues[0].(memoryresolve.ReviewedSplitCandidatesIssue)
	if !ok {
		t.Fatalf("split issue = %T; want ReviewedSplitCandidatesIssue", issues[0])
	}
	refs := issue.CandidateEntityRefs()
	if len(refs) != 2 ||
		refs[0].ReferenceID().String() != policy.String() ||
		refs[1].ReferenceID().String() != token.String() ||
		issue.HistoricalEntity() != source ||
		issue.Reconciliation().String() != receipt.ReconciliationRef() {
		t.Fatalf("reviewed split issue lost exact candidates/history: %#v", issue)
	}
	loader, err := typedmemorystore.NewSQLiteCurrentProjectReadFrameLoader(
		fixture.database,
		fixture.loader,
	)
	if err != nil {
		t.Fatalf("NewSQLiteCurrentProjectReadFrameLoader: %v", err)
	}
	_, err = projectmemory.NewCurrentMemoryReadRuntimeWithIdentityReconciliation(
		fixture.project,
		loader,
		nil,
	)
	if !errors.Is(err, projectmemory.ErrCommittedIdentityResolutionSourceMissing) {
		t.Fatalf("strict runtime without source = %v; want missing-source error", err)
	}
}

func TestCurrentPublicResolutionPreservesMergeToSplitHistoryChain(
	t *testing.T,
) {
	fixture := newIdentityFixture(t)
	survivor := fixture.commitEntity(t, 0, "chain-survivor", "Chain survivor")
	historical := fixture.commitEntity(t, 1, "chain-historical", "Chain historical")
	alsoMerged := fixture.commitEntity(t, 2, "chain-merged", "Chain merged")
	left := fixture.commitEntity(t, 3, "chain-left", "Chain left")
	right := fixture.commitEntity(t, 4, "chain-right", "Chain right")
	service := fixture.service(t)
	merge := fixture.reconciliationRequest(
		t,
		"reviewed-chain-merge",
		typedmemory.ReconciliationMergeEntities,
		survivor,
		[]typedmemory.EntityID{historical, alsoMerged},
		5,
		fixture.environment.Ref(),
	)
	mergeReceipt, err := service.Commit(context.Background(), merge)
	if err != nil {
		t.Fatalf("Commit(chain merge): %v", err)
	}
	split := fixture.reconciliationRequest(
		t,
		"reviewed-chain-split",
		typedmemory.ReconciliationSplitEntity,
		survivor,
		[]typedmemory.EntityID{left, right},
		6,
		fixture.environment.Ref(),
	)
	splitReceipt, err := service.Commit(context.Background(), split)
	if err != nil {
		t.Fatalf("Commit(chain split): %v", err)
	}
	result := fixture.resolveEntity(t, fixture.strictReadRuntime(t), historical)
	unsettled, ok := result.(memoryresolve.ResolutionUnsettled)
	if !ok || len(unsettled.Issues()) != 1 {
		t.Fatalf("Resolve(merge-to-split history) = %#v; want one unsettled issue", result)
	}
	issue, ok := unsettled.Issues()[0].(memoryresolve.ReviewedSplitCandidatesIssue)
	if !ok {
		t.Fatalf("merge-to-split issue = %T; want ReviewedSplitCandidatesIssue", unsettled.Issues()[0])
	}
	history := issue.ReconciliationHistory()
	if len(history) != 2 ||
		history[0].String() != mergeReceipt.ReconciliationRef() ||
		history[1].String() != splitReceipt.ReconciliationRef() {
		t.Fatalf("merge-to-split history = %#v; want both durable effects in order", history)
	}
}

func TestReviewedIdentityConcurrentCASHasOneWinnerAndNoPartialLoser(
	t *testing.T,
) {
	fixture := newIdentityFixture(t)
	survivor := fixture.commitEntity(t, 0, "cas-auth", "CAS auth")
	legacyA := fixture.commitEntity(t, 1, "cas-legacy-a", "CAS legacy A")
	legacyB := fixture.commitEntity(t, 2, "cas-legacy-b", "CAS legacy B")
	service := fixture.service(t)
	requests := []identityreconciliation.Request{
		fixture.reconciliationRequest(
			t,
			"reviewed-cas-merge-a",
			typedmemory.ReconciliationMergeEntities,
			survivor,
			[]typedmemory.EntityID{legacyA, legacyB},
			3,
			fixture.environment.Ref(),
		),
		fixture.reconciliationRequest(
			t,
			"reviewed-cas-merge-b",
			typedmemory.ReconciliationMergeEntities,
			survivor,
			[]typedmemory.EntityID{legacyA, legacyB},
			3,
			fixture.environment.Ref(),
		),
	}
	type result struct {
		receipt identityreconciliation.Receipt
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, len(requests))
	for _, request := range requests {
		request := request
		go func() {
			<-start
			receipt, err := service.Commit(context.Background(), request)
			results <- result{receipt: receipt, err: err}
		}()
	}
	close(start)
	applied := 0
	stale := 0
	for range requests {
		result := <-results
		switch {
		case result.err == nil && result.receipt.Disposition() == identityreconciliation.CommitApplied:
			applied++
		case errors.Is(result.err, identityreconciliation.ErrStaleGraphRevision):
			stale++
		default:
			t.Fatalf("concurrent reconciliation result = receipt %#v, error %v", result.receipt, result.err)
		}
	}
	if applied != 1 || stale != 1 {
		t.Fatalf("concurrent CAS results: applied=%d stale=%d; want 1 and 1", applied, stale)
	}
	if got := fixture.head(t).revision; got != 4 {
		t.Fatalf("concurrent CAS graph revision = %d; want 4", got)
	}
	fixture.assertRows(t, map[string]int64{
		"typed_memory_identity_reconciliations":             1,
		"typed_memory_identity_reconciliation_participants": 2,
		"typed_memory_identity_redirects":                   2,
		"typed_memory_identity_reconciliation_closures":     1,
		"typed_memory_graph_events":                         4,
		"typed_memory_graph_commits":                        4,
	})
}

func TestReviewedIdentityReplayRejectsCorruptedDurableCarrier(
	t *testing.T,
) {
	fixture := newIdentityFixture(t)
	survivor := fixture.commitEntity(t, 0, "replay-auth", "Replay auth")
	legacyA := fixture.commitEntity(t, 1, "replay-legacy-a", "Replay legacy A")
	legacyB := fixture.commitEntity(t, 2, "replay-legacy-b", "Replay legacy B")
	service := fixture.service(t)
	request := fixture.reconciliationRequest(
		t,
		"reviewed-replay-corruption",
		typedmemory.ReconciliationMergeEntities,
		survivor,
		[]typedmemory.EntityID{legacyA, legacyB},
		3,
		fixture.environment.Ref(),
	)
	receipt, err := service.Commit(context.Background(), request)
	if err != nil {
		t.Fatalf("Commit(merge): %v", err)
	}
	const triggerName = "typed_memory_identity_reconciliations_v52_no_update"
	var triggerSQL string
	err = fixture.database.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type = 'trigger' AND name = ?`,
		triggerName,
	).Scan(&triggerSQL)
	if err != nil {
		t.Fatalf("load immutable trigger SQL: %v", err)
	}
	if _, err := fixture.database.Exec("DROP TRIGGER " + triggerName); err != nil {
		t.Fatalf("drop immutable trigger for corruption injection: %v", err)
	}
	_, err = fixture.database.Exec(
		`UPDATE typed_memory_identity_reconciliations
		 SET canonical_reconciliation_bytes = X'00'
		 WHERE project_id = ? AND event_ref = ?`,
		fixture.project.String(),
		receipt.EventRef(),
	)
	if err != nil {
		t.Fatalf("inject durable carrier corruption: %v", err)
	}
	if _, err := fixture.database.Exec(triggerSQL); err != nil {
		t.Fatalf("restore immutable trigger: %v", err)
	}
	_, found, err := service.Replay(context.Background(), request)
	if found || !errors.Is(err, identityreconciliation.ErrStoredIntegrity) {
		t.Fatalf("Replay(corrupted carrier) = found %v, error %v; want rejected ErrStoredIntegrity", found, err)
	}
	basis, basisErr := identityreconciliation.NewCommittedResolutionBasis(
		fixture.project,
		typedmemory.NewGraphRevision(4),
		fixture.environment.Ref(),
	)
	if basisErr != nil {
		t.Fatalf("NewCommittedResolutionBasis(corrupted carrier): %v", basisErr)
	}
	_, err = service.LoadCommittedResolutionState(context.Background(), basis)
	if !errors.Is(err, identityreconciliation.ErrStoredIntegrity) {
		t.Fatalf(
			"LoadCommittedResolutionState(corrupted carrier) = %v; want ErrStoredIntegrity",
			err,
		)
	}
	if got := fixture.head(t).revision; got != 4 {
		t.Fatalf("corrupted replay changed graph revision = %d; want 4", got)
	}
}

type identityFixture struct {
	database    *sql.DB
	project     projectledger.ProjectID
	environment typedmemory.TypeEnv
	registry    typedmemory.CodecRegistry
	context     typedmemory.BoundedContextRef
	clock       fixedClock
	loader      exactTypeEnvLoader
	adapter     *typedmemorystore.SQLiteAdapter
}

func newIdentityFixture(t *testing.T) identityFixture {
	t.Helper()
	root := t.TempDir()
	store, err := db.NewStore(filepath.Join(root, "identity-reconciliation.db"))
	if err != nil {
		t.Fatalf("db.NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	database := store.GetRawDB()
	project := mustProject(t, "qnt_1d3e71a1")
	insertProjectBinding(t, database, project, root)
	canonical := []byte(`{"schema":"test.identity-reconciliation.typeenv/v1","context":"ctx:identity"}`)
	typeEnvRef := mustTypeEnvRef(t, canonical)
	sourceRevision := mustSourceRevision(t, "identity-reconciliation-fpf-revision")
	compilerVersion := mustCompilerVersion(t, "identity.reconciliation.compiler.v1")
	contextRef := mustContext(t, "ctx:identity")
	provenance := mustFPFProvenance(t, sourceRevision)
	contextValue, err := typedmemory.NewBoundedContext(contextRef, provenance)
	if err != nil {
		t.Fatalf("NewBoundedContext: %v", err)
	}
	entityKindID, err := typedmemory.NewKindID("U.Entity")
	if err != nil {
		t.Fatalf("NewKindID(U.Entity): %v", err)
	}
	entityKind, err := typedmemory.NewKindDefinition(entityKindID, provenance)
	if err != nil {
		t.Fatalf("NewKindDefinition(U.Entity): %v", err)
	}
	entityValueKind, err := typedmemory.NewValueKindRef(typeEnvRef, entityKindID)
	if err != nil {
		t.Fatalf("NewValueKindRef(U.Entity): %v", err)
	}
	entityRefID, err := typedmemory.NewRefKindID("U.EntityRef")
	if err != nil {
		t.Fatalf("NewRefKindID(U.EntityRef): %v", err)
	}
	entityRef, err := typedmemory.NewRefKindRef(typeEnvRef, entityRefID)
	if err != nil {
		t.Fatalf("NewRefKindRef(U.EntityRef): %v", err)
	}
	entityRefDefinition, err := typedmemory.NewRefKindDefinition(
		entityRef,
		entityValueKind,
		provenance,
	)
	if err != nil {
		t.Fatalf("NewRefKindDefinition(U.EntityRef): %v", err)
	}
	coverageSubject, err := typedmemory.SourceUnitCoverage(provenance.Location().UnitID())
	if err != nil {
		t.Fatalf("SourceUnitCoverage: %v", err)
	}
	coverageEntry, err := typedmemory.NewCompiledCoverageEntry(
		coverageSubject,
		provenance.Location(),
	)
	if err != nil {
		t.Fatalf("NewCompiledCoverageEntry: %v", err)
	}
	coverage, err := typedmemory.NewCoverageManifest([]typedmemory.CoverageEntry{coverageEntry})
	if err != nil {
		t.Fatalf("NewCoverageManifest: %v", err)
	}
	environment, err := typedmemory.NewTypeEnvBuilder(typeEnvRef).
		SetSourceRevision(sourceRevision).
		SetCompilerSchemaVersion(compilerVersion).
		SetCoverageManifest(coverage).
		AddBoundedContext(contextValue).
		AddKindDefinition(entityKind).
		AddRefKindDefinition(entityRefDefinition).
		Build()
	if err != nil {
		t.Fatalf("build identity TypeEnv: %v", err)
	}
	format, err := typedmemorystore.NewSnapshotFormat(typedmemorystore.BaseTypeEnvSnapshotFormat)
	if err != nil {
		t.Fatalf("NewSnapshotFormat: %v", err)
	}
	snapshot, err := typedmemorystore.NewTypeEnvSnapshotBuilder(typeEnvRef).
		SetFormat(format).
		SetCanonicalBytes(canonical).
		SetSourceRevision(sourceRevision).
		SetCompilerSchemaVersion(compilerVersion).
		Build()
	if err != nil {
		t.Fatalf("build TypeEnv snapshot: %v", err)
	}
	registry := typedmemory.NewCodecRegistry()
	clock := fixedClock{value: time.Date(2026, 7, 19, 10, 30, 0, 123456789, time.UTC)}
	loader := exactTypeEnvLoader{
		reference:   typeEnvRef,
		environment: environment,
		registry:    registry,
	}
	initializer, err := typedmemorystore.NewSQLiteProjectGraphInitializer(
		database,
		loader,
		clock,
	)
	if err != nil {
		t.Fatalf("NewSQLiteProjectGraphInitializer: %v", err)
	}
	if _, err := initializer.InitializeProjectGraphAtBaseTypeEnv(
		context.Background(),
		project,
		snapshot,
	); err != nil {
		t.Fatalf("InitializeProjectGraphAtBaseTypeEnv: %v", err)
	}
	adapter, err := typedmemorystore.NewGenericSQLiteAdapter(
		database,
		loader,
		clock,
		unexpectedMemberOf{},
		unexpectedReference{},
		unexpectedObservable{},
	)
	if err != nil {
		t.Fatalf("NewGenericSQLiteAdapter: %v", err)
	}
	return identityFixture{
		database:    database,
		project:     project,
		environment: environment,
		registry:    registry,
		context:     contextRef,
		clock:       clock,
		loader:      loader,
		adapter:     adapter,
	}
}

func (fixture identityFixture) service(t *testing.T) *identityreconciliation.SQLiteService {
	t.Helper()
	service, err := identityreconciliation.NewSQLiteService(fixture.database, fixture.clock)
	if err != nil {
		t.Fatalf("NewSQLiteService: %v", err)
	}
	return service
}

func (fixture identityFixture) strictReadRuntime(
	t *testing.T,
) projectmemory.CurrentMemoryReadRuntime {
	t.Helper()
	loader, err := typedmemorystore.NewSQLiteCurrentProjectReadFrameLoader(
		fixture.database,
		fixture.loader,
	)
	if err != nil {
		t.Fatalf("NewSQLiteCurrentProjectReadFrameLoader: %v", err)
	}
	source, err := identityreconciliation.NewSQLiteCommittedResolutionStateSource(
		fixture.database,
	)
	if err != nil {
		t.Fatalf("NewSQLiteCommittedResolutionStateSource: %v", err)
	}
	runtime, err := projectmemory.NewCurrentMemoryReadRuntimeWithIdentityReconciliation(
		fixture.project,
		loader,
		source,
	)
	if err != nil {
		t.Fatalf("NewCurrentMemoryReadRuntimeWithIdentityReconciliation: %v", err)
	}
	return runtime
}

func (fixture identityFixture) resolveEntity(
	t *testing.T,
	runtime projectmemory.CurrentMemoryReadRuntime,
	entity typedmemory.EntityID,
) memoryresolve.EntityResolutionResult {
	t.Helper()
	basis, err := runtime.CurrentSnapshotBasis(context.Background())
	if err != nil {
		t.Fatalf("CurrentSnapshotBasis: %v", err)
	}
	query, err := memoryresolve.NewResolutionQuery(entity.String())
	if err != nil {
		t.Fatalf("NewResolutionQuery: %v", err)
	}
	contextRef, err := memoryresolve.NewExactContext(fixture.context)
	if err != nil {
		t.Fatalf("NewExactContext: %v", err)
	}
	request, err := memoryresolve.NewResolutionRequest(
		query,
		contextRef,
		basis,
		10,
	)
	if err != nil {
		t.Fatalf("NewResolutionRequest: %v", err)
	}
	result, err := runtime.Resolve(context.Background(), request)
	if err != nil {
		t.Fatalf("Resolve(%s): %v", entity.String(), err)
	}
	return result
}

func (fixture identityFixture) commitEntity(
	t *testing.T,
	revision uint64,
	suffix string,
	labelText string,
) typedmemory.EntityID {
	t.Helper()
	entity, err := typedmemory.NewEntityID("entity:" + suffix)
	if err != nil {
		t.Fatalf("NewEntityID: %v", err)
	}
	localRef, err := typedmemory.NewBatchLocalRef("local:" + suffix)
	if err != nil {
		t.Fatalf("NewBatchLocalRef: %v", err)
	}
	label, err := typedmemory.NewEntityLabel(labelText)
	if err != nil {
		t.Fatalf("NewEntityLabel: %v", err)
	}
	provenance, err := typedmemory.NewProvenanceRef("memory:identity:" + suffix)
	if err != nil {
		t.Fatalf("NewProvenanceRef: %v", err)
	}
	declaration, err := typedmemory.NewDeclareEntity(
		entity,
		localRef,
		fixture.context,
		label,
		provenance,
	)
	if err != nil {
		t.Fatalf("NewDeclareEntity: %v", err)
	}
	candidate, err := typedmemory.NewMemoryChangeSet([]typedmemory.MemoryChange{declaration})
	if err != nil {
		t.Fatalf("NewMemoryChangeSet: %v", err)
	}
	snapshot := declarationSnapshot{
		project:  fixture.project,
		revision: typedmemory.NewGraphRevision(revision),
		typeEnv:  fixture.environment.Ref(),
	}
	verdict := typedmemory.ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		snapshot,
		candidate,
	)
	valid, ok := verdict.(typedmemory.Valid)
	if !ok {
		t.Fatalf("ValidateMemoryChangeSet(declaration) = %T; want Valid", verdict)
	}
	key, err := typedmemorystore.NewIdempotencyKey("identity-reconciliation-seed-" + suffix)
	if err != nil {
		t.Fatalf("NewIdempotencyKey: %v", err)
	}
	requestProvenance, err := typedmemory.NewProvenanceRef("memory:identity:declaration-request")
	if err != nil {
		t.Fatalf("NewProvenanceRef(request): %v", err)
	}
	request, err := typedmemorystore.NewCommitRequestBuilder().
		SetContractVersion(typedmemorystore.AdmissionContractV2()).
		SetProject(fixture.project).
		SetExpectedRevision(typedmemory.NewGraphRevision(revision)).
		SetExpectedTypeEnv(fixture.environment.Ref()).
		SetIdempotencyKey(key).
		SetRequestProvenance(requestProvenance).
		SetCandidate(candidate).
		SetAdmissionBatch(valid.AdmissionBatch()).
		Build()
	if err != nil {
		t.Fatalf("build declaration request: %v", err)
	}
	if _, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request); err != nil {
		t.Fatalf("CommitMemoryChangeSet(%s): %v", suffix, err)
	}
	return entity
}

func (fixture identityFixture) reconciliationRequest(
	t *testing.T,
	keyText string,
	operation typedmemory.IdentityReconciliationOperation,
	primary typedmemory.EntityID,
	related []typedmemory.EntityID,
	revision uint64,
	typeEnv typedmemory.TypeEnvRef,
) identityreconciliation.Request {
	t.Helper()
	basisRef, err := typedmemory.NewReconciliationBasisRef("reconciliation:" + keyText)
	if err != nil {
		t.Fatalf("NewReconciliationBasisRef: %v", err)
	}
	provenance, err := typedmemory.NewProvenanceRef("memory:review:" + keyText)
	if err != nil {
		t.Fatalf("NewProvenanceRef: %v", err)
	}
	resolved, err := typedmemory.NewResolvedReconciliationBasis(
		basisRef,
		operation,
		fixture.context,
		primary,
		related,
		typedmemory.NewGraphRevision(revision),
		typeEnv,
		identityDigest(t, "review-payload:"+keyText),
		provenance,
	)
	if err != nil {
		t.Fatalf("NewResolvedReconciliationBasis: %v", err)
	}
	var change typedmemory.IdentityChange
	switch operation {
	case typedmemory.ReconciliationMergeEntities:
		change, err = typedmemory.NewMergeEntities(primary, related, fixture.context, basisRef)
	case typedmemory.ReconciliationSplitEntity:
		change, err = typedmemory.NewSplitEntity(primary, related, fixture.context, basisRef)
	default:
		t.Fatalf("unsupported identity test operation %q", operation)
	}
	if err != nil {
		t.Fatalf("build identity change: %v", err)
	}
	admission, err := typedmemory.NewReviewedIdentityReconciliationAdmission(change, resolved)
	if err != nil {
		t.Fatalf("NewReviewedIdentityReconciliationAdmission: %v", err)
	}
	key, err := identityreconciliation.NewIdempotencyKey(keyText)
	if err != nil {
		t.Fatalf("NewIdempotencyKey: %v", err)
	}
	request, err := identityreconciliation.NewRequestBuilder().
		SetProject(fixture.project).
		SetIdempotencyKey(key).
		SetAdmission(admission).
		Build()
	if err != nil {
		t.Fatalf("build identity reconciliation request: %v", err)
	}
	return request
}

func (fixture identityFixture) assertCASFailuresDoNotWrite(
	t *testing.T,
	service *identityreconciliation.SQLiteService,
	primary typedmemory.EntityID,
	related []typedmemory.EntityID,
) {
	t.Helper()
	stale := fixture.reconciliationRequest(
		t,
		"stale-reviewed-auth-merge",
		typedmemory.ReconciliationMergeEntities,
		primary,
		related,
		3,
		fixture.environment.Ref(),
	)
	if _, err := service.Commit(context.Background(), stale); !errors.Is(err, identityreconciliation.ErrStaleGraphRevision) {
		t.Fatalf("stale reconciliation error = %v; want ErrStaleGraphRevision", err)
	}
	wrongTypeEnv := mustTypeEnvRef(t, []byte(`{"schema":"wrong-typeenv"}`))
	changed := fixture.reconciliationRequest(
		t,
		"wrong-typeenv-reviewed-auth-merge",
		typedmemory.ReconciliationMergeEntities,
		primary,
		related,
		4,
		wrongTypeEnv,
	)
	if _, err := service.Commit(context.Background(), changed); !errors.Is(err, identityreconciliation.ErrActiveTypeEnvChanged) {
		t.Fatalf("wrong-TypeEnv reconciliation error = %v; want ErrActiveTypeEnvChanged", err)
	}
	fixture.assertRows(t, map[string]int64{
		"typed_memory_identity_reconciliations": 1,
		"typed_memory_graph_events":             4,
		"typed_memory_graph_commits":            4,
	})
}

func (fixture identityFixture) assertHistoricalRows(
	t *testing.T,
	entities []typedmemory.EntityID,
) {
	t.Helper()
	for _, entity := range entities {
		var entityCount int
		var contextCount int
		if err := fixture.database.QueryRow(
			`SELECT COUNT(*) FROM typed_memory_entities WHERE project_id = ? AND entity_id = ?`,
			fixture.project.String(),
			entity.String(),
		).Scan(&entityCount); err != nil {
			t.Fatalf("count historical entity %s: %v", entity.String(), err)
		}
		if err := fixture.database.QueryRow(
			`SELECT COUNT(*) FROM typed_memory_entity_contexts
			 WHERE project_id = ? AND entity_id = ? AND bounded_context_ref = ?`,
			fixture.project.String(),
			entity.String(),
			fixture.context.String(),
		).Scan(&contextCount); err != nil {
			t.Fatalf("count historical context %s: %v", entity.String(), err)
		}
		if entityCount != 1 || contextCount != 1 {
			t.Fatalf("historical entity %s was not preserved: entity=%d context=%d", entity.String(), entityCount, contextCount)
		}
	}
}

func (fixture identityFixture) assertRows(t *testing.T, wanted map[string]int64) {
	t.Helper()
	for table, expected := range wanted {
		var actual int64
		if err := fixture.database.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&actual); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if actual != expected {
			t.Fatalf("%s row count = %d; want %d", table, actual, expected)
		}
	}
}

func (fixture identityFixture) assertIdentityLedgerRowsAreImmutable(
	t *testing.T,
	eventRef string,
) {
	t.Helper()
	for _, table := range []string{
		"typed_memory_identity_reconciliations",
		"typed_memory_identity_reconciliation_participants",
		"typed_memory_identity_redirects",
		"typed_memory_identity_reconciliation_closures",
	} {
		_, updateErr := fixture.database.Exec(
			"UPDATE "+table+" SET project_id = project_id WHERE project_id = ? AND event_ref = ?",
			fixture.project.String(),
			eventRef,
		)
		if updateErr == nil || !strings.Contains(updateErr.Error(), "append-only") {
			t.Fatalf("append-only table %s UPDATE error = %v", table, updateErr)
		}
		_, deleteErr := fixture.database.Exec(
			"DELETE FROM "+table+" WHERE project_id = ? AND event_ref = ?",
			fixture.project.String(),
			eventRef,
		)
		if deleteErr == nil || !strings.Contains(deleteErr.Error(), "append-only") {
			t.Fatalf("append-only table %s DELETE error = %v", table, deleteErr)
		}
	}
}

type headCoordinate struct {
	revision int64
	typeEnv  string
	eventRef string
	commit   string
}

func (fixture identityFixture) head(t *testing.T) headCoordinate {
	t.Helper()
	result := headCoordinate{}
	err := fixture.database.QueryRow(
		`SELECT graph_revision, active_type_env_ref, last_event_ref, last_commit_ref
		 FROM typed_memory_graph_heads WHERE project_id = ?`,
		fixture.project.String(),
	).Scan(&result.revision, &result.typeEnv, &result.eventRef, &result.commit)
	if err != nil {
		t.Fatalf("load graph head: %v", err)
	}
	return result
}

type declarationSnapshot struct {
	project  projectledger.ProjectID
	revision typedmemory.GraphRevision
	typeEnv  typedmemory.TypeEnvRef
}

func (snapshot declarationSnapshot) GraphRevision() typedmemory.GraphRevision {
	return snapshot.revision
}

func (snapshot declarationSnapshot) TypeEnvRef() typedmemory.TypeEnvRef {
	return snapshot.typeEnv
}

func (snapshot declarationSnapshot) ResolveEntity(
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.EntityResolution {
	basisText := derivedSnapshotBasis(snapshot.project, snapshot.revision)
	basis, _ := typedmemory.NewResolutionBasisRef(basisText)
	resolution, _ := typedmemory.NewAbsentEntityResolution(entity, contextRef, basis)
	return resolution
}

func (declarationSnapshot) ResolveReference(
	typedmemory.StrongRef,
	typedmemory.BoundedContextRef,
) typedmemory.StrongReferenceResolution {
	return nil
}

func (declarationSnapshot) EvaluateMemberOf(
	typedmemory.MemberOfEvaluationRequest,
) typedmemory.MemberOfJudgement {
	return nil
}

func (declarationSnapshot) AssertionState(typedmemory.AssertionID) typedmemory.AssertionState {
	return nil
}

func (declarationSnapshot) ResolveAlias(
	typedmemory.EntityAlias,
	typedmemory.BoundedContextRef,
) typedmemory.AliasAvailability {
	return nil
}

func (declarationSnapshot) ResolveReconciliationBasis(
	typedmemory.ReconciliationBasisRef,
	typedmemory.BoundedContextRef,
) typedmemory.ReconciliationBasisResolution {
	return nil
}

type fixedClock struct{ value time.Time }

func (clock fixedClock) Now() time.Time { return clock.value }

type exactTypeEnvLoader struct {
	reference   typedmemory.TypeEnvRef
	environment typedmemory.TypeEnv
	registry    typedmemory.CodecRegistry
}

func (loader exactTypeEnvLoader) LoadTypeEnv(
	snapshot typedmemorystore.TypeEnvSnapshot,
) (typedmemory.TypeEnv, typedmemory.CodecRegistry, error) {
	if snapshot.Ref() != loader.reference {
		return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, fmt.Errorf(
			"unexpected identity test TypeEnv %s",
			snapshot.Ref().String(),
		)
	}
	return loader.environment, loader.registry, nil
}

type unexpectedMemberOf struct{}

func (unexpectedMemberOf) EvaluateMemberOf(
	context.Context,
	typedmemorystore.MemberOfEvaluationInput,
) (typedmemory.MemberOfJudgement, error) {
	return nil, fmt.Errorf("unexpected MemberOf evaluation")
}

type unexpectedReference struct{}

func (unexpectedReference) ResolveStrongReference(
	context.Context,
	typedmemorystore.StrongReferenceResolutionInput,
) (typedmemory.StrongReferenceResolution, error) {
	return nil, fmt.Errorf("unexpected strong-reference resolution")
}

type unexpectedObservable struct{}

func (unexpectedObservable) LoadObservableInput(
	context.Context,
	projectledger.ProjectID,
	typedmemory.ObservableInputRef,
	typedmemory.SHA256Digest,
) (typedmemorystore.ObservableInputBlob, error) {
	return typedmemorystore.ObservableInputBlob{}, fmt.Errorf("unexpected observable input")
}

func insertProjectBinding(
	t *testing.T,
	database *sql.DB,
	project projectledger.ProjectID,
	root string,
) {
	t.Helper()
	physicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	boundAt := "2026-07-19T10:00:00Z"
	carrier := fmt.Sprintf(
		`{"schema":"haft.project-ledger-binding/v1","project_id":%q,"project_root":%q,"bound_at":%q}`,
		project.String(),
		physicalRoot,
		boundAt,
	)
	_, err = database.Exec(
		`INSERT INTO project_ledger_binding (
			binding_slot, project_id, project_root, binding_digest, binding_json, bound_at
		) VALUES (1, ?, ?, ?, ?, ?)`,
		project.String(),
		physicalRoot,
		digestText([]byte(carrier)),
		carrier,
		boundAt,
	)
	if err != nil {
		t.Fatalf("insert project binding: %v", err)
	}
}

func mustProject(t *testing.T, raw string) projectledger.ProjectID {
	t.Helper()
	value, err := projectledger.ParseProjectID(raw)
	if err != nil {
		t.Fatalf("ParseProjectID: %v", err)
	}
	return value
}

func mustTypeEnvRef(t *testing.T, canonical []byte) typedmemory.TypeEnvRef {
	t.Helper()
	digest, err := typedmemory.NewSHA256Digest(digestText(canonical))
	if err != nil {
		t.Fatalf("NewSHA256Digest: %v", err)
	}
	value, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef: %v", err)
	}
	return value
}

func mustSourceRevision(t *testing.T, raw string) typedmemory.SourceRevision {
	t.Helper()
	value, err := typedmemory.NewSourceRevision(raw)
	if err != nil {
		t.Fatalf("NewSourceRevision: %v", err)
	}
	return value
}

func mustCompilerVersion(t *testing.T, raw string) typedmemory.CompilerSchemaVersion {
	t.Helper()
	value, err := typedmemory.NewCompilerSchemaVersion(raw)
	if err != nil {
		t.Fatalf("NewCompilerSchemaVersion: %v", err)
	}
	return value
}

func mustContext(t *testing.T, raw string) typedmemory.BoundedContextRef {
	t.Helper()
	value, err := typedmemory.NewBoundedContextRef(raw)
	if err != nil {
		t.Fatalf("NewBoundedContextRef: %v", err)
	}
	return value
}

func mustFPFProvenance(
	t *testing.T,
	revision typedmemory.SourceRevision,
) typedmemory.FPFSourceProvenance {
	t.Helper()
	unit, err := typedmemory.NewSourceUnitID("spec:identity-reconciliation-test-typeenv")
	if err != nil {
		t.Fatalf("NewSourceUnitID: %v", err)
	}
	contentDigest, err := typedmemory.NewSHA256Digest(digestText([]byte("identity TypeEnv source")))
	if err != nil {
		t.Fatalf("NewSHA256Digest: %v", err)
	}
	lineRange, err := typedmemory.NewSourceLineRange(1, 1)
	if err != nil {
		t.Fatalf("NewSourceLineRange: %v", err)
	}
	location, err := typedmemory.NewUnpatternedSourceLocation(
		unit,
		revision,
		contentDigest,
		lineRange,
	)
	if err != nil {
		t.Fatalf("NewUnpatternedSourceLocation: %v", err)
	}
	reference, err := typedmemory.NewProvenanceRef("prov:fpf:identity-reconciliation-test")
	if err != nil {
		t.Fatalf("NewProvenanceRef: %v", err)
	}
	rule, err := typedmemory.NewCompilerRuleID("identity.reconciliation.context.v1")
	if err != nil {
		t.Fatalf("NewCompilerRuleID: %v", err)
	}
	provenance, err := typedmemory.NewFPFSourceProvenance(reference, location, rule)
	if err != nil {
		t.Fatalf("NewFPFSourceProvenance: %v", err)
	}
	return provenance
}

func identityDigest(t *testing.T, value string) typedmemory.SHA256Digest {
	t.Helper()
	digest, err := typedmemory.NewSHA256Digest(digestText([]byte(value)))
	if err != nil {
		t.Fatalf("NewSHA256Digest: %v", err)
	}
	return digest
}

func digestText(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func derivedSnapshotBasis(
	project projectledger.ProjectID,
	revision typedmemory.GraphRevision,
) string {
	domain := "typed-memory-snapshot-resolution-basis.v1"
	fields := []string{project.String(), strconv.FormatUint(revision.Value(), 10)}
	buffer := make([]byte, 0)
	appendField := func(value string) {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		buffer = append(buffer, length[:]...)
		buffer = append(buffer, value...)
	}
	appendField("haft.typedmemorystore.digest.v1")
	appendField(domain)
	for _, field := range fields {
		appendField(field)
	}
	hexDigest := strings.TrimPrefix(digestText(buffer), "sha256:")
	return domain + ":" + hexDigest
}
