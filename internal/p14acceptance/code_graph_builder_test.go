package p14acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	p14CodeExploreExactBuilderID      = "code.explore-exact.v1"
	p14CodeExploreConcernBuilderID    = "code.explore-concern.v1"
	p14CodeExploreAmbiguousBuilderID  = "code.explore-ambiguous.v1"
	p14CodeExploreDiagnosticBuilderID = "code.explore-coverage-diagnostic.v1"

	p14CodeExploreSemanticSchema     = "haft.p14.code-explore-semantic/v1"
	p14CodeExploreCLISurfaceSchema   = "haft.p14.code-explore-cli-surface/v1"
	p14CodeExploreMCPSurfaceSchema   = "haft.p14.code-explore-mcp-surface/v1"
	p14CodeExploreNormalizedSchema   = "haft.p14.code-explore-normalized/v1"
	p14CodeExploreLocalOracleSchema  = "haft.p14.code-explore-local-oracle/v1"
	p14CodeExploreNormalizationID    = "haft.p14.code-explore-normalizer/v1"
	p14CodeExplorePreparationTimeout = 3 * time.Minute

	p14CodeExploreCurrentDecisionRef = "dec-20260716-11f33e36"
	p14CodeExploreStaleDecisionRef   = "dec-20260713-9ed66ef0"
)

var p14CodeExploreBuilderIDs = []string{
	p14CodeExploreExactBuilderID,
	p14CodeExploreConcernBuilderID,
	p14CodeExploreAmbiguousBuilderID,
	p14CodeExploreDiagnosticBuilderID,
}

type p14CodeExplorePolicy struct {
	ScenarioID                string
	BuilderID                 string
	RequestKind               string
	Symbol                    string
	File                      string
	Query                     string
	MaxCandidates             int
	View                      string
	ExpectedKind              string
	ExpectedSeedKind          string
	MinimumCandidates         int
	RequireModuleDecisionRef  bool
	RequiredModuleDecisionRef string
	RequireDiagnostics        bool
}

type p14CodeExploreSemanticRequest struct {
	Schema                    string   `json:"schema"`
	ScenarioID                string   `json:"scenario_id"`
	RequestKind               string   `json:"request_kind"`
	Symbol                    string   `json:"symbol,omitempty"`
	File                      string   `json:"file,omitempty"`
	Query                     string   `json:"query,omitempty"`
	MaxCandidates             int      `json:"max_candidates,omitempty"`
	View                      string   `json:"view"`
	ExpectedKind              string   `json:"expected_kind"`
	ExpectedSeedKind          string   `json:"expected_seed_kind"`
	MinimumCandidates         int      `json:"minimum_candidates"`
	RequireModuleDecisionRef  bool     `json:"require_module_decision_ref"`
	RequiredModuleDecisionRef string   `json:"required_module_decision_ref,omitempty"`
	RequireDiagnostics        bool     `json:"require_diagnostics"`
	RequiredBindings          []string `json:"required_bindings"`
}

type p14CodeExploreCLISurface struct {
	Schema                string   `json:"schema"`
	SemanticRequestDigest string   `json:"semantic_request_digest"`
	Argv                  []string `json:"argv"`
}

type p14CodeExploreMCPSurface struct {
	Schema                string         `json:"schema"`
	SemanticRequestDigest string         `json:"semantic_request_digest"`
	Tool                  string         `json:"tool"`
	Args                  map[string]any `json:"args"`
}

type p14CodeExploreExecutor func(
	context.Context,
	string,
	string,
	p14CodeExploreSemanticRequest,
	[]string,
) (p14FPFProjectionCommandObservation, error)

type p14CodeExploreNormalizedOutput struct {
	Schema          string                              `json:"schema"`
	ScenarioID      string                              `json:"scenario_id"`
	ContractVersion string                              `json:"contract_version"`
	View            string                              `json:"view"`
	Kind            string                              `json:"kind"`
	RequestKind     string                              `json:"request_kind"`
	SeedKind        string                              `json:"seed_kind"`
	ExactSeed       *p14CodeExploreNormalizedExactSeed  `json:"exact_seed,omitempty"`
	Candidates      []p14CodeExploreNormalizedCandidate `json:"candidates"`
	Reasoning       []p14CodeExploreNormalizedReasoning `json:"reasoning"`
	ModuleDecisions []string                            `json:"module_decision_refs"`
	SourceDigest    string                              `json:"source_digest,omitempty"`
	Diagnostics     p14CodeExploreNormalizedDiagnostics `json:"diagnostics"`
}

type p14CodeExploreNormalizedExactSeed struct {
	AnchorID string `json:"anchor_id"`
	Name     string `json:"name"`
	File     string `json:"file"`
}

type p14CodeExploreNormalizedCandidate struct {
	Rank                            int      `json:"rank"`
	Name                            string   `json:"name"`
	QualifiedName                   string   `json:"qualified_name,omitempty"`
	SymbolKind                      string   `json:"symbol_kind"`
	File                            string   `json:"file"`
	RankingIsAdvisory               bool     `json:"ranking_is_advisory"`
	IdentityAutoSelected            bool     `json:"identity_auto_selected"`
	DecisionRefs                    []string `json:"decision_refs"`
	ExactBindingDecisionRefs        []string `json:"exact_binding_decision_refs"`
	AffectedPathContextDecisionRefs []string `json:"affected_path_context_decision_refs"`
	ModuleDecisionRefs              []string `json:"module_decision_refs"`
}

type p14CodeExploreNormalizedReasoning struct {
	SymbolAnchor                    string   `json:"symbol_anchor"`
	DecisionRefs                    []string `json:"decision_refs"`
	ExactBindingDecisionRefs        []string `json:"exact_binding_decision_refs"`
	AffectedPathContextDecisionRefs []string `json:"affected_path_context_decision_refs"`
	ModuleDecisionRefs              []string `json:"module_decision_refs"`
}

type p14CodeExploreNormalizedDiagnostics struct {
	ResolutionPresent bool `json:"resolution_present"`
	RetrievalPresent  bool `json:"retrieval_present"`
}

type p14CodeExploreLocalOracle struct {
	Schema                string   `json:"schema"`
	CandidateDigest       string   `json:"candidate_digest"`
	SemanticRequestDigest string   `json:"semantic_request_digest"`
	ExpectedResultDigest  string   `json:"expected_result_digest"`
	CandidateOutputDigest string   `json:"candidate_output_digest"`
	LocalOracleTests      []string `json:"local_oracle_tests"`
}

func p14CodeExplorePolicies() map[string]p14CodeExplorePolicy {
	return map[string]p14CodeExplorePolicy{
		p14CodeExploreExactBuilderID: {
			ScenarioID:                "code_graph_exact_explore",
			BuilderID:                 p14CodeExploreExactBuilderID,
			RequestKind:               "symbol",
			Symbol:                    "NeighborhoodRead",
			File:                      "internal/cli/memory_read_runtime.go",
			View:                      "working",
			ExpectedKind:              "resolved",
			ExpectedSeedKind:          "resolved_seed",
			RequireModuleDecisionRef:  true,
			RequiredModuleDecisionRef: p14CodeExploreCurrentDecisionRef,
		},
		p14CodeExploreConcernBuilderID: {
			ScenarioID:        "code_graph_concern_explore",
			BuilderID:         p14CodeExploreConcernBuilderID,
			RequestKind:       "concern",
			Query:             "How does typed project memory neighborhood read connect the CLI to project decisions?",
			MaxCandidates:     5,
			View:              "working",
			ExpectedKind:      "candidate_set",
			ExpectedSeedKind:  "candidate_set",
			MinimumCandidates: 1,
		},
		p14CodeExploreAmbiguousBuilderID: {
			ScenarioID:        "code_graph_ambiguous_concern",
			BuilderID:         p14CodeExploreAmbiguousBuilderID,
			RequestKind:       "concern",
			Query:             "memory read",
			MaxCandidates:     5,
			View:              "working",
			ExpectedKind:      "candidate_set",
			ExpectedSeedKind:  "candidate_set",
			MinimumCandidates: 2,
		},
		p14CodeExploreDiagnosticBuilderID: {
			ScenarioID:         "code_graph_coverage_diagnostic",
			BuilderID:          p14CodeExploreDiagnosticBuilderID,
			RequestKind:        "concern",
			Query:              "How does code governance path coverage resolve a file to its module?",
			MaxCandidates:      5,
			View:               "diagnostic",
			ExpectedKind:       "candidate_set",
			ExpectedSeedKind:   "candidate_set",
			MinimumCandidates:  1,
			RequireDiagnostics: true,
		},
	}
}

func buildP14CodeExploreScenario(
	ctx context.Context,
	declared scenarioContract,
	executable string,
	projectRoot string,
	executableDigest string,
	executor p14CodeExploreExecutor,
) (preparedP14Scenario, error) {
	policy, err := p14CodeExplorePolicyForBuilder(declared.RequestBuilder)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	semantic := p14CodeExploreSemanticRequest{
		Schema:                    p14CodeExploreSemanticSchema,
		ScenarioID:                declared.ID,
		RequestKind:               policy.RequestKind,
		Symbol:                    policy.Symbol,
		File:                      policy.File,
		Query:                     policy.Query,
		MaxCandidates:             policy.MaxCandidates,
		View:                      policy.View,
		ExpectedKind:              policy.ExpectedKind,
		ExpectedSeedKind:          policy.ExpectedSeedKind,
		MinimumCandidates:         policy.MinimumCandidates,
		RequireModuleDecisionRef:  policy.RequireModuleDecisionRef,
		RequiredModuleDecisionRef: policy.RequiredModuleDecisionRef,
		RequireDiagnostics:        policy.RequireDiagnostics,
		RequiredBindings:          slices.Clone(declared.RequiredBindings),
	}
	if err := validateP14CodeExploreSemantic(declared, semantic); err != nil {
		return preparedP14Scenario{}, err
	}
	argv := p14CodeExploreCLIArgv(semantic)
	observation, err := executor(
		ctx,
		executable,
		projectRoot,
		semantic,
		argv,
	)
	if err != nil {
		return preparedP14Scenario{}, fmt.Errorf(
			"execute P14 code Explore %q: %w",
			declared.ID,
			err,
		)
	}
	normalized, candidateOutput, err := normalizeP14CodeExploreObservation(
		semantic,
		observation,
	)
	if err != nil {
		return preparedP14Scenario{}, fmt.Errorf(
			"normalize P14 code Explore %q: %w",
			declared.ID,
			err,
		)
	}
	semanticBytes, err := marshalP14CanonicalJSON(semantic)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	semanticDigest := p14Digest(semanticBytes)
	requests, err := buildP14CodeExploreSurfaceRequests(
		declared,
		semantic,
		semanticDigest,
	)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	normalizedBytes, err := marshalP14CanonicalJSON(normalized)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	expectedDigest := p14Digest(normalizedBytes)
	localOracle := p14CodeExploreLocalOracle{
		Schema:                p14CodeExploreLocalOracleSchema,
		CandidateDigest:       executableDigest,
		SemanticRequestDigest: semanticDigest,
		ExpectedResultDigest:  expectedDigest,
		CandidateOutputDigest: p14Digest(candidateOutput),
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
			NormalizationID:         p14CodeExploreNormalizationID,
			ExpectedResultDigest:    expectedDigest,
			ExpectedEffect:          declared.ExpectedEffect,
			LocalOracleOutputDigest: p14Digest(localOracleBytes),
		},
	}, nil
}

func p14CodeExplorePolicyForBuilder(
	builderID string,
) (p14CodeExplorePolicy, error) {
	policy, present := p14CodeExplorePolicies()[builderID]
	if !present {
		return p14CodeExplorePolicy{}, fmt.Errorf(
			"P14 code Explore builder %q is unknown",
			builderID,
		)
	}
	return policy, nil
}

func validateP14CodeExploreSemantic(
	declared scenarioContract,
	semantic p14CodeExploreSemanticRequest,
) error {
	policy, err := p14CodeExplorePolicyForBuilder(declared.RequestBuilder)
	if err != nil {
		return err
	}
	if semantic.Schema != p14CodeExploreSemanticSchema ||
		semantic.ScenarioID != declared.ID ||
		semantic.RequestKind != policy.RequestKind ||
		semantic.Symbol != policy.Symbol ||
		semantic.File != policy.File ||
		semantic.Query != policy.Query ||
		semantic.MaxCandidates != policy.MaxCandidates ||
		semantic.View != policy.View ||
		semantic.ExpectedKind != policy.ExpectedKind ||
		semantic.ExpectedSeedKind != policy.ExpectedSeedKind ||
		semantic.MinimumCandidates != policy.MinimumCandidates ||
		semantic.RequireModuleDecisionRef !=
			policy.RequireModuleDecisionRef ||
		semantic.RequiredModuleDecisionRef !=
			policy.RequiredModuleDecisionRef ||
		semantic.RequireDiagnostics != policy.RequireDiagnostics ||
		!slices.Equal(
			semantic.RequiredBindings,
			declared.RequiredBindings,
		) {
		return fmt.Errorf(
			"P14 code Explore semantic request differs for %q",
			declared.ID,
		)
	}
	if semantic.RequestKind == "symbol" &&
		(semantic.Symbol == "" || semantic.Query != "" ||
			semantic.MaxCandidates != 0) {
		return fmt.Errorf("P14 exact code Explore seed differs")
	}
	if semantic.RequestKind == "concern" &&
		(semantic.Query == "" || semantic.Symbol != "" ||
			semantic.File != "" || semantic.MaxCandidates <= 0) {
		return fmt.Errorf("P14 concern code Explore seed differs")
	}
	if semantic.RequireModuleDecisionRef &&
		semantic.RequiredModuleDecisionRef == "" {
		return fmt.Errorf(
			"P14 exact code Explore current decision is absent",
		)
	}
	return nil
}

func buildP14CodeExploreSurfaceRequests(
	declared scenarioContract,
	semantic p14CodeExploreSemanticRequest,
	semanticDigest string,
) ([]preparedP14Request, error) {
	builders := map[string]func() ([]byte, string, error){
		"installed_cli": func() ([]byte, string, error) {
			payload := p14CodeExploreCLISurface{
				Schema:                p14CodeExploreCLISurfaceSchema,
				SemanticRequestDigest: semanticDigest,
				Argv:                  p14CodeExploreCLIArgv(semantic),
			}
			raw, err := marshalP14CanonicalJSON(payload)
			return raw, "argv_json", err
		},
		"live_mcp": func() ([]byte, string, error) {
			payload := p14CodeExploreMCPSurface{
				Schema:                p14CodeExploreMCPSurfaceSchema,
				SemanticRequestDigest: semanticDigest,
				Tool:                  "haft_query",
				Args:                  p14CodeExploreMCPArgs(semantic),
			}
			raw, err := marshalP14CanonicalJSON(payload)
			return raw, "canonical_json", err
		},
	}
	requests := make([]preparedP14Request, 0, len(declared.Surfaces))
	for _, surface := range declared.Surfaces {
		builder, present := builders[surface]
		if !present {
			return nil, fmt.Errorf(
				"P14 code Explore surface %q is unsupported",
				surface,
			)
		}
		payload, encoding, err := builder()
		if err != nil {
			return nil, err
		}
		requests = append(requests, preparedP14Request{
			Surface:               surface,
			Builder:               declared.RequestBuilder,
			Encoding:              encoding,
			CanonicalPayload:      string(payload),
			PayloadDigest:         p14Digest(payload),
			SemanticRequestDigest: semanticDigest,
		})
	}
	return requests, nil
}

func p14CodeExploreCLIArgv(
	semantic p14CodeExploreSemanticRequest,
) []string {
	argv := []string{"graph", "explore"}
	if semantic.RequestKind == "symbol" {
		argv = append(argv, "--symbol", semantic.Symbol)
		if semantic.File != "" {
			argv = append(argv, "--file", semantic.File)
		}
	}
	if semantic.RequestKind == "concern" {
		argv = append(argv, "--query", semantic.Query)
		argv = append(
			argv,
			"--max-candidates",
			fmt.Sprintf("%d", semantic.MaxCandidates),
		)
	}
	argv = append(argv, "--view", semantic.View)
	return append(argv, "--json")
}

func p14CodeExploreMCPArgs(
	semantic p14CodeExploreSemanticRequest,
) map[string]any {
	args := map[string]any{
		"action": "explore",
		"view":   semantic.View,
	}
	if semantic.RequestKind == "symbol" {
		args["symbol"] = semantic.Symbol
		if semantic.File != "" {
			args["file"] = semantic.File
		}
	}
	if semantic.RequestKind == "concern" {
		args["query"] = semantic.Query
		args["max_candidates"] = semantic.MaxCandidates
	}
	return args
}

func executeP14CodeExploreCandidate(
	ctx context.Context,
	executable string,
	projectRoot string,
	_ p14CodeExploreSemanticRequest,
	argv []string,
) (p14FPFProjectionCommandObservation, error) {
	commandContext, cancel := context.WithTimeout(
		ctx,
		p14CodeExplorePreparationTimeout,
	)
	defer cancel()
	command := exec.CommandContext(commandContext, executable, argv...)
	command.Dir = projectRoot
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
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		return p14FPFProjectionCommandObservation{}, err
	}
	return p14FPFProjectionCommandObservation{
		Stdout:   slices.Clone(stdout.Bytes()),
		Stderr:   slices.Clone(stderr.Bytes()),
		ExitCode: exitError.ExitCode(),
	}, nil
}

func executeSyntheticP14CodeExploreCandidate(
	_ context.Context,
	_ string,
	_ string,
	semantic p14CodeExploreSemanticRequest,
	_ []string,
) (p14FPFProjectionCommandObservation, error) {
	payload := syntheticP14CodeExplorePayload(semantic)
	raw, err := marshalP14CanonicalJSON(payload)
	if err != nil {
		return p14FPFProjectionCommandObservation{}, err
	}
	return p14FPFProjectionCommandObservation{
		Stdout:   raw,
		ExitCode: 0,
	}, nil
}

func syntheticP14CodeExplorePayload(
	semantic p14CodeExploreSemanticRequest,
) map[string]any {
	payload := map[string]any{
		"contract_version": "haft.code_explore.v1",
		"view":             semantic.View,
		"kind":             semantic.ExpectedKind,
		"request_basis": map[string]any{
			"kind":           semantic.RequestKind,
			"symbol":         semantic.Symbol,
			"file":           semantic.File,
			"query":          semantic.Query,
			"max_candidates": semantic.MaxCandidates,
		},
		"seed_resolution": map[string]any{
			"kind": semantic.ExpectedSeedKind,
		},
	}
	if semantic.RequestKind == "symbol" {
		payload["source_hops"] = []any{
			map[string]any{
				"symbol": map[string]any{
					"anchor_id": "sym:v2:synthetic-neighborhood-read",
					"name":      semantic.Symbol,
					"file":      semantic.File,
				},
			},
		}
		payload["reasoning_context"] = []any{
			map[string]any{
				"symbol_anchor":                       "sym:v2:synthetic-neighborhood-read",
				"decisions":                           []any{},
				"exact_binding_decision_refs":         []any{},
				"affected_path_context_decision_refs": []any{},
				"module_decision_refs": []any{
					p14CodeExploreCurrentDecisionRef,
				},
			},
		}
		payload["source"] = map[string]any{
			"available": true,
			"content":   "func NeighborhoodRead() {}",
		}
		return payload
	}
	candidateCount := max(semantic.MinimumCandidates, 2)
	candidates := make([]any, 0, candidateCount)
	for index := range candidateCount {
		candidates = append(candidates, map[string]any{
			"rank": index + 1,
			"symbol": map[string]any{
				"name":           fmt.Sprintf("MemoryCandidate%d", index+1),
				"qualified_name": fmt.Sprintf("internal/cli.MemoryCandidate%d", index+1),
				"symbol_kind":    "func",
				"file":           fmt.Sprintf("internal/cli/memory_candidate_%d.go", index+1),
			},
			"ranking_is_advisory":    true,
			"identity_auto_selected": false,
			"governance": map[string]any{
				"decisions":            []any{},
				"module_decision_refs": []any{"dec-current-memory"},
			},
		})
	}
	payload["candidates"] = candidates
	if semantic.RequireDiagnostics {
		payload["resolution_diagnostics"] = map[string]any{"basis": "synthetic"}
		payload["retrieval_diagnostics"] = map[string]any{"basis": "synthetic"}
	}
	return payload
}

func normalizeP14CodeExploreObservation(
	semantic p14CodeExploreSemanticRequest,
	observation p14FPFProjectionCommandObservation,
) (
	p14CodeExploreNormalizedOutput,
	[]byte,
	error,
) {
	payload, canonical, err := decodeP14CodeExploreJSONObservation(observation)
	if err != nil {
		return p14CodeExploreNormalizedOutput{}, nil, err
	}
	contractVersion := p14JSONText(payload["contract_version"])
	view := p14JSONText(payload["view"])
	kind := p14JSONText(payload["kind"])
	seed := p14JSONMap(payload["seed_resolution"])
	seedKind := p14JSONText(seed["kind"])
	requestBasis := p14JSONMap(payload["request_basis"])
	if contractVersion != "haft.code_explore.v1" ||
		view != semantic.View ||
		kind != semantic.ExpectedKind ||
		seedKind != semantic.ExpectedSeedKind {
		return p14CodeExploreNormalizedOutput{}, nil, fmt.Errorf(
			"code Explore envelope differs",
		)
	}
	if err := validateP14CodeExploreRequestBasis(
		semantic,
		requestBasis,
	); err != nil {
		return p14CodeExploreNormalizedOutput{}, nil, err
	}
	candidates, candidateRefs, err := normalizeP14CodeExploreCandidates(
		payload["candidates"],
		semantic,
	)
	if err != nil {
		return p14CodeExploreNormalizedOutput{}, nil, err
	}
	reasoning, reasoningRefs, err := normalizeP14CodeExploreReasoning(
		payload["reasoning_context"],
	)
	if err != nil {
		return p14CodeExploreNormalizedOutput{}, nil, err
	}
	if err := validateP14CodeExploreDecisionLanes(
		semantic,
		candidates,
		reasoning,
	); err != nil {
		return p14CodeExploreNormalizedOutput{}, nil, err
	}
	exactSeed, err := normalizeP14CodeExploreExactSeed(
		payload["source_hops"],
		semantic,
		reasoning,
	)
	if err != nil {
		return p14CodeExploreNormalizedOutput{}, nil, err
	}
	moduleRefs := append(candidateRefs, reasoningRefs...)
	moduleRefs = p14SortedUniqueStrings(moduleRefs)
	if semantic.RequireModuleDecisionRef &&
		(exactSeed == nil ||
			!slices.Contains(
				reasoningRefs,
				semantic.RequiredModuleDecisionRef,
			) ||
			slices.Contains(
				reasoningRefs,
				p14CodeExploreStaleDecisionRef,
			)) {
		return p14CodeExploreNormalizedOutput{}, nil, fmt.Errorf(
			"exact code Explore omitted the current module decision",
		)
	}
	diagnostics := p14CodeExploreNormalizedDiagnostics{
		ResolutionPresent: payload["resolution_diagnostics"] != nil,
		RetrievalPresent:  payload["retrieval_diagnostics"] != nil,
	}
	if semantic.RequireDiagnostics &&
		(!diagnostics.ResolutionPresent ||
			!diagnostics.RetrievalPresent) {
		return p14CodeExploreNormalizedOutput{}, nil, fmt.Errorf(
			"diagnostic code Explore omitted diagnostics",
		)
	}
	sourceDigest := ""
	source := p14JSONMap(payload["source"])
	if p14JSONBool(source["available"]) {
		content := p14JSONText(source["content"])
		if content != "" {
			sourceDigest = p14Digest([]byte(content))
		}
	}
	normalized := p14CodeExploreNormalizedOutput{
		Schema:          p14CodeExploreNormalizedSchema,
		ScenarioID:      semantic.ScenarioID,
		ContractVersion: contractVersion,
		View:            view,
		Kind:            kind,
		RequestKind:     semantic.RequestKind,
		SeedKind:        seedKind,
		ExactSeed:       exactSeed,
		Candidates:      candidates,
		Reasoning:       reasoning,
		ModuleDecisions: moduleRefs,
		SourceDigest:    sourceDigest,
		Diagnostics:     diagnostics,
	}
	return normalized, canonical, nil
}

func normalizeP14CodeExploreExactSeed(
	raw any,
	semantic p14CodeExploreSemanticRequest,
	reasoning []p14CodeExploreNormalizedReasoning,
) (*p14CodeExploreNormalizedExactSeed, error) {
	if semantic.RequestKind != "symbol" {
		return nil, nil
	}
	hops := p14JSONArray(raw)
	if len(hops) == 0 {
		return nil, fmt.Errorf("exact code Explore omitted its seed hop")
	}
	symbol := p14JSONMap(p14JSONMap(hops[0])["symbol"])
	seed := p14CodeExploreNormalizedExactSeed{
		AnchorID: p14JSONText(symbol["anchor_id"]),
		Name:     p14JSONText(symbol["name"]),
		File:     p14JSONText(symbol["file"]),
	}
	if seed.AnchorID == "" ||
		seed.Name != semantic.Symbol ||
		seed.File != semantic.File {
		return nil, fmt.Errorf(
			"exact code Explore seed identity differs",
		)
	}
	matched := slices.ContainsFunc(
		reasoning,
		func(item p14CodeExploreNormalizedReasoning) bool {
			return item.SymbolAnchor == seed.AnchorID &&
				slices.Contains(
					item.ModuleDecisionRefs,
					semantic.RequiredModuleDecisionRef,
				) &&
				!slices.Contains(
					item.ExactBindingDecisionRefs,
					semantic.RequiredModuleDecisionRef,
				) &&
				!slices.Contains(
					item.AffectedPathContextDecisionRefs,
					semantic.RequiredModuleDecisionRef,
				)
		},
	)
	if !matched {
		return nil, fmt.Errorf(
			"exact code Explore seed is not tied to its module decision lane",
		)
	}
	return &seed, nil
}

func validateP14CodeExploreRequestBasis(
	semantic p14CodeExploreSemanticRequest,
	requestBasis map[string]any,
) error {
	if p14JSONText(requestBasis["kind"]) != semantic.RequestKind {
		return fmt.Errorf("code Explore request basis kind differs")
	}
	if semantic.RequestKind == "symbol" {
		if p14JSONText(requestBasis["symbol"]) != semantic.Symbol ||
			p14JSONText(requestBasis["file"]) != semantic.File ||
			p14JSONText(requestBasis["query"]) != "" {
			return fmt.Errorf("code Explore exact request basis differs")
		}
		return nil
	}
	maxCandidates, validMaxCandidates := p14JSONInt(
		requestBasis["max_candidates"],
	)
	if p14JSONText(requestBasis["query"]) != semantic.Query ||
		p14JSONText(requestBasis["symbol"]) != "" ||
		p14JSONText(requestBasis["file"]) != "" ||
		!validMaxCandidates ||
		maxCandidates != semantic.MaxCandidates {
		return fmt.Errorf("code Explore concern request basis differs")
	}
	return nil
}

func decodeP14CodeExploreJSONObservation(
	observation p14FPFProjectionCommandObservation,
) (map[string]any, []byte, error) {
	if observation.ExitCode != 0 {
		return nil, nil, fmt.Errorf(
			"code Explore exit code = %d; stderr=%s",
			observation.ExitCode,
			boundedP14FPFText(observation.Stderr),
		)
	}
	if len(bytes.TrimSpace(observation.Stderr)) != 0 {
		return nil, nil, fmt.Errorf(
			"code Explore emitted stderr on success: %s",
			boundedP14FPFText(observation.Stderr),
		)
	}
	canonical := bytes.TrimSpace(observation.Stdout)
	if len(canonical) == 0 || !canonicalCompactJSON(canonical) {
		return nil, nil, fmt.Errorf(
			"code Explore response is not compact JSON",
		)
	}
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.UseNumber()
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return nil, nil, fmt.Errorf("decode code Explore response: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return nil, nil, fmt.Errorf("code Explore response has trailing JSON")
	}
	return payload, slices.Clone(canonical), nil
}

func normalizeP14CodeExploreCandidates(
	raw any,
	semantic p14CodeExploreSemanticRequest,
) (
	[]p14CodeExploreNormalizedCandidate,
	[]string,
	error,
) {
	values := p14JSONArray(raw)
	if len(values) < semantic.MinimumCandidates {
		return nil, nil, fmt.Errorf(
			"code Explore candidate count = %d, want at least %d",
			len(values),
			semantic.MinimumCandidates,
		)
	}
	result := make(
		[]p14CodeExploreNormalizedCandidate,
		0,
		len(values),
	)
	moduleRefs := make([]string, 0)
	for index, value := range values {
		candidate := p14JSONMap(value)
		symbol := p14JSONMap(candidate["symbol"])
		governance := p14JSONMap(candidate["governance"])
		rank, validRank := p14JSONInt(candidate["rank"])
		advisory := p14JSONBool(candidate["ranking_is_advisory"])
		autoSelected := p14JSONBool(candidate["identity_auto_selected"])
		if !validRank ||
			rank != index+1 ||
			p14JSONText(symbol["name"]) == "" ||
			p14JSONText(symbol["symbol_kind"]) == "" ||
			p14JSONText(symbol["file"]) == "" ||
			!advisory ||
			autoSelected {
			return nil, nil, fmt.Errorf(
				"code Explore candidate %d violates advisory identity semantics",
				index+1,
			)
		}
		candidateModuleRefs := p14JSONStrings(
			governance["module_decision_refs"],
		)
		moduleRefs = append(moduleRefs, candidateModuleRefs...)
		result = append(result, p14CodeExploreNormalizedCandidate{
			Rank:                 rank,
			Name:                 p14JSONText(symbol["name"]),
			QualifiedName:        p14JSONText(symbol["qualified_name"]),
			SymbolKind:           p14JSONText(symbol["symbol_kind"]),
			File:                 p14JSONText(symbol["file"]),
			RankingIsAdvisory:    advisory,
			IdentityAutoSelected: autoSelected,
			DecisionRefs: p14ArtifactIDs(
				governance["decisions"],
			),
			ExactBindingDecisionRefs: p14JSONStrings(
				governance["exact_binding_decision_refs"],
			),
			AffectedPathContextDecisionRefs: p14JSONStrings(
				governance["affected_path_context_decision_refs"],
			),
			ModuleDecisionRefs: candidateModuleRefs,
		})
	}
	return result, moduleRefs, nil
}

func validateP14CodeExploreDecisionLanes(
	semantic p14CodeExploreSemanticRequest,
	candidates []p14CodeExploreNormalizedCandidate,
	reasoning []p14CodeExploreNormalizedReasoning,
) error {
	if !semantic.RequireModuleDecisionRef {
		return nil
	}
	for _, candidate := range candidates {
		allRefs := slices.Concat(
			candidate.DecisionRefs,
			candidate.ExactBindingDecisionRefs,
			candidate.AffectedPathContextDecisionRefs,
			candidate.ModuleDecisionRefs,
		)
		if slices.Contains(allRefs, p14CodeExploreStaleDecisionRef) {
			return fmt.Errorf("code Explore exposed a superseded decision")
		}
		if slices.Contains(
			candidate.ExactBindingDecisionRefs,
			semantic.RequiredModuleDecisionRef,
		) || slices.Contains(
			candidate.AffectedPathContextDecisionRefs,
			semantic.RequiredModuleDecisionRef,
		) {
			return fmt.Errorf("code Explore collapsed module context into an authority lane")
		}
	}
	for _, item := range reasoning {
		allRefs := slices.Concat(
			item.DecisionRefs,
			item.ExactBindingDecisionRefs,
			item.AffectedPathContextDecisionRefs,
			item.ModuleDecisionRefs,
		)
		if slices.Contains(allRefs, p14CodeExploreStaleDecisionRef) {
			return fmt.Errorf("code Explore exposed a superseded decision")
		}
		if slices.Contains(
			item.ExactBindingDecisionRefs,
			semantic.RequiredModuleDecisionRef,
		) || slices.Contains(
			item.AffectedPathContextDecisionRefs,
			semantic.RequiredModuleDecisionRef,
		) {
			return fmt.Errorf("code Explore collapsed module context into an authority lane")
		}
	}
	return nil
}

func normalizeP14CodeExploreReasoning(
	raw any,
) (
	[]p14CodeExploreNormalizedReasoning,
	[]string,
	error,
) {
	values := p14JSONArray(raw)
	result := make(
		[]p14CodeExploreNormalizedReasoning,
		0,
		len(values),
	)
	moduleRefs := make([]string, 0)
	for index, value := range values {
		reasoning := p14JSONMap(value)
		anchor := p14JSONText(reasoning["symbol_anchor"])
		if anchor == "" {
			return nil, nil, fmt.Errorf(
				"code Explore reasoning item %d omits symbol anchor",
				index+1,
			)
		}
		currentModuleRefs := p14JSONStrings(
			reasoning["module_decision_refs"],
		)
		moduleRefs = append(moduleRefs, currentModuleRefs...)
		result = append(result, p14CodeExploreNormalizedReasoning{
			SymbolAnchor: anchor,
			DecisionRefs: p14ArtifactIDs(
				reasoning["decisions"],
			),
			ExactBindingDecisionRefs: p14JSONStrings(
				reasoning["exact_binding_decision_refs"],
			),
			AffectedPathContextDecisionRefs: p14JSONStrings(
				reasoning["affected_path_context_decision_refs"],
			),
			ModuleDecisionRefs: currentModuleRefs,
		})
	}
	return result, moduleRefs, nil
}

func p14ArtifactIDs(raw any) []string {
	values := p14JSONArray(raw)
	result := make([]string, 0, len(values))
	for _, value := range values {
		id := p14JSONText(p14JSONMap(value)["id"])
		if id != "" {
			result = append(result, id)
		}
	}
	return p14SortedUniqueStrings(result)
}

func p14JSONMap(value any) map[string]any {
	result, _ := value.(map[string]any)
	if result == nil {
		return map[string]any{}
	}
	return result
}

func p14JSONArray(value any) []any {
	result, _ := value.([]any)
	return result
}

func p14JSONText(value any) string {
	result, _ := value.(string)
	return strings.TrimSpace(result)
}

func p14JSONBool(value any) bool {
	result, _ := value.(bool)
	return result
}

func p14JSONInt(value any) (int, bool) {
	number, isNumber := value.(json.Number)
	if isNumber {
		parsed, err := number.Int64()
		if err == nil && parsed >= 0 {
			return int(parsed), true
		}
	}
	integer, isInteger := value.(int)
	return integer, isInteger && integer >= 0
}

func p14JSONStrings(value any) []string {
	values := p14JSONArray(value)
	result := make([]string, 0, len(values))
	for _, item := range values {
		text := p14JSONText(item)
		if text != "" {
			result = append(result, text)
		}
	}
	return p14SortedUniqueStrings(result)
}

func p14SortedUniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func decodeP14CodeExploreSemantic(
	raw []byte,
) (p14CodeExploreSemanticRequest, error) {
	semantic := p14CodeExploreSemanticRequest{}
	if err := decodeP14StrictCompactJSON(
		string(raw),
		&semantic,
		"code Explore semantic request",
	); err != nil {
		return p14CodeExploreSemanticRequest{}, err
	}
	return semantic, nil
}

func validateP14CodeExplorePreparedScenario(
	declared scenarioContract,
	scenario preparedP14Scenario,
) error {
	semantic, err := decodeP14CodeExploreSemantic(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return err
	}
	if err := validateP14CodeExploreSemantic(declared, semantic); err != nil {
		return err
	}
	expectedRequests, err := buildP14CodeExploreSurfaceRequests(
		declared,
		semantic,
		scenario.SemanticRequestDigest,
	)
	if err != nil {
		return err
	}
	expectedBytes, err := marshalP14CanonicalJSON(expectedRequests)
	if err != nil {
		return err
	}
	actualBytes, err := marshalP14CanonicalJSON(scenario.Requests)
	if err != nil {
		return err
	}
	if !bytes.Equal(expectedBytes, actualBytes) ||
		scenario.Oracle.NormalizationID !=
			p14CodeExploreNormalizationID {
		return fmt.Errorf(
			"P14 code Explore prepared scenario %q differs",
			declared.ID,
		)
	}
	return nil
}

func runP14InstalledCLICodeExplore(
	ctx context.Context,
	execution p14InstalledCLIExecutionContext,
	scenario preparedP14Scenario,
	request preparedP14Request,
) (p14InstalledCLIFamilyResult, error) {
	semantic, err := decodeP14CodeExploreSemantic(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	surface := p14CodeExploreCLISurface{}
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"code Explore installed CLI surface",
	); err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	if surface.Schema != p14CodeExploreCLISurfaceSchema ||
		surface.SemanticRequestDigest != scenario.SemanticRequestDigest ||
		!slices.Equal(surface.Argv, p14CodeExploreCLIArgv(semantic)) {
		return p14InstalledCLIFamilyResult{}, fmt.Errorf(
			"P14 code Explore installed CLI surface differs",
		)
	}
	fixture, err := beginP14InstalledCLIReadOnlyFixture(
		execution,
		scenario.ID,
	)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	project := execution.Prepared.Preparation.FrozenBasis.SelectedProject
	environment := p14InstalledCLIEnvironment(map[string]string{
		"HOME":                     fixture.HomeRoot,
		"HAFT_PROJECT_ROOT":        fixture.ProjectRoot,
		"HAFT_EXPECTED_PROJECT_ID": project.ProjectID,
	})
	results := p14ExecuteInstalledCLICalls(
		ctx,
		execution,
		fixture.ProjectRoot,
		environment,
		[]p14InstalledCLIProcessRequest{{
			ID:   "explore",
			Argv: slices.Clone(surface.Argv),
		}},
	)
	result := results[0]
	fixtureReceipt, err := finishP14InstalledCLIReadOnlyFixture(fixture)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	receipt := p14InstalledCLIExecutionReceipt{
		Schema:               p14InstalledCLIReceiptSchema,
		ScenarioID:           scenario.ID,
		Builder:              request.Builder,
		CandidateDigest:      execution.ExecutableDigest,
		RequestPayloadDigest: request.PayloadDigest,
		Fixture:              &fixtureReceipt,
		Commands:             p14InstalledCLICommandReceipts(results),
	}
	if p14InstalledCLIProcessResultsHaveExecutionFailure(results) {
		receipt.Checks = p14InstalledCLIChecks(
			false,
			"exact_graph_argv_from_sealed_payload",
			"closed_code_explore_normalizer",
		)
		return p14InstalledCLIFamilyResult{
			Receipt:        receipt,
			FailureCode:    "command_execution_failed",
			FailureDetail:  "installed code Explore command could not execute",
			ExecutionError: true,
		}, nil
	}
	normalized, _, normalizeErr := normalizeP14CodeExploreObservation(
		semantic,
		p14FPFProjectionCommandObservation{
			Stdout:   slices.Clone(result.Stdout),
			Stderr:   slices.Clone(result.Stderr),
			ExitCode: result.ExitCode,
		},
	)
	if normalizeErr != nil {
		receipt.Checks = p14InstalledCLIChecks(
			false,
			"exact_graph_argv_from_sealed_payload",
			"closed_code_explore_normalizer",
		)
		return p14InstalledCLIFamilyResult{
			Receipt:       receipt,
			FailureCode:   "normalization_failed",
			FailureDetail: boundedP14InstalledCLIError(normalizeErr),
		}, nil
	}
	if err := validateP14InstalledCLIReadOnlyFixture(
		fixtureReceipt,
	); err != nil {
		receipt.Checks = p14InstalledCLIChecks(
			false,
			"exact_graph_argv_from_sealed_payload",
			"closed_code_explore_normalizer",
		)
		return p14InstalledCLIFamilyResult{
			Receipt:       receipt,
			FailureCode:   "read_only_fixture_changed",
			FailureDetail: boundedP14InstalledCLIError(err),
		}, nil
	}
	normalizedBytes, err := marshalP14CanonicalJSON(normalized)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	receipt.Checks = p14InstalledCLIChecks(
		true,
		"exact_graph_argv_from_sealed_payload",
		"closed_code_explore_normalizer",
	)
	return p14InstalledCLIFamilyResult{
		Receipt:    receipt,
		Normalized: normalizedBytes,
	}, nil
}

func p14CodexMCPCodeExploreCalls(
	_ preparedP14Scenario,
	request preparedP14Request,
) ([]p14CodexMCPCallDefinition, error) {
	surface := p14CodeExploreMCPSurface{}
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"actual Codex MCP code Explore surface",
	); err != nil {
		return nil, err
	}
	if surface.Schema != p14CodeExploreMCPSurfaceSchema ||
		surface.Tool != "haft_query" ||
		len(surface.Args) == 0 {
		return nil, fmt.Errorf(
			"P14 Codex MCP code Explore surface differs",
		)
	}
	args, err := cloneP14JSONMap(surface.Args)
	if err != nil {
		return nil, err
	}
	return []p14CodexMCPCallDefinition{{
		CaseID: "explore",
		Tool:   surface.Tool,
		Args:   args,
	}}, nil
}

func normalizeP14CodexMCPCodeExplore(
	_ preparedRequestOracleCarrier,
	scenario preparedP14Scenario,
	_ preparedP14Request,
	evidence []p14CodexMCPCallEvidence,
) (p14CodexMCPFamilyResult, error) {
	semantic, err := decodeP14CodeExploreSemantic(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return p14CodexMCPFamilyResult{}, err
	}
	if len(evidence) != 1 || evidence[0].CaseID != "explore" {
		return p14CodexMCPNormalizedFailure(
			"code_explore_mismatch",
			"code Explore response count differs",
			"closed_code_explore_normalizer",
		), nil
	}
	body, err := p14CodexMCPResponseBody(evidence[0])
	if err != nil {
		return p14CodexMCPFamilyResult{}, err
	}
	exitCode := 0
	stderr := []byte(nil)
	stdout := slices.Clone(body)
	if evidence[0].Response.IsError {
		exitCode = 1
		stderr = slices.Clone(body)
		stdout = nil
	}
	normalized, _, normalizeErr := normalizeP14CodeExploreObservation(
		semantic,
		p14FPFProjectionCommandObservation{
			Stdout:   stdout,
			Stderr:   stderr,
			ExitCode: exitCode,
		},
	)
	return p14CodexMCPNormalizedResult(
		normalized,
		normalizeErr,
		"closed_code_explore_normalizer",
	)
}

func syntheticP14InstalledCLICodeExploreResult(
	scenario preparedP14Scenario,
	startedAt time.Time,
) (p14InstalledCLIProcessResult, error) {
	semantic, err := decodeP14CodeExploreSemantic(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return p14InstalledCLIProcessResult{}, err
	}
	payload, err := marshalP14CanonicalJSON(
		syntheticP14CodeExplorePayload(semantic),
	)
	if err != nil {
		return p14InstalledCLIProcessResult{}, err
	}
	return p14InstalledCLIProcessResult{
		ID:         "explore",
		Argv:       p14CodeExploreCLIArgv(semantic),
		StartedAt:  startedAt,
		FinishedAt: startedAt.Add(time.Millisecond),
		ExitCode:   0,
		Stdout:     payload,
	}, nil
}

func TestP14CodeExploreBuildersCloseInstalledAndLiveSurfaces(
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
	for _, builderID := range p14CodeExploreBuilderIDs {
		declared, err := findP14ScenarioContractByBuilder(
			contract,
			builderID,
		)
		if err != nil {
			t.Fatal(err)
		}
		scenario, err := buildP14CodeExploreScenario(
			context.Background(),
			declared,
			"/synthetic/haft",
			"/synthetic/project",
			p14TestDigest("synthetic-code-explore"),
			executeSyntheticP14CodeExploreCandidate,
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateP14CodeExplorePreparedScenario(
			declared,
			scenario,
		); err != nil {
			t.Fatal(err)
		}
		if len(scenario.Requests) != 2 ||
			scenario.Requests[0].Surface != "installed_cli" ||
			scenario.Requests[1].Surface != "live_mcp" {
			t.Fatalf(
				"P14 code Explore %q surfaces = %#v",
				declared.ID,
				scenario.Requests,
			)
		}
		tampered := scenario
		tampered.Requests = slices.Clone(scenario.Requests)
		tampered.Requests[0].CanonicalPayload = `{"schema":"tampered"}`
		tampered.Requests[0].PayloadDigest = p14Digest(
			[]byte(tampered.Requests[0].CanonicalPayload),
		)
		if err := validateP14CodeExplorePreparedScenario(
			declared,
			tampered,
		); err == nil {
			t.Fatalf(
				"P14 code Explore %q accepted surface drift",
				declared.ID,
			)
		}
	}
}

func TestP14CodeExploreNormalizerRejectsMissingModuleAndAdvisorySemantics(
	t *testing.T,
) {
	exact := p14CodeExploreSemanticRequest{
		Schema:                    p14CodeExploreSemanticSchema,
		ScenarioID:                "code_graph_exact_explore",
		RequestKind:               "symbol",
		Symbol:                    "NeighborhoodRead",
		File:                      "internal/cli/memory_read_runtime.go",
		View:                      "working",
		ExpectedKind:              "resolved",
		ExpectedSeedKind:          "resolved_seed",
		RequireModuleDecisionRef:  true,
		RequiredModuleDecisionRef: p14CodeExploreCurrentDecisionRef,
	}
	exactPayload := syntheticP14CodeExplorePayload(exact)
	exactPayload["reasoning_context"].([]any)[0].(map[string]any)["module_decision_refs"] = []any{}
	if _, _, err := normalizeP14CodeExplorePayloadForTest(
		exact,
		exactPayload,
	); err == nil {
		t.Fatal("P14 exact Explore accepted missing module decision context")
	}

	exactPayload = syntheticP14CodeExplorePayload(exact)
	exactReasoning := exactPayload["reasoning_context"].([]any)[0].(map[string]any)
	exactReasoning["module_decision_refs"] = []any{"dec-stale-or-unrelated"}
	if _, _, err := normalizeP14CodeExplorePayloadForTest(
		exact,
		exactPayload,
	); err == nil {
		t.Fatal("P14 exact Explore accepted another module decision")
	}

	exactPayload = syntheticP14CodeExplorePayload(exact)
	exactPayload["reasoning_context"].([]any)[0].(map[string]any)["symbol_anchor"] =
		"go:internal/cli:AnotherRead"
	if _, _, err := normalizeP14CodeExplorePayloadForTest(
		exact,
		exactPayload,
	); err == nil {
		t.Fatal("P14 exact Explore accepted another symbol anchor")
	}

	exactPayload = syntheticP14CodeExplorePayload(exact)
	exactPayload["source_hops"].([]any)[0].(map[string]any)["symbol"].(map[string]any)["file"] = "internal/cli/another_runtime.go"
	if _, _, err := normalizeP14CodeExplorePayloadForTest(
		exact,
		exactPayload,
	); err == nil {
		t.Fatal("P14 exact Explore accepted another seed file")
	}

	exactPayload = syntheticP14CodeExplorePayload(exact)
	exactReasoning = exactPayload["reasoning_context"].([]any)[0].(map[string]any)
	exactReasoning["module_decision_refs"] = []any{
		p14CodeExploreCurrentDecisionRef,
		p14CodeExploreStaleDecisionRef,
	}
	if _, _, err := normalizeP14CodeExplorePayloadForTest(
		exact,
		exactPayload,
	); err == nil {
		t.Fatal("P14 exact Explore accepted a superseded module decision")
	}

	exactPayload = syntheticP14CodeExplorePayload(exact)
	exactReasoning = exactPayload["reasoning_context"].([]any)[0].(map[string]any)
	exactReasoning["decisions"] = []any{
		map[string]any{"id": p14CodeExploreStaleDecisionRef},
	}
	if _, _, err := normalizeP14CodeExplorePayloadForTest(
		exact,
		exactPayload,
	); err == nil {
		t.Fatal("P14 exact Explore accepted a superseded aggregate decision")
	}

	exactPayload = syntheticP14CodeExplorePayload(exact)
	exactReasoning = exactPayload["reasoning_context"].([]any)[0].(map[string]any)
	exactReasoning["exact_binding_decision_refs"] = []any{
		p14CodeExploreCurrentDecisionRef,
	}
	if _, _, err := normalizeP14CodeExplorePayloadForTest(
		exact,
		exactPayload,
	); err == nil {
		t.Fatal("P14 exact Explore collapsed module context into exact binding")
	}

	exactPayload = syntheticP14CodeExplorePayload(exact)
	exactReasoning = exactPayload["reasoning_context"].([]any)[0].(map[string]any)
	exactReasoning["affected_path_context_decision_refs"] = []any{
		p14CodeExploreCurrentDecisionRef,
	}
	if _, _, err := normalizeP14CodeExplorePayloadForTest(
		exact,
		exactPayload,
	); err == nil {
		t.Fatal("P14 exact Explore collapsed module context into affected-path context")
	}

	exactPayload = syntheticP14CodeExplorePayload(exact)
	exactPayload["candidates"] = []any{
		map[string]any{
			"rank": 1,
			"symbol": map[string]any{
				"name":        "NeighborhoodRead",
				"symbol_kind": "func",
				"file":        "internal/cli/memory_read_runtime.go",
			},
			"ranking_is_advisory":    true,
			"identity_auto_selected": false,
			"governance": map[string]any{
				"decisions": []any{
					map[string]any{"id": p14CodeExploreStaleDecisionRef},
				},
				"exact_binding_decision_refs":         []any{},
				"affected_path_context_decision_refs": []any{},
				"module_decision_refs":                []any{},
			},
		},
	}
	if _, _, err := normalizeP14CodeExplorePayloadForTest(
		exact,
		exactPayload,
	); err == nil {
		t.Fatal("P14 exact Explore accepted a superseded candidate decision")
	}

	ambiguous := p14CodeExploreSemanticRequest{
		Schema:            p14CodeExploreSemanticSchema,
		ScenarioID:        "code_graph_ambiguous_concern",
		RequestKind:       "concern",
		Query:             "memory read",
		MaxCandidates:     5,
		View:              "working",
		ExpectedKind:      "candidate_set",
		ExpectedSeedKind:  "candidate_set",
		MinimumCandidates: 2,
	}
	ambiguousPayload := syntheticP14CodeExplorePayload(ambiguous)
	ambiguousPayload["candidates"].([]any)[0].(map[string]any)["identity_auto_selected"] = true
	if _, _, err := normalizeP14CodeExplorePayloadForTest(
		ambiguous,
		ambiguousPayload,
	); err == nil {
		t.Fatal("P14 concern Explore accepted implicit identity selection")
	}

	ambiguousPayload = syntheticP14CodeExplorePayload(ambiguous)
	ambiguousPayload["request_basis"].(map[string]any)["query"] =
		"a different concern"
	if _, _, err := normalizeP14CodeExplorePayloadForTest(
		ambiguous,
		ambiguousPayload,
	); err == nil {
		t.Fatal("P14 concern Explore accepted a mismatched request basis")
	}
}

func normalizeP14CodeExplorePayloadForTest(
	semantic p14CodeExploreSemanticRequest,
	payload map[string]any,
) (
	p14CodeExploreNormalizedOutput,
	[]byte,
	error,
) {
	raw, err := marshalP14CanonicalJSON(payload)
	if err != nil {
		return p14CodeExploreNormalizedOutput{}, nil, err
	}
	return normalizeP14CodeExploreObservation(
		semantic,
		p14FPFProjectionCommandObservation{
			Stdout:   raw,
			ExitCode: 0,
		},
	)
}
