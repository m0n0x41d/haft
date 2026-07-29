package p14acceptance

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

const (
	p14InstalledCLICaptureEnvironmentKey   = "HAFT_P14_CAPTURE_INSTALLED_CLI"
	p14InstalledCLICandidateEnvironmentKey = "HAFT_P14_INSTALLED_CLI_CANDIDATE"

	p14InstalledCLICaptureInputSchema   = "haft.p14.installed-cli-capture-input/v1"
	p14InstalledCLICaptureCarrierSchema = "haft.p14.installed-cli-capture/v1"
	p14InstalledCLIReceiptSchema        = "haft.p14.installed-cli-receipt/v2"
	p14InstalledCLICaptureStatus        = "captured_not_final"
	p14InstalledCLICaptureSemantics     = "Exact installed-CLI P14 observations bound to one sealed preparation and candidate digest. This partial carrier is not a complete P14 verdict, live-MCP evidence, restart evidence, performed Work, release authority, or a release claim."
	p14InstalledCLIReadOnlyIsolation    = "selected_project_read_only_with_fresh_bounded_home_clone"

	p14InstalledCLICommandTimeout = 90 * time.Second
	p14InstalledCLIOutputLimit    = 4 << 20
)

type p14InstalledCLICaptureInput struct {
	Schema                    string                           `json:"schema"`
	Status                    string                           `json:"status"`
	ResultSemantics           string                           `json:"result_semantics"`
	ReleaseClaim              bool                             `json:"release_claim"`
	PreparedCarrier           p14PreparedObservationBinding    `json:"prepared_carrier"`
	InstalledExecutablePath   string                           `json:"installed_executable_path"`
	InstalledExecutableDigest string                           `json:"installed_executable_digest"`
	CapturedAt                string                           `json:"captured_at"`
	ScenarioCaptures          []p14InstalledCLIScenarioCapture `json:"scenario_captures"`
}

type p14InstalledCLIScenarioCapture struct {
	ID                    string                         `json:"id"`
	SemanticRequestDigest string                         `json:"semantic_request_digest"`
	SurfaceObservation    p14InstalledSurfaceObservation `json:"surface_observation"`
}

type p14InstalledCLICaptureCarrier struct {
	Schema        string                      `json:"schema"`
	Status        string                      `json:"status"`
	CarrierPath   string                      `json:"carrier_path"`
	CaptureDigest string                      `json:"capture_digest"`
	Capture       p14InstalledCLICaptureInput `json:"capture"`
}

type p14InstalledCLIExecutionReceipt struct {
	Schema               string                          `json:"schema"`
	ScenarioID           string                          `json:"scenario_id"`
	Builder              string                          `json:"builder"`
	CandidateDigest      string                          `json:"candidate_digest"`
	RequestPayloadDigest string                          `json:"request_payload_digest"`
	ObservedAt           string                          `json:"observed_at,omitempty"`
	Fixture              *p14InstalledCLIFixtureReceipt  `json:"fixture,omitempty"`
	Fixtures             []p14InstalledCLIFixtureReceipt `json:"fixtures,omitempty"`
	Commands             []p14InstalledCLICommandReceipt `json:"commands,omitempty"`
	Checks               []p14InstalledCLICheckReceipt   `json:"checks"`
	FailureDetail        string                          `json:"failure_detail,omitempty"`
}

type p14InstalledCLIFixtureReceipt struct {
	CaseID              string `json:"case_id,omitempty"`
	Isolation           string `json:"isolation"`
	ProjectBasisDigest  string `json:"project_basis_digest"`
	HomeTemplateDigest  string `json:"home_template_digest"`
	ProjectBeforeDigest string `json:"project_before_digest"`
	ProjectAfterDigest  string `json:"project_after_digest"`
	HomeBeforeDigest    string `json:"home_before_digest"`
	HomeAfterDigest     string `json:"home_after_digest"`
}

type p14InstalledCLICommandReceipt struct {
	ID             string   `json:"id"`
	ParallelGroup  string   `json:"parallel_group,omitempty"`
	Argv           []string `json:"argv"`
	StdinDigest    string   `json:"stdin_digest"`
	StartedAt      string   `json:"started_at"`
	FinishedAt     string   `json:"finished_at"`
	ExitCode       int      `json:"exit_code"`
	StdoutBase64   string   `json:"stdout_base64"`
	StdoutDigest   string   `json:"stdout_digest"`
	StderrBase64   string   `json:"stderr_base64"`
	StderrDigest   string   `json:"stderr_digest"`
	OutputLimited  bool     `json:"output_limited"`
	ExecutionError string   `json:"execution_error,omitempty"`
}

type p14InstalledCLICheckReceipt struct {
	ID        string `json:"id"`
	Satisfied bool   `json:"satisfied"`
}

type p14InstalledCLIProcessRequest struct {
	ID            string
	ParallelGroup string
	Executable    string
	Argv          []string
	Stdin         string
	Directory     string
	Environment   []string
	Timeout       time.Duration
}

type p14InstalledCLIProcessResult struct {
	ID             string
	ParallelGroup  string
	Argv           []string
	StdinDigest    string
	StartedAt      time.Time
	FinishedAt     time.Time
	ExitCode       int
	Stdout         []byte
	Stderr         []byte
	OutputLimited  bool
	ExecutionError string
}

type p14InstalledCLIProcessExecutor func(
	context.Context,
	p14InstalledCLIProcessRequest,
) p14InstalledCLIProcessResult

type p14InstalledCLIExecutionContext struct {
	RepositoryRoot   string
	WorkspaceRoot    string
	ExecutablePath   string
	ExecutableDigest string
	Prepared         preparedRequestOracleCarrier
	PriorCaptures    map[string]p14InstalledCLIScenarioCapture
	Executor         p14InstalledCLIProcessExecutor
}

type p14InstalledCLIFamilyResult struct {
	Receipt        p14InstalledCLIExecutionReceipt
	Normalized     []byte
	FailureCode    string
	FailureDetail  string
	ExecutionError bool
}

type p14InstalledCLIReadOnlyFixture struct {
	ProjectRoot        string
	HomeRoot           string
	ProjectBasisDigest string
	HomeTemplateDigest string
	ProjectBefore      string
	HomeBefore         string
}

type p14InstalledCLIFamilyRunner func(
	context.Context,
	p14InstalledCLIExecutionContext,
	preparedP14Scenario,
	preparedP14Request,
) (p14InstalledCLIFamilyResult, error)

func TestP14CaptureInstalledCLI(t *testing.T) {
	preparedPath := os.Getenv(p14InstalledCLICaptureEnvironmentKey)
	if preparedPath == "" {
		t.Skip("set HAFT_P14_CAPTURE_INSTALLED_CLI after the exact candidate is installed")
	}
	candidatePath := os.Getenv(p14InstalledCLICandidateEnvironmentKey)
	if candidatePath == "" {
		t.Fatal("HAFT_P14_INSTALLED_CLI_CANDIDATE is required")
	}
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, _, err := loadRequestOracleContract(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	prepared, preparedDigest, err := loadP14PreparedCarrierForExecution(
		repositoryRoot,
		contract,
		preparedPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyPreparedInputCurrentBasis(
		repositoryRoot,
		prepared.Preparation,
	); err != nil {
		t.Fatal(err)
	}
	canonicalCandidate, err := canonicalP14InstalledExecutable(
		candidatePath,
		prepared.Preparation.FrozenBasis.Candidate.ExecutableDigest,
	)
	if err != nil {
		t.Fatal(err)
	}
	workspaceRoot, err := prepareP14InstalledCLIWorkspace(
		prepared.Preparation.FrozenBasis.Candidate.ExecutableDigest,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workspaceRoot)
	execution := p14InstalledCLIExecutionContext{
		RepositoryRoot:   repositoryRoot,
		WorkspaceRoot:    workspaceRoot,
		ExecutablePath:   canonicalCandidate,
		ExecutableDigest: prepared.Preparation.FrozenBasis.Candidate.ExecutableDigest,
		Prepared:         prepared,
		Executor:         executeP14InstalledCLIProcess,
	}
	captures, capturedAt, err := executeP14InstalledCLISurfaces(
		context.Background(),
		execution,
	)
	if err != nil {
		t.Fatal(err)
	}
	carrier, err := buildP14InstalledCLICaptureCarrier(
		contract,
		prepared.CarrierPath,
		preparedDigest,
		prepared,
		execution,
		capturedAt,
		captures,
	)
	if err != nil {
		t.Fatal(err)
	}
	path, digest, err := persistP14InstalledCLICaptureCarrier(
		repositoryRoot,
		contract,
		carrier,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("P14_INSTALLED_CLI_CAPTURE path=%s digest=%s", path, digest)
}

func executeP14InstalledCLISurfaces(
	ctx context.Context,
	execution p14InstalledCLIExecutionContext,
) ([]p14InstalledCLIScenarioCapture, time.Time, error) {
	if execution.Executor == nil {
		return nil, time.Time{}, fmt.Errorf("P14 installed CLI executor is required")
	}
	if err := verifyP14FileDigest(
		execution.ExecutablePath,
		execution.ExecutableDigest,
	); err != nil {
		return nil, time.Time{}, err
	}
	runners := p14InstalledCLIFamilyRunners()
	results := make(map[string]p14InstalledCLIScenarioCapture)
	latest := time.Time{}
	for _, scenario := range execution.Prepared.Preparation.Scenarios {
		request, present := p14PreparedSurfaceRequest(
			scenario,
			"installed_cli",
		)
		if !present || scenario.ID == "runtime_identity" {
			continue
		}
		runner := runners[request.Builder]
		if runner == nil {
			return nil, time.Time{}, fmt.Errorf(
				"P14 installed CLI builder %q has no runner",
				request.Builder,
			)
		}
		result, err := runner(ctx, execution, scenario, request)
		if err != nil {
			return nil, time.Time{}, err
		}
		observation, observedAt, err := p14InstalledCLIObservation(
			scenario,
			request,
			result,
		)
		if err != nil {
			return nil, time.Time{}, err
		}
		results[scenario.ID] = p14InstalledCLIScenarioCapture{
			ID:                    scenario.ID,
			SemanticRequestDigest: scenario.SemanticRequestDigest,
			SurfaceObservation:    observation,
		}
		execution.PriorCaptures = results
		if observedAt.After(latest) {
			latest = observedAt
		}
	}
	runtimeScenario, err := preparedP14ScenarioByID(
		execution.Prepared.Preparation.Scenarios,
		"runtime_identity",
	)
	if err != nil {
		return nil, time.Time{}, err
	}
	runtimeRequest, present := p14PreparedSurfaceRequest(
		runtimeScenario,
		"installed_cli",
	)
	if !present {
		return nil, time.Time{}, fmt.Errorf(
			"P14 runtime identity omits installed CLI",
		)
	}
	runtimeRunner := runners[p14RuntimeIdentityBuilderID]
	runtimeExecution := execution
	runtimeExecution.PriorCaptures = results
	runtimeResult, err := runtimeRunner(
		ctx,
		runtimeExecution,
		runtimeScenario,
		runtimeRequest,
	)
	if err != nil {
		return nil, time.Time{}, err
	}
	runtimeObservation, runtimeAt, err := p14InstalledCLIObservation(
		runtimeScenario,
		runtimeRequest,
		runtimeResult,
	)
	if err != nil {
		return nil, time.Time{}, err
	}
	results[runtimeScenario.ID] = p14InstalledCLIScenarioCapture{
		ID:                    runtimeScenario.ID,
		SemanticRequestDigest: runtimeScenario.SemanticRequestDigest,
		SurfaceObservation:    runtimeObservation,
	}
	if runtimeAt.After(latest) {
		latest = runtimeAt
	}
	captures := make([]p14InstalledCLIScenarioCapture, 0, len(results))
	for _, scenario := range execution.Prepared.Preparation.Scenarios {
		if _, present := p14PreparedSurfaceRequest(scenario, "installed_cli"); !present {
			continue
		}
		capture, present := results[scenario.ID]
		if !present {
			return nil, time.Time{}, fmt.Errorf(
				"P14 installed CLI scenario %q was not executed",
				scenario.ID,
			)
		}
		captures = append(captures, capture)
	}
	if latest.IsZero() {
		return nil, time.Time{}, fmt.Errorf("P14 installed CLI capture time is absent")
	}
	return captures, latest, nil
}

func p14InstalledCLIFamilyRunners() map[string]p14InstalledCLIFamilyRunner {
	runners := map[string]p14InstalledCLIFamilyRunner{
		p14RuntimeIdentityBuilderID:        runP14InstalledCLIRuntimeIdentity,
		p14FPFProjectionBuilderID:          runP14InstalledCLIFPFProjection,
		p14InitMatrixBuilderID:             runP14InstalledCLIInitMatrix,
		p14ExistingRecordBackfillBuilderID: runP14InstalledCLIExistingRecordBackfill,
	}
	for _, builderID := range p14CodeExploreBuilderIDs {
		runners[builderID] = runP14InstalledCLICodeExplore
	}
	for _, builderID := range p14AgentOrientationBuilderIDs {
		runners[builderID] = runP14InstalledCLIAgentOrientation
	}
	for _, builderID := range p14MemoryReadBuilderIDs {
		runners[builderID] = runP14InstalledCLIMemoryRead
	}
	for _, builderID := range p14MemoryOperationBuilderIDs {
		runners[builderID] = runP14InstalledCLIMemoryOperation
	}
	return runners
}

func p14PreparedSurfaceRequest(
	scenario preparedP14Scenario,
	surface string,
) (preparedP14Request, bool) {
	for _, request := range scenario.Requests {
		if request.Surface == surface {
			return request, true
		}
	}
	return preparedP14Request{}, false
}

func p14InstalledCLIObservation(
	scenario preparedP14Scenario,
	request preparedP14Request,
	result p14InstalledCLIFamilyResult,
) (p14InstalledSurfaceObservation, time.Time, error) {
	receipt := result.Receipt
	receipt.FailureDetail = result.FailureDetail
	receiptBytes, err := marshalP14CanonicalJSON(receipt)
	if err != nil {
		return p14InstalledSurfaceObservation{}, time.Time{}, err
	}
	observedAt, err := p14InstalledCLIReceiptTime(receipt)
	if err != nil {
		return p14InstalledSurfaceObservation{}, time.Time{}, err
	}
	outcome := p14SurfaceOutcomeObserved
	failureCode := ""
	normalizedDigest := ""
	if result.FailureCode != "" {
		outcome = p14SurfaceOutcomeMismatch
		failureCode = result.FailureCode
		if result.ExecutionError {
			outcome = p14SurfaceOutcomeExecutionError
		}
	} else if scenario.Oracle.Kind == "normalized_digest" {
		normalizedDigest = p14Digest(result.Normalized)
		if normalizedDigest != scenario.Oracle.ExpectedResultDigest {
			outcome = p14SurfaceOutcomeMismatch
			failureCode = "normalized_digest_mismatch"
			normalizedDigest = ""
		}
	}
	observation := p14InstalledSurfaceObservation{
		Surface:                request.Surface,
		RequestPayloadDigest:   request.PayloadDigest,
		Source:                 p14ObservationSourceInstalledCLI,
		SourceReceiptDigest:    p14Digest(receiptBytes),
		ObservedAt:             observedAt.UTC().Format(time.RFC3339Nano),
		Outcome:                outcome,
		ObservationCanonical:   string(receiptBytes),
		ObservationDigest:      p14Digest(receiptBytes),
		NormalizedResultDigest: normalizedDigest,
		FailureCode:            failureCode,
	}
	if err := validateP14InstalledSurfaceObservation(request, observation); err != nil {
		return p14InstalledSurfaceObservation{}, time.Time{}, err
	}
	return observation, observedAt, nil
}

func p14InstalledCLIReceiptTime(
	receipt p14InstalledCLIExecutionReceipt,
) (time.Time, error) {
	latest := time.Time{}
	for _, command := range receipt.Commands {
		finishedAt, err := time.Parse(time.RFC3339Nano, command.FinishedAt)
		if err != nil {
			return time.Time{}, fmt.Errorf(
				"P14 installed CLI receipt time: %w",
				err,
			)
		}
		if finishedAt.After(latest) {
			latest = finishedAt
		}
	}
	if latest.IsZero() {
		observedAt, err := time.Parse(time.RFC3339Nano, receipt.ObservedAt)
		if err != nil {
			return time.Time{}, fmt.Errorf(
				"P14 installed CLI receipt observation time: %w",
				err,
			)
		}
		return observedAt, nil
	}
	return latest, nil
}

func buildP14InstalledCLICaptureCarrier(
	contract requestOracleContract,
	preparedPath string,
	preparedDigest string,
	prepared preparedRequestOracleCarrier,
	execution p14InstalledCLIExecutionContext,
	capturedAt time.Time,
	captures []p14InstalledCLIScenarioCapture,
) (p14InstalledCLICaptureCarrier, error) {
	if err := verifyP14PreparedObservationBindingValue(
		preparedPath,
		preparedDigest,
		prepared,
	); err != nil {
		return p14InstalledCLICaptureCarrier{}, err
	}
	input := p14InstalledCLICaptureInput{
		Schema:          p14InstalledCLICaptureInputSchema,
		Status:          p14InstalledCLICaptureStatus,
		ResultSemantics: p14InstalledCLICaptureSemantics,
		ReleaseClaim:    false,
		PreparedCarrier: p14PreparedObservationBinding{
			CarrierPath:       preparedPath,
			CarrierDigest:     preparedDigest,
			PreparationDigest: prepared.PreparationDigest,
		},
		InstalledExecutablePath:   execution.ExecutablePath,
		InstalledExecutableDigest: execution.ExecutableDigest,
		CapturedAt:                capturedAt.UTC().Format(time.RFC3339Nano),
		ScenarioCaptures:          captures,
	}
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return p14InstalledCLICaptureCarrier{}, fmt.Errorf(
			"encode P14 installed CLI capture digest basis: %w",
			err,
		)
	}
	captureDigest := p14Digest(inputBytes)
	body := strings.TrimPrefix(captureDigest, "sha256:")
	path := filepath.ToSlash(filepath.Join(
		".context",
		"p14",
		"p14-installed-cli-capture-"+body[:16]+".json",
	))
	carrier := p14InstalledCLICaptureCarrier{
		Schema:        p14InstalledCLICaptureCarrierSchema,
		Status:        p14InstalledCLICaptureStatus,
		CarrierPath:   path,
		CaptureDigest: captureDigest,
		Capture:       input,
	}
	if err := validateP14InstalledCLICaptureCarrier(
		contract,
		prepared,
		carrier,
	); err != nil {
		return p14InstalledCLICaptureCarrier{}, err
	}
	return carrier, nil
}

func validateP14InstalledCLICaptureCarrier(
	contract requestOracleContract,
	prepared preparedRequestOracleCarrier,
	carrier p14InstalledCLICaptureCarrier,
) error {
	if carrier.Schema != p14InstalledCLICaptureCarrierSchema ||
		carrier.Status != p14InstalledCLICaptureStatus ||
		!validP14Digest(carrier.CaptureDigest) {
		return fmt.Errorf("P14 installed CLI capture header is invalid")
	}
	input := carrier.Capture
	if input.Schema != p14InstalledCLICaptureInputSchema ||
		input.Status != p14InstalledCLICaptureStatus ||
		input.ResultSemantics != p14InstalledCLICaptureSemantics ||
		input.ReleaseClaim ||
		!filepath.IsAbs(input.InstalledExecutablePath) ||
		input.InstalledExecutableDigest !=
			prepared.Preparation.FrozenBasis.Candidate.ExecutableDigest {
		return fmt.Errorf("P14 installed CLI capture basis is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, input.CapturedAt); err != nil {
		return fmt.Errorf("P14 installed CLI capture time is invalid: %w", err)
	}
	if err := validateP14PreparedObservationBinding(
		input.PreparedCarrier,
	); err != nil {
		return err
	}
	expected := p14InstalledCLIScenarios(prepared.Preparation.Scenarios)
	if len(input.ScenarioCaptures) != len(expected) {
		return fmt.Errorf("P14 installed CLI capture scenario count differs")
	}
	available := make(
		map[string]p14InstalledCLIScenarioCapture,
		len(input.ScenarioCaptures),
	)
	for index, capture := range input.ScenarioCaptures {
		scenario := expected[index]
		if capture.ID != scenario.ID ||
			capture.SemanticRequestDigest != scenario.SemanticRequestDigest {
			return fmt.Errorf(
				"P14 installed CLI capture scenario %q differs",
				capture.ID,
			)
		}
		available[capture.ID] = capture
	}
	for index, capture := range input.ScenarioCaptures {
		scenario := expected[index]
		request, present := p14PreparedSurfaceRequest(
			scenario,
			"installed_cli",
		)
		if !present ||
			capture.ID != scenario.ID ||
			capture.SemanticRequestDigest != scenario.SemanticRequestDigest {
			return fmt.Errorf(
				"P14 installed CLI capture scenario %q differs",
				capture.ID,
			)
		}
		if err := validateP14InstalledSurfaceObservation(
			request,
			capture.SurfaceObservation,
		); err != nil {
			return err
		}
		if err := validateP14InstalledCLIExecutionReceipt(
			scenario,
			request,
			prepared.Preparation.FrozenBasis,
			available,
			capture.SurfaceObservation,
		); err != nil {
			return err
		}
		if scenario.Oracle.Kind == "live_predicate" &&
			capture.SurfaceObservation.NormalizedResultDigest != "" {
			return fmt.Errorf(
				"P14 installed CLI live predicate carries a normalized result",
			)
		}
	}
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return err
	}
	wantDigest := p14Digest(inputBytes)
	body := strings.TrimPrefix(wantDigest, "sha256:")
	wantPath := filepath.ToSlash(filepath.Join(
		".context",
		"p14",
		"p14-installed-cli-capture-"+body[:16]+".json",
	))
	if carrier.CaptureDigest != wantDigest || carrier.CarrierPath != wantPath {
		return fmt.Errorf("P14 installed CLI capture path or digest differs")
	}
	canonical, err := json.Marshal(carrier)
	if err != nil {
		return err
	}
	if bytes.Contains(canonical, []byte(`"nonce"`)) {
		return fmt.Errorf("P14 installed CLI capture exposes a private nonce")
	}
	if len(contract.Scenarios) != len(prepared.Preparation.Scenarios) {
		return fmt.Errorf("P14 installed CLI capture uses another contract")
	}
	return nil
}

func validateP14InstalledCLIExecutionReceipt(
	scenario preparedP14Scenario,
	request preparedP14Request,
	basis frozenP14Basis,
	prior map[string]p14InstalledCLIScenarioCapture,
	observation p14InstalledSurfaceObservation,
) error {
	receipt := p14InstalledCLIExecutionReceipt{}
	if err := decodeP14StrictCompactJSON(
		observation.ObservationCanonical,
		&receipt,
		"installed CLI execution receipt",
	); err != nil {
		return err
	}
	if receipt.Schema != p14InstalledCLIReceiptSchema ||
		receipt.ScenarioID != scenario.ID ||
		receipt.Builder != request.Builder ||
		receipt.CandidateDigest != basis.Candidate.ExecutableDigest ||
		receipt.RequestPayloadDigest != request.PayloadDigest ||
		len(receipt.Checks) == 0 {
		return fmt.Errorf("P14 installed CLI execution receipt basis differs")
	}
	seenChecks := make(map[string]struct{}, len(receipt.Checks))
	allSatisfied := true
	for _, check := range receipt.Checks {
		if strings.TrimSpace(check.ID) == "" {
			return fmt.Errorf("P14 installed CLI execution receipt has a blank check")
		}
		if _, duplicate := seenChecks[check.ID]; duplicate {
			return fmt.Errorf("P14 installed CLI execution receipt repeats a check")
		}
		seenChecks[check.ID] = struct{}{}
		allSatisfied = allSatisfied && check.Satisfied
	}
	if observation.Outcome == p14SurfaceOutcomeObserved &&
		(!allSatisfied || receipt.FailureDetail != "") {
		return fmt.Errorf("P14 observed installed CLI receipt contains failure")
	}
	if observation.Outcome != p14SurfaceOutcomeObserved &&
		receipt.FailureDetail == "" {
		return fmt.Errorf("P14 failed installed CLI receipt omits failure detail")
	}
	if len(receipt.Commands) == 0 {
		if _, err := time.Parse(time.RFC3339Nano, receipt.ObservedAt); err != nil {
			return fmt.Errorf(
				"P14 commandless installed CLI receipt time is invalid: %w",
				err,
			)
		}
	} else if receipt.ObservedAt != "" {
		return fmt.Errorf("P14 command receipt duplicates its observation time")
	}
	seenCommands := make(map[string]struct{}, len(receipt.Commands))
	for _, command := range receipt.Commands {
		if err := validateP14InstalledCLICommandReceipt(command); err != nil {
			return err
		}
		if _, duplicate := seenCommands[command.ID]; duplicate {
			return fmt.Errorf("P14 installed CLI receipt repeats a command")
		}
		seenCommands[command.ID] = struct{}{}
	}
	if receipt.Fixture != nil {
		if err := validateP14InstalledCLIFixtureReceipt(*receipt.Fixture); err != nil {
			return err
		}
	}
	seenFixtures := make(map[string]struct{}, len(receipt.Fixtures))
	for _, fixture := range receipt.Fixtures {
		if err := validateP14InstalledCLIFixtureReceipt(fixture); err != nil {
			return err
		}
		if fixture.CaseID == "" {
			return fmt.Errorf("P14 installed CLI matrix fixture omits case ID")
		}
		if _, duplicate := seenFixtures[fixture.CaseID]; duplicate {
			return fmt.Errorf("P14 installed CLI matrix repeats a fixture")
		}
		seenFixtures[fixture.CaseID] = struct{}{}
	}
	return validateP14InstalledCLIReceiptEvidence(
		scenario,
		request,
		basis,
		prior,
		receipt,
		observation,
	)
}

func validateP14InstalledCLICommandReceipt(
	command p14InstalledCLICommandReceipt,
) error {
	startedAt, startErr := time.Parse(time.RFC3339Nano, command.StartedAt)
	finishedAt, finishErr := time.Parse(time.RFC3339Nano, command.FinishedAt)
	if strings.TrimSpace(command.ID) == "" ||
		len(command.Argv) == 0 ||
		startErr != nil ||
		finishErr != nil ||
		finishedAt.Before(startedAt) ||
		!validP14Digest(command.StdinDigest) ||
		!validP14Digest(command.StdoutDigest) ||
		!validP14Digest(command.StderrDigest) {
		return fmt.Errorf("P14 installed CLI command receipt is invalid")
	}
	stdout, err := base64.StdEncoding.DecodeString(command.StdoutBase64)
	if err != nil || p14Digest(stdout) != command.StdoutDigest {
		return fmt.Errorf("P14 installed CLI stdout receipt differs")
	}
	stderr, err := base64.StdEncoding.DecodeString(command.StderrBase64)
	if err != nil || p14Digest(stderr) != command.StderrDigest {
		return fmt.Errorf("P14 installed CLI stderr receipt differs")
	}
	if command.OutputLimited && command.ExecutionError !=
		"command_output_limit" {
		return fmt.Errorf("P14 installed CLI output limit is not explicit")
	}
	return nil
}

func validateP14InstalledCLIFixtureReceipt(
	fixture p14InstalledCLIFixtureReceipt,
) error {
	digests := []string{
		fixture.ProjectBasisDigest,
		fixture.HomeTemplateDigest,
		fixture.ProjectBeforeDigest,
		fixture.ProjectAfterDigest,
		fixture.HomeBeforeDigest,
		fixture.HomeAfterDigest,
	}
	if strings.TrimSpace(fixture.Isolation) == "" ||
		!allP14Digests(digests) {
		return fmt.Errorf("P14 installed CLI fixture receipt is invalid")
	}
	return nil
}

func p14InstalledCLIScenarios(
	scenarios []preparedP14Scenario,
) []preparedP14Scenario {
	result := make([]preparedP14Scenario, 0)
	for _, scenario := range scenarios {
		if _, present := p14PreparedSurfaceRequest(
			scenario,
			"installed_cli",
		); present {
			result = append(result, scenario)
		}
	}
	return result
}

func persistP14InstalledCLICaptureCarrier(
	repositoryRoot string,
	contract requestOracleContract,
	carrier p14InstalledCLICaptureCarrier,
) (string, string, error) {
	preparedPath := filepath.Join(
		repositoryRoot,
		filepath.FromSlash(carrier.Capture.PreparedCarrier.CarrierPath),
	)
	preparedRaw, err := os.ReadFile(preparedPath)
	if err != nil {
		return "", "", fmt.Errorf(
			"read prepared carrier for installed CLI capture: %w",
			err,
		)
	}
	if p14Digest(preparedRaw) !=
		carrier.Capture.PreparedCarrier.CarrierDigest {
		return "", "", fmt.Errorf(
			"prepared carrier changed before installed CLI capture persistence",
		)
	}
	prepared, err := decodePreparedRequestOracleCarrier(contract, preparedRaw)
	if err != nil {
		return "", "", err
	}
	if err := validateP14InstalledCLICaptureCarrier(
		contract,
		prepared,
		carrier,
	); err != nil {
		return "", "", err
	}
	canonical, err := json.MarshalIndent(carrier, "", "  ")
	if err != nil {
		return "", "", err
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

func loadP14PreparedCarrierForExecution(
	repositoryRoot string,
	contract requestOracleContract,
	path string,
) (preparedRequestOracleCarrier, string, error) {
	clean, err := resolveP14ExecutionCarrierPath(
		repositoryRoot,
		path,
		"p14-prepared-request-oracle-",
	)
	if err != nil {
		return preparedRequestOracleCarrier{}, "", err
	}
	raw, err := os.ReadFile(clean)
	if err != nil {
		return preparedRequestOracleCarrier{}, "", fmt.Errorf(
			"read P14 prepared execution carrier: %w",
			err,
		)
	}
	carrier, err := decodePreparedRequestOracleCarrier(contract, raw)
	if err != nil {
		return preparedRequestOracleCarrier{}, "", err
	}
	return carrier, p14Digest(raw), nil
}

func resolveP14ExecutionCarrierPath(
	repositoryRoot string,
	path string,
	prefix string,
) (string, error) {
	candidate := path
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(repositoryRoot, filepath.FromSlash(candidate))
	}
	canonicalRoot, err := filepath.EvalSymlinks(repositoryRoot)
	if err != nil {
		return "", err
	}
	canonicalPath, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(canonicalRoot, canonicalPath)
	if err != nil {
		return "", err
	}
	if filepath.Dir(relative) != filepath.Join(".context", "p14") ||
		!strings.HasPrefix(filepath.Base(relative), prefix) {
		return "", fmt.Errorf("P14 execution carrier path is outside its closed directory")
	}
	return canonicalPath, nil
}

func canonicalP14InstalledExecutable(
	path string,
	expectedDigest string,
) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("P14 installed executable path must be absolute")
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("canonicalize P14 installed executable: %w", err)
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("P14 installed executable is not executable")
	}
	if err := verifyP14FileDigest(canonical, expectedDigest); err != nil {
		return "", err
	}
	return canonical, nil
}

func executeP14InstalledCLIProcess(
	parent context.Context,
	request p14InstalledCLIProcessRequest,
) p14InstalledCLIProcessResult {
	timeout := request.Timeout
	if timeout <= 0 {
		timeout = p14InstalledCLICommandTimeout
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	startedAt := time.Now().UTC()
	command := exec.CommandContext(ctx, request.Executable, request.Argv...)
	command.Dir = request.Directory
	command.Env = slices.Clone(request.Environment)
	command.Stdin = strings.NewReader(request.Stdin)
	stdout := &p14InstalledCLILimitedBuffer{limit: p14InstalledCLIOutputLimit}
	stderr := &p14InstalledCLILimitedBuffer{limit: p14InstalledCLIOutputLimit}
	command.Stdout = stdout
	command.Stderr = stderr
	runErr := command.Run()
	finishedAt := time.Now().UTC()
	exitCode := 0
	executionError := ""
	if runErr != nil {
		var exitError *exec.ExitError
		if errors.As(runErr, &exitError) {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1
			executionError = boundedP14InstalledCLIError(runErr)
		}
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		executionError = "command_timeout"
	}
	if stdout.exceeded || stderr.exceeded {
		executionError = "command_output_limit"
	}
	return p14InstalledCLIProcessResult{
		ID:             request.ID,
		ParallelGroup:  request.ParallelGroup,
		Argv:           slices.Clone(request.Argv),
		StdinDigest:    p14Digest([]byte(request.Stdin)),
		StartedAt:      startedAt,
		FinishedAt:     finishedAt,
		ExitCode:       exitCode,
		Stdout:         stdout.Bytes(),
		Stderr:         stderr.Bytes(),
		OutputLimited:  stdout.exceeded || stderr.exceeded,
		ExecutionError: executionError,
	}
}

type p14InstalledCLILimitedBuffer struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
}

func (buffer *p14InstalledCLILimitedBuffer) Write(content []byte) (int, error) {
	remaining := buffer.limit - buffer.buffer.Len()
	if remaining > 0 {
		toWrite := content
		if len(toWrite) > remaining {
			toWrite = toWrite[:remaining]
		}
		_, _ = buffer.buffer.Write(toWrite)
	}
	if len(content) > remaining {
		buffer.exceeded = true
	}
	return len(content), nil
}

func (buffer *p14InstalledCLILimitedBuffer) Bytes() []byte {
	return slices.Clone(buffer.buffer.Bytes())
}

func p14InstalledCLICommandReceiptFromResult(
	result p14InstalledCLIProcessResult,
) p14InstalledCLICommandReceipt {
	return p14InstalledCLICommandReceipt{
		ID:             result.ID,
		ParallelGroup:  result.ParallelGroup,
		Argv:           slices.Clone(result.Argv),
		StdinDigest:    result.StdinDigest,
		StartedAt:      result.StartedAt.UTC().Format(time.RFC3339Nano),
		FinishedAt:     result.FinishedAt.UTC().Format(time.RFC3339Nano),
		ExitCode:       result.ExitCode,
		StdoutBase64:   base64.StdEncoding.EncodeToString(result.Stdout),
		StdoutDigest:   p14Digest(result.Stdout),
		StderrBase64:   base64.StdEncoding.EncodeToString(result.Stderr),
		StderrDigest:   p14Digest(result.Stderr),
		OutputLimited:  result.OutputLimited,
		ExecutionError: result.ExecutionError,
	}
}

func p14ExecuteInstalledCLICalls(
	ctx context.Context,
	execution p14InstalledCLIExecutionContext,
	directory string,
	environment []string,
	calls []p14InstalledCLIProcessRequest,
) []p14InstalledCLIProcessResult {
	results := make([]p14InstalledCLIProcessResult, len(calls))
	for index := 0; index < len(calls); {
		group := calls[index].ParallelGroup
		if group == "" {
			request := p14BindInstalledCLIProcessRequest(
				execution,
				directory,
				environment,
				calls[index],
			)
			results[index] = execution.Executor(ctx, request)
			index++
			continue
		}
		end := index
		for end < len(calls) && calls[end].ParallelGroup == group {
			end++
		}
		var wait sync.WaitGroup
		for current := index; current < end; current++ {
			current := current
			wait.Add(1)
			go func() {
				defer wait.Done()
				request := p14BindInstalledCLIProcessRequest(
					execution,
					directory,
					environment,
					calls[current],
				)
				results[current] = execution.Executor(ctx, request)
			}()
		}
		wait.Wait()
		index = end
	}
	return results
}

func p14BindInstalledCLIProcessRequest(
	execution p14InstalledCLIExecutionContext,
	directory string,
	environment []string,
	request p14InstalledCLIProcessRequest,
) p14InstalledCLIProcessRequest {
	bound := request
	bound.Executable = execution.ExecutablePath
	bound.Directory = directory
	bound.Environment = slices.Clone(environment)
	if bound.Timeout <= 0 {
		bound.Timeout = p14InstalledCLICommandTimeout
	}
	return bound
}

func p14InstalledCLIEnvironment(
	overrides map[string]string,
) []string {
	excluded := map[string]struct{}{
		"HAFT_PROJECT_ROOT":        {},
		"QUINT_PROJECT_ROOT":       {},
		"HAFT_EXPECTED_PROJECT_ID": {},
		"HOME":                     {},
	}
	environment := make([]string, 0, len(os.Environ())+len(overrides))
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if _, remove := excluded[key]; remove {
			continue
		}
		environment = append(environment, item)
	}
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		environment = append(environment, key+"="+overrides[key])
	}
	return environment
}

func cloneP14InstalledCLIHomeFixture(
	workspaceRoot string,
	scenarioID string,
	caseID string,
	homeTemplateRoot string,
	homeTemplateDigest string,
) (string, error) {
	homeObserved, err := observeP14InitTree(homeTemplateRoot)
	if err != nil {
		return "", err
	}
	if homeObserved != homeTemplateDigest {
		return "", fmt.Errorf("P14 installed CLI home template changed")
	}
	caseRoot := filepath.Join(workspaceRoot, scenarioID, caseID)
	homeRoot := filepath.Join(caseRoot, "home")
	if err := copyP14InstalledCLITree(homeTemplateRoot, homeRoot); err != nil {
		return "", err
	}
	homeCloneDigest, err := observeP14InitTree(homeRoot)
	if err != nil {
		return "", err
	}
	if homeCloneDigest != homeTemplateDigest {
		return "", fmt.Errorf("P14 installed CLI home fixture clone differs")
	}
	return homeRoot, nil
}

func beginP14InstalledCLIReadOnlyFixture(
	execution p14InstalledCLIExecutionContext,
	scenarioID string,
) (p14InstalledCLIReadOnlyFixture, error) {
	binding, err := preparedP14BindingByGroup(
		execution.Prepared.Preparation.Bindings,
		"golden_memory_fixture",
	)
	if err != nil {
		return p14InstalledCLIReadOnlyFixture{}, err
	}
	fixturePath := filepath.Join(
		execution.RepositoryRoot,
		filepath.FromSlash(binding.CarrierPath),
	)
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		return p14InstalledCLIReadOnlyFixture{}, fmt.Errorf(
			"read P14 installed CLI read-only fixture: %w",
			err,
		)
	}
	if p14Digest(raw) != binding.CarrierDigest {
		return p14InstalledCLIReadOnlyFixture{}, fmt.Errorf(
			"P14 installed CLI read-only fixture digest changed",
		)
	}
	fixture, err := decodeP14MemoryReadFixture(raw)
	if err != nil {
		return p14InstalledCLIReadOnlyFixture{}, err
	}
	if err := validateP14MemoryReadFixtureShape(fixture); err != nil {
		return p14InstalledCLIReadOnlyFixture{}, err
	}
	project := execution.Prepared.Preparation.FrozenBasis.SelectedProject
	operations := fixture.Operations
	if operations.SelectedProjectRoot != project.ProjectRoot {
		return p14InstalledCLIReadOnlyFixture{}, fmt.Errorf(
			"P14 installed CLI read-only fixture uses another project",
		)
	}
	projectBefore, err := observeP14SelectedProjectMemoryBasis(
		project.ProjectRoot,
	)
	if err != nil {
		return p14InstalledCLIReadOnlyFixture{}, err
	}
	if projectBefore != operations.SelectedProjectBasisDigest {
		return p14InstalledCLIReadOnlyFixture{}, fmt.Errorf(
			"P14 installed CLI read-only project basis changed",
		)
	}
	homeRoot, err := cloneP14InstalledCLIHomeFixture(
		execution.WorkspaceRoot,
		scenarioID,
		"read-only",
		operations.HomeTemplateRoot,
		operations.HomeTemplateDigest,
	)
	if err != nil {
		return p14InstalledCLIReadOnlyFixture{}, err
	}
	homeBefore, err := observeP14InstalledCLISemanticTree(homeRoot)
	if err != nil {
		return p14InstalledCLIReadOnlyFixture{}, err
	}
	if homeBefore != operations.HomeTemplateDigest {
		return p14InstalledCLIReadOnlyFixture{}, fmt.Errorf(
			"P14 installed CLI read-only home clone changed before execution",
		)
	}
	return p14InstalledCLIReadOnlyFixture{
		ProjectRoot:        project.ProjectRoot,
		HomeRoot:           homeRoot,
		ProjectBasisDigest: operations.SelectedProjectBasisDigest,
		HomeTemplateDigest: operations.HomeTemplateDigest,
		ProjectBefore:      projectBefore,
		HomeBefore:         homeBefore,
	}, nil
}

func finishP14InstalledCLIReadOnlyFixture(
	fixture p14InstalledCLIReadOnlyFixture,
) (p14InstalledCLIFixtureReceipt, error) {
	projectAfter, err := observeP14SelectedProjectMemoryBasis(
		fixture.ProjectRoot,
	)
	if err != nil {
		return p14InstalledCLIFixtureReceipt{}, err
	}
	homeAfter, err := observeP14InstalledCLISemanticTree(fixture.HomeRoot)
	if err != nil {
		return p14InstalledCLIFixtureReceipt{}, err
	}
	return p14InstalledCLIFixtureReceipt{
		Isolation:           p14InstalledCLIReadOnlyIsolation,
		ProjectBasisDigest:  fixture.ProjectBasisDigest,
		HomeTemplateDigest:  fixture.HomeTemplateDigest,
		ProjectBeforeDigest: fixture.ProjectBefore,
		ProjectAfterDigest:  projectAfter,
		HomeBeforeDigest:    fixture.HomeBefore,
		HomeAfterDigest:     homeAfter,
	}, nil
}

func validateP14InstalledCLIReadOnlyFixture(
	fixture p14InstalledCLIFixtureReceipt,
) error {
	if fixture.Isolation != p14InstalledCLIReadOnlyIsolation ||
		fixture.ProjectBeforeDigest != fixture.ProjectBasisDigest ||
		fixture.HomeBeforeDigest != fixture.HomeTemplateDigest ||
		fixture.ProjectBeforeDigest != fixture.ProjectAfterDigest ||
		fixture.HomeBeforeDigest != fixture.HomeAfterDigest {
		return fmt.Errorf(
			"P14 installed CLI read-only project or HOME basis changed",
		)
	}
	return nil
}

func restoreP14InstalledCLIInitFixture(
	workspaceRoot string,
	scenarioID string,
	caseID string,
	projectTemplateRoot string,
	projectTemplateDigest string,
	homeTemplateRoot string,
	homeTemplateDigest string,
	projectExecutionRoot string,
	homeExecutionRoot string,
) (string, string, error) {
	expectedProjectRoot, expectedHomeRoot := p14InstalledCLIInitExecutionRoots(
		workspaceRoot,
		scenarioID,
		caseID,
	)
	if projectExecutionRoot != expectedProjectRoot ||
		homeExecutionRoot != expectedHomeRoot {
		return "", "", fmt.Errorf(
			"P14 installed CLI init execution roots differ from the sealed workspace",
		)
	}
	projectObserved, err := observeP14InitTree(projectTemplateRoot)
	if err != nil {
		return "", "", err
	}
	homeObserved, err := observeP14InitTree(homeTemplateRoot)
	if err != nil {
		return "", "", err
	}
	if projectObserved != projectTemplateDigest ||
		homeObserved != homeTemplateDigest {
		return "", "", fmt.Errorf("P14 installed CLI init template changed")
	}
	if p14InstalledCLIPathExists(projectExecutionRoot) ||
		p14InstalledCLIPathExists(homeExecutionRoot) {
		return "", "", fmt.Errorf(
			"P14 installed CLI init execution root is not fresh",
		)
	}
	if err := copyP14InstalledCLITree(
		projectTemplateRoot,
		projectExecutionRoot,
	); err != nil {
		return "", "", err
	}
	if err := copyP14InstalledCLITree(
		homeTemplateRoot,
		homeExecutionRoot,
	); err != nil {
		return "", "", err
	}
	projectCloneDigest, err := observeP14InitTree(projectExecutionRoot)
	if err != nil {
		return "", "", err
	}
	homeCloneDigest, err := observeP14InitTree(homeExecutionRoot)
	if err != nil {
		return "", "", err
	}
	if projectCloneDigest != projectTemplateDigest ||
		homeCloneDigest != homeTemplateDigest {
		return "", "", fmt.Errorf(
			"P14 installed CLI init fixture restore differs",
		)
	}
	return projectExecutionRoot, homeExecutionRoot, nil
}

func p14InstalledCLIInitExecutionRoots(
	workspaceRoot string,
	scenarioID string,
	caseID string,
) (string, string) {
	caseRoot := filepath.Join(workspaceRoot, scenarioID, caseID)
	return filepath.Join(caseRoot, "project"), filepath.Join(caseRoot, "home")
}

func p14InstalledCLIWorkspaceRoot(
	candidateDigest string,
) (string, error) {
	if !validP14Digest(candidateDigest) {
		return "", fmt.Errorf("P14 installed CLI candidate digest is invalid")
	}
	temporaryRoot, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return "", fmt.Errorf("resolve P14 temporary root: %w", err)
	}
	digestBody := strings.TrimPrefix(candidateDigest, "sha256:")
	return filepath.Join(
		temporaryRoot,
		"haft-p14-installed-cli",
		digestBody,
	), nil
}

func prepareP14InstalledCLIWorkspace(
	candidateDigest string,
) (string, error) {
	workspaceRoot, err := p14InstalledCLIWorkspaceRoot(candidateDigest)
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(workspaceRoot); err != nil {
		return "", fmt.Errorf("reset P14 installed CLI workspace: %w", err)
	}
	if err := os.MkdirAll(workspaceRoot, 0o700); err != nil {
		return "", fmt.Errorf("create P14 installed CLI workspace: %w", err)
	}
	return workspaceRoot, nil
}

func copyP14InstalledCLITree(source string, destination string) error {
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(
		path string,
		entry fs.DirEntry,
		walkErr error,
	) error {
		if walkErr != nil {
			return walkErr
		}
		if path == source {
			return nil
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.Mkdir(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("P14 fixture contains non-regular path %s", path)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(
			target,
			os.O_CREATE|os.O_EXCL|os.O_WRONLY,
			info.Mode().Perm(),
		)
		if err != nil {
			return errors.Join(err, input.Close())
		}
		_, copyErr := io.Copy(output, input)
		inputCloseErr := input.Close()
		closeErr := output.Close()
		return errors.Join(copyErr, inputCloseErr, closeErr)
	})
}

func observeP14InstalledCLISemanticTree(root string) (string, error) {
	entries := make([]p14InitTreeEntry, 0)
	err := filepath.WalkDir(root, func(
		path string,
		entry fs.DirEntry,
		walkErr error,
	) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), "-shm") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		item := p14InitTreeEntry{
			Path: filepath.ToSlash(relative),
			Mode: uint32(info.Mode().Perm()),
		}
		if entry.IsDir() {
			item.Kind = "directory"
			entries = append(entries, item)
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("P14 fixture contains non-regular path %s", path)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		item.Kind = "file"
		item.Digest = p14Digest(content)
		entries = append(entries, item)
		return nil
	})
	if err != nil {
		return "", err
	}
	slices.SortFunc(entries, func(left p14InitTreeEntry, right p14InitTreeEntry) int {
		return strings.Compare(left.Path, right.Path)
	})
	raw, err := marshalP14CanonicalJSON(entries)
	if err != nil {
		return "", err
	}
	return p14Digest(raw), nil
}

func observeP14SelectedProjectMemoryBasis(
	projectRoot string,
) (string, error) {
	relativeRoots := []string{
		"project.yaml",
		"config.yaml",
		"project-profile.yaml",
		"problems",
		"decisions",
		"notes",
		"solutions",
		"specs",
		"evidence",
		"commissions",
	}
	entries := make([]p14InitTreeEntry, 0)
	for _, relativeRoot := range relativeRoots {
		path := filepath.Join(projectRoot, ".haft", relativeRoot)
		_, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		walkErr := filepath.WalkDir(path, func(
			currentPath string,
			entry fs.DirEntry,
			entryErr error,
		) error {
			if entryErr != nil {
				return entryErr
			}
			relative, err := filepath.Rel(projectRoot, currentPath)
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			item := p14InitTreeEntry{
				Path: filepath.ToSlash(relative),
				Mode: uint32(info.Mode().Perm()),
			}
			if entry.IsDir() {
				item.Kind = "directory"
				entries = append(entries, item)
				return nil
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf(
					"P14 selected-project memory basis contains non-regular path %s",
					currentPath,
				)
			}
			content, err := os.ReadFile(currentPath)
			if err != nil {
				return err
			}
			item.Kind = "file"
			item.Digest = p14Digest(content)
			entries = append(entries, item)
			return nil
		})
		if walkErr != nil {
			return "", walkErr
		}
	}
	slices.SortFunc(entries, func(
		left p14InitTreeEntry,
		right p14InitTreeEntry,
	) int {
		return strings.Compare(left.Path, right.Path)
	})
	raw, err := marshalP14CanonicalJSON(entries)
	if err != nil {
		return "", err
	}
	return p14Digest(raw), nil
}

func p14InstalledCLIProcessResultsHaveExecutionFailure(
	results []p14InstalledCLIProcessResult,
) bool {
	for _, result := range results {
		if result.ExecutionError != "" || result.OutputLimited {
			return true
		}
	}
	return false
}

func p14InstalledCLICommandReceipts(
	results []p14InstalledCLIProcessResult,
) []p14InstalledCLICommandReceipt {
	receipts := make([]p14InstalledCLICommandReceipt, 0, len(results))
	for _, result := range results {
		receipts = append(
			receipts,
			p14InstalledCLICommandReceiptFromResult(result),
		)
	}
	return receipts
}

func boundedP14InstalledCLIError(err error) string {
	if err == nil {
		return ""
	}
	text := strings.TrimSpace(err.Error())
	if len(text) > 512 {
		text = text[:512]
	}
	return text
}

func decodeP14StrictCompactJSON(
	raw string,
	target any,
	label string,
) error {
	content := []byte(raw)
	if !canonicalCompactJSON(content) {
		return fmt.Errorf("P14 %s is not compact canonical JSON", label)
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode P14 %s: %w", label, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("P14 %s has trailing JSON", label)
	}
	return nil
}

func TestP14InstalledCLIRunnerRegistryCoversEveryDeclaredSurface(
	t *testing.T,
) {
	root, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, _, err := loadRequestOracleContract(root)
	if err != nil {
		t.Fatal(err)
	}
	runners := p14InstalledCLIFamilyRunners()
	for _, declared := range contract.Scenarios {
		if !slices.Contains(declared.Surfaces, "installed_cli") {
			continue
		}
		if runners[declared.RequestBuilder] == nil {
			t.Fatalf(
				"P14 installed CLI builder %q has no runner",
				declared.RequestBuilder,
			)
		}
	}
}

func TestP14InstalledCLIReceiptValidationDerivesResultFromRawEvidence(
	t *testing.T,
) {
	root, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, rawContract, err := loadRequestOracleContract(root)
	if err != nil {
		t.Fatal(err)
	}
	input, err := completePreparedInputForTest(
		contract,
		p14Digest(rawContract),
	)
	if err != nil {
		t.Fatal(err)
	}
	prepared := preparedRequestOracleCarrier{Preparation: input}
	scenario, err := preparedP14ScenarioByID(
		prepared.Preparation.Scenarios,
		"code_graph_exact_explore",
	)
	if err != nil {
		t.Fatal(err)
	}
	request, present := p14PreparedSurfaceRequest(
		scenario,
		"installed_cli",
	)
	if !present {
		t.Fatal("P14 code Explore scenario lost installed CLI")
	}
	observedAt := time.Unix(1_750_000_000, 0).UTC()
	processResult, err := syntheticP14InstalledCLICodeExploreResult(
		scenario,
		observedAt,
	)
	if err != nil {
		t.Fatal(err)
	}
	processResult.StdinDigest = p14Digest(nil)
	projectDigest := p14TestDigest("installed-cli-read-only-project")
	homeDigest := p14TestDigest("installed-cli-read-only-home")
	receipt := p14InstalledCLIExecutionReceipt{
		Schema:               p14InstalledCLIReceiptSchema,
		ScenarioID:           scenario.ID,
		Builder:              request.Builder,
		CandidateDigest:      prepared.Preparation.FrozenBasis.Candidate.ExecutableDigest,
		RequestPayloadDigest: request.PayloadDigest,
		Fixture: &p14InstalledCLIFixtureReceipt{
			Isolation:           p14InstalledCLIReadOnlyIsolation,
			ProjectBasisDigest:  projectDigest,
			HomeTemplateDigest:  homeDigest,
			ProjectBeforeDigest: projectDigest,
			ProjectAfterDigest:  projectDigest,
			HomeBeforeDigest:    homeDigest,
			HomeAfterDigest:     homeDigest,
		},
		Commands: p14InstalledCLICommandReceipts(
			[]p14InstalledCLIProcessResult{processResult},
		),
		Checks: p14InstalledCLIChecks(
			true,
			"exact_graph_argv_from_sealed_payload",
			"closed_code_explore_normalizer",
		),
	}
	normalized, err := recomputeP14InstalledCLINormalizedResult(
		scenario,
		request,
		prepared.Preparation.FrozenBasis,
		receipt,
	)
	if err != nil {
		t.Fatal(err)
	}
	if p14Digest(normalized) != scenario.Oracle.ExpectedResultDigest {
		t.Fatal("P14 synthetic installed CLI result differs from sealed oracle")
	}
	observation := p14InstalledCLITestObservation(
		t,
		request,
		receipt,
		p14Digest(normalized),
		observedAt,
	)
	if err := validateP14InstalledCLIExecutionReceipt(
		scenario,
		request,
		prepared.Preparation.FrozenBasis,
		nil,
		observation,
	); err != nil {
		t.Fatal(err)
	}

	t.Run("commandless copied oracle is rejected", func(t *testing.T) {
		tampered := receipt
		tampered.Commands = nil
		observation := p14InstalledCLITestObservation(
			t,
			request,
			tampered,
			scenario.Oracle.ExpectedResultDigest,
			observedAt,
		)
		if err := validateP14InstalledCLIExecutionReceipt(
			scenario,
			request,
			prepared.Preparation.FrozenBasis,
			nil,
			observation,
		); err == nil {
			t.Fatal("P14 installed CLI accepted a commandless copied oracle")
		}
	})

	t.Run("raw output cannot be replaced by the oracle digest", func(t *testing.T) {
		tampered := receipt
		command := tampered.Commands[0]
		command.StdoutBase64 = base64.StdEncoding.EncodeToString(
			[]byte(`{"fabricated":true}`),
		)
		command.StdoutDigest = p14Digest([]byte(`{"fabricated":true}`))
		tampered.Commands = []p14InstalledCLICommandReceipt{command}
		observation := p14InstalledCLITestObservation(
			t,
			request,
			tampered,
			scenario.Oracle.ExpectedResultDigest,
			observedAt,
		)
		if err := validateP14InstalledCLIExecutionReceipt(
			scenario,
			request,
			prepared.Preparation.FrozenBasis,
			nil,
			observation,
		); err == nil {
			t.Fatal("P14 installed CLI accepted fabricated raw stdout")
		}
	})

	t.Run("arbitrary satisfied checks are rejected", func(t *testing.T) {
		tampered := receipt
		tampered.Checks = p14InstalledCLIChecks(
			true,
			"synthetic_exact_receipt",
		)
		observation := p14InstalledCLITestObservation(
			t,
			request,
			tampered,
			scenario.Oracle.ExpectedResultDigest,
			observedAt,
		)
		if err := validateP14InstalledCLIExecutionReceipt(
			scenario,
			request,
			prepared.Preparation.FrozenBasis,
			nil,
			observation,
		); err == nil {
			t.Fatal("P14 installed CLI accepted arbitrary satisfied checks")
		}
	})

	t.Run("HOME mutation is rejected despite zero-write prose", func(t *testing.T) {
		tampered := receipt
		fixture := *tampered.Fixture
		fixture.HomeAfterDigest = p14TestDigest(
			"installed-cli-mutated-home",
		)
		tampered.Fixture = &fixture
		observation := p14InstalledCLITestObservation(
			t,
			request,
			tampered,
			scenario.Oracle.ExpectedResultDigest,
			observedAt,
		)
		if err := validateP14InstalledCLIExecutionReceipt(
			scenario,
			request,
			prepared.Preparation.FrozenBasis,
			nil,
			observation,
		); err == nil {
			t.Fatal("P14 installed CLI accepted a changed HOME snapshot")
		}
	})

	t.Run("commandless live predicate requires exact prior captures", func(t *testing.T) {
		runtimeScenario, err := preparedP14ScenarioByID(
			prepared.Preparation.Scenarios,
			"runtime_identity",
		)
		if err != nil {
			t.Fatal(err)
		}
		runtimeRequest, present := p14PreparedSurfaceRequest(
			runtimeScenario,
			"installed_cli",
		)
		if !present {
			t.Fatal("P14 runtime identity lost installed CLI")
		}
		runtimeReceipt := p14InstalledCLIExecutionReceipt{
			Schema:               p14InstalledCLIReceiptSchema,
			ScenarioID:           runtimeScenario.ID,
			Builder:              runtimeRequest.Builder,
			CandidateDigest:      prepared.Preparation.FrozenBasis.Candidate.ExecutableDigest,
			RequestPayloadDigest: runtimeRequest.PayloadDigest,
			ObservedAt:           observedAt.Format(time.RFC3339Nano),
			Checks: p14InstalledCLIChecks(
				true,
				"candidate_path_and_digest_match",
				"frozen_query_and_typeenv_basis_match",
			),
		}
		runtimeObservation := p14InstalledCLITestObservation(
			t,
			runtimeRequest,
			runtimeReceipt,
			"",
			observedAt,
		)
		if err := validateP14InstalledCLIExecutionReceipt(
			runtimeScenario,
			runtimeRequest,
			prepared.Preparation.FrozenBasis,
			nil,
			runtimeObservation,
		); err == nil {
			t.Fatal(
				"P14 installed CLI accepted a self-attested commandless predicate",
			)
		}
	})
}

func p14InstalledCLITestObservation(
	t *testing.T,
	request preparedP14Request,
	receipt p14InstalledCLIExecutionReceipt,
	normalizedDigest string,
	observedAt time.Time,
) p14InstalledSurfaceObservation {
	t.Helper()
	raw, err := marshalP14CanonicalJSON(receipt)
	if err != nil {
		t.Fatal(err)
	}
	digest := p14Digest(raw)
	return p14InstalledSurfaceObservation{
		Surface:                "installed_cli",
		RequestPayloadDigest:   request.PayloadDigest,
		Source:                 p14ObservationSourceInstalledCLI,
		SourceReceiptDigest:    digest,
		ObservedAt:             observedAt.Format(time.RFC3339Nano),
		Outcome:                p14SurfaceOutcomeObserved,
		ObservationCanonical:   string(raw),
		ObservationDigest:      digest,
		NormalizedResultDigest: normalizedDigest,
	}
}

func TestP14InstalledCLIFixtureLifecyclePreservesProjectIdentity(
	t *testing.T,
) {
	t.Run("init restores a root-bound ledger at its sealed roots", func(t *testing.T) {
		temporaryRoot, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		workspaceRoot := filepath.Join(temporaryRoot, "workspace")
		projectExecutionRoot, homeExecutionRoot :=
			p14InstalledCLIInitExecutionRoots(
				workspaceRoot,
				p14InitMatrixScenarioID,
				"software",
			)
		harness := profileadmissionfixture.New(t, projectExecutionRoot)
		harness.AdmitSoftwareRevision(t, "p14-root-binding")
		databasePath := harness.DatabasePath()
		if err := harness.Close(); err != nil {
			t.Fatal(err)
		}
		snapshotPath := filepath.Join(temporaryRoot, "snapshot", "haft.db")
		if err := os.MkdirAll(filepath.Dir(snapshotPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := snapshotP14SQLiteDatabase(
			databasePath,
			snapshotPath,
		); err != nil {
			t.Fatal(err)
		}
		projectDatabaseRoot := filepath.Dir(databasePath)
		projectsRoot := filepath.Dir(projectDatabaseRoot)
		haftHomeRoot := filepath.Dir(projectsRoot)
		sourceHomeRoot := filepath.Dir(haftHomeRoot)
		projectTemplateRoot := filepath.Join(
			temporaryRoot,
			"templates",
			"project",
		)
		homeTemplateRoot := filepath.Join(
			temporaryRoot,
			"templates",
			"home",
		)
		if err := copyP14InstalledCLITree(
			projectExecutionRoot,
			projectTemplateRoot,
		); err != nil {
			t.Fatal(err)
		}
		if err := copyP14InstalledCLITree(
			sourceHomeRoot,
			homeTemplateRoot,
		); err != nil {
			t.Fatal(err)
		}
		projectTemplateDigest, err := observeP14InitTree(projectTemplateRoot)
		if err != nil {
			t.Fatal(err)
		}
		homeTemplateDigest, err := observeP14InitTree(homeTemplateRoot)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(projectExecutionRoot); err != nil {
			t.Fatal(err)
		}
		projectRoot, homeRoot, err := restoreP14InstalledCLIInitFixture(
			workspaceRoot,
			p14InitMatrixScenarioID,
			"software",
			projectTemplateRoot,
			projectTemplateDigest,
			homeTemplateRoot,
			homeTemplateDigest,
			projectExecutionRoot,
			homeExecutionRoot,
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", homeRoot)
		handle, err := projectledger.OpenExisting(
			context.Background(),
			projectRoot,
			projectledger.ReadOnly,
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := handle.Close(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("memory clones only home and keeps selected project read-only", func(t *testing.T) {
		temporaryRoot, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		selectedProjectRoot := filepath.Join(
			temporaryRoot,
			"selected-project",
		)
		projectIdentityPath := filepath.Join(
			selectedProjectRoot,
			".haft",
			"project.yaml",
		)
		if err := os.MkdirAll(filepath.Dir(projectIdentityPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			projectIdentityPath,
			[]byte("id: qnt_p14_memory\nname: p14-memory\n"),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
		projectBefore, err := observeP14SelectedProjectMemoryBasis(
			selectedProjectRoot,
		)
		if err != nil {
			t.Fatal(err)
		}
		homeTemplateRoot := filepath.Join(
			temporaryRoot,
			"home-template",
		)
		homeSeedPath := filepath.Join(homeTemplateRoot, ".haft", "seed.json")
		if err := os.MkdirAll(filepath.Dir(homeSeedPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(homeSeedPath, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		homeTemplateDigest, err := observeP14InitTree(homeTemplateRoot)
		if err != nil {
			t.Fatal(err)
		}
		workspaceRoot := filepath.Join(temporaryRoot, "memory-workspace")
		homeRoot, err := cloneP14InstalledCLIHomeFixture(
			workspaceRoot,
			"positive_typed_write",
			"scenario",
			homeTemplateRoot,
			homeTemplateDigest,
		)
		if err != nil {
			t.Fatal(err)
		}
		projectAfter, err := observeP14SelectedProjectMemoryBasis(
			selectedProjectRoot,
		)
		if err != nil {
			t.Fatal(err)
		}
		clonedProjectRoot := filepath.Join(
			workspaceRoot,
			"positive_typed_write",
			"scenario",
			"project",
		)
		if projectBefore != projectAfter ||
			!p14InstalledCLIPathExists(homeRoot) ||
			p14InstalledCLIPathExists(clonedProjectRoot) {
			t.Fatal("P14 memory fixture lifecycle changed or cloned the selected project")
		}
	})
}
