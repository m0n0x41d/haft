package typeenvsql

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	_ "modernc.org/sqlite"
)

const sourceUnitFixtureSchema = `CREATE TABLE source_units (
	unit_id TEXT PRIMARY KEY,
	content_hash TEXT NOT NULL,
	source_revision TEXT NOT NULL,
	start_line INTEGER NOT NULL,
	end_line INTEGER NOT NULL,
	pattern_id TEXT NOT NULL DEFAULT ''
)`

func TestEnvelopeRoundTripKeepsCompatibilityOutsideArtifactIdentity(t *testing.T) {
	database := openFixtureDatabase(t)
	fixture := newStorageFixture(t)
	insertFixtureSourceUnit(t, database, fixture)

	initial, err := typeenv.NewCompilationEnvelope(
		fixture.artifact,
		typeenv.NewInitialCompatibilityAssessment(),
	)
	if err != nil {
		t.Fatalf("NewCompilationEnvelope(initial) error = %v", err)
	}
	if err := ReplaceEnvelopeDB(context.Background(), database, initial); err != nil {
		t.Fatalf("ReplaceEnvelopeDB(initial) error = %v", err)
	}
	loadedInitial, err := LoadEnvelopeDB(context.Background(), database)
	if err != nil {
		t.Fatalf("LoadEnvelopeDB(initial) error = %v", err)
	}
	if loadedInitial.Artifact().Digest() != fixture.artifact.Digest() {
		t.Fatalf("round-trip digest = %s, want %s", loadedInitial.Artifact().Digest().String(), fixture.artifact.Digest().String())
	}
	if _, ok := loadedInitial.Compatibility().(typeenv.InitialCompatibilityAssessment); !ok {
		t.Fatalf("round-trip compatibility = %T, want InitialCompatibilityAssessment", loadedInitial.Compatibility())
	}

	compared := comparedEnvelope(t, fixture.artifact, fixture.symbol)
	if err := ReplaceEnvelopeDB(context.Background(), database, compared); err != nil {
		t.Fatalf("ReplaceEnvelopeDB(compared) error = %v", err)
	}
	loadedCompared, err := LoadEnvelopeDB(context.Background(), database)
	if err != nil {
		t.Fatalf("LoadEnvelopeDB(compared) error = %v", err)
	}
	if loadedCompared.Artifact().Digest() != fixture.artifact.Digest() {
		t.Fatalf("compatibility changed artifact digest to %s", loadedCompared.Artifact().Digest().String())
	}
	assessment, ok := loadedCompared.Compatibility().(typeenv.ComparedCompatibilityAssessment)
	if !ok {
		t.Fatalf("round-trip compatibility = %T, want ComparedCompatibilityAssessment", loadedCompared.Compatibility())
	}
	changes := assessment.Diff().Changes()
	if len(changes) != 1 || changes[0].Symbol().String() != fixture.symbol.String() {
		t.Fatalf("round-trip compatibility changes = %+v", changes)
	}
}

func TestLoadEnvelopeAndArtifactReadOnlyDBDoNotMutateOlderIndex(t *testing.T) {
	database := openFixtureDatabase(t)
	before := countTypeEnvTables(t, database)
	if before != 0 {
		t.Fatalf("fresh older-index fixture has %d TypeEnv tables", before)
	}

	_, err := LoadEnvelopeReadOnlyDB(context.Background(), database)
	if !IsMissingArtifact(err) {
		t.Fatalf("LoadEnvelopeReadOnlyDB() error = %v, want missing artifact", err)
	}
	_, err = LoadArtifactReadOnlyDB(context.Background(), database)
	if !IsMissingArtifact(err) {
		t.Fatalf("LoadArtifactReadOnlyDB() error = %v, want missing artifact", err)
	}
	after := countTypeEnvTables(t, database)
	if after != before {
		t.Fatalf("read-only probe created %d TypeEnv tables", after-before)
	}

	fixture := newStorageFixture(t)
	insertFixtureSourceUnit(t, database, fixture)
	envelope, err := typeenv.NewCompilationEnvelope(
		fixture.artifact,
		typeenv.NewInitialCompatibilityAssessment(),
	)
	if err != nil {
		t.Fatalf("NewCompilationEnvelope() error = %v", err)
	}
	if err := ReplaceEnvelopeDB(context.Background(), database, envelope); err != nil {
		t.Fatalf("ReplaceEnvelopeDB() error = %v", err)
	}
	loadedEnvelope, err := LoadEnvelopeReadOnlyDB(context.Background(), database)
	if err != nil {
		t.Fatalf("LoadEnvelopeReadOnlyDB(populated) error = %v", err)
	}
	if loadedEnvelope.Artifact().Digest() != fixture.artifact.Digest() {
		t.Fatalf(
			"read-only envelope digest = %s, want %s",
			loadedEnvelope.Artifact().Digest().String(),
			fixture.artifact.Digest().String(),
		)
	}
	if _, ok := loadedEnvelope.Compatibility().(typeenv.InitialCompatibilityAssessment); !ok {
		t.Fatalf(
			"read-only envelope compatibility = %T, want InitialCompatibilityAssessment",
			loadedEnvelope.Compatibility(),
		)
	}
	loaded, err := LoadArtifactReadOnlyDB(context.Background(), database)
	if err != nil {
		t.Fatalf("LoadArtifactReadOnlyDB(populated) error = %v", err)
	}
	if loaded.Digest() != fixture.artifact.Digest() {
		t.Fatalf("read-only loaded digest = %s, want %s", loaded.Digest().String(), fixture.artifact.Digest().String())
	}
}

func TestLoadRejectsCanonicalAndProjectionTampering(t *testing.T) {
	tests := []struct {
		name   string
		tamper string
		args   []any
	}{
		{
			name:   "canonical payload",
			tamper: `UPDATE fpf_typeenv_artifact SET canonical_bytes = ? WHERE singleton = 1`,
			args:   []any{[]byte("not-an-artifact")},
		},
		{
			name:   "declaration projection",
			tamper: `UPDATE fpf_typeenv_declarations SET compiler_rule_id = ?`,
			args:   []any{"tampered.rule.v1"},
		},
		{
			name:   "coverage projection",
			tamper: `UPDATE fpf_typeenv_coverage SET rationale = ?`,
			args:   []any{"tampered"},
		},
		{
			name:   "projection summary",
			tamper: `UPDATE fpf_typeenv_artifact SET source_count = source_count + 1`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			database := openFixtureDatabase(t)
			fixture := newStorageFixture(t)
			insertFixtureSourceUnit(t, database, fixture)
			envelope, err := typeenv.NewCompilationEnvelope(
				fixture.artifact,
				typeenv.NewInitialCompatibilityAssessment(),
			)
			if err != nil {
				t.Fatalf("NewCompilationEnvelope() error = %v", err)
			}
			if err := ReplaceEnvelopeDB(context.Background(), database, envelope); err != nil {
				t.Fatalf("ReplaceEnvelopeDB() error = %v", err)
			}
			if _, err := database.Exec(test.tamper, test.args...); err != nil {
				t.Fatalf("tamper projection: %v", err)
			}
			if _, err := LoadEnvelopeDB(context.Background(), database); err == nil {
				t.Fatal("LoadEnvelopeDB() accepted tampered storage")
			}
		})
	}
}

func TestLoadCrossVerifiesExactSourceUnit(t *testing.T) {
	tests := []struct {
		name   string
		column string
		value  any
		want   string
	}{
		{name: "content hash", column: "content_hash", value: strings.Repeat("c", 64), want: "content hash mismatch"},
		{name: "revision", column: "source_revision", value: "another-revision", want: "revision mismatch"},
		{name: "start line", column: "start_line", value: 8, want: "line range mismatch"},
		{name: "end line", column: "end_line", value: 18, want: "line range mismatch"},
		{name: "PatternID", column: "pattern_id", value: "C.3.1", want: "PatternID mismatch"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			database := openFixtureDatabase(t)
			fixture := newStorageFixture(t)
			insertFixtureSourceUnit(t, database, fixture)
			envelope, err := typeenv.NewCompilationEnvelope(
				fixture.artifact,
				typeenv.NewInitialCompatibilityAssessment(),
			)
			if err != nil {
				t.Fatalf("NewCompilationEnvelope() error = %v", err)
			}
			if err := ReplaceEnvelopeDB(context.Background(), database, envelope); err != nil {
				t.Fatalf("ReplaceEnvelopeDB() error = %v", err)
			}
			statement := "UPDATE source_units SET " + test.column + " = ? WHERE unit_id = ?"
			if _, err := database.Exec(statement, test.value, fixture.unitID); err != nil {
				t.Fatalf("tamper source unit: %v", err)
			}
			_, err = LoadEnvelopeDB(context.Background(), database)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("LoadEnvelopeDB() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestCoverageOnlyArtifactNeverMintsStoredTypeEnvRef(t *testing.T) {
	database := openFixtureDatabase(t)
	fixture := newStorageFixture(t)
	insertFixtureSourceUnit(t, database, fixture)
	subject, err := typedmemory.SourceUnitCoverage(fixture.location.UnitID())
	if err != nil {
		t.Fatalf("SourceUnitCoverage() error = %v", err)
	}
	entry, err := typedmemory.NewSourceOnlyCoverageEntry(
		subject,
		fixture.location,
		"reference-scheme-cardinality-ambiguous",
	)
	if err != nil {
		t.Fatalf("NewSourceOnlyCoverageEntry() error = %v", err)
	}
	coverage, err := typedmemory.NewCoverageManifest([]typedmemory.CoverageEntry{entry})
	if err != nil {
		t.Fatalf("NewCoverageManifest() error = %v", err)
	}
	ir, err := typeenv.NewCoverageOnlyLinkedTypeEnvIR(
		fixture.revision,
		fixture.compiler,
		coverage,
		"no-complete-environment",
	)
	if err != nil {
		t.Fatalf("NewCoverageOnlyLinkedTypeEnvIR() error = %v", err)
	}
	artifact, err := typeenv.SealBaseTypeEnv(ir)
	if err != nil {
		t.Fatalf("SealBaseTypeEnv() error = %v", err)
	}
	envelope, err := typeenv.NewCompilationEnvelope(
		artifact,
		typeenv.NewInitialCompatibilityAssessment(),
	)
	if err != nil {
		t.Fatalf("NewCompilationEnvelope() error = %v", err)
	}
	if err := ReplaceEnvelopeDB(context.Background(), database, envelope); err != nil {
		t.Fatalf("ReplaceEnvelopeDB() error = %v", err)
	}
	var storedRef sql.NullString
	if err := database.QueryRow(`SELECT typeenv_ref FROM fpf_typeenv_artifact`).Scan(&storedRef); err != nil {
		t.Fatalf("scan stored TypeEnvRef: %v", err)
	}
	if storedRef.Valid {
		t.Fatalf("coverage-only artifact stored active TypeEnvRef %q", storedRef.String)
	}
	loaded, err := LoadEnvelopeDB(context.Background(), database)
	if err != nil {
		t.Fatalf("LoadEnvelopeDB() error = %v", err)
	}
	if _, exists := loaded.Artifact().TypeEnvRef(); exists {
		t.Fatal("coverage-only round trip minted a TypeEnvRef")
	}
}

func TestUnmaterializableCompiledArtifactCannotReplaceStoredEnvelope(t *testing.T) {
	database := openFixtureDatabase(t)
	fixture := newStorageFixture(t)
	insertFixtureSourceUnit(t, database, fixture)
	goodEnvelope, err := typeenv.NewCompilationEnvelope(
		fixture.artifact,
		typeenv.NewInitialCompatibilityAssessment(),
	)
	if err != nil {
		t.Fatalf("NewCompilationEnvelope(good) error = %v", err)
	}
	if err := ReplaceEnvelopeDB(context.Background(), database, goodEnvelope); err != nil {
		t.Fatalf("ReplaceEnvelopeDB(good) error = %v", err)
	}

	err = sealUnmaterializableCompiledArtifact(t, fixture)
	if err == nil || !strings.Contains(err.Error(), "cannot materialize") {
		t.Fatalf("SealBaseTypeEnv(adversarial) error = %v, want pre-persistence rejection", err)
	}

	loaded, err := LoadEnvelopeDB(context.Background(), database)
	if err != nil {
		t.Fatalf("LoadEnvelopeDB(after rejection) error = %v", err)
	}
	if loaded.Artifact().Digest() != fixture.artifact.Digest() {
		t.Fatalf(
			"rejected artifact replaced stored digest with %s; want %s",
			loaded.Artifact().Digest().String(),
			fixture.artifact.Digest().String(),
		)
	}
}

type storageFixture struct {
	unitID   string
	revision typedmemory.SourceRevision
	compiler typedmemory.CompilerSchemaVersion
	location typedmemory.SourceLocation
	symbol   typedmemory.SchemaSymbolRef
	artifact typeenv.BaseTypeEnvArtifact
}

func newStorageFixture(t *testing.T) storageFixture {
	t.Helper()
	revision, err := typedmemory.NewSourceRevision("44dd88188a07646ef23aca32627a3f670525853f")
	if err != nil {
		t.Fatalf("NewSourceRevision() error = %v", err)
	}
	compiler, err := typedmemory.NewCompilerSchemaVersion("fpf-typeenv.v1")
	if err != nil {
		t.Fatalf("NewCompilerSchemaVersion() error = %v", err)
	}
	unitIDText := "spec:pattern_section:a-6-5:1161"
	unitID, err := typedmemory.NewSourceUnitID(unitIDText)
	if err != nil {
		t.Fatalf("NewSourceUnitID() error = %v", err)
	}
	digest, err := typedmemory.NewSHA256Digest("sha256:" + strings.Repeat("b", 64))
	if err != nil {
		t.Fatalf("NewSHA256Digest() error = %v", err)
	}
	lineRange, err := typedmemory.NewSourceLineRange(10, 20)
	if err != nil {
		t.Fatalf("NewSourceLineRange() error = %v", err)
	}
	patternID, err := typedmemory.NewPatternID("A.6.5")
	if err != nil {
		t.Fatalf("NewPatternID() error = %v", err)
	}
	location, err := typedmemory.NewPatternedSourceLocation(
		unitID,
		revision,
		digest,
		lineRange,
		patternID,
	)
	if err != nil {
		t.Fatalf("NewPatternedSourceLocation() error = %v", err)
	}
	kindID, err := typedmemory.NewKindID("U.Kind")
	if err != nil {
		t.Fatalf("NewKindID() error = %v", err)
	}
	symbol, err := typedmemory.KindSymbolRef(kindID)
	if err != nil {
		t.Fatalf("KindSymbolRef() error = %v", err)
	}
	contextRef, err := typedmemory.NewBoundedContextRef("fpf:publication")
	if err != nil {
		t.Fatalf("NewBoundedContextRef() error = %v", err)
	}
	contextSymbol, err := typedmemory.BoundedContextSymbolRef(contextRef)
	if err != nil {
		t.Fatalf("BoundedContextSymbolRef() error = %v", err)
	}
	contextDeclaration := newFixtureContextDeclaration(
		t,
		contextSymbol,
		contextRef,
		revision,
		location,
	)
	kindDeclaration := newFixtureKindDeclaration(t, symbol, "U.Kind", location)
	declarations := []typeenv.LinkedDeclaration{contextDeclaration, kindDeclaration}
	coverageEntries := make([]typedmemory.CoverageEntry, 0, len(declarations))
	for _, declaration := range declarations {
		coverageSubject, coverageErr := typedmemory.SchemaSymbolCoverage(declaration.Symbol())
		if coverageErr != nil {
			t.Fatalf("SchemaSymbolCoverage() error = %v", coverageErr)
		}
		coverageEntry, coverageErr := typedmemory.NewCompiledCoverageEntry(coverageSubject, location)
		if coverageErr != nil {
			t.Fatalf("NewCompiledCoverageEntry() error = %v", coverageErr)
		}
		coverageEntries = append(coverageEntries, coverageEntry)
	}
	coverage, err := typedmemory.NewCoverageManifest(coverageEntries)
	if err != nil {
		t.Fatalf("NewCoverageManifest() error = %v", err)
	}
	ir, err := typeenv.NewCompiledLinkedTypeEnvIR(
		revision,
		compiler,
		coverage,
		declarations,
	)
	if err != nil {
		t.Fatalf("NewCompiledLinkedTypeEnvIR() error = %v", err)
	}
	artifact, err := typeenv.SealBaseTypeEnv(ir)
	if err != nil {
		t.Fatalf("SealBaseTypeEnv() error = %v", err)
	}
	return storageFixture{
		unitID:   unitIDText,
		revision: revision,
		compiler: compiler,
		location: location,
		symbol:   symbol,
		artifact: artifact,
	}
}

func newFixtureContextDeclaration(
	t *testing.T,
	symbol typedmemory.SchemaSymbolRef,
	contextRef typedmemory.BoundedContextRef,
	revision typedmemory.SourceRevision,
	location typedmemory.SourceLocation,
) typeenv.LinkedDeclaration {
	t.Helper()
	contextField, err := typeenv.NewDeclarationField(
		"context_ref",
		typeenv.NewTextValue(contextRef.String()),
	)
	if err != nil {
		t.Fatalf("NewDeclarationField(context_ref) error = %v", err)
	}
	revisionField, err := typeenv.NewDeclarationField(
		"source_revision",
		typeenv.NewTextValue(revision.String()),
	)
	if err != nil {
		t.Fatalf("NewDeclarationField(source_revision) error = %v", err)
	}
	return newFixtureDeclaration(
		t,
		symbol,
		[]typeenv.DeclarationField{contextField, revisionField},
		"fpf.publication-context.v1",
		location,
	)
}

func newFixtureKindDeclaration(
	t *testing.T,
	symbol typedmemory.SchemaSymbolRef,
	kindName string,
	location typedmemory.SourceLocation,
) typeenv.LinkedDeclaration {
	t.Helper()
	kindField, err := typeenv.NewDeclarationField(
		"kind_id",
		typeenv.NewTextValue(kindName),
	)
	if err != nil {
		t.Fatalf("NewDeclarationField(kind_id) error = %v", err)
	}
	roleField, err := typeenv.NewDeclarationField(
		"semantic_role",
		typeenv.NewTextValue("value_kind"),
	)
	if err != nil {
		t.Fatalf("NewDeclarationField(semantic_role) error = %v", err)
	}
	return newFixtureDeclaration(
		t,
		symbol,
		[]typeenv.DeclarationField{kindField, roleField},
		"fpf.value-kind.declaration.v1",
		location,
	)
}

func newFixtureDeclaration(
	t *testing.T,
	symbol typedmemory.SchemaSymbolRef,
	fields []typeenv.DeclarationField,
	rule string,
	location typedmemory.SourceLocation,
) typeenv.LinkedDeclaration {
	t.Helper()
	ruleID, err := typedmemory.NewCompilerRuleID(rule)
	if err != nil {
		t.Fatalf("NewCompilerRuleID(%s) error = %v", rule, err)
	}
	body, err := typeenv.NewDeclarationBody(fields)
	if err != nil {
		t.Fatalf("NewDeclarationBody() error = %v", err)
	}
	provenanceRef, err := typedmemory.NewProvenanceRef("fixture:" + symbol.String())
	if err != nil {
		t.Fatalf("NewProvenanceRef() error = %v", err)
	}
	provenance, err := typedmemory.NewFPFSourceProvenance(
		provenanceRef,
		location,
		ruleID,
	)
	if err != nil {
		t.Fatalf("NewFPFSourceProvenance() error = %v", err)
	}
	basis, err := typeenv.NewSourceDeclarationBasis(provenance)
	if err != nil {
		t.Fatalf("NewSourceDeclarationBasis() error = %v", err)
	}
	declaration, err := typeenv.NewLinkedDeclaration(symbol, ruleID, body, basis)
	if err != nil {
		t.Fatalf("NewLinkedDeclaration() error = %v", err)
	}
	return declaration
}

func sealUnmaterializableCompiledArtifact(
	t *testing.T,
	fixture storageFixture,
) error {
	t.Helper()
	ruleID, err := typedmemory.NewCompilerRuleID("fpf.unsupported-shape.v1")
	if err != nil {
		t.Fatalf("NewCompilerRuleID() error = %v", err)
	}
	contextRef, err := typedmemory.NewBoundedContextRef("fpf:publication")
	if err != nil {
		t.Fatalf("NewBoundedContextRef() error = %v", err)
	}
	contextSymbol, err := typedmemory.BoundedContextSymbolRef(contextRef)
	if err != nil {
		t.Fatalf("BoundedContextSymbolRef() error = %v", err)
	}
	shapeID, err := typedmemory.NewShapeID("unsupported-shape")
	if err != nil {
		t.Fatalf("NewShapeID() error = %v", err)
	}
	shapeSymbol, err := typedmemory.ValueShapeSymbolRef(shapeID)
	if err != nil {
		t.Fatalf("ValueShapeSymbolRef() error = %v", err)
	}
	declarations := []typeenv.LinkedDeclaration{
		newFixtureContextDeclaration(
			t,
			contextSymbol,
			contextRef,
			fixture.revision,
			fixture.location,
		),
		newFixtureDeclaration(
			t,
			shapeSymbol,
			[]typeenv.DeclarationField{
				mustFixtureField(t, "name", typeenv.NewTextValue("unsupported-shape")),
			},
			ruleID.String(),
			fixture.location,
		),
	}
	coverageEntries := make([]typedmemory.CoverageEntry, 0, len(declarations))
	for _, declaration := range declarations {
		subject, subjectErr := typedmemory.SchemaSymbolCoverage(declaration.Symbol())
		if subjectErr != nil {
			t.Fatalf("SchemaSymbolCoverage() error = %v", subjectErr)
		}
		entry, entryErr := typedmemory.NewCompiledCoverageEntry(subject, fixture.location)
		if entryErr != nil {
			t.Fatalf("NewCompiledCoverageEntry() error = %v", entryErr)
		}
		coverageEntries = append(coverageEntries, entry)
	}
	coverage, err := typedmemory.NewCoverageManifest(coverageEntries)
	if err != nil {
		t.Fatalf("NewCoverageManifest() error = %v", err)
	}
	ir, err := typeenv.NewCompiledLinkedTypeEnvIR(
		fixture.revision,
		fixture.compiler,
		coverage,
		declarations,
	)
	if err != nil {
		t.Fatalf("NewCompiledLinkedTypeEnvIR() error = %v", err)
	}
	_, err = typeenv.SealBaseTypeEnv(ir)
	return err
}

func mustFixtureField(
	t *testing.T,
	name string,
	value typeenv.DeclarationValue,
) typeenv.DeclarationField {
	t.Helper()
	field, err := typeenv.NewDeclarationField(name, value)
	if err != nil {
		t.Fatalf("NewDeclarationField(%s) error = %v", name, err)
	}
	return field
}

func comparedEnvelope(
	t *testing.T,
	artifact typeenv.BaseTypeEnvArtifact,
	symbol typedmemory.SchemaSymbolRef,
) typeenv.CompilationEnvelope {
	t.Helper()
	baseDigest, err := typedmemory.NewSHA256Digest("sha256:" + strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("NewSHA256Digest(base) error = %v", err)
	}
	base, err := typedmemory.NewTypeEnvRef(baseDigest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef(base) error = %v", err)
	}
	change, err := typedmemory.NewCompatibilityChange(
		symbol,
		typedmemory.CompatibilityAdded,
		"new-source-derived-kind",
	)
	if err != nil {
		t.Fatalf("NewCompatibilityChange() error = %v", err)
	}
	diff, err := typedmemory.NewTypeEnvCompatibilityDiff(
		base,
		[]typedmemory.CompatibilityChange{change},
	)
	if err != nil {
		t.Fatalf("NewTypeEnvCompatibilityDiff() error = %v", err)
	}
	assessment, err := typeenv.NewComparedCompatibilityAssessment(diff)
	if err != nil {
		t.Fatalf("NewComparedCompatibilityAssessment() error = %v", err)
	}
	envelope, err := typeenv.NewCompilationEnvelope(artifact, assessment)
	if err != nil {
		t.Fatalf("NewCompilationEnvelope(compared) error = %v", err)
	}
	return envelope
}

func openFixtureDatabase(t *testing.T) *sql.DB {
	t.Helper()
	dsn := "file:" + t.TempDir() + "/typeenv.db?_pragma=foreign_keys(1)"
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if _, err := database.Exec(sourceUnitFixtureSchema); err != nil {
		t.Fatalf("create source_units fixture: %v", err)
	}
	return database
}

func insertFixtureSourceUnit(t *testing.T, database *sql.DB, fixture storageFixture) {
	t.Helper()
	statement := `INSERT INTO source_units (
		unit_id, content_hash, source_revision, start_line, end_line, pattern_id
	) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := database.Exec(
		statement,
		fixture.unitID,
		strings.TrimPrefix(fixture.location.ContentHash().String(), "sha256:"),
		fixture.revision.String(),
		fixture.location.LineRange().Start(),
		fixture.location.LineRange().End(),
		"A.6.5",
	)
	if err != nil {
		t.Fatalf("insert source_units fixture: %v", err)
	}
}

func countTypeEnvTables(t *testing.T, database *sql.DB) int {
	t.Helper()
	statement := `SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name LIKE 'fpf_typeenv_%'`
	var count int
	if err := database.QueryRow(statement).Scan(&count); err != nil {
		t.Fatalf("count TypeEnv tables: %v", err)
	}
	return count
}
