//go:build darwin || linux

package agenthostrestart

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPreparedCheckpointCanonicalRoundTripAndClosedState(t *testing.T) {
	root := newRestartProject(t)
	checkpoint := newPreparedFixture(t, root, "restart-one", digestOf('a'))

	if checkpoint.State() != StatePrepared || checkpoint.Attempt() != 1 {
		t.Fatalf("checkpoint = state %s attempt %d", checkpoint.State().String(), checkpoint.Attempt())
	}
	content, err := checkpoint.CanonicalBytes()
	if err != nil {
		t.Fatalf("CanonicalBytes: %v", err)
	}
	decoded, err := DecodeCheckpoint(content)
	if err != nil {
		t.Fatalf("DecodeCheckpoint: %v", err)
	}
	decodedContent, err := decoded.CanonicalBytes()
	if err != nil {
		t.Fatalf("decoded CanonicalBytes: %v", err)
	}
	if !bytes.Equal(decodedContent, content) {
		t.Fatal("canonical round trip changed bytes")
	}

	unknown := bytes.Replace(content, []byte(`"restart_id"`), []byte(`"unknown":true,"restart_id"`), 1)
	if _, err := DecodeCheckpoint(unknown); err == nil {
		t.Fatal("DecodeCheckpoint accepted an unknown field")
	}
	predecessor := bytes.Replace(
		content,
		[]byte(`"resume_intent_digest"`),
		[]byte(`"goal_objective_digest"`),
		1,
	)
	if _, err := DecodeCheckpoint(predecessor); !errors.Is(err, ErrPredecessorCheckpoint) {
		t.Fatalf("DecodeCheckpoint predecessor error = %v", err)
	}
	predecessorWithCount := bytes.Replace(
		content,
		[]byte(`"attempt":1`),
		[]byte(`"goal_resume_count":1,"attempt":1`),
		1,
	)
	if _, err := DecodeCheckpoint(predecessorWithCount); !errors.Is(err, ErrPredecessorCheckpoint) {
		t.Fatalf("DecodeCheckpoint predecessor count error = %v", err)
	}
	predecessorWithZeroCount := bytes.Replace(
		content,
		[]byte(`"attempt":1`),
		[]byte(`"goal_resume_count":0,"attempt":1`),
		1,
	)
	if _, err := DecodeCheckpoint(predecessorWithZeroCount); !errors.Is(err, ErrPredecessorCheckpoint) {
		t.Fatalf("DecodeCheckpoint predecessor zero count error = %v", err)
	}
	predecessorWithEmptyDigest := bytes.Replace(
		content,
		[]byte(`"attempt":1`),
		[]byte(`"goal_objective_digest":"","attempt":1`),
		1,
	)
	if _, err := DecodeCheckpoint(predecessorWithEmptyDigest); !errors.Is(err, ErrPredecessorCheckpoint) {
		t.Fatalf("DecodeCheckpoint predecessor empty digest error = %v", err)
	}
	invalidAttempt := bytes.Replace(content, []byte(`"attempt":1`), []byte(`"attempt":2`), 1)
	if _, err := DecodeCheckpoint(invalidAttempt); err == nil {
		t.Fatal("DecodeCheckpoint accepted attempt=2")
	}
}

func TestSupervisorStateMachineFailsClosedThenVerifiesExactRuntime(t *testing.T) {
	root := newRestartProject(t)
	prepared := newPreparedFixture(t, root, "restart-state", digestOf('b'))
	submission, err := MarkSubmitted(prepared)
	if err != nil {
		t.Fatalf("MarkSubmitted: %v", err)
	}
	submitted := submission.After()

	wrongInstall := validInstallObservation(submitted)
	wrongInstall.InstalledHaftDigest = digestOf('c')
	if _, err := AuthorizeQuit(submitted, wrongInstall); !errors.Is(err, ErrPreQuitDenied) {
		t.Fatalf("AuthorizeQuit wrong digest error = %v", err)
	}
	permit, err := AuthorizeQuit(submitted, validInstallObservation(submitted))
	if err != nil {
		t.Fatalf("AuthorizeQuit: %v", err)
	}

	openObservation := AppOpenObservation{
		GracefulQuitSucceeded: true,
		OldApplicationAbsent:  true,
		OldTaskRuntimeAbsent:  true,
		NewApplicationStarted: true,
		DeepLinkOpened:        "codex://threads/" + submitted.threadID,
		ApplicationStartedAt:  submitted.createdAt.Add(time.Minute),
	}
	missingRuntimeStop := openObservation
	missingRuntimeStop.OldTaskRuntimeAbsent = false
	if _, err := MarkAppOpened(
		submitted,
		permit,
		missingRuntimeStop,
	); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("MarkAppOpened missing task runtime stop error = %v", err)
	}
	opening, err := MarkAppOpened(submitted, permit, openObservation)
	if err != nil {
		t.Fatalf("MarkAppOpened: %v", err)
	}
	opened := opening.After()
	if _, err := MarkResumed(opened, ResumeObservation{
		ThreadID:           "00000000-0000-0000-0000-000000000000",
		ResumeIntentDigest: opened.resumeIntentDigest,
		RepositoryRoot:     root,
	}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("MarkResumed wrong thread error = %v", err)
	}
	if _, err := MarkResumed(opened, ResumeObservation{
		ThreadID:           opened.threadID,
		ResumeIntentDigest: digestOf('0'),
		RepositoryRoot:     root,
	}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("MarkResumed wrong resume intent error = %v", err)
	}
	if _, err := MarkResumed(opened, ResumeObservation{
		ThreadID:           opened.threadID,
		ResumeIntentDigest: opened.resumeIntentDigest,
		RepositoryRoot:     filepath.Join(root, "another-project"),
	}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("MarkResumed wrong repository error = %v", err)
	}
	resumption, err := MarkResumed(opened, ResumeObservation{
		ThreadID:           opened.threadID,
		ResumeIntentDigest: opened.resumeIntentDigest,
		RepositoryRoot:     root,
	})
	if err != nil {
		t.Fatalf("MarkResumed: %v", err)
	}
	resumed := resumption.After()

	verification := validRuntimeVerification(resumed)
	verification.supervisorRemoval = supervisorRemovalObservation{}
	if _, err := MarkVerified(resumed, verification); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("MarkVerified unsafe proof error = %v", err)
	}
	verification.supervisorRemoval = observedSupervisorRemoval(resumed.launchdLabel + ".other")
	if _, err := MarkVerified(resumed, verification); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("MarkVerified mismatched supervisor proof error = %v", err)
	}
	verification.supervisorRemoval = observedSupervisorRemoval(resumed.launchdLabel)
	verificationChange, err := MarkVerified(resumed, verification)
	if err != nil {
		t.Fatalf("MarkVerified: %v", err)
	}
	verified := verificationChange.After()
	if verified.State() != StateVerified {
		t.Fatalf("verified state = %s", verified.State().String())
	}
	if _, err := MarkSubmitted(verified); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("verified checkpoint restarted: %v", err)
	}
}

func TestRuntimeVerificationRejectsEveryProcessAndCarrierIdentityMismatch(t *testing.T) {
	root := newRestartProject(t)
	prepared := newPreparedFixture(t, root, "restart-runtime-mismatch", digestOf('b'))
	submission, err := MarkSubmitted(prepared)
	if err != nil {
		t.Fatalf("MarkSubmitted: %v", err)
	}
	permit, err := AuthorizeQuit(submission.After(), validInstallObservation(submission.After()))
	if err != nil {
		t.Fatalf("AuthorizeQuit: %v", err)
	}
	opening, err := MarkAppOpened(submission.After(), permit, AppOpenObservation{
		GracefulQuitSucceeded: true,
		OldApplicationAbsent:  true,
		OldTaskRuntimeAbsent:  true,
		NewApplicationStarted: true,
		DeepLinkOpened:        "codex://threads/" + prepared.threadID,
		ApplicationStartedAt:  prepared.createdAt.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("MarkAppOpened: %v", err)
	}
	resumption, err := MarkResumed(opening.After(), ResumeObservation{
		ThreadID:           prepared.threadID,
		ResumeIntentDigest: prepared.resumeIntentDigest,
		RepositoryRoot:     root,
	})
	if err != nil {
		t.Fatalf("MarkResumed: %v", err)
	}
	resumed := resumption.After()
	tests := []struct {
		name   string
		mutate func(*RuntimeVerification)
		want   string
	}{
		{name: "CLI path", mutate: func(value *RuntimeVerification) { value.CLIExecutablePath = filepath.Join(root, "other-haft") }, want: "CLI executable path differs"},
		{name: "CLI digest", mutate: func(value *RuntimeVerification) { value.CLIExecutableDigest = digestOf('0') }, want: "CLI executable digest differs"},
		{name: "MCP path", mutate: func(value *RuntimeVerification) {
			value.LiveMCPReceipt.ExecutablePath = filepath.Join(root, "old-haft")
		}, want: "MCP executable path differs"},
		{name: "MCP digest", mutate: func(value *RuntimeVerification) { value.LiveMCPReceipt.ExecutableDigest = digestOf('0') }, want: "MCP executable digest differs"},
		{name: "MCP start", mutate: func(value *RuntimeVerification) {
			value.LiveMCPReceipt.ProcessStartedAt = resumed.createdAt.Add(-time.Second)
		}, want: "MCP process predates checkpoint"},
		{name: "skills", mutate: func(value *RuntimeVerification) { value.Carriers.SkillDigest = digestOf('0') }, want: "skill carrier digest differs"},
		{name: "instructions", mutate: func(value *RuntimeVerification) { value.Carriers.InstructionDigest = digestOf('0') }, want: "instruction carrier digest differs"},
		{name: "MCP config", mutate: func(value *RuntimeVerification) { value.Carriers.MCPConfigDigest = digestOf('0') }, want: "MCP config digest differs"},
		{name: "repository HEAD", mutate: func(value *RuntimeVerification) { value.ProjectBasis.RepositoryHead = gitRevisionOf('0') }, want: "repository HEAD differs"},
		{name: "dirty state", mutate: func(value *RuntimeVerification) { value.ProjectBasis.DirtyStateDigest = digestOf('0') }, want: "dirty-state digest differs"},
		{name: "FPF revision", mutate: func(value *RuntimeVerification) { value.ProjectBasis.FPFRevision = gitRevisionOf('0') }, want: "FPF revision differs"},
		{name: "TypeEnv", mutate: func(value *RuntimeVerification) { value.ProjectBasis.TypeEnvDigest = digestOf('0') }, want: "selected TypeEnv digest differs"},
		{name: "TypeEnv head", mutate: func(value *RuntimeVerification) { value.ProjectBasis.TypeEnvHeadRevision++ }, want: "selected TypeEnv head revision differs"},
		{name: "graph revision", mutate: func(value *RuntimeVerification) { value.ProjectBasis.GraphRevision++ }, want: "project graph revision differs"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verification := validRuntimeVerification(resumed)
			test.mutate(&verification)
			_, err := MarkVerified(resumed, verification)
			if !errors.Is(err, ErrInvalidTransition) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("MarkVerified error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestInstallFailureIsTerminalForStateMachine(t *testing.T) {
	root := newRestartProject(t)
	prepared := newPreparedFixture(t, root, "restart-failed", digestOf('d'))
	submission, err := MarkSubmitted(prepared)
	if err != nil {
		t.Fatalf("MarkSubmitted: %v", err)
	}
	submitted := submission.After()
	failure, err := MarkInstallFailed(submitted, "task install exited before app quit")
	if err != nil {
		t.Fatalf("MarkInstallFailed: %v", err)
	}
	failed := failure.After()
	if failed.State() != StateInstallFailed {
		t.Fatalf("failed state = %s", failed.State().String())
	}
	if _, err := AuthorizeQuit(failed, validInstallObservation(failed)); !errors.Is(err, ErrPreQuitDenied) {
		t.Fatalf("failed checkpoint authorized quit: %v", err)
	}
}

func TestStoreWritesPrivateGitignoredCheckpointAndExactCAS(t *testing.T) {
	root := newRestartProject(t)
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	prepared := newPreparedFixture(t, root, "restart-store", digestOf('e'))
	if err := store.Prepare(prepared); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	info, err := osStat(store.CheckpointPath())
	if err != nil {
		t.Fatalf("stat checkpoint: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("checkpoint mode = %o", info.Mode().Perm())
	}
	ignoreBytes, err := readRegularNoFollow(filepath.Join(root, ".haft", "restart", ".gitignore"), MaximumCheckpointBytes)
	if err != nil {
		t.Fatalf("read restart .gitignore: %v", err)
	}
	if !ignoreProtectsRestartFiles(ignoreBytes) {
		t.Fatalf("restart .gitignore does not protect checkpoint/log: %q", ignoreBytes)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".haft", "restart"))
	if err != nil {
		t.Fatalf("read restart directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), stagePrefix) {
			t.Fatalf("atomic checkpoint stage was not cleaned up: %s", entry.Name())
		}
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.RestartID() != prepared.RestartID() {
		t.Fatalf("loaded restart = %q", loaded.RestartID())
	}
	submission, err := MarkSubmitted(prepared)
	if err != nil {
		t.Fatalf("MarkSubmitted: %v", err)
	}
	submitted := submission.After()
	if err := store.Apply(submission); err != nil {
		t.Fatalf("Apply submitted: %v", err)
	}
	if err := store.Apply(submission); err != nil {
		t.Fatalf("idempotent Apply: %v", err)
	}
	failure, err := MarkInstallFailed(submitted, "install failed safely")
	if err != nil {
		t.Fatalf("MarkInstallFailed: %v", err)
	}
	if err := store.Apply(Change{before: prepared, after: failure.After()}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("manufactured skipped transition error = %v", err)
	}
}

func TestStoreLoadMissingIsReadOnly(t *testing.T) {
	root := newRestartProject(t)
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrCheckpointNotFound) {
		t.Fatalf("Load missing error = %v", err)
	}
	if _, err := osStat(filepath.Join(root, ".haft", "restart")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read-only Load created restart directory: %v", err)
	}
}

func TestStoreConcurrentDistinctTransitionsHaveOneWriter(t *testing.T) {
	root := newRestartProject(t)
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	prepared := newPreparedFixture(t, root, "restart-cas", digestOf('c'))
	if err := store.Prepare(prepared); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	submission, err := MarkSubmitted(prepared)
	if err != nil {
		t.Fatalf("MarkSubmitted: %v", err)
	}
	if err := store.Apply(submission); err != nil {
		t.Fatalf("Apply submission: %v", err)
	}
	submitted := submission.After()
	failure, err := MarkInstallFailed(submitted, "bounded test failure")
	if err != nil {
		t.Fatalf("MarkInstallFailed: %v", err)
	}
	permit, err := AuthorizeQuit(submitted, validInstallObservation(submitted))
	if err != nil {
		t.Fatalf("AuthorizeQuit: %v", err)
	}
	opening, err := MarkAppOpened(submitted, permit, AppOpenObservation{
		GracefulQuitSucceeded: true,
		OldApplicationAbsent:  true,
		OldTaskRuntimeAbsent:  true,
		NewApplicationStarted: true,
		DeepLinkOpened:        "codex://threads/" + submitted.threadID,
		ApplicationStartedAt:  submitted.createdAt.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("MarkAppOpened: %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for _, change := range []Change{failure, opening} {
		wait.Add(1)
		go func(candidate Change) {
			defer wait.Done()
			<-start
			results <- store.Apply(candidate)
		}(change)
	}
	close(start)
	wait.Wait()
	close(results)
	successes := 0
	conflicts := 0
	for result := range results {
		if result == nil {
			successes++
			continue
		}
		if errors.Is(result, ErrConcurrentUpdate) {
			conflicts++
			continue
		}
		t.Fatalf("unexpected concurrent Apply error: %v", result)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent Apply = successes %d conflicts %d", successes, conflicts)
	}
}

func TestStoreLoopGuardRequiresChangedRepairBasisAfterFailure(t *testing.T) {
	root := newRestartProject(t)
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	first := newPreparedFixture(t, root, "restart-loop-one", digestOf('f'))
	if err := store.Prepare(first); err != nil {
		t.Fatalf("Prepare first: %v", err)
	}
	duplicate := newPreparedFixture(t, root, "restart-loop-two", digestOf('f'))
	if err := store.Prepare(duplicate); !errors.Is(err, ErrLoopGuard) {
		t.Fatalf("active same-digest preparation error = %v", err)
	}
	submission, err := MarkSubmitted(first)
	if err != nil {
		t.Fatalf("MarkSubmitted: %v", err)
	}
	submitted := submission.After()
	if err := store.Apply(submission); err != nil {
		t.Fatalf("Apply submitted: %v", err)
	}
	failure, err := MarkInstallFailed(submitted, "pre-quit install failure")
	if err != nil {
		t.Fatalf("MarkInstallFailed: %v", err)
	}
	if err := store.Apply(failure); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if err := store.Prepare(duplicate); !errors.Is(err, ErrLoopGuard) {
		t.Fatalf("failed same-digest preparation error = %v", err)
	}
	changed := newPreparedFixture(t, root, "restart-loop-three", digestOf('1'))
	if err := store.Prepare(changed); err != nil {
		t.Fatalf("changed repair basis Prepare: %v", err)
	}
}

func TestStorePersistsOneFullHandoffAndRejectsVerifiedDigestReplay(t *testing.T) {
	root := newRestartProject(t)
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	prepared := newPreparedFixture(t, root, "restart-full", digestOf('b'))
	if err := store.Prepare(prepared); err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	submission, err := MarkSubmitted(prepared)
	if err != nil {
		t.Fatalf("MarkSubmitted: %v", err)
	}
	if err := store.Apply(submission); err != nil {
		t.Fatalf("Apply submission: %v", err)
	}
	submitted := submission.After()
	permit, err := AuthorizeQuit(submitted, validInstallObservation(submitted))
	if err != nil {
		t.Fatalf("AuthorizeQuit: %v", err)
	}
	opening, err := MarkAppOpened(submitted, permit, AppOpenObservation{
		GracefulQuitSucceeded: true,
		OldApplicationAbsent:  true,
		OldTaskRuntimeAbsent:  true,
		NewApplicationStarted: true,
		DeepLinkOpened:        "codex://threads/" + submitted.threadID,
		ApplicationStartedAt:  submitted.createdAt.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("MarkAppOpened: %v", err)
	}
	if err := store.Apply(opening); err != nil {
		t.Fatalf("Apply opening: %v", err)
	}
	opened := opening.After()
	resumption, err := MarkResumed(opened, ResumeObservation{
		ThreadID:           opened.threadID,
		ResumeIntentDigest: opened.resumeIntentDigest,
		RepositoryRoot:     root,
	})
	if err != nil {
		t.Fatalf("MarkResumed: %v", err)
	}
	if err := store.Apply(resumption); err != nil {
		t.Fatalf("Apply resumption: %v", err)
	}
	resumed := resumption.After()
	verification, err := MarkVerified(resumed, validRuntimeVerification(resumed))
	if err != nil {
		t.Fatalf("MarkVerified: %v", err)
	}
	if err := store.Apply(verification); err != nil {
		t.Fatalf("Apply verification: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load verified: %v", err)
	}
	if loaded.State() != StateVerified {
		t.Fatalf("loaded state = %s", loaded.State().String())
	}

	replay := newPreparedFixture(t, root, "restart-full-replay", prepared.DesiredHaftBinaryDigest())
	if err := store.Prepare(replay); !errors.Is(err, ErrAlreadyVerified) {
		t.Fatalf("verified digest replay error = %v", err)
	}
}

func TestStoreRejectsSymlinkedRestartDirectory(t *testing.T) {
	root := newRestartProject(t)
	outside := t.TempDir()
	restartPath := filepath.Join(root, ".haft", "restart")
	if err := createSymlink(outside, restartPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	checkpoint := newPreparedFixture(t, root, "restart-symlink", digestOf('2'))
	if err := store.Prepare(checkpoint); err == nil || !strings.Contains(err.Error(), "non-symlink directory") {
		t.Fatalf("Prepare symlink error = %v", err)
	}
}

func validInstallObservation(checkpoint Checkpoint) InstallObservation {
	return InstallObservation{
		TaskInstallSucceeded: true,
		HaftInitSucceeded:    true,
		InstalledHaftPath:    checkpoint.expectedHaftBinaryPath,
		InstalledHaftDigest:  checkpoint.desiredHaftBinaryDigest,
		ProjectBasis:         validProjectBasis(checkpoint),
		Carriers:             validCarrierObservation(checkpoint),
	}
}

func validRuntimeVerification(checkpoint Checkpoint) RuntimeVerification {
	basisDigest, err := checkpoint.BasisDigest()
	if err != nil {
		panic(err)
	}
	return RuntimeVerification{
		CLIExecutablePath:   checkpoint.expectedHaftBinaryPath,
		CLIExecutableDigest: checkpoint.desiredHaftBinaryDigest,
		ProjectBasis:        validProjectBasis(checkpoint),
		Carriers:            validCarrierObservation(checkpoint),
		LiveMCPReceipt: LiveMCPReceipt{
			RestartID:             checkpoint.restartID,
			CheckpointBasisDigest: basisDigest,
			Nonce:                 checkpoint.liveMCPChallengeNonce,
			PID:                   42,
			ExecutablePath:        checkpoint.expectedHaftBinaryPath,
			ExecutableDigest:      checkpoint.desiredHaftBinaryDigest,
			ProjectRoot:           checkpoint.repositoryRoot,
			ProcessStartedAt:      checkpoint.createdAt.Add(2 * time.Minute),
			FulfilledAt:           checkpoint.createdAt.Add(3 * time.Minute),
		},
		FallbackReceipt: ResumeFallbackReceipt{
			RestartID:             checkpoint.restartID,
			CheckpointBasisDigest: basisDigest,
			Nonce:                 checkpoint.resumeFallbackNonce,
			WakeCount:             1,
			ClearedAt:             checkpoint.createdAt.Add(2 * time.Minute),
		},
		ChangedContractSmokePassed: true,
		supervisorRemoval:          observedSupervisorRemoval(checkpoint.launchdLabel),
	}
}

func validProjectBasis(checkpoint Checkpoint) ProjectBasisObservation {
	return ProjectBasisObservation{
		RepositoryHead:      checkpoint.repositoryHead,
		DirtyStateDigest:    checkpoint.dirtyStateDigest,
		FPFRevision:         checkpoint.expectedFPFRevision,
		TypeEnvDigest:       checkpoint.expectedTypeEnvDigest,
		TypeEnvHeadRevision: checkpoint.expectedTypeEnvHeadRevision,
		GraphRevision:       checkpoint.expectedGraphRevision,
	}
}

func validCarrierObservation(checkpoint Checkpoint) CarrierObservation {
	return CarrierObservation{
		SkillCarriersRoot: checkpoint.expectedSkillCarriersRoot,
		SkillDigest:       checkpoint.expectedSkillCarriersDigest,
		InstructionPath:   checkpoint.expectedInstructionPath,
		InstructionDigest: checkpoint.expectedInstructionDigest,
		MCPConfigPath:     checkpoint.expectedMCPConfigPath,
		MCPConfigDigest:   checkpoint.expectedMCPConfigDigest,
	}
}
