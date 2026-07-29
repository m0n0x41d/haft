//go:build darwin

package agenthostrestart

import (
	"bytes"
	"context"
	"io"
	"os"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestParseApplicationProcessIDsAcceptsExactLSAppInfoResult(t *testing.T) {
	processIDs, err := parseApplicationProcessIDs(
		[]byte(`[ NULL ]  ASN:0x0-0x7c27c2:
    bundleID="com.openai.codex"
    pid = 90000 type="Foreground"
    pid = 78721 type="Foreground"
    pid = 90000 type="Foreground"
`),
		"com.openai.codex",
	)
	if err != nil {
		t.Fatalf("parseApplicationProcessIDs: %v", err)
	}
	if !slices.Equal(processIDs, []int{78721, 90000}) {
		t.Fatalf("process IDs = %v", processIDs)
	}
}

func TestParseApplicationProcessIDsRejectsInvalidOrNonPositivePID(t *testing.T) {
	for _, output := range [][]byte{
		[]byte(`bundleID="com.openai.codex"`),
		[]byte("bundleID=\"com.openai.codex\"\npid = not-a-pid"),
		[]byte("bundleID=\"com.openai.codex\"\npid = 0"),
		[]byte("bundleID=\"com.openai.codex\"\npid = -4"),
		[]byte("bundleID=\"com.openai.other\"\npid = 42"),
	} {
		if _, err := parseApplicationProcessIDs(output, "com.openai.codex"); err == nil {
			t.Fatalf("invalid process output %q unexpectedly accepted", output)
		}
	}
}

func TestParseApplicationProcessIDsTreatsNullLSAppInfoRecordAsAbsent(t *testing.T) {
	processIDs, err := parseApplicationProcessIDs(
		[]byte("[ NULL ]\n    bundleID=[ NULL ]\n    pid = 0\n"),
		"com.openai.codex",
	)
	if err != nil {
		t.Fatalf("parseApplicationProcessIDs: %v", err)
	}
	if len(processIDs) != 0 {
		t.Fatalf("process IDs = %v", processIDs)
	}
}

func TestProcessIdentityGenerationMatcherRejectsPIDReuse(t *testing.T) {
	fixed := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	identity := ProcessIdentity{PID: 42, StartedAt: fixed}
	if !processIdentityMatches(identity, fixed) {
		t.Fatal("equal PID generation start was not recognized")
	}
	if processIdentityMatches(identity, fixed.Add(time.Second)) {
		t.Fatal("reused PID generation was accepted as the old process")
	}
}

func TestOldProcessIdentityDistinguishesSamePIDFromPIDReuse(t *testing.T) {
	ctx := context.Background()
	pid := os.Getpid()
	startedAt, err := observeProcessStart(ctx, pid)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skip("sandbox does not permit child ps observation")
		}
		t.Fatalf("observe current process start: %v", err)
	}
	same := ApplicationInstance{Processes: []ProcessIdentity{{
		PID:       pid,
		StartedAt: startedAt,
	}}}
	absent, err := oldProcessesAbsent(ctx, same)
	if err != nil {
		t.Fatalf("oldProcessesAbsent(same): %v", err)
	}
	if absent {
		t.Fatal("same PID and start time was treated as absent")
	}
	reused := ApplicationInstance{Processes: []ProcessIdentity{{
		PID:       pid,
		StartedAt: startedAt.Add(-time.Second),
	}}}
	absent, err = oldProcessesAbsent(ctx, reused)
	if err != nil {
		t.Fatalf("oldProcessesAbsent(reused): %v", err)
	}
	if !absent {
		t.Fatal("same PID with a different start time was treated as the old process")
	}
}

func TestSignalOldApplicationTerminationTargetsOnlyMatchingPIDGeneration(t *testing.T) {
	ctx := context.Background()
	pid := os.Getpid()
	startedAt, err := observeProcessStart(ctx, pid)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skip("sandbox does not permit child ps observation")
		}
		t.Fatalf("observe current process start: %v", err)
	}
	instance := ApplicationInstance{Processes: []ProcessIdentity{
		{PID: pid, StartedAt: startedAt.Add(-time.Second)},
		{PID: pid, StartedAt: startedAt},
	}}
	signals := make([]syscall.Signal, 0)
	processIDs := make([]int, 0)
	signal := func(observedPID int, observedSignal syscall.Signal) error {
		processIDs = append(processIDs, observedPID)
		signals = append(signals, observedSignal)
		return nil
	}
	var output bytes.Buffer
	err = signalOldApplicationTermination(ctx, instance, &output, signal)
	if err != nil {
		t.Fatalf("signalOldApplicationTermination: %v", err)
	}
	if !reflect.DeepEqual(processIDs, []int{pid}) {
		t.Fatalf("process IDs = %v", processIDs)
	}
	if !reflect.DeepEqual(signals, []syscall.Signal{syscall.SIGTERM}) {
		t.Fatalf("signals = %v", signals)
	}
	if !strings.Contains(output.String(), "explicit termination escalation: SIGTERM pid=") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestApplicationTerminationPolicyRequiresExplicitOptIn(t *testing.T) {
	ctx := context.Background()
	instance := ApplicationInstance{Processes: []ProcessIdentity{{
		PID:       42,
		StartedAt: time.Date(2026, 7, 27, 8, 0, 0, 0, time.UTC),
	}}}
	waitCalls := 0
	terminateCalls := 0
	wait := func(
		context.Context,
		ApplicationInstance,
		time.Duration,
	) (bool, error) {
		waitCalls++
		return false, nil
	}
	terminate := func(
		context.Context,
		ApplicationInstance,
		io.Writer,
	) error {
		terminateCalls++
		return nil
	}
	if err := applyApplicationTerminationPolicy(
		ctx,
		terminationGracefulOnly,
		instance,
		io.Discard,
		wait,
		terminate,
	); err != nil {
		t.Fatalf("graceful-only policy: %v", err)
	}
	if waitCalls != 0 || terminateCalls != 0 {
		t.Fatalf(
			"graceful-only policy used termination callbacks: wait=%d terminate=%d",
			waitCalls,
			terminateCalls,
		)
	}
	if err := applyApplicationTerminationPolicy(
		ctx,
		applicationTerminationPolicyForAllowTerm(true),
		instance,
		io.Discard,
		wait,
		terminate,
	); err != nil {
		t.Fatalf("explicit SIGTERM policy: %v", err)
	}
	if waitCalls != 1 || terminateCalls != 1 {
		t.Fatalf(
			"explicit SIGTERM policy calls: wait=%d terminate=%d",
			waitCalls,
			terminateCalls,
		)
	}
}

func TestRuntimeProcessIdentityRejectsStalePIDAndCapturesExactCurrentProcess(t *testing.T) {
	ctx := context.Background()
	path, startedAt, err := processExecutableIdentity(ctx, os.Getpid())
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skip("sandbox does not permit child process observation")
		}
		t.Fatalf("processExecutableIdentity(current): %v", err)
	}
	if path == "" || startedAt.IsZero() {
		t.Fatalf("current process identity = %q %s", path, startedAt)
	}
	if _, _, err := processExecutableIdentity(ctx, 99999999); err == nil {
		t.Fatal("processExecutableIdentity accepted a stale PID")
	}
}
