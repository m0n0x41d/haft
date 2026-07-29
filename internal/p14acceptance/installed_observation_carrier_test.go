package p14acceptance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/agenthostrestart"
)

const (
	p14InstalledObservationInputSchema   = "haft.p14.installed-observation-input/v2"
	p14InstalledObservationCarrierSchema = "haft.p14.installed-observation/v2"
	p14RuntimeObservationBindingSchema   = "haft.p14.runtime-observation-binding/v1"
	p14InstalledObservationSemantics     = "Observed P14 installed surfaces bound to one sealed preparation and safe digests of private restart receipts. This carrier records runtime evidence but grants no release, specification, decision, or execution authority."

	p14ObservationStatusPassed = "passed"
	p14ObservationStatusFailed = "failed"

	p14ObservationSourceInstalledCLI = "installed_cli_execution"
	p14ObservationSourceLiveMCP      = "actual_codex_mcp_capture"
	p14ObservationSourceHostProcess  = "restart_checkpoint_verification"

	p14SurfaceOutcomeObserved       = "observed"
	p14SurfaceOutcomeExecutionError = "execution_error"
	p14SurfaceOutcomeMismatch       = "oracle_mismatch"
)

type p14InstalledObservationInput struct {
	Schema               string                            `json:"schema"`
	Status               string                            `json:"status"`
	ResultSemantics      string                            `json:"result_semantics"`
	ReleaseClaim         bool                              `json:"release_claim"`
	PreparedCarrier      p14PreparedObservationBinding     `json:"prepared_carrier"`
	Runtime              p14RuntimeObservationBinding      `json:"runtime"`
	ClaudeHostProof      p14ClaudeHostProofBinding         `json:"claude_host_proof"`
	ScenarioObservations []p14InstalledScenarioObservation `json:"scenario_observations"`
}

type p14PreparedObservationBinding struct {
	CarrierPath       string `json:"carrier_path"`
	CarrierDigest     string `json:"carrier_digest"`
	PreparationDigest string `json:"preparation_digest"`
}

type p14RuntimeObservationBinding struct {
	Schema                         string `json:"schema"`
	RestartID                      string `json:"restart_id"`
	ThreadID                       string `json:"thread_id"`
	ResumeIntentDigest             string `json:"resume_intent_digest"`
	RestartCheckpointState         string `json:"restart_checkpoint_state"`
	RestartCheckpointCreatedAt     string `json:"restart_checkpoint_created_at"`
	PreparedTaskRuntimePID         int    `json:"prepared_task_runtime_pid"`
	PreparedTaskRuntimeStartedAt   string `json:"prepared_task_runtime_started_at"`
	PreparedTaskRuntimeExecutable  string `json:"prepared_task_runtime_executable"`
	PreparedTaskRuntimeArgsDigest  string `json:"prepared_task_runtime_args_digest"`
	CodexStateRoot                 string `json:"codex_state_root"`
	CodexSessionRoot               string `json:"codex_session_root"`
	InstalledExecutablePath        string `json:"installed_executable_path"`
	InstalledExecutableDigest      string `json:"installed_executable_digest"`
	ProjectRoot                    string `json:"project_root"`
	LiveMCPPID                     int    `json:"live_mcp_pid"`
	LiveMCPStartedAt               string `json:"live_mcp_started_at"`
	LiveMCPFulfilledAt             string `json:"live_mcp_fulfilled_at"`
	LiveMCPExecutablePath          string `json:"live_mcp_executable_path"`
	LiveMCPExecutableDigest        string `json:"live_mcp_executable_digest"`
	LiveMCPProjectRoot             string `json:"live_mcp_project_root"`
	LiveMCPReceiptDigest           string `json:"live_mcp_receipt_digest"`
	RestartCheckpointDigest        string `json:"restart_checkpoint_digest"`
	FallbackReceiptDigest          string `json:"fallback_receipt_digest"`
	FallbackClearedAt              string `json:"fallback_cleared_at"`
	ExactTaskResumeCount           int    `json:"exact_task_resume_count"`
	FallbackWakeCount              int    `json:"fallback_wake_count"`
	SingleWriterObserved           bool   `json:"single_writer_observed"`
	LaunchdRemovalObserved         bool   `json:"launchd_removal_observed"`
	PrivateStateGitignoredObserved bool   `json:"private_state_gitignored_observed"`
	CandidateDigestReserved        bool   `json:"candidate_digest_reserved"`
	TemporaryStagesAbsent          bool   `json:"temporary_stages_absent"`
}

type p14InstalledScenarioObservation struct {
	ID                    string                             `json:"id"`
	SemanticRequestDigest string                             `json:"semantic_request_digest"`
	Verdict               string                             `json:"verdict"`
	SurfaceObservations   []p14InstalledSurfaceObservation   `json:"surface_observations"`
	PredicateObservations []p14InstalledPredicateObservation `json:"predicate_observations,omitempty"`
}

type p14InstalledSurfaceObservation struct {
	Surface                string `json:"surface"`
	RequestPayloadDigest   string `json:"request_payload_digest"`
	Source                 string `json:"source"`
	SourceReceiptDigest    string `json:"source_receipt_digest"`
	ObservedAt             string `json:"observed_at"`
	Outcome                string `json:"outcome"`
	ObservationCanonical   string `json:"observation_canonical"`
	ObservationDigest      string `json:"observation_digest"`
	NormalizedResultDigest string `json:"normalized_result_digest,omitempty"`
	FailureCode            string `json:"failure_code,omitempty"`
}

type p14InstalledPredicateObservation struct {
	PredicateID   string `json:"predicate_id"`
	SourceSurface string `json:"source_surface"`
	Satisfied     bool   `json:"satisfied"`
}

type p14InstalledObservationCarrier struct {
	Schema            string                       `json:"schema"`
	Status            string                       `json:"status"`
	CarrierPath       string                       `json:"carrier_path"`
	ObservationDigest string                       `json:"observation_digest"`
	Observation       p14InstalledObservationInput `json:"observation"`
}

func TestP14InstalledObservationCarrierNeedsEveryExecutedSurface(
	t *testing.T,
) {
	repositoryRoot := t.TempDir()
	sourceRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, rawContract, err := loadRequestOracleContract(sourceRoot)
	if err != nil {
		t.Fatal(err)
	}
	preparedInput, err := completePreparedInputForTest(
		contract,
		p14Digest(rawContract),
	)
	if err != nil {
		t.Fatal(err)
	}
	preparedPath, preparedDigest, err := persistPreparedRequestOracleCarrier(
		repositoryRoot,
		contract,
		preparedInput,
	)
	if err != nil {
		t.Fatal(err)
	}
	preparedRaw, err := os.ReadFile(
		filepath.Join(repositoryRoot, filepath.FromSlash(preparedPath)),
	)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := decodePreparedRequestOracleCarrier(contract, preparedRaw)
	if err != nil {
		t.Fatal(err)
	}
	runtime := syntheticP14RuntimeObservationBinding(prepared.Preparation)
	observations := syntheticPassingP14InstalledObservations(
		prepared.Preparation,
		runtime,
	)
	if _, err := buildP14InstalledObservationCarrier(
		contract,
		preparedPath,
		preparedDigest,
		prepared,
		runtime,
		p14ClaudeHostProofBinding{},
		observations,
	); err == nil {
		t.Fatal("P14 passing carrier accepted missing Claude host proof")
	}
	carrier, err := buildP14InstalledObservationCarrier(
		contract,
		preparedPath,
		preparedDigest,
		prepared,
		runtime,
		syntheticP14ClaudeHostProofBinding(),
		observations,
	)
	if err != nil {
		t.Fatal(err)
	}
	if carrier.Status != p14ObservationStatusPassed ||
		carrier.Observation.Status != p14ObservationStatusPassed {
		t.Fatal("complete executed observations did not produce a passing carrier")
	}
	path, digest, err := persistP14InstalledObservationCarrier(
		repositoryRoot,
		contract,
		carrier,
	)
	if err != nil {
		t.Fatal(err)
	}
	if path == "" || !validP14Digest(digest) {
		t.Fatalf("persisted P14 observation = %q %q", path, digest)
	}
	raw, err := os.ReadFile(
		filepath.Join(repositoryRoot, filepath.FromSlash(path)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeP14InstalledObservationCarrier(
		repositoryRoot,
		contract,
		raw,
	); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(`"nonce"`)) {
		t.Fatal("P14 installed observation carrier copied a private nonce")
	}
	if _, _, err := persistP14InstalledObservationCarrier(
		repositoryRoot,
		contract,
		carrier,
	); err == nil {
		t.Fatal("P14 installed observation carrier replaced existing bytes")
	}

	missingSurface := cloneP14InstalledScenarioObservations(observations)
	missingSurface[0].SurfaceObservations =
		missingSurface[0].SurfaceObservations[:2]
	if _, err := buildP14InstalledObservationCarrier(
		contract,
		preparedPath,
		preparedDigest,
		prepared,
		runtime,
		syntheticP14ClaudeHostProofBinding(),
		missingSurface,
	); err == nil {
		t.Fatal("P14 observation carrier accepted a prepared-only missing surface")
	}

	mismatch := cloneP14InstalledScenarioObservations(observations)
	mismatch[1].SurfaceObservations[0].NormalizedResultDigest =
		p14TestDigest("installed-mismatch")
	failed, err := buildP14InstalledObservationCarrier(
		contract,
		preparedPath,
		preparedDigest,
		prepared,
		runtime,
		syntheticP14ClaudeHostProofBinding(),
		mismatch,
	)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != p14ObservationStatusFailed ||
		failed.Observation.ScenarioObservations[1].Verdict !=
			p14ObservationStatusFailed {
		t.Fatal("P14 observed oracle mismatch was not retained as failed evidence")
	}

	preparedFile := filepath.Join(
		repositoryRoot,
		filepath.FromSlash(preparedPath),
	)
	if err := os.WriteFile(preparedFile, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeP14InstalledObservationCarrier(
		repositoryRoot,
		contract,
		raw,
	); err == nil {
		t.Fatal("P14 installed observation survived prepared-carrier byte drift")
	}
}

func TestP14RuntimeBindingUsesOnlyVerifiedRestartSnapshot(
	t *testing.T,
) {
	sourceRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, rawContract, err := loadRequestOracleContract(sourceRoot)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := completePreparedInputForTest(
		contract,
		p14Digest(rawContract),
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := syntheticP14VerifiedRuntimeSnapshot(prepared)
	runtime, err := p14RuntimeBindingFromVerifiedSnapshot(
		prepared,
		snapshot,
	)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.RestartID != snapshot.RestartID ||
		runtime.ThreadID != snapshot.ThreadID ||
		runtime.RestartCheckpointDigest !=
			snapshot.RestartCheckpointDigest ||
		runtime.LiveMCPReceiptDigest != snapshot.LiveMCPReceiptDigest ||
		runtime.FallbackReceiptDigest != snapshot.FallbackReceiptDigest ||
		!runtime.CandidateDigestReserved ||
		!runtime.TemporaryStagesAbsent {
		t.Fatal("P14 runtime binding lost verified restart evidence")
	}
	raw, err := json.Marshal(runtime)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(`"nonce"`)) {
		t.Fatal("P14 runtime binding exposed a private nonce field")
	}
	snapshot.CheckpointState = "resumed"
	if _, err := p14RuntimeBindingFromVerifiedSnapshot(
		prepared,
		snapshot,
	); err == nil {
		t.Fatal("P14 runtime binding accepted a merely resumed checkpoint")
	}
}

func buildP14InstalledObservationCarrier(
	contract requestOracleContract,
	preparedCarrierPath string,
	preparedCarrierDigest string,
	prepared preparedRequestOracleCarrier,
	runtime p14RuntimeObservationBinding,
	claudeHostProof p14ClaudeHostProofBinding,
	observations []p14InstalledScenarioObservation,
) (p14InstalledObservationCarrier, error) {
	if err := verifyPreparedRequestOracleCarrier(contract, prepared); err != nil {
		return p14InstalledObservationCarrier{}, err
	}
	if err := verifyP14PreparedObservationBindingValue(
		preparedCarrierPath,
		preparedCarrierDigest,
		prepared,
	); err != nil {
		return p14InstalledObservationCarrier{}, err
	}
	if err := validateP14RuntimeObservationBinding(
		prepared.Preparation,
		runtime,
	); err != nil {
		return p14InstalledObservationCarrier{}, err
	}
	if err := validateP14ClaudeHostProofBinding(claudeHostProof); err != nil {
		return p14InstalledObservationCarrier{}, err
	}
	evaluated, status, err := evaluateP14InstalledObservations(
		prepared.Preparation,
		observations,
	)
	if err != nil {
		return p14InstalledObservationCarrier{}, err
	}
	input := p14InstalledObservationInput{
		Schema:          p14InstalledObservationInputSchema,
		Status:          status,
		ResultSemantics: p14InstalledObservationSemantics,
		ReleaseClaim:    false,
		PreparedCarrier: p14PreparedObservationBinding{
			CarrierPath:       preparedCarrierPath,
			CarrierDigest:     preparedCarrierDigest,
			PreparationDigest: prepared.PreparationDigest,
		},
		Runtime:              runtime,
		ClaudeHostProof:      claudeHostProof,
		ScenarioObservations: evaluated,
	}
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return p14InstalledObservationCarrier{}, fmt.Errorf(
			"encode P14 installed observation digest basis: %w",
			err,
		)
	}
	observationDigest := p14Digest(inputBytes)
	digestBody := strings.TrimPrefix(observationDigest, "sha256:")
	name := "p14-installed-observation-" + digestBody[:16] + ".json"
	carrierPath := filepath.ToSlash(filepath.Join(".context", "p14", name))
	carrier := p14InstalledObservationCarrier{
		Schema:            p14InstalledObservationCarrierSchema,
		Status:            status,
		CarrierPath:       carrierPath,
		ObservationDigest: observationDigest,
		Observation:       input,
	}
	if err := validateP14InstalledObservationCarrier(contract, carrier); err != nil {
		return p14InstalledObservationCarrier{}, err
	}
	return carrier, nil
}

func evaluateP14InstalledObservations(
	prepared preparedRequestOracleInput,
	observations []p14InstalledScenarioObservation,
) ([]p14InstalledScenarioObservation, string, error) {
	if len(observations) != len(prepared.Scenarios) {
		return nil, "", fmt.Errorf(
			"P14 installed observation scenario count = %d",
			len(observations),
		)
	}
	evaluated := cloneP14InstalledScenarioObservations(observations)
	status := p14ObservationStatusPassed
	for index, scenario := range evaluated {
		preparedScenario := prepared.Scenarios[index]
		verdict, err := evaluateP14InstalledScenario(
			preparedScenario,
			scenario,
		)
		if err != nil {
			return nil, "", err
		}
		evaluated[index].Verdict = verdict
		if verdict == p14ObservationStatusFailed {
			status = p14ObservationStatusFailed
		}
	}
	return evaluated, status, nil
}

func evaluateP14InstalledScenario(
	prepared preparedP14Scenario,
	observed p14InstalledScenarioObservation,
) (string, error) {
	if observed.ID != prepared.ID ||
		observed.SemanticRequestDigest != prepared.SemanticRequestDigest ||
		len(observed.SurfaceObservations) != len(prepared.Requests) {
		return "", fmt.Errorf(
			"P14 installed observation scenario %q shape differs",
			observed.ID,
		)
	}
	for index, surface := range observed.SurfaceObservations {
		request := prepared.Requests[index]
		if err := validateP14InstalledSurfaceObservation(
			request,
			surface,
		); err != nil {
			return "", err
		}
	}
	evaluators := map[string]func(
		preparedP14Scenario,
		p14InstalledScenarioObservation,
	) (string, error){
		"normalized_digest": evaluateP14NormalizedObservation,
		"live_predicate":    evaluateP14PredicateObservation,
	}
	evaluator, present := evaluators[prepared.Oracle.Kind]
	if !present {
		return "", fmt.Errorf(
			"P14 installed observation oracle kind %q is open",
			prepared.Oracle.Kind,
		)
	}
	return evaluator(prepared, observed)
}

func validateP14InstalledSurfaceObservation(
	request preparedP14Request,
	observation p14InstalledSurfaceObservation,
) error {
	if observation.Surface != request.Surface ||
		observation.RequestPayloadDigest != request.PayloadDigest ||
		observation.Source != p14ObservationSourceForSurface(request.Surface) ||
		!validP14Digest(observation.SourceReceiptDigest) ||
		!validP14Digest(observation.ObservationDigest) {
		return fmt.Errorf(
			"P14 installed surface observation for %q has a different basis",
			request.Surface,
		)
	}
	if _, err := time.Parse(time.RFC3339Nano, observation.ObservedAt); err != nil {
		return fmt.Errorf(
			"P14 installed surface observation time is invalid: %w",
			err,
		)
	}
	raw := []byte(observation.ObservationCanonical)
	if !canonicalCompactJSON(raw) ||
		p14Digest(raw) != observation.ObservationDigest {
		return fmt.Errorf(
			"P14 installed surface observation is not exact canonical JSON",
		)
	}
	validOutcomes := []string{
		p14SurfaceOutcomeObserved,
		p14SurfaceOutcomeExecutionError,
		p14SurfaceOutcomeMismatch,
	}
	if !slices.Contains(validOutcomes, observation.Outcome) {
		return fmt.Errorf("P14 installed surface observation outcome is open")
	}
	if observation.Outcome == p14SurfaceOutcomeObserved &&
		observation.FailureCode != "" {
		return fmt.Errorf("P14 observed surface carries a failure code")
	}
	if observation.Outcome != p14SurfaceOutcomeObserved &&
		observation.FailureCode == "" {
		return fmt.Errorf("P14 failed surface omits its failure code")
	}
	if observation.NormalizedResultDigest != "" &&
		!validP14Digest(observation.NormalizedResultDigest) {
		return fmt.Errorf(
			"P14 installed surface normalized result digest is invalid",
		)
	}
	return nil
}

func p14ObservationSourceForSurface(surface string) string {
	sources := map[string]string{
		"installed_cli": p14ObservationSourceInstalledCLI,
		"live_mcp":      p14ObservationSourceLiveMCP,
		"host_process":  p14ObservationSourceHostProcess,
	}
	return sources[surface]
}

func evaluateP14NormalizedObservation(
	prepared preparedP14Scenario,
	observed p14InstalledScenarioObservation,
) (string, error) {
	if len(observed.PredicateObservations) != 0 {
		return "", fmt.Errorf(
			"P14 normalized observation %q carries predicate assertions",
			observed.ID,
		)
	}
	verdict := p14ObservationStatusPassed
	for _, surface := range observed.SurfaceObservations {
		if surface.Outcome != p14SurfaceOutcomeObserved ||
			surface.NormalizedResultDigest !=
				prepared.Oracle.ExpectedResultDigest {
			verdict = p14ObservationStatusFailed
		}
	}
	return verdict, nil
}

func evaluateP14PredicateObservation(
	prepared preparedP14Scenario,
	observed p14InstalledScenarioObservation,
) (string, error) {
	if len(observed.PredicateObservations) != len(prepared.Oracle.PredicateIDs) {
		return "", fmt.Errorf(
			"P14 predicate observation %q is incomplete",
			observed.ID,
		)
	}
	declaredSurfaces := make([]string, 0, len(prepared.Requests))
	for _, request := range prepared.Requests {
		declaredSurfaces = append(declaredSurfaces, request.Surface)
	}
	seen := make(map[string]struct{}, len(observed.PredicateObservations))
	verdict := p14ObservationStatusPassed
	for index, predicate := range observed.PredicateObservations {
		if predicate.PredicateID != prepared.Oracle.PredicateIDs[index] ||
			!slices.Contains(declaredSurfaces, predicate.SourceSurface) {
			return "", fmt.Errorf(
				"P14 predicate observation %q differs from its oracle",
				observed.ID,
			)
		}
		if _, duplicate := seen[predicate.PredicateID]; duplicate {
			return "", fmt.Errorf(
				"P14 predicate observation %q repeats a predicate",
				observed.ID,
			)
		}
		seen[predicate.PredicateID] = struct{}{}
		if !predicate.Satisfied {
			verdict = p14ObservationStatusFailed
		}
	}
	for _, surface := range observed.SurfaceObservations {
		if surface.Outcome != p14SurfaceOutcomeObserved ||
			surface.NormalizedResultDigest != "" {
			verdict = p14ObservationStatusFailed
		}
	}
	return verdict, nil
}

func validateP14RuntimeObservationBinding(
	prepared preparedRequestOracleInput,
	runtime p14RuntimeObservationBinding,
) error {
	candidate := prepared.FrozenBasis.Candidate
	project := prepared.FrozenBasis.SelectedProject
	if runtime.Schema != p14RuntimeObservationBindingSchema ||
		strings.TrimSpace(runtime.RestartID) == "" ||
		strings.TrimSpace(runtime.ThreadID) == "" ||
		!validP14Digest(runtime.ResumeIntentDigest) ||
		runtime.RestartCheckpointState != "verified" ||
		runtime.PreparedTaskRuntimePID <= 0 ||
		!filepath.IsAbs(runtime.PreparedTaskRuntimeExecutable) ||
		!validP14Digest(runtime.PreparedTaskRuntimeArgsDigest) ||
		!filepath.IsAbs(runtime.CodexStateRoot) ||
		!filepath.IsAbs(runtime.CodexSessionRoot) ||
		!filepath.IsAbs(runtime.InstalledExecutablePath) ||
		!filepath.IsAbs(runtime.ProjectRoot) ||
		!filepath.IsAbs(runtime.LiveMCPExecutablePath) ||
		!filepath.IsAbs(runtime.LiveMCPProjectRoot) ||
		runtime.LiveMCPPID <= 0 {
		return fmt.Errorf("P14 runtime observation identity is incomplete")
	}
	if runtime.InstalledExecutableDigest != candidate.ExecutableDigest ||
		runtime.LiveMCPExecutableDigest != candidate.ExecutableDigest ||
		runtime.LiveMCPExecutablePath != runtime.InstalledExecutablePath ||
		runtime.ProjectRoot != project.ProjectRoot ||
		runtime.LiveMCPProjectRoot != project.ProjectRoot {
		return fmt.Errorf("P14 runtime observation differs from the frozen candidate")
	}
	checkpointCreatedAt, err := time.Parse(
		time.RFC3339Nano,
		runtime.RestartCheckpointCreatedAt,
	)
	if err != nil {
		return fmt.Errorf("P14 restart checkpoint time is invalid: %w", err)
	}
	preparedTaskStartedAt, err := time.Parse(
		time.RFC3339Nano,
		runtime.PreparedTaskRuntimeStartedAt,
	)
	if err != nil {
		return fmt.Errorf(
			"P14 prepared task runtime start is invalid: %w",
			err,
		)
	}
	liveStartedAt, err := time.Parse(
		time.RFC3339Nano,
		runtime.LiveMCPStartedAt,
	)
	if err != nil {
		return fmt.Errorf("P14 live MCP process start is invalid: %w", err)
	}
	liveFulfilledAt, err := time.Parse(
		time.RFC3339Nano,
		runtime.LiveMCPFulfilledAt,
	)
	if err != nil {
		return fmt.Errorf("P14 live MCP receipt time is invalid: %w", err)
	}
	fallbackClearedAt, err := time.Parse(
		time.RFC3339Nano,
		runtime.FallbackClearedAt,
	)
	if err != nil {
		return fmt.Errorf("P14 fallback cleanup time is invalid: %w", err)
	}
	if preparedTaskStartedAt.After(checkpointCreatedAt) ||
		!liveStartedAt.After(checkpointCreatedAt) ||
		liveFulfilledAt.Before(liveStartedAt) ||
		!fallbackClearedAt.After(checkpointCreatedAt) {
		return fmt.Errorf("P14 runtime observation timestamps are inconsistent")
	}
	expectedStateRoot, expectedSessionRoot, err :=
		p14CanonicalCodexRuntimeRoots()
	if err != nil {
		return err
	}
	if runtime.CodexStateRoot != expectedStateRoot ||
		runtime.CodexSessionRoot != expectedSessionRoot ||
		!p14PathIsWithin(
			runtime.CodexStateRoot,
			runtime.CodexSessionRoot,
		) {
		return fmt.Errorf(
			"P14 Codex state or session root differs from the canonical host account",
		)
	}
	digests := []string{
		runtime.LiveMCPReceiptDigest,
		runtime.RestartCheckpointDigest,
		runtime.FallbackReceiptDigest,
	}
	if !allP14Digests(digests) ||
		runtime.ExactTaskResumeCount != 1 ||
		runtime.FallbackWakeCount < 0 ||
		runtime.FallbackWakeCount > 1 ||
		!runtime.SingleWriterObserved ||
		!runtime.LaunchdRemovalObserved ||
		!runtime.PrivateStateGitignoredObserved ||
		!runtime.CandidateDigestReserved ||
		!runtime.TemporaryStagesAbsent {
		return fmt.Errorf(
			"P14 runtime observation lacks verified resume or cleanup receipts",
		)
	}
	return nil
}

func p14RuntimeBindingFromVerifiedSnapshot(
	prepared preparedRequestOracleInput,
	snapshot agenthostrestart.VerifiedRuntimeSnapshot,
) (p14RuntimeObservationBinding, error) {
	codexStateRoot, codexSessionRoot, err :=
		p14CanonicalCodexRuntimeRoots()
	if err != nil {
		return p14RuntimeObservationBinding{}, err
	}
	runtime := p14RuntimeObservationBinding{
		Schema:                     p14RuntimeObservationBindingSchema,
		RestartID:                  snapshot.RestartID,
		ThreadID:                   snapshot.ThreadID,
		ResumeIntentDigest:         snapshot.ResumeIntentDigest,
		RestartCheckpointState:     snapshot.CheckpointState,
		RestartCheckpointCreatedAt: snapshot.CheckpointCreatedAt.UTC().Format(time.RFC3339Nano),
		PreparedTaskRuntimePID:     snapshot.PreparedTaskRuntimePID,
		PreparedTaskRuntimeStartedAt: snapshot.PreparedTaskRuntimeStartedAt.
			UTC().
			Format(time.RFC3339Nano),
		PreparedTaskRuntimeExecutable: snapshot.
			PreparedTaskRuntimeExecutable,
		PreparedTaskRuntimeArgsDigest: snapshot.
			PreparedTaskRuntimeArgsDigest,
		CodexStateRoot:            codexStateRoot,
		CodexSessionRoot:          codexSessionRoot,
		InstalledExecutablePath:   snapshot.InstalledExecutablePath,
		InstalledExecutableDigest: snapshot.InstalledExecutableDigest,
		ProjectRoot:               snapshot.ProjectRoot,
		LiveMCPPID:                snapshot.LiveMCPPID,
		LiveMCPStartedAt:          snapshot.LiveMCPStartedAt.UTC().Format(time.RFC3339Nano),
		LiveMCPFulfilledAt:        snapshot.LiveMCPFulfilledAt.UTC().Format(time.RFC3339Nano),
		LiveMCPExecutablePath:     snapshot.LiveMCPExecutablePath,
		LiveMCPExecutableDigest:   snapshot.LiveMCPExecutableDigest,
		LiveMCPProjectRoot:        snapshot.LiveMCPProjectRoot,
		LiveMCPReceiptDigest:      snapshot.LiveMCPReceiptDigest,
		RestartCheckpointDigest:   snapshot.RestartCheckpointDigest,
		FallbackReceiptDigest:     snapshot.FallbackReceiptDigest,
		FallbackClearedAt:         snapshot.FallbackClearedAt.UTC().Format(time.RFC3339Nano),
		ExactTaskResumeCount:      int(snapshot.ExactTaskResumeCount),
		FallbackWakeCount:         int(snapshot.FallbackWakeCount),
		SingleWriterObserved:      snapshot.SingleWriterObserved,
		LaunchdRemovalObserved:    snapshot.LaunchdRemovalObserved,
		PrivateStateGitignoredObserved: snapshot.
			PrivateStateGitignoredObserved,
		CandidateDigestReserved: snapshot.CandidateDigestReserved,
		TemporaryStagesAbsent:   snapshot.TemporaryStagesAbsent,
	}
	if snapshot.CheckpointAttempt != 1 {
		return p14RuntimeObservationBinding{}, fmt.Errorf(
			"P14 restart checkpoint attempt is not one",
		)
	}
	if err := validateP14RuntimeObservationBinding(
		prepared,
		runtime,
	); err != nil {
		return p14RuntimeObservationBinding{}, err
	}
	return runtime, nil
}

func validateP14InstalledObservationCarrier(
	contract requestOracleContract,
	carrier p14InstalledObservationCarrier,
) error {
	if carrier.Schema != p14InstalledObservationCarrierSchema ||
		!validP14Digest(carrier.ObservationDigest) ||
		carrier.Status != carrier.Observation.Status {
		return fmt.Errorf("P14 installed observation carrier header is invalid")
	}
	input := carrier.Observation
	if input.Schema != p14InstalledObservationInputSchema ||
		input.ResultSemantics != p14InstalledObservationSemantics ||
		input.ReleaseClaim {
		return fmt.Errorf(
			"P14 installed observation carrier overstates its authority",
		)
	}
	if input.Status != p14ObservationStatusPassed &&
		input.Status != p14ObservationStatusFailed {
		return fmt.Errorf("P14 installed observation status is open")
	}
	if err := validateP14PreparedObservationBinding(
		input.PreparedCarrier,
	); err != nil {
		return err
	}
	if err := validateP14ClaudeHostProofBinding(
		input.ClaudeHostProof,
	); err != nil {
		return err
	}
	prepared := preparedRequestOracleInput{
		FrozenBasis: frozenP14Basis{
			Candidate: candidateP14Basis{
				ExecutableDigest: input.Runtime.InstalledExecutableDigest,
			},
			SelectedProject: selectedProjectP14Basis{
				ProjectRoot: input.Runtime.ProjectRoot,
			},
		},
	}
	if err := validateP14RuntimeObservationBinding(
		prepared,
		input.Runtime,
	); err != nil {
		return err
	}
	if len(input.ScenarioObservations) != len(contract.Scenarios) {
		return fmt.Errorf(
			"P14 installed observation contract scenario count differs",
		)
	}
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("redigest P14 installed observation: %w", err)
	}
	wantDigest := p14Digest(inputBytes)
	digestBody := strings.TrimPrefix(wantDigest, "sha256:")
	wantPath := filepath.ToSlash(filepath.Join(
		".context",
		"p14",
		"p14-installed-observation-"+digestBody[:16]+".json",
	))
	if carrier.ObservationDigest != wantDigest ||
		carrier.CarrierPath != wantPath {
		return fmt.Errorf(
			"P14 installed observation carrier path or digest differs",
		)
	}
	return nil
}

func validateP14PreparedObservationBinding(
	binding p14PreparedObservationBinding,
) error {
	clean := filepath.Clean(filepath.FromSlash(binding.CarrierPath))
	if filepath.IsAbs(clean) ||
		filepath.Dir(clean) != filepath.Join(".context", "p14") ||
		!strings.HasPrefix(
			filepath.Base(clean),
			"p14-prepared-request-oracle-",
		) ||
		!validP14Digest(binding.CarrierDigest) ||
		!validP14Digest(binding.PreparationDigest) {
		return fmt.Errorf("P14 installed observation prepared binding is invalid")
	}
	return nil
}

func verifyP14PreparedObservationBindingValue(
	carrierPath string,
	carrierDigest string,
	prepared preparedRequestOracleCarrier,
) error {
	if carrierPath != prepared.CarrierPath ||
		!validP14Digest(carrierDigest) {
		return fmt.Errorf(
			"P14 prepared observation carrier path or digest is invalid",
		)
	}
	canonical, err := json.MarshalIndent(prepared, "", "  ")
	if err != nil {
		return fmt.Errorf("reencode P14 prepared observation carrier: %w", err)
	}
	canonical = append(canonical, '\n')
	if p14Digest(canonical) != carrierDigest {
		return fmt.Errorf(
			"P14 prepared observation carrier digest differs from its bytes",
		)
	}
	return nil
}

func verifyP14InstalledObservationCarrierAgainstPrepared(
	repositoryRoot string,
	contract requestOracleContract,
	carrier p14InstalledObservationCarrier,
) error {
	if err := validateP14InstalledObservationCarrier(contract, carrier); err != nil {
		return err
	}
	binding := carrier.Observation.PreparedCarrier
	path := filepath.Join(
		repositoryRoot,
		filepath.FromSlash(binding.CarrierPath),
	)
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read P14 prepared carrier for observation: %w", err)
	}
	if p14Digest(raw) != binding.CarrierDigest {
		return fmt.Errorf("P14 prepared carrier bytes changed after observation")
	}
	prepared, err := decodePreparedRequestOracleCarrier(contract, raw)
	if err != nil {
		return err
	}
	if prepared.PreparationDigest != binding.PreparationDigest {
		return fmt.Errorf("P14 observation uses another preparation digest")
	}
	if err := validateP14RuntimeObservationBinding(
		prepared.Preparation,
		carrier.Observation.Runtime,
	); err != nil {
		return err
	}
	evaluated, status, err := evaluateP14InstalledObservations(
		prepared.Preparation,
		carrier.Observation.ScenarioObservations,
	)
	if err != nil {
		return err
	}
	expectedBytes, err := json.Marshal(evaluated)
	if err != nil {
		return err
	}
	observedBytes, err := json.Marshal(carrier.Observation.ScenarioObservations)
	if err != nil {
		return err
	}
	if status != carrier.Status ||
		status != carrier.Observation.Status ||
		!bytes.Equal(expectedBytes, observedBytes) {
		return fmt.Errorf(
			"P14 installed observation verdict differs from executed observations",
		)
	}
	return nil
}

func persistP14InstalledObservationCarrier(
	repositoryRoot string,
	contract requestOracleContract,
	carrier p14InstalledObservationCarrier,
) (string, string, error) {
	if err := verifyP14InstalledObservationCarrierAgainstPrepared(
		repositoryRoot,
		contract,
		carrier,
	); err != nil {
		return "", "", err
	}
	canonical, err := json.MarshalIndent(carrier, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf(
			"encode P14 installed observation carrier: %w",
			err,
		)
	}
	canonical = append(canonical, '\n')
	if err := publishP14NoClobber(
		repositoryRoot,
		filepath.FromSlash(carrier.CarrierPath),
		canonical,
	); err != nil {
		return "", "", err
	}
	return carrier.CarrierPath, p14Digest(canonical), nil
}

func decodeP14InstalledObservationCarrier(
	repositoryRoot string,
	contract requestOracleContract,
	raw []byte,
) (p14InstalledObservationCarrier, error) {
	reader := bytes.NewReader(raw)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var carrier p14InstalledObservationCarrier
	if err := decoder.Decode(&carrier); err != nil {
		return p14InstalledObservationCarrier{}, fmt.Errorf(
			"decode P14 installed observation carrier: %w",
			err,
		)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		return p14InstalledObservationCarrier{}, fmt.Errorf(
			"P14 installed observation carrier has trailing JSON",
		)
	}
	if err := verifyP14InstalledObservationCarrierAgainstPrepared(
		repositoryRoot,
		contract,
		carrier,
	); err != nil {
		return p14InstalledObservationCarrier{}, err
	}
	canonical, err := json.MarshalIndent(carrier, "", "  ")
	if err != nil {
		return p14InstalledObservationCarrier{}, err
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(raw, canonical) {
		return p14InstalledObservationCarrier{}, fmt.Errorf(
			"P14 installed observation carrier is not canonical JSON",
		)
	}
	return carrier, nil
}

func syntheticP14RuntimeObservationBinding(
	prepared preparedRequestOracleInput,
) p14RuntimeObservationBinding {
	runtime, err := p14RuntimeBindingFromVerifiedSnapshot(
		prepared,
		syntheticP14VerifiedRuntimeSnapshot(prepared),
	)
	if err != nil {
		panic(err)
	}
	return runtime
}

func syntheticP14ClaudeHostProofBinding() p14ClaudeHostProofBinding {
	return p14ClaudeHostProofBinding{
		CarrierPath: filepath.ToSlash(filepath.Join(
			".context",
			"p14",
			"p14-claude-host-proof-synthetic.json",
		)),
		CarrierDigest:  p14TestDigest("claude-host-proof-carrier"),
		EvidenceDigest: p14TestDigest("claude-host-proof-evidence"),
	}
}

func validateP14ClaudeHostProofBinding(
	binding p14ClaudeHostProofBinding,
) error {
	if filepath.Dir(binding.CarrierPath) != ".context/p14" ||
		!strings.HasPrefix(
			filepath.Base(binding.CarrierPath),
			p14ClaudeHostProofCarrierPrefix+"-",
		) ||
		!validP14Digest(binding.CarrierDigest) ||
		!validP14Digest(binding.EvidenceDigest) {
		return fmt.Errorf("P14 Claude host proof binding is invalid")
	}
	return nil
}

func syntheticP14VerifiedRuntimeSnapshot(
	prepared preparedRequestOracleInput,
) agenthostrestart.VerifiedRuntimeSnapshot {
	return agenthostrestart.VerifiedRuntimeSnapshot{
		RestartID:                      "p14-restart",
		ThreadID:                       "019f5a6e-fba1-7cd3-8421-677d5431bd12",
		ResumeIntentDigest:             p14TestDigest("resume-intent"),
		CheckpointState:                "verified",
		CheckpointAttempt:              1,
		CheckpointCreatedAt:            time.Date(2026, 7, 26, 11, 59, 0, 0, time.UTC),
		RestartCheckpointDigest:        p14TestDigest("restart-checkpoint"),
		PreparedTaskRuntimePID:         4001,
		PreparedTaskRuntimeStartedAt:   time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC),
		PreparedTaskRuntimeExecutable:  "/Applications/Codex.app/Contents/Resources/codex",
		PreparedTaskRuntimeArgsDigest:  p14TestDigest("prepared-task-runtime-args"),
		InstalledExecutablePath:        "/installed/haft",
		InstalledExecutableDigest:      prepared.FrozenBasis.Candidate.ExecutableDigest,
		ProjectRoot:                    prepared.FrozenBasis.SelectedProject.ProjectRoot,
		LiveMCPPID:                     4242,
		LiveMCPStartedAt:               time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC),
		LiveMCPFulfilledAt:             time.Date(2026, 7, 26, 12, 0, 1, 0, time.UTC),
		LiveMCPExecutablePath:          "/installed/haft",
		LiveMCPExecutableDigest:        prepared.FrozenBasis.Candidate.ExecutableDigest,
		LiveMCPProjectRoot:             prepared.FrozenBasis.SelectedProject.ProjectRoot,
		LiveMCPReceiptDigest:           p14TestDigest("live-mcp-receipt"),
		FallbackReceiptDigest:          p14TestDigest("fallback-receipt"),
		FallbackWakeCount:              1,
		FallbackClearedAt:              time.Date(2026, 7, 26, 12, 0, 2, 0, time.UTC),
		ExactTaskResumeCount:           1,
		SingleWriterObserved:           true,
		LaunchdRemovalObserved:         true,
		PrivateStateGitignoredObserved: true,
		CandidateDigestReserved:        true,
		TemporaryStagesAbsent:          true,
	}
}

func syntheticPassingP14InstalledObservations(
	prepared preparedRequestOracleInput,
	runtime p14RuntimeObservationBinding,
) []p14InstalledScenarioObservation {
	observedAt := runtime.LiveMCPStartedAt
	scenarios := make([]p14InstalledScenarioObservation, 0, len(prepared.Scenarios))
	for _, preparedScenario := range prepared.Scenarios {
		surfaces := make(
			[]p14InstalledSurfaceObservation,
			0,
			len(preparedScenario.Requests),
		)
		for _, request := range preparedScenario.Requests {
			raw, _ := marshalP14CanonicalJSON(map[string]any{
				"scenario_id": preparedScenario.ID,
				"surface":     request.Surface,
				"observed":    true,
			})
			normalizedDigest := preparedScenario.Oracle.ExpectedResultDigest
			if preparedScenario.Oracle.Kind == "live_predicate" {
				normalizedDigest = ""
			}
			surfaces = append(surfaces, p14InstalledSurfaceObservation{
				Surface:                request.Surface,
				RequestPayloadDigest:   request.PayloadDigest,
				Source:                 p14ObservationSourceForSurface(request.Surface),
				SourceReceiptDigest:    p14TestDigest("receipt:" + request.Surface),
				ObservedAt:             observedAt,
				Outcome:                p14SurfaceOutcomeObserved,
				ObservationCanonical:   string(raw),
				ObservationDigest:      p14Digest(raw),
				NormalizedResultDigest: normalizedDigest,
			})
		}
		predicates := make(
			[]p14InstalledPredicateObservation,
			0,
			len(preparedScenario.Oracle.PredicateIDs),
		)
		for _, predicateID := range preparedScenario.Oracle.PredicateIDs {
			predicates = append(predicates, p14InstalledPredicateObservation{
				PredicateID:   predicateID,
				SourceSurface: preparedScenario.Requests[0].Surface,
				Satisfied:     true,
			})
		}
		scenarios = append(scenarios, p14InstalledScenarioObservation{
			ID:                    preparedScenario.ID,
			SemanticRequestDigest: preparedScenario.SemanticRequestDigest,
			SurfaceObservations:   surfaces,
			PredicateObservations: predicates,
		})
	}
	return scenarios
}

func cloneP14InstalledScenarioObservations(
	observations []p14InstalledScenarioObservation,
) []p14InstalledScenarioObservation {
	cloned := make([]p14InstalledScenarioObservation, 0, len(observations))
	for _, observation := range observations {
		cloned = append(cloned, p14InstalledScenarioObservation{
			ID:                    observation.ID,
			SemanticRequestDigest: observation.SemanticRequestDigest,
			Verdict:               observation.Verdict,
			SurfaceObservations: slices.Clone(
				observation.SurfaceObservations,
			),
			PredicateObservations: slices.Clone(
				observation.PredicateObservations,
			),
		})
	}
	return cloned
}
