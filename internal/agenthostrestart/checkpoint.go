// Package agenthostrestart defines the durable, fail-closed handoff used to
// prove one installed Haft runtime across a Codex host restart.
//
// It is an acceptance mechanism for Haft itself. It is not a public Haft
// workflow, a project-memory artifact, or authority to cross a human gate.
package agenthostrestart

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	MaximumCheckpointBytes = 64 << 10
	maximumFieldBytes      = 4 << 10
	checkpointAttempt      = 1
)

var (
	exactSHA256Digest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	exactGitRevision  = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
	exactThreadID     = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	exactToken        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)
)

var (
	ErrCheckpointNotFound    = errors.New("restart checkpoint not found")
	ErrLoopGuard             = errors.New("restart loop guard rejected preparation")
	ErrAlreadyVerified       = errors.New("desired Haft binary is already verified")
	ErrConcurrentUpdate      = errors.New("restart checkpoint changed concurrently")
	ErrInvalidTransition     = errors.New("restart checkpoint transition is invalid")
	ErrPreQuitDenied         = errors.New("pre-quit verification denied")
	ErrDuplicateResumeWriter = errors.New("another resumed turn already owns the restart")
	ErrPredecessorCheckpoint = errors.New("restart checkpoint uses a predecessor schema")
)

// PredecessorCheckpointError identifies only the explicitly supported
// goal-coupled predecessor fields. Current checkpoint decoding remains strict
// for every other unknown field, and predecessor bytes are never promoted into
// current restart authority.
type PredecessorCheckpointError struct {
	Fields []string
}

func (err PredecessorCheckpointError) Error() string {
	return fmt.Sprintf(
		"%s: deprecated fields %s",
		ErrPredecessorCheckpoint,
		strings.Join(err.Fields, ", "),
	)
}

func (PredecessorCheckpointError) Unwrap() error {
	return ErrPredecessorCheckpoint
}

// State is the closed durable state of one restart handoff.
type State uint8

const (
	StatePrepared State = iota + 1
	StateSubmitted
	StateInstallFailed
	StateAppOpened
	StateResumed
	StateVerified
)

func (state State) String() string {
	switch state {
	case StatePrepared:
		return "prepared"
	case StateSubmitted:
		return "submitted"
	case StateInstallFailed:
		return "install_failed"
	case StateAppOpened:
		return "app_opened"
	case StateResumed:
		return "resumed"
	case StateVerified:
		return "verified"
	default:
		return ""
	}
}

func parseState(raw string) (State, error) {
	switch raw {
	case "prepared":
		return StatePrepared, nil
	case "submitted":
		return StateSubmitted, nil
	case "install_failed":
		return StateInstallFailed, nil
	case "app_opened":
		return StateAppOpened, nil
	case "resumed":
		return StateResumed, nil
	case "verified":
		return StateVerified, nil
	default:
		return 0, fmt.Errorf("restart checkpoint state %q is not defined", raw)
	}
}

// Draft supplies the exact pre-restart observations. NewPreparedCheckpoint
// derives attempt=1 and state=prepared; callers cannot choose either value.
type Draft struct {
	RestartID                   string
	ThreadID                    string
	ResumeIntentDigest          string
	PlanPath                    string
	LastCompletedPlanItem       string
	ResumePlanItem              string
	MethodRunID                 string
	MethodRunAbsence            string
	RepositoryRoot              string
	RepositoryHead              string
	DirtyStateDigest            string
	ExpectedHaftBinaryPath      string
	DesiredHaftBinaryDigest     string
	ExpectedFPFRevision         string
	ExpectedTypeEnvDigest       string
	ExpectedTypeEnvHeadRevision uint64
	ExpectedGraphRevision       uint64
	ExpectedSkillCarriersRoot   string
	ExpectedInstructionPath     string
	ExpectedMCPConfigPath       string
	ExpectedSkillCarriersDigest string
	ExpectedInstructionDigest   string
	ExpectedMCPConfigDigest     string
	TaskRuntime                 TaskRuntimeIdentity
	ResumeFallbackNonce         string
	LiveMCPChallengeNonce       string
	LaunchdLabel                string
	SupervisorLogPath           string
	CreatedAt                   time.Time
}

// Checkpoint is immutable outside this package. State changes are returned as
// new values by the closed transition functions in supervisor.go.
type Checkpoint struct {
	restartID                   string
	threadID                    string
	resumeIntentDigest          string
	planPath                    string
	lastCompletedPlanItem       string
	resumePlanItem              string
	methodRunID                 string
	methodRunAbsence            string
	repositoryRoot              string
	repositoryHead              string
	dirtyStateDigest            string
	expectedHaftBinaryPath      string
	desiredHaftBinaryDigest     string
	expectedFPFRevision         string
	expectedTypeEnvDigest       string
	expectedTypeEnvHeadRevision uint64
	expectedGraphRevision       uint64
	expectedSkillCarriersRoot   string
	expectedInstructionPath     string
	expectedMCPConfigPath       string
	expectedSkillCarriersDigest string
	expectedInstructionDigest   string
	expectedMCPConfigDigest     string
	taskRuntime                 TaskRuntimeIdentity
	resumeFallbackNonce         string
	liveMCPChallengeNonce       string
	attempt                     uint8
	state                       State
	launchdLabel                string
	supervisorLogPath           string
	createdAt                   time.Time
	failureDetail               string
}

type checkpointWire struct {
	RestartID                   string          `json:"restart_id"`
	ThreadID                    string          `json:"thread_id"`
	ResumeIntentDigest          string          `json:"resume_intent_digest"`
	PlanPath                    string          `json:"plan_path"`
	LastCompletedPlanItem       string          `json:"last_completed_plan_item"`
	ResumePlanItem              string          `json:"resume_plan_item"`
	MethodRunID                 string          `json:"method_run_id"`
	MethodRunAbsence            string          `json:"method_run_absence,omitempty"`
	RepositoryRoot              string          `json:"repository_root"`
	RepositoryHead              string          `json:"repository_head"`
	DirtyStateDigest            string          `json:"dirty_state_digest"`
	ExpectedHaftBinaryPath      string          `json:"expected_haft_binary_path"`
	DesiredHaftBinaryDigest     string          `json:"desired_haft_binary_digest"`
	ExpectedFPFRevision         string          `json:"expected_fpf_revision"`
	ExpectedTypeEnvDigest       string          `json:"expected_type_env_digest"`
	ExpectedTypeEnvHeadRevision uint64          `json:"expected_type_env_head_revision"`
	ExpectedGraphRevision       uint64          `json:"expected_graph_revision"`
	ExpectedSkillCarriersRoot   string          `json:"expected_skill_carriers_root"`
	ExpectedInstructionPath     string          `json:"expected_instruction_carrier_path"`
	ExpectedMCPConfigPath       string          `json:"expected_mcp_config_path"`
	ExpectedSkillCarriersDigest string          `json:"expected_skill_carriers_digest"`
	ExpectedInstructionDigest   string          `json:"expected_instruction_carriers_digest"`
	ExpectedMCPConfigDigest     string          `json:"expected_mcp_config_digest"`
	TaskRuntime                 taskRuntimeWire `json:"task_runtime"`
	ResumeFallbackNonce         string          `json:"resume_fallback_nonce"`
	LiveMCPChallengeNonce       string          `json:"live_mcp_challenge_nonce"`
	Attempt                     uint8           `json:"attempt"`
	State                       string          `json:"state"`
	LaunchdLabel                string          `json:"launchd_label"`
	SupervisorLogPath           string          `json:"supervisor_log_path"`
	CreatedAt                   string          `json:"created_at"`
	FailureDetail               string          `json:"failure_detail,omitempty"`
	GoalObjectiveDigest         *string         `json:"goal_objective_digest,omitempty"`
	GoalResumeCount             *uint8          `json:"goal_resume_count,omitempty"`
}

type taskRuntimeWire struct {
	PID             int    `json:"pid"`
	StartedAt       string `json:"started_at"`
	ExecutablePath  string `json:"executable_path"`
	ArgumentsDigest string `json:"arguments_digest"`
}

// NewPreparedCheckpoint validates one complete restart basis and seals its
// initial state. An absent MethodRun must be represented by an explicit
// MethodRunAbsence rather than an invented identifier.
func NewPreparedCheckpoint(draft Draft) (Checkpoint, error) {
	checkpoint := Checkpoint{
		restartID:                   draft.RestartID,
		threadID:                    draft.ThreadID,
		resumeIntentDigest:          draft.ResumeIntentDigest,
		planPath:                    draft.PlanPath,
		lastCompletedPlanItem:       draft.LastCompletedPlanItem,
		resumePlanItem:              draft.ResumePlanItem,
		methodRunID:                 draft.MethodRunID,
		methodRunAbsence:            draft.MethodRunAbsence,
		repositoryRoot:              filepath.Clean(draft.RepositoryRoot),
		repositoryHead:              draft.RepositoryHead,
		dirtyStateDigest:            draft.DirtyStateDigest,
		expectedHaftBinaryPath:      filepath.Clean(draft.ExpectedHaftBinaryPath),
		desiredHaftBinaryDigest:     draft.DesiredHaftBinaryDigest,
		expectedFPFRevision:         draft.ExpectedFPFRevision,
		expectedTypeEnvDigest:       draft.ExpectedTypeEnvDigest,
		expectedTypeEnvHeadRevision: draft.ExpectedTypeEnvHeadRevision,
		expectedGraphRevision:       draft.ExpectedGraphRevision,
		expectedSkillCarriersRoot:   filepath.Clean(draft.ExpectedSkillCarriersRoot),
		expectedInstructionPath:     filepath.Clean(draft.ExpectedInstructionPath),
		expectedMCPConfigPath:       filepath.Clean(draft.ExpectedMCPConfigPath),
		expectedSkillCarriersDigest: draft.ExpectedSkillCarriersDigest,
		expectedInstructionDigest:   draft.ExpectedInstructionDigest,
		expectedMCPConfigDigest:     draft.ExpectedMCPConfigDigest,
		taskRuntime:                 draft.TaskRuntime,
		resumeFallbackNonce:         draft.ResumeFallbackNonce,
		liveMCPChallengeNonce:       draft.LiveMCPChallengeNonce,
		attempt:                     checkpointAttempt,
		state:                       StatePrepared,
		launchdLabel:                draft.LaunchdLabel,
		supervisorLogPath:           filepath.Clean(draft.SupervisorLogPath),
		createdAt:                   draft.CreatedAt.UTC(),
	}
	if err := checkpoint.validate(); err != nil {
		return Checkpoint{}, err
	}
	return checkpoint, nil
}

// DecodeCheckpoint accepts one strict JSON object and rejects unknown fields,
// trailing values, invalid states, and non-canonical field values.
func DecodeCheckpoint(content []byte) (Checkpoint, error) {
	if len(content) == 0 {
		return Checkpoint{}, fmt.Errorf("restart checkpoint is empty")
	}
	if len(content) > MaximumCheckpointBytes {
		return Checkpoint{}, fmt.Errorf("restart checkpoint exceeds %d bytes", MaximumCheckpointBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var wire checkpointWire
	if err := decoder.Decode(&wire); err != nil {
		return Checkpoint{}, fmt.Errorf("decode restart checkpoint: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Checkpoint{}, fmt.Errorf("restart checkpoint has trailing JSON")
	}
	predecessorFields := predecessorCheckpointFields(wire)
	if len(predecessorFields) > 0 {
		return Checkpoint{}, PredecessorCheckpointError{
			Fields: predecessorFields,
		}
	}
	checkpoint, err := checkpointFromWire(wire)
	if err != nil {
		return Checkpoint{}, err
	}
	return checkpoint, nil
}

func predecessorCheckpointFields(wire checkpointWire) []string {
	fields := make([]string, 0, 2)
	if wire.GoalObjectiveDigest != nil {
		fields = append(fields, "goal_objective_digest")
	}
	if wire.GoalResumeCount != nil {
		fields = append(fields, "goal_resume_count")
	}
	return fields
}

func checkpointFromWire(wire checkpointWire) (Checkpoint, error) {
	state, err := parseState(wire.State)
	if err != nil {
		return Checkpoint{}, err
	}
	createdAt, err := time.Parse(time.RFC3339Nano, wire.CreatedAt)
	if err != nil {
		return Checkpoint{}, fmt.Errorf("restart checkpoint created_at is invalid: %w", err)
	}
	taskRuntimeStartedAt, err := time.Parse(time.RFC3339Nano, wire.TaskRuntime.StartedAt)
	if err != nil {
		return Checkpoint{}, fmt.Errorf("restart checkpoint task_runtime.started_at is invalid: %w", err)
	}
	checkpoint := Checkpoint{
		restartID:                   wire.RestartID,
		threadID:                    wire.ThreadID,
		resumeIntentDigest:          wire.ResumeIntentDigest,
		planPath:                    wire.PlanPath,
		lastCompletedPlanItem:       wire.LastCompletedPlanItem,
		resumePlanItem:              wire.ResumePlanItem,
		methodRunID:                 wire.MethodRunID,
		methodRunAbsence:            wire.MethodRunAbsence,
		repositoryRoot:              wire.RepositoryRoot,
		repositoryHead:              wire.RepositoryHead,
		dirtyStateDigest:            wire.DirtyStateDigest,
		expectedHaftBinaryPath:      wire.ExpectedHaftBinaryPath,
		desiredHaftBinaryDigest:     wire.DesiredHaftBinaryDigest,
		expectedFPFRevision:         wire.ExpectedFPFRevision,
		expectedTypeEnvDigest:       wire.ExpectedTypeEnvDigest,
		expectedTypeEnvHeadRevision: wire.ExpectedTypeEnvHeadRevision,
		expectedGraphRevision:       wire.ExpectedGraphRevision,
		expectedSkillCarriersRoot:   wire.ExpectedSkillCarriersRoot,
		expectedInstructionPath:     wire.ExpectedInstructionPath,
		expectedMCPConfigPath:       wire.ExpectedMCPConfigPath,
		expectedSkillCarriersDigest: wire.ExpectedSkillCarriersDigest,
		expectedInstructionDigest:   wire.ExpectedInstructionDigest,
		expectedMCPConfigDigest:     wire.ExpectedMCPConfigDigest,
		taskRuntime: TaskRuntimeIdentity{
			PID:             wire.TaskRuntime.PID,
			StartedAt:       taskRuntimeStartedAt.UTC(),
			ExecutablePath:  wire.TaskRuntime.ExecutablePath,
			ArgumentsDigest: wire.TaskRuntime.ArgumentsDigest,
		},
		resumeFallbackNonce:   wire.ResumeFallbackNonce,
		liveMCPChallengeNonce: wire.LiveMCPChallengeNonce,
		attempt:               wire.Attempt,
		state:                 state,
		launchdLabel:          wire.LaunchdLabel,
		supervisorLogPath:     wire.SupervisorLogPath,
		createdAt:             createdAt.UTC(),
		failureDetail:         wire.FailureDetail,
	}
	if err := checkpoint.validate(); err != nil {
		return Checkpoint{}, err
	}
	return checkpoint, nil
}

// CanonicalBytes returns deterministic JSON with one terminal newline.
func (checkpoint Checkpoint) CanonicalBytes() ([]byte, error) {
	if err := checkpoint.validate(); err != nil {
		return nil, err
	}
	wire := checkpoint.toWire()
	content, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode restart checkpoint: %w", err)
	}
	return append(content, '\n'), nil
}

// Digest authenticates the exact canonical checkpoint bytes.
func (checkpoint Checkpoint) Digest() (string, error) {
	content, err := checkpoint.CanonicalBytes()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

// BasisDigest authenticates the immutable restart basis independently of its
// current state. Challenge and fallback receipts use it across transitions.
func (checkpoint Checkpoint) BasisDigest() (string, error) {
	checkpoint.state = StatePrepared
	checkpoint.failureDetail = ""
	return checkpoint.Digest()
}

func (checkpoint Checkpoint) RestartID() string { return checkpoint.restartID }
func (checkpoint Checkpoint) ThreadID() string  { return checkpoint.threadID }
func (checkpoint Checkpoint) State() State      { return checkpoint.state }
func (checkpoint Checkpoint) Attempt() uint8    { return checkpoint.attempt }
func (checkpoint Checkpoint) DesiredHaftBinaryDigest() string {
	return checkpoint.desiredHaftBinaryDigest
}
func (checkpoint Checkpoint) RepositoryRoot() string    { return checkpoint.repositoryRoot }
func (checkpoint Checkpoint) SupervisorLogPath() string { return checkpoint.supervisorLogPath }

func (checkpoint Checkpoint) toWire() checkpointWire {
	return checkpointWire{
		RestartID:                   checkpoint.restartID,
		ThreadID:                    checkpoint.threadID,
		ResumeIntentDigest:          checkpoint.resumeIntentDigest,
		PlanPath:                    checkpoint.planPath,
		LastCompletedPlanItem:       checkpoint.lastCompletedPlanItem,
		ResumePlanItem:              checkpoint.resumePlanItem,
		MethodRunID:                 checkpoint.methodRunID,
		MethodRunAbsence:            checkpoint.methodRunAbsence,
		RepositoryRoot:              checkpoint.repositoryRoot,
		RepositoryHead:              checkpoint.repositoryHead,
		DirtyStateDigest:            checkpoint.dirtyStateDigest,
		ExpectedHaftBinaryPath:      checkpoint.expectedHaftBinaryPath,
		DesiredHaftBinaryDigest:     checkpoint.desiredHaftBinaryDigest,
		ExpectedFPFRevision:         checkpoint.expectedFPFRevision,
		ExpectedTypeEnvDigest:       checkpoint.expectedTypeEnvDigest,
		ExpectedTypeEnvHeadRevision: checkpoint.expectedTypeEnvHeadRevision,
		ExpectedGraphRevision:       checkpoint.expectedGraphRevision,
		ExpectedSkillCarriersRoot:   checkpoint.expectedSkillCarriersRoot,
		ExpectedInstructionPath:     checkpoint.expectedInstructionPath,
		ExpectedMCPConfigPath:       checkpoint.expectedMCPConfigPath,
		ExpectedSkillCarriersDigest: checkpoint.expectedSkillCarriersDigest,
		ExpectedInstructionDigest:   checkpoint.expectedInstructionDigest,
		ExpectedMCPConfigDigest:     checkpoint.expectedMCPConfigDigest,
		TaskRuntime: taskRuntimeWire{
			PID:             checkpoint.taskRuntime.PID,
			StartedAt:       checkpoint.taskRuntime.StartedAt.Format(time.RFC3339Nano),
			ExecutablePath:  checkpoint.taskRuntime.ExecutablePath,
			ArgumentsDigest: checkpoint.taskRuntime.ArgumentsDigest,
		},
		ResumeFallbackNonce:   checkpoint.resumeFallbackNonce,
		LiveMCPChallengeNonce: checkpoint.liveMCPChallengeNonce,
		Attempt:               checkpoint.attempt,
		State:                 checkpoint.state.String(),
		LaunchdLabel:          checkpoint.launchdLabel,
		SupervisorLogPath:     checkpoint.supervisorLogPath,
		CreatedAt:             checkpoint.createdAt.Format(time.RFC3339Nano),
		FailureDetail:         checkpoint.failureDetail,
	}
}

func (checkpoint Checkpoint) validate() error {
	validations := []func(Checkpoint) error{
		validateCheckpointIdentity,
		validateCheckpointCoordinates,
		validateCheckpointDigests,
		validateCheckpointAuthorityBoundary,
		validateCheckpointState,
	}
	for _, validate := range validations {
		if err := validate(checkpoint); err != nil {
			return err
		}
	}
	return nil
}

func validateCheckpointIdentity(checkpoint Checkpoint) error {
	if !exactToken.MatchString(checkpoint.restartID) {
		return fmt.Errorf("restart_id is invalid")
	}
	if !exactThreadID.MatchString(checkpoint.threadID) {
		return fmt.Errorf("thread_id is invalid")
	}
	if checkpoint.taskRuntime.PID <= 1 {
		return fmt.Errorf("task_runtime.pid must identify a non-init process")
	}
	if !exactToken.MatchString(checkpoint.launchdLabel) {
		return fmt.Errorf("launchd_label is invalid")
	}
	if !strings.HasPrefix(checkpoint.launchdLabel, "com.openai.codex.haft-restart.") {
		return fmt.Errorf("launchd_label is outside the Haft restart namespace")
	}
	if !exactSHA256Digest.MatchString(checkpoint.resumeFallbackNonce) {
		return fmt.Errorf("resume_fallback_nonce is invalid")
	}
	if !exactSHA256Digest.MatchString(checkpoint.liveMCPChallengeNonce) {
		return fmt.Errorf("live_mcp_challenge_nonce is invalid")
	}
	return nil
}

func validateCheckpointCoordinates(checkpoint Checkpoint) error {
	fields := map[string]string{
		"last_completed_plan_item": checkpoint.lastCompletedPlanItem,
		"resume_plan_item":         checkpoint.resumePlanItem,
	}
	for name, value := range fields {
		if err := validateText(name, value); err != nil {
			return err
		}
	}
	if filepath.IsAbs(checkpoint.planPath) {
		return fmt.Errorf("plan_path must be repository-relative")
	}
	if filepath.Clean(checkpoint.planPath) != checkpoint.planPath {
		return fmt.Errorf("plan_path is not clean")
	}
	if checkpoint.planPath == "." || strings.HasPrefix(checkpoint.planPath, "..") {
		return fmt.Errorf("plan_path escapes the repository")
	}
	if !filepath.IsAbs(checkpoint.repositoryRoot) || checkpoint.repositoryRoot == string(filepath.Separator) {
		return fmt.Errorf("repository_root must be an absolute non-root path")
	}
	if !filepath.IsAbs(checkpoint.expectedHaftBinaryPath) {
		return fmt.Errorf("expected_haft_binary_path must be absolute")
	}
	if !filepath.IsAbs(checkpoint.taskRuntime.ExecutablePath) ||
		filepath.Clean(checkpoint.taskRuntime.ExecutablePath) != checkpoint.taskRuntime.ExecutablePath {
		return fmt.Errorf("task_runtime.executable_path must be a clean absolute path")
	}
	paths := map[string]string{
		"expected_skill_carriers_root":      checkpoint.expectedSkillCarriersRoot,
		"expected_instruction_carrier_path": checkpoint.expectedInstructionPath,
		"expected_mcp_config_path":          checkpoint.expectedMCPConfigPath,
	}
	for name, path := range paths {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return fmt.Errorf("%s must be a clean absolute path", name)
		}
	}
	if !filepath.IsAbs(checkpoint.supervisorLogPath) {
		return fmt.Errorf("supervisor_log_path must be absolute")
	}
	expectedLogDirectory := filepath.Join(checkpoint.repositoryRoot, ".haft", "restart")
	if filepath.Dir(checkpoint.supervisorLogPath) != expectedLogDirectory {
		return fmt.Errorf("supervisor_log_path must stay inside .haft/restart")
	}
	if err := validateMethodRun(checkpoint); err != nil {
		return err
	}
	return nil
}

func validateMethodRun(checkpoint Checkpoint) error {
	hasID := strings.TrimSpace(checkpoint.methodRunID) != ""
	hasAbsence := strings.TrimSpace(checkpoint.methodRunAbsence) != ""
	if hasID == hasAbsence {
		return fmt.Errorf("exactly one of method_run_id or method_run_absence is required")
	}
	if hasID {
		return validateText("method_run_id", checkpoint.methodRunID)
	}
	return validateText("method_run_absence", checkpoint.methodRunAbsence)
}

func validateCheckpointDigests(checkpoint Checkpoint) error {
	digests := map[string]string{
		"resume_intent_digest":                 checkpoint.resumeIntentDigest,
		"dirty_state_digest":                   checkpoint.dirtyStateDigest,
		"desired_haft_binary_digest":           checkpoint.desiredHaftBinaryDigest,
		"expected_type_env_digest":             checkpoint.expectedTypeEnvDigest,
		"expected_skill_carriers_digest":       checkpoint.expectedSkillCarriersDigest,
		"expected_instruction_carriers_digest": checkpoint.expectedInstructionDigest,
		"expected_mcp_config_digest":           checkpoint.expectedMCPConfigDigest,
		"task_runtime.arguments_digest":        checkpoint.taskRuntime.ArgumentsDigest,
	}
	for name, value := range digests {
		if !exactSHA256Digest.MatchString(value) {
			return fmt.Errorf("%s is not an exact sha256 digest", name)
		}
	}
	if !exactGitRevision.MatchString(checkpoint.repositoryHead) {
		return fmt.Errorf("repository_head is not an exact git revision")
	}
	if !exactGitRevision.MatchString(checkpoint.expectedFPFRevision) {
		return fmt.Errorf("expected_fpf_revision is not an exact git revision")
	}
	if checkpoint.expectedTypeEnvHeadRevision == 0 {
		return fmt.Errorf("expected_type_env_head_revision must be positive")
	}
	return nil
}

func validateCheckpointAuthorityBoundary(checkpoint Checkpoint) error {
	if checkpoint.attempt != checkpointAttempt {
		return fmt.Errorf("restart attempt must be exactly 1")
	}
	if checkpoint.createdAt.IsZero() {
		return fmt.Errorf("created_at is required")
	}
	if checkpoint.createdAt.Location() != time.UTC {
		return fmt.Errorf("created_at must be normalized to UTC")
	}
	if checkpoint.taskRuntime.StartedAt.IsZero() ||
		checkpoint.taskRuntime.StartedAt.Location() != time.UTC {
		return fmt.Errorf("task_runtime.started_at must be normalized to UTC")
	}
	if checkpoint.taskRuntime.StartedAt.After(checkpoint.createdAt) {
		return fmt.Errorf("task_runtime.started_at cannot follow checkpoint creation")
	}
	return nil
}

func validateCheckpointState(checkpoint Checkpoint) error {
	if checkpoint.state.String() == "" {
		return fmt.Errorf("restart checkpoint state is invalid")
	}
	if checkpoint.state == StateInstallFailed {
		return validateText("failure_detail", checkpoint.failureDetail)
	}
	if checkpoint.failureDetail != "" {
		return fmt.Errorf("failure_detail is allowed only for install_failed")
	}
	return nil
}

func validateText(name string, value string) error {
	if strings.TrimSpace(value) != value || value == "" {
		return fmt.Errorf("%s is empty or not normalized", name)
	}
	if !utf8.ValidString(value) || len(value) > maximumFieldBytes {
		return fmt.Errorf("%s is invalid or exceeds %d bytes", name, maximumFieldBytes)
	}
	for _, runeValue := range value {
		if unicode.IsControl(runeValue) {
			return fmt.Errorf("%s contains control characters", name)
		}
	}
	return nil
}

func checkpointBasisEqual(left Checkpoint, right Checkpoint) bool {
	left.state = StatePrepared
	left.failureDetail = ""
	right.state = StatePrepared
	right.failureDetail = ""
	return left == right
}
