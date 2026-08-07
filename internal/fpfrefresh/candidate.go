package fpfrefresh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/m0n0x41d/haft/internal/fpf"
)

const (
	candidateTemporaryPrefix = "haft-fpf-candidate-"

	candidateLogicalReadmePath = "data/FPF/Readme.md"
	candidateLogicalSpecPath   = "data/FPF/FPF-Spec.md"
	candidateLogicalDBPath     = "internal/cli/fpf.db"
	candidateLogicalLockPath   = "data/haft/fpf-integration.lock.json"
)

var (
	// ErrCandidateNonDeterministic means two builds from the same source and
	// predecessor bytes produced different candidate database bytes.
	ErrCandidateNonDeterministic = errors.New("candidate index build is not deterministic")

	// ErrCandidatePredecessorChanged means the predecessor database changed
	// while candidate preparation was reading it.
	ErrCandidatePredecessorChanged = errors.New("candidate predecessor database changed during preparation")
)

// IndexBuildInput gives an injected builder one private workspace and stable
// logical paths within it. Builders should set their process working directory
// to WorkingDirectory rather than substituting absolute temporary paths into
// source provenance or index metadata.
type IndexBuildInput struct {
	WorkingDirectory  string
	ReadmePath        string
	SpecificationPath string
	DatabasePath      string
	SourceRevision    string
}

// IndexBuilder builds one candidate index in a prepared private workspace.
// DatabasePath already contains an exact copy of the predecessor database.
type IndexBuilder interface {
	BuildIndex(ctx context.Context, input IndexBuildInput) error
}

// IndexBuilderFunc adapts a function to IndexBuilder.
type IndexBuilderFunc func(ctx context.Context, input IndexBuildInput) error

func (function IndexBuilderFunc) BuildIndex(
	ctx context.Context,
	input IndexBuildInput,
) error {
	if function == nil {
		return fmt.Errorf("candidate index builder function is required")
	}
	return function(ctx, input)
}

// CandidatePreparationInput is the complete non-binding input to one
// candidate build. GeneratedBy and TokenGate are carried into the canonical
// lock rather than inferred or hardcoded by this package.
type CandidatePreparationInput struct {
	Source                  GitSourceSnapshot
	PredecessorDatabasePath string
	Builder                 IndexBuilder
	GeneratedBy             string
	TokenGate               *TokenGateCoordinates
}

// CandidateArtifact owns one verified candidate database and its exact source
// materialization. Its paths remain valid until Cleanup succeeds.
type CandidateArtifact struct {
	mutex             sync.Mutex
	source            GitSourceSnapshot
	ownedRootPath     string
	workspacePath     string
	readmePath        string
	specificationPath string
	databasePath      string
	lockPath          string
	lockDigest        string
	integrationLock   IntegrationLock
	querySmokeResults []QuerySmokeResult
	querySmokeFailure error
	sourceDiagnostics []fpf.SourceGrammarDiagnostic
	cleaned           bool
}

// SourceSnapshot returns the immutable Git source basis.
func (artifact *CandidateArtifact) SourceSnapshot() GitSourceSnapshot {
	if artifact == nil {
		return GitSourceSnapshot{}
	}
	artifact.mutex.Lock()
	defer artifact.mutex.Unlock()
	return artifact.source
}

// OwnedRootPath returns the private directory removed by Cleanup.
func (artifact *CandidateArtifact) OwnedRootPath() string {
	if artifact == nil {
		return ""
	}
	artifact.mutex.Lock()
	defer artifact.mutex.Unlock()
	return artifact.ownedRootPath
}

// WorkspacePath returns the retained build workspace.
func (artifact *CandidateArtifact) WorkspacePath() string {
	if artifact == nil {
		return ""
	}
	artifact.mutex.Lock()
	defer artifact.mutex.Unlock()
	return artifact.workspacePath
}

// ReadmePath returns the retained absolute Readme.md path.
func (artifact *CandidateArtifact) ReadmePath() string {
	if artifact == nil {
		return ""
	}
	artifact.mutex.Lock()
	defer artifact.mutex.Unlock()
	return artifact.readmePath
}

// SpecificationPath returns the retained absolute FPF-Spec.md path.
func (artifact *CandidateArtifact) SpecificationPath() string {
	if artifact == nil {
		return ""
	}
	artifact.mutex.Lock()
	defer artifact.mutex.Unlock()
	return artifact.specificationPath
}

// DatabasePath returns the retained verified candidate database path.
func (artifact *CandidateArtifact) DatabasePath() string {
	if artifact == nil {
		return ""
	}
	artifact.mutex.Lock()
	defer artifact.mutex.Unlock()
	return artifact.databasePath
}

// LockPath returns the retained canonical generated integration-lock path.
func (artifact *CandidateArtifact) LockPath() string {
	if artifact == nil {
		return ""
	}
	artifact.mutex.Lock()
	defer artifact.mutex.Unlock()
	return artifact.lockPath
}

// LockDigest returns the sha256 digest of the retained canonical lock bytes.
func (artifact *CandidateArtifact) LockDigest() string {
	if artifact == nil {
		return ""
	}
	artifact.mutex.Lock()
	defer artifact.mutex.Unlock()
	return artifact.lockDigest
}

// IntegrationLock returns a copy of the verified generated lock.
func (artifact *CandidateArtifact) IntegrationLock() IntegrationLock {
	if artifact == nil {
		return IntegrationLock{}
	}
	artifact.mutex.Lock()
	defer artifact.mutex.Unlock()
	owned := artifact.integrationLock
	owned.TokenGate = cloneTokenGateCoordinates(artifact.integrationLock.TokenGate)
	return owned
}

// QuerySmokeResults returns copies of the successful candidate query smokes.
func (artifact *CandidateArtifact) QuerySmokeResults() []QuerySmokeResult {
	if artifact == nil {
		return nil
	}
	artifact.mutex.Lock()
	defer artifact.mutex.Unlock()
	return cloneCandidateQuerySmokeResults(artifact.querySmokeResults)
}

// querySmokeError returns source-specific query expectation drift retained on
// an otherwise structurally valid candidate. It is review evidence, not a
// candidate-integrity result.
func (artifact *CandidateArtifact) querySmokeError() error {
	if artifact == nil {
		return nil
	}
	artifact.mutex.Lock()
	defer artifact.mutex.Unlock()
	return artifact.querySmokeFailure
}

func (artifact *CandidateArtifact) sourceGrammarDiagnostics() []fpf.SourceGrammarDiagnostic {
	if artifact == nil {
		return nil
	}
	artifact.mutex.Lock()
	defer artifact.mutex.Unlock()
	return cloneCandidateSourceGrammarDiagnostics(artifact.sourceDiagnostics)
}

// Cleaned reports whether Cleanup has successfully removed the owned root.
func (artifact *CandidateArtifact) Cleaned() bool {
	if artifact == nil {
		return true
	}
	artifact.mutex.Lock()
	defer artifact.mutex.Unlock()
	return artifact.cleaned
}

// Cleanup removes only the exact private root created by
// PrepareCandidateArtifact. It is idempotent.
func (artifact *CandidateArtifact) Cleanup() error {
	if artifact == nil {
		return nil
	}
	artifact.mutex.Lock()
	defer artifact.mutex.Unlock()
	if artifact.cleaned {
		return nil
	}
	if err := removeCandidateArtifactRoot(artifact.ownedRootPath); err != nil {
		return err
	}
	artifact.cleaned = true
	return nil
}

// PrepareCandidateArtifact builds and verifies one candidate without changing
// the source checkout or predecessor database. Two independent workspaces use
// identical logical paths so randomized temporary roots cannot enter database
// provenance or defeat byte determinism.
func PrepareCandidateArtifact(
	ctx context.Context,
	input CandidatePreparationInput,
) (artifact *CandidateArtifact, resultErr error) {
	if ctx == nil {
		return nil, fmt.Errorf("candidate preparation context is required")
	}
	if input.Builder == nil {
		return nil, fmt.Errorf("candidate index builder is required")
	}
	sourceRevision, readmeBytes, specificationBytes, err :=
		validateCandidateSourceSnapshot(input.Source)
	if err != nil {
		return nil, err
	}
	generatedBy, err := validateCandidateGeneratedBy(input.GeneratedBy)
	if err != nil {
		return nil, err
	}
	predecessorPath, err := validateCandidatePredecessorPath(
		input.PredecessorDatabasePath,
	)
	if err != nil {
		return nil, err
	}
	predecessorDigest, err := digestFile(predecessorPath)
	if err != nil {
		return nil, fmt.Errorf("digest predecessor candidate basis: %w", err)
	}

	ownedRootPath, err := os.MkdirTemp("", candidateTemporaryPrefix)
	if err != nil {
		return nil, fmt.Errorf("create private candidate root: %w", err)
	}
	rootTransferred := false
	defer func() {
		if rootTransferred {
			return
		}
		cleanupErr := removeCandidateArtifactRoot(ownedRootPath)
		if cleanupErr == nil {
			return
		}
		wrapped := fmt.Errorf("clean failed candidate root: %w", cleanupErr)
		if resultErr == nil {
			resultErr = wrapped
			return
		}
		resultErr = errors.Join(resultErr, wrapped)
	}()

	builds := make([]candidateBuildWorkspace, 0, 2)
	for ordinal := 1; ordinal <= 2; ordinal++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("prepare candidate build %d: %w", ordinal, err)
		}
		workspace, copyDigest, prepareErr := prepareCandidateBuildWorkspace(
			ownedRootPath,
			ordinal,
			sourceRevision,
			readmeBytes,
			specificationBytes,
			predecessorPath,
		)
		if prepareErr != nil {
			return nil, prepareErr
		}
		if copyDigest != predecessorDigest {
			return nil, fmt.Errorf(
				"%w: build %d copied digest %q, initial digest %q",
				ErrCandidatePredecessorChanged,
				ordinal,
				copyDigest,
				predecessorDigest,
			)
		}
		if buildErr := input.Builder.BuildIndex(ctx, workspace.input); buildErr != nil {
			return nil, fmt.Errorf("build candidate index pass %d: %w", ordinal, buildErr)
		}
		if verifyErr := verifyCandidateBuildWorkspace(
			workspace,
			readmeBytes,
			specificationBytes,
		); verifyErr != nil {
			return nil, fmt.Errorf("verify candidate index pass %d: %w", ordinal, verifyErr)
		}
		builds = append(builds, workspace)
	}

	finalPredecessorDigest, err := digestFile(predecessorPath)
	if err != nil {
		return nil, fmt.Errorf("re-digest predecessor candidate basis: %w", err)
	}
	if finalPredecessorDigest != predecessorDigest {
		return nil, fmt.Errorf(
			"%w: final digest %q, initial digest %q",
			ErrCandidatePredecessorChanged,
			finalPredecessorDigest,
			predecessorDigest,
		)
	}
	identical, err := candidateFilesIdentical(
		builds[0].databasePath,
		builds[1].databasePath,
	)
	if err != nil {
		return nil, fmt.Errorf("compare candidate database builds: %w", err)
	}
	if !identical {
		return nil, ErrCandidateNonDeterministic
	}

	sourceDiagnostics, err := verifyGitSourceDerivedProjection(
		builds[0].databasePath,
		input.Source,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"verify candidate source and TypeEnv projection: %w",
			err,
		)
	}
	if err := VerifySourceQueryRuntime(builds[0].databasePath); err != nil {
		return nil, fmt.Errorf("verify candidate source Query runtime: %w", err)
	}
	queryResults, err := VerifyCandidateQueryContract(builds[0].databasePath)
	querySmokeFailure := err
	lockInput := IntegrationCoordinateInput{
		SourceRevision: sourceRevision,
		ReadmePath:     builds[0].readmePath,
		SpecPath:       builds[0].specificationPath,
		DatabasePath:   builds[0].databasePath,
		GeneratedBy:    generatedBy,
		TokenGate:      cloneTokenGateCoordinates(input.TokenGate),
	}
	integrationLock, err := BuildIntegrationLock(lockInput)
	if err != nil {
		return nil, fmt.Errorf("build candidate integration lock: %w", err)
	}
	if err := VerifyIntegrationLock(integrationLock, lockInput); err != nil {
		return nil, fmt.Errorf("verify candidate integration lock: %w", err)
	}
	lockPath := candidateAbsolutePath(
		builds[0].workspacePath,
		candidateLogicalLockPath,
	)
	if err := WriteIntegrationLock(lockPath, integrationLock); err != nil {
		return nil, fmt.Errorf("write canonical candidate integration lock: %w", err)
	}
	lockDigest, err := verifyCandidateLockFile(lockPath, integrationLock)
	if err != nil {
		return nil, fmt.Errorf("verify canonical candidate integration lock bytes: %w", err)
	}
	if err := verifyCandidateDatabaseFile(builds[0].databasePath); err != nil {
		return nil, fmt.Errorf("verify retained candidate database: %w", err)
	}

	if err := os.RemoveAll(builds[1].workspacePath); err != nil {
		return nil, fmt.Errorf("remove deterministic comparison workspace: %w", err)
	}
	artifact = &CandidateArtifact{
		source:            input.Source,
		ownedRootPath:     ownedRootPath,
		workspacePath:     builds[0].workspacePath,
		readmePath:        builds[0].readmePath,
		specificationPath: builds[0].specificationPath,
		databasePath:      builds[0].databasePath,
		lockPath:          lockPath,
		lockDigest:        lockDigest,
		integrationLock:   integrationLock,
		querySmokeResults: cloneCandidateQuerySmokeResults(queryResults),
		querySmokeFailure: querySmokeFailure,
		sourceDiagnostics: cloneCandidateSourceGrammarDiagnostics(sourceDiagnostics),
	}
	rootTransferred = true
	return artifact, nil
}

type candidateBuildWorkspace struct {
	workspacePath     string
	readmePath        string
	specificationPath string
	databasePath      string
	input             IndexBuildInput
}

func validateCandidateSourceSnapshot(
	source GitSourceSnapshot,
) (string, []byte, []byte, error) {
	revision, err := normalizeCommitSHA(source.CommitSHA())
	if err != nil {
		return "", nil, nil, fmt.Errorf("candidate source revision: %w", err)
	}
	if revision != source.CommitSHA() {
		return "", nil, nil, fmt.Errorf(
			"candidate source revision must be one canonical full commit SHA",
		)
	}
	readmeBytes := source.ReadmeBytes()
	if len(readmeBytes) == 0 {
		return "", nil, nil, fmt.Errorf("candidate source Readme.md is empty")
	}
	specificationBytes := source.SpecificationBytes()
	if len(specificationBytes) == 0 {
		return "", nil, nil, fmt.Errorf("candidate source FPF-Spec.md is empty")
	}
	return revision, readmeBytes, specificationBytes, nil
}

func validateCandidateGeneratedBy(value string) (string, error) {
	if value == "" ||
		value != strings.TrimSpace(value) ||
		strings.ContainsAny(value, "\x00\r\n") {
		return "", fmt.Errorf(
			"candidate lock generated_by must be exact, non-empty, and single-line",
		)
	}
	return value, nil
}

func validateCandidatePredecessorPath(path string) (string, error) {
	if path == "" || path != strings.TrimSpace(path) {
		return "", fmt.Errorf("predecessor database path must be exact and non-empty")
	}
	absolutePath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("resolve predecessor database path: %w", err)
	}
	info, err := os.Stat(absolutePath)
	if err != nil {
		return "", fmt.Errorf("inspect predecessor database: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("predecessor database must be one regular file")
	}
	return absolutePath, nil
}

func prepareCandidateBuildWorkspace(
	ownedRootPath string,
	ordinal int,
	sourceRevision string,
	readmeBytes []byte,
	specificationBytes []byte,
	predecessorDatabasePath string,
) (candidateBuildWorkspace, string, error) {
	workspacePath := filepath.Join(ownedRootPath, fmt.Sprintf("build-%d", ordinal))
	if err := os.Mkdir(workspacePath, 0o700); err != nil {
		return candidateBuildWorkspace{}, "", fmt.Errorf(
			"create candidate build %d workspace: %w",
			ordinal,
			err,
		)
	}
	readmePath := candidateAbsolutePath(workspacePath, candidateLogicalReadmePath)
	specificationPath := candidateAbsolutePath(workspacePath, candidateLogicalSpecPath)
	databasePath := candidateAbsolutePath(workspacePath, candidateLogicalDBPath)
	if err := writeCandidateSourceFile(readmePath, readmeBytes); err != nil {
		return candidateBuildWorkspace{}, "", fmt.Errorf(
			"materialize candidate build %d Readme.md: %w",
			ordinal,
			err,
		)
	}
	if err := writeCandidateSourceFile(specificationPath, specificationBytes); err != nil {
		return candidateBuildWorkspace{}, "", fmt.Errorf(
			"materialize candidate build %d FPF-Spec.md: %w",
			ordinal,
			err,
		)
	}
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o700); err != nil {
		return candidateBuildWorkspace{}, "", fmt.Errorf(
			"create candidate build %d database directory: %w",
			ordinal,
			err,
		)
	}
	if err := copyCandidatePredecessorDatabase(
		predecessorDatabasePath,
		databasePath,
	); err != nil {
		return candidateBuildWorkspace{}, "", fmt.Errorf(
			"copy predecessor database for candidate build %d: %w",
			ordinal,
			err,
		)
	}
	copyDigest, err := digestFile(databasePath)
	if err != nil {
		return candidateBuildWorkspace{}, "", fmt.Errorf(
			"digest predecessor copy for candidate build %d: %w",
			ordinal,
			err,
		)
	}
	return candidateBuildWorkspace{
		workspacePath:     workspacePath,
		readmePath:        readmePath,
		specificationPath: specificationPath,
		databasePath:      databasePath,
		input: IndexBuildInput{
			WorkingDirectory:  workspacePath,
			ReadmePath:        candidateLogicalReadmePath,
			SpecificationPath: candidateLogicalSpecPath,
			DatabasePath:      candidateLogicalDBPath,
			SourceRevision:    sourceRevision,
		},
	}, copyDigest, nil
}

func candidateAbsolutePath(workspacePath string, logicalPath string) string {
	return filepath.Join(workspacePath, filepath.FromSlash(logicalPath))
}

func writeCandidateSourceFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	removeOnFailure := true
	defer func() {
		_ = file.Close()
		if removeOnFailure {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(content); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	removeOnFailure = false
	return nil
}

func copyCandidatePredecessorDatabase(sourcePath string, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()
	sourceInfo, err := source.Stat()
	if err != nil {
		return err
	}
	if !sourceInfo.Mode().IsRegular() {
		return fmt.Errorf("predecessor database is not a regular file")
	}
	target, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	removeOnFailure := true
	defer func() {
		_ = target.Close()
		if removeOnFailure {
			_ = os.Remove(targetPath)
		}
	}()
	if _, err := io.Copy(target, source); err != nil {
		return err
	}
	if err := target.Sync(); err != nil {
		return err
	}
	if err := target.Close(); err != nil {
		return err
	}
	removeOnFailure = false
	return nil
}

func verifyCandidateBuildWorkspace(
	workspace candidateBuildWorkspace,
	readmeBytes []byte,
	specificationBytes []byte,
) error {
	if err := verifyCandidateSourceFile(workspace.readmePath, readmeBytes); err != nil {
		return fmt.Errorf("verify Readme.md: %w", err)
	}
	if err := verifyCandidateSourceFile(
		workspace.specificationPath,
		specificationBytes,
	); err != nil {
		return fmt.Errorf("verify FPF-Spec.md: %w", err)
	}
	return verifyCandidateDatabaseFile(workspace.databasePath)
}

func verifyCandidateSourceFile(path string, expected []byte) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("materialized source is not a regular file")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(content, expected) {
		return fmt.Errorf("materialized source bytes changed during build")
	}
	return nil
}

func verifyCandidateDatabaseFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("candidate database is not one non-empty regular file")
	}
	for _, suffix := range []string{"-journal", "-shm", "-wal"} {
		sidecarPath := path + suffix
		_, sidecarErr := os.Lstat(sidecarPath)
		if sidecarErr == nil {
			return fmt.Errorf("candidate database left SQLite sidecar %s", filepath.Base(sidecarPath))
		}
		if !errors.Is(sidecarErr, os.ErrNotExist) {
			return fmt.Errorf("inspect candidate database sidecar %s: %w", sidecarPath, sidecarErr)
		}
	}
	return nil
}

func verifyCandidateLockFile(
	path string,
	integrationLock IntegrationLock,
) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return "", fmt.Errorf("candidate integration lock is not one non-empty regular file")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	canonical, err := MarshalIntegrationLock(integrationLock)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(content, canonical) {
		return "", fmt.Errorf("candidate integration lock differs from canonical generated bytes")
	}
	parsed, err := ParseIntegrationLock(content)
	if err != nil {
		return "", err
	}
	parsedCanonical, err := MarshalIntegrationLock(parsed)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(parsedCanonical, canonical) {
		return "", fmt.Errorf("candidate integration lock did not round-trip canonically")
	}
	digest, err := digestFile(path)
	if err != nil {
		return "", err
	}
	return digest, nil
}

func candidateFilesIdentical(firstPath string, secondPath string) (bool, error) {
	first, err := os.Open(firstPath)
	if err != nil {
		return false, err
	}
	defer func() { _ = first.Close() }()
	second, err := os.Open(secondPath)
	if err != nil {
		return false, err
	}
	defer func() { _ = second.Close() }()
	firstInfo, err := first.Stat()
	if err != nil {
		return false, err
	}
	secondInfo, err := second.Stat()
	if err != nil {
		return false, err
	}
	if firstInfo.Size() != secondInfo.Size() {
		return false, nil
	}
	firstBuffer := make([]byte, 128<<10)
	secondBuffer := make([]byte, len(firstBuffer))
	for {
		firstCount, firstErr := io.ReadFull(first, firstBuffer)
		secondCount, secondErr := io.ReadFull(second, secondBuffer)
		if firstCount != secondCount ||
			!bytes.Equal(firstBuffer[:firstCount], secondBuffer[:secondCount]) {
			return false, nil
		}
		firstDone := errors.Is(firstErr, io.EOF) ||
			errors.Is(firstErr, io.ErrUnexpectedEOF)
		secondDone := errors.Is(secondErr, io.EOF) ||
			errors.Is(secondErr, io.ErrUnexpectedEOF)
		if firstDone || secondDone {
			if firstDone && secondDone {
				return true, nil
			}
			return false, nil
		}
		if firstErr != nil {
			return false, firstErr
		}
		if secondErr != nil {
			return false, secondErr
		}
	}
}

func cloneCandidateQuerySmokeResults(values []QuerySmokeResult) []QuerySmokeResult {
	cloned := make([]QuerySmokeResult, len(values))
	for index, value := range values {
		cloned[index] = value
		cloned[index].UnitIDs = append([]string{}, value.UnitIDs...)
	}
	return cloned
}

func cloneCandidateSourceGrammarDiagnostics(
	diagnostics []fpf.SourceGrammarDiagnostic,
) []fpf.SourceGrammarDiagnostic {
	cloned := make([]fpf.SourceGrammarDiagnostic, len(diagnostics))
	for index, diagnostic := range diagnostics {
		diagnostic.LabelsDiscovered = append(
			[]string(nil),
			diagnostic.LabelsDiscovered...,
		)
		diagnostic.LabelsRecognized = append(
			[]string(nil),
			diagnostic.LabelsRecognized...,
		)
		cloned[index] = diagnostic
	}
	return cloned
}

func removeCandidateArtifactRoot(path string) error {
	cleanPath := filepath.Clean(path)
	if path == "" ||
		path != cleanPath ||
		!filepath.IsAbs(path) ||
		!strings.HasPrefix(filepath.Base(path), candidateTemporaryPrefix) ||
		filepath.Dir(path) == path {
		return fmt.Errorf("candidate cleanup root %q is not an owned private root", path)
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove candidate artifact root %s: %w", path, err)
	}
	return nil
}
