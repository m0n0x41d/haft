package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/legacydualread"
	"github.com/m0n0x41d/haft/internal/projectmemory/legacyimport"
	"github.com/m0n0x41d/haft/internal/projectmemory/legacyimporteffect"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

func TestLegacySemanticImportUsesSelectedRuntimeAndRemainsDormantUntilExactFixtureActivation(
	t *testing.T,
) {
	fixture := newGenesisE2EFixture(t)
	ctx := context.Background()
	selection, err := fixture.service.SelectGenesis(
		ctx,
		genesisSelectionInput(fixture),
	)
	if err != nil {
		t.Fatalf("SelectGenesis(): %v", err)
	}
	fresh, ok :=
		selection.(projecttypeenvselectioneffect.FreshlyCommitted)
	if !ok {
		t.Fatalf(
			"SelectGenesis() = %T, want FreshlyCommitted",
			selection,
		)
	}
	closure := fresh.Closure()
	runtime, contextRef := newLegacyImportAdmissionRuntime(t, fixture)
	validate := genesisE2EProjectCurrentDeclareRequest(t, contextRef)
	candidate, err := validate.BindChangeSet(closure.Target().Composite())
	if err != nil {
		t.Fatalf("BindChangeSet(): %v", err)
	}
	opaque, carrier := newLegacyImportApplyRequest(
		t,
		fixture.project,
		"semantic-e2e",
	)
	bridge := newLegacyImportBridge(
		t,
		fixture.project,
		carrier,
		contextRef,
	)
	typedKey, err := typedmemorystore.NewIdempotencyKey(
		"legacy-semantic-import-e2e",
	)
	if err != nil {
		t.Fatalf("NewIdempotencyKey(): %v", err)
	}
	provenance, err := typedmemory.NewProvenanceRef(
		"provenance:legacy-semantic-import-e2e",
	)
	if err != nil {
		t.Fatalf("NewProvenanceRef(): %v", err)
	}
	request, err := legacyimporteffect.NewSemanticImportRequest(
		legacyimporteffect.SemanticImportRequestInput{
			OpaqueRequest: opaque,
			Selector:      validate.Basis(),
			Candidate:     candidate,
			Bridges:       []legacydualread.IdentityBridge{bridge},
			TypedKey:      typedKey,
			Provenance:    provenance,
		},
	)
	if err != nil {
		t.Fatalf("NewSemanticImportRequest(): %v", err)
	}
	prepared, err := legacyimporteffect.PrepareSemanticImport(
		ctx,
		runtime,
		request,
	)
	if err != nil {
		t.Fatalf("PrepareSemanticImport(): %v", err)
	}
	store, err := legacyimporteffect.NewSQLiteStore(
		ctx,
		fixture.database,
		NewCurrentCommittedClosureLoader(),
	)
	if err != nil {
		t.Fatalf("NewSQLiteStore(): %v", err)
	}

	first, err := legacyimporteffect.NewSemanticImportService().Apply(
		ctx,
		store,
		runtime,
		prepared,
	)
	if err != nil {
		t.Fatalf("SemanticImportService.Apply(fresh): %v", err)
	}
	if first.TypedReceipt().Disposition() != typedmemorystore.CommitApplied {
		t.Fatalf(
			"typed disposition = %s, want applied",
			first.TypedReceipt().Disposition(),
		)
	}
	if first.Record().GraphRevision() != first.TypedReceipt().GraphRevision() ||
		first.Record().GraphEventRef() != first.TypedReceipt().EventRef() ||
		first.Record().GraphCommitRef() != first.TypedReceipt().CommitRef() {
		t.Fatal("semantic import marker differs from typed commit receipt")
	}
	assertLegacyImportCounts(t, fixture.database, []int{1, 1, 1, 1, 1, 1})
	assertLegacyWriteMode(t, fixture.database, "legacy_compatible")
	conflicting, _ := newLegacyImportApplyRequest(
		t,
		fixture.project,
		"semantic-e2e-conflict",
	)
	conflicting, err = legacyimporteffect.NewImportApplyRequest(
		conflicting.Plan(),
		opaque.IdempotencyKey(),
	)
	if err != nil {
		t.Fatalf("NewImportApplyRequest(conflict): %v", err)
	}
	_, err = legacyimporteffect.NewApplyService().Apply(
		ctx,
		store,
		conflicting,
	)
	if !errors.Is(err, legacyimporteffect.ErrImportReplayConflict) {
		t.Fatalf("conflicting opaque replay error = %v", err)
	}
	assertLegacyImportCounts(t, fixture.database, []int{1, 1, 1, 1, 1, 1})
	insertLegacyCompatibilityArtifact(
		t,
		fixture.database,
		"legacy-compatible-before-activation",
	)

	second, err := legacyimporteffect.NewSemanticImportService().Apply(
		ctx,
		store,
		runtime,
		prepared,
	)
	if err != nil {
		t.Fatalf("SemanticImportService.Apply(replay): %v", err)
	}
	if second.TypedReceipt().Disposition() != typedmemorystore.CommitReplay {
		t.Fatalf(
			"typed replay disposition = %s, want replay",
			second.TypedReceipt().Disposition(),
		)
	}
	if second.Record().Ref() != first.Record().Ref() {
		t.Fatal("semantic import replay returned another durable record")
	}
	assertLegacyImportCounts(t, fixture.database, []int{1, 1, 1, 1, 1, 1})

	activateLegacySingleWriteFixture(t, fixture.database, first)
	assertLegacyWriteMode(t, fixture.database, "typed_single_write")
	_, err = fixture.database.Exec(
		legacyCompatibilityArtifactInsertSQL,
		"legacy-write-after-activation",
		"2026-07-19T00:00:00Z",
		"2026-07-19T00:00:00Z",
	)
	if err == nil {
		t.Fatal("typed single-write guard accepted a new legacy artifact")
	}
	if !strings.Contains(err.Error(), "legacy semantic writes are disabled") {
		t.Fatalf("legacy write guard error = %v", err)
	}

	if _, err := fixture.database.Exec(
		`DROP TRIGGER legacy_import_dispositions_no_delete`,
	); err != nil {
		t.Fatalf("drop append-only trigger for tamper fixture: %v", err)
	}
	if _, err := fixture.database.Exec(
		`DELETE FROM legacy_import_dispositions`,
	); err != nil {
		t.Fatalf("tamper legacy import footprint: %v", err)
	}
	_, err = legacyimporteffect.NewApplyService().Apply(ctx, store, opaque)
	if !errors.Is(err, legacyimporteffect.ErrImportReplayCorrupt) {
		t.Fatalf("tampered exact replay error = %v", err)
	}
}

func newLegacyImportAdmissionRuntime(
	t *testing.T,
	fixture genesisE2EFixture,
) (projectmemory.AdmissionRuntime, typedmemory.BoundedContextRef) {
	t.Helper()
	ctx := context.Background()
	resolver := genesisE2EProjectRuntimeResolver(t, fixture)
	loader, err := typedmemorystore.NewProjectAwareSQLiteCurrentProjectSnapshotLoader(
		fixture.database,
		projectmemory.NewBaseTypeEnvLoader(),
		resolver,
	)
	if err != nil {
		t.Fatalf("NewProjectAwareSQLiteCurrentProjectSnapshotLoader(): %v", err)
	}
	current, err := loader.LoadCurrentProjectSnapshot(ctx, fixture.project)
	if err != nil {
		t.Fatalf("LoadCurrentProjectSnapshot(): %v", err)
	}
	contexts := current.Environment().BoundedContexts()
	if len(contexts) == 0 {
		t.Fatal("selected TypeEnv exposes no bounded context")
	}
	source := newGenesisE2ECurrentProjectBasisSource(t, loader)
	clock := &genesisE2EClock{
		value: time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC),
	}
	adapter, err := typedmemorystore.NewProjectExecutableGenericSQLiteAdapterBuilder(
		fixture.database,
	).
		SetTypeEnvLoader(projectmemory.NewBaseTypeEnvLoader()).
		SetClock(clock).
		SetReferenceEngine(genesisE2EUnexpectedReferenceEngine{}).
		SetObservableInputs(genesisE2EUnexpectedObservableProvider{}).
		SetSelectedProjectRuntime(resolver).
		Build()
	if err != nil {
		t.Fatalf("build project executable adapter: %v", err)
	}
	runtime, err := projectmemory.NewAdmissionRuntime(
		fixture.project,
		source,
		adapter,
	)
	if err != nil {
		t.Fatalf("NewAdmissionRuntime(): %v", err)
	}
	return runtime, contexts[0].Ref()
}

func newLegacyImportApplyRequest(
	t *testing.T,
	project projectidentity.ProjectID,
	suffix string,
) (
	legacyimporteffect.ImportApplyRequest,
	legacyimport.CarrierSnapshot,
) {
	t.Helper()
	coordinate, err := legacyimport.NewSourceCoordinate("source:" + suffix)
	if err != nil {
		t.Fatalf("NewSourceCoordinate(): %v", err)
	}
	carrierRef, err := typedmemory.NewCarrierRef("carrier:" + suffix)
	if err != nil {
		t.Fatalf("NewCarrierRef(): %v", err)
	}
	edition, err := typedmemory.NewCarrierEdition("edition:1")
	if err != nil {
		t.Fatalf("NewCarrierEdition(): %v", err)
	}
	format, err := legacyimport.NewCarrierFormat("application/json")
	if err != nil {
		t.Fatalf("NewCarrierFormat(): %v", err)
	}
	legacyRef, err := legacyimport.NewLegacyIdentityRef("legacy:" + suffix)
	if err != nil {
		t.Fatalf("NewLegacyIdentityRef(): %v", err)
	}
	identity, err := legacyimport.NewIdentifiedLegacyCarrier(legacyRef)
	if err != nil {
		t.Fatalf("NewIdentifiedLegacyCarrier(): %v", err)
	}
	carrier, err := legacyimport.NewCarrierSnapshot(
		coordinate,
		carrierRef,
		edition,
		format,
		[]byte(`{"legacy":"opaque"}`),
		identity,
	)
	if err != nil {
		t.Fatalf("NewCarrierSnapshot(): %v", err)
	}
	subject, err := legacyimport.NewSemanticSubjectRef(
		"legacy-subject:" + suffix,
	)
	if err != nil {
		t.Fatalf("NewSemanticSubjectRef(): %v", err)
	}
	observation, err := legacyimport.NewCarrierObservation(subject, carrier)
	if err != nil {
		t.Fatalf("NewCarrierObservation(): %v", err)
	}
	classification, err := legacyimport.NewCarrierOnly(
		subject,
		[]legacyimport.CarrierObservation{observation},
	)
	if err != nil {
		t.Fatalf("NewCarrierOnly(): %v", err)
	}
	catalog, err := legacyimport.NewCarrierCatalog(
		[]legacyimport.CarrierSnapshot{carrier},
	)
	if err != nil {
		t.Fatalf("NewCarrierCatalog(): %v", err)
	}
	observations, err := legacyimport.NewObservationSet(
		[]legacyimport.SubjectObservation{observation},
	)
	if err != nil {
		t.Fatalf("NewObservationSet(): %v", err)
	}
	source, err := legacyimport.NewLegacySourceSnapshot(catalog, observations)
	if err != nil {
		t.Fatalf("NewLegacySourceSnapshot(): %v", err)
	}
	classifier, err := legacyimport.NewClassifierVersion(
		"legacy-import-classifier.v1",
	)
	if err != nil {
		t.Fatalf("NewClassifierVersion(): %v", err)
	}
	report, err := legacyimport.NewDryRunReport(
		project,
		classifier,
		source,
		[]legacyimport.SubjectClassification{classification},
	)
	if err != nil {
		t.Fatalf("NewDryRunReport(): %v", err)
	}
	plan, err := legacyimport.NewImportPlan(report)
	if err != nil {
		t.Fatalf("NewImportPlan(): %v", err)
	}
	key, err := legacyimporteffect.NewImportIdempotencyKey(
		"opaque-import:" + suffix,
	)
	if err != nil {
		t.Fatalf("NewImportIdempotencyKey(): %v", err)
	}
	request, err := legacyimporteffect.NewImportApplyRequest(plan, key)
	if err != nil {
		t.Fatalf("NewImportApplyRequest(): %v", err)
	}
	return request, carrier
}

func newLegacyImportBridge(
	t *testing.T,
	project projectidentity.ProjectID,
	carrier legacyimport.CarrierSnapshot,
	contextRef typedmemory.BoundedContextRef,
) legacydualread.IdentityBridge {
	t.Helper()
	identity, ok :=
		carrier.LegacyIdentity().(legacyimport.IdentifiedLegacyCarrier)
	if !ok {
		t.Fatal("legacy carrier has no exact identity")
	}
	basis, err := legacydualread.NewMappingCarrierBasis(
		carrier.Ref(),
		carrier.Edition(),
		carrier.Digest(),
	)
	if err != nil {
		t.Fatalf("NewMappingCarrierBasis(): %v", err)
	}
	entity, err := typedmemory.NewEntityID(
		"entity:genesis-e2e-project-current",
	)
	if err != nil {
		t.Fatalf("NewEntityID(): %v", err)
	}
	bridge, err := legacydualread.NewIdentityBridge(
		legacydualread.IdentityBridgeInput{
			Project: project,
			Legacy:  identity.Ref(),
			Entity:  entity,
			Context: contextRef,
			Basis:   basis,
		},
	)
	if err != nil {
		t.Fatalf("NewIdentityBridge(): %v", err)
	}
	return bridge
}

func assertLegacyImportCounts(
	t *testing.T,
	database *sql.DB,
	want []int,
) {
	t.Helper()
	tables := []string{
		"legacy_import_runs",
		"legacy_import_carriers",
		"legacy_import_run_carriers",
		"legacy_import_dispositions",
		"legacy_semantic_imports",
		"legacy_identity_bridges",
	}
	got := make([]int, len(tables))
	for index, table := range tables {
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
		if err := database.QueryRow(query).Scan(&got[index]); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("legacy import counts = %v, want %v", got, want)
	}
}

func assertLegacyWriteMode(
	t *testing.T,
	database *sql.DB,
	want string,
) {
	t.Helper()
	var got string
	err := database.QueryRow(
		`SELECT mode
		 FROM legacy_semantic_write_policy
		 WHERE singleton = 1`,
	).Scan(&got)
	if err != nil {
		t.Fatalf("load legacy semantic write mode: %v", err)
	}
	if got != want {
		t.Fatalf("legacy semantic write mode = %q, want %q", got, want)
	}
}

func activateLegacySingleWriteFixture(
	t *testing.T,
	database *sql.DB,
	result legacyimporteffect.SemanticImportResult,
) {
	t.Helper()
	opaque := result.OpaqueImport().Receipt()
	basis := opaque.SelectedProjectTypeEnv()
	record := result.Record()
	_, err := database.Exec(
		`UPDATE legacy_semantic_write_policy
		 SET
			mode = 'typed_single_write',
			activation_semantic_import_ref = ?,
			activation_import_receipt_ref = ?,
			activation_head_ref = ?,
			activation_head_revision = ?,
			activation_type_env_ref = ?,
			activation_graph_revision = ?,
			activation_graph_commit_ref = ?
		 WHERE singleton = 1`,
		record.Ref(),
		opaque.Ref().String(),
		basis.HeadRef().String(),
		basis.HeadRevision().Value(),
		basis.TypeEnvRef().String(),
		record.GraphRevision().Value(),
		record.GraphCommitRef(),
	)
	if err != nil {
		t.Fatalf("activate typed single-write fixture: %v", err)
	}
}

const legacyCompatibilityArtifactInsertSQL = `
INSERT INTO artifacts (
	id,
	kind,
	title,
	content,
	created_at,
	updated_at
) VALUES (?, 'Note', 'legacy compatibility fixture', 'opaque', ?, ?)
`

func insertLegacyCompatibilityArtifact(
	t *testing.T,
	database *sql.DB,
	id string,
) {
	t.Helper()
	_, err := database.Exec(
		legacyCompatibilityArtifactInsertSQL,
		id,
		"2026-07-19T00:00:00Z",
		"2026-07-19T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert compatibility legacy artifact: %v", err)
	}
}
