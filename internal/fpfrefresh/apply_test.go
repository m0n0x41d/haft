package fpfrefresh

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExecuteReceiptResumeAcrossInterruptionStages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                         string
		state                        ReceiptState
		nextEffectComplete           bool
		initialCandidate             bool
		fullIntegrationPostcondition bool
	}{
		{
			name:                         "prepared",
			state:                        ReceiptStatePrepared,
			fullIntegrationPostcondition: true,
		},
		{
			name:               "prepared after source checkout before receipt advance",
			state:              ReceiptStatePrepared,
			nextEffectComplete: true,
		},
		{
			name:             "prepared with an already-candidate initial checkout",
			state:            ReceiptStatePrepared,
			initialCandidate: true,
		},
		{
			name:               "source applied after database rename before receipt advance",
			state:              ReceiptStateSourceApplied,
			nextEffectComplete: true,
		},
		{
			name:               "database applied after lock rename before receipt advance",
			state:              ReceiptStateDBApplied,
			nextEffectComplete: true,
		},
		{name: "lock applied", state: ReceiptStateLockApplied},
		{
			name:                         "verified",
			state:                        ReceiptStateVerified,
			fullIntegrationPostcondition: true,
		},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			fixture := newRefreshEffectsFixture(t)
			ctx := context.Background()
			basis := fixture.receiptBasis(
				ReceiptLockPresent,
				fixture.predecessorSHA,
			)
			protected := fixture.protectedFiles(t)
			caseBasis := basis
			if testCase.initialCandidate {
				caseBasis.InitialSourceSHA = fixture.candidateSHA
			}
			prepareEffectsReceipt(
				t,
				fixture,
				caseBasis,
				testCase.state,
				testCase.nextEffectComplete,
			)
			completed, err := ExecuteReceiptResume(ctx, fixture.receiptPath)
			if err != nil {
				t.Fatalf("ExecuteReceiptResume() error = %v", err)
			}
			if completed.State != ReceiptStateComplete {
				t.Fatalf("receipt state = %s, want complete", completed.State)
			}
			assertCandidateEffectsPair(t, fixture)
			// Every resume path performs the production candidate-pair and
			// query-contract verification before it marks the receipt complete.
			// The full source-derived integration postcondition is identical
			// for every final candidate byte set, so retain it at both state
			// endpoints without recompiling it for every intermediate state.
			if testCase.fullIntegrationPostcondition {
				assertCandidateRepositoryIntegration(t, fixture)
			}
			assertProtectedEffectsFiles(t, protected)
			assertNoApplyStages(t, fixture)

			status, err := InspectRecovery(fixture.receiptPath)
			if err != nil {
				t.Fatalf("InspectRecovery() error = %v", err)
			}
			if status.Required || status.Directions.Required {
				t.Fatalf("complete receipt still requires recovery: %#v", status)
			}
		})
	}

	// Both terminal operations are exact replays: neither the receipt nor any
	// target changes when a caller repeats resume or asks to restore a complete
	// refresh.
	t.Run("complete terminal replay", func(t *testing.T) {
		t.Parallel()

		fixture := newRefreshEffectsFixture(t)
		ctx := context.Background()
		basis := fixture.receiptBasis(
			ReceiptLockPresent,
			fixture.predecessorSHA,
		)
		prepareEffectsReceipt(t, fixture, basis, ReceiptStatePrepared, false)
		if _, err := ExecuteReceiptResume(ctx, fixture.receiptPath); err != nil {
			t.Fatal(err)
		}
		terminalReceipt := readEffectsFile(t, fixture.receiptPath)
		targetDatabaseDigest := effectsDigest(t, fixture.targetDatabase)
		targetLockDigest := effectsDigest(t, fixture.targetLock)
		for _, operation := range []struct {
			name string
			run  func(context.Context, string) (ApplyReceipt, error)
		}{
			{name: "resume", run: ExecuteReceiptResume},
			{name: "restore", run: ExecuteReceiptRestore},
		} {
			t.Run(operation.name, func(t *testing.T) {
				replayed, err := operation.run(ctx, fixture.receiptPath)
				if err != nil {
					t.Fatalf("terminal %s error = %v", operation.name, err)
				}
				if replayed.State != ReceiptStateComplete {
					t.Fatalf("terminal replay state = %s", replayed.State)
				}
				if got := readEffectsFile(t, fixture.receiptPath); got != terminalReceipt {
					t.Fatal("terminal replay rewrote receipt bytes")
				}
				assertFileDigest(t, fixture.targetDatabase, targetDatabaseDigest)
				assertFileDigest(t, fixture.targetLock, targetLockDigest)
			})
		}
	})
}

func TestExecuteReceiptRestoreHandlesPresentMissingAndMixedPredecessors(t *testing.T) {
	t.Parallel()

	fixture := newRefreshEffectsFixture(t)
	ctx := context.Background()

	for _, presence := range []ReceiptLockPresence{
		ReceiptLockPresent,
		ReceiptLockMissing,
	} {
		t.Run(string(presence), func(t *testing.T) {
			basis := fixture.receiptBasis(presence, fixture.predecessorSHA)
			prepareEffectsReceipt(
				t,
				fixture,
				basis,
				ReceiptStateDBApplied,
				true,
			)
			restored, err := ExecuteReceiptRestore(ctx, fixture.receiptPath)
			if err != nil {
				t.Fatalf("ExecuteReceiptRestore() error = %v", err)
			}
			if restored.State != ReceiptStateRestored {
				t.Fatalf("receipt state = %s, want restored", restored.State)
			}
			assertPredecessorEffectsPair(t, fixture, basis)

			terminalReceipt := readEffectsFile(t, fixture.receiptPath)
			for _, operation := range []struct {
				name string
				run  func(context.Context, string) (ApplyReceipt, error)
			}{
				{name: "resume", run: ExecuteReceiptResume},
				{name: "restore", run: ExecuteReceiptRestore},
			} {
				replayed, replayErr := operation.run(ctx, fixture.receiptPath)
				if replayErr != nil {
					t.Fatalf("restored terminal %s error = %v", operation.name, replayErr)
				}
				if replayed.State != ReceiptStateRestored {
					t.Fatalf("restored replay state = %s", replayed.State)
				}
				if got := readEffectsFile(t, fixture.receiptPath); got != terminalReceipt {
					t.Fatal("restored terminal replay rewrote receipt bytes")
				}
				assertPredecessorEffectsPair(t, fixture, basis)
			}
		})
	}

	t.Run("prepared mixed candidate checkout", func(t *testing.T) {
		basis := fixture.receiptBasis(ReceiptLockPresent, fixture.candidateSHA)
		prepareEffectsReceipt(t, fixture, basis, ReceiptStatePrepared, false)
		current, err := CheckedOutSourceRevision(
			ctx,
			RepositoryLayout{SourceRepository: fixture.sourceRepository},
		)
		if err != nil {
			t.Fatal(err)
		}
		if current != fixture.candidateSHA {
			t.Fatalf("mixed checkout = %s, want candidate %s", current, fixture.candidateSHA)
		}
		restored, err := ExecuteReceiptRestore(ctx, fixture.receiptPath)
		if err != nil {
			t.Fatalf("ExecuteReceiptRestore(mixed) error = %v", err)
		}
		if restored.State != ReceiptStateRestored {
			t.Fatalf("mixed restore state = %s", restored.State)
		}
		assertPredecessorEffectsPair(t, fixture, basis)
	})
}

func TestExecuteReceiptRecoveryTransactsTokenGateFixture(t *testing.T) {
	t.Parallel()

	fixture := newRefreshEffectsFixture(t)
	ctx := context.Background()
	targetFixture := filepath.Join(
		fixture.root,
		DefaultTokenGateFixtureRelativePath,
	)
	candidateFixture := filepath.Join(
		fixture.stateDirectory,
		"prepared",
		"candidate-token-gate.json",
	)
	predecessorFixture := filepath.Join(
		fixture.stateDirectory,
		"prepared",
		"predecessor-token-gate.json",
	)
	writeTokenFixture := func(path string, revision string) TokenGateCoordinates {
		t.Helper()
		payload := []byte(`{
  "schema_version": "haft.fpf-query-token-gate-corpus/v1",
  "fixture_revision": "` + revision + `",
  "cases": [{"case_id": "recovery-transaction"}]
}
`)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, payload, 0o600); err != nil {
			t.Fatal(err)
		}
		coordinates, err := ReadTokenGateCoordinates(path)
		if err != nil {
			t.Fatal(err)
		}
		return coordinates
	}
	predecessorToken := writeTokenFixture(
		predecessorFixture,
		"predecessor-fixture-v1",
	)
	candidateToken := writeTokenFixture(
		candidateFixture,
		"candidate-fixture-v2",
	)
	rewriteLock := func(
		path string,
		sourceRevision string,
		databasePath string,
		token TokenGateCoordinates,
	) {
		t.Helper()
		lock, err := BuildIntegrationLock(IntegrationCoordinateInput{
			SourceRevision: sourceRevision,
			ReadmePath: filepath.Join(
				fixture.sourceRepository,
				gitSourceReadmePath,
			),
			SpecPath: filepath.Join(
				fixture.sourceRepository,
				gitSourceSpecPath,
			),
			DatabasePath: databasePath,
			GeneratedBy:  "fpf-refresh-token-transaction-test",
			TokenGate:    &token,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := WriteIntegrationLock(path, lock); err != nil {
			t.Fatal(err)
		}
	}
	rewriteLock(
		fixture.predecessorLockBackup,
		fixture.predecessorSHA,
		fixture.predecessorDatabaseBackup,
		predecessorToken,
	)
	rewriteLock(
		fixture.candidateLock,
		fixture.candidateSHA,
		fixture.candidateDatabase,
		candidateToken,
	)
	fixture.predecessorLockDigest = effectsDigest(
		t,
		fixture.predecessorLockBackup,
	)
	fixture.candidateLockDigest = effectsDigest(t, fixture.candidateLock)

	basis := fixture.receiptBasis(ReceiptLockPresent, fixture.predecessorSHA)
	basis.Targets.TokenGateFixturePath = targetFixture
	basis.Artifacts.CandidateTokenGateFixturePath = candidateFixture
	basis.Artifacts.CandidateTokenGateFixtureDigest = candidateToken.FixtureDigest
	basis.Artifacts.PredecessorTokenGateFixturePresence = ReceiptLockPresent
	basis.Artifacts.PredecessorTokenGateFixturePath = predecessorFixture
	basis.Artifacts.PredecessorTokenGateFixtureDigest = predecessorToken.FixtureDigest

	t.Run("resume installs candidate fixture before candidate lock", func(t *testing.T) {
		prepareEffectsReceipt(
			t,
			fixture,
			basis,
			ReceiptStateDBApplied,
			false,
		)
		copyEffectsFile(t, predecessorFixture, targetFixture)
		receipt, err := LoadReceipt(fixture.receiptPath)
		if err != nil {
			t.Fatal(err)
		}
		directions, err := receipt.RecoveryDirections()
		if err != nil {
			t.Fatal(err)
		}
		if got := receiptRecoveryKinds(directions.Resume); !reflect.DeepEqual(
			got,
			[]ReceiptRecoveryStepKind{
				RecoveryApplyCandidateTokenGate,
				RecoveryMaterializeCandidateLock,
				RecoveryVerifyCandidatePair,
				RecoveryMarkReceiptComplete,
			},
		) {
			t.Fatalf("token-gate resume kinds = %#v", got)
		}
		completed, err := ExecuteReceiptResume(ctx, fixture.receiptPath)
		if err != nil {
			t.Fatalf("ExecuteReceiptResume() error = %v", err)
		}
		if completed.State != ReceiptStateComplete {
			t.Fatalf("resume state = %s, want complete", completed.State)
		}
		assertFileDigest(t, targetFixture, candidateToken.FixtureDigest)
		assertFileDigest(t, fixture.targetLock, fixture.candidateLockDigest)
		if err := verifyReceiptPair(ctx, basis, true); err != nil {
			t.Fatalf("verifyReceiptPair(candidate) error = %v", err)
		}
	})

	t.Run("restore reinstalls predecessor lock and fixture as one pair", func(t *testing.T) {
		prepareEffectsReceipt(
			t,
			fixture,
			basis,
			ReceiptStateLockApplied,
			false,
		)
		copyEffectsFile(t, candidateFixture, targetFixture)
		restored, err := ExecuteReceiptRestore(ctx, fixture.receiptPath)
		if err != nil {
			t.Fatalf("ExecuteReceiptRestore() error = %v", err)
		}
		if restored.State != ReceiptStateRestored {
			t.Fatalf("restore state = %s, want restored", restored.State)
		}
		assertFileDigest(t, targetFixture, predecessorToken.FixtureDigest)
		assertFileDigest(t, fixture.targetLock, fixture.predecessorLockDigest)
		if err := verifyReceiptPair(ctx, basis, false); err != nil {
			t.Fatalf("verifyReceiptPair(predecessor) error = %v", err)
		}
	})
}

func TestExecuteReceiptRecoveryRejectsStaleStateWithoutBroadMutation(t *testing.T) {
	t.Parallel()

	fixture := newRefreshEffectsFixture(t)
	ctx := context.Background()
	presentBasis := fixture.receiptBasis(ReceiptLockPresent, fixture.predecessorSHA)

	t.Run("unexpected source revision", func(t *testing.T) {
		prepareEffectsReceipt(t, fixture, presentBasis, ReceiptStatePrepared, false)
		fixture.checkoutSource(t, fixture.databaseOnlySHA)
		expectResumeRecoveryRequired(t, ctx, fixture.receiptPath)
		current, err := CheckedOutSourceRevision(
			ctx,
			RepositoryLayout{SourceRepository: fixture.sourceRepository},
		)
		if err != nil {
			t.Fatal(err)
		}
		if current != fixture.databaseOnlySHA {
			t.Fatalf("stale source changed to %s", current)
		}
		assertFileDigest(t, fixture.targetDatabase, fixture.predecessorDatabaseDigest)
		assertFileDigest(t, fixture.targetLock, fixture.predecessorLockDigest)
	})

	t.Run("unrelated source dirt", func(t *testing.T) {
		prepareEffectsReceipt(t, fixture, presentBasis, ReceiptStatePrepared, false)
		dirtPath := filepath.Join(fixture.sourceRepository, "operator-dirt.txt")
		const dirt = "do not touch\n"
		if err := os.WriteFile(dirtPath, []byte(dirt), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Remove(dirtPath) })
		expectResumePreflightRecoveryRequired(t, ctx, fixture.receiptPath)
		if got := readEffectsFile(t, dirtPath); got != dirt {
			t.Fatalf("source dirt changed to %q", got)
		}
		assertFileDigest(t, fixture.targetDatabase, fixture.predecessorDatabaseDigest)
		assertFileDigest(t, fixture.targetLock, fixture.predecessorLockDigest)
	})

	t.Run("unrelated source dirt after source advancement", func(t *testing.T) {
		prepareEffectsReceipt(
			t,
			fixture,
			presentBasis,
			ReceiptStateSourceApplied,
			false,
		)
		dirtPath := filepath.Join(fixture.sourceRepository, "operator-late-dirt.txt")
		const dirt = "do not touch after source apply\n"
		if err := os.WriteFile(dirtPath, []byte(dirt), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Remove(dirtPath) })
		expectResumePreflightRecoveryRequired(t, ctx, fixture.receiptPath)
		if got := readEffectsFile(t, dirtPath); got != dirt {
			t.Fatalf("late source dirt changed to %q", got)
		}
		assertFileDigest(t, fixture.targetDatabase, fixture.predecessorDatabaseDigest)
		assertFileDigest(t, fixture.targetLock, fixture.predecessorLockDigest)
	})

	t.Run("unrecognized database target", func(t *testing.T) {
		prepareEffectsReceipt(
			t,
			fixture,
			presentBasis,
			ReceiptStateSourceApplied,
			false,
		)
		const stale = "unrecognized database bytes\n"
		if err := os.WriteFile(fixture.targetDatabase, []byte(stale), 0o644); err != nil {
			t.Fatal(err)
		}
		expectResumeRecoveryRequired(t, ctx, fixture.receiptPath)
		if got := readEffectsFile(t, fixture.targetDatabase); got != stale {
			t.Fatal("stale database target was overwritten")
		}
		assertFileDigest(t, fixture.targetLock, fixture.predecessorLockDigest)
	})

	t.Run("missing database target", func(t *testing.T) {
		prepareEffectsReceipt(
			t,
			fixture,
			presentBasis,
			ReceiptStateSourceApplied,
			false,
		)
		if err := os.Remove(fixture.targetDatabase); err != nil {
			t.Fatal(err)
		}
		expectResumeRecoveryRequired(t, ctx, fixture.receiptPath)
		if _, err := os.Lstat(fixture.targetDatabase); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("missing database was silently materialized: %v", err)
		}
		assertFileDigest(t, fixture.targetLock, fixture.predecessorLockDigest)
	})

	t.Run("unrecognized lock target", func(t *testing.T) {
		prepareEffectsReceipt(t, fixture, presentBasis, ReceiptStateDBApplied, false)
		const stale = "unrecognized lock bytes\n"
		if err := os.WriteFile(fixture.targetLock, []byte(stale), 0o644); err != nil {
			t.Fatal(err)
		}
		expectResumeRecoveryRequired(t, ctx, fixture.receiptPath)
		if got := readEffectsFile(t, fixture.targetLock); got != stale {
			t.Fatal("stale lock target was overwritten")
		}
		assertFileDigest(t, fixture.targetDatabase, fixture.candidateDatabaseDigest)
	})

	t.Run("missing present predecessor lock target", func(t *testing.T) {
		prepareEffectsReceipt(t, fixture, presentBasis, ReceiptStateDBApplied, false)
		if err := os.Remove(fixture.targetLock); err != nil {
			t.Fatal(err)
		}
		expectResumeRecoveryRequired(t, ctx, fixture.receiptPath)
		if _, err := os.Lstat(fixture.targetLock); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("missing predecessor lock was silently replaced: %v", err)
		}
		assertFileDigest(t, fixture.targetDatabase, fixture.candidateDatabaseDigest)
	})

	t.Run("corrupt candidate database artifact", func(t *testing.T) {
		prepareEffectsReceipt(
			t,
			fixture,
			presentBasis,
			ReceiptStateSourceApplied,
			false,
		)
		original := append([]byte(nil), []byte(readEffectsFile(t, fixture.candidateDatabase))...)
		if err := os.WriteFile(
			fixture.candidateDatabase,
			[]byte("corrupt candidate database\n"),
			0o644,
		); err != nil {
			t.Fatal(err)
		}
		expectResumeRecoveryRequired(t, ctx, fixture.receiptPath)
		assertFileDigest(t, fixture.targetDatabase, fixture.predecessorDatabaseDigest)
		if err := os.WriteFile(fixture.candidateDatabase, original, 0o644); err != nil {
			t.Fatal(err)
		}
		assertFileDigest(t, fixture.candidateDatabase, fixture.candidateDatabaseDigest)
	})

	t.Run("corrupt candidate lock artifact", func(t *testing.T) {
		prepareEffectsReceipt(t, fixture, presentBasis, ReceiptStateDBApplied, false)
		original := readEffectsFile(t, fixture.candidateLock)
		if err := os.WriteFile(
			fixture.candidateLock,
			[]byte("corrupt candidate lock\n"),
			0o644,
		); err != nil {
			t.Fatal(err)
		}
		expectResumeRecoveryRequired(t, ctx, fixture.receiptPath)
		assertFileDigest(t, fixture.targetLock, fixture.predecessorLockDigest)
		if err := os.WriteFile(fixture.candidateLock, []byte(original), 0o644); err != nil {
			t.Fatal(err)
		}
		assertFileDigest(t, fixture.candidateLock, fixture.candidateLockDigest)
	})

	t.Run("missing-lock predecessor source and database mismatch", func(t *testing.T) {
		incoherentBackup := filepath.Join(
			fixture.stateDirectory,
			"prepared",
			"incoherent-predecessor.db",
		)
		copyEffectsFile(t, fixture.candidateDatabase, incoherentBackup)
		basis := fixture.receiptBasis(ReceiptLockMissing, fixture.predecessorSHA)
		basis.Predecessor.DatabaseDigest = fixture.candidateDatabaseDigest
		basis.Artifacts.PredecessorDatabaseBackupPath = incoherentBackup
		fixture.checkoutSource(t, fixture.predecessorSHA)
		copyEffectsFile(t, fixture.candidateDatabase, fixture.targetDatabase)
		if err := os.Remove(fixture.targetLock); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		createEffectsReceipt(t, fixture.receiptPath, basis, ReceiptStatePrepared)
		expectRestoreRecoveryRequired(t, ctx, fixture.receiptPath)
		current, err := CheckedOutSourceRevision(
			ctx,
			RepositoryLayout{SourceRepository: fixture.sourceRepository},
		)
		if err != nil {
			t.Fatal(err)
		}
		if current != fixture.predecessorSHA {
			t.Fatalf("incoherent restore moved source to %s", current)
		}
		assertFileDigest(t, fixture.targetDatabase, fixture.candidateDatabaseDigest)
	})

	if got := readEffectsFile(t, fixture.unrelatedSentinel); got != fixture.unrelatedSentinelBytes {
		t.Fatalf("stale-state tests changed unrelated root file to %q", got)
	}
}

func TestRecoveryOperationLockIsConcurrentAndCrashReplaySafe(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	receiptPath := filepath.Join(directory, "receipt.json")
	release, err := acquireOperationLock(receiptPath)
	if err != nil {
		t.Fatalf("acquireOperationLock(first) error = %v", err)
	}
	if _, err := acquireOperationLock(receiptPath); !errors.Is(err, ErrReceiptBusy) {
		release()
		t.Fatalf("concurrent acquire error = %v, want ErrReceiptBusy", err)
	}
	release()

	carrierPath := receiptPath + ".operation.lock"
	info, err := os.Lstat(carrierPath)
	if err != nil {
		t.Fatalf("operation carrier should survive release for crash-safe replay: %v", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("operation carrier mode = %v, want private regular 0600", info.Mode())
	}
	replayedRelease, err := acquireOperationLock(receiptPath)
	if err != nil {
		t.Fatalf("acquireOperationLock(existing unlocked carrier) error = %v", err)
	}
	replayedRelease()
}

func prepareEffectsReceipt(
	t *testing.T,
	fixture *refreshEffectsFixture,
	basis ReceiptBasis,
	state ReceiptState,
	nextEffectComplete bool,
) {
	t.Helper()
	fixture.installPredecessorPair(t, basis.Artifacts.PredecessorLock.Presence)
	if basis.InitialSourceSHA == fixture.candidateSHA {
		fixture.checkoutSource(t, fixture.candidateSHA)
	}
	if receiptStateAtLeast(state, ReceiptStateSourceApplied) {
		fixture.checkoutSource(t, fixture.candidateSHA)
	}
	if receiptStateAtLeast(state, ReceiptStateDBApplied) {
		copyEffectsFile(t, fixture.candidateDatabase, fixture.targetDatabase)
	}
	if receiptStateAtLeast(state, ReceiptStateLockApplied) {
		copyEffectsFile(t, fixture.candidateLock, fixture.targetLock)
	}
	switch {
	case nextEffectComplete && state == ReceiptStatePrepared:
		fixture.checkoutSource(t, fixture.candidateSHA)
	case nextEffectComplete && state == ReceiptStateSourceApplied:
		copyEffectsFile(t, fixture.candidateDatabase, fixture.targetDatabase)
	case nextEffectComplete && state == ReceiptStateDBApplied:
		copyEffectsFile(t, fixture.candidateLock, fixture.targetLock)
	}
	createEffectsReceipt(t, fixture.receiptPath, basis, state)
}

func createEffectsReceipt(
	t *testing.T,
	path string,
	basis ReceiptBasis,
	state ReceiptState,
) {
	t.Helper()
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	receipt, err := NewApplyReceipt(basis)
	if err != nil {
		t.Fatalf("NewApplyReceipt() error = %v", err)
	}
	if err := CreateReceipt(path, receipt); err != nil {
		t.Fatalf("CreateReceipt() error = %v", err)
	}
	for _, next := range []ReceiptState{
		ReceiptStateSourceApplied,
		ReceiptStateDBApplied,
		ReceiptStateLockApplied,
		ReceiptStateVerified,
		ReceiptStateComplete,
	} {
		if !receiptStateAtLeast(state, next) {
			break
		}
		receipt, err = AdvanceReceipt(path, basis, next)
		if err != nil {
			t.Fatalf("AdvanceReceipt(%s) error = %v", next, err)
		}
	}
}

func receiptStateAtLeast(state ReceiptState, threshold ReceiptState) bool {
	rank := map[ReceiptState]int{
		ReceiptStatePrepared:      0,
		ReceiptStateSourceApplied: 1,
		ReceiptStateDBApplied:     2,
		ReceiptStateLockApplied:   3,
		ReceiptStateVerified:      4,
		ReceiptStateComplete:      5,
		ReceiptStateRestored:      6,
	}
	return rank[state] >= rank[threshold]
}

func assertCandidateEffectsPair(t *testing.T, fixture *refreshEffectsFixture) {
	t.Helper()
	current, err := exactGitRevisionAt(context.Background(), fixture.sourceRepository)
	if err != nil {
		t.Fatal(err)
	}
	if current != fixture.candidateSHA {
		t.Fatalf("source revision = %s, want candidate %s", current, fixture.candidateSHA)
	}
	assertFileDigest(t, fixture.targetDatabase, fixture.candidateDatabaseDigest)
	assertFileDigest(t, fixture.targetLock, fixture.candidateLockDigest)
	layout, err := ResolveRepositoryLayout(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	tracked, err := TrackedSourceRevision(context.Background(), layout)
	if err != nil {
		t.Fatal(err)
	}
	if tracked != fixture.predecessorSHA {
		t.Fatalf("apply changed tracked root gitlink to %s", tracked)
	}
}

func assertCandidateRepositoryIntegration(
	t *testing.T,
	fixture *refreshEffectsFixture,
) {
	t.Helper()
	layout, err := ResolveRepositoryLayout(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifyRepositoryIntegrationForTest(
		context.Background(),
		layout,
		"fpf-refresh-test",
		nil,
	); err != nil {
		t.Fatalf("VerifyRepositoryIntegration(candidate) error = %v", err)
	}
}

func assertPredecessorEffectsPair(
	t *testing.T,
	fixture *refreshEffectsFixture,
	basis ReceiptBasis,
) {
	t.Helper()
	if err := verifyReceiptPair(context.Background(), basis, false); err != nil {
		t.Fatalf("verifyReceiptPair(predecessor) error = %v", err)
	}
	current, err := exactGitRevisionAt(context.Background(), fixture.sourceRepository)
	if err != nil {
		t.Fatal(err)
	}
	if current != fixture.predecessorSHA {
		t.Fatalf("source revision = %s, want predecessor %s", current, fixture.predecessorSHA)
	}
	assertFileDigest(t, fixture.targetDatabase, fixture.predecessorDatabaseDigest)
	switch basis.Artifacts.PredecessorLock.Presence {
	case ReceiptLockPresent:
		assertFileDigest(t, fixture.targetLock, fixture.predecessorLockDigest)
	case ReceiptLockMissing:
		if _, err := os.Lstat(fixture.targetLock); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("first-ever predecessor lock exists after restore: %v", err)
		}
	}
}

func (fixture *refreshEffectsFixture) protectedFiles(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{
		fixture.candidateDatabase:         effectsDigest(t, fixture.candidateDatabase),
		fixture.candidateLock:             effectsDigest(t, fixture.candidateLock),
		fixture.predecessorDatabaseBackup: effectsDigest(t, fixture.predecessorDatabaseBackup),
		fixture.predecessorLockBackup:     effectsDigest(t, fixture.predecessorLockBackup),
		fixture.unrelatedSentinel:         effectsDigest(t, fixture.unrelatedSentinel),
	}
}

func assertProtectedEffectsFiles(t *testing.T, protected map[string]string) {
	t.Helper()
	for path, digest := range protected {
		assertFileDigest(t, path, digest)
	}
}

func assertNoApplyStages(t *testing.T, fixture *refreshEffectsFixture) {
	t.Helper()
	for _, target := range []string{fixture.targetDatabase, fixture.targetLock} {
		stages, err := filepath.Glob(
			filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".refresh-*"),
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(stages) != 0 {
			t.Fatalf("atomic install stages remain for %s: %v", target, stages)
		}
	}
}

func expectResumeRecoveryRequired(
	t *testing.T,
	ctx context.Context,
	receiptPath string,
) {
	t.Helper()
	_, err := ExecuteReceiptResume(ctx, receiptPath)
	expectRecoveryRequired(t, err)
}

func expectResumePreflightRecoveryRequired(
	t *testing.T,
	ctx context.Context,
	receiptPath string,
) {
	t.Helper()
	_, err := ExecuteReceiptResume(ctx, receiptPath)
	if err == nil || !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("recovery error = %v, want ErrRecoveryRequired", err)
	}
	if !strings.Contains(err.Error(), "preflight-source-cleanliness") {
		t.Fatalf(
			"recovery error = %v, want source-cleanliness preflight detail",
			err,
		)
	}
}

func expectRestoreRecoveryRequired(
	t *testing.T,
	ctx context.Context,
	receiptPath string,
) {
	t.Helper()
	_, err := ExecuteReceiptRestore(ctx, receiptPath)
	expectRecoveryRequired(t, err)
}

func expectRecoveryRequired(t *testing.T, err error) {
	t.Helper()
	if err == nil || !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("recovery error = %v, want ErrRecoveryRequired", err)
	}
	if !strings.Contains(err.Error(), ErrReceiptStale.Error()) {
		t.Fatalf("recovery error = %v, want stale-basis detail", err)
	}
}
