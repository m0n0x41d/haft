package agenthostrestart

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const codexApplicationBundleID = "com.openai.codex"

var (
	ErrDuplicateSupervisor      = errors.New("another restart supervisor owns the checkpoint")
	ErrInstallStageFailed       = errors.New("restart install stage failed")
	ErrGracefulQuitFailed       = errors.New("graceful Codex quit failed")
	ErrGracefulQuitTimeout      = errors.New("graceful Codex quit timed out")
	ErrTaskRuntimeStopFailed    = errors.New("codex task runtime termination failed")
	ErrTaskRuntimeStopTimeout   = errors.New("codex task runtime termination timed out")
	ErrExactThreadOpenFailed    = errors.New("exact Codex task open failed")
	ErrExactThreadResumeTimeout = errors.New("exact Codex task did not acquire the resume lease")
)

// ProcessIdentity identifies the application process observed before quit.
// WaitApplicationAbsent must distinguish absence from PID reuse.
type ProcessIdentity struct {
	PID       int
	StartedAt time.Time
}

// TaskRuntimeIdentity is the exact Codex app-server generation that owns the
// preparing task. The executable and argument digests prevent a matching PID
// generation from being reinterpreted as another process role.
type TaskRuntimeIdentity struct {
	PID             int
	StartedAt       time.Time
	ExecutablePath  string
	ArgumentsDigest string
}

// ApplicationInstance is the exact old application process set.
type ApplicationInstance struct {
	Processes []ProcessIdentity
}

// Effects is the imperative boundary owned by the detached one-shot process.
// SIGTERM escalation is disabled by default and can only be enabled by the
// hidden supervisor's explicit termination policy. SIGKILL is not supported.
type Effects interface {
	RunTaskInstall(context.Context, string, io.Writer) error
	RunHaftInit(context.Context, string, string, io.Writer) error
	HashFile(string) (string, error)
	CaptureProjectBasis(context.Context, string) (ProjectBasisObservation, error)
	CaptureCarriers(string, string, string) (CarrierObservation, error)
	CaptureApplication(context.Context, string) (ApplicationInstance, error)
	GracefulQuit(context.Context, string, ApplicationInstance, io.Writer) error
	WaitApplicationAbsent(context.Context, ApplicationInstance, time.Duration) error
	TerminateTaskRuntime(context.Context, TaskRuntimeIdentity, io.Writer) error
	WaitTaskRuntimeAbsent(context.Context, TaskRuntimeIdentity, time.Duration) error
	OpenExactThread(context.Context, string, time.Duration, io.Writer) (time.Time, error)
	WakeExactThread(context.Context, string, io.Writer) error
}

// Supervisor runs the fail-closed detached pipeline for exactly one prepared
// checkpoint. Resumption and post-restart verification belong to the new turn.
type Supervisor struct {
	store       Store
	effects     Effects
	log         io.Writer
	quitTimeout time.Duration
}

func NewSupervisor(
	store Store,
	effects Effects,
	log io.Writer,
	quitTimeout time.Duration,
) (Supervisor, error) {
	if effects == nil {
		return Supervisor{}, fmt.Errorf("restart supervisor effects are required")
	}
	if log == nil {
		return Supervisor{}, fmt.Errorf("restart supervisor log is required")
	}
	if quitTimeout < 2*time.Nanosecond || quitTimeout > 5*time.Minute {
		return Supervisor{}, fmt.Errorf("restart supervisor timeout must be within [2ns, 5m]")
	}
	return Supervisor{
		store:       store,
		effects:     effects,
		log:         log,
		quitTimeout: quitTimeout,
	}, nil
}

// Run consumes one prepared attempt. Any install/init/hash failure records
// install_failed and returns before the application boundary. Quit/open
// failures remain submitted and cannot auto-retry under the same checkpoint.
func (supervisor Supervisor) Run(ctx context.Context) (Checkpoint, error) {
	return supervisor.store.withSupervisorLease(func() (Checkpoint, error) {
		return supervisor.runOwned(ctx)
	})
}

func (supervisor Supervisor) runOwned(ctx context.Context) (Checkpoint, error) {
	prepared, err := supervisor.store.Load()
	if err != nil {
		return Checkpoint{}, err
	}
	if prepared.state != StatePrepared {
		return prepared, fmt.Errorf(
			"%w: checkpoint is %s rather than prepared",
			ErrLoopGuard,
			prepared.state.String(),
		)
	}
	supervisor.logEvent("supervisor_started", "prepared checkpoint acquired")
	submission, err := MarkSubmitted(prepared)
	if err != nil {
		return prepared, err
	}
	if err := supervisor.store.Apply(submission); err != nil {
		return prepared, err
	}
	submitted := submission.After()

	oldApplication, err := supervisor.effects.CaptureApplication(
		ctx,
		codexApplicationBundleID,
	)
	if err != nil {
		return submitted, fmt.Errorf("capture old Codex process: %w", err)
	}
	if len(oldApplication.Processes) == 0 {
		return submitted, fmt.Errorf("capture old Codex process: no process identity")
	}
	supervisor.logEvent(
		"old_application_captured",
		fmt.Sprintf("process_count=%d", len(oldApplication.Processes)),
	)

	if err := supervisor.effects.RunTaskInstall(ctx, submitted.repositoryRoot, supervisor.log); err != nil {
		return supervisor.recordInstallFailure(submitted, "task install", err)
	}
	supervisor.logEvent("task_install_succeeded", submitted.expectedHaftBinaryPath)
	if err := supervisor.effects.RunHaftInit(
		ctx,
		submitted.expectedHaftBinaryPath,
		submitted.repositoryRoot,
		supervisor.log,
	); err != nil {
		return supervisor.recordInstallFailure(submitted, "haft init --codex", err)
	}
	supervisor.logEvent("haft_init_succeeded", submitted.repositoryRoot)
	installedDigest, err := supervisor.effects.HashFile(submitted.expectedHaftBinaryPath)
	if err != nil {
		return supervisor.recordInstallFailure(submitted, "hash installed Haft", err)
	}
	if installedDigest != submitted.desiredHaftBinaryDigest {
		return supervisor.recordInstallFailure(
			submitted,
			"verify installed Haft digest",
			fmt.Errorf("installed Haft digest differs from checkpoint"),
		)
	}
	projectBasis, err := supervisor.effects.CaptureProjectBasis(ctx, submitted.repositoryRoot)
	if err != nil {
		return supervisor.recordInstallFailure(submitted, "capture post-init project basis", err)
	}
	carriers, err := supervisor.effects.CaptureCarriers(
		submitted.expectedSkillCarriersRoot,
		submitted.expectedInstructionPath,
		submitted.expectedMCPConfigPath,
	)
	if err != nil {
		return supervisor.recordInstallFailure(submitted, "capture post-init Codex carriers", err)
	}
	permit, err := AuthorizeQuit(submitted, InstallObservation{
		TaskInstallSucceeded: true,
		HaftInitSucceeded:    true,
		InstalledHaftPath:    submitted.expectedHaftBinaryPath,
		InstalledHaftDigest:  installedDigest,
		ProjectBasis:         projectBasis,
		Carriers:             carriers,
	})
	if err != nil {
		return supervisor.recordInstallFailure(submitted, "verify installed Haft", err)
	}
	supervisor.logEvent("pre_quit_verified", installedDigest)
	if err := supervisor.store.InstallLiveMCPChallenge(submitted); err != nil {
		return supervisor.recordInstallFailure(submitted, "install live MCP challenge", err)
	}
	supervisor.logEvent("live_mcp_challenge_installed", submitted.liveMCPChallengeNonce)

	if err := supervisor.effects.GracefulQuit(
		ctx,
		codexApplicationBundleID,
		oldApplication,
		supervisor.log,
	); err != nil {
		supervisor.logEvent("graceful_quit_failed", err.Error())
		return submitted, fmt.Errorf("%w: %v", ErrGracefulQuitFailed, err)
	}
	if err := supervisor.effects.WaitApplicationAbsent(ctx, oldApplication, supervisor.quitTimeout); err != nil {
		supervisor.logEvent("graceful_quit_timeout", err.Error())
		return submitted, fmt.Errorf("%w: %v", ErrGracefulQuitTimeout, err)
	}
	supervisor.logEvent("old_application_absent", codexApplicationBundleID)

	if err := supervisor.effects.TerminateTaskRuntime(
		ctx,
		submitted.taskRuntime,
		supervisor.log,
	); err != nil {
		supervisor.logEvent("task_runtime_termination_failed", err.Error())
		return submitted, fmt.Errorf("%w: %v", ErrTaskRuntimeStopFailed, err)
	}
	if err := supervisor.effects.WaitTaskRuntimeAbsent(
		ctx,
		submitted.taskRuntime,
		supervisor.quitTimeout,
	); err != nil {
		supervisor.logEvent("task_runtime_termination_timeout", err.Error())
		return submitted, fmt.Errorf("%w: %v", ErrTaskRuntimeStopTimeout, err)
	}
	supervisor.logEvent(
		"old_task_runtime_absent",
		fmt.Sprintf("pid=%d", submitted.taskRuntime.PID),
	)

	deepLink := "codex://threads/" + submitted.threadID
	startedAt, err := supervisor.effects.OpenExactThread(
		ctx,
		deepLink,
		supervisor.quitTimeout,
		supervisor.log,
	)
	if err != nil {
		supervisor.logEvent("exact_thread_open_failed", err.Error())
		return submitted, fmt.Errorf("%w: %v", ErrExactThreadOpenFailed, err)
	}
	opening, err := MarkAppOpened(submitted, permit, AppOpenObservation{
		GracefulQuitSucceeded: true,
		OldApplicationAbsent:  true,
		OldTaskRuntimeAbsent:  true,
		NewApplicationStarted: true,
		DeepLinkOpened:        deepLink,
		ApplicationStartedAt:  startedAt,
	})
	if err != nil {
		return submitted, err
	}
	if err := supervisor.store.Apply(opening); err != nil {
		return submitted, err
	}
	opened := opening.After()
	supervisor.logEvent("app_opened", deepLink)
	return supervisor.waitForSingleResume(ctx, opened, deepLink)
}

func (supervisor Supervisor) waitForSingleResume(
	ctx context.Context,
	opened Checkpoint,
	deepLink string,
) (Checkpoint, error) {
	resumed, observed, err := supervisor.waitForResumeState(ctx, opened, supervisor.quitTimeout)
	if err != nil {
		return opened, err
	}
	wakeCount := uint8(0)
	if !observed {
		if err := supervisor.effects.WakeExactThread(ctx, deepLink, supervisor.log); err != nil {
			return opened, fmt.Errorf("%w: fallback wake failed: %v", ErrExactThreadOpenFailed, err)
		}
		wakeCount = 1
		supervisor.logEvent("resume_fallback_opened", deepLink)
		resumed, observed, err = supervisor.waitForResumeState(ctx, opened, supervisor.quitTimeout)
		if err != nil {
			return opened, err
		}
	}
	if !observed {
		return opened, fmt.Errorf("%w: no resumed lease after primary and fallback windows", ErrExactThreadResumeTimeout)
	}
	if _, err := supervisor.store.RecordResumeFallbackCleared(
		resumed,
		wakeCount,
		time.Now().UTC(),
	); err != nil {
		return resumed, err
	}
	supervisor.logEvent("resume_fallback_cleared", fmt.Sprintf("wake_count=%d", wakeCount))
	return resumed, nil
}

func (supervisor Supervisor) waitForResumeState(
	ctx context.Context,
	basis Checkpoint,
	window time.Duration,
) (Checkpoint, bool, error) {
	deadline := time.NewTimer(window)
	defer deadline.Stop()
	pollInterval := window / 2
	maximumPollInterval := 250 * time.Millisecond
	if pollInterval > maximumPollInterval {
		pollInterval = maximumPollInterval
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		current, observed, err := supervisor.observeResumeState(basis)
		if err != nil || observed {
			return current, observed, err
		}
		select {
		case <-ctx.Done():
			return basis, false, ctx.Err()
		case <-deadline.C:
			return supervisor.observeResumeState(basis)
		case <-ticker.C:
		}
	}
}

func (supervisor Supervisor) observeResumeState(
	basis Checkpoint,
) (Checkpoint, bool, error) {
	current, err := supervisor.store.Load()
	if err != nil {
		return basis, false, err
	}
	if !checkpointBasisEqual(current, basis) {
		return basis, false, ErrConcurrentUpdate
	}
	switch current.state {
	case StateAppOpened:
		return current, false, nil
	case StateResumed:
		return current, true, nil
	default:
		return basis, false, fmt.Errorf(
			"%w: checkpoint became %s while awaiting resume",
			ErrInvalidTransition,
			current.state.String(),
		)
	}
}

func (supervisor Supervisor) recordInstallFailure(
	submitted Checkpoint,
	stage string,
	cause error,
) (Checkpoint, error) {
	detail := strings.TrimSpace(stage + " failed: " + cause.Error())
	failure, transitionErr := MarkInstallFailed(submitted, detail)
	if transitionErr != nil {
		return submitted, errors.Join(cause, transitionErr)
	}
	if err := supervisor.store.Apply(failure); err != nil {
		return submitted, errors.Join(cause, err)
	}
	failed := failure.After()
	supervisor.logEvent("install_failed", detail)
	return failed, fmt.Errorf("%w: %s", ErrInstallStageFailed, detail)
}

func (supervisor Supervisor) logEvent(kind string, detail string) {
	_, _ = fmt.Fprintf(
		supervisor.log,
		"%s kind=%s detail=%q\n",
		time.Now().UTC().Format(time.RFC3339Nano),
		kind,
		detail,
	)
}
