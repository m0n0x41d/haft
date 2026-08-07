//go:build darwin

package agenthostrestart

import (
	"context"
	"io"
	"os"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

const liveTaskRuntimeCaptureEnvironmentKey = "HAFT_LIVE_TASK_RUNTIME_CAPTURE"

func TestCaptureCurrentCodexTaskRuntimeLive(t *testing.T) {
	if os.Getenv(liveTaskRuntimeCaptureEnvironmentKey) != "1" {
		t.Skip("set HAFT_LIVE_TASK_RUNTIME_CAPTURE=1 inside a Codex task runtime")
	}
	identity, err := captureCurrentCodexTaskRuntime(context.Background())
	if err != nil {
		t.Fatalf("captureCurrentCodexTaskRuntime: %v", err)
	}
	if identity.PID <= 1 ||
		identity.StartedAt.IsZero() ||
		identity.ExecutablePath == "" ||
		identity.ArgumentsDigest == "" {
		t.Fatalf("captured task runtime identity is incomplete: %#v", identity)
	}
	t.Logf(
		"captured exact Codex task runtime pid=%d started_at=%s",
		identity.PID,
		identity.StartedAt.Format(time.RFC3339),
	)
}

func TestCaptureCodexTaskRuntimeSelectsExactOwningAncestor(t *testing.T) {
	startedAt := time.Date(2026, 7, 27, 5, 20, 55, 0, time.UTC)
	observations := map[int]taskRuntimeProcessObservation{
		300: {
			identity: TaskRuntimeIdentity{
				PID:             300,
				StartedAt:       startedAt.Add(time.Hour),
				ExecutablePath:  "/bin/zsh",
				ArgumentsDigest: digestProcessArguments([]string{"-c", "haft"}),
			},
			parentPID: 200,
			arguments: []string{"-c", "haft"},
		},
		200: {
			identity: TaskRuntimeIdentity{
				PID:            200,
				StartedAt:      startedAt,
				ExecutablePath: "/Applications/ChatGPT.app/Contents/Resources/codex",
				ArgumentsDigest: digestProcessArguments([]string{
					"-c",
					"features.code_mode_host=true",
					"app-server",
				}),
			},
			parentPID: 1,
			arguments: []string{
				"-c",
				"features.code_mode_host=true",
				"app-server",
			},
		},
	}
	observer := func(
		_ context.Context,
		pid int,
	) (taskRuntimeProcessObservation, error) {
		return observations[pid], nil
	}
	identity, err := captureCodexTaskRuntimeAncestor(
		context.Background(),
		300,
		0,
		observer,
	)
	if err != nil {
		t.Fatalf("captureCodexTaskRuntimeAncestor: %v", err)
	}
	if !reflect.DeepEqual(identity, observations[200].identity) {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestCaptureCodexTaskRuntimeRejectsNonOwningAncestry(t *testing.T) {
	observer := func(
		_ context.Context,
		pid int,
	) (taskRuntimeProcessObservation, error) {
		return taskRuntimeProcessObservation{
			identity: TaskRuntimeIdentity{
				PID:             pid,
				StartedAt:       time.Date(2026, 7, 27, 5, 20, 55, 0, time.UTC),
				ExecutablePath:  "/usr/local/bin/codex",
				ArgumentsDigest: digestProcessArguments([]string{"app-server"}),
			},
			parentPID: 1,
			arguments: []string{"app-server"},
		}, nil
	}
	_, err := captureCodexTaskRuntimeAncestor(
		context.Background(),
		300,
		0,
		observer,
	)
	if err == nil || !strings.Contains(err.Error(), "no owning Codex app-server") {
		t.Fatalf("capture error = %v", err)
	}
}

func TestTerminateTaskRuntimeSignalsOnlyExactPIDGenerationAndRole(t *testing.T) {
	pid := os.Getpid()
	identity := TaskRuntimeIdentity{
		PID:             pid,
		StartedAt:       time.Date(2026, 7, 27, 5, 20, 55, 0, time.UTC),
		ExecutablePath:  "/Applications/ChatGPT.app/Contents/Resources/codex",
		ArgumentsDigest: digestProcessArguments([]string{"app-server"}),
	}
	observed := taskRuntimeProcessObservation{identity: identity}
	observer := func(
		context.Context,
		int,
	) (taskRuntimeProcessObservation, error) {
		return observed, nil
	}
	signals := make([]syscall.Signal, 0)
	signal := func(observedPID int, observedSignal syscall.Signal) error {
		if observedPID != pid {
			t.Fatalf("signal PID = %d", observedPID)
		}
		signals = append(signals, observedSignal)
		return nil
	}
	if err := terminateTaskRuntimeWithPolicy(
		context.Background(),
		terminationGracefulOnly,
		identity,
		io.Discard,
		observer,
		signal,
	); err != nil {
		t.Fatalf("graceful-only runtime policy: %v", err)
	}
	if len(signals) != 0 {
		t.Fatalf("default runtime policy sent signals = %v", signals)
	}

	if err := terminateTaskRuntimeWithPolicy(
		context.Background(),
		terminationAllowSIGTERM,
		identity,
		io.Discard,
		observer,
		signal,
	); err != nil {
		t.Fatalf("terminate exact runtime: %v", err)
	}
	if !reflect.DeepEqual(signals, []syscall.Signal{syscall.SIGTERM}) {
		t.Fatalf("signals = %v", signals)
	}

	signals = signals[:0]
	observed.identity.ArgumentsDigest = digestProcessArguments([]string{"other-role"})
	if err := terminateTaskRuntimeWithPolicy(
		context.Background(),
		terminationAllowSIGTERM,
		identity,
		io.Discard,
		observer,
		signal,
	); err != nil {
		t.Fatalf("terminate changed runtime: %v", err)
	}
	if len(signals) != 0 {
		t.Fatalf("changed runtime received signals = %v", signals)
	}
}
