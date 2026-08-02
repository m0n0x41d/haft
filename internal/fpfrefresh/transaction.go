package fpfrefresh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ApplyCandidateRequest makes the ReviewReady technical-apply exception
// explicit. It does not approve semantics, bind a decision, rebaseline a spec,
// activate a ProjectTypeEnvHead, commit, install, or release.
type ApplyCandidateRequest struct {
	Layout                         RepositoryLayout
	Check                          CandidateCheckResult
	AllowReviewReadyTechnicalApply bool
}

type ApplyCandidateResult struct {
	ReceiptArchivePath string
	TerminalReceipt    ApplyReceipt
}

// ApplyCheckedCandidate transfers exact prepared bytes into a durable receipt,
// executes that receipt, and archives it only after candidate-pair verification.
func ApplyCheckedCandidate(
	ctx context.Context,
	request ApplyCandidateRequest,
) (ApplyCandidateResult, error) {
	if request.Check.CandidateArtifact == nil {
		return ApplyCandidateResult{}, fmt.Errorf("candidate check has no prepared artifact")
	}
	state := request.Check.Report.Outcome().State()
	switch state {
	case StateApplyReady:
	case StateReviewReady:
		if !request.AllowReviewReadyTechnicalApply {
			return ApplyCandidateResult{}, fmt.Errorf(
				"candidate is ReviewReady; inspect the report and explicitly request the bounded technical apply",
			)
		}
	default:
		return ApplyCandidateResult{}, fmt.Errorf(
			"candidate state %s cannot be applied",
			state.String(),
		)
	}
	release, err := acquireOperationLock(request.Layout.Receipt)
	if err != nil {
		return ApplyCandidateResult{}, err
	}
	defer release()
	if err := WriteCompatibilityReport(
		request.Layout.Report,
		request.Check.Report,
	); err != nil {
		return ApplyCandidateResult{}, err
	}
	_, err = prepareApplyReceiptLocked(ctx, request.Layout, request.Check)
	cleanupErr := request.Check.CandidateArtifact.Cleanup()
	if err != nil {
		return ApplyCandidateResult{}, errors.Join(err, cleanupErr)
	}
	if cleanupErr != nil {
		return ApplyCandidateResult{}, fmt.Errorf(
			"candidate receipt is prepared but temporary artifact cleanup failed: %w",
			cleanupErr,
		)
	}
	terminal, err := executeReceiptRecoveryLocked(
		ctx,
		request.Layout.Receipt,
		false,
	)
	if err != nil {
		return ApplyCandidateResult{}, err
	}
	if terminal.State != ReceiptStateComplete {
		return ApplyCandidateResult{}, fmt.Errorf(
			"%w: resume ended in %s",
			ErrRecoveryRequired,
			terminal.State,
		)
	}
	archivePath, err := archiveTerminalReceiptLocked(
		request.Layout.Receipt,
		filepath.Join(request.Layout.StateDirectory, "receipts"),
	)
	if err != nil {
		return ApplyCandidateResult{}, err
	}
	return ApplyCandidateResult{
		ReceiptArchivePath: archivePath,
		TerminalReceipt:    terminal,
	}, nil
}

// PrepareApplyReceipt copies every mutable byte source into a bounded durable
// artifact directory before publishing the prepared receipt.
func PrepareApplyReceipt(
	ctx context.Context,
	layout RepositoryLayout,
	check CandidateCheckResult,
) (ApplyReceipt, error) {
	release, err := acquireOperationLock(layout.Receipt)
	if err != nil {
		return ApplyReceipt{}, err
	}
	defer release()
	return prepareApplyReceiptLocked(ctx, layout, check)
}

func prepareApplyReceiptLocked(
	ctx context.Context,
	layout RepositoryLayout,
	check CandidateCheckResult,
) (ApplyReceipt, error) {
	if ctx == nil {
		return ApplyReceipt{}, fmt.Errorf("apply preparation context is required")
	}
	artifact := check.CandidateArtifact
	if artifact == nil {
		return ApplyReceipt{}, fmt.Errorf("apply preparation requires a candidate artifact")
	}
	if err := check.Report.Verify(); err != nil {
		return ApplyReceipt{}, err
	}
	if err := verifyCheckedPredecessorLockIdentity(
		layout.IntegrationLock,
		check.checkedPredecessorLockIdentity,
	); err != nil {
		return ApplyReceipt{}, fmt.Errorf(
			"verify checked predecessor integration lock before apply: %w",
			err,
		)
	}
	candidateLock := artifact.IntegrationLock()
	candidateCoordinates := candidateLock.Coordinates
	if err := verifyRepositoryTokenGateFixture(
		layout.TokenGateFixture,
		candidateLock.TokenGate,
	); err != nil {
		return ApplyReceipt{}, fmt.Errorf(
			"verify token-gate fixture before apply: %w",
			err,
		)
	}
	reportCandidate, complete := check.Report.Candidate().Derived()
	if !complete ||
		reportCandidate.DatabaseDigest().String() != candidateCoordinates.DatabaseDigest ||
		check.Report.Candidate().Source().Revision().String() != candidateCoordinates.SourceRevision {
		return ApplyReceipt{}, fmt.Errorf(
			"candidate report and retained artifact coordinates differ",
		)
	}
	predecessor := check.Report.Predecessor()
	predecessorSourceSHA := predecessor.Source().Revision().String()
	predecessorDatabaseDigest := predecessor.Derived().DatabaseDigest().String()
	if err := verifyRegularFileDigest(layout.Database, predecessorDatabaseDigest); err != nil {
		return ApplyReceipt{}, fmt.Errorf("verify predecessor database before apply: %w", err)
	}
	initialSourceSHA, err := exactGitRevisionAt(ctx, layout.SourceRepository)
	if err != nil {
		return ApplyReceipt{}, err
	}
	if initialSourceSHA != predecessorSourceSHA &&
		initialSourceSHA != candidateCoordinates.SourceRevision {
		return ApplyReceipt{}, fmt.Errorf(
			"%w: checked-out source %s is neither predecessor %s nor candidate %s",
			ErrReceiptStale,
			initialSourceSHA,
			predecessorSourceSHA,
			candidateCoordinates.SourceRevision,
		)
	}
	status, err := runRepositoryGit(
		ctx,
		layout.SourceRepository,
		"status",
		"--porcelain=v1",
		"--untracked-files=all",
	)
	if err != nil {
		return ApplyReceipt{}, err
	}
	if strings.TrimSpace(string(status)) != "" {
		return ApplyReceipt{}, fmt.Errorf(
			"FPF source checkout has unrelated dirt; apply refuses to publish a receipt:\n%s",
			strings.TrimSpace(string(status)),
		)
	}

	artifactDirectory := filepath.Join(layout.StateDirectory, "artifacts")
	if err := os.MkdirAll(artifactDirectory, 0o700); err != nil {
		return ApplyReceipt{}, fmt.Errorf("create durable refresh artifact directory: %w", err)
	}
	candidateDatabasePath := filepath.Join(
		artifactDirectory,
		"candidate-"+candidateCoordinates.SourceRevision+"-"+
			strings.TrimPrefix(candidateCoordinates.DatabaseDigest, "sha256:")+".db",
	)
	candidateLockPath := filepath.Join(
		artifactDirectory,
		durableLockArtifactName(
			"candidate",
			candidateCoordinates.SourceRevision,
			artifact.LockDigest(),
		),
	)
	predecessorDatabasePath := filepath.Join(
		artifactDirectory,
		"predecessor-"+predecessorSourceSHA+"-"+
			strings.TrimPrefix(predecessorDatabaseDigest, "sha256:")+".db",
	)
	if err := persistExactArtifact(
		artifact.DatabasePath(),
		candidateDatabasePath,
		candidateCoordinates.DatabaseDigest,
	); err != nil {
		return ApplyReceipt{}, err
	}
	if err := persistExactArtifact(
		artifact.LockPath(),
		candidateLockPath,
		artifact.LockDigest(),
	); err != nil {
		return ApplyReceipt{}, err
	}
	if err := persistExactArtifact(
		layout.Database,
		predecessorDatabasePath,
		predecessorDatabaseDigest,
	); err != nil {
		return ApplyReceipt{}, err
	}

	predecessorLock := ReceiptPredecessorLock{Presence: ReceiptLockMissing}
	var predecessorLockSnapshot *IntegrationLock
	switch check.checkedPredecessorLockIdentity.presence {
	case ReceiptLockMissing:
	case ReceiptLockPresent:
		lockDigest := check.checkedPredecessorLockIdentity.digest
		predecessorLockPath := filepath.Join(
			artifactDirectory,
			durableLockArtifactName(
				"predecessor",
				predecessorSourceSHA,
				lockDigest,
			),
		)
		if err := persistExactArtifact(
			layout.IntegrationLock,
			predecessorLockPath,
			lockDigest,
		); err != nil {
			return ApplyReceipt{}, err
		}
		predecessorLock = ReceiptPredecessorLock{
			Presence:   ReceiptLockPresent,
			BackupPath: predecessorLockPath,
			Digest:     lockDigest,
		}
		payload, readErr := os.ReadFile(predecessorLockPath)
		if readErr != nil {
			return ApplyReceipt{}, fmt.Errorf(
				"read durable predecessor integration lock: %w",
				readErr,
			)
		}
		parsed, parseErr := ParseIntegrationLock(payload)
		if parseErr != nil {
			return ApplyReceipt{}, fmt.Errorf(
				"parse durable predecessor integration lock: %w",
				parseErr,
			)
		}
		predecessorLockSnapshot = &parsed
	default:
		return ApplyReceipt{}, fmt.Errorf(
			"checked predecessor lock presence %q is not defined",
			check.checkedPredecessorLockIdentity.presence,
		)
	}
	var candidateTokenGateFixturePath string
	var candidateTokenGateFixtureDigest string
	var predecessorTokenGateFixturePresence ReceiptLockPresence
	var predecessorTokenGateFixturePath string
	var predecessorTokenGateFixtureDigest string
	var tokenGateFixtureTarget string
	if candidateLock.TokenGate != nil {
		tokenGateFixtureTarget = layout.TokenGateFixture
		candidateTokenGateFixtureDigest = candidateLock.TokenGate.FixtureDigest
		candidateTokenGateFixturePath = filepath.Join(
			artifactDirectory,
			"candidate-token-gate-"+
				strings.TrimPrefix(candidateTokenGateFixtureDigest, "sha256:")+
				".json",
		)
		if err := persistExactArtifact(
			layout.TokenGateFixture,
			candidateTokenGateFixturePath,
			candidateTokenGateFixtureDigest,
		); err != nil {
			return ApplyReceipt{}, err
		}
		predecessorTokenGateFixturePresence = ReceiptLockMissing
		if predecessorLockSnapshot != nil {
			if predecessorLockSnapshot.TokenGate != nil {
				predecessorTokenGateFixturePresence = ReceiptLockPresent
				predecessorTokenGateFixtureDigest =
					predecessorLockSnapshot.TokenGate.FixtureDigest
				predecessorTokenGateFixturePath = filepath.Join(
					artifactDirectory,
					"predecessor-token-gate-"+
						strings.TrimPrefix(
							predecessorTokenGateFixtureDigest,
							"sha256:",
						)+".json",
				)
				if err := persistPredecessorTokenGateFixture(
					ctx,
					layout,
					predecessorTokenGateFixturePath,
					predecessorTokenGateFixtureDigest,
				); err != nil {
					return ApplyReceipt{}, err
				}
			}
		}
	}
	basis := ReceiptBasis{
		Predecessor: ReceiptCoordinates{
			SourceSHA:      predecessorSourceSHA,
			DatabaseDigest: predecessorDatabaseDigest,
		},
		Candidate: ReceiptCoordinates{
			SourceSHA:      candidateCoordinates.SourceRevision,
			DatabaseDigest: candidateCoordinates.DatabaseDigest,
		},
		InitialSourceSHA: initialSourceSHA,
		Targets: ReceiptTargets{
			SourcePath:           layout.SourceRepository,
			DatabasePath:         layout.Database,
			LockPath:             layout.IntegrationLock,
			TokenGateFixturePath: tokenGateFixtureTarget,
		},
		Artifacts: ReceiptArtifacts{
			CandidateDatabasePath:               candidateDatabasePath,
			CandidateLockPath:                   candidateLockPath,
			CandidateLockDigest:                 artifact.LockDigest(),
			PredecessorDatabaseBackupPath:       predecessorDatabasePath,
			PredecessorLock:                     predecessorLock,
			CandidateTokenGateFixturePath:       candidateTokenGateFixturePath,
			CandidateTokenGateFixtureDigest:     candidateTokenGateFixtureDigest,
			PredecessorTokenGateFixturePresence: predecessorTokenGateFixturePresence,
			PredecessorTokenGateFixturePath:     predecessorTokenGateFixturePath,
			PredecessorTokenGateFixtureDigest:   predecessorTokenGateFixtureDigest,
		},
	}
	receipt, err := NewApplyReceipt(basis)
	if err != nil {
		return ApplyReceipt{}, err
	}
	if err := verifyApplyPreparationBasis(
		ctx,
		layout,
		initialSourceSHA,
		predecessorDatabaseDigest,
		check.checkedPredecessorLockIdentity,
		candidateLock.TokenGate,
	); err != nil {
		return ApplyReceipt{}, err
	}
	if err := CreateReceipt(layout.Receipt, receipt); err != nil {
		return ApplyReceipt{}, err
	}
	return receipt, nil
}

func verifyApplyPreparationBasis(
	ctx context.Context,
	layout RepositoryLayout,
	initialSourceSHA string,
	predecessorDatabaseDigest string,
	predecessorLockIdentity checkedPredecessorLockIdentity,
	candidateTokenGate *TokenGateCoordinates,
) error {
	if err := verifyRegularFileDigest(
		layout.Database,
		predecessorDatabaseDigest,
	); err != nil {
		return fmt.Errorf(
			"predecessor database changed before receipt publication: %w",
			err,
		)
	}
	observedSourceSHA, err := exactGitRevisionAt(ctx, layout.SourceRepository)
	if err != nil {
		return err
	}
	if observedSourceSHA != initialSourceSHA {
		return fmt.Errorf(
			"%w: source revision changed from %s to %s before receipt publication",
			ErrReceiptStale,
			initialSourceSHA,
			observedSourceSHA,
		)
	}
	if err := VerifySourceCheckoutClean(ctx, layout); err != nil {
		return fmt.Errorf(
			"source checkout changed before receipt publication: %w",
			err,
		)
	}
	if err := verifyCheckedPredecessorLockIdentity(
		layout.IntegrationLock,
		predecessorLockIdentity,
	); err != nil {
		return fmt.Errorf(
			"predecessor integration lock changed before receipt publication: %w",
			err,
		)
	}
	if err := verifyRepositoryTokenGateFixture(
		layout.TokenGateFixture,
		candidateTokenGate,
	); err != nil {
		return fmt.Errorf(
			"token-gate fixture changed before receipt publication: %w",
			err,
		)
	}
	return nil
}

func persistPredecessorTokenGateFixture(
	ctx context.Context,
	layout RepositoryLayout,
	targetPath string,
	expectedDigest string,
) error {
	if actual, exists, err := optionalRegularFileDigest(targetPath); err != nil {
		return err
	} else if exists {
		if actual != expectedDigest {
			return fmt.Errorf(
				"durable predecessor token-gate artifact %s has digest %s, want %s",
				targetPath,
				actual,
				expectedDigest,
			)
		}
		return nil
	}
	if current, exists, err := optionalRegularFileDigest(
		layout.TokenGateFixture,
	); err != nil {
		return err
	} else if exists && current == expectedDigest {
		return persistExactArtifact(
			layout.TokenGateFixture,
			targetPath,
			expectedDigest,
		)
	}
	payload, err := runRepositoryGit(
		ctx,
		layout.Root,
		"show",
		"HEAD:"+DefaultTokenGateFixtureRelativePath,
	)
	if err != nil {
		return fmt.Errorf(
			"recover predecessor token-gate fixture %s from root HEAD: %w",
			expectedDigest,
			err,
		)
	}
	if digestBytesSHA256(payload) != expectedDigest {
		return fmt.Errorf(
			"root HEAD token-gate fixture does not match predecessor digest %s",
			expectedDigest,
		)
	}
	if err := writeExclusiveFile(targetPath, payload, 0o600); err != nil {
		return err
	}
	return verifyRegularFileDigest(targetPath, expectedDigest)
}

func durableLockArtifactName(role string, sourceRevision string, digest string) string {
	return role + "-" + sourceRevision + "-" +
		strings.TrimPrefix(digest, "sha256:") +
		"-integration.lock.json"
}

func persistExactArtifact(sourcePath string, targetPath string, digest string) error {
	if err := verifyRegularFileDigest(sourcePath, digest); err != nil {
		return err
	}
	actual, exists, err := optionalRegularFileDigest(targetPath)
	if err != nil {
		return err
	}
	if exists {
		if actual != digest {
			return fmt.Errorf(
				"durable refresh artifact %s exists with digest %s, want %s",
				targetPath,
				actual,
				digest,
			)
		}
		return nil
	}
	return copyFileExclusive(sourcePath, targetPath, 0o600, digest)
}

func copyFileExclusive(
	sourcePath string,
	targetPath string,
	mode os.FileMode,
	expectedDigest string,
) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()
	target, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		_ = target.Close()
		if !keep {
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
	if err := verifyRegularFileDigest(targetPath, expectedDigest); err != nil {
		return err
	}
	keep = true
	return syncDirectory(filepath.Dir(targetPath))
}
