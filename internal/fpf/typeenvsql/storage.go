// Package typeenvsql persists a source-derived FPF TypeEnv artifact and its
// disposable SQL projections. The canonical artifact payload is the sole
// authority; declarations, source inputs, coverage, and compatibility rows
// exist only for indexed inspection and integrity checks.
package typeenvsql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	artifactSingleton = 1
	projectionDomain  = "haft.fpf.typeenvsql.projection.v1"
)

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS fpf_typeenv_artifact (
		singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
		canonical_bytes BLOB NOT NULL,
		artifact_digest TEXT NOT NULL UNIQUE,
		posture TEXT NOT NULL CHECK (posture IN ('compiled_environment', 'coverage_only')),
		typeenv_ref TEXT,
		source_revision TEXT NOT NULL,
		compiler_schema_version TEXT NOT NULL,
		coverage_only_reason TEXT,
		declaration_count INTEGER NOT NULL CHECK (declaration_count >= 0),
		declaration_projection_digest TEXT NOT NULL,
		source_count INTEGER NOT NULL CHECK (source_count >= 0),
		source_projection_digest TEXT NOT NULL,
		coverage_count INTEGER NOT NULL CHECK (coverage_count > 0),
		coverage_projection_digest TEXT NOT NULL,
		CHECK (
			(posture = 'compiled_environment' AND typeenv_ref IS NOT NULL AND coverage_only_reason IS NULL)
			OR
			(posture = 'coverage_only' AND typeenv_ref IS NULL AND coverage_only_reason IS NOT NULL)
		)
	)`,
	`CREATE TABLE IF NOT EXISTS fpf_typeenv_declarations (
		symbol TEXT PRIMARY KEY,
		symbol_kind TEXT NOT NULL,
		symbol_key TEXT NOT NULL,
		declaration_digest TEXT NOT NULL,
		compiler_rule_id TEXT NOT NULL,
		basis_kind TEXT NOT NULL CHECK (basis_kind IN ('source_authored', 'compiler_derived')),
		dependency_count INTEGER NOT NULL CHECK (dependency_count >= 0),
		dependency_digest TEXT NOT NULL,
		artifact_digest TEXT NOT NULL,
		FOREIGN KEY (artifact_digest) REFERENCES fpf_typeenv_artifact(artifact_digest) ON DELETE CASCADE
	)`,
	`CREATE TABLE IF NOT EXISTS fpf_typeenv_declaration_dependencies (
		symbol TEXT NOT NULL,
		dependency_ordinal INTEGER NOT NULL CHECK (dependency_ordinal >= 0),
		dependency_symbol TEXT NOT NULL,
		PRIMARY KEY (symbol, dependency_ordinal),
		UNIQUE (symbol, dependency_symbol),
		FOREIGN KEY (symbol) REFERENCES fpf_typeenv_declarations(symbol) ON DELETE CASCADE
	)`,
	`CREATE TABLE IF NOT EXISTS fpf_typeenv_coverage (
		subject TEXT PRIMARY KEY,
		subject_kind TEXT NOT NULL CHECK (subject_kind IN ('source_unit', 'schema_symbol')),
		source_unit_id TEXT,
		schema_symbol TEXT,
		posture TEXT NOT NULL CHECK (posture IN ('compiled', 'source_only', 'unsupported')),
		rationale TEXT NOT NULL,
		artifact_digest TEXT NOT NULL,
		CHECK (
			(subject_kind = 'source_unit' AND source_unit_id IS NOT NULL AND schema_symbol IS NULL)
			OR
			(subject_kind = 'schema_symbol' AND source_unit_id IS NULL AND schema_symbol IS NOT NULL)
		),
		FOREIGN KEY (artifact_digest) REFERENCES fpf_typeenv_artifact(artifact_digest) ON DELETE CASCADE
	)`,
	`CREATE TABLE IF NOT EXISTS fpf_typeenv_sources (
		owner_kind TEXT NOT NULL CHECK (owner_kind IN ('declaration', 'coverage')),
		owner_key TEXT NOT NULL,
		source_ordinal INTEGER NOT NULL CHECK (source_ordinal >= 0),
		unit_id TEXT NOT NULL,
		source_revision TEXT NOT NULL,
		content_hash TEXT NOT NULL,
		start_line INTEGER NOT NULL CHECK (start_line > 0),
		end_line INTEGER NOT NULL CHECK (end_line >= start_line),
		pattern_id TEXT,
		artifact_digest TEXT NOT NULL,
		PRIMARY KEY (owner_kind, owner_key, source_ordinal),
		UNIQUE (owner_kind, owner_key, unit_id, start_line, end_line),
		FOREIGN KEY (unit_id) REFERENCES source_units(unit_id),
		FOREIGN KEY (artifact_digest) REFERENCES fpf_typeenv_artifact(artifact_digest) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS fpf_typeenv_sources_unit
		ON fpf_typeenv_sources(unit_id, owner_kind, owner_key)`,
	`CREATE TABLE IF NOT EXISTS fpf_typeenv_compatibility (
		singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
		artifact_digest TEXT NOT NULL,
		assessment_kind TEXT NOT NULL CHECK (assessment_kind IN ('initial', 'compared')),
		base_typeenv_ref TEXT,
		change_count INTEGER NOT NULL CHECK (change_count >= 0),
		changes_digest TEXT NOT NULL,
		CHECK (
			(assessment_kind = 'initial' AND base_typeenv_ref IS NULL AND change_count = 0)
			OR
			(assessment_kind = 'compared' AND base_typeenv_ref IS NOT NULL)
		),
		FOREIGN KEY (artifact_digest) REFERENCES fpf_typeenv_artifact(artifact_digest) ON DELETE CASCADE
	)`,
	`CREATE TABLE IF NOT EXISTS fpf_typeenv_compatibility_changes (
		change_ordinal INTEGER PRIMARY KEY CHECK (change_ordinal >= 0),
		symbol TEXT NOT NULL UNIQUE,
		change_kind TEXT NOT NULL CHECK (change_kind IN ('added', 'changed', 'removed')),
		rationale TEXT NOT NULL,
		artifact_digest TEXT NOT NULL,
		FOREIGN KEY (artifact_digest) REFERENCES fpf_typeenv_artifact(artifact_digest) ON DELETE CASCADE
	)`,
}

// ProjectionSummary is a compact integrity witness stored beside the
// authoritative canonical payload.
type ProjectionSummary struct {
	DeclarationCount  int
	DeclarationDigest string
	SourceCount       int
	SourceDigest      string
	CoverageCount     int
	CoverageDigest    string
}

// SchemaStatements returns an owned copy so the schema-11 composition root can
// include this package without duplicating its publication grammar.
func SchemaStatements() []string {
	return append([]string(nil), schemaStatements...)
}

func EnsureSchemaDB(ctx context.Context, database *sql.DB) error {
	if database == nil {
		return fmt.Errorf("TypeEnv database is required")
	}
	return ensureSchema(ctx, database)
}

func EnsureSchemaTx(ctx context.Context, transaction *sql.Tx) error {
	if transaction == nil {
		return fmt.Errorf("TypeEnv transaction is required")
	}
	return ensureSchema(ctx, transaction)
}

func ReplaceEnvelopeDB(
	ctx context.Context,
	database *sql.DB,
	envelope typeenv.CompilationEnvelope,
) error {
	if database == nil {
		return fmt.Errorf("TypeEnv database is required")
	}
	if err := EnsureSchemaDB(ctx, database); err != nil {
		return err
	}
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin TypeEnv transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()
	if err := ReplaceEnvelopeTx(ctx, transaction, envelope); err != nil {
		return err
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit TypeEnv transaction: %w", err)
	}
	return nil
}

func ReplaceEnvelopeTx(
	ctx context.Context,
	transaction *sql.Tx,
	envelope typeenv.CompilationEnvelope,
) error {
	if transaction == nil {
		return fmt.Errorf("TypeEnv transaction is required")
	}
	if err := EnsureSchemaTx(ctx, transaction); err != nil {
		return err
	}
	projection, err := projectEnvelope(envelope)
	if err != nil {
		return err
	}
	if err := clearEnvelope(ctx, transaction); err != nil {
		return err
	}
	if err := insertArtifact(ctx, transaction, projection); err != nil {
		return err
	}
	if err := insertDeclarations(ctx, transaction, projection); err != nil {
		return err
	}
	if err := insertCoverage(ctx, transaction, projection); err != nil {
		return err
	}
	if err := insertSources(ctx, transaction, projection); err != nil {
		return err
	}
	if err := insertCompatibility(ctx, transaction, projection); err != nil {
		return err
	}
	return VerifyEnvelopeTx(ctx, transaction)
}

func LoadEnvelopeDB(
	ctx context.Context,
	database *sql.DB,
) (typeenv.CompilationEnvelope, error) {
	if database == nil {
		return typeenv.CompilationEnvelope{}, fmt.Errorf("TypeEnv database is required")
	}
	if err := EnsureSchemaDB(ctx, database); err != nil {
		return typeenv.CompilationEnvelope{}, err
	}
	return loadAndVerifyEnvelope(ctx, database)
}

func LoadEnvelopeTx(
	ctx context.Context,
	transaction *sql.Tx,
) (typeenv.CompilationEnvelope, error) {
	if transaction == nil {
		return typeenv.CompilationEnvelope{}, fmt.Errorf("TypeEnv transaction is required")
	}
	return loadAndVerifyEnvelope(ctx, transaction)
}

func LoadArtifactDB(
	ctx context.Context,
	database *sql.DB,
) (typeenv.BaseTypeEnvArtifact, error) {
	envelope, err := LoadEnvelopeDB(ctx, database)
	if err != nil {
		return typeenv.BaseTypeEnvArtifact{}, err
	}
	return envelope.Artifact(), nil
}

// LoadArtifactReadOnlyDB probes an already-built index without creating or
// migrating any table. This is the safe prior-target read used before an
// atomic rebuild: an older index with no TypeEnv tables reports a missing
// artifact and remains byte-for-byte untouched.
func LoadArtifactReadOnlyDB(
	ctx context.Context,
	database *sql.DB,
) (typeenv.BaseTypeEnvArtifact, error) {
	envelope, err := LoadEnvelopeReadOnlyDB(ctx, database)
	if err != nil {
		return typeenv.BaseTypeEnvArtifact{}, err
	}
	return envelope.Artifact(), nil
}

// LoadEnvelopeReadOnlyDB probes an already-built index without creating or
// migrating any table. Unlike LoadEnvelopeDB, it is safe at the prior-target
// boundary of an atomic rebuild and retains the run-relative compatibility
// assessment needed to make an unchanged rebuild idempotent.
func LoadEnvelopeReadOnlyDB(
	ctx context.Context,
	database *sql.DB,
) (typeenv.CompilationEnvelope, error) {
	if database == nil {
		return typeenv.CompilationEnvelope{}, fmt.Errorf("TypeEnv database is required")
	}
	exists, err := artifactTableExists(ctx, database)
	if err != nil {
		return typeenv.CompilationEnvelope{}, err
	}
	if !exists {
		return typeenv.CompilationEnvelope{}, fmt.Errorf(
			"source-derived TypeEnv artifact is not present: %w",
			sql.ErrNoRows,
		)
	}
	return loadAndVerifyEnvelope(ctx, database)
}

func LoadArtifactTx(
	ctx context.Context,
	transaction *sql.Tx,
) (typeenv.BaseTypeEnvArtifact, error) {
	envelope, err := LoadEnvelopeTx(ctx, transaction)
	if err != nil {
		return typeenv.BaseTypeEnvArtifact{}, err
	}
	return envelope.Artifact(), nil
}

func VerifyEnvelopeDB(ctx context.Context, database *sql.DB) error {
	if database == nil {
		return fmt.Errorf("TypeEnv database is required")
	}
	if err := EnsureSchemaDB(ctx, database); err != nil {
		return err
	}
	_, err := loadAndVerifyEnvelope(ctx, database)
	return err
}

func VerifyEnvelopeTx(ctx context.Context, transaction *sql.Tx) error {
	if transaction == nil {
		return fmt.Errorf("TypeEnv transaction is required")
	}
	_, err := loadAndVerifyEnvelope(ctx, transaction)
	return err
}

func artifactTableExists(ctx context.Context, queryer sqlQueryer) (bool, error) {
	statement := `SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name = 'fpf_typeenv_artifact'`
	var count int
	if err := queryer.QueryRowContext(ctx, statement).Scan(&count); err != nil {
		return false, fmt.Errorf("probe TypeEnv artifact table: %w", err)
	}
	return count == 1, nil
}

type sqlExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type sqlQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func ensureSchema(ctx context.Context, executor sqlExecutor) error {
	for _, statement := range schemaStatements {
		if _, err := executor.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("create TypeEnv schema: %w", err)
		}
	}
	return nil
}

type artifactProjection struct {
	artifact      typeenv.BaseTypeEnvArtifact
	canonical     []byte
	posture       string
	typeEnvRef    sql.NullString
	reason        sql.NullString
	declarations  []declarationRow
	sources       []sourceRow
	coverage      []coverageRow
	compatibility compatibilityProjection
	summary       ProjectionSummary
}

type declarationRow struct {
	Symbol            string   `json:"symbol"`
	SymbolKind        string   `json:"symbol_kind"`
	SymbolKey         string   `json:"symbol_key"`
	DeclarationDigest string   `json:"declaration_digest"`
	CompilerRuleID    string   `json:"compiler_rule_id"`
	BasisKind         string   `json:"basis_kind"`
	Dependencies      []string `json:"dependencies"`
}

type sourceRow struct {
	OwnerKind      string `json:"owner_kind"`
	OwnerKey       string `json:"owner_key"`
	Ordinal        int    `json:"ordinal"`
	UnitID         string `json:"unit_id"`
	Revision       string `json:"revision"`
	ContentHash    string `json:"content_hash"`
	StartLine      uint64 `json:"start_line"`
	EndLine        uint64 `json:"end_line"`
	PatternID      string `json:"pattern_id"`
	ArtifactDigest string `json:"-"`
}

type coverageRow struct {
	Subject      string `json:"subject"`
	SubjectKind  string `json:"subject_kind"`
	SourceUnitID string `json:"source_unit_id"`
	SchemaSymbol string `json:"schema_symbol"`
	Posture      string `json:"posture"`
	Rationale    string `json:"rationale"`
}

type compatibilityProjection struct {
	Kind       string
	BaseRef    sql.NullString
	Changes    []compatibilityChangeRow
	Digest     string
	ArtifactID string
}

type compatibilityChangeRow struct {
	Ordinal   int    `json:"ordinal"`
	Symbol    string `json:"symbol"`
	Kind      string `json:"kind"`
	Rationale string `json:"rationale"`
}

func projectEnvelope(envelope typeenv.CompilationEnvelope) (artifactProjection, error) {
	artifact := envelope.Artifact()
	if err := artifact.Verify(); err != nil {
		return artifactProjection{}, fmt.Errorf("verify authoritative TypeEnv artifact: %w", err)
	}
	if err := verifyExecutableArtifact(artifact); err != nil {
		return artifactProjection{}, err
	}
	projection := artifactProjection{
		artifact:  artifact,
		canonical: artifact.CanonicalBytes(),
		posture:   artifact.Posture().String(),
	}
	if ref, exists := artifact.TypeEnvRef(); exists {
		projection.typeEnvRef = sql.NullString{String: ref.String(), Valid: true}
	}
	if reason, exists := artifact.CoverageOnlyReason(); exists {
		projection.reason = sql.NullString{String: reason, Valid: true}
	}
	projection.declarations = projectDeclarations(artifact)
	projection.coverage = projectCoverage(artifact)
	projection.sources = projectSources(artifact)
	compatibility, err := projectCompatibility(envelope)
	if err != nil {
		return artifactProjection{}, err
	}
	projection.compatibility = compatibility
	summary, err := summarizeProjection(projection)
	if err != nil {
		return artifactProjection{}, err
	}
	projection.summary = summary
	return projection, nil
}

func verifyExecutableArtifact(artifact typeenv.BaseTypeEnvArtifact) error {
	if artifact.Posture() == typeenv.CoverageOnly {
		return nil
	}
	artifactRef, exists := artifact.TypeEnvRef()
	if !exists {
		return fmt.Errorf("compiled TypeEnv artifact has no identity")
	}
	environment, err := typeenv.LowerBaseTypeEnvArtifact(artifact)
	if err != nil {
		return fmt.Errorf("compiled TypeEnv artifact is not runtime-materializable: %w", err)
	}
	if environment.Ref().String() != artifactRef.String() {
		return fmt.Errorf("lowered TypeEnv identity differs from stored artifact identity")
	}
	return nil
}

func projectDeclarations(artifact typeenv.BaseTypeEnvArtifact) []declarationRow {
	projections := artifact.DeclarationProjections()
	rows := make([]declarationRow, 0, len(projections))
	for _, projection := range projections {
		dependencies := projection.Dependencies()
		encodedDependencies := make([]string, 0, len(dependencies))
		for _, dependency := range dependencies {
			encodedDependencies = append(encodedDependencies, dependency.String())
		}
		basisKind := "source_authored"
		if projection.BasisKind() == typeenv.CompilerDerivedBasis {
			basisKind = "compiler_derived"
		}
		row := declarationRow{
			Symbol:            projection.Symbol().String(),
			SymbolKind:        projection.Symbol().Kind().String(),
			SymbolKey:         projection.Symbol().Key(),
			DeclarationDigest: projection.Digest().String(),
			CompilerRuleID:    projection.RuleID().String(),
			BasisKind:         basisKind,
			Dependencies:      encodedDependencies,
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(left, right int) bool {
		return rows[left].Symbol < rows[right].Symbol
	})
	return rows
}

func projectCoverage(artifact typeenv.BaseTypeEnvArtifact) []coverageRow {
	entries := artifact.CoverageManifest().Entries()
	rows := make([]coverageRow, 0, len(entries))
	for _, entry := range entries {
		row := coverageRow{
			Subject:   entry.Subject().String(),
			Posture:   entry.Posture().String(),
			Rationale: entry.Rationale(),
		}
		if unitID, exists := entry.Subject().SourceUnitID(); exists {
			row.SubjectKind = "source_unit"
			row.SourceUnitID = unitID.String()
		}
		if symbol, exists := entry.Subject().SchemaSymbol(); exists {
			row.SubjectKind = "schema_symbol"
			row.SchemaSymbol = symbol.String()
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(left, right int) bool {
		return rows[left].Subject < rows[right].Subject
	})
	return rows
}

func projectSources(artifact typeenv.BaseTypeEnvArtifact) []sourceRow {
	digest := artifact.Digest().String()
	rows := make([]sourceRow, 0)
	for _, projection := range artifact.DeclarationProjections() {
		locations := projection.SourceInputs()
		for ordinal, location := range locations {
			row := sourceProjection("declaration", projection.Symbol().String(), ordinal, location)
			row.ArtifactDigest = digest
			rows = append(rows, row)
		}
	}
	for _, entry := range artifact.CoverageManifest().Entries() {
		row := sourceProjection("coverage", entry.Subject().String(), 0, entry.Source())
		row.ArtifactDigest = digest
		rows = append(rows, row)
	}
	sort.Slice(rows, func(left, right int) bool {
		leftKey := sourceProjectionKey(rows[left])
		rightKey := sourceProjectionKey(rows[right])
		return leftKey < rightKey
	})
	return rows
}

func sourceProjection(
	ownerKind string,
	ownerKey string,
	ordinal int,
	location typedmemory.SourceLocation,
) sourceRow {
	row := sourceRow{
		OwnerKind:   ownerKind,
		OwnerKey:    ownerKey,
		Ordinal:     ordinal,
		UnitID:      location.UnitID().String(),
		Revision:    location.Revision().String(),
		ContentHash: location.ContentHash().String(),
		StartLine:   location.LineRange().Start(),
		EndLine:     location.LineRange().End(),
	}
	if patternID, exists := location.PatternID(); exists {
		row.PatternID = patternID.String()
	}
	return row
}

func sourceProjectionKey(row sourceRow) string {
	return fmt.Sprintf("%s\x00%s\x00%020d", row.OwnerKind, row.OwnerKey, row.Ordinal)
}

func projectCompatibility(
	envelope typeenv.CompilationEnvelope,
) (compatibilityProjection, error) {
	artifactID := envelope.Artifact().Digest().String()
	switch assessment := envelope.Compatibility().(type) {
	case typeenv.InitialCompatibilityAssessment:
		digest, err := projectionDigest("compatibility", []compatibilityChangeRow{})
		if err != nil {
			return compatibilityProjection{}, err
		}
		return compatibilityProjection{
			Kind:       "initial",
			Changes:    []compatibilityChangeRow{},
			Digest:     digest,
			ArtifactID: artifactID,
		}, nil
	case typeenv.ComparedCompatibilityAssessment:
		changes := assessment.Diff().Changes()
		rows := make([]compatibilityChangeRow, 0, len(changes))
		for ordinal, change := range changes {
			row := compatibilityChangeRow{
				Ordinal:   ordinal,
				Symbol:    change.Symbol().String(),
				Kind:      change.Kind().String(),
				Rationale: change.Rationale(),
			}
			rows = append(rows, row)
		}
		digest, err := projectionDigest("compatibility", rows)
		if err != nil {
			return compatibilityProjection{}, err
		}
		return compatibilityProjection{
			Kind:       "compared",
			BaseRef:    sql.NullString{String: assessment.Diff().Base().String(), Valid: true},
			Changes:    rows,
			Digest:     digest,
			ArtifactID: artifactID,
		}, nil
	default:
		return compatibilityProjection{}, fmt.Errorf("unsupported compatibility assessment")
	}
}

func summarizeProjection(projection artifactProjection) (ProjectionSummary, error) {
	declarationDigest, err := projectionDigest("declarations", projection.declarations)
	if err != nil {
		return ProjectionSummary{}, err
	}
	sourceDigest, err := projectionDigest("sources", projection.sources)
	if err != nil {
		return ProjectionSummary{}, err
	}
	coverageDigest, err := projectionDigest("coverage", projection.coverage)
	if err != nil {
		return ProjectionSummary{}, err
	}
	return ProjectionSummary{
		DeclarationCount:  len(projection.declarations),
		DeclarationDigest: declarationDigest,
		SourceCount:       len(projection.sources),
		SourceDigest:      sourceDigest,
		CoverageCount:     len(projection.coverage),
		CoverageDigest:    coverageDigest,
	}, nil
}

func projectionDigest(role string, value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode %s projection: %w", role, err)
	}
	payload := projectionDomain + "\x00" + role + "\x00" + string(encoded)
	digest := sha256.Sum256([]byte(payload))
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func clearEnvelope(ctx context.Context, executor sqlExecutor) error {
	statements := []string{
		`DELETE FROM fpf_typeenv_compatibility_changes`,
		`DELETE FROM fpf_typeenv_compatibility`,
		`DELETE FROM fpf_typeenv_sources`,
		`DELETE FROM fpf_typeenv_declaration_dependencies`,
		`DELETE FROM fpf_typeenv_coverage`,
		`DELETE FROM fpf_typeenv_declarations`,
		`DELETE FROM fpf_typeenv_artifact`,
	}
	for _, statement := range statements {
		if _, err := executor.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("clear TypeEnv projection: %w", err)
		}
	}
	return nil
}

func insertArtifact(
	ctx context.Context,
	executor sqlExecutor,
	projection artifactProjection,
) error {
	statement := `INSERT INTO fpf_typeenv_artifact (
		singleton, canonical_bytes, artifact_digest, posture, typeenv_ref,
		source_revision, compiler_schema_version, coverage_only_reason,
		declaration_count, declaration_projection_digest,
		source_count, source_projection_digest,
		coverage_count, coverage_projection_digest
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := executor.ExecContext(
		ctx,
		statement,
		artifactSingleton,
		projection.canonical,
		projection.artifact.Digest().String(),
		projection.posture,
		projection.typeEnvRef,
		projection.artifact.SourceRevision().String(),
		projection.artifact.CompilerSchemaVersion().String(),
		projection.reason,
		projection.summary.DeclarationCount,
		projection.summary.DeclarationDigest,
		projection.summary.SourceCount,
		projection.summary.SourceDigest,
		projection.summary.CoverageCount,
		projection.summary.CoverageDigest,
	)
	if err != nil {
		return fmt.Errorf("insert authoritative TypeEnv artifact: %w", err)
	}
	return nil
}

func insertDeclarations(
	ctx context.Context,
	executor sqlExecutor,
	projection artifactProjection,
) error {
	declarationStatement := `INSERT INTO fpf_typeenv_declarations (
		symbol, symbol_kind, symbol_key, declaration_digest, compiler_rule_id,
		basis_kind, dependency_count, dependency_digest, artifact_digest
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	dependencyStatement := `INSERT INTO fpf_typeenv_declaration_dependencies (
		symbol, dependency_ordinal, dependency_symbol
	) VALUES (?, ?, ?)`
	artifactID := projection.artifact.Digest().String()
	for _, row := range projection.declarations {
		dependencyDigest, err := projectionDigest("dependencies:"+row.Symbol, row.Dependencies)
		if err != nil {
			return err
		}
		_, err = executor.ExecContext(
			ctx,
			declarationStatement,
			row.Symbol,
			row.SymbolKind,
			row.SymbolKey,
			row.DeclarationDigest,
			row.CompilerRuleID,
			row.BasisKind,
			len(row.Dependencies),
			dependencyDigest,
			artifactID,
		)
		if err != nil {
			return fmt.Errorf("insert TypeEnv declaration %s: %w", row.Symbol, err)
		}
		for ordinal, dependency := range row.Dependencies {
			_, err = executor.ExecContext(ctx, dependencyStatement, row.Symbol, ordinal, dependency)
			if err != nil {
				return fmt.Errorf("insert TypeEnv dependency %s -> %s: %w", row.Symbol, dependency, err)
			}
		}
	}
	return nil
}

func insertCoverage(
	ctx context.Context,
	executor sqlExecutor,
	projection artifactProjection,
) error {
	statement := `INSERT INTO fpf_typeenv_coverage (
		subject, subject_kind, source_unit_id, schema_symbol, posture,
		rationale, artifact_digest
	) VALUES (?, ?, ?, ?, ?, ?, ?)`
	artifactID := projection.artifact.Digest().String()
	for _, row := range projection.coverage {
		sourceUnit := nullableString(row.SourceUnitID)
		schemaSymbol := nullableString(row.SchemaSymbol)
		_, err := executor.ExecContext(
			ctx,
			statement,
			row.Subject,
			row.SubjectKind,
			sourceUnit,
			schemaSymbol,
			row.Posture,
			row.Rationale,
			artifactID,
		)
		if err != nil {
			return fmt.Errorf("insert TypeEnv coverage %s: %w", row.Subject, err)
		}
	}
	return nil
}

func insertSources(
	ctx context.Context,
	executor sqlExecutor,
	projection artifactProjection,
) error {
	statement := `INSERT INTO fpf_typeenv_sources (
		owner_kind, owner_key, source_ordinal, unit_id, source_revision,
		content_hash, start_line, end_line, pattern_id, artifact_digest
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	for _, row := range projection.sources {
		_, err := executor.ExecContext(
			ctx,
			statement,
			row.OwnerKind,
			row.OwnerKey,
			row.Ordinal,
			row.UnitID,
			row.Revision,
			row.ContentHash,
			row.StartLine,
			row.EndLine,
			nullableString(row.PatternID),
			projection.artifact.Digest().String(),
		)
		if err != nil {
			return fmt.Errorf("insert TypeEnv source %s/%s: %w", row.OwnerKind, row.OwnerKey, err)
		}
	}
	return nil
}

func insertCompatibility(
	ctx context.Context,
	executor sqlExecutor,
	projection artifactProjection,
) error {
	compatibility := projection.compatibility
	statement := `INSERT INTO fpf_typeenv_compatibility (
		singleton, artifact_digest, assessment_kind, base_typeenv_ref,
		change_count, changes_digest
	) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := executor.ExecContext(
		ctx,
		statement,
		artifactSingleton,
		projection.artifact.Digest().String(),
		compatibility.Kind,
		compatibility.BaseRef,
		len(compatibility.Changes),
		compatibility.Digest,
	)
	if err != nil {
		return fmt.Errorf("insert TypeEnv compatibility assessment: %w", err)
	}
	changeStatement := `INSERT INTO fpf_typeenv_compatibility_changes (
		change_ordinal, symbol, change_kind, rationale, artifact_digest
	) VALUES (?, ?, ?, ?, ?)`
	for _, change := range compatibility.Changes {
		_, err = executor.ExecContext(
			ctx,
			changeStatement,
			change.Ordinal,
			change.Symbol,
			change.Kind,
			change.Rationale,
			projection.artifact.Digest().String(),
		)
		if err != nil {
			return fmt.Errorf("insert TypeEnv compatibility change %s: %w", change.Symbol, err)
		}
	}
	return nil
}

func nullableString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func loadAndVerifyEnvelope(
	ctx context.Context,
	queryer sqlQueryer,
) (typeenv.CompilationEnvelope, error) {
	stored, err := loadStoredArtifact(ctx, queryer)
	if err != nil {
		return typeenv.CompilationEnvelope{}, err
	}
	artifact, err := typeenv.DecodeBaseTypeEnvArtifact(stored.Canonical)
	if err != nil {
		return typeenv.CompilationEnvelope{}, fmt.Errorf("decode authoritative TypeEnv artifact: %w", err)
	}
	compatibility, err := loadCompatibility(ctx, queryer, artifact)
	if err != nil {
		return typeenv.CompilationEnvelope{}, err
	}
	envelope, err := typeenv.NewCompilationEnvelope(artifact, compatibility)
	if err != nil {
		return typeenv.CompilationEnvelope{}, fmt.Errorf("rebuild TypeEnv envelope: %w", err)
	}
	if err := verifyStoredEnvelope(ctx, queryer, stored, envelope); err != nil {
		return typeenv.CompilationEnvelope{}, err
	}
	return envelope, nil
}

type storedArtifactRow struct {
	Canonical         []byte
	Digest            string
	Posture           string
	TypeEnvRef        sql.NullString
	Revision          string
	Compiler          string
	Reason            sql.NullString
	DeclarationCount  int
	DeclarationDigest string
	SourceCount       int
	SourceDigest      string
	CoverageCount     int
	CoverageDigest    string
}

func loadStoredArtifact(ctx context.Context, queryer sqlQueryer) (storedArtifactRow, error) {
	statement := `SELECT
		canonical_bytes, artifact_digest, posture, typeenv_ref,
		source_revision, compiler_schema_version, coverage_only_reason,
		declaration_count, declaration_projection_digest,
		source_count, source_projection_digest,
		coverage_count, coverage_projection_digest
	FROM fpf_typeenv_artifact WHERE singleton = ?`
	row := queryer.QueryRowContext(ctx, statement, artifactSingleton)
	stored := storedArtifactRow{}
	err := row.Scan(
		&stored.Canonical,
		&stored.Digest,
		&stored.Posture,
		&stored.TypeEnvRef,
		&stored.Revision,
		&stored.Compiler,
		&stored.Reason,
		&stored.DeclarationCount,
		&stored.DeclarationDigest,
		&stored.SourceCount,
		&stored.SourceDigest,
		&stored.CoverageCount,
		&stored.CoverageDigest,
	)
	if err != nil {
		return storedArtifactRow{}, fmt.Errorf("load authoritative TypeEnv artifact: %w", err)
	}
	return stored, nil
}

func loadCompatibility(
	ctx context.Context,
	queryer sqlQueryer,
	artifact typeenv.BaseTypeEnvArtifact,
) (typeenv.CompatibilityAssessment, error) {
	statement := `SELECT assessment_kind, base_typeenv_ref, change_count, changes_digest
		FROM fpf_typeenv_compatibility WHERE singleton = ? AND artifact_digest = ?`
	row := queryer.QueryRowContext(ctx, statement, artifactSingleton, artifact.Digest().String())
	var kind string
	var baseRef sql.NullString
	var count int
	var digest string
	if err := row.Scan(&kind, &baseRef, &count, &digest); err != nil {
		return nil, fmt.Errorf("load TypeEnv compatibility assessment: %w", err)
	}
	changes, err := loadCompatibilityChanges(ctx, queryer, artifact.Digest().String())
	if err != nil {
		return nil, err
	}
	expectedDigest, err := projectionDigest("compatibility", changes)
	if err != nil {
		return nil, err
	}
	if count != len(changes) || digest != expectedDigest {
		return nil, fmt.Errorf("TypeEnv compatibility projection integrity mismatch")
	}
	if kind == "initial" {
		if baseRef.Valid || len(changes) != 0 {
			return nil, fmt.Errorf("initial TypeEnv compatibility has comparison state")
		}
		return typeenv.NewInitialCompatibilityAssessment(), nil
	}
	if kind != "compared" || !baseRef.Valid {
		return nil, fmt.Errorf("unknown TypeEnv compatibility posture %q", kind)
	}
	return rebuildComparedCompatibility(baseRef.String, changes)
}

func loadCompatibilityChanges(
	ctx context.Context,
	queryer sqlQueryer,
	artifactID string,
) ([]compatibilityChangeRow, error) {
	statement := `SELECT change_ordinal, symbol, change_kind, rationale
		FROM fpf_typeenv_compatibility_changes
		WHERE artifact_digest = ? ORDER BY change_ordinal`
	rows, err := queryer.QueryContext(ctx, statement, artifactID)
	if err != nil {
		return nil, fmt.Errorf("load TypeEnv compatibility changes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	changes := make([]compatibilityChangeRow, 0)
	for rows.Next() {
		row := compatibilityChangeRow{}
		if err := rows.Scan(&row.Ordinal, &row.Symbol, &row.Kind, &row.Rationale); err != nil {
			return nil, fmt.Errorf("scan TypeEnv compatibility change: %w", err)
		}
		if row.Ordinal != len(changes) {
			return nil, fmt.Errorf("TypeEnv compatibility ordinals are not contiguous")
		}
		changes = append(changes, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate TypeEnv compatibility changes: %w", err)
	}
	return changes, nil
}

func rebuildComparedCompatibility(
	baseRef string,
	rows []compatibilityChangeRow,
) (typeenv.CompatibilityAssessment, error) {
	base, err := parseTypeEnvRef(baseRef)
	if err != nil {
		return nil, err
	}
	changes := make([]typedmemory.CompatibilityChange, 0, len(rows))
	for _, row := range rows {
		symbol, err := parseSchemaSymbol(row.Symbol)
		if err != nil {
			return nil, err
		}
		kind, err := parseCompatibilityKind(row.Kind)
		if err != nil {
			return nil, err
		}
		change, err := typedmemory.NewCompatibilityChange(symbol, kind, row.Rationale)
		if err != nil {
			return nil, fmt.Errorf("decode compatibility change %s: %w", row.Symbol, err)
		}
		changes = append(changes, change)
	}
	diff, err := typedmemory.NewTypeEnvCompatibilityDiff(base, changes)
	if err != nil {
		return nil, fmt.Errorf("decode TypeEnv compatibility diff: %w", err)
	}
	assessment, err := typeenv.NewComparedCompatibilityAssessment(diff)
	if err != nil {
		return nil, fmt.Errorf("decode compared TypeEnv compatibility: %w", err)
	}
	return assessment, nil
}

func parseTypeEnvRef(raw string) (typedmemory.TypeEnvRef, error) {
	digestText := strings.TrimPrefix(raw, "typeenv:")
	if digestText == raw {
		return typedmemory.TypeEnvRef{}, fmt.Errorf("invalid TypeEnvRef %q", raw)
	}
	digest, err := typedmemory.NewSHA256Digest(digestText)
	if err != nil {
		return typedmemory.TypeEnvRef{}, fmt.Errorf("decode TypeEnvRef: %w", err)
	}
	return typedmemory.NewTypeEnvRef(digest)
}

func parseSchemaSymbol(raw string) (typedmemory.SchemaSymbolRef, error) {
	kind, key, exists := strings.Cut(raw, ":")
	if !exists || key == "" {
		return typedmemory.SchemaSymbolRef{}, fmt.Errorf("invalid schema symbol %q", raw)
	}
	switch kind {
	case "context":
		ref, err := typedmemory.NewBoundedContextRef(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.BoundedContextSymbolRef(ref)
	case "kind":
		id, err := typedmemory.NewKindID(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.KindSymbolRef(id)
	case "slot_kind":
		return parseSlotKindSymbol(key)
	case "ref_kind":
		id, err := typedmemory.NewRefKindID(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.RefKindSymbolRef(id)
	case "bridge":
		id, err := typedmemory.NewContextBridgeID(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.ContextBridgeSymbolRef(id)
	case "signature":
		id, err := typedmemory.NewSignatureID(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.RelationSymbolRef(id)
	case "shape":
		id, err := typedmemory.NewShapeID(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.ValueShapeSymbolRef(id)
	case "codec":
		id, err := typedmemory.NewCodecID(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.CodecSymbolRef(id)
	case "constraint":
		id, err := typedmemory.NewConstraintID(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.ConstraintSymbolRef(id)
	case "entity_set":
		id, err := typedmemory.NewEntitySetSymbolID(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.EntitySetSymbolRef(id)
	case "kind_signature":
		id, err := typedmemory.NewKindSignatureSymbolID(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.KindSignatureSymbolRef(id)
	default:
		return typedmemory.SchemaSymbolRef{}, fmt.Errorf("unknown schema symbol kind %q", kind)
	}
}

func parseSlotKindSymbol(key string) (typedmemory.SchemaSymbolRef, error) {
	signatureText, slotText, exists := strings.Cut(key, "/slot/")
	if !exists || signatureText == "" || slotText == "" {
		return typedmemory.SchemaSymbolRef{}, fmt.Errorf("invalid SlotKind schema symbol %q", key)
	}
	signature, err := typedmemory.NewSignatureID(signatureText)
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, err
	}
	slotKind, err := typedmemory.NewSlotKindID(slotText)
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, err
	}
	return typedmemory.SlotKindSymbolRef(signature, slotKind)
}

func parseCompatibilityKind(raw string) (typedmemory.CompatibilityChangeKind, error) {
	switch raw {
	case "added":
		return typedmemory.CompatibilityAdded, nil
	case "changed":
		return typedmemory.CompatibilityChanged, nil
	case "removed":
		return typedmemory.CompatibilityRemoved, nil
	default:
		return 0, fmt.Errorf("unknown compatibility change kind %q", raw)
	}
}

func verifyStoredEnvelope(
	ctx context.Context,
	queryer sqlQueryer,
	stored storedArtifactRow,
	envelope typeenv.CompilationEnvelope,
) error {
	expected, err := projectEnvelope(envelope)
	if err != nil {
		return err
	}
	if err := verifyStoredArtifactMetadata(stored, expected); err != nil {
		return err
	}
	if err := verifyProjectionCardinalities(ctx, queryer, expected); err != nil {
		return err
	}
	declarations, err := loadDeclarationProjection(ctx, queryer, stored.Digest)
	if err != nil {
		return err
	}
	if !equalJSON(declarations, expected.declarations) {
		return fmt.Errorf("TypeEnv declaration projection does not match authoritative artifact")
	}
	coverage, err := loadCoverageProjection(ctx, queryer, stored.Digest)
	if err != nil {
		return err
	}
	if !equalJSON(coverage, expected.coverage) {
		return fmt.Errorf("TypeEnv coverage projection does not match authoritative artifact")
	}
	sources, err := loadSourceProjection(ctx, queryer, stored.Digest)
	if err != nil {
		return err
	}
	if !equalJSON(sources, expected.sources) {
		return fmt.Errorf("TypeEnv source projection does not match authoritative artifact")
	}
	for _, source := range sources {
		if err := verifySourceUnit(ctx, queryer, source); err != nil {
			return err
		}
	}
	return verifyStoredCompatibility(ctx, queryer, expected.compatibility)
}

func verifyProjectionCardinalities(
	ctx context.Context,
	queryer sqlQueryer,
	expected artifactProjection,
) error {
	dependencyCount := 0
	for _, declaration := range expected.declarations {
		dependencyCount += len(declaration.Dependencies)
	}
	checks := []struct {
		table string
		want  int
	}{
		{table: "fpf_typeenv_declarations", want: len(expected.declarations)},
		{table: "fpf_typeenv_declaration_dependencies", want: dependencyCount},
		{table: "fpf_typeenv_coverage", want: len(expected.coverage)},
		{table: "fpf_typeenv_sources", want: len(expected.sources)},
		{table: "fpf_typeenv_compatibility_changes", want: len(expected.compatibility.Changes)},
	}
	for _, check := range checks {
		statement := "SELECT COUNT(*) FROM " + check.table
		var count int
		if err := queryer.QueryRowContext(ctx, statement).Scan(&count); err != nil {
			return fmt.Errorf("count %s: %w", check.table, err)
		}
		if count != check.want {
			return fmt.Errorf("%s has %d rows, want %d", check.table, count, check.want)
		}
	}
	return nil
}

func verifyStoredArtifactMetadata(
	stored storedArtifactRow,
	expected artifactProjection,
) error {
	if stored.Digest != expected.artifact.Digest().String() {
		return fmt.Errorf("stored TypeEnv digest does not match canonical payload")
	}
	if stored.Posture != expected.posture {
		return fmt.Errorf("stored TypeEnv posture does not match canonical payload")
	}
	if stored.TypeEnvRef != expected.typeEnvRef {
		return fmt.Errorf("stored TypeEnvRef does not match canonical payload")
	}
	if stored.Revision != expected.artifact.SourceRevision().String() {
		return fmt.Errorf("stored TypeEnv source revision does not match canonical payload")
	}
	if stored.Compiler != expected.artifact.CompilerSchemaVersion().String() {
		return fmt.Errorf("stored compiler schema does not match canonical payload")
	}
	if stored.Reason != expected.reason {
		return fmt.Errorf("stored TypeEnv coverage-only reason does not match canonical payload")
	}
	actualSummary := ProjectionSummary{
		DeclarationCount:  stored.DeclarationCount,
		DeclarationDigest: stored.DeclarationDigest,
		SourceCount:       stored.SourceCount,
		SourceDigest:      stored.SourceDigest,
		CoverageCount:     stored.CoverageCount,
		CoverageDigest:    stored.CoverageDigest,
	}
	if actualSummary != expected.summary {
		return fmt.Errorf("stored TypeEnv projection summary does not match canonical payload")
	}
	return nil
}

func loadDeclarationProjection(
	ctx context.Context,
	queryer sqlQueryer,
	artifactID string,
) ([]declarationRow, error) {
	statement := `SELECT
		symbol, symbol_kind, symbol_key, declaration_digest,
		compiler_rule_id, basis_kind, dependency_count, dependency_digest
	FROM fpf_typeenv_declarations
	WHERE artifact_digest = ? ORDER BY symbol`
	rows, err := queryer.QueryContext(ctx, statement, artifactID)
	if err != nil {
		return nil, fmt.Errorf("load TypeEnv declarations: %w", err)
	}
	type storedDeclaration struct {
		row              declarationRow
		dependencyCount  int
		dependencyDigest string
	}
	stored := make([]storedDeclaration, 0)
	for rows.Next() {
		item := storedDeclaration{}
		err := rows.Scan(
			&item.row.Symbol,
			&item.row.SymbolKind,
			&item.row.SymbolKey,
			&item.row.DeclarationDigest,
			&item.row.CompilerRuleID,
			&item.row.BasisKind,
			&item.dependencyCount,
			&item.dependencyDigest,
		)
		if err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan TypeEnv declaration: %w", err)
		}
		stored = append(stored, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate TypeEnv declarations: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close TypeEnv declaration rows: %w", err)
	}
	declarations := make([]declarationRow, 0, len(stored))
	for _, item := range stored {
		dependencies, err := loadDependencies(ctx, queryer, item.row.Symbol)
		if err != nil {
			return nil, err
		}
		expectedDigest, err := projectionDigest("dependencies:"+item.row.Symbol, dependencies)
		if err != nil {
			return nil, err
		}
		if item.dependencyCount != len(dependencies) || item.dependencyDigest != expectedDigest {
			return nil, fmt.Errorf("TypeEnv dependency projection mismatch for %s", item.row.Symbol)
		}
		item.row.Dependencies = dependencies
		declarations = append(declarations, item.row)
	}
	return declarations, nil
}

func loadDependencies(
	ctx context.Context,
	queryer sqlQueryer,
	symbol string,
) ([]string, error) {
	statement := `SELECT dependency_ordinal, dependency_symbol
		FROM fpf_typeenv_declaration_dependencies
		WHERE symbol = ? ORDER BY dependency_ordinal`
	rows, err := queryer.QueryContext(ctx, statement, symbol)
	if err != nil {
		return nil, fmt.Errorf("load TypeEnv dependencies for %s: %w", symbol, err)
	}
	defer func() { _ = rows.Close() }()
	dependencies := make([]string, 0)
	for rows.Next() {
		var ordinal int
		var dependency string
		if err := rows.Scan(&ordinal, &dependency); err != nil {
			return nil, fmt.Errorf("scan TypeEnv dependency: %w", err)
		}
		if ordinal != len(dependencies) {
			return nil, fmt.Errorf("TypeEnv dependency ordinals are not contiguous for %s", symbol)
		}
		dependencies = append(dependencies, dependency)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate TypeEnv dependencies: %w", err)
	}
	return dependencies, nil
}

func loadCoverageProjection(
	ctx context.Context,
	queryer sqlQueryer,
	artifactID string,
) ([]coverageRow, error) {
	statement := `SELECT
		subject, subject_kind, source_unit_id, schema_symbol, posture, rationale
	FROM fpf_typeenv_coverage
	WHERE artifact_digest = ? ORDER BY subject`
	rows, err := queryer.QueryContext(ctx, statement, artifactID)
	if err != nil {
		return nil, fmt.Errorf("load TypeEnv coverage: %w", err)
	}
	defer func() { _ = rows.Close() }()
	coverage := make([]coverageRow, 0)
	for rows.Next() {
		row := coverageRow{}
		var sourceUnit sql.NullString
		var schemaSymbol sql.NullString
		if err := rows.Scan(
			&row.Subject,
			&row.SubjectKind,
			&sourceUnit,
			&schemaSymbol,
			&row.Posture,
			&row.Rationale,
		); err != nil {
			return nil, fmt.Errorf("scan TypeEnv coverage: %w", err)
		}
		row.SourceUnitID = sourceUnit.String
		row.SchemaSymbol = schemaSymbol.String
		coverage = append(coverage, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate TypeEnv coverage: %w", err)
	}
	return coverage, nil
}

func loadSourceProjection(
	ctx context.Context,
	queryer sqlQueryer,
	artifactID string,
) ([]sourceRow, error) {
	statement := `SELECT
		owner_kind, owner_key, source_ordinal, unit_id, source_revision,
		content_hash, start_line, end_line, pattern_id, artifact_digest
	FROM fpf_typeenv_sources
	WHERE artifact_digest = ?
	ORDER BY owner_kind, owner_key, source_ordinal`
	rows, err := queryer.QueryContext(ctx, statement, artifactID)
	if err != nil {
		return nil, fmt.Errorf("load TypeEnv sources: %w", err)
	}
	defer func() { _ = rows.Close() }()
	sources := make([]sourceRow, 0)
	for rows.Next() {
		row := sourceRow{}
		var patternID sql.NullString
		if err := rows.Scan(
			&row.OwnerKind,
			&row.OwnerKey,
			&row.Ordinal,
			&row.UnitID,
			&row.Revision,
			&row.ContentHash,
			&row.StartLine,
			&row.EndLine,
			&patternID,
			&row.ArtifactDigest,
		); err != nil {
			return nil, fmt.Errorf("scan TypeEnv source: %w", err)
		}
		row.PatternID = patternID.String
		sources = append(sources, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate TypeEnv sources: %w", err)
	}
	return sources, nil
}

func verifySourceUnit(ctx context.Context, queryer sqlQueryer, source sourceRow) error {
	statement := `SELECT content_hash, source_revision, start_line, end_line, pattern_id
		FROM source_units WHERE unit_id = ?`
	row := queryer.QueryRowContext(ctx, statement, source.UnitID)
	var contentHash string
	var revision string
	var startLine uint64
	var endLine uint64
	var patternID string
	if err := row.Scan(&contentHash, &revision, &startLine, &endLine, &patternID); err != nil {
		return fmt.Errorf("resolve TypeEnv source unit %s: %w", source.UnitID, err)
	}
	if normalizedDigest(contentHash) != normalizedDigest(source.ContentHash) {
		return fmt.Errorf("TypeEnv source unit %s content hash mismatch", source.UnitID)
	}
	if revision != source.Revision {
		return fmt.Errorf("TypeEnv source unit %s revision mismatch", source.UnitID)
	}
	if startLine != source.StartLine || endLine != source.EndLine {
		return fmt.Errorf("TypeEnv source unit %s line range mismatch", source.UnitID)
	}
	if source.PatternID != "" && patternID != source.PatternID {
		return fmt.Errorf("TypeEnv source unit %s PatternID mismatch", source.UnitID)
	}
	return nil
}

func normalizedDigest(raw string) string {
	value := strings.TrimSpace(raw)
	return strings.TrimPrefix(value, "sha256:")
}

func verifyStoredCompatibility(
	ctx context.Context,
	queryer sqlQueryer,
	expected compatibilityProjection,
) error {
	statement := `SELECT artifact_digest, assessment_kind, base_typeenv_ref,
		change_count, changes_digest
	FROM fpf_typeenv_compatibility WHERE singleton = ?`
	row := queryer.QueryRowContext(ctx, statement, artifactSingleton)
	var artifactID string
	var kind string
	var baseRef sql.NullString
	var count int
	var digest string
	if err := row.Scan(&artifactID, &kind, &baseRef, &count, &digest); err != nil {
		return fmt.Errorf("verify TypeEnv compatibility assessment: %w", err)
	}
	if artifactID != expected.ArtifactID || kind != expected.Kind || baseRef != expected.BaseRef {
		return fmt.Errorf("TypeEnv compatibility metadata mismatch")
	}
	if count != len(expected.Changes) || digest != expected.Digest {
		return fmt.Errorf("TypeEnv compatibility summary mismatch")
	}
	changes, err := loadCompatibilityChanges(ctx, queryer, artifactID)
	if err != nil {
		return err
	}
	if !equalJSON(changes, expected.Changes) {
		return fmt.Errorf("TypeEnv compatibility changes mismatch")
	}
	return nil
}

func equalJSON(left, right any) bool {
	leftBytes, leftErr := json.Marshal(left)
	rightBytes, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftBytes) == string(rightBytes)
}

func IsMissingArtifact(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
