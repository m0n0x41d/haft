//go:build darwin

package agenthostrestart

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type commandEffects struct {
	taskExecutable    string
	terminationPolicy applicationTerminationPolicy
}

const applicationQuitSettleWindow = 5 * time.Second

func NewCommandEffects(
	taskExecutable string,
	terminationPolicy applicationTerminationPolicy,
) (Effects, error) {
	if err := terminationPolicy.validate(); err != nil {
		return nil, err
	}
	path := filepath.Clean(taskExecutable)
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("task executable path must be absolute")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect task executable: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return nil, fmt.Errorf("task executable is not an executable regular file")
	}
	return commandEffects{
		taskExecutable:    path,
		terminationPolicy: terminationPolicy,
	}, nil
}

func (effects commandEffects) RunTaskInstall(
	ctx context.Context,
	repositoryRoot string,
	output io.Writer,
) error {
	command := exec.CommandContext(ctx, effects.taskExecutable, "install")
	command.Dir = repositoryRoot
	command.Stdout = output
	command.Stderr = output
	return command.Run()
}

func (commandEffects) RunHaftInit(
	ctx context.Context,
	haftExecutable string,
	repositoryRoot string,
	output io.Writer,
) error {
	command := exec.CommandContext(ctx, haftExecutable, "init", "--codex")
	command.Dir = repositoryRoot
	command.Stdout = output
	command.Stderr = output
	return command.Run()
}

func (commandEffects) HashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(digest.Sum(nil)), nil
}

func (commandEffects) CaptureProjectBasis(
	ctx context.Context,
	repositoryRoot string,
) (ProjectBasisObservation, error) {
	return captureProjectBasis(ctx, repositoryRoot)
}

func (commandEffects) CaptureCarriers(
	skillRoot string,
	instructionPath string,
	mcpConfigPath string,
) (CarrierObservation, error) {
	return captureCarrierObservation(skillRoot, instructionPath, mcpConfigPath)
}

func (commandEffects) CaptureApplication(
	ctx context.Context,
	bundleID string,
) (ApplicationInstance, error) {
	command := exec.CommandContext(
		ctx,
		"/usr/bin/lsappinfo",
		"info",
		"-only",
		"bundleID,pid",
		"-app",
		bundleID,
	)
	output, err := command.Output()
	if err != nil {
		return ApplicationInstance{}, err
	}
	processIDs, err := parseApplicationProcessIDs(output, bundleID)
	if err != nil {
		return ApplicationInstance{}, err
	}
	processes := make([]ProcessIdentity, 0)
	for _, pid := range processIDs {
		startedAt, startErr := observeProcessStart(ctx, pid)
		if startErr != nil {
			return ApplicationInstance{}, startErr
		}
		processes = append(processes, ProcessIdentity{PID: pid, StartedAt: startedAt})
	}
	sort.Slice(processes, func(left int, right int) bool {
		return processes[left].PID < processes[right].PID
	})
	return ApplicationInstance{Processes: processes}, nil
}

func parseApplicationProcessIDs(
	output []byte,
	bundleID string,
) ([]int, error) {
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return []int{}, nil
	}
	exactBundle := `bundleID="` + bundleID + `"`
	if !strings.Contains(trimmed, exactBundle) {
		if strings.Contains(trimmed, "bundleID=[ NULL ]") {
			return []int{}, nil
		}
		return nil, fmt.Errorf("lsappinfo result does not contain exact bundle ID %q", bundleID)
	}
	processIDs := make([]int, 0)
	seen := make(map[int]struct{})
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "pid = ") {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, "pid = "))
		if len(fields) == 0 {
			return nil, fmt.Errorf("lsappinfo result has an empty process PID")
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, fmt.Errorf("parse application process PID %q: %w", fields[0], err)
		}
		if pid <= 0 {
			return nil, fmt.Errorf("application process PID must be positive")
		}
		if _, exists := seen[pid]; exists {
			continue
		}
		seen[pid] = struct{}{}
		processIDs = append(processIDs, pid)
	}
	if len(processIDs) == 0 {
		return nil, fmt.Errorf("lsappinfo result has no process PID for bundle ID %q", bundleID)
	}
	sort.Ints(processIDs)
	return processIDs, nil
}

func (effects commandEffects) GracefulQuit(
	ctx context.Context,
	bundleID string,
	instance ApplicationInstance,
	output io.Writer,
) error {
	script := fmt.Sprintf(`tell application id %q to quit`, bundleID)
	command := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script)
	command.Stdout = output
	command.Stderr = output
	if err := command.Run(); err != nil {
		return err
	}
	terminate := func(
		terminationContext context.Context,
		oldApplication ApplicationInstance,
		terminationOutput io.Writer,
	) error {
		return signalOldApplicationTermination(
			terminationContext,
			oldApplication,
			terminationOutput,
			syscall.Kill,
		)
	}
	return applyApplicationTerminationPolicy(
		ctx,
		effects.terminationPolicy,
		instance,
		output,
		waitApplicationAbsent,
		terminate,
	)
}

type applicationAbsenceWaiter func(
	context.Context,
	ApplicationInstance,
	time.Duration,
) (bool, error)

type applicationTerminator func(
	context.Context,
	ApplicationInstance,
	io.Writer,
) error

func applyApplicationTerminationPolicy(
	ctx context.Context,
	policy applicationTerminationPolicy,
	instance ApplicationInstance,
	output io.Writer,
	wait applicationAbsenceWaiter,
	terminate applicationTerminator,
) error {
	if err := policy.validate(); err != nil {
		return err
	}
	if policy == terminationGracefulOnly {
		return nil
	}
	absent, err := wait(
		ctx,
		instance,
		applicationQuitSettleWindow,
	)
	if err != nil {
		return err
	}
	if absent {
		return nil
	}
	return terminate(
		ctx,
		instance,
		output,
	)
}

func (commandEffects) WaitApplicationAbsent(
	ctx context.Context,
	instance ApplicationInstance,
	timeout time.Duration,
) error {
	absent, err := waitApplicationAbsent(ctx, instance, timeout)
	if err != nil {
		return err
	}
	if absent {
		return nil
	}
	return fmt.Errorf("old Codex process remained after %s", timeout)
}

func waitApplicationAbsent(
	ctx context.Context,
	instance ApplicationInstance,
	timeout time.Duration,
) (bool, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		absent, err := oldProcessesAbsent(ctx, instance)
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

func signalOldApplicationTermination(
	ctx context.Context,
	instance ApplicationInstance,
	output io.Writer,
	signal func(int, syscall.Signal) error,
) error {
	processes, err := liveOldProcesses(ctx, instance)
	if err != nil {
		return err
	}
	for _, process := range processes {
		if _, err := fmt.Fprintf(
			output,
			"explicit termination escalation: SIGTERM pid=%d\n",
			process.PID,
		); err != nil {
			return err
		}
		if err := signal(process.PID, syscall.SIGTERM); err != nil &&
			!errors.Is(err, syscall.ESRCH) {
			return err
		}
	}
	return nil
}

func (effects commandEffects) OpenExactThread(
	ctx context.Context,
	deepLink string,
	timeout time.Duration,
	output io.Writer,
) (time.Time, error) {
	if !strings.HasPrefix(deepLink, "codex://threads/") {
		return time.Time{}, fmt.Errorf("codex deep link is not an exact task link")
	}
	openedAt := time.Now().UTC()
	command := exec.CommandContext(ctx, "/usr/bin/open", deepLink)
	command.Stdout = output
	command.Stderr = output
	if err := command.Run(); err != nil {
		return time.Time{}, err
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		instance, err := effects.CaptureApplication(ctx, codexApplicationBundleID)
		if err == nil {
			for _, process := range instance.Processes {
				if process.StartedAt.After(openedAt.Add(-2 * time.Second)) {
					return process.StartedAt, nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return time.Time{}, ctx.Err()
		case <-deadline.C:
			return time.Time{}, fmt.Errorf("new Codex process was not observed after %s", timeout)
		case <-ticker.C:
		}
	}
}

func (commandEffects) WakeExactThread(
	ctx context.Context,
	deepLink string,
	output io.Writer,
) error {
	if !strings.HasPrefix(deepLink, "codex://threads/") {
		return fmt.Errorf("codex deep link is not an exact task link")
	}
	command := exec.CommandContext(ctx, "/usr/bin/open", deepLink)
	command.Stdout = output
	command.Stderr = output
	return command.Run()
}

func oldProcessesAbsent(
	ctx context.Context,
	instance ApplicationInstance,
) (bool, error) {
	processes, err := liveOldProcesses(ctx, instance)
	if err != nil {
		return false, err
	}
	return len(processes) == 0, nil
}

func liveOldProcesses(
	ctx context.Context,
	instance ApplicationInstance,
) ([]ProcessIdentity, error) {
	processes := make([]ProcessIdentity, 0)
	for _, process := range instance.Processes {
		err := syscall.Kill(process.PID, 0)
		if errors.Is(err, syscall.ESRCH) {
			continue
		}
		if err != nil {
			return nil, err
		}
		startedAt, err := observeProcessStart(ctx, process.PID)
		if err != nil {
			var exitError *exec.ExitError
			if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
				continue
			}
			return nil, err
		}
		if processIdentityMatches(process, startedAt) {
			processes = append(processes, process)
		}
	}
	return processes, nil
}

func processIdentityMatches(expected ProcessIdentity, observedStart time.Time) bool {
	return observedStart.Equal(expected.StartedAt)
}

func observeProcessStart(ctx context.Context, pid int) (time.Time, error) {
	command := exec.CommandContext(
		ctx,
		"/bin/ps",
		"-p",
		strconv.Itoa(pid),
		"-o",
		"lstart=",
	)
	output, err := command.Output()
	if err != nil {
		return time.Time{}, err
	}
	startedAt, err := time.ParseInLocation(
		"Mon Jan _2 15:04:05 2006",
		strings.TrimSpace(string(output)),
		time.Local,
	)
	if err != nil {
		return time.Time{}, err
	}
	return startedAt.UTC(), nil
}
