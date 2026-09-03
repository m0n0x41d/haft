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

	"github.com/m0n0x41d/haft/internal/artifact"
)

const (
	p14IdentifierNamespaceBuilderID     = "query.identifier-namespace-matrix.v2"
	p14IdentifierSemanticSchema         = "haft.p14.identifier-namespace-semantic/v2"
	p14IdentifierMCPSurfaceSchema       = "haft.p14.identifier-namespace-mcp/v2"
	p14IdentifierNormalizedOutputSchema = "haft.p14.identifier-namespace-output/v2"
	p14IdentifierLocalOracleSchema      = "haft.p14.identifier-namespace-local-oracle/v2"
	p14IdentifierFixtureSchema          = "haft.p14.identifier-fixture/v1"
	p14IdentifierNormalizationID        = "p14.identifier-namespace.semantic-recovery.v2"
	p14IdentifierFPFRef                 = "A.7"
	p14IdentifierCodeSymbol             = "NeighborhoodRead"
	p14IdentifierMemoryRef              = "entity:haft"
)

var p14IdentifierLocalOracleTests = []string{
	"github.com/m0n0x41d/haft/internal/cli::TestEveryEmittedQueryRecoveryCallPassesPublicSchemaAndStrictDecoder",
	"github.com/m0n0x41d/haft/internal/cli::TestExactIdentifierNamespaceMatrixRecoversAndExecutesUnchanged",
	"github.com/m0n0x41d/haft/internal/cli::TestExactIdentifierNamespaceMatrixDoesNotGuess",
}

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
	Tool string         `json:"tool"`
	Args map[string]any `json:"args"`
}

type p14IdentifierSemanticExpectedResult struct {
	Outcome          string                       `json:"outcome"`
	Error            p14IdentifierNormalizedError `json:"error"`
	RecoveryExecuted bool                         `json:"recovery_executed"`
}

type p14IdentifierMCPSurface struct {
	Schema                string                     `json:"schema"`
	SemanticRequestDigest string                     `json:"semantic_request_digest"`
	Cases                 []p14IdentifierMCPCallCase `json:"cases"`
}

type p14IdentifierMCPCallCase struct {
	ID   string         `json:"id"`
	Tool string         `json:"tool"`
	Args map[string]any `json:"args"`
}

type p14IdentifierNormalizedOutput struct {
	Schema string                              `json:"schema"`
	Cases  []p14IdentifierNormalizedCaseOutput `json:"cases"`
}

type p14IdentifierNormalizedCaseOutput struct {
	ID               string                       `json:"id"`
	Outcome          string                       `json:"outcome"`
	Error            p14IdentifierNormalizedError `json:"error"`
	RecoveryExecuted bool                         `json:"recovery_executed"`
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
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments"`
}

type p14IdentifierLocalOracle struct {
	Schema                string   `json:"schema"`
	SemanticRequestDigest string   `json:"semantic_request_digest"`
	ExpectedResultDigest  string   `json:"expected_result_digest"`
	LocalOracleTests      []string `json:"local_oracle_tests"`
}

type p14IdentifierRoute struct {
	Action    string
	Parameter string
	Namespace string
	Args      func(string) map[string]any
	Recovery  func(string) map[string]any
}

func p14IdentifierRoutes() []p14IdentifierRoute {
	return []p14IdentifierRoute{
		{
			Action: "node", Parameter: "symbol", Namespace: "code_symbol",
			Args: func(identifier string) map[string]any {
				return map[string]any{"action": "node", "symbol": identifier}
			},
			Recovery: func(identifier string) map[string]any {
				return map[string]any{"action": "node", "symbol": identifier}
			},
		},
		{
			Action: "fpf", Parameter: "identifier", Namespace: "fpf_source_identifier",
			Args: func(identifier string) map[string]any {
				return map[string]any{"action": "fpf", "mode": "inspect", "identifier": identifier}
			},
			Recovery: func(identifier string) map[string]any {
				return map[string]any{"action": "fpf", "mode": "inspect", "identifier": identifier}
			},
		},
		{
			Action: "related", Parameter: "artifact_ref", Namespace: "haft_artifact_id",
			Args: func(identifier string) map[string]any {
				return map[string]any{"action": "related", "artifact_ref": identifier}
			},
			Recovery: func(identifier string) map[string]any {
				return map[string]any{"action": "related", "artifact_ref": identifier}
			},
		},
		{
			Action: "memory", Parameter: "query", Namespace: "typed_memory_entity_id",
			Recovery: func(identifier string) map[string]any {
				return map[string]any{
					"action": "memory",
					"memory_request": map[string]any{
						"contract_version": "haft.memory.v1",
						"mode":             "resolve",
						"basis":            map[string]any{"kind": "project_current"},
						"query":            identifier,
						"max_candidates":   8,
					},
				}
			},
		},
	}
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
		LocalOracleTests:      slices.Clone(p14IdentifierLocalOracleTests),
	}
	localOracleBytes, err := marshalP14CanonicalJSON(localOracle)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	return preparedP14Scenario{
		ID:                       declared.ID,
		SemanticRequestCanonical: string(semanticBytes),
		SemanticRequestDigest:    semanticDigest,
		Requests: []preparedP14Request{{
			Surface:               "live_mcp",
			Builder:               p14IdentifierNamespaceBuilderID,
			Encoding:              "canonical_json",
			CanonicalPayload:      string(surfaceBytes),
			PayloadDigest:         p14Digest(surfaceBytes),
			SemanticRequestDigest: semanticDigest,
		}},
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
	routes := p14IdentifierRoutes()
	identifiers := []struct {
		Name       string
		Value      string
		RouteIndex int
	}{
		{Name: "artifact", Value: artifactRef, RouteIndex: 2},
		{Name: "fpf", Value: p14IdentifierFPFRef, RouteIndex: 1},
		{Name: "code", Value: p14IdentifierCodeSymbol, RouteIndex: 0},
		{Name: "memory", Value: p14IdentifierMemoryRef, RouteIndex: 3},
	}
	cases := make([]p14IdentifierSemanticCase, 0, 9)
	for targetIndex := 0; targetIndex < 3; targetIndex++ {
		target := routes[targetIndex]
		for _, identifier := range identifiers {
			if identifier.RouteIndex == targetIndex {
				continue
			}
			source := routes[identifier.RouteIndex]
			id := identifier.Name + "_in_" + p14IdentifierTargetName(target.Action)
			errorValue := p14IdentifierNormalizedError{
				Code:              "wrong_identifier_namespace",
				Tool:              "haft_query",
				Action:            target.Action,
				Parameter:         target.Parameter,
				Identifier:        identifier.Value,
				ReceivedNamespace: source.Namespace,
				ExpectedNamespace: target.Namespace,
				SameCallRetryable: false,
				RecoveryCall: p14IdentifierNormalizedRecoveryCall{
					Tool:      "haft_query",
					Arguments: source.Recovery(identifier.Value),
				},
			}
			cases = append(cases, p14IdentifierSemanticCase{
				ID: id,
				Request: p14IdentifierSemanticMCPRequest{
					Tool: "haft_query",
					Args: target.Args(identifier.Value),
				},
				Expected: p14IdentifierSemanticExpectedResult{
					Outcome:          "error_then_recovery",
					Error:            errorValue,
					RecoveryExecuted: true,
				},
			})
		}
	}
	return p14IdentifierSemanticRequest{
		Schema:      p14IdentifierSemanticSchema,
		ArtifactRef: artifactRef,
		Cases:       cases,
	}
}

func p14IdentifierTargetName(action string) string {
	if action == "related" {
		return "artifact"
	}
	return action
}

func buildP14IdentifierMCPSurface(
	semantic p14IdentifierSemanticRequest,
	semanticDigest string,
) ([]byte, error) {
	cases := make([]p14IdentifierMCPCallCase, 0, len(semantic.Cases)*2)
	for _, testCase := range semantic.Cases {
		cases = append(cases,
			p14IdentifierMCPCallCase{
				ID:   testCase.ID + "_reject",
				Tool: testCase.Request.Tool,
				Args: testCase.Request.Args,
			},
			p14IdentifierMCPCallCase{
				ID:   testCase.ID + "_recovery",
				Tool: testCase.Expected.Error.RecoveryCall.Tool,
				Args: testCase.Expected.Error.RecoveryCall.Arguments,
			},
		)
	}
	return marshalP14CanonicalJSON(p14IdentifierMCPSurface{
		Schema:                p14IdentifierMCPSurfaceSchema,
		SemanticRequestDigest: semanticDigest,
		Cases:                 cases,
	})
}

func canonicalP14IdentifierNormalizedOutput(
	artifactRef string,
) p14IdentifierNormalizedOutput {
	semantic := canonicalP14IdentifierSemanticRequest(artifactRef)
	cases := make([]p14IdentifierNormalizedCaseOutput, 0, len(semantic.Cases))
	for _, testCase := range semantic.Cases {
		cases = append(cases, p14IdentifierNormalizedCaseOutput{
			ID:               testCase.ID,
			Outcome:          testCase.Expected.Outcome,
			Error:            testCase.Expected.Error,
			RecoveryExecuted: testCase.Expected.RecoveryExecuted,
		})
	}
	return p14IdentifierNormalizedOutput{
		Schema: p14IdentifierNormalizedOutputSchema,
		Cases:  cases,
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
	want := canonicalP14IdentifierSemanticRequest(semantic.ArtifactRef)
	wantSemantic, _ := marshalP14CanonicalJSON(want)
	gotSemantic, _ := marshalP14CanonicalJSON(semantic)
	if !bytes.Equal(gotSemantic, wantSemantic) ||
		!artifact.IsCanonicalArtifactID(semantic.ArtifactRef) ||
		len(semantic.Cases) != 9 {
		return fmt.Errorf("P14 identifier semantic carrier is not the closed matrix v2")
	}
	wantSurface, err := buildP14IdentifierMCPSurface(want, scenario.SemanticRequestDigest)
	if err != nil {
		return err
	}
	if len(scenario.Requests) != 1 ||
		scenario.Requests[0].CanonicalPayload != string(wantSurface) ||
		scenario.Requests[0].Encoding != "canonical_json" {
		return fmt.Errorf("P14 identifier MCP calls are not derived from matrix v2")
	}
	wantOutput, err := marshalP14CanonicalJSON(
		canonicalP14IdentifierNormalizedOutput(semantic.ArtifactRef),
	)
	if err != nil {
		return err
	}
	wantDigest := p14Digest(wantOutput)
	localOracle, err := marshalP14CanonicalJSON(p14IdentifierLocalOracle{
		Schema:                p14IdentifierLocalOracleSchema,
		SemanticRequestDigest: scenario.SemanticRequestDigest,
		ExpectedResultDigest:  wantDigest,
		LocalOracleTests:      slices.Clone(p14IdentifierLocalOracleTests),
	})
	if err != nil {
		return err
	}
	if scenario.Oracle.NormalizationID != p14IdentifierNormalizationID ||
		scenario.Oracle.ExpectedResultDigest != wantDigest ||
		scenario.Oracle.LocalOracleOutputDigest != p14Digest(localOracle) {
		return fmt.Errorf("P14 identifier matrix v2 oracle differs")
	}
	return nil
}

func decodeP14IdentifierSemanticRequest(
	raw []byte,
) (p14IdentifierSemanticRequest, error) {
	reader := bytes.NewReader(raw)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	var semantic p14IdentifierSemanticRequest
	if err := decoder.Decode(&semantic); err != nil {
		return p14IdentifierSemanticRequest{}, fmt.Errorf(
			"decode P14 identifier semantic carrier: %w",
			err,
		)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
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
	if filepath.IsAbs(clean) || strings.HasPrefix(portable, "../") ||
		!strings.HasPrefix(portable, ".haft/") {
		return fmt.Errorf("P14 identifier artifact carrier path is invalid")
	}
	if !strings.HasPrefix(filepath.Base(clean), fixture.ArtifactRef+".") {
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
	raw, err := os.ReadFile(filepath.Join(
		repositoryRoot,
		filepath.FromSlash(binding.CarrierPath),
	))
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
	if err := verifyP14FileDigest(
		filepath.Join(repositoryRoot, filepath.FromSlash(fixture.ArtifactCarrierPath)),
		fixture.ArtifactCarrierDigest,
	); err != nil {
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
	if err := decoder.Decode(&trailing); err != io.EOF {
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

func TestP14IdentifierNamespaceBuilderClosesExecutableRecoveryMatrix(t *testing.T) {
	root, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, _, err := loadRequestOracleContract(root)
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
	semantic, err := decodeP14IdentifierSemanticRequest(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		t.Fatal(err)
	}
	definitions, err := p14CodexMCPIdentifierCalls(scenario, scenario.Requests[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(semantic.Cases) != 9 || len(definitions) != 18 {
		t.Fatalf("P14 identifier matrix = semantic:%d calls:%d", len(semantic.Cases), len(definitions))
	}
	for index, testCase := range semantic.Cases {
		recovery := definitions[index*2+1]
		want, _ := marshalP14CanonicalJSON(testCase.Expected.Error.RecoveryCall.Arguments)
		got, _ := marshalP14CanonicalJSON(recovery.Args)
		if !bytes.Equal(got, want) {
			t.Fatalf("P14 recovery %q changed arguments", testCase.ID)
		}
	}
}

func TestP14IdentifierNamespaceBuilderClosesExactRecovery(t *testing.T) {
	TestP14IdentifierNamespaceBuilderClosesExecutableRecoveryMatrix(t)
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
