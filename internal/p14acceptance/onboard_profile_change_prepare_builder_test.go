package p14acceptance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const (
	p14OnboardProfileChangeBuilderID       = "onboard.profile-change-prepare-replay.v1"
	p14OnboardProfileChangeSemanticSchema  = "haft.p14.onboard-profile-change-semantic/v1"
	p14OnboardProfileChangeSurfaceSchema   = "haft.p14.onboard-profile-change-mcp/v1"
	p14OnboardProfileChangeOutputSchema    = "haft.p14.onboard-profile-change-output/v1"
	p14OnboardProfileChangeOracleSchema    = "haft.p14.onboard-profile-change-local-oracle/v1"
	p14OnboardProfileChangeNormalizationID = "p14.onboard-profile-change-prepare.semantic-output.v1"
)

type p14OnboardProfileChangeSemantic struct {
	Schema               string         `json:"schema"`
	SemanticCorpusDigest string         `json:"semantic_corpus_digest"`
	CorpusCaseDigest     string         `json:"corpus_case_digest"`
	Args                 map[string]any `json:"args"`
}

type p14OnboardProfileChangeSurface struct {
	Schema                string                           `json:"schema"`
	SemanticRequestDigest string                           `json:"semantic_request_digest"`
	Tool                  string                           `json:"tool"`
	Cases                 []p14OnboardProfileChangeMCPCase `json:"cases"`
}

type p14OnboardProfileChangeMCPCase struct {
	ID   string         `json:"id"`
	Args map[string]any `json:"args"`
}

type p14OnboardProfileChangeNormalizedOutput struct {
	Schema                    string   `json:"schema"`
	SemanticCorpusDigest      string   `json:"semantic_corpus_digest"`
	Results                   []string `json:"results"`
	CanonicalProfileUnchanged bool     `json:"canonical_profile_unchanged"`
	ProjectionUnchanged       bool     `json:"canonical_projection_unchanged"`
	StructuredMemoryEnabled   bool     `json:"structured_memory_enabled"`
	AuthorityGranted          bool     `json:"authority_granted"`
	ApplyCalls                int      `json:"apply_calls"`
}

type p14OnboardProfileChangeOracle struct {
	Schema                string `json:"schema"`
	SemanticRequestDigest string `json:"semantic_request_digest"`
	ExpectedResultDigest  string `json:"expected_result_digest"`
}

func loadP14OnboardProfileChangeCorpusCase() (
	p14AgentFPFCorpusScenario,
	string,
	error,
) {
	root, err := p14RepositoryRoot()
	if err != nil {
		return p14AgentFPFCorpusScenario{}, "", err
	}
	raw, err := os.ReadFile(filepath.Join(
		root,
		filepath.FromSlash(p14HReasonSemanticCorpusPath),
	))
	if err != nil {
		return p14AgentFPFCorpusScenario{}, "", err
	}
	var corpus p14AgentFPFCorpusEnvelope
	if err := json.Unmarshal(raw, &corpus); err != nil {
		return p14AgentFPFCorpusScenario{}, "", err
	}
	for _, scenario := range corpus.Scenarios {
		if scenario.ID == "profile_change_prepare_no_apply" {
			return scenario, p14Digest(raw), nil
		}
	}
	return p14AgentFPFCorpusScenario{}, "", fmt.Errorf(
		"P14 semantic corpus omits profile_change_prepare_no_apply",
	)
}

func buildP14OnboardProfileChangeScenario(
	declared scenarioContract,
) (preparedP14Scenario, error) {
	if declared.ID != "onboard_profile_change_prepare" ||
		declared.RequestBuilder != p14OnboardProfileChangeBuilderID ||
		declared.OracleKind != "normalized_digest" ||
		declared.ExpectedEffect != "fixture_non_binding_review_write" ||
		!slices.Equal(declared.Surfaces, []string{"live_mcp"}) {
		return preparedP14Scenario{}, fmt.Errorf(
			"P14 onboard profile-change declaration differs",
		)
	}
	corpusCase, corpusDigest, err := loadP14OnboardProfileChangeCorpusCase()
	if err != nil {
		return preparedP14Scenario{}, err
	}
	var args map[string]any
	for _, event := range corpusCase.Events {
		if p14AgentFPFJSONText(event["kind"]) != "tool_call" ||
			p14AgentFPFJSONText(event["tool"]) != "haft_onboard" {
			continue
		}
		if err := json.Unmarshal(event["payload"], &args); err != nil {
			return preparedP14Scenario{}, err
		}
	}
	if args["action"] != "profile_change_prepare" ||
		args["scope_id"] == "" || args["entity_ref"] == "" ||
		corpusCase.Expected.Writes != 1 ||
		!slices.Equal(corpusCase.Expected.Effects, []string{"non_binding_review_carrier"}) {
		return preparedP14Scenario{}, fmt.Errorf(
			"P14 onboard profile-change corpus case differs",
		)
	}
	caseBytes, err := marshalP14CanonicalJSON(corpusCase)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	semantic := p14OnboardProfileChangeSemantic{
		Schema:               p14OnboardProfileChangeSemanticSchema,
		SemanticCorpusDigest: corpusDigest,
		CorpusCaseDigest:     p14Digest(caseBytes),
		Args:                 args,
	}
	semanticBytes, err := marshalP14CanonicalJSON(semantic)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	semanticDigest := p14Digest(semanticBytes)
	statusArgs := map[string]any{"action": "status"}
	prepareArgs, err := cloneP14JSONMap(args)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	replayArgs, err := cloneP14JSONMap(args)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	surface := p14OnboardProfileChangeSurface{
		Schema:                p14OnboardProfileChangeSurfaceSchema,
		SemanticRequestDigest: semanticDigest,
		Tool:                  "haft_onboard",
		Cases: []p14OnboardProfileChangeMCPCase{
			{ID: "status_before", Args: statusArgs},
			{ID: "prepare", Args: prepareArgs},
			{ID: "replay", Args: replayArgs},
			{ID: "status_after", Args: statusArgs},
		},
	}
	surfaceBytes, err := marshalP14CanonicalJSON(surface)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	expected := canonicalP14OnboardProfileChangeOutput(corpusDigest)
	expectedBytes, err := marshalP14CanonicalJSON(expected)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	oracle := p14OnboardProfileChangeOracle{
		Schema:                p14OnboardProfileChangeOracleSchema,
		SemanticRequestDigest: semanticDigest,
		ExpectedResultDigest:  p14Digest(expectedBytes),
	}
	oracleBytes, err := marshalP14CanonicalJSON(oracle)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	return preparedP14Scenario{
		ID:                       declared.ID,
		SemanticRequestCanonical: string(semanticBytes),
		SemanticRequestDigest:    semanticDigest,
		Requests: []preparedP14Request{{
			Surface:               "live_mcp",
			Builder:               declared.RequestBuilder,
			Encoding:              "canonical_json",
			CanonicalPayload:      string(surfaceBytes),
			PayloadDigest:         p14Digest(surfaceBytes),
			SemanticRequestDigest: semanticDigest,
		}},
		Oracle: preparedP14Oracle{
			Kind:                    declared.OracleKind,
			NormalizationID:         p14OnboardProfileChangeNormalizationID,
			ExpectedResultDigest:    p14Digest(expectedBytes),
			ExpectedEffect:          declared.ExpectedEffect,
			LocalOracleOutputDigest: p14Digest(oracleBytes),
		},
	}, nil
}

func canonicalP14OnboardProfileChangeOutput(
	corpusDigest string,
) p14OnboardProfileChangeNormalizedOutput {
	return p14OnboardProfileChangeNormalizedOutput{
		Schema:               p14OnboardProfileChangeOutputSchema,
		SemanticCorpusDigest: corpusDigest,
		Results: []string{
			"profile_change_review_prepared",
			"profile_change_review_reused",
		},
		CanonicalProfileUnchanged: true,
		ProjectionUnchanged:       true,
		StructuredMemoryEnabled:   false,
		AuthorityGranted:          false,
		ApplyCalls:                0,
	}
}

func validateP14OnboardProfileChangePreparedScenario(
	declared scenarioContract,
	scenario preparedP14Scenario,
) error {
	want, err := buildP14OnboardProfileChangeScenario(declared)
	if err != nil {
		return err
	}
	wantBytes, err := marshalP14CanonicalJSON(want)
	if err != nil {
		return err
	}
	gotBytes, err := marshalP14CanonicalJSON(scenario)
	if err != nil {
		return err
	}
	if !bytes.Equal(gotBytes, wantBytes) {
		return fmt.Errorf("P14 onboard profile-change prepared bytes differ")
	}
	return nil
}

func p14CodexMCPOnboardProfileChangeCalls(
	_ preparedP14Scenario,
	request preparedP14Request,
) ([]p14CodexMCPCallDefinition, error) {
	var surface p14OnboardProfileChangeSurface
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"actual Codex MCP onboard profile change",
	); err != nil {
		return nil, err
	}
	definitions := make([]p14CodexMCPCallDefinition, 0, len(surface.Cases))
	for _, testCase := range surface.Cases {
		args, err := cloneP14JSONMap(testCase.Args)
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, p14CodexMCPCallDefinition{
			CaseID: testCase.ID,
			Tool:   surface.Tool,
			Args:   args,
		})
	}
	return definitions, nil
}

type p14OnboardResponseWire struct {
	Result        string          `json:"result"`
	Status        string          `json:"status"`
	ProfileOrigin string          `json:"profile_origin"`
	Scopes        json.RawMessage `json:"scopes"`
	Effects       struct {
		ReviewCarrierCreated    bool `json:"review_carrier_created"`
		ReviewCarrierReused     bool `json:"review_carrier_reused"`
		CanonicalProfileChanged bool `json:"canonical_profile_changed"`
		StructuredMemoryEnabled bool `json:"structured_memory_enabled"`
		AuthorityGranted        bool `json:"authority_granted"`
	} `json:"effects"`
}

func normalizeP14CodexMCPOnboardProfileChange(
	_ preparedRequestOracleCarrier,
	scenario preparedP14Scenario,
	_ preparedP14Request,
	evidence []p14CodexMCPCallEvidence,
) (p14CodexMCPFamilyResult, error) {
	if len(evidence) != 4 {
		return p14CodexMCPNormalizedFailure(
			"onboard_profile_change_mismatch",
			"profile-change prepare/replay call count differs",
			"closed_onboard_profile_change_normalizer",
		), nil
	}
	wires := make([]p14OnboardResponseWire, 0, len(evidence))
	for index, wantID := range []string{"status_before", "prepare", "replay", "status_after"} {
		if evidence[index].CaseID != wantID || evidence[index].Response.IsError {
			return p14CodexMCPNormalizedFailure(
				"onboard_profile_change_mismatch",
				"profile-change response order or error posture differs",
				"closed_onboard_profile_change_normalizer",
			), nil
		}
		body, err := p14CodexMCPResponseBody(evidence[index])
		if err != nil {
			return p14CodexMCPFamilyResult{}, err
		}
		var wire p14OnboardResponseWire
		if err := json.Unmarshal(body, &wire); err != nil {
			return p14CodexMCPNormalizedFailure(
				"onboard_profile_change_mismatch",
				err.Error(),
				"closed_onboard_profile_change_normalizer",
			), nil
		}
		wires = append(wires, wire)
	}
	prepared := wires[1]
	reused := wires[2]
	before := wires[0]
	after := wires[3]
	unchanged := bytes.Equal(before.Scopes, after.Scopes) &&
		before.ProfileOrigin == after.ProfileOrigin
	effectsClosed := prepared.Result == "profile_change_review_prepared" &&
		reused.Result == "profile_change_review_reused" &&
		prepared.Effects.ReviewCarrierCreated &&
		!prepared.Effects.ReviewCarrierReused &&
		!reused.Effects.ReviewCarrierCreated &&
		reused.Effects.ReviewCarrierReused &&
		!prepared.Effects.CanonicalProfileChanged &&
		!reused.Effects.CanonicalProfileChanged &&
		!prepared.Effects.StructuredMemoryEnabled &&
		!reused.Effects.StructuredMemoryEnabled &&
		!prepared.Effects.AuthorityGranted &&
		!reused.Effects.AuthorityGranted
	if !unchanged || !effectsClosed {
		return p14CodexMCPNormalizedFailure(
			"onboard_profile_change_mismatch",
			"profile-change prepare crossed apply/authority boundary or failed exact reuse",
			"closed_onboard_profile_change_normalizer",
		), nil
	}
	var semantic p14OnboardProfileChangeSemantic
	if err := decodeP14StrictCompactJSON(
		scenario.SemanticRequestCanonical,
		&semantic,
		"P14 onboard profile-change semantic request",
	); err != nil {
		return p14CodexMCPFamilyResult{}, err
	}
	return p14CodexMCPNormalizedResult(
		canonicalP14OnboardProfileChangeOutput(semantic.SemanticCorpusDigest),
		nil,
		"exact_prepare_then_reuse",
		"canonical_profile_and_projection_unchanged",
		"zero_apply_memory_or_authority_effects",
		"closed_onboard_profile_change_normalizer",
	)
}

func TestP14OnboardProfileChangePrepareBuilderClosesReplayAndNoApply(t *testing.T) {
	root, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, _, err := loadRequestOracleContract(root)
	if err != nil {
		t.Fatal(err)
	}
	declared, err := findP14ScenarioContract(contract, "onboard_profile_change_prepare")
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := buildP14OnboardProfileChangeScenario(declared)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateP14OnboardProfileChangePreparedScenario(declared, scenario); err != nil {
		t.Fatal(err)
	}
	request := scenario.Requests[0]
	definitions, err := p14CodexMCPOnboardProfileChangeCalls(scenario, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(definitions) != 4 ||
		definitions[1].Args["action"] != "profile_change_prepare" ||
		definitions[1].Args["entity_ref"] != "entity:p14-profile-change-target" ||
		!bytes.Equal(
			[]byte(definitions[1].Args["scope_id"].(string)),
			[]byte(definitions[2].Args["scope_id"].(string)),
		) {
		t.Fatalf("P14 onboard profile-change definitions = %#v", definitions)
	}
}
