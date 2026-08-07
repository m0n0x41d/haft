package p14acceptance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"
)

const (
	p14SpecSectionProtocolBuilderID       = "spec.lifecycle-exact-section-rejection.v1"
	p14SpecSectionProtocolSectionID       = "SS.interfaces.code-graph.001"
	p14SpecSectionProtocolUseContext      = "v9 source acceptance and public contract claim"
	p14SpecSectionProtocolSemanticSchema  = "haft.p14.spec-section-protocol-semantic/v1"
	p14SpecSectionProtocolSurfaceSchema   = "haft.p14.spec-section-protocol-mcp/v1"
	p14SpecSectionProtocolOutputSchema    = "haft.p14.spec-section-protocol-normalized-output/v1"
	p14SpecSectionProtocolOracleSchema    = "haft.p14.spec-section-protocol-local-oracle/v1"
	p14SpecSectionProtocolNormalizationID = "spec-section-protocol-error.v1"
	p14SpecSectionProtocolLocalOracleTest = "github.com/m0n0x41d/haft/internal/cli::TestHandleHaftSpecSection_ProjectLifecycleRejectsExactSectionID"
)

type p14SpecSectionProtocolSemanticRequest struct {
	Schema    string                               `json:"schema"`
	SectionID string                               `json:"section_id"`
	Cases     []p14SpecSectionProtocolSemanticCase `json:"cases"`
}

type p14SpecSectionProtocolSemanticCase struct {
	ID       string                                 `json:"id"`
	Action   string                                 `json:"action"`
	Expected p14SpecSectionProtocolSemanticExpected `json:"expected"`
}

type p14SpecSectionProtocolSemanticExpected struct {
	Outcome       string `json:"outcome"`
	Code          string `json:"code"`
	ProjectObject string `json:"project_object"`
	TraceAction   string `json:"trace_action"`
	UseAction     string `json:"use_action"`
}

type p14SpecSectionProtocolMCPSurface struct {
	Schema                string                          `json:"schema"`
	SemanticRequestDigest string                          `json:"semantic_request_digest"`
	Tool                  string                          `json:"tool"`
	Cases                 []p14SpecSectionProtocolMCPCase `json:"cases"`
}

type p14SpecSectionProtocolMCPCase struct {
	ID   string         `json:"id"`
	Args map[string]any `json:"args"`
}

type p14SpecSectionProtocolNormalizedOutput struct {
	Schema string                                 `json:"schema"`
	Cases  []p14SpecSectionProtocolNormalizedCase `json:"cases"`
}

type p14SpecSectionProtocolNormalizedCase struct {
	ID      string                                `json:"id"`
	Outcome string                                `json:"outcome"`
	Error   p14SpecSectionProtocolNormalizedError `json:"error"`
}

type p14SpecSectionProtocolNormalizedError struct {
	Code          string                                     `json:"code"`
	Tool          string                                     `json:"tool"`
	Action        string                                     `json:"action"`
	SectionID     string                                     `json:"section_id"`
	ProjectObject string                                     `json:"project_object"`
	RecoveryCalls []p14SpecSectionProtocolNormalizedRecovery `json:"recovery_calls"`
}

type p14SpecSectionProtocolNormalizedRecovery struct {
	Tool       string `json:"tool"`
	Action     string `json:"action"`
	SectionID  string `json:"section_id"`
	UseContext string `json:"use_context,omitempty"`
}

type p14SpecSectionProtocolLocalOracle struct {
	Schema                string `json:"schema"`
	SemanticRequestDigest string `json:"semantic_request_digest"`
	ExpectedResultDigest  string `json:"expected_result_digest"`
	LocalOracleTest       string `json:"local_oracle_test"`
}

func canonicalP14SpecSectionProtocolSemanticRequest(
	sectionID string,
) p14SpecSectionProtocolSemanticRequest {
	expected := p14SpecSectionProtocolSemanticExpected{
		Outcome:       "error",
		Code:          "section_id_not_applicable",
		ProjectObject: "ProjectSpecificationSet",
		TraceAction:   "spec_trace",
		UseAction:     "spec_use",
	}
	return p14SpecSectionProtocolSemanticRequest{
		Schema:    p14SpecSectionProtocolSemanticSchema,
		SectionID: sectionID,
		Cases: []p14SpecSectionProtocolSemanticCase{
			{
				ID:       "lifecycle_rejects_exact_section",
				Action:   "lifecycle",
				Expected: expected,
			},
			{
				ID:       "next_step_rejects_exact_section",
				Action:   "next_step",
				Expected: expected,
			},
		},
	}
}

func buildP14SpecSectionProtocolMCPSurface(
	semantic p14SpecSectionProtocolSemanticRequest,
	semanticDigest string,
) ([]byte, error) {
	cases := make([]p14SpecSectionProtocolMCPCase, 0, len(semantic.Cases))
	for _, testCase := range semantic.Cases {
		args := map[string]any{
			"action":     testCase.Action,
			"section_id": semantic.SectionID,
		}
		cases = append(cases, p14SpecSectionProtocolMCPCase{
			ID:   testCase.ID,
			Args: args,
		})
	}
	payload := p14SpecSectionProtocolMCPSurface{
		Schema:                p14SpecSectionProtocolSurfaceSchema,
		SemanticRequestDigest: semanticDigest,
		Tool:                  "haft_spec_section",
		Cases:                 cases,
	}
	return marshalP14CanonicalJSON(payload)
}

func canonicalP14SpecSectionProtocolNormalizedOutput(
	semantic p14SpecSectionProtocolSemanticRequest,
) p14SpecSectionProtocolNormalizedOutput {
	cases := make(
		[]p14SpecSectionProtocolNormalizedCase,
		0,
		len(semantic.Cases),
	)
	for _, testCase := range semantic.Cases {
		recoveries := []p14SpecSectionProtocolNormalizedRecovery{
			{
				Tool:      "haft_query",
				Action:    testCase.Expected.TraceAction,
				SectionID: semantic.SectionID,
			},
			{
				Tool:       "haft_query",
				Action:     testCase.Expected.UseAction,
				SectionID:  semantic.SectionID,
				UseContext: p14SpecSectionProtocolUseContext,
			},
		}
		normalized := p14SpecSectionProtocolNormalizedCase{
			ID:      testCase.ID,
			Outcome: testCase.Expected.Outcome,
			Error: p14SpecSectionProtocolNormalizedError{
				Code:          testCase.Expected.Code,
				Tool:          "haft_spec_section",
				Action:        testCase.Action,
				SectionID:     semantic.SectionID,
				ProjectObject: testCase.Expected.ProjectObject,
				RecoveryCalls: recoveries,
			},
		}
		cases = append(cases, normalized)
	}
	return p14SpecSectionProtocolNormalizedOutput{
		Schema: p14SpecSectionProtocolOutputSchema,
		Cases:  cases,
	}
}

func buildP14SpecSectionProtocolScenario(
	declared scenarioContract,
) (preparedP14Scenario, error) {
	semantic := canonicalP14SpecSectionProtocolSemanticRequest(
		p14SpecSectionProtocolSectionID,
	)
	semanticBytes, err := marshalP14CanonicalJSON(semantic)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	semanticDigest := p14Digest(semanticBytes)
	surfaceBytes, err := buildP14SpecSectionProtocolMCPSurface(
		semantic,
		semanticDigest,
	)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	expected := canonicalP14SpecSectionProtocolNormalizedOutput(semantic)
	expectedBytes, err := marshalP14CanonicalJSON(expected)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	expectedDigest := p14Digest(expectedBytes)
	localOracle := p14SpecSectionProtocolLocalOracle{
		Schema:                p14SpecSectionProtocolOracleSchema,
		SemanticRequestDigest: semanticDigest,
		ExpectedResultDigest:  expectedDigest,
		LocalOracleTest:       p14SpecSectionProtocolLocalOracleTest,
	}
	localOracleBytes, err := marshalP14CanonicalJSON(localOracle)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	request := preparedP14Request{
		Surface:               "live_mcp",
		Builder:               p14SpecSectionProtocolBuilderID,
		Encoding:              "canonical_json",
		CanonicalPayload:      string(surfaceBytes),
		PayloadDigest:         p14Digest(surfaceBytes),
		SemanticRequestDigest: semanticDigest,
	}
	return preparedP14Scenario{
		ID:                       declared.ID,
		SemanticRequestCanonical: string(semanticBytes),
		SemanticRequestDigest:    semanticDigest,
		Requests:                 []preparedP14Request{request},
		Oracle: preparedP14Oracle{
			Kind:                    declared.OracleKind,
			NormalizationID:         p14SpecSectionProtocolNormalizationID,
			ExpectedResultDigest:    expectedDigest,
			ExpectedEffect:          declared.ExpectedEffect,
			LocalOracleOutputDigest: p14Digest(localOracleBytes),
		},
	}, nil
}

func validateP14SpecSectionProtocolPreparedScenario(
	declared scenarioContract,
	scenario preparedP14Scenario,
) error {
	if declared.RequestBuilder != p14SpecSectionProtocolBuilderID {
		return fmt.Errorf(
			"P14 SpecSection protocol validator received builder %q",
			declared.RequestBuilder,
		)
	}
	semantic, err := decodeP14SpecSectionProtocolSemanticRequest(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return err
	}
	expectedSemantic := canonicalP14SpecSectionProtocolSemanticRequest(
		semantic.SectionID,
	)
	actualSemanticBytes, err := marshalP14CanonicalJSON(semantic)
	if err != nil {
		return err
	}
	expectedSemanticBytes, err := marshalP14CanonicalJSON(expectedSemantic)
	if err != nil {
		return err
	}
	if semantic.SectionID != p14SpecSectionProtocolSectionID ||
		!bytes.Equal(actualSemanticBytes, expectedSemanticBytes) {
		return fmt.Errorf(
			"P14 SpecSection semantic carrier is not the closed rejection matrix",
		)
	}
	expectedSurface, err := buildP14SpecSectionProtocolMCPSurface(
		expectedSemantic,
		scenario.SemanticRequestDigest,
	)
	if err != nil {
		return err
	}
	if len(scenario.Requests) != 1 ||
		scenario.Requests[0].CanonicalPayload != string(expectedSurface) ||
		scenario.Requests[0].Encoding != "canonical_json" {
		return fmt.Errorf(
			"P14 SpecSection MCP request is not derived from the semantic carrier",
		)
	}
	expectedOutput := canonicalP14SpecSectionProtocolNormalizedOutput(
		expectedSemantic,
	)
	expectedOutputBytes, err := marshalP14CanonicalJSON(expectedOutput)
	if err != nil {
		return err
	}
	expectedDigest := p14Digest(expectedOutputBytes)
	localOracle := p14SpecSectionProtocolLocalOracle{
		Schema:                p14SpecSectionProtocolOracleSchema,
		SemanticRequestDigest: scenario.SemanticRequestDigest,
		ExpectedResultDigest:  expectedDigest,
		LocalOracleTest:       p14SpecSectionProtocolLocalOracleTest,
	}
	localOracleBytes, err := marshalP14CanonicalJSON(localOracle)
	if err != nil {
		return err
	}
	if scenario.Oracle.NormalizationID !=
		p14SpecSectionProtocolNormalizationID ||
		scenario.Oracle.ExpectedResultDigest != expectedDigest ||
		scenario.Oracle.LocalOracleOutputDigest != p14Digest(localOracleBytes) {
		return fmt.Errorf(
			"P14 SpecSection oracle is not the closed normalized error",
		)
	}
	return nil
}

func decodeP14SpecSectionProtocolSemanticRequest(
	raw []byte,
) (p14SpecSectionProtocolSemanticRequest, error) {
	reader := bytes.NewReader(raw)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var semantic p14SpecSectionProtocolSemanticRequest
	if err := decoder.Decode(&semantic); err != nil {
		return p14SpecSectionProtocolSemanticRequest{}, fmt.Errorf(
			"decode P14 SpecSection semantic carrier: %w",
			err,
		)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		return p14SpecSectionProtocolSemanticRequest{}, fmt.Errorf(
			"P14 SpecSection semantic carrier has trailing JSON",
		)
	}
	return semantic, nil
}

func TestP14SpecSectionProtocolBuilderClosesExactReadRecovery(
	t *testing.T,
) {
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, _, err := loadRequestOracleContract(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	declared, err := findP14ScenarioContract(
		contract,
		"spec_section_read_protocol",
	)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := buildP14SpecSectionProtocolScenario(declared)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateP14SpecSectionProtocolPreparedScenario(
		declared,
		scenario,
	); err != nil {
		t.Fatal(err)
	}
	tampered := scenario
	tamperedBytes := bytes.ReplaceAll(
		[]byte(scenario.SemanticRequestCanonical),
		[]byte(`"spec_use"`),
		[]byte(`"spec_review"`),
	)
	tampered.SemanticRequestCanonical = string(tamperedBytes)
	if err := validateP14SpecSectionProtocolPreparedScenario(
		declared,
		tampered,
	); err == nil {
		t.Fatal("P14 SpecSection validator accepted changed recovery action")
	}
}
