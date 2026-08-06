package fpfrefresh

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareApplyReceiptRejectsPredecessorLockDriftAfterCheck(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name            string
		checkedPresence ReceiptLockPresence
		mutate          func(*testing.T, *refreshEffectsFixture)
		assertLiveState func(*testing.T, *refreshEffectsFixture)
	}{
		{
			name:            "changed",
			checkedPresence: ReceiptLockPresent,
			mutate: func(t *testing.T, fixture *refreshEffectsFixture) {
				copyEffectsFile(t, fixture.candidateLock, fixture.targetLock)
			},
			assertLiveState: func(t *testing.T, fixture *refreshEffectsFixture) {
				assertFileDigest(t, fixture.targetLock, fixture.candidateLockDigest)
			},
		},
		{
			name:            "removed",
			checkedPresence: ReceiptLockPresent,
			mutate: func(t *testing.T, fixture *refreshEffectsFixture) {
				if err := os.Remove(fixture.targetLock); err != nil {
					t.Fatal(err)
				}
			},
			assertLiveState: func(t *testing.T, fixture *refreshEffectsFixture) {
				assertTransactionPathMissing(t, fixture.targetLock)
			},
		},
		{
			name:            "appeared",
			checkedPresence: ReceiptLockMissing,
			mutate: func(t *testing.T, fixture *refreshEffectsFixture) {
				copyEffectsFile(t, fixture.predecessorLockBackup, fixture.targetLock)
			},
			assertLiveState: func(t *testing.T, fixture *refreshEffectsFixture) {
				assertFileDigest(t, fixture.targetLock, fixture.predecessorLockDigest)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRefreshEffectsFixture(t)
			fixture.installPredecessorPair(t, test.checkedPresence)
			layout := transactionTestLayout(t, fixture)
			check := transactionTestCandidateCheck(t, fixture, layout)

			if got := check.checkedPredecessorLockIdentity.presence; got != test.checkedPresence {
				t.Fatalf(
					"checked predecessor lock presence = %q, want %q",
					got,
					test.checkedPresence,
				)
			}
			if test.checkedPresence == ReceiptLockPresent &&
				check.checkedPredecessorLockIdentity.digest != fixture.predecessorLockDigest {
				t.Fatalf(
					"checked predecessor lock digest = %s, want %s",
					check.checkedPredecessorLockIdentity.digest,
					fixture.predecessorLockDigest,
				)
			}

			test.mutate(t, fixture)
			_, err := PrepareApplyReceipt(context.Background(), layout, check)
			if !errors.Is(err, ErrReceiptStale) {
				t.Fatalf(
					"PrepareApplyReceipt() error = %v, want ErrReceiptStale",
					err,
				)
			}

			assertTransactionPathMissing(t, layout.Receipt)
			assertTransactionPathMissing(
				t,
				filepath.Join(layout.StateDirectory, "artifacts"),
			)
			test.assertLiveState(t, fixture)
			assertFileDigest(t, layout.Database, fixture.predecessorDatabaseDigest)
			assertFileDigest(
				t,
				check.CandidateArtifact.DatabasePath(),
				fixture.candidateDatabaseDigest,
			)
			assertFileDigest(
				t,
				check.CandidateArtifact.LockPath(),
				fixture.candidateLockDigest,
			)
		})
	}
}

func TestPrepareApplyReceiptUsesCheckedPredecessorLockDigest(t *testing.T) {
	t.Parallel()

	fixture := newRefreshEffectsFixture(t)
	layout := transactionTestLayout(t, fixture)
	check := transactionTestCandidateCheck(t, fixture, layout)
	checked := check.checkedPredecessorLockIdentity
	if checked.presence != ReceiptLockPresent {
		t.Fatalf("checked predecessor lock presence = %q, want present", checked.presence)
	}

	receipt, err := PrepareApplyReceipt(context.Background(), layout, check)
	if err != nil {
		t.Fatalf("PrepareApplyReceipt() error = %v", err)
	}
	predecessorLock := receipt.Basis().Artifacts.PredecessorLock
	if predecessorLock.Presence != ReceiptLockPresent {
		t.Fatalf(
			"prepared predecessor lock presence = %q, want present",
			predecessorLock.Presence,
		)
	}
	if predecessorLock.Digest != checked.digest {
		t.Fatalf(
			"prepared predecessor lock digest = %s, want checked %s",
			predecessorLock.Digest,
			checked.digest,
		)
	}
	assertFileDigest(t, predecessorLock.BackupPath, checked.digest)
	if !strings.Contains(
		filepath.Base(predecessorLock.BackupPath),
		strings.TrimPrefix(checked.digest, "sha256:"),
	) {
		t.Fatalf(
			"predecessor lock backup %q omits checked digest %s",
			predecessorLock.BackupPath,
			checked.digest,
		)
	}
	loaded, err := LoadReceipt(layout.Receipt)
	if err != nil {
		t.Fatalf("LoadReceipt() error = %v", err)
	}
	if got := loaded.Basis().Artifacts.PredecessorLock.Digest; got != checked.digest {
		t.Fatalf("published predecessor lock digest = %s, want %s", got, checked.digest)
	}
}

func TestPrepareApplyReceiptReturnsBusyWithoutMutation(t *testing.T) {
	t.Parallel()

	fixture := newRefreshEffectsFixture(t)
	layout := transactionTestLayout(t, fixture)
	check := transactionTestCandidateCheck(t, fixture, layout)
	sourceBefore, err := CheckedOutSourceRevision(context.Background(), layout)
	if err != nil {
		t.Fatal(err)
	}

	release, err := acquireOperationLock(layout.Receipt)
	if err != nil {
		t.Fatalf("acquireOperationLock() error = %v", err)
	}
	defer release()
	stateBefore := transactionTestTree(t, layout.StateDirectory)

	_, err = PrepareApplyReceipt(context.Background(), layout, check)
	if !errors.Is(err, ErrReceiptBusy) {
		t.Fatalf("PrepareApplyReceipt() error = %v, want ErrReceiptBusy", err)
	}

	if stateAfter := transactionTestTree(t, layout.StateDirectory); stateAfter != stateBefore {
		t.Fatalf(
			"refresh state tree changed while operation lock was held:\nbefore:\n%s\nafter:\n%s",
			stateBefore,
			stateAfter,
		)
	}
	assertTransactionPathMissing(t, layout.Receipt)
	assertTransactionPathMissing(
		t,
		filepath.Join(layout.StateDirectory, "artifacts"),
	)
	assertFileDigest(t, layout.Database, fixture.predecessorDatabaseDigest)
	assertFileDigest(t, layout.IntegrationLock, fixture.predecessorLockDigest)
	assertFileDigest(
		t,
		check.CandidateArtifact.DatabasePath(),
		fixture.candidateDatabaseDigest,
	)
	assertFileDigest(
		t,
		check.CandidateArtifact.LockPath(),
		fixture.candidateLockDigest,
	)
	sourceAfter, err := CheckedOutSourceRevision(context.Background(), layout)
	if err != nil {
		t.Fatal(err)
	}
	if sourceAfter != sourceBefore {
		t.Fatalf(
			"source revision changed while operation lock was held: %s -> %s",
			sourceBefore,
			sourceAfter,
		)
	}
}

func TestApplyCheckedCandidateCompletesSerializedReceiptLifecycle(t *testing.T) {
	t.Parallel()

	fixture := newRefreshEffectsFixture(t)
	layout := transactionTestLayout(t, fixture)
	check := transactionTestCandidateCheck(t, fixture, layout)

	result, err := ApplyCheckedCandidate(context.Background(), ApplyCandidateRequest{
		Layout:                         layout,
		Check:                          check,
		AllowReviewReadyTechnicalApply: true,
	})
	if err != nil {
		t.Fatalf("ApplyCheckedCandidate() error = %v", err)
	}
	if result.TerminalReceipt.State != ReceiptStateComplete {
		t.Fatalf(
			"terminal receipt state = %s, want %s",
			result.TerminalReceipt.State,
			ReceiptStateComplete,
		)
	}
	if result.ReceiptArchivePath == "" {
		t.Fatal("completed receipt archive path is empty")
	}
	if _, err := os.Stat(result.ReceiptArchivePath); err != nil {
		t.Fatalf("inspect completed receipt archive: %v", err)
	}
	assertTransactionPathMissing(t, layout.Receipt)
	if !check.CandidateArtifact.Cleaned() {
		t.Fatal("transferred candidate artifact was not cleaned")
	}
	assertCandidateEffectsPair(t, fixture)
	assertCandidateRepositoryIntegration(t, fixture)
}

func TestCheckCandidatePreservesReportWhenWorkspaceCleanupFails(t *testing.T) {
	t.Parallel()

	fixture := newRefreshEffectsFixture(t)
	layout := transactionTestLayout(t, fixture)
	cleanupFailure := errors.New("injected predecessor workspace cleanup failure")
	var predecessorWorkspace string
	builder := IndexBuilderFunc(func(
		_ context.Context,
		input IndexBuildInput,
	) error {
		content, err := os.ReadFile(fixture.candidateDatabase)
		if err != nil {
			return err
		}
		return os.WriteFile(
			candidateAbsolutePath(input.WorkingDirectory, input.DatabasePath),
			content,
			0o600,
		)
	})
	check, err := CheckCandidate(context.Background(), CandidateCheckRequest{
		Layout:       layout,
		CandidateRef: fixture.candidateSHA,
		Builder:      builder,
		ToolRevision: "fpf-refresh-test",
		TokenGateVerifier: CandidateTokenGateFunc(func(
			context.Context,
			CandidateTokenGateInput,
		) error {
			return nil
		}),
		removeWorkspace: func(path string) error {
			predecessorWorkspace = path
			return cleanupFailure
		},
	})
	if !errors.Is(err, cleanupFailure) {
		t.Fatalf("CheckCandidate() error = %v, want cleanup failure", err)
	}
	if predecessorWorkspace == "" {
		t.Fatal("predecessor workspace cleanup was not attempted")
	}
	t.Cleanup(func() { _ = os.RemoveAll(predecessorWorkspace) })
	if err := check.Report.Verify(); err != nil {
		t.Fatalf("cleanup failure erased report: %v", err)
	}
	if check.CandidateArtifact == nil {
		t.Fatal("cleanup failure erased candidate artifact ownership")
	}
	t.Cleanup(func() {
		if err := check.Cleanup(); err != nil {
			t.Errorf("CandidateCheckResult.Cleanup() error = %v", err)
		}
	})
}

func TestCheckCandidateKeepsVerifiedArtifactWhenTokenGateNeedsReview(t *testing.T) {
	t.Parallel()

	fixture := newRefreshEffectsFixture(t)
	layout := transactionTestLayout(t, fixture)
	builder := IndexBuilderFunc(func(
		_ context.Context,
		input IndexBuildInput,
	) error {
		content, err := os.ReadFile(fixture.candidateDatabase)
		if err != nil {
			return err
		}
		databasePath := candidateAbsolutePath(input.WorkingDirectory, input.DatabasePath)
		return os.WriteFile(databasePath, content, 0o600)
	})
	tokenGateFailure := errors.New("frozen query behavior changed on fresh source")
	ctx := context.Background()
	check, err := CheckCandidate(ctx, CandidateCheckRequest{
		Layout:       layout,
		CandidateRef: fixture.candidateSHA,
		Builder:      builder,
		ToolRevision: "fpf-refresh-test",
		TokenGateVerifier: CandidateTokenGateFunc(func(
			context.Context,
			CandidateTokenGateInput,
		) error {
			return tokenGateFailure
		}),
	})
	if err != nil {
		t.Fatalf("CheckCandidate() error = %v", err)
	}
	state := check.Report.Outcome().State()
	if state != StateReviewReady {
		t.Fatalf(
			"token-gate outcome = %s, want %s",
			state.String(),
			StateReviewReady.String(),
		)
	}
	if check.CandidateArtifact == nil {
		t.Fatal("token-gate review discarded the verified candidate artifact")
	}
	diagnostics := check.Report.Diagnostics()
	if len(diagnostics) != 1 || diagnostics[0].Code() != DiagnosticTokenGateFailed {
		t.Fatalf("token-gate diagnostics = %#v", diagnostics)
	}
	result, err := ApplyCheckedCandidate(ctx, ApplyCandidateRequest{
		Layout:                         layout,
		Check:                          check,
		AllowReviewReadyTechnicalApply: true,
	})
	if err != nil {
		t.Fatalf("ApplyCheckedCandidate() error = %v", err)
	}
	if result.TerminalReceipt.State != ReceiptStateComplete {
		t.Fatalf("terminal receipt state = %s", result.TerminalReceipt.State)
	}
	assertCandidateEffectsPair(t, fixture)
	assertCandidateRepositoryIntegration(t, fixture)
}

func TestDurableLockArtifactNameSeparatesSameSourceLockRevisions(t *testing.T) {
	t.Parallel()

	const sourceRevision = "308edacfa2bdb2c60d07e4e10c0deb1f260a6a31"
	firstDigest := "sha256:" + strings.Repeat("a", 64)
	secondDigest := "sha256:" + strings.Repeat("b", 64)

	first := durableLockArtifactName("candidate", sourceRevision, firstDigest)
	second := durableLockArtifactName("candidate", sourceRevision, secondDigest)

	if first == second {
		t.Fatal("same-source lock revisions collided in durable artifact storage")
	}
	for _, part := range []string{
		"candidate",
		sourceRevision,
		strings.TrimPrefix(firstDigest, "sha256:"),
	} {
		if !strings.Contains(first, part) {
			t.Fatalf("durable lock artifact %q omits identity component %q", first, part)
		}
	}
}

func transactionTestLayout(
	t *testing.T,
	fixture *refreshEffectsFixture,
) RepositoryLayout {
	t.Helper()
	layout, err := ResolveRepositoryLayout(fixture.root)
	if err != nil {
		t.Fatalf("ResolveRepositoryLayout() error = %v", err)
	}
	localPracticeCandidate, err := filepath.Abs(filepath.Join(
		"..",
		"..",
		DefaultLocalPracticeCandidateRelative,
	))
	if err != nil {
		t.Fatalf("resolve production Local-Practice candidate: %v", err)
	}
	layout.LatestLocalPracticeCandidate = localPracticeCandidate
	return layout
}

func transactionTestCandidateCheck(
	t *testing.T,
	fixture *refreshEffectsFixture,
	layout RepositoryLayout,
) CandidateCheckResult {
	t.Helper()
	builder := IndexBuilderFunc(func(
		_ context.Context,
		input IndexBuildInput,
	) error {
		content, err := os.ReadFile(fixture.candidateDatabase)
		if err != nil {
			return err
		}
		return os.WriteFile(
			candidateAbsolutePath(input.WorkingDirectory, input.DatabasePath),
			content,
			0o600,
		)
	})
	check, err := CheckCandidate(context.Background(), CandidateCheckRequest{
		Layout:       layout,
		CandidateRef: fixture.candidateSHA,
		Builder:      builder,
		ToolRevision: "fpf-refresh-test",
		TokenGateVerifier: CandidateTokenGateFunc(func(
			context.Context,
			CandidateTokenGateInput,
		) error {
			return nil
		}),
	})
	if err != nil {
		t.Fatalf("CheckCandidate() error = %v", err)
	}
	if check.CandidateArtifact == nil {
		t.Fatalf(
			"CheckCandidate() outcome = %s without candidate artifact",
			check.Report.Outcome().State().String(),
		)
	}
	t.Cleanup(func() {
		if err := check.Cleanup(); err != nil {
			t.Errorf("CandidateCheckResult.Cleanup() error = %v", err)
		}
	})
	return check
}

func assertTransactionPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s exists or cannot be inspected: %v", path, err)
	}
}

func transactionTestTree(t *testing.T, root string) string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(root, func(
		path string,
		entry os.DirEntry,
		err error,
	) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if entry.IsDir() {
			relative += "/"
		}
		entries = append(entries, relative)
		return nil
	})
	if err != nil {
		t.Fatalf("walk refresh state tree: %v", err)
	}
	return strings.Join(entries, "\n")
}
