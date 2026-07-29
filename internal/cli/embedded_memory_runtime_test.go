package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	_ "modernc.org/sqlite"
)

func TestOpenFPFDBExtractsImmutableReadOnlyImage(t *testing.T) {
	database, cleanup, err := openFPFDB()
	if err != nil {
		t.Fatalf("openFPFDB() error = %v", err)
	}
	databasePath := embeddedDatabasePath(t, database)
	t.Cleanup(cleanup)

	info, err := os.Stat(databasePath)
	if err != nil {
		t.Fatalf("stat extracted database: %v", err)
	}
	if got := info.Mode().Perm(); got != 0400 {
		t.Fatalf("extracted database mode = %04o, want 0400", got)
	}
	if got := database.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("maximum open connections = %d, want 1", got)
	}
	var queryOnly int
	if err := database.QueryRow(`PRAGMA query_only`).Scan(&queryOnly); err != nil {
		t.Fatalf("read query_only pragma: %v", err)
	}
	if queryOnly != 1 {
		t.Fatalf("query_only = %d, want 1", queryOnly)
	}

	before, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatalf("read extracted database before rejected writes: %v", err)
	}
	if _, err := database.Exec(`CREATE TABLE forbidden_write (id INTEGER)`); err == nil {
		t.Fatal("CREATE unexpectedly succeeded against embedded database")
	}
	if _, err := database.Exec(`INSERT INTO meta (key, value) VALUES ('forbidden', 'write')`); err == nil {
		t.Fatal("INSERT unexpectedly succeeded against embedded database")
	}
	after, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatalf("read extracted database after rejected writes: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf(
			"extracted database bytes changed: before=%x after=%x",
			sha256.Sum256(before),
			sha256.Sum256(after),
		)
	}

	tempDir := filepath.Dir(databasePath)
	cleanup()
	if _, err := os.Stat(tempDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cleanup left extracted directory %q: %v", tempDir, err)
	}
}

func TestLoadEmbeddedMemoryRuntimeReturnsExactExecutableEnvironment(t *testing.T) {
	expectedArtifact, expectedRevision, expectedReference := embeddedArtifactExpectation(t)
	runtime, err := loadEmbeddedMemoryRuntime(context.Background())
	if err != nil {
		t.Fatalf("loadEmbeddedMemoryRuntime() error = %v", err)
	}

	artifact := runtime.Artifact()
	reference, hasReference := artifact.TypeEnvRef()
	if !hasReference {
		t.Fatal("embedded artifact has no TypeEnvRef")
	}
	if got := artifact.Digest().String(); got != expectedArtifact.Digest().String() {
		t.Fatalf("artifact digest = %q, want %q", got, expectedArtifact.Digest().String())
	}
	if !bytes.Equal(artifact.CanonicalBytes(), expectedArtifact.CanonicalBytes()) {
		t.Fatal("runtime artifact differs from the exact canonical artifact embedded in fpf.db")
	}
	if got := reference.String(); got != expectedReference {
		t.Fatalf("artifact TypeEnvRef = %q, want embedded metadata %q", got, expectedReference)
	}
	if got := artifact.SourceRevision().String(); got != expectedRevision {
		t.Fatalf("artifact source revision = %q, want embedded metadata %q", got, expectedRevision)
	}

	environment := runtime.Environment()
	if got := environment.Ref().String(); got != reference.String() {
		t.Fatalf("runtime TypeEnvRef = %q, want artifact %q", got, reference.String())
	}
	if got := environment.SourceRevision().String(); got != artifact.SourceRevision().String() {
		t.Fatalf("runtime source revision = %q, want artifact %q", got, artifact.SourceRevision().String())
	}
	if got := environment.CompilerSchemaVersion().String(); got != artifact.CompilerSchemaVersion().String() {
		t.Fatalf("runtime compiler = %q, want artifact %q", got, artifact.CompilerSchemaVersion().String())
	}

	registry := runtime.CodecRegistry()
	for _, binding := range environment.ValueBindings() {
		if _, exists := environment.ValueShape(binding.ValueShape()); !exists {
			t.Fatalf("ValueBinding %s has no declared shape %s", binding.ValueKind(), binding.ValueShape())
		}
		if !registry.Contains(binding.Codec()) {
			t.Fatalf("ValueBinding %s has no runtime codec %s", binding.ValueKind(), binding.Codec())
		}
	}

	expectedEnvironment, expectedRegistry, err := typeenv.LowerBaseTypeEnvArtifactWithCodecs(expectedArtifact)
	if err != nil {
		t.Fatalf("lower exact embedded artifact: %v", err)
	}
	if environment.Ref() != expectedEnvironment.Ref() {
		t.Fatalf("runtime environment ref = %q, want direct lowering %q", environment.Ref(), expectedEnvironment.Ref())
	}
	if registry.Len() != expectedRegistry.Len() {
		t.Fatalf("runtime codec count = %d, want direct lowering %d", registry.Len(), expectedRegistry.Len())
	}
}

func TestLoadEmbeddedMemoryRuntimeRejectsMetadataMismatch(t *testing.T) {
	testCases := []struct {
		name string
		key  string
	}{
		{name: "schema", key: "schema_version"},
		{name: "publication revision", key: "fpf_commit"},
		{name: "TypeEnv source revision", key: "typeenv_source_revision"},
		{name: "TypeEnv digest", key: "typeenv_artifact_digest"},
		{name: "TypeEnv reference", key: "typeenv_ref"},
		{name: "compiler", key: "typeenv_compiler_schema_version"},
		{name: "posture", key: "typeenv_posture"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			database := writableEmbeddedFPFDatabase(t)
			if _, err := database.Exec(
				`UPDATE meta SET value = ? WHERE key = ?`,
				"mismatch",
				testCase.key,
			); err != nil {
				t.Fatalf("tamper %s metadata: %v", testCase.key, err)
			}

			_, err := loadEmbeddedMemoryRuntimeFromDB(context.Background(), database)
			if err == nil {
				t.Fatalf("metadata mismatch %q unexpectedly accepted", testCase.key)
			}
			if !strings.Contains(err.Error(), testCase.key) {
				t.Fatalf("error %q does not identify mismatched key %q", err, testCase.key)
			}
		})
	}
}

func TestLoadEmbeddedMemoryRuntimeRejectsSourceProjectionMismatch(t *testing.T) {
	database := writableEmbeddedFPFDatabase(t)
	result, err := database.Exec(`
		UPDATE source_units
		SET content_hash = 'sha256:tampered'
		WHERE unit_id = (
			SELECT unit_id
			FROM fpf_typeenv_sources
			ORDER BY unit_id
			LIMIT 1
		)
	`)
	if err != nil {
		t.Fatalf("tamper source-unit projection: %v", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("read tamper row count: %v", err)
	}
	if changed != 1 {
		t.Fatalf("tampered source-unit rows = %d, want 1", changed)
	}

	_, err = loadEmbeddedMemoryRuntimeFromDB(context.Background(), database)
	if err == nil {
		t.Fatal("source-unit mismatch unexpectedly accepted")
	}
	if !strings.Contains(err.Error(), "content hash mismatch") {
		t.Fatalf("source mismatch error = %q", err)
	}
}

func TestLoadEmbeddedMemoryRuntimeDoesNotCreateMissingSchema(t *testing.T) {
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatalf("open empty database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	_, err = loadEmbeddedMemoryRuntimeFromDB(context.Background(), database)
	if err == nil {
		t.Fatal("database without TypeEnv artifact unexpectedly accepted")
	}
	var tableCount int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table'`,
	).Scan(&tableCount); err != nil {
		t.Fatalf("count tables after failed load: %v", err)
	}
	if tableCount != 0 {
		t.Fatalf("read-only loader created %d table(s)", tableCount)
	}
}

func TestLoadEmbeddedMemoryRuntimeRejectsNilInputs(t *testing.T) {
	if _, err := loadEmbeddedMemoryRuntime(nil); err == nil {
		t.Fatal("nil context unexpectedly accepted")
	}
	if _, err := loadEmbeddedMemoryRuntimeFromDB(context.Background(), nil); err == nil {
		t.Fatal("nil database unexpectedly accepted")
	}
}

func embeddedDatabasePath(t *testing.T, database *sql.DB) string {
	t.Helper()
	rows, err := database.Query(`PRAGMA database_list`)
	if err != nil {
		t.Fatalf("query database list: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var sequence int
		var name string
		var path string
		if err := rows.Scan(&sequence, &name, &path); err != nil {
			t.Fatalf("scan database list: %v", err)
		}
		if name == "main" {
			return path
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate database list: %v", err)
	}
	t.Fatal("main database path was not reported")
	return ""
}

func embeddedArtifactExpectation(
	t *testing.T,
) (typeenv.BaseTypeEnvArtifact, string, string) {
	t.Helper()
	database, cleanup, err := openFPFDB()
	if err != nil {
		t.Fatalf("open embedded database for exact expectation: %v", err)
	}
	defer cleanup()

	var canonical []byte
	if err := database.QueryRow(
		`SELECT canonical_bytes FROM fpf_typeenv_artifact WHERE singleton = 1`,
	).Scan(&canonical); err != nil {
		t.Fatalf("read exact embedded artifact: %v", err)
	}
	artifact, err := typeenv.DecodeBaseTypeEnvArtifact(canonical)
	if err != nil {
		t.Fatalf("decode exact embedded artifact: %v", err)
	}
	reference, hasReference := artifact.TypeEnvRef()
	if !hasReference {
		t.Fatal("exact embedded artifact has no TypeEnvRef")
	}

	metadata := map[string]string{}
	for _, key := range []string{"fpf_commit", "typeenv_ref"} {
		var value string
		if err := database.QueryRow(
			`SELECT value FROM meta WHERE key = ?`,
			key,
		).Scan(&value); err != nil {
			t.Fatalf("read exact embedded metadata %s: %v", key, err)
		}
		metadata[key] = value
	}
	if metadata["fpf_commit"] != artifact.SourceRevision().String() {
		t.Fatalf(
			"embedded fpf_commit = %q, want artifact source revision %q",
			metadata["fpf_commit"],
			artifact.SourceRevision(),
		)
	}
	if metadata["typeenv_ref"] != reference.String() {
		t.Fatalf(
			"embedded typeenv_ref = %q, want artifact reference %q",
			metadata["typeenv_ref"],
			reference,
		)
	}
	return artifact, metadata["fpf_commit"], metadata["typeenv_ref"]
}

func writableEmbeddedFPFDatabase(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fpf.db")
	if err := os.WriteFile(path, embeddedFPFDB, 0600); err != nil {
		t.Fatalf("write embedded database fixture: %v", err)
	}
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open embedded database fixture: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}
