package fpfrefresh

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestApplyReceiptCanonicalJSONIsDeterministicAndStrict(t *testing.T) {
	t.Parallel()

	basis := ReceiptBasis{
		Predecessor: ReceiptCoordinates{
			SourceSHA:      strings.Repeat("a", 40),
			DatabaseDigest: "sha256:" + strings.Repeat("1", 64),
		},
		Candidate: ReceiptCoordinates{
			SourceSHA:      strings.Repeat("b", 40),
			DatabaseDigest: "sha256:" + strings.Repeat("2", 64),
		},
		InitialSourceSHA: strings.Repeat("a", 40),
		Targets: ReceiptTargets{
			SourcePath:   "/repo/data/FPF",
			DatabasePath: "/repo/internal/cli/fpf.db",
			LockPath:     "/repo/data/haft/fpf-integration.lock.json",
		},
		Artifacts: ReceiptArtifacts{
			CandidateDatabasePath:         "/repo/.haft/fpf-refresh/candidate-fpf.db",
			CandidateLockPath:             "/repo/.haft/fpf-refresh/candidate-integration.lock.json",
			CandidateLockDigest:           "sha256:" + strings.Repeat("4", 64),
			PredecessorDatabaseBackupPath: "/repo/.haft/fpf-refresh/predecessor-fpf.db",
			PredecessorLock: ReceiptPredecessorLock{
				Presence: ReceiptLockMissing,
			},
		},
	}
	receipt, err := NewApplyReceipt(basis)
	if err != nil {
		t.Fatalf("NewApplyReceipt(): %v", err)
	}

	first, err := receipt.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON() first: %v", err)
	}
	second, err := receipt.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON() second: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("canonical receipt JSON changed between encodes")
	}
	want := `{"schema":"haft.fpf-refresh.apply-receipt/v1","state":"prepared","predecessor":{"source_sha":"` +
		strings.Repeat("a", 40) +
		`","database_digest":"sha256:` +
		strings.Repeat("1", 64) +
		`"},"candidate":{"source_sha":"` +
		strings.Repeat("b", 40) +
		`","database_digest":"sha256:` +
		strings.Repeat("2", 64) +
		`"},"initial_source_sha":"` +
		strings.Repeat("a", 40) +
		`","targets":{"source_path":"/repo/data/FPF","database_path":"/repo/internal/cli/fpf.db","lock_path":"/repo/data/haft/fpf-integration.lock.json"},"artifacts":{"candidate_database_path":"/repo/.haft/fpf-refresh/candidate-fpf.db","candidate_lock_path":"/repo/.haft/fpf-refresh/candidate-integration.lock.json","candidate_lock_digest":"sha256:` +
		strings.Repeat("4", 64) +
		`","predecessor_database_backup_path":"/repo/.haft/fpf-refresh/predecessor-fpf.db","predecessor_lock":{"presence":"missing"}}}` +
		"\n"
	if string(first) != want {
		t.Fatalf("CanonicalJSON() = %q, want %q", first, want)
	}

	decoded, err := DecodeApplyReceipt(first)
	if err != nil {
		t.Fatalf("DecodeApplyReceipt(): %v", err)
	}
	if decoded != receipt {
		t.Fatalf("decoded receipt = %#v, want %#v", decoded, receipt)
	}

	nonCanonical := bytes.TrimSuffix(first, []byte("\n"))
	if _, err := DecodeApplyReceipt(nonCanonical); !errors.Is(err, ErrReceiptCorrupt) {
		t.Fatalf("DecodeApplyReceipt(non-canonical) error = %v, want ErrReceiptCorrupt", err)
	}
	withUnknown := bytes.Replace(
		first,
		[]byte(`"state":"prepared"`),
		[]byte(`"state":"prepared","unexpected":true`),
		1,
	)
	if _, err := DecodeApplyReceipt(withUnknown); !errors.Is(err, ErrReceiptCorrupt) {
		t.Fatalf("DecodeApplyReceipt(unknown field) error = %v, want ErrReceiptCorrupt", err)
	}
}

func TestApplyReceiptTransitionsAreClosedLinearAndIdempotent(t *testing.T) {
	t.Parallel()

	receipt := newReceiptTestReceipt(t, "/repo")
	original, err := receipt.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON(): %v", err)
	}

	same, err := TransitionApplyReceipt(receipt, ReceiptStatePrepared)
	if err != nil {
		t.Fatalf("idempotent prepared transition: %v", err)
	}
	sameJSON, err := same.CanonicalJSON()
	if err != nil {
		t.Fatalf("idempotent CanonicalJSON(): %v", err)
	}
	if !bytes.Equal(sameJSON, original) {
		t.Fatal("idempotent transition changed receipt bytes")
	}
	if _, err := TransitionApplyReceipt(receipt, ReceiptStateDBApplied); !errors.Is(err, ErrReceiptTransition) {
		t.Fatalf("skipped transition error = %v, want ErrReceiptTransition", err)
	}

	sequence := []ReceiptState{
		ReceiptStateSourceApplied,
		ReceiptStateDBApplied,
		ReceiptStateLockApplied,
		ReceiptStateVerified,
		ReceiptStateComplete,
	}
	for _, state := range sequence {
		receipt, err = TransitionApplyReceipt(receipt, state)
		if err != nil {
			t.Fatalf("transition to %q: %v", state, err)
		}
		replayed, replayErr := TransitionApplyReceipt(receipt, state)
		if replayErr != nil {
			t.Fatalf("replay transition to %q: %v", state, replayErr)
		}
		if replayed != receipt {
			t.Fatalf("replay transition to %q changed receipt", state)
		}
		if state != ReceiptStateComplete {
			if _, reverseErr := TransitionApplyReceipt(receipt, ReceiptStatePrepared); !errors.Is(reverseErr, ErrReceiptTransition) {
				t.Fatalf("reverse transition from %q error = %v, want ErrReceiptTransition", state, reverseErr)
			}
		}
	}
	if _, err := TransitionApplyReceipt(receipt, ReceiptStateVerified); !errors.Is(err, ErrReceiptTransition) {
		t.Fatalf("reopen complete receipt error = %v, want ErrReceiptTransition", err)
	}
	if _, err := TransitionApplyReceipt(receipt, ReceiptStateRestored); !errors.Is(err, ErrReceiptTransition) {
		t.Fatalf("cross complete to restored error = %v, want ErrReceiptTransition", err)
	}
}

func TestApplyReceiptRecoveryDirectionsAreExactForEveryState(t *testing.T) {
	t.Parallel()

	receipt := newReceiptTestReceipt(t, "/repo")
	states := []struct {
		state        ReceiptState
		resumeKinds  []ReceiptRecoveryStepKind
		restoreKinds []ReceiptRecoveryStepKind
		required     bool
	}{
		{
			state: ReceiptStatePrepared,
			resumeKinds: []ReceiptRecoveryStepKind{
				RecoveryApplyCandidateSource,
				RecoveryApplyCandidateDatabase,
				RecoveryMaterializeCandidateLock,
				RecoveryVerifyCandidatePair,
				RecoveryMarkReceiptComplete,
			},
			restoreKinds: []ReceiptRecoveryStepKind{
				RecoveryRestorePredecessorSource,
				RecoveryVerifyPredecessorPair,
				RecoveryMarkReceiptRestored,
			},
			required: true,
		},
		{
			state: ReceiptStateSourceApplied,
			resumeKinds: []ReceiptRecoveryStepKind{
				RecoveryApplyCandidateDatabase,
				RecoveryMaterializeCandidateLock,
				RecoveryVerifyCandidatePair,
				RecoveryMarkReceiptComplete,
			},
			restoreKinds: []ReceiptRecoveryStepKind{
				RecoveryRestorePredecessorDatabase,
				RecoveryRestorePredecessorSource,
				RecoveryVerifyPredecessorPair,
				RecoveryMarkReceiptRestored,
			},
			required: true,
		},
		{
			state: ReceiptStateDBApplied,
			resumeKinds: []ReceiptRecoveryStepKind{
				RecoveryMaterializeCandidateLock,
				RecoveryVerifyCandidatePair,
				RecoveryMarkReceiptComplete,
			},
			restoreKinds: []ReceiptRecoveryStepKind{
				RecoveryRemoveCandidateLock,
				RecoveryRestorePredecessorDatabase,
				RecoveryRestorePredecessorSource,
				RecoveryVerifyPredecessorPair,
				RecoveryMarkReceiptRestored,
			},
			required: true,
		},
		{
			state: ReceiptStateLockApplied,
			resumeKinds: []ReceiptRecoveryStepKind{
				RecoveryVerifyCandidatePair,
				RecoveryMarkReceiptComplete,
			},
			restoreKinds: []ReceiptRecoveryStepKind{
				RecoveryRemoveCandidateLock,
				RecoveryRestorePredecessorDatabase,
				RecoveryRestorePredecessorSource,
				RecoveryVerifyPredecessorPair,
				RecoveryMarkReceiptRestored,
			},
			required: true,
		},
		{
			state: ReceiptStateVerified,
			resumeKinds: []ReceiptRecoveryStepKind{
				RecoveryMarkReceiptComplete,
			},
			restoreKinds: []ReceiptRecoveryStepKind{
				RecoveryRemoveCandidateLock,
				RecoveryRestorePredecessorDatabase,
				RecoveryRestorePredecessorSource,
				RecoveryVerifyPredecessorPair,
				RecoveryMarkReceiptRestored,
			},
			required: true,
		},
		{
			state:        ReceiptStateComplete,
			resumeKinds:  []ReceiptRecoveryStepKind{},
			restoreKinds: []ReceiptRecoveryStepKind{},
			required:     false,
		},
	}

	allowedTargets := map[string]bool{
		receipt.Targets.SourcePath:   true,
		receipt.Targets.DatabasePath: true,
		receipt.Targets.LockPath:     true,
	}
	for index, testCase := range states {
		if index > 0 {
			var err error
			receipt, err = TransitionApplyReceipt(receipt, testCase.state)
			if err != nil {
				t.Fatalf("transition to %q: %v", testCase.state, err)
			}
		}
		directions, err := receipt.RecoveryDirections()
		if err != nil {
			t.Fatalf("RecoveryDirections(%q): %v", testCase.state, err)
		}
		if directions.CurrentState != testCase.state {
			t.Fatalf("directions state = %q, want %q", directions.CurrentState, testCase.state)
		}
		if directions.Required != testCase.required {
			t.Fatalf("directions required at %q = %t, want %t", testCase.state, directions.Required, testCase.required)
		}
		if got := receiptRecoveryKinds(directions.Resume); !reflect.DeepEqual(got, testCase.resumeKinds) {
			t.Fatalf("resume kinds at %q = %#v, want %#v", testCase.state, got, testCase.resumeKinds)
		}
		if got := receiptRecoveryKinds(directions.Restore); !reflect.DeepEqual(got, testCase.restoreKinds) {
			t.Fatalf("restore kinds at %q = %#v, want %#v", testCase.state, got, testCase.restoreKinds)
		}
		for _, step := range append(
			append([]ReceiptRecoveryStep{}, directions.Resume...),
			directions.Restore...,
		) {
			for _, target := range step.TargetPaths {
				if !allowedTargets[target] {
					t.Fatalf("step %q escaped receipt targets with %q", step.Kind, target)
				}
			}
			assertReceiptRecoveryIdentity(t, receipt, step)
		}
	}
}

func TestApplyReceiptPreparedMixedCheckoutHasExactResumeAndRestore(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	basis := newReceiptTestReceipt(t, root).Basis()
	basis.InitialSourceSHA = basis.Candidate.SourceSHA
	receipt, err := NewApplyReceipt(basis)
	if err != nil {
		t.Fatalf("NewApplyReceipt(mixed checkout): %v", err)
	}
	directions, err := receipt.RecoveryDirections()
	if err != nil {
		t.Fatalf("RecoveryDirections(): %v", err)
	}
	firstResume := directions.Resume[0]
	if firstResume.Kind != RecoveryApplyCandidateSource ||
		firstResume.ExpectedSourceSHA != basis.Candidate.SourceSHA ||
		firstResume.SourceSHA != basis.Candidate.SourceSHA ||
		firstResume.ResultState != ReceiptStateSourceApplied {
		t.Fatalf("mixed-checkout resume source step is not idempotent: %#v", firstResume)
	}
	if got := receiptRecoveryKinds(directions.Restore); !reflect.DeepEqual(
		got,
		[]ReceiptRecoveryStepKind{
			RecoveryRestorePredecessorSource,
			RecoveryVerifyPredecessorPair,
			RecoveryMarkReceiptRestored,
		},
	) {
		t.Fatalf("mixed-checkout restore kinds = %#v", got)
	}
	restoreSource := directions.Restore[0]
	if restoreSource.ExpectedSourceSHA != basis.Candidate.SourceSHA ||
		restoreSource.SourceSHA != basis.Predecessor.SourceSHA ||
		!reflect.DeepEqual(
			restoreSource.TargetPaths,
			[]string{basis.Targets.SourcePath},
		) {
		t.Fatalf("mixed-checkout restore source step is inexact: %#v", restoreSource)
	}
	path := filepath.Join(root, "mixed-apply-receipt.json")
	if err := CreateReceipt(path, receipt); err != nil {
		t.Fatalf("CreateReceipt(mixed checkout): %v", err)
	}
	loaded, err := LoadReceiptFor(path, basis)
	if err != nil {
		t.Fatalf("LoadReceiptFor(mixed checkout): %v", err)
	}
	if loaded.InitialSourceSHA != basis.Candidate.SourceSHA {
		t.Fatalf(
			"loaded initial source SHA = %q, want candidate %q",
			loaded.InitialSourceSHA,
			basis.Candidate.SourceSHA,
		)
	}
}

func TestApplyReceiptRestoredIsTerminalAndIdempotent(t *testing.T) {
	t.Parallel()

	receipt := newReceiptTestReceipt(t, "/repo")
	var err error
	receipt, err = TransitionApplyReceipt(receipt, ReceiptStateSourceApplied)
	if err != nil {
		t.Fatalf("transition to source-applied: %v", err)
	}
	restored, err := TransitionApplyReceipt(receipt, ReceiptStateRestored)
	if err != nil {
		t.Fatalf("transition to restored: %v", err)
	}
	replayed, err := TransitionApplyReceipt(restored, ReceiptStateRestored)
	if err != nil {
		t.Fatalf("replay restored: %v", err)
	}
	if replayed != restored {
		t.Fatal("restored replay changed receipt")
	}
	directions, err := restored.RecoveryDirections()
	if err != nil {
		t.Fatalf("restored RecoveryDirections(): %v", err)
	}
	if directions.Required ||
		len(directions.Resume) != 0 ||
		len(directions.Restore) != 0 {
		t.Fatalf("restored receipt still requires recovery: %#v", directions)
	}
	for _, next := range []ReceiptState{
		ReceiptStatePrepared,
		ReceiptStateSourceApplied,
		ReceiptStateComplete,
	} {
		if _, err := TransitionApplyReceipt(restored, next); !errors.Is(err, ErrReceiptTransition) {
			t.Fatalf("restored -> %q error = %v, want ErrReceiptTransition", next, err)
		}
	}
}

func TestApplyReceiptRestoreDistinguishesPresentAndFirstEverLock(t *testing.T) {
	t.Parallel()

	missing := newReceiptTestReceipt(t, "/repo")
	for _, state := range []ReceiptState{
		ReceiptStateSourceApplied,
		ReceiptStateDBApplied,
		ReceiptStateLockApplied,
	} {
		var err error
		missing, err = TransitionApplyReceipt(missing, state)
		if err != nil {
			t.Fatalf("transition missing-lock receipt to %q: %v", state, err)
		}
	}
	missingDirections, err := missing.RecoveryDirections()
	if err != nil {
		t.Fatalf("missing-lock RecoveryDirections(): %v", err)
	}
	if got := missingDirections.Restore[0]; got.Kind != RecoveryRemoveCandidateLock ||
		got.ExpectedLockPresence != ReceiptLockPresent ||
		got.LockPresence != ReceiptLockMissing ||
		got.ArtifactPath != "" ||
		got.ExpectedArtifactDigest != missing.Artifacts.CandidateLockDigest ||
		got.ArtifactDigest != "" {
		t.Fatalf("first-ever lock restore step = %#v, want exact removal", got)
	}

	presentBasis := missing.Basis()
	presentBasis.Artifacts.PredecessorLock = ReceiptPredecessorLock{
		Presence:   ReceiptLockPresent,
		BackupPath: "/repo/.haft/fpf-refresh/predecessor-integration.lock.json",
		Digest:     "sha256:" + strings.Repeat("3", 64),
	}
	present, err := NewApplyReceipt(presentBasis)
	if err != nil {
		t.Fatalf("NewApplyReceipt(present lock): %v", err)
	}
	for _, state := range []ReceiptState{
		ReceiptStateSourceApplied,
		ReceiptStateDBApplied,
		ReceiptStateLockApplied,
	} {
		present, err = TransitionApplyReceipt(present, state)
		if err != nil {
			t.Fatalf("transition present-lock receipt to %q: %v", state, err)
		}
	}
	presentDirections, err := present.RecoveryDirections()
	if err != nil {
		t.Fatalf("present-lock RecoveryDirections(): %v", err)
	}
	got := presentDirections.Restore[0]
	if got.Kind != RecoveryRestorePredecessorLock ||
		got.ExpectedLockPresence != ReceiptLockPresent ||
		got.LockPresence != ReceiptLockPresent ||
		got.ArtifactPath != presentBasis.Artifacts.PredecessorLock.BackupPath ||
		got.ExpectedArtifactDigest != presentBasis.Artifacts.CandidateLockDigest ||
		got.ArtifactDigest != presentBasis.Artifacts.PredecessorLock.Digest ||
		got.SourceSHA != presentBasis.Predecessor.SourceSHA ||
		got.DatabaseDigest != presentBasis.Predecessor.DatabaseDigest {
		t.Fatalf("present predecessor lock restore step is inexact: %#v", got)
	}
}

func TestReceiptStoreCreatesAdvancesAndReplaysCompleteReceipt(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	receipt := newReceiptTestReceipt(t, root)
	path := filepath.Join(root, "apply-receipt.json")

	if err := CreateReceipt(path, receipt); err != nil {
		t.Fatalf("CreateReceipt(): %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat receipt: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("receipt mode = %04o, want 0600", got)
	}
	if err := CreateReceipt(path, receipt); !errors.Is(err, ErrReceiptExists) {
		t.Fatalf("duplicate CreateReceipt() error = %v, want ErrReceiptExists", err)
	}

	loaded, err := LoadReceiptFor(path, receipt.Basis())
	if err != nil {
		t.Fatalf("LoadReceiptFor(): %v", err)
	}
	if loaded != receipt {
		t.Fatalf("loaded receipt = %#v, want %#v", loaded, receipt)
	}
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read receipt: %v", err)
	}
	canonical, err := receipt.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON(): %v", err)
	}
	if !bytes.Equal(onDisk, canonical) {
		t.Fatal("stored receipt bytes are not canonical")
	}

	if _, err := AdvanceReceipt(path, receipt.Basis(), ReceiptStateDBApplied); !errors.Is(err, ErrReceiptTransition) {
		t.Fatalf("persisted skipped transition error = %v, want ErrReceiptTransition", err)
	}
	unchanged, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read receipt after rejected transition: %v", err)
	}
	if !bytes.Equal(unchanged, onDisk) {
		t.Fatal("rejected persisted transition changed receipt bytes")
	}

	for _, state := range []ReceiptState{
		ReceiptStateSourceApplied,
		ReceiptStateDBApplied,
		ReceiptStateLockApplied,
		ReceiptStateVerified,
		ReceiptStateComplete,
	} {
		advanced, advanceErr := AdvanceReceipt(path, receipt.Basis(), state)
		if advanceErr != nil {
			t.Fatalf("AdvanceReceipt(%q): %v", state, advanceErr)
		}
		if advanced.State != state {
			t.Fatalf("advanced state = %q, want %q", advanced.State, state)
		}
		replayed, replayErr := AdvanceReceipt(path, receipt.Basis(), state)
		if replayErr != nil {
			t.Fatalf("AdvanceReceipt replay(%q): %v", state, replayErr)
		}
		if replayed != advanced {
			t.Fatalf("replayed receipt at %q changed", state)
		}
	}

	completeBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read complete receipt: %v", err)
	}
	replayed, err := AdvanceReceipt(path, receipt.Basis(), ReceiptStateComplete)
	if err != nil {
		t.Fatalf("replay completed receipt: %v", err)
	}
	if replayed.State != ReceiptStateComplete {
		t.Fatalf("replayed state = %q, want complete", replayed.State)
	}
	afterReplay, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replayed receipt: %v", err)
	}
	if !bytes.Equal(afterReplay, completeBytes) {
		t.Fatal("successful receipt replay rewrote canonical bytes")
	}
	assertNoReceiptPersistenceDebt(t, path)
}

func TestReceiptStoreClosesAndReplaysRestoredReceipt(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	receipt := newReceiptTestReceipt(t, root)
	path := filepath.Join(root, "apply-receipt.json")
	if err := CreateReceipt(path, receipt); err != nil {
		t.Fatalf("CreateReceipt(): %v", err)
	}
	if _, err := AdvanceReceipt(
		path,
		receipt.Basis(),
		ReceiptStateSourceApplied,
	); err != nil {
		t.Fatalf("AdvanceReceipt(source-applied): %v", err)
	}
	restored, err := AdvanceReceipt(path, receipt.Basis(), ReceiptStateRestored)
	if err != nil {
		t.Fatalf("AdvanceReceipt(restored): %v", err)
	}
	if restored.State != ReceiptStateRestored {
		t.Fatalf("restored state = %q, want restored", restored.State)
	}
	beforeReplay, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read restored receipt: %v", err)
	}
	replayed, err := AdvanceReceipt(path, receipt.Basis(), ReceiptStateRestored)
	if err != nil {
		t.Fatalf("replay restored receipt: %v", err)
	}
	if replayed != restored {
		t.Fatal("persisted restored replay changed receipt value")
	}
	afterReplay, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replayed restored receipt: %v", err)
	}
	if !bytes.Equal(beforeReplay, afterReplay) {
		t.Fatal("persisted restored replay changed receipt bytes")
	}
	directions, err := replayed.RecoveryDirections()
	if err != nil {
		t.Fatalf("RecoveryDirections(restored): %v", err)
	}
	if directions.Required {
		t.Fatal("persisted restored receipt still returns RecoveryRequired")
	}
	if _, err := AdvanceReceipt(
		path,
		receipt.Basis(),
		ReceiptStateComplete,
	); !errors.Is(err, ErrReceiptTransition) {
		t.Fatalf("restored -> complete error = %v, want ErrReceiptTransition", err)
	}
	assertNoReceiptPersistenceDebt(t, path)
}

func TestReceiptStoreRejectsStaleAndCorruptReceipts(t *testing.T) {
	t.Parallel()

	t.Run("stale exact basis", func(t *testing.T) {
		root := t.TempDir()
		receipt := newReceiptTestReceipt(t, root)
		path := filepath.Join(root, "apply-receipt.json")
		if err := CreateReceipt(path, receipt); err != nil {
			t.Fatalf("CreateReceipt(): %v", err)
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read receipt: %v", err)
		}

		staleBasis := receipt.Basis()
		staleBasis.Candidate.SourceSHA = strings.Repeat("c", 40)
		if _, err := LoadReceiptFor(path, staleBasis); !errors.Is(err, ErrReceiptStale) {
			t.Fatalf("LoadReceiptFor(stale) error = %v, want ErrReceiptStale", err)
		}
		if _, err := AdvanceReceipt(path, staleBasis, ReceiptStateSourceApplied); !errors.Is(err, ErrReceiptStale) {
			t.Fatalf("AdvanceReceipt(stale) error = %v, want ErrReceiptStale", err)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read unchanged receipt: %v", err)
		}
		if !bytes.Equal(after, before) {
			t.Fatal("stale advance changed receipt bytes")
		}
	})

	for _, testCase := range []struct {
		name    string
		content func(ApplyReceipt) []byte
	}{
		{
			name: "truncated JSON",
			content: func(ApplyReceipt) []byte {
				return []byte(`{"schema":`)
			},
		},
		{
			name: "unknown field",
			content: func(receipt ApplyReceipt) []byte {
				canonical, err := receipt.CanonicalJSON()
				if err != nil {
					t.Fatalf("CanonicalJSON(): %v", err)
				}
				return bytes.Replace(
					canonical,
					[]byte(`"state":"prepared"`),
					[]byte(`"state":"prepared","foreign":true`),
					1,
				)
			},
		},
		{
			name: "non-canonical bytes",
			content: func(receipt ApplyReceipt) []byte {
				canonical, err := receipt.CanonicalJSON()
				if err != nil {
					t.Fatalf("CanonicalJSON(): %v", err)
				}
				return bytes.TrimSpace(canonical)
			},
		},
		{
			name: "invalid state",
			content: func(receipt ApplyReceipt) []byte {
				canonical, err := receipt.CanonicalJSON()
				if err != nil {
					t.Fatalf("CanonicalJSON(): %v", err)
				}
				return bytes.Replace(
					canonical,
					[]byte(`"state":"prepared"`),
					[]byte(`"state":"guessed-from-timestamp"`),
					1,
				)
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			receipt := newReceiptTestReceipt(t, root)
			path := filepath.Join(root, "apply-receipt.json")
			if err := os.WriteFile(path, testCase.content(receipt), 0o600); err != nil {
				t.Fatalf("write corrupt receipt: %v", err)
			}
			if _, err := LoadReceipt(path); !errors.Is(err, ErrReceiptCorrupt) {
				t.Fatalf("LoadReceipt(corrupt) error = %v, want ErrReceiptCorrupt", err)
			}
		})
	}
}

func TestCreateReceiptIsExclusiveUnderConcurrency(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	receipt := newReceiptTestReceipt(t, root)
	path := filepath.Join(root, "apply-receipt.json")

	const callers = 32
	start := make(chan struct{})
	results := make(chan error, callers)
	var wait sync.WaitGroup
	wait.Add(callers)
	for range callers {
		go func() {
			defer wait.Done()
			<-start
			results <- CreateReceipt(path, receipt)
		}()
	}
	close(start)
	wait.Wait()
	close(results)

	successes := 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrReceiptBusy), errors.Is(err, ErrReceiptExists):
		default:
			t.Errorf("concurrent CreateReceipt() error = %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent CreateReceipt() successes = %d, want 1", successes)
	}
	loaded, err := LoadReceiptFor(path, receipt.Basis())
	if err != nil {
		t.Fatalf("LoadReceiptFor() after concurrent create: %v", err)
	}
	if loaded.State != ReceiptStatePrepared {
		t.Fatalf("concurrently created receipt state = %q, want prepared", loaded.State)
	}
	assertNoReceiptPersistenceDebt(t, path)
}

func TestCreateReceiptReusesUnlockedPersistentLockCarrier(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	receipt := newReceiptTestReceipt(t, root)
	path := filepath.Join(root, "apply-receipt.json")
	if err := os.WriteFile(path+".lock", nil, 0o600); err != nil {
		t.Fatalf("write unlocked lock carrier: %v", err)
	}

	if err := CreateReceipt(path, receipt); err != nil {
		t.Fatalf("CreateReceipt() with prior unlocked carrier: %v", err)
	}
	loaded, err := LoadReceiptFor(path, receipt.Basis())
	if err != nil {
		t.Fatalf("LoadReceiptFor(): %v", err)
	}
	if loaded != receipt {
		t.Fatalf("loaded receipt = %#v, want %#v", loaded, receipt)
	}
	assertNoReceiptPersistenceDebt(t, path)
}

func TestApplyReceiptRejectsInexactIdentityAndTargetPaths(t *testing.T) {
	t.Parallel()

	valid := newReceiptTestReceipt(t, "/repo").Basis()
	tests := []struct {
		name   string
		mutate func(*ReceiptBasis)
	}{
		{
			name: "abbreviated source SHA",
			mutate: func(basis *ReceiptBasis) {
				basis.Candidate.SourceSHA = "abc123"
			},
		},
		{
			name: "uppercase source SHA",
			mutate: func(basis *ReceiptBasis) {
				basis.Candidate.SourceSHA = strings.Repeat("A", 40)
			},
		},
		{
			name: "initial source is outside closed pair",
			mutate: func(basis *ReceiptBasis) {
				basis.InitialSourceSHA = strings.Repeat("c", 40)
			},
		},
		{
			name: "digest without algorithm",
			mutate: func(basis *ReceiptBasis) {
				basis.Predecessor.DatabaseDigest = strings.Repeat("1", 64)
			},
		},
		{
			name: "relative target",
			mutate: func(basis *ReceiptBasis) {
				basis.Targets.LockPath = "data/haft/fpf-integration.lock.json"
			},
		},
		{
			name: "unclean target",
			mutate: func(basis *ReceiptBasis) {
				basis.Targets.DatabasePath = "/repo/internal/../internal/cli/fpf.db"
			},
		},
		{
			name: "duplicate target",
			mutate: func(basis *ReceiptBasis) {
				basis.Targets.LockPath = basis.Targets.DatabasePath
			},
		},
		{
			name: "missing lock carries backup",
			mutate: func(basis *ReceiptBasis) {
				basis.Artifacts.PredecessorLock.BackupPath =
					"/repo/.haft/fpf-refresh/impossible-lock-backup.json"
				basis.Artifacts.PredecessorLock.Digest =
					"sha256:" + strings.Repeat("3", 64)
			},
		},
		{
			name: "present lock lacks exact digest",
			mutate: func(basis *ReceiptBasis) {
				basis.Artifacts.PredecessorLock = ReceiptPredecessorLock{
					Presence:   ReceiptLockPresent,
					BackupPath: "/repo/.haft/fpf-refresh/predecessor-lock.json",
				}
			},
		},
		{
			name: "database backup collides with target",
			mutate: func(basis *ReceiptBasis) {
				basis.Artifacts.PredecessorDatabaseBackupPath =
					basis.Targets.DatabasePath
			},
		},
		{
			name: "candidate lock lacks exact digest",
			mutate: func(basis *ReceiptBasis) {
				basis.Artifacts.CandidateLockDigest = "sha256:short"
			},
		},
		{
			name: "candidate lock artifact collides with database artifact",
			mutate: func(basis *ReceiptBasis) {
				basis.Artifacts.CandidateLockPath =
					basis.Artifacts.CandidateDatabasePath
			},
		},
		{
			name: "artifact is inside source checkout",
			mutate: func(basis *ReceiptBasis) {
				basis.Artifacts.CandidateDatabasePath =
					"/repo/data/FPF/candidate-fpf.db"
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			basis := valid
			testCase.mutate(&basis)
			if _, err := NewApplyReceipt(basis); !errors.Is(err, ErrReceiptInvalid) {
				t.Fatalf("NewApplyReceipt() error = %v, want ErrReceiptInvalid", err)
			}
		})
	}
}

func newReceiptTestReceipt(t *testing.T, root string) ApplyReceipt {
	t.Helper()
	basis := ReceiptBasis{
		Predecessor: ReceiptCoordinates{
			SourceSHA:      strings.Repeat("a", 40),
			DatabaseDigest: "sha256:" + strings.Repeat("1", 64),
		},
		Candidate: ReceiptCoordinates{
			SourceSHA:      strings.Repeat("b", 40),
			DatabaseDigest: "sha256:" + strings.Repeat("2", 64),
		},
		InitialSourceSHA: strings.Repeat("a", 40),
		Targets: ReceiptTargets{
			SourcePath:   filepath.Join(root, "data", "FPF"),
			DatabasePath: filepath.Join(root, "internal", "cli", "fpf.db"),
			LockPath:     filepath.Join(root, "data", "haft", "fpf-integration.lock.json"),
		},
		Artifacts: ReceiptArtifacts{
			CandidateDatabasePath: filepath.Join(
				root,
				".haft",
				"fpf-refresh",
				"candidate-fpf.db",
			),
			CandidateLockPath: filepath.Join(
				root,
				".haft",
				"fpf-refresh",
				"candidate-integration.lock.json",
			),
			CandidateLockDigest: "sha256:" + strings.Repeat("4", 64),
			PredecessorDatabaseBackupPath: filepath.Join(
				root,
				".haft",
				"fpf-refresh",
				"predecessor-fpf.db",
			),
			PredecessorLock: ReceiptPredecessorLock{
				Presence: ReceiptLockMissing,
			},
		},
	}
	receipt, err := NewApplyReceipt(basis)
	if err != nil {
		t.Fatalf("NewApplyReceipt(): %v", err)
	}
	return receipt
}

func receiptRecoveryKinds(steps []ReceiptRecoveryStep) []ReceiptRecoveryStepKind {
	kinds := make([]ReceiptRecoveryStepKind, len(steps))
	for index, step := range steps {
		kinds[index] = step.Kind
	}
	return kinds
}

func assertReceiptRecoveryIdentity(
	t *testing.T,
	receipt ApplyReceipt,
	step ReceiptRecoveryStep,
) {
	t.Helper()
	switch step.Kind {
	case RecoveryApplyCandidateSource:
		if step.ExpectedSourceSHA != receipt.InitialSourceSHA ||
			step.SourceSHA != receipt.Candidate.SourceSHA ||
			step.DatabaseDigest != "" ||
			!reflect.DeepEqual(step.TargetPaths, []string{receipt.Targets.SourcePath}) ||
			step.ResultState != ReceiptStateSourceApplied {
			t.Fatalf("candidate source step is inexact: %#v", step)
		}
	case RecoveryApplyCandidateDatabase:
		if step.SourceSHA != "" ||
			step.ExpectedDatabaseDigest != receipt.Predecessor.DatabaseDigest ||
			step.DatabaseDigest != receipt.Candidate.DatabaseDigest ||
			step.ArtifactPath != receipt.Artifacts.CandidateDatabasePath ||
			step.ArtifactDigest != receipt.Candidate.DatabaseDigest ||
			!reflect.DeepEqual(step.TargetPaths, []string{receipt.Targets.DatabasePath}) ||
			step.ResultState != ReceiptStateDBApplied {
			t.Fatalf("candidate database step is inexact: %#v", step)
		}
	case RecoveryMaterializeCandidateLock:
		if step.SourceSHA != receipt.Candidate.SourceSHA ||
			step.DatabaseDigest != receipt.Candidate.DatabaseDigest ||
			step.ArtifactPath != receipt.Artifacts.CandidateLockPath ||
			step.ExpectedArtifactDigest != receipt.Artifacts.PredecessorLock.Digest ||
			step.ArtifactDigest != receipt.Artifacts.CandidateLockDigest ||
			step.ExpectedLockPresence != receipt.Artifacts.PredecessorLock.Presence ||
			step.LockPresence != ReceiptLockPresent ||
			!reflect.DeepEqual(step.TargetPaths, []string{receipt.Targets.LockPath}) ||
			step.ResultState != ReceiptStateLockApplied {
			t.Fatalf("candidate lock step is inexact: %#v", step)
		}
	case RecoveryVerifyCandidatePair:
		if step.SourceSHA != receipt.Candidate.SourceSHA ||
			step.DatabaseDigest != receipt.Candidate.DatabaseDigest ||
			step.ArtifactDigest != receipt.Artifacts.CandidateLockDigest ||
			step.LockPresence != ReceiptLockPresent ||
			step.ResultState != ReceiptStateVerified {
			t.Fatalf("candidate verification step is inexact: %#v", step)
		}
	case RecoveryMarkReceiptComplete:
		if step.SourceSHA != receipt.Candidate.SourceSHA ||
			step.DatabaseDigest != receipt.Candidate.DatabaseDigest ||
			step.ArtifactDigest != receipt.Artifacts.CandidateLockDigest ||
			step.LockPresence != ReceiptLockPresent ||
			len(step.TargetPaths) != 0 ||
			step.ResultState != ReceiptStateComplete {
			t.Fatalf("completion step is inexact: %#v", step)
		}
	case RecoveryRestorePredecessorSource:
		if step.ExpectedSourceSHA != receipt.Candidate.SourceSHA ||
			step.SourceSHA != receipt.Predecessor.SourceSHA ||
			step.DatabaseDigest != "" ||
			!reflect.DeepEqual(step.TargetPaths, []string{receipt.Targets.SourcePath}) ||
			step.ResultState != "" {
			t.Fatalf("predecessor source step is inexact: %#v", step)
		}
	case RecoveryRestorePredecessorDatabase:
		if step.SourceSHA != "" ||
			step.ExpectedDatabaseDigest != receipt.Candidate.DatabaseDigest ||
			step.DatabaseDigest != receipt.Predecessor.DatabaseDigest ||
			step.ArtifactPath != receipt.Artifacts.PredecessorDatabaseBackupPath ||
			step.ArtifactDigest != receipt.Predecessor.DatabaseDigest ||
			!reflect.DeepEqual(step.TargetPaths, []string{receipt.Targets.DatabasePath}) ||
			step.ResultState != "" {
			t.Fatalf("predecessor database step is inexact: %#v", step)
		}
	case RecoveryRestorePredecessorLock:
		if step.SourceSHA != receipt.Predecessor.SourceSHA ||
			step.DatabaseDigest != receipt.Predecessor.DatabaseDigest ||
			step.ArtifactPath != receipt.Artifacts.PredecessorLock.BackupPath ||
			step.ExpectedArtifactDigest != receipt.Artifacts.CandidateLockDigest ||
			step.ArtifactDigest != receipt.Artifacts.PredecessorLock.Digest ||
			step.ExpectedLockPresence != ReceiptLockPresent ||
			step.LockPresence != ReceiptLockPresent ||
			!reflect.DeepEqual(step.TargetPaths, []string{receipt.Targets.LockPath}) ||
			step.ResultState != "" {
			t.Fatalf("predecessor lock step is inexact: %#v", step)
		}
	case RecoveryRemoveCandidateLock:
		if step.SourceSHA != "" ||
			step.DatabaseDigest != "" ||
			step.ArtifactPath != "" ||
			step.ExpectedArtifactDigest != receipt.Artifacts.CandidateLockDigest ||
			step.ArtifactDigest != "" ||
			step.ExpectedLockPresence != ReceiptLockPresent ||
			step.LockPresence != ReceiptLockMissing ||
			!reflect.DeepEqual(step.TargetPaths, []string{receipt.Targets.LockPath}) ||
			step.ResultState != "" {
			t.Fatalf("missing predecessor lock removal step is inexact: %#v", step)
		}
	case RecoveryVerifyPredecessorPair:
		if step.SourceSHA != receipt.Predecessor.SourceSHA ||
			step.DatabaseDigest != receipt.Predecessor.DatabaseDigest ||
			step.ArtifactDigest != receipt.Artifacts.PredecessorLock.Digest ||
			step.LockPresence != receipt.Artifacts.PredecessorLock.Presence ||
			step.ResultState != "" {
			t.Fatalf("predecessor verification step is inexact: %#v", step)
		}
	case RecoveryMarkReceiptRestored:
		if step.SourceSHA != receipt.Predecessor.SourceSHA ||
			step.DatabaseDigest != receipt.Predecessor.DatabaseDigest ||
			step.ArtifactDigest != receipt.Artifacts.PredecessorLock.Digest ||
			step.LockPresence != receipt.Artifacts.PredecessorLock.Presence ||
			len(step.TargetPaths) != 0 ||
			step.ResultState != ReceiptStateRestored {
			t.Fatalf("restored closure step is inexact: %#v", step)
		}
	default:
		t.Fatalf("unexpected recovery step kind %q", step.Kind)
	}
}

func assertNoReceiptPersistenceDebt(t *testing.T, path string) {
	t.Helper()
	lockInfo, err := os.Lstat(path + ".lock")
	if err != nil {
		t.Fatalf("stat persistent receipt lock carrier: %v", err)
	}
	if !lockInfo.Mode().IsRegular() || lockInfo.Mode().Perm() != 0o600 {
		t.Fatalf(
			"receipt lock carrier mode = %v, want private regular 0600",
			lockInfo.Mode(),
		)
	}
	lockContent, err := os.ReadFile(path + ".lock")
	if err != nil {
		t.Fatalf("read persistent receipt lock carrier: %v", err)
	}
	if len(lockContent) != 0 {
		t.Fatalf("receipt lock carrier contains unexpected bytes: %q", lockContent)
	}
	stages, err := filepath.Glob(
		filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*"),
	)
	if err != nil {
		t.Fatalf("glob receipt stages: %v", err)
	}
	if len(stages) != 0 {
		t.Fatalf("receipt stages remain: %v", stages)
	}
}
