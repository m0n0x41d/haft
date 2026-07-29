package agenthostrestart

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CheckpointEvidence derives machine observations from readable coordinates.
// Callers never supply digests or checkpoint JSON themselves.
type CheckpointEvidence interface {
	CapturePreparation(context.Context, PreparationRequest) (PreparationEvidence, error)
	CaptureRuntime(context.Context, VerificationRequest, io.Writer) (RuntimeVerification, error)
}

// PreparationRequest names human-readable restart coordinates. Every digest in
// the durable checkpoint is derived by CheckpointEvidence.
type PreparationRequest struct {
	ProjectRoot         string
	ThreadID            string
	ResumeIntent        string
	PlanPath            string
	LastCompletedItem   string
	ResumeItem          string
	MethodRunID         string
	MethodRunAbsence    string
	CandidateHaftBinary string
	InstalledHaftBinary string
	SkillCarriersRoot   string
	InstructionCarrier  string
	MCPConfigCarrier    string
}

// PreparationEvidence is the derived byte identity captured before handoff.
type PreparationEvidence struct {
	RepositoryHead              string
	DirtyStateDigest            string
	DesiredHaftBinaryDigest     string
	ExpectedFPFRevision         string
	ExpectedTypeEnvDigest       string
	ExpectedTypeEnvHeadRevision uint64
	ExpectedGraphRevision       uint64
	ExpectedSkillCarriersDigest string
	ExpectedInstructionDigest   string
	ExpectedMCPConfigDigest     string
	TaskRuntime                 TaskRuntimeIdentity
}

// VerificationRequest names runtime observations that cannot be inferred from
// the checkpoint itself. Smoke commands are executed, not trusted booleans.
type VerificationRequest struct {
	ProjectRoot            string
	ContractSmokeArguments []string
	Checkpoint             Checkpoint
	LiveMCPReceipt         LiveMCPReceipt
	FallbackReceipt        ResumeFallbackReceipt
	supervisorRemoval      supervisorRemovalObservation
}

type checkpointCommand struct {
	evidence CheckpointEvidence
	launcher OneShotLauncher
	now      func() time.Time
}

// RunCheckpointCommand is a hidden maintainer-only preparation and recovery
// surface. It is deliberately not registered in Haft's public CLI.
func RunCheckpointCommand(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	command := checkpointCommand{
		evidence: NewOSEvidence(),
		now:      time.Now,
	}
	return command.run(ctx, args, stdout, stderr)
}

func (command checkpointCommand) runSubmit(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	flags := flag.NewFlagSet("haft-restart-checkpoint submit", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectRoot := flags.String("project-root", "", "absolute initialized Haft project root")
	supervisor := flags.String("supervisor", "", "exact detached supervisor executable")
	task := flags.String("task-executable", "", "exact Task executable")
	quitTimeout := flags.Duration("quit-timeout", 0, "bounded graceful quit/start timeout")
	allowTerm := flags.Bool("allow-term", false, "explicitly allow SIGTERM after graceful quit stalls")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*projectRoot) == "" {
		_, _ = fmt.Fprintln(stderr, "--project-root is required")
		return 2
	}
	root, err := canonicalExistingDirectory(*projectRoot)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "--project-root: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*supervisor) == "" || strings.TrimSpace(*task) == "" || *quitTimeout <= 0 {
		_, _ = fmt.Fprintln(stderr, "--supervisor, --task-executable, and positive --quit-timeout are required")
		return 2
	}
	store, err := NewStore(root)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	launcher := command.launcher
	if launcher == nil {
		launcher, err = NewLaunchctlLauncher()
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
	}
	job, err := SubmitPreparedRestart(ctx, store, launcher, SubmissionRequest{
		SupervisorExecutable: *supervisor,
		TaskExecutable:       *task,
		QuitTimeout:          *quitTimeout,
		TerminationPolicy:    applicationTerminationPolicyForAllowTerm(*allowTerm),
	})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "submitted one-shot restart %s\n", job.Label())
	_, _ = fmt.Fprintf(stdout, "  executable: %s\n", job.Executable())
	for index, argument := range job.Arguments() {
		_, _ = fmt.Fprintf(stdout, "  argv[%d]: %q\n", index, argument)
	}
	return 0
}

func (command checkpointCommand) run(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	if len(args) == 0 {
		command.printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "prepare":
		return command.runPrepare(ctx, args[1:], stdout, stderr)
	case "resume":
		return command.runResume(args[1:], stdout, stderr)
	case "challenge":
		return command.runChallenge(ctx, args[1:], stdout, stderr)
	case "verify":
		return command.runVerify(ctx, args[1:], stdout, stderr)
	case "submit":
		return command.runSubmit(ctx, args[1:], stdout, stderr)
	case "show":
		return command.runShow(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown checkpoint command %q\n", args[0])
		command.printUsage(stderr)
		return 2
	}
}

func (command checkpointCommand) runPrepare(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	flags := flag.NewFlagSet("haft-restart-checkpoint prepare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	request := PreparationRequest{}
	flags.StringVar(&request.ProjectRoot, "project-root", "", "absolute initialized Haft project root")
	flags.StringVar(&request.ThreadID, "thread-id", "", "exact Codex task ID")
	flags.StringVar(&request.ResumeIntent, "resume-intent", "", "exact task continuation intent")
	flags.StringVar(&request.PlanPath, "plan-path", "", "repository-relative WorkPlan path")
	flags.StringVar(&request.LastCompletedItem, "last-completed", "", "last completed PlanItem")
	flags.StringVar(&request.ResumeItem, "resume-at", "", "PlanItem to resume")
	flags.StringVar(&request.MethodRunID, "method-run-id", "", "current MethodRun ID")
	flags.StringVar(&request.MethodRunAbsence, "method-run-absence", "", "why no MethodRun applies")
	flags.StringVar(&request.CandidateHaftBinary, "candidate-haft", "", "candidate Haft binary to install")
	flags.StringVar(&request.InstalledHaftBinary, "installed-haft", "", "expected installed Haft path")
	flags.StringVar(&request.SkillCarriersRoot, "skill-root", "", "installed Codex skill root")
	flags.StringVar(&request.InstructionCarrier, "instruction-carrier", "", "managed instruction carrier")
	flags.StringVar(&request.MCPConfigCarrier, "mcp-config", "", "Codex MCP config carrier")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	request, err := normalizePreparationRequest(request)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	evidence, err := command.evidence.CapturePreparation(ctx, request)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "capture restart preparation evidence: %v\n", err)
		return 1
	}
	store, err := NewStore(request.ProjectRoot)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	createdAt := command.now().UTC()
	restartID := deriveRestartID(createdAt, evidence.DesiredHaftBinaryDigest)
	resumeFallbackNonce, err := randomRestartNonce()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "generate resume fallback nonce: %v\n", err)
		return 1
	}
	liveMCPNonce, err := randomRestartNonce()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "generate live MCP nonce: %v\n", err)
		return 1
	}
	logPath, err := store.SupervisorLogPath(restartID)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	checkpoint, err := NewPreparedCheckpoint(Draft{
		RestartID:                   restartID,
		ThreadID:                    request.ThreadID,
		ResumeIntentDigest:          digestText(request.ResumeIntent),
		PlanPath:                    request.PlanPath,
		LastCompletedPlanItem:       request.LastCompletedItem,
		ResumePlanItem:              request.ResumeItem,
		MethodRunID:                 request.MethodRunID,
		MethodRunAbsence:            request.MethodRunAbsence,
		RepositoryRoot:              request.ProjectRoot,
		RepositoryHead:              evidence.RepositoryHead,
		DirtyStateDigest:            evidence.DirtyStateDigest,
		ExpectedHaftBinaryPath:      request.InstalledHaftBinary,
		DesiredHaftBinaryDigest:     evidence.DesiredHaftBinaryDigest,
		ExpectedFPFRevision:         evidence.ExpectedFPFRevision,
		ExpectedTypeEnvDigest:       evidence.ExpectedTypeEnvDigest,
		ExpectedTypeEnvHeadRevision: evidence.ExpectedTypeEnvHeadRevision,
		ExpectedGraphRevision:       evidence.ExpectedGraphRevision,
		ExpectedSkillCarriersRoot:   request.SkillCarriersRoot,
		ExpectedInstructionPath:     request.InstructionCarrier,
		ExpectedMCPConfigPath:       request.MCPConfigCarrier,
		ExpectedSkillCarriersDigest: evidence.ExpectedSkillCarriersDigest,
		ExpectedInstructionDigest:   evidence.ExpectedInstructionDigest,
		ExpectedMCPConfigDigest:     evidence.ExpectedMCPConfigDigest,
		TaskRuntime:                 evidence.TaskRuntime,
		ResumeFallbackNonce:         resumeFallbackNonce,
		LiveMCPChallengeNonce:       liveMCPNonce,
		LaunchdLabel:                "com.openai.codex.haft-restart." + restartID,
		SupervisorLogPath:           logPath,
		CreatedAt:                   createdAt,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "seal restart checkpoint: %v\n", err)
		return 1
	}
	if err := store.Prepare(checkpoint); err != nil {
		_, _ = fmt.Fprintf(stderr, "prepare restart checkpoint: %v\n", err)
		return 1
	}
	command.printCheckpoint(stdout, checkpoint, store.CheckpointPath())
	return 0
}

func (command checkpointCommand) runResume(
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	flags := flag.NewFlagSet("haft-restart-checkpoint resume", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectRoot := flags.String("project-root", "", "absolute initialized Haft project root")
	threadID := flags.String("thread-id", "", "exact resumed Codex task ID")
	resumeIntent := flags.String("resume-intent", "", "exact task continuation intent")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *projectRoot == "" || *threadID == "" || strings.TrimSpace(*resumeIntent) == "" {
		_, _ = fmt.Fprintln(stderr, "--project-root, --thread-id, and --resume-intent are required")
		return 2
	}
	store, checkpoint, err := loadCheckpoint(*projectRoot)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	change, err := MarkResumed(checkpoint, ResumeObservation{
		ThreadID:           *threadID,
		ResumeIntentDigest: digestText(strings.TrimSpace(*resumeIntent)),
		RepositoryRoot:     filepath.Clean(*projectRoot),
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "resume checkpoint: %v\n", err)
		return 1
	}
	if err := store.Apply(change); err != nil {
		_, _ = fmt.Fprintf(stderr, "persist resumed checkpoint: %v\n", err)
		return 1
	}
	command.printCheckpoint(stdout, change.After(), store.CheckpointPath())
	return 0
}

func (command checkpointCommand) runChallenge(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	flags := flag.NewFlagSet("haft-restart-checkpoint challenge", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectRoot := flags.String("project-root", "", "absolute initialized Haft project root")
	mcpPID := flags.Int("mcp-pid", 0, "fresh live Haft MCP process ID")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*projectRoot) == "" || *mcpPID <= 0 {
		_, _ = fmt.Fprintln(stderr, "--project-root and positive --mcp-pid are required")
		return 2
	}
	store, checkpoint, err := loadCheckpoint(*projectRoot)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	observed, err := store.BindLiveMCPChallenge(
		ctx,
		checkpoint,
		*mcpPID,
		command.now().UTC(),
	)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bind live MCP challenge: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(
		stdout,
		"bound live MCP challenge to pid=%d started=%s executable=%s cwd=%s\n",
		observed.PID,
		observed.StartedAt.Format(time.RFC3339),
		observed.ExecutablePath,
		observed.ProjectRoot,
	)
	_, _ = fmt.Fprintln(stdout, "next: call haft_query(action=\"status\") through the live Codex MCP transport")
	return 0
}

func (command checkpointCommand) runVerify(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	flags := flag.NewFlagSet("haft-restart-checkpoint verify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	request := VerificationRequest{}
	flags.StringVar(&request.ProjectRoot, "project-root", "", "absolute initialized Haft project root")
	contractArguments := argumentList{}
	flags.Var(&contractArguments, "contract-arg", "one read-only changed-contract Haft argv item; repeat in order")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	request.ContractSmokeArguments = contractArguments.values()
	request, err := normalizeVerificationRequest(request)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	store, checkpoint, err := loadCheckpoint(request.ProjectRoot)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	liveReceipt, err := store.LoadLiveMCPReceipt(checkpoint)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load live MCP receipt: %v\n", err)
		return 1
	}
	fallbackReceipt, err := store.LoadResumeFallbackReceipt(checkpoint)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load resume fallback receipt: %v\n", err)
		return 1
	}
	launcher := command.launcher
	if launcher == nil {
		launcher, err = NewLaunchctlLauncher()
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
	}
	removal, err := removeOneShot(
		ctx,
		launcher,
		checkpoint.launchdLabel,
	)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "observe detached supervisor removal: %v\n", err)
		return 1
	}
	request.Checkpoint = checkpoint
	request.LiveMCPReceipt = liveReceipt
	request.FallbackReceipt = fallbackReceipt
	request.supervisorRemoval = removal
	verification, err := command.evidence.CaptureRuntime(ctx, request, stdout)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "capture post-restart evidence: %v\n", err)
		return 1
	}
	change, err := MarkVerified(checkpoint, verification)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "verify restart checkpoint: %v\n", err)
		return 1
	}
	if err := store.Apply(change); err != nil {
		_, _ = fmt.Fprintf(stderr, "persist verified checkpoint: %v\n", err)
		return 1
	}
	command.printRuntimeVerification(stdout, verification)
	command.printCheckpoint(stdout, change.After(), store.CheckpointPath())
	return 0
}

func (command checkpointCommand) runShow(
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	flags := flag.NewFlagSet("haft-restart-checkpoint show", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectRoot := flags.String("project-root", "", "absolute initialized Haft project root")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *projectRoot == "" {
		_, _ = fmt.Fprintln(stderr, "--project-root is required")
		return 2
	}
	store, checkpoint, err := loadCheckpoint(*projectRoot)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	command.printCheckpoint(stdout, checkpoint, store.CheckpointPath())
	return 0
}

func normalizePreparationRequest(request PreparationRequest) (PreparationRequest, error) {
	if strings.TrimSpace(request.ProjectRoot) == "" {
		return PreparationRequest{}, fmt.Errorf("--project-root is required")
	}
	root, err := canonicalExistingDirectory(request.ProjectRoot)
	if err != nil {
		return PreparationRequest{}, fmt.Errorf("--project-root: %w", err)
	}
	request.ProjectRoot = root
	request.ResumeIntent = strings.TrimSpace(request.ResumeIntent)
	request.LastCompletedItem = strings.TrimSpace(request.LastCompletedItem)
	request.ResumeItem = strings.TrimSpace(request.ResumeItem)
	request.MethodRunID = strings.TrimSpace(request.MethodRunID)
	request.MethodRunAbsence = strings.TrimSpace(request.MethodRunAbsence)
	request.PlanPath = filepath.Clean(strings.TrimSpace(request.PlanPath))
	required := map[string]string{
		"--thread-id":      request.ThreadID,
		"--resume-intent":  request.ResumeIntent,
		"--plan-path":      request.PlanPath,
		"--last-completed": request.LastCompletedItem,
		"--resume-at":      request.ResumeItem,
		"--candidate-haft": request.CandidateHaftBinary,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" || value == "." {
			return PreparationRequest{}, fmt.Errorf("%s is required", name)
		}
	}
	if (request.MethodRunID == "") == (request.MethodRunAbsence == "") {
		return PreparationRequest{}, fmt.Errorf("exactly one of --method-run-id or --method-run-absence is required")
	}
	request.CandidateHaftBinary, err = canonicalExistingFile(request.CandidateHaftBinary)
	if err != nil {
		return PreparationRequest{}, fmt.Errorf("--candidate-haft: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return PreparationRequest{}, fmt.Errorf("resolve home directory: %w", err)
	}
	if request.InstalledHaftBinary == "" {
		request.InstalledHaftBinary = filepath.Join(home, ".local", "bin", "haft")
	}
	request.InstalledHaftBinary, err = filepath.Abs(request.InstalledHaftBinary)
	if err != nil {
		return PreparationRequest{}, fmt.Errorf("--installed-haft: %w", err)
	}
	request.SkillCarriersRoot = firstNonEmpty(request.SkillCarriersRoot, filepath.Join(home, ".agents", "skills"))
	request.InstructionCarrier = firstNonEmpty(request.InstructionCarrier, filepath.Join(root, "AGENTS.md"))
	request.MCPConfigCarrier = firstNonEmpty(request.MCPConfigCarrier, filepath.Join(root, ".codex", "config.toml"))
	request.SkillCarriersRoot, err = canonicalExistingDirectory(request.SkillCarriersRoot)
	if err != nil {
		return PreparationRequest{}, fmt.Errorf("--skill-root: %w", err)
	}
	request.InstructionCarrier, err = canonicalExistingFile(request.InstructionCarrier)
	if err != nil {
		return PreparationRequest{}, fmt.Errorf("--instruction-carrier: %w", err)
	}
	request.MCPConfigCarrier, err = canonicalExistingFile(request.MCPConfigCarrier)
	if err != nil {
		return PreparationRequest{}, fmt.Errorf("--mcp-config: %w", err)
	}
	return request, nil
}

func normalizeVerificationRequest(request VerificationRequest) (VerificationRequest, error) {
	if strings.TrimSpace(request.ProjectRoot) == "" {
		return VerificationRequest{}, fmt.Errorf("--project-root is required")
	}
	root, err := canonicalExistingDirectory(request.ProjectRoot)
	if err != nil {
		return VerificationRequest{}, fmt.Errorf("--project-root: %w", err)
	}
	request.ProjectRoot = root
	if len(request.ContractSmokeArguments) == 0 {
		return VerificationRequest{}, fmt.Errorf("at least one --contract-arg is required")
	}
	if err := validateReadOnlyContractArguments(request.ContractSmokeArguments); err != nil {
		return VerificationRequest{}, err
	}
	return request, nil
}

func removeOneShot(
	ctx context.Context,
	launcher OneShotLauncher,
	label string,
) (supervisorRemovalObservation, error) {
	if err := launcher.Remove(ctx, label); err != nil {
		return supervisorRemovalObservation{}, err
	}
	exists, err := launcher.Exists(ctx, label)
	if err != nil {
		return supervisorRemovalObservation{}, err
	}
	if exists {
		return supervisorRemovalObservation{}, fmt.Errorf(
			"launchd label %s remained after bootout",
			label,
		)
	}
	return observedSupervisorRemoval(label), nil
}

func loadCheckpoint(projectRoot string) (Store, Checkpoint, error) {
	store, err := NewStore(projectRoot)
	if err != nil {
		return Store{}, Checkpoint{}, err
	}
	checkpoint, err := store.Load()
	if err != nil {
		return Store{}, Checkpoint{}, err
	}
	return store, checkpoint, nil
}

func (command checkpointCommand) printCheckpoint(
	output io.Writer,
	checkpoint Checkpoint,
	path string,
) {
	_, _ = fmt.Fprintf(output, "restart checkpoint %s\n", checkpoint.restartID)
	_, _ = fmt.Fprintf(output, "  state: %s\n", checkpoint.state.String())
	_, _ = fmt.Fprintf(output, "  task: %s\n", checkpoint.threadID)
	_, _ = fmt.Fprintf(output, "  resume: %s\n", checkpoint.resumePlanItem)
	_, _ = fmt.Fprintf(output, "  candidate: %s (%s)\n", checkpoint.expectedHaftBinaryPath, checkpoint.desiredHaftBinaryDigest)
	_, _ = fmt.Fprintf(output, "  repository: %s @ %s\n", checkpoint.repositoryRoot, checkpoint.repositoryHead)
	_, _ = fmt.Fprintf(output, "  FPF: %s\n", checkpoint.expectedFPFRevision)
	_, _ = fmt.Fprintf(output, "  TypeEnv: %s (head revision %d)\n", checkpoint.expectedTypeEnvDigest, checkpoint.expectedTypeEnvHeadRevision)
	_, _ = fmt.Fprintf(output, "  graph revision: %d\n", checkpoint.expectedGraphRevision)
	_, _ = fmt.Fprintf(output, "  checkpoint: %s\n", path)
	_, _ = fmt.Fprintf(output, "  supervisor log: %s\n", checkpoint.supervisorLogPath)
}

func (command checkpointCommand) printRuntimeVerification(
	output io.Writer,
	verification RuntimeVerification,
) {
	_, _ = fmt.Fprintf(
		output,
		"live MCP receipt: pid=%d started=%s executable=%s digest=%s cwd=%s\n",
		verification.LiveMCPReceipt.PID,
		verification.LiveMCPReceipt.ProcessStartedAt.Format(time.RFC3339),
		verification.LiveMCPReceipt.ExecutablePath,
		verification.LiveMCPReceipt.ExecutableDigest,
		verification.LiveMCPReceipt.ProjectRoot,
	)
	_, _ = fmt.Fprintf(
		output,
		"frozen basis: repository=%s dirty=%s FPF=%s TypeEnv=%s head=%d graph=%d\n",
		verification.ProjectBasis.RepositoryHead,
		verification.ProjectBasis.DirtyStateDigest,
		verification.ProjectBasis.FPFRevision,
		verification.ProjectBasis.TypeEnvDigest,
		verification.ProjectBasis.TypeEnvHeadRevision,
		verification.ProjectBasis.GraphRevision,
	)
	_, _ = fmt.Fprintf(
		output,
		"fallback receipt: wake_count=%d cleared=%s launchd_removed=%t\n",
		verification.FallbackReceipt.WakeCount,
		verification.FallbackReceipt.ClearedAt.Format(time.RFC3339Nano),
		verification.supervisorRemoval.observed(),
	)
}

func (command checkpointCommand) printUsage(output io.Writer) {
	_, _ = fmt.Fprintln(output, "usage: haft-restart-checkpoint <prepare|submit|resume|challenge|verify|show> [flags]")
	_, _ = fmt.Fprintln(output, "hidden maintainer acceptance tool; only explicit submit contacts launchd")
}

type argumentList struct {
	items []string
}

func (list *argumentList) String() string {
	return strings.Join(list.items, " ")
}

func (list *argumentList) Set(value string) error {
	if strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("argv item contains NUL")
	}
	list.items = append(list.items, value)
	return nil
}

func (list argumentList) values() []string {
	return append([]string(nil), list.items...)
}

func validateReadOnlyContractArguments(arguments []string) error {
	if arguments[0] == "interface" {
		return nil
	}
	if len(arguments) >= 2 && arguments[0] == "memory" && arguments[1] == "validate" {
		return nil
	}
	return fmt.Errorf("--contract-arg must select read-only `interface ...` or `memory validate ...`")
}

func deriveRestartID(createdAt time.Time, digest string) string {
	suffix := strings.TrimPrefix(digest, "sha256:")
	if len(suffix) > 12 {
		suffix = suffix[:12]
	}
	return "v9-" + createdAt.UTC().Format("20060102T150405") + "-" + suffix
}

func digestText(value string) string {
	digest := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func firstNonEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func canonicalExistingDirectory(raw string) (string, error) {
	absolute, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	physical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(physical)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%s is not a real directory", raw)
	}
	return filepath.Clean(physical), nil
}

func canonicalExistingFile(raw string) (string, error) {
	absolute, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	physical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(physical)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%s is not a real regular file", raw)
	}
	return filepath.Clean(physical), nil
}
