//go:build darwin || linux

package agenthostrestart

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

type fakeOneShotLauncher struct {
	exists        bool
	existsError   error
	submitError   error
	removeError   error
	existsLabels  []string
	removedLabels []string
	jobs          []OneShotJob
}

func (launcher *fakeOneShotLauncher) Exists(
	_ context.Context,
	label string,
) (bool, error) {
	launcher.existsLabels = append(launcher.existsLabels, label)
	return launcher.exists, launcher.existsError
}

func (launcher *fakeOneShotLauncher) Submit(
	_ context.Context,
	job OneShotJob,
) error {
	launcher.jobs = append(launcher.jobs, job)
	return launcher.submitError
}

func (launcher *fakeOneShotLauncher) Remove(
	_ context.Context,
	label string,
) error {
	launcher.removedLabels = append(launcher.removedLabels, label)
	launcher.exists = false
	return launcher.removeError
}

func TestSubmitPreparedRestartBuildsExactArgvWithoutShellText(t *testing.T) {
	root := newRestartProject(t)
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	checkpoint := newPreparedFixture(t, root, "restart-submit", digestOf('a'))
	if err := store.Prepare(checkpoint); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	supervisor := executableFixture(t, root, "supervisor $(not-evaluated)")
	task := executableFixture(t, root, "task;still-one-argv")
	supervisor, err = canonicalExistingFile(supervisor)
	if err != nil {
		t.Fatalf("canonical supervisor: %v", err)
	}
	task, err = canonicalExistingFile(task)
	if err != nil {
		t.Fatalf("canonical task: %v", err)
	}
	launcher := &fakeOneShotLauncher{}
	job, err := SubmitPreparedRestart(
		context.Background(),
		store,
		launcher,
		SubmissionRequest{
			SupervisorExecutable: supervisor,
			TaskExecutable:       task,
			QuitTimeout:          75 * time.Second,
		},
	)
	if err != nil {
		t.Fatalf("SubmitPreparedRestart: %v", err)
	}
	wantArguments := []string{
		"--project-root",
		root,
		"--task-executable",
		task,
		"--quit-timeout",
		"1m15s",
	}
	if job.Label() != checkpoint.launchdLabel || job.Executable() != supervisor {
		t.Fatalf("job identity = %q %q", job.Label(), job.Executable())
	}
	if !reflect.DeepEqual(job.Arguments(), wantArguments) {
		t.Fatalf("job arguments = %#v, want %#v", job.Arguments(), wantArguments)
	}
	if slices.Contains(job.Arguments(), "--allow-term") {
		t.Fatalf("default job unexpectedly allows SIGTERM: %#v", job.Arguments())
	}
	if len(launcher.jobs) != 1 || len(launcher.existsLabels) != 1 {
		t.Fatalf("launcher calls = exists %#v jobs %#v", launcher.existsLabels, launcher.jobs)
	}
	stored, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if stored.State() != StatePrepared {
		t.Fatalf("submission advanced checkpoint before supervisor ownership: %s", stored.State().String())
	}
	arguments := job.Arguments()
	arguments[0] = "mutated"
	if reflect.DeepEqual(job.Arguments(), arguments) {
		t.Fatal("OneShotJob exposed mutable argument ownership")
	}
}

func TestBuildOneShotJobPropagatesExplicitTerminationOptIn(t *testing.T) {
	root := newRestartProject(t)
	checkpoint := newPreparedFixture(
		t,
		root,
		"restart-explicit-term",
		digestOf('e'),
	)
	request := SubmissionRequest{
		SupervisorExecutable: executableFixture(t, root, "supervisor"),
		TaskExecutable:       executableFixture(t, root, "task"),
		QuitTimeout:          time.Minute,
		TerminationPolicy:    terminationAllowSIGTERM,
	}
	job, err := buildOneShotJob(checkpoint, request)
	if err != nil {
		t.Fatalf("buildOneShotJob: %v", err)
	}
	if !slices.Contains(job.Arguments(), "--allow-term") {
		t.Fatalf("explicit opt-in is absent from supervisor argv: %#v", job.Arguments())
	}
}

func TestSubmitPreparedRestartRejectsLoadedLabelAndNonPreparedState(t *testing.T) {
	root := newRestartProject(t)
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	checkpoint := newPreparedFixture(t, root, "restart-pending", digestOf('b'))
	if err := store.Prepare(checkpoint); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	request := SubmissionRequest{
		SupervisorExecutable: executableFixture(t, root, "supervisor"),
		TaskExecutable:       executableFixture(t, root, "task"),
		QuitTimeout:          time.Minute,
	}
	pending := &fakeOneShotLauncher{exists: true}
	if _, err := SubmitPreparedRestart(context.Background(), store, pending, request); !errors.Is(err, ErrRestartSubmissionPending) {
		t.Fatalf("pending label error = %v", err)
	}
	if len(pending.jobs) != 0 {
		t.Fatalf("pending label submitted jobs: %#v", pending.jobs)
	}
	change, err := MarkSubmitted(checkpoint)
	if err != nil {
		t.Fatalf("MarkSubmitted: %v", err)
	}
	if err := store.Apply(change); err != nil {
		t.Fatalf("Apply submitted: %v", err)
	}
	launcher := &fakeOneShotLauncher{}
	if _, err := SubmitPreparedRestart(context.Background(), store, launcher, request); !errors.Is(err, ErrLoopGuard) {
		t.Fatalf("submitted checkpoint error = %v", err)
	}
	if len(launcher.existsLabels) != 0 || len(launcher.jobs) != 0 {
		t.Fatalf("non-prepared checkpoint reached launcher: %#v %#v", launcher.existsLabels, launcher.jobs)
	}
	invalidAttempt := checkpoint
	invalidAttempt.attempt = 2
	if _, err := buildOneShotJob(invalidAttempt, request); !errors.Is(err, ErrLoopGuard) {
		t.Fatalf("attempt=2 error = %v", err)
	}
}

func TestCheckpointSubmitCommandUsesFakeLauncherAndReadableArgv(t *testing.T) {
	root := restartCommandProject(t)
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	checkpoint := newPreparedFixture(t, root, "restart-command-submit", digestOf('c'))
	if err := store.Prepare(checkpoint); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	launcher := &fakeOneShotLauncher{}
	command := checkpointCommand{launcher: launcher, now: time.Now}
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	code := command.run(
		context.Background(),
		[]string{
			"submit",
			"--project-root", root,
			"--supervisor", executableFixture(t, root, "supervisor-command"),
			"--task-executable", executableFixture(t, root, "task-command"),
			"--quit-timeout", "90s",
		},
		stdout,
		stderr,
	)
	if code != 0 {
		t.Fatalf("submit code = %d stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "argv[0]") || len(launcher.jobs) != 1 {
		t.Fatalf("submit output/jobs = %s / %#v", stdout.String(), launcher.jobs)
	}
}

func TestOneShotLaunchAgentPlistDisablesKeepAliveAndEscapesArguments(t *testing.T) {
	job := OneShotJob{
		label:      "com.openai.codex.haft-restart.test",
		executable: "/tmp/supervisor&exact",
		arguments:  []string{"--value", "<not-shell-text>"},
	}
	content, err := oneShotLaunchAgentPlist(job)
	if err != nil {
		t.Fatalf("oneShotLaunchAgentPlist: %v", err)
	}
	encoded := string(content)
	required := []string{
		"<key>RunAtLoad</key><true/>",
		"<key>KeepAlive</key><false/>",
		"<string>/tmp/supervisor&amp;exact</string>",
		"<string>&lt;not-shell-text&gt;</string>",
	}
	for _, fragment := range required {
		if !strings.Contains(encoded, fragment) {
			t.Fatalf("launch agent omits %q: %s", fragment, encoded)
		}
	}
}

func TestSmokeCommandPassesMetacharactersAsOneArgument(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "must-not-exist")
	script := filepath.Join(root, "echo-argv")
	content := []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n")
	if err := os.WriteFile(script, content, 0o700); err != nil {
		t.Fatalf("write argv script: %v", err)
	}
	argument := "$(touch " + marker + ")"
	output := bytes.NewBuffer(nil)
	if err := runSmokeCommand(context.Background(), root, "argv", script, []string{argument}, output); err != nil {
		t.Fatalf("runSmokeCommand: %v", err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("metacharacter argument was evaluated: %v", err)
	}
	if !strings.Contains(output.String(), argument) {
		t.Fatalf("argument not preserved in output: %s", output.String())
	}
}

func TestMCPProcessArgumentsRequireExactHaftServe(t *testing.T) {
	if !isHaftServeArguments([]string{"/Users/operator/.local/bin/haft", "serve"}) {
		t.Fatal("exact Haft serve argv was rejected")
	}
	invalid := [][]string{
		{"/Users/operator/.local/bin/haft", "query", "status"},
		{"/bin/sh", "-lc", "haft serve"},
		{"/Users/operator/.local/bin/other", "serve"},
		{"/Users/operator/.local/bin/haft"},
		{"/Users/operator/.local/bin/haft", "serve", "--project-root", "/tmp/other"},
	}
	for _, arguments := range invalid {
		if isHaftServeArguments(arguments) {
			t.Fatalf("non-MCP argv accepted: %#v", arguments)
		}
	}
}

func executableFixture(t *testing.T, root string, name string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte("executable fixture"), 0o700); err != nil {
		t.Fatalf("write executable fixture: %v", err)
	}
	return path
}
