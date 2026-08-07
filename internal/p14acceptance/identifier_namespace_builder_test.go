package p14acceptance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

const (
	p14IdentifierNamespaceBuilderID     = "query.node-artifact-rejection.v1"
	p14IdentifierSemanticSchema         = "haft.p14.identifier-namespace-semantic/v1"
	p14IdentifierMCPSurfaceSchema       = "haft.p14.identifier-namespace-mcp/v1"
	p14IdentifierNormalizedOutputSchema = "haft.p14.identifier-namespace-output/v1"
	p14IdentifierLocalOracleSchema      = "haft.p14.identifier-namespace-local-oracle/v1"
	p14IdentifierFixtureSchema          = "haft.p14.identifier-fixture/v1"
	p14IdentifierNormalizationID        = "p14.identifier-namespace.semantic-error.v1"
	p14IdentifierCaseID                 = "node_rejects_artifact_id"
	p14IdentifierLocalOracleTest        = "github.com/m0n0x41d/haft/internal/cli::TestNodeWrongNamespaceSurvivesV5MCPBoundary"
)

type p14IdentifierFixture struct {
	Schema                string `json:"schema"`
	ArtifactRef           string `json:"artifact_ref"`
	ArtifactCarrierPath   string `json:"artifact_carrier_path"`
	ArtifactCarrierDigest string `json:"artifact_carrier_digest"`
}

type p14IdentifierSemanticRequest struct {
	Schema      string                      `json:"schema"`
	ArtifactRef string                      `json:"artifact_ref"`
	Cases       []p14IdentifierSemanticCase `json:"cases"`
}

type p14IdentifierSemanticCase struct {
	ID       string                              `json:"id"`
	Request  p14IdentifierSemanticMCPRequest     `json:"request"`
	Expected p14IdentifierSemanticExpectedResult `json:"expected"`
}

type p14IdentifierSemanticMCPRequest struct {
	Action string `json:"action"`
	Symbol string `json:"symbol"`
}

type p14IdentifierSemanticExpectedResult struct {
	Outcome           string `json:"outcome"`
	Code              string `json:"code"`
	SameCallRetryable bool   `json:"same_call_retryable"`
	RecoveryAction    string `json:"recovery_action"`
}

type p14IdentifierMCPSurface struct {
	Schema                string                     `json:"schema"`
	SemanticRequestDigest string                     `json:"semantic_request_digest"`
	Tool                  string                     `json:"tool"`
	Cases                 []p14IdentifierMCPCallCase `json:"cases"`
}

type p14IdentifierMCPCallCase struct {
	ID   string         `json:"id"`
	Args map[string]any `json:"args"`
}

type p14IdentifierNormalizedOutput struct {
	Schema string                              `json:"schema"`
	Cases  []p14IdentifierNormalizedCaseOutput `json:"cases"`
}

type p14IdentifierNormalizedCaseOutput struct {
	ID      string                       `json:"id"`
	Outcome string                       `json:"outcome"`
	Error   p14IdentifierNormalizedError `json:"error"`
}

type p14IdentifierNormalizedError struct {
	Code              string                              `json:"code"`
	Tool              string                              `json:"tool"`
	Action            string                              `json:"action"`
	Parameter         string                              `json:"parameter"`
	Identifier        string                              `json:"identifier"`
	ReceivedNamespace string                              `json:"received_namespace"`
	ExpectedNamespace string                              `json:"expected_namespace"`
	SameCallRetryable bool                                `json:"same_call_retryable"`
	RecoveryCall      p14IdentifierNormalizedRecoveryCall `json:"recovery_call"`
}

type p14IdentifierNormalizedRecoveryCall struct {
	Tool      string                                   `json:"tool"`
	Arguments p14IdentifierNormalizedRecoveryArguments `json:"arguments"`
}

type p14IdentifierNormalizedRecoveryArguments struct {
	Action      string `json:"action"`
	ArtifactRef string `json:"artifact_ref"`
}

type p14IdentifierLocalOracle struct {
	Schema                string `json:"schema"`
	SemanticRequestDigest string `json:"semantic_request_digest"`
	ExpectedResultDigest  string `json:"expected_result_digest"`
	LocalOracleTest       string `json:"local_oracle_test"`
}

func buildP14IdentifierNamespaceScenario(
	declared scenarioContract,
	fixture p14IdentifierFixture,
) (preparedP14Scenario, error) {
	if err := validateP14IdentifierFixtureShape(fixture); err != nil {
		return preparedP14Scenario{}, err
	}
	semantic := canonicalP14IdentifierSemanticRequest(fixture.ArtifactRef)
	semanticBytes, err := marshalP14CanonicalJSON(semantic)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	semanticDigest := p14Digest(semanticBytes)
	surfaceBytes, err := buildP14IdentifierMCPSurface(semantic, semanticDigest)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	expected := canonicalP14IdentifierNormalizedOutput(fixture.ArtifactRef)
	expectedBytes, err := marshalP14CanonicalJSON(expected)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	expectedDigest := p14Digest(expectedBytes)
	localOracle := p14IdentifierLocalOracle{
		Schema:                p14IdentifierLocalOracleSchema,
		SemanticRequestDigest: semanticDigest,
		ExpectedResultDigest:  expectedDigest,
		LocalOracleTest:       p14IdentifierLocalOracleTest,
	}
	localOracleBytes, err := marshalP14CanonicalJSON(localOracle)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	request := preparedP14Request{
		Surface:               "live_mcp",
		Builder:               p14IdentifierNamespaceBuilderID,
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
			NormalizationID:         p14IdentifierNormalizationID,
			ExpectedResultDigest:    expectedDigest,
			ExpectedEffect:          declared.ExpectedEffect,
			LocalOracleOutputDigest: p14Digest(localOracleBytes),
		},
	}, nil
}

func canonicalP14IdentifierSemanticRequest(
	artifactRef string,
) p14IdentifierSemanticRequest {
	return p14IdentifierSemanticRequest{
		Schema:      p14IdentifierSemanticSchema,
		ArtifactRef: artifactRef,
		Cases: []p14IdentifierSemanticCase{
			{
				ID: p14IdentifierCaseID,
				Request: p14IdentifierSemanticMCPRequest{
					Action: "node",
					Symbol: artifactRef,
				},
				Expected: p14IdentifierSemanticExpectedResult{
					Outcome:           "error",
					Code:              "wrong_identifier_namespace",
					SameCallRetryable: false,
					RecoveryAction:    "related",
				},
			},
		},
	}
}

func buildP14IdentifierMCPSurface(
	semantic p14IdentifierSemanticRequest,
	semanticDigest string,
) ([]byte, error) {
	cases := make([]p14IdentifierMCPCallCase, 0, len(semantic.Cases))
	for _, testCase := range semantic.Cases {
		cases = append(cases, p14IdentifierMCPCallCase{
			ID: testCase.ID,
			Args: map[string]any{
				"action": testCase.Request.Action,
				"symbol": testCase.Request.Symbol,
			},
		})
	}
	payload := p14IdentifierMCPSurface{
		Schema:                p14IdentifierMCPSurfaceSchema,
		SemanticRequestDigest: semanticDigest,
		Tool:                  "haft_query",
		Cases:                 cases,
	}
	return marshalP14CanonicalJSON(payload)
}

func canonicalP14IdentifierNormalizedOutput(
	artifactRef string,
) p14IdentifierNormalizedOutput {
	return p14IdentifierNormalizedOutput{
		Schema: p14IdentifierNormalizedOutputSchema,
		Cases: []p14IdentifierNormalizedCaseOutput{
			{
				ID:      p14IdentifierCaseID,
				Outcome: "error",
				Error: p14IdentifierNormalizedError{
					Code:              "wrong_identifier_namespace",
					Tool:              "haft_query",
					Action:            "node",
					Parameter:         "symbol",
					Identifier:        artifactRef,
					ReceivedNamespace: "haft_artifact_id",
					ExpectedNamespace: "code_symbol",
					SameCallRetryable: false,
					RecoveryCall: p14IdentifierNormalizedRecoveryCall{
						Tool: "haft_query",
						Arguments: p14IdentifierNormalizedRecoveryArguments{
							Action:      "related",
							ArtifactRef: artifactRef,
						},
					},
				},
			},
		},
	}
}

func validateP14IdentifierNamespacePreparedScenario(
	declared scenarioContract,
	scenario preparedP14Scenario,
) error {
	if declared.RequestBuilder != p14IdentifierNamespaceBuilderID {
		return fmt.Errorf("P14 identifier validator received builder %q", declared.RequestBuilder)
	}
	semantic, err := decodeP14IdentifierSemanticRequest(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return err
	}
	expectedSemantic := canonicalP14IdentifierSemanticRequest(semantic.ArtifactRef)
	actualSemanticBytes, err := marshalP14CanonicalJSON(semantic)
	if err != nil {
		return err
	}
	expectedSemanticBytes, err := marshalP14CanonicalJSON(expectedSemantic)
	if err != nil {
		return err
	}
	if !bytes.Equal(actualSemanticBytes, expectedSemanticBytes) ||
		!artifact.IsCanonicalArtifactID(semantic.ArtifactRef) {
		return fmt.Errorf("P14 identifier semantic carrier is not the closed artifact rejection case")
	}
	expectedSurface, err := buildP14IdentifierMCPSurface(
		expectedSemantic,
		scenario.SemanticRequestDigest,
	)
	if err != nil {
		return err
	}
	if len(scenario.Requests) != 1 ||
		scenario.Requests[0].CanonicalPayload != string(expectedSurface) ||
		scenario.Requests[0].Encoding != "canonical_json" {
		return fmt.Errorf("P14 identifier MCP request is not derived from the semantic carrier")
	}
	expectedOutput := canonicalP14IdentifierNormalizedOutput(semantic.ArtifactRef)
	expectedOutputBytes, err := marshalP14CanonicalJSON(expectedOutput)
	if err != nil {
		return err
	}
	expectedDigest := p14Digest(expectedOutputBytes)
	localOracle := p14IdentifierLocalOracle{
		Schema:                p14IdentifierLocalOracleSchema,
		SemanticRequestDigest: scenario.SemanticRequestDigest,
		ExpectedResultDigest:  expectedDigest,
		LocalOracleTest:       p14IdentifierLocalOracleTest,
	}
	localOracleBytes, err := marshalP14CanonicalJSON(localOracle)
	if err != nil {
		return err
	}
	if scenario.Oracle.NormalizationID != p14IdentifierNormalizationID ||
		scenario.Oracle.ExpectedResultDigest != expectedDigest ||
		scenario.Oracle.LocalOracleOutputDigest != p14Digest(localOracleBytes) {
		return fmt.Errorf("P14 identifier oracle is not the closed normalized error")
	}
	return nil
}

func decodeP14IdentifierSemanticRequest(
	raw []byte,
) (p14IdentifierSemanticRequest, error) {
	reader := bytes.NewReader(raw)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var semantic p14IdentifierSemanticRequest
	if err := decoder.Decode(&semantic); err != nil {
		return p14IdentifierSemanticRequest{}, fmt.Errorf(
			"decode P14 identifier semantic carrier: %w",
			err,
		)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		return p14IdentifierSemanticRequest{}, fmt.Errorf(
			"P14 identifier semantic carrier has trailing JSON",
		)
	}
	return semantic, nil
}

func validateP14IdentifierFixtureShape(fixture p14IdentifierFixture) error {
	if fixture.Schema != p14IdentifierFixtureSchema ||
		!artifact.IsCanonicalArtifactID(fixture.ArtifactRef) ||
		!validP14Digest(fixture.ArtifactCarrierDigest) {
		return fmt.Errorf("P14 identifier fixture identity is invalid")
	}
	clean := filepath.Clean(filepath.FromSlash(fixture.ArtifactCarrierPath))
	portable := filepath.ToSlash(clean)
	if filepath.IsAbs(clean) ||
		strings.HasPrefix(portable, "../") ||
		!strings.HasPrefix(portable, ".haft/") {
		return fmt.Errorf("P14 identifier artifact carrier path is invalid")
	}
	base := filepath.Base(clean)
	if !strings.HasPrefix(base, fixture.ArtifactRef+".") {
		return fmt.Errorf("P14 identifier artifact carrier does not name its artifact")
	}
	return nil
}

func verifyP14IdentifierFixtureBinding(
	repositoryRoot string,
	input preparedRequestOracleInput,
) error {
	binding, err := preparedP14BindingByGroup(input.Bindings, "identifier_fixture")
	if err != nil {
		return err
	}
	path := filepath.Join(repositoryRoot, filepath.FromSlash(binding.CarrierPath))
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read P14 identifier fixture: %w", err)
	}
	fixture, err := decodeP14IdentifierFixture(raw)
	if err != nil {
		return err
	}
	if err := validateP14IdentifierFixtureShape(fixture); err != nil {
		return err
	}
	artifactPath := filepath.Join(
		repositoryRoot,
		filepath.FromSlash(fixture.ArtifactCarrierPath),
	)
	if err := verifyP14FileDigest(artifactPath, fixture.ArtifactCarrierDigest); err != nil {
		return fmt.Errorf("verify P14 identifier artifact carrier: %w", err)
	}
	scenario, err := preparedP14ScenarioByID(input.Scenarios, "identifier_namespace")
	if err != nil {
		return err
	}
	semantic, err := decodeP14IdentifierSemanticRequest(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return err
	}
	if semantic.ArtifactRef != fixture.ArtifactRef {
		return fmt.Errorf("P14 identifier request differs from the frozen fixture")
	}
	return nil
}

func decodeP14IdentifierFixture(raw []byte) (p14IdentifierFixture, error) {
	reader := bytes.NewReader(raw)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var fixture p14IdentifierFixture
	if err := decoder.Decode(&fixture); err != nil {
		return p14IdentifierFixture{}, fmt.Errorf("decode P14 identifier fixture: %w", err)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		return p14IdentifierFixture{}, fmt.Errorf("P14 identifier fixture has trailing JSON")
	}
	canonical, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		return p14IdentifierFixture{}, err
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(raw, canonical) {
		return p14IdentifierFixture{}, fmt.Errorf("P14 identifier fixture is not canonical JSON")
	}
	return fixture, nil
}

func preparedP14BindingByGroup(
	bindings []preparedP14Binding,
	group string,
) (preparedP14Binding, error) {
	for _, binding := range bindings {
		if binding.Group == group {
			return binding, nil
		}
	}
	return preparedP14Binding{}, fmt.Errorf("P14 binding %q is absent", group)
}

func preparedP14ScenarioByID(
	scenarios []preparedP14Scenario,
	id string,
) (preparedP14Scenario, error) {
	for _, scenario := range scenarios {
		if scenario.ID == id {
			return scenario, nil
		}
	}
	return preparedP14Scenario{}, fmt.Errorf("P14 scenario %q is absent", id)
}

func syntheticP14IdentifierFixture() p14IdentifierFixture {
	return p14IdentifierFixture{
		Schema:                p14IdentifierFixtureSchema,
		ArtifactRef:           "note-20260717-a1b2c3d4",
		ArtifactCarrierPath:   ".haft/notes/note-20260717-a1b2c3d4.md",
		ArtifactCarrierDigest: p14TestDigest("identifier-artifact"),
	}
}

func TestP14IdentifierNamespaceBuilderClosesExactRecovery(t *testing.T) {
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, _, err := loadRequestOracleContract(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	declared, err := findP14ScenarioContract(contract, "identifier_namespace")
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := buildP14IdentifierNamespaceScenario(
		declared,
		syntheticP14IdentifierFixture(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateP14IdentifierNamespacePreparedScenario(declared, scenario); err != nil {
		t.Fatal(err)
	}
	tampered := scenario
	tampered.SemanticRequestCanonical = strings.Replace(
		scenario.SemanticRequestCanonical,
		`"recovery_action":"related"`,
		`"recovery_action":"node"`,
		1,
	)
	tampered.SemanticRequestDigest = p14Digest([]byte(tampered.SemanticRequestCanonical))
	if err := validateP14IdentifierNamespacePreparedScenario(declared, tampered); err == nil {
		t.Fatal("P14 identifier validator accepted same-call retry recovery")
	}
}

func findP14ScenarioContract(
	contract requestOracleContract,
	id string,
) (scenarioContract, error) {
	for _, scenario := range contract.Scenarios {
		if scenario.ID == id {
			return scenario, nil
		}
	}
	return scenarioContract{}, fmt.Errorf("P14 scenario %q is absent", id)
}
