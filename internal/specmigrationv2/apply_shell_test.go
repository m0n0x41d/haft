package specmigrationv2

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
)

const (
	effectCrashRootEnvironment  = "HAFT_TEST_MIGRATION_CRASH_ROOT"
	effectCrashPointEnvironment = "HAFT_TEST_MIGRATION_CRASH_POINT"
	effectCrashModeEnvironment  = "HAFT_TEST_MIGRATION_CRASH_MODE"
)

func TestMigrationEffectSagaArchivesExactBytesWritesReceiptAndReplays(t *testing.T) {
	plan := newEffectPlanFixture(t)

	result := startMigrationSaga(plan, nil)
	appliedResult, ok := result.(Applied)
	if !ok {
		t.Fatalf("result = %T, want Applied", result)
	}
	assertCompletedEffectState(t, plan)

	journal, found, err := loadJournal(plan.paths.journal)
	if err != nil || !found {
		t.Fatalf("loadJournal: found=%v err=%v", found, err)
	}
	replayedResult, ok := resultForExistingJournal(plan, journal).(Replayed)
	if !ok {
		t.Fatalf("replay result is not Replayed")
	}
	if replayedResult.Receipt().PacketDigest().String() != appliedResult.Receipt().PacketDigest().String() {
		t.Fatal("replay receipt does not bind the original packet")
	}
	expectedRelativeRef, err := confinedRelativePath(
		plan.request.projectRoot.String(),
		plan.paths.receipt,
	)
	if err != nil {
		t.Fatalf("confinedRelativePath: %v", err)
	}
	expectedRef := filepath.ToSlash(expectedRelativeRef)
	expectedDigest := DigestBytes(plan.receiptBytes)
	assertMigrationEffectReceiptCarrier(
		t,
		appliedResult.ReceiptCarrier(),
		expectedRef,
		expectedDigest,
	)
	assertMigrationEffectReceiptCarrier(
		t,
		replayedResult.ReceiptCarrier(),
		expectedRef,
		expectedDigest,
	)
}

func assertMigrationEffectReceiptCarrier(
	t *testing.T,
	carrier MigrationEffectReceiptCarrier,
	wantRef string,
	wantDigest SHA256,
) {
	t.Helper()
	if !carrier.valid() {
		t.Fatal("migration effect receipt carrier is invalid")
	}
	if carrier.Ref().String() != wantRef {
		t.Fatalf("receipt carrier ref = %q, want %q", carrier.Ref().String(), wantRef)
	}
	if !carrier.Digest().Equal(wantDigest) {
		t.Fatalf(
			"receipt carrier digest = %q, want %q",
			carrier.Digest().String(),
			wantDigest.String(),
		)
	}
}

func TestFreshApplyReviewGateRechecksSemanticZeroPassAndFPF(t *testing.T) {
	plan := newEffectPlanFixture(t)
	ctx := context.Background()
	if err := verifyCurrentReviewBasis(ctx, plan); err != nil {
		t.Fatalf("exact current review basis: %v", err)
	}
	semantic := plan.request.review.semanticZeroPass
	semanticPath := filepath.Join(
		plan.request.projectRoot.String(),
		filepath.FromSlash(semantic.Carrier().String()),
	)
	if err := os.WriteFile(semanticPath, []byte("semantic drift\n"), 0o600); err != nil {
		t.Fatalf("drift semantic zero-pass: %v", err)
	}
	if err := verifyCurrentReviewBasis(ctx, plan); err == nil ||
		!strings.Contains(err.Error(), "semantic_zero_pass") {
		t.Fatalf("semantic zero-pass drift passed fresh-apply gate: %v", err)
	}
	if err := os.WriteFile(
		semanticPath,
		[]byte("reviewed semantic zero-pass\n"),
		0o600,
	); err != nil {
		t.Fatalf("restore semantic zero-pass: %v", err)
	}
	fpfSpec := filepath.Join(
		plan.request.projectRoot.String(),
		"data",
		"FPF",
		"FPF-Spec.md",
	)
	if err := os.WriteFile(fpfSpec, []byte("# dirty FPF fixture\n"), 0o600); err != nil {
		t.Fatalf("drift FPF fixture: %v", err)
	}
	if err := verifyCurrentReviewBasis(ctx, plan); err == nil ||
		!strings.Contains(err.Error(), "FPF worktree is not clean") {
		t.Fatalf("dirty FPF passed fresh-apply gate: %v", err)
	}
}

func TestMigrationEffectSagaFailureInjectionIsRecoverableAtEveryDurabilityBoundary(t *testing.T) {
	for _, point := range allMigrationEffectPoints() {
		t.Run(string(point), func(t *testing.T) {
			plan := newEffectPlanFixture(t)
			hook := func(observed effectPoint) error {
				if observed == point {
					return errors.New("stop here")
				}
				return nil
			}

			result := startMigrationSaga(plan, hook)
			if _, ok := result.(RecoveryRequired); !ok {
				t.Fatalf("result = %T, want RecoveryRequired", result)
			}

			recovered := recoverEffectPlanForTest(t, plan)
			switch recovered.(type) {
			case Applied, Replayed:
				assertCompletedEffectState(t, plan)
			default:
				reason := ""
				if blocked, ok := recovered.(RecoveryRequired); ok {
					reason = blocked.Reason()
				}
				t.Fatalf("recovery result = %T: %s", recovered, reason)
			}
		})
	}
}

func TestMigrationEffectSubprocessCrashRecoversAtEveryDurabilityBoundary(t *testing.T) {
	root := os.Getenv(effectCrashRootEnvironment)
	if root != "" {
		runMigrationEffectCrashChild(t, root)
		return
	}
	for _, point := range allMigrationEffectPoints() {
		t.Run(string(point), func(t *testing.T) {
			plan := newEffectPlanFixture(t)
			runMigrationCrashProcess(t, plan, point, "apply")
			assertRecoveredAfterCrash(t, plan)
		})
	}
	for _, point := range []effectPoint{
		effectRecoveryPreparedRename,
		effectRecoveryPreparedDirSync,
	} {
		t.Run(string(point), func(t *testing.T) {
			plan := newEffectPlanFixture(t)
			primePreparedJournalTemp(t, plan)
			runMigrationCrashProcess(t, plan, point, "recover_prepared")
			assertRecoveredAfterCrash(t, plan)
		})
	}
	probe := newEffectPlanFixture(t)
	recoveryPoints, err := recoveryEffectPointsForProgress(probe, progressCompleted)
	if err != nil {
		t.Fatalf("recoveryEffectPointsForProgress: %v", err)
	}
	for _, point := range recoveryPoints {
		t.Run(string(point), func(t *testing.T) {
			plan := newEffectPlanFixture(t)
			if _, ok := startMigrationSaga(plan, nil).(Applied); !ok {
				t.Fatal("failed to prime completed migration for recovery fsync crash")
			}
			runMigrationCrashProcess(t, plan, point, "resynchronize")
			assertRecoveredAfterCrash(t, plan)
		})
	}
}

func runMigrationEffectCrashChild(t *testing.T, root string) {
	t.Helper()
	plan := newEffectPlanFixtureAtRoot(t, root, false)
	point := effectPoint(os.Getenv(effectCrashPointEnvironment))
	mode := os.Getenv(effectCrashModeEnvironment)
	hook := func(observed effectPoint) error {
		if observed == point {
			os.Exit(86)
		}
		return nil
	}
	switch mode {
	case "apply":
		_ = startMigrationSaga(plan, hook)
	case "recover_prepared":
		_ = promotePreparedJournalTemp(plan.paths.journal, hook)
	case "resynchronize":
		_ = resynchronizeObservedPrefix(plan, progressCompleted, hook)
	default:
		t.Fatalf("unknown crash helper mode %q", mode)
	}
	os.Exit(87)
}

func runMigrationCrashProcess(
	t *testing.T,
	plan migrationEffectPlan,
	point effectPoint,
	mode string,
) {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestMigrationEffectSubprocessCrashRecoversAtEveryDurabilityBoundary$")
	command.Env = append(
		os.Environ(),
		effectCrashRootEnvironment+"="+plan.request.projectRoot.String(),
		effectCrashPointEnvironment+"="+string(point),
		effectCrashModeEnvironment+"="+mode,
	)
	output, err := command.CombinedOutput()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 86 {
		t.Fatalf("crash helper at %s: err=%v output=%s", point, err, output)
	}
}

func primePreparedJournalTemp(t *testing.T, plan migrationEffectPlan) {
	t.Helper()
	hook := func(observed effectPoint) error {
		if observed == effectJournalPreparedFileSync {
			return errors.New("leave prepared journal temp")
		}
		return nil
	}
	if _, ok := startMigrationSaga(plan, hook).(RecoveryRequired); !ok {
		t.Fatal("prepared-journal priming did not stop in recovery-required state")
	}
}

func assertRecoveredAfterCrash(t *testing.T, plan migrationEffectPlan) {
	t.Helper()
	result := recoverEffectPlanForTest(t, plan)
	switch result.(type) {
	case Applied, Replayed:
		assertCompletedEffectState(t, plan)
	default:
		t.Fatalf("subprocess crash recovery result = %T", result)
	}
}

func recoveryEffectPointsForProgress(
	plan migrationEffectPlan,
	progress sagaProgress,
) ([]effectPoint, error) {
	directories := durabilityDirectoriesForProgress(plan, progress)
	points := make([]effectPoint, 0, len(directories))
	for _, directory := range directories {
		point, err := recoveryDirectoryEffectPoint(plan, directory)
		if err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, nil
}

func TestMigrationEffectRejectsCollisionAndDriftBeforeDurableJournal(t *testing.T) {
	for _, mutate := range []func(testing.TB, migrationEffectPlan){
		func(t testing.TB, plan migrationEffectPlan) {
			writeFixtureFile(t, plan.paths.target, []byte("collision"))
		},
		func(t testing.TB, plan migrationEffectPlan) {
			writeFixtureFile(t, plan.paths.archive, []byte("collision"))
		},
		func(t testing.TB, plan migrationEffectPlan) {
			writeFixtureFile(t, plan.paths.source, []byte("drift"))
		},
	} {
		plan := newEffectPlanFixture(t)
		mutate(t, plan)
		if err := verifyInitialFilesystemState(plan); err == nil {
			t.Fatal("initial state verification accepted collision or drift")
		}
		if exists, err := safePathExists(plan.paths.journal); err != nil || exists {
			t.Fatalf("journal exists=%v err=%v on rejected initial state", exists, err)
		}
	}
}

func TestCompletedJournalWithDifferentPacketCannotReplay(t *testing.T) {
	plan := newEffectPlanFixture(t)
	if _, ok := startMigrationSaga(plan, nil).(Applied); !ok {
		t.Fatal("initial saga did not apply")
	}
	journal, found, err := loadJournal(plan.paths.journal)
	if err != nil || !found {
		t.Fatalf("loadJournal: found=%v err=%v", found, err)
	}
	different := plan
	differentDigest, err := NewPacketDigest(DigestBytes([]byte("different packet")).String())
	if err != nil {
		t.Fatalf("NewPacketDigest: %v", err)
	}
	different.request.analysis.packetDigest = differentDigest
	different.journal.packetDigest = differentDigest

	result := resultForExistingJournal(different, journal)
	if _, ok := result.(RecoveryRequired); !ok {
		t.Fatalf("result = %T, want RecoveryRequired", result)
	}
}

func TestMigrationEffectNoReplacePreservesConcurrentTargetAndArchiveCollisions(t *testing.T) {
	t.Run("target", func(t *testing.T) {
		plan := newEffectPlanFixture(t)
		source := readFixtureFile(t, plan.paths.source)
		collision := []byte("concurrent target collision\n")
		hook := func(observed effectPoint) error {
			if observed == effectTargetStageDirectorySync {
				writeFixtureFile(t, plan.paths.target, collision)
			}
			return nil
		}
		result := startMigrationSaga(plan, hook)
		if _, ok := result.(RecoveryRequired); !ok {
			t.Fatalf("result = %T, want RecoveryRequired", result)
		}
		assertFileBytes(t, plan.paths.target, collision)
		assertFileBytes(t, plan.paths.source, source)
	})

	t.Run("archive", func(t *testing.T) {
		plan := newEffectPlanFixture(t)
		source := readFixtureFile(t, plan.paths.source)
		collision := []byte("concurrent archive collision\n")
		hook := func(observed effectPoint) error {
			if observed == effectArchiveParentParentSync {
				writeFixtureFile(t, plan.paths.archive, collision)
			}
			return nil
		}
		result := startMigrationSaga(plan, hook)
		if _, ok := result.(RecoveryRequired); !ok {
			t.Fatalf("result = %T, want RecoveryRequired", result)
		}
		assertFileBytes(t, plan.paths.archive, collision)
		assertFileBytes(t, plan.paths.source, source)
	})
}

func TestMigrationEffectRejectsSourceDriftImmediatelyBeforeArchive(t *testing.T) {
	plan := newEffectPlanFixture(t)
	drift := []byte("source drift after target install\n")
	hook := func(observed effectPoint) error {
		if observed == effectArchiveParentParentSync {
			writeFixtureFile(t, plan.paths.source, drift)
		}
		return nil
	}
	result := startMigrationSaga(plan, hook)
	if _, ok := result.(RecoveryRequired); !ok {
		t.Fatalf("result = %T, want RecoveryRequired", result)
	}
	assertFileBytes(t, plan.paths.source, drift)
	if exists, err := safePathExists(plan.paths.archive); err != nil || exists {
		t.Fatalf("archive exists=%v err=%v after pre-archive source drift", exists, err)
	}
}

func TestMigrationEffectRejectsTargetDriftBeforeAndAfterInstall(t *testing.T) {
	t.Run("staged target", func(t *testing.T) {
		plan := newEffectPlanFixture(t)
		hook := func(observed effectPoint) error {
			if observed == effectTargetStageDirectorySync {
				writeFixtureFile(t, plan.paths.targetStage, []byte("staged target drift\n"))
			}
			return nil
		}
		if _, ok := startMigrationSaga(plan, hook).(RecoveryRequired); !ok {
			t.Fatal("staged target drift did not require recovery")
		}
		if exists, err := safePathExists(plan.paths.target); err != nil || exists {
			t.Fatalf("target exists=%v err=%v after staged drift", exists, err)
		}
	})

	t.Run("installed target", func(t *testing.T) {
		plan := newEffectPlanFixture(t)
		drift := []byte("installed target drift\n")
		hook := func(observed effectPoint) error {
			if observed == effectTargetInstallDirectorySync {
				writeFixtureFile(t, plan.paths.target, drift)
			}
			return nil
		}
		if _, ok := startMigrationSaga(plan, hook).(RecoveryRequired); !ok {
			t.Fatal("installed target drift did not require recovery")
		}
		assertFileBytes(t, plan.paths.target, drift)
	})
}

func TestMigrationEffectRechecksSemanticReviewCarriersBeforeReceipt(t *testing.T) {
	plan := newEffectPlanFixture(t)
	binding := plan.request.review.targetCarrierDigests.Values()[0]
	root := plan.request.projectRoot.String()
	carrier := binding.carrier.String()
	carrierPath := filepath.FromSlash(carrier)
	path := filepath.Join(root, carrierPath)
	hook := func(observed effectPoint) error {
		if observed == effectJournalLineageDirectorySync {
			writeFixtureFile(t, path, []byte("review carrier drift\n"))
		}
		return nil
	}
	result := startMigrationSaga(plan, hook)
	if _, ok := result.(RecoveryRequired); !ok {
		t.Fatalf("result = %T, want RecoveryRequired", result)
	}
	if exists, err := safePathExists(plan.paths.receipt); err != nil || exists {
		t.Fatalf("receipt exists=%v err=%v after review-carrier drift", exists, err)
	}
}

func TestMigrationEffectRejectsSymlinkEscapeAndNeverTruncatesItsReferent(t *testing.T) {
	plan := newEffectPlanFixture(t)
	outside := filepath.Join(t.TempDir(), "outside.md")
	original := []byte("outside bytes must survive\n")
	writeFixtureFile(t, outside, original)
	temporary := plan.paths.journal + ".tmp"
	if err := os.Symlink(outside, temporary); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	if err := verifyInitialFilesystemState(plan); err == nil {
		t.Fatal("initial verification accepted a symlinked journal temporary")
	}
	if err := writeExclusiveOrVerify(temporary, []byte("replacement"), 0o600); err == nil {
		t.Fatal("exclusive temporary writer followed a symlink")
	}
	assertFileBytes(t, outside, original)
	if exists, err := safePathExists(plan.paths.journal); err != nil || exists {
		t.Fatalf("journal exists=%v err=%v after symlink rejection", exists, err)
	}
}

func TestMigrationEffectRejectsSymlinkedCarrierAndMissingTargetParentBeforeJournal(t *testing.T) {
	t.Run("source symlink", func(t *testing.T) {
		plan := newEffectPlanFixture(t)
		outside := filepath.Join(t.TempDir(), "source.md")
		source := readFixtureFile(t, plan.paths.source)
		writeFixtureFile(t, outside, source)
		if err := os.Remove(plan.paths.source); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if err := os.Symlink(outside, plan.paths.source); err != nil {
			t.Fatalf("Symlink: %v", err)
		}
		if err := verifyInitialFilesystemState(plan); err == nil {
			t.Fatal("initial verification accepted a symlinked source")
		}
		assertNoDurableJournal(t, plan)
	})

	t.Run("missing target parent", func(t *testing.T) {
		plan := newEffectPlanFixture(t)
		plan.paths.target = filepath.Join(
			plan.request.projectRoot.String(),
			".haft",
			"missing-target-parent",
			"software-system.md",
		)
		if err := verifyInitialFilesystemState(plan); err == nil {
			t.Fatal("initial verification accepted a missing target parent")
		}
		assertNoDurableJournal(t, plan)
	})

	t.Run("symlinked target parent", func(t *testing.T) {
		plan := newEffectPlanFixture(t)
		outside := t.TempDir()
		link := filepath.Join(plan.request.projectRoot.String(), ".haft", "escaped-target")
		if err := os.Symlink(outside, link); err != nil {
			t.Fatalf("Symlink: %v", err)
		}
		plan.paths.target = filepath.Join(link, "software-system.md")
		if err := verifyInitialFilesystemState(plan); err == nil {
			t.Fatal("initial verification accepted a symlinked target parent")
		}
		assertNoDurableJournal(t, plan)
	})
}

func TestCompletedReplayRejectsMatchingByteSymlink(t *testing.T) {
	plan := newEffectPlanFixture(t)
	if _, ok := startMigrationSaga(plan, nil).(Applied); !ok {
		t.Fatal("initial saga did not apply")
	}
	journal, found, err := loadJournal(plan.paths.journal)
	if err != nil || !found {
		t.Fatalf("loadJournal: found=%v err=%v", found, err)
	}
	outside := filepath.Join(t.TempDir(), "same-target.md")
	writeFixtureFile(t, outside, plan.targetBytes)
	if err := os.Remove(plan.paths.target); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := os.Symlink(outside, plan.paths.target); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	result := resultForExistingJournal(plan, journal)
	if _, ok := result.(RecoveryRequired); !ok {
		t.Fatalf("result = %T, want RecoveryRequired", result)
	}
}

func TestPersistedGitWitnessMustMatchExactRequestProvenance(t *testing.T) {
	plan := newEffectPlanFixture(t)
	otherCommit, err := NewGitCommitOID("sha1:" + strings.Repeat("2", 40))
	if err != nil {
		t.Fatalf("NewGitCommitOID: %v", err)
	}
	otherOrigin, err := NewRepositoryEdition(
		plan.journal.gitWitness.origin.ProjectRoot(),
		otherCommit,
		plan.journal.sourceCarrier,
		plan.journal.sourceDigest,
	)
	if err != nil {
		t.Fatalf("NewRepositoryEdition: %v", err)
	}
	other := plan.journal.gitWitness
	other.headCommit = otherCommit
	other.origin = otherOrigin
	canonical, err := encodeGitWitness(other)
	if err != nil {
		t.Fatalf("encodeGitWitness: %v", err)
	}
	other.canonical = canonical
	other.digest = DigestBytes(canonical)
	if err := validateGitWitnessAgainstProvenance(
		other,
		plan.request.projectRoot,
		plan.request.analysis.sourceProvenance,
	); err == nil {
		t.Fatal("an independently valid but wrong Git witness matched the request provenance")
	}
}

func TestRejectedApplyRequestCreatesNoLockTemporaryOrJournalCarrier(t *testing.T) {
	rootPath := t.TempDir()
	haftPath := filepath.Join(rootPath, ".haft")
	if err := os.Mkdir(haftPath, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	root, err := NewApplyProjectRoot(rootPath)
	if err != nil {
		t.Fatalf("NewApplyProjectRoot: %v", err)
	}
	before := directorySnapshot(t, rootPath)
	result := ApplyMigration(
		context.Background(),
		profileadmissionsqlite.Service{},
		ApplyRequest{projectRoot: root},
	)
	if _, ok := result.(ApplyRejected); !ok {
		t.Fatalf("result = %T, want ApplyRejected", result)
	}
	after := directorySnapshot(t, rootPath)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("rejected apply changed project files: before=%v after=%v", before, after)
	}
}

func recoverEffectPlanForTest(t *testing.T, plan migrationEffectPlan) MigrationApplyResult {
	t.Helper()
	if err := promotePreparedJournalTemp(plan.paths.journal, nil); err != nil {
		t.Fatalf("promotePreparedJournalTemp: %v", err)
	}
	journal, found, err := loadJournal(plan.paths.journal)
	if err != nil || !found {
		t.Fatalf("loadJournal: found=%v err=%v", found, err)
	}
	progress, err := inspectSagaProgress(plan)
	if err != nil {
		t.Fatalf("inspectSagaProgress: %v", err)
	}
	if progress == progressCompleted && journal.phase == JournalCompleted {
		return resultForExistingJournal(plan, journal)
	}
	if progress == progressCompleted {
		progress = progressReceiptWritten
	}
	if err := resynchronizeObservedPrefix(plan, progress, nil); err != nil {
		t.Fatalf("resynchronizeObservedPrefix: %v", err)
	}
	aligned, err := alignJournalToProgress(plan, journal, progress, nil)
	if err != nil {
		t.Fatalf("alignJournalToProgress: %v", err)
	}
	return continueMigrationSaga(plan, aligned, progress, nil)
}

func assertCompletedEffectState(t *testing.T, plan migrationEffectPlan) {
	t.Helper()
	if _, err := os.Stat(plan.paths.source); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source still exists or stat failed unexpectedly: %v", err)
	}
	assertFileBytes(t, plan.paths.target, plan.targetBytes)
	assertFileDigest(t, plan.paths.archive, plan.journal.sourceDigest.String())
	assertFileBytes(t, plan.paths.lineage, plan.lineageBytes)
	assertFileBytes(t, plan.paths.receipt, plan.receiptBytes)
	journal, found, err := loadJournal(plan.paths.journal)
	if err != nil || !found {
		t.Fatalf("loadJournal: found=%v err=%v", found, err)
	}
	if journal.phase != JournalCompleted {
		t.Fatalf("journal phase = %q", journal.phase)
	}
}

func newEffectPlanFixture(t *testing.T) migrationEffectPlan {
	t.Helper()
	rootPath := physicalTestRoot(t, t.TempDir())
	return newEffectPlanFixtureAtRoot(t, rootPath, true)
}

func newEffectPlanFixtureAtRoot(
	t *testing.T,
	rootPath string,
	initialize bool,
) migrationEffectPlan {
	t.Helper()
	if initialize {
		if err := os.MkdirAll(filepath.Join(rootPath, ".haft", "specs"), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	root, err := NewApplyProjectRoot(rootPath)
	if err != nil {
		t.Fatalf("NewApplyProjectRoot: %v", err)
	}
	migrationID, err := NewMigrationPacketID("migration-effect-test")
	if err != nil {
		t.Fatalf("NewMigrationPacketID: %v", err)
	}
	packetDigest, err := NewPacketDigest(DigestBytes([]byte("packet")).String())
	if err != nil {
		t.Fatalf("NewPacketDigest: %v", err)
	}
	sourceCarrier, err := NewSourceCarrierID(".haft/specs/enabling-system.md")
	if err != nil {
		t.Fatalf("NewSourceCarrierID: %v", err)
	}
	targetCarrier, err := NewTargetCarrierID(".haft/specs/software-system.md")
	if err != nil {
		t.Fatalf("NewTargetCarrierID: %v", err)
	}
	archiveCarrier, err := NewArchiveCarrierID(".haft/specs/archive/enabling-system.md")
	if err != nil {
		t.Fatalf("NewArchiveCarrierID: %v", err)
	}
	sourceBytes := []byte("exact designated enabling-system source\n")
	targetBytes := []byte("exact reviewed software-system target\n")
	sourceDigest := SourceDigestOf(sourceBytes)
	targetDigest := TargetDigestOf(targetBytes)
	provenance, witness := effectGitProvenanceFixture(t, root, sourceCarrier, sourceDigest)
	review := effectReviewFixture(
		t,
		root,
		packetDigest,
		sourceDigest,
		targetBytes,
		initialize,
	)
	lineagePolicy := minimalLineagePolicy(t, sourceBytes, archiveCarrier, sourceDigest)
	lineageDigest, err := LineagePolicyDigestOf(lineagePolicy)
	if err != nil {
		t.Fatalf("LineagePolicyDigestOf: %v", err)
	}
	analysis := structuralAnalysis{
		packetID:         migrationID,
		packetDigest:     packetDigest,
		sourceCarrier:    sourceCarrier,
		sourceDigest:     sourceDigest,
		targetCarrier:    targetCarrier,
		targetDigest:     targetDigest,
		archiveCarrier:   archiveCarrier,
		lineagePolicy:    lineagePolicy,
		lineageDigest:    lineageDigest,
		sourceProvenance: provenance,
	}
	now := time.Date(2026, 7, 14, 20, 0, 0, 0, time.UTC)
	request := ApplyRequest{
		projectRoot: root,
		analysis:    analysis,
		profileBinding: opaqueProfileBinding{
			ref:            "profile-admission:test",
			digest:         DigestBytes([]byte("profile admission")).String(),
			ledgerRevision: 1,
		},
		review:      review,
		requestedAt: now,
	}
	paths := effectPaths(request)
	if initialize {
		writeFixtureFile(t, paths.source, sourceBytes)
	}
	lineageBytes, err := encodeLineageRecord(migrationID, packetDigest, lineagePolicy, lineageDigest)
	if err != nil {
		t.Fatalf("encodeLineageRecord: %v", err)
	}
	reviewDigest, err := semanticReviewDigest(review)
	if err != nil {
		t.Fatalf("semanticReviewDigest: %v", err)
	}
	journal := migrationJournal{
		migrationID:           migrationID,
		packetDigest:          packetDigest,
		projectRoot:           root,
		sourceCarrier:         sourceCarrier,
		sourceDigest:          sourceDigest,
		targetCarrier:         targetCarrier,
		targetDigest:          targetDigest,
		archiveCarrier:        archiveCarrier,
		lineageDigest:         lineageDigest,
		lineageRecordDigest:   DigestBytes(lineageBytes),
		profileAdmissionRef:   request.profileBinding.ref,
		profileAdmissionHash:  request.profileBinding.digest,
		profileLedgerRevision: request.profileBinding.ledgerRevision,
		semanticReviewRef:     review.reviewRef,
		semanticAdmissionHash: review.admissionDigest,
		semanticReviewDigest:  reviewDigest,
		gitWitness:            witness,
		gitWitnessDigest:      witness.digest,
		phase:                 JournalPrepared,
		startedAt:             now,
		updatedAt:             now,
	}
	receipt := receiptFromJournal(journal)
	receiptBytes, err := encodeReceipt(receipt)
	if err != nil {
		t.Fatalf("encodeReceipt: %v", err)
	}
	journal.receiptDigest = DigestBytes(receiptBytes)
	if err := validateJournal(journal); err != nil {
		t.Fatalf("validateJournal: %v", err)
	}
	return migrationEffectPlan{
		request:      request,
		journal:      journal,
		paths:        paths,
		targetBytes:  targetBytes,
		lineageBytes: lineageBytes,
		receiptBytes: receiptBytes,
	}
}

func effectReviewFixture(
	t *testing.T,
	root ApplyProjectRoot,
	packetDigest PacketDigest,
	sourceDigest SourceDigest,
	targetBytes []byte,
	initialize bool,
) admittedMigrationReview {
	t.Helper()
	targetSystemBytes := []byte("reviewed target-system carrier\n")
	termMapBytes := []byte("reviewed term-map carrier\n")
	fixtures := []struct {
		role    ReviewCarrierRole
		carrier string
		content []byte
	}{
		{role: ReviewTargetSystemCarrier, carrier: ".context/review-target-system.md", content: targetSystemBytes},
		{role: ReviewSoftwareSystemCarrier, carrier: ".context/review-software-system.md", content: targetBytes},
		{role: ReviewTermMapCarrier, carrier: ".context/review-term-map.md", content: termMapBytes},
	}
	bindings := make([]ReviewCarrierDigest, 0, len(fixtures))
	for _, fixture := range fixtures {
		carrier, err := NewTargetCarrierID(fixture.carrier)
		if err != nil {
			t.Fatalf("NewTargetCarrierID: %v", err)
		}
		bindings = append(bindings, ReviewCarrierDigest{
			role:    fixture.role,
			carrier: carrier,
			digest:  DigestBytes(fixture.content),
		})
		if initialize {
			carrierPath := filepath.FromSlash(fixture.carrier)
			path := filepath.Join(root.String(), carrierPath)
			writeFixtureFile(t, path, fixture.content)
		}
	}
	reviewRef, err := newReviewRef("review:effect-test")
	if err != nil {
		t.Fatalf("newReviewRef: %v", err)
	}
	fpfRevisionValue := strings.Repeat("a", 40)
	if initialize {
		fpfRevisionValue = effectFPFRevisionFixture(t, root)
	} else if _, err := os.Lstat(
		filepath.Join(root.String(), "data", "FPF", ".git"),
	); err == nil {
		fpfRevisionValue = strings.TrimSpace(string(runApplyE2EGit(
			t,
			filepath.Join(root.String(), "data", "FPF"),
			"rev-parse",
			"HEAD",
		)))
	}
	fpfRevision, err := newFPFRevision(fpfRevisionValue)
	if err != nil {
		t.Fatalf("newFPFRevision: %v", err)
	}
	semanticCarrier, err := NewTargetCarrierID(".context/review-semantic-zero-pass.md")
	if err != nil {
		t.Fatalf("NewTargetCarrierID: %v", err)
	}
	semanticBytes := []byte("reviewed semantic zero-pass\n")
	if initialize {
		writeFixtureFile(
			t,
			filepath.Join(root.String(), filepath.FromSlash(semanticCarrier.String())),
			semanticBytes,
		)
	}
	speechActRef, err := NewSemanticReviewSpeechActRef("speech-act:effect-test")
	if err != nil {
		t.Fatalf("NewSemanticReviewSpeechActRef: %v", err)
	}
	sourceCarrier, err := NewSourceCarrierID(".haft/specs/enabling-system.md")
	if err != nil {
		t.Fatalf("NewSourceCarrierID: %v", err)
	}
	return admittedMigrationReview{
		reviewRef:           reviewRef,
		admissionDigest:     DigestBytes([]byte("effect review admission")),
		speechActRef:        speechActRef,
		speechActDigest:     DigestBytes([]byte("effect review SpeechAct")),
		projectRoot:         root,
		packetDigest:        packetDigest,
		packetCarrierDigest: PacketCarrierDigest{value: DigestBytes([]byte("effect packet carrier"))},
		partitionAudit: PacketPartitionAuditBinding{
			schema: PacketPartitionAuditSchemaVersionV1,
			status: PacketPartitionAuditVerified,
			digest: PacketPartitionAuditDigest{value: DigestBytes([]byte("effect partition audit"))},
		},
		sourceCarrier:        sourceCarrier,
		sourceDigest:         sourceDigest,
		targetCarrierDigests: ReviewCarrierDigestSet{values: bindings},
		fpfRevision:          fpfRevision,
		semanticZeroPass: SemanticZeroPassBinding{
			carrier: semanticCarrier,
			digest:  DigestBytes(semanticBytes),
		},
		lifecycleIntent: LifecycleIntent{values: []LifecycleIntentItem{
			{sectionRef: "SS.contract.001", operation: LifecycleActivate},
		}},
	}
}

func effectFPFRevisionFixture(t *testing.T, root ApplyProjectRoot) string {
	t.Helper()
	fpfRoot := filepath.Join(root.String(), "data", "FPF")
	if err := os.MkdirAll(fpfRoot, 0o755); err != nil {
		t.Fatalf("mkdir effect FPF fixture: %v", err)
	}
	runApplyE2EGit(t, fpfRoot, "init")
	runApplyE2EGit(t, fpfRoot, "config", "user.email", "test@example.com")
	runApplyE2EGit(t, fpfRoot, "config", "user.name", "Haft Test")
	fpfSpec := filepath.Join(fpfRoot, "FPF-Spec.md")
	if err := os.WriteFile(fpfSpec, []byte("# Effect FPF fixture\n"), 0o600); err != nil {
		t.Fatalf("write effect FPF fixture: %v", err)
	}
	runApplyE2EGit(t, fpfRoot, "add", "FPF-Spec.md")
	runApplyE2EGit(t, fpfRoot, "commit", "-m", "fixture FPF")
	return strings.TrimSpace(string(runApplyE2EGit(t, fpfRoot, "rev-parse", "HEAD")))
}

func effectGitProvenanceFixture(
	t *testing.T,
	root ApplyProjectRoot,
	carrier SourceCarrierID,
	digest SourceDigest,
) (DesignatedSourceProvenance, gitSourceProvenanceWitness) {
	t.Helper()
	projectRef, err := NewProjectRootRef(root.String())
	if err != nil {
		t.Fatalf("NewProjectRootRef: %v", err)
	}
	commit, err := NewGitCommitOID("sha1:" + strings.Repeat("1", 40))
	if err != nil {
		t.Fatalf("NewGitCommitOID: %v", err)
	}
	origin, err := NewRepositoryEdition(projectRef, commit, carrier, digest)
	if err != nil {
		t.Fatalf("NewRepositoryEdition: %v", err)
	}
	recordRef, err := NewProvenanceRecordRef(".context/source-designation.md")
	if err != nil {
		t.Fatalf("NewProvenanceRecordRef: %v", err)
	}
	recordDigest := ProvenanceRecordDigestOf([]byte("effect test source designation"))
	record, err := NewProvenanceRecordBinding(recordRef, recordDigest)
	if err != nil {
		t.Fatalf("NewProvenanceRecordBinding: %v", err)
	}
	provenance, err := NewDesignatedSourceProvenance(origin, record)
	if err != nil {
		t.Fatalf("NewDesignatedSourceProvenance: %v", err)
	}
	witness := gitSourceProvenanceWitness{
		projectRoot:      root,
		sourceCarrier:    carrier,
		designatedDigest: digest,
		headCommit:       commit,
		origin:           origin,
		resolutionRecord: record,
	}
	canonical, err := encodeGitWitness(witness)
	if err != nil {
		t.Fatalf("encodeGitWitness: %v", err)
	}
	witness.canonical = canonical
	witness.digest = DigestBytes(canonical)
	return provenance, witness
}

func minimalLineagePolicy(
	t *testing.T,
	sourceBytes []byte,
	archive ArchiveCarrierID,
	sourceDigest SourceDigest,
) LineagePolicy {
	t.Helper()
	sectionID, err := NewSourceSectionID("ES.alpha.001")
	if err != nil {
		t.Fatalf("NewSourceSectionID: %v", err)
	}
	length, err := NewByteLength(uint64(len(sourceBytes)))
	if err != nil {
		t.Fatalf("NewByteLength: %v", err)
	}
	span, err := NewExactByteSpan(0, length, FragmentDigestOf(sourceBytes))
	if err != nil {
		t.Fatalf("NewExactByteSpan: %v", err)
	}
	entry := LineageEntry{
		subject: wholeSourceSection{source: sectionID, span: span},
		outcome: retainedAsHistoryOnly{
			archive:       archive,
			sourceEdition: sourceDigest,
			reason:        "preserve exact source history",
		},
	}
	return LineagePolicy{schemaVersion: LineageSchemaVersionV1, entries: []LineageEntry{entry}}
}

func allMigrationEffectPoints() []effectPoint {
	return []effectPoint{
		effectJournalPreparedFileSync,
		effectJournalPreparedRename,
		effectJournalPreparedDirectorySync,
		effectTargetStageFileSync,
		effectTargetStageDirectorySync,
		effectTargetInstallRename,
		effectTargetStageRemovalDirSync,
		effectTargetInstallDirectorySync,
		effectJournalTargetFileSync,
		effectJournalTargetRename,
		effectJournalTargetDirectorySync,
		effectArchiveParentCreate,
		effectArchiveParentParentSync,
		effectSourceArchiveRename,
		effectSourceDirectorySync,
		effectArchiveDirectorySync,
		effectJournalArchiveFileSync,
		effectJournalArchiveRename,
		effectJournalArchiveDirectorySync,
		effectLineageFileSync,
		effectLineageRename,
		effectLineageDirectorySync,
		effectJournalLineageFileSync,
		effectJournalLineageRename,
		effectJournalLineageDirectorySync,
		effectReceiptFileSync,
		effectReceiptRename,
		effectReceiptDirectorySync,
		effectJournalReceiptFileSync,
		effectJournalReceiptRename,
		effectJournalReceiptDirectorySync,
		effectJournalCompleteFileSync,
		effectJournalCompleteRename,
		effectJournalCompleteDirectorySync,
	}
}

func writeFixtureFile(t testing.TB, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func assertFileBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("file %s = %q, want %q", path, got, want)
	}
}

func assertFileDigest(t *testing.T, path string, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	if got := DigestBytes(content).String(); got != want {
		t.Fatalf("digest(%s) = %s, want %s", path, got, want)
	}
}

func readFixtureFile(t testing.TB, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	return content
}

func assertNoDurableJournal(t testing.TB, plan migrationEffectPlan) {
	t.Helper()
	for _, path := range []string{plan.paths.journal, plan.paths.journal + ".tmp"} {
		exists, err := safePathExists(path)
		if err != nil || exists {
			t.Fatalf("journal carrier %s exists=%v err=%v", path, exists, err)
		}
	}
}

func directorySnapshot(t testing.TB, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			snapshot[relative] = "directory"
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snapshot[relative] = "file:" + string(content)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot project directory: %v", err)
	}
	return snapshot
}

func physicalTestRoot(t testing.TB, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", path, err)
	}
	return resolved
}
