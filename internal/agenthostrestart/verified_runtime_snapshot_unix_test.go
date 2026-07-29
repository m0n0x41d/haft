//go:build darwin || linux

package agenthostrestart

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadVerifiedRuntimeSnapshotProjectsOnlySafeEvidence(t *testing.T) {
	root := newRestartProject(t)
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	prepared := newPreparedFixture(
		t,
		root,
		"restart-safe-snapshot",
		digestOf('d'),
	)
	if err := store.Prepare(prepared); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if _, err := LoadVerifiedRuntimeSnapshot(root); err == nil {
		t.Fatal("prepared checkpoint produced verified runtime evidence")
	}
	submission, err := MarkSubmitted(prepared)
	if err != nil {
		t.Fatalf("MarkSubmitted: %v", err)
	}
	if err := store.Apply(submission); err != nil {
		t.Fatalf("apply submitted: %v", err)
	}
	permit, err := AuthorizeQuit(
		submission.After(),
		validInstallObservation(submission.After()),
	)
	if err != nil {
		t.Fatalf("AuthorizeQuit: %v", err)
	}
	opening, err := MarkAppOpened(
		submission.After(),
		permit,
		AppOpenObservation{
			GracefulQuitSucceeded: true,
			OldApplicationAbsent:  true,
			OldTaskRuntimeAbsent:  true,
			NewApplicationStarted: true,
			DeepLinkOpened:        "codex://threads/" + prepared.threadID,
			ApplicationStartedAt:  prepared.createdAt.Add(time.Minute),
		},
	)
	if err != nil {
		t.Fatalf("MarkAppOpened: %v", err)
	}
	if err := store.Apply(opening); err != nil {
		t.Fatalf("apply app-opened: %v", err)
	}
	resumption, err := MarkResumed(
		opening.After(),
		ResumeObservation{
			ThreadID:           prepared.threadID,
			ResumeIntentDigest: prepared.resumeIntentDigest,
			RepositoryRoot:     root,
		},
	)
	if err != nil {
		t.Fatalf("MarkResumed: %v", err)
	}
	if err := store.Apply(resumption); err != nil {
		t.Fatalf("apply resumed: %v", err)
	}
	resumed := resumption.After()
	verification := validRuntimeVerification(resumed)
	if _, err := store.RecordResumeFallbackCleared(
		resumed,
		verification.FallbackReceipt.WakeCount,
		verification.FallbackReceipt.ClearedAt,
	); err != nil {
		t.Fatalf("RecordResumeFallbackCleared: %v", err)
	}
	if err := store.withExclusiveLock(func() error {
		return store.writeLiveMCPReceiptUnlocked(
			resumed,
			verification.LiveMCPReceipt,
		)
	}); err != nil {
		t.Fatalf("writeLiveMCPReceiptUnlocked: %v", err)
	}
	verifiedChange, err := MarkVerified(resumed, verification)
	if err != nil {
		t.Fatalf("MarkVerified: %v", err)
	}
	if err := store.Apply(verifiedChange); err != nil {
		t.Fatalf("apply verified: %v", err)
	}

	snapshot, err := LoadVerifiedRuntimeSnapshot(root)
	if err != nil {
		t.Fatalf("LoadVerifiedRuntimeSnapshot: %v", err)
	}
	if snapshot.CheckpointState != StateVerified.String() ||
		snapshot.CheckpointAttempt != 1 ||
		snapshot.RestartID != prepared.restartID ||
		snapshot.ThreadID != prepared.threadID ||
		snapshot.ResumeIntentDigest != prepared.resumeIntentDigest ||
		snapshot.InstalledExecutableDigest != prepared.desiredHaftBinaryDigest ||
		snapshot.PreparedTaskRuntimePID != prepared.taskRuntime.PID ||
		!snapshot.PreparedTaskRuntimeStartedAt.Equal(
			prepared.taskRuntime.StartedAt,
		) ||
		snapshot.PreparedTaskRuntimeExecutable !=
			prepared.taskRuntime.ExecutablePath ||
		snapshot.PreparedTaskRuntimeArgsDigest !=
			prepared.taskRuntime.ArgumentsDigest ||
		snapshot.LiveMCPReceiptDigest == "" ||
		snapshot.FallbackReceiptDigest == "" ||
		snapshot.RestartCheckpointDigest == "" ||
		snapshot.ExactTaskResumeCount != 1 ||
		!snapshot.SingleWriterObserved ||
		!snapshot.LaunchdRemovalObserved ||
		!snapshot.PrivateStateGitignoredObserved ||
		!snapshot.CandidateDigestReserved ||
		!snapshot.TemporaryStagesAbsent {
		t.Fatalf("safe verified runtime snapshot is incomplete: %#v", snapshot)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal safe snapshot: %v", err)
	}
	if bytes.Contains(raw, []byte("nonce")) ||
		bytes.Contains(raw, []byte(prepared.resumeFallbackNonce)) ||
		bytes.Contains(raw, []byte(prepared.liveMCPChallengeNonce)) {
		t.Fatal("safe verified runtime snapshot exposed a private nonce")
	}

	stagePath := filepath.Join(
		store.directoryPath,
		stagePrefix+"left-behind",
	)
	if err := os.WriteFile(stagePath, []byte("stage"), 0o600); err != nil {
		t.Fatalf("write leftover stage: %v", err)
	}
	if _, err := LoadVerifiedRuntimeSnapshot(root); err == nil {
		t.Fatal("verified runtime snapshot accepted a leftover stage")
	}
	if err := os.Remove(stagePath); err != nil {
		t.Fatalf("remove leftover stage: %v", err)
	}
	markerPath := attemptMarkerPath(
		t,
		store,
		prepared.desiredHaftBinaryDigest,
	)
	if err := os.WriteFile(markerPath, []byte("wrong\n"), 0o600); err != nil {
		t.Fatalf("corrupt attempt marker: %v", err)
	}
	if _, err := LoadVerifiedRuntimeSnapshot(root); err == nil {
		t.Fatal("verified runtime snapshot accepted a corrupt attempt marker")
	}
}
