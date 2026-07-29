package p14acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/agenthostrestart"
)

const (
	p14InstalledObservationFinalizationEnvironmentKey = "HAFT_P14_FINALIZE_INSTALLED_OBSERVATION"
	p14InstalledObservationFinalizationRequestSchema  = "haft.p14.installed-observation-finalization-request/v2"
	p14InstalledObservationFinalizationRequestPrefix  = "p14-installed-observation-finalization-request"
	p14HostProcessReceiptSchema                       = "haft.p14.host-process-receipt/v1"
)

type p14InstalledObservationFinalizationRequest struct {
	Schema                  string `json:"schema"`
	PreparedCarrierPath     string `json:"prepared_carrier_path"`
	InstalledCLICapturePath string `json:"installed_cli_capture_path"`
	CodexMCPCapturePath     string `json:"codex_mcp_capture_path"`
	ClaudeHostProofPath     string `json:"claude_host_proof_path"`
}

type p14HostProcessReceipt struct {
	Schema                string                        `json:"schema"`
	ScenarioID            string                        `json:"scenario_id"`
	Builder               string                        `json:"builder"`
	RestartID             string                        `json:"restart_id"`
	ThreadID              string                        `json:"thread_id"`
	ResumeIntentDigest    string                        `json:"resume_intent_digest"`
	RequestPayloadDigest  string                        `json:"request_payload_digest"`
	RestartReceiptDigests []string                      `json:"restart_receipt_digests"`
	ObservedAt            string                        `json:"observed_at"`
	Checks                []p14InstalledCLICheckReceipt `json:"checks"`
}

type p14FinalizationCaptureSet struct {
	InstalledCLI map[string]p14InstalledCLIScenarioCapture
	CodexMCP     map[string]p14CodexMCPScenarioCapture
}

func TestP14FinalizeInstalledObservationCarrier(t *testing.T) {
	requestPath := os.Getenv(
		p14InstalledObservationFinalizationEnvironmentKey,
	)
	if requestPath == "" {
		t.Skip(
			"set HAFT_P14_FINALIZE_INSTALLED_OBSERVATION after verified restart",
		)
	}
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, _, err := loadRequestOracleContract(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	request, err := loadP14InstalledObservationFinalizationRequest(
		repositoryRoot,
		requestPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	prepared, preparedDigest, err := loadP14PreparedCarrierForExecution(
		repositoryRoot,
		contract,
		request.PreparedCarrierPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	installedCLI, err := loadP14InstalledCLICaptureForFinalization(
		repositoryRoot,
		contract,
		request.PreparedCarrierPath,
		preparedDigest,
		prepared,
		request.InstalledCLICapturePath,
	)
	if err != nil {
		t.Fatal(err)
	}
	codexMCP, packet, err := loadP14CodexMCPCaptureForFinalization(
		repositoryRoot,
		contract,
		request.PreparedCarrierPath,
		preparedDigest,
		prepared,
		request.CodexMCPCapturePath,
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := agenthostrestart.LoadVerifiedRuntimeSnapshot(
		repositoryRoot,
	)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := p14RuntimeBindingFromVerifiedSnapshot(
		prepared.Preparation,
		snapshot,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyP14FinalizationRuntimeBindings(
		runtime,
		installedCLI,
		packet,
	); err != nil {
		t.Fatal(err)
	}
	claudeContext, cancel := context.WithTimeout(
		context.Background(),
		p14ClaudeProcessObservationTimeout,
	)
	defer cancel()
	claudeProof, claudeProofDigest, err :=
		loadP14ClaudeHostProofForFinalization(
			claudeContext,
			repositoryRoot,
			request.ClaudeHostProofPath,
			prepared,
			runtime,
		)
	if err != nil {
		t.Fatal(err)
	}
	claudeBinding := p14ClaudeHostProofBinding{
		CarrierPath:    claudeProof.CarrierPath,
		CarrierDigest:  claudeProofDigest,
		EvidenceDigest: claudeProof.EvidenceDigest,
	}
	observations, err := assembleP14InstalledObservations(
		prepared.Preparation,
		runtime,
		installedCLI.Capture.ScenarioCaptures,
		codexMCP.ScenarioCaptures,
	)
	if err != nil {
		t.Fatal(err)
	}
	carrier, err := buildP14InstalledObservationCarrier(
		contract,
		request.PreparedCarrierPath,
		preparedDigest,
		prepared,
		runtime,
		claudeBinding,
		observations,
	)
	if err != nil {
		t.Fatal(err)
	}
	path, digest, err := persistP14FinalizedObservationCarrier(
		repositoryRoot,
		contract,
		carrier,
	)
	if path != "" {
		t.Logf(
			"P14_INSTALLED_OBSERVATION path=%s digest=%s status=%s",
			path,
			digest,
			carrier.Status,
		)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestP14InstalledObservationFinalizerMergesExactCaptureSet(
	t *testing.T,
) {
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, rawContract, err := loadRequestOracleContract(repositoryRoot)
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
	runtime := syntheticP14RuntimeObservationBinding(prepared)
	all := syntheticPassingP14InstalledObservations(prepared, runtime)
	installedCLI := p14SyntheticInstalledCLICaptures(all)
	codexMCP := p14SyntheticCodexMCPCaptures(all)
	observations, err := assembleP14InstalledObservations(
		prepared,
		runtime,
		installedCLI,
		codexMCP,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, status, err := evaluateP14InstalledObservations(
		prepared,
		observations,
	)
	if err != nil {
		t.Fatal(err)
	}
	if status != p14ObservationStatusPassed {
		t.Fatalf("assembled P14 observation status = %q", status)
	}
	if len(installedCLI) == 0 {
		t.Fatal("synthetic P14 installed CLI capture set is empty")
	}
	_, err = assembleP14InstalledObservations(
		prepared,
		runtime,
		installedCLI[:len(installedCLI)-1],
		codexMCP,
	)
	if err == nil {
		t.Fatal("P14 finalizer accepted an incomplete installed CLI capture set")
	}
}

func TestP14InstalledObservationFinalizerPersistsFailureAndExitsRed(
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
	preparedPath, preparedDigest, err :=
		persistPreparedRequestOracleCarrier(
			repositoryRoot,
			contract,
			preparedInput,
		)
	if err != nil {
		t.Fatal(err)
	}
	preparedRaw, err := os.ReadFile(
		filepath.Join(
			repositoryRoot,
			filepath.FromSlash(preparedPath),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := decodePreparedRequestOracleCarrier(
		contract,
		preparedRaw,
	)
	if err != nil {
		t.Fatal(err)
	}
	runtime := syntheticP14RuntimeObservationBinding(
		prepared.Preparation,
	)
	observations := syntheticPassingP14InstalledObservations(
		prepared.Preparation,
		runtime,
	)
	observations[1].SurfaceObservations[0].NormalizedResultDigest =
		p14TestDigest("finalizer-failed-observation")
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
	if carrier.Status != p14ObservationStatusFailed {
		t.Fatal("adversarial P14 carrier is not failed")
	}
	headerOnlyPassing := carrier
	headerOnlyPassing.Status = p14ObservationStatusPassed
	headerOnlyPassing.Observation.Status = p14ObservationStatusPassed
	if err := requireP14PassingFinalization(headerOnlyPassing); err == nil {
		t.Fatal("P14 finalizer ignored a failed scenario verdict")
	}
	path, digest, finalizationErr :=
		persistP14FinalizedObservationCarrier(
			repositoryRoot,
			contract,
			carrier,
		)
	if finalizationErr == nil {
		t.Fatal("P14 finalizer returned success for a failed carrier")
	}
	if path == "" || !validP14Digest(digest) {
		t.Fatal("P14 finalizer discarded failed diagnostic evidence")
	}
	if !strings.Contains(finalizationErr.Error(), path) ||
		!strings.Contains(finalizationErr.Error(), digest) {
		t.Fatal("P14 finalizer failure omits diagnostic carrier identity")
	}
	raw, err := os.ReadFile(
		filepath.Join(repositoryRoot, filepath.FromSlash(path)),
	)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := decodeP14InstalledObservationCarrier(
		repositoryRoot,
		contract,
		raw,
	)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Status != p14ObservationStatusFailed {
		t.Fatal("P14 finalizer persisted a false passing verdict")
	}
}

func persistP14FinalizedObservationCarrier(
	repositoryRoot string,
	contract requestOracleContract,
	carrier p14InstalledObservationCarrier,
) (string, string, error) {
	path, digest, err := persistP14InstalledObservationCarrier(
		repositoryRoot,
		contract,
		carrier,
	)
	if err != nil {
		return "", "", err
	}
	if err := requireP14PassingFinalization(carrier); err != nil {
		return path, digest, fmt.Errorf(
			"%w; diagnostic carrier path=%s digest=%s",
			err,
			path,
			digest,
		)
	}
	return path, digest, nil
}

func requireP14PassingFinalization(
	carrier p14InstalledObservationCarrier,
) error {
	nonPassingScenarios := make(
		[]string,
		0,
		len(carrier.Observation.ScenarioObservations),
	)
	for _, scenario := range carrier.Observation.ScenarioObservations {
		if scenario.Verdict == p14ObservationStatusPassed {
			continue
		}
		entry := scenario.ID + "=" + scenario.Verdict
		nonPassingScenarios = append(nonPassingScenarios, entry)
	}
	carrierPassed := carrier.Status == p14ObservationStatusPassed
	observationPassed :=
		carrier.Observation.Status == p14ObservationStatusPassed
	scenariosPassed := len(nonPassingScenarios) == 0
	if carrierPassed && observationPassed && scenariosPassed {
		return nil
	}
	return fmt.Errorf(
		"P14 finalization is not passing: carrier_status=%q observation_status=%q non_passing_scenarios=%q",
		carrier.Status,
		carrier.Observation.Status,
		nonPassingScenarios,
	)
}

func loadP14InstalledObservationFinalizationRequest(
	repositoryRoot string,
	path string,
) (p14InstalledObservationFinalizationRequest, error) {
	canonical, err := resolveP14ExecutionCarrierPath(
		repositoryRoot,
		path,
		p14InstalledObservationFinalizationRequestPrefix,
	)
	if err != nil {
		return p14InstalledObservationFinalizationRequest{}, err
	}
	raw, err := os.ReadFile(canonical)
	if err != nil {
		return p14InstalledObservationFinalizationRequest{}, err
	}
	request := p14InstalledObservationFinalizationRequest{}
	err = decodeP14CanonicalCarrier(
		raw,
		&request,
		"installed observation finalization request",
	)
	if err != nil {
		return p14InstalledObservationFinalizationRequest{}, err
	}
	if request.Schema !=
		p14InstalledObservationFinalizationRequestSchema {
		return p14InstalledObservationFinalizationRequest{}, fmt.Errorf(
			"P14 installed observation finalization request schema differs",
		)
	}
	paths := []string{
		request.PreparedCarrierPath,
		request.InstalledCLICapturePath,
		request.CodexMCPCapturePath,
		request.ClaudeHostProofPath,
	}
	for _, carrierPath := range paths {
		if strings.TrimSpace(carrierPath) == "" {
			return p14InstalledObservationFinalizationRequest{}, fmt.Errorf(
				"P14 installed observation finalization path is empty",
			)
		}
	}
	return request, nil
}

func loadP14InstalledCLICaptureForFinalization(
	repositoryRoot string,
	contract requestOracleContract,
	preparedPath string,
	preparedDigest string,
	prepared preparedRequestOracleCarrier,
	path string,
) (p14InstalledCLICaptureCarrier, error) {
	canonical, relative, raw, err := loadP14FinalizationCarrierBytes(
		repositoryRoot,
		path,
		"p14-installed-cli-capture-",
	)
	if err != nil {
		return p14InstalledCLICaptureCarrier{}, err
	}
	carrier := p14InstalledCLICaptureCarrier{}
	err = decodeP14CanonicalCarrier(
		raw,
		&carrier,
		"installed CLI capture carrier",
	)
	if err != nil {
		return p14InstalledCLICaptureCarrier{}, err
	}
	if relative != carrier.CarrierPath {
		return p14InstalledCLICaptureCarrier{}, fmt.Errorf(
			"P14 installed CLI capture path differs from its bytes",
		)
	}
	if err := validateP14InstalledCLICaptureCarrier(
		contract,
		prepared,
		carrier,
	); err != nil {
		return p14InstalledCLICaptureCarrier{}, err
	}
	binding := carrier.Capture.PreparedCarrier
	if binding.CarrierPath != preparedPath ||
		binding.CarrierDigest != preparedDigest ||
		binding.PreparationDigest != prepared.PreparationDigest {
		return p14InstalledCLICaptureCarrier{}, fmt.Errorf(
			"P14 installed CLI capture uses another prepared carrier",
		)
	}
	_ = canonical
	return carrier, nil
}

func loadP14CodexMCPCaptureForFinalization(
	repositoryRoot string,
	contract requestOracleContract,
	preparedPath string,
	preparedDigest string,
	prepared preparedRequestOracleCarrier,
	path string,
) (
	p14CodexMCPCaptureCarrier,
	p14CodexMCPRequestCarrier,
	error,
) {
	_, relative, raw, err := loadP14FinalizationCarrierBytes(
		repositoryRoot,
		path,
		"p14-codex-mcp-capture-",
	)
	if err != nil {
		return p14CodexMCPCaptureCarrier{},
			p14CodexMCPRequestCarrier{},
			err
	}
	carrier := p14CodexMCPCaptureCarrier{}
	err = decodeP14CanonicalCarrier(
		raw,
		&carrier,
		"Codex MCP capture carrier",
	)
	if err != nil {
		return p14CodexMCPCaptureCarrier{},
			p14CodexMCPRequestCarrier{},
			err
	}
	if relative != carrier.CarrierPath {
		return p14CodexMCPCaptureCarrier{},
			p14CodexMCPRequestCarrier{},
			fmt.Errorf("P14 Codex MCP capture path differs from its bytes")
	}
	packet, packetPrepared, err := loadP14CodexMCPPacketBinding(
		repositoryRoot,
		contract,
		carrier.RequestPacket,
	)
	if err != nil {
		return p14CodexMCPCaptureCarrier{},
			p14CodexMCPRequestCarrier{},
			err
	}
	if packetPrepared.CarrierPath != preparedPath ||
		packet.Packet.PreparedCarrier.CarrierDigest != preparedDigest ||
		packetPrepared.PreparationDigest != prepared.PreparationDigest {
		return p14CodexMCPCaptureCarrier{},
			p14CodexMCPRequestCarrier{},
			fmt.Errorf("P14 Codex MCP capture uses another prepared carrier")
	}
	recomputed, err := validateAndRecomputeP14CodexMCPCaptureCarrier(
		contract,
		prepared,
		packet,
		carrier,
	)
	if err != nil {
		return p14CodexMCPCaptureCarrier{},
			p14CodexMCPRequestCarrier{},
			err
	}
	carrier.ScenarioCaptures = recomputed
	reconstructedInput, err := reconstructP14CodexMCPCaptureInput(
		packet,
		carrier,
	)
	if err != nil {
		return p14CodexMCPCaptureCarrier{},
			p14CodexMCPRequestCarrier{},
			err
	}
	if err := verifyP14CodexSessionHistorySource(
		packet,
		reconstructedInput,
		carrier.SessionHistory,
	); err != nil {
		return p14CodexMCPCaptureCarrier{},
			p14CodexMCPRequestCarrier{},
			err
	}
	return carrier, packet, nil
}

func loadP14FinalizationCarrierBytes(
	repositoryRoot string,
	path string,
	prefix string,
) (string, string, []byte, error) {
	canonical, err := resolveP14ExecutionCarrierPath(
		repositoryRoot,
		path,
		prefix,
	)
	if err != nil {
		return "", "", nil, err
	}
	raw, err := os.ReadFile(canonical)
	if err != nil {
		return "", "", nil, err
	}
	relative, err := filepath.Rel(repositoryRoot, canonical)
	if err != nil {
		return "", "", nil, err
	}
	return canonical, filepath.ToSlash(relative), raw, nil
}

func verifyP14FinalizationRuntimeBindings(
	runtime p14RuntimeObservationBinding,
	installedCLI p14InstalledCLICaptureCarrier,
	packet p14CodexMCPRequestCarrier,
) error {
	capture := installedCLI.Capture
	if capture.InstalledExecutablePath !=
		runtime.InstalledExecutablePath ||
		capture.InstalledExecutableDigest !=
			runtime.InstalledExecutableDigest {
		return fmt.Errorf(
			"P14 installed CLI capture uses another installed executable",
		)
	}
	left, err := json.Marshal(runtime)
	if err != nil {
		return err
	}
	right, err := json.Marshal(packet.Packet.Runtime)
	if err != nil {
		return err
	}
	if !bytes.Equal(left, right) {
		return fmt.Errorf(
			"P14 Codex MCP capture uses another verified runtime",
		)
	}
	return nil
}

func assembleP14InstalledObservations(
	prepared preparedRequestOracleInput,
	runtime p14RuntimeObservationBinding,
	installedCLI []p14InstalledCLIScenarioCapture,
	codexMCP []p14CodexMCPScenarioCapture,
) ([]p14InstalledScenarioObservation, error) {
	captures, err := indexP14FinalizationCaptures(
		installedCLI,
		codexMCP,
	)
	if err != nil {
		return nil, err
	}
	observations := make(
		[]p14InstalledScenarioObservation,
		0,
		len(prepared.Scenarios),
	)
	for _, scenario := range prepared.Scenarios {
		observation, assembleErr := assembleP14InstalledScenario(
			scenario,
			runtime,
			captures,
		)
		if assembleErr != nil {
			return nil, assembleErr
		}
		observations = append(observations, observation)
	}
	return observations, nil
}

func indexP14FinalizationCaptures(
	installedCLI []p14InstalledCLIScenarioCapture,
	codexMCP []p14CodexMCPScenarioCapture,
) (p14FinalizationCaptureSet, error) {
	captures := p14FinalizationCaptureSet{
		InstalledCLI: make(
			map[string]p14InstalledCLIScenarioCapture,
			len(installedCLI),
		),
		CodexMCP: make(
			map[string]p14CodexMCPScenarioCapture,
			len(codexMCP),
		),
	}
	for _, capture := range installedCLI {
		if _, duplicate := captures.InstalledCLI[capture.ID]; duplicate {
			return p14FinalizationCaptureSet{}, fmt.Errorf(
				"P14 finalizer repeats installed CLI scenario %q",
				capture.ID,
			)
		}
		captures.InstalledCLI[capture.ID] = capture
	}
	for _, capture := range codexMCP {
		if _, duplicate := captures.CodexMCP[capture.ID]; duplicate {
			return p14FinalizationCaptureSet{}, fmt.Errorf(
				"P14 finalizer repeats Codex MCP scenario %q",
				capture.ID,
			)
		}
		captures.CodexMCP[capture.ID] = capture
	}
	return captures, nil
}

func assembleP14InstalledScenario(
	scenario preparedP14Scenario,
	runtime p14RuntimeObservationBinding,
	captures p14FinalizationCaptureSet,
) (p14InstalledScenarioObservation, error) {
	handlers := map[string]func(
		preparedP14Scenario,
		preparedP14Request,
	) (p14InstalledSurfaceObservation, error){
		"installed_cli": func(
			current preparedP14Scenario,
			_ preparedP14Request,
		) (p14InstalledSurfaceObservation, error) {
			capture, present := captures.InstalledCLI[current.ID]
			if !present ||
				capture.SemanticRequestDigest !=
					current.SemanticRequestDigest {
				return p14InstalledSurfaceObservation{}, fmt.Errorf(
					"P14 finalizer lacks installed CLI scenario %q",
					current.ID,
				)
			}
			return capture.SurfaceObservation, nil
		},
		"live_mcp": func(
			current preparedP14Scenario,
			_ preparedP14Request,
		) (p14InstalledSurfaceObservation, error) {
			capture, present := captures.CodexMCP[current.ID]
			if !present ||
				capture.SemanticRequestDigest !=
					current.SemanticRequestDigest {
				return p14InstalledSurfaceObservation{}, fmt.Errorf(
					"P14 finalizer lacks Codex MCP scenario %q",
					current.ID,
				)
			}
			return capture.SurfaceObservation, nil
		},
		"host_process": func(
			current preparedP14Scenario,
			request preparedP14Request,
		) (p14InstalledSurfaceObservation, error) {
			return buildP14HostProcessObservation(
				current,
				request,
				runtime,
			)
		},
	}
	surfaces := make(
		[]p14InstalledSurfaceObservation,
		0,
		len(scenario.Requests),
	)
	for _, request := range scenario.Requests {
		handler, present := handlers[request.Surface]
		if !present {
			return p14InstalledScenarioObservation{}, fmt.Errorf(
				"P14 finalizer surface %q is open",
				request.Surface,
			)
		}
		surface, err := handler(scenario, request)
		if err != nil {
			return p14InstalledScenarioObservation{}, err
		}
		surfaces = append(surfaces, surface)
	}
	predicates, err := buildP14RuntimePredicateObservations(
		scenario,
		runtime,
		surfaces,
	)
	if err != nil {
		return p14InstalledScenarioObservation{}, err
	}
	return p14InstalledScenarioObservation{
		ID:                    scenario.ID,
		SemanticRequestDigest: scenario.SemanticRequestDigest,
		SurfaceObservations:   surfaces,
		PredicateObservations: predicates,
	}, nil
}

func buildP14HostProcessObservation(
	scenario preparedP14Scenario,
	request preparedP14Request,
	runtime p14RuntimeObservationBinding,
) (p14InstalledSurfaceObservation, error) {
	policy, err := p14LiveProtocolPolicyForScenario(scenario.ID)
	if err != nil {
		return p14InstalledSurfaceObservation{}, err
	}
	checkIDs := policy.SurfaceChecks["host_process"]
	checks, err := buildP14HostProcessChecks(checkIDs, runtime)
	if err != nil {
		return p14InstalledSurfaceObservation{}, err
	}
	observedAt := p14HostProcessObservedAt(runtime)
	receipt := p14HostProcessReceipt{
		Schema:               p14HostProcessReceiptSchema,
		ScenarioID:           scenario.ID,
		Builder:              request.Builder,
		RestartID:            runtime.RestartID,
		ThreadID:             runtime.ThreadID,
		ResumeIntentDigest:   runtime.ResumeIntentDigest,
		RequestPayloadDigest: request.PayloadDigest,
		RestartReceiptDigests: []string{
			runtime.RestartCheckpointDigest,
			runtime.LiveMCPReceiptDigest,
			runtime.FallbackReceiptDigest,
		},
		ObservedAt: observedAt,
		Checks:     checks,
	}
	raw, err := marshalP14CanonicalJSON(receipt)
	if err != nil {
		return p14InstalledSurfaceObservation{}, err
	}
	digest := p14Digest(raw)
	return p14InstalledSurfaceObservation{
		Surface:              request.Surface,
		RequestPayloadDigest: request.PayloadDigest,
		Source:               p14ObservationSourceHostProcess,
		SourceReceiptDigest:  digest,
		ObservedAt:           observedAt,
		Outcome:              p14SurfaceOutcomeObserved,
		ObservationCanonical: string(raw),
		ObservationDigest:    digest,
	}, nil
}

func buildP14HostProcessChecks(
	ids []string,
	runtime p14RuntimeObservationBinding,
) ([]p14InstalledCLICheckReceipt, error) {
	checks := make([]p14InstalledCLICheckReceipt, 0, len(ids))
	for _, id := range ids {
		satisfied, present := p14HostProcessCheck(id, runtime)
		if !present {
			return nil, fmt.Errorf(
				"P14 host process check %q is open",
				id,
			)
		}
		checks = append(checks, p14InstalledCLICheckReceipt{
			ID:        id,
			Satisfied: satisfied,
		})
	}
	return checks, nil
}

func p14HostProcessCheck(
	id string,
	runtime p14RuntimeObservationBinding,
) (bool, bool) {
	values := map[string]bool{
		"frozen_project_basis_match": runtime.ProjectRoot ==
			runtime.LiveMCPProjectRoot,
		"frozen_carrier_digests_match": runtime.
			RestartCheckpointState == "verified",
		"new_process_generation_is_observed": validP14Digest(
			runtime.LiveMCPReceiptDigest,
		),
		"exact_task_deep_link_resumed": runtime.ExactTaskResumeCount == 1,
		"exact_task_acquired_one_resume_lease": runtime.
			ExactTaskResumeCount == 1,
		"resume_lease_has_one_writer": runtime.SingleWriterObserved,
		"same_link_fallback_wake_count_at_most_one": runtime.
			FallbackWakeCount <= 1,
		"full_candidate_digest_is_durably_reserved": runtime.
			CandidateDigestReserved,
		"a_to_b_to_a_is_rejected": runtime.CandidateDigestReserved,
		"private_receipts_and_attempt_history_are_gitignored": runtime.
			PrivateStateGitignoredObserved,
		"checkpoint_stages_are_removed": runtime.TemporaryStagesAbsent,
		"launchd_label_is_observed_absent": runtime.
			LaunchdRemovalObserved,
	}
	value, present := values[id]
	return value, present
}

func buildP14RuntimePredicateObservations(
	scenario preparedP14Scenario,
	runtime p14RuntimeObservationBinding,
	surfaces []p14InstalledSurfaceObservation,
) ([]p14InstalledPredicateObservation, error) {
	if scenario.Oracle.Kind != "live_predicate" {
		return nil, nil
	}
	predicates := make(
		[]p14InstalledPredicateObservation,
		0,
		len(scenario.Oracle.PredicateIDs),
	)
	for _, id := range scenario.Oracle.PredicateIDs {
		satisfied, present := p14RuntimePredicate(id, runtime)
		sourceSurface := "host_process"
		if !present {
			satisfied, sourceSurface, present =
				p14AgentOrientationPredicate(id, surfaces)
		}
		if !present {
			return nil, fmt.Errorf(
				"P14 runtime predicate %q is open",
				id,
			)
		}
		predicates = append(predicates, p14InstalledPredicateObservation{
			PredicateID:   id,
			SourceSurface: sourceSurface,
			Satisfied:     satisfied,
		})
	}
	return predicates, nil
}

func p14RuntimePredicate(
	id string,
	runtime p14RuntimeObservationBinding,
) (bool, bool) {
	values := map[string]bool{
		"p14.runtime.executable_identity.v1": runtime.
			InstalledExecutableDigest == runtime.LiveMCPExecutableDigest,
		"p14.runtime.process_generation.v1": validP14Digest(
			runtime.LiveMCPReceiptDigest,
		),
		"p14.runtime.project_basis.v1": runtime.ProjectRoot ==
			runtime.LiveMCPProjectRoot,
		"p14.runtime.carrier_basis.v1": runtime.
			RestartCheckpointState == "verified",
		"p14.runtime.private_nonce.v1": validP14Digest(
			runtime.LiveMCPReceiptDigest,
		),
		"p14.host.exact_task_resume.v1": runtime.
			ExactTaskResumeCount == 1,
		"p14.host.exact_task_resume_once.v1": runtime.
			ExactTaskResumeCount == 1,
		"p14.host.single_writer.v1":    runtime.SingleWriterObserved,
		"p14.host.bounded_fallback.v1": runtime.FallbackWakeCount <= 1,
		"p14.cleanup.full_digest_history.v1": runtime.
			CandidateDigestReserved,
		"p14.cleanup.private_state.v1": runtime.
			PrivateStateGitignoredObserved,
		"p14.cleanup.stage_removal.v1": runtime.TemporaryStagesAbsent,
		"p14.cleanup.launchd_absent.v1": runtime.
			LaunchdRemovalObserved,
		"p14.agent.code_graph.host_generation.v1": validP14Digest(
			runtime.LiveMCPReceiptDigest,
		),
		"p14.agent.memory.host_generation.v1": validP14Digest(
			runtime.LiveMCPReceiptDigest,
		),
	}
	value, present := values[id]
	return value, present
}

func p14AgentOrientationPredicate(
	id string,
	surfaces []p14InstalledSurfaceObservation,
) (bool, string, bool) {
	type predicateSource struct {
		Surface string
	}
	values := map[string]predicateSource{
		"p14.agent.code_graph.installed_surface.v1": {
			Surface: "installed_cli",
		},
		"p14.agent.code_graph.read_only_probe.v1": {
			Surface: "live_mcp",
		},
		"p14.agent.code_graph.no_identity_autoselect.v1": {
			Surface: "live_mcp",
		},
		"p14.agent.memory.installed_surface.v1": {
			Surface: "installed_cli",
		},
		"p14.agent.memory.read_only_probe.v1": {
			Surface: "live_mcp",
		},
		"p14.agent.memory.no_implicit_admission.v1": {
			Surface: "live_mcp",
		},
		"p14.agent.memory.explicit_save_gate.v1": {
			Surface: "live_mcp",
		},
		"p14.agent.memory.single_establishment_commit.v1": {
			Surface: "live_mcp",
		},
		"p14.agent.memory.entity_ref_round_trip.v1": {
			Surface: "live_mcp",
		},
		"p14.agent.memory.non_authorizing_interpretation.v1": {
			Surface: "live_mcp",
		},
	}
	source, present := values[id]
	if !present {
		return false, "", false
	}
	for _, surface := range surfaces {
		if surface.Surface == source.Surface {
			return surface.Outcome == p14SurfaceOutcomeObserved,
				source.Surface,
				true
		}
	}
	return false, source.Surface, true
}

func p14LiveProtocolPolicyForScenario(
	scenarioID string,
) (p14LiveProtocolPolicy, error) {
	for _, policy := range p14LiveProtocolPolicies() {
		if policy.ScenarioID == scenarioID {
			return policy, nil
		}
	}
	return p14LiveProtocolPolicy{}, fmt.Errorf(
		"P14 live protocol scenario %q is open",
		scenarioID,
	)
}

func p14HostProcessObservedAt(
	runtime p14RuntimeObservationBinding,
) string {
	fallback, fallbackErr := time.Parse(
		time.RFC3339Nano,
		runtime.FallbackClearedAt,
	)
	fulfilled, fulfilledErr := time.Parse(
		time.RFC3339Nano,
		runtime.LiveMCPFulfilledAt,
	)
	if fallbackErr == nil && fulfilledErr == nil &&
		fulfilled.After(fallback) {
		return fulfilled.UTC().Format(time.RFC3339Nano)
	}
	return fallback.UTC().Format(time.RFC3339Nano)
}

func p14SyntheticInstalledCLICaptures(
	observations []p14InstalledScenarioObservation,
) []p14InstalledCLIScenarioCapture {
	captures := make([]p14InstalledCLIScenarioCapture, 0)
	for _, observation := range observations {
		for _, surface := range observation.SurfaceObservations {
			if surface.Surface != "installed_cli" {
				continue
			}
			captures = append(captures, p14InstalledCLIScenarioCapture{
				ID:                    observation.ID,
				SemanticRequestDigest: observation.SemanticRequestDigest,
				SurfaceObservation:    surface,
			})
		}
	}
	return captures
}

func p14SyntheticCodexMCPCaptures(
	observations []p14InstalledScenarioObservation,
) []p14CodexMCPScenarioCapture {
	captures := make([]p14CodexMCPScenarioCapture, 0)
	for _, observation := range observations {
		for _, surface := range observation.SurfaceObservations {
			if surface.Surface != "live_mcp" {
				continue
			}
			captures = append(captures, p14CodexMCPScenarioCapture{
				ID:                    observation.ID,
				SemanticRequestDigest: observation.SemanticRequestDigest,
				SurfaceObservation:    surface,
			})
		}
	}
	return captures
}
