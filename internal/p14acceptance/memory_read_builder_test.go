package p14acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	p14MemoryReadFixtureSchema           = "haft.p14.golden-memory-fixture/v3"
	p14MemoryReadSemanticSchema          = "haft.p14.memory-read-semantic/v1"
	p14MemoryReadCLISurfaceSchema        = "haft.p14.memory-read-cli/v1"
	p14MemoryReadMCPSurfaceSchema        = "haft.p14.memory-read-mcp/v1"
	p14MemoryReadNormalizedOutputSchema  = "haft.p14.memory-read-output/v1"
	p14MemoryReadLocalOracleSchema       = "haft.p14.memory-read-local-oracle/v1"
	p14MemoryReadNormalizationID         = "p14.memory-read.semantic-output.v1"
	p14MemoryReadContractVersion         = "haft.memory.v1"
	p14MemoryReadFixtureEnvironmentKey   = "HAFT_P14_MEMORY_READ_FIXTURE"
	p14MemoryReadCandidateEnvironmentKey = "HAFT_P14_MEMORY_READ_CANDIDATE"
	p14MemoryReadProjectEnvironmentKey   = "HAFT_P14_MEMORY_READ_PROJECT_ROOT"
)

var p14MemoryReadBuilderIDs = []string{
	"memory.legacy-read.v1",
	"memory.neighborhood-exact-profile.v1",
	"memory.resolve-unknown.v1",
	"memory.recall-known.v1",
	"memory.neighborhood-facet-matrix.v1",
	"memory.stale-basis-retry.v1",
	"memory.read-affordance.v1",
}

type p14MemoryReadFixture struct {
	Schema          string                    `json:"schema"`
	ContractVersion string                    `json:"contract_version"`
	Basis           p14MemoryReadBasis        `json:"basis"`
	EntityRef       p14MemoryReadEntityRef    `json:"entity_ref"`
	LegacyEntityRef p14MemoryReadEntityRef    `json:"legacy_entity_ref"`
	StaleBasis      p14MemoryReadBasis        `json:"stale_basis"`
	BoundedContext  string                    `json:"bounded_context_ref"`
	View            p14MemoryReadView         `json:"view"`
	ReadBudget      p14MemoryReadBudget       `json:"read_budget"`
	UnknownQuery    string                    `json:"unknown_query"`
	RecallQuery     string                    `json:"recall_query"`
	MaxCandidates   int                       `json:"max_candidates"`
	CandidateBudget p14MemoryCandidateBudget  `json:"candidate_budget"`
	Operations      p14MemoryOperationFixture `json:"operations"`
}

type p14MemoryReadBasis struct {
	Kind          string `json:"kind"`
	TypeEnvDigest string `json:"type_env_digest"`
	GraphRevision int64  `json:"graph_revision"`
}

type p14MemoryReadEntityRef struct {
	RefKindID   string `json:"ref_kind_id"`
	ReferenceID string `json:"reference_id"`
}

type p14MemoryReadView struct {
	ProjectionProfileRef string   `json:"projection_profile_ref"`
	RequestedFacets      []string `json:"requested_facets"`
	Detail               string   `json:"detail"`
	IncludeHistory       bool     `json:"include_history"`
}

type p14MemoryReadBudget struct {
	MaxFacets                   int `json:"max_facets"`
	MaxItemsPerFacet            int `json:"max_items_per_facet"`
	MaxRelationPathsPerItem     int `json:"max_relation_paths_per_item"`
	MaxCarrierExcerptCharacters int `json:"max_carrier_excerpt_characters"`
	MaxProvenanceDepth          int `json:"max_provenance_depth"`
}

type p14MemoryCandidateBudget struct {
	MaxCandidates int `json:"max_candidates"`
}

type p14MemoryReadSemanticRequest struct {
	Schema     string                      `json:"schema"`
	ScenarioID string                      `json:"scenario_id"`
	Cases      []p14MemoryReadSemanticCase `json:"cases"`
}

type p14MemoryReadSemanticCase struct {
	ID       string                      `json:"id"`
	Request  p14MemoryReadCoreRequest    `json:"request"`
	Expected p14MemoryReadExpectedResult `json:"expected"`
}

type p14MemoryReadCoreRequest struct {
	ContractVersion string                    `json:"contract_version"`
	Action          string                    `json:"action"`
	Basis           p14MemoryReadBasis        `json:"basis"`
	Query           string                    `json:"query,omitempty"`
	BoundedContext  string                    `json:"bounded_context_ref,omitempty"`
	MaxCandidates   int                       `json:"max_candidates,omitempty"`
	EntityRef       *p14MemoryReadEntityRef   `json:"entity_ref,omitempty"`
	View            *p14MemoryReadView        `json:"view,omitempty"`
	ReadBudget      *p14MemoryReadBudget      `json:"read_budget,omitempty"`
	CandidateBudget *p14MemoryCandidateBudget `json:"candidate_budget,omitempty"`
}

type p14MemoryReadExpectedResult struct {
	ResultKind    string              `json:"result_kind"`
	RequiredBasis *p14MemoryReadBasis `json:"required_basis,omitempty"`
	AssertionIDs  []string            `json:"assertion_ids"`
}

type p14MemoryReadCLISurface struct {
	Schema                string                     `json:"schema"`
	SemanticRequestDigest string                     `json:"semantic_request_digest"`
	Cases                 []p14MemoryReadCLICallCase `json:"cases"`
}

type p14MemoryReadCLICallCase struct {
	ID    string   `json:"id"`
	Argv  []string `json:"argv"`
	Stdin string   `json:"stdin"`
}

type p14MemoryReadMCPSurface struct {
	Schema                string                     `json:"schema"`
	SemanticRequestDigest string                     `json:"semantic_request_digest"`
	Tool                  string                     `json:"tool"`
	Cases                 []p14MemoryReadMCPCallCase `json:"cases"`
}

type p14MemoryReadMCPCallCase struct {
	ID   string         `json:"id"`
	Args map[string]any `json:"args"`
}

type p14MemoryReadNormalizedOutput struct {
	Schema string                              `json:"schema"`
	Cases  []p14MemoryReadNormalizedCaseOutput `json:"cases"`
}

type p14MemoryReadNormalizedCaseOutput struct {
	ID      string          `json:"id"`
	Outcome string          `json:"outcome"`
	Payload json.RawMessage `json:"payload"`
}

type p14MemoryReadLocalOracle struct {
	Schema                string   `json:"schema"`
	CandidateDigest       string   `json:"candidate_digest"`
	SemanticRequestDigest string   `json:"semantic_request_digest"`
	ExpectedResultDigest  string   `json:"expected_result_digest"`
	CaseOutputDigests     []string `json:"case_output_digests"`
	LocalOracleTests      []string `json:"local_oracle_tests"`
}

type p14MemoryReadPolicy struct {
	BuilderID       string
	Action          string
	ResultKind      string
	AssertionIDs    []string
	UseLegacyEntity bool
	UseStaleBasis   bool
}

type p14MemoryReadExecutor func(
	context.Context,
	string,
	string,
	p14MemoryReadSemanticCase,
	p14MemoryReadCLICallCase,
) (p14FPFProjectionCommandObservation, error)

func p14MemoryReadPolicies() map[string]p14MemoryReadPolicy {
	return map[string]p14MemoryReadPolicy{
		"memory.legacy-read.v1": {
			BuilderID:       "memory.legacy-read.v1",
			Action:          "neighborhood",
			ResultKind:      "exact_neighborhood",
			AssertionIDs:    []string{"legacy_posture_explicit"},
			UseLegacyEntity: true,
		},
		"memory.neighborhood-exact-profile.v1": {
			BuilderID:    "memory.neighborhood-exact-profile.v1",
			Action:       "neighborhood",
			ResultKind:   "exact_neighborhood",
			AssertionIDs: []string{"exact_profile_basis"},
		},
		"memory.resolve-unknown.v1": {
			BuilderID:    "memory.resolve-unknown.v1",
			Action:       "resolve",
			ResultKind:   "known_absent",
			AssertionIDs: []string{"unknown_identity_not_minted"},
		},
		"memory.recall-known.v1": {
			BuilderID:    "memory.recall-known.v1",
			Action:       "recall",
			ResultKind:   "scoped_memory_candidate_set",
			AssertionIDs: []string{"known_eoc_scope_before_rank"},
		},
		"memory.neighborhood-facet-matrix.v1": {
			BuilderID:    "memory.neighborhood-facet-matrix.v1",
			Action:       "neighborhood",
			ResultKind:   "exact_neighborhood",
			AssertionIDs: []string{"closed_facet_postures"},
		},
		"memory.stale-basis-retry.v1": {
			BuilderID:     "memory.stale-basis-retry.v1",
			Action:        "neighborhood",
			ResultKind:    "retry_required",
			AssertionIDs:  []string{"typed_retry_required"},
			UseStaleBasis: true,
		},
		"memory.read-affordance.v1": {
			BuilderID:    "memory.read-affordance.v1",
			Action:       "neighborhood",
			ResultKind:   "exact_neighborhood",
			AssertionIDs: []string{"closed_read_affordances"},
		},
	}
}

func buildP14MemoryReadScenario(
	ctx context.Context,
	declared scenarioContract,
	fixture p14MemoryReadFixture,
	executable string,
	projectRoot string,
	executableDigest string,
	executor p14MemoryReadExecutor,
) (preparedP14Scenario, error) {
	if err := validateP14MemoryReadFixtureShape(fixture); err != nil {
		return preparedP14Scenario{}, err
	}
	policy, err := p14MemoryReadPolicyForBuilder(declared.RequestBuilder)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	semantic := canonicalP14MemoryReadSemanticRequest(
		declared.ID,
		fixture,
		policy,
	)
	semanticBytes, err := marshalP14CanonicalJSON(semantic)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	semanticDigest := p14Digest(semanticBytes)
	requests, cliCases, err := buildP14MemoryReadSurfaceRequests(
		declared,
		semantic,
		semanticDigest,
	)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	observations, err := executeP14MemoryReadCases(
		ctx,
		executable,
		projectRoot,
		semantic.Cases,
		cliCases,
		executor,
	)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	normalized, outputDigests, err := normalizeP14MemoryReadObservations(
		semantic,
		observations,
	)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	normalizedBytes, err := marshalP14CanonicalJSON(normalized)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	expectedResultDigest := p14Digest(normalizedBytes)
	localOracle := p14MemoryReadLocalOracle{
		Schema:                p14MemoryReadLocalOracleSchema,
		CandidateDigest:       executableDigest,
		SemanticRequestDigest: semanticDigest,
		ExpectedResultDigest:  expectedResultDigest,
		CaseOutputDigests:     outputDigests,
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
			NormalizationID:         p14MemoryReadNormalizationID,
			ExpectedResultDigest:    expectedResultDigest,
			ExpectedEffect:          declared.ExpectedEffect,
			LocalOracleOutputDigest: p14Digest(localOracleBytes),
		},
	}, nil
}

func p14MemoryReadPolicyForBuilder(
	builderID string,
) (p14MemoryReadPolicy, error) {
	policy, present := p14MemoryReadPolicies()[builderID]
	if !present {
		return p14MemoryReadPolicy{}, fmt.Errorf(
			"P14 memory-read builder %q is unsupported",
			builderID,
		)
	}
	return policy, nil
}

func canonicalP14MemoryReadSemanticRequest(
	scenarioID string,
	fixture p14MemoryReadFixture,
	policy p14MemoryReadPolicy,
) p14MemoryReadSemanticRequest {
	entity := fixture.EntityRef
	if policy.UseLegacyEntity {
		entity = fixture.LegacyEntityRef
	}
	basis := fixture.Basis
	if policy.UseStaleBasis {
		basis = fixture.StaleBasis
	}
	requestBuilders := map[string]func() p14MemoryReadCoreRequest{
		"resolve": func() p14MemoryReadCoreRequest {
			return p14MemoryReadCoreRequest{
				ContractVersion: fixture.ContractVersion,
				Action:          "resolve",
				Basis:           basis,
				Query:           fixture.UnknownQuery,
				BoundedContext:  fixture.BoundedContext,
				MaxCandidates:   fixture.MaxCandidates,
			}
		},
		"neighborhood": func() p14MemoryReadCoreRequest {
			requestEntity := entity
			view := cloneP14MemoryReadView(fixture.View)
			budget := fixture.ReadBudget
			if policy.UseLegacyEntity {
				view = p14MemoryReadLegacyView(view)
				budget.MaxFacets = len(view.RequestedFacets)
			}
			return p14MemoryReadCoreRequest{
				ContractVersion: fixture.ContractVersion,
				Action:          "neighborhood",
				Basis:           basis,
				BoundedContext:  fixture.BoundedContext,
				EntityRef:       &requestEntity,
				View:            &view,
				ReadBudget:      &budget,
			}
		},
		"recall": func() p14MemoryReadCoreRequest {
			requestEntity := entity
			view := cloneP14MemoryReadView(fixture.View)
			budget := fixture.ReadBudget
			candidateBudget := fixture.CandidateBudget
			return p14MemoryReadCoreRequest{
				ContractVersion: fixture.ContractVersion,
				Action:          "recall",
				Basis:           basis,
				Query:           fixture.RecallQuery,
				BoundedContext:  fixture.BoundedContext,
				EntityRef:       &requestEntity,
				View:            &view,
				ReadBudget:      &budget,
				CandidateBudget: &candidateBudget,
			}
		},
	}
	builder := requestBuilders[policy.Action]
	request := builder()
	expected := p14MemoryReadExpectedResult{
		ResultKind:   policy.ResultKind,
		AssertionIDs: slices.Clone(policy.AssertionIDs),
	}
	if policy.UseStaleBasis {
		required := fixture.Basis
		expected.RequiredBasis = &required
	}
	return p14MemoryReadSemanticRequest{
		Schema:     p14MemoryReadSemanticSchema,
		ScenarioID: scenarioID,
		Cases: []p14MemoryReadSemanticCase{
			{
				ID:       scenarioID + "_primary",
				Request:  request,
				Expected: expected,
			},
		},
	}
}

func p14MemoryReadLegacyView(
	view p14MemoryReadView,
) p14MemoryReadView {
	if slices.Contains(view.RequestedFacets, "epistemes") {
		return view
	}
	view.RequestedFacets = append(
		[]string{"epistemes"},
		view.RequestedFacets...,
	)
	return view
}

func cloneP14MemoryReadView(view p14MemoryReadView) p14MemoryReadView {
	cloned := view
	cloned.RequestedFacets = slices.Clone(view.RequestedFacets)
	return cloned
}

func buildP14MemoryReadSurfaceRequests(
	declared scenarioContract,
	semantic p14MemoryReadSemanticRequest,
	semanticDigest string,
) ([]preparedP14Request, []p14MemoryReadCLICallCase, error) {
	cliBytes, cliCases, err := buildP14MemoryReadCLISurface(
		semantic,
		semanticDigest,
	)
	if err != nil {
		return nil, nil, err
	}
	mcpBytes, err := buildP14MemoryReadMCPSurface(semantic, semanticDigest)
	if err != nil {
		return nil, nil, err
	}
	payloads := map[string]struct {
		Encoding string
		Bytes    []byte
	}{
		"installed_cli": {Encoding: "argv_json", Bytes: cliBytes},
		"live_mcp":      {Encoding: "canonical_json", Bytes: mcpBytes},
	}
	requests := make([]preparedP14Request, 0, len(declared.Surfaces))
	for _, surface := range declared.Surfaces {
		payload, present := payloads[surface]
		if !present {
			return nil, nil, fmt.Errorf(
				"P14 memory-read surface %q is unsupported",
				surface,
			)
		}
		requests = append(requests, preparedP14Request{
			Surface:               surface,
			Builder:               declared.RequestBuilder,
			Encoding:              payload.Encoding,
			CanonicalPayload:      string(payload.Bytes),
			PayloadDigest:         p14Digest(payload.Bytes),
			SemanticRequestDigest: semanticDigest,
		})
	}
	return requests, cliCases, nil
}

func buildP14MemoryReadCLISurface(
	semantic p14MemoryReadSemanticRequest,
	semanticDigest string,
) ([]byte, []p14MemoryReadCLICallCase, error) {
	cases := make([]p14MemoryReadCLICallCase, 0, len(semantic.Cases))
	for _, testCase := range semantic.Cases {
		stdin, err := marshalP14CanonicalJSON(testCase.Request)
		if err != nil {
			return nil, nil, err
		}
		cases = append(cases, p14MemoryReadCLICallCase{
			ID: testCase.ID,
			Argv: []string{
				"memory",
				testCase.Request.Action,
				"--input-file",
				"-",
			},
			Stdin: string(stdin),
		})
	}
	payload := p14MemoryReadCLISurface{
		Schema:                p14MemoryReadCLISurfaceSchema,
		SemanticRequestDigest: semanticDigest,
		Cases:                 cases,
	}
	raw, err := marshalP14CanonicalJSON(payload)
	if err != nil {
		return nil, nil, err
	}
	return raw, cases, nil
}

func buildP14MemoryReadMCPSurface(
	semantic p14MemoryReadSemanticRequest,
	semanticDigest string,
) ([]byte, error) {
	cases := make([]p14MemoryReadMCPCallCase, 0, len(semantic.Cases))
	for _, testCase := range semantic.Cases {
		args, err := p14MemoryReadMCPArgs(testCase.Request)
		if err != nil {
			return nil, err
		}
		cases = append(cases, p14MemoryReadMCPCallCase{
			ID:   testCase.ID,
			Args: args,
		})
	}
	payload := p14MemoryReadMCPSurface{
		Schema:                p14MemoryReadMCPSurfaceSchema,
		SemanticRequestDigest: semanticDigest,
		Tool:                  "haft_query",
		Cases:                 cases,
	}
	return marshalP14CanonicalJSON(payload)
}

func p14MemoryReadMCPArgs(
	request p14MemoryReadCoreRequest,
) (map[string]any, error) {
	raw, err := marshalP14CanonicalJSON(request)
	if err != nil {
		return nil, err
	}
	args := make(map[string]any)
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&args); err != nil {
		return nil, fmt.Errorf("decode P14 memory-read MCP args: %w", err)
	}
	return p14MemoryQueryMCPEnvelope(args, request.Action)
}

func p14MemoryQueryMCPEnvelope(
	flat map[string]any,
	mode string,
) (map[string]any, error) {
	if mode != "resolve" &&
		mode != "neighborhood" &&
		mode != "recall" {
		return nil, fmt.Errorf(
			"P14 public memory-query mode %q is unsupported",
			mode,
		)
	}
	inner := make(map[string]any, len(flat)+1)
	for key, value := range flat {
		if key == "action" || key == "mode" {
			continue
		}
		inner[key] = value
	}
	inner["mode"] = mode
	return map[string]any{
		"action":         "memory",
		"memory_request": inner,
	}, nil
}

func executeP14MemoryReadCases(
	ctx context.Context,
	executable string,
	projectRoot string,
	semanticCases []p14MemoryReadSemanticCase,
	cliCases []p14MemoryReadCLICallCase,
	executor p14MemoryReadExecutor,
) (map[string]p14FPFProjectionCommandObservation, error) {
	if len(semanticCases) != len(cliCases) {
		return nil, fmt.Errorf("P14 memory-read semantic and CLI case counts differ")
	}
	observations := make(
		map[string]p14FPFProjectionCommandObservation,
		len(semanticCases),
	)
	for index, testCase := range semanticCases {
		cliCase := cliCases[index]
		if testCase.ID != cliCase.ID {
			return nil, fmt.Errorf("P14 memory-read case order differs")
		}
		observation, err := executor(
			ctx,
			executable,
			projectRoot,
			testCase,
			cliCase,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"execute P14 memory-read case %q: %w",
				testCase.ID,
				err,
			)
		}
		observations[testCase.ID] = observation
	}
	return observations, nil
}

func executeP14MemoryReadCandidate(
	ctx context.Context,
	executable string,
	projectRoot string,
	_ p14MemoryReadSemanticCase,
	cliCase p14MemoryReadCLICallCase,
) (p14FPFProjectionCommandObservation, error) {
	command := exec.CommandContext(ctx, executable, cliCase.Argv...)
	command.Dir = projectRoot
	command.Stdin = strings.NewReader(cliCase.Stdin)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err == nil {
		return p14FPFProjectionCommandObservation{
			Stdout:   slices.Clone(stdout.Bytes()),
			Stderr:   slices.Clone(stderr.Bytes()),
			ExitCode: 0,
		}, nil
	}
	exitError, valid := err.(*exec.ExitError)
	if !valid {
		return p14FPFProjectionCommandObservation{}, err
	}
	return p14FPFProjectionCommandObservation{
		Stdout:   slices.Clone(stdout.Bytes()),
		Stderr:   slices.Clone(stderr.Bytes()),
		ExitCode: exitError.ExitCode(),
	}, nil
}

func normalizeP14MemoryReadObservations(
	semantic p14MemoryReadSemanticRequest,
	observations map[string]p14FPFProjectionCommandObservation,
) (p14MemoryReadNormalizedOutput, []string, error) {
	normalized := make(
		[]p14MemoryReadNormalizedCaseOutput,
		0,
		len(semantic.Cases),
	)
	digests := make([]string, 0, len(semantic.Cases))
	for _, testCase := range semantic.Cases {
		observation, present := observations[testCase.ID]
		if !present {
			return p14MemoryReadNormalizedOutput{}, nil, fmt.Errorf(
				"P14 memory-read case %q was not observed",
				testCase.ID,
			)
		}
		payload, canonical, err := decodeP14CandidateJSONObservation(observation)
		if err != nil {
			return p14MemoryReadNormalizedOutput{}, nil, fmt.Errorf(
				"P14 memory-read case %q: %w",
				testCase.ID,
				err,
			)
		}
		if err := assertP14MemoryReadCase(testCase, payload); err != nil {
			return p14MemoryReadNormalizedOutput{}, nil, fmt.Errorf(
				"P14 memory-read case %q: %w",
				testCase.ID,
				err,
			)
		}
		normalized = append(normalized, p14MemoryReadNormalizedCaseOutput{
			ID:      testCase.ID,
			Outcome: "json",
			Payload: slices.Clone(canonical),
		})
		digests = append(digests, p14Digest(canonical))
	}
	if len(observations) != len(semantic.Cases) {
		return p14MemoryReadNormalizedOutput{}, nil, fmt.Errorf(
			"P14 memory-read observation count differs",
		)
	}
	return p14MemoryReadNormalizedOutput{
		Schema: p14MemoryReadNormalizedOutputSchema,
		Cases:  normalized,
	}, digests, nil
}

func assertP14MemoryReadCase(
	testCase p14MemoryReadSemanticCase,
	payload map[string]any,
) error {
	if payload["contract_version"] != testCase.Request.ContractVersion ||
		payload["action"] != testCase.Request.Action ||
		payload["result_kind"] != testCase.Expected.ResultKind {
		return fmt.Errorf("memory-read envelope differs from the semantic request")
	}
	result, valid := payload["result"].(map[string]any)
	if !valid {
		return fmt.Errorf("memory-read result object is absent")
	}
	if err := assertP14MemoryInterpretationIsNonAuthorizing(result); err != nil {
		return err
	}
	if testCase.Expected.RequiredBasis == nil {
		if err := assertP14MemorySnapshotBasis(result, testCase.Request.Basis); err != nil {
			return err
		}
	} else {
		if err := assertP14MemoryRequiredSnapshot(
			result,
			*testCase.Expected.RequiredBasis,
		); err != nil {
			return err
		}
	}
	assertions := map[string]func(p14MemoryReadSemanticCase, map[string]any) error{
		"exact_profile_basis":         assertP14MemoryExactProfileBasis,
		"unknown_identity_not_minted": assertP14MemoryUnknownIdentityNotMinted,
		"known_eoc_scope_before_rank": assertP14MemoryKnownEoCScopeBeforeRank,
		"closed_read_affordances":     assertP14MemoryClosedReadAffordances,
		"closed_facet_postures":       assertP14MemoryClosedFacetPostures,
		"legacy_posture_explicit":     assertP14MemoryLegacyPostureExplicit,
		"typed_retry_required":        assertP14MemoryTypedRetryRequired,
	}
	for _, assertionID := range testCase.Expected.AssertionIDs {
		assertion, present := assertions[assertionID]
		if !present {
			return fmt.Errorf("memory-read assertion %q is open", assertionID)
		}
		if err := assertion(testCase, result); err != nil {
			return fmt.Errorf("memory-read assertion %q: %w", assertionID, err)
		}
	}
	return nil
}

func assertP14MemoryRequiredSnapshot(
	result map[string]any,
	basis p14MemoryReadBasis,
) error {
	required, valid := result["required_snapshot"].(map[string]any)
	if !valid || required["type_env_digest"] != basis.TypeEnvDigest {
		return fmt.Errorf("memory-read required TypeEnv snapshot differs")
	}
	graphRevision, valid := p14FPFJSONPositiveInt(required["graph_revision"])
	if !valid || graphRevision != basis.GraphRevision {
		return fmt.Errorf("memory-read required graph snapshot differs")
	}
	return nil
}

func assertP14MemoryInterpretationIsNonAuthorizing(
	result map[string]any,
) error {
	contract, valid := result["interpretation_contract"].(map[string]any)
	if !valid {
		return fmt.Errorf("memory-read interpretation contract is absent")
	}
	expected := map[string]string{
		"applicability": "not_implied",
		"authority":     "not_granted",
		"truth":         "not_implied",
		"work_order":    "not_implied",
	}
	for key, value := range expected {
		if contract[key] != value {
			return fmt.Errorf("interpretation %s = %#v, want %q", key, contract[key], value)
		}
	}
	return nil
}

func assertP14MemorySnapshotBasis(
	result map[string]any,
	basis p14MemoryReadBasis,
) error {
	snapshot, valid := result["snapshot_basis"].(map[string]any)
	if !valid || snapshot["type_env_digest"] != basis.TypeEnvDigest {
		return fmt.Errorf("memory-read TypeEnv snapshot differs from exact basis")
	}
	graphRevision, valid := p14FPFJSONPositiveInt(snapshot["graph_revision"])
	if !valid || graphRevision != basis.GraphRevision {
		return fmt.Errorf("memory-read graph snapshot differs from exact basis")
	}
	return nil
}

func assertP14MemoryExactProfileBasis(
	testCase p14MemoryReadSemanticCase,
	result map[string]any,
) error {
	if testCase.Request.View == nil {
		return fmt.Errorf("exact-profile request has no view")
	}
	projection, valid := result["projection_basis"].(map[string]any)
	if !valid || projection["profile_ref"] != testCase.Request.View.ProjectionProfileRef {
		return fmt.Errorf("projection basis differs from requested profile")
	}
	for _, key := range []string{
		"schema",
		"profile_edition",
		"profile_digest",
		"projection_schema_version",
		"canonical_inputs",
		"declared_input_families",
		"declared_slot_kinds",
		"item_basis",
	} {
		if _, present := projection[key]; !present {
			return fmt.Errorf("projection basis omits %q", key)
		}
	}
	return nil
}

func assertP14MemoryUnknownIdentityNotMinted(
	testCase p14MemoryReadSemanticCase,
	result map[string]any,
) error {
	scope, valid := result["resolution_scope"].(map[string]any)
	if !valid || scope["query"] != testCase.Request.Query ||
		scope["bounded_context_ref"] != testCase.Request.BoundedContext {
		return fmt.Errorf("unknown-EoC resolution scope differs")
	}
	keys := make(map[string]int)
	collectP14FPFJSONKeys(result, keys)
	for _, forbidden := range []string{"entity", "candidates", "selected_entity"} {
		if keys[forbidden] > 0 {
			return fmt.Errorf("unknown-EoC result minted %q", forbidden)
		}
	}
	return nil
}

func assertP14MemoryKnownEoCScopeBeforeRank(
	testCase p14MemoryReadSemanticCase,
	result map[string]any,
) error {
	if testCase.Request.EntityRef == nil || testCase.Request.View == nil {
		return fmt.Errorf("known-EoC recall request is incomplete")
	}
	scope, valid := result["scope"].(map[string]any)
	if !valid || scope["bounded_context_ref"] != testCase.Request.BoundedContext ||
		scope["projection_profile_ref"] != testCase.Request.View.ProjectionProfileRef {
		return fmt.Errorf("known-EoC recall scope differs")
	}
	entity, valid := scope["entity_ref"].(map[string]any)
	if !valid ||
		entity["ref_kind_id"] != testCase.Request.EntityRef.RefKindID ||
		entity["reference_id"] != testCase.Request.EntityRef.ReferenceID ||
		entity["ref_kind"] != nil {
		return fmt.Errorf("known-EoC recall changed the entity identity")
	}
	candidates, valid := result["candidates"].([]any)
	if !valid || len(candidates) == 0 {
		return fmt.Errorf("known-EoC recall returned no candidate")
	}
	for _, value := range candidates {
		candidate, valid := value.(map[string]any)
		if !valid {
			return fmt.Errorf("known-EoC recall candidate is not an object")
		}
		keys := make(map[string]int)
		collectP14FPFJSONKeys(candidate, keys)
		for _, forbidden := range []string{
			"applicability",
			"authority",
			"recommended",
			"selected",
			"work_order",
		} {
			if keys[forbidden] > 0 {
				return fmt.Errorf("recall candidate contains inferred %q", forbidden)
			}
		}
	}
	return nil
}

func assertP14MemoryClosedReadAffordances(
	_ p14MemoryReadSemanticCase,
	result map[string]any,
) error {
	affordances, valid := result["read_affordances"].([]any)
	if !valid || len(affordances) == 0 {
		return fmt.Errorf("read-affordance result is empty")
	}
	allowed := map[string]struct{}{
		"expand_facet":          {},
		"inspect_entity":        {},
		"hydrate_carrier":       {},
		"follow_context_bridge": {},
	}
	for _, value := range affordances {
		affordance, valid := value.(map[string]any)
		if !valid {
			return fmt.Errorf("read affordance is not an object")
		}
		kind, valid := affordance["kind"].(string)
		if !valid {
			return fmt.Errorf("read affordance has no kind")
		}
		if _, present := allowed[kind]; !present {
			return fmt.Errorf("read affordance %q is capability or work-like", kind)
		}
	}
	return nil
}

func assertP14MemoryClosedFacetPostures(
	_ p14MemoryReadSemanticCase,
	result map[string]any,
) error {
	facets, valid := result["facets"].([]any)
	if !valid || len(facets) == 0 {
		return fmt.Errorf("facet-coverage result has no facets")
	}
	allowed := map[string]struct{}{
		"complete":       {},
		"partial":        {},
		"not_applicable": {},
		"unavailable":    {},
		"stale":          {},
	}
	for _, value := range facets {
		facet, valid := value.(map[string]any)
		if !valid {
			return fmt.Errorf("facet-coverage entry is not an object")
		}
		coverage, valid := facet["coverage"].(map[string]any)
		if !valid {
			return fmt.Errorf("facet-coverage entry has no coverage")
		}
		kind, valid := coverage["kind"].(string)
		if !valid || kind == "" {
			return fmt.Errorf("facet-coverage entry has no kind")
		}
		if _, present := allowed[kind]; !present {
			return fmt.Errorf("facet-coverage entry has open kind %q", kind)
		}
	}
	return nil
}

func TestP14FacetCoverageAssertionAcceptsAClosedProductionSubset(t *testing.T) {
	exactResult := map[string]any{
		"facets": []any{
			map[string]any{
				"coverage": map[string]any{
					"kind":     "complete",
					"included": float64(1),
				},
			},
		},
	}
	if err := assertP14MemoryClosedFacetPostures(
		p14MemoryReadSemanticCase{},
		exactResult,
	); err != nil {
		t.Fatalf("closed production subset rejected: %v", err)
	}

	openResult := map[string]any{
		"facets": []any{
			map[string]any{
				"coverage": map[string]any{
					"kind":     "guessed",
					"included": float64(0),
				},
			},
		},
	}
	if err := assertP14MemoryClosedFacetPostures(
		p14MemoryReadSemanticCase{},
		openResult,
	); err == nil {
		t.Fatal("open facet-coverage posture unexpectedly accepted")
	}
}

func assertP14MemoryLegacyPostureExplicit(
	_ p14MemoryReadSemanticCase,
	result map[string]any,
) error {
	values := make(map[string]int)
	collectP14JSONStringValues(result, values)
	allowed := []string{
		"legacy_unqualified_assertion",
		"historical",
		"degraded",
	}
	for _, value := range allowed {
		if values[value] > 0 {
			return nil
		}
	}
	return fmt.Errorf("legacy read has no explicit historical or degraded posture")
}

func assertP14MemoryTypedRetryRequired(
	testCase p14MemoryReadSemanticCase,
	result map[string]any,
) error {
	if testCase.Expected.RequiredBasis == nil {
		return fmt.Errorf("retry result has no required basis")
	}
	cause, valid := result["cause"].(map[string]any)
	if !valid {
		return fmt.Errorf("retry result has no typed cause")
	}
	kind, valid := cause["kind"].(string)
	if !valid || kind == "" {
		return fmt.Errorf("retry cause kind is absent")
	}
	retry, valid := result["retry_operation"].(string)
	if !valid || retry != "reload_snapshot" {
		return fmt.Errorf("retry operation = %q, want reload_snapshot", retry)
	}
	return nil
}

func collectP14JSONStringValues(value any, values map[string]int) {
	switch typed := value.(type) {
	case string:
		values[typed]++
	case map[string]any:
		for _, child := range typed {
			collectP14JSONStringValues(child, values)
		}
	case []any:
		for _, child := range typed {
			collectP14JSONStringValues(child, values)
		}
	}
}

func validateP14MemoryReadPreparedScenario(
	declared scenarioContract,
	scenario preparedP14Scenario,
) error {
	policy, err := p14MemoryReadPolicyForBuilder(declared.RequestBuilder)
	if err != nil {
		return err
	}
	semantic, err := decodeP14MemoryReadSemanticRequest(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return err
	}
	if semantic.Schema != p14MemoryReadSemanticSchema ||
		semantic.ScenarioID != declared.ID || len(semantic.Cases) != 1 {
		return fmt.Errorf("P14 memory-read semantic envelope differs")
	}
	testCase := semantic.Cases[0]
	expectedID := declared.ID + "_primary"
	if testCase.ID != expectedID ||
		testCase.Request.Action != policy.Action ||
		testCase.Expected.ResultKind != policy.ResultKind ||
		!slices.Equal(testCase.Expected.AssertionIDs, policy.AssertionIDs) {
		return fmt.Errorf("P14 memory-read case differs from policy")
	}
	if policy.UseStaleBasis {
		if testCase.Expected.RequiredBasis == nil ||
			*testCase.Expected.RequiredBasis == testCase.Request.Basis {
			return fmt.Errorf("P14 stale-basis case has no distinct required basis")
		}
	} else if testCase.Expected.RequiredBasis != nil {
		return fmt.Errorf("P14 ordinary memory read carries a retry-only required basis")
	}
	if err := validateP14MemoryReadCoreRequest(testCase.Request); err != nil {
		return err
	}
	expectedRequests, _, err := buildP14MemoryReadSurfaceRequests(
		declared,
		semantic,
		scenario.SemanticRequestDigest,
	)
	if err != nil {
		return err
	}
	if len(expectedRequests) != len(scenario.Requests) {
		return fmt.Errorf("P14 memory-read surface count differs")
	}
	for index, expected := range expectedRequests {
		if scenario.Requests[index] != expected {
			return fmt.Errorf("P14 memory-read surface %q diverges", expected.Surface)
		}
	}
	if scenario.Oracle.NormalizationID != p14MemoryReadNormalizationID {
		return fmt.Errorf("P14 memory-read normalizer differs")
	}
	return nil
}

func validateP14MemoryReadCoreRequest(
	request p14MemoryReadCoreRequest,
) error {
	if request.ContractVersion != p14MemoryReadContractVersion ||
		request.Basis.Kind != "exact_project" ||
		!validP14Digest(request.Basis.TypeEnvDigest) ||
		request.Basis.GraphRevision <= 0 {
		return fmt.Errorf("P14 memory-read exact basis is invalid")
	}
	validators := map[string]func() bool{
		"resolve": func() bool {
			return request.Query != "" && request.BoundedContext != "" &&
				request.MaxCandidates > 0 && request.EntityRef == nil &&
				request.View == nil && request.ReadBudget == nil &&
				request.CandidateBudget == nil
		},
		"neighborhood": func() bool {
			return request.Query == "" && request.MaxCandidates == 0 &&
				request.BoundedContext != "" && request.EntityRef != nil &&
				request.View != nil && request.ReadBudget != nil &&
				request.CandidateBudget == nil
		},
		"recall": func() bool {
			return request.Query != "" && request.MaxCandidates == 0 &&
				request.BoundedContext != "" && request.EntityRef != nil &&
				request.View != nil && request.ReadBudget != nil &&
				request.CandidateBudget != nil &&
				request.CandidateBudget.MaxCandidates > 0
		},
	}
	validator, present := validators[request.Action]
	if !present || !validator() {
		return fmt.Errorf("P14 memory-read %q request shape is invalid", request.Action)
	}
	return nil
}

func decodeP14MemoryReadSemanticRequest(
	raw []byte,
) (p14MemoryReadSemanticRequest, error) {
	reader := bytes.NewReader(raw)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var semantic p14MemoryReadSemanticRequest
	if err := decoder.Decode(&semantic); err != nil {
		return p14MemoryReadSemanticRequest{}, fmt.Errorf(
			"decode P14 memory-read semantic carrier: %w",
			err,
		)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		return p14MemoryReadSemanticRequest{}, fmt.Errorf(
			"P14 memory-read semantic carrier has trailing JSON",
		)
	}
	return semantic, nil
}

func validateP14MemoryReadFixtureShape(fixture p14MemoryReadFixture) error {
	if fixture.Schema != p14MemoryReadFixtureSchema ||
		fixture.ContractVersion != p14MemoryReadContractVersion ||
		fixture.Basis.Kind != "exact_project" ||
		!validP14Digest(fixture.Basis.TypeEnvDigest) ||
		fixture.Basis.GraphRevision <= 0 {
		return fmt.Errorf("P14 memory-read fixture basis is invalid")
	}
	if fixture.EntityRef.RefKindID == "" || fixture.EntityRef.ReferenceID == "" ||
		fixture.LegacyEntityRef.RefKindID == "" ||
		fixture.LegacyEntityRef.ReferenceID == "" ||
		fixture.BoundedContext == "" || fixture.UnknownQuery == "" ||
		fixture.RecallQuery == "" || fixture.UnknownQuery == fixture.RecallQuery {
		return fmt.Errorf("P14 memory-read fixture identity or query is invalid")
	}
	if fixture.StaleBasis.Kind != "exact_project" ||
		!validP14Digest(fixture.StaleBasis.TypeEnvDigest) ||
		fixture.StaleBasis.GraphRevision <= 0 ||
		fixture.StaleBasis == fixture.Basis {
		return fmt.Errorf("P14 memory-read stale basis is invalid")
	}
	if fixture.View.ProjectionProfileRef != "agent_orientation.v2" ||
		fixture.View.Detail != "evidence" || !fixture.View.IncludeHistory ||
		len(fixture.View.RequestedFacets) == 0 ||
		hasBlankOrDuplicate(fixture.View.RequestedFacets) {
		return fmt.Errorf("P14 memory-read fixture view is invalid")
	}
	budgetValues := []int{
		fixture.ReadBudget.MaxFacets,
		fixture.ReadBudget.MaxItemsPerFacet,
		fixture.ReadBudget.MaxRelationPathsPerItem,
		fixture.ReadBudget.MaxCarrierExcerptCharacters,
		fixture.ReadBudget.MaxProvenanceDepth,
		fixture.MaxCandidates,
		fixture.CandidateBudget.MaxCandidates,
	}
	for _, value := range budgetValues {
		if value <= 0 {
			return fmt.Errorf("P14 memory-read fixture budget is invalid")
		}
	}
	if fixture.ReadBudget.MaxFacets < len(fixture.View.RequestedFacets) {
		return fmt.Errorf("P14 memory-read facet budget cannot cover requested facets")
	}
	return validateP14MemoryOperationFixtureShape(fixture.Operations)
}

func verifyP14MemoryReadFixtureBinding(
	repositoryRoot string,
	input preparedRequestOracleInput,
) error {
	binding, err := preparedP14BindingByGroup(
		input.Bindings,
		"golden_memory_fixture",
	)
	if err != nil {
		return err
	}
	path := filepath.Join(repositoryRoot, filepath.FromSlash(binding.CarrierPath))
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read P14 golden-memory fixture: %w", err)
	}
	fixture, err := decodeP14MemoryReadFixture(raw)
	if err != nil {
		return err
	}
	if err := validateP14MemoryReadFixtureShape(fixture); err != nil {
		return err
	}
	selected := input.FrozenBasis.SelectedProject
	selectedDigest := strings.TrimPrefix(selected.SelectedCompositeRef, "typeenv:")
	if fixture.Basis.TypeEnvDigest != selectedDigest ||
		fixture.Basis.GraphRevision != selected.GraphRevision ||
		fixture.Operations.SelectedProjectRoot != selected.ProjectRoot {
		return fmt.Errorf("P14 golden-memory fixture differs from selected project basis")
	}
	contractByID := make(map[string]scenarioContract)
	contract, _, err := loadRequestOracleContract(repositoryRoot)
	if err != nil {
		return err
	}
	for _, declared := range contract.Scenarios {
		contractByID[declared.ID] = declared
	}
	for _, builderID := range p14MemoryReadBuilderIDs {
		declared, err := p14MemoryReadDeclaredByBuilder(contractByID, builderID)
		if err != nil {
			return err
		}
		policy, err := p14MemoryReadPolicyForBuilder(builderID)
		if err != nil {
			return err
		}
		expected := canonicalP14MemoryReadSemanticRequest(
			declared.ID,
			fixture,
			policy,
		)
		scenario, err := preparedP14ScenarioByID(input.Scenarios, declared.ID)
		if err != nil {
			return err
		}
		expectedBytes, err := marshalP14CanonicalJSON(expected)
		if err != nil {
			return err
		}
		if scenario.SemanticRequestCanonical != string(expectedBytes) {
			return fmt.Errorf(
				"P14 memory-read scenario %q differs from frozen fixture",
				declared.ID,
			)
		}
	}
	operationDigest, err := observeP14SelectedProjectMemoryBasis(
		fixture.Operations.SelectedProjectRoot,
	)
	if err != nil {
		return err
	}
	if operationDigest != fixture.Operations.SelectedProjectBasisDigest {
		return fmt.Errorf("P14 memory-operation selected-project basis differs")
	}
	homeDigest, err := observeP14InitTree(
		fixture.Operations.HomeTemplateRoot,
	)
	if err != nil {
		return err
	}
	if homeDigest != fixture.Operations.HomeTemplateDigest {
		return fmt.Errorf("P14 memory-operation home template differs")
	}
	for _, builderID := range p14MemoryOperationBuilderIDs {
		declared, err := p14MemoryReadDeclaredByBuilder(contractByID, builderID)
		if err != nil {
			return err
		}
		expected, err := buildP14MemoryOperationScenario(
			declared,
			fixture.Operations,
		)
		if err != nil {
			return err
		}
		scenario, err := preparedP14ScenarioByID(input.Scenarios, declared.ID)
		if err != nil {
			return err
		}
		if scenario.SemanticRequestCanonical != expected.SemanticRequestCanonical {
			return fmt.Errorf(
				"P14 memory-operation scenario %q differs from frozen fixture",
				declared.ID,
			)
		}
	}
	return nil
}

func p14MemoryReadDeclaredByBuilder(
	contractByID map[string]scenarioContract,
	builderID string,
) (scenarioContract, error) {
	for _, declared := range contractByID {
		if declared.RequestBuilder == builderID {
			return declared, nil
		}
	}
	return scenarioContract{}, fmt.Errorf(
		"P14 memory-read builder %q has no scenario",
		builderID,
	)
}

func decodeP14MemoryReadFixture(raw []byte) (p14MemoryReadFixture, error) {
	reader := bytes.NewReader(raw)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var fixture p14MemoryReadFixture
	if err := decoder.Decode(&fixture); err != nil {
		return p14MemoryReadFixture{}, fmt.Errorf(
			"decode P14 golden-memory fixture: %w",
			err,
		)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		return p14MemoryReadFixture{}, fmt.Errorf(
			"P14 golden-memory fixture has trailing JSON",
		)
	}
	canonical, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		return p14MemoryReadFixture{}, err
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(raw, canonical) {
		return p14MemoryReadFixture{}, fmt.Errorf(
			"P14 golden-memory fixture is not canonical JSON",
		)
	}
	return fixture, nil
}

func syntheticP14MemoryReadFixture() p14MemoryReadFixture {
	fixture := p14MemoryReadFixture{
		Schema:          p14MemoryReadFixtureSchema,
		ContractVersion: p14MemoryReadContractVersion,
		Basis: p14MemoryReadBasis{
			Kind:          "exact_project",
			TypeEnvDigest: p14TestDigest("synthetic-memory-typeenv"),
			GraphRevision: 4,
		},
		EntityRef: p14MemoryReadEntityRef{
			RefKindID:   "U.EntityRef",
			ReferenceID: "entity:haft-v9-typed-memory",
		},
		LegacyEntityRef: p14MemoryReadEntityRef{
			RefKindID:   "U.EntityRef",
			ReferenceID: "entity:haft-v8-legacy-memory",
		},
		StaleBasis: p14MemoryReadBasis{
			Kind:          "exact_project",
			TypeEnvDigest: p14TestDigest("synthetic-stale-memory-typeenv"),
			GraphRevision: 3,
		},
		BoundedContext: "haft-project",
		View: p14MemoryReadView{
			ProjectionProfileRef: "agent_orientation.v2",
			RequestedFacets: []string{
				"problems",
				"alternatives",
				"decisions",
				"specifications",
				"evidence",
				"work",
				"implementation",
				"unresolved",
			},
			Detail:         "evidence",
			IncludeHistory: true,
		},
		ReadBudget: p14MemoryReadBudget{
			MaxFacets:                   8,
			MaxItemsPerFacet:            8,
			MaxRelationPathsPerItem:     4,
			MaxCarrierExcerptCharacters: 2048,
			MaxProvenanceDepth:          3,
		},
		UnknownQuery:  "p14-definitely-unknown-entity-7f9b2c",
		RecallQuery:   "typed memory architecture decision evidence",
		MaxCandidates: 8,
		CandidateBudget: p14MemoryCandidateBudget{
			MaxCandidates: 8,
		},
	}
	fixture.Operations = syntheticP14MemoryOperationFixture(fixture)
	return fixture
}

func syntheticP14MemoryReadScenario(
	declared scenarioContract,
) (preparedP14Scenario, error) {
	return buildP14MemoryReadScenario(
		context.Background(),
		declared,
		syntheticP14MemoryReadFixture(),
		"/synthetic/haft",
		"/synthetic/project",
		p14TestDigest("synthetic-memory-candidate"),
		executeSyntheticP14MemoryReadCandidate,
	)
}

func executeSyntheticP14MemoryReadCandidate(
	_ context.Context,
	_ string,
	_ string,
	testCase p14MemoryReadSemanticCase,
	_ p14MemoryReadCLICallCase,
) (p14FPFProjectionCommandObservation, error) {
	payload := syntheticP14MemoryReadPayload(testCase)
	raw, err := json.Marshal(payload)
	if err != nil {
		return p14FPFProjectionCommandObservation{}, err
	}
	raw = append(raw, '\n')
	return p14FPFProjectionCommandObservation{
		Stdout:   raw,
		ExitCode: 0,
	}, nil
}

func syntheticP14MemoryReadPayload(
	testCase p14MemoryReadSemanticCase,
) map[string]any {
	resultBuilders := map[string]func() map[string]any{
		"exact_neighborhood": func() map[string]any {
			return syntheticP14ExactNeighborhoodResult(testCase)
		},
		"known_absent": func() map[string]any {
			return syntheticP14KnownAbsentResult(testCase)
		},
		"scoped_memory_candidate_set": func() map[string]any {
			return syntheticP14RecallResult(testCase)
		},
		"retry_required": func() map[string]any {
			return syntheticP14RetryRequiredResult(testCase)
		},
	}
	result := resultBuilders[testCase.Expected.ResultKind]()
	payload := map[string]any{
		"contract_version": testCase.Request.ContractVersion,
		"action":           testCase.Request.Action,
		"result_kind":      testCase.Expected.ResultKind,
		"result":           result,
	}
	if testCase.Expected.ResultKind == "exact_neighborhood" {
		payload["result_digest"] = p14TestDigest(testCase.ID + ":result")
	}
	return payload
}

func syntheticP14ExactNeighborhoodResult(
	testCase p14MemoryReadSemanticCase,
) map[string]any {
	result := map[string]any{
		"schema":                  "haft.memory-neighborhood/v1",
		"snapshot_basis":          syntheticP14MemorySnapshot(testCase.Request.Basis),
		"interpretation_contract": syntheticP14MemoryInterpretation("exact"),
		"projection_basis": map[string]any{
			"schema":                    "haft.projection-basis/v1",
			"profile_ref":               testCase.Request.View.ProjectionProfileRef,
			"profile_edition":           2,
			"profile_digest":            p14TestDigest("profile"),
			"projection_schema_version": "haft.projection-profile/v1",
			"canonical_inputs":          []any{},
			"declared_input_families":   []any{"canonical_typed_memory"},
			"declared_slot_kinds":       []any{"EntityOfConcernSlot"},
			"item_basis":                []any{},
		},
		"read_affordances": []any{map[string]any{
			"kind":                "inspect_entity",
			"entity_reference_id": testCase.Request.EntityRef.ReferenceID,
			"bounded_context_ref": testCase.Request.BoundedContext,
		}},
	}
	if slices.Contains(testCase.Expected.AssertionIDs, "closed_facet_postures") {
		result["facets"] = syntheticP14FacetCoverageMatrix()
	}
	if slices.Contains(testCase.Expected.AssertionIDs, "legacy_posture_explicit") {
		result["facets"] = []any{map[string]any{
			"facet": "unresolved",
			"coverage": map[string]any{
				"kind":     "complete",
				"included": 1,
			},
			"items": []any{map[string]any{
				"relational_record_posture": "legacy_unqualified_assertion",
			}},
		}}
	}
	return result
}

func syntheticP14FacetCoverageMatrix() []any {
	kinds := []string{
		"complete",
		"partial",
		"not_applicable",
		"unavailable",
		"stale",
	}
	facets := make([]any, 0, len(kinds))
	for index, kind := range kinds {
		facets = append(facets, map[string]any{
			"facet": fmt.Sprintf("facet-%d", index),
			"coverage": map[string]any{
				"kind":     kind,
				"included": 0,
			},
			"items": []any{},
		})
	}
	return facets
}

func syntheticP14KnownAbsentResult(
	testCase p14MemoryReadSemanticCase,
) map[string]any {
	return map[string]any{
		"resolution_scope": map[string]any{
			"query":               testCase.Request.Query,
			"bounded_context_ref": testCase.Request.BoundedContext,
			"context_kind":        "exact_context",
		},
		"snapshot_basis":          syntheticP14MemorySnapshot(testCase.Request.Basis),
		"interpretation_contract": syntheticP14MemoryInterpretation("unresolved"),
		"inspected_index": map[string]any{
			"index_ref":     "current-entity-directory:synthetic",
			"index_version": "haft.current-resolution-index/v1@synthetic",
		},
		"completeness_basis_ref": "synthetic-complete-index",
	}
}

func syntheticP14RecallResult(
	testCase p14MemoryReadSemanticCase,
) map[string]any {
	return map[string]any{
		"scope": map[string]any{
			"bounded_context_ref":    testCase.Request.BoundedContext,
			"projection_profile_ref": testCase.Request.View.ProjectionProfileRef,
			"entity_ref": map[string]any{
				"ref_kind_id":  testCase.Request.EntityRef.RefKindID,
				"reference_id": testCase.Request.EntityRef.ReferenceID,
			},
		},
		"snapshot_basis":          syntheticP14MemorySnapshot(testCase.Request.Basis),
		"interpretation_contract": syntheticP14MemoryInterpretation("exact"),
		"candidates": []any{map[string]any{
			"rank":            1,
			"candidate_ref":   "memory-candidate:synthetic",
			"retrieval_basis": "lexical_exact_scope",
		}},
		"candidate_set_coverage": map[string]any{
			"included":         1,
			"omitted_at_least": 0,
		},
		"applied_budget": map[string]any{"max_candidates": 8},
	}
}

func syntheticP14RetryRequiredResult(
	testCase p14MemoryReadSemanticCase,
) map[string]any {
	return map[string]any{
		"cause": map[string]any{
			"kind":              "stale_snapshot",
			"observed_snapshot": syntheticP14MemorySnapshot(testCase.Request.Basis),
			"required_snapshot": syntheticP14MemorySnapshot(*testCase.Expected.RequiredBasis),
		},
		"required_snapshot":       syntheticP14MemorySnapshot(*testCase.Expected.RequiredBasis),
		"retry_operation":         "reload_snapshot",
		"interpretation_contract": syntheticP14MemoryInterpretation("unresolved"),
	}
}

func syntheticP14MemorySnapshot(basis p14MemoryReadBasis) map[string]any {
	return map[string]any{
		"type_env_ref":    "typeenv:" + basis.TypeEnvDigest,
		"type_env_digest": basis.TypeEnvDigest,
		"graph_revision":  basis.GraphRevision,
	}
}

func syntheticP14MemoryInterpretation(identity string) map[string]any {
	return map[string]any{
		"structure":               "exact_snapshot",
		"identity":                identity,
		"relational_records":      "assertions_exact_at_snapshot",
		"ranking":                 "discovery_only",
		"truth":                   "not_implied",
		"applicability":           "not_implied",
		"authority":               "not_granted",
		"work_order":              "not_implied",
		"completeness":            "bounded",
		"hydrate_before_reliance": true,
	}
}

func TestP14MemoryReadBuildersShareOneExactCLIAndMCPRequest(t *testing.T) {
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, _, err := loadRequestOracleContract(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	fixture := syntheticP14MemoryReadFixture()
	for _, builderID := range p14MemoryReadBuilderIDs {
		declared, err := findP14ScenarioContractByBuilder(contract, builderID)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(declared.ID, func(t *testing.T) {
			scenario, err := buildP14MemoryReadScenario(
				context.Background(),
				declared,
				fixture,
				"/synthetic/haft",
				"/synthetic/project",
				p14TestDigest("synthetic-memory-candidate"),
				executeSyntheticP14MemoryReadCandidate,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateP14MemoryReadPreparedScenario(declared, scenario); err != nil {
				t.Fatal(err)
			}
			if len(scenario.Requests) != 2 ||
				scenario.Requests[0].Surface != "installed_cli" ||
				scenario.Requests[1].Surface != "live_mcp" {
				t.Fatalf("memory-read surfaces = %#v", scenario.Requests)
			}
			mcpSurface := p14MemoryReadMCPSurface{}
			if err := decodeP14StrictCompactJSON(
				scenario.Requests[1].CanonicalPayload,
				&mcpSurface,
				"memory-read live MCP surface",
			); err != nil {
				t.Fatal(err)
			}
			for index, call := range mcpSurface.Cases {
				memoryRequest, present :=
					call.Args["memory_request"].(map[string]any)
				if !present ||
					len(call.Args) != 2 ||
					call.Args["action"] != "memory" ||
					call.Args["mode"] != nil ||
					call.Args["basis"] != nil ||
					memoryRequest["mode"] !=
						scenarioMemoryAction(
							scenario,
							index,
						) ||
					memoryRequest["action"] != nil {
					t.Fatal(
						"memory-read live MCP call is not the exact nested public envelope",
					)
				}
			}
			tampered := scenario
			tampered.Requests = slices.Clone(scenario.Requests)
			tampered.Requests[1].CanonicalPayload = `{"schema":"tampered"}`
			tampered.Requests[1].PayloadDigest = p14Digest(
				[]byte(tampered.Requests[1].CanonicalPayload),
			)
			if err := validateP14MemoryReadPreparedScenario(declared, tampered); err == nil {
				t.Fatal("memory-read validator accepted divergent MCP args")
			}
		})
	}
}

func TestP14KnownRecallRejectsLegacyOrPartialEntityRef(t *testing.T) {
	fixture := syntheticP14MemoryReadFixture()
	policy, err := p14MemoryReadPolicyForBuilder("memory.recall-known.v1")
	if err != nil {
		t.Fatal(err)
	}
	semantic := canonicalP14MemoryReadSemanticRequest(
		"known_eoc_recall",
		fixture,
		policy,
	)
	testCase := semantic.Cases[0]
	raw, err := marshalP14CanonicalJSON(
		syntheticP14MemoryReadPayload(testCase),
	)
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if err := assertP14MemoryReadCase(testCase, payload); err != nil {
		t.Fatal(err)
	}
	result := p14JSONMap(payload["result"])
	scope := p14JSONMap(result["scope"])
	entityRef := p14JSONMap(scope["entity_ref"])
	delete(entityRef, "ref_kind_id")
	entityRef["ref_kind"] =
		"typeenv:legacy/ref-kind/U.EntityRef"
	if err := assertP14MemoryReadCase(testCase, payload); err == nil {
		t.Fatal(
			"P14 recall accepted a legacy EntityRef without ref_kind_id",
		)
	}
}

func scenarioMemoryAction(
	scenario preparedP14Scenario,
	index int,
) string {
	semantic := p14MemoryReadSemanticRequest{}
	if err := decodeP14StrictCompactJSON(
		scenario.SemanticRequestCanonical,
		&semantic,
		"memory-read semantic request",
	); err != nil {
		return ""
	}
	if index < 0 || index >= len(semantic.Cases) {
		return ""
	}
	return semantic.Cases[index].Request.Action
}

func TestP14CaptureMemoryReadsAgainstCandidate(t *testing.T) {
	fixturePath := os.Getenv(p14MemoryReadFixtureEnvironmentKey)
	executable := os.Getenv(p14MemoryReadCandidateEnvironmentKey)
	projectRoot := os.Getenv(p14MemoryReadProjectEnvironmentKey)
	if fixturePath == "" || executable == "" || projectRoot == "" {
		t.Skip("set the P14 memory-read fixture, candidate, and project root after P13")
	}
	rawFixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := decodeP14MemoryReadFixture(rawFixture)
	if err != nil {
		t.Fatal(err)
	}
	executableBytes, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, _, err := loadRequestOracleContract(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, builderID := range p14MemoryReadBuilderIDs {
		declared, err := findP14ScenarioContractByBuilder(contract, builderID)
		if err != nil {
			t.Fatal(err)
		}
		scenario, err := buildP14MemoryReadScenario(
			context.Background(),
			declared,
			fixture,
			executable,
			projectRoot,
			p14Digest(executableBytes),
			executeP14MemoryReadCandidate,
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf(
			"P14_MEMORY_READ_PREPARED scenario=%s semantic=%s expected=%s local_oracle=%s",
			scenario.ID,
			scenario.SemanticRequestDigest,
			scenario.Oracle.ExpectedResultDigest,
			scenario.Oracle.LocalOracleOutputDigest,
		)
	}
}

func findP14ScenarioContractByBuilder(
	contract requestOracleContract,
	builderID string,
) (scenarioContract, error) {
	for _, declared := range contract.Scenarios {
		if declared.RequestBuilder == builderID {
			return declared, nil
		}
	}
	return scenarioContract{}, fmt.Errorf(
		"P14 scenario for builder %q is absent",
		builderID,
	)
}
