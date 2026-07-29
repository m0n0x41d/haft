package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	_ "modernc.org/sqlite"
)

var storeSourceUnitsFunc = fpf.StoreSourceUnits
var compileBaseTypeEnvFunc = typeenv.CompileBaseTypeEnv
var storeTypeEnvEnvelopeFunc = storeTypeEnvEnvelope
var verifyBuiltIndexFunc = verifyBuiltIndex
var afterPreviousTypeEnvCompilerProbeFunc = func(string) error { return nil }
var closePreviousTypeEnvSnapshotFunc = func(database *sql.DB) error {
	return database.Close()
}

type previousTypeEnvSnapshot struct {
	database    *sql.DB
	transaction *sql.Tx
}

const (
	legacyBaseTypeEnvCompilerSchemaV1   = "fpf-base-typeenv.cov2.v1"
	previousBaseTypeEnvCompilerSchemaV2 = typeenv.BaseTypeEnvCompilerSchemaV2
	previousBaseTypeEnvCompilerSchemaV3 = typeenv.BaseTypeEnvCompilerSchemaV3
	previousBaseTypeEnvCompilerSchemaV4 = typeenv.BaseTypeEnvCompilerSchemaV4
)

// verifyIndex is the CI guard for the committed, source-derived FPF index.
// It never rebuilds. A stale revision, schema mismatch, broken provenance, or
// incomplete source index must fail loudly before a release can use fpf.db.
func verifyIndex(args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("usage: indexer -verify <fpf.db> <FPF-Spec.md> <expected-fpf-commit-sha>")
	}

	dbPath := args[0]
	specPath := filepath.Clean(args[1])
	expectedSHA := strings.TrimSpace(args[2])
	if expectedSHA == "" {
		return fmt.Errorf("expected FPF commit SHA is required")
	}
	readmePath := filepath.Join(filepath.Dir(specPath), "Readme.md")
	snapshot, err := fpf.LoadPublicationSnapshot(readmePath, specPath, expectedSHA)
	if err != nil {
		return fmt.Errorf("load checked-out FPF publication snapshot: %w", err)
	}
	compilation, err := compileBaseTypeEnvFunc(snapshot)
	if err != nil {
		return fmt.Errorf("compile checked-out FPF TypeEnv: %w", err)
	}
	if compilation == nil {
		return fmt.Errorf("compile checked-out FPF TypeEnv returned no result")
	}
	if compilation.Rejected() {
		return fmt.Errorf(
			"compile checked-out FPF TypeEnv rejected publication: %s",
			formatCompilerDiagnostics(compilation.Diagnostics()),
		)
	}
	expectedArtifact, exists := compilation.Artifact()
	if !exists {
		return fmt.Errorf("checked-out FPF TypeEnv compilation returned no artifact")
	}

	db, err := openSQLiteReadOnly(dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	commit, err := fpf.GetSpecMeta(db, "fpf_commit")
	if err != nil {
		return fmt.Errorf("read fpf_commit meta: %w", err)
	}
	if strings.TrimSpace(commit) != expectedSHA {
		return fmt.Errorf("fpf.db is STALE: meta fpf_commit=%q but submodule HEAD=%q — run `task fpf-refresh` and commit the result", commit, expectedSHA)
	}

	schemaVersion, err := fpf.GetSpecMeta(db, "schema_version")
	if err != nil {
		return fmt.Errorf("read schema_version meta: %w", err)
	}
	if strings.TrimSpace(schemaVersion) != fpf.SpecIndexSchemaVersion {
		return fmt.Errorf("fpf.db schema_version=%q but code expects %q — run `task fpf-refresh` and commit the result", schemaVersion, fpf.SpecIndexSchemaVersion)
	}

	if err := fpf.VerifyPublicationSnapshotReadOnlyDB(db, snapshot); err != nil {
		return fmt.Errorf("verify checked-out FPF publication snapshot: %w", err)
	}
	artifact, err := typeenvsql.LoadArtifactReadOnlyDB(context.Background(), db)
	if err != nil {
		return fmt.Errorf("load source-derived FPF TypeEnv: %w", err)
	}
	if err := verifyExpectedTypeEnvArtifact(artifact, expectedArtifact); err != nil {
		return err
	}
	if err := verifyArtifactMetadata(db, artifact, expectedSHA); err != nil {
		return err
	}
	if err := verifyTypeEnvSourceJoin(db); err != nil {
		return err
	}
	staleSourceUnits, err := countSourceUnitsOutsideRevision(db, expectedSHA)
	if err != nil {
		return fmt.Errorf("verify source unit revisions: %w", err)
	}
	if staleSourceUnits > 0 {
		return fmt.Errorf("fpf.db source rows are STALE: %d source unit(s) do not match submodule HEAD %q", staleSourceUnits, expectedSHA)
	}

	sourceUnits, err := fpf.CountSourceUnits(db)
	if err != nil {
		return fmt.Errorf("count source units: %w", err)
	}
	expectedSourceUnits, err := readExpectedSourceUnitCount(db)
	if err != nil {
		return err
	}
	if sourceUnits != expectedSourceUnits {
		return fmt.Errorf("fpf.db source unit count mismatch: metadata expects %d, found %d", expectedSourceUnits, sourceUnits)
	}

	shortCommit := commit[:min(8, len(commit))]
	artifactRef, _ := artifact.TypeEnvRef()
	fmt.Printf(
		"fpf.db OK: commit %s, schema %s, %d source units and TypeEnv %s with verified provenance, typed relations, and FTS\n",
		shortCommit,
		schemaVersion,
		sourceUnits,
		artifactRef.String(),
	)
	return nil
}

func readExpectedSourceUnitCount(db *sql.DB) (int, error) {
	value, err := fpf.GetSpecMeta(db, "indexed_source_units")
	if err != nil {
		return 0, fmt.Errorf("read indexed_source_units meta: %w", err)
	}
	count, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("indexed_source_units=%q is not an integer", value)
	}
	if count <= 0 {
		return 0, fmt.Errorf("indexed_source_units=%d must be positive", count)
	}
	return count, nil
}

func countSourceUnitsOutsideRevision(db *sql.DB, expectedRevision string) (int, error) {
	var count int
	err := db.
		QueryRow(
			`SELECT COUNT(*) FROM source_units WHERE source_revision <> ?`,
			expectedRevision,
		).
		Scan(&count)
	return count, err
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: indexer <FPF-Spec.md> [output.db] [fpf-commit-sha]  |  indexer -verify <fpf.db> <FPF-Spec.md> <expected-sha>")
	}
	if os.Args[1] == "-verify" {
		return verifyIndex(os.Args[2:])
	}

	specPath := os.Args[1]
	dbPath := filepath.Join("internal", "cli", "fpf.db")
	if len(os.Args) >= 3 {
		dbPath = os.Args[2]
	}
	commitSHA := ""
	if len(os.Args) >= 4 {
		commitSHA = os.Args[3]
	}

	return buildIndex(specPath, dbPath, commitSHA)
}

func buildIndex(specPath, dbPath, commitSHA string) error {
	readmePath := filepath.Join(filepath.Dir(specPath), "Readme.md")
	resolvedCommit := resolveSpecCommit(commitSHA, specPath)
	snapshot, err := fpf.LoadPublicationSnapshot(readmePath, specPath, resolvedCommit)
	if err != nil {
		return fmt.Errorf("load FPF source publications: %w", err)
	}
	sourceUnits := snapshot.SourceUnits()
	compilation, err := compileBaseTypeEnvFunc(snapshot)
	if err != nil {
		return fmt.Errorf("compile source-derived FPF TypeEnv: %w", err)
	}
	if compilation == nil {
		return fmt.Errorf("compile source-derived FPF TypeEnv returned no result")
	}
	if compilation.Rejected() {
		return fmt.Errorf(
			"compile source-derived FPF TypeEnv rejected publication: %s",
			formatCompilerDiagnostics(compilation.Diagnostics()),
		)
	}
	artifact, exists := compilation.Artifact()
	if !exists {
		return fmt.Errorf("accepted FPF TypeEnv compilation returned no artifact")
	}
	compatibility, err := comparePreviousTypeEnv(dbPath, artifact)
	if err != nil {
		return err
	}
	envelope, err := typeenv.NewCompilationEnvelope(artifact, compatibility)
	if err != nil {
		return fmt.Errorf("seal FPF TypeEnv compilation envelope: %w", err)
	}

	buildTime := resolveSpecBuildTime(resolvedCommit, specPath)
	metadata := buildSourceIndexMetadata(
		specPath,
		readmePath,
		len(sourceUnits),
		resolvedCommit,
		buildTime,
	)
	metadata["readme_document_digest"] = snapshot.ReadmeDigest().String()
	metadata["spec_document_digest"] = snapshot.SpecDigest().String()
	addTypeEnvMetadata(metadata, artifact)
	build := func(tempPath string) error {
		if err := storeSourceUnitsFunc(tempPath, sourceUnits); err != nil {
			return fmt.Errorf("store source query units: %w", err)
		}
		if err := ensureIndexMetadataTable(tempPath); err != nil {
			return err
		}
		if err := fpf.SetSpecMetaEntries(tempPath, metadata); err != nil {
			return fmt.Errorf("set source index metadata: %w", err)
		}
		if err := storeTypeEnvEnvelopeFunc(tempPath, envelope); err != nil {
			return fmt.Errorf("store source-derived FPF TypeEnv: %w", err)
		}
		return nil
	}
	verify := func(database *sql.DB) error {
		return verifyBuiltIndexFunc(database, snapshot, artifact)
	}
	if err := fpf.RebuildSourceIndexAtomic(dbPath, build, verify); err != nil {
		return fmt.Errorf("atomically rebuild source-native FPF index: %w", err)
	}

	fmt.Printf(
		"Indexed %d source units and TypeEnv %s from Readme.md + FPF-Spec.md into %s\n",
		len(sourceUnits),
		artifact.Digest().String(),
		dbPath,
	)
	return nil
}

func formatCompilerDiagnostics(diagnostics []typeenv.CompilerDiagnostic) string {
	if len(diagnostics) == 0 {
		return "no compiler diagnostic was provided"
	}
	formatted := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		formatted = append(
			formatted,
			fmt.Sprintf(
				"%s[%s]: %s",
				diagnostic.Code(),
				diagnostic.UnitID(),
				diagnostic.Message(),
			),
		)
	}
	return strings.Join(formatted, "; ")
}

func comparePreviousTypeEnv(
	dbPath string,
	current typeenv.BaseTypeEnvArtifact,
) (typeenv.CompatibilityAssessment, error) {
	snapshot, hasPreviousSnapshot, err := openPreviousTypeEnvSnapshot(dbPath)
	if err != nil {
		return nil, err
	}
	if !hasPreviousSnapshot {
		return typeenv.NewInitialCompatibilityAssessment(), nil
	}
	assessment, compareErr := comparePreviousTypeEnvSnapshot(
		dbPath,
		snapshot.transaction,
		current,
	)
	if err := finishPreviousTypeEnvSnapshot(snapshot, compareErr); err != nil {
		return nil, err
	}
	return assessment, nil
}

func comparePreviousTypeEnvSnapshot(
	dbPath string,
	transaction *sql.Tx,
	current typeenv.BaseTypeEnvArtifact,
) (typeenv.CompatibilityAssessment, error) {
	previousCompiler, hasPreviousCompiler, err :=
		loadPreviousTypeEnvCompilerSchemaSnapshot(transaction)
	if err != nil {
		return nil, err
	}
	if !hasPreviousCompiler {
		return typeenv.NewInitialCompatibilityAssessment(), nil
	}
	if err := afterPreviousTypeEnvCompilerProbeFunc(filepath.Clean(dbPath)); err != nil {
		return nil, fmt.Errorf("after previous FPF TypeEnv compiler probe: %w", err)
	}
	if previousCompiler == legacyBaseTypeEnvCompilerSchemaV1 {
		return typeenv.NewInitialCompatibilityAssessment(), nil
	}
	currentCompiler := current.CompilerSchemaVersion().String()
	if previousCompiler != currentCompiler &&
		previousCompiler != previousBaseTypeEnvCompilerSchemaV2 &&
		previousCompiler != previousBaseTypeEnvCompilerSchemaV3 &&
		previousCompiler != previousBaseTypeEnvCompilerSchemaV4 {
		return nil, fmt.Errorf(
			"previous FPF TypeEnv compiler schema %q is neither current %q nor a known predecessor (%q, %q, %q, %q)",
			previousCompiler,
			currentCompiler,
			legacyBaseTypeEnvCompilerSchemaV1,
			previousBaseTypeEnvCompilerSchemaV2,
			previousBaseTypeEnvCompilerSchemaV3,
			previousBaseTypeEnvCompilerSchemaV4,
		)
	}
	previousEnvelope, exists, err := loadPreviousTypeEnvEnvelopeSnapshot(transaction)
	if err != nil {
		return nil, err
	}
	if !exists {
		return typeenv.NewInitialCompatibilityAssessment(), nil
	}
	previous := previousEnvelope.Artifact()
	if previous.Digest() == current.Digest() {
		compatibility := previousEnvelope.Compatibility()
		compared, comparedAssessment := compatibility.(typeenv.ComparedCompatibilityAssessment)
		currentRef, hasCurrentRef := current.TypeEnvRef()
		if comparedAssessment &&
			hasCurrentRef &&
			compared.Diff().Base() == currentRef {
			return nil, fmt.Errorf(
				"previous FPF TypeEnv compatibility assessment compares unchanged artifact %s against itself",
				currentRef.String(),
			)
		}
		return compatibility, nil
	}
	assessment, err := typeenv.CompareBaseTypeEnvArtifacts(previous, current)
	if err != nil {
		return nil, fmt.Errorf("compare previous source-derived FPF TypeEnv: %w", err)
	}
	return assessment, nil
}

func openPreviousTypeEnvSnapshot(
	dbPath string,
) (*previousTypeEnvSnapshot, bool, error) {
	cleanPath := filepath.Clean(dbPath)
	_, err := os.Stat(cleanPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("inspect previous FPF index: %w", err)
	}
	database, err := openSQLiteReadOnly(cleanPath)
	if err != nil {
		return nil, false, err
	}
	transaction, err := database.BeginTx(
		context.Background(),
		&sql.TxOptions{ReadOnly: true},
	)
	if err != nil {
		beginErr := fmt.Errorf("begin previous FPF index read-only snapshot: %w", err)
		closeErr := database.Close()
		if closeErr != nil {
			return nil, false, errors.Join(
				beginErr,
				fmt.Errorf("close previous FPF index after snapshot failure: %w", closeErr),
			)
		}
		return nil, false, beginErr
	}
	return &previousTypeEnvSnapshot{
		database:    database,
		transaction: transaction,
	}, true, nil
}

func finishPreviousTypeEnvSnapshot(
	snapshot *previousTypeEnvSnapshot,
	operationErr error,
) error {
	errs := []error{operationErr}
	if rollbackErr := snapshot.transaction.Rollback(); rollbackErr != nil &&
		!errors.Is(rollbackErr, sql.ErrTxDone) {
		errs = append(
			errs,
			fmt.Errorf("end previous FPF index read-only snapshot: %w", rollbackErr),
		)
	}
	if closeErr := closePreviousTypeEnvSnapshotFunc(snapshot.database); closeErr != nil {
		errs = append(errs, fmt.Errorf("close previous FPF index: %w", closeErr))
	}
	return errors.Join(errs...)
}

func loadPreviousTypeEnvCompilerSchemaSnapshot(
	transaction *sql.Tx,
) (string, bool, error) {
	var compiler string
	err := transaction.QueryRowContext(
		context.Background(),
		`SELECT compiler_schema_version
		 FROM fpf_typeenv_artifact
		 WHERE singleton = 1`,
	).Scan(&compiler)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf(
			"load previous FPF TypeEnv compiler schema read-only: %w",
			err,
		)
	}
	return compiler, true, nil
}

func loadPreviousTypeEnvEnvelopeSnapshot(
	transaction *sql.Tx,
) (typeenv.CompilationEnvelope, bool, error) {
	envelope, err := typeenvsql.LoadEnvelopeTx(context.Background(), transaction)
	if errors.Is(err, sql.ErrNoRows) {
		return typeenv.CompilationEnvelope{}, false, nil
	}
	if err != nil {
		return typeenv.CompilationEnvelope{}, false, fmt.Errorf(
			"load previous FPF TypeEnv envelope read-only: %w",
			err,
		)
	}
	return envelope, true, nil
}

func openSQLiteReadOnly(dbPath string) (*sql.DB, error) {
	absolutePath, err := filepath.Abs(filepath.Clean(dbPath))
	if err != nil {
		return nil, fmt.Errorf("resolve FPF index path: %w", err)
	}
	readOnlyURI := url.URL{Scheme: "file", Path: absolutePath}
	query := readOnlyURI.Query()
	query.Set("mode", "ro")
	readOnlyURI.RawQuery = query.Encode()
	database, err := sql.Open("sqlite", readOnlyURI.String())
	if err != nil {
		return nil, fmt.Errorf("open FPF index read-only: %w", err)
	}
	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("open FPF index read-only: %w", err)
	}
	return database, nil
}

func storeTypeEnvEnvelope(
	dbPath string,
	envelope typeenv.CompilationEnvelope,
) error {
	database, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open FPF TypeEnv index: %w", err)
	}
	storeErr := typeenvsql.ReplaceEnvelopeDB(context.Background(), database, envelope)
	closeErr := database.Close()
	if storeErr != nil {
		return storeErr
	}
	if closeErr != nil {
		return fmt.Errorf("close FPF TypeEnv index: %w", closeErr)
	}
	return nil
}

func addTypeEnvMetadata(
	metadata map[string]string,
	artifact typeenv.BaseTypeEnvArtifact,
) {
	metadata["typeenv_artifact_digest"] = artifact.Digest().String()
	metadata["typeenv_compiler_schema_version"] = artifact.CompilerSchemaVersion().String()
	metadata["typeenv_posture"] = artifact.Posture().String()
	metadata["typeenv_source_revision"] = artifact.SourceRevision().String()
	if reference, exists := artifact.TypeEnvRef(); exists {
		metadata["typeenv_ref"] = reference.String()
	}
}

func verifyBuiltIndex(
	database *sql.DB,
	snapshot fpf.PublicationSnapshot,
	artifact typeenv.BaseTypeEnvArtifact,
) error {
	if err := fpf.VerifyPublicationSnapshotReadOnlyDB(database, snapshot); err != nil {
		return fmt.Errorf("verify exact FPF publication snapshot: %w", err)
	}
	loaded, err := typeenvsql.LoadArtifactReadOnlyDB(context.Background(), database)
	if err != nil {
		return fmt.Errorf("load rebuilt FPF TypeEnv artifact: %w", err)
	}
	if loaded.Digest().String() != artifact.Digest().String() {
		return fmt.Errorf(
			"rebuilt TypeEnv digest %q differs from compiled artifact %q",
			loaded.Digest().String(),
			artifact.Digest().String(),
		)
	}
	if artifact.SourceRevision().String() != snapshot.Revision() {
		return fmt.Errorf(
			"compiled TypeEnv revision %q differs from publication revision %q",
			artifact.SourceRevision().String(),
			snapshot.Revision(),
		)
	}
	if err := verifyArtifactMetadata(database, loaded, snapshot.Revision()); err != nil {
		return err
	}
	return verifyTypeEnvSourceJoin(database)
}

func verifyExpectedTypeEnvArtifact(
	stored typeenv.BaseTypeEnvArtifact,
	expected typeenv.BaseTypeEnvArtifact,
) error {
	if stored.Digest().String() != expected.Digest().String() {
		return fmt.Errorf(
			"fpf.db TypeEnv digest %q differs from checked-out source compilation %q",
			stored.Digest().String(),
			expected.Digest().String(),
		)
	}
	if !bytes.Equal(stored.CanonicalBytes(), expected.CanonicalBytes()) {
		return fmt.Errorf("fpf.db TypeEnv canonical payload differs from checked-out source compilation")
	}
	if stored.Posture() != expected.Posture() {
		return fmt.Errorf(
			"fpf.db TypeEnv posture %q differs from checked-out source compilation %q",
			stored.Posture().String(),
			expected.Posture().String(),
		)
	}
	if stored.SourceRevision().String() != expected.SourceRevision().String() {
		return fmt.Errorf(
			"fpf.db TypeEnv revision %q differs from checked-out source compilation %q",
			stored.SourceRevision().String(),
			expected.SourceRevision().String(),
		)
	}
	if stored.CompilerSchemaVersion().String() != expected.CompilerSchemaVersion().String() {
		return fmt.Errorf(
			"fpf.db TypeEnv compiler %q differs from checked-out source compilation %q",
			stored.CompilerSchemaVersion().String(),
			expected.CompilerSchemaVersion().String(),
		)
	}
	storedRef, storedHasRef := stored.TypeEnvRef()
	expectedRef, expectedHasRef := expected.TypeEnvRef()
	if storedHasRef != expectedHasRef || storedRef.String() != expectedRef.String() {
		return fmt.Errorf(
			"fpf.db TypeEnv ref %q differs from checked-out source compilation %q",
			storedRef.String(),
			expectedRef.String(),
		)
	}
	return nil
}

func verifyArtifactMetadata(
	database *sql.DB,
	artifact typeenv.BaseTypeEnvArtifact,
	expectedRevision string,
) error {
	values := map[string]string{
		"typeenv_artifact_digest":         artifact.Digest().String(),
		"typeenv_compiler_schema_version": artifact.CompilerSchemaVersion().String(),
		"typeenv_posture":                 artifact.Posture().String(),
		"typeenv_source_revision":         artifact.SourceRevision().String(),
	}
	reference, hasReference := artifact.TypeEnvRef()
	if !hasReference {
		return fmt.Errorf("compiled FPF TypeEnv artifact has no TypeEnvRef")
	}
	values["typeenv_ref"] = reference.String()
	if artifact.SourceRevision().String() != expectedRevision {
		return fmt.Errorf(
			"FPF TypeEnv source revision %q differs from expected revision %q",
			artifact.SourceRevision().String(),
			expectedRevision,
		)
	}
	for key, want := range values {
		got, err := fpf.GetSpecMeta(database, key)
		if err != nil {
			return fmt.Errorf("read %s metadata: %w", key, err)
		}
		if got != want {
			return fmt.Errorf("FPF index metadata %s=%q, want %q", key, got, want)
		}
	}
	return nil
}

func verifyTypeEnvSourceJoin(database *sql.DB) error {
	var mismatches int
	err := database.QueryRow(`
		SELECT COUNT(*)
		FROM fpf_typeenv_sources AS typed
		WHERE NOT EXISTS (
			SELECT 1
			FROM source_units AS source
			WHERE source.unit_id = typed.unit_id
			  AND source.source_revision = typed.source_revision
			  AND CASE
				WHEN source.content_hash LIKE 'sha256:%' THEN source.content_hash
				ELSE 'sha256:' || source.content_hash
			  END = typed.content_hash
			  AND source.start_line = typed.start_line
			  AND source.end_line = typed.end_line
			  AND source.pattern_id = COALESCE(typed.pattern_id, '')
		)
	`).Scan(&mismatches)
	if err != nil {
		return fmt.Errorf("verify FPF TypeEnv source join: %w", err)
	}
	if mismatches > 0 {
		return fmt.Errorf("FPF TypeEnv has %d source input(s) outside the exact Query snapshot", mismatches)
	}
	return nil
}

func ensureIndexMetadataTable(dbPath string) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open source index metadata db: %w", err)
	}
	_, execErr := db.Exec(`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT)`)
	closeErr := db.Close()
	if execErr != nil {
		return fmt.Errorf("create source index metadata table: %w", execErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close source index metadata db: %w", closeErr)
	}
	return nil
}

func buildSourceIndexMetadata(
	specPath string,
	readmePath string,
	sourceUnits int,
	explicitCommit string,
	buildTime time.Time,
) map[string]string {
	return map[string]string{
		"fpf_commit":           resolveSpecCommit(explicitCommit, specPath),
		"indexed_sections":     "0",
		"indexed_source_units": fmt.Sprintf("%d", sourceUnits),
		"build_time":           buildTime.UTC().Format(time.RFC3339),
		"spec_path":            filepath.Clean(specPath),
		"readme_path":          filepath.Clean(readmePath),
		"schema_version":       fpf.SpecIndexSchemaVersion,
	}
}

func resolveSpecCommit(explicitCommit, specPath string) string {
	commit := strings.TrimSpace(explicitCommit)
	if commit != "" {
		return commit
	}

	return detectSpecCommit(specPath)
}

// resolveSpecBuildTime returns the committer date of the FPF source commit, so
// rebuilding one revision is deterministic. Outside Git it uses the Unix epoch
// rather than wall-clock time.
func resolveSpecBuildTime(commitSHA, specPath string) time.Time {
	epoch := time.Unix(0, 0).UTC()
	gitDir, err := specGitLookupDir(specPath)
	if err != nil {
		return epoch
	}
	ref, ok := cleanSpecCommitRef(commitSHA)
	if !ok {
		return epoch
	}

	cmd := exec.Command("git", "show", "-s", "--format=%cI", ref)
	cmd.Dir = gitDir
	output, err := cmd.Output()
	if err != nil {
		return epoch
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(string(output)))
	if err != nil {
		return epoch
	}
	return parsed.UTC()
}

func cleanSpecCommitRef(commitSHA string) (string, bool) {
	ref := strings.TrimSpace(commitSHA)
	if ref == "" {
		return "HEAD", true
	}
	if len(ref) != 40 {
		return "", false
	}
	for _, runeValue := range ref {
		if !isHexCommitRune(runeValue) {
			return "", false
		}
	}
	return strings.ToLower(ref), true
}

func isHexCommitRune(value rune) bool {
	return (value >= '0' && value <= '9') ||
		(value >= 'a' && value <= 'f') ||
		(value >= 'A' && value <= 'F')
}

func detectSpecCommit(specPath string) string {
	gitDir, err := specGitLookupDir(specPath)
	if err != nil {
		return ""
	}

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = gitDir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func specGitLookupDir(specPath string) (string, error) {
	absPath, err := filepath.Abs(specPath)
	if err != nil {
		return "", err
	}
	return filepath.Dir(absPath), nil
}
