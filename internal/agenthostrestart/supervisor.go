package agenthostrestart

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// InstallObservation is the complete pre-quit evidence. The current app must
// remain alive unless AuthorizeQuit returns a permit.
type InstallObservation struct {
	TaskInstallSucceeded bool
	HaftInitSucceeded    bool
	InstalledHaftPath    string
	InstalledHaftDigest  string
	ProjectBasis         ProjectBasisObservation
	Carriers             CarrierObservation
}

// ProjectBasisObservation is the exact frozen repository and selected-memory
// basis. Both pre-quit and resumed verification must reproduce it.
type ProjectBasisObservation struct {
	RepositoryHead      string
	DirtyStateDigest    string
	FPFRevision         string
	TypeEnvDigest       string
	TypeEnvHeadRevision uint64
	GraphRevision       uint64
}

// CarrierObservation binds the exact installed Codex carrier locations and
// bytes. Paths are part of the proof so equal bytes at another location do not
// satisfy the handoff.
type CarrierObservation struct {
	SkillCarriersRoot string
	SkillDigest       string
	InstructionPath   string
	InstructionDigest string
	MCPConfigPath     string
	MCPConfigDigest   string
}

// QuitPermit can only be constructed from a submitted checkpoint and matching
// install/init evidence. It is intentionally not serializable.
type QuitPermit struct {
	restartID  string
	digest     string
	checkpoint string
}

// Change is a closed state transition. Its fields are private so callers
// cannot manufacture app_opened or verified without the corresponding proof.
type Change struct {
	before Checkpoint
	after  Checkpoint
}

func (change Change) Before() Checkpoint { return change.before }
func (change Change) After() Checkpoint  { return change.after }

// AppOpenObservation records facts observed after the pre-quit permit was
// consumed by the detached supervisor.
type AppOpenObservation struct {
	GracefulQuitSucceeded bool
	OldApplicationAbsent  bool
	OldTaskRuntimeAbsent  bool
	NewApplicationStarted bool
	DeepLinkOpened        string
	ApplicationStartedAt  time.Time
}

// ResumeObservation binds one explicit exact-task continuation to the original
// task, project, and checkpointed intent without creating a Goal.
type ResumeObservation struct {
	ThreadID           string
	ResumeIntentDigest string
	RepositoryRoot     string
}

// RuntimeVerification contains only post-restart observations. Pre-restart
// command output is not an admissible source for this proof.
type RuntimeVerification struct {
	CLIExecutablePath          string
	CLIExecutableDigest        string
	ProjectBasis               ProjectBasisObservation
	Carriers                   CarrierObservation
	LiveMCPReceipt             LiveMCPReceipt
	FallbackReceipt            ResumeFallbackReceipt
	ChangedContractSmokePassed bool
	supervisorRemoval          supervisorRemovalObservation
}

// supervisorRemovalObservation is package-owned proof that the detached
// launchd label was observed absent. Its zero value cannot satisfy verification.
type supervisorRemovalObservation struct {
	launchdLabel string
}

func observedSupervisorRemoval(launchdLabel string) supervisorRemovalObservation {
	return supervisorRemovalObservation{launchdLabel: launchdLabel}
}

func (observation supervisorRemovalObservation) observed() bool {
	return observation.launchdLabel != ""
}

func (observation supervisorRemovalObservation) matches(launchdLabel string) bool {
	return observation.observed() && observation.launchdLabel == launchdLabel
}

// MarkSubmitted records successful handoff to the detached one-shot owner.
func MarkSubmitted(checkpoint Checkpoint) (Change, error) {
	return transition(checkpoint, StatePrepared, StateSubmitted, "")
}

// MarkInstallFailed is terminal for this restart basis. A subsequent attempt
// needs a changed desired digest or fresh operator authorization outside this
// state machine.
func MarkInstallFailed(checkpoint Checkpoint, detail string) (Change, error) {
	if err := validateText("failure_detail", detail); err != nil {
		return Change{}, err
	}
	return transition(checkpoint, StateSubmitted, StateInstallFailed, detail)
}

// AuthorizeQuit is the exact fail-closed boundary before terminating the host.
func AuthorizeQuit(
	checkpoint Checkpoint,
	observation InstallObservation,
) (QuitPermit, error) {
	if checkpoint.state != StateSubmitted {
		return QuitPermit{}, fmt.Errorf("%w: pre-quit checkpoint state is %s", ErrPreQuitDenied, checkpoint.state.String())
	}
	if !observation.TaskInstallSucceeded {
		return QuitPermit{}, fmt.Errorf("%w: task install did not succeed", ErrPreQuitDenied)
	}
	if !observation.HaftInitSucceeded {
		return QuitPermit{}, fmt.Errorf("%w: haft init --codex did not succeed", ErrPreQuitDenied)
	}
	installedPath := filepath.Clean(observation.InstalledHaftPath)
	if installedPath != checkpoint.expectedHaftBinaryPath {
		return QuitPermit{}, fmt.Errorf("%w: installed Haft path differs from checkpoint", ErrPreQuitDenied)
	}
	if observation.InstalledHaftDigest != checkpoint.desiredHaftBinaryDigest {
		return QuitPermit{}, fmt.Errorf("%w: installed Haft digest differs from checkpoint", ErrPreQuitDenied)
	}
	issues := verifyProjectBasis(checkpoint, observation.ProjectBasis)
	issues = append(issues, verifyCarriers(checkpoint, observation.Carriers)...)
	if len(issues) > 0 {
		return QuitPermit{}, fmt.Errorf("%w: %s", ErrPreQuitDenied, strings.Join(issues, "; "))
	}
	digest, err := checkpoint.Digest()
	if err != nil {
		return QuitPermit{}, err
	}
	return QuitPermit{
		restartID:  checkpoint.restartID,
		digest:     checkpoint.desiredHaftBinaryDigest,
		checkpoint: digest,
	}, nil
}

// MarkAppOpened accepts only the exact deep link after a graceful bounded
// shutdown and a newly observed application process.
func MarkAppOpened(
	checkpoint Checkpoint,
	permit QuitPermit,
	observation AppOpenObservation,
) (Change, error) {
	if err := validateQuitPermit(checkpoint, permit); err != nil {
		return Change{}, err
	}
	if !observation.GracefulQuitSucceeded || !observation.OldApplicationAbsent {
		return Change{}, fmt.Errorf("%w: old application shutdown was not proven", ErrInvalidTransition)
	}
	if !observation.OldTaskRuntimeAbsent {
		return Change{}, fmt.Errorf("%w: old task runtime shutdown was not proven", ErrInvalidTransition)
	}
	if !observation.NewApplicationStarted {
		return Change{}, fmt.Errorf("%w: new application process was not observed", ErrInvalidTransition)
	}
	expectedLink := "codex://threads/" + checkpoint.threadID
	if observation.DeepLinkOpened != expectedLink {
		return Change{}, fmt.Errorf("%w: exact task deep link was not opened", ErrInvalidTransition)
	}
	startedAt := observation.ApplicationStartedAt.UTC()
	if !startedAt.After(checkpoint.createdAt) {
		return Change{}, fmt.Errorf("%w: new application process is not newer than the checkpoint", ErrInvalidTransition)
	}
	return transition(checkpoint, StateSubmitted, StateAppOpened, "")
}

// MarkResumed requires the exact persisted task, resume intent, and project.
func MarkResumed(
	checkpoint Checkpoint,
	observation ResumeObservation,
) (Change, error) {
	if observation.ThreadID != checkpoint.threadID {
		return Change{}, fmt.Errorf("%w: another task resumed", ErrInvalidTransition)
	}
	if observation.ResumeIntentDigest != checkpoint.resumeIntentDigest {
		return Change{}, fmt.Errorf("%w: resume intent digest changed", ErrInvalidTransition)
	}
	root := filepath.Clean(observation.RepositoryRoot)
	if root != checkpoint.repositoryRoot {
		return Change{}, fmt.Errorf("%w: another repository resumed", ErrInvalidTransition)
	}
	return transition(checkpoint, StateAppOpened, StateResumed, "")
}

// MarkVerified closes the handoff only after all installed identities,
// carriers, smokes, supervisor cleanup, and the single-writer condition match.
func MarkVerified(
	checkpoint Checkpoint,
	verification RuntimeVerification,
) (Change, error) {
	issues := verifyRuntime(checkpoint, verification)
	if len(issues) > 0 {
		return Change{}, fmt.Errorf("%w: %s", ErrInvalidTransition, strings.Join(issues, "; "))
	}
	return transition(checkpoint, StateResumed, StateVerified, "")
}

func verifyRuntime(
	checkpoint Checkpoint,
	verification RuntimeVerification,
) []string {
	issues := make([]string, 0)
	checks := []struct {
		matches bool
		detail  string
	}{
		{filepath.Clean(verification.CLIExecutablePath) == checkpoint.expectedHaftBinaryPath, "CLI executable path differs"},
		{verification.CLIExecutableDigest == checkpoint.desiredHaftBinaryDigest, "CLI executable digest differs"},
		{verification.LiveMCPReceipt.ExecutablePath == checkpoint.expectedHaftBinaryPath, "MCP executable path differs"},
		{verification.LiveMCPReceipt.ExecutableDigest == checkpoint.desiredHaftBinaryDigest, "MCP executable digest differs"},
		{verification.LiveMCPReceipt.ProcessStartedAt.UTC().After(checkpoint.createdAt), "MCP process predates checkpoint"},
		{verification.LiveMCPReceipt.ProjectRoot == checkpoint.repositoryRoot, "MCP process serves another project root"},
		{verification.LiveMCPReceipt.RestartID == checkpoint.restartID, "MCP receipt belongs to another restart"},
		{verification.LiveMCPReceipt.Nonce == checkpoint.liveMCPChallengeNonce, "MCP receipt nonce differs"},
		{verification.FallbackReceipt.RestartID == checkpoint.restartID, "fallback receipt belongs to another restart"},
		{verification.FallbackReceipt.Nonce == checkpoint.resumeFallbackNonce, "fallback receipt nonce differs"},
		{verification.FallbackReceipt.WakeCount <= 1, "fallback receipt exceeds one-shot wake bound"},
		{verification.FallbackReceipt.ClearedAt.After(checkpoint.createdAt), "fallback receipt predates checkpoint"},
		{verification.supervisorRemoval.matches(checkpoint.launchdLabel), "detached supervisor job remains"},
		{verification.ChangedContractSmokePassed, "changed contract smoke did not pass"},
	}
	for _, check := range checks {
		if !check.matches {
			issues = append(issues, check.detail)
		}
	}
	issues = append(issues, verifyProjectBasis(checkpoint, verification.ProjectBasis)...)
	issues = append(issues, verifyCarriers(checkpoint, verification.Carriers)...)
	basisDigest, err := checkpoint.BasisDigest()
	if err != nil {
		issues = append(issues, "checkpoint basis digest is unavailable")
		return issues
	}
	if verification.LiveMCPReceipt.CheckpointBasisDigest != basisDigest {
		issues = append(issues, "MCP receipt checkpoint basis differs")
	}
	if verification.FallbackReceipt.CheckpointBasisDigest != basisDigest {
		issues = append(issues, "fallback receipt checkpoint basis differs")
	}
	return issues
}

func verifyProjectBasis(
	checkpoint Checkpoint,
	observation ProjectBasisObservation,
) []string {
	checks := []struct {
		matches bool
		detail  string
	}{
		{observation.RepositoryHead == checkpoint.repositoryHead, "repository HEAD differs"},
		{observation.DirtyStateDigest == checkpoint.dirtyStateDigest, "dirty-state digest differs"},
		{observation.FPFRevision == checkpoint.expectedFPFRevision, "FPF revision differs"},
		{observation.TypeEnvDigest == checkpoint.expectedTypeEnvDigest, "selected TypeEnv digest differs"},
		{observation.TypeEnvHeadRevision == checkpoint.expectedTypeEnvHeadRevision, "selected TypeEnv head revision differs"},
		{observation.GraphRevision == checkpoint.expectedGraphRevision, "project graph revision differs"},
	}
	return failedChecks(checks)
}

func verifyCarriers(
	checkpoint Checkpoint,
	observation CarrierObservation,
) []string {
	checks := []struct {
		matches bool
		detail  string
	}{
		{filepath.Clean(observation.SkillCarriersRoot) == checkpoint.expectedSkillCarriersRoot, "skill carrier root differs"},
		{observation.SkillDigest == checkpoint.expectedSkillCarriersDigest, "skill carrier digest differs"},
		{filepath.Clean(observation.InstructionPath) == checkpoint.expectedInstructionPath, "instruction carrier path differs"},
		{observation.InstructionDigest == checkpoint.expectedInstructionDigest, "instruction carrier digest differs"},
		{filepath.Clean(observation.MCPConfigPath) == checkpoint.expectedMCPConfigPath, "MCP config path differs"},
		{observation.MCPConfigDigest == checkpoint.expectedMCPConfigDigest, "MCP config digest differs"},
	}
	return failedChecks(checks)
}

func failedChecks(checks []struct {
	matches bool
	detail  string
}) []string {
	issues := make([]string, 0)
	for _, check := range checks {
		if !check.matches {
			issues = append(issues, check.detail)
		}
	}
	return issues
}

func validateQuitPermit(checkpoint Checkpoint, permit QuitPermit) error {
	if checkpoint.state != StateSubmitted {
		return fmt.Errorf("%w: app-open checkpoint state is %s", ErrInvalidTransition, checkpoint.state.String())
	}
	digest, err := checkpoint.Digest()
	if err != nil {
		return err
	}
	if permit.restartID != checkpoint.restartID || permit.digest != checkpoint.desiredHaftBinaryDigest || permit.checkpoint != digest {
		return fmt.Errorf("%w: quit permit belongs to another checkpoint", ErrInvalidTransition)
	}
	return nil
}

func transition(
	checkpoint Checkpoint,
	expected State,
	next State,
	failureDetail string,
) (Change, error) {
	if checkpoint.state != expected {
		return Change{}, fmt.Errorf(
			"%w: %s cannot transition to %s",
			ErrInvalidTransition,
			checkpoint.state.String(),
			next.String(),
		)
	}
	nextCheckpoint := checkpoint
	nextCheckpoint.state = next
	nextCheckpoint.failureDetail = failureDetail
	if err := nextCheckpoint.validate(); err != nil {
		return Change{}, err
	}
	return Change{before: checkpoint, after: nextCheckpoint}, nil
}

func (change Change) valid() bool {
	return legalDurableTransition(change.before, change.after)
}

func legalDurableTransition(current Checkpoint, proposed Checkpoint) bool {
	if !checkpointBasisEqual(current, proposed) {
		return false
	}
	pair := [2]State{current.state, proposed.state}
	legal := map[[2]State]bool{
		{StatePrepared, StateSubmitted}:      true,
		{StateSubmitted, StateInstallFailed}: true,
		{StateSubmitted, StateAppOpened}:     true,
		{StateAppOpened, StateResumed}:       true,
		{StateResumed, StateVerified}:        true,
	}
	return legal[pair]
}
