package p14acceptance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

const (
	p14MemoryOperationSemanticSchema    = "haft.p14.memory-operation-semantic/v2"
	p14MemoryOperationCLISchema         = "haft.p14.memory-operation-cli/v2"
	p14MemoryOperationMCPSchema         = "haft.p14.memory-operation-mcp/v1"
	p14MemoryOperationOutputSchema      = "haft.p14.memory-operation-output/v1"
	p14MemoryOperationLocalOracleSchema = "haft.p14.memory-operation-local-oracle/v1"
	p14MemoryOperationNormalizationID   = "p14.memory-operation.semantic-outcome.v1"
	p14MemoryOperationFixtureIsolation  = "selected_project_read_only_with_fresh_home_clone_per_scenario"
	p14MemoryOperationLiveMCPExecution  = "selected_project_ordered_dogfood"
)

var p14MemoryOperationBuilderIDs = []string{
	"memory.admit-valid.v2",
	"memory.validate-invalid.v2",
	"memory.validate-underdetermined.v2",
	"memory.authority-rejection.v1",
	"memory.concurrent-replay.v1",
}

type p14MemoryOperationFixture struct {
	SelectedProjectRoot        string                          `json:"selected_project_root"`
	SelectedProjectBasisDigest string                          `json:"selected_project_basis_digest"`
	HomeTemplateRoot           string                          `json:"home_template_root"`
	HomeTemplateDigest         string                          `json:"home_template_digest"`
	Cases                      []p14MemoryOperationFixtureCase `json:"cases"`
}

type p14MemoryOperationFixtureCase struct {
	ScenarioID string                   `json:"scenario_id"`
	BuilderID  string                   `json:"builder_id"`
	Steps      []p14MemoryOperationStep `json:"steps"`
}

type p14MemoryOperationStep struct {
	ID            string          `json:"id"`
	ParallelGroup string          `json:"parallel_group,omitempty"`
	Request       json.RawMessage `json:"request"`
}

type p14MemoryOperationStepPolicy struct {
	ID            string
	Action        string
	ParallelGroup string
}

type p14MemoryOperationPolicy struct {
	ScenarioID          string
	BuilderID           string
	Protocol            string
	LiveMCPPredecessors []string
	Steps               []p14MemoryOperationStepPolicy
	Verdict             string
	AdmissionResult     string
	ConflictResult      string
	BoundaryErrorCode   string
	BoundaryErrorPath   string
	CommitCount         int
	GraphRevisionDelta  int64
	Assertions          []string
}

type p14MemoryOperationSemanticRequest struct {
	Schema                       string                     `json:"schema"`
	ScenarioID                   string                     `json:"scenario_id"`
	Protocol                     string                     `json:"protocol"`
	InstalledCLIFixtureIsolation string                     `json:"installed_cli_fixture_isolation"`
	LiveMCPExecutionContext      string                     `json:"live_mcp_execution_context"`
	SelectedProjectRoot          string                     `json:"selected_project_root"`
	SelectedProjectBasisDigest   string                     `json:"selected_project_basis_digest"`
	HomeTemplateRoot             string                     `json:"home_template_root"`
	HomeTemplateDigest           string                     `json:"home_template_digest"`
	Steps                        []p14MemoryOperationStep   `json:"steps"`
	Expected                     p14MemoryOperationExpected `json:"expected"`
}

type p14MemoryOperationExpected struct {
	Verdict            string   `json:"verdict,omitempty"`
	AdmissionResult    string   `json:"admission_result,omitempty"`
	ConflictResult     string   `json:"conflict_result,omitempty"`
	BoundaryErrorCode  string   `json:"boundary_error_code,omitempty"`
	BoundaryErrorPath  string   `json:"boundary_error_path,omitempty"`
	CommitCount        int      `json:"commit_count"`
	GraphRevisionDelta int64    `json:"graph_revision_delta"`
	AuthorityGranted   bool     `json:"authority_granted"`
	Assertions         []string `json:"assertions"`
}

type p14MemoryOperationCLISurface struct {
	Schema                     string                      `json:"schema"`
	SemanticRequestDigest      string                      `json:"semantic_request_digest"`
	FixtureIsolation           string                      `json:"fixture_isolation"`
	SelectedProjectRoot        string                      `json:"selected_project_root"`
	SelectedProjectBasisDigest string                      `json:"selected_project_basis_digest"`
	HomeTemplateRoot           string                      `json:"home_template_root"`
	HomeTemplateDigest         string                      `json:"home_template_digest"`
	Calls                      []p14MemoryOperationCLICall `json:"calls"`
}

type p14MemoryOperationCLICall struct {
	ID            string   `json:"id"`
	ParallelGroup string   `json:"parallel_group,omitempty"`
	Argv          []string `json:"argv"`
	Stdin         string   `json:"stdin"`
}

type p14MemoryOperationMCPSurface struct {
	Schema                string                      `json:"schema"`
	SemanticRequestDigest string                      `json:"semantic_request_digest"`
	ExecutionContext      string                      `json:"execution_context"`
	RequiredPredecessors  []string                    `json:"required_predecessors"`
	Calls                 []p14MemoryOperationMCPCall `json:"calls"`
}

type p14MemoryOperationMCPCall struct {
	ID            string         `json:"id"`
	ParallelGroup string         `json:"parallel_group,omitempty"`
	Tool          string         `json:"tool"`
	Args          map[string]any `json:"args"`
}

type p14MemoryOperationNormalizedOutput struct {
	Schema     string                     `json:"schema"`
	ScenarioID string                     `json:"scenario_id"`
	Expected   p14MemoryOperationExpected `json:"expected"`
}

type p14MemoryOperationLocalOracle struct {
	Schema                string   `json:"schema"`
	SemanticRequestDigest string   `json:"semantic_request_digest"`
	ExpectedResultDigest  string   `json:"expected_result_digest"`
	LocalOracleTests      []string `json:"local_oracle_tests"`
}

func p14MemoryOperationPolicies() []p14MemoryOperationPolicy {
	return []p14MemoryOperationPolicy{
		{
			ScenarioID: "positive_typed_write",
			BuilderID:  "memory.admit-valid.v2",
			Protocol:   "validate_admit_replay_reread",
			Steps: []p14MemoryOperationStepPolicy{
				{ID: "validate", Action: typedmemorywire.ActionValidate},
				{ID: "admit", Action: typedmemorywire.ActionAdmit},
				{ID: "replay", Action: typedmemorywire.ActionAdmit},
				{ID: "reread", Action: typedmemorywire.ActionNeighborhood},
			},
			Verdict:            "valid",
			AdmissionResult:    "committed_then_exact_replay",
			CommitCount:        1,
			GraphRevisionDelta: 1,
			Assertions: []string{
				"validation_writes_zero_rows",
				"admission_commits_once",
				"same_key_replay_returns_original_receipt",
				"exact_reread_observes_admitted_record",
				"binding_authority_not_granted",
			},
		},
		{
			ScenarioID: "invalid",
			BuilderID:  "memory.validate-invalid.v2",
			Protocol:   "validate_without_write",
			Steps: []p14MemoryOperationStepPolicy{
				{ID: "validate", Action: typedmemorywire.ActionValidate},
			},
			Verdict:            "invalid",
			GraphRevisionDelta: 0,
			Assertions: []string{
				"positive_contradiction_remains_invalid",
				"semantic_rows_unchanged",
				"binding_authority_not_granted",
			},
		},
		{
			ScenarioID: "underdetermined",
			BuilderID:  "memory.validate-underdetermined.v2",
			Protocol:   "validate_without_write",
			Steps: []p14MemoryOperationStepPolicy{
				{ID: "validate", Action: typedmemorywire.ActionValidate},
			},
			Verdict:            "underdetermined",
			GraphRevisionDelta: 0,
			Assertions: []string{
				"missing_basis_remains_underdetermined",
				"semantic_rows_unchanged",
				"binding_authority_not_granted",
			},
		},
		{
			ScenarioID: "authority_rejection",
			BuilderID:  "memory.authority-rejection.v1",
			Protocol:   "strict_boundary_rejection",
			Steps: []p14MemoryOperationStepPolicy{
				{ID: "reject", Action: typedmemorywire.ActionAdmit},
			},
			BoundaryErrorCode:  string(typedmemorywire.ErrorInvalidContract),
			BoundaryErrorPath:  "$.authority_class",
			GraphRevisionDelta: 0,
			Assertions: []string{
				"binding_authority_is_unrepresentable",
				"semantic_rows_unchanged",
				"typeenv_head_unchanged",
				"spec_decision_and_commission_state_unchanged",
			},
		},
		{
			ScenarioID: "concurrency_idempotency",
			BuilderID:  "memory.concurrent-replay.v1",
			Protocol:   "parallel_same_request_then_replay_and_conflict",
			LiveMCPPredecessors: []string{
				"positive_typed_write",
			},
			Steps: []p14MemoryOperationStepPolicy{
				{ID: "writer_a", Action: typedmemorywire.ActionAdmit, ParallelGroup: "same_request"},
				{ID: "writer_b", Action: typedmemorywire.ActionAdmit, ParallelGroup: "same_request"},
				{ID: "replay", Action: typedmemorywire.ActionAdmit},
				{ID: "conflict", Action: typedmemorywire.ActionAdmit},
			},
			AdmissionResult:    "one_commit_and_exact_replays",
			ConflictResult:     "idempotency_conflict",
			CommitCount:        1,
			GraphRevisionDelta: 1,
			Assertions: []string{
				"parallel_same_request_commits_once",
				"all_same_request_receipts_are_identical",
				"same_key_different_request_is_rejected",
				"conflict_does_not_hide_ambiguity",
				"binding_authority_not_granted",
			},
		},
	}
}

func buildP14MemoryOperationScenario(
	declared scenarioContract,
	fixture p14MemoryOperationFixture,
) (preparedP14Scenario, error) {
	if err := validateP14MemoryOperationFixtureShape(fixture); err != nil {
		return preparedP14Scenario{}, err
	}
	policy, err := p14MemoryOperationPolicyByBuilder(declared.RequestBuilder)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	fixtureCase, err := p14MemoryOperationFixtureCaseByBuilder(
		fixture.Cases,
		declared.RequestBuilder,
	)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	return buildP14MemoryOperationScenarioCase(
		declared,
		fixture.SelectedProjectRoot,
		fixture.SelectedProjectBasisDigest,
		fixture.HomeTemplateRoot,
		fixture.HomeTemplateDigest,
		fixtureCase,
		policy,
	)
}

func buildP14MemoryOperationScenarioCase(
	declared scenarioContract,
	selectedProjectRoot string,
	selectedProjectBasisDigest string,
	homeTemplateRoot string,
	homeTemplateDigest string,
	fixtureCase p14MemoryOperationFixtureCase,
	policy p14MemoryOperationPolicy,
) (preparedP14Scenario, error) {
	if err := validateP14MemoryOperationContract(declared, policy); err != nil {
		return preparedP14Scenario{}, err
	}
	if err := validateP14MemoryOperationCase(
		fixtureCase,
		policy,
		p14MemoryReadBasis{},
		false,
	); err != nil {
		return preparedP14Scenario{}, err
	}
	expected := p14MemoryOperationExpected{
		Verdict:            policy.Verdict,
		AdmissionResult:    policy.AdmissionResult,
		ConflictResult:     policy.ConflictResult,
		BoundaryErrorCode:  policy.BoundaryErrorCode,
		BoundaryErrorPath:  policy.BoundaryErrorPath,
		CommitCount:        policy.CommitCount,
		GraphRevisionDelta: policy.GraphRevisionDelta,
		AuthorityGranted:   false,
		Assertions:         slices.Clone(policy.Assertions),
	}
	semantic := p14MemoryOperationSemanticRequest{
		Schema:                       p14MemoryOperationSemanticSchema,
		ScenarioID:                   fixtureCase.ScenarioID,
		Protocol:                     policy.Protocol,
		InstalledCLIFixtureIsolation: p14MemoryOperationFixtureIsolation,
		LiveMCPExecutionContext:      p14MemoryOperationLiveMCPExecution,
		SelectedProjectRoot:          selectedProjectRoot,
		SelectedProjectBasisDigest:   selectedProjectBasisDigest,
		HomeTemplateRoot:             homeTemplateRoot,
		HomeTemplateDigest:           homeTemplateDigest,
		Steps:                        cloneP14MemoryOperationSteps(fixtureCase.Steps),
		Expected:                     expected,
	}
	semanticBytes, err := marshalP14CanonicalJSON(semantic)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	semanticDigest := p14Digest(semanticBytes)
	requests, err := buildP14MemoryOperationSurfaceRequests(
		declared,
		semantic,
		semanticDigest,
	)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	normalized := p14MemoryOperationNormalizedOutput{
		Schema:     p14MemoryOperationOutputSchema,
		ScenarioID: fixtureCase.ScenarioID,
		Expected:   expected,
	}
	normalizedBytes, err := marshalP14CanonicalJSON(normalized)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	localOracle := p14MemoryOperationLocalOracle{
		Schema:                p14MemoryOperationLocalOracleSchema,
		SemanticRequestDigest: semanticDigest,
		ExpectedResultDigest:  p14Digest(normalizedBytes),
		LocalOracleTests:      slices.Clone(declared.LocalOracleTests),
	}
	localOracleBytes, err := marshalP14CanonicalJSON(localOracle)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	return preparedP14Scenario{
		ID:                       declared.ID,
		SemanticRequestCanonical: string(semanticBytes),
		SemanticRequestDigest:    semanticDigest,
		Requests:                 requests,
		Oracle: preparedP14Oracle{
			Kind:                    declared.OracleKind,
			NormalizationID:         p14MemoryOperationNormalizationID,
			ExpectedResultDigest:    p14Digest(normalizedBytes),
			ExpectedEffect:          declared.ExpectedEffect,
			LocalOracleOutputDigest: p14Digest(localOracleBytes),
		},
	}, nil
}

func validateP14MemoryOperationContract(
	declared scenarioContract,
	policy p14MemoryOperationPolicy,
) error {
	if declared.ID != policy.ScenarioID ||
		declared.RequestBuilder != policy.BuilderID ||
		!slices.Equal(declared.Surfaces, []string{"installed_cli", "live_mcp"}) ||
		declared.OracleKind != "normalized_digest" ||
		!slices.Contains(declared.RequiredBindings, "golden_memory_fixture") {
		return fmt.Errorf("P14 memory-operation contract differs for %q", policy.BuilderID)
	}
	return nil
}

func buildP14MemoryOperationSurfaceRequests(
	declared scenarioContract,
	semantic p14MemoryOperationSemanticRequest,
	semanticDigest string,
) ([]preparedP14Request, error) {
	cliBytes, err := buildP14MemoryOperationCLISurface(semantic, semanticDigest)
	if err != nil {
		return nil, err
	}
	mcpBytes, err := buildP14MemoryOperationMCPSurface(semantic, semanticDigest)
	if err != nil {
		return nil, err
	}
	payloads := map[string]struct {
		Encoding string
		Payload  []byte
	}{
		"installed_cli": {Encoding: "argv_json", Payload: cliBytes},
		"live_mcp":      {Encoding: "canonical_json", Payload: mcpBytes},
	}
	requests := make([]preparedP14Request, 0, len(declared.Surfaces))
	for _, surface := range declared.Surfaces {
		payload, present := payloads[surface]
		if !present {
			return nil, fmt.Errorf("P14 memory-operation surface %q is unsupported", surface)
		}
		requests = append(requests, preparedP14Request{
			Surface:               surface,
			Builder:               declared.RequestBuilder,
			Encoding:              payload.Encoding,
			CanonicalPayload:      string(payload.Payload),
			PayloadDigest:         p14Digest(payload.Payload),
			SemanticRequestDigest: semanticDigest,
		})
	}
	return requests, nil
}

func buildP14MemoryOperationCLISurface(
	semantic p14MemoryOperationSemanticRequest,
	semanticDigest string,
) ([]byte, error) {
	calls := make([]p14MemoryOperationCLICall, 0, len(semantic.Steps))
	for _, step := range semantic.Steps {
		action, err := p14MemoryOperationRequestAction(step.Request)
		if err != nil {
			return nil, err
		}
		canonical, err := canonicalP14MemoryOperationRequest(step.Request)
		if err != nil {
			return nil, err
		}
		calls = append(calls, p14MemoryOperationCLICall{
			ID:            step.ID,
			ParallelGroup: step.ParallelGroup,
			Argv:          []string{"memory", action, "--input-file", "-"},
			Stdin:         string(canonical),
		})
	}
	payload := p14MemoryOperationCLISurface{
		Schema:                     p14MemoryOperationCLISchema,
		SemanticRequestDigest:      semanticDigest,
		FixtureIsolation:           semantic.InstalledCLIFixtureIsolation,
		SelectedProjectRoot:        semantic.SelectedProjectRoot,
		SelectedProjectBasisDigest: semantic.SelectedProjectBasisDigest,
		HomeTemplateRoot:           semantic.HomeTemplateRoot,
		HomeTemplateDigest:         semantic.HomeTemplateDigest,
		Calls:                      calls,
	}
	return marshalP14CanonicalJSON(payload)
}

func buildP14MemoryOperationMCPSurface(
	semantic p14MemoryOperationSemanticRequest,
	semanticDigest string,
) ([]byte, error) {
	policy, err := p14MemoryOperationPolicyByBuilder(
		p14MemoryOperationBuilderForScenario(semantic.ScenarioID),
	)
	if err != nil {
		return nil, err
	}
	calls := make([]p14MemoryOperationMCPCall, 0, len(semantic.Steps))
	for _, step := range semantic.Steps {
		action, err := p14MemoryOperationRequestAction(step.Request)
		if err != nil {
			return nil, err
		}
		args, err := p14MemoryOperationRequestMap(step.Request)
		if err != nil {
			return nil, err
		}
		args, err = rebaseP14MemoryOperationLiveMCPArgs(
			semantic.ScenarioID,
			args,
		)
		if err != nil {
			return nil, err
		}
		tool := "haft_memory"
		if action == typedmemorywire.ActionNeighborhood {
			tool = "haft_query"
			args, err = p14MemoryQueryMCPEnvelope(
				args,
				typedmemorywire.ActionNeighborhood,
			)
			if err != nil {
				return nil, err
			}
		} else {
			args = map[string]any{
				"request": args,
			}
		}
		calls = append(calls, p14MemoryOperationMCPCall{
			ID:            step.ID,
			ParallelGroup: step.ParallelGroup,
			Tool:          tool,
			Args:          args,
		})
	}
	payload := p14MemoryOperationMCPSurface{
		Schema:                p14MemoryOperationMCPSchema,
		SemanticRequestDigest: semanticDigest,
		ExecutionContext:      semantic.LiveMCPExecutionContext,
		RequiredPredecessors:  slices.Clone(policy.LiveMCPPredecessors),
		Calls:                 calls,
	}
	return marshalP14CanonicalJSON(payload)
}

func p14MemoryOperationBuilderForScenario(
	scenarioID string,
) string {
	for _, policy := range p14MemoryOperationPolicies() {
		if policy.ScenarioID == scenarioID {
			return policy.BuilderID
		}
	}
	return ""
}

func rebaseP14MemoryOperationLiveMCPArgs(
	scenarioID string,
	args map[string]any,
) (map[string]any, error) {
	if scenarioID != "concurrency_idempotency" {
		return args, nil
	}
	basis, valid := args["basis"].(map[string]any)
	if !valid {
		return nil, fmt.Errorf("P14 live MCP concurrency request has no basis")
	}
	revision, valid := p14FPFJSONPositiveInt(basis["graph_revision"])
	if !valid {
		return nil, fmt.Errorf("P14 live MCP concurrency revision is invalid")
	}
	basis["graph_revision"] = json.Number(strconv.FormatInt(revision+1, 10))
	return args, nil
}

func validateP14MemoryOperationPreparedScenario(
	declared scenarioContract,
	scenario preparedP14Scenario,
) error {
	policy, err := p14MemoryOperationPolicyByBuilder(declared.RequestBuilder)
	if err != nil {
		return err
	}
	semantic, err := decodeP14MemoryOperationSemantic(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return err
	}
	fixtureCase := p14MemoryOperationFixtureCase{
		ScenarioID: semantic.ScenarioID,
		BuilderID:  policy.BuilderID,
		Steps:      cloneP14MemoryOperationSteps(semantic.Steps),
	}
	expected, err := buildP14MemoryOperationScenarioCase(
		declared,
		semantic.SelectedProjectRoot,
		semantic.SelectedProjectBasisDigest,
		semantic.HomeTemplateRoot,
		semantic.HomeTemplateDigest,
		fixtureCase,
		policy,
	)
	if err != nil {
		return err
	}
	expectedBytes, err := marshalP14CanonicalJSON(expected)
	if err != nil {
		return err
	}
	actualBytes, err := marshalP14CanonicalJSON(scenario)
	if err != nil {
		return err
	}
	if !bytes.Equal(actualBytes, expectedBytes) {
		return fmt.Errorf("P14 memory-operation prepared scenario differs")
	}
	return nil
}

func validateP14MemoryOperationFixtureShape(
	fixture p14MemoryOperationFixture,
) error {
	policies := p14MemoryOperationPolicies()
	if !filepath.IsAbs(fixture.SelectedProjectRoot) ||
		!validP14Digest(fixture.SelectedProjectBasisDigest) ||
		!filepath.IsAbs(fixture.HomeTemplateRoot) ||
		!validP14Digest(fixture.HomeTemplateDigest) ||
		fixture.SelectedProjectRoot == fixture.HomeTemplateRoot ||
		len(fixture.Cases) != len(policies) {
		return fmt.Errorf("P14 memory-operation fixture shape differs")
	}
	basis, err := p14MemoryOperationFixtureBasis(fixture.Cases)
	if err != nil {
		return err
	}
	for index, fixtureCase := range fixture.Cases {
		if err := validateP14MemoryOperationCase(
			fixtureCase,
			policies[index],
			basis,
			true,
		); err != nil {
			return err
		}
	}
	return nil
}

func validateP14MemoryOperationCase(
	fixtureCase p14MemoryOperationFixtureCase,
	policy p14MemoryOperationPolicy,
	basis p14MemoryReadBasis,
	checkBasis bool,
) error {
	if fixtureCase.ScenarioID != policy.ScenarioID ||
		fixtureCase.BuilderID != policy.BuilderID ||
		len(fixtureCase.Steps) != len(policy.Steps) {
		return fmt.Errorf("P14 memory-operation case differs for %q", policy.BuilderID)
	}
	for index, step := range fixtureCase.Steps {
		stepPolicy := policy.Steps[index]
		if step.ID != stepPolicy.ID || step.ParallelGroup != stepPolicy.ParallelGroup {
			return fmt.Errorf("P14 memory-operation step order differs for %q", policy.BuilderID)
		}
		action, err := p14MemoryOperationRequestAction(step.Request)
		if err != nil {
			return err
		}
		if action != stepPolicy.Action {
			return fmt.Errorf("P14 memory-operation step %q action differs", step.ID)
		}
		if err := validateP14MemoryOperationStepRequest(
			policy,
			step,
			basis,
			checkBasis,
		); err != nil {
			return err
		}
	}
	return validateP14MemoryOperationStepRelations(fixtureCase, policy)
}

func validateP14MemoryOperationStepRequest(
	policy p14MemoryOperationPolicy,
	step p14MemoryOperationStep,
	basis p14MemoryReadBasis,
	checkBasis bool,
) error {
	if policy.BuilderID == "memory.authority-rejection.v1" {
		return validateP14MemoryAuthorityRejection(step.Request, basis, checkBasis)
	}
	action, err := p14MemoryOperationRequestAction(step.Request)
	if err != nil {
		return err
	}
	requestedBasis := p14MemoryReadBasis{}
	switch action {
	case typedmemorywire.ActionValidate:
		request, decodeErr := typedmemorywire.DecodeValidateRequest(step.Request)
		if decodeErr != nil {
			return decodeErr
		}
		if request.ContractVersion() != typedmemorywire.ContractVersionV2 {
			return fmt.Errorf("P14 validation request is not current v2")
		}
		requestedBasis, err = p14MemoryOperationBasisFromSelector(request.Basis())
	case typedmemorywire.ActionAdmit:
		request, decodeErr := typedmemorywire.DecodeAdmitRequest(step.Request)
		if decodeErr != nil {
			return decodeErr
		}
		if request.ContractVersion() != typedmemorywire.ContractVersionV2 ||
			request.AuthorityClass() != typedmemorywire.AuthorityClassNonBindingSemanticAssertion {
			return fmt.Errorf("P14 admission request broadens contract or authority")
		}
		requestedBasis = p14MemoryOperationBasisFromExact(request.Basis())
	case typedmemorywire.ActionNeighborhood:
		request, decodeErr := typedmemorywire.DecodeNeighborhoodReadRequest(step.Request)
		if decodeErr != nil {
			return decodeErr
		}
		requestedBasis, err = p14MemoryOperationBasisFromSelector(request.Basis())
	default:
		return fmt.Errorf("P14 memory-operation action %q is unsupported", action)
	}
	if err != nil {
		return err
	}
	if !checkBasis {
		return nil
	}
	expected := basis
	if step.ID == "reread" {
		expected.GraphRevision++
	}
	if requestedBasis != expected {
		return fmt.Errorf("P14 memory-operation step %q basis differs", step.ID)
	}
	return nil
}

func validateP14MemoryAuthorityRejection(
	raw json.RawMessage,
	basis p14MemoryReadBasis,
	checkBasis bool,
) error {
	_, err := typedmemorywire.DecodeAdmitRequest(raw)
	decodeError := &typedmemorywire.DecodeError{}
	if !errors.As(err, &decodeError) ||
		decodeError.Code() != typedmemorywire.ErrorInvalidContract ||
		decodeError.Path() != "$.authority_class" {
		return fmt.Errorf("P14 authority request is not rejected at the authority boundary")
	}
	request, err := p14MemoryOperationRequestMap(raw)
	if err != nil {
		return err
	}
	if request["contract_version"] != typedmemorywire.ContractVersionV2 ||
		request["action"] != typedmemorywire.ActionAdmit ||
		request["authority_class"] == typedmemorywire.AuthorityClassNonBindingSemanticAssertion {
		return fmt.Errorf("P14 authority rejection request does not exercise forbidden authority")
	}
	if !checkBasis {
		return nil
	}
	requested, err := p14MemoryOperationBasisFromMap(request["basis"])
	if err != nil {
		return err
	}
	if requested != basis {
		return fmt.Errorf("P14 authority rejection basis differs")
	}
	return nil
}

func validateP14MemoryOperationStepRelations(
	fixtureCase p14MemoryOperationFixtureCase,
	policy p14MemoryOperationPolicy,
) error {
	steps := make(map[string]p14MemoryOperationStep, len(fixtureCase.Steps))
	for _, step := range fixtureCase.Steps {
		steps[step.ID] = step
	}
	if policy.BuilderID == "memory.admit-valid.v2" {
		validateCore, err := p14MemoryOperationSemanticCoreDigest(steps["validate"].Request)
		if err != nil {
			return err
		}
		admitCore, err := p14MemoryOperationSemanticCoreDigest(steps["admit"].Request)
		if err != nil {
			return err
		}
		admitRequest, err := canonicalP14MemoryOperationRequest(steps["admit"].Request)
		if err != nil {
			return err
		}
		replayRequest, err := canonicalP14MemoryOperationRequest(steps["replay"].Request)
		if err != nil {
			return err
		}
		if validateCore != admitCore || !bytes.Equal(admitRequest, replayRequest) {
			return fmt.Errorf("P14 positive write does not validate and replay one exact request")
		}
	}
	if policy.BuilderID == "memory.concurrent-replay.v1" {
		writerA, err := canonicalP14MemoryOperationRequest(steps["writer_a"].Request)
		if err != nil {
			return err
		}
		writerB, err := canonicalP14MemoryOperationRequest(steps["writer_b"].Request)
		if err != nil {
			return err
		}
		replay, err := canonicalP14MemoryOperationRequest(steps["replay"].Request)
		if err != nil {
			return err
		}
		if !bytes.Equal(writerA, writerB) || !bytes.Equal(writerA, replay) {
			return fmt.Errorf("P14 concurrent writers and replay do not share exact request bytes")
		}
		primary, err := typedmemorywire.DecodeAdmitRequest(writerA)
		if err != nil {
			return err
		}
		conflict, err := typedmemorywire.DecodeAdmitRequest(steps["conflict"].Request)
		if err != nil {
			return err
		}
		primaryCore, err := p14MemoryOperationSemanticCoreDigest(writerA)
		if err != nil {
			return err
		}
		conflictCore, err := p14MemoryOperationSemanticCoreDigest(steps["conflict"].Request)
		if err != nil {
			return err
		}
		if primary.IdempotencyKey() != conflict.IdempotencyKey() ||
			primary.Basis() != conflict.Basis() || primaryCore == conflictCore {
			return fmt.Errorf("P14 concurrency conflict does not reuse one key with different bytes")
		}
	}
	return nil
}

func p14MemoryOperationFixtureBasis(
	cases []p14MemoryOperationFixtureCase,
) (p14MemoryReadBasis, error) {
	if len(cases) == 0 || len(cases[0].Steps) == 0 {
		return p14MemoryReadBasis{}, fmt.Errorf("P14 memory-operation fixture has no basis source")
	}
	request, err := typedmemorywire.DecodeValidateRequest(cases[0].Steps[0].Request)
	if err != nil {
		return p14MemoryReadBasis{}, err
	}
	return p14MemoryOperationBasisFromSelector(request.Basis())
}

func p14MemoryOperationBasisFromSelector(
	selector typedmemorywire.BasisSelector,
) (p14MemoryReadBasis, error) {
	exact, valid := selector.(typedmemorywire.ExactProjectSelector)
	if !valid {
		return p14MemoryReadBasis{}, fmt.Errorf("P14 memory operation requires exact_project")
	}
	return p14MemoryOperationBasisFromExact(exact), nil
}

func p14MemoryOperationBasisFromExact(
	exact typedmemorywire.ExactProjectSelector,
) p14MemoryReadBasis {
	return p14MemoryReadBasis{
		Kind:          "exact_project",
		TypeEnvDigest: exact.RequestedTypeEnvDigest().String(),
		GraphRevision: int64(exact.RequestedGraphRevision().Value()),
	}
}

func p14MemoryOperationBasisFromMap(value any) (p14MemoryReadBasis, error) {
	basis, valid := value.(map[string]any)
	if !valid || basis["kind"] != "exact_project" {
		return p14MemoryReadBasis{}, fmt.Errorf("P14 memory-operation map has no exact basis")
	}
	digest, valid := basis["type_env_digest"].(string)
	if !valid || !validP14Digest(digest) {
		return p14MemoryReadBasis{}, fmt.Errorf("P14 memory-operation map TypeEnv digest is invalid")
	}
	revision, valid := p14FPFJSONPositiveInt(basis["graph_revision"])
	if !valid {
		return p14MemoryReadBasis{}, fmt.Errorf("P14 memory-operation map graph revision is invalid")
	}
	return p14MemoryReadBasis{
		Kind:          "exact_project",
		TypeEnvDigest: digest,
		GraphRevision: revision,
	}, nil
}

func p14MemoryOperationSemanticCoreDigest(raw json.RawMessage) (string, error) {
	request, err := p14MemoryOperationRequestMap(raw)
	if err != nil {
		return "", err
	}
	core := map[string]any{
		"contract_version": request["contract_version"],
		"basis":            request["basis"],
		"change_set":       request["change_set"],
	}
	canonical, err := marshalP14CanonicalJSON(core)
	if err != nil {
		return "", err
	}
	return p14Digest(canonical), nil
}

func p14MemoryOperationRequestAction(raw json.RawMessage) (string, error) {
	request, err := p14MemoryOperationRequestMap(raw)
	if err != nil {
		return "", err
	}
	action, valid := request["action"].(string)
	if !valid || action == "" {
		return "", fmt.Errorf("P14 memory-operation request has no action")
	}
	return action, nil
}

func p14MemoryOperationRequestMap(raw json.RawMessage) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	request := make(map[string]any)
	if err := decoder.Decode(&request); err != nil {
		return nil, fmt.Errorf("decode P14 memory-operation request: %w", err)
	}
	trailing := json.RawMessage{}
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		return nil, fmt.Errorf("P14 memory-operation request has trailing JSON")
	}
	return request, nil
}

func canonicalP14MemoryOperationRequest(raw json.RawMessage) ([]byte, error) {
	request, err := p14MemoryOperationRequestMap(raw)
	if err != nil {
		return nil, err
	}
	return marshalP14CanonicalJSON(request)
}

func decodeP14MemoryOperationSemantic(
	raw []byte,
) (p14MemoryOperationSemanticRequest, error) {
	semantic := p14MemoryOperationSemanticRequest{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&semantic); err != nil {
		return p14MemoryOperationSemanticRequest{}, fmt.Errorf(
			"decode P14 memory-operation semantic request: %w",
			err,
		)
	}
	trailing := json.RawMessage{}
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		return p14MemoryOperationSemanticRequest{}, fmt.Errorf(
			"P14 memory-operation semantic request has trailing JSON",
		)
	}
	if semantic.Schema != p14MemoryOperationSemanticSchema ||
		semantic.InstalledCLIFixtureIsolation !=
			p14MemoryOperationFixtureIsolation ||
		semantic.LiveMCPExecutionContext !=
			p14MemoryOperationLiveMCPExecution ||
		!filepath.IsAbs(semantic.SelectedProjectRoot) ||
		!validP14Digest(semantic.SelectedProjectBasisDigest) ||
		!filepath.IsAbs(semantic.HomeTemplateRoot) ||
		!validP14Digest(semantic.HomeTemplateDigest) ||
		semantic.SelectedProjectRoot == semantic.HomeTemplateRoot {
		return p14MemoryOperationSemanticRequest{}, fmt.Errorf(
			"P14 memory-operation semantic envelope differs",
		)
	}
	return semantic, nil
}

func p14MemoryOperationPolicyByBuilder(
	builderID string,
) (p14MemoryOperationPolicy, error) {
	for _, policy := range p14MemoryOperationPolicies() {
		if policy.BuilderID == builderID {
			return policy, nil
		}
	}
	return p14MemoryOperationPolicy{}, fmt.Errorf(
		"P14 memory-operation builder %q is unsupported",
		builderID,
	)
}

func p14MemoryOperationFixtureCaseByBuilder(
	cases []p14MemoryOperationFixtureCase,
	builderID string,
) (p14MemoryOperationFixtureCase, error) {
	for _, fixtureCase := range cases {
		if fixtureCase.BuilderID == builderID {
			return fixtureCase, nil
		}
	}
	return p14MemoryOperationFixtureCase{}, fmt.Errorf(
		"P14 memory-operation fixture has no case for %q",
		builderID,
	)
}

func cloneP14MemoryOperationSteps(
	steps []p14MemoryOperationStep,
) []p14MemoryOperationStep {
	cloned := make([]p14MemoryOperationStep, 0, len(steps))
	for _, step := range steps {
		cloned = append(cloned, p14MemoryOperationStep{
			ID:            step.ID,
			ParallelGroup: step.ParallelGroup,
			Request:       slices.Clone(step.Request),
		})
	}
	return cloned
}

func syntheticP14MemoryOperationFixture(
	fixture p14MemoryReadFixture,
) p14MemoryOperationFixture {
	basis := fixture.Basis
	positiveDeclaration := syntheticP14MemoryEntityDeclaration(
		"positive-write",
		fixture.BoundedContext,
		"Positive typed write",
	)
	positiveValidate := syntheticP14MemoryOperationWriteRequest(
		typedmemorywire.ActionValidate,
		basis,
		positiveDeclaration,
		"",
	)
	positiveAdmit := syntheticP14MemoryOperationWriteRequest(
		typedmemorywire.ActionAdmit,
		basis,
		positiveDeclaration,
		"positive-write-key",
	)
	invalidDeclaration := syntheticP14ExistingEntityContradiction(
		fixture.EntityRef.ReferenceID,
		fixture.BoundedContext,
	)
	invalid := syntheticP14MemoryOperationWriteRequest(
		typedmemorywire.ActionValidate,
		basis,
		invalidDeclaration,
		"",
	)
	underdeterminedDeclaration := syntheticP14MemoryEntityDeclaration(
		"missing-basis",
		"p14-unadmitted-bounded-context",
		"Missing bounded-context basis",
	)
	underdetermined := syntheticP14MemoryOperationWriteRequest(
		typedmemorywire.ActionValidate,
		basis,
		underdeterminedDeclaration,
		"",
	)
	authority := syntheticP14MemoryOperationAuthorityRequest(basis)
	concurrentDeclaration := syntheticP14MemoryEntityDeclaration(
		"concurrent-write",
		fixture.BoundedContext,
		"Concurrent write",
	)
	concurrent := syntheticP14MemoryOperationWriteRequest(
		typedmemorywire.ActionAdmit,
		basis,
		concurrentDeclaration,
		"concurrent-write-key",
	)
	conflictingDeclaration := syntheticP14MemoryEntityDeclaration(
		"concurrent-write",
		fixture.BoundedContext,
		"Conflicting concurrent write",
	)
	conflict := syntheticP14MemoryOperationWriteRequest(
		typedmemorywire.ActionAdmit,
		basis,
		conflictingDeclaration,
		"concurrent-write-key",
	)
	reread := syntheticP14MemoryOperationRereadRequest(fixture)
	return p14MemoryOperationFixture{
		SelectedProjectRoot:        "/synthetic/p14/memory/selected-project",
		SelectedProjectBasisDigest: p14TestDigest("memory-selected-project-basis"),
		HomeTemplateRoot:           "/synthetic/p14/memory/home-template",
		HomeTemplateDigest:         p14TestDigest("memory-home-template"),
		Cases: []p14MemoryOperationFixtureCase{
			{
				ScenarioID: "positive_typed_write",
				BuilderID:  "memory.admit-valid.v2",
				Steps: []p14MemoryOperationStep{
					{ID: "validate", Request: positiveValidate},
					{ID: "admit", Request: positiveAdmit},
					{ID: "replay", Request: slices.Clone(positiveAdmit)},
					{ID: "reread", Request: reread},
				},
			},
			{
				ScenarioID: "invalid",
				BuilderID:  "memory.validate-invalid.v2",
				Steps: []p14MemoryOperationStep{
					{ID: "validate", Request: invalid},
				},
			},
			{
				ScenarioID: "underdetermined",
				BuilderID:  "memory.validate-underdetermined.v2",
				Steps: []p14MemoryOperationStep{
					{ID: "validate", Request: underdetermined},
				},
			},
			{
				ScenarioID: "authority_rejection",
				BuilderID:  "memory.authority-rejection.v1",
				Steps: []p14MemoryOperationStep{
					{ID: "reject", Request: authority},
				},
			},
			{
				ScenarioID: "concurrency_idempotency",
				BuilderID:  "memory.concurrent-replay.v1",
				Steps: []p14MemoryOperationStep{
					{ID: "writer_a", ParallelGroup: "same_request", Request: concurrent},
					{ID: "writer_b", ParallelGroup: "same_request", Request: slices.Clone(concurrent)},
					{ID: "replay", Request: slices.Clone(concurrent)},
					{ID: "conflict", Request: conflict},
				},
			},
		},
	}
}

type p14SyntheticMemoryEntityDeclaration struct {
	EntityID             string
	LocalRef             string
	Context              string
	Label                string
	ChangeProvenanceRef  string
	RequestProvenanceRef string
}

func syntheticP14MemoryEntityDeclaration(
	token string,
	context string,
	label string,
) p14SyntheticMemoryEntityDeclaration {
	return p14SyntheticMemoryEntityDeclaration{
		EntityID:             "entity:" + token,
		LocalRef:             "local:" + token,
		Context:              context,
		Label:                label,
		ChangeProvenanceRef:  "provenance:" + token + "-change",
		RequestProvenanceRef: "provenance:" + token + "-request",
	}
}

func syntheticP14ExistingEntityContradiction(
	existingEntityID string,
	context string,
) p14SyntheticMemoryEntityDeclaration {
	return p14SyntheticMemoryEntityDeclaration{
		EntityID:             existingEntityID,
		LocalRef:             "local:invalid-existing-reference",
		Context:              context,
		Label:                "Contradictory redeclaration of existing entity",
		ChangeProvenanceRef:  "provenance:invalid-existing-reference-change",
		RequestProvenanceRef: "provenance:invalid-existing-reference-request",
	}
}

func syntheticP14MemoryOperationWriteRequest(
	action string,
	basis p14MemoryReadBasis,
	declaration p14SyntheticMemoryEntityDeclaration,
	idempotencyKey string,
) json.RawMessage {
	admissionFields := ""
	if action == typedmemorywire.ActionAdmit {
		admissionFields = fmt.Sprintf(
			`,"authority_class":"%s","idempotency_key":%q,"request_provenance_ref":%q`,
			typedmemorywire.AuthorityClassNonBindingSemanticAssertion,
			idempotencyKey,
			declaration.RequestProvenanceRef,
		)
	}
	return json.RawMessage(fmt.Sprintf(
		`{"contract_version":"%s","action":"%s","basis":{"kind":"exact_project","type_env_digest":"%s","graph_revision":%d}%s,"change_set":{"changes":[{"kind":"declare_entity","entity_id":%q,"local_ref":%q,"context":%q,"label":%q,"provenance":%q}]}}`,
		typedmemorywire.ContractVersionV2,
		action,
		basis.TypeEnvDigest,
		basis.GraphRevision,
		admissionFields,
		declaration.EntityID,
		declaration.LocalRef,
		declaration.Context,
		declaration.Label,
		declaration.ChangeProvenanceRef,
	))
}

func syntheticP14MemoryOperationAuthorityRequest(
	basis p14MemoryReadBasis,
) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(
		`{"contract_version":"%s","action":"admit","basis":{"kind":"exact_project","type_env_digest":"%s","graph_revision":%d},"authority_class":"decision_binding","idempotency_key":"authority-rejection-key","request_provenance_ref":"provenance:authority-rejection","change_set":{"changes":[{"kind":"declare_entity","entity_id":"entity:authority-rejection","local_ref":"local:authority-rejection","context":"haft-project","label":"Authority rejection","provenance":"provenance:authority-rejection-change"}]}}`,
		typedmemorywire.ContractVersionV2,
		basis.TypeEnvDigest,
		basis.GraphRevision,
	))
}

func syntheticP14MemoryOperationRereadRequest(
	fixture p14MemoryReadFixture,
) json.RawMessage {
	quotedFacets := make([]string, 0, len(fixture.View.RequestedFacets))
	for _, facet := range fixture.View.RequestedFacets {
		quotedFacets = append(quotedFacets, strconv.Quote(facet))
	}
	return json.RawMessage(fmt.Sprintf(
		`{"contract_version":%q,"action":"neighborhood","basis":{"kind":"exact_project","type_env_digest":%q,"graph_revision":%d},"bounded_context_ref":%q,"entity_ref":{"ref_kind_id":%q,"reference_id":"entity:positive-write"},"view":{"projection_profile_ref":%q,"requested_facets":[%s],"detail":%q,"include_history":%t},"read_budget":{"max_facets":%d,"max_items_per_facet":%d,"max_relation_paths_per_item":%d,"max_carrier_excerpt_characters":%d,"max_provenance_depth":%d}}`,
		fixture.ContractVersion,
		fixture.Basis.TypeEnvDigest,
		fixture.Basis.GraphRevision+1,
		fixture.BoundedContext,
		fixture.EntityRef.RefKindID,
		fixture.View.ProjectionProfileRef,
		strings.Join(quotedFacets, ","),
		fixture.View.Detail,
		fixture.View.IncludeHistory,
		fixture.ReadBudget.MaxFacets,
		fixture.ReadBudget.MaxItemsPerFacet,
		fixture.ReadBudget.MaxRelationPathsPerItem,
		fixture.ReadBudget.MaxCarrierExcerptCharacters,
		fixture.ReadBudget.MaxProvenanceDepth,
	))
}

func TestP14MemoryOperationBuildersCloseTypedWriteAndRejectionProtocols(
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
	fixture := syntheticP14MemoryReadFixture()
	for _, builderID := range p14MemoryOperationBuilderIDs {
		declared, err := findP14ScenarioContractByBuilder(contract, builderID)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(declared.ID, func(t *testing.T) {
			scenario, err := buildP14MemoryOperationScenario(
				declared,
				fixture.Operations,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateP14MemoryOperationPreparedScenario(
				declared,
				scenario,
			); err != nil {
				t.Fatal(err)
			}
			if len(scenario.Requests) != 2 ||
				scenario.Requests[0].Surface != "installed_cli" ||
				scenario.Requests[1].Surface != "live_mcp" {
				t.Fatalf("memory-operation surfaces = %#v", scenario.Requests)
			}
			mcpSurface := p14MemoryOperationMCPSurface{}
			if err := decodeP14StrictCompactJSON(
				scenario.Requests[1].CanonicalPayload,
				&mcpSurface,
				"memory-operation live MCP surface",
			); err != nil {
				t.Fatal(err)
			}
			if mcpSurface.ExecutionContext !=
				p14MemoryOperationLiveMCPExecution ||
				bytes.Contains(
					[]byte(scenario.Requests[1].CanonicalPayload),
					[]byte(`"project_template_root"`),
				) {
				t.Fatal("live MCP surface falsely claims fixture execution")
			}
			if scenario.ID == "concurrency_idempotency" {
				if !slices.Equal(
					mcpSurface.RequiredPredecessors,
					[]string{"positive_typed_write"},
				) {
					t.Fatal("live MCP concurrency lost its ordered predecessor")
				}
				request := mcpSurface.Calls[0].Args["request"].(map[string]any)
				basis := request["basis"].(map[string]any)
				revision, valid := basis["graph_revision"].(float64)
				if !valid ||
					int64(revision) != fixture.Basis.GraphRevision+1 {
					t.Fatal("live MCP concurrency did not bind post-write basis")
				}
			}
			for _, call := range mcpSurface.Calls {
				if call.Tool == "haft_query" {
					if _, wrapped := call.Args["request"]; wrapped {
						t.Fatal("haft_query memory read was wrapped as raw memory")
					}
					memoryRequest, wrapped :=
						call.Args["memory_request"].(map[string]any)
					if !wrapped ||
						len(call.Args) != 2 ||
						call.Args["action"] != "memory" ||
						memoryRequest["mode"] !=
							typedmemorywire.ActionNeighborhood ||
						memoryRequest["action"] != nil ||
						call.Args["mode"] != nil {
						t.Fatal(
							"haft_query memory read does not use the exact memory_request envelope",
						)
					}
					continue
				}
				request, wrapped := call.Args["request"].(map[string]any)
				if call.Tool != "haft_memory" ||
					!wrapped ||
					len(call.Args) != 1 ||
					request["action"] == nil ||
					call.Args["action"] != nil {
					t.Fatal("haft_memory call does not use the exact request envelope")
				}
			}
			tampered := scenario
			tampered.Requests = slices.Clone(scenario.Requests)
			tampered.Requests[0].CanonicalPayload = `{}`
			tampered.Requests[0].PayloadDigest = p14Digest([]byte(`{}`))
			if err := validateP14MemoryOperationPreparedScenario(
				declared,
				tampered,
			); err == nil {
				t.Fatal("memory-operation validator accepted divergent CLI protocol")
			}
		})
	}
}

func TestP14MemoryOperationFixtureRejectsAuthorityAndReplayBroadening(
	t *testing.T,
) {
	fixture := syntheticP14MemoryReadFixture().Operations
	tamperedAuthority := fixture
	tamperedAuthority.Cases = slices.Clone(fixture.Cases)
	tamperedAuthority.Cases[3].Steps = cloneP14MemoryOperationSteps(
		fixture.Cases[3].Steps,
	)
	tamperedAuthority.Cases[3].Steps[0].Request = bytes.Replace(
		tamperedAuthority.Cases[3].Steps[0].Request,
		[]byte(`"decision_binding"`),
		[]byte(`"non_binding_semantic_assertion"`),
		1,
	)
	if err := validateP14MemoryOperationFixtureShape(tamperedAuthority); err == nil {
		t.Fatal("P14 operation fixture accepted a non-rejected authority case")
	}

	tamperedReplay := fixture
	tamperedReplay.Cases = slices.Clone(fixture.Cases)
	tamperedReplay.Cases[0].Steps = cloneP14MemoryOperationSteps(
		fixture.Cases[0].Steps,
	)
	tamperedReplay.Cases[0].Steps[2].Request = bytes.Replace(
		tamperedReplay.Cases[0].Steps[2].Request,
		[]byte(`"Positive typed write"`),
		[]byte(`"Different replay"`),
		1,
	)
	if err := validateP14MemoryOperationFixtureShape(tamperedReplay); err == nil {
		t.Fatal("P14 operation fixture accepted changed replay bytes")
	}
}
