package db

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTypedMemoryKindClassificationMigration54PreservesV53HistoryByteExactly(
	t *testing.T,
) {
	t.Parallel()

	database, _, fixture := openDatabaseBeforeRelationalAssertion53(t, true)
	defer database.Close()
	if err := Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryRelationalAssertionMigration53},
	); err != nil {
		t.Fatalf("migrate fixture to v53: %v", err)
	}
	eventBefore := loadTypedMemoryGraphEventSnapshot53(t, database, fixture.eventRef)
	writerBefore := loadTypedMemoryWriterGenerationSnapshot53(t, database, fixture.eventRef)
	relationBefore := loadTypedMemoryLegacyRelationSnapshot53(t, database, fixture.eventRef)
	var basisBefore []byte
	var closureBefore []byte
	err := database.QueryRow(
		`SELECT canonical_admission_basis_bytes
		FROM typed_memory_event_admission_bases
		WHERE project_id = ? AND event_ref = ?`,
		fixture.projectID,
		fixture.eventRef,
	).Scan(&basisBefore)
	if err != nil {
		t.Fatalf("load v53 basis bytes: %v", err)
	}
	err = database.QueryRow(
		`SELECT canonical_materialization_bytes
		FROM typed_memory_commit_materialization_closures
		WHERE project_id = ? AND event_ref = ?`,
		fixture.projectID,
		fixture.eventRef,
	).Scan(&closureBefore)
	if err != nil {
		t.Fatalf("load v53 closure bytes: %v", err)
	}

	if err := Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryKindClassificationMigration54},
	); err != nil {
		t.Fatalf("migrate v53 fixture through v54: %v", err)
	}

	eventAfter := loadTypedMemoryGraphEventSnapshot53(t, database, fixture.eventRef)
	writerAfter := loadTypedMemoryWriterGenerationSnapshot53(t, database, fixture.eventRef)
	relationAfter := loadTypedMemoryLegacyRelationSnapshot53(t, database, fixture.eventRef)
	if !reflect.DeepEqual(eventAfter, eventBefore) {
		t.Fatalf("v54 changed historical graph event: before=%+v after=%+v", eventBefore, eventAfter)
	}
	if writerAfter != writerBefore {
		t.Fatalf("v54 changed historical writer row: before=%+v after=%+v", writerBefore, writerAfter)
	}
	if !reflect.DeepEqual(relationAfter, relationBefore) {
		t.Fatalf("v54 changed historical relation row: before=%+v after=%+v", relationBefore, relationAfter)
	}
	var basisAfter []byte
	var closureAfter []byte
	var sourceCount int
	var evaluationCount int
	var featureCount int
	var useCount int
	err = database.QueryRow(
		`SELECT admission.canonical_admission_basis_bytes,
			closure.canonical_materialization_bytes,
			closure.kind_classification_source_blob_count,
			closure.kind_classification_evaluation_count,
			closure.kind_classification_feature_count,
			closure.kind_classification_use_count
		FROM typed_memory_event_admission_bases admission
		JOIN typed_memory_commit_materialization_closures closure
			ON closure.project_id = admission.project_id
			AND closure.event_ref = admission.event_ref
		WHERE admission.project_id = ? AND admission.event_ref = ?`,
		fixture.projectID,
		fixture.eventRef,
	).Scan(
		&basisAfter,
		&closureAfter,
		&sourceCount,
		&evaluationCount,
		&featureCount,
		&useCount,
	)
	if err != nil {
		t.Fatalf("load v54 preserved carriers: %v", err)
	}
	if !bytes.Equal(basisAfter, basisBefore) || !bytes.Equal(closureAfter, closureBefore) {
		t.Fatal("v54 rewrote historical admission or materialization bytes")
	}
	if sourceCount != 0 || evaluationCount != 0 || featureCount != 0 || useCount != 0 {
		t.Fatalf(
			"v54 invented current classification rows for history: %d/%d/%d/%d",
			sourceCount,
			evaluationCount,
			featureCount,
			useCount,
		)
	}
	assertNoForeignKeyViolationsV38(t, database)
}

func TestTypedMemoryKindClassificationMigration54InstallsExactCurrentBoundary(
	t *testing.T,
) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "v54-fresh.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()
	database := store.conn
	var admissionSQL string
	var closureSQL string
	err = database.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'typed_memory_event_admission_bases'",
	).Scan(&admissionSQL)
	if err != nil {
		t.Fatalf("load current admission table: %v", err)
	}
	err = database.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'typed_memory_commit_materialization_closures'",
	).Scan(&closureSQL)
	if err != nil {
		t.Fatalf("load current closure table: %v", err)
	}
	if !strings.Contains(admissionSQL, "context_slice_classification") ||
		!strings.Contains(closureSQL, "kind_classification_use_count") {
		t.Fatal("v54 did not install the current classification basis and exact footprint")
	}
	var generation int
	var digest string
	var marker []byte
	err = database.QueryRow(
		`SELECT writer_generation, capability_digest, canonical_bytes
		FROM typed_memory_writer_capabilities_v54
		WHERE capability_key = ?`,
		typedMemoryKindClassificationCapabilityKey54,
	).Scan(&generation, &digest, &marker)
	if err != nil {
		t.Fatalf("load v54 capability: %v", err)
	}
	if generation != 54 || digest != typedMemoryKindClassificationMarkerDigest54 ||
		!bytes.Equal(marker, []byte(typedMemoryKindClassificationMarkerBytes54)) {
		t.Fatal("v54 capability differs from its sealed marker")
	}
	assertNoForeignKeyViolationsV38(t, database)
}
