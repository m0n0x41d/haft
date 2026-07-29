package agenthostrestart

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type checkpointCommandEvidence struct {
	preparation PreparationEvidence
	runtime     RuntimeVerification
}

func (evidence checkpointCommandEvidence) CapturePreparation(
	context.Context,
	PreparationRequest,
) (PreparationEvidence, error) {
	return evidence.preparation, nil
}

func (evidence checkpointCommandEvidence) CaptureRuntime(
	context.Context,
	VerificationRequest,
	io.Writer,
) (RuntimeVerification, error) {
	return evidence.runtime, nil
}

func TestCheckpointCommandPreparesResumesAndVerifiesWithoutCallerDigests(t *testing.T) {
	root := restartCommandProject(t)
	candidate := filepath.Join(root, "candidate-haft")
	if err := os.WriteFile(candidate, []byte("candidate"), 0o700); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	installed := filepath.Join(root, "installed", "haft")
	createdAt := time.Date(2026, 7, 19, 8, 9, 10, 0, time.UTC)
	preparation := PreparationEvidence{
		RepositoryHead:              strings.Repeat("a", 40),
		DirtyStateDigest:            digestOf('b'),
		DesiredHaftBinaryDigest:     digestOf('c'),
		ExpectedFPFRevision:         strings.Repeat("d", 40),
		ExpectedTypeEnvDigest:       digestOf('e'),
		ExpectedTypeEnvHeadRevision: 7,
		ExpectedGraphRevision:       11,
		ExpectedSkillCarriersDigest: digestOf('f'),
		ExpectedInstructionDigest:   digestOf('1'),
		ExpectedMCPConfigDigest:     digestOf('2'),
		TaskRuntime: TaskRuntimeIdentity{
			PID:             4242,
			StartedAt:       createdAt.Add(-time.Hour),
			ExecutablePath:  "/Applications/ChatGPT.app/Contents/Resources/codex",
			ArgumentsDigest: digestOf('4'),
		},
	}
	command := checkpointCommand{
		evidence: checkpointCommandEvidence{preparation: preparation},
		now:      func() time.Time { return createdAt },
	}
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	prepareArgs := []string{
		"prepare",
		"--project-root", root,
		"--thread-id", "019f5a6e-fba1-7cd3-8421-677d5431bd12",
		"--resume-intent", "Continue P14 in this exact task",
		"--plan-path", ".context/plan.md",
		"--last-completed", "P13",
		"--resume-at", "P14",
		"--method-run-absence", "installed acceptance has no MethodRun",
		"--candidate-haft", candidate,
		"--installed-haft", installed,
		"--skill-root", filepath.Join(root, ".agents", "skills"),
		"--instruction-carrier", filepath.Join(root, "AGENTS.md"),
		"--mcp-config", filepath.Join(root, ".codex", "config.toml"),
	}
	if code := command.run(context.Background(), prepareArgs, stdout, stderr); code != 0 {
		t.Fatalf("prepare code = %d, stderr = %s", code, stderr.String())
	}
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	checkpoint, err := store.Load()
	if err != nil {
		t.Fatalf("Load prepared: %v", err)
	}
	if checkpoint.State() != StatePrepared {
		t.Fatalf("prepared state = %s", checkpoint.State().String())
	}
	if !strings.Contains(stdout.String(), "resume: P14") {
		t.Fatalf("prepare output is not human-readable: %s", stdout.String())
	}

	submission, err := MarkSubmitted(checkpoint)
	if err != nil {
		t.Fatalf("MarkSubmitted: %v", err)
	}
	if err := store.Apply(submission); err != nil {
		t.Fatalf("apply submitted: %v", err)
	}
	if err := store.InstallLiveMCPChallenge(submission.After()); err != nil {
		t.Fatalf("InstallLiveMCPChallenge: %v", err)
	}
	permit, err := AuthorizeQuit(submission.After(), InstallObservation{
		TaskInstallSucceeded: true,
		HaftInitSucceeded:    true,
		InstalledHaftPath:    installed,
		InstalledHaftDigest:  preparation.DesiredHaftBinaryDigest,
		ProjectBasis:         validProjectBasis(submission.After()),
		Carriers:             validCarrierObservation(submission.After()),
	})
	if err != nil {
		t.Fatalf("AuthorizeQuit: %v", err)
	}
	opening, err := MarkAppOpened(submission.After(), permit, AppOpenObservation{
		GracefulQuitSucceeded: true,
		OldApplicationAbsent:  true,
		OldTaskRuntimeAbsent:  true,
		NewApplicationStarted: true,
		DeepLinkOpened:        "codex://threads/019f5a6e-fba1-7cd3-8421-677d5431bd12",
		ApplicationStartedAt:  createdAt.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("MarkAppOpened: %v", err)
	}
	if err := store.Apply(opening); err != nil {
		t.Fatalf("apply opened: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	resumeArgs := []string{
		"resume",
		"--project-root", root,
		"--thread-id", "019f5a6e-fba1-7cd3-8421-677d5431bd12",
		"--resume-intent", "Continue P14 in this exact task",
	}
	if code := command.run(context.Background(), resumeArgs, stdout, stderr); code != 0 {
		t.Fatalf("resume code = %d, stderr = %s", code, stderr.String())
	}

	resumed, err := store.Load()
	if err != nil {
		t.Fatalf("Load resumed: %v", err)
	}
	verification := validRuntimeVerification(resumed)
	if _, err := store.RecordResumeFallbackCleared(
		resumed,
		verification.FallbackReceipt.WakeCount,
		verification.FallbackReceipt.ClearedAt,
	); err != nil {
		t.Fatalf("RecordResumeFallbackCleared: %v", err)
	}
	if err := store.withExclusiveLock(func() error {
		return store.writeLiveMCPReceiptUnlocked(resumed, verification.LiveMCPReceipt)
	}); err != nil {
		t.Fatalf("writeLiveMCPReceiptUnlocked: %v", err)
	}
	command.evidence = checkpointCommandEvidence{runtime: verification}
	launcher := &fakeOneShotLauncher{exists: true}
	command.launcher = launcher
	stdout.Reset()
	stderr.Reset()
	verifyArgs := []string{
		"verify",
		"--project-root", root,
		"--contract-arg", "interface",
		"--contract-arg", "memory.validate",
		"--contract-arg=--json",
	}
	if code := command.run(context.Background(), verifyArgs, stdout, stderr); code != 0 {
		t.Fatalf("verify code = %d, stderr = %s", code, stderr.String())
	}
	verified, err := store.Load()
	if err != nil {
		t.Fatalf("Load verified: %v", err)
	}
	if verified.State() != StateVerified {
		t.Fatalf("verified state = %s", verified.State().String())
	}
	if !reflect.DeepEqual(
		launcher.removedLabels,
		[]string{resumed.launchdLabel},
	) {
		t.Fatalf("removed launchd labels = %v", launcher.removedLabels)
	}
}

func TestCheckpointCommandRejectsAmbiguousMethodRunAndMutatingContract(t *testing.T) {
	root := restartCommandProject(t)
	candidate := filepath.Join(root, "candidate-haft")
	if err := os.WriteFile(candidate, []byte("candidate"), 0o700); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	command := checkpointCommand{
		evidence: checkpointCommandEvidence{},
		now:      time.Now,
	}
	stderr := bytes.NewBuffer(nil)
	prepareArgs := []string{
		"prepare",
		"--project-root", root,
		"--thread-id", "019f5a6e-fba1-7cd3-8421-677d5431bd12",
		"--resume-intent", "Continue P14",
		"--plan-path", ".context/plan.md",
		"--last-completed", "P13",
		"--resume-at", "P14",
		"--candidate-haft", candidate,
		"--skill-root", filepath.Join(root, ".agents", "skills"),
		"--instruction-carrier", filepath.Join(root, "AGENTS.md"),
		"--mcp-config", filepath.Join(root, ".codex", "config.toml"),
	}
	if code := command.run(context.Background(), prepareArgs, io.Discard, stderr); code != 2 {
		t.Fatalf("ambiguous MethodRun code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "exactly one") {
		t.Fatalf("ambiguous MethodRun error = %s", stderr.String())
	}

	request := VerificationRequest{
		ProjectRoot:            root,
		ContractSmokeArguments: []string{"interface", "memory.validate", "--json"},
	}
	request.ContractSmokeArguments = []string{"memory", "admit", "--input-file", "request.json"}
	if _, err := normalizeVerificationRequest(request); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("mutating contract smoke error = %v", err)
	}
}

func TestDigestGitDirtyStateTracksUntrackedFileBytes(t *testing.T) {
	git, err := execLookPathForTest("git")
	if err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	root := t.TempDir()
	runGitForTest(t, git, root, "init", "-q")
	runGitForTest(t, git, root, "config", "user.email", "test@example.invalid")
	runGitForTest(t, git, root, "config", "user.name", "Restart Test")
	tracked := filepath.Join(root, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("tracked\n"), 0o644); err != nil {
		t.Fatalf("write tracked: %v", err)
	}
	runGitForTest(t, git, root, "add", "tracked.txt")
	runGitForTest(t, git, root, "commit", "-qm", "base")
	untracked := filepath.Join(root, "untracked.txt")
	if err := os.WriteFile(untracked, []byte("first\n"), 0o644); err != nil {
		t.Fatalf("write untracked first: %v", err)
	}
	first, err := digestGitDirtyState(context.Background(), root)
	if err != nil {
		t.Fatalf("digest first: %v", err)
	}
	if err := os.WriteFile(untracked, []byte("second\n"), 0o644); err != nil {
		t.Fatalf("write untracked second: %v", err)
	}
	second, err := digestGitDirtyState(context.Background(), root)
	if err != nil {
		t.Fatalf("digest second: %v", err)
	}
	if first == second {
		t.Fatal("dirty state digest ignored untracked byte change")
	}
}

func restartCommandProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".haft"), 0o700); err != nil {
		t.Fatalf("mkdir .haft: %v", err)
	}
	paths := map[string][]byte{
		filepath.Join(root, "AGENTS.md"):                                 []byte("instructions\n"),
		filepath.Join(root, ".codex", "config.toml"):                     []byte("[mcp_servers.haft]\n"),
		filepath.Join(root, ".agents", "skills", "h-reason", "SKILL.md"): []byte("skill\n"),
	}
	for path, content := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("mkdir fixture carrier: %v", err)
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("write fixture carrier: %v", err)
		}
	}
	physical, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("resolve temp project root: %v", err)
	}
	return physical
}

func execLookPathForTest(name string) (string, error) {
	return exec.LookPath(name)
}

func runGitForTest(t *testing.T, git string, root string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"-C", root}, args...)
	command := exec.Command(git, commandArgs...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}
