package fpfrefresh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

var candidateTemporaryPathPattern = regexp.MustCompile(
	`(?:/[^[:space:]]*)?/` + candidateTemporaryPrefix + `[^/[:space:]]+(?:/[^[:space:]]*)?`,
)

// CandidateCheckRequest supplies all effectful adapters explicitly. The pure
// parser/report policies remain inside their domain packages.
type CandidateCheckRequest struct {
	Layout            RepositoryLayout
	CandidateRef      string
	Fetch             *GitFetchRequest
	Builder           IndexBuilder
	ToolRevision      string
	TokenGate         *TokenGateCoordinates
	TokenGateVerifier CandidateTokenGate
	removeWorkspace   func(string) error
}

// CandidateCheckResult owns a prepared artifact only for a changed, verified
// candidate. Call Cleanup when the artifact is not transferred into an apply
// receipt.
type CandidateCheckResult struct {
	Report                         Report
	CandidateArtifact              *CandidateArtifact
	PredecessorSource              GitSourceSnapshot
	CandidateSource                GitSourceSnapshot
	ExecutionTimings               []StageTiming
	checkedPredecessorLockIdentity checkedPredecessorLockIdentity
}

// checkedPredecessorLockIdentity is the exact lock state admitted by
// CheckCandidate. Apply preparation must revalidate this identity rather than
// discover a new predecessor from the mutable repository path.
type checkedPredecessorLockIdentity struct {
	presence ReceiptLockPresence
	digest   string
}

func (result *CandidateCheckResult) Cleanup() error {
	if result == nil || result.CandidateArtifact == nil {
		return nil
	}
	return result.CandidateArtifact.Cleanup()
}

// CheckCandidate builds and compares one exact candidate without changing the
// source checkout, current DB, generated lock, candidate carriers, or specs.
func CheckCandidate(
	ctx context.Context,
	request CandidateCheckRequest,
) (checkResult CandidateCheckResult, resultErr error) {
	if ctx == nil {
		return CandidateCheckResult{}, fmt.Errorf("candidate check context is required")
	}
	if request.Builder == nil {
		return CandidateCheckResult{}, fmt.Errorf("candidate check index builder is required")
	}
	if request.TokenGateVerifier == nil {
		return CandidateCheckResult{}, fmt.Errorf(
			"candidate check token-gate verifier is required",
		)
	}
	if strings.TrimSpace(request.ToolRevision) == "" {
		return CandidateCheckResult{}, fmt.Errorf("candidate check tool revision is required")
	}
	candidateRef := strings.TrimSpace(request.CandidateRef)
	if candidateRef == "" {
		candidateRef = DefaultCandidateRef
	}
	if err := os.MkdirAll(request.Layout.StateDirectory, 0o700); err != nil {
		return CandidateCheckResult{}, fmt.Errorf("create FPF refresh state directory: %w", err)
	}
	recovery, err := InspectRecovery(request.Layout.Receipt)
	if err != nil {
		return CandidateCheckResult{}, err
	}
	if recovery.Required {
		return CandidateCheckResult{}, fmt.Errorf(
			"%w: receipt=%s state=%s; resume or restore that exact receipt",
			ErrRecoveryRequired,
			request.Layout.Receipt,
			recovery.Receipt.State,
		)
	}
	if recovery.Receipt.Schema != "" {
		if _, err := ArchiveTerminalReceipt(
			request.Layout.Receipt,
			filepath.Join(request.Layout.StateDirectory, "receipts"),
		); err != nil {
			return CandidateCheckResult{}, err
		}
	}

	predecessorDatabaseDigest, err := digestFile(request.Layout.Database)
	if err != nil {
		return CandidateCheckResult{}, err
	}
	predecessorWorkspace, err := os.MkdirTemp(
		"",
		candidateTemporaryPrefix+"predecessor-*",
	)
	if err != nil {
		return CandidateCheckResult{}, err
	}
	removeWorkspace := request.removeWorkspace
	if removeWorkspace == nil {
		removeWorkspace = os.RemoveAll
	}
	defer func() {
		resultErr = joinCandidateCheckCleanupError(
			resultErr,
			"clean captured predecessor workspace",
			removeWorkspace(predecessorWorkspace),
		)
	}()
	predecessorDatabasePath := filepath.Join(predecessorWorkspace, "predecessor.db")
	if err := copyFileExclusive(
		request.Layout.Database,
		predecessorDatabasePath,
		0o600,
		predecessorDatabaseDigest,
	); err != nil {
		return CandidateCheckResult{}, fmt.Errorf(
			"capture immutable predecessor database: %w",
			err,
		)
	}
	predecessorRevision, err := DatabaseSourceRevision(predecessorDatabasePath)
	if err != nil {
		return CandidateCheckResult{}, err
	}
	verifyPredecessorDatabaseStillCurrent := func() error {
		if err := verifyRegularFileDigest(
			request.Layout.Database,
			predecessorDatabaseDigest,
		); err != nil {
			return fmt.Errorf(
				"predecessor database changed during candidate check: %w",
				err,
			)
		}
		return nil
	}
	sourceReadStarted := time.Now()
	predecessorSource, err := AcquireGitSource(ctx, GitSourceRequest{
		RepositoryPath: request.Layout.SourceRepository,
		CandidateRef:   predecessorRevision,
	})
	if err != nil {
		return CandidateCheckResult{}, fmt.Errorf("acquire predecessor source: %w", err)
	}
	predecessorLock, err := BuildIntegrationLockFromBytes(
		IntegrationByteCoordinateInput{
			SourceRevision: predecessorSource.CommitSHA(),
			ReadmeBytes:    predecessorSource.ReadmeBytes(),
			SpecBytes:      predecessorSource.SpecificationBytes(),
			DatabasePath:   predecessorDatabasePath,
			GeneratedBy:    request.ToolRevision,
			TokenGate:      request.TokenGate,
		},
	)
	if err != nil {
		return CandidateCheckResult{}, fmt.Errorf("verify predecessor source/database basis: %w", err)
	}
	existingLock, predecessorLockIdentity, err := verifyExistingRepositoryLock(
		request.Layout.IntegrationLock,
		predecessorSource,
		predecessorDatabasePath,
	)
	if err != nil {
		return CandidateCheckResult{}, err
	}
	if shouldVerifyPredecessorProjection(existingLock, request.ToolRevision) {
		if _, err := verifyGitSourceDerivedProjection(
			predecessorDatabasePath,
			predecessorSource,
		); err != nil {
			return CandidateCheckResult{}, fmt.Errorf(
				"verify predecessor source-derived projection: %w",
				err,
			)
		}
	}
	verifyPredecessorStillCurrent := func() error {
		if err := verifyPredecessorDatabaseStillCurrent(); err != nil {
			return err
		}
		if err := verifyCheckedPredecessorLockIdentity(
			request.Layout.IntegrationLock,
			predecessorLockIdentity,
		); err != nil {
			return fmt.Errorf(
				"predecessor integration lock changed during candidate check: %w",
				err,
			)
		}
		return nil
	}
	tokenGateDeltas, err := tokenGateCompatibilityDeltas(existingLock, request.TokenGate)
	if err != nil {
		return CandidateCheckResult{}, err
	}
	toolchainDeltas, err := derivationToolCompatibilityDeltas(
		existingLock,
		request.ToolRevision,
	)
	if err != nil {
		return CandidateCheckResult{}, err
	}
	additionalDeltas := make([]Delta, 0, len(tokenGateDeltas)+len(toolchainDeltas))
	additionalDeltas = append(additionalDeltas, tokenGateDeltas...)
	additionalDeltas = append(additionalDeltas, toolchainDeltas...)
	sourceReadTiming, err := NewStageTiming(
		StageSourceObjectRead,
		time.Since(sourceReadStarted),
	)
	if err != nil {
		return CandidateCheckResult{}, err
	}

	candidateReadStarted := time.Now()
	candidateSource, candidateErr := AcquireGitSource(ctx, GitSourceRequest{
		RepositoryPath: request.Layout.SourceRepository,
		CandidateRef:   candidateRef,
		Fetch:          request.Fetch,
	})
	fetchTiming, err := NewStageTiming(StageFetch, time.Since(candidateReadStarted))
	if err != nil {
		return CandidateCheckResult{}, err
	}
	timings := []StageTiming{fetchTiming, sourceReadTiming}
	if candidateErr != nil {
		if candidateSource.CommitSHA() == "" ||
			(!errors.Is(candidateErr, ErrGitSourceMissing) &&
				!errors.Is(candidateErr, ErrGitSourceMalformed)) {
			return CandidateCheckResult{}, fmt.Errorf(
				"acquire candidate source: %w",
				candidateErr,
			)
		}
		report, reportErr := rejectedSourceAcquisitionReport(
			request,
			predecessorLock.Coordinates,
			candidateSource,
			candidateErr,
			additionalDeltas,
		)
		if reportErr != nil {
			return CandidateCheckResult{}, errors.Join(candidateErr, reportErr)
		}
		return CandidateCheckResult{
			Report:                         report,
			PredecessorSource:              predecessorSource,
			CandidateSource:                candidateSource,
			ExecutionTimings:               timings,
			checkedPredecessorLockIdentity: predecessorLockIdentity,
		}, verifyPredecessorStillCurrent()
	}

	if canReuseVerifiedPredecessor(
		predecessorSource.CommitSHA(),
		candidateSource.CommitSHA(),
		additionalDeltas,
	) {
		if err := VerifySourceQueryRuntime(predecessorDatabasePath); err != nil {
			return CandidateCheckResult{}, err
		}
		report, err := BuildCompatibilityReport(CompatibilityReportInput{
			ToolRevision:                 request.ToolRevision,
			Predecessor:                  predecessorLock.Coordinates,
			Candidate:                    predecessorLock.Coordinates,
			PredecessorDatabasePath:      predecessorDatabasePath,
			CandidateDatabasePath:        predecessorDatabasePath,
			LatestLocalPracticeCandidate: request.Layout.LatestLocalPracticeCandidate,
			AdditionalDeltas:             additionalDeltas,
		})
		if err != nil {
			return CandidateCheckResult{}, err
		}
		result := CandidateCheckResult{
			Report:                         report,
			PredecessorSource:              predecessorSource,
			CandidateSource:                candidateSource,
			ExecutionTimings:               timings,
			checkedPredecessorLockIdentity: predecessorLockIdentity,
		}
		return result, verifyPredecessorStillCurrent()
	}

	buildStarted := time.Now()
	artifact, buildErr := PrepareCandidateArtifact(
		ctx,
		CandidatePreparationInput{
			Source:                  candidateSource,
			PredecessorDatabasePath: predecessorDatabasePath,
			Builder:                 request.Builder,
			GeneratedBy:             request.ToolRevision,
			TokenGate:               request.TokenGate,
		},
	)
	buildTiming, timingErr := NewStageTiming(StageSQLiteBuild, time.Since(buildStarted))
	if timingErr != nil {
		return CandidateCheckResult{}, cleanupCandidateArtifact(artifact, timingErr)
	}
	timings = append(timings, buildTiming)
	if buildErr != nil {
		report, reportErr := rejectedCandidateReport(
			request,
			predecessorLock.Coordinates,
			candidateSource,
			buildErr,
			additionalDeltas,
		)
		if reportErr != nil {
			return CandidateCheckResult{}, errors.Join(buildErr, reportErr)
		}
		result := CandidateCheckResult{
			Report:                         report,
			PredecessorSource:              predecessorSource,
			CandidateSource:                candidateSource,
			ExecutionTimings:               timings,
			checkedPredecessorLockIdentity: predecessorLockIdentity,
		}
		return result, verifyPredecessorStillCurrent()
	}

	tokenGateStarted := time.Now()
	tokenGateErr := request.TokenGateVerifier.VerifyCandidate(
		ctx,
		CandidateTokenGateInput{
			DatabasePath:        artifact.DatabasePath(),
			IntegrationLockPath: artifact.LockPath(),
			FixturePath:         request.Layout.TokenGateFixture,
		},
	)
	tokenGateTiming, timingErr := NewStageTiming(
		StageTokenGate,
		time.Since(tokenGateStarted),
	)
	if timingErr != nil {
		return CandidateCheckResult{}, cleanupCandidateArtifact(artifact, timingErr)
	}
	timings = append(timings, tokenGateTiming)
	reviewDiagnostics, diagnosticErr := candidateSourceGrammarReviewDiagnostics(
		candidateSource.CommitSHA(),
		artifact.sourceGrammarDiagnostics(),
	)
	if diagnosticErr != nil {
		return CandidateCheckResult{}, cleanupCandidateArtifact(
			artifact,
			diagnosticErr,
		)
	}
	structureDiagnostic, collapsed, diagnosticErr := candidateSourceStructureDiagnostic(
		predecessorLock.Coordinates.SourceUnitCount,
		artifact.IntegrationLock().Coordinates.SourceUnitCount,
		candidateSource.CommitSHA(),
	)
	if diagnosticErr != nil {
		return CandidateCheckResult{}, cleanupCandidateArtifact(
			artifact,
			diagnosticErr,
		)
	}
	if collapsed {
		reviewDiagnostics = append(reviewDiagnostics, structureDiagnostic)
	}
	querySmokeErr := artifact.querySmokeError()
	if querySmokeErr != nil {
		diagnostic, diagnosticErr := NewDiagnostic(
			DiagnosticQueryContractRegression,
			"candidate "+candidateSource.CommitSHA()+" source-specific Query expectations",
			sanitizeCandidateDiagnostic(querySmokeErr.Error()),
			"internal/fpfrefresh/query_verify.go",
			"go test -count=1 ./internal/fpfrefresh -run TestVerifyCandidateQueryContractAgainstCurrentProductionSource",
		)
		if diagnosticErr != nil {
			return CandidateCheckResult{}, cleanupCandidateArtifact(
				artifact,
				errors.Join(querySmokeErr, diagnosticErr),
			)
		}
		reviewDiagnostics = append(reviewDiagnostics, diagnostic)
	}
	if tokenGateErr != nil {
		// Query behavior is source-dependent. Preserve exact drift as review
		// evidence without discarding an otherwise complete verified candidate.
		diagnostic, diagnosticErr := NewDiagnostic(
			DiagnosticTokenGateFailed,
			"candidate "+candidateSource.CommitSHA()+" query/token acceptance",
			sanitizeCandidateDiagnostic(tokenGateErr.Error()),
			DefaultTokenGateFixtureRelativePath,
			"bash scripts/fpf_query_token_gate.sh",
		)
		if diagnosticErr != nil {
			return CandidateCheckResult{}, cleanupCandidateArtifact(
				artifact,
				errors.Join(tokenGateErr, diagnosticErr),
			)
		}
		reviewDiagnostics = append(reviewDiagnostics, diagnostic)
	}

	comparisonStarted := time.Now()
	report, err := BuildCompatibilityReport(CompatibilityReportInput{
		ToolRevision:                 request.ToolRevision,
		Predecessor:                  predecessorLock.Coordinates,
		Candidate:                    artifact.IntegrationLock().Coordinates,
		PredecessorDatabasePath:      predecessorDatabasePath,
		CandidateDatabasePath:        artifact.DatabasePath(),
		LatestLocalPracticeCandidate: request.Layout.LatestLocalPracticeCandidate,
		AdditionalDeltas:             additionalDeltas,
		AdditionalDiagnostics:        reviewDiagnostics,
		// Measured timings live in the execution envelope returned above. They
		// are intentionally excluded from canonical compatibility bytes so
		// identical source/artifact inputs produce byte-identical reports.
		Timings: nil,
	})
	comparisonTiming, timingErr := NewStageTiming(
		StageCompatibilityComparison,
		time.Since(comparisonStarted),
	)
	if timingErr != nil {
		return CandidateCheckResult{}, cleanupCandidateArtifact(artifact, timingErr)
	}
	timings = append(timings, comparisonTiming)
	if err != nil {
		return CandidateCheckResult{}, cleanupCandidateArtifact(artifact, err)
	}
	result := CandidateCheckResult{
		Report:                         report,
		CandidateArtifact:              artifact,
		PredecessorSource:              predecessorSource,
		CandidateSource:                candidateSource,
		ExecutionTimings:               timings,
		checkedPredecessorLockIdentity: predecessorLockIdentity,
	}
	if err := verifyPredecessorStillCurrent(); err != nil {
		return result, cleanupCandidateArtifact(artifact, err)
	}
	return result, nil
}

func cleanupCandidateArtifact(artifact *CandidateArtifact, primaryErr error) error {
	if artifact == nil {
		return primaryErr
	}
	cleanupErr := artifact.Cleanup()
	return joinCandidateCheckCleanupError(
		primaryErr,
		"clean checked candidate artifact",
		cleanupErr,
	)
}

func joinCandidateCheckCleanupError(
	primaryErr error,
	action string,
	cleanupErr error,
) error {
	if cleanupErr == nil {
		return primaryErr
	}
	return errors.Join(primaryErr, fmt.Errorf("%s: %w", action, cleanupErr))
}

func canReuseVerifiedPredecessor(
	predecessorRevision string,
	candidateRevision string,
	rebuildDeltas []Delta,
) bool {
	return candidateRevision == predecessorRevision && len(rebuildDeltas) == 0
}

// A predecessor produced by the current derivation tool must reproduce under
// that exact tool. When the verified integration lock names an older tool,
// rebuilding the predecessor with the successor parser would confuse an
// intentional compiler migration with repository corruption. The old lock
// still binds the immutable source bytes, database bytes, source revision,
// schema, unit count, and Base TypeEnv; the candidate is rebuilt twice and
// verified under the successor tool before it can be applied.
func shouldVerifyPredecessorProjection(
	predecessor *IntegrationLock,
	candidateToolRevision string,
) bool {
	if predecessor == nil {
		return true
	}
	return predecessor.GeneratedBy == candidateToolRevision
}

func verifyExistingRepositoryLock(
	path string,
	source GitSourceSnapshot,
	databasePath string,
) (*IntegrationLock, checkedPredecessorLockIdentity, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, checkedPredecessorLockIdentity{
			presence: ReceiptLockMissing,
		}, nil
	}
	if err != nil {
		return nil, checkedPredecessorLockIdentity{}, fmt.Errorf(
			"open existing FPF integration lock: %w",
			err,
		)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, checkedPredecessorLockIdentity{}, fmt.Errorf(
			"stat existing FPF integration lock: %w",
			err,
		)
	}
	const maximumIntegrationLockBytes = 64 << 10
	if !info.Mode().IsRegular() ||
		info.Size() <= 0 ||
		info.Size() > maximumIntegrationLockBytes {
		return nil, checkedPredecessorLockIdentity{}, fmt.Errorf(
			"existing FPF integration lock must be a non-empty regular file of at most %d bytes",
			maximumIntegrationLockBytes,
		)
	}
	payload, err := io.ReadAll(io.LimitReader(file, maximumIntegrationLockBytes+1))
	if err != nil {
		return nil, checkedPredecessorLockIdentity{}, fmt.Errorf(
			"read existing FPF integration lock: %w",
			err,
		)
	}
	if len(payload) > maximumIntegrationLockBytes {
		return nil, checkedPredecessorLockIdentity{}, fmt.Errorf(
			"existing FPF integration lock exceeds %d bytes",
			maximumIntegrationLockBytes,
		)
	}
	lock, err := ParseIntegrationLock(payload)
	if err != nil {
		return nil, checkedPredecessorLockIdentity{}, err
	}
	if err := VerifyIntegrationLockFromBytes(
		lock,
		IntegrationByteCoordinateInput{
			SourceRevision: source.CommitSHA(),
			ReadmeBytes:    source.ReadmeBytes(),
			SpecBytes:      source.SpecificationBytes(),
			DatabasePath:   databasePath,
			GeneratedBy:    lock.GeneratedBy,
			TokenGate:      lock.TokenGate,
		},
	); err != nil {
		return nil, checkedPredecessorLockIdentity{}, fmt.Errorf(
			"verify existing FPF integration lock: %w",
			err,
		)
	}
	return &lock, checkedPredecessorLockIdentity{
		presence: ReceiptLockPresent,
		digest:   digestBytesSHA256(payload),
	}, nil
}

func verifyCheckedPredecessorLockIdentity(
	path string,
	identity checkedPredecessorLockIdentity,
) error {
	digest, exists, err := optionalRegularFileDigest(path)
	if err != nil {
		return err
	}
	switch identity.presence {
	case ReceiptLockMissing:
		if identity.digest != "" {
			return fmt.Errorf("checked missing predecessor lock carries a digest")
		}
		if exists {
			return fmt.Errorf(
				"%w: predecessor integration lock appeared after check",
				ErrReceiptStale,
			)
		}
		return nil
	case ReceiptLockPresent:
		if !exactReceiptSHA256Digest.MatchString(identity.digest) {
			return fmt.Errorf("checked predecessor lock digest is not exact")
		}
		if !exists {
			return fmt.Errorf(
				"%w: checked predecessor integration lock is absent",
				ErrReceiptStale,
			)
		}
		if digest != identity.digest {
			return fmt.Errorf(
				"%w: predecessor integration lock digest %s, checked %s",
				ErrReceiptStale,
				digest,
				identity.digest,
			)
		}
		return nil
	default:
		return fmt.Errorf(
			"checked predecessor lock presence %q is not defined",
			identity.presence,
		)
	}
}

func tokenGateCompatibilityDeltas(
	predecessor *IntegrationLock,
	candidate *TokenGateCoordinates,
) ([]Delta, error) {
	var predecessorCoordinates *TokenGateCoordinates
	if predecessor != nil {
		predecessorCoordinates = predecessor.TokenGate
	}
	before := tokenGateCoordinateIdentity(predecessorCoordinates)
	after := tokenGateCoordinateIdentity(candidate)
	if before == after {
		return nil, nil
	}
	delta, err := NewDelta(
		DeltaTokenBudgetCorpus,
		DeltaTokenBudgetCorpusChanged,
		"token-gate behavior fixture identity",
		before,
		after,
		DefaultTokenGateFixtureRelativePath,
	)
	if err != nil {
		return nil, err
	}
	return []Delta{delta}, nil
}

func tokenGateCoordinateIdentity(coordinates *TokenGateCoordinates) string {
	if coordinates == nil {
		return "unbound"
	}
	return coordinates.FixtureRevision + "@" + coordinates.FixtureDigest
}

const refreshToolInputGraphSourceRef = "canonical-transitive-build-input-graph:cmd/fpf-refresh+cmd/indexer+nonstandard-dependencies+runtime-inputs"

func derivationToolCompatibilityDeltas(
	predecessor *IntegrationLock,
	candidateToolRevision string,
) ([]Delta, error) {
	before := "unbound"
	if predecessor != nil {
		before = predecessor.GeneratedBy
	}
	if before == candidateToolRevision {
		return nil, nil
	}
	delta, err := NewDelta(
		DeltaDerivationToolchain,
		DeltaDerivationToolChanged,
		"FPF derived-artifact generator identity",
		before,
		candidateToolRevision,
		refreshToolInputGraphSourceRef,
	)
	if err != nil {
		return nil, err
	}
	return []Delta{delta}, nil
}

func rejectedSourceAcquisitionReport(
	request CandidateCheckRequest,
	predecessorCoordinates IntegrationCoordinates,
	candidateSource GitSourceSnapshot,
	acquisitionErr error,
	additionalDeltas []Delta,
) (Report, error) {
	predecessor, err := predecessorReportSnapshot(predecessorCoordinates)
	if err != nil {
		return Report{}, err
	}
	revision, err := typedmemory.NewSourceRevision(candidateSource.CommitSHA())
	if err != nil {
		return Report{}, err
	}
	candidate, err := NewCandidateRevisionSnapshot(revision)
	if err != nil {
		return Report{}, err
	}
	diagnostic, err := NewDiagnostic(
		DiagnosticSourcePublicationMalformed,
		"candidate "+candidateSource.CommitSHA()+" source publication",
		sanitizeCandidateDiagnostic(acquisitionErr.Error()),
		candidateSource.CommitSHA()+":{Readme.md,FPF-Spec.md}",
		"go run ./cmd/fpf-refresh check --candidate-ref "+
			candidateSource.CommitSHA()+" --no-fetch",
	)
	if err != nil {
		return Report{}, err
	}
	toolRevision, err := typedmemory.NewSourceRevision(request.ToolRevision)
	if err != nil {
		return Report{}, err
	}
	return NewReport(
		toolRevision,
		predecessor,
		candidate,
		additionalDeltas,
		[]Diagnostic{diagnostic},
		nil,
	)
}

func rejectedCandidateReport(
	request CandidateCheckRequest,
	predecessorCoordinates IntegrationCoordinates,
	candidateSource GitSourceSnapshot,
	buildErr error,
	additionalDeltas []Delta,
) (Report, error) {
	predecessor, err := predecessorReportSnapshot(predecessorCoordinates)
	if err != nil {
		return Report{}, err
	}
	revision, err := typedmemory.NewSourceRevision(candidateSource.CommitSHA())
	if err != nil {
		return Report{}, err
	}
	readmeDigest, err := typedmemory.NewSHA256Digest(
		digestBytesSHA256(candidateSource.ReadmeBytes()),
	)
	if err != nil {
		return Report{}, err
	}
	specDigest, err := typedmemory.NewSHA256Digest(
		digestBytesSHA256(candidateSource.SpecificationBytes()),
	)
	if err != nil {
		return Report{}, err
	}
	source, err := NewSourceCoordinates(revision, readmeDigest, specDigest)
	if err != nil {
		return Report{}, err
	}
	candidate, err := NewCandidateSourceSnapshot(source)
	if err != nil {
		return Report{}, err
	}
	code := classifyCandidateBuildDiagnostic(buildErr)
	message := sanitizeCandidateDiagnostic(buildErr.Error())
	diagnostic, err := NewDiagnostic(
		code,
		"candidate "+candidateSource.CommitSHA(),
		message,
		candidateLogicalSpecPath+"@"+candidateSource.CommitSHA(),
		"go run ./cmd/fpf-refresh check --candidate-ref "+candidateSource.CommitSHA()+" --no-fetch",
	)
	if err != nil {
		return Report{}, err
	}
	toolRevision, err := typedmemory.NewSourceRevision(request.ToolRevision)
	if err != nil {
		return Report{}, err
	}
	return NewReport(
		toolRevision,
		predecessor,
		candidate,
		additionalDeltas,
		[]Diagnostic{diagnostic},
		nil,
	)
}

func predecessorReportSnapshot(
	coordinates IntegrationCoordinates,
) (PredecessorSnapshot, error) {
	source, derived, err := reportCoordinates(coordinates)
	if err != nil {
		return PredecessorSnapshot{}, err
	}
	return NewPredecessorSnapshot(source, derived)
}

func classifyCandidateBuildDiagnostic(err error) DiagnosticCode {
	message := err.Error()
	switch {
	case strings.Contains(message, "source_publication_malformed"):
		return DiagnosticSourcePublicationMalformed
	case strings.Contains(message, "adapter_grammar_unsupported"):
		return DiagnosticCandidateVerificationFailed
	case strings.Contains(message, "source_reference_unresolved"):
		return DiagnosticSourceReferenceUnresolved
	case strings.Contains(message, "TypeEnv") &&
		strings.Contains(message, "rejected"):
		return DiagnosticTypeEnvSemanticRejection
	case strings.Contains(message, "compiler") &&
		strings.Contains(message, "unsupported"):
		return DiagnosticCandidateVerificationFailed
	case strings.Contains(message, "compiler version") &&
		strings.Contains(message, "neither current") &&
		strings.Contains(message, "known predecessor"):
		return DiagnosticCandidateVerificationFailed
	default:
		return DiagnosticCandidateVerificationFailed
	}
}

func candidateSourceGrammarReviewDiagnostics(
	candidateRevision string,
	observed []fpf.SourceGrammarDiagnostic,
) ([]Diagnostic, error) {
	diagnostics := make([]Diagnostic, 0, len(observed))
	for _, sourceDiagnostic := range observed {
		subject := "candidate " + candidateRevision +
			" practical-use card " + sourceDiagnostic.SourceID
		sourceRef := fmt.Sprintf(
			"%s:%d-%d",
			sourceDiagnostic.SourcePath,
			sourceDiagnostic.StartLine,
			sourceDiagnostic.EndLine,
		)
		code := DiagnosticSourceProjectionDegraded
		if sourceDiagnostic.Class == fpf.SourceGrammarUnsupported {
			code = DiagnosticAdapterGrammarUnsupported
		}
		diagnostic, err := NewDiagnostic(
			code,
			subject,
			sanitizeCandidateDiagnostic(sourceDiagnostic.Error()),
			sourceRef,
			"go test -count=1 ./internal/fpf -run 'PracticalUse|SourceUseCue|SourceGrammar|ProductionGrammar'",
		)
		if err != nil {
			return nil, err
		}
		diagnostics = append(diagnostics, diagnostic)
	}
	return diagnostics, nil
}

func candidateSourceStructureDiagnostic(
	predecessorCount int,
	candidateCount int,
	candidateRevision string,
) (Diagnostic, bool, error) {
	if predecessorCount <= 0 || candidateCount <= 0 {
		return Diagnostic{}, false, fmt.Errorf(
			"source structure comparison requires positive unit counts",
		)
	}
	predecessor := uint64(predecessorCount)
	candidate := uint64(candidateCount)
	if candidate*2 >= predecessor {
		return Diagnostic{}, false, nil
	}
	diagnostic, err := NewDiagnostic(
		DiagnosticSourceStructureCollapse,
		"candidate "+candidateRevision+" source-unit projection",
		fmt.Sprintf(
			"source-unit count collapsed from %d to %d; candidate retains less than 50%% of the previous structurally verified projection",
			predecessorCount,
			candidateCount,
		),
		candidateRevision+":{Readme.md,FPF-Spec.md}",
		"go run ./cmd/fpf-refresh check --candidate-ref "+candidateRevision+" --no-fetch",
	)
	if err != nil {
		return Diagnostic{}, false, err
	}
	return diagnostic, true, nil
}

func sanitizeCandidateDiagnostic(message string) string {
	message = candidateTemporaryPathPattern.ReplaceAllString(
		message,
		"<candidate-workspace>",
	)
	message = strings.Join(strings.Fields(message), " ")
	if len(message) <= maxMessageText {
		return message
	}
	const suffix = " [truncated]"
	limit := maxMessageText - len(suffix)
	for limit > 0 && !utf8.ValidString(message[:limit]) {
		limit--
	}
	return message[:limit] + suffix
}

// WriteCompatibilityReport publishes canonical report bytes without changing
// source, database, lock, candidate carriers, or specs.
func WriteCompatibilityReport(path string, report Report) error {
	if err := report.Verify(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	payload := append(report.CanonicalBytes(), '\n')
	return writeFileAtomicBytes(path, payload, 0o600)
}

func writeFileAtomicBytes(path string, content []byte, mode os.FileMode) error {
	parent := filepath.Dir(path)
	stage, err := os.CreateTemp(parent, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	stagePath := stage.Name()
	keep := false
	defer func() {
		_ = stage.Close()
		if !keep {
			_ = os.Remove(stagePath)
		}
	}()
	if err := stage.Chmod(mode); err != nil {
		return err
	}
	if _, err := stage.Write(content); err != nil {
		return err
	}
	if err := stage.Sync(); err != nil {
		return err
	}
	if err := stage.Close(); err != nil {
		return err
	}
	if err := os.Rename(stagePath, path); err != nil {
		return err
	}
	keep = true
	return syncDirectory(parent)
}
