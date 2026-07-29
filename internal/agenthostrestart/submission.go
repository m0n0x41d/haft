package agenthostrestart

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var ErrRestartSubmissionPending = errors.New("restart launchd submission already exists")

// OneShotJob is the exact argv-only detached supervisor invocation. It has no
// shell form and exposes defensive copies of its argument vector.
type OneShotJob struct {
	label      string
	executable string
	arguments  []string
}

func (job OneShotJob) Label() string      { return job.label }
func (job OneShotJob) Executable() string { return job.executable }
func (job OneShotJob) Arguments() []string {
	return append([]string(nil), job.arguments...)
}

// OneShotLauncher is the sole launchd effect boundary. Tests use a fake; the
// production implementation calls launchctl directly with an argv slice.
type OneShotLauncher interface {
	Exists(context.Context, string) (bool, error)
	Submit(context.Context, OneShotJob) error
	Remove(context.Context, string) error
}

type SubmissionRequest struct {
	SupervisorExecutable string
	TaskExecutable       string
	QuitTimeout          time.Duration
	TerminationPolicy    applicationTerminationPolicy
}

// SubmitPreparedRestart submits one prepared attempt without advancing the
// checkpoint. The detached supervisor owns prepared -> submitted so a process
// crash cannot leave a submitted checkpoint with no surviving owner.
func SubmitPreparedRestart(
	ctx context.Context,
	store Store,
	launcher OneShotLauncher,
	request SubmissionRequest,
) (OneShotJob, error) {
	if launcher == nil {
		return OneShotJob{}, fmt.Errorf("one-shot launcher is required")
	}
	checkpoint, err := store.Load()
	if err != nil {
		return OneShotJob{}, err
	}
	job, err := buildOneShotJob(checkpoint, request)
	if err != nil {
		return OneShotJob{}, err
	}
	exists, err := launcher.Exists(ctx, job.label)
	if err != nil {
		return OneShotJob{}, fmt.Errorf("inspect restart launchd label: %w", err)
	}
	if exists {
		return OneShotJob{}, fmt.Errorf("%w: %s", ErrRestartSubmissionPending, job.label)
	}
	if err := launcher.Submit(ctx, job); err != nil {
		return OneShotJob{}, fmt.Errorf("submit restart launchd job: %w", err)
	}
	return job, nil
}

func buildOneShotJob(
	checkpoint Checkpoint,
	request SubmissionRequest,
) (OneShotJob, error) {
	if checkpoint.state != StatePrepared {
		return OneShotJob{}, fmt.Errorf(
			"%w: checkpoint %s is %s rather than prepared",
			ErrLoopGuard,
			checkpoint.restartID,
			checkpoint.state.String(),
		)
	}
	if checkpoint.attempt != checkpointAttempt {
		return OneShotJob{}, fmt.Errorf("%w: restart attempt is not exactly 1", ErrLoopGuard)
	}
	if request.QuitTimeout < 2*time.Nanosecond || request.QuitTimeout > 5*time.Minute {
		return OneShotJob{}, fmt.Errorf("quit timeout must be within [2ns, 5m]")
	}
	if err := request.TerminationPolicy.validate(); err != nil {
		return OneShotJob{}, err
	}
	supervisor, err := canonicalExistingFile(request.SupervisorExecutable)
	if err != nil {
		return OneShotJob{}, fmt.Errorf("supervisor executable: %w", err)
	}
	if err := ensureExecutablePath(supervisor); err != nil {
		return OneShotJob{}, fmt.Errorf("supervisor executable: %w", err)
	}
	task, err := canonicalExistingFile(request.TaskExecutable)
	if err != nil {
		return OneShotJob{}, fmt.Errorf("task executable: %w", err)
	}
	if err := ensureExecutablePath(task); err != nil {
		return OneShotJob{}, fmt.Errorf("task executable: %w", err)
	}
	arguments := []string{
		"--project-root",
		checkpoint.repositoryRoot,
		"--task-executable",
		task,
		"--quit-timeout",
		request.QuitTimeout.String(),
	}
	arguments = request.TerminationPolicy.appendSupervisorArguments(arguments)
	return OneShotJob{
		label:      checkpoint.launchdLabel,
		executable: supervisor,
		arguments:  arguments,
	}, nil
}

type launchctlLauncher struct {
	executable string
	domain     string
}

func NewLaunchctlLauncher() (OneShotLauncher, error) {
	executable, err := canonicalExistingFile("/bin/launchctl")
	if err != nil {
		return nil, fmt.Errorf("launchctl executable: %w", err)
	}
	return launchctlLauncher{
		executable: executable,
		domain:     "gui/" + strconv.Itoa(os.Getuid()),
	}, nil
}

func (launcher launchctlLauncher) Exists(
	ctx context.Context,
	label string,
) (bool, error) {
	target := launcher.domain + "/" + label
	command := exec.CommandContext(ctx, launcher.executable, "print", target)
	output, err := command.CombinedOutput()
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && launchctlServiceMissing(output) {
		return false, nil
	}
	return false, fmt.Errorf(
		"launchctl print %s: %w: %s",
		target,
		err,
		strings.TrimSpace(string(output)),
	)
}

func (launcher launchctlLauncher) Submit(
	ctx context.Context,
	job OneShotJob,
) error {
	directory, err := os.MkdirTemp("", "haft-restart-launchagent-")
	if err != nil {
		return fmt.Errorf("create one-shot launch agent directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(directory)
	}()
	content, err := oneShotLaunchAgentPlist(job)
	if err != nil {
		return err
	}
	path := filepath.Join(directory, job.label+".plist")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("write one-shot launch agent: %w", err)
	}
	command := exec.CommandContext(
		ctx,
		launcher.executable,
		"bootstrap",
		launcher.domain,
		path,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl bootstrap: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (launcher launchctlLauncher) Remove(
	ctx context.Context,
	label string,
) error {
	target := launcher.domain + "/" + label
	command := exec.CommandContext(
		ctx,
		launcher.executable,
		"bootout",
		target,
	)
	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && launchctlServiceMissing(output) {
		return nil
	}
	return fmt.Errorf(
		"launchctl bootout %s: %w: %s",
		target,
		err,
		strings.TrimSpace(string(output)),
	)
}

func oneShotLaunchAgentPlist(job OneShotJob) ([]byte, error) {
	builder := strings.Builder{}
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	builder.WriteString("\n")
	builder.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "https://www.apple.com/DTDs/PropertyList-1.0.dtd">`)
	builder.WriteString("\n")
	builder.WriteString(`<plist version="1.0"><dict>`)
	builder.WriteString(`<key>Label</key>`)
	if err := appendLaunchAgentPlistString(&builder, job.label); err != nil {
		return nil, err
	}
	builder.WriteString(`<key>ProgramArguments</key><array>`)
	arguments := append([]string{job.executable}, job.arguments...)
	for _, argument := range arguments {
		if err := appendLaunchAgentPlistString(&builder, argument); err != nil {
			return nil, err
		}
	}
	builder.WriteString(`</array>`)
	builder.WriteString(`<key>RunAtLoad</key><true/>`)
	builder.WriteString(`<key>KeepAlive</key><false/>`)
	builder.WriteString(`<key>ProcessType</key><string>Background</string>`)
	builder.WriteString(`</dict></plist>`)
	builder.WriteString("\n")
	return []byte(builder.String()), nil
}

func appendLaunchAgentPlistString(
	builder *strings.Builder,
	value string,
) error {
	builder.WriteString(`<string>`)
	if err := xml.EscapeText(builder, []byte(value)); err != nil {
		return fmt.Errorf("escape one-shot launch agent value: %w", err)
	}
	builder.WriteString(`</string>`)
	return nil
}

func launchctlServiceMissing(output []byte) bool {
	message := strings.ToLower(string(output))
	markers := []string{
		"could not find service",
		"service not found",
		"no such process",
	}
	for _, marker := range markers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func ensureExecutablePath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("%s is not executable", filepath.Clean(path))
	}
	return nil
}
