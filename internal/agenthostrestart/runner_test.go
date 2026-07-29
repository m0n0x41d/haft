//go:build darwin || linux

package agenthostrestart

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSupervisorInstallFailureNeverReachesApplicationBoundary(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*fakeSupervisorEffects, Checkpoint)
		wantCalls []string
	}{
		{
			name: "task install",
			configure: func(effects *fakeSupervisorEffects, _ Checkpoint) {
				effects.taskErr = errors.New("build failed")
			},
			wantCalls: []string{"capture", "task_install"},
		},
		{
			name: "haft init",
			configure: func(effects *fakeSupervisorEffects, _ Checkpoint) {
				effects.initErr = errors.New("carrier refresh failed")
			},
			wantCalls: []string{"capture", "task_install", "haft_init"},
		},
		{
			name: "installed digest mismatch",
			configure: func(effects *fakeSupervisorEffects, _ Checkpoint) {
				effects.installedDigest = digestOf('0')
			},
			wantCalls: []string{"capture", "task_install", "haft_init", "hash"},
		},
		{
			name: "installed digest read failure",
			configure: func(effects *fakeSupervisorEffects, _ Checkpoint) {
				effects.hashErr = errors.New("installed binary disappeared")
			},
			wantCalls: []string{"capture", "task_install", "haft_init", "hash"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, prepared := preparedSupervisorStore(t, "restart-fail-"+stringsToken(test.name))
			effects := newFakeSupervisorEffects(prepared)
			test.configure(effects, prepared)
			supervisor := mustSupervisor(t, store, effects)
			result, err := supervisor.Run(context.Background())
			if !errors.Is(err, ErrInstallStageFailed) {
				t.Fatalf("Run error = %v", err)
			}
			if result.State() != StateInstallFailed {
				t.Fatalf("result state = %s", result.State().String())
			}
			if calls := effects.Calls(); !reflect.DeepEqual(calls, test.wantCalls) {
				t.Fatalf("calls = %#v, want %#v", calls, test.wantCalls)
			}
			stored, loadErr := store.Load()
			if loadErr != nil {
				t.Fatalf("Load: %v", loadErr)
			}
			if stored.State() != StateInstallFailed {
				t.Fatalf("stored state = %s", stored.State().String())
			}
		})
	}
}

func TestSupervisorGracefulQuitTimeoutDoesNotOpenOrForceKill(t *testing.T) {
	store, prepared := preparedSupervisorStore(t, "restart-timeout")
	effects := newFakeSupervisorEffects(prepared)
	effects.waitErr = errors.New("old process still present")
	supervisor := mustSupervisor(t, store, effects)
	result, err := supervisor.Run(context.Background())
	if !errors.Is(err, ErrGracefulQuitTimeout) {
		t.Fatalf("Run error = %v", err)
	}
	if result.State() != StateSubmitted {
		t.Fatalf("result state = %s", result.State().String())
	}
	want := []string{"capture", "task_install", "haft_init", "hash", "capture_basis", "capture_carriers", "graceful_quit", "wait_absent"}
	if calls := effects.Calls(); !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
	stored, loadErr := store.Load()
	if loadErr != nil {
		t.Fatalf("Load: %v", loadErr)
	}
	if stored.State() != StateSubmitted {
		t.Fatalf("stored state = %s", stored.State().String())
	}
}

func TestSupervisorTaskRuntimeFailureDoesNotOpenThread(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*fakeSupervisorEffects)
		wantError error
		wantCalls []string
	}{
		{
			name: "termination failed",
			configure: func(effects *fakeSupervisorEffects) {
				effects.runtimeStopErr = errors.New("runtime identity changed")
			},
			wantError: ErrTaskRuntimeStopFailed,
			wantCalls: []string{
				"capture",
				"task_install",
				"haft_init",
				"hash",
				"capture_basis",
				"capture_carriers",
				"graceful_quit",
				"wait_absent",
				"terminate_runtime",
			},
		},
		{
			name: "termination timed out",
			configure: func(effects *fakeSupervisorEffects) {
				effects.runtimeWaitErr = errors.New("runtime remained")
			},
			wantError: ErrTaskRuntimeStopTimeout,
			wantCalls: []string{
				"capture",
				"task_install",
				"haft_init",
				"hash",
				"capture_basis",
				"capture_carriers",
				"graceful_quit",
				"wait_absent",
				"terminate_runtime",
				"wait_runtime_absent",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, prepared := preparedSupervisorStore(
				t,
				"restart-runtime-"+stringsToken(test.name),
			)
			effects := newFakeSupervisorEffects(prepared)
			test.configure(effects)
			supervisor := mustSupervisor(t, store, effects)
			result, err := supervisor.Run(context.Background())
			if !errors.Is(err, test.wantError) {
				t.Fatalf("Run error = %v", err)
			}
			if result.State() != StateSubmitted {
				t.Fatalf("result state = %s", result.State().String())
			}
			if calls := effects.Calls(); !reflect.DeepEqual(calls, test.wantCalls) {
				t.Fatalf("calls = %#v, want %#v", calls, test.wantCalls)
			}
		})
	}
}

func TestSupervisorPostInitCarrierOrBasisDriftFailsBeforeApplicationBoundary(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*fakeSupervisorEffects)
	}{
		{
			name: "swallowed MCP config warning left stale bytes",
			mutate: func(effects *fakeSupervisorEffects) {
				effects.carriers.MCPConfigDigest = digestOf('0')
			},
		},
		{
			name: "init changed selected TypeEnv head",
			mutate: func(effects *fakeSupervisorEffects) {
				effects.projectBasis.TypeEnvHeadRevision++
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, prepared := preparedSupervisorStore(t, "restart-prequit-"+stringsToken(test.name))
			effects := newFakeSupervisorEffects(prepared)
			test.mutate(effects)
			supervisor := mustSupervisor(t, store, effects)
			result, err := supervisor.Run(context.Background())
			if !errors.Is(err, ErrInstallStageFailed) {
				t.Fatalf("Run error = %v", err)
			}
			if result.State() != StateInstallFailed {
				t.Fatalf("result state = %s", result.State().String())
			}
			for _, call := range effects.Calls() {
				if call == "graceful_quit" || call == "open_exact_thread" {
					t.Fatalf("pre-quit mismatch reached application boundary: %#v", effects.Calls())
				}
			}
		})
	}
}

func TestSupervisorOpensOnlyExactPersistedThread(t *testing.T) {
	store, prepared := preparedSupervisorStore(t, "restart-open")
	effects := newFakeSupervisorEffects(prepared)
	configureAutoResume(t, store, effects, false)
	supervisor := mustSupervisor(t, store, effects)
	result, err := supervisor.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State() != StateResumed {
		t.Fatalf("result state = %s", result.State().String())
	}
	expectedLink := "codex://threads/" + prepared.ThreadID()
	if effects.OpenedLink() != expectedLink {
		t.Fatalf("opened link = %q, want %q", effects.OpenedLink(), expectedLink)
	}
	want := []string{
		"capture",
		"task_install",
		"haft_init",
		"hash",
		"capture_basis",
		"capture_carriers",
		"graceful_quit",
		"wait_absent",
		"terminate_runtime",
		"wait_runtime_absent",
		"open_exact_thread",
	}
	if calls := effects.Calls(); !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestSupervisorRequiresApplicationIdentityBeforeInstallation(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*fakeSupervisorEffects)
	}{
		{
			name: "capture failed",
			configure: func(effects *fakeSupervisorEffects) {
				effects.captureErr = errors.New("system events unavailable")
			},
		},
		{
			name: "application absent",
			configure: func(effects *fakeSupervisorEffects) {
				effects.oldApplication = ApplicationInstance{}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, prepared := preparedSupervisorStore(
				t,
				"restart-app-precondition-"+stringsToken(test.name),
			)
			effects := newFakeSupervisorEffects(prepared)
			test.configure(effects)
			supervisor := mustSupervisor(t, store, effects)
			result, err := supervisor.Run(context.Background())
			if err == nil || !strings.Contains(err.Error(), "capture old Codex process") {
				t.Fatalf("Run error = %v", err)
			}
			if result.State() != StateSubmitted {
				t.Fatalf("result state = %s", result.State().String())
			}
			if calls := effects.Calls(); !reflect.DeepEqual(calls, []string{"capture"}) {
				t.Fatalf("calls = %#v", calls)
			}
		})
	}
}

func TestSupervisorFallbackWakesExactThreadOnceAndClearsAfterResumeLease(t *testing.T) {
	store, prepared := preparedSupervisorStore(t, "restart-fallback")
	effects := newFakeSupervisorEffects(prepared)
	configureAutoResume(t, store, effects, true)
	supervisor := mustSupervisor(t, store, effects)
	result, err := supervisor.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State() != StateResumed {
		t.Fatalf("result state = %s", result.State().String())
	}
	receipt, err := store.LoadResumeFallbackReceipt(result)
	if err != nil {
		t.Fatalf("LoadResumeFallbackReceipt: %v", err)
	}
	if receipt.WakeCount != 1 || effects.OpenedLink() != "codex://threads/"+prepared.ThreadID() {
		t.Fatalf("fallback receipt/link = %#v / %q", receipt, effects.OpenedLink())
	}
	wakes := 0
	for _, call := range effects.Calls() {
		if call == "wake_exact_thread" {
			wakes++
		}
	}
	if wakes != 1 {
		t.Fatalf("fallback wakes = %d, calls %#v", wakes, effects.Calls())
	}
}

func TestSupervisorLeaseRejectsDuplicateWriter(t *testing.T) {
	store, prepared := preparedSupervisorStore(t, "restart-lease")
	effects := newFakeSupervisorEffects(prepared)
	effects.taskStarted = make(chan struct{})
	effects.releaseTask = make(chan struct{})
	configureAutoResume(t, store, effects, false)
	first := mustSupervisor(t, store, effects)
	second := mustSupervisor(t, store, effects)
	firstResult := make(chan error, 1)
	go func() {
		_, err := first.Run(context.Background())
		firstResult <- err
	}()
	<-effects.taskStarted
	if _, err := second.Run(context.Background()); !errors.Is(err, ErrDuplicateSupervisor) {
		t.Fatalf("second Run error = %v", err)
	}
	close(effects.releaseTask)
	if err := <-firstResult; err != nil {
		t.Fatalf("first Run: %v", err)
	}
}

func TestRunCommandRequiresExplicitAcceptanceCoordinates(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := RunCommand(context.Background(), nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code = %d", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("--project-root")) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

type fakeSupervisorEffects struct {
	mu              sync.Mutex
	calls           []string
	taskErr         error
	initErr         error
	hashErr         error
	installedDigest string
	projectBasis    ProjectBasisObservation
	carriers        CarrierObservation
	captureErr      error
	quitErr         error
	waitErr         error
	runtimeStopErr  error
	runtimeWaitErr  error
	openErr         error
	openedLink      string
	oldApplication  ApplicationInstance
	newStartedAt    time.Time
	afterOpen       func()
	afterWake       func()
	taskStarted     chan struct{}
	releaseTask     chan struct{}
}

func newFakeSupervisorEffects(checkpoint Checkpoint) *fakeSupervisorEffects {
	return &fakeSupervisorEffects{
		installedDigest: checkpoint.desiredHaftBinaryDigest,
		projectBasis:    validProjectBasis(checkpoint),
		carriers:        validCarrierObservation(checkpoint),
		oldApplication: ApplicationInstance{Processes: []ProcessIdentity{{
			PID:       42,
			StartedAt: checkpoint.createdAt.Add(-time.Hour),
		}}},
		newStartedAt: checkpoint.createdAt.Add(time.Minute),
	}
}

func (effects *fakeSupervisorEffects) RunTaskInstall(
	context.Context,
	string,
	io.Writer,
) error {
	effects.record("task_install")
	if effects.taskStarted != nil {
		close(effects.taskStarted)
	}
	if effects.releaseTask != nil {
		<-effects.releaseTask
	}
	return effects.taskErr
}

func (effects *fakeSupervisorEffects) RunHaftInit(
	context.Context,
	string,
	string,
	io.Writer,
) error {
	effects.record("haft_init")
	return effects.initErr
}

func (effects *fakeSupervisorEffects) HashFile(string) (string, error) {
	effects.record("hash")
	return effects.installedDigest, effects.hashErr
}

func (effects *fakeSupervisorEffects) CaptureProjectBasis(
	context.Context,
	string,
) (ProjectBasisObservation, error) {
	effects.record("capture_basis")
	return effects.projectBasis, nil
}

func (effects *fakeSupervisorEffects) CaptureCarriers(
	string,
	string,
	string,
) (CarrierObservation, error) {
	effects.record("capture_carriers")
	return effects.carriers, nil
}

func (effects *fakeSupervisorEffects) CaptureApplication(
	context.Context,
	string,
) (ApplicationInstance, error) {
	effects.record("capture")
	return effects.oldApplication, effects.captureErr
}

func (effects *fakeSupervisorEffects) GracefulQuit(
	context.Context,
	string,
	ApplicationInstance,
	io.Writer,
) error {
	effects.record("graceful_quit")
	return effects.quitErr
}

func (effects *fakeSupervisorEffects) WaitApplicationAbsent(
	context.Context,
	ApplicationInstance,
	time.Duration,
) error {
	effects.record("wait_absent")
	return effects.waitErr
}

func (effects *fakeSupervisorEffects) TerminateTaskRuntime(
	context.Context,
	TaskRuntimeIdentity,
	io.Writer,
) error {
	effects.record("terminate_runtime")
	return effects.runtimeStopErr
}

func (effects *fakeSupervisorEffects) WaitTaskRuntimeAbsent(
	context.Context,
	TaskRuntimeIdentity,
	time.Duration,
) error {
	effects.record("wait_runtime_absent")
	return effects.runtimeWaitErr
}

func (effects *fakeSupervisorEffects) OpenExactThread(
	_ context.Context,
	deepLink string,
	_ time.Duration,
	_ io.Writer,
) (time.Time, error) {
	effects.record("open_exact_thread")
	effects.mu.Lock()
	effects.openedLink = deepLink
	effects.mu.Unlock()
	if effects.afterOpen != nil {
		effects.afterOpen()
	}
	return effects.newStartedAt, effects.openErr
}

func (effects *fakeSupervisorEffects) WakeExactThread(
	_ context.Context,
	deepLink string,
	_ io.Writer,
) error {
	effects.record("wake_exact_thread")
	effects.mu.Lock()
	effects.openedLink = deepLink
	effects.mu.Unlock()
	if effects.afterWake != nil {
		effects.afterWake()
	}
	return effects.openErr
}

func (effects *fakeSupervisorEffects) Calls() []string {
	effects.mu.Lock()
	defer effects.mu.Unlock()
	return append([]string(nil), effects.calls...)
}

func (effects *fakeSupervisorEffects) OpenedLink() string {
	effects.mu.Lock()
	defer effects.mu.Unlock()
	return effects.openedLink
}

func (effects *fakeSupervisorEffects) record(call string) {
	effects.mu.Lock()
	defer effects.mu.Unlock()
	effects.calls = append(effects.calls, call)
}

func preparedSupervisorStore(
	t *testing.T,
	restartID string,
) (Store, Checkpoint) {
	t.Helper()
	root := newRestartProject(t)
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	prepared := newPreparedFixture(t, root, restartID, digestOf('d'))
	if err := store.Prepare(prepared); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	return store, prepared
}

func mustSupervisor(
	t *testing.T,
	store Store,
	effects Effects,
) Supervisor {
	t.Helper()
	supervisor, err := NewSupervisor(store, effects, io.Discard, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("NewSupervisor: %v", err)
	}
	return supervisor
}

func configureAutoResume(
	t *testing.T,
	store Store,
	effects *fakeSupervisorEffects,
	onFallback bool,
) {
	t.Helper()
	resume := func() {
		go func() {
			deadline := time.NewTimer(time.Second)
			defer deadline.Stop()
			ticker := time.NewTicker(time.Millisecond)
			defer ticker.Stop()
			for {
				checkpoint, err := store.Load()
				if err == nil && checkpoint.State() == StateAppOpened {
					change, changeErr := MarkResumed(checkpoint, ResumeObservation{
						ThreadID:           checkpoint.threadID,
						ResumeIntentDigest: checkpoint.resumeIntentDigest,
						RepositoryRoot:     checkpoint.repositoryRoot,
					})
					if changeErr == nil {
						_ = store.Apply(change)
					}
					return
				}
				select {
				case <-deadline.C:
					return
				case <-ticker.C:
				}
			}
		}()
	}
	if onFallback {
		effects.afterWake = resume
		return
	}
	effects.afterOpen = resume
}

func stringsToken(value string) string {
	result := make([]rune, 0, len(value))
	for _, runeValue := range value {
		if runeValue == ' ' {
			result = append(result, '-')
			continue
		}
		result = append(result, runeValue)
	}
	return string(result)
}
