package projecttypeenvstage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstore"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

func TestStageRecordMetadataCompatibilityIsLimitedToKnownHistoricalV3Bug(
	t *testing.T,
) {
	expected := stageRecord{
		ref:             "stage",
		digest:          "digest",
		project:         "project",
		verificationRef: "verification",
		executableRef:   "executable",
		canonicalSchema: projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV3,
		canonical:       []byte("canonical"),
	}
	historical := expected.clone()
	historical.canonicalSchema = projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV2
	if !stageRecordMatchesCanonical(
		expected,
		historical,
		projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV3,
	) {
		t.Fatal("known historical v3 row mislabeled as v2 is not inspectable")
	}
	current := expected.clone()
	current.canonicalSchema = projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV4
	if stageRecordMatchesCanonical(
		current,
		historical,
		projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV4,
	) {
		t.Fatal("historical metadata exception broadened to current v4 Stage")
	}
}

func TestPutStageRecordIsIdempotentForKnownHistoricalV3MetadataBug(t *testing.T) {
	fixture := newStageStoreFixture(t, "historical-v3-retry")
	ctx := context.Background()
	if err := fixture.store.Put(
		ctx,
		fixture.domain.stage,
		fixture.domain.verification,
		fixture.domain.snapshot,
	); err != nil {
		t.Fatalf("seed Stage closure: %v", err)
	}

	expected, _, _, err := preparePersistedStage(
		fixture.domain.stage,
		fixture.domain.verification,
		fixture.domain.snapshot,
	)
	if err != nil {
		t.Fatalf("prepare Stage record: %v", err)
	}
	expected.digest = "sha256:" + strings.Repeat("a", 64)
	expected.ref = "project-typeenv-stage:" + expected.digest
	expected.canonicalSchema = projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV3
	expected.canonical = []byte("historical-v3-canonical")
	historical := expected.clone()
	historical.canonicalSchema = projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV2

	_, err = fixture.database.ExecContext(
		ctx,
		`INSERT INTO project_typeenv_stages (
			stage_ref,
			stage_digest,
			project_id,
			composite_verification_ref,
			executable_type_env_ref,
			canonical_schema_version,
			canonical_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		historical.ref,
		historical.digest,
		historical.project,
		historical.verificationRef,
		historical.executableRef,
		historical.canonicalSchema,
		historical.canonical,
	)
	if err != nil {
		t.Fatalf("insert historical Stage metadata fixture: %v", err)
	}

	transaction, err := fixture.database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin historical Stage retry: %v", err)
	}
	defer func() { _ = transaction.Rollback() }()
	if err := putStageRecord(ctx, transaction, expected); err != nil {
		t.Fatalf("retry historical Stage: %v", err)
	}
	if err := transaction.Commit(); err != nil {
		t.Fatalf("commit historical Stage retry: %v", err)
	}

	var storedSchema string
	if err := fixture.database.QueryRowContext(
		ctx,
		`SELECT canonical_schema_version
		 FROM project_typeenv_stages
		 WHERE stage_ref = ?`,
		expected.ref,
	).Scan(&storedSchema); err != nil {
		t.Fatalf("read historical Stage metadata after retry: %v", err)
	}
	if storedSchema != projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV2 {
		t.Fatalf(
			"historical Stage metadata changed to %q; want preserved v2 label",
			storedSchema,
		)
	}
}

func TestStorePutGetIsImmutableIdempotentAndDataOnly(t *testing.T) {
	fixture := newStageStoreFixture(t, "a")
	ctx := context.Background()
	if err := fixture.store.PutArtifactClosure(ctx, fixture.domain.closure); err != nil {
		t.Fatalf("PutArtifactClosure(): %v", err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := fixture.store.Put(
			ctx,
			fixture.domain.stage,
			fixture.domain.verification,
			fixture.domain.snapshot,
		); err != nil {
			t.Fatalf("Put(attempt %d): %v", attempt+1, err)
		}
	}
	persisted, err := fixture.store.Get(ctx, fixture.domain.stage.Ref())
	if err != nil {
		t.Fatalf("Get(): %v", err)
	}
	if err := persisted.Verify(); err != nil {
		t.Fatalf("PersistedStage.Verify(): %v", err)
	}
	if !bytes.Equal(
		persisted.Stage().CanonicalBytes(),
		fixture.domain.stage.CanonicalBytes(),
	) {
		t.Fatalf("plain Get changed Stage bytes")
	}
	if !bytes.Equal(
		persisted.VerificationRecord().CanonicalBytes(),
		fixture.domain.verification.CanonicalBytes(),
	) {
		t.Fatalf("plain Get changed verification bytes")
	}
	if !bytes.Equal(
		persisted.ExecutableSnapshotRecord().CanonicalBytes(),
		fixture.domain.snapshot.CanonicalBytes(),
	) {
		t.Fatalf("plain Get changed executable snapshot bytes")
	}
	if err := (SelectionReadyStage{}).Verify(); err == nil ||
		!strings.Contains(err.Error(), "capability") {
		t.Fatalf("zero selection-ready result verified: %v", err)
	}
	var storedSchema string
	err = fixture.database.QueryRowContext(
		ctx,
		`SELECT canonical_schema_version
		   FROM project_typeenv_stages
		  WHERE stage_ref = ?`,
		fixture.domain.stage.Ref().String(),
	).Scan(&storedSchema)
	if err != nil {
		t.Fatalf("read stored Stage schema metadata: %v", err)
	}
	if storedSchema != fixture.domain.stage.SchemaEdition() {
		t.Fatalf(
			"stored Stage schema metadata = %q; want exact canonical edition %q",
			storedSchema,
			fixture.domain.stage.SchemaEdition(),
		)
	}
	assertStageStoreRowCount(t, fixture, "project_typeenv_stages", 1)
	assertStageStoreRowCount(t, fixture, "project_typeenv_composite_verifications", 1)
	assertStageStoreRowCount(t, fixture, "project_typeenv_executable_snapshots", 1)
}

func TestLoadExecutableSnapshotTxRestoresExactImmutableCWithoutStageLookup(
	t *testing.T,
) {
	fixture := newStageStoreFixture(t, "load-executable-by-c")
	ctx := context.Background()
	if err := fixture.store.PutArtifactClosure(ctx, fixture.domain.closure); err != nil {
		t.Fatalf("PutArtifactClosure(): %v", err)
	}
	if err := fixture.store.Put(
		ctx,
		fixture.domain.stage,
		fixture.domain.verification,
		fixture.domain.snapshot,
	); err != nil {
		t.Fatalf("Put(): %v", err)
	}
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead(): %v", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	snapshot, err := fixture.store.LoadExecutableSnapshotTx(
		ctx,
		transaction,
		fixture.domain.snapshot.TypeEnvRef(),
	)
	if err != nil {
		t.Fatalf("LoadExecutableSnapshotTx(): %v", err)
	}
	if err := snapshot.Verify(); err != nil {
		t.Fatalf("restored snapshot Verify(): %v", err)
	}
	if snapshot.TypeEnvRef() != fixture.domain.snapshot.TypeEnvRef() ||
		!bytes.Equal(
			snapshot.Record().CanonicalBytes(),
			fixture.domain.snapshot.CanonicalBytes(),
		) {
		t.Fatal("C-addressed executable reload changed snapshot identity")
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		t.Fatalf("commit executable reload: %v", finish.Err())
	}
}

func TestStoreLoadSelectionReadyReusesExactReconstruction(t *testing.T) {
	fixture := newStageStoreFixture(t, "b")
	ctx := context.Background()
	if err := fixture.store.PutArtifactClosure(ctx, fixture.domain.closure); err != nil {
		t.Fatalf("PutArtifactClosure(): %v", err)
	}
	if err := fixture.store.Put(
		ctx,
		fixture.domain.stage,
		fixture.domain.verification,
		fixture.domain.snapshot,
	); err != nil {
		t.Fatalf("Put(): %v", err)
	}
	ready, err := fixture.store.LoadSelectionReady(ctx, fixture.domain.stage.Ref())
	if err != nil {
		t.Fatalf("LoadSelectionReady(): %v", err)
	}
	if err := ready.Verify(); err != nil {
		t.Fatalf("SelectionReadyStage.Verify(): %v", err)
	}
	if err := ready.FinalLowererVerification().Verify(); err != nil {
		t.Fatalf("restored final-lowerer capability: %v", err)
	}
	if ready.Stage().Ref() != fixture.domain.stage.Ref() ||
		ready.FinalLowererVerification().Ref() != fixture.domain.verification.Ref() ||
		ready.ExecutableSnapshot().Digest() != fixture.domain.snapshot.Digest() {
		t.Fatalf("selection-ready result changed Stage or verification identity")
	}
	cached, err := fixture.store.LoadSelectionReady(ctx, fixture.domain.stage.Ref())
	if err != nil {
		t.Fatalf("LoadSelectionReady(cached): %v", err)
	}
	if cached.capability != ready.capability {
		t.Fatal("exact reconstruction did not reuse the per-Store cache entry")
	}
	if err := cached.Verify(); err != nil {
		t.Fatalf("cached SelectionReadyStage.Verify(): %v", err)
	}
}

func TestStoreLoadSelectionReadyFailsClosedOnPersistedCorruption(t *testing.T) {
	t.Run("executable snapshot canonical bytes", func(t *testing.T) {
		fixture := newStageStoreFixture(t, "corrupt-snapshot-bytes")
		ctx := context.Background()
		if err := fixture.store.PutArtifactClosure(ctx, fixture.domain.closure); err != nil {
			t.Fatalf("PutArtifactClosure(): %v", err)
		}
		if err := fixture.store.Put(
			ctx,
			fixture.domain.stage,
			fixture.domain.verification,
			fixture.domain.snapshot,
		); err != nil {
			t.Fatalf("Put(): %v", err)
		}
		warmSelectionReadyStageCache(t, fixture)
		_, err := fixture.database.ExecContext(
			ctx,
			`UPDATE project_typeenv_executable_snapshots
			 SET canonical_bytes = ?
			 WHERE type_env_ref = ?`,
			[]byte{0x00},
			fixture.domain.snapshot.TypeEnvRef().String(),
		)
		if err != nil {
			t.Fatalf("corrupt executable snapshot canonical bytes: %v", err)
		}
		_, err = fixture.store.LoadSelectionReady(ctx, fixture.domain.stage.Ref())
		if !errors.Is(err, ErrStageIntegrity) {
			t.Fatalf(
				"LoadSelectionReady(corrupt snapshot bytes) error = %v; want ErrStageIntegrity",
				err,
			)
		}
	})

	t.Run("executable snapshot digest", func(t *testing.T) {
		fixture := newStageStoreFixture(t, "corrupt-snapshot-digest")
		ctx := context.Background()
		if err := fixture.store.PutArtifactClosure(ctx, fixture.domain.closure); err != nil {
			t.Fatalf("PutArtifactClosure(): %v", err)
		}
		if err := fixture.store.Put(
			ctx,
			fixture.domain.stage,
			fixture.domain.verification,
			fixture.domain.snapshot,
		); err != nil {
			t.Fatalf("Put(): %v", err)
		}
		warmSelectionReadyStageCache(t, fixture)
		_, err := fixture.database.ExecContext(
			ctx,
			`UPDATE project_typeenv_executable_snapshots
			 SET snapshot_digest = ?
			 WHERE type_env_ref = ?`,
			"sha256:"+strings.Repeat("f", 64),
			fixture.domain.snapshot.TypeEnvRef().String(),
		)
		if err != nil {
			t.Fatalf("corrupt executable snapshot digest: %v", err)
		}
		_, err = fixture.store.LoadSelectionReady(ctx, fixture.domain.stage.Ref())
		if !errors.Is(err, ErrStageIntegrity) {
			t.Fatalf(
				"LoadSelectionReady(corrupt snapshot digest) error = %v; want ErrStageIntegrity",
				err,
			)
		}
	})

	artifactCoordinates := func(
		fixture stageStoreFixture,
	) []struct {
		name         string
		kind         string
		ref          string
		errorContext string
	} {
		return []struct {
			name         string
			kind         string
			ref          string
			errorContext string
		}{
			{
				name:         "B",
				kind:         "base_type_env",
				ref:          fixture.domain.stage.Base().String(),
				errorContext: "reload exact Stage B",
			},
			{
				name:         "E",
				kind:         "project_type_env_extension",
				ref:          fixture.domain.stage.OrderedExtensions()[0].String(),
				errorContext: "reload exact Stage E[0]",
			},
			{
				name:         "X",
				kind:         "runtime_evaluation_basis",
				ref:          fixture.domain.stage.RuntimeBasis().String(),
				errorContext: "reload exact Stage X",
			},
			{
				name:         "C",
				kind:         "project_type_env_composite",
				ref:          fixture.domain.stage.VerifiedComposite().String(),
				errorContext: "reload exact Stage C",
			},
		}
	}
	corruptions := []struct {
		name            string
		coordinateIndex int
		column          string
		value           any
	}{
		{name: "B canonical bytes", coordinateIndex: 0, column: "canonical_bytes", value: []byte{0x00}},
		{name: "E canonical bytes", coordinateIndex: 1, column: "canonical_bytes", value: []byte{0x00}},
		{name: "X canonical bytes", coordinateIndex: 2, column: "canonical_bytes", value: []byte{0x00}},
		{name: "C canonical bytes", coordinateIndex: 3, column: "canonical_bytes", value: []byte{0x00}},
		{name: "B digest", coordinateIndex: 0, column: "artifact_digest", value: "sha256:" + strings.Repeat("f", 64)},
		{name: "E digest", coordinateIndex: 1, column: "artifact_digest", value: "sha256:" + strings.Repeat("f", 64)},
		{name: "X digest", coordinateIndex: 2, column: "artifact_digest", value: "sha256:" + strings.Repeat("f", 64)},
		{name: "C digest", coordinateIndex: 3, column: "artifact_digest", value: "sha256:" + strings.Repeat("f", 64)},
	}
	for sequence, corruption := range corruptions {
		fixture := newStageStoreFixture(
			t,
			fmt.Sprintf("corrupt-artifact-%d", sequence),
		)
		coordinate := artifactCoordinates(fixture)[corruption.coordinateIndex]
		t.Run(corruption.name, func(t *testing.T) {
			ctx := context.Background()
			if err := fixture.store.PutArtifactClosure(ctx, fixture.domain.closure); err != nil {
				t.Fatalf("PutArtifactClosure(): %v", err)
			}
			if err := fixture.store.Put(
				ctx,
				fixture.domain.stage,
				fixture.domain.verification,
				fixture.domain.snapshot,
			); err != nil {
				t.Fatalf("Put(): %v", err)
			}
			warmSelectionReadyStageCache(t, fixture)
			statement := `UPDATE project_typeenv_artifacts
				 SET ` + corruption.column + ` = ?
				 WHERE artifact_kind = ? AND artifact_ref = ?`
			_, err := fixture.database.ExecContext(
				ctx,
				statement,
				corruption.value,
				coordinate.kind,
				coordinate.ref,
			)
			if err != nil {
				t.Fatalf("corrupt exact %s: %v", coordinate.name, err)
			}
			_, err = fixture.store.LoadSelectionReady(ctx, fixture.domain.stage.Ref())
			if !errors.Is(err, projecttypeenvstore.ErrArtifactIntegrity) ||
				!strings.Contains(err.Error(), coordinate.errorContext) {
				t.Fatalf(
					"LoadSelectionReady(%s) error = %v; want ErrArtifactIntegrity with %q",
					corruption.name,
					err,
					coordinate.errorContext,
				)
			}
		})
	}
}

func TestStoreRejectsCorruptionAndVerificationSubstitution(t *testing.T) {
	t.Run("Stage canonical corruption", func(t *testing.T) {
		fixture := newStageStoreFixture(t, "c")
		ctx := context.Background()
		if err := fixture.store.PutArtifactClosure(ctx, fixture.domain.closure); err != nil {
			t.Fatalf("PutArtifactClosure(): %v", err)
		}
		if err := fixture.store.Put(
			ctx,
			fixture.domain.stage,
			fixture.domain.verification,
			fixture.domain.snapshot,
		); err != nil {
			t.Fatalf("Put(): %v", err)
		}
		warmSelectionReadyStageCache(t, fixture)
		_, err := fixture.database.ExecContext(
			ctx,
			`UPDATE project_typeenv_stages SET canonical_bytes = ? WHERE stage_ref = ?`,
			[]byte{0x00},
			fixture.domain.stage.Ref().String(),
		)
		if err != nil {
			t.Fatalf("corrupt Stage row: %v", err)
		}
		_, err = fixture.store.LoadSelectionReady(ctx, fixture.domain.stage.Ref())
		if !errors.Is(err, ErrStageIntegrity) {
			t.Fatalf("LoadSelectionReady(corrupt Stage) error = %v; want ErrStageIntegrity", err)
		}
	})

	t.Run("verification substitution", func(t *testing.T) {
		fixture := newStageStoreFixture(t, "d")
		ctx := context.Background()
		other := newStageStoreDomainFixture(t, "e")
		if err := fixture.store.Put(
			ctx,
			fixture.domain.stage,
			fixture.domain.verification,
			fixture.domain.snapshot,
		); err != nil {
			t.Fatalf("Put(first): %v", err)
		}
		if err := fixture.store.Put(ctx, other.stage, other.verification, other.snapshot); err != nil {
			t.Fatalf("Put(other): %v", err)
		}
		_, err := fixture.database.ExecContext(
			ctx,
			`UPDATE project_typeenv_stages
			 SET composite_verification_ref = ?
			 WHERE stage_ref = ?`,
			other.verification.Ref().String(),
			fixture.domain.stage.Ref().String(),
		)
		if err != nil {
			t.Fatalf("substitute verification ref: %v", err)
		}
		_, err = fixture.store.Get(ctx, fixture.domain.stage.Ref())
		if !errors.Is(err, ErrStageIntegrity) {
			t.Fatalf("Get(substituted verification) error = %v; want ErrStageIntegrity", err)
		}
	})

	t.Run("pair substitution before write", func(t *testing.T) {
		fixture := newStageStoreFixture(t, "f")
		other := newStageStoreDomainFixture(t, "g")
		err := fixture.store.Put(
			context.Background(),
			fixture.domain.stage,
			other.verification,
			other.snapshot,
		)
		if !errors.Is(err, ErrStageIntegrity) {
			t.Fatalf("Put(substituted pair) error = %v; want ErrStageIntegrity", err)
		}
		assertStageStoreRowCount(t, fixture, "project_typeenv_stages", 0)
		assertStageStoreRowCount(t, fixture, "project_typeenv_composite_verifications", 0)
	})
}

func TestStoreSelectionReadyFailsClosedForMissingArtifactClosure(t *testing.T) {
	t.Run("missing B/E/X/C closure", func(t *testing.T) {
		fixture := newStageStoreFixture(t, "h")
		ctx := context.Background()
		if err := fixture.store.Put(
			ctx,
			fixture.domain.stage,
			fixture.domain.verification,
			fixture.domain.snapshot,
		); err != nil {
			t.Fatalf("Put(): %v", err)
		}
		persisted, err := fixture.store.Get(ctx, fixture.domain.stage.Ref())
		if err != nil {
			t.Fatalf("Get(data-only Stage without artifact closure): %v", err)
		}
		if err := persisted.Verify(); err != nil {
			t.Fatalf("Verify(data-only Stage without artifact closure): %v", err)
		}
		_, err = fixture.store.LoadSelectionReady(ctx, fixture.domain.stage.Ref())
		if !errors.Is(err, projecttypeenvstore.ErrArtifactNotFound) {
			t.Fatalf("LoadSelectionReady(missing closure) error = %v", err)
		}
	})

	t.Run("missing exact E", func(t *testing.T) {
		fixture := newStageStoreFixture(t, "k")
		ctx := context.Background()
		if err := fixture.store.PutArtifactClosure(ctx, fixture.domain.closure); err != nil {
			t.Fatalf("PutArtifactClosure(): %v", err)
		}
		if err := fixture.store.Put(
			ctx,
			fixture.domain.stage,
			fixture.domain.verification,
			fixture.domain.snapshot,
		); err != nil {
			t.Fatalf("Put(): %v", err)
		}
		warmSelectionReadyStageCache(t, fixture)
		if _, err := fixture.database.ExecContext(
			ctx,
			`DELETE FROM project_typeenv_artifacts
			 WHERE artifact_kind = 'project_type_env_extension'`,
		); err != nil {
			t.Fatalf("delete exact E: %v", err)
		}
		_, err := fixture.store.LoadSelectionReady(ctx, fixture.domain.stage.Ref())
		if !errors.Is(err, projecttypeenvstore.ErrArtifactNotFound) ||
			!strings.Contains(err.Error(), "Stage E[0]") {
			t.Fatalf("LoadSelectionReady(missing E) error = %v", err)
		}
	})

	t.Run("missing runtime mechanism catalog", func(t *testing.T) {
		fixture := newStageStoreFixture(t, "i")
		ctx := context.Background()
		if err := fixture.store.PutArtifactClosure(ctx, fixture.domain.closure); err != nil {
			t.Fatalf("PutArtifactClosure(): %v", err)
		}
		if err := fixture.store.Put(
			ctx,
			fixture.domain.stage,
			fixture.domain.verification,
			fixture.domain.snapshot,
		); err != nil {
			t.Fatalf("Put(): %v", err)
		}
		warmSelectionReadyStageCache(t, fixture)
		if _, err := fixture.database.ExecContext(
			ctx,
			`DELETE FROM project_typeenv_runtime_mechanisms`,
		); err != nil {
			t.Fatalf("delete runtime mechanism catalog: %v", err)
		}
		_, err := fixture.store.LoadSelectionReady(ctx, fixture.domain.stage.Ref())
		if !errors.Is(err, projecttypeenvstore.ErrArtifactIntegrity) ||
			!strings.Contains(err.Error(), "runtime mechanism") {
			t.Fatalf("LoadSelectionReady(missing runtime catalog) error = %v", err)
		}
	})
}

func TestStorePutRollsBackVerificationWhenStageInsertFails(t *testing.T) {
	fixture := newStageStoreFixture(t, "j")
	ctx := context.Background()
	_, err := fixture.database.ExecContext(
		ctx,
		`CREATE TRIGGER force_stage_insert_failure
		 BEFORE INSERT ON project_typeenv_stages
		 BEGIN
			SELECT RAISE(ABORT, 'forced Stage insert failure');
		 END`,
	)
	if err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	err = fixture.store.Put(
		ctx,
		fixture.domain.stage,
		fixture.domain.verification,
		fixture.domain.snapshot,
	)
	if err == nil || !strings.Contains(err.Error(), "forced Stage insert failure") {
		t.Fatalf("Put() error = %v; want forced failure", err)
	}
	assertStageStoreRowCount(t, fixture, "project_typeenv_stages", 0)
	assertStageStoreRowCount(t, fixture, "project_typeenv_composite_verifications", 0)
}

func TestStoreSelectionReadyCacheDoesNotCrossDatabases(t *testing.T) {
	ctx := context.Background()
	first := newStageStoreFixture(t, "cache-database-first")
	if err := first.store.PutArtifactClosure(ctx, first.domain.closure); err != nil {
		t.Fatalf("PutArtifactClosure(first): %v", err)
	}
	if err := first.store.Put(
		ctx,
		first.domain.stage,
		first.domain.verification,
		first.domain.snapshot,
	); err != nil {
		t.Fatalf("Put(first): %v", err)
	}
	warmSelectionReadyStageCache(t, first)

	second := newStageStoreFixture(t, "cache-database-second")
	if err := second.store.Put(
		ctx,
		first.domain.stage,
		first.domain.verification,
		first.domain.snapshot,
	); err != nil {
		t.Fatalf("Put(second Stage only): %v", err)
	}
	_, err := second.store.LoadSelectionReady(ctx, first.domain.stage.Ref())
	if !errors.Is(err, projecttypeenvstore.ErrArtifactNotFound) {
		t.Fatalf(
			"LoadSelectionReady(second database without closure) error = %v; want ErrArtifactNotFound",
			err,
		)
	}
}

func warmSelectionReadyStageCache(t *testing.T, fixture stageStoreFixture) {
	t.Helper()
	ready, err := fixture.store.LoadSelectionReady(
		context.Background(),
		fixture.domain.stage.Ref(),
	)
	if err != nil {
		t.Fatalf("warm LoadSelectionReady(): %v", err)
	}
	if err := ready.Verify(); err != nil {
		t.Fatalf("warm SelectionReadyStage.Verify(): %v", err)
	}
}

func assertStageStoreRowCount(
	t *testing.T,
	fixture stageStoreFixture,
	table string,
	want int,
) {
	t.Helper()
	var count int
	query := "SELECT COUNT(*) FROM " + table
	if err := fixture.database.QueryRowContext(context.Background(), query).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if count != want {
		t.Fatalf("%s rows = %d; want %d", table, count, want)
	}
}
