package p14acceptance

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const (
	p14FPFProjectionBuilderID         = "fpf.query-projection-matrix.v1"
	p14FPFSemanticSchema              = "haft.p14.fpf-query-projection-semantic/v1"
	p14FPFCLISurfaceSchema            = "haft.p14.fpf-query-projection-cli/v1"
	p14FPFMCPSurfaceSchema            = "haft.p14.fpf-query-projection-mcp/v1"
	p14FPFNormalizedOutputSchema      = "haft.p14.fpf-query-projection-output/v1"
	p14FPFLocalOracleSchema           = "haft.p14.fpf-query-projection-local-oracle/v1"
	p14FPFNormalizationID             = "p14.fpf-query-projection.semantic-output.v1"
	p14FPFCandidateEnvironmentKey     = "HAFT_P14_FPF_QUERY_CANDIDATE"
	p14FPFExpectedRevisionEnvironment = "HAFT_P14_FPF_QUERY_EXPECTED_REVISION"
	p14FPFConcern                     = "How should intended Work remain distinct from evidence and authority?"
	p14FPFRequestMismatchConcern      = "How should performed Work remain distinct from evidence and authority?"
	p14FPFRelationPatternID           = "A.6.REL"
	p14FPFWorkPlanPatternID           = "A.15.2"
	p14FPFPatternBodyRole             = "pattern_body"
	p14FPFExpectedOutcomeJSON         = "json"
	p14FPFExpectedOutcomeError        = "error"
	p14FPFWorkingTraceRejectedCode    = "working_trace_ref_forbidden"
	p14FPFTraceRefPrefix              = "fpf-query-trace:v1"
	p14FPFTraceRefDigestCount         = 3
	p14FPFTraceRefDigestBytes         = 32
	p14FPFExpectedCandidateSetKind    = "candidate_set"
	p14FPFExpectedExactHitKind        = "exact_hit"
	p14FPFExpectedReplayMismatchKind  = "replay_mismatch"
	p14FPFRequestMismatchCode         = "query_request_mismatch"
	p14FPFSourceSnapshotMismatchCode  = "source_snapshot_mismatch"
)

var p14FPFWorkingForbiddenKeys = []string{
	"provenance",
	"source_path",
	"start_line",
	"end_line",
	"content_hash",
	"source_revision",
	"match_grounds",
	"tier",
	"probe_field",
	"source_field",
	"matched_value",
	"phrase_kind",
	"evidence",
	"projection_relation",
	"authored_phrases",
	"keywords",
	"target_class",
	"origin",
	"canonical_unit_id",
	"subject_pattern_id",
	"basis",
	"producer_ids",
	"concern",
	"query",
	"trace",
	"diagnostic",
}

type p14FPFProjectionSemanticRequest struct {
	Schema string                         `json:"schema"`
	Cases  []p14FPFProjectionSemanticCase `json:"cases"`
}

type p14FPFProjectionSemanticCase struct {
	ID          string                      `json:"id"`
	Query       p14FPFProjectionQuery       `json:"query"`
	Publication p14FPFProjectionPublication `json:"publication"`
	Expected    p14FPFProjectionExpected    `json:"expected"`
}

type p14FPFProjectionQuery struct {
	Mode            string                 `json:"mode"`
	Concern         string                 `json:"concern,omitempty"`
	Identifier      string                 `json:"identifier,omitempty"`
	Roles           []string               `json:"roles,omitempty"`
	EntityOfConcern string                 `json:"entity_of_concern,omitempty"`
	KnownContext    []string               `json:"known_context,omitempty"`
	IntendedUse     string                 `json:"intended_use,omitempty"`
	Budget          p14FPFProjectionBudget `json:"budget,omitempty"`
}

type p14FPFProjectionBudget struct {
	MaxCandidatesPerRole     int `json:"max_candidates_per_role,omitempty"`
	MaxTotalCandidates       int `json:"max_total_candidates,omitempty"`
	MaxExcerptCharacters     int `json:"max_excerpt_characters,omitempty"`
	MaxRelationsPerCandidate int `json:"max_relations_per_candidate,omitempty"`
}

type p14FPFProjectionPublication struct {
	View     string `json:"view,omitempty"`
	TraceRef string `json:"trace_ref,omitempty"`
}

type p14FPFProjectionExpected struct {
	Outcome      string   `json:"outcome"`
	View         string   `json:"view,omitempty"`
	Kind         string   `json:"kind,omitempty"`
	Code         string   `json:"code,omitempty"`
	EqualTo      string   `json:"equal_to,omitempty"`
	AssertionIDs []string `json:"assertion_ids,omitempty"`
}

type p14FPFProjectionCLISurface struct {
	Schema                string                    `json:"schema"`
	SemanticRequestDigest string                    `json:"semantic_request_digest"`
	Cases                 []p14FPFProjectionCLICase `json:"cases"`
}

type p14FPFProjectionCLICase struct {
	ID   string   `json:"id"`
	Argv []string `json:"argv"`
}

type p14FPFProjectionMCPSurface struct {
	Schema                string                    `json:"schema"`
	SemanticRequestDigest string                    `json:"semantic_request_digest"`
	Tool                  string                    `json:"tool"`
	Cases                 []p14FPFProjectionMCPCase `json:"cases"`
}

type p14FPFProjectionMCPCase struct {
	ID   string         `json:"id"`
	Args map[string]any `json:"args"`
}

type p14FPFProjectionCommandObservation struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

type p14FPFProjectionExecutor func(
	context.Context,
	string,
	p14FPFProjectionSemanticCase,
	[]string,
) (p14FPFProjectionCommandObservation, error)

type p14FPFProjectionNormalizedOutput struct {
	Schema string                                 `json:"schema"`
	Cases  []p14FPFProjectionNormalizedCaseOutput `json:"cases"`
}

type p14FPFProjectionNormalizedCaseOutput struct {
	ID      string          `json:"id"`
	Outcome string          `json:"outcome"`
	Code    string          `json:"code,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type p14FPFProjectionLocalOracle struct {
	Schema                string                            `json:"schema"`
	CandidateDigest       string                            `json:"candidate_digest"`
	SemanticRequestDigest string                            `json:"semantic_request_digest"`
	ExpectedResultDigest  string                            `json:"expected_result_digest"`
	CaseObservations      []p14FPFProjectionCaseObservation `json:"case_observations"`
}

type p14FPFProjectionCaseObservation struct {
	ID           string `json:"id"`
	Outcome      string `json:"outcome"`
	OutputDigest string `json:"output_digest"`
}

func buildP14FPFQueryProjectionScenario(
	ctx context.Context,
	declared scenarioContract,
	executable string,
	executableDigest string,
	expectedRevision string,
	executor p14FPFProjectionExecutor,
) (preparedP14Scenario, error) {
	traceCases := p14FPFProjectionTraceSeedCases()
	traceObservations, err := executeP14FPFProjectionCases(
		ctx,
		executable,
		traceCases,
		executor,
	)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	traceRefs, err := extractP14FPFProjectionTraceRefs(
		traceCases,
		traceObservations,
		expectedRevision,
	)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	cases, err := materializeP14FPFProjectionCases(traceRefs)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	observations, err := executeP14FPFProjectionCases(
		ctx,
		executable,
		cases,
		executor,
	)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	normalized, caseObservations, err := normalizeP14FPFProjectionObservations(
		cases,
		observations,
		expectedRevision,
	)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	semantic := p14FPFProjectionSemanticRequest{
		Schema: p14FPFSemanticSchema,
		Cases:  cases,
	}
	semanticBytes, err := marshalP14CanonicalJSON(semantic)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	semanticDigest := p14Digest(semanticBytes)
	requests, err := buildP14FPFProjectionSurfaceRequests(
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
	expectedResultDigest := p14Digest(normalizedBytes)
	localOracle := p14FPFProjectionLocalOracle{
		Schema:                p14FPFLocalOracleSchema,
		CandidateDigest:       executableDigest,
		SemanticRequestDigest: semanticDigest,
		ExpectedResultDigest:  expectedResultDigest,
		CaseObservations:      caseObservations,
	}
	localOracleBytes, err := marshalP14CanonicalJSON(localOracle)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	scenario := preparedP14Scenario{
		ID:                       declared.ID,
		SemanticRequestCanonical: string(semanticBytes),
		SemanticRequestDigest:    semanticDigest,
		Requests:                 requests,
		Oracle: preparedP14Oracle{
			Kind:                    declared.OracleKind,
			NormalizationID:         p14FPFNormalizationID,
			ExpectedResultDigest:    expectedResultDigest,
			ExpectedEffect:          declared.ExpectedEffect,
			LocalOracleOutputDigest: p14Digest(localOracleBytes),
		},
	}
	return scenario, nil
}

func p14FPFProjectionTraceSeedCases() []p14FPFProjectionSemanticCase {
	base := p14FPFProjectionBaseCases()
	traceCases := make([]p14FPFProjectionSemanticCase, 0, 3)
	for _, testCase := range base {
		if testCase.Publication.View == "trace" {
			traceCases = append(traceCases, testCase)
		}
	}
	return traceCases
}

func p14FPFProjectionBaseCases() []p14FPFProjectionSemanticCase {
	concern := p14FPFConcernQuery(p14FPFConcern)
	lookup := p14FPFExactQuery("lookup", p14FPFRelationPatternID)
	inspect := p14FPFExactQuery("inspect", p14FPFRelationPatternID)
	cases := make([]p14FPFProjectionSemanticCase, 0, 13)
	cases = append(cases, p14FPFProjectionViewCases("concern", concern, p14FPFExpectedCandidateSetKind)...)
	cases = append(cases, p14FPFProjectionViewCases("lookup_relation", lookup, p14FPFExpectedExactHitKind)...)
	cases = append(cases, p14FPFProjectionViewCases("inspect_relation", inspect, p14FPFExpectedExactHitKind)...)
	workPlan := p14FPFExactQuery("inspect", p14FPFWorkPlanPatternID)
	cases = append(cases, p14FPFProjectionSemanticCase{
		ID:          "inspect_workplan_working",
		Query:       workPlan,
		Publication: p14FPFProjectionPublication{View: "working"},
		Expected: p14FPFProjectionExpected{
			Outcome:      p14FPFExpectedOutcomeJSON,
			View:         "working",
			Kind:         p14FPFExpectedExactHitKind,
			AssertionIDs: []string{"working_denylist", "exact_workplan_body"},
		},
	})
	return cases
}

func p14FPFProjectionViewCases(
	prefix string,
	query p14FPFProjectionQuery,
	kind string,
) []p14FPFProjectionSemanticCase {
	workingAssertions := []string{"working_denylist"}
	if query.Mode == "inspect" {
		workingAssertions = append(workingAssertions, "exact_relation_body")
	}
	diagnosticAssertions := []string{"diagnostic_coordinates"}
	if query.Mode == "concern" {
		diagnosticAssertions = append(diagnosticAssertions, "diagnostic_retrieval_internals")
	}
	return []p14FPFProjectionSemanticCase{
		{
			ID:          prefix + "_default",
			Query:       query,
			Publication: p14FPFProjectionPublication{},
			Expected: p14FPFProjectionExpected{
				Outcome:      p14FPFExpectedOutcomeJSON,
				View:         "working",
				Kind:         kind,
				AssertionIDs: slices.Clone(workingAssertions),
			},
		},
		{
			ID:          prefix + "_working",
			Query:       query,
			Publication: p14FPFProjectionPublication{View: "working"},
			Expected: p14FPFProjectionExpected{
				Outcome:      p14FPFExpectedOutcomeJSON,
				View:         "working",
				Kind:         kind,
				EqualTo:      prefix + "_default",
				AssertionIDs: slices.Clone(workingAssertions),
			},
		},
		{
			ID:          prefix + "_trace",
			Query:       query,
			Publication: p14FPFProjectionPublication{View: "trace"},
			Expected: p14FPFProjectionExpected{
				Outcome:      p14FPFExpectedOutcomeJSON,
				View:         "trace",
				Kind:         kind,
				AssertionIDs: []string{"trace_reconstructable"},
			},
		},
		{
			ID:          prefix + "_diagnostic",
			Query:       query,
			Publication: p14FPFProjectionPublication{View: "diagnostic"},
			Expected: p14FPFProjectionExpected{
				Outcome:      p14FPFExpectedOutcomeJSON,
				View:         "diagnostic",
				Kind:         kind,
				AssertionIDs: diagnosticAssertions,
			},
		},
	}
}

func p14FPFConcernQuery(concern string) p14FPFProjectionQuery {
	return p14FPFProjectionQuery{
		Mode:            "concern",
		Concern:         concern,
		EntityOfConcern: "FPF Query public projection for Haft v9 installed runtime",
		KnownContext: []string{
			"intended Work is not performed Work",
			"retrieval metadata is not applicability or authority",
		},
		IntendedUse: "prepare one exact installed CLI and live MCP parity observation",
		Budget: p14FPFProjectionBudget{
			MaxCandidatesPerRole:     2,
			MaxTotalCandidates:       4,
			MaxExcerptCharacters:     480,
			MaxRelationsPerCandidate: 4,
		},
	}
}

func p14FPFExactQuery(mode string, identifier string) p14FPFProjectionQuery {
	return p14FPFProjectionQuery{
		Mode:       mode,
		Identifier: identifier,
		Roles:      []string{p14FPFPatternBodyRole},
		Budget: p14FPFProjectionBudget{
			MaxCandidatesPerRole:     2,
			MaxTotalCandidates:       4,
			MaxExcerptCharacters:     480,
			MaxRelationsPerCandidate: 4,
		},
	}
}

func materializeP14FPFProjectionCases(
	traceRefs map[string]string,
) ([]p14FPFProjectionSemanticCase, error) {
	required := []string{"concern", "lookup_relation", "inspect_relation"}
	for _, key := range required {
		if !validP14FPFTraceRef(traceRefs[key]) {
			return nil, fmt.Errorf("P14 FPF projection trace ref %q is invalid", key)
		}
	}
	cases := p14FPFProjectionBaseCases()
	queryByPrefix := map[string]p14FPFProjectionQuery{
		"concern":          p14FPFConcernQuery(p14FPFConcern),
		"lookup_relation":  p14FPFExactQuery("lookup", p14FPFRelationPatternID),
		"inspect_relation": p14FPFExactQuery("inspect", p14FPFRelationPatternID),
	}
	for _, prefix := range required {
		query := queryByPrefix[prefix]
		traceRef := traceRefs[prefix]
		cases = append(cases, p14FPFProjectionReplayCase(prefix, query, "trace", traceRef))
		cases = append(cases, p14FPFProjectionReplayCase(prefix, query, "diagnostic", traceRef))
	}
	requestMismatch := p14FPFProjectionReplayMismatchCase(
		"concern_request_mismatch",
		p14FPFConcernQuery(p14FPFRequestMismatchConcern),
		traceRefs["concern"],
		p14FPFRequestMismatchCode,
	)
	cases = append(cases, requestMismatch)
	mutatedRef, err := mutateP14FPFTraceSnapshot(traceRefs["concern"])
	if err != nil {
		return nil, err
	}
	snapshotMismatch := p14FPFProjectionReplayMismatchCase(
		"concern_snapshot_mismatch",
		p14FPFConcernQuery(p14FPFConcern),
		mutatedRef,
		p14FPFSourceSnapshotMismatchCode,
	)
	cases = append(cases, snapshotMismatch)
	cases = append(cases, p14FPFProjectionSemanticCase{
		ID:    "concern_working_replay_rejected",
		Query: p14FPFConcernQuery(p14FPFConcern),
		Publication: p14FPFProjectionPublication{
			View:     "working",
			TraceRef: traceRefs["concern"],
		},
		Expected: p14FPFProjectionExpected{
			Outcome: p14FPFExpectedOutcomeError,
			Code:    p14FPFWorkingTraceRejectedCode,
		},
	})
	return cases, nil
}

func p14FPFProjectionReplayCase(
	prefix string,
	query p14FPFProjectionQuery,
	view string,
	traceRef string,
) p14FPFProjectionSemanticCase {
	assertions := []string{"trace_reconstructable"}
	if view == "diagnostic" {
		assertions = []string{"diagnostic_coordinates"}
		if query.Mode == "concern" {
			assertions = append(assertions, "diagnostic_retrieval_internals")
		}
	}
	kind := p14FPFExpectedExactHitKind
	if query.Mode == "concern" {
		kind = p14FPFExpectedCandidateSetKind
	}
	return p14FPFProjectionSemanticCase{
		ID:    prefix + "_" + view + "_replay",
		Query: query,
		Publication: p14FPFProjectionPublication{
			View:     view,
			TraceRef: traceRef,
		},
		Expected: p14FPFProjectionExpected{
			Outcome:      p14FPFExpectedOutcomeJSON,
			View:         view,
			Kind:         kind,
			EqualTo:      prefix + "_" + view,
			AssertionIDs: assertions,
		},
	}
}

func p14FPFProjectionReplayMismatchCase(
	id string,
	query p14FPFProjectionQuery,
	traceRef string,
	code string,
) p14FPFProjectionSemanticCase {
	return p14FPFProjectionSemanticCase{
		ID:    id,
		Query: query,
		Publication: p14FPFProjectionPublication{
			View:     "trace",
			TraceRef: traceRef,
		},
		Expected: p14FPFProjectionExpected{
			Outcome: p14FPFExpectedOutcomeJSON,
			View:    "trace",
			Kind:    p14FPFExpectedReplayMismatchKind,
			Code:    code,
		},
	}
}

func mutateP14FPFTraceSnapshot(traceRef string) (string, error) {
	parts := strings.Split(traceRef, ":")
	wantParts := 2 + p14FPFTraceRefDigestCount
	if len(parts) != wantParts || !validP14FPFTraceRef(traceRef) {
		return "", fmt.Errorf("cannot mutate invalid P14 FPF trace ref")
	}
	snapshot := []byte(parts[2])
	replacement := byte('0')
	if snapshot[0] == replacement {
		replacement = '1'
	}
	snapshot[0] = replacement
	parts[2] = string(snapshot)
	mutated := strings.Join(parts, ":")
	return mutated, nil
}

func validP14FPFTraceRef(traceRef string) bool {
	parts := strings.Split(traceRef, ":")
	wantParts := 2 + p14FPFTraceRefDigestCount
	if len(parts) != wantParts || strings.Join(parts[:2], ":") != p14FPFTraceRefPrefix {
		return false
	}
	for _, digest := range parts[2:] {
		decoded, err := hex.DecodeString(digest)
		if err != nil || len(decoded) != p14FPFTraceRefDigestBytes {
			return false
		}
	}
	return true
}

func executeP14FPFProjectionCases(
	ctx context.Context,
	executable string,
	cases []p14FPFProjectionSemanticCase,
	executor p14FPFProjectionExecutor,
) (map[string]p14FPFProjectionCommandObservation, error) {
	observations := make(
		map[string]p14FPFProjectionCommandObservation,
		len(cases),
	)
	for _, testCase := range cases {
		if _, duplicate := observations[testCase.ID]; duplicate {
			return nil, fmt.Errorf("P14 FPF projection case %q is duplicated", testCase.ID)
		}
		argv, err := p14FPFProjectionCLIArgv(testCase)
		if err != nil {
			return nil, err
		}
		observation, err := executor(ctx, executable, testCase, argv)
		if err != nil {
			return nil, fmt.Errorf(
				"execute P14 FPF projection case %q: %w",
				testCase.ID,
				err,
			)
		}
		observations[testCase.ID] = observation
	}
	return observations, nil
}

func executeP14FPFProjectionCandidate(
	ctx context.Context,
	executable string,
	_ p14FPFProjectionSemanticCase,
	argv []string,
) (p14FPFProjectionCommandObservation, error) {
	command := exec.CommandContext(ctx, executable, argv...)
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

func extractP14FPFProjectionTraceRefs(
	cases []p14FPFProjectionSemanticCase,
	observations map[string]p14FPFProjectionCommandObservation,
	expectedRevision string,
) (map[string]string, error) {
	traceRefs := make(map[string]string, len(cases))
	for _, testCase := range cases {
		observation, present := observations[testCase.ID]
		if !present {
			return nil, fmt.Errorf("P14 FPF trace seed %q was not observed", testCase.ID)
		}
		payload, _, err := decodeP14CandidateJSONObservation(observation)
		if err != nil {
			return nil, fmt.Errorf("decode P14 FPF trace seed %q: %w", testCase.ID, err)
		}
		if err := assertP14FPFTraceReconstructable(payload, expectedRevision); err != nil {
			return nil, fmt.Errorf("validate P14 FPF trace seed %q: %w", testCase.ID, err)
		}
		traceRef, valid := payload["trace_ref"].(string)
		if !valid || !validP14FPFTraceRef(traceRef) {
			return nil, fmt.Errorf("P14 FPF trace seed %q returned invalid trace_ref", testCase.ID)
		}
		prefix := strings.TrimSuffix(testCase.ID, "_trace")
		traceRefs[prefix] = traceRef
	}
	return traceRefs, nil
}

func buildP14FPFProjectionSurfaceRequests(
	declared scenarioContract,
	semantic p14FPFProjectionSemanticRequest,
	semanticDigest string,
) ([]preparedP14Request, error) {
	builders := map[string]func() ([]byte, string, error){
		"installed_cli": func() ([]byte, string, error) {
			payload, err := buildP14FPFProjectionCLISurface(semantic, semanticDigest)
			return payload, "argv_json", err
		},
		"live_mcp": func() ([]byte, string, error) {
			payload, err := buildP14FPFProjectionMCPSurface(semantic, semanticDigest)
			return payload, "canonical_json", err
		},
	}
	requests := make([]preparedP14Request, 0, len(declared.Surfaces))
	for _, surface := range declared.Surfaces {
		builder, present := builders[surface]
		if !present {
			return nil, fmt.Errorf("P14 FPF projection surface %q is unsupported", surface)
		}
		payload, encoding, err := builder()
		if err != nil {
			return nil, err
		}
		requests = append(requests, preparedP14Request{
			Surface:               surface,
			Builder:               p14FPFProjectionBuilderID,
			Encoding:              encoding,
			CanonicalPayload:      string(payload),
			PayloadDigest:         p14Digest(payload),
			SemanticRequestDigest: semanticDigest,
		})
	}
	return requests, nil
}

func buildP14FPFProjectionCLISurface(
	semantic p14FPFProjectionSemanticRequest,
	semanticDigest string,
) ([]byte, error) {
	cases := make([]p14FPFProjectionCLICase, 0, len(semantic.Cases))
	for _, testCase := range semantic.Cases {
		argv, err := p14FPFProjectionCLIArgv(testCase)
		if err != nil {
			return nil, err
		}
		cases = append(cases, p14FPFProjectionCLICase{
			ID:   testCase.ID,
			Argv: argv,
		})
	}
	payload := p14FPFProjectionCLISurface{
		Schema:                p14FPFCLISurfaceSchema,
		SemanticRequestDigest: semanticDigest,
		Cases:                 cases,
	}
	return marshalP14CanonicalJSON(payload)
}

func buildP14FPFProjectionMCPSurface(
	semantic p14FPFProjectionSemanticRequest,
	semanticDigest string,
) ([]byte, error) {
	cases := make([]p14FPFProjectionMCPCase, 0, len(semantic.Cases))
	for _, testCase := range semantic.Cases {
		args, err := p14FPFProjectionMCPArgs(testCase)
		if err != nil {
			return nil, err
		}
		cases = append(cases, p14FPFProjectionMCPCase{
			ID:   testCase.ID,
			Args: args,
		})
	}
	payload := p14FPFProjectionMCPSurface{
		Schema:                p14FPFMCPSurfaceSchema,
		SemanticRequestDigest: semanticDigest,
		Tool:                  "haft_query",
		Cases:                 cases,
	}
	return marshalP14CanonicalJSON(payload)
}

func p14FPFProjectionCLIArgv(
	testCase p14FPFProjectionSemanticCase,
) ([]string, error) {
	query := testCase.Query
	argvBuilders := map[string]func() []string{
		"concern": func() []string {
			argv := []string{"fpf", "query", query.Concern}
			argv = appendP14CLIStringFlag(argv, "--entity-of-concern", query.EntityOfConcern)
			for _, value := range query.KnownContext {
				argv = appendP14CLIStringFlag(argv, "--known-context", value)
			}
			argv = appendP14CLIStringFlag(argv, "--intended-use", query.IntendedUse)
			return appendP14FPFBudgetFlags(argv, query.Budget)
		},
		"lookup": func() []string {
			argv := []string{"fpf", "lookup", query.Identifier}
			for _, role := range query.Roles {
				argv = appendP14CLIStringFlag(argv, "--role", role)
			}
			return appendP14FPFBudgetFlags(argv, query.Budget)
		},
		"inspect": func() []string {
			argv := []string{"fpf", "inspect", query.Identifier}
			for _, role := range query.Roles {
				argv = appendP14CLIStringFlag(argv, "--role", role)
			}
			return argv
		},
	}
	builder, present := argvBuilders[query.Mode]
	if !present {
		return nil, fmt.Errorf("P14 FPF projection mode %q is unsupported", query.Mode)
	}
	argv := builder()
	argv = appendP14CLIStringFlag(argv, "--view", testCase.Publication.View)
	argv = appendP14CLIStringFlag(argv, "--replay-ref", testCase.Publication.TraceRef)
	argv = append(argv, "--json")
	return argv, nil
}

func appendP14CLIStringFlag(argv []string, flag string, value string) []string {
	if value == "" {
		return argv
	}
	return append(argv, flag, value)
}

func appendP14FPFBudgetFlags(
	argv []string,
	budget p14FPFProjectionBudget,
) []string {
	values := []struct {
		Flag  string
		Value int
	}{
		{Flag: "--max-candidates-per-role", Value: budget.MaxCandidatesPerRole},
		{Flag: "--max-total-candidates", Value: budget.MaxTotalCandidates},
		{Flag: "--max-excerpt-characters", Value: budget.MaxExcerptCharacters},
		{Flag: "--max-relations-per-candidate", Value: budget.MaxRelationsPerCandidate},
	}
	for _, value := range values {
		if value.Value > 0 {
			argv = append(argv, value.Flag, fmt.Sprintf("%d", value.Value))
		}
	}
	return argv
}

func p14FPFProjectionMCPArgs(
	testCase p14FPFProjectionSemanticCase,
) (map[string]any, error) {
	query := testCase.Query
	args := map[string]any{
		"action": "fpf",
		"mode":   query.Mode,
	}
	queryBuilders := map[string]func(){
		"concern": func() {
			args["query"] = query.Concern
			setP14MCPString(args, "entity_of_concern", query.EntityOfConcern)
			setP14MCPStrings(args, "known_context", query.KnownContext)
			setP14MCPString(args, "intended_use", query.IntendedUse)
			setP14MCPBudget(args, query.Budget)
		},
		"lookup": func() {
			args["identifier"] = query.Identifier
			setP14MCPStrings(args, "roles", query.Roles)
			setP14MCPBudget(args, query.Budget)
		},
		"inspect": func() {
			args["identifier"] = query.Identifier
			setP14MCPStrings(args, "roles", query.Roles)
		},
	}
	builder, present := queryBuilders[query.Mode]
	if !present {
		return nil, fmt.Errorf("P14 FPF projection mode %q is unsupported", query.Mode)
	}
	builder()
	setP14MCPString(args, "view", testCase.Publication.View)
	setP14MCPString(args, "trace_ref", testCase.Publication.TraceRef)
	return args, nil
}

func setP14MCPString(args map[string]any, key string, value string) {
	if value != "" {
		args[key] = value
	}
}

func setP14MCPStrings(args map[string]any, key string, values []string) {
	if len(values) > 0 {
		args[key] = slices.Clone(values)
	}
}

func setP14MCPBudget(args map[string]any, budget p14FPFProjectionBudget) {
	values := map[string]int{
		"max_candidates_per_role":     budget.MaxCandidatesPerRole,
		"max_total_candidates":        budget.MaxTotalCandidates,
		"max_excerpt_characters":      budget.MaxExcerptCharacters,
		"max_relations_per_candidate": budget.MaxRelationsPerCandidate,
	}
	for key, value := range values {
		if value > 0 {
			args[key] = value
		}
	}
}

func marshalP14CanonicalJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode canonical P14 JSON: %w", err)
	}
	if !canonicalCompactJSON(raw) {
		return nil, fmt.Errorf("encoded P14 JSON is not compact canonical JSON")
	}
	return raw, nil
}

func normalizeP14FPFProjectionObservations(
	cases []p14FPFProjectionSemanticCase,
	observations map[string]p14FPFProjectionCommandObservation,
	expectedRevision string,
) (p14FPFProjectionNormalizedOutput, []p14FPFProjectionCaseObservation, error) {
	normalizedCases := make(
		[]p14FPFProjectionNormalizedCaseOutput,
		0,
		len(cases),
	)
	caseObservations := make(
		[]p14FPFProjectionCaseObservation,
		0,
		len(cases),
	)
	canonicalByID := make(map[string][]byte, len(cases))
	for _, testCase := range cases {
		observation, present := observations[testCase.ID]
		if !present {
			return p14FPFProjectionNormalizedOutput{}, nil, fmt.Errorf(
				"P14 FPF projection case %q was not observed",
				testCase.ID,
			)
		}
		normalized, canonical, err := normalizeP14FPFProjectionObservation(
			testCase,
			observation,
			expectedRevision,
		)
		if err != nil {
			return p14FPFProjectionNormalizedOutput{}, nil, fmt.Errorf(
				"P14 FPF projection case %q: %w",
				testCase.ID,
				err,
			)
		}
		if testCase.Expected.EqualTo != "" {
			expected, exists := canonicalByID[testCase.Expected.EqualTo]
			if !exists || !bytes.Equal(canonical, expected) {
				return p14FPFProjectionNormalizedOutput{}, nil, fmt.Errorf(
					"P14 FPF projection case %q differs from %q",
					testCase.ID,
					testCase.Expected.EqualTo,
				)
			}
		}
		canonicalByID[testCase.ID] = slices.Clone(canonical)
		normalizedCases = append(normalizedCases, normalized)
		caseObservations = append(caseObservations, p14FPFProjectionCaseObservation{
			ID:           testCase.ID,
			Outcome:      normalized.Outcome,
			OutputDigest: p14Digest(canonical),
		})
	}
	if len(observations) != len(cases) {
		return p14FPFProjectionNormalizedOutput{}, nil, fmt.Errorf(
			"P14 FPF projection observation count = %d, want %d",
			len(observations),
			len(cases),
		)
	}
	return p14FPFProjectionNormalizedOutput{
		Schema: p14FPFNormalizedOutputSchema,
		Cases:  normalizedCases,
	}, caseObservations, nil
}

func normalizeP14FPFProjectionObservation(
	testCase p14FPFProjectionSemanticCase,
	observation p14FPFProjectionCommandObservation,
	expectedRevision string,
) (p14FPFProjectionNormalizedCaseOutput, []byte, error) {
	normalizers := map[string]func() (p14FPFProjectionNormalizedCaseOutput, []byte, error){
		p14FPFExpectedOutcomeJSON: func() (p14FPFProjectionNormalizedCaseOutput, []byte, error) {
			return normalizeP14FPFProjectionJSONObservation(
				testCase,
				observation,
				expectedRevision,
			)
		},
		p14FPFExpectedOutcomeError: func() (p14FPFProjectionNormalizedCaseOutput, []byte, error) {
			return normalizeP14FPFProjectionErrorObservation(testCase, observation)
		},
	}
	normalizer, present := normalizers[testCase.Expected.Outcome]
	if !present {
		return p14FPFProjectionNormalizedCaseOutput{}, nil, fmt.Errorf(
			"expected outcome %q is unsupported",
			testCase.Expected.Outcome,
		)
	}
	return normalizer()
}

func normalizeP14FPFProjectionJSONObservation(
	testCase p14FPFProjectionSemanticCase,
	observation p14FPFProjectionCommandObservation,
	expectedRevision string,
) (p14FPFProjectionNormalizedCaseOutput, []byte, error) {
	payload, canonical, err := decodeP14CandidateJSONObservation(observation)
	if err != nil {
		return p14FPFProjectionNormalizedCaseOutput{}, nil, err
	}
	if payload["view"] != testCase.Expected.View {
		return p14FPFProjectionNormalizedCaseOutput{}, nil, fmt.Errorf(
			"view = %#v, want %q",
			payload["view"],
			testCase.Expected.View,
		)
	}
	if payload["kind"] != testCase.Expected.Kind {
		return p14FPFProjectionNormalizedCaseOutput{}, nil, fmt.Errorf(
			"kind = %#v, want %q",
			payload["kind"],
			testCase.Expected.Kind,
		)
	}
	if testCase.Expected.Code != "" && payload["code"] != testCase.Expected.Code {
		return p14FPFProjectionNormalizedCaseOutput{}, nil, fmt.Errorf(
			"code = %#v, want %q",
			payload["code"],
			testCase.Expected.Code,
		)
	}
	if err := assertP14FPFProjectionCase(
		testCase,
		payload,
		expectedRevision,
	); err != nil {
		return p14FPFProjectionNormalizedCaseOutput{}, nil, err
	}
	normalized := p14FPFProjectionNormalizedCaseOutput{
		ID:      testCase.ID,
		Outcome: p14FPFExpectedOutcomeJSON,
		Payload: slices.Clone(canonical),
	}
	return normalized, canonical, nil
}

func decodeP14CandidateJSONObservation(
	observation p14FPFProjectionCommandObservation,
) (map[string]any, []byte, error) {
	if observation.ExitCode != 0 {
		return nil, nil, fmt.Errorf(
			"candidate exit code = %d; stderr=%s",
			observation.ExitCode,
			boundedP14FPFText(observation.Stderr),
		)
	}
	if len(bytes.TrimSpace(observation.Stderr)) != 0 {
		return nil, nil, fmt.Errorf(
			"candidate emitted stderr on success: %s",
			boundedP14FPFText(observation.Stderr),
		)
	}
	if !bytes.HasSuffix(observation.Stdout, []byte("\n")) {
		return nil, nil, fmt.Errorf("candidate JSON lacks one terminal newline")
	}
	canonical := bytes.TrimSuffix(observation.Stdout, []byte("\n"))
	if len(canonical) == 0 || bytes.HasSuffix(canonical, []byte("\n")) {
		return nil, nil, fmt.Errorf("candidate JSON has invalid terminal newlines")
	}
	if !canonicalCompactJSON(canonical) {
		return nil, nil, fmt.Errorf("candidate response is not compact JSON")
	}
	reader := bytes.NewReader(canonical)
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return nil, nil, fmt.Errorf("decode candidate response: %w", err)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		return nil, nil, fmt.Errorf("candidate response has trailing JSON")
	}
	return payload, slices.Clone(canonical), nil
}

func normalizeP14FPFProjectionErrorObservation(
	testCase p14FPFProjectionSemanticCase,
	observation p14FPFProjectionCommandObservation,
) (p14FPFProjectionNormalizedCaseOutput, []byte, error) {
	if observation.ExitCode == 0 {
		return p14FPFProjectionNormalizedCaseOutput{}, nil, fmt.Errorf(
			"candidate accepted expected error %q",
			testCase.Expected.Code,
		)
	}
	if len(bytes.TrimSpace(observation.Stdout)) != 0 {
		return p14FPFProjectionNormalizedCaseOutput{}, nil, fmt.Errorf(
			"rejected request emitted stdout",
		)
	}
	markers := map[string][]string{
		p14FPFWorkingTraceRejectedCode: {
			"trace_ref",
			"trace or diagnostic",
		},
	}
	required, present := markers[testCase.Expected.Code]
	if !present {
		return p14FPFProjectionNormalizedCaseOutput{}, nil, fmt.Errorf(
			"error code %q has no closed normalizer",
			testCase.Expected.Code,
		)
	}
	stderr := string(observation.Stderr)
	for _, marker := range required {
		if !strings.Contains(stderr, marker) {
			return p14FPFProjectionNormalizedCaseOutput{}, nil, fmt.Errorf(
				"rejection stderr omits %q: %s",
				marker,
				boundedP14FPFText(observation.Stderr),
			)
		}
	}
	normalized := p14FPFProjectionNormalizedCaseOutput{
		ID:      testCase.ID,
		Outcome: p14FPFExpectedOutcomeError,
		Code:    testCase.Expected.Code,
	}
	canonical, err := marshalP14CanonicalJSON(normalized)
	if err != nil {
		return p14FPFProjectionNormalizedCaseOutput{}, nil, err
	}
	return normalized, canonical, nil
}

func boundedP14FPFText(raw []byte) string {
	const limit = 2048
	if len(raw) <= limit {
		return string(raw)
	}
	return string(raw[len(raw)-limit:])
}

func assertP14FPFProjectionCase(
	testCase p14FPFProjectionSemanticCase,
	payload map[string]any,
	expectedRevision string,
) error {
	assertions := map[string]func(map[string]any, string) error{
		"working_denylist":               assertP14FPFWorkingDenylist,
		"trace_reconstructable":          assertP14FPFTraceReconstructable,
		"diagnostic_coordinates":         assertP14FPFDiagnosticCoordinates,
		"diagnostic_retrieval_internals": assertP14FPFDiagnosticRetrievalInternals,
		"exact_relation_body":            assertP14FPFExactRelationBody,
		"exact_workplan_body":            assertP14FPFExactWorkPlanBody,
	}
	for _, assertionID := range testCase.Expected.AssertionIDs {
		assertion, present := assertions[assertionID]
		if !present {
			return fmt.Errorf("assertion %q is not closed", assertionID)
		}
		if err := assertion(payload, expectedRevision); err != nil {
			return fmt.Errorf("assertion %q: %w", assertionID, err)
		}
	}
	return nil
}

func assertP14FPFWorkingDenylist(payload map[string]any, _ string) error {
	keys := make(map[string]int)
	collectP14FPFJSONKeys(payload, keys)
	for _, forbidden := range p14FPFWorkingForbiddenKeys {
		if keys[forbidden] > 0 {
			return fmt.Errorf("working response contains forbidden key %q", forbidden)
		}
	}
	return nil
}

func assertP14FPFTraceReconstructable(
	payload map[string]any,
	expectedRevision string,
) error {
	keys := make(map[string]int)
	collectP14FPFJSONKeys(payload, keys)
	if keys["source_revision"] != 1 || keys["match_grounds"] != 0 {
		return fmt.Errorf("trace source revision or diagnostic-ground cardinality is invalid")
	}
	trace, valid := payload["trace"].(map[string]any)
	if !valid {
		return fmt.Errorf("trace object is absent")
	}
	snapshot, valid := trace["source_snapshot"].(map[string]any)
	if !valid || snapshot["source_revision"] != expectedRevision {
		return fmt.Errorf("trace source revision differs from frozen FPF revision")
	}
	for _, key := range []string{
		"index_schema_version",
		"readme_document_digest",
		"specification_document_digest",
	} {
		value, present := snapshot[key].(string)
		if !present || value == "" {
			return fmt.Errorf("trace snapshot %q is absent", key)
		}
	}
	provenance, valid := trace["provenance"].([]any)
	if !valid || len(provenance) == 0 {
		return fmt.Errorf("trace provenance catalog is absent")
	}
	refs, err := p14FPFTraceProvenanceRefs(provenance)
	if err != nil {
		return err
	}
	for _, key := range []string{
		"unit_bindings",
		"relation_bindings",
		"retrieval_evidence_bindings",
	} {
		if err := assertP14FPFTraceBindings(trace[key], refs); err != nil {
			return fmt.Errorf("trace %s: %w", key, err)
		}
	}
	unitBindings, _ := trace["unit_bindings"].([]any)
	if len(unitBindings) == 0 {
		return fmt.Errorf("trace has no unit binding")
	}
	return nil
}

func p14FPFTraceProvenanceRefs(provenance []any) (map[string]struct{}, error) {
	refs := make(map[string]struct{}, len(provenance))
	for _, value := range provenance {
		entry, valid := value.(map[string]any)
		if !valid {
			return nil, fmt.Errorf("trace provenance entry is not an object")
		}
		ref, refValid := entry["ref"].(string)
		path, pathValid := entry["source_path"].(string)
		hash, hashValid := entry["content_hash"].(string)
		start, startValid := p14FPFJSONPositiveInt(entry["start_line"])
		end, endValid := p14FPFJSONPositiveInt(entry["end_line"])
		if !refValid || ref == "" || !pathValid || path == "" ||
			!hashValid || len(hash) != 64 || !startValid || !endValid || end < start {
			return nil, fmt.Errorf("trace provenance entry is incomplete")
		}
		if _, duplicate := refs[ref]; duplicate {
			return nil, fmt.Errorf("trace provenance ref %q is duplicated", ref)
		}
		refs[ref] = struct{}{}
	}
	return refs, nil
}

func assertP14FPFTraceBindings(value any, refs map[string]struct{}) error {
	if value == nil {
		return nil
	}
	bindings, valid := value.([]any)
	if !valid {
		return fmt.Errorf("bindings are not an array")
	}
	for _, value := range bindings {
		binding, valid := value.(map[string]any)
		if !valid {
			return fmt.Errorf("binding is not an object")
		}
		ref, valid := binding["provenance_ref"].(string)
		if !valid {
			return fmt.Errorf("binding has no provenance_ref")
		}
		if _, present := refs[ref]; !present {
			return fmt.Errorf("binding provenance_ref %q is unresolved", ref)
		}
	}
	return nil
}

func p14FPFJSONPositiveInt(value any) (int64, bool) {
	number, valid := value.(json.Number)
	if !valid {
		return 0, false
	}
	parsed, err := number.Int64()
	return parsed, err == nil && parsed > 0
}

func assertP14FPFDiagnosticCoordinates(
	payload map[string]any,
	_ string,
) error {
	diagnostic, valid := payload["diagnostic"].(map[string]any)
	if !valid {
		return fmt.Errorf("diagnostic coordinates are absent")
	}
	mode, valid := diagnostic["retrieval_mode"].(string)
	if !valid || mode == "" {
		return fmt.Errorf("diagnostic retrieval_mode is absent")
	}
	producers, valid := diagnostic["producer_ids"].([]any)
	if !valid || len(producers) == 0 {
		return fmt.Errorf("diagnostic producer_ids are absent")
	}
	return nil
}

func assertP14FPFDiagnosticRetrievalInternals(
	payload map[string]any,
	_ string,
) error {
	keys := make(map[string]int)
	collectP14FPFJSONKeys(payload, keys)
	for _, required := range []string{
		"provenance",
		"match_grounds",
		"tier",
		"probe_field",
		"source_field",
		"matched_value",
		"producer_ids",
	} {
		if keys[required] == 0 {
			return fmt.Errorf("diagnostic response omits %q", required)
		}
	}
	return nil
}

func assertP14FPFExactRelationBody(payload map[string]any, _ string) error {
	return assertP14FPFExactBody(
		payload,
		p14FPFRelationPatternID,
		[]string{
			"### A.6.REL:1 - Problem frame",
			"### A.6.REL:2 - Problem",
			"### A.6.REL:3 - Forces",
			"### A.6.REL:4 - Solution",
			"### A.6.REL:5 - Archetypal Grounding",
			"### A.6.REL:7 - Conformance Checklist",
			"### A.6.REL:End",
		},
	)
}

func assertP14FPFExactWorkPlanBody(payload map[string]any, _ string) error {
	return assertP14FPFExactBody(
		payload,
		p14FPFWorkPlanPatternID,
		[]string{
			"### A.15.2:1 - Context",
			"### A.15.2:2 - Problem",
			"### A.15.2:3 - Forces",
			"### A.15.2:4 - Solution",
			"### A.15.2:7a - Conformance Checklist",
			"### A.15.2:End",
		},
	)
}

func assertP14FPFExactBody(
	payload map[string]any,
	patternID string,
	markers []string,
) error {
	identifier, valid := payload["identifier"].(string)
	if !valid || identifier != patternID {
		return fmt.Errorf("exact identifier differs from %q", patternID)
	}
	unit, valid := payload["unit"].(map[string]any)
	if !valid || unit["pattern_id"] != patternID {
		return fmt.Errorf("exact unit pattern_id differs from %q", patternID)
	}
	body, valid := unit["body"].(string)
	if !valid || body == "" {
		return fmt.Errorf("exact unit body is absent")
	}
	for _, marker := range markers {
		if !strings.Contains(body, marker) {
			return fmt.Errorf("exact %s body omits %q", patternID, marker)
		}
	}
	return nil
}

func collectP14FPFJSONKeys(value any, keys map[string]int) {
	collectors := map[string]func(){
		"object": func() {
			object := value.(map[string]any)
			for key, child := range object {
				keys[key]++
				collectP14FPFJSONKeys(child, keys)
			}
		},
		"array": func() {
			array := value.([]any)
			for _, child := range array {
				collectP14FPFJSONKeys(child, keys)
			}
		},
	}
	kind := "scalar"
	if _, valid := value.(map[string]any); valid {
		kind = "object"
	}
	if _, valid := value.([]any); valid {
		kind = "array"
	}
	collector, present := collectors[kind]
	if present {
		collector()
	}
}

func validateP14FPFProjectionPreparedScenario(
	declared scenarioContract,
	scenario preparedP14Scenario,
) error {
	if declared.RequestBuilder != p14FPFProjectionBuilderID {
		return fmt.Errorf("P14 FPF projection validator received builder %q", declared.RequestBuilder)
	}
	semantic, err := decodeP14FPFProjectionSemanticRequest(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return err
	}
	if err := validateP14FPFProjectionSemanticRequest(semantic); err != nil {
		return err
	}
	expectedRequests, err := buildP14FPFProjectionSurfaceRequests(
		declared,
		semantic,
		scenario.SemanticRequestDigest,
	)
	if err != nil {
		return err
	}
	if len(expectedRequests) != len(scenario.Requests) {
		return fmt.Errorf("P14 FPF projection surface request count differs")
	}
	for index, expected := range expectedRequests {
		actual := scenario.Requests[index]
		if actual != expected {
			return fmt.Errorf(
				"P14 FPF projection surface request %q is not derived from the semantic carrier",
				expected.Surface,
			)
		}
	}
	if scenario.Oracle.NormalizationID != p14FPFNormalizationID {
		return fmt.Errorf("P14 FPF projection normalizer is not exact")
	}
	return nil
}

func decodeP14FPFProjectionSemanticRequest(
	raw []byte,
) (p14FPFProjectionSemanticRequest, error) {
	reader := bytes.NewReader(raw)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var semantic p14FPFProjectionSemanticRequest
	if err := decoder.Decode(&semantic); err != nil {
		return p14FPFProjectionSemanticRequest{}, fmt.Errorf(
			"decode P14 FPF projection semantic carrier: %w",
			err,
		)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		return p14FPFProjectionSemanticRequest{}, fmt.Errorf(
			"P14 FPF projection semantic carrier has trailing JSON",
		)
	}
	return semantic, nil
}

func validateP14FPFProjectionSemanticRequest(
	semantic p14FPFProjectionSemanticRequest,
) error {
	if semantic.Schema != p14FPFSemanticSchema {
		return fmt.Errorf("P14 FPF projection semantic schema is invalid")
	}
	traceRefs, err := p14FPFProjectionTraceRefsFromMaterializedCases(semantic.Cases)
	if err != nil {
		return err
	}
	expectedCases, err := materializeP14FPFProjectionCases(traceRefs)
	if err != nil {
		return err
	}
	actualBytes, err := marshalP14CanonicalJSON(semantic.Cases)
	if err != nil {
		return err
	}
	expectedBytes, err := marshalP14CanonicalJSON(expectedCases)
	if err != nil {
		return err
	}
	if !bytes.Equal(actualBytes, expectedBytes) {
		return fmt.Errorf("P14 FPF projection semantic matrix is not the closed 22-case contract")
	}
	return nil
}

func p14FPFProjectionTraceRefsFromMaterializedCases(
	cases []p14FPFProjectionSemanticCase,
) (map[string]string, error) {
	byID := make(map[string]p14FPFProjectionSemanticCase, len(cases))
	for _, testCase := range cases {
		if _, duplicate := byID[testCase.ID]; duplicate {
			return nil, fmt.Errorf("P14 FPF projection semantic case %q is duplicated", testCase.ID)
		}
		byID[testCase.ID] = testCase
	}
	coordinates := map[string]string{
		"concern":          "concern_trace_replay",
		"lookup_relation":  "lookup_relation_trace_replay",
		"inspect_relation": "inspect_relation_trace_replay",
	}
	traceRefs := make(map[string]string, len(coordinates))
	for prefix, caseID := range coordinates {
		testCase, present := byID[caseID]
		if !present || !validP14FPFTraceRef(testCase.Publication.TraceRef) {
			return nil, fmt.Errorf("P14 FPF projection replay coordinate %q is invalid", caseID)
		}
		traceRefs[prefix] = testCase.Publication.TraceRef
	}
	return traceRefs, nil
}

func syntheticP14FPFProjectionScenario(
	declared scenarioContract,
) (preparedP14Scenario, error) {
	traceRefs := map[string]string{
		"concern":          syntheticP14FPFTraceRef("concern"),
		"lookup_relation":  syntheticP14FPFTraceRef("lookup"),
		"inspect_relation": syntheticP14FPFTraceRef("inspect"),
	}
	cases, err := materializeP14FPFProjectionCases(traceRefs)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	semantic := p14FPFProjectionSemanticRequest{
		Schema: p14FPFSemanticSchema,
		Cases:  cases,
	}
	semanticBytes, err := marshalP14CanonicalJSON(semantic)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	semanticDigest := p14Digest(semanticBytes)
	requests, err := buildP14FPFProjectionSurfaceRequests(
		declared,
		semantic,
		semanticDigest,
	)
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
			NormalizationID:         p14FPFNormalizationID,
			ExpectedResultDigest:    p14TestDigest("fpf-query-result"),
			ExpectedEffect:          declared.ExpectedEffect,
			LocalOracleOutputDigest: p14TestDigest("fpf-query-local-oracle"),
		},
	}, nil
}

func syntheticP14FPFTraceRef(seed string) string {
	digests := []string{
		strings.TrimPrefix(p14TestDigest(seed+":snapshot"), "sha256:"),
		strings.TrimPrefix(p14TestDigest(seed+":request"), "sha256:"),
		strings.TrimPrefix(p14TestDigest(seed+":result"), "sha256:"),
	}
	parts := append([]string{p14FPFTraceRefPrefix}, digests...)
	return strings.Join(parts, ":")
}

func findP14FPFProjectionScenarioContract(
	contract requestOracleContract,
) (scenarioContract, error) {
	for _, scenario := range contract.Scenarios {
		if scenario.ID == "fpf_query_projection" {
			return scenario, nil
		}
	}
	return scenarioContract{}, fmt.Errorf("P14 FPF projection scenario is absent")
}

func TestP14FPFQueryProjectionBuilderProducesSharedExactSurfaceMatrix(t *testing.T) {
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, _, err := loadRequestOracleContract(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	declared, err := findP14FPFProjectionScenarioContract(contract)
	if err != nil {
		t.Fatal(err)
	}
	revision := strings.Repeat("a", 40)
	scenario, err := buildP14FPFQueryProjectionScenario(
		context.Background(),
		declared,
		"/synthetic/haft",
		p14TestDigest("synthetic-candidate"),
		revision,
		executeSyntheticP14FPFProjectionCandidate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateP14FPFProjectionPreparedScenario(declared, scenario); err != nil {
		t.Fatal(err)
	}
	semantic, err := decodeP14FPFProjectionSemanticRequest(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(semantic.Cases) != 22 {
		t.Fatalf("P14 FPF projection case count = %d, want 22", len(semantic.Cases))
	}
	if len(scenario.Requests) != 2 ||
		scenario.Requests[0].Encoding != "argv_json" ||
		scenario.Requests[1].Encoding != "canonical_json" {
		t.Fatalf("P14 FPF projection surface encodings = %#v", scenario.Requests)
	}
	tampered := scenario
	tampered.Requests = slices.Clone(scenario.Requests)
	tampered.Requests[0].CanonicalPayload = `{"schema":"tampered"}`
	tampered.Requests[0].PayloadDigest = p14Digest([]byte(tampered.Requests[0].CanonicalPayload))
	if err := validateP14FPFProjectionPreparedScenario(declared, tampered); err == nil {
		t.Fatal("P14 FPF projection validator accepted a divergent CLI matrix")
	}
}

func TestP14CaptureFPFQueryProjectionAgainstCandidate(t *testing.T) {
	executable := os.Getenv(p14FPFCandidateEnvironmentKey)
	if executable == "" {
		t.Skip("set HAFT_P14_FPF_QUERY_CANDIDATE after the candidate basis is frozen")
	}
	expectedRevision := os.Getenv(p14FPFExpectedRevisionEnvironment)
	if expectedRevision == "" {
		t.Fatal("HAFT_P14_FPF_QUERY_EXPECTED_REVISION is required with the candidate")
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
	declared, err := findP14FPFProjectionScenarioContract(contract)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scenario, err := buildP14FPFQueryProjectionScenario(
		ctx,
		declared,
		executable,
		p14Digest(executableBytes),
		expectedRevision,
		executeP14FPFProjectionCandidate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateP14FPFProjectionPreparedScenario(declared, scenario); err != nil {
		t.Fatal(err)
	}
	t.Logf(
		"P14_FPF_QUERY_PREPARED semantic=%s expected=%s local_oracle=%s",
		scenario.SemanticRequestDigest,
		scenario.Oracle.ExpectedResultDigest,
		scenario.Oracle.LocalOracleOutputDigest,
	)
}

func executeSyntheticP14FPFProjectionCandidate(
	_ context.Context,
	_ string,
	testCase p14FPFProjectionSemanticCase,
	_ []string,
) (p14FPFProjectionCommandObservation, error) {
	if testCase.Expected.Outcome == p14FPFExpectedOutcomeError {
		return p14FPFProjectionCommandObservation{
			Stderr:   []byte("Error: trace_ref requires trace or diagnostic view\n"),
			ExitCode: 1,
		}, nil
	}
	payload := syntheticP14FPFProjectionPayload(testCase)
	raw, err := json.Marshal(payload)
	if err != nil {
		return p14FPFProjectionCommandObservation{}, err
	}
	raw = append(raw, '\n')
	return p14FPFProjectionCommandObservation{Stdout: raw, ExitCode: 0}, nil
}

func syntheticP14FPFProjectionPayload(
	testCase p14FPFProjectionSemanticCase,
) map[string]any {
	if testCase.Expected.Kind == p14FPFExpectedReplayMismatchKind {
		return map[string]any{
			"view":               testCase.Expected.View,
			"kind":               testCase.Expected.Kind,
			"code":               testCase.Expected.Code,
			"expected_trace_ref": testCase.Publication.TraceRef,
			"current_replay_basis_ref": "fpf-query-replay-basis:v1:" +
				strings.Repeat("b", 64) + ":" + strings.Repeat("c", 64),
		}
	}
	view := testCase.Expected.View
	traceRef := syntheticP14FPFQueryTraceRef(testCase.Query)
	if testCase.Publication.TraceRef != "" {
		traceRef = testCase.Publication.TraceRef
	}
	payload := map[string]any{
		"view":      view,
		"trace_ref": traceRef,
		"kind":      testCase.Expected.Kind,
	}
	if testCase.Expected.Kind == p14FPFExpectedCandidateSetKind {
		payload["groups"] = syntheticP14FPFCandidateGroups(view)
		payload["truncation"] = map[string]any{
			"applied":             true,
			"budget":              map[string]any{"max_total_candidates": 4},
			"included_candidates": 1,
			"omitted_at_least":    1,
		}
	}
	if testCase.Expected.Kind == p14FPFExpectedExactHitKind {
		payload["identifier"] = testCase.Query.Identifier
		payload["unit"] = syntheticP14FPFExactUnit(testCase.Query, view)
	}
	if view == "trace" {
		payload["trace"] = syntheticP14FPFTrace(strings.Repeat("a", 40))
	}
	if view == "diagnostic" {
		payload["diagnostic"] = map[string]any{
			"retrieval_mode": testCase.Query.Mode,
			"producer_ids":   []string{"exact_source", "role_local_fts"},
		}
		if testCase.Query.Mode == "concern" {
			payload["concern"] = testCase.Query.Concern
		}
	}
	return payload
}

func syntheticP14FPFQueryTraceRef(query p14FPFProjectionQuery) string {
	seed := query.Mode + ":" + query.Identifier + ":" + query.Concern
	return syntheticP14FPFTraceRef(seed)
}

func syntheticP14FPFCandidateGroups(view string) []any {
	source := map[string]any{
		"unit_id":           "readme:practical_use_card:working-documents",
		"source_id":         "WORKING-DOCUMENTS",
		"source_role":       "practical_use_card",
		"title":             "Working Documents",
		"excerpt":           "Keep intended Work distinct from performed Work and evidence.",
		"excerpt_truncated": false,
	}
	candidate := map[string]any{"source": source}
	if view == "diagnostic" {
		source["provenance"] = syntheticP14FPFProvenance()
		candidate["match_grounds"] = []any{map[string]any{
			"tier":          "source_phrase",
			"probe_field":   "query",
			"source_field":  "body",
			"matched_value": "Work",
		}}
	}
	return []any{map[string]any{
		"source_role": "practical_use_card",
		"candidates":  []any{candidate},
	}}
}

func syntheticP14FPFExactUnit(
	query p14FPFProjectionQuery,
	view string,
) map[string]any {
	unit := map[string]any{
		"unit_id":     "spec:pattern_body:" + strings.ToLower(strings.ReplaceAll(query.Identifier, ".", "-")),
		"source_id":   query.Identifier,
		"source_role": p14FPFPatternBodyRole,
		"title":       "Synthetic exact pattern",
		"pattern_id":  query.Identifier,
	}
	if query.Mode == "inspect" {
		unit["body"] = syntheticP14FPFExactBody(query.Identifier)
	}
	if view == "diagnostic" {
		unit["provenance"] = syntheticP14FPFProvenance()
	}
	return unit
}

func syntheticP14FPFExactBody(identifier string) string {
	bodies := map[string]string{
		p14FPFRelationPatternID: strings.Join([]string{
			"### A.6.REL:1 - Problem frame",
			"### A.6.REL:2 - Problem",
			"### A.6.REL:3 - Forces",
			"### A.6.REL:4 - Solution",
			"### A.6.REL:5 - Archetypal Grounding",
			"### A.6.REL:7 - Conformance Checklist",
			"### A.6.REL:End",
		}, "\n"),
		p14FPFWorkPlanPatternID: strings.Join([]string{
			"### A.15.2:1 - Context (plain-language motivation)",
			"### A.15.2:2 - Problem (what breaks without WorkPlan)",
			"### A.15.2:3 - Forces (what the definition balances)",
			"### A.15.2:4 - Solution - U.WorkPlan",
			"### A.15.2:7a - Conformance Checklist",
			"### A.15.2:End",
		}, "\n"),
	}
	return bodies[identifier]
}

func syntheticP14FPFTrace(revision string) map[string]any {
	provenance := syntheticP14FPFProvenance()
	return map[string]any{
		"source_snapshot": map[string]any{
			"index_schema_version":          "v1",
			"source_revision":               revision,
			"readme_document_digest":        p14TestDigest("readme"),
			"specification_document_digest": p14TestDigest("spec"),
		},
		"provenance": []any{map[string]any{
			"ref":          "source-provenance:v1:synthetic",
			"source_path":  provenance["source_path"],
			"start_line":   provenance["start_line"],
			"end_line":     provenance["end_line"],
			"content_hash": provenance["content_hash"],
		}},
		"unit_bindings": []any{map[string]any{
			"unit_id":        "synthetic-unit",
			"provenance_ref": "source-provenance:v1:synthetic",
		}},
	}
}

func syntheticP14FPFProvenance() map[string]any {
	return map[string]any{
		"source_path":     "data/FPF/FPF-Spec.md",
		"start_line":      10,
		"end_line":        20,
		"content_hash":    strings.Repeat("d", 64),
		"source_revision": strings.Repeat("a", 40),
	}
}
