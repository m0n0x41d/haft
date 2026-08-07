package p14acceptance

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type p14CodexMCPIdentifierErrorWire struct {
	Code              string                              `json:"code"`
	Tool              string                              `json:"tool"`
	Action            string                              `json:"action"`
	Parameter         string                              `json:"parameter"`
	Identifier        string                              `json:"identifier"`
	ReceivedNamespace string                              `json:"received_namespace"`
	ExpectedNamespace string                              `json:"expected_namespace"`
	SameCallRetryable bool                                `json:"same_call_retryable"`
	Message           string                              `json:"message"`
	RecoveryCall      p14IdentifierNormalizedRecoveryCall `json:"recovery_call"`
}

func p14CodexMCPFamilyNormalizers() map[string]p14CodexMCPFamilyNormalizer {
	normalizers := map[string]p14CodexMCPFamilyNormalizer{
		p14RuntimeIdentityBuilderID:     normalizeP14CodexMCPRuntimeIdentity,
		p14FPFProjectionBuilderID:       normalizeP14CodexMCPFPFProjection,
		p14IdentifierNamespaceBuilderID: normalizeP14CodexMCPIdentifier,
		p14SpecSectionProtocolBuilderID: normalizeP14CodexMCPSpecSection,
	}
	for _, builderID := range p14CodeExploreBuilderIDs {
		normalizers[builderID] = normalizeP14CodexMCPCodeExplore
	}
	for _, builderID := range p14AgentOrientationBuilderIDs {
		normalizers[builderID] = normalizeP14CodexMCPAgentOrientation
	}
	for _, builderID := range p14MemoryReadBuilderIDs {
		normalizers[builderID] = normalizeP14CodexMCPMemoryRead
	}
	for _, builderID := range p14MemoryOperationBuilderIDs {
		normalizers[builderID] = normalizeP14CodexMCPMemoryOperation
	}
	return normalizers
}

func normalizeP14CodexMCPRuntimeIdentity(
	prepared preparedRequestOracleCarrier,
	scenario preparedP14Scenario,
	request preparedP14Request,
	evidence []p14CodexMCPCallEvidence,
) (p14CodexMCPFamilyResult, error) {
	if len(evidence) != 1 ||
		evidence[0].CaseID != "status_probe" {
		return p14CodexMCPFamilyResult{}, fmt.Errorf(
			"P14 Codex MCP runtime probe count differs",
		)
	}
	body, err := p14CodexMCPResponseBody(evidence[0])
	if err != nil {
		return p14CodexMCPFamilyResult{}, err
	}
	runtime := prepared.Preparation.FrozenBasis
	satisfied := !evidence[0].Response.IsError &&
		len(bytes.TrimSpace(body)) > 0 &&
		request.Builder == p14RuntimeIdentityBuilderID &&
		scenario.ID == "runtime_identity" &&
		runtime.Candidate.ExecutableDigest != "" &&
		runtime.SelectedProject.ProjectRoot != ""
	checks := []p14InstalledCLICheckReceipt{
		{ID: "actual_task_tool_call_projection_bound", Satisfied: satisfied},
		{ID: "verified_live_mcp_process_receipt_bound", Satisfied: satisfied},
		{ID: "status_response_captured_from_bound_transport", Satisfied: satisfied},
	}
	result := p14CodexMCPFamilyResult{Checks: checks}
	if !satisfied {
		result.FailureCode = "runtime_identity_mismatch"
		result.FailureDetail =
			"actual task status probe or verified runtime binding differs"
	}
	return result, nil
}

func normalizeP14CodexMCPFPFProjection(
	prepared preparedRequestOracleCarrier,
	scenario preparedP14Scenario,
	_ preparedP14Request,
	evidence []p14CodexMCPCallEvidence,
) (p14CodexMCPFamilyResult, error) {
	semantic, err := decodeP14FPFProjectionSemanticRequest(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return p14CodexMCPFamilyResult{}, err
	}
	observations, err := p14CodexMCPCommandObservations(evidence)
	if err != nil {
		return p14CodexMCPFamilyResult{}, err
	}
	normalized, _, normalizeErr := normalizeP14FPFProjectionObservations(
		semantic.Cases,
		observations,
		prepared.Preparation.FrozenBasis.Candidate.FPFRevision,
	)
	return p14CodexMCPNormalizedResult(
		normalized,
		normalizeErr,
		"closed_fpf_projection_normalizer",
	)
}

func normalizeP14CodexMCPMemoryRead(
	_ preparedRequestOracleCarrier,
	scenario preparedP14Scenario,
	_ preparedP14Request,
	evidence []p14CodexMCPCallEvidence,
) (p14CodexMCPFamilyResult, error) {
	semantic, err := decodeP14MemoryReadSemanticRequest(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return p14CodexMCPFamilyResult{}, err
	}
	observations, err := p14CodexMCPCommandObservations(evidence)
	if err != nil {
		return p14CodexMCPFamilyResult{}, err
	}
	normalized, _, normalizeErr := normalizeP14MemoryReadObservations(
		semantic,
		observations,
	)
	return p14CodexMCPNormalizedResult(
		normalized,
		normalizeErr,
		"closed_memory_read_normalizer",
	)
}

func normalizeP14CodexMCPMemoryOperation(
	_ preparedRequestOracleCarrier,
	scenario preparedP14Scenario,
	_ preparedP14Request,
	evidence []p14CodexMCPCallEvidence,
) (p14CodexMCPFamilyResult, error) {
	semantic, err := decodeP14MemoryOperationSemantic(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return p14CodexMCPFamilyResult{}, err
	}
	results, err := p14CodexMCPProcessResults(evidence)
	if err != nil {
		return p14CodexMCPFamilyResult{}, err
	}
	normalizers := map[string]func(
		p14MemoryOperationSemanticRequest,
		[]p14InstalledCLIProcessResult,
	) error{
		"positive_typed_write":    normalizeP14InstalledCLIPositiveWrite,
		"invalid":                 normalizeP14InstalledCLIValidationOnly,
		"underdetermined":         normalizeP14InstalledCLIValidationOnly,
		"authority_rejection":     normalizeP14InstalledCLIAuthorityRejection,
		"concurrency_idempotency": normalizeP14InstalledCLIConcurrency,
	}
	normalizer := normalizers[semantic.ScenarioID]
	if normalizer == nil {
		return p14CodexMCPFamilyResult{}, fmt.Errorf(
			"P14 Codex MCP memory operation %q is open",
			semantic.ScenarioID,
		)
	}
	normalizeErr := normalizer(semantic, results)
	normalized := p14MemoryOperationNormalizedOutput{
		Schema:     p14MemoryOperationOutputSchema,
		ScenarioID: semantic.ScenarioID,
		Expected:   semantic.Expected,
	}
	return p14CodexMCPNormalizedResult(
		normalized,
		normalizeErr,
		"closed_memory_operation_normalizer",
	)
}

func normalizeP14CodexMCPIdentifier(
	_ preparedRequestOracleCarrier,
	scenario preparedP14Scenario,
	_ preparedP14Request,
	evidence []p14CodexMCPCallEvidence,
) (p14CodexMCPFamilyResult, error) {
	semantic, err := decodeP14IdentifierSemanticRequest(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return p14CodexMCPFamilyResult{}, err
	}
	if len(evidence) != 1 || !evidence[0].Response.IsError {
		return p14CodexMCPNormalizedFailure(
			"identifier_namespace_mismatch",
			"wrong-namespace request was not rejected",
			"closed_identifier_namespace_normalizer",
		), nil
	}
	body, err := p14CodexMCPResponseBody(evidence[0])
	if err != nil {
		return p14CodexMCPFamilyResult{}, err
	}
	wire := p14CodexMCPIdentifierErrorWire{}
	if err := decodeP14StrictCompactJSON(
		string(bytes.TrimSpace(body)),
		&wire,
		"actual Codex MCP identifier error",
	); err != nil {
		return p14CodexMCPNormalizedFailure(
			"identifier_namespace_mismatch",
			err.Error(),
			"closed_identifier_namespace_normalizer",
		), nil
	}
	expected := canonicalP14IdentifierNormalizedOutput(semantic.ArtifactRef)
	expectedError := expected.Cases[0].Error
	if wire.Code != expectedError.Code ||
		wire.Tool != expectedError.Tool ||
		wire.Action != expectedError.Action ||
		wire.Parameter != expectedError.Parameter ||
		wire.Identifier != expectedError.Identifier ||
		wire.ReceivedNamespace != expectedError.ReceivedNamespace ||
		wire.ExpectedNamespace != expectedError.ExpectedNamespace ||
		wire.SameCallRetryable != expectedError.SameCallRetryable ||
		wire.RecoveryCall != expectedError.RecoveryCall ||
		strings.TrimSpace(wire.Message) == "" {
		return p14CodexMCPNormalizedFailure(
			"identifier_namespace_mismatch",
			"wrong-namespace error fields differ",
			"closed_identifier_namespace_normalizer",
		), nil
	}
	return p14CodexMCPNormalizedResult(
		expected,
		nil,
		"closed_identifier_namespace_normalizer",
	)
}

func normalizeP14CodexMCPSpecSection(
	_ preparedRequestOracleCarrier,
	scenario preparedP14Scenario,
	_ preparedP14Request,
	evidence []p14CodexMCPCallEvidence,
) (p14CodexMCPFamilyResult, error) {
	semantic, err := decodeP14SpecSectionProtocolSemanticRequest(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return p14CodexMCPFamilyResult{}, err
	}
	if len(evidence) != len(semantic.Cases) {
		return p14CodexMCPNormalizedFailure(
			"spec_section_protocol_mismatch",
			"SpecSection response count differs",
			"closed_spec_section_protocol_normalizer",
		), nil
	}
	for index, testCase := range semantic.Cases {
		body, bodyErr := p14CodexMCPResponseBody(evidence[index])
		if bodyErr != nil {
			return p14CodexMCPFamilyResult{}, bodyErr
		}
		text := string(body)
		markers := []string{
			testCase.Expected.Code,
			"project/scope-level",
			semantic.SectionID,
			`haft_query(action="` + testCase.Expected.TraceAction + `"`,
			`haft_query(action="` + testCase.Expected.UseAction + `"`,
		}
		if !evidence[index].Response.IsError ||
			!p14TextContainsEvery(text, markers) {
			return p14CodexMCPNormalizedFailure(
				"spec_section_protocol_mismatch",
				"SpecSection rejection or recovery calls differ",
				"closed_spec_section_protocol_normalizer",
			), nil
		}
	}
	expected := canonicalP14SpecSectionProtocolNormalizedOutput(semantic)
	return p14CodexMCPNormalizedResult(
		expected,
		nil,
		"closed_spec_section_protocol_normalizer",
	)
}

func p14CodexMCPNormalizedResult(
	value any,
	normalizeErr error,
	checkIDs ...string,
) (p14CodexMCPFamilyResult, error) {
	if normalizeErr != nil {
		return p14CodexMCPNormalizedFailure(
			"normalization_failed",
			boundedP14InstalledCLIError(normalizeErr),
			checkIDs...,
		), nil
	}
	normalized, err := marshalP14CanonicalJSON(value)
	if err != nil {
		return p14CodexMCPFamilyResult{}, err
	}
	checks := []p14InstalledCLICheckReceipt{
		{ID: "actual_task_tool_call_projection_bound", Satisfied: true},
		{ID: "captured_response_digest_bound", Satisfied: true},
	}
	for _, checkID := range checkIDs {
		checks = append(checks, p14InstalledCLICheckReceipt{
			ID:        checkID,
			Satisfied: true,
		})
	}
	return p14CodexMCPFamilyResult{
		Normalized: normalized,
		Checks:     checks,
	}, nil
}

func p14CodexMCPNormalizedFailure(
	code string,
	detail string,
	checkIDs ...string,
) p14CodexMCPFamilyResult {
	checks := []p14InstalledCLICheckReceipt{
		{ID: "actual_task_tool_call_projection_bound", Satisfied: true},
		{ID: "captured_response_digest_bound", Satisfied: true},
	}
	for _, checkID := range checkIDs {
		checks = append(checks, p14InstalledCLICheckReceipt{
			ID:        checkID,
			Satisfied: false,
		})
	}
	return p14CodexMCPFamilyResult{
		FailureCode:   code,
		FailureDetail: detail,
		Checks:        checks,
	}
}

func p14CodexMCPCommandObservations(
	evidence []p14CodexMCPCallEvidence,
) (map[string]p14FPFProjectionCommandObservation, error) {
	result := make(
		map[string]p14FPFProjectionCommandObservation,
		len(evidence),
	)
	for _, call := range evidence {
		observation, err := p14CodexMCPCommandObservation(call)
		if err != nil {
			return nil, err
		}
		if _, duplicate := result[call.CaseID]; duplicate {
			return nil, fmt.Errorf(
				"P14 Codex MCP repeats response case %q",
				call.CaseID,
			)
		}
		result[call.CaseID] = observation
	}
	return result, nil
}

func p14CodexMCPProcessResults(
	evidence []p14CodexMCPCallEvidence,
) ([]p14InstalledCLIProcessResult, error) {
	result := make([]p14InstalledCLIProcessResult, 0, len(evidence))
	for _, call := range evidence {
		observation, err := p14CodexMCPCommandObservation(call)
		if err != nil {
			return nil, err
		}
		result = append(result, p14InstalledCLIProcessResult{
			ID:            call.CaseID,
			ParallelGroup: p14CodexMCPParallelGroup(call),
			ExitCode:      observation.ExitCode,
			Stdout:        slices.Clone(observation.Stdout),
			Stderr:        slices.Clone(observation.Stderr),
		})
	}
	return result, nil
}

func p14CodexMCPCommandObservation(
	evidence p14CodexMCPCallEvidence,
) (p14FPFProjectionCommandObservation, error) {
	body, err := p14CodexMCPResponseBody(evidence)
	if err != nil {
		return p14FPFProjectionCommandObservation{}, err
	}
	if evidence.Response.IsError {
		return p14FPFProjectionCommandObservation{
			Stderr:   appendTerminalNewlineP14(body),
			ExitCode: 1,
		}, nil
	}
	return p14FPFProjectionCommandObservation{
		Stdout:   appendTerminalNewlineP14(body),
		ExitCode: 0,
	}, nil
}

func p14CodexMCPResponseBody(
	evidence p14CodexMCPCallEvidence,
) ([]byte, error) {
	body, err := base64.StdEncoding.DecodeString(
		evidence.Response.BodyBase64,
	)
	if err != nil {
		return nil, err
	}
	if p14Digest(body) != evidence.Response.BodyDigest {
		return nil, fmt.Errorf("P14 Codex MCP response digest differs")
	}
	return body, nil
}

func appendTerminalNewlineP14(input []byte) []byte {
	trimmed := bytes.TrimRight(input, "\n")
	return append(slices.Clone(trimmed), '\n')
}

func p14CodexMCPParallelGroup(
	evidence p14CodexMCPCallEvidence,
) string {
	return evidence.ParallelGroup
}

func p14TextContainsEvery(text string, markers []string) bool {
	for _, marker := range markers {
		if !strings.Contains(text, marker) {
			return false
		}
	}
	return true
}

func TestP14CodexMCPOwnNormalizersAcceptExactResponses(t *testing.T) {
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
	carrierRoot := t.TempDir()
	preparedPath, _, err := persistPreparedRequestOracleCarrier(
		carrierRoot,
		contract,
		input,
	)
	if err != nil {
		t.Fatal(err)
	}
	preparedRaw, err := os.ReadFile(filepath.Join(
		carrierRoot,
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

	t.Run("runtime identity", func(t *testing.T) {
		scenario, err := preparedP14ScenarioByID(
			prepared.Preparation.Scenarios,
			"runtime_identity",
		)
		if err != nil {
			t.Fatal(err)
		}
		request, present := p14PreparedSurfaceRequest(scenario, "live_mcp")
		if !present {
			t.Fatal("runtime identity live-MCP request is absent")
		}
		evidence := []p14CodexMCPCallEvidence{
			p14CodexMCPNormalizerEvidence(
				"status_probe",
				false,
				[]byte(`{"status":"ok"}`),
			),
		}
		result, err := normalizeP14CodexMCPRuntimeIdentity(
			prepared,
			scenario,
			request,
			evidence,
		)
		if err != nil {
			t.Fatal(err)
		}
		assertP14CodexMCPFamilySuccess(t, result, "")
	})

	t.Run("identifier namespace", func(t *testing.T) {
		scenario, err := preparedP14ScenarioByID(
			prepared.Preparation.Scenarios,
			"identifier_namespace",
		)
		if err != nil {
			t.Fatal(err)
		}
		request, present := p14PreparedSurfaceRequest(scenario, "live_mcp")
		if !present {
			t.Fatal("identifier namespace live-MCP request is absent")
		}
		semantic, err := decodeP14IdentifierSemanticRequest(
			[]byte(scenario.SemanticRequestCanonical),
		)
		if err != nil {
			t.Fatal(err)
		}
		expected := canonicalP14IdentifierNormalizedOutput(
			semantic.ArtifactRef,
		)
		expectedError := expected.Cases[0].Error
		wire := p14CodexMCPIdentifierErrorWire{
			Code:              expectedError.Code,
			Tool:              expectedError.Tool,
			Action:            expectedError.Action,
			Parameter:         expectedError.Parameter,
			Identifier:        expectedError.Identifier,
			ReceivedNamespace: expectedError.ReceivedNamespace,
			ExpectedNamespace: expectedError.ExpectedNamespace,
			SameCallRetryable: expectedError.SameCallRetryable,
			Message:           "identifier belongs to Haft project memory",
			RecoveryCall:      expectedError.RecoveryCall,
		}
		body, err := marshalP14CanonicalJSON(wire)
		if err != nil {
			t.Fatal(err)
		}
		evidence := []p14CodexMCPCallEvidence{
			p14CodexMCPNormalizerEvidence(
				p14IdentifierCaseID,
				true,
				body,
			),
		}
		result, err := normalizeP14CodexMCPIdentifier(
			prepared,
			scenario,
			request,
			evidence,
		)
		if err != nil {
			t.Fatal(err)
		}
		assertP14CodexMCPFamilySuccess(
			t,
			result,
			scenario.Oracle.ExpectedResultDigest,
		)
	})

	t.Run("SpecSection protocol", func(t *testing.T) {
		scenario, err := preparedP14ScenarioByID(
			prepared.Preparation.Scenarios,
			"spec_section_read_protocol",
		)
		if err != nil {
			t.Fatal(err)
		}
		request, present := p14PreparedSurfaceRequest(scenario, "live_mcp")
		if !present {
			t.Fatal("SpecSection live-MCP request is absent")
		}
		semantic, err := decodeP14SpecSectionProtocolSemanticRequest(
			[]byte(scenario.SemanticRequestCanonical),
		)
		if err != nil {
			t.Fatal(err)
		}
		evidence := make(
			[]p14CodexMCPCallEvidence,
			0,
			len(semantic.Cases),
		)
		for _, testCase := range semantic.Cases {
			body := []byte(fmt.Sprintf(
				"%s project/scope-level %s "+
					`haft_query(action="%s" `+
					`haft_query(action="%s"`,
				testCase.Expected.Code,
				semantic.SectionID,
				testCase.Expected.TraceAction,
				testCase.Expected.UseAction,
			))
			evidence = append(
				evidence,
				p14CodexMCPNormalizerEvidence(
					testCase.ID,
					true,
					body,
				),
			)
		}
		result, err := normalizeP14CodexMCPSpecSection(
			prepared,
			scenario,
			request,
			evidence,
		)
		if err != nil {
			t.Fatal(err)
		}
		assertP14CodexMCPFamilySuccess(
			t,
			result,
			scenario.Oracle.ExpectedResultDigest,
		)
	})

	t.Run("concurrency group", func(t *testing.T) {
		evidence := p14CodexMCPNormalizerEvidence(
			"concurrent_admission_a",
			false,
			[]byte(`{"result":"committed"}`),
		)
		evidence.ParallelGroup = "parallel_same_request"
		results, err := p14CodexMCPProcessResults(
			[]p14CodexMCPCallEvidence{evidence},
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 ||
			results[0].ParallelGroup != evidence.ParallelGroup {
			t.Fatal("P14 Codex MCP concurrency group was not preserved")
		}
	})
}

func p14CodexMCPNormalizerEvidence(
	caseID string,
	isError bool,
	body []byte,
) p14CodexMCPCallEvidence {
	return p14CodexMCPCallEvidence{
		CaseID: caseID,
		Response: p14CodexMCPResponseCapture{
			IsError:    isError,
			BodyBase64: base64.StdEncoding.EncodeToString(body),
			BodyDigest: p14Digest(body),
		},
	}
}

func assertP14CodexMCPFamilySuccess(
	t *testing.T,
	result p14CodexMCPFamilyResult,
	expectedDigest string,
) {
	t.Helper()
	if result.FailureCode != "" || result.FailureDetail != "" {
		t.Fatalf(
			"P14 Codex MCP normalizer failed: %s %s",
			result.FailureCode,
			result.FailureDetail,
		)
	}
	if len(result.Checks) == 0 {
		t.Fatal("P14 Codex MCP normalizer omitted checks")
	}
	for _, check := range result.Checks {
		if !check.Satisfied {
			t.Fatalf("P14 Codex MCP check %q failed", check.ID)
		}
	}
	if expectedDigest != "" && p14Digest(result.Normalized) != expectedDigest {
		t.Fatal("P14 Codex MCP normalized digest differs")
	}
}
