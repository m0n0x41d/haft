//go:build darwin

package agenthostrestart

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	codexTaskRuntimeExecutableSuffix = "/ChatGPT.app/Contents/Resources/codex"
	maximumTaskRuntimeAncestryDepth  = 32
)

type taskRuntimeProcessObservation struct {
	identity  TaskRuntimeIdentity
	parentPID int
	arguments []string
}

type taskRuntimeProcessObserver func(
	context.Context,
	int,
) (taskRuntimeProcessObservation, error)

func captureCurrentCodexTaskRuntime(
	ctx context.Context,
) (TaskRuntimeIdentity, error) {
	return captureCodexTaskRuntimeAncestor(
		ctx,
		os.Getppid(),
		0,
		observeTaskRuntimeProcess,
	)
}

func captureCodexTaskRuntimeAncestor(
	ctx context.Context,
	pid int,
	depth int,
	observe taskRuntimeProcessObserver,
) (TaskRuntimeIdentity, error) {
	if pid <= 1 || depth >= maximumTaskRuntimeAncestryDepth {
		return TaskRuntimeIdentity{}, fmt.Errorf(
			"no owning Codex app-server found in process ancestry",
		)
	}
	observation, err := observe(ctx, pid)
	if err != nil {
		return TaskRuntimeIdentity{}, err
	}
	if isCodexTaskRuntime(observation) {
		return observation.identity, nil
	}
	return captureCodexTaskRuntimeAncestor(
		ctx,
		observation.parentPID,
		depth+1,
		observe,
	)
}

func observeTaskRuntimeProcess(
	ctx context.Context,
	pid int,
) (taskRuntimeProcessObservation, error) {
	parentPID, err := observeProcessParent(ctx, pid)
	if err != nil {
		return taskRuntimeProcessObservation{}, err
	}
	commandLine, err := observeProcessCommand(ctx, pid)
	if err != nil {
		return taskRuntimeProcessObservation{}, err
	}
	fields := strings.Fields(commandLine)
	if len(fields) == 0 {
		return taskRuntimeProcessObservation{}, fmt.Errorf(
			"process %d has no command",
			pid,
		)
	}
	executablePath := fields[0]
	if filepath.IsAbs(executablePath) {
		executablePath, err = canonicalExistingFile(executablePath)
		if err != nil {
			return taskRuntimeProcessObservation{}, fmt.Errorf(
				"process %d executable: %w",
				pid,
				err,
			)
		}
	}
	startedAt, err := observeProcessStart(ctx, pid)
	if err != nil {
		return taskRuntimeProcessObservation{}, err
	}
	arguments := append([]string(nil), fields[1:]...)
	return taskRuntimeProcessObservation{
		identity: TaskRuntimeIdentity{
			PID:             pid,
			StartedAt:       startedAt,
			ExecutablePath:  executablePath,
			ArgumentsDigest: digestProcessArguments(arguments),
		},
		parentPID: parentPID,
		arguments: arguments,
	}, nil
}

func observeProcessParent(ctx context.Context, pid int) (int, error) {
	command := exec.CommandContext(
		ctx,
		"/bin/ps",
		"-p",
		strconv.Itoa(pid),
		"-o",
		"ppid=",
	)
	output, err := command.Output()
	if err != nil {
		return 0, err
	}
	parentPID, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0, fmt.Errorf("parse parent PID for process %d: %w", pid, err)
	}
	return parentPID, nil
}

func observeProcessCommand(ctx context.Context, pid int) (string, error) {
	command := exec.CommandContext(
		ctx,
		"/bin/ps",
		"-ww",
		"-p",
		strconv.Itoa(pid),
		"-o",
		"command=",
	)
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	commandLine := strings.TrimSpace(string(output))
	if commandLine == "" {
		return "", fmt.Errorf("process %d has an empty command", pid)
	}
	return commandLine, nil
}

func isCodexTaskRuntime(
	observation taskRuntimeProcessObservation,
) bool {
	executablePath := filepath.ToSlash(observation.identity.ExecutablePath)
	return strings.HasSuffix(
		executablePath,
		codexTaskRuntimeExecutableSuffix,
	) &&
		slices.Contains(observation.arguments, "features.code_mode_host=true") &&
		slices.Contains(observation.arguments, "app-server")
}

func digestProcessArguments(arguments []string) string {
	return digestText(strings.Join(arguments, "\x00"))
}

func (effects commandEffects) TerminateTaskRuntime(
	ctx context.Context,
	identity TaskRuntimeIdentity,
	output io.Writer,
) error {
	return terminateTaskRuntimeWithPolicy(
		ctx,
		effects.terminationPolicy,
		identity,
		output,
		observeTaskRuntimeProcess,
		syscall.Kill,
	)
}

func terminateTaskRuntimeWithPolicy(
	ctx context.Context,
	policy applicationTerminationPolicy,
	identity TaskRuntimeIdentity,
	output io.Writer,
	observe taskRuntimeProcessObserver,
	signal func(int, syscall.Signal) error,
) error {
	if err := policy.validate(); err != nil {
		return err
	}
	if policy == terminationGracefulOnly {
		return nil
	}
	return terminateTaskRuntime(
		ctx,
		identity,
		output,
		observe,
		signal,
	)
}

func terminateTaskRuntime(
	ctx context.Context,
	identity TaskRuntimeIdentity,
	output io.Writer,
	observe taskRuntimeProcessObserver,
	signal func(int, syscall.Signal) error,
) error {
	absent, err := taskRuntimeAbsent(ctx, identity, observe)
	if err != nil {
		return err
	}
	if absent {
		return nil
	}
	if _, err := fmt.Fprintf(
		output,
		"explicit termination escalation: Codex task runtime SIGTERM pid=%d\n",
		identity.PID,
	); err != nil {
		return err
	}
	if err := signal(identity.PID, syscall.SIGTERM); err != nil &&
		!errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

func (commandEffects) WaitTaskRuntimeAbsent(
	ctx context.Context,
	identity TaskRuntimeIdentity,
	timeout time.Duration,
) error {
	absent, err := waitTaskRuntimeAbsent(
		ctx,
		identity,
		timeout,
		observeTaskRuntimeProcess,
	)
	if err != nil {
		return err
	}
	if absent {
		return nil
	}
	return fmt.Errorf("old Codex task runtime remained after %s", timeout)
}

func waitTaskRuntimeAbsent(
	ctx context.Context,
	identity TaskRuntimeIdentity,
	timeout time.Duration,
	observe taskRuntimeProcessObserver,
) (bool, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		absent, err := taskRuntimeAbsent(ctx, identity, observe)
		if err != nil {
			return false, err
		}
		if absent {
			return true, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-deadline.C:
			return false, nil
		case <-ticker.C:
		}
	}
}

func taskRuntimeAbsent(
	ctx context.Context,
	identity TaskRuntimeIdentity,
	observe taskRuntimeProcessObserver,
) (bool, error) {
	err := syscall.Kill(identity.PID, 0)
	if errors.Is(err, syscall.ESRCH) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	observation, err := observe(ctx, identity.PID)
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
			return true, nil
		}
		return false, err
	}
	return !taskRuntimeIdentityMatches(identity, observation.identity), nil
}

func taskRuntimeIdentityMatches(
	expected TaskRuntimeIdentity,
	observed TaskRuntimeIdentity,
) bool {
	return expected.PID == observed.PID &&
		expected.StartedAt.Equal(observed.StartedAt) &&
		expected.ExecutablePath == observed.ExecutablePath &&
		expected.ArgumentsDigest == observed.ArgumentsDigest
}
