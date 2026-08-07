package fpfrefresh

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"

	_ "modernc.org/sqlite"
)

const candidateArtifactTestRevision = "308edacfa2bdb2c60d07e4e10c0deb1f260a6a31"
const candidateArtifactTestGeneratedBy = "haft-test@candidate"
const candidateArtifactTestTokenDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestPrepareCandidateArtifactBuildsExactVerifiedOwnedCandidate(t *testing.T) {
	source := candidateArtifactProductionSource(t)
	predecessorPath := candidateArtifactPredecessorDatabase(t)
	predecessorBefore := candidateArtifactReadFile(t, predecessorPath)
	builder := &candidateArtifactVerifyingBuilder{}

	artifact, err := PrepareCandidateArtifact(
		context.Background(),
		CandidatePreparationInput{
			Source:                  source,
			PredecessorDatabasePath: predecessorPath,
			Builder:                 builder,
			GeneratedBy:             candidateArtifactTestGeneratedBy,
			TokenGate: &TokenGateCoordinates{
				FixtureRevision: "candidate-token-fixture-v1",
				FixtureDigest:   candidateArtifactTestTokenDigest,
			},
		},
	)
	if err != nil {
		t.Fatalf("PrepareCandidateArtifact() error = %v", err)
	}
	cleaned := false
	t.Cleanup(func() {
		if !cleaned {
			_ = artifact.Cleanup()
		}
	})

	if len(builder.inputs) != 2 {
		t.Fatalf("builder calls = %d, want 2", len(builder.inputs))
	}
	for index, input := range builder.inputs {
		if input.ReadmePath != candidateLogicalReadmePath ||
			input.SpecificationPath != candidateLogicalSpecPath ||
			input.DatabasePath != candidateLogicalDBPath ||
			input.SourceRevision != candidateArtifactTestRevision {
			t.Fatalf("builder input %d = %#v, want stable logical paths and exact revision", index, input)
		}
		if !filepath.IsAbs(input.WorkingDirectory) {
			t.Fatalf("builder workspace %d = %q, want absolute private path", index, input.WorkingDirectory)
		}
	}
	if builder.inputs[0].WorkingDirectory == builder.inputs[1].WorkingDirectory {
		t.Fatal("determinism builds reused one workspace")
	}
	if len(builder.readmeBytes) != 2 ||
		!bytes.Equal(builder.readmeBytes[0], source.ReadmeBytes()) ||
		!bytes.Equal(builder.readmeBytes[1], source.ReadmeBytes()) {
		t.Fatal("builder did not observe exact Readme.md bytes in both workspaces")
	}
	if len(builder.specificationBytes) != 2 ||
		!bytes.Equal(builder.specificationBytes[0], source.SpecificationBytes()) ||
		!bytes.Equal(builder.specificationBytes[1], source.SpecificationBytes()) {
		t.Fatal("builder did not observe exact FPF-Spec.md bytes in both workspaces")
	}
	if len(builder.predecessorBytes) != 2 ||
		!bytes.Equal(builder.predecessorBytes[0], predecessorBefore) ||
		!bytes.Equal(builder.predecessorBytes[1], predecessorBefore) {
		t.Fatal("each build did not start from an exact predecessor database copy")
	}
	if got := candidateArtifactReadFile(t, predecessorPath); !bytes.Equal(got, predecessorBefore) {
		t.Fatal("candidate preparation changed predecessor database bytes")
	}
	if got := candidateArtifactReadFile(t, artifact.ReadmePath()); !bytes.Equal(got, source.ReadmeBytes()) {
		t.Fatal("retained Readme.md differs from immutable source snapshot")
	}
	if got := candidateArtifactReadFile(t, artifact.SpecificationPath()); !bytes.Equal(got, source.SpecificationBytes()) {
		t.Fatal("retained FPF-Spec.md differs from immutable source snapshot")
	}
	if !candidateArtifactPathWithin(artifact.OwnedRootPath(), artifact.DatabasePath()) {
		t.Fatalf("candidate database %q is outside owned root %q", artifact.DatabasePath(), artifact.OwnedRootPath())
	}
	rootInfo, err := os.Stat(artifact.OwnedRootPath())
	if err != nil {
		t.Fatal(err)
	}
	if permissions := rootInfo.Mode().Perm(); permissions != 0o700 {
		t.Fatalf("candidate root permissions = %o, want 700", permissions)
	}

	lock := artifact.IntegrationLock()
	if err := lock.Validate(); err != nil {
		t.Fatalf("candidate IntegrationLock.Validate() error = %v", err)
	}
	if lock.Coordinates.SourceRevision != candidateArtifactTestRevision ||
		lock.Coordinates.DatabaseDigest == "" ||
		lock.GeneratedBy != candidateArtifactTestGeneratedBy ||
		lock.TokenGate == nil ||
		lock.TokenGate.FixtureDigest != candidateArtifactTestTokenDigest {
		t.Fatalf("candidate lock = %#v", lock)
	}
	if err := VerifyIntegrationLock(lock, IntegrationCoordinateInput{
		SourceRevision: candidateArtifactTestRevision,
		ReadmePath:     artifact.ReadmePath(),
		SpecPath:       artifact.SpecificationPath(),
		DatabasePath:   artifact.DatabasePath(),
		GeneratedBy:    candidateArtifactTestGeneratedBy,
		TokenGate: &TokenGateCoordinates{
			FixtureRevision: "candidate-token-fixture-v1",
			FixtureDigest:   candidateArtifactTestTokenDigest,
		},
	}); err != nil {
		t.Fatalf("VerifyIntegrationLock() against retained candidate error = %v", err)
	}
	canonicalLock, err := MarshalIntegrationLock(lock)
	if err != nil {
		t.Fatal(err)
	}
	if got := candidateArtifactReadFile(t, artifact.LockPath()); !bytes.Equal(got, canonicalLock) {
		t.Fatal("retained candidate lock differs from canonical generated bytes")
	}
	if artifact.LockDigest() == "" {
		t.Fatal("candidate artifact omitted canonical lock digest")
	}
	lockDigest, err := digestFile(artifact.LockPath())
	if err != nil {
		t.Fatal(err)
	}
	if artifact.LockDigest() != lockDigest {
		t.Fatalf("candidate lock digest = %q, want %q", artifact.LockDigest(), lockDigest)
	}
	if !candidateArtifactPathWithin(artifact.OwnedRootPath(), artifact.LockPath()) {
		t.Fatalf("candidate lock %q is outside owned root %q", artifact.LockPath(), artifact.OwnedRootPath())
	}
	results := artifact.QuerySmokeResults()
	if len(results) != 12 {
		t.Fatalf("candidate query smoke count = %d, want 12", len(results))
	}
	results[0].UnitIDs = append(results[0].UnitIDs, "caller-mutation")
	if containsString(artifact.QuerySmokeResults()[0].UnitIDs, "caller-mutation") {
		t.Fatal("caller mutation changed owned query-smoke evidence")
	}

	comparisonWorkspace := builder.inputs[1].WorkingDirectory
	if _, err := os.Stat(comparisonWorkspace); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("comparison workspace remains after successful preparation: %v", err)
	}
	rootPath := artifact.OwnedRootPath()
	sentinelPath := filepath.Join(filepath.Dir(rootPath), "candidate-cleanup-sentinel")
	if err := os.WriteFile(sentinelPath, []byte("outside owned root\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(sentinelPath) })
	if err := artifact.Cleanup(); err != nil {
		t.Fatalf("CandidateArtifact.Cleanup() error = %v", err)
	}
	cleaned = true
	if !artifact.Cleaned() {
		t.Fatal("CandidateArtifact.Cleaned() = false after successful cleanup")
	}
	if _, err := os.Stat(rootPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owned candidate root still exists after cleanup: %v", err)
	}
	if got := candidateArtifactReadFile(t, sentinelPath); string(got) != "outside owned root\n" {
		t.Fatalf("bounded cleanup changed external sentinel to %q", got)
	}
	if err := artifact.Cleanup(); err != nil {
		t.Fatalf("idempotent CandidateArtifact.Cleanup() error = %v", err)
	}
}

func TestPrepareCandidateArtifactRejectsNonDeterministicBuildAndCleansRoot(t *testing.T) {
	t.Parallel()

	source := candidateArtifactSmallSource()
	predecessorPath := candidateArtifactPredecessorDatabase(t)
	predecessorBefore := candidateArtifactReadFile(t, predecessorPath)
	var inputs []IndexBuildInput
	builder := IndexBuilderFunc(func(_ context.Context, input IndexBuildInput) error {
		inputs = append(inputs, input)
		databasePath := candidateAbsolutePath(input.WorkingDirectory, input.DatabasePath)
		return os.WriteFile(
			databasePath,
			[]byte(fmt.Sprintf("non-deterministic-build-%d\n", len(inputs))),
			0o600,
		)
	})

	artifact, err := PrepareCandidateArtifact(
		context.Background(),
		CandidatePreparationInput{
			Source:                  source,
			PredecessorDatabasePath: predecessorPath,
			Builder:                 builder,
			GeneratedBy:             candidateArtifactTestGeneratedBy,
		},
	)
	if artifact != nil {
		t.Fatal("non-deterministic build returned a candidate artifact")
	}
	if !errors.Is(err, ErrCandidateNonDeterministic) {
		t.Fatalf("error = %v, want ErrCandidateNonDeterministic", err)
	}
	if len(inputs) != 2 {
		t.Fatalf("builder calls = %d, want 2 before deterministic rejection", len(inputs))
	}
	ownedRoot := filepath.Dir(inputs[0].WorkingDirectory)
	if _, statErr := os.Stat(ownedRoot); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("failed candidate root remains after rejection: %v", statErr)
	}
	if got := candidateArtifactReadFile(t, predecessorPath); !bytes.Equal(got, predecessorBefore) {
		t.Fatal("non-deterministic rejection changed predecessor database bytes")
	}
}

func TestPrepareCandidateArtifactRetainsSourceSpecificQueryDriftForReview(t *testing.T) {
	source := candidateArtifactProductionSource(t)
	source.specificationBytes = bytes.ReplaceAll(
		source.specificationBytes,
		[]byte("SYSTEM-RECOGNITION"),
		[]byte("SYSTEM-RECOGNITION-ALT"),
	)
	source.specificationBytes = bytes.Replace(
		source.specificationBytes,
		[]byte("- **Template A.**"),
		[]byte("- **Fresh outcome route.**"),
		1,
	)
	predecessorPath := candidateArtifactPredecessorDatabase(t)
	builder := &candidateArtifactVerifyingBuilder{}

	artifact, err := PrepareCandidateArtifact(
		context.Background(),
		CandidatePreparationInput{
			Source:                  source,
			PredecessorDatabasePath: predecessorPath,
			Builder:                 builder,
			GeneratedBy:             candidateArtifactTestGeneratedBy,
		},
	)
	if err != nil {
		t.Fatalf("PrepareCandidateArtifact() error = %v", err)
	}
	if artifact == nil {
		t.Fatal("source-specific query drift discarded a structurally valid candidate")
	}
	t.Cleanup(func() { _ = artifact.Cleanup() })
	if len(builder.inputs) != 2 {
		t.Fatalf(
			"builder calls = %d, want 2 deterministic copies before verification",
			len(builder.inputs),
		)
	}
	if err := VerifySourceQueryRuntime(artifact.DatabasePath()); err != nil {
		t.Fatalf("candidate source Query runtime is invalid: %v", err)
	}
	querySmokeErr := artifact.querySmokeError()
	if querySmokeErr == nil ||
		!strings.Contains(querySmokeErr.Error(), "query_contract_regression") {
		t.Fatalf(
			"query smoke error = %v, want retained source-specific regression",
			querySmokeErr,
		)
	}
	if len(artifact.QuerySmokeResults()) != 0 {
		t.Fatalf(
			"failed source-specific smoke results = %#v, want no invented successes",
			artifact.QuerySmokeResults(),
		)
	}
	sourceDiagnostics := artifact.sourceGrammarDiagnostics()
	if len(sourceDiagnostics) != 1 ||
		sourceDiagnostics[0].Class != fpf.SourceGrammarUnsupported {
		t.Fatalf("source grammar diagnostics = %#v", sourceDiagnostics)
	}
}

func TestPrepareCandidateArtifactRejectsWrongSourceProjectionAndCleansRoot(
	t *testing.T,
) {
	source := candidateArtifactProductionSource(t)
	predecessorPath := candidateArtifactPredecessorDatabase(t)
	builder := &candidateArtifactVerifyingBuilder{
		afterBuild: func(databasePath string) error {
			database, err := sql.Open("sqlite", databasePath)
			if err != nil {
				return err
			}
			if _, err := database.Exec(
				`UPDATE meta
				 SET value = 'sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
				 WHERE key = 'spec_document_digest'`,
			); err != nil {
				_ = database.Close()
				return err
			}
			if err := database.Close(); err != nil {
				return err
			}
			if _, err := VerifyCandidateQueryContract(databasePath); err != nil {
				return fmt.Errorf(
					"wrong-provenance fixture must retain the query contract: %w",
					err,
				)
			}
			return nil
		},
	}

	artifact, err := PrepareCandidateArtifact(
		context.Background(),
		CandidatePreparationInput{
			Source:                  source,
			PredecessorDatabasePath: predecessorPath,
			Builder:                 builder,
			GeneratedBy:             candidateArtifactTestGeneratedBy,
		},
	)
	if artifact != nil {
		t.Fatal("wrong source projection returned a candidate artifact")
	}
	if err == nil ||
		!strings.Contains(
			err.Error(),
			"verify candidate source and TypeEnv projection",
		) {
		t.Fatalf("error = %v, want exact candidate projection rejection", err)
	}
	if len(builder.inputs) != 2 {
		t.Fatalf("builder calls = %d, want 2 deterministic copies", len(builder.inputs))
	}
	ownedRoot := filepath.Dir(builder.inputs[0].WorkingDirectory)
	if _, statErr := os.Stat(ownedRoot); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("wrong-projection candidate root remains after rejection: %v", statErr)
	}
}

type candidateArtifactVerifyingBuilder struct {
	inputs             []IndexBuildInput
	readmeBytes        [][]byte
	specificationBytes [][]byte
	predecessorBytes   [][]byte
	afterBuild         func(databasePath string) error
}

func (builder *candidateArtifactVerifyingBuilder) BuildIndex(
	ctx context.Context,
	input IndexBuildInput,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	readmePath := candidateAbsolutePath(input.WorkingDirectory, input.ReadmePath)
	specificationPath := candidateAbsolutePath(
		input.WorkingDirectory,
		input.SpecificationPath,
	)
	databasePath := candidateAbsolutePath(input.WorkingDirectory, input.DatabasePath)
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		return err
	}
	specification, err := os.ReadFile(specificationPath)
	if err != nil {
		return err
	}
	predecessor, err := os.ReadFile(databasePath)
	if err != nil {
		return err
	}
	snapshot, err := fpf.BuildPublicationSnapshot(fpf.SourceBundle{
		Readme: fpf.SourceDocument{
			Path:           input.ReadmePath,
			SourceRevision: input.SourceRevision,
			Markdown:       readme,
		},
		Spec: fpf.SourceDocument{
			Path:           input.SpecificationPath,
			SourceRevision: input.SourceRevision,
			Markdown:       specification,
		},
	})
	if err != nil {
		return err
	}
	if err := fpf.StoreSourceUnits(databasePath, snapshot.SourceUnits()); err != nil {
		return err
	}
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return err
	}
	if _, err := database.Exec(
		`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
	); err != nil {
		_ = database.Close()
		return err
	}
	if err := database.Close(); err != nil {
		return err
	}
	compilation, err := typeenv.CompileBaseTypeEnv(snapshot)
	if err != nil {
		return err
	}
	artifact, accepted := compilation.Artifact()
	if !accepted {
		return fmt.Errorf(
			"candidate test TypeEnv compilation rejected: %v",
			compilation.Diagnostics(),
		)
	}
	envelope, err := typeenv.NewCompilationEnvelope(
		artifact,
		typeenv.NewInitialCompatibilityAssessment(),
	)
	if err != nil {
		return err
	}
	database, err = sql.Open("sqlite", databasePath)
	if err != nil {
		return err
	}
	if err := typeenvsql.ReplaceEnvelopeDB(
		context.Background(),
		database,
		envelope,
	); err != nil {
		_ = database.Close()
		return err
	}
	if err := database.Close(); err != nil {
		return err
	}
	typeEnvRef, exists := artifact.TypeEnvRef()
	if !exists {
		return fmt.Errorf("candidate test TypeEnv has no reference")
	}
	if err := fpf.SetSpecMetaEntries(databasePath, map[string]string{
		"fpf_commit":                      input.SourceRevision,
		"indexed_source_units":            strconv.Itoa(len(snapshot.SourceUnits())),
		"readme_document_digest":          snapshot.ReadmeDigest().String(),
		"schema_version":                  fpf.SpecIndexSchemaVersion,
		"spec_document_digest":            snapshot.SpecDigest().String(),
		"typeenv_artifact_digest":         artifact.Digest().String(),
		"typeenv_compiler_schema_version": artifact.CompilerSchemaVersion().String(),
		"typeenv_ref":                     typeEnvRef.String(),
		"typeenv_source_revision":         input.SourceRevision,
	}); err != nil {
		return err
	}
	if builder.afterBuild != nil {
		if err := builder.afterBuild(databasePath); err != nil {
			return err
		}
	}
	builder.inputs = append(builder.inputs, input)
	builder.readmeBytes = append(builder.readmeBytes, bytes.Clone(readme))
	builder.specificationBytes = append(
		builder.specificationBytes,
		bytes.Clone(specification),
	)
	builder.predecessorBytes = append(builder.predecessorBytes, bytes.Clone(predecessor))
	return nil
}

func candidateArtifactProductionSource(t *testing.T) GitSourceSnapshot {
	t.Helper()
	readmePath := filepath.Join("..", "..", "data", "FPF", "Readme.md")
	specificationPath := filepath.Join("..", "..", "data", "FPF", "FPF-Spec.md")
	return GitSourceSnapshot{
		candidateRef:       "refs/remotes/origin/main",
		commitSHA:          candidateArtifactTestRevision,
		readmeBytes:        candidateArtifactReadFile(t, readmePath),
		specificationBytes: candidateArtifactReadFile(t, specificationPath),
	}
}

func candidateArtifactSmallSource() GitSourceSnapshot {
	return GitSourceSnapshot{
		candidateRef:       "refs/heads/candidate",
		commitSHA:          candidateArtifactTestRevision,
		readmeBytes:        []byte("candidate readme\n"),
		specificationBytes: []byte("candidate specification\n"),
	}
}

func candidateArtifactPredecessorDatabase(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "predecessor.db")
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(
		`CREATE TABLE predecessor_marker (value TEXT NOT NULL)`,
	); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if _, err := database.Exec(
		`INSERT INTO predecessor_marker(value) VALUES ('last-good')`,
	); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func candidateArtifactReadFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return content
}

func candidateArtifactPathWithin(root string, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative != "." &&
		relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
