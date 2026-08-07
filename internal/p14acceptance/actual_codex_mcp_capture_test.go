package p14acceptance

import (
	"bytes"
	"context"
	"encoding/base64"
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
	p14CodexMCPRequestEnvironmentKey = "HAFT_P14_GENERATE_CODEX_MCP_REQUEST"
	p14CodexMCPCaptureEnvironmentKey = "HAFT_P14_INGEST_CODEX_MCP_CAPTURE"

	p14CodexMCPRequestInputSchema   = "haft.p14.codex-mcp-request-input/v3"
	p14CodexMCPRequestCarrierSchema = "haft.p14.codex-mcp-request/v3"
	p14CodexMCPCaptureInputSchema   = "haft.p14.codex-mcp-capture-input/v3"
	p14CodexMCPCaptureCarrierSchema = "haft.p14.codex-mcp-capture/v5"
	p14CodexMCPReceiptSchema        = "haft.p14.codex-mcp-scenario-receipt/v4"

	p14CodexMCPRequestStatus = "capture_requested_not_executed"
	p14CodexMCPCaptureStatus = "captured_not_final"

	p14CodexMCPRequestSemantics = "Exact P14 live-MCP calls and agent-goal prompts derived from one sealed preparation and one verified resumed-Codex runtime. Prompt-driven calls do not expose a tool recipe. This packet is not performed Work, response evidence, a P14 verdict, release authority, or a release claim."
	p14CodexMCPCaptureSemantics = "Actual resumed-Codex user-prompt and MCP call-history projections plus response bodies normalized against one sealed P14 request packet and independently checked against the append-only Codex session-history prefix for the verified thread. The carrier also preserves raw initialize and tools/list JSON-RPC bytes observed from the exact installed executable after verified live-MCP fulfillment. This partial carrier is not a complete P14 verdict, release authority, or a release claim."

	p14CodexMCPResponseLimit = 4 << 20
)

type p14CodexMCPRequestInput struct {
	Schema          string                        `json:"schema"`
	Status          string                        `json:"status"`
	ResultSemantics string                        `json:"result_semantics"`
	ReleaseClaim    bool                          `json:"release_claim"`
	PreparedCarrier p14PreparedObservationBinding `json:"prepared_carrier"`
	Runtime         p14RuntimeObservationBinding  `json:"runtime"`
	GeneratedAt     string                        `json:"generated_at"`
	Calls           []p14CodexMCPPlannedCall      `json:"calls"`
}

type p14CodexMCPPlannedCall struct {
	Sequence             int                            `json:"sequence"`
	ScenarioID           string                         `json:"scenario_id"`
	Builder              string                         `json:"builder"`
	CaseID               string                         `json:"case_id"`
	ExchangeID           string                         `json:"exchange_id"`
	ExchangeRole         string                         `json:"exchange_role"`
	ParallelGroup        string                         `json:"parallel_group,omitempty"`
	RequiredPredecessors []string                       `json:"required_predecessors"`
	Tool                 string                         `json:"tool"`
	ArgsCanonical        string                         `json:"args_canonical"`
	ArgsDigest           string                         `json:"args_digest"`
	RequestPayloadDigest string                         `json:"request_payload_digest"`
	AgentPrompt          *p14CodexMCPPlannedAgentPrompt `json:"agent_prompt,omitempty"`
}

type p14CodexMCPPlannedAgentPrompt struct {
	ID                    string `json:"id"`
	TextCanonical         string `json:"text_canonical"`
	TextDigest            string `json:"text_digest"`
	ExpectedToolCallCount int    `json:"expected_tool_call_count"`
}

type p14CodexMCPRequestCarrier struct {
	Schema       string                  `json:"schema"`
	Status       string                  `json:"status"`
	CarrierPath  string                  `json:"carrier_path"`
	PacketDigest string                  `json:"packet_digest"`
	Packet       p14CodexMCPRequestInput `json:"packet"`
}

type p14CodexMCPRequestBinding struct {
	CarrierPath   string `json:"carrier_path"`
	CarrierDigest string `json:"carrier_digest"`
	PacketDigest  string `json:"packet_digest"`
}

type p14CodexMCPTranscriptProjection struct {
	ThreadID             string `json:"thread_id"`
	TurnID               string `json:"turn_id"`
	ToolCallID           string `json:"tool_call_id"`
	Server               string `json:"server"`
	Tool                 string `json:"tool"`
	ArgsCanonical        string `json:"args_canonical"`
	ArgsDigest           string `json:"args_digest"`
	Status               string `json:"status"`
	DurationMilliseconds int64  `json:"duration_milliseconds"`
	TurnToolCallOrdinal  int    `json:"turn_tool_call_ordinal"`
	TurnToolCallCount    int    `json:"turn_tool_call_count"`
	HistoryReadAt        string `json:"history_read_at"`
}

type p14CodexMCPPromptTranscriptProjection struct {
	PromptID      string `json:"prompt_id"`
	ThreadID      string `json:"thread_id"`
	TurnID        string `json:"turn_id"`
	Role          string `json:"role"`
	TextCanonical string `json:"text_canonical"`
	TextDigest    string `json:"text_digest"`
	HistoryReadAt string `json:"history_read_at"`
}

type p14CodexMCPResponseCapture struct {
	ToolCallID string `json:"tool_call_id"`
	CapturedAt string `json:"captured_at"`
	IsError    bool   `json:"is_error"`
	BodyBase64 string `json:"body_base64"`
	BodyDigest string `json:"body_digest"`
}

type p14CodexMCPCallEvidence struct {
	Sequence      int                                    `json:"sequence"`
	ScenarioID    string                                 `json:"scenario_id"`
	CaseID        string                                 `json:"case_id"`
	ExchangeID    string                                 `json:"exchange_id"`
	ExchangeRole  string                                 `json:"exchange_role"`
	ParallelGroup string                                 `json:"parallel_group,omitempty"`
	AgentPrompt   *p14CodexMCPPromptTranscriptProjection `json:"agent_prompt,omitempty"`
	Transcript    p14CodexMCPTranscriptProjection        `json:"transcript"`
	Response      p14CodexMCPResponseCapture             `json:"response"`
}

type p14CodexMCPCaptureInput struct {
	Schema          string                    `json:"schema"`
	Status          string                    `json:"status"`
	ResultSemantics string                    `json:"result_semantics"`
	ReleaseClaim    bool                      `json:"release_claim"`
	RequestPacket   p14CodexMCPRequestBinding `json:"request_packet"`
	CapturedAt      string                    `json:"captured_at"`
	Calls           []p14CodexMCPCallEvidence `json:"calls"`
}

type p14CodexMCPScenarioCapture struct {
	ID                    string                         `json:"id"`
	SemanticRequestDigest string                         `json:"semantic_request_digest"`
	SurfaceObservation    p14InstalledSurfaceObservation `json:"surface_observation"`
}

type p14CodexMCPCaptureCarrier struct {
	Schema            string                         `json:"schema"`
	Status            string                         `json:"status"`
	CarrierPath       string                         `json:"carrier_path"`
	CaptureDigest     string                         `json:"capture_digest"`
	RequestPacket     p14CodexMCPRequestBinding      `json:"request_packet"`
	CapturedAt        string                         `json:"captured_at"`
	SessionHistory    p14CodexSessionHistoryEvidence `json:"session_history"`
	ProtocolDiscovery p14MCPProtocolDiscovery        `json:"protocol_discovery"`
	ScenarioCaptures  []p14CodexMCPScenarioCapture   `json:"scenario_captures"`
}

type p14CodexMCPScenarioReceipt struct {
	Schema                  string                        `json:"schema"`
	ScenarioID              string                        `json:"scenario_id"`
	Builder                 string                        `json:"builder"`
	ThreadID                string                        `json:"thread_id"`
	TurnIDs                 []string                      `json:"turn_ids"`
	RuntimeReceiptDigest    string                        `json:"runtime_receipt_digest"`
	RequestPayloadDigest    string                        `json:"request_payload_digest"`
	ProtocolDiscoveryDigest string                        `json:"protocol_discovery_digest,omitempty"`
	Calls                   []p14CodexMCPCallEvidence     `json:"calls"`
	Checks                  []p14InstalledCLICheckReceipt `json:"checks"`
	FailureDetail           string                        `json:"failure_detail,omitempty"`
}

type p14CodexMCPCallDefinition struct {
	CaseID               string
	ParallelGroup        string
	RequiredPredecessors []string
	Tool                 string
	Args                 map[string]any
	AgentPrompt          *p14CodexMCPPlannedAgentPrompt
}

type p14CodexMCPFamilyResult struct {
	Normalized    []byte
	Checks        []p14InstalledCLICheckReceipt
	FailureCode   string
	FailureDetail string
}

type p14CodexMCPFamilyNormalizer func(
	preparedRequestOracleCarrier,
	preparedP14Scenario,
	preparedP14Request,
	[]p14CodexMCPCallEvidence,
) (p14CodexMCPFamilyResult, error)

func TestP14GenerateActualCodexMCPRequestPacket(t *testing.T) {
	preparedPath := os.Getenv(p14CodexMCPRequestEnvironmentKey)
	if preparedPath == "" {
		t.Skip("set HAFT_P14_GENERATE_CODEX_MCP_REQUEST after verified resume")
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
	snapshot, err := agenthostrestart.LoadVerifiedRuntimeSnapshot(repositoryRoot)
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
	carrier, err := buildP14CodexMCPRequestCarrier(
		contract,
		prepared.CarrierPath,
		preparedDigest,
		prepared,
		runtime,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatal(err)
	}
	path, digest, err := persistP14CodexMCPRequestCarrier(
		repositoryRoot,
		contract,
		prepared,
		carrier,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("P14_CODEX_MCP_REQUEST path=%s digest=%s", path, digest)
}

func TestP14IngestActualCodexMCPCapture(t *testing.T) {
	inputPath := os.Getenv(p14CodexMCPCaptureEnvironmentKey)
	if inputPath == "" {
		t.Skip("set HAFT_P14_INGEST_CODEX_MCP_CAPTURE after actual task calls")
	}
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, _, err := loadRequestOracleContract(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	input, err := loadP14CodexMCPCaptureInput(repositoryRoot, inputPath)
	if err != nil {
		t.Fatal(err)
	}
	packet, prepared, err := loadP14CodexMCPPacketBinding(
		repositoryRoot,
		contract,
		input.RequestPacket,
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := agenthostrestart.LoadVerifiedRuntimeSnapshot(repositoryRoot)
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
	if runtime != packet.Packet.Runtime {
		t.Fatal("P14 Codex MCP runtime changed after request generation")
	}
	sessionHistory, err := captureP14CodexSessionHistory(
		packet,
		input,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatal(err)
	}
	protocolDiscovery, err := captureP14MCPProtocolDiscovery(
		context.Background(),
		runtime,
	)
	if err != nil {
		t.Fatal(err)
	}
	carrier, err := buildP14CodexMCPCaptureCarrier(
		contract,
		prepared,
		packet,
		input,
		sessionHistory,
		protocolDiscovery,
	)
	if err != nil {
		t.Fatal(err)
	}
	path, digest, err := persistP14CodexMCPCaptureCarrier(
		repositoryRoot,
		contract,
		prepared,
		packet,
		carrier,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("P14_CODEX_MCP_CAPTURE path=%s digest=%s", path, digest)
}

func buildP14CodexMCPRequestCarrier(
	contract requestOracleContract,
	preparedPath string,
	preparedDigest string,
	prepared preparedRequestOracleCarrier,
	runtime p14RuntimeObservationBinding,
	generatedAt time.Time,
) (p14CodexMCPRequestCarrier, error) {
	if err := verifyPreparedRequestOracleCarrier(contract, prepared); err != nil {
		return p14CodexMCPRequestCarrier{}, err
	}
	if err := verifyP14PreparedObservationBindingValue(
		preparedPath,
		preparedDigest,
		prepared,
	); err != nil {
		return p14CodexMCPRequestCarrier{}, err
	}
	if err := validateP14RuntimeObservationBinding(
		prepared.Preparation,
		runtime,
	); err != nil {
		return p14CodexMCPRequestCarrier{}, err
	}
	calls, err := buildP14CodexMCPPlannedCalls(prepared.Preparation.Scenarios)
	if err != nil {
		return p14CodexMCPRequestCarrier{}, err
	}
	packet := p14CodexMCPRequestInput{
		Schema:          p14CodexMCPRequestInputSchema,
		Status:          p14CodexMCPRequestStatus,
		ResultSemantics: p14CodexMCPRequestSemantics,
		ReleaseClaim:    false,
		PreparedCarrier: p14PreparedObservationBinding{
			CarrierPath:       preparedPath,
			CarrierDigest:     preparedDigest,
			PreparationDigest: prepared.PreparationDigest,
		},
		Runtime:     runtime,
		GeneratedAt: generatedAt.UTC().Format(time.RFC3339Nano),
		Calls:       calls,
	}
	packetBytes, err := json.Marshal(packet)
	if err != nil {
		return p14CodexMCPRequestCarrier{}, err
	}
	packetDigest := p14Digest(packetBytes)
	body := strings.TrimPrefix(packetDigest, "sha256:")
	carrier := p14CodexMCPRequestCarrier{
		Schema: p14CodexMCPRequestCarrierSchema,
		Status: p14CodexMCPRequestStatus,
		CarrierPath: filepath.ToSlash(filepath.Join(
			".context",
			"p14",
			"p14-codex-mcp-request-"+body[:16]+".json",
		)),
		PacketDigest: packetDigest,
		Packet:       packet,
	}
	if err := validateP14CodexMCPRequestCarrier(
		contract,
		prepared,
		carrier,
	); err != nil {
		return p14CodexMCPRequestCarrier{}, err
	}
	return carrier, nil
}

func buildP14CodexMCPPlannedCalls(
	scenarios []preparedP14Scenario,
) ([]p14CodexMCPPlannedCall, error) {
	ordered, err := p14CodexMCPScenarioExecutionOrder(scenarios)
	if err != nil {
		return nil, err
	}
	calls := make([]p14CodexMCPPlannedCall, 0)
	sequence := 1
	for _, scenario := range ordered {
		request, present := p14PreparedSurfaceRequest(scenario, "live_mcp")
		if !present {
			return nil, fmt.Errorf(
				"P14 Codex MCP ordered scenario %q has no live surface",
				scenario.ID,
			)
		}
		definitions, err := p14CodexMCPCallDefinitions(scenario, request)
		if err != nil {
			return nil, err
		}
		exchangeCalls, nextSequence, err :=
			buildP14CodexMCPExchangePlannedCalls(
				scenario,
				request,
				definitions,
				sequence,
			)
		if err != nil {
			return nil, err
		}
		calls = append(calls, exchangeCalls...)
		sequence = nextSequence
	}
	return calls, nil
}

func p14CodexMCPScenarioExecutionOrder(
	scenarios []preparedP14Scenario,
) ([]preparedP14Scenario, error) {
	noEffect := make([]preparedP14Scenario, 0)
	writes := make(map[string]preparedP14Scenario)
	var agentMemory preparedP14Scenario
	for _, scenario := range scenarios {
		if _, present := p14PreparedSurfaceRequest(scenario, "live_mcp"); !present {
			continue
		}
		switch scenario.ID {
		case "positive_typed_write", "concurrency_idempotency":
			writes[scenario.ID] = scenario
		case "agent_typed_memory_orientation":
			agentMemory = scenario
		default:
			agentObservation := scenario.ID ==
				"agent_code_graph_orientation"
			if scenario.Oracle.ExpectedEffect != "none" &&
				!agentObservation {
				return nil, fmt.Errorf(
					"P14 live MCP scenario %q has an open effect %q",
					scenario.ID,
					scenario.Oracle.ExpectedEffect,
				)
			}
			noEffect = append(noEffect, scenario)
		}
	}
	if agentMemory.ID == "" {
		return nil, fmt.Errorf(
			"P14 live MCP execution order omits %q",
			"agent_typed_memory_orientation",
		)
	}
	for _, required := range []string{
		"positive_typed_write",
		"concurrency_idempotency",
	} {
		scenario, present := writes[required]
		if !present {
			return nil, fmt.Errorf(
				"P14 live MCP execution order omits %q",
				required,
			)
		}
		noEffect = append(noEffect, scenario)
	}
	noEffect = append(noEffect, agentMemory)
	return noEffect, nil
}

func p14CodexMCPCallDefinitions(
	scenario preparedP14Scenario,
	request preparedP14Request,
) ([]p14CodexMCPCallDefinition, error) {
	builders := map[string]func(
		preparedP14Scenario,
		preparedP14Request,
	) ([]p14CodexMCPCallDefinition, error){
		p14RuntimeIdentityBuilderID:     p14CodexMCPLiveProtocolCalls,
		p14FPFProjectionBuilderID:       p14CodexMCPFPFCalls,
		p14IdentifierNamespaceBuilderID: p14CodexMCPIdentifierCalls,
		p14SpecSectionProtocolBuilderID: p14CodexMCPSpecSectionCalls,
	}
	for _, builderID := range p14CodeExploreBuilderIDs {
		builders[builderID] = p14CodexMCPCodeExploreCalls
	}
	for _, builderID := range p14AgentOrientationBuilderIDs {
		builders[builderID] = p14CodexMCPLiveProtocolCalls
	}
	for _, builderID := range p14MemoryReadBuilderIDs {
		builders[builderID] = p14CodexMCPMemoryReadCalls
	}
	for _, builderID := range p14MemoryOperationBuilderIDs {
		builders[builderID] = p14CodexMCPMemoryOperationCalls
	}
	builder := builders[request.Builder]
	if builder == nil {
		return nil, fmt.Errorf(
			"P14 live MCP builder %q has no call extractor",
			request.Builder,
		)
	}
	return builder(scenario, request)
}

func p14CodexMCPLiveProtocolCalls(
	scenario preparedP14Scenario,
	request preparedP14Request,
) ([]p14CodexMCPCallDefinition, error) {
	surface := p14LiveProtocolSurface{}
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"actual Codex MCP live protocol",
	); err != nil {
		return nil, err
	}
	policy, err := p14LiveProtocolPolicyForScenario(scenario.ID)
	if err != nil {
		return nil, err
	}
	if request.Builder != policy.BuilderID ||
		surface.Surface != "live_mcp" ||
		surface.Probe == nil {
		return nil, fmt.Errorf("P14 live MCP protocol has no exact probe")
	}
	agentScenario := slices.Contains(
		p14AgentOrientationBuilderIDs,
		request.Builder,
	)
	if agentScenario && surface.AgentPrompt == nil {
		return nil, fmt.Errorf(
			"P14 agent live MCP protocol has no prompt",
		)
	}
	if !agentScenario &&
		(surface.AgentPrompt != nil ||
			surface.PersistencePrompt != nil) {
		return nil, fmt.Errorf(
			"P14 non-agent live MCP protocol has an agent prompt",
		)
	}
	args, err := cloneP14JSONMap(surface.Probe.Args)
	if err != nil {
		return nil, err
	}
	if scenario.ID == "runtime_identity" {
		return []p14CodexMCPCallDefinition{{
			CaseID: "status_probe",
			Tool:   surface.Probe.Tool,
			Args:   args,
		}}, nil
	}
	prompt, err := p14CodexMCPAgentPrompt(
		*surface.AgentPrompt,
	)
	if err != nil {
		return nil, err
	}
	if scenario.ID == "agent_code_graph_orientation" {
		return []p14CodexMCPCallDefinition{{
			CaseID:      "orientation_probe",
			Tool:        surface.Probe.Tool,
			Args:        args,
			AgentPrompt: &prompt,
		}}, nil
	}
	if scenario.ID != "agent_typed_memory_orientation" {
		return nil, fmt.Errorf(
			"P14 agent live MCP scenario %q is unknown",
			scenario.ID,
		)
	}
	if surface.PersistencePrompt == nil {
		return nil, fmt.Errorf(
			"P14 agent memory live protocol has no explicit-save prompt",
		)
	}
	persistencePrompt, err := p14CodexMCPAgentPrompt(
		*surface.PersistencePrompt,
	)
	if err != nil {
		return nil, err
	}
	beforeArgs, err := cloneP14JSONMap(surface.Probe.Args)
	if err != nil {
		return nil, err
	}
	afterArgs, err := cloneP14JSONMap(surface.Probe.Args)
	if err != nil {
		return nil, err
	}
	establishArgs := p14AgentMemoryEstablishArgs()
	replayArgs, err := cloneP14JSONMap(establishArgs)
	if err != nil {
		return nil, err
	}
	definitions := []p14CodexMCPCallDefinition{
		{
			CaseID: "basis_before",
			Tool:   surface.Probe.Tool,
			Args:   beforeArgs,
		},
		{
			CaseID:      "orientation_probe",
			Tool:        surface.Probe.Tool,
			Args:        args,
			AgentPrompt: &prompt,
		},
		{
			CaseID: "basis_after",
			Tool:   surface.Probe.Tool,
			Args:   afterArgs,
		},
		{
			CaseID:      "explicit_save_establish",
			Tool:        "haft_entity",
			Args:        establishArgs,
			AgentPrompt: &persistencePrompt,
		},
		{
			CaseID: "establish_replay",
			Tool:   "haft_entity",
			Args:   replayArgs,
		},
		{
			CaseID: "resolve_exact",
			Tool:   "haft_query",
			Args:   p14AgentMemoryResolveExactArgs(),
		},
		{
			CaseID: "neighborhood",
			Tool:   "haft_query",
			Args:   p14AgentMemoryNeighborhoodArgs(),
		},
		{
			CaseID: "recall",
			Tool:   "haft_query",
			Args:   p14AgentMemoryRecallArgs(),
		},
	}
	for index := range definitions {
		definitions[index].RequiredPredecessors =
			[]string{"concurrency_idempotency"}
	}
	return definitions, nil
}

func p14CodexMCPAgentPrompt(
	prompt p14LiveProtocolAgentPrompt,
) (p14CodexMCPPlannedAgentPrompt, error) {
	if prompt.ID == "" ||
		strings.TrimSpace(prompt.Text) == "" ||
		prompt.Text != strings.TrimSpace(prompt.Text) {
		return p14CodexMCPPlannedAgentPrompt{}, fmt.Errorf(
			"P14 agent prompt is invalid",
		)
	}
	return p14CodexMCPPlannedAgentPrompt{
		ID:                    prompt.ID,
		TextCanonical:         prompt.Text,
		TextDigest:            p14Digest([]byte(prompt.Text)),
		ExpectedToolCallCount: 1,
	}, nil
}

func p14CodexMCPFPFCalls(
	_ preparedP14Scenario,
	request preparedP14Request,
) ([]p14CodexMCPCallDefinition, error) {
	surface := p14FPFProjectionMCPSurface{}
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"actual Codex MCP FPF surface",
	); err != nil {
		return nil, err
	}
	result := make([]p14CodexMCPCallDefinition, 0, len(surface.Cases))
	for _, testCase := range surface.Cases {
		args, err := cloneP14JSONMap(testCase.Args)
		if err != nil {
			return nil, err
		}
		result = append(result, p14CodexMCPCallDefinition{
			CaseID: testCase.ID,
			Tool:   surface.Tool,
			Args:   args,
		})
	}
	return result, nil
}

func p14CodexMCPMemoryReadCalls(
	_ preparedP14Scenario,
	request preparedP14Request,
) ([]p14CodexMCPCallDefinition, error) {
	surface := p14MemoryReadMCPSurface{}
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"actual Codex MCP memory-read surface",
	); err != nil {
		return nil, err
	}
	result := make([]p14CodexMCPCallDefinition, 0, len(surface.Cases))
	for _, testCase := range surface.Cases {
		args, err := cloneP14JSONMap(testCase.Args)
		if err != nil {
			return nil, err
		}
		result = append(result, p14CodexMCPCallDefinition{
			CaseID: testCase.ID,
			Tool:   surface.Tool,
			Args:   args,
		})
	}
	return result, nil
}

func p14CodexMCPMemoryOperationCalls(
	_ preparedP14Scenario,
	request preparedP14Request,
) ([]p14CodexMCPCallDefinition, error) {
	surface := p14MemoryOperationMCPSurface{}
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"actual Codex MCP memory-operation surface",
	); err != nil {
		return nil, err
	}
	if surface.ExecutionContext != p14MemoryOperationLiveMCPExecution {
		return nil, fmt.Errorf("P14 live MCP operation context differs")
	}
	result := make([]p14CodexMCPCallDefinition, 0, len(surface.Calls))
	for _, call := range surface.Calls {
		args, err := cloneP14JSONMap(call.Args)
		if err != nil {
			return nil, err
		}
		result = append(result, p14CodexMCPCallDefinition{
			CaseID:               call.ID,
			ParallelGroup:        call.ParallelGroup,
			RequiredPredecessors: slices.Clone(surface.RequiredPredecessors),
			Tool:                 call.Tool,
			Args:                 args,
		})
	}
	return result, nil
}

func p14CodexMCPIdentifierCalls(
	_ preparedP14Scenario,
	request preparedP14Request,
) ([]p14CodexMCPCallDefinition, error) {
	surface := p14IdentifierMCPSurface{}
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"actual Codex MCP identifier surface",
	); err != nil {
		return nil, err
	}
	result := make([]p14CodexMCPCallDefinition, 0, len(surface.Cases))
	for _, testCase := range surface.Cases {
		args, err := cloneP14JSONMap(testCase.Args)
		if err != nil {
			return nil, err
		}
		result = append(result, p14CodexMCPCallDefinition{
			CaseID: testCase.ID,
			Tool:   surface.Tool,
			Args:   args,
		})
	}
	return result, nil
}

func p14CodexMCPSpecSectionCalls(
	_ preparedP14Scenario,
	request preparedP14Request,
) ([]p14CodexMCPCallDefinition, error) {
	surface := p14SpecSectionProtocolMCPSurface{}
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"actual Codex MCP SpecSection surface",
	); err != nil {
		return nil, err
	}
	result := make([]p14CodexMCPCallDefinition, 0, len(surface.Cases))
	for _, testCase := range surface.Cases {
		args, err := cloneP14JSONMap(testCase.Args)
		if err != nil {
			return nil, err
		}
		result = append(result, p14CodexMCPCallDefinition{
			CaseID: testCase.ID,
			Tool:   surface.Tool,
			Args:   args,
		})
	}
	return result, nil
}

func cloneP14JSONMap(input map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("clone P14 MCP arguments: %w", err)
	}
	cloned := map[string]any{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&cloned); err != nil {
		return nil, fmt.Errorf("decode cloned P14 MCP arguments: %w", err)
	}
	return cloned, nil
}

func validateP14CodexMCPRequestCarrier(
	contract requestOracleContract,
	prepared preparedRequestOracleCarrier,
	carrier p14CodexMCPRequestCarrier,
) error {
	packet := carrier.Packet
	if carrier.Schema != p14CodexMCPRequestCarrierSchema ||
		carrier.Status != p14CodexMCPRequestStatus ||
		!validP14Digest(carrier.PacketDigest) ||
		packet.Schema != p14CodexMCPRequestInputSchema ||
		packet.Status != p14CodexMCPRequestStatus ||
		packet.ResultSemantics != p14CodexMCPRequestSemantics ||
		packet.ReleaseClaim {
		return fmt.Errorf("P14 Codex MCP request header is invalid")
	}
	if _, err := p14CodexCaptureWindowStart(packet); err != nil {
		return fmt.Errorf("P14 Codex MCP request time is invalid: %w", err)
	}
	if err := validateP14PreparedObservationBinding(
		packet.PreparedCarrier,
	); err != nil {
		return err
	}
	if err := verifyP14PreparedObservationBindingValue(
		packet.PreparedCarrier.CarrierPath,
		packet.PreparedCarrier.CarrierDigest,
		prepared,
	); err != nil {
		return err
	}
	if err := validateP14RuntimeObservationBinding(
		prepared.Preparation,
		packet.Runtime,
	); err != nil {
		return err
	}
	expected, err := buildP14CodexMCPPlannedCalls(
		prepared.Preparation.Scenarios,
	)
	if err != nil {
		return err
	}
	expectedBytes, err := marshalP14CanonicalJSON(expected)
	if err != nil {
		return err
	}
	actualBytes, err := marshalP14CanonicalJSON(packet.Calls)
	if err != nil {
		return err
	}
	if !bytes.Equal(actualBytes, expectedBytes) {
		return fmt.Errorf("P14 Codex MCP request calls differ")
	}
	if err := validateP14CodexMCPPlannedCalls(packet.Calls); err != nil {
		return err
	}
	packetBytes, err := json.Marshal(packet)
	if err != nil {
		return err
	}
	wantDigest := p14Digest(packetBytes)
	body := strings.TrimPrefix(wantDigest, "sha256:")
	wantPath := filepath.ToSlash(filepath.Join(
		".context",
		"p14",
		"p14-codex-mcp-request-"+body[:16]+".json",
	))
	if carrier.PacketDigest != wantDigest ||
		carrier.CarrierPath != wantPath ||
		len(contract.Scenarios) != len(prepared.Preparation.Scenarios) {
		return fmt.Errorf("P14 Codex MCP request path or basis differs")
	}
	canonical, err := json.Marshal(carrier)
	if err != nil {
		return err
	}
	if bytes.Contains(canonical, []byte(`"nonce"`)) {
		return fmt.Errorf("P14 Codex MCP request exposes a private nonce")
	}
	return nil
}

func validateP14CodexMCPPlannedCalls(
	calls []p14CodexMCPPlannedCall,
) error {
	seenCases := make(map[string]struct{}, len(calls))
	seenScenarios := make(map[string]struct{})
	seenPrompts := make(map[string]struct{})
	promptScenarios := make(map[string]struct{})
	for index, call := range calls {
		if call.Sequence != index+1 ||
			call.ScenarioID == "" ||
			call.Builder == "" ||
			call.CaseID == "" ||
			call.ExchangeID == "" ||
			!slices.Contains(
				p14CodexMCPExchangeRoles,
				call.ExchangeRole,
			) ||
			call.Tool == "" ||
			!canonicalCompactJSON([]byte(call.ArgsCanonical)) ||
			p14Digest([]byte(call.ArgsCanonical)) != call.ArgsDigest ||
			!validP14Digest(call.RequestPayloadDigest) {
			return fmt.Errorf("P14 Codex MCP planned call %d is invalid", index+1)
		}
		key := call.ScenarioID + "/" + call.CaseID
		if _, duplicate := seenCases[key]; duplicate {
			return fmt.Errorf("P14 Codex MCP repeats call %q", key)
		}
		for _, predecessor := range call.RequiredPredecessors {
			if _, observed := seenScenarios[predecessor]; !observed {
				return fmt.Errorf(
					"P14 Codex MCP call %q precedes required scenario %q",
					key,
					predecessor,
				)
			}
		}
		if call.AgentPrompt != nil {
			if call.ExchangeRole != p14CodexMCPExchangeTarget {
				return fmt.Errorf(
					"P14 agent prompt appears outside a target call",
				)
			}
			prompt := *call.AgentPrompt
			promptCase := call.CaseID == "orientation_probe" ||
				call.CaseID == "explicit_save_establish"
			if !promptCase ||
				prompt.ID == "" ||
				strings.TrimSpace(prompt.TextCanonical) == "" ||
				prompt.TextCanonical !=
					strings.TrimSpace(prompt.TextCanonical) ||
				p14Digest([]byte(prompt.TextCanonical)) !=
					prompt.TextDigest ||
				prompt.ExpectedToolCallCount != 1 {
				return fmt.Errorf(
					"P14 agent prompt differs for %q",
					key,
				)
			}
			if _, duplicate := seenPrompts[prompt.ID]; duplicate {
				return fmt.Errorf(
					"P14 agent prompt %q is duplicated",
					prompt.ID,
				)
			}
			seenPrompts[prompt.ID] = struct{}{}
			promptScenarios[call.ScenarioID] = struct{}{}
		}
		seenCases[key] = struct{}{}
		seenScenarios[call.ScenarioID] = struct{}{}
	}
	if err := validateP14CodexMCPExchangePlan(calls); err != nil {
		return err
	}
	for _, scenarioID := range []string{
		"agent_code_graph_orientation",
		"agent_typed_memory_orientation",
	} {
		if _, present := promptScenarios[scenarioID]; !present {
			return fmt.Errorf(
				"P14 agent scenario %q has no prompt",
				scenarioID,
			)
		}
	}
	return nil
}

func buildP14CodexMCPCaptureCarrier(
	contract requestOracleContract,
	prepared preparedRequestOracleCarrier,
	packet p14CodexMCPRequestCarrier,
	input p14CodexMCPCaptureInput,
	sessionHistory p14CodexSessionHistoryEvidence,
	protocolDiscovery p14MCPProtocolDiscovery,
) (p14CodexMCPCaptureCarrier, error) {
	if err := validateP14CodexMCPCaptureInput(packet, input); err != nil {
		return p14CodexMCPCaptureCarrier{}, err
	}
	if err := validateP14CodexSessionHistoryEvidence(
		packet,
		input,
		sessionHistory,
	); err != nil {
		return p14CodexMCPCaptureCarrier{}, err
	}
	if err := validateP14MCPProtocolDiscovery(
		packet.Packet.Runtime,
		protocolDiscovery,
	); err != nil {
		return p14CodexMCPCaptureCarrier{}, err
	}
	byScenario := make(map[string][]p14CodexMCPCallEvidence)
	for _, call := range input.Calls {
		byScenario[call.ScenarioID] = append(
			byScenario[call.ScenarioID],
			call,
		)
	}
	normalizers := p14CodexMCPFamilyNormalizers()
	scenarioCaptures := make([]p14CodexMCPScenarioCapture, 0)
	for _, scenario := range prepared.Preparation.Scenarios {
		request, present := p14PreparedSurfaceRequest(scenario, "live_mcp")
		if !present {
			continue
		}
		normalizer := normalizers[request.Builder]
		if normalizer == nil {
			return p14CodexMCPCaptureCarrier{}, fmt.Errorf(
				"P14 Codex MCP builder %q has no normalizer",
				request.Builder,
			)
		}
		evidence := byScenario[scenario.ID]
		targetEvidence := p14CodexMCPTargetCalls(evidence)
		result, err := normalizer(
			prepared,
			scenario,
			request,
			targetEvidence,
		)
		if err != nil {
			return p14CodexMCPCaptureCarrier{}, err
		}
		observation, err := p14CodexMCPScenarioObservation(
			packet,
			scenario,
			request,
			evidence,
			result,
			protocolDiscovery.EvidenceDigest,
		)
		if err != nil {
			return p14CodexMCPCaptureCarrier{}, err
		}
		scenarioCaptures = append(
			scenarioCaptures,
			p14CodexMCPScenarioCapture{
				ID:                    scenario.ID,
				SemanticRequestDigest: scenario.SemanticRequestDigest,
				SurfaceObservation:    observation,
			},
		)
	}
	carrier := p14CodexMCPCaptureCarrier{
		Schema:            p14CodexMCPCaptureCarrierSchema,
		Status:            p14CodexMCPCaptureStatus,
		RequestPacket:     input.RequestPacket,
		CapturedAt:        input.CapturedAt,
		SessionHistory:    sessionHistory,
		ProtocolDiscovery: protocolDiscovery,
		ScenarioCaptures:  scenarioCaptures,
	}
	digestBasis, err := p14CodexMCPCaptureDigestBasis(carrier)
	if err != nil {
		return p14CodexMCPCaptureCarrier{}, err
	}
	carrier.CaptureDigest = p14Digest(digestBasis)
	body := strings.TrimPrefix(carrier.CaptureDigest, "sha256:")
	carrier.CarrierPath = filepath.ToSlash(filepath.Join(
		".context",
		"p14",
		"p14-codex-mcp-capture-"+body[:16]+".json",
	))
	if err := validateP14CodexMCPCaptureCarrier(
		contract,
		prepared,
		packet,
		carrier,
	); err != nil {
		return p14CodexMCPCaptureCarrier{}, err
	}
	return carrier, nil
}

func p14CodexMCPScenarioObservation(
	packet p14CodexMCPRequestCarrier,
	scenario preparedP14Scenario,
	request preparedP14Request,
	evidence []p14CodexMCPCallEvidence,
	result p14CodexMCPFamilyResult,
	protocolDiscoveryDigest string,
) (p14InstalledSurfaceObservation, error) {
	runtimeProtocolDigest := ""
	if scenario.ID == "runtime_identity" {
		if !validP14Digest(protocolDiscoveryDigest) {
			return p14InstalledSurfaceObservation{}, fmt.Errorf(
				"P14 runtime identity omits MCP protocol discovery",
			)
		}
		runtimeProtocolDigest = protocolDiscoveryDigest
	}
	receipt := p14CodexMCPScenarioReceipt{
		Schema:                  p14CodexMCPReceiptSchema,
		ScenarioID:              scenario.ID,
		Builder:                 request.Builder,
		ThreadID:                packet.Packet.Runtime.ThreadID,
		TurnIDs:                 p14CodexMCPTurnIDs(evidence),
		RuntimeReceiptDigest:    packet.Packet.Runtime.LiveMCPReceiptDigest,
		RequestPayloadDigest:    request.PayloadDigest,
		ProtocolDiscoveryDigest: runtimeProtocolDigest,
		Calls:                   slices.Clone(evidence),
		Checks:                  slices.Clone(result.Checks),
		FailureDetail:           result.FailureDetail,
	}
	receiptBytes, err := marshalP14CanonicalJSON(receipt)
	if err != nil {
		return p14InstalledSurfaceObservation{}, err
	}
	outcome := p14SurfaceOutcomeObserved
	failureCode := ""
	normalizedDigest := ""
	if result.FailureCode != "" {
		outcome = p14SurfaceOutcomeMismatch
		failureCode = result.FailureCode
	} else if scenario.Oracle.Kind == "normalized_digest" {
		normalizedDigest = p14Digest(result.Normalized)
		if normalizedDigest != scenario.Oracle.ExpectedResultDigest {
			outcome = p14SurfaceOutcomeMismatch
			failureCode = "normalized_digest_mismatch"
			normalizedDigest = ""
		}
	}
	observedAt, err := p14LatestCodexMCPObservationTime(evidence)
	if err != nil {
		return p14InstalledSurfaceObservation{}, err
	}
	observation := p14InstalledSurfaceObservation{
		Surface:                "live_mcp",
		RequestPayloadDigest:   request.PayloadDigest,
		Source:                 p14ObservationSourceLiveMCP,
		SourceReceiptDigest:    p14Digest(receiptBytes),
		ObservedAt:             observedAt.Format(time.RFC3339Nano),
		Outcome:                outcome,
		ObservationCanonical:   string(receiptBytes),
		ObservationDigest:      p14Digest(receiptBytes),
		NormalizedResultDigest: normalizedDigest,
		FailureCode:            failureCode,
	}
	if err := validateP14InstalledSurfaceObservation(
		request,
		observation,
	); err != nil {
		return p14InstalledSurfaceObservation{}, err
	}
	return observation, nil
}

func p14CodexMCPTurnIDs(
	evidence []p14CodexMCPCallEvidence,
) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, call := range evidence {
		turnID := call.Transcript.TurnID
		if _, duplicate := seen[turnID]; duplicate {
			continue
		}
		seen[turnID] = struct{}{}
		result = append(result, turnID)
	}
	return result
}

func validateP14CodexMCPCaptureInput(
	packet p14CodexMCPRequestCarrier,
	input p14CodexMCPCaptureInput,
) error {
	if input.Schema != p14CodexMCPCaptureInputSchema ||
		input.Status != p14CodexMCPCaptureStatus ||
		input.ResultSemantics != p14CodexMCPCaptureSemantics ||
		input.ReleaseClaim ||
		input.RequestPacket.CarrierPath != packet.CarrierPath ||
		!validP14Digest(input.RequestPacket.CarrierDigest) ||
		input.RequestPacket.PacketDigest != packet.PacketDigest ||
		len(input.Calls) != len(packet.Packet.Calls) {
		return fmt.Errorf("P14 Codex MCP capture input basis differs")
	}
	if _, err := time.Parse(time.RFC3339Nano, input.CapturedAt); err != nil {
		return fmt.Errorf("P14 Codex MCP capture time is invalid: %w", err)
	}
	seenToolCalls := make(map[string]struct{}, len(input.Calls))
	seenTurnOrdinals := make(map[string]map[int]struct{})
	responseTimes := make([]time.Time, len(input.Calls))
	latestResponseByScenario := make(map[string]time.Time)
	latestResponse := time.Time{}
	for index, evidence := range input.Calls {
		planned := packet.Packet.Calls[index]
		if err := validateP14CodexMCPCallEvidence(
			packet.Packet.Runtime,
			planned,
			evidence,
		); err != nil {
			return err
		}
		if _, duplicate := seenToolCalls[evidence.Transcript.ToolCallID]; duplicate {
			return fmt.Errorf("P14 Codex MCP capture repeats a tool call")
		}
		seenToolCalls[evidence.Transcript.ToolCallID] = struct{}{}
		turnKey := evidence.Transcript.ThreadID +
			"/" +
			evidence.Transcript.TurnID
		ordinals := seenTurnOrdinals[turnKey]
		if ordinals == nil {
			ordinals = make(map[int]struct{})
			seenTurnOrdinals[turnKey] = ordinals
		}
		ordinal := evidence.Transcript.TurnToolCallOrdinal
		if _, duplicate := ordinals[ordinal]; duplicate {
			return fmt.Errorf(
				"P14 Codex MCP capture repeats a turn tool-call ordinal",
			)
		}
		ordinals[ordinal] = struct{}{}
		capturedAt, err := time.Parse(
			time.RFC3339Nano,
			evidence.Response.CapturedAt,
		)
		if err != nil {
			return fmt.Errorf(
				"P14 Codex MCP response chronology is invalid",
			)
		}
		responseTimes[index] = capturedAt
		scenarioLatest := latestResponseByScenario[planned.ScenarioID]
		if capturedAt.After(scenarioLatest) {
			latestResponseByScenario[planned.ScenarioID] = capturedAt
		}
		if evidence.ParallelGroup == "" &&
			!latestResponse.IsZero() &&
			!capturedAt.After(latestResponse) {
			return fmt.Errorf(
				"P14 Codex MCP sequential response order differs",
			)
		}
		if capturedAt.After(latestResponse) {
			latestResponse = capturedAt
		}
	}
	for index, planned := range packet.Packet.Calls {
		capturedAt := responseTimes[index]
		for _, predecessor := range planned.RequiredPredecessors {
			predecessorAt, present :=
				latestResponseByScenario[predecessor]
			if !present || !capturedAt.After(predecessorAt) {
				return fmt.Errorf(
					"P14 Codex MCP response for %s/%s does not follow required predecessor %q",
					planned.ScenarioID,
					planned.CaseID,
					predecessor,
				)
			}
		}
	}
	return nil
}

func validateP14CodexMCPCallEvidence(
	runtime p14RuntimeObservationBinding,
	planned p14CodexMCPPlannedCall,
	evidence p14CodexMCPCallEvidence,
) error {
	transcript := evidence.Transcript
	response := evidence.Response
	if evidence.Sequence != planned.Sequence ||
		evidence.ScenarioID != planned.ScenarioID ||
		evidence.CaseID != planned.CaseID ||
		evidence.ExchangeID != planned.ExchangeID ||
		evidence.ExchangeRole != planned.ExchangeRole ||
		evidence.ParallelGroup != planned.ParallelGroup ||
		transcript.ThreadID != runtime.ThreadID ||
		strings.TrimSpace(transcript.TurnID) == "" ||
		strings.TrimSpace(transcript.ToolCallID) == "" ||
		transcript.Server != "haft" ||
		transcript.Tool != planned.Tool ||
		transcript.ArgsCanonical != planned.ArgsCanonical ||
		transcript.ArgsDigest != planned.ArgsDigest ||
		transcript.DurationMilliseconds < 0 ||
		transcript.TurnToolCallOrdinal <= 0 ||
		transcript.TurnToolCallCount <= 0 ||
		transcript.TurnToolCallOrdinal >
			transcript.TurnToolCallCount ||
		response.ToolCallID != transcript.ToolCallID ||
		response.BodyDigest == "" {
		return fmt.Errorf(
			"P14 Codex MCP call evidence differs for %s/%s",
			planned.ScenarioID,
			planned.CaseID,
		)
	}
	if transcript.Status != "completed" && transcript.Status != "failed" {
		return fmt.Errorf("P14 Codex MCP transcript status is open")
	}
	if response.IsError != (transcript.Status == "failed") {
		return fmt.Errorf("P14 Codex MCP response error posture differs")
	}
	if err := validateP14CodexMCPPromptEvidence(
		planned.AgentPrompt,
		evidence.AgentPrompt,
		transcript,
	); err != nil {
		return err
	}
	historyReadAt, historyErr := time.Parse(
		time.RFC3339Nano,
		transcript.HistoryReadAt,
	)
	capturedAt, captureErr := time.Parse(
		time.RFC3339Nano,
		response.CapturedAt,
	)
	liveStartedAt, liveErr := time.Parse(
		time.RFC3339Nano,
		runtime.LiveMCPStartedAt,
	)
	if historyErr != nil ||
		captureErr != nil ||
		liveErr != nil ||
		capturedAt.Before(liveStartedAt) ||
		historyReadAt.Before(capturedAt) {
		return fmt.Errorf("P14 Codex MCP call evidence time is invalid")
	}
	body, err := base64.StdEncoding.DecodeString(response.BodyBase64)
	if err != nil ||
		len(body) == 0 ||
		len(body) > p14CodexMCPResponseLimit ||
		p14Digest(body) != response.BodyDigest {
		return fmt.Errorf("P14 Codex MCP response body differs")
	}
	return nil
}

func validateP14CodexMCPPromptEvidence(
	planned *p14CodexMCPPlannedAgentPrompt,
	evidence *p14CodexMCPPromptTranscriptProjection,
	tool p14CodexMCPTranscriptProjection,
) error {
	if planned == nil && evidence == nil {
		return nil
	}
	if planned == nil || evidence == nil {
		return fmt.Errorf("P14 Codex MCP prompt evidence is absent")
	}
	if evidence.PromptID != planned.ID ||
		evidence.ThreadID != tool.ThreadID ||
		evidence.TurnID != tool.TurnID ||
		evidence.Role != "user" ||
		evidence.TextCanonical != planned.TextCanonical ||
		evidence.TextDigest != planned.TextDigest ||
		p14Digest([]byte(evidence.TextCanonical)) !=
			evidence.TextDigest ||
		tool.TurnToolCallOrdinal != 1 ||
		tool.TurnToolCallCount !=
			planned.ExpectedToolCallCount {
		return fmt.Errorf(
			"P14 Codex MCP prompt-driven tool evidence differs",
		)
	}
	historyReadAt, err := time.Parse(
		time.RFC3339Nano,
		evidence.HistoryReadAt,
	)
	if err != nil {
		return fmt.Errorf(
			"P14 Codex MCP prompt history time is invalid",
		)
	}
	toolHistoryReadAt, err := time.Parse(
		time.RFC3339Nano,
		tool.HistoryReadAt,
	)
	if err != nil || historyReadAt != toolHistoryReadAt {
		return fmt.Errorf(
			"P14 Codex MCP prompt and tool history reads differ",
		)
	}
	return nil
}

func p14LatestCodexMCPObservationTime(
	evidence []p14CodexMCPCallEvidence,
) (time.Time, error) {
	latest := time.Time{}
	for _, call := range evidence {
		capturedAt, err := time.Parse(
			time.RFC3339Nano,
			call.Response.CapturedAt,
		)
		if err != nil {
			return time.Time{}, err
		}
		if capturedAt.After(latest) {
			latest = capturedAt
		}
	}
	if latest.IsZero() {
		return time.Time{}, fmt.Errorf("P14 Codex MCP observation time is absent")
	}
	return latest.UTC(), nil
}

func validateP14CodexMCPCaptureCarrier(
	contract requestOracleContract,
	prepared preparedRequestOracleCarrier,
	packet p14CodexMCPRequestCarrier,
	carrier p14CodexMCPCaptureCarrier,
) error {
	_, err := validateAndRecomputeP14CodexMCPCaptureCarrier(
		contract,
		prepared,
		packet,
		carrier,
	)
	return err
}

func validateAndRecomputeP14CodexMCPCaptureCarrier(
	contract requestOracleContract,
	prepared preparedRequestOracleCarrier,
	packet p14CodexMCPRequestCarrier,
	carrier p14CodexMCPCaptureCarrier,
) ([]p14CodexMCPScenarioCapture, error) {
	if carrier.Schema != p14CodexMCPCaptureCarrierSchema ||
		carrier.Status != p14CodexMCPCaptureStatus ||
		!validP14Digest(carrier.CaptureDigest) ||
		carrier.RequestPacket.CarrierPath != packet.CarrierPath ||
		carrier.RequestPacket.PacketDigest != packet.PacketDigest {
		return nil, fmt.Errorf("P14 Codex MCP capture header differs")
	}
	if err := validateP14MCPProtocolDiscovery(
		packet.Packet.Runtime,
		carrier.ProtocolDiscovery,
	); err != nil {
		return nil, err
	}
	if _, err := time.Parse(time.RFC3339Nano, carrier.CapturedAt); err != nil {
		return nil, fmt.Errorf(
			"P14 Codex MCP carrier time is invalid: %w",
			err,
		)
	}
	expectedScenarios := p14CodexMCPScenarios(prepared.Preparation.Scenarios)
	if len(carrier.ScenarioCaptures) != len(expectedScenarios) {
		return nil, fmt.Errorf("P14 Codex MCP scenario count differs")
	}
	recomputed := make(
		[]p14CodexMCPScenarioCapture,
		0,
		len(expectedScenarios),
	)
	orderedCalls := make(
		[]p14CodexMCPCallEvidence,
		len(packet.Packet.Calls),
	)
	seenCalls := make([]bool, len(packet.Packet.Calls))
	for index, capture := range carrier.ScenarioCaptures {
		scenario := expectedScenarios[index]
		request, _ := p14PreparedSurfaceRequest(scenario, "live_mcp")
		if capture.ID != scenario.ID ||
			capture.SemanticRequestDigest != scenario.SemanticRequestDigest {
			return nil, fmt.Errorf("P14 Codex MCP scenario order differs")
		}
		if err := validateP14InstalledSurfaceObservation(
			request,
			capture.SurfaceObservation,
		); err != nil {
			return nil, err
		}
		observation, calls, err :=
			recomputeP14CodexMCPScenarioObservation(
				prepared,
				packet,
				scenario,
				request,
				capture.SurfaceObservation,
				carrier.ProtocolDiscovery.EvidenceDigest,
			)
		if err != nil {
			return nil, err
		}
		recomputed = append(recomputed, p14CodexMCPScenarioCapture{
			ID:                    scenario.ID,
			SemanticRequestDigest: scenario.SemanticRequestDigest,
			SurfaceObservation:    observation,
		})
		for _, call := range calls {
			slot := call.Sequence - 1
			if slot < 0 ||
				slot >= len(orderedCalls) ||
				seenCalls[slot] {
				return nil, fmt.Errorf(
					"P14 Codex MCP receipt call sequence differs",
				)
			}
			orderedCalls[slot] = call
			seenCalls[slot] = true
		}
	}
	for _, seen := range seenCalls {
		if !seen {
			return nil, fmt.Errorf(
				"P14 Codex MCP receipt call sequence is incomplete",
			)
		}
	}
	reconstructedInput := p14CodexMCPCaptureInput{
		Schema:          p14CodexMCPCaptureInputSchema,
		Status:          p14CodexMCPCaptureStatus,
		ResultSemantics: p14CodexMCPCaptureSemantics,
		ReleaseClaim:    false,
		RequestPacket:   carrier.RequestPacket,
		CapturedAt:      carrier.CapturedAt,
		Calls:           orderedCalls,
	}
	if err := validateP14CodexMCPCaptureInput(
		packet,
		reconstructedInput,
	); err != nil {
		return nil, err
	}
	if err := validateP14CodexSessionHistoryEvidence(
		packet,
		reconstructedInput,
		carrier.SessionHistory,
	); err != nil {
		return nil, err
	}
	digestBasis, err := p14CodexMCPCaptureDigestBasis(carrier)
	if err != nil {
		return nil, err
	}
	wantDigest := p14Digest(digestBasis)
	body := strings.TrimPrefix(wantDigest, "sha256:")
	wantPath := filepath.ToSlash(filepath.Join(
		".context",
		"p14",
		"p14-codex-mcp-capture-"+body[:16]+".json",
	))
	if carrier.CaptureDigest != wantDigest ||
		carrier.CarrierPath != wantPath ||
		len(contract.Scenarios) != len(prepared.Preparation.Scenarios) {
		return nil, fmt.Errorf(
			"P14 Codex MCP carrier digest or path differs",
		)
	}
	canonical, err := json.Marshal(carrier)
	if err != nil {
		return nil, err
	}
	if bytes.Contains(canonical, []byte(`"nonce"`)) {
		return nil, fmt.Errorf(
			"P14 Codex MCP capture exposes a private nonce",
		)
	}
	return recomputed, nil
}

func recomputeP14CodexMCPScenarioObservation(
	prepared preparedRequestOracleCarrier,
	packet p14CodexMCPRequestCarrier,
	scenario preparedP14Scenario,
	request preparedP14Request,
	observation p14InstalledSurfaceObservation,
	protocolDiscoveryDigest string,
) (
	p14InstalledSurfaceObservation,
	[]p14CodexMCPCallEvidence,
	error,
) {
	receipt := p14CodexMCPScenarioReceipt{}
	if err := decodeP14StrictCompactJSON(
		observation.ObservationCanonical,
		&receipt,
		"actual Codex MCP scenario receipt",
	); err != nil {
		return p14InstalledSurfaceObservation{}, nil, err
	}
	if receipt.Schema != p14CodexMCPReceiptSchema ||
		receipt.ScenarioID != scenario.ID ||
		receipt.Builder != request.Builder ||
		receipt.ThreadID != packet.Packet.Runtime.ThreadID ||
		len(receipt.TurnIDs) == 0 ||
		receipt.RuntimeReceiptDigest !=
			packet.Packet.Runtime.LiveMCPReceiptDigest ||
		receipt.RequestPayloadDigest != request.PayloadDigest ||
		len(receipt.Calls) == 0 ||
		len(receipt.Checks) == 0 {
		return p14InstalledSurfaceObservation{}, nil, fmt.Errorf(
			"P14 Codex MCP scenario receipt basis differs",
		)
	}
	expectedProtocolDigest := ""
	if scenario.ID == "runtime_identity" {
		expectedProtocolDigest = protocolDiscoveryDigest
	}
	if receipt.ProtocolDiscoveryDigest != expectedProtocolDigest {
		return p14InstalledSurfaceObservation{}, nil, fmt.Errorf(
			"P14 Codex MCP scenario protocol discovery binding differs",
		)
	}
	planned := make([]p14CodexMCPPlannedCall, 0)
	for _, call := range packet.Packet.Calls {
		if call.ScenarioID == scenario.ID {
			planned = append(planned, call)
		}
	}
	if len(planned) != len(receipt.Calls) ||
		!slices.Equal(
			receipt.TurnIDs,
			p14CodexMCPTurnIDs(receipt.Calls),
		) {
		return p14InstalledSurfaceObservation{}, nil, fmt.Errorf(
			"P14 Codex MCP scenario receipt calls or turns differ",
		)
	}
	for index, call := range receipt.Calls {
		if err := validateP14CodexMCPCallEvidence(
			packet.Packet.Runtime,
			planned[index],
			call,
		); err != nil {
			return p14InstalledSurfaceObservation{}, nil, err
		}
	}
	normalizer := p14CodexMCPFamilyNormalizers()[request.Builder]
	if normalizer == nil {
		return p14InstalledSurfaceObservation{}, nil, fmt.Errorf(
			"P14 Codex MCP builder %q has no normalizer",
			request.Builder,
		)
	}
	targetCalls := p14CodexMCPTargetCalls(receipt.Calls)
	result, err := normalizer(
		prepared,
		scenario,
		request,
		targetCalls,
	)
	if err != nil {
		return p14InstalledSurfaceObservation{}, nil, err
	}
	if !slices.Equal(receipt.Checks, result.Checks) {
		return p14InstalledSurfaceObservation{}, nil, fmt.Errorf(
			"P14 Codex MCP receipt checks differ from the exact family normalizer policy",
		)
	}
	recomputed, err := p14CodexMCPScenarioObservation(
		packet,
		scenario,
		request,
		receipt.Calls,
		result,
		protocolDiscoveryDigest,
	)
	if err != nil {
		return p14InstalledSurfaceObservation{}, nil, err
	}
	actualBytes, err := marshalP14CanonicalJSON(observation)
	if err != nil {
		return p14InstalledSurfaceObservation{}, nil, err
	}
	recomputedBytes, err := marshalP14CanonicalJSON(recomputed)
	if err != nil {
		return p14InstalledSurfaceObservation{}, nil, err
	}
	if !bytes.Equal(actualBytes, recomputedBytes) {
		return p14InstalledSurfaceObservation{}, nil, fmt.Errorf(
			"P14 Codex MCP observation outcome or normalized result differs from raw receipt calls",
		)
	}
	return recomputed, slices.Clone(receipt.Calls), nil
}

func p14CodexMCPCaptureDigestBasis(
	carrier p14CodexMCPCaptureCarrier,
) ([]byte, error) {
	basis := struct {
		Schema            string                         `json:"schema"`
		Status            string                         `json:"status"`
		RequestPacket     p14CodexMCPRequestBinding      `json:"request_packet"`
		CapturedAt        string                         `json:"captured_at"`
		SessionHistory    p14CodexSessionHistoryEvidence `json:"session_history"`
		ProtocolDiscovery p14MCPProtocolDiscovery        `json:"protocol_discovery"`
		ScenarioCaptures  []p14CodexMCPScenarioCapture   `json:"scenario_captures"`
	}{
		Schema:            carrier.Schema,
		Status:            carrier.Status,
		RequestPacket:     carrier.RequestPacket,
		CapturedAt:        carrier.CapturedAt,
		SessionHistory:    carrier.SessionHistory,
		ProtocolDiscovery: carrier.ProtocolDiscovery,
		ScenarioCaptures:  carrier.ScenarioCaptures,
	}
	return json.Marshal(basis)
}

func p14CodexMCPScenarios(
	scenarios []preparedP14Scenario,
) []preparedP14Scenario {
	result := make([]preparedP14Scenario, 0)
	for _, scenario := range scenarios {
		if _, present := p14PreparedSurfaceRequest(
			scenario,
			"live_mcp",
		); present {
			result = append(result, scenario)
		}
	}
	return result
}

func reconstructP14CodexMCPCaptureInput(
	packet p14CodexMCPRequestCarrier,
	carrier p14CodexMCPCaptureCarrier,
) (p14CodexMCPCaptureInput, error) {
	orderedCalls := make(
		[]p14CodexMCPCallEvidence,
		len(packet.Packet.Calls),
	)
	seen := make([]bool, len(orderedCalls))
	for _, capture := range carrier.ScenarioCaptures {
		receipt := p14CodexMCPScenarioReceipt{}
		if err := decodeP14StrictCompactJSON(
			capture.SurfaceObservation.ObservationCanonical,
			&receipt,
			"actual Codex MCP session-history receipt",
		); err != nil {
			return p14CodexMCPCaptureInput{}, err
		}
		for _, call := range receipt.Calls {
			slot := call.Sequence - 1
			if slot < 0 ||
				slot >= len(orderedCalls) ||
				seen[slot] {
				return p14CodexMCPCaptureInput{}, fmt.Errorf(
					"P14 Codex MCP reconstructed call sequence differs",
				)
			}
			orderedCalls[slot] = call
			seen[slot] = true
		}
	}
	for _, present := range seen {
		if !present {
			return p14CodexMCPCaptureInput{}, fmt.Errorf(
				"P14 Codex MCP reconstructed calls are incomplete",
			)
		}
	}
	input := p14CodexMCPCaptureInput{
		Schema:          p14CodexMCPCaptureInputSchema,
		Status:          p14CodexMCPCaptureStatus,
		ResultSemantics: p14CodexMCPCaptureSemantics,
		ReleaseClaim:    false,
		RequestPacket:   carrier.RequestPacket,
		CapturedAt:      carrier.CapturedAt,
		Calls:           orderedCalls,
	}
	if err := validateP14CodexMCPCaptureInput(packet, input); err != nil {
		return p14CodexMCPCaptureInput{}, err
	}
	return input, nil
}

func persistP14CodexMCPRequestCarrier(
	repositoryRoot string,
	contract requestOracleContract,
	prepared preparedRequestOracleCarrier,
	carrier p14CodexMCPRequestCarrier,
) (string, string, error) {
	if err := validateP14CodexMCPRequestCarrier(
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

func persistP14CodexMCPCaptureCarrier(
	repositoryRoot string,
	contract requestOracleContract,
	prepared preparedRequestOracleCarrier,
	packet p14CodexMCPRequestCarrier,
	carrier p14CodexMCPCaptureCarrier,
) (string, string, error) {
	if err := validateP14CodexMCPCaptureCarrier(
		contract,
		prepared,
		packet,
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

func loadP14CodexMCPCaptureInput(
	repositoryRoot string,
	path string,
) (p14CodexMCPCaptureInput, error) {
	clean, err := resolveP14ExecutionCarrierPath(
		repositoryRoot,
		path,
		"p14-codex-mcp-capture-input-",
	)
	if err != nil {
		return p14CodexMCPCaptureInput{}, err
	}
	raw, err := os.ReadFile(clean)
	if err != nil {
		return p14CodexMCPCaptureInput{}, err
	}
	input := p14CodexMCPCaptureInput{}
	if err := decodeP14CanonicalCarrier(
		raw,
		&input,
		"Codex MCP capture input",
	); err != nil {
		return p14CodexMCPCaptureInput{}, err
	}
	return input, nil
}

func loadP14CodexMCPPacketBinding(
	repositoryRoot string,
	contract requestOracleContract,
	binding p14CodexMCPRequestBinding,
) (
	p14CodexMCPRequestCarrier,
	preparedRequestOracleCarrier,
	error,
) {
	clean, err := resolveP14ExecutionCarrierPath(
		repositoryRoot,
		binding.CarrierPath,
		"p14-codex-mcp-request-",
	)
	if err != nil {
		return p14CodexMCPRequestCarrier{}, preparedRequestOracleCarrier{}, err
	}
	raw, err := os.ReadFile(clean)
	if err != nil {
		return p14CodexMCPRequestCarrier{}, preparedRequestOracleCarrier{}, err
	}
	if p14Digest(raw) != binding.CarrierDigest {
		return p14CodexMCPRequestCarrier{}, preparedRequestOracleCarrier{}, fmt.Errorf(
			"P14 Codex MCP request carrier bytes changed",
		)
	}
	packet := p14CodexMCPRequestCarrier{}
	if err := decodeP14CanonicalCarrier(
		raw,
		&packet,
		"Codex MCP request carrier",
	); err != nil {
		return p14CodexMCPRequestCarrier{}, preparedRequestOracleCarrier{}, err
	}
	if packet.PacketDigest != binding.PacketDigest {
		return p14CodexMCPRequestCarrier{}, preparedRequestOracleCarrier{}, fmt.Errorf(
			"P14 Codex MCP packet digest differs",
		)
	}
	preparedPath := filepath.Join(
		repositoryRoot,
		filepath.FromSlash(packet.Packet.PreparedCarrier.CarrierPath),
	)
	preparedRaw, err := os.ReadFile(preparedPath)
	if err != nil {
		return p14CodexMCPRequestCarrier{}, preparedRequestOracleCarrier{}, err
	}
	if p14Digest(preparedRaw) !=
		packet.Packet.PreparedCarrier.CarrierDigest {
		return p14CodexMCPRequestCarrier{}, preparedRequestOracleCarrier{}, fmt.Errorf(
			"P14 prepared carrier changed before MCP capture",
		)
	}
	prepared, err := decodePreparedRequestOracleCarrier(contract, preparedRaw)
	if err != nil {
		return p14CodexMCPRequestCarrier{}, preparedRequestOracleCarrier{}, err
	}
	if err := validateP14CodexMCPRequestCarrier(
		contract,
		prepared,
		packet,
	); err != nil {
		return p14CodexMCPRequestCarrier{}, preparedRequestOracleCarrier{}, err
	}
	return packet, prepared, nil
}

func decodeP14CanonicalCarrier(
	raw []byte,
	target any,
	label string,
) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode P14 %s: %w", label, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("P14 %s has trailing JSON", label)
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil {
		return err
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(raw, canonical) {
		return fmt.Errorf("P14 %s is not canonical JSON", label)
	}
	return nil
}

func TestP14CodexMCPRequestPacketClosesActualTaskCallsAndOrdering(
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
	input, err := completePreparedInputForTest(
		contract,
		p14Digest(rawContract),
	)
	if err != nil {
		t.Fatal(err)
	}
	repositoryRoot := t.TempDir()
	executable := filepath.Join(repositoryRoot, "haft")
	executableBytes := []byte("synthetic P14 installed executable")
	if err := os.WriteFile(executable, executableBytes, 0o755); err != nil {
		t.Fatal(err)
	}
	input.FrozenBasis.Candidate.ExecutablePath = executable
	input.FrozenBasis.Candidate.ExecutableDigest = p14Digest(executableBytes)
	preparedPath, preparedDigest, err := persistPreparedRequestOracleCarrier(
		repositoryRoot,
		contract,
		input,
	)
	if err != nil {
		t.Fatal(err)
	}
	preparedRaw, err := os.ReadFile(filepath.Join(
		repositoryRoot,
		filepath.FromSlash(preparedPath),
	))
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
	runtime := syntheticP14RuntimeObservationBinding(prepared.Preparation)
	runtime.InstalledExecutablePath = executable
	runtime.LiveMCPExecutablePath = executable
	generatedAt := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	packet, err := buildP14CodexMCPRequestCarrier(
		contract,
		preparedPath,
		preparedDigest,
		prepared,
		runtime,
		generatedAt,
	)
	if err != nil {
		t.Fatal(err)
	}
	if packet.Packet.ReleaseClaim ||
		packet.Packet.Status != p14CodexMCPRequestStatus ||
		len(packet.Packet.Calls) == 0 {
		t.Fatal("P14 Codex MCP request packet overstates or omits Work")
	}
	statusProbe := packet.Packet.Calls[0]
	if statusProbe.ScenarioID != "runtime_identity" ||
		statusProbe.Tool != "haft_query" ||
		!strings.Contains(statusProbe.ArgsCanonical, `"action":"status"`) {
		t.Fatal("P14 Codex MCP request omitted exact resumed-task status probe")
	}
	promptCallIndexes := make([]int, 0, 3)
	memoryOrientationCalls := 0
	for index, call := range packet.Packet.Calls {
		if call.AgentPrompt != nil {
			promptCallIndexes = append(promptCallIndexes, index)
		}
		if call.ScenarioID == "agent_typed_memory_orientation" &&
			call.ExchangeRole == p14CodexMCPExchangeTarget {
			memoryOrientationCalls++
		}
	}
	if len(promptCallIndexes) != 3 ||
		memoryOrientationCalls != 8 {
		t.Fatal(
			"P14 Codex MCP request omitted prompt-gated entity round-trip calls",
		)
	}
	positiveIndex := p14CodexMCPScenarioFirstCallIndex(
		packet.Packet.Calls,
		"positive_typed_write",
	)
	concurrencyIndex := p14CodexMCPScenarioFirstCallIndex(
		packet.Packet.Calls,
		"concurrency_idempotency",
	)
	concurrencyTargetIndex := p14CodexMCPScenarioFirstTargetCallIndex(
		packet.Packet.Calls,
		"concurrency_idempotency",
	)
	agentMemoryIndex := p14CodexMCPScenarioFirstCallIndex(
		packet.Packet.Calls,
		"agent_typed_memory_orientation",
	)
	if positiveIndex < 1 ||
		concurrencyIndex <= positiveIndex ||
		agentMemoryIndex <= concurrencyIndex {
		t.Fatal(
			"P14 Codex MCP writes and task-level entity round trip are not ordered",
		)
	}
	for _, call := range packet.Packet.Calls[concurrencyIndex:agentMemoryIndex] {
		if !slices.Equal(
			call.RequiredPredecessors,
			[]string{"positive_typed_write"},
		) {
			t.Fatal("P14 Codex MCP concurrency lost its predecessor")
		}
	}
	for _, call := range packet.Packet.Calls[agentMemoryIndex:] {
		if call.ScenarioID != "agent_typed_memory_orientation" {
			t.Fatal(
				"P14 Codex MCP entity round trip is not the final scenario",
			)
		}
		if !slices.Equal(
			call.RequiredPredecessors,
			[]string{"concurrency_idempotency"},
		) {
			t.Fatal(
				"P14 Codex MCP entity round trip lost its write predecessor",
			)
		}
	}
	packetPath, packetCarrierDigest, err := persistP14CodexMCPRequestCarrier(
		repositoryRoot,
		contract,
		prepared,
		packet,
	)
	if err != nil {
		t.Fatal(err)
	}
	capturedAt := generatedAt.Add(2 * time.Minute)
	captureInput := syntheticP14CodexMCPCaptureInput(
		packet,
		packetPath,
		packetCarrierDigest,
		capturedAt,
	)
	if err := validateP14CodexMCPCaptureInput(
		packet,
		captureInput,
	); err != nil {
		t.Fatal(err)
	}
	sessionHistory := syntheticP14CodexSessionHistoryEvidence(
		t,
		packet,
		captureInput,
	)
	protocolDiscovery, err := syntheticP14MCPProtocolDiscovery(
		runtime,
		generatedAt.Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	captureCarrier, err := buildP14CodexMCPCaptureCarrier(
		contract,
		prepared,
		packet,
		captureInput,
		sessionHistory,
		protocolDiscovery,
	)
	if err != nil {
		t.Fatal(err)
	}
	if captureCarrier.Status != p14CodexMCPCaptureStatus ||
		captureCarrier.CaptureDigest == "" ||
		len(captureCarrier.ScenarioCaptures) !=
			len(p14CodexMCPScenarios(prepared.Preparation.Scenarios)) {
		t.Fatal("P14 Codex MCP partial carrier shape differs")
	}
	capturePath, captureDigest, err := persistP14CodexMCPCaptureCarrier(
		repositoryRoot,
		contract,
		prepared,
		packet,
		captureCarrier,
	)
	if err != nil {
		t.Fatal(err)
	}
	captureRaw, err := os.ReadFile(filepath.Join(
		repositoryRoot,
		filepath.FromSlash(capturePath),
	))
	if err != nil {
		t.Fatal(err)
	}
	if p14Digest(captureRaw) != captureDigest {
		t.Fatal("P14 Codex MCP persisted carrier digest differs")
	}
	reloadedCapture := p14CodexMCPCaptureCarrier{}
	if err := decodeP14CanonicalCarrier(
		captureRaw,
		&reloadedCapture,
		"Codex MCP capture carrier",
	); err != nil {
		t.Fatal(err)
	}
	if err := validateP14CodexMCPCaptureCarrier(
		contract,
		prepared,
		packet,
		reloadedCapture,
	); err != nil {
		t.Fatal(err)
	}
	normalizedTampered := cloneP14CodexMCPCaptureCarrierForTest(
		t,
		captureCarrier,
	)
	normalizedIndex := -1
	for index, capture := range normalizedTampered.ScenarioCaptures {
		for _, scenario := range prepared.Preparation.Scenarios {
			if capture.ID == scenario.ID &&
				scenario.Oracle.Kind == "normalized_digest" {
				normalizedIndex = index
				break
			}
		}
		if normalizedIndex >= 0 {
			break
		}
	}
	if normalizedIndex < 0 {
		t.Fatal("P14 normalizer regression lacks a normalized scenario")
	}
	normalizedTampered.
		ScenarioCaptures[normalizedIndex].
		SurfaceObservation.
		NormalizedResultDigest = p14Digest(
		[]byte("forged-normalized-result"),
	)
	rehashP14CodexMCPCaptureCarrierForTest(t, &normalizedTampered)
	err = validateP14CodexMCPCaptureCarrier(
		contract,
		prepared,
		packet,
		normalizedTampered,
	)
	if err == nil ||
		!strings.Contains(
			err.Error(),
			"outcome or normalized result differs",
		) {
		t.Fatal(
			"P14 Codex MCP validator accepted a self-hashed forged normalized result",
		)
	}
	tamperedRaw, marshalErr := json.MarshalIndent(
		normalizedTampered,
		"",
		"  ",
	)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	tamperedRaw = append(tamperedRaw, '\n')
	if err := publishP14NoClobber(
		repositoryRoot,
		filepath.FromSlash(normalizedTampered.CarrierPath),
		tamperedRaw,
	); err != nil {
		t.Fatal(err)
	}
	_, _, err = loadP14CodexMCPCaptureForFinalization(
		repositoryRoot,
		contract,
		preparedPath,
		preparedDigest,
		prepared,
		normalizedTampered.CarrierPath,
	)
	if err == nil {
		t.Fatal(
			"P14 finalization loader accepted a self-hashed forged normalized result",
		)
	}
	checkTampered := cloneP14CodexMCPCaptureCarrierForTest(
		t,
		captureCarrier,
	)
	checkObservation :=
		&checkTampered.ScenarioCaptures[0].SurfaceObservation
	checkReceipt := p14CodexMCPScenarioReceipt{}
	if err := decodeP14StrictCompactJSON(
		checkObservation.ObservationCanonical,
		&checkReceipt,
		"check-policy tampered Codex MCP receipt",
	); err != nil {
		t.Fatal(err)
	}
	checkReceipt.Checks = []p14InstalledCLICheckReceipt{
		{ID: "arbitrary_pass", Satisfied: true},
	}
	checkReceiptRaw, err := marshalP14CanonicalJSON(checkReceipt)
	if err != nil {
		t.Fatal(err)
	}
	checkReceiptDigest := p14Digest(checkReceiptRaw)
	checkObservation.ObservationCanonical = string(checkReceiptRaw)
	checkObservation.ObservationDigest = checkReceiptDigest
	checkObservation.SourceReceiptDigest = checkReceiptDigest
	rehashP14CodexMCPCaptureCarrierForTest(t, &checkTampered)
	err = validateP14CodexMCPCaptureCarrier(
		contract,
		prepared,
		packet,
		checkTampered,
	)
	if err == nil ||
		!strings.Contains(err.Error(), "exact family normalizer policy") {
		t.Fatal(
			"P14 Codex MCP validator accepted an arbitrary satisfied check",
		)
	}
	tampered := captureInput
	tampered.Calls = slices.Clone(captureInput.Calls)
	tampered.Calls[0].Transcript.ToolCallID = "call-from-another-task"
	if err := validateP14CodexMCPCaptureInput(
		packet,
		tampered,
	); err == nil {
		t.Fatal("P14 Codex MCP capture accepted mismatched call identity")
	}
	promptTampered := captureInput
	promptTampered.Calls = slices.Clone(captureInput.Calls)
	promptIndex := promptCallIndexes[0]
	promptCall := promptTampered.Calls[promptIndex]
	promptCall.AgentPrompt = nil
	promptTampered.Calls[promptIndex] = promptCall
	if err := validateP14CodexMCPCaptureInput(
		packet,
		promptTampered,
	); err == nil {
		t.Fatal("P14 Codex MCP capture accepted missing prompt evidence")
	}
	ordinalTampered := captureInput
	ordinalTampered.Calls = slices.Clone(captureInput.Calls)
	ordinalCall := ordinalTampered.Calls[promptIndex]
	ordinalCall.Transcript.TurnToolCallOrdinal = 2
	ordinalCall.Transcript.TurnToolCallCount = 2
	ordinalTampered.Calls[promptIndex] = ordinalCall
	if err := validateP14CodexMCPCaptureInput(
		packet,
		ordinalTampered,
	); err == nil {
		t.Fatal("P14 Codex MCP capture accepted a non-first prompt tool call")
	}
	predecessorTampered := captureInput
	predecessorTampered.Calls = slices.Clone(captureInput.Calls)
	positiveLatest := time.Time{}
	for _, call := range captureInput.Calls {
		if call.ScenarioID != "positive_typed_write" {
			continue
		}
		responseAt, parseErr := time.Parse(
			time.RFC3339Nano,
			call.Response.CapturedAt,
		)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		if responseAt.After(positiveLatest) {
			positiveLatest = responseAt
		}
	}
	concurrencyCall := predecessorTampered.Calls[concurrencyTargetIndex]
	if concurrencyCall.ParallelGroup == "" || positiveLatest.IsZero() {
		t.Fatal("P14 chronology regression lacks parallel predecessor basis")
	}
	beforePredecessor := positiveLatest.Add(-time.Nanosecond)
	concurrencyCall.Response.CapturedAt =
		beforePredecessor.Format(time.RFC3339Nano)
	concurrencyCall.Transcript.HistoryReadAt =
		positiveLatest.Add(time.Second).Format(time.RFC3339Nano)
	predecessorTampered.Calls[concurrencyTargetIndex] = concurrencyCall
	err = validateP14CodexMCPCaptureInput(
		packet,
		predecessorTampered,
	)
	if err == nil || !strings.Contains(err.Error(), "required predecessor") {
		t.Fatal(
			"P14 Codex MCP capture accepted a parallel response before its required predecessor",
		)
	}
}

func cloneP14CodexMCPCaptureCarrierForTest(
	t *testing.T,
	carrier p14CodexMCPCaptureCarrier,
) p14CodexMCPCaptureCarrier {
	t.Helper()
	raw, err := json.Marshal(carrier)
	if err != nil {
		t.Fatal(err)
	}
	cloned := p14CodexMCPCaptureCarrier{}
	if err := json.Unmarshal(raw, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}

func rehashP14CodexMCPCaptureCarrierForTest(
	t *testing.T,
	carrier *p14CodexMCPCaptureCarrier,
) {
	t.Helper()
	basis, err := p14CodexMCPCaptureDigestBasis(*carrier)
	if err != nil {
		t.Fatal(err)
	}
	carrier.CaptureDigest = p14Digest(basis)
	body := strings.TrimPrefix(carrier.CaptureDigest, "sha256:")
	carrier.CarrierPath = filepath.ToSlash(filepath.Join(
		".context",
		"p14",
		"p14-codex-mcp-capture-"+body[:16]+".json",
	))
}

func TestP14CodexMCPNormalizerRegistryCoversEveryDeclaredLiveSurface(
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
	normalizers := p14CodexMCPFamilyNormalizers()
	for _, scenario := range contract.Scenarios {
		if !slices.Contains(scenario.Surfaces, "live_mcp") {
			continue
		}
		if normalizers[scenario.RequestBuilder] == nil {
			t.Fatalf(
				"P14 live MCP builder %q has no normalizer",
				scenario.RequestBuilder,
			)
		}
	}
}

func syntheticP14CodexMCPCaptureInput(
	packet p14CodexMCPRequestCarrier,
	packetPath string,
	packetCarrierDigest string,
	capturedAt time.Time,
) p14CodexMCPCaptureInput {
	calls := make([]p14CodexMCPCallEvidence, 0, len(packet.Packet.Calls))
	for _, planned := range packet.Packet.Calls {
		body := syntheticP14CodexMCPResponseBody(
			packet.Packet.Runtime,
			planned,
		)
		bodyDigest := p14Digest(body)
		toolCallID := fmt.Sprintf("call-p14-%03d", planned.Sequence)
		turnID := fmt.Sprintf("turn-p14-%03d", planned.Sequence)
		responseAt := capturedAt.Add(
			time.Duration(planned.Sequence) * 100 * time.Millisecond,
		)
		historyReadAt := responseAt.
			Add(time.Second).
			Format(time.RFC3339Nano)
		evidence := p14CodexMCPCallEvidence{
			Sequence:      planned.Sequence,
			ScenarioID:    planned.ScenarioID,
			CaseID:        planned.CaseID,
			ExchangeID:    planned.ExchangeID,
			ExchangeRole:  planned.ExchangeRole,
			ParallelGroup: planned.ParallelGroup,
			Transcript: p14CodexMCPTranscriptProjection{
				ThreadID:             packet.Packet.Runtime.ThreadID,
				TurnID:               turnID,
				ToolCallID:           toolCallID,
				Server:               "haft",
				Tool:                 planned.Tool,
				ArgsCanonical:        planned.ArgsCanonical,
				ArgsDigest:           planned.ArgsDigest,
				Status:               "completed",
				DurationMilliseconds: 1,
				TurnToolCallOrdinal:  1,
				TurnToolCallCount:    1,
				HistoryReadAt:        historyReadAt,
			},
			Response: p14CodexMCPResponseCapture{
				ToolCallID: toolCallID,
				CapturedAt: responseAt.Format(time.RFC3339Nano),
				IsError:    false,
				BodyBase64: base64.StdEncoding.EncodeToString(body),
				BodyDigest: bodyDigest,
			},
		}
		if planned.AgentPrompt != nil {
			evidence.AgentPrompt =
				&p14CodexMCPPromptTranscriptProjection{
					PromptID:      planned.AgentPrompt.ID,
					ThreadID:      packet.Packet.Runtime.ThreadID,
					TurnID:        turnID,
					Role:          "user",
					TextCanonical: planned.AgentPrompt.TextCanonical,
					TextDigest:    planned.AgentPrompt.TextDigest,
					HistoryReadAt: historyReadAt,
				}
		}
		calls = append(calls, evidence)
	}
	for index := 0; index < len(calls); {
		exchangeID := calls[index].ExchangeID
		end := index + 1
		for end < len(calls) &&
			calls[end].ExchangeID == exchangeID {
			end++
		}
		targetIndexes := make([]int, 0)
		for callIndex := index; callIndex < end; callIndex++ {
			if calls[callIndex].ExchangeRole ==
				p14CodexMCPExchangeTarget {
				targetIndexes = append(targetIndexes, callIndex)
			}
		}
		if len(targetIndexes) > 1 {
			turnID := fmt.Sprintf(
				"turn-p14-parallel-%03d",
				calls[targetIndexes[0]].Sequence,
			)
			for ordinal, callIndex := range targetIndexes {
				call := calls[callIndex]
				call.Transcript.TurnID = turnID
				call.Transcript.TurnToolCallOrdinal = ordinal + 1
				call.Transcript.TurnToolCallCount = len(targetIndexes)
				call.Transcript.DurationMilliseconds = 10
				calls[callIndex] = call
			}
		}
		index = end
	}
	return p14CodexMCPCaptureInput{
		Schema:          p14CodexMCPCaptureInputSchema,
		Status:          p14CodexMCPCaptureStatus,
		ResultSemantics: p14CodexMCPCaptureSemantics,
		ReleaseClaim:    false,
		RequestPacket: p14CodexMCPRequestBinding{
			CarrierPath:   packetPath,
			CarrierDigest: packetCarrierDigest,
			PacketDigest:  packet.PacketDigest,
		},
		CapturedAt: capturedAt.Format(time.RFC3339Nano),
		Calls:      calls,
	}
}

func syntheticP14CodexMCPResponseBody(
	runtime p14RuntimeObservationBinding,
	planned p14CodexMCPPlannedCall,
) []byte {
	if planned.ExchangeRole ==
		p14CodexMCPExchangeIdentityBefore ||
		planned.ExchangeRole ==
			p14CodexMCPExchangeIdentityAfter {
		startedAt, err := time.Parse(
			time.RFC3339Nano,
			runtime.LiveMCPStartedAt,
		)
		if err != nil {
			panic(err)
		}
		return []byte(fmt.Sprintf(
			"Project status: synthetic\n### Runtime\n\n- `haft serve`: pid=%d started=%s executable=`%s` executable_mtime=2026-07-28T00:00:00Z\n",
			runtime.LiveMCPPID,
			startedAt.UTC().Format(time.RFC3339),
			runtime.LiveMCPExecutablePath,
		))
	}
	if planned.ExchangeRole != p14CodexMCPExchangeBasisBefore &&
		planned.ExchangeRole != p14CodexMCPExchangeBasisAfter {
		return []byte(`{"synthetic":"response"}`)
	}
	body, err := marshalP14CanonicalJSON(map[string]any{
		"contract_version": "haft.memory.v1",
		"action":           "resolve",
		"result_kind":      "known_absent",
		"result": map[string]any{
			"snapshot_basis": map[string]any{
				"type_env_ref": "typeenv:" +
					p14TestDigest("synthetic-exchange-type-env"),
				"type_env_digest": p14TestDigest(
					"synthetic-exchange-type-env",
				),
				"graph_revision": 17,
			},
		},
	})
	if err != nil {
		panic(err)
	}
	return body
}

func p14CodexMCPScenarioFirstCallIndex(
	calls []p14CodexMCPPlannedCall,
	scenarioID string,
) int {
	for index, call := range calls {
		if call.ScenarioID == scenarioID {
			return index
		}
	}
	return -1
}

func p14CodexMCPScenarioFirstTargetCallIndex(
	calls []p14CodexMCPPlannedCall,
	scenarioID string,
) int {
	for index, call := range calls {
		if call.ScenarioID == scenarioID &&
			call.ExchangeRole == p14CodexMCPExchangeTarget {
			return index
		}
	}
	return -1
}
