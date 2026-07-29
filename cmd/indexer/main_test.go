package main

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	_ "modernc.org/sqlite"
)

func TestBuildIndex_ContentOnlySourceChangeRebuildsWithoutRoutesOrVectors(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "first authored phrase")
	dbPath := filepath.Join(dir, "fpf.db")

	if err := buildIndex(specPath, dbPath, "revision-one"); err != nil {
		t.Fatalf("first buildIndex() error: %v", err)
	}
	firstDB := openTestDB(t, dbPath)
	firstSpecDigest, err := fpf.GetSpecMeta(firstDB, "spec_document_digest")
	if err != nil {
		t.Fatalf("read first spec document digest: %v", err)
	}
	firstTypeEnvRef, err := fpf.GetSpecMeta(firstDB, "typeenv_ref")
	if err != nil {
		t.Fatalf("read first TypeEnv ref: %v", err)
	}
	if err := firstDB.Close(); err != nil {
		t.Fatalf("close first source index: %v", err)
	}
	if !strings.HasPrefix(firstSpecDigest, "sha256:") {
		t.Fatalf("spec document digest = %q, want sha256 identity", firstSpecDigest)
	}
	assertTablesAbsent(
		t,
		dbPath,
		"routes",
		"fpf_embeddings",
		"pattern_use_route_embeddings",
		"pattern_use_intent_embeddings",
		"pattern_atlas_distillates",
	)
	first := inspectSourceUnit(t, dbPath, "TEST")
	directPattern := lookupSourceUnit(t, dbPath, "A.1")
	if directPattern.Role != fpf.SourceUnitRolePatternBody {
		t.Fatalf("exact PatternID resolved to %s, want pattern_body", directPattern.Role)
	}
	if !strings.Contains(directPattern.Body, "Solution") {
		t.Fatal("exact PatternID did not hydrate the full source pattern body")
	}

	initialDigest := fileDigest(t, dbPath)
	if err := buildIndex(specPath, dbPath, "revision-one"); err != nil {
		t.Fatalf("exact initial rebuild error: %v", err)
	}
	if rebuiltDigest := fileDigest(t, dbPath); rebuiltDigest != initialDigest {
		t.Fatal("exact initial rebuild changed the source-index bytes")
	}

	specPath = writeSourceFixture(t, dir, "second authored phrase")
	if err := buildIndex(specPath, dbPath, "revision-two"); err != nil {
		t.Fatalf("content-only rebuild error: %v", err)
	}
	secondDB := openTestDB(t, dbPath)
	secondSpecDigest, err := fpf.GetSpecMeta(secondDB, "spec_document_digest")
	if err != nil {
		t.Fatalf("read second spec document digest: %v", err)
	}
	var compatibilityKind string
	var compatibilityBase string
	err = secondDB.QueryRow(`
		SELECT assessment_kind, base_typeenv_ref
		FROM fpf_typeenv_compatibility
		WHERE singleton = 1
	`).Scan(&compatibilityKind, &compatibilityBase)
	if err != nil {
		t.Fatalf("read second TypeEnv compatibility assessment: %v", err)
	}
	if compatibilityKind != "compared" || compatibilityBase != firstTypeEnvRef {
		t.Fatalf(
			"second compatibility = %s against %q, want compared against %q",
			compatibilityKind,
			compatibilityBase,
			firstTypeEnvRef,
		)
	}
	if err := secondDB.Close(); err != nil {
		t.Fatalf("close second source index: %v", err)
	}
	if secondSpecDigest == firstSpecDigest {
		t.Fatal("content-only rebuild retained stale source-document digest")
	}
	second := inspectSourceUnit(t, dbPath, "TEST")

	if first.Provenance.ContentHash == second.Provenance.ContentHash {
		t.Fatalf("content-only rebuild retained stale hash %q", first.Provenance.ContentHash)
	}
	if second.Provenance.SourceRevision != "revision-two" {
		t.Fatalf("source revision = %q, want revision-two", second.Provenance.SourceRevision)
	}
	if err := verifyIndex([]string{dbPath, specPath, "revision-two"}); err != nil {
		t.Fatalf("verifyIndex() requires no route/intent/section vectors: %v", err)
	}
}

func TestBuildIndex_KnownLegacyTypeEnvStartsFreshCompatibility(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "legacy compiler predecessor")
	dbPath := filepath.Join(dir, "fpf.db")
	if err := buildIndex(specPath, dbPath, "legacy-source-revision"); err != nil {
		t.Fatalf("initial buildIndex() error: %v", err)
	}
	database := openTestDB(t, dbPath)
	_, err := database.Exec(
		`UPDATE fpf_typeenv_artifact
		 SET compiler_schema_version = ?
		 WHERE singleton = 1`,
		legacyBaseTypeEnvCompilerSchemaV1,
	)
	if err != nil {
		t.Fatalf("mark previous TypeEnv as known legacy schema: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close known legacy database: %v", err)
	}

	if err := buildIndex(specPath, dbPath, "current-source-revision"); err != nil {
		t.Fatalf("migrate known legacy TypeEnv: %v", err)
	}
	database = openTestDB(t, dbPath)
	defer func() { _ = database.Close() }()
	var compiler string
	var assessment string
	var base sql.NullString
	err = database.QueryRow(`
		SELECT artifact.compiler_schema_version,
		       compatibility.assessment_kind,
		       compatibility.base_typeenv_ref
		FROM fpf_typeenv_artifact AS artifact
		JOIN fpf_typeenv_compatibility AS compatibility
		  ON compatibility.artifact_digest = artifact.artifact_digest
		WHERE artifact.singleton = 1
	`).Scan(&compiler, &assessment, &base)
	if err != nil {
		t.Fatalf("read migrated TypeEnv envelope: %v", err)
	}
	if compiler == legacyBaseTypeEnvCompilerSchemaV1 {
		t.Fatalf("migrated TypeEnv retained legacy compiler schema %q", compiler)
	}
	if assessment != "initial" || base.Valid {
		t.Fatalf(
			"legacy migration compatibility = %q against %q, want initial without base",
			assessment,
			base.String,
		)
	}
}

func TestBuildIndex_KnownCompilerPredecessorsGetComparedIntoV5(t *testing.T) {
	tests := []struct {
		name     string
		compiler string
	}{
		{name: "v2", compiler: previousBaseTypeEnvCompilerSchemaV2},
		{name: "v3", compiler: previousBaseTypeEnvCompilerSchemaV3},
		{name: "v4", compiler: previousBaseTypeEnvCompilerSchemaV4},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertCompilerPredecessorComparedIntoCurrent(t, test.compiler)
		})
	}
}

func assertCompilerPredecessorComparedIntoCurrent(
	t *testing.T,
	predecessorCompiler string,
) {
	t.Helper()
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "known compiler predecessor")
	dbPath := filepath.Join(dir, "fpf.db")
	if err := buildIndex(specPath, dbPath, "predecessor-source-revision"); err != nil {
		t.Fatalf("initial buildIndex() error: %v", err)
	}
	currentEnvelope, exists, err := loadPreviousTypeEnvEnvelope(dbPath)
	if err != nil || !exists {
		t.Fatalf("load current TypeEnv: exists=%v err=%v", exists, err)
	}
	current := currentEnvelope.Artifact()
	predecessorVersion, err := typedmemory.NewCompilerSchemaVersion(predecessorCompiler)
	if err != nil {
		t.Fatalf("NewCompilerSchemaVersion(%s): %v", predecessorCompiler, err)
	}
	predecessorIR, err := typeenv.NewCompiledLinkedTypeEnvIR(
		current.SourceRevision(),
		predecessorVersion,
		current.CoverageManifest(),
		current.Declarations(),
	)
	if err != nil {
		t.Fatalf("NewCompiledLinkedTypeEnvIR(%s): %v", predecessorCompiler, err)
	}
	predecessorArtifact, err := typeenv.SealBaseTypeEnv(predecessorIR)
	if err != nil {
		t.Fatalf("SealBaseTypeEnv(%s): %v", predecessorCompiler, err)
	}
	predecessorCompatibility := typeenv.NewInitialCompatibilityAssessment()
	predecessorEnvelope, err := typeenv.NewCompilationEnvelope(
		predecessorArtifact,
		predecessorCompatibility,
	)
	if err != nil {
		t.Fatalf("NewCompilationEnvelope(%s): %v", predecessorCompiler, err)
	}
	if err := storeTypeEnvEnvelope(dbPath, predecessorEnvelope); err != nil {
		t.Fatalf("store %s predecessor: %v", predecessorCompiler, err)
	}
	predecessorRef, exists := predecessorArtifact.TypeEnvRef()
	if !exists {
		t.Fatal("compiler predecessor has no TypeEnvRef")
	}

	if err := buildIndex(specPath, dbPath, "v5-source-revision"); err != nil {
		t.Fatalf("buildIndex() across compiler %s -> v5: %v", predecessorCompiler, err)
	}
	database := openTestDB(t, dbPath)
	var compiler string
	var assessment string
	var base string
	err = database.QueryRow(`
		SELECT artifact.compiler_schema_version,
		       compatibility.assessment_kind,
		       compatibility.base_typeenv_ref
		FROM fpf_typeenv_artifact AS artifact
		JOIN fpf_typeenv_compatibility AS compatibility
		  ON compatibility.artifact_digest = artifact.artifact_digest
		WHERE artifact.singleton = 1
	`).Scan(&compiler, &assessment, &base)
	if err != nil {
		t.Fatalf("read v5 compatibility: %v", err)
	}
	if compiler == predecessorCompiler {
		t.Fatalf("compiler remained on predecessor %q", predecessorCompiler)
	}
	if assessment != "compared" || base != predecessorRef.String() {
		t.Fatalf(
			"v5 compatibility = %q against %q, want compared against %q",
			assessment,
			base,
			predecessorRef.String(),
		)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close compared V5 index: %v", err)
	}

	beforeExactRebuild := fileDigest(t, dbPath)
	if err := buildIndex(specPath, dbPath, "v5-source-revision"); err != nil {
		t.Fatalf("exact rebuild across compiler %s -> v5: %v", predecessorCompiler, err)
	}
	if afterExactRebuild := fileDigest(t, dbPath); afterExactRebuild != beforeExactRebuild {
		t.Fatalf(
			"exact rebuild changed compiler %s -> v5 compatibility bytes",
			predecessorCompiler,
		)
	}
}

func TestBuildIndex_ExactRebuildRejectsStoredSelfComparison(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "self-comparison guard")
	dbPath := filepath.Join(dir, "fpf.db")
	if err := buildIndex(specPath, dbPath, "self-comparison-revision"); err != nil {
		t.Fatalf("initial buildIndex() error: %v", err)
	}
	envelope, exists, err := loadPreviousTypeEnvEnvelope(dbPath)
	if err != nil || !exists {
		t.Fatalf("load current TypeEnv: exists=%v err=%v", exists, err)
	}
	artifact := envelope.Artifact()
	currentRef, hasCurrentRef := artifact.TypeEnvRef()
	if !hasCurrentRef {
		t.Fatal("current TypeEnv has no reference")
	}
	selfDiff, err := typedmemory.NewTypeEnvCompatibilityDiff(currentRef, nil)
	if err != nil {
		t.Fatalf("NewTypeEnvCompatibilityDiff(self): %v", err)
	}
	selfAssessment, err := typeenv.NewComparedCompatibilityAssessment(selfDiff)
	if err != nil {
		t.Fatalf("NewComparedCompatibilityAssessment(self): %v", err)
	}
	selfEnvelope, err := typeenv.NewCompilationEnvelope(artifact, selfAssessment)
	if err != nil {
		t.Fatalf("NewCompilationEnvelope(self): %v", err)
	}
	if err := storeTypeEnvEnvelope(dbPath, selfEnvelope); err != nil {
		t.Fatalf("store self-comparison envelope: %v", err)
	}
	before := fileDigest(t, dbPath)

	err = buildIndex(specPath, dbPath, "self-comparison-revision")
	if err == nil || !strings.Contains(err.Error(), "compares unchanged artifact") {
		t.Fatalf("self-comparison rebuild error = %v, want fail-closed diagnostic", err)
	}
	if after := fileDigest(t, dbPath); after != before {
		t.Fatal("rejected self-comparison rebuild changed the existing database")
	}
}

func TestBuildIndex_KnownLegacyReplacementFailurePreservesDatabase(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "stable legacy predecessor")
	dbPath := filepath.Join(dir, "fpf.db")
	if err := buildIndex(specPath, dbPath, "legacy-source-revision"); err != nil {
		t.Fatalf("initial buildIndex() error: %v", err)
	}
	database := openTestDB(t, dbPath)
	_, err := database.Exec(
		`UPDATE fpf_typeenv_artifact
		 SET compiler_schema_version = ?
		 WHERE singleton = 1`,
		legacyBaseTypeEnvCompilerSchemaV1,
	)
	if err != nil {
		t.Fatalf("mark previous TypeEnv as known legacy schema: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close known legacy database: %v", err)
	}
	before := fileDigest(t, dbPath)

	originalVerify := verifyBuiltIndexFunc
	verifyBuiltIndexFunc = func(
		*sql.DB,
		fpf.PublicationSnapshot,
		typeenv.BaseTypeEnvArtifact,
	) error {
		return errors.New("injected legacy replacement verification failure")
	}
	t.Cleanup(func() {
		verifyBuiltIndexFunc = originalVerify
	})

	err = buildIndex(specPath, dbPath, "replacement-source-revision")
	if err == nil || !strings.Contains(err.Error(), "injected legacy replacement verification failure") {
		t.Fatalf("expected injected legacy replacement failure, got %v", err)
	}
	after := fileDigest(t, dbPath)
	if before != after {
		t.Fatal("failed legacy replacement changed the previous database")
	}
}

func TestBuildIndex_UnknownPreviousCompilerSchemaFailsClosed(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "unknown compiler predecessor")
	dbPath := filepath.Join(dir, "fpf.db")
	if err := buildIndex(specPath, dbPath, "stable-source-revision"); err != nil {
		t.Fatalf("initial buildIndex() error: %v", err)
	}
	database := openTestDB(t, dbPath)
	_, err := database.Exec(`
		UPDATE fpf_typeenv_artifact
		SET compiler_schema_version = 'fpf-base-typeenv.cov2.unknown'
		WHERE singleton = 1
	`)
	if err != nil {
		t.Fatalf("set unknown previous compiler schema: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close unknown-schema database: %v", err)
	}
	before := fileDigest(t, dbPath)

	err = buildIndex(specPath, dbPath, "replacement-source-revision")
	if err == nil || !strings.Contains(err.Error(), "neither current") {
		t.Fatalf("expected unknown previous compiler failure, got %v", err)
	}
	after := fileDigest(t, dbPath)
	if before != after {
		t.Fatal("unknown previous compiler failure changed the database")
	}
}

func TestBuildIndex_CurrentCompilerCorruptionFailsClosed(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "current compiler corruption")
	dbPath := filepath.Join(dir, "fpf.db")
	if err := buildIndex(specPath, dbPath, "stable-source-revision"); err != nil {
		t.Fatalf("initial buildIndex() error: %v", err)
	}
	database := openTestDB(t, dbPath)
	_, err := database.Exec(`
		UPDATE fpf_typeenv_artifact
		SET canonical_bytes = X'00'
		WHERE singleton = 1
	`)
	if err != nil {
		t.Fatalf("corrupt current compiler artifact: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close corrupted current database: %v", err)
	}
	before := fileDigest(t, dbPath)

	err = buildIndex(specPath, dbPath, "replacement-source-revision")
	if err == nil || !strings.Contains(err.Error(), "load previous FPF TypeEnv envelope read-only") {
		t.Fatalf("expected current compiler corruption failure, got %v", err)
	}
	after := fileDigest(t, dbPath)
	if before != after {
		t.Fatal("current compiler corruption failure changed the database")
	}
}

func TestComparePreviousTypeEnv_PathReplacementKeepsOneOpenedSnapshot(t *testing.T) {
	dir := t.TempDir()
	current, currentDBPath := buildCurrentTypeEnvArtifact(t, dir)
	dbPath := filepath.Join(dir, "predecessor.db")
	replacementPath := filepath.Join(dir, "replacement.db")
	archivedPath := filepath.Join(dir, "opened-predecessor.db")
	knownPredecessor := storeTypeEnvCompilerVariant(
		t,
		dbPath,
		currentDBPath,
		current,
		previousBaseTypeEnvCompilerSchemaV4,
	)
	unknownPredecessor := storeTypeEnvCompilerVariant(
		t,
		replacementPath,
		currentDBPath,
		current,
		"fpf-base-typeenv.cov2.unknown",
	)

	originalHook := afterPreviousTypeEnvCompilerProbeFunc
	hookCalls := 0
	afterPreviousTypeEnvCompilerProbeFunc = func(probedPath string) error {
		hookCalls++
		if probedPath != dbPath {
			return fmt.Errorf("compiler probe path = %q, want %q", probedPath, dbPath)
		}
		return replaceDatabasePath(dbPath, replacementPath, archivedPath)
	}
	t.Cleanup(func() {
		afterPreviousTypeEnvCompilerProbeFunc = originalHook
	})

	assessment, err := comparePreviousTypeEnv(dbPath, current)
	if err != nil {
		t.Fatalf("compare predecessor across path replacement: %v", err)
	}
	if hookCalls != 1 {
		t.Fatalf("compiler-probe replacement seam called %d times, want once", hookCalls)
	}
	compared, ok := assessment.(typeenv.ComparedCompatibilityAssessment)
	if !ok {
		t.Fatalf("compatibility = %T, want compared assessment from opened predecessor", assessment)
	}
	knownRef, hasKnownRef := knownPredecessor.TypeEnvRef()
	if !hasKnownRef {
		t.Fatal("known predecessor has no TypeEnvRef")
	}
	unknownRef, hasUnknownRef := unknownPredecessor.TypeEnvRef()
	if !hasUnknownRef {
		t.Fatal("unknown predecessor has no TypeEnvRef")
	}
	if base := compared.Diff().Base(); base != knownRef {
		t.Fatalf(
			"comparison base = %q, want opened predecessor %q (replacement was %q)",
			base.String(),
			knownRef.String(),
			unknownRef.String(),
		)
	}
}

func TestComparePreviousTypeEnv_PathReplacementCannotHideUnknownCompiler(t *testing.T) {
	dir := t.TempDir()
	current, currentDBPath := buildCurrentTypeEnvArtifact(t, dir)
	dbPath := filepath.Join(dir, "predecessor.db")
	replacementPath := filepath.Join(dir, "replacement.db")
	archivedPath := filepath.Join(dir, "opened-predecessor.db")
	storeTypeEnvCompilerVariant(
		t,
		dbPath,
		currentDBPath,
		current,
		"fpf-base-typeenv.cov2.unknown",
	)
	storeTypeEnvCompilerVariant(
		t,
		replacementPath,
		currentDBPath,
		current,
		previousBaseTypeEnvCompilerSchemaV4,
	)

	originalHook := afterPreviousTypeEnvCompilerProbeFunc
	hookCalls := 0
	afterPreviousTypeEnvCompilerProbeFunc = func(string) error {
		hookCalls++
		return replaceDatabasePath(dbPath, replacementPath, archivedPath)
	}
	t.Cleanup(func() {
		afterPreviousTypeEnvCompilerProbeFunc = originalHook
	})

	_, err := comparePreviousTypeEnv(dbPath, current)
	if err == nil || !strings.Contains(err.Error(), "neither current") {
		t.Fatalf("unknown compiler comparison error = %v, want fail-closed diagnostic", err)
	}
	if hookCalls != 1 {
		t.Fatalf("compiler-probe replacement seam called %d times, want once", hookCalls)
	}
}

func TestComparePreviousTypeEnv_ClosesSnapshotOnceAndKeepsCloseError(t *testing.T) {
	dir := t.TempDir()
	current, currentDBPath := buildCurrentTypeEnvArtifact(t, dir)
	dbPath := filepath.Join(dir, "predecessor.db")
	storeTypeEnvCompilerVariant(
		t,
		dbPath,
		currentDBPath,
		current,
		"fpf-base-typeenv.cov2.unknown",
	)

	originalClose := closePreviousTypeEnvSnapshotFunc
	closeCalls := 0
	injectedCloseErr := errors.New("injected predecessor snapshot close failure")
	closePreviousTypeEnvSnapshotFunc = func(database *sql.DB) error {
		closeCalls++
		return errors.Join(database.Close(), injectedCloseErr)
	}
	t.Cleanup(func() {
		closePreviousTypeEnvSnapshotFunc = originalClose
	})

	_, err := comparePreviousTypeEnv(dbPath, current)
	if err == nil ||
		!strings.Contains(err.Error(), "neither current") ||
		!strings.Contains(err.Error(), injectedCloseErr.Error()) {
		t.Fatalf("comparison error = %v, want compiler and close diagnostics", err)
	}
	if closeCalls != 1 {
		t.Fatalf("predecessor snapshot closed %d times, want exactly once", closeCalls)
	}
}

func TestBuildIndex_VerifiesExactSharedPublicationSnapshot(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "shared snapshot source")
	readmePath := filepath.Join(dir, "Readme.md")
	dbPath := filepath.Join(dir, "fpf.db")
	if err := buildIndex(specPath, dbPath, "snapshot-revision"); err != nil {
		t.Fatalf("buildIndex() error: %v", err)
	}
	snapshot, err := fpf.LoadPublicationSnapshot(readmePath, specPath, "snapshot-revision")
	if err != nil {
		t.Fatalf("LoadPublicationSnapshot() error: %v", err)
	}
	db := openTestDB(t, dbPath)
	if err := fpf.VerifyPublicationSnapshotDB(db, snapshot); err != nil {
		t.Fatalf("VerifyPublicationSnapshotDB() error: %v", err)
	}
	if _, err := db.Exec(
		`UPDATE meta SET value = 'sha256:stale' WHERE key = 'spec_document_digest'`,
	); err != nil {
		t.Fatalf("corrupt spec document digest: %v", err)
	}
	err = fpf.VerifyPublicationSnapshotDB(db, snapshot)
	if err == nil || !strings.Contains(err.Error(), "spec_document_digest") {
		t.Fatalf("snapshot metadata mismatch error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close source index: %v", err)
	}
}

func TestVerifyIndexRejectsDirtyCheckedOutSourceUnderCleanClaimedRevision(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "committed source phrase")
	dbPath := filepath.Join(dir, "fpf.db")
	if err := buildIndex(specPath, dbPath, "clean-claimed-revision"); err != nil {
		t.Fatalf("buildIndex() error: %v", err)
	}

	writeSourceFixture(t, dir, "dirty uncommitted source phrase")
	err := verifyIndex([]string{dbPath, specPath, "clean-claimed-revision"})
	if err == nil || !strings.Contains(err.Error(), "publication snapshot") {
		t.Fatalf("dirty checked-out source passed clean claimed revision: %v", err)
	}
}

func TestVerifyIndexLegacyDatabaseIsReadOnly(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "legacy read-only source")
	dbPath := filepath.Join(dir, "legacy.db")
	writeMinimalMetadataDatabase(
		t,
		dbPath,
		map[string]string{
			"fpf_commit":     "source-revision",
			"schema_version": "10",
		},
	)
	assertFailedVerificationDoesNotMutate(
		t,
		dbPath,
		func() error {
			return verifyIndex([]string{dbPath, specPath, "source-revision"})
		},
		`code expects "11"`,
	)
}

func TestVerifyIndexMalformedSchema11DatabaseIsReadOnly(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "malformed read-only source")
	dbPath := filepath.Join(dir, "malformed.db")
	writeMinimalMetadataDatabase(
		t,
		dbPath,
		map[string]string{
			"fpf_commit":     "source-revision",
			"schema_version": "11",
		},
	)
	database := openTestDB(t, dbPath)
	if _, err := database.Exec(`CREATE TABLE source_units (unit_id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create malformed source table: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close malformed source database: %v", err)
	}
	assertFailedVerificationDoesNotMutate(
		t,
		dbPath,
		func() error {
			return verifyIndex([]string{dbPath, specPath, "source-revision"})
		},
		"verify checked-out FPF publication snapshot",
	)
}

func TestBuildIndex_CallbackFailurePreservesPreviousGoodDatabase(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "stable source")
	dbPath := filepath.Join(dir, "fpf.db")

	if err := buildIndex(specPath, dbPath, "stable-revision"); err != nil {
		t.Fatalf("initial buildIndex() error: %v", err)
	}
	before := fileDigest(t, dbPath)

	originalStore := storeSourceUnitsFunc
	storeSourceUnitsFunc = func(string, []fpf.SourceUnit) error {
		return errors.New("injected source-store failure")
	}
	t.Cleanup(func() {
		storeSourceUnitsFunc = originalStore
	})

	err := buildIndex(specPath, dbPath, "replacement-revision")
	if err == nil || !strings.Contains(err.Error(), "injected source-store failure") {
		t.Fatalf("expected injected callback error, got %v", err)
	}
	after := fileDigest(t, dbPath)
	if before != after {
		t.Fatal("failed atomic rebuild changed the previous good database")
	}
	if err := verifyIndex([]string{dbPath, specPath, "stable-revision"}); err != nil {
		t.Fatalf("previous database no longer verifies: %v", err)
	}
}

func TestBuildIndex_CurrentC3GrammarMutationPreservesPreviousGoodDatabase(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "stable source")
	dbPath := filepath.Join(dir, "fpf.db")

	if err := buildIndex(specPath, dbPath, "stable-revision"); err != nil {
		t.Fatalf("initial buildIndex() error: %v", err)
	}
	before := fileDigest(t, dbPath)

	spec, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read source fixture: %v", err)
	}
	broken := strings.Replace(
		string(spec),
		"Keep a partial order over obtaining facts.",
		"Keep an unspecified ordering over obtaining facts.",
		1,
	)
	if err := os.WriteFile(specPath, []byte(broken), 0o644); err != nil {
		t.Fatalf("write broken source fixture: %v", err)
	}

	err = buildIndex(specPath, dbPath, "replacement-revision")
	if err == nil || !strings.Contains(err.Error(), "current_c3_contract_malformed") {
		t.Fatalf("expected fail-loud current C.3 grammar error, got %v", err)
	}
	after := fileDigest(t, dbPath)
	if before != after {
		t.Fatal("malformed current C.3 source changed the previous good database")
	}
}

func TestVerifyIndexRejectsBrokenSourceProvenance(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "provenance source")
	dbPath := filepath.Join(dir, "fpf.db")

	if err := buildIndex(specPath, dbPath, "source-revision"); err != nil {
		t.Fatalf("buildIndex() error: %v", err)
	}
	db := openTestDB(t, dbPath)
	if _, err := db.Exec(
		`UPDATE source_units SET content_hash = 'stale' WHERE unit_id = 'readme:practical_use_card:test'`,
	); err != nil {
		t.Fatalf("corrupt source provenance: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close corrupted db: %v", err)
	}

	err := verifyIndex([]string{dbPath, specPath, "source-revision"})
	if err == nil || !strings.Contains(err.Error(), "content hash mismatch") {
		t.Fatalf("expected provenance verification error, got %v", err)
	}
}

func TestVerifyIndexRejectsConsistentlyDeletedSourceUnit(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "completeness source")
	dbPath := filepath.Join(dir, "fpf.db")

	if err := buildIndex(specPath, dbPath, "source-revision"); err != nil {
		t.Fatalf("buildIndex() error: %v", err)
	}
	db := openTestDB(t, dbPath)
	var unitID string
	err := db.
		QueryRow(
			`SELECT unit_id FROM source_units WHERE source_role = 'pattern_section' ORDER BY unit_id LIMIT 1`,
		).
		Scan(&unitID)
	if err != nil {
		t.Fatalf("select removable source unit: %v", err)
	}
	for _, statement := range []string{
		`DELETE FROM source_units_fts WHERE unit_id = ?`,
		`DELETE FROM source_unit_refs WHERE unit_id = ?`,
		`DELETE FROM source_authored_phrases WHERE unit_id = ?`,
		`DELETE FROM source_keywords WHERE unit_id = ?`,
		`DELETE FROM source_units WHERE unit_id = ?`,
	} {
		if _, err := db.Exec(statement, unitID); err != nil {
			t.Fatalf("delete source unit %s with %q: %v", unitID, statement, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close incomplete db: %v", err)
	}

	err = verifyIndex([]string{dbPath, specPath, "source-revision"})
	if err == nil || !strings.Contains(err.Error(), "publication snapshot has") {
		t.Fatalf("expected completeness error, got %v", err)
	}
}

func TestVerifyIndexRejectsStaleSourceUnitRevision(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "revision source")
	dbPath := filepath.Join(dir, "fpf.db")

	if err := buildIndex(specPath, dbPath, "source-revision"); err != nil {
		t.Fatalf("buildIndex() error: %v", err)
	}
	db := openTestDB(t, dbPath)
	if _, err := db.Exec(
		`UPDATE source_units SET source_revision = 'older-revision' WHERE unit_id = 'readme:practical_use_card:test'`,
	); err != nil {
		t.Fatalf("stale source revision: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close stale-revision db: %v", err)
	}

	err := verifyIndex([]string{dbPath, specPath, "source-revision"})
	if err == nil || !strings.Contains(err.Error(), "differs from publication snapshot") {
		t.Fatalf("expected source revision error, got %v", err)
	}
}

func TestVerifyIndexRejectsCommitAndSchemaMismatch(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "metadata source")
	dbPath := filepath.Join(dir, "fpf.db")

	if err := buildIndex(specPath, dbPath, "source-revision"); err != nil {
		t.Fatalf("buildIndex() error: %v", err)
	}
	if err := verifyIndex([]string{dbPath, specPath, "different-revision"}); err == nil || !strings.Contains(err.Error(), "STALE") {
		t.Fatalf("expected commit mismatch, got %v", err)
	}

	db := openTestDB(t, dbPath)
	if _, err := db.Exec(`UPDATE meta SET value = '9' WHERE key = 'schema_version'`); err != nil {
		t.Fatalf("corrupt schema metadata: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close schema db: %v", err)
	}
	if err := verifyIndex([]string{dbPath, specPath, "source-revision"}); err == nil || !strings.Contains(err.Error(), `code expects "11"`) {
		t.Fatalf("expected v9 cache without source relations to fail closed, got %v", err)
	}
	if err := buildIndex(specPath, dbPath, "source-revision"); err != nil {
		t.Fatalf("rebuild schema-9 cache into schema 11: %v", err)
	}
	if err := verifyIndex([]string{dbPath, specPath, "source-revision"}); err != nil {
		t.Fatalf("rebuilt schema-11 cache did not verify: %v", err)
	}
}

func TestVerifyIndexRejectsTypeEnvMetadataMismatch(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "TypeEnv metadata source")
	dbPath := filepath.Join(dir, "fpf.db")
	if err := buildIndex(specPath, dbPath, "source-revision"); err != nil {
		t.Fatalf("buildIndex() error: %v", err)
	}
	database := openTestDB(t, dbPath)
	if _, err := database.Exec(`
		UPDATE meta
		SET value = 'sha256:stale'
		WHERE key = 'typeenv_artifact_digest'
	`); err != nil {
		t.Fatalf("corrupt TypeEnv metadata: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close corrupted TypeEnv metadata db: %v", err)
	}

	err := verifyIndex([]string{dbPath, specPath, "source-revision"})
	if err == nil || !strings.Contains(err.Error(), "typeenv_artifact_digest") {
		t.Fatalf("expected TypeEnv metadata verification error, got %v", err)
	}
}

func TestTypeEnvSourceJoinRejectsDifferentQuerySnapshotRow(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "TypeEnv join source")
	dbPath := filepath.Join(dir, "fpf.db")
	if err := buildIndex(specPath, dbPath, "source-revision"); err != nil {
		t.Fatalf("buildIndex() error: %v", err)
	}
	database := openTestDB(t, dbPath)
	if _, err := database.Exec(`
		UPDATE fpf_typeenv_sources
		SET content_hash = 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
		WHERE rowid = (SELECT rowid FROM fpf_typeenv_sources ORDER BY rowid LIMIT 1)
	`); err != nil {
		t.Fatalf("corrupt TypeEnv source join: %v", err)
	}
	err := verifyTypeEnvSourceJoin(database)
	if err == nil || !strings.Contains(err.Error(), "outside the exact Query snapshot") {
		t.Fatalf("expected exact snapshot join error, got %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close corrupted TypeEnv source db: %v", err)
	}
}

func TestBuildIndex_CompilerFailurePreservesPreviousGoodDatabase(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "stable compiler source")
	dbPath := filepath.Join(dir, "fpf.db")
	if err := buildIndex(specPath, dbPath, "stable-revision"); err != nil {
		t.Fatalf("initial buildIndex() error: %v", err)
	}
	before := fileDigest(t, dbPath)

	originalCompile := compileBaseTypeEnvFunc
	compileBaseTypeEnvFunc = func(fpf.PublicationSnapshot) (typeenv.BaseTypeEnvCompilation, error) {
		return nil, errors.New("injected TypeEnv compiler failure")
	}
	t.Cleanup(func() {
		compileBaseTypeEnvFunc = originalCompile
	})

	err := buildIndex(specPath, dbPath, "replacement-revision")
	if err == nil || !strings.Contains(err.Error(), "injected TypeEnv compiler failure") {
		t.Fatalf("expected injected compiler error, got %v", err)
	}
	after := fileDigest(t, dbPath)
	if before != after {
		t.Fatal("failed TypeEnv compilation changed the previous good database")
	}
}

func TestBuildIndex_CompilerRejectionPreservesPreviousGoodDatabase(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "stable grammar source")
	dbPath := filepath.Join(dir, "fpf.db")
	if err := buildIndex(specPath, dbPath, "stable-revision"); err != nil {
		t.Fatalf("initial buildIndex() error: %v", err)
	}
	before := fileDigest(t, dbPath)

	specification, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read source fixture: %v", err)
	}
	broken := strings.Replace(
		string(specification),
		"SlotSpec := <SlotKind, ValueKind, refMode>",
		"SlotSpec := <SlotKind, ValueKind>",
		1,
	)
	if err := os.WriteFile(specPath, []byte(broken), 0o644); err != nil {
		t.Fatalf("write rejected source fixture: %v", err)
	}

	err = buildIndex(specPath, dbPath, "replacement-revision")
	if err == nil || !strings.Contains(err.Error(), "rejected publication") {
		t.Fatalf("expected source compiler rejection, got %v", err)
	}
	after := fileDigest(t, dbPath)
	if before != after {
		t.Fatal("rejected source compilation changed the previous good database")
	}
}

func TestBuildIndex_TypeEnvStoreFailurePreservesPreviousGoodDatabase(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "stable TypeEnv store source")
	dbPath := filepath.Join(dir, "fpf.db")
	if err := buildIndex(specPath, dbPath, "stable-revision"); err != nil {
		t.Fatalf("initial buildIndex() error: %v", err)
	}
	before := fileDigest(t, dbPath)

	originalStore := storeTypeEnvEnvelopeFunc
	storeTypeEnvEnvelopeFunc = func(string, typeenv.CompilationEnvelope) error {
		return errors.New("injected TypeEnv store failure")
	}
	t.Cleanup(func() {
		storeTypeEnvEnvelopeFunc = originalStore
	})

	err := buildIndex(specPath, dbPath, "replacement-revision")
	if err == nil || !strings.Contains(err.Error(), "injected TypeEnv store failure") {
		t.Fatalf("expected injected TypeEnv store error, got %v", err)
	}
	after := fileDigest(t, dbPath)
	if before != after {
		t.Fatal("failed TypeEnv persistence changed the previous good database")
	}
}

func TestBuildIndex_FinalVerificationFailurePreservesPreviousGoodDatabase(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "stable final verification source")
	dbPath := filepath.Join(dir, "fpf.db")
	if err := buildIndex(specPath, dbPath, "stable-revision"); err != nil {
		t.Fatalf("initial buildIndex() error: %v", err)
	}
	before := fileDigest(t, dbPath)

	originalVerify := verifyBuiltIndexFunc
	verifyBuiltIndexFunc = func(
		*sql.DB,
		fpf.PublicationSnapshot,
		typeenv.BaseTypeEnvArtifact,
	) error {
		return errors.New("injected combined final verification failure")
	}
	t.Cleanup(func() {
		verifyBuiltIndexFunc = originalVerify
	})

	err := buildIndex(specPath, dbPath, "replacement-revision")
	if err == nil || !strings.Contains(err.Error(), "injected combined final verification failure") {
		t.Fatalf("expected injected final verifier error, got %v", err)
	}
	after := fileDigest(t, dbPath)
	if before != after {
		t.Fatal("failed combined verification changed the previous good database")
	}
}

func TestLoadPreviousTypeEnvIsReadOnly(t *testing.T) {
	dir := t.TempDir()
	specPath := writeSourceFixture(t, dir, "read-only probe source")
	dbPath := filepath.Join(dir, "fpf.db")
	if err := buildIndex(specPath, dbPath, "source-revision"); err != nil {
		t.Fatalf("buildIndex() error: %v", err)
	}
	before := fileDigest(t, dbPath)
	envelope, exists, err := loadPreviousTypeEnvEnvelope(dbPath)
	if err != nil {
		t.Fatalf("loadPreviousTypeEnvEnvelope() error: %v", err)
	}
	if !exists {
		t.Fatal("read-only prior probe did not find TypeEnv")
	}
	artifact := envelope.Artifact()
	if _, hasReference := artifact.TypeEnvRef(); !hasReference {
		t.Fatal("read-only prior probe returned artifact without TypeEnvRef")
	}
	after := fileDigest(t, dbPath)
	if before != after {
		t.Fatal("read-only prior TypeEnv probe changed the database")
	}
}

func TestResolveSpecCommit(t *testing.T) {
	specPath := filepath.Join(t.TempDir(), "FPF-Spec.md")
	tests := []struct {
		name           string
		explicitCommit string
		want           string
	}{
		{name: "empty", explicitCommit: "", want: ""},
		{name: "trimmed", explicitCommit: "  abc123  ", want: "abc123"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := resolveSpecCommit(test.explicitCommit, specPath)
			if got != test.want {
				t.Fatalf("resolveSpecCommit(%q) = %q, want %q", test.explicitCommit, got, test.want)
			}
		})
	}
}

func TestResolveSpecCommit_DetectsGitCommitFromSpecPath(t *testing.T) {
	repoDir := t.TempDir()
	specDir := filepath.Join(repoDir, "data", "FPF")
	specPath := filepath.Join(specDir, "FPF-Spec.md")

	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(specPath, []byte("# spec\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "init")

	want := strings.TrimSpace(runGit(t, repoDir, "rev-parse", "HEAD"))
	got := resolveSpecCommit("", specPath)
	if got != want {
		t.Fatalf("resolveSpecCommit() = %q, want %q", got, want)
	}
}

func TestBuildSourceIndexMetadata_RecordsBothSourceCarriers(t *testing.T) {
	buildTime := time.Date(2026, time.March, 26, 12, 34, 56, 0, time.UTC)
	dir := t.TempDir()
	specPath := filepath.Join(dir, "FPF-Spec.md")
	readmePath := filepath.Join(dir, "Readme.md")
	metadata := buildSourceIndexMetadata(specPath, readmePath, 77, "revision", buildTime)

	if metadata["fpf_commit"] != "revision" {
		t.Fatalf("fpf_commit = %q", metadata["fpf_commit"])
	}
	if metadata["indexed_sections"] != "0" || metadata["indexed_source_units"] != "77" {
		t.Fatalf("unexpected counts: %#v", metadata)
	}
	if metadata["spec_path"] != specPath || metadata["readme_path"] != readmePath {
		t.Fatalf("source carrier paths missing: %#v", metadata)
	}
}

func TestCleanSpecCommitRef(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{name: "empty defaults to HEAD", in: "", want: "HEAD", ok: true},
		{name: "lowercase sha", in: "0123456789abcdef0123456789abcdef01234567", want: "0123456789abcdef0123456789abcdef01234567", ok: true},
		{name: "uppercase sha normalizes", in: "ABCDEF0123456789ABCDEF0123456789ABCDEF01", want: "abcdef0123456789abcdef0123456789abcdef01", ok: true},
		{name: "option injection rejected", in: "--format=%H", ok: false},
		{name: "short ref rejected", in: "abc123", ok: false},
		{name: "pathspec rejected", in: "HEAD:cmd/indexer/main.go", ok: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := cleanSpecCommitRef(test.in)
			if got != test.want || ok != test.ok {
				t.Fatalf("cleanSpecCommitRef(%q) = %q, %v; want %q, %v", test.in, got, ok, test.want, test.ok)
			}
		})
	}
}

func writeSourceFixture(t *testing.T, dir, authoredPhrase string) string {
	t.Helper()
	readmeBody := `
## Practical-Use Cards

### TEST - Find a source-owned candidate

- **Situation and question.** The project needs a source-native candidate without a hidden route.
- Ask: ` + authoredPhrase + `
- **Template 1. Solution ->** Inspect A.1 and return one exact source unit.
- **Boundaries.** Stop after the exact source unit is inspected.
`
	readme := "# First Principles Framework (FPF) - Core Conceptual Specification\n" + readmeBody
	spec := "# First Principles Framework (FPF) Readme\n" + readmeBody + `

# Preface - One Connected Framework

## Why the publication roles stay distinct

The README, preface, table of contents, and full pattern answer different source-navigation questions.

# Table of Content

| Pattern ID | Name | Status | Search vocabulary | Dependencies |
| --- | --- | --- | --- | --- |
| A.1 | A.1 - Alpha Pattern | active | Keywords: alpha, source. Queries: "find alpha source" | |
| A.6.5 | A.6.5 - Relation Slot Discipline | active | Keywords: slot, type. Queries: "compile slot grammar" | |
| C.2.1 | C.2.1 - Episteme Slot Relation | active | Keywords: episteme, relation. Queries: "compile episteme relation" | A.6.5 |
| C.3.1 | C.3.1 - Kind Core | active | Keywords: kind, subkind. Queries: "compile kind order" | |
| C.3.2 | C.3.2 - Kind Classification | active | Keywords: kind, classification. Queries: "compile kind classification" | C.3.1 |
| C.3.3 | C.3.3 - Kind Bridge | active | Keywords: kind, bridge. Queries: "compile kind bridge" | C.3.2 |
| C.3.4 | C.3.4 - Role Mask | active | Keywords: kind, mask. Queries: "compile role mask" | C.3.2 |
| C.3.A | C.3.A - Kind Guards | active | Keywords: kind, guard. Queries: "compile kind guards" | C.3.2 |

# Pattern Language

## A.1 - Alpha Pattern

This source-owned pattern body is intentionally complete enough to remain an indexed pattern carrier.

### A.1:1 - Problem

The current concern needs one exact pattern body with stable provenance and no inferred project order.

### A.1:2 - Solution

Inspect the full source pattern and decide applicability from its stated condition and result kind.
` + structuralTypeEnvFixture()

	readmePath := filepath.Join(dir, "Readme.md")
	specPath := filepath.Join(dir, "FPF-Spec.md")
	if err := os.WriteFile(readmePath, []byte(readme), 0o644); err != nil {
		t.Fatalf("write Readme.md: %v", err)
	}
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write FPF-Spec.md: %v", err)
	}
	return specPath
}

func structuralTypeEnvFixture() string {
	return `

## A.6.5 - Relation Slot Discipline

### A.6.5:4 - Solution

#### A.6.5:4.2 - Declare one complete SlotSpec for each relation-participant meaning needed by typed reuse

The following code block represents the exact current SlotSpec grammar.

` + "```text" + `
SlotSpec := <SlotKind, ValueKind, refMode>
refMode := ByValue | RefKind
` + "```" + `

#### A.6.5:4.3 - Apply the well-formedness constraints

` + "```text" + `
A6.5-S1 CompleteSlotSpec:
  every relation-participant meaning needed by reusable typed use has one SlotSpec
  with exactly one SlotKind, one ValueKind, and one refMode.

A6.5-S2 LocalSlotKind:
  SlotKind is interpreted only inside the exact RelationSignature that
  contains the corresponding SlotSpec.

A6.5-S3 ExactParticipantKind:
  each actual participant corresponding to the declared relation-participant meaning
  has the declared ValueKind.

A6.5-S4 HonestReference:
  when refMode is a RefKind, the receiving assertion or description carries
  a reference of that RefKind whose resolution denotes a participant.

A6.5-S5 DirectPredicateGovernance:
  the direct governing pattern contains statements of the relation predicate,
  applicability, and any relation occurrence-identity rule.

A6.5-S6 NoHiddenUnion:
  one ValueKind does not hide participant kinds for which the direct
  predicate has different semantics.

A6.5-S7 RepresentationBoundary:
  a representation or publication form does not become the
  world-side participant or relation occurrence by form.
` + "```" + `

## C.2.1 - Episteme Constitution and Direct Relations

### C.2.1:4 - Solution

#### C.2.1:4.1 - Identify the episteme by its constitution

` + "```text" + `
<claim content, exact EntityOfConcern, effective ReferenceScheme>
` + "```" + `

#### C.2.1:4.2 - Govern the core direct relation

**Tech name:** ` + "`EpistemeConstitutionRelation`" + `.

##### C.2.1:4.2.1 - Participants and the shared reusable-declaration rule

Each declaration is one exact C.2.1 episteme before its ` + "`U.Signature`" + ` membership is recognized.

Applying that shared rule locally, typed reuse of ` + "`EpistemeConstitutionRelation`" + ` uses the one declaration episteme ` + "`EpistemeConstitutionRelationSignature`" + `, whose exact EntityOfConcern is ` + "`EpistemeConstitutionRelation`" + ` and whose declaration includes these SlotSpecs:

| SlotKind | Relation-participant meaning | ValueKind | refMode |
| --- | --- | --- | --- |
| ` + "`ClaimGraphSlot`" + ` | constitutive claim content | ` + "`U.ClaimGraph`" + ` | ` + "`ByValue`" + ` |
| ` + "`EntityOfConcernSlot`" + ` | exact entity the claims concern | ` + "`U.Entity`" + ` | ` + "`U.EntityRef`" + ` |
| ` + "`ReferenceSchemeSlot`" + ` | effective designation and interpretation scheme | ` + "`U.ReferenceScheme`" + ` | ` + "`ByValue`" + ` |

##### C.2.1:4.2.2 - Obtaining and occurrence identity

` + "`EpistemeConstitutionRelation`" + ` obtains exactly when the effective reference scheme coherently interprets the claims about the exact entity.

The relation occurrence is participant-determined by the exact claim graph, EntityOfConcern, and ReferenceScheme triple.

#### C.2.1:4.3 - Add empirical grounding through its own relation

**Tech name:** ` + "`EpistemeEmpiricalGroundingRelation`" + `.

Applying the shared declaration rule in 4.2.1, ` + "`EpistemeEmpiricalGroundingRelationSignature`" + ` is one declaration episteme whose exact EntityOfConcern is ` + "`EpistemeEmpiricalGroundingRelation`" + `; its declaration includes these SlotSpecs:

| SlotKind | Relation-participant meaning | ValueKind | refMode |
| --- | --- | --- | --- |
| ` + "`GroundedEpistemeSlot`" + ` | episteme containing the exact covered claim subgraph | ` + "`U.Episteme`" + ` | ` + "`U.EpistemeRef`" + ` |
| ` + "`GroundingHolonSlot`" + ` | exact holon involved in mapped observation, intervention, measurement, or test relations | ` + "`U.Holon`" + ` | ` + "`U.HolonRef`" + ` |

` + "`EpistemeEmpiricalGroundingRelation`" + ` over participants ` + "`(E,H)`" + `, with ` + "`covered=C`" + `, obtains exactly while every empirical claim in the exact covered claim subgraph has a current claim-to-world mapping.

One occurrence is identified by ` + "`<episteme, exact covered claim subgraph, grounding holon, maximal continuous interval during which the complete coverage predicate is true>`" + `.

#### C.2.1:4.5 - Relate distinct episteme editions explicitly

**Tech name:** ` + "`EpistemeEditionRelation`" + `.

` + "`EpistemeEditionRelation`" + ` has exactly two direct participants. ` + "`EpistemeEditionRelationSignature`" + ` is one declaration episteme whose exact EntityOfConcern is ` + "`EpistemeEditionRelation`" + ` and whose declaration includes these SlotSpecs:

| SlotKind | Relation-participant meaning | ValueKind | refMode |
| --- | --- | --- | --- |
| ` + "`EarlierEpistemeSlot`" + ` | exact earlier episteme | ` + "`U.Episteme`" + ` | ` + "`U.EpistemeRef`" + ` |
| ` + "`LaterEpistemeSlot`" + ` | exact later episteme | ` + "`U.Episteme`" + ` | ` + "`U.EpistemeRef`" + ` |

The relation obtains when the two epistemes have different identities and governed revision, refinement, or supersession work establishes continuation.

One occurrence is participant-determined by the exact earlier and later episteme pair.
` + currentC3TypeEnvFixture()
}

func currentC3TypeEnvFixture() string {
	lines := []string{
		"",
		"## C.3.1 - Kind Core",
		"",
		"### C.3.1:4 - Declare the direct subkind relation",
		"",
		"| `U.SubkindOf` | narrower and broader local kinds |",
		"`SubkindOfObtains(k1, k2; RS)` is the direct predicate.",
		"`R_sub : U.SubkindOf` names the relation occurrence.",
		"Keep the subkind assertion episteme separate.",
		"Participant identities plus the exact effective reference-scheme edition determine its identity.",
		"",
		"### C.3.1:5 - Preserve order laws",
		"",
		"Keep a partial order over obtaining facts.",
		"Reflexivity, transitivity, and antisymmetry govern the order.",
		"Compare the same candidate and context slice under aligned editions.",
		"`unknown` remains non-settlement.",
		"",
		"## C.3.2 - Kind Classification",
		"",
		"### C.3.2:5 - Declare the KindSignature",
		"",
		"Declare the exact local kind that is its `EntityOfConcern`.",
		"Declare the candidate `ValueKind`.",
		"Declare direct governed candidate qualities, relations, constructive grounding, or other features.",
		"Pin the exact `U.ContextSlice` conditions.",
		"Pin the effective `U.ReferenceScheme`.",
		"Name named assumptions, dependencies, standards, versions, units, and temporal policy.",
		"Declare its `U.Formality` and an optional `ExtentRule`.",
		"",
		"### C.3.2:6 - Evaluate one classification judgement",
		"",
		"`J(candidate, kind, signatureEdition, slice) ∈ {true, false, unknown}`.",
		"Pin all four inputs.",
		"Evaluate direct governed features.",
		"Missing settlement gives `unknown`, not `false`.",
		"Separate support from satisfaction.",
		"Separate guard disposition.",
		"",
		"### C.3.2:7 - Materialize an optional extension",
		"",
		"Materialize `KindExtension(k, slice)` only when a named receiving use needs it.",
		"Pin the `KindSignature` edition without inventing `U.EntitySet`.",
		"Include only a candidate whose pinned judgment is `true`.",
		"They do not create a collection holon, an A.14 membership occurrence, a direct classification relation, or the candidate features.",
		"",
		"## C.3.3 - Kind Bridge",
		"",
		"### C.3.3:5 - Declare and apply a bridge",
		"",
		"A `KindBridge` occurrence is an obtaining direct relation between one exact source local `U.Kind` and one exact target local `U.Kind`.",
		"Pin source and target scheme editions.",
		"Keep the direct relation separate from the C.2.1 bridge-assertion episteme.",
		"Re-evaluate `J(candidate, targetKind, targetSignatureEdition, TargetSlice) ∈ {true, false, unknown}`.",
		"A source result is never reused as target truth.",
		"",
		"## C.3.4 - Role Mask",
		"",
		"### C.3.4:5 - Declare and evaluate a role mask",
		"",
		"A `RoleMask` is a named, versioned C.2.1 declaration episteme.",
		"Declare additional direct candidate-feature predicates.",
		"Route scope expectations routed separately to USM Scope.",
		"`J_mask(candidate, kind, kindSignatureEdition, roleMaskEdition, slice) ∈ {true, false, unknown}`.",
		"A scope refusal is separate so that refusal is not a `false` classification.",
		"",
		"## C.3.A - Kind Guards",
		"",
		"### C.3.A:3 - Keep classification and guard disposition separate",
		"",
		"Three classification values.",
		"Separate guard disposition.",
		"Both `false` and `unknown` normally cause fail-closed refusal.",
		"Scope separation.",
		"Bridge separation.",
		"",
	}
	return strings.Join(lines, "\n")
}

func inspectSourceUnit(t *testing.T, dbPath, identifier string) fpf.SourceUnit {
	t.Helper()
	db := openTestDB(t, dbPath)
	defer func() { _ = db.Close() }()

	index := fpf.NewSQLiteQueryIndex(db)
	roles, err := fpf.NormalizeSourceUnitRoles(nil)
	if err != nil {
		t.Fatalf("normalize source roles: %v", err)
	}
	unit, found, err := index.InspectExact(identifier, roles)
	if err != nil {
		t.Fatalf("inspect %s: %v", identifier, err)
	}
	if !found {
		t.Fatalf("source unit %s not found", identifier)
	}
	return unit
}

func lookupSourceUnit(t *testing.T, dbPath, identifier string) fpf.SourceUnit {
	t.Helper()
	db := openTestDB(t, dbPath)
	defer func() { _ = db.Close() }()

	index := fpf.NewSQLiteQueryIndex(db)
	roles, err := fpf.NormalizeSourceUnitRoles(nil)
	if err != nil {
		t.Fatalf("normalize source roles: %v", err)
	}
	unit, found, err := index.LookupExact(identifier, roles)
	if err != nil {
		t.Fatalf("lookup %s: %v", identifier, err)
	}
	if !found {
		t.Fatalf("source unit %s not found", identifier)
	}
	return unit
}

func openTestDB(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open %s: %v", dbPath, err)
	}
	return db
}

func loadPreviousTypeEnvEnvelope(
	dbPath string,
) (typeenv.CompilationEnvelope, bool, error) {
	snapshot, hasPreviousSnapshot, err := openPreviousTypeEnvSnapshot(dbPath)
	if err != nil {
		return typeenv.CompilationEnvelope{}, false, err
	}
	if !hasPreviousSnapshot {
		return typeenv.CompilationEnvelope{}, false, nil
	}
	envelope, exists, loadErr :=
		loadPreviousTypeEnvEnvelopeSnapshot(snapshot.transaction)
	if err := finishPreviousTypeEnvSnapshot(snapshot, loadErr); err != nil {
		return typeenv.CompilationEnvelope{}, false, err
	}
	return envelope, exists, nil
}

func buildCurrentTypeEnvArtifact(
	t *testing.T,
	dir string,
) (typeenv.BaseTypeEnvArtifact, string) {
	t.Helper()
	specPath := writeSourceFixture(t, dir, "snapshot replacement fixture")
	dbPath := filepath.Join(dir, "current.db")
	if err := buildIndex(specPath, dbPath, "snapshot-replacement-revision"); err != nil {
		t.Fatalf("build current TypeEnv fixture: %v", err)
	}
	envelope, exists, err := loadPreviousTypeEnvEnvelope(dbPath)
	if err != nil || !exists {
		t.Fatalf("load current TypeEnv fixture: exists=%v err=%v", exists, err)
	}
	return envelope.Artifact(), dbPath
}

func storeTypeEnvCompilerVariant(
	t *testing.T,
	dbPath string,
	sourceDBPath string,
	current typeenv.BaseTypeEnvArtifact,
	compiler string,
) typeenv.BaseTypeEnvArtifact {
	t.Helper()
	version, err := typedmemory.NewCompilerSchemaVersion(compiler)
	if err != nil {
		t.Fatalf("NewCompilerSchemaVersion(%s): %v", compiler, err)
	}
	compiled, err := typeenv.NewCompiledLinkedTypeEnvIR(
		current.SourceRevision(),
		version,
		current.CoverageManifest(),
		current.Declarations(),
	)
	if err != nil {
		t.Fatalf("NewCompiledLinkedTypeEnvIR(%s): %v", compiler, err)
	}
	artifact, err := typeenv.SealBaseTypeEnv(compiled)
	if err != nil {
		t.Fatalf("SealBaseTypeEnv(%s): %v", compiler, err)
	}
	envelope, err := typeenv.NewCompilationEnvelope(
		artifact,
		typeenv.NewInitialCompatibilityAssessment(),
	)
	if err != nil {
		t.Fatalf("NewCompilationEnvelope(%s): %v", compiler, err)
	}
	sourceBytes, err := os.ReadFile(sourceDBPath)
	if err != nil {
		t.Fatalf("read complete source-index fixture: %v", err)
	}
	if err := os.WriteFile(dbPath, sourceBytes, 0o600); err != nil {
		t.Fatalf("clone complete source-index fixture: %v", err)
	}
	if err := storeTypeEnvEnvelope(dbPath, envelope); err != nil {
		t.Fatalf("store TypeEnv compiler variant %s: %v", compiler, err)
	}
	return artifact
}

func replaceDatabasePath(activePath, replacementPath, archivedPath string) error {
	if err := os.Rename(activePath, archivedPath); err != nil {
		return fmt.Errorf("archive opened predecessor: %w", err)
	}
	if err := os.Rename(replacementPath, activePath); err != nil {
		_ = os.Rename(archivedPath, activePath)
		return fmt.Errorf("install predecessor replacement: %w", err)
	}
	return nil
}

func writeMinimalMetadataDatabase(
	t *testing.T,
	dbPath string,
	metadata map[string]string,
) {
	t.Helper()
	database := openTestDB(t, dbPath)
	if _, err := database.Exec(`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		t.Fatalf("create minimal metadata table: %v", err)
	}
	for key, value := range metadata {
		if _, err := database.Exec(
			`INSERT INTO meta (key, value) VALUES (?, ?)`,
			key,
			value,
		); err != nil {
			t.Fatalf("insert minimal metadata %s: %v", key, err)
		}
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close minimal metadata database: %v", err)
	}
}

func assertFailedVerificationDoesNotMutate(
	t *testing.T,
	dbPath string,
	verify func() error,
	wantError string,
) {
	t.Helper()
	beforeDigest := fileDigest(t, dbPath)
	beforeObjects := sqliteObjectCount(t, dbPath)
	err := verify()
	if err == nil || !strings.Contains(err.Error(), wantError) {
		t.Fatalf("verification error = %v, want %q", err, wantError)
	}
	if strings.Contains(strings.ToLower(err.Error()), "readonly") {
		t.Fatalf("verification attempted a write instead of using read-only validation: %v", err)
	}
	afterDigest := fileDigest(t, dbPath)
	afterObjects := sqliteObjectCount(t, dbPath)
	if beforeDigest != afterDigest {
		t.Fatal("read-only verification changed database bytes")
	}
	if beforeObjects != afterObjects {
		t.Fatalf(
			"read-only verification changed SQLite object count from %d to %d",
			beforeObjects,
			afterObjects,
		)
	}
}

func sqliteObjectCount(t *testing.T, dbPath string) int {
	t.Helper()
	database := openTestDB(t, dbPath)
	defer func() { _ = database.Close() }()
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM sqlite_master`).Scan(&count); err != nil {
		t.Fatalf("count SQLite objects: %v", err)
	}
	return count
}

func assertTablesAbsent(t *testing.T, dbPath string, tableNames ...string) {
	t.Helper()
	db := openTestDB(t, dbPath)
	defer func() { _ = db.Close() }()

	for _, tableName := range tableNames {
		var count int
		err := db.
			QueryRow(
				`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
				tableName,
			).
			Scan(&count)
		if err != nil {
			t.Fatalf("inspect table %s: %v", tableName, err)
		}
		if count != 0 {
			t.Fatalf("obsolete table %s is present in source-native index", tableName)
		}
	}
}

func fileDigest(t *testing.T, path string) [sha256.Size]byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return sha256.Sum256(content)
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}
